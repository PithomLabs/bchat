# bchat Durable Execution Architecture — Plan 5 (plan5.md)

**Version:** 5.0
**Date:** 2026-08-05
**Status:** Approved with Nits
**Reviews Addressed:** plan2_review N1-N6, deepseek D1-D9, plan4_review R1-R6

---

## Changelog from plan4.md

| # | Finding | Resolution |
|---|---------|------------|
| R1 | CEL binding uses nonexistent `env.Eval` API | Rewrite to `env.Compile` → `env.Program` → `prg.Eval(cel.Vars(vars))` with all node identifiers declared |
| R2 | Chat + detached double-owner race | Add `trigger_path` column (`chat`/`event`/`api`/`cron`); worker filters `WHERE trigger_path != 'chat'` |
| R3 | Stop doesn't apply to detached runs | Register detached cancel funcs; checkpoint writes re-read row status before write |
| R4 | Parser reads `params["code"]` not `params["name"]` | Fix to use `params["code"]`; add `LineStart` to `annotationBlock` |
| R5 | CockroachDB is versioned, not single-file | Fix to 4 versioned dirs (`sqlite`, `postgres`, `mysql`, `cockroach`) + 4 × LATEST.sql |
| R6 | RBAC uses nonexistent `CheckUserPermission` | Replace with `h.hasPermission(c, tenant.ID, PermXxx)` pattern + `h.isAdmin(c)` bypass |

---

## 1. Executive Summary

**Goal:** Transform bchat from a reactive chat agent into a durable automation pipeline that survives failures, resumes from checkpoints, and provides full audit trails — all driven by declarative markdown annotations in SCRIPT.md.

**Key Differentiator vs n8n:** "Declarative markdown config + hybrid Go/LLM execution + CockroachDB persistent memory vs visual drag-and-drop + JavaScript nodes + no built-in AI memory."

**Execution Model (D2, R2 — Closed):** Synchronous chat execution for `/chat/ext` and `/chat/int` requests; a detached background worker owns recovery, event-triggered, and API-triggered runs. Chat-path executions carry `trigger_path = 'chat'` and are excluded from the worker's claim query. This eliminates the double-owner race while preserving the existing synchronous chat contract with zero client impact.

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
│  │ (sync, inline,  │ │ Worker      │ │ outbox (existing)   │  │
│  │  trigger_path=  │ │ (trigger_   │ │                     │  │
│  │  'chat')        │ │  path!=     │ │                     │  │
│  │                 │ │  'chat')    │ │                     │  │
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
│ 3. Create execution row: trigger_path = 'chat'              │
│ 4. LLM call with tool definitions                           │
│ 5. LLM decides which skills to call (tool-calling)          │
│ 6. Runtime enforces DAG constraints (dependencies)          │
│ 7. Execute builtin handler OR LLM reasoning step            │
│ 8. Check context cancellation (D3/R3) before each write     │
│ 9. Re-read row status before checkpoint write (R3)          │
│ 10. Check stop condition (D4) after each step               │
│ 11. Return result as tool response to LLM                   │
│ 12. Checkpoint to DB after each step                        │
│ 13. LLM continues with tool result in context               │
│ 14. Repeat until LLM produces final response                │
└─────────────────────────────────────────────────────────────┘
```

### 2.3 Data Flow — Detached Path (Event/API/Cron Triggers)

```
External Event (AgentEvent) / API Call (/workflows/start) / Cron
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Create SkillExecution (status: pending,                  │
│    trigger_path: 'event'|'api'|'cron')                     │
│ 2. Detached worker claims execution (D5: lease, R2: filter) │
│ 3. Register cancel func in activeExecutions (R3)            │
│ 4. Load cached SkillGraph for tenant (D9)                   │
│ 5. Execute tool-calling loop (same as chat path)            │
│ 6. Re-read row status before each checkpoint write (R3)     │
│ 7. Checkpoint after each step                               │
│ 8. On completion: evaluate stop condition (D4)              │
│ 9. Emit outbound event via dispatchEvent (D6)               │
│ 10. Release claim + deregister cancel func                  │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. SCRIPT.md Extension

### 3.1 Extended Annotation Format (N2: Single-Line Inline, N9: No "none")

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

### 3.2 Parser Extension (D1: Signal Data Path, R4: Fix Name Key + LineStart)

`ParseScriptWithSkills` extracts `@skill`, `@trigger`, and `@signal` blocks in a single pass.

