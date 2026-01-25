-- Add simulation_human_model column to tenant_config
-- This field specifies the LLM model to use for the human role in agent simulations
ALTER TABLE tenant_config ADD COLUMN simulation_human_model TEXT DEFAULT '';
