# Plan: Port Postgres Database Migrations to CockroachDB

**Bug ID:** 057
**Date:** 2026-08-02
**Status:** Draft — Pending Adversarial Review
**Goal:** Seamless developer experience for local development, testing, and deployment with CockroachDB

---

## 1. User Prompt

> You are senior Go and CockroachDB architect, under bugs/050 I started CockroachDB support, bchat has live production support for Postgres on Neon and I want you to port existing Postgres database migration to CockroachDB as well, the goal is to test CockroachDB locally (refer to /home/chaschel/Desktop/cockroach/best-practices), once that is done the target is to port fly_pg.toml to CockroachDB-native fly_crdb.toml, let us make this question and answer until we agree to a sound plan and for you to ask clarifying questions, for context I still want to deploy to fly.io but with additional support for CockroachDB, verify if the Postgres connection string is stored as fly secret in fly.io (I have the connection string for CockroachDB)

### Clarification Answers

| Question | Answer |
|----------|--------|
| Driver strategy | **New cockroach driver** — separate from `postgres` in profile.go/db.go |
| Migration directory | **New `store/migration/cockroach/` directory** — CockroachDB-specific LATEST.sql |
| Fly.io CRDB target | **CockroachDB Cloud** — managed, like Neon pattern |
| Vector DB in CRDB Fly | **Yes, `-tags cockroach`** — native VECTOR(1536) support |
| CRDB Cloud cluster | **Already provisioned** — connection string ready |

---

## 2. Background Investigation

### 2.1 Existing CockroachDB Work (bugs/050)

The `bugs/050/` directory contains a complete CockroachDB vector store implementation:

| File | Purpose |
|------|---------|
| `server/router/api/v1/agent/vectordb_cockroach.go` | Full VectorDB interface (356 lines, `//go:build cockroach`) — native `VECTOR(1536)`, `crdb.ExecuteTx` for retry, cosine distance `<=>` operator |
| `server/router/api/v1/agent/vectordb_nocockroach.go` | Build tag stub (54 lines, `//go:build !cockroach`) — all 12 methods return errors/nil |
| `scripts/docker-compose.cockroach.yml` | Local CockroachDB v25.2.21 Docker setup — single-node insecure, ports 26257 (SQL) and 8080 (DB Console) |
| `Taskfile.yml` | 12 `crdb:*` tasks mirroring `fly:*` pattern |

**Key bugs found and fixed in bugs/050:**
1. `vector_ip_ops` not supported in CRDB v25.2.21 (error 0A000) — fixed by removing opclass from DDL
2. `Search()` ignored pre-computed `QueryEmbedding` — fixed to match LanceDB priority pattern
3. `MinScore` filtering was ignored — fixed with `AND (embedding <=> $1::VECTOR) <= 1 - $4`
4. `pq.Error` → `pgconn.PgError` fix (since `lib/pq` is banned)

**Dependencies already in go.mod:**
```
github.com/cockroachdb/cockroach-go/v2 v2.4.3
```

### 2.2 Current Postgres/Neon Architecture

#### DSN Flow

```
fly secrets set DATABASE_URL="postgresql://..."
    │
    ▼
Docker container starts, entrypoint.sh runs
    │  (file_env expands MEMOS_DSN_FILE if present)
    ▼
bin/memos/main.go → viper reads MEMOS_DRIVER=postgres from [env]
    │
    ▼
profile.Validate(): p.DSN == "" → p.DSN = os.Getenv("DATABASE_URL")
    │
    ▼
store/db/db.go: switch "postgres" → postgres.NewDB(profile)
    │
    ▼
store/db/postgres/postgres.go: sql.Open("pgx", profile.DSN + "?default_query_exec_mode=simple_protocol")
    │
    ▼
pgx/v5 handles sslmode=require natively for Neon
```

#### Driver: pgx/v5 via database/sql stdlib adapter

