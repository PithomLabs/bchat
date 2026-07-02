Here is the adversarial code review of the implemented changes.

---

## CRITICAL Finding — Rate Limiting Not Implemented on Admin Mutation Endpoints

**Severity:** HIGH

The plan explicitly required:
> "Dedicated rate-limit key: `(tenantID, 'admin_mutation', clientIP)` with RPM from `TenantConfig.AdminMutationRateLimitRPM`"

`service.go` includes `GetAdminMutationRateLimit()` and `CheckRateLimit()` exists, but **none of the four new admin mutation handlers call them**:
- `HandleCreateRoleTemplate`
- `HandleUpdateRoleTemplate`
- `HandleDeleteRoleTemplate`
- `HandleAssignRoleTemplate`

A `tenant:admin` user can flood these endpoints without throttling. This is a direct plan violation.

**Required fix:** Each mutation handler must call:
```go
rpm := h.service.GetAdminMutationRateLimit(ctx, tenant.ID)
allowed, err := h.service.CheckRateLimit(ctx, tenant.ID, "admin_mutation", clientIP, rpm)
if err != nil || !allowed { ... }
```

---

## HIGH Finding — System Template Contents Gated on Global Role, Not `tenant:admin` Permission

**Severity:** HIGH

In `HandleListRoleTemplates` (line 2550):
```go
isAdmin := h.isAdmin(c)  // checks HOST/ADMIN global role
```

The plan states:
> "System template contents visible to `tenant:admin` only"

But `isAdmin()` returns `true` only for global `HOST`/`ADMIN`. A user with `tenant:admin` permission but global `RoleUser` gets `isAdmin = false` and sees system template **names only**, not contents. This contradicts the plan and means tenant admins cannot inspect what permissions they are granting.

**Required fix:** Replace:
```go
isAdmin := h.isAdmin(c)
```
with:
```go
canViewTemplateContents := h.isAdmin(c) || h.hasPermission(c, tenant.ID, PermTenantAdmin)
```

---

## MEDIUM Finding — `HandleGrantPermission` Uses `0` as NULL Sentinel for `SourceTemplateID`

**Severity:** MEDIUM

At line 2481–2485:
```go
zero := int32(0)
existing, _ := h.store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{
    SourceTemplateID: &zero,
})
```

The SQLite layer interprets `*SourceTemplateID == 0` as `IS NULL`. This works because template IDs are auto-increment starting at `1`, but it couples business logic to an arbitrary sentinel. A future developer who creates a template fixture with ID `0` (possible in tests) would silently break explicit-grant targeting.

**Required fix:** Change the API contract. Either:
- Add an explicit `OnlyExplicitGrants` boolean to `FindUserTenantPermission`, or
- Use a typed constant `ExplicitGrantSentinel = -1` and document it.

---

## MEDIUM Finding — `HandleRevokePermission` Deletes All Rows Including Template Assignments

**Severity:** MEDIUM

`DeleteUserTenantPermission` executes:
```sql
DELETE FROM user_tenant_permission WHERE user_id = ? AND tenant_id = ?
```

This removes **all** rows for the user/tenant, including template-linked assignments. An admin revoking a user's explicit `chat:logs` grant also silently removes their `viewer` template assignment, with no audit trail or warning.

**Required fix:** Decide intent. If revoke means "remove all access," document it. If it means "remove explicit grants only," change to:
```sql
DELETE FROM user_tenant_permission WHERE user_id = ? AND tenant_id = ? AND source_template_id IS NULL
```

---

## LOW Finding — `HandleListUserRoles` Omits Template Identity from Response

**Severity:** LOW

The endpoint returns `[]ResolvedPermission{Permission, Source}`. `Source` is `"tenant_template"` but does not include the template ID or name. An admin auditing a user cannot determine which template(s) contributed permissions without cross-referencing the DB manually.

**Required fix:** Either extend `ResolvedPermission` with `TemplateID *int32` / `TemplateName string`, or include an `assignments` array in the response listing active template IDs.

---

## Test Coverage Gaps

| Gap | Risk |
|---|---|
| No test verifies system template **contents** are visible to a user with `tenant:admin` (only `tenant:read` is tested) | Visibility bug undetected |
| No test verifies rate limiting on mutation endpoints | DoS vector untested |
| No test verifies `HandleRevokePermission` preserves template assignments | Data loss risk |
| No test verifies `HandleGrantPermission` with **multiple** explicit-grant rows | Orphaned-row risk |

---

## Summary

| # | Finding | Severity | Blocking? |
|---|---------|----------|-----------|
| 1 | Rate limiting missing on 4 mutation endpoints | HIGH | **Yes — security regression from plan** |
| 2 | System template contents gated on global role, not `tenant:admin` perm | HIGH | **Yes — plan deviation** |
| 3 | `0` sentinel for `SourceTemplateID` NULL filter | MEDIUM | No — works but fragile |
| 4 | `DeleteUserTenantPermission` removes template assignments | MEDIUM | No — needs decision |
| 5 | `HandleListUserRoles` lacks template identity | LOW | No — UX gap |

**Recommended action:** Fix findings 1 and 2 before merging. Findings 3–5 can be tracked as follow-ups.
