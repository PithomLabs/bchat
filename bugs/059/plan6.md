# bchat Durable Execution Architecture — Plan 6 (plan6.md)

**Version:** 6.0
**Date:** 2026-08-05
**Status:** APPROVED — Ready for Implementation
**Reviews Addressed:** All prior reviews + plan5_review R1, D4, D1/R4, R3, I1 + minors 1-4

---

## Changelog from plan5.md

| # | Finding | Resolution |
|---|---------|------------|
| R1 | `cel.Vars` doesn't exist in cel-go v0.25.0 | `prg.Eval(vars)` directly (map is valid input to `Program.Eval`) |
| D4 | Node declarations from checkpoint keys → incomplete nodes fail at compile | Declare all nodes from `SkillGraph.Nodes` at env construction; missing nodes evaluate as nil |
| D1/R4 | `ParsedScript.AnnotationBlocks` doesn't exist | Call package-level `extractAnnotationBlocks(content)` directly; `ParsedScript` has only `Summary`/`Sections`/`RawContent` |
| R3 | Status re-read guard applied after the unconditional write | Re-read row status at top of `executeSkill` before any mutation |
| I1 | Detached path uses `context.Background()`, `getTenantFromContext` returns nil | Seed tenant from `exec.TenantID` into detached context via `context.WithValue` |
| M1 | `h.getTenant(c, slug)` doesn't exist | Use `getTenantOrFail(ctx, h.store, c)` |
| M2 | `s.ragEnabled` not a field | Use `s.IsRAGEnabled()` |
| M3 | `Content` is `openrouter.Content` struct, not string | `Content: openrouter.Content{Text: buildSkillPrompt(...)}` |
| M4 | Chat path skips `created`/`pending` states | Document: chat path uses `running`→`completed`/`failed`/`stopped` only |

---

## 1. Executive Summary

**Goal:** Transform bchat from a reactive chat agent into a durable automation pipeline that survives failures, resumes from checkpoints, and provides full audit trails — all driven by declarative markdown annotations in SCRIPT.md.

**Execution Model (D2, R2 — Closed):** Synchronous chat execution for `/chat/ext` and `/chat/int` requests; a detached background worker owns recovery, event-triggered, and API-triggered runs. Chat-path executions carry `trigger_path = 'chat'` and are excluded from the worker's claim query.

**Hackathon Requirements Met:**
- CockroachDB: Distributed vector indexing + durable execution state
- AWS: Amazon Bedrock (via OpenRouter) + Amazon S3 (skill assets)
- Go: All extensions written in Go
- 2+ CRDB tools: Vector Index + Skill Execution State
- 1+ AWS service: Bedrock + S3

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        bchat Platform                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │  SCRIPT.md  │───▶│ SkillParser │───▶│ SkillGraph  │         │
│  │  (Extended) │    │ (Annotation)│    │ (DAG+Triggers)│       │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                              │                  │                │
│              ┌───────────────┘                  │                │
│              ▼                                  ▼                │
│  ┌──────────────────────┐    ┌─────────────────────────────┐   │
│  │   ConfigCache        │    │   Skill Tool Registry       │   │
│  └──────────────────────┘    └─────────────────────────────┘   │
│              │                                                  │
│              ▼                                                  │
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
│  │  trigger_path=  │ │ (trigger_   │ │                     │  │
│  │  'chat')        │ │  path!=     │ │                     │  │
│  │                 │ │  'chat')    │ │                     │  │
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

## 3. SCRIPT.md Extension

### 3.1 Annotation Format

```markdown
<!-- @trigger: start, type: "chat" -->
## Workflow Opening

<!-- @skill: classify_intent, handler: "builtin:classify_intent", timeout: "30s", max_retries: 3 -->
## Classify Intent

<!-- @skill: search_kb, handler: "builtin:search_kb", depends_on: "classify_intent", timeout: "10s" -->
## Search Knowledge Base

<!-- @skill: create_ticket, handler: "builtin:create_ticket", depends_on: "search_kb", condition: "search_kb.found == false", timeout: "15s" -->
## Create Support Ticket

<!-- @skill: respond, handler: "llm:respond", depends_on: "classify_intent, search_kb, create_ticket" -->
## Respond to Customer

<!-- @signal: stop, condition: "create_ticket.ticket_id != ''", emit_event: "pipeline_completed" -->
## Workflow Completion
```

