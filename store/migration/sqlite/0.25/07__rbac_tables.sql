-- RBAC Tables for Agent System
-- Adds user-tenant permissions, tenant configuration, and system secrets

-- 1. User-Tenant Permission Association
-- Stores explicit permissions granted to users for specific tenants
CREATE TABLE IF NOT EXISTS user_tenant_permission (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    permissions TEXT NOT NULL DEFAULT '',  -- Comma-separated: "tenant:read,chat:test"
    granted_by INTEGER REFERENCES user(id) ON DELETE SET NULL,
    granted_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    UNIQUE(user_id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_user ON user_tenant_permission(user_id);
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_tenant ON user_tenant_permission(tenant_id);

-- 2. Tenant Configuration (LLM settings, API keys)
-- Stores per-tenant LLM model and encrypted API key
CREATE TABLE IF NOT EXISTS tenant_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE REFERENCES agent_tenants(id) ON DELETE CASCADE,
    llm_model TEXT NOT NULL DEFAULT '',
    openrouter_api_key_encrypted BLOB,
    openrouter_api_key_nonce BLOB,
    features TEXT NOT NULL DEFAULT '{}',
    updated_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_by INTEGER REFERENCES user(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_tenant_config_tenant ON tenant_config(tenant_id);

-- 3. System Secrets (encryption salt storage)
-- Singleton table for storing the encryption salt
CREATE TABLE IF NOT EXISTS system_secret (
    id INTEGER PRIMARY KEY CHECK (id = 1),  -- Singleton constraint
    encryption_salt BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    rotated_at BIGINT
);
