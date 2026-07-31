# Code Documentation: Bug 053 — EscalateTicket Missing CreatorID

**Bug ID:** 053
**Date:** 2026-07-31
**Status:** Implemented

---

## Problem Statement

When creating a ticket via `POST /api/v1/agent/:slug/escalate`, the ticket was not saved to the SQLite database. The endpoint returned HTTP 500 (`"Escalation service unavailable"`) because the `INSERT INTO tickets` failed with a foreign key constraint violation.

## Root Cause

Three defects in `EscalateTicket`:

1. **Missing `CreatorID`** — `store.Ticket.CreatorID` defaulted to 0 (Go zero value). The `tickets` table enforces `creator_id INTEGER NOT NULL REFERENCES user(id)`. No user with `id = 0` exists (users are 1–4). INSERT fails.

2. **Missing `Type` field** — `store.Ticket.Type` defaulted to `'TASK'` from schema. Other escalation paths (`createEscalationTicket`, `createEscalationTicketFallback`) set `Type: "agent_escalation"`. Downstream filtering `WHERE type = 'agent_escalation'` missed these tickets.

3. **Route never registered** — `HandleEscalateTicket` was defined in `handlers.go` but never mounted in `RegisterAgentRoutes` (`v1.go`). The endpoint returned 404 regardless of the service fix.

## Files Modified

| File | Lines Changed | Description |
|------|---------------|-------------|
| `server/router/api/v1/v1.go` | +3 (line 373) | Register route in `authGroup` |
| `server/router/api/v1/agent/handlers.go` | ~50 (lines 464–515) | Rewrite handler with auth, permission check, priority validation |
| `server/router/api/v1/agent/service.go` | ~70 (lines 5524–5593) | Fix signature, set missing fields, add defensive guard |

---

## Change 1: Route Registration

**File:** `server/router/api/v1/v1.go`

### Before

No route existed for `/:slug/escalate`. `HandleEscalateTicket` was unreachable.

### After

```go
// Escalation route (requires auth + tenant binding)
authGroup.POST("/:slug/escalate", s.agentHandler.HandleEscalateTicket)
```

Inserted in `RegisterAgentRoutes()` at line 373, after the learning routes and before the user tenants section.

### Why `authGroup`

| Middleware | Purpose |
|-----------|---------|
| `s.AuthMiddleware` | Requires valid JWT — prevents anonymous spam |
| `TenantBindingMiddleware` | Ensures user has access to the target tenant |
| `adminCORS` | Restrictive CORS policy |

Other sensitive agent endpoints (`HandleChatInternal`, `HandleStartSimulation`) also use `authGroup`.

---

## Change 2: Handler Rewrite

**File:** `server/router/api/v1/agent/handlers.go`

### Before

```go
func (h *Handler) HandleEscalateTicket(c echo.Context) error {
    ctx := c.Request().Context()
    tenant, err := getTenantOrFail(ctx, h.store, c)
    if err != nil { return err }
    // ... bind, validate ...
    resp, err := h.service.EscalateTicket(ctx, tenant.Slug, req)
    // ...
}
```

Problems: no auth extraction, no permission check, passes `tenant.Slug` (forces redundant DB lookup in service).

### After

```go
func (h *Handler) HandleEscalateTicket(c echo.Context) error {
    ctx := c.Request().Context()

    // 1. Extract authenticated user from JWT context
    userID := h.getUserID(c)
    if userID == 0 {
        return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
    }

    // 2. Get tenant from context (set by TenantBindingMiddleware)
    tenant, err := getTenantOrFail(ctx, h.store, c)
    if err != nil { return err }

    // 3. Permission check: admin or chat:test permission required
    if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
        return echo.NewHTTPError(http.StatusForbidden,
            "Permission denied: requires admin role or chat:test permission")
    }

    // 4. Bind + validate request
    var req EscalateTicketRequest
    if err := c.Bind(&req); err != nil { ... }
    if req.Title == "" { ... }
    if req.Description == "" { ... }
    if len(req.Description) > 10000 { ... }

    // 5. Validate priority
    if req.Priority != "" && req.Priority != "high" &&
       req.Priority != "medium" && req.Priority != "low" {
        return echo.NewHTTPError(http.StatusBadRequest,
            "priority must be high, medium, or low")
    }

    // 6. Call service with tenant.ID (not tenant.Slug)
    resp, err := h.service.EscalateTicket(ctx, tenant.ID, req, userID)
    if err != nil { ... }
    return c.JSON(http.StatusOK, resp)
}
```

