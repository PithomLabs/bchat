-- Migration: Add agent_learning_memory table for agent self-improvement

CREATE TABLE IF NOT EXISTS agent_learning_memory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE,

    -- Aggregated insights from analysis
    common_issues TEXT NOT NULL DEFAULT '[]',        -- JSON: frequently occurring issues
    learned_behaviors TEXT NOT NULL DEFAULT '[]',   -- JSON: specific behavioral guidance
    improvement_areas TEXT NOT NULL DEFAULT '[]',   -- JSON: categories needing attention
    pending_suggestions TEXT NOT NULL DEFAULT '[]', -- JSON: suggestions awaiting approval

    -- Metadata
    analysis_count INTEGER DEFAULT 0,               -- Number of analyses incorporated
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INTEGER DEFAULT 1,

    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_learning_memory_tenant ON agent_learning_memory(tenant_id);
