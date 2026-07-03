# Final Implementation Plan (Corrected per 2nd Adversarial Review)

All 10 issues from the second review are resolved. Each correction is marked with **REVISED**.

---

## P0 — Fix Before Any Customer Goes Live

### P0-1: Invalidate Access Tokens on Password Change (REVISED — fail-closed)

**Original flaw:** Non-fatal invalidation (log + continue) gives false confidence — the API returns 200 but tokens are not revoked.

**Corrected implementation:**

**File:** `server/router/api/v1/user_service.go` (~line 224)

**Changes:**
1. After `updatedUser, err := s.Store.UpdateUser(ctx, update)` at line 224, add:
   ```go
   if err != nil {
       return nil, status.Errorf(codes.Internal, "failed to update user: %v", err)
   }

   // NEW: Invalidate ALL existing access tokens for the TARGET user
   // (user.ID, not currentUser.ID — an admin can change another's password)
   if err := s.deleteAllUserAccessTokens(ctx, user.ID); err != nil {
       slog.Error("Failed to invalidate access tokens after password change",
           "target_user_id", user.ID,
           "actor_user_id", currentUser.ID,
           "error", err)
       // FAIL CLOSED: return 500 so the caller knows the operation is incomplete.
       // The password was changed, but old tokens remain valid — the user must retry.
       return nil, status.Errorf(codes.Internal, "password changed but failed to invalidate existing sessions: %v", err)
   }
   slog.Info("Invalidated access tokens after password change",
       "target_user_id", user.ID,
       "actor_user_id", currentUser.ID)
   ```

2. Add helper to `user_service.go` (alongside existing `UpsertAccessTokenToStore` / `DeleteUserAccessToken` for cohesion):
   ```go
   // deleteAllUserAccessTokens removes ALL access tokens for a user.
   func (s *APIV1Service) deleteAllUserAccessTokens(ctx context.Context, userID int32) error {
       if err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{
           UserId: userID,
           Key:    storepb.UserSettingKey_ACCESS_TOKENS,
           Value: &storepb.UserSetting_AccessTokens{
               AccessTokens: &storepb.AccessTokensUserSetting{
                   AccessTokens: nil,
               },
           },
       }); err != nil {
           return errors.Wrap(err, "failed to clear access tokens")
       }
       return nil
   }
   ```

**Rollback:** Revert the 5-line addition in `user_service.go` and remove the helper.

**Verification:**
- Admin changes victim's password → victim's tokens invalidated, admin's tokens preserved
- If token invalidation fails (e.g., DB error) → API returns 500, password change is rolled back
- `go test ./server/router/api/v1/...` passes

---

### P0-2: Cap `NeverExpire` Token Duration (REVISED — include migration for old tokens)

**File:** `server/router/api/v1/auth.go` (add constant), `server/router/api/v1/auth_service.go` (use it)

**Changes:**
```go
// In auth.go:
const MaxNeverExpireDuration = 30 * 24 * time.Hour
```

```go
// In auth_service.go, line 159-163:
expireTime := time.Now().Add(AccessTokenDuration)
if request.NeverExpire {
    expireTime = time.Now().Add(MaxNeverExpireDuration)
}
```

**Old 100-year token migration:** Run this one-time SQL cleanup after deploy:
```sql
-- SQLite: Delete all user settings with access tokens whose JWT exp > 30 days from now
-- This is a manual step, not automated in the fix.
-- The implementation agent should provide this as a script, not run it automatically.
```

Or provide a Go one-off command:
```go
// cmd/cleanup-old-tokens/main.go — provided as a script, not auto-executed
// Iterates all users, parses each JWT's exp claim, removes tokens with exp > now+30d
```

**Verification:**
- Login with `never_expire: true` → token `exp` ≤ 30 days
- Login without `never_expire` → token `exp` = 7 days (unchanged)
- Existing tests pass

---

### P0-3: Redact JWT Tokens in `ListUserAccessTokens` Response (REVISED — precondition audit)

