# Adversarial Code Review Prompt

**Review Target:** Security remediation implementation in bchat (`bugs/030/code.md`)
**Reviewer Role:** Adversarial security auditor — your job is to find bugs, regressions, and missed findings
**Working Tree:** Full diff of all modified files (see `code.md` for file list)

---

## Review Instructions

You are performing an **adversarial code review** of a security remediation implementation. The implementation addresses Critical and High severity findings from a previous security audit. Your job is to verify every fix is correct, complete, and does not introduce regressions.

**Do NOT be polite.** Find every bug, every missed case, every regression, every new vulnerability. Be skeptical of every claim in `code.md`.

---

## Review Scope

### Phase 0 — Emergency (H7, C2, C3)

| Finding | Claimed Fix | What to Verify |
|---------|-------------|----------------|
| H7 | Credentials replaced with placeholders | Check `.env` and `bugs/026/s3_probe/.env` contain no live keys. Check rotation instructions are clear. |
| C2 | pprof gated behind `profile.IsDev()` or env flag | Verify pprof routes are not registered in prod. Check the env flag name matches exactly. Check for other debug endpoints that might be exposed. |
| C3 | Debug mode + error handler conditional on IsDev | Verify `echoServer.Debug` is only true in dev. Verify prod error handler returns no internal details (stack traces, file paths, line numbers). |

---

### Phase 1 — Tenant Isolation (H2, H1, C1)

| Finding | Claimed Fix | What to Verify |
|---------|-------------|----------------|
| H2 Part A | `isSuperUser` fixed — only RoleHost or unscoped RoleAdmin | Verify the definition. Check if any call site passes a non-Host user to `isSuperUser` and expects super access. |
| H2 Part B | Tenant filter derived from `AllowedTenantIDs` | Verify `deriveTenantIDsForScopedAdmin` handles GUID lookup failures gracefully. Check that the `TenantIDs` field actually triggers SQL filtering in SQLite/Postgres drivers. |
| H1 | `TenantBindingMiddleware` enforces for all roles | Verify RoleUser path — does it actually deny access? Verify the `ListUserTenantPermissions` call works. Check edge cases: user with no permissions, user with permissions for different tenants. |
| C1 | `isSuperAdmin` + 9 handlers guarded | Verify each handler returns 403 for unauthorized access. Check that `isSuperAdmin` is consistent with `isSuperUser`. Verify no handler was missed. |

---

### Phase 2 — Hardening (H3, H4, H5, H6)

| Finding | Claimed Fix | What to Verify |
|---------|-------------|----------------|
| H3 | Filename sanitization + path traversal | Test `sanitizeFilename` with: `../../../etc/passwd`, null bytes, `.` , `..`, empty string, Windows paths. Verify containment assertion on both save and read paths. |
| H4 | Cookie SameSite=Lax + CSRF middleware | Verify cookie change. Check CSRF middleware: does it skip safe methods correctly? Does Bearer bypass work? Does Sec-Fetch-Site check work? Check for double-submit issues. |
| H5 | Non-root container + gosu | Verify Dockerfiles have no `USER` directive. Check entrypoint.sh preserves `_FILE` logic. Verify gosu is installed. |
| H6 | URL validation + IP-pinned dialer | Test URL validation with: `http://169.254.169.254`, `http://[::1]`, `javascript:`, `file:///etc/passwd`, empty string. Check IP-pinned dialer for TOCTOU issues. Check redirect following (does it re-resolve DNS?). |

---

## Specific Questions to Answer

1. **Is `isSuperAdmin` consistent with `isSuperUser`?** If a user is super in one, are they super in both?
2. **Does the `TenantIDs` SQL filter work for all databases?** MySQL was skipped — is this safe?
3. **Is there any path where a scoped admin can access a tenant they shouldn't?** Check the full request flow: middleware → handler → store query.
4. **Can an attacker bypass CSRF by using a different Content-Type?** Check if the middleware handles multipart form data correctly.
5. **Does the IP-pinned dialer follow redirects?** If a webhook endpoint redirects to an internal IP, does the second request use the same validation?
6. **Is there any information leakage in error messages?** Check all error responses for file paths, stack traces, database details.
7. **Are there any new race conditions?** Check the GUID lookup in `deriveTenantIDsForScopedAdmin`.
8. **Does the entrypoint handle the case where `gosu` is not installed?** (e.g., in dev mode)
9. **Are there any existing tests that now fail?** Run the full test suite and check for regressions.
10. **Is there any code path that bypasses the new guards?** Check for middleware ordering issues.

---

## Output Format

For each finding, report:

```
### [SEVERITY] Finding Title

**File:** path/to/file.go:line
**Claimed Fix:** What `code.md` says
**Actual:** What the code actually does
**Verdict:** ✅ VERIFIED / ❌ NOT FIXED / ⚠️ PARTIAL / 🐛 REGRESSION
**Details:** Explanation
```

**Severity levels:** CRITICAL / HIGH / MEDIUM / LOW / INFO

---

## Final Checklist

- [ ] All 10 questions answered
- [ ] Every file in `code.md` reviewed
- [ ] No regressions introduced
- [ ] No new vulnerabilities found (or documented if found)
- [ ] Build still passes
- [ ] Tests still pass

---

## Critical Files to Review

```
server/server.go
server/router/api/v1/common.go
server/router/api/v1/tenant_binding.go
server/router/api/v1/tenant_context.go
server/router/api/v1/csrf.go
server/router/api/v1/auth_service.go
server/router/api/v1/v1.go
server/router/api/v1/resource_service.go
server/router/api/v1/webhook_service.go
server/router/api/v1/agent/handlers.go
plugin/webhook/webhook.go
store/agent.go
store/ticket.go
store/memo.go
store/db/sqlite/agent.go
store/db/sqlite/ticket.go
store/db/sqlite/memo.go
store/db/postgres/agent.go
store/db/postgres/ticket.go
store/db/postgres/memo.go
Dockerfile.fly
Dockerfile.s3.fly
scripts/entrypoint.sh
.env
bugs/026/s3_probe/.env
```

---

## Constraints

- **Do not skip any file** — review every modified file
- **Do not assume correctness** — verify every claim in `code.md`
- **Be adversarial** — your job is to find bugs, not confirm fixes
- **Report regressions** — if a fix breaks existing functionality, flag it
- **Check edge cases** — what happens with empty inputs, missing data, concurrent access?
