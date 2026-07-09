-- Relax agent_rate_limits foreign key constraint on tenant_id.
--
-- The agent_rate_limits table uses tenant_id=0 as a sentinel value for global
-- admin rate limiting (see HandleOnboard in handlers.go). The FK constraint
-- REFERENCES agent_tenants(id) prevents this because no row with id=0 exists.
-- This migration removes the FK so that sentinel tenant_id values are allowed.
--
-- This matches the equivalent SQLite migration at 0.30/00__relax_agent_rate_limits_fk.sql.

ALTER TABLE agent_rate_limits DROP CONSTRAINT IF EXISTS agent_rate_limits_tenant_id_fkey;
