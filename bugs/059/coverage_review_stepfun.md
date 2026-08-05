# Adversarial Review of bugs/059 Coverage Assessment — StepFun Architecture Review

**Reviewer:** Kilo (senior Go architect, acting as StepFun reviewer)
**Date:** 2026-08-06
**Scope:** `bugs/059/coverage.md`, `coverage_corrected.md`, `coverage3.md`, and all review artifacts verified against the live source tree
**Method:** Read-only verification of every claimed implementation point, line reference, and behavior against actual code in `server/router/api/v1/agent/`, `store/db/sqlite/`, `store/test/`, and `docs/`. No source edits applied; this is the review gate.

---

## 0. Verdict

**APPROVED WITH REQUIRED REVISIONS (non-blocking for the code, blocking for the assessment).** The core engine (parser → graph → CEL → store → recovery → endpoints) is implemented, functional, and green on its main paths. The coverage documents accurately describe a ~73-88% complete implementation of plan6 scope. However, `coverage_corrected.md` contains **stale line references**, **overstated test scores**, an **understated severity** for a user-facing correctness hazard, and an **incomplete deferred table**. None of these defects invalidate the core assessment, but all must be fixed before the document can serve as a stable inventory.

---

## 1. Confirmed Accurate (spot-verified in tree)

| coverage.md / coverage_corrected.md claim | Verification |
|-------------------------------------------|--------------|
| Parser: `@skill`/`@trigger`/`@signal`, `LineStart`, SkillGraph, DAG validation | `parser.go:105-129` (structs), `:1205-1249` (`ParseScriptWithSkills`), `parser_skill_test.go:10-83` (round-trip + cycle detection) |
| CEL with real cel-go, `CompileError`, tolerant eval, `normalizeNumbers` | `evaluator.go:73-83` (`CompileError`), `:89-153` (`EvalConditionDynamic`), `execution.go:452-492` (`buildNodeOutput`/`normalizeNumbers`) |
| `errors.As` compile → hard-fail | `execution.go:260-265` (stop condition), `:313-316` (node condition) |
| Same-worker carve-out in both drivers | `store/db/sqlite/agent_skill.go:168` (`OR claimed_by = ?`); postgres equivalent verified via grep |
| Stop sentinel + status re-read + ctx cancellation | `execution.go:109-121` (re-read, terminal guard), `:226-229` (`StopSkillExecution` before `errStopSignal`) |
| `workflow.completed` dispatch | `checkpoint.go:65` |
| `EmitEvent` dispatch on stop | `execution.go:277-284` |
| Recovery worker 30s ticker, `SKILL_RECOVERY_ENABLED` + `IsRAGEnabled` gating | `recovery.go:13-35` |
| Tenant scoping `*int32`, tenant-filtered list | `checkpoint.go:91-94` (`TenantID`), `store/db/sqlite/agent_skill.go:282-285` (tenant WHERE) |
| Endpoints RBAC-gated, non-MySQL gated | `v1.go:352-357` (`if s.Profile.Driver != "mysql"`), `handlers.go:6743` (permission check) |
| MySQL migration exists | `store/migration/mysql/0.26/00__add_skill_executions.sql`, `LATEST.sql:145-190` |
| Per-node timeout enforcement | `execution.go:345-367` (`time.ParseDuration` → clamp ≤280s → `WithTimeout` scoped to `h.Execute`) |
| Whole-run deadline (env-configurable) | `execution.go:71-79` (`SKILL_WHOLE_RUN_TIMEOUT`, default 15min) |
| Retry with `isPermanentError` classifier + data-preserving re-queue | `execution.go:122-161` (retry count bump, claim release, maps/trigger_path intact) |
| Chat-triggered retry dead-end fix | `execution.go:131-134` (`TriggerPath == "chat"` → `failExecution` immediately) |
| Dead-`ctx` error-path fix | `execution.go:126-127` (`errCtx := context.WithTimeout(context.Background(), 10*time.Second)`) |
| Non-positive timeout guard | `execution.go:353-356` (`d <= 0` → warn + ignore) |
| CompileError + RuntimeError tests | `evaluator_test.go:257-287` |
| Different-worker exclusivity test | `skill_execution_test.go:82-84` |
| Timeout enforcement test | `execution_test.go:250-268` |
| DAG + store integration test | `execution_test.go:270-322` (`TestExecuteWorkflow_DAGBuiltin`) |
| `isPermanentError` table test | `execution_test.go:324-364` |
| Chat-triggered retry skip test | `execution_test.go:366-416` (`TestRetryRequeue_SkipsChat`) |
| Single LLM call in chat path | `service.go:3081-3111` (tools computed once, single branch) |

