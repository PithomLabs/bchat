# Adversarial Code Review: Bug 055 — Ticket List Filtered by JWT Tenant

**Bug ID:** 055
**Reviewer:** Kilo (Senior Go Architect)
**Date:** 2026-07-31
**Verdict:** APPROVED WITH NITS — implementation is correct and matches the plan. Two low-severity items are follow-up notes, not blockers.

---

## Executive Summary

The implementation in `code.md` is verified against the actual codebase. All four fixes are correctly implemented:

| Fix | File | Status |
|-----|------|--------|
| Fix 1: `ApplyTicketTenantFilter` super-user bypass | `tenant_context.go:93-111` | ✅ Verified |
| Fix 2: `HandleSelectTenant` nil guard | `auth_service.go:529-539` | ✅ Verified |
| Fix 3: `TenantID`/`TenantName` in response | `ticket_service.go:17-32, 276-292, 556-572` | ✅ Verified |
| Fix 4: Tenant column in UI | `Tickets.tsx:437, 486-492, 505` | ✅ Verified |

Build and tests pass. No critical or high-severity issues.

---

## Finding 1: LOW — Fix 2 Nil Guard Is Defensive But Incomplete

**Severity:** LOW

`auth_service.go:531`:
```go
if matchedUser != nil && !isSuperUser(matchedUser) {
```

At line 511, `matchedUser` is guaranteed non-nil because the function returns 401 if nil:
```go
if err != nil || matchedUser == nil {
    return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired selection token")
}
```

However, at line 542, `matchedUser.ID` is dereferenced without a nil check:
```go
if err := s.Store.RemoveUserAccessToken(ctx, matchedUser.ID, "selection:"+req.SelectionToken); err != nil {
```

If `matchedUser` were somehow nil (e.g., a race condition in `FindUserByAccessToken`), line 531's guard would skip the permission check, but line 542 would still panic.

**Recommendation:** This is acceptable because `matchedUser` is guaranteed non-nil by the earlier check. Document the invariant in a comment, or remove the redundant nil guard and rely on the existing 401 return. No code change needed.

---

## Finding 2: LOW — N+1 Queries Not Flagged in Code

**Severity:** LOW

`ticket_service.go:276-292` does N+1 `GetAgentTenant` queries:

```go
tenantMap := make(map[int32]string)
for _, t := range result {
    if t.TenantID != nil {
        if _, ok := tenantMap[*t.TenantID]; !ok {
            tenant, err := s.Store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: t.TenantID})
            if err == nil && tenant != nil {
                tenantMap[*t.TenantID] = tenant.CompanyName
            }
        }
    }
}
```

This is called on every `ListTickets` request. For 50 tickets across 10 tenants, that's 10 sequential queries. Acceptable for current scale, but there is no comment in the code flagging this as a known limitation.

**Recommendation:** Add a comment above the loop:
```go
// N+1: acceptable for admin-scale ticket lists (< 100 tickets, < 50 tenants).
// TODO: batch-resolve with ListAgentTenants when tenant count grows.
```

---

## Finding 3: LOW — `TenantName` Inconsistency Between Endpoints

**Severity:** LOW (documentation)

`ListTickets` populates `TenantName` via batch resolution. `GetTicket` returns `TenantID` but not `TenantName` because the enrichment only happens in `ListTickets`.

The frontend `TicketDetail.tsx` doesn't display `tenantName`, so this is acceptable. But it's not documented anywhere.

**Recommendation:** Add a comment in `GetTicket` or `convertTicketFromStore`:
```go
// Note: TenantName is only populated by ListTickets, not GetTicket.
```

---

## Finding 4: LOW — No Audit Logging for Super-User Bypass

**Severity:** LOW

When a super user bypasses tenant filtering (`ApplyTicketTenantFilter`) or permission checks (`HandleSelectTenant`), there is no log entry. This makes it harder to audit admin actions.

**Recommendation:** Add optional `slog.Info` logging:
```go
if user != nil && isSuperUser(user) {
    slog.Info("super-user bypass: ApplyTicketTenantFilter", "user_id", user.ID, "role", user.Role)
    return
}
```

And:
```go
if matchedUser != nil && !isSuperUser(matchedUser) {
    // ... permission check ...
}
slog.Info("super-user bypass: HandleSelectTenant", "user_id", matchedUser.ID, "tenant_id", req.TenantID)
```

Track as follow-up. Not a blocker for this bug.

---

## What Is Correct

| Check | Status | Notes |
|-------|--------|-------|
| `ApplyTicketTenantFilter` super-user bypass | **CORRECT** | Matches plan specification. Nil guard present. Scoped admins still filtered via `AllowedTenantIDs`. |
| `HandleSelectTenant` nil guard | **CORRECT** | Defensive nil check before `isSuperUser`. `matchedUser` is guaranteed non-nil by earlier 401 return. |
| `Ticket` response struct | **CORRECT** | `TenantID *int32`, `TenantName string` added. JSON tags match frontend expectations. |
| `convertTicketFromStore` | **CORRECT** | Copies `TenantID`. `TenantName` left empty (populated only in list responses). |
| `ListTickets` tenant name resolution | **CORRECT** | N+1 with dedup via `tenantMap`. Acceptable for scale. |
| Frontend tenant column visibility | **CORRECT** | Wrapped with `{isAdmin && ...}` on both header and cell. colSpan updated to 10/8. |
| Frontend fallback display | **CORRECT** | `ticket.tenantName || #${ticket.tenantId}` handles empty `TenantName`. |
| Backward compatibility | **CORRECT** | Additive fields only. Existing API consumers unaffected. |

---

## Regression Analysis

| Scenario | Expected | Actual |
|----------|----------|--------|
| HOST `ListTickets` | All tickets | **CORRECT** — super-user bypass |
| Scoped ADMIN `ListTickets` | Allowed tenants only | **CORRECT** — `isSuperUser` returns false |
| USER `ListTickets` | Own tenant | **CORRECT** — JWT tenant filter |
| HOST `select-tenant` | JWT with selected tenant | **CORRECT** — permission check skipped |
| Scoped ADMIN `select-tenant` | Requires perm row | **CORRECT** — unchanged |
| Regular user `select-tenant` | Requires perm row | **CORRECT** — unchanged |
| `GetTicket` response | `TenantID` populated, `TenantName` empty | **CORRECT** |
| `ListTickets` response | Both populated | **CORRECT** |
| Frontend tenant column visibility | Admin only | **CORRECT** — `isAdmin` guard |

---

## Final Verdict

**APPROVED WITH NITS.** Implementation is correct, secure, and matches the plan specification. Two follow-up items:

1. **LOW** — Add TODO comment flagging N+1 queries in `ListTickets`
2. **LOW** — Document `TenantName` only-in-list-responses behavior

No blockers for merge.

---

## Prompt for Next Review Cycle

When the N+1 queries become a bottleneck (> 50 tenants), implement `BatchGetAgentTenantsByIDs` in the store layer and replace the loop in `ListTickets` with a single batch query.