```go
// store/db/postgres/postgres.go
import _ "github.com/jackc/pgx/v5/stdlib"

func NewDB(profile *profile.Profile) (store.Driver, error) {
    dsn := profile.DSN
    // Auto-append simple_protocol for Neon PgBouncer compatibility
    if !strings.Contains(dsn, "default_query_exec_mode") {
        sep := "?"
        if strings.Contains(dsn, "?") { sep = "&" }
        dsn += sep + "default_query_exec_mode=simple_protocol"
    }
    db, err := sql.Open("pgx", dsn)
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)
    return &DB{db: db, profile: profile}, nil
}
```

**`lib/pq` is NOT used** — confirmed by AGENTS.md and no import matches.

#### Migration System

- `store/migrator.go` — uses `//go:embed migration` to embed all migration directories
- `getMigrationBasePath()` returns `migration/{driver}/` — driver-specific migrations
- `preMigrate()` applies `LATEST.sql` when no migration history exists
- `Migrate()` applies incremental `.sql` files when schema version > latest applied
- **SQLite-only helpers** in `migration_helper.go` use `PRAGMA table_info()` — gated on `s.profile.Driver == "sqlite"`

#### Fly Secrets (confirmed)

The connection string **is** stored as a Fly secret via `scripts/fly-pg-secrets.sh`:

```bash
fly -a bchat-pg secrets set DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require"
fly -a bchat-pg secrets set OPENROUTER_API_KEY="sk-or-v1-xxx"
fly -a bchat-pg secrets set ENCRYPTION_MASTER_KEY="$(uuidgen)"
fly -a bchat-pg secrets set LANCEDB_S3_BUCKET="your-bucket-name"
```

The `DATABASE_URL` is read via `os.Getenv()` in `internal/profile/profile.go`, not via viper.

#### Postgres LATEST.sql Schema (1030 lines)

Key tables: `migration_history`, `system_setting`, `user`, `memo`, `resource`, `activity`, `idp`, `inbox`, `webhook`, `reaction`, `agent_tenants`, `agent_audiences`, `tenant_role_templates`, `user_tenant_permission`, `tenant_config`, `agent_messages`, `agent_transcripts`, `agent_leads`, `agent_services`, `agent_exclusions`, `agent_coverage`, `agent_faqs`, `agent_safety_protocols`, `agent_kb_sections`, `agent_intents`, `agent_rules`, `agent_sessions`, `agent_source_files`, `agent_rate_limits`, `agent_simulation_transcripts`, `agent_tenant_scripts`, `agent_simulations`, `agent_script_analysis`, `agent_analysis_results`, `agent_learning_memory`, `agent_compliance_audits`, `agent_scoring_config`, `agent_qa_pairs`, `tickets`, `agent_workflows`, `agent_reindex_checkpoints`, `agent_rag_active_versions`, `agent_observations`, `system_secret`, `notifications`, `bridge_external_sessions`, `bridge_handoffs`, `bridge_handoff_replies`, `bridge_reply_outbox`, `bridge_auth_keys`, `bridge_auth_nonces`, `agent_integrations`, `agent_events`, `user_access_token_lookup`.

**Note:** `agent_vectors` is intentionally NOT in migrations — created at runtime by `CockroachVectorDB.Validate()` because `VECTOR(1536)` has different requirements per database.

### 2.3 CockroachDB Best Practices

From `/home/chaschel/Desktop/cockroach/best-practices/`:

**prod_checklist.md:**
- Transaction retry handling is MANDATORY (client-side, SQLSTATE 40001)
- Connection pooling: active connections ≤ 4x vCPU count
- TLS certificates required for all production connections
- `--cache=.25 --max-sql-memory=.25` (defaults are too low)
- Use `--locality` flag on all nodes

**test_locally.md:**
```bash
cockroach start-single-node --insecure --store=type=mem,size=0.25 --advertise-addr=localhost
# Recommended cluster settings for faster testing:
SET CLUSTER SETTING kv.range_merge.queue_interval = '50ms';
SET CLUSTER SETTING jobs.registry.interval.gc = '30s';
SET CLUSTER SETTING jobs.retention_time = '15s';
SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
ALTER RANGE default CONFIGURE ZONE USING "gc.ttlseconds" = 600;
```

