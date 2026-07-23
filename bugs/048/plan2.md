# Bug 048 Plan v2: Incorporating Review Findings

**Status:** Planning  
**Created:** 2026-07-24  
**Based on:** plan.md + plan_review.md  

---

## Review Findings Assessment

| Finding | Verdict | Action |
|---------|---------|--------|
| Nit 1: DB errors return 404 instead of 500 | **Valid** | Implement |
| Nit 2: Magic string "tenant-id" key | **Valid** | Implement |
| Minor 1: OPTIONS preflight behavior change | **Valid** | Document in PR |
| Minor 2: HandlePlaygroundRun redundant lookup | **Valid** | Add TODO comment |
| Minor 3: Widget error content-type mismatch | **Not actionable** | Skip |
| Minor 4: Missing context-propagation test | **Valid** | Add to test plan |

---

## Changes to plan.md

### Change 1: Separate DB errors from not-found (Nit 1)

In `tenant_resolver.go`, replace:

```go
tenant, err := dbStore.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
if err != nil || tenant == nil {
    return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
}
```

With:

```go
tenant, err := dbStore.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
if err != nil {
    return echo.NewHTTPError(http.StatusInternalServerError, "Failed to resolve agent")
}
if tenant == nil {
    return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
}
```

This prevents transient DB failures from being silently swallowed as 404s.

### Change 2: Add constant for tenant context key (Nit 2)

At the top of `tenant_resolver.go`, add:

```go
// tenantContextKey is the Echo context key for tenant ID.
// Must match v1.getTenantIDContextKey() in server/router/api/v1/ticket_service.go.
const tenantContextKey = "tenant-id"
```

Then use `c.Set(tenantContextKey, tenant.ID)` instead of the magic string.

### Change 3: OPTIONS preflight behavior (Minor 1)

After adding the middleware, the OPTIONS route handler at `v1.go:293`:

```go
publicGroup.OPTIONS("/:slug/*", func(c echo.Context) error {
    return c.NoContent(http.StatusNoContent)
})
```

will no longer run for non-existent tenants — the middleware returns 404 first. This is correct behavior (no reason to CORS-preflight a non-existent tenant) but should be documented in the PR description as an intentional behavioral change.

No code change needed — just a PR description note.

### Change 4: Add TODO for PlaygroundRun (Minor 2)

At `v1.go:299`, add a TODO comment:

```go
publicGroup.POST("/:slug/playground/run", s.agentHandler.HandlePlaygroundRun) // TODO: can use context instead of direct lookup after ResolveSlugTenantMiddleware is added
```

### Change 5: Expand unit test plan (Minor 4)

Add to the verification section:

**Context-propagation test**: When a valid tenant is resolved, assert `c.Get("tenant-id")` equals the expected tenant ID inside the `next()` handler. This is the core correctness property.

```go
func TestResolveSlugTenantMiddleware_SetsContext(t *testing.T) {
    // ... setup store with test tenant ...
    
    var capturedTenantID int32
    next := func(c echo.Context) error {
        capturedTenantID = c.Get("tenant-id").(int32)
        return c.NoContent(http.StatusOK)
    }
    
    // ... create request with :slug param ...
    middleware(next)(c)
    
    assert.Equal(t, testTenant.ID, capturedTenantID)
}
```

---

## Updated Files to Modify

| File | Change |
|------|--------|
| `server/router/api/v1/agent/tenant_resolver.go` | **NEW** — with Nit 1 + Nit 2 fixes |
| `server/router/api/v1/v1.go` | Add middleware to `publicGroup` + `widgetGroup`; add TODO comment at PlaygroundRun |

---

## Updated Implementation Order

1. Create `tenant_resolver.go` with:
   - `tenantContextKey` constant (Nit 2)
   - Separated DB error handling (Nit 1)
2. Add middleware to `publicGroup` and `widgetGroup` in `v1.go`
3. Add TODO comment at `HandlePlaygroundRun` route (Minor 2)
4. Run smoke tests
5. Write unit tests including context-propagation test (Minor 4)
6. Document OPTIONS preflight change in PR description (Minor 1)
7. Test Hugo integration
