-- Add allowed_tenant_ids to user for admin tenant binding (Issue #12)
-- Null means user can access all tenants; non-null restricts to listed GUIDs.

ALTER TABLE user ADD COLUMN allowed_tenant_ids TEXT DEFAULT NULL;
