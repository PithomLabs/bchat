# Plan: Tenant-Level Role Templates for Multi-Tenant RBAC (Security-Hardened v4)

## Goal
Add assignable tenant-level role templates (`tenant_viewer`, `tenant_editor`, `tenant_admin`, plus custom variants) that map to predefined `UserTenantPermission` bundles, enabling multi-tenant isolation without changing the global `User.Role` semantics.

## Decisions Made

| Topic | Decision |
|---|---|
| Architecture | Single shared instance (Option B) with application-layer multitenant RBAC |
| RBAC approach | Hybrid: system defaults in code + optional custom templates stored per tenant |
| Scope | Full stack: types + DB + handlers + service + tests |
| Permissions source | Reuse existing `UserTenantPermission`, `PermissionPresets`, and `ContainsPermission` |
| Assignment semantics | **Separate rows per assignment**: one `UserTenantPermission` row per template assignment with `SourceTemplateID = templateID`; explicit grants remain separate rows with `SourceTemplateID = nil` |
| Permission resolution | Unions ALL rows for `(user_id, tenant_id)` and returns deduped `[]ResolvedPermission` with source metadata |
| Resolver output | `ResolvedPermission{Permission string, Source string}` where Source is `"global_role"` \| `"tenant_template"` \| `"explicit"` |
| ADMIN resolution | `RoleAdmin` returns `[PermTenantRead, PermAPIConfig]` only — **no wildcard** — preserves existing `hasPermission()` semantics exactly |
| Cache strategy | No per-user permission caching; resolve on each request (cheap single-user lookup) |
| System template visibility | Contents visible to **tenant:admin only**; names/descriptions visible to tenant:read |
| Rate limiting | Dedicated `admin_mutation` rate-limit key with `AdminMutationRateLimitRPM` configurable in `TenantConfig` + env default |
| System template sentinel | `tenant_id = -1` for system templates (reserved, outside auto-increment range) |
| Wildcard-only templates | **Rejected** in `ValidatePermissions` to prevent namespace bypass |
| Global ADMIN + tenant template | Documented behavior: global ADMINs can be assigned tenant templates; escalated permissions are unioned and auditable |

---

## Change Summary

### 1. Types

**`server/router/api/v1/agent/role_template.go`** — new file
- `TenantRoleTemplate` struct: `ID`, `TenantID (*int32, nil = not used; sentinel -1 used instead)`, `Name`, `Code`, `Permissions []string`, `CreatedBy *int32` (nullable), `CreatedAt`, `UpdatedAt`
- `FindTenantRoleTemplate` struct: filters for `ID`, `TenantID`, `Code`, `Name`
- `CreateTenantRoleTemplateRequest`, `UpdateTenantRoleTemplateRequest`
- `TenantRoleTemplateResponse`
- **`getSystemRoleTemplate(code string) ([]string, bool)` unexported** — no timing oracle risk

**`store/role_template.go`** — new file
- Define `TenantRoleTemplate` and find structs in `store` package
- Add interface to `RBACStore`:
  - `CreateTenantRoleTemplate(ctx, *TenantRoleTemplate) (*TenantRoleTemplate, error)`
  - `GetTenantRoleTemplate(ctx, *FindTenantRoleTemplate) (*TenantRoleTemplate, error)`
  - `ListTenantRoleTemplates(ctx, *FindTenantRoleTemplate) ([]*TenantRoleTemplate, error)`
  - `UpdateTenantRoleTemplate(ctx, *TenantRoleTemplate) (*TenantRoleTemplate, error)`
  - `DeleteTenantRoleTemplate(ctx, id int32) error`
- Add `Store` wrapper methods delegating to driver

**`store/rbac.go`** — update `UserTenantPermission`
- Add `SourceTemplateID *int32` (nullable) to track live template assignment
- Add `ResolvedPermission` struct (or define in `permissions.go`): `Permission string`, `Source string`

