---

# Adversarial Review: Revised Security Plan (Role Templates)

## Overall Assessment

The revised plan correctly resolves **4 of the 5 prior P0/P1 findings** by addressing them in-line. However, the plan introduces **3 new gaps** and **3 incomplete remediations** that create exploitable conditions. I also identify **1 missing data-model requirement** that breaks the "live reference" invariant.

---

## Findings Still Open or Inadequately Closed

### Finding 1-A — `ResolveEffectivePermissions` is not wired into `hasPermission()`
**Severity:** HIGH

The plan mandates a single resolver but never updates the existing `hasPermission()` helper (lines 2157–2262 in `handlers.go`), which still calls `h.store.GetUserTenantPermission()` directly and runs raw `ContainsPermission()` on stored permissions. If any handler continues to call `hasPermission()` without routing through `ResolveEffectivePermissions`, the `tenant:admin` expansion never happens for that request.

**Attack scenario:** An admin assigns a `tenant_admin` template to a user. The user's `UserTenantPermission` stores `["tenant:admin"]`. A handler calling `hasPermission(c, tenantID, PermTenantWrite)` checks `ContainsPermission(["tenant:admin"], "tenant:write")`. Since `tenant:admin` implies all `tenant:*` prefixes, `ContainsPermission` returns `true` — so this specific path works. But a handler checking `PermChatLogs` would fail because `tenant:admin` does not imply `chat:*`, and the resolver would also not expand it. The inconsistency between stored and resolved representations remains a footgun.

**Required fix:** Show explicit pseudocode for `hasPermission()` calling `svc.ResolveUserPermissionsForTenant()` and caching the result per-request.

---

### Finding 1-B — "Live reference" semantics require a data-model change not described
**Severity:** HIGH

The plan says `HandleDeleteRoleTemplate` "blocks deletion if any user has active assignment (live-reference semantics)". But `UserTenantPermission` currently stores only `Permissions []string` — no `TemplateID` field, no linking table. Without a schema change, there is no way to determine whether a user was assigned a template or granted permissions explicitly. Any deletion check would be either:
- A full-table scan of all permissions matching the template's exact permission array (imprecise, false positives)
- Impossible

**Attack scenario:** Admin creates template `special_access` with permissions `["tenant:read", "chat:logs"]`. Admin assigns it to User A. Admin revokes User A's explicit `chat:logs` grant. Admin deletes template `special_access`. The plan's "blocks deletion if any user has active assignment" cannot distinguish between User A's assignment and their explicit grant without a `TemplateID` reference.

**Required fix:** Add `SourceTemplateID *int32` to `UserTenantPermission`, or create a new `user_role_template_assignments(user_id, tenant_id, template_id)` table. Update `HandleAssignRoleTemplate` to populate it.

---

### Finding 2 — Rate limiter reuse for admin mutations is semantically wrong
**Severity:** MEDIUM-HIGH

`AgentRateLimit` is keyed by `(tenantID, audienceType, clientIP)` and is designed for audience chat with a 1-minute rolling window. Reusing it for admin mutation endpoints has two problems:
1. `audienceType` is required but has no meaning for admin actions. Using `"admin"` hardcodes a string into the rate-limit table that collides with no other code path — this is fine but undocumented.
2. More critically, if `RateLimitRPM` is set to `60` for the tenant's external audience, the admin mutation endpoints inherit the same 60 RPM cap. A `tenant:admin` user performing legitimate bulk operations (e.g., assigning templates to 100 users) hits the rate limit.

**Attack scenario:** Attacker with `tenant:admin` rights floods `POST /role-templates` until the rate limit kicks in, then claims the tenant's legitimate admin is "rate limited" — a denial-of-service against the tenant's own admin.

**Required fix:** Use a separate `admin_mutation_rate_limit` keyed by `(tenantID, userID, clientIP)`, or at minimum use a dedicated `audienceType` like `"admin_mutation"` with its own configurable RPM.

---

### Finding 3 — System template "ON DELETE RESTRICT" does not work on SQLite without FK enforcement
**Severity:** MEDIUM

