# Neon PostgreSQL Setup Plan (v3)

**Status:** Ready to implement
**Date:** 2026-07-08
**Review:** Addressed findings from `plan_review.md` (14 valid, 1 invalid)

---

## Overview

This guide covers the full SQLite → Neon Postgres → Fly.io workflow.

| Phase | Environment | Database | Config |
|-------|------------|----------|--------|
| 1. Feature development | Local | SQLite | `task run` |
| 2. Postgres validation | Local | Neon (remote) | `.env` with `MEMOS_DRIVER=postgres` |
| 3. Production | Fly.io | Neon (remote) | `fly_pg.toml` + `fly secrets set DATABASE_URL=...` |

**Key fact:** The Postgres driver is already fully implemented (`store/db/postgres/`, 24 files). The 6 unimplemented stubs (OM + Workflows) need to be added as part of this plan.

**Bridge delivery note:** Bridge features require Postgres (`SupportsBridgeDelivery()` returns `true` only on Postgres). Do not test bridge features in Phase 1 (SQLite).

---

## Step 1: Implement Postgres Stubs

Six methods in the Postgres driver are stubs that will error at runtime. Implement them before porting features.

### 1a. Observational Memory (`store/db/postgres/agent_observations.go`)

Replace the 3 stub methods with real implementations. Use the SQLite version (`store/db/sqlite/agent_observations.go`) as reference, adapting to Postgres syntax.

**`UpsertObservationLog`** — Use `INSERT ... ON CONFLICT(session_id) DO UPDATE SET` (not `DO NOTHING`). The SQLite version at line 17 uses this exact pattern. Key Postgres differences:
- Placeholders: `$1, $2, ...` instead of `?`
- Timestamp: `EXTRACT(EPOCH FROM NOW())` for defaults (not needed here since Go sets `LastUpdatedAt`)
- Use `RETURNING created_at` to get the timestamp back (same as SQLite line 25)
- Import `common.go` helpers: use `placeholder(n)` for single-param queries

```sql
INSERT INTO agent_observations (
    session_id, tenant_id, resource_id, observation_log,
    last_observed_msg_index, tokens_in_log, current_task,
    suggested_response, last_updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT(session_id) DO UPDATE SET
    resource_id = EXCLUDED.resource_id,
    observation_log = EXCLUDED.observation_log,
    last_observed_msg_index = EXCLUDED.last_observed_msg_index,
    tokens_in_log = EXCLUDED.tokens_in_log,
    current_task = EXCLUDED.current_task,
    suggested_response = EXCLUDED.suggested_response,
    last_updated_at = EXCLUDED.last_updated_at
RETURNING created_at
```

**`GetObservationLog`** — SELECT by `session_id = $1`. Return `nil, nil` on `sql.ErrNoRows` (same as SQLite line 54-55).

**`GetObservationLogByResource`** — SELECT by `resource_id = $1`, `ORDER BY last_updated_at DESC LIMIT 1`. Return `nil, nil` on `sql.ErrNoRows`.

### 1b. Agent Workflows (`store/db/postgres/agent_workflow.go`)

Replace the 3 silent no-op methods. Use the SQLite version (`store/db/sqlite/agent_workflow.go`) as reference.

**`CreateAgentWorkflow`** — Use `RETURNING id` (same as SQLite line 26). Key differences:
- Placeholders: `$1` through `$10` (10 columns)
- Import `placeholders(n)` from `common.go` if building dynamic queries

```sql
INSERT INTO agent_workflows (
    ticket_id, session_id, agent_name, task_name, task_mode,
    task_status, task_summary, predicted_size, created_ts, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id
```

**`ListAgentWorkflows`** — Build WHERE clause dynamically using `placeholders(n)` from `common.go` for the args slice. The SQLite version at lines 62-92 builds `where` and `args` slices; the Postgres version should do the same but use `$N` placeholders instead of `?`.

