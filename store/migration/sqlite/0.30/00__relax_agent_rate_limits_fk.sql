-- Relax the foreign-key constraint on agent_rate_limits.
--
-- The rate-limit counter table does not require strict referential integrity to
-- agent_tenants: rows are ephemeral (reset every 60s window) and orphan rows after
-- a tenant is deleted are harmless.
--
-- More importantly, the global/onboarding rate-limit "bucket" intentionally uses
-- tenant_id = 0, which has no matching agent_tenants row. With foreign_keys=ON
-- (the app enables this pragma), inserting that sentinel row failed with
-- "FOREIGN KEY constraint failed", which surfaced to callers as
-- "Rate limit check failed" and blocked tenant onboarding / KB uploads.
--
-- Rebuild the table without the REFERENCES clause.

ALTER TABLE agent_rate_limits RENAME TO agent_rate_limits_old;

CREATE TABLE agent_rate_limits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    audience_type TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    request_count INTEGER DEFAULT 0,
    window_start TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, audience_type, client_ip)
);

INSERT INTO agent_rate_limits (id, tenant_id, audience_type, client_ip, request_count, window_start)
    SELECT id, tenant_id, audience_type, client_ip, request_count, window_start
    FROM agent_rate_limits_old;

DROP TABLE agent_rate_limits_old;

CREATE INDEX IF NOT EXISTS idx_agent_rate_limits_lookup ON agent_rate_limits(tenant_id, audience_type, client_ip);
