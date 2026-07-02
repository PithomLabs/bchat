# Adversarial Review: `plan_sec4.md` (Security-Hardened v3)

## Overall Assessment

The v3 plan materially closes the prior blockers: separate rows per assignment, multi-row resolver, `SourceTemplateID` linkage, SQLite FK pragma, and dedicated rate-limit key. However, I found **4 new design gaps** and **3 signature/plumbing mismatches** that will cause runtime bugs or partial security at implementation time. Two of these are **P0** because they break the security invariants the plan claims to enforce.

---

## P0 — Security Invariant Breakers

### Finding 1 — `HandleGrantPermission` silently overwrites template assignments
**Severity:** HIGH

Current `GetUserTenantPermission` returns the first row from `ListUserTenantPermissions` ordered by `granted_at DESC` — i.e., the **most recent** row. With the new multi-row model, if a template assignment is more recent than an explicit grant, `HandleGrantPermission` finds the template row, updates it, and sets `SourceTemplateID = nil`. Result:
- The template assignment is corrupted
- The deletion guard is weakened because `SourceTemplateID` is now `nil`
- The user's explicit grant is merged into the template row, breaking provenance

**Attack scenario:**
1. Admin assigns `viewer` template to User X (row A: `granted_at=100`, `source_template_id=1`)
2. Admin explicitly grants `chat:logs` to User X. `HandleGrantPermission` calls `GetUserTenantPermission`, which returns row A (most recent). It updates row A to `{permissions: ["tenant:read","chat:logs"], source_template_id: nil}`.
3. Admin deletes `viewer` template. Deletion succeeds because no row has `source_template_id=1`.
4. User X retains `tenant:read` from a deleted template with no trace.

**Fix required:** `HandleGrantPermission` must query specifically for the explicit-grant row (`WHERE source_template_id IS NULL`) before updating. If none exists, create a new row. Never update a template-linked row.

---

### Finding 2 — Repeated `HandleAssignRoleTemplate` creates duplicate rows
**Severity:** MEDIUM-HIGH

The plan says "creates separate `UserTenantPermission` row" but does not include an idempotency check. Calling `POST /role-templates/:id/assign` twice for the same `(user_id, tenant_id, template_id)` creates two identical rows. The resolver deduplicates permissions, but:
- The deletion guard `SELECT COUNT(*) WHERE source_template_id = ?` returns `2`, making template deletion appear "more blocked" than intended
- `HandleListPermissions` returns duplicate entries for the same user
- Audit trails show phantom assignments

**Fix required:** Before inserting, query `SELECT id FROM user_tenant_permissions WHERE user_id = ? AND tenant_id = ? AND source_template_id = ?`. If exists, return `200 OK` idempotently without inserting.

---

## P1 — Design Gaps That Cause Runtime Bugs

### Finding 3 — `AdminMutationRateLimitRPM` plumbing incomplete
**Severity:** MEDIUM

The plan adds `AdminMutationRateLimitRPM` to `TenantConfig` but does not:
1. Specify the DB migration to add this column
2. Specify how it flows into `AudienceConfig` (which is what `configCache` caches)
3. Specify the env default or safe minimum

Currently `TenantConfig` has no rate-limit field at all — `RateLimitRPM` lives on `AgentAudience`. `CheckRateLimit(ctx, tenantID, "external", clientIP, rpm)` reads from `config.Audience.RateLimitRPM`. The new `admin_mutation` path needs either:
- A new field on `TenantConfig` with a migration
- Or a separate lookup in `HandleAssignRoleTemplate` that bypasses `AudienceConfig`

Without this, the dedicated rate-limit key falls back to a hardcoded magic number or the external audience RPM.

---

### Finding 4 — `ContainsPermission` / `containsResolvedPermission` signature mismatch
**Severity:** MEDIUM

The plan rewrites `hasPermission()` to cache `[]ResolvedPermission` and calls `containsResolvedPermission(cached, permission)`. But `ContainsPermission` currently takes `[]string`. The plan does not specify whether:
- `ContainsPermission` is overloaded to accept `[]ResolvedPermission` (impossible in Go without generics or interface)
- A new `containsResolvedPermission([]ResolvedPermission, string) bool` is added

This is a straightforward fix but must be explicit in the plan to avoid implementation drift.

---

### Finding 5 — `HandleListPermissions` response format needs new struct
**Severity:** MEDIUM

Currently `UserPermissionResponse` has `Permissions []string`. The plan says `HandleListPermissions` returns source metadata via the resolver. After multi-row union, the response is a single effective set per user. The plan needs to specify the new response shape, e.g.:

```go
type UserPermissionResponse struct {
    UserID      int32             `json:"user_id"`
    Username    string            `json:"username"`
    Permissions []ResolvedPermission `json:"permissions"`
    GrantedBy   string            `json:"granted_by,omitempty"`
    GrantedAt   string            `json:"granted_at"`
}
```

