All findings resolved. `go build ./...` and `go test ./store/... ./server/router/api/v1/agent/...` pass.

## Summary of Changes

### Fix 1 — `HandleListUserRoles` includes template identity (HIGH)
Extended `ResolvedPermission` with `TemplateID *int32` and `TemplateName *string`. `ResolveEffectivePermissions` now looks up the template name when `SourceTemplateID` is set, so the response includes full template provenance.

### Fix 2 — `HandleGrantPermission` deduplicates orphaned explicit rows (MEDIUM)
After updating the newest explicit row, the handler now deletes any additional explicit rows for the same `(user_id, tenant_id)`. This prevents table growth from pre-existing duplicates.

### Fix 3 — `DeleteUserTenantPermission` blast radius made explicit (LOW)
- Renamed the old "delete all" method to `DeleteAllUserTenantPermissions` in store, driver, SQLite, Postgres, and MySQL.
- `DeleteUserTenantPermission` now takes an `id int32` parameter for targeted single-row deletion.
- All callers updated: `HandleRevokePermission` uses `DeleteExplicitUserTenantPermissions`; dedupe loop in `HandleGrantPermission` uses the targeted `DeleteUserTenantPermission`.

### Fix 4 — Tests added
- `tenant_admin_sees_system_template_contents` — verifies `tenant:admin` sees permission arrays
- `list_user_roles_includes_template_identity` — verifies template ID/name in resolved permissions
- `grant_deduplicates_orphaned_explicit_rows` — verifies multiple explicit rows collapse to one
- `revoke_preserves_template_assignments` — verifies template rows survive explicit revoke