### Key Differences

| Aspect | Before | After |
|--------|--------|-------|
| Auth extraction | None | `h.getUserID(c)` — returns 0 if unauthenticated |
| Permission check | None | `h.isAdmin(c) \|\| h.hasPermission(c, tenant.ID, PermChatTest)` |
| Priority validation | None (silently defaulted to MEDIUM) | Returns 400 for invalid values |
| Service call | `EscalateTicket(ctx, tenant.Slug, req)` | `EscalateTicket(ctx, tenant.ID, req, userID)` |

### Helper Functions Used

- **`h.getUserID(c)`** — agent package helper at `handlers.go:2365`. Extracts `c.Get("user-id").(int32)`, returns 0 if not present.
- **`h.isAdmin(c)`** — agent package helper at `handlers.go:2295`. Checks if user has HOST or ADMIN role.
- **`h.hasPermission(c, tenantID, permission)`** — agent package helper at `handlers.go:2375`. Checks RBAC permission via `ResolveEffectivePermissions`.
- **`getTenantOrFail(ctx, store, c)`** — agent package helper at `tenant_helpers.go:36`. Reads `tenantID` from context (set by middleware), then calls `GetAgentTenant(ctx, &FindAgentTenant{ID: &tenantID})`.

---

## Change 3: Service Signature + Ticket Construction

**File:** `server/router/api/v1/agent/service.go`

### Before

```go
func (s *Service) EscalateTicket(ctx context.Context, tenantSlug string,
    req EscalateTicketRequest) (*EscalateTicketResponse, error) {

    tenant, err := s.store.GetAgentTenant(ctx,
        &store.FindAgentTenant{Slug: &tenantSlug})
    if err != nil || tenant == nil {
        return nil, fmt.Errorf("tenant not found: %s", tenantSlug)
    }
    // ...
    ticket := &store.Ticket{
        Title:       req.Title,
        Description: description,
        Status:      store.TicketStatusOpen,
        Priority:    priority,
        TenantID:    &tenant.ID,
        Tags:        req.Tags,
        // CreatorID = 0 (BUG)
        // CreatedTs = 0 (BUG)
        // UpdatedTs = 0 (BUG)
        // Type = "" → defaults to 'TASK' (BUG)
    }
}
```

### After

```go
func (s *Service) EscalateTicket(ctx context.Context, tenantID int32,
    req EscalateTicketRequest, creatorID int32) (*EscalateTicketResponse, error) {
    // No GetAgentTenant call — tenantID passed directly from handler

    priority := store.TicketPriorityMedium
    if req.Priority == "high" {
        priority = store.TicketPriorityHigh
    } else if req.Priority == "low" {
        priority = store.TicketPriorityLow
    }

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
        CreatorID:   creatorID,       // FIX: authenticated user ID
        CreatedTs:   now,             // FIX: current timestamp
        UpdatedTs:   now,             // FIX: current timestamp
        Type:        "agent_escalation", // FIX: matches other escalation paths
        TenantID:    &tenantID,       // FIX: direct ID, no pointer to tenant struct
        Tags:        req.Tags,
    }

    // Defensive guard
    if ticket.CreatorID == 0 || ticket.CreatedTs == 0 ||
       ticket.UpdatedTs == 0 || ticket.Type == "" {
        slog.Error("EscalateTicket: ticket construction incomplete",
            "title", req.Title, "tenant_id", tenantID,
            "creator_id", ticket.CreatorID,
            "created_ts", ticket.CreatedTs,
            "type", ticket.Type)
        return nil, fmt.Errorf("failed to construct ticket: internal error")
    }

    createdTicket, err := s.store.CreateTicket(ctx, ticket)
    // ... vector DB search ...
}
```

### Signature Change

| Parameter | Before | After | Reason |
|-----------|--------|-------|--------|
| Tenant identifier | `tenantSlug string` | `tenantID int32` | Eliminates redundant `GetAgentTenant` DB lookup |
| Creator | (none) | `creatorID int32` | Passed from handler's JWT extraction |

### Tenant Lookup Elimination

Before this fix, one escalation request triggered 3 DB queries:

1. `TenantBindingMiddleware` → `GetAgentTenant(slug)` → sets `tenantID` in context
2. `getTenantOrFail` in handler → `GetAgentTenant(ID)` → returns full struct
3. `EscalateTicket` in service → `GetAgentTenant(slug)` → redundant

