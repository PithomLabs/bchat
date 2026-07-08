# Adversarial Code Review — bugs/030 Security Remediation

**Reviewer:** Stepfun Adversarial Security Auditor
**Target:** `bugs/030/code.md` (Phase 0–2 security remediation)
**Date:** 2026-07-09
**Stance:** Skeptical, adversarial — every claim verified against actual source.

---

## Executive Summary

The remediation introduces meaningful improvements but contains **one critical regression**, **two high-severity gaps**, and **several medium/low issues** that undermine the claimed fix completeness. The most dangerous finding is a **SameSite=None cookie regression in the tenant-selection flow** that re-introduces CSRF exposure. Additionally, **10+ handlers on `authGroup` lack tenant binding**, allowing scoped admins to access any tenant.

---

## Phase 0 — Emergency (H7, C2, C3)

### [CRITICAL] H4 Regression: `HandleSelectTenant` Uses `SameSite=None` for HTTPS

**File:** `server/router/api/v1/auth_service.go:535-549`
**Claimed Fix:** "Always SameSite=Lax" (code.md §H4, line 286)
**Actual:**
```go
if isHTTPS {
    cookie.SameSite = http.SameSiteNoneMode   // line 544
    cookie.Secure = true
} else {
    cookie.SameSite = http.SameSiteStrictMode  // line 547
}
```
**Verdict:** ❌ NOT FIXED — REGRESSION
**Details:** The `buildAccessTokenCookie` helper correctly sets `SameSite=Lax` (line 307), but `HandleSelectTenant` constructs a **separate** cookie with the OLD conditional logic (`None` for HTTPS, `Strict` for HTTP). This is the exact pre-remediation behavior. An attacker on a HTTPS site can forge cross-site requests to `/api/v1/auth/select-tenant` because `SameSite=None` disables the CSRF defense. The claimed "Always SameSite=Lax" fix was only applied to the main login cookie, not the tenant-selection cookie.

---

### [HIGH] H7: Missing `bugs/026/s3_probe/.env` File

**File:** `bugs/026/s3_probe/.env`
**Claimed Fix:** "Replaced live S3 credentials with placeholders" (code.md §H7, line 14)
**Actual:** File does not exist. Directory contains only `go.mod`, `go.sum`, `main.go`, `README.md`.
**Verdict:** ❌ NOT FIXED
**Details:** Either the file was never committed or it was removed after rotation. The claim in `code.md` is false. If the `.env` was git-tracked and contained live credentials, those credentials remain in git history. The remediation should either create the placeholder file or verify the directory is gitignored.

---

### [MEDIUM] C2: pprof Exposed on Public Interface When Env Flag Set

**File:** `server/server.go:80-82`
**Claimed Fix:** "pprof gated behind `profile.IsDev()` or env flag" (code.md §C2)
**Actual:**
```go
if profile.IsDev() || os.Getenv("MEMOS_ENABLE_PPROF") == "true" {
    s.profiler.RegisterRoutes(echoServer)
}
```
**Verdict:** ⚠️ PARTIAL
**Details:** The code is correct — pprof is disabled by default. However, the comment claims "Loopback binding dropped" and "env flag is the correct and sufficient control." This is **only true if the flag is never accidentally set in production**. There is no startup validation warning when `MEMOS_ENABLE_PPROF=true` in prod mode. A single environment misconfiguration exposes `/debug/pprof/*` on the public listener with no additional network-layer protection.

---

### [LOW] C3: Prod Error Handler Leaks HTTP Status Code Semantics

**File:** `server/server.go:57-71`
**Claimed Fix:** "Production gets generic error messages" (code.md §C3)
**Actual:**
```go
if he, ok := err.(*echo.HTTPError); ok {
    _ = c.JSON(he.Code, map[string]string{"error": "Internal server error"})
} else {
    _ = c.JSON(http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
}
```
**Verdict:** ✅ VERIFIED (with note)
**Details:** No stack traces or file paths are returned to the client. However, the error response preserves the original HTTP status code (e.g., 404, 403) while the body always says "Internal server error." This is a minor information leak — an attacker can distinguish "not found" from "forbidden" from "server error" by the status code alone. This is acceptable for most threat models but worth noting.