---

## 2. Structural Findings

### F-1 (HIGH) — coverage_corrected.md line numbers are stale

The coverage_corrected.md evidence column cites line numbers that no longer match the tree after the coverage3 fix round shifted code:

| Doc ref | Actual line |
|---------|-------------|
| `execution.go:289-310` (per-node timeout) | `:336-367` |
| `execution.go:70-75` (whole-run) | `:71-79` |
| `execution.go:117-145` (retry) | `:122-161` |
| `execution.go:235-240` (EmitEvent) | `:265-284` |
| `execution.go:256-263, :225` (CompileError) | `:260-270, :303-308` |

**Impact:** The document cannot be used as a verifiable inventory. Every reader must manually grep to find the claimed code.

**Required fix:** Update all line references in `coverage_corrected.md` §2 evidence column to match the current tree. The fix round landed at 04:19; the document was last updated at 04:03.

---

### F-2 (HIGH) — P4 test scores overstate coverage

`coverage_corrected.md` claims:
- Per-node timeout enforcement test (R-6) | 100%
- DAG traversal + builtin handler integration test | 100%

But `execution_test.go:250-268` (`TestExecuteStep_Timeout`) tests only the **happy-path timeout** (node exceeds its `Timeout`). It does **not** test:
- Non-positive timeout (`"0s"`, `"-1m"`) — the F-2 guard added in coverage3_imp_review2
- Clamp behavior (timeout >280s → 280s)
- Whole-run deadline behavior

Similarly, `TestExecuteWorkflow_DAGBuiltin` tests DAG traversal with a real store, but there is **no test** for:
- Retry/requeue through `runDetachedExecution` (engine-level)
- `isPermanentError` classification in the retry branch
- Stop-condition hard-fail path
- `EmitEvent` dispatch on stop

The 100% scores are unjustified. Realistic scores:
- Timeout test: **60%** (happy path only; missing non-positive, clamp, whole-run)
- DAG integration: **80%** (traversal + store round-trip; missing retry, stop, event paths)

**Required fix:** Re-score P4 test rows in `coverage_corrected.md` to reflect actual test coverage, not just test existence.

---

### F-3 (MED-HIGH) — `graph.Trigger` is dead config; coverage understates the user-facing risk

`coverage3.md` and `coverage_corrected.md` classify `graph.Trigger` (parsed at `parser.go:1240`) as a deferred item with **MED** severity. The actual risk is higher:

The system prompt in `service.go:3436-3443` tells the LLM:
> "To trigger a workflow, call the appropriate tool. **The system will execute the workflow steps automatically.**"

This is **false**. Calling a skill tool executes only that single handler via `toolCallingLoop` — no downstream DAG execution, no topological order, no checkpoint, no stop-signal check. A tenant scripting a multi-step workflow in SCRIPT.md will believe the full pipeline ran when only one node executed.

This is a **user-facing correctness hazard**, not just a missing feature. The deferred-table severity should be **HIGH**, not MED, and the description should mention the misleading system-prompt claim.

**Required fix:** Upgrade `graph.Trigger` / event-triggered start from MED to HIGH in `coverage_corrected.md` deferred table, with note about false system-prompt claim.

---

### F-4 (MED) — coverage_corrected.md P3 "100% retry" glosses over untested branches

`coverage_corrected.md` line 79:
> Retry honoring max_retries (S5/MED-3) | 100%

The implementation is functionally correct, but:
- **No test exercises the retry branch in `runDetachedExecution`** (grep confirms no test references `isPermanentError` or triggers the requeue path through the engine)
- The retry semantics are **total attempts = max_retries**, not "max_retries retries after the first attempt" (F-3 from coverage3_imp_review.md, accepted but undocumented in coverage_corrected.md)
- The `isPermanentError` classifier is **string-match based** (`strings.Contains(msg, "handler not found")`), which is fragile if a handler's own error text contains the phrase

**Required fix:** Either add an engine-level retry test (see §3 item T5) or downgrade the score to "implemented, untested" and add the fragility note to the deferred table.

---

### F-5 (MED) — State machine wording misleading

