-- Agent Simulation Transcript Table
-- Version: 0.25.8
-- Stores simulation conversation transcripts

CREATE TABLE IF NOT EXISTS agent_simulation_transcripts (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES user(id),
    initial_prompt TEXT NOT NULL,
    persona_hint TEXT,
    total_turns INTEGER NOT NULL DEFAULT 0,
    end_reason TEXT NOT NULL DEFAULT 'unknown',
    messages TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_simulation_transcript_tenant ON agent_simulation_transcripts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_simulation_transcript_user ON agent_simulation_transcripts(user_id);
CREATE INDEX IF NOT EXISTS idx_simulation_transcript_created ON agent_simulation_transcripts(created_at);
