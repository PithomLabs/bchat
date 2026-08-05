-- Add skill execution tables for durable automation pipeline

CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INT DEFAULT NULL,
    conversation_id VARCHAR(255) NOT NULL,
    skill_graph JSON NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    trigger_path VARCHAR(10) NOT NULL DEFAULT 'chat',
    current_node VARCHAR(255),
    checkpoint_data JSON DEFAULT (JSON_OBJECT()),
    completed_nodes JSON DEFAULT (JSON_OBJECT()),
    failed_nodes JSON DEFAULT (JSON_OBJECT()),
    error_message TEXT DEFAULT '',
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    parent_execution_id VARCHAR(36),
    claimed_at TIMESTAMP NULL,
    claimed_by VARCHAR(255),
    claim_expires_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    INDEX idx_skill_exec_tenant (tenant_id),
    INDEX idx_skill_exec_status (status),
    INDEX idx_skill_exec_conversation (conversation_id),
    INDEX idx_skill_exec_claim (status, trigger_path, claimed_at)
);

CREATE TABLE IF NOT EXISTS agent_skill_logs (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INT DEFAULT NULL,
    execution_id VARCHAR(36) NOT NULL,
    skill_name VARCHAR(255) NOT NULL,
    handler VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL,
    input JSON,
    output JSON,
    error_message TEXT,
    duration_ms INT DEFAULT 0,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    INDEX idx_skill_log_tenant (tenant_id),
    INDEX idx_skill_log_execution (execution_id),
    FOREIGN KEY (execution_id) REFERENCES agent_skill_executions(id) ON DELETE CASCADE
);
