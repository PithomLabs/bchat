# Bug 058 — Meta-Test Plan v4: Validating plan_e2e.md

**Date:** 2026-08-03
**Author:** opencode
**Status:** Revised per adversarial reviews (plan3_test_e2e_review.md, plan3_test_e2e_review_claude.md)
**Depends on:** `plan_e2e.md` (v2), `plan3_e2e.md`

---

## Revision Summary

This plan addresses all findings from both adversarial reviews of `plan3_e2e.md`:

| # | Finding | Severity | Change |
|---|---------|----------|--------|
| 1 | Phase 2 ordering contradicts test-isolation — `crdb:migrate` blocks before Go tests | Critical | **Merged Phase 2+3** into single phase: Go tests → build → app startup |
| 1b | Backgrounding `crdb:migrate &` creates port conflict with Phase 3 | Critical | Resolved by merge — no Phase 3 app startup needed |
| 2 | `crdb:migrate` has no build dependency | High | Added explicit `task build:backend:cockroach` step |
| 3 | T9 missing orphaned-process fallback | Medium | Added `pkill -f build/memos` fallback |
| 4 | T10 grep pattern misses ERROR-before-SQLSTATE format | Medium | Adjusted pattern to match both orderings |

---

## Critical Structural Finding

**`bin/memos/main.go:117-133`** — The app **never exits after migration**:

```go
storeInstance.Migrate(ctx)          // line 98 — runs migrations
s, err := server.NewServer(...)     // line 104 — creates HTTP server
s.Start(ctx)                        // line 117 — starts HTTP server
<-ctx.Done()                        // line 133 — blocks until SIGINT/SIGTERM
```

Both `crdb:migrate` and `run:cockroach` run the same binary with the same root command. Both start the HTTP server and block. Backgrounding `crdb:migrate &` and then running `run:cockroach` in Phase 3 creates a port conflict — both bind to port 5230.

**Solution:** Merge Phase 2 and Phase 3 into a single phase. The app starts once and runs through the data path verification.

---

## Restructured Phase Ordering

```
Phase 1: Infrastructure Startup (crdb:reset)
    ↓
Phase 2: Tests + App Startup
    ├── Go tests (BEFORE app starts — prevents resetCockroachDB() conflict)
    ├── Build binary (task build:backend:cockroach)
    ├── Start app in background (run:cockroach &)
    ├── Wait for healthz
    └── Run crdb:verify
    ↓
Phase 3: Data Path (verify-production.sh — app already running)
    ↓
Phase 4: Idempotency (crdb:down → crdb:up → verify)
    ↓
Phase 5: Cleanup & Gate
```

**Key changes:**
1. Go tests run BEFORE app starts (prevents `resetCockroachDB()` conflict)
2. App starts ONCE in Phase 2 and runs through Phase 3
3. No Phase 3 app startup — just healthz check against running app
4. Single PID capture (`BCHAT_PID=$!`) in Phase 2

---

## Test Cases

### T1: Taskfile Target Existence

| Target | Plan Step | Expected |
|--------|-----------|----------|
| `crdb:reset` | Phase 1 step 1 | Exists |
| `crdb:init` | Phase 1 internal | Exists (chained from crdb:reset) |
| `run:cockroach` | Phase 2 step 5 | Exists |
| `crdb:verify` | Phase 2 step 7, Phase 4 step 10 | Exists |
| `crdb:down` | Phase 4 step 11, Phase 5 step 16 | Exists |
| `crdb:up` | Phase 4 step 12 | Exists |

```bash
grep -E "^  (crdb:reset|crdb:init|run:cockroach|crdb:verify|crdb:down|crdb:up):" Taskfile.yml
```

### T1b: Build Dependency Verification

| Target | Has Build Dep? | Verified? |
|--------|---------------|-----------|
| `run:cockroach` | Yes (`deps: [build:backend:cockroach]`) | ✅ |
| `crdb:migrate` | No | ⚠️ Not used in restructured plan |

**Note:** `crdb:migrate` is NOT used in the restructured plan. Phase 2 uses `run:cockroach` which has the build dependency.

### T2: Go Test Function Existence

| Function | File |
|----------|------|
| `TestCockroachP0` | `store/cockroach_p0_test.go` |
| `TestCockroachMigrateEndToEnd` | `store/test/cockroach_migrate_test.go` |

### T2b: Test Isolation Verification

| Check | Verification |
|-------|-------------|
| `TestCockroachMigrateEndToEnd` uses `BCHAT_ALLOW_DB_RESET` | `grep "BCHAT_ALLOW_DB_RESET" store/test/cockroach_migrate_test.go` |
| `resetCockroachDB()` drops all tables | `grep "DROP TABLE IF EXISTS" store/test/cockroach_migrate_test.go` |
| Tests run BEFORE app starts | Phase 2 steps 2-3 run before step 5 |
| No conflict with running app | App not started until step 5 |