**`server/router/api/v1/agent/permissions.go`**
- Add `ResolvedPermission` struct: `Permission string`, `Source string`
- Add `containsResolvedPermission([]ResolvedPermission, string) bool` — mirrors `ContainsPermission` for `[]string`
- Add `PermTenantAdmin` to `AllPermissions` (fixes Finding 1)
- Add unexported `getSystemRoleTemplate(code string) ([]string, bool)`
- **New mandatory resolver**:
  ```go
  ResolveEffectivePermissions(ctx, store, tenantID, userID int32) ([]ResolvedPermission, error)
  ```
  - For `RoleHost`: returns `[{Permission: "*", Source: "global_role"}]`
  - For `RoleAdmin`: returns `[{Permission: PermTenantRead, Source: "global_role"}, {Permission: PermAPIConfig, Source: "global_role"}]` — **matches existing `hasPermission()` implicit grants exactly**
  - For `RoleUser`/others: queries **ALL** `UserTenantPermission` rows for `(user_id, tenant_id)` (no single-row assumption)
  - Unions all permissions across rows
  - Deduplicates
  - Annotates each permission with source: `"global_role"` \| `"tenant_template"` \| `"explicit"`
  - `tenant:admin` prefix matching handled at check time by `containsResolvedPermission`

### 2. Database

**Migration**: `store/migration/<dialect>/NN__tenant_role_templates.sql`
- Table `tenant_role_templates` with `IF NOT EXISTS`
- Columns: `id`, `tenant_id (integer, NOT NULL DEFAULT -1)`, `name`, `code`, `permissions (text/json)`, `created_by (nullable, FK to user)`, `created_at`, `updated_at`
- Unique index on `(tenant_id, code)` — system templates use `tenant_id = -1`; custom templates use real tenant IDs
- `CHECK (tenant_id = -1 OR tenant_id >= 1)` to prevent accidental `0` or negative custom tenant IDs
- FK: `tenant_id REFERENCES agent_tenant(id) ON DELETE CASCADE`
- FK: `created_by REFERENCES user(id) ON DELETE SET NULL`
- **Protection**: application-level guard + unit tests; DB constraint is supplementary
- Backfill seeds: idempotent `INSERT OR IGNORE` (SQLite) / `ON CONFLICT DO NOTHING` (Postgres) for system-default rows with `tenant_id = -1`: `viewer`, `tester`, `analyst`, `editor`, `tenant_admin`

**Migration**: `store/migration/<dialect>/NN__add_template_source_to_permissions.sql`
- Add `source_template_id` column to `user_tenant_permissions` (nullable int)
- Add index on `(user_id, tenant_id, source_template_id)` for fast deletion-guard queries
- FK: `source_template_id REFERENCES tenant_role_templates(id) ON DELETE SET NULL`
- Backfill: `UPDATE user_tenant_permissions SET source_template_id = NULL WHERE ...` (existing rows remain explicit grants)

**Migration**: `store/migration/<dialect>/NN__add_admin_mutation_rate_limit.sql`
- Add `admin_mutation_rate_limit_rpm` column to `tenant_config` (integer, default `30`)
- Or add to `agent_audience` if that's where rate limits are configured for the tenant

**Note:** Confirm whether `RateLimitRPM` lives on `AgentAudience` or `TenantConfig`. If on `AgentAudience`, add `AdminMutationRateLimitRPM` there instead and backfill from `TenantConfig` in service layer.

### 3. SQLite FK Enforcement

**`store/db/sqlite/`** — update connection setup
- Add `PRAGMA foreign_keys = ON` to every new SQLite connection
- Add migration step that verifies pragma is active (e.g., `PRAGMA foreign_keys` returns `1`)
- **Never rely on SQLite FK enforcement alone for critical constraints** — application guard is primary protection

### 4. Permission Resolution — Single Source of Truth

**`server/router/api/v1/agent/permissions.go`** — update
- Add `PermTenantAdmin` to `AllPermissions`
- Add `ResolvedPermission` struct: `Permission string`, `Source string`
- Add `containsResolvedPermission([]ResolvedPermission, string) bool`
- Add unexported `getSystemRoleTemplate(code string) ([]string, bool)`
- **New mandatory resolver**:
  ```go
  ResolveEffectivePermissions(ctx context.Context, store *store.Store, tenantID, userID int32) ([]ResolvedPermission, error)
  ```
  - For `RoleHost`: returns `[{Permission: "*", Source: "global_role"}]`
  - For `RoleAdmin`: returns `[{Permission: PermTenantRead, Source: "global_role"}, {Permission: PermAPIConfig, Source: "global_role"}]`
  - For `RoleUser`/others: queries **ALL** `UserTenantPermission` rows for `(user_id, tenant_id)`
  - Unions all permissions across rows
  - Deduplicates
  - Annotates source: rows with `SourceTemplateID != nil` → `"tenant_template"`; rows with `SourceTemplateID == nil` → `"explicit"`
