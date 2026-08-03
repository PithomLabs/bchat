# Bug 058 — Meta-Test Plan v2: Validating plan_e2e.md

**Date:** 2026-08-03
**Author:** opencode
**Status:** Revised per adversarial review (plan_test_e2e_review.md)
**Depends on:** `plan_e2e.md` (v2), `plan_test_e2e.md`

---

## Revision Summary

This plan addresses all 5 findings from the adversarial review of `plan_test_e2e.md`:

| # | Finding | Severity | Change |
|---|---------|----------|--------|
| 1 | App never exits after migration — blocks on HTTP server | Critical | Restructured: Phase 2 and Phase 3 merged (Option A) |
| 2 | `crdb:migrate` has no build dependency | High | Added explicit build step |
| 3 | Phase 3 can't start if Phase 2 blocks | High | Resolved by Finding 1 fix |
| 4 | `run:cockroach` background PID capture unverified | Medium | Added T9 (Process Lifecycle) |
| 5 | 3 gates require manual log inspection | Medium | Added T10 (Automated Log Checks) |

---

## Critical Structural Finding

**`bin/memos/main.go:117-133`** — The app **never exits after migration**:

```go
// main.go lines 98-133
storeInstance.Migrate(ctx)          // line 98 — runs migrations
s, err := server.NewServer(...)     // line 104 — creates HTTP server
s.Start(ctx)                        // line 117 — starts HTTP server
<-ctx.Done()                        // line 133 — blocks until SIGINT/SIGTERM
```

Both `crdb:migrate` and `run:cockroach` run the same binary with the same root command. Both start the HTTP server and block. This means:
- `task crdb:migrate` does NOT exit after migration
- Phase 3 cannot start a second instance while Phase 2 is running
- The only difference between the two tasks is: `crdb:migrate` passes `--driver=cockroach` via CLI flag, `run:cockroach` reads from `.env`

---

## Restructured Test Cases

### T1: Taskfile Target Existence (unchanged)

| Target | Plan Step | Expected |
|--------|-----------|----------|
| `crdb:reset` | Phase 1 step 1 | Exists |
| `crdb:init` | Phase 1 internal | Exists (chained from crdb:reset) |
| `run:cockroach` | Phase 2 step 4 | Exists |
| `crdb:verify` | Phase 2 step 7, Phase 4 step 12 | Exists |
| `crdb:down` | Phase 4 step 15, Phase 5 step 19 | Exists |
| `crdb:up` | Phase 4 step 16 | Exists |

```bash
grep -E "^  (crdb:reset|crdb:init|run:cockroach|crdb:verify|crdb:down|crdb:up):" Taskfile.yml
```

### T1b: Build Dependency Verification

| Target | Has Build Dep? | Verified? |
|--------|---------------|-----------|
| `run:cockroach` | Yes (`deps: [build:backend:cockroach]`) | ✅ |
| `crdb:verify` | No (runs shell commands) | ✅ N/A |

```bash
grep -A 3 "run:cockroach:" Taskfile.yml | grep "deps:"
```

### T2: Go Test Function Existence (unchanged)

| Function | File |
|----------|------|
| `TestCockroachP0` | `store/cockroach_p0_test.go` |
| `TestCockroachMigrateEndToEnd` | `store/test/cockroach_migrate_test.go` |

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
| Binary built | `run:cockroach` has `deps: [build:backend:cockroach]` | T1b |

#### Phase 2 → Phase 3 (Merged — same process)

| Phase 3 Needs | Phase 2 Provides | Verified By |
|---------------|------------------|-------------|
| App running on :5230 | Phase 2 step 4 starts app | Gate check (healthz) |
| Schema applied | `Validate()` creates tables | Phase 2 gate |
| Vector DB initialized | Phase 2 steps 5-6 verify | Gate check |
| `MEMOS_DRIVER=cockroach` | Phase 2 prerequisite check | Gate check |

#### Phase 3 → Phase 4

| Phase 4 Needs | Phase 3 Provides | Verified By |
|---------------|------------------|-------------|
| App still running | Phase 3 didn't kill it | Gate check |
| PID captured | Phase 2 step 4 captures PID | Variable available |

#### Phase 4 → Phase 5

| Phase 5 Needs | Phase 4 Provides | Verified By |
|---------------|------------------|-------------|
| Restart verified | Phase 4 steps 12-14 | Gate checks |
| App stopped | Phase 4 step 15 (`kill $BCHAT_PID`) | Cleanup |
| Container stopped | Phase 4 step 15 (`crdb:down`) | Cleanup |

### T6: Gate Criteria Audit (restructured)

