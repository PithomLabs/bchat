# Adversarial Plan Review: Bug 054 — Ticket Tenant Association Missing

**Bug ID:** 054
**Reviewer:** Kilo (Senior Go Architect)
**Date:** 2026-07-31
**Verdict:** REJECT WITH REWORK — two critical schema/security defects and one architectural flaw. Fixes 1, 2, and 3 require redesign before implementation.

---

## Executive Summary

The plan correctly diagnoses the root cause chain: HOST gets `nil` `tenant_id` in JWT because the sign-in and tenant-selection flows skip HOST/ADMIN. This causes `CreateTicket` to insert `tenant_id = NULL`, which skips RAG indexing and inference.

However, the proposed fixes contain a **schema error**, a **tenant-isolation bypass for scoped admins**, and an **incomplete understanding of which code paths actually affect HOST**. The plan conflates two separate bugs and proposes partially redundant fixes.

---

## Finding 1: Migration References Non-Existent Column

**Severity:** CRITICAL

Fix 1's migration inserts into `user_tenant_permission` using a `role` column:

```sql
INSERT OR IGNORE INTO user_tenant_permission (user_id, tenant_id, role, source_template_id)
SELECT 1, id, 'admin', NULL
FROM agent_tenants ...
```

The actual schema (`store/migration/sqlite/LATEST.sql:436-445`) is:

```sql
CREATE TABLE user_tenant_permission (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    permissions TEXT NOT NULL DEFAULT '',
    granted_by INTEGER REFERENCES user(id) ON DELETE SET NULL,
    granted_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    source_template_id INTEGER REFERENCES tenant_role_templates(id) ON DELETE SET NULL,
    UNIQUE(user_id, tenant_id)
);
```

There is **no `role` column**. The permission data is stored as a JSON string in `permissions` (e.g., `'["tenant:read","chat:test"]'`).

**Required fix:** Rewrite the migration to insert valid data:

```sql
-- SQLite
INSERT OR IGNORE INTO user_tenant_permission (user_id, tenant_id, permissions, granted_at)
SELECT 1, id, '["tenant:admin"]', strftime('%s', 'now')
FROM agent_tenants
WHERE id = (SELECT MIN(id) FROM agent_tenants)
AND NOT EXISTS (
    SELECT 1 FROM user_tenant_permission WHERE user_id = 1
);
```

Same correction needed for the Postgres migration.

---

## Finding 2: Fix 2 Bypasses Tenant Isolation for Scoped Admins

**Severity:** CRITICAL

Fix 2's `HandleAuthTenants` bypass:

```go
if user.Role == store.RoleHost || user.Role == store.RoleAdmin {
    allTenants, err := s.Store.ListAgentTenants(...)
    ...
}
```

This returns **all tenants** for **all** admins, including scoped admins (`RoleAdmin` with non-empty `AllowedTenantIDs`). `TenantBindingMiddleware` correctly distinguishes:

```go
if user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0) {
    // super user — bypass tenant binding
}
```

A scoped admin with `AllowedTenantIDs = ["tenant-a", "tenant-b"]` should only see those two tenants, not every tenant in the system. The plan's condition leaks all tenants to scoped admins.

**Required fix:** Use the same super-user check as `TenantBindingMiddleware`:

```go
if user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0) {
    allTenants, err := s.Store.ListAgentTenants(...)
    ...
} else {
    // existing permission-query path for scoped admins and regular users
}
```

---

## Finding 3: Fix 3 Changes HOST gRPC Sign-In Semantics

**Severity:** CRITICAL

Currently, HOST signing in via gRPC gets a JWT with `tenant_id = nil`. Several downstream paths rely on this:

- `TenantBindingMiddleware` treats nil/no-tenant requests differently for super users
- Some HOST workflows intentionally operate without a specific tenant context (instance-level admin)

Fix 3 changes `doSignIn` so HOST/ADMIN auto-select a tenant. After this change, HOST JWT always carries a specific `tenant_id`. This is a **breaking behavior change** for HOST.

The plan notes this as "better than nil" but doesn't address:
- Existing server-side checks like `if ticket.TenantID == nil` that treated HOST-created tickets specially
- The `createEscalationTicket` and similar paths that may behave differently when a tenant is present

**Required fix:** Either:
1. Document the behavior change explicitly and audit all `TenantID == nil` checks for HOST implications, OR
2. Keep `nil` for HOST in `doSignIn` and only populate it on explicit tenant selection (less invasive)

---

## Finding 4: Fix 1 and Fix 2+3 Are Redundant

**Severity:** HIGH

The plan says:

> "Steps 1-3 can be combined: Fix 2+3 are code-level fallbacks that make Fix 1 optional."

If Fix 2+3 correctly handle HOST without permission rows, then Fix 1 is purely cosmetic — it seeds data that the code now handles gracefully. The migration adds no functional value.

Conversely, if the plan intends Fix 1 to be the primary fix and Fix 2+3 as fallbacks, then Fix 2 should not bypass tenant selection for scoped admins.

**Required fix:** Choose one strategy:
- **Strategy A:** Keep Fix 1 as primary, keep Fix 2+3 as robust fallbacks, but fix Fix 2's scoped-admin bypass.
- **Strategy B:** Drop Fix 1 entirely, keep Fix 2+3 only.

Strategy B is cleaner. HOST shouldn't need permission rows by design.

---

## Finding 5: Fix 4 Doesn't Apply to HOST

**Severity:** HIGH

