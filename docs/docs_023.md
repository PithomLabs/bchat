# SPA Static Asset Serving Fix (2026-07-06)

## Summary

Fixed a critical bug where all Vite-hashed JavaScript assets (e.g., `RagStats.XaXVsIZb.js`, `SignIn.CN5M3ne1.js`) returned `index.html` instead of their actual content, causing `Failed to fetch dynamically imported module` errors in the browser. The fix was a single-line change in `server/router/frontend/frontend.go`. Also fixed a Taskfile race condition where parallel build deps could cause the Go binary to embed stale frontend assets.

---

## Symptoms

### fly.io Production Error

```
Failed to fetch dynamically imported module: https://bchat0534.fly.dev/assets/SignIn.DjUcLEAY.js
```

### Local Development Error

```
Failed to fetch dynamically imported module: http://localhost:8081/assets/RagStats.XaXVsIZb.js
```

### Root Observation

Every request to a Vite-hashed asset (`.js` in `assets/`) returned the HTML content of `index.html` with a `200 OK` status. The browser received HTML where it expected JavaScript, causing a MIME type mismatch and module loading failure.

---

## Root Cause: `c.Path()` vs `c.Request().URL.Path`

### The Bug

In `server/router/frontend/frontend.go:42`, the frontend middleware used `c.Path()` to determine the request path:

```go
reqPath := c.Path()  // BUG: returns route pattern, not URL path
```

### Why This Fails

In Echo v4, `c.Path()` returns the **registered route pattern** — not the actual HTTP request URL. This is set by the router's `Find()` method after route matching:

| Scenario | `c.Path()` returns | `c.Request().URL.Path` returns |
|----------|-------------------|-------------------------------|
| Matched route `GET /api/v1/agent/:slug/chat` | `/api/v1/agent/:slug/chat` | `/api/v1/agent/acme-corp/chat` |
| No route matched (static asset) | `""` (empty string) | `/assets/RagStats.XaXVsIZb.js` |
| Middleware (pre-routing) | `""` (empty string) | `/anything` |

From Echo's source code (`context.go:320-322`):

```go
func (c *context) Path() string {
    return c.path
}
```

The `path` field is set by the router (`router.go:758`):

```go
ctx.path = rPath  // rPath is the REGISTERED pattern, not the request URL
```

### The Cascade

For a request to `/assets/RagStats.XaXVsIZb.js`:

1. **Router runs**: No route registered for `/assets/*` → `c.path = ""`
2. **Global middleware executes** (including frontend middleware)
3. **`c.Path()`** returns `""` (empty string)
4. **`reqPath = ""`** — API skip check passes (not `/api` or `/memos.api.v1`)
5. **`filePath = strings.TrimPrefix("", "/")`** → `""`
6. **`filePath == ""`** → set to `"index.html"`
7. **`distFS.Open("index.html")`** → succeeds
8. **Serves `index.html`** instead of the requested JS file
9. **Browser receives HTML** for a JavaScript request → MIME error → `Failed to fetch dynamically imported module`

### Why SPA Fallback Worked Before

With the original `HTML5: true` in Echo's static middleware, SPA fallback happened inside Echo's built-in handler. The custom middleware replicated this behavior but inherited the same `c.Path()` semantics.

### Key Insight

**`c.Path()` is designed for route handlers**, not middleware. In middleware:
- Use `c.Request().URL.Path` for the actual HTTP request path
- Use `c.Path()` only when you need the registered route pattern (e.g., for parameterized routes)

---

## Root Cause: Taskfile Race Condition

### The Problem

In `Taskfile.yml`, `build:rag:all` listed `build:frontend` and `build:backend:rag` as parallel dependencies:

```yaml
build:rag:all:
    deps: [build:frontend, build:backend:rag, build:widget]
```

Taskfile v3 runs `deps` in parallel by default. The Go binary uses `//go:embed dist/*` to capture frontend assets at compile time. If `build:backend:rag` finishes before `build:frontend`, the binary embeds **stale** `dist/` files.

### Evidence

