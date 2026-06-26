CREATE TABLE IF NOT EXISTS agent_leads (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    transcript_id TEXT,
    name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    topic TEXT,
    location TEXT,
    detected_intent TEXT,
    status TEXT NOT NULL DEFAULT 'new',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_message_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    converted_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (transcript_id) REFERENCES agent_transcripts(id) ON DELETE SET NULL,
    CHECK (email IS NOT NULL OR phone IS NOT NULL),
    UNIQUE(tenant_id, session_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_leads_tenant_status
    ON agent_leads(tenant_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_leads_session
    ON agent_leads(tenant_id, session_id);
