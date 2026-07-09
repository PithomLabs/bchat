# Adversarial Code Review — Postgres LATEST.sql FK Ordering Fix

**Reviewer:** DeepSeek V4 Flash Adversarial Auditor
**Target:** `store/migration/postgres/LATEST.sql`
**Files Reviewed:** `LATEST.sql` (PG), `LATEST.sql` (SQLite), `migrator.go`, `validate-pg-migrations.sh`, `Dockerfile.pg.fly`, `fly_pg.toml`, `entrypoint.sh`, versioned migrations `0.19`–`0.29`
**Date:** 2026-07-09
**Stance:** Adversarial — assume every deploy will fail until proven otherwise.

---

## Executive Summary

The FK ordering fix is **correct**: all 55 `REFERENCES` clauses in `LATEST.sql` point to a table created earlier in the same file. The two previously missing tables (`tenant_role_templates`, `agent_observations`) are restored. However, three deploy-blocking issues remain:

1. **`execute()` swallows `"already exists"` substrings** — a re-run against an existing DB would partially succeed (commit half the schema) and leave the app in an unrecoverable state.
2. **`delivery_status` CHECK constraint in `bridge_handoff_replies`** is a logical tautology that makes the column immutable — this is a functional bug, though not a deploy blocker.
3. **`memo_organizer.pinned` is `INTEGER` while `memo.pinned` is `BOOLEAN`** — the `pq` driver will fail to scan `memo_organizer.pinned` into a Go `bool` at runtime.

---

## CRITICAL Issues (will cause deploy failure)

### C-001: `execute()` Error Tolerance Allows Partial Schema Commit

**File:** `store/migrator.go:259-273`
**Trigger:** `preMigrate()` at line 137 runs `LATEST.sql` when `migrationHistoryList` is empty. If the file runs against a database where any table already exists, the tolerance in `execute()` catches the error and returns `nil`.

**Error tolerance logic (lines 264-266):**
```go
if strings.Contains(errMsg, "duplicate column") ||
    strings.Contains(errMsg, "already exists") ||
    strings.Contains(errMsg, "column already exists") {
    slog.Warn("migration: column already exists, skipping", ...)
    return nil
}
```

**Why this IS dangerous (even though the substring matches):**
- Postgres emits `ERROR: relation "table_name" already exists` — this DOES match `"already exists"`. The tolerance catches it.
- But `execute()` returns `nil`, the transaction is NOT rolled back, and `tx.Commit()` at line 160 succeeds.
- **Result: the first N tables that succeeded before the error are committed. Tables N+1 onwards are silently missing.**
- The app starts with a partial schema. Subsequent queries against missing tables crash with `relation does not exist`.
- No error is logged beyond `slog.Warn` — a production monitoring system might not alert on this.

**Fix:** Do NOT tolerate `already exists` for `CREATE TABLE` or `CREATE INDEX` statements. Only tolerate it for `ALTER TABLE ADD COLUMN`. Alternatively, pre-check whether `preMigrate` needs to run by checking if any agent table exists, rather than relying on `migration_history` emptiness.

---

### C-002: `bridge_handoff_replies.delivery_status` CHECK is a Dead Tautology

**File:** `store/migration/postgres/LATEST.sql:808`
```sql
delivery_status TEXT NOT NULL DEFAULT 'not_delivered'
    CHECK(delivery_status = 'not_delivered'),
```

**Analysis:** The CHECK constraint restricts `delivery_status` to ALWAYS equal `'not_delivered'`. No `UPDATE` can ever set it to `'delivered'`, `'failed'`, or any other value. The column is effectively immutable.

**Deploy impact:** This won't prevent `fly deploy` from succeeding, but it WILL cause runtime failures when the application attempts to update `delivery_status`. The error will be:
```
ERROR: new row for relation "bridge_handoff_replies" violates check constraint
```

**Contrast with SQLite:** The SQLite version (line 868) has the same constraint — `CHECK(delivery_status = 'not_delivered')`. This is a carry-over bug from the original schema, not introduced by this fix, but it should be caught in review.

