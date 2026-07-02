The implementation is close to complete. Most prior findings are resolved. I found **2 remaining gaps** and **1 test-coverage weakness**.

---

## HIGH — `HandleListUserRoles` still omits template identity

**Location:** `handlers.go:2896`

The response returns:
```go
"permissions": resolved,  // []ResolvedPermission{Permission, Source}
```

`Source` is `"tenant_template"` but there is no `TemplateID` or `TemplateName`. An admin auditing a user cannot determine which template(s) contributed permissions without manual DB queries.

**Required fix:** Include template identity in the response, e.g.:
```go
"permissions": resolved,
"assignments": []TemplateAssignment{{ID: templateID, Name: templateName}}
```
Or extend `ResolvedPermission` with `TemplateID *int32` and `TemplateName string`.

---

## MEDIUM — `HandleGrantPermission` accumulates orphaned explicit rows

**Location:** `handlers.go:2498-2511`

```go
existing, _ := h.store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{
    SourceTemplateID: intPtr(ExplicitGrantSourceTemplate),
})
if len(existing) > 0 {
    perm.ID = existing[0].ID  // updates only the newest
    _, err = h.store.UpdateUserTenantPermission(ctx, perm)
} else {
    _, err = h.store.CreateUserTenantPermission(ctx, perm)
}
```

If multiple explicit-grant rows already exist (pre-existing data or prior bug), this updates only the newest and leaves older rows orphaned. The resolver unions them, so permissions are correct, but the table accumulates duplicates.

**Required fix:** After finding existing explicit rows, delete any other explicit rows for the same `(user_id, tenant_id)` before updating:
```go
if len(existing) > 1 {
    // dedupe: keep only the newest explicit row
    h.store.DeleteUserTenantPermission(ctx, req.UserID, existing[1].ID) // or bulk delete by IDs
}
```

---

## LOW — `DeleteUserTenantPermission` remains a footgun

**Location:** `store/db/sqlite/rbac.go:134`

`DeleteUserTenantPermission` still deletes **all** rows (`WHERE user_id = ? AND tenant_id = ?`), including template assignments. `HandleRevokePermission` was migrated to `DeleteExplicitUserTenantPermissions`, but the old method remains public. Any future caller or test inadvertently using it will wipe template assignments.

**Required fix:** Either deprecate `DeleteUserTenantPermission` with a code comment warning, or rename it to `DeleteAllUserTenantPermissions` to make the blast radius explicit.

---

## Test Coverage Gaps

| Gap | Risk |
|---|---|
| No test for multiple explicit-grant rows and deduplication on grant | Orphaned-row accumulation |
| No test for `HandleListUserRoles` template identity | Missing template metadata in response |
| No test verifying `DeleteUserTenantPermission` is unused in prod paths | Footgun remains undocumented |

---

## Summary

| # | Finding | Severity | Status |
|---|---------|----------|--------|
| 1 | `HandleListUserRoles` lacks template identity | HIGH | Open |
| 2 | `HandleGrantPermission` orphaned explicit rows | MEDIUM | Open |
| 3 | `DeleteUserTenantPermission` footgun | LOW | Open |
| 4 | Rate limiting wired to all 4 mutations | — | **Closed** |
| 5 | System template contents gated on `tenant:admin` | — | **Closed** |
| 6 | Explicit-grant NULL filter uses typed constant | — | **Closed** |
| 7 | Revoke preserves template assignments | — | **Closed** |
| 8 | Tests for visibility and revoke behavior | — | **Closed** |

**Recommendation:** Fix findings 1 and 2 before merge. Finding 3 can be a follow-up deprecation.
