# Adversarial Review: `plan_sec5.md` (Security-Hardened v4 / Minimum Viable v5)

## Overall Assessment

The v4/v5 plan is substantially more mature than prior drafts. It closes the major P0 blockers from v1–v3: separate assignment rows, idempotent assignment, `SourceTemplateID`-aware `HandleGrantPermission`, sentinel `tenant_id = -1`, and dedicated rate-limit key. However, the "minimum viable" distillation introduces **2 P0 plumbing mismatches** with the existing store layer, **1 P1 unresolved configuration decision**, and **2 P2 API compatibility risks** that will surface at implementation time.

---

## P0 — Implementation-Breaking Issues

### Finding 1 — `GetUserTenantPermission` returns only the first row; resolver needs all rows
**Severity:** HIGH

`store/db/sqlite/rbac.go` lines 35–44:
```go
func (d *DB) GetUserTenantPermission(...) (*store.UserTenantPermission, error) {
    perms, err := d.ListUserTenantPermissions(ctx, find)
    ...
    return perms[0], nil
}
```

The plan requires `ResolveEffectivePermissions` to query **ALL** `UserTenantPermission` rows for `(user_id, tenant_id)` and union them. But `GetUserTenantPermission` always returns `perms[0]` — the most recent row by `granted_at DESC`. If the resolver calls `GetUserTenantPermission`, it silently drops explicit grants when a newer template assignment exists.

**Attack scenario:**
1. Admin assigns `viewer` template to User X (`source_template_id=1`, `granted_at=100`).
2. Admin explicitly grants `chat:logs` to User X (`source_template_id=NULL`, `granted_at=200`).
3. `GetUserTenantPermission` returns the explicit grant row (newer). User gets `chat:logs`. ✓
4. Admin assigns `editor` template to User X (`source_template_id=2`, `granted_at=300`).
5. `GetUserTenantPermission` now returns the `editor` template row. User loses `chat:logs`. ✗

**Fix required:** The resolver must call `ListUserTenantPermissions`, not `GetUserTenantPermission`. Alternatively, add a new store method `GetAllUserTenantPermissions(ctx, userID, tenantID int32) ([]*UserTenantPermission, error)` that returns all rows without the first-row truncation.

---

### Finding 2 — `FindUserTenantPermission` has no `SourceTemplateID` filter; `HandleGrantPermission` cannot target explicit rows
**Severity:** HIGH

The plan states:
> "`HandleGrantPermission` MUST query `WHERE source_template_id IS NULL` before updating."

But `FindUserTenantPermission` currently has:
```go
type FindUserTenantPermission struct {
    ID       *int32
    UserID   *int32
    TenantID *int32
}
```

There is no `SourceTemplateID` field. The SQLite/Postgres `ListUserTenantPermissions` methods build `WHERE` clauses dynamically from this struct — they cannot filter on `source_template_id` without a code change.

**Attack scenario:** Implementation adds `WHERE user_id = ? AND tenant_id = ?` to `HandleGrantPermission` but forgets `AND source_template_id IS NULL`. The `GetUserTenantPermission` call returns the most recent row (likely a template assignment). The update overwrites it, setting `source_template_id = NULL`. The template assignment is corrupted; the deletion guard is bypassed.

**Fix required:** Add `SourceTemplateID *int32` to `FindUserTenantPermission` and update both SQLite and Postgres `ListUserTenantPermissions` to filter on it.

---

## P1 — Design Decisions Still Open

### Finding 3 — `AdminMutationRateLimitRPM` location is unresolved
**Severity:** MEDIUM

The plan says:
> "Add `AdminMutationRateLimitRPM` field to `TenantConfig` with safe default (`30`)"
> "Note: Confirm whether `RateLimitRPM` lives on `AgentAudience` or `TenantConfig`."

`TenantConfig` and `AgentAudience` are different tables. `RateLimitRPM` currently lives on `AgentAudience`. Adding the new field to `TenantConfig` means:
- Two separate tables hold rate-limit config for the same tenant
- The handler must query `TenantConfig` for admin mutations but `AgentAudience` for chat traffic
- Future developers will conflate them

**Fix required:** Decide definitively. If admin mutations are tenant-wide policy, `TenantConfig` is correct. If they're audience-specific, add to `AgentAudience`. Document the choice in the plan and don't leave it as an "open decision."

