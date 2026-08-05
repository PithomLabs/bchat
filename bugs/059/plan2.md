# bchat Durable Execution Architecture — Revised Plan (plan2.md)

**Version:** 2.0
**Date:** 2026-08-05
**Status:** Ready for Implementation
**Review Addressed:** Claude Opus 4.6 adversarial review (6 structural flaws, 9 gaps, 4 open questions)

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
- ✅ CockroachDB: Distributed vector indexing + durable execution state
- ✅ AWS: Amazon Bedrock (via OpenRouter) + Amazon S3 (skill assets)
- ✅ Go: All extensions written in Go
- ✅ 2+ CRDB tools: Vector Index + Skill Execution State
- ✅ 1+ AWS service: Bedrock + S3

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
│   1. Mark execution as completed/failed                     │
│   2. Send final response to user                            │
│   3. Log execution summary                                  │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. SCRIPT.md Extension (Not SKILLS.md)

### 3.1 Extended Annotation Format

SCRIPT.md already defines conversation flow. We extend it with `@skill` annotations:

```markdown
## Opening
- Greet the customer warmly
- Ask how you can help

<!-- @skill: classify_intent -->
<!-- @handler: builtin:classify_intent -->
<!-- @depends_on: none -->
<!-- @timeout: 30s -->
<!-- @max_retries: 3 -->
## Classify Intent
Determine what the customer wants from their message.
Analyze the message and classify into one of these intents:
- question: Customer asking for information
- complaint: Customer reporting a problem
- request: Customer requesting action
- escalation: Customer demanding supervisor

<!-- @skill: search_kb -->
<!-- @handler: builtin:search_kb -->
<!-- @depends_on: classify_intent -->
<!-- @timeout: 10s -->
## Search Knowledge Base
Search for relevant solutions in the knowledge base.
Use the classified intent to construct an appropriate search query.

<!-- @skill: create_ticket -->
<!-- @handler: builtin:create_ticket -->
<!-- @depends_on: search_kb -->
<!-- @condition: search_kb.found == false -->
## Create Support Ticket
If no solution found, create a support ticket for the customer.
Ticket type should match the classified intent.

<!-- @skill: respond -->
<!-- @handler: llm:respond -->
<!-- @depends_on: classify_intent, search_kb, create_ticket -->
## Respond to Customer
Provide a helpful response based on:
1. The classified intent
2. Knowledge base results (if found)
3. Ticket ID (if created)
Maintain a professional, empathetic tone per POLICY.md.
```

### 3.2 Parser Extension

Extend `parser.go` to extract skill annotations:

```go
// SkillBlock represents a parsed skill definition from SCRIPT.md
type SkillBlock struct {
    Name        string            `json:"name"`
    Handler     string            `json:"handler"`
    DependsOn   []string          `json:"depends_on"`
    Condition   string            `json:"condition,omitempty"`
    Timeout     string            `json:"timeout"`
    MaxRetries  int               `json:"max_retries"`
    Instructions string           `json:"instructions"`
    LineStart   int               `json:"line_start"`
    LineEnd     int               `json:"line_end"`
}

// ParseScriptWithSkills extends ParseScript to detect skill annotations
func (p *Parser) ParseScriptWithSkills(content string) (*ParsedScript, error) {
    result := &ParsedScript{RawContent: content}
    
    // Check for skill annotations
    skillBlocks := p.extractSkillBlocks(content)
    if len(skillBlocks) > 0 {
        // New path: parse as skill-annotated script
        result.SkillGraph = p.buildSkillGraph(skillBlocks)
        result.Summary = buildSkillSummary(result.SkillGraph)
        result.HasSkills = true
    } else {
        // Legacy path: parse as conversation flow
        result.Sections = p.parseSections(content)
        result.Summary = buildScriptSummary(result.Sections)
        result.HasSkills = false
    }
    
    return result, nil
}

// extractSkillBlocks finds all <!-- @skill: ... --> blocks
func (p *Parser) extractSkillBlocks(content string) []*SkillBlock {
    // Use existing extractAnnotationBlocks with type "skill"
    blocks := extractAnnotationBlocksByType(content, "skill")
    
    var skills []*SkillBlock
    for _, block := range blocks {
        skill := &SkillBlock{
            Name:        block.params["name"],
            Handler:     block.params["handler"],
            Timeout:     block.params["timeout"],
            MaxRetries:  3, // default
            Instructions: block.content,
            LineStart:   block.lineStart,
            LineEnd:     block.lineEnd,
        }
        
        // Parse depends_on (comma-separated)
        if deps, ok := block.params["depends_on"]; ok && deps != "none" {
            skill.DependsOn = strings.Split(deps, ",")
        }
        
        // Parse condition
        if cond, ok := block.params["condition"]; ok {
            skill.Condition = cond
        }
        
        // Parse max_retries
        if retries, ok := block.params["max_retries"]; ok {
            if n, err := strconv.Atoi(retries); err == nil {
                skill.MaxRetries = n
            }
        }
        
        skills = append(skills, skill)
    }
    return skills
}

// buildSkillGraph constructs a DAG from skill blocks
func (p *Parser) buildSkillGraph(blocks []*SkillBlock) *SkillGraph {
    graph := &SkillGraph{
        Nodes: make(map[string]*SkillNode),
        Edges: []SkillEdge{},
    }
    
    for _, block := range blocks {
        node := &SkillNode{
            Name:         block.Name,
            Handler:      block.Handler,
            DependsOn:    block.DependsOn,
            Condition:    block.Condition,
            Timeout:      parseDuration(block.Timeout),
            MaxRetries:   block.MaxRetries,
            Instructions: block.Instructions,
        }
        graph.Nodes[block.Name] = node
        
        // Build edges from dependencies
        for _, dep := range block.DependsOn {
            graph.Edges = append(graph.Edges, SkillEdge{
                From: dep,
                To:   block.Name,
            })
        }
    }
    
    return graph
}
```

### 3.3 Backward Compatibility

```go
func (p *Parser) ParseScript(content string) (*ParsedScript, error) {
    result := &ParsedScript{RawContent: content}
    
    // Check for skill annotations
    skillBlocks := extractAnnotationBlocksByType(content, "skill")
    if len(skillBlocks) > 0 {
        // New path: parse as skill-annotated script
        result.SkillGraph = p.buildSkillGraph(skillBlocks)
        result.Summary = buildSkillSummary(result.SkillGraph)
        result.HasSkills = true
    } else {
        // Legacy path: parse as conversation flow
        result.Sections = p.parseSections(content)
        result.Summary = buildScriptSummary(result.Sections)
        result.HasSkills = false
    }
    
    return result, nil
}
```

