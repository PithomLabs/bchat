# Implementation Plan: SQLite to PostgreSQL Migration (Neon) — Revised

**Date:** 2026-07-04
**Revision:** Per `plan_review.md` findings
**Scope:** Fresh deploy on Neon serverless Postgres, keeping SQLite for local dev
**Deploy target:** Fly.io (existing)
**Bridge:** Full implementation (all methods, feature parity with SQLite)

---

## Corrected Numbers

| Metric | Original (wrong) | Corrected |
|--------|------------------|-----------|
| Missing Postgres tables | 38+ | **34** (53 SQLite - 19 Postgres) |
| Stub methods to implement | "90+ to verify" | **91 stubs** in `agent.go` + **9 bridge stubs** in `bridge.go` |
| Total driver methods | "90+" | **183** (both drivers have exactly 183) |
| Effort estimate | 1-2 days | **3-5 days** (Sprint 3 alone is 3-5 days) |

---

## Sprint 1: Driver Swap (`lib/pq` → `pgx/v5`) — 1-2 hours

### 1.1 Replace dependency

```bash
go get github.com/jackc/pgx/v5
go get github.com/lib/pq@none  # explicitly remove
go mod tidy
```

Verify `lib/pq` is gone from `go.mod`:
```bash
grep "lib/pq" go.mod  # should return nothing
```

### 1.2 Update `store/db/postgres/postgres.go`

```go
import (
    _ "github.com/jackc/pgx/v5/stdlib"
)

func NewDB(profile *profile.Profile) (store.Driver, error) {
    db, err := sql.Open("pgx", profile.DSN)
    if err != nil {
        return nil, errors.Wrapf(err, "failed to open database")
    }

    // Neon serverless: conservative pool settings
    // Neon free tier allows ~20 concurrent connections
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)

    // Verify connection with configurable timeout
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        return nil, errors.Wrapf(err, "failed to ping database")
    }

    return &DB{db: db, profile: profile}, nil
}
```

### 1.3 Verify compilation

```bash
go build ./store/db/postgres/...
go build ./server/...
```

---

## Sprint 2: Complete Postgres Schema — 2-3 hours

### 2.1 Missing tables (34 total)

All tables that exist in SQLite `LATEST.sql` but not in Postgres `LATEST.sql`:

**Agent tables (20):**
`agent_tenants`, `agent_audiences`, `agent_services`, `agent_exclusions`, `agent_faqs`, `agent_safety_protocols`, `agent_kb_sections`, `agent_intents`, `agent_rules`, `agent_sessions`, `agent_source_files`, `agent_rate_limits`, `agent_simulation_transcripts`, `agent_messages`, `agent_tenant_scripts`, `agent_analysis_results`, `agent_learning_memory`, `agent_compliance_audits`, `agent_scoring_config`, `agent_qa_pairs`

**Agent tables continued (8):**
`agent_transcripts`, `agent_leads`, `agent_observations`, `agent_workflows`, `agent_reindex_checkpoints`, `agent_simulations`, `agent_script_analysis`, `agent_workflows`

**Infrastructure tables (6):**
`tenant_role_templates`, `user_tenant_permission`, `tenant_config`, `system_secret`, `tickets`, `notifications`

**Bridge tables (6):**
`bridge_external_sessions`, `bridge_handoffs`, `bridge_handoff_replies`, `bridge_reply_outbox`, `bridge_auth_keys`, `bridge_auth_nonces`

### 2.2 SQLite → Postgres column translations

| SQLite | Postgres |
|--------|----------|
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` |
| `BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))` | `BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())` |
| `INTEGER NOT NULL CHECK (x IN (0, 1))` | `BOOLEAN NOT NULL DEFAULT FALSE` |
| `payload TEXT` | `payload JSONB DEFAULT '{}'` |
| `blob BLOB` | `blob BYTEA` |
| `user` (unquoted) | `"user"` (reserved word) |
| `CHECK(status IN ('active','closed','expired'))` | `CHECK(status IN ('active','closed','expired'))` (same) |

### 2.3 Baseline migration with version guard

**File:** `store/migration/postgres/0.0/00__baseline.sql`

**Version guard logic** — the migration runner must:
1. Check if `migration_history` table exists
2. If yes, skip baseline (existing deployment with 0.19+ migrations)
3. If no, run baseline (fresh deploy)

```sql
-- Only run on fresh databases
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'migration_history') THEN
        -- Create all tables here
        -- (full schema from LATEST.sql)
    END IF;
