-- Tenant-specific LLM model for Generate KB/Policy (reasoning tasks)
-- Replaces the global LLM_MODEL_REASONING environment variable per tenant
-- Priority: tenant config > env var > hardcoded default (google/gemini-2.5-pro)

ALTER TABLE tenant_config ADD COLUMN reasoning_model TEXT DEFAULT '';
