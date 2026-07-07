# Adversarial Code Review: bugs/028 Schema Parity Implementation

**Scope:** Current working tree changes for bugs/027 (UpsertTenantConfig missing column) and bugs/028 (4 failing tests + schema parity), per `bugs/028/prompt_code_review.md`.

---

## 1. Correctness

### Critical Issues

- **`server/router/api/v1/agent/bridge_endpoints_test.go:744`** — `TestBridgeReplySuccessPersisted` issues a `SELECT name FROM sqlite_master ...` query without a skip guard. The function has no `t.Skip` for non-SQLite drivers, so this test will fail with "sqlite_master does not exist" on Postgres. **Impact:** CI fails on Postgres. **Fix:** Add `if os.Getenv("DRIVER") != "" && os.Getenv("DRIVER") != "sqlite" { t.Skip("SQLite-specific test") }` at the top of the function.

- **`store/migration/sqlite/LATEST.sql:219-236` vs `store/migration/postgres/LATEST.sql:161-178`** — `agent_audiences` schema parity is incomplete. SQLite still has nullable columns (`guidelines TEXT DEFAULT '[]'`, `secondary_phones TEXT DEFAULT '[]'`, `emergency_urgency_threshold INTEGER DEFAULT 4`, `escalation_confidence_threshold REAL DEFAULT 0.85`, `rate_limit_rpm INTEGER DEFAULT 60`, `require_contact_on_fallback INTEGER DEFAULT 1`, `max_message_length INTEGER DEFAULT 2000`, `updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`) while Postgres has them as `NOT NULL`. SQLite also lacks the `CHECK (audience_type IN ('internal', 'external'))` constraint that Postgres has on `audience_type`. **Impact:** Insert/update behavior diverges between dev (SQLite) and prod (Postgres). A row valid in SQLite may be rejected in Postgres. **Fix:** Apply the missing `NOT NULL` constraints and `audience_type` CHECK to SQLite `LATEST.sql`.

- **`store/migration/sqlite/LATEST.sql:456-460` vs `store/migration/postgres/LATEST.sql:196-205`** — `tenant_config` schema parity is incomplete. SQLite still has nullable columns (`simulation_human_model TEXT DEFAULT ''`, `retrieval_mode TEXT DEFAULT 'long_context'`, `content_tokens INTEGER DEFAULT 0`, `record_transcripts INTEGER DEFAULT 1`, `reasoning_model TEXT DEFAULT ''`) while Postgres has them as `NOT NULL`. **Impact:** Same as above — divergent behavior between dev and prod. **Fix:** Apply missing `NOT NULL` constraints to SQLite `tenant_config`.

### Warnings

- **`store/migration/postgres/LATEST.sql:387-388` vs `store/migration/sqlite/LATEST.sql:364-365`** — `agent_sessions.created_at` and `updated_at` are `TIMESTAMPTZ DEFAULT NOW()` in Postgres and `TIMESTAMP DEFAULT CURRENT_TIMESTAMP` in SQLite, and **neither has `NOT NULL`**. The plan's Phase 2c claimed to add `NOT NULL` to match Postgres, but Postgres never had it. **Impact:** NULL timestamps can be inserted in both databases, which may break Go code that assumes non-nil `time.Time`. **Fix:** Decide whether `NOT NULL` is desired and apply consistently to both, or remove from the plan.

- **`store/migration/postgres/LATEST.sql:775` vs `store/migration/sqlite/LATEST.sql:796`** — `bridge_handoffs.active` is `BOOLEAN DEFAULT TRUE CHECK(active IN (TRUE, FALSE))` in Postgres but `INTEGER DEFAULT 1 CHECK(active IN (0, 1))` in SQLite. Both are functionally correct within their engines, but the type mismatch means any cross-database raw SQL (e.g., in a migration or reporting query) will behave differently. **Impact:** Low — the Go layer has driver-specific `scanBridgeHandoff` functions that handle each type correctly. **Fix:** None required for current architecture, but document the intentional divergence.

- **`store/migration/postgres/LATEST.sql:670`** — `CREATE TABLE IF NOT EXISTS tenant_role_templates` uses `IF NOT EXISTS` while the SQLite counterpart (line 409) uses plain `CREATE TABLE`. For a fresh-DB plan, `IF NOT EXISTS` masks errors if the table already exists from a partial migration. **Impact:** Silent schema drift if someone runs `LATEST.sql` against a partially-migrated DB. **Fix:** Either remove `IF NOT EXISTS` from Postgres for consistency, or document that it's intentional for idempotent re-runs.

