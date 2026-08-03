# Bug 058 — Adversarial Review: plan2_test_e2e.md

**Date:** 2026-08-03  
**Reviewer:** Senior Go / CockroachDB Architect  
**Artifact under review:** `bugs/058/plan2_test_e2e.md`  
**Plan being validated:** `bugs/058/plan_e2e.md` (v2, revised)  
**Implementation:** 3-file change (vectordb_cockroach.go, crdb-init.sql, Taskfile.yml)

---

## Executive Summary

The revised meta-test plan **correctly addresses the Critical blocking issue** from the previous review by merging Phase 2 and Phase 3. The new test cases T8-T11 are well-chosen and cover the previously gaps.

However, there are **2 High-severity findings** and **2 Medium-severity findings** that need to be addressed before this plan can be trusted to validate `plan_e2e.md` correctly.

**Verdict:** REQUEST CHANGES — 2 High findings, 2 Medium findings.

---

## Finding 1 (High) — T9 Signal Propagation Note is Misleading

**Section:** `plan2_test_e2e.md` — T9  
**Severity:** High  
**Type:** Operational correctness

### What the plan says

```markdown
**Note:** `$!` captures the shell PID spawned by `task`. When `kill` sends SIGTERM, 
the shell exits and the app (child process) receives SIGTERM too. If the app doesn't 
terminate, `kill -9 $BCHAT_PID` forces it.
```

### What actually happens

1. `task run:cockroach &` — bash spawns the `task` binary as a background process
2. `$!` captures the **PID of the `task` binary**, not a shell
3. `task` is a Go program that reads Taskfile.yml and spawns shells to run commands
4. Those shells spawn `./build/memos --mode dev ...`
5. The app becomes a **grandchild** of the original bash

When `kill $BCHAT_PID` sends SIGTERM:
- It goes to the `task` process
- Go programs do **not** automatically forward SIGTERM to child processes
- The `task` binary may or may not forward the signal depending on its implementation
- The shell spawned by `task` may or may not forward the signal to its children
- The app may **not** receive SIGTERM and could become an orphaned process

### Impact

If `kill $BCHAT_PID` doesn't terminate the app:
- Phase 5 cleanup fails because the app is still running
- Port 5230 remains bound
- Subsequent test runs fail with "address already in use"
- The cleanup step leaves orphaned processes

### Fix

Update T9 to verify actual signal propagation behavior:

```markdown
### T9: Background Process Control

| Check | Verification |
|-------|-------------|
| `task run:cockroach &` puts app in background | `jobs` shows background job |
| `BCHAT_PID=$!` captures task PID | `ps -p $BCHAT_PID -o comm=` shows `task` |
| `kill $BCHAT_PID` terminates task | `ps -p $BCHAT_PID` returns empty within 5s |
| App terminates with task | `ps aux | grep build/memos` shows no matching process |
| `wait $BCHAT_PID` doesn't hang | Returns within 5s of kill |

**Note:** Signal propagation from `task` to the app is implementation-dependent. 
Verify that killing the task PID also kills the app. If the app persists, use 
`kill -9 $BCHAT_PID` or implement a process group kill (`kill -TERM -$BCHAT_PID`).
```

And add a test case for orphaned processes:

```markdown
### T9b: Orphaned Process Check

After `kill $BCHAT_PID`:
```bash
# Verify no orphaned app processes
ps aux | grep "build/memos" | grep -v grep | wc -l
# Expected: 0
```
```

---

## Finding 2 (High) — T10 Automated Log Checks Would False-Positive on Expected Errors

**Section:** `plan2_test_e2e.md` — T10  
**Severity:** High  
**Type:** Test correctness

### What the plan says

