# Plan: Ticket Not Saved — EscalateTicket Missing CreatorID

**Bug ID:** 053
**Date:** 2026-07-31
**Status:** Draft

---

## 1. Background

### User Prompt

> I just created a ticket when I run task run:rag but it was not saved in sqlite db, double check where it was stored

### Investigation Summary

Ran `task run:rag` (SQLite-backed dev server) and attempted to create a ticket. The ticket did not appear in the `tickets` table in `build/data/memos_dev.db`.

**What was checked:**

| Location | Result |
|----------|--------|
| SQLite DB (`build/data/memos_dev.db`) | 65 tickets present, all from 2026-07-30 (seeded), **zero from today** |
| CockroachDB (`bchat-crdb` Docker) | Only `agent_vectors` table — **no tickets table** |
| PostgreSQL (`bchat-postgres` Docker) | Running but not configured — no `DATABASE_URL` or `MEMOS_DRIVER=postgres` in `.env` |
| `.env` file | `MEMOS_DSN` commented out, no `DATABASE_URL`, no `COCKROACH_DSN` — defaults to SQLite |
| Shell environment | No `DATABASE_URL`, `MEMOS_DRIVER`, or `COCKROACH_DSN` set |
| WAL file | Flushed via `PRAGMA wal_checkpoint(TRUNCATE)` — no hidden data |
| Agent sessions / transcripts | `agent_sessions` empty, `agent_transcripts` has 61 rows from Jul 2–24 |
| Agent messages | 0 rows — no recent chat activity |

**Conclusion:** The ticket was **never saved to any database**. The creation failed at the application layer before reaching SQLite.

---

## 2. Root Cause Analysis

### The Bug

`EscalateTicket` in `server/router/api/v1/agent/service.go:5546-5553` constructs a `store.Ticket` **without setting `CreatorID`**:

```go
ticket := &store.Ticket{
    Title:       req.Title,
    Description: description,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    TenantID:    &tenant.ID,
    Tags:        req.Tags,
    // CreatorID is MISSING — Go zero-value for int32 is 0
}
```

The `tickets` table schema enforces:

```sql
creator_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE
```

`CreatorID = 0` violates both constraints:
1. **NOT NULL** — value 0 is technically non-NULL but semantically invalid
2. **FK to user(id)** — no user with `id = 0` exists (users are 1–4)

This causes the `INSERT` to fail with a **foreign key constraint violation**. The error is returned as `fmt.Errorf("failed to create ticket: %w", err)`, which the handler translates to HTTP 500 (`"Escalation service unavailable"`).

### Why Other Paths Work

| Path | CreatorID Set? | Source | Works? |
|------|---------------|--------|--------|
| `EscalateTicket` (service.go:5546) | **NO** | — | **NO** |
| `createEscalationTicket` (service.go:4727) | Yes | `s.systemTicketCreatorID(ctx)` | Yes |
| `createEscalationTicketFallback` (service.go:4792) | Yes | `s.systemTicketCreatorID(ctx)` | Yes |
| `handleAutoTicketCreation` (memo_service.go:1170) | Yes | `user.ID` | Yes |
| REST `CreateTicket` (ticket_service.go:90) | Yes | `userID` from JWT | Yes |

The `systemTicketCreatorID` function (service.go:4574) returns a valid system user ID (typically user 1, `ibm2100` with role `HOST`). All internal ticket creation paths use it, except `EscalateTicket`.

### Additional Missing Fields

`CreatedTs` and `UpdatedTs` are also not set in `EscalateTicket`, defaulting to 0. The `createEscalationTicket` path correctly sets them to `time.Now().Unix()`. These should be fixed simultaneously.

---

## 3. Implementation Plan

### Step 1: Fix `EscalateTicket` — Set CreatorID + Timestamps

**File:** `server/router/api/v1/agent/service.go`

At line 5546, add `CreatorID`, `CreatedTs`, and `UpdatedTs` to the ticket construction. The function doesn't receive an authenticated user context (it's called from the agent service), so use the system ticket creator ID:

```go
// Before (line 5546):
ticket := &store.Ticket{
    Title:       req.Title,
    Description: description,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    TenantID:    &tenant.ID,
    Tags:        req.Tags,
}

// After:
ticket := &store.Ticket{
    Title:       req.Title,
    Description: description,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    CreatorID:   s.systemTicketCreatorID(ctx),
    TenantID:    &tenant.ID,
    Tags:        req.Tags,
    CreatedTs:   time.Now().Unix(),
    UpdatedTs:   time.Now().Unix(),
}
```

### Step 2: Add Defensive Guard

After ticket construction, before `CreateTicket` call, add a zero-value check to catch this class of bug early:

```go
if ticket.CreatorID == 0 {
    slog.Error("EscalateTicket: CreatorID is 0, falling back to system creator",
        "title", req.Title, "tenant_id", tenant.ID)
    ticket.CreatorID = s.systemTicketCreatorID(ctx)
}
```

