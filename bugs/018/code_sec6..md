Implementation complete. Final validation:

```
go build ./...      # BUILD OK
go test ./store/... ./server/router/api/v1/agent/...  # PASS
tsc --noEmit         # AgentAdmin.tsx clean
```

## Summary of Changes

### Backend (Go)
**Store layer**
- `store/role_template.go` — `TenantRoleTemplate` types + `Store` wrapper methods
- `store/rbac.go` — added `SourceTemplateID` to `UserTenantPermission` / `FindUserTenantPermission`, `AdminMutationRateLimitRPM` to `TenantConfig`, role-template CRUD to `RBACStore` interface and `Store` wrapper
- `store/driver.go` — role-template methods on `Driver` interface
- `store/db/sqlite/rbac.go` — implemented role-template CRUD (system templates use `tenant_id IS NULL`), timestamp fields as `BIGINT`, `DeleteTenantRoleTemplate` blocks on active assignments, `source_template_id` filter in `ListUserTenantPermissions`, `AdminMutationRateLimitRPM` plumbing
- `store/db/postgres/rbac.go` — same parity changes (Postgres array style)
- `store/db/mysql/rbac.go` — stub implementations for interface compliance

**Migrations**
- `store/migration/sqlite/0.26/04__tenant_role_templates.sql`
- `store/migration/sqlite/0.26/05__add_template_source_to_permissions.sql`
- `store/migration/sqlite/0.26/06__add_admin_mutation_rate_limit.sql`
- `store/migration/sqlite/LATEST.sql` — synced to match

**Handlers / Service**
- `server/router/api/v1/agent/permissions.go` — added `ResolvedPermission`, `containsResolvedPermission`, `ResolveEffectivePermissions` (HOST wildcard / ADMIN tenant:read+api:config / multi-row union for others), `ValidatePermissions` rejects sole `*`, system presets aliased as `SystemRoleTemplates`
- `server/router/api/v1/agent/handlers.go` — `hasPermission` now routes through resolver + per-request cache; `HandleGrantPermission` targets only `source_template_id IS NULL` rows; `HandleListPermissions` uses resolved output with `permissions_with_source`; added `HandleListRoleTemplates`, `HandleCreateRoleTemplate`, `HandleUpdateRoleTemplate`, `HandleDeleteRoleTemplate`, `HandleAssignRoleTemplate`, `HandleListUserRoles`
- `server/router/api/v1/agent/service.go` — added `GetAdminMutationRateLimit` with env fallback, `ResolveUserPermissionsForTenant`
- `server/router/api/v1/v1.go` — registered role-template routes under ` adminGroup`

**Tests**
- `server/router/api/v1/agent/role_template_handler_test.go` — handler integration tests for list/create/assign/idempotency/roles
- `store/test/role_template_test.go` — store CRUD + delete-blocked-by-assignments

### Frontend (minimal)
- `web/src/store/v2/agentAdmin.ts` — added `TenantRoleTemplate`, `ResolvedPermission`, request types; added `roleTemplates` state + `fetchRoleTemplates`, `createRoleTemplate`, `updateRoleTemplate`, `deleteRoleTemplate`, `assignRoleTemplate` actions
- `web/src/pages/AgentAdmin.tsx` — added `RoleTemplatesSection` (list/assign/delete/create + modals), imported new types, wired `allUsers` and `roleTemplates` from store state, section gated by `canManagePermissions`