- **Update `hasPermission()`** in `handlers.go` to call `ResolveEffectivePermissions()` and cache `[]ResolvedPermission` per-request. Use `containsResolvedPermission()` for checks.

### 5. Handlers

**`/v1/agent/handlers.go`** — update `hasPermission()` + append new handlers + audit all direct store calls

**Update `hasPermission()`:**
```go
func (h *Handler) hasPermission(c echo.Context, tenantID int32, permission string) bool {
    if cached, ok := c.Get("resolved_perms").([]ResolvedPermission); ok {
        return containsResolvedPermission(cached, permission)
    }
    
    userID := getUserID(c)
    perms, err := ResolveEffectivePermissions(c.Request().Context(), h.store, tenantID, userID)
    if err != nil {
        slog.Warn("permission resolution failed", ...)
        return false
    }
    
    c.Set("resolved_perms", perms)
    return containsResolvedPermission(perms, permission)
}
```

**Audit and fix direct store calls:**
- `HandleGrantPermission` (line ~2503): **MUST query `WHERE source_template_id IS NULL`** before updating. If no explicit-grant row exists, create a new row with `SourceTemplateID = nil`. Never update a template-linked row.
- `HandleListPermissions` (line ~2435): route through `ResolveEffectivePermissions` per user; return `[]ResolvedPermission` with source metadata
- Any other `GetUserTenantPermission` / `ListUserTenantPermissions` callers: route through resolver or explicitly document why direct access is safe

**New endpoint handlers:**
- `HandleListRoleTemplates` `GET /api/v1/agent/:slug/role-templates`
  - Validates caller has `tenant:read` for the tenant before applying any filter
  - Returns **system templates with contents only to `tenant:admin`**; `tenant:read` sees names/descriptions only
  - No `code` filter exposed in API
- `HandleCreateRoleTemplate` `POST /api/v1/agent/:slug/role-templates`
  - Creates custom template; requires `tenant:admin`
  - **Dedicated rate-limit key**: `(tenantID, "admin_mutation", clientIP)` with RPM from `TenantConfig.AdminMutationRateLimitRPM` or env default
  - Validates permissions via updated `ValidatePermissions`; **rejects wildcard-only templates** (`["*"]` alone)
- `HandleUpdateRoleTemplate` `PATCH /api/v1/agent/role-templates/:id`
  - Updates custom template; requires owning tenant `tenant:admin`
  - System templates (`tenant_id = -1`) protected: 403
- `HandleDeleteRoleTemplate` `DELETE /api/v1/agent/role-templates/:id`
  - Deletes custom template; system templates protected: 403
  - **Blocks deletion if any `UserTenantPermission` has `SourceTemplateID = templateID`** (live-reference guard)
  - Queries `SELECT COUNT(*) FROM user_tenant_permissions WHERE source_template_id = ?`
  - **Primary protection is application guard**, not FK `ON DELETE SET NULL`
- `HandleAssignRoleTemplate` `POST /api/v1/agent/:slug/role-templates/:id/assign`
  - Assigns template permissions to a user; requires `tenant:admin`
  - **Idempotency check**: query `SELECT id FROM user_tenant_permissions WHERE user_id = ? AND tenant_id = ? AND source_template_id = ?`. If exists, return 200 without inserting.
  - **Creates separate `UserTenantPermission` row** with `SourceTemplateID = templateID` and the template's permissions
  - Does NOT merge with existing rows; existing explicit grants remain separate
  - Dedicated rate-limit key: `(tenantID, "admin_mutation", clientIP)`
  - **No caching of resolved permissions** — computed per request
- `HandleListUserRoles` `GET /api/v1/agent/:slug/users/:userId/roles`
  - Lists user's effective permissions with source metadata for tenant
  - **Restricted to `tenant:admin` only** (not `tenant:read`)

### 6. Service & Config

**`/v1/agent/service.go`** — updates
- Add `ResolveUserPermissionsForTenant(ctx, tenantID, userID int32) ([]ResolvedPermission, error)` wrapper
- **No configCache invalidation for permissions** — resolved per request
- Add `AdminMutationRateLimitRPM` field to `TenantConfig` with safe default (`30`); read from env `ADMIN_MUTATION_RATE_LIMIT_RPM` if set
- If `RateLimitRPM` lives on `AgentAudience`, add `AdminMutationRateLimitRPM` there too and plumb through
- Invalidate `configCache` on `TenantConfig` mutations as before

### 7. Route Registration