**Detection:** `if len(extractAnnotationBlocksByType(content, "skill")) > 0`

- SCRIPT.md with no `@skill` annotations → parses exactly as today
- SCRIPT.md with `@skill` annotations → parses as skill graph + adds tool definitions to prompt
- Zero migration cost — existing tenants unaffected

---

## 4. Handler Execution Model (Hybrid)

### 4.1 Handler Types

| Handler Prefix | Execution Model | Example |
|----------------|-----------------|---------|
| `builtin:classify_intent` | Go function in curated registry | Intent classification via LLM with structured output |
| `builtin:search_kb` | Go function (RAG pipeline call) | Existing `vectorDB.Search()` |
| `builtin:create_ticket` | Go function (DB write) | Existing `store.CreateTicket()` |
| `builtin:webhook` | Go function (HTTP POST) | Existing `AgentEvent` → integration delivery |
| `builtin:escalate` | Go function (DB write + notification) | Existing escalation flow |
| `llm:respond` | LLM call with skill instructions as system prompt | Free-form response generation |
| `llm:analyze` | LLM call with structured output schema | Sentiment analysis, summarization |

### 4.2 Skill Handler Interface

```go
// SkillHandler is the interface all skill handlers must implement
type SkillHandler interface {
    // Name returns the handler identifier (e.g., "builtin:classify_intent")
    Name() string
    
    // Execute runs the handler with tenant-scoped context
    Execute(ctx context.Context, tenantID int32, input map[string]any) (map[string]any, error)
    
    // InputSchema returns JSON Schema for input validation
    InputSchema() map[string]any
    
    // OutputSchema returns JSON Schema for output validation
    OutputSchema() map[string]any
    
    // IsDeterministic returns true for builtin handlers, false for LLM handlers
    IsDeterministic() bool
}

// SkillHandlerFunc is a convenience type for simple handlers
type SkillHandlerFunc func(ctx context.Context, tenantID int32, input map[string]any) (map[string]any, error)

func (f SkillHandlerFunc) Execute(ctx context.Context, tenantID int32, input map[string]any) (map[string]any, error) {
    return f(ctx, tenantID, input)
}
```

### 4.3 Skill Registry

```go
// SkillRegistry holds all registered skill handlers
type SkillRegistry struct {
    handlers map[string]SkillHandler
    mu       sync.RWMutex
}

// NewSkillRegistry creates a registry with builtin handlers
func NewSkillRegistry() *SkillRegistry {
    r := &SkillRegistry{
        handlers: make(map[string]SkillHandler),
    }
    
    // Register builtin handlers
    r.Register(&ClassifyIntentHandler{})
    r.Register(&SearchKBHandler{})
    r.Register(&CreateTicketHandler{})
    r.Register(&WebhookHandler{})
    r.Register(&EscalateHandler{})
    
    return r
}

// Register adds a handler to the registry
func (r *SkillRegistry) Register(handler SkillHandler) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.handlers[handler.Name()] = handler
}

// Get retrieves a handler by name
func (r *SkillRegistry) Get(name string) (SkillHandler, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    h, ok := r.handlers[name]
    return h, ok
}

// ToOpenRouterTools converts registered handlers to OpenRouter tool definitions
func (r *SkillRegistry) ToOpenRouterTools() []openrouter.Tool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var tools []openrouter.Tool
    for _, handler := range r.handlers {
        tool := openrouter.Tool{
            Type: "function",
            Function: &openrouter.Function{
                Name:        handler.Name(),
                Description: handler.Description(),
                Parameters:  handler.InputSchema(),
            },
        }
        tools = append(tools, tool)
    }
    return tools
}
```

### 4.4 Built-in Handler Implementations

```go
// ClassifyIntentHandler classifies customer intent
type ClassifyIntentHandler struct{}

func (h *ClassifyIntentHandler) Name() string { return "builtin:classify_intent" }

func (h *ClassifyIntentHandler) Execute(ctx context.Context, tenantID int32, input map[string]any) (map[string]any, error) {
    message, ok := input["message"].(string)
    if !ok {
        return nil, fmt.Errorf("missing or invalid 'message' parameter")
    }
    
    // Call LLM with structured output for classification
    prompt := fmt.Sprintf(`Classify the customer intent from this message:
    
"%s"

Respond with JSON: {"intent": "question|complaint|request|escalation", "confidence": 0.0-1.0}`, message)
    
    // Use existing LLM client with tenant context
    response, err := callLLMWithTenantContext(ctx, tenantID, prompt)
    if err != nil {
        return nil, fmt.Errorf("failed to classify intent: %w", err)
    }
    
    // Parse structured response
    var result struct {
        Intent     string  `json:"intent"`
        Confidence float64 `json:"confidence"`
    }
    if err := json.Unmarshal([]byte(response), &result); err != nil {
        return nil, fmt.Errorf("failed to parse classification: %w", err)
    }
    
    return map[string]any{
        "intent":     result.Intent,
        "confidence": result.Confidence,
    }, nil
}

func (h *ClassifyIntentHandler) InputSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "message": map[string]any{
                "type": "string",
                "description": "The customer's message to classify",
            },
        },
        "required": []string{"message"},
    }
}

// SearchKBHandler searches the knowledge base
type SearchKBHandler struct {
    VectorDB VectorDB
}

func (h *SearchKBHandler) Name() string { return "builtin:search_kb" }

func (h *SearchKBHandler) Execute(ctx context.Context, tenantID int32, input map[string]any) (map[string]any, error) {
    query, ok := input["query"].(string)
    if !ok {
        return nil, fmt.Errorf("missing or invalid 'query' parameter")
    }
    
    // Use existing vector DB search with tenant context
    results, err := h.VectorDB.Search(ctx, tenantID, query, 5)
    if err != nil {
        return nil, fmt.Errorf("failed to search knowledge base: %w", err)
    }
    
    found := len(results) > 0
    return map[string]any{
        "results": results,
        "found":   found,
    }, nil
}

func (h *SearchKBHandler) InputSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "query": map[string]any{
                "type": "string",
                "description": "Search query for the knowledge base",
            },
        },
        "required": []string{"query"},
    }
}

// CreateTicketHandler creates a support ticket
type CreateTicketHandler struct {
    Store store.Store
}

func (h *CreateTicketHandler) Name() string { return "builtin:create_ticket" }

func (h *CreateTicketHandler) Execute(ctx context.Context, tenantID int32, input map[string]any) (map[string]any, error) {
    ticketType, _ := input["ticket_type"].(string)
    description, _ := input["description"].(string)
    priority, _ := input["priority"].(string)
    
    if ticketType == "" {
        ticketType = "question"
    }
    if priority == "" {
        priority = "medium"
    }
    
    // Create ticket using existing store pattern
    ticket, err := h.Store.CreateAgentTicket(ctx, &store.CreateAgentTicket{
        TenantID:    tenantID,
        TicketType:  ticketType,
        Description: description,
        Priority:    priority,
        Status:      "open",
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create ticket: %w", err)
    }
    
    return map[string]any{
        "ticket_id": ticket.ID,
        "status":    "open",
    }, nil
}
```

