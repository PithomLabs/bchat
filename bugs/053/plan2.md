# Plan 2: Ticket Not Saved — EscalateTicket Missing CreatorID (Revised)

**Bug ID:** 053
**Date:** 2026-07-31
**Status:** Draft — Incorporates adversarial review findings
**Supersedes:** `plan.md`

---

## 1. Background

### User Prompt

> I just created a ticket when I run task run:rag but it was not saved in sqlite db, double check where it was stored

### Investigation Summary

Ran `task run:rag` (SQLite-backed dev server) and attempted to create a ticket. The ticket did not appear in the `tickets` table in `build/data/memos_dev.db`.

| Location | Result |
|----------|--------|
| SQLite DB (`build/data/memos_dev.db`) | 65 tickets, all from 2026-07-30 (seeded), **zero from today** |
| CockroachDB (`bchat-crdb` Docker) | Only `agent_vectors` table — **no tickets table** |
| PostgreSQL (`bchat-postgres` Docker) | Running but not configured |
| `.env` file | No `DATABASE_URL`, `MEMOS_DSN` commented out — defaults to SQLite |
| WAL file | Flushed — no hidden data |

**Conclusion:** The ticket was **never saved to any database**.

### Adversarial Review Findings

The initial plan correctly identified the missing `CreatorID` but had critical gaps:

| # | Finding | Severity | Status |
|---|---------|----------|--------|
| 1 | Route never registered in v1.go | CRITICAL | New — added |
| 2 | Missing `Type: "agent_escalation"` field | HIGH | New — added |
| 3 | Authentication model undefined | HIGH | New — added |
| 4 | Defensive zero-value check incomplete | MEDIUM | Revised |
| 5 | `systemTicketCreatorID` side effects | MEDIUM | Out of scope — noted |
| 6 | Testing gap (depends on Finding 1) | HIGH | Revised |
| 7 | Line number drift in audit table | LOW | Revised |

---

## 2. Root Cause Analysis

### The Bug (Three Defects)

**Defect 1 — Missing CreatorID (FK constraint violation):**

`EscalateTicket` in `service.go` constructs a `store.Ticket` without `CreatorID`:

```go
ticket := &store.Ticket{
    Title:       req.Title,
    Description: description,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    TenantID:    &tenant.ID,
    Tags:        req.Tags,
    // CreatorID = 0 (zero value)
}
```

Schema enforces `creator_id INTEGER NOT NULL REFERENCES user(id)`. No user with `id = 0` exists (users are 1–4). INSERT fails with FK constraint violation.

**Defect 2 — Missing Type field:**

Both `createEscalationTicket` (service.go) and `createEscalationTicketFallback` (service.go) set `Type: "agent_escalation"`. `EscalateTicket` omits it — defaults to `'TASK'` from schema. Downstream filtering `WHERE type = 'agent_escalation'` misses these tickets.

**Defect 3 — Route never registered:**

`HandleEscalateTicket` (handlers.go) is defined but **never mounted** in `RegisterAgentRoutes` (v1.go). No route group — `publicGroup`, `authGroup`, or `bridgeGroup` — contains `/:slug/escalate`. The endpoint returns 404 regardless of the service fix.

### Why Other Paths Work

| Path | CreatorID | Type | Route | Works? |
|------|-----------|------|-------|--------|
| `EscalateTicket` | **NONE** | **MISSING** | **NOT REGISTERED** | **NO** |
| `createEscalationTicket` | `systemTicketCreatorID(ctx)` | `"agent_escalation"` | Internal (service call) | Yes |
| `createEscalationTicketFallback` | `systemTicketCreatorID(ctx)` | `"agent_escalation"` | Internal (service call) | Yes |
| `handleAutoTicketCreation` | `user.ID` | Derived from content tags | Internal (memo creation) | Yes |
| REST `CreateTicket` | `userID` from JWT | User-provided or `"TASK"` | `ticketGroup.POST("/tickets", ...)` | Yes |

---

## 3. Implementation Plan

### Step 1: Register Route in v1.go

**File:** `server/router/api/v1/v1.go`

Add inside `RegisterAgentRoutes()`, in the `authGroup` section (after the existing `authGroup` routes, around line 371):

```go
// Escalation route (requires auth + tenant binding)
authGroup.POST("/:slug/escalate", s.agentHandler.HandleEscalateTicket)
```

**Why `authGroup`:**
- Requires JWT authentication (prevents anonymous spam)
- Has `TenantBindingMiddleware` (ensures user has access to the tenant)
- Has `adminCORS` (restrictive CORS)
- Matches pattern of other sensitive agent operations (simulate, chat/int, etc.)

**Impact:** After this step, `POST /api/v1/agent/:slug/escalate` becomes reachable with auth.

