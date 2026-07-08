# Adversarial Code Review — Security Remediation Implementation

**Reviewer:** OpenCode CLI (DeepSeek V4 Pro)
**Date:** 2026-07-09
**Reviewed Against:** `code.md` claims + `prompt_code_review.md` instructions
**Files Reviewed:** 22/22 (all files in `code.md`)

---

## Overall Assessment

The remediation is **directionally correct** and the most critical findings from the original audit are substantially addressed. However, there are **2 dead-code defense-in-depth gaps**, **4 partial-implementation issues**, and **2 regressions**. The core load-bearing controls (H1 middleware, H2 isSuperUser, C2 pprof gate) are correctly implemented. The primary concern is **inconsistent application of secondary defense layers** across the codebase.

---

## Findings by Severity

---

### 🔴 HIGH — H-001: `ApplyTenantFilter` is dead code for Memos

**File:** `server/router/api/v1/tenant_context.go:73-87`
**Claimed Fix:** "Updated filter functions" with `deriveTenantIDsForScopedAdmin` apply to both Memos and Tickets.
**Actual:** `ApplyTenantFilter` (for Memos) is **never called in production code**. Grep confirms zero callsites outside of `tenant_context_test.go`. The `ApplyTicketTenantFilter` is correctly called at `ticket_service.go:198`. For Memos, the service layer only uses `GetTenantIDFromContext(ctx)` (gRPC context), which returns `nil` for admin users (admins have no `tenant_id` in their JWT claims per `doSignIn` at `auth_service.go:172-188`). This means:

- Scoped admins listing/reading memos get **no SQL-level tenant filter** — `FindMemo.TenantIDs` is never populated.
- The `isSuperUser` authorization check is the **sole** load-bearing control for cross-tenant memo access.
- If any `isSuperUser` check in `memo_service.go` is missed or buggy, scoped admins can see all memos.

**Verdict:** ❌ NOT FIXED — `TenantIDs` plumbing exists in store/driver layer but the wiring from the service layer is missing. The `isSuperUser` fix mitigates exploitation, but the defense-in-depth SQL safety net that `code.md` claims exists for Memos does not.

**Impact:** `memo_service.go`

**Fix:** Call `ApplyTenantFilter` from Echo-based memo handlers, or add equivalent `TenantIDs` derivation to every `ListMemos`/`GetMemo` call site in `memo_service.go` that uses `GetTenantIDFromContext`.

---

### 🔴 HIGH — H-002: `deleteBackingResourceFile` has no containment assertion

**File:** `server/router/api/v1/memo_resource_service.go:252-283`
**Claimed Fix:** "Containment assertion (applied to save + read paths)"
**Actual:** The `deleteBackingResourceFile` function reconstructs a path from `resource.Reference` at line 254, joins it with `profile.Data` at line 257, and calls `os.Remove(p)` at line 260 — with **no `sanitizeFilename` call and no `strings.HasPrefix(filepath.Clean(p), cleanDataDir)` containment assertion**. This is inconsistent with `SaveResourceBlob` (lines 325, 347) and `GetResourceBlob` (line 419), which both have the assertion.

**Verdict:** ❌ NOT FIXED — The H3 path traversal fix claims to cover "read/delete paths" but the delete path in `memo_resource_service.go` was missed entirely.

**Mitigation:** An attacker would need to corrupt the stored `resource.Reference` in the database (save path is guarded) to exploit this. Defense-in-depth should still be applied.

---

### 🟠 MEDIUM — M-001: `isSuperAdmin` has no audit logging vs `isAdmin`

**File:** `server/router/api/v1/agent/handlers.go:2246-2268`
**Claimed Fix:** "Added `isSuperAdmin()` helper and permission checks to 9 previously unprotected handlers"
**Actual:** `isAdmin` (line 2202) logs security events via `slog.Warn`/`slog.Debug`/`slog.Info` for every code path (user not found, missing context, permission granted, permission denied). `isSuperAdmin` (line 2246) returns silently on all failure/denial paths. No audit trail for scoped admins being denied access. This makes security incident investigation harder — you cannot distinguish "scoped admin tried to access wrong tenant" from "no user in context."

**Verdict:** ⚠️ PARTIAL — The security check itself is correct, but the audit trail is missing. In a multi-tenant SaaS, knowing *who attempted what* is critical.

**Severity:** MEDIUM (not exploitable, but operational security gap)

