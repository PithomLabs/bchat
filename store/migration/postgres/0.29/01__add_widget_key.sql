-- Add widget_key column to agent_tenants for edge-gate authentication.
ALTER TABLE agent_tenants ADD COLUMN widget_key TEXT;
CREATE INDEX IF NOT EXISTS idx_agent_tenants_widget_key ON agent_tenants(widget_key);

-- Backfill existing tenants with a UUID widget key.
UPDATE agent_tenants SET widget_key = gen_random_uuid()::text WHERE widget_key IS NULL OR widget_key = '';