---

## Phase 1 — Tenant Isolation (H2, H1, C1)

### [HIGH] H1/H2/C1: `authGroup` Handlers Lack `TenantBindingMiddleware`

**File:** `server/router/api/v1/v1.go:278-332` (route registration), `server/router/api/v1/tenant_binding.go` (middleware)
**Claimed Fix:** "TenantBindingMiddleware enforces for all roles" (code.md §H1)
**Actual:**
```go
authGroup := echoServer.Group("/api/v1/agent")
authGroup.Use(s.AuthMiddleware)
authGroup.Use(adminCORS)
// NO TenantBindingMiddleware here
authGroup.GET("/:slug/validate", s.agentHandler.HandleValidateTenant)
authGroup.POST("/:slug/chat/int", s.agentHandler.HandleChatInternal)
authGroup.GET("/:slug/llm-config", s.agentHandler.HandleGetLLMConfig)
authGroup.PUT("/:slug/llm-config", s.agentHandler.HandleSetLLMConfig)
authGroup.GET("/:slug/permissions", s.agentHandler.HandleListPermissions)
authGroup.POST("/:slug/permissions", s.agentHandler.HandleGrantPermission)
authGroup.DELETE("/:slug/permissions/:userId", s.agentHandler.HandleRevokePermission)
authGroup.GET("/:slug/sessions", s.agentHandler.HandleListSessions)
authGroup.GET("/:slug/sessions/:sessionId", s.agentHandler.HandleGetSession)
authGroup.POST("/:slug/simulate", s.agentHandler.HandleStartSimulation)
// ... and more
```
**Verdict:** ❌ NOT FIXED
**Details:** The `TenantBindingMiddleware` is only applied to `adminGroup` (line 338). The `authGroup` has **no tenant binding at all**. These handlers use `h.isAdmin(c)` for authorization, and `isAdmin` returns `true` for scoped admins. Therefore, a scoped admin can:
- Probe any tenant via `HandleValidateTenant`
- Read/write LLM config for any tenant via `HandleSetLLMConfig`
- List/revoke permissions for any tenant via `HandleListPermissions` / `HandleRevokePermission`
- Read chat sessions for any tenant via `HandleListSessions` / `HandleGetSession`
- Import scripts for any tenant via `HandleImportScript`

This is a **cross-tenant information disclosure and privilege escalation** vulnerability. The C1 claim that "9 handlers guarded" is misleading — those 9 handlers are on `adminGroup` where tenant binding already existed. The real gap is the `authGroup` handlers.

---

### [HIGH] H2 Part A: `isAdmin` Still Grants Super Access to Scoped Admins

**File:** `server/router/api/v1/agent/handlers.go:2222`
**Claimed Fix:** "isSuperUser fixed — only RoleHost or unscoped RoleAdmin" (code.md §H2 Part A)
**Actual:**
```go
func (h *Handler) isAdmin(c echo.Context) bool {
    // ...
    isAdmin := user.Role == store.RoleHost || user.Role == store.RoleAdmin
    return isAdmin
}
```
**Verdict:** ⚠️ PARTIAL — `isSuperUser` fixed, but `isAdmin` unchanged
**Details:** The `isSuperUser` fix in `common.go:70-71` is correct. However, `isAdmin` in `handlers.go:2222` still returns `true` for ALL `RoleAdmin` users, including scoped admins. This function is used in **66+ call sites** across the agent handler. While `adminGroup` handlers are protected by `TenantBindingMiddleware`, `authGroup` handlers are NOT. This means scoped admins pass `isAdmin` checks on unbound routes and gain cross-tenant access. The fix should either:
1. Add `TenantBindingMiddleware` to `authGroup`, or
2. Replace `isAdmin` with `isSuperAdmin` on authGroup routes, or
3. Add per-route tenant ownership checks.

---

### [MEDIUM] H2 Part B: Empty `TenantIDs` Slice Bypasses `nil` Check

