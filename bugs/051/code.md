# Code: Internal Notes + RAG-Based Bug Inference Implementation

**Date:** 2026-07-30
**Status:** Implemented
**Bug:** 051

---

## 1. Implementation Summary

### What Was Built

Added an `internal_notes` field to tickets with RBAC-controlled visibility, an async bug import pipeline that reads 50 bug folders (001-050) into ~130 tickets, and a synchronous resolution inference system that searches CockroachDB vector index for similar past tickets when a new ticket is created.

### File Inventory

| File | Type | Lines | Purpose |
|------|------|-------|---------|
| `store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql` | NEW | 1 | SQLite migration: add `internal_notes` column |
| `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql` | NEW | 1 | Postgres migration: add `internal_notes` column |
| `store/migration/sqlite/LATEST.sql` | MODIFIED | +1 | Add `internal_notes` to tickets schema |
| `store/migration/postgres/LATEST.sql` | MODIFIED | +1 | Add `internal_notes` to tickets schema |
| `store/ticket.go` | MODIFIED | +2 | `InternalNotes` on `Ticket` and `UpdateTicket` structs |
| `store/db/sqlite/ticket.go` | MODIFIED | +12 | CRUD with `internal_notes` in INSERT, SELECT, UPDATE, RETURNING |
| `store/db/postgres/ticket.go` | MODIFIED | +12 | CRUD with `internal_notes` using `$N` placeholders |
| `server/router/api/v1/agent/permissions.go` | MODIFIED | +10 | `PermTicketInternalNotes` constant, `HasPermission()` helper, preset updates |
| `server/router/api/v1/ticket_service.go` | MODIFIED | +45 | RBAC filtering, `filterInternalNotes()`, inference trigger, `InternalNotes` on response/request structs |
| `web/src/pages/TicketDetail.tsx` | MODIFIED | +10 | Internal notes display section |
| `server/router/api/v1/agent/ticket_embedder.go` | MODIFIED | +1 | Include `internal_notes` in embedding content |
| `server/router/api/v1/agent/service.go` | MODIFIED | +40 | `InferResolutionForNewTicket()` with `vectorDBMu` lock |
| `cmd/import-bugs/main.go` | NEW | ~300 | Bug folder import script (SQLite + Postgres) |

---

## 2. Architecture

### Component Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    CreateTicket Handler                          │
│  ┌──────────────────┐    ┌──────────────────────────────────┐   │
│  │ Bind request     │───▶│ store.CreateTicket()             │   │
│  └──────────────────┘    └──────────┬───────────────────────┘   │
│                                     │                            │
│  ┌──────────────────┐    ┌──────────▼───────────────────────┐   │
│  │ go goroutine     │───▶│ InferResolutionForNewTicket()    │   │
│  │ (background)     │    │   ├─ vectorDB.Search()           │   │
│  └──────────────────┘    │   └─ store.UpdateTicket()        │   │
│                          └──────────────────────────────────┘   │
│  ┌──────────────────┐    ┌──────────────────────────────────┐   │
│  │ RBAC filter      │───▶│ filterInternalNotes()            │   │
│  │ (per-handler)    │    │   superuser/creator/assignee/perm│   │
│  └──────────────────┘    └──────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    Import Pipeline                               │
│  ┌──────────────────┐    ┌──────────────────────────────────┐   │
│  │ Phase 1          │───▶│ Read bugs/001-050 folders        │   │
│  │ (synchronous)    │    │ Parse .md files by phase type    │   │
│  │                  │    │ Create tickets (two-step:        │   │
│  │                  │    │   create + update internal_notes) │   │
│  └──────────────────┘    └──────────────────────────────────┘   │
│  ┌──────────────────┐    ┌──────────────────────────────────┐   │
│  │ Phase 2          │───▶│ Worker pool (5 goroutines)       │   │
│  │ (async, future)  │    │ Generate LLM summaries           │   │
│  │                  │    │ Fallback on failure              │   │
│  └──────────────────┘    └──────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### RBAC Visibility Rules

Internal notes are visible to:
1. **HOST/ADMIN** (superusers) — all tickets
2. **Ticket creator** — their own tickets
3. **Assigned users** (`assignee_id`) — tickets assigned to them
4. **Users with `ticket:internal_notes` permission** — all tickets in their tenant

All other users see `internalNotes: ""`.

```
User → ResolveEffectivePermissions() → []ResolvedPermission
  → HasPermission(perms, "ticket:internal_notes") → bool
  → filterInternalNotes(resp, ticket, user, hasPerm)
```

### Permission Check Mechanism

