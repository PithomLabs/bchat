# Adversarial Review: plan_028.md

**Verdict: REWORK REQUIRED** — Multiple factual errors in Phase 1a, 1d, 2e, and 3. Skipping these would ship a broken plan.

---

## Critical Factual Errors

### 1. Phase 1a — Column "missing" table is wrong for SQLite

The plan lists 5 columns as missing in Postgres with the implicit claim that SQLite already has them. The opposite is true for 3 of them:

| Column | SQLite LATEST.sql | Postgres LATEST.sql | Plan claim |
|--------|-------------------|---------------------|------------|
| `user_tenant_permission.source_template_id` | ✅ line 440 | ❌ missing | Says Postgres missing — correct |
| `tenant_config.admin_mutation_rate_limit_rpm` | ✅ line 460 | ❌ missing | Says Postgres missing — correct |
| `tenant_config.vector_db_s3_override` | ✅ line 461 | ❌ missing | Says Postgres missing — correct |
| `user.allowed_tenant_ids` | ❌ missing | ❌ missing | Says Postgres missing — correct |
| `memo_relation.tenant_id` | ❌ missing | ❌ missing | Says Postgres missing — correct |

The table's two-column layout ("Table | Column") implies both databases need the column, but only Postgres needs 1, 4, and 5. The plan must be rewritten to say: "Postgres is missing X, Y, Z; SQLite already has them."

### 2. Phase 3 — Postgres store layer already has most of these columns

The plan claims Phase 3 must add `source_template_id`, `admin_mutation_rate_limit_rpm`, and `tenant_id` (memo_relation) to the Postgres store layer. **They are already there:**

- `store/db/postgres/rbac.go` line 17: `INSERT INTO user_tenant_permission ... source_template_id`
- `store/db/postgres/rbac.go` line 140: `SELECT ... admin_mutation_rate_limit_rpm`
- `store/db/postgres/rbac.go` line 183: `admin_mutation_rate_limit_rpm=EXCLUDED.admin_mutation_rate_limit_rpm`
- `store/db/postgres/memo_relation.go` line 18: `tenant_id` in INSERT, line 21 ON CONFLICT, line 22 RETURNING, line 56 WHERE, line 84 SELECT, line 125 WHERE

Only `vector_db_s3_override` is actually missing from Postgres `rbac.go` (grep confirms zero matches). Phase 3 should be reduced to: add `vector_db_s3_override` to Postgres `GetTenantConfig` and `UpsertTenantConfig` queries.

### 3. Phase 1d — Internally contradictory approach

The body shows `ALTER TABLE ... ADD CONSTRAINT` syntax. The note says: *"Since this is a fresh DB created from LATEST.sql, these CHECK constraints should be added inline in the CREATE TABLE statements, not as ALTER TABLE."* You cannot do both. For a fresh-DB plan, the body must be rewritten to show inline `CHECK (...)` clauses inside the relevant `CREATE TABLE` blocks. `ALTER TABLE` is the incremental-migration approach and is out of scope per the plan's own context.

### 4. Phase 2e — Falsely implies all 4 bridge tables lack FK in both databases

The plan says "Add FK on bridge tables' tenant_id" as a SQLite fix. But **Postgres already has the FK on 3 of the 4 tables:**

- `bridge_external_sessions.tenant_id` → `agent_tenants` ✅ (Postgres line 737)
- `bridge_auth_keys.tenant_id` → `agent_tenants` ✅ (Postgres line 904)
- `bridge_auth_nonces.tenant_id` → `agent_tenants` ✅ (Postgres line 928)
- `bridge_handoffs.tenant_id` → no direct FK ❌ (Postgres line 756: `tenant_id INTEGER NOT NULL` with no REFERENCES)

The plan should say: "SQLite is missing FK on all 4 bridge tables. Postgres already has it on 3; consider adding it to `bridge_handoffs` for defense-in-depth, though cascade already works via the parent `bridge_external_sessions` FK chain."

