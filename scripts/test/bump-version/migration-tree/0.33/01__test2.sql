-- Migration: test_migration
-- Driver: sqlite
-- Date: 2026-07-23
--
-- TODO: Write migration SQL here.

CREATE TABLE IF NOT EXISTS test_table_v2 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    value TEXT NOT NULL
);
