# Adversarial Review of `plan3.md`

The plan has been substantially revised from plan2 and addresses most issues from my prior review. **However, three critical and several significant issues remain.**

## Critical flaws (must fix)

### 1. INFRA-13: Nginx rule blocks a path that does not exist
The plan scopes the rule to `location = /api/v1/workspace/instance-setting`. **No such route exists in this codebase.** I verified:
- `v1.go` registers `WorkspaceSettingServiceServer` (gRPC), not a REST route.
- `acl_config.go` allowlists `/memos.api.v1.WorkspaceSettingService/GetWorkspaceSetting` and `.../SetWorkspaceSetting` (gRPC method names, not URL paths).
- The grpc-gateway transcoding exposes paths like `/api/v1/workspace.settings/{setting.name}` — not `instance-setting`.

**Result: the nginx rule is a complete no-op.** It blocks zero traffic. Worse, the claim "External request to `/api/v1/workspace/instance-setting` → 403" cannot be verified because no traffic ever hits that path.

**What the rule should actually target:**
- The gRPC-Gateway REST path for settings (likely `/api/v1/workspace.settings/*`), OR
- The gRPC-Web path `/memos.api.v1.WorkspaceSettingService/SetWorkspaceSetting`, OR
- Better: drop the nginx WAF entirely (since the backend already requires `RoleHost` for `SetWorkspaceSetting` per `workspace_setting_service.go:68`) and instead add an explicit check in `SetWorkspaceSetting` that rejects `setting.name == "instance-setting"` (or whatever the real name is) unless caller is on an allowlist IP.

The plan also needs to confirm what "UpdateInstanceSetting" actually maps to in bchat's compiled proto — it may not be exposed at all.

### 2. P0-1: Verification claim "password change is rolled back" is false
The verification step says: "If token invalidation fails (e.g., DB error) → API returns 500, password change is rolled back."

**The code does not roll back the password change.** Looking at the sequence in `user_service.go`:
```go
updatedUser, err := s.Store.UpdateUser(ctx, update)  // <-- password already persisted
...
if err := s.deleteAllUserAccessTokens(ctx, user.ID); err != nil {
    return nil, status.Errorf(codes.Internal, "password changed but failed to invalidate...")
}
```
The password is committed in the database before the invalidation runs. The 500 tells the caller to retry, but the user is now in a dangerous state: the new password is active AND the old tokens are still valid. A retry will try to set the (same) new password again, succeed, and try the invalidation again — but if invalidation keeps failing, the user is permanently stuck in "old tokens work, new password required" with no recovery.

**Options the plan should pick from:**
- (a) Wrap both `UpdateUser` and `deleteAllUserAccessTokens` in a single DB transaction. If either fails, both roll back. This requires the store to expose a transactional API.
- (b) Two-phase: write a `pending_password_change` row, set a background job to commit. Too complex for this fix.
- (c) Document explicitly: "If invalidation fails, the new password is in effect AND old tokens remain valid. Admin must manually purge tokens via SQL." Update the verification step to match reality.

Option (a) is the right fix. Option (c) is the minimum honest documentation.

### 3. P0-2: Old-token migration is a TODO, not a deliverable
The "Old 100-year token migration" section is:
```sql
-- SQLite: Delete all user settings with access tokens whose JWT exp > 30 days from now
-- This is a manual step, not automated in the fix.
-- The implementation agent should provide this as a script, not run it automatically.
```
That is a comment, not a migration. The Go one-off is also just a description (`// cmd/cleanup-old-tokens/main.go — provided as a script`). The plan still ships with no actual code for this.

For a P0 fix intended to run before any customer goes live, the migration needs to be one of:
- An actual SQL statement that does the cleanup (challenging because JWT `exp` is inside a serialized JSON column, not a queryable field).
- An actual Go program file (not a description) under `cmd/cleanup-old-tokens/main.go` with the implementation.
- Or a different design: bump the JWT signing key (`s.Secret`) once during deploy, which invalidates ALL existing tokens regardless of `exp`. This is much simpler and is the standard JWT rotation pattern.

