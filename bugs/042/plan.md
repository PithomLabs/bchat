# Bug 042: Chat Widget Embed Loading Failure

## Symptom

`http://localhost:8081/widget/evpn/embed.js` returns 404. The chat widget never loads on tenant pages.

Console error:
```
Loading failed for the <script> with source "http://localhost:8081/widget/evpn/embed.js".
```

## Root Cause

The frontend SPA middleware in `server/router/frontend/frontend.go:45` intercepts all non-API requests. It tries to serve `/widget/evpn/embed.js` from the embedded frontend `dist/` filesystem. The file doesn't exist there, so the middleware returns HTTP 404 before the Echo router ever dispatches to the actual `HandleWidgetEmbed` handler.

The skip list only includes `/api` and `/memos.api.v1`, not `/widget`.

## Secondary Bug

`layouts/partials/custom/head-end.html:1` has a leading space in the script path: `" js/disable-right-click.js"` renders as `/%20js/disable-right-click.js` (404).

## Fixes

### Fix 1: bchat — Add `/widget` to middleware skip list

**File:** `server/router/frontend/frontend.go:45`

```go
// Before
if util.HasPrefixes(reqPath, "/api", "/memos.api.v1") {

// After
if util.HasPrefixes(reqPath, "/api", "/memos.api.v1", "/widget") {
```

### Fix 2: Hugo — Fix space in disable-right-click path

**File:** `izaakmaine.github.io-main/layouts/partials/custom/head-end.html:1`

```html
<!-- Before -->
<script src="{{ " js/disable-right-click.js" | relURL }}"></script>

<!-- After -->
<script src="{{ "js/disable-right-click.js" | relURL }}"></script>
```

## Verification

1. Restart bchat server after Fix 1
2. `curl -I http://localhost:8081/widget/evpn/embed.js` -> should return 200
3. Visit `http://localhost:1313/evpn/` -> chat widget should appear
4. Visit `http://localhost:1313/` -> disable-right-click should work, no console error

## Adversarial Review

Before implementing, challenge this plan:

1. **Is the root cause correct?** Could the 404 be caused by something else (CORS, port mismatch, middleware ordering)?
2. **Is the skip list the right fix?** Does adding `/widget` to `HasPrefixes` introduce any security or routing conflicts?
3. **Are there other routes affected?** Check if `/api/v1/agent/:slug/widget.js` (line 267 in v1.go) also needs the skip.
4. **Is Fix 2 safe?** Does removing the leading space in `head-end.html` break any existing behavior?
5. **Edge cases:** What happens if the bchat server is down? Does the widget script fail silently or show an error to users?
6. **Alternative approaches:** Should the widget bundle be embedded in the Go binary via `//go:embed` instead of using `os.ReadFile` with a relative path?
