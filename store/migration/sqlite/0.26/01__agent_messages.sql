CREATE TABLE IF NOT EXISTS agent_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_source_lookup
    ON agent_messages(session_id, source, source_id);
CREATE INDEX IF NOT EXISTS idx_agent_messages_tenant ON agent_messages(tenant_id);
