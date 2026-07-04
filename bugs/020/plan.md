# Implementation Plan: SQLite to PostgreSQL Migration (Neon)

**Date:** 2026-07-04
**Scope:** Fresh deploy on Neon serverless Postgres, keeping SQLite for local dev
**Deploy target:** Fly.io (existing)

---

## Context

The project already has a Postgres driver (`store/db/postgres/` — 24 files) and Postgres migrations (versions 0.19-0.26 + `LATEST.sql`). However:

- The Postgres `LATEST.sql` (237 lines) is missing ~30 agent/bridge tables that exist in SQLite's `LATEST.sql` (1008 lines)
- The driver uses deprecated `lib/pq` — Neon recommends `pgx/v5`
- `profile.go` only auto-generates DSN for SQLite, not Postgres
- Postgres migrations start at 0.19 — versions 0.2-0.18 have no Postgres equivalent
- No Neon-specific connection config (SSL, pooling, timeouts)

---

## Sprint 1: Driver Swap (`lib/pq` → `pgx/v5`)

### 1.1 Replace `lib/pq` with `pgx/v5` in `go.mod`

```bash
go get github.com/jackc/pgx/v5
go mod tidy
```

Remove `github.com/lib/pq` if no longer referenced.

### 1.2 Update `store/db/postgres/postgres.go`

**Current:**
```go
import (
    _ "github.com/lib/pq"
)

func NewDB(profile *profile.Profile) (store.Driver, error) {
    db, err := sql.Open("postgres", profile.DSN)
```

**Change to:**
```go
import (
    _ "github.com/jackc/pgx/v5/stdlib"
)

func NewDB(profile *profile.Profile) (store.Driver, error) {
    db, err := sql.Open("pgx", profile.DSN)
```

`pgx/v5/stdlib` provides a `database/sql`-compatible driver named `"pgx"`. This is a drop-in replacement — all existing `database/sql` calls work unchanged.

### 1.3 Add Neon-specific connection config

Neon is serverless with connection pooling (PgBouncer). Add connection tuning:

```go
func NewDB(profile *profile.Profile) (store.Driver, error) {
    db, err := sql.Open("pgx", profile.DSN)
    if err != nil {
        return nil, errors.Wrapf(err, "failed to open database")
    }

    // Neon serverless: conservative pool settings
    db.SetMaxOpenConns(5)       // Neon free tier limit ~20 concurrent
    db.SetMaxIdleConns(2)       // Keep minimal idle connections
    db.SetConnMaxLifetime(5 * time.Minute) // Neon closes idle connections after ~5 min
    db.SetConnMaxIdleTime(1 * time.Minute)

    // Verify connection
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        return nil, errors.Wrapf(err, "failed to ping database")
    }

    return &DB{db: db, profile: profile}, nil
}
```

### 1.4 Verify compilation

```bash
go build ./store/db/postgres/...
go build ./server/...
```

---

## Sprint 2: Complete Postgres Schema

### 2.1 Audit missing tables

Compare `store/migration/sqlite/LATEST.sql` (1008 lines) against `store/migration/postgres/LATEST.sql` (237 lines).

**Missing from Postgres LATEST.sql** (tables added in SQLite versions 0.2-0.18 and agent tables):

| Table | SQLite Version | Notes |
|-------|---------------|-------|
| `agent_tenants` | 0.25 | Core tenant table |
| `agent_audiences` | 0.25 | FK → agent_tenants |
| `agent_services` | 0.25 | FK → agent_tenants |
| `agent_exclusions` | 0.25 | FK → agent_tenants |
| `agent_faqs` | 0.25 | FK → agent_tenants |
| `agent_safety_protocols` | 0.25 | FK → agent_tenants |
| `agent_kb_sections` | 0.25 | FK → agent_tenants |
| `agent_intents` | 0.25 | FK → agent_tenants |
| `agent_rules` | 0.25 | FK → agent_tenants |
| `agent_sessions` | 0.25 | TEXT PK, FK → agent_tenants |
| `agent_source_files` | 0.25 | FK → agent_tenants |
| `agent_rate_limits` | 0.25 | |
| `agent_simulation_transcripts` | 0.25 | TEXT PK |
| `agent_messages` | 0.26 | |
| `agent_tenant_scripts` | 0.25 | FK → agent_tenants |
| `agent_analysis_results` | 0.25 | TEXT PK |
| `agent_learning_memory` | 0.25 | |
| `agent_compliance_audits` | 0.25 | TEXT PK |
| `agent_scoring_config` | 0.25 | |
| `agent_qa_pairs` | 0.25 | |
| `agent_transcripts` | 0.25 | TEXT PK, FK → agent_tenants |
| `agent_leads` | 0.26 | TEXT PK |
| `agent_observations` | 0.25 | TEXT PK (session_id) |
| `agent_workflows` | 0.25 | |
| `agent_reindex_checkpoints` | 0.26 | |
| `agent_simulations` | 0.25 | TEXT PK |
| `agent_script_analysis` | 0.25 | |
| `tenant_role_templates` | 0.25 | |
| `user_tenant_permission` | 0.25 | |
| `tenant_config` | 0.25 | |
| `system_secret` | 0.25 | CHECK id=1 |
| `tickets` | 0.25 | |
| `notifications` | 0.25 | |
| `bridge_external_sessions` | 0.25 | |
| `bridge_handoffs` | 0.25 | |
| `bridge_handoff_replies` | 0.25 | |
| `bridge_reply_outbox` | 0.25 | Complex CHECK constraints |
| `bridge_auth_keys` | 0.25 | |
| `bridge_auth_nonces` | 0.25 | |

