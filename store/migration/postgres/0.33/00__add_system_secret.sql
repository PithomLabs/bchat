-- Add system_secret table for encryption salt storage (missing from incremental path).
-- Fresh installs get this from LATEST.sql; this migration covers upgrade paths.
CREATE TABLE IF NOT EXISTS system_secret (
    id SERIAL PRIMARY KEY CHECK (id = 1),
    encryption_salt BYTEA NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    rotated_at BIGINT
);
