# Plan Review: Plan4 — Fix CORS Preflight Failure

## Overview

Two CORS issues prevent cross-origin widget chat from working. Evidence summary:

| Evidence | Location |
|----------|----------|
| Widget sends `X-Widget-Key` header | `widget/src/core/api.ts:18`, `widget/site/embed.min.js:1` |
| Server validates `X-Widget-Key` | `handlers.go:427` |
| Widget key injected into embed.js | `handlers.go:2104` |
| `publicCORS` AllowHeaders omits `X-Widget-Key` | `v1.go:249` |
| `publicGroup` has no OPTIONS route for `/:slug/chat/ext` | `v1.go:260-267` |
| `gwGroup.Any("/api/v1/*")` catches unhandled methods | `v1.go:207` |

---

## Problem 1: OPTIONS Falls Through to gRPC Gateway — CONFIRMED

**Root cause:** Echo's router stores routes per-method. The `publicGroup` (v1.go:260) registers `POST` and `GET` but never `OPTIONS`. When a browser sends `OPTIONS /api/v1/agent/bchat/chat/ext`, Echo's router looks in the OPTIONS method tree:

1. No `OPTIONS` entry for the specific path `/api/v1/agent/:slug/chat/ext`
2. Finds `OPTIONS /api/v1/*` registered at `v1.go:207` via `gwGroup.Any("/api/v1/*")`
3. Routes to gRPC gateway returns 401 "Missing access token"

**Echo route tree (simplified):**

```
OPTIONS tree:
  /api/v1/*          gwGroup handler (gRPC gateway, 401)

POST tree:
  /api/v1/agent/:slug/chat/ext  publicGroup handler (HandleChatExternal, 200)

GET tree:
  /api/v1/agent/:slug/chat/ext/transcript  publicGroup handler (200)
  /api/v1/agent/:slug/widget.js            publicGroup handler (200)
```

The POST itself works (200 with CORS headers), but the browser never sends it because the OPTIONS preflight fails first.

---

## Problem 2: `X-Widget-Key` Missing from AllowHeaders — CONFIRMED

The widget authenticates each chat request via the `X-Widget-Key` header:

- **Widget source:** `widget/src/core/api.ts:17-18` headers['X-Widget-Key'] = config.widgetKey
- **Server validation:** `handlers.go:427` c.Request().Header.Get("X-Widget-Key")
- **Config injection:** `handlers.go:2104` window.AgentChatConfig.widgetKey=... injected into embed.js

The `publicCORS` config at `v1.go:249` only declares:

```go
AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization}
```

Without `X-Widget-Key` in `Access-Control-Allow-Headers`, the browser blocks the actual POST even if the OPTIONS preflight were to succeed.

---

## Fix 1: Add `X-Widget-Key` to AllowHeaders — APPROVED

**Correct.** Add `"X-Widget-Key"` to the `AllowHeaders` slice at `v1.go:249`.

---

## Fix 2: OPTIONS Catch-All — APPROVED (with improvement)

The plan proposes `Any("/:slug/*")`. This works but is too broad:

```go
publicGroup.Any("/:slug/*", func(c echo.Context) error {
    return c.NoContent(http.StatusOK)
})
```

**Problem:** `Any` registers handlers for ALL HTTP methods (GET, POST, PUT, DELETE, OPTIONS, etc.). For a `GET /api/v1/agent/bchat/nonexistent` request (unknown path, no handler), this catch-all matches and returns **200** instead of the correct **404**. This silently swallows routing bugs.

**Better fix use `OPTIONS` instead of `Any`:**

```go
publicGroup.OPTIONS("/:slug/*", func(c echo.Context) error {
    return c.NoContent(http.StatusNoContent)
})
```

**Why this is better:**
- Only catches `OPTIONS` requests for unknown paths under `/:slug/*`
- Non-OPTIONS requests to unknown paths still return proper 404s
- The `publicCORS` middleware (registered via `publicGroup.Use(publicCORS)` at line 261) runs before this handler and handles the preflight, returning 204 with CORS headers the handler itself is never reached for OPTIONS

**How it works (after fix):**
1. Browser sends `OPTIONS /api/v1/agent/bchat/chat/ext`
2. Echo matches `OPTIONS /api/v1/agent/:slug/*` on publicGroup
3. `publicCORS` middleware runs: checks origin, sets `Access-Control-Allow-Origin: *`, `Access-Control-Allow-Headers: ..., X-Widget-Key`
4. CORS middleware returns 204 No Content handler is not reached
5. Browser sees preflight success, sends actual `POST /api/v1/agent/bchat/chat/ext`
6. Echo matches `POST /api/v1/agent/:slug/chat/ext` on publicGroup
7. `publicCORS` middleware sets CORS headers (for response visibility)
8. `HandleChatExternal` processes the request

---

## Edge Cases Considered

| Scenario | Behavior with OPTIONS-only catch-all |
|----------|---------------------------------------|
| `OPTIONS /api/v1/agent/bchat/chat/ext` | Matches `/:slug/*` CORS middleware returns 204 |
| `OPTIONS /api/v1/agent/bchat/nonexistent` | Matches `/:slug/*` CORS middleware returns 204 (harmless) |
| `POST /api/v1/agent/bchat/chat/ext` (actual request) | Matches specific POST route 200 |
| `POST /api/v1/agent/bchat/nonexistent` | No POST match, no POST catch-all 404 |
| `OPTIONS /api/v1/auth/signin` | Does not match `/:slug/*` prefix falls to gwGroup catch-all (acceptable auth routes have own CORS) |

---

## Decision

| Item | Verdict |
|------|---------|
| Problem 1 analysis | **Confirmed** |
| Problem 2 analysis | **Confirmed** |
| Fix 1 (X-Widget-Key) | **Approved** |
| Fix 2 (Any catch-all) | **Approved with improvement** use `OPTIONS` instead of `Any` |

**Overall: APPROVED WITH NITS** replace `publicGroup.Any("/:slug/*")` with `publicGroup.OPTIONS("/:slug/*")` for tighter scope.