**`GetAgentWorkflow`** — Delegate to `ListAgentWorkflows` and return first result (same pattern as SQLite line 128-136).

---

## Step 2: Fix Taskfile_pg.yml Bug

The env var `DB_DRIVER=postgres` doesn't work because viper uses a `MEMOS_` prefix with `AutomaticEnv()` (`bin/memos/main.go:167`).

**File:** `Taskfile_pg.yml`

| Line | Current | Fix |
|------|---------|-----|
| 72 | `DB_DRIVER=postgres` | `MEMOS_DRIVER=postgres` |
| 83 | `DB_DRIVER=postgres` | `MEMOS_DRIVER=postgres` |
| 94 | `DB_DRIVER=postgres` | `MEMOS_DRIVER=postgres` |
| 104 | `DB_DRIVER=postgres` | `MEMOS_DRIVER=postgres` |
| 115 | `DB_DRIVER=postgres` | `MEMOS_DRIVER=postgres` |

**Also:** `.env.example` line 92 has `DB_DRIVER=sqlite` which is inconsistent with the viper `MEMOS_` prefix. Update to `MEMOS_DRIVER=sqlite`. Note: `.env.example` already has `MEMOS_DSN` at line 96, so this aligns the naming. This is documentation-only; the real fix is in the user's `.env`.

---

## Step 3: Configure Local `.env` for Neon

**Before adding `MEMOS_DRIVER=postgres`**, check if your `.env` already contains `DB_DRIVER=...`. If so, **comment it out or remove it** to avoid confusion (the dead variable won't cause errors, but it's misleading).

Add to your `.env` file:

```bash
# Database driver (overrides default "sqlite")
# NOTE: Must be MEMOS_DRIVER (not DB_DRIVER) — viper uses MEMOS_ prefix
MEMOS_DRIVER=postgres

# Neon connection string (replace with your actual credentials)
DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require"
```

**`channel_binding=require` note:** The connection string above uses `sslmode=require` only. You can add `&channel_binding=require` if your Neon project supports SCRAM-SHA-256 channel binding, but it's optional and not required for a secure connection.

### Env Var Flow (Production)

```
fly secrets set DATABASE_URL="postgresql://..."
    ↓ (sets OS environment variable directly)
Docker container starts, entrypoint.sh runs
    ↓ (DATABASE_URL already in env — no _FILE processing needed)
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

## Step 4: Verify Local Neon Connection

```bash
# Build backend
task build:backend

# Run with Postgres driver
MEMOS_DRIVER=postgres ./build/memos --mode dev
```

**Expected startup output:**
```
DSN: postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require
```

**Migrations run automatically** from `store/migration/postgres/`.

If connection fails, check:
- Neon is not paused (free tier autosuspends after ~5 min)
- `sslmode=require` is in the connection string
- Network connectivity (no firewall blocking port 5432)

---

## Step 5: Create `Dockerfile.pg.fly`

Create a Postgres-specific Dockerfile based on `Dockerfile.s3.fly`, removing the unnecessary `VOLUME /var/opt/memos` directive.

### Changes from `Dockerfile.s3.fly`

| Line | `Dockerfile.s3.fly` | `Dockerfile.pg.fly` |
|------|---------------------|---------------------|
| 80-82 | `RUN mkdir -p /var/opt/memos` + `VOLUME /var/opt/memos` | `RUN mkdir -p /var/opt/memos` (keep mkdir, remove VOLUME) |
| Comment | "Create data directory for SQLite" | "Create data directory" |

The `mkdir -p` is still needed (LanceDB may use local fallback), but the `VOLUME` declaration is dead weight for Postgres deployments.

---

## Step 6: Create `fly_pg.toml`

Create a new `fly_pg.toml` based on the existing `fly.toml`, with these changes.

### Changes from `fly.toml`

| Setting | `fly.toml` (SQLite) | `fly_pg.toml` (Neon) |
|---------|---------------------|----------------------|
| App name | `bchat0534` | **MUST CHANGE** — e.g., `bchat0534-pg` |
| `[build] dockerfile` | `Dockerfile.s3.fly` | `Dockerfile.pg.fly` |
| `[env] MEMOS_DRIVER` | not set | `'postgres'` |
| `[[mounts]]` | `source = "memos_data"`, `destination = "/var/opt/memos"` | **Remove entirely** |
| `[env] LANCEDB_LOCAL_PATH` | `'/var/opt/memos/lancedb'` (stale) | **Remove** (not needed with S3) |
| `[http_service] auto_stop_machines` | `'stop'` | `'stop'` (use string, not boolean) |
| All other env | Same | Same |

### `fly_pg.toml` Template

```toml
# ============================================================
# MUST CHANGE: Replace 'bchat0534-pg' with your Fly.io app name
# ============================================================
app = 'bchat0534-pg'
primary_region = 'sjc'

