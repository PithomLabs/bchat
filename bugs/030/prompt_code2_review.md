# Adversarial Code Review Prompt — v3 Remediation

**Review Target:** Security remediation implementation in bchat (code2_plan.md items #1–#15)  
**Reviewer Role:** Adversarial security auditor — your job is to find bugs, regressions, and missed findings  
**Working Tree:** All modified files listed below

---

## Review Instructions

You are performing an **adversarial code review** of a security remediation implementation. The implementation addresses 14 findings (3 Critical, 4 High, 5 Medium, 2 Low) from a previous round of 3 adversarial reviews. Your job is to verify every fix is correct, complete, and does not introduce regressions.

**Do NOT be polite.** Find every bug, every missed case, every regression, every new vulnerability. Be skeptical of every claim.

---

## What Was Changed (Items #1–#15)

### #1 — HandleSelectTenant SameSite=Lax (CRITICAL)
- **File:** `server/router/api/v1/auth_service.go:543`
- **Change:** Replaced `SameSite=None` (HTTPS) / `SameSite=Strict` (HTTP) with unconditional `SameSite=LaxMode`

### #2 — deleteBackingResourceFile containment assertion (CRITICAL)
- **File:** `server/router/api/v1/memo_resource_service.go:252-268`
- **Change:** Added `strings.HasPrefix(filepath.Clean(p), cleanDataDir)` check before `os.Remove`

### #3 — ApplyTenantFilter wiring for Memos (CRITICAL)
- **File:** `server/router/api/v1/memo_service.go:166-172`
- **Change:** After `GetTenantIDFromContext` returns nil, calls `deriveTenantIDsForScopedAdmin` and sets `find.TenantIDs`

### #4 — authGroup + TenantBindingMiddleware (HIGH)
- **File:** `server/router/api/v1/v1.go:281`
- **Change:** Added `authGroup.Use(TenantBindingMiddleware(s.Store))` to authGroup routes

### #6 — sanitizeFilename null-byte ordering (HIGH)
- **File:** `server/router/api/v1/resource_service.go:304-314`
- **Change:** Reversed order: strip null bytes first, then `filepath.Base`

### #7 — isSuperAdmin mirrors isSuperUser (HIGH)
- **File:** `server/router/api/v1/agent/handlers.go:2248-2274`
- **Change:** Inlined `isSuperUser` logic with sync comment; added exported `IsSuperUser` wrapper in `common.go:74-77`

### #8 — isSuperAdmin audit logging (MEDIUM)
- **File:** `server/router/api/v1/agent/handlers.go:2251-2270`
- **Change:** Added `slog.Debug`/`slog.Warn` calls for all failure/denial paths

### #9 — HandleDeleteTenant/Onboard use isSuperAdmin (MEDIUM)
- **File:** `server/router/api/v1/agent/handlers.go:1298-1299, 1659-1660`
- **Change:** Replaced `h.isAdmin(c)` with `h.isSuperAdmin(c)` on both handlers

### #10 — entrypoint.sh gosu fallback (MEDIUM)
- **File:** `scripts/entrypoint.sh:50-54`
- **Change:** Added `command -v gosu` check with fallback to `exec "$@"`

### #11 — Empty TenantIDs deny-all sentinel (MEDIUM)
- **File:** `server/router/api/v1/tenant_context.go:60-63`
- **Change:** Returns `[]int32{-1}` instead of `[]int32{}` when no valid tenants found

### #12 — Reindex error message genericity (MEDIUM)
- **File:** `server/router/api/v1/agent/handlers.go:1208-1220, 1249-1252`
- **Change:** `reindexHTTPError` returns generic messages; `HandleReindexStatus` also returns generic on error

### #13 — CSRF missing-Origin behavior (LOW)
- **File:** `server/router/api/v1/csrf.go:43-49`
- **Change:** Added `slog.Debug` audit log when both Origin and Sec-Fetch-Site are absent

### #14 — file_env missing file check (LOW)
- **File:** `scripts/entrypoint.sh:18-21`
- **Change:** Added `[ ! -f "$val_fileVar" ]` check with `exit 1`

### #15 — sanitizeFilename on read path (LOW)
- **File:** `server/router/api/v1/resource_service.go:412`
- **Change:** Added `sanitizeFilename(filepath.Base(resourcePath))` before containment assertion

---

## Review Scope

### Files Modified

| File | Items |
|------|-------|
| `server/router/api/v1/auth_service.go` | #1 |
| `server/router/api/v1/memo_resource_service.go` | #2 |
| `server/router/api/v1/memo_service.go` | #3 |
| `server/router/api/v1/v1.go` | #4 |
| `server/router/api/v1/resource_service.go` | #6, #15 |
| `server/router/api/v1/agent/handlers.go` | #7, #8, #9, #12 |
| `server/router/api/v1/common.go` | #7 |
| `server/router/api/v1/tenant_context.go` | #11 |
| `server/router/api/v1/csrf.go` | #13 |
| `scripts/entrypoint.sh` | #10, #14 |

### What to Verify for Each Item

| Item | Verify |
|------|--------|
| #1 | All `SameSite` assignments in `auth_service.go` are `SameSiteLaxMode`. No other cookie-setting paths use `None`. Widget compatibility preserved. |
| #2 | Containment assertion uses `filepath.Clean` + `os.PathSeparator`. Handles relative vs absolute paths correctly. Does not break when `Profile` is nil. |
| #3 | `deriveTenantIDsForScopedAdmin` is called with correct args. `TenantIDs` field is set on `FindMemo`. Does not double-filter when `TenantID` is already set. Works for gRPC context (not echo). |
| #4 | `TenantBindingMiddleware` on `authGroup` does not break routes without `:slug` param. Middleware skips correctly for no-slug routes. No double-application with `adminGroup`. |
| #6 | Null bytes stripped before `filepath.Base`. Does not break normal filenames. Edge cases: `""`, `"."`, `".."`, `"\x00"`. |
| #7 | `isSuperAdmin` logic matches `isSuperUser` exactly. No import cycle. `IsSuperUser` export is correct. |
| #8 | Audit logs include user_id, role, allowed_tenants. Log levels appropriate (Debug for normal flow, Warn for errors). No PII leakage. |
| #9 | `isSuperAdmin` correctly denies scoped admins on `HandleDeleteTenant`/`HandleOnboard`. Does not break global admin access. |
| #10 | `command -v gosu` check is POSIX-compatible. Fallback warning goes to stderr. Does not break when running as non-root. |
| #11 | `[]int32{-1}` sentinel never matches a real tenant ID. Filter functions handle it correctly. Does not break super user path (nil). |
| #12 | Generic messages do not leak error details. Error is still logged server-side. Does not break error-type switching for known errors. |
| #13 | Log includes method and path. Does not log sensitive data. Does not change behavior (still allows through). |
| #14 | `exit 1` on missing file. Error message goes to stderr. Does not break when `_FILE` var is unset. |
| #15 | `sanitizeFilename` on read path does not break when `resource.Reference` is empty or contains path separators. Containment assertion still works. |

---

## Specific Questions to Answer

1. **Does `deriveTenantIDsForScopedAdmin` work correctly when called from gRPC context (not echo context)?** The function takes `context.Context`, not `echo.Context`. Verify the store calls work in gRPC context.

2. **Does `TenantBindingMiddleware` on `authGroup` break routes without `:slug`?** The middleware checks `c.Param("slug")` — if empty, it skips. Verify no route is broken.

3. **Is the `[]int32{-1}` sentinel safe?** Could a tenant ever have ID -1? Check the auto-increment schema. Could the filter generate invalid SQL?

4. **Does the `sanitizeFilename` on read path change behavior for existing resources?** If `resource.Reference` already has a clean path, does re-sanitizing alter it?

5. **Does `isSuperAdmin` on `HandleDeleteTenant` break the admin flow?** Global admins should still be able to delete tenants. Scoped admins should be blocked.

6. **Does the entrypoint `command -v gosu` check work in alpine-based images?** `command` is a POSIX shell builtin, but verify it works in `/usr/bin/env sh`.

7. **Is there a race condition in `deriveTenantIDsForScopedAdmin`?** The GUID-to-ID lookup iterates `AllowedTenantIDs` sequentially. Could a tenant be deleted between lookup and query?

8. **Does `reindexHTTPError` still correctly differentiate error types?** The switch still checks `errors.Is` — verify the generic messages don't lose the ability to distinguish error types for monitoring.

9. **Does `memo_service.go` line 166 correctly avoid double-filtering?** If `GetTenantIDFromContext` returns non-nil AND the user is a scoped admin, does it skip the `TenantIDs` derivation?

10. **Does `deleteBackingResourceFile` handle the case where `resource.Reference` starts with `/` (absolute path)?** The containment assertion must not be bypassed by absolute paths.

---

## Output Format

For each finding, report:

```
### [SEVERITY] Finding Title

**File:** path/to/file.go:line
**Item:** #N
**Claimed Fix:** What the implementation claims
**Actual:** What the code actually does
**Verdict:** ✅ VERIFIED / ❌ NOT FIXED / ⚠️ PARTIAL / 🐛 REGRESSION
**Details:** Explanation
```

**Severity levels:** CRITICAL / HIGH / MEDIUM / LOW / INFO

---

## Final Checklist

- [ ] All 10 questions answered
- [ ] Every modified file reviewed
- [ ] No regressions introduced
- [ ] No new vulnerabilities found (or documented if found)
- [ ] Build still passes
- [ ] Tests still pass

---

## Constraints

- **Do not skip any file** — review every modified file
- **Do not assume correctness** — verify every change
- **Be adversarial** — find bugs, not confirm fixes
- **Check edge cases** — empty inputs, missing data, concurrent access, nil pointers
- **Check for import cycles** — verify no circular dependencies introduced
- **Check middleware ordering** — verify no bypass through ordering issues