### T3: SQL Query Validity

All 9 queries verified against CRDB v26.2 syntax (unchanged from plan3_e2e.md).

### T4: HTTP Endpoint Existence

`/healthz` endpoint verified in `server/router/api/v1/v1.go` (unchanged).

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
| App running on :5230 | Phase 2 step 5 starts app | Gate check (healthz) |
| Schema applied | `Validate()` creates tables | Phase 2 gate |
| Vector DB initialized | Phase 2 steps 6-7 verify | Gate check |
| `MEMOS_DRIVER=cockroach` | Phase 2 prerequisite check | Gate check |
| PID captured | Phase 2 step 5 captures PID | Variable available |

#### Phase 3 → Phase 4

| Phase 4 Needs | Phase 3 Provides | Verified By |
|---------------|------------------|-------------|
| Test tenant created | Phase 3 step 3 (verify-production.sh) | Script output |
| Embeddings stored | Phase 3 step 4 (`SELECT count(*)`) | Gate check |
| App stopped | Phase 4 step 11 (`kill $BCHAT_PID`) | Cleanup |
| Container stopped | Phase 4 step 11 (`crdb:down`) | Cleanup |

#### Phase 4 → Phase 5

| Phase 5 Needs | Phase 4 Provides | Verified By |
|---------------|------------------|-------------|
| Restart verified | Phase 4 steps 10-13 | Gate checks |
| App stopped | Phase 4 step 11 (`kill $BCHAT_PID`) | Cleanup |
| Container stopped | Phase 4 step 11 (`crdb:down`) | Cleanup |

### T6: Gate Criteria Audit (restructured)

| Gate | Check | Measurable? | Notes |
|------|-------|-------------|-------|
| Phase 1 | Container running | ✅ `docker ps` | Standard Docker |
| Phase 1 | Healthcheck passing | ✅ `docker inspect` | Standard Docker |
| Phase 1 | SQL connectivity | ✅ `cockroach sql -e "SELECT 1"` | Standard SQL |
| Phase 1 | `feature.vector_index.enabled` | ✅ `SHOW CLUSTER SETTING` | CRDB syntax |
| Phase 1 | SHOW JOBS no stuck jobs | ✅ `SHOW JOBS` | CRDB syntax |
| Phase 2 | `TestCockroachP0` passes | ✅ `go test` exit code | Standard Go |
| Phase 2 | `TestCockroachMigrateEndToEnd` passes | ✅ `go test` exit code | Standard Go |
| Phase 2 | `run:cockroach` starts | ✅ `curl -fsS` healthz | Standard HTTP |
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

### T8: Process Lifecycle Verification

| Claim | Expected | Actual | Match? |
|-------|----------|--------|--------|
| `run:cockroach` starts HTTP server | Process binds to port 5230 | `main.go:117`: `s.Start(ctx)` | ✅ Correct |
| `run:cockroach` blocks until SIGINT | Process blocks on `<-ctx.Done()` | `main.go:133` | ✅ Correct |
| `crdb:verify` runs shell commands only | No app process needed | Taskfile cmds are SQL checks | ✅ Correct |
| `verify-production.sh` starts no app | Script uses `curl` only | No Go binary execution | ✅ Correct |

**Key insight:** Only ONE app process can run at a time. The restructured plan ensures this by starting the app once in Phase 2 and keeping it running through Phase 3.

### T9: Background Process Control (rewritten with fallback)

| Check | Verification |
|-------|-------------|
| `task run:cockroach &` puts app in background | `jobs` shows background job |
| `BCHAT_PID=$!` captures task PID | `ps -p $BCHAT_PID -o comm=` shows `task` |
| `kill $BCHAT_PID` terminates task | `ps -p $BCHAT_PID` returns empty within 5s |
| App terminates with task | `ps aux | grep build/memos | grep -v grep | wc -l` == 0 |
| `wait $BCHAT_PID` doesn't hang | Returns within 5s of kill |
| Orphan cleanup if needed | `pkill -f "build/memos"` or `kill -9 $BCHAT_PID` | Fallback |

**Note:** Signal propagation from `task` to the app is implementation-dependent. `task` is a Go binary that spawns shells, which spawn the app. Go programs don't automatically forward SIGTERM to child processes. Verify that killing the task PID also kills the app. If the app persists, use `pkill -f "build/memos"` or `kill -9 $BCHAT_PID`.

### T9b: Orphaned Process Check

After `kill $BCHAT_PID`:
```bash
# Verify no orphaned app processes
ps aux | grep "build/memos" | grep -v grep | wc -l
# Expected: 0
```

