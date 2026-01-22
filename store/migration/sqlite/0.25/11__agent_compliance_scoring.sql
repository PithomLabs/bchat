-- Agent Compliance Audits table
CREATE TABLE IF NOT EXISTS agent_compliance_audits (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    conversation_id TEXT NOT NULL,
    conversation_type TEXT NOT NULL,
    score INTEGER NOT NULL,
    checks TEXT NOT NULL,
    overall_passed BOOLEAN NOT NULL DEFAULT 0,
    audited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_compliance_audit_tenant ON agent_compliance_audits(tenant_id);
CREATE INDEX IF NOT EXISTS idx_compliance_audit_conversation ON agent_compliance_audits(conversation_id);
CREATE INDEX IF NOT EXISTS idx_compliance_audit_score ON agent_compliance_audits(score);
CREATE INDEX IF NOT EXISTS idx_compliance_audit_date ON agent_compliance_audits(audited_at);

-- Agent Scoring Config table
CREATE TABLE IF NOT EXISTS agent_scoring_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE,
    version TEXT NOT NULL DEFAULT '1.0',
    config TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_scoring_config_tenant ON agent_scoring_config(tenant_id);