### 2.2 Update `store/migration/postgres/LATEST.sql`

Add all missing tables with Postgres-compatible syntax:

| SQLite | Postgres |
|--------|----------|
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` |
| `TEXT PRIMARY KEY` | `TEXT PRIMARY KEY` |
| `BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))` | `BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())` |
| `INTEGER NOT NULL CHECK (x IN (0, 1))` | `BOOLEAN NOT NULL DEFAULT FALSE` |
| `payload TEXT` | `payload JSONB DEFAULT '{}'` |
| `blob BLOB` | `blob BYTEA` |
| `user` (unquoted) | `"user"` (quoted — reserved word) |
| `BEGIN IMMEDIATE` | `BEGIN` (row-level locking via SELECT FOR UPDATE) |

**Important:** The Postgres schema already quotes `"user"` as a table name (line 15 of LATEST.sql). This is correct.

### 2.3 Create baseline migration

Since Postgres migrations start at 0.19, create a single baseline migration that creates all tables from scratch:

**File:** `store/migration/postgres/0.0/00__baseline.sql`

This is a complete schema with all tables. For a fresh deploy, this is the starting point.

### 2.4 Verify schema

```bash
# Connect to Neon and run the baseline
psql $DATABASE_URL -f store/migration/postgres/0.0/00__baseline.sql

# Verify all tables exist
psql $DATABASE_URL -c "\dt"
```

---

## Sprint 3: Driver Completeness Audit

### 3.1 Compare SQLite vs Postgres driver methods

The `store.Driver` interface has 90+ methods. For each file in `store/db/sqlite/`, verify the corresponding file in `store/db/postgres/` implements the same methods.

**Known gaps to check:**

| File | SQLite lines | Postgres lines | Status |
|------|-------------|----------------|--------|
| `agent.go` | 2485 | ~2400 | Check for missing methods |
| `bridge.go` | 1139 | ~900 | Check for missing methods |
| `rbac.go` | 564 | ~400 | Check for missing methods |
| `memo_filter.go` | Has test | Has test | Compare SQL syntax |
| `agent_observations.go` | Present | Present | Compare |
| `agent_workflow.go` | Present | Present | Compare |

### 3.2 For each missing method, implement it

The Postgres driver may have stub implementations or missing methods. For each gap:

1. Read the SQLite implementation
2. Translate SQL to Postgres syntax
3. Implement in the Postgres driver
4. Add to `store/db/postgres/` file

**Common SQLite → Postgres translations:**

| SQLite | Postgres |
|--------|----------|
| `ON CONFLICT(col) DO UPDATE SET` | `ON CONFLICT(col) DO UPDATE SET` (same) |
| `INSERT ... RETURNING id` | `INSERT ... RETURNING id` (same) |
| `strftime('%s', 'now')` | `EXTRACT(EPOCH FROM NOW())` |
| `AUTOINCREMENT` | `SERIAL` / `BIGSERIAL` |
| `?` placeholder | `$1, $2, ...` |
| `PRAGMA table_info(...)` | `SELECT column_name, data_type FROM information_schema.columns WHERE table_name = ...` |
| `BEGIN IMMEDIATE` | `BEGIN` + `SELECT FOR UPDATE` if needed |

### 3.3 Verify all methods compile

```bash
go build ./store/db/postgres/...
go vet ./store/db/postgres/...
```

---

## Sprint 4: Profile & Configuration

### 4.1 Update `internal/profile/profile.go`

Add Postgres DSN handling:

```go
func (p *Profile) Validate() error {
    // ... existing validation ...

    if p.Driver == "sqlite" && p.DSN == "" {
        dbFile := fmt.Sprintf("memos_%s.db", p.Mode)
        p.DSN = filepath.Join(dataDir, dbFile)
    }

    // NEW: Postgres DSN from environment
    if p.Driver == "postgres" && p.DSN == "" {
        p.DSN = os.Getenv("DATABASE_URL")
        if p.DSN == "" {
            return errors.New("postgres driver requires DSN or DATABASE_URL environment variable")
        }
    }

    return nil
}
```

### 4.2 Update driver selection in `store/store.go` or `server/server.go`

Find where the store is initialized and add driver switching:

```go
var driver store.Driver
switch profile.Driver {
case "sqlite":
    driver, err = sqlite.NewDB(profile)
case "postgres":
    driver, err = postgres.NewDB(profile)
default:
    return nil, fmt.Errorf("unsupported driver: %s", profile.Driver)
}
```

### 4.3 Update `.env.example`

```bash
# Database driver: "sqlite" or "postgres"
DB_DRIVER=sqlite