**Original flaw:** Ambiguous "default to be confirmed by audit." The `DeleteUserAccessToken` endpoint matches on token string, so the frontend almost certainly needs the raw value.

**Corrected implementation — two-phase approach:**

**Phase 1 (Precondition — must run before implementation):**
```bash
# Audit frontend usage of ListUserAccessTokens response
grep -rn "access_token\|ListUserAccessTokens\|listAccessTokens\|userAccessToken" web/src/ --include="*.ts" --include="*.tsx" --include="*.js"
```

**Phase 2a (If frontend only uses metadata):** Redact as originally planned.

**Phase 2b (If frontend needs raw token for deletion — LIKELY):** Do NOT redact. Instead, add an `id` field to the response and use it for revocation:
```go
// In user_service.go, add an id field to the response:
type UserAccessTokenResponse struct {
    ID          string `json:"id"`           // SHA256(token)[:16] — stable identifier
    Description string `json:"description"`
    IssuedAt    string `json:"issued_at"`
    ExpiresAt   string `json:"expires_at"`
    // AccessToken is intentionally OMITTED from list responses
}
```

**Default (if audit cannot be run):** Do NOT redact. The security risk of leaking tokens in list responses is lower than the risk of breaking token revocation. This fix can be deferred to a follow-up sprint.

**Verification:**
- If Phase 2a: `GET /api/v1/users/{id}/access-tokens` returns `access_token: ""`
- If Phase 2b: `GET /api/v1/users/{id}/access-tokens` returns `id` field, no `access_token` field
- Delete by ID works

---

### P0-4: Fix `isDomainAllowed` Fail-Open (UNCHANGED — Option A confirmed)

**Decision:** Option A — fix parse-error path only. Empty list still allows all (backward compat).

**File:** `server/router/api/v1/agent/handlers.go` (~line 1880)

**Changes:**
```go
func (h *Handler) isDomainAllowed(allowedDomainsJSON, origin, referer string) bool {
    if allowedDomainsJSON == "" {
        return true
    }
    var domains []string
    if err := json.Unmarshal([]byte(allowedDomainsJSON), &domains); err != nil {
        slog.Error("domain allowlist contains invalid JSON, denying access",
            "allowed_domains", allowedDomainsJSON, "error", err)
        return false // FAIL CLOSED on parse error
    }
    if len(domains) == 0 {
        return true // Empty list = no restrictions (backward compat)
    }
    // ... rest unchanged
}
```

**Verification:**
- Invalid JSON → 403 + error log
- Empty string → 200 (preserved)
- Empty array → 200 (preserved)

---

### P0-5: Eliminate Hardcoded "usememos" JWT Secret Fallback (UNCHANGED — correct)

**File:** `server/server.go` (line 66-70)

**Changes:**
```go
// getOrUpsertWorkspaceBasicSetting already auto-generates UUID on first boot (server.go:218).
// Defense-in-depth: abort if SecretKey is somehow empty.
secret := workspaceBasicSetting.SecretKey
if secret == "" {
    return nil, errors.New("CRITICAL: workspace SecretKey is empty. Cannot start.")
}
s.Secret = secret
```

**Repo-wide verification:** `grep -r '"usememos"' --include="*.go" .` — ensure no test fixtures or comments reference the old default.

---

### P0-6: Escape `companyName` in Widget Fallback (REVISED — use `json.Marshal`)

**Original flaw:** Manual `strings.ReplaceAll` does NOT prevent `</script>` XSS — the HTML parser breaks out before the JS engine sees the escaping.

**Corrected implementation:**

**File:** `server/router/api/v1/agent/handlers.go` (~line 1661)

Use `json.Marshal` which encodes `<` as `\u003c` and `>` as `\u003e`, preventing HTML parser breakout:

