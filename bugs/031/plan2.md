# Implementation Plan: Consolidated Review Fixes for LATEST.sql

## Review Synthesis

Three adversarial reviews were conducted on `store/migration/postgres/LATEST.sql`:
- **hyk3** — Verdict: SHIP IT (for fresh Neon deploy), with upgrades flagged
- **DeepSeek V4 Flash** — Verdict: FIX FIRST (execute tolerance, delivery_status CHECK)
- **Stepfun** — Verdict: RISKY (execute tolerance, INSERT idempotency)

All three agree on the FK ordering fix being correct. The disagreements are about severity of secondary issues. This plan addresses every valid finding across all three reviews.

---

## Consolidated Findings (deduplicated)

| ID | Finding | Reviews | Severity | Action |
|----|---------|---------|----------|--------|
| C-001 | `execute()` tolerates "already exists" causing partial schema commit | DeepSeek, Stepfun | CRITICAL | **DROPPED** — would break upgrades; C-001 premise flawed |
| C-002 | `bridge_handoff_replies.delivery_status` CHECK tautology | DeepSeek | LOW (no runtime impact) | Fix in `LATEST.sql` |
| H-001 | `tenant_role_templates` INSERT lacks `ON CONFLICT DO NOTHING` | All 3 | HIGH | Fix in `LATEST.sql` |
| H-002 | `memo_organizer.pinned` INTEGER vs `memo.pinned` BOOLEAN | DeepSeek | LOW (pq handles both) | Skip — pre-existing, pq driver handles it |
| H-003 | Boolean/integer schema drift between Postgres and SQLite | DeepSeek, Stepfun | LOW (intentional dialect diff) | Skip — by design |
| H-004 | Incremental migrations missing `tenant_role_templates`, `agent_observations`, `source_template_id` | hyk3 | HIGH (upgrade path) | Create `0.30` migration |
| H-005 | `IF NOT EXISTS` inconsistency across 35+ tables | hyk3, DeepSeek, Stepfun | MEDIUM | Standardize in `LATEST.sql` |
| M-001 | `validate-pg-migrations.sh` doesn't match production tx behavior | hyk3, DeepSeek, Stepfun | MEDIUM | Update validation script |
| M-002 | `agent_tenant_scripts` FK syntax inconsistency (table-level vs inline) | DeepSeek, Stepfun | LOW | Skip — cosmetic |
| M-003 | No standalone `tenant_id` FK on `bridge_handoff_replies` | DeepSeek | LOW | Skip — enforced via composite FKs |
| L-001 | `memo_organizer` missing FK constraints | DeepSeek | LOW (pre-existing) | Skip — pre-existing |
| L-002 | Partial index `'/m/%'` semantic issue | DeepSeek, Stepfun | LOW | Skip — application-layer concern |

---

## Implementation Plan

> **Note:** Step 1 of the original plan (`execute()` tolerance change) was **dropped** per the hyk3 plan2 review. It would break upgrades — versioned migrations 0.20, 0.25 use bare `CREATE TABLE` without `IF NOT EXISTS`. The C-001 premise is flawed: `tx.ExecContext(wholeFile)` aborts at first error regardless of tolerance, and failed deploys roll back fully. See `plan2_signoff.md` for full rationale.

### Step 1 (renumbered): Add `ON CONFLICT` to Seed INSERT (H-001)

**File:** `store/migration/postgres/LATEST.sql:199-205`

**Problem:** No idempotency guard on the `tenant_role_templates` INSERT. A re-run would fail on duplicate key.

**Fix:** Add `ON CONFLICT (tenant_id, code) DO NOTHING`:
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

### Step 3 (renumbered): Fix `delivery_status` CHECK Tautology (C-002)

**File:** `store/migration/postgres/LATEST.sql:808`

**Problem:** `CHECK(delivery_status = 'not_delivered')` makes the column immutable. No UPDATE can ever change it.

**Current state:** The Go code only ever INSERTs with `"not_delivered"` (verified: no UPDATE statements for `delivery_status` exist). So this is a latent bug, not an active runtime failure.

