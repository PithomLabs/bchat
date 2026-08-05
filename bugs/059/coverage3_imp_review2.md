# Adversarial Review of F-1..F-7 Fix Round — bugs/059 Durable Skill Execution

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-06
**Scope:** Verification of the fix round addressing `coverage3_imp_review.md` findings F-1..F-7 (implementation of `coverage3.md` per `coverage3_review.md`), plus independent re-review of the retry/whole-run-deadline paths.
**Method:** Read-only verification against the live tree (exact lines, driver write semantics, recovery/claim interactions, test harnesses, env documentation). No edits applied — this is the review gate. Compilation verified statically against real signatures.

---

## 0. Tree Status (verified)

Fix round landed at 04:19 (execution.go, execution_test.go) with support changes:

| File | mtime | Change |
|------|-------|--------|
| server/router/api/v1/agent/execution.go | Aug 6 04:19 | F-1 errCtx, F-2 d<=0 guard |
| server/router/api/v1/agent/execution_test.go | Aug 6 04:19 | F-5 T3 rewrite |
| bugs/059/coverage_corrected.md | Aug 6 04:19 | F-6 §6 citation fix |
| docs/DOCS_ENV_VAR.MD | modified | F-7 SKILL_WHOLE_RUN_TIMEOUT documented |
| bugs/059/coverage3_imp_review.md | Aug 6 04:12 | prior review (this round's mandate) |

No other source files touched this round.

---

## 1. Verdict

**APPROVED (conditional).** Every fix item F-1..F-7 from `coverage3_imp_review.md` is implemented and verified in the tree. **One new condition before merge: N-1 (MED)** — retry requeue dead-ends for `trigger_path='chat'` executions because `ListPendingSkillExecutions` excludes them; `max_retries` is silently ignored for the documented `trigger: "chat"` API path. N-2..N-4 fold as nits; the carried test gap (retry/`isPermanentError` coverage) remains open.

---

## 2. Fix Disposition (coverage3_imp_review → tree)

| Item | Required | Verified disposition |
|------|----------|----------------------|
| F-1 (MED-HIGH) dead-`ctx` error-path writes | Write with non-expired ctx | ✓ **Fixed** — execution.go:126-152: `errCtx := context.Background()`; requeue write (:146) and all three `failExecution` calls (:131 permanent, :148 requeue-failure, :152 max-retries-exceeded) use it; explanatory comment at :126-127 |
| F-2 (LOW-MED) parseable non-positive timeouts | `d <= 0` guard | ✓ **Fixed** — execution.go:344-347: warn + ignore before the 280s clamp |
| F-3 (LOW) retry off-by-one semantics | Document total-attempts | △ Accepted as designed; `RetryCount++` then `< MaxRetries` unchanged (max_retries=3 ⇒ 1 initial + 2 retries); still undocumented in coverage_corrected.md |
| F-4 (LOW) string-match classifier fragility | Comment or harden | △ Accepted — engine's own errors (`handler not found: %s` :333, `deserialize graph: %v` :104) match exactly; residual risk is a handler error containing the phrase; no comment added |
| F-5 (LOW) T3 verbose-mode skip | Remove skip, use strings.Contains | ✓ **Fixed** — execution_test.go:248-266: no `testing.Verbose()` gate; `strings.Contains(err.Error(), "deadline")` |
| F-6 (LOW) §10 citation | Cite §6 | ✓ **Fixed** — coverage_corrected.md:63: "plan6 §6 declares `created → pending → running`" |
| F-7 (INFO) env var undocumented | DOCS_ENV_VAR.MD entry | ✓ **Fixed** — docs/DOCS_ENV_VAR.MD:924-948: SKILL_RECOVERY_ENABLED + SKILL_WHOLE_RUN_TIMEOUT, type/default/example, positive-duration guidance |

---

## 3. Retry/Deadline Path Re-verification (independent)

- Whole-run deadline fires → `executeWorkflow` returns `execution cancelled: %w ctx.Err()` (execution.go:189) → `errors.Is` resolves `context.DeadlineExceeded` through the `%w` wrap → `isPermanentError` correctly classifies **transient** → `RetryCount++` (in-memory) → requeue write persists counter via all-columns `UpdateSkillExecution` (store/db/sqlite/agent_skill.go:104-135) ✓.
- Requeue sets `Status="pending"`, clears claim fields, keeps all maps and `trigger_path` — R-1 checkpoint-preservation contract intact ✓.
- Re-claim: `ClaimSkillExecution` (:159-183) allows `status='pending'` unconditionally and `running` with expired lease; recovery worker (30s ticker) re-runs via `runDetachedExecution` ✓. Loop is bounded: `max_retries` total attempts, counter persists across attempts ✓.
- Terminal re-read at :112 uses `context.Background()` — stop/write race detection unaffected by the fix ✓.
- `errStopSignal` branch (:117-120) returns before the retry block, preserving the already-written `stopped` status ✓.

---

## 4. New Findings

### N-1 (MED) — REQUIRED BEFORE MERGE: retry requeue dead-ends for chat-triggered executions

**Trigger path:** `POST /workflow/start` accepts `trigger: "chat"` (handlers.go:6725, passed at :6770) → `StartDetachedExecution` runs it. On a transient failure (e.g., the 15-min whole-run deadline, or any non-permanent error), the retry branch requeues the row to `status='pending'` **keeping `trigger_path='chat'`** (execution.go:141-150). But `ListPendingSkillExecutions` filters `AND trigger_path != 'chat'` (store/db/sqlite/agent_skill.go:147; same guard in the Postgres driver). Result: **the row is `pending`, claimable only by the recovery worker, and the recovery worker never lists it** — the execution is stuck in `pending` forever, `RetryCount` frozen at 1, `max_retries` silently ignored. (Pre-F-1-fix behavior: same class — the row was stuck in `running` with expired lease; F-1 fixed the write path but did not address reachability of the retry.)

**Fix options:**
- (a) Skip the requeue for `trigger_path='chat'` and fail immediately (chat-triggered = in-line flow, no detached retry), or
- (b) Extend the recovery list to include `trigger_path='chat'` rows with `retry_count > 0` (retry explicitly; still exclude virgin chat rows), or
- (c) Explicitly defer: add a deferred-registry row in coverage_corrected.md stating chat-triggered retries are unsupported.

### N-2 (LOW) — unbounded `errCtx` + silent skip when terminal re-read fails

- `errCtx := context.Background()` (execution.go:128) has no timeout — a hung DB write blocks the goroutine indefinitely. Prefer `context.WithTimeout(context.Background(), 10*time.Second)`; writes that fail then fall through to `failExecution` (also bounded) instead of leaking.
- If the terminal re-read fails (`current == nil`, :125), the entire error block is skipped and the function returns silently — the row stays `running`, `RetryCount` is not bumped, so `max_retries` is not enforced for that failure. The recovery worker still re-claims after lease expiry, so no permanent hang — only lost retry accounting.

### N-3 (LOW) — stale line refs in coverage_corrected.md evidence column

This round's edits shifted execution.go lines; the coverage doc's evidence refs no longer point at the claimed code:

| Doc ref | Actual |
|---------|--------|
| :65 `execution.go:289-310` (per-node timeout) | :336-358 |
| :66 `execution.go:70-75` (whole-run) | :71-79 |
| :79 `execution.go:117-145` (retry) | :122-152 |
| :77 `execution.go:235-240` (EmitEvent) | :265-275 |
| :61 `execution.go:256-263, :225` (CompileError) | :251-257, :303-308 |

### N-4 (INFO) — whole-run budget is per-attempt, not cumulative

DOCS_ENV_VAR.MD:939 says "If the sum of all step durations exceeds this value, the execution fails", but the budget is created fresh per `runDetachedExecution` invocation (execution.go:78) — each retry/resume attempt gets its own 15 min. With checkpoint-resume this is the *correct* durable semantics; the doc wording should say "per execution attempt".

### Carried gap — retry path has no test

The recommended `isPermanentError`/requeue table test (coverage3_imp_review §5) was not added; grep confirms no test references `isPermanentError` or exercises the requeue branch. N-1's fix is the natural vehicle for it (table test: permanent → fail; transient under max_retries → requeue with preserved checkpoint; transient at max_retries → fail; chat-triggered → per chosen option).

---

## 5. Test Matrix (as of this round)

| Test | File:line | Status |
|------|-----------|--------|
| TestExecuteStep_Timeout | execution_test.go:248-266 | ✓ rewritten (F-5) |
| TestExecuteWorkflow_DAGBuiltin | execution_test.go:268-320 | ✓ unchanged |
| TestEvalConditionDynamic_* | evaluator_test.go | ✓ unchanged |
| T2 worker-2 exclusivity | skill_execution_test.go:82-84 | ✓ unchanged |
| Retry/requeue/isPermanentError | — | ✗ still missing |

---

## 6. Bottom Line

All seven fix items from the prior review are verified in the tree — F-1's dead-`ctx` write bug is genuinely resolved and the retry loop is now bounded and durable for API-triggered executions. **Sign-off is conditional on N-1** (chat-triggered retry dead-end — one of three small fix options, ideally with the carried retry table test), with N-2..N-4 as nits. Once N-1 lands, this round is **APPROVED** and the durable-execution engine is complete and green on its main paths.