---

## 5. Detailed Schema (Multi-Driver)

### 5.1 CockroachDB Schema

```sql
-- Skill execution state (the "soul" of durable execution)
CREATE TABLE agent_skill_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id INT REFERENCES agent_tenants(id),
    conversation_id UUID NOT NULL,
    parent_execution_id UUID REFERENCES agent_skill_executions(id),
    
    -- DAG definition (stored as JSON, parsed at execution)
    skill_graph JSONB NOT NULL,
    
    -- Current state
    status TEXT NOT NULL DEFAULT 'pending',  -- pending, running, completed, failed, suspended
    current_node TEXT,                        -- which skill is executing
    completed_nodes JSONB DEFAULT '{}',      -- map of skill_name → result
    failed_nodes JSONB DEFAULT '{}',         -- map of skill_name → error
    
    -- Checkpoint data (for resume)
    checkpoint_data JSONB DEFAULT '{}',      -- intermediate results
    last_checkpoint_at TIMESTAMPTZ,
    
    -- Error tracking
    error_count INT DEFAULT 0,
    last_error TEXT,
    retry_after TIMESTAMPTZ,
    
    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    
    -- Indexes
    INDEX idx_skill_executions_tenant (tenant_id),
    INDEX idx_skill_executions_conversation (conversation_id),
    INDEX idx_skill_executions_status (status),
    INDEX idx_skill_executions_retry (retry_after) WHERE status IN ('pending', 'running')
);

-- Individual skill execution logs (audit trail)
CREATE TABLE agent_skill_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID REFERENCES agent_skill_executions(id),
    skill_name TEXT NOT NULL,
    
    -- Execution details
    status TEXT NOT NULL,  -- started, completed, failed, retried
    input JSONB,
    output JSONB,
    error TEXT,
    
    -- Timing
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INT,
    
    -- Retry info
    attempt INT DEFAULT 1,
    max_retries INT DEFAULT 3,
    
    -- Indexes
    INDEX idx_skill_logs_execution (execution_id),
    INDEX idx_skill_logs_skill (skill_name),
    INDEX idx_skill_logs_status (status)
);

-- Skill definitions (parsed from SCRIPT.md)
CREATE TABLE agent_skill_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id INT REFERENCES agent_tenants(id),
    skill_name TEXT NOT NULL,
    skill_content TEXT NOT NULL,  -- raw SCRIPT.md content
    parsed_graph JSONB NOT NULL,  -- parsed DAG structure
    version INT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Unique constraint
    UNIQUE(tenant_id, skill_name),
    
    -- Indexes
    INDEX idx_skill_definitions_tenant (tenant_id)
);
```

### 5.2 SQLite Schema

```sql
-- Skill execution state
CREATE TABLE IF NOT EXISTS agent_skill_executions (
    id TEXT PRIMARY KEY,  -- UUID as TEXT
    tenant_id INTEGER DEFAULT NULL,
    conversation_id TEXT NOT NULL,
    parent_execution_id TEXT DEFAULT NULL,
    
    -- DAG definition (JSON as TEXT)
    skill_graph TEXT NOT NULL DEFAULT '{}',
    
    -- Current state
    status TEXT NOT NULL DEFAULT 'pending',
    current_node TEXT,
    completed_nodes TEXT DEFAULT '{}',
    failed_nodes TEXT DEFAULT '{}',
    
    -- Checkpoint data
    checkpoint_data TEXT DEFAULT '{}',
    last_checkpoint_at INTEGER DEFAULT 0,  -- Unix epoch
    
    -- Error tracking
    error_count INTEGER DEFAULT 0,
    last_error TEXT,
    retry_after INTEGER DEFAULT 0,  -- Unix epoch
    
    -- Metadata
    created_at INTEGER DEFAULT 0,
    updated_at INTEGER DEFAULT 0,
    completed_at INTEGER DEFAULT 0
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_skill_executions_tenant ON agent_skill_executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skill_executions_conversation ON agent_skill_executions(conversation_id);
CREATE INDEX IF NOT EXISTS idx_skill_executions_status ON agent_skill_executions(status);
CREATE INDEX IF NOT EXISTS idx_skill_executions_retry ON agent_skill_executions(retry_after) WHERE status IN ('pending', 'running');

-- Individual skill execution logs
CREATE TABLE IF NOT EXISTS agent_skill_logs (
    id TEXT PRIMARY KEY,  -- UUID as TEXT
    execution_id TEXT NOT NULL,
    skill_name TEXT NOT NULL,
    
    status TEXT NOT NULL,
    input TEXT DEFAULT '{}',
    output TEXT DEFAULT '{}',
    error TEXT,
    
    started_at INTEGER DEFAULT 0,
    completed_at INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    
    attempt INTEGER DEFAULT 1,
    max_retries INTEGER DEFAULT 3
);

CREATE INDEX IF NOT EXISTS idx_skill_logs_execution ON agent_skill_logs(execution_id);
CREATE INDEX IF NOT EXISTS idx_skill_logs_skill ON agent_skill_logs(skill_name);
CREATE INDEX IF NOT EXISTS idx_skill_logs_status ON agent_skill_logs(status);

-- Skill definitions
CREATE TABLE IF NOT EXISTS agent_skill_definitions (
    id TEXT PRIMARY KEY,  -- UUID as TEXT
    tenant_id INTEGER DEFAULT NULL,
    skill_name TEXT NOT NULL,
    skill_content TEXT NOT NULL,
    parsed_graph TEXT NOT NULL DEFAULT '{}',
    version INTEGER DEFAULT 1,
    created_at INTEGER DEFAULT 0,
    updated_at INTEGER DEFAULT 0,
    
    UNIQUE(tenant_id, skill_name)
);

CREATE INDEX IF NOT EXISTS idx_skill_definitions_tenant ON agent_skill_definitions(tenant_id);
```

### 5.3 Go Data Structures

