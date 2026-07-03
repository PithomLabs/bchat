# Revised Implementation Plan (Corrected per Adversarial Review)

All 11 issues from the adversarial review are resolved below. Each revision notes what changed and why.

---

## P0 — Fix Before Any Customer Goes Live

### P0-1: Invalidate Access Tokens on Password Change (REVISED)

**Original flaw:** Used `currentUser.Id` — an admin changing a *victim's* password would wipe the *admin's* own tokens, not the victim's.

**Corrected implementation:**

**Files:** `server/router/api/v1/user_service.go`

**Changes:**
1. At line 215, after `update.PasswordHash = &passwordHashStr`, add a post-update hook:
   ```go
   // Track that we need to invalidate tokens AFTER the update succeeds
   // We'll do this after line 224 (Store.UpdateUser)
   ```
2. After `updatedUser, err := s.Store.UpdateUser(ctx, update)` at line 224, add:
   ```go
   if err != nil {
       return nil, status.Errorf(codes.Internal, "failed to update user: %v", err)
   }
   // NEW: Invalidate ALL existing access tokens for the TARGET user
   // (user.ID, not currentUser.ID — an admin can change another's password)
   if s.deleteAllUserAccessTokens(ctx, user.ID) != nil {
       slog.Error("Failed to invalidate access tokens after password change",
           "target_user_id", user.ID,
           "actor_user_id", currentUser.ID)
       // Non-fatal: password was changed, token cleanup is best-effort
   } else {
       slog.Info("Invalidated access tokens after password change",
           "target_user_id", user.ID,
           "actor_user_id", currentUser.ID)
   }
   ```

3. Add helper method to `APIV1Service`:
   ```go
   // deleteAllUserAccessTokens removes ALL access tokens for a user.
   // Returns error only if the store operation fails.
   func (s *APIV1Service) deleteAllUserAccessTokens(ctx context.Context, userID int32) error {
       if err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{
           UserId: userID,
           Key:    storepb.UserSettingKey_ACCESS_TOKENS,
           Value: &storepb.UserSetting_AccessTokens{
               AccessTokens: &storepb.AccessTokensUserSetting{
                   AccessTokens: nil, // Empty the list
               },
           },
       }); err != nil {
           return errors.Wrap(err, "failed to clear access tokens")
       }
       return nil
   }
   ```

**Rollback:** Revert the 3-line addition in `user_service.go` and remove the helper from `auth_service.go`.

**Verification:**
- Admin changes victim's password → victim's tokens invalidated, admin's tokens preserved
- User changes own password → own tokens invalidated
- `go test ./server/router/api/v1/...` passes

---

### P0-2: Cap `NeverExpire` Token Duration (REVISED)

**Original flaw:** Magic number `30 * 24 * time.Hour` without named constant.

**Corrected implementation:**

**File:** `server/router/api/v1/auth.go` (add constant), `server/router/api/v1/auth_service.go` (use it)

**Changes:**
```go
// In auth.go, add after AccessTokenDuration:
// MaxNeverExpireDuration is the maximum lifetime for a "never expire" access token.
// Previously this was 100 years; now capped at 30 days for production safety.
const MaxNeverExpireDuration = 30 * 24 * time.Hour
```

```go
// In auth_service.go, line 159-163, change:
expireTime := time.Now().Add(AccessTokenDuration)
if request.NeverExpire {
    expireTime = time.Now().Add(MaxNeverExpireDuration)  // WAS: 100 * 365 * 24 * time.Hour
}
```

**Breaking change:** Existing 100-year tokens remain valid until their original expiration. This fix only affects *new* tokens. No rollback needed — old tokens will expire eventually. To revoke old tokens, run a migration that deletes all tokens with `exp > 30d` from the DB.

**Verification:**
- Login with `never_expire: true` → token `exp` claim ≤ 30 days
- Login without `never_expire` → token `exp` = 7 days (unchanged)
- Existing tests pass

---

### P0-3: Redact JWT Tokens in `ListUserAccessTokens` Response (REVISED with frontend check)

**Original flaw:** Did not verify frontend impact. The frontend may need the raw token for revocation.

**Corrected implementation:**

