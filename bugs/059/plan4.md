# bchat Durable Execution Architecture — Plan 4 (plan4.md)

**Version:** 4.0
**Date:** 2026-08-05
**Status:** Approved — All design gaps closed
**Reviews Addressed:** plan2_review N1-N6, deepseek review D1-D9, start/stop lifecycle S1-S5

---

## Changelog from plan3.md

| # | Finding | Resolution |
|---|---------|------------|
| D1 | Start/stop signals have no data path | `ParseScriptWithSkills` extracts `@trigger`/`@signal` into `SkillGraph` |
| D2 | No owner for the execution loop | Sync chat + detached recovery worker model |
| D3 | Stop doesn't cancel in-flight work | `context.WithCancel` per execution, checkpoint-gate stop |
| D4 | Condition binding/failure policy | Binding contract, incomplete-node guard, fail-closed default |
| D5 | Recovery has no claim/lease | `claimed_at`/`attempts` lease on `agent_skill_executions` |
| D6 | Reinvents outbox | Reuse existing `dispatchEvent` + `ClaimPendingEvents` |
| D7 | SSE progress has no transport | Cut N3 for demo; document as scope cut |
| D8 | Multi-driver scope understated | 3 implementations, 4 deploy targets, 4 × LATEST.sql |
| D9 | Skill-graph lifecycle unspecified | Cached in `LoadConfig`/`configCache`, invalidated on upload |
| N7 | `*int32` TenantID convention | Enforce pointer convention on all new structs |
| N8 | RBAC for new endpoints | `tenant:admin` for `/workflows/start`, `api:config` for `/stop` |
| N9 | `depends_on` convention | Normalize to omit `none`; empty slice = entry point |
| N10 | `agent_workflows` name clash | Document distinction; use `agent_skill_executions` table |
| N11 | Recovery worker gating | Gate on `SKILL_RECOVERY_ENABLED` + RAG feature availability |

---

## 1. Executive Summary

**Goal:** Transform bchat from a reactive chat agent into a durable automation pipeline that survives failures, resumes from checkpoints, and provides full audit trails — all driven by declarative markdown annotations in SCRIPT.md.

**Key Differentiator vs n8n:** "Declarative markdown config + hybrid Go/LLM execution + CockroachDB persistent memory vs visual drag-and-drop + JavaScript nodes + no built-in AI memory."

**Execution Model (D2 — Closed):** Synchronous chat execution for `/chat/ext` and `/chat/int` requests; a detached background worker owns recovery, event-triggered, and API-triggered runs. This preserves the existing synchronous chat contract with zero client impact, while the recovery worker (§10) seeds the detached half.

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
│  │  (Extended) │    │ (Annotation)│    │ (DAG+Triggers)│       │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                              │                  │                │
│              ┌───────────────┘                  │                │
│              ▼                                  ▼                │
│  ┌──────────────────────┐    ┌─────────────────────────────┐   │
│  │   ConfigCache        │    │   Skill Tool Registry       │   │
│  │ (parsed graph cached │    │  ┌──────────┐ ┌──────────┐ │   │
│  │  by content-hash,    │    │  │ builtin: │ │ llm:     │ │   │
│  │  invalidated on      │    │  │ classify │ │ respond  │ │   │
│  │  upload)             │    │  │ search   │ │          │ │   │
│  └──────────────────────┘    │  │ ticket   │ │          │ │   │
│              │               │  └──────────┘ └──────────┘ │   │
│              ▼               └─────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              LLM Tool-Calling Pipeline                   │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │   │
│  │  │  LLM     │→│ Tool     │→│ Execute  │→│Checkpoint│  │   │
│  │  │  Call    │ │ Dispatch │ │ Handler  │ │ Stage    │  │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│              ┌───────────────┼───────────────┐                  │
│              ▼               ▼               ▼                  │
│  ┌─────────────────┐ ┌─────────────┐ ┌─────────────────────┐  │
│  │ Chat Path       │ │ Recovery    │ │ dispatchEvent       │  │
│  │ (sync, inline)  │ │ Worker      │ │ outbox (existing)   │  │
│  └─────────────────┘ └─────────────┘ └─────────────────────┘  │
│              │               │               │                  │
│              ▼               ▼               ▼                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  CockroachDB / SQLite / PostgreSQL / MySQL              │   │
│  │  ┌─────────────────┐  ┌─────────────────┐              │   │
│  │  │ Skill Executions │  │ Skill Logs      │              │   │
│  │  │ (State Machine)  │  │ (Audit Trail)   │              │   │
│  │  └─────────────────┘  └─────────────────┘              │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Data Flow — Chat Path (Synchronous)

