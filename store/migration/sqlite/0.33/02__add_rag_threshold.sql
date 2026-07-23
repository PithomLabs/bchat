-- P0: Add per-tenant RAG activation threshold
-- Allows tenant-specific override of the 30K token RAG activation threshold
ALTER TABLE tenant_config ADD COLUMN retrieval_token_threshold INTEGER NOT NULL DEFAULT 0;