**`/v1/v1.go`**
- Register all new role-template routes under `agent/:slug` prefix
- Apply CORS middleware to agent group if not already present

### 8. Tests

- **Unit**: `/v1/agent/role_template_test.go`
  - CRUD for templates (system defaults protected, custom CRUD allowed)
  - Assignment idempotency: duplicate assign returns 200 without inserting
  - Assignment creates separate row with `SourceTemplateID = templateID`; does not merge
  - `HandleGrantPermission` explicitly targets `source_template_id IS NULL` rows; does not corrupt template rows
  - Resolver unions ALL rows for user/tenant and deduplicates
  - Permission expansion (`tenant:admin` → all `tenant:*`)
  - `ValidatePermissions` accepts `PermTenantAdmin`; rejects wildcard-only templates
  - `hasPermission()` wired to resolver with per-request cache
  - ADMIN role returns `[PermTenantRead, PermAPIConfig]` only — no wildcard
  - System templates visible to `tenant:read` with names only, not contents
- **Store tests**: SQLite + Postgres
  - Sentinel `-1` handling for system templates
  - `CHECK (tenant_id = -1 OR tenant_id >= 1)` enforced
  - Unique index `(tenant_id, code)` prevents duplicate system templates
  - `SourceTemplateID` foreign key behavior
  - `AdminMutationRateLimitRPM` migration and default
  - Seed backfill idempotency
  - `PRAGMA foreign_keys = ON` verification
- **Handler tests**:
  - Each new endpoint's required permission and unauthorized path
  - `HandleGrantPermission` does not overwrite template-linked rows
  - `HandleListPermissions` returns `ResolvedPermission` with source metadata
  - `HandleListRoleTemplates`: `tenant:read` sees names only, `tenant:admin` sees contents
  - `HandleDeleteRoleTemplate`: blocks deletion when `SourceTemplateID` links exist
  - Rate limiting on mutation endpoints with dedicated key and configurable RPM
  - Tenant access validated before applying tenant filter
- **Integration**: concurrent access
  - Two admins assigning same template simultaneously (idempotent, no duplicate rows)
  - Template deletion during active assignment (should return 409 conflict)

### 9. Frontend (light updates)

- Call `HandleListRoleTemplates` in tenant admin user management UI
- Display template name plus permission list (full contents only for tenant:admin)
- Call `HandleAssignRoleTemplate` from user-role dropdown
- Do not expose template codes to non-admin users

---

## Security Remediation Checklist (from review v4)

| Priority | Item | Status |
|---|---|---|
| P0 | `HandleGrantPermission` queries `WHERE source_template_id IS NULL` explicitly; never updates template-linked rows | ✅ Step 5 |
| P0 | `HandleAssignRoleTemplate` idempotency check before insert | ✅ Step 5 |
| P0 | Resolver unions ALL rows for user/tenant, returns `[]ResolvedPermission` with source | ✅ Step 4 |
| P0 | Audit all `GetUserTenantPermission`/`ListUserTenantPermissions` callers; route through resolver | ✅ Step 5 |
| P0 | `HandleListPermissions` returns `ResolvedPermission` with source metadata | ✅ Step 5 |
| P0 | ADMIN role: `[PermTenantRead, PermAPIConfig]` only, no wildcard | ✅ Step 4 |
| P1 | `AdminMutationRateLimitRPM` added to `TenantConfig` (or `AgentAudience`) + migration + env default | ✅ Step 2, 6 |
| P1 | Application guard primary for system template deletion; FK `ON DELETE SET NULL` not relied upon for security | ✅ Step 5 |
| P1 | System template sentinel `tenant_id = -1` with `CHECK (tenant_id = -1 OR tenant_id >= 1)` | ✅ Step 2 |
| P1 | `PRAGMA foreign_keys = ON` + verification | ✅ Step 3 |
| P1 | System template contents restricted to `tenant:admin` | ✅ Step 5 |
| P1 | `containsResolvedPermission([]ResolvedPermission, string) bool` defined | ✅ Step 1, 4 |
| P2 | No per-user permission caching — resolve on each request | ✅ Step 4, 6 |
| P2 | `getSystemRoleTemplate` unexported | ✅ Step 1, 4 |
| P2 | Wildcard-only templates rejected in `ValidatePermissions` | ✅ Step 5 |
| P2 | Validate tenant access before applying tenant filter | ✅ Step 5 |
| P2 | Global ADMIN + tenant template behavior documented | ✅ Step 1, 5 |
| P2 | `HandleGrantPermission` creates explicit grant rows with `SourceTemplateID = nil` | ✅ Step 5 |

