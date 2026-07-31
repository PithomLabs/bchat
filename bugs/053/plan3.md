# Plan 3: Ticket Not Saved — EscalateTicket Missing CreatorID (Final)

**Bug ID:** 053
**Date:** 2026-07-31
**Status:** Draft — Incorporates plan2 review findings
**Supersedes:** `plan.md`, `plan2.md`

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

### Review Progression

| Review | Verdict | Key Additions |
|--------|---------|---------------|
| plan.md | REJECT | Identified missing CreatorID; missed route registration, Type field, auth model |
| plan2.md | APPROVED WITH NITS | Added route, Type, auth, defensive guard; missed port, permission check, redundant lookup, priority validation |
| plan3.md | FINAL | Incorporates all findings from both reviews |

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

Both `createEscalationTicket` and `createEscalationTicketFallback` set `Type: "agent_escalation"`. `EscalateTicket` omits it — defaults to `'TASK'` from schema.

**Defect 3 — Route never registered:**

`HandleEscalateTicket` (handlers.go) is defined but **never mounted** in `RegisterAgentRoutes` (v1.go). The endpoint returns 404.

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

Add inside `RegisterAgentRoutes()`, in the `authGroup` section (after the learning routes, around line 371):

```go
// Escalation route (requires auth + tenant binding)
authGroup.POST("/:slug/escalate", s.agentHandler.HandleEscalateTicket)
```

**Why `authGroup`:**
- Requires JWT authentication (prevents anonymous spam)
- Has `TenantBindingMiddleware` (ensures user has access to the tenant)
- Has `adminCORS` (restrictive CORS)
- Matches pattern of other sensitive agent operations (simulate, chat/int, etc.)

### Step 2: Update Handler with Auth, Permission Check, and Priority Validation

**File:** `server/router/api/v1/agent/handlers.go`

Replace `HandleEscalateTicket` with:

```go
func (h *Handler) HandleEscalateTicket(c echo.Context) error {
    ctx := c.Request().Context()

    // Extract authenticated user from JWT context
    userID := h.getUserID(c)
    if userID == 0 {
        return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
    }

    // Get tenant from context (set by TenantBindingMiddleware)
    tenant, err := getTenantOrFail(ctx, h.store, c)
    if err != nil {
        return err
    }

    // Permission check: admin or chat:test permission required
    if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
        return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:test permission")
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

    // Validate priority if provided
    if req.Priority != "" && req.Priority != "high" && req.Priority != "medium" && req.Priority != "low" {
        return echo.NewHTTPError(http.StatusBadRequest, "priority must be high, medium, or low")
    }

    // Call escalation service with tenant ID (avoids redundant DB lookup)
    resp, err := h.service.EscalateTicket(ctx, tenant.ID, req, userID)
    if err != nil {
        slog.Error("ticket escalation failed", "slug", tenant.Slug, "error", err)
        return echo.NewHTTPError(http.StatusInternalServerError, "Escalation service unavailable")
    }

    return c.JSON(http.StatusOK, resp)
}
```

**Changes from plan2:**
- Uses `h.getUserID(c)` instead of raw `c.Get("user-id")` — follows agent package pattern (handlers.go:2365)
- Adds `hasPermission(c, tenant.ID, PermChatTest)` check — matches HandleChatInternal, HandleStartSimulation
- Adds priority validation — returns 400 for invalid values
- Passes `tenant.ID` instead of `tenant.Slug` — eliminates redundant DB lookup in service

### Step 3: Update Service Signature and Fix Ticket Construction

**File:** `server/router/api/v1/agent/service.go`

Update `EscalateTicket` signature and body:

