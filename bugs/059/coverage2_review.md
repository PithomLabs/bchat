# Adversarial Review of coverage2.md — Implementation Plan for coverage_review.md Findings

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-06
**Scope:** `bugs/059/coverage2.md` (plan addressing coverage_review.md findings G-1..G-5, F-1..F-5) verified against the live tree (execution.go, service.go, checkpoint.go, evaluator.go, store/test, drivers) and the review chain (coverage_review.md, code10_review.md).
**Method:** Read-only verification of every claimed line reference and behavior; compile-level scrutiny of the two proposed test additions; assessment of the plan's own Q1-Q3 recommendations. No source edits applied — this is the review gate.

---

## 0. Verdict

**APPROVED WITH NITS (conditional).** All four code changes (C1-C4) are directionally correct and verified against the tree; the assessment-correction diff (D1-D5) fully implements the coverage_review registry corrections. **However, both test additions (T1, T2) fail to compile against the real codebase as written, C4's lease reasoning has two structural gaps, and the plan drops one registry item (G-3) while mislabeling another (R-8).** None of this invalidates the plan's architecture; all conditions are fixups, not redesigns.

---

## 1. Verified Correct

| Claim | Verification |
|-------|-------------|
| C1 target line | execution.go:290 is exactly `output, err := h.Execute(ctx, node.Params, celVars)` |
| `Timeout` parsed, never read | parser.go:105 (`Timeout string`), :1228 (`block.params["timeout"]`); no read in execution path (grep) |
| DeadlineExceeded exit path | `executeStep` err → `executeWorkflow` wrap → `runDetachedExecution` re-read + terminal-guarded `failExecution` (execution.go:114-121) — fails as `failed` with error_message ✓ |
| C2 current behavior | service.go:3086-3092 no-tools call; :3097-3106 tools call replaces `resp`, discarding the first ✓ |
| toolCallingLoop no-tool return | service.go:~3637 `if len(msg.ToolCalls) == 0 { return &resp, nil }` ✓ (plan cites :3631 — off by ~6 lines) |
| C3 target | execution.go:224-229 stop branch writes `stopped`, returns `errStopSignal`, no dispatch ✓; checkpoint.go:65 dispatches only on complete ✓ |
| dispatchEvent no-op guarantee | service.go:5601-5612 returns early with no webhook integrations ✓ "safe to add unconditionally" ✓ |
| C4 lease facts | lease 300s (checkpoint.go:17), claim once per run (execution.go:86), no renewal ✓ |
| D1-D5 targets | coverage.md lines 60, 104, 112, 119, 133, 143 match the cited content ✓ |

---

## 2. Findings

### R-1 (HIGH) — T1 does not compile and tests the wrong function

```go
_, err := EvalCondition("invalid_func(!@#$)", nil, nil)
var compileErr *CompileError
assert.True(t, errors.As(err, &compileErr), ...)
```

Two independent defects:
1. **Signature mismatch (compile error).** `EvalCondition(ctx context.Context, expr string, vars map[string]any)` (evaluator.go:32). Passing the expression string as the first argument passes a `string` where `context.Context` is required — the file will not build.
2. **Wrong function even if fixed.** `EvalCondition`/`EvalConditionWithTimeout` return **plain errors** — `fmt.Errorf("cel compile: %v", issues)` (evaluator.go:53) and `fmt.Errorf("cel program: %w", err)` (:57). They never return `*CompileError`. The `errors.As` assertion would fail at runtime.

`*CompileError` is only produced by `EvalConditionDynamic` (evaluator.go:121, :126) — the function the node-condition and stop-condition paths actually use.

**Required fix:**
```go
func TestEvalConditionDynamic_CompileError(t *testing.T) {
    ctx := context.Background()
    graph := &SkillGraph{Nodes: map[string]*SkillDefinition{
        "search_kb": {Name: "search_kb", Handler: "builtin:log"},
    }}
    _, err := EvalConditionDynamic(ctx, "search_kb.found == treu", map[string]any{}, graph)
    var compileErr *CompileError
    assert.True(t, errors.As(err, &compileErr),
        "expected CompileError, got %T: %v", err, err)
}
```
(`treu` is a valid identifier but undeclared → check-time error → CompileError; `1/0` remains the complementary runtime-error case per code10_review F-1.)

### R-2 (HIGH) — T2 uses a test harness that does not exist