[build]
  dockerfile = 'Dockerfile.pg.fly'

[env]
  MEMOS_DRIVER = 'postgres'
  MEMOS_MODE = 'prod'
  MEMOS_PORT = '5230'
  RAG_PIPELINE_ENABLED = 'true'
  EMBEDDING_PROVIDER = 'openrouter'
  EMBEDDING_MODEL = 'openai/text-embedding-3-small'
  EMBEDDING_BATCH_SIZE = '10'
  EMBEDDING_TIMEOUT = '10m'
  LANCEDB_STORAGE_PROVIDER = 's3'
  LANCEDB_S3_FORCE_PATH_STYLE = 'false'
  LLM_MODEL = "poolside/laguna-m.1:free"
  LLM_MODEL_REASONING = "nvidia/nemotron-3-ultra-550b-a55b:free"
  LLM_VERIFIER_ENABLED = 'false'
  FORCE_REINDEX_ON_STARTUP = 'false'
  RAG_STARTUP_REINDEX_DISABLED = 'true'
  TZ = 'UTC'

# NO [[mounts]] section — Neon replaces the SQLite volume

[http_service]
  internal_port = 5230
  force_https = true
  auto_stop_machines = 'stop'
  auto_start_machines = true
  min_machines_running = 0
  processes = ['app']
  request_timeout = "30s"

  [http_service.concurrency]
    type = 'connections'
    hard_limit = 25
    soft_limit = 20

  [[http_service.checks]]
    grace_period = "15s"
    interval = "5s"
    method = "GET"
    path = "/healthz"

[[vm]]
  memory = '1024mb'
  cpu_kind = 'shared'
  cpus = 1
  memory_mb = 1024
```

**Note:** `DATABASE_URL` is NOT in `[env]` — it's a secret, set via `fly secrets set`.
**Note:** `LANCEDB_S3_BUCKET` is also a secret (set via `fly secrets set`), not in `[env]`. Secrets override `[env]` values.

---

## Step 7: Deploy to Fly.io with Neon

### 7a. Set secrets

```bash
# REQUIRED
fly secrets set DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require" --app bchat0534-pg
fly secrets set OPENROUTER_API_KEY="sk-or-v1-xxx" --app bchat0534-pg
fly secrets set LANCEDB_S3_BUCKET="your-bucket" --app bchat0534-pg

# OPTIONAL — only needed if you use tenant API key encryption
# fly secrets set ENCRYPTION_MASTER_KEY="$(uuidgen)" --app bchat0534-pg
```

**`ENCRYPTION_MASTER_KEY` note:** This is only required if you enable tenant-specific API key encryption. The app runs fine without it — it only fails when encryption is invoked (`server/router/api/v1/agent/handlers.go:2456`).

### 7b. Deploy

```bash
fly deploy -c fly_pg.toml --app bchat0534-pg
```

### 7c. Verify

```bash
# Check logs for DSN
fly logs --app bchat0534-pg

