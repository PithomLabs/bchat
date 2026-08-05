Viewed plan059_review.md:80-91

**Recommendation: Keep completed/failed terminal. Retry by cloning, not by mutating.**

Here's why, and the concrete state machine:

## The State Machine

```
                    ┌─────────────────────────────────┐
                    │         crash recovery           │
                    │     (startup worker, R6)         │
                    ▼                                  │
               ┌─────────┐                      ┌───────────┐
   create ───▶ │ pending  │ ────────────────────▶│  running   │
               └─────────┘                      └───────────┘
                    ▲                              │       │
                    │                              │       │
                    │  operator resume             ▼       ▼
               ┌───────────┐              ┌──────────┐ ┌────────┐
               │ suspended  │◀─────────── │  failed   │ │completed│
               └───────────┘  resource    └──────────┘ └────────┘
                              exhaustion    TERMINAL      TERMINAL
                              only
```

**Allowed transitions:**

| From | To | Trigger |
|------|----|---------|
| `pending` | `running` | Executor picks up next node |
| `running` | `completed` | All nodes finished |
| `running` | `failed` | Node exhausted retries, no more runnable nodes |
| `running` | `pending` | **Crash recovery only** (startup worker R6) |
| `running` | `suspended` | Resource exhaustion (OOM, rate-limited) |
| `suspended` | `pending` | Operator explicitly resumes |

**Disallowed transitions:**

| Transition | Why Not |
|------------|---------|
| `completed` → `running` | Completed is a historical fact. "Run it again" = new execution. |
| `failed` → `pending` | Mutating a failed record destroys the audit trail. Clone instead. |

## Why Not Manual Retry (failed → pending)?

Look at what the existing codebase does. [AgentEvent](file:///home/chaschel/Documents/go/bchat/store/agent.go#L1269) has `pending → processing → delivered → failed` — failed is terminal. The `attempts` counter tracks retries *within* the processing lifecycle, not as backward state transitions. Same pattern for [ReindexCheckpoint](file:///home/chaschel/Documents/go/bchat/store/agent.go#L1183) — it goes `in_progress → completed/failed`, never backward.

If you allow `failed → pending`:
- **The audit trail becomes unreliable.** "This execution failed at 3:00 PM" — did it? Or was it retried at 3:05 and succeeded? The single record now represents two different execution histories
- **Checkpoint data is ambiguous.** The `completed_nodes` map contains outputs from the first run. Are they still valid after the root cause was fixed? Maybe the KB was updated between runs
- **Error counts become meaningless.** `error_count: 5` — from one run or across retries?

## The Clone Pattern Instead

Add one endpoint:

```
POST /api/v1/agent/:slug/executions/:id/retry
```

This creates a **new** `SkillExecution` record that:
1. Copies the `skill_graph` from the failed execution
2. Pre-populates `completed_nodes` with the successfully completed nodes' outputs (skipping re-execution)
3. Resets `failed_nodes`, `error_count`, `current_node`
4. Sets `status = "pending"`
5. Links to the original via a `parent_execution_id` field

```go
func (e *SkillExecutor) RetryFromFailure(ctx context.Context, failedExecID string) (*SkillExecution, error) {
    original, err := e.store.GetSkillExecution(ctx, failedExecID)
    if err != nil { return nil, err }
    if original.Status != "failed" {
        return nil, fmt.Errorf("can only retry failed executions, got %s", original.Status)
    }

    clone := &SkillExecution{
        TenantID:         original.TenantID,
        ConversationID:   original.ConversationID,
        SkillGraph:       original.SkillGraph,
        Status:           "pending",
        CompletedNodes:   original.CompletedNodes,  // carry over successes
        FailedNodes:      map[string]any{},          // reset failures
        CheckpointData:   original.CheckpointData,   // carry over intermediate data
        ParentExecutionID: &original.ID,              // audit link
    }
    return e.store.CreateSkillExecution(ctx, clone)
}
```

**Benefits:**
- Original execution is preserved as an immutable audit record
- The retry starts from where the failure occurred (completed nodes are skipped)
- You can see the full retry chain: `exec_001 (failed) → exec_002 (retry, failed) → exec_003 (retry, completed)`
- Each execution has its own clean error counts and timing

## Why Not Re-execution (completed → running)?

"Run it again" is just "create a new execution with the same graph." There's no value in mutating a completed record:

```go
// This is cleaner than completed → running
func (e *SkillExecutor) ReExecute(ctx context.Context, execID string) (*SkillExecution, error) {
    original, _ := e.store.GetSkillExecution(ctx, execID)
    return e.store.CreateSkillExecution(ctx, &SkillExecution{
        TenantID:       original.TenantID,
        SkillGraph:     original.SkillGraph,
        Status:         "pending",
        CompletedNodes: map[string]any{},  // fresh start
    })
}
```

## Why Keep `suspended`?

This is the one non-terminal "failure" state, and it exists for a specific reason: **resource exhaustion is not a logic error.** If the LLM API is rate-limited or the process is running out of memory, the execution should pause (not fail permanently) and wait for an operator to resume it. This is the safety valve:

- Rate-limited by OpenRouter → `suspended` (auto-resume after cooldown, or operator resume)
- OOM pressure detected → `suspended` (operator scales up, then resumes)
- Operator wants to pause a runaway execution → `suspended` (manual pause via admin API)

`suspended → pending` is the only "backward" transition, and it requires explicit operator action (not automatic).

## Bottom Line

Keep it simple. The state machine has **5 states, 6 transitions, no backward transitions except crash recovery and suspend/resume.** Retry is a clone operation that preserves the audit trail. This matches how Temporal handles workflow retries (new run ID, old run preserved) and is consistent with how bchat's existing `AgentEvent` system treats terminal states.