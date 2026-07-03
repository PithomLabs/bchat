# Adversarial Review of `plan2.md`

The plan has improved substantially on the original, but several flaws remain that will cause incorrect implementations or security regressions. **Do not hand to coding agent until these are fixed.**

## Critical flaws (must fix before implementation)

### 1. P0-6: Escaping does NOT prevent XSS — `</script>` breaks out
The plan uses `strings.ReplaceAll(companyName, '\'', '\\\'')` then injects into a JS single-quoted string. **This is insufficient for values rendered inside a `<script>` block.** A `companyName` of `</script><img src=x onerror=alert(1)>` produces:
```js
var config = { companyName: '</script><img src=x onerror=alert(1)>', ... }
```
The browser ends the `<script>` at `</script>` and renders the `<img>` outside the JS context — the `'` escape is never reached. The test case shown (`'</script><script>alert(1)</script>'`) is also wrong: the example will be escaped to `\'</script><script>alert(1)</script>` which is still parsed as a script-end by the HTML parser before the JS engine sees the `\'`.

**Required fix:** also neutralize `</script>` (and ideally `</style>`, `<!--`, `<![CDATA[`) regardless of JS-string escaping. The cleanest approach is `json.Marshal` (which encodes `<` as `\u003c` and `>` as `\u003e`, both of which prevent the HTML parser from breaking out). The plan's original `json.Marshal` approach was correct — reverting to manual escaping was a regression. The `strconv.Quote` reference is also inconsistent with the code shown (which uses `strings.ReplaceAll`, not `strconv.Quote`).

### 2. P0-1: Silent failure of token invalidation is a security regression
Plan marks the post-update `deleteAllUserAccessTokens` call as "non-fatal" — if it errors, the password was changed and the API still returns 200. The caller believes the user is locked out; the user is not. This is worse than the current state (no invalidation attempted) only marginally, because the audit log gives false confidence. Options:
- Retry once, then fail-closed (return 500 so the caller knows the operation is incomplete).
- Make invalidation part of a DB transaction with the password update.
- At minimum, return a warning header (e.g., `X-TokenInvalidation-Partial: true`) so the frontend can show a banner.

The plan should pick one, not leave it as "best effort" with a log line.

### 3. P0-3: Still ambiguous after correction
Plan says "Default decision (to be confirmed by frontend audit): Redact the raw JWT." This is a contradiction: a default that's confirmed-by-audit isn't a default. The coding agent will pick one and the audit may not happen. **Make the audit a precondition**, not a confirmation:
- Step 1: grep frontend for `access_token` / `ListUserAccessTokens` usage.
- Step 2: classify as "needs raw" or "metadata-only."
- Step 3: implement based on the actual finding.

The current `DeleteUserAccessToken` at `user_service.go:486` matches on the token string, so the frontend almost certainly uses the raw value. The audit will likely reveal "needs raw" — the plan should anticipate that and propose an `id` field for revocation rather than defer the decision.

### 4. P1-8: Plan changes behavior at soft-fallback call sites
At `service.go:2006-2013` (classifyIntent) and `service.go:2152-2154` (chat handler), the code already has `if apiKey == ""` checks that return soft fallbacks ("unknown intent" / canned message). The plan's "Add early return: `return nil, fmt.Errorf(...)`" replaces soft fallback with hard error — this is a UX regression for tenants with broken keys (chat would 500 instead of returning a friendly message). The plan also misses **9 call sites** of `getLLMConfig` (lines 1233, 1247, 1262, 2005, 2151, 2614, 3900, 3930, 3961) — only 2 are addressed.

**Better fix:** make `getLLMConfig` itself return `(model, apiKey, error)`, or introduce a `requireLLMConfig` wrapper. The decryption-fail branch in `getLLMConfig` should propagate the error so callers can decide. The current call-site pattern can keep its soft fallback (logged + warning) while the new fail-closed behavior applies to the broken-tenant case.

## Significant gaps

### 5. P1-7: Line numbers still off, and the "HandleOnboard" rate limit bypasses tenant scoping
- HandleDeleteTenant: plan says `~1579`, actual permission check is at line 1581 (function starts at 1575).
- HandleGrantPermission: plan says `~2469`, actual at line 2469 (correct).
- HandleRevokePermission: plan says `~2538`, actual permission check at line 2537.
- HandleRestoreFileVersion: plan says `~867`, actual at line 867 (correct).
- HandleImportSingleFile: plan says `~985`, actual at line 985 (correct).
- HandleImport: plan says `~1331`, actual function at line 1327, admin check location needs verification.
- HandleReindexTenant: plan says `~1099`, actual at line 1099 (correct).
- HandleSetLLMConfig: plan says `~2320`, actual function at 2309 — line 2320 is mid-function, may be wrong.

The pattern is mostly correct now, but **the agent should grep for the actual `// Check admin role` / `// Check permission` comment above each insertion point** rather than rely on these ~line numbers. Several are still off by 5-15 lines.

For `HandleOnboard`, the plan uses `tenantID=0` as a global key. **This means every admin onboarding attempt shares one rate-limit bucket across all tenants** — a legitimate admin onboarding the second tenant of the day will get 429'd. Either:
- Scope by `clientIP + "onboard"` (already partially done via `clientIP`), or
- Cap the global onboard rate at a high number (e.g., 100/min) since this is a one-shot per-tenant operation.