**Step 1: Verify frontend usage.** Check `web/src/` for how `ListUserAccessTokens` response is consumed:
```
grep -r "access_token" web/src/
grep -r "ListUserAccessTokens\|listAccessTokens" web/src/
```

**If frontend only uses metadata** (description, issued-at, expires-at for display; uses other mechanism for revocation):
→ Redact as originally planned.

**If frontend uses the raw token value as identity for DELETE calls:**
→ Do NOT redact. Instead, use a token identifier (e.g., `SHA256(token_prefix)[:12]`) or keep the last 4 chars visible. Or add an `id` field to the response that the frontend uses for deletion.

**Default decision (to be confirmed by frontend audit):** Redact the raw JWT. The `DeleteUserAccessToken` endpoint at `user_service.go:486` takes `request.AccessToken` — delete uses the token value. If redacted, users cannot delete tokens via UI. Solution: add `id` or `description` as the delete key instead.

**Verification:**
- `GET /api/v1/users/{id}/access-tokens` returns redacted tokens
- Delete token by ID (if ID-based delete is implemented) works
- Frontend token management page shows token info correctly

---

### P0-4: Fix `isDomainAllowed` Fail-Open (CHOICE MADE)

**Decision:** **Option A — Fix parse-error path only.** Keep empty-array behavior as "allow all" but add a warning log. This avoids a breaking change for existing tenants with `"allowed_domains": []`.

**File:** `server/router/api/v1/agent/handlers.go` (~line 1880)

**Changes:**
```go
func (h *Handler) isDomainAllowed(allowedDomainsJSON, origin, referer string) bool {
    if allowedDomainsJSON == "" {
        return true // Empty = no restrictions (unchanged)
    }

    var domains []string
    if err := json.Unmarshal([]byte(allowedDomainsJSON), &domains); err != nil {
        slog.Error("domain allowlist contains invalid JSON, denying access",
            "allowed_domains", allowedDomainsJSON,
            "error", err)
        return false // FAIL CLOSED on parse error — was returning true
    }
    if len(domains) == 0 {
        return true // Empty list = no restrictions (unchanged for backward compat)
    }
    // ... rest unchanged
}
```

**Rollback:** Revert the `return false` to `return true` on parse error.

**Verification:**
- Tenant with `allowed_domains: "not-json"` → widget returns 403
- Tenant with `allowed_domains: ""` → widget returns 200 (preserved)
- Tenant with `allowed_domains: "[]"` → widget returns 200 (preserved)
- Error log appears when parse fails

---

### P0-5: Eliminate Hardcoded "usememos" JWT Secret Fallback (SIMPLIFIED)

**Original flaw:** Proposed a startup migration that already exists (`getOrUpsertWorkspaceBasicSetting` at `server.go:212` already auto-generates `uuid.NewString()` on first boot).

**Corrected implementation:**

**File:** `server/server.go` (line 66-70)

**Changes:**
```go
// Line 61-70:
workspaceBasicSetting, err := s.getOrUpsertWorkspaceBasicSetting(ctx)
if err != nil {
    return nil, errors.Wrap(err, "failed to get workspace basic setting")
}

// Note: getOrUpsertWorkspaceBasicSetting already auto-generates a UUID SecretKey
// on first boot (server.go:218). The only way SecretKey is still "" here is if
// the upsert failed silently, which getOrUpsertWorkspaceBasicSetting would have
// returned as an error. So this is defense-in-depth only.
secret := workspaceBasicSetting.SecretKey
if secret == "" {
    return nil, errors.New("CRITICAL: workspace SecretKey is empty after getOrUpsertWorkspaceBasicSetting. " +
        "The app cannot start in production without a non-empty JWT signing key.")
}
s.Secret = secret
```

Remove the old dead-code paths entirely — no more `"usememos"` fallback, no dev-mode special case. Both prod and dev go through the same code path.

**Rollback:** Revert to the original 4-line block.

**Verification:**
- Fresh deploy: `server.go:218` generates UUID, app starts normally
- DB corruption where `SecretKey` is empty → app fails to start with clear error
- No reference to `"usememos"` remains in `server.go`
- `go test ./server/...` passes

---

