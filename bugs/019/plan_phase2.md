# Implementation Plan: Phase 2 Follow-ups

**Date:** 2026-07-04
**Scope:** 3 remaining security follow-ups from `code_review.md` / `prompt_phase2.md`
**Prerequisite:** All fixes from `code2.md` have been applied and verified.

---

## Sprint Order

1. **Follow-up 3 (P0-6) first** — Smallest, lowest risk. Removes dead code and validates proto/TS regeneration workflow.
2. **Follow-up 1 (P0-3 Phase 2) second** — Medium effort. Protobuf change requires regenerating Go and TS code. Functional change to token deletion UX.
3. **Follow-up 2 (P1-8) third** — Medium effort. Scoped to 2 call sites. Adds tenant billing isolation fail-closed behavior.

---

## Follow-up 1: P0-3 Phase 2 — ID-Based Token Deletion

### Problem
`ListUserAccessTokens` currently returns raw JWTs to the frontend. The frontend uses the raw token for React keys, display, copy, and delete. This works but leaks raw JWTs in API responses and history. Phase 2 introduces a stable `id` field so the frontend can delete tokens without receiving the raw JWT.

### Files to Modify

1. `proto/api/v1/user_service.proto`
2. `server/router/api/v1/user_service.go`
3. `web/src/components/Settings/AccessTokenSection.tsx`
4. `web/src/types/proto/api/v1/user_service.ts`

### Changes

#### 1.1 `proto/api/v1/user_service.proto`

Add `id` to `UserAccessToken`:
```proto
message UserAccessToken {
  string access_token = 1;
  string description = 2;
  google.protobuf.Timestamp issued_at = 3;
  google.protobuf.Timestamp expires_at = 4;
  // Stable identifier for deletion without exposing raw token.
  // Format: SHA256 prefix of the access token (first 16 hex chars).
  string id = 5;
}
```

Add `id` to `DeleteUserAccessTokenRequest` (keep `access_token` for backward compatibility during migration):
```proto
message DeleteUserAccessTokenRequest {
  // The name of the user.
  string name = 1;
  // access_token is the access token to delete.
  string access_token = 2;
  // id is the stable identifier returned by ListUserAccessTokens.
  // When provided, access_token is ignored.
  string id = 3;
}
```

#### 1.2 Regenerate Protobuf Code

Run the project's proto regeneration command. If not available, manually regenerate using:
```bash
buf generate
# or
protoc --go_out=. --go_opt=paths=source_relative \
  --ts_out=. --ts_opt=output_nano_msgs=false \
  proto/api/v1/user_service.proto
```

Update generated files:
- `proto/gen/api/v1/user_service.pb.go`
- `web/src/types/proto/api/v1/user_service.ts`

#### 1.3 `server/router/api/v1/user_service.go`

**A. `ListUserAccessTokens` — populate `id` and redact token:**
```go
// Current (approx line 416):
userAccessToken := &v1pb.UserAccessToken{
    AccessToken: userAccessToken.AccessToken,
    Description: userAccessToken.Description,
    IssuedAt:    timestamppb.New(claims.IssuedAt.Time),
}

// Change to:
userAccessToken := &v1pb.UserAccessToken{
    // Phase 2: redact raw token from list response.
    AccessToken: "",
    Description: userAccessToken.Description,
    IssuedAt:    timestamppb.New(claims.IssuedAt.Time),
    // Stable ID derived from token prefix for client-side delete.
    Id: sha256Prefix(userAccessToken.AccessToken),
}
```

Add helper near `deleteAllUserAccessTokens`:
```go
// sha256Prefix returns the first 16 hex chars of the SHA256 hash.
func sha256Prefix(s string) string {
    hash := sha256.Sum256([]byte(s))
    return hex.EncodeToString(hash[:])[:16]
}
```

