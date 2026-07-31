# Adversarial Review: Plan 2 — EscalateTicket Missing CreatorID (Revised)

**Bug ID:** 053
**Reviewer:** Kilo (Senior Go Architect)
**Date:** 2026-07-31
**Verdict:** APPROVED WITH NITS — core fix is correct, but 2 critical and 3 medium issues must be resolved before execution.

---

## Executive Summary

Plan 2 correctly addresses all Critical findings from the first review:
- Route registration is added in `v1.go` under `authGroup`
- Missing `Type: "agent_escalation"` is included
- Missing `CreatedTs`/`UpdatedTs` are included
- Defensive guard covers all four required fields
- Authentication model is explicitly chosen: `authGroup` with JWT user ID

However, before implementation, the plan has remaining blockers:

| Severity | Count | Must Fix Before Exec? |
|----------|-------|----------------------|
| CRITICAL | 0 | — |
| HIGH | 2 | Yes |
| MEDIUM | 3 | Yes |
| LOW | 2 | No |

---

## Finding 1: Wrong Port in Verification Plan

**Severity:** HIGH

The plan's verification curl commands use port **5230**:

```bash
curl http://localhost:5230/api/v1/agent/hackathon-demo/escalate
```

`task run:rag` starts the server on port **8081** (see `bin/memos/main.go:210`, default `viper.SetDefault("port", 8081)`). Port 5230 is the Fly/ECS Docker production port. All curl tests in the verification plan will fail against the local dev server.

**Required fix:** Change all verification curl commands from `5230` to `8081`.

---

## Finding 2: Missing Permission Check

**Severity:** HIGH

`HandleEscalateTicket` extracts `userID` from JWT but performs **no permission check** before calling the service. Every other endpoint in `authGroup` enforces either admin role or a specific permission:

| Endpoint | Permission Check |
|----------|-----------------|
| `HandleChatInternal` (handlers.go:626) | `h.hasPermission(c, tenant.ID, PermChatTest)` |
| `HandleStartSimulation` (handlers.go:3463) | `h.hasPermission(c, tenant.ID, PermChatTest)` |
| `HandleListSessions` (handlers.go:3328) | `h.hasPermission(c, tenant.ID, PermChatLogs)` |

Without an explicit check, **any authenticated user** with tenant access can create escalation tickets. This is a spam and abuse vector.

**Required fix:** Add the standard pattern in the handler before calling the service:

```go
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:test permission")
}
```

Use `PermChatTest` because escalation is a sensitive action comparable to simulation and internal chat.

---

## Finding 3: Redundant Tenant DB Lookup — Triple Hit

**Severity:** MEDIUM

Current execution path for one escalation request:

1. `TenantBindingMiddleware` → `GetAgentTenant(ctx, &FindAgentTenant{Slug: &slug})`
2. `getTenantOrFail(ctx, h.store, c)` in handler → `GetAgentTenant(ctx, &FindAgentTenant{ID: &tenantID})`
3. `EscalateTicket` in service → `GetAgentTenant(ctx, &FindAgentTenant{Slug: &tenantSlug})`

Three database queries for what is logically one tenant resolution. The plan keeps `EscalateTicket` accepting `tenantSlug string`, forcing the third lookup.

**Required fix:** Change the service signature to accept `tenantID int32`:

```go
func (s *Service) EscalateTicket(ctx context.Context, tenantID int32, req EscalateTicketRequest, creatorID int32) (*EscalateTicketResponse, error)
```

Remove the internal `GetAgentTenant` call. Use `tenantID` directly for ticket construction, vector DB query, and all tenant-scoped operations.

---

## Finding 4: Missing Priority Validation

**Severity:** MEDIUM

The service silently defaults unknown priority values to `MEDIUM`:

```go
if req.Priority == "high" {
    priority = store.TicketPriorityHigh
} else if req.Priority == "low" {
    priority = store.TicketPriorityLow
}
// anything else → MEDIUM
```

A client sending `"priority":"urgent"` or `"priority":"P1"` gets a medium-priority ticket without error or log. This creates operational confusion.