**R4 fix:** The existing `parseParams` stores the first positional value under `params["code"]`, not `params["name"]`. The parser reads `params["code"]` for the skill name. Additionally, `LineStart` is added to `annotationBlock` by deriving byte offsets from `FindAllStringSubmatchIndex`.

```go
// R4: Add LineStart to annotationBlock
type annotationBlock struct {
    annotationType string
    params         map[string]string
    title          string
    content        string
    lineStart      int  // R4: byte offset → line number
}

// R4: Derive line number from byte offset
func byteOffsetToLine(content string, offset int) int {
    return strings.Count(content[:offset], "\n") + 1
}

// R4: extractAnnotationBlocks now tracks line numbers
func extractAnnotationBlocks(content string) []annotationBlock {
    var blocks []annotationBlock
    annotationPattern := regexp.MustCompile(`<!--\s*@(\w+)(?::\s*([^>]*))?\s*-->`)
    matches := annotationPattern.FindAllStringSubmatchIndex(content, -1)

    for i, match := range matches {
        if len(match) < 6 {
            continue
        }
        annotationType := content[match[2]:match[3]]
        params := ""
        if match[4] >= 0 && match[5] >= 0 {
            params = content[match[4]:match[5]]
        }

        // ... existing block-end detection ...

        blocks = append(blocks, annotationBlock{
            annotationType: annotationType,
            params:         parseParams(params),
            title:          title,
            content:        actualContent,
            lineStart:      byteOffsetToLine(content, match[0]),  // R4
        })
    }
    return blocks
}
```

```go
type TriggerDefinition struct {
    Type      string `json:"type"`
    EventType string `json:"event_type"`
}

type StopSignalDefinition struct {
    Condition string `json:"condition"`
    EmitEvent string `json:"emit_event"`
}

type SkillGraph struct {
    Nodes       map[string]*SkillDefinition `json:"nodes"`
    EntryPoints []string                    `json:"entry_points"`
    Trigger     *TriggerDefinition          `json:"trigger,omitempty"`
    Stop        *StopSignalDefinition       `json:"stop,omitempty"`
    HasSkills   bool                        `json:"has_skills"`
    ContentHash string                      `json:"content_hash"`
}

func (p *Parser) ParseScriptWithSkills(content string) (*ParsedScript, *SkillGraph, error) {
    parsed := p.ParseScript(content)
    graph := &SkillGraph{Nodes: make(map[string]*SkillDefinition)}
    graph.ContentHash = sha256Hex(content)

    for _, block := range parsed.AnnotationBlocks {
        switch block.annotationType {
        case "skill":
            // R4: Read name from params["code"] (first positional value)
            name := block.params["code"]
            if name == "" {
                continue
            }
            skill := &SkillDefinition{
                Name:       name,
                Handler:    block.params["handler"],
                DependsOn:  parseCommaSeparated(block.params["depends_on"]),
                Timeout:    block.params["timeout"],
                Condition:  block.params["condition"],
                LineStart:  block.lineStart,  // R4: real line number
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
        if err := graph.Validate(); err != nil {
            return nil, nil, fmt.Errorf("invalid skill graph at line %d: %w", err.Line, err)
        }
    }

    return parsed, graph, nil
}
```

### 3.3 DAG Validation (N5: Fail-Fast on Upload)

Fail-fast enforced at upload time in `HandleImportScript` (`handlers.go:4059`):

```go
func (h *Handler) HandleImportScript(c echo.Context) error {
    // ... existing upload logic ...

    parsed, graph, err := h.parser.ParseScriptWithSkills(content)
    if err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }
    if graph != nil && graph.HasSkills {
        if valErr := graph.Validate(); valErr != nil {
            return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
                "SCRIPT.md skill validation failed at line %d: %s", valErr.Line, valErr.Message))
        }
    }

    // ... continue with save ...
}
```

---

## 4. Skill Tool Registry