```go
// Before:
func (s *Service) EscalateTicket(ctx context.Context, tenantSlug string, req EscalateTicketRequest) (*EscalateTicketResponse, error) {
    // Get tenant by slug
    tenant, err := s.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &tenantSlug})
    if err != nil || tenant == nil {
        return nil, fmt.Errorf("tenant not found: %s", tenantSlug)
    }
    ...
    ticket := &store.Ticket{
        Title:       req.Title,
        Description: description,
        Status:      store.TicketStatusOpen,
        Priority:    priority,
        TenantID:    &tenant.ID,
        Tags:        req.Tags,
    }
    ...

// After:
func (s *Service) EscalateTicket(ctx context.Context, tenantID int32, req EscalateTicketRequest, creatorID int32) (*EscalateTicketResponse, error) {
    // tenantID passed directly from handler — no DB lookup needed

    // Create ticket in database
    priority := store.TicketPriorityMedium
    if req.Priority == "high" {
        priority = store.TicketPriorityHigh
    } else if req.Priority == "low" {
        priority = store.TicketPriorityLow
    }

    // Build description with memo link prefix if not already present
    description := req.Description
    if len(description) < 3 || description[:3] != "/m/" {
        description = "/m/" + description
    }

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
        TenantID:    &tenantID,
        Tags:        req.Tags,
    }

    // Defensive guard
    if ticket.CreatorID == 0 || ticket.CreatedTs == 0 || ticket.UpdatedTs == 0 || ticket.Type == "" {
        slog.Error("EscalateTicket: ticket construction incomplete",
            "title", req.Title, "tenant_id", tenantID,
            "creator_id", ticket.CreatorID, "created_ts", ticket.CreatedTs, "type", ticket.Type)
        return nil, fmt.Errorf("failed to construct ticket: internal error")
    }

    createdTicket, err := s.store.CreateTicket(ctx, ticket)
    if err != nil {
        return nil, fmt.Errorf("failed to create ticket: %w", err)
    }

    // Search for similar tickets via vector DB
    s.vectorDBMu.RLock()
    vectorDB := s.vectorDB
    s.vectorDBMu.RUnlock()

    similarCount := 0
    if vectorDB != nil {
        query := fmt.Sprintf("%s %s", req.Title, req.Description)
        result, err := vectorDB.Search(ctx, SearchQuery{
            QueryText:    query,
            TenantID:     tenantID,
            ContentTypes: []string{"ticket"},
            TopK:         5,
            MinScore:     0.5,
        })
        if err == nil {
            similarCount = result.Total
        }
    }

    return &EscalateTicketResponse{
        TicketID:     createdTicket.ID,
        Status:       "created",
        SimilarCount: similarCount,
    }, nil
}
```

**Key changes from plan2:**
- Signature: `tenantSlug string` → `tenantID int32`, added `creatorID int32`
- Removed `GetAgentTenant` call — tenant already resolved by middleware + handler
- Added `CreatedTs`, `UpdatedTs`, `Type` fields
- Added defensive guard checking all 4 required fields
- Uses `tenantID` directly for vector DB query

### Step 4: Verify No Other Callers

Confirmed via grep: only one caller — `HandleEscalateTicket` in `handlers.go`. No other code calls `s.EscalateTicket()`.

---

## 4. Files Modified

| File | Step | Action | Description |
|------|------|--------|-------------|
| `server/router/api/v1/v1.go` | 1 | MODIFY | Register `POST /:slug/escalate` in `authGroup` |
| `server/router/api/v1/agent/handlers.go` | 2 | MODIFY | Rewrite `HandleEscalateTicket`: extract user via `h.getUserID(c)`, add permission check, validate priority, pass `tenant.ID` |
| `server/router/api/v1/agent/service.go` | 3 | MODIFY | Change `EscalateTicket` signature (`tenantSlug` → `tenantID`, add `creatorID`), remove internal `GetAgentTenant`, set all 4 missing fields, add defensive guard |

---

## 5. Behavior Change

After this fix, `tickets.creator_id` for agent escalations will reflect the **authenticated user** who submitted the escalation, not the system user (`agent_system`). This provides a correct audit trail but changes the historical pattern. If backward compatibility is required, a config flag can be added later.

---

## 6. Verification Plan

| Step | Command | Expected |
|------|---------|----------|
| Build | `task build:backend` | Compiles |
| Build (RAG) | `task build:backend:rag` | Compiles |
| Run | `task run:rag` | Server starts on port 8081, no errors |
| Verify route exists (no auth) | `curl http://localhost:8081/api/v1/agent/hackathon-demo/escalate` | HTTP 401 (route mounted, auth required) |
| Verify 403 without permission | `curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/escalate -H "Authorization: Bearer <token_without_chat_test>" -d '{"title":"Test","description":"test"}'` | HTTP 403 |
| Authenticated escalation | `curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/escalate -H "Authorization: Bearer <admin_token>" -d '{"title":"Test ticket","description":"test issue","priority":"high"}'` | HTTP 200 with `ticket_id` |
| Verify in DB | `sqlite3 build/data/memos_dev.db "SELECT id, title, creator_id, type, created_ts, datetime(created_ts, 'unixepoch', 'localtime') FROM tickets ORDER BY id DESC LIMIT 1"` | `creator_id` = auth user, `type=agent_escalation`, current timestamp |
| Verify priority | `sqlite3 build/data/memos_dev.db "SELECT priority FROM tickets ORDER BY id DESC LIMIT 1"` | `HIGH` |
| Verify invalid priority rejected | `curl -X POST ... -d '{"title":"T","description":"D","priority":"urgent"}'` | HTTP 400 |
| Verify 401 without auth | `curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/escalate -d '{"title":"Test","description":"test"}'` | HTTP 401 |

