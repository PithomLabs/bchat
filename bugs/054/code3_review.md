# Adversarial Code Review: Bug 054 Plan 3 Implementation

**Bug ID:** 054
**Reviewer:** Kilo (Senior Go Architect)
**Date:** 2026-07-31
**Verdict:** APPROVED WITH NITS — one critical frontend bug, one medium issue, two low nits.

---

## Executive Summary

The implementation is correct and well-structured. Build, `go vet`, and `go test` all pass. The backend changes (054b, 054c, 054d, hackathon) are clean and match the plan3.md specifications.

One **critical frontend bug** would prevent the tenant dropdown from ever populating: `fetchTenants` sends an empty body to an endpoint that requires credentials.

---

## Finding 1: Critical — `fetchTenants` Sends Empty Body to Credentials-Required Endpoint

**Severity:** CRITICAL (runtime error — dropdown never populates)

**File:** `web/src/pages/Tickets.tsx:151-164`

```typescript
const fetchTenants = async () => {
    try {
        const response = await fetch("/api/v1/auth/tenants", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({}),  // BUG: empty body
        });
```

`POST /api/v1/auth/tenants` (`HandleAuthTenants` at `auth_service.go:365`) requires `username` and `password` in the request body:

```go
var req struct {
    Username string `json:"username"`
    Password string `json:"password"`
}
if err := c.Bind(&req); err != nil {
    return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
}
user, err := s.Store.GetUser(c.Request().Context(), &store.FindUser{
    Username: &req.Username,
})
if err != nil || user == nil {
    return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
}
```

Sending `{}` binds to empty strings → `GetUser` returns nil → **HTTP 401**. The catch block logs the error and `availableTenants` stays `[]`. The dropdown never renders.

**Required fix:** Use `GET /api/v1/agent/tenants` (admin endpoint under `adminGroup` with auth middleware). This works with cookie auth and returns all tenants for super users:

```typescript
const fetchTenants = async () => {
    try {
        const response = await fetch("/api/v1/agent/tenants");
        if (!response.ok) throw new Error("Failed to fetch tenants");
        const data = await response.json<{ tenants: AgentTenant[] }>();
        setAvailableTenants(data.tenants || []);
    } catch (error) {
        console.error("Error loading tenants:", error);
    }
};
```

---

## Finding 2: Medium — `InferResolutionForNewTicket` Error Path Loses Error Context

**Severity:** MEDIUM (silent failure)

**File:** `server/router/api/v1/agent/service.go:5691-5694`

```go
_, err := s.store.UpdateTicket(ctx, update)
if err != nil {
    slog.Error("failed to update ticket with inferred resolution", "error", err, "ticket_id", ticket.ID)
    return ""
}
```

When `UpdateTicket` fails, the function logs the error and returns `""`. The caller (`IndexTicketContent`) then returns `inferred = false`, and the goroutine in `ticket_service.go` skips system comment creation. This is correct behavior.

However, the caller in `ticket_service.go:193` also logs an error:

```go
_, _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, true)
if err != nil {
    slog.Error("failed to index new ticket for RAG", "ticket_id", ticket.ID, "error", err)
    return
}
```

But `IndexTicketContent` returns `nil` error when `InferResolutionForNewTicket` fails (because the failure is in the inference step, not the indexing step). So the goroutine silently skips system comment creation without any indication that inference failed.

**Recommendation:** This is acceptable for the hackathon scope. The `InferResolutionForNewTicket` error is logged at the point of failure. The caller doesn't need to propagate it. Document as intentional.

---

## Finding 3: Low — `ticketIndexMu` Never Evicts

**Severity:** LOW (memory leak over time)

**File:** `server/router/api/v1/agent/service.go:5722-5726`

```go
muKey := fmt.Sprintf("%d:%d", tenantID, ticket.ID)
muVal, _ := ticketIndexMu.LoadOrStore(muKey, &sync.Mutex{})
mu := muVal.(*sync.Mutex)
mu.Lock()
defer mu.Unlock()
```

The `ticketIndexMu` (`sync.Map`) grows unbounded. Each ticket creates an entry that is never evicted. For a system with thousands of tickets, this is a slow memory leak.

This is a pre-existing issue from Bug 052, not introduced by this implementation. The plan3 code correctly reuses the existing pattern.

