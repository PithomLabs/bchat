# bchat Durable Execution Architecture - Grand Master Plan

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

**Why agentskills.io Compliance?**
- Standard format recognized by major AI tools (Claude, Cursor, VS Code, etc.)
- Progressive disclosure reduces context window usage
- Portable across agent implementations
- Community adoption and ecosystem support

**Why CockroachDB?**
- Distributed SQL with serializable transactions
- Native vector indexing for RAG
- Global distribution for multi-tenant deployments
- ACID compliance for durable execution state

---

## 1. Executive Summary

**Goal:** Transform bchat from a reactive chat agent into a durable automation pipeline that survives failures, resumes from checkpoints, and provides full audit trails — all driven by declarative markdown (SKILLS.md).

**Key Differentiator vs n8n:** "Declarative markdown config + compiled Go skills + CockroachDB persistent memory vs visual drag-and-drop + JavaScript nodes + no built-in AI memory."

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
│  │  SKILLS.md  │───▶│ SkillParser │───▶│ SkillGraph  │         │
│  │  (Declarative)│  │ (Annotation)│    │ (DAG)       │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                                               │                 │
│                                               ▼                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Skill Execution Pipeline                    │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │   │
│  │  │  Parse   │→│ Resolve  │→│ Execute  │→│Checkpoint│  │   │
│  │  │  Stage   │ │ Deps     │ │ Skill    │ │ Stage    │  │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    CockroachDB                           │   │
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
│ 1. Parse SKILLS.md → Build DAG                              │
│ 2. Create execution record in CockroachDB                   │
│ 3. Start from root nodes (no dependencies)                  │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ FOR EACH ready node:                                        │
│   1. Mark as 'running' in CockroachDB                       │
│   2. Execute Go handler                                     │
│   3. On success:                                            │
│      - Save output to checkpoint_data                       │
│      - Mark node as 'completed'                             │
│      - Unlock dependent nodes                               │
│   4. On failure:                                            │
│      - Increment error_count                                │
│      - If retries remaining: schedule retry                 │
│      - If max retries: mark as 'failed', continue DAG       │
│   5. Checkpoint to CockroachDB after each step              │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Detailed Schema

### 3.1 SKILLS.md Format (agentskills.io compliant)

```yaml
---
name: customer-support-pipeline
description: Full customer support workflow with ticket creation and escalation
license: Proprietary
compatibility: Requires bchat agent runtime with OpenRouter API and CockroachDB
metadata:
  author: bchat
  version: "1.0"
  category: automation-pipeline
---

<!-- @skill: classify_intent -->
<!-- @handler: classifyIntentHandler -->
<!-- @input_schema: {"message": "string"} -->
<!-- @output_schema: {"intent": "string", "confidence": "number"} -->
<!-- @depends_on: none -->
<!-- @max_retries: 3 -->
<!-- @timeout: 30s -->

<!-- @skill: search_kb -->
<!-- @handler: searchKnowledgeBaseHandler -->
<!-- @input_schema: {"query": "string"} -->
<!-- @output_schema: {"results": "array", "found": "boolean"} -->
<!-- @depends_on: classify_intent -->

<!-- @skill: create_ticket -->
<!-- @handler: createTicketHandler -->
<!-- @input_schema: {"ticket_type": "string", "description": "string", "priority": "string"} -->
<!-- @output_schema: {"ticket_id": "string", "status": "string"} -->
<!-- @depends_on: search_kb -->
<!-- @condition: "search_kb.found == false" -->

<!-- @skill: respond_to_customer -->
<!-- @handler: respondToCustomerHandler -->
<!-- @input_schema: {"response": "string", "context": "object"} -->
<!-- @output_schema: {"sent": "boolean"} -->
<!-- @depends_on: create_ticket -->

<!-- @skill: escalate_to_human -->
<!-- @handler: escalateToHumanHandler -->
<!-- @input_schema: {"ticket_id": "string", "reason": "string"} -->
<!-- @output_schema: {"escalated": "boolean"} -->
<!-- @depends_on: create_ticket -->
<!-- @condition: "create_ticket.priority == 'high'" -->

## Instructions

1. **Classify Intent**: Use `classify_intent` to understand what the customer wants
2. **Search Knowledge Base**: Use `search_kb` to find solutions
3. **Create Ticket**: If no solution found, use `create_ticket` to log the issue
4. **Respond**: Use `respond_to_customer` to send a helpful response
5. **Escalate**: If priority is high, use `escalate_to_human` for immediate attention

## Error Handling

- If `classify_intent` fails, retry up to 3 times with exponential backoff
- If `search_kb` fails, continue to `create_ticket` with fallback query
- If `create_ticket` fails, log error and notify admin
- If `respond_to_customer` fails, queue for manual send
- If `escalate_to_human` fails, create high-priority ticket instead

## Checkpoints

- Checkpoint after each skill execution
- Store intermediate results in `checkpoint_data`
- Resume from last checkpoint on failure
```