After: only 2 queries (middleware + handler). The service uses `tenantID` directly.

### Defensive Guard

The guard checks all 4 fields that were missing in the original code:

```go
if ticket.CreatorID == 0 || ticket.CreatedTs == 0 ||
   ticket.UpdatedTs == 0 || ticket.Type == "" {
    return nil, fmt.Errorf("failed to construct ticket: internal error")
}
```

This catches the class of bug where a future code change removes one of the field assignments. The existing `store.Ticket.Validate()` (`store/ticket.go:67`) only checks `Title`, `Status`, `Priority`, and `Description` — not `CreatorID`, `CreatedTs`, `UpdatedTs`, or `Type`.

---

## Behavior Change

After this fix, `tickets.creator_id` for agent escalations reflects the **authenticated user** who submitted the escalation, not the system user (`agent_system`, typically user 1). This provides a correct audit trail but changes the historical pattern.

| Before | After |
|--------|-------|
| `creator_id = 1` (system user) | `creator_id = <authenticated user>` |
| No audit trail | Correct audit trail |

---

## Request/Response

### Request

```http
POST /api/v1/agent/:slug/escalate
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
    "title": "Customer reports water damage",
    "description": "Standing water in basement, needs emergency extraction",
    "priority": "high",
    "tags": ["urgent", "water-damage"]
}
```

### Fields

| Field | Required | Validation |
|-------|----------|------------|
| `title` | Yes | Non-empty string |
| `description` | Yes | Non-empty, max 10000 chars. Prepended with `/m/` if missing. |
| `priority` | No | `"high"`, `"medium"`, `"low"`. Invalid values return 400. |
| `tags` | No | Array of strings |
| `session_id` | No | String (unused by service, available for future use) |

### Success Response

```json
{
    "ticket_id": 169,
    "status": "created",
    "similar_count": 2
}
```

### Error Responses

| Status | Condition |
|--------|-----------|
| 401 | No JWT or invalid JWT |
| 403 | User lacks `chat:test` permission and is not admin |
| 400 | Missing title/description, description > 10000 chars, or invalid priority |
| 500 | Ticket creation failed (DB error, constraint violation, etc.) |

---

## Database Impact

### tickets Table

New rows inserted with:

| Column | Value |
|--------|-------|
| `title` | From request |
| `description` | From request (with `/m/` prefix) |
| `status` | `OPEN` |
| `priority` | `HIGH`, `MEDIUM`, or `LOW` from request |
| `creator_id` | Authenticated user's ID from JWT |
| `created_ts` | `time.Now().Unix()` |
| `updated_ts` | `time.Now().Unix()` |
| `type` | `agent_escalation` |
| `tenant_id` | From context (resolved by middleware) |
| `tags` | JSON array from request |
| `internal_notes` | Empty string (default) |

### Vector DB (LanceDB)

After ticket creation, the service searches for similar tickets via vector DB. The `similar_count` in the response reflects how many similar tickets exist. This is a read-only operation — no additional writes.

---

## Testing

### Build

```bash
task build:backend      # Compiles
task build:backend:rag  # Compiles with RAG support
```

### Unit Tests

```bash
go test ./server/router/api/v1/agent/... -run "TestEscalateTicket" -v
# SKIP: Requires real CockroachDB + store (integration test)
```

### Integration Test (Manual)

```bash
# 1. Start server
task run:rag

# 2. Verify route is mounted (401 without auth)
curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/escalate \
  -d '{"title":"Test","description":"test"}'
# Expected: 401

# 3. Verify permission check (403 without chat:test)
curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/escalate \
  -H "Authorization: Bearer <token_without_chat_test>" \
  -d '{"title":"Test","description":"test"}'
# Expected: 403

# 4. Verify priority validation (400 for invalid priority)
curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/escalate \
  -H "Authorization: Bearer <admin_token>" \
  -d '{"title":"Test","description":"test","priority":"urgent"}'
# Expected: 400

# 5. Create ticket (200)
curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/escalate \
  -H "Authorization: Bearer <admin_token>" \
  -d '{"title":"Water damage report","description":"Basement flooding","priority":"high"}'
# Expected: 200 with ticket_id

# 6. Verify in DB
sqlite3 build/data/memos_dev.db \
  "SELECT id, title, creator_id, type, priority, datetime(created_ts, 'unixepoch', 'localtime') \
   FROM tickets ORDER BY id DESC LIMIT 1"
# Expected: creator_id=<auth user>, type=agent_escalation, priority=HIGH
```