---

### 🟠 MEDIUM — M-002: C1 handlers rely entirely on H1 middleware for scoped admin blocking

**File:** `server/router/api/v1/agent/handlers.go` (9 handlers) + `server/router/api/v1/agent/permissions.go`
**Claimed Fix:** "Added `isSuperAdmin()`) and permission checks to 9 previously unprotected handlers"
**Actual:** The guard is `!h.isSuperAdmin(c) && !h.hasPermission(c, tenant.ID, PermX)`. For scoped admins, `isSuperAdmin` correctly returns `false`. However, `hasPermission` calls `ResolveEffectivePermissions`, which unconditionally grants `tenant:read` + `api:config` to **any** `RoleAdmin` regardless of `AllowedTenantIDs` (permissions.go). So a scoped admin on tenant A would pass `PermTenantRead` for tenant B. The **only** thing blocking them is the H1 `TenantBindingMiddleware`.

**Verdict:** ⚠️ PARTIAL — The architecture acknowledges this (`plan2.md` says "C1's per-handler guards are defense-in-depth for RoleUser only"). The C1 fix is effective for `RoleUser` but not for scoped admins — the H1 middleware is the sole load-bearing control. If middleware ordering is wrong or slug parsing fails, scoped admins bypass.

**Severity:** MEDIUM (relies on architectural assumption; no bypass found in current code but fragile)

---

### 🟠 MEDIUM — M-003: `HandleDeleteTenant` and `HandleOnboard` still use `isAdmin` not `isSuperAdmin`

**File:** `server/router/api/v1/agent/handlers.go:1659,1298`
**Claimed Fix:** "All 9 handlers return 403 for unauthorized access" (C1 list)
**Actual:** The 9 specified handlers correctly use `isSuperAdmin`. However, `HandleDeleteTenant` (line 1659), `HandleOnboard` (line 1298), `HandleListTenants` (line 660), and other global tenant management handlers still use `isAdmin` only — meaning **any scoped admin can delete/onboard/list any tenant**. This was outside the C1 scope, but it means scoped admins retain destructive cross-tenant capabilities.

**Verdict:** ⚠️ PARTIAL — The C1 scope was limited to the 9 handlers. But claiming "scoped admins cannot access tenants they shouldn't" while they can delete arbitrary tenants is inconsistent.

**Severity:** MEDIUM (deliberately scoped, but represents residual cross-tenant risk)

---

### 🟠 MEDIUM — M-004: `entrypoint.sh` crashes if `gosu` is missing and running as root

**File:** `scripts/entrypoint.sh:46-52`
**Claimed Fix:** "Container now drops privileges to non-root user via gosu" with entrypoint handling privilege drop.
**Actual:** If the process runs as root (id -u = 0) but `gosu` is not installed (common in dev environments, non-Docker setups, or minimal base images), line 51 (`exec gosu memos "$@"`) crashes with `gosu: command not found` and the script terminates without reaching line 55's fallback `exec "$@"`. There is no `command -v gosu` check.

**Verdict:** 🐛 REGRESSION — Breaking change for any dev workflow that runs `sudo` without gosu installed.

**Fix:** 
```sh
if command -v gosu >/dev/null 2>&1; then
    exec gosu memos "$@"
else
    echo "WARNING: gosu not found, running as root" >&2
    exec "$@"
fi
```

---

### 🟠 MEDIUM — M-005: `sanitizeFilename` null-byte stripping ordering

**File:** `server/router/api/v1/resource_service.go:304-314`
**Claimed Fix:** "Filenames are sanitized" with `filepath.Base` then null-byte removal.
**Actual:** Null bytes are stripped **after** `filepath.Base`:
```go
filename = filepath.Base(filename)         // line 307
filename = strings.ReplaceAll(filename, "\x00", "")  // line 309
```
On some Go versions/platforms, `filepath.Base` may truncate at null bytes before the explicit replacement runs. The correct order is: strip null bytes **first**, then `filepath.Base`. While `filepath.Base` on Linux/amd64 does not currently truncate at `\x00`, this is an implementation detail, not a documented guarantee.

**Verdict:** 🐛 REGRESSION — Ordering is wrong; null bytes should be removed before `filepath.Base`. Current Go runtime behavior masks this, but it's fragile.

---

### 🟡 LOW — L-001: Prod error handler mangles non-500 status messages

