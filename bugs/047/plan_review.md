# Adversarial Plan Review — Question 1

**Reviewer:** Senior Go Architect (automated)
**Date:** 2026-07-23
**Scope:** Multi-tenant auth flow — REST `/auth/tenants` + `/auth/select-tenant` implementation status and JWT `tenant_id` validation

---

## Question

> *"Multi-tenant auth flow: The REST `/auth/tenants` + `/auth/select-tenant` flow — is this fully implemented and tested? The review found JWT `tenant_id` set but not validated against `allowed_tenant_ids`."*

---

## Verdict: Partially Implemented, Architecturally Sound but Has Gaps

The REST tenant selection flow is **fully implemented** in `auth_service.go:363-554` with rate limiting, 5-minute token expiry, single-use selection tokens, and permission verification via `GetUserTenantPermission`.

However, the original plan's concern about "JWT `tenant_id` set but not validated against `allowed_tenant_ids`" reveals a **deeper architectural issue** than the plan articulates.

---

## Detailed Findings

### 1. REST Flow Implementation — ✅ Complete

Both endpoints are registered as standalone unauthenticated routes in `v1.go:213-214`:

| Endpoint | File:Line | Status |
|----------|-----------|--------|
| `POST /api/v1/auth/tenants` | `auth_service.go:363-446` | Fully implemented |
| `POST /api/v1/auth/select-tenant` | `auth_service.go:448-554` | Fully implemented |

**`HandleAuthTenants` flow:**
1. Rate limits by client IP (5 attempts/min) — line 371
2. Validates username/password — lines 375-396
3. Fetches `ListUserTenantPermissions` — line 407
4. Returns 403 if zero tenant associations — line 411
5. Generates 32-byte random `selectionToken` — lines 430-431
6. Stores in `user_access_token` with `"selection:"` prefix — lines 436-439
7. Returns `AuthTenantsResponse` with tenant list + selection token — lines 442-445

**`HandleSelectTenant` flow:**
1. Rate limits by client IP — lines 451-458
2. Validates `selection_token` and `tenant_id` — lines 465-467
3. Scans users to find token owner — lines 472-499
4. Validates 5-minute expiry — lines 505-510
5. Verifies user permission for target tenant — lines 512-518
6. Deletes selection token (single-use) — lines 521-523
7. Generates JWT with `tenant_id` — line 528
8. Sets cookie — lines 537-549

### 2. JWT `tenant_id` in Claims — ✅ Set Correctly

`ClaimsMessage` struct at `auth.go:31-35`:
```go
type ClaimsMessage struct {
    Name     string `json:"name"`
    TenantID *int32 `json:"tenant_id,omitempty"`
    jwt.RegisteredClaims
}
```

**How `tenant_id` is populated per auth path:**

| Auth Path | tenant_id Value | File:Line |
|-----------|----------------|-----------|
| gRPC `SignIn` (single tenant) | Auto-selected | `auth_service.go:174-211` |
| gRPC `SignIn` (multi tenant) | Error: use `/auth/tenants` | `auth_service.go:186-188` |
| REST `HandleSignIn` | **Always nil** | `auth_service.go:664` |
| REST `HandleSelectTenant` | User's chosen `req.TenantID` | `auth_service.go:528` |
| REST `HandleSignUp` | nil | `auth_service.go:614` |

### 3. TenantBindingMiddleware — ✅ Exists but Different Scope

`tenant_binding.go:16-72` validates that the **user** has access to the tenant identified by the **URL slug**, not the JWT's `tenant_id`:

```go
// Lines 35-37: Bypass for Host/Admin without scoped tenant
if user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0) {
    return next(c)
}

// Lines 52-67: Check permission against slug-derived tenant
if user.Role == store.RoleAdmin {
    if !contains(user.AllowedTenantIDs, tenant.GUID) {
        return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
    }
} else {
    perms, err := s.ListUserTenantPermissions(...)
    if err != nil || len(perms) == 0 {
        return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
    }
}
```

### 4. Agent Handlers — ❌ JWT `tenant_id` Never Validated

**0 of 87 slug-extracting handlers compare JWT `tenant_id` against the slug-derived tenant.**

Every handler follows this pattern (e.g., `HandleChatInternal` at line 600):
```go
slug := c.Param("slug")                                    // URL-controlled
tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
    return echo.NewHTTPError(http.StatusForbidden, "...")
}
// tenant.ID is NEVER compared against getTenantFromContext(c)
```

`getTenantFromContext(c)` at `tenant_context.go:14` is **never called in any agent handler**.

### 5. `ApplyTenantFilter` — ❌ Not Available for Agent Data