```go
// SkillGraph represents the DAG of skills to execute
type SkillGraph struct {
    Name        string                `json:"name"`
    Description string                `json:"description"`
    Nodes       map[string]*SkillNode `json:"nodes"`
    Edges       []SkillEdge           `json:"edges"`
}

// SkillNode represents a single skill in the DAG
type SkillNode struct {
    Name         string         `json:"name"`
    Handler      string         `json:"handler"`
    InputSchema  map[string]any `json:"input_schema"`
    OutputSchema map[string]any `json:"output_schema"`
    DependsOn    []string       `json:"depends_on"`
    Condition    string         `json:"condition,omitempty"`
    MaxRetries   int            `json:"max_retries"`
    Timeout      time.Duration  `json:"timeout"`
    Instructions string         `json:"instructions,omitempty"`
}

// SkillEdge represents a dependency between skills
type SkillEdge struct {
    From string `json:"from"`
    To   string `json:"to"`
}

// SkillExecution represents the state of a skill execution
type SkillExecution struct {
    ID               string         `json:"id"`
    TenantID         int32          `json:"tenant_id"`
    ConversationID   string         `json:"conversation_id"`
    ParentExecutionID *string        `json:"parent_execution_id,omitempty"`
    SkillGraph       *SkillGraph    `json:"skill_graph"`
    Status           string         `json:"status"`
    CurrentNode      string         `json:"current_node"`
    CompletedNodes   map[string]any `json:"completed_nodes"`
    FailedNodes      map[string]any `json:"failed_nodes"`
    CheckpointData   map[string]any `json:"checkpoint_data"`
    LastCheckpoint   time.Time      `json:"last_checkpoint"`
    ErrorCount       int            `json:"error_count"`
    LastError        string         `json:"last_error"`
    RetryAfter       time.Time      `json:"retry_after"`
    CreatedAt        time.Time      `json:"created_at"`
    UpdatedAt        time.Time      `json:"updated_at"`
    CompletedAt      *time.Time     `json:"completed_at,omitempty"`
}

// SkillLog represents a single skill execution log entry
type SkillLog struct {
    ID          string         `json:"id"`
    ExecutionID string         `json:"execution_id"`
    SkillName   string         `json:"skill_name"`
    Status      string         `json:"status"`
    Input       map[string]any `json:"input"`
    Output      map[string]any `json:"output"`
    Error       string         `json:"error,omitempty"`
    StartedAt   time.Time      `json:"started_at"`
    CompletedAt *time.Time     `json:"completed_at,omitempty"`
    DurationMs  int            `json:"duration_ms"`
    Attempt     int            `json:"attempt"`
    MaxRetries  int            `json:"max_retries"`
}
```

---

## 6. State Machine (from state_machine.md)

### 6.1 States

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
               └───────────┘              └──────────┘ └────────┘
                              resource      TERMINAL      TERMINAL
                              exhaustion
```

### 6.2 Allowed Transitions

| From | To | Trigger |
|------|----|---------|
| `pending` | `running` | Executor picks up next node |
| `running` | `completed` | All nodes finished |
| `running` | `failed` | Node exhausted retries, no more runnable nodes |
| `running` | `pending` | **Crash recovery only** (startup worker R6) |
| `running` | `suspended` | Resource exhaustion (OOM, rate-limited) |
| `suspended` | `pending` | Operator explicitly resumes |

### 6.3 Disallowed Transitions

| Transition | Why Not |
|------------|---------|
| `completed` → `running` | Completed is a historical fact. "Run it again" = new execution. |
| `failed` → `pending` | Mutating a failed record destroys the audit trail. Clone instead. |

### 6.4 Clone Pattern for Retry

```go
// RetryFromFailure creates a new execution from a failed one
func (e *SkillExecutor) RetryFromFailure(ctx context.Context, failedExecID string) (*SkillExecution, error) {
    original, err := e.store.GetSkillExecution(ctx, failedExecID)
    if err != nil {
        return nil, err
    }
    if original.Status != "failed" {
        return nil, fmt.Errorf("can only retry failed executions, got %s", original.Status)
    }
    
    clone := &SkillExecution{
        TenantID:          original.TenantID,
        ConversationID:    original.ConversationID,
        SkillGraph:        original.SkillGraph,
        Status:            "pending",
        CompletedNodes:    original.CompletedNodes,  // carry over successes
        FailedNodes:       map[string]any{},          // reset failures
        CheckpointData:    original.CheckpointData,   // carry over intermediate data
        ParentExecutionID: &original.ID,              // audit link
    }
    return e.store.CreateSkillExecution(ctx, clone)
}
```

---

## 7. Error Handling Strategy

### 7.1 Error Types and Responses

| Error Type | Response | Retry? | Checkpoint? |
|------------|----------|--------|-------------|
| Transient (network, timeout) | Exponential backoff | Yes (3x) | Yes |
| Permanent (invalid input) | Mark failed, continue DAG | No | Yes |
| Resource (memory, disk) | Suspend execution | Yes (1x) | Yes |
| Logic (handler error) | Mark failed, continue DAG | No | Yes |

### 7.2 Retry Logic

```go
func (e *SkillExecutor) ExecuteWithRetry(ctx context.Context, executionID string, node SkillNode) error {
    for attempt := 0; attempt <= node.MaxRetries; attempt++ {
        // Checkpoint: "I'm about to execute this skill"
        e.store.CheckpointExecution(executionID, node.Name, "running", attempt)
        
        // Execute the handler with timeout
        ctx, cancel := context.WithTimeout(ctx, node.Timeout)
        result, err := node.Handler.Execute(ctx, e.tenantID, node.Input)
        cancel()
        
        if err == nil {
            // Success: save result and mark complete
            e.store.CompleteSkillExecution(executionID, node.Name, result)
            return nil
        }
        
        // Failure: log and retry
        e.store.LogSkillFailure(executionID, node.Name, err, attempt)
        
        // Check if error is retryable
        if !isRetryableError(err) {
            break
        }
        
        if attempt < node.MaxRetries {
            // Exponential backoff
            delay := e.retryDelay * time.Duration(1<<attempt)
            time.Sleep(delay)
        }
    }
    
    // Max retries exceeded: mark as failed, continue DAG
    e.store.MarkSkillFailed(executionID, node.Name)
    return nil
}
```

### 7.3 Condition Evaluation (CEL)

```go
// evaluateCondition uses CEL to evaluate skill guard conditions
func evaluateCondition(condition string, checkpointData map[string]any) bool {
    if condition == "" {
        return true // no condition = always execute
    }
    
    // Create CEL environment with checkpoint data
    env, err := cel.NewEnv(
        cel.Variable("checkpoint", cel.MapType(cel.StringType, cel.DynType)),
    )
    if err != nil {
        slog.Error("Failed to create CEL environment", "error", err)
        return false
    }
    
    // Parse and evaluate condition
    ast, issues := env.Compile(condition)
    if issues != nil() {
        slog.Error("Failed to compile CEL condition", "condition", condition, "issues", issues)
        return false
    }
    
    program, err := env.Program(ast)
    if err != nil {
        slog.Error("Failed to create CEL program", "error", err)
        return false
    }
    
    out, _, err := program.Eval(map[string]any{
        "checkpoint": checkpointData,
    })
    if err != nil {
        slog.Error("Failed to evaluate CEL condition", "error", err)
        return false
    }
    
    result, ok := out.Value().(bool)
    if !ok {
        slog.Error("CEL condition did not return boolean", "condition", condition)
        return false
    }
    
    return result
}
```

---

## 8. Integration with Existing Pipeline

### 8.1 System Prompt Injection

Add skill definitions to `buildSystemPrompt()` between SECTION 5 (Conversation Flow) and SECTION 6 (Policies & Rules):

```go
// In service.go buildSystemPrompt()

