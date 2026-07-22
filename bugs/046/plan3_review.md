# Adversarial Plan Review — Bug 046 (plan3.md)

**Reviewer:** AI Architect  
**Plan under review:** `plan3.md` (supersedes plan2.md)  
**Status:** ⚠️ REWORK (deeper review — not deadline-constrained)

---

## Summary

All 6 findings from the first review cycle (`plan_review.md`) were mechanically addressed in plan3. But five deeper architectural and operational issues remain. These are not nits — they are design choices that will compound into incident patterns (045-class) if left unaddressed.

---

## Remaining Issues

### 1. Version Constant via Sed Is the Wrong Abstraction

**Status:** Addressed mechanically (anchored patterns, sanity check) but the architecture is backward.

The version is derived from the migration filesystem at build time, then *written back* to Go source code via awk/sed, which then gets committed to git. This creates:
- Merge conflicts when two PRs run `bump-version.sh` independently
- A sed-incompatible Go source file if any developer reformats `version.go`
- No traceability between a binary's version and the migration files it was built from

**Correct approach:** Derive the version at runtime from the embedded filesystem. The migration files are already embedded via `//go:embed store/migration`. `GetCurrentSchemaVersion()` already walks `fs.Glob` — it just does it wrong (only scans one directory). Fix it to scan all directories, and the version is computed at startup with zero build-time ceremony. No `bump-version.sh`, no sed, no `version.go` edits, no merge conflicts.

The plan labels this "future improvement" (line 200). That deferral is the same pattern that created the 045/046 debt — the structural fix is deferred and the workaround gets committed.

**Required change:** Move version derivation from build-time sed to runtime FS globbing. `version.go`'s `Version` / `DevVersion` remain as defaults, but `GetCurrentSchemaVersion()` overrides them by scanning the embedded filesystem. The scripts become informational (show the computed version) rather than mutating source code.

### 2. Auto-Generated Postgres SQL Creates False Confidence

**Status:** Acknowledged (overwrite protection, flag for review) but the core approach is unsound.

The 10-row substitution table (line 228-240) is a leaky abstraction. Real SQLite→Postgres translation involves:

| Area | SQLite | Postgres | Auto-gen Handles? |
|------|--------|----------|-------------------|
| DDL transactions | ALTER TABLE is transactional | ALTER TABLE is NOT transactional | No |
| Partial indexes | `WHERE x IS NOT NULL` | Same syntax (OK) | Yes (trivial) |
| INSERT OR REPLACE | `INSERT OR REPLACE INTO` | `INSERT INTO ... ON CONFLICT ... DO UPDATE` | No — different semantics |
| Type affinity | `INTEGER`, `TEXT` are affinities | `INTEGER`, `TEXT` are concrete types | False confidence — works for simple cases |
| CHECK constraints | Simple expressions | Same syntax (mostly OK) | Yes (trivial) |
| Generated columns | Not well supported | Well supported | Not handled |
| RETURNING | Not supported | Supported | Not needed (no-op) |

A developer who trusts the auto-generated output and skips review will deploy broken Postgres migrations. The "flag for review" mechanism is a human process with no automation gate.

**Required change:** Remove auto-generation. `create-migration.sh` should create both files with TODO templates and prompt the developer to write SQL for each driver. Add a schema-level check that both files exist (file-list parity enforced by CI) but don't pretend the content can be machine-translated.

### 3. Zero Tests for Automation Scripts

**Status:** Not addressed in plan3.

The plan adds three bash scripts (`bump-version.sh`, `create-migration.sh`, `validate-parity.sh`) and a documentation file (`TYPE_MAPPING.md`). None have:
- Test fixtures (temp migration dir, fake files, run script, assert exit code + output)
- CI integration that exercises them against known-good/known-bad inputs
- Idempotency tests (run twice, assert same result)
- Edge-case tests (empty migration dir, mixed patch numbers, missing Postgres files)