### 3.2 CockroachDB Tables

```sql
-- Skill execution state (the "soul" of durable execution)
CREATE TABLE agent_skill_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id INT REFERENCES agent_tenants(id),
    conversation_id UUID NOT NULL,
    
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
    INDEX idx_skill_executions_retry (retry_after) WHERE status = 'pending'
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

-- Skill definitions (parsed from SKILLS.md)
CREATE TABLE agent_skill_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id INT REFERENCES agent_tenants(id),
    skill_name TEXT NOT NULL,
    skill_content TEXT NOT NULL,  -- raw SKILLS.md content
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

### 3.3 Go Data Structures

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
}

// SkillEdge represents a dependency between skills
type SkillEdge struct {
    From string `json:"from"`
    To   string `json:"to"`
}

// SkillExecution represents the state of a skill execution
type SkillExecution struct {
    ID              string         `json:"id"`
    TenantID        int32          `json:"tenant_id"`
    ConversationID  string         `json:"conversation_id"`
    SkillGraph      *SkillGraph    `json:"skill_graph"`
    Status          string         `json:"status"`
    CurrentNode     string         `json:"current_node"`
    CompletedNodes  map[string]any `json:"completed_nodes"`
    FailedNodes     map[string]any `json:"failed_nodes"`
    CheckpointData  map[string]any `json:"checkpoint_data"`
    LastCheckpoint  time.Time      `json:"last_checkpoint"`
    ErrorCount      int            `json:"error_count"`
    LastError       string         `json:"last_error"`
    RetryAfter      time.Time      `json:"retry_after"`
    CreatedAt       time.Time      `json:"created_at"`
    UpdatedAt       time.Time      `json:"updated_at"`
    CompletedAt     *time.Time     `json:"completed_at,omitempty"`
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

## 4. End-to-End Workflow

### 4.1 Skill Definition Flow

```
┌─────────────────────────────────────────────────────────────┐
│ 1. User uploads SKILLS.md via API                           │
│ 2. Parser extracts annotations and frontmatter              │
│ 3. Build SkillGraph DAG from dependencies                   │
│ 4. Validate DAG (no cycles, all deps exist)                 │
│ 5. Store in agent_skill_definitions table                   │
│ 6. Register handlers in SkillRegistry                       │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 Skill Execution Flow

```
┌─────────────────────────────────────────────────────────────┐
│ 1. User sends message that triggers skill                   │
│ 2. System prompt includes available skills                  │
│ 3. LLM decides to use a skill (tool-calling)                │
│ 4. Create SkillExecution record in CockroachDB              │
│ 5. Parse skill graph and resolve dependencies               │
│ 6. Start execution from root nodes                          │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ FOR EACH iteration:                                         │
│   1. Find ready nodes (all deps completed)                  │
│   2. For each ready node:                                   │
│      a. Mark as 'running'                                   │
│      b. Execute Go handler                                  │
│      c. On success: save result, mark completed             │
│      d. On failure: retry with backoff, or mark failed      │
│      e. Checkpoint to CockroachDB                           │
│   3. Check if execution complete                            │
│   4. If not, continue to next iteration                     │
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

### 4.3 Checkpoint and Resume Flow

```
┌─────────────────────────────────────────────────────────────┐
│ On Crash/Restart:                                           │
│   1. Query CockroachDB for pending/running executions       │
│   2. For each execution:                                    │
│      a. Load skill graph from checkpoint_data               │
│      b. Mark running nodes as 'pending' for retry           │
│      c. Resume execution from last checkpoint               │
│   3. Continue normal execution flow                         │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. Error Handling Strategy