---

### Finding 4 — `ResolvedPermission` struct location is inconsistent
**Severity:** MEDIUM

- Step 1 (`store/rbac.go`): "Add `ResolvedPermission` struct"
- Step 4 (`permissions.go`): "Add `ResolvedPermission` struct"

If defined in `store/rbac.go`, it must not import `server/router/api/v1/agent` (circular). If defined in `permissions.go`, it cannot be used in `store` package response types. The plan should place it in `permissions.go` (application layer) and have handlers convert it to response DTOs.

---

## P2 — Residual Risks & API Compatibility

### Finding 5 — `HandleListPermissions` response format is a breaking API change
**Severity:** LOW-MEDIUM

The plan says `HandleListPermissions` returns `ResolvedPermission` with source metadata. The current response is:
```go
type UserPermissionResponse struct {
    Permissions []string `json:"permissions"`
}
```

If `Permissions` becomes `[]ResolvedPermission`, existing frontend code that iterates `permissions` as strings breaks. The plan should either:
- Add a new field `permissions_with_source []ResolvedPermission` and keep `permissions []string` for backward compatibility, or
- Document the breaking change and update the frontend accordingly.

---

### Finding 6 — `admin_mutation` `AudienceType` may violate `AgentRateLimit` schema assumptions
**Severity:** LOW

`AgentRateLimit` has `AudienceType string`. If the schema or any check constraint limits values to `"external"` / `"internal"`, inserting `"admin_mutation"` will fail. The plan does not verify the schema allows arbitrary `AudienceType` values.

**Fix required:** Verify `AgentRateLimit.AudienceType` has no CHECK constraint before using `"admin_mutation"`.

---

### Finding 7 — `tenant_id = -1` sentinel may conflict with test fixtures or legacy data
**Severity:** LOW

The plan assumes tenant IDs are auto-increment starting at `1`. But test fixtures, backfills, or manual inserts could create a tenant with `id = -1` (unlikely but possible in tests). The `CHECK (tenant_id = -1 OR tenant_id >= 1)` prevents new inserts, but existing bad data would cause migration failure.

**Fix required:** Add a pre-migration assertion: `SELECT COUNT(*) FROM agent_tenant WHERE id <= 0` must return `0`.

---

## Positive Changes From v3

| Area | v3 Gap | v5 Resolution |
|------|--------|---------------|
| Template row merge | Open | Fixed: separate rows, no merge |
| `HandleGrantPermission` corruption | Open | Fixed: `WHERE source_template_id IS NULL` |
| Assignment idempotency | Open | Fixed: pre-insert check |
| System template NULL uniqueness | Partial | Fixed: sentinel `-1` + `CHECK` |
| `containsResolvedPermission` | Undefined | Defined |
| `AdminMutationRateLimitRPM` | No plumbing | Added to `TenantConfig` |
| Global ADMIN + template | Unclear | Documented: allowed, auditable |

---

## Prioritized Actions Before Implementation

| Priority | Finding | Action |
|----------|---------|--------|
| P0 | `GetUserTenantPermission` truncates to first row | Resolver must use `ListUserTenantPermissions`; add new store method if needed |
| P0 | `FindUserTenantPermission` missing `SourceTemplateID` filter | Add field and update SQLite/Postgres query builders |
| P1 | `AdminMutationRateLimitRPM` location unresolved | Choose `TenantConfig` vs `AgentAudience` definitively |
| P1 | `ResolvedPermission` struct location inconsistent | Place in `permissions.go`; import-safe |
| P2 | `HandleListPermissions` breaking API change | Add backward-compatible response field or document breaking change |
| P2 | `AgentRateLimit` schema may reject `"admin_mutation"` | Verify schema allows arbitrary `AudienceType` |
| P2 | `tenant_id = -1` pre-migration data check | Add assertion in migration script |

---

## Conclusion

The v5 "minimum viable plan" is close to implementation-ready. The **critical blocker** is that the existing store layer does not support the `source_template_id IS NULL` filter required by `HandleGrantPermission`, and `GetUserTenantPermission` returns only the first row, which breaks the multi-row resolver invariant. These are straightforward store-layer changes but must be reflected in the plan before implementation begins. The rate-limit location decision should also be finalized to avoid mid-implementation rework.
