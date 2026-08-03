# Bug 058 — Meta-Test Plan v3: Validating plan_e2e.md

**Date:** 2026-08-03
**Author:** opencode
**Status:** Revised per adversarial review (plan2_test_e2e_review.md)
**Depends on:** `plan_e2e.md` (v2), `plan2_e2e.md`

---

## Revision Summary

This plan addresses all 5 findings from the adversarial review of `plan2_e2e.md`:

| # | Finding | Severity | Change |
|---|---------|----------|--------|
| 1 | T9 signal propagation misleading — `task` is Go binary, doesn't forward SIGTERM | High | Rewrote T9: use `kill -TERM` + orphaned process check |
| 2 | T10 log checks false-positive on expected errors | High | Rewrote T10: check for unexpected errors only |
| 3 | T11 verifies code patterns, not behavior | Medium | Added behavioral verification notes |
| 4 | T2 doesn't verify test isolation — `resetCockroachDB()` drops ALL tables | Medium | Added T2b: test isolation verification; restructured phase ordering |
| 5 | T5 cleanup gap | Medium | Added T5b: cleanup verification with `trap` |

---

## Critical Structural Insight

**`store/test/cockroach_migrate_test.go:76-94`** — `resetCockroachDB()` drops ALL tables:

```go
func resetCockroachDB(t *testing.T) {
    db := cockroachRawDB(t)
    rows, err := db.QueryContext(context.Background(), `
        SELECT string_agg(format('DROP TABLE IF EXISTS %I CASCADE', table_name), '; ')
        FROM information_schema.tables
        WHERE table_schema = 'public'`)
    // ... executes the DROP statements
}
```

If `TestCockroachMigrateEndToEnd` runs while the app is using the database, the app's connection becomes invalid. **Solution: Run Go tests BEFORE starting the app.**

---

## Restructured Phase Ordering

```
Phase 1: Infrastructure Startup (crdb:reset)
    ↓
Phase 2: Migration & Tests (crdb:migrate → Go tests → crdb:verify)
    ↓                          ↑ tests run here, BEFORE app starts
Phase 3: App Startup (run:cockroach & — background)
    ↓
Phase 4: Data Path (verify-production.sh)
    ↓
Phase 5: Idempotency (crdb:down → crdb:up → verify-production.sh)
    ↓
Phase 6: Cleanup & Gate
```

**Key change:** Go tests run in Phase 2 BEFORE the app starts. This ensures `resetCockroachDB()` doesn't conflict with a running app.

---

## Test Cases

### T1: Taskfile Target Existence (unchanged)

| Target | Plan Step | Expected |
|--------|-----------|----------|
| `crdb:reset` | Phase 1 step 1 | Exists |
| `crdb:init` | Phase 1 internal | Exists (chained from crdb:reset) |
| `crdb:migrate` | Phase 2 step 4 | Exists |
| `run:cockroach` | Phase 3 step 4 | Exists |
| `crdb:verify` | Phase 2 step 7, Phase 5 step 11 | Exists |
| `crdb:down` | Phase 5 step 14, Phase 6 step 19 | Exists |
| `crdb:up` | Phase 5 step 15 | Exists |

```bash
grep -E "^  (crdb:reset|crdb:init|crdb:migrate|run:cockroach|crdb:verify|crdb:down|crdb:up):" Taskfile.yml
```

### T1b: Build Dependency Verification (unchanged)

| Target | Has Build Dep? | Verified? |
|--------|---------------|-----------|
| `crdb:migrate` | No | ⚠️ Add explicit build step |
| `run:cockroach` | Yes (`deps: [build:backend:cockroach]`) | ✅ |

### T2: Go Test Function Existence (unchanged)

| Function | File |
|----------|------|
| `TestCockroachP0` | `store/cockroach_p0_test.go` |
| `TestCockroachMigrateEndToEnd` | `store/test/cockroach_migrate_test.go` |

### T2b: Test Isolation Verification (NEW)

