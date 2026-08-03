# Bug 058 — Meta-Test Plan v5: Validating plan_e2e.md

**Date:** 2026-08-03
**Author:** opencode
**Status:** Revised per adversarial reviews (plan4_test_e2e_review.md, plan4_test_e2e_review_claude.md)
**Depends on:** `plan_e2e.md` (v2), `plan4_e2e.md`

---

## Revision Summary

This plan addresses all findings from both adversarial reviews of `plan4_e2e.md`:

| # | Finding | Severity | Change |
|---|---------|----------|--------|
| 1 | Phase 2 gate timing — `agent_vectors` doesn't exist before reindex | High | Split `crdb:verify` into Phase 2 (P1-P5) and Phase 3 (P6 after reindex) |
| 2 | T10 false-positive guaranteed in Phase 4 (idempotency re-triggers 42P07) | Medium-High | Anchor to `level=(ERROR|FATAL)` or use critical SQLSTATE codes |
| 3 | `trap` fallback not wired in — `pkill` is manual afterthought | Medium | Add `pkill -f build/memos` directly in trap |
| 4 | `trap` placement slightly off — PID not captured when trap is set | Low | Place trap immediately after PID capture |

---

## Critical Finding: Phase 2 Gate Timing

**`service.go:232-258`** — Auto-bootstrap logic:

```go
} else {
    go func() {
        time.Sleep(5 * time.Second)
        if svc.IsRAGEnabled() {
            stats, err := svc.GetVectorDB().Stats(ctx)
            if err == nil && stats.TotalChunks == 0 {
                files, err := s.ListAgentSourceFiles(ctx, ...)
                if err == nil && len(files) > 0 {
                    svc.ReindexAllContent(ctx)  // triggers Validate()
                }
            }
        }
    }()
}
```

On a fresh database with no source files, auto-bootstrap doesn't trigger (`len(files) > 0` fails). `Validate()` never runs. `agent_vectors` table and index don't exist.

**`Taskfile.yml:369-371`** — `crdb:verify` P6 check:

```bash
I=$(run_sql "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';")
echo "$I" | grep -qv "^0" || { echo "FAIL: agent_vectors has no indexes"; exit 1; }
```

This returns 0 on fresh DB. Phase 2's `crdb:verify` fails on a correct, healthy first run.