### 3.2 Parser Extension (D1/R4)

**D1/R4 fix:** `ParsedScript` has no `AnnotationBlocks` field. Call `extractAnnotationBlocks(content)` directly.

**R4 fix:** Skill name read from `params["code"]` (first positional value). `LineStart` derived from byte offset.

```go
func (p *Parser) ParseScriptWithSkills(content string) (*ParsedScript, *SkillGraph, error) {
    parsed, err := p.ParseScript(content)
    if err != nil {
        return nil, nil, err
    }

    graph := &SkillGraph{Nodes: make(map[string]*SkillDefinition)}
    graph.ContentHash = sha256Hex(content)

    // D1/R4: Call package-level extractAnnotationBlocks directly
    blocks := extractAnnotationBlocks(content)

    for _, block := range blocks {
        switch block.annotationType {
        case "skill":
            name := block.params["code"]
            if name == "" {
                continue
            }
            skill := &SkillDefinition{
                Name:      name,
                Handler:   block.params["handler"],
                DependsOn: parseCommaSeparated(block.params["depends_on"]),
                Timeout:   block.params["timeout"],
                Condition: block.params["condition"],
                LineStart: block.lineStart,
            }
            if skill.MaxRetries == 0 {
                skill.MaxRetries = 3
            }
            graph.Nodes[skill.Name] = skill
            graph.HasSkills = true

        case "trigger":
            graph.Trigger = &TriggerDefinition{
                Type:      block.params["type"],
                EventType: block.params["event_type"],
            }

        case "signal":
            if block.params["condition"] != "" {
                graph.Stop = &StopSignalDefinition{
                    Condition: block.params["condition"],
                    EmitEvent: block.params["emit_event"],
                }
            }
        }
    }

    if graph.HasSkills {
        if valErr := graph.Validate(); valErr != nil {
            return nil, nil, fmt.Errorf("invalid skill graph at line %d: %w", valErr.Line, valErr.Message)
        }
    }

    return parsed, graph, nil
}
```

### 3.3 DAG Validation

```go
type ValidationError struct {
    Message string
    Line    int
}

func (g *SkillGraph) Validate() *ValidationError {
    if cycle := g.detectCycle(); cycle != nil {
        return &ValidationError{
            Message: fmt.Sprintf("cycle detected: %s", strings.Join(cycle, " → ")),
            Line:    g.Nodes[cycle[0]].LineStart,
        }
    }
    for name, skill := range g.Nodes {
        for _, dep := range skill.DependsOn {
            if _, exists := g.Nodes[dep]; !exists {
                return &ValidationError{
                    Message: fmt.Sprintf("skill %q depends on %q which does not exist", name, dep),
                    Line:    skill.LineStart,
                }
            }
        }
    }
    for name, skill := range g.Nodes {
        if len(skill.DependsOn) == 0 {
            g.EntryPoints = append(g.EntryPoints, name)
        }
    }
    return nil
}
```

---

## 4. Skill Tool Registry

```go
type SkillHandler interface {
    Name() string
    Execute(ctx context.Context, input map[string]any, checkpoint map[string]any) (map[string]any, error)
    Schema() map[string]any
}

type SkillRegistry struct {
    handlers map[string]SkillHandler
    mu       sync.RWMutex
}

func (r *SkillRegistry) Register(handler SkillHandler) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.handlers[handler.Name()] = handler
}

func (r *SkillRegistry) Get(name string) (SkillHandler, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    h, ok := r.handlers[name]
    return h, ok
}
```

---

## 5. Database Schema

All tables include `trigger_path` (R2) and claim columns (D5).

### 5.1 CockroachDB DDL