**File:** `server/router/api/v1/tenant_context.go:60-63`
**Claimed Fix:** "Tenant filter derived from AllowedTenantIDs" (code.md §H2 Part B)
**Actual:**
```go
if len(tenantIDs) == 0 {
    // No valid tenants found — deny all (return empty slice, not nil)
    return []int32{}
}
```
**Verdict:** ⚠️ PARTIAL
**Details:** The comment says "return empty slice, not nil" to trigger the `IN ()` clause. But the caller `ApplyTenantFilter` checks `if tenantIDs != nil` (line 83). An empty slice `[]int32{}` is **not nil**, so the filter is applied. However, the SQLite and Postgres drivers check `if len(find.TenantIDs) > 0` before generating the `IN` clause. This means the empty slice is harmless in practice, but the logic is fragile:
- If a future developer adds a `WHERE tenant_id = ANY(?)` clause without length checking, it would generate invalid SQL.
- The function should return `nil` for consistency with the "super users see all" path (line 43).

---

### [LOW] C1: `isSuperAdmin` Duplicates `isSuperUser` Logic

**File:** `server/router/api/v1/agent/handlers.go:2246-2268` vs `server/router/api/v1/common.go:70-71`
**Claimed Fix:** "isSuperAdmin + 9 handlers guarded" (code.md §C1)
**Actual:** Two separate functions with identical logic:
```go
// common.go
func isSuperUser(user *store.User) bool {
    return user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0)
}

// handlers.go
func (h *Handler) isSuperAdmin(c echo.Context) bool {
    // ... fetches user from DB, then:
    return user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0)
}
```
**Verdict:** ⚠️ PARTIAL
**Details:** Functionality is correct, but maintenance risk: if the super-user definition changes, both functions must be updated. `isSuperAdmin` should delegate to `isSuperUser` to avoid divergence.

---

## Phase 2 — Hardening (H3, H4, H5, H6)

### [MEDIUM] H3: Thumbnail Cache Path Not Explicitly Contained

**File:** `server/router/api/v1/resource_service.go:448-452`
**Claimed Fix:** "Containment assertion on both save and read paths" (code.md §H3)
**Actual:**
```go
thumbnailCacheFolder := filepath.Join(s.Profile.Data, ThumbnailCacheFolder)
filePath := filepath.Join(thumbnailCacheFolder, fmt.Sprintf("%d%s", resource.ID, filepath.Ext(resource.Filename)))
```
**Verdict:** ⚠️ PARTIAL
**Details:** The save path (`SaveResourceBlob`) and read path (`GetResourceBlob`) both have containment assertions. However, `getOrGenerateThumbnail` does NOT re-sanitize `resource.Filename` before using `filepath.Ext`. If a pre-existing tainted resource has `Filename = "../../../etc/passwd"`, then `filepath.Ext(resource.Filename)` returns `".passwd"` and the thumbnail path is still safely under `ThumbnailCacheFolder` because `filepath.Join` resolves the relative components. But the filename is not passed through `sanitizeFilename` before extension extraction. This is safe in this specific case because the path is always prefixed with the cache folder, but it's inconsistent with the defense-in-depth approach.

---

### [MEDIUM] H6: Save-Time Webhook Validation Does Not Block Internal IPs

**File:** `server/router/api/v1/webhook_service.go:21-38`
**Claimed Fix:** "URL validation on create/update" (code.md §H6)
**Actual:**
```go
func validateWebhookURL(rawURL string) error {
    parsed, err := url.Parse(rawURL)
    // ...
    if parsed.Scheme != "http" && parsed.Scheme != "https" { return error }
    if parsed.Hostname() == "" { return error }
    return nil
}
```
**Verdict:** ⚠️ PARTIAL
**Details:** The save-time check only validates URL format (scheme + hostname). It does **not** resolve DNS or check for internal IPs. The claimed fix says "Security enforcement happens at dispatch time with IP-pinned dialer." This means:
1. An attacker can save `http://169.254.169.254/latest/meta-data/` as a webhook URL without error.
2. The URL is only blocked when `Post()` is called.
3. If dispatch is delayed or the webhook is never triggered, the tainted URL persists in the database.

