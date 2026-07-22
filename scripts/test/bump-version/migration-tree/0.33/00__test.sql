-- Migration: test_migration
-- Driver: sqlite
-- Date: 2026-07-23
--
-- TODO: Write migration SQL here.

CREATE TABLE IF NOT EXISTS test_table (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL
);
