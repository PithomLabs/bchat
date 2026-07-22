# Adversarial Plan Review — Bug 046 Plan 5 (Definitive Long-Term Fix)

**Reviewer:** AI Architect (Senior Go / Systems)  
**Plan under review:** `plan5.md`  
**Status:** ✅ APPROVED (3 blocking nits, 3 non-blocking)

---

## Verdict

**Approved for coding after fixing 3 blocking nits (B1-B3).** The plan correctly eliminates the root cause class for the first time — version derivation from the embedded FS, with no human step required. The four-layer defense is well-structured.

However, three issues will cause compilation failures or incorrect behavior if implemented as written.

---

## 🔴 Blocking Nits (Fix Before Coding)

### B1. Step 5: Test Cannot Access `migrationFS` (Different Package)

**Issue:** `migrationFS` is declared at `store/migrator.go:22`:
```go
var migrationFS embed.FS
```

It is **unexported** (lowercase). The test in Step 5 is in `package teststore` (`store/test/migrator_test.go`). The proposed code calls:
```go
filePaths, err := fs.Glob(migrationFS, fmt.Sprintf("migration/sqlite/*/*.sql"))
```

This will not compile — `migrationFS` is inaccessible from the test package.

**Fix:** Choose one:
- **Option A** (preferred): Export `migrationFS` → `MigrationFS` (`store/migrator.go`). This is safe — it's an `embed.FS` that's read-only by construction.
- **Option B**: Make `TestAllMigrationFilesCoveredBySchemaVersion` an internal test in `package store` (move it or add a file to `store/` package). Loses access to `NewTestingStore` from `teststore`.
- **Option C**: Have the test read the real filesystem from the project root, not the embedded FS. Fragile (depends on working directory).

Option A is cleanest — one-line change, no behavioral impact.

The `getMigrationDirs` helper in Step 4 has the same problem (also uses `migrationFS`).

### B2. Step 21: `alreadyApplied` Algorithm Is Wrong

**Issue:** The proposed fix at line 666-670:
```go
alreadyApplied := false
for applied := range appliedVersions {
    if !version.IsVersionGreaterThan(fileSchemaVersion, applied) {
        alreadyApplied = true
        break
    }
}
```

This marks a file as already-applied if ANY applied version is `>=` the file version. This is **incorrect**. Consider:

- History has `"0.33.1"` (from `0.33/00__fix_max_message_length_default.sql`)
- File `0.32/00__foo.sql` computes to `"0.32.1"`
- `"0.32.1" > "0.33.1"` → **false** → `!false` → **true** → `alreadyApplied = true`
- But `"0.32.1"` was **never applied** — it was skipped by the bug!

The correct check is exact version match:
```go
if appliedVersions[fileSchemaVersion] {
    continue // already applied, skip
}
```

**But there's a deeper issue:** this scenario (history has `"0.33.1"` without `"0.32.x"`) likely **cannot occur** after Step 1's fix. The migration loop at line 77 already globs `*/*.sql` (all directories), so when it runs, it processes ALL files. The ONLY way to get a gap in history is through the phantom-version bug (GetCurrentSchemaVersion returning a wrong version), which Step 1 eliminates.

**Recommendation:** Add the map-lookup fix as defensive programming (the O(1) version, not the O(n*m) loop), but document in the code comment that this covers a theoretical edge case from the pre-Step-1 era, not a scenario that can occur after the fix. Reduce Step 21 from "High risk — new logic" to "Low risk — defensive hardening."

### B3. Step 22: SQLite `system_secret` Schema Mismatches LATEST.sql