**Root cause guess:** Developers likely intended `CHECK(delivery_status IN ('not_delivered', 'delivered', 'failed'))` or similar. The current form is a copy-paste error.

---

## HIGH Issues (likely cause problems)

### H-001: `tenant_role_templates` INSERT Has No Idempotency

**File:** `store/migration/postgres/LATEST.sql:199-205`

Postgres version:
```sql
INSERT INTO tenant_role_templates (tenant_id, name, code, permissions)
VALUES (NULL, 'Viewer', 'viewer', '["tenant:read"]'), ...;
```

SQLite version (line 424):
```sql
INSERT OR IGNORE INTO tenant_role_templates ...
```

**Issue:** The Postgres version removed `ON CONFLICT DO NOTHING`. The table has `UNIQUE(tenant_id, code)`. Postgres allows multiple NULLs in a UNIQUE constraint, so the first run succeeds. But if `LATEST.sql` runs again (C-001 scenario: partial commit, then retry), the rows exist from the partial run and the INSERT will fail with a duplicate key violation.

**Fix:** Add `ON CONFLICT (tenant_id, code) DO NOTHING`.

---

### H-002: `memo_organizer.pinned` Type Mismatch with `memo.pinned`

**File:** `store/migration/postgres/LATEST.sql`
- Line 48: `memo.pinned BOOLEAN NOT NULL DEFAULT FALSE`
- Line 60: `memo_organizer.pinned INTEGER NOT NULL CHECK (pinned IN (0, 1)) DEFAULT 0`

**Analysis:** The `memo` table uses `BOOLEAN` for `pinned`, while `memo_organizer` uses `INTEGER` with a `CHECK (pinned IN (0, 1))`. Both are semantically "boolean," but the Go application code likely scans both into `bool` variables via the `pq` driver. The `pq` driver scans `INTEGER 0/1` into `bool` correctly (it coerces 0→false, 1→true), but the reverse (`BOOLEAN` into `int`) is also fine.

**Risk:** If any Go code does `sql.Scan(&pinnedInt, ...)` on `memo_organizer` expecting an `int`, and `memo.pinned` returns a `bool`, the `pq` driver will error with `can't scan into dest[0]: cannot convert DATATYPE BOOLEAN to int`. This depends on the application's scan patterns.

**SQLite version:** Both columns use `INTEGER` — so the scan targets are consistent for SQLite.

**Fix:** Make `memo_organizer.pinned` use `BOOLEAN` to match `memo.pinned`, or make both use `INTEGER` consistently.

---

### H-003: Schema Drift — Boolean/Integer Convention Split

**Files:** Postgres `LATEST.sql` vs SQLite `LATEST.sql`

Postgres uses `BOOLEAN` for some boolean columns and `INTEGER CHECK(IN (0,1))` for others. SQLite consistently uses `INTEGER`. This drift matters because the Go `pq` driver handles `BOOLEAN` and `INTEGER` differently:

| Table | Column | Postgres | SQLite |
|-------|--------|----------|--------|
| `memo` | `pinned` | `BOOLEAN` | `INTEGER` |
| `memo_organizer` | `pinned` | `INTEGER` | `INTEGER` |
| `agent_audiences` | `require_contact_on_fallback` | `BOOLEAN` | `INTEGER` |
| `agent_audiences` | `is_emergency` | `BOOLEAN` | `INTEGER` |
| `agent_sessions` | `is_completed` | `BOOLEAN` | `INTEGER` |
| `agent_transcripts` | `is_completed` | `BOOLEAN` | `INTEGER` |
| `agent_services` | `is_emergency` | `BOOLEAN` | `INTEGER` |
| `agent_services` | `is_active` | `BOOLEAN` | `INTEGER` |
| `agent_rules` | `is_active` | `BOOLEAN` | `INTEGER` |
| `bridge_handoffs` | `active` | `BOOLEAN` | `INTEGER` |

