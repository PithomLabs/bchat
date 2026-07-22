-- Migration: add_system_secret
-- Driver: sqlite
-- Date: 2026-07-23
-- Bug: 046
--
-- Adds system_secret table for encryption salt storage.
-- Already present in LATEST.sql for fresh installs.
-- Schema must match LATEST.sql exactly (not a simplified version).

CREATE TABLE IF NOT EXISTS system_secret (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    encryption_salt BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    rotated_at BIGINT
);
