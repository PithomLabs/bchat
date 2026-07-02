# Plan: Tenant-Level Role Templates for Multi-Tenant RBAC (Security-Hardened)

## Goal
Add assignable tenant-level role templates (`tenant_viewer`, `tenant_editor`, `tenant_admin`, plus custom variants) that map to predefined `UserTenantPermission` bundles, enabling multi-tenant isolation without changing the global `User.Role` semantics.

## Decisions Made

| Topic | Decision |
|---|---|
| Architecture | Single shared instance (Option B) with application-layer multitenant RBAC |
| RBAC approach | Hybrid: system defaults in code + optional custom templates stored per tenant |
| Scope | Full stack: types + DB + handlers + service + tests |
| Permissions source | Reuse existing `UserTenantPermission`, `PermissionPresets`, and `ContainsPermission` |

---

## Change Summary

### 1. Types

**`server/router/api/v1/agent/role_template.go`** — new file
- `TenantRoleTemplate` struct: `ID`, `TenantID (*int32, nil = system default)`, `Name`, `Code`, `Permissions []string`, `CreatedBy *int32` (nullable), `CreatedAt`, `UpdatedAt`
- `FindTenantRoleTemplate` struct: filters for `ID`, `TenantID`, `Code`, `Name`
- `CreateTenantRoleTemplateRequest`, `UpdateTenantRoleTemplateRequest`
- `TenantRoleTemplateResponse`

**`store/role_template.go`** — new file
- Define `TenantRoleTemplate` and find structs in `store` package
- Add interface to `RBACStore`:
  - `CreateTenantRoleTemplate(ctx, *TenantRoleTemplate) (*TenantRoleTemplate, error)`
  - `GetTenantRoleTemplate(ctx, *FindTenantRoleTemplate) (*TenantRoleTemplate, error)`
  - `ListTenantRoleTemplates(ctx, *FindTenantRoleTemplate) ([]*TenantRoleTemplate, error)`
  - `UpdateTenantRoleTemplate(ctx, *TenantRoleTemplate) (*TenantRoleTemplate, error)`
  - `DeleteTenantRoleTemplate(ctx, id int32) error`
- Add `Store` wrapper methods delegating to driver

### 2. Database

**Migration**: `store/migration/<dialect>/NN__tenant_role_templates.sql`
- Table `tenant_role_templates` with `IF NOT EXISTS`
- Columns: `id`, `tenant_id (nullable, foreign key to agent_tenant)`, `name`, `code`, `permissions (text/json)`, `created_by (nullable, foreign key to users)`, `created_at`, `updated_at`
- Unique index on `(tenant_id, code)` where `tenant_id` is `NULL` for system templates
- FK: `tenant_id REFERENCES agent_tenant(id) ON DELETE CASCADE`
- FK: `created_by REFERENCES user(id) ON DELETE SET NULL`
- Backfill seeds: idempotent `INSERT OR IGNORE` (SQLite) / `ON CONFLICT DO NOTHING` (Postgres) for system-default rows: `viewer`, `tester`, `analyst`, `editor`, `tenant_admin`
- **System template protection**: `ON DELETE RESTRICT` on a generated column or application guard + unit tests

### 3. Store/Database

**`store/db/sqlite/rbac.go`** and **`store/db/postgres/rbac.go`**
- Implement all `RBACStore` methods for `tenant_role_templates`
- Use JSON marshaling/unmarshaling for `Permissions`
- All queries for system templates use explicit `WHERE tenant_id IS NULL` (never `tenant_id = ?`)
- `ListTenantRoleTemplates` splits results: system templates (visible to all authenticated tenant members) + custom templates (visible only to `tenant:admin`)

### 4. Permission Resolution — Single Source of Truth

**`server/router/api/v1/agent/permissions.go`** — update
- Add `PermTenantAdmin` to `AllPermissions` (fixes Finding 1)
- Add `SystemRoleTemplates` map initialized from existing `PermissionPresets`
- Add `GetSystemRoleTemplate(code string) ([]string, bool)`
- **New mandatory resolver** — all callers must use this:
  ```go
  ResolveEffectivePermissions(ctx, store, tenantID, userID int32) ([]string, error)
  ```
  - Returns wildcard if global role is `HOST`/`ADMIN` (with `permission_source: "global_role"`)
  - Loads `UserTenantPermission` for the user/tenant
  - Expands `tenant:admin` into all `tenant:*` via `ContainsPermission`
  - Returns deduplicated permission list with metadata (`permission_source: "tenant_template" | "explicit"`)
- **Audit all existing callers** of `GetUserTenantPermission` / `ListUserTenantPermissions` and route through `ResolveEffectivePermissions`

### 5. Handlers

**`/v1/agent/handlers.go`** — append

New endpoint handlers:
- `HandleListRoleTemplates` `GET /api/v1/agent/:slug/role-templates`
  - Returns system + tenant custom templates
  - Requires `tenant:read` (system templates visible to all; custom templates only if `tenant:admin`)
  - No `code` filter exposed in API
- `HandleCreateRoleTemplate` `POST /api/v1/agent/:slug/role-templates`
  - Creates custom template; requires `tenant:admin`
  - Rate-limited per tenant (uses existing `AgentRateLimit`)
  - Validates permissions via updated `ValidatePermissions`