### 4.1 Handler Interface

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
    trigger_path STRING NOT NULL DEFAULT 'chat',  -- R2: 'chat'|'event'|'api'|'cron'
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
    INDEX idx_skill_exec_claim (status, trigger_path, claimed_at)  -- R2: worker query
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
    trigger_path TEXT NOT NULL DEFAULT 'chat',  -- R2
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
    trigger_path VARCHAR(10) NOT NULL DEFAULT 'chat',  -- R2
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
    TenantID           *int32         `json:"tenant_id"`
    ConversationID     string         `json:"conversation_id"`
    SkillGraph         *SkillGraph    `json:"skill_graph"`
    Status             string         `json:"status"`
    TriggerPath        string         `json:"trigger_path"`        // R2: 'chat'|'event'|'api'|'cron'
    CurrentNode        string         `json:"current_node"`
    CheckpointData     map[string]any `json:"checkpoint_data"`
    CompletedNodes     map[string]any `json:"completed_nodes"`
    FailedNodes        map[string]any `json:"failed_nodes"`
    RetryCount         int            `json:"retry_count"`
    MaxRetries         int            `json:"max_retries"`
    ParentExecutionID  *string        `json:"parent_execution_id,omitempty"`
    ClaimedAt          *time.Time     `json:"claimed_at,omitempty"`
    ClaimedBy          *string        `json:"claimed_by,omitempty"`
    ClaimExpiresAt     *time.Time     `json:"claim_expires_at,omitempty"`
    CreatedAt          time.Time      `json:"created_at"`
    UpdatedAt          time.Time      `json:"updated_at"`
    CompletedAt        *time.Time     `json:"completed_at,omitempty"`
}

