# Adversarial Review: `plan_sec3.md` (Security-Hardened v2)

## Overall Assessment

The v2 plan materially improves on v1: it introduces `SourceTemplateID` for live-reference guardrails, fixes the ADMIN wildcard semantics, wires `hasPermission()` through a single resolver, and adds SQLite FK pragma enforcement. These close most P0/P1 issues from the first review.

However, I found **3 new design gaps** and **3 residual weaknesses** that remain exploitable or incomplete.

---

## New Findings

### Finding 1 — Merged template permissions destroy `SourceTemplateID` provenance
**Severity:** HIGH
**Location:** Step 5 (`HandleAssignRoleTemplate`)

The plan states:
> "Merges template permissions with existing explicit grants (deduped)"

But `SourceTemplateID` is meant to block template deletion while users are assigned to it. If permissions are merged into a single `UserTenantPermission` row, `SourceTemplateID` can only reflect one source:
- Set to template ID: explicit grants become falsely protected
- Set to `nil`: template grants become orphans, deletion guard is bypassed

**Attack scenario:**
1. Admin assigns template `template_A` (`["tenant:read"]`) to User X. Row created: `{user_id: X, permissions: ["tenant:read"], source_template_id: 1}`.
2. Admin explicitly grants `chat:logs` to User X. Plan "merges": row updated to `{permissions: ["tenant:read", "chat:logs"], source_template_id: 1}`.
3. Admin deletes `template_A`. Deletion is blocked because `SourceTemplateID=1` exists.
4. If admin changes plan to "delete anyway", User X loses explicit `chat:logs` too.

Conversely, if `SourceTemplateID` is set to `nil` after merge, the deletion guard is bypassed: User X keeps `tenant:read` from a deleted template with no trace.

**Fix required:** Do NOT merge permissions into existing rows. Create a new `UserTenantPermission` row per template assignment with `SourceTemplateID = templateID`. Keep existing explicit grants in a separate row. Update `ContainsPermission`/resolution to union across rows.

---

### Finding 2 — `HandleGrantPermission` or direct `ListUserTenantPermissions` still bypasses the resolver
**Severity:** HIGH
**Location:** `handlers.go` line 2435; `HandleGrantPermission` line 2503

The plan wires `hasPermission()` through `ResolveEffectivePermissions`. But `HandleListPermissions` currently does:
```go
perms, err := h.store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{TenantID: &tenant.ID})
```
and returns `p.Permissions` directly. This bypasses:
- `tenant:admin` expansion
- `permission_source` tracking
- Any future logic in `ResolveEffectivePermissions`

Additionally, `HandleGrantPermission` creates `UserTenantPermission` with `SourceTemplateID` left as `nil`, which is correct for explicit grants. But if `HandleAssignRoleTemplate` creates separate rows, the `ListUserTenantPermissions` response now contains multiple rows per user/tenant. The existing `HandleListPermissions` response format flattens them without deduplication.

**Fix required:** Audit **all** direct `GetUserTenantPermission`/`ListUserTenantPermissions` calls. Update `HandleListPermissions` to resolve effective permissions per-user via `ResolveEffectivePermissions`. Define response behavior for multiple rows: union with source metadata or aggregate.

---

### Finding 3 — Rate-limit config for admin mutations is not actually added to `TenantConfig`
**Severity:** MEDIUM-HIGH
**Location:** Step 5, Step 6

The plan says:
> "Dedicated rate-limit key: `(tenantID, "admin_mutation", clientIP)` with separate configurable RPM via `TenantConfig` or env default"

But `TenantConfig` has no `AdminMutationRPM` field, and the plan does not propose one. Using the existing `AgentRateLimit` mechanism means:
- The RPM falls back to whatever `TenantConfig` defaults would be for `"external"` audience (often `60` from policy files)
- Or it requires a new env var that isn't defined

**Attack scenario:** An attacker who gained `tenant:admin` rights can trigger the rate limit to throttle legitimate admin operations (finding 2 from v1), because the cap is controlled by tenant-wide audience config, not admin-specific policy.

