-- tenant_role_templates
CREATE TABLE IF NOT EXISTS tenant_role_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER CHECK (tenant_id IS NULL OR tenant_id >= 1) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    permissions TEXT NOT NULL DEFAULT '[]',
    created_by INTEGER REFERENCES user(id) ON DELETE SET NULL,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    UNIQUE(tenant_id, code)
);

CREATE INDEX IF NOT EXISTS idx_tenant_role_templates_tenant ON tenant_role_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_role_templates_code ON tenant_role_templates(code);

INSERT OR IGNORE INTO tenant_role_templates (tenant_id, name, code, permissions)
VALUES
    (NULL, 'Viewer', 'viewer', '["tenant:read"]'),
    (NULL, 'Tester', 'tester', '["tenant:read","chat:test"]'),
    (NULL, 'Analyst', 'analyst', '["tenant:read","chat:logs"]'),
    (NULL, 'Editor', 'editor', '["tenant:read","tenant:write","files:upload"]'),
    (NULL, 'Tenant Admin', 'tenant_admin', '["tenant:admin"]');