### 5.1 Error Types and Responses

| Error Type | Response | Retry? | Checkpoint? |
|------------|----------|--------|-------------|
| Transient (network, timeout) | Exponential backoff | Yes (3x) | Yes |
| Permanent (invalid input) | Mark failed, continue DAG | No | Yes |
| Resource (memory, disk) | Suspend execution | Yes (1x) | Yes |
| Logic (handler error) | Mark failed, continue DAG | No | Yes |

### 5.2 Retry Logic

```go
func (e *SkillExecutor) ExecuteWithRetry(ctx context.Context, executionID string, node SkillNode) error {
    for attempt := 0; attempt <= node.MaxRetries; attempt++ {
        // Checkpoint: "I'm about to execute this skill"
        e.store.CheckpointExecution(executionID, node.Name, "running", attempt)
        
        // Execute the handler with timeout
        ctx, cancel := context.WithTimeout(ctx, node.Timeout)
        result, err := node.Handler.Execute(ctx, node.Input)
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

### 5.3 Condition Evaluation

```go
func evaluateCondition(condition string, checkpointData map[string]any) bool {
    // Simple expression evaluator for conditions
    // Supports: ==, !=, >, <, >=, <=, &&, ||
    // Example: "search_kb.found == false"
    
    evaluator := NewExpressionEvaluator(checkpointData)
    result, err := evaluator.Evaluate(condition)
    if err != nil {
        // If condition evaluation fails, default to false (don't execute)
        return false
    }
    return result.(bool)
}
```

---

## 6. How to Write Skills

### 6.1 Skill Structure

```
skills/
├── customer-support/
│   ├── SKILL.md          # Main skill definition
│   ├── scripts/          # Optional: helper scripts
│   ├── references/       # Optional: documentation
│   └── assets/           # Optional: templates
├── data-processing/
│   ├── SKILL.md
│   └── scripts/
└── multi-tenant-agent/
    ├── SKILL.md
    └── references/
```

### 6.2 SKILL.md Template

```yaml
---
name: skill-name
description: What this skill does and when to use it
license: Proprietary
compatibility: Requires bchat agent runtime
metadata:
  author: your-name
  version: "1.0"
  category: automation-pipeline
---

<!-- @skill: step1_name -->
<!-- @handler: step1Handler -->
<!-- @input_schema: {"param1": "string", "param2": "number"} -->
<!-- @output_schema: {"result": "string"} -->
<!-- @depends_on: none -->
<!-- @max_retries: 3 -->
<!-- @timeout: 30s -->

<!-- @skill: step2_name -->
<!-- @handler: step2Handler -->
<!-- @input_schema: {"data": "string"} -->
<!-- @output_schema: {"success": "boolean"} -->
<!-- @depends_on: step1_name -->
<!-- @condition: "step1_name.result != 'error'" -->

## Instructions

1. **Step 1**: Use `step1_name` to process input
2. **Step 2**: Use `step2_name` to transform data
3. **Handle Errors**: If step fails, retry with backoff

## Error Handling

- If `step1_name` fails, retry up to 3 times
- If `step2_name` fails, log error and notify admin

## Checkpoints

- Checkpoint after each step
- Store intermediate results
- Resume from last checkpoint on failure
```

### 6.3 Go Handler Registration

```go
// Register skill handlers in SkillRegistry
registry := NewSkillRegistry()

// Register handlers
registry.RegisterHandler("classifyIntentHandler", classifyIntentHandler)
registry.RegisterHandler("searchKnowledgeBaseHandler", searchKnowledgeBaseHandler)
registry.RegisterHandler("createTicketHandler", createTicketHandler)
registry.RegisterHandler("respondToCustomerHandler", respondToCustomerHandler)
registry.RegisterHandler("escalateToHumanHandler", escalateToHumanHandler)

