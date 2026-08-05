# Adversarial Review of coverage3.md — Full-Phase Implementation Plan

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-06
**Scope:** `bugs/059/coverage3.md` (full-phase implementation plan addressing coverage_review.md + coverage2_review.md) verified against the live tree (execution.go, service.go, checkpoint.go, evaluator.go, store drivers, test files) and the review chain.
**Method:** Read-only verification of every line reference, signature, and driver behavior; compile-level scrutiny of all four test additions; recomputation of the projected phase scores. No source edits applied — this is the review gate.

---

## 0. Verdict

**REWORK (targeted).** The plan's scope framing is correct — it finally covers all phases (P1-P4 + §9) rather than gap-patching — and C1 (per-step), C2, C3, C5, T1, T2, T3, and D1-D5 verify against the tree. It cannot land as-is because:

- **R-1 (CRITICAL):** C4's re-queue wipes `checkpoint_data`/`completed_nodes`/`failed_nodes`/`trigger_path` via a sparse-struct `UpdateSkillExecution`, so a retried execution re-runs the whole DAG from scratch — re-executing side-effecting handlers. This is the double-execution hazard the plan claims to be closing.
- **R-2 (HIGH):** T4 does not compile (missing `*Service` receiver), would panic on a nil store, and contains no LLM at all — it is a DAG unit test mislabeled as the mock-LLM E2E the P4 gap requires.
- **R-3 (MED-HIGH):** C1's hard 5-minute whole-run default regresses exactly the class of runs the plan's own per-step rule promises not to regress.

