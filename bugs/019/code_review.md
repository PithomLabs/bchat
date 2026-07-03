# Adversarial Code Review: Security Implementation

**Reviewer:** mimo
**Date:** 2026-07-04
**Scope:** All P0/P1 security fixes in the bchat codebase

---

## CRITICAL: P0-3 Token Redaction Breaks Frontend (Functional Regression)

**Severity:** Critical — token management page is completely broken
**Status:** SHIPPING BLOCKER

**Files:** `server/router/api/v1/user_service.go:417` + `web/src/components/Settings/AccessTokenSection.tsx`

The backend redacts `AccessToken = ""` in `ListUserAccessTokens` (line 417), but the frontend depends on the raw token value for **4 operations**:

| Line | Usage | Impact with `""` |
|------|-------|-------------------|
| 99 | `key={userAccessToken.accessToken}` | All tokens have same React key — rendering broken, cannot distinguish tokens |
| 101 | `getFormatedAccessToken(userAccessToken.accessToken)` | Displays `""****""` — meaningless to user |
| 102 | `copyAccessToken(userAccessToken.accessToken)` | Copies empty string to clipboard |
| 119 | `handleDeleteAccessToken(userAccessToken.accessToken)` | Sends `accessToken: ""` to backend |

The backend `DeleteUserAccessToken` at `user_service.go:517` does:
```go
if userAccessToken.AccessToken == request.AccessToken {
    continue // skip this token (i.e., delete it)
}
```

When the frontend sends `accessToken: ""`, this comparison matches **zero tokens** (none have `AccessToken == ""` in the DB — only the API response is redacted). **Token deletion is silently broken.** Users cannot revoke sessions.

**Additionally:** `getFormatedAccessToken` at line 48-50 does `accessToken.slice(0, 4)` on an empty string → returns `""`, so the display shows `****` and the copy button copies nothing.

**Verdict:** The P0-3 Phase 1 redaction was deployed without the Phase 2 ID-based deletion. The frontend was not updated. This is a **shipping blocker** — the token management page is non-functional.

**Fix options (urgent):**
1. **Revert P0-3** — un-redact the List response until Phase 2 (ID-based delete) is implemented with frontend changes.
2. **Ship Phase 2 now** — add an `id` field (e.g., SHA256 prefix) to the protobuf response, update `DeleteUserAccessToken` to accept `id` instead of raw token, update the frontend.

---

## HIGH: P1-8 `requireLLMConfig` Not Implemented

**Severity:** High — tenant isolation leak persists
**Status:** Not implemented

**File:** `server/router/api/v1/agent/service.go:1197-1230`

The plan specifies adding a `requireLLMConfig` wrapper that fails hard when a tenant's encrypted API key cannot be decrypted. The implementation does **not exist** — `grep -r "requireLLMConfig"` returns zero results.

Current code at `service.go:1212-1213`:
```go
} else if err != nil {
    slog.Warn("Failed to decrypt tenant OpenRouter API key", "tenantID", tenantID, "error", err)
}
```

After this warning, `apiKey` remains `""`, and line 1225-1226 falls back to the global env key:
```go
if apiKey == "" {
    apiKey = s.profile.OpenRouterAPIKey
}
```

**Impact:** A tenant with a corrupted encrypted key silently uses the global OpenRouter API key. This breaks tenant isolation for LLM billing — the platform owner pays for the tenant's LLM usage without knowing it.

---

## HIGH: P1-10 `convertWorkspaceSettingToStore` Error Still Discarded

**Severity:** High — data corruption vector
**Status:** Not fixed

**File:** `server/router/api/v1/workspace_setting_service.go:107`

```go
func convertWorkspaceSettingToStore(setting *v1pb.WorkspaceSetting) *storepb.WorkspaceSetting {
    settingKeyString, _ := ExtractWorkspaceSettingKeyFromName(setting.Name) // ERROR DISCARDED!
```

While `SetWorkspaceSetting` (line 73) validates the name before calling this function, the `_, _` discard means any future caller of `convertWorkspaceSettingToStore` that skips validation will silently map a malformed name to `GeneralSetting` (the zero-value enum default), writing data under the wrong key.

---

## MEDIUM: P0-1 Deferred Goroutine Leaks Context

**Severity:** Medium — goroutine leak on shutdown

**File:** `server/router/api/v1/user_service.go:244-249`

```go
go func(uid int32) {
    if deferredErr := s.deleteAllUserAccessTokens(ctx, uid); deferredErr != nil {
        slog.Error("Deferred token cleanup also failed; manual intervention required",
            "target_user_id", uid, "error", deferredErr)
    }
}(user.ID)
```