- `HandleUpdateRoleTemplate` `PATCH /api/v1/agent/role-templates/:id`
  - Updates custom template; requires owning tenant `tenant:admin`
  - System templates (`tenant_id IS NULL`) protected: 403
- `HandleDeleteRoleTemplate` `DELETE /api/v1/agent/role-templates/:id`
  - Deletes custom template; system templates protected: 403
  - Blocks deletion if any user has active assignment (live-reference semantics)
- `HandleAssignRoleTemplate` `POST /api/v1/agent/:slug/role-templates/:id/assign`
  - Assigns template permissions to a user; requires `tenant:admin`
  - **Semantics: live reference** — permissions resolved at check time via resolver
  - Rate-limited per tenant
  - Invalidates `configCache` synchronously
- `HandleListUserRoles` `GET /api/v1/agent/:slug/users/:userId/roles`
  - Lists user's effective permissions and assigned template names for tenant
  - **Restricted to `tenant:admin` only** (not `tenant:read`)

### 6. Service & Cache

**`/v1/agent/service.go`** — updates
- Add `ResolveUserPermissionsForTenant(ctx, tenantID, userID int32) ([]string, error)` wrapper
- Ensure `configCache` invalidates on:
  - `UserTenantPermission` create/update/delete
  - `TenantRoleTemplate` create/update/delete
  - `HandleAssignRoleTemplate`
- Reduce TTL for admin-mutation paths or make invalidation synchronous

### 7. Route Registration

**`/v1/v1.go`** (or agent route registration file)
- Register all new role-template routes under `agent/:slug` prefix
- Apply CORS middleware to agent group (Finding R1)

### 8. Tests

- **Unit**: `/v1/agent/role_template_test.go`
  - CRUD for templates (system defaults protected, custom CRUD allowed)
  - Assignment idempotency
  - Permission expansion (`tenant:admin` → all `tenant:*`)
  - `ValidatePermissions` accepts `PermTenantAdmin`
  - `ContainsPermission` wildcard and prefix logic
- **Store tests**: SQLite + Postgres query correctness (explicit `IS NULL` handling)
- **Handler tests**: each new endpoint's required permission and unauthorized path
- **Integration**: concurrent access
  - Two admins assigning same template simultaneously
  - Template deletion during active assignment (should block/return conflict)

### 9. Frontend (light updates)

- Call `HandleListRoleTemplates` in tenant admin user management UI
- Display template name plus permission list
- Call `HandleAssignRoleTemplate` from user-role dropdown
- No template codes exposed to non-admin users

---

## Security Remediation Checklist (from review)

| Priority | Item | Status |
|---|---|---|
| P0 | Add `PermTenantAdmin` to `AllPermissions` / update `ValidatePermissions` | ✅ In Step 4 |
| P0 | Single resolver `ResolveEffectivePermissions` + audit all DB callers | ✅ In Step 4 |
| P0 | Explicit `tenant_id IS NULL` handling + DB RESTRICT for system templates | ✅ In Step 2, 3 |
| P0 | Define assignment semantics (live reference) + cascade/block rules | ✅ In Step 5 |
| P0 | Synchronous cache invalidation for admin mutations | ✅ In Step 6 |
| P1 | Restrict template listing filters (no `code` query param) | ✅ In Step 5 |
| P1 | `CreatedBy` nullable FK with `ON DELETE SET NULL` | ✅ In Step 2 |
| P1 | Idempotent seed backfill | ✅ In Step 2 |
| P1 | Restrict `HandleListUserRoles` to `tenant:admin` | ✅ In Step 5 |
| P1 | Startup check for default JWT secret `"usememos"` | ✅ New: add in `server/server.go` init |
| P2 | Rate limiting on mutation endpoints | ✅ In Step 5 |
| P2 | CORS on agent routes | ✅ In Step 7 |
| P2 | Audit logging at `Debug` level for template details | ✅ In Step 5 |
| P2 | Concurrent access integration tests | ✅ In Step 8 |

---

## Execution Order

1. `store/role_template.go` types + interface
2. DB migration + `store/db/sqlite` + `store/db/postgres` implementation
3. `permissions.go` helpers: add `PermTenantAdmin` to `AllPermissions`, resolver, defaults
4. `handlers.go` endpoints (with rate limiting, permission checks, synchronous cache invalidation)
5. Route registration + CORS on agent group
6. Startup JWT secret check in `server.go`
7. Tests (unit + store + handler + integration)
8. Frontend integration (minimal)

## Validation

1. Existing tests still pass
2. New stores compile on both SQLite and Postgres
3. Backfill creates system-default templates on migration (idempotent)
4. Assigning `tenant_admin` template grants effective `tenant:admin` resolution
5. Global `HOST`/`ADMIN` retain broad access with audit source tracking
6. `tenant:admin` effectively implies all `tenant:*` permissions via `ContainsPermission`
7. System templates cannot be deleted/updated via API
8. Cache invalidation is synchronous for admin mutations
9. `HandleListUserRoles` restricted to `tenant:admin`
10. No `code` filter exposed in list API
11. Concurrent assignment + delete tested
