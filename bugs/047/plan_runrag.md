# `task run:rag` Failure — Parity Validation Fix

**Status:** Ready to implement (awaiting go-signal)
**Created:** 2026-07-23
**Scope:** Fix `validate:parity` exit status 3 blocking `task run:rag`

---

## Error Chain

```
task run:rag
  → task build:backend:rag
    → task validate:parity
      → ./scripts/validate-parity.sh
        → exit status 3 (file-list + schema mismatch)
```

---

## Root Cause

The `user_access_token_lookup` table (P0 #6 selection token O(1) lookup) was added as SQLite-only. No Postgres migration file or LATEST.sql update was created.

**Parity check output:**
```
Check 1: File-list parity
  MISSING: sqlite/0.34/ (exists in sqlite but not in other driver)

Check 2: Schema parity (best-effort lint)
  Tables in SQLite but NOT in Postgres:
    user_access_token_lookup
  Indexes in SQLite but NOT in Postgres:
    idx_user_access_token_lookup_user_id
```

---

## Migration System Context

The migrator has **two code paths** — both must be addressed:

| Path | Trigger | Reads |
|------|---------|-------|
| Fresh install | No `migration_history` rows | `LATEST.sql` (full schema) |
| Upgrade | `migration_history` has older version | Numbered files `0.34/00__*.sql` (incremental) |

Both migration files use `CREATE TABLE IF NOT EXISTS` / `CREATE INDEX IF NOT EXISTS` — idempotent, safe for both paths.

---

## Fix

### File 1: Create `store/migration/postgres/0.34/00__add_user_access_token_lookup.sql`

Incremental migration for upgrade paths (existing databases):

```sql
-- P0: Add user_access_token_lookup table for O(1) token lookups
-- Eliminates N+1 query pattern in selection token lookup (auth_service.go)
CREATE TABLE IF NOT EXISTS user_access_token_lookup (
    token_hash TEXT NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_access_token_lookup_user_id ON user_access_token_lookup(user_id);
```

**Postgres syntax differences from SQLite:**
- `EXTRACT(EPOCH FROM NOW())::BIGINT` instead of `strftime('%s', 'now')`
- `"user"` (quoted) instead of `user` (reserved word in Postgres)

### File 2: Update `store/migration/postgres/LATEST.sql`

Append the table + index at the end (after `agent_events` block, before end of file):

```sql
-- user_access_token_lookup
CREATE TABLE user_access_token_lookup (
    token_hash TEXT NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
);

CREATE INDEX idx_user_access_token_lookup_user_id ON user_access_token_lookup(user_id);
```

**Note:** LATEST.sql uses plain `CREATE TABLE` (no `IF NOT EXISTS`) — consistent with all other tables in the file.

---

## Files to Modify

| File | Action |
|------|--------|
| `store/migration/postgres/0.34/00__add_user_access_token_lookup.sql` | **CREATE** |
| `store/migration/postgres/LATEST.sql` | **APPEND** table + index |

---

## Verification

### 1. Parity Check
```bash
./scripts/validate-parity.sh
```
Expected: `PASS: All checks passed` (exit 0)

### 2. Task Build
```bash
task build:backend:rag
```
Expected: Clean build

### 3. Task Run
```bash
task run:rag
```
Expected: Starts successfully

---

## Definition of Done

- [ ] `store/migration/postgres/0.34/00__add_user_access_token_lookup.sql` created
- [ ] `store/migration/postgres/LATEST.sql` updated with table + index
- [ ] `./scripts/validate-parity.sh` exits 0
- [ ] `task build:backend:rag` succeeds
- [ ] `task run:rag` starts successfully
