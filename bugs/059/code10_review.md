# bchat Durable Execution — Adversarial Review of code10.md Implementation (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** Implementation of `code10.md` (fires code9_imp_review.md gates R-F1, R-F2, N-1…N-4) verified against the live tree.
**Method:** Read `code10.md`; independently verified every step against the source (mtimes, exact SQL/param binding, gofmt, parity script), and re-ran the suites. No edits applied — this is the review gate.

---

## 0. Tree Status (verified)

Implementation **is in the tree** (mtimes 11:55–11:57, after code10.md 11:53):

| File | mtime | Step |
|------|-------|------|
| store/db/sqlite/agent_skill.go | 11:55:07 | R-F1 (1) |
| store/db/postgres/agent_skill.go | 11:55:27 | R-F1 (2) |
| store/test/skill_execution_test.go | 11:55:45 | R-F1 (3) |
| store/test/skill_execution_postgres_test.go | 11:28 | unchanged (4, correct) |
| server/router/api/v1/agent/evaluator.go | 11:56:20 | R-F2 (5) |
| server/router/api/v1/agent/execution.go | 11:57:23 | R-F2 (6,7) + N-3 |
| server/router/api/v1/agent/service.go | 11:57:23* | N-1 |
| scripts/validate-parity.sh | 11:21* | N-2 (mtime pre-dates; verified by content) |
| server/router/api/v1/agent/skill_builtins.go | 07:27* | N-4 (comment; verified by content) |

Independent verification runs (this review):
- `go test ./server/router/api/v1/agent/...` → **PASS (12.601s)**
- `go test ./store/test/...` → **PASS (9.585s)** — includes `TestSkillExecutionRoundTrip` with the new same-worker re-claim against real sqlite
- `bash scripts/validate-parity.sh` → **PASS**, Check 2c `postgres=2, crdb=2`
- `bash scripts/validate-migrations.sh` → **PASS** (LATEST in sync)
- `gofmt -l server/router/api/v1/agent/service.go` → **empty (clean)**

---

## 1. Verified correct

### R-F1 — Same-worker carve-out (Steps 1-4)
- **sqlite** (`store/db/sqlite/agent_skill.go`): WHERE `(status = 'pending' OR (status = 'running' AND (claim_expires_at < ? OR claimed_by = ?)))`; ExecContext binds `now, workerID, expiresAt, id, now, workerID` — order matches placeholders exactly.
- **postgres** (`store/db/postgres/agent_skill.go`): same clause with `$4`/`$5`/`$6`; binds `now, workerID, expiresAt, id, now, workerID` — `$1`-`$6` order correct.
- Semantics: same-worker re-claim succeeds and renews the lease; different worker blocked until lease expiry; stopped/completed rows still excluded (status guard untouched).
- **Test (3):** `skill_execution_test.go:78-79` same-worker re-claim added before Case 4 — passes on real sqlite. **Test (4):** postgres test re-claim at :80-82 unchanged and now satisfiable by the driver fix.

### R-F2 — Compile vs runtime separation (Steps 5-7)
- `CompileError` type (evaluator.go:73-83) with `Error()` + `Unwrap()`; returned from `env.Compile` issues (:121) and `env.Program` errors (:126).
- `executeStep` (execution.go:256-263): `errors.As(err, &compileErr)` → hard-fail `condition compile error for %s` (surfaces via K-1 error_message again); all other eval errors → `slog.Warn` incl. **`exec_id`** + node + condition + error, treat as not-met.

### Nits 1-4
- **N-1:** service.go gofmt-clean (verified; note the whole file was reformatted — acceptable, uncommitted).
- **N-2:** Check 2c scoped via `awk '/CREATE TABLE IF NOT EXISTS agent_skill_executions/,/CREATE INDEX IF NOT EXISTS idx_skill_log_execution/'` — cleanly spans both skill tables without the plan's `head -n -1` hack; PASS 2/2.
- **N-3:** `normalizeNumbers` comment corrected (canonicalization wording, no "no int == double" claim).
- **N-4:** `skill_builtins.go:140` output-contract LLM-impact comment present.

---

## 2. Findings (foldable — no behavioral rework required)

### F-1 (MED) — R-F2's new behavior is untested
The compile→hard-fail vs runtime→log-and-skip distinction has **zero tests**: no `CompileError` reference, typo-expression, or `1/0` case exists in `evaluator_test.go` or `execution_test.go`, yet the harness is ready:
- `TestExecuteStep_*` already uses `executeStepHelper` (execution_test.go:136-165 pattern). Add:
  - `Condition: "search_kb.found == treu"` with `search_kb` map in state → expect error (hard fail).
  - `Condition: "1/0"` (cel-go integer division by zero = genuine runtime eval error, not missing-key) → expect no error + empty output (skip path).
- Evaluator-level: `EvalConditionDynamic` with a typo → `errors.As(err, *CompileError)` true.
The code10 spot-checks list these behaviors but the file manifest added no tests.

### F-2 (LOW) — different-worker lease exclusivity untested
The carve-out's negative case never fires in the suite: the sqlite `worker-2` claim (`skill_execution_test.go:91`) runs **after** Stop, so it exercises only status-gating, not unexpired-lease exclusivity. Add, immediately after the same-worker re-claim:
```go
_, err = ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-2", 60)
require.Error(t, err)
```

### F-3 (LOW) — stop-condition compile errors still log-and-skip (inconsistency)
execution.go:217-222 unchanged: a typo in the stop condition is logged and treated as not-met, so the intended stop silently never fires — while node-condition typos now hard-fail. Defensible (Fix 5 was approved as tolerant) but inconsistent with R-F2's rationale. Either apply the same `CompileError` distinction to the stop path (recommended: broken stop condition = graph bug → fail), or record an explicit documented decision.

### INFO — carve-out is currently dormant in production
`runDetachedExecution` claims exactly once per run (execution.go:86) with a per-run uuid workerID (`"worker-" + uuid[:8]`); a crashed-and-restarted run gets a **new** workerID and still waits out the 5-minute lease (checkpoint.go:14, `leaseSeconds = 300`). Same-worker re-claim only fires if a future path claims twice in-process with the same workerID. Strictly additive and harmless, but code10's "lease renewal pattern" framing oversells current production impact.

---

## 3. Conformance

| code9_imp_review gate | Implementation | Disposition |
|------------------------|----------------|-------------|
| R-F1 (same-worker carve-out + tests) | Steps 1-4, both drivers + sqlite re-claim test | FIXED (F-2 adds the exclusivity negative) |
| R-F2 (compile vs runtime + exec_id) | CompileError type, errors.As hard-fail, exec_id warn | FIXED (F-1 adds the missing tests) |
| N-1 (gofmt) | clean | FIXED |
| N-2 (Check 2c scoping) | awk-scoped, PASS | FIXED |
| N-3 (rationale doc) | comment corrected | FIXED |
| N-4 (LLM impact doc) | comment present | FIXED |

---

## 4. Bottom Line / Sign-off

- **Tree:** code10.md implemented; all suites and gates green in this review (agent, store, parity, migrations, gofmt).
- **Core correctness:** R-F1 driver SQL/param binding and R-F2 error-class separation are correct and verified; nits all addressed.
- **Gate condition:** **APPROVED (conditional)** — fold F-1 (add R-F2 tests via the existing `executeStepHelper` harness), F-2 (different-worker exclusivity assertion), and F-3 (stop-condition compile-error treatment: apply the same hard-fail or document the decision). No behavioral rework required; the conditions are test/doc additions.