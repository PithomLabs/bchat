# bchat Durable Execution — Code Review Fix Plan (code2.md)

**Date:** 2026-08-05
**Input:** `bugs/059/code_review.md` (DeepSeek adversarial review)
**Status:** Ready for Implementation

---

## Fix Triage

| ID | Severity | Verdict | Rationale |
|----|----------|---------|-----------|
| CRITICAL-1 | Critical | **FIX NOW** | DDL/Go type mismatch breaks all persistence on Postgres/CockroachDB |
| CRITICAL-1b | Critical | **FIX NOW** | Postgres migration uses CockroachDB dialect (`STRING`) — cannot run |
| CRITICAL-2 | Critical | **FIX NOW** | Cross-tenant data leak via list endpoint |
| CRITICAL-3 | Critical | **FIX NOW** | Stop can never produce `stopped` terminal state |
| HIGH-1 | High | **FIX NOW** | CEL conditions can't reference skill outputs (plan6 D4 not implemented) |
| HIGH-2 | High | **DEFER** | Chat path durability is a Phase 4 integration concern; document as non-durable |
| HIGH-3 | High | **PARTIAL** | Wire `GenerateFn`; document which builtins exist vs planned |
| HIGH-4 | High | **FIX NOW** | List endpoint returns single row / leaks cross-tenant |
| HIGH-5 | High | **FIX NOW** | `@signal`/`@trigger` are dead code — implement stop condition eval |
| MED-1 | Medium | **FIX NOW** | `current_node` stores full output, not node name |
| MED-2 | Medium | **FIX NOW** | Postgres `ListSkillLogs` JSONB scan broken |
| MED-3 | Medium | **DEFER** | Retry/timeout is Phase 4 scope; remove unused fields or document as planned |
| MED-4 | Medium | **DEFER** | Graceful shutdown needs app lifecycle hook — separate concern |
| MED-5 | Medium | **FIX NOW** | `failExecution` stores error in checkpoint data; no terminal guard |
| MED-6 | Medium | **FIX NOW** | Stop on terminal executions; MySQL stubs need gating |
| LOW-1 | Low | **FIX** | Track actual step duration |
| LOW-2 | Low | **DEFER** | CEL caching is optimization; not blocking |
| LOW-3 | Low | **FIX** | Filter unregistered handlers from SECTION 7B |
| LOW-4 | Low | **FIX** | Reorder recovery: claim before deserialize |

**Total: 11 fixes to implement now, 5 deferred**

---

## Implementation Plan

### Fix 1: CRITICAL-1 + CRITICAL-1b — DDL/Go Type Mismatch + Postgres Dialect

**Problem:** `SkillExecution` uses `int64` unix epochs but DDL uses `TIMESTAMPTZ`. Postgres migration uses CockroachDB types (`STRING`).

**Approach:** Change `SkillExecution` timestamps to `time.Time` (matching codebase convention). Rewrite both postgres and cockroach DDLs. Fix all drivers.

**Files to modify:**
- `store/agent.go` — Change `SkillExecution` timestamp fields from `int64` to `time.Time` (or `*time.Time` for nullable)
- `store/migration/postgres/0.36/00__add_skill_executions.sql` — Rewrite in PG dialect
- `store/migration/cockroach/0.36/00__add_skill_executions.sql` — Fix to match Go types
- `store/migration/sqlite/0.36/00__add_skill_executions.sql` — Keep INT8 for SQLite (no TIMESTAMPTZ)
- `store/migration/mysql/0.26/00__add_skill_executions.sql` — Keep INT8 for MySQL
- `store/db/postgres/agent_skill.go` — Rewrite all CRUD to use `time.Time`
- `store/db/sqlite/agent_skill.go` — Convert between `time.Time` ↔ INT8 epoch for SQLite

**SkillExecution struct change:**
```go
// Before:
ClaimedAt       *int64
ClaimExpiresAt  *int64
CreatedAt       int64
UpdatedAt       int64
CompletedAt     *int64

// After:
ClaimedAt       *time.Time
ClaimExpiresAt  *time.Time
CreatedAt       time.Time
UpdatedAt       time.Time
CompletedAt     *time.Time
```

