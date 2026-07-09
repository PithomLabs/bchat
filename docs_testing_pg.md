# Local Postgres Testing Guide

**Date:** 2026-07-08
**Status:** Active
**Companion to:** [`docs_deployment.md`](docs_deployment.md)

---

## Overview

This document covers how to run and validate Postgres migrations against a **local Docker container** before deploying to Neon. It is a companion to the full [Testing & Deployment Guide](docs_deployment.md), focusing specifically on the local Postgres testing workflow.

| Use case | Database |
|----------|----------|
| Quick iteration, no external deps | SQLite (`task run`) |
| Validate Postgres migrations locally | Local Docker Postgres (this guide) |
| Production / staging | Neon Postgres |

---

## Prerequisites

| Requirement | Check command | Install |
|-------------|--------------|---------|
| Docker | `docker --version` | [docs.docker.com](https://docs.docker.com/get-docker/) |
| psql client | `psql --version` | `sudo apt install postgresql-client` (Ubuntu) / `brew install libpq` (macOS) |
| Port 5432 free | `lsof -i :5432` | Stop any existing Postgres container on that port |

---

## Default Credentials

| Field | Value |
|-------|-------|
| Host | `localhost` |
| Port | `5432` |
| Database | `bchat` |
| User | `bchat` |
| Password | `bchat` |
| Full URL | `postgresql://bchat:bchat@localhost:5432/bchat` |

Defined in [`scripts/docker-compose.postgres.yml`](scripts/docker-compose.postgres.yml).

---

## Quick Start

```bash
# 1. Start local Postgres container
task -t Taskfile_pg.yml postgres:start

# 2. Validate migrations
task -t Taskfile_pg.yml validate:migrations

# 3. Stop when done (data preserved)
task -t Taskfile_pg.yml postgres:stop
```

---

## What the Validation Script Does

`scripts/validate-pg-migrations.sh` runs 4 steps against the local Postgres instance:

| Step | What it does | Pass condition |
|------|-------------|----------------|
| 0 | Check database connectivity | `SELECT 1` succeeds |
| 1 | Create test DB from `LATEST.sql` | All tables created (currently 49) |
| 2 | Drop and recreate test DB | Clean slate for sequential test |
| 3 | Apply migrations in version order | All migration files apply without errors |
| 4 | Compare schemas (LATEST.sql vs migrations) | Table lists match |

**Note:** Step 4 may emit a `WARNING` about table list differences. This is expected when `LATEST.sql` contains tables not yet represented in versioned migrations. The script still passes as long as all migrations apply cleanly.

---

## Troubleshooting

### Port conflict (stale container)

Another container occupying port 5432:

```bash
# Find and remove it
docker ps -a --filter "publish=5432"
docker stop <container_name>
docker rm <container_name>

# Then start bchat-postgres
task -t Taskfile_pg.yml postgres:start
```

### psql socket error

If `psql` connects via Unix socket instead of TCP, verify the URL is passed correctly:

```bash
psql "postgresql://bchat:bchat@localhost:5432/bchat" -c "SELECT 1;"
```

### Reset database (destroy all data)

```bash
task -t Taskfile_pg.yml postgres:reset
```

---

## Commands Reference

| Command | Description |
|---------|-------------|
| `task -t Taskfile_pg.yml postgres:start` | Start local Postgres 16 container |
| `task -t Taskfile_pg.yml postgres:stop` | Stop container (data preserved) |
| `task -t Taskfile_pg.yml postgres:status` | Show container status |
| `task -t Taskfile_pg.yml postgres:logs` | Stream container logs |
| `task -t Taskfile_pg.yml postgres:reset` | Destroy and recreate database |
| `task -t Taskfile_pg.yml validate:migrations` | Validate LATEST.sql + all migrations |
| `task -t Taskfile_pg.yml fly:db-check` | Pre-deploy migration check (same as validate:migrations) |
| `task -t Taskfile_pg.yml run` | Run dev server with Postgres |
| `task -t Taskfile_pg.yml run:rag` | Run dev server with RAG + Postgres |

---

*Document Version: 1.0*
