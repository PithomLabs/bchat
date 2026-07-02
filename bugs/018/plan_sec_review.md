I cannot write the review back to `plan_sec.md` because of the permission restriction. Instead, here is the adversarial security review directly.

---

# Security Review: Tenant-Level Role Templates (`plan_sec.md`)

## Executive Summary

The plan introduces tenant-level role templates that map to `UserTenantPermission` bundles. After reviewing the plan against the existing auth, RBAC, bridge HMAC, and store-isolation code in `/home/chaschel/Documents/go/bchat`, I found **5 design-level vulnerabilities** and **8 hardening recommendations** that must be resolved before implementation.

---

## CRITICAL / HIGH Findings

### Finding 1 — `ValidatePermissions` rejects `tenant:admin` if it appears in a template
**Severity:** HIGH
**Location:** `permissions.go` `ValidatePermissions()` and plan Step 4

The existing `ValidatePermissions` only accepts:
- `*` (wildcard)
- `tenant:*` patterns
- Exact matches from `AllPermissions`

But `tenant:admin` is **not** in `AllPermissions`. If a template stores permissions as `["tenant:admin"]`, validation will return `false` and block creation/update. The plan says templates map to `UserTenantPermission` bundles, but does not account for this existing validation gap.

**Fix required:**
- Add `PermTenantAdmin` to `AllPermissions`
- Or add explicit tolerance for `PermTenantAdmin` in `ValidatePermissions`

---

### Finding 2 — `ResolveEffectivePermissions` permission expansion vs existing handlers inconsistency
**Severity:** HIGH
**Location:** Plan Step 4 (`ResolveEffectivePermissions`) vs `hasPermission()` in `handlers.go`

Existing `hasPermission()` checks the **stored** `UserTenantPermission.Permissions` via `ContainsPermission`. The plan's new helper expands `tenant:admin` into all `tenant:*` **at resolution time**. This creates a dual representation:

- Stored: `["tenant:admin"]`
- Resolved: `["tenant:read","tenant:write","tenant:admin","files:upload",...]`

If any existing handler (or cache layer) reads `UserTenantPermission.Permissions` directly instead of going through `ResolveEffectivePermissions`, the expansion is bypassed. The plan says to update `service.go` to use `ResolveUserPermissionsForTenant`, but **does not enforce** that all downstream consumers use it.

**Fix required:**
- Make `ResolveEffectivePermissions()` the **only** public way to read user-tenant permissions.
- Audit all existing calls to `GetUserTenantPermission` + direct `ContainsPermission` to ensure they route through the resolver.
- Update `configCache` invalidation logic to key off the resolved permission set, not the raw stored set.

---

### Finding 3 — System-default template protection is underspecified
**Severity:** HIGH
**Location:** Plan Step 3, Handlers Step 4 (`HandleDeleteRoleTemplate`, `HandleUpdateRoleTemplate`)

The plan says "system defaults protected" but does not define:
- What database constraint prevents deletion (`ON DELETE RESTRICT` vs application guard)
- Whether application-level checks are atomic with the query
- How the plan handles the case where `tenant_id` is NULL in SQL

In SQL, `NULL = NULL` is `FALSE`. If the query uses `WHERE tenant_id = ?` without `IS NULL`, system templates are invisible. If the index definition `(tenant_id, code)` doesn't cover NULL rows properly, lookups can miss system templates and fall back to empty defaults—creating authorization bypasses.

**Fix required:**
- Use `WHERE tenant_id IS NULL` explicitly for system template queries.
- Add a database-level `ON DELETE RESTRICT` or a CHECK constraint, not just in-application logic.
- Add a constant or explicit `nil` checks everywhere.

---

### Finding 4 — Concurrent template assignment vs template mutation is unsafe
**Severity:** HIGH
**Location:** Plan Step 4 (`HandleAssignRoleTemplate`) vs delete/update handlers

If an admin assigns template ID `5` to a user, and another admin simultaneously deletes template `5`, the assignment becomes a dangling reference. The plan does not address:
- Whether assignment should snapshot permissions or reference the template by ID.
- What happens if a template is deleted: do assigned users lose permissions immediately?

**Fix required:**
- Decide: assignments are either snapshots (copy permissions at assign time) or live references.
- If live: `DELETE` must cascade or be blocked if users are assigned.
- If snapshots: assignments don't need the template ID after creation.

---

### Finding 5 — Cache invalidation is racy and incomplete
**Severity:** MEDIUM-HIGH
**Location:** Plan Step 6 (`configCache` invalidation)

