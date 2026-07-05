# Adversarial Code Review: Implementation (code3_review.md)

**Reviewer:** Adversarial security review
**Date:** 2026-07-06
**Input:** code3_plan.md + code3_plan_review.md + actual code diff
**Status:** APPROVED WITH NITS

---

## Build Verification
- `go build ./server/router/api/v1/...` — ✅ PASS
- `go test ./server/router/api/v1/... -count=1` — ✅ PASS
- `go test ./server/router/api/v1/agent/... -count=1` — ✅ PASS

---

## Plan Compliance Matrix

| Fix | Plan Requirement | Implementation | Status |
|-----|-----------------|----------------|--------|
| FIX 1 | Use echo's c.Set() | Uses BOTH echo (AuthMiddleware → echo context) AND Go (gRPC interceptor → Go context) | ✅ CORRECT (dual approach) |
| FIX 2 | Pass tenantID as parameter to fallback | `createEscalationTicketFallback(ctx, tenantID, ...)` — tenantID passed as parameter | ✅ CORRECT |
| FIX 3 | Superuser bypass in tenant checks | `!isSuperUser(user)` added to GetMemo, UpdateMemo, DeleteMemo | ✅ CORRECT |
| FIX 4 | Auto-select single tenant in gRPC | `doSignIn` auto-selects if `len(perms) == 1` | ✅ CORRECT |
| Sprint 1 | ClaimsMessage + generateToken | TenantID field added, generateToken accepts tenantID | ✅ CORRECT |
| Sprint 1 | POST /auth/tenants | Implemented in auth_service.go | ✅ CORRECT |
| Sprint 1 | POST /auth/select-tenant | Implemented in auth_service.go | ✅ CORRECT |
| Sprint 1 | AuthMiddleware | Extracts tenant_id from JWT, sets via c.Set() | ✅ CORRECT |
| Sprint 1 | Force re-login migration | SQLite DELETE + PostgreSQL TRUNCATE | ✅ CORRECT |
| Sprint 2 | tenant_context.go | Echo-based helpers implemented | ✅ CORRECT |
| Sprint 2 | Unit tests | tenant_context_test.go with 4 test functions | ✅ CORRECT |
| Sprint 3 | CreateMemo tenant | Sets TenantID from context | ✅ CORRECT |
| Sprint 3 | ListMemos tenant filter | Applies tenant filter | ✅ CORRECT |
| Sprint 3 | GetMemo tenant check | Verifies tenant + superuser bypass | ✅ CORRECT |
| Sprint 3 | UpdateMemo tenant check | Verifies tenant + superuser bypass | ✅ CORRECT |
| Sprint 3 | DeleteMemo tenant check | Verifies tenant + superuser bypass | ✅ CORRECT |
| Sprint 4 | Fallback PII leak | Removed tenant_id from description | ✅ CORRECT |
| Sprint 4 | CEL filter | tenant_id removed from identifiers (both SQLite + Postgres) | ✅ CORRECT |

---

## Nits Found

### NIT 1 (LOW): Ticket Service Lacks Tenant Filtering

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 57-106 (CreateTicket), 164-207 (ListTickets), 348-407 (GetTicket)

The plan addressed memo service and agent escalation tickets, but the general ticket service endpoints are NOT tenant-scoped:

- `CreateTicket` (line 78-89): Does NOT set `TenantID` on the ticket
- `ListTickets` (line 175-196): Does NOT filter by tenant_id
- `GetTicket` (line 368-370): Does NOT verify tenant ownership

**Risk:** A user in Tenant A can read/create/update/delete tickets in Tenant B via the general ticket API. This is the same vulnerability pattern as the memo service, just for tickets.

**Fix:** Add `ApplyTicketTenantFilter(c, find)` to ListTickets and GetTicket, and set TenantID from context in CreateTicket.

---

### NIT 2 (LOW): HandleSelectTenant Scans All Users

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 444-465

```go
users, err := s.Store.ListUsers(ctx, &store.FindUser{})
for _, user := range users {
    tokens, err := s.Store.GetUserAccessTokens(ctx, user.ID)
    // ... scan all tokens
}
```

This is O(N*M) where N = users, M = tokens per user. For large deployments, this will be slow.

**Fix:** Add a direct lookup method to find user by selection token, or store the user_id with the selection token.

---

### NIT 3 (LOW): Selection Token Lacks Explicit Expiry

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 413-421