# For Postgres (Neon):
DATABASE_URL=postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require
```

### 4.4 Update `fly.toml` or Fly secrets

```bash
fly secrets set DB_DRIVER=postgres
fly secrets set DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require"
```

---

## Sprint 5: Testing

### 5.1 Unit tests against Postgres

```bash
# Run all tests with Postgres driver
go test ./store/db/postgres/... -count=1 -v

# Run all tests
go test ./server/router/api/v1/... -count=1 -race
go test ./server/router/api/v1/agent/... -count=1 -race
```

### 5.2 Integration test checklist

| Test | SQLite | Postgres |
|------|--------|----------|
| User CRUD | ✅ | Verify |
| Memo CRUD | ✅ | Verify |
| Agent tenant CRUD | ✅ | Verify |
| RBAC permissions | ✅ | Verify |
| Bridge handoffs | ✅ | Verify |
| Source file upsert | ✅ | Verify |
| Rate limiting | ✅ | Verify |
| Migration history | ✅ | Verify |

### 5.3 Neon-specific tests

- Connection pooling works (set `MaxOpenConns=5`)
- SSL connection succeeds (`sslmode=require`)
- Auto-suspend/resume works (Neon pauses after inactivity)
- Connection timeout handling (10s ping timeout)

---

## Sprint 6: Deployment

### 6.1 Neon setup (manual steps)

1. Create Neon project at https://console.neon.tech
2. Copy connection string from Dashboard → Connect
3. Format: `postgresql://[user]:[password]@[neon_hostname]/[dbname]?sslmode=require`

### 6.2 Deploy to Fly.io

```bash
# Set secrets
fly secrets set DB_DRIVER=postgres
fly secrets set DATABASE_URL="postgresql://..."

# Deploy
fly deploy

# Verify
fly logs | grep "database"
fly ssh console -C "curl http://localhost:5230/api/v1/status"
```

### 6.3 Post-deploy verification

```bash
# Check migration history
fly ssh console -C "sqlite3 /var/opt/memos/memos_prod.db 'SELECT * FROM migration_history'" # SQLite
# vs
psql $DATABASE_URL -c "SELECT * FROM migration_history" # Postgres

# Check all tables exist
psql $DATABASE_URL -c "\dt"

# Test user login
curl -X POST https://your-app.fly.dev/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"..."}'
```

---

## Neon Deployment Instructions (Separate)

See `neon_deploy.md` for step-by-step Neon console setup.

---

## Effort Estimates

| Sprint | Effort | Risk |
|--------|--------|------|
| 1: Driver swap | 1-2 hours | Low — drop-in replacement |
| 2: Schema completion | 2-3 hours | Medium — 38+ tables to translate |
| 3: Driver audit | 3-5 hours | High — 90+ methods to verify |
| 4: Profile & config | 30 min | Low |
| 5: Testing | 2-3 hours | Medium |
| 6: Deployment | 1 hour | Low |

**Total estimated effort: 1-2 days**

---

## Risk Mitigation

1. **Keep SQLite working** — All changes are additive. SQLite driver and tests remain untouched.
2. **Driver swap is safe** — `pgx/v5/stdlib` implements `database/sql` interface. No code changes needed in CRUD methods.
3. **Baseline migration** — Fresh deploy means no data migration. Single SQL file creates all tables.
4. **Neon free tier** — Can test without cost. Upgrade to paid if needed.
5. **Rollback** — Change `DB_DRIVER=sqlite` to revert to SQLite.