`ConfigCache` is initialized with a 5-minute TTL. The plan says "invalidate tenant on permission/role-template mutations." But:
- If invalidation uses a timestamp or version check, it must cover **both** `UserTenantPermission` mutations and `TenantRoleTemplate` mutations.
- The plan does not specify whether assignment (linking a template to a user) also triggers invalidation.
- A 5-minute TTL means a template's permission change takes up to 5 minutes to propagate—during which time the old permissions are cached. For `tenant:admin` ↔  downgrade scenarios, this is a privilege-escalation window.

**Fix required:**
- Invalidate on `Create/Update/Delete` of both `TenantRoleTemplate` and `UserTenantPermission` AND on `HandleAssignRoleTemplate`.
- Consider reducing the TTL or making invalidation synchronous for admin mutations.

---

## MEDIUM Findings

### Finding 6 — `ListTenantRoleTemplates` via `Code` filter enables enumeration
**Severity:** MEDIUM
**Location:** Plan Step 2 (`FindTenantRoleTemplate` struct)

If the API exposes a `code` query parameter, an attacker can brute-force template codes (e.g., `admin`, `viewer`, `custom_1`) to enumerate all templates regardless of tenant. The plan does not restrict which filters are exposed to the API.

**Fix required:**
- Only expose `tenant_id` filtering in the API. Never expose `Code` as a query parameter unless it's the exact value.
- Verify that `ListTenantRoleTemplates` splits into "system templates (visible to all authenticated tenant members)" and "custom templates (visible only to tenant:admin)".

---

### Finding 7 — `CreatedBy` becomes dangling reference on user deletion
**Severity:** MEDIUM
**Location:** Plan Step 1 (`TenantRoleTemplate.CreatedBy int32`)

The existing codebase has user accounts with roles. If a user is deleted, `CreatedBy` points to a non-existent user. The plan does not set `ON DELETE SET NULL` or a foreign key constraint.

**Fix required:**
- Make `CreatedBy` a nullable FK with `ON DELETE SET NULL` so deletions don't orphan template records.

---

### Finding 8 — Backfill seed durability under re-migration
**Severity:** MEDIUM
**Location:** Plan Step 3 (DB migration)

The migration says `INSERT` seeds after `CREATE TABLE IF NOT EXISTS`. If a migration is re-run (e.g., in CI, staging reset, or recovery), the unique `(tenant_id, code)` constraint will cause the second run to fail. The plan does not address idempotent seed insertion.

**Fix required:**
- Use `INSERT OR IGNORE` (SQLite) or `ON CONFLICT DO NOTHING` (Postgres) for seed inserts.

---

### Finding 9 — `isAdmin()` grants implicit `tenant:read` and `api:config` to all ADMINs
**Severity:** MEDIUM
**Location:** `handlers.go` `hasPermission()` lines 2194–2218

Every user with global `RoleAdmin` gets `tenant:read` and `api:config` on **all tenants**, regardless of tenant-specific permissions. The plan's `ResolveEffectivePermissions` returns the wildcard if global role is HOST/ADMIN.

If the plan introduces role templates that can **also** grant `tenant:admin` to non-admins, there's a privilege inversion risk: a tenant could accidentally grant `tenant:admin` template to a user who is also a global ADMIN, making it impossible to distinguish who performed an action for audit purposes.

**Fix required:**
- Add an audit log field that captures both the global role **and** the effective resolved permissions at access time.
- Consider adding `permission_source: "global_role" | "tenant_template" | "explicit"` to audit records.

---

### Finding 10 — `HandleListUserRoles` enables cross-tenant user enumeration
**Severity:** MEDIUM
**Location:** Plan Step 4 (`HandleListUserRoles`)

This endpoint lists a user's effective permissions and assigned template names for a tenant. If a user with minimal permissions calls this for a tenant they have `tenant:read` access to, they can enumerate:
- All users in that tenant
- Their permission sets (which templates they hold)
- The templates themselves (names, codes)

The plan does not restrict this to `tenant:admin` only.

**Fix required:**
- Restrict `HandleListUserRoles` to `tenant:admin` or `chat:logs` permission, not `tenant:read`.
- Do not expose template codes to users who should not know about admin internals.

---

### Finding 11 — JWT secret fallback to `"usememos"` in dev mode
**Severity:** MEDIUM (pre-existing, plan should flag)
**Location:** `server/server.go` lines 66–69

