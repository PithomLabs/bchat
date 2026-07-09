-- NOTE: status DEFAULT 'processing' is intentional — every insert path pre-claims.
-- A brand-new row is always "in-flight" because the insertor dispatches immediately.
-- Non-dispatch insert paths should explicitly set status='pending'.
CREATE TABLE IF NOT EXISTS agent_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    integration_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'processing',
    claimed_at BIGINT DEFAULT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT DEFAULT NULL,
    idempotency_key TEXT UNIQUE,
    created_at BIGINT NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (integration_id) REFERENCES agent_integrations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_events_tenant ON agent_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_events_status ON agent_events(status);
CREATE INDEX IF NOT EXISTS idx_agent_events_claimed ON agent_events(claimed_at);
