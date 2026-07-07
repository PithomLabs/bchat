# Adversarial Code Review Prompt — Schema Parity Implementation

## Context

We achieved bidirectional schema parity between SQLite (local dev) and Postgres (production Neon DB) for the bchat multi-tenant AI chat agent platform. The codebase uses `store/migration/sqlite/LATEST.sql` and `store/migration/postgres/LATEST.sql` as the source of truth for fresh DBs (applied via `store/migrator.go:133-173` when no migration history exists).

This review covers all changes in the current working tree related to bugs/027 (UpsertTenantConfig missing column) and bugs/028 (4 failing tests + schema parity).

## Files to Review

### Schema Files (SQL)
- `store/migration/postgres/LATEST.sql` — Added 5 columns, 6 indexes, `ON CONFLICT DO NOTHING` fix, 7 CHECK constraints, 5 default values, 2 FKs
- `store/migration/sqlite/LATEST.sql` — Added UNIQUE constraint on `user_tenant_permission`, NOT NULL + UNIQUE on `agent_tenants.guid`, NOT NULL on 20+ timestamp columns, FK on 17 tables' `tenant_id` (including 4 bridge tables)

### Store Layer (Go)
- `store/db/postgres/rbac.go` — Added `vector_db_s3_override` to `GetTenantConfig` (SELECT/Scan) and `UpsertTenantConfig` (INSERT/ON CONFLICT)
- `store/migration_helper.go` — Added SQLite-only warning comments

### Test Files (Go)
- `store/test/migrator_test.go` — Changed schema version assertion from `0.29.x` to `0.30.x`
- `store/test/ticket_test.go` — Added SQLite skip guard + `os` import
- `store/test/bridge_auth_test.go` — Added SQLite skip guard
- `store/test/bridge_test.go` — Added SQLite skip guard + `os` import
- `store/test/schema_validation_test.go` — Added SQLite skip guards (uses `PRAGMA table_info`)
- `store/test/bridge_postgres_cascade_test.go` — New file, Postgres FK cascade verification test
- `server/router/api/v1/agent/bridge_endpoints_test.go` — Added SQLite skip guard
- `server/router/api/v1/agent/bridge_delivery_test.go` — Added SQLite skip guard
- `server/router/api/v1/agent/bridge_middleware_test.go` — Added SQLite skip guard

## Review Objectives

Perform an **adversarial** review. Your goal is to find problems, not validate correctness. Assume every change has at least one bug, one security issue, and one maintenance burden.

### 1. Correctness — Does the schema actually match?

For each change, verify:
- **Column definitions**: Are types, NULLability, defaults, and CHECK constraints exactly equivalent between SQLite and Postgres? (e.g., SQLite `BOOLEAN` is an alias for `INTEGER`, Postgres `BOOLEAN` is native — are defaults consistent?)
- **FK definitions**: Do all FKs reference the same parent table and column? Are ON DELETE/ON UPDATE clauses consistent?
- **Index definitions**: Are the same columns indexed? Are UNIQUE indexes equivalent? Are partial indexes or expression indexes missing from one side?
- **INSERT/UPSERT syntax**: Is the Postgres `ON CONFLICT` clause semantically identical to SQLite's `INSERT OR IGNORE`? Could `ON CONFLICT DO NOTHING` silently swallow errors that `INSERT OR IGNORE` would also swallow?
- **Default values**: Are timezone assumptions correct? (e.g., `NOW()` in Postgres vs `datetime('now')` in SQLite — do they produce the same epoch for `created_at`?)
- **CHECK constraints**: Are the allowed values identical? Could one database accept values the other rejects?

### 2. Security — Does this introduce vulnerabilities?

- **SQL injection**: Are any of the new SQL literals or dynamic values injectable? Review all `fmt.Sprintf` or string concatenation in the store layer.
- **Tenant isolation**: Do the new FKs actually enforce tenant isolation, or can cross-tenant data leak through JOINs or subqueries that bypass the FK?
- **Error information leakage**: Does `UpsertTenantConfig`'s new error handling expose `vector_db_s3_override` values in error messages? Could S3 credentials leak via logs or API responses?
- **Permission escalation**: Do the new CHECK constraints or defaults create paths where a lower-privilege user can set values they shouldn't?
- **Race conditions**: Can concurrent `UpsertTenantConfig` calls cause data corruption with the new `ON CONFLICT` clause?

### 3. Migration Safety — Will this break existing databases?

- **Column addition**: Adding NOT NULL columns to existing tables without defaults — will SQLite/Postgres reject existing rows? What happens to databases that already have data?
- **FK addition**: Adding FKs to existing tables — will the ALTER TABLE succeed if orphaned rows exist? Is there a data backfill step missing?
- **Index addition**: Will `CREATE INDEX IF NOT EXISTS` block writes on large tables? Is there a risk of exceeding Neon's storage during index creation?
- **Constraint addition**: Adding CHECK constraints — will existing data violate them? Is `ALTER TABLE ... ADD CONSTRAINT` safe on Postgres with live data?
- **Fresh DB vs upgrade**: The plan says "fresh DB only" — but what if someone runs `LATEST.sql` against an existing database that went through incremental migrations? Will the `IF NOT EXISTS` clauses cause silent conflicts?

