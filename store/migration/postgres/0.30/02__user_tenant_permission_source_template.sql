ALTER TABLE user_tenant_permission ADD COLUMN IF NOT EXISTS source_template_id INTEGER REFERENCES tenant_role_templates(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_template ON user_tenant_permission(source_template_id);

CREATE INDEX IF NOT EXISTS idx_tenant_config_tenant ON tenant_config(tenant_id);
