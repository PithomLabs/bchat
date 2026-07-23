-- P0: Add user_access_token_lookup table for O(1) token lookups
-- Eliminates N+1 query pattern in selection token lookup (auth_service.go)
CREATE TABLE IF NOT EXISTS user_access_token_lookup (
    token_hash TEXT NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_access_token_lookup_user_id ON user_access_token_lookup(user_id);
