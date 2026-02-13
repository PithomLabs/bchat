-- Add current_task and suggested_response columns to agent_observations
-- These fields store the Observer's current task tracking and response hints

ALTER TABLE agent_observations ADD COLUMN current_task TEXT;
ALTER TABLE agent_observations ADD COLUMN suggested_response TEXT;