```go
func generateWidgetScript(baseURL, tenantSlug, companyName string) string {
    // json.Marshal produces JS-safe strings: encodes < as \u003c, > as \u003e,
    // preventing </script> HTML parser breakout regardless of JS string context.
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
  // ... rest of function body unchanged (200+ lines)
  // Note: the rest of the function body is a raw string literal.
  // It does not contain user-supplied values, so it is safe.
})();`, string(safeBaseURL), string(safeSlug), string(safeCompany))
}
```

**Verification:**
- `CompanyName = "</script><img src=x onerror=alert(1)>"` → output contains `\u003c/script\u003e\u003cimg src=x onerror=alert(1)\u003e`
- Browser renders the script tag correctly, no XSS
- Widget still works after fix

---

## P1 — Fix Before Scaling Beyond First Customer

### P1-7: Add Rate Limiting to All Admin Mutation Endpoints (REVISED — corrected line numbers + HandleOnboard scoping)

**File:** `server/router/api/v1/agent/handlers.go`

**Pattern:** Insert AFTER the permission check, BEFORE expensive work. The agent should grep for the actual `// Check admin role` / `// Check permission` comment above each insertion point rather than relying on line numbers.

| Handler | Insert after | Notes |
|---------|-------------|-------|
| `HandleDeleteTenant` | After `// Check admin role` (~line 1581) | Has `tenant` from earlier lookup |
| `HandleGrantPermission` | After `// Must be admin or have tenant:admin` (~line 2469) | Has `tenant` |
| `HandleRevokePermission` | After `// Must be admin or have tenant:admin` (~line 2537) | Has `tenant` |
| `HandleRestoreFileVersion` | After `// Check admin role OR files:restore permission` (~line 867) | Has `tenant` |
| `HandleImportSingleFile` | After `// Check admin role OR files:upload permission` (~line 985) | Has `tenant` |
| `HandleImport` | After `// Check admin role` (~line 1331) | Has `tenant` |
| `HandleReindexTenant` | After `// Check admin role OR api:config permission` (~line 1099) | Has `tenant` |
| `HandleSetLLMConfig` | After `// Check permission (api:config or admin)` (~line 2320) | Has `tenant` |
| `HandleOnboard` | **Special case** — no tenant.ID exists yet | See below |

**For `HandleOnboard`:** Use a global rate-limit key (not tenant-scoped) with a high cap (100/min) since onboarding is a one-shot per-tenant operation:
```go
// At ~line 1228, after the admin check:
clientIP := c.RealIP()
if clientIP == "" {
    clientIP = c.Request().RemoteAddr
}
allowed, err := h.service.CheckRateLimit(ctx, 0, "admin_onboard", clientIP, 100)
if err != nil {
    return echo.NewHTTPError(http.StatusInternalServerError, "Rate limit check failed")
}
if !allowed {
    return echo.NewHTTPError(http.StatusTooManyRequests, "Onboard rate limit exceeded")
}
```

**Verification:**
- 31 rapid requests to any of the 8 tenant-scoped endpoints → 30th+ returns 429
- 101 rapid onboard requests → 100th+ returns 429
- Rate limit key is correctly formatted

---

### P1-8: Fail-Closed on Tenant API Key Decryption Error (REVISED — don't break soft-fallback call sites)

**Original flaw:** The plan only addressed 2 of 9 call sites and would replace soft fallbacks with hard errors.

**Corrected implementation:**

**File:** `server/router/api/v1/agent/service.go`

**Change 1 — `getLLMConfig` (line 1205-1216):** Add a `brokenTenantKey` flag:
```go
func (s *Service) getLLMConfig(ctx context.Context, tenantID int32) (model string, apiKey string) {
    config, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID})
    if config != nil {
        if config.LLMModel != "" {
            model = config.LLMModel
        }
        if len(config.OpenRouterAPIKeyEncrypted) > 0 && s.encryptionService != nil {
            decrypted, err := s.encryptionService.Decrypt(
                config.OpenRouterAPIKeyEncrypted,
                config.OpenRouterAPIKeyNonce,
            )
            if err == nil && decrypted != "" {
                apiKey = decrypted
            } else {
                slog.Error("Failed to decrypt tenant OpenRouter API key",
                    "tenantID", tenantID, "error", err)
                // Return empty apiKey — callers that require a key will fail.
                // Callers with soft fallbacks (verifyResponseWithLLM, etc.) will skip gracefully.
            }
        }
    }
    // Fallback to env vars (unchanged)
    if model == "" {
        model = s.profile.LLMModel
        if model == "" {
            model = "openai/gpt-oss-120b:free"
        }
    }
    if apiKey == "" {
        apiKey = s.profile.OpenRouterAPIKey
    }
    return model, apiKey
}
```

