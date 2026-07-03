# Implementation Plan: Security Remediation (per sec_review_stepfun.md)

This plan maps each finding from `bugs/019/sec_review_stepfun.md` to specific files, changes, verification steps, and testing criteria. Issues are ordered by the tiered priority defined in that document.

---

## P0 — Fix Before Any Customer Goes Live

### P0-1: Invalidate Access Tokens on Password Change

**Finding:** `user_service.go:209-215` updates `PasswordHash` via `store.UpdateUser`, but never deletes or invalidates existing access tokens. An attacker with a stolen JWT retains access after the victim changes their password.

**Files to modify:**
- `server/router/api/v1/user_service.go` (~line 213)
- `server/router/api/v1/auth_service.go` (add new helper)

**Changes:**
1. In `user_service.go` `UpdateUser()` handler, after the `password` field case computes the new hash (~line 215), add:
   ```go
   // When password changes, invalidate ALL existing access tokens for this user.
   // The user is authenticated at this point, so we have currentUser from context.
   if field == "password" {
       // After generating passwordHash...
       // Signal that tokens need invalidation
       s.deleteAllUserAccessTokens(ctx, currentUser.Id)
   }
   ```
2. Add `deleteAllUserAccessTokens(ctx, userID)` method to `APIV1Service` in `auth_service.go`:
   - Fetch current user settings
   - Set `AccessTokens` to empty list
   - `UpsertUserSetting` with empty access tokens
   - Log the invalidation for audit

**Verification:**
- Before fix: Sign in → get cookie → change password → old cookie still works
- After fix: Sign in → get cookie → change password → old cookie returns 401
- Existing tests pass: `go test ./server/router/api/v1/...`

---

### P0-2: Remove or Guard `NeverExpire` (100-Year Token Bypass)

**Finding:** `auth_service.go:159-163` — The client sends `request.NeverExpire = true` and gets a ~100-year token. Neither `plan_priority.md` nor `sec_review_deepseek.md` mentioned this, but it's a critical bypass of the intended token lifetime.

**Files to modify:**
- `server/router/api/v1/auth_service.go` (line 159-163)

**Changes:**
Option A (recommended — simplest): Cap `NeverExpire` at 30 days:
```go
expireTime := time.Now().Add(AccessTokenDuration)
if request.NeverExpire {
    expireTime = time.Now().Add(30 * 24 * time.Hour) // Cap at 30 days, not 100 years
}
```

Option B (stricter): Reject `NeverExpire` entirely in production:
```go
if request.NeverExpire {
    if profile.Mode == "prod" {
        return nil, status.Errorf(codes.InvalidArgument, "never_expire is not allowed in production")
    }
    expireTime = time.Now().Add(30 * 24 * time.Hour)
}
```

**Verification:**
- `curl` with `never_expire: true` returns a token with `exp` ≤ 30 days
- Existing auth tests pass

---

### P0-3: Redact JWT Tokens in `ListUserAccessTokens` Response

**Finding:** `user_service.go:385-386` populates `userAccessToken.AccessToken` with the raw JWT string. Any process or log that records this gRPC response leaks the bearer token.

**Files to modify:**
- `server/router/api/v1/user_service.go` (line 385-386)

**Changes:**
Change from:
```go
userAccessToken := &v1pb.UserAccessToken{
    AccessToken: userAccessToken.AccessToken,  // LEAKS raw JWT
    Description: userAccessToken.Description,
    IssuedAt:    timestamppb.New(claims.IssuedAt.Time),
}
```
To:
```go
userAccessToken := &v1pb.UserAccessToken{
    AccessToken: "",  // REDACTED — never return raw JWT in list response
    Description: userAccessToken.Description,
    IssuedAt:    timestamppb.New(claims.IssuedAt.Time),
}
```

