# Adversarial Code Review — Postgres LATEST.sql FK Ordering Fix

**Reviewer:** Kilo/Stepfun Adversarial Auditor
**Target:** `store/migration/postgres/LATEST.sql`
**Date:** 2026-07-09
**Stance:** Skeptical, adversarial — finding failure modes, not validating the fix.

---

## Executive Summary

The FK ordering fix is directionally correct: every `REFERENCES` clause in the Postgres `LATEST.sql` now points to a table created earlier in the same file. The two previously missing tables (`tenant_role_templates`, `agent_observations`) are restored. However, the fix does **not** address a critical gap in `store/migrator.go:259-273`: the `execute()` function's error-tolerance logic is incomplete, meaning any re-run scenario (corrupted migration history, manual DB reset without dropping tables) will fail with a full transaction rollback. There are also schema drifts between the Postgres and SQLite versions that will cause runtime type mismatches.

---

## CRITICAL Issues (will cause deploy failure)

### C-001: `execute()` Does Not Tolerate "Relation Already Exists"

**File:** `store/migrator.go:259-273`
**Trigger:** `preMigrate()` at line 137 runs `LATEST.sql` when `migrationHistoryList` is empty. The guard is `err != nil || len(migrationHistoryList) == 0`. If the database exists with tables but `migration_history` is empty (corrupted DB, manual cleanup, test environment), all 41 `CREATE TABLE` statements without `IF NOT EXISTS` will fail with `ERROR: relation "X" already exists`.

**Actual code:**
```go
errMsg := err.Error()
if strings.Contains(errMsg, "duplicate column") ||
    strings.Contains(errMsg, "already exists") ||
    strings.Contains(errMsg, "column already exists") {
    slog.Warn("migration: column already exists, skipping", slog.String("error", errMsg))
    return nil
}
return errors.Wrap(err, "failed to execute statement")
```

**Analysis:** The tolerance covers `duplicate column` and `column already exists` but NOT `relation "X" already exists` (the Postgres error for `CREATE TABLE` on an existing table). Only ~12 of 53 tables/indexes use `IF NOT EXISTS`. The remaining 41+ will hard-fail.

**Deploy impact:** Any scenario where `preMigrate()` runs against a non-empty database will crash the app on startup. This is not just a re-run issue — it can happen on Fly.io if the volume retains data across deploys and `migration_history` is somehow cleared.

---

## HIGH Issues (likely cause problems)

### H-001: `tenant_role_templates` INSERT Fails on Re-Run

**File:** `store/migration/postgres/LATEST.sql:199-205`
**Current:**
```sql
INSERT INTO tenant_role_templates (tenant_id, name, code, permissions)
VALUES
    (NULL, 'Viewer', 'viewer', '["tenant:read"]'),
    ...
```

**Issue:** No `ON CONFLICT DO NOTHING`. The table has `UNIQUE(tenant_id, code)`. Postgres allows multiple NULLs in a UNIQUE constraint, so the first run succeeds. But if `LATEST.sql` runs again (e.g., after a failed deploy where the transaction rolled back but the table persisted), the INSERT will fail with a duplicate key violation because the rows were committed before the failure.

**Contrast with SQLite:** The SQLite version uses `INSERT OR IGNORE` (line 424 of `sqlite/LATEST.sql`), which is idempotent. The Postgres version lacks equivalent protection.

**Deploy impact:** Partial deploys where tables exist but seed data was not inserted will fail on retry.

---

### H-002: Schema Drift — Boolean vs Integer Types

**Files:** `store/migration/postgres/LATEST.sql` vs `store/migration/sqlite/LATEST.sql`

Multiple columns use different types between Postgres and SQLite:

