# Taskfile Postgres Commands Reference

**Status:** Draft (not yet tested)
**Date:** 2026-07-05
**Related Documents:**
- [DOCS_TASKFILE.MD](./DOCS_TASKFILE.MD) - SQLite Taskfile reference
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables
- [bugs/020/plan.md](../bugs/020/plan.md) - Postgres migration plan

---

## Overview

This document describes the Postgres-specific Taskfile (`Taskfile_pg.yml`) for local development against a Postgres database. It mirrors `Taskfile.yml` but replaces SQLite-specific behavior with Postgres equivalents.

All commands are invoked with `task -t Taskfile_pg.yml <command>`.

### Architecture

`Taskfile_pg.yml` uses Task v3 `includes` with `flatten: true` to share all database-agnostic build tasks from `Taskfile.yml`. Only the tasks that differ for Postgres (run, validate, fly:db-check) are overridden, plus new `postgres:*` tasks are added.

```
Taskfile_pg.yml
├── includes: Taskfile.yml (flatten)  → build, setup, fly:check, etc.
├── overrides: run, validate:migrations, fly:db-check
└── new:      postgres:start, postgres:stop, postgres:reset, etc.
```

### Prerequisites

Same as `Taskfile.yml` (Go, Node.js, Task), plus:

- **Docker** — for running local Postgres container
- **psql** (Postgres client) — for migration validation (optional, falls back to docker exec)
- **Postgres server** — running locally (via Docker or native install)

---

## Docker Compose (Local Postgres)

Start, stop, and manage a local Postgres 16 container for development.

| Command | Description |
|---------|-------------|
| `postgres:start` | Start Postgres container |
| `postgres:stop` | Stop container (data preserved) |
| `postgres:status` | Show container status |
| `postgres:logs` | Stream container logs |
| `postgres:reset` | Destroy and recreate database |

**Credentials (local dev):**

| Field | Value |
|-------|-------|
| URL | `postgresql://bchat:bchat@localhost:5432/bchat` |
| Host | `localhost` |
| Port | `5432` |
| Database | `bchat` |
| User | `bchat` |
| Password | `bchat` |

These credentials are configured in `scripts/docker-compose.postgres.yml`.

---

## Run Commands

Run the application against a Postgres database instead of SQLite.

| Command | Description |
|---------|-------------|
| `task -t Taskfile_pg.yml run` | Run dev server (Postgres) |
| `task -t Taskfile_pg.yml run:rag` | Run with RAG + Postgres |
| `task -t Taskfile_pg.yml run:testrag` | Run with RAG + forced reindex + Postgres |
| `task -t Taskfile_pg.yml run:binary` | Run pre-compiled binary with RAG + Postgres |
| `task -t Taskfile_pg.yml run:rag:l12` | Run with RAG + L12 embeddings + Postgres |

**Key differences from SQLite:**
- `DB_DRIVER=postgres` is set automatically
- `--data` flag is omitted (Postgres uses `DATABASE_URL` from `.env`, not a local file)
- `.env` is sourced to load `DATABASE_URL` and `OPENROUTER_API_KEY`

---

## Migration Validation

Postgres-specific migration validation commands:

| Command | Description |
|---------|-------------|
| `task -t Taskfile_pg.yml validate:migrations` | Validate LATEST.sql vs migration files (Postgres) |
| `task -t Taskfile_pg.yml fly:db-check` | Full pre-deployment migration check (Postgres) |

The `validate-pg-migrations.sh` script performs these checks against a running Postgres instance:
1. Tests `store/migration/postgres/LATEST.sql` creates a valid fresh database
2. Tests all migration files apply in sequence
3. Compares table lists between LATEST.sql and migrated schema

**Requires:** A running Postgres instance (use `postgres:start` first).

---

## Shared Tasks (from Taskfile.yml)

These tasks are inherited from `Taskfile.yml` via the `includes` mechanism and work identically for Postgres:

| Command | Description |
|---------|-------------|
| `task -t Taskfile_pg.yml setup` | Install dependencies |
| `task -t Taskfile_pg.yml setup:lancedb` | Download LanceDB libraries |
| `task -t Taskfile_pg.yml build` | Build frontend + backend |
| `task -t Taskfile_pg.yml build:rag` | Build with RAG support |
| `task -t Taskfile_pg.yml fly:check` | Validate env chain |
| `task -t Taskfile_pg.yml fly:pre-deploy` | All pre-deployment checks |

You can also run these with plain `task` (no `-t`) since they're database-agnostic.

---

## Environment Variables

Add to your `.env` file:

```bash
# Database driver
DB_DRIVER=postgres

# Postgres connection string (local dev via docker-compose):
DATABASE_URL=postgresql://bchat:bchat@localhost:5432/bchat

# Postgres connection string (Neon production):
# DATABASE_URL=postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require
```

---

## Quick Start

```bash
# 1. Start local Postgres
task -t Taskfile_pg.yml postgres:start

# 2. Run the app
task -t Taskfile_pg.yml run

# 3. (optional) With RAG
export OPENROUTER_API_KEY=sk-or-v1-xxx
task -t Taskfile_pg.yml run:rag

# 4. Validate migrations
task -t Taskfile_pg.yml fly:db-check

# 5. Stop Postgres when done
task -t Taskfile_pg.yml postgres:stop
```

---

## File Locations

| File | Purpose |
|------|---------|
| [`Taskfile_pg.yml`](../Taskfile_pg.yml) | Postgres Taskfile |
| [`scripts/docker-compose.postgres.yml`](../scripts/docker-compose.postgres.yml) | Postgres container config |
| [`scripts/validate-pg-migrations.sh`](../scripts/validate-pg-migrations.sh) | Migration validation |
| [`store/migration/postgres/LATEST.sql`](../store/migration/postgres/LATEST.sql) | Postgres schema |
| [`store/migration/postgres/`](../store/migration/postgres/) | Postgres migration files |

---

## Comparison: SQLite vs Postgres Commands

| Action | SQLite | Postgres |
|--------|--------|----------|
| Run dev server | `task run` | `task -t Taskfile_pg.yml run` |
| Run with RAG | `task run:rag` | `task -t Taskfile_pg.yml run:rag` |
| Validate migrations | `task validate:migrations` | `task -t Taskfile_pg.yml validate:migrations` |
| Pre-deploy DB check | `task fly:db-check` | `task -t Taskfile_pg.yml fly:db-check` |
| Start database | N/A (file-based) | `task -t Taskfile_pg.yml postgres:start` |
| Build | `task build` | `task build` (same) |

---

## Omitted Tasks

The following tasks from `Taskfile.yml` are not present in `Taskfile_pg.yml`:

| Task | Reason |
|------|--------|
| `fly:ssh:db` | SQLite-specific (opens `sqlite3` on Fly volume) |

---

*Document Version: 1.0*
*Updated: 2026-07-05* - Initial draft for Postgres Taskfile