type FindSkillExecution struct {
    ID             *string
    TenantID       *int32
    ConversationID *string
    Status         *string
    TriggerPath    *string  // R2: filter by trigger path
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

### 5.5 Multi-Driver Parity (D8, R5: Corrected)

**3 driver implementations, 4 deploy targets:**

| Implementation | Deploy Targets |
|----------------|----------------|
| `store/db/sqlite/` | SQLite (local dev) |
| `store/db/postgres/` | PostgreSQL + CockroachDB (CRDB reuses postgres driver) |
| `store/db/mysql/` | MySQL |

**R5 correction — CockroachDB is versioned, not single-file:**

```
store/migration/sqlite/<version>/NN__add_skill_executions.sql
store/migration/postgres/<version>/NN__add_skill_executions.sql
store/migration/mysql/<version>/NN__add_skill_executions.sql
store/migration/cockroach/<version>/NN__add_skill_executions.sql
```

Plus 4 × `LATEST.sql` drift files. **All 4 versioned dirs + 4 × LATEST.sql + `task validate:parity` must pass.**

### 5.6 Skill-Graph Lifecycle (D9: Cached, Invalidated on Upload)

```go
func (s *Service) LoadConfig(ctx context.Context, tenantSlug, audienceType string) (*AudienceConfig, error) {
    if config := s.configCache.Get(tenantSlug, audienceType); config != nil {
        return config, nil
    }
    // ... existing config loading ...

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

Invalidation via existing `HandleImportScript` → `configCache.Invalidate(tenant.Slug)` (handlers.go:4129). Per-execution rows store a snapshot for audit, not the source of truth.

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
        TenantID:          original.TenantID,
        ConversationID:    original.ConversationID,
        SkillGraph:        original.SkillGraph,
        Status:            "pending",
        TriggerPath:       original.TriggerPath,  // R2: preserve trigger path
        CheckpointData:    original.CheckpointData,
        CompletedNodes:    original.CompletedNodes,
        FailedNodes:       make(map[string]any),
        RetryCount:        0,
        MaxRetries:        original.MaxRetries,
        ParentExecutionID: &original.ID,
        CreatedAt:         time.Now(),
    }

    return s.store.CreateSkillExecution(ctx, clone)
}
```

---

## 7. Execution Loop Owner (D2/R2 — Closed)

### 7.1 Model: Synchronous Chat + Detached Recovery

**Chat path (synchronous):** `processChatWithToolCalls` runs inside `generateResponse` → `ChatExternal`/`ChatInternal`. The execution row is created with `trigger_path = 'chat'`. The detached worker's claim query filters `WHERE trigger_path != 'chat'`, so chat-path rows are never double-claimed.

**Detached path (background worker):** Event-triggered, API-triggered, and cron-triggered runs are created with `trigger_path` set to `'event'`, `'api'`, or `'cron'`. The worker claims these exclusively.

**Abnormal termination (chat path):** If the chat request crashes mid-loop, the execution row is left in `pending` or `running` state with `trigger_path = 'chat'`. Since the worker skips chat rows, these are never auto-resumed — they become stale audit records. The operator can manually retry via `/executions/:id/retry`.

### 7.2 Chat Path Integration

```go
func (s *Service) generateResponse(ctx context.Context, config *AudienceConfig, userMessage string, history []*store.Message) (*ChatResponse, error) {
    if config.SkillGraph != nil && config.SkillGraph.HasSkills {
        return s.processChatWithToolCalls(ctx, config, userMessage, history)
    }
    // Fall through to existing non-skill chat path
    return s.processChat(ctx, config, userMessage, history)
}
```

### 7.3 Cancellation Token + Status Re-read (D3, R3)

Every execution — chat AND detached — carries a `context.WithCancel`. The `activeExecutions` map stores cancel funcs for both paths. Checkpoint writes re-read the row status before writing.

```go
// activeExecutions stores cancel funcs for all running executions
var activeExecutions sync.Map // map[string]context.CancelFunc

func (s *Service) processChatWithToolCalls(ctx context.Context, config *AudienceConfig, userMessage string, history []*store.Message) (*ChatResponse, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Create execution row (chat path)
    execution, err := s.store.CreateSkillExecution(ctx, &store.SkillExecution{
        TenantID:       config.TenantID,
        ConversationID: uuid.New().String(),
        SkillGraph:     config.SkillGraph,
        Status:         "running",
        TriggerPath:    "chat",  // R2: mark as chat-path
        CheckpointData: make(map[string]any),
        CreatedAt:      time.Now(),
    })
    if err != nil {
        return nil, err
    }

    // R3: Register cancel func for both chat and detached paths
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

            // R3: Re-read status before checkpoint write
            if err := s.checkpointWithStatusReRead(ctx, execution); err != nil {
                return nil, err
            }
        }
    }
    return nil, fmt.Errorf("max iterations exceeded")
}
```

### 7.4 Checkpoint with Status Re-read (R3)

```go
func (s *Service) checkpointWithStatusReRead(ctx context.Context, exec *store.SkillExecution) error {
    // R3: Re-read row status from DB before writing
    fresh, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
    if err != nil {
        return err
    }

    // If someone stopped this execution, abort before overwriting
    if fresh.Status == "stopped" {
        slog.Info("Execution stopped, aborting checkpoint", "execution_id", exec.ID)
        return fmt.Errorf("execution %s was stopped", exec.ID)
    }

    // Proceed with checkpoint
    exec.UpdatedAt = time.Now()
    return s.store.UpdateSkillExecution(ctx, exec)
}
```

### 7.5 Stop Semantics (D3: Accepted + Applied, R3: Both Paths)

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

    // 2. Apply: cancel in-flight context (works for BOTH chat and detached)
    if cancel, ok := activeExecutions.Load(execID); ok {
        cancel.(context.CancelFunc)()
    }

    // 3. Recovery worker: skip stopped rows (checked at claim time + checkpoint re-read)

    return nil
}
```

---

## 8. CEL Condition Evaluation (R1: Fixed cel-go API, D4: Binding + Prerequisite + Fail-Safe)

### 8.1 Binding Contract

Checkpoint data is `map[string]any` where each key is a skill name. CEL bindings use the `checkpoint` map with top-level node aliases.

**R1 fix:** Uses the real cel-go v0.25.0 API: `env.Compile` → `env.Program` → `prg.Eval(cel.Vars(vars))`. All node names are declared as `cel.Variable` at env construction.

```go
// R1: Use real cel-go Program API
func newCLEEnv(nodeNames []string) (*cel.Env, error) {
    opts := []cel.EnvOption{
        cel.Variable("checkpoint", cel.MapType(cel.StringType, cel.DynType)),
    }

    // R1: Declare every runtime node name as a top-level identifier
    for _, name := range nodeNames {
        opts = append(opts, cel.Variable(name, cel.DynType))
    }

    return cel.NewEnv(opts...)
}

// R1: Compile + Program + Eval (matches plugin/filter/filter.go pattern)
func compileCLE(expr string, nodeNames []string) (*cel.Program, error) {
    env, err := newCLEEnv(nodeNames)
    if err != nil {
        return nil, fmt.Errorf("failed to create CEL env: %w", err)
    }

    ast, issues := env.Compile(expr)
    if issues != nil && issues.Err() != nil {
        return nil, fmt.Errorf("failed to compile CEL expression %q: %w", expr, issues.Err())
    }

    prg, err := env.Program(ast)
    if err != nil {
        return nil, fmt.Errorf("failed to create CEL program: %w", err)
    }

    return prg, nil
}

