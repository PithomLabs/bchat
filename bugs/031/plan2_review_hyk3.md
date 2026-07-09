# Final Plan Review: `bugs/031/plan2.md` → MVP for Coding Agent

**Reviewer:** hyk3
**Input reviewed:** `bugs/031/plan2.md` (388 lines — synthesizes the three `LATEST.sql` reviews + the pgx/Neon consolidation).
**Goal:** Produce a minimum-viable, self-contained plan a coding agent can execute without another review round.

---

## 1. Review Verdict

`plan2.md` is a solid synthesis, and its pgx/Neon section (Steps 6–8) is **correct**. However, it contains **one harmful step** and **one parameter-name nuance** that must be fixed before hand-off.

### 1.1 CRITICAL — Step 1 (`execute()` tolerance change) will break upgrades — DROP IT

The plan removes "already exists" tolerance for `CREATE TABLE`/`CREATE INDEX`, asserting only `ALTER TABLE ADD COLUMN` relies on it. **This is false.** Versioned migrations use bare `CREATE TABLE` (no `IF NOT EXISTS`):

- `store/migration/postgres/0.20/00__reaction.sql`
- `store/migration/postgres/0.25/00__tickets.sql`
- `store/migration/postgres/0.25/01__notifications.sql`

On an **existing-DB upgrade**, re-running these hits `relation "…" already exists`. With the plan's change, `execute()` would no longer tolerate that and would **hard-fail the migration** → broken deploy for every tenant upgrading a non-fresh DB.

Additionally, the C-001 premise is flawed:
- A single `tx.ExecContext(wholeFile)` **aborts the entire file at the first erroring statement** regardless of tolerance — so "already exists" tolerance does not cause a "partial commit"; it just suppresses that one error.
- Failed deploys **roll back fully** (transactional DDL; `preMigrate` returns before `tx.Commit()`, `defer tx.Rollback()` at `migrator.go:156`). So the "tables exist but `migration_history` empty" scenario the plan fears does not arise from a failed deploy.

**Action:** Leave `store/migrator.go` `execute()` unchanged. Do **not** implement Step 1.

### 1.2 Parameter-name nuance — plan2.md is RIGHT, `plan_pgx_hy3.md` is WRONG

The pgx connection-string key is **`default_query_exec_mode`** (confirmed in source: `conn.go:192` reads `config.RuntimeParams["default_query_exec_mode"]`; `conn_test.go:179` → `pgx.ParseConfig("default_query_exec_mode=simple_protocol")`).

- `plan2.md` Step 6 correctly uses `default_query_exec_mode=simple_protocol` ✅
- `plan_pgx_hy3.md` (this author's earlier file) mistakenly used `query_exec_mode=simple_protocol`, which pgx **silently ignores** (unknown runtime param) → the fix would not actually apply.

**Action:** Use `default_query_exec_mode` in the final plan (already correct in plan2.md). Fix `plan_pgx_hy3.md` to match.

### 1.3 Verified-correct items
- **Step 3 (`delivery_status` CHECK):** both `store/migration/postgres/LATEST.sql:808` and `store/migration/sqlite/LATEST.sql:868` contain `CHECK(delivery_status = 'not_delivered')` → edit is accurate. Keep as low-priority.
- **Step 7 (CI guard wiring):** `Taskfile_pg.yml` includes `Taskfile.yml` and overrides `validate:migrations` at line 22 (runs `validate-pg-migrations.sh`), so `validate:no-libpq` belongs there. ✅
- **Step 4 (0.30 migration):** internal FK ordering is valid (tenant_role_templates created in `00__`, `source_template_id` added in `02__` which sorts after). ✅

### 1.4 Minor
- **Step 5 (validation diff → error):** flipping the table-list diff from warning to `exit 1` can cause false CI failures (fresh vs fully-migrated tables may still legitimately differ). Keep as **warning** for MVP.
- **Step 6 terminology:** pgx v5 default is `QueryExecModeExec` ("exec"), not `cache_statement` (a v4 term). The *action* (set `default_query_exec_mode=simple_protocol`) is correct.

---

## 2. Final MVP Plan (hand-off ready)

### M1 — Seed INSERT idempotency (`store/migration/postgres/LATEST.sql`)
Add `ON CONFLICT (tenant_id, code) DO NOTHING` to the `tenant_role_templates` seed INSERT (lines 199–205):
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

### M2 — Incremental migration `store/migration/postgres/0.30/` (upgrade path)
All statements idempotent (`IF NOT EXISTS` / `ON CONFLICT`). Order matters (sorted by filename).

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

### M3 — Force pgx simple protocol for Neon (`store/db/postgres/postgres.go`)
Add `"strings"` import; derive DSN before `sql.Open("pgx", …)`:
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
Keep existing pool settings (`SetMaxOpenConns(10)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5m)`, `SetConnMaxIdleTime(1m)`). **Do not** add `pool_*` DSN params (no-ops under `database/sql` stdlib).

### M4 — CI guard against `lib/pq` re-entry (`Taskfile_pg.yml`)
Add task and wire into the existing `validate:migrations` (line 22):
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

### M5 (optional/low) — `delivery_status` CHECK (`LATEST.sql` + `sqlite/LATEST.sql`)
Change `CHECK(delivery_status = 'not_delivered')` →
`CHECK(delivery_status IN ('not_delivered', 'delivered', 'failed'))` at postgres `:808` and sqlite `:868`.

### M6 (optional) — Validation script transaction wrap (`scripts/validate-pg-migrations.sh`)
Replace `psql "$TEST_URL" < "$LATEST_SQL"` with `echo "BEGIN; $(cat "$LATEST_SQL") COMMIT;" | psql "$TEST_URL"` so it exercises the production transaction path. Keep the fresh-vs-migrated table-list diff as a **warning** (exit 0), not an error.

### M7 (optional) — `AGENTS.md` note
Add: *"Postgres driver: pgx/v5 only (`lib/pq` is not used and must not be re-added). DSN auto-appends `default_query_exec_mode=simple_protocol` for Neon."*

---

## 3. Explicitly Excluded
- **Step 1 of plan2.md** (`execute()` tolerance change) — dropped (see §1.1; would break upgrades).
- `plan_pgx_hy3.md` had the wrong DSN key (`query_exec_mode`) — correct it to `default_query_exec_mode` to stay consistent with M3.
- MySQL driver (`go-sql-driver/mysql`) — out of scope, untouched.
- Migrating `database/sql`+stdlib to `pgxpool` — future enhancement, not MVP.

---

## 4. Execution Order & Verification
1. M1 (LATEST.sql INSERT) → 2. M2 (0.30 migration) → 3. M3 (postgres.go) → 4. M4 (Taskfile_pg.yml) → 5. [M5/M6/M7 optional].
6. `go build ./...` (clean).
7. `grep -q "lib/pq" go.mod` → empty.
8. `task -t Taskfile_pg.yml validate:migrations` (needs local Postgres) → passes; multi-statement `LATEST.sql` applies under `simple_protocol`.
9. `fly -a bchat-pg deploy -c fly_pg.toml` → migration succeeds, `/healthz` passes.
