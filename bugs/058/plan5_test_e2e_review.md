# Bug 058 — Adversarial Review: plan5_test_e2e.md

**Date:** 2026-08-03  
**Reviewer:** Senior Go / CockroachDB Architect  
**Artifact under review:** `bugs/058/plan5_e2e.md`  
**Plan being validated:** `bugs/058/plan_e2e.md` (v2)  
**Implementation:** 3-file change (vectordb_cockroach.go, crdb-init.sql, Taskfile.yml)

---

## Executive Summary

This revision fixes the Critical Phase 2 gate-timing issue from the previous review. The P6 deferral is correct: `agent_vectors` does not exist until reindex runs, so checking for it in Phase 2 is a guaranteed false-negative. Splitting `crdb:verify` into P1-P5 (Phase 2) and P6 (Phase 3) is the right fix.

The remaining issues are nits. The plan is executable.

**Verdict:** APPROVE WITH NITS — 2 nits only. No blockers.

---

## Approved As-Is

### Critical Finding Resolved

| Finding | Status |
|---------|--------|
| Phase 2 gate timing — `agent_vectors` doesn't exist before reindex | ✅ Fixed — P6 deferred to Phase 3 |

The plan correctly identifies that on a fresh database with no source files, auto-bootstrap doesn't trigger (`len(files) > 0` fails in `service.go:232-258`), so `Validate()` never runs and `agent_vectors` isn't created. Running P6 in Phase 2 would fail on a correct, healthy first run. Deferring P6 to Phase 3 (after `verify-production.sh` triggers reindex) is correct.

### All Previous Findings Resolved

| Finding | Status |
|---------|--------|
| Phase 2 ordering contradicts test-isolation | ✅ Fixed — Go tests before `run:cockroach &` |
| `crdb:migrate` blocks before tests run | ✅ Fixed — `crdb:migrate` removed; `run:cockroach` used |
| `crdb:migrate` has no build dep | ✅ Fixed — `run:cockroach` has `deps: [build:backend:cockroach]` |
| T9 signal propagation misleading | ✅ Fixed — orphaned process check + `pkill` fallback |
| T10 log checks false-positive on expected errors | ✅ Fixed — anchored to `level=(ERROR|FATAL)` or critical codes |
| T11 verifies patterns not behavior | ✅ Fixed — behavioral tests added |
| T5 cleanup gap | ✅ Fixed — `trap` with `pkill` fallback, correct placement |

---

## Nits (Not Blockers)

### Nit 1 — T10 Log Format Assumption Unverified

**Section:** T10  
**Severity:** Low  
**Type:** Pattern robustness

The plan offers two approaches:

**Approach A:**
```
! grep -iE "level=(ERROR|FATAL).*SQLSTATE|SQLSTATE.*level=(ERROR|FATAL)" build/memos.log
```

**Approach B:**
```
! grep -iE "SQLSTATE.*(28P01|28P02|3D000|08001|08006|42P02|53300)" build/memos.log
```

Approach A assumes the log format contains `level=ERROR` or `level=FATAL`. This is true for JSON logs, but the application uses Go's default `slog` text handler, which outputs:

```
2026/08/03 12:34:56 ERROR failed to create db driver error="&pgconn.PgError{...Code:\"28P01\"...}"
```

The level appears as `ERROR` without a `level=` prefix. The pattern `level=(ERROR|FATAL)` would **not match** this format.

**Fix:** Before execution, capture one real log line per case and test the pattern. If the app uses text handler (not JSON), use:

```markdown
| No unexpected SQLSTATE errors | `! grep -i "SQLSTATE" build/memos.log | grep -iE " ERROR | FATAL "` |
```

Or use Approach B (critical SQLSTATE codes), which is format-agnostic and actually more precise — it catches only real failures (auth, connection, schema corruption) rather than trying to parse log levels.

**Recommendation:** Use Approach B. It's simpler, format-agnostic, and more targeted.

---

### Nit 2 — Phase 3 P6 Check Not Detailed

**Section:** Phase 3, T6  
**Severity:** Low  
**Type:** Execution detail

The plan says:

```
Phase 3: Data Path (verify-production.sh)
    ├── Reindex triggered (creates agent_vectors + vector index)
    └── P6 check (agent_vectors indexed — now exists)
```

But it doesn't show the actual P6 check command. The Phase 2 `crdb:verify` runs P1-P5 only, but there's no explicit "run P6 check" step in Phase 3.

**Fix:** Add to Phase 3 steps:

```markdown
```bash
# After verify-production.sh completes (reindex triggered)
cockroach sql --url "postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
  -e "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';"
```

**Expected:** > 0 (vector index exists)
```

Or run the full `crdb:verify` again in Phase 3 (it will pass P6 now that `agent_vectors` exists).

---

## What Makes This Plan Viable

1. **P6 deferral is correct** — `agent_vectors` doesn't exist until reindex runs, so checking for it before reindex is a guaranteed false-negative
2. **Phase 2 structure is correct** — tests before app, single app start, PID captured
3. **T10 addresses false-positives** — both approaches avoid matching expected SQLSTATE handling
4. **T5b trap is correct** — `pkill` fallback included, placement after PID capture
5. **T9 signal handling is robust** — orphaned process check included

---

## Final Verdict

**APPROVE WITH NITS**

Execute the plan. Fix the two nits during execution if they bite you:
1. Verify T10 pattern against actual log output (or use Approach B)
2. Add explicit P6 check command in Phase 3

