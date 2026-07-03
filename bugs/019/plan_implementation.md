# Implementation Plan: Security Remediation (per plan3_review.md)

**Date:** 2026-07-04
**Status:** Final — ready for coding agent

This plan incorporates all 3 critical findings from `plan3_review.md` and addresses every remaining open issue. INFRA-13 is excluded per user directive ("not using nginx").

---

## Critical Fixes from plan3_review.md

### 🔴 Critical 1: P0-1 — Password-change rollback claim is false (RESOLVED)

**Problem:** plan3.md claims "If token invalidation fails → API returns 500, password change is rolled back." This is false — `Store.UpdateUser` commits the password change first, and the 500 is returned *after* the password is already persisted. The user is in a dangerous state: new password active + old tokens still valid.

**Decision: Option (a) + (c) hybrid**
- **Wrap in retry:** The `deleteAllUserAccessTokens` call will retry once on failure before returning 500.
- **Honest documentation:** Update the verification step to state: "If token cleanup fails after retry, the password IS changed but old tokens remain. Admin must manually purge tokens via SQL. The API returns 500 to indicate the incomplete state."
- **Graceful recovery:** Use `defer` to attempt token cleanup even after returning 500, so transient DB errors self-heal on retry.

**Files:**
- `server/router/api/v1/user_service.go` (~line 224)

**Changes:**
```go
// After line 224 (updatedUser, err := s.Store.UpdateUser(ctx, update))
if err != nil {
    return nil, status.Errorf(codes.Internal, "failed to update user: %v", err)
}

// NEW: Invalidate ALL existing access tokens for the TARGET user.
// The password IS already committed. If cleanup fails, the user has
// a new password AND old tokens remain valid — admin must manually purge.
// We retry once, then defer one more attempt for transient-fault recovery.
if err := s.deleteAllUserAccessTokens(ctx, user.ID); err != nil {
    // Retry once for transient DB errors
    slog.Warn("First attempt to invalidate tokens failed, retrying...",
        "target_user_id", user.ID, "error", err)
    if retryErr := s.deleteAllUserAccessTokens(ctx, user.ID); retryErr != nil {
        slog.Error("CRITICAL: Password changed but failed to invalidate access tokens after retry",
            "target_user_id", user.ID,
            "actor_user_id", currentUser.ID,
            "error", retryErr)
        // Defer one more attempt before returning 500 (transient recovery)
        go func(uid int32) {
            if deferredErr := s.deleteAllUserAccessTokens(ctx, uid); deferredErr != nil {
                slog.Error("Deferred token cleanup also failed; manual intervention required",
                    "target_user_id", uid, "error", deferredErr)
            }
        }(user.ID)
        return nil, status.Errorf(codes.Internal,
            "password changed but failed to invalidate existing sessions. "+
                "Admin must manually purge tokens via SQL. Error: %v", err)
    }
}
slog.Info("Invalidated access tokens after password change",
    "target_user_id", user.ID, "actor_user_id", currentUser.ID)
```

**Add helper** to `user_service.go` (alongside existing `UpsertAccessTokenToStore` / `DeleteUserAccessToken`):
```go
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

**Verification:**
- Admin changes victim's password → victim's tokens invalidated, admin's tokens preserved ✓
- If token cleanup fails repeatedly → API returns 500, password IS already changed, error message says "manual intervention required" ✓
- Transient DB error → retry succeeds → password changed AND tokens cleaned up ✓
- `go test ./server/router/api/v1/...` passes ✓

---

### 🔴 Critical 2: P0-2 — Old-token migration is a comment, not code (RESOLVED)

**Problem:** plan3.md has a `-- This is a manual step, not automated` SQL comment. No actual migration code exists.

**Decision:** Replace SQL comment with a real Go one-off migration script at `scripts/migrate-old-tokens/main.go`.

**Files:**
- `scripts/migrate-old-tokens/main.go` (new file)
- `server/router/api/v1/auth.go` — `MaxNeverExpireDuration` already exists (confirmed at line 22)
- `server/router/api/v1/auth_service.go` — already uses `MaxNeverExpireDuration` (confirmed at line 163)

**New migration script** `scripts/migrate-old-tokens/main.go`:
```go
package main

import (
    "database/sql"
    "flag"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
    _ "github.com/mattn/go-sqlite3"
)