**docker.md:**
- Uses Docker bridge network and volumes for data persistence
- Insecure mode is for non-production testing only
- Single-node supports auto-creating database/user/password via environment variables
- Graceful shutdown recommended with 5-minute timeout

### 2.4 CockroachDB Postgres Compatibility

CockroachDB is wire-compatible with PostgreSQL. Key compatibility notes:

| Feature | CockroachDB | Notes |
|---------|-------------|-------|
| `SERIAL` | Supported | But `INT DEFAULT unique_rowid()` preferred for performance |
| `INT GENERATED BY DEFAULT AS IDENTITY` | Supported | Preferred over SERIAL |
| `JSONB` | Supported | Works as-is |
| `BYTEA` | Supported | Works as-is |
| `BOOLEAN` | Supported | Works as-is |
| `TIMESTAMPTZ` | Supported | Works as-is |
| `DOUBLE PRECISION` | Supported | Works as-is |
| `EXTRACT(EPOCH FROM NOW())` | Supported | Works as-is |
| `ON CONFLICT DO NOTHING` | Supported | Works as-is |
| `ON CONFLICT (col) DO UPDATE SET` | Supported | Works as-is |
| `CREATE INDEX IF NOT EXISTS` | Supported | Works as-is |
| Foreign keys | Supported | Works as-is |
| CHECK constraints | Supported | Works as-is |
| `pgx/v5` driver | Supported | Wire-compatible |
| SQLSTATE 40001 | Serialization error | Must handle with client-side retry |
| `CREATE EXTENSION` | NOT supported | No extension system |
| `pgvector` | NOT supported | Use native `VECTOR` type instead |

### 2.5 Key Files Reference

| File | Path |
|------|------|
| Driver factory | `store/db/db.go` |
| Profile/DSN resolution | `internal/profile/profile.go` |
| Postgres driver | `store/db/postgres/postgres.go` |
| Postgres resilience | `store/db/postgres/resilience.go` |
| Postgres common | `store/db/postgres/common.go` |
| Store interface | `store/driver.go` |
| Migrator | `store/migrator.go` |
| Migration helper (SQLite-only) | `store/migration_helper.go` |
| Postgres LATEST.sql | `store/migration/postgres/LATEST.sql` (1030 lines) |
| Fly PG config | `fly_pg.toml` |
| Dockerfile PG Fly | `Dockerfile.pg.fly` |
| Fly secrets script | `scripts/fly-pg-secrets.sh` |
| CockroachDB Docker Compose | `scripts/docker-compose.cockroach.yml` |
| CockroachDB VectorDB | `server/router/api/v1/agent/vectordb_cockroach.go` |
| CockroachDB VectorDB stub | `server/router/api/v1/agent/vectordb_nocockroach.go` |
| Taskfile (crdb tasks) | `Taskfile.yml` (lines 222-353) |
| go.mod (cockroach-go) | `go.mod` (line 13) |
| bugs/050 summary | `bugs/050/summary.md` |
| bugs/050 plan5 | `bugs/050/plan5.md` |

---

## 3. Implementation Plan

### Phase 1: CockroachDB Driver (`store/db/cockroach/`)

**Goal:** Create a new `cockroach` driver that reuses the `pgx/v5` wire protocol but adds CockroachDB-specific behaviors.

#### 1.1 New Files