**Postgres DDL (fixed):**
```sql
CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id TEXT PRIMARY KEY,
    tenant_id INT4 DEFAULT NULL,
    conversation_id TEXT NOT NULL,
    skill_graph JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    trigger_path TEXT NOT NULL DEFAULT 'chat',
    current_node TEXT DEFAULT '',
    checkpoint_data JSONB DEFAULT '{}',
    completed_nodes JSONB DEFAULT '{}',
    failed_nodes JSONB DEFAULT '{}',
    retry_count INT4 DEFAULT 0,
    max_retries INT4 DEFAULT 3,
    parent_execution_id TEXT DEFAULT NULL,
    claimed_at TIMESTAMPTZ DEFAULT NULL,
    claimed_by TEXT DEFAULT NULL,
    claim_expires_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ DEFAULT NULL
);
```

**CockroachDB DDL (fixed):**
```sql
CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id STRING PRIMARY KEY,
    tenant_id INT4 DEFAULT NULL,
    conversation_id STRING NOT NULL,
    skill_graph JSONB NOT NULL,
    status STRING NOT NULL DEFAULT 'pending',
    trigger_path STRING NOT NULL DEFAULT 'chat',
    current_node STRING DEFAULT '',
    checkpoint_data JSONB DEFAULT '{}',
    completed_nodes JSONB DEFAULT '{}',
    failed_nodes JSONB DEFAULT '{}',
    retry_count INT4 DEFAULT 0,
    max_retries INT4 DEFAULT 3,
    parent_execution_id STRING DEFAULT NULL,
    claimed_at TIMESTAMPTZ DEFAULT NULL,
    claimed_by STRING DEFAULT NULL,
    claim_expires_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ DEFAULT NULL
);
```

**Postgres driver rewrite — key patterns:**
```go
// Create: use time.Now() directly (pgx handles TIMESTAMPTZ)
func (d *DB) CreateSkillExecution(ctx context.Context, execution *store.SkillExecution) (*store.SkillExecution, error) {
    now := time.Now()
    if execution.CreatedAt.IsZero() { execution.CreatedAt = now }
    if execution.UpdatedAt.IsZero() { execution.UpdatedAt = now }
    stmt := `INSERT INTO agent_skill_executions (...) VALUES ($1, $2, ...)`
    _, err := d.db.ExecContext(ctx, stmt,
        execution.ID, execution.TenantID, execution.ConversationID,
        execution.SkillGraphJSON, execution.Status, execution.TriggerPath,
        execution.CurrentNode, checkpointJSON, completedJSON, failedJSON,
        execution.RetryCount, execution.MaxRetries, execution.ParentExecutionID,
        execution.ClaimedAt, execution.ClaimedBy, execution.ClaimExpiresAt,
        execution.CreatedAt, execution.UpdatedAt, execution.CompletedAt,
    )
    ...
}

// Scan: pgx scans TIMESTAMPTZ directly into *time.Time
func scanSkillExecutions(rows *sql.Rows) ([]*store.SkillExecution, error) {
    ...
    if err := rows.Scan(
        &e.ID, &e.TenantID, &e.ConversationID, &e.SkillGraphJSON, &e.Status, &e.TriggerPath,
        &e.CurrentNode, &checkpointStr, &completedStr, &failedStr,
        &e.RetryCount, &e.MaxRetries, &parentID,
        &e.ClaimedAt, &e.ClaimedBy, &e.ClaimExpiresAt,
        &e.CreatedAt, &e.UpdatedAt, &e.CompletedAt,
    ); err != nil {
        return nil, err
    }
    ...
}
```

**SQLite driver — convert epoch:**
```go
// SQLite stores INT8 epoch; convert to/from time.Time
now := time.Now().Unix()
stmt := `INSERT INTO ... VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
_, err := d.db.ExecContext(ctx, stmt,
    ..., execution.ClaimedAt, execution.ClaimedBy, execution.ClaimExpiresAt,
    now, now, execution.CompletedAt,
)
// On scan:
if claimedAt.Valid { e.ClaimedAt = ptrTime(time.Unix(claimedAt.Int64, 0)) }
```

**Checkpoint/execution.go — fix all `time.Now().Unix()` calls:**
Replace with `time.Now()` since fields are now `time.Time`.

---

### Fix 2: CRITICAL-2 — Cross-Tenant Data Leak

**Problem:** `listSkillExecutionsByTenant` falls back to `ListPendingSkillExecutions` which has no `tenant_id` filter.

**Approach:** Add `ListSkillExecutions` to the store interface and implement in all drivers with proper tenant filtering.

**Files to modify:**
- `store/driver.go` — Add `ListSkillExecutions(ctx, find *FindSkillExecution, limit int) ([]*SkillExecution, error)`
- `store/agent.go` — Add wrapper method
- `store/db/sqlite/agent_skill.go` — Implement with `WHERE tenant_id = ? AND status = ?`
- `store/db/postgres/agent_skill.go` — Implement with `$1` placeholders
- `store/db/mysql/agent_skill.go` — Stub
- `server/router/api/v1/agent/checkpoint.go` — Replace `listSkillExecutionsByTenant` with `s.store.ListSkillExecutions`

**SQL pattern:**
```sql
SELECT ... FROM agent_skill_executions
WHERE tenant_id = $1
  AND ($2 = '' OR status = $2)
ORDER BY created_at DESC
LIMIT $3
```

---

### Fix 3: CRITICAL-3 — Stop Cannot Produce `stopped` State

**Problem:** `runDetachedExecution` calls `failExecution` unconditionally on any error, overwriting `stopped` with `failed`.

**Approach:** Guard `failExecution` and `completeExecution` on terminal status. Re-read status before final write.

**Files to modify:**
- `server/router/api/v1/agent/checkpoint.go` — Add terminal status guard to `failExecution` and `completeExecution`
- `server/router/api/v1/agent/execution.go` — Re-read status in `runDetachedExecution` before calling fail/complete

**Logic:**
```go
func (s *Service) failExecution(ctx context.Context, exec *store.SkillExecution, errMsg string) error {
    // Re-read current status
    current, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
    if err != nil { return err }
    if current == nil { return fmt.Errorf("execution not found") }

    // Don't overwrite terminal states
    switch current.Status {
    case "completed", "stopped":
        return nil // already terminal
    }

    now := time.Now()
    exec.Status = "failed"
    exec.UpdatedAt = now
    exec.CompletedAt = &now
    if exec.CheckpointData == nil { exec.CheckpointData = make(map[string]any) }
    exec.CheckpointData["error"] = errMsg
    return s.store.UpdateSkillExecution(ctx, exec)
}