`coverage_corrected.md` line 63:
> 5/6 states implemented; `created` fused into `pending` by implementation choice (plan6 §6 declares `created → pending → running`)

But `checkpoint.go:94` (`createExecution`) sets `Status: "pending"` directly — there is no path that writes `"created"`. The DDL (`store/migration/sqlite/0.36/00__add_skill_executions.sql`) defaults `status` to `'pending'`. The `created` state is not "fused by design" — it **never existed in the implementation**.

**Required fix:** Reword to: "5 states implemented (`pending`/`running`/`completed`/`failed`/`stopped`); `created` was never materialized in the DDL or code — the state machine starts at `pending`."

---

### F-6 (LOW) — Deferred table incomplete

`coverage_corrected.md` deferred table (lines 128-137) does not list:
- Mock-LLM E2E test gap (still open, classified as MED in coverage3.md)
- Retry path test gap (no engine-level test for `isPermanentError` + requeue)
- `isPermanentError` string-match fragility (LOW)
- `created` state never materialized (LOW, reworded from F-5)

**Required fix:** Expand deferred table to include all open items from the review chain.

---

## 3. Required Code Changes

### C1 — Refresh coverage_corrected.md line numbers (F-1)

**File:** `bugs/059/coverage_corrected.md`
**Action:** Update all line references in §2 evidence column to match current tree after coverage3 fix round (04:19).

Specific replacements:
- `execution.go:289-310` → `execution.go:336-367`
- `execution.go:70-75` → `execution.go:71-79`
- `execution.go:117-145` → `execution.go:122-161`
- `execution.go:235-240` → `execution.go:265-284`
- `execution.go:256-263, :225` → `execution.go:260-270, :303-308`

---

### C2 — Re-score P4 test coverage (F-2)

**File:** `bugs/059/coverage_corrected.md`
**Action:** Update §2 Phase 4 table:

| Scope | Old % | New % | Rationale |
|-------|-------|-------|-----------|
| Per-node timeout enforcement test | 100% | 60% | Happy-path only; missing non-positive, clamp, whole-run tests |
| DAG traversal + builtin handler integration test | 100% | 80% | Traversal + store round-trip verified; retry/stop/event paths untested |

---

### C3 — Upgrade `graph.Trigger` severity to HIGH (F-3)

**File:** `bugs/059/coverage_corrected.md`
**Action:** Update deferred table row:

```
| G-3 | `@trigger` annotation parsed but unconsumed; event-triggered start unimplemented; system prompt falsely claims "system will execute workflow steps automatically" | HIGH |
```

---

### C4 — Add engine-level retry test (F-4 / O-1)

**File:** `server/router/api/v1/agent/execution_test.go`
**Rationale:** The retry branch in `runDetachedExecution` (`execution.go:122-161`) has zero test coverage at the engine level. A table test should verify:
1. Transient error + retry count < max → requeue with preserved checkpoint
2. Permanent error → immediate fail, no retry
3. Transient error + retry count >= max → fail
4. Retry preserves `CheckpointData`, `CompletedNodes`, `TriggerPath`

**Proposed test:**
```go
func TestRunDetachedExecution_RetryRequeue(t *testing.T) {
    ctx := context.Background()
    ts := teststore.NewTestingStore(ctx, t)
    defer ts.Close()

    registry := NewSkillRegistry()
    RegisterBuiltins(registry)

    // Build graph with a permanent-failure node
    graph := &SkillGraph{
        HasSkills: true,
        Nodes: map[string]*SkillDefinition{
            "bad": {Name: "bad", Handler: "builtin:nonexistent"},
        },
        EntryPoints: []string{"bad"},
    }
    graphJSON, _ := json.Marshal(graph)

    exec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
        ID:             uuid.New().String(),
        Status:         "pending",
        SkillGraphJSON: string(graphJSON),
        TriggerPath:    "api",
        MaxRetries:     3,
    })
    if err != nil {
        t.Fatalf("create execution: %v", err)
    }

    svc := &Service{
        store:               ts,
        skillRegistry:       registry,
        activeCancellations: make(map[string]context.CancelFunc),
    }

    svc.runDetachedExecution(ctx, exec)

    updated, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
    if err != nil {
        t.Fatalf("get execution: %v", err)
    }
    if updated.Status != "failed" {
        t.Fatalf("expected status failed (permanent error), got %s", updated.Status)
    }
    if updated.RetryCount != 0 {
        t.Fatalf("expected retry_count 0 (permanent error, no retry), got %d", updated.RetryCount)
    }
}
```

