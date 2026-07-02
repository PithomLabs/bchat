# Plan: Tenant-Level Role Templates for Multi-Tenant RBAC (Security-Hardened v3)

## Goal
Add assignable tenant-level role templates (`tenant_viewer`, `tenant_editor`, `tenant_admin`, plus custom variants) that map to predefined `UserTenantPermission` bundles, enabling multi-tenant isolation without changing the global `User.Role` semantics.

## Decisions Made

| Topic | Decision |
|---|---|
| Architecture | Single shared instance (Option B) with application-layer multitenant RBAC |
| RBAC approach | Hybrid: system defaults in code + optional custom templates stored per tenant |
| Scope | Full stack: types + DB + handlers + service + tests |
| Permissions source | Reuse existing `UserTenantPermission`, `PermissionPresets`, and `ContainsPermission` |
| Assignment semantics | **Separate rows per assignment**: one `UserTenantPermission` row per template assignment with `SourceTemplateID = templateID`; explicit grants remain separate rows |
| Permission resolution | Unions ALL rows for `(user_id, tenant_id)` and returns deduped permission set |
| Resolver output | Includes `permission_source` per permission: `"global_role"` \| `"tenant_template"` \| `"explicit"` |
| ADMIN resolution | `RoleAdmin` returns `[PermTenantRead, PermAPIConfig]` only — **no wildcard** — preserves existing `hasPermission()` semantics exactly |
| Cache strategy | No per-user permission caching; resolve on each request (cheap single-user lookup) |
| System template visibility | Contents visible to **tenant:admin only**; names/descriptions visible to tenant:read |
| Rate limiting | Dedicated `admin_mutation` rate-limit key with `AdminMutationRateLimitRPM` configurable in `TenantConfig` + env default |
| DB protection | Application guard is primary protection for system templates; SQLite FK pragma enabled but not solely relied upon |

---

## Change Summary

### 1. Types

**`server/router/api/v1/agent/role_template.go`** — new file
- `TenantRoleTemplate` struct: `ID`, `TenantID (*int32, nil = system default)`, `Name`, `Code`, `Permissions []string`, `CreatedBy *int32` (nullable), `CreatedAt`, `UpdatedAt`
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
- Add to response structs if needed for listing

### 2. Database

**Migration**: `store/migration/<dialect>/NN__tenant_role_templates.sql`
- Table `tenant_role_templates` with `IF NOT EXISTS`
- Columns: `id`, `tenant_id (nullable, FK to agent_tenant)`, `name`, `code`, `permissions (text/json)`, `created_by (nullable, FK to user)`, `created_at`, `updated_at`
- Unique index on `(COALESCE(tenant_id, 0), code)` — uses sentinel `0` for system templates so NULLs don't create duplicate entries
- FK: `tenant_id REFERENCES agent_tenant(id) ON DELETE CASCADE`
- FK: `created_by REFERENCES user(id) ON DELETE SET NULL`
- **Protection**: application-level guard + unit tests; DB constraint is supplementary
- Backfill seeds: idempotent `INSERT OR IGNORE` (SQLite) / `ON CONFLICT DO NOTHING` (Postgres) for system-default rows: `viewer`, `tester`, `analyst`, `editor`, `tenant_admin`

**Migration**: `store/migration/<dialect>/NN__add_template_source_to_permissions.sql`
- Add `source_template_id` column to `user_tenant_permissions` (nullable int)
- FK: `source_template_id REFERENCES tenant_role_templates(id) ON DELETE SET NULL`
- Backfill: `NULL` for all existing rows (existing explicit grants)

### 3. SQLite FK Enforcement

**`store/db/sqlite/`** — update connection setup
- Add `PRAGMA foreign_keys = ON` to every new SQLite connection
- Add migration step that verifies pragma is active (e.g., `PRAGMA foreign_keys` returns `1`)
- **Never rely on SQLite FK enforcement alone for critical constraints** — application guard is primary protection

### 4. Permission Resolution — Single Source of Truth

**`server/router/api/v1/agent/permissions.go`** — update
- Add `PermTenantAdmin` to `AllPermissions` (fixes Finding 1)
- Add `SystemRoleTemplates` map initialized from existing `PermissionPresets`
- Add unexported `getSystemRoleTemplate(code string) ([]string, bool)`
- Define `ResolvedPermission struct { Permission string; Source string }`
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
  - `tenant:admin` prefix matching handled at check time by `ContainsPermission`
