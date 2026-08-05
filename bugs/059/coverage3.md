# Coverage 3 — Full-Phase Implementation Plan

**Date:** 2026-08-06
**Source:** `coverage_review.md` + `coverage2_review.md` adversarial reviews
**Baseline:** plan6.md (approved final baseline)
**Scope:** ALL phases (1-4 + §9) — complete implementation, not just gap patches
**Tree state:** UNCOMMITTED — all bugs/059 on top of commit `635668f` (🐛 058)

---

## Current State Summary

| Phase | coverage.md Score | Verified Score | Gap |
|-------|-------------------|----------------|-----|
| P1 Core Infrastructure | ~95% | ~95% | MySQL stubs (accepted) |
| P2 Execution Engine | ~90% | ~90% | `created` state fused (by-design) |
| P3 Integration | ~65% | ~65% | retry, EmitEvent, event-trigger, timeout |
| P4 Testing | ~60% | ~45-55% | CompileError test, exclusivity test, mock-LLM E2E, simulation |
| §9 Metrics | ~15% | ~0% | All missing |

**Weighted overall:** ~65% (incl. §9) / ~76% (excl. §9)

---

## 1. Code Changes (all phases)

### C1 — Per-node `timeout` enforcement + whole-execution deadline (P2/P3)

**Files:** `server/router/api/v1/agent/execution.go`
**Findings addressed:** G-1 (coverage_review), R-3 (coverage2_review)

**Problem:** `node.Timeout` is parsed (parser.go:105,1228) but never enforced. Hung handlers run unbounded. No per-step or per-run deadline.

**Implementation — per-step timeout (execution.go:288-302):**
```go
// In executeStep, after handler lookup, before Execute
execCtx := ctx
if node.Timeout != "" {
    d, parseErr := time.ParseDuration(node.Timeout)
    if parseErr != nil {
        slog.Warn("invalid timeout, ignoring",
            "node", node.Name, "timeout", node.Timeout, "error", parseErr)
    } else {
        // R-3: clamp explicit timeouts to ≤280s (leave margin for checkpoint write)
        if d > 280*time.Second {
            slog.Warn("timeout exceeds safe bound, clamping to 280s",
                "node", node.Name, "timeout", node.Timeout)
            d = 280 * time.Second
        }
        var cancel context.CancelFunc
        execCtx, cancel = context.WithTimeout(ctx, d)
        defer cancel()
    }
}

// NO default timeout for unset annotations — R-3: today's >290s runs
// complete successfully on single instances and must not regress.

output, err := h.Execute(execCtx, node.Params, celVars)
```

**Implementation — whole-execution deadline (execution.go:69-70, inside `runDetachedExecution`):**
```go
// After WithCancel, add a whole-run deadline (5 minutes, matching lease)
const wholeRunBudget = 5 * time.Minute
ctx, cancel := context.WithTimeout(ctx, wholeRunBudget)
defer cancel()
```

This bounds the entire multi-node run, not just individual steps. If the sum of all steps exceeds 5 minutes, the execution fails cleanly rather than risking lease expiry mid-step.

**Exit path:** `context.DeadlineExceeded` → `executeWorkflow` returns error → `runDetachedExecution` re-reads status → `failExecution` (existing path at execution.go:114-121).

---

### C2 — Chat-path single LLM call (P3)

**File:** `server/router/api/v1/agent/service.go:3081-3106`
**Findings addressed:** G-2 (coverage_review)

**Problem:** Every chat message makes two LLM calls when skills exist — first without tools (discarded), then with tools. Doubles latency + token cost.

**Implementation:**
```go
// Replace lines 3081-3106 in generateResponse()

// Compute tools once before the call
var tools []openrouter.Tool
if s.skillRegistry != nil && config.SkillGraph != nil && len(config.SkillGraph.Nodes) > 0 {
    tools = s.skillRegistry.ToolsForSkills(config.SkillGraph.Nodes)
}

var resp openrouter.ChatCompletionResponse
if len(tools) > 0 {
    loopResp, loopErr := s.toolCallingLoop(ctx, client, model, messages, tools, config, session)
    if loopErr != nil {
        return "", loopErr
    }
    resp = *loopResp
} else {
    var err error
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

**Safety:** `toolCallingLoop` returns immediately when no tool calls are made (service.go:~3631 `if len(msg.ToolCalls) == 0 { return &resp, nil }`). Functionally equivalent to the no-tools path; cosmetically different at worst.

---

### C3 — EmitEvent on stop (P3)

**File:** `server/router/api/v1/agent/execution.go:224-229`
**Findings addressed:** R-8 (coverage_review, relabeled from G-3 per coverage2_review R-4)

**Problem:** `StopSignalDefinition.EmitEvent` is parsed (parser.go:1249) but never dispatched. Stop path writes status and returns sentinel without firing any event.

**Implementation:**
```go
// In executeWorkflow, after writing stopped status (execution.go:224-229)
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