---

### C5 — Add non-positive timeout test (completes T3 coverage)

**File:** `server/router/api/v1/agent/execution_test.go`
**Rationale:** `TestExecuteStep_Timeout` covers only the happy-path timeout. The F-2 guard (`d <= 0` → warn + ignore) is untested.

**Proposed addition:**
```go
func TestExecuteStep_NonPositiveTimeout(t *testing.T) {
    ctx := context.Background()
    exec := &store.SkillExecution{ID: "t", TenantID: nil, ConversationID: "c"}
    node := &SkillDefinition{
        Name:    "sleep",
        Handler: "builtin:sleep",
        Params:  map[string]string{"duration": "1"},
        Timeout: "0s",
    }
    registry := NewSkillRegistry()
    RegisterBuiltins(registry)

    output, err := executeStepHelper(t, ctx, exec, node, registry, map[string]any{})
    if err != nil {
        t.Fatalf("expected no error for zero timeout (ignored), got: %v", err)
    }
    if output != "slept 1s" {
        t.Fatalf("expected 'slept 1s', got: %s", output)
    }
}
```

---

### C6 — Document `isPermanentError` string-match fragility (F-4 / O-2)

**File:** `server/router/api/v1/agent/execution.go`
**Action:** Add a comment above `isPermanentError` (line 520) documenting the fragility:

```go
// isPermanentError classifies errors that should not be retried.
// WARNING: string-match based — a handler error containing "handler not found"
// or "deserialize graph" will be misclassified as permanent. This is acceptable
// for v1 because the engine's own errors use these exact phrases; handlers should
// avoid them in user-facing messages.
func isPermanentError(err error) bool {
```

---

### C7 — Document `graph.Trigger` dead config + false system prompt (F-3 / O-3)

**Files:**
- `server/router/api/v1/agent/parser.go:1240` (where `graph.Trigger` is populated)
- `server/router/api/v1/agent/service.go` (system prompt, ~line 3436)

**Required changes:**
1. Add a comment at `parser.go:1239-1243`:
   ```go
   case "trigger":
       // graph.Trigger is parsed but currently unconsumed by the execution engine.
       // The only start path is the API endpoint (HandleStartWorkflow), which
       // uses req.Trigger as a free-form string. The system prompt in service.go
       // tells the LLM "the system will execute the workflow steps automatically"
       // when a skill tool is called — this is FALSE; toolCallingLoop executes
       // only the single selected handler, not the downstream DAG.
       // TODO: wire graph.Trigger into event-driven start, or fix the prompt.
       graph.Trigger = &TriggerDefinition{
   ```
2. Fix or remove the misleading system-prompt claim in `service.go:3436-3443`.

---

### C8 — Expand deferred table in coverage_corrected.md (F-6 / O-4)

**File:** `bugs/059/coverage_corrected.md`
**Action:** Update §4 deferred table to include all open items:

| ID | Item | Severity |
|----|------|----------|
| G-3 | `@trigger` annotation parsed but unconsumed; event-triggered start unimplemented; system prompt falsely claims automatic DAG execution | HIGH |
| O-1 | Mock-LLM E2E test gap (requires Service.store interface refactor) | MED |
| O-1 | Retry/requeue engine-level test gap (no test for `isPermanentError` + requeue through `runDetachedExecution`) | MED |
| §9 | Exec success rate / latency / concurrency measurement | LOW |
| §9 | Checkpoint cadence (configurable, currently every-step) | LOW |
| §9 | Exponential backoff (recovery worker 30s poll provides spacing) | LOW |
| O-2 | `isPermanentError` string-match fragility (`"handler not found"` substring) | LOW |
| — | Simulation skill extension (new feature, not coverage gap) | LOW |
| — | `created` state never materialized (by-design omission) | LOW |

---

### C9 — Reword state machine description (F-5)

**File:** `bugs/059/coverage_corrected.md`
**Action:** Update §2 Phase 2 state machine row:

```
| State machine (5 states) | ~83% | 5 states implemented (`pending`/`running`/`completed`/`failed`/`stopped`); `created` was never materialized in DDL or code — the machine starts at `pending` |
```

And update deferred table:
```
| — | `created` state never materialized (DLD defaults to `pending`, `createExecution` writes `pending` directly) | LOW |
```

---

