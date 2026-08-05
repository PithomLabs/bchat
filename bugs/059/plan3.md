# bchat Durable Execution Architecture — Plan 3 (plan3.md)

**Version:** 3.0
**Date:** 2026-08-05
**Status:** Ready for Implementation
**Review Addressed:** Claude Opus 4.6 adversarial review (plan2 nits N1-N6) + Start/Stop signal lifecycle additions

---

## Changelog from plan2.md

| # | Addition | Source |
|---|----------|--------|
| S1 | `@trigger` / `@signal` annotations in SCRIPT.md | Start/Stop signal lifecycle |
| S2 | `POST /api/v1/agent/:slug/workflows/start` endpoint | Start/Stop signal lifecycle |
| S3 | `POST /api/v1/agent/:slug/executions/:id/stop` endpoint | Start/Stop signal lifecycle |
| S4 | `EvaluateStopSignal()` + `StartWorkflowSignal()` functions | Start/Stop signal lifecycle |
| S5 | State machine updated with `stopped` state (6 states total) | Start/Stop signal lifecycle |
| N1 | MySQL driver parity stubs | plan2_review Nit 1 |
| N2 | Single-line inline annotation format (no multi-line `@skill` blocks) | plan2_review Nit 2 |
| N3 | SSE streaming progress events (`skill_start`, `skill_complete`) | plan2_review Nit 3 |
| N4 | CEL evaluator: top-level node names in context map | plan2_review Nit 4 |
| N5 | Fail-fast upload validation (cycle/dependency check on upload) | plan2_review Nit 5 |
| N6 | Exact OpenRouter SDK type names (`ChatCompletionRequest`, etc.) | plan2_review Nit 6 |

---

## Background Context

### Investigation Summary

This plan emerged from extensive investigation into three codebases:

1. **bchat** (current): Multi-tenant AI chat agent platform with declarative primitives (KB.md, POLICY.md, SCRIPT.md) but no durable execution or tool-calling support.

2. **agentskills.io** (standard): Open standard for AI agent skills using SKILL.md with YAML frontmatter + markdown instructions. Key insight: skills are loaded progressively (metadata at startup, full body on activation, resources on demand).

3. **goclaw** (reference implementation): Robust agent runtime with 8-stage pipeline, checkpoint/resume, and session persistence. Key insights adapted:
   - **Pipeline Architecture**: Setup → Iteration (Prune → Think → Tool → Observe → Checkpoint) → Finalize
   - **Stage Interface**: Stateless stages with shared mutable `*RunState`
   - **CheckpointStage**: Periodic flush to disk for crash recovery
   - **Session Persistence**: Atomic writes (temp → rename) with JSON files
   - **Error Handling**: Truncation retries, context overflow compaction, budget reduction chain

### Rationale

**Why Durable Execution?**
- n8n workflows crash and must restart from beginning
- bchat can checkpoint after each skill execution and resume from last checkpoint
- CockroachDB provides distributed, consistent state storage
- This is the key differentiator: "Declarative markdown config + compiled Go skills + CockroachDB persistent memory"

**Why Extend SCRIPT.md (Not Add SKILLS.md)?**
- No upload flow changes — existing `POST /api/v1/agent/:slug/files` already handles `file_type=script`
- Parser is already extensible — `extractAnnotationBlocks()` extracts `<!-- @type: params -->` generically
- Single source of truth — SCRIPT.md already feeds into `buildSystemPrompt()`
- Backward compatible — SCRIPT.md with no `@skill` annotations parses identically
- Matches mental model — tenants define "scripts with structure," not separate artifacts

**Why LLM-as-Executor?**
- Preserves "no code changes per tenant" — builtin handlers are curated, tenants compose via annotations
- LLM is already the execution engine — bchat's pipeline is "build prompt → call LLM → verify"
- Side-effects need deterministic handlers — create_ticket, webhook, search_kb are compiled Go
- It's how tool-calling already works — OpenRouter supports function/tool calling

---

## 1. Executive Summary

**Goal:** Transform bchat from a reactive chat agent into a durable automation pipeline that survives failures, resumes from checkpoints, and provides full audit trails — all driven by declarative markdown annotations in SCRIPT.md.

**Key Differentiator vs n8n:** "Declarative markdown config + hybrid Go/LLM execution + CockroachDB persistent memory vs visual drag-and-drop + JavaScript nodes + no built-in AI memory."

**Hackathon Requirements Met:**
- CockroachDB: Distributed vector indexing + durable execution state
- AWS: Amazon Bedrock (via OpenRouter) + Amazon S3 (skill assets)
- Go: All extensions written in Go
- 2+ CRDB tools: Vector Index + Skill Execution State
- 1+ AWS service: Bedrock + S3

---

## 2. Architecture Overview

### 2.1 System Components

