# Neon PostgreSQL Setup Guide (v2)

**Status:** Complete
**Date:** 2026-07-08

---

## Overview

This guide covers the full SQLite → Neon Postgres → Fly.io workflow.

| Phase | Environment | Database | Config |
|-------|------------|----------|--------|
| 1. Feature development | Local | SQLite | `task run` |
| 2. Postgres validation | Local | Neon (remote) | `.env` with `MEMOS_DRIVER=postgres` |
| 3. Production | Fly.io | Neon (remote) | `fly_pg.toml` + `fly secrets set DATABASE_URL=...` |

**Key fact:** The Postgres driver is already fully implemented (`store/db/postgres/`, 24 files), including OM and Workflow methods.

---

## Step 1: Configure Local `.env` for Neon

Add to your `.env` file:

```bash
# Database driver (overrides default "sqlite")
MEMOS_DRIVER=postgres

# Neon connection string (replace with your actual credentials)
DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require"
```

### Env Var Flow (Production)

```
fly secrets set DATABASE_URL="postgresql://..."
    ↓ (sets OS environment variable)
Docker container starts, entrypoint.sh runs
    ↓ (DATABASE_URL is already in env, no _FILE processing needed)
bin/memos/main.go → viper reads MEMOS_DRIVER=postgres from env
    ↓
profile.Validate(): p.DSN == "" → p.DSN = os.Getenv("DATABASE_URL")
    ↓
store/db/db.go: switch "postgres" → postgres.NewDB(profile)
    ↓
store/db/postgres/postgres.go: sql.Open("pgx", profile.DSN)
    ↓
pgx/v5 handles sslmode=require natively for Neon
```

**Important:** `DATABASE_URL` is read via `os.Getenv()` in `internal/profile/profile.go:98`, NOT via viper. This means `fly secrets set DATABASE_URL=...` works directly without any viper binding.

---

## Step 2: Verify Local Neon Connection

```bash
# Build backend
task build:backend

# Run with Postgres driver
MEMOS_DRIVER=postgres ./build/memos --mode dev
```

**Expected startup output:**
```
DSN: postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require
```

**Migrations run automatically** from `store/migration/postgres/`.

If connection fails, check:
- Neon is not paused (free tier autosuspends after ~5 min)
- `sslmode=require` is in the connection string
- Network connectivity (no firewall blocking port 5432)

---

## Step 3: Create `fly_pg.toml`

Create a new `fly_pg.toml` based on the existing `fly.toml`, with these changes:

### Changes from `fly.toml`

| Setting | `fly.toml` (SQLite) | `fly_pg.toml` (Neon) |
|---------|---------------------|----------------------|
| App name | `bchat0534` | `bchat0534-pg` (or your choice) |
| `[env] MEMOS_DRIVER` | not set | `'postgres'` |
| `[[mounts]]` | `source = "memos_data"`, `destination = "/var/opt/memos"` | **Remove entirely** |
| `Dockerfile` | `Dockerfile.s3.fly` | `Dockerfile.pg.fly` |
| All other env | Same | Same |

### `fly_pg.toml` Template

```toml
app = 'bchat0534-pg'
primary_region = 'sjc'

[build]
  dockerfile = 'Dockerfile.pg.fly'

[env]
  MEMOS_DRIVER = 'postgres'
  RAG_PIPELINE_ENABLED = 'true'
  EMBEDDING_PROVIDER = 'openrouter'
  EMBEDDING_MODEL = 'openai/text-embedding-3-small'
  EMBEDDING_BATCH_SIZE = '10'
  LANCEDB_STORAGE_PROVIDER = 's3'
  LANCEDB_S3_FORCE_PATH_STYLE = 'false'
  LLM_MODEL = 'poolside/laguna-m.1:free'
  LLM_MODEL_REASONING = 'nvidia/nemotron-3-ultra-550b-a55b:free'
  LLM_VERIFIER_ENABLED = 'false'
  FORCE_REINDEX_ON_STARTUP = 'false'
  RAG_STARTUP_REINDEX_DISABLED = 'true'

# NO [[mounts]] section — Neon replaces the SQLite volume

[http_service]
  internal_port = 5230
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 0

  [http_service.concurrency]
    type = 'connections'
    hard_limit = 25
    soft_limit = 20

  [[http_service.checks]]
    grace_period = '15s'
    interval = '5s'
    method = 'GET'
    timeout = '5s'
    path = '/healthz'

[[vm]]
  memory = '1024mb'
  cpu_kind = 'shared'
  cpus = 1
```

**Note:** `DATABASE_URL` is NOT in `[env]` — it's a secret, set via `fly secrets set`.

---

## Step 4: Deploy to Fly.io with Neon

### 6a. Set secrets

```bash
fly secrets set DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require" --app bchat0534-pg
fly secrets set OPENROUTER_API_KEY="sk-or-v1-xxx" --app bchat0534-pg
fly secrets set ENCRYPTION_MASTER_KEY="$(uuidgen)" --app bchat0534-pg
fly secrets set LANCEDB_S3_BUCKET="your-bucket" --app bchat0534-pg
```

### 6b. Deploy

```bash
fly deploy -c fly_pg.toml --app bchat0534-pg
```

### 6c. Verify