```
User Message (POST /api/v1/agent/:slug/chat)
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. LoadConfig() → cached SkillGraph (D9)                    │
│ 2. buildSystemPrompt() → inject skill definitions           │
│ 3. LLM call with tool definitions                           │
│ 4. LLM decides which skills to call (tool-calling)          │
│ 5. Runtime enforces DAG constraints (dependencies)          │
│ 6. Execute builtin handler OR LLM reasoning step            │
│ 7. Check context cancellation (D3) before each node write   │
│ 8. Check stop condition (D4) after each step                │
│ 9. Return result as tool response to LLM                    │
│ 10. Checkpoint to DB after each step                        │
│ 11. LLM continues with tool result in context               │
│ 12. Repeat until LLM produces final response                │
└─────────────────────────────────────────────────────────────┘
```

### 2.3 Data Flow — Detached Path (Event/API/Cron Triggers)

```
External Event (AgentEvent) / API Call (/workflows/start) / Cron
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Create SkillExecution record (status: pending)           │
│ 2. Detached worker claims execution (D5: lease)             │
│ 3. Load cached SkillGraph for tenant (D9)                   │
│ 4. Execute tool-calling loop (same as chat path)            │
│ 5. Checkpoint after each step                               │
│ 6. On completion: evaluate stop condition (D4)              │
│ 7. Emit outbound event via dispatchEvent (D6)               │
│ 8. Release claim                                            │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. SCRIPT.md Extension

### 3.1 Extended Annotation Format (N2: Single-Line Inline, N9: No "none")

All `@skill` annotations use **single-line inline parameters**. No multi-line blocks.

```markdown
## Opening
- Greet the customer warmly
- Ask how you can help

<!-- @trigger: start, type: "chat" -->
## Workflow Opening
Adopt persona and process the conversation.

<!-- @skill: classify_intent, handler: "builtin:classify_intent", timeout: "30s", max_retries: 3 -->
## Classify Intent
Determine what the customer wants from their message.

<!-- @skill: search_kb, handler: "builtin:search_kb", depends_on: "classify_intent", timeout: "10s" -->
## Search Knowledge Base
Search for relevant solutions in the knowledge base.

<!-- @skill: create_ticket, handler: "builtin:create_ticket", depends_on: "search_kb", condition: "search_kb.found == false", timeout: "15s" -->
## Create Support Ticket
If no solution found, create a support ticket for the customer.

<!-- @skill: respond, handler: "llm:respond", depends_on: "classify_intent, search_kb, create_ticket" -->
## Respond to Customer
Provide a helpful response based on skill results.

<!-- @signal: stop, condition: "create_ticket.ticket_id != ''", emit_event: "pipeline_completed" -->
## Workflow Completion
Stop pipeline execution once ticket is logged.
```

**Convention (N9):** Entry points are nodes with an empty `depends_on` field (or no `depends_on` at all). The literal string `"none"` is no longer accepted.

### 3.2 Parser Extension (D1: Signal Data Path)

`ParseScriptWithSkills` extracts `@skill`, `@trigger`, and `@signal` blocks in a single pass. The parsed graph — including trigger/stop metadata — is stored per-tenant in the config cache (D9).

```go
type TriggerDefinition struct {
    Type      string `json:"type"`       // "event", "api", "chat"
    EventType string `json:"event_type"` // for event triggers
}

type StopSignalDefinition struct {
    Condition string `json:"condition"` // CEL expression
    EmitEvent string `json:"emit_event"` // AgentEvent type to emit on stop
}

type SkillGraph struct {
    Nodes       map[string]*SkillDefinition `json:"nodes"`
    EntryPoints []string                    `json:"entry_points"`
    Trigger     *TriggerDefinition          `json:"trigger,omitempty"`
    Stop        *StopSignalDefinition       `json:"stop,omitempty"`
    HasSkills   bool                        `json:"has_skills"`
    ContentHash string                      `json:"content_hash"` // for cache invalidation
}