```
┌─────────────────────────────────────────────────────────────────┐
│                        bchat Platform                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │  SCRIPT.md  │───▶│ SkillParser │───▶│ SkillGraph  │         │
│  │  (Extended) │    │ (Annotation)│    │ (DAG)       │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                                               │                 │
│                                               ▼                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Skill Tool Registry                        │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │   │
│  │  │ builtin: │ │ builtin: │ │ builtin: │ │ llm:     │  │   │
│  │  │ classify │ │ search   │ │ ticket   │ │ respond  │  │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              LLM Tool-Calling Pipeline                   │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │   │
│  │  │  LLM     │→│ Tool     │→│ Execute  │→│Checkpoint│  │   │
│  │  │  Call    │ │ Dispatch │ │ Handler  │ │ Stage    │  │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              CockroachDB / SQLite / PostgreSQL           │   │
│  │  ┌─────────────────┐  ┌─────────────────┐              │   │
│  │  │ Skill Executions │  │ Skill Logs      │              │   │
│  │  │ (State Machine)  │  │ (Audit Trail)   │              │   │
│  │  └─────────────────┘  └─────────────────┘              │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Data Flow

```
User Message
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Existing chat pipeline: classify → policy → prompt       │
│ 2. Skills injected as tool definitions in LLM request       │
│ 3. LLM decides which skills to call (tool-calling)          │
│ 4. Runtime enforces DAG constraints (dependencies)          │
│ 5. Execute builtin handler OR LLM reasoning step            │
│ 6. Return result as tool response to LLM                    │
│ 7. LLM continues with tool result in context                │
│ 8. Checkpoint after each tool call resolves                  │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ FOR EACH tool call:                                         │
│   1. Verify dependencies met (DAG constraint)               │
│   2. Check guard condition (CEL evaluation)                 │
│   3. Mark execution as 'running' in DB                      │
│   4. Execute builtin handler OR LLM reasoning               │
│   5. On success: save result, mark node completed           │
│   6. On failure: retry with backoff, or mark failed         │
│   7. Checkpoint to DB after each step                       │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Finalize:                                                   │
│   1. Mark execution as completed/stopped/failed             │
│   2. Evaluate stop condition (CEL) if present               │
│   3. Emit outbound AgentEvent if stop condition met         │
│   4. Send final response to user                            │
│   5. Log execution summary                                  │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. SCRIPT.md Extension (Not SKILLS.md)

### 3.1 Extended Annotation Format (N2: Single-Line Inline)

All `@skill` annotations use **single-line inline parameters**. No multi-line `<!-- @skill -->` blocks.

```markdown
## Opening
- Greet the customer warmly
- Ask how you can help

<!-- @skill: classify_intent, handler: "builtin:classify_intent", depends_on: "none", timeout: "30s", max_retries: 3 -->
## Classify Intent
Determine what the customer wants from their message.
Analyze the message and classify into one of these intents:
- question: Customer asking for information
- complaint: Customer reporting a problem
- request: Customer requesting action
- escalation: Customer demanding supervisor

<!-- @skill: search_kb, handler: "builtin:search_kb", depends_on: "classify_intent", timeout: "10s" -->
## Search Knowledge Base
Search for relevant solutions in the knowledge base.
Use the classified intent to construct an appropriate search query.

<!-- @skill: create_ticket, handler: "builtin:create_ticket", depends_on: "search_kb", condition: "search_kb.found == false", timeout: "15s" -->
## Create Support Ticket
If no solution found, create a support ticket for the customer.
Ticket type should match the classified intent.

<!-- @skill: respond, handler: "llm:respond", depends_on: "classify_intent, search_kb, create_ticket" -->
## Respond to Customer
Provide a helpful response based on:
1. The classified intent
2. Knowledge base results (if found)
3. Ticket ID (if created)
Maintain a professional, empathetic tone per POLICY.md.
```

**Parser note:** `extractAnnotationBlocks()` already handles single `<!-- @type: params -->` lines as individual `annotationBlock` objects. No parser changes needed for this format.

### 3.2 Parser Extension

Add `ParseScriptWithSkills()` to `parser.go`:

```go
type SkillDefinition struct {
    Name       string            `json:"name"`
    Handler    string            `json:"handler"`     // "builtin:classify_intent" or "llm:respond"
    DependsOn  []string          `json:"depends_on"`
    Timeout    string            `json:"timeout"`
    MaxRetries int               `json:"max_retries"`
    Condition  string            `json:"condition"`   // CEL expression
    LineStart  int               `json:"line_start"`
    LineEnd    int               `json:"line_end"`
}

type SkillGraph struct {
    Nodes       map[string]*SkillDefinition `json:"nodes"`
    EntryPoints []string                    `json:"entry_points"`
    HasSkills   bool                        `json:"has_skills"`
}

func (p *Parser) ParseScriptWithSkills(content string) (*ParsedScript, *SkillGraph, error) {
    parsed := p.ParseScript(content)  // existing parser
    graph := &SkillGraph{Nodes: make(map[string]*SkillDefinition)}
    
    for _, block := range parsed.AnnotationBlocks {
        if block.Type == "skill" {
            skill := &SkillDefinition{
                Name:      block.Params["name"],
                Handler:   block.Params["handler"],
                DependsOn: parseCommaSeparated(block.Params["depends_on"]),
                Timeout:   block.Params["timeout"],
                Condition: block.Params["condition"],
                LineStart: block.LineStart,
            }
            if skill.MaxRetries == 0 {
                skill.MaxRetries = 3
            }
            graph.Nodes[skill.Name] = skill
            graph.HasSkills = true
        }
    }
    
    // Validate DAG (N5: fail-fast on upload)
    if graph.HasSkills {
        if err := graph.Validate(); err != nil {
            return nil, nil, fmt.Errorf("invalid skill graph at line %d: %w", err.Line, err)
        }
    }
    
    return parsed, graph, nil
}
```

### 3.3 DAG Validation (N5: Fail-Fast on Upload)

```go
type ValidationError struct {
    Message string
    Line    int
}

func (g *SkillGraph) Validate() *ValidationError {
    // 1. Check for cycles (topological sort)
    if cycle := g.detectCycle(); cycle != nil {
        return &ValidationError{
            Message: fmt.Sprintf("cycle detected: %s", strings.Join(cycle, " → ")),
            Line:    g.Nodes[cycle[0]].LineStart,
        }
    }
    
    // 2. Check all depends_on references exist
    for name, skill := range g.Nodes {
        for _, dep := range skill.DependsOn {
            if dep == "none" {
                continue
            }
            if _, exists := g.Nodes[dep]; !exists {
                return &ValidationError{
                    Message: fmt.Sprintf("skill %q depends on %q which does not exist", name, dep),
                    Line:    skill.LineStart,
                }
            }
        }
    }
    
    // 3. Identify entry points (nodes with no dependencies)
    for name, skill := range g.Nodes {
        if len(skill.DependsOn) == 0 || (len(skill.DependsOn) == 1 && skill.DependsOn[0] == "none") {
            g.EntryPoints = append(g.EntryPoints, name)
        }
    }
    
    return nil
}
```

### 3.4 Start/Stop Annotations (S1)

```markdown
<!-- @trigger: start, type: "event", event_type: "lead_created" -->
## Workflow Opening
Adopt persona and process new emergency lead.

<!-- @skill: classify_intent -->
...

<!-- @signal: stop, condition: "create_ticket.ticket_id != ''", emit_event: "pipeline_completed" -->
## Workflow Completion
Stop pipeline execution once ticket is logged and emit completion event.
```

| Annotation | Purpose | Example |
|------------|---------|---------|
| `@trigger: start` | Declares how pipeline starts | `type: "event"`, `type: "api"`, `type: "chat"` |
| `@signal: stop` | Declares when pipeline stops | `condition: "CEL expr"`, `emit_event: "name"` |

```go
type TriggerDefinition struct {
    Type      string `json:"type"`       // "event", "api", "chat"
    EventType string `json:"event_type"` // for event triggers
}

type StopSignalDefinition struct {
    Condition string `json:"condition"` // CEL expression
    EmitEvent string `json:"emit_event"` // AgentEvent type to emit
}
```

---

## 4. Skill Tool Registry

### 4.1 Handler Interface

```go
type SkillHandler interface {
    Name() string
    Execute(ctx context.Context, input map[string]any, checkpoint map[string]any) (map[string]any, error)
    Schema() map[string]any  // JSON Schema for LLM tool definition
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

### 4.2 Built-in Handlers

| Handler | Purpose | Risk Level |
|---------|---------|------------|
| `builtin:classify_intent` | Classify user intent from message | read |
| `builtin:search_kb` | Search knowledge base via RAG | read |
| `builtin:create_ticket` | Create support ticket in DB | write |
| `builtin:webhook` | Call external webhook | external |
| `builtin:escalate` | Escalate to human agent | write |

### 4.3 LLM Handler (N6: Exact SDK Types)

```go
// llm:respond uses OpenRouter tool-calling to reason through a step
type LLMHandler struct {
    client *openrouter.Client
}