**File:** `server/server.go:57-71`
**Claimed Fix:** "Production error handler returns no internal details (stack traces, file paths, line numbers)"
**Actual:** The custom error handler always returns `{"error": "Internal server error"}` regardless of the HTTP status code. A 404 (Not Found) responds with HTTP 404 and body `{"error": "Internal server error"}`. An HTTP 400 (Bad Request) responds with HTTP 400 and body `"Internal server error"`. This is misleading to API consumers but does not leak sensitive data.

**Verdict:** ✅ VERIFIED (security goal met) / INFO (UX regression: consider returning status-code-appropriate generic messages, e.g., "Bad request" for 4xx, "Internal server error" for 5xx)

**Severity:** LOW (no security impact, but UX regression)

---

### 🟡 LOW — L-002: `sanitizeFilename` only called on save path, not read path

**File:** `server/router/api/v1/resource_service.go:325,409-439`
**Claimed Fix:** "Filenames are sanitized and resolved paths are checked"
**Actual:** `sanitizeFilename` is called at line 325 (save path) but NOT at the read path (`GetResourceBlob`, line 409). The read path relies solely on the containment assertion. If a stored `resource.Reference` were corrupted (bypassing the save path), the filename would be used unsanitized on read. The containment assertion would catch traversal, but defense-in-depth would be stronger with sanitization on both paths.

**Verdict:** ⚠️ PARTIAL — Defense-in-depth gap. The containment assertion is the actual safety boundary on read, which works, but the claim that filenames are "sanitized" is only true for writes.

**Severity:** LOW (containment assertion provides equivalent protection)

---

### 🟡 LOW — L-003: `s3_probe/.env` credentials untraceable

**File:** `bugs/026/s3_probe/.env`
**Claimed Fix:** "Replaced live S3 credentials with placeholders"
**Actual:** The file `bugs/026/s3_probe/.env` does not exist in the repository. The `s3_probe/` directory contains only `main.go`, `go.mod`, `go.sum`, and `README.md`. It is unclear whether the `.env` file was deleted, gitignored, or never existed. The H7 rotation claim cannot be verified for this file.

**Verdict:** ⚠️ PARTIAL — The `.env` at the root level is correctly placeholdered. The `s3_probe/.env` is missing from the tree, so its status is unknown. If the Tigris credentials were only in this missing file and the file has been removed, the rotation is effectively verified.

**Severity:** LOW (instructional gap — no way to verify from repo state alone)

---

### 🟡 LOW — L-004: `TenantBindingMiddleware` error leaks internal failure mode

**File:** `server/router/api/v1/tenant_binding.go:25-26`
**Actual:** If `s.GetUser` returns an unexpected error (DB failure), the middleware returns `"failed to verify tenant binding"` with HTTP 500. This leaks that a user ID exists but the verification step failed, which is an information disclosure vector distinct from "access denied." In contrast, `user == nil` returns `"access denied"` (line 29).

**Verdict:** ✅ VERIFIED (middleware works correctly) / INFO (minor info leak on DB errors — return generic "access denied" on all failure paths)

**Severity:** LOW

---

### 🟡 LOW — L-005: `ListUserTenantPermissions` called on every RoleUser request (N+1)

**File:** `server/router/api/v1/tenant_binding.go:60-66`
**Actual:** Every RoleUser request that hits the middleware triggers a `ListUserTenantPermissions` DB query. For high-traffic tenants, this adds latency to every admin page load. Not a security issue, but a performance concern.

**Verdict:** ✅ VERIFIED (works correctly, but consider caching per-user tenant list in context after first lookup per request cycle)

**Severity:** LOW (performance, not security)

---

## Verified Fixes (✅)

### Phase 0 — Emergency

**H7: Credential Rotation**
- `server/server.go`: N/A (credentials not in Go code)
- `.env`: Keys replaced with placeholders ✅
- `bugs/026/s3_probe/.env`: File absent from tree (see L-003)

**C2: Gate pprof Endpoints**
- `server/server.go:80-82`: Gated behind `profile.IsDev() || os.Getenv("MEMOS_ENABLE_PPROF") == "true"` ✅
- `server/profiler/profiler.go`: `StartMemoryMonitor` runs unconditionally (line 79) — this is safe, it only logs memory stats, doesn't expose routes.

**C3: Fix Echo Debug Mode**
- `server/server.go:51`: `echoServer.Debug = profile.IsDev()` ✅
- `server/server.go:57-71`: Custom error handler for prod ✅ (see L-001 for UX note)

