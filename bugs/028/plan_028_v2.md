# Plan: SQLite ↔ Postgres Full Parity + Test Fixes (v2)

**Goal:** Achieve 100% schema and store-layer parity between SQLite (local dev) and Postgres (production Neon DB). Fix all 4 failing tests from bugs/028.

**Context:** Fresh Neon Postgres database. Only `LATEST.sql` matters for schema creation (`store/migrator.go:133-173` applies it when no migration history exists). The 67 missing incremental migration files are irrelevant for fresh DB setup.

**Scope:** These changes apply to **fresh databases only**. Existing dev databases must be recreated or use a separate migration script.

---

## Phase 1: Fix Postgres LATEST.sql

**File:** `store/migration/postgres/LATEST.sql`

### 1a. Add 5 missing columns

These columns exist in SQLite LATEST.sql but are missing from Postgres LATEST.sql.

| Table | Column | Type | Default |
|-------|--------|------|---------|
| `user` | `allowed_tenant_ids` | `TEXT` | `DEFAULT NULL` |
| `memo_relation` | `tenant_id` | `INTEGER` | `DEFAULT NULL` |
| `user_tenant_permission` | `source_template_id` | `INTEGER` | `REFERENCES tenant_role_templates(id) ON DELETE SET NULL` |
| `tenant_config` | `admin_mutation_rate_limit_rpm` | `INTEGER NOT NULL` | `DEFAULT 30` |
| `tenant_config` | `vector_db_s3_override` | `TEXT` | `DEFAULT ''` |

### 1b. Add 6 missing indexes

Two indexes from SQLite are redundant with existing Postgres UNIQUE constraints and are omitted:
- `idx_user_username` — redundant with `username TEXT NOT NULL UNIQUE`
- `idx_tenant_config_tenant` — redundant with `tenant_id INTEGER NOT NULL UNIQUE`

Adding them would be harmless but misleading in a "missing" list. The remaining 6 are added for explicit parity:

```sql
CREATE INDEX IF NOT EXISTS idx_memo_creator_id ON memo (creator_id);
CREATE INDEX IF NOT EXISTS idx_memo_relation_tenant ON memo_relation (tenant_id);
CREATE INDEX IF NOT EXISTS idx_resource_creator_id ON resource (creator_id);
CREATE INDEX IF NOT EXISTS idx_resource_memo_id ON resource (memo_id);
CREATE INDEX IF NOT EXISTS idx_webhook_creator_id ON webhook (creator_id);
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_template ON user_tenant_permission (source_template_id);
```

### 1c. Fix INSERT OR IGNORE syntax bug (line 672)

```sql
-- Before (invalid Postgres):
INSERT OR IGNORE INTO tenant_role_templates (tenant_id, name, code, permissions)
VALUES (...);

-- After:
INSERT INTO tenant_role_templates (tenant_id, name, code, permissions)
VALUES (...)
ON CONFLICT (tenant_id, code) DO NOTHING;
```

### 1d. Add 7 missing CHECK constraints (inline in CREATE TABLE)

Since this is a fresh DB, these go inline in the CREATE TABLE statements — not as ALTER TABLE.

```sql
-- "user" table: add to CREATE TABLE
row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
role TEXT NOT NULL CHECK (role IN ('HOST', 'ADMIN', 'USER')) DEFAULT 'USER',

-- memo table: add to CREATE TABLE
row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
visibility TEXT NOT NULL CHECK (visibility IN ('PUBLIC', 'PROTECTED', 'PRIVATE')) DEFAULT 'PRIVATE',

-- memo_organizer table: add to CREATE TABLE
pinned INTEGER NOT NULL CHECK (pinned IN (0, 1)) DEFAULT 0,

-- activity table: add to CREATE TABLE
level TEXT NOT NULL CHECK (level IN ('INFO', 'WARN', 'ERROR')) DEFAULT 'INFO',

-- webhook table: add to CREATE TABLE
row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
```

### 1e. Add 5 missing default values

| Table | Column | Default |
|-------|--------|---------|
| `system_setting` | `description` | `DEFAULT ''` |
| `user` | `avatar_url` | `DEFAULT ''` |
| `memo` | `content` | `DEFAULT ''` |
| `resource` | `filename` | `DEFAULT ''` |
| `inbox` | `message` | `DEFAULT '{}'` |

