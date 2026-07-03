# Prompt: Write Implementation Plan for Remaining Follow-ups

Write an implementation plan for the 3 remaining security follow-ups below. For each, provide: files to modify, exact changes, verification steps, and estimated effort.

Reference the existing plans in `/home/chaschel/Documents/go/bchat/bugs/019/` for context (especially `plan_implementation.md` and `code_review.md`).

---

## Follow-up 1: P0-3 Phase 2 — ID-Based Token Deletion

**Problem:** `ListUserAccessTokens` now returns raw JWTs (reverted in Fix 1). The frontend's `AccessTokenSection.tsx` uses the raw token for React key, display, copy, and delete. This works but leaks raw JWTs in the API response. Phase 2 adds an `id` field so the frontend can delete tokens without receiving the raw JWT.

**Scope:**
- Add a stable identifier field to the `UserAccessToken` protobuf (e.g., `string id = N;` using SHA256 of the token prefix, or `int64 issued_at` as the identifier).
- Update `DeleteUserAccessToken` in `user_service.go` to accept the `id` field instead of (or in addition to) the raw token.
- Update `ListUserAccessTokens` to return the `id` field and redact the raw token (`AccessToken: ""`).
- Update `web/src/components/Settings/AccessTokenSection.tsx` to use the `id` field for React key and delete operations, and remove raw token usage.
- Keep the raw token in the `CreateUserAccessToken` response (user needs to copy it once at creation time).

**Files likely affected:**
- `proto/api/v1/user_service.proto` (add `id` field to `UserAccessToken`)
- `server/router/api/v1/user_service.go` (populate `id` in List, use `id` in Delete)
- `web/src/components/Settings/AccessTokenSection.tsx` (use `id` instead of `accessToken`)
- `web/src/types/proto/api/v1/user_service.ts` (regenerated from proto)

---

## Follow-up 2: P1-8 — `requireLLMConfig` Wrapper

**Problem:** `getLLMConfig` at `service.go:1198` silently falls back to the global `OpenRouterAPIKey` when a tenant's encrypted key fails to decrypt. This breaks tenant LLM billing isolation — the platform owner pays for the tenant's usage.

**Scope:**
- Add a `requireLLMConfig(ctx, tenantID) (model, apiKey, error)` wrapper that returns an error when a tenant key exists but decryption fails.
- Apply it ONLY to the 2 chat-critical call sites: `generateResponse` (line ~2151) and `generateResponseWithRAG` (line ~2614).
- Leave the other 7 call sites (`withTenantEmbeddingAPIKey`, `getSimulationHumanModel`, `verifyResponseWithLLM`, `classifyIntent`, `GenerateAnnotatedKB`, `GenerateAnnotatedPolicy`, `CallLLMSimple`) using the existing `getLLMConfig` with soft fallback.
- Add an explicit `if apiKey == ""` early return at the chat call sites so the user gets a clear "API key not configured" message instead of a cryptic 500.

**Files likely affected:**
- `server/router/api/v1/agent/service.go` (add `requireLLMConfig`, modify 2 call sites)

---

## Follow-up 3: P0-6 — Remove Unused `companyName` Parameter

**Problem:** `generateWidgetScript` at `handlers.go:1692` accepts `companyName` but marshals it and discards the result. Dead code.

**Scope:**
- Remove the `companyName` parameter from `generateWidgetScript`.
- Remove the `_, _ = json.Marshal(companyName)` line.
- Update the call site at `HandleWidgetEmbed` (around line 1662) to stop passing `companyName`.
- Verify no other callers exist.

**Files likely affected:**
- `server/router/api/v1/agent/handlers.go` (function signature + call site)

---

## Output Format

Write the plan to `/home/chaschel/Documents/go/bchat/bugs/019/plan_phase2.md` with:
1. One section per follow-up
2. For each: problem statement, files, verbatim code changes, verification steps, effort estimate
3. Sprint ordering (which to implement first and why)
4. A verification block with `go build`, `go test`, and frontend smoke test commands