`handleAutoTicketCreation` at `memo_service.go:1123` is only called when `!isSuperUser(user)` (line 117). HOST/ADMIN never trigger this path.

Fix 4 propagates `memo.TenantID` to the auto-created ticket. This only benefits regular users. The reported bug is about HOST creating tickets via the web UI REST API, not auto-ticket creation.

**Required fix:** Fix 4 is a valid improvement for regular users' auto-ticket creation, but it does **not** fix the reported bug. The plan must clearly separate scope:
- Bug fix: Fix HOST JWT `tenant_id` (Fix 2+3)
- Improvement: Propagate tenant in auto-ticket creation (Fix 4)

---

## Finding 6: Fix 5 Creates Frontend/Backend Inconsistency

**Severity:** MEDIUM

Fix 5 adds `TenantID *int32` to `CreateTicketRequest`. The handler uses it only for super users:

```go
if tenantID == nil && request.TenantID != nil && isSuperUser(user) {
```

This means:
- Regular users: `tenantId` in request body is silently ignored.
- Super users: `tenantId` overrides JWT tenant.

The frontend (Fix 6) sends `tenantId` only for HOST/ADMIN. So the field is effectively write-only for super users. This is workable, but:
- The OpenAPI/schema documentation should mark this field as `admin_only` or similar.
- The handler should explicitly ignore (not silently drop) the field for non-superusers, or return 400 if present.

**Required fix:** Either validate and reject `tenantId` for non-superusers, or document it as an internal admin field.

---

## Finding 7: UI Dropdown Visibility Logic Under-Specified

**Severity:** MEDIUM

Fix 6 says:

> "Show only for users with access to multiple tenants (HOST/ADMIN)"

But after Fix 3, HOST always has a tenant in JWT. The dropdown needs to show when:
- HOST has multiple tenants (to allow switching)
- Scoped admin has multiple allowed tenants

The plan doesn't specify how the frontend determines "multiple tenants". After Fix 2, HOST can list all tenants. But scoped admins can only list allowed tenants. The frontend needs different logic for each role.

**Required fix:** Specify the exact visibility rule:
```typescript
// Show tenant dropdown if:
// - user is HOST, OR
// - user is scoped ADMIN with >1 AllowedTenantIDs
const showTenantDropdown = user.role === 'HOST' || 
  (user.role === 'ADMIN' && user.allowedTenantIds.length > 1);
```

---

## Finding 8: Plan Conflates Two Bugs

**Severity:** MEDIUM

The plan mixes two distinct issues:

| Bug | Symptom | Root Cause |
|-----|---------|------------|
| **054a** | HOST ticket has `tenant_id = NULL` | HOST gets `nil` tenant_id in JWT |
| **054b** | Auto-created ticket has `tenant_id = NULL` | `handleAutoTicketCreation` doesn't propagate `memo.TenantID` |

Bug 054a is the reported bug. Bug 054b is a separate issue that only affects non-superusers.

**Required fix:** Rename Fix 4 as a separate follow-up ticket. The bug fix should focus on HOST JWT tenant resolution.

---

## Finding 9: Missing Migration Version

**Severity:** LOW (Nit)

The plan references `store/migration/sqlite/NN__seed_host_permission.sql` without specifying the version number. The current highest version is `0.35` (`store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql`).

The new migration should be `0.36/01__seed_host_permission.sql`.

---

## Finding 10: Plan Doesn't Verify `HandleAuthTenants` Return Format

**Severity:** LOW (Nit)

Fix 2 returns `tenants` built from `ListAgentTenants`. The existing code builds `TenantInfo` from permission rows. The plan assumes the return format is identical, but doesn't verify that `ListAgentTenants` returns the same fields (`ID`, `CompanyName`, `Slug`) as `GetAgentTenant` does.

In practice they do, but the plan should confirm this assumption.

---

## Revised Implementation Order

| Step | Fix | Description | Status |
|------|-----|-------------|--------|
| 1 | Fix 2 (revised) | `HandleAuthTenants` HOST bypass — use `isSuperUser` check, not blanket `RoleAdmin` | REQUIRED |
| 2 | Fix 3 (revised) | `doSignIn` HOST tenant resolution — document behavior change or scope to explicit selection | REQUIRED |
| 3 | ~~Fix 1~~ | ~~Seed HOST permission row~~ | DROP — Fix 2+3 handle nil perms gracefully |
| 4 | Fix 4 | `handleAutoTicketCreation` tenant propagation | DEFER — separate ticket |
| 5 | Fix 5 | `CreateTicketRequest.tenantId` | DEFER — not needed if Fix 2+3 work |
| 6 | Fix 6 | UI tenant dropdown | DEFER — depends on confirmed multi-tenant UX |

**Recommended minimal fix for Bug 054:**
1. Fix `HandleAuthTenants` to return all tenants only for super users (`RoleHost` or `RoleAdmin` with empty `AllowedTenantIDs`).
2. Fix `doSignIn` to resolve tenant for HOST/ADMIN when perms exist, or auto-select first tenant when no perms exist.
3. Document the behavior change: HOST JWT now carries a specific `tenant_id` instead of `nil`.

This is a 2-file change (`auth_service.go` only) plus tests.

---

## Final Verdict

**REJECT WITH REWORK.** The diagnosis is correct, but Fix 1 contains a schema error, Fix 2 breaks tenant isolation for scoped admins, and Fix 1+2+3 are architecturally inconsistent. 

Recommended action: Redesign the fix as a minimal 2-file change focused on `HandleAuthTenants` and `doSignIn`, dropping the migration and the ticket/UI changes.