`ApplyTenantFilter` at `tenant_context.go:74-88` works **exclusively for `*store.FindMemo`**. There is no equivalent for agent data types (`FindAgentService`, `FindAgentSession`, etc.).

**All 20 tenant-scoped List methods correctly include `tenant_id` in SQL when `find.TenantID` is set**, but there is no enforcement layer — callers can forget to set it.

---

## Critical Distinction the Plan Misses

The plan frames this as "JWT `tenant_id` not validated against `allowed_tenant_ids`" but the real issue is **two separate isolation models that don't intersect**:

| Model | Isolation Mechanism | JWT `tenant_id` Role |
|-------|--------------------|--------------------|
| Memo/Ticket | `ApplyTenantFilter` → SQL WHERE clause | **Enforcement point** |
| Agent | Slug → RBAC check | **Informational only** (never checked) |

The `TenantBindingMiddleware` validates slug-to-user permission — which is **correct by design** for the admin UI (scoped admins can switch tenants). The plan's proposed fix ("validate `tenant.ID == tenantID`") would **break legitimate multi-tenant admin workflows**.

---

## Nits on the Original Plan

### Nit 1: P0 Issue #1 Fix Is Over-Broad

**Plan says:** "Every handler must extract `tenantID := getTenantFromContext(c)` and validate `tenant.ID == tenantID` with superuser bypass"

**Problem:** This would prevent scoped admins from accessing tenants they have RBAC permission for but whose `tenant_id` differs from their JWT. The `TenantBindingMiddleware` already handles this correctly at `tenant_binding.go:35-67`.

**Correct fix:** Create `AgentTenantBindingMiddleware` that resolves slug → tenant → RBAC check → sets resolved tenant in context. Keep JWT `tenant_id` as informational.

### Nit 2: P0 Issue #4 Is Under-Scoped

**Plan says:** "11 of 23 List* methods missing `ApplyTenantFilter`"

**Reality:** All 20 tenant-scoped List methods correctly include `tenant_id` in SQL when `find.TenantID` is set. The risk is **caller-side omission**, not missing SQL. The fix should be an `ApplyAgentTenantFilter(ctx, find)` function that injects tenant_id from context into any agent Find struct.

### Nit 3: Missing Finding — O(N×M) Token Scan

`HandleSelectTenant` at `auth_service.go:472-499` performs an O(N×M) scan across all users and all their tokens to find the matching selection token. This is a performance bomb at scale. Should be a direct lookup by token hash.

### Nit 4: Missing Finding — REST SignIn Sets nil tenant

`HandleSignIn` REST endpoint at `auth_service.go:664` always sets `tenant_id=nil`. REST-only users get no tenant context without the separate selection flow. This is by design but should be documented as a known behavior, not a bug.

---

## Recommended Approach

Rather than adding JWT-tenant-vs-slug validation in every handler, implement **middleware-level enforcement**:

1. **Create `AgentTenantBindingMiddleware`** that:
   - Extracts slug from URL
   - Resolves tenant by slug
   - Validates user has RBAC permission (reuses existing `TenantBindingMiddleware` logic)
   - Sets resolved tenant ID in context (so handlers can use it)

2. **Add `ApplyAgentTenantFilter(ctx, find)`** that injects tenant_id from context into any agent Find struct

3. **Keep JWT `tenant_id` as informational** for downstream use (memo queries, logging), not as an authorization boundary

This preserves the existing multi-tenant admin UX while adding defense-in-depth.

---

## Summary Table

| Finding | Plan's Assessment | Correct Assessment | Impact |
|---------|-------------------|---------------------|--------|
| REST flow implemented | ✅ | ✅ Fully implemented | None |
| JWT `tenant_id` set | ✅ | ✅ Set correctly | None |
| JWT `tenant_id` validated against allowed_tenant_ids | ❌ Missing | ❌ Missing, but **by design** — RBAC is the enforcement point | Plan fix is over-broad |
| Agent handlers check JWT vs slug | ❌ Missing | ❌ Missing, but **middleware handles this** | Need middleware-level fix |
| List* methods tenant filter | ❌ 11/23 missing | ✅ All 20 have SQL WHERE when TenantID set | Risk is caller omission, not missing SQL |
| O(N×M) token scan | Not mentioned | ⚠️ Performance risk | Add direct token lookup |

---

**Bottom line:** The plan correctly identifies that JWT `tenant_id` is not an enforcement boundary in agent handlers, but mischaracterizes this as a security bug rather than an architectural gap. The fix should be middleware-level, not handler-level, and should not break the legitimate multi-tenant admin workflow that `TenantBindingMiddleware` already supports.