```sql
CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id INT8 NOT NULL,
    conversation_id STRING NOT NULL,
    skill_graph JSONB NOT NULL,
    status STRING NOT NULL DEFAULT 'pending',
    trigger_path STRING NOT NULL DEFAULT 'chat',
    current_node STRING,
    checkpoint_data JSONB DEFAULT '{}',
    completed_nodes JSONB DEFAULT '{}',
    failed_nodes JSONB DEFAULT '{}',
    retry_count INT4 DEFAULT 0,
    max_retries INT4 DEFAULT 3,
    parent_execution_id UUID,
    claimed_at TIMESTAMPTZ,
    claimed_by STRING,
    claim_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    INDEX idx_skill_exec_tenant (tenant_id),
    INDEX idx_skill_exec_status (status),
    INDEX idx_skill_exec_conversation (conversation_id),
    INDEX idx_skill_exec_claim (status, trigger_path, claimed_at)
);

CREATE TABLE IF NOT EXISTS agent_skill_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id INT8 NOT NULL,
    execution_id UUID NOT NULL REFERENCES agent_skill_executions(id) ON DELETE CASCADE,
    skill_name STRING NOT NULL,
    handler STRING NOT NULL,
    status STRING NOT NULL,
    input JSONB,
    output JSONB,
    error_message STRING,
    duration_ms INT4,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    INDEX idx_skill_log_tenant (tenant_id),
    INDEX idx_skill_log_execution (execution_id)
);
```

### 5.2 SQLite DDL

```sql
CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER DEFAULT NULL,
    conversation_id TEXT NOT NULL,
    skill_graph TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    trigger_path TEXT NOT NULL DEFAULT 'chat',
    current_node TEXT,
    checkpoint_data TEXT DEFAULT '{}',
    completed_nodes TEXT DEFAULT '{}',
    failed_nodes TEXT DEFAULT '{}',
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    parent_execution_id TEXT,
    claimed_at INTEGER DEFAULT 0,
    claimed_by TEXT,
    claim_expires_at INTEGER DEFAULT 0,
    created_at INTEGER DEFAULT 0,
    updated_at INTEGER DEFAULT 0,
    completed_at INTEGER DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_skill_exec_tenant ON agent_skill_executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_exec_status ON agent_skill_executions(status);
CREATE INDEX IF NOT EXISTS idx_skill_exec_conversation ON agent_skill_executions(conversation_id);
CREATE INDEX IF NOT EXISTS idx_skill_exec_claim ON agent_skill_executions(status, trigger_path, claimed_at);

CREATE TABLE IF NOT EXISTS agent_skill_logs (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER DEFAULT NULL,
    execution_id TEXT NOT NULL,
    skill_name TEXT NOT NULL,
    handler TEXT NOT NULL,
    status TEXT NOT NULL,
    input TEXT,
    output TEXT,
    error_message TEXT,
    duration_ms INTEGER DEFAULT 0,
    started_at INTEGER DEFAULT 0,
    completed_at INTEGER DEFAULT 0,
    FOREIGN KEY (execution_id) REFERENCES agent_skill_executions(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_skill_log_tenant ON agent_skill_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_log_execution ON agent_skill_logs(execution_id);
```

### 5.3 MySQL DDL

```sql
CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INT DEFAULT NULL,
    conversation_id VARCHAR(255) NOT NULL,
    skill_graph JSON NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    trigger_path VARCHAR(10) NOT NULL DEFAULT 'chat',
    current_node VARCHAR(255),
    checkpoint_data JSON DEFAULT (JSON_OBJECT()),
    completed_nodes JSON DEFAULT (JSON_OBJECT()),
    failed_nodes JSON DEFAULT (JSON_OBJECT()),
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    parent_execution_id VARCHAR(36),
    claimed_at TIMESTAMP NULL,
    claimed_by VARCHAR(255),
    claim_expires_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    INDEX idx_skill_exec_tenant (tenant_id),
    INDEX idx_skill_exec_status (status),
    INDEX idx_skill_exec_conversation (conversation_id),
    INDEX idx_skill_exec_claim (status, trigger_path, claimed_at)
);

CREATE TABLE IF NOT EXISTS agent_skill_logs (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INT DEFAULT NULL,
    execution_id VARCHAR(36) NOT NULL,
    skill_name VARCHAR(255) NOT NULL,
    handler VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL,
    input JSON,
    output JSON,
    error_message TEXT,
    duration_ms INT DEFAULT 0,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    INDEX idx_skill_log_tenant (tenant_id),
    INDEX idx_skill_log_execution (execution_id),
    FOREIGN KEY (execution_id) REFERENCES agent_skill_executions(id) ON DELETE CASCADE
);
```

### 5.4 Store Interface (N7: `*int32` Convention)

