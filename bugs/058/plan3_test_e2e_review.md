# Bug 058 — Adversarial Review: plan3_test_e2e.md

**Date:** 2026-08-03  
**Reviewer:** Senior Go / CockroachDB Architect  
**Artifact under review:** `bugs/058/plan3_test_e2e.md`  
**Plan being validated:** `bugs/058/plan_e2e.md` (v2)  
**Implementation:** 3-file change (vectordb_cockroach.go, crdb-init.sql, Taskfile.yml)

---

## Executive Summary

This revision is **much closer** to executable. The critical test-isolation fix (run Go tests before the app starts) is the right call, and T9/T10/T11 are materially improved.

However, there is **one remaining Critical execution blocker** in the Phase 2 ordering, and **one High-severity gap** in build dependency handling. Everything else is nits or already correct.

**Verdict:** REQUEST CHANGES — 1 Critical, 1 High. Rest is approved.

---

## Finding 1 (Critical) — Phase 2 Ordering Still Contradicts Test-Isolation Goal

**Section:** `plan3_e2e.md` — "Restructured Phase Ordering", T5, T8  
**Severity:** Critical  
**Type:** Execution blocker

### What the plan says

The "Critical Structural Insight" correctly identifies:

> `resetCockroachDB()` drops ALL tables... If `TestCockroachMigrateEndToEnd` runs while the app is using the database, the app's connection becomes invalid. **Solution: Run Go tests BEFORE starting the app.**

The phase ordering diagram shows:

```
Phase 2: Migration & Tests (crdb:migrate → Go tests → crdb:verify)
```

And T5 says:

| Phase 3 Needs | Phase 2 Provides | Verified By |
|---------------|------------------|-------------|
| App NOT running | Go tests finished, no app process yet | Gate check |

### What `crdb:migrate` actually does

`Taskfile.yml`:
```yaml
crdb:migrate:
  cmds:
    - |
      ...
      COCKROACH_DSN="${COCKROACH_DSN}" ./build/memos --driver=cockroach --mode dev --data {{.ROOT_DIR}}/build/data
```

`bin/memos/main.go` root command:
1. `storeInstance.Migrate(ctx)` — applies migrations
2. `server.NewServer(...)` — creates HTTP/gRPC server
3. `s.Start(ctx)` — binds to port 5230, starts serving
4. `<-ctx.Done()` — blocks until SIGINT/SIGTERM

`crdb:migrate` **starts the app and blocks**. It does not exit after migration.

### The contradiction

T8 correctly records:

| `crdb:migrate` starts app and blocks | Process binds to port 5230 | ✅ Correct |

But T5 assumes:

| App NOT running | Go tests finished, no app process yet | Gate check |

These two statements **cannot both be true**. If `crdb:migrate` blocks on port 5230, the Go tests in Phase 2 cannot run afterward, and the app IS running when Phase 3 starts.

The plan's stated goal (run tests before app starts) and its actual step ordering (`crdb:migrate` first, then tests) are directly contradictory.

### Impact

Executing the plan as written:
1. `crdb:migrate` starts the app in the foreground and blocks
2. Go tests never run
3. Phase 3 tries to start a second app instance — port conflict
4. The test-isolation problem is "solved" only because the tests never execute

This is a **showstopper**.

### Fix

Change Phase 2 ordering to match the stated intent:

```markdown
## Phase 2: Migration & Tests

### Steps

```bash
# 4. Run Go tests FIRST (before app starts — prevents resetCockroachDB() conflict)
go test -tags "cockroach integration" ./store/ -run TestCockroachP0 -v
BCHAT_ALLOW_DB_RESET=1 go test -tags "cockroach integration" ./store/test/ -run TestCockroachMigrateEndToEnd -v
```

```bash
# 5. Boot app against CockroachDB (applies migrations, starts HTTP server)
task crdb:migrate &
MIGRATE_PID=$!
# Wait for healthz...
```

```bash
# 6. Run full verification
task crdb:verify
```
```