| File | Purpose |
|------|---------|
| `store/db/cockroach/cockroach.go` | Driver init: `sql.Open("pgx", dsn)` — no `default_query_exec_mode=simple_protocol` (not needed for CRDB) |
| `store/db/cockroach/migration_history.go` | Migration history CRUD (adapted from postgres) |
| `store/db/cockroach/common.go` | Placeholder helpers (`$1`, `$2`, ...) |
| `store/db/cockroach/resilience.go` | Transient error detection + `crdb.ExecuteTx` wrapper for SQLSTATE 40001 |
| `store/db/cockroach/activity.go` | Activity store methods |
| `store/db/cockroach/agent.go` | Agent store methods |
| `store/db/cockroach/bridge.go` | Bridge store methods |
| `store/db/cockroach/memo.go` | Memo store methods |
| `store/db/cockroach/ticket.go` | Ticket store methods |
| `store/db/cockroach/user.go` | User store methods |
| `store/db/cockroach/resource.go` | Resource store methods |
| `store/db/cockroach/inbox.go` | Inbox store methods |
| `store/db/cockroach/webhook.go` | Webhook store methods |
| `store/db/cockroach/reaction.go` | Reaction store methods |
| `store/db/cockroach/rbac.go` | RBAC store methods |
| `store/db/cockroach/notification.go` | Notification store methods |
| `store/db/cockroach/workspace_setting.go` | Workspace settings |
| `store/db/cockroach/user_setting.go` | User settings |
| `store/db/cockroach/idp.go` | Identity provider |
| `store/db/cockroach/memo_filter.go` | Memo filtering |
| `store/db/cockroach/agent_observations.go` | Agent observations |
| `store/db/cockroach/agent_workflow.go` | Agent workflows |

#### 1.2 Key Differences from Postgres Driver

**`cockroach.go` (driver init):**
```go
func NewDB(profile *profile.Profile) (store.Driver, error) {
    dsn := profile.DSN
    // NO default_query_exec_mode=simple_protocol — not needed for CockroachDB
    db, err := sql.Open("pgx", dsn)
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)
    return &DB{db: db, profile: profile}, nil
}
```

**`resilience.go` (transaction retry):**
```go
import "github.com/cockroachdb/cockroach-go/v2/crdb"

// ExecuteTx wraps crdb.ExecuteTx for automatic retry on SQLSTATE 40001
func ExecuteTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
    return crdb.ExecuteTx(ctx, db, nil, fn)
}
```

#### 1.3 Modify Existing Files

| File | Change |
|------|--------|
| `store/db/db.go` | Add `case "cockroach":` to switch, import cockroach package |
| `internal/profile/profile.go` | Add cockroach driver case: read DSN from `COCKROACH_DSN` env var with `DATABASE_URL` fallback |

**`profile.go` addition:**
```go
if p.Driver == "cockroach" && p.DSN == "" {
    p.DSN = os.Getenv("COCKROACH_DSN")
    if p.DSN == "" {
        p.DSN = os.Getenv("DATABASE_URL")  // fallback for consistency
    }
    if p.DSN == "" {
        return errors.New("cockroach driver requires COCKROACH_DSN or DATABASE_URL environment variable")
    }
}
```

**`db.go` addition:**
```go
import "github.com/usememos/memos/store/db/cockroach"

case "cockroach":
    driver, err = cockroach.NewDB(profile)
```

#### 1.4 Method Migration Strategy

The postgres store methods are SQL-agnostic (they use `$1`, `$2` placeholders which work in both Postgres and CockroachDB). The migration approach:

1. Copy all `.go` files from `store/db/postgres/` to `store/db/cockroach/`
2. Change package name from `postgres` to `cockroach`
3. Replace `isTransientError` with CockroachDB-aware version (add SQLSTATE 40001)
4. Replace direct `tx.ExecContext` calls in transaction-heavy methods with `crdb.ExecuteTx` wrapper
5. Remove `default_query_exec_mode=simple_protocol` from connection setup

**Estimated effort:** ~22 files to copy and adapt. Most are direct copies with package name change.

---

### Phase 2: Migration Schema (`store/migration/cockroach/`)

**Goal:** Create a CockroachDB-specific LATEST.sql that optimizes for CRDB performance characteristics.

#### 2.1 New File

| File | Purpose |
|------|---------|
| `store/migration/cockroach/LATEST.sql` | Full schema adapted for CockroachDB |

#### 2.2 Key Adaptations from Postgres LATEST.sql

