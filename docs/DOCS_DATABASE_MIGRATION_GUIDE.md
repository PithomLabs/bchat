# Database Migration Guide

**Status:** Current — supersedes `DOCS_DATABASE_MIGRATION.MD`
**Date:** 2026-07-23

---

## Quick Reference

| Task | Command |
|------|---------|
| Create new migration | `task migrate:new NAME=add_widget_config` |
| Check version info | `task version:info` |
| Validate parity | `task validate:parity` |
| Validate schema | `task validate:schema` |
| Validate LATEST.sql | `./scripts/validate-migrations.sh` |
| Run script tests | `task test:scripts` |
| Build binary | `task build:backend` |

---

## Architecture

### Directory Structure

```
store/migration/
├── sqlite/
│   ├── LATEST.sql              # Full schema for new databases
│   ├── 0.25/                   # Version directories (minor version)
│   │   ├── 00__create_agent.sql
│   │   ├── 01__add_token.sql
│   │   └── ...
│   ├── 0.26/
│   └── ...
├── postgres/
│   ├── LATEST.sql
│   ├── 0.19/
│   └── ...
```

### Version Detection

At runtime, `GetCurrentSchemaVersion()` in `store/migrator.go` scans all directories under `store/migration/sqlite/`, finds the highest minor version and patch number, and returns the full version string (e.g., `"0.33.1"`).

`Version` and `DevVersion` in `internal/version/version.go` are defaults. The runtime version overrides them. `scripts/bump-version.sh` shows what the runtime would compute (informational, read-only).

### Migration Flow

1. Server starts → `Migrate()` calls `GetCurrentSchemaVersion()` → scans all FS directories
2. For each migration file with version > current schema version: apply it
3. Record applied version in `migration_history` table
4. `LATEST.sql` is used for fresh database creation only

---

## Adding a New Migration

### Step 1: Create migration files

```bash
task migrate:new NAME=add_widget_config
```

This creates:
- `store/migration/sqlite/0.34/00__add_widget_config.sql` (SQLite template)
- `store/migration/postgres/0.34/00__add_widget_config.sql` (Postgres template)

### Step 2: Write SQL for each driver

Write the migration SQL in both files. Use `docs/TYPE_MAPPING.md` for SQLite→Postgres type mapping.

**SQLite rules:**
- Backtick quoting: `` `column_name` ``
- `INTEGER PRIMARY KEY AUTOINCREMENT`
- `INTEGER CHECK (x IN (0,1))` for booleans
- `BLOB` for binary data
- `INSERT OR IGNORE` for idempotent inserts

**Postgres rules:**
- Double-quote quoting: `"column_name"`
- `SERIAL PRIMARY KEY`
- `BOOLEAN` for booleans
- `BYTEA` for binary data
- `INSERT INTO ... ON CONFLICT DO NOTHING` for idempotent inserts
- Quote reserved words: `"user"`, `"order"`, `"group"`
- `TIMESTAMPTZ` instead of `TIMESTAMP`

### Step 3: Update LATEST.sql

Update both `store/migration/sqlite/LATEST.sql` and `store/migration/postgres/LATEST.sql` with the new table/column/index definitions.

### Step 4: Validate

```bash
task validate:parity    # Cross-driver schema + file-list parity
task validate:schema    # Schema validation tests
```

---

## Rules and Conventions

### Migration File Rules

1. **Naming:** `NN__snake_case_description.sql` (NN = two-digit patch number)
2. **One logical change per file** — makes rollback easier
3. **Idempotent where possible** — use `IF NOT EXISTS`, `IF EXISTS`
4. **No business logic in migrations** — migrations are DDL + seed data only
5. **Both drivers must have files** — CI enforces file-list parity

### LATEST.sql Rules

1. **Must be in sync with migration files** — `validate-migrations.sh` checks this
2. **Used for fresh database creation only** — incremental migrations use the numbered files
3. **Must match between drivers** — `validate:parity` checks this

### Version Naming

- `0.<minor>.<patch>` — semver format
- Minor version = directory name (e.g., `0.33/`)
- Patch version = file number within directory (e.g., `00` → `.0`, `01` → `.1`)
- Current development version = latest directory + latest patch + 1

---

## Cross-Driver Parity

### Three Levels of Parity