func (s *Service) completeExecution(ctx context.Context, exec *store.SkillExecution, output string) error {
    // Same re-read guard
    current, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
    if err != nil { return err }
    if current == nil { return fmt.Errorf("execution not found") }
    switch current.Status {
    case "stopped", "failed":
        return nil
    }

    now := time.Now()
    exec.Status = "completed"
    exec.CompletedAt = &now
    exec.UpdatedAt = now
    exec.CurrentNode = output
    ...
}
```

Also in `runDetachedExecution`, check for `stopped` status after context cancellation:
```go
if err := s.executeWorkflow(ctx, exec, graph, s.skillRegistry); err != nil {
    // Check if execution was stopped (not a real error)
    current, _ := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
    if current != nil && current.Status == "stopped" {
        return // already stopped, don't overwrite
    }
    s.failExecution(ctx, exec, err.Error())
}
```

---

### Fix 4: HIGH-1 — CEL Conditions Can't Reference Skill Outputs (D4)

**Problem:** `standardCELVars` doesn't include dynamic node names, so conditions like `search_kb.found == false` fail at compile time.

**Approach:** Build CEL env from `graph.Nodes` keys (dynamic type) plus standard vars. Pass `state[nodeName]` values.

**Files to modify:**
- `server/router/api/v1/agent/evaluator.go` — Add `EvalConditionDynamic` function
- `server/router/api/v1/agent/execution.go` — Call dynamic evaluator in `executeStep`

**New function in evaluator.go:**
```go
// EvalConditionDynamic builds a CEL env with both standard vars and node output vars.
func EvalConditionDynamic(ctx context.Context, expr string, standardVars map[string]any, nodeNames []string, nodeOutputs map[string]any) (*ConditionResult, error) {
    if expr == "" {
        return &ConditionResult{Met: true, Bindings: standardVars}, nil
    }

    // Start with standard vars
    opts := append([]cel.EnvOption{}, standardCELVars...)

    // Add dynamic node variables (any type — could be string, bool, etc.)
    for _, name := range nodeNames {
        opts = append(opts, cel.Variable(name, cel.DynType))
    }

    env, err := cel.NewEnv(opts...)
    if err != nil {
        return nil, fmt.Errorf("cel env: %w", err)
    }

    ast, issues := env.Compile(expr)
    if issues != nil {
        return nil, fmt.Errorf("cel compile: %v", issues)
    }

    prg, err := env.Program(ast)
    if err != nil {
        return nil, fmt.Errorf("cel program: %w", err)
    }

    // Merge standard vars + node outputs
    allVars := make(map[string]any)
    for k, v := range standardVars { allVars[k] = v }
    for k, v := range nodeOutputs { allVars[k] = v }

    out, _, err := prg.Eval(allVars)
    if err != nil {
        return nil, fmt.Errorf("cel eval: %w", err)
    }

    met, ok := out.Value().(bool)
    if !ok {
        return nil, fmt.Errorf("cel expr did not return bool: got %T", out.Value())
    }

    return &ConditionResult{Met: met, Bindings: allVars}, nil
}
```

**execution.go — use dynamic evaluator:**
```go
// In executeStep, build node list and outputs:
nodeNames := make([]string, 0, len(state))
for k := range state { nodeNames = append(nodeNames, k) }

result, err := EvalConditionDynamic(ctx, node.Condition, celVars, nodeNames, state)
```

---

### Fix 5: HIGH-5 — `@signal`/`@trigger` Dead Code

**Problem:** `graph.Stop.Condition` and `graph.Stop.EmitEvent` are parsed but never evaluated.

**Approach:** After each step in `executeWorkflow`, evaluate stop condition. On match, emit event and mark stopped.

**Files to modify:**
- `server/router/api/v1/agent/execution.go` — Add stop condition check in `executeWorkflow`

**Logic (after each step in executeWorkflow):**
```go
// Check stop condition after each step
if graph.Stop != nil && graph.Stop.Condition != "" {
    stopMet, err := EvalConditionDynamic(ctx, graph.Stop.Condition, celVars, nodeNames, state)
    if err != nil {
        slog.Warn("stop condition eval failed", "error", err)
    } else if stopMet.Met {
        slog.Info("stop condition met", "condition", graph.Stop.Condition)
        // Emit stop event
        if graph.Stop.EmitEvent != "" && exec.TenantID != nil {
            s.dispatchEvent(ctx, *exec.TenantID, exec.ConversationID, graph.Stop.EmitEvent, buildWorkflowOutput(state, completed))
        }
        // Mark stopped
        now := time.Now()
        exec.Status = "stopped"
        exec.UpdatedAt = now
        s.store.UpdateSkillExecution(ctx, exec)
        return nil
    }
}
```

---

### Fix 6: HIGH-3 — Wire `GenerateFn` + Document Builtins

**Problem:** `LLMHandler.GenerateFn` is never set, so `llm_call` always errors.

**Approach:** Wire `GenerateFn` in `NewService` to use the service's LLM path. Document which handlers exist.

**Files to modify:**
- `server/router/api/v1/agent/service.go` — Wire `LLMHandler.GenerateFn` after registry init

**Wiring in NewService:**
```go
svc.skillRegistry = NewSkillRegistry()
RegisterBuiltins(svc.skillRegistry)