**B. `DeleteUserAccessToken` — accept `id` in addition to `access_token`:**
```go
func (s *APIV1Service) DeleteUserAccessToken(ctx context.Context, request *v1pb.DeleteUserAccessTokenRequest) (*emptypb.Empty, error) {
    // ... existing permission checks ...

    userAccessTokens, err := s.Store.GetUserAccessTokens(ctx, currentUser.ID)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to list access tokens: %v", err)
    }

    updatedUserAccessTokens := []*storepb.AccessTokensUserSetting_AccessToken{}
    for _, userAccessToken := range userAccessTokens {
        matched := false
        if request.Id != "" {
            matched = (sha256Prefix(userAccessToken.AccessToken) == request.Id)
        } else {
            matched = (userAccessToken.AccessToken == request.AccessToken)
        }
        if matched {
            continue // skip this token (i.e., delete it)
        }
        updatedUserAccessTokens = append(updatedUserAccessTokens, userAccessToken)
    }
    // ... upsert updated list ...
}
```

**C. Keep `CreateUserAccessToken` response unchanged** — it still returns the raw token once so the user can copy it at creation time.

#### 1.4 `web/src/components/Settings/AccessTokenSection.tsx`

```tsx
// Change functions to use token.id instead of token.accessToken
const copyAccessToken = async (accessToken: string) => {
    copy(accessToken);
    toast.success(t("setting.access-token-section.access-token-copied-to-clipboard"));
};

const handleDeleteAccessToken = async (id: string, accessToken: string) => {
    const formatedAccessToken = getFormatedAccessToken(accessToken);
    const confirmed = window.confirm(t("setting.access-token-section.access-token-deletion", { accessToken: formatedAccessToken }));
    if (confirmed) {
        await userServiceClient.deleteUserAccessToken({ name: currentUser.name, id });
        setUserAccessTokens(userAccessTokens.filter((token) => token.id !== id));
    }
};

// In table body:
{userAccessTokens.map((userAccessToken) => (
    <tr key={userAccessToken.id}> {/* use id for React key */}
        {/* display still shows formatted token for UX, but key is id */}
        <span className="font-mono">{getFormatedAccessToken(userAccessToken.accessToken)}</span>
        <Button variant="plain" onClick={() => copyAccessToken(userAccessToken.accessToken)}>
            <ClipboardIcon className="w-4 h-auto text-gray-400" />
        </Button>
        {/* ... */}
        <Button variant="plain" onClick={() => {
            handleDeleteAccessToken(userAccessToken.id, userAccessToken.accessToken);
        }}>
            <TrashIcon className="text-red-600 w-4 h-auto" />
        </Button>
    </tr>
))}
```

### Verification

```bash
# Build
go build ./server/router/api/v1/...

# Run tests
go test ./server/router/api/v1/... -count=1 -race

# Frontend type check (from web/)
npm run typecheck 2>/dev/null || npx tsc --noEmit

# Smoke test frontend
# 1. Navigate to Settings > Access Tokens
# 2. Verify tokens display formatted (first 4 + **** + last 4)
# 3. Verify React keys are stable (no duplicate key warnings)
# 4. Verify copy button copies raw token
# 5. Verify delete button works without sending raw token
```

---

## Follow-up 2: P1-8 — `requireLLMConfig` Wrapper

### Problem
`getLLMConfig` silently falls back to the global `OpenRouterAPIKey` when a tenant's encrypted key fails to decrypt. This breaks tenant LLM billing isolation — the platform owner pays for the tenant's usage.

### Files to Modify

1. `server/router/api/v1/agent/service.go`

### Changes

#### 2.1 Add `requireLLMConfig` wrapper

Near `getLLMConfig`:
```go
// requireLLMConfig returns the LLM model and API key for a tenant.
// Unlike getLLMConfig, it does NOT fall back to global env vars.
// It returns an error when a tenant key exists but decryption fails,
// ensuring tenant billing isolation for chat-critical paths.
func (s *Service) requireLLMConfig(ctx context.Context, tenantID int32) (model string, apiKey string, err error) {
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
            if err != nil {
                return "", "", status.Errorf(codes.Internal,
                    "failed to decrypt tenant OpenRouter API key for tenant %d: %v", tenantID, err)
            }
            if decrypted != "" {
                apiKey = decrypted
            }
        }
    }

    if model == "" {
        model = s.profile.LLMModel
        if model == "" {
            model = "openai/gpt-oss-120b:free"
        }
    }
    if apiKey == "" {
        return "", "", status.Errorf(codes.FailedPrecondition,
            "OpenRouter API key not configured for tenant %d", tenantID)
    }
    return model, apiKey, nil
}
```

#### 2.2 Apply to `generateResponse` only

Current (`service.go:2151`):
```go
model, apiKey := s.getLLMConfig(ctx, config.TenantID)
```

