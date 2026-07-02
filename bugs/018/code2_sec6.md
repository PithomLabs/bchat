All findings addressed. `go build ./...` and `go test ./store/... ./server/router/api/v1/agent/...` pass.

## Changes Made

### Fix 1 — Admin Mutation Rate Limiting (HIGH)
Added `checkAdminMutationRateLimit` helper in `handlers.go` and wired it into:
- `HandleCreateRoleTemplate`
- `HandleUpdateRoleTemplate`
- `HandleDeleteRoleTemplate`
- `HandleAssignRoleTemplate`

Each now returns `429 Too Many Requests` when the caller exceeds `TenantConfig.AdminMutationRateLimitRPM` via the existing `CheckRateLimit` infrastructure.

### Fix 2 — System Template Contents gated on `tenant:admin` (HIGH)
Changed `HandleListRoleTemplates`:
```go
canViewTemplateContents := h.isAdmin(c) || h.hasPermission(c, tenant.ID, PermTenantAdmin)
```
System template `permissions` arrays and custom template listings now follow `tenant:admin`, not just global HOST/ADMIN.

### Fix 3 — Explicit-grant NULL filter (MEDIUM)
Replaced the bare `int32(0)` sentinel in `HandleGrantPermission` with a typed constant:
```go
const ExplicitGrantSourceTemplate = int32(0)
```
Documented that template IDs are positive auto-increment starting at `1`.

### Fix 4 — Revoke preserves template assignments (MEDIUM)
- Added `DeleteExplicitUserTenantPermissions(ctx, userID, tenantID)` to `RBACStore`, `Driver`, SQLite, Postgres, and MySQL stubs.
- `HandleRevokePermission` now calls `DeleteExplicitUserTenantPermissions`, which targets only `source_template_id IS NULL` rows.
- Template-linked assignments remain intact after revoke.

### Fix 5 — Test Coverage
Added to `role_template_handler_test.go`:
- `tenant_admin_sees_system_template_contents` — verifies `tenant:admin` user sees permission arrays
- `revoke_preserves_template_assignments` — verifies template assignment survives an explicit grant revoke

### Frontend
No frontend changes needed for these plan-alignment fixes; the minimal integration from the prior pass remains intact.