func (p *Parser) ParseScriptWithSkills(content string) (*ParsedScript, *SkillGraph, error) {
    parsed := p.ParseScript(content)
    graph := &SkillGraph{Nodes: make(map[string]*SkillDefinition)}
    graph.ContentHash = sha256Hex(content)

    for _, block := range parsed.AnnotationBlocks {
        switch block.Type {
        case "skill":
            skill := &SkillDefinition{
                Name:       block.Params["name"],
                Handler:    block.Params["handler"],
                DependsOn:  parseCommaSeparated(block.Params["depends_on"]),
                Timeout:    block.Params["timeout"],
                Condition:  block.Params["condition"],
                LineStart:  block.LineStart,
            }
            if skill.MaxRetries == 0 {
                skill.MaxRetries = 3
            }
            graph.Nodes[skill.Name] = skill
            graph.HasSkills = true

        case "trigger":
            graph.Trigger = &TriggerDefinition{
                Type:      block.Params["type"],
                EventType: block.Params["event_type"],
            }

        case "signal":
            if block.Params["condition"] != "" {
                graph.Stop = &StopSignalDefinition{
                    Condition: block.Params["condition"],
                    EmitEvent: block.Params["emit_event"],
                }
            }
        }
    }

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

Fail-fast is enforced at upload time in `HandleImportScript` (`handlers.go`):

```go
// In HandleImportScript, after parsing:
parsed, graph, err := p.ParseScriptWithSkills(content)
if err != nil {
    return echo.NewHTTPError(http.StatusBadRequest, err.Error())
}
if graph != nil && graph.HasSkills {
    if valErr := graph.Validate(); valErr != nil {
        return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
            "SCRIPT.md skill validation failed at line %d: %s", valErr.Line, valErr.Message))
    }
}
```

---

## 4. Skill Tool Registry

### 4.1 Handler Interface

```go
type SkillHandler interface {
    Name() string
    Execute(ctx context.Context, input map[string]any, checkpoint map[string]any) (map[string]any, error)
    Schema() map[string]any // JSON Schema for LLM tool definition
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

func (r *SkillRegistry) List() map[string]SkillHandler {
    r.mu.RLock()
    defer r.mu.RUnlock()
    out := make(map[string]SkillHandler, len(r.handlers))
    for k, v := range r.handlers {
        out[k] = v
    }
    return out
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
    claimed_at TIMESTAMPTZ,                          -- D5: lease timestamp
    claimed_by STRING,                               -- D5: worker instance ID
    claim_expires_at TIMESTAMPTZ,                    -- D5: lease expiry
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    INDEX idx_skill_exec_tenant (tenant_id),
    INDEX idx_skill_exec_status (status),
    INDEX idx_skill_exec_conversation (conversation_id),
    INDEX idx_skill_exec_claim (status, claimed_at)  -- D5: for claim query
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
    claimed_at INTEGER DEFAULT 0,         -- D5: unix epoch
    claimed_by TEXT,                      -- D5: worker instance ID
    claim_expires_at INTEGER DEFAULT 0,   -- D5: unix epoch
    created_at INTEGER DEFAULT 0,
    updated_at INTEGER DEFAULT 0,
    completed_at INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_skill_exec_tenant ON agent_skill_executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_exec_status ON agent_skill_executions(status);
CREATE INDEX IF NOT EXISTS idx_skill_exec_conversation ON agent_skill_executions(conversation_id);
CREATE INDEX IF NOT EXISTS idx_skill_exec_claim ON agent_skill_executions(status, claimed_at);

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

### 5.3 MySQL DDL

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
    claimed_at TIMESTAMP NULL,                          -- D5
    claimed_by VARCHAR(255),                            -- D5
    claim_expires_at TIMESTAMP NULL,                    -- D5
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,

    INDEX idx_skill_exec_tenant (tenant_id),
    INDEX idx_skill_exec_status (status),
    INDEX idx_skill_exec_conversation (conversation_id),
    INDEX idx_skill_exec_claim (status, claimed_at)
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

Add to `store/driver.go`:

```go
CreateSkillExecution(ctx context.Context, execution *SkillExecution) (*SkillExecution, error)
GetSkillExecution(ctx context.Context, find *FindSkillExecution) (*SkillExecution, error)
UpdateSkillExecution(ctx context.Context, execution *SkillExecution) error
ListPendingSkillExecutions(ctx context.Context) ([]*SkillExecution, error)
ClaimSkillExecution(ctx context.Context, id string, workerID string, leaseDuration time.Duration) (*SkillExecution, error)
ReleaseSkillClaim(ctx context.Context, id string) error

CreateSkillLog(ctx context.Context, log *SkillLog) error
ListSkillLogs(ctx context.Context, find *FindSkillLog) ([]*SkillLog, error)
```

Add to `store/agent.go`:

```go
type SkillExecution struct {
    ID                 string         `json:"id"`
    TenantID           *int32         `json:"tenant_id"`           // N7: pointer convention
    ConversationID     string         `json:"conversation_id"`
    SkillGraph         *SkillGraph    `json:"skill_graph"`
    Status             string         `json:"status"`
    CurrentNode        string         `json:"current_node"`
    CheckpointData     map[string]any `json:"checkpoint_data"`
    CompletedNodes     map[string]any `json:"completed_nodes"`
    FailedNodes        map[string]any `json:"failed_nodes"`
    RetryCount         int            `json:"retry_count"`
    MaxRetries         int            `json:"max_retries"`
    ParentExecutionID  *string        `json:"parent_execution_id,omitempty"` // N7: pointer
    ClaimedAt          *time.Time     `json:"claimed_at,omitempty"`         // D5
    ClaimedBy          *string        `json:"claimed_by,omitempty"`         // D5
    ClaimExpiresAt     *time.Time     `json:"claim_expires_at,omitempty"`   // D5
    CreatedAt          time.Time      `json:"created_at"`
    UpdatedAt          time.Time      `json:"updated_at"`
    CompletedAt        *time.Time     `json:"completed_at,omitempty"`
}

type FindSkillExecution struct {
    ID             *string
    TenantID       *int32   // N7: pointer
    ConversationID *string
    Status         *string
}

type SkillLog struct {
    ID           string         `json:"id"`
    TenantID     *int32         `json:"tenant_id"`     // N7: pointer
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
    TenantID    *int32   // N7: pointer
    ExecutionID *string
}
```

### 5.5 Multi-Driver Parity (D8)

**3 driver implementations, 4 deployment targets:**

| Implementation | Deploy Targets |
|----------------|----------------|
| `store/db/sqlite/` | SQLite (local dev) |
| `store/db/postgres/` | PostgreSQL + CockroachDB (CRDB reuses postgres driver) |
| `store/db/mysql/` | MySQL |

**Migration files required (4 × versioned SQL + 4 × LATEST.sql):**

```
store/migration/sqlite/<version>/NN__add_skill_executions.sql
store/migration/postgres/<version>/NN__add_skill_executions.sql
store/migration/mysql/<version>/NN__add_skill_executions.sql
# CockroachDB: single LATEST.sql file
```

**Validation:**
```bash
task validate:parity   # cross-driver schema + file-list parity
task validate:schema   # schema validation tests
```

### 5.6 Skill-Graph Lifecycle (D9: Cached, Invalidated on Upload)

The per-tenant `SkillGraph` is a **derived, cached artifact** of SCRIPT.md content. It rides the existing `LoadConfig`/`configCache` path:

```go
// In LoadConfig, after parsing SCRIPT.md:
func (s *Service) LoadConfig(ctx context.Context, tenantSlug, audienceType string) (*AudienceConfig, error) {
    if config := s.configCache.Get(tenantSlug, audienceType); config != nil {
        return config, nil
    }
    // ... existing config loading ...
    
    // Parse skill graph from SCRIPT.md
    if script != nil {
        _, skillGraph, err := s.parser.ParseScriptWithSkills(script.Content)
        if err == nil && skillGraph != nil && skillGraph.HasSkills {
            config.SkillGraph = skillGraph
        }
    }
    
    s.configCache.Set(config)
    return config, nil
}
```

**Invalidation:** `HandleImportScript` already calls `configCache.Invalidate(tenant.Slug)` after upload. The new skill graph is parsed and cached on the next `LoadConfig` call.

**Per-execution rows** in `agent_skill_executions.skill_graph` store a **snapshot** of the graph that ran (for audit), not the source of truth.

---

## 6. State Machine

### 6.1 States (6 total)

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
| `pending` | `running` | Worker claims execution | Claim lease acquired |
| `running` | `completed` | All nodes done OR stop condition met | Stop CEL evaluates `true` |
| `running` | `failed` | Unrecoverable error | Retry count exceeded |
| `running` | `stopped` | Explicit stop API + context cancelled | D3: cancellation applied |
| `running` | `pending` | Crash recovery | Startup worker resets claim |

### 6.3 Clone Pattern for Retry

```go
func (s *Service) CloneSkillExecution(ctx context.Context, execID string) (*SkillExecution, error) {
    original, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &execID})
    if err != nil {
        return nil, err
    }

    clone := &SkillExecution{
        ID:                uuid.New().String(),
        TenantID:          original.TenantID,          // N7: pointer copy
        ConversationID:    original.ConversationID,
        SkillGraph:        original.SkillGraph,
        Status:            "pending",
        CheckpointData:    original.CheckpointData,
        CompletedNodes:    original.CompletedNodes,
        FailedNodes:       make(map[string]any),
        RetryCount:        0,
        MaxRetries:        original.MaxRetries,
        ParentExecutionID: &original.ID,               // N7: pointer
        CreatedAt:         time.Now(),
    }

    return s.store.CreateSkillExecution(ctx, clone)
}
```

---

## 7. Execution Loop Owner (D2 — Closed)

### 7.1 Model: Synchronous Chat + Detached Recovery

**Chat path (synchronous):** `processChatWithToolCalls` runs inside `generateResponse` → `ChatExternal`/`ChatInternal`. The tool-calling loop is synchronous within the HTTP request. A process crash mid-loop loses in-flight work, but the last checkpoint is durable — on restart, the recovery worker marks the execution `pending` and the next user message or operator resume picks it up.

**Detached path (background worker):** Event-triggered, API-triggered, and cron-triggered runs are created as `pending` executions. The detached worker (§10) claims and executes them independently of any chat request.

**Why this model:**
- Zero disruption to existing synchronous chat contract
- No client impact for `/chat/ext` and `/chat/int`
- Recovery worker already exists as the seed of the detached half
- Hackathon demo remains simple

### 7.2 Chat Path Integration

The tool-calling loop is called from within the existing `generateResponse` flow:

```go
// In generateResponse(), after building the prompt:
if config.SkillGraph != nil && config.SkillGraph.HasSkills {
    return s.processChatWithToolCalls(ctx, config, userMessage, conversationHistory)
}
// Fall through to existing non-skill chat path
```

### 7.3 Cancellation Token (D3)

Every execution carries a `context.WithCancel`. The stop API cancels the context; the loop checks it before each node write:

```go
func (s *Service) processChatWithToolCalls(ctx context.Context, config *AudienceConfig, userMessage string, history []*store.Message) (*ChatResponse, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Store cancel func for StopExecution to call
    s.activeExecutions.Store(executionID, cancel)
    defer s.activeExecutions.Delete(executionID)

    for i := 0; i < 10; i++ {
        // Check cancellation before each iteration
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
            // Check cancellation before executing
            select {
            case <-ctx.Done():
                return nil, fmt.Errorf("execution cancelled: %w", ctx.Err())
            default:
            }

            result, err := s.executeSkill(ctx, session, toolCall)
            if err != nil {
                return nil, err
            }

            // Checkpoint after each step
            s.checkpointExecution(ctx, session.Execution)
        }
    }
    return nil, fmt.Errorf("max iterations exceeded")
}
```

### 7.4 Stop Semantics (D3: Accepted + Applied)

Stop is **accepted** (DB row flip) and **applied** (next checkpoint boundary):

```go
func (s *Service) StopExecution(ctx context.Context, execID string, tenantID *int32) error {
    exec, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &execID, TenantID: tenantID})
    if err != nil {
        return fmt.Errorf("execution not found: %w", err)
    }

    if exec.Status != "running" && exec.Status != "pending" {
        return fmt.Errorf("execution in terminal state %q cannot be stopped", exec.Status)
    }

    // 1. Accept: flip DB state
    exec.Status = "stopped"
    now := time.Now()
    exec.CompletedAt = &now
    if err := s.store.UpdateSkillExecution(ctx, exec); err != nil {
        return err
    }

    // 2. Apply: cancel in-flight context (if chat-path execution)
    if cancel, ok := s.activeExecutions.Load(execID); ok {
        cancel.(context.CancelFunc)()
    }

    // 3. Skip in recovery loop (D3: mark as explicitly stopped)
    // Recovery worker checks: if status == "stopped", skip

    return nil
}
```

---

## 8. CEL Condition Evaluation (D4: Binding, Prerequisite, Fail-Safe)

### 8.1 Binding Contract

Checkpoint data is structured as `map[string]any` where each key is a skill name and the value is that skill's output. CEL bindings:

| CEL Identifier | Source | Type |
|----------------|--------|------|
| `checkpoint` | Full `CheckpointData` map | `map(string, dyn)` |
| `<skill_name>` | `CheckpointData[skill_name]` (top-level) | `map(string, dyn)` |

```go
func newCLEnv() *cel.Env {
    env, _ := cel.NewEnv(
        cel.Variable("checkpoint", cel.MapType(cel.StringType, cel.DynType)),
    )
    return env
}