**Solution:** Split `crdb:verify` into two invocations:
- Phase 2: `crdb:verify` (P1-P5 only — skip `agent_vectors` check)
- Phase 3: Run P6 check manually after reindex

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
    └── Run crdb:verify (P1-P5 only — agent_vectors doesn't exist yet)
    ↓
Phase 3: Data Path (verify-production.sh)
    ├── Reindex triggered (creates agent_vectors + vector index)
    └── P6 check (agent_vectors indexed — now exists)
    ↓
Phase 4: Idempotency (crdb:down → crdb:up → verify)
    ↓
Phase 5: Cleanup & Gate
```

---

## Test Cases

### T1: Taskfile Target Existence (unchanged)

| Target | Plan Step | Expected |
|--------|-----------|----------|
| `crdb:reset` | Phase 1 step 1 | Exists |
| `crdb:init` | Phase 1 internal | Exists (chained from crdb:reset) |
| `run:cockroach` | Phase 2 step 5 | Exists |
| `crdb:verify` | Phase 2 step 7, Phase 4 step 10 | Exists |
| `crdb:down` | Phase 4 step 11, Phase 5 step 16 | Exists |
| `crdb:up` | Phase 4 step 12 | Exists |

### T1b: Build Dependency Verification (unchanged)

| Target | Has Build Dep? | Verified? |
|--------|---------------|-----------|
| `run:cockroach` | Yes (`deps: [build:backend:cockroach]`) | ✅ |

### T2: Go Test Function Existence (unchanged)

| Function | File |
|----------|------|
| `TestCockroachP0` | `store/cockroach_p0_test.go` |
| `TestCockroachMigrateEndToEnd` | `store/test/cockroach_migrate_test.go` |

### T2b: Test Isolation Verification (unchanged)

| Check | Verification |
|-------|-------------|
| `TestCockroachMigrateEndToEnd` uses `BCHAT_ALLOW_DB_RESET` | `grep "BCHAT_ALLOW_DB_RESET" store/test/cockroach_migrate_test.go` |
| `resetCockroachDB()` drops all tables | `grep "DROP TABLE IF EXISTS" store/test/cockroach_migrate_test.go` |
| Tests run BEFORE app starts | Phase 2 steps 2-3 run before step 5 |

### T3: SQL Query Validity (unchanged)

All 9 queries verified against CRDB v26.2 syntax.

### T4: HTTP Endpoint Existence (unchanged)

`/healthz` endpoint verified in `server/router/api/v1/v1.go`.

### T5: Prerequisite Chain Trace (updated)

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
| Schema applied | `crdb:migrate` runs during app start | Phase 2 gate (P1-P5) |
| Tests passed | Go tests verified schema | T2, T2b |
| PID captured | Phase 2 step 5 captures PID | Variable available |
| **`agent_vectors` does NOT exist** | **No reindex triggered yet** | **Expected — P6 deferred to Phase 3** |

#### Phase 3 → Phase 4

| Phase 4 Needs | Phase 3 Provides | Verified By |
|---------------|------------------|-------------|
| `agent_vectors` exists | Reindex triggered by `verify-production.sh` | P6 check |
| Vector index exists | `Validate()` creates index | P6 check |
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

### T6: Gate Criteria Audit (updated)

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
| Phase 2 | `crdb:verify` P1-P5 exits 0 | ✅ Exit code | Standard shell |
| Phase 2 | **P6 (`agent_vectors`) DEFERRED** | ⏳ Phase 3 | **Not checked in Phase 2** |
| Phase 3 | `MEMOS_DRIVER` set in `.env` | ✅ `grep` | Standard shell |
| Phase 3 | `/healthz` returns 200 | ✅ `curl -fsS` | Standard HTTP |
| Phase 3 | `agent_vectors` exists | ✅ `SHOW TABLES LIKE` | CRDB syntax |
| Phase 3 | Vector index exists | ✅ `SHOW INDEXES FROM` | CRDB syntax |
| Phase 3 | SHOW JOBS no failed/running | ✅ `SHOW JOBS` | CRDB syntax |
| Phase 3 | No unexpected SQLSTATE errors | ✅ Automated grep (T10) | Updated |
| Phase 3 | Driver = cockroach | ✅ Automated grep (T10) | Updated |
| Phase 4 | Sign-in succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | KB import succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | RAG reindex succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | RAG search returns results | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | Cleanup succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | `agent_vectors` count > 0 | ✅ `SELECT count(*)` | CRDB syntax |
| Phase 4 | DB vs OpenRouter errors | ✅ Automated grep (T10) | Updated |
| Phase 5 | `crdb:up` starts | ✅ `docker ps` | Standard Docker |
| Phase 5 | `crdb:verify` passes | ✅ Exit code | Standard shell |
| Phase 5 | `verify-production.sh` passes | ✅ Exit code | Script output |
| Phase 5 | `agent_vectors` count unchanged | ✅ `SELECT count(*)` | CRDB syntax |
| Phase 5 | No duplicate data | ⚠️ Manual comparison | Acceptable |

**Summary:** 27/28 gates directly measurable. P6 deferred to Phase 3 (correct timing). 1 gate requires manual comparison — acceptable.

### T7: Failure Mode Walk-Through (unchanged)

### T8: Process Lifecycle Verification (unchanged)

### T9: Background Process Control (updated with trap fix)

| Check | Verification |
|-------|-------------|
| `task run:cockroach &` puts app in background | `jobs` shows background job |
| `BCHAT_PID=$!` captures task PID | `ps -p $BCHAT_PID -o comm=` shows `task` |
| `kill $BCHAT_PID` terminates task | `ps -p $BCHAT_PID` returns empty within 5s |
| App terminates with task | `ps aux | grep build/memos | grep -v grep | wc -l` == 0 |
| `wait $BCHAT_PID` doesn't hang | Returns within 5s of kill |
| Orphan cleanup if needed | `pkill -f "build/memos"` or `kill -9 $BCHAT_PID` | Fallback |

### T9b: Orphaned Process Check (unchanged)

### T10: Automated Log Checks (rewritten — two approaches)

**Approach A: Anchor to log level (recommended)**

| Gate | Automated Check |
|------|-----------------|
| No unexpected SQLSTATE errors | `! grep -iE "level=(ERROR|FATAL).*SQLSTATE|SQLSTATE.*level=(ERROR|FATAL)" build/memos.log` |
| Driver = cockroach | `grep -q "driver=cockroach" build/memos.log` (exact match) |
| No DB errors vs OpenRouter errors | `! grep -iE "level=(ERROR|FATAL).*SQLSTATE|SQLSTATE.*level=(ERROR|FATAL)" build/memos.log && ! grep -qi "level=(ERROR|FATAL).*openrouter\|openrouter.*level=(ERROR|FATAL)" build/memos.log` |

**Approach B: Critical SQLSTATE codes (simpler)**

| Gate | Automated Check |
|------|-----------------|
| No critical SQLSTATE errors | `! grep -iE "SQLSTATE.*(28P01|28P02|3D000|08001|08006|42P02|53300)" build/memos.log` |

**Why the current pattern fails:**
- `grep -i "SQLSTATE" | grep -iE "ERROR|FAIL"` matches pgx `.Error()` format: `ERROR: ... (SQLSTATE 28P01)` — contains both substrings
- Phase 4 re-triggers reindex, hitting 42P07 ("index already exists") — the `pgconn.PgError` object logged at WARN level contains "SQLSTATE" in its string representation
- This is a **guaranteed false-positive** on a successful idempotency run

**Verification needed before execution:** Capture one real log line per case (42P07 handled, 0A000 handled, one genuinely unexpected error) and test the pattern against the literal text.

### T11: `crdb:init` Behavioral Verification (unchanged)

### T5b: Cleanup Verification (updated with trap fix)

| Check | Verification |
|-------|-------------|
| Cleanup runs even if Phase 3 fails | `trap` with `pkill` fallback |
| `kill $BCHAT_PID` terminates app | `ps aux | grep build/memos` returns empty |
| `pkill -f build/memos` catches orphans | Fallback in trap |
| `crdb:down` stops container | `docker ps` shows no `bchat-crdb` |
| Phase 4 can start fresh | `crdb:up` succeeds after cleanup |

**`trap` recommendation for plan_e2e.md (corrected placement):**

```bash
# Phase 2 step 5 — immediately after PID capture:
task run:cockroach &
BCHAT_PID=$!
trap "kill $BCHAT_PID 2>/dev/null; pkill -f 'build/memos' 2>/dev/null; task crdb:down 2>/dev/null" EXIT INT TERM
```

**Why `pkill` is in the trap:** Signal propagation from `task` to the child app is implementation-dependent. `task` is a Go binary that spawns shells, which spawn the app. Go programs don't automatically forward SIGTERM to child processes. If `kill $BCHAT_PID` doesn't terminate the app, `pkill -f 'build/memos'` catches it.

---

## Adversarial Review Prompt

```
You are reviewing a revised meta-test plan that validates a local E2E testing plan
for a CockroachDB-backed Go application (bchat).

META-TEST PLAN: bugs/058/plan5_e2e.md
E2E PLAN UNDER TEST: bugs/058/plan_e2e.md (v2, revised)
IMPLEMENTATION: 3-file change (vectordb_cockroach.go, crdb-init.sql, Taskfile.yml)

KEY STRUCTURAL CHANGES:
1. P6 check (agent_vectors) deferred from Phase 2 to Phase 3 — table doesn't exist before reindex
2. T10 log checks anchored to log level markers (level=ERROR|FATAL) to avoid false-positives
3. trap includes pkill -f build/memos as automatic fallback (not manual afterthought)
4. Phase 2 uses crdb:verify P1-P5 only; Phase 3 runs P6 after reindex

REVIEW FOR:

1. COMPLETENESS:
   - Does the restructured plan cover ALL commands in plan_e2e.md?
   - Is the P6 deferral correct — does agent_vectors really not exist before reindex?
   - Are the T10 patterns correct for the actual log format?
   - Is the trap placement correct (immediately after PID capture)?

2. CORRECTNESS:
   - Does the merged Phase 2+3 structure match the app lifecycle?
   - Are the automated log checks (T10) correct for expected errors?
   - Is the signal propagation handling (T9) robust?
   - Are the gate criteria updated for the new structure?

3. RISK:
   - Could the P6 deferral cause issues if someone runs crdb:verify directly?
   - Is the T10 pattern robust against all log formats?
   - Are there orphaned process risks after cleanup?
   - Does the restructured plan introduce new failure modes?

4. OPERATIONAL:
   - Can the revised plan be executed sequentially without manual intervention?
   - Are all commands copy-pasteable?
   - Is the troubleshooting section updated for the new structure?
   - Is the trap recommendation correct for bash cleanup?

EXPECTED OUTPUT:
- For each finding: severity (Critical/High/Medium/Low), whether it's a blocker,
  and suggested fix.
- Overall verdict: APPROVE / APPROVE WITH NITS / REQUEST CHANGES
```
