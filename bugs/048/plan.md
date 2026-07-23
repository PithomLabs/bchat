# Bug 048: Public/Widget Route Groups Missing Tenant Context Resolution

**Status:** Planning  
**Severity:** Critical — all public-facing endpoints return HTTP 400  
**Created:** 2026-07-24  
**Assigned:** Senior Go Architect  

---

## Executive Summary

Five public-facing HTTP handlers fail with `400 "tenant context not set"` because they call `getTenantOrFail()`, which reads tenant ID from the Echo context (`c.Get("tenant-id")`), but the route groups they belong to never set this context. The `TenantBindingMiddleware` (which sets it) is only applied to the `authGroup` and `adminGroup`. The public and widget groups have no tenant-resolution middleware at all.

This means the **entire external chat flow is broken**: widget embed, iframe, legacy widget, external chat, and external transcript endpoints all return 400 on every request.

---

## Root Cause Analysis

### How tenant context flows in bchat

```
Request → Middleware → c.Set("tenant-id", tenantID) → Handler → getTenantOrFail() → c.Get("tenant-id")
```

### Affected groups vs. working groups

| Route Group | Middleware | Tenant Context Set? | Handlers Working? |
|-------------|-----------|--------------------|--------------------|
| `publicGroup` (`/api/v1/agent`) | CORS + BodyLimit | **NO** | **5 BROKEN** |
| `widgetGroup` (`/widget`) | CORS | **NO** | **2 BROKEN** |
| `bridgeGroup` (`/api/v1/agent/:slug/bridge`) | RequireBridgeHMAC | YES (line 229) | OK |
| `authGroup` (`/api/v1/agent`) | Auth + TenantBinding | YES (line 78) | OK |
| `adminGroup` (`/api/v1/agent`) | Auth + TenantBinding + CSRF | YES (line 78) | OK |
| `ragGroup` (`/api/v1/admin/rag`) | Auth | N/A (direct lookups) | OK |
| `userGroup` (`/api/v1/user`) | Auth | N/A (user-based) | OK |

### Broken handlers (5 total)

| Handler | Route | File:Line | Uses |
|---------|-------|-----------|------|
| `HandleChatExternal` | `POST /api/v1/agent/:slug/chat/ext` | `handlers.go:390` | `getTenantOrFail()` |
| `HandleGetExternalTranscript` | `GET /api/v1/agent/:slug/chat/ext/transcript` | `handlers.go:496` | `getTenantOrFail()` |
| `HandleWidget` (legacy) | `GET /api/v1/agent/:slug/widget.js` | `handlers.go:1739` | `getTenantOrFail()` |
| `HandleWidgetEmbed` | `GET /widget/:slug/embed.js` | `handlers.go:2063` | `getTenantOrFail()` |
| `HandleWidgetIframe` | `GET /widget/:slug/iframe` | `handlers.go:2121` | `getTenantOrFail()` |

### Working handlers in same group (no changes needed)

| Handler | Why It Works |
|---------|-------------|
| `HandlePlaygroundCatalog` | Does not use tenant context at all |
| `HandlePlaygroundRun` | Does its own direct store lookup: `h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})` |
| `HandleBridgeTakeover/Reply/Release` | Bridge HMAC middleware sets context at `bridge_middleware.go:229` |

### Context key compatibility

Both the agent package and v1 package use the same string key `"tenant-id"`:
- `agent/tenant_helpers.go:14` -- `c.Get("tenant-id")`
- `v1/ticket_service.go:452` -- `getTenantIDContextKey()` returns `"tenant-id"`

---

## Fix: Add `ResolveSlugTenantMiddleware`

### Design

Create a lightweight middleware that resolves `:slug` to tenant ID without requiring authentication. This differs from `TenantBindingMiddleware` because:

| Aspect | TenantBindingMiddleware | ResolveSlugTenantMiddleware |
|--------|------------------------|----------------------------|
| Auth required | Yes (reads user from context) | No |
| Permission check | Yes (RBAC) | No |
| Purpose | Auth + tenant scoping | Tenant resolution only |
| Used by | authGroup, adminGroup | publicGroup, widgetGroup |

### Implementation