| Artifact | Timestamp | Notes |
|----------|-----------|-------|
| `web/dist/assets/` | Jul 6 19:06 | Source of truth (vite build output) |
| `server/router/frontend/dist/assets/` | Jul 6 19:23 | Copy from `nub run release` |
| `build/memos` | Jul 6 19:23 | Go binary with embedded dist |

In this specific build, the timing worked out correctly. But the race condition is non-deterministic — on slower machines or with different load, `build:backend:rag` could finish first.

### Hash Mismatch Evidence

The fly.io error referenced `SignIn.DjUcLEAY.js` while the current dist contained `SignIn.CN5M3ne1.js`. This hash difference indicates the deployed binary embedded an older version of the frontend.

### Why `run:rag` Also Had Issues

```yaml
run:rag:
    deps: [build:backend:rag]  # Missing build:frontend dependency
```

Running `task build:frontend` followed by `task run:rag` would rebuild the backend but not guarantee the frontend dist was current.

---

## The Fix

### 1. Frontend Middleware (`server/router/frontend/frontend.go`)

**Before:**
```go
reqPath := c.Path()
```

**After:**
```go
reqPath := c.Request().URL.Path
```

**Why this works:** `c.Request().URL.Path` always returns the actual HTTP request URL path, regardless of route matching. For `/assets/RagStats.XaXVsIZb.js`, it returns exactly that string, allowing the middleware to find and serve the correct file.

### 2. Taskfile Race Condition (`Taskfile.yml`)

**Before:**
```yaml
build:backend:rag:
    desc: Build the Go binary with LanceDB RAG support
    deps: [setup:lancedb, validate:migrations]
```

**After:**
```yaml
build:backend:rag:
    desc: Build the Go binary with LanceDB RAG support
    deps: [build:frontend, setup:lancedb, validate:migrations]
```

**Why this works:** Adding `build:frontend` as a dependency ensures the frontend dist is built and copied to `server/router/frontend/dist/` **before** the Go binary is compiled and embeds those files. Taskfile deduplicates shared deps, so `build:rag:all` still works correctly.

---

## Verification

### Test Static Asset Serving

```bash
# Start server
task run:rag

# Test Vite-hashed asset (should return JS, not HTML)
curl -s -o /dev/null -w "HTTP %{http_code}, Content-Type: %{content_type}, Size: %{size_download}\n" \
  http://localhost:8081/assets/RagStats.XaXVsIZb.js
# Expected: HTTP 200, Content-Type: text/javascript; charset=utf-8, Size: ~14000

# Verify content is valid JavaScript (not HTML)
curl -s http://localhost:8081/assets/RagStats.XaXVsIZb.js | head -1
# Expected: starts with "import" or JavaScript code, NOT "<!doctype html>"
```

### Test SPA Fallback

```bash
# SPA route (should serve index.html for client-side routing)
curl -s http://localhost:8081/rag-stats | head -1
# Expected: <!doctype html>

# Root (should serve index.html)
curl -s http://localhost:8081/ | head -1
# Expected: <!doctype html>
```

### Test 404 for Missing Assets

```bash
# Missing .js asset (should return 404, not SPA fallback)
curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8081/assets/nonexistent.js
# Expected: HTTP 404

# Missing .css asset (should return 404)
curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8081/assets/nonexistent.css
# Expected: HTTP 404

# Unknown extension (SPA fallback, since it's not a known asset type)
curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8081/assets/unknown.xyz
# Expected: HTTP 200 (serves index.html, since .xyz is not a recognized asset extension)
```

### Verify Embedding

```bash
# Confirm asset is embedded in the binary
strings build/memos | grep "RagStats.XaXVsIZb"
# Expected: dist/assets/RagStats.XaXVsIZb.js

# Compare with dist directory
ls server/router/frontend/dist/assets/RagStats*
# Should match the embedded filename
```

### Test Taskfile Ordering

```bash
# Clean build to force rebuild
rm -rf server/router/frontend/dist/assets/ build/memos

# Build with RAG (frontend deps now block backend)
task build:rag:all

# Verify binary has latest assets
strings build/memos | grep "RagStats" | head -1
# Should match the current dist filename
```

