-- Versioned RAG index: active-version pointer + reindex checkpoint key change.

CREATE TABLE IF NOT EXISTS agent_rag_active_versions (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    audience_type TEXT NOT NULL,
    file_type TEXT NOT NULL,
    version INTEGER NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rag_active_version_lookup
ON agent_rag_active_versions(tenant_id, audience_type, file_type);

-- Extend reindex checkpoints to key on (tenant, audience, file_type, version).
ALTER TABLE agent_reindex_checkpoints ADD COLUMN IF NOT EXISTS file_type TEXT;
ALTER TABLE agent_reindex_checkpoints ADD COLUMN IF NOT EXISTS version INTEGER;

DROP INDEX IF EXISTS idx_reindex_checkpoint_tenant_audience;
CREATE UNIQUE INDEX IF NOT EXISTS idx_reindex_checkpoint_tenant_audience
ON agent_reindex_checkpoints(tenant_id, audience, file_type, version);
