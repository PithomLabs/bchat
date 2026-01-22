-- Migration 13: Ensure tickets table schema is consistent
-- This is a verification migration that confirms the expected schema exists.
--
-- If your database is missing the 'type' or 'tags' columns, you need to either:
-- 1. Delete the database file and restart to apply LATEST.sql fresh, OR
-- 2. Run these commands manually:
--    ALTER TABLE tickets ADD COLUMN type TEXT NOT NULL DEFAULT 'TASK';
--    ALTER TABLE tickets ADD COLUMN tags TEXT NOT NULL DEFAULT '[]';
--
-- This migration is intentionally a no-op for correctly migrated databases.

-- Verify schema by creating indexes that may not exist
-- These are safe to run multiple times with IF NOT EXISTS
CREATE INDEX IF NOT EXISTS idx_tickets_type ON tickets(type);
CREATE INDEX IF NOT EXISTS idx_tickets_assignee_id ON tickets(assignee_id);