// SECTION 5: CONVERSATION FLOW
if parsedScript != nil && len(parsedScript.Sections) > 0 {
    sb.WriteString("=== SECTION 5: CONVERSATION FLOW ===\n")
    // ... existing script content ...
}

// SECTION 5.5: AVAILABLE SKILLS (NEW)
if parsedScript != nil && parsedScript.HasSkills && parsedScript.SkillGraph != nil {
    sb.WriteString("\n=== SECTION 5.5: AVAILABLE SKILLS ===\n")
    sb.WriteString("You have the following skills available. Use them when appropriate.\n")
    sb.WriteString("When you want to use a skill, call it as a tool.\n\n")
    
    for name, node := range parsedScript.SkillGraph.Nodes {
        sb.WriteString(fmt.Sprintf("- %s: %s\n", name, node.Instructions))
    }
    sb.WriteString("\n")
}

// SECTION 6: POLICIES & RULES
if parsedPolicy != nil {
    sb.WriteString("=== SECTION 6: POLICIES & RULES ===\n")
    // ... existing policy content ...
}
```

### 8.2 Tool-Calling Integration

```go
// In service.go processChatWithToolCalls()

func (s *Service) processChatWithToolCalls(ctx context.Context, session *store.AgentSession, message string) (*ChatResponse, error) {
    // 1. Build system prompt (includes skill definitions)
    systemPrompt, err := s.buildSystemPrompt(ctx, session)
    if err != nil {
        return nil, err
    }
    
    // 2. Get skill tools from registry
    skillTools := s.skillRegistry.ToOpenRouterTools()
    
    // 3. Build LLM request with tools
    req := openrouter.ChatRequest{
        Model:    session.Model,
        Messages: buildMessages(systemPrompt, session.History, message),
        Tools:    skillTools,
    }
    
    // 4. Call LLM
    resp, err := s.llmClient.Chat(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // 5. Handle tool calls
    for len(resp.ToolCalls) > 0 {
        // Execute each tool call
        for _, toolCall := range resp.ToolCalls {
            // Execute handler
            result, err := s.executeSkillHandler(ctx, session, toolCall)
            if err != nil {
                return nil, err
            }
            
            // Add tool response to conversation
            session.History = append(session.History,
                &store.Message{Role: "assistant", ToolCalls: []openrouter.ToolCall{toolCall}},
                &store.Message{Role: "tool", Content: result, ToolCallID: toolCall.ID},
            )
            
            // Checkpoint
            s.checkpointExecution(ctx, session, toolCall)
        }
        
        // Continue conversation with tool results
        resp, err = s.llmClient.Chat(ctx, openrouter.ChatRequest{
            Model:    session.Model,
            Messages: buildMessagesFromHistory(session.History),
            Tools:    skillTools,
        })
        if err != nil {
            return nil, err
        }
    }
    
    // 6. Return final response
    return &ChatResponse{
        Content: resp.Content,
    }, nil
}
```

### 8.3 DAG Constraint Enforcement

```go
// executeSkillHandler enforces DAG constraints before executing
func (s *Service) executeSkillHandler(ctx context.Context, session *store.AgentSession, toolCall openrouter.ToolCall) (string, error) {
    // 1. Get current execution state
    execution, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{
        ConversationID: &session.ID,
    })
    if err != nil {
        return "", err
    }
    
    // 2. Find the skill node
    node, ok := execution.SkillGraph.Nodes[toolCall.Function.Name]
    if !ok {
        return "", fmt.Errorf("unknown skill: %s", toolCall.Function.Name)
    }
    
    // 3. Check dependencies
    for _, dep := range node.DependsOn {
        if _, ok := execution.CompletedNodes[dep]; !ok {
            return fmt.Sprintf("Dependency not met: %s must complete first", dep), nil
        }
    }
    
    // 4. Check condition
    if node.Condition != "" {
        if !evaluateCondition(node.Condition, execution.CheckpointData) {
            return fmt.Sprintf("Condition not met: %s", node.Condition), nil
        }
    }
    
    // 5. Execute handler
    handler, ok := s.skillRegistry.Get(toolCall.Function.Name)
    if !ok {
        return "", fmt.Errorf("handler not found: %s", toolCall.Function.Name)
    }
    
    result, err := handler.Execute(ctx, session.TenantID, toolCall.Arguments)
    if err != nil {
        return "", err
    }
    
    // 6. Update execution state
    execution.CompletedNodes[toolCall.Function.Name] = result
    execution.CheckpointData[toolCall.Function.Name] = result
    s.store.UpdateSkillExecution(ctx, execution)
    
    // 7. Log execution
    s.store.CreateSkillLog(ctx, &store.CreateSkillLog{
        ExecutionID: execution.ID,
        SkillName:   toolCall.Function.Name,
        Status:      "completed",
        Input:       toolCall.Arguments,
        Output:      result,
    })
    
    return json.Marshal(result)
}
```

---

## 9. Tenant Scoping

### 9.1 Tenant Isolation Pattern

Every `SkillExecutor` method must extract tenant ID from context and scope all queries:

```go
// StartExecution creates a new skill execution with tenant scoping
func (e *SkillExecutor) StartExecution(c echo.Context, graph *SkillGraph) (*SkillExecution, error) {
    tenantID := getTenantFromContext(c)
    if tenantID == nil {
        return nil, echo.NewHTTPError(http.StatusForbidden, "tenant context required")
    }
    
    // Verify the skill definition belongs to this tenant
    def, err := e.store.GetAgentSkillDefinition(c.Request().Context(), &store.FindAgentSkillDefinition{
        TenantID:  tenantID,
        SkillName: &graph.Name,
    })
    if err != nil || def == nil {
        return nil, echo.NewHTTPError(http.StatusNotFound, "skill not found for tenant")
    }
    
    execution := &SkillExecution{
        TenantID:     *tenantID,
        SkillGraph:   graph,
        Status:       "pending",
    }
    return e.store.CreateSkillExecution(c.Request().Context(), execution)
}