// RemoveOldTokens deletes any stored access tokens whose JWT `exp` claim
// exceeds MaxNeverExpireDuration (30 days) from now.
// Run ONCE after deploying the P0-2 code fix.
func main() {
    dbPath := flag.String("db", "", "Path to the SQLite database file")
    flag.Parse()
    if *dbPath == "" {
        fmt.Fprintln(os.Stderr, "Usage: go run scripts/migrate-old-tokens/main.go -db=/path/to/memos_prod.db")
        os.Exit(1)
    }

    db, err := sql.Open("sqlite3", *dbPath)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()

    // Read all rows from the user_setting table where key=ACCESS_TOKENS
    rows, err := db.Query(`SELECT id, user_id, value FROM user_setting WHERE key = ?`, "ACCESS_TOKENS")
    ...
```

**Note:** P0-2 constant and usage are already deployed. Only the migration script is missing. The migration script parses each stored JWT's `exp` claim and removes tokens with `exp > now + 30d`.

**Verification:**
- Run `go run scripts/migrate-old-tokens/main.go -db=/path/to/memos_prod.db` → removes old 100-year tokens ✓
- Existing tokens with `exp ≤ 30d` are preserved ✓
- Login creates new tokens capped at `MaxNeverExpireDuration` ✓

---

### 🔴 Critical 3: INFRA-13 — Dropped per user directive

**User confirmed:** "I am not using nginx." INFRA-13 (WAF/nginx sidecar) is removed from the sprint plan entirely. No code or config changes needed.

---

## Remaining Issues from plan3_review.md

### P0-3 — Token redaction conflicts with frontend usage

**Problem:** The frontend (`AccessTokenSection.tsx`) uses the raw token value for:
1. Display (formatted as `first4****last4` via `getFormatedAccessToken`)
2. Copy-to-clipboard (`copyAccessToken`)
3. **Delete** (`handleDeleteAccessToken` passes raw token to `deleteUserAccessToken`)

If we redact `AccessToken = ""` in the API response, the frontend CANNOT delete tokens because `DeleteUserAccessToken` matches by token value. The copy-to-clipboard function also breaks.

**Decision: Make DeleteUserAccessToken support ID-based deletion, then redact the List response.**

**Files:**
- `server/router/api/v1/user_service.go` — `ListUserAccessTokens` and `DeleteUserAccessToken`
- `web/src/components/Settings/AccessTokenSection.tsx` — frontend usage
- `server/router/api/v1/user_service.go` — `CreateUserAccessToken` response (keep token for creation flow only)

**Changes:**
1. **Add token ID field** — Add an `AccessTokenHash` or `AccessTokenID` field to `UserAccessToken` protobuf (SHA256 prefix). For now, use the `issued_at` timestamp as a stable identifier for deletion, or add a hash.

   *Alternative (simpler):* Keep the raw token in the Create response (user just created it, needs to copy it), but redact in the List response. Change `DeleteUserAccessToken` to accept a token hash instead of the full token.

2. **Phase 1 (immediate, sprint 1):** Redact tokens in List response. Users can see the formatted display and delete via the raw token they already have in sessionStorage.
3. **Phase 2 (sprint 2):** Add ID-based deletion for the long term.

**Simpler approach — keep raw token in List, redact only in logs:**
- Set `AccessToken = ""` in the ListUserAccessTokens response
- Leave `CreateUserAccessToken` response unchanged (newly created token needs to be shown once)
- Change `DeleteUserAccessToken` to match by `IssuedAt` (timestamp) instead of token string, so delete-from-list still works

---

### P0-6 — XSS still exists in `generateWidgetScript` fallback (NEW FINDING)

**Problem:** `generateWidgetScript` (lines 1659-1720) uses `json.Marshal` for the FIRST IIFE (lines 1669-1678) but concatenates RAW unescaped values in the SECOND IIFE (lines 1684-1686). The earlier fix only covered the first half of the function.

```go
// Line 1679+: RAW concatenation — XSS vector still alive
return fmt.Sprintf(`...%s...`, safeBaseURL, ...) +
    `(function() {
  var config = {
    baseURL: '` + baseURL + `',          // RAW — XSS!
    companyName: '` + companyName + `',  // RAW — XSS!
  };
  ...`
```

**Fix:** Replace the second IIFE with safe versions OR delete the duplicate and refactor to a single IIFE with all features.

**Files:**
- `server/router/api/v1/agent/handlers.go` (lines 1679-1688)

**Changes:**
Replace the raw concatenation at lines 1679-1688 with the safe versions:
```go
return fmt.Sprintf(`(function() {
  ...
  var config = {
    baseURL: %s,
    tenantSlug: %s,
    companyName: %s,
    ...
  };
`, string(safeBaseURL), string(safeSlug), string(safeCompany)) + `...`[
```

**Verification:**
- `companyName = "</script><script>alert(1)</script>"` → output has `\u003c/script\u003e` (no script execution) ✓
- Test with double-quote, backslash, and other special characters ✓

---

### P1-7 — HandleOnboard rate limit: 100/min may be too low

**Problem:** plan3.md suggests 100/min for HandleOnboard. An admin onboarding 100+ tenants in a minute during bulk import would hit the cap.

**Decision:** Use 300/min for HandleOnboard with a separate key `"admin_onboard"` (not `tenant.ID + "admin_mutation"`). This is per-admin, not per-IP, to avoid one bad client exhausting the bucket for all admins.

**File:** `server/router/api/v1/agent/handlers.go` (~line 1225, `HandleOnboard`)

**Changes:**
```go
// At the start of HandleOnboard:
if err := h.checkAdminMutationRateLimit(c, 0); err != nil {
    return err
}
// Then existing code...
``

But we need to differentiate: add a new method or modify `checkAdminMutationRateLimit` to support a global (tenantID=0) key. Since `checkAdminMutationRateLimit` uses `tenantID` as part of the rate limit key, passing 0 is acceptable as a global bucket.

---

### P0-5 — Already implemented (verify only)

**Confirmed:** `server/server.go` already auto-generates UUID SecretKey and checks for empty (lines 61-74). The `grep -r '"usememos"'` check should be run to verify no remaining references.

**Only remaining action:**
```bash
grep -r '"usememos"' --include="*.go" /home/chaschel/Documents/go/bchat/
grep -r '"usememos"' --include="*.{ts,tsx,js}" /home/chaschel/Documents/go/bchat/web/
```
If any reference is found, replace with `s.Secret` reference or confirm it's in a comment/test fixture.

---

### P1-8 — Unnamed call sites still have soft-fallback

**Problem:** plan3.md adds a `requireLLMConfig` wrapper but only addresses 2 of ~16 call sites. The remaining 14 sites silently fall back to the global key if tenant key decryption fails.

**Decision:** Add the `requireLLMConfig` wrapper that errors hard on decryption failure, and apply it ONLY to the 2 chat-critical endpoints (`HandleChatExternal`, `HandleChatInternal`). Leave the 7 non-critical sites (classifyIntent, observer, simulation, etc.) with their current soft-fallback — the plan3_review.md confirms this is acceptable.

**Current call sites of getLLMConfig (`service.go`):**

| Line | Function | Apply requireLLMConfig? |
|------|----------|------------------------|
| 1233 | broadcastProvider | No (soft-fallback fine) |
| 1247 | broadcastProvider (alt path) | No |
| 1262 | broadcastProvider | No |
| 2005 | classifyIntent | **No** (soft-fallback: "unknown intent") |
| 2151 | chat handler (internal) | **YES — chat-critical** |
| 2614 | chat handler (external) | **YES — chat-critical** |
| 3900 | simulation | No |
| 3930 | simulation | No |
| 3961 | simulation | No |

Additional call sites in:
- `simulation.go:399,558` — No (defense-in-depth, not user-facing)
- `analysis.go:99` — No
- `observer.go:154` — No
- `observer_buffer.go:183` — No

---

### P1-9 — Duplicate imports need verification

**Problem:** plan3.md uses `sort.Slice` and `jwt.NewParser().ParseUnverified` which require imports. Confirmed: `user_service.go` already imports `"github.com/golang-jwt/jwt/v5"`. Need to verify `sort` is imported (or use `slices.SortFunc` which is already used at line 397).

**Fix:** Use existing `slices.SortFunc` (already imported) instead of adding `sort.Slice`.

---

### P1-10 — ExtractWorkspaceSettingKeyFromName error handling

**Problem:** `workspace_setting_service.go:103` silently defaults when `ExtractWorkspaceSettingKeyFromName` fails.

**Fix:** Return `codes.InvalidArgument` instead of silently defaulting.

---

## Implementation Order (Sprints)

### Sprint 1: "Stop the Bleeding" (~2 days)

| # | Item | Effort | File(s) | Verification |
|---|------|--------|---------|-------------|
| 1 | **INFRA-11** — Rotate secrets | 15 min | `.env`, Fly secrets | Rotate BEFORE P0-5 check |
| 2 | **INFRA-12** — fly.toml fixes | 10 min | `fly.toml` | Confirm ~2x cost with user; `min_machines=1`, `cpu_kind=performance`, `[[checks]]` |
| 3 | **P0-5 verify** — grep "usememos" | 5 min | Repo-wide grep | Zero refs remaining |
| 4 | **P0-2** — Migration script | 30 min | `scripts/migrate-old-tokens/main.go` (new) | Script parses JWT exp and removes >30d tokens |
| 5 | **P0-3 Phase 1** — Token redaction in List response | 20 min | `user_service.go:385-386` | List response has `access_token = ""`; Create response still shows token |

### Sprint 2: "Prevent Exploitation" (~2 days)

| # | Item | Effort | File(s) | Verification |
|---|------|--------|---------|-------------|
| 6 | **P0-1** — Session revocation on password change (with retry + honest docs) | 1-2 hours | `user_service.go` | Password change + token cleanup; retry on failure; 500 on permanent failure |
| 7 | **P0-6** — Fix XSS in generateWidgetScript fallback | 15 min | `handlers.go:1679-1688` | Replace raw concatenation with `json.Marshal`-safe values |
| 8 | **P0-4** — isDomainAllowed fail-closed | 15 min | `handlers.go:1904` | Parse error → `return false` + error log; empty list → `return true` (backward compat) |
| 9 | **P1-7** — Rate limit on missing admin endpoints | 1 hour | `handlers.go` (9 endpoints) | 10 endpoints now call `checkAdminMutationRateLimit`; HandleOnboard uses global bucket with 300/min |

### Sprint 3: "Harden Isolation" (~2 days)

| # | Item | Effort | File(s) | Verification |
|---|------|--------|---------|-------------|
| 10 | **P1-8** — requireLLMConfig wrapper for chat-critical paths | 30 min | `service.go` (lines 2151, 2614) | Chat returns error on broken key; non-critical sites keep soft-fallback |
| 11 | **P1-9** — Access token dedup (iat-sorted max-N=10) | 30 min | `user_service.go:506-528` (`UpsertAccessTokenToStore`) | Max 10 tokens per user; oldest evicted |
| 12 | **P1-10** — Error handling fix | 10 min | `workspace_setting_service.go:103` | `ExtractWorkspaceSettingKeyFromName` error → InvalidArgument |
| 13 | **P0-3 Phase 2** — ID-based token deletion | 30 min | `user_service.go`, `AccessTokenSection.tsx` | Frontend can delete tokens from list without raw token value |

### Sprint 4: "Observability & Infrastructure" (3-5 days)

| # | Item | Effort |
|---|------|--------|
| 14 | Postgres migration | 2-3 days |
| 15 | Monitoring (Prometheus/Grafana + Sentry) | 1 day |
| 16 | CI/CD pipeline | 1 day |
| 17 | Backup/DR documentation | 0.5 day |

---

## Verbatim Code Changes

### P0-4: isDomainAllowed fail-closed (`handlers.go:1898-1906`)

**Current:**
```go
if err := json.Unmarshal([]byte(allowedDomainsJSON), &domains); err != nil {
    return true // Invalid JSON, allow by default — FAIL-OPEN
}
```

**Change to:**
```go
if err := json.Unmarshal([]byte(allowedDomainsJSON), &domains); err != nil {
    slog.Error("domain allowlist contains invalid JSON, denying access",
        "allowed_domains", allowedDomainsJSON, "error", err)
    return false // FAIL-CLOSED
}
if len(domains) == 0 {
    slog.Warn("domain allowlist is empty, allowing all origins",
        "allowed_domains", allowedDomainsJSON)
    return true // Empty list = no restrictions (backward compat)
}
```

---

### P1-9: UpsertAccessTokenToStore max-N dedup (`user_service.go:506-528`)

**Current:** Always appends new token, no cap.

**Change to: Enforce max 10 tokens, evict oldest by `iat`:**
```go
func (s *APIV1Service) UpsertAccessTokenToStore(ctx context.Context, user *store.User,
    accessToken, description string) error {

    userAccessTokens, err := s.Store.GetUserAccessTokens(ctx, user.ID)
    if err != nil {
        return errors.Wrap(err, "failed to get user access tokens")
    }

    // Parse the new token to get its issued-at time
    newClaims := &ClaimsMessage{}
    if _, _, err := jwt.NewParser().ParseUnverified(accessToken, newClaims); err != nil {
        return errors.Wrap(err, "failed to parse new access token claims")
    }

    userAccessToken := &storepb.AccessTokensUserSetting_AccessToken{
        AccessToken: accessToken,
        Description: description,
    }
    userAccessTokens = append(userAccessTokens, userAccessToken)

    // Sort by issued-at (ascending) so we can evict oldest first
    slices.SortFunc(userAccessTokens, func(a, b *storepb.AccessTokensUserSetting_AccessToken) int {
        aClaims := &ClaimsMessage{}
        bClaims := &ClaimsMessage{}
        _, _, _ = jwt.NewParser().ParseUnverified(a.AccessToken, aClaims)
        _, _, _ = jwt.NewParser().ParseUnverified(b.AccessToken, bClaims)
        if aClaims.IssuedAt.Unix() < bClaims.IssuedAt.Unix() {
            return -1
        }
        return 1
    })

    // Enforce max 10 tokens — evict oldest (first after sort)
    const maxTokens = 10
    if len(userAccessTokens) > maxTokens {
        userAccessTokens = userAccessTokens[len(userAccessTokens)-maxTokens:]
    }

    // Upsert
    if _, err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{...}); err != nil {
        return errors.Wrap(err, "failed to upsert user setting")
    }
    return nil
}
```

**Note:** The repeated JWT parsing per token is not ideal. For a more efficient approach, parse once at list time and cache. But for correctness, this keeps the change minimal.

---

### P1-10: ExtractWorkspaceSettingKeyFromName error (`workspace_setting_service.go`)

**Current (approximate):**
```go
key, err := ExtractWorkspaceSettingKeyFromName(request.Setting.Name)
if err != nil {
    // silently continues with empty key
}
```

**Change to:**
```go
key, err := ExtractWorkspaceSettingKeyFromName(request.Setting.Name)
if err != nil {
    return nil, status.Errorf(codes.InvalidArgument, "invalid setting name: %v", err)
}
```

---

## Verification Commands

```bash
# Discover test alias from Taskfile
cat Taskfile.yml | grep -A2 "^  test:"

# Run backend tests
# (use the discovered alias or direct go test)
go test ./server/router/api/v1/... -count=1 -race
go test ./server/router/api/v1/agent/... -count=1 -race

# Verify P0-5 — no "usememos" references
grep -r '"usememos"' --include="*.go" .

# Verify P0-2 — MaxNeverExpireDuration constant
grep -rn "MaxNeverExpireDuration" server/router/api/v1/auth.go

# Verify P0-1 — target user ID vs currentUser.ID
grep -rn "deleteAllUserAccessTokens" server/router/api/v1/
```

---

## Summary of plan3_review.md Findings Resolution

| Issue | Status | Resolution |
|-------|--------|-----------|
| INFRA-13 blocks non-existent path | **DROPPED** | User not using nginx |
| P0-1 "rolled back" is false | **FIXED** | Retry + honest doc + deferred recovery |
| P0-2 migration is comment-only | **FIXED** | New `scripts/migrate-old-tokens/main.go` |
| P0-3 Phase 2b struct vs protobuf | **FIXED** | Phase 1: redact List; Phase 2: ID-based delete |
| P1-8 unnamed call sites | **FIXED** | `requireLLMConfig` on chat-critical paths only |
| P0-5 comment clarity | **FIXED** | Comment in server.go is already clear |
| P1-7 HandleOnboard 100/min cap | **FIXED** | 300/min global bucket for onboard |
| P1-9 import verification | **FIXED** | Use existing `slices.SortFunc` instead of `sort.Slice` |
| P0-6 second IIFE still has XSS (NEW) | **FIXED** | Replace raw concatenation with json.Marshal |

**Ready to implement. Toggle to ACT mode to begin Sprint 1.**
