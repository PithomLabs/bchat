# Fix Plan: Test Failures in store/test

## Summary of Failures

Two tests are failing based on the test output in `/home/chaschel/Documents/go/bchat/bugs/012/testing.txt`:

### 1. TestGetCurrentSchemaVersion (migrator_test.go:16)
- **Expected**: `"0.24.2"`
- **Actual**: `"0.25.29"`
- **Root cause**: The test hardcodes an expected schema version that doesn't match the current codebase. The version in `internal/version/version.go` shows `DevVersion = "0.25.0"`, and `GetCurrentSchemaVersion()` calculates the version based on migration files in `store/migration/sqlite/0.25/`, which has migrations up to `28__tickets_creator_description_unique.sql`. This results in version `0.25.29` (patch 28 + 1).

### 2. TestTicketForeignKeyConstraints (ticket_test.go:119)
- **Expected**: An error when creating a ticket with invalid `creator_id` (non-existent user)
- **Actual**: No error - ticket was created successfully
- **Root cause**: The `LATEST.sql` schema shows `tickets` table WITHOUT foreign key constraints on `creator_id` and `assignee_id`. However, the migration `0.25/04__tickets_add_foreign_keys.sql` adds these constraints. The test uses a fresh database created via `NewTestingStore()` which applies `LATEST.sql` directly, bypassing the migration that adds the foreign keys.

## Analysis

### Test 1 - Schema Version Test
The test `TestGetCurrentSchemaVersion` is checking that the schema version matches `"0.24.2"`, which is outdated. The codebase has evolved to `0.25.x` with 28 migration files in the `0.25` directory. Per the principle "do not modify passing tests unless fundamentally flawed", this test is **flawed** - it's checking an obsolete value and should be updated.

### Test 2 - Foreign Key Constraint Test
The test `TestTicketForeignKeyConstraints` expects foreign key constraints to be enforced on the `tickets` table, but:
1. The `LATEST.sql` schema does NOT include `FOREIGN KEY (creator_id) REFERENCES user(id)` or `FOREIGN KEY (assignee_id) REFERENCES user(id)`
2. Migration `04__tickets_add_foreign_keys.sql` adds these constraints via table recreation
3. The test setup calls `store.Migrate()` which uses `preMigrate()` to apply `LATEST.sql` directly
4. Since `resetTestingDB()` only clears tables for MySQL/PostgreSQL (not SQLite), and the migration check compares versions, the LATEST.sql is applied without the FK constraints

This test is **invalid for the current architecture** - it expects constraints that exist in an incremental migration file but NOT in the authoritative `LATEST.sql` schema.

## Recommended Actions

### Action 1: Update TestGetCurrentSchemaVersion
**File**: `store/test/migrator_test.go`

Update line 16 from:
```go
require.Equal(t, "0.24.2", currentSchemaVersion)
```
to:
```go
require.Equal(t, "0.25.29", currentSchemaVersion)
```

This aligns the test with the actual current schema version calculated from the migration files.

### Action 2: Fix Foreign Key Constraints in LATEST.sql
**File**: `store/migration/sqlite/LATEST.sql`

Update the `tickets` table definition (lines 145-165) to include foreign key constraints:

```sql
-- tickets
CREATE TABLE tickets (
   id INTEGER PRIMARY KEY AUTOINCREMENT,
   title TEXT NOT NULL,
   description TEXT NOT NULL DEFAULT '',
   status TEXT NOT NULL DEFAULT 'OPEN',
   priority TEXT NOT NULL DEFAULT 'MEDIUM',
   creator_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
   assignee_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
   created_ts BIGINT NOT NULL,
   updated_ts BIGINT NOT NULL,
   type TEXT NOT NULL DEFAULT 'TASK',
   tags TEXT NOT NULL DEFAULT '[]',
   beads_id TEXT UNIQUE,
   parent_id INTEGER REFERENCES tickets(id),
   labels TEXT DEFAULT '[]',
   dependencies TEXT DEFAULT '[]',
   discovery_context TEXT,
   closed_reason TEXT,
   issue_type TEXT
 );
```

Also add the missing index:
```sql
CREATE INDEX idx_tickets_assignee_id ON tickets (assignee_id);
```

### Action 3: Verify Test After Fixes
After making the changes, run `go test ./store/test/...` to verify both tests pass.

## Alternative Approach (If LATEST.sql Should Not Have FKs)

If the foreign keys should remain only in the migration file (and not in `LATEST.sql`), then:
1. Remove the FK constraint checks from `TestTicketForeignKeyConstraints` 
2. Move them to a migration-specific test or mark them as skipped for the base LATEST.sql schema

However, this would be inconsistent with the purpose of `LATEST.sql` as the authoritative schema definition.

## Implementation Order

1. Fix `LATEST.sql` to include foreign key constraints (fixes root schema issue)
2. Update `TestGetCurrentSchemaVersion` to expect `"0.25.29"` (fixes version mismatch)
3. Run tests to verify fixes