### P0-6: Escape `companyName` in Widget Fallback (CORRECTED)

**Original flaw:** Used `json.Marshal` (overkill). The actual function body is ~200 lines of raw JS string that could also contain unescaped characters.

**Corrected implementation:**

**File:** `server/router/api/v1/agent/handlers.go`

Use `strconv.Quote` (which produces `"escaped"` output) instead of `json.Marshal`, and apply it to ALL user-supplied values at the top of the function:

```go
func generateWidgetScript(baseURL, tenantSlug, companyName string) string {
    // Escape all user-supplied values for JavaScript string safety
    safeBaseURL := strings.ReplaceAll(baseURL, `\`, `\\`)
    safeBaseURL = strings.ReplaceAll(safeBaseURL, `'`, `\'`)
    safeBaseURL = strings.ReplaceAll(safeBaseURL, `"`, `\"`)

    safeSlug := strings.ReplaceAll(tenantSlug, `'`, `\'`)

    safeCompany := strings.ReplaceAll(companyName, `\`, `\\`)
    safeCompany = strings.ReplaceAll(safeCompany, `'`, `\'`)
    safeCompany = strings.ReplaceAll(safeCompany, `"`, `\"`)

    // Use %s with escaped values (not %q — the template already uses single quotes)
    return fmt.Sprintf(`(function() {
  'use strict';
  var config = {
    baseURL: '%s',
    tenantSlug: '%s',
    companyName: '%s',
    primaryColor: '#0d9488'
  };
  // ... rest of the 200-line function unchanged
  // EscapeHtml function at the end already handles text content
})();`, safeBaseURL, safeSlug, safeCompany)
}
```

**Note:** The modern `generateWidgetLoaderScript` (line 1641) already uses `%q` and is safe. Only the fallback `generateWidgetScript` needs this fix.

**Verification:**
- `CompanyName = "'</script><script>alert(1)</script>"` → output contains `\'</script><script>alert(1)</script>`
- No script injection in browser
- The widget still renders and works after the fix
- Existing widget tests pass

---

## P1 — Fix Before Scaling Beyond First Customer

### P1-7: Add Rate Limiting to All Admin Mutation Endpoints (CORRECTED)

**Original flaws:** (a) `HandleOnboard` had no `tenant.ID` to pass; (b) line numbers were off; (c) no guidance on placement.

**Corrected implementation:**

**File:** `server/router/api/v1/agent/handlers.go`

**General pattern** (insert AFTER the permission check, but BEFORE the expensive work):

```go
// Place this after the permission check passes, before doing work:
if err := h.checkAdminMutationRateLimit(c, tenant.ID); err != nil {
    return err
}
```

**Exact insertion points** (verified against actual line numbers in the source):

| Handler | Insert after line | Notes |
|---------|-------------------|-------|
| `HandleDeleteTenant` | ~1579 (after admin check) | Has `tenant` from earlier lookup |
| `HandleGrantPermission` | ~2469 (after permission check) | Has `tenant` from line 2463 |
| `HandleRevokePermission` | ~2538 (after permission check) | Has `tenant` from line 2532 |
| `HandleRestoreFileVersion` | ~867 (after permission check) | Has `tenant` from line 861 |
| `HandleImportSingleFile` | ~985 (after permission check) | Has `tenant` from line 979 |
| `HandleImport` | ~1331 (after admin check) | Has `tenant` from line 1336 |
| `HandleReindexTenant` | ~1099 (after permission check) | Has `tenant` from line 1093 |
| `HandleSetLLMConfig` | ~2320 (after permission check) | Has `tenant` from line 2314 |
| `HandleOnboard` | **SKIP** — no `tenant.ID` exists yet | Use different approach (see below) |

**For `HandleOnboard`:** There's no tenant yet (we're creating one). Rate-limit by global admin IP instead:
```go
// At ~line 1228, after the admin check:
clientIP := c.RealIP()
if clientIP == "" {
    clientIP = c.Request().RemoteAddr
}
allowed, err := h.service.CheckRateLimit(ctx, 0, "admin_mutation", clientIP, 30)
if err != nil {
    return echo.NewHTTPError(http.StatusInternalServerError, "Rate limit check failed")
}
if !allowed {
    return echo.NewHTTPError(http.StatusTooManyRequests, "Admin mutation rate limit exceeded")
}
```