### 1f. Add missing FK on agent_leads.transcript_id

```sql
FOREIGN KEY (transcript_id) REFERENCES agent_transcripts(id) ON DELETE SET NULL
```

### 1g. Add FK on bridge_handoffs.tenant_id

Postgres already has FK on 3 of 4 bridge tables (`bridge_external_sessions`, `bridge_auth_keys`, `bridge_auth_nonces`). Only `bridge_handoffs` is missing a direct FK on `tenant_id`. Add:

```sql
FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
```

---

## Phase 2: Fix SQLite LATEST.sql

**File:** `store/migration/sqlite/LATEST.sql`

### 2a. Add UNIQUE constraint on user_tenant_permission

```sql
CREATE TABLE user_tenant_permission (
    ...
    UNIQUE(user_id, tenant_id)
);
```

**Existing dev databases:** Before applying to an existing DB, deduplicate first:
```sql
DELETE FROM user_tenant_permission
WHERE rowid NOT IN (
    SELECT MIN(rowid) FROM user_tenant_permission GROUP BY user_id, tenant_id
);
```

### 2b. Add NOT NULL + UNIQUE on agent_tenants.guid

```sql
-- Before:
guid TEXT,
-- After:
guid TEXT NOT NULL UNIQUE,
```

### 2c. Add NOT NULL constraints to match Postgres

| Table | Columns | Change |
|-------|---------|--------|
| `agent_tenants` | `created_at`, `updated_at` | Add `NOT NULL` |
| `agent_audiences` | `guidelines`, `secondary_phones`, `emergency_urgency_threshold`, `escalation_confidence_threshold`, `rate_limit_rpm`, `require_contact_on_fallback`, `max_message_length`, `updated_at` | Add `NOT NULL` |
| `tenant_config` | `simulation_human_model`, `reasoning_model`, `retrieval_mode`, `content_tokens`, `record_transcripts` | Add `NOT NULL` |
| `agent_messages` | `created_at` | Add `NOT NULL` |
| `agent_leads` | `created_at`, `updated_at`, `last_message_at` | Add `NOT NULL` |

**Existing dev databases:** These changes are fresh-DB-only. ALTER TABLE on existing data with NULL values will fail. Existing dev databases must be recreated.

### 2d. Add FK on tenant_id for 13 tables

Add `FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE` to:

1. `user_tenant_permission` (on `tenant_id`)
2. `agent_audiences`
3. `agent_services`
4. `agent_exclusions`
5. `agent_coverage`
6. `agent_faqs`
7. `agent_safety_protocols`
8. `agent_kb_sections`
9. `agent_rules`
10. `agent_sessions`
11. `agent_source_files`
12. `agent_rate_limits`
13. `agent_observations`

### 2e. Add FK on all 4 bridge tables' tenant_id (fixes bugs/028)

Postgres already has FK on `bridge_external_sessions`, `bridge_auth_keys`, `bridge_auth_nonces`. SQLite is missing FK on all 4. Add `FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE` to:

1. `bridge_external_sessions`
2. `bridge_handoffs`
3. `bridge_auth_keys`
4. `bridge_auth_nonces`

**This fixes all 3 bridge-related test failures from bugs/028:**
- `TestBridgeExternalSessionUsesAgentTenantsTable` — FK enforces tenant existence on insert
- `TestBridgeAuthKeyTenantCascade` — FK + CASCADE enforces child row deletion on tenant delete
- `TestSQLiteFKCascadeOnTenantDeletion` — FK + CASCADE on bridge tables enforces cascade

---

## Phase 3: Fix Postgres Store Layer

**Only `vector_db_s3_override` is missing from the Postgres store layer.** The other columns (`source_template_id`, `admin_mutation_rate_limit_rpm`, `memo_relation.tenant_id`) are already referenced in the Go code — the queries exist but the schema columns don't. Adding the columns to LATEST.sql (Phase 1a) resolves the mismatch.

### 3a. store/db/postgres/rbac.go — 2 queries

Add `vector_db_s3_override` to:

| Function | Line | Query Type |
|----------|------|------------|
| `GetTenantConfig` | 140 | SELECT — add `vector_db_s3_override` to column list |
| `UpsertTenantConfig` | 174 | INSERT — add column |
| `UpsertTenantConfig` | 183 | ON CONFLICT UPDATE — add `vector_db_s3_override=EXCLUDED.vector_db_s3_override` |