The goroutine captures `ctx` from the request context. When the HTTP request completes and the server shuts down, `ctx` is cancelled. The `deleteAllUserAccessTokens` call uses `s.Store.UpsertUserSetting(ctx, ...)` — with a cancelled context, the DB operation will fail immediately, so the deferred recovery never actually executes.

**Fix:** Create a new background context: `bgCtx := context.Background()` and pass it to the goroutine.

---

## MEDIUM: P0-6 `generateWidgetScript` — `companyName` Unused But Marshaled

**Severity:** Low (no current vulnerability) — dead code

**File:** `server/router/api/v1/agent/handlers.go:1697`

```go
_, _ = json.Marshal(companyName) // kept for future use (displaying company name in widget UI)
```

The `companyName` parameter is marshaled but discarded. The second IIFE no longer references `companyName` in its config. This is harmless but misleading — a future developer might see the parameter and assume it's used. Either remove the parameter entirely or add it to the config.

---

## MEDIUM: P1-9 Unparseable Tokens Silently Persist

**Severity:** Medium — token accumulation vector

**File:** `server/router/api/v1/user_service.go:553-554`

```go
_, _, _ = jwt.NewParser().ParseUnverified(a.AccessToken, aClaims)
_, _, _ = jwt.NewParser().ParseUnverified(b.AccessToken, bClaims)
```

If a token is malformed (e.g., corrupted DB write, old format), `ParseUnverified` returns an error that is silently discarded. The `ClaimsMessage` struct retains its zero value (`IssuedAt` = epoch zero). These corrupted tokens sort to the front (oldest) and are evicted first — which is the correct behavior. However, they persist in the DB indefinitely until the user signs in enough times to trigger eviction. There is no logging of corrupted tokens.

**Fix:** Add a `slog.Warn` when `ParseUnverified` fails so corrupted tokens are visible in logs.

---

## MEDIUM: P0-4 `isDomainAllowed` Missing Error Log

**Severity:** Low — observability gap

**File:** `server/router/api/v1/agent/handlers.go:1923-1924`

```go
if err := json.Unmarshal([]byte(allowedDomainsJSON), &domains); err != nil {
    return false // Invalid JSON, deny (fail-closed)
}
```

The plan specified adding `slog.Error("domain allowlist contains invalid JSON, ...")` but the implementation only returns `false` without logging. A tenant with corrupted `AllowedDomains` will get silent 403s with no error trail.

---

## LOW: P0-1 Error Message Returns Original Error, Not Retry Error

**Severity:** Low — misleading error message

**File:** `server/router/api/v1/user_service.go:250-252`

```go
return nil, status.Errorf(codes.Internal,
    "password changed but failed to invalidate existing sessions. "+
        "Admin must manually purge tokens via SQL. Error: %v", err)
```

The `%v` formats the **original** `err` (first attempt), not `retryErr` (the actual failure reason after retry). If the first attempt failed with "connection refused" but the retry failed with "permission denied", the admin sees the wrong error.

**Fix:** Change `err` to `retryErr`.

---

## LOW: Migration Script `extractExpClaim` Uses Non-Standard Base64 Fallback

**Severity:** Low — defensive coding, not a bug

**File:** `scripts/migrate-old-tokens/main.go:140-147`

```go
payload, err := base64.RawURLEncoding.DecodeString(parts[1])
if err != nil {
    payload, err = base64.StdEncoding.DecodeString(parts[1])
```

JWT payloads are always `base64.RawURLEncoding` per RFC 7515. The `StdEncoding` fallback is dead code — if a token has standard base64 padding, it's not a valid JWT. Harmless but unnecessary.

---

## Summary

| Issue | Severity | Status | Action |
|-------|----------|--------|--------|
| **P0-3: Frontend broken by token redaction** | **Critical** | **SHIPPING BLOCKER** | Revert redaction OR ship Phase 2 with frontend |
| **P1-8: `requireLLMConfig` not implemented** | High | Not done | Implement the wrapper for chat-critical paths |
| **P1-10: Error discarded in `convertWorkspaceSettingToStore`** | High | Not done | Fix line 107 |
| P0-1: Deferred goroutine leaks context | Medium | Bug | Use `context.Background()` |
| P1-9: Unparseable tokens not logged | Medium | Missing log | Add `slog.Warn` |
| P0-4: Missing error log on parse failure | Medium | Missing log | Add `slog.Error` |
| P0-1: Error message shows first attempt error | Low | Bug | Use `retryErr` |
| P0-6: `companyName` parameter unused | Low | Dead code | Remove or use |
| Migration: Non-standard base64 fallback | Low | Unnecessary | Remove |

**Bottom line:** P0-3 is a shipping blocker — the token management page is completely non-functional. P1-8 and P1-10 are not implemented. The remaining items are bugs and observability gaps.