### Phase 1 — Tenant Isolation

**H2 Part A: Fix `isSuperUser`**
- `server/router/api/v1/common.go:70-71`: Correctly excludes scoped admins ✅
- 29 call sites in `resource_service.go`, `memo_service.go`, `memo_relation_service.go`, `ticket_service.go` all inherit the fix.
- Consistent with `TenantBindingMiddleware` super-user check (tenant_binding.go:35) ✅

**H2 Part B: Tenant Filter Derivation**
- `server/router/api/v1/tenant_context.go:37-68`: `deriveTenantIDsForScopedAdmin` correctly handles all cases:
  - `user == nil` → nil ✅
  - Super users (RoleHost, global RoleAdmin) → nil ✅
  - Scoped admin → resolves GUIDs via `store.GetAgentTenant({GUID: &guid})` ✅
  - GUID lookup failure → `slog.Warn` + skip (graceful degradation) ✅
  - Empty result → returns `[]int32{}` (empty, not nil) → denies all ✅
  - RoleUser → nil (RBAC path handles it) ✅

- `server/router/api/v1/tenant_context.go:73-87`: `ApplyTenantFilter` for Memos exists but is DEAD CODE (see H-001).

- `server/router/api/v1/tenant_context.go:91-105`: `ApplyTicketTenantFilter` correctly called at `ticket_service.go:198` ✅

- `store/agent.go:29`: `FindAgentTenant.GUID *string` field present ✅
- `store/ticket.go:46`: `FindTicket.TenantIDs []int32` field present ✅
- `store/memo.go:85`: `FindMemo.TenantIDs []int32` field present ✅

- `store/db/sqlite/agent.go:59-62`: GUID filter `guid = ?` ✅
- `store/db/sqlite/ticket.go:79-85`: `tenant_id IN (?,?,?)` clause ✅
- `store/db/sqlite/memo.go:85-96`: `` `memo`.`tenant_id` IN (?,?,?) `` clause ✅
- `store/db/postgres/agent.go:55-70` + lines 78-82: GUID filter `guid = $N`. **Note:** `tenant.GUID = guid.String` is assigned unconditionally (no `guid.Valid` check). If the column were NULL, this would set GUID to empty string. Not exploitable but inconsistent with SQLite (which checks `guid.Valid`). ✅ (correct in practice since GUID column is NOT NULL in schema)

- `store/db/postgres/ticket.go:80-94`: `tenant_id IN ($1,$2,$3)` clause using `argCounter` ✅
- `store/db/postgres/memo.go:84-95`: `memo.tenant_id IN ($1,$2,$3)` clause using `placeholder()` helper ✅

**H1: Fix TenantBindingMiddleware**
- `server/router/api/v1/tenant_binding.go:16-72`: Complete rework ✅
  - Super users bypass correctly (RoleHost, global RoleAdmin) — line 35 ✅
  - Scoped admins checked via `contains(user.AllowedTenantIDs, tenant.GUID)` — line 55 ✅
  - RoleUser checked via `ListUserTenantPermissions` — line 60-66 ✅
  - Empty slug → skip (no-slug routes self-check) — line 40-44 ✅
  - `contains()` helper uses `strings.TrimSpace` (line 78) ✅

**C1: Permission Guards on 9 Handlers**
All 9 handlers verified with correct `isSuperAdmin` + permission check pattern:

| Handler | Line | Pattern Present | Permission |
|---------|------|----------------|------------|
| `HandleListTranscripts` | 5923 | ✅ | `PermChatLogs` |
| `HandleGetTranscript` | 5956 | ✅ | `PermChatLogs` |
| `HandleDeleteTranscript` | 5990 | ✅ | `PermChatLogs` |
| `HandleListLeads` | 6026 | ✅ | `PermTenantRead` |
| `HandleGetLead` | 6067 | ✅ | `PermTenantRead` |
| `HandleUpdateLeadStatus` | 6094 | ✅ | `PermTenantWrite` |
| `HandleExportLeads` | 6136 | ✅ | `PermTenantRead` |
| `HandleGetTenantSettings` | 6213 | ✅ | `PermTenantRead` |
| `HandleUpdateTenantSettings` | 6248 | ✅ | `PermTenantWrite` |

See M-002 for the scoped-admin `hasPermission` gap.

### Phase 2 — Hardening