```markdown
| Gate | Automated Check |
|------|-----------------|
| No SQLSTATE errors in logs | `! grep -qi "SQLSTATE" build/memos.log"` |
| Driver = cockroach | `grep -q "driver.*cockroach\|Driver.*cockroach" build/memos.log"` |
| DB vs OpenRouter errors | `grep -c "OpenRouter\|openrouter" build/memos.log"` and `grep -c "SQLSTATE" build/memos.log"` |
```

### What the code actually does

`vectordb_cockroach.go:112-135` — `Validate()` intentionally handles and logs expected SQLSTATE errors:

```go
_, err = v.db.ExecContext(ctx, `
    CREATE VECTOR INDEX IF NOT EXISTS idx_agent_vectors_embedding
    ON agent_vectors (embedding)
`)
if err != nil {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "42P07":
            slog.Info("Vector index already exists", "index", "idx_agent_vectors_embedding")
        case "0A000":
            slog.Warn("Vector index feature not supported, using brute-force search",
                "error", err,
                "hint", "Ensure feature.vector_index.enabled = true or upgrade CRDB")
        default:
            slog.Warn("Vector index creation failed",
                "error", err,
                "hint", "May need feature.vector_index.enabled or CRDB v25.2+")
        }
    } else {
        slog.Warn("Vector index creation failed (non-PG error)",
            "error", err)
    }
}
```

These are **expected, handled errors** during concurrent index creation or feature detection. They are logged at INFO/WARN level, not ERROR.

### Impact

The check `! grep -qi "SQLSTATE" build/memos.log` would **fail** if any of these expected errors occur, even though they are part of normal operation. This makes the gate criteria non-deterministic — it depends on whether concurrent index creation happens during the test run.

Similarly, `grep -q "driver.*cockroach"` might match unrelated log lines, and `grep -c "OpenRouter"` counts all occurrences including successful API calls.

### Fix

Update T10 to check for unexpected errors only:

```markdown
### T10: Automated Log Checks

For the 3 gates that previously required manual log inspection:

| Gate | Automated Check |
|------|-----------------|
| No unexpected SQLSTATE errors | `! grep -i "SQLSTATE.*ERROR\|failed to.*SQLSTATE" build/memos.log` |
| Driver = cockroach | `grep -q "driver=cockroach" build/memos.log` (exact match) |
| No DB errors vs OpenRouter errors | `! grep -i "SQLSTATE.*ERROR" build/memos.log && ! grep -qi "OpenRouter.*error\|OpenRouter.*fail" build/memos.log` |

If any check fails, print matching log lines for human review.
```

---

## Finding 3 (Medium) — T11 Verifies Code Patterns, Not Behavior

**Section:** `plan2_test_e2e.md` — T11  
**Severity:** Medium  
**Type:** Verification completeness

### What the plan verifies

```markdown
| Check | Verification |
|-------|-------------|
| `set -e` is first line | `grep -A 2 "crdb:init:" Taskfile.yml | grep "set -e"` |
| `task: crdb:init` syntax in crdb:reset | `grep "task: crdb:init" Taskfile.yml` (not `task crdb:init`) |
| Retry loop exists | `grep "seq 1 30" Taskfile.yml` |
```

These are **static code pattern checks**. They verify that the Taskfile contains certain text, but they don't verify that the behavior is correct.

### What's missing

1. **`set -e` effectiveness**: The check verifies `set -e` is present, but doesn't verify it actually causes the task to fail when `cockroach sql` fails. A shell block with `set -e` could still swallow errors if commands are chained with `||` or `&&`.

2. **`task: crdb:init` ordering**: The check verifies the syntax exists, but doesn't verify it's in the right place (after `docker compose up -d --wait`, not before).

3. **Retry loop behavior**: The check verifies the loop exists, but doesn't verify it actually retries on failure.

### Impact

The meta-test plan could pass even if `crdb:init` has subtle behavioral bugs that cause it to silently succeed after partial failure.

### Fix

Add behavioral checks:

```markdown
### T11: `crdb:init` Behavioral Verification