**Fix required:** Either add `AdminMutationRateLimitRPM` to `TenantConfig` with a safe default (e.g., `30`), or create a separate `admin_rate_limit` table / dedicated in-memory rate limiter that is not shared with audience traffic.

---

### Finding 4 — `SourceTemplateID` FK `ON DELETE SET NULL` silently unlinks assignments
**Severity:** MEDIUM
**Location:** Step 2 migration

The migration adds:
```sql
source_template_id REFERENCES tenant_role_templates(id) ON DELETE SET NULL
```

If an attacker somehow gains DDL access or finds a SQL injection path, they can delete a template and the `ON DELETE SET NULL` silently sets `SourceTemplateID = NULL`. The application guard in `HandleDeleteRoleTemplate` is bypassed because the DB itself nullifies the linkage. The "live reference guard" is not atomic with the delete.

**Attack scenario:** Admin A assigns template T to User X. Attacker with limited access triggers `DELETE FROM tenant_role_templates WHERE id = T.id` (e.g., via SQLi). FK cascades to SET NULL. `HandleDeleteRoleTemplate` never fires because the template is already gone. User X retains permissions from a nonexistent template.

**Fix required:** Remove reliance on FK `ON DELETE SET NULL` for the security invariant. Either:
- Keep the application guard as the **primary** protection and make the FK `ON DELETE RESTRICT` (but cite SQLite FK pragma caveat), or
- Add a trigger/check that raises an error instead of SET NULL when `SourceTemplateID` rows exist.

---

### Finding 5 — `ResolveEffectivePermissions` does not define behavior when multiple rows exist for same user/tenant
**Severity:** MEDIUM
**Location:** Step 4

After adding `SourceTemplateID`, a user may have multiple `UserTenantPermission` rows for the same `(user_id, tenant_id)`:
- Explicit grant: `{permissions: ["chat:logs"], source_template_id: NULL}`
- Template assignment: `{permissions: ["tenant:read"], source_template_id: 5}`

The plan says the resolver "loads UserTenantPermission for the user/tenant" — singular. It does not specify whether it:
- Queries all rows and unions permissions
- Only queries the "most recent" row
- Returns only template-derived permissions

**Attack scenario:** Admin assigns template `viewer` (`["tenant:read"]`) to User X. Admin explicitly grants `chat:logs`. If the resolver only returns the most recent row, User X loses previously granted permissions based on insertion order, not intended effective permissions.

**Fix required:** `ResolveEffectivePermissions` must query ALL `UserTenantPermission` rows for the user/tenant, union permissions, and return the deduplicated set. Document that a user has exactly one effective permission set per tenant regardless of row count.

---

### Finding 6 — `HandleListPermissions` exposes raw permission strings without source context
**Severity:** LOW-MEDIUM
**Location:** `handlers.go` line 2435+, not addressed in plan

`HandleListPermissions` returns `p.Permissions` directly. After `SourceTemplateID` is added, the response does not indicate whether a permission came from a template or an explicit grant. This breaks the audit differentiation promised by `permission_source` metadata in the resolver.

**Fix required:** Either extend `UserTenantPermissionResponse` to include `source_template_id` / `source_template_name`, or have `HandleListPermissions` resolve through `ResolveEffectivePermissions` which already has `permission_source`.

---

## Additional Observations

### A. `ContainsPermission` short-circuits on exact match before prefix expansion
The existing `ContainsPermission` code:
```go
if p == PermWildcard || p == required {
    return true
}
if strings.HasSuffix(p, ":*") {
    prefix := strings.TrimSuffix(p, "*")
    if strings.HasPrefix(required, prefix) {
        return true
    }
}
if p == PermTenantAdmin && strings.HasPrefix(required, "tenant:") {
    return true
}
```

If a user has `["tenant:admin", "tenant:read"]` and we check `chat:logs`, the function correctly returns `false`. If they have only `["tenant:admin"]`, the prefix-matching branch handles `tenant:*` checks but `tenant:admin` implies `tenant:*` only through the hardcoded special case. The plan's resolver doesn't need to expand `tenant:admin` at resolution time; `ContainsPermission` handles it at check time. The plan's wording on "expands `tenant:admin` into all `tenant:*`" is misleading but functionally acceptable.

