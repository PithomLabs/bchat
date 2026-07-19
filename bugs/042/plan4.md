# Plan4: Fix CORS Preflight Failure — "Something went wrong" Chat Widget Error

## Root Cause

Two issues in `server/router/api/v1/v1.go` prevent the chat widget from working cross-origin:

### Problem 1: OPTIONS preflight goes to gRPC gateway

The `publicGroup` only registers `POST` for `/:slug/chat/ext` (line 264). When the browser sends an OPTIONS preflight, Echo has no OPTIONS route on `publicGroup`, so the request falls through to `gwGroup.Any("/api/v1/*")` (line 207). The gRPC gateway's auth interceptor rejects it with 401 "Missing access token".

**Evidence:**
```
$ curl -v -X OPTIONS http://localhost:8081/api/v1/agent/bchat/chat/ext \
  -H "Origin: http://localhost:1313" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type, X-Widget-Key"
< HTTP/1.1 401 Unauthorized
"error": "code=401, message=Missing access token"
```

The POST itself works (200 with CORS headers) because it matches the publicGroup route directly. But the browser never sends the POST because the preflight fails first.

### Problem 2: `X-Widget-Key` not in CORS AllowHeaders

The embed.js sends `X-Widget-Key` as a custom header (`widget/site/embed.min.js`):
```javascript
t.widgetKey && (o["X-Widget-Key"] = t.widgetKey);
```

Custom headers trigger a CORS preflight. The `publicCORS` config (line 249) only allows:
```go
AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization}
```

`X-Widget-Key` is missing. Even if Problem 1 is fixed, the browser would reject the response because the server doesn't declare `X-Widget-Key` as an allowed header.

## Fix

Two changes in `server/router/api/v1/v1.go`:

### Fix 1: Add `X-Widget-Key` to `publicCORS` AllowHeaders (line 249)

```go
// Before:
AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},

// After:
AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-Widget-Key"},
```

### Fix 2: Add OPTIONS catch-all on `publicGroup` (after line 262)

```go
// After publicGroup.Use(middleware.BodyLimit("16KB")):
publicGroup.Any("/:slug/*", func(c echo.Context) error {
    return c.NoContent(http.StatusOK)
})
```

This catches ALL requests (including OPTIONS) under `/api/v1/agent/:slug/*` on the `publicGroup`. The `publicCORS` middleware runs first, handles the OPTIONS preflight, and returns proper CORS headers. The gRPC gateway never sees these requests.

## Why This Works

1. Browser sends OPTIONS preflight to `/api/v1/agent/bchat/chat/ext`
2. Echo matches `publicGroup.Any("/:slug/*")` — more specific than `gwGroup.Any("/api/v1/*")`
3. `publicCORS` middleware runs, sets `Access-Control-Allow-Origin: *`, `Access-Control-Allow-Headers: ..., X-Widget-Key`
4. Handler returns 200
5. Browser sends the actual POST with `X-Widget-Key` header
6. Echo matches `publicGroup.POST("/:slug/chat/ext")`
7. `publicCORS` middleware runs, sets CORS headers
8. `HandleChatExternal` processes the chat

## Verification

After applying the fix:
```bash
# Should return 200 with CORS headers (not 401)
curl -v -X OPTIONS http://localhost:8081/api/v1/agent/bchat/chat/ext \
  -H "Origin: http://localhost:1313" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type, X-Widget-Key"

# Should show Access-Control-Allow-Headers including X-Widget-Key
# Should show Access-Control-Allow-Origin: *
```

## Files Changed

- `server/router/api/v1/v1.go` — two edits (AllowHeaders + OPTIONS catch-all)