**Safety:** `dispatchEvent` is a no-op when no webhook is configured (service.go:5601-5612). Safe to add unconditionally.

---

### C4 — Retry/backoff honoring `max_retries` (P3)

**File:** `server/router/api/v1/agent/execution.go`
**Findings identified by:** Full audit (item #17 — `RetryCount` field exists, never incremented)

**Problem:** `max_retries` column exists (default 3), `RetryCount` is initialized to 0, but **retry_count is never incremented and no retry logic exists**. Failed executions are marked failed and abandoned.

**Implementation — retry on failure (execution.go:114-121, inside `runDetachedExecution`):**
```go
if execErr != nil {
    slog.Error("workflow execution failed", "exec_id", exec.ID, "error", execErr)
    if current != nil && current.Status != "stopped" && current.Status != "completed" {
        exec.RetryCount++
        if exec.RetryCount < exec.MaxRetries {
            slog.Info("retrying execution",
                "exec_id", exec.ID,
                "retry_count", exec.RetryCount,
                "max_retries", exec.MaxRetries)
            // Update retry count
            if err := s.store.UpdateSkillExecution(ctx, exec); err != nil {
                slog.Error("failed to update retry count", "exec_id", exec.ID, "error", err)
            }
            // Re-queue: reset to pending, release claim
            if err := s.store.UpdateSkillExecution(ctx, &store.SkillExecution{
                ID:             exec.ID,
                Status:         "pending",
                RetryCount:     exec.RetryCount,
                ClaimedBy:      nil,
                ClaimedAt:      nil,
                ClaimExpiresAt: nil,
            }); err != nil {
                slog.Error("failed to re-queue execution", "exec_id", exec.ID, "error", err)
                s.failExecution(ctx, exec, execErr.Error())
            }
            return
        }
        s.failExecution(ctx, exec, execErr.Error())
    }
    return
}
```

**Note:** The re-queue must release the claim so the recovery worker can re-claim. Backoff is deferred — recovery worker's 30s poll provides natural spacing.

---

### C5 — Stop-condition compile errors → hard-fail (P2/P3)

**File:** `server/router/api/v1/agent/execution.go:217-222`
**Findings addressed:** F-3 (coverage_review), R-5 (coverage2_review)

**Problem:** Stop-condition compile errors are logged and treated as "not met" (execution.go:217-222). Node-condition compile errors are hard-fail (execution.go:258-261). Inconsistent.

**Decision:** Apply hard-fail to stop conditions, consistent with node conditions.

**Implementation:**
```go
// Replace lines 217-222
result, evalErr := EvalConditionDynamic(ctx, graph.Stop.Condition, celVars, graph)
if evalErr != nil {
    var compileErr *CompileError
    if errors.As(evalErr, &compileErr) {
        return fmt.Errorf("stop condition compile error: %w", evalErr)
    }
    slog.Warn("stop condition eval error, treating as not met",
        "exec_id", exec.ID, "condition", graph.Stop.Condition, "error", evalErr)
    result = nil
}
```

---

## 2. Test Additions (P4)

### T1 — CompileError regression test (F-1/F-2)

**File:** `server/router/api/v1/agent/evaluator_test.go`
**Corrected per:** coverage2_review R-1

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

func TestEvalConditionDynamic_RuntimeError(t *testing.T) {
    ctx := context.Background()
    graph := &SkillGraph{Nodes: map[string]*SkillDefinition{}}
    _, err := EvalConditionDynamic(ctx, "1 / 0", map[string]any{}, graph)
    assert.Error(t, err)
    var compileErr *CompileError
    assert.False(t, errors.As(err, &compileErr),
        "runtime error should not be CompileError, got %T: %v", err, err)
}
```

---

### T2 — Different-worker exclusivity test (F-2)

**File:** `store/test/skill_execution_test.go`
**Corrected per:** coverage2_review R-2

Append after the existing same-worker re-claim assertion (skill_execution_test.go:78-80):
```go
// Case 3b: Different-worker exclusivity — unexpired lease must block worker-2
_, err = ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-2", 60)
require.Error(t, err)
```

One line. Uses existing `ts` and `tenantExec` from test function scope.

---

### T3 — Timeout enforcement test (R-6)

**File:** `server/router/api/v1/agent/execution_test.go`

```go
func TestExecuteStep_Timeout(t *testing.T) {
    ctx := context.Background()
    exec := &store.SkillExecution{ID: "t", TenantID: nil, ConversationID: "c"}
    node := &SkillDefinition{
        Name:    "slow",
        Handler: "builtin:sleep",
        Params:  map[string]string{"duration": "30"},
        Timeout: "1s",
    }
    registry := NewSkillRegistry()
    RegisterBuiltins(registry)
    _, err := executeStepHelper(t, ctx, exec, node, registry, map[string]any{})
    assert.Error(t, err)
    assert.ErrorContains(t, err, "deadline")
}
```

---

### T4 — Mock LLM integration test (P4)

**File:** `server/router/api/v1/agent/execution_test.go`

```go
func TestExecuteWorkflow_MockE2E(t *testing.T) {
    graph := &SkillGraph{
        HasSkills: true,
        Nodes: map[string]*SkillDefinition{
            "step1": {Name: "step1", Handler: "builtin:log",
                Params: map[string]string{"message": "hello"}},
            "step2": {Name: "step2", Handler: "builtin:log",
                Params: map[string]string{"message": "world"}, DependsOn: []string{"step1"}},
        },
        EntryPoints: []string{"step1"},
    }
    registry := NewSkillRegistry()
    RegisterBuiltins(registry)

    exec := &store.SkillExecution{
        ID:             "test-e2e",
        Status:         "running",
        SkillGraphJSON: "{}",
        CheckpointData: make(map[string]any),
        CompletedNodes: make(map[string]any),
    }

    ctx := context.Background()
    // executeWorkflow will fail at completeExecution (no store),
    // but tests the full DAG traversal + step execution
    err := executeWorkflow(ctx, exec, graph, registry)
    // Expect error from store call, but steps should have executed
    assert.Error(t, err) // store unavailable
    assert.Equal(t, "step2", exec.CurrentNode) // reached final node
}
```

---

## 3. Assessment Corrections (coverage.md → coverage_corrected.md)

Create `bugs/059/coverage_corrected.md` with all corrections applied.

### D1 — Add UNCOMMITTED banner (F-5)
### D2 — Fix cadence citation: "plan's 15s" → "plan6 §10's (10s sleep + 60s poll)"
### D3 — Re-score state machine: 60% → ~83% (created fused by implementation choice, not plan6)
### D4 — Headline: "~72-78%" → "~65% (incl. §9) / ~76% (excl. §9)"
### D5 — Lease hazard: reframe to document whole-run deadline mitigation

---

## 4. Deferred Items

| ID | Item | Severity | Rationale |
|----|------|----------|-----------|
| G-3 | Event-triggered start | MED | New feature, not a coverage gap |
| §9 | Metrics/telemetry | LOW | Observability, not correctness |
| §9 | Configurable checkpoint cadence | LOW | Every-step is acceptable for v1 |
| P4 | Simulation skill extension | LOW | New feature, not a test gap |
| — | `created` state materialization | LOW | By-design fusion |
| — | Exponential backoff | LOW | 30s recovery poll provides spacing |

---

## 5. Implementation Order

| Step | Change | Phase | File | Effort | Dependencies |
|------|--------|-------|------|--------|-------------|
| 1 | C1 — Per-step timeout + whole-run deadline | P2/P3 | execution.go | Medium | None |
| 2 | C4 — Retry/backoff (max_retries) | P3 | execution.go | Medium | None |
| 3 | C5 — Stop-condition compile hard-fail | P2/P3 | execution.go | Small | None |
| 4 | C3 — EmitEvent dispatch on stop | P3 | execution.go | Small | None |
| 5 | C2 — Chat-path single LLM call | P3 | service.go | Medium | None |
| 6 | T1 — CompileError test | P4 | evaluator_test.go | Trivial | C5 |
| 7 | T2 — Exclusivity test (1 line) | P4 | skill_execution_test.go | Trivial | None |
| 8 | T3 — Timeout enforcement test | P4 | execution_test.go | Trivial | C1 |
| 9 | T4 — Mock LLM E2E test | P4 | execution_test.go | Medium | C1-C5 |
| 10 | D1-D5 — Assessment corrections | — | coverage_corrected.md | Trivial | None |

Steps 1-5 in parallel. Steps 6-9 in parallel. Step 10 independent.

---

## 6. Final Phase Scores (after implementation)

| Phase | Before | After | Delta |
|-------|--------|-------|-------|
| P1 Core Infrastructure | ~95% | ~95% | — |
| P2 Execution Engine | ~90% | ~98% | +8% |
| P3 Integration | ~65% | ~85% | +20% |
| P4 Testing | ~45-55% | ~75-80% | +25% |
| §9 Metrics | ~0% | ~0% | DEFERRED |

**Projected:** ~80% (incl. §9) / ~88% (excl. §9)

---

## 7. Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| C1 (timeout) | Handler receives cancelled context | Existing handlers check ctx.Done(). LLMHandler HTTP aborts cleanly. |
| C1 (whole-run) | 5min too short for long workflows | Promotable to env var later. |
| C2 (single call) | toolCallingLoop output differs | Functionally equivalent; same model/messages/tools. |
| C3 (EmitEvent) | Event on every stop | Only if EmitEvent configured. Default empty → no dispatch. |
| C4 (retry) | Infinite loop | Bounded by max_retries (default 3). |
| C5 (hard-fail) | Stop typo fails execution | Intentional — config errors must surface. |