## 4. Test Additions Required

### T1 — Engine-level retry/requeue test (C4 above)

See C4 for proposed test. This test must:
- Create a `pending` execution with `MaxRetries: 3`
- Use a graph with a permanent-failure handler (`builtin:nonexistent`)
- Call `runDetachedExecution`
- Assert final status is `failed`, `RetryCount == 0` (permanent error, no retry)
- Assert `CheckpointData` is preserved (empty map, not nil)

### T2 — Transient retry with checkpoint preservation

**File:** `server/router/api/v1/agent/execution_test.go`
**Rationale:** Verify that a transient error (e.g., context deadline exceeded) triggers requeue with preserved checkpoint data.

**Proposed test:**
```go
func TestRunDetachedExecution_RetryPreservesCheckpoint(t *testing.T) {
    ctx := context.Background()
    ts := teststore.NewTestingStore(ctx, t)
    defer ts.Close()

    registry := NewSkillRegistry()
    // Register a handler that always fails with a transient error
    registry.Register(&FailingHandler{
        failErr: context.DeadlineExceeded,
    })

    graph := &SkillGraph{
        HasSkills: true,
        Nodes: map[string]*SkillDefinition{
            "step": {Name: "step", Handler: "builtin:failing"},
        },
        EntryPoints: []string{"step"},
    }
    graphJSON, _ := json.Marshal(graph)

    exec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
        ID:             uuid.New().String(),
        Status:         "pending",
        SkillGraphJSON: string(graphJSON),
        TriggerPath:    "api",
        MaxRetries:     3,
        CheckpointData: map[string]any{"key": "value"},
    })
    if err != nil {
        t.Fatalf("create execution: %v", err)
    }

    svc := &Service{
        store:               ts,
        skillRegistry:       registry,
        activeCancellations: make(map[string]context.CancelFunc),
    }

    svc.runDetachedExecution(ctx, exec)

    updated, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
    if err != nil {
        t.Fatalf("get execution: %v", err)
    }
    // After first transient failure: requeued to pending, RetryCount=1
    if updated.Status != "pending" {
        t.Fatalf("expected status pending after requeue, got %s", updated.Status)
    }
    if updated.RetryCount != 1 {
        t.Fatalf("expected retry_count 1, got %d", updated.RetryCount)
    }
    if updated.CheckpointData == nil {
        t.Fatal("expected CheckpointData to be preserved")
    }
    if updated.CheckpointData["key"] != "value" {
        t.Fatalf("expected CheckpointData['key']='value', got %v", updated.CheckpointData["key"])
    }
}
```

Note: This test requires a `FailingHandler` builtin that returns a configurable error. See C10.

### T3 — Non-positive timeout test (C5 above)

See C5 for proposed test.

---

### C10 — Add FailingHandler builtin for retry tests

**File:** `server/router/api/v1/agent/skill_builtins.go`
**Rationale:** The retry tests (T1, T2) need a handler that returns a controlled error. Currently there is no such builtin.

**Proposed addition:**
```go
// FailingHandler always returns a configurable error (for testing).
type FailingHandler struct {
    FailError error
}

func (h *FailingHandler) Name() string { return "builtin:failing" }

func (h *FailingHandler) Execute(_ context.Context, _ map[string]string, _ map[string]any) (string, error) {
    if h.FailError == nil {
        return "", fmt.Errorf("builtin:failing: no error configured")
    }
    return "", h.FailError
}

func (h *FailingHandler) Definition() openrouter.FunctionDefinition {
    return openrouter.FunctionDefinition{
        Name:        "failing",
        Description: "Always fails with a configurable error (for testing)",
        Parameters: jsonschema.Definition{
            Type: jsonschema.Object,
            Properties: map[string]jsonschema.Definition{
                "message": {
                    Type:        jsonschema.String,
                    Description: "Error message (ignored — FailError is used instead)",
                },
            },
        },
    }
}
```

And register it in `RegisterBuiltins`:
```go
func RegisterBuiltins(reg *SkillRegistry) {
    reg.Register(&LogHandler{})
    reg.Register(&SleepHandler{})
    reg.Register(&LLMHandler{})
    reg.Register(&FailingHandler{})
}
```

---

## 5. Assessment Corrections Required

Apply these corrections to `coverage_corrected.md`:

### D1 — Refresh line numbers (§2 evidence column)

See C1 above.

### D2 — Re-score P4 test coverage (§2 Phase 4 table)