// Wire LLM handler to use service's LLM path
svc.skillRegistry.Register(&LLMHandler{
    GenerateFn: func(ctx context.Context, prompt string, vars map[string]any) (string, error) {
        // Use the service's LLM config
        model, apiKey, err := svc.requireLLMConfig(ctx, 0) // tenant 0 = default
        if err != nil {
            return "", fmt.Errorf("LLM config unavailable: %w", err)
        }
        client := newOpenRouterClient(apiKey)
        resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
            Model: model,
            Messages: []openrouter.ChatCompletionMessage{
                openrouter.SystemMessage(prompt),
            },
        })
        if err != nil {
            return "", err
        }
        if len(resp.Choices) == 0 {
            return "", fmt.Errorf("no LLM response")
        }
        return resp.Choices[0].Message.Content.Text, nil
    },
})
```

---

### Fix 7: MED-1 — `current_node` Stores Full Output

**Problem:** `writeCheckpoint` sets `exec.CurrentNode = output` (full handler result).

**Approach:** Pass node name to `writeCheckpoint`, keep output only in state.

**Files to modify:**
- `server/router/api/v1/agent/checkpoint.go` — Change `writeCheckpoint` signature to accept node name
- `server/router/api/v1/agent/execution.go` — Pass node name instead of output

**Change:**
```go
// Before:
func (s *Service) writeCheckpoint(ctx context.Context, exec *store.SkillExecution, state map[string]any, output string) error {
    exec.CurrentNode = output
    ...
}

// After:
func (s *Service) writeCheckpoint(ctx context.Context, exec *store.SkillExecution, state map[string]any, nodeName string) error {
    exec.CurrentNode = nodeName
    ...
}
```

---

### Fix 8: MED-2 — Postgres `ListSkillLogs` JSONB Scan

**Problem:** `rows.Scan(&l.Input, &l.Output)` with `map[string]any` fails on JSONB.

**Approach:** Use `sql.NullString` + `json.Unmarshal` like SQLite.

**Files to modify:**
- `store/db/postgres/agent_skill.go` — Fix `ListSkillLogs` scan

**Fix:**
```go
var inputStr, outputStr sql.NullString
if err := rows.Scan(
    &l.ID, &l.TenantID, &l.ExecutionID, &l.SkillName, &l.Handler, &l.Status,
    &inputStr, &outputStr, &l.ErrorMessage, &l.DurationMs, &l.StartedAt, &completedAt,
); err != nil {
    return nil, err
}
if inputStr.Valid { json.Unmarshal([]byte(inputStr.String), &l.Input) }
if outputStr.Valid { json.Unmarshal([]byte(outputStr.String), &l.Output) }
```

---

### Fix 9: MED-5 + MED-6 — failExecution Terminal Guard + Stop Guard

Already covered by Fix 3 (CRITICAL-3). Additionally:

- `stopExecution` should only allow transitioning from `pending`/`running`:
```go
func (s *Service) stopExecution(ctx context.Context, execID string) error {
    exec, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &execID})
    if err != nil { return err }
    if exec == nil { return fmt.Errorf("execution not found") }
    if exec.Status != "pending" && exec.Status != "running" {
        return fmt.Errorf("cannot stop execution in %s state", exec.Status)
    }
    now := time.Now()
    exec.Status = "stopped"
    exec.UpdatedAt = now
    return s.store.UpdateSkillExecution(ctx, exec)
}
```

- MySQL stubs: Gate the endpoints on driver support or add a `NotImplementedError` check.

---

### Fix 10: LOW-1 — Track Actual Step Duration

**Files to modify:**
- `server/router/api/v1/agent/execution.go` — Pass duration to `logSkillStep`

**Change:**
```go
// In executeStep:
start := time.Now()
// ... execute handler ...
duration := time.Since(start)

// In executeWorkflow, pass duration:
s.logSkillStep(ctx, exec, node, output, duration)

