# Neon PostgreSQL Setup Guide

**Status:** Ready to use (Postgres driver fully implemented)
**Date:** 2026-07-08

---

## Overview

PostgreSQL support is **already fully implemented** in the codebase. The driver lives in `store/db/postgres/` (24 Go files, ~4000+ lines), with complete agent, bridge, and RBAC implementations. No code changes are needed.

This guide covers connecting to a **Neon serverless Postgres** database.

---

## Step 1: Update `.env` File

Add two variables to your `.env` file:

```bash
# Database driver (overrides default "sqlite")
MEMOS_DRIVER=postgres

# Neon connection string (replace with your actual connection string)
DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require"
```

**Important:** The env var must be `MEMOS_DRIVER` (not `DB_DRIVER`) because viper uses a `MEMOS_` prefix with `AutomaticEnv()` (`bin/memos/main.go:167`). The `DB_DRIVER` used in `Taskfile_pg.yml` is a known bug that doesn't actually work via env vars.

---

## Step 2: Verify Connection

Build and run the backend:

```bash
task build:backend
MEMOS_DRIVER=postgres ./build/memos --mode dev
```

Or use the Postgres Taskfile (fix the env var first, or use `--driver` flag):

```bash
task -t Taskfile_pg.yml run
```

**Expected output on startup:**

```
DSN: postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require
```

If the connection fails, you'll see: `failed to ping database`

---

## Step 3: Run Migrations

Migrations run **automatically** on startup. The system reads from `store/migration/postgres/`:

- `LATEST.sql` — Full Postgres schema (957 lines)
- `0.19/` through `0.29/` — Versioned migration files

No manual migration step is needed.

---

## Configuration Details

### How the Driver is Selected

```
bin/memos/main.go
  → viper reads "driver" from CLI flag (--driver) or env var (MEMOS_DRIVER)
  → default: "sqlite"
  → passed to profile.Profile.Driver

store/db/db.go
  → switch profile.Driver:
    case "postgres": postgres.NewDB(profile)

store/db/postgres/postgres.go
  → sql.Open("pgx", profile.DSN)
  → pgx/v5 driver handles Neon SSL (sslmode=require)
```

### DSN Resolution Priority

1. `--dsn` CLI flag (highest)
2. `DATABASE_URL` env var (when driver=postgres and DSN is empty)
3. Default SQLite file path (when driver=sqlite)

From `internal/profile/profile.go:97-101`:
```go
if p.Driver == "postgres" && p.DSN == "" {
    p.DSN = os.Getenv("DATABASE_URL")
    if p.DSN == "" {
        return errors.New("postgres driver requires DSN or DATABASE_URL environment variable")
    }
}
```

### Connection Pool Settings

From `store/db/postgres/postgres.go:32-35`:
- MaxOpenConns: 10
- MaxIdleConns: 5
- ConnMaxLifetime: 5 minutes
- ConnMaxIdleTime: 1 minute
- Ping timeout: 60 seconds

### Neon SSL

The `pgx/v5` driver handles `sslmode=require` natively. No extra configuration needed. The Neon connection string should include `sslmode=require` (and optionally `channel_binding=require`).

---

## Taskfile Commands

Use `task -t Taskfile_pg.yml` for Postgres-specific commands:

| Command | Description |
|---------|-------------|
| `task -t Taskfile_pg.yml run` | Run dev server with Postgres |
| `task -t Taskfile_pg.yml run:rag` | Run with RAG + Postgres |
| `task -t Taskfile_pg.yml run:testrag` | Run with RAG + force reindex + Postgres |
| `task -t Taskfile_pg.yml validate:migrations` | Validate migration files |
| `task -t Taskfile_pg.yml fly:db-check` | Pre-deploy migration check |

**Note:** These tasks source `.env` but also set `DB_DRIVER=postgres` inline, which doesn't actually work (viper uses `MEMOS_DRIVER`). The `.env` file approach with `MEMOS_DRIVER=postgres` is the correct way.

---

## Local Postgres (Alternative to Neon)

If you want a local Postgres for development instead of Neon:

```bash
# Start local Postgres container
task -t Taskfile_pg.yml postgres:start

# Credentials: postgresql://bchat:bchat@localhost:5432/bchat

# Set in .env:
# MEMOS_DRIVER=postgres
# DATABASE_URL=postgresql://bchat:bchat@localhost:5432/bchat

# Run
task -t Taskfile_pg.yml run

# Stop when done
task -t Taskfile_pg.yml postgres:stop
```

---

## Troubleshooting

### "unknown db driver"

The `MEMOS_DRIVER` env var is not being read. Ensure it's `MEMOS_DRIVER` (not `DB_DRIVER`). Alternatively, use the `--driver postgres` CLI flag.

### "postgres driver requires DSN or DATABASE_URL environment variable"

Set `DATABASE_URL` in `.env` or pass `--dsn` on the command line.

### "failed to ping database"

- Check Neon is not paused (Neon free tier pauses after inactivity)
- Verify `sslmode=require` is in the connection string
- Check network connectivity (firewall, VPN)
- Verify credentials are correct

### Neon Paused / Autosuspend

Neon free tier databases pause after ~5 minutes of inactivity. On first connection after pause, there may be a ~2-5 second cold start delay. The 60-second ping timeout in `postgres.go` handles this.

### SSL Errors

If you see SSL-related errors, ensure your Neon connection string includes:
```
?sslmode=require
```

Do not use `sslmode=disable` with Neon.

---

## Related Files

| File | Purpose |
|------|---------|
| `store/db/postgres/postgres.go` | Connection setup, pgx driver |
| `store/db/postgres/agent.go` | Agent CRUD (2474 lines, fully implemented) |
| `store/db/db.go` | Driver selection switch |
| `internal/profile/profile.go` | DSN resolution logic |
| `bin/memos/main.go` | Viper config, env var binding |
| `store/migration/postgres/LATEST.sql` | Full Postgres schema |
| `Taskfile_pg.yml` | Postgres-specific Taskfile |
| `scripts/docker-compose.postgres.yml` | Local Postgres container |
| `.env.example` | Reference env file with DATABASE_URL |

---

*Document Version: 1.0*