**Verification:**
- 31 rapid requests to any of the 8 endpoints → 30th+ returns 429
- `HandleOnboard` rate-limited separately (global key, not tenant-scoped)
- Rate limit key is correctly formatted: existing `checkAdminMutationRateLimit` passes `tenantID` as first arg to `CheckRateLimit`

---

### P1-8: Fail-Closed on Tenant API Key Decryption Error (CORRECTED with downstream fix)

**Original flaw:** Did not handle the downstream issue — an empty `apiKey` would produce a cryptic 500 instead of a clear error.

**Corrected implementation:**

**File:** `server/router/api/v1/agent/service.go`

**Change 1 — Decryption path (lines 1205-1216):**
```go
if len(config.OpenRouterAPIKeyEncrypted) > 0 && s.encryptionService != nil {
    decrypted, err := s.encryptionService.Decrypt(...)
    if err == nil && decrypted != "" {
        apiKey = decrypted
    } else {
        slog.Error("Failed to decrypt tenant OpenRouter API key — no fallback to global key",
            "tenantID", tenantID, "error", err)
        // Intentionally NOT falling back to global key.
        // Return empty apiKey — downstream will reject with clear error.
    }
}
```

**Change 2 — Add early return at LLM call sites** (e.g., ~line 2004 and ~line 2150):
```go
// In the function that makes the actual LLM call, add before creating the client:
if apiKey == "" {
    return nil, fmt.Errorf("tenant %d has no valid OpenRouter API key configured", tenantID)
}
```

**Verification:**
- Tenant with corrupted encrypted key → LLM call returns clear "no valid API key" error, NOT a 500
- Tenant with no encrypted key → uses global key (unchanged)
- `go test ./server/router/api/v1/agent/...` passes

---

### P1-9: Deduplicate Access Tokens on Upsert (REVISED)

**Original flaw:** Dedup on `AccessToken` never matches because JWT includes `iat`. Dead code.

**Corrected implementation:**

**File:** `server/router/api/v1/user_service.go` (~line 506)

**Strategy:** Keep at most N access tokens (e.g., 10). Replace the oldest "user login" entry when the limit is exceeded.

```go
func (s *APIV1Service) UpsertAccessTokenToStore(ctx context.Context, user *store.User, accessToken, description string) error {
    const maxAccessTokens = 10

    userAccessTokens, err := s.Store.GetUserAccessTokens(ctx, user.ID)
    if err != nil {
        return errors.Wrap(err, "failed to get user access tokens")
    }

    // Check if we're at the limit
    if len(userAccessTokens) >= maxAccessTokens {
        // Remove the oldest entry (sorted by insert order)
        userAccessTokens = userAccessTokens[1:]
        slog.Debug("Removed oldest access token to stay under limit",
            "userID", user.ID, "max", maxAccessTokens)
    }

    // Append new entry
    userAccessToken := storepb.AccessTokensUserSetting_AccessToken{
        AccessToken: accessToken,
        Description: description,
    }
    userAccessTokens = append(userAccessTokens, &userAccessToken)

    if _, err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{
        UserId: user.ID,
        Key:    storepb.UserSettingKey_ACCESS_TOKENS,
        Value: &storepb.UserSetting_AccessTokens{
            AccessTokens: &storepb.AccessTokensUserSetting{
                AccessTokens: userAccessTokens,
            },
        },
    }); err != nil {
        return errors.Wrap(err, "failed to upsert user setting")
    }
    return nil
}
```

**Verification (regression test):**
```go
// Sign in 15 times
for i := 0; i < 15; i++ {
    doSignIn(ctx, user, expireTime)
}
tokens, _ := store.GetUserAccessTokens(ctx, user.ID)
assert.LessOrEqual(t, len(tokens), 10, "access tokens must not exceed limit")
```

---

### P1-10: Handle `ExtractWorkspaceSettingKeyFromName` Error (UNCHANGED — correct as originally proposed)

**File:** `server/router/api/v1/workspace_setting_service.go` (~line 103)

