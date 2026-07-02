# Adversarial Review: `plan_sec.md` (Security-Hardened v6)

## Overall Assessment

The v6 plan resolves every P0/P1 blocker from v5 and is **nearly implementation-ready**. It correctly identifies that `GetUserTenantPermission` returns only the first row and routes the resolver through `ListUserTenantPermissions`, adds the missing `SourceTemplateID` filter to `FindUserTenantPermission`, and defines backward-compatible `HandleListPermissions` output. The `tenant_id = -1` sentinel, pre-migration assertion, and `CHECK` constraint are all sound.

I found **2 residual design gaps** and **1 implementation footgun** remaining. None are P0, but all will surface during implementation if not addressed.

---

## Finding 1 — Multiple explicit-grant rows per user/tenant are possible with no guard
**Severity:** MEDIUM

`HandleGrantPermission` now targets `WHERE source_template_id IS NULL` rows. But if two explicit grants already exist for the same `(user_id, tenant_id)` — possible if data was inserted before this plan, or via direct DB access — `HandleGrantPermission` updates only one row (the first returned by `ListUserTenantPermissions` ordering). The other explicit rows become orphaned, and `ResolveEffectivePermissions` unions them all, returning duplicate permissions.

More importantly, the plan does not add a uniqueness or deduplication guard for explicit grants. A future admin could inadvertently create multiple explicit-grant rows.

**Fix:** Add a `CHECK` or application-level assertion that at most one `UserTenantPermission` row exists per `(user_id, tenant_id)` with `source_template_id IS NULL`. Or, in `HandleGrantPermission`, after checking for an existing explicit row, delete any other explicit rows for the same user/tenant before updating.

---

## Finding 2 — Rate-limiting handler must fetch `TenantConfig` per mutation
**Severity:** LOW-MEDIUM

The plan places `AdminMutationRateLimitRPM` on `TenantConfig`. The existing `CheckRateLimit` helper takes `rpm int` directly. Each new admin-mutation handler must therefore query `TenantConfig` before calling `CheckRateLimit`. This is not a security flaw, but it is an extra DB round-trip per mutation that the plan does not account for.

If the implementation instead hardcodes the RPM or reads it once at startup, it becomes stale when `TenantConfig` is updated.

**Fix:** Add a `GetAdminMutationRateLimit(ctx, tenantID int32) int` helper to `Service` that reads `TenantConfig.AdminMutationRateLimitRPM` with env fallback, and use it in the three mutation handlers.

---

## Finding 3 — `ValidatePermissions` "rejects wildcard-only" conflates `["*"]` with `["*", "tenant:admin"]`
**Severity:** LOW

The plan says:
> "reject wildcard-only templates (`["*"]` alone)"

If the validation checks `len(permissions) == 1 && permissions[0] == "*"`, a template with `["*", "tenant:admin"]` is accepted. This is correct behavior — the wildcard makes the extra permission redundant but harmless. However, the plan's wording suggests any template containing `*` is rejected, which would break legitimate templates that include `*` alongside other permissions.

**Fix:** Clarify that only templates whose **sole** permission is `*` are rejected. Templates with `*` plus additional permissions are accepted (the extra permissions are no-ops but harmless).

---

## Finding 4 — Client-facing API uniqueness for template assignments
**Severity:** LOW

The plan defines idempotency at the DB row level: "query `SELECT id FROM user_tenant_permissions WHERE user_id = ? AND tenant_id = ? AND source_template_id = ?`". But the API endpoint is `POST /role-templates/:id/assign`. If the same template is assigned, unassigned, and re-assigned, the original row's `SourceTemplateID` may have been cleared or the row deleted. The idempotency check does not distinguish between:
- "already assigned" → return 200
- "previously assigned, then unassigned" → create new row

This is actually correct behavior, but the API response should include whether the assignment was newly created or already existed, for audit purposes.

---

## Summary of v5→v6 Resolution

| v5 Finding | v6 Resolution |
|------------|---------------|
| `GetUserTenantPermission` truncates resolver | Resolver uses `ListUserTenantPermissions` explicitly |
| `FindUserTenantPermission` missing `SourceTemplateID` | Added; SQLite/Postgres query builders updated |
| `HandleGrantPermission` corruption | Targets `source_template_id IS NULL`; creates new row if none exists |
| `AdminMutationRateLimitRPM` location | Definitive: `TenantConfig` default `30` |
| `ResolvedPermission` location | `permissions.go`; handlers convert to DTOs |
| `HandleListPermissions` breaking change | Backward-compatible: keep `permissions []string`, add `permissions_with_source []ResolvedPermission` |
| Sentinel `-1` collision | Pre-migration assertion + `CHECK` |
| Multiple explicit grants | **Open** — no guard against multiple explicit-grant rows per user/tenant |

---

## Recommendation

The plan is **close to implementation-ready**. The open items are all P2 or lower. If you want to proceed now, the implementation agent should address the multiple-explicit-grant edge case inline. If you want one more revision to close it formally, add a uniqueness constraint or dedup logic in `HandleGrantPermission`.