func evaluateCondition(expr string, checkpointData map[string]any) (bool, error) {
    env, ast := compileCLE(expr)
    if env == nil {
        return false, fmt.Errorf("failed to compile CEL expression: %s", expr)
    }

    vars := map[string]any{
        "checkpoint": checkpointData,
    }

    // Populate top-level node names for direct access
    for key, val := range checkpointData {
        vars[key] = val
    }

    out, _, err := env.Eval(ast)
    if err != nil {
        return false, fmt.Errorf("CEL evaluation error: %w", err)
    }

    return out.Value().(bool), nil
}
```

### 8.2 Prerequisite Policy (D4)

A condition referencing a node that hasn't run yet is **not-yet-evaluable** → treat as `false` (don't fire). The condition is re-evaluated after the referenced node completes.

```go
// Guard condition evaluation (before executing a skill)
func (s *Service) evaluateGuard(condition string, checkpointData map[string]any) bool {
    if condition == "" {
        return true // no guard = always execute
    }

    // Parse the condition to find referenced skill names
    referencedSkills := extractSkillRefs(condition)

    // Check if all referenced skills have completed
    for _, skillName := range referencedSkills {
        if _, completed := checkpointData[skillName]; !completed {
            slog.Debug("Guard condition references incomplete skill, deferring",
                "condition", condition, "incomplete_skill", skillName)
            return false // not-yet-evaluable → don't fire
        }
    }

    result, err := evaluateCondition(condition, checkpointData)
    if err != nil {
        slog.Error("Guard condition evaluation failed, failing closed",
            "condition", condition, "error", err)
        return false // D4: fail-closed for guards
    }

    return result
}
```

### 8.3 Fail-Safe Default (D4)

| Context | Fail-Open or Fail-Closed | Rationale |
|---------|--------------------------|-----------|
| Guard conditions (pre-execution) | **Fail-closed** | Don't execute a skill if we can't evaluate its guard |
| Stop conditions (post-execution) | **Fail-closed** | Don't stop if we can't evaluate the condition |
| All other conditions | **Fail-closed** with log | Safe default for automation |

### 8.4 Stop Signal Evaluation

```go
func (s *Service) EvaluateStopSignal(ctx context.Context, exec *SkillExecution, stopCondition, emitEvent string) (bool, error) {
    if stopCondition == "" {
        return false, nil
    }

    // Check prerequisites first
    referencedSkills := extractSkillRefs(stopCondition)
    for _, skillName := range referencedSkills {
        if _, completed := exec.CheckpointData[skillName]; !completed {
            return false, nil // not-yet-evaluable
        }
    }

    result, err := evaluateCondition(stopCondition, exec.CheckpointData)
    if err != nil {
        slog.Error("Stop condition evaluation failed, failing closed",
            "execution_id", exec.ID, "condition", stopCondition, "error", err)
        return false, err // fail-closed
    }

    if !result {
        return false, nil
    }

    exec.Status = "completed"
    now := time.Now()
    exec.CompletedAt = &now

    if err := s.store.UpdateSkillExecution(ctx, exec); err != nil {
        return false, err
    }

    // Emit outbound event via existing dispatchEvent pattern (D6)
    if emitEvent != "" {
        s.emitStopEvent(ctx, exec, emitEvent)
    }

    return true, nil
}
```

---

## 9. Outbound Events (D6: Reuse Existing Outbox)

`EvaluateStopSignal` and any other emit point **invoke the existing `dispatchEvent` outbox pattern**, not direct `AgentEvent` inserts:

```go
func (s *Service) emitStopEvent(ctx context.Context, exec *SkillExecution, emitEvent string) {
    payload, _ := json.Marshal(map[string]any{
        "execution_id": exec.ID,
        "status":       exec.Status,
        "checkpoint":   exec.CheckpointData,
    })

    // Reuse existing dispatchEvent (service.go:5422) which:
    // 1. Creates AgentEvent with pre-claimed status
    // 2. Spawns immediate delivery goroutine
    // 3. Uses integration_id for routing
    s.dispatchEvent(ctx, *exec.TenantID, exec.ID, emitEvent, string(payload))
}
```

**Distinction from `agent_workflows` (N10):** `agent_workflows` is a task-boundary log for conversation-level events. `agent_skill_executions` is a durable state machine for individual skill pipeline runs. They serve different purposes and have different schemas.

---

## 10. Startup Recovery Worker (D5: Claim/Lease, N11: Gating)

### 10.1 Claim/Lease Model (D5)

```sql
-- Claim an execution (CockroachDB)
UPDATE agent_skill_executions
SET status = 'running',
    claimed_at = NOW(),
    claimed_by = $1,
    claim_expires_at = NOW() + INTERVAL '5 minutes'
