# Coverage 2 — Implementation Plan (coverage_review.md findings)

**Date:** 2026-08-06
**Source:** `coverage_review.md` adversarial review of `coverage.md`
**Baseline:** plan6.md (approved final baseline)
**Scope:** 4 code changes, 2 test additions, 5 assessment corrections

---

## 1. Code Changes

### C1 — Per-node `timeout` enforcement (G-1)

**File:** `server/router/api/v1/agent/execution.go`
**Location:** `executeStep()` — line 290

**Current behavior:**
```go
// Line 290
output, err := h.Execute(ctx, node.Params, celVars)
```
`node.Timeout` is parsed into `SkillDefinition.Timeout` (parser.go:105) but never read in the execution path. A hung handler (e.g. `sleep` with no internal cap, or a slow LLM call) runs unbounded within the 300s lease.

**Proposed change:**
```go
// Line ~289 — after handler lookup, before Execute
execCtx := ctx
if node.Timeout != "" {
    d, parseErr := time.ParseDuration(node.Timeout)
    if parseErr != nil {
        slog.Warn("invalid timeout, ignoring",
            "node", node.Name, "timeout", node.Timeout, "error", parseErr)
    } else {
        var cancel context.CancelFunc
        execCtx, cancel = context.WithTimeout(ctx, d)
        defer cancel()
    }
}

output, err := h.Execute(execCtx, node.Params, celVars)
```

**Error handling:** `context.DeadlineExceeded` propagates up through `executeStep` → `executeWorkflow` → `runDetachedExecution`, where the existing error path calls `failExecution`. No new error handling needed.

**Lease interaction (C4):** If `node.Timeout` ≤ 300s (the lease duration), the step completes before the lease expires. No separate lease renewal needed. Timeout is the primary guard.

**Tests to add:** None directly — covered by the existing `execution_test.go` handler tests. The timeout path is a `context.WithTimeout` wrapper around existing behavior.

---

### C2 — Chat-path double LLM call (G-2)

**File:** `server/router/api/v1/agent/service.go`
**Location:** `generateResponse()` — lines 3081-3106

**Current behavior:**
```go
// Line 3081 — first call WITHOUT tools
resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
    Model:    model,
    Messages: messages,
})
// ...
// Line 3097 — if skills exist, second call WITH tools, discards first resp
if s.skillRegistry != nil && config.SkillGraph != nil && len(config.SkillGraph.Nodes) > 0 {
    tools := s.skillRegistry.ToolsForSkills(config.SkillGraph.Nodes)
    if len(tools) > 0 {
        loopResp, loopErr := s.toolCallingLoop(ctx, client, model, messages, tools, config, session)
        // ...
        resp = *loopResp  // first response discarded
    }
}
```

Every chat message against a skill-bearing tenant costs a **doubled LLM round trip** (latency + tokens). The first call's response is silently discarded.

**Proposed change:**
```go
// Compute tools once before the call
var tools []openrouter.Tool
if s.skillRegistry != nil && config.SkillGraph != nil && len(config.SkillGraph.Nodes) > 0 {
    tools = s.skillRegistry.ToolsForSkills(config.SkillGraph.Nodes)
}

// Single call — always includes tools when available
if len(tools) > 0 {
    resp, err = s.toolCallingLoop(ctx, client, model, messages, tools, config, session)
    if err != nil {
        return "", err
    }
} else {
    resp, err = client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
        Model:    model,
        Messages: messages,
    })
    if err != nil {
        return "", err
    }
    if len(resp.Choices) == 0 {
        return "", fmt.Errorf("no response from LLM")
    }
}
```

**Risk:** If `toolCallingLoop` behaves differently from a single call (e.g. it always loops at least once), this could change behavior. Need to verify that `toolCallingLoop` handles the no-tool-call case correctly — it does (line 3631: `if len(msg.ToolCalls) == 0 { return &resp, nil }`).

**Behavioral impact:** Removes one LLM call per message for skill-bearing tenants. No change for non-skill tenants.

---

### C3 — EmitEvent on stop not dispatched (G-3 + R-8)

**Files:**
- `server/router/api/v1/agent/execution.go` — lines 224-229
- `server/router/api/v1/agent/checkpoint.go` — lines 59-66

**Current behavior:**

In `executeWorkflow` (execution.go:224-229):
```go
if result != nil && result.Met {
    slog.Info("stop condition matched", "exec_id", exec.ID, "condition", graph.Stop.Condition)
    if stopErr := s.store.StopSkillExecution(ctx, exec.ID); stopErr != nil {
        slog.Error("failed to write stopped status", "exec_id", exec.ID, "error", stopErr)
    }
    return errStopSignal
}
```

The `graph.Stop.EmitEvent` field is parsed (parser.go:1249) but never read in the execution path. The stop path writes `stopped` status and returns the sentinel — it never dispatches any event.