---

## Phase 4: Fix Handler/Service Layer

### 4a. store/migration_helper.go — Add panic guard

The 3 PRAGMA-based functions (`AddColumnIfNotExists`, `GetTableColumns`, `ValidateTableSchema`) are SQLite-only. Add a runtime panic guard as defense-in-depth:

```go
// WARNING: This file contains SQLite-only helper functions.
// They use PRAGMA table_info() which is not portable to Postgres.
// These are only called from SQLite migration codepaths (migrator.go:44-53 gates on driver).
// For Postgres, schema is created from LATEST.sql — do not call these functions on Postgres.

func AddColumnIfNotExists(ctx context.Context, db *sql.DB, tableName, columnName, columnDef string) error {
    // Defense-in-depth: this function is SQLite-only.
    // migrator.go gates on driver, but if somehow called on Postgres, fail loudly.
    if err := db.PingContext(ctx); err != nil {
        // PRAGMA will fail on non-Sqlite; let it panic with a clear message
    }
    query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
    ...
}
```

**Recommended approach:** Add a driver parameter or wrap in a function that panics with a clear message if called on a non-SQLite driver. The simplest defense:

```go
func addColumnIfNotExistsSQLite(ctx context.Context, db *sql.DB, tableName, columnName, columnDef string) error {
    // SQLite-only: uses PRAGMA table_info(). Do not call on Postgres.
    query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
    ...
}
```

And update callers in `migrator.go` to verify driver before calling.

### 4b. memo_resource_service.go — No change needed

The `replacePlaceholders` function (line 319-334) already converts `?` to `$1/$2/$3` for Postgres. The driver detection at line 52 defaults to `"sqlite"` but is overridden by `s.Profile.Driver`. As long as `DRIVER=postgres` env var is set in production, this works correctly.

### 4c. delivery.go — No change needed

`SupportsBridgeDelivery()` returns `false` for SQLite, `true` for Postgres. This is intentional: bridge features require a real database for concurrent access. The handler at `delivery.go:20-23` correctly gates on this.

---

## Phase 5: Fix Tests (bugs/028 + parity)

### 5a. Fix stale schema version assertion

**File:** `store/test/migrator_test.go:18`

```go
// Before:
require.Contains(t, currentSchemaVersion, "0.29.", "schema version should be 0.29.x")
// After:
require.Contains(t, currentSchemaVersion, "0.30.", "schema version should be 0.30.x")
```

### 5b. Add SQLite skip guards to SQLite-specific tests

For tests that use `sqlite_master`, `PRAGMA`, or SQLite-specific trigger syntax, add a driver check at the top:

```go
func TestXxx(t *testing.T) {
    if os.Getenv("DRIVER") != "" && os.Getenv("DRIVER") != "sqlite" {
        t.Skip("SQLite-specific test")
    }
    ...
}
```

**Files to update:**

| File | Lines | SQLite-specific pattern |
|------|-------|------------------------|
| `store/test/ticket_test.go` | 85, 93 | `PRAGMA foreign_keys`, `sqlite_master` |
| `store/test/bridge_auth_test.go` | 26, 352 | `sqlite_master`, `PRAGMA foreign_keys` |
| `store/test/bridge_test.go` | 28, 32, 974, 977, 989 | `PRAGMA foreign_keys`, `sqlite_master` |
| `server/.../bridge_endpoints_test.go` | 741, 1146 | `sqlite_master`, hardcoded path |
| `server/.../bridge_delivery_test.go` | 300 | `PRAGMA foreign_keys` |
| `server/.../bridge_middleware_test.go` | 552-557 | SQLite trigger syntax |

### 5c. Make schema_validation_test.go driver-aware

**File:** `store/test/schema_validation_test.go`

Replace `PRAGMA table_info` with a driver-aware helper:

```go
func getTableColumns(t *testing.T, db *sql.DB, driver, tableName string) []string {
    t.Helper()
    var rows *sql.Rows
    var err error
    if driver == "postgres" {
        rows, err = db.QueryContext(t.Context(),
            "SELECT column_name FROM information_schema.columns WHERE table_name = $1", tableName)
    } else {
        rows, err = db.QueryContext(t.Context(),
            fmt.Sprintf("PRAGMA table_info(%s)", tableName))
    }
    require.NoError(t, err)
    defer rows.Close()
    var columns []string
    for rows.Next() {
        if driver == "postgres" {
            var col string
            require.NoError(t, rows.Scan(&col))
            columns = append(columns, col)
        } else {
            var cid int
            var name, ctype string
            var notnull int
            var dflt *string
            var pk int
            require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
            columns = append(columns, name)
        }
    }
    return columns
}
```

### 5d. Add Postgres bridge cascade test (coverage gap fix)

To avoid a coverage hole from skip guards, add a Postgres-compatible bridge cascade test:

```go
func TestPostgresBridgeFKCascade(t *testing.T) {
    if os.Getenv("DRIVER") != "postgres" {
        t.Skip("Postgres-specific test")
    }
    // Create tenant, bridge_external_sessions row, delete tenant, verify cascade
    // Uses standard SQL (no sqlite_master or PRAGMA)
    ...
}
```

This verifies the same FK behavior as the SQLite tests but using Postgres-compatible SQL.

---

## Phase 6: Verification

### 6a. Compilation

```bash
task build:backend
```

### 6b. SQLite tests

```bash
go test ./store/test/... -count=1 -v
go test ./server/router/api/v1/agent/... -count=1 -v
```

### 6c. Postgres tests (requires running Neon DB)

```bash
DRIVER=postgres DATABASE_URL="postgres://..." go test ./store/test/... -count=1 -v
```

### 6d. Manual verification

1. Start fresh Neon Postgres DB
2. Run `task run:rag` with `DRIVER=postgres DATABASE_URL=...`
3. Confirm migration applies: server logs `start migration` ... `end migrate`
4. Test agent CRUD: create tenant, upload KB, chat
5. Verify `INSERT OR IGNORE` fix doesn't break seed data

---

## File Count Summary

| Category | Files Modified | New Files |
|----------|---------------|-----------|
| LATEST.sql (Postgres) | 1 | 0 |
| LATEST.sql (SQLite) | 1 | 0 |
| Postgres store layer | 1 | 0 |
| Handler/service layer | 1 | 0 |
| Test files | 8 | 1 (bridge cascade test) |
| **Total** | **~12** | **1** |

---

## Bugs/028 Test Fixes Summary

| Test | Root Cause | Fix |
|------|-----------|-----|
| `TestGetCurrentSchemaVersion` | Stale assertion: expects `0.29.x`, actual is `0.30.x` | Update assertion to `"0.30."` (Phase 5a) |
| `TestBridgeExternalSessionUsesAgentTenantsTable` | `bridge_external_sessions.tenant_id` has no FK | Add FK to SQLite LATEST.sql (Phase 2e) |
| `TestBridgeAuthKeyTenantCascade` | `bridge_auth_keys.tenant_id` has no FK | Add FK to SQLite LATEST.sql (Phase 2e) |
| `TestSQLiteFKCascadeOnTenantDeletion` | `bridge_external_sessions`/`bridge_handoffs.tenant_id` have no FK | Add FK to SQLite LATEST.sql (Phase 2e) |

---

## Risks / Notes

- **Fresh-DB-only scope:** Phases 1 and 2 changes to LATEST.sql only affect new databases. Existing dev databases must be recreated. A separate migration script would be needed for existing databases.
- **UNIQUE constraint on user_tenant_permission:** Adding `UNIQUE(user_id, tenant_id)` to SQLite may reject existing data. Run deduplication SQL before applying on existing DBs.
- **Bridge table FKs on SQLite:** Adding `ON DELETE CASCADE` changes behavior — tenant deletion will now cascade to bridge tables. This is the desired behavior (tested by the 3 failing tests).
- **`INSERT OR IGNORE` in Postgres LATEST.sql:** This is a runtime error that will block fresh Postgres DB creation. Must be fixed before any production deployment.
- **`memo_resource_service.go`:** No changes needed, but verify `DRIVER=postgres` env var is set in production deployment config (fly.toml, Dockerfile, etc.).
- **Redundant indexes:** `idx_user_username` and `idx_tenant_config_tenant` are omitted from Phase 1b as they duplicate existing UNIQUE constraints.
