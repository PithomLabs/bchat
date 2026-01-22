-- Migration: Add agent_tenant_scripts and agent_analysis_results tables

-- SCRIPT.MD storage (tenant-level conversation flow guide)
CREATE TABLE IF NOT EXISTS agent_tenant_scripts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    summary TEXT,
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INTEGER DEFAULT 1,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_tenant_scripts_tenant ON agent_tenant_scripts(tenant_id);

-- Analysis results storage (transcript benchmark analysis)
CREATE TABLE IF NOT EXISTS agent_analysis_results (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    conversation_id TEXT NOT NULL,
    conversation_type TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    score INTEGER NOT NULL,
    grade TEXT NOT NULL,
    breakdown TEXT NOT NULL,
    issues TEXT NOT NULL,
    suggestions TEXT,
    benchmark_version TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_analysis_tenant ON agent_analysis_results(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_analysis_conversation ON agent_analysis_results(conversation_id);
CREATE INDEX IF NOT EXISTS idx_agent_analysis_created ON agent_analysis_results(created_at);