| Check | Verification |
|-------|-------------|
| `TestCockroachMigrateEndToEnd` uses `BCHAT_ALLOW_DB_RESET` | `grep "BCHAT_ALLOW_DB_RESET" store/test/cockroach_migrate_test.go` |
| `resetCockroachDB()` drops all tables | `grep "DROP TABLE IF EXISTS" store/test/cockroach_migrate_test.go` |
| Tests run BEFORE app starts | Phase 2 step 5-6 run before Phase 3 step 4 |
| No conflict with running app | App not started until Phase 3 |

**Critical:** The restructured phase ordering ensures Go tests run BEFORE the app starts. This prevents `resetCockroachDB()` from invalidating the app's database connection.

### T3: SQL Query Validity (unchanged)

All 9 queries verified against CRDB v26.2 syntax.

### T4: HTTP Endpoint Existence (unchanged)

`/healthz` endpoint verified in `server/router/api/v1/v1.go`.

### T5: Prerequisite Chain Trace (restructured)

#### Phase 1 → Phase 2

| Phase 2 Needs | Phase 1 Provides | Verified By |
|---------------|------------------|-------------|
| CockroachDB running | `crdb:reset` starts container | T1 |
| SQL reachable | Phase 1 step 3 (`SELECT 1`) | Gate check |
| `feature.vector_index.enabled = true` | Phase 1 step 2 (cluster setting) | Gate check |
| No stuck schema jobs | Phase 1 gate (SHOW JOBS) | Gate check |

#### Phase 2 → Phase 3

| Phase 3 Needs | Phase 2 Provides | Verified By |
|---------------|------------------|-------------|
| Schema applied | `crdb:migrate` applies LATEST.sql | T1 |
| Tests passed | Go tests verified schema | T2, T2b |
| App binary built | `run:cockroach` has `deps: [build:backend:cockroach]` | T1b |
| App NOT running | Go tests finished, no app process yet | Gate check |

#### Phase 3 → Phase 4

| Phase 4 Needs | Phase 3 Provides | Verified By |
|---------------|------------------|-------------|
| App running on :5230 | Phase 3 step 4 starts app | Gate check (healthz) |
| `MEMOS_DRIVER=cockroach` | Phase 3 prerequisite check | Gate check |
| Vector DB initialized | `Validate()` creates tables | Phase 2 gate |
| PID captured | Phase 3 step 4 captures PID | Variable available |

#### Phase 4 → Phase 5

| Phase 5 Needs | Phase 4 Provides | Verified By |
|---------------|------------------|-------------|
| Test tenant created | Phase 4 step 5 (verify-production.sh) | Script output |
| Embeddings stored | Phase 4 step 6 (`SELECT count(*)`) | Gate check |
| App stopped | Phase 5 step 14 (`kill $BCHAT_PID`) | Cleanup |
| Container stopped | Phase 5 step 14 (`crdb:down`) | Cleanup |

### T6: Gate Criteria Audit (restructured)