### T10: Automated Log Checks (rewritten for correct pattern)

| Gate | Automated Check |
|------|-----------------|
| No unexpected SQLSTATE errors | `! grep -i "SQLSTATE" build/memos.log | grep -iE "ERROR|FAIL"` |
| Driver = cockroach | `grep -q "driver=cockroach" build/memos.log` (exact match) |
| No DB errors vs OpenRouter errors | `! grep -i "SQLSTATE" build/memos.log | grep -iE "ERROR|FAIL"` && `! grep -qi "OpenRouter.*error\|OpenRouter.*fail" build/memos.log` |

**Why not `grep -qi "SQLSTATE.*ERROR"`:** Standard PostgreSQL error format is `ERROR: ... (SQLSTATE 28P01)` — "ERROR" appears BEFORE "SQLSTATE". The pattern `SQLSTATE.*ERROR` would miss these.

**Why not `grep -qi "SQLSTATE"` alone:** `vectordb_cockroach.go:112-135` logs expected SQLSTATE errors at INFO/WARN level:
- `42P07` = "Vector index already exists" (expected during concurrent creation)
- `0A000` = "feature not supported" (expected when feature flag is off)

**Solution:** Pipe SQLSTATE lines through a second grep for ERROR/FAIL. This catches unexpected errors while allowing expected INFO/WARN lines to pass.

**Verification needed before execution:** Capture one real log line per case (42P07 handled, 0A000 handled, one genuinely unexpected error) and test the pattern against the literal text.

### T11: `crdb:init` Behavioral Verification (unchanged)

| Check | Verification |
|-------|-------------|
| `set -e` is first line | `grep -A 2 "crdb:init:" Taskfile.yml | grep "set -e"` |
| `set -e` causes failure on error | Behavioral: run with invalid DSN; verify task exits non-zero |
| `task: crdb:init` syntax in crdb:reset | `grep "task: crdb:init" Taskfile.yml` (not `task crdb:init`) |
| `task: crdb:init` is after `up -d --wait` | Verify ordering in `crdb:reset` cmds |
| Retry loop exists | `grep "seq 1 30" Taskfile.yml` |
| Retry loop actually retries | Behavioral: run with container not ready; verify retries before failing |

**Note:** Behavioral tests (set -e effectiveness, retry loop behavior) require actual execution. They are validated by running the E2E plan itself, not by static analysis.

### T5b: Cleanup Verification (unchanged)

| Check | Verification |
|-------|-------------|
| Cleanup runs even if Phase 3 fails | Add `trap` in plan_e2e.md |
| `kill $BCHAT_PID` terminates app | `ps aux | grep build/memos` returns empty |
| `crdb:down` stops container | `docker ps` shows no `bchat-crdb` |
| Phase 4 can start fresh | `crdb:up` succeeds after cleanup |

**`trap` recommendation for plan_e2e.md:**
```bash
# Add at the start of Phase 2 (after PID capture):
trap "kill $BCHAT_PID 2>/dev/null; task crdb:down 2>/dev/null" EXIT INT TERM
```

---

## Adversarial Review Prompt

```
You are reviewing a revised meta-test plan that validates a local E2E testing plan
for a CockroachDB-backed Go application (bchat).

META-TEST PLAN: bugs/058/plan4_e2e.md
E2E PLAN UNDER TEST: bugs/058/plan_e2e.md (v2, revised)
IMPLEMENTATION: 3-file change (vectordb_cockroach.go, crdb-init.sql, Taskfile.yml)

KEY STRUCTURAL CHANGES:
1. Phase 2 and Phase 3 merged — app starts once in Phase 2, runs through Phase 3
2. Go tests run BEFORE app starts (prevents resetCockroachDB() conflict)
3. Single PID capture in Phase 2 — no Phase 3 app startup
4. T10 log checks use pipe pattern to match both ERROR-before-SQLSTATE and SQLSTATE-before-ERROR
5. T9 includes orphaned-process fallback (pkill -f build/memos)

REVIEW FOR:

1. COMPLETENESS:
   - Does the restructured plan cover ALL commands in plan_e2e.md?
   - Are the new test cases (T2b, T5b, T9b) sufficient?
   - Does the prerequisite chain trace account for the merged phases?
   - Is the test isolation issue fully resolved?

2. CORRECTNESS:
   - Does the merged Phase 2+3 structure match the app lifecycle?
   - Are the automated log checks (T10) correct for expected errors?
   - Is the signal propagation handling (T9) robust?
   - Are the gate criteria updated for the new structure?

3. RISK:
   - Could Go tests still conflict with the running app?
   - Is `kill $BCHAT_PID` sufficient, or do we need `pkill -f build/memos`?
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