In `completeExecution` (checkpoint.go:65):
```go
s.dispatchEvent(ctx, *exec.TenantID, leadID, "workflow.completed", output)
```
This only fires on successful completion, not on stop.

**Proposed change:**

In `executeWorkflow`, after writing stopped status, dispatch the EmitEvent:
```go
if result != nil && result.Met {
    slog.Info("stop condition matched", "exec_id", exec.ID, "condition", graph.Stop.Condition)
    if stopErr := s.store.StopSkillExecution(ctx, exec.ID); stopErr != nil {
        slog.Error("failed to write stopped status", "exec_id", exec.ID, "error", stopErr)
    }
    // R-8: Dispatch EmitEvent if configured
    if graph.Stop.EmitEvent != "" && exec.TenantID != nil {
        leadID := ""
        if exec.ConversationID != "" {
            leadID = exec.ConversationID
        }
        s.dispatchEvent(ctx, *exec.TenantID, leadID, graph.Stop.EmitEvent, "")
    }
    return errStopSignal
}
```

**Note:** `dispatchEvent` is a no-op if no webhook is configured (per plan6 §9), so this is safe to add unconditionally.

---

### C4 — Lease hazard mitigation (G-4)

**File:** `server/router/api/v1/agent/execution.go`
**Location:** `executeStep()` — line 290

**Current behavior:**
- `claimExecution` takes a 300s (5min) lease (checkpoint.go:17)
- No lease renewal exists
- A step running >300s allows the recovery worker to re-claim and re-run side-effecting handlers

**Proposed approach:**
C1 (timeout enforcement) is the primary mitigation. Add a guard:
- If `node.Timeout` is set and > 300s, clamp to 290s (10s before lease expiry)
- If `node.Timeout` is unset, set a default of 290s for detached executions

This ensures no step can exceed the lease. The recovery worker's 30s poll interval + 300s lease means a step must complete within 290s to avoid double-claim.

```go
// In executeStep, after timeout parsing:
if execCtx == ctx {
    // No explicit timeout set — apply lease-safe default for detached executions
    var cancel context.CancelFunc
    execCtx, cancel = context.WithTimeout(ctx, 290*time.Second)
    defer cancel()
} else if d, err := time.ParseDuration(node.Timeout); err == nil && d > 290*time.Second {
    slog.Warn("timeout exceeds lease, clamping to 290s",
        "node", node.Name, "timeout", node.Timeout)
    var cancel context.CancelFunc
    execCtx, cancel = context.WithTimeout(ctx, 290*time.Second)
    defer cancel()
}
```

**Note:** This only applies to the detached execution path. The chat-path inline execution doesn't use leases.

---

## 2. Test Additions

### T1 — CompileError regression test (F-1)

**File:** `server/router/api/v1/agent/evaluator_test.go`

**Add test:**
```go
func TestEvalCondition_CompileError(t *testing.T) {
    // Invalid CEL expression — should return CompileError
    _, err := EvalCondition("invalid_func(!@#$)", nil, nil)
    assert.Error(t, err)

    var compileErr *CompileError
    assert.True(t, errors.As(err, &compileErr),
        "expected CompileError, got %T: %v", err, err)
}
```

**Purpose:** Guards against the F-3 conditional (stop-condition compile errors still log-and-skip). Ensures compile errors are properly typed for downstream `errors.As` checks.

---

### T2 — Different-worker exclusivity test (F-2)

**File:** `store/test/skill_execution_test.go`

**Add test case (within the SQLite test suite):**
```go
func TestClaimExecution_DifferentWorkerExclusivity(t *testing.T) {
    // Create execution, claim with worker A
    exec := createTestExecution(t, db, tenantID)
    claimed1, err := db.ClaimSkillExecution(ctx, exec.ID, "worker-A", 300)
    require.NoError(t, err)
    require.NotNil(t, claimed1)

    // Attempt claim with worker B — should fail (already claimed)
    claimed2, err := db.ClaimSkillExecution(ctx, exec.ID, "worker-B", 300)
    assert.Error(t, err, "different worker should fail to claim already-claimed execution")
    assert.Nil(t, claimed2)

    // Verify original claim still holds
    current, err := db.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
    require.NoError(t, err)
    assert.Equal(t, "worker-A", *current.ClaimedBy)
}
```

**Purpose:** Validates the optimistic locking contract — only the claiming worker can operate on the execution. Guards against the lease-hazard scenario.

---

## 3. Assessment Corrections (coverage.md)

These are documentation-only changes to `bugs/059/coverage.md`.

### D1 — Add UNCOMMITTED banner (F-5)

Insert at top of file (after title block):
```
> **⚠ Tree state: UNCOMMITTED** — All bugs/059 changes exist as untracked/modified files on top of commit `635668f` (🐛 058). The entire deliverable is one `git checkout .` / `git clean -fd` away from zero.
```

### D2 — Fix cadence citation (G-5)