In dev/staging, if the JWT secret is `"usememos"`, any attacker who knows this can forge tokens and bypass authentication. The plan adds new `tenant:admin` functionality that is a high-value target.

**Fix required:**
- Add a startup check that warns or refuses to start in any mode (including dev) if `Secret == "usememos"` unless explicitly overridden.
- Generate a random secret on first boot if none is configured.

---

### Finding 12 — No rate-limit context for admin mutation endpoints
**Severity:** LOW-MEDIUM
**Location:** Plan Step 4 handlers

The plan adds new admin endpoints (`POST`, `PATCH`, `DELETE` for role templates). The existing codebase has an `AgentRateLimit` table but it's unclear if it covers mutation endpoints. If an attacker gains any tenant-level access, they can flood these endpoints.

**Fix required:**
- Apply per-tenant rate limiting to all `tenant:admin`-guarded mutation endpoints.

---

## Recommendations (Hardening)

### R1 — CORS on Echo agent routes
The plan adds new Echo handlers. The existing CORS middleware is only on the gRPC-gateway group. If the frontend is served from a different origin, new endpoints may be blocked by CORS unless the agent group also has CORS configured.

### R2 — `DeleteTenantRoleTemplate` PostgreSQL type alignment
The plan uses `id int32` for deletion. PostgreSQL auto-increment columns are `serial`/`bigserial`. Ensure the interface type matches across both drivers.

### R3 — `tenant_admin` preset vs `tenant:admin` permission naming
The plan refers to "tenant_admin template" while the permission is `tenant:admin`. The preset name in code is already `"tenant_admin"`. Ensure the plan's API responses use the permission format for consistency.

### R4 — Do not log sensitive template contents at INFO level
The new endpoints return permission lists. Logging them at `slog.Info` level (as current handlers do for permission checks) writes permission sets to logs. Ensure template details are logged at `Debug` level only.

### R5 — Add integration tests for concurrent access
The plan doesn't test concurrent template mutations or concurrent assign + delete. Add tests for:
- Two admins assigning the same template simultaneously.
- Template deletion during active assignment.

### R6 — `system_default` tenant_id needs explicit NULL handling
Using `nil` for system and `int32` for tenant is clear in Go but needs explicit SQL `IS NULL` checks. Consider adding a helper method to prevent logic errors.

### R7 — `FindTenantRoleTemplate` should not allow empty `Code` filter
If the API exposes an empty `code` filter, it returns all templates, which could bypass intended scoping. Validate that `Code` is non-empty when provided.

---

## Cross-Cutting Risk: RBAC Bypass via Direct Store Access
**Severity:** HIGH (architectural)

The plan adds a new table `tenant_role_templates` and new store methods. If **any existing handler bypasses `hasPermission()`** by calling the store directly, the effective permissions are read directly from the DB without the expansion logic. The plan does not audit these bypasses.

**Required action:** Before implementing the plan, grep for all `h.store.GetUserTenantPermission` and `h.store.ListUserTenantPermissions` calls. Every caller must be updated to route through `ResolveEffectivePermissions`.

---

## Prioritized Remediation Checklist

| Priority | Item | Finding |
|----------|------|---------|
| P0 | Add `PermTenantAdmin` to `AllPermissions` or update `ValidatePermissions` | Finding 1 |
| P0 | Enforce single resolver for effective permissions; audit all DB callers | Finding 2 |
| P0 | Implement `tenant_id IS NULL` explicit handling + DB RESTRICT for system templates | Finding 3 |
| P0 | Define assignment semantics (snapshot vs live reference) + cascade rules | Finding 4 |
| P0 | Make cache invalidation synchronous for admin mutations | Finding 5 |
| P1 | Restrict template listing filters in API | Finding 6 |
| P1 | Add `CreatedBy` nullable FK with `ON DELETE SET NULL` | Finding 7 |
| P1 | Make seed backfill idempotent | Finding 8 |
| P1 | Restrict `HandleListUserRoles` to `tenant:admin` | Finding 10 |
| P1 | Startup check for default JWT secret | Finding 11 |
| P2 | Apply rate limiting to mutation endpoints | Finding 12 |
| P2 | Frontend CORS check on agent routes | R1 |
| P2 | Audit logging for all template CRUD | R4 |
| P2 | Concurrent access integration tests | R5 |

## Out of Scope
- Bridge HMAC security review (previously reviewed)
- JWT rotation mechanism design
- Frontend CORS configuration (to be addressed separately)
- PostgreSQL migration dialect specifics
