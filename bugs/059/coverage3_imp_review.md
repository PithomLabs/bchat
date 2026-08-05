# Adversarial Review of coverage3.md Implementation — bugs/059 Durable Skill Execution

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-06
**Scope:** Implementation of `coverage3.md` per `coverage3_review.md` gates (R-1..R-7) verified against the live tree.
**Method:** Read-only verification of every claimed change (exact lines, driver write semantics, test harnesses, env loading); independent review of the new retry/whole-run-deadline paths. No edits applied — this is the review gate. Compilation was verified statically against real signatures (suites not re-run this session).

---

## 0. Tree Status (verified)

Implementation is in the tree (mtimes 03:59-04:03, after coverage3.md):

| File | mtime | Change |
|------|-------|--------|
| server/router/api/v1/agent/execution.go | 03:59:36 | C1 (per-step + whole-run), C3, C4 (retry), C5, isPermanentError |
| server/router/api/v1/agent/service.go | 03:59:53 | C2 single LLM call |
| server/router/api/v1/agent/evaluator_test.go | 04:00:26 | T1 CompileError + RuntimeError |
| store/test/skill_execution_test.go | 04:00:44 | T2 different-worker exclusivity |
| server/router/api/v1/agent/execution_test.go | 04:02:16 | T3 timeout, T4 DAG-builtin integration |
| bugs/059/coverage_corrected.md | 04:03:07 | D1-D5 corrections |

No source file outside the coverage3 scope was touched this round (store drivers unchanged at 11:55).

---

## 1. Verdict

**APPROVED (conditional).** Every coverage3_review rework gate (R-1..R-5, R-7) is implemented and verified in the tree; T4 was re-authored honestly against a real store, and the mock-LLM E2E row was correctly promoted into the deferred table. **One required condition before merge: F-1** — a whole-run deadline failure leaves the execution stuck in `running` because the retry *and* the fail writes use the already-expired `ctx`, producing an unbounded recovery re-claim loop where `RetryCount` never persists. F-2..F-7 fold as nits; none change the engine's main-path correctness.

---

## 2. Gate Disposition (coverage3_review → tree)

| Gate | Required | Verified disposition |
|------|----------|----------------------|
| R-1 (CRITICAL) checkpoint preservation in re-queue | Single data-preserving write | ✓ **Fixed** — execution.go:137-146: one `UpdateSkillExecution` with `Status:"pending"`, claim fields nil, `ErrorMessage=""`, all maps/trigger_path intact on the local `exec` |
| R-2 (HIGH) T4 re-author (compile + real store + LLM) | Receiver, store stub, LLM step | △ **Fixed/relabeled** — `TestExecuteWorkflow_DAGBuiltin` (execution_test.go:283-334): real `teststore.NewTestingStore`, `svc := &Service{store: ts, skillRegistry: registry}` (compiles — no import cycle: store/test imports store, not agent), asserts DB status `completed` + 2 nodes. **No LLM step** — honest relabel; mock-LLM E2E moved to deferred (MED) per the review's allowed option |
| R-3 (MED-HIGH) whole-run budget | Above lease + env-configurable or renewal | ✓ Default 15min > 300s lease; `SKILL_WHOLE_RUN_TIMEOUT` env; positive-only validation; terminal re-read uses `context.Background()` (execution.go:112) |
| R-4 (MED) retryability classification | Transient vs permanent | ✓ `isPermanentError` (execution.go:502-514): CompileError permanent; DeadlineExceeded transient; handler-not-found / deserialize-graph permanent |
| R-5 (LOW) `1 / 0` constant-fold | Runtime divisor | ✓ `1 / x`, node `x` declared DynType, bound to `int64(0)` at eval → genuine runtime div error, not CompileError (evaluator_test.go:273-286) |
| R-6 (LOW) per-node max_retries | Wire or defer explicitly | △ Wire not done; **deferred-table row missed** (no `max_retries` row in coverage_corrected.md deferred list) |
| R-7 (LOW) projected arithmetic | Method-stated numbers | ✓ Corrective bottom-line ("~12-27% depending on §9 inclusion") is consistent |

---

## 3. Verified Correct (change-by-change)

- **C1 per-step timeout** (execution.go:333-350): `time.ParseDuration` → warn+ignore on parse error → clamp `>280s`→`280s` → `WithTimeout` scoped to `h.Execute` only; exit path flows through the existing `executeStep → executeWorkflow → runDetachedExecution → failExecution` chain ✓. Comment correctly refuses a default timeout (no-regression stance) ✓.
- **C1 whole-run deadline** (execution.go:71-79): default 15 min > 300s lease; env `SKILL_WHOLE_RUN_TIMEOUT` with `d > 0` guard; cancelled ctx does not poison the terminal re-read (`context.Background()` at :112) ✓.
- **C2 single LLM call** (service.go:3081-3108): tools computed once; `len(tools)>0` → `toolCallingLoop` (returns first no-tool response), else plain call with `len(resp.Choices)==0` check; both branches typecheck; the discarded first call is gone ✓.
- **C3 EmitEvent on stop** (execution.go:265-272): dispatch only when `EmitEvent != "" && exec.TenantID != nil`; `dispatchEvent` verified no-op without webhooks (service.go:5601-5612) ✓.
- **C5 stop-condition compile hard-fail** (execution.go:250-254): `errors.As(*CompileError)` → return error; runtime errors still log-and-skip — consistent with node-condition handling (:301-309) ✓.
- **C4 retry** (execution.go:122-151): stop/completed terminal guard retained; permanent-error → immediate fail; else `RetryCount++`, `RetryCount < MaxRetries` → data-preserving requeue with claim release; `ErrorMessage` cleared on requeue ✓.
- **T1** CompileError via `treu` (evaluator_test.go:257-270) ✓; **T2** one-line append (store/test:82-84) ✓; **T3** `sleep(30)` + `Timeout:"1s"` through `executeStepHelper` (execution_test.go:247-268) ✓; **T4** DAG + store integration (execution_test.go:283-334) ✓.
- **D1-D5** coverage_corrected.md: UNCOMMITTED banner, cadence citation, 83% state machine, 65/76 headline, lease reframe, deferred table with mock-LLM E2E row ✓.

