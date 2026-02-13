-- Create agent_observations table
CREATE TABLE agent_observations (
    session_id TEXT PRIMARY KEY REFERENCES agent_sessions(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    
    -- The Observation State
    observation_log TEXT DEFAULT '',
    last_observed_msg_index INTEGER DEFAULT 0,
    
    -- Metrics
    tokens_in_log INTEGER DEFAULT 0,
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for tenant lookup (though PK is session_id, tenant_id is useful for cleanups)
CREATE INDEX idx_observations_tenant ON agent_observations(tenant_id);