| Gate | Check | Measurable? | Notes |
|------|-------|-------------|-------|
| Phase 1 | Container running | ✅ `docker ps` | Standard Docker |
| Phase 1 | Healthcheck passing | ✅ `docker inspect` | Standard Docker |
| Phase 1 | SQL connectivity | ✅ `cockroach sql -e "SELECT 1"` | Standard SQL |
| Phase 1 | `feature.vector_index.enabled` | ✅ `SHOW CLUSTER SETTING` | CRDB syntax |
| Phase 1 | SHOW JOBS no stuck jobs | ✅ `SHOW JOBS` | CRDB syntax |
| Phase 2 | `crdb:migrate` starts | ✅ Process starts | Standard shell |
| Phase 2 | `TestCockroachP0` passes | ✅ `go test` exit code | Standard Go |
| Phase 2 | `TestCockroachMigrateEndToEnd` passes | ✅ `go test` exit code | Standard Go |
| Phase 2 | `crdb:verify` exits 0 | ✅ Exit code | Standard shell |
| Phase 2 | `agent_vectors` table exists | ✅ `SHOW TABLES LIKE` | CRDB syntax |
| Phase 2 | Vector index exists | ✅ `SHOW INDEXES FROM` | CRDB syntax |
| Phase 2 | SHOW JOBS no failed/running | ✅ `SHOW JOBS` | CRDB syntax |
| Phase 3 | `MEMOS_DRIVER` set in `.env` | ✅ `grep` | Standard shell |
| Phase 3 | `/healthz` returns 200 | ✅ `curl -fsS` | Standard HTTP |
| Phase 3 | No unexpected SQLSTATE errors | ✅ Automated grep (T10) | New |
| Phase 3 | Driver = cockroach | ✅ Automated grep (T10) | New |
| Phase 4 | Sign-in succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | KB import succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | RAG reindex succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | RAG search returns results | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | Cleanup succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | `agent_vectors` count > 0 | ✅ `SELECT count(*)` | CRDB syntax |
| Phase 4 | DB vs OpenRouter errors | ✅ Automated grep (T10) | New |
| Phase 5 | `crdb:up` starts | ✅ `docker ps` | Standard Docker |
| Phase 5 | `crdb:verify` passes | ✅ Exit code | Standard shell |
| Phase 5 | `verify-production.sh` passes | ✅ Exit code | Script output |
| Phase 5 | `agent_vectors` count unchanged | ✅ `SELECT count(*)` | CRDB syntax |
| Phase 5 | No duplicate data | ⚠️ Manual comparison | Acceptable |

**Summary:** 27/28 gates directly measurable. 1 gate requires manual comparison — acceptable.

### T7: Failure Mode Walk-Through (unchanged)

All 8 executable fixes verified. 1 code-inspection-only fix acceptable.

### T8: Process Lifecycle Verification (updated)

| Claim | Expected | Actual | Match? |
|-------|----------|--------|--------|
| `crdb:migrate` starts app and blocks | Process binds to port 5230 | Same root command as `run:cockroach` | ✅ Correct |
| `run:cockroach` starts app and blocks | Process binds to port 5230 | Same root command | ✅ Correct |
| `crdb:verify` runs shell commands only | No app process needed | Taskfile cmds are SQL checks | ✅ Correct |
| `verify-production.sh` starts no app | Script uses `curl` only | No Go binary execution | ✅ Correct |

**Key insight:** Only ONE app process can run at a time. The restructured plan ensures this by:
1. Running Go tests in Phase 2 (BEFORE app starts)
2. Starting the app in Phase 3
3. Keeping it running through Phase 4
4. Killing it in Phase 5

### T9: Background Process Control (rewritten)

| Check | Verification |
|-------|-------------|
| `task run:cockroach &` puts app in background | `jobs` shows background job |
| `BCHAT_PID=$!` captures task PID | `ps -p $BCHAT_PID -o comm=` shows `task` |
| `kill $BCHAT_PID` terminates task | `ps -p $BCHAT_PID` returns empty within 5s |
| App terminates with task | `ps aux | grep build/memos` shows no matching process |
| `wait $BCHAT_PID` doesn't hang | Returns within 5s of kill |

**Note:** Signal propagation from `task` to the app is implementation-dependent. `task` is a Go binary that spawns shells, which spawn the app. Go programs don't automatically forward SIGTERM to child processes. Verify that killing the task PID also kills the app. If the app persists, use `kill -9 $BCHAT_PID` or process group kill.

### T9b: Orphaned Process Check (NEW)

After `kill $BCHAT_PID`:
```bash
# Verify no orphaned app processes
ps aux | grep "build/memos" | grep -v grep | wc -l
# Expected: 0
```

### T10: Automated Log Checks (rewritten)

For the 3 gates that previously required manual log inspection:

| Gate | Automated Check |
|------|-----------------|
| No unexpected SQLSTATE errors | `! grep -i "SQLSTATE.*ERROR\|failed to.*SQLSTATE" build/memos.log` |
| Driver = cockroach | `grep -q "driver=cockroach" build/memos.log` (exact match) |
| No DB errors vs OpenRouter errors | `! grep -i "SQLSTATE.*ERROR" build/memos.log && ! grep -qi "OpenRouter.*error\|OpenRouter.*fail" build/memos.log` |

