-- Add skill execution tables for durable automation pipeline

CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id BIGINT DEFAULT NULL,
    conversation_id TEXT NOT NULL,
    skill_graph JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    trigger_path TEXT NOT NULL DEFAULT 'chat',
    current_node TEXT,
    checkpoint_data JSONB DEFAULT '{}',
    completed_nodes JSONB DEFAULT '{}',
    failed_nodes JSONB DEFAULT '{}',
    error_message TEXT DEFAULT '',
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    parent_execution_id UUID,
    claimed_at TIMESTAMPTZ,
    claimed_by TEXT,
    claim_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_skill_exec_tenant ON agent_skill_executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_exec_status ON agent_skill_executions(status);
CREATE INDEX IF NOT EXISTS idx_skill_exec_conversation ON agent_skill_executions(conversation_id);
CREATE INDEX IF NOT EXISTS idx_skill_exec_claim ON agent_skill_executions(status, trigger_path, claimed_at);

CREATE TABLE IF NOT EXISTS agent_skill_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id BIGINT DEFAULT NULL,
    execution_id UUID NOT NULL REFERENCES agent_skill_executions(id) ON DELETE CASCADE,
    skill_name TEXT NOT NULL,
    handler TEXT NOT NULL,
    status TEXT NOT NULL,
    input JSONB,
    output JSONB,
    error_message TEXT,
    duration_ms INTEGER,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_skill_log_tenant ON agent_skill_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_log_execution ON agent_skill_logs(execution_id);