Change to:
```go
model, apiKey, err := s.requireLLMConfig(ctx, config.TenantID)
if err != nil {
    return "I apologize, but the chat service is not currently available. Please call us directly.", nil
}
```

#### 2.3 Apply to `generateRAGResponse` only

Current (`service.go:2614`):
```go
model, apiKey := s.getLLMConfig(ctx, config.TenantID)
```

Change to:
```go
model, apiKey, err := s.requireLLMConfig(ctx, config.TenantID)
if err != nil {
    return "I apologize, but the chat service is not currently available. Please call us directly.", nil
}
```

#### 2.4 Leave other 7 call sites unchanged

Keep soft-fallback behavior for:
- `withTenantEmbeddingAPIKey` (line 1232)
- `getSimulationHumanModel` (line 1239)
- `verifyResponseWithLLM` (line 1254)
- `classifyIntent` (line ~2003)
- `GenerateAnnotatedKB` (line ~3899)
- `GenerateAnnotatedPolicy` (line ~3929)
- `CallLLMSimple` (line ~3960)

### Verification

```bash
# Build
go build ./server/router/api/v1/...

# Run tests
go test ./server/router/api/v1/agent/... -count=1 -race

# Positive test: tenant with valid key → chat works
# Negative test: tenant with corrupted encrypted key → returns friendly message,
# does NOT fall back to global key
```

---

## Follow-up 3: P0-6 — Remove Unused `companyName` Parameter

### Problem
`generateWidgetScript` accepts `companyName` but marshals it and discards the result. Dead code.

### Files to Modify

1. `server/router/api/v1/agent/handlers.go`

### Changes

#### 3.1 Update `generateWidgetScript` signature and body

Current (`handlers.go:1692`):
```go
func generateWidgetScript(baseURL, tenantSlug, companyName string) string {
    // ...
    _, _ = json.Marshal(companyName) // kept for future use (displaying company name in widget UI)
    // ...
}
```

Change to:
```go
func generateWidgetScript(baseURL, tenantSlug string) string {
    // json.Marshal produces JS-safe strings: encodes < as \\u003c, > as \\u003e,
    // preventing </script> HTML parser breakout regardless of JS string context.
    safeBaseURL, _ := json.Marshal(baseURL)
    safeSlug, _ := json.Marshal(tenantSlug)

    return `(function() {
  'use strict';

  // Configuration -- all values are json.Marshal-safe (no XSS via </script>)
  var config = {
    baseURL: ` + string(safeBaseURL) + `,
    tenantSlug: ` + string(safeSlug) + `,
    primaryColor: '#0d9488'
  };
  // ... rest unchanged ...
})();`
}
```

#### 3.2 Update call site at `HandleWidgetEmbed`

Current (`handlers.go:1995`):
```go
script := generateWidgetScript(baseURL, tenant.Slug, tenant.CompanyName)
```

Change to:
```go
script := generateWidgetScript(baseURL, tenant.Slug)
```

#### 3.3 Verify no other callers
```bash
grep -rn "generateWidgetScript(" server/
```

Expected: only the `HandleWidgetEmbed` call site.

### Verification

```bash
# Build
go build ./server/router/api/v1/...

# Run tests
go test ./server/router/api/v1/... -count=1 -race

# Manual test:
# 1. Visit /widget/:slug/embed.js and confirm it loads
# 2. Confirm widget still renders with correct baseURL and tenantSlug
```

---

## Final Verification Block

```bash
# Build all
go build ./...

# Backend tests
go test ./server/router/api/v1/... -count=1 -race

go test ./server/router/api/v1/agent/... -count=1 -race

# Grep checks
grep -r '"usememos"' --include='*.go' .

# Frontend typecheck
cd web && npm run typecheck 2>/dev/null || npx tsc --noEmit
```

---

## Effort Estimates

| Follow-up | Effort | Risk |
|-----------|--------|------|
| P0-6 remove `companyName` | 15 min | Low — single function + one caller |
| P0-3 Phase 2 ID-based delete | 2-3 hours | Medium — proto change + regeneration + frontend + backend |
| P1-8 `requireLLMConfig` | 1-2 hours | Medium — new wrapper + 2 call site changes + tests |

Total estimated effort: **half day**.