**H3: Path Traversal Fix**
- `server/router/api/v1/resource_service.go:302-314`: `sanitizeFilename` defined ✅
- `server/router/api/v1/resource_service.go:325`: Called on save path ✅
- `server/router/api/v1/resource_service.go:345-349`: Containment assertion on save path ✅
- `server/router/api/v1/resource_service.go:417-421`: Containment assertion on read path ✅
- Missing: read path sanitizeFilename (see L-002), delete path (see H-002)

**H4: CSRF Fix**
- `server/router/api/v1/auth_service.go:307`: Always `SameSite=Lax` ✅
- `server/router/api/v1/csrf.go`: Complete middleware with:
  - Safe method skip (GET/HEAD/OPTIONS) — line 19-21 ✅
  - Bearer token skip (no cookie = no CSRF) — line 24-27 ✅
  - `Sec-Fetch-Site` preferred over `Origin` — line 30-38 ✅
  - Missing Origin + no Sec-Fetch-Site → allowed (SameSite=Lax as primary defense) — line 44-46 ✅
  - Origin hostname exact match against allowlist — line 59-74 ✅
  - Empty allowlist → denies all cross-origin (safe default) — line 60-62 ✅
- `server/router/api/v1/v1.go`:
  - Line 173: `gwGroup.Use(CSRFProtectionMiddleware(...))` ✅
  - Line 340: `adminGroup.Use(CSRFProtectionMiddleware(...))` ✅
  - Both use `getEnvSlice("CSRF_ALLOWED_ORIGINS", []string{})` ✅

**Potential CSRF bypass:** An attacker could strip both `Sec-Fetch-Site` and `Origin` headers (possible in older browsers or some configurations). The middleware allows such requests through (line 44-46). However, `SameSite=Lax` prevents the browser from sending cookies on cross-site requests in those cases, so this is safe.

**H5: Non-Root Container**
- `Dockerfile.fly:62-88`: gosu installed, non-root user created, no USER directive, entrypoint handles drop ✅
- `Dockerfile.s3.fly:63-88`: Identical setup ✅
- `scripts/entrypoint.sh:46-52`: Root → chown volume → gosu drop ✅ (but see M-004 for gosu-missing crash)

**H6: Webhook SSRF Fix**
- `server/router/api/v1/webhook_service.go:21-35`: `validateWebhookURL` (save-time UX pre-check) ✅
  - Called on create (line 47) ✅
  - Called on update when URL in update mask (line 115) ✅
- `plugin/webhook/webhook.go:27-40`: `isInternalIP` — blocks loopback, private, link-local, unspecified, AWS metadata (169.254.169.254), IPv6 metadata (fd00:ec2::254) ✅
- `plugin/webhook/webhook.go:45-87`: `validateAndResolveWebhookURL` — scheme check, DNS resolve once, iterate ALL IPs, reject internal, return first valid external ✅
- `plugin/webhook/webhook.go:109-122`: IP-pinned transport — `DialContext` forces connection to validated IP, no re-resolution ✅
- `plugin/webhook/webhook.go:128-135`: Redirect `CheckRedirect` — cap at 3 redirects, re-validate via `validateAndResolveWebhookURL` on each redirect ✅
- **No duplicate `DialContext`** (M1 from plan2 fixed — only one `DialContext` function at line 110) ✅
- Redirect handling note: The transport's `DialContext` still pins to the original `dialIP`. If a redirect goes to a different IP (same hostname, different DNS), the redirect validation at line 133 would **pass** (same hostname, different valid external IP), but the transport would still dial the **original** pinned IP. This is actually more secure — it prevents DNS-rebinding redirect attacks. ✅

---

## Answers to 10 Specific Questions

### Q1: Is `isSuperAdmin` consistent with `isSuperUser`?

**Yes.** Both check `RoleHost || (RoleAdmin && len(AllowedTenantIDs) == 0)`. `isSuperUser` in `common.go:70-71` and `isSuperAdmin` in `handlers.go:2263-2265` have identical logic. However, `isSuperAdmin` lacks audit logging (see M-001). If one is updated and the other isn't, they could diverge — there is no shared implementation.

### Q2: Does the `TenantIDs` SQL filter work for all databases?

**Yes for SQLite and Postgres.** Both drivers correctly implement `tenant_id IN (?,?,?)` / `tenant_id IN ($1,$2,$3)` with parameterized placeholders. MySQL was skipped per `code.md` ("MySQL skipped — doesn't have tenant_id columns"), which is acceptable since MySQL is not the primary target.