**Recommendation:** invalidate by rotating `s.Secret` as part of the P0-2 / P0-5 deploy. Old tokens (including 100-year ones) become unverifiable. New signins issue new tokens with the 30-day cap. The plan's P0-5 already generates a UUID secret on first boot, so this can be triggered by `UPDATE`ing the SecretKey to a new UUID and restarting — no JWT parsing required.

## Significant gaps

### 4. P0-3: Phase 2b struct is wrong shape
```go
type UserAccessTokenResponse struct {
    ID          string `json:"id"`
    ...
    // AccessToken is intentionally OMITTED
}
```
The actual gRPC response type is `*v1pb.UserAccessToken` (a generated protobuf type), not a Go struct with JSON tags. Adding an `id` field requires:
- A new field in `proto/api/v1/user_service.proto` (e.g., `string id = 4;`).
- Regenerating `proto/gen/api/v1/*.pb.go` (requires `task proto` or equivalent).
- A new field in `storepb.AccessTokensUserSetting_AccessToken` if the `id` should be persisted (recommended, since the `id` must remain stable across list calls).
- Updating `DeleteUserAccessToken` to accept an `id` instead of the raw token, OR a new endpoint.

The plan glosses over all of this with a stand-in struct. The agent will likely try to change the Go struct directly and fail compilation, or implement a Go-side `id` that doesn't get serialized to the wire.