// Handler implementations
func classifyIntentHandler(ctx context.Context, input map[string]any) (map[string]any, error) {
    message := input["message"].(string)
    
    // Call LLM to classify intent
    intent, confidence, err := classifyIntent(ctx, message)
    if err != nil {
        return nil, fmt.Errorf("failed to classify intent: %w", err)
    }
    
    return map[string]any{
        "intent":     intent,
        "confidence": confidence,
    }, nil
}
```

---

## 7. Adversarial Plan Review

### 7.1 Potential Failure Modes

| Failure Mode | Impact | Mitigation |
|--------------|--------|------------|
| CockroachDB unavailable | Cannot persist state | Fallback to SQLite (existing) |
| Handler panic | Execution stuck | Recover + mark failed |
| Infinite loop in DAG | Resource exhaustion | Max iterations + timeout |
| Condition evaluation error | Skip required steps | Default to false + log |
| Checkpoint corruption | Resume from wrong state | Verify checksums |
| Race condition in parallel | Data inconsistency | Mutex + atomic operations |

### 7.2 Performance Concerns

| Concern | Current | Optimized |
|---------|---------|-----------|
| Checkpoint frequency | Every iteration | Every N iterations (configurable) |
| DAG parsing | On every execution | Cache parsed graphs |
| Handler lookup | Map scan | Pre-registered handlers |
| Condition evaluation | Expression parser | Simple evaluator |

### 7.3 Security Considerations

| Risk | Mitigation |
|------|------------|
| Injection in conditions | Sanitize inputs, use safe evaluator |
| Privilege escalation | Tenant isolation, RBAC |
| Resource exhaustion | Rate limiting, timeouts |
| Data leakage | Audit logs, encryption |

### 7.4 Scalability Limits

| Limit | Current | Future |
|-------|---------|--------|
| Concurrent executions | 100 | 1000+ |
| DAG size | 50 nodes | 100+ nodes |
| Checkpoint size | 1MB | 10MB+ |
| Execution time | 1 hour | 24 hours+ |

---

## 8. Implementation Phases

### Phase 1: Core Infrastructure (Days 1-3)
- [ ] Implement SkillGraph parsing from SKILLS.md
- [ ] Create CockroachDB tables with migrations
- [ ] Build SkillExecution state machine
- [ ] Implement checkpoint/resume logic

### Phase 2: Execution Engine (Days 4-6)
- [ ] Build SkillExecutor with retry logic
- [ ] Implement condition evaluation
- [ ] Add handler registration system
- [ ] Create audit logging

### Phase 3: Integration (Days 7-9)
- [ ] Integrate with existing bchat agent service
- [ ] Add tool-calling support for skills
- [ ] Implement multi-tenant isolation
- [ ] Add API endpoints for skill management

### Phase 4: Testing & Polish (Days 10-12)
- [ ] Unit tests for all components
- [ ] Integration tests with CockroachDB
- [ ] Load testing for concurrent executions
- [ ] Documentation and examples

### Phase 5: Demo Preparation (Days 13-14)
- [ ] Create demo scenarios
- [ ] Prepare hackathon presentation
- [ ] Write documentation
- [ ] Final testing

---

## 9. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Execution success rate | 99.9% | CockroachDB logs |
| Checkpoint frequency | Every 5 iterations | Configuration |
| Resume success rate | 100% | Test scenarios |
| Response time | < 2s | API metrics |
| Concurrent executions | 100+ | Load testing |

---

## 10. Open Questions for Discussion

1. **Condition Evaluation**: Should we use a simple expression parser or integrate a full YAML/CEL evaluator?

2. **Checkpoint Frequency**: Should checkpointing be configurable per skill or global?

3. **Handler Timeout**: What's the default timeout for skill handlers? 30s? 60s?

4. **Error Notification**: Should we notify admins on every failure or only on critical failures?

5. **Skill Composition**: Should skills be able to call other skills (nested DAGs)?

6. **State Eviction**: How long should we retain execution history? 7 days? 30 days?

7. **Parallel Execution**: Should we support parallel skill execution within a DAG level?

---

**Please review this plan and let me know:**
1. What needs clarification?
2. What should be changed?
3. What's missing?
4. Are there any concerns about the approach?

Once we agree, we can proceed with implementation.