---

## Lessons for Coding Agents

### Gotchas Table

| Gotcha | Details |
|--------|---------|
| **`c.Path()` is NOT the URL** | In Echo v4, `c.Path()` returns the registered route pattern. Use `c.Request().URL.Path` for the actual URL in middleware. |
| **`//go:embed` is compile-time** | `embed.FS` captures files when the binary is built. Rebuilding frontend without rebuilding Go binary = stale embed. |
| **Taskfile `deps` run in parallel** | Use `deps` ordering to enforce build sequence. Adding a task as a dep of another ensures it runs first. |
| **SPA fallback is a double-edged sword** | Serving `index.html` for missing assets gives a 200 OK with wrong content. Always 404 for known asset extensions (.js, .css, etc.). |
| **Hash differences = stale binary** | If browser requests `SignIn.ABC123.js` but dist has `SignIn.XYZ789.js`, the binary was built from an older dist. |
| **`strings` grep confirms embed** | `strings build/memos | grep "FileName"` is the fastest way to verify a file is embedded in the binary. |

### Debugging "Failed to fetch dynamically imported module"

1. **Check the URL in the error** — Extract the asset filename (e.g., `SignIn.CN5M3ne1.js`)
2. **Verify the file exists in dist** — `ls server/router/frontend/dist/assets/SignIn*`
3. **Check if it's embedded** — `strings build/memos | grep "SignIn.CN5M3ne1"`
4. **Test serving** — `curl -s -o /dev/null -w "%{http_code}" http://localhost:PORT/assets/SignIn.CN5M3ne1.js`
5. **Check content type** — Should be `text/javascript`, not `text/html`
6. **If content is HTML** — The middleware is serving `index.html` instead of the asset. Check `c.Path()` vs `c.Request().URL.Path`.
7. **If file not found** — The binary was built before the frontend dist. Rebuild with `task build:rag:all`.

### Echo Middleware Best Practices

```go
// CORRECT: Use c.Request().URL.Path for actual URL
reqPath := c.Request().URL.Path

// WRONG: c.Path() returns route pattern, not URL
reqPath := c.Path()
```

### Build Order Best Practices

```yaml
# CORRECT: Backend depends on frontend
build:backend:rag:
    deps: [build:frontend, setup:lancedb, validate:migrations]

# WRONG: Frontend and backend run in parallel (race condition)
build:rag:
    deps: [build:frontend, build:backend:rag]
```

---

## Related Files

| File | Change | Purpose |
|------|--------|---------|
| `server/router/frontend/frontend.go:42` | `c.Path()` → `c.Request().URL.Path` | Fix static asset serving |
| `Taskfile.yml:55` | Added `build:frontend` dep to `build:backend:rag` | Fix build race condition |

---

## Architecture Reference

### Frontend Serving Flow

```
Request → Echo Router → Global Middleware Chain → [Frontend Middleware] → ...
                            ↓
                    Router.Find() sets c.path = ""
                            ↓
                    Frontend Middleware runs
                            ↓
                    c.Request().URL.Path = "/assets/X.js" (FIXED)
                            ↓
                    distFS.Open("assets/X.js") → found → serve JS
                            ↓
                    or not found → isAssetRequest() → 404 (for .js/.css)
                                                 → SPA fallback (for navigation)
```

### Build Pipeline

```
task build:rag:all
    ├── build:frontend          (vite build → server/router/frontend/dist/)
    ├── build:backend:rag       (go build -tags rag → embeds dist/*)
    │   └── deps: build:frontend  (ensures frontend is built FIRST)
    └── build:widget            (embeddable chat widget)
```

---

## See Also

- `docs/DOCS_FIXES.MD` — General fixes documentation
- `docs/DOCS_FLY.MD` — fly.io deployment notes
- `docs/DOCS_TASKFILE.MD` — Build commands reference
- `docs/DOCS_DEPLOY_FLY.MD` — Deployment guide
