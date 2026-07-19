# Plan Review: Bug 042 — Chat Widget Embed Loading Failure

## Fix 1: Add `/widget` to middleware skip list — APPROVED (with nits)

**Root cause confirmed.** The frontend middleware at `server/router/frontend/frontend.go:40` is registered as a global `e.Use()` in `server/server.go:110`. It runs **before** the Echo router on every request. Since `/widget/evpn/embed.js` is not in the skip list (`/api`, `/memos.api.v1`), the middleware tries to open it from the embedded `dist/` filesystem. The file doesn't exist there, and because `isAssetRequest()` returns `true` (the path ends in `.js`), the middleware returns HTTP 404 — the router never gets a chance to dispatch to `HandleWidgetEmbed`.

The fix on line 45 (`util.HasPrefixes(reqPath, "/api", "/memos.api.v1", "/widget")`) is minimal and correct. The legacy route `/:slug/widget.js` at `v1.go:267` is under `/api/v1/agent` and is already covered by the existing skip list.

### Nits

1. **Prefer `"/widget/"` over `"/widget"`** — The bare prefix `"/widget"` also matches hypothetical future paths like `/widget-admin`, `/widget-config`, `/widget-something-else`. Using `"/widget/"` is more precise and avoids accidental bypasses. (A bare `GET /widget` with no trailing path has no registered route anyway, so it would be a router 404 — harmless.)

2. **Document middleware ordering** — The plan should note that `FrontendService.Serve` is called via `e.Use()` and runs *before* route-group middleware. Adding to the skip list is the correct approach; no reordering is needed, but the implicit dependency is worth flagging for future maintainers.

3. **Check for other global middleware** — Verify no other `e.Use()` or `e.Pre()` middleware intercepts `/widget` paths (e.g., auth, rate-limiting). None were found in this review, but it's worth a grep if routes are ever added.

---

## Fix 2: Space in `head-end.html` — REWORK NEEDED

**The file does not exist in this repository.** No `head-end.html` was found anywhere under `/home/chaschel/Documents/go/bchat`. The path `izaakmaine.github.io-main/layouts/partials/custom/head-end.html` references an **external** Hugo GitHub Pages site, not a file in the bchat codebase.

The fix itself is correct — a leading space in `{{ " js/disable-right-click.js" | relURL }}` renders as `/%20js/disable-right-click.js` — but the plan must:

- Clarify where this file lives (external repo? submodule? needs to be created?)
- If external, note that the fix must be applied in that repository, not in bchat
- Update the "Verification" section to reflect the actual deployment context

---

## Additional Observations Not in Plan

### `HandleWidgetEmbed` uses a relative `os.ReadFile` path

At `server/router/api/v1/agent/handlers.go:2087`:
```go
widgetPath := filepath.Join("widget", "dist", "embed.min.js")
content, err := os.ReadFile(widgetPath)
```
This is relative to the process working directory. If the server is started from a different directory (e.g., via systemd, `task run:docker`, etc.), the file won't be found and the handler silently falls back to inline-generated JS (lines 2091-2095). This is a **separate reliability concern** from the 404 bug, not a blocker. The adversarial review already raises `//go:embed` as an alternative — this can be deferred to a follow-up issue.

---

## Verification Plan Review

| Step | Assessment |
|------|------------|
| 1. Restart server | ✅ Required after Fix 1 |
| 2. `curl -I http://localhost:8081/widget/evpn/embed.js` → 200 | ✅ Directly validates Fix 1 |
| 3. Visit Hugo site → widget appears | ⚠️ Requires external Hugo server (`:1313`) — note dependency |
| 4. Visit Hugo site → no console error | ⚠️ Same external dependency; Fix 2 may not be in this repo |

---

## Adversarial Questions Answered

| # | Question | Answer |
|---|----------|--------|
| 1 | Could the 404 be from CORS, port mismatch, or middleware ordering? | **No.** The frontend middleware intercepts before the router; file not in `dist/` + `isAssetRequest` = hard 404. |
| 2 | Does the skip-list fix introduce security/routing conflicts? | **Minimal risk.** `/widget/` prefix is narrow and only two routes exist under it. The trailing-slash nit above reduces risk further. |
| 3 | Does the legacy `/:slug/widget.js` route need the skip? | **No.** It's under `/api/v1/agent` which is already in the skip list. |
| 4 | Does removing the space in `head-end.html` break anything? | **No** — but the file isn't in this repo, so moot. |
| 5 | What if the bchat server is down? | The widget script loaded from `embed.js` would fail to fetch. This is a runtime concern outside scope of this bug. |
| 6 | Should widget bundle be embedded via `//go:embed`? | **Worthy follow-up.** Currently uses `os.ReadFile` with a relative path — fragile. Not a blocker for the 404 fix. |

---

## Decision

| Fix | Verdict |
|-----|---------|
| Fix 1: Skip list | **APPROVED** — apply with `"/widget/"` (trailing slash) |
| Fix 2: Space in template | **REWORK** — clarify file location; may belong in a different repo |
