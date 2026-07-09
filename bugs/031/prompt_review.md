# Adversarial Code Review: Postgres LATEST.sql FK Ordering Fix

## Mission

You are conducting an **adversarial code review** of a critical fix to `store/migration/postgres/LATEST.sql`. The goal is to catch **every possible failure mode** that could cause `fly deploy` to fail on Fly.io. A failed deploy wastes 5-10 minutes per attempt. We have **zero tolerance for deploy failures**.

**You must assume the worst.** Your job is to find bugs, not to validate that the fix works.

---

## Background

### The Problem
The Neon Postgres database on Fly.io was failing to start because `LATEST.sql` had foreign key (FK) ordering bugs. The file runs inside a **transaction** via `store/migrator.go:152-162`:

```go
tx, err := s.driver.GetDB().Begin()
// ...
if err := s.execute(ctx, tx, string(bytes)); err != nil { ... }
if err := tx.Commit(); err != nil { ... }
```

The `execute()` function at line 259 runs the **entire SQL file as a single statement** via `tx.ExecContext()`. Unlike `psql` batch mode (which defers FK validation to end-of-transaction), this mode enforces FK constraints **immediately at each statement**. So if table A references table B, table B must already exist when table A's CREATE TABLE runs.

The local validation script (`validate-pg-migrations.sh`) passes because it uses `psql` which handles FK constraints differently. This masked the bug.

### The Fix
The fix reorganized table creation order in `LATEST.sql` so that every FK reference points to a table created earlier in the file. Additionally, two tables that were completely missing (`tenant_role_templates` and `agent_observations`) were restored.

### What Changed
- **File**: `store/migration/postgres/LATEST.sql` (960 lines, 53 tables)
- **4 tables moved** to earlier positions: `tenant_role_templates`, `agent_transcripts`, `agent_simulations`, `tickets`
- **2 tables restored** from missing state: `tenant_role_templates`, `agent_observations`
- **1 index moved**: `idx_memo_relation_tenant` (was before `memo_relation` table, now after it)
- **1 INSERT added**: Seed data for `tenant_role_templates` (5 role templates)

### Deploy Context
- Target: Fly.io with Neon Postgres
- App name: `bchat-pg`
- Config: `fly_pg.toml` (uses `Dockerfile.pg.fly`)
- The app starts, connects to Neon, runs `preMigrate()` which executes `LATEST.sql` in a transaction
- If any SQL statement fails, the entire transaction rolls back and the app crashes
- `fly deploy` builds a Docker image, deploys, and waits for health check (`/healthz`)
- A failed deploy shows up as: app starts but immediately crashes, or health check never passes

---

## Files to Review

### Primary Target
- **`store/migration/postgres/LATEST.sql`** — The fixed schema file (960 lines)

### Context Files (READ THESE)
- **`store/migrator.go`** — How `LATEST.sql` is executed (lines 133-179 for `preMigrate`, lines 258-273 for `execute`)
- **`scripts/validate-pg-migrations.sh`** — Local validation script (why it passed but deploy would fail)
- **`Dockerfile.pg.fly`** — Build and runtime configuration
- **`fly_pg.toml`** — Fly.io deploy configuration
- **`scripts/entrypoint.sh`** — Container entrypoint
- **`store/migration/sqlite/LATEST.sql`** — SQLite version (source of truth for table definitions, compare schemas)

### Reference
- **`bugs/030/code_review_deepseek.md`** — Previous adversarial review (for methodology reference)
- **`bugs/030/code2_review_qwen37plus.md`** — Previous adversarial review (for methodology reference)

---

## Review Checklist

### 1. FK Ordering (CRITICAL — the original bug)

For **every** `REFERENCES` clause in the file, verify the referenced table is created **earlier** in the same file. Do NOT trust the Python audit script — manually verify at least 10 random FK references.

Check these specifically (they were the bugs):
- `user_tenant_permission.source_template_id → tenant_role_templates.id`
- `agent_leads.transcript_id → agent_transcripts.id`
- `agent_script_analysis.simulation_id → agent_simulations.id`
- `agent_workflows.ticket_id → tickets.id`
- `agent_observations.session_id → agent_sessions.id`

Also check ALL other FK references. There are ~53 tables with many FK relationships.

### 2. Missing Tables

Verify that every table referenced by a FK in `LATEST.sql` actually exists as a CREATE TABLE in the same file. Cross-reference with the SQLite version (`store/migration/sqlite/LATEST.sql`) to find any tables that might be missing from the Postgres version.

### 3. Schema Drift (Postgres vs SQLite)

