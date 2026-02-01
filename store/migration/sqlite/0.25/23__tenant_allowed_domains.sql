-- Add allowed_domains column for optional domain allowlisting
-- NULL or empty means allow all domains (no restrictions)
-- When set, contains JSON array of allowed domain patterns

ALTER TABLE agent_tenants ADD COLUMN allowed_domains TEXT;
