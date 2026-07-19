# Implementation Plan: Bug 042 — Chat Widget Embed Loading Failure

Based on adversarial review in `plan_review.md`.

## Fix 1: bchat middleware skip list (APPROVED)

**File:** `server/router/frontend/frontend.go:45`

```go
// Before
if util.HasPrefixes(reqPath, "/api", "/memos.api.v1") {

// After
if util.HasPrefixes(reqPath, "/api", "/memos.api.v1", "/widget/") {
```

Using `"/widget/"` with trailing slash per review nit — avoids matching hypothetical paths like `/widget-admin`.

## Fix 2: Hugo space in head-end.html (different repo)

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
4. Visit `http://localhost:1313/` -> no `/%20js/` console error