---

## 7. Edge Cases

| Case | Behavior |
|------|----------|
| No auth token | HTTP 401 — "Authentication required" |
| Invalid/expired JWT | HTTP 401 — auth middleware rejects |
| User not authorized for tenant | `TenantBindingMiddleware` returns 403 |
| User has no `chat:test` permission | Handler returns 403 — "Permission denied" |
| Invalid priority value (e.g., "urgent") | Handler returns 400 — "priority must be high, medium, or low" |
| `CreatorID` somehow 0 after auth | Defensive guard catches it — returns internal error |
| Multiple rapid escalations | No race — each creates independent ticket |
| Description without `/m/` prefix | Service prepends `/m/` automatically |
| Vector DB unavailable | `similarCount` stays 0, ticket still created |

---

## 8. Out of Scope

| Item | Reason | Track Separately |
|------|--------|-----------------|
| `systemTicketCreatorID` side effects | Pre-existing behavior used by other paths | Yes — separate bug |
| `Ticket.Validate()` completeness | Only checks 4 fields; should check CreatorID, Type, timestamps | Yes — separate bug |
| Cross-package `getUserID` consistency | Agent package uses `h.getUserID(c)`, v1 package uses `getUserIDContextKey()` — different packages, pre-existing | Yes — separate refactor |

---

## 9. Adversarial Review Prompt

```
You are an adversarial code reviewer. Review this final implementation plan
for bugs/053 (Ticket Not Saved — EscalateTicket Missing CreatorID). Focus on:

1. CORRECTNESS: The handler calls `getTenantOrFail` which does a DB lookup
   by tenant ID from context. Then it passes `tenant.ID` to the service. Is
   there any scenario where `tenant.ID` from the handler differs from what
   the middleware set? Could a race condition or context manipulation cause
   a mismatch?

2. SECURITY: The permission check uses `PermChatTest`. Is this the right
   permission for escalation? Escalation creates tickets — should it use
   a dedicated permission like `ticket:create` instead? Or is `chat:test`
   appropriate since escalation is a chat-side action?

3. CONCURRENCY: If two authenticated users escalate simultaneously for the
   same tenant, could the vector DB search and ticket creation race? The
   ticket creation is serialized by SQLite, but the vector DB insert is
   async. Any consistency concerns?

4. ERROR HANDLING: The handler returns generic "Escalation service unavailable"
   for all service errors. Should we surface specific error messages (e.g.,
   "ticket creation failed" vs "vector search failed") to aid debugging?

5. TESTING: The verification plan requires manual curl commands. Should we
   add an automated test (unit or integration) to prevent regression?

Provide a severity rating (Critical/High/Medium/Low) for each finding and a
recommended fix or mitigation.
```

---

## 10. References

- `HandleEscalateTicket`: `handlers.go` in `server/router/api/v1/agent/`
- `EscalateTicket`: `service.go` in `server/router/api/v1/agent/`
- `createEscalationTicket`: `service.go` in `server/router/api/v1/agent/`
- `createEscalationTicketFallback`: `service.go` in `server/router/api/v1/agent/`
- `getUserID`: `handlers.go:2365` in `server/router/api/v1/agent/`
- `hasPermission`: `handlers.go:2375` in `server/router/api/v1/agent/`
- `isAdmin`: `handlers.go:2295` in `server/router/api/v1/agent/`
- `getTenantOrFail`: `tenant_helpers.go:36` in `server/router/api/v1/agent/`
- `PermChatTest`: `permissions.go:15` in `server/router/api/v1/agent/`
- `RegisterAgentRoutes`: `v1.go:258` in `server/router/api/v1/`
- `authGroup` definition: `v1.go:323` in `server/router/api/v1/`
- `TenantBindingMiddleware`: `tenant_binding.go:16` in `server/router/api/v1/`
- Default port: `main.go:210` — `viper.SetDefault("port", 8081)`
- `Ticket.Validate()`: `store/ticket.go:67`
- Ticket schema: `store/migration/sqlite/LATEST.sql:152-173`
- Users: `id=1 ibm2100 HOST`, `id=2 ate USER`, `id=3 ading USER`, `id=4 ading2 USER`