**However:** For Memos specifically, `TenantIDs` is never populated at the service layer (see H-001), so the SQL filter exists but is never activated for Memo queries. For Tickets, it works correctly via `ApplyTicketTenantFilter`.

### Q3: Is there any path where a scoped admin can access a tenant they shouldn't?

**Yes, two paths:**

1. **Memo queries (H-001):** If `isSuperUser` is missed in any memo_service.go code path, scoped admins get nil `TenantID` and nil `TenantIDs` = see all memos. No defense-in-depth SQL layer.
2. **Permission bypass (M-002):** `ResolveEffectivePermissions` grants `tenant:read` to any `RoleAdmin` regardless of `AllowedTenantIDs`. If H1 middleware fails (wrong route group, slug parsing issue), scoped admins pass C1 handler guards via `hasPermission`.

**Full request flow analysis:**

```
Request → Echo Router → AuthMiddleware → TenantBindingMiddleware (H1)
                                              ↓
                                    Scoped admin: contains(AllowedTenantIDs, tenant.GUID)?
                                    RoleUser: ListUserTenantPermissions?
                                              ↓
                                    Handler guard: isSuperAdmin || hasPermission (C1)
                                              ↓
                                    Store query: ApplyTenantFilter/ApplyTicketTenantFilter
                                    ApplyTenantFilter = DEAD CODE for Memos (H-001)
```

The H1 middleware is correctly implemented and should block at the first check. But the C1/ApplyTenantFilter layers are meant to be independent defense-in-depth — for Memos, they are not.

### Q4: Can an attacker bypass CSRF by using a different Content-Type?

**Not in the current implementation.** The CSRF middleware does not check Content-Type at all — it relies on `Sec-Fetch-Site` and `Origin` headers, which are independent of the request's Content-Type. A `multipart/form-data` POST from a cross-site page would still have `Sec-Fetch-Site: cross-site` and be blocked. However, an attacker could potentially use `text/plain` with `Sec-Fetch-Site: same-origin` (via form submission to the same origin with `enctype="text/plain"`), but `SameSite=Lax` cookies are not sent on cross-site form submissions, so the cookie wouldn't be present. The layered defense (SameSite=Lax + Sec-Fetch-Site + Origin fallback) is sound.

### Q5: Does the IP-pinned dialer follow redirects?

**Partially.** The `CheckRedirect` at `plugin/webhook/webhook.go:128-135` re-validates each redirect target via `validateAndResolveWebhookURL`. This blocks redirects to internal IPs. However, the **transport's `DialContext` still pins to the original `dialIP`** (resolved from the original URL). If a redirect goes to `example.com:8080/redirect` and the transport tries to dial `example.com:8080`, it would actually dial the original IP + port 8080. If the original URL was on port 443 and the redirect points to port 8080, the transport correctly uses port 8080 with the original IP. If the original host was `example.com` and the redirect is to `other.example.com` (different host), the redirect validation would re-resolve DNS for the new hostname, but the transport would still dial the original pinned IP. This is **safe** — it prevents the attack where a redirect points to a different IP.

### Q6: Is there any information leakage in error messages?

**Minor issues found:**

1. `TenantBindingMiddleware:25-26` — DB errors return `"failed to verify tenant binding"` which leaks that a user exists but verification failed (vs `"access denied"` for missing user at line 29). **LOW severity.**
2. `server.go:57-71` — All errors get `"Internal server error"` body regardless of actual status code. No sensitive info leaked. **Secure but misleading UX.** See L-001.

**No file paths, stack traces, or DB details found in error responses.** The prod error handler correctly suppresses internal details. ✅

### Q7: Are there any new race conditions?

**`deriveTenantIDsForScopedAdmin` in `tenant_context.go:37-68`:** The GUID-to-ID lookup iterates `user.AllowedTenantIDs` and calls `s.GetAgentTenant` for each GUID. If a tenant is deleted between the GUID lookup and the query execution, the list may include a now-deleted tenant ID. This is benign — the subsequent filtered query would return empty results for that tenant. No privilege escalation path found.

**No new race conditions identified in the changed code.** The existing codebase patterns (request-scoped DB queries, no shared mutable state in middleware) are preserved.

### Q8: Does the entrypoint handle the case where `gosu` is not installed?

