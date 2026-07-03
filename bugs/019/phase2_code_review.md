# Code Review: Implementation of plan_phase2.md

**Reviewer:** mimo
**Date:** 2026-07-04
**Verdict:** All 3 follow-ups implemented correctly. Build passes, tests pass.

---

## Follow-up 3 (P0-6): Remove Unused `companyName` — ✅ PASS

**File:** `handlers.go:1692`

```go
func generateWidgetScript(baseURL, tenantSlug string) string {
```

`companyName` parameter removed. `json.Marshal(companyName)` dead code removed. Call site at line 1994 updated:
```go
script := generateWidgetScript(baseURL, tenant.Slug)
```

---

## Follow-up 1 (P0-3 Phase 2): ID-Based Token Deletion — ✅ PASS

### Proto changes
**File:** `proto/api/v1/user_service.proto:234-242, 262-270`

```proto
message UserAccessToken {
  string access_token = 1;
  string description = 2;
  google.protobuf.Timestamp issued_at = 3;
  google.protobuf.Timestamp expires_at = 4;
  string id = 5;
}

message DeleteUserAccessTokenRequest {
  string name = 1;
  string access_token = 2;
  string id = 3;
}
```

Field numbers correct. Backward-compatible (old clients send `access_token`, new clients send `id`).

### Generated code
**File:** `proto/gen/api/v1/user_service.pb.go:965, 1193`

`Id string` field present in both `UserAccessToken` and `DeleteUserAccessTokenRequest` structs. TypeScript types at `web/src/types/proto/api/v1/user_service.ts:196, 224` also generated correctly.

### Backend — ListUserAccessTokens
**File:** `user_service.go:418-423`

```go
userAccessToken := &v1pb.UserAccessToken{
    AccessToken: "",
    Description: userAccessToken.Description,
    IssuedAt:    timestamppb.New(claims.IssuedAt.Time),
    Id:          sha256Prefix(userAccessToken.AccessToken),
}
```

Raw token redacted. `id` populated via `sha256Prefix` helper at line 498-502.

### Backend — DeleteUserAccessToken
**File:** `user_service.go:525-535`

```go
if request.Id != "" {
    matched = (sha256Prefix(userAccessToken.AccessToken) == request.Id)
} else {
    matched = (userAccessToken.AccessToken == request.AccessToken)
}
```

ID-based matching with raw-token fallback. Correct.

### Frontend — AccessTokenSection.tsx
**File:** `web/src/components/Settings/AccessTokenSection.tsx`

- `copy-to-clipboard` import removed (line 2)
- `ClipboardIcon` removed
- `copyAccessToken` function removed
- `getFormatedAccessToken` function removed
- Token column removed from table (no raw token displayed)
- React key uses `userAccessToken.id` (line 85)
- Delete uses `userAccessToken.id` (line 99)
- `handleDeleteAccessToken` takes `id` only (line 33)

**Copy button correctly removed** per plan_phase2_review.md finding. Users copy the token once at creation time only.

---

## Follow-up 2 (P1-8): `requireLLMConfig` Wrapper — ✅ PASS

### Wrapper
**File:** `service.go:1232-1248`

```go
func (s *Service) requireLLMConfig(ctx context.Context, tenantID int32) (model string, apiKey string, err error) {
    config, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID})
    if config != nil && len(config.OpenRouterAPIKeyEncrypted) > 0 && s.encryptionService != nil {
        if _, decryptErr := s.encryptionService.Decrypt(config.OpenRouterAPIKeyEncrypted, config.OpenRouterAPIKeyNonce); decryptErr != nil {
            return "", "", fmt.Errorf("tenant %d API key decryption failed: %w", tenantID, decryptErr)
        }
    }
    model, apiKey = s.getLLMConfig(ctx, tenantID)
    if apiKey == "" {
        return "", "", fmt.Errorf("no OpenRouter API key configured for tenant %d", tenantID)
    }
    return model, apiKey, nil
}
```

**Wraps `getLLMConfig`** (not duplicating logic — per review nit). Uses `fmt.Errorf` (not `status.Errorf`). Checks for decryption failure AND empty key.

### Applied to 2 chat-critical sites only
- `generateResponse` at line 2169: `model, apiKey, err := s.requireLLMConfig(ctx, config.TenantID)`
- `generateRAGResponse` at line 2632: `model, apiKey, err := s.requireLLMConfig(ctx, config.TenantID)`

Both return friendly message on error:
```go
if err != nil {
    return "I apologize, but the chat service is not currently available. Please call us directly.", nil
}
```

### Other 7 call sites unchanged
`withTenantEmbeddingAPIKey`, `getSimulationHumanModel`, `verifyResponseWithLLM`, `classifyIntent`, `GenerateAnnotatedKB`, `GenerateAnnotatedPolicy`, `CallLLMSimple` — all still use `getLLMConfig` with soft fallback.

---

## Build & Test Results

```
$ go build ./server/router/api/v1/...
(no output — success)

$ go test ./server/router/api/v1/... -count=1 -race
ok  github.com/usememos/memos/server/router/api/v1         9.912s
ok  github.com/usememos/memos/server/router/api/v1/agent  47.697s
```

---

## Summary

| Follow-up | Status | Correct? |
|-----------|--------|----------|
| P0-6: Remove `companyName` | ✅ Applied | Yes |
| P0-3 Phase 2: ID-based delete | ✅ Applied | Yes |
| P1-8: `requireLLMConfig` wrapper | ✅ Applied | Yes |
| Proto + generated code | ✅ Updated | Yes |
| Frontend | ✅ Updated | Yes |
| Build | ✅ Passes | — |
| Tests | ✅ Pass | — |

**Ready to merge.**
