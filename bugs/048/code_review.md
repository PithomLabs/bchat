# Code Review: Bug 048 — Public/Widget Route Groups Missing Tenant Context Resolution

**Reviewer:** Senior Go Architect  
**Date:** 2026-07-24  
**Status:** Approved with Nits

---

## Verdict

The implementation is correct, well-tested, and fixes all five broken endpoints. The code matches the plan and the verification results confirm the fix works. One new nit surfaced during code review that should be fixed before merging.

---

## Nit: `bridge_middleware.go:229` Hardcodes `"tenant-id"`

**File:** `server/router/api/v1/agent/bridge_middleware.go:229`

```go
c.Set("tenant-id", tenant.ID)
```

This is in the same `agent` package as the new `tenantContextKey` constant (`tenant_helpers.go:13`). It should use the constant:

```go
c.Set(tenantContextKey, tenant.ID)
```

Add `bridge_middleware.go` to the scope. This is a trivially correct, one-line find-and-replace.

---

## Minor Observations

### Error message style inconsistency within `tenant_resolver.go`

| Line | Message | Style |
|------|---------|-------|
| 28 | `"failed to resolve agent"` | Lowercase (matches `tenant_binding.go`) |
| 31, 34 | `"Agent not found"` | Title-case (matches `integrations.go`: `"Tenant not found"`) |

Not a correctness issue — both styles exist in the codebase. Noted for reviewer awareness.

### Test files hardcode `"tenant-id"` in 18 places

- `role_template_handler_test.go` — 10 occurrences of `c.Set("tenant-id", tenant.ID)`
- `bridge_delivery_test.go` — 8 occurrences

These test files are in the `agent` package and can access `tenantContextKey`. Updating them would be consistent but is not required for correctness — these tests are about other features and the hardcoded key is functionally identical.

### Empty-slug test missing negative assertion

`TestResolveSlugTenantMiddleware_EmptySlugSkipsResolution` does not verify the context key is absent after middleware runs. Adding `require.Nil(t, c.Get(tenantContextKey))` after the handler call would strengthen the test.

---

## Security Review

| Concern | Assessment |
|---------|-----------|
| New attack surface | None — middleware only resolves slug to ID, no auth/authorization |
| Slug injection | Echo's router handles URL path traversal; store lookup uses parameterized SQL |
| Inactive tenant leakage | Returns same 404 as non-existent tenant — no information disclosure |
| Error message leakage | No tenant IDs, slugs, or internal state exposed in error messages |
| Cross-tenant access | Middleware sets a single tenant ID per request; handlers scope queries to that ID |
| Auth bypass | Public endpoints were already public; middleware only enables them to function |

---

## Completeness Check

| Route Group | Middleware | Status |
|-------------|-----------|--------|
| `publicGroup` | `ResolveSlugTenantMiddleware` | ✅ Added |
| `widgetGroup` | `ResolveSlugTenantMiddleware` | ✅ Added |
| `bridgeGroup` | `RequireBridgeHMAC` (own context) | ✅ Excluded by design |
| `authGroup` | `TenantBindingMiddleware` | ✅ Already had it |
| `adminGroup` | `TenantBindingMiddleware` | ✅ Already had it |
| `ragGroup` | Direct ID lookup | ✅ No slug needed |
| `userGroup` | User-based | ✅ No tenant slug needed |
| Standalone routes | Auth endpoints, system cron | ✅ No tenant context needed |

---

## Test Coverage

| Test | Scenario | Verdict |
|------|----------|---------|
| `TestResolveSlugTenantMiddleware_ValidTenant` | Valid slug → context set correctly, 200 | ✅ |
| `TestResolveSlugTenantMiddleware_TenantNotFound` | Non-existent slug → 404 | ✅ |
| `TestResolveSlugTenantMiddleware_InactiveTenant` | Inactive tenant → 404 | ✅ |
| `TestResolveSlugTenantMiddleware_EmptySlugSkipsResolution` | No slug param → middleware skips, next called | ✅ (*) |

(*) Missing negative assertion on context key absence.

---

## Summary

| Category | Count | Detail |
|----------|-------|--------|
| Nits | 1 | `bridge_middleware.go:229` — use `tenantContextKey` constant |
| Minors | 3 | Error style inconsistency; test file hardcodes; empty-slug test assertion |
| Security | 0 | Clean |
| Completeness | 0 | All groups accounted for |