**No.** See M-004. If running as root without gosu installed, the entrypoint crashes. This affects dev workflows where users run `sudo` or where the container image doesn't include gosu.

### Q9: Are there any existing tests that now fail?

Not verifiable from static analysis alone. `code.md` claims tests pass. The `isSuperUser` change (excluding scoped admins) could break tests that assumed scoped admins had super-user access to all tenants — a test suite run is recommended.

### Q10: Is there any code path that bypasses the new guards?

**Two paths identified:**

1. `memo_service.go` uses `GetTenantIDFromContext(ctx)` (gRPC context, returns nil for admins) instead of `ApplyTenantFilter` (Echo context, handles scoped admins). The gRPC-based memo handlers bypass the `TenantIDs` filter entirely. Mitigated by `isSuperUser` checks.

2. `HandleDeleteTenant` and other global tenant management handlers use `isAdmin` (which passes scoped admins) instead of `isSuperAdmin` (which blocks them). Scoped admins retain global destructive capabilities.

**Middleware ordering in v1.go:**

```
adminGroup → AuthMiddleware → adminCORS → TenantBindingMiddleware → CSRFProtectionMiddleware → route handlers
```

The ordering is correct — auth runs first, then tenant binding, then CSRF. No ordering bypass found.

---

## Additional Findings Not in Original Scope

### INFO — N-001: `postgres/agent.go` null handling differs from SQLite

**File:** `store/db/postgres/agent.go:82`

SQLite checks `if guid.Valid { t.GUID = guid.String }`. Postgres assigns `tenant.GUID = guid.String` unconditionally (without checking `guid.Valid`). If the GUID column were ever NULL, Postgres would store an empty string. Since the GUID column has a NOT NULL constraint, this is benign.

### INFO — N-002: `sanitizeFilename` does not handle Windows-style backslash paths

**File:** `server/router/api/v1/resource_service.go:307`

`filepath.Base("foo\\bar")` on Linux returns `"foo\\bar"` unchanged (backslash is not a path separator on Linux). The containment assertion would catch `/data/foo\\bar` as within the data directory, but the filename itself would contain a backslash, which is unusual. Not exploitable on Linux but worth noting for defense-in-depth.

### INFO — N-003: `validateWebhookURL` allows `http://` (no TLS enforcement)

**File:** `server/router/api/v1/webhook_service.go:21-35`

The save-time validation allows both `http` and `https`. The dispatch-time `isInternalIP` check blocks internal targets, but an attacker could use plaintext HTTP to an external host for man-in-the-middle. This is a deliberate design choice (not all webhook endpoints support HTTPS). The risk is mitigated by the internal-IP block at dispatch.

---

## Final Checklist

- [x] All 10 questions answered
- [x] Every file in `code.md` reviewed (22/22 files)
- [x] Regressions documented (2: entrypoint gosu crash, sanitizeFilename ordering)
- [x] New vulnerabilities documented (H-001, H-002)
- [ ] Build still passes — **not verified** (run `go build ./...`)
- [ ] Tests still pass — **not verified** (run `go test ./server/router/api/v1/...`)
- [x] No middleware ordering issues found

---

## Severity Summary

| Severity | Count | IDs |
|----------|-------|-----|
| CRITICAL | 0 | — |
| HIGH | 2 | H-001 (dead-code ApplyTenantFilter for Memos), H-002 (missing containment assertion on delete) |
| MEDIUM | 5 | M-001 (no audit logging in isSuperAdmin), M-002 (C1 relies on H1 for scoped admins), M-003 (isAdmin on global handlers), M-004 (gosu crash), M-005 (null-byte ordering) |
| LOW | 5 | L-001 (error handler UX), L-002 (no sanitizeFilename on read), L-003 (s3_probe/.env untraceable), L-004 (middleware error info leak), L-005 (N+1 DB queries) |
| INFO | 3 | N-001 (postgres null handling diff), N-002 (windows path), N-003 (plaintext HTTP allowed) |
| ✅ VERIFIED | 16 | Core fixes for H7, C2, C3, H2A, H2B (tickets), H1, C1 (9 handlers), H3 (save+read), H4, H5 (Dockerfiles), H6 |

---

*Review completed 2026-07-09 by DeepSeek V4 Pro via OpenCode CLI. All 22 files in `code.md` reviewed against working tree. Build and test verification recommended before merge.*