func evaluateCondition(expr string, checkpointData map[string]any) (bool, error) {
    // Collect node names from checkpoint
    nodeNames := make([]string, 0, len(checkpointData))
    for key := range checkpointData {
        nodeNames = append(nodeNames, key)
    }

    prg, err := compileCLE(expr, nodeNames)
    if err != nil {
        return false, err
    }

    // R1: Build activation with all bindings
    vars := map[string]any{
        "checkpoint": checkpointData,
    }
    for key, val := range checkpointData {
        vars[key] = val
    }

    // R1: Real eval — cel.Vars binds variables to the program
    out, _, err := prg.Eval(cel.Vars(vars))
    if err != nil {
        return false, fmt.Errorf("CEL evaluation error: %w", err)
    }

    return out.Value().(bool), nil
}
```

### 8.2 Prerequisite Policy (D4)

A condition referencing a node that hasn't run yet is **not-yet-evaluable** → treat as `false` (don't fire). The condition is re-evaluated after the referenced node completes.

```go
func (s *Service) evaluateGuard(condition string, checkpointData map[string]any) bool {
    if condition == "" {
        return true
    }

    referencedSkills := extractSkillRefs(condition)
    for _, skillName := range referencedSkills {
        if _, completed := checkpointData[skillName]; !completed {
            slog.Debug("Guard condition references incomplete skill, deferring",
                "condition", condition, "incomplete_skill", skillName)
            return false
        }
    }

    result, err := evaluateCondition(condition, checkpointData)
    if err != nil {
        slog.Error("Guard condition evaluation failed, failing closed",
            "condition", condition, "error", err)
        return false
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

---

## 9. Outbound Events (D6: Reuse Existing Outbox)

```go
func (s *Service) emitStopEvent(ctx context.Context, exec *store.SkillExecution, emitEvent string) {
    payload, _ := json.Marshal(map[string]any{
        "execution_id": exec.ID,
        "status":       exec.Status,
        "checkpoint":   exec.CheckpointData,
    })

    // Reuse existing dispatchEvent (service.go:5422)
    // Note: short-circuits when tenant has no active webhook integrations
    s.dispatchEvent(ctx, *exec.TenantID, exec.ID, emitEvent, string(payload))
}
```

**Distinction from `agent_workflows` (N10):** `agent_workflows` is a task-boundary log for conversation-level events. `agent_skill_executions` is a durable state machine for individual skill pipeline runs.

---

## 10. Startup Recovery Worker (D5: Claim/Lease, R2: Path Filter, N11: Gating)

### 10.1 Claim/Lease Model (D5)

```sql
-- Claim: only pending rows, only non-chat triggers (R2)
UPDATE agent_skill_executions
SET status = 'running',
    claimed_at = NOW(),
    claimed_by = $1,
    claim_expires_at = NOW() + INTERVAL '5 minutes'
WHERE id = $2
  AND trigger_path != 'chat'  -- R2: never claim chat-path rows
  AND (status = 'pending' OR (status = 'running' AND claim_expires_at < NOW()))
RETURNING *;
```

```go
func (s *Service) ClaimSkillExecution(ctx context.Context, execID string, workerID string) (*store.SkillExecution, error) {
    leaseDuration := 5 * time.Minute
    return s.store.ClaimSkillExecution(ctx, execID, workerID, leaseDuration)
}

func (s *Service) ReleaseSkillClaim(ctx context.Context, execID string, workerID string) error {
    return s.store.ReleaseSkillClaim(ctx, execID, workerID)
}
```

### 10.2 Recovery Worker

```go
func (s *Service) startSkillRecoveryWorker() {
    if os.Getenv("SKILL_RECOVERY_ENABLED") != "true" {
        return
    }
    if !s.ragEnabled {
        return
    }

    time.Sleep(10 * time.Second)
    workerID := fmt.Sprintf("worker-%s-%d", hostname, os.Getpid())

    for {
        // R2: ListPendingSkillExecutions filters WHERE trigger_path != 'chat'
        execs, err := s.store.ListPendingSkillExecutions(context.Background())
        if err != nil {
            slog.Error("Failed to list pending skill executions", "error", err)
            time.Sleep(30 * time.Second)
            continue
        }

        for _, exec := range execs {
            claimed, err := s.store.ClaimSkillExecution(context.Background(), exec.ID, workerID, 5*time.Minute)
            if err != nil || claimed == nil {
                continue
            }

            if claimed.Status == "stopped" {
                s.ReleaseSkillClaim(context.Background(), exec.ID, workerID)
                continue
            }

            // R3: Register cancel func for detached execution too
            ctx, cancel := context.WithCancel(context.Background())
            activeExecutions.Store(exec.ID, cancel)

            go func() {
                defer activeExecutions.Delete(exec.ID)
                defer cancel()
                s.executeDetachedPipeline(ctx, claimed, workerID)
            }()
        }

        time.Sleep(60 * time.Second)
    }
}
```

---

## 11. Tenant Isolation

```go
func (s *Service) executeSkill(ctx context.Context, exec *store.SkillExecution, toolCall openrouter.ToolCall) (string, error) {
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
    checkpoint := exec.CheckpointData

    result, err := handler.Execute(ctx, input, checkpoint)
    if err != nil {
        return "", err
    }

    exec.CheckpointData[toolCall.Function.Name] = result
    s.store.UpdateSkillExecution(ctx, exec)

    return result, nil
}
```

---

## 12. API Endpoints (R6: Real RBAC Seam)

### 12.1 Skill Management

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/v1/agent/:slug/skills` | `PermTenantRead` | List skill definitions |
| POST | `/api/v1/agent/:slug/skills/validate` | `PermTenantAdmin` | Validate SCRIPT.md annotations |
| GET | `/api/v1/agent/:slug/executions` | `PermChatLogs` | List skill executions |
| GET | `/api/v1/agent/:slug/executions/:id` | `PermChatLogs` | Get execution details |
| POST | `/api/v1/agent/:slug/executions/:id/retry` | `PermChatTest` | Clone failed execution |
| POST | `/api/v1/agent/:slug/executions/:id/suspend` | `PermAPIConfig` | Suspend execution |
| POST | `/api/v1/agent/:slug/executions/:id/resume` | `PermAPIConfig` | Resume execution |

### 12.2 Start/Stop Lifecycle (S2-S3, R6: Real RBAC)

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| POST | `/api/v1/agent/:slug/workflows/start` | `PermTenantAdmin` | Start pipeline from event/API |
| POST | `/api/v1/agent/:slug/executions/:id/stop` | `PermAPIConfig` | Explicit execution stop |

```go
// R6: Real RBAC pattern — matches handlers.go:1221
func (h *Handler) StartWorkflow(c echo.Context) error {
    slug := c.Param("slug")
    tenant, err := h.getTenant(c, slug)
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
    }

    // R6: Use h.hasPermission, not h.service.CheckUserPermission
    if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
        return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin or tenant:admin")
    }

    var req struct {
        TriggerType string         `json:"trigger_type"`
        EventType   string         `json:"event_type"`
        Payload     map[string]any `json:"payload"`
    }
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
    }

    exec, err := h.service.StartWorkflowSignal(c.Request().Context(), tenant.ID, req.TriggerType, req.EventType, req.Payload)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    return c.JSON(http.StatusCreated, exec)
}