`createTestExecution(t, db, tenantID)`, `db.ClaimSkillExecution`, and `tenantID` are nowhere in `store/test/skill_execution_test.go`. The file is package `teststore`, one monolith `TestSkillExecutionRoundTrip` over `ts := NewTestingStore(ctx, t)`, with executions created via inline `ts.CreateSkillExecution(&store.SkillExecution{...})` and a tenant from `createBridgeTenant(t, ctx, ts, ...)`. T2 as written will not compile.

The intent is correct and the driver already returns the needed error — sqlite `ClaimSkillExecution` returns `"execution %s could not be claimed"` on 0 rows (store/db/sqlite/agent_skill.go:178-180). **Required fix** — this is exactly code10_review F-2, one assertion appended after the existing same-worker re-claim (skill_execution_test.go:78-80):

```go
// Case 3b: Different-worker exclusivity — unexpired lease must block worker-2
_, err = ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-2", 60)
require.Error(t, err)
```

### R-3 (MED) — C4 does not bound total execution duration

The per-step 290s cap protects a **single step** only. An N-node run accumulates across steps: each node ≤290s, no lease renewal anywhere, so a run of two 200s nodes crosses the 300s lease boundary at t≈400s; the recovery worker (30s poll, `claim_expires_at < now`) re-claims the `running` row and re-runs side-effecting handlers from that point. The plan's D5 mitigation text ("Mitigated by C1 (timeout enforcement capping steps to 290s)") is therefore only true for single-node workflows.

**Additionally,** the 290s default for unset timeouts **regresses working behavior**: today a single-node detached run with no explicit timeout runs unbounded and completes on a single instance; after C4 it hard-fails at 290s with `context deadline exceeded` → status `failed`. The plan trades a theoretical double-claim for a certain new failure mode.

**Required fix:** (a) clamp **only explicitly-configured** timeouts; leave unset timeouts unclamped (or a generous, non-load-bearing floor); (b) add a whole-execution deadline (or a lease heartbeat in `writeCheckpoint`) so multi-node runs are bounded; (c) reduce the clamp to ~280s — the post-step checkpoint write and node bookkeeping run on the *un-timed* workflow ctx, so the "10s margin" does not actually cover DB commit time at the 290s boundary.

### R-4 (MED) — C3 is mislabeled; G-3 dropped

C3's title: "EmitEvent on stop not dispatched **(G-3 + R-8)**". coverage_review's G-3 was *"`@trigger` dead config / event-triggered start unimplemented"* — a distinct item (parser populates `graph.Trigger` at parser.go:1240; nothing reads it; plan6 §1 promises event-triggered runs). C3 implements only R-8 (EmitEvent-on-stop). The G-3 item from the registry-corrections delta is **not present anywhere in coverage2's scope** — neither a change item nor an explicit deferral. Relabel C3 to "(R-8)" and add a scope row: "G-3: event-triggered start — defer to follow-up, keep registry row MED."

### R-5 (LOW) — F-3 decision unrecorded; T1's purpose text overclaims

T1's stated purpose — "Guards against the F-3 conditional (stop-condition compile errors still log-and-skip)" — is false: even corrected per R-1, the test exercises `EvalConditionDynamic` in isolation, not execution.go:217-222 (which swallows the error and treats the stop as not-met). coverage2 never decides F-3: apply the `CompileError` hard-fail to the stop path (consistent with node-condition handling) or record the acceptance in the registry. Either is fine; silence is not.

### R-6 (LOW) — No tests for the code changes

C1/C4 change core engine behavior; the plan adds "Tests to add: None" for both. A ~10-line test using the existing `executeStepHelper` harness (execution_test.go:237-241) with `Timeout: "1s"` + the sleep handler guards the timeout path and the clamp:

```go
func TestExecuteStep_Timeout(t *testing.T) {
    ctx := context.Background()
    exec := &store.SkillExecution{ID: "t", TenantID: nil, ConversationID: "c"}
    node := &SkillDefinition{Name: "slow", Handler: "builtin:sleep",
        Params: map[string]string{"duration": "30"}, Timeout: "1s"}
    registry := NewSkillRegistry()
    RegisterBuiltins(registry)
    _, err := executeStepHelper(t, ctx, exec, node, registry, map[string]any{})
    assert.ErrorContains(t, err, "deadline")
}
```

---

## 3. Nits

