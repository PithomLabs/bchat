-- Add max_message_length to agent_audiences for message validation.
ALTER TABLE agent_audiences ADD COLUMN max_message_length INTEGER NOT NULL DEFAULT 2000;
