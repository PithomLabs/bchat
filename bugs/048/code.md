# Bug 048: Implementation — Public/Widget Route Groups Missing Tenant Context Resolution

**Status:** Implemented  
**Date:** 2026-07-24  
**Author:** Senior Go Architect  

---

## Problem

Five public-facing HTTP handlers returned `400 "tenant context not set - middleware may not be configured correctly"` because they called `getTenantOrFail()`, which reads tenant ID from the Echo context (`c.Get("tenant-id")`), but the route groups they belonged to never set this context.

The `TenantBindingMiddleware` (which sets tenant context) was only applied to `authGroup` and `adminGroup`. The `publicGroup` and `widgetGroup` had no tenant-resolution middleware.

## Broken Endpoints (Before Fix)

| Endpoint | Error |
|----------|-------|
| `GET /widget/:slug/embed.js` | 400 |
| `GET /widget/:slug/iframe` | 400 |
| `GET /api/v1/agent/:slug/widget.js` | 400 |
| `POST /api/v1/agent/:slug/chat/ext` | 400 |
| `GET /api/v1/agent/:slug/chat/ext/transcript` | 400 |

## Solution

Add a lightweight `ResolveSlugTenantMiddleware` that resolves `:slug` → tenant ID without requiring authentication. This middleware runs before handlers and sets the `tenant-id` context key, which `getTenantOrFail()` reads.

---

## Files Changed

### 1. `server/router/api/v1/agent/tenant_helpers.go`

Added `tenantContextKey` constant and updated `getTenantFromContext` to reference it.

**Before:**
```go
func getTenantFromContext(c echo.Context) *int32 {
    if v, ok := c.Get("tenant-id").(int32); ok {
        return &v
    }
    return nil
}
```

**After:**
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

### 2. `server/router/api/v1/agent/tenant_resolver.go` (NEW)

```go
package agent

import (
    "net/http"

    "github.com/labstack/echo/v4"

    "github.com/usememos/memos/store"
)

// ResolveSlugTenantMiddleware resolves the :slug URL parameter to a tenant ID
// and sets it in the Echo context. Used for public routes that don't require
// authentication but need tenant context (e.g., widget, external chat).
//
// This middleware is intentionally lightweight: no auth, no permission check.
// Downstream handlers must perform their own authorization checks.
func ResolveSlugTenantMiddleware(dbStore *store.Store) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            slug := c.Param("slug")
            if slug == "" {
                // Routes without :slug (e.g., /playground/catalog) skip resolution
                return next(c)
            }

            tenant, err := dbStore.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
            if err != nil {
                return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve agent")
            }
            if tenant == nil {
                return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
            }
            if !tenant.IsActive {
                return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
            }

            c.Set(tenantContextKey, tenant.ID)
            return next(c)
        }
    }
}
```

### 3. `server/router/api/v1/v1.go`

Added middleware registration to `publicGroup` and `widgetGroup`. Added TODO comment at `HandlePlaygroundRun` route.

**publicGroup change:**
```go
// Before
publicGroup.Use(publicCORS)
publicGroup.Use(middleware.BodyLimit("16KB"))
publicGroup.OPTIONS("/:slug/*", ...)

// After
publicGroup.Use(publicCORS)
publicGroup.Use(middleware.BodyLimit("16KB"))
publicGroup.Use(agent.ResolveSlugTenantMiddleware(s.Store))
publicGroup.OPTIONS("/:slug/*", ...)
```

**widgetGroup change:**
```go
// Before
widgetGroup.Use(widgetPermissiveCORS)
widgetGroup.GET("/:slug/embed.js", ...)

// After
widgetGroup.Use(widgetPermissiveCORS)
widgetGroup.Use(agent.ResolveSlugTenantMiddleware(s.Store))
widgetGroup.GET("/:slug/embed.js", ...)
```

**PlaygroundRun TODO:**
```go
// Before
publicGroup.POST("/:slug/playground/run", s.agentHandler.HandlePlaygroundRun)

// After
publicGroup.POST("/:slug/playground/run", s.agentHandler.HandlePlaygroundRun) // TODO: can use context instead of direct lookup after ResolveSlugTenantMiddleware
```

### 4. `server/router/api/v1/agent/tenant_resolver_test.go` (NEW)

Four unit tests covering:
- Valid tenant → context set correctly, 200
- Tenant not found → 404
- Inactive tenant → 404
- Empty slug (no `:slug` param) → middleware skips, next called

---

## Verification Results

### Unit Tests
```
=== RUN   TestResolveSlugTenantMiddleware_ValidTenant          --- PASS
=== RUN   TestResolveSlugTenantMiddleware_TenantNotFound       --- PASS
=== RUN   TestResolveSlugTenantMiddleware_InactiveTenant       --- PASS
=== RUN   TestResolveSlugTenantMiddleware_EmptySlugSkipsResolution --- PASS
ok   ...agent   0.197s
```

### Build
```
go build ./...  →  clean, no errors
```

### Smoke Tests (Live Server)

| Endpoint | Before | After |
|----------|--------|-------|
| `GET /widget/rgresidences/embed.js` | 400 | **200** |
| `GET /widget/rgresidences/iframe` | 400 | **200** |
| `GET /api/v1/agent/rgresidences/widget.js` | 400 | **200** |
| `POST /api/v1/agent/rgresidences/chat/ext` | 400 | **403** (expected — no widget key) |
| `GET /api/v1/agent/playground/catalog` | 200 | **200** (unchanged) |
| `GET /widget/nonexistent/embed.js` | — | **404** (correct) |

---

## Behavioral Changes

| Change | Rationale |
|--------|-----------|
| OPTIONS preflight for non-existent slugs now returns 404 instead of 204 | No reason to CORS-preflight a non-existent tenant |
| `chat/ext` returns 403 instead of 400 | Tenant resolves correctly; 403 is from widget key validation (correct behavior) |

---

## Code Review Request

Please review the following implementation for correctness, security, and code quality:

1. **`tenant_resolver.go`** — Does the middleware correctly resolve tenants by slug? Are error cases handled properly (DB error vs not-found vs inactive)?

2. **`tenant_helpers.go`** — Is the `tenantContextKey` constant correctly placed? Does the reference to `v1.getTenantIDContextKey()` in the comment accurately describe the coupling?

3. **`v1.go`** — Is the middleware registered in the correct position (after CORS/BodyLimit, before route handlers)? Are both `publicGroup` and `widgetGroup` covered?

4. **`tenant_resolver_test.go`** — Are the test cases sufficient? Is the test setup pattern consistent with existing tests in the package?

5. **Security** — Does this introduce any new attack surface? Are public endpoints correctly scoped (no auth bypass, no cross-tenant data leakage)?

6. **Missing items** — Are there any other route groups or handlers that need this middleware but were missed?
