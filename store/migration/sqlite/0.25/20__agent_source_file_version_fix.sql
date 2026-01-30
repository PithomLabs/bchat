-- Fix: Add version column that was missing from migration 12
-- Migration 12 created an index on 'version' column but never added the column itself
-- This caused "no such column: version" error when saving KB/Policy files
--
-- Required for file versioning in agent_source_files table
-- Version tracks multiple revisions of KB.MD and POLICY.MD per tenant+audience

ALTER TABLE agent_source_files ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
