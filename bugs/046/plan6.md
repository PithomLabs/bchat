# Bug 046: Migration Loop Skip Logic — Plan 6

**Status:** PLANNED
**Date:** 2026-07-23
**Depends on:** Plan 5 (GetCurrentSchemaVersion FS-scan fix, Steps 0-5 already applied)
**Exposed by:** Plan 5 Step 1 — fixing `GetCurrentSchemaVersion()` to scan the FS correctly entered the migration loop for the first time, surfacing a dormant bug in the skip logic.

---

## Problem Statement

The migration loop in `store/migrator.go:108-138` uses an **exact-version-match** to skip already-applied migrations:

```go
appliedVersions := make(map[string]bool)
for _, v := range migrationHistoryVersions {
    appliedVersions[v] = true
}
// ...
if appliedVersions[fileSchemaVersion] {
    continue
}
```

`migration_history` stores **only one version per migration run** (the target). For a database at `0.31.3`, `appliedVersions = {"0.31.3": true}`. When the loop encounters `0.14/00__drop_resource_public_id.sql` (version `0.14.1`):

1. `appliedVersions["0.14.1"]` → false (not in map)
2. `IsVersionGreaterOrEqualThan("0.33.2", "0.14.1")` → true
3. **Executes the non-idempotent migration** → crashes with `no such column: external_link`

Before Plan 5 Step 1, this code path was never reached because `GetCurrentSchemaVersion()` returned a hardcoded constant that was ≤ the latest applied version. Our fix correctly computes `0.33.2` from the FS, which IS greater than `0.31.3`, so the loop runs — and the pre-existing bug surfaces.

### Why This Is Non-Idempotent

`0.14/00__drop_resource_public_id.sql` does:
```sql
INSERT INTO resource_temp ... SELECT ... external_link FROM resource;
```

By version `0.22`, `external_link` was DROPPED from `resource`. Re-running this migration on any database that passed `0.22` will crash.

---

## Root Cause

The skip logic checks if the **exact file version** exists in `migration_history`. But `migration_history` doesn't contain every individual file version — only the batch target. So all old migrations (0.2.x through 0.31.x) are NOT in the map and get re-executed. Most succeed due to idempotency tolerance (duplicate column/table errors are caught), but non-idempotent table-rebuild migrations crash.

---

## Fix

### Step 1: Replace exact-match with range check

**File:** `store/migrator.go`

Replace lines 78-86 and 113-116:

```go
// BEFORE (lines 78-86):
appliedVersions := make(map[string]bool)
for _, v := range migrationHistoryVersions {
    appliedVersions[v] = true
}

// BEFORE (lines 113-116):
if appliedVersions[fileSchemaVersion] {
    continue
}
```

With:

```go
// AFTER (replaces lines 78-86 — map no longer needed):
// (delete the appliedVersions map entirely)

// AFTER (replaces lines 113-116):
// Skip migrations already applied. migration_history stores only the batch
// target version, not every individual file version. Any file whose version
// is <= the latest applied version was already executed — either during
// incremental migration or when the database was first created via LATEST.sql.
if !version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion) {
    continue
}
```

This means:
- Files with version ≤ `0.31.3` → **skipped** (already applied)
- Files with version > `0.31.3` → **executed** (0.32.x, 0.33.x)

### Step 2: Add test `TestMigrationLoopSkipsAlreadyApplied`

**File:** `store/test/migrator_test.go`

Test that:
1. Creates a fresh SQLite DB
2. Reads `LATEST.sql` and applies it manually (simulating a database at version X)
3. Inserts a `migration_history` entry for version X
4. Runs `Migrate()`
5. Verifies only migrations > X are applied, not ALL migrations

### Step 3: Add test `TestNonIdempotentMigrationsNeverRerun`

**File:** `store/test/migrator_test.go`

Test that:
1. Lists all migration files that contain `INSERT INTO ... SELECT ... FROM` patterns (non-idempotent)
2. For each, verifies the migration version is ≤ the FS max version (i.e., it was already applied)
3. Ensures the skip logic would prevent re-execution

### Step 4: Run full verification

```bash
go test ./store/test/... -count=1 -timeout 30s   # unit tests
go test ./store/test/... -count=1 -run TestMigrationLoop -v  # new tests
task run:rag   # full pipeline
```

### Step 5: Update plan5.md

Add a "Known Issues" section documenting this as an additional finding from Plan 5's implementation.

---

## Adversarial Plan Review

After implementation, read `plan6.md` into context and run this prompt:

```
You are performing an adversarial plan review. Your job is to find flaws,
edge cases, and risks that the plan author missed. Be thorough and skeptical.

Review this plan for:

1. CORRECTNESS: Does the fix actually solve the problem? Could it introduce new bugs?
   - What happens if migration_history has MULTIPLE versions (e.g., from
     normalizedMigrationHistoryList)? Does the range check still work?
   - What happens if a migration was partially applied (tx commit failed mid-batch)?
   - What happens if someone manually inserts a wrong version into migration_history?

2. EDGE CASES: What about:
   - Fresh database (no migration_history) → preMigrate applies LATEST.sql
   - Database at version 0.22.1 where migration_history has both 0.22.1 and 0.31.3
   - Database with gaps in migration_history (e.g., 0.14.1, 0.31.3 — missing 0.22.x)
   - Migration files added AFTER the database was created (new minor version)

3. REGRESSION: Could this fix break existing working databases?
   - Databases already at 0.33.2: migration loop won't run (version not greater) — safe
   - Databases at 0.31.3: will now correctly apply 0.32.x and 0.33.x — correct
   - Databases at 0.22.1: will correctly apply 0.23.x through 0.33.x — correct

4. IDENTITY: Is the range check semantically correct?
   - `!IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion)` means
     fileSchemaVersion <= latestMigrationHistoryVersion → skip
   - Is there a case where a migration at version X should run even though
     migration_history contains a version >= X?

5. TEST COVERAGE: Do the new tests adequately cover the failure mode?
   - Do they test the exact scenario from taskrunrag_fail.md?
   - Do they test the non-idempotent migration case specifically?

6. PLAN COMPLETENESS: Are there any steps missing?
   - Should we also clean up the `appliedVersions` map construction?
   - Should we add a migration version consistency check (every file version <= FS max)?
   - Should we update the plan5_review.md with this finding?

For each finding, rate severity (CRITICAL / HIGH / MEDIUM / LOW) and suggest
a specific mitigation. If no issues found, say "No issues found" explicitly.
```

---

## Expected Findings

| # | Finding | Severity | Mitigation |
|---|---------|----------|------------|
| 1 | `normalizedMigrationHistoryList` can insert multiple versions into `migration_history` for pre-0.22 databases. Range check still works because we compare against `latestMigrationHistoryVersion` (the max). | LOW | No change needed — already correct |
| 2 | If tx commit fails mid-batch, `migration_history` won't be updated, so next run re-applies the batch. Non-idempotent migrations in that batch would crash. | MEDIUM | This is a pre-existing issue — document as known limitation. Wrap batch in a single tx (already done). |
| 3 | Fresh database: `preMigrate` applies `LATEST.sql` and sets `migration_history` to FS version. The migration loop is never entered. Safe. | LOW | No change needed |
| 4 | The `appliedVersions` map construction is dead code after this fix. Should be removed. | LOW | Remove lines 78-86 in Step 1 |