**Fix:** Expand the CHECK to allow all expected values:
```sql
delivery_status TEXT NOT NULL DEFAULT 'not_delivered'
    CHECK(delivery_status IN ('not_delivered', 'delivered', 'failed')),
```

Also apply the same fix to `store/migration/sqlite/LATEST.sql:868` for consistency.

### Step 4 (renumbered): Create Incremental Migration `0.30` (H-004)

**Directory:** `store/migration/postgres/0.30/`

**Problem:** Existing databases upgraded via versioned migrations (0.25-0.29) are missing:
1. `tenant_role_templates` table (new in `LATEST.sql`)
2. `agent_observations` table (new in `LATEST.sql`)
3. `source_template_id` column on `user_tenant_permission` (new in `LATEST.sql`)
4. `idx_user_tenant_permission_template` index (new in `LATEST.sql`)
5. `idx_tenant_config_tenant` index (new in `LATEST.sql`)

**Files to create:**

`store/migration/postgres/0.30/00__tenant_role_templates.sql`:
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

`store/migration/postgres/0.30/01__agent_observations.sql`:
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

`store/migration/postgres/0.30/02__user_tenant_permission_source_template.sql`:
```sql
ALTER TABLE user_tenant_permission ADD COLUMN IF NOT EXISTS source_template_id INTEGER REFERENCES tenant_role_templates(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_template ON user_tenant_permission(source_template_id);
CREATE INDEX IF NOT EXISTS idx_tenant_config_tenant ON tenant_config(tenant_id);
```

### Step 5 (renumbered): Update Validation Script (M-001)

**File:** `scripts/validate-pg-migrations.sh`

**Problem:** The script runs `psql < LATEST.sql` outside a transaction, which doesn't match the production `tx.ExecContext()` behavior. It also only warns (exit 0) on table-list differences.

**Fixes:**

1. Wrap `LATEST.sql` execution in a transaction to match production:
```bash
# Instead of:
psql "$TEST_URL" < "$LATEST_SQL"
# Use:
echo "BEGIN; $(cat "$LATEST_SQL") COMMIT;" | psql "$TEST_URL"
```

2. Change the schema comparison from WARNING to ERROR (exit 1) when table lists differ.

