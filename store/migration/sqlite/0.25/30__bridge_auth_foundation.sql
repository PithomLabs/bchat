CREATE TABLE IF NOT EXISTS bridge_auth_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    key_id TEXT NOT NULL,
    label TEXT,
    secret_key_encrypted BLOB NOT NULL,
    secret_key_nonce BLOB NOT NULL CHECK(length(secret_key_nonce) = 12),
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'revoked')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_used_at INTEGER,
    revoked_at INTEGER,
    UNIQUE(tenant_id, key_id),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    CHECK(length(key_id) BETWEEN 16 AND 128),
    CHECK(
        (status = 'active' AND revoked_at IS NULL)
        OR
        (status = 'revoked' AND revoked_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_bridge_auth_keys_tenant_status ON bridge_auth_keys(tenant_id, status);

CREATE TABLE IF NOT EXISTS bridge_auth_nonces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    key_id TEXT NOT NULL,
    nonce TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    UNIQUE(tenant_id, key_id, nonce),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, key_id) REFERENCES bridge_auth_keys(tenant_id, key_id) ON DELETE CASCADE,
    CHECK(length(key_id) BETWEEN 16 AND 128),
    CHECK(length(nonce) BETWEEN 16 AND 128),
    CHECK(expires_at > created_at),
    CHECK(expires_at > timestamp)
);

CREATE INDEX IF NOT EXISTS idx_bridge_auth_nonces_tenant_key ON bridge_auth_nonces(tenant_id, key_id);
CREATE INDEX IF NOT EXISTS idx_bridge_auth_nonces_expiry ON bridge_auth_nonces(expires_at);