### 4. Test Coverage — Are the tests actually testing what they claim?

- **Skip guards**: The skip pattern `if os.Getenv("DRIVER") != "" && os.Getenv("DRIVER") != "sqlite"` — is this correct? What if `DRIVER` is unset (empty string)? Does the condition mean "skip if DRIVER is set to anything other than sqlite"?
- **Schema validation test**: `TestSchemaValidation` uses `PRAGMA table_info()` which is SQLite-only. Is skipping it on Postgres actually the right approach, or should the test be rewritten to use `information_schema.columns`?
- **Postgres cascade test**: `TestPostgresBridgeFKCascade` creates a session, deletes the tenant, checks cascade — but does it verify the FK constraint name matches what's in LATEST.sql? Could the test pass even if the FK is missing?
- **Stale version assertion**: Changing `0.29.x` to `0.30.x` — is this a band-aid? What happens when the next migration bumps to `0.31.x`? Should the test be version-agnostic?
- **Missing test cases**: Are there scenarios NOT covered? (e.g., updating a tenant config when `vector_db_s3_override` is NULL, bulk operations, concurrent FK deletions)

### 5. Maintainability — Does this create tech debt?

- **Dual schema drift**: We now maintain two LATEST.sql files that must be kept in sync. Is there a validation script or CI check that enforces this? What happens when someone adds a column to SQLite but forgets Postgres?
- **Comment warnings**: The "SQLite-only" comments in `migration_helper.go` — are they enforceable, or just documentation that will be ignored?
- **Skip guard proliferation**: We added SQLite skip guards to 7 test files. Is this pattern sustainable, or should there be a test helper like `SkipIfNotSQLite(t)`?
- **CHECK constraint duplication**: We added CHECK constraints inline in CREATE TABLE blocks. Are these also defined in the incremental migration files? Could there be conflicts?
- **vector_db_s3_override**: This is a new column in the `tenant_config` table. Is it documented? Is there a migration for existing databases? What happens if someone queries this column on an old database?

### 6. Edge Cases — What can go wrong?

- **Empty/nil values**: What happens when `vector_db_s3_override` is an empty JSON object `{}` vs `null` vs missing?
- **Concurrent modifications**: Two admins editing the same tenant config simultaneously — does `UpsertTenantConfig` handle this safely?
- **Large payloads**: `vector_db_s3_override` stores S3 credentials — is there a size limit? Could a malicious payload exceed column limits?
- **Cascading deletes**: If a tenant is deleted, what happens to `vector_db_s3_override` data? Is there sensitive data that should be wiped, not just cascade-deleted?
- **Cross-database queries**: If someone runs a query joining SQLite and Postgres tables (e.g., in a migration script), will the different FK behaviors cause issues?

## Deliverable

For each category (Correctness, Security, Migration Safety, Test Coverage, Maintainability, Edge Cases), provide:

1. **Critical issues** — Must fix before merge. These are bugs that will cause data loss, security breaches, or test failures.
2. **Warnings** — Should fix. These are correctness issues that may cause subtle bugs in production.
3. **Suggestions** — Nice to have. These are maintainability or clarity improvements.
4. **Questions** — Need clarification. These are design decisions that may have been intentional but seem risky.

Use the format:
```
### [Category]

#### Critical Issues
- **[File:Line]** Description of issue. Impact. Suggested fix.

#### Warnings
- **[File:Line]** Description of concern. Potential impact.

#### Suggestions
- **[File:Line]** Description of improvement.

#### Questions
- **[File:Line]** What was the reasoning behind X?
```

## Specific Attack Vectors to Probe

1. **Can `ON CONFLICT DO NOTHING` in Postgres silently drop errors that `INSERT OR IGNORE` in SQLite would also drop?** Trace the exact semantics of both.
2. **Does adding `NOT NULL` to `agent_tenants.guid` break the `UpsertAgentTenant` path?** What if an existing row has NULL guid?
3. **Is `vector_db_s3_override` stored in plaintext?** Should it be encrypted at rest? Is there a migration path for existing configs?
4. **Can the skip guard pattern `DRIVER != "" && DRIVER != "sqlite"` be bypassed?** What if DRIVER is set to `SQLITE` (uppercase)?
5. **Does `TestSchemaValidation` actually validate anything useful?** It checks column names exist, but not types, defaults, or constraints.
6. **Are the 7 new CHECK constraints in Postgres also enforced in SQLite?** SQLite CHECK constraints were already defined — verify they're identical.
7. **Can `bridge_external_sessions` FK cascade delete sessions that are still referenced by active conversations?**
8. **Is the `0.30.x` version assertion in migrator_test.go brittle?** Should it assert `>= 0.30.0` instead of exact prefix match?