---

## Execution Order

1. `store/role_template.go` types + interface + `SourceTemplateID` on `UserTenantPermission` + `ResolvedPermission`
2. DB migrations: `tenant_role_templates` (sentinel `-1`, CHECK constraint) + `user_tenant_permissions.source_template_id` + `admin_mutation_rate_limit_rpm`
3. SQLite FK pragma + verification
4. `permissions.go`: `PermTenantAdmin` in `AllPermissions`, unexported `getSystemRoleTemplate`, `containsResolvedPermission`, `ResolveEffectivePermissions` with multi-row union
5. `handlers.go`: update `hasPermission()` with per-request cache + audit all direct store calls + fix `HandleGrantPermission` + new endpoints with rate limiting and access controls
6. `service.go` + `TenantConfig`/`AgentAudience`: add `AdminMutationRateLimitRPM`
7. Route registration + CORS
8. Tests (unit + store + handler + integration)
9. Frontend integration (minimal)

## Validation

1. Existing tests still pass
2. New stores compile on both SQLite and Postgres
3. Backfill idempotent on re-run
4. `tenant_role_templates` uses sentinel `-1` for system templates; `CHECK` prevents invalid tenant IDs
5. Unique index `(tenant_id, code)` prevents duplicate system templates
6. Assigning `tenant_admin` template creates separate row with `SourceTemplateID = templateID`
7. Repeated assignment is idempotent (no duplicate rows)
8. Resolver unions ALL rows for user/tenant and returns deduped `[]ResolvedPermission` with source metadata
9. Global `HOST`/`ADMIN` retain intended access: HOST gets `*`, ADMIN gets `[tenant:read, api:config]`
10. `tenant:admin` effectively implies all `tenant:*` permissions via `containsResolvedPermission`
11. System templates cannot be deleted/updated via API
12. `HandleGrantPermission` targets `source_template_id IS NULL` rows only; never corrupts template assignments
13. `HandleListPermissions` returns `ResolvedPermission` with source per permission
14. `HandleListUserRoles` restricted to `tenant:admin`
15. No `code` filter exposed in list API
16. Dedicated rate-limit key for admin mutations with configurable RPM
17. `PRAGMA foreign_keys = ON` verified
18. Wildcard-only templates rejected in `ValidatePermissions`
19. Concurrent assignment + delete tested (409 conflict)

---

## Open Decisions (Non-Blocking)

| Decision | Options | Recommendation |
|----------|---------|----------------|
| `AdminMutationRateLimitRPM` location | `TenantConfig` vs `AgentAudience` | Add to `TenantConfig` for tenant-wide policy; read in handler directly |
| Global ADMIN + tenant template | Allow / block / warn | Allow; escalate permissions are unioned and fully auditable via `source` metadata |
| Wildcard-only templates | Reject vs flag | Reject in `ValidatePermissions`; too dangerous to allow |




## FINAL MINIMUM VIABLE PLAN

Here is the **minimum viable security plan** distilled from all reviews. It preserves every P0/P1 finding while dropping speculative or redundant controls.

---

# Minimum Viable Plan: Tenant-Level Role Templates (Security-Hardened v5)

## Goal
Add assignable tenant-level role templates that map to `UserTenantPermission` bundles — multi-tenant isolation on a shared instance, no global `User.Role` changes.

## Core Decisions

| Topic | Decision |
|---|---|
| Architecture | Single shared instance + application-layer multitenant RBAC |
| Templates | Hybrid: system defaults in code + optional custom templates per tenant in DB |
| Assignment model | Separate `UserTenantPermission` rows per assignment; `SourceTemplateID` tracks linkage |
| Deletion guard | App-level `SELECT COUNT(*) WHERE source_template_id = ?` blocks template deletion while assignments exist |
| Resolver | Single `ResolveEffectivePermissions()`; queries ALL rows per `(user_id, tenant_id)`, unions permissions, returns `[]ResolvedPermission{Permission, Source}` |
| ADMIN semantics | `RoleAdmin` → `[tenant:read, api:config]` only; **no wildcard** |
| Caching | No per-user permission cache; resolve on each request |
| Rate limiting | Dedicated `(tenantID, "admin_mutation", clientIP)` key with `AdminMutationRateLimitRPM` default `30` |
| System templates | `tenant_id = -1` sentinel; `CHECK (tenant_id = -1 OR tenant_id >= 1)` |
| Visibility | System template **names/descriptions** visible to `tenant:read`; **contents** visible only to `tenant:admin` |
| Wildcard templates | **Rejected** in `ValidatePermissions` |