### Suggestions

- **`store/db/postgres/rbac.go:182-191`** — `UpsertTenantConfig` uses `COALESCE` for encrypted fields (`openrouter_api_key_encrypted`, `openrouter_api_key_nonce`) but raw `EXCLUDED.vector_db_s3_override` for the new plaintext field. If a caller passes an empty string for `VectorDBS3Override`, it will overwrite an existing non-empty value. Consider whether this is intentional or whether `COALESCE` should be used for consistency.

---

## 2. Security

### Critical Issues

- **`store/migration/postgres/LATEST.sql:206` and `store/migration/sqlite/LATEST.sql:462`** — `vector_db_s3_override` is stored as plaintext `TEXT DEFAULT ''`, while sibling credential fields like `openrouter_api_key_encrypted` use `BYTEA` + `BLOB` with encryption. The prompt explicitly flags this: *"Is `vector_db_s3_override` stored in plaintext? Should it be encrypted at rest?"* **Impact:** S3 credentials (potentially AWS keys) are exposed in database backups, logs, and any process with DB read access. **Fix:** Either encrypt the column like the OpenRouter key fields, or document a security exception with compensating controls (e.g., IAM role restrictions, DB-level encryption).

### Warnings

- **`store/migration_helper.go:19`** — `AddColumnIfNotExists` builds SQL via `fmt.Sprintf("PRAGMA table_info(%s)", tableName)`. While current callers pass hardcoded table names, the function signature accepts arbitrary strings. A future caller passing user input would create an SQL injection vector. **Impact:** Low today, but the function is exported and reusable. **Fix:** Add a comment warning against user-supplied `tableName`, or validate against a allowlist.

---

## 3. Migration Safety

### Critical Issues

- **`store/migration/sqlite/LATEST.sql:205`** — `agent_tenants.guid` is now `TEXT NOT NULL UNIQUE`. Any existing dev database with NULL `guid` values will fail on migration. The plan acknowledges this but only says "Existing dev databases must be recreated." **Impact:** Developers with existing dev DBs lose data or must manually backfill. **Fix:** Provide a backfill migration: `UPDATE agent_tenants SET guid = lower(hex(randomblob(16))) WHERE guid IS NULL;` before adding the constraint, or ship an incremental migration instead of relying on fresh-DB-only.

- **`store/migration/sqlite/LATEST.sql:441`** — `UNIQUE(user_id, tenant_id)` on `user_tenant_permission` will reject existing duplicate rows. The plan provides deduplication SQL but doesn't mention running it automatically. **Impact:** Migration fails on existing DBs with duplicates. **Fix:** Add an incremental migration that deduplicates before adding the constraint, or document the required manual step.

### Warnings

- **`store/migration/sqlite/LATEST.sql:219-236`** — Adding `NOT NULL` to `agent_audiences` columns without defaults on an existing DB will fail if any row has NULL. Same risk as `agent_tenants.guid` above. **Fix:** Either backfill defaults first or enforce fresh-DB-only with clear documentation.

---

## 4. Test Coverage

### Critical Issues

- **`server/router/api/v1/agent/bridge_endpoints_test.go:744`** — As noted in Correctness, `TestBridgeReplySuccessPersisted` lacks a skip guard for `sqlite_master`. This is a test coverage gap that causes CI failures on Postgres.

### Warnings

- **`store/test/bridge_postgres_cascade_test.go:15-51`** — The new Postgres cascade test verifies behavior (rows disappear after tenant delete) but not the mechanism (FK constraint exists). It would pass even if cascade was implemented via application-level deletes rather than DB-level `ON DELETE CASCADE`. **Impact:** If someone later replaces the FK with soft-delete logic, this test won't catch the regression. **Fix:** Add a second assertion that queries `information_schema.table_constraints` to verify the FK constraint name exists.

- **`store/test/bridge_test.go:41`** — `TestBridgeExternalSessionUsesAgentTenantsTable` now runs on both SQLite and Postgres without a skip guard. This is correct behavior (it tests a portable invariant), but it relies on FK enforcement being active in both drivers. If a future SQLite configuration disables `foreign_keys`, this test will silently pass on a broken setup. **Impact:** Low. **Fix:** None needed, but consider adding a debug log of `PRAGMA foreign_keys` status.

