-- =============================================================================
-- Local CockroachDB Initialization Script
-- =============================================================================
-- Usage:
--   cockroach sql --url "postgresql://root@localhost:26257/bchat?sslmode=disable" \
--     < scripts/crdb-init.sql
--
-- Idempotent: Safe to run multiple times. All SET statements are no-ops if
-- already applied. Run after crdb:up, crdb:reset, or container restart.
-- =============================================================================

-- === REQUIRED: Vector index support ===
-- Must be enabled before any CREATE VECTOR INDEX statements.
SET CLUSTER SETTING feature.vector_index.enabled = true;

-- === REQUIRED: Sequence normalization ===
-- Session-level setting. Ensures SERIAL columns use nextval() sequences
-- instead of unique_rowid().
--
-- WHY DUPLICATED WITH migrator.go?
--   crdb-init.sql  → ensures manual cockroach sql sessions get correct behavior
--   migrator.go     → guarantees migration behavior regardless of external setup
-- Both are needed: the migrator can't assume the init script was run, and the
-- init script can't assume the migrator will run before manual SQL sessions.
SET serial_normalization = 'sql_sequence';

-- === DEV-ONLY: Performance tuning for local development ===
-- These speed up migration re-runs by reducing GC and stats overhead.
-- They are NOT required for correctness. Safe to skip on Basic tier.
SET CLUSTER SETTING jobs.registry.interval.gc = '30s';
SET CLUSTER SETTING jobs.retention_time = '15s';
SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