| Level | Definition | Enforced? |
|-------|-----------|----------|
| **Schema parity** | LATEST.sql produces the same logical schema | **Yes — CI gate** |
| **File-list parity** | Migration files exist in both drivers | **Yes — CI gate** |
| **Incremental path parity** | Identical SQL in each file | **No** — different drivers need different SQL |

### CI Gates Matrix

| Historical Bug | CI Gate That Would Catch It |
|----------------|---------------------------|
| 008 — Unique constraint failure | `validate-pg-migrations.sh` (runs all migrations against real PG) |
| 009 — Migration 28 hotfix | `validate-db-migrations.sh` (applies incrementally, compares schemas) |
| 045 — Version directory skip | `version:info` (shows computed version from FS) |
| 046 — LATEST.sql drift | `validate:parity` (compares sqlite vs postgres schema + file lists) |

---

## Gotchas and Known Issues

1. **Tests always use LATEST.sql** — `go test` creates fresh databases from LATEST.sql, not incremental migrations. Always test with an existing database too.

2. **DDL transactionality** — `ALTER TABLE` is transactional in SQLite but NOT in Postgres. A failed Postgres migration may leave the schema inconsistent.

3. **WAL visibility** — SQLite WAL mode may not immediately show changes to concurrent readers. Test with `PRAGMA journal_mode=WAL`.

4. **go:embed rebuild** — Migration files are embedded at compile time. After adding new files, rebuild the binary.

5. **Dual version tracking** — `Version` (released) and `DevVersion` (development) in version.go are both overridden at runtime by `GetCurrentSchemaVersion()`.

6. **LATEST.sql drift** — If migration files and LATEST.sql get out of sync, new databases will have a different schema than migrated databases. Always run `validate-migrations.sh`.

---

## Rollback Contract

### Safe to Roll Back

- `ALTER TABLE ADD COLUMN` — column exists but is unused by old code
- `CREATE TABLE` — table exists but is unused by old code
- `CREATE INDEX` — index is unused by old code

### Not Safe to Roll Back

- `UPDATE`/`DELETE` data changes — data is lost
- `DROP COLUMN` — old code expects the column
- `ALTER COLUMN TYPE` — old code may use incompatible type
- `RENAME` — old code uses the old name
- `INSERT` seed data — old code may not handle the data

### Procedure

If a forward-only migration fails mid-flight:
1. **Do not re-run** — the database may be in an unknown state
2. **Restore from backup** — this is the only safe recovery
3. **Check migration_history** — see which version was partially applied
4. **Fix the migration** — make it idempotent, then re-deploy

---

## Testing Checklist

- [ ] Fresh database: `go test ./store/test/...` passes
- [ ] Existing database: incremental migrations apply cleanly
- [ ] Idempotency: running migrations twice doesn't fail
- [ ] Schema validation: `task validate:schema` passes
- [ ] Parity validation: `task validate:parity` passes
- [ ] Script tests: `task test:scripts` passes
- [ ] Widget loads: chat widget appears in browser (after bug 045 fix)

---

## Deployment Checklist

- [ ] All validation commands pass
- [ ] LATEST.sql is in sync with migration files
- [ ] Both SQLite and Postgres files exist for new migrations
- [ ] Binary builds: `task build:backend`
- [ ] `version:info` shows expected version
- [ ] No production URLs hardcoded in Hugo site

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Widget doesn't appear | `chatBaseUrl` hardcoded to localhost in content | Remove from `_index.md`, use `hugo.yaml` param |
| "Agent not found" 404 | Missing columns in `agent_tenants` | Run pending migrations (bug 045 fix) |
| `validate:parity` fails | LATEST.sql drift between drivers | Update LATEST.sql to match |
| `validate:migrations` fails | Migration files not in LATEST.sql | Update LATEST.sql |
| Build fails `test:scripts` | Script regression | Fix the script, re-run tests |

---

## Historical Context

| Bug | Issue | Fix |
|-----|-------|-----|
| 008 | Unique constraint failure in Postgres migration | `validate-pg-migrations.sh` |
| 009 | Migration 28 hotfix needed rollback | `validate-db-migrations.sh` |
| 015 | LATEST.sql drift | `validate-migrations.sh` |
| 045 | `GetCurrentSchemaVersion()` only scanned one directory | FS-derived version detection |
| 046 | No cross-driver parity enforcement | `validate:parity`, `create-migration.sh` |
