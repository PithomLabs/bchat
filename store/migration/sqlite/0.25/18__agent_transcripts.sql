-- Chat Transcript Recording
-- Persists external chat sessions for review and analysis

CREATE TABLE IF NOT EXISTS agent_transcripts (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    audience_type TEXT NOT NULL,

    -- Conversation data
    messages TEXT NOT NULL DEFAULT '[]',
    message_count INTEGER DEFAULT 0,

    -- Metadata
    client_ip TEXT,
    user_agent TEXT,

    -- Extracted info
    customer_name TEXT,
    customer_phone TEXT,
    customer_email TEXT,
    customer_location TEXT,
    detected_intent TEXT,

    -- Timestamps
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    last_message_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Completion
    is_completed INTEGER DEFAULT 0,
    completion_reason TEXT,

    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_transcripts_tenant ON agent_transcripts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_transcripts_started ON agent_transcripts(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_transcripts_audience ON agent_transcripts(tenant_id, audience_type);
CREATE INDEX IF NOT EXISTS idx_transcripts_session ON agent_transcripts(session_id);

-- Add setting to tenant_config
ALTER TABLE tenant_config ADD COLUMN record_transcripts INTEGER DEFAULT 1;