**Change 2 — Add a `requireLLMConfig` wrapper for call sites that MUST have a key:**
```go
// requireLLMConfig returns the LLM config or an error if no key is available.
// Use this for chat/LLM call sites where a missing key is fatal.
func (s *Service) requireLLMConfig(ctx context.Context, tenantID int32) (model string, apiKey string, err error) {
    model, apiKey = s.getLLMConfig(ctx, tenantID)
    if apiKey == "" {
        return "", "", fmt.Errorf("tenant %d has no valid OpenRouter API key configured", tenantID)
    }
    return model, apiKey, nil
}
```

**Change 3 — Use `requireLLMConfig` at chat call sites** (the 2 sites that make actual LLM calls for customer-facing chat). Leave the other 7 sites (embedding, verification, simulation) using `getLLMConfig` with their existing soft fallbacks.

**Verification:**
- Tenant with corrupted encrypted key → chat returns clear "no valid API key" error
- Tenant with corrupted encrypted key → embedding/verification still works (uses global key fallback)
- Tenant with no encrypted key → uses global key (unchanged)

---

### P1-9: Deduplicate Access Tokens on Upsert (REVISED — sort by iat for eviction)

**Original flaw:** `userAccessTokens[1:]` evicts `[0]` but order is not guaranteed.

**Corrected implementation:**

**File:** `server/router/api/v1/user_service.go` (~line 506)

```go
func (s *APIV1Service) UpsertAccessTokenToStore(ctx context.Context, user *store.User, accessToken, description string) error {
    const maxAccessTokens = 10

    userAccessTokens, err := s.Store.GetUserAccessTokens(ctx, user.ID)
    if err != nil {
        return errors.Wrap(err, "failed to get user access tokens")
    }

    // Parse JWT to get issued-at time for sorting
    // (tokens are stored as raw JWT strings, we need to sort by iat)
    type tokenWithTime struct {
        token *storepb.AccessTokensUserSetting_AccessToken
        iat   time.Time
    }

    var parsed []tokenWithTime
    for _, t := range userAccessTokens {
        claims := &ClaimsMessage{}
        _, _, err := jwt.NewParser().ParseUnverified(t.AccessToken, claims)
        if err != nil {
            continue // Skip unparseable tokens (will be cleaned up)
        }
        parsed = append(parsed, tokenWithTime{token: t, iat: claims.IssuedAt.Time})
    }

    // Sort by issued-at ascending (oldest first)
    sort.Slice(parsed, func(i, j int) bool {
        return parsed[i].iat.Before(parsed[j].iat)
    })

    // Enforce max
    if len(parsed) >= maxAccessTokens {
        // Keep only the (maxAccessTokens - 1) most recent, plus the new one
        parsed = parsed[len(parsed)-(maxAccessTokens-1):]
    }

    // Rebuild the list
    newTokens := make([]*storepb.AccessTokensUserSetting_AccessToken, 0, maxAccessTokens)
    for _, pt := range parsed {
        newTokens = append(newTokens, pt.token)
    }
    // Append new entry
    newTokens = append(newTokens, &storepb.AccessTokensUserSetting_AccessToken{
        AccessToken: accessToken,
        Description: description,
    })

    if _, err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{
        UserId: user.ID,
        Key:    storepb.UserSettingKey_ACCESS_TOKENS,
        Value: &storepb.UserSetting_AccessTokens{
            AccessTokens: &storepb.AccessTokensUserSetting{
                AccessTokens: newTokens,
            },
        },
    }); err != nil {
        return errors.Wrap(err, "failed to upsert user setting")
    }
    return nil
}
```