---

## Required Changes

### A. Types
- **`store/role_template.go`**: `TenantRoleTemplate`, `FindTenantRoleTemplate`, CRUD interface on `RBACStore`
- **`store/rbac.go`**: add `SourceTemplateID *int32` to `UserTenantPermission`
- **`permissions.go`**: `ResolvedPermission{Permission, Source}`, `containsResolvedPermission()`, `getSystemRoleTemplate()`, `ResolveEffectivePermissions()`, add `PermTenantAdmin` to `AllPermissions`

### B. Database (2 migrations)
1. **`tenant_role_templates`**: `tenant_id = -1` sentinel for system; `CHECK` constraint; unique index on `(tenant_id, code)`; idempotent backfill seeds
2. **`user_tenant_permissions.source_template_id`**: nullable FK; index on `(user_id, tenant_id, source_template_id)`; backfill `NULL`
3. **`admin_mutation_rate_limit_rpm`**: add to `TenantConfig` default `30`

### C. Permission Resolver
```go
ResolveEffectivePermissions(ctx, store, tenantID, userID) ([]ResolvedPermission, error)
```
- `RoleHost` → `[{*, global_role}]`
- `RoleAdmin` → `[{tenant:read, global_role}, {api:config, global_role}]`
- Others → ALL rows for `(user_id, tenant_id)`, union, annotate source (`tenant_template` / `explicit`)

### D. Handlers
- **`hasPermission()`**: route through resolver; cache `[]ResolvedPermission` per-request
- **Audit all direct `GetUserTenantPermission` / `ListUserTenantPermissions` callers**
- **`HandleGrantPermission`**: query `WHERE source_template_id IS NULL` explicitly; never update template-linked rows
- **`HandleListPermissions`**: return `ResolvedPermission` with source metadata
- **New endpoints**:
  - `GET /role-templates` — system templates to `tenant:admin` only for contents
  - `POST /role-templates` — `tenant:admin`; reject wildcard-only; rate-limited
  - `PATCH /role-templates/:id` — `tenant:admin`; system templates protected
  - `DELETE /role-templates/:id` — blocks if `SourceTemplateID` references exist
  - `POST /role-templates/:id/assign` — idempotent insert; `SourceTemplateID = templateID`; rate-limited
  - `GET /users/:userId/roles` — `tenant:admin` only

### E. Config & Service
- `TenantConfig.AdminMutationRateLimitRPM` default `30`
- No permission cache invalidation needed

### F. SQLite
- `PRAGMA foreign_keys = ON` on every connection; verify in migration

### G. Tests
- Assignment idempotency; no merge
- `HandleGrantPermission` doesn't corrupt template rows
- Resolver multi-row union
- System template deletion blocked
- Rate limiting on admin mutations
- `PRAGMA foreign_keys = ON`

---

## Security Guarantees

| Guarantee | Mechanism |
|---|---|
| Template deletion guard | App checks `SourceTemplateID` references before delete |
| No permission bypass | Single resolver; all handlers route through it |
| Explicit grant isolation | `HandleGrantPermission` targets `source_template_id IS NULL` only |
| No duplicate assignments | Idempotency check on `(user_id, tenant_id, template_id)` |
| Rate limiting | Separate `admin_mutation` key with configurable RPM |
| Audit fidelity | `source` metadata on every resolved permission |
| Wildcard containment | `ValidatePermissions` rejects `["*"]`-only templates |

---

## Execution Order

1. Types + store interface
2. DB migrations
3. SQLite FK pragma
4. `permissions.go` resolver + helpers
5. `handlers.go` — fix `hasPermission()`, audit callers, add endpoints
6. `service.go` / `TenantConfig` — rate-limit field
7. Route registration + CORS
8. Tests
9. Frontend (minimal)

---

## Open Decisions (Non-Blocking)

| Decision | Default |
|---|---|
| `AdminMutationRateLimitRPM` location | `TenantConfig` (recommended) |
| Global ADMIN + tenant template | Allow; auditable via `source` metadata |
| Wildcard-only templates | Reject |

---

This is the minimum viable plan that closes all P0/P1 findings. It is implementation-ready.