```go
// store/driver.go additions:
CreateSkillExecution(ctx context.Context, execution *SkillExecution) (*SkillExecution, error)
GetSkillExecution(ctx context.Context, find *FindSkillExecution) (*SkillExecution, error)
UpdateSkillExecution(ctx context.Context, execution *SkillExecution) error
ListPendingSkillExecutions(ctx context.Context) ([]*SkillExecution, error)
ClaimSkillExecution(ctx context.Context, id string, workerID string, leaseDuration time.Duration) (*SkillExecution, error)
ReleaseSkillClaim(ctx context.Context, id string) error
CreateSkillLog(ctx context.Context, log *SkillLog) error
ListSkillLogs(ctx context.Context, find *FindSkillLog) ([]*SkillLog, error)
```

---

## 6. State Machine (6 states)

Chat path: `running` → `completed`/`failed`/`stopped` (skips `created`/`pending`).
Detached path: `created` → `pending` → `running` → `completed`/`failed`/`stopped`.

---

## 7. Execution Loop Owner (D2/R2/I1)

### 7.1 Chat Path

```go
func (s *Service) processChatWithToolCalls(ctx context.Context, config *AudienceConfig, userMessage string, history []*store.Message) (*ChatResponse, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    execution, err := s.store.CreateSkillExecution(ctx, &store.SkillExecution{
        TenantID:       config.TenantID,
        ConversationID: uuid.New().String(),
        SkillGraph:     config.SkillGraph,
        Status:         "running",
        TriggerPath:    "chat",
        CheckpointData: make(map[string]any),
        CreatedAt:      time.Now(),
    })
    if err != nil {
        return nil, err
    }

    activeExecutions.Store(execution.ID, cancel)
    defer activeExecutions.Delete(execution.ID)

    for i := 0; i < 10; i++ {
        select {
        case <-ctx.Done():
            return nil, fmt.Errorf("execution cancelled: %w", ctx.Err())
        default:
        }

        resp, err := s.llmClient.CreateChatCompletion(ctx, req)
        if err != nil {
            return nil, err
        }

        for _, toolCall := range resp.Choices[0].Message.ToolCalls {
            select {
            case <-ctx.Done():
                return nil, fmt.Errorf("execution cancelled: %w", ctx.Err())
            default:
            }

            result, err := s.executeSkill(ctx, execution, toolCall)
            if err != nil {
                return nil, err
            }
            _ = result
        }
    }
    return nil, fmt.Errorf("max iterations exceeded")
}
```

### 7.2 Detached Path (I1: Tenant from claimed row)

```go
func (s *Service) executeDetachedPipeline(ctx context.Context, exec *store.SkillExecution, workerID string) {
    defer s.ReleaseSkillClaim(ctx, exec.ID, workerID)

    // I1: Seed tenant from claimed row into context
    ctx = context.WithValue(ctx, tenantContextKey, *exec.TenantID)

    // Register cancel func
    ctx, cancel := context.WithCancel(ctx)
    activeExecutions.Store(exec.ID, cancel)
    defer activeExecutions.Delete(exec.ID)
    defer cancel()

    // Execute tool-calling loop (same as chat path)
    // ...
}
```

### 7.3 Stop with Status Re-read (R3)

```go
func (s *Service) executeSkill(ctx context.Context, exec *store.SkillExecution, toolCall openrouter.ToolCall) (map[string]any, error) {
    // R3: Re-read status BEFORE any mutation
    fresh, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
    if err == nil && fresh.Status == "stopped" {
        return nil, fmt.Errorf("execution %s was stopped", exec.ID)
    }

    handler, ok := s.registry.Get(toolCall.Function.Name)
    if !ok {
        return nil, fmt.Errorf("unknown skill: %s", toolCall.Function.Name)
    }

    input := parseToolInput(toolCall.Function.Arguments)
    result, err := handler.Execute(ctx, input, exec.CheckpointData)
    if err != nil {
        return nil, err
    }

    exec.CheckpointData[toolCall.Function.Name] = result
    exec.UpdatedAt = time.Now()
    s.store.UpdateSkillExecution(ctx, exec)

    return result, nil
}
```

---

## 8. CEL Condition Evaluation (R1/D4)

### 8.1 Fixed cel-go API

