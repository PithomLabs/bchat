-- Add resource_id column for resource-scoped memory
-- This enables cross-conversation memory sharing when OM_SCOPE=resource

ALTER TABLE agent_observations ADD COLUMN resource_id TEXT DEFAULT '';

-- Create index for resource-scoped queries
CREATE INDEX IF NOT EXISTS idx_agent_observations_resource ON agent_observations(resource_id);
