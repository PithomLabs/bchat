Now I have sufficient code-level evidence to deliver the security review. Here is the consolidated report.

---

## Security Review: bchat Codebase

### Methodology
Verified claims in `bugs/019/plan_priority.md` and `bugs/019/sec_review_deepseek.md` against the actual source code, focusing on `server/router/api/v1/`, `fly.toml`, `Dockerfile.local.fly`, and the `.env` working-tree file.

---

### CONFIRMED CRITICAL VULNERABILITIES (Not just config)

**1. No Session Revocation on Password Change (GO-2025-4127 / CVE-2024-21635)**
- **Files:** `server/router/api/v1/user_service.go:209-215`
- `UpdateUser` updates `PasswordHash` but never invalidates stored access tokens or touches any session blacklist. An attacker who stole a JWT before the password change retains full access indefinitely.
- **Verdict:** Confirmed critical. The docs correctly flag this.

**2. `NeverExpire` Creates 100-Year Tokens**
- **File:** `server/router/api/v1/auth_service.go:159-163`
- The client can set `request.NeverExpire = true`, which sets expiration to `time.Now().Add(100 * 365 * 24 * time.Hour)`. Neither security doc mentions this bypass.
- **Verdict:** Confirmed high. The docs correctly identify 7-day tokens but miss the permanent-token bypass.

**3. Raw JWT Tokens Leaked via `ListUserAccessTokens` Response**
- **File:** `server/router/api/v1/user_service.go:385-386`
- The gRPC response populates `userAccessToken.AccessToken` with the unredacted JWT string. Any process or log that records this response leaks the bearer token.
- **Verdict:** Confirmed high. Not mentioned in either doc.

**4. Hardcoded JWT Secret Fallback in Production**
- **File:** `server/server.go:66-69`
- `secret := "usememos"` is the default; in prod it switches to `workspaceBasicSetting.SecretKey`. If that database field is empty or uninitialized on first deploy, the app signs all JWTs with the well-known string `"usememos"`.
- **Verdict:** Confirmed high. Not mentioned in either doc.

**5. Stored XSS in Widget Fallback via `companyName`**
- **File:** `server/router/api/v1/agent/handlers.go:1661-1657` (the `generateWidgetScript` fallback, not the modern `generateWidgetLoaderScript`)
- `companyName` is concatenated into a JavaScript string literal via `'` + companyName + `'` with zero escaping. A tenant-supplied `CompanyName` containing `'</script><script>steal(document.cookie)</script>` executes in every visitor's browser.
- **Verdict:** Confirmed critical in the fallback path. The modern loader (line 1641-1657) uses `%q` and is safe. Not mentioned in either doc.

**6. Admin Mutation Rate Limiting Is Incomplete**
- **File:** `server/router/api/v1/agent/handlers.go`
- `checkAdminMutationRateLimit` is only invoked at lines 2640, 2707, 2777, 2818 (role-template CRUD + assign). It is **absent** from:
  - `HandleSetLLMConfig` (line 2309)
  - `HandleOnboard` (line 1225)
  - `HandleDeleteTenant` (line 1575)
  - `HandleGrantPermission` (line 2459)
  - `HandleRevokePermission` (line 2527)
  - `HandleRestoreFileVersion` (line 854)
  - `HandleImportSingleFile` (line 974)
  - `HandleImport` (line 1327)
  - `HandleReindexTenant` (line 1088)
- **Verdict:** Confirmed high. The docs correctly flag this.

**7. `isDomainAllowed` Fail-Open on Invalid Config**
- **File:** `server/router/api/v1/agent/handlers.go:1881-1891`
- Empty JSON string, invalid JSON, or empty array all return `true` (allow all). If the `AllowedDomains` column is corrupted or misconfigured, CORS-like widget protection silently disables.
- **Verdict:** Confirmed medium-high. The docs correctly flag this.

**8. Tenant API Key Falls Back to Global Key on Decryption Failure**
- **File:** `server/router/api/v1/agent/service.go:1205-1226`
- If `Decrypt` fails for a tenant-specific key, it logs a warning and silently falls back to `s.profile.OpenRouterAPIKey` (the global key). This breaks tenant isolation for LLM billing and logging.
- **Verdict:** Confirmed high. Not mentioned in either doc.

**9. `UpsertAccessTokenToStore` Appends Without Deduplication**
- **File:** `server/router/api/v1/user_service.go:506-528`
- Repeated logins or token refreshes append duplicate token rows indefinitely. No uniqueness check before append.
- **Verdict:** Confirmed low-medium. Not mentioned in either doc.

**10. `convertWorkspaceSettingToStore` Swallows Name-Parse Error**
- **File:** `server/router/api/v1/workspace_setting_service.go:103`
- `ExtractWorkspaceSettingKeyFromName(setting.Name)` error is discarded with `_, _`. A malformed `Name` silently maps to the zero-value enum, and the switch at lines 110-123 defaults to `GeneralSetting`, potentially writing general settings data under the wrong key.
- **Verdict:** Confirmed medium. Not mentioned in either doc.

**11. Working-Tree `.env` Contains Live Production Secrets**
- **File:** `.env:16`, `.env:38`
- `OPENROUTER_API_KEY=REDACTED_KEY...` and `ENCRYPTION_MASTER_KEY=e2590f42-...` are present in the repository working tree. While `.env` is correctly listed in `.gitignore:92` and is not tracked in git, these credentials are visible to anyone with filesystem access.
- **Verdict:** High. The docs mention `.env` sourcing as a general risk but do not note that the working tree currently contains real, active credentials.