- **Skip guard pattern** — All skip guards use `os.Getenv("DRIVER") != "" && os.Getenv("DRIVER") != "sqlite"`. This is correct, but `getDriverFromEnv()` defaults to `"sqlite"` when unset. If a developer sets `DRIVER=SQLITE` (uppercase), the skip guard treats it as non-SQLite and skips. **Impact:** Low — environment variables are typically lowercase. **Fix:** Document that `DRIVER` must be lowercase `sqlite`, or use `strings.EqualFold`.

---

## 5. Maintainability

### Warnings

- **Dual LATEST.sql drift** — The plan claims "100% schema parity" but the current files have measurable drift (missing NOT NULL, missing CHECK, type mismatches). There is no CI validation that enforces parity. **Impact:** Future changes will drift further. **Fix:** Add a CI script that diffs the two LATEST.sql files for structural parity (column names, nullability, defaults, constraints) and fails on divergence.

- **Skip guard proliferation** — 7 test files now contain the same `os.Getenv("DRIVER")` pattern. This is technical debt. **Fix:** Extract to `teststore.SkipIfNotSQLite(t)` helper.

- **`store/migration_helper.go:10-13`** — The "WARNING" comment is documentation-only. The functions are still exported and callable from Postgres code. **Impact:** A future contributor may call `AddColumnIfNotExists` from a Postgres migration, producing a runtime error. **Fix:** Rename to unexported `addColumnIfNotExistsSQLite` as the v2 review suggested, or add a runtime driver check that panics with a clear message.

### Suggestions

- **`store/test/schema_validation_test.go:66`** — The test validates column *names* but not types, defaults, or constraints. It would not catch the `agent_audiences` NOT NULL drift or the `audience_type` CHECK drift. **Fix:** Expand validation to include `information_schema` (Postgres) and `PRAGMA table_info` (SQLite) for nullability and type checks.

---

## 6. Edge Cases

### Warnings

- **`vector_db_s3_override` NULL/empty semantics** — In Postgres, the column is `TEXT DEFAULT ''` (never NULL). In SQLite, same. The Go `sql.NullString` scan in `GetTenantConfig` treats empty string as `Valid=true, String=""`. This is consistent. **Impact:** None. **Fix:** None needed.

- **`agent_tenants.guid` uniqueness on UUID** — `guid TEXT NOT NULL UNIQUE` with auto-generated UUIDs should be safe. But `CreateAgentTenant` in SQLite (line 26) uses `uuid.New().String()` while Postgres (line 23) uses `uuid.NewString()`. Both produce RFC 4122 UUIDs, so collision risk is negligible. **Impact:** None. **Fix:** None needed.

- **Cascading deletes and active conversations** — The prompt asks: *"Can `bridge_external_sessions` FK cascade delete sessions that are still referenced by active conversations?"* With `ON DELETE CASCADE` from `agent_tenants` → `bridge_external_sessions` → `bridge_handoffs` → `bridge_handoff_replies` → `bridge_reply_outbox`, deleting a tenant wipes all bridge data. This is the intended behavior (the 3 failing tests verify it), but there is no audit log or soft-delete tombstone. **Impact:** Data loss is irreversible. **Fix:** Consider adding a `deleted_at` tombstone or audit log if compliance requires retention.

---

## Summary

| Category | Critical | Warnings | Suggestions |
|----------|----------|----------|-------------|
| Correctness | 3 | 4 | 1 |
| Security | 1 | 1 | — |
| Migration Safety | 2 | 1 | — |
| Test Coverage | 1 | 3 | 1 |
| Maintainability | — | 3 | 1 |
| Edge Cases | — | 1 | — |

### Top 3 Must-Fix Before Merge

1. **`bridge_endpoints_test.go:744`** — Add SQLite skip guard to `TestBridgeReplySuccessPersisted`.
2. **`agent_audiences` and `tenant_config` NOT NULL constraints in SQLite** — Either apply the missing constraints or explicitly remove them from the plan and document the intentional divergence.
3. **`vector_db_s3_override` plaintext storage** — Encrypt the column or document the security exception.