```sql
-- =====================================================
-- COCKROACHDB SCHEMA (adapted from Postgres LATEST.sql)
-- =====================================================

-- migration_history
CREATE TABLE IF NOT EXISTS migration_history (
  version TEXT NOT NULL PRIMARY KEY,
  created_ts INT NOT NULL DEFAULT extract(epoch from now())
);

-- user
-- SERIAL → INT DEFAULT unique_rowid() for better CRDB performance
CREATE TABLE "user" (
  id INT DEFAULT unique_rowid() PRIMARY KEY,
  created_ts INT NOT NULL DEFAULT extract(epoch from now()),
  updated_ts INT NOT NULL DEFAULT extract(epoch from now()),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  username TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL CHECK (role IN ('HOST', 'ADMIN', 'USER')) DEFAULT 'USER',
  email TEXT NOT NULL DEFAULT '',
  nickname TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  avatar_url TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  allowed_tenant_ids TEXT DEFAULT NULL
);

-- memo
CREATE TABLE memo (
  id INT DEFAULT unique_rowid() PRIMARY KEY,
  uid TEXT NOT NULL UNIQUE,
  creator_id INT NOT NULL,
  created_ts INT NOT NULL DEFAULT extract(epoch from now()),
  updated_ts INT NOT NULL DEFAULT extract(epoch from now()),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  content TEXT NOT NULL DEFAULT '',
  visibility TEXT NOT NULL CHECK (visibility IN ('PUBLIC', 'PROTECTED', 'PRIVATE')) DEFAULT 'PRIVATE',
  pinned BOOLEAN NOT NULL DEFAULT FALSE,
  payload JSONB NOT NULL DEFAULT '{}',
  tenant_id INT DEFAULT NULL
);

-- ... (all other tables follow same pattern)
-- SERIAL → INT DEFAULT unique_rowid()
-- INTEGER → INT
-- BIGINT → INT (CRDB handles large ints natively)
-- Everything else stays the same
```

#### 2.3 DDL Differences Summary

| Postgres | CockroachDB | Reason |
|----------|-------------|--------|
| `SERIAL PRIMARY KEY` | `INT DEFAULT unique_rowid() PRIMARY KEY` | Better performance, avoids sequence contention |
| `INTEGER` | `INT` | Idiomatic CRDB (both work) |
| `BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())` | `INT NOT NULL DEFAULT extract(epoch from now())` | CRDB INT handles 64-bit |
| `EXTRACT(EPOCH FROM NOW())::BIGINT` | `extract(epoch from now())` | No cast needed |
| Everything else | Same | CRDB is wire-compatible |

#### 2.4 Seed Data

The `tenant_role_templates` INSERT in LATEST.sql uses `ON CONFLICT DO NOTHING` — this works in CockroachDB as-is.

---

### Phase 3: Local Development & Testing

**Goal:** Seamless `task run:cockroach` experience.

#### 3.1 Docker Compose (already exists)

`scripts/docker-compose.cockroach.yml` is already configured for local testing:
- Image: `cockroachdb/cockroach:v25.2.21`
- Single-node insecure mode
- Ports: 26257 (SQL), 8080 (DB Console)
- Persistent volume: `bchat_crdb_data`
- Healthcheck configured

#### 3.2 Local Testing Workflow

```bash
# 1. Start CockroachDB
docker compose -f scripts/docker-compose.cockroach.yml up -d

# 2. Build with cockroach tag (existing task)
task build:backend:cockroach

# 3. Run (existing task)
task run:cockroach

# 4. Or manual:
COCKROACH_DSN="postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
MEMOS_DRIVER=cockroach \
MEMOS_MODE=dev \
RAG_PIPELINE_ENABLED=true \
VECTOR_DB_PROVIDER=cockroach \
./build/memos
```

#### 3.3 Verify Schema

```bash
# Check tables created
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost \
  -d bchat -e "SHOW TABLES;"

# Check migration history
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost \
  -d bchat -e "SELECT * FROM migration_history;"

# Check vector support (agent_vectors table)
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost \
  -d bchat -e "SHOW TABLES LIKE 'agent_vectors';"
```

#### 3.4 Recommended Test Settings (from best-practices)

For faster local testing, apply these cluster settings:

```bash
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost -e "
SET CLUSTER SETTING kv.range_merge.queue_interval = '50ms';
SET CLUSTER SETTING jobs.registry.interval.gc = '30s';
SET CLUSTER SETTING jobs.retention_time = '15s';
SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
ALTER RANGE default CONFIGURE ZONE USING gc.ttlseconds = 600;
"
```