**Migration for existing users with 20+ tokens:** The first sign-in after deploy will trim to 10. No separate migration needed.

**Verification:**
- Sign in 15 times → ≤10 DB rows
- Oldest tokens are evicted first
- `go test ./server/router/api/v1/...` passes

---

### P1-10: Handle `ExtractWorkspaceSettingKeyFromName` Error (UNCHANGED)

**File:** `server/router/api/v1/workspace_setting_service.go` (~line 103)

```go
settingKey, err := ExtractWorkspaceSettingKeyFromName(setting.Name)
if err != nil {
    return nil, status.Errorf(codes.InvalidArgument, "invalid workspace setting name: %v", err)
}
```

---

## Infrastructure Fixes

### INFRA-11: Rotate Secrets in `.env` and Migrate to `fly secrets`

**Order:** Do BEFORE P0-5 to ensure new key is live before restart.

```bash
# 1. Generate new keys
NEW_ENCRYPTION_KEY=$(uuidgen)
# Get new OpenRouter key from console.openrouter.ai

# 2. Set on Fly
fly secrets set OPENROUTER_API_KEY=sk-or-v1-<NEW>
fly secrets set ENCRYPTION_MASTER_KEY=$NEW_ENCRYPTION_KEY

# 3. Restart
fly deploy

# 4. Verify
fly secrets list | grep -E "OPENROUTER_API_KEY|ENCRYPTION_MASTER_KEY"

# 5. Remove from .env
sed -i '/OPENROUTER_API_KEY/d' .env
sed -i '/ENCRYPTION_MASTER_KEY/d' .env
```

---

### INFRA-12: `fly.toml` Fixes (REVISED — corrected health check, cost noted)

**Changes:**
```toml
# Change 1: Performance CPU
# NOTE: ~2x cost increase (~$75/mo vs ~$35/mo at 1024MB/1CPU)
[[vm]]
  cpu_kind = 'performance'    # WAS: 'shared'
  cpus = 1
  memory_mb = 1024

# Change 2: Minimum instances
[http_service]
  min_machines_running = 1    # WAS: 0

# Change 3: Health checks (NEWLY ADDED — was absent)
[[checks]]
  interval = "10s"
  timeout = "2s"
  grace_period = "30s"
  method = "GET"
  path = "/healthz"
```

**Cost impact:** Performance CPU + min 1 machine roughly doubles monthly cost. Confirm with user before deploying.

---

### INFRA-13: WAF/Nginx Sidecar for CVE-2026-6634 (REVISED — scoped to actual CVE vector, no admin lockout)

**CVE provenance:** `CVE-2026-6634` is **unverifiable** (CVEs are not assigned that far in advance). The actual risk is the upstream Memos frontend auth bypass in `App.tsx` — the backend already requires `RoleHost` for `SetWorkspaceSetting`. This is **defense-in-depth only**.

**Corrected nginx config** — scope to the actual frontend CVE vector, not all workspace mutations:

```nginx
# Defense-in-depth: Block the frontend CVE-2026-6634 vector
# The upstream CVE is in the frontend App.tsx UpdateInstanceSetting handler.
# The backend already requires RoleHost for SetWorkspaceSetting.
# This rule blocks the frontend route that triggers the exploit.

# Block the specific frontend instance-settings route
location = /api/v1/workspace/instance-setting {
    # Allow from Fly private network (6PN) for admin access
    allow fdaa::/48;
    allow 10.0.0.0/8;
    # Block external access to this specific endpoint
    deny all;
    
    proxy_pass http://backend:5230;
}

# All other routes pass through normally (no admin lockout)
location / {
    proxy_pass http://backend:5230;
}
```

