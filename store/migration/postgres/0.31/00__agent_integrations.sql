CREATE TABLE IF NOT EXISTS agent_integrations (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    integration_type TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    updated_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_integrations_tenant ON agent_integrations(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_integrations_tenant_type ON agent_integrations(tenant_id, integration_type);