func (h *Handler) StopExecution(c echo.Context) error {
    execID := c.Param("id")
    tenant, err := h.getTenant(c, c.Param("slug"))
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
    }

    // R6: Use h.hasPermission with PermAPIConfig
    if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) {
        return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin or api:config")
    }

    if err := h.service.StopExecution(c.Request().Context(), execID, &tenant.ID); err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    return c.JSON(http.StatusOK, map[string]string{"status": "stopped"})
}
```

### 12.3 SSE Progress (D7: Scope Cut for Demo)

SSE progress events are **dropped for the demo**. Progress is visible via:
- Simulation/playground surface (SSE already exists via `HandleSimulationStream`)
- Execution log API (`GET /api/v1/agent/:slug/executions/:id`)

Deliberate scope cut, not an oversight.

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
SKILL_RECOVERY_ENABLED=true                    # Enable startup recovery worker (N11)
SKILL_MAX_RETRIES=3                            # Default max retries per skill
SKILL_TIMEOUT=30s                              # Default timeout per skill
SKILL_CHECKPOINT_INTERVAL=1                    # Checkpoint after N tool calls
SKILL_LEASE_DURATION=5m                        # D5: Claim lease duration
CEL_CONDITION_ENABLED=true                     # Enable CEL condition evaluation
SKILL_MAX_TOKENS=1500                          # Max tokens for skill definitions in prompt
```

