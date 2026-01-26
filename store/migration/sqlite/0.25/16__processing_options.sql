-- Add processing_options column to agent_tenants table
-- Stores JSON configuration for Format for RAG options per tenant
ALTER TABLE agent_tenants ADD COLUMN processing_options TEXT;