END $$;
```

### 2.4 Verify schema

```bash
psql $DATABASE_URL -f store/migration/postgres/0.0/00__baseline.sql
psql $DATABASE_URL -c "\dt"  # should show all 53 tables
```

---

## Sprint 3: Driver Implementation — 3-5 days

### 3.1 Systematic SQL translation strategy

Every query string in the Postgres driver needs `?` → `$1, $2, ...` conversion.

**Mechanical approach:**
```bash
# Find all queries with ? placeholders in postgres driver
grep -n '?' store/db/postgres/*.go | grep -v '//' | grep -v '_test.go'
```

**Translation rules:**
1. `?` → `$1`, `$2`, `$3`... (positional, not named)
2. `ON CONFLICT(col) DO UPDATE SET` → same (Postgres 9.5+)
3. `INSERT ... RETURNING id` → same
4. `strftime('%s', 'now')` → `EXTRACT(EPOCH FROM NOW())`
5. `PRAGMA table_info(...)` → `SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1`
6. `BEGIN IMMEDIATE` → `BEGIN` (use `SELECT FOR UPDATE` for explicit locking)

### 3.2 Implement `agent.go` stubs (91 methods)

All methods returning `errNotImplemented` need full implementation.

**Source:** `store/db/sqlite/agent.go` (2485 lines)
**Target:** `store/db/postgres/agent.go` (~2400 lines)

For each method:
1. Read SQLite implementation
2. Translate `?` → `$1, $2, ...`
3. Translate SQLite-specific SQL
4. Implement in Postgres driver
5. Verify compilation

### 3.3 Implement `bridge.go` stubs (9 methods)

All methods returning `ErrBridgeUnsupportedDatabase` need full implementation.

**Source:** `store/db/sqlite/bridge.go` (1139 lines)
**Target:** `store/db/postgres/bridge.go` (~900 lines)

**Critical:** After implementing all bridge methods, update `SupportsBridgeDelivery()` to return `true`.

### 3.4 Implement remaining stubs

Check all other Postgres driver files for `errNotImplemented` stubs:
- `agent_observations.go`
- `agent_workflow.go`
- `rbac.go`
- `memo_filter.go`

### 3.5 Verify compilation

```bash
go build ./store/db/postgres/...
go vet ./store/db/postgres/...
```

---

## Sprint 4: Profile & Configuration — 30 minutes

### 4.1 Update `internal/profile/profile.go`

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

### 4.2 Update store initialization

Find where the store is created and add driver switching:

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
DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require"
```

### 4.4 Update Fly.io secrets

```bash
fly secrets set DB_DRIVER=postgres
fly secrets set DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require"
```

---

## Sprint 5: Testing — 2-3 hours

### 5.1 Port `memo_filter_test.go`

SQLite has `store/db/sqlite/memo_filter_test.go`. Create equivalent:
`store/db/sqlite/memo_filter_test.go` exists but `store/db/postgres/memo_filter_test.go` does not.

Port the test with Postgres-specific SQL translations.

### 5.2 Test harness

Create a test database target. Options:
- **Local:** Docker Postgres container (`docker run -e POSTGRES_PASSWORD=test -p 5432:5432 postgres:17`)
- **Neon:** Use Neon free-tier project for integration tests
- **CI:** GitHub Actions with Postgres service container

### 5.3 Test matrix

| Test | SQLite | Postgres |
|------|--------|----------|
| User CRUD | ✅ | Verify |
| Memo CRUD | ✅ | Verify |
| Agent tenant CRUD | ✅ | Verify |
| Agent sessions | ✅ | Verify |
| RBAC permissions | ✅ | Verify |
| Bridge handoffs | ✅ | Verify |
| Source file upsert | ✅ | Verify |
| Rate limiting | ✅ | Verify |
| Migration history | ✅ | Verify |
| memo_filter | ✅ | **Port test** |

### 5.4 Neon-specific tests

- Connection pooling works (`MaxOpenConns=10`)
- SSL connection succeeds (`sslmode=require`)
- Channel binding works (`channel_binding=require`)
- Auto-suspend/resume works (Neon pauses after inactivity)
- Connection timeout handling (10s ping timeout)

---

## Sprint 6: Deployment — 1 hour

### 6.1 Neon setup

1. Create Neon project at https://console.neon.tech
2. Copy connection string from Dashboard → Connect
3. Format: `postgresql://[user]:[password]@[neon_hostname]/[dbname]?sslmode=require&channel_binding=require`

### 6.2 Deploy to Fly.io

```bash
fly secrets set DB_DRIVER=postgres
fly secrets set DATABASE_URL="postgresql://...?sslmode=require&channel_binding=require"
fly deploy
```

### 6.3 Post-deploy verification

```bash
# Check migration history
psql $DATABASE_URL -c "SELECT * FROM migration_history"

# Check all tables exist
psql $DATABASE_URL -c "\dt"  # should show 53 tables

# Test user login
curl -X POST https://your-app.fly.dev/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"..."}'

# Test bridge feature (if applicable)
curl https://your-app.fly.dev/api/v1/agent/tenants
```

---

## Effort Estimates (Revised)

| Sprint | Effort | Risk |
|--------|--------|------|
| 1: Driver swap | 1-2 hours | Low |
| 2: Schema completion | 2-3 hours | Medium |
| 3: Driver implementation | **3-5 days** | High — 100 methods |
| 4: Profile & config | 30 min | Low |
| 5: Testing | 2-3 hours | Medium |
| 6: Deployment | 1 hour | Low |

**Total estimated effort: 4-6 days**

---

## Risk Mitigation

1. **Keep SQLite working** — All changes are additive. SQLite driver untouched.
2. **Driver swap is safe** — `pgx/v5/stdlib` implements `database/sql`.
3. **Version guard on baseline** — Won't conflict with existing 0.19+ deployments.
4. **Bridge is binary** — Either fully implemented or `SupportsBridgeDelivery()` stays false. We chose full implementation.
5. **Rollback** — Change `DB_DRIVER=sqlite` to revert.