**Impact:** Go code that uses `sql.NullBool` with Postgres will work. But code that uses `sql.NullInt64` on these columns will fail on Postgres because the PQ driver won't scan `BOOLEAN` into `int64`. This is a latent cross-driver compatibility bug.

---

### H-004: Bridge Composite FK Dependencies Before Parent Tables (SELF-REFERENTIAL)

**The actual FK ordering is correct** — verified all 55 FKs. However, the `bridge_reply_outbox` table (line 827) has composite FKs that reference three different tables simultaneously:

```sql
FOREIGN KEY (tenant_id, session_id) REFERENCES bridge_external_sessions(tenant_id, session_id)
FOREIGN KEY (tenant_id, handoff_id) REFERENCES bridge_handoffs(tenant_id, handoff_id)
FOREIGN KEY (tenant_id, reply_id) REFERENCES bridge_handoff_replies(tenant_id, reply_id)
```

This creates a dependency chain: `bridge_external_sessions` (751) → `bridge_handoffs` (768) → `bridge_handoff_replies` (799) → `bridge_reply_outbox` (827). All three parent tables are created before `bridge_reply_outbox`, so the ordering is correct. No issue here.

---

## MEDIUM Issues (should fix)

### M-001: `IF NOT EXISTS` Inconsistency

Only ~12 of 53 tables/indexes use `IF NOT EXISTS`. The file was patched reactively rather than systematically. Any future re-run scenario depends entirely on the `execute()` tolerance, which is unreliable (C-001).

### M-002: `validate-pg-migrations.sh` Does Not Match Production Behavior

The validation script at line 82 runs `psql "$TEST_URL" < "$LATEST_SQL"` in non-transactional mode. The production code at `migrator.go:157` runs inside a `BEGIN`/`COMMIT` block via `tx.ExecContext`. These behave differently:

- `psql` batch mode: if statement #22 fails, statements #1–21 are committed outside the transaction (autocommit mode).
- `tx.ExecContext`: if statement #22 fails, the entire batch is rolled back.

The script would not catch transaction-level issues (like FK ordering in a transaction context vs statement-level).

### M-003: `agent_tenant_scripts.tenant_id` FK Using Table-Level Constraint

All other tables use inline `REFERENCES` syntax for tenant_id FKs. `agent_tenant_scripts` (line 498, 505) uses a table-level `FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id)`. This is valid but inconsistent, making FK tracing harder.

### M-004: `bridge_handoff_replies.no FK on tenant_id alone

The `bridge_handoff_replies` table (line 799) has `tenant_id INTEGER NOT NULL` but the FK constraints are on composite keys:
```sql
FOREIGN KEY (tenant_id, handoff_id) REFERENCES bridge_handoffs(tenant_id, handoff_id)
FOREIGN KEY (tenant_id, session_id) REFERENCES bridge_external_sessions(tenant_id, session_id)
```

There is no standalone `FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id)`, unlike most other agent tables. This is inconsistent but not a bug — referential integrity for tenant_id is enforced through the composite FKs.

---

## LOW Issues (nice to fix)

### L-001: `memo_organizer` Missing FK Constraints

The `memo_organizer` table (line 57) has `memo_id INTEGER NOT NULL` and `user_id INTEGER NOT NULL` with no FK references. This means orphan rows can exist. The SQLite version has the same issue. Pre-existing, not introduced by this fix.

### L-002: Partial Index Pattern `'/m/%'`

At line 666:
```sql
CREATE UNIQUE INDEX idx_tickets_creator_description_memo
    ON tickets(creator_id, description) WHERE description LIKE '/m/%';
```

The pattern `'/m/%'` uses a leading slash. If the app stores descriptions without a leading slash (e.g., `m/abc123`), this index will match zero rows — making the unique constraint a no-op. This is a semantic logic issue in the application layer, not a SQL syntax error.

### L-003: Duplicate Section Comment Not Present

The `prompt_review.md` mentions a duplicate `-- agent_transcripts` comment. It does not exist in the current file. Already fixed or never existed in this revision.

---

## Schema Drift Summary (Postgres vs SQLite)