// logSkillStep signature:
func (s *Service) logSkillStep(ctx context.Context, exec *store.SkillExecution, node *SkillDefinition, output string, duration time.Duration) {
    log := &store.SkillLog{
        ...
        DurationMs: int(duration.Milliseconds()),
        ...
    }
}
```

---

### Fix 11: LOW-3 + LOW-4 — Filter Unregistered Handlers + Recovery Order

**LOW-3 — Filter SECTION 7B:**
In `buildSystemPrompt`, only list skills that have registered handlers:
```go
for _, node := range config.SkillGraph.Nodes {
    if node.Handler == "condition" || node.Handler == "handler" {
        continue
    }
    if _, ok := s.skillRegistry.Get(node.Handler); !ok {
        continue // skip unregistered
    }
    // ... write to prompt
}
```

**LOW-4 — Recovery reorder:**
In `recoverPendingExecutions`, attempt claim before deserializing:
```go
for _, exec := range pending {
    // Try claim first
    claimed, err := s.claimExecution(ctx, exec.ID, "recovery-"+uuid.New().String()[:8])
    if err != nil {
        continue // someone else claimed it
    }
    // Now deserialize
    graph := &SkillGraph{}
    if err := json.Unmarshal([]byte(claimed.SkillGraphJSON), graph); err != nil {
        s.failExecution(ctx, claimed, "recovery: deserialize failed")
        continue
    }
    go s.runDetachedExecution(ctx, claimed)
}
```

---

## Implementation Order

| Step | Fix | Estimated LOC | Dependencies |
|------|-----|---------------|--------------|
| 1 | CRITICAL-1 + CRITICAL-1b (DDL + store types) | ~200 | None |
| 2 | CRITICAL-2 (ListSkillExecutions store method) | ~80 | Step 1 |
| 3 | CRITICAL-3 (Terminal status guards) | ~60 | Step 1 |
| 4 | HIGH-1 (CEL dynamic evaluator) | ~50 | None |
| 5 | HIGH-5 (Stop condition eval) | ~30 | Step 4 |
| 6 | HIGH-3 (Wire GenerateFn) | ~30 | None |
| 7 | MED-1 (current_node fix) | ~10 | Step 1 |
| 8 | MED-2 (Postgres JSONB scan) | ~15 | Step 1 |
| 9 | MED-5 + MED-6 (stop guard) | ~20 | Step 3 |
| 10 | LOW-1 (duration tracking) | ~10 | Step 1 |
| 11 | LOW-3 + LOW-4 (filter + recovery) | ~20 | None |
| **Total** | | **~525** | |

**Deferred to Phase 4:**
- HIGH-2 (chat path durability)
- MED-3 (retry/timeout semantics)
- MED-4 (graceful shutdown)
- LOW-2 (CEL caching)

---

## Test Plan

### New Tests Required

| Test | File | What it verifies |
|------|------|-----------------|
| `TestCreateSkillExecution_TimeTypes` | execution_test.go | time.Time round-trips correctly |
| `TestFailExecution_TerminalGuard` | execution_test.go | failExecution no-ops on stopped/completed |
| `TestCompleteExecution_TerminalGuard` | execution_test.go | completeExecution no-ops on stopped |
| `TestStopExecution_OnlyRunning` | execution_test.go | stop rejects completed/failed |
| `TestEvalConditionDynamic_WithNodeOutputs` | evaluator_test.go | CEL can reference skill outputs |
| `TestStopConditionMet` | execution_test.go | Stop condition triggers stopped state |
| `TestListSkillExecutions_TenantScoped` | execution_test.go | Only returns tenant's executions |
| `TestWriteCheckpoint_NodeName` | execution_test.go | current_node stores name, not output |

### Existing Tests to Update

- All `SkillExecution` creation in tests needs `time.Time` instead of `int64`
- `TestSkillGraphJSON_RoundTrip` — verify timestamp fields survive marshal/unmarshal

---

## Verification Checklist

After implementation:
1. `go build ./...` — clean
2. `go test ./server/router/api/v1/agent/ -run 'Skill|Execution|Evaluator|Builtin|Parse'` — all pass
3. `go test ./store/...` — all pass (if store tests exist)
4. Manual verification: start workflow → stop → confirm status is `stopped` (not `failed`)
5. Manual verification: list executions for tenant A → confirm no tenant B rows
6. Manual verification: SCRIPT.md with `@signal: condition: "urgency > 5"` → confirm stops early
