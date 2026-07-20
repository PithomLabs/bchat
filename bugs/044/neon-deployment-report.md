# Neon PostgreSQL Deployment Report

**Date:** 2026-07-20
**Database:** Neon PostgreSQL (bchat-pg)
**Fly App:** bchat-pg
**Status:** Pre-Deployment Complete — Awaiting Deploy

---

## Deployment Overview

### Purpose

Deploy database migration parity fixes to Neon PostgreSQL:
1. **Postgres `GetSystemSecret`/`UpsertSystemSecret`** — Replace stubs with real SQL queries
2. **Missing `system_secret` migration** — Add incremental migration for upgrade paths
3. **SQLite `max_message_length` fix** — Correct wrong default value (SQLite only, no Neon impact)

### Why This Matters

The Postgres store had stub implementations for `SystemSecret` operations, silently breaking API key encryption and transcript signing on Neon. The migration ensures upgrade paths from pre-0.26 databases get the `system_secret` table.

---

## Pre-Deployment Checklist

| # | Step | Status | Notes |
|---|------|--------|-------|
| 1 | Local Postgres started | ✅ | Docker Compose, postgres:16-alpine |
| 2 | Migration validation | ✅ | `validate-pg-migrations.sh` passed |
| 3 | Fresh schema test | ✅ | LATEST.sql creates 56 tables |
| 4 | Sequential migration test | ✅ | All migrations 0.19→0.33 apply cleanly |
| 5 | `0.33/00__add_system_secret.sql` created | ✅ | IF NOT EXISTS, idempotent |
| 6 | Postgres store implementation | ✅ | GetSystemSecret + UpsertSystemSecret |
| 7 | Build verification | ✅ | `go build ./...` clean |
| 8 | Vet verification | ✅ | `go vet ./store/db/postgres/...` clean |
| 9 | Store tests | ✅ | All pass (sqlite, postgres, mysql) |
| 10 | Neon backup branch | ✅ | Created manually via Neon Dashboard |

---

## Local Postgres Validation Report

### Test Environment

- **Postgres Version:** 16.14 (Alpine)
- **Container:** bchat-postgres (Docker Compose)
- **Connection:** `postgresql://bchat:bchat@localhost:5432/bchat`
- **Validation Script:** `scripts/validate-pg-migrations.sh`

### Step 0: Database Connectivity

```
PASSED: Database is reachable
```

### Step 1: Fresh Schema from LATEST.sql

```
PASSED: Created database with 56 tables
```

LATEST.sql applied successfully in a transaction. All 56 tables created:
- Core tables: user, memo, resource, activity, idp, inbox, webhook, reaction
- Agent tables: agent_tenants, agent_audiences, agent_services, agent_sessions, etc.
- RBAC tables: tenant_role_templates, user_tenant_permission, tenant_config, system_secret
- Bridge tables: bridge_external_sessions, bridge_handoffs, bridge_handoff_replies, etc.

### Step 2: Database Reset

```
PASSED
```

Test database dropped and recreated for sequential migration test.

### Step 3: Sequential Migration Application

```
PASSED: All migrations applied, 11 tables
```

**Migration directories applied:**
```
0.19 → 0.20 → 0.21 → 0.22 → 0.23 → 0.24 → 0.25 → 0.26 → 0.27 → 0.28 → 0.29 → 0.30 → 0.31 → 0.32 → 0.33
```

**Key migrations verified:**
- `0.26/00__agent_tenant_rbac_foundation.sql` — Creates agent_tenants, agent_audiences, user_tenant_permission, tenant_config
- `0.30/00__tenant_role_templates.sql` — Creates tenant_role_templates with seed data
- `0.30/01__agent_observations.sql` — Creates agent_observations
- `0.31/00__agent_integrations.sql` — Creates agent_integrations
- `0.32/01__transcript_signing_key.sql` — Adds transcript_signing_key columns
- **`0.33/00__add_system_secret.sql`** — Creates system_secret table ✅

### Step 4: Schema Comparison

```
WARNING: Table list differs between LATEST.sql and migrations
```

**This is EXPECTED and NOT an error.**

**Explanation:** The incremental migration path (0.19→0.33) creates only 11 tables because it assumes the baseline schema already exists from a prior `LATEST.sql` install. Most tables (45 of 56) are created by `LATEST.sql` on fresh installs, not by incremental migrations.

**How the Go migrator handles this:**

```
preMigrate():
  ├── migration_history empty? → Apply LATEST.sql (56 tables) → Insert version
  └── migration_history exists? → Skip LATEST.sql

Migrate():
  ├── Check latest migration_history version
  ├── Compare against code version (0.33)
  └── Apply any unapplied incremental migrations (0.33/00)
```

**On Neon:** The database was created from LATEST.sql previously, so all 56 tables exist. The `0.33/00` migration only adds `system_secret` if missing (IF NOT EXISTS).

### Validation Result

```
=== All Checks Passed ===
Postgres migrations are ready for deployment
```

---

## Files Changed

### Modified

| File | Change |
|------|--------|
| `store/db/postgres/rbac.go` | Implemented `GetSystemSecret` and `UpsertSystemSecret` |