### Step 2: Extract Authenticated User in Handler

**File:** `server/router/api/v1/agent/handlers.go`

Update `HandleEscalateTicket` to extract the authenticated user from JWT context, following the pattern used by `HandleChatInternal` (handlers.go:614):

```go
func (h *Handler) HandleEscalateTicket(c echo.Context) error {
    ctx := c.Request().Context()

    // Extract authenticated user from JWT context
    userID, ok := c.Get("user-id").(int32)
    if !ok {
        return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
    }

    // Get tenant from context (set by TenantBindingMiddleware)
    tenant, err := getTenantOrFail(ctx, h.store, c)
    if err != nil {
        return err
    }

    // Bind request
    var req EscalateTicketRequest
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
    }

    if req.Title == "" {
        return echo.NewHTTPError(http.StatusBadRequest, "title is required")
    }
    if req.Description == "" {
        return echo.NewHTTPError(http.StatusBadRequest, "description is required")
    }
    if len(req.Description) > 10000 {
        return echo.NewHTTPError(http.StatusBadRequest, "description must be under 10000 characters")
    }

    // Call escalation service with authenticated user ID
    resp, err := h.service.EscalateTicket(ctx, tenant.Slug, req, userID)
    if err != nil {
        slog.Error("ticket escalation failed", "slug", tenant.Slug, "error", err)
        return echo.NewHTTPError(http.StatusInternalServerError, "Escalation service unavailable")
    }

    return c.JSON(http.StatusOK, resp)
}
```

### Step 3: Update Service Signature to Accept CreatorID

**File:** `server/router/api/v1/agent/service.go`

Update `EscalateTicket` signature and ticket construction:

```go
// Before:
func (s *Service) EscalateTicket(ctx context.Context, tenantSlug string, req EscalateTicketRequest) (*EscalateTicketResponse, error) {

// After:
func (s *Service) EscalateTicket(ctx context.Context, tenantSlug string, req EscalateTicketRequest, creatorID int32) (*EscalateTicketResponse, error) {
```

Update ticket construction:

```go
// Before:
ticket := &store.Ticket{
    Title:       req.Title,
    Description: description,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    TenantID:    &tenant.ID,
    Tags:        req.Tags,
}

// After:
now := time.Now().Unix()
ticket := &store.Ticket{
    Title:       req.Title,
    Description: description,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    CreatorID:   creatorID,
    CreatedTs:   now,
    UpdatedTs:   now,
    Type:        "agent_escalation",
    TenantID:    &tenant.ID,
    Tags:        req.Tags,
}
```

### Step 4: Add Defensive Guard

**File:** `server/router/api/v1/agent/service.go`

After ticket construction, before `CreateTicket` call, validate all required fields:

```go
if ticket.CreatorID == 0 || ticket.CreatedTs == 0 || ticket.UpdatedTs == 0 || ticket.Type == "" {
    slog.Error("EscalateTicket: ticket construction incomplete",
        "title", req.Title, "tenant_id", tenant.ID,
        "creator_id", ticket.CreatorID, "created_ts", ticket.CreatedTs, "type", ticket.Type)
    return nil, fmt.Errorf("failed to construct ticket: internal error")
}
```

**Note on `Ticket.Validate()`:** The existing `store.Ticket.Validate()` (store/ticket.go:67) only checks `Title`, `Status`, `Priority`, and `Description`. It does **not** check `CreatorID`, `CreatedTs`, `UpdatedTs`, or `Type`. The explicit guard is necessary.

### Step 5: Verify No Other Callers

Confirmed via grep: only one caller exists — `HandleEscalateTicket` in `handlers.go`. The lower-level functions `createEscalationTicket` and `createEscalationTicketFallback` are separate functions that already set `CreatorID`, `CreatedTs`, `UpdatedTs`, and `Type` correctly. No changes needed to those functions.

---

## 4. Files Modified

| File | Step | Action | Description |
|------|------|--------|-------------|
| `server/router/api/v1/v1.go` | 1 | MODIFY | Register `POST /:slug/escalate` in `authGroup` |
| `server/router/api/v1/agent/handlers.go` | 2 | MODIFY | Extract `user-id` from JWT, pass `userID` to service |
| `server/router/api/v1/agent/service.go` | 3, 4 | MODIFY | Update `EscalateTicket` signature (add `creatorID` param), set all 4 missing fields, add defensive guard |

---

## 5. Verification Plan