**Simplification:** if the audit shows the frontend uses `access_token` for delete, defer P0-3 entirely (as the plan's "default" suggests) and ship a follow-up proto change as a separate ticket.

### 5. P1-8: Doesn't name the 2 chat call sites
Plan says "Use `requireLLMConfig` at chat call sites (the 2 sites that make actual LLM calls for customer-facing chat). Leave the other 7 sites (embedding, verification, simulation) using `getLLMConfig` with their existing soft fallbacks."

From my earlier grep, the candidates are:
- Line 2005 (in `classifyIntent` — but this is intent classification, may not be "customer-facing chat")
- Line 2151 (in the chat handler — the actual customer-facing response)
- Line 2614 (likely a verifyResponseWithLLM call — verification, not customer-facing)
- Lines 3900, 3930, 3961 (simulation)

The two customer-facing chat sites are most likely lines **2151** (chat response) and **2005** (intent classification that gates the chat). The plan should name these explicitly with line numbers, or the agent will pick the wrong two (e.g., include simulation or verifier sites, which would break their existing soft fallbacks).

### 6. P1-9: ParseUnverified error path leaks storage
The code does:
```go
if err != nil {
    continue // Skip unparseable tokens (will be cleaned up)
}
```
The comment says "will be cleaned up" but the code does NOT clean up unparseable tokens — they persist in the user's setting. Over time, a user could accumulate many unparseable tokens (e.g., from a prior JWT format change, a bad secret rotation, or corruption). The fix should either:
- Remove unparseable tokens (treat them as oldest and evict).
- Move them to a separate "quarantine" list.
- Log a warning so the issue is visible.

A token that was signed with the OLD secret after a rotation will be unparseable with `ParseUnverified`? No — `ParseUnverified` does not verify the signature, it only parses the JWT structure. So this is mostly safe, but malformed/corrupted tokens would still leak.

## Minor issues

- **P0-6 verification case**: `CompanyName = "</script><img src=x onerror=alert(1)>"` — the expected output `baseURL: %s` with `json.Marshal` produces `baseURL: "\u003c/script\u003e..."`. **The plan still shows `baseURL: %s` (no surrounding quotes), but `json.Marshal` returns a string already including the quotes.** So the format string should be `baseURL: %s,` (which is what the code shows) AND json.Marshal output is `"value"` (with quotes) — the resulting JS is `baseURL: "value",` which is valid. OK, this is actually correct. The verification test in the plan correctly shows `\u003c` in the output. Good.

- **P0-1 helper location**: plan now says "Add helper to `user_service.go` (alongside existing `UpsertAccessTokenToStore` / `DeleteUserAccessToken` for cohesion)" — good, this was corrected from the prior plan2 which said `auth_service.go`.

- **P0-5 verification**: added `grep -r '"usememos"' --include="*.go" .` — good.

- **INFRA-12**: removed the bogus `protocol = "http"` and empty `[checks.headers]`. The `[[checks]]` block is now clean. Good.

- **Test plan**: still says `grep -A2 "^  test:" Taskfile.yml` to discover the alias. Acceptable as a discovery step, but the agent should be told to **commit the discovered command** to the plan output, not just discover it.

- **P1-7 `HandleImport` placement**: Plan says insert after `// Check admin role (~line 1331)`. Actual is at line 1332 (`if !h.isAdmin(c)` at 1332, comment at 1331). Insertion after the `if` block (after line 1334) is what we want, not after the comment. Plan should clarify "after the `return echo.NewHTTPError` on line 1333, before the tenant lookup."

- **P1-7 `HandleOnboard`**: 100/min cap is a reasonable improvement, but for a single admin onboarding 100 tenants in a minute is plausible during a bulk import. The cap should be either higher (e.g., 1000/min) or keyed by `clientIP + "onboard"` with a per-IP cap of e.g. 30/min (to prevent one bad client from exhausting the bucket for all admins).

- **P0-5 comment in code**: the comment "Defense-in-depth: abort if SecretKey is somehow empty" is accurate, but the variable name `secret := workspaceBasicSetting.SecretKey` followed by `if secret == ""` is a tautology given that `getOrUpsertWorkspaceBasicSetting` already generated a UUID. The check is correct but the variable name should clarify: `secret := workspaceBasicSetting.SecretKey // always non-empty after getOrUpsertWorkspaceBasicSetting unless DB write failed`.

- **P1-9 import**: uses `sort.Slice` and `jwt.NewParser().ParseUnverified` — the imports need to be added to `user_service.go`. Plan doesn't say so. The existing file uses `golang-jwt/jwt/v5` (per `v1.go:10`), so `jwt` is already imported in some files, but not necessarily in `user_service.go`. The agent should verify.

## What's solid (now resolved from prior reviews)

- P0-6 XSS: correctly uses `json.Marshal` which neutralizes `</script>` via `\u003c` encoding.
- P0-1: now fail-closed (returns 500 on invalidation failure).
- P0-3: now framed as two-phase with audit as a precondition.
- P1-8: now uses a `requireLLMConfig` wrapper that preserves soft fallbacks at the 7 non-chat sites.
- P1-9: now sorts by JWT `iat` before eviction.
- INFRA-13: now scoped (though to a non-existent path — see critical #1).
- INFRA-12: clean health check block.
- Test plan: now starts with `grep` of Taskfile.yml.
- P0-2: acknowledges old tokens remain valid, though the actual migration is still a TODO.
- P0-5: includes repo-wide grep for "usememos".
- P1-7: HandleOnboard now uses a separate key with a higher cap.

## Bottom line

**Three critical issues block safe implementation:**
1. INFRA-13 blocks a non-existent path → no-op.
2. P0-1 claim "rolled back" is false → user is in a worse state on partial failure.
3. P0-2 old-token migration is a comment, not code.

Plus the P0-3 Phase 2b struct vs protobuf reality, and the P1-8 unnamed call sites, should be clarified before handoff.

**Recommended path forward:** either (a) replace INFRA-13 with a code-level fix in `SetWorkspaceSetting` that explicitly rejects the instance-setting name, (b) wrap P0-1 in a DB transaction (or correct the verification claim), and (c) replace the P0-2 migration comment with `s.Secret` rotation (the standard pattern). After these three corrections and the P0-3/P1-8 clarifications, the plan is implementation-ready.
