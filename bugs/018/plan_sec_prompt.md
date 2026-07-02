## prompt

answer the following based on your security recommendation:
For tenant-specific admin and user roles, what should we add?
Select one answer

Add new User.Role values: TENANT_ADMIN, TENANT_USER
Add RoleTenantAdmin and RoleTenantUser as User.Role values, automatically granting corresponding permissions

Create tenant-level role templates mapped to permission bundles
Keep User.Role global but add role templates/assignment over UserTenantPermission

New TenantMember / TenantRole entity
Add a TenantMember or TenantRole table scoped to tenant, separate from global User role

Custom approach: extend service/auth layer only
Reuse existing permission strings and add role-based access middleware or UI roles

Type your own answer


answer:

**Create tenant-level role templates mapped to permission bundles** — keep `User.Role` global, but introduce role templates (e.g., `tenant_viewer`, `tenant_editor`, `tenant_admin`) that map to predefined `UserTenantPermission` bundles, rather than adding new values to the global `User.Role` enum or creating separate tables.

## Why this is the right choice

- **Avoids privilege conflation.** `User.Role` is already global (`host` / `admin` / `user`) and used throughout the codebase (`isAdmin`, `RoleHost`, `RoleAdmin`). Injecting tenant-scoped roles there would make it impossible to express “admin of tenant A but not tenant B” and would break existing permission checks.
- **Reuses existing infrastructure.** The project already has `UserTenantPermission` with string-based permissions and `permissions.go` with `PermissionPresets` (`viewer`, `editor`, `tenant_admin`, etc.). Formalizing those presets as assignable role templates requires no schema changes and keeps authorization logic centralized.
- **Least privilege by default.** Role templates let you enforce the prompt’s requirement of disabling public memo visibility, open registration, and admin-only write access to workspace settings by assigning the minimal bundle per audience instead of relying on global role checks.
- **Scales to Option A isolation later.** If you move to one Fly app per customer, these same role templates continue to work without requiring a new global role hierarchy.


## prompt

Now I have a clear picture. The codebase already has PermissionPresets as unnamed Go variables in permissions.go, and UserTenantPermission already stores permission bundles per user per tenant. We need to promote these presets into formal tenant-level role templates.

1 of 1 questions

Should role templates be persisted in a new database table, kept as code constants, or both?
Select one answer

Persistent templates: new DB table (tenant-scoped, editable per customer)
New TenantRoleTemplate entity in DB. Per-tenant custom templates + system defaults.

In-code templates only: no tables, no migrations
No DB changes; add service-level helpers wrapping PermissionPresets

Hybrid: system defaults + optional custom templates
Hybrid: system defaults in code, custom templates stored per tenant in DB

Type your own answer


**Hybrid: system defaults + optional custom templates**

Ship system defaults in code for backward compatibility and zero-migration onboarding, but also store custom role templates per tenant in the database. This gives you sensible out-of-the-box behavior while supporting per-tenant customization — which is essential for a multi-tenant SaaS where different customers may need different permission bundles.


## prompt

What is the implementation scope for this RBAC plan?
Select one answer

Full stack: types + DB + handlers + service + tests
Include type definitions, store/db layer, handlers, service helpers, and tests

Backend core: types + DB + service + basic handlers
Focus on store types, DB schema/migrations, and core service logic; defer UI and some handlers

Plan only; tests and frontend deferred
Plan only; no tests or frontend work included yet

Type your own answer