```go
// NOT CheckUserPermission (doesn't exist)
resolvedPerms, err := agent.ResolveEffectivePermissions(ctx, s.Store, tenantID, userID)
hasPerm := agent.HasPermission(resolvedPerms, agent.PermTicketInternalNotes)
```

### Resolution Inference Flow

```
New ticket created
  → go InferResolutionForNewTicket(ctx, ticket)
    → vectorDBMu.RLock()  // NB: protects against data race during reindex
    → vectorDB.Search(ctx, SearchQuery{
        TenantID:     *ticket.TenantID,
        QueryText:    ticket.Title + "\n" + ticket.Description,
        ContentTypes: []string{"ticket"},
        TopK:         5,
        MinScore:     0.7,
      })
    → Format suggested resolution from matching chunks
    → store.UpdateTicket() to set internal_notes
```

---

## 3. Database Schema

### Migration

**SQLite** (`store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql`):
```sql
ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';
```

**Postgres** (`store/migration/postgres/0.35/00__tickets_add_internal_notes.sql`):
```sql
ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';
```

**CockroachDB Compatibility:** `ALTER TABLE ... ADD COLUMN` is standard SQL. Works identically on SQLite, Postgres, and CockroachDB.

### LATEST.sql Updates

**SQLite** (`store/migration/sqlite/LATEST.sql`, line 171):
```sql
CREATE TABLE tickets (
   id INTEGER PRIMARY KEY AUTOINCREMENT,
   title TEXT NOT NULL,
   description TEXT NOT NULL DEFAULT '',
   status TEXT NOT NULL DEFAULT 'OPEN',
   priority TEXT NOT NULL DEFAULT 'MEDIUM',
   creator_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
   assignee_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
   created_ts BIGINT NOT NULL,
   updated_ts BIGINT NOT NULL,
   type TEXT NOT NULL DEFAULT 'TASK',
   tags TEXT NOT NULL DEFAULT '[]',
   beads_id TEXT UNIQUE,
   parent_id INTEGER REFERENCES tickets(id) ON DELETE CASCADE,
   labels TEXT DEFAULT '[]',
   dependencies TEXT DEFAULT '[]',
   discovery_context TEXT,
   closed_reason TEXT,
   issue_type TEXT,
   tenant_id INTEGER DEFAULT NULL,
   internal_notes TEXT DEFAULT ''  -- NEW
);
```

**Postgres** (`store/migration/postgres/LATEST.sql`, line 660):
```sql
CREATE TABLE tickets (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'OPEN',
    priority TEXT NOT NULL DEFAULT 'MEDIUM',
    creator_id INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    assignee_id INTEGER REFERENCES "user"(id) ON DELETE SET NULL,
    created_ts BIGINT NOT NULL,
    updated_ts BIGINT NOT NULL,
    type TEXT NOT NULL DEFAULT 'TASK',
    tags TEXT NOT NULL DEFAULT '[]',
    beads_id TEXT UNIQUE,
    parent_id INTEGER REFERENCES tickets(id) ON DELETE CASCADE,
    labels TEXT DEFAULT '[]',
    dependencies TEXT DEFAULT '[]',
    discovery_context TEXT,
    closed_reason TEXT,
    issue_type TEXT,
    tenant_id INTEGER DEFAULT NULL,
    internal_notes TEXT DEFAULT ''  -- NEW
);
```

### Store Types

```go
// store/ticket.go
type Ticket struct {
    // ... existing fields ...
    InternalNotes string  // NEW
}

type UpdateTicket struct {
    // ... existing fields ...
    InternalNotes *string  // NEW
}
```

### SQLite Driver Changes

**CreateTicket** — 12th `?` placeholder for `internal_notes`:
```go
INSERT INTO tickets (..., tenant_id, internal_notes)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id
```

**ListTickets** — `internal_notes` added to SELECT and Scan.

**UpdateTicket** — Dynamic SET clause:
```go
if update.InternalNotes != nil {
    set = append(set, "internal_notes = ?")
    args = append(args, *update.InternalNotes)
}
```

### Postgres Driver Changes

Same as SQLite but with `$N` parameter syntax:
```go
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
```

`tenant_id` stays at `$11`, `internal_notes` is `$12`.

---

## 4. Environment Variables

No new environment variables required. Uses existing:
- `COCKROACH_DSN` or `DATABASE_URL` or `MEMOS_DSN` for import script
- `SQLITE_PATH` for local SQLite import (defaults to `build/data/memos_dev.db`)
- `BUGS_DIR` for import script (defaults to `bugs`)

---

## 5. Build & Run Commands

