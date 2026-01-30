-- Reindex checkpoint tracking for resume-from-error support
CREATE TABLE IF NOT EXISTS agent_reindex_checkpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    audience TEXT NOT NULL,
    total_chunks INTEGER NOT NULL,
    processed_chunks INTEGER NOT NULL DEFAULT 0,
    current_batch INTEGER NOT NULL DEFAULT 0,
    total_batches INTEGER NOT NULL,
    batch_size INTEGER NOT NULL DEFAULT 25,
    status TEXT NOT NULL DEFAULT 'in_progress',
    error_message TEXT,
    error_batch INTEGER,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_reindex_checkpoint_tenant_audience
ON agent_reindex_checkpoints(tenant_id, audience);