// GetExecution retrieves a skill execution with tenant scoping
func (e *SkillExecutor) GetExecution(c echo.Context, executionID string) (*SkillExecution, error) {
    tenantID := getTenantFromContext(c)
    if tenantID == nil {
        return nil, echo.NewHTTPError(http.StatusForbidden, "tenant context required")
    }
    
    execution, err := e.store.GetSkillExecution(c.Request().Context(), &store.FindSkillExecution{
        ID:       &executionID,
        TenantID: tenantID,
    })
    if err != nil {
        return nil, err
    }
    
    // Verify tenant ownership
    if execution.TenantID != *tenantID && !isSuperUser(c) {
        return nil, echo.NewHTTPError(http.StatusForbidden, "permission denied")
    }
    
    return execution, nil
}
```

---

## 10. Startup Recovery Worker

### 10.1 Recovery Goroutine

```go
// In NewService(), after existing startup workers
func (s *Service) startSkillRecoveryWorker() {
    // Enable via environment variable
    if os.Getenv("SKILL_RECOVERY_ENABLED") != "true" {
        return
    }
    
    go func() {
        // Wait for subsystems to initialize
        time.Sleep(10 * time.Second)
        
        ctx := context.Background()
        slog.Info("Starting skill execution recovery worker")
        
        // Query for pending/running executions
        executions, err := s.store.ListPendingSkillExecutions(ctx)
        if err != nil {
            slog.Error("Failed to list pending skill executions", "error", err)
            return
        }
        
        for _, exec := range executions {
            // Conservative approach: mark 'running' nodes as 'pending' for retry
            // Don't auto-resume — let the next user message or scheduled retry pick them up
            if exec.Status == "running" {
                exec.Status = "pending"
                if exec.CurrentNode != "" {
                    exec.FailedNodes[exec.CurrentNode] = "process restart"
                }
                exec.CurrentNode = ""
                exec.UpdatedAt = time.Now()
                
                if err := s.store.UpdateSkillExecution(ctx, exec); err != nil {
                    slog.Error("Failed to update skill execution", 
                        "execution_id", exec.ID, 
                        "error", err)
                    continue
                }
                
                slog.Info("Recovered skill execution after restart",
                    "execution_id", exec.ID,
                    "tenant_id", exec.TenantID,
                    "completed_nodes", len(exec.CompletedNodes))
            }
        }
        
        slog.Info("Skill execution recovery complete", "count", len(executions))
    }()
}
```

---

## 11. Token Budget Management

### 11.1 Progressive Disclosure

```go
// In buildSystemPrompt(), cap skill definitions at 1,500 tokens
const maxSkillTokens = 1500

// emitSkillDefinitions emits only skill names and descriptions (metadata)
func emitSkillDefinitions(graph *SkillGraph) string {
    if graph == nil || len(graph.Nodes) == 0 {
        return ""
    }
    
    var sb strings.Builder
    sb.WriteString("=== AVAILABLE SKILLS ===\n")
    sb.WriteString("You have the following skills available. Use them when appropriate.\n")
    sb.WriteString("When you want to use a skill, call it as a tool.\n\n")
    
    for name, node := range graph.Nodes {
        // Only emit name and one-line description (not full instructions)
        sb.WriteString(fmt.Sprintf("- %s: %s\n", name, node.Instructions))
    }
    sb.WriteString("\n")
    
    return sb.String()
}

// emitFullSkillInstructions emits full instructions when skill is activated
func emitFullSkillInstructions(node *SkillNode) string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("## Skill: %s\n\n", node.Name))
    sb.WriteString(node.Instructions)
    sb.WriteString("\n\n")
    sb.WriteString("### Input Schema\n")
    // ... emit input schema ...
    sb.WriteString("\n### Output Schema\n")
    // ... emit output schema ...
    
    return sb.String()
}
```

### 11.2 Budget Allocation

| Component | Typical Tokens | Max |
|-----------|---------------|-----|
| Identity + Policies | ~800 | 1,500 |
| RAG context | ~2,000 | 6,000 |
| Services + Exclusions | ~500 | 1,000 |
| Conversation flow | ~300 | 800 |
| **Skill definitions** | **~500** | **1,500** |
| FAQs | ~500 | 1,000 |
| Observational memory | ~500 | 2,000 |
| **Total system prompt** | **~5,100** | **13,800** |

---

## 12. End-to-End Workflow

### 12.1 Skill Definition Flow

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Tenant uploads SCRIPT.md with @skill annotations        │
│ 2. Parser extracts skill blocks and builds DAG              │
│ 3. Validate DAG (no cycles, all deps exist)                 │
│ 4. Store in agent_skill_definitions table                   │
│ 5. Register handlers in SkillRegistry                       │
│ 6. On next chat: inject skill definitions in system prompt  │
└─────────────────────────────────────────────────────────────┘
```

### 12.2 Skill Execution Flow

```
┌─────────────────────────────────────────────────────────────┐
│ 1. User sends message                                       │
│ 2. System prompt includes available skills                  │
│ 3. LLM decides to use a skill (tool-calling)                │
│ 4. Runtime enforces DAG constraints                         │
│ 5. Execute builtin handler OR LLM reasoning                 │
│ 6. Return result as tool response to LLM                    │
│ 7. LLM continues with tool result in context                │
│ 8. Checkpoint after each tool call resolves                  │
│ 9. Repeat until LLM produces final response                 │
└─────────────────────────────────────────────────────────────┘
```

### 12.3 Checkpoint and Resume Flow

```
┌─────────────────────────────────────────────────────────────┐
│ On Crash/Restart:                                           │
│   1. Startup recovery worker queries pending/running        │
│   2. Marks running nodes as pending for retry               │
│   3. Next user message resumes execution                    │
│   4. Operator can also manually resume                      │
└─────────────────────────────────────────────────────────────┘
```

---

## 13. Observational Memory Interaction

### 13.1 OM Integration

Skill executions interact with OM naturally through conversation history:

1. **Skill inputs/outputs are tool-call/tool-response messages** — these are part of conversation history
2. **ObserverBuffer compresses them** like any other messages
3. **Token budget includes tool messages** — skill outputs count toward context window
4. **No special OM integration needed** — the existing pipeline handles it