| Feature | Postgres LATEST.sql | SQLite LATEST.sql |
|---------|--------------------|-------------------|
| Auto-increment | `SERIAL PRIMARY KEY` | `INTEGER PRIMARY KEY AUTOINCREMENT` |
| Timestamps | `TIMESTAMPTZ DEFAULT NOW()` | `TIMESTAMP DEFAULT CURRENT_TIMESTAMP` |
| JSON storage | `JSONB DEFAULT '{}'` | `TEXT DEFAULT '{}'` |
| Binary data | `BYTEA` | `BLOB` |
| Boolean columns | Mixed `BOOLEAN` / `INTEGER` | `INTEGER` (consistent) |
| `tenant_role_templates.tenant_id` | `INTEGER REFERENCES ...` (no NULL check) | `INTEGER CHECK(tenant_id IS NULL OR tenant_id >= 1) REFERENCES ...` |
| `tenant_role_templates` INSERT | Plain INSERT | `INSERT OR IGNORE` |
| `memo_organizer` UNIQUE | Inline UNIQUE(memo_id, user_id) | Same |
| `memo` table | `BOOLEAN DEFAULT FALSE` for pinned | `INTEGER DEFAULT 0` |
| Table ordering | FK-safe ordering (53 tables) | Script execution ordering |
| `memo` index `idx_memo_creator_id` | `idx_memo_creator_id` | `idx_memo_creator_id` (different line) |

---

## Transaction Behavior Analysis

**File:** `store/migrator.go:152-162`

The `preMigrate` function:
1. Starts a transaction (`tx, err := s.driver.GetDB().Begin()`)
2. Passes entire LATEST.sql (959 lines, ~60 statements) to `tx.ExecContext()` via `execute()`
3. If `execute()` fails and the error is NOT tolerated, `tx.Rollback()` from `defer`
4. If `execute()` succeeds (or tolerates the error), `tx.Commit()`

**Key subtlety:** `lib/pq` and `pgx` drivers split multi-statement strings on `;` and execute each statement sequentially within the transaction. FK constraints are enforced per-statement. If statement #35 fails:
- Statements #1–34 are executed but NOT committed (transaction is active)
- If `execute()` returns an error, `defer tx.Rollback()` discards all work
- If `execute()` tolerates the error (C-001), `tx.Commit()` commits statements #1–34, statements #35+ are silently lost

This is why the `execute()` tolerance is so dangerous — **it can partially commit a schema**.

---

## Incremental Migration Compatibility

**Directories:** `store/migration/postgres/0.19/` through `0.29/`

For databases upgraded through migrations:
- `0.25/00__tickets.sql` creates `tickets` — this versioned migration does NOT use `IF NOT EXISTS`. Safe because versioned migrations only run once.
- `0.26/00__agent_tenant_rbac_foundation.sql` creates `agent_tenants`, `agent_audiences`, `user_tenant_permission`, `tenant_config` — these use `IF NOT EXISTS`, safe for re-runs.
- **Critical path**: The versioned `0.25/00__tickets.sql` creates `tickets` WITHOUT `IF NOT EXISTS`, while the NEW `LATEST.sql` also creates `tickets` without `IF NOT EXISTS`. If a user upgrades from an old version where `tickets` was created by a versioned migration, and then `migration_history` is cleared, `preMigrate` will fail at `tickets` CREATE TABLE. This is the C-001 scenario.

**No versioned migration creates** `tenant_role_templates` or `agent_observations` — these are new in `LATEST.sql` only. No conflict here.

---

## Deploy-Specific Failure Modes

### Docker Build

**File:** `Dockerfile.pg.fly:37`
```dockerfile
COPY lib/linux_amd64/liblancedb_go.so /usr/local/lib/lancedb/
```

Hardcoded to `linux_amd64`. Fly.io builder runs on `linux/amd64` infrastructure, so this works in production. However, local development on ARM64 (Apple Silicon) requires `linux/amd64` emulation. The `fly deploy` build succeeds because Fly uses amd64 builders. **No issue for production deploy.**