See C2 above.

### D3 — Upgrade `graph.Trigger` severity (§4 deferred table)

See C3 above.

### D4 — Reword state machine description (§2 Phase 2 + §4 deferred)

See C9 above.

### D5 — Expand deferred table (§4)

See C8 above.

### D6 — Add engine-level retry test gap to P4 scoring

In §2 Phase 4 table, add:
```
| Retry/requeue engine-level test | 0% | No test exercises `isPermanentError` + requeue through `runDetachedExecution` |
```

---

## 6. Registry Corrections (delta from coverage_review.md)

| Action | Item | Severity | Status |
|--------|------|----------|--------|
| CLOSE | Per-node `timeout` parsed, never enforced | — | Fixed in code (C1 per coverage2/3) |
| CLOSE | Chat-path double LLM call | — | Fixed in code (C2 per coverage2/3) |
| UPGRADE | `@trigger` dead config / event-triggered start + false system prompt | HIGH | Deferred, severity upgraded (F-3) |
| REFRAME | Lease hazard (double-claim, not dormancy) | LOW-MED | Mitigated by C1 (timeout) + whole-run deadline |
| REFRAME | `created` never materialized (not "fused by design") | LOW | Deferred, wording corrected (F-5) |
| FIX | Cadence citation "15s" → plan6 (10s sleep + 60s poll) | — | Fixed in coverage3 |
| FIX | Tree state: UNCOMMITTED banner added | — | Fixed in coverage3 |
| ADD | Engine-level retry/requeue test gap | MED | New (O-1) |
| ADD | `isPermanentError` string-match fragility | LOW | New (O-2) |
| ADD | Mock-LLM E2E test gap | MED | Carried from coverage3 |
| ADD | Retry semantics documentation (total attempts = max_retries) | LOW | Carried from coverage3_imp_review F-3 |

Net registry: 16 → **19 items** (+3, 2 reframed, 1 corrected, 2 closed).

---

## 7. Final Phase Scores (corrected)

| Phase | coverage_corrected.md | Corrected | Delta |
|-------|----------------------|-----------|-------|
| P1 Core Infrastructure | ~95% | ~95% | — |
| P2 Execution Engine | ~98% | ~98% | — |
| P3 Integration | ~90% | ~90% | — |
| P4 Testing | ~75% | ~65% | -10% (overstated timeout/DAG scores) |
| §9 Metrics | ~0% | ~0% | — |

**Weighted overall (including §9):** ~70% (was ~73%)
**Weighted overall (excluding §9):** ~85% (was ~88%)

---

## 8. Implementation Priority

| Priority | Change | Effort | Impact |
|----------|--------|--------|--------|
| P0 | C1: Refresh line numbers in coverage_corrected.md | Trivial | Documentation accuracy |
| P0 | C2: Re-score P4 test coverage | Trivial | Honest assessment |
| P0 | C3: Upgrade `graph.Trigger` severity to HIGH | Trivial | User-facing risk |
| P1 | C4: Add engine-level retry test (T1) | Small | Test coverage |
| P1 | C5: Add non-positive timeout test (T3) | Trivial | Test coverage |
| P1 | C7: Document `graph.Trigger` dead config + fix system prompt | Small | User-facing correctness |
| P2 | C6: Document `isPermanentError` fragility | Trivial | Code maintainability |
| P2 | C8: Expand deferred table | Trivial | Completeness |
| P2 | C9: Reword state machine description | Trivial | Accuracy |
| P3 | C10: Add FailingHandler builtin | Small | Test infrastructure |

---

## 9. Bottom Line

The coverage documents accurately describe a **~73-88% complete implementation** of the plan6 scope. The core engine is functional and green. The defects are all in the assessment document, not the code:

1. **Line numbers are stale** — document references shifted code locations
2. **P4 scores overstate test completeness** — timeout and DAG tests cover only happy paths
3. **`@trigger` severity understated** — MED should be HIGH due to false system-prompt claim
4. **Deferred table incomplete** — missing mock-LLM E2E, retry test gap, string-match fragility
5. **State machine wording misleading** — `created` was never materialized, not "fused by design"

**Sign-off is conditional on C1-C5 and C8-C9** (documentation corrections), with C6-C7 and C10 as recommended improvements. Once these are applied, `coverage_corrected.md` is a stable, accurate inventory and the durable-skill execution engine is complete and green on its main paths.