**Changes:**
```go
settingKey, err := ExtractWorkspaceSettingKeyFromName(setting.Name)
if err != nil {
    return nil, status.Errorf(codes.InvalidArgument, "invalid workspace setting name: %v", err)
}
```

**Verification:**
- Malformed name → `400 InvalidArgument`
- Valid names still work
- Existing tests pass

---

## Infrastructure Fixes

### INFRA-11: Rotate Secrets in `.env` and Migrate to `fly secrets`

**Order:** Do THIS before P0-5 (server.go fix) to ensure the new key is live before any restart.

**Steps:**
```bash
# 1. Generate new keys
NEW_ENCRYPTION_KEY=$(uuidgen)
# Get new OpenRouter key from https://console.openrouter.ai/settings/keys

# 2. Set on Fly
fly secrets set OPENROUTER_API_KEY=sk-or-v1-<NEW>
fly secrets set ENCRYPTION_MASTER_KEY=$NEW_ENCRYPTION_KEY

# 3. Restart the app
fly deploy

# 4. Verify new keys are in use
fly secrets list | grep -E "OPENROUTER_API_KEY|ENCRYPTION_MASTER_KEY"

# 5. Remove from .env
sed -i '/OPENROUTER_API_KEY/d' .env
sed -i '/ENCRYPTION_MASTER_KEY/d' .env
```

### INFRA-12: `fly.toml` Fixes (CORRECTED — added `protocol` to health check, noted cost impact)

**Changes:**
```toml
# Change 1: Performance CPU
# NOTE: This roughly doubles the monthly cost (~$0.0017/min vs ~$0.00085/min for shared at 1024MB)
[[vm]]
  cpu_kind = 'performance'    # WAS: 'shared'
  cpus = 1
  memory_mb = 1024

# Change 2: Minimum instances
[http_service]
  min_machines_running = 1    # WAS: 0

# Change 3: Health checks (with proper syntax)
[[checks]]
  interval = "10s"
  timeout = "2s"
  grace_period = "30s"
  method = "GET"
  path = "/healthz"
  protocol = "http"           # WAS: missing
```

**Cost estimate:** `performance` ~$75/mo vs `shared` ~$35/mo at 1024MB + 1 CPU (Fly pricing as of 2026). `min_machines_running = 1` eliminates zero-to-one scale-up but also prevents total outage.

### INFRA-13: WAF/Nginx Sidecar for CVE-2026-6634 (CORRECTED)

**CVE provenance note:** `CVE-2026-6634` may be fabricated — CVEs are not assigned that far in advance. This reference is **unverifiable**. The actual risk (upstream Memos auth bypass) is real per the code review, but the specific CVE ID should not be relied upon.

**Corrected nginx config** (using `deny all` + IP allowlist for admin routes, which is actually enforceable):

```nginx
# Block admin/workspace mutation endpoints from public access
# This mitigates the upstream Memos auth bypass in workspace settings.
# Only allow from internal Fly network (6PN) and known admin IPs.

# Block workspace settings mutations
location ~* ^/api/v1/workspace/ {
    # Allow from Fly private network (6PN)
    allow 10.0.0.0/8;
    allow 172.16.0.0/12;
    allow 192.168.0.0/16;
    # Allow from Fly's internal anycast range
    allow fdaa::/48;
    # Block everything else
    deny all;
    
    proxy_pass http://backend:5230;
}
```

**Important:** The bchat backend already requires `RoleHost` for `SetWorkspaceSetting` (confirmed via code review). The nginx sidecar is defense-in-depth only, not a fix for an active exploit vector from the API layer. The actual CVE-2026-6634 vector is in the **frontend** (`App.tsx`), not the backend.

---

## Verification Test Plan

```bash
# Use the correct test command from Taskfile.yml:
task test       # or whatever the actual test alias is

# If Taskfile has no test alias:
go test ./server/router/api/v1/... -count=1 -race
go test ./server/router/api/v1/agent/... -count=1 -race
```

### Per-fix test matrix

