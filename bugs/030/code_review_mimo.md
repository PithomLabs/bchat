# Adversarial Code Review — Security Remediation (bugs/030)

**Reviewer:** Adversarial Security Auditor  
**Date:** 2026-07-09  
**Target:** `code.md` (security remediation implementation, Phases 0–2)

---

## Phase 0 — Emergency (H7, C2, C3)

### [HIGH] H4 REGRESSION — HandleSelectTenant Sets SameSite=None Cookie

**File:** `server/router/api/v1/auth_service.go:536-548`  
**Claimed Fix:** "Cookie changed to `SameSite=Lax`" (H4)  
**Actual:** `HandleSelectTenant` constructs its own cookie and sets `SameSite=None` for HTTPS origins, directly contradicting the H4 remediation.  
**Verdict:** ❌ NOT FIXED  

The `buildAccessTokenCookie` method (line 307) correctly uses `SameSite=Lax`, but `HandleSelectTenant` (lines 534-549) bypasses it entirely:

```go
if isHTTPS {
    cookie.SameSite = http.SameSiteNoneMode  // BUG: should be Lax
    cookie.Secure = true
} else {
    cookie.SameSite = http.SameSiteStrictMode  // Inconsistent with buildAccessTokenCookie (Lax)
}
```

The multi-tenant login flow sets a cookie that permits cross-site sends, defeating CSRF protection.

---

### [INFO] H7 — Credential Rotation ✅ VERIFIED

`.env` contains placeholders. Live key is gone. `bugs/026/s3_probe/.env` does not exist on disk (claimed as "replaced with placeholders" but actually absent — misleading documentation).

---

### [INFO] C2 — pprof Gating ✅ VERIFIED

`server/server.go:78-82` correctly checks `profile.IsDev() || os.Getenv("MEMOS_ENABLE_PPROF") == "true"`.

---

### [INFO] C3 — Debug Mode + Error Handler ✅ VERIFIED

`echoServer.Debug = profile.IsDev()`. Production handler returns `{"error":"Internal server error"}` only.

---

## Phase 1 — Tenant Isolation (H2, H1, C1)

### [MEDIUM] H2 Part B — Empty TenantIDs Produces SQL Error

**File:** `server/router/api/v1/tenant_context.go:60-63`  
**Verdict:** ⚠️ PARTIAL  

When all GUID lookups fail, returns `[]int32{}` (empty slice). The filter functions check `if tenantIDs != nil`, so the empty slice IS applied, producing `tenant_id IN ()` — a syntax error in both SQLite and Postgres. Correct security posture (deny all), but produces 500 errors instead of 403.

---

### [INFO] H2 Part A — isSuperUser ✅ VERIFIED

`common.go:70-71` correctly returns `true` only for `RoleHost` or unscoped `RoleAdmin`.

---

### [INFO] H2 Part B — TenantIDs SQL Filter ✅ VERIFIED

Both SQLite (`memo.go:88-96`, `ticket.go:77-85`) and Postgres generate parameterized `IN (?)` clauses correctly.

---

### [INFO] H1 — TenantBindingMiddleware ✅ VERIFIED

RoleUser path calls `ListUserTenantPermissions` and denies with 403 if no permissions. RoleAdmin checks `AllowedTenantIDs` against GUID. No-slug routes self-check from the record.

---

### [INFO] C1 — isSuperAdmin + 9 Handlers ✅ VERIFIED

All 9 handlers use `!h.isSuperAdmin(c) && !h.hasPermission(c, tenant.ID, <perm>)`. `isSuperAdmin` mirrors `isSuperUser` exactly.

---

## Phase 2 — Hardening (H3, H4, H5, H6)

### [INFO] H3 — Filename Sanitization ✅ VERIFIED

`sanitizeFilename` uses `filepath.Base()`, strips null bytes, rejects `.`, `..`, empty string. Containment assertion applied on both save and read paths.

---

### [HIGH] H4 — CSRF Middleware Skips Missing Origin

**File:** `server/router/api/v1/csrf.go:41-46`  
**Verdict:** ⚠️ PARTIAL  

When both `Sec-Fetch-Site` and `Origin` are absent, the middleware allows the request through (line 45-46). Comment says "SameSite=Lax already blocks cross-site POST, so this is safe." This is correct for browsers, but non-browser clients (curl, scripts, mobile apps) that use cookie auth without a Bearer header bypass CSRF entirely. If `CSRF_ALLOWED_ORIGINS` is empty (default), requests WITH an Origin header are denied, but requests WITHOUT are allowed — an inconsistent security posture.

**Recommended fix:** Fail-closed: deny state-changing requests when both headers are absent.

---

### [INFO] H4 — Cookie SameSite=Lax ✅ VERIFIED

`buildAccessTokenCookie` unconditionally appends `SameSite=Lax` (line 307). Correct for gRPC-gateway flow. (See H4 REGRESSION above for `HandleSelectTenant`.)

---

### [INFO] H5 — Non-Root Container ✅ VERIFIED

Dockerfile installs `gosu`, creates `memos` user, no `USER` directive. Entrypoint drops privileges via `exec gosu memos "$@"`. Falls through to `exec "$@"` in non-root mode.

---

### [MEDIUM] H6 — Redirect Re-validation Without IP Re-pinning

**File:** `plugin/webhook/webhook.go:124-136`  
**Verdict:** ⚠️ PARTIAL (by design)  

`CheckRedirect` validates each redirect target via `validateAndResolveWebhookURL`, blocking internal IPs. However, the transport's `DialContext` is still pinned to the ORIGINAL IP. Redirects to different external IPs will go to the original IP with the new `Host` header. This is actually MORE secure (prevents DNS rebinding), but the behavior is undocumented and could cause confusion.

---

### [INFO] H6 — URL Validation + isInternalIP ✅ VERIFIED

`validateWebhookURL` checks scheme and hostname. `isInternalIP` covers loopback, private, link-local, metadata IPs.

---

## New Findings

### [MEDIUM] Information Leakage in Reindex Error Messages

**File:** `server/router/api/v1/agent/handlers.go:1208-1220, 1249`  
**Verdict:** 🐛 REGRESSION (minor)  

`reindexHTTPError` and `HandleReindexStatus` include `err.Error()` in HTTP responses. These are admin-only endpoints, but the C3 fix was supposed to return generic messages in production.

---

### [LOW] HandleSelectTenant O(users × tokens) Scan

**File:** `server/router/api/v1/auth_service.go:471-497`  
**Verdict:** 🐛 REGRESSION (performance)  

Lists ALL users and iterates their access tokens to find a selection token. Pre-existing issue, not introduced by this change, but worth noting.

---

## Summary

| Finding | Severity | Verdict |
|---------|----------|---------|
| HandleSelectTenant SameSite=None | HIGH | ❌ NOT FIXED |
| CSRF missing-Origin pass-through | HIGH | ⚠️ PARTIAL |
| deriveTenantIDsForScopedAdmin empty IN | MEDIUM | ⚠️ PARTIAL |
| Redirect re-validation (no re-pin) | MEDIUM | ⚠️ PARTIAL (by design) |
| Reindex error message leakage | MEDIUM | 🐛 REGRESSION |
| HandleSelectTenant O(n) scan | LOW | 🐛 REGRESSION |
| All other findings | INFO | ✅ VERIFIED |

**Critical items requiring immediate attention:**
1. **HandleSelectTenant cookie** — Set `SameSite=Lax` unconditionally (HIGH)
2. **CSRF missing-Origin** — Fail-closed for non-browser clients (HIGH)
3. **Empty TenantIDs IN clause** — Handle gracefully to avoid 500 errors (MEDIUM)