This is a design choice, but it violates the principle of "fail closed" — invalid URLs should be rejected at input time, not deferred to dispatch.

---

### [LOW] H5: `file_env` Silently Ignores Missing Secret Files

**File:** `scripts/entrypoint.sh:15-19`
**Claimed Fix:** "Added gosu privilege drop (preserved _FILE logic)" (code.md §H5)
**Actual:**
```sh
if [ -n "$val_fileVar" ]; then
    val="$(cat "$fileVar")"
fi
```
**Verdict:** ⚠️ PARTIAL
**Details:** If `OPENROUTER_API_KEY_FILE=/run/secrets/api_key` is set but the file does not exist (misconfigured Docker secret, K8s volume not mounted), `cat` fails silently (no `set -e`) and the variable is set to empty string. The application then starts with an empty API key and fails at runtime with a less obvious error. The function should `exit 1` if the file cannot be read.

---

### [LOW] H6: Redirect Re-Validation Does Not Update Transport

**File:** `plugin/webhook/webhook.go:125-136`
**Claimed Fix:** "Redirect policy — cap redirects + re-validate each target" (code.md §H6)
**Actual:**
```go
CheckRedirect: func(req *http.Request, via []*http.Request) error {
    if len(via) >= 3 { return errors.Errorf("too many redirects") }
    _, err := validateAndResolveWebhookURL(req.URL.String())
    return err
},
```
**Verdict:** ⚠️ PARTIAL
**Details:** The `CheckRedirect` re-validates the redirect target URL, but the `http.Transport` is still pinned to the **original** `dialIP`. This means:
1. The redirect request is sent to the original validated IP, not the redirect target.
2. The `req.URL` is updated to the redirect target, but the TCP connection remains pinned.
3. This is **safe** (we never connect to an unvalidated IP), but it creates a semantic mismatch: the URL in the request line points to the redirect target, but the TCP connection goes to the original server. Some webhook receivers may reject this as a malformed request.

---

### [INFO] H5: Container Runs as Root Until Entrypoint Executes

**File:** `Dockerfile.fly:112`, `Dockerfile.s3.fly:116`
**Claimed Fix:** "Non-root container + gosu" (code.md §H5)
**Actual:** `ENTRYPOINT ["./entrypoint.sh", "./memos"]`
**Verdict:** ✅ VERIFIED (with note)
**Details:** The container starts as root, chowns the volume, then drops privileges via `gosu`. This is the standard pattern. However, the process runs as root for a brief window during startup. If an attacker can exploit a vulnerability during this window (e.g., via a malicious volume mount), they gain root access. This is acceptable for Docker but should be noted.

---

## Specific Questions Answered

### 1. Is `isSuperAdmin` consistent with `isSuperUser`?

**Yes.** Both implement the same logic: `RoleHost` OR (`RoleAdmin` AND empty `AllowedTenantIDs`). `isSuperAdmin` fetches the user from DB; `isSuperUser` takes a user pointer. They are functionally equivalent but not code-sharing.

### 2. Does the `TenantIDs` SQL filter work for all databases?

**Partially.** SQLite and Postgres drivers implement `TenantIDs IN (...)` correctly. MySQL driver is **completely skipped** (no `tenant_id` columns). The code.md acknowledges this as "upstream issue" but calls it safe. It is **not safe** — if MySQL is ever deployed, tenant filtering silently returns all rows, bypassing all isolation.

### 3. Is there any path where a scoped admin can access a tenant they shouldn't?

**Yes — multiple paths.** The `authGroup` routes lack `TenantBindingMiddleware`. Scoped admins use `isAdmin` (which returns true for them) to bypass authorization on 10+ endpoints, including:
- `HandleValidateTenant` — tenant enumeration
- `HandleSetLLMConfig` — LLM config tampering
- `HandleListPermissions` / `HandleGrantPermission` / `HandleRevokePermission` — permission manipulation
- `HandleListSessions` / `HandleGetSession` — chat log disclosure
- `HandleImportScript` — script injection

