# Plan: Tenant-Level Role Templates for Multi-Tenant RBAC (Security-Hardened v2)

## Goal
Add assignable tenant-level role templates (`tenant_viewer`, `tenant_editor`, `tenant_admin`, plus custom variants) that map to predefined `UserTenantPermission` bundles, enabling multi-tenant isolation without changing the global `User.Role` semantics.

## Decisions Made

| Topic | Decision |
|---|---|
| Architecture | Single shared instance (Option B) with application-layer multitenant RBAC |
| RBAC approach | Hybrid: system defaults in code + optional custom templates stored per tenant |
| Scope | Full stack: types + DB + handlers + service + tests |
| Permissions source | Reuse existing `UserTenantPermission`, `PermissionPresets`, and `ContainsPermission` |
| Assignment semantics | **Live reference** with explicit `TemplateID` linkage via `UserTenantPermission` update |
| ADMIN resolution | `RoleAdmin` returns `[PermTenantRead, PermAPIConfig]` only — **no wildcard** — preserves existing semantics |
| Cache strategy | No per-user permission caching; resolve on each request (cheap single-user lookup) |
| System template visibility | Contents visible to **tenant:admin only**; names/descriptions visible to tenant:read |

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
- This field is set when permissions come from a template assignment, `nil` when explicitly granted

### 2. Database

**Migration**: `store/migration/<dialect>/NN__tenant_role_templates.sql`
- Table `tenant_role_templates` with `IF NOT EXISTS`
- Columns: `id`, `tenant_id (nullable, FK to agent_tenant)`, `name`, `code`, `permissions (text/json)`, `created_by (nullable, FK to user)`, `created_at`, `updated_at`
- Unique index on `(tenant_id, code)` where `tenant_id` is `NULL` for system templates
- FK: `tenant_id REFERENCES agent_tenant(id) ON DELETE CASCADE`
- FK: `created_by REFERENCES user(id) ON DELETE SET NULL`
- **Protection**: application-level guard + unit tests; DB `ON DELETE RESTRICT` is **not relied upon for SQLite** (see Step 3)
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
- **New mandatory resolver**:
  ```go
  ResolveEffectivePermissions(ctx, store, tenantID, userID int32) ([]string, error)
  ```
  - For `RoleHost`: returns `["*"]` with `permission_source: "global_role"`
  - For `RoleAdmin`: returns `[PermTenantRead, PermAPIConfig]` with `permission_source: "global_role"` — **no wildcard, matches existing `hasPermission()` semantics exactly**
  - For `RoleUser`/others: loads `UserTenantPermission` for the user/tenant
  - Expands `tenant:admin` into all `tenant:*` prefix matches via `ContainsPermission`
  - Returns deduplicated permission list with `permission_source` metadata (`"global_role" | "tenant_template" | "explicit"`)
- **Update `hasPermission()`** in `handlers.go` to call `ResolveEffectivePermissions()` and cache the result per-request (e.g., in Echo context or request-local memory). This is the **only** path for permission checks.

### 5. Handlers

**`/v1/agent/handlers.go`** — update `hasPermission()` + append new handlers

**Update `hasPermission()`:**
```go
func (h *Handler) hasPermission(c echo.Context, tenantID int32, permission string) bool {
    // Use per-request cache if available
    if cached, ok := c.Get("resolved_perms").([]string); ok {
        return ContainsPermission(cached, permission)
    }
    
    perms, err := ResolveEffectivePermissions(c.Request().Context(), h.store, tenantID, userID)
    if err != nil {
        slog.Warn("permission resolution failed", ...)
        return false
    }
    
    c.Set("resolved_perms", perms)
    return ContainsPermission(perms, permission)
}
```

**New endpoint handlers:**
- `HandleListRoleTemplates` `GET /api/v1/agent/:slug/role-templates`
  - Validates caller has `tenant:read` for the tenant before applying any filter
  - Returns **system templates with contents only to `tenant:admin`**; `tenant:read` sees names/descriptions only
  - No `code` filter exposed in API
- `HandleCreateRoleTemplate` `POST /api/v1/agent/:slug/role-templates`
  - Creates custom template; requires `tenant:admin`
  - **Dedicated rate-limit key**: `(tenantID, "admin_mutation", clientIP)` with separate configurable RPM via `TenantConfig` or env default
  - Validates permissions via updated `ValidatePermissions`
- `HandleUpdateRoleTemplate` `PATCH /api/v1/agent/role-templates/:id`
  - Updates custom template; requires owning tenant `tenant:admin`
  - System templates (`tenant_id IS NULL`) protected: 403
- `HandleDeleteRoleTemplate` `DELETE /api/v1/agent/role-templates/:id`
  - Deletes custom template; system templates protected: 403
  - **Blocks deletion if any `UserTenantPermission` has `SourceTemplateID` pointing to this template** (live-reference guard)
  - Queries `SELECT COUNT(*) FROM user_tenant_permissions WHERE source_template_id = ?`
- `HandleAssignRoleTemplate` `POST /api/v1/agent/:slug/role-templates/:id/assign`
  - Assigns template permissions to a user; requires `tenant:admin`
  - **Live reference semantics**: sets `SourceTemplateID` on `UserTenantPermission` to the template ID
  - Merges template permissions with existing explicit grants (deduped)
  - Dedicated rate-limit key: `(tenantID, "admin_mutation", clientIP)`
  - **No caching of resolved permissions** — computed per request