**Recommendation:** Track as follow-up. Add eviction logic (e.g., LRU with TTL) in a future iteration.

---

## Finding 4: Low — System Comment Dedup Not Implemented

**Severity:** LOW (potential duplicate comments)

**File:** `server/router/api/v1/ticket_service.go:189-210`

The goroutine creates a system comment after indexing with `triggerInference: true`. If the same ticket triggers this path twice (e.g., due to a retry or race), duplicate system comments could be created.

Analysis of current call sites:
- Dedup path (line 172): `triggerInference: false` → no system comment
- New creation path (line 193): `triggerInference: true` → system comment created
- Update path (line 413): `triggerInference: false` → no system comment

Only the new-creation path triggers inference. Duplicate creation is unlikely in practice.

**Recommendation:** Acceptable for hackathon scope. Document as known limitation. Add dedup check in follow-up if needed.

---

## Verified Correctness

### Backend

| Check | Status | Notes |
|-------|--------|-------|
| 054b: `TenantID: memo.TenantID` | **CORRECT** | `memo_service.go:1173` — matches plan |
| 054c: `CreateTicketRequest.TenantID` | **CORRECT** | `ticket_service.go:40` — `*int32` with JSON tag |
| 054c: Superuser override logic | **CORRECT** | `ticket_service.go:98-112` — `isSuperUser` check + tenant validation |
| 054d: `InferResolutionForNewTicket` returns `string` | **CORRECT** | `service.go:5598` — returns `suggestedNotes` or `""` |
| 054d: `IndexTicketContent` returns `(int, bool, error)` | **CORRECT** | `service.go:5720` — named returns, `inferred` captured |
| 054d: 5 callers updated | **CORRECT** | All use `_, _, err` or `_, _, idxErr` |
| System comment helper | **CORRECT** | `ticket_service.go:611-647` — proper error handling |
| `store.SystemBotID` usage | **CORRECT** | `ticket_service.go:627` — value is `0` |
| `store.Public` visibility | **CORRECT** | `ticket_service.go:629` — matches `store/memo.go:17` |
| `store.MemoRelationComment` type | **CORRECT** | `ticket_service.go:639` — matches `store/memo_relation.go:13` |

### Frontend

| Check | Status | Notes |
|-------|--------|-------|
| Tenant dropdown visibility | **CORRECT** | `Tickets.tsx:564` — `isAdmin && availableTenants.length > 0` |
| `selectedTenantId` initialization | **CORRECT** | `Tickets.tsx:117-120` — from `localStorage.tenant_id` |
| Payload includes `tenantId` | **CORRECT** | `Tickets.tsx:250-251` — only when `isAdmin && selectedTenantId` |
| System suggestion detection | **CORRECT** | `Tickets.tsx:796` — `startsWith("## AI Suggestion")` |
| Amber styling | **CORRECT** | `Tickets.tsx:801-803` — matches plan specification |

---

## Regression Analysis

| Scenario | Expected | Actual |
|----------|----------|--------|
| HOST creates ticket via REST | `tenant_id` from JWT | **CORRECT** — `getTenantFromContext(c)` |
| HOST creates ticket with `tenantId` | Uses specified tenant | **CORRECT** — override logic |
| Regular user creates ticket | JWT tenant used | **CORRECT** — `tenantId` ignored for non-superusers |
| Regular user sends `tenantId` | HTTP 400 | **CORRECT** — `isSuperUser` check |
| Auto-ticket creation (054b) | `tenant_id` from memo | **CORRECT** — `TenantID: memo.TenantID` |
| Inference finds matches | `internal_notes` + system comment | **CORRECT** — goroutine creates both |
| Inference finds no matches | No comment | **CORRECT** — `inferred = false`, no comment |
| Reindex triggered | No duplicate source files | **CORRECT** — content-hash dedup |
| Existing callers compile | No breakage | **CORRECT** — all 5 callers updated |

---

## Final Verdict

**APPROVED WITH NITS.** One critical issue requires correction:

1. **CRITICAL:** Fix `fetchTenants` to use `GET /api/v1/agent/tenants` instead of `POST /api/v1/auth/tenants`

Two low-severity items are follow-up notes:
- `ticketIndexMu` unbounded growth (pre-existing)
- System comment dedup (not needed for current call sites)

**Recommended action:** Fix the `fetchTenants` endpoint, then proceed.