---

## 15. Testing Strategy

### 15.1 Layer 1: Unit Tests

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

func TestCELConditionEvaluation(t *testing.T) {
    checkpoint := map[string]any{
        "search_kb": map[string]any{"found": false},
    }
    result, err := evaluateCondition("search_kb.found == false", checkpoint)
    assert.NoError(t, err)
    assert.True(t, result)
}

func TestCELConditionIncompleteNode(t *testing.T) {
    checkpoint := map[string]any{
        "classify_intent": map[string]any{"intent": "complaint"},
    }
    // search_kb hasn't run → not-yet-evaluable → false
    result, err := evaluateCondition("search_kb.found == false", checkpoint)
    assert.NoError(t, err)
    assert.False(t, result)
}

func TestStateTransitions(t *testing.T) {
    exec := &SkillExecution{Status: "pending"}
    assert.True(t, exec.CanTransitionTo("running"))
    assert.False(t, exec.CanTransitionTo("completed"))
}

func TestParseSkillAnnotations(t *testing.T) {
    content := `<!-- @skill: classify_intent, handler: "builtin:classify_intent", timeout: "30s" -->
## Classify Intent
Determine intent.`
    _, graph, err := parser.ParseScriptWithSkills(content)
    assert.NoError(t, err)
    assert.True(t, graph.HasSkills)
    assert.Equal(t, "classify_intent", graph.Nodes["classify_intent"].Name)
    assert.Equal(t, "builtin:classify_intent", graph.Nodes["classify_intent"].Handler)
}
```

### 15.2 Layer 2: Integration Tests

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
- [ ] Extend parser.go: `@skill`/`@trigger`/`@signal` extraction with `params["code"]` + `LineStart` (R4)
- [ ] Create SQLite, CockroachDB, MySQL, and Postgres tables with migrations (R5: 4 versioned dirs)
- [ ] Build SkillGraph parsing and validation (N5: fail-fast, N9: no "none")
- [ ] Implement state machine transitions (6 states)
- [ ] Add claim/lease schema columns (D5) + `trigger_path` column (R2)

### Phase 2: Execution Engine (Days 4-6)
- [ ] Build SkillRegistry with builtin handlers
- [ ] Implement SkillHandler interface
- [ ] Add CEL condition evaluation — real cel-go Program API (R1)
- [ ] Create checkpoint/resume logic with claim model (D5)
- [ ] Implement context cancellation for stop (D3)
- [ ] Implement status re-read before checkpoint writes (R3)

### Phase 3: Integration (Days 7-9)
- [ ] Integrate tool-calling loop into `generateResponse` (D2: sync path, R2: trigger_path filter)
- [ ] Add skill definitions to `buildSystemPrompt()` via `LoadConfig` (D9)
- [ ] Implement tenant scoping on all executor methods (N7)
- [ ] Add API endpoints with `h.hasPermission` RBAC (R6): `/workflows/start`, `/executions/:id/stop`
- [ ] Wire outbound events through `dispatchEvent` (D6)
- [ ] Build recovery worker with claim model + path filter (D5, R2, N11)
- [ ] Register detached cancel funcs in `activeExecutions` (R3)

### Phase 4: Testing & Polish (Days 10-12)
- [ ] Unit tests for DAG parser, state machine, CEL evaluator (real API, R1)
- [ ] Integration tests with mock LLM server
- [ ] Extend simulation framework for skill assertions
- [ ] `task validate:parity` + `task validate:schema` (R5)
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
| Stop races with in-flight work | Context cancellation + status re-read (R3) |
| Chat-row claimed by worker | `trigger_path != 'chat'` filter (R2) |
| Detached stop not applied | Cancel func registered + status re-read (R3) |

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

**This plan is ready for implementation.** All design gaps (D1-D9) are closed. All review findings (R1-R6) are addressed against real codebase seams. The CEL binding uses the correct cel-go Program API (R1). The double-owner race is eliminated via `trigger_path` filtering (R2). Stop applies to both chat and detached paths via cancel registration + status re-read (R3). The parser reads `params["code"]` with `LineStart` tracking (R4). Migrations use 4 versioned directories (R5). RBAC uses `h.hasPermission` with `PermXxx` constants (R6).

**Next step:** Begin Phase 1 implementation (Core Infrastructure).