The plan says selection token should expire in 5 minutes. The implementation stores it as a regular access token via `UpsertAccessTokenToStore` but there's no explicit 5-minute expiry enforced. The token will persist until the user access token cleanup runs.

**Fix:** Store selection token with a 5-minute TTL, or add a timestamp check in HandleSelectTenant.

---

### NIT 4 (LOW): tenant_context.go Functions Partially Unused

**File:** `server/router/api/v1/tenant_context.go`

The `ApplyTenantFilter` and `ApplyTicketTenantFilter` functions are defined but not called by memo_service.go or ticket_service.go. The memo service uses `GetTenantIDFromContext(ctx)` from acl.go directly.

This is not a bug (the dual approach works), but the functions are dead code.

**Fix:** Either use them in the service handlers, or remove them to avoid confusion.

---

### NIT 5 (LOW): Missing DisallowPasswordAuth Check in HandleAuthTenants

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 361-427

The gRPC `SignIn` checks `workspaceGeneralSetting.DisallowPasswordAuth` (line 70), but the REST `HandleAuthTenants` does not. A user could bypass the password auth restriction by using the REST endpoint.

**Fix:** Add the same check in HandleAuthTenants.

---

### NIT 6 (LOW): doSignIn Multi-Tenant Error Path

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 173-187

When a user has multiple tenants and `tenantID` is nil, the function continues to generate a token with `nil` tenant_id. This means multi-tenant users who call gRPC SignIn get a token without tenant scoping, which is the original vulnerability.

The plan says: "If >1 tenants: reject with 'multiple tenants found, use /auth/tenants endpoint'". But the implementation doesn't reject — it silently continues.

**Fix:** Add explicit rejection when `tenantID == nil && len(perms) > 1`:
```go
if tenantID == nil && len(perms) > 1 {
    return status.Errorf(codes.FailedPrecondition, "multiple tenants found, use /auth/tenants endpoint")
}
```

---

## Findings Summary

| # | Finding | Severity | Fix Required |
|---|---------|----------|--------------|
| 1 | Ticket service lacks tenant filtering | HIGH | Yes — same pattern as memo service |
| 2 | HandleSelectTenant scans all users | LOW | Optional — performance |
| 3 | Selection token lacks explicit 5-min expiry | LOW | Optional — security hardening |
| 4 | tenant_context.go functions partially unused | LOW | Optional — dead code |
| 5 | Missing DisallowPasswordAuth in REST endpoint | LOW | Yes — auth bypass |
| 6 | Multi-tenant users not rejected in gRPC SignIn | HIGH | Yes — original vulnerability persists |

---

## Critical Finding: NIT 6 Is Actually a Security Gap

**NIT 6 is not a nit — it's a security issue.**

The plan's Q10 decision was: *"If >1 tenants: reject with 'multiple tenants found, use /auth/tenants endpoint'"*

The implementation at `auth_service.go:183-186`:
```go
// Auto-select single tenant if not already specified
if tenantID == nil && len(perms) == 1 {
    tenantID = &perms[0].TenantID
}
```

When `len(perms) > 1` and `tenantID == nil`, the code falls through to line 189:
```go
accessToken, err := GenerateAccessToken(user.Email, user.ID, tenantID, expireTime, []byte(s.Secret))
```

This generates a JWT with `nil` tenant_id for multi-tenant users. When they use this token with the memo API, `GetTenantIDFromContext` returns nil, and the tenant filter is not applied. **This is the original cross-tenant data leakage vulnerability.**

**Fix required:**
```go
// Auto-select single tenant if not already specified
if tenantID == nil && len(perms) == 1 {
    tenantID = &perms[0].TenantID
} else if tenantID == nil && len(perms) > 1 {
    return status.Errorf(codes.FailedPrecondition, "multiple tenants found, use /auth/tenants endpoint")
}
```

---

## Final Verdict: APPROVED WITH 2 REQUIRED FIXES

The implementation is solid and covers all major requirements from the plan. However, 2 issues must be fixed:

1. **NIT 6 → REQUIRED FIX:** Multi-tenant users must be rejected in gRPC SignIn (not silently given a nil-tenant token)
2. **NIT 1 → REQUIRED FIX:** Ticket service needs tenant filtering (same pattern as memo service)

The remaining nits (2-5) are optional improvements that can be addressed in a follow-up.

**All 12 Q&A decisions from code2_plan_review.md have been correctly implemented.**