Compare the Postgres `LATEST.sql` with the SQLite `LATEST.sql` table-by-table:
- Are all tables present in both?
- Are column types correct for Postgres? (e.g., `INTEGER PRIMARY KEY AUTOINCREMENT` → `SERIAL PRIMARY KEY`, `TIMESTAMP` → `TIMESTAMPTZ`, etc.)
- Are there Postgres-specific syntax errors?
- Are `IF NOT EXISTS` clauses consistent? (Some tables have them, some don't)

### 4. INSERT Statement Correctness

The `tenant_role_templates` INSERT uses `NULL` for `tenant_id` (global templates). Verify:
- The `tenant_id` column allows NULL (check the CREATE TABLE)
- The `ON CONFLICT` clause was removed (it was `ON CONFLICT DO NOTHING` in old version, new version has no ON CONFLICT — is this safe for re-runs?)
- The JSON permissions strings are valid JSON
- The INSERT runs after the CREATE TABLE and indexes

### 5. Index Correctness

Check that:
- Every `CREATE INDEX` references a table that exists at that point in the file
- Index column names match the table's actual column names
- No duplicate index names
- Partial indexes (WITH conditions) use valid SQL syntax

### 6. Transaction Behavior

The `execute()` function runs the entire file as one `tx.ExecContext()` call. This means:
- If ANY statement fails, EVERYTHING rolls back
- FK constraints are enforced at statement level (not deferred)
- `IF NOT EXISTS` is NOT used on most tables — re-running on an existing DB would fail
- The `INSERT INTO tenant_role_templates` would fail on re-run due to UNIQUE constraint

**Question**: Is there any scenario where `LATEST.sql` runs on a non-empty database? The code at `migrator.go:137` says it only runs if `len(migrationHistoryList) == 0`, so it should only run on fresh databases. But what if migration_history is empty but tables exist?

### 7. Edge Cases and Gotchas

- The `agent_transcripts` block has a **duplicate comment**: `-- agent_transcripts` appears twice (lines 257-258 in the diff). Is this a problem?
- The `tickets` table has a self-referential FK: `parent_id INTEGER REFERENCES tickets(id)`. This is valid in PostgreSQL but verify it doesn't cause issues in the transaction context.
- The `agent_leads` table has an inline `CHECK` constraint: `CHECK (email IS NOT NULL OR phone IS NOT NULL)`. Verify this is valid Postgres syntax.
- Partial indexes with `WHERE` clauses: `idx_tickets_beads_id` and `idx_tickets_creator_description_memo`. Verify the WHERE expressions are valid.
- The `bridge_reply_outbox` table definition is at line ~828. Does it have any FK references that might be broken?

### 8. Migration Compatibility

The `preMigrate()` function only runs `LATEST.sql` when `migrationHistoryList` is empty (fresh database). After that, incremental migrations in `store/migration/postgres/<version>/` directories apply.

**Critical question**: If someone already has a database with tables from a previous version of `LATEST.sql`, and they upgrade to this version, will the incremental migrations handle the new tables (`tenant_role_templates`, `agent_observations`) and the moved tables? Or will there be conflicts?

Check: `store/migration/postgres/` for versioned migration directories that might CREATE these same tables.

### 9. Deploy-Specific Failure Modes

- **Docker build**: Does `COPY lib/linux_amd64/liblancedb_go.so` work? (was changed from `COPY lib/ lib/`)
- **Runtime**: Does the app connect to Neon successfully? (check `MEMOS_DSN` handling in entrypoint)
- **Health check**: Will `/healthz` pass after migration? (migration must complete before health check succeeds)
- **Memory**: 1024 MB VM — is this enough for 53 tables + RAG pipeline?

---

## Output Format

Structure your review as:

### CRITICAL Issues (will cause deploy failure)
Issues that WILL cause `fly deploy` to fail. Must be fixed before deploy.

### HIGH Issues (likely cause problems)
Issues that will probably cause runtime errors or data corruption.

### MEDIUM Issues (should fix)
Issues that are technically wrong but might not cause immediate failure.

### LOW Issues (nice to fix)
Cosmetic or minor issues.

### VERDICT
- **SHIP IT**: No critical or high issues. Safe to deploy.
- **FIX FIRST**: Has critical issues that must be fixed.
- **RISKY**: Has high issues that might cause problems.

---

## How to Conduct the Review

1. **Read ALL context files** listed above before starting
2. **Manually trace FK references** — do not trust automated tools
3. **Compare Postgres vs SQLite** schemas table by table
4. **Check every CREATE INDEX** matches actual column names
5. **Look for subtle syntax errors** — missing commas, wrong quotes, unmatched parentheses
6. **Think about re-entrancy** — what happens if LATEST.sql runs twice?
7. **Think about incremental migrations** — will they conflict with the new LATEST.sql?
8. **Assume the deploy WILL fail** and try to find why

Be thorough. Be adversarial. Be paranoid. The goal is zero deploy failures.
