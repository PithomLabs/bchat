-- Fix max_message_length default from 4000 to 2000 (matching LATEST.sql and Postgres).
-- Migration 0.28/01 used DEFAULT 4000 and lacked NOT NULL, so rows may be NULL or 4000.
-- NOTE: This UPDATE cannot distinguish user-set 4000 from default-origin 4000.
--       Tenants that explicitly configured max_message_length=4000 will be reset to 2000.
--       The Go layer (CreateAgentAudience) always sets this value explicitly, so the
--       SQLite schema default of 4000 is never used for new inserts.
UPDATE agent_audiences
   SET max_message_length = 2000
 WHERE max_message_length IS NULL
    OR max_message_length = 4000;