```bash
# Build
go build ./bin/memos/main.go

# Build import script
go build ./cmd/import-bugs/

# Run tests
go test ./store/... -v
go test ./server/router/api/v1/agent/... -v

# Import bugs (SQLite local)
go run ./cmd/import-bugs/

# Import bugs (CockroachDB)
export COCKROACH_DSN="postgresql://..."
go run ./cmd/import-bugs/

# Import bugs (custom bugs directory)
BUGS_DIR=/path/to/bugs go run ./cmd/import-bugs/

# Validate schema
task validate:schema
task validate:parity
```

---

## 6. Testing Guide

### Build Verification

```bash
go build ./bin/memos/main.go                    # must compile
go build ./cmd/import-bugs/                     # must compile
go test ./store/... -count=1                    # must pass
go test ./server/router/api/v1/agent/... -count=1  # must pass
task validate:schema                            # must pass
task validate:parity                            # must pass
```

### Manual Verification

```bash
# 1. Start server (migration runs automatically)
task run

# 2. Import bugs
go run ./cmd/import-bugs/

# 3. Verify tickets created
sqlite3 build/data/memos_dev.db \
  "SELECT id, title, substr(internal_notes, 1, 50) FROM tickets WHERE tenant_id=1 LIMIT 10;"

# 4. Verify RBAC filtering
# Create ticket as customer → internal_notes empty
# View as admin → internal_notes populated

# 5. Verify resolution inference
# Create new ticket → check if internal_notes auto-populated
curl -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"Test ticket","description":"/m/test","status":"OPEN","priority":"MEDIUM","type":"TASK"}'
```

### Import Script Behavior

```bash
# First run: creates all tickets
go run ./cmd/import-bugs/
# Output: Created: ~130 tickets, Skipped: 0

# Second run: skips existing
go run ./cmd/import-bugs/
# Output: Created: 0, Skipped: ~130

# Interrupted run: re-run detects pending
# Tickets with internal_notes = "Pending summary..." are re-processed

# Manual cleanup if needed
sqlite3 build/data/memos_dev.db \
  "UPDATE tickets SET internal_notes = '' WHERE internal_notes = 'Pending summary...';"
```

---

## 7. Adversarial Code Review Prompt

Copy and paste this prompt into Claude/Gemini for a thorough code review:

---

**PROMPT:**

