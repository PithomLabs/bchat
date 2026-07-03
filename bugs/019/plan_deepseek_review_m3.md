# Adversarial Plan Review — bugs/019/plan_deepseek.md

I verified all line-number claims and read the surrounding code. The plan is broadly accurate on the existence of vulnerabilities but has several flaws that will cause the coding agent to implement incorrect fixes. **Do not instruct the agent to implement until these are resolved.**

## Critical flaws (must fix)

### 1. P0-1: Wrong user ID — invalidates the WRONG user's tokens
Plan calls `s.deleteAllUserAccessTokens(ctx, currentUser.Id)` after a password change. But `UpdateUser` at `user_service.go:161` explicitly allows `currentUser.ID != userID` when `currentUser` is admin/host. **An admin changing a victim's password will wipe the admin's own tokens, not the victim's.** The plan must:
- Use the *target* user ID (the one in `request.User.Name` / `userID`), not `currentUser.Id`.
- Add a check that the caller has permission to do this (admin/host self-change is fine; otherwise no other user can change another's password).
- Log the actor (`currentUser.ID`) AND the target for audit.

### 2. P0-5: Half the proposed fix is already implemented
`server.go:212-231` (`getOrUpsertWorkspaceBasicSetting`) **already** auto-generates `SecretKey = uuid.NewString()` on first boot and upserts it. So:
- The "Additionally, add a startup migration..." paragraph is **dead work**.
- The `s.Secret = "usememos"` fallback in dev mode is **already dead code** under normal operation — after first boot, `workspaceBasicSetting.SecretKey` is the UUID, not empty. The proposed code's "dev mode → uses 'usememos'" verification step is **factually wrong**: it will use the UUID, not "usememos".
- The real residual risk is the prod path only. Simplify the fix to: in prod mode, after `getOrUpsertWorkspaceBasicSetting`, if `s.Secret == ""` (only possible if the upsert failed), return an error and abort startup.

### 3. P1-9: Dedup key is wrong — branch is unreachable
JWT tokens include `iat`, so every call to `GenerateAccessToken` produces a unique string. The plan's loop `if t.AccessToken == accessToken` will never match on a re-login. The "Update existing entry" branch is dead code; the "Append new entry" branch still runs every time, so the bug is **not actually fixed**.
- Correct fix: dedup on `description` (e.g., always-replace the `"user login"` entry) OR keep a max-N cap and evict oldest.
- This also needs a regression test that checks the DB row count after N sign-ins.

### 4. P0-4: Plan refuses to choose between Option A and Option B
The plan presents two valid options with a clear tradeoff and never picks one. The agent will guess. **Pick one.** Recommendation: Option A (fix parse error only) for safety in a customer-facing product, with the warning log.

### 5. INFRA-13: Nginx snippet is broken
The proposed config uses `internal;` inside a `location ~* ^/api/v1/(workspace|setting|instance)`. `internal;` only marks a location as callable from internal subrequests, it does NOT block external clients. External requests will still hit the location and the directive does nothing. The snippet also doesn't authenticate or return 403 — it just declares the location. The plan needs:
- A real auth check (e.g., `if ($http_cookie !~* "memos_access_token")` paired with an internal subrequest validator), OR
- A `deny all;` plus IP allowlist, OR
- Use `auth_request` to a backend validator.
- Note also: `"instance"` does not match any current route — the upstream CVE is about `/api/v1/workspace/*` settings. Confirm the regex matches the actual routes bchat serves.

## Significant gaps

### 6. P0-3: Frontend impact not verified
Plan says "Existing frontend token listing still works (it should use description + issued-at to identify tokens)" — this is unverified. The frontend likely uses the token value as the `name` for the DELETE call (see `DeleteUserAccessToken` at `user_service.go:486`, which matches on `request.AccessToken`). If yes, redacting breaks revocation. **Confirm against `web/src/` before implementing.**

### 7. P0-6: Fix is incomplete and the proposed code is misleading
The plan writes `// ... rest of function using %s, %s, %s` but the actual `generateWidgetScript` body (lines 1661+) is hundreds of lines and also references other JS strings (`createWidgetHTML`, `getWidgetStyles`, `sendMessage`, etc.). Even after the head is fixed, the rest of the function is emitted as a raw string literal — meaning any of those internal JS function bodies could contain unescaped backticks/backslashes that break parsing. Also, `json.Marshal` is overkill: use `strconv.Quote` (which is what `%q` already does) and then `+` the resulting `"foo"` directly into the single-quoted JS string, OR refactor the entire fallback to a `text/template` with proper context.

### 8. P0-2: 30-day cap is hardcoded magic number
No constant, no rationale. Extract `const MaxNeverExpireDuration = 30 * 24 * time.Hour` and reference. Also: existing tokens issued with the old 100-year logic are still valid — consider also lowering the cap on `GenerateAccessToken` so a one-off fix doesn't have to revisit this. And document the breaking change to clients that rely on the 100-year token.

### 9. P1-7: Rate-limit placement is underspecified
"After the permission check" varies per handler:
- `HandleOnboard` (creating a new tenant) — there's no `tenant.ID` yet at the auth check. Use a different key (e.g., `("global", "admin_mutation", clientIP)`) or skip rate-limit on first-tenant creation.
- `HandleDeleteTenant` — once you delete, the rate-limit entry leaks. Not critical but worth noting.
- `HandleImport` does heavy work — rate limit should be BEFORE the work, not after.

Plan should specify exact line per handler where the call goes (the listed `~line` numbers are off by 5-20 from actual: HandleDeleteTenant is 1575 not 1595; HandleImportSingleFile is 974 not 985; HandleReindexTenant is 1088 not 1099; etc.). Inaccurate line refs will cause the agent to insert in the wrong place.

### 10. INFRA-12: Health-check snippet is malformed and infra tradeoffs unstated
The `[[checks]]` block has no closing — missing `grace_period` value style and the `path` is given but no `protocol`. Fly expects `protocol = "http"` (or `"https"`) explicitly. Also: `min_machines_running = 1` + `cpu_kind = "performance"` will roughly double the monthly cost — the plan should note this and ask the user to confirm.

### 11. CVE-2026-6634 provenance
Multiple files in `bugs/018` and `bugs/019` cite this CVE, but CVEs are assigned by MITRE/CNA and `CVE-2026-6634` cannot have been assigned in 2026-07 (current date) since CVEs are not assigned that far in advance and the CVE database is the source of truth. **This may be a fabricated CVE ID.** Before implementing WAF rules, the agent should:
- Verify the CVE in MITRE's CVE database.
- If unverifiable, downgrade the WAF recommendation to a generic "block admin routes from public IPs" with explicit acknowledgment that the CVE reference is unverified.
- Note that the same bchat backend already requires `RoleHost` for `SetWorkspaceSetting` (per `bugs/018/prompt_sec_review.md:18`), so the actual exploitability at the API layer is limited.

## Minor issues

- **P0-1**: `s.deleteAllUserAccessTokens` is undefined in the plan; agent will create it from scratch. Consider having it call the existing `s.Store.GetUserAccessTokens` + `UpsertUserSetting` pattern shown at `user_service.go:480-501` (the `DeleteUserAccessToken` flow), not introduce a duplicate helper.
- **P0-2**: `AccessTokenDuration` is referenced as if it's defined; confirm its value and the gap to 30 days.
- **P0-3**: Sort key `int(i.IssuedAt.Seconds - j.IssuedAt.Seconds)` can overflow for distant timestamps. Not introduced by this change, but the agent should not refactor this.
- **P1-7**: Plan says "(tenantID, admin_mutation, clientIP)" but rate-limit service may already prepend `tenantID` internally (see `checkAdminMutationRateLimit` at handlers.go:2232, which passes `tenantID` as the first arg). Verify the actual `CheckRateLimit` signature to avoid double-namespacing.
- **P1-8**: Plan silently changes behavior from "log warning + fall back" to "log error + return empty". The LLM call site will then attempt to call OpenRouter with an empty `Authorization` header. Confirm the downstream returns a clean auth error rather than a 500 with a stack trace. Consider adding an explicit `if apiKey == ""` early return at the call site (`service.go:2004`, `2150`) so the user gets a clear "API key not configured" message.
- **Sprint order**: P0-5 (server.go) before INFRA-11 (rotate secrets) means after rotating, the prod deploy may not pick up the new key cleanly. Reverse the order, or at least require a restart + verification of the new key.
- **No rollback plan**: For each P0 fix, a one-line "if this breaks, revert via X" is missing. P0-4 and P0-2 especially need rollback notes since they change authenticated user-visible behavior.
- **Test plan runs `go test ./...` but the project uses `task test` per AGENTS.md**: confirm the actual test command from Taskfile.yml; `go test ./server/router/api/v1/agent/...` may not be the right entry point if there's a Taskfile alias.

## What's solid

- P0-1, P0-2, P0-3, P0-6, P1-10 line numbers are accurate (within rounding).
- Vulnerability statements for P0-1, P0-2, P0-3, P0-4, P0-6, P1-7, P1-8, P1-9, P1-10 are valid.
- The general structure (P0/P1/Infra tiers, sprint ordering) is sensible.

## Recommended next step

Resolve items 1-5 (critical) and 6, 7, 11 (significant) before handing off. Once those are corrected and the plan explicitly picks a side on P0-4 Option A/B, the coding agent can implement safely.
