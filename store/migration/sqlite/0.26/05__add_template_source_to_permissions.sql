-- add_template_source_to_permissions
-- Add source_template_id column to user_tenant_permission and drop the
-- unique(user_id, tenant_id) constraint so multiple template-assignment rows
-- can exist per user/tenant.

CREATE TABLE IF NOT EXISTS user_tenant_permission_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    permissions TEXT NOT NULL DEFAULT '',
    granted_by INTEGER REFERENCES user(id) ON DELETE SET NULL,
    granted_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    source_template_id INTEGER REFERENCES tenant_role_templates(id) ON DELETE SET NULL
);

INSERT INTO user_tenant_permission_new
SELECT id, user_id, tenant_id, permissions, granted_by, granted_at, NULL
FROM user_tenant_permission;

DROP TABLE user_tenant_permission;
ALTER TABLE user_tenant_permission_new RENAME TO user_tenant_permission;

CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_user ON user_tenant_permission(user_id);
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_tenant ON user_tenant_permission(tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_template ON user_tenant_permission(source_template_id);
