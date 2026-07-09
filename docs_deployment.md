# Testing & Deployment Guide: SQLite vs Postgres

**Date:** 2026-07-08
**Status:** Plan

---

## Overview

This document clarifies the full lifecycle — build, test, run locally, deploy — for both SQLite and Postgres (Neon) backends. The key insight: **the binary is identical**. The database driver is selected at runtime, not compile time.

| Aspect | SQLite | Postgres |
|--------|--------|----------|
| Taskfile | `Taskfile.yml` | `Taskfile_pg.yml` |
| Build command | `task build` / `task build:rag` | Same (identical binary) |
| Run command | `task run` | `task -t Taskfile_pg.yml run` |
| Driver selection | Default (`--driver sqlite`) | `MEMOS_DRIVER=postgres` env var |
| Migration validation | `task validate:migrations` | `task -t Taskfile_pg.yml validate:migrations` |
| Local DB | File: `build/data/memos_dev.db` | Remote Neon or local Docker container |
| Deploy config | `fly.toml` + `Dockerfile.s3.fly` | `fly_pg.toml` + `Dockerfile.pg.fly` |
| Fly.io volume | Required (`memos_data`) | Not needed (Neon replaces it) |

---

## Phase 1: Setup (One-Time)

| Step | SQLite | Postgres |
|------|--------|----------|
| Install deps | `task setup` | Same |
| LanceDB libs | `task setup:lancedb` | Same |
| Env file | `cp .env.example .env` + fill `OPENROUTER_API_KEY` | Same, plus add `MEMOS_DRIVER=postgres` and `DATABASE_URL` |
| Local Postgres | N/A | `task -t Taskfile_pg.yml postgres:start` |

---

## Phase 2: Build

**The build is identical for both databases.** All three drivers (sqlite, postgres, mysql) are compiled into every binary. The `rag` build tag only gates LanceDB vector DB files, not database drivers.

| Command | What it builds | Use for |
|---------|---------------|---------|
| `task build` | Frontend + Go backend (no RAG) | Quick SQLite testing |
| `task build:rag` | Frontend + Go backend with LanceDB | RAG features |
| `task build:rag:all` | Frontend + backend + widget + LanceDB | Full build |
| `task build:frontend` | React frontend only | Frontend-only iteration |
| `task build:backend` | Go binary only (depends on `validate:migrations`) | Backend-only iteration |

**Build dependency chain:** `build:backend` → `validate:migrations` → runs `validate-migrations.sh` (SQLite) or `validate-pg-migrations.sh` (Postgres, if using `Taskfile_pg.yml`).

**Note:** `Taskfile_pg.yml` inherits all build tasks from `Taskfile.yml` via `includes` with `flatten: true`. Only run and validation tasks are overridden.

---

## Phase 3: Test

There is **no `task test` command**. Tests are run via raw `go test` or `task validate:schema`.

### Schema Validation

| Command | Database | What it checks |
|---------|----------|---------------|
| `task validate:schema` | SQLite | Runs 4 specific tests: `TestSchemaValidation`, `TestAgentSourceFile`, `TestAgentTenantScript`, `TestMigrationHistoryVersion` |
| `task validate:migrations` | SQLite | Validates LATEST.sql matches migration files |
| `task -t Taskfile_pg.yml validate:migrations` | Postgres | Validates LATEST.sql + all migrations apply cleanly against running Postgres |

### Store Tests

| Command | Database | Notes |
|---------|----------|-------|
| `go test -v ./store/test/...` | SQLite (default) | Uses temp DB file, auto-migrates |
| `DRIVER=postgres DSN="postgresql://bchat:bchat@localhost:5432/bchat" go test -v ./store/test/...` | Postgres | Requires running Postgres |
| `DRIVER=mysql DSN="root@/memos_test" go test -v ./store/test/...` | MySQL | Requires running MySQL |

**Test gating:** Some tests skip based on `DRIVER` env var:
- `schema_validation_test.go` — skips unless `DRIVER=""` or `"sqlite"`
- `agent_lead_postgres_test.go` — skips unless `DRIVER=postgres`
- `bridge_postgres_cascade_test.go` — skips unless `DRIVER=postgres`

### Agent/Server Tests

| Command | Build tag | Notes |
|---------|-----------|-------|
| `go test -v ./server/router/api/v1/agent/...` | None | Runs all agent tests (no LanceDB) |
| `go test -tags rag -v ./server/router/api/v1/agent/...` | `rag` | Includes LanceDB tests |
| `go test -tags "rag integration" -v ./server/router/api/v1/agent/...` | `rag + integration` | Full integration (needs native lib + `LD_LIBRARY_PATH`) |

### Migration Validation Scripts

| Script | Database | Requires | What it checks |
|--------|----------|----------|---------------|
| `scripts/validate-migrations.sh` | SQLite | `sqlite3` CLI | LATEST.sql table count matches |
| `scripts/validate-db-migrations.sh` | SQLite | `sqlite3` CLI | Numbering, schema, sequencing (used by `fly:db-check`) |
| `scripts/validate-pg-migrations.sh` | Postgres | Running Postgres + `psql` | LATEST.sql vs migration schema equivalence |

---

## Phase 4: Run Locally

### SQLite (Taskfile.yml)

| Command | RAG | Embeddings | Storage |
|---------|-----|------------|---------|
| `task run` | No | N/A | File: `build/data/memos_dev.db` |
| `task run:rag` | Yes | `openai/text-embedding-3-small` | Local LanceDB |
| `task run:testrag` | Yes | `qwen/qwen3-embedding-8b` | Local LanceDB, force reindex |
| `task run:rag:l12` | Yes | `all-MiniLM-L12-v2` (local) | Local LanceDB |
| `task run:rag:s3` | Yes | `openai/text-embedding-3-small` | S3/Tigris |