| Table | Column | Postgres | SQLite |
|-------|--------|----------|--------|
| `agent_audiences` | `require_contact_on_fallback` | `BOOLEAN DEFAULT TRUE` | `INTEGER DEFAULT 1` |
| `agent_audiences` | `is_emergency` | `BOOLEAN DEFAULT FALSE` | `INTEGER DEFAULT 0` |
| `agent_sessions` | `is_completed` | `BOOLEAN DEFAULT FALSE` | `INTEGER DEFAULT 0` |
| `agent_rate_limits` | `request_count` | `INTEGER DEFAULT 0` | `INTEGER DEFAULT 0` (ok) |
| `bridge_handoffs` | `active` | `BOOLEAN DEFAULT TRUE` | `INTEGER DEFAULT 1` |
| `bridge_handoffs` | `version` | `INTEGER DEFAULT 1` (ok) | `INTEGER DEFAULT 1` |
| `memo` | `pinned` | `BOOLEAN DEFAULT FALSE` | `INTEGER DEFAULT 0` |
| `agent_services` | `is_emergency` | `BOOLEAN DEFAULT FALSE` | `INTEGER DEFAULT 0` |
| `agent_services` | `is_active` | `BOOLEAN DEFAULT TRUE` | `INTEGER DEFAULT 1` |

**Impact:** The Go application code likely uses `sql.NullBool` or scans into `bool` variables. SQLite's flexible typing will coerce `0`/`1` to `false`/`true`, but Postgres enforces strict types. If the application layer has dialect-specific scan logic, this could cause runtime panics or data corruption when switching drivers.

---

### H-003: Versioned Migration `0.25/00__tickets.sql` Lacks `IF NOT EXISTS`

**File:** `store/migration/postgres/0.25/00__tickets.sql`
```sql
CREATE TABLE tickets (
  id SERIAL PRIMARY KEY,
  ...
```