- **Update `hasPermission()`** in `handlers.go` to call `ResolveEffectivePermissions()` and cache the result per-request (e.g., in Echo context or request-local memory). This is the **only** path for permission checks.

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

**Audit all direct store calls:**
- `HandleListPermissions` (line ~2435): route through `ResolveEffectivePermissions` per user; return source metadata
- `HandleGrantPermission` (line ~2503): keep `SourceTemplateID = nil` for explicit grants
- Any other `GetUserTenantPermission` / `ListUserTenantPermissions` callers: route through resolver or explicitly document why direct access is safe

**New endpoint handlers:**
- `HandleListRoleTemplates` `GET /api/v1/agent/:slug/role-templates`
  - Validates caller has `tenant:read` for the tenant before applying any filter
  - Returns **system templates with contents only to `tenant:admin`**; `tenant:read` sees names/descriptions only
  - No `code` filter exposed in API
- `HandleCreateRoleTemplate` `POST /api/v1/agent/:slug/role-templates`
  - Creates custom template; requires `tenant:admin`
  - **Dedicated rate-limit key**: `(tenantID, "admin_mutation", clientIP)` with RPM from `TenantConfig.AdminMutationRateLimitRPM` or env default
  - Validates permissions via updated `ValidatePermissions`; **explicitly flag or reject wildcard-only templates** that bypass tenant namespace
- `HandleUpdateRoleTemplate` `PATCH /api/v1/agent/role-templates/:id`
  - Updates custom template; requires owning tenant `tenant:admin`
  - System templates (`tenant_id IS NULL` or `tenant_id = 0` sentinel) protected: 403
- `HandleDeleteRoleTemplate` `DELETE /api/v1/agent/role-templates/:id`
  - Deletes custom template; system templates protected: 403
  - **Blocks deletion if any `UserTenantPermission` has `SourceTemplateID` pointing to this template** (live-reference guard)
  - Queries `SELECT COUNT(*) FROM user_tenant_permissions WHERE source_template_id = ?`
  - **Primary protection is application guard**, not FK `ON DELETE SET NULL`
- `HandleAssignRoleTemplate` `POST /api/v1/agent/:slug/role-templates/:id/assign`
  - Assigns template permissions to a user; requires `tenant:admin`
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
- Add `AdminMutationRateLimitRPM` field to `TenantConfig` with safe default (e.g., `30`); read from env `ADMIN_MUTATION_RATE_LIMIT_RPM` if set
- Invalidate `configCache` on `TenantConfig` mutations as before

### 7. Route Registration

**`/v1/v1.go`**
- Register all new role-template routes under `agent/:slug` prefix
- Apply CORS middleware to agent group if not already present

### 8. Tests

- **Unit**: `/v1/agent/role_template_test.go`
  - CRUD for templates (system defaults protected, custom CRUD allowed)
  - Assignment creates separate row with `SourceTemplateID` set; does not merge
  - Resolver unions ALL rows for user/tenant and deduplicates
  - Permission expansion (`tenant:admin` → all `tenant:*`)
  - `ValidatePermissions` accepts `PermTenantAdmin`; documents/rejects wildcard-only templates
  - `hasPermission()` wired to resolver with per-request cache
  - ADMIN role returns `[PermTenantRead, PermAPIConfig]` only — no wildcard
  - System templates visible to `tenant:read` with names only, not contents
- **Store tests**: SQLite + Postgres
  - Explicit `IS NULL` / sentinel `0` handling for system templates
  - `SourceTemplateID` foreign key behavior
  - Seed backfill idempotency
  - Unique index `(COALESCE(tenant_id, 0), code)` prevents duplicate system templates
  - `PRAGMA foreign_keys = ON` verification
- **Handler tests**:
  - Each new endpoint's required permission and unauthorized path
  - `HandleListPermissions` returns source metadata per permission
  - `HandleListRoleTemplates`: `tenant:read` sees names only, `tenant:admin` sees contents
  - `HandleDeleteRoleTemplate`: blocks deletion when `SourceTemplateID` links exist
  - Rate limiting on mutation endpoints with dedicated key and configurable RPM
  - Tenant access validated before applying tenant filter