The rework is surgical (fix C4's write semantics, re-author T4 against a real store stub + `GenerateFn`, soften the whole-run budget); no architecture change is required.

---

## 1. Verified Correct

| Plan item | Verification |
|-----------|-------------|
| C1 per-step timeout (execution.go:288-302) | Target lines match; `time.ParseDuration` + clamp ≤280s + `WithTimeout` scoping to `h.Execute` only is correct; parse-error path logs and ignores (no regression for malformed values) |
| C1 exit path | DeadlineExceeded → `executeStep` err → `executeWorkflow` wrap → `runDetachedExecution` terminal-guarded `failExecution` (execution.go:114-121); post-run re-read uses `context.Background()` (execution.go:104), so the timed-out ctx cannot break it ✓ |
| C2 (service.go:3081-3106) | No-tools call at :3086-3092, tools branch :3097-3106; restructure hoists `ToolsForSkills` and routes through `toolCallingLoop` (returns first no-tool response, ~:3637); both branches check `len(resp.Choices)==0` ✓ |
| C3 EmitEvent on stop (execution.go:224-229) | Snippet correct; `dispatchEvent` verified no-op without webhook integrations (service.go:5601-5612); relabel to R-8 correct per coverage2_review R-4 ✓ |
| C5 stop-condition hard-fail (execution.go:217-222) | `errors.As(CompileError)` → return; runtime errors still log-and-skip — consistent with node-condition handling (execution.go:258-265) and closes F-3/R-5 ✓ |
| T1 CompileError case | `search_kb.found == treu` → undeclared identifier → check error → `*CompileError`; `EvalConditionDynamic` is the correct function (evaluator.go:121,126) ✓ |
| T2 | One-line append to `TestSkillExecutionRoundTrip` using existing `ts`/`tenantExec` — matches real harness; sqlite returns `"could not be claimed"` on 0 rows (agent_skill.go:178-180) ✓ |
| T3 | `executeStepHelper` (execution_test.go:237-242) is the real harness; `builtin:sleep` with `duration:30` + `Timeout:"1s"` → `ctx.Err()` = "context deadline exceeded" wrapped in `execute step %s:` — `ErrorContains("deadline")` passes ✓ |
| D1-D5 | All corrections match coverage_review/coverage2_review recommendations, incl. corrected D3 wording ("fused by implementation choice, not plan6") ✓ |
| Deferred table | G-3, §9 metrics, checkpoint cadence, simulation, `created`, backoff — correctly classified; exponential-backoff deferral justified by the 30s recovery poll ✓ |
| `created` state | Still never written (checkpoint.go:94 sets `"pending"` for all paths) — consistent with the LOW by-design deferral ✓ |

---

## 2. Findings

### R-1 (CRITICAL) — C4's re-queue destroys the checkpoint and re-runs side effects

`UpdateSkillExecution` (store/db/sqlite/agent_skill.go:104-134, postgres equivalent) **updates all 16 columns unconditionally**, JSON-marshaling whatever maps are in the passed struct. C4's second call:

```go
s.store.UpdateSkillExecution(ctx, &store.SkillExecution{
    ID:             exec.ID,
    Status:         "pending",
    RetryCount:     exec.RetryCount,
    ClaimedBy:      nil,
    ClaimedAt:      nil,
    ClaimExpiresAt: nil,
})
```

leaves `CheckpointData`, `CompletedNodes`, `FailedNodes`, `CurrentNode`, `TriggerPath`, `ErrorMessage`, `CompletedAt` as zero values — which the driver marshals to `{}`/`""`/`0` and writes. Consequences:

1. **`completed_nodes → {}`:** the resume-from-checkpoint contract breaks; the retry re-runs every node from scratch.
2. **Side effects re-execute:** a failed multi-node run retries by re-running all completed steps — create_ticket-class handlers fire twice. This is precisely the double-execution hazard C4/D5 exist to prevent; the plan's retry *creates* it deterministically.
3. **`trigger_path → ""`:** a re-queued `event`/`api` run loses its path; if a `chat` row were ever re-queued, `ListPendingSkillExecutions` (`trigger_path != 'chat'`) would not match it.
4. The first `UpdateSkillExecution(ctx, exec)` (retry-count only) is redundant — a wasted write that the second call partially undoes.

**Required fix — single write, data-preserving.** Either:
- One call with the full local `exec` (`Status:"pending"`, `RetryCount` bumped, `ClaimedAt/By/Expires` nil, all maps/fields intact), or
- A dedicated store method (`RequeueSkillExecution(ctx, id, retryCount)`) whose SQL updates only `status`, `retry_count`, and the three claim columns — which also avoids a future caller repeating this footgun.

### R-2 (HIGH) — T4 cannot compile, would panic, and tests the wrong thing

Three independent defects:

1. **Syntax.** `executeWorkflow` is a method: `func (s *Service) executeWorkflow(ctx, exec, graph, registry)` (execution.go:127). The plan calls it as a bare function → **build error**.
2. **Nil store.** Even with a receiver, the first step calls `s.logSkillStep` → `s.store.CreateSkillLog` (execution.go:320) on a nil store → **panic**, not the asserted error. `writeCheckpoint` then also dereferences the store (checkpoint.go:30). The "will fail at completeExecution (no store)" comment is wrong about where and how it fails.
3. **Wrong target.** Two `builtin:log` steps contain no LLM — no `GenerateFn`, no chat path, no mock LLM. This is a DAG-traversal unit test, not the "integration with mock LLM (end-to-end chat + skill)" that the P4 gap (and coverage.md's "no full chat-skill E2E") demands. Implementing T4 as written would leave the actual E2E gap unfilled while the plan claims +25% on P4.

Also, `assert.Equal(t, "step2", exec.CurrentNode)` only holds if both checkpoints persist — impossible without a functioning store; `CurrentNode` would be `"step1"` at the first failure.

**Required fix:** re-author as a `*Service` test with a store stub that satisfies the methods `executeWorkflow` touches (`GetSkillExecution`, `UpdateSkillExecution`, `CreateSkillLog`, `StopSkillExecution`, `CompleteSkillExecution`) — the agent package cannot import `teststore`, so an in-package fake (or a real sqlite via the store/test pattern used elsewhere) — and include at least one `builtin:llm_call` step with a stubbed `GenerateFn` so the LLM-in-DAG path is genuinely exercised. If a full E2E is out of scope, relabel the deliverable honestly as "DAG traversal + store round-trip" and leave the mock-LLM row at its current score.

### R-3 (MED-HIGH) — C1's whole-run 5-minute default regresses the runs the plan promised not to regress

The per-step section explicitly refuses a default timeout because "today's >290s runs complete successfully on single instances and must not regress" — then C1 imposes a **5-minute hard cap on the entire run**. Any currently-working multi-node run whose total exceeds 300s (e.g., 4 × 80s nodes = 320s) becomes a guaranteed `context deadline exceeded` → `failed` after C1. The risk row's "promotable to env var later" is not in the plan — there is no env read in the snippet.

The 5-minute value is also exactly the lease, so it cannot *close* the multi-node hazard either — it merely converts a double-claim risk into a deterministic failure. The lease problem is a lease problem; the correct mitigations are:
- **(a)** a whole-run budget **higher** than the lease (e.g., env-configurable, default ≥15m) so single-instance runs never regress, **plus**
- **(b)** lease renewal (heartbeat in `writeCheckpoint`, ~20 lines, single-goroutine-safe) so long runs keep their claim, **or** an explicit documented acceptance of the residual multi-node double-claim window.

The per-step clamp (≤280s, explicit-only) stays as-is — it is correct and regression-free.

### R-4 (MED) — C4 retries permanent failures, contradicting C5's rationale

C5 hard-fails on stop-condition compile errors because "config errors must surface." C4 then retries **every** failure, including compile errors (node conditions, C5's stop errors), unknown handlers, and JSON-deserialize failures — 3 re-runs of a dead config over ~90s, then `failed`. Either:
- classify retryability (transient = ctx deadline/timeout, LLM 5xx; permanent = `*CompileError`, handler-not-found, graph deserialize) and only retry transient, or
- document that retries are blanket and accept the masking.

Blanket retry also interacts badly with R-1: until the checkpoint preservation is fixed, each retry is a full side-effect replay.

### R-5 (LOW) — T1's `1 / 0` runtime case may constant-fold at compile time

cel-go constant-folding can reject a literal `1 / 0` at check time — if so, `errors.As(..., &compileErr)` is **true** and the `assert.False` goes red. code10_review asserted it is a runtime error, but that assertion was never empirically landed (the F-1 test was never written). Use a runtime-varying divisor to be robust:

```go
graph := &SkillGraph{Nodes: map[string]*SkillDefinition{
    "x": {Name: "x", Handler: "builtin:log"},
}}
_, err := EvalConditionDynamic(ctx, "1 / x", map[string]any{"x": int64(0)}, graph)
```
(declared node `x` as DynType; zero at eval time → genuine runtime division error → plain error, not CompileError).

### R-6 (LOW) — C4 addresses run-level `max_retries` only; per-node annotation stays dead

`SkillDefinition.MaxRetries` (parser.go:106, :1233-1234) is still never read; `StartDetachedExecution` hardcodes `createExecution(..., 3)` (execution.go:28). C4 honors the column but not the annotation. Either wire per-node max_retries into the retry decision (node failed vs run failed) or add it to the deferred table explicitly.

### R-7 (LOW) — Projected-phase arithmetic is off

Scope-count-weighted using the plan's own post scores (P1 95, P2 98, P3 85, P4 77 mid, §9 0; scope counts 5/5/8/5/5):

- **Incl. §9:** (95·5 + 98·5 + 85·8 + 77·5 + 0·5)/28 = 2030/28 = **72.5%** — plan claims "~80% incl. §9" (off ~7.5 pts)
- **Excl. §9:** 2030/23 = **88.3%** — plan's "~88% excl." ✓

Same hidden-method issue as coverage.md §F-2: state the method, or the incl.§9 headline is unjustified.

---

## 3. Conformance vs Prior Gates

| Gate (coverage_review / coverage2_review) | coverage3 disposition | Status |
|-------------------------------------------|----------------------|--------|
| ADD per-node timeout | C1 | ✓ (per-step part) |
| ADD chat-path double call | C2 | ✓ |
| ADD `@trigger`/event-triggered start | Deferred G-3 (MED) | ✓ |
| REFRAME lease hazard | C1 whole-run + D5 | △ R-3 — budget regression + renewal gap |
| REFRAME `created` fusion | D3 | ✓ |
| FIX cadence citation | D2 | ✓ |
| UNCOMMITTED banner | D1 | ✓ |
| R-1 T1 (EvalConditionDynamic) | T1 | ✓ (R-5 divisor risk) |
| R-2 T2 (real harness) | T2 | ✓ |
| R-6 timeout test | T3 | ✓ |
| Mock-LLM E2E (P4) | T4 | ✗ R-2 — broken + mislabeled |
| R-3 clamp semantics + total-duration bound | C1 | △ R-3 — half-fixed |
| R-5 F-3 decision | C5 | ✓ |
| `max_retries` honored | C4 | △ R-1 (checkpoint wipe) + R-4 (blanket retry) + R-6 (per-node dead) |
| Reconcile headline arithmetic | §6 projection | ✗ R-7 |

---

## 4. Bottom Line

coverage3.md is the right shape of plan — full-phase, with the deferred table and assessment corrections exactly where they should be — and its verified core (C1-per-step, C2, C3, C5, T1/T2/T3, D1-D5) is sound. The rework block is C4's write semantics (R-1, critical), T4's re-authoring against a real store stub with an actual LLM step (R-2, high), and the whole-run budget decision (R-3, med-high), plus the small fixes in R-4..R-7. **Gate: rework C4 to a single data-preserving re-queue (or new store method), re-author T4 (or relabel it), set the whole-run budget above the lease with env configurability or add a lease heartbeat, classify retryability, and correct the projected headline.** After that, the plan is executable and the registry closes completely.