**WARNING:** These settings are NOT for production or benchmarking.

#### 3.5 Integration Tests

```bash
# Run CockroachDB-specific tests
task crdb:test

# Or manually:
go test -v -tags "cockroach" ./server/router/api/v1/agent/... -run "TestProcessPendingTickets|TestEmbedTenantTickets"
```

---

### Phase 4: Fly.io Deployment (`fly_crdb.toml`)

**Goal:** Deploy bchat to Fly.io with CockroachDB Cloud backend.

#### 4.1 New Files

| File | Purpose |
|------|---------|
| `fly_crdb.toml` | Fly.io config for CockroachDB Cloud backend |
| `Dockerfile.crdb.fly` | Dockerfile with `-tags cockroach` for native vector support |
| `scripts/fly-crdb-secrets.sh` | Secrets setup script for CockroachDB deployment |

#### 4.2 `fly_crdb.toml`

```toml
# ============================================================
# CockroachDB Cloud backend deployment
# ============================================================
app = 'bchat-crdb'
primary_region = 'sjc'

[build]
  dockerfile = 'Dockerfile.crdb.fly'

[env]
  MEMOS_DRIVER = 'cockroach'
  MEMOS_MODE = 'prod'
  MEMOS_PORT = '5230'
  RAG_PIPELINE_ENABLED = 'true'
  VECTOR_DB_PROVIDER = 'cockroach'
  EMBEDDING_PROVIDER = 'openrouter'
  EMBEDDING_MODEL = 'openai/text-embedding-3-small'
  EMBEDDING_BATCH_SIZE = '10'
  EMBEDDING_TIMEOUT = '10m'
  LANCEDB_STORAGE_PROVIDER = 's3'
  LANCEDB_S3_FORCE_PATH_STYLE = 'false'
  LLM_MODEL = "openrouter/free"
  LLM_MODEL_REASONING = "openrouter/free"
  LLM_VERIFIER_ENABLED = 'false'
  FORCE_REINDEX_ON_STARTUP = 'false'
  RAG_STARTUP_REINDEX_DISABLED = 'true'
  TZ = 'UTC'

# NO [[mounts]] — CockroachDB Cloud replaces SQLite volume

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

#### 4.3 `Dockerfile.crdb.fly`

Based on `Dockerfile.pg.fly` with key change in Stage 2:

```dockerfile
# Build with CockroachDB native vector + LanceDB RAG support
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-I/usr/local/include/lancedb"
ENV CGO_LDFLAGS="-L/usr/local/lib/lancedb -llancedb_go -Wl,-rpath,/usr/local/lib/lancedb"

RUN go build -tags "cockroach rag" -ldflags="-s -w" -o memos ./bin/memos/main.go
```

Rest of Dockerfile identical to `Dockerfile.pg.fly`.

#### 4.4 `scripts/fly-crdb-secrets.sh`

```bash
#!/bin/bash
set -euo pipefail

APP_NAME="bchat-crdb"

echo "Setting secrets for $APP_NAME..."

fly -a "$APP_NAME" secrets set \
  COCKROACH_DSN="postgresql://user:password@your-cluster.cockroachlabs.cloud:26257/bchat?sslmode=require"

fly -a "$APP_NAME" secrets set \
  OPENROUTER_API_KEY="sk-or-v1-xxx"

fly -a "$APP_NAME" secrets set \
  ENCRYPTION_MASTER_KEY="$(uuidgen)"

fly -a "$APP_NAME" secrets set \
  LANCEDB_S3_BUCKET="your-bucket-name"

fly -a "$APP_NAME" secrets set \
  LANCEDB_S3_PREFIX="$APP_NAME"

fly -a "$APP_NAME" secrets set \
  AWS_ACCESS_KEY_ID="xxx"

fly -a "$APP_NAME" secrets set \
  AWS_SECRET_ACCESS_KEY="xxx"

fly -a "$APP_NAME" secrets set \
  AWS_ENDPOINT_URL_S3="https://t3.storage.dev"