Without this, the handler cannot return source metadata, and validation item 12 ("returns source metadata per permission") fails.

---

### Finding 6 — `COALESCE(tenant_id, 0)` sentinel risks collision with real tenant IDs
**Severity:** LOW-MEDIUM

The plan uses `COALESCE(tenant_id, 0)` for the unique index. Tenant IDs are auto-increment integers starting at `1` in most DB setups. If `0` ever appears as a real `tenant_id` (e.g., from a data migration, manual insert, or test fixture), a custom template for tenant `0` collides with the system-template sentinel in the unique index.

**Fix required:** Either:
- Use a sentinel outside the auto-increment range (e.g., `-1`), or
- Add a `CHECK (tenant_id >= 1)` constraint for non-system rows, or
- Document that `tenant_id = 0` is reserved and enforce at application level

---

## Additional Concerns

### A. `RoleAdmin` implicit grants vs explicit `tenant:admin` template
An `RoleAdmin` user gets `[PermTenantRead, PermAPIConfig]` from the resolver. If a tenant admin assigns a `tenant_admin` template to an `RoleAdmin` user, the resolver returns the union: `[PermTenantRead, PermAPIConfig, PermTenantAdmin]`. The `tenant:admin` permission then expands to all `tenant:*` via `ContainsPermission`. This means a **global ADMIN** can be escalated to full tenant admin by a tenant admin. Whether this is intentional is unclear.

**Recommendation:** Document whether global ADMINs can receive `tenant:admin` via template, or block template assignment to users with `RoleHost`/`RoleAdmin`.

---

### B. `configCache` key includes `audienceType`; admin mutations are audience-agnostic
`ConfigCache` is keyed by `tenantSlug:audienceType`. The plan invalidates it on `TenantConfig` mutations. But `AudienceConfig` is audience-specific. If `AdminMutationRateLimitRPM` lives on `TenantConfig`, the cache invalidation on tenant config change is correct. However, the field must be added to `AudienceConfig` or read directly from `TenantConfig` in the handler.

---

### C. `ValidatePermissions` wildcard-only template policy
The plan says "explicitly flag or reject wildcard-only templates" but does not choose. If wildcard-only templates are rejected, `PermissionPresets` is unaffected. If flagged, the API response must include the warning. This decision should be finalized before tests are written.

---

## Checklist Gap Analysis

| Checklist Item | Status | Gap |
|----------------|--------|-----|
| Separate rows per assignment; no merging | Partial | No idempotency check; `HandleGrantPermission` can corrupt template rows |
| Resolver unions ALL rows | Closed | N/A |
| Audit all direct store callers | Partial | `HandleGrantPermission` still uses `GetUserTenantPermission` unsafely |
| `HandleListPermissions` returns source metadata | Partial | Response struct not defined |
| ADMIN role `[PermTenantRead, PermAPIConfig]` only | Closed | N/A |
| `AdminMutationRateLimitRPM` in `TenantConfig` | Partial | No migration specified; no `AudienceConfig` plumbing |
| Application guard primary for system templates | Closed | N/A |
| Unique index `(COALESCE(tenant_id, 0), code)` | Partial | Sentinel `0` collision risk with real tenant IDs |
| `PRAGMA foreign_keys = ON` | Closed | N/A |
| System template contents to `tenant:admin` only | Closed | N/A |
| No per-user permission caching | Closed | N/A |
| `getSystemRoleTemplate` unexported | Closed | N/A |
| Wildcard-only templates flagged/rejected | Open | Policy decision not made |
| Validate tenant access before filter | Closed | N/A |

---

## Prioritized Actions Before Implementation

| Priority | Finding | Action |
|----------|---------|--------|
| P0 | `HandleGrantPermission` overwrites template rows | Query `WHERE source_template_id IS NULL` explicitly for explicit grants |
| P0 | Duplicate assignment rows on repeated assign | Add idempotency check before insert |
| P1 | `AdminMutationRateLimitRPM` migration + plumbing missing | Add migration; add field to `AudienceConfig` or read from `TenantConfig` in handler |
| P1 | `containsResolvedPermission` signature undefined | Define new function or adapt `ContainsPermission` |
| P1 | `HandleListPermissions` response struct undefined | Define `ResolvedPermission` JSON response and update `UserPermissionResponse` |
| P1 | Sentinel `0` collision risk for system templates | Use `-1` or add `CHECK` constraint; document reservation |
| P2 | Wildcard-only template policy not decided | Choose "reject" or "flag with warning" before writing tests |
| P2 | Global ADMIN receiving `tenant:admin` via template | Document intended behavior or add guard |

---

## Conclusion

The v3 plan is structurally sound and closes most prior gaps. The **critical blocker** is that `HandleGrantPermission` will corrupt template assignments in the multi-row model unless it explicitly targets `source_template_id IS NULL` rows. The **second blocker** is missing idempotency on template assignment. These should be fixed in the plan before implementation begins.