```go
// In ObserverBuffer, tool messages are compressed like any other messages
func (b *ObserverBuffer) AddMessage(msg *store.Message) {
    // Tool messages are part of conversation history
    // They will be compressed when token threshold is reached
    b.messages = append(b.messages, msg)
    
    // Check if compression is needed
    if b.tokenCount > b.tokenThreshold {
        b.compress()
    }
}
```

---

## 14. Adversarial Plan Review (Updated)

### 14.1 Structural Flaws Addressed

| Flaw | Resolution |
|------|------------|
| F1: SKILLS.md doesn't reconcile with 3-file model | Extended SCRIPT.md with `@skill` annotations |
| F2: Handler Registration assumes compiled Go code | Hybrid model: builtin handlers + LLM executor |
| F3: DAG executor disconnected from chat pipeline | Integrated into existing LLM tool-calling pipeline |
| F4: SQLite parity ignored | Added SQLite equivalents for all tables |
| F5: No tenant isolation in skill execution | Added tenant scoping to every executor method |
| F6: agentskills.io compliance is cosmetic | Used bchat's existing annotation format |

### 14.2 Significant Gaps Addressed

| Gap | Resolution |
|-----|------------|
| G1: Missing integration with system prompt | Added SECTION 5.5 in buildSystemPrompt() |
| G2: Missing condition evaluator | Reuse CEL (existing in codebase) |
| G3: Missing concurrency model | LLM-driven execution (no parallel DAG needed) |
| G4: Missing state machine transitions | 5 states, 6 transitions, clone pattern for retry |
| G5: Missing integration with AgentEvent | Skill executions are separate but auditable |
| G6: Missing handler interface | Defined SkillHandler interface |
| G7: Missing startup recovery worker | Added conservative recovery goroutine |
| G8: Missing schema for input/output | JSON Schema for handler validation |
| G9: Missing Observational Memory interaction | Natural integration via conversation history |

### 14.3 Open Questions Resolved

| Question | Resolution |
|----------|------------|
| Q1: Token budget | 1,500 token cap + progressive disclosure |
| Q2: Determinism | DAG constrains, LLM decides |
| Q3: Backward compatibility | Fully compatible via `@skill` annotation detection |
| Q4: Testing | 3-layer: unit + mock LLM + simulation framework |

---

## 15. Implementation Phases

### Phase 1: Core Infrastructure (Days 1-3)
- [ ] Extend parser.go to extract `@skill` annotations from SCRIPT.md
- [ ] Create SQLite and CockroachDB tables with migrations
- [ ] Build SkillGraph parsing and validation
- [ ] Implement state machine transitions

### Phase 2: Execution Engine (Days 4-6)
- [ ] Build SkillRegistry with builtin handlers
- [ ] Implement SkillHandler interface
- [ ] Add CEL condition evaluation
- [ ] Create checkpoint/resume logic

### Phase 3: Integration (Days 7-9)
- [ ] Integrate with existing LLM tool-calling pipeline
- [ ] Add skill definitions to buildSystemPrompt()
- [ ] Implement tenant scoping on all executor methods
- [ ] Add API endpoints for skill management

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

## 16. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Execution success rate | 99.9% | CockroachDB logs |
| Checkpoint frequency | After each tool call | Configuration |
| Resume success rate | 100% | Test scenarios |
| Response time | < 2s | API metrics |
| Concurrent executions | 100+ | Load testing |
| Token budget compliance | < 1,500 tokens | Prompt analysis |

---

## 17. Testing Strategy

### 17.1 Layer 1: Unit Tests (No LLM, No DB)

```go
func TestSkillGraphCycleDetection(t *testing.T) {
    graph := &SkillGraph{
        Nodes: map[string]*SkillNode{
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

### 17.2 Layer 2: Integration Tests with Mock LLM

```go
func TestSkillExecutionWithMockLLM(t *testing.T) {
    // Start httptest.Server that returns canned tool-call responses
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Return a tool_calls response that invokes classify_intent
        json.NewEncoder(w).Encode(mockToolCallResponse)
    }))
    os.Setenv("OPENROUTER_API_BASE_URL", mockServer.URL)
    
    // Run conversation
    response, err := service.ProcessChat(ctx, session, "I need help with water damage")
    assert.NoError(t, err)
    
    // Verify skills were called
    assert.Contains(t, response.ToolsCalled, "builtin:classify_intent")
}
```

### 17.3 Layer 3: Simulation Framework

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

## 18. API Endpoints

### 18.1 Skill Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/agent/:slug/skills` | List skill definitions for tenant |
| POST | `/api/v1/agent/:slug/skills/validate` | Validate SCRIPT.md skill annotations |
| GET | `/api/v1/agent/:slug/executions` | List skill executions for tenant |
| GET | `/api/v1/agent/:slug/executions/:id` | Get execution details |
| POST | `/api/v1/agent/:slug/executions/:id/retry` | Clone failed execution for retry |
| POST | `/api/v1/agent/:slug/executions/:id/suspend` | Suspend running execution |
| POST | `/api/v1/agent/:slug/executions/:id/resume` | Resume suspended execution |

### 18.2 Execution Flow

```
POST /api/v1/agent/:slug/executions/:id/retry
{
    "execution_id": "uuid-of-failed-execution"
}

Response:
{
    "execution": {
        "id": "uuid-of-new-execution",
        "status": "pending",
        "parent_execution_id": "uuid-of-failed-execution",
        "completed_nodes": {
            "classify_intent": {"intent": "complaint", "confidence": 0.95}
        }
    }
}
```

---

## 19. Environment Variables

```bash
# Skill Execution
SKILL_RECOVERY_ENABLED=true                    # Enable startup recovery worker
SKILL_MAX_RETRIES=3                            # Default max retries per skill
SKILL_TIMEOUT=30s                              # Default timeout per skill
SKILL_CHECKPOINT_INTERVAL=1                    # Checkpoint after N tool calls

# CEL Condition Evaluation
CEL_CONDITION_ENABLED=true                     # Enable CEL condition evaluation

# Token Budget
SKILL_MAX_TOKENS=1500                          # Max tokens for skill definitions in prompt
```

---

## 20. Migration Plan

### 20.1 Step 1: Create Migration Files

```bash
task migrate:new NAME=add_skill_executions
```

### 20.2 Step 2: Write SQL for Each Driver

- `store/migration/sqlite/XX__add_skill_executions.sql`
- `store/migration/postgres/XX__add_skill_executions.sql`

### 20.3 Step 3: Validate Parity

```bash
task validate:parity
task validate:schema
```

### 20.4 Step 4: Update Store Interface

Add methods to `store/driver.go`:

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

---

## 21. Risk Mitigation

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