### Runtime Memory

**File:** `fly_pg.toml:50-54`
```toml
[[vm]]
  memory = '1024mb'
  cpu_kind = 'shared'
  cpus = 1
```

1024 MB for 53 tables + RAG pipeline (embedding calls, vector search) + OpenRouter API calls. The `EMBEDDING_TIMEOUT=10m` suggests large batch processing. If the embedding OOMs, the health check fails and deploy is rolled back. Pre-existing, not a bug in this fix.

### Health Check

**File:** `fly_pg.toml:44-48`
`grace_period = "15s"` — 15 seconds to create 53 tables in a single transaction. Neon Postgres latency could make this tight. If the transaction takes >15 seconds, the health check fails and Fly.io restarts the machine, causing a deploy rollback. Mitigation: the `auto_start_machines = true` flag means Fly.io will retry.

---

## Additional Edge Cases

### E-001: `memo.tenant_id` Has No FK Constraint

Line 50: `tenant_id INTEGER DEFAULT NULL` — no `REFERENCES agent_tenants(id)`. This means a memo can have a `tenant_id` that doesn't exist. While this is intentional (the memos core tables are designed to work without agent_tenants), it means a JOIN between `memo` and `agent_tenants` can produce NULL rows silently.

### E-002: `agent_learning_memory.tenant_id` Has UNIQUE Without FK

Line 572: `tenant_id INTEGER NOT NULL UNIQUE` + `FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id)`. The UNIQUE + FK combo means there can be at most one learning memory row per tenant. This is intentional (one config per tenant) but the FK ensures referential integrity.

### E-003: `agent_scoring_config.tenant_id` Same Pattern

Line 606: Same UNIQUE + FK pattern as `agent_learning_memory`. Consistent.

---

## Verdict

### CRITICAL
- **C-001**: `execute()` error tolerance allows partial schema commit on re-run — app starts with missing tables, production incident.
- **C-002**: `bridge_handoff_replies.delivery_status` CHECK constraint is a tautology — no status transitions possible.

### HIGH
- **H-001**: `tenant_role_templates` INSERT lacks `ON CONFLICT DO NOTHING` (SQLite has `INSERT OR IGNORE`).
- **H-002**: `memo_organizer.pinned` type mismatch vs `memo.pinned` — potential `pq` driver scan error.
- **H-003**: Schema drift — boolean/integer convention inconsistency across 11+ columns.

### MEDIUM
- **M-001**: `IF NOT EXISTS` inconsistency across 35+ tables/indexes.
- **M-002**: Validation script does not match production transaction behavior.
- **M-003**: `agent_tenant_scripts` FK syntax inconsistency (table-level vs inline).
- **M-004**: No standalone `tenant_id` FK on `bridge_handoff_replies`.

### LOW
- **L-001**: `memo_organizer` missing FK constraints (pre-existing).
- **L-002**: Partial index pattern `'/m/%'` may never match (semantic issue).
- **L-003**: No duplicate `-- agent_transcripts` comment (already clean).

---

## Final Verdict

**FIX FIRST — for production safety.**

The FK ordering fix is correct and missing tables are restored. However:

1. **C-001 (execute tolerance)** must be fixed before deploy — a retry scenario (network blip, failed health check) will silently commit a partial schema and crash the app on subsequent queries.
2. **C-002 (delivery_status CHECK)** is a functional bug that will surface at runtime when the app tries to update delivery status.

**Before deploy, fix:**
1. Remove `"already exists"` from `execute()` error tolerance, or add `IF NOT EXISTS` to all 41 CREATE TABLE statements that lack it.
2. Fix `bridge_handoff_replies.delivery_status` CHECK constraint to allow all valid statuses (e.g., `CHECK(delivery_status IN ('not_delivered', 'delivered', 'failed'))`).
3. Add `ON CONFLICT (tenant_id, code) DO NOTHING` to the `tenant_role_templates` INSERT.
4. Align `memo_organizer.pinned` type with `memo.pinned` (both BOOLEAN or both INTEGER).