### 6. P1-9: Eviction order is not guaranteed
`userAccessTokens = userAccessTokens[1:]` evicts `[0]`. `GetUserAccessTokens` (defined elsewhere, not verified) returns tokens in some order — likely by `AccessToken` string or insertion order, but **the plan does not verify or assert this**. If the list is alphabetical by token, this evicts a random entry. The plan should:
- Sort by issued-at (using the JWT `iat` claim, as the ListUserAccessTokens code does at `user_service.go:397-399`).
- Or assert the order returned by `GetUserAccessTokens` is insertion-order.
- Or explicitly cap the size using a `(keep only the 10 most recent by iat)` predicate.

Also: if a user already has 20 tokens pre-fix, the next sign-in drops to 11, not 10. Document the migration story (or run a one-off cleanup).

### 7. INFRA-13: New nginx config locks out legitimate admins
The config restricts to `fdaa::/48` (Fly 6PN) and RFC1918 ranges. **Admins connecting from a home/office IP will be blocked** from `SetWorkspaceSetting` and other workspace mutations. The plan says "this is defense-in-depth only" — but defense-in-depth that breaks legitimate functionality is a regression. Either:
- Add an `allow <admin_ip_cidr>;` directive that the user must configure, OR
- Scope the deny to only the unauthenticated `UpdateInstanceSetting` route (the actual CVE vector), not all workspace mutations.

The original CVE was about the **frontend** (`App.tsx`), not backend `SetWorkspaceSetting` which already requires `RoleHost`. So this nginx rule is locking down a path that's already protected.

### 8. INFRA-12: Health check protocol claim is wrong
Plan says `protocol = "http" WAS: missing` — but the original `fly.toml` has **no `[[checks]]` block at all**. "WAS: missing" implies the field existed. The accurate wording is "**newly added**." Also, Fly's `[[checks]]` uses `protocol` only when checking non-HTTP services; for HTTP, the default is `http`. Adding `protocol = "http"` is harmless but the comment is misleading. Also need to verify Fly accepts both `method = "GET"` and `path = "/healthz"` together — Fly checks have a `headers` field but the plan shows `[checks.headers]` empty, which should be removed.

### 9. Test plan still says "task test (or whatever the actual test alias is)"
This is the third time the plan has punted on test command. **Grep `Taskfile.yml` for the test target name and document it explicitly.** If it doesn't exist, the correct command is the `go test ./...` shown, but state that.

### 10. P0-2: Old 100-year tokens remain valid
Plan notes "Existing 100-year tokens remain valid until their original expiration" and offers a migration, but doesn't include the migration. With the same `secret` (now UUID, not "usememos") all previously-issued 100-year tokens are still cryptographically valid until their `exp`. The plan should at minimum include a one-off SQL/gocode snippet that:
- Lists all `AccessTokensUserSetting` entries,
- Parses each JWT's `exp` claim,
- Removes any whose `exp` is > 30 days from now.

This is non-trivial code (JWT parsing in migration logic), so the plan should call out whether the implementation agent should write it or whether the team will run a manual cleanup.

## Minor issues

- **P0-5 verification step 3** says "No reference to 'usememos' remains in `server.go`" — but the search should be repo-wide: `grep -r '"usememos"' --include="*.go" .` to catch test fixtures, comments, or any default-cookie generation paths.
- **P0-1** helper `deleteAllUserAccessTokens` is placed in `auth_service.go` (per Rollback note), but the new method doesn't reference auth_service-specific state. It could live in `user_service.go` alongside the existing `UpsertAccessTokenToStore` / `DeleteUserAccessToken` for cohesion. The plan should say which file.
- **P0-5** comment says "no more 'usememos' fallback, no dev-mode special case" — verify no test relies on `s.Secret == "usememos"` (e.g., a test fixture that signs tokens with the default).
- **Sprint 1 step 2** flags `~2x cost impact: confirm with user` but doesn't actually ask the user. This is the kind of thing that should be a pre-implementation question, not a sprint note.
- **P1-7** for `HandleImport` and `HandleImportSingleFile`, the rate-limit key is `tenant.ID`. A single admin can still upload 30 files/minute across multiple tenants, and 30/min/tenant is reasonable, but for a platform with many tenants, the `clientIP`-based limiter may be too strict for admins who manage multiple tenants.

## What's solid

- P0-1 user ID bug is correctly addressed.
- P0-5 simplification is correct; the dead-work paragraph is gone.
- P1-9 dedup-by-eviction is the right idea.
- P0-4 picks Option A clearly.
- CVE-2026-6634 is correctly downgraded to "unverifiable, defense-in-depth only."
- P0-2 has a named constant now.
- Rollback notes are present (small but useful).

## Bottom line

Three items must be fixed before the agent implements:
1. **P0-6** — current escaping does not stop `</script>` XSS. Use `json.Marshal` or add explicit `</script>` neutralization.
2. **P0-1** — non-fatal invalidation failure is a security regression. Pick a fail-closed or warning-propagation strategy.
3. **P0-3** — make the frontend audit a precondition, not a confirmation step.

Plus the P1-8 soft-fallback regression and the INFRA-13 admin-lockout should be addressed. After these, the plan is implementation-ready.
