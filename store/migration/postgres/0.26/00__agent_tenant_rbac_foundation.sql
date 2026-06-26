CREATE TABLE IF NOT EXISTS agent_tenants (
    id SERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    company_name TEXT NOT NULL,
    guid TEXT NOT NULL UNIQUE,
    vertical TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    processing_options TEXT,
    allowed_domains TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_audiences (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL CHECK (audience_type IN ('internal', 'external')),
    role TEXT NOT NULL,
    tone TEXT NOT NULL,
    brand_voice TEXT,
    guidelines TEXT NOT NULL DEFAULT '[]',
    emergency_phone TEXT NOT NULL DEFAULT '',
    secondary_phones TEXT NOT NULL DEFAULT '[]',
    email TEXT,
    address TEXT,
    emergency_urgency_threshold INTEGER NOT NULL DEFAULT 4,
    escalation_confidence_threshold DOUBLE PRECISION NOT NULL DEFAULT 0.85,
    rate_limit_rpm INTEGER NOT NULL DEFAULT 60,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, audience_type)
);

CREATE TABLE IF NOT EXISTS user_tenant_permission (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    permissions TEXT NOT NULL DEFAULT '',
    granted_by INTEGER REFERENCES "user"(id) ON DELETE SET NULL,
    granted_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
    UNIQUE(user_id, tenant_id)
);

CREATE TABLE IF NOT EXISTS tenant_config (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL UNIQUE REFERENCES agent_tenants(id) ON DELETE CASCADE,
    llm_model TEXT NOT NULL DEFAULT '',
    simulation_human_model TEXT NOT NULL DEFAULT '',
    reasoning_model TEXT NOT NULL DEFAULT '',
    openrouter_api_key_encrypted BYTEA,
    openrouter_api_key_nonce BYTEA,
    features JSONB NOT NULL DEFAULT '{}',
    retrieval_mode TEXT NOT NULL DEFAULT 'long_context',
    content_tokens INTEGER NOT NULL DEFAULT 0,
    record_transcripts BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
    updated_by INTEGER REFERENCES "user"(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_tenants_guid ON agent_tenants(guid);
CREATE INDEX IF NOT EXISTS idx_agent_audiences_tenant ON agent_audiences(tenant_id, audience_type);
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_user ON user_tenant_permission(user_id);
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_tenant ON user_tenant_permission(tenant_id);