---

## Follow-Up Items

| Item | Severity | Ticket |
|------|----------|--------|
| Add rate limiting to escalation endpoint | MEDIUM | Track separately |
| Add automated unit test with mock store | MEDIUM | Track separately |
| Consider dedicated `ticket:escalate` permission | LOW | Track as permission-system enhancement |
| Fix `systemTicketCreatorID` side effects | MEDIUM | Track separately |
| Extend `Ticket.Validate()` to check CreatorID, Type, timestamps | MEDIUM | Track separately |

---

## Adversarial Code Review Prompt

```
You are a senior Go architect performing an adversarial code review of the
Bug 053 implementation (EscalateTicket Missing CreatorID). The implementation
spans 3 files:

  1. server/router/api/v1/v1.go — Route registration
  2. server/router/api/v1/agent/handlers.go — Handler rewrite
  3. server/router/api/v1/agent/service.go — Service signature + ticket construction

Review for:

1. CORRECTNESS
   - Does the handler correctly extract userID via h.getUserID(c)? What happens
     if the auth middleware hasn't run yet (e.g., route ordering issue)?
   - The service no longer calls GetAgentTenant. Is there any code path where
     tenantID could be stale or mismatched between middleware and handler?
   - The defensive guard checks CreatorID == 0. Could a valid user ID ever be 0
     in this schema? (Check: user table uses INTEGER PRIMARY KEY AUTOINCREMENT
     starting from 1)

2. SECURITY
   - The permission check uses PermChatTest. Could a user with chat:test
     permission create tickets for tenants they shouldn't access? (Check:
     TenantBindingMiddleware should prevent this, but verify the ordering)
   - The handler passes userID directly to the service. Could a malicious user
     manipulate the JWT to escalate as a different user? (Check: JWT signing
     and middleware validation)
   - Priority validation accepts "high", "medium", "low" (lowercase). The
     store constants are "HIGH", "MEDIUM", "LOW" (uppercase). The service
     does case-sensitive comparison. Is this intentional or a bug?

3. CONCURRENCY
   - Two users escalate simultaneously for the same tenant. The ticket creation
     is serialized by SQLite, but the vector DB search is async. Could the
     search return stale results? Is this acceptable?
   - The defensive guard reads ticket fields right after construction. Could a
     race condition corrupt the struct? (Hint: Go struct construction is
     single-threaded per goroutine)

4. ERROR HANDLING
   - The handler returns generic "Escalation service unavailable" for all
     service errors. Should we distinguish "ticket creation failed" from
     "vector search failed"?
   - The defensive guard returns fmt.Errorf. Should it use a typed error
     for better error handling upstream?

5. EDGE CASES
   - What happens if description is exactly "/m/" (3 chars)? The service checks
     len(description) < 3 || description[:3] != "/m/". Is "/m/" a valid memo
     link? What if the memo doesn't exist?
   - What happens if req.Tags is nil vs empty slice? The store marshals tags
     to JSON. Does nil produce "null" or "[]"?
   - What if the request body is valid JSON but with unexpected types (e.g.,
     priority as a number instead of string)? How does echo.Bind handle this?

6. BACKWARD COMPATIBILITY
   - The service signature changed from (ctx, tenantSlug, req) to
     (ctx, tenantID, req, creatorID). Are there any other callers beyond the
     handler? (Grep confirmed: no)
   - The response format is unchanged (ticket_id, status, similar_count). Any
     downstream consumers that depend on specific ticket_id values or creator_id?

7. TESTING
   - The existing TestEscalateTicket is skipped (requires real CockroachDB).
     What unit tests should be added to prevent regression?
   - Should we add a test for the defensive guard? How would we inject a 0
     CreatorID in a test?

8. STYLE & CONVENTIONS
   - Does the code follow existing patterns in the codebase? Compare with
     HandleChatInternal and HandleStartSimulation.
   - Are comments consistent with the project style?
   - Is the slog.Error call structured correctly (key-value pairs)?

Provide a severity rating (Critical/High/Medium/Low) for each finding.
Critical/High findings must be fixed before merging. Medium findings should
be tracked. Low findings are informational.
```
