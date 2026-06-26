CREATE TABLE IF NOT EXISTS agent_messages (
    id SERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_source_lookup
    ON agent_messages(session_id, source, source_id);
CREATE INDEX IF NOT EXISTS idx_agent_messages_tenant ON agent_messages(tenant_id);