And update T8 to reflect that `crdb:migrate` runs in background during Phase 2:

| `crdb:migrate` runs in background | Process binds to port 5230 | Started with `&`, PID captured | ✅ Correct |

---

## Finding 2 (High) — `crdb:migrate` Build Dependency Still Unresolved

**Section:** `plan3_e2e.md` — T1b  
**Severity:** High  
**Type:** Execution failure on clean checkout

### What the plan says

T1b notes:

| `crdb:migrate` | No | ⚠️ Add explicit build step |

But the plan does **not** add the explicit build step anywhere in Phase 2. The phase ordering shows:

```
Phase 2: Migration & Tests (crdb:migrate → Go tests → crdb:verify)
```

No `task build:backend:cockroach` appears before `crdb:migrate`.

### Impact

On a clean checkout (no `build/memos` binary), `crdb:migrate` fails:

```
bash: ./build/memos: No such file or directory
```

The plan assumes `run:cockroach`'s `deps: [build:backend:cockroach]` covers this, but `crdb:migrate` does not have that dependency.

### Fix

Add an explicit build step at the start of Phase 2:

```markdown
```bash
# 3. Build binary (required — crdb:migrate does not have build dep)
task build:backend:cockroach
```
```

Or add `deps: [build:backend:cockroach]` to `crdb:migrate` in `Taskfile.yml`.

---

## Finding 3 (Medium) — T9/T9b PID Capture Assumes Single Process, But `task` Spawns a Subprocess Tree

**Section:** `plan3_e2e.md` — T9, T9b  
**Severity:** Medium  
**Type:** Operational gap

### What the plan says

T9 verifies:
- `BCHAT_PID=$!` captures task PID
- `kill $BCHAT_PID` terminates task
- App terminates with task

T9b checks:
- `ps aux | grep "build/memos" | grep -v grep | wc -l` == 0

### The issue

When `task run:cockroach &` is executed:
1. bash spawns `task` as a background process
2. `task` (Go binary) spawns a shell to execute the commands
3. That shell spawns `./build/memos`
4. The app may spawn additional goroutines/processes (embedding service, etc.)

`$!` captures the `task` PID. `kill $BCHAT_PID` sends SIGTERM to `task`. If `task` doesn't forward signals to its child shell, and that shell doesn't forward to the app, the app persists.

T9b checks for `build/memos` processes, which is correct. But the plan doesn't say what to do if T9b fails (i.e., orphaned processes found).

### Fix

Add a fallback to T9:

```markdown
| App terminates with task | `ps aux | grep build/memos | grep -v grep | wc -l` == 0 | ✅ |
| Orphan cleanup if needed | `pkill -f "build/memos"` or `kill -9 $BCHAT_PID` | Fallback |
```

---

## Finding 4 (Medium) — T10 Pattern Would Still Match Log Lines That Aren't Errors

**Section:** `plan3_e2e.md` — T10  
**Severity:** Medium  
**Type:** Test reliability

### What the plan says

