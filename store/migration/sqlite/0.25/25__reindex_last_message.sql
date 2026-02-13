-- Add last_message column to agent_reindex_checkpoints
ALTER TABLE agent_reindex_checkpoints ADD COLUMN last_message TEXT NOT NULL DEFAULT '';