3. Fix the premise comment in the script header — psql does NOT defer FK validation (this is correct per hyk3's analysis). The script's FK ordering check is valid; the issue was that the old `LATEST.sql` had genuinely wrong ordering, and the script correctly detected it. Update comments accordingly.

---

## What We're NOT Fixing (and Why)

| Finding | Reason to Skip |
|---------|---------------|
| H-002 (memo_organizer.pinned type) | `pq` driver handles INTEGER→bool coercion. Pre-existing, not introduced by this fix. |
| H-003 (boolean/integer schema drift) | Intentional dialect difference. SQLite uses INTEGER for booleans, Postgres uses BOOLEAN. Go driver handles both. |
| M-002 (agent_tenant_scripts FK syntax) | Cosmetic inconsistency only. Table-level vs inline FK is valid Postgres either way. |
| M-003 (bridge_handoff_replies no standalone tenant_id FK) | Referential integrity enforced via composite FKs. No standalone FK is intentional. |
| L-001 (memo_organizer missing FKs) | Pre-existing design decision. Adding FKs now would be a schema change beyond scope. |
| L-002 (partial index '/m/%' pattern) | Application-layer semantic issue. The pattern works correctly for the memo reference format. |

---

## Execution Order

> **Note:** Step 1 (`execute()` tolerance) was dropped per plan2 review. See `plan2_signoff.md`.

1. **Step 1** (LATEST.sql) — Add `ON CONFLICT` to seed INSERT
2. **Step 2** (new files in 0.30/) — Create incremental migrations
3. **Step 3** (postgres.go) — Force pgx simple protocol for Neon
4. **Step 4** (Taskfile_pg.yml) — Add `validate:no-libpq` CI guard
5. **Step 5** (LATEST.sql + sqlite LATEST.sql) — Fix `delivery_status` CHECK (LOW)
6. **Step 6** (validate-pg-migrations.sh) — Update validation script (LOW)
7. **Step 7** (AGENTS.md) — Document pgx as sole driver (LOW)
8. Run `go build ./...` to verify
9. Run `task -t Taskfile_pg.yml validate:migrations` to verify
10. Deploy with `fly -a bchat-pg deploy -c fly_pg.toml`

---

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| `execute()` tolerance tightening | Could break incremental migrations that use "already exists" | Only removed tolerance for CREATE TABLE/INDEX, kept for ALTER TABLE ADD COLUMN |
| `ON CONFLICT DO NOTHING` | Could mask real conflicts | UNIQUE constraint on `(tenant_id, code)` ensures correctness |
| `delivery_status` CHECK expansion | Could allow unexpected values | Only 3 known values: not_delivered, delivered, failed |
| New 0.30 migration | Could conflict with existing data | All statements use `IF NOT EXISTS` and `ON CONFLICT` |
| Validation script changes | Could produce false positives | Transaction wrapping matches production behavior exactly |

---

## Section 2: pgx Consolidation and Neon Hardening

**Source:** `plan_pgx_hy3.md`, `plan_pgx_deepseek.md`, verified against pgx source code

### Current State

- `lib/pq` is **already fully eliminated** — no `go.mod`, no imports, no usage anywhere in the codebase
- The codebase is 100% `github.com/jackc/pgx/v5` via `database/sql` stdlib wrapper
- Single import: `store/db/postgres/postgres.go:9` → `_ "github.com/jackc/pgx/v5/stdlib"`
- Single open: `store/db/postgres/postgres.go:26` → `sql.Open("pgx", profile.DSN)`
- No `Prepare()` call sites exist in `store/db/postgres/` — verified via grep
- No `pq.Array`, `pq.CopyIn`, or native `TEXT[]` columns exist

### The Problem: PgBouncer Transaction Mode

Neon runs PgBouncer in **transaction pooling mode** behind the `-pooler` endpoint. PgBouncer transaction mode returns connections to the pool after each transaction. pgx's default mode (`cache_statement`) uses prepared statements cached across transactions — if a connection is reassigned, the next query on a different connection fails:

```
ERROR: prepared statement "1" does not exist (26000)
```

This is the classic "pq worked but pgx fails" problem. `lib/pq` uses simple protocol only and works fine behind pgbouncer. pgx defaults to extended protocol with statement caching, which breaks.

### Why simple_protocol is Correct for Neon

From pgx source code (`conn.go`):
- `default_query_exec_mode=simple_protocol` → `QueryExecModeSimpleProtocol`
- Uses client-side parameter interpolation, single round trip, no prepared statements
- Explicitly recommended for "connecting to a proxy server, connection pool server, or non-PostgreSQL server that does not support the extended protocol"
- Alternative `exec` mode also works (extended protocol but no caching), but `simple_protocol` is the safest choice for pgbouncer transaction mode

### JSONB Safety Check

DeepSeek flagged a concern: simple protocol encodes `[]byte` as PostgreSQL `bytea` instead of `jsonb`. Verified safe: all JSONB columns in `LATEST.sql` use `TEXT`/`JSONB DEFAULT '{}'` and the Go code passes `string` values (not `[]byte`) for JSONB fields. No risk of bytea contamination.

### Implementation Steps

#### Step 6: Force pgx simple protocol for Neon (from plan_pgx_hy3 + plan_pgx_deepseek)

**File:** `store/db/postgres/postgres.go`

**Problem:** pgx defaults to `cache_statement` mode, which uses prepared statements incompatible with Neon's pgbouncer transaction mode.

**Fix:** Append `default_query_exec_mode=simple_protocol` to the DSN before opening.

```go
import (
    "context"
    "database/sql"
    "log"
    "strings"
    "time"

    _ "github.com/jackc/pgx/v5/stdlib"
    "github.com/pkg/errors"

    "github.com/usememos/memos/internal/profile"
    "github.com/usememos/memos/store"
)

func NewDB(profile *profile.Profile) (store.Driver, error) {
    if profile == nil {
        return nil, errors.New("profile is nil")
    }

    dsn := profile.DSN
    if !strings.Contains(dsn, "default_query_exec_mode") {
        sep := "?"
        if strings.Contains(dsn, "?") {
            sep = "&"
        }
        dsn += sep + "default_query_exec_mode=simple_protocol"
    }

    db, err := sql.Open("pgx", dsn)
    if err != nil {
        log.Printf("Failed to open database: %s", err)
        return nil, errors.Wrapf(err, "failed to open database: %s", profile.DSN)
    }

    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)

    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        return nil, errors.Wrapf(err, "failed to ping database")
    }

    return &DB{db: db, profile: profile}, nil
}
```

**Effect:** All queries (including `LATEST.sql` multi-statement execution in `preMigrate`) use simple protocol. No prepared statements are cached. Fully compatible with Neon pgbouncer transaction mode.

**Performance note:** Simple protocol sends results as text (not binary). ~5-10% overhead for numeric/timestamp parsing. Negligible for bchat's workload.

#### Step 7: Add CI guard to prevent lib/pq re-entry

**File:** `Taskfile.yml`

**Problem:** Nothing prevents a future dependency or developer from re-introducing `lib/pq` into `go.mod`.

**Fix:** Add a validation task:
```yaml
  validate:no-libpq:
    desc: Fail if github.com/lib/pq re-enters the dependency tree
    cmds:
      - '! grep -q "lib/pq" go.mod'
```

Wire it as a dependency of `validate:migrations` in `Taskfile_pg.yml` (which is already in the `build:backend` chain):
```yaml
  validate:migrations:
    desc: Validate Postgres LATEST.sql is in sync with migration files
    deps: [validate:no-libpq]
    cmds:
      - ./scripts/validate-pg-migrations.sh
```

#### Step 8: Document pgx as sole driver

**File:** `AGENTS.md`

Add a note in the Technology Stack section:
```markdown
| LLM Provider | OpenRouter API |
| Vector Database | LanceDB (optional, for RAG) |
| Postgres Driver | pgx/v5 (sole driver — `lib/pq` is NOT used and must not be added) |
```

Add a note in the Environment Variables section:
```markdown
**Postgres DSN:** Uses `default_query_exec_mode=simple_protocol` automatically. No manual DSN tuning needed for Neon.
```

---

## Updated Execution Order

1. **Step 1** (migrator.go) — Fix `execute()` tolerance
2. **Step 2** (LATEST.sql) — Add `ON CONFLICT` to INSERT
3. **Step 3** (LATEST.sql + sqlite LATEST.sql) — Fix `delivery_status` CHECK
4. **Step 4** (new files in 0.30/) — Create incremental migrations
5. **Step 5** (validate-pg-migrations.sh) — Update validation script
6. **Step 6** (postgres.go) — Force pgx simple protocol for Neon
7. **Step 7** (Taskfile.yml + Taskfile_pg.yml) — Add `validate:no-libpq` CI guard
8. **Step 8** (AGENTS.md) — Document pgx as sole driver
9. Run `go build ./...` to verify compilation
10. Run `task -t Taskfile_pg.yml validate:migrations` to verify
11. Deploy with `fly -a bchat-pg deploy -c fly_pg.toml`

---

## Updated Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| `execute()` tolerance tightening | Could break incremental migrations that use "already exists" | Only removed tolerance for CREATE TABLE/INDEX, kept for ALTER TABLE ADD COLUMN |
| `ON CONFLICT DO NOTHING` | Could mask real conflicts | UNIQUE constraint on `(tenant_id, code)` ensures correctness |
| `delivery_status` CHECK expansion | Could allow unexpected values | Only 3 known values: not_delivered, delivered, failed |
| New 0.30 migration | Could conflict with existing data | All statements use `IF NOT EXISTS` and `ON CONFLICT` |
| Validation script changes | Could produce false positives | Transaction wrapping matches production behavior exactly |
| `default_query_exec_mode=simple_protocol` | 5-10% query overhead from text encoding | Negligible for bchat workload; eliminates pgbouncer prepared-statement failures |
| CI guard `validate:no-libpq` | Could block legitimate transitive deps | `lib/pq` is not a transitive dep of any current dependency |