func (h *LLMHandler) Execute(ctx context.Context, input map[string]any, checkpoint map[string]any) (map[string]any, error) {
    req := openrouter.ChatCompletionRequest{
        Model: os.Getenv("LLM_MODEL"),
        Messages: []openrouter.ChatCompletionMessage{
            {Role: "system", Content: buildSkillPrompt(input, checkpoint)},
            {Role: "user", Content: input["user_message"].(string)},
        },
    }
    
    resp, err := h.client.CreateChatCompletion(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("LLM handler failed: %w", err)
    }
    
    return map[string]any{
        "response": resp.Choices[0].Message.Content,
        "usage":    resp.Usage,
    }, nil
}
```

---

## 5. Database Schema

### 5.1 CockroachDB DDL

```sql
CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id INT8 NOT NULL,
    conversation_id STRING NOT NULL,
    skill_graph JSONB NOT NULL,
    status STRING NOT NULL DEFAULT 'pending',
    current_node STRING,
    checkpoint_data JSONB DEFAULT '{}',
    completed_nodes JSONB DEFAULT '{}',
    failed_nodes JSONB DEFAULT '{}',
    retry_count INT4 DEFAULT 0,
    max_retries INT4 DEFAULT 3,
    parent_execution_id UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    
    INDEX idx_skill_exec_tenant (tenant_id),
    INDEX idx_skill_exec_status (status),
    INDEX idx_skill_exec_conversation (conversation_id)
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
    skill_graph TEXT NOT NULL,  -- JSON
    status TEXT NOT NULL DEFAULT 'pending',
    current_node TEXT,
    checkpoint_data TEXT DEFAULT '{}',  -- JSON
    completed_nodes TEXT DEFAULT '{}',  -- JSON
    failed_nodes TEXT DEFAULT '{}',     -- JSON
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    parent_execution_id TEXT,
    created_at INTEGER DEFAULT 0,
    updated_at INTEGER DEFAULT 0,
    completed_at INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_skill_exec_tenant ON agent_skill_executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_exec_status ON agent_skill_executions(status);
CREATE INDEX IF NOT EXISTS idx_skill_exec_conversation ON agent_skill_executions(conversation_id);