| Check | Verification |
|-------|-------------|
| `set -e` is first line | `grep -A 2 "crdb:init:" Taskfile.yml | grep "set -e"` |
| `set -e` causes failure on error | Run `crdb:init` with invalid DSN; verify task exits non-zero |
| `task: crdb:init` syntax in crdb:reset | `grep "task: crdb:init" Taskfile.yml` (not `task crdb:init`) |
| `task: crdb:init` is after `up -d --wait` | Verify ordering in `crdb:reset` cmds |
| Retry loop exists | `grep "seq 1 30" Taskfile.yml` |
| Retry loop actually retries | Run `crdb:init` with container not ready; verify retries before failing |
```

---

## Finding 4 (Medium) — T2 Doesn't Verify Test Isolation

**Section:** `plan2_test_e2e.md` — T2, T5  
**Severity:** Medium  
**Type:** Test correctness

### What the plan verifies

```markdown
| Function | File |
|----------|------|
| `TestCockroachP0` | `store/cockroach_p0_test.go` |
| `TestCockroachMigrateEndToEnd` | `store/test/cockroach_migrate_test.go` |
```

The meta-test verifies that these test functions exist, but it doesn't verify what they do.

### What the tests actually do

`TestCockroachMigrateEndToEnd` uses `BCHAT_ALLOW_DB_RESET=1`, which allows the test to reset the database. In the merged Phase 2 structure:
1. App starts in background (`task run:cockroach &`)
2. Go tests run, potentially resetting the database
3. App is still running and using the database

If the test resets the database while the app is running:
- The app's database connection becomes invalid
- The app may crash or return errors
- Subsequent `verify-production.sh` steps fail

### Impact

The merged phase structure introduces a **test isolation issue** that the meta-test plan doesn't catch. The Go tests and the running app share the same database, and the tests are designed to reset it.

### Fix

Add a test case that verifies test isolation:

```markdown
### T2b: Test Isolation Verification

| Check | Verification |
|-------|-------------|
| `TestCockroachMigrateEndToEnd` uses `BCHAT_ALLOW_DB_RESET` | `grep "BCHAT_ALLOW_DB_RESET" store/test/cockroach_migrate_test.go` |
| Test does not conflict with running app | Review test code for database resets while app is running |
| Test uses separate database or schema | Check test setup for isolation mechanism |

**Note:** If the test resets the database while the app is running, the merged 
phase structure will fail. Consider:
- Running tests before starting the app, OR
- Using a separate test database, OR
- Disabling `BCHAT_ALLOW_DB_RESET` during E2E test
```

---

## Finding 5 (Medium) — T5 Phase 3→Phase 4 Trace Has Cleanup Gap

**Section:** `plan2_test_e2e.md` — T5  
**Severity:** Medium  
**Type:** Execution gap

### What the plan says

```
#### Phase 3 → Phase 4

| Phase 4 Needs | Phase 3 Provides | Verified By |
|---------------|------------------|-------------|
| App still running | Phase 3 didn't kill it | Gate check |
| PID captured | Phase 2 step 4 captures PID | Variable available |
```

### What's missing

The trace assumes Phase 3 doesn't kill the app. But what if:
1. Phase 3's environment check fails and the plan aborts?
2. Phase 4's `verify-production.sh` fails and the user kills the app manually?
3. Phase 4's cleanup steps (kill app, stop container) are skipped?

The Phase 4 → Phase 5 trace shows:
```
| App stopped | Phase 4 step 15 (`kill $BCHAT_PID`) | Cleanup |
| Container stopped | Phase 4 step 15 (`crdb:down`) | Cleanup |
```

But Phase 4 step 15 is listed as **both** `kill $BCHAT_PID` and `crdb:down`. Are these the same step or different steps? If they're the same step, the trace is confusing. If they're different steps, the trace should show them separately.

### Impact

If cleanup is skipped or fails, Phase 5's idempotency proof will fail because:
- The app is still running and holding port 5230
- The container is still running with the old data
- Phase 5's `crdb:up` might fail because the port is already in use

### Fix

Add cleanup verification to T5:

```markdown
#### Phase 4 → Phase 5

| Phase 5 Needs | Phase 4 Provides | Verified By |
|---------------|------------------|-------------|
| App stopped | Phase 4 cleanup step | `ps aux | grep build/memos` returns empty |
| Container stopped | Phase 4 cleanup step | `docker ps` shows no `bchat-crdb` |
| PID variable still set | Phase 2 step 4 | `[ -n "$BCHAT_PID" ]` |
```

And add a test case:

```markdown
### T5b: Cleanup Verification

