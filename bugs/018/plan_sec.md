# Plan: Tenant-Level Role Templates for Multi-Tenant RBAC (Security-Hardened v6)

## Goal
Add assignable tenant-level role templates that map to `UserTenantPermission` bundles — multi-tenant isolation on a shared instance, no global `User.Role` changes.

## Core Decisions

| Topic | Decision |
|---|---|
| Architecture | Single shared instance + application-layer multitenant RBAC |
| Templates | Hybrid: system defaults in code + optional custom templates per tenant in DB |
| Assignment model | Separate `UserTenantPermission` rows per assignment; `SourceTemplateID` tracks linkage |
| Deletion guard | App-level `SELECT COUNT(*) WHERE source_template_id = ?` blocks template deletion while assignments exist |
| Resolver | Single `ResolveEffectivePermissions()`; queries ALL rows per `(user_id, tenant_id)` via `ListUserTenantPermissions`, unions permissions, returns `[]ResolvedPermission{Permission, Source}` |
| ADMIN semantics | `RoleAdmin` → `[tenant:read, api:config]` only; **no wildcard** |
| Caching | No per-user permission cache; resolve on each request |
| Rate limiting | Dedicated `(tenantID, "admin_mutation", clientIP)` key with `AdminMutationRateLimitRPM` default `30` from `TenantConfig` |
| System templates | `tenant_id = -1` sentinel; pre-migration assertion `SELECT COUNT(*) FROM agent_tenant WHERE id <= 0` must return 0 |
| Visibility | System template **names/descriptions** visible to `tenant:read`; **contents** visible only to `tenant:admin` |
| Wildcard templates | **Rejected** in `ValidatePermissions` |
| `ResolvedPermission` location | Defined in `permissions.go` (application layer); handlers convert to response DTOs |

---

## Required Changes

### A. Types

**`server/router/api/v1/agent/role_template.go`** — new file
- `TenantRoleTemplate` struct: `ID`, `TenantID *int32` (nullable; `-1` sentinel for system), `Name`, `Code`, `Permissions []string`, `CreatedBy *int32`, `CreatedAt`, `UpdatedAt`
- `FindTenantRoleTemplate` struct
- Request/response DTOs
- **`getSystemRoleTemplate(code string) ([]string, bool)` unexported**

**`store/role_template.go`** — new file
- `TenantRoleTemplate`, `FindTenantRoleTemplate`
- `RBACStore` interface: CRUD methods
- `Store` wrapper methods

**`store/rbac.go`** — update `UserTenantPermission`
- Add `SourceTemplateID *int32` (nullable)
- Add `ResolvedPermission` response helper if needed at store layer

**`server/router/api/v1/agent/permissions.go`** — update
- `ResolvedPermission{Permission string, Source string}`
- `containsResolvedPermission([]ResolvedPermission, string) bool`
- `getSystemRoleTemplate(code string) ([]string, bool)` unexported
- `ResolveEffectivePermissions(ctx, store, tenantID, userID) ([]ResolvedPermission, error)`
- Add `PermTenantAdmin` to `AllPermissions`

**`server/router/api/v1/agent/handlers.go`** — update `hasPermission()`
- Cache `[]ResolvedPermission` per-request in Echo context
- Use `containsResolvedPermission()` for checks

### B. Database (3 migrations)

1. **`tenant_role_templates`**
   - `tenant_id integer NOT NULL DEFAULT -1`
   - `CHECK (tenant_id = -1 OR tenant_id >= 1)`
   - Unique index on `(tenant_id, code)`
   - Pre-migration assertion: `SELECT COUNT(*) FROM agent_tenant WHERE id <= 0` must return `0`
   - Idempotent backfill with `tenant_id = -1`

2. **`user_tenant_permissions.source_template_id`**
   - Nullable int column
   - Index on `(user_id, tenant_id, source_template_id)`
   - FK `ON DELETE SET NULL` (application guard is primary protection)
   - Backfill `NULL`

3. **`tenant_config.admin_mutation_rate_limit_rpm`**
   - Integer, default `30`
   - Env fallback: `ADMIN_MUTATION_RATE_LIMIT_RPM`

**`store/db/sqlite/`** — FK pragma
- `PRAGMA foreign_keys = ON` on every connection; verify in migration

### C. Store Layer Changes (P0 blockers)

**`store/db/sqlite/rbac.go`** and **`store/db/postgres/rbac.go`**
- Update `ListUserTenantPermissions` to accept optional `SourceTemplateID` filter via `FindUserTenantPermission`
- **Do NOT change `GetUserTenantPermission` semantics** — it intentionally returns the first row; the resolver will use `ListUserTenantPermissions` instead
- Add `FindUserTenantPermission.SourceTemplateID *int32`

**Resolver implementation:**
- Uses `ListUserTenantPermissions(ctx, &FindUserTenantPermission{UserID: &userID, TenantID: &tenantID})`
- Unions all returned rows
- Annotates source based on `SourceTemplateID != nil`

