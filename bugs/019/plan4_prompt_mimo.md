# Coding Agent Prompt: Security Fixes per plan4_review_mimo.md

Implement the following fixes in order. Each fix includes the file, line, and exact change.

---

## Fix 1: P0-3 EMERGENCY — Revert Token Redaction (CRITICAL)

**File:** `server/router/api/v1/user_service.go:417`

**Problem:** `AccessToken: ""` breaks the frontend's `AccessTokenSection.tsx` — users cannot display, copy, or delete tokens.

**Change:** Revert to returning the raw token:
```go
// Line 417: change from
AccessToken: "",
// to
AccessToken: userAccessToken.AccessToken,
```

Keep the comment explaining Phase 2 will add ID-based deletion later.

**Verify:** The frontend at `web/src/components/Settings/AccessTokenSection.tsx` uses `userAccessToken.accessToken` for React key, display, copy, and delete. All 4 operations must work.

---

## Fix 2: P0-1 — Fix Deferred Goroutine Context Leak

**File:** `server/router/api/v1/user_service.go:244-249`

**Problem:** The goroutine captures the request `ctx`. When the request ends, `ctx` is cancelled and the deferred token cleanup always fails silently.

**Change:**
```go
// Line 244: change from
go func(uid int32) {
    if deferredErr := s.deleteAllUserAccessTokens(ctx, uid); deferredErr != nil {
// to
go func(uid int32) {
    bgCtx := context.Background()
    if deferredErr := s.deleteAllUserAccessTokens(bgCtx, uid); deferredErr != nil {
```

Ensure `"context"` is imported (it likely already is).

---

## Fix 3: P0-1 — Fix Error Message Showing Wrong Error Variable

**File:** `server/router/api/v1/user_service.go:250-252`

**Problem:** The error message formats the original `err` (first attempt) instead of `retryErr` (actual failure).

**Change:**
```go
// Line 252: change from
"Admin must manually purge tokens via SQL. Error: %v", err)
// to
"Admin must manually purge tokens via SQL. Error: %v", retryErr)
```

---

## Fix 4: P0-4 — Add Missing Error Log in `isDomainAllowed`

**File:** `server/router/api/v1/agent/handlers.go:1923-1924`

**Problem:** Parse error returns `false` but logs nothing. Tenant gets silent 403s.

**Change:**
```go
// Line 1923-1924: change from
if err := json.Unmarshal([]byte(allowedDomainsJSON), &domains); err != nil {
    return false // Invalid JSON, deny (fail-closed)
}
// to
if err := json.Unmarshal([]byte(allowedDomainsJSON), &domains); err != nil {
    slog.Error("domain allowlist contains invalid JSON, denying access",
        "allowed_domains", allowedDomainsJSON, "error", err)
    return false // Invalid JSON, deny (fail-closed)
}
```

---

## Fix 5: P1-9 — Log Unparseable Tokens During Sort

**File:** `server/router/api/v1/user_service.go:553-554`

**Problem:** `ParseUnverified` errors are silently discarded. Corrupted tokens accumulate with no visibility.

**Change:**
```go
// Lines 553-554: change from
_, _, _ = jwt.NewParser().ParseUnverified(a.AccessToken, aClaims)
_, _, _ = jwt.NewParser().ParseUnverified(b.AccessToken, bClaims)
// to
if _, _, err := jwt.NewParser().ParseUnverified(a.AccessToken, aClaims); err != nil {
    slog.Warn("failed to parse access token during dedup sort", "error", err)
}
if _, _, err := jwt.NewParser().ParseUnverified(b.AccessToken, bClaims); err != nil {
    slog.Warn("failed to parse access token during dedup sort", "error", err)
}
```

---

## Fix 6: P1-10 — Fix Discarded Error in `convertWorkspaceSettingToStore`

**File:** `server/router/api/v1/workspace_setting_service.go:107`

**Problem:** `ExtractWorkspaceSettingKeyFromName` error is discarded with `_, _`. Malformed name maps to zero-value enum.

**Change:** This function is called from `SetWorkspaceSetting` which already validates at line 73. The cleanest fix is to propagate the error:
```go
// Line 106-107: change from
func convertWorkspaceSettingToStore(setting *v1pb.WorkspaceSetting) *storepb.WorkspaceSetting {
    settingKeyString, _ := ExtractWorkspaceSettingKeyFromName(setting.Name)
// to
func convertWorkspaceSettingToStore(setting *v1pb.WorkspaceSetting) (*storepb.WorkspaceSetting, error) {
    settingKeyString, err := ExtractWorkspaceSettingKeyFromName(setting.Name)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid setting name: %v", err)
    }
```

Update the call site at line 76 to handle the error:
```go
updateSetting, err := convertWorkspaceSettingToStore(request.Setting)
if err != nil {
    return nil, err
}
```

Update the return type at line 128 accordingly. If this is too invasive (touches many callers), an acceptable alternative is to add a `slog.Error` at line 107 instead of propagating.

---

## Verification

After all fixes:
```bash
go test ./server/router/api/v1/... -count=1 -race
go test ./server/router/api/v1/agent/... -count=1 -race
grep -r '"usememos"' --include="*.go" .
```

Check the frontend loads the token management page and can display, copy, and delete tokens.