| # | Item |
|---|------|
| N-1 | D3 misattributes the `created` fusion to plan6. plan6 §6 detached path is `created → pending → running`; the fusion is the **implementation's** choice — `createExecution` sets `"pending"` for all paths (checkpoint.go:94). Also cite §6, not "§10" (plan6 §10 is the Recovery Worker). |
| N-2 | C2's "output should be identical" overstates: a single pass *with tools* has a different output distribution than a no-tools pass + a tools pass. Frame as "functionally equivalent, cosmetically different at worst." |
| N-3 | C2's "skill-bearing tenants" condition is imprecise — the doubled call only fires when ≥1 **callable** node exists (`ToolsForSkills` emits nothing for condition/handler-only graphs). |
| N-4 | C4's `execCtx == ctx` interface-equality guard is correct but fragile; an explicit `timed := false` bool is clearer. |
| N-5 | Q1's "No tenant relies on the discarded first response" is true only because the first response *was* discarded — fine, but the sentence is circular as written; the real invariant is "no tenant sees the first response today." |

---

## 4. Answers to the Plan's Open Questions (Q1-Q3)

### Q1 — C2 double LLM call: proceed?
**Yes, proceed.** Verified: the pre-loop response is discarded for tool-bearing tenants, `toolCallingLoop` returns the first no-tool response, and error paths are equivalent (`len(resp.Choices)==0` is checked inside the loop at each iteration). The restructure is the cheapest win in the plan (halves latency/tokens for callable-node tenants). Caveats: single-pass-with-tools output may differ cosmetically (N-2), and the saved-cost claim applies only to tenants with ≥1 callable node (N-3).

### Q2 — C4 lease hazard: timeout-only (A) or explicit renewal (B)?
**Neither as written — implement "A-lite":**
1. Clamp explicit timeouts to ≤280s (10s margin is insufficient post-step — checkpoint write runs on the untimed ctx).
2. **No default timeout for unset annotations** — today's >290s single-node runs are safe on single instances and must not start failing (R-3).
3. Bound **total run duration**, not just per-step: a whole-execution deadline on the detached ctx (simplest) or a lease heartbeat in `writeCheckpoint` (option B, ~20 lines, single-goroutine-safe since one goroutine owns the write path). If a whole-run budget is unacceptable, accept and document the multi-node double-claim window as the residual risk rather than pretend the per-step cap closes it.

### Q3 — D1-D5: apply to coverage.md (A), reference-only (B), or coverage_corrected.md (C)?
**Option C, with corrections.** Everything is uncommitted (coverage.md itself is untracked), so no history is at stake either way; C preserves the review chain and matches repo convention of keeping originals + review artifacts. When producing `coverage_corrected.md`, apply the corrected D3 wording (N-1) — the plan's own D3 diff repeats the plan6-attribution error the review originally flagged.

---

## 5. Conformance vs coverage_review.md Registry Corrections

| coverage_review item | coverage2 disposition | Status |
|----------------------|----------------------|--------|
| ADD: per-node `timeout` never enforced | C1 | ✓ (R-6 adds missing test) |
| ADD: chat-path double LLM call | C2 | ✓ |
| ADD: `@trigger` dead config / event-triggered start | **absent** | ✗ R-4 — relabel C3, defer G-3 explicitly |
| REFRAME: lease hazard (double-claim, not dormancy) | C4 + D5 | △ R-3 — fix cap semantics + total-duration bound |
| REFRAME: `suspended` → `created` fused | D3 | △ N-1 — wording re-correction needed |
| FIX: cadence "15s" → plan6 (10s+60s) | D2 | ✓ |
| ADD: UNCOMMITTED banner | D1 | ✓ |
| (carried) F-1 CompileError tests | T1 | ✗ R-1 — broken as written |
| (carried) F-2 exclusivity test | T2 | ✗ R-2 — broken as written |
| (carried) F-3 stop-compile decision | — | ✗ R-5 — not decided |

---

## 6. Bottom Line

coverage2.md is a competent plan whose code changes are verified against the tree and whose assessment corrections faithfully implement coverage_review's registry. It cannot land as-is because T1 and T2 do not compile against the real signatures (R-1, R-2), C4's lease mitigation over-reaches and under-bounds (R-3), and G-3 silently disappears from scope (R-4). None of this requires redesign. **Conditions: fix T1 to use `EvalConditionDynamic`, rewrite T2 as the one-line append to `TestSkillExecutionRoundTrip`, scope C4 to explicit-timeout clamp + whole-run bound, relabel C3 and defer G-3 explicitly, record the F-3 decision, and add the C1/C4 timeout test (R-6).** With those, the plan is executable and the registry is then fully reconciled.
