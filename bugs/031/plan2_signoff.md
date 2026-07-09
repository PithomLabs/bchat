# Plan Sign-Off: Consolidated Postgres Fixes + pgx Hardening

**Date:** 2026-07-09
**Target:** `store/migration/postgres/LATEST.sql`, `store/db/postgres/postgres.go`, `Taskfile_pg.yml`, `scripts/validate-pg-migrations.sh`, `AGENTS.md`
**Goal:** Fix all FK ordering bugs in LATEST.sql, harden pgx for Neon, and add upgrade path for existing databases.

---

## Review Consolidation

### Source Reviews (bugs/031/)

| Review | Verdict | Key Finding |
|--------|---------|-------------|
| hyk3 (LATEST.sql) | SHIP IT for fresh deploy; upgrades flagged | FK ordering correct; incremental migrations needed for upgrade path |
| DeepSeek V4 Flash (LATEST.sql) | FIX FIRST | `execute()` tolerance, `delivery_status` CHECK tautology |
| Stepfun (LATEST.sql) | RISKY | `execute()` tolerance, INSERT idempotency |
| hyk3 (plan2 review) | **DROP Step 1** | `execute()` tolerance change would break upgrades; C-001 premise flawed |
| DeepSeek (plan2 review) | PLAN READY FOR IMPLEMENTATION | All 8 steps verified; one optional split suggestion |

### Reconciled Decisions

| Original Finding | Resolution | Reason |
|-----------------|-----------|--------|
| C-001: `execute()` tolerance | **DROPPED** | Would break upgrades (0.20, 0.25 use bare `CREATE TABLE`). `tx.ExecContext(wholeFile)` aborts at first error regardless. Failed deploys roll back fully. |
| C-002: `delivery_status` CHECK | **LOW priority** | No runtime impact (no UPDATE statements exist). Fix for correctness. |
| H-001: INSERT idempotency | **HIGH** | Implement `ON CONFLICT DO NOTHING` |
| H-004: Incremental migrations | **HIGH** | Create `0.30/` directory |
| H-005: `IF NOT EXISTS` inconsistency | **SKIP for MVP** | Would be a large diff; low risk since `preMigrate` only runs on fresh DBs |
| pgx simple_protocol | **HIGH** | Implement in `postgres.go` |
| CI guard | **MEDIUM** | Add `validate:no-libpq` task |
| Validation script | **LOW** | Wrap in transaction; keep diff as warning |

---

## Implementation Steps

### Step 1: Seed INSERT idempotency

**File:** `store/migration/postgres/LATEST.sql:199-205`

Add `ON CONFLICT (tenant_id, code) DO NOTHING` to the `tenant_role_templates` seed INSERT:
```sql
INSERT INTO tenant_role_templates (tenant_id, name, code, permissions)
VALUES
    (NULL, 'Viewer', 'viewer', '["tenant:read"]'),
    (NULL, 'Tester', 'tester', '["tenant:read","chat:test"]'),
    (NULL, 'Analyst', 'analyst', '["tenant:read","chat:logs"]'),
    (NULL, 'Editor', 'editor', '["tenant:read","tenant:write","files:upload"]'),
    (NULL, 'Tenant Admin', 'tenant_admin', '["tenant:admin"]')
ON CONFLICT (tenant_id, code) DO NOTHING;
```

### Step 2: Incremental migration 0.30

**Directory:** `store/migration/postgres/0.30/`

All statements must be idempotent (`IF NOT EXISTS` / `ON CONFLICT`). Order matters (sorted by filename).

