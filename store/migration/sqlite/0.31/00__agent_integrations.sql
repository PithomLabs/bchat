CREATE TABLE IF NOT EXISTS agent_integrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    integration_type TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
    updated_at BIGINT NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_integrations_tenant ON agent_integrations(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_integrations_tenant_type ON agent_integrations(tenant_id, integration_type);