The plan says `ON DELETE RESTRICT` but the codebase uses SQLite by default. SQLite does **not** enforce foreign key constraints unless `PRAGMA foreign_keys = ON` is set per-connection. If the migration runs without this pragma, `ON DELETE RESTRICT` is silently ignored and system templates can be deleted.

**Attack scenario:** Developer runs migration, creates system templates. Later, a `DELETE FROM tenant_role_templates WHERE tenant_id IS NULL` succeeds because FK enforcement is off. The application guard might catch it, but if a future developer removes the guard (thinking the DB constraint protects them), all system templates are gone.

**Required fix:** Add `PRAGMA foreign_keys = ON` to the SQLite connection setup, and add a database migration step that verifies the pragma is active.

---

### Finding 4 — `Admin` role wildcard changes existing security semantics silently
**Severity:** MEDIUM

The plan says `ResolveEffectivePermissions` "returns wildcard if global role is HOST/ADMIN". But existing `hasPermission()` only grants `tenant:read` and `api:config` to `RoleAdmin`, not the full wildcard. Making `ResolveEffectivePermissions` return `["*"]` for ADMIN means:
- `hasPermission(c, tenantID, PermChatLogs)` would return `true` for any ADMIN, even on tenants they never explicitly accessed
- The existing implicit `tenant:read` and `api:config` checks become redundant
- Audit trails lose fidelity: an ADMIN accessing `chat:logs` on Tenant B looks identical to a HOST accessing it

**Required fix:** Clarify whether `RoleAdmin` returns `["*"]` or `[PermTenantRead, PermAPIConfig]`. If wildcard, document it and remove the redundant implicit-grant logic in `hasPermission()`. If not, the resolver should return the same two permissions.

---

### Finding 5 — `HandleListUserRoles` leaks template existence to non-admins
**Severity:** MEDIUM

The plan restricts `HandleListUserRoles` to `tenant:admin`, which is correct. But `HandleListRoleTemplates` returns system + custom templates for users with `tenant:read`. If `tenant:read` users can see system templates (which include `tenant_admin`), they learn the exact permissions the `tenant:admin` role grants, including `PermTenantAdmin` itself. This is information disclosure that facilitates targeted privilege escalation.

**Attack scenario:** A `tenant:read` user calls `GET /role-templates`, sees the `tenant_admin` system template with permission `["tenant:admin"]`, then knows exactly what permissions they need to socially-engineer an admin into granting.

**Required fix:** Return system templates with contents to `tenant:admin` only. Return only template names/descriptions (or nothing) to `tenant:read`.

---

### Finding 6 — `GetSystemRoleTemplate` is a template-code enumeration oracle
**Severity:** LOW-MEDIUM

The plan adds a public helper `GetSystemRoleTemplate(code string) ([]string, bool)`. If any handler accidentally exposes this via an API parameter (e.g., `?code=admin`), an attacker can test arbitrary strings against it. Even if the API doesn't expose it, the unexported function should still avoid timing side-channels if it ever becomes exported.

**Required fix:** Make `GetSystemRoleTemplate` unexported (`getSystemRoleTemplate`), or if exported, ensure it runs in constant time for the `bool` return path.

---

## New Attack Surface: Cache Invalidation Bypass

**Severity:** MEDIUM

`configCache` has a 5-minute TTL. The plan says mutations invalidate synchronously. But if `ResolveEffectivePermissions` is called and the result is cached at the service layer (not just `configCache`), an admin could:
1. Assign `tenant_admin` template to User X
2. Wait 1 second
3. User X makes a request before the cache invalidates
4. User X gets old permissions (no `tenant:admin`) even though the DB has been updated

The plan says "synchronous cache invalidation" but doesn't define where the resolved permissions are cached. If they're cached in `Service` memory, a multi-instance deployment (Fly.io with multiple machines) means cache invalidation on one instance doesn't propagate.

