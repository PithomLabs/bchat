-- Add explicit version column to agent_source_files for proper version control
-- Version is auto-calculated per tenant_id + audience_type + file_type combination
-- NOTE: Column may already exist if added manually - this migration is idempotent

-- Create index for version lookups (idempotent - uses IF NOT EXISTS)
CREATE INDEX IF NOT EXISTS idx_source_files_version ON agent_source_files(tenant_id, audience_type, file_type, version DESC);