```go
// D4: Declare ALL node names from SkillGraph, not just checkpoint keys
func newCLEEnv(graph *SkillGraph) (*cel.Env, error) {
    opts := []cel.EnvOption{
        cel.Variable("checkpoint", cel.MapType(cel.StringType, cel.DynType)),
    }
    for name := range graph.Nodes {
        opts = append(opts, cel.Variable(name, cel.DynType))
    }
    return cel.NewEnv(opts...)
}

func compileCLE(expr string, graph *SkillGraph) (*cel.Program, error) {
    env, err := newCLEEnv(graph)
    if err != nil {
        return nil, err
    }
    ast, issues := env.Compile(expr)
    if issues != nil && issues.Err() != nil {
        return nil, fmt.Errorf("failed to compile %q: %w", expr, issues.Err())
    }
    prg, err := env.Program(ast)
    if err != nil {
        return nil, err
    }
    return prg, nil
}

// R1: prg.Eval(vars) directly — no cel.Vars()
func evaluateCondition(expr string, checkpointData map[string]any, graph *SkillGraph) (bool, error) {
    prg, err := compileCLE(expr, graph)
    if err != nil {
        return false, err
    }
    vars := map[string]any{"checkpoint": checkpointData}
    for key, val := range checkpointData {
        vars[key] = val
    }
    out, _, err := prg.Eval(vars)
    if err != nil {
        return false, err
    }
    return out.Value().(bool), nil
}
```

---

## 9. Outbound Events (D6)

Reuses existing `dispatchEvent` (service.go:5422). Document that without a configured webhook, the event is a no-op.

---

## 10. Recovery Worker (D5/R2)

```go
func (s *Service) startSkillRecoveryWorker() {
    if os.Getenv("SKILL_RECOVERY_ENABLED") != "true" {
        return
    }
    if !s.IsRAGEnabled() {  // M2: Use IsRAGEnabled()
        return
    }
    time.Sleep(10 * time.Second)
    workerID := fmt.Sprintf("worker-%s-%d", hostname, os.Getpid())

    for {
        execs, err := s.store.ListPendingSkillExecutions(context.Background())
        if err != nil {
            time.Sleep(30 * time.Second)
            continue
        }
        for _, exec := range execs {
            claimed, err := s.store.ClaimSkillExecution(context.Background(), exec.ID, workerID, 5*time.Minute)
            if err != nil || claimed == nil {
                continue
            }
            go s.executeDetachedPipeline(context.Background(), claimed, workerID)
        }
        time.Sleep(60 * time.Second)
    }
}
```

---

## 11. API Endpoints (R6/M1)

```go
// M1: Use getTenantOrFail, not h.getTenant
func (h *Handler) StartWorkflow(c echo.Context) error {
    ctx := c.Request().Context()
    tenant, err := getTenantOrFail(ctx, h.store, c)
    if err != nil {
        return err
    }
    if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
        return echo.NewHTTPError(http.StatusForbidden, "permission denied")
    }
    // ...
}

func (h *Handler) StopExecution(c echo.Context) error {
    ctx := c.Request().Context()
    tenant, err := getTenantOrFail(ctx, h.store, c)
    if err != nil {
        return err
    }
    if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) {
        return echo.NewHTTPError(http.StatusForbidden, "permission denied")
    }
    // ...
}
```

---

## 12. Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Extend parser.go: `@skill`/`@trigger`/`@signal` extraction (D1/R4)
- [ ] Add `LineStart` to `annotationBlock`
- [ ] Create migration files (4 versioned dirs + LATEST.sql)
- [ ] Build SkillGraph parsing and validation
- [ ] Add store interface methods + implementations

### Phase 2: Execution Engine
- [ ] Build SkillRegistry
- [ ] CEL evaluation with real cel-go API (R1/D4)
- [ ] Checkpoint/resume with claim model
- [ ] Context cancellation for stop (R3)

### Phase 3: Integration
- [ ] Tool-calling loop in `generateResponse` (D2)
- [ ] Skill definitions in `buildSystemPrompt` via `LoadConfig` (D9)
- [ ] Tenant scoping (I1)
- [ ] API endpoints with RBAC (R6/M1)
- [ ] Outbound events via `dispatchEvent` (D6)
- [ ] Recovery worker (D5/R2)

### Phase 4: Testing
- [ ] Unit tests for parser, state machine, CEL
- [ ] Integration tests with mock LLM
- [ ] Simulation framework extension

---

**This plan is ready for implementation.** All design gaps closed against real codebase seams.
