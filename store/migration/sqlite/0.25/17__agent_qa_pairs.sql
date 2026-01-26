-- Q&A pairs for embedding/retrieval quality testing
CREATE TABLE IF NOT EXISTS agent_qa_pairs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    question TEXT NOT NULL,
    expected_answer TEXT NOT NULL,
    source_section TEXT,
    source_chunk_id TEXT,
    difficulty TEXT DEFAULT 'medium',
    category TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_qa_pairs_tenant ON agent_qa_pairs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_qa_pairs_category ON agent_qa_pairs(category);
CREATE INDEX IF NOT EXISTS idx_qa_pairs_active ON agent_qa_pairs(is_active);