**Issue:** The versioned migration creates `tickets` without `IF NOT EXISTS`. For normal upgrade paths this is safe (versioned migrations only run if the table doesn't exist). But for the same re-run scenario as C-001, this migration would fail if `tickets` already exists but `migration_history` is empty.

**Note:** The `LATEST.sql` also creates `tickets` without `IF NOT EXISTS`, so this is consistent but still fragile.

---

### H-004: `IF NOT EXISTS` Inconsistency Across Tables/Indexes

**File:** `store/migration/postgres/LATEST.sql`
- **With `IF NOT EXISTS`:** `migration_history`, `memo_relation` index, `agent_tenants` index, `agent_messages` (table + 2 indexes), `agent_leads` (table + 2 indexes), `agent_tenant_scripts`, `agent_observations`, `agent_reindex_checkpoints`, `bridge_handoff_replies` (table + 2 indexes), `bridge_reply_outbox` (table + 1 index)
- **Without `IF NOT EXISTS`:** All other 35+ tables and indexes

**Impact:** The inconsistency suggests the file was patched reactively rather than systematically. Any future table addition without `IF NOT EXISTS` will re-introduce the re-run failure mode.

---

## MEDIUM Issues (should fix)

### M-001: `validate-pg-migrations.sh` Claimed Masking Is Likely Incorrect

**File:** `scripts/validate-pg-migrations.sh:82`
```bash
if ! psql "$TEST_URL" < "$LATEST_SQL" 2>"$TEMP_DIR/latest_errors.txt"; then
```

**Analysis:** `psql` in non-transactional mode sends each statement individually. Postgres enforces FK constraints at statement execution time unless explicitly deferred. The original FK ordering bug (table A referencing table B created later) should have been caught by this script. The prompt_review.md claim that "psql defers FK validation" is incorrect.

**Possible explanation for original bug escape:** The original `LATEST.sql` may have had a different failure mode (e.g., missing `tenant_role_templates` table entirely, causing `relation "tenant_role_templates" does not exist` rather than FK ordering). The fix addresses both issues, but the validation script's effectiveness claim is overstated.

**Recommendation:** Update the script to wrap `LATEST.sql` in a transaction (`BEGIN; ... COMMIT;`) to match the actual runtime behavior in `migrator.go:152-162`.

---

### M-002: Missing `tenant_id` FK on `agent_tenant_scripts`

**File:** `store/migration/postgres/LATEST.sql:495-506`
```sql
CREATE TABLE agent_tenant_scripts (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    ...
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);
```

**Issue:** The `tenant_id` column lacks `REFERENCES agent_tenants(id)` inline syntax. The FK is defined as a table constraint at the bottom, which is valid Postgres, but inconsistent with the rest of the file where inline `REFERENCES` is the norm. This is cosmetic but reduces readability during FK tracing.

---

### M-003: `agent_observations` Index Before Table in SQLite, After in Postgres

**File:** `store/migration/postgres/LATEST.sql:724-725`
```sql
CREATE INDEX idx_observations_tenant ON agent_observations(tenant_id);
CREATE INDEX idx_agent_observations_resource ON agent_observations(resource_id);
```

These indexes are created immediately after the `agent_observations` table definition (line 710), which is correct. However, in the SQLite version, these indexes are at the very end of the file (after all tables). This is a minor inconsistency but not a bug — both orders work since the table exists before the indexes.

---

## LOW Issues (nice to fix)

### L-001: Duplicate Section Comment Not Present

The prompt_review.md mentions a duplicate `-- agent_transcripts` comment (lines 257-258). After manual inspection of the current file, this duplicate does not exist. The comment appears only once at line 259. This was likely fixed in the current revision.

---

### L-002: `bridge_handoffs` `active` Column Type Drift

**File:** `store/migration/postgres/LATEST.sql:777`
```sql
active BOOLEAN NOT NULL DEFAULT TRUE CHECK(active IN (TRUE, FALSE)),
```

**SQLite equivalent:**
```sql
active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
```

The Postgres version uses `BOOLEAN` with `TRUE`/`FALSE` literals, while SQLite uses `INTEGER` with `0`/`1`. This is intentional dialect-specific typing but creates a maintenance burden. Any code that queries `active = 1` will work on both, but `active = TRUE` is Postgres-specific.

---

### L-003: Partial Index `WHERE` Clause Validation

**File:** `store/migration/postgres/LATEST.sql:663,666`
```sql
CREATE UNIQUE INDEX idx_tickets_beads_id ON tickets(beads_id) WHERE beads_id IS NOT NULL;
CREATE UNIQUE INDEX idx_tickets_creator_description_memo ON tickets(creator_id, description) WHERE description LIKE '/m/%';
```

These partial indexes use `LIKE` in a `WHERE` clause, which is valid Postgres. However, the pattern `'/m/%'` uses a leading slash, which is unusual for memo references (typically `/m/` is used). This is not a SQL error but may indicate a logic bug in the application layer that generates these descriptions.

---

## Schema Drift Summary (Postgres vs SQLite)

Beyond the boolean/integer drift noted in H-002:

| Feature | Postgres | SQLite |
|---------|----------|--------|
| ID type | `SERIAL PRIMARY KEY` | `INTEGER PRIMARY KEY AUTOINCREMENT` |
| Timestamps | `TIMESTAMPTZ DEFAULT NOW()` | `TIMESTAMP DEFAULT CURRENT_TIMESTAMP` |
| JSON columns | `JSONB DEFAULT '{}'` | `TEXT DEFAULT '{}'` |
| Binary data | `BYTEA` | `BLOB` |
| Boolean default | `BOOLEAN DEFAULT TRUE` | `INTEGER DEFAULT 1` |
| `tenant_role_templates` FK | `tenant_id INTEGER REFERENCES ...` (no NULL check) | `tenant_id INTEGER CHECK (tenant_id IS NULL OR tenant_id >= 1) REFERENCES ...` |

The SQLite version has an extra `CHECK (tenant_id IS NULL OR tenant_id >= 1)` on `tenant_role_templates.tenant_id` that the Postgres version lacks. This is a minor gap — Postgres allows NULL FKs by default, and the table uses NULL for global templates.

---

## Transaction Behavior Analysis

**File:** `store/migrator.go:152-162`

The `preMigrate()` function runs `LATEST.sql` inside a transaction. If any statement fails, the entire transaction rolls back. This means:

1. **Partial success is impossible** — either all 53 tables are created or none are.
2. **Seed data is atomic** — the `tenant_role_templates` INSERT runs in the same transaction as the CREATE TABLE.
3. **No deferred FK checking** — Postgres enforces FK constraints immediately within the transaction. The FK ordering fix is load-bearing.

**Edge case:** If `tx.Commit()` fails (e.g., connection lost after all statements succeed), the transaction rolls back and the app crashes. This is a transient network issue, not a code bug.

---

## Incremental Migration Compatibility

**Directory:** `store/migration/postgres/0.19/` through `0.29/`

For databases upgraded from previous versions:
- `0.25/00__tickets.sql` creates `tickets` — safe because versioned migrations only run once.
- No versioned migration creates `tenant_role_templates` or `agent_observations` — these are new in `LATEST.sql` only.
- `0.26/00__agent_tenant_rbac_foundation.sql` creates `agent_tenants`, `agent_audiences`, `user_tenant_permission`, `tenant_config` — safe because these tables already exist in upgraded DBs (created by earlier versioned migrations).

**Critical path:** If a user has a database at version 0.29 with all tables present, and then `migration_history` is cleared, `preMigrate()` will attempt to run `LATEST.sql` and fail on the first `CREATE TABLE` without `IF NOT EXISTS`. This is the same failure mode as C-001.

---

## Deploy-Specific Failure Modes

### Docker Build

**File:** `Dockerfile.pg.fly:37`
```dockerfile
COPY lib/linux_amd64/liblancedb_go.so /usr/local/lib/lancedb/
```

This path is hardcoded to `linux_amd64`. If the build host is ARM64 (Apple Silicon), this will fail unless the `.so` is cross-compiled or the build uses `linux/amd64` platform. The `fly deploy` build runs on Fly.io's infrastructure, which may use different architectures.

**Mitigation:** The Go build stage uses `golang:1.25` which is multi-arch. The `COPY lib/linux_amd64/...` will fail if the build context doesn't contain that path. This is a pre-existing issue, not introduced by this fix.

---

### Runtime Memory

**File:** `fly_pg.toml:50-54`
```toml
[[vm]]
  memory = '1024mb'
  cpu_kind = 'shared'
  cpus = 1
  memory_mb = 1024
```

1024 MB is tight for 53 tables + RAG pipeline + OpenRouter API calls. The `EMBEDDING_TIMEOUT=10m` suggests large batch processing. If the embedding process spikes memory, the VM may OOM. This is a pre-existing config, not a bug in the fix.

---

### Health Check Timing

**File:** `fly_pg.toml:44-48`
```toml
[[http_service.checks]]
  grace_period = "15s"
  interval = "5s"
  method = "GET"
  path = "/healthz"
```

The `grace_period = "15s"` gives the app 15 seconds to start and run migrations before health checks begin. For 53 tables in a transaction, this should be sufficient unless the Neon database connection is slow. No issue here.

---

## Verdict

### CRITICAL
- **C-001**: `execute()` error tolerance incomplete — re-run scenarios will fail.

### HIGH
- **H-001**: `tenant_role_templates` INSERT lacks `ON CONFLICT DO NOTHING`.
- **H-002**: Schema drift (boolean vs integer) between Postgres and SQLite.
- **H-003**: Versioned migration `0.25/00__tickets.sql` lacks `IF NOT EXISTS`.
- **H-004**: Inconsistent `IF NOT EXISTS` usage across tables/indexes.

### MEDIUM
- **M-001**: Validation script's transactional behavior claim is incorrect.
- **M-002**: `agent_tenant_scripts` FK syntax inconsistency.
- **M-003**: Minor index placement inconsistency between dialects.

### LOW
- **L-001**: Duplicate comment not present (already fixed).
- **L-002**: `bridge_handoffs.active` type drift (intentional).
- **L-003**: Partial index `LIKE` pattern may be semantically wrong.

---

## Final Verdict

**RISKY**

The FK ordering fix is correct and the missing tables are restored. However, C-001 (`execute()` tolerance gap) is a deploy-blocking issue for any re-run scenario, and H-001 (INSERT without `ON CONFLICT`) creates a fragile retry path. The schema drift in H-002 is a runtime risk if the application layer assumes consistent types across drivers.

**Before deploy, fix:**
1. Add `relation already exists` and `duplicate table` to `execute()` error tolerance (or use `CREATE TABLE IF NOT EXISTS` on all tables).
2. Add `ON CONFLICT (tenant_id, code) DO NOTHING` to the `tenant_role_templates` INSERT.
3. Audit boolean/integer column usage in Go scan code to ensure cross-driver compatibility.