# Test health endpoint
curl https://bchat0534-pg.fly.dev/healthz

# Test agent endpoint
curl https://bchat0534-pg.fly.dev/api/v1/agent/your-slug/validate
```

---

## Step 8: Validate Migrations

**Before deploying**, validate that Postgres migrations are correct. **Do not run `fly deploy` if validation returns non-zero.**

```bash
# Start local Postgres for validation (or use Neon directly)
task -t Taskfile_pg.yml postgres:start

# Set DATABASE_URL for validation script
export DATABASE_URL="postgresql://bchat:bchat@localhost:5432/bchat"

# Run validation — FAILS deployment if non-zero exit
task -t Taskfile_pg.yml validate:migrations
```

This validates:
1. `LATEST.sql` creates a valid fresh schema
2. All versioned migrations apply in sequence
3. Table lists match between LATEST.sql and migrations

**Note:** Postgres LATEST.sql already embeds `tenant_role_templates` seed data (lines 685-692: Viewer, Tester, Analyst, Editor, Tenant Admin). No separate seed step is needed — the `seed()` function is SQLite-only by design.

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

**Do not test bridge features in Phase 1.** Bridge delivery requires Postgres.

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
| Upsert (update) | `INSERT ... ON CONFLICT DO UPDATE SET col = excluded.col` | Same syntax (Postgres supports `EXCLUDED`) |
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
| Bridge delivery not on SQLite | `SupportsBridgeDelivery()` returns false for SQLite | Test bridge features on Postgres only |
| Neon free tier autosuspend | ~2-5s cold start on first connection | 60s ping timeout handles this |
| Multiple fly.toml variants | Confusion about which is active | Keep `fly.toml` (SQLite) and `fly_pg.toml` (Neon), archive or delete others |
| `VOLUME /var/opt/memos` in shared Dockerfiles | Harmless but dead weight for Postgres | Use `Dockerfile.pg.fly` (no VOLUME) |

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
Ensure Step 1 (implement stubs) is complete before testing these features.

---

## Related Files

| File | Purpose |
|------|---------|
| `store/db/postgres/postgres.go` | Connection setup, pgx driver |
| `store/db/postgres/agent.go` | Agent CRUD (2474 lines) |
| `store/db/postgres/agent_observations.go` | OM stubs → to implement |
| `store/db/postgres/agent_workflow.go` | Workflow stubs → to implement |
| `store/db/postgres/common.go` | `$N` placeholder helpers |
| `store/db/sqlite/agent_observations.go` | SQLite reference for OM implementation |
| `store/db/sqlite/agent_workflow.go` | SQLite reference for Workflow implementation |
| `store/db/db.go` | Driver selection switch |
| `internal/profile/profile.go` | DSN resolution (`DATABASE_URL` fallback) |
| `bin/memos/main.go` | Viper config, `MEMOS_` env prefix |
| `store/migration/postgres/LATEST.sql` | Full Postgres schema (includes role templates) |
| `Taskfile_pg.yml` | Postgres Taskfile (to fix `DB_DRIVER` bug) |
| `fly.toml` | SQLite deployment config (keep as-is) |
| `fly_pg.toml` | Neon Postgres deployment config (to create) |
| `Dockerfile.s3.fly` | SQLite/S3 Dockerfile (keep as-is) |
| `Dockerfile.pg.fly` | Postgres Dockerfile (to create — no VOLUME) |
| `scripts/entrypoint.sh` | Docker entrypoint (`MEMOS_DSN` `_FILE` support) |
| `scripts/validate-pg-migrations.sh` | Migration validation script |
| `.env.example` | Reference env file (to fix `DB_DRIVER` → `MEMOS_DRIVER`) |
| `.env` | Local dev env file (to add `MEMOS_DRIVER` + `DATABASE_URL`) |

---

*Document Version: 3.0*
*Review findings addressed: 2026-07-08*