#### 1. New file: `server/router/api/v1/agent/tenant_resolver.go`

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
            if err != nil || tenant == nil {
                return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
            }
            if !tenant.IsActive {
                return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
            }

            c.Set("tenant-id", tenant.ID)
            return next(c)
        }
    }
}
```

#### 2. Register middleware in `server/router/api/v1/v1.go`

In `RegisterAgentRoutes()`, add the middleware to both groups:

```go
// Public routes (no auth required) - permissive CORS
publicGroup := echoServer.Group("/api/v1/agent")
publicGroup.Use(publicCORS)
publicGroup.Use(middleware.BodyLimit("16KB"))
publicGroup.Use(agent.ResolveSlugTenantMiddleware(s.Store))  // ADD THIS
```

```go
// Widget routes (public, no auth) - permissive CORS for cross-origin script loading
widgetGroup := echoServer.Group("/widget")
widgetPermissiveCORS := middleware.CORSWithConfig(middleware.CORSConfig{
    AllowOrigins: []string{"*"},
    AllowMethods: []string{echo.GET},
    AllowHeaders: []string{echo.HeaderOrigin},
})
widgetGroup.Use(widgetPermissiveCORS)
widgetGroup.Use(agent.ResolveSlugTenantMiddleware(s.Store))  // ADD THIS
```

**NOT added to `bridgeGroup`** -- already handled by `RequireBridgeHMAC` middleware.

### What this fixes

| Endpoint | Before | After |
|----------|--------|-------|
| `POST /api/v1/agent/:slug/chat/ext` | 400 | Works |
| `GET /api/v1/agent/:slug/chat/ext/transcript` | 400 | Works |
| `GET /api/v1/agent/:slug/widget.js` | 400 | Works |
| `GET /widget/:slug/embed.js` | 400 | Works |
| `GET /widget/:slug/iframe` | 400 | Works |
| `GET /api/v1/agent/playground/catalog` | 200 | Still works (no slug, middleware skips) |
| `POST /api/v1/agent/:slug/playground/run` | 200 | Still works (redundant lookup, harmless) |

### No handler changes required

All five broken handlers already call `getTenantOrFail()` which reads `c.Get("tenant-id")`. The middleware sets this value before the handler runs, so no handler code changes are needed.

---

## Files to Modify

| File | Change |
|------|--------|
| `server/router/api/v1/agent/tenant_resolver.go` | **NEW** -- `ResolveSlugTenantMiddleware` |
| `server/router/api/v1/v1.go` | Add middleware to `publicGroup` and `widgetGroup` |

---

## Verification

### 1. Manual smoke test

```bash
# Widget embed -- should return JavaScript, not 400
curl -v http://localhost:8081/widget/rgresidences/embed.js

# Widget iframe -- should return HTML
curl -v http://localhost:8081/widget/rgresidences/iframe

# Legacy widget -- should return JavaScript
curl -v http://localhost:8081/api/v1/agent/rgresidences/widget.js

# External chat -- should accept POST (may fail at widget key level, but not 400)
curl -X POST http://localhost:8081/api/v1/agent/rgresidences/chat/ext \
  -H "Content-Type: application/json" \
  -d '{"message":"hello"}'

# Playground catalog -- should still work
curl http://localhost:8081/api/v1/agent/playground/catalog

# Non-existent tenant -- should return 404
curl http://localhost:8081/widget/nonexistent/embed.js
```

### 2. Unit test

Add `tenant_resolver_test.go` covering:
- Slug present + valid tenant -- context set, next called
- Slug present + tenant not found -- 404
- Slug present + tenant inactive -- 404
- Slug empty (no `:slug` in URL) -- next called without setting context

### 3. Hugo integration

After fix, verify the rgresidences landing page loads the widget:
- `hugo server` at localhost:1313
- Navigate to `http://localhost:1313/rgresidences/`
- Chat widget bubble should appear in bottom-right corner

---

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Middleware adds DB query per request | Single indexed lookup by slug; same cost as existing `TenantBindingMiddleware` |
| Public endpoints now expose tenant lookup | Already public -- no new attack surface |
| `HandlePlaygroundRun` does redundant lookup | Harmless; can optimize later by switching to context |
| Slug injection / path traversal | Echo's `c.Param()` handles URL decoding; store lookup is parameterized SQL |

---

## Implementation Order

1. Create `tenant_resolver.go` with `ResolveSlugTenantMiddleware`
2. Add middleware to `publicGroup` and `widgetGroup` in `v1.go`
3. Run smoke tests
4. Write unit test for the middleware
5. Test Hugo integration