### Step 3: Verify `systemTicketCreatorID` Works

**File:** `server/router/api/v1/agent/service.go` (line 4574)

Already implemented — returns the system user ID from the database. No changes needed, just confirm it returns a valid user ID at runtime (user 1, `ibm2100`).

### Step 4: Audit Other Ticket Creation Paths (No Changes Needed)

All other `CreateTicket` callers already set `CreatorID` correctly:

| Caller | File:Line | CreatorID Source | Status |
|--------|-----------|-----------------|--------|
| `createEscalationTicket` | service.go:4739 | `systemTicketCreatorID(ctx)` | OK |
| `createEscalationTicketFallback` | service.go:4804 | `systemTicketCreatorID(ctx)` | OK |
| `handleAutoTicketCreation` | memo_service.go:1175 | `user.ID` | OK |
| REST `CreateTicket` | ticket_service.go:171 | `userID` from JWT | OK |

---

## 4. Files Modified

| File | Action | Description |
|------|--------|-------------|
| `server/router/api/v1/agent/service.go` | MODIFY | Add `CreatorID`, `CreatedTs`, `UpdatedTs` to `EscalateTicket` ticket construction (line 5546) |

---

## 5. Verification Plan

| Step | Command | Expected |
|------|---------|----------|
| Build | `task build:backend` | Compiles |
| Build (RAG) | `task build:backend:rag` | Compiles |
| Run | `task run:rag` | Server starts |
| Create ticket via escalation | `POST /api/v1/agent/:slug/escalate` with title + description | HTTP 200 with ticket ID |
| Verify in DB | `sqlite3 build/data/memos_dev.db "SELECT id, title, creator_id FROM tickets ORDER BY id DESC LIMIT 1"` | `creator_id = 1` |
| Verify timestamps | `sqlite3 build/data/memos_dev.db "SELECT created_ts, datetime(created_ts, 'unixepoch', 'localtime') FROM tickets ORDER BY id DESC LIMIT 1"` | Current timestamp |
| Verify creator user | `sqlite3 build/data/memos_dev.db "SELECT u.username FROM tickets t JOIN user u ON t.creator_id = u.id ORDER BY t.id DESC LIMIT 1"` | `ibm2100` |

---

## 6. Edge Cases

| Case | Behavior |
|------|----------|
| `systemTicketCreatorID` fails (no users in DB) | Returns 0 — fallback guard catches it; logged as error |
| Multiple rapid escalations | No race — each call creates independent ticket |
| Tenant has no system user | `systemTicketCreatorID` queries `user` table globally, not per-tenant — OK |
| EscalateTicket called with valid JWT in future | Currently ignores auth context — acceptable for agent-initiated escalation |

---

## 7. Adversarial Review Prompt

```
You are an adversarial code reviewer. Review this implementation plan for bugs/053
(Ticket Not Saved — EscalateTicket Missing CreatorID). Focus on:

1. CORRECTNESS: Is using `systemTicketCreatorID` the right fix for the agent
   escalation path? Should the handler extract the authenticated user from JWT
   instead and pass it to the service? What are the tradeoffs?

2. SECURITY: The `EscalateTicket` endpoint (POST /api/v1/agent/:slug/escalate)
   doesn't appear to require authentication. Is it a public endpoint? If so,
   using the system creator is correct. But if it requires auth, we should use
   the authenticated user's ID.

3. DEFENSIVE CODING: Is the zero-value check (CreatorID == 0) sufficient? Should
   we also validate that the creator user actually exists before INSERT?

4. COMPLETENESS: Are there other fields missing in EscalateTicket that are set
   in createEscalationTicket? (CreatedTs, UpdatedTs, Type are all missing.)

5. TESTING: The proposed test uses mocks. Is an integration test with the real
   SQLite DB needed to catch FK constraint violations?

Provide a severity rating (Critical/High/Medium/Low) for each finding and a
recommended fix or mitigation.
```

---

## 8. References

- `EscalateTicket`: `server/router/api/v1/agent/service.go:5525-5585`
- `createEscalationTicket`: `server/router/api/v1/agent/service.go:4725-4756`
- `createEscalationTicketFallback`: `server/router/api/v1/agent/service.go:4758-4818`
- `systemTicketCreatorID`: `server/router/api/v1/agent/service.go:4574`
- `handleAutoTicketCreation`: `server/router/api/v1/memo_service.go:1123-1183`
- REST `CreateTicket`: `server/router/api/v1/ticket_service.go:62-191`
- SQLite `CreateTicket`: `store/db/sqlite/ticket.go:12-55`
- Ticket schema: `store/migration/sqlite/LATEST.sql:152-173`
- Users in DB: `id=1 ibm2100 HOST`, `id=2 ate USER`, `id=3 ading USER`, `id=4 ading2 USER`
