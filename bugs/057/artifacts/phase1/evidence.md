# Phase 1 — Local Rehearsal Evidence

Date: 2026-08-02

## Setup
- `scripts/docker-compose.cockroach.yml` image: `cockroachdb/cockroach:v26.2.1`
- Volume wiped (`down -v`) — v25.2 store is incompatible with v26.2
  ("store last used with cockroach version v25.2 is too old for running version v26.2")
- Created `bchat_user` (insecure mode: no password — passwords rejected SQLSTATE 28P01)
- Database `bchat` owner `bchat_user`
- DSN: `postgresql://bchat_user@localhost:26257/bchat?sslmode=disable`
- Build: `task build:cockroach` → `build/memos` (87 MB)
- Run env (Phase 1 run script `/tmp/opencode/phase1-run.sh`):
  - `MEMOS_DRIVER=cockroach MEMOS_MODE=dev MEMOS_PORT=5231`
  - `RAG_PIPELINE_ENABLED=true LANCEDB_STORAGE_PROVIDER=cockroach`
  - `EMBEDDING_PROVIDER=mock FORCE_REINDEX_ON_STARTUP=true`
  - `--data /home/chaschel/Documents/go/bchat/build/data` (absolute path required;
    relative `--data build/data` resolves to `build/build/data` and panics at main.go:86)

## First Boot (clean DB, 0 tables)
- T0 03:21:3xZ launch → 03:22:04Z poll shows: **tables=57, history=1, healthz=200** (~35s total)
- `migration_history` = 1 row, version 0.35.1 (fs_version); code_version 0.34.0 —
  pre-existing "bump Version/DevVersion" warning logged
- DDL rate: whole migration ~35s locally vs ~25–60 min estimated on Cloud serverless
  → Cloud slowness is serverless-specific (config-only fix remains sufficient)
- Startup reindex ran (FORCE=true, RAG_STARTUP_REINDEX_DISABLED not set locally) and
  failed for all tenants: `relation "agent_vectors" does not exist (SQLSTATE 42P01)`.

## agent_vectors Finding (local-test artifact, NOT a Cloud blocker)
- `agent_vectors` is created at runtime by `CockroachVectorDB.Validate()`
  (vectordb_cockroach.go:81-95 `CREATE TABLE IF NOT EXISTS agent_vectors`),
  NOT by `store/migration/cockroach/LATEST.sql`.
- Startup reindex failure cause: with `EMBEDDING_PROVIDER=mock` in dev-mode, the
  vectorDB pool has no tenants registered at startup-reindex time, so
  `TenantVectorDBPool.Validate()` (vectordb_pool.go:269-280) iterates an empty map
  and never creates the table before reindex inserts.
- On Cloud (verify:production): startup reindex is disabled by
  `RAG_STARTUP_REINDEX_DISABLED=true` (H4, service.go:213/224-231) and reindex happens
  via the manual reindex endpoint, which calls `s.vectorDB.Validate(ctx)`
  (service.go:1274) after tenants are registered → table gets created. Not a blocker.
- Recorded as Bug 057 follow-up note: consider calling Validate() before startup reindex.

## Idempotency Restart
- `pkill -f 'build/memos'` (note: `pkill -f 'bin/memos'` does NOT match; use the
  binary path as seen in ps)
- Restart boot: **zero migration lines** in log, history stays 1 row, tables stay 57,
  healthz 200 at first poll (~2s after launch). Migration is idempotent. ✓
- Log evidence: `restart-boot.log` (no pre-migrate WARN, no migration lines)

## Conclusions
- C1 ✓ Migration converges and is idempotent locally.
- Cloud-only gap is the slow DDL rate on Serverless → Phase 3 config fix holds.
- Phase 1 complete. Next: Phase 2 (3-node local v26.2.1, Experiment A).
