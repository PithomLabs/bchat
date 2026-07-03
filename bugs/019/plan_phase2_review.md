# Review of `plan_phase2.md`

**Reviewer:** mimo
**Date:** 2026-07-04
**Verdict:** Approved with nits. One functional gap must be addressed before implementation.

---

## Functional Gap: P0-3 Phase 2 Copy Button Breaks

**Severity:** High — UX regression

**File:** `web/src/components/Settings/AccessTokenSection.tsx:102`

The plan's frontend code at line 157:
```tsx
<Button variant="plain" onClick={() => copyAccessToken(userAccessToken.accessToken)}>
```

Since `AccessToken` is redacted to `""` in the List response (plan line 88: `AccessToken: ""`), `userAccessToken.accessToken` is always `""`. **The copy button copies an empty string.** The user cannot copy their token after leaving the page.

The plan's verification step (line 186) says "Verify copy button copies raw token" — this will fail.

**Fix options (pick one):**
1. **Remove the copy button from the list view.** Users can only copy the token at creation time (from the `CreateUserAccessToken` response). This is the cleanest approach and aligns with the security goal of not exposing raw tokens.
2. **Add a `raw_token` field to the Create response only.** Store it in component state after creation and use it for the copy button. More complex.
3. **Keep `AccessToken` in the List response but only for the creating user's own tokens.** This defeats the purpose of Phase 2.

**Recommended:** Option 1. Remove the copy button and the `getFormatedAccessToken` display from the list view. Show only `description`, `issuedAt`, `expiresAt`, and the delete button.

---

## Minor Issues

### 1. P1-8: `requireLLMConfig` Duplicates `getLLMConfig` Logic

**File:** `service.go`

The plan's `requireLLMConfig` (lines 211-243) duplicates the entire body of `getLLMConfig` (lines 1198-1230). If `getLLMConfig` is ever updated (e.g., new config fields, different fallback logic), `requireLLMConfig` will drift.

**Better approach:** Wrap `getLLMConfig` instead of duplicating:
```go
func (s *Service) requireLLMConfig(ctx context.Context, tenantID int32) (model string, apiKey string, err error) {
    // Check if tenant has an encrypted key that failed to decrypt
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

This reuses `getLLMConfig` and only adds the fail-closed checks. Less code, no drift risk.

### 2. P1-8: `status.Errorf` in Non-gRPC Context

**File:** `service.go`

The plan uses `status.Errorf(codes.Internal, ...)` and `status.Errorf(codes.FailedPrecondition, ...)`. These are gRPC status codes. The callers return the error as a user-facing string, so the status codes are unused. Use `fmt.Errorf` instead for clarity.

### 3. P0-3 Phase 2: Proto Regeneration Tooling

The plan says "Run the project's proto regeneration command. If not available, manually regenerate using `buf generate` or `protoc`." This is vague. The implementing agent should first check:
```bash
grep -A2 "proto" Taskfile.yml
```
to find the actual command. If the project uses `buf`, the command is `buf generate`. If it uses `protoc`, the flags need to match the existing generated code style.

### 4. P0-6: Line Number Correct

The plan says `HandleWidgetEmbed` is at `handlers.go:1995`. Verified — the call `generateWidgetScript(baseURL, tenant.Slug, tenant.CompanyName)` is at line 1995. Correct.

---

## What's Solid

- Sprint ordering (P0-6 first, then P0-3 Phase 2, then P1-8) is correct — smallest/lowest risk first.
- P0-3 Phase 2 proto changes are well-specified (add `id` field, keep `access_token` in Delete for backward compat).
- P0-3 Phase 2 backend logic (SHA256 prefix, `DeleteUserAccessToken` matching) is correct.
- P1-8 correctly scoped to 2 call sites (`generateResponse` at 2151, `generateRAGResponse` at 2614).
- P1-8 correctly leaves 7 other call sites with soft fallback.
- P0-6 changes are minimal and correct.
- Effort estimates are reasonable.
- Verification blocks are comprehensive.

---

## Summary

| Issue | Severity | Action |
|-------|----------|--------|
| P0-3 Phase 2: copy button breaks | **High** | Remove copy button from list view or redesign |
| P1-8: code duplication | Nit | Wrap `getLLMConfig` instead of duplicating |
| P1-8: `status.Errorf` in HTTP context | Nit | Use `fmt.Errorf` |
| P0-3 Phase 2: proto tooling vague | Nit | Check Taskfile.yml first |

**Bottom line:** Fix the copy button gap (either remove it or redesign the flow), and this plan is implementation-ready.
