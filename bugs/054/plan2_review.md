# Adversarial Plan Review: Bug 054 Plan 2 — HOST JWT tenant_id = nil

**Bug ID:** 054
**Reviewer:** Kilo (Senior Go Architect)
**Date:** 2026-07-31
**Verdict:** APPROVED WITH NITS — no critical or high-severity blockers remain. Three low/medium items are follow-up notes.

---

## Executive Summary

Plan 2 resolves every critical blocker from `plan.md`:

| Finding from plan.md | Status in plan2 |
|---------------------|-----------------|
| Migration schema error | ✅ Fixed via minimal downgrade |
| Scoped-admin tenant bypass | ✅ Fixed — uses `isSuperUser` check |
| HOST sign-in behavior change | ✅ Documented |
| Architectural inconsistency | ✅ Fixed |
| Fix 4 scope | ✅ Fixed |

**Current blockers:** None.  
**Nits:** 2 medium, 1 low.

---

## Finding 1: `doSignIn` HOST Behavior Is Slightly Inconsistent

**Severity:** MEDIUM (Nit)

`doSignIn` treats HOST in two different ways depending on whether permission rows exist:

| HOST case | Behavior |
|-----------|----------|
| Has explicit `user_tenant_permission` rows | Uses USER-like logic: auto-selects if 1, errors if >1 |
| Has **no** permission rows | Auto-selects first tenant from `ListAgentTenants` |

So HOST with 0 perm rows is treated differently from HOST with 1 perm row. This is defensible, but it is a **behavior inconsistency**: for the same role, the same absence of selection leads to either auto-selection or error.

**Recommendation:** Call this out explicitly in the plan as intentional, with rationale:

> HOST without explicit permission rows is treated as a super-user with implicit access to the first tenant. HOST with explicit permission rows must select or is auto-selected by count. This preserves backward compatibility for both managed and unmanaged HOST setups.

---

## Finding 2: Wrong helper reference

**Severity:** LOW (Nit)

Plan2 references `isSuperUser` at `tenant_binding.go:80`, but the helper actually lives at `common.go:68`:

```go
// common.go:68
func isSuperUser(user *store.User) bool {
    return store.IsSuperUser(user)
}
```

`tenant_binding.go:37` uses inline logic instead:

```go
if user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0) {
```

**Recommendation:** Update the plan's references to `common.go:68` or note that `tenant_binding.go` uses inline logic equivalent to `isSuperUser`.

---

## Finding 3: `HandleAuthTenants` With Empty Tenant System

**Severity:** LOW (Nit)

If the system has zero tenants, the new HOST path returns 200 with an empty `tenants: []` list instead of an error. The original path for regular users with no perms returned 403.

This is actually **better** behavior for HOST, but the plan doesn't mention it. Worth noting as an intentional side effect:

```go
// Plan2 says "no tenants exist, tenantID stays nil — acceptable for HOST"
// However, for HandleAuthTenants, the same situation returns HTTP 200 with empty list.
```

**Recommendation:** Add one sentence noting that `HandleAuthTenants` returns `200 []` for HOST when no tenants exist, instead of erroring.

---

## Detailed Check: Behavioral Correctness

### HOST gRPC sign-in flow

**Current:** `doSignIn` is called with `nil` `tenantID`. For `RoleUser`, it resolves from perms. For `RoleHost`, it skips entirely → JWT has `nil` `tenant_id`.

**After Fix 2:** HOST/ADMIN enter the new block. If they have perm rows, same as `RoleUser`. If they don't, `ListAgentTenants` is called and the first tenant is auto-selected.

**Edge case — no tenants at all:** `requestedTenantID` stays `nil`. `GenerateAccessToken` is called with `nil`. This matches the pre-fix behavior, so no regression.

**Edge case — HOST with multiple perm rows:** Returns the same `"multiple tenants"` gRPC status. This matches the USER behavior and is consistent.

**Edge case — HOST gRPC re-login after explicit `select-tenant`:** `doSignIn` is called with whatever `tenantID` was selected. The new block only modifies `tenantID` when it is `nil`. Explicit selection is preserved.

All HOST paths are correct.

### `HandleAuthTenants` flow

**Current:** Queries `user_tenant_permission`. HOST has 0 rows → 403.

**After Fix 1:** Super-user check returns all tenants. Scoped-admin check still queries perms.

**Edge case — HOST with no tenants:** `ListAgentTenants` returns `[]`. The handler builds an empty `tenants` slice and returns it as JSON. This is a 200 response, not an error. This is arguably correct: HOST is not "not associated", the system is simply empty.

**Edge case — HOST with 1 tenant:** Returns list of 1 tenant. Frontend can present it or the JWT auto-selection in Fix 2 handles it.

**Edge case — Scoped admin:** Uses existing `ListUserTenantPermissions` path. No change.

All auth flows are correct.

---

## Final Verdict

**APPROVED WITH NITS.** Plan2 correctly addresses all critical blockers from `plan.md`. No implementation changes are required before execution. The three nits above are clarifications, not blockers.