### Created

| File | Purpose |
|------|---------|
| `store/migration/postgres/0.33/00__add_system_secret.sql` | Adds system_secret table for upgrade paths |
| `store/migration/sqlite/0.33/00__fix_max_message_length_default.sql` | Fixes max_message_length default (SQLite only) |

### Documentation

| File | Purpose |
|------|---------|
| `bugs/044/plan.md` | Original audit plan |
| `bugs/044/plan_review.md` | Plan review with rework requests |
| `bugs/044/signoff.md` | Signoff with adversarial findings |
| `bugs/044/neon-deployment-report.md` | This report |

---

## Migration Detail: `0.33/00__add_system_secret.sql`

```sql
CREATE TABLE IF NOT EXISTS system_secret (
    id SERIAL PRIMARY KEY CHECK (id = 1),
    encryption_salt BYTEA NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    rotated_at BIGINT
);
```

**Safety properties:**
- `IF NOT EXISTS` — Safe for fresh installs (table already from LATEST.sql) and upgrades
- `CHECK (id = 1)` — Enforces singleton constraint
- `BYTEA` — Correct type for binary encryption salt
- `EXTRACT(EPOCH FROM NOW())::BIGINT` — Matches codebase convention

---

## Postgres Store Implementation

### Before (Stub)

```go
func (d *DB) GetSystemSecret(ctx context.Context) (*store.SystemSecret, error) {
    return nil, nil  // Silently fails
}

func (d *DB) UpsertSystemSecret(ctx context.Context, secret *store.SystemSecret) (*store.SystemSecret, error) {
    return nil, nil  // Silently discards writes
}
```

### After (Live)

```go
func (d *DB) GetSystemSecret(ctx context.Context) (*store.SystemSecret, error) {
    query := `SELECT id, encryption_salt, key_version, created_at, rotated_at FROM system_secret WHERE id = 1`
    // ... scans BYTEA, BIGINT into Go types, returns nil,nil on ErrNoRows
}

func (d *DB) UpsertSystemSecret(ctx context.Context, secret *store.SystemSecret) (*store.SystemSecret, error) {
    stmt := `INSERT INTO system_secret (id, encryption_salt, key_version, created_at)
             VALUES (1, $1, $2, $3)
             ON CONFLICT(id) DO UPDATE SET
                 encryption_salt = EXCLUDED.encryption_salt,
                 key_version = EXCLUDED.key_version,
                 rotated_at = $4
             RETURNING id`
    // ... uses Postgres $N params, EXCLUDED syntax
}
```

**Pattern follows:** `tenant_config` upsert at `store/db/postgres/rbac.go:180-206`

---

## Neon Schema Verification (Post-Deploy)

**Status:** Pending — fill after `fly deploy`

### Query: Check system_secret exists

```sql
SELECT EXISTS (
    SELECT FROM information_schema.tables
    WHERE table_name = 'system_secret'
) AS has_system_secret;
```

**Expected:** `true` (either from LATEST.sql or 0.33 migration)

### Query: Table count

```sql
SELECT count(*) AS table_count
FROM information_schema.tables
WHERE table_schema = 'public';
```

**Expected:** 56

### Query: Last migration

```sql
SELECT version, created_ts
FROM migration_history
ORDER BY created_ts DESC LIMIT 3;
```

**Expected:** `0.33.1` (our new migration)

---

## Rollback Plan

### If deploy fails

```bash
fly releases rollback --app bchat-pg
```

### If database migration causes issues

1. Restore from Neon backup branch (created pre-deploy)
2. Redeploy previous binary version

### If SystemSecret implementation has bugs

The previous behavior was `return nil, nil`. Reverting to stubs:
- Restores silent failure (no encryption, no signing)
- No data loss — the `system_secret` table remains but is unused

---

## Risk Assessment

| Item | Risk | Mitigation |
|------|------|-----------|
| `0.33/00` migration | **Low** | IF NOT EXISTS is idempotent |
| SystemSecret store | **Low** | Follows established pattern, tested locally |
| Neon cold start | **Low** | 60s timeout handles autosuspend |
| Existing data | **None** | No ALTER TABLE, no data migration |
| Build failure | **Low** | `go build` and `go vet` passed locally |

---

## Deployment Commands

```bash
# 1. Commit (recommended)
git add .
git commit -m "fix: Postgres SystemSecret + migration parity (0.33)"

# 2. Deploy
fly deploy --config fly_pg.toml --app bchat-pg

# 3. Verify
fly logs --app bchat-pg 2>&1 | grep -i "migrate"
curl https://bchat-pg.fly.dev/healthz
```

---

## Open Items

| # | Item | Owner | Status |
|---|------|-------|--------|
| 1 | Neon backup branch | User | ✅ Created |
| 2 | Git commit | User | ⬜ Pending |
| 3 | `fly deploy` | User | ⬜ Pending |
| 4 | Post-deploy schema verification | User | ⬜ Pending |
| 5 | MySQL stubs tracking | Follow-up | ⬜ Out of scope |

---

*Report Version: 1.0*
*Created: 2026-07-20 — Pre-deployment documentation*