| Check | Verification |
|-------|-------------|
| Cleanup runs even if Phase 4 fails | Add `trap` in plan_e2e.md |
| `kill $BCHAT_PID` terminates app | `ps` check after kill |
| `crdb:down` stops container | `docker ps` check after down |
| Phase 5 can start fresh | `crdb:up` succeeds after cleanup |
```

---

## Items Reviewed and Approved

### T1-T4: Static Validation

**Status:** ✅ Correct

All Taskfile targets exist, Go test functions exist, SQL queries are valid, and `/healthz` endpoint exists.

### T1b: Build Dependency Verification

**Status:** ✅ Correct

Verifies that `run:cockroach` has `deps: [build:backend:cockroach]`. This addresses the previous finding about missing build dependency.

### T6: Gate Criteria Audit

**Status:** ⚠️ Partially correct (see Finding 2)

The gate criteria are updated for the merged phases, but the automated log checks are too strict.

### T7: Failure Mode Walk-Through

**Status:** ✅ Correct

All failure modes are verified.

### T8: Process Lifecycle Verification

**Status:** ⚠️ Partially correct (see Finding 1)

The test cases are correct in principle, but the "Actual" column just references code rather than measuring behavior. The note about signal propagation is misleading.

### Environment Audit

**Status:** ✅ Correct

All environment variables are documented and verified.

---

## Additional Observations

### Positive Changes from Previous Review

1. **Merged Phase 2+3**: The right fix for the blocking issue. Starting the app once and keeping it running through Phase 4 is the correct structure.

2. **T8 Process Lifecycle**: Correctly identifies that only one app process can run at a time.

3. **T9 Background Process Control**: Acknowledges the need to verify PID capture and cleanup.

4. **T10 Automated Log Checks**: Addresses the previous finding about manual log inspection.

5. **T11 crdb:init Verification**: Addresses the previous finding about behavioral verification.

### Remaining Gaps

1. **Concurrent reindex test**: The previous review mentioned that `Validate()` can be triggered concurrently via auto-bootstrap or `FORCE_REINDEX_ON_STARTUP=true`. The revised meta-test plan doesn't include a test for this scenario.

2. **Test isolation**: The Go tests in Phase 2 might reset the database while the app is running. The meta-test plan doesn't verify that the tests are isolated from the running app.

3. **Signal propagation**: The T9 note makes assumptions about signal propagation that may not hold in practice.

---

## Correctness Summary by Dimension

| Dimension | Verdict | Notes |
|-----------|---------|-------|
| **Completeness** | ⚠️ Partial | Covers most previous findings, but misses test isolation and signal propagation |
| **Correctness** | ⚠️ Partial | T9 and T10 have correctness issues |
| **Gaps** | ⚠️ Medium risk | Test isolation and signal propagation are unverified |
| **Risk coverage** | ⚠️ Partial | Static validation is good; dynamic behavior still has gaps |
| **Operational** | ⚠️ Partial | Commands are copy-pasteable, but cleanup and signal handling are uncertain |
| **Cloud readiness** | ⚠️ Blocked | Local E2E structure is improved, but test isolation needs verification |

---

## Required Changes Before Merging

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | T9 signal propagation note is misleading | High | Verify actual signal propagation behavior; add orphaned process check |
| 2 | T10 log checks would false-positive on expected errors | High | Make checks specific to unexpected/handled errors |
| 3 | T11 verifies code patterns, not behavior | Medium | Add behavioral tests for `set -e` effectiveness and retry loop |
| 4 | T2 doesn't verify test isolation | Medium | Add test that verifies Go tests don't conflict with running app |
| 5 | T5 cleanup gap | Medium | Add cleanup verification and `trap` recommendation |

---

## Final Verdict

**REQUEST CHANGES**

The revised meta-test plan is a **significant improvement** over the previous version. The merged Phase 2+3 structure correctly addresses the blocking issue, and the new test cases T8-T11 cover most of the previous gaps.

However, the plan still has **operational correctness issues**:
1. Signal propagation assumptions in T9 are unverified and likely incorrect
2. Automated log checks in T10 would false-positive on expected errors
3. Test isolation between Go tests and the running app is unverified

**Before `plan2_e2e.md` can be approved:**
1. Fix T9 to verify actual signal propagation behavior
2. Fix T10 to check for unexpected errors only
3. Add T2b for test isolation verification
4. Add behavioral tests to T11
5. Add cleanup verification to T5

The structural fix (merged phases) is correct, but the operational details need more rigor before this plan can be trusted to validate `plan_e2e.md`.