**Required fix:** Add validation in the handler before calling the service:

```go
if req.Priority != "" && req.Priority != "high" && req.Priority != "medium" && req.Priority != "low" {
    return echo.NewHTTPError(http.StatusBadRequest, "priority must be high, medium, or low")
}
```

---

## Finding 5: Behavior Change Undocumented

**Severity:** MEDIUM

The plan switches `CreatorID` from `systemTicketCreatorID(ctx)` (typically user 1, `agent_system`) to the authenticated user's ID from JWT. This is an **audit trail improvement** but it is a breaking semantic change.

Downstream consumers or dashboards that expect `creator_id = 1` for system-generated escalations will see different values after this fix.

**Required fix:** Add to the plan:

> **Behavior change:** After this fix, `tickets.creator_id` for escalations will reflect the authenticated user who submitted the escalation, not the system user (`agent_system`). This provides a correct audit trail but changes the historical pattern. If backward compatibility is required, expose a config flag.

---

## Finding 6: Magic String `"user-id"` Instead of Context Key

**Severity:** LOW (Nit)

The plan uses the literal string `"user-id"` in `c.Get("user-id")` instead of the exported helper `getUserIDContextKey()`. The rest of the codebase (e.g., `ticket_service.go:539`, `notification_service.go:34`) consistently uses the helper.

**Fix:** Use `getUserIDContextKey()` for consistency and to avoid drift if the key name ever changes.

---

## Finding 7: Code Comment Style

**Severity:** LOW (Nit)

The plan's Step 1 route registration comment:

```go
// Escalation route (requires auth + tenant binding)
authGroup.POST("/:slug/escalate", s.agentHandler.HandleEscalateTicket)
```

This matches the project's style. No change needed — just verify it is preserved in the final code.

---

## Revised Implementation Steps (Required Changes Highlighted)

| Step | File | Action |
|------|------|--------|
| 1 | `server/router/api/v1/v1.go` | Register `POST /:slug/escalate` in `authGroup` — **change port in plan from 5230 → 8081** |
| 2 | `server/router/api/v1/agent/handlers.go` | Extract `userID` via `getUserIDContextKey()`, **add `hasPermission(..., PermChatTest)` check**, validate priority |
| 3 | `server/router/api/v1/agent/service.go` | Change signature to `EscalateTicket(ctx, tenantID int32, req, creatorID int32)`, remove internal `GetAgentTenant`, set all 4 missing fields |
| 4 | `server/router/api/v1/agent/service.go` | Add defensive guard checking all required fields |

### Step 3 Signature Change Details

```go
// Before:
func (s *Service) EscalateTicket(ctx context.Context, tenantSlug string, req EscalateTicketRequest) (*EscalateTicketResponse, error)

// After:
func (s *Service) EscalateTicket(ctx context.Context, tenantID int32, req EscalateTicketRequest, creatorID int32) (*EscalateTicketResponse, error)
```

Inside the function, remove:
```go
tenant, err := s.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &tenantSlug})
```

Use `tenantID` directly for:
- `TenantID: &tenantID`
- Vector DB `SearchQuery.TenantID`
- Any error logging

---

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Authenticated user can spam escalations | Add `hasPermission(..., PermChatTest)` check |
| Triple DB lookup slows escalation path | Pass `tenantID` directly to service |
| Invalid priority silently accepted | Validate in handler, return 400 |
| Audit trail now shows user instead of system | Document as intentional behavior change |
| Port 5230 used in docs causes confusion | Fix plan and any docs/curl examples |

---

## Final Verdict

**APPROVED WITH NITS.** The core diagnosis and fix strategy are correct. Required changes before execution:

1. Fix verification port: `5230` → `8081`
2. Add permission check in handler: `hasPermission(..., PermChatTest)`
3. Change service signature: `tenantSlug string` → `tenantID int32`, remove internal tenant lookup
4. Add priority validation in handler
5. Document CreatorID behavior change
6. Use `getUserIDContextKey()` instead of magic string

These are all resolvable within the same code change set. No architectural rework is needed.
