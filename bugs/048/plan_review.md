# Plan Review: Bug 048 — Public/Widget Route Groups Missing Tenant Context Resolution

**Reviewer:** Senior Go Architect  
**Date:** 2026-07-24  
**Status:** Approved with Nits

---

## Verdict

The plan correctly identifies the root cause (missing tenant context middleware on `publicGroup` and `widgetGroup`) and proposes a sensible, minimal fix (create `ResolveSlugTenantMiddleware`, add it to both groups, change zero handlers). The overall approach is sound and consistent with existing patterns. Approve with the two substantive nits below.

---

## Nit 1: DB Errors Return 404 Instead of 500

**File:** `tenant_resolver.go:107-108`

```go
tenant, err := dbStore.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
if err != nil || tenant == nil {
    return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
}
```

Returning 404 when `err != nil` conflates "tenant not found" with "database unavailable / query failed". A transient DB failure should surface as 500, not 404. This pattern is inherited from `TenantBindingMiddleware` (`tenant_binding.go:56`) where it is also a preexisting smell, but the fix here should not propagate it.

**Recommendation:** Separate the two cases:

```go
tenant, err := dbStore.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
if err != nil {
    return echo.NewHTTPError(http.StatusInternalServerError, "Failed to resolve agent")
}
if tenant == nil {
    return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
}
```

---

## Nit 2: Magic String `"tenant-id"` Key

**File:** `tenant_resolver.go:114`

```go
c.Set("tenant-id", tenant.ID)
```

The agent package cannot import `v1.getTenantIDContextKey()` without creating a circular import (`v1` imports `agent`). However, hardcoding a magic string is fragile — if the canonical key ever changes in `v1/tenant_context.go`, the agent package will silently break.

**Recommendation:** Add a package-level constant:

```go
// tenantContextKey is the Echo context key for tenant ID.
// Must match v1.getTenantIDContextKey() in server/router/api/v1/ticket_service.go.
const tenantContextKey = "tenant-id"
```

Then use `c.Set(tenantContextKey, tenant.ID)`. This isolates the magic string to one declaration and makes the coupling explicit in a comment.

---

## Minor Observations

### OPTIONS preflight behavior change

The middleware runs before `publicGroup.OPTIONS("/:slug/*")`. Currently, OPTIONS for any slug (even non-existent) returns 204. After the fix, OPTIONS for a non-existent slug returns 404. This is technically correct — there is no reason to CORS-preflight a tenant that doesn't exist — but should be called out in the PR description as an intentional behavioral change.

### `HandlePlaygroundRun` redundant lookup

The plan correctly notes that `HandlePlaygroundRun` does its own direct store lookup instead of reading from context. This is harmless today but should have a `// TODO` comment at the route registration in `v1.go` documenting the optimization opportunity.

### Widget error response content-type mismatch

When the middleware returns 404 for a non-existent tenant on a widget route (e.g., `GET /widget/:slug/embed.js`), Echo renders the error using its default format (JSON or plain text). The handler normally returns `application/javascript`. The HTTP status is correct but integrators debugging missing tenants may see a content-type they don't expect. Worth noting in the PR description but not actionable here.

### Missing context-propagation test

The unit test plan covers `slug → 404` and `empty slug → skip` but should also verify: when a valid tenant is resolved, `c.Get("tenant-id")` equals the expected ID inside `next()`. This is the core correctness property.

---

## Summary

| Category | Count | Items |
|----------|-------|-------|
| Critical | 0 | — |
| Nits | 2 | DB error conflated with not-found; magic string key |
| Minors | 4 | OPTIONS preflight change; PlaygroundRun TODO; widget content-type; context-propagation test |