| Step | Command | Expected |
|------|---------|----------|
| Build | `task build:backend` | Compiles |
| Build (RAG) | `task build:backend:rag` | Compiles |
| Run | `task run:rag` | Server starts, no errors |
| Verify route exists | `curl http://localhost:5230/api/v1/agent/hackathon-demo/escalate` | Not 404 (will get 401 without auth, confirming route is mounted) |
| Authenticated escalation | `curl -X POST http://localhost:5230/api/v1/agent/hackathon-demo/escalate -H "Authorization: Bearer <token>" -d '{"title":"Test ticket","description":"test issue"}'` | HTTP 200 with `ticket_id` |
| Verify in DB | `sqlite3 build/data/memos_dev.db "SELECT id, title, creator_id, type, created_ts, datetime(created_ts, 'unixepoch', 'localtime') FROM tickets ORDER BY id DESC LIMIT 1"` | `creator_id=1`, `type=agent_escalation`, current timestamp |
| Verify creator user | `sqlite3 build/data/memos_dev.db "SELECT u.username FROM tickets t JOIN user u ON t.creator_id = u.id ORDER BY t.id DESC LIMIT 1"` | `ibm2100` |
| Verify 401 without auth | `curl -X POST http://localhost:5230/api/v1/agent/hackathon-demo/escalate -d '{"title":"Test","description":"test"}'` | HTTP 401 |

---

## 6. Edge Cases

| Case | Behavior |
|------|----------|
| No auth token provided | HTTP 401 — "Authentication required" |
| Invalid/expired JWT | HTTP 401 — auth middleware rejects |
| User not authorized for tenant | `TenantBindingMiddleware` returns 403 |
| `systemTicketCreatorID` fails (no users in DB) | Would return 0, but guard catches it — returns internal error |
| Multiple rapid escalations | No race — each creates independent ticket |
| Description without `/m/` prefix | Handler passes raw description; service prepends `/m/` automatically |

---

## 7. Adversarial Review Prompt

```
You are an adversarial code reviewer. Review this revised implementation plan
for bugs/053 (Ticket Not Saved — EscalateTicket Missing CreatorID). Focus on:

1. CORRECTNESS: The plan registers the route under `authGroup` which has
   `TenantBindingMiddleware`. Does this middleware conflict with
   `ResolveSlugTenantMiddleware` already used in `publicGroup`? The auth group
   also calls `getTenantOrFail` for tenant lookup — will the tenant be set
   correctly since it goes through `TenantBindingMiddleware` instead of
   `ResolveSlugTenantMiddleware`?

2. SECURITY: After this fix, authenticated users can create escalation tickets
   for any tenant they have access to. Should we add a permission check (e.g.,
   `chat:test` permission) before allowing escalation? Or is tenant binding sufficient?

3. COMPLETENESS: The handler now passes `userID` to the service. But the service
   also calls `getAgentTenant` for tenant lookup. Is there a redundant tenant
   lookup? The handler already resolves the tenant — should it pass the tenant
   object instead of the slug?

4. DEFENSIVE CODING: The guard returns an error if any field is zero-value.
   Is returning an error the right behavior, or should it attempt recovery
   (e.g., falling back to system creator)?

5. TESTING: The verification plan requires a valid JWT token. Should we add a
   unit test with mock store to validate the ticket construction without
   needing a running server?

Provide a severity rating (Critical/High/Medium/Low) for each finding and a
recommended fix or mitigation.
```

---

## 8. Out of Scope

| Item | Reason | Track Separately |
|------|--------|-----------------|
| `systemTicketCreatorID` side effects | Pre-existing behavior used by other paths | Yes — separate bug |
| `Ticket.Validate()` completeness | Only checks 4 fields; should check CreatorID, Type, timestamps | Yes — separate bug |
| Line number references in documentation | Low priority, use function names instead | No action needed |

---

## 9. References

- `HandleEscalateTicket`: `handlers.go` in `server/router/api/v1/agent/`
- `EscalateTicket`: `service.go` in `server/router/api/v1/agent/`
- `createEscalationTicket`: `service.go` in `server/router/api/v1/agent/`
- `createEscalationTicketFallback`: `service.go` in `server/router/api/v1/agent/`
- `systemTicketCreatorID`: `service.go` in `server/router/api/v1/agent/`
- `RegisterAgentRoutes`: `v1.go` in `server/router/api/v1/`
- `authGroup` definition: `v1.go` in `server/router/api/v1/`
- `TenantBindingMiddleware`: `tenant_binding.go` in `server/router/api/v1/`
- `HandleChatInternal` (auth pattern): `handlers.go` in `server/router/api/v1/agent/`
- `Ticket.Validate()`: `store/ticket.go`
- Ticket schema: `store/migration/sqlite/LATEST.sql:152-173`
- Users: `id=1 ibm2100 HOST`, `id=2 ate USER`, `id=3 ading USER`, `id=4 ading2 USER`
