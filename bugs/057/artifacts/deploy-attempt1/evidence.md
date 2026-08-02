# Bug 057 — Deploy Attempt 1 Evidence (2026-08-02)

## Timeline

| Event | Time (UTC) |
|-------|------------|
| Machine boot (machine 860312fe920408, sjc) | 02:19:27 |
| OpenRouter key loaded | 02:19:29 |
| Pre-migrate WARN (no migration_history; expected A1) | 02:19:32 |
| Health check failed (grace 15s) | 02:19:27 (immediate) |
| Cron trigger-cron curl exit 7 (nothing listening) | 02:20:00, 02:25:00 |
| **Autostop #1** (excess capacity; ~5.5 min lifetime) | 02:25:03 |
| Restart | 02:25:16 |
| **Autostop #2** | 02:31:27 (machine stopped; 42/57 tables) |
| 3rd boot would re-run LATEST.sql idempotently | (machine currently stopped) |

## Evidence

| Item | File |
|------|------|
| Fly logs (full) | `fly-logs.txt` |
| Deploy chain log | `crdb-deploy.log` (stage 5 fly deploy timeout; healthz never 200) |
| SHOW JOBS snapshot | `show-jobs-snapshot.txt` |

## DB State (Cloud `great-goat`, database `bchat`)

- **42/57 tables** created (of 57 CREATE TABLE in cockroach LATEST.sql); 90 indexes total (83 CREATE INDEX + 7 CREATE UNIQUE INDEX)
- `migration_history` **empty** — written only at end of migration (migrator.go:241); **progress must be inferred from schema objects, not migration_history**
- **254/254 jobs succeeded, 0 running, 0 failed, 0 pending** at stop time (queue empty while stopped — NOT proof of post-restart convergence; re-checked in Phase 4)
- Job breakdown: 132 SCHEMA CHANGE + 59 NEW SCHEMA CHANGE + 59 SCHEMA CHANGE GC + 3 TYPEDESC
- `crdb_internal` restricted on Cloud serverless (SQLSTATE 42501) — retry stats local-only
- Observed DDL rate: ~10–24s per statement (jobs created ~10–14s apart), → ~25–60 min for 147 DDL statements

## Root Cause

Per-statement serverless DDL latency × 147 statements ≈ 25–60 min needed vs machine killed every ~6 min by:
- `grace_period = 15s` (fly_cockroach.toml:48) << migration time
- `auto_stop_machines = 'stop'` + `min_machines_running = 0`

/healthz registered only after migration completes (server.go:104-107) → health check can never pass mid-migration.

## Additional Finding (H4)

`RAG_STARTUP_REINDEX_DISABLED=true` (service.go:213, checked first) short-circuits `FORCE_REINDEX_ON_STARTUP=true` (service.go:224, else-if) → **no startup reindex ever runs**. Reindex is async anyway (service.go:225). healthz 200 = migration + workspace + listen; reindex is a separate milestone.
