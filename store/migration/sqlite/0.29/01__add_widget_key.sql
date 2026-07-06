-- Add widget_key column to agent_tenants for edge-gate authentication.
-- NOTE: ALTER TABLE ADD COLUMN is NOT re-run-safe in SQLite.
-- The version tracker (migration_history) prevents re-runs under normal operation.
-- If re-run is needed (e.g., corrupted history), the Go migrator checks column existence first.
ALTER TABLE agent_tenants ADD COLUMN widget_key TEXT;
CREATE INDEX IF NOT EXISTS idx_agent_tenants_widget_key ON agent_tenants(widget_key);

-- Backfill existing tenants with a UUID widget key.
-- New tenants get a key via CreateAgentTenant; this covers pre-existing rows.
UPDATE agent_tenants
SET widget_key = lower(hex(randomblob(16))) || '-' || lower(hex(randomblob(4))) || '-4' || substr(lower(hex(randomblob(4))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(4))), 2) || '-' || lower(hex(randomblob(6)))
WHERE widget_key IS NULL OR widget_key = '';
