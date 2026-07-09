CREATE TABLE IF NOT EXISTS agent_observations (
    session_id TEXT PRIMARY KEY REFERENCES agent_sessions(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    observation_log TEXT DEFAULT '',
    last_observed_msg_index INTEGER DEFAULT 0,
    tokens_in_log INTEGER DEFAULT 0,
    current_task TEXT,
    suggested_response TEXT,
    resource_id TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_observations_tenant ON agent_observations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_observations_resource ON agent_observations(resource_id);