echo "Done. Secrets set for $APP_NAME."
```

---

### Phase 5: Verification Checklist

#### Local Verification

- [ ] `docker compose -f scripts/docker-compose.cockroach.yml up -d` starts successfully
- [ ] `task build:backend:cockroach` compiles without errors
- [ ] `task run:cockroach` starts and connects to CockroachDB
- [ ] Schema created (all tables present in CockroachDB)
- [ ] Migration history populated
- [ ] Agent vectors table created (by `CockroachVectorDB.Validate()`)
- [ ] Can create/read/update/delete memos, tickets, users
- [ ] RAG pipeline works with native vector search
- [ ] `task crdb:test` passes

#### Production Verification

- [ ] `fly deploy -c fly_crdb.toml` succeeds
- [ ] Health check passes at `/healthz`
- [ ] Migrations applied on startup
- [ ] Can connect to CockroachDB Cloud via connection string
- [ ] Vector search works with native CockroachDB vectors
- [ ] All API endpoints functional
- [ ] No SQLSTATE 40001 errors in logs (or handled gracefully)

---

## 4. Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| `SERIAL` sequence contention on CRDB | Medium | Use `INT DEFAULT unique_rowid()` in LATEST.sql |
| SQLSTATE 40001 serialization errors | High | Wrap all write transactions with `crdb.ExecuteTx` |
| `EXTRACT(EPOCH FROM NOW())::BIGINT` cast | Low | Test in local — CRDB supports this syntax |
| `ON CONFLICT DO NOTHING` | Low | Verified — works in CRDB |
| Connection pooling limits | Medium | CRDB recommends max 4x vCPU connections; pgx pool settings |
| CGO for LanceDB in Dockerfile | Low | Keep CGO_ENABLED=1; CRDB vector is pure Go via cockroach-go |
| `agent_vectors` table creation timing | Medium | Validate() creates it — ensure it runs before first vector operation |
| `migration_helper.go` SQLite-only functions | Low | Already gated on `s.profile.Driver == "sqlite"` — no change needed |

---

## 5. Files to Create/Modify

### New Files (24)

| # | File | Lines (est) |
|---|------|-------------|
| 1 | `store/db/cockroach/cockroach.go` | ~65 |
| 2 | `store/db/cockroach/common.go` | ~26 |
| 3 | `store/db/cockroach/resilience.go` | ~90 |
| 4 | `store/db/cockroach/migration_history.go` | ~60 |
| 5 | `store/db/cockroach/activity.go` | ~80 |
| 6 | `store/db/cockroach/agent.go` | ~600 |
| 7 | `store/db/cockroach/bridge.go` | ~500 |
| 8 | `store/db/cockroach/memo.go` | ~200 |
| 9 | `store/db/cockroach/ticket.go` | ~200 |
| 10 | `store/db/cockroach/user.go` | ~150 |
| 11 | `store/db/cockroach/resource.go` | ~100 |
| 12 | `store/db/cockroach/inbox.go` | ~80 |
| 13 | `store/db/cockroach/webhook.go` | ~80 |
| 14 | `store/db/cockroach/reaction.go` | ~60 |
| 15 | `store/db/cockroach/rbac.go` | ~200 |
| 16 | `store/db/cockroach/notification.go` | ~60 |
| 17 | `store/db/cockroach/workspace_setting.go` | ~60 |
| 18 | `store/db/cockroach/user_setting.go` | ~50 |
| 19 | `store/db/cockroach/idp.go` | ~60 |
| 20 | `store/db/cockroach/memo_filter.go` | ~200 |
| 21 | `store/db/cockroach/agent_observations.go` | ~80 |
| 22 | `store/db/cockroach/agent_workflow.go` | ~80 |
| 23 | `store/migration/cockroach/LATEST.sql` | ~1050 |
| 24 | `fly_crdb.toml` | ~55 |
| 25 | `Dockerfile.crdb.fly` | ~130 |
| 26 | `scripts/fly-crdb-secrets.sh` | ~40 |

### Modified Files (2)

| # | File | Change |
|---|------|--------|
| 1 | `store/db/db.go` | Add `case "cockroach":` + import |
| 2 | `internal/profile/profile.go` | Add cockroach DSN resolution |

---

## 6. Adversarial Review Prompt

```
You are a senior Go and CockroachDB architect performing an adversarial review of this
implementation plan. Your job is to find every flaw, assumption, and risk that could cause
the implementation to fail or produce incorrect results.