```
You are performing an adversarial code review of an Internal Notes + RAG-Based Bug Inference implementation for a multi-tenant AI chat agent platform (bchat). This is a hackathon submission for CockroachDB × AWS.

Review the following files against the codebase conventions, RBAC requirements, and CockroachDB best practices:

FILES TO REVIEW:
1. store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql (SQLite migration)
2. store/migration/postgres/0.35/00__tickets_add_internal_notes.sql (Postgres migration)
3. store/migration/sqlite/LATEST.sql (line 171: internal_notes column)
4. store/migration/postgres/LATEST.sql (line 660: internal_notes column)
5. store/ticket.go (Ticket and UpdateTicket structs)
6. store/db/sqlite/ticket.go (CreateTicket, ListTickets, UpdateTicket with internal_notes)
7. store/db/postgres/ticket.go (CreateTicket, ListTickets, UpdateTicket with internal_notes)
8. server/router/api/v1/agent/permissions.go (PermTicketInternalNotes, HasPermission, presets)
9. server/router/api/v1/ticket_service.go (RBAC filtering, filterInternalNotes, inference trigger)
10. web/src/pages/TicketDetail.tsx (internal notes display)
11. server/router/api/v1/agent/ticket_embedder.go (include internal_notes in embedding content)
12. server/router/api/v1/agent/service.go (InferResolutionForNewTicket with vectorDBMu lock)
13. cmd/import-bugs/main.go (bug folder import script)

CONSTRAINTS TO VERIFY:
- SQLite uses ? placeholders, Postgres uses $N placeholders
- convertTicketFromStore keeps single-arg signature (no API break)
- filterInternalNotes is called in each handler post-conversion (not in converter)
- ResolveEffectivePermissions + HasPermission used (NOT CheckUserPermission which doesn't exist)
- vectorDBMu.RLock/RUnlock wraps vectorDB access in InferResolutionForNewTicket
- vectorDB.Search() API used (NOT raw SQL against agent_vectors)
- Import script creates ticket then immediately UpdateTicket for internal_notes (two-step)
- Import script uses modernc.org/sqlite driver (not mattn/go-sqlite3)
- Tenant ID never exposed in error messages
- internal_notes defaults to "" in CreateTicket handler (not settable via API)

REVIEW CHECKLIST:

RBAC:
[C-1] CRITICAL: Does filterInternalNotes correctly check superuser, creator, assignee, AND permission?
[C-2] CRITICAL: Is ResolveEffectivePermissions called with correct tenantID and userID?
[C-3] CRITICAL: Does UpdateTicket handler only allow internal_notes update for superuser or ticket:internal_notes permission?
[C-4] CRITICAL: Is tenant_id enforced in vector search (InferResolutionForNewTicket)?

DATA INTEGRITY:
[H-1] HIGH: Does SQLite CreateTicket bind 12 parameters correctly?
[H-2] HIGH: Does Postgres CreateTicket use $11 for tenant_id and $12 for internal_notes?
[H-3] HIGH: Does ListTickets Scan include internal_notes in both SQLite and Postgres?
[H-4] HIGH: Does UpdateTicket RETURNING clause include internal_notes?

CONCURRENCY:
[H-5] HIGH: Is vectorDBMu.RLock used before accessing s.vectorDB in InferResolutionForNewTicket?
[H-6] HIGH: Does InferResolutionForNewTicket handle nil TenantID gracefully?

IMPORT SCRIPT:
[M-1] MEDIUM: Does import script use correct SQLite DSN with pragmas?
[M-2] MEDIUM: Does import script skip empty folders (bug 007)?
[M-3] MEDIUM: Does import script detect existing tickets and skip?
[M-4] MEDIUM: Does import script handle the two-step create+update correctly?

FRONTEND:
[N-1] NIT: Is internalNotes optional in the Ticket TypeScript interface?
[N-2] NIT: Is internal notes section only rendered when non-empty?
[N-3] NIT: Is the yellow background styling consistent with codebase?

GENERAL:
[N-4] NIT: Are error messages wrapped with context?
[N-5] NIT: Is slog usage consistent with codebase conventions?
[N-6] NIT: Is import ordering consistent?

INVARIANTS TO VERIFY:

1. INV_TICKET_INTERNAL_NOTES_RBAC
   Internal notes must be hidden from users who are not superusers, not the creator,
   not assigned, and don't have ticket:internal_notes permission.

2. INV_TICKET_INTERNAL_NOTES_PERSISTENCE
   Internal notes must survive CREATE → READ → UPDATE → READ cycles without data loss.

3. INV_VECTOR_SEARCH_TENANT_ISOLATION
   InferResolutionForNewTicket must only return results from the same tenant.
   Cross-tenant data must never leak through vector search.

4. INV_IMPORT_IDEMPOTENCY
   Running the import script twice must not create duplicate tickets.
   Existing tickets must be detected and skipped.

5. INV_RESOLUTION_INference_GRACEFUL_DEGRADATION
   If CockroachDB is unavailable or no similar tickets found, the new ticket
   must still be created successfully with empty or default internal_notes.

OUTPUT FORMAT:
For each finding, provide:
- File:line_number
- Severity: CRITICAL/HIGH/MEDIUM/NIT
- Description: What's wrong
- Fix: Exact code change

Also verify:
- All 13 files compile without errors
- Both store tests and agent tests pass
- task validate:schema passes
- task validate:parity passes
```

---

## 8. Known Limitations

| Issue | Severity | Mitigation |
|-------|----------|------------|
| Import interrupt leaves "Pending summary..." tickets | LOW | Re-run detects and re-processes; manual cleanup SQL documented |
| Internal notes included in vector embeddings | MEDIUM | Trade-off: richer search vs RBAC gap at vector level. Acceptable for hackathon. |
| No LLM summary generation in import script (Phase 2) | LOW | Phase 1 creates tickets with placeholder; Phase 2 can be added later |
| No test coverage for new code | MEDIUM | Unit tests for filterInternalNotes, migration, import parsing needed |
| `HasPermission` wrapper delegates to unexported `containsResolvedPermission` | LOW | Works; could export for cleaner API in future |
| Import script uses database/sql directly (not store layer) | LOW | Simpler; store layer not needed for one-shot import |

---

## 9. Rollback Plan

If internal notes feature causes issues:

1. **Database:** Column defaults to `""`, so no rollback needed. Existing tickets unaffected.
2. **RBAC:** If `ticket:internal_notes` permission causes issues, remove from `AllPermissions` and `PermissionPresets`. All users see `""` for internal notes.
3. **Inference:** If `InferResolutionForNewTicket` causes performance issues, remove the `go` goroutine trigger in `CreateTicket` handler.
4. **Import:** If import script causes issues, delete imported tickets:
   ```sql
   DELETE FROM tickets WHERE type = 'BUG' AND tags LIKE '%imported%';
   ```
5. **Frontend:** If internal notes display causes issues, remove the `{ticket.internalNotes && ...}` block from `TicketDetail.tsx`.