All SQLite run commands pass `--data {{.ROOT_DIR}}/build/data` for the DB file location.

### Postgres (Taskfile_pg.yml)

| Command | RAG | Embeddings | Storage |
|---------|-----|------------|---------|
| `task -t Taskfile_pg.yml run` | No | N/A | Neon (via `DATABASE_URL`) |
| `task -t Taskfile_pg.yml run:rag` | Yes | `openai/text-embedding-3-small` | Neon + Local LanceDB |
| `task -t Taskfile_pg.yml run:testrag` | Yes | `qwen/qwen3-embedding-8b` | Neon + Local LanceDB, force reindex |
| `task -t Taskfile_pg.yml run:rag:l12` | Yes | `all-MiniLM-L12-v2` (local) | Neon + Local LanceDB |

All Postgres run commands prepend `MEMOS_DRIVER=postgres` and omit `--data` (Postgres uses `DATABASE_URL`, not a file).

### Key Differences

| Aspect | SQLite | Postgres |
|--------|--------|----------|
| `--data` flag | Passed (file path) | Omitted |
| `MEMOS_DRIVER` | Not set (defaults to `sqlite`) | Set to `postgres` |
| DB location | `build/data/memos_dev.db` | Remote Neon or local Docker |
| Startup time | Instant (file open) | ~1-3s (connection + ping) |
| Data persistence | File survives restarts | Neon: persists; Docker: persists via volume |

### Docker Compose (Postgres only)

| Command | Description |
|---------|-------------|
| `task -t Taskfile_pg.yml postgres:start` | Start local Postgres 16 container |
| `task -t Taskfile_pg.yml postgres:stop` | Stop container (data preserved) |
| `task -t Taskfile_pg.yml postgres:status` | Show container status |
| `task -t Taskfile_pg.yml postgres:logs` | Stream container logs |
| `task -t Taskfile_pg.yml postgres:reset` | Destroy and recreate database |

Local Postgres credentials: `postgresql://bchat:bchat@localhost:5432/bchat`

---

## Phase 5: Deploy to Fly.io

### SQLite Deployment

```bash
# Pre-deployment checks
task fly:pre-deploy          # Runs fly:check + fly:db-check

# Set secrets
fly secrets set OPENROUTER_API_KEY=sk-or-v1-xxx
fly secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)

# Create volume (first time only)
fly volumes create memos_data --size 1 --region ord

# Deploy
fly deploy
```

Config: `fly.toml` + `Dockerfile.s3.fly`

### Postgres Deployment

```bash
# Pre-deployment checks
task -t Taskfile_pg.yml fly:db-check    # Validates Postgres migrations
task fly:check                           # Validates env chain

# Set secrets (different app name)
fly -a appname secrets set DATABASE_URL="postgresql://..."
fly -a appname secrets set OPENROUTER_API_KEY=sk-or-v1-xxx
fly -a appname secrets set LANCEDB_S3_BUCKET="your-bucket"
# fly -a appname secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)  # optional

# Deploy (no volume needed)
fly -a appname deploy -c fly_pg.toml
```

Config: `fly_pg.toml` + `Dockerfile.pg.fly`

### Deployment Comparison

| Aspect | SQLite | Postgres |
|--------|--------|----------|
| Config file | `fly.toml` | `fly_pg.toml` |
| Dockerfile | `Dockerfile.s3.fly` | `Dockerfile.pg.fly` |
| Volume | Required (`memos_data`) | Not needed |
| `DATABASE_URL` | Not set | Set via `fly secrets set` |
| `MEMOS_DRIVER` | Not set (defaults to sqlite) | In `fly_pg.toml` `[env]` |
| App name | `appname` | `appname` (separate app) |
| Pre-deploy check | `task fly:pre-deploy` | `task -t Taskfile_pg.yml fly:db-check` |

### fly.toml Variants (Stale)

The following files are **stale/unused** and should not be used for new deployments:

| File | Status | Notes |
|------|--------|-------|
| `fly.s3.toml` | Stale | Same app as `fly.toml`, different embedding model |
| `fly.local.toml` | Stale | Uses local LanceDB storage |
| `fly_prod.toml` | Stale | Uses deepinfra embeddings, different LLM |
| `fly copy.toml` | Stale | Backup of older local config |

Only `fly.toml` (SQLite) and `fly_pg.toml` (Postgres) are active.

---

## Quick Reference: SQLite vs Postgres Commands

| Action | SQLite | Postgres |
|--------|--------|----------|
| Build | `task build:rag` | `task build:rag` (same) |
| Validate migrations | `task validate:migrations` | `task -t Taskfile_pg.yml validate:migrations` |
| Run dev server | `task run` | `task -t Taskfile_pg.yml run` |
| Run with RAG | `task run:rag` | `task -t Taskfile_pg.yml run:rag` |
| Start local DB | N/A (file-based) | `task -t Taskfile_pg.yml postgres:start` |
| Run store tests | `go test -v ./store/test/...` | `DRIVER=postgres DSN="..." go test -v ./store/test/...` |
| Validate env chain | `task fly:check` | `task fly:check` (same) |
| Pre-deploy check | `task fly:pre-deploy` | `task -t Taskfile_pg.yml fly:db-check` |
| Deploy | `fly deploy` | `fly -a appname deploy -c fly_pg.toml` |

---

*Document Version: 1.0 (Plan)*
