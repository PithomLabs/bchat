CREATE TABLE IF NOT EXISTS bridge_external_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK(status IN ('active', 'closed', 'expired')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    expires_at INTEGER,
    last_seen_at INTEGER,
    UNIQUE(tenant_id, session_id),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bridge_external_sessions_tenant_status
ON bridge_external_sessions(tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_bridge_external_sessions_expiry
ON bridge_external_sessions(expires_at);

CREATE INDEX IF NOT EXISTS idx_bridge_external_sessions_tenant_session
ON bridge_external_sessions(tenant_id, session_id);

CREATE TABLE IF NOT EXISTS bridge_handoffs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    external_session_id INTEGER NOT NULL,
    handoff_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    generation INTEGER NOT NULL CHECK(generation > 0),
    routing_mode TEXT NOT NULL DEFAULT 'handoff_queued'
        CHECK(routing_mode IN ('handoff_queued', 'human_active', 'closed')),
    outcome TEXT
        CHECK(outcome IS NULL OR outcome IN ('released', 'timeout_released', 'resolved', 'rejected', 'failed', 'closed')),
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
    version INTEGER NOT NULL DEFAULT 1 CHECK(version > 0),
    harness_id TEXT,
    operator_id TEXT,
    ticket_id INTEGER,
    memo_uid TEXT,
    transition_reason TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    closed_at INTEGER,
    UNIQUE(external_session_id, generation),
    UNIQUE(tenant_id, session_id, generation),
    UNIQUE(tenant_id, handoff_id),
    FOREIGN KEY (external_session_id) REFERENCES bridge_external_sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bridge_handoffs_external_active
ON bridge_handoffs(external_session_id, active);

CREATE INDEX IF NOT EXISTS idx_bridge_handoffs_tenant_session_active
ON bridge_handoffs(tenant_id, session_id, active);

CREATE INDEX IF NOT EXISTS idx_bridge_handoffs_tenant_mode
ON bridge_handoffs(tenant_id, routing_mode);

CREATE INDEX IF NOT EXISTS idx_bridge_handoffs_tenant_handoff
ON bridge_handoffs(tenant_id, handoff_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bridge_handoffs_one_active
ON bridge_handoffs(external_session_id)
WHERE active = 1;