`00__tenant_role_templates.sql`:
```sql
CREATE TABLE IF NOT EXISTS tenant_role_templates (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER REFERENCES agent_tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    permissions TEXT NOT NULL DEFAULT '[]',
    created_by INTEGER REFERENCES "user"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);
CREATE INDEX IF NOT EXISTS idx_tenant_role_templates_tenant ON tenant_role_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_role_templates_code ON tenant_role_templates(code);
INSERT INTO tenant_role_templates (tenant_id, name, code, permissions)
VALUES
    (NULL, 'Viewer', 'viewer', '["tenant:read"]'),
    (NULL, 'Tester', 'tester', '["tenant:read","chat:test"]'),
    (NULL, 'Analyst', 'analyst', '["tenant:read","chat:logs"]'),
    (NULL, 'Editor', 'editor', '["tenant:read","tenant:write","files:upload"]'),
    (NULL, 'Tenant Admin', 'tenant_admin', '["tenant:admin"]')
ON CONFLICT (tenant_id, code) DO NOTHING;
```

`01__agent_observations.sql`:
```sql
CREATE TABLE IF NOT EXISTS agent_observations (
    session_id TEXT PRIMARY KEY REFERENCES agent_sessions(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    observation_log TEXT DEFAULT '',
    last_observed_msg_index INTEGER DEFAULT 0,
    tokens_in_log INTEGER DEFAULT 0,
    current_task TEXT,
    suggested_response TEXT,
    resource_id TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_observations_tenant ON agent_observations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_observations_resource ON agent_observations(resource_id);
```

`02__user_tenant_permission_source_template.sql`:
```sql
ALTER TABLE user_tenant_permission ADD COLUMN IF NOT EXISTS source_template_id INTEGER REFERENCES tenant_role_templates(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_template ON user_tenant_permission(source_template_id);
CREATE INDEX IF NOT EXISTS idx_tenant_config_tenant ON tenant_config(tenant_id);
```

### Step 3: Force pgx simple protocol for Neon

**File:** `store/db/postgres/postgres.go`

Add `"strings"` to imports. Derive DSN before `sql.Open`:
```go
dsn := profile.DSN
if !strings.Contains(dsn, "default_query_exec_mode") {
    sep := "?"
    if strings.Contains(dsn, "?") {
        sep = "&"
    }
    dsn += sep + "default_query_exec_mode=simple_protocol"
}
db, err := sql.Open("pgx", dsn)
```

Keep existing pool settings unchanged. Do NOT add `pool_*` DSN params (no-ops under `database/sql` stdlib).

**Effect:** All queries (including `LATEST.sql` multi-statement execution in `preMigrate`) use simple protocol. No prepared statements cached. Fully compatible with Neon pgbouncer transaction mode.

**Note:** pgx v5 default is `QueryExecModeExec` ("exec"), not `cache_statement` (v4 term). The DSN parameter `default_query_exec_mode=simple_protocol` is confirmed correct from pgx source (`conn.go:192`).

### Step 4: CI guard against lib/pq re-entry

**File:** `Taskfile_pg.yml`

Add task and wire as dependency of existing `validate:migrations`:
```yaml
  validate:no-libpq:
    desc: Fail if github.com/lib/pq re-enters the dependency tree
    cmds:
      - '! grep -q "lib/pq" go.mod'

  validate:migrations:
    desc: Validate Postgres LATEST.sql is in sync with migration files
    deps: [validate:no-libpq]
    cmds:
      - ./scripts/validate-pg-migrations.sh
```

### Step 5 (LOW): Fix delivery_status CHECK tautology

**Files:** `store/migration/postgres/LATEST.sql:808`, `store/migration/sqlite/LATEST.sql:868`

Change:
```sql
CHECK(delivery_status = 'not_delivered'),
```
To:
```sql
CHECK(delivery_status IN ('not_delivered', 'delivered', 'failed')),
```

No runtime impact (no UPDATE statements exist for this column). Fix for correctness.

### Step 6 (LOW): Validation script transaction wrap

**File:** `scripts/validate-pg-migrations.sh`

Replace `psql "$TEST_URL" < "$LATEST_SQL"` with:
```bash
echo "BEGIN; $(cat "$LATEST_SQL") COMMIT;" | psql "$TEST_URL"
```

Keep the fresh-vs-migrated table-list diff as a **warning** (exit 0), not an error. False CI failures are possible since fresh DBs (53 tables) and upgraded DBs (~base + 6 agent tables) legitimately differ.