- `HandleListUserRoles` `GET /api/v1/agent/:slug/users/:userId/roles`
  - Lists user's effective permissions and assigned template names for tenant
  - **Restricted to `tenant:admin` only** (not `tenant:read`)

### 6. Service

**`/v1/agent/service.go`** — updates
- Add `ResolveUserPermissionsForTenant(ctx, tenantID, userID int32) ([]string, error)` wrapper
- **No configCache invalidation needed for permission changes** — permissions are resolved on each request
- If `configCache` is used elsewhere for tenant config, invalidate on `UserTenantPermission` and `TenantRoleTemplate` mutations as before

### 7. Route Registration

**`/v1/v1.go`**
- Register all new role-template routes under `agent/:slug` prefix
- Apply CORS middleware to agent group if not already present

### 8. Tests

- **Unit**: `/v1/agent/role_template_test.go`
  - CRUD for templates (system defaults protected, custom CRUD allowed)
  - Assignment idempotency and `SourceTemplateID` linkage
  - Permission expansion (`tenant:admin` → all `tenant:*`)
  - `ValidatePermissions` accepts `PermTenantAdmin`
  - `hasPermission()` wired to resolver (not bypassed)
  - ADMIN role returns `[PermTenantRead, PermAPIConfig]` only — no wildcard
  - System templates visible to `tenant:read` with names only, not contents
- **Store tests**: SQLite + Postgres
  - Explicit `IS NULL` handling for system templates
  - `SourceTemplateID` foreign key behavior
  - Seed backfill idempotency
  - `PRAGMA foreign_keys = ON` verification
- **Handler tests**:
  - Each new endpoint's required permission and unauthorized path
  - `HandleListRoleTemplates`: `tenant:read` sees names only, `tenant:admin` sees contents
  - `HandleDeleteRoleTemplate`: blocks deletion when `SourceTemplateID` links exist
  - Rate limiting on mutation endpoints with dedicated key
- **Integration**: concurrent access
  - Two admins assigning same template simultaneously
  - Template deletion during active assignment (should return 409 conflict)

### 9. Frontend (light updates)

- Call `HandleListRoleTemplates` in tenant admin user management UI
- Display template name plus permission list (full contents only for tenant:admin)
- Call `HandleAssignRoleTemplate` from user-role dropdown
- Do not expose template codes to non-admin users

---

## Security Remediation Checklist (from review v2)

| Priority | Item | Status |
|---|---|---|
| P0 | Wire `hasPermission()` through `ResolveEffectivePermissions` with per-request cache | ✅ Step 4, 5 |
| P0 | Add `SourceTemplateID *int32` to `UserTenantPermission` for live-reference guard | ✅ Step 1, 2 |
| P0 | Clarify ADMIN role resolution: `[PermTenantRead, PermAPIConfig]`, no wildcard | ✅ Step 4 |
| P1 | Separate rate-limit key `(tenantID, "admin_mutation", clientIP)` | ✅ Step 5 |
| P1 | `PRAGMA foreign_keys = ON` + verification in SQLite migration | ✅ Step 3 |
| P1 | System template contents restricted to `tenant:admin` | ✅ Step 5 |
| P2 | No per-user permission caching — resolve on each request | ✅ Step 4, 6 |
| P2 | `getSystemRoleTemplate` unexported | ✅ Step 1, 4 |
| P2 | Template permission explosion: document that wildcard templates bypass tenant namespace | ✅ Step 5 validation |
| P2 | Validate tenant access before applying `tenant_id` filter in `ListTenantRoleTemplates` | ✅ Step 5 |

---

## Execution Order

1. `store/role_template.go` types + interface + `SourceTemplateID` on `UserTenantPermission`
2. DB migrations: `tenant_role_templates` + `user_tenant_permissions.source_template_id`
3. SQLite FK pragma + verification
4. `permissions.go`: `PermTenantAdmin` in `AllPermissions`, unexported `getSystemRoleTemplate`, `ResolveEffectivePermissions`
5. `handlers.go`: update `hasPermission()` + new endpoints with rate limiting and access controls
6. Route registration + CORS
7. Tests (unit + store + handler + integration)
8. Frontend integration (minimal)

## Validation

1. Existing tests still pass
2. New stores compile on both SQLite and Postgres
3. Backfill idempotent on re-run
4. Assigning `tenant_admin` template grants effective `tenant:admin` resolution
5. `hasPermission()` routes through resolver — no direct `ContainsPermission` bypass
6. `RoleAdmin` returns `[PermTenantRead, PermAPIConfig]` — no wildcard, matches legacy behavior
7. System templates cannot be deleted/updated via API; `tenant:read` sees names only
8. `HandleDeleteRoleTemplate` blocks when `SourceTemplateID` links exist
9. `HandleListUserRoles` restricted to `tenant:admin`
10. No `code` filter exposed in list API
11. Dedicated rate-limit key for admin mutations
12. `PRAGMA foreign_keys = ON` verified
13. Concurrent assignment + delete tested (409 conflict)