```markdown
| No unexpected SQLSTATE errors | `! grep -i "SQLSTATE.*ERROR\|failed to.*SQLSTATE" build/memos.log"` |
```

### The issue

The pattern `SQLSTATE.*ERROR` requires "ERROR" to appear on the same line as "SQLSTATE". But log lines are typically:

```
ERROR: password authentication failed for user bchat (SQLSTATE 28P01)
```

Here, "ERROR" appears at the start and "SQLSTATE" appears later. The pattern `SQLSTATE.*ERROR` would NOT match this because "ERROR" comes before "SQLSTATE".

Similarly, `failed to.*SQLSTATE` would match "failed to connect: SQLSTATE 28P01", which is correct, but might also match "failed to retry after SQLSTATE 40P01" which could be a retryable error that's actually handled.

### Impact

The check may miss real SQLSTATE errors that appear in the standard PostgreSQL error format (`ERROR: ... (SQLSTATE ...)`).

### Fix

Use a pattern that matches both orderings:

```markdown
| No unexpected SQLSTATE errors | `! grep -iE "SQLSTATE.*(ERROR|FAIL)|ERROR.*SQLSTATE|failed to.*SQLSTATE" build/memos.log` |
```

Or simpler:
```markdown
| No unexpected SQLSTATE errors | `! grep -i "SQLSTATE" build/memos.log | grep -iE "ERROR|FAIL"` |
```

---

## Items Reviewed and Approved

### T1-T4: Static Validation ✅
All targets exist, test functions exist, SQL queries are valid, `/healthz` exists.

### T1b: Build Dependency ⚠️ (see Finding 2)
Correctly identifies the gap. Fix needed.

### T2, T2b: Test Isolation ✅
The restructuring (tests before app start) correctly addresses the `resetCockroachDB()` conflict. T2b verifies the structural fix.

### T5: Prerequisite Chain ⚠️ (see Finding 1)
The chain logic is correct in principle, but Phase 2 → Phase 3 assumes app is NOT running after Phase 2, which contradicts `crdb:migrate` blocking.

### T6: Gate Criteria ✅
Updated correctly for the new structure.

### T7: Failure Mode Walk-Through ✅
All verified.

### T8: Process Lifecycle ⚠️ (see Finding 1)
Correctly identifies that `crdb:migrate` blocks, but doesn't reconcile this with the Phase 2 ordering.

### T9, T9b: Background Process Control ⚠️ (see Finding 3)
Correct direction, but missing fallback for orphaned processes.

### T10: Automated Log Checks ⚠️ (see Finding 4)
Correct intent, pattern needs adjustment for standard error format.

### T11: crdb:init Behavioral Verification ✅
Correctly notes that behavioral tests require execution, not just static analysis.

### T5b: Cleanup Verification ✅
`trap` recommendation is correct and essential for interrupt safety.

---

## Correctness Summary by Dimension

| Dimension | Verdict | Notes |
|-----------|---------|-------|
| **Completeness** | ⚠️ Partial | Phase 2 ordering contradicts test-isolation goal |
| **Correctness** | ⚠️ Partial | T10 pattern needs adjustment; T9 missing fallback |
| **Gaps** | ❌ Critical | Phase 2 cannot execute as written |
| **Risk coverage** | ✅ Good | Signal propagation, orphaned processes, cleanup all covered |
| **Operational** | ⚠️ Partial | Commands are copy-pasteable but Phase 2 ordering is wrong |
| **Cloud readiness** | ⚠️ Blocked | Local E2E structure is correct in principle, but Phase 2 is broken |

---

## Required Changes Before Merging

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | Phase 2 ordering contradicts test-isolation goal — `crdb:migrate` blocks before Go tests run | Critical | Move Go tests before `crdb:migrate` in Phase 2, or run `crdb:migrate` in background |
| 2 | `crdb:migrate` has no build dependency | High | Add `task build:backend:cockroach` as first Phase 2 step |
| 3 | T9 missing orphaned-process fallback | Medium | Add `pkill -f build/memos` or `kill -9` fallback |
| 4 | T10 grep pattern misses standard ERROR-before-SQLSTATE format | Medium | Adjust pattern to match both orderings |

---

## Final Verdict

**REQUEST CHANGES**

The structural fix (tests before app start) is correct, but the **Phase 2 step ordering still defeats it**. `crdb:migrate` starts the app and blocks, which means:
1. Go tests never run after it
2. The app is already running when Phase 3 starts
3. The test-isolation problem returns

**Minimum viable fix:**
1. In Phase 2, run Go tests **before** `crdb:migrate`, OR run `crdb:migrate` in background with PID capture
2. Add explicit build step before `crdb:migrate`

Once those two changes are made, this plan is ready to execute. The remaining findings (T9 fallback, T10 pattern) are nits that can be fixed during execution.