**Verification:**
- `GET /api/v1/users/{id}/access-tokens` returns empty string for `access_token` field
- The `CreateUserAccessToken` response (single creation) still returns the raw token (that's intentional)
- Existing frontend token listing still works (it should use the description + issued-at to identify tokens)

---

### P0-4: Fix `isDomainAllowed` Fail-Open

**Finding:** `handlers.go:1881-1891` — Empty JSON, invalid JSON, or empty array all return `true` (allow all origins). Widget CORS-like protection silently disables.

**Files to modify:**
- `server/router/api/v1/agent/handlers.go` (line 1880-1919)

**Changes:**
```go
func (h *Handler) isDomainAllowed(allowedDomainsJSON, origin, referer string) bool {
    if allowedDomainsJSON == "" {
        slog.Warn("domain allowlist is empty — no restrictions applied. Set AllowedDomains to restrict.")
        return true // Empty = no restrictions (preserve backward compatibility)
    }

    var domains []string
    if err := json.Unmarshal([]byte(allowedDomainsJSON), &domains); err != nil {
        slog.Error("domain allowlist contains invalid JSON, denying access",
            "allowed_domains", allowedDomainsJSON,
            "error", err)
        return false // FAIL CLOSED on parse error
    }
    if len(domains) == 0 {
        slog.Warn("domain allowlist is an empty array — denying all origins. Remove AllowedDomains to allow all.")
        return false // FAIL CLOSED on empty list (breaking change, see note)
    }
    // ... rest of function unchanged
}
```

**NOTE:** Changing empty-array behavior from allow-all to deny-all IS a breaking change. Existing tenants with `"allowed_domains": []` will break. Options:
- Option A (safer): Only fix the parse-error path (`return false`). Keep empty-array as `return true` but add a warning log.
- Option B (strict): Fix both. Update any affected tenants' `AllowedDomains` to `null` or remove the field.

**Verification:**
- Send request with `Origin: https://evil.com` against a tenant with `allowed_domains: "invalid json"` → returns 403
- Send request with `Origin: https://evil.com` against a tenant with `allowed_domains: ""` → returns 200 (preserved backward compat)
- Existing domain allowlist tests pass

---

### P0-5: Eliminate Hardcoded "usememos" JWT Secret Fallback

**Finding:** `server.go:66-69` — In production, `secret = workspaceBasicSetting.SecretKey`. If that DB field is empty (first deploy, migration issue), the app falls back to the well-known string `"usememos"`, allowing anyone to forge valid JWTs.

**Files to modify:**
- `server/server.go` (line 66-70)

**Changes:**
Replace:
```go
secret := "usememos"
if profile.Mode == "prod" {
    secret = workspaceBasicSetting.SecretKey
}
s.Secret = secret
```
With:
```go
if profile.Mode == "prod" {
    if workspaceBasicSetting.SecretKey == "" {
        return nil, errors.New("production mode requires a non-empty SecretKey in workspace basic settings. " +
            "Generate one via: openssl rand -base64 32")
    }
    s.Secret = workspaceBasicSetting.SecretKey
} else {
    // Dev mode: allow "usememos" fallback for local development
    if workspaceBasicSetting.SecretKey != "" {
        s.Secret = workspaceBasicSetting.SecretKey
    } else {
        s.Secret = "usememos"
        slog.Warn("Using default JWT secret 'usememos' in dev mode. Set workspace SecretKey for production.")
    }
}
```

Additionally, add a startup migration/initialization that sets a random `SecretKey` on first boot:
- In `getOrUpsertWorkspaceBasicSetting`, if the setting doesn't exist, generate `SecretKey = base64(rand(32))` before persisting.

**Verification:**
- Start app in prod mode with empty DB → **fails to start** with clear error message
- Start app in prod mode with valid SecretKey → starts normally
- Start app in dev mode → starts with warning, uses "usememos"
- Existing JWT tests pass

---

### P0-6: Escape `companyName` in Widget Fallback

**Finding:** `handlers.go:1661-1657` — The `generateWidgetScript` fallback concatenates `companyName` into a JavaScript string literal with no escaping. A tenant-supplied `CompanyName` containing `'</script><script>steal(document.cookie)</script>'` executes XSS in every visitor's browser.

**Files to modify:**
- `server/router/api/v1/agent/handlers.go` (line 1661-1864)

**Changes:**
Replace the string-concatenation approach in `generateWidgetScript` with proper Go template escaping or at minimum JSON-string escaping:
```go
func generateWidgetScript(baseURL, tenantSlug, companyName string) string {
    // Use json.Marshal for proper JS string escaping
    safeBaseURL, _ := json.Marshal(baseURL)
    safeSlug, _ := json.Marshal(tenantSlug)
    safeCompany, _ := json.Marshal(companyName)

    return fmt.Sprintf(`(function() {
  'use strict';
  var config = {
    baseURL: %s,
    tenantSlug: %s,
    companyName: %s,
    primaryColor: '#0d9488'
  };
  // ... rest of function using %s, %s, %s
`, string(safeBaseURL), string(safeSlug), string(safeCompany),
       string(safeBaseURL), string(safeSlug), string(safeCompany))
}
```

The modern `generateWidgetLoaderScript` (line 1641) already uses `%q` and is safe — only the fallback `generateWidgetScript` is vulnerable.

**Verification:**
- Set `CompanyName` to `'</script><script>alert(1)</script>'`
- Hit `/widget/:slug/embed.js` → response contains escaped version: `\u003c/script\u003e\u003cscript\u003ealert(1)\u003c/script\u003e`
- No script injection possible
- Existing widget tests pass

---

## P1 — Fix Before Scaling Beyond First Customer

### P1-7: Add Rate Limiting to All Admin Mutation Endpoints

**Finding:** `checkAdminMutationRateLimit` is only called on role-template endpoints. It's absent from 9 other mutation endpoints.

**Files to modify:**
- `server/router/api/v1/agent/handlers.go` (multiple handler functions)

**Changes:**
Add this line after the permission check in each of these handlers:
```go
if err := h.checkAdminMutationRateLimit(c, tenant.ID); err != nil {
    return err
}
```

**Affected handlers to modify (alphabetical):**
1. `HandleDeleteTenant` (line ~1595) — after permission check
2. `HandleGrantPermission` (line ~2469) — after permission check
3. `HandleImport` (line ~1332) — after admin check
4. `HandleImportSingleFile` (line ~985) — after permission check
5. `HandleOnboard` (line ~1228) — after admin check
6. `HandleReindexTenant` (line ~1099) — after permission check
7. `HandleRestoreFileVersion` (line ~867) — after permission check
8. `HandleRevokePermission` (line ~2538) — after permission check
9. `HandleSetLLMConfig` (line ~2320) — after permission check

**Verification:**
- Send 31 rapid requests to `HandleGrantPermission` → 30th+ returns 429
- Rate limit key is correctly scoped: `(tenantID, "admin_mutation", clientIP)`
- Admin mutation RPM respects `TenantConfig.AdminMutationRateLimitRPM`
- Existing rate limit tests pass

---

### P1-8: Fail-Closed on Tenant API Key Decryption Error

**Finding:** `service.go:1205-1226` — If `Decrypt` fails for a tenant-specific key, it logs a warning and silently falls back to the global `s.profile.OpenRouterAPIKey`. This breaks tenant isolation for LLM billing and allows a tenant with a broken key to use the global key without knowing it.

**Files to modify:**
- `server/router/api/v1/agent/service.go` (lines 1205-1216)

**Changes:**
```go
if len(config.OpenRouterAPIKeyEncrypted) > 0 && s.encryptionService != nil {
    decrypted, err := s.encryptionService.Decrypt(...)
    if err == nil && decrypted != "" {
        apiKey = decrypted
    } else {
        slog.Error("Failed to decrypt tenant OpenRouter API key — tenant will get NO API key, not global fallback",
            "tenantID", tenantID, "error", err)
        // Intentionally do NOT fall back to global key.
        // The tenant key is broken; returning "" will cause LLM calls to fail
        // with a clear error, which is better than silently billing the wrong account.
    }
}
```

**Verification:**
- Tenant with valid encrypted key → uses tenant key
- Tenant with corrupted encrypted key → LLM calls fail with auth error (no silent global fallback)
- Tenant with no encrypted key → uses global key (preserved)
- Existing LLM config tests pass

---

### P1-9: Deduplicate Access Tokens on Upsert

**Finding:** `user_service.go:506-528` — `append()` adds a new token every time without checking for existing duplicates. Repeated logins create unbounded token rows.

**Files to modify:**
- `server/router/api/v1/user_service.go` (line 506-528)

**Changes:**
```go
func (s *APIV1Service) UpsertAccessTokenToStore(ctx context.Context, user *store.User, accessToken, description string) error {
    userAccessTokens, err := s.Store.GetUserAccessTokens(ctx, user.ID)
    if err != nil {
        return errors.Wrap(err, "failed to get user access tokens")
    }

    // Check for existing token to avoid duplicates
    existingIndex := -1
    for i, t := range userAccessTokens {
        if t.AccessToken == accessToken {
            existingIndex = i
            break
        }
    }

    if existingIndex >= 0 {
        // Update existing entry
        userAccessTokens[existingIndex].Description = description
        slog.Debug("Updated existing access token entry", "userID", user.ID)
    } else {
        // Append new entry
        userAccessToken := storepb.AccessTokensUserSetting_AccessToken{
            AccessToken: accessToken,
            Description: description,
        }
        userAccessTokens = append(userAccessTokens, &userAccessToken)
    }

    // ... rest of upsert unchanged
}
```

**Verification:**
- Sign in 10 times → only 1 token row in DB per session token
- Existing sign-in tests pass

---

### P1-10: Handle `ExtractWorkspaceSettingKeyFromName` Error

**Finding:** `workspace_setting_service.go:103` — Error from `ExtractWorkspaceSettingKeyFromName` is discarded with `_, _`. A malformed name silently maps to `GeneralSetting`, potentially writing data under the wrong key.

**Files to modify:**
- `server/router/api/v1/workspace_setting_service.go` (line 103)

**Changes:**
```go
settingKey, err := ExtractWorkspaceSettingKeyFromName(setting.Name)
if err != nil {
    return nil, status.Errorf(codes.InvalidArgument, "invalid workspace setting name: %v", err)
}
```

**Verification:**
- Sending a workspace setting with a malformed name returns `400 InvalidArgument`
- Existing workspace setting tests pass

---

## Infrastructure Fixes

### INFRA-11: Rotate Secrets in `.env` and Migrate to `fly secrets`

**Finding:** `.env` contains live `OPENROUTER_API_KEY` and `ENCRYPTION_MASTER_KEY`. While `.gitignore`d, they're visible on the filesystem.

**Actions:**
1. Rotate both keys immediately (they are compromised by being on disk)
   ```bash
   # Generate new ENCRYPTION_MASTER_KEY
   uuidgen
   # Get new OpenRouter API key from console.openrouter.ai
   ```
2. Set on Fly:
   ```bash
   fly secrets set OPENROUTER_API_KEY=sk-or-v1-<NEW>
   fly secrets set ENCRYPTION_MASTER_KEY=<NEW>
   ```
3. Remove from `.env` after rotation

---

### INFRA-12: `fly.toml` Fixes

**Changes to `fly.toml`:**
```toml
# Change 1: Performance CPU
[[vm]]
  memory = '1024mb'
  cpu_kind = 'performance'    # WAS: 'shared'
  cpus = 1

# Change 2: Minimum instances
[http_service]
  internal_port = 5230
  force_https = true
  auto_stop_machines = 'stop'
  auto_start_machines = true
  min_machines_running = 1    # WAS: 0

# Change 3: Health checks
[[checks]]
  interval = "10s"
  timeout = "2s"
  grace_period = "30s"
  method = "GET"
  path = "/healthz"
  [checks.headers]
```

---

### INFRA-13: WAF/Nginx Sidecar for CVE-2026-6634

Create `deployment/nginx-sidecar.conf`:
```nginx
# Block CVE-2026-6634 auth bypass vector: instance settings endpoint
location ~* ^/api/v1/(workspace|setting|instance) {
    # Only allow HOST role to modify settings
    internal;  # Or implement token-role validation
}
```

Or add Fly edge rules via `fly.toml`:
```toml
[http_service.headers]
  allowed_origin = "*"
  # Block admin routes from external access
  # Note: Fly networking doesn't support path-based ACL — use nginx sidecar
```

---

## Testing Requirements

### Per-Fix Tests
| Fix | Test | Pass Criteria |
|-----|------|---------------|
| P0-1 | Password change invalidates old token | Old cookie returns 401 |
| P0-2 | `NeverExpire` capped at 30 days | Token `exp` claim ≤ 30d |
| P0-3 | List tokens returns redacted | `access_token` field is `""` |
| P0-4 | Invalid JSON → deny | Returns 403, logs error |
| P0-5 | Empty SecretKey → startup failure | `server.Start()` returns error |
| P0-6 | XSS in companyName → escaped | Response doesn't execute `<script>` |
| P1-7 | 31st admin mutation in 1 min → 429 | Returns `429 Too Many Requests` |
| P1-8 | Corrupted tenant key → no global fallback | LLM calls return auth error |
| P1-9 | Duplicate login → no duplicate rows | Only 1 token per session in DB |
| P1-10 | Malformed setting name → 400 | Returns `400 InvalidArgument` |
| INFRA-12 | Deploy to Fly with new config | App health check passes |

### Integration Test
```bash
# Full regression
go test ./server/router/api/v1/... -count=1 -race
# Agent-specific tests
go test ./server/router/api/v1/agent/... -count=1 -race
```

---

## Execution Order (Recommended Sprint Plan)

### Sprint 1: "Stop the Bleeding" (2-3 days)
1. INFRA-12: `fly.toml` fixes (5 min)
2. P0-5: Hardcoded secret elimination (30 min)
3. P0-2: `NeverExpire` cap (10 min)
4. P0-3: Token redaction (10 min)
5. INFRA-11: Rotate secrets (15 min)

### Sprint 2: "Prevent Exploitation" (2-3 days)
6. P0-1: Session revocation on password change (1-2 hours)
7. P0-6: Widget XSS fix (30 min)
8. P0-4: `isDomainAllowed` fail-closed (30 min)
9. P1-7: Rate limiting on all admin endpoints (1 hour)

### Sprint 3: "Harden Isolation" (2-3 days)
10. P1-8: Tenant API key fail-closed (30 min)
11. INFRA-13: WAF/nginx sidecar (1-2 hours)
12. P1-9: Access token deduplication (30 min)
13. P1-10: Error handling fix (15 min)

### Sprint 4: "Observability & Infrastructure" (3-5 days)
14. Postgres migration (remove SQLite)
15. Monitoring (Prometheus/Grafana + Sentry)
16. CI/CD pipeline (GitHub Actions)
17. Backup/DR documentation

---

## Summary

| Tier | Count | Effort | Risk Reduction |
|------|-------|--------|----------------|
| **P0** | 6 fixes | ~1 day | Eliminates active XSS, JWT forgery, auth bypass vulnerabilities |
| **P1** | 4 fixes | ~1 day | Plugs rate-limiting gaps, isolation leaks, data corruption risks |
| **Infra** | 3 fixes | ~1 day | Eliminates infrastructure single points of failure |

**Total engineering time: ~3 days for all 13 P0+P1+Infra fixes.**

Ready to begin implementation — toggle to ACT mode to start coding the fixes.