### B. `PRAGMA foreign_keys = ON` is good, but SQLite still allows multiple NULLs in unique indexes
SQLite treats `NULL` as distinct in unique constraints, so multiple `tenant_role_templates` rows with `tenant_id = NULL` and `code = 'viewer"` could coexist if backfill is not idempotent **or** if the index is `NULLS NOT DISTINCT` (not supported in SQLite). The plan relies on `INSERT OR IGNORE`, which is correct. But the unique index definition should be verified: SQLite unique indexes allow multiple NULLs unless the column is declared `NOT NULL`. With `tenant_id` nullable, two rows with `tenant_id = NULL` and `code = 'viewer'` would violate the intended uniqueness.

**Fix:** Add a `CHECK (tenant_id IS NOT NULL)` for custom templates via application logic; for system templates, ensure only one row per code by using a sentinel `tenant_id = 0` with `NOT NULL` constraint or a composite unique index on `(COALESCE(tenant_id, 0), code)` (not directly supported in SQLite — requires generated column).

### C. `admin_mutation` rate-limit `AudienceType` collides with no existing code path
Using `"admin_mutation"` as `AudienceType` is technically fine, but `AgentRateLimit` rows are never cleaned up except by window expiry. An attacker creating many admin operations from different IPs creates many rows. This is a DoS concern against the rate-limit table itself, but low-severity.

---

## Checklist Gap Analysis

| Checklist Item | Status | Gap |
|----------------|--------|-----|
| Wire `hasPermission()` through resolver | Partial | `HandleListPermissions` bypass remains |
| `SourceTemplateID` for live-reference guard | Partial | Merging destroys provenance |
| ADMIN role `[PermTenantRead, PermAPIConfig]`, no wildcard | Closed | N/A |
| Separate rate-limit key | Partial | No configurable RPM field added |
| `PRAGMA foreign_keys = ON` | Closed | N/A |
| System template contents to `tenant:admin` only | Closed | N/A |
| No per-user permission caching | Closed | N/A |
| `getSystemRoleTemplate` unexported | Closed | N/A |
| Template permission explosion documented | Partial | Should be enforced in `ValidatePermissions` |
| Validate tenant access before filter | Closed | N/A |

---

## Prioritized Blockers Before Implementation

| Priority | Finding | Action |
|----------|---------|--------|
| P0 | Merged template permissions destroy `SourceTemplateID` | Separate rows per assignment; union in resolver |
| P0 | `HandleListPermissions` bypasses resolver + exposes raw strings | Route through `ResolveEffectivePermissions`; add source metadata |
| P0 | `ResolveEffectivePermissions` unspecified multi-row union behavior | Define exact query: `SELECT * FROM user_tenant_permission WHERE user_id = ? AND tenant_id = ?` |
| P1 | `AdminMutationRPM` not added to `TenantConfig` or env | Add configurable RPM field with safe default |
| P1 | `ON DELETE SET NULL` silently nullifies linkage | Consider `ON DELETE RESTRICT` + stronger app guard, or trigger |
| P2 | Unique index on `(tenant_id, code)` allows multiple NULLs in SQLite | Add `NOT NULL` sentinel or app-level dedup for system templates |
| P2 | `HandleGrantPermission` doesn't set `SourceTemplateID` (correct) but no separation from template rows | Document that explicit grants and template assignments are separate rows; handle N+1 resolver query cost |

---

## Conclusion

The v2 plan closes the majority of prior security gaps and shows strong security design intent. The **critical gap** is the "merge permissions" instruction, which defeats the purpose of `SourceTemplateID`. The **second critical gap** is that `HandleListPermissions` and other direct store calls are not audited or updated. The **third** is that rate-limit RPM configuration is not actually added anywhere. These should be resolved before writing the final plan or proceeding to implementation.