Review criteria:

1. CORRECTNESS: Does the plan correctly handle CockroachDB's SQL dialect differences?
   Are there Postgres features used in LATEST.sql that CockroachDB does not support?
   Will the migration system work correctly with the new driver?

2. COMPATIBILITY: Is the pgx/v5 driver truly wire-compatible with CockroachDB for all
   query patterns used in the codebase? Are there any Postgres-specific SQL idioms in the
   store methods that will fail on CockroachDB?

3. PERFORMANCE: Is `INT DEFAULT unique_rowid()` actually better than SERIAL for all
   tables? Are there tables where SERIAL is preferred? Will the connection pool settings
   be appropriate for CockroachDB?

4. SECURITY: Is the COCKROACH_DSN handling secure? Are there any places where the DSN
   could be logged or exposed? Is sslmode=require properly enforced?

5. MIGRATION SAFETY: Can the migration system safely apply LATEST.sql to an empty
   CockroachDB cluster? Will incremental migrations work? What happens if someone
   accidentally runs Postgres migrations against CockroachDB?

6. DEPLOYMENT: Is the Dockerfile.crdb.fly build correct? Will `-tags "cockroach rag"`
   produce a working binary? Are all CGO dependencies resolved?

7. OPERATIONAL: What happens when CockroachDB returns SQLSTATE 40001? Is the retry
   logic in crdb.ExecuteTx sufficient? What about connection drops?

8. EDGE CASES: What happens if the CockroachDB Cloud cluster is unreachable at startup?
   What about schema drift between Postgres and CockroachDB migrations?

For each finding, provide:
- Severity: CRITICAL / HIGH / MEDIUM / LOW
- Description of the issue
- Recommended fix

If the plan is sound, state APPROVED. If there are critical issues, state BLOCKED and
list the required fixes before implementation can begin.
```

---

## 7. Implementation Order

| Step | Task | Estimated Effort | Dependencies |
|------|------|-----------------|--------------|
| 1 | Create `store/db/cockroach/cockroach.go` (driver init) | 30 min | None |
| 2 | Create `store/db/cockroach/common.go` (placeholders) | 10 min | Step 1 |
| 3 | Create `store/db/cockroach/resilience.go` (retry logic) | 30 min | Step 1 |
| 4 | Copy all postgres store methods → cockroach (22 files) | 2-3 hrs | Steps 1-3 |
| 5 | Modify `store/db/db.go` to add cockroach case | 5 min | Step 1 |
| 6 | Modify `internal/profile/profile.go` for COCKROACH_DSN | 10 min | Step 1 |
| 7 | Create `store/migration/cockroach/LATEST.sql` | 2-3 hrs | None |
| 8 | Test local: `docker compose up` + `task run:cockroach` | 1-2 hrs | Steps 1-7 |
| 9 | Create `fly_crdb.toml` | 30 min | Steps 1-7 |
| 10 | Create `Dockerfile.crdb.fly` | 30 min | Steps 1-7 |
| 11 | Create `scripts/fly-crdb-secrets.sh` | 15 min | Step 9 |
| 12 | Test Fly.io deployment | 1-2 hrs | Steps 8-11 |

**Total estimated effort:** 8-12 hours

---

## 8. Success Criteria

1. `task run:cockroach` starts bchat against local CockroachDB with zero errors
2. All existing functionality works (CRUD, agent, RAG, tickets, bridges)
3. `fly deploy -c fly_crdb.toml` deploys successfully to Fly.io
4. CockroachDB Cloud connection works via `COCKROACH_DSN` Fly secret
5. Native vector search works with `-tags cockroach`
6. No regressions in existing Postgres/SQLite deployments