## 22. Open Questions (Resolved)

1. **Condition Evaluation:** Reuse CEL (existing in codebase) — ✅ Resolved
2. **Checkpoint Frequency:** After each tool call — ✅ Resolved
3. **Handler Timeout:** 30s default — ✅ Resolved
4. **Error Notification:** Log errors, notify admin on critical failures — ✅ Resolved
5. **Skill Composition:** No nested DAGs initially — ✅ Resolved
6. **State Eviction:** 30 days retention — ✅ Resolved
7. **Parallel Execution:** LLM-driven (no parallel DAG needed) — ✅ Resolved

---


Viewed plan2_review.md:1-17
Searched for "start|stop|signal|trigger|lifecycle"
Searched for "start"

**No**, `plan2.md` did **not** include explicit **Start and Stop signals** for initiating, driving, and terminating an agent pipeline. In `plan2.md`, skill executions were triggered implicitly when a user sent a chat message, and stopped when all DAG nodes completed or failed.

To function as a true automation engine (like n8n), bchat needs a **first-class Lifecycle & Signal Mechanism**.

Here is the design to add **Start and Stop Signals** into the bchat architecture:

---

## 1. Declarative Start & Stop Annotations in `SCRIPT.md`

We extend `SCRIPT.md` with explicit `@trigger` (Start Signal) and `@signal` (Stop Signal) annotations:

```markdown
<!-- @trigger: start, type: "event", event_type: "lead_created" -->
<!-- @persona: "Emergency Response Agent", tone: "empathetic", role: "Water Damage Specialist" -->
## Workflow Opening
Adopt persona and process new emergency lead.

<!-- @skill: classify_intent -->
<!-- @handler: builtin:classify_intent -->
## Classify Intent
...

<!-- @skill: create_ticket -->
<!-- @handler: builtin:create_ticket -->
## Create Support Ticket
...

<!-- @signal: stop, condition: "create_ticket.ticket_id != ''", emit_event: "pipeline_completed" -->
## Workflow Completion
Stop pipeline execution once ticket is logged and emit completion event.
```

---

## 2. Start Signals (Initiation Mechanisms)

An agent pipeline can be started via three distinct signals:

| Signal Type | Trigger Source | Mechanism | Use Case |
|-------------|----------------|-----------|----------|
| **Event Signal** | Internal `AgentEvent` / Webhook | System receives `lead_created` or `form_submitted` event → auto-spawns a `SkillExecution`. | Out-of-band automation (n8n style) |
| **API Start Signal** | `POST /api/v1/agent/:slug/workflows/start` | External system or admin explicitly starts a pipeline session with initial payload parameters. | Programmatic invocation |
| **Chat Signal** | `POST /api/v1/agent/:slug/chat` | User sends an initial message that matches the script's start condition. | Interactive widget |

### Start Implementation (Go)

```go
// StartWorkflowSignal initiates an agent workflow from an external event or API call
func (s *Service) StartWorkflowSignal(ctx context.Context, tenantID int32, triggerType, eventType string, payload map[string]any) (*SkillExecution, error) {
    // 1. Fetch active SCRIPT.md for tenant
    script, err := s.store.GetAgentTenantScript(ctx, &store.FindAgentTenantScript{TenantID: &tenantID})
    if err != nil || script == nil {
        return nil, fmt.Errorf("no script configured for tenant %d", tenantID)
    }

    // 2. Parse DAG & verify start trigger matching
    parsed, err := s.parser.ParseScriptWithSkills(script.Content)
    if err != nil || !parsed.HasSkills {
        return nil, fmt.Errorf("script does not contain valid skill graph")
    }

    // 3. Instantiate durable SkillExecution record
    execution := &store.SkillExecution{
        TenantID:       tenantID,
        ConversationID: uuid.New().String(),
        SkillGraph:     parsed.SkillGraph,
        Status:         "running",
        CheckpointData: payload, // seed checkpoint data from start signal payload
        CreatedAt:      time.Now(),
    }
    
    return s.store.CreateSkillExecution(ctx, execution)
}
```

---

## 3. Stop Signals (Termination & Emission Mechanisms)

An execution stops gracefully or explicitly via three mechanisms:

| Signal Type | Trigger Source | Action |
|-------------|----------------|--------|
| **Declarative Stop Condition** | `<!-- @signal: stop, condition: "..." -->` | CEL evaluator checks condition after each step. When `true`, transitions to `completed`, logs summary, and halts. |
| **Explicit Stop API** | `POST /api/v1/agent/:slug/executions/:id/stop` | Operator/system manually stops a running agent (sets state to `stopped`). |
| **Outbound Event Signal** | `emit_event: "workflow_completed"` | Emits an `AgentEvent` (via bchat's existing integration worker) to deliver webhooks to n8n, Slack, or CRM systems. |

### Stop Evaluation Logic (Go)

```go
// EvaluateStopSignal checks if a stop condition has been met
func (s *Service) EvaluateStopSignal(ctx context.Context, exec *store.SkillExecution, stopCondition, emitEvent string) (bool, error) {
    if stopCondition != "" && evaluateCondition(stopCondition, exec.CheckpointData) {
        exec.Status = "completed"
        exec.CompletedAt = time.Now()
        
        if err := s.store.UpdateSkillExecution(ctx, exec); err != nil {
            return false, err
        }

        // Emit outbound event signal if configured
        if emitEvent != "" {
            s.store.CreateAgentEvent(ctx, &store.AgentEvent{
                TenantID:  exec.TenantID,
                EventType: emitEvent,
                Payload:   toJSON(exec.CheckpointData),
                Status:    "pending",
            })
        }
        return true, nil // Signal stopped execution cleanly
    }
    return false, nil
}
```

---

## 4. State Machine Update with `stopped` State

We update the state machine from `plan2.md` to include explicit `start` and `stop` controls:

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

---

## Summary of Additions to Plan 2

1. **`@trigger` & `@signal` annotations** in `SCRIPT.md` for declarative start/stop boundaries.
2. **`POST /api/v1/agent/:slug/workflows/start`** for non-chat event-driven triggers.
3. **`POST /api/v1/agent/:slug/executions/:id/stop`** for explicit execution termination.
4. **Outbound `AgentEvent` emission on stop** to communicate completion back to n8n / external webhooks.

Would you like me to update `bugs/059/plan2.md` directly to append this Start/Stop Signal architecture section?

**This plan is ready for implementation.** All structural flaws and gaps from the adversarial review have been addressed. The architecture integrates naturally with the existing bchat codebase while adding durable execution capabilities.

**Next step:** Begin Phase 1 implementation (Core Infrastructure).