CREATE TABLE IF NOT EXISTS agent_skill_logs (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER DEFAULT NULL,
    execution_id TEXT NOT NULL,
    skill_name TEXT NOT NULL,
    handler TEXT NOT NULL,
    status TEXT NOT NULL,
    input TEXT,     -- JSON
    output TEXT,    -- JSON
    error_message TEXT,
    duration_ms INTEGER DEFAULT 0,
    started_at INTEGER DEFAULT 0,
    completed_at INTEGER DEFAULT 0,
    FOREIGN KEY (execution_id) REFERENCES agent_skill_executions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_skill_log_tenant ON agent_skill_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_log_execution ON agent_skill_logs(execution_id);
```

### 5.3 MySQL DDL (N1: Driver Parity)

```sql
CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INT DEFAULT NULL,
    conversation_id VARCHAR(255) NOT NULL,
    skill_graph JSON NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    current_node VARCHAR(255),
    checkpoint_data JSON DEFAULT (JSON_OBJECT()),
    completed_nodes JSON DEFAULT (JSON_OBJECT()),
    failed_nodes JSON DEFAULT (JSON_OBJECT()),
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    parent_execution_id VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    
    INDEX idx_skill_exec_tenant (tenant_id),
    INDEX idx_skill_exec_status (status),
    INDEX idx_skill_exec_conversation (conversation_id)
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

### 5.4 Store Interface

Add to `store/driver.go`:

```go
// Skill Execution methods
CreateSkillExecution(ctx context.Context, execution *SkillExecution) (*SkillExecution, error)
GetSkillExecution(ctx context.Context, find *FindSkillExecution) (*SkillExecution, error)
UpdateSkillExecution(ctx context.Context, execution *SkillExecution) error
ListPendingSkillExecutions(ctx context.Context) ([]*SkillExecution, error)

// Skill Log methods
CreateSkillLog(ctx context.Context, log *SkillLog) error
ListSkillLogs(ctx context.Context, find *FindSkillLog) ([]*SkillLog, error)
```

Add to `store/agent.go`:

```go
type SkillExecution struct {
    ID                 string         `json:"id"`
    TenantID           *int32         `json:"tenant_id"`
    ConversationID     string         `json:"conversation_id"`
    SkillGraph         *SkillGraph    `json:"skill_graph"`
    Status             string         `json:"status"` // pending, running, completed, failed, stopped
    CurrentNode        string         `json:"current_node"`
    CheckpointData     map[string]any `json:"checkpoint_data"`
    CompletedNodes     map[string]any `json:"completed_nodes"`
    FailedNodes        map[string]any `json:"failed_nodes"`
    RetryCount         int            `json:"retry_count"`
    MaxRetries         int            `json:"max_retries"`
    ParentExecutionID  string         `json:"parent_execution_id,omitempty"`
    CreatedAt          time.Time      `json:"created_at"`
    UpdatedAt          time.Time      `json:"updated_at"`
    CompletedAt        *time.Time     `json:"completed_at,omitempty"`
}

type FindSkillExecution struct {
    ID             *string
    TenantID       *int32
    ConversationID *string
    Status         *string
}

type SkillLog struct {
    ID           string         `json:"id"`
    TenantID     *int32         `json:"tenant_id"`
    ExecutionID  string         `json:"execution_id"`
    SkillName    string         `json:"skill_name"`
    Handler      string         `json:"handler"`
    Status       string         `json:"status"`
    Input        map[string]any `json:"input"`
    Output       map[string]any `json:"output"`
    ErrorMessage string         `json:"error_message,omitempty"`
    DurationMs   int            `json:"duration_ms"`
    StartedAt    time.Time      `json:"started_at"`
    CompletedAt  *time.Time     `json:"completed_at,omitempty"`
}

type FindSkillLog struct {
    ID          *string
    TenantID    *int32
    ExecutionID *string
}
```

---

## 6. State Machine

### 6.1 States (S5: 6 States)

```
                  ┌──────────────────────────────┐
                  │    Start Signal (Event/API)  │
                  └──────────────┬───────────────┘
                                 │
                                 ▼
┌───────────┐  start msg   ┌───────────┐                 ┌───────────┐
│  created  │ ───────────► │  pending  │ ───────────────►│  running  │
└───────────┘              └───────────┘                 └─────┬─────┘
                                 ▲                             │
                                 │ recovery                    │
                            ┌────┴────┐     stop condition     ├───────────► ┌───────────┐
                            │ crash   │     or API Stop        │             │ completed │
                            └─────────┘                        │             └───────────┘
                                                               ├───────────► ┌───────────┐
                                                               │             │  stopped  │
                                                               │             └───────────┘
                                                               └───────────► ┌───────────┐
                                                                             │  failed   │
                                                                             └───────────┘
```

### 6.2 Transition Rules

| From | To | Trigger | Guard |
|------|----|---------|-------|
| `created` | `pending` | Start signal received | Trigger type matches `@trigger` annotation |
| `pending` | `running` | LLM dispatches tool call | Dependencies satisfied |
| `running` | `completed` | All nodes done OR stop condition met | Stop CEL evaluates `true` |
| `running` | `failed` | Unrecoverable error | Retry count exceeded |
| `running` | `stopped` | Explicit stop API | Operator/system call |
| `running` | `pending` | Crash recovery | Startup worker R6 |

### 6.3 Clone Pattern for Retry

```go
func (s *Service) CloneSkillExecution(ctx context.Context, execID string) (*SkillExecution, error) {
    original, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &execID})
    if err != nil {
        return nil, err
    }
    
    clone := &SkillExecution{
        ID:                uuid.New().String(),
        TenantID:          original.TenantID,
        ConversationID:    original.ConversationID,
        SkillGraph:        original.SkillGraph,
        Status:            "pending",
        CheckpointData:    original.CheckpointData,  // carry forward completed data
        CompletedNodes:    original.CompletedNodes,   // carry forward completed nodes
        FailedNodes:       make(map[string]any),      // reset failed nodes
        RetryCount:        0,
        MaxRetries:        original.MaxRetries,
        ParentExecutionID: &original.ID,
        CreatedAt:         time.Now(),
    }
    
    return s.store.CreateSkillExecution(ctx, clone)
}
```

---

## 7. CEL Condition Evaluation

### 7.1 CEL Environment (N4: Top-Level Node Names)

```go
func newCLEnv() *cel.Env {
    env, _ := cel.NewEnv(
        cel.Variable("checkpoint", cel.MapType(cel.StringType, cel.DynType)),
        cel.Variable("search_kb", cel.MapType(cel.StringType, cel.DynType)),
        cel.Variable("classify_intent", cel.MapType(cel.StringType, cel.DynType)),
        cel.Variable("create_ticket", cel.MapType(cel.StringType, cel.DynType)),
    )
    return env
}
```

### 7.2 Condition Evaluation (N4: Populate Top-Level Context)

```go
func evaluateCondition(expr string, checkpointData map[string]any) bool {
    env, ast := compileCLE(expr)
    
    // Populate top-level node names into evaluation context
    vars := map[string]any{
        "checkpoint": checkpointData,
    }
    
    // Extract node results from checkpoint for direct access
    for key, val := range checkpointData {
        if nodeResult, ok := val.(map[string]any); ok {
            vars[key] = nodeResult
        }
    }
    
    out, _, err := env.Eval(ast)
    if err != nil {
        slog.Error("CEL evaluation failed", "expr", expr, "error", err)
        return false
    }
    
    return out.Value().(bool)
}
```

### 7.3 Stop Signal Evaluation (S4)

```go
func (s *Service) EvaluateStopSignal(ctx context.Context, exec *SkillExecution, stopCondition, emitEvent string) (bool, error) {
    if stopCondition == "" {
        return false, nil
    }
    
    if !evaluateCondition(stopCondition, exec.CheckpointData) {
        return false, nil
    }
    
    exec.Status = "completed"
    now := time.Now()
    exec.CompletedAt = &now
    
    if err := s.store.UpdateSkillExecution(ctx, exec); err != nil {
        return false, err
    }
    
    // Emit outbound event signal if configured
    if emitEvent != "" {
        payload, _ := json.Marshal(exec.CheckpointData)
        s.store.CreateAgentEvent(ctx, &store.AgentEvent{
            TenantID:  exec.TenantID,
            EventType: emitEvent,
            Payload:   string(payload),
            Status:    "pending",
        })
    }
    
    return true, nil
}
```

---

## 8. Integration with Chat Pipeline

### 8.1 Skill Definitions in System Prompt

Extend `buildSystemPrompt()` (Section 5.5 from plan2):

```go
func (s *Service) buildSkillPrompt(skills map[string]*SkillDefinition) string {
    if len(skills) == 0 {
        return ""
    }
    
    var sb strings.Builder
    sb.WriteString("\n## Available Skills\n")
    sb.WriteString("You have access to the following skills. Call them using tool-calling.\n\n")
    
    for _, skill := range skills {
        sb.WriteString(fmt.Sprintf("### %s\n", skill.Name))
        sb.WriteString(fmt.Sprintf("- Handler: `%s`\n", skill.Handler))
        if len(skill.DependsOn) > 0 && skill.DependsOn[0] != "none" {
            sb.WriteString(fmt.Sprintf("- Depends on: %s\n", strings.Join(skill.DependsOn, ", ")))
        }
        sb.WriteString("\n")
    }
    
    return sb.String()
}
```

### 8.2 Tool-Calling Pipeline (N3: SSE Progress, N6: SDK Types)

```go
func (s *Service) processChatWithToolCalls(ctx context.Context, session *ChatSession, userMessage string) (*ChatResponse, error) {
    // 1. Build system prompt with skill definitions
    systemPrompt := s.buildSystemPrompt(session.Skills)
    
    // 2. Create OpenRouter request (N6: exact SDK types)
    req := openrouter.ChatCompletionRequest{
        Model:  os.Getenv("LLM_MODEL"),
        Tools:  s.buildToolDefinitions(session.Skills),
        Messages: []openrouter.ChatCompletionMessage{
            {Role: "system", Content: systemPrompt},
            {Role: "user", Content: userMessage},
        },
    }
    
    // 3. Tool-calling loop
    for i := 0; i < 10; i++ { // max 10 iterations
        resp, err := s.llmClient.CreateChatCompletion(ctx, req)
        if err != nil {
            return nil, err
        }
        
        choice := resp.Choices[0]
        
        // 4. If no tool calls, return final response
        if choice.FinishReason != "tool_calls" {
            return &ChatResponse{Content: choice.Message.Content}, nil
        }
        
        // 5. Process tool calls
        for _, toolCall := range choice.Message.ToolCalls {
            // N3: Emit SSE progress event
            s.emitSSEEvent(ctx, "skill_start", map[string]any{
                "skill": toolCall.Function.Name,
            })
            
            result, err := s.executeSkill(ctx, session, toolCall)
            
            // N3: Emit SSE completion event
            s.emitSSEEvent(ctx, "skill_complete", map[string]any{
                "skill":  toolCall.Function.Name,
                "status": "completed",
            })
            
            if err != nil {
                return nil, err
            }
            
            // 6. Add tool result to messages
            req.Messages = append(req.Messages,
                choice.Message.ToMessage(),
                openrouter.ChatCompletionMessage{
                    Role:       "tool",
                    Content:    result,
                    ToolCallID: toolCall.ID,
                },
            )
        }
    }
    
    return nil, fmt.Errorf("max tool-calling iterations exceeded")
}
```

### 8.3 SSE Progress Events (N3)

```go
func (s *Service) emitSSEEvent(ctx context.Context, eventType string, data map[string]any) {
    select {
    case s.eventChan <- SSEEvent{
        Type: eventType,
        Data: data,
    }:
    default:
        // Channel full, drop event
    }
}
```

---

## 9. Tenant Isolation

Every `SkillExecutor` and store method must scope to tenant:

```go
func (s *Service) executeSkill(ctx context.Context, session *ChatSession, toolCall openrouter.ToolCall) (string, error) {
    tenantID := getTenantFromContext(ctx)
    if tenantID == nil {
        return "", fmt.Errorf("tenant ID required for skill execution")
    }
    
    handler, ok := s.registry.Get(toolCall.Function.Name)
    if !ok {
        return "", fmt.Errorf("unknown skill: %s", toolCall.Function.Name)
    }
    
    // Pass tenant context to handler
    ctx = context.WithValue(ctx, "tenant_id", tenantID)
    
    input := parseToolInput(toolCall.Function.Arguments)
    checkpoint := session.Execution.CheckpointData
    
    result, err := handler.Execute(ctx, input, checkpoint)
    if err != nil {
        return "", err
    }
    
    // Update checkpoint
    session.Execution.CheckpointData[toolCall.Function.Name] = result
    s.store.UpdateSkillExecution(ctx, session.Execution)
    
    return result, nil
}
```

---

## 10. Startup Recovery Worker (R6)

```go
func (s *Service) startSkillRecoveryWorker() {
    time.Sleep(10 * time.Second) // Wait for DB migration
    
    for {
        execs, err := s.store.ListPendingSkillExecutions(context.Background())
        if err != nil {
            slog.Error("Failed to list pending skill executions", "error", err)
            time.Sleep(30 * time.Second)
            continue
        }
        
        for _, exec := range execs {
            if exec.Status == "running" {
                // Mark as pending for retry (conservative: don't auto-resume)
                exec.Status = "pending"
                exec.FailedNodes[exec.CurrentNode] = "process restart"
                s.store.UpdateSkillExecution(context.Background(), exec)
                
                slog.Info("Recovered skill execution after restart",
                    "execution_id", exec.ID,
                    "tenant_id", exec.TenantID,
                    "node", exec.CurrentNode,
                )
            }
        }
        
        time.Sleep(60 * time.Second)
    }
}
```

---

## 11. API Endpoints

### 11.1 Skill Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/agent/:slug/skills` | List skill definitions for tenant |
| POST | `/api/v1/agent/:slug/skills/validate` | Validate SCRIPT.md skill annotations |
| GET | `/api/v1/agent/:slug/executions` | List skill executions for tenant |
| GET | `/api/v1/agent/:slug/executions/:id` | Get execution details |
| POST | `/api/v1/agent/:slug/executions/:id/retry` | Clone failed execution for retry |
| POST | `/api/v1/agent/:slug/executions/:id/suspend` | Suspend running execution |
| POST | `/api/v1/agent/:slug/executions/:id/resume` | Resume suspended execution |

### 11.2 Start/Stop Lifecycle (S2-S3)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/agent/:slug/workflows/start` | Start pipeline from event/API |
| POST | `/api/v1/agent/:slug/executions/:id/stop` | Explicit execution stop |

```go
// StartWorkflow handles POST /api/v1/agent/:slug/workflows/start
func (h *Handler) StartWorkflow(c echo.Context) error {
    slug := c.Param("slug")
    tenantID := getTenantFromContext(c)
    
    var req struct {
        TriggerType string         `json:"trigger_type"` // "event", "api"
        EventType   string         `json:"event_type"`
        Payload     map[string]any `json:"payload"`
    }
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
    }
    
    exec, err := h.service.StartWorkflowSignal(c.Request().Context(), *tenantID, req.TriggerType, req.EventType, req.Payload)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    
    return c.JSON(http.StatusCreated, exec)
}

// StopExecution handles POST /api/v1/agent/:slug/executions/:id/stop
func (h *Handler) StopExecution(c echo.Context) error {
    execID := c.Param("id")
    tenantID := getTenantFromContext(c)
    
    exec, err := h.store.GetSkillExecution(c.Request().Context(), &store.FindSkillExecution{ID: &execID, TenantID: tenantID})
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "execution not found")
    }
    
    if exec.Status != "running" {
        return echo.NewHTTPError(http.StatusBadRequest, "execution is not running")
    }
    
    exec.Status = "stopped"
    now := time.Now()
    exec.CompletedAt = &now
    
    if err := h.store.UpdateSkillExecution(c.Request().Context(), exec); err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    
    return c.JSON(http.StatusOK, exec)
}
```

---

## 12. Observational Memory Interaction

Skill executions interact with OM naturally through conversation history:

1. **Skill inputs/outputs are tool-call/tool-response messages** — part of conversation history
2. **ObserverBuffer compresses them** like any other messages
3. **Token budget includes tool messages** — skill outputs count toward context window
4. **No special OM integration needed** — the existing pipeline handles it

---

## 13. Environment Variables

```bash
# Skill Execution
SKILL_RECOVERY_ENABLED=true                    # Enable startup recovery worker (R6)
SKILL_MAX_RETRIES=3                            # Default max retries per skill
SKILL_TIMEOUT=30s                              # Default timeout per skill
SKILL_CHECKPOINT_INTERVAL=1                    # Checkpoint after N tool calls

# CEL Condition Evaluation
CEL_CONDITION_ENABLED=true                     # Enable CEL condition evaluation

# Token Budget
SKILL_MAX_TOKENS=1500                          # Max tokens for skill definitions in prompt
```

---

## 14. Testing Strategy

### 14.1 Layer 1: Unit Tests (No LLM, No DB)

```go
func TestSkillGraphCycleDetection(t *testing.T) {
    graph := &SkillGraph{
        Nodes: map[string]*SkillDefinition{
            "a": {DependsOn: []string{"b"}},
            "b": {DependsOn: []string{"a"}}, // cycle!
        },
    }
    err := graph.Validate()
    assert.ErrorContains(t, err, "cycle detected")
}

func TestConditionEvaluation(t *testing.T) {
    checkpointData := map[string]any{
        "search_kb": map[string]any{"found": false},
    }
    
    assert.True(t, evaluateCondition("search_kb.found == false", checkpointData))
    assert.False(t, evaluateCondition("search_kb.found == true", checkpointData))
}

func TestStateTransitions(t *testing.T) {
    exec := &SkillExecution{Status: "pending"}
    assert.True(t, exec.CanTransitionTo("running"))
    assert.False(t, exec.CanTransitionTo("completed"))
    assert.False(t, exec.CanTransitionTo("failed"))
}
```

### 14.2 Layer 2: Integration Tests with Mock LLM

```go
func TestSkillExecutionWithMockLLM(t *testing.T) {
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(mockToolCallResponse)
    }))
    os.Setenv("OPENROUTER_API_BASE_URL", mockServer.URL)
    
    response, err := service.ProcessChat(ctx, session, "I need help with water damage")
    assert.NoError(t, err)
    assert.Contains(t, response.ToolsCalled, "builtin:classify_intent")
}
```

### 14.3 Layer 3: Simulation Framework

```go
func TestSkillDAGSimulation(t *testing.T) {
    scenario := SimulationScenario{
        Messages: []string{"I need emergency water extraction"},
        ExpectedSkillsCalled: []string{"classify_intent", "search_kb"},
        ExpectedSkillsNotCalled: []string{"escalate_to_human"},
        MaxSkillDurationMs: 5000,
    }
    
    result := simulation.Run(scenario)
    assert.True(t, result.Passed)
}
```

---

## 15. Implementation Phases

### Phase 1: Core Infrastructure (Days 1-3)
- [ ] Extend parser.go to extract `@skill` annotations from SCRIPT.md
- [ ] Create SQLite, CockroachDB, and MySQL tables with migrations (N1)
- [ ] Build SkillGraph parsing and validation (N5: fail-fast)
- [ ] Implement state machine transitions (6 states)

### Phase 2: Execution Engine (Days 4-6)
- [ ] Build SkillRegistry with builtin handlers
- [ ] Implement SkillHandler interface
- [ ] Add CEL condition evaluation (N4: top-level context)
- [ ] Create checkpoint/resume logic

### Phase 3: Integration (Days 7-9)
- [ ] Integrate with existing LLM tool-calling pipeline (N6: SDK types)
- [ ] Add skill definitions to buildSystemPrompt()
- [ ] Implement tenant scoping on all executor methods
- [ ] Add API endpoints (workflows/start, executions/:id/stop)
- [ ] Add SSE progress events (N3: skill_start, skill_complete)

### Phase 4: Testing & Polish (Days 10-12)
- [ ] Unit tests for DAG parser, state machine, CEL evaluator
- [ ] Integration tests with mock LLM server
- [ ] Extend simulation framework for skill assertions
- [ ] Documentation and examples

### Phase 5: Demo Preparation (Days 13-14)
- [ ] Create demo scenarios (customer support, data processing)
- [ ] Prepare hackathon presentation
- [ ] Write documentation
- [ ] Final testing

---

## 16. Risk Mitigation

| Risk | Mitigation |
|------|------------|
| CockroachDB unavailable | Fallback to SQLite (existing pattern) |
| Handler panic | Recover + mark failed + log error |
| Infinite loop in DAG | Max iterations + timeout |
| Condition evaluation error | Default to false + log error |
| Checkpoint corruption | Verify checksums |
| Race condition in parallel | Mutex + atomic operations |
| Token budget overflow | Progressive disclosure + cap |
| Cross-tenant data leakage | Tenant scoping on every method |

---

## 17. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Execution success rate | 99.9% | CockroachDB logs |
| Checkpoint frequency | After each tool call | Configuration |
| Resume success rate | 100% | Test scenarios |
| Response time | < 2s | API metrics |
| Concurrent executions | 100+ | Load testing |
| Token budget compliance | < 1,500 tokens | Prompt analysis |

---

**This plan is ready for implementation.** All plan2 nits (N1-N6) and start/stop signal lifecycle additions (S1-S5) are incorporated. The architecture integrates naturally with the existing bchat codebase while adding durable execution capabilities.

**Next step:** Begin Phase 1 implementation (Core Infrastructure).