**Issue:** The proposed migration at lines 695-699 defines:
```sql
CREATE TABLE IF NOT EXISTS system_secret (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    salt BLOB NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

But the actual `LATEST.sql` (at `store/migration/sqlite/LATEST.sql:472-478`) defines:
```sql
CREATE TABLE system_secret (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    encryption_salt BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    rotated_at BIGINT
);
```

Differences:
| Column | Step 22 | LATEST.sql | Impact |
|--------|---------|------------|--------|
| id constraint | `AUTOINCREMENT` (no CHECK) | `CHECK (id = 1)` | Missing CHECK — allows multiple rows |
| `encryption_salt` vs `salt` | `salt` | `encryption_salt` | Wrong column name — application code references `encryption_salt` |
| `key_version` | Missing | `key_version INTEGER NOT NULL DEFAULT 1` | Missing column |
| `created_at` type | `DATETIME DEFAULT CURRENT_TIMESTAMP` | `BIGINT NOT NULL DEFAULT (strftime('%s','now'))` | Wrong type — epoch seconds vs datetime |
| `rotated_at` | Missing | `BIGINT` | Missing column |

**Fix:** Match LATEST.sql exactly:
```sql
CREATE TABLE IF NOT EXISTS system_secret (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    encryption_salt BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    rotated_at BIGINT
);
```

This migration will be a no-op for fresh installs (table already exists from LATEST.sql) and a create for incremental upgrades.

---

## 🟡 Non-Blocking Nits

### N1. Step 2: No Escape Hatch for Prod Hard Error

**Issue:** The plan makes skipped migrations a hard error in prod mode (line 239-242). But production operators performing a rollback may need to intentionally skip a migration. The code offers no escape hatch — the operator would need to modify migration_history or comment out the error.

**Fix:** Add a `MIGRATE_SKIP_ERROR` env var or `--migrate-force-skip` flag that downgrades the error to a warning:
```go
if s.profile.Mode == "prod" && os.Getenv("MIGRATE_SKIP_ERROR") == "" {
    return errors.Errorf(...)
}
```
Document its use in the deployment guide: "Use only for emergency rollback scenarios."

### N2. Step 4: `getMigrationDirs` Can't Access `migrationFS`

**Issue:** Same as B1 — `migrationFS` is unexported. The helper in Step 4 also calls `fs.ReadDir(migrationFS, ...)`.

**Fix:** Same as B1 — export `migrationFS` to `MigrationFS`. Or inline the directory discovery into `TestAllMigrationFilesCoveredBySchemaVersion`.

### N3. Step 1: Glob Ordering Not Guaranteed

**Issue:** The new `GetCurrentSchemaVersion()` iterates over `fs.Glob` results but does not sort them before processing:
```go
for _, filePath := range filePaths {
    fileVer, err := s.getSchemaVersionOfMigrateScript(filePath)
    ...
    if maxVersion == "" || version.IsVersionGreaterThan(fileVer, maxVersion) {
        maxVersion = fileVer
    }
}
```

`fs.Glob` does NOT guarantee sorted output. While `IsVersionGreaterThan` handles unsorted input (it computes the max regardless of order), relying on unsorted iteration is fragile — a reader might assume sorted order and introduce a bug later.

**Fix:** Add the same `sort.Strings(filePaths)` that the current code uses at line 81. The sort is cheap and makes the semantics clear.

---

## ✅ Confirmed Correct

These items from the plan's adversarial prompt check out:

| # | Concern | Verdict |
|---|---------|---------|
| 1 | `*/*.sql` glob matches LATEST.sql? | **No** — LATEST.sql is at base path, not in subdir. Confirmed correct. |
| 2 | Empty migration directory? | Handled — glob returns nothing from empty dirs. Correct. |
| 3 | Future migrations committed before release? | Desirable — FS is source of truth. Feature, not bug. |
| 4 | Phantom version `"0.34.0"` transition? | **Correct** — databases at `"0.34.0"` become read/only (no-op), any future `0.34/00__` pushes version to `"0.34.1"` > `"0.34.0"`, migrations apply. |
| 8 | Hugo `or` vs `default` | `or` handles empty string `""`. Confirmed correct. |
| 10 | Backward compatibility with phantom version | Traced correctly through all scenarios 1-11. Correct. |

---

## Summary for Coding Agent

1. **Export `migrationFS`** to `MigrationFS` (B1 + N2, one-line fix in `store/migrator.go`)
2. **Fix `alreadyApplied` in Step 21** — use map lookup, not relative comparison (B2)
3. **Fix Step 22 schema** — copy columns verbatim from LATEST.sql (B3)
4. **Add `MIGRATE_SKIP_ERROR` env var** — escape hatch for prod rollbacks (N1)
5. **Add `sort.Strings`** to new `GetCurrentSchemaVersion` (N3)

After fixing the 3 blocking nits, the plan is sound for long-term use.