WHERE id = $2
  AND (status = 'pending' OR (status = 'running' AND claim_expires_at < NOW()))
RETURNING *;

-- Release a claim
UPDATE agent_skill_executions
SET claimed_at = NULL,
    claimed_by = NULL,
    claim_expires_at = NULL
WHERE id = $1 AND claimed_by = $2;
```

```go
func (s *Service) ClaimSkillExecution(ctx context.Context, execID string, workerID string) (*SkillExecution, error) {
    leaseDuration := 5 * time.Minute
    exec, err := s.store.ClaimSkillExecution(ctx, execID, workerID, leaseDuration)
    if err != nil {
        return nil, err
    }
    return exec, nil
}

func (s *Service) ReleaseSkillClaim(ctx context.Context, execID string, workerID string) error {
    return s.store.ReleaseSkillClaim(ctx, execID, workerID)
}
```

### 10.2 Recovery Worker

```go
func (s *Service) startSkillRecoveryWorker() {
    // N11: Gate on feature availability
    if os.Getenv("SKILL_RECOVERY_ENABLED") != "true" {
        return
    }
    if !s.ragEnabled {
        return // skip if RAG not configured
    }

    time.Sleep(10 * time.Second) // wait for DB migration
    workerID := fmt.Sprintf("worker-%s-%d", hostname, os.Getpid())

    for {
        execs, err := s.store.ListPendingSkillExecutions(context.Background())
        if err != nil {
            slog.Error("Failed to list pending skill executions", "error", err)
            time.Sleep(30 * time.Second)
            continue
        }

        for _, exec := range execs {
            // D5: Try to claim before executing
            claimed, err := s.store.ClaimSkillExecution(context.Background(), exec.ID, workerID, 5*time.Minute)
            if err != nil || claimed == nil {
                continue // another worker claimed it
            }

            // D3: Skip explicitly stopped executions
            if claimed.Status == "stopped" {
                s.ReleaseSkillClaim(context.Background(), exec.ID, workerID)
                continue
            }

            // Execute the pipeline
            go s.executeDetachedPipeline(context.Background(), claimed, workerID)
        }

        time.Sleep(60 * time.Second)
    }
}
```

---

## 11. Tenant Isolation

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

    ctx = context.WithValue(ctx, "tenant_id", tenantID)

    input := parseToolInput(toolCall.Function.Arguments)
    checkpoint := session.Execution.CheckpointData

    result, err := handler.Execute(ctx, input, checkpoint)
    if err != nil {
        return "", err
    }

    session.Execution.CheckpointData[toolCall.Function.Name] = result
    s.store.UpdateSkillExecution(ctx, session.Execution)

    return result, nil
}
```

