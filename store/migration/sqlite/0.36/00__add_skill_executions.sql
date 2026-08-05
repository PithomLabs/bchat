-- Add skill execution tables for durable automation pipeline

CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER DEFAULT NULL,
    conversation_id TEXT NOT NULL,
    skill_graph TEXT NOT NULL,  -- JSON
    status TEXT NOT NULL DEFAULT 'pending',
    trigger_path TEXT NOT NULL DEFAULT 'chat',
    current_node TEXT,
    checkpoint_data TEXT DEFAULT '{}',  -- JSON
    completed_nodes TEXT DEFAULT '{}',  -- JSON
    failed_nodes TEXT DEFAULT '{}',     -- JSON
    error_message TEXT DEFAULT '',
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    parent_execution_id TEXT,
    claimed_at INTEGER DEFAULT 0,
    claimed_by TEXT,
    claim_expires_at INTEGER DEFAULT 0,
    created_at INTEGER DEFAULT 0,
    updated_at INTEGER DEFAULT 0,
    completed_at INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_skill_exec_tenant ON agent_skill_executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_exec_status ON agent_skill_executions(status);
CREATE INDEX IF NOT EXISTS idx_skill_exec_conversation ON agent_skill_executions(conversation_id);
CREATE INDEX IF NOT EXISTS idx_skill_exec_claim ON agent_skill_executions(status, trigger_path, claimed_at);

CREATE TABLE IF NOT EXISTS agent_skill_logs (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER DEFAULT NULL,
    execution_id TEXT NOT NULL,
    skill_name TEXT NOT NULL,
    handler TEXT NOT NULL,
    status TEXT NOT NULL,
    input TEXT,     -- JSON
    output TEXT,    -- JSON
    error_message TEXT,
    duration_ms INTEGER DEFAULT 0,
    started_at INTEGER DEFAULT 0,
    completed_at INTEGER DEFAULT 0,
    FOREIGN KEY (execution_id) REFERENCES agent_skill_executions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_skill_log_tenant ON agent_skill_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_log_execution ON agent_skill_logs(execution_id);