These scripts will be modified over time. Without tests, regressions go undetected until someone's migration is silently skipped — the exact pattern that caused bug 045.

**Required change:** Add a `scripts/test/bump-version/` directory with test fixtures (`version.go` with known values, fake migration trees) and a test runner. Add a `task test:scripts` task. At minimum, validate in CI that each script exits 0 on valid input and non-zero on invalid input.

### 4. No Rollback Strategy Documented

**Status:** Not addressed in plan3.

The plan only covers the forward path: create migration, bump version, deploy. It omits:
- What happens when a binary panics mid-migration? (Partial `ALTER TABLE` applied, version not recorded)
- What happens when a rollback deploys old code against a database with new columns?
- What happens when a deployment is aborted after `bump-version.sh` ran but before the binary was built?

The plan's implicit answer is "migrations are idempotent" (line 654 of plan.md), but `ALTER TABLE DROP COLUMN`, data-migration DML (`UPDATE`, `DELETE`), and `INSERT` seed data are not idempotent. The `execute()` function's tolerance for "duplicate column" errors (line 264-269 of migrator.go) only covers `ALTER TABLE ADD COLUMN`.

**Required change:** Document the rollback contract explicitly in the migration guide:
- **Safe to roll back:** `ALTER TABLE ADD COLUMN`, `CREATE TABLE` (additive-only changes)
- **Not safe to roll back:** `UPDATE`/`DELETE` data changes, `DROP COLUMN`, `ALTER COLUMN TYPE`, `RENAME`
- **Procedure:** If a forward-only migration fails mid-flight, the database may be in an unknown state. The operator should restore from backup, not re-run.

This isn't a code change — it's a documentation gap that will cause a production incident.

### 5. Hugo `warnf` Is a Whisper, Not a Gate

**Status:** Addressed mechanically (warnf added) but the check is insufficient.

`warnf` prints to Hugo stderr but doesn't fail the build. A production deployment that omits `bchat.baseUrl` will:
1. Build successfully
2. Deploy successfully  
3. Serve pages where the widget silently fails (loads from localhost in the visitor's browser)
4. No developer is notified unless they watch the build logs

Hugo doesn't have a built-in `errorf` that fails the build conditionally. But the deploy script or CI can grep for the warning and fail.

**Required change:** In the deploy pipeline (GitHub Actions, deploy script), grep Hugo's build output for "WARNING: bchat.baseUrl not set" and fail the step if found:
```bash
hugo --environment production 2>&1 | tee hugo-build.log
if grep -q "bchat.baseUrl not set" hugo-build.log; then
  echo "FATAL: Production build missing bchat.baseUrl"
  exit 1
fi
```

---

## What Should Change in plan4.md

| Issue | Severity | Change |
|-------|----------|--------|
| 1 — Sed-based versioning | High | Derive version at runtime from embedded FS; scripts become read-only |
| 2 — Auto-generation false confidence | High | Remove auto-gen; create templates only; CI gate on file existence |
| 3 — No script tests | High | Add test fixtures + `task test:scripts` |
| 4 — No rollback strategy | Medium | Document rollback contract in migration guide |
| 5 — Hugo warnf insufficient | Low | Add CI grep on Hugo build output for missing bchat.baseUrl |

Issues 1-2 are architectural and should be resolved before implementation. Issues 3-4 can be added as follow-ups but should be documented now. Issue 5 is a deploy-script change that can be added independently.

---

## Summary for Next Plan

If this were an MVP with a hard deadline, the original plan3 was safe enough — file-list parity catches the most common drift, Hugo `or` fixes the widget, sed anchoring prevents corruption. The deeper issues (runtime versioning, auto-gen confidence, script tests, rollback) are debt that will eventually need paying but won't cause a production outage today.

If you're writing plan4 for a foundation you'll build on for months, resolve issues 1-3 now. They are the same class of problem as the original 045 bug: fragile tooling that fails silently, deferring failures to incident time.