**Required fix:** Either do not cache resolved permissions (compute on each request, which is cheap for a single-user lookup), or use a distributed cache (Redis) keyed by `tenantID:userID`.

---

## Prior Findings: Which Are Now Closed

| Finding | Status |
|---------|--------|
| Finding 1 — `ValidatePermissions` rejects `tenant:admin` | **CLOSED** — `PermTenantAdmin` added to `AllPermissions` |
| Finding 2 — Dual stored/resolved representation | **PARTIAL** — resolver exists, but `hasPermission()` still bypasses it |
| Finding 3 — System template SQL NULL handling | **CLOSED** — explicit `IS NULL`, `ON DELETE RESTRICT` specified |
| Finding 4 — Concurrent assign/delete race | **PARTIAL** — semantics defined, but data model lacks `TemplateID` linkage |
| Finding 5 — Cache invalidation incomplete | **PARTIAL** — synchronous invalidation promised, but no per-request caching strategy |
| Finding 6 — Code filter enumeration | **CLOSED** — no `code` filter in API |
| Finding 7 — `CreatedBy` dangling reference | **CLOSED** — nullable FK with `ON DELETE SET NULL` |
| Finding 8 — Backfill seed idempotency | **CLOSED** — `INSERT OR IGNORE` / `ON CONFLICT DO NOTHING` |
| Finding 10 — `HandleListUserRoles` unrestricted | **CLOSED** — restricted to `tenant:admin` |
| Finding 11 — JWT secret fallback | **CLOSED** — startup check added |
| Finding 12 — Rate limiting | **OPEN** — reuses `AgentRateLimit` with wrong semantics |

---

## Additional Attack Scenarios

### Scenario 1: Template permission explosion
An admin creates a template with permissions `["*", "tenant:admin", "tenant:*", "chat:test", ...]` — all valid per `ValidatePermissions`. The resolver deduplicates, but `ContainsPermission` short-circuits on `*` (wildcard). The user with this template gets all permissions, including non-tenant ones. This is by design, but the plan should explicitly validate that wildcard-only templates are either disallowed or explicitly flagged.

### Scenario 2: Self-assignment via `tenant:admin`
A `tenant:admin` creates a template called `permanent_admin` with `["tenant:admin"]` and assigns it to themselves. They then delete the original `tenant_admin` system template (blocked — good). But they could grant themselves additional permissions like `api:config` or `chat:logs` via a custom template, bypassing the intended permission model. The plan should restrict `tenant:admin`-level templates from granting permissions beyond the `tenant:` namespace, or at least audit such grants.

### Scenario 3: `tenant_id` parameter pollution
`ListTenantRoleTemplates` accepts a `tenantID` filter. If the API layer doesn't validate that the authenticated user has access to that tenant, a user could enumerate custom templates across tenants by brute-forcing tenant IDs. The plan says "visible only to tenant:admin" for custom templates, but must ensure the handler validates tenant access before applying the filter.

---

## Prioritized Findings for Revision

| Priority | Finding | Action Required |
|----------|---------|-----------------|
| P0 | Missing `TemplateID` linkage for live-reference deletion guard | Add `SourceTemplateID *int32` to `UserTenantPermission` |
| P0 | `hasPermission()` not wired to `ResolveEffectivePermissions` | Update `hasPermission()` implementation |
| P0 | Admin role wildcard changes existing semantics | Clarify and align resolver behavior with `hasPermission()` |
| P1 | Rate limiter semantic mismatch for admin mutations | Separate rate-limit key or dedicated RPM |
| P1 | SQLite FK enforcement for `ON DELETE RESTRICT` | Add `PRAGMA foreign_keys = ON` |
| P1 | System templates visible to `tenant:read` users | Restrict system template contents to `tenant:admin` |
| P2 | Cache invalidation in multi-instance deployments | Define caching strategy: per-request compute vs distributed |
| P2 | Template permission explosion / namespace boundary | Audit `api:config`, `chat:*` grants to `tenant:admin` templates |
| P2 | Template enumeration via `tenant_id` brute-force | Validate tenant access before applying tenant filter |