Change line 104:
```
- INFO | 30s recovery ticker vs plan's 15s cadence | accepted deviation
+ INFO | 30s recovery ticker vs plan6 §10's (10s sleep + 60s poll) | accepted deviation
```

### D3 — Re-score state machine (F-1)

Change line 60:
```
- | State machine (6 states) | 60% | 5 statuses implemented (pending/running/completed/failed/stopped); `suspended` + retry-by-clone **absent** |
+ | State machine (6 states) | ~83% | 5/6 states implemented; `created` fused into `pending` by plan6 design (detached path skips `created`, starts at `pending`) |
```

Change deferred item at line 119:
```
- | — | `suspended` state + retry-by-clone resume | MED
+ | — | `created` state never materialized (fused into `pending` — by-design deviation from plan6 §10) | LOW
```

### D4 — Reconcile headline arithmetic (F-2/F-3)

Change line 112:
```
- **Weighted overall: ~72-78%** of the plan6 final scope is implemented and verified in the source tree.
+ **Weighted overall: ~65% (incl. §9 metrics) / ~76% (excl. §9, method-stated)** of the plan6 final scope. Method: scope-count-weighted average across phases. §9 metrics excluded from the ~76% figure as they are from plan.md (REWORK verdict) and not yet measured.
```

Change line 143:
```
- Not started work — the remaining ~25% — is a short, well-scoped list:
+ Not started work — the remaining ~35% (or ~24% excl. §9) — is a short, well-scoped list:
```

### D5 — Reframe lease hazard (G-4)

Change line 133:
```
- | INFO | Same-worker carve-out dormant in prod (per-run UUID workerID, 300s lease) | accepted |
+ | LOW-MED | Lease-renewal absent: step running >300s allows recovery worker to re-claim and re-run side-effecting handlers (double execution). Mitigated by C1 (timeout enforcement capping steps to 290s). | accepted w/ mitigation |
```

---

## 4. Open Questions

### Q1 — C2 (double LLM call): Defer or proceed?

The chat-path double LLM call is the **largest efficiency improvement** in this plan (halves latency + token cost for skill-bearing tenants). However:

- It changes behavior for all tenants with skills configured
- The existing flow (two calls) is tested and working
- The `toolCallingLoop` already handles the no-tool-call case correctly (returns immediately)

**Recommendation:** Proceed. The change is safe because:
1. `toolCallingLoop` returns the first response if no tool calls are made (line 3631)
2. The existing behavior is wasteful by construction (first call discarded)
3. No tenant relies on the discarded first response

### Q2 — C4 (lease hazard): Timeout-only or explicit renewal?

C1 (timeout enforcement) implicitly solves the lease hazard if `node.Timeout` ≤ 300s. Two options:

- **Option A (recommended):** Timeout-only + clamp. Simpler, no goroutine management. Add a 290s default for detached steps without explicit timeout.
- **Option B:** Explicit lease renewal goroutine. More robust for arbitrarily long steps, but adds complexity (goroutine, ticker, channel).

**Recommendation:** Option A. The 300s lease is already a hard cap; timeout enforcement makes it a soft cap at 290s. If a step needs >290s, the tenant should configure a shorter workflow.

### Q3 — D1-D5: Apply to coverage.md or leave untouched?

Three options:
- **Option A:** Apply all D1-D5 corrections to `coverage.md` directly
- **Option B:** Leave `coverage.md` untouched, reference corrections in `coverage2.md` only
- **Option C:** Create a `coverage_corrected.md` with all fixes, keep original as historical record

**Recommendation:** Option C. Preserves the review chain intact while providing a corrected baseline.

---

## 5. Implementation Order

| Step | Change | Dependencies | Effort |
|------|--------|-------------|--------|
| 1 | C1 — Timeout enforcement | None | Small |
| 2 | C4 — Lease clamp (part of C1) | C1 | Trivial |
| 3 | C3 — EmitEvent dispatch | None | Small |
| 4 | C2 — Chat-path single call | None | Medium |
| 5 | T1 — CompileError test | None | Trivial |
| 6 | T2 — Exclusivity test | None | Small |
| 7 | D1-D5 — Assessment corrections | None | Trivial |

Steps 1-4 can be done in parallel (independent files). Steps 5-6 can be done in parallel. Step 7 is independent.

---

## 6. Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| C1 (timeout) | Handler receives cancelled context mid-execution | Existing handlers already check `ctx.Done()` (e.g. SleepHandler). LLMHandler has no internal timeout — context cancel will abort the HTTP call cleanly. |
| C2 (single call) | toolCallingLoop behaves differently than expected | Verified: `toolCallingLoop` returns immediately if no tool calls (line 3631). Same model, same messages, same tools — output should be identical. |
| C3 (EmitEvent) | Event dispatched on every stop, even unintended | `EmitEvent` is only set if tenant explicitly configures `@signal` annotation. Default is empty string → no dispatch. |
| C4 (clamp) | Timeout clamped silently | Warning logged at WARN level. Tenant can still set explicit timeout ≤ 290s. |