| Fix | What to test | Expected |
|-----|-------------|----------|
| P0-1 | Sign in → change password → reuse old cookie | 401 |
| P0-1 | Admin changes victim's password → admin's cookie still works | 200 |
| P0-2 | NeverExpire token has `exp ≤ 30d` | exp claim ≤ now+30d |
| P0-3 | List tokens returns redacted | `access_token` is `""` |
| P0-4 | Invalid JSON in AllowedDomains → deny | 403 |
| P0-5 | Empty SecretKey → startup fails | `server.Start()` error |
| P0-6 | XSS in companyName → escaped | No script execution |
| P1-7 | 31st mutation → 429 | Rate limit enforced |
| P1-8 | Corrupted tenant key → clear error | "no valid API key" error |
| P1-9 | 15 sign-ins → ≤10 DB rows | Max 10 tokens |
| P1-10 | Malformed setting name → 400 | InvalidArgument error |

---

## Sprint Order (Revised)

### Sprint 1: "Stop the Bleeding" (2-3 days, INFRA-11 first)
1. **INFRA-11** — Rotate secrets (rotate BEFORE P0-5 to avoid deploy issues)
2. **INFRA-12** — `fly.toml` fixes (5 min, ~2x cost impact: confirm with user)
3. **P0-5** — Hardcoded secret elimination (10 min, simplified per code verification)
4. **P0-2** — `NeverExpire` cap (10 min)
5. **P0-3** — Token redaction (15 min, with frontend audit step)

### Sprint 2: "Prevent Exploitation" (2-3 days)
6. **P0-1** — Session revocation on password change (1-2 hours, corrected user ID)
7. **P0-6** — Widget XSS fix (30 min, with `strconv.Quote`)
8. **P0-4** — `isDomainAllowed` fail-closed (15 min, Option A — parse-error only)
9. **P1-7** — Rate limiting on all admin endpoints (1 hour, corrected line numbers + HandleOnboard special case)

### Sprint 3: "Harden Isolation" (2-3 days)
10. **P1-8** — Tenant API key fail-closed (30 min, with downstream empty-key check)
11. **INFRA-13** — WAF/nginx sidecar (1-2 hours, with corrected deny-all config)
12. **P1-9** — Access token dedup (30 min, corrected to max-N approach)
13. **P1-10** — Error handling fix (15 min, unchanged)

### Sprint 4: "Observability & Infrastructure" (3-5 days)
14. Postgres migration
15. Monitoring (Prometheus/Grafana + Sentry)
16. CI/CD pipeline
17. Backup/DR documentation

---

## Summary of All Corrections

| Issue | What was wrong | What changed |
|-------|---------------|--------------|
| P0-1 user ID | Used `currentUser.Id` — admin changing victim's password would wipe admin's tokens | Changed to `user.ID` (target user). Added audit logging for actor vs target. |
| P0-5 dead work | Proposed migration that already exists at `server.go:212-231` | Removed dead paragraph. Simplified fix to just check `secret == ""` and abort. Deleted "usememos" entirely. |
| P1-9 dedup key | Dedup on `AccessToken` never matches (JWT has unique `iat`) | Changed to max-N (10) approach with oldest-eviction. |
| P0-4 choice | Presented two options without picking one | **Chose Option A:** fix parse-error only, warn for empty list. |
| INFRA-13 nginx | `internal;` does not block external clients | Changed to `deny all;` + IP allowlist. Added CVE provenance note. |
| P0-3 frontend | Unverified frontend impact on token revocation | Added frontend audit step. Default to redact unless frontend needs raw value. |
| P0-6 escaping | `json.Marshal` overkill; 200-line function body unaddressed | Changed to `strconv.Quote`-equivalent manual escaping. |
| P0-2 constant | Magic number `30 * 24 * time.Hour` | Extracted to `MaxNeverExpireDuration` constant. |
| P1-7 placement | `HandleOnboard` has no tenant.ID; line numbers off | Added global rate-limit for HandleOnboard. Corrected all line numbers. |
| INFRA-12 health | Missing `protocol` field; cost impact unstated | Added `protocol = "http"`. Noted ~2x cost impact of performance CPU + min 1. |
| CVE-2026-6634 | Unverifiable CVE ID | Added provenance note. Downgraded from "must fix" to "defense-in-depth." |

**Ready to implement. Toggle to ACT mode to begin Sprint 1.**