| Gate | Check | Measurable? | Notes |
|------|-------|-------------|-------|
| Phase 1 | Container running | ✅ `docker ps` | Standard Docker |
| Phase 1 | Healthcheck passing | ✅ `docker inspect` | Standard Docker |
| Phase 1 | SQL connectivity | ✅ `cockroach sql -e "SELECT 1"` | Standard SQL |
| Phase 1 | `feature.vector_index.enabled` | ✅ `SHOW CLUSTER SETTING` | CRDB syntax |
| Phase 1 | SHOW JOBS no stuck jobs | ✅ `SHOW JOBS` | CRDB syntax |
| Phase 2 | `run:cockroach` starts | ✅ `curl -fsS` healthz | Standard HTTP |
| Phase 2 | `TestCockroachP0` passes | ✅ `go test` exit code | Standard Go |
| Phase 2 | `TestCockroachMigrateEndToEnd` passes | ✅ `go test` exit code | Standard Go |
| Phase 2 | `crdb:verify` exits 0 | ✅ Exit code | Standard shell |
| Phase 2 | `agent_vectors` table exists | ✅ `SHOW TABLES LIKE` | CRDB syntax |
| Phase 2 | Vector index exists | ✅ `SHOW INDEXES FROM` | CRDB syntax |
| Phase 2 | SHOW JOBS no failed/running | ✅ `SHOW JOBS` | CRDB syntax |
| Phase 3 | `MEMOS_DRIVER` set in `.env` | ✅ `grep` | Standard shell |
| Phase 3 | `/healthz` returns 200 | ✅ `curl -fsS` | Standard HTTP |
| Phase 3 | No SQLSTATE errors | ✅ Automated grep (T10) | New |
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

**Summary:** 27/28 gates directly measurable (up from 28/31). 1 gate requires manual comparison — acceptable.

### T7: Failure Mode Walk-Through (unchanged)

All 8 executable fixes verified. 1 code-inspection-only fix acceptable.

### T8: Process Lifecycle Verification (NEW)

| Claim | Expected | Actual | Match? |
|-------|----------|--------|--------|
| `run:cockroach` starts HTTP server | Process binds to port 5230 | Same root command as `crdb:migrate` | ✅ Correct |
| `run:cockroach` blocks until SIGINT | Process blocks on `<-ctx.Done()` | `main.go:133` | ✅ Correct |
| `crdb:verify` runs shell commands only | No app process needed | Taskfile cmds are SQL checks | ✅ Correct |
| `verify-production.sh` starts no app | Script uses `curl` only | No Go binary execution | ✅ Correct |

**Key insight:** Only ONE app process can run at a time. The restructured plan ensures this by starting the app in Phase 2 and keeping it running through Phase 4.

### T9: Background Process Control (NEW)

| Check | Verification |
|-------|-------------|
| `task run:cockroach &` puts app in background | `jobs` shows background job |
| `BCHAT_PID=$!` captures correct PID | `ps -p $BCHAT_PID` shows task or app |
| `kill $BCHAT_PID` terminates app | `ps -p $BCHAT_PID` returns empty |
| `wait $BCHAT_PID` doesn't hang | Returns within 5s of kill |

**Note:** `$!` captures the shell PID spawned by `task`. When `kill` sends SIGTERM, the shell exits and the app (child process) receives SIGTERM too. If the app doesn't terminate, `kill -9 $BCHAT_PID` forces it.

### T10: Automated Log Checks (NEW)

For the 3 gates that previously required manual log inspection:

| Gate | Automated Check |
|------|-----------------|
| No SQLSTATE errors in logs | `! grep -qi "SQLSTATE" build/memos.log` |
| Driver = cockroach | `grep -q "driver.*cockroach\|Driver.*cockroach" build/memos.log` |
| DB vs OpenRouter errors | `grep -c "OpenRouter\|openrouter" build/memos.log` and `grep -c "SQLSTATE" build/memos.log` |

If any check fails, print matching log lines for human review.

### T11: `crdb:init` Behavioral Verification (NEW)

| Check | Verification |
|-------|-------------|
| `set -e` is first line | `grep -A 2 "crdb:init:" Taskfile.yml | grep "set -e"` |
| `task: crdb:init` syntax in crdb:reset | `grep "task: crdb:init" Taskfile.yml` (not `task crdb:init`) |
| Retry loop exists | `grep "seq 1 30" Taskfile.yml` |

---

## Adversarial Review Prompt

```
You are reviewing a revised meta-test plan that validates a local E2E testing plan
for a CockroachDB-backed Go application (bchat).

META-TEST PLAN: bugs/058/plan2_e2e.md
E2E PLAN UNDER TEST: bugs/058/plan_e2e.md (v2)
IMPLEMENTATION: 3-file change (vectordb_cockroach.go, crdb-init.sql, Taskfile.yml)

KEY STRUCTURAL CHANGE: Phase 2 and Phase 3 are now merged — the app starts
in Phase 2 and runs through Phase 4. Only ONE app process at a time.

REVIEW FOR:

1. COMPLETENESS:
   - Does the restructured plan cover ALL commands in plan_e2e.md?
   - Are the new test cases (T8-T11) sufficient?
   - Does the prerequisite chain trace account for the merged phases?
   - Is the environment audit complete?

2. CORRECTNESS:
   - Does the merged Phase 2→Phase 3 structure match the app lifecycle?
   - Are the automated log checks (T10) reliable?
   - Is the PID capture/cleanup (T9) correct for Taskfile subprocesses?
   - Are the gate criteria updated for the new structure?

3. RISK:
   - Could the app still be running when Phase 4 tries to start?
   - Is `kill $BCHAT_PID` sufficient, or do we need `kill -9`?
   - Are there orphaned process risks?
   - Does the merged structure introduce new failure modes?

4. OPERATIONAL:
   - Can the revised plan be executed sequentially without manual intervention?
   - Are all commands copy-pasteable?
   - Is the troubleshooting section updated for the new structure?

EXPECTED OUTPUT:
- For each finding: severity (Critical/High/Medium/Low), whether it's a blocker,
  and suggested fix.
- Overall verdict: APPROVE / APPROVE WITH NITS / REQUEST CHANGES
```