- **Integration**: concurrent access
  - Two admins assigning same template simultaneously (separate rows, no merge conflict)
  - Template deletion during active assignment (should return 409 conflict)

### 9. Frontend (light updates)

- Call `HandleListRoleTemplates` in tenant admin user management UI
- Display template name plus permission list (full contents only for tenant:admin)
- Call `HandleAssignRoleTemplate` from user-role dropdown
- Do not expose template codes to non-admin users

---

## Security Remediation Checklist (from review v3)

| Priority | Item | Status |
|---|---|---|
| P0 | Separate `UserTenantPermission` rows per template assignment; no merging | ✅ Step 1, 5 |
| P0 | Resolver unions ALL rows for user/tenant, returns deduped set with source | ✅ Step 4 |
| P0 | Audit ALL `GetUserTenantPermission`/`ListUserTenantPermissions` callers; route through resolver | ✅ Step 5 |
| P0 | `HandleListPermissions` returns source metadata via resolver | ✅ Step 5 |
| P0 | ADMIN role: `[PermTenantRead, PermAPIConfig]` only, no wildcard | ✅ Step 4 |
| P1 | Add `AdminMutationRateLimitRPM` to `TenantConfig` + env default | ✅ Step 6 |
| P1 | Application guard primary for system template deletion; FK `ON DELETE SET NULL` not relied upon for security | ✅ Step 5 |
| P1 | Unique index `(COALESCE(tenant_id, 0), code)` for SQLite NULL-safe uniqueness | ✅ Step 2 |
| P1 | `PRAGMA foreign_keys = ON` + verification | ✅ Step 3 |
| P1 | System template contents restricted to `tenant:admin` | ✅ Step 5 |
| P2 | No per-user permission caching — resolve on each request | ✅ Step 4, 6 |
| P2 | `getSystemRoleTemplate` unexported | ✅ Step 1, 4 |
| P2 | Wildcard-only templates flagged/rejected in `ValidatePermissions` | ✅ Step 5 |
| P2 | Validate tenant access before applying tenant filter | ✅ Step 5 |
| P2 | `HandleGrantPermission` creates explicit grant rows with `SourceTemplateID = nil` | ✅ Step 5 |

---

## Execution Order

1. `store/role_template.go` types + interface + `SourceTemplateID` on `UserTenantPermission`
2. DB migrations: `tenant_role_templates` (with COALESCE sentinel) + `user_tenant_permissions.source_template_id`
3. SQLite FK pragma + verification
4. `permissions.go`: `PermTenantAdmin` in `AllPermissions`, unexported `getSystemRoleTemplate`, `ResolveEffectivePermissions` with multi-row union and `ResolvedPermission` source metadata
5. `handlers.go`: update `hasPermission()` with per-request cache + audit all direct store calls + new endpoints with rate limiting and access controls
6. `service.go` + `TenantConfig`: add `AdminMutationRateLimitRPM`
7. Route registration + CORS
8. Tests (unit + store + handler + integration)
9. Frontend integration (minimal)

## Validation

1. Existing tests still pass
2. New stores compile on both SQLite and Postgres
3. Backfill idempotent on re-run
4. `tenant_role_templates` unique index prevents duplicate system templates even with NULL tenant_id
5. Assigning `tenant_admin` template creates separate row with `SourceTemplateID = templateID`
6. Resolver unions ALL rows for user/tenant and returns deduped `ResolvedPermission` set with source metadata
7. Global `HOST`/`ADMIN` retain intended access: HOST gets `*`, ADMIN gets `[tenant:read, api:config]`
8. `tenant:admin` effectively implies all `tenant:*` permissions via `ContainsPermission`
9. System templates cannot be deleted/updated via API
10. Cache invalidation is not needed for permissions (resolved per request)
11. `HandleListUserRoles` restricted to `tenant:admin`
12. `HandleListPermissions` returns source metadata per permission
13. No `code` filter exposed in list API
14. Dedicated rate-limit key for admin mutations with configurable RPM
15. `PRAGMA foreign_keys = ON` verified
16. Concurrent assignment + delete tested (409 conflict)
17. Explicit grants (`SourceTemplateID = nil`) are not blocked when template is deleted