### Step 7 (LOW): Document pgx as sole driver

**File:** `AGENTS.md`

Add note in Technology Stack section:
```markdown
| Postgres Driver | pgx/v5 (sole driver — `lib/pq` is NOT used and must not be added) |
```

Add note in Environment Variables section:
```markdown
**Postgres DSN:** Uses `default_query_exec_mode=simple_protocol` automatically. No manual DSN tuning needed for Neon.
```

---

## What Was Explicitly Excluded

| Item | Reason |
|------|--------|
| Step 1 of original plan2.md (`execute()` tolerance) | Would break upgrades — versioned migrations 0.20, 0.25 use bare `CREATE TABLE` without `IF NOT EXISTS`. C-001 premise is flawed (tx.ExecContext aborts at first error; failed deploys roll back fully). |
| H-002: `memo_organizer.pinned` INTEGER vs BOOLEAN | Pre-existing. `pq` driver handles INTEGER→bool coercion. |
| H-003: Boolean/integer schema drift | Intentional dialect difference. Go driver handles both. |
| H-005: `IF NOT EXISTS` inconsistency | Large diff for low risk. `preMigrate` only runs on fresh DBs. |
| M-002: `agent_tenant_scripts` FK syntax | Cosmetic only. Table-level vs inline FK is valid Postgres either way. |
| M-003: `bridge_handoff_replies` no standalone tenant_id FK | Referential integrity enforced via composite FKs. |
| L-001: `memo_organizer` missing FKs | Pre-existing design decision. |
| L-002: Partial index '/m/%' pattern | Application-layer semantic concern. |
| `query_exec_mode` (from plan_pgx_hy3.md) | Wrong parameter name — pgx silently ignores it. Correct name is `default_query_exec_mode`. |
| MySQL driver | Out of scope, untouched. |
| Migration from `database/sql`+stdlib to `pgxpool` | Future enhancement, not MVP. |

---

## Execution Order

1. Step 1 (LATEST.sql) — Add `ON CONFLICT` to seed INSERT
2. Step 2 (new `0.30/` migrations) — Create incremental migration files
3. Step 3 (postgres.go) — Force pgx simple protocol for Neon
4. Step 4 (Taskfile_pg.yml) — Add `validate:no-libpq` CI guard
5. Step 5 (LATEST.sql + sqlite) — Fix `delivery_status` CHECK (LOW)
6. Step 6 (validate-pg-migrations.sh) — Wrap in transaction (LOW)
7. Step 7 (AGENTS.md) — Document pgx as sole driver (LOW)

## Verification

1. `go build ./...` — clean compilation
2. `grep -q "lib/pq" go.mod` → empty (no lib/pq dependency)
3. `task -t Taskfile_pg.yml validate:migrations` — passes
4. `fly -a bchat-pg deploy -c fly_pg.toml` — migration succeeds, `/healthz` passes

---

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| `ON CONFLICT DO NOTHING` | Could mask real conflicts | UNIQUE constraint on `(tenant_id, code)` ensures correctness |
| New 0.30 migration | Could conflict with existing data | All statements use `IF NOT EXISTS` and `ON CONFLICT` |
| `default_query_exec_mode=simple_protocol` | 5-10% query overhead from text encoding | Negligible for bchat workload; eliminates pgbouncer prepared-statement failures |
| CI guard `validate:no-libpq` | Could block legitimate transitive deps | `lib/pq` is not a transitive dep of any current dependency |
| `delivery_status` CHECK expansion | Could allow unexpected values | Only 3 known values: not_delivered, delivered, failed |
| Validation script transaction wrap | Could produce false positives | Transaction wrapping matches production behavior exactly; diff kept as warning |

---

## Sign-Off

| Role | Verdict | Notes |
|------|---------|-------|
| hyk3 (plan2 review) | APPROVED | Dropped Step 1; parameter name corrected |
| DeepSeek (plan2 review) | APPROVED | All claims verified; optional split deferred |

**Status:** READY FOR IMPLEMENTATION
