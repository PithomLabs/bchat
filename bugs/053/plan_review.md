# Adversarial Review: Ticket Not Saved — EscalateTicket Missing CreatorID

**Bug ID:** 053
**Reviewer:** Kilo (Senior Go Architect)
**Date:** 2026-07-31
**Verdict:** REJECT with NITS — plan has a critical missing route registration and incomplete field analysis. Rewrite required before implementation.

---

## Executive Summary

The plan correctly identifies that `EscalateTicket` omits `CreatorID`, causing a FK constraint violation. However, it fails on three critical issues:

1. **The endpoint is never mounted.** `POST /api/v1/agent/:slug/escalate` is documented but not registered in any Echo route group. A search of the entire `server/router` tree confirms there is no `POST("/:slug/escalate"` registration. No fix in `service.go` will make it reachable.
2. **`Type` field is also missing** from the plan's "Additional Missing Fields" analysis, even though `createEscalationTicket` and `createEscalationTicketFallback` both set `Type: "agent_escalation"`.
3. **Authentication model is undefined.** The handler does not extract a user from JWT. The plan must decide whether this is a public endpoint (system creator is appropriate, but opens a spam vector) or an authenticated endpoint (must pass the caller's user ID).

---

## Finding 1: Route Never Registered

**Severity:** CRITICAL

`HandleEscalateTicket` is defined in `handlers.go:466` but is **never added to any Echo group** in `server/router/api/v1/v1.go`. A search of the entire `server/router` tree confirms there is no `POST("/:slug/escalate"` registration.

Even with the `CreatorID` fix, the endpoint returns 404.

**Required fix:** Add route registration in `RegisterAgentRoutes()` in `v1.go`. The plan must state whether this goes in `publicGroup`, `authGroup`, or `adminGroup`.

### Recommendation

```go
// In server/router/api/v1/v1.go inside RegisterAgentRoutes:
authGroup.POST("/:slug/escalate", s.agentHandler.HandleEscalateTicket)
```

Use `authGroup` (not public) to prevent unauthentated ticket spam.

---

## Finding 2: Missing `Type` Field

**Severity:** HIGH

The plan's "Additional Missing Fields" section lists only `CreatedTs` and `UpdatedTs`. It omits `Type`.

Both existing escalation paths set:

```go
Type: "agent_escalation"
```

Without it, the ticket gets the schema default `'TASK'` (see `LATEST.sql:162`). Downstream consumers (billing, triage, reporting) that filter `WHERE type = 'agent_escalation'` will miss these tickets.

**Required fix:** Add `Type: "agent_escalation"` to the ticket constructor.

---

## Finding 3: Authentication Model Undefined

**Severity:** HIGH

`HandleEscalateTicket` calls `getTenantOrFail(ctx, h.store, c)` but does **not** call `getUserIDOrFail` or extract a user ID from the JWT claim in context. The handler has no concept of an authenticated caller.

The plan says:

> "The function doesn't receive an authenticated user context (it's called from the agent service), so use the system ticket creator ID."

This is true for the internal service method, but the **handler** is the entry point for the HTTP endpoint. The plan must choose:

| Option | CreatorID | Implication |
|--------|-----------|-------------|
| **Public endpoint** | `systemTicketCreatorID(ctx)` | Anyone can create tickets anonymously. Spam risk. Justified only if escalation is unauthenticated by design. |
| **Authenticated endpoint** | `user.ID` from JWT | Requires extending `HandleEscolateTicket` to extract the authenticated user and passing it into the service, or at minimum documenting why auth is omitted. |

### Recommendation

Register under `authGroup` with `AuthMiddleware`. Modify `HandleEscalateTicket` to extract the authenticated user from JWT context and pass it to the service. This prevents anonymous spam while preserving audit trail.

---

## Finding 4: Defensive Zero-Value Check Is Incomplete

**Severity:** MEDIUM

The plan's defensive guard:

```go
if ticket.CreatorID == 0 {
    slog.Error("EscalateTicket: CreatorID is 0, falling back to system creator",
        "title", req.Title, "tenant_id", tenant.ID)
    ticket.CreatorID = s.systemTicketCreatorID(ctx)
}
```

This only catches one of three missing fields. It does **not** check:

- `CreatedTs == 0` — means "epoch", causing incorrect ordering and timezone display.
- `UpdatedTs == 0`
- `Type == ""` — defaults to `'TASK'` silently.

### Recommendation

Call `store.Ticket.Validate()` after construction. Or expand the guard to check all four required fields.

---

## Finding 5: `systemTicketCreatorID` Has Unintended Side Effects

**Severity:** MEDIUM

`systemTicketCreatorID` (service.go:4574) does the following:

1. `ListUsers` — returns first user globally.
2. If empty, `CreateUser` with `RoleAdmin`.
3. If that fails, looks up `agent_system` by username.
4. If all fail, hardcodes fallback to `user 1`.

**Issues:**
- **Side effect in read path:** `CreateUser` mutates the database just to get a creator ID.
- **Non-deterministic selection:** `ListUsers` without tenant filter returns a user from any tenant. In multi-tenant mode, this may be incorrect — the creator should ideally be tenant-local or a globally shared system user.
- **Hardcoded fallback:** `return 1` is a magic number. If `user 1` is deleted, this silently references a non-existent user.

This is pre-existing behavior and other paths use it, so changing it is out of scope for this bug. But the plan should at least note the coupling. A separate ticket should be created to make `systemTicketCreatorID` a constructor/factory or cache.

---

## Finding 6: Testing Gap — Unmounted Route

**Severity:** HIGH

The plan's verification step lists:

```bash
curl -X POST "http://localhost:5230/api/v1/agent/:slug/escalate" ...
```

If the route is not registered (Finding 1), this test gets 404. The plan does not include a route-registration step in the verification workflow.

### Recommended Verification Amendment

1. Add route registration.
2. Verify `GET` list of routes includes `/api/v1/agent/:slug/escalate`.
3. Only then test with `curl`.

---

## Finding 7: Audit Table Uses Drifting Line Numbers

**Severity:** LOW

The "Audit Other Ticket Creation Paths" table uses `service.go:4727` etc. as line references. These line numbers will drift on every edit. Use function names instead.

---

## Revised Implementation Steps

| Step | File | Action |
|------|------|--------|
| 1 | `server/router/api/v1/agent/service.go` | Add `CreatorID`, `CreatedTs`, `UpdatedTs`, and `Type` to `EscalateTicket` ticket constructor |
| 2 | `server/router/api/v1/agent/service.go` | Add defensive guard after construction checking all four fields |
| 3 | `server/router/api/v1/v1.go` | Register route: `authGroup.POST("/:slug/escalate", s.agentHandler.HandleEscalateTicket)` |
| 4 | `server/router/api/v1/agent/handlers.go` | Extract authenticated user from JWT and pass to service |
| 5 | `server/router/api/v1/agent/service.go` | Update `EscalateTicket` signature to accept `creatorID int32` |

### Step 1 — Fix Service Constructor

```go
// server/router/api/v1/agent/service.go:5546
now := time.Now().Unix()
creatorID := s.systemTicketCreatorID(ctx)
ticket := &store.Ticket{
    Title:       req.Title,
    Description: description,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    CreatorID:   creatorID,
    TenantID:    &tenant.ID,
    Tags:        req.Tags,
    CreatedTs:   now,
    UpdatedTs:   now,
    Type:        "agent_escalation",
}
```

### Step 2 — Defensive Guard

```go
if ticket.CreatorID == 0 || ticket.CreatedTs == 0 || ticket.UpdatedTs == 0 || ticket.Type == "" {
    slog.Error("EscalateTicket: ticket construction incomplete",
        "title", req.Title, "tenant_id", tenant.ID,
        "creator_id", ticket.CreatorID, "created_ts", ticket.CreatedTs, "type", ticket.Type)
    return nil, fmt.Errorf("failed to construct ticket: internal error")
}
```

### Step 3 — Register Route

```go
// server/router/api/v1/v1.go inside RegisterAgentRoutes(), under authGroup:
authGroup.POST("/:slug/escalate", s.agentHandler.HandleEscalateTicket)
```

### Step 4 — Extract Authenticated User (Recommended)

Update handler to extract the authenticated user from JWT context and pass to service.

### Step 5 — Update Verification Plan

Add route registration verification and actual curl test against a running server.

---

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Public escalation endpoint is abused | Require auth (`authGroup`) |
| systemTicketCreatorID creates user side-effect | Accepted for this bug; track separately |
| Multi-tenant user selection returns wrong creator | Clarified as global HOST user behavior |
| Timestamp defaults to epoch if guard missed | Guard catches all zero-value cases |

---

## Final Verdict

**REJECT.** The plan is not implementation-ready. Rewrite required with:
1. Route registration step
2. `Type` field inclusion
3. Explicit auth model
4. Complete defensive checks

**Recommended action:** Update `plan.md` to match the steps above, then re-submit for review.
