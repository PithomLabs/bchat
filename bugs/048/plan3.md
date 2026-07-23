# Bug 048 Plan v3: Incorporating plan2_review.md Findings

**Status:** Planning
**Created:** 2026-07-24
**Based on:** plan2.md + plan2_review.md

---

## Review Findings Assessment

| Finding | Verdict | Action |
|---------|---------|--------|
| Nit 1: `tenantContextKey` should be shared with `tenant_helpers.go` | **Valid** | Implement |
| Nit 2: Error message capitalization `"Failed"` -> `"failed"` | **Valid** | Implement |
| Minor: Test snippet uses magic string | **Valid** | Self-fixes with Nit 1 |
| Minor: DB error test case not covered | **Accepted** | No action (mocking infra absent) |
| Minor: No test file path in plan | **Valid** | Add `tenant_resolver_test.go` |

---

## Changes to plan2.md

### Change 1: Define constant in `tenant_helpers.go`, use everywhere (Nit 1)

Define the constant once in `tenant_helpers.go` (the existing file that already hardcodes `"tenant-id"`). Both `tenant_resolver.go` and `tenant_helpers.go` are in the `agent` package, so the constant is shared.

**In `tenant_helpers.go`** — add constant and update `getTenantFromContext`:

```go
// tenantContextKey is the Echo context key for tenant ID.
// Must match v1.getTenantIDContextKey() in server/router/api/v1/ticket_service.go.
const tenantContextKey = "tenant-id"

func getTenantFromContext(c echo.Context) *int32 {
    if v, ok := c.Get(tenantContextKey).(int32); ok {
        return &v
    }
    return nil
}
```

**In `tenant_resolver.go`** — use the shared constant (no local definition):

```go
c.Set(tenantContextKey, tenant.ID)
```

This keeps the magic string in exactly one place across the entire `agent` package.

### Change 2: Lowercase error message (Nit 2)

In `tenant_resolver.go`, use lowercase to match `tenant_helpers.go:25`:

```go
return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve agent")
```

Not `"Failed to resolve agent"`.

**Note on codebase convention:** The agent package has mixed conventions — `handlers.go` uses title case (100+ instances), while the tenant-binding layer (`tenant_helpers.go`, `tenant_binding.go`) uses lowercase. Since the new middleware lives in the tenant-binding layer, lowercase is the correct match.

### Change 3: Test file path and constant usage (Minor)

Specify the test file path: `server/router/api/v1/agent/tenant_resolver_test.go`

Update the test snippet to use the shared constant:

```go
capturedTenantID = c.Get(tenantContextKey).(int32)
```

---

## Updated Files to Modify

| File | Change |
|------|--------|
| `server/router/api/v1/agent/tenant_helpers.go` | Add `tenantContextKey` constant; update `getTenantFromContext` to use it |
| `server/router/api/v1/agent/tenant_resolver.go` | **NEW** — middleware using shared constant + lowercase error |
| `server/router/api/v1/v1.go` | Add middleware to `publicGroup` + `widgetGroup`; add TODO comment at PlaygroundRun |
| `server/router/api/v1/agent/tenant_resolver_test.go` | **NEW** — unit tests including context-propagation test |

---

## Updated Implementation Order

1. Add `tenantContextKey` constant to `tenant_helpers.go`; update `getTenantFromContext` to reference it
2. Create `tenant_resolver.go` with `ResolveSlugTenantMiddleware` using shared constant + lowercase error
3. Add middleware to `publicGroup` and `widgetGroup` in `v1.go`
4. Add TODO comment at `HandlePlaygroundRun` route
5. Create `tenant_resolver_test.go` with unit tests (including context-propagation test)
6. Run smoke tests
7. Test Hugo integration
8. Document OPTIONS preflight behavioral change in PR description
