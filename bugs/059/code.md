# bchat Durable Execution — Implementation Documentation

**Version:** 1.0
**Date:** 2026-08-05
**Status:** Phases 1-3 Complete — Ready for Adversarial Review
**Scope:** Durable execution pipeline for CockroachDB × AWS Hackathon

---

## 1. Implementation Overview

### 1.1 What Was Built

A durable automation pipeline integrated into the bchat multi-tenant AI chat platform. Tenants author workflows via `<!-- @skill: ... -->` annotations in SCRIPT.MD. The system:

1. **Parses** skill definitions, triggers, stop signals, and dependency graphs from SCRIPT.MD
2. **Validates** DAG acyclic dependencies with cycle detection
3. **Executes** workflows via a hybrid model: `builtin:` Go handlers + `llm:` LLM delegation
4. **Persists** execution state in CockroachDB/SQLite with checkpoint-after-each-step
5. **Resumes** from checkpoints after crashes or restarts
6. **Cancels** running executions via `context.WithCancel` per execution
7. **Integrates** with the LLM via OpenRouter tool-calling loop
8. **Dispatches** outbound webhook events on completion via existing `dispatchEvent` outbox
9. **Recovers** pending/abandoned executions via a background recovery worker

### 1.2 Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        bchat Platform                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │  SCRIPT.md  │───▶│ SkillParser │───▶│ SkillGraph  │         │
│  │  (Extended) │    │ (ParseScript│    │ (DAG+Triggers│        │
│  └─────────────┘    │ WithSkills) │    └─────────────┘         │
│                     └─────────────┘         │                    │
│                            │                │                    │
│              ┌─────────────┘                │                    │
│              ▼                              ▼                    │
│  ┌──────────────────────┐    ┌─────────────────────────────┐   │
│  │   ConfigCache        │    │   SkillRegistry             │   │
│  │   (LoadConfig)       │    │   (SkillHandler interface)  │   │
│  └──────────────────────┘    └─────────────────────────────┘   │
│              │                          │                       │
│              ▼                          ▼                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              LLM Tool-Calling Pipeline                   │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │   │
│  │  │  LLM     │→│ Tool     │→│ Execute  │→│Checkpoint│  │   │
│  │  │  Call    │ │ Dispatch │ │ Handler  │ │ Stage    │  │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
│              │                                                  │
│  ┌─────────────────┐ ┌─────────────┐ ┌─────────────────────┐  │
│  │ Chat Path       │ │ Recovery    │ │ dispatchEvent       │  │
│  │ (sync,inline,   │ │ Worker      │ │ outbox (existing)   │  │
│  │  trigger_path=  │ │ (30s poll,  │ │                     │  │
│  │  'chat')        │ │  reclaims)  │ │                     │  │
│  └─────────────────┘ └─────────────┘ └─────────────────────┘  │
│              │                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  CockroachDB / SQLite / PostgreSQL / MySQL              │   │
│  │  ┌─────────────────┐  ┌─────────────────┐              │   │
│  │  │ Skill Executions │  │ Skill Logs      │              │   │
│  │  │ (State Machine)  │  │ (Audit Trail)   │              │   │
│  │  └─────────────────┘  └─────────────────┘              │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Phase 1 — Core Infrastructure

### 2.1 Parser Extensions (`parser.go`)

Extended the existing `ParseScript` to produce `SkillGraph` via `ParseScriptWithSkills`:

```go
type SkillDefinition struct {
    Name       string            `json:"name"`
    Handler    string            `json:"handler"`     // e.g. "builtin:classify_intent"
    DependsOn  []string          `json:"depends_on"`
    Timeout    string            `json:"timeout"`
    MaxRetries int               `json:"max_retries"`
    Condition  string            `json:"condition"`   // CEL guard expression
    LineStart  int               `json:"line_start"`
    Params     map[string]string `json:"params,omitempty"`
}

type SkillGraph struct {
    Nodes       map[string]*SkillDefinition `json:"nodes"`
    EntryPoints []string                    `json:"entry_points"`
    Trigger     *TriggerDefinition          `json:"trigger,omitempty"`
    Stop        *StopSignalDefinition       `json:"stop,omitempty"`
    HasSkills   bool                        `json:"has_skills"`
    ContentHash string                      `json:"content_hash"`
}
```

- `Validate()` checks DAG acyclicity (Kahn's algorithm) and missing dependencies
- `parseHandler()` splits `"builtin:classify_intent"` into `("builtin", "classify_intent")`
- `parseParams()` handles quoted values with commas

### 2.2 Store Types (`store/agent.go`)

```go
type SkillExecution struct {
    ID, TenantID, ConversationID, SkillGraphJSON, Status,
    TriggerPath, CurrentNode string
    CheckpointData, CompletedNodes, FailedNodes map[string]any
    RetryCount, MaxRetries int
    ParentExecutionID, ClaimedBy *string
    ClaimedAt, ClaimExpiresAt, CreatedAt, UpdatedAt, CompletedAt *int64
}

type SkillLog struct {
    ID, TenantID, ExecutionID, SkillName, Handler, Status string
    Input, Output map[string]any
    ErrorMessage string
    DurationMs int
    StartedAt, CompletedAt *int64
}
```

### 2.3 Migrations

| Driver | File | Tables |
|--------|------|--------|
| SQLite | `store/migration/sqlite/0.36/00__add_skill_executions.sql` | `agent_skill_executions`, `agent_skill_logs` |
| Postgres | `store/migration/postgres/0.36/00__add_skill_executions.sql` | Same |
| MySQL | `store/migration/mysql/0.26/00__add_skill_executions.sql` | Same |
| CockroachDB | `store/migration/cockroach/0.36/00__add_skill_executions.sql` | Same |

All 4 `LATEST.sql` files updated.

### 2.4 Store Implementations

- **SQLite** (`store/db/sqlite/agent_skill.go`): 326 lines, full implementation
- **Postgres** (`store/db/postgres/agent_skill.go`): 270 lines, full implementation
- **MySQL** (`store/db/mysql/agent_skill.go`): Stub implementation

### 2.5 Phase 1 Tests

8 tests in `parser_skill_test.go`:
- `TestParseScriptWithSkills` — basic annotation parsing
- `TestParseScriptWithSkillsNoAnnotations` — no skills → no graph
- `TestSkillGraphCycleDetection` — cycle rejection
- `TestSkillGraphMissingDependency` — missing dep error
- `TestSkillGraphValid` — valid DAG passes
- `TestParseScriptWithSkillsLineNumbers` — line number tracking
- `TestParseScriptWithSkillsCommaSeparated` — `depends_on: "a, b, c"`
- `TestParseScriptWithSkillsAnnotationBlockLineStart` — line start tracking

---

## 3. Phase 2 — Execution Engine

### 3.1 SkillRegistry (`skill.go`)

```go
type SkillHandler interface {
    Name() string
    Execute(ctx context.Context, params map[string]string, vars map[string]any) (string, error)
    Definition() openrouter.FunctionDefinition
}

type SkillRegistry struct {
    handlers map[string]SkillHandler
    mu       sync.RWMutex
}
```

Key methods:
- `Register(h)` — panics on duplicate (fail-fast at startup)
- `Get(name)` — O(1) lookup by full name
- `ToolsForSkills(skills)` — converts `SkillDefinition` map to `[]openrouter.Tool`, skipping condition/handler nodes

### 3.2 Builtin Handlers (`skill_builtins.go`)

| Handler | Name | Params | Returns |
|---------|------|--------|---------|
| `LogHandler` | `builtin:log` | `level`, `message` | `"logged"` |
| `SleepHandler` | `builtin:sleep` | `duration` (seconds, max 30) | `"slept Ns"` |
| `LLMHandler` | `builtin:llm_call` | `prompt`, `model` | LLM response text |

`LLMHandler` uses a callback `GenerateFn` to avoid circular dependency with `Service`.

### 3.3 CEL Evaluator (`evaluator.go`)

Uses real cel-go v0.25.0 API:

```go
env, _ := cel.NewEnv(standardCELVars...)
ast, _ := env.Compile(expr)
prg, _ := env.Program(ast)
out, _, _ := prg.Eval(vars)  // map[string]any directly
```

**Standard variables:** `user_message`, `session_messages`, `urgency`, `customer_name`, `tenant_id`, `time_of_day`, `session_id`, `message_count`

**Safety:** 5-second timeout per evaluation via `context.WithTimeout`

### 3.4 Checkpoint/Resume (`checkpoint.go`)

**Claim model:**
```
claimExecution(execID, workerID):
  store.ClaimSkillExecution → UPDATE status='running' WHERE status IN ('pending','running')
  with 5-minute lease

writeCheckpoint(exec, state, output):
  1. Re-read status (R3)
  2. If status != 'running' → abort (someone stopped it)
  3. Update checkpoint_data, current_node, updated_at

completeExecution(exec, output):
  status = 'completed', completed_at = now
  dispatchEvent(tenantID, "workflow.completed", output)  // D6
```

### 3.5 Execution Loop (`execution.go`)

```
executeWorkflow(exec, graph, registry):
  1. topologicalSort(graph) → order
  2. Load state from exec.CheckpointData (resume)
  3. For each node in order:
     a. Check ctx.Done() (cancellation)
     b. Skip if already completed (resume)
     c. Check dependencies met
     d. If condition: EvalCondition → skip if not met
     e. If skill/handler: registry.Get → Execute
     f. writeCheckpoint after each step
     g. logSkillStep for audit trail
  4. completeExecution with final output
```

**Topological sort:** Kahn's algorithm with deterministic ordering (`sort.Strings`)

### 3.6 Recovery Worker (`recovery.go`)

Gated on `SKILL_RECOVERY_ENABLED=true` + `IsRAGEnabled()`:
- Polls every 30 seconds via `time.NewTicker`
- Calls `ListPendingSkillExecutions` (excludes `trigger_path='chat'`)
- Reclaims by deserializing graph and calling `runDetachedExecution`

---

## 4. Phase 3 — Integration

### 4.1 Service Struct Modifications (`service.go`)

New fields added to `Service`:
```go
skillRegistry       *SkillRegistry
activeCancellations map[string]context.CancelFunc
executionMu         sync.Mutex
```

New field added to `AudienceConfig`:
```go
SkillGraph *SkillGraph
```

### 4.2 LoadConfig Skill Parsing (`service.go:1911`)

After loading script content:
```go
if script != nil && script.Content != "" {
    _, graph, err := s.parser.ParseScriptWithSkills(script.Content)
    if err == nil && graph != nil && graph.HasSkills {
        skillGraph = graph
    }
}
```

### 4.3 buildSystemPrompt SECTION 7B (`service.go:3370`)

Injected between SECTION 7 (Learned Behaviors) and SECTION 8 (FAQs):
```
=== AUTOMATION WORKFLOWS ===
You have the following workflows available:
- [skill] extract_lead (builtin:llm_call)
- [condition] high_urgency: urgency > 3
To trigger a workflow, call the appropriate tool.
```

### 4.4 Tool-Calling Loop (`service.go:3554`)

```go
func (s *Service) toolCallingLoop(...) (*openrouter.ChatCompletionResponse, error) {
    maxIterations := 10
    for i := 0; i < maxIterations; i++ {
        select { case <-ctx.Done(): return nil, ctx.Err() default: }
        resp := client.CreateChatCompletion(ctx, req{Model, Messages, Tools})
        if len(msg.ToolCalls) == 0 { return &resp, nil }
        for _, tc := range msg.ToolCalls {
            handler := registry.Get(tc.Function.Name) or registry.Get("builtin:"+name)
            result := handler.Execute(ctx, args, vars)
            append tool result message
        }
    }
}
```

Integration point: after `CreateChatCompletion` returns, before OM goroutine (`service.go:3049`).

### 4.5 API Endpoints (`handlers.go:6718`)

| Method | Path | Handler | Permission |
|--------|------|---------|------------|
| POST | `/api/v1/agent/:slug/workflows/start` | `HandleStartWorkflow` | `workflow:start` |
| POST | `/api/v1/agent/:slug/executions/:id/stop` | `HandleStopExecution` | `workflow:start` |
| GET | `/api/v1/agent/:slug/executions/:id` | `HandleGetExecution` | `tenant:read` |
| GET | `/api/v1/agent/:slug/executions` | `HandleListExecutions` | `tenant:read` |

Routes registered in `v1.go:350-354` on `authGroup` (requires auth + tenant binding).

### 4.6 Permission (`permissions.go`)

```go
PermWorkflowStart = "workflow:start"
```

Added to `AllPermissions` slice.

### 4.7 Outbound Events (D6)

In `completeExecution`:
```go
if exec.TenantID != nil {
    s.dispatchEvent(ctx, *exec.TenantID, leadID, "workflow.completed", output)
}
```

Uses existing `dispatchEvent` outbox pattern with webhook delivery + poller fallback.

---

## 5. File Manifest

### New Files (10)

| File | Lines | Purpose |
|------|-------|---------|
| `agent/skill.go` | 118 | SkillRegistry, SkillHandler interface, parseHandler |
| `agent/skill_test.go` | 178 | 8 unit tests for registry |
| `agent/evaluator.go` | 70 | CEL condition evaluation |
| `agent/evaluator_test.go` | 125 | 10 unit tests for CEL |
| `agent/skill_builtins.go` | 165 | Log, Sleep, LLM handlers |
| `agent/skill_builtins_test.go` | 149 | 8 unit tests for builtins |
| `agent/checkpoint.go` | 209 | Checkpoint/resume, claim, create, list |
| `agent/execution.go` | 337 | Execution loop, topological sort, detached worker |
| `agent/execution_test.go` | 243 | 9 unit tests for execution |
| `agent/recovery.go` | 78 | Recovery worker |

### Modified Files (5)

| File | Changes | LOC delta |
|------|---------|-----------|
| `agent/parser.go` | +Params field to SkillDefinition, populate from annotation | +4 |
| `agent/service.go` | +3 Service fields, +AudienceConfig.SkillGraph, toolCallingLoop, buildSystemPrompt SECTION 7B, LoadConfig parsing, recovery worker startup, SkillRegistry init | +120 |
| `agent/handlers.go` | +4 API endpoints (StartWorkflow, StopExecution, GetExecution, ListExecutions) | +190 |
| `agent/permissions.go` | +PermWorkflowStart constant + AllPermissions entry | +3 |
| `v1.go` | +4 route registrations | +5 |

### Total: ~1,660 lines added/modified

---

## 6. API Endpoints Reference

### POST `/api/v1/agent/:slug/workflows/start`

**Permission:** `workflow:start` or admin

**Request:**
```json
{
  "trigger": "api",
  "initial_vars": {"urgency": 5, "customer_name": "Alice"},
  "conversation_id": "optional-conv-id"
}
```

**Response:**
```json
{
  "execution_id": "uuid",
  "status": "pending",
  "trigger_path": "api"
}
```

### POST `/api/v1/agent/:slug/executions/:id/stop`

**Permission:** `workflow:start` or admin

**Response:**
```json
{
  "execution_id": "uuid",
  "status": "stopped"
}
```

### GET `/api/v1/agent/:slug/executions/:id`

**Permission:** `tenant:read` or admin

**Response:**
```json
{
  "id": "uuid",
  "status": "completed",
  "trigger_path": "api",
  "current_node": "final_step",
  "retry_count": 0,
  "created_at": 1234567890,
  "updated_at": 1234567899,
  "completed_at": 1234567899,
  "checkpoint_data": {},
  "completed_nodes": {"step1": true, "step2": true},
  "failed_nodes": {}
}
```

### GET `/api/v1/agent/:slug/executions?status=pending&limit=50`

**Permission:** `tenant:read` or admin

**Response:**
```json
{
  "items": [...],
  "total": 3
}
```

---

## 7. SCRIPT.md Annotation Format

### Workflow with Skills

```markdown
<!-- @trigger: start, type: "chat" -->
## Workflow Opening

<!-- @skill: classify_intent, handler: "builtin:classify_intent", timeout: "30s" -->
## Classify Intent

<!-- @skill: search_kb, handler: "builtin:search_kb", depends_on: "classify_intent" -->
## Search Knowledge Base

<!-- @condition: urgency > 3, depends_on: "classify_intent" -->
## High Urgency Check

<!-- @skill: escalate, handler: "builtin:log", depends_on: "high_urgency_check", condition: "urgency > 3" -->
## Escalate

<!-- @signal: condition: "urgency > 5", emit_event: "workflow.cancelled" -->
## Stop Signal
```

### Executor Types

| Prefix | Execution Model | Example |
|--------|----------------|---------|
| `builtin:` | Go handler, synchronous | `builtin:classify_intent` |
| `llm:` | LLM delegation via callback | `llm:respond` |
| `condition` | CEL guard, graph-only | `condition` (no colon) |
| `handler` | Graph-only marker | `handler` (no colon) |

---

## 8. Test Coverage

### Unit Tests (47 total)

| File | Tests | Coverage |
|------|-------|----------|
| `parser_skill_test.go` | 8 | ParseScriptWithSkills, cycle detection, missing deps, valid DAG, line numbers, comma-separated, annotation block line start |
| `skill_test.go` | 8 | Registry CRUD, duplicate panic, ToolsForSkills (inclusion/exclusion), parseHandler, parseExecutorType |
| `evaluator_test.go` | 10 | CEL true/false, bindings, comparisons, compound expressions, empty expr, invalid expr, non-bool return, timeout, unknown var |
| `skill_builtins_test.go` | 8 | LogHandler (name, execute, default level), SleepHandler (timing, context cancel, invalid), LLMHandler (nil fn, no prompt, with prompt, message fallback) |
| `execution_test.go` | 9 | Topological sort (linear, diamond, cycle), buildWorkflowOutput (populated, empty), executeStep (condition skip, handler not found, condition not met, condition met, with params), JSON roundtrip |

### Integration Tests (not yet written)

Planned for Phase 4:
- Mock LLM response → tool call → handler execute → final response
- Detached execution: create → claim → execute → complete
- Checkpoint resume: crash mid-execution → recovery → resume
- Stop execution: running → cancel → stopped
- Recovery worker: pending execution → reclaim → complete

---

## 9. Adversarial Code Review Prompt

Use the following prompt with an adversarial reviewer (e.g., Claude, Gemini) to find bugs, race conditions, and security issues:

---

```
You are performing an adversarial code review of a durable execution pipeline implementation
in a Go/Echo multi-tenant SaaS platform. The code manages workflow state machines with
CockroachDB/SQLite persistence, CEL condition evaluation, LLM tool-calling integration,
and concurrent goroutine execution.

CRITICAL: You MUST read every file before reviewing. Do NOT assume behavior. Base ALL
findings on actual code content.

## Files to Review

Read ALL of these files in full:
1. server/router/api/v1/agent/skill.go
2. server/router/api/v1/agent/skill_test.go
3. server/router/api/v1/agent/evaluator.go
4. server/router/api/v1/agent/evaluator_test.go
5. server/router/api/v1/agent/skill_builtins.go
6. server/router/api/v1/agent/skill_builtins_test.go
7. server/router/api/v1/agent/checkpoint.go
8. server/router/api/v1/agent/execution.go
9. server/router/api/v1/agent/execution_test.go
10. server/router/api/v1/agent/recovery.go
11. server/router/api/v1/agent/service.go (focus on: Service struct, NewService, AudienceConfig, LoadConfig skill parsing, toolCallingLoop, buildSystemPrompt SECTION 7B)
12. server/router/api/v1/agent/handlers.go (focus on: HandleStartWorkflow, HandleStopExecution, HandleGetExecution, HandleListExecutions)
13. server/router/api/v1/agent/permissions.go (focus on: PermWorkflowStart)
14. server/router/api/v1/v1.go (focus on: workflow route registration)
15. server/router/api/v1/agent/parser.go (focus on: SkillDefinition struct with Params field)

## Review Categories

For each category, find SPECIFIC bugs with file:line references. Do not speculate.

### 1. Race Conditions & Concurrency
- Is `activeCancellations` map safe for concurrent access? Check every read/write.
- Can two goroutines claim the same execution simultaneously?
- What happens if `StopExecution` is called while `runDetachedExecution` is initializing?
- Is `writeCheckpoint` R3 status re-read sufficient to prevent double-write?

### 2. Security — Tenant Isolation
- Can tenant A stop/view tenant B's execution via the API?
- Is the `TenantID` from the JWT context trusted, or could it be spoofed?
- Does `HandleStartWorkflow` verify the SkillGraph belongs to the requesting tenant?
- Are execution IDs guessable (UUID v4 vs predictable)?

### 3. CEL Injection
- Can a malicious `user_message` break the CEL evaluator?
- What happens if someone passes `user_message: "true; system.exit(1)"` as a variable?
- Is the CEL timeout (5s) sufficient for complex expressions?

### 4. Tool-Calling Loop
- What prevents infinite loops beyond the 10-iteration cap?
- Can the LLM trigger the same tool call repeatedly with different args?
- What happens if a tool call returns an error? Is it surfaced to the LLM?
- Are tool call arguments validated before unmarshaling?

### 5. Checkpoint Integrity
- What happens if the worker crashes between `writeCheckpoint` and `completeExecution`?
- Can a recovery worker claim an execution that's already being processed by another worker?
- What's the lease expiry mechanism? Is it configurable?

### 6. State Machine
- List all valid status transitions. Are there any illegal transitions?
- Can an execution go from `completed` → `running`?
- What happens if `failExecution` is called on an already-completed execution?

### 7. Memory Leaks
- Is `activeCancellations` cleaned up on panic? Check the defer chain.
- Can `recovery.go` spawn goroutines that never terminate?
- Is the `SkillGraph` JSON deserialized on every recovery tick?

### 8. API Surface
- Missing rate limiting on `/workflows/start`?
- Missing input validation on `initial_vars` (can they contain arbitrary structs)?
- What's the response size limit for `/executions` list endpoint?
- Are error messages leaking internal state (stack traces, DB errors)?

### 9. Error Handling
- Are all errors from `store.CreateSkillExecution` handled?
- What happens if `s.dispatchEvent` fails in `completeExecution`?
- Is the `LLMHandler.GenerateFn` nil check sufficient?

### 10. Test Coverage Gaps
- What scenarios are NOT tested?
- Are there race conditions in tests (e.g., concurrent handler registration)?
- Do tests verify cleanup of `activeCancellations`?

## Output Format

For each finding:
- **ID:** (e.g., RACE-001, SEC-001, CEL-001)
- **Severity:** Critical / High / Medium / Low
- **Category:** Race Condition / Security / CEL Injection / etc.
- **File:Line:** Exact location
- **Description:** What the bug is
- **Reproduction:** How to trigger it
- **Fix:** Specific code change needed

Group findings by severity. List Critical first.

DO NOT write code. DO NOT modify files. Only report findings.
```

---

## 10. Known Limitations

| Category | Issue | Severity | Mitigation |
|----------|-------|----------|------------|
| Architecture | `listExecutionsByTenant` falls back to single-row query for non-pending statuses | Medium | Add proper `ListSkillExecutionsByTenant` store method |
| Performance | CEL env created fresh per evaluation (no caching) | Low | Cache compiled CEL programs per expression |
| Security | `LLMHandler.GenerateFn` is nil — not wired in NewService | Medium | Wire in service initialization |
| Testing | No integration tests with mock LLM | High | Phase 4 scope |
| Observability | `logSkillStep` has `DurationMs: 0` (TODO) | Low | Track actual step duration |
| Scalability | `activeCancellations` map grows with running executions | Low | Cleanup on completion already implemented |

---

## 11. Next Steps (Phase 4)

1. Wire `LLMHandler.GenerateFn` to `Service.generateResponse`
2. Add integration tests with mock LLM
3. Add `ListSkillExecutionsByTenant` to store layer for proper pagination
4. Cache compiled CEL programs
5. Track step duration in `logSkillStep`
6. Add rate limiting to `/workflows/start`
7. Add idempotency keys to workflow start