### 4. Can an attacker bypass CSRF by using a different Content-Type?

**Partially.** The CSRF middleware skips `GET`, `HEAD`, `OPTIONS` and Bearer auth. For state-changing methods with cookies, it checks `Sec-Fetch-Site` or `Origin`. **Multipart form data** is not explicitly handled. If a browser submits a multipart POST without `Sec-Fetch-Site` and without `Origin`, the middleware falls through to `next(c)` (line 45: "Treat missing Origin conservatively"). This is safe because `SameSite=Lax` blocks cross-site multipart POSTs, but the interaction between `SameSite=None` on the select-tenant cookie and missing Origin headers is risky.

### 5. Does the IP-pinned dialer follow redirects?

**Yes, but with a quirk.** The `CheckRedirect` callback re-validates the redirect target URL, but the `http.Transport`'s `DialContext` is still pinned to the original validated IP. The redirect request goes to the original server, not the redirect target. This is safe but semantically odd.

### 6. Is there any information leakage in error messages?

**Minor.** The custom prod error handler returns generic "Internal server error" bodies. However, the HTTP status code is preserved (e.g., 404, 403), which leaks some information. The `HandleSelectTenant` cookie inconsistency (SameSite=None) is a more serious issue.

### 7. Are there any new race conditions?

**No new race conditions introduced.** The `deriveTenantIDsForScopedAdmin` does sequential DB lookups per GUID, but these are read-only and stateless. No new locks or shared mutable state.

### 8. Does the entrypoint handle the case where `gosu` is not installed?

**No.** `scripts/entrypoint.sh:51` calls `exec gosu memos "$@"` without checking if `gosu` exists. If the Docker image is built without gosu (e.g., a custom base image), the script fails with `exec: "gosu": executable file not found in $PATH`. In dev mode where the container runs as non-root, this is bypassed (line 55: `exec "$@"`), but in prod it is fatal.

### 9. Are there any existing tests that now fail?

**Cannot verify from static review.** The code.md claims "tests pass" but no test output was provided. Given the changes to `isAdmin` semantics and the addition of `TenantBindingMiddleware`, integration tests for scoped-admin flows are likely to fail or reveal gaps.

### 10. Is there any code path that bypasses the new guards?

**Yes — `authGroup` routes.** The `TenantBindingMiddleware` is only on `adminGroup`. The `authGroup` has no tenant binding, and its handlers use `isAdmin` (not `isSuperAdmin`). This is the most significant bypass path.

---

## Final Checklist

| Item | Status |
|------|--------|
| All 10 questions answered | ✅ |
| Every file in `code.md` reviewed | ✅ |
| No regressions introduced | ❌ — SameSite=None regression in `HandleSelectTenant` |
| No new vulnerabilities found | ❌ — Cross-tenant access via `authGroup` handlers |
| Build still passes | ⚠️ — Cannot verify; `go build ./...` not executed |
| Tests still pass | ⚠️ — Cannot verify; `go test ./...` not executed |

---

## Recommendations

1. **CRITICAL:** Fix `HandleSelectTenant` cookie to use `SameSite=Lax` unconditionally.
2. **HIGH:** Add `TenantBindingMiddleware` to `authGroup`, OR replace `isAdmin` with `isSuperAdmin` + explicit `hasPermission` on all `authGroup` routes.
3. **HIGH:** Create or verify existence of `bugs/026/s3_probe/.env` with placeholder values.
4. **MEDIUM:** Refactor `deriveTenantIDsForScopedAdmin` to return `nil` instead of `[]int32{}` for consistency.
5. **MEDIUM:** Add internal-IP check to `validateWebhookURL` (save-time) or document why it's deferred to dispatch.
6. **LOW:** Add startup warning if `MEMOS_ENABLE_PPROF=true` in prod mode.
7. **LOW:** Add `set -e` and explicit file-existence check in `entrypoint.sh` `file_env`.
8. **LOW:** Consolidate `isSuperAdmin` and `isSuperUser` into a single exported helper.
