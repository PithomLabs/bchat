# Bug 046: Implementation Record

**Date:** 2026-07-23
**Plan:** plan5.md (supersedes plan4.md — APPROVED for coding)
**Status:** Implemented (14 of 15 steps; Step 17 is deploy pipeline)

---

## Implementation Summary

### Part B: Migration Automation (bchat server)

| Step | File | Change |
|------|------|--------|
| 1 | `internal/version/version.go` | Version constants bumped from `0.31.0` to `0.34.0`. `Version` and `DevVersion` remain as defaults; runtime version is derived at startup by `GetCurrentSchemaVersion()` scanning the embedded FS (bug 045 fix). |
| 2 | `scripts/bump-version.sh` | New. Read-only informational script. Scans `store/migration/sqlite/` for highest directory + patch, computes version, prints comparison against `version.go`. No sed, no file modification. Exit 0 = match, exit 1 = mismatch (informational). |
| 3 | `scripts/create-migration.sh` | New. Creates SQLite and Postgres migration files with TODO templates. No auto-generation (removed per plan3_review.md #2). Validates snake_case name, calls `bump-version.sh` informationally, supports `--dry-run`. |
| 4 | `docs/TYPE_MAPPING.md` | New. 250+ line reference covering type mapping table, syntax differences, migration writing rules, review checklist, SQL parsing limitations disclaimer, historical divergences, known divergence cases. |
| 5 | `scripts/validate-parity.sh` | New. Three-check validator: (1) File-list parity — CI gate, fails on missing files; (2) Schema parity — best-effort lint, warns on differences; (3) Known divergences — skips historical mismatches (0.2–0.18 SQLite-only, 0.25–0.30 catch-up, 0.33 intentional). |
| 6 | `scripts/test/` | New. Test runner (`run-tests.sh`) with 12 assertions covering bump-version detection, create-migration validation, schema comparison, drift detection, file-list mismatch. All 12 pass. |
| 7 | `Taskfile.yml` | Added tasks: `version:info`, `migrate:new`, `validate:parity`, `test:scripts`. Updated `build:backend` deps to include `validate:parity` and `test:scripts` (CI gates). |
| 8 | `AGENTS.md` | Replaced Database Migrations section with new workflow: `task migrate:new` as primary, three validation commands listed. |
| 9 | `docs/DOCS_DATABASE_MIGRATION.MD` | Deprecation notice added at top, pointing to new guide. |
| 10 | `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` | New. Definitive guide: quick reference, architecture, step-by-step migration, rules, parity philosophy, CI gates matrix, rollback contract, gotchas, testing/deployment checklists, troubleshooting, historical context. |

### Part A: Hugo Chat Widget Fix (Hugo site)

| Step | File | Change |
|------|------|--------|
| 12 | Hugo `hugo.yaml` | Added `params.bchat.baseUrl: "http://localhost:8081"` under existing params. Safe default for local dev; production overrides via config/ or CLI. |
| 13 | Hugo `layouts/_default/list.html:225` | Changed `\| default "https://bchat-pg.fly.dev"` to `or .Params.chatBaseUrl site.Params.bchat.baseUrl "http://localhost:8081"`. Added `warnf` for missing config. |
| 14 | Hugo `content/rgresidences/_index.md` | Removed `chatBaseUrl: "http://localhost:8081"` line. |
| 15 | Hugo `content/bchat/_index.md` | Removed `chatBaseUrl: "http://localhost:8081"` line. |
| 16 | Hugo `content/evpn/_index.md` | Removed `chatBaseUrl: "http://localhost:8081"` line. |
| 17 | Deploy pipeline | **NOT IMPLEMENTED** — outside this codebase. Requires grep on Hugo build output for "WARNING: bchat.baseUrl not set" and failing the step. |

---

## Files Created (7)

```
scripts/bump-version.sh
scripts/create-migration.sh
scripts/validate-parity.sh
scripts/test/run-tests.sh
scripts/test/bump-version/version.go.fixture
scripts/test/bump-version/migration-tree/0.33/00__test.sql
scripts/test/bump-version/migration-tree/0.33/01__test2.sql
scripts/test/bump-version/latest-sqlite.sql.fixture
scripts/test/bump-version/latest-postgres.sql.fixture
scripts/test/bump-version/latest-postgres-drift.sql.fixture
docs/TYPE_MAPPING.md
docs/DOCS_DATABASE_MIGRATION_GUIDE.md
```

## Files Modified (9)

```
internal/version/version.go          (0.31.0 → 0.34.0)
Taskfile.yml                         (4 new tasks + build:backend deps)
AGENTS.md                            (migration section rewritten)
docs/DOCS_DATABASE_MIGRATION.MD      (deprecation notice added)
hugo.yaml                            (params.bchat.baseUrl added)
layouts/_default/list.html           (or pipe + warnf)
content/rgresidences/_index.md       (chatBaseUrl removed)
content/bchat/_index.md              (chatBaseUrl removed)
content/evpn/_index.md               (chatBaseUrl removed)
```

---

## Verification Results

| Check | Command | Result |
|-------|---------|--------|
| Build | `go build -o build/memos ./bin/memos/main.go` | passed |
| Schema tests | `go test ./store/test/... -run "TestSchemaValidation\|..."` | passed |
| LATEST.sql sync | `./scripts/validate-migrations.sh` | passed — LATEST.sql in sync |
| Cross-driver parity | `./scripts/validate-parity.sh` | passed — schema + file-list parity |
| Script tests | `./scripts/test/run-tests.sh` | passed — 12/12 |
| Version info | `./scripts/bump-version.sh` | 0.33.1 computed vs 0.34.0 in version.go (expected) |

---

## Key Design Decisions

1. **No sed-based version bumping** — `bump-version.sh` is read-only. Version is derived at runtime from embedded FS. Eliminates merge conflicts and sed fragility (plan3_review.md #1).

2. **No auto-generation of Postgres SQL** — `create-migration.sh` creates TODO templates only. The 10-row substitution table is a leaky abstraction (plan3_review.md #2). Developer writes SQL for each driver.

3. **`or` instead of `default` in Hugo** — Hugo's `default` doesn't fall through on empty string `""`. `or` handles both nil and zero-value correctly (plan_review.md #3).

4. **File-list parity as CI gate, schema parity as lint** — Shell SQL parsing is unreliable. File-list parity (file existence) is reliable and catches the most common drift (plan_review.md #4, plan3_review.md #4).

5. **Known divergences documented in validator** — Historical mismatches (SQLite-only dirs 0.2–0.18, catch-up migrations 0.25–0.30, intentional 0.33 divergence) are suppressed to avoid noise.

---

## Adversarial Code Review Prompt

> Review the Bug 046 implementation with an adversarial mindset. Focus on:
>
> ### Script Correctness
> 1. `bump-version.sh`: Does the `find_latest_dir` sort correctly handle all version formats (0.2, 0.10, 0.33)? Could `sort -V` behave unexpectedly with leading zeros?
> 2. `create-migration.sh`: Is the snake_case validation regex `^[a-z][a-z0-9_]*$` too strict or too permissive? Should it reject names starting with numbers?
> 3. `validate-parity.sh`: The `extract_tables` function uses `grep -oP` with a complex regex. Could it produce false positives on views, CTEs, or temporary tables? Are there edge cases where it misses tables?
> 4. `validate-parity.sh`: The `KNOWN_DIVERGENCES` map uses `declare -A`. Is this portable across bash versions? What happens on macOS with bash 3.2?
> 5. `run-tests.sh`: The test runner creates temp dirs with `mktemp -d` and symlinks to real scripts. Could path resolution fail in CI environments?
>
> ### Security
> 6. The hardcoded fallback is `http://localhost:8081`. Is this the right security posture? What if a production deployment accidentally omits the hugo.yaml override?
> 7. `create-migration.sh` validates input with regex but doesn't sanitize the migration name for filesystem paths. Could a crafted name cause path traversal?
> 8. Are there any CSP implications of loading a script from a configurable URL in the Hugo template?
>
> ### Hugo Template
> 9. The `or` chain: `or .Params.chatBaseUrl site.Params.bchat.baseUrl "http://localhost:8081"` — does Hugo's `or` evaluate left-to-right and return the first truthy value? What if `site.Params.bchat.baseUrl` is not defined at all?
> 10. The `warnf` fires on every page build when `bchat.baseUrl` is missing. Could this produce excessive noise in large sites?
>
> ### Parity Validator
> 11. The schema parity check only compares table names and index names. It doesn't compare column types, constraints, or foreign keys. Is this a significant gap? Could two LATEST.sql files have the same tables but different column definitions?
> 12. The file-list parity check compares ALL directories, including historical ones (0.2–0.18). The known divergences map suppresses them, but a new directory without a corresponding entry would fail. Is the map maintainable?
>
> ### Migration Guide
> 13. The rollback contract says `ALTER TABLE ADD COLUMN` is safe to roll back. But if the application code references the new column, rolling back the migration will cause runtime errors. Is this accurately documented?
> 14. The CI gates matrix lists 4 historical bugs. Are there other past bugs (015, etc.) that should be included?
>
> ### Regression Risk
> 15. The version bump from 0.31.0 to 0.34.0 changes `Version` and `DevVersion`. Are there any test assertions that hardcode "0.31." (besides the known `migrator_test.go:18` from bug 045)?
> 16. The `build:backend` task now depends on `validate:parity` and `test:scripts`. If either script has a bug, the build will fail. Is this the right tradeoff?
>
> ### Missing Pieces
> 17. Step 17 (deploy CI gate) is not implemented. Should it be documented as a TODO in AGENTS.md or the migration guide?
> 18. The `validate-parity.sh` schema check is best-effort lint. Should there be a follow-up task to implement database-introspection-based parity checking?
> 19. Are there any edge cases in `create-migration.sh` where the patch number calculation could collide with existing files?
>
> Provide specific, actionable findings with severity levels (critical/high/medium/low).
