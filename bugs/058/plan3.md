# Bug 058 — Plan v3: Implementation Plan

**Date:** 2026-08-03
**Status:** FINAL — Ready to implement
**Reviews incorporated:** `plan2_review_claude.md`, `plan2_review_chatgpt.md`

---

## Changes

### 1. `server/router/api/v1/agent/vectordb_cockroach.go` (line 112)

- Add `IF NOT EXISTS` to `CREATE VECTOR INDEX`
- Add TODO comment on SQLSTATE fallback

### 2. `scripts/crdb-init.sql` (new file)

- `set -e` for fail-fast
- Retry loop for container readiness
- Documented dual-purpose `serial_normalization`
- Required vs dev-only sections

### 3. `Taskfile.yml`

- Add `crdb:init` target with `--wait` on `docker compose up`
- Update `crdb:reset` to chain `crdb:init`

### 4. `crdb:verify`

- Add `SHOW JOBS` check for failed schema jobs

---

## Gate 0 (Simplified)

```
task crdb:reset          # wipe + start (waits for healthcheck) + init (chained)
    ↓
task build:backend:cockroach
    ↓
task crdb:migrate
    ↓
task crdb:verify         # P1-P6 + SHOW JOBS
    ↓
verify-production.sh     # full data path
    ↓
restart app
    ↓
verify-production.sh     # idempotency proof
```

Concurrent startup test: **optional** (reindex is admin-triggered singleton, not per-replica).