---

## Significant Issues

### 5. Phase 1b — Some indexes are redundant with existing UNIQUE constraints

Postgres already has implicit indexes from:
- `username TEXT NOT NULL UNIQUE` → covers `idx_user_username`
- `tenant_id INTEGER NOT NULL UNIQUE` (tenant_config) → covers `idx_tenant_config_tenant`

Adding explicit `CREATE INDEX` for these is harmless but misleading in a "missing indexes" list. The plan should note these are redundant and either drop them or label them as "explicit for parity."

### 6. Phase 5b — Skip guards create a coverage hole

If SQLite-specific tests are skipped on Postgres, the 3 bridge FK fixes are never exercised on Postgres. The plan should add one of:
- Postgres-specific test variants that verify the same behavior via Postgres-compatible SQL
- A shared test helper that checks FK enforcement in a driver-agnostic way
- At minimum, an integration test that runs the bridge cascade scenario on both drivers

### 7. Phase 4a — "Add a runtime check" is underspecified

The plan says to add a guard to `migration_helper.go` but doesn't specify what the guard does: panic? return error? log + skip? This file is called from `migrator.go` which gates on driver, so the guard is defense-in-depth. Recommend: explicit `panic("migration_helper.go is SQLite-only")` with a clear message, since silent failure would mask bugs.

### 8. Phase 2a — No migration path for existing dev databases

The plan warns that `UNIQUE(user_id, tenant_id)` may fail on existing SQLite data, but only says "Verify with SELECT ... before applying." For a complete plan, specify the remediation: either deduplicate first (`DELETE ... WHERE rowid NOT IN (SELECT MIN(rowid) ...)`) or document that this change is fresh-DB-only and requires a separate migration for existing databases.

### 9. Phase 2b/2c — Same fresh-DB-only problem for NOT NULL constraints

Adding `NOT NULL` to `agent_tenants.guid`, `created_at`, etc. via ALTER TABLE on an existing database with NULL values will fail. The plan must clarify: "These changes apply to fresh databases only. Existing dev databases must be recreated or migrated via a separate script."

---

## What the Plan Gets Right

- `INSERT OR IGNORE` bug at Postgres line 672 is real and must become `ON CONFLICT (tenant_id, code) DO NOTHING`
- Postgres is genuinely missing: `user.allowed_tenant_ids`, `memo_relation.tenant_id`, `tenant_config.vector_db_s3_override`, CHECK constraints, several DEFAULT values, and 4 indexes
- SQLite bridge tables genuinely lack FK on `tenant_id` to `agent_tenants` — this is the root cause of the 3 test failures
- SQLite agent_* tables genuinely lack FKs on `tenant_id` (compare to Postgres which has them)
- `TestGetCurrentSchemaVersion` stale assertion is correctly identified

---

## Required Changes Before Approval

1. **Rewrite Phase 1a table** to separate Postgres-only from SQLite-only changes, or split into two tables.
2. **Rewrite Phase 1d body** to use inline `CHECK (...)` in `CREATE TABLE` (matching the note), not `ALTER TABLE`.
3. **Rewrite Phase 2e** to acknowledge Postgres already has FK on 3 bridge tables; decide whether `bridge_handoffs.tenant_id` needs a direct FK.
4. **Rewrite Phase 3** to reflect that Postgres store layer already has `source_template_id`, `admin_mutation_rate_limit_rpm`, and `memo_relation.tenant_id`. Only add `vector_db_s3_override`.
5. **Add Postgres verification strategy** for bridge FK tests (Phase 5b), not just skip guards.
6. **Specify migration_helper.go guard behavior** (panic vs error).
7. **Add deduplication/migration guidance** for existing dev databases affected by Phase 2a/2b/2c.
8. **Clarify fresh-DB-only scope** for NOT NULL and UNIQUE constraint additions, or provide incremental migration paths.
