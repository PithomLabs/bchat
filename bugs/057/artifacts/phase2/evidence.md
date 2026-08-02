# Phase 2 — 3-Node Functional Validation Evidence

Date: 2026-08-02

## Setup
- Throwaway compose `/tmp/opencode/docker-compose.crdb3.yml`: 3× `cockroachdb/cockroach:v26.2.1`
  - crdb3-1: `--locality=region=us-east-1`, port 26357
  - crdb3-2: `--locality=region=us-east-2`, port 26358
  - crdb3-3: `--locality=region=us-west-2`, port 26359
  - init container; insecure
- `cockroach node status`: 3 nodes live, v26.2.1, localities set (zone-survival mirror of Cloud)
- Created `bchat_user` + DB `bchat` (public schema has `public` CREATE/USAGE grants)
- **Note:** defaultdb vs bchat — `GRANT ON SCHEMA public` must run against the bchat DB;
  canary test confirmed bchat_user DDL works in bchat

## Experiment A — Execution Mode (harness: build/dryrun/main.go, app's exact driver stack)
- Harness uses `db.NewDBDriver(&profile.Profile{Driver:"cockroach"})` (postgres.NewCockroachDB → pgx,
  simple_protocol) + `store.MigrationFS` `migration/cockroach/LATEST.sql` — same code path as migrator.go:212
- Fresh DB (0 tables) for each mode:
  - **Mode (a) one-shot ExecContext** (current migrator behavior): `SET serial_normalization='sql_sequence';` + whole file in one Exec
    → **wall = 62.525s**
  - **Mode (b) per-statement autocommit** (148 statements):
    → **wall = 59.467s**, total_exec 59.467s, **max single statement = 656ms**, statements over 10s = **0**
- **Conclusion (Q2 answered): NO material difference** (62.5s vs 59.5s; per-statement max 656ms).
  Execution mode is NOT the bottleneck → **config-only fix holds; no migrator chunking needed**.
  The Cloud slowness (est. 25–60 min) is serverless DDL scheduling, not client-side mode.

## Job / Retry Observations
- After one-shot ExecContext returned: `succeeded 188, running 91` — all running are
  **SCHEMA CHANGE GC** (25h GC TTL by design) + system jobs (KEY VISUALIZER, MVCC STATS, etc.).
  All **NEW SCHEMA CHANGE (91) + SCHEMA CHANGE (97) succeeded**; 0 failed.
- **M3 finding:** `crdb_internal.jobs` in v26.2.1 has **NO `num_runs` column** (only `running_status`).
  Retry metric `num_runs > 1` from plan is unavailable — use `error` column / failed-job proxy instead.
  Full column list archived: job_id, job_type, description, statement, user_name, status, running_status,
  created, finished, modified, fraction_completed, high_water_timestamp, error, coordinator_id.
- App-level boot on 3-node (dev mode, mock embeddings, FORCE_REINDEX=true, port 5232):
  healthz 200 at first poll, migration_history=1, tables=57 — same as Phase 1.

## Idempotency
- One-shot re-run on populated DB: **353ms, 0 table change, 57 tables intact** ✓

## Gates
- ✓ Migration SQL correctness/completeness on 3-region topology (57 tables, history, healthz)
- ✓ Idempotent re-run proven
- ✓ Execution mode comparison (Q2): no change required
- → Phase 3 (Fly config fix)
