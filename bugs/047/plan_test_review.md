# Adversarial Plan Review — plan_test.md (Test Failure Remediation)

**Reviewer:** Senior Go Architect (automated)
**Date:** 2026-07-23
**Scope:** 17 pre-existing test failures (16 bridge/role_template + 1 migration)

---

## Verdict: APPROVED (2 nits)

Root cause analysis is correct for both issues. Fix 1 (bridge middleware) is a genuine production bug. Fix 3 (dynamic version check) is sound. One minor line count inaccuracy and one missing impact note on middleware tests.

---

## Verified Correct

| Claim | Status | Evidence |
|-------|--------|----------|
| Root cause: bridge middleware missing tenant context | ✅ | `bridge_middleware.go:228` — `return next(c)` without `c.Set("tenant-id", tenant.ID)`. Tenant is available at line 53. |
| Root cause: handlers call `getTenantOrFail` | ✅ | `handlers.go:120` (HandleBridgeTakeover), `handlers.go:2771` (HandleListRoleTemplates) |
| Root cause: role template tests don't set tenant-id | ✅ | `role_template_handler_test.go` — `c.Set("user-id", ...)` on every subtest, zero have `c.Set("tenant-id", ...)` |
| Fix 1 location: `bridge_middleware.go:228` | ✅ | Line 228 is literally `return next(c)` — right after HMAC verification, nonce protection, last-used update |
| Fix 2: role template test context | ✅ | Each subtest creates its own echo context manually; `tenant` variable is in scope (returned from `setupRoleTemplateTestStore`) |
| Fix 3: `GetCurrentSchemaVersion()` exists | ✅ | `store/migrator.go:257` — returns max file version from embedded FS. Already called by `store/test/migrator_test.go:23` in `TestGetCurrentSchemaVersion` |
| 16 test all fail same error | ✅ | `gotest_fail.md` — all show `"tenant context not set"` |
| 1 migration test fails | ✅ | `gotest_fail.md` — hardcoded `"0.33.2"` no longer matches latest batch target |
| `bridgeGroup` uses `RequireBridgeHMAC` only, not `TenantBindingMiddleware` | ✅ | `v1.go:302-307` — bridgeGroup uses only `RequireBridgeHMAC` middleware |

---

## Nit 1: Fix 2 Line Count Is Understated (Line 128)

**Plan says:** `~3-5 lines` for `role_template_handler_test.go`

**Reality:** There are **10 subtests**, each with its own echo context and a `c.Set("user-id", ...)` call. Each needs an additional `c.Set("tenant-id", tenant.ID)`:

| Subtest | Line | Context Variable |
|---------|------|------------------|
| list templates includes seeded templates | 80 | adminUser |
| list templates hides permissions from non-admin | 112 | regularUser |
| create template | 138 | adminUser |
| assign template | 159 | adminUser |
| assign template idempotent | 180 | adminUser |
| list user roles | 199 | adminUser |
| tenant_admin_sees_system_template_contents | 232 | tenantAdmin |
| revoke_preserves_template_assignments | 284 | adminUser |
| list_user_roles_includes_template_identity | 331 | adminUser |
| grant_deduplicates_orphaned_explicit_rows | 391 | adminUser |

**Actual line count:** ~10 lines (one per subtest).

**Risk:** Low — fixing only the first subtest unblocks the panic at line 83, but the remaining 9 subtests would then fail individually with the same error when they run.

---

## Nit 2: No Mention of `bridge_middleware_test.go` Impact

**Plan omission:** Adding `c.Set("tenant-id", tenant.ID)` to `RequireBridgeHMAC` changes the middleware's observable side effect. The file `bridge_middleware_test.go` has **30+ unit tests** that test `RequireBridgeHMAC` directly. None of them currently expect `tenant-id` to be set. The plan doesn't discuss whether these tests need updating.

**Risk:** Very low — the tests validate HMAC verification outcomes (200/400/401 status codes), not context state. Adding a context key is unlikely to break existing assertions. But it should be noted in the plan for completeness.

**Recommendation:** Add a note: "No changes expected to `bridge_middleware_test.go` — existing tests validate HTTP response codes, not context state after successful verification."

---

## Summary

| Finding | Plan's Assessment | Actual | Action |
|---------|-------------------|--------|--------|
| Root cause A (bridge) | Correct | ✅ Confirmed | No change |
| Root cause B (migration) | Correct | ✅ Confirmed | No change |
| Fix 1 location | Correct | ✅ Confirmed | No change |
| Fix 2 approach | Correct | ✅ Confirmed | No change |
| Fix 2 line count | ~3-5 | **~10** | Update count |
| Fix 3 approach | Correct | ✅ Confirmed | No change |
| Middleware test impact | Not mentioned | Should be noted | Add note |

**Bottom line:** The plan is correct and ready for implementation. Only minor adjustments needed for the `role_template_handler_test.go` line count and a note about middleware test impact.