### D. Handlers

**`hasPermission()` update:**
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
- `HandleGrantPermission`: query `FindUserTenantPermission{UserID, TenantID, SourceTemplateID: nil}` explicitly; create new explicit-grant row if none exists; **never update a template-linked row**
- `HandleListPermissions`: route through `ResolveEffectivePermissions`; return `[]ResolvedPermission` with source metadata; keep backward-compatible `permissions []string` field
- All other `GetUserTenantPermission` / `ListUserTenantPermissions` callers: route through resolver or document why direct access is safe

**New endpoints:**
- `GET /role-templates` — system templates to `tenant:admin` for contents; `tenant:read` sees names/descriptions only
- `POST /role-templates` — `tenant:admin`; reject wildcard-only; rate-limited with `AdminMutationRateLimitRPM`
- `PATCH /role-templates/:id` — `tenant:admin`; system templates (`tenant_id = -1`) protected
- `DELETE /role-templates/:id` — blocks if `SourceTemplateID` references exist; app guard is primary
- `POST /role-templates/:id/assign` — idempotent insert; `SourceTemplateID = templateID`; rate-limited
- `GET /users/:userId/roles` — `tenant:admin` only

### E. Service & Config

**`service.go`**
- Add `ResolveUserPermissionsForTenant(ctx, tenantID, userID) ([]ResolvedPermission, error)`
- No permission cache invalidation needed

**`TenantConfig`**
- Add `AdminMutationRateLimitRPM int` default `30`
- Env fallback: `ADMIN_MUTATION_RATE_LIMIT_RPM`

### F. Route Registration

- Register role-template routes under `agent/:slug`
- CORS on agent group

### G. Tests

- **Unit**: `role_template_test.go`
  - CRUD; system templates protected
  - Assignment idempotency
  - `HandleGrantPermission` targets `source_template_id IS NULL` only
  - Resolver unions ALL rows via `ListUserTenantPermissions`
  - `containsResolvedPermission` works with `[]ResolvedPermission`
  - ADMIN returns `[tenant:read, api:config]` only
- **Store**: SQLite + Postgres
  - `FindUserTenantPermission.SourceTemplateID` filter works
  - Sentinel `-1` + `CHECK` constraint
  - Pre-migration assertion
  - `PRAGMA foreign_keys = ON`
- **Handler**:
  - `HandleListPermissions` returns source metadata + backward-compatible `permissions []string`
  - Rate limiting with dedicated key
  - Tenant access validated before filter
- **Integration**: concurrent assign + delete (409)

### H. Frontend (minimal)

- Display templates; assign from user management
- No template codes to non-admins

---

## P0 Fixes in This Revision

| Finding | Fix |
|---|---|
| `GetUserTenantPermission` truncates resolver | Resolver uses `ListUserTenantPermissions`; no store change to `GetUserTenantPermission` |
| `FindUserTenantPermission` missing `SourceTemplateID` | Add `SourceTemplateID *int32` to struct; update SQLite/Postgres query builders |

## P1 Fixes in This Revision

| Finding | Fix |
|---|---|
| `AdminMutationRateLimitRPM` location | Definitive: `TenantConfig` with default `30` + env fallback |
| `ResolvedPermission` struct location | Defined in `permissions.go`; handlers convert to DTOs |

## P2 Fixes in This Revision

| Finding | Fix |
|---|---|
| `HandleListPermissions` breaking change | Add `permissions_with_source []ResolvedPermission`; keep `permissions []string` |
| `AgentRateLimit` schema for `admin_mutation` | Verify `AudienceType` has no CHECK constraint before using `"admin_mutation"` |
| `tenant_id = -1` collision | Pre-migration assertion + `CHECK` constraint |

---

## Execution Order

1. `store/role_template.go` + `store/rbac.go` types + `SourceTemplateID`
2. DB migrations + pre-migration assertions
3. SQLite FK pragma
4. `permissions.go`: `ResolvedPermission`, `containsResolvedPermission`, `ResolveEffectivePermissions`
5. `handlers.go`: `hasPermission()` + audit all direct store calls + new endpoints
6. `service.go` + `TenantConfig` rate-limit field
7. Routes + CORS
8. Tests
9. Frontend

## Validation

1. Existing tests pass
2. Resolver uses `ListUserTenantPermissions`; ALL rows unioned
3. `HandleGrantPermission` targets `source_template_id IS NULL` only
4. `FindUserTenantPermission.SourceTemplateID` filter works in both SQLite and Postgres
5. Assignment idempotent; no duplicate rows
6. System templates (`tenant_id = -1`) protected
7. `AdminMutationRateLimitRPM` defaults to `30`
8. `PRAGMA foreign_keys = ON` verified
9. Backward-compatible `HandleListPermissions` response
10. Pre-migration assertion for `tenant_id = -1` sentinel