---

## 12. API Endpoints (N8: RBAC)

### 12.1 Skill Management

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/v1/agent/:slug/skills` | `tenant:read` | List skill definitions |
| POST | `/api/v1/agent/:slug/skills/validate` | `tenant:admin` | Validate SCRIPT.md annotations |
| GET | `/api/v1/agent/:slug/executions` | `chat:logs` | List skill executions |
| GET | `/api/v1/agent/:slug/executions/:id` | `chat:logs` | Get execution details |
| POST | `/api/v1/agent/:slug/executions/:id/retry` | `chat:test` | Clone failed execution |
| POST | `/api/v1/agent/:slug/executions/:id/suspend` | `api:config` | Suspend execution |
| POST | `/api/v1/agent/:slug/executions/:id/resume` | `api:config` | Resume execution |

### 12.2 Start/Stop Lifecycle (S2-S3, N8: RBAC)

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| POST | `/api/v1/agent/:slug/workflows/start` | `tenant:admin` | Start pipeline from event/API |
| POST | `/api/v1/agent/:slug/executions/:id/stop` | `api:config` | Explicit execution stop |

```go
// StartWorkflow handles POST /api/v1/agent/:slug/workflows/start
func (h *Handler) StartWorkflow(c echo.Context) error {
    slug := c.Param("slug")
    tenantID := getTenantFromContext(c)

    // N8: RBAC check
    hasPermission, _ := h.service.CheckUserPermission(c.Request().Context(), userID, *tenantID, "tenant:admin")
    if !hasPermission {
        return echo.NewHTTPError(http.StatusForbidden, "permission denied")
    }

    var req struct {
        TriggerType string         `json:"trigger_type"`
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

    // N8: RBAC check
    hasPermission, _ := h.service.CheckUserPermission(c.Request().Context(), userID, *tenantID, "api:config")
    if !hasPermission {
        return echo.NewHTTPError(http.StatusForbidden, "permission denied")
    }

    if err := h.service.StopExecution(c.Request().Context(), execID, tenantID); err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    return c.JSON(http.StatusOK, map[string]string{"status": "stopped"})
}
```

### 12.3 SSE Progress (D7: Scope Cut for Demo)

**Decision:** SSE progress events (`skill_start`, `skill_complete`) are **dropped for the demo**. The `/chat/stream` endpoint does not exist in the codebase. Progress is visible in:
- Simulation/playground surface (SSE already exists via `HandleSimulationStream`)
- Execution log API (`GET /api/v1/agent/:slug/executions/:id`)

This is a deliberate scope cut, not an oversight. If a chat SSE route is added post-demo, progress events can be wired in.

---

## 13. Observational Memory Interaction

Skill executions interact with OM naturally through conversation history:

1. Skill inputs/outputs are tool-call/tool-response messages — part of conversation history
2. ObserverBuffer compresses them like any other messages
3. Token budget includes tool messages — skill outputs count toward context window
4. No special OM integration needed — the existing pipeline handles it

---

## 14. Environment Variables

```bash
# Skill Execution
SKILL_RECOVERY_ENABLED=true                    # Enable startup recovery worker (N11)
SKILL_MAX_RETRIES=3                            # Default max retries per skill
SKILL_TIMEOUT=30s                              # Default timeout per skill
SKILL_CHECKPOINT_INTERVAL=1                    # Checkpoint after N tool calls
SKILL_LEASE_DURATION=5m                        # D5: Claim lease duration

# CEL Condition Evaluation
CEL_CONDITION_ENABLED=true                     # Enable CEL condition evaluation

# Token Budget
SKILL_MAX_TOKENS=1500                          # Max tokens for skill definitions in prompt
```

---

## 15. Testing Strategy

### 15.1 Layer 1: Unit Tests (No LLM, No DB)

```go
func TestSkillGraphCycleDetection(t *testing.T) {
    graph := &SkillGraph{
        Nodes: map[string]*SkillDefinition{
            "a": {DependsOn: []string{"b"}},
            "b": {DependsOn: []string{"a"}},
        },
    }
    err := graph.Validate()
    assert.ErrorContains(t, err, "cycle detected")
}

func TestConditionPrerequisitePolicy(t *testing.T) {
    checkpoint := map[string]any{
        "classify_intent": map[string]any{"intent": "complaint"},
    }
    // search_kb hasn't run yet → condition not-yet-evaluable → false
    assert.False(t, evaluateCondition("search_kb.found == false", checkpoint))
}

func TestConditionEvaluation(t *testing.T) {
    checkpoint := map[string]any{
        "search_kb": map[string]any{"found": false},
    }
    result, err := evaluateCondition("search_kb.found == false", checkpoint)
    assert.NoError(t, err)
    assert.True(t, result)
}

func TestStateTransitions(t *testing.T) {
    exec := &SkillExecution{Status: "pending"}
    assert.True(t, exec.CanTransitionTo("running"))
    assert.False(t, exec.CanTransitionTo("completed"))
    assert.False(t, exec.CanTransitionTo("failed"))
}
```

### 15.2 Layer 2: Integration Tests with Mock LLM

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

### 15.3 Layer 3: Simulation Framework

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

## 16. Implementation Phases

### Phase 1: Core Infrastructure (Days 1-3)
- [ ] Extend parser.go: extract `@skill`, `@trigger`, `@signal` blocks (D1)
- [ ] Create SQLite, CockroachDB, and MySQL tables with migrations (D8)
- [ ] Build SkillGraph parsing and validation (N5: fail-fast, N9: no "none")
- [ ] Implement state machine transitions (6 states)
- [ ] Add claim/lease schema columns (D5)

### Phase 2: Execution Engine (Days 4-6)
- [ ] Build SkillRegistry with builtin handlers
- [ ] Implement SkillHandler interface
- [ ] Add CEL condition evaluation (D4: binding, prerequisites, fail-closed)
- [ ] Create checkpoint/resume logic with claim model (D5)
- [ ] Implement context cancellation for stop (D3)

### Phase 3: Integration (Days 7-9)
- [ ] Integrate tool-calling loop into `generateResponse` (D2: sync path)
- [ ] Add skill definitions to `buildSystemPrompt()` via `LoadConfig` (D9)
- [ ] Implement tenant scoping on all executor methods (N7)
- [ ] Add API endpoints with RBAC (N8): `/workflows/start`, `/executions/:id/stop`
- [ ] Wire outbound events through `dispatchEvent` (D6)
- [ ] Build recovery worker with claim model (D5, N11)

### Phase 4: Testing & Polish (Days 10-12)
- [ ] Unit tests for DAG parser, state machine, CEL evaluator
- [ ] Integration tests with mock LLM server
- [ ] Extend simulation framework for skill assertions
- [ ] `task validate:parity` + `task validate:schema` (D8)
- [ ] Documentation and examples

### Phase 5: Demo Preparation (Days 13-14)
- [ ] Create demo scenarios (customer support, data processing)
- [ ] Prepare hackathon presentation
- [ ] Write documentation
- [ ] Final testing

---

## 17. Risk Mitigation

| Risk | Mitigation |
|------|------------|
| CockroachDB unavailable | Fallback to SQLite (existing pattern) |
| Handler panic | Recover + mark failed + log error |
| Infinite loop in DAG | Max iterations + timeout |
| Condition evaluation error | Fail-closed + log error (D4) |
| Checkpoint corruption | Verify checksums |
| Multi-instance claim race | Claim/lease model with expiry (D5) |
| Token budget overflow | Progressive disclosure + cap |
| Cross-tenant data leakage | Tenant scoping on every method |
| Stop races with in-flight work | Context cancellation + checkpoint gate (D3) |

---

## 18. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Execution success rate | 99.9% | CockroachDB logs |
| Checkpoint frequency | After each tool call | Configuration |
| Resume success rate | 100% | Test scenarios |
| Response time | < 2s | API metrics |
| Concurrent executions | 100+ | Load testing |
| Token budget compliance | < 1,500 tokens | Prompt analysis |

---

**This plan is ready for implementation.** All design gaps (D1-D9) are closed. The execution-loop ownership model (D2) is specified. Stop semantics (D3) include cancellation. Condition evaluation (D4) has binding, prerequisite, and fail-safe policies. The recovery worker (D5) has a claim/lease model. Outbound events (D6) reuse the existing outbox. SSE progress (D7) is explicitly scoped out for the demo. Multi-driver parity (D8) is precisely stated. The skill-graph lifecycle (D9) is cached and invalidated correctly.

**Next step:** Begin Phase 1 implementation (Core Infrastructure).