---

### INFRASTRUCTURE CONFIRMATIONS

| Item | Status | Evidence |
|---|---|---|
| SQLite in production | Confirmed | `fly.toml:12-30`, `Dockerfile.local.fly:83-85,89-95` |
| `min_machines_running = 0` | Confirmed | `fly.toml:37` |
| Shared CPU (`cpu_kind = 'shared'`) | Confirmed | `fly.toml:47` |
| No `[[checks]]` health checks | Confirmed | `fly.toml` has no `[[checks]]` block |
| No WAF/proxy rules for CVE-2026-6634 | Confirmed | No nginx sidecar or Fly edge rules configured |
| Env vars exposed in `fly.toml [env]` | Confirmed | `fly.toml:12-26` contains `EMBEDDING_MODEL`, `LLM_MODEL`, `MEMOS_MODE`, etc. |

---

### CVE VERDICTS

| CVE | Status | Notes |
|---|---|---|
| **CVE-2026-6634** | **Real, upstream** | Affects `memos` ≤0.22.1 frontend component `UpdateInstanceSetting` in `src/App.tsx`. The bchat backend does **not** directly expose `UpdateInstanceSetting`; backend workspace settings require `RoleHost`. Risk is from the bundled upstream frontend. |
| **GO-2025-4127** / CVE-2024-21635 | **Real, applicable** | Confirmed in code: password change in `user_service.go:209-215` does not invalidate access tokens. |
| **CVE-2025-3492 (SSRF)** | Not directly assessable | `plugin/httpgetter` exists in the repo but was not reviewed in depth; verify `MEMOS_ALLOW_PRIVATE_WEBHOOKS=false`. |
| **CVE-2025-3936 (Path traversal)** | Not directly assessable | Attachment upload path not reviewed in depth; requires separate file-upload sanitization audit. |

---

### INACCURACIES IN THE REVIEW DOCUMENTS

**`plan_priority.md` §9 (lines 58-61): `HandleChatExternal` has "no rate limiting"**
- **Verdict: INACCURATE.** `server/router/api/v1/agent/service.go:1514` calls `s.CheckRateLimit(ctx, config.TenantID, "external", clientIP, config.Audience.RateLimitRPM)`. Rate limiting is implemented in the service layer.

**`sec_review_deepseek.md` (line 109): "HandleChatExternal has no rate limiting beyond what the LLM provider imposes"**
- **Verdict: INACCURATE.** Same as above — IP-based rate limiting is enforced before the LLM call.

**Neither document mentions:**
- `NeverExpire` (100-year JWT bypass)
- Raw JWT leakage via `ListUserAccessTokens`
- Tenant API key global fallback on decryption error
- Stored XSS in widget loader fallback
- Hardcoded `"usememos"` JWT secret fallback
- Live secrets present in the working-tree `.env`

---

### TIERED REMEDIATION PRIORITY

**P0 — Fix before any customer goes live**
1. **Invalidate all access tokens on password change** (`user_service.go:209`) — call `DeleteUserAccessToken` for all tokens belonging to the user, or introduce a token version field.
2. **Remove or guard `NeverExpire`** — reject `request.NeverExpire` in production, or cap at a sane maximum (e.g., 30 days).
3. **Redact JWT tokens in `ListUserAccessTokens`** — return only metadata (issued-at, description, last-used), never the raw token string.
4. **Fix `isDomainAllowed` fail-open** — return `false` on JSON parse error and on empty list; log a warning so misconfiguration is visible.
5. **Eliminate `"usememos"` hardcoded secret** — fail to start if `workspaceBasicSetting.SecretKey` is empty in prod; generate and persist one on first boot.
6. **Escape `companyName` in widget fallback** (`handlers.go:1661`) — use `%q` or equivalent JS-string escaping.

**P1 — Fix before scaling beyond first customer**
7. **Add rate limiting to `HandleSetLLMConfig`, `HandleOnboard`, `HandleDeleteTenant`, `HandleGrantPermission`, `HandleRevokePermission`, `HandleRestoreFileVersion`, `HandleImportSingleFile`, `HandleImport`, `HandleReindexTenant`** — wrap each in `checkAdminMutationRateLimit`.
8. **Fail-closed on tenant API key decryption error** (`service.go:1205-1226`) — do not fall back to global key; return an error indicating the tenant key is broken.
9. **Deduplicate access tokens on upsert** — check for existing token before appending.
10. **Handle `ExtractWorkspaceSettingKeyFromName` error** (`workspace_setting_service.go:103`) — return `InvalidArgument` instead of silently defaulting.

**Infrastructure (from docs, confirmed in code)**
11. Migrate from SQLite to Postgres.
12. Set `min_machines_running = 1`.
13. Change `cpu_kind = 'performance'`.
14. Add `[[checks]]` health check block.
15. Add WAF rules for CVE-2026-6634 frontend paths.

**Immediate hygiene**
16. Rotate the `OPENROUTER_API_KEY` and `ENCRYPTION_MASTER_KEY` currently in `.env` (lines 16, 38). They are not tracked in git but are present in the working tree. Ensure deployment uses Fly secrets (`fly secrets set`), not files.