---

## 4. Findings

### F-1 (MED-HIGH) — REQUIRED BEFORE MERGE: dead-`ctx` writes on whole-run timeout → unbounded recovery re-claim loop

**Trigger path:** whole-run budget fires (default 15 min, or `SKILL_WHOLE_RUN_TIMEOUT`) → `executeWorkflow` returns `execution cancelled: context deadline exceeded` → terminal re-read (background ctx) sees `running` → `isPermanentError` correctly classifies deadline as **transient** → retry branch:

- `UpdateSkillExecution(ctx, exec)` (execution.go:143) — `ctx` is the **already-expired** whole-run ctx → write fails ("failed to re-queue execution").
- `failExecution(ctx, exec, ...)` (:145) — same dead ctx → also fails.

Net effect: the row remains `status='running'` with an **expired lease**; `RetryCount` was never persisted (the bump write failed). The recovery worker (30s poll, `claim_expires_at < now`) re-claims the row, `runDetachedExecution` runs again with a fresh ctx, and 15 minutes later the same thing happens — **an unbounded loop where `max_retries` never binds**. This is exactly the class of bug the R-1 review round was designed to prevent.

**Fix (one line plus const):** use `context.Background()` (or a short fixed-timeout ctx, e.g. `context.WithTimeout(context.Background(), 10*time.Second)`) for the requeue and the two `failExecution` calls in the error path. All other writes on this path are already on live ctxs.

### F-2 (LOW-MED) — parseable non-positive timeouts kill the node

`time.ParseDuration("-1m")` and `"0s"` parse cleanly (err nil); there is no `d <= 0` guard at execution.go:341-345, so `context.WithTimeout(ctx, -1m)` is immediately expired — every execution of that node hard-fails with `deadline exceeded`, and per F-1's transient classification, gets retried until `max_retries`. Guard `d <= 0 → warn + ignore` (treat as unset).

### F-3 (LOW) — retry-count semantics off-by-one

`RetryCount++` then `if exec.RetryCount < exec.MaxRetries` ⇒ default `max_retries: 3` = 1 initial attempt + 2 retries = **3 total attempts**, not "3 retries". Defensible, but the plan's "bounded by max_retries (default 3)" phrasing and the coverage_corrected.md "Retry honoring max_retries | 100%" should state the semantics (total attempts = max_retries).

### F-4 (LOW) — `isPermanentError` string-match fragility

`strings.Contains(msg, "handler not found")` can misfire if a handler's own error text contains the phrase; CEL runtime type-mismatch errors (non-compile) are treated as retryable and will re-run up to `max_retries` on a permanently-broken expression. Acceptable for v1; worth a comment.

### F-5 (LOW) — T3's deadline assertion is skipped in verbose mode

`if !testing.Verbose() { ... }` (execution_test.go:262) inverts intent — `go test -v` (the CI habit) skips the `"deadline"` content check. Also, hand-rolled `contains`/`containsSubstr` replaces `strings.Contains`. Harmless, but the test is weaker than intended where it matters most.

### F-6 (LOW) — §10 citation persists in coverage_corrected.md:63

"plan6 §10 declares `created → pending → running`" — the state machine is plan6 **§6**; §10 is the Recovery Worker. Same citation error flagged in coverage3_review N-1, re-introduced in the corrected doc.

### F-7 (INFO) — `SKILL_WHOLE_RUN_TIMEOUT` undocumented

New env surface with no entry in docs/DOCS_ENV_VAR.md or README; coverage.md's "only SKILL_RECOVERY_ENABLED added" claim is now stale in that file (coverage_corrected.md does not correct it).

---

## 5. Test Coverage Matrix (this round)

| Test | File:line | Guards |
|------|-----------|--------|
| TestEvalConditionDynamic_CompileError | evaluator_test.go:257 | F-1 gate (CompileError typing) |
| TestEvalConditionDynamic_RuntimeError | evaluator_test.go:273 | CompileError vs runtime separation |
| T2 worker-2 exclusivity | skill_execution_test.go:82-84 | Lease exclusivity negative case |
| TestExecuteStep_Timeout | execution_test.go:247 | C1 per-step timeout |
| TestExecuteWorkflow_DAGBuiltin | execution_test.go:283 | DAG traversal + real store round-trip |

Not covered by any new test: C4 retry semantics (requeue preserving checkpoint, RetryCount persistence), C5 stop hard-fail path, whole-run deadline behavior, C2 single-call refactor (needs mock LLM), C3 EmitEvent dispatch. The retry path in particular deserves a table test once F-1 is fixed — it now has two failure branches that silently degrade.

---

## 6. Bottom Line

The implementation lands every coverage3_review gate that was blocked (R-1 checkpoint preservation is genuinely fixed; T4 is honest and compiles; the whole-run budget and retryability classifier are correct in design), and the remaining open items are a well-scoped deferred list. **Sign-off is conditional on F-1** (dead-ctx error-path writes — one line), with F-2..F-6 as recommended nits and F-7 as documentation. Once F-1 is applied, this round is **APPROVED** and the execution engine is complete and green on its main paths.