```bash
# Check logs for DSN
fly logs --app bchat0534-pg

# Test health endpoint
curl https://bchat0534-pg.fly.dev/healthz

# Test agent endpoint
curl https://bchat0534-pg.fly.dev/api/v1/agent/your-slug/validate
```

---

## Step 5: Validate Migrations

Before deploying, validate that Postgres migrations are correct:

```bash
# Start local Postgres for validation (or use Neon directly)
task -t Taskfile_pg.yml postgres:start

# Set DATABASE_URL for validation script
export DATABASE_URL="postgresql://bchat:bchat@localhost:5432/bchat"

# Run validation
task -t Taskfile_pg.yml validate:migrations
```

This validates:
1. `LATEST.sql` creates a valid fresh schema
2. All versioned migrations apply in sequence
3. Table lists match between LATEST.sql and migrations

---

## Dual-Database Workflow

### Feature Development Cycle

```
1. Write feature with SQLite
   task run                          # SQLite, fast iteration

2. Port to Postgres
   - Add migration to store/migration/postgres/0.XX/
   - Test against Neon locally
   MEMOS_DRIVER=postgres ./build/memos --mode dev

3. Deploy to production
   fly deploy -c fly_pg.toml --app bchat0534-pg
```

### SQLite → Postgres Migration Checklist

When adding a new table or column:

| Step | SQLite | Postgres |
|------|--------|----------|
| Migration file | `store/migration/sqlite/0.XX/NN__name.sql` | `store/migration/postgres/0.XX/NN__name.sql` |
| Schema syntax | `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` |
| Boolean | `INTEGER CHECK (col IN (0,1))` | `BOOLEAN DEFAULT FALSE` |
| Timestamp | `strftime('%s', 'now')` | `EXTRACT(EPOCH FROM NOW())` |
| JSON | `TEXT DEFAULT '{}'` | `JSONB DEFAULT '{}'` |
| Upsert | `INSERT OR IGNORE` | `INSERT ... ON CONFLICT DO NOTHING` |
| Reserved words | No quoting needed | Quote: `"user"`, `"group"` |
| Store implementation | `store/db/sqlite/agent.go` | `store/db/postgres/agent.go` |
| Placeholder style | `?` | `$1, $2, ...` |

### Postgres-Specific SQL Helpers

From `store/db/postgres/common.go`:
- `placeholder(n)` → returns `$N` for single parameter
- `placeholders(n)` → returns `$1, $2, ..., $N` for multiple parameters

---

## Known Limitations

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| Seeding is SQLite-only | Default tenant_role_templates only seeded on SQLite | Run seed manually on Postgres if needed |
| Bridge delivery not on SQLite | `SupportsBridgeDelivery()` returns false for SQLite | Test bridge features on Postgres |
| Neon free tier autosuspend | ~2-5s cold start on first connection | 60s ping timeout handles this |
| Five fly.toml variants | Confusion about which is active | Keep `fly.toml` (SQLite) and `fly_pg.toml` (Neon) only |

---

## Troubleshooting

### "unknown db driver"
`MEMOS_DRIVER` env var not set. Use `MEMOS_DRIVER=postgres` (not `DB_DRIVER`).

### "postgres driver requires DSN or DATABASE_URL environment variable"
Set `DATABASE_URL` in `.env` or pass `--dsn` on command line, or set via `fly secrets set DATABASE_URL=...`.

### "failed to ping database"
- Check Neon is not paused (free tier)
- Verify `sslmode=require` in connection string
- Check network connectivity

### OM/Workflow errors on Postgres
OM and Workflow methods are fully implemented. If you see errors, check that the database migrations have run successfully.

---

## Related Files

| File | Purpose |
|------|---------|
| `store/db/postgres/postgres.go` | Connection setup, pgx driver |
| `store/db/postgres/agent.go` | Agent CRUD (2474 lines) |
| `store/db/postgres/agent_observations.go` | OM implementation (UpsertObservationLog, GetObservationLog, GetObservationLogByResource) |
| `store/db/postgres/agent_workflow.go` | Workflow implementation (CreateAgentWorkflow, ListAgentWorkflows, GetAgentWorkflow) |
| `store/db/postgres/common.go` | `$N` placeholder helpers |
| `store/db/db.go` | Driver selection switch |
| `internal/profile/profile.go` | DSN resolution (`DATABASE_URL` fallback) |
| `bin/memos/main.go` | Viper config, `MEMOS_` env prefix |
| `store/migration/postgres/LATEST.sql` | Full Postgres schema |
| `Taskfile_pg.yml` | Postgres Taskfile (uses `MEMOS_DRIVER=postgres`) |
| `fly.toml` | SQLite deployment config (keep as-is) |
| `fly_pg.toml` | Neon Postgres deployment config |
| `scripts/entrypoint.sh` | Docker entrypoint (`MEMOS_DSN` `_FILE` support) |
| `scripts/validate-pg-migrations.sh` | Migration validation script |
| `.env.example` | Reference env file |
| `.env` | Local dev env file (to add `MEMOS_DRIVER` + `DATABASE_URL`) |

---

*Document Version: 2.1*
*Updated: 2026-07-08 — Cleaned up stale references (stubs implemented, DB_DRIVER fixed, Dockerfile corrected)*