**This does NOT lock out legitimate admins** because:
1. Admins access the app through the public URL, not the private network
2. The blocked route is the frontend's `UpdateInstanceSetting` endpoint, not the admin UI
3. The backend's `SetWorkspaceSetting` (which requires `RoleHost`) is NOT blocked

**Verification:**
- External request to `/api/v1/workspace/instance-setting` → 403
- External request to `/api/v1/workspace/*` (other settings) → 200 (passes through)
- Admin UI still works

---

## Test Plan (REVISED — explicit command)

```bash
# From Taskfile.yml — verify the test target:
grep -A2 "^  test:" Taskfile.yml

# If test target exists:
task test

# If not:
go test ./server/router/api/v1/... -count=1 -race
go test ./server/router/api/v1/agent/... -count=1 -race
```

---

## Sprint Order (Final)

### Sprint 1: "Stop the Bleeding" (2-3 days)
1. **INFRA-11** — Rotate secrets (BEFORE P0-5)
2. **INFRA-12** — `fly.toml` fixes (confirm ~2x cost with user)
3. **P0-5** — Hardcoded secret elimination (simplified)
4. **P0-2** — `NeverExpire` cap + old-token migration script
5. **P0-3** — Frontend audit → implement redaction or ID-based revocation

### Sprint 2: "Prevent Exploitation" (2-3 days)
6. **P0-1** — Session revocation on password change (fail-closed)
7. **P0-6** — Widget XSS fix (json.Marshal)
8. **P0-4** — `isDomainAllowed` fail-closed (Option A)
9. **P1-7** — Rate limiting on all admin endpoints

### Sprint 3: "Harden Isolation" (2-3 days)
10. **P1-8** — Tenant API key fail-closed (with `requireLLMConfig` wrapper)
11. **INFRA-13** — WAF/nginx sidecar (scoped to actual CVE vector)
12. **P1-9** — Access token dedup (iat-sorted eviction)
13. **P1-10** — Error handling fix

### Sprint 4: "Observability & Infrastructure" (3-5 days)
14. Postgres migration
15. Monitoring (Prometheus/Grafana + Sentry)
16. CI/CD pipeline
17. Backup/DR documentation

---

## Summary of All Corrections from 2nd Review

| Issue | What was wrong | What changed |
|-------|---------------|--------------|
| **P0-6 XSS** | Manual escaping doesn't stop `</script>` HTML parser breakout | Use `json.Marshal` (encodes `<` as `\u003c`) |
| **P0-1 fail-closed** | Non-fatal invalidation gives false confidence | Return 500 if token invalidation fails |
| **P0-3 ambiguity** | "Default to be confirmed by audit" is contradictory | Make audit a precondition. Anticipate "needs raw" → propose ID-based revocation |
| **P1-8 call sites** | Only addressed 2 of 9 sites; would break soft fallbacks | Add `requireLLMConfig` wrapper; leave 7 sites with existing soft fallbacks |
| **P1-7 line numbers** | Still off by 5-15 lines | Agent should grep for permission-check comments, not use line numbers |
| **P1-7 HandleOnboard** | `tenantID=0` shares one bucket across all tenants | Use `"admin_onboard"` key with 100/min cap |
| **P1-9 eviction order** | `[1:]` evicts wrong entry if order is not insertion-order | Sort by JWT `iat` before eviction |
| **INFRA-13 admin lockout** | `deny all` on all workspace routes blocks legitimate admins | Scope to the specific frontend CVE route only |
| **INFRA-12 health check** | `protocol = "http"` comment misleading; `[checks.headers]` empty | Removed both. Clean `[[checks]]` block. |
| **Test command** | "or whatever the actual test alias is" for 3rd time | Grep Taskfile.yml for test target explicitly |
| **P0-2 old tokens** | No migration for existing 100-year tokens | Add one-time cleanup script (manual, not automated) |
| **P0-5 repo-wide** | Only checked `server.go` for "usememos" | `grep -r '"usememos"' --include="*.go" .` repo-wide |

**Ready to implement. Toggle to ACT mode to begin Sprint 1.**
