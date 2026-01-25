-- Add retrieval mode and content tokens to tenant_config
-- retrieval_mode: "long_context" (default) or "rag" - determines KB retrieval strategy
-- content_tokens: estimated token count of KB + Policy content

ALTER TABLE tenant_config ADD COLUMN retrieval_mode TEXT DEFAULT 'long_context';
ALTER TABLE tenant_config ADD COLUMN content_tokens INTEGER DEFAULT 0;
