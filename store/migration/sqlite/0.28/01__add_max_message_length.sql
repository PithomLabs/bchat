-- Add MaxMessageLength to agent_audience for message validation (Issue #2)

ALTER TABLE agent_audiences ADD COLUMN max_message_length INTEGER DEFAULT 4000;