**Why not `grep -qi "SQLSTATE"`:** `vectordb_cockroach.go:112-135` intentionally logs SQLSTATE errors at INFO/WARN level as part of normal operation:
- `42P07` = "Vector index already exists" (expected during concurrent creation)
- `0A000` = "feature not supported" (expected when feature flag is off)

These are **expected, handled errors**. The check should only fail on **unexpected** SQLSTATE errors.

If any check fails, print matching log lines for human review.

### T11: `crdb:init` Behavioral Verification (updated)

| Check | Verification |
|-------|-------------|
| `set -e` is first line | `grep -A 2 "crdb:init:" Taskfile.yml | grep "set -e"` |
| `set -e` causes failure on error | Behavioral: run with invalid DSN; verify task exits non-zero |
| `task: crdb:init` syntax in crdb:reset | `grep "task: crdb:init" Taskfile.yml` (not `task crdb:init`) |
| `task: crdb:init` is after `up -d --wait` | Verify ordering in `crdb:reset` cmds |
| Retry loop exists | `grep "seq 1 30" Taskfile.yml` |
| Retry loop actually retries | Behavioral: run with container not ready; verify retries before failing |

**Note:** Behavioral tests (set -e effectiveness, retry loop behavior) require actual execution. They are validated by running the E2E plan itself, not by static analysis.

### T5b: Cleanup Verification (NEW)

| Check | Verification |
|-------|-------------|
| Cleanup runs even if Phase 4 fails | Add `trap` in plan_e2e.md |
| `kill $BCHAT_PID` terminates app | `ps aux | grep build/memos` returns empty |
| `crdb:down` stops container | `docker ps` shows no `bchat-crdb` |
| Phase 5 can start fresh | `crdb:up` succeeds after cleanup |

**`trap` recommendation for plan_e2e.md:**
```bash
# Add at the start of Phase 3 (after PID capture):
trap "kill $BCHAT_PID 2>/dev/null; task crdb:down 2>/dev/null" EXIT INT TERM
```

This ensures cleanup runs even if the plan is interrupted by Ctrl+C or error.

---

## Adversarial Review Prompt

```
You are reviewing a revised meta-test plan that validates a local E2E testing plan
for a CockroachDB-backed Go application (bchat).

META-TEST PLAN: bugs/058/plan3_e2e.md
E2E PLAN UNDER TEST: bugs/058/plan_e2e.md (v2, revised)
IMPLEMENTATION: 3-file change (vectordb_cockroach.go, crdb-init.sql, Taskfile.yml)

KEY STRUCTURAL CHANGES:
1. Go tests run BEFORE app starts (Phase 2, not Phase 3)
2. Signal propagation verified with orphaned process check
3. Log checks target unexpected errors only (not expected SQLSTATE handling)
4. Cleanup verification with trap recommendation

REVIEW FOR:

1. COMPLETENESS:
   - Does the restructured plan cover ALL commands in plan_e2e.md?
   - Are the new test cases (T2b, T5b, T9b) sufficient?
   - Does the prerequisite chain trace account for the restructured phases?
   - Is the test isolation issue (Finding 4) fully resolved?

2. CORRECTNESS:
   - Does the phase ordering prevent test/app conflicts?
   - Are the automated log checks (T10) correct for expected errors?
   - Is the signal propagation handling (T9) robust?
   - Are the gate criteria updated for the new structure?

3. RISK:
   - Could Go tests still conflict with the running app?
   - Is `kill $BCHAT_PID` sufficient, or do we need `kill -9`?
   - Are there orphaned process risks after cleanup?
   - Does the restructured plan introduce new failure modes?

4. OPERATIONAL:
   - Can the revised plan be executed sequentially without manual intervention?
   - Are all commands copy-pasteable?
   - Is the troubleshooting section updated for the new structure?
   - Is the `trap` recommendation correct for bash cleanup?

EXPECTED OUTPUT:
- For each finding: severity (Critical/High/Medium/Low), whether it's a blocker,
  and suggested fix.
- Overall verdict: APPROVE / APPROVE WITH NITS / REQUEST CHANGES
```
