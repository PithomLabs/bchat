# bchat Pipeline Engine — Design Plan

> **The pitch:** n8n makes you drag nodes. bchat lets you write intent in markdown and the agent executes it. Powered by CockroachDB's unbreakable memory.

**Status:** Draft for review
**Target:** CockroachDB × AWS Hackathon (deadline Aug 19, 2026)
**Source repo:** /home/chaschel/Documents/go/bchat
**Winning criteria hit:** Agentic Memory Design (CRDB everything) · Technical Implementation (Bedrock tools + CRDB vectors + Go tool interface) · Real-World Impact (MyBiz.Fit demo) · Production Readiness (sandbox + audit + outbox) · Creativity (markdown-as-pipeline, no canvas)

---

## Table of Contents

1. [Decisions locked](#1-decisions-locked)
2. [Current state assessment](#2-current-state-assessment)
3. [Architecture](#3-architecture)
4. [Data model (new CRDB tables)](#4-data-model)
5. [Declarative config additions](#5-declarative-config-additions)
6. [Agent loop runtime](#6-agent-loop-runtime)
7. [Tool interface + built-in tools](#7-tool-interface--built-in-tools)
8. [Sandbox + security](#8-sandbox--security)
9. [LLM client (Bedrock + OpenRouter)](#9-llm-client)
10. [Triggers](#10-triggers)
11. [Comparison to n8n](#11-comparison-to-n8n)
12. [Implementation plan (file-by-file)](#12-implementation-plan)
13. [Demo scripts](#13-demo-scripts)
14. [Hackathon compliance checklist](#14-hackathon-compliance-checklist)

---

## 1. Decisions locked

| Decision | Choice |
|---|---|
| Execution model | **Job-based triggered runs** — pipeline runs are discrete jobs with a state machine in CRDB |
| Pipeline definition | **Structured `@step` annotations in SCRIPT.MD** — deterministic step machine, not just prose |
| Chat relationship | **Separate** — `processChat` stays untouched; new `PipelineRunner` handles pipeline jobs |
| Tool extensibility | **Compiled-in Go** — `Tool` interface, registered at startup |
| AWS | **Amazon Bedrock** (LLM inference, native tool-calling) + **Amazon S3** (artifact/file storage) |
| LLM | **Bedrock primary + OpenRouter fallback** — shared `LLMClient` interface |
| CRDB | **Native VECTOR columns** (RAG) + **ccloud CLI** (ops) + CRDB for all agent state |
| Demo | **MyBiz.Fit landing-page builder** (chat-triggered) + **nightly lead-export to S3** (cron-triggered) |
| Build sequence | **This design doc first**, then build against it |

---

## 2. Current state assessment

### What bchat already has (we build ON TOP of this)

| Component | Location | Reusable? |
|---|---|---|
| Multi-tenant chat engine | `agent/service.go:2531` `processChat()` | ✅ Untouched — pipeline runner is separate |
| Config parser | `agent/parser.go` (KB/POLICY/SCRIPT annotations) | ✅ Extend with new `@step`, `@lifecycle` types |
| CRDB native vectors | `agent/vectordb_cockroach.go` | ✅ RAG for pipeline steps |
| CRDB as primary store | `MEMOS_DRIVER=cockroach`, `fly_cockroach.toml` | ✅ All new tables go here |
| Event outbox + webhook delivery | `agent/service.go:5422` `dispatchEvent()`, `integrations.go` | ✅ Pipeline emits completion events via this |
| Bridge CAS state machine | `store/bridge.go:205` `UpdateBridgeHandoffRoutingModeCAS` | ✅ Pattern to copy for pipeline state transitions |
| Observational Memory | `agent/observer.go` | ✅ Pipeline runs contribute to OM |
| Tenant isolation | `tenant_context.go`, RBAC | ✅ All pipeline APIs scoped to tenant |
| Store.Driver interface | `store/driver.go` | ✅ Add new pipeline CRUD methods |
| AWS S3 SDK | `go.mod` (`aws-sdk-go-v2/service/s3`) | ✅ Already vendored, used for LanceDB-S3 |
| ccloud CLI integration | deploy scripts, `Dockerfile.cockroach.fly` | ✅ For ops demos |
| supercronic | `Dockerfile.pg.fly:70-88` | ✅ Cron trigger already wired |

### What's missing (we build THIS)

1. **PipelineRunner** — job-based agent execution loop with tool-calling
2. **`Tool` interface + registry** — compiled-in Go tools
3. **`@step` parser** — structured pipeline definition in SCRIPT.MD
4. **`@lifecycle` parser** — start/stop signals in POLICY.MD
5. **Pipeline state tables** — `agent_pipeline_runs`, `agent_pipeline_steps`, `agent_tool_calls`, `agent_pipeline_schedules`
6. **Bedrock LLM client** — `aws-sdk-go-v2/service/bedrockruntime` (add to go.mod)
7. **Sandbox** — per-tenant tool whitelist, rate limits, max iterations
8. **Pipeline API endpoints** — trigger, status, cancel, list

---

## 3. Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        TRIGGERS                                  │
│  chat widget    │   cron (supercronic)   │   webhook   │   API    │
│  (via @intent   │   (handleTriggerCron   │   (/pipeline/   │  manual  │
│   .action map)  │    enhanced)            │    webhook)     │         │
└──────────┬─────────────────────────────────────────────────┘
           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    PIPELINE RUNNER                                │
│                    (new: pipeline/runner.go)                     │
│                                                                   │
│  ┌─ START signal ─────────────────────────────────────────────┐  │
│  │  load POLICY @identity → adopt persona                    │  │
│  │  load KB context → RAG retrieve (CRDB vectors)           │  │
│  │  load SCRIPT @step sequence → step cursor               │  │
│  │  load Observational Memory → inject as context          │  │
│  │  emit pipeline.started event (outbox)                   │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌─ AGENT LOOP (per step) ───────────────────────────────────┐ │
│  │  while step_cursor != done && iter < max_iter:           │ │
│  │    ┌─ Reason ──────────────────────────────────────┐     │ │
│  │    │ buildSystemPrompt(persona, KB, step, OM)     │     │ │
│  │    │ llm.Converse(messages, tools=step.tools)     │     │ │
│  │    └──────────────────────────────────────────────┘     │ │
│  │    if finish_reason == "stop":                          │ │
│  │      persist assistant text → step done → next step     │ │
│  │    elif finish_reason == "tool_use":                    │ │
│  │      ┌─ Act ──────────────────────────────────────┐     │ │
│  │      │ for each tool_call:                         │     │ │
│  │      │   sandbox.check(tenant, tool)               │     │ │
│  │      │   result = registry.execute(tool, input)    │     │ │
│  │      │   audit_log.append(tool_call, result)       │     │ │
│  │      └────────────────────────────────────────────┘     │ │
│  │      ┌─ Observe ────────────────────────────────────┐    │ │
│  │      │ append tool results as "tool" messages       │    │ │
│  │      │ → re-prompt (loop)                           │    │ │
│  │      └─────────────────────────────────────────────┘    │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                                   │
│  ┌─ STOP signal ─────────────────────────────────────────────┐ │
│  │  summarize session → Observational Memory                 │ │
│  │  emit pipeline.completed event (outbox)                   │ │
│  │  trigger on_complete hooks (webhook, S3 upload, etc.)     │ │
│  │  finalize run state in CRDB                               │ │
│  └─────────────────────────────────────────────────────────┘ │
└──────────────────┬──────────────────────────────────────────┘
                   ▼
┌─────────────────────────────────────────────────────────────────┐
│              MEMORY LAYER — CockroachDB (everything)              │
│                                                                   │
│  agent_pipeline_runs     │ agent_tool_calls      │ agent_events  │
│  agent_pipeline_steps    │ agent_observations    │ agent_sessions│
│  agent_embeddings (VECTOR)│ agent_transcripts    │ agent_tenants │
│                                                                   │
│  ccloud CLI: provision, backup, monitor, audit logs              │
└─────────────────────────────────────────────────────────────────┘
                   ▼
┌─────────────────────────────────────────────────────────────────┐
│             TOOLS (Go, compiled-in, registered at startup)        │
│                                                                   │
│  Built-in:                                                        │
│   crdb_query  │ crdb_vector_search │ http_fetch  │ s3_upload      │
│   s3_download │ create_ticket      │ write_memo  │ send_webhook   │
│                                                                   │
│  Custom: any Go func implementing Tool interface                  │
│  Sandboxed: per-tenant whitelist, rate limit, max iterations     │
└─────────────────────────────────────────────────────────────────┘
```

### Component map (Go packages)

All paths relative to bchat root.

```
server/router/api/v1/
├── agent/                  # existing — untouched except parser
│   ├── service.go          # processChat — 10-line hook added
│   ├── parser.go           # EXTEND: add @step, @lifecycle parsing
│   ├── vectordb_cockroach.go  # existing CRDB vectors — pipeline RAG
│   ├── observer.go         # existing — pipeline calls it
│   └── ...
├── pipeline/               # NEW — the engine
│   ├── runner.go           # PipelineRunner: the agent loop
│   ├── state.go            # run state machine (CAS, like Bridge)
│   ├── handlers.go         # HTTP endpoints (trigger, status, cancel)
│   ├── llmclient.go        # LLMClient interface
│   ├── llm_bedrock.go      # Bedrock Converse implementation
│   ├── llm_openrouter.go   # OpenRouter fallback (wraps existing)
│   ├── sandbox.go          # per-tenant tool whitelist + limits
│   └── tools/              # built-in tools
│       ├── registry.go     # ToolRegistry
│       ├── interface.go    # Tool interface + ToolResult type
│       ├── crdb_query.go   # crdb_query tool
│       ├── crdb_vector.go  # crdb_vector_search tool
│       ├── http_fetch.go   # http_fetch tool
│       ├── s3.go           # s3_upload, s3_download tools
│       ├── ticket.go       # create_ticket tool
│       ├── memo.go         # write_memo tool
│       └── webhook.go      # send_webhook tool
└── v1.go                   # EXTEND: register pipeline routes

store/
├── pipeline.go             # NEW — pipeline types + store interface
├── driver.go               # EXTEND: add pipeline CRUD methods
├── db/postgres/pipeline.go # NEW — CRDB/Postgres impl
└── migration/
    └── postgres/<ver>/NN__pipeline_runs.sql   # NEW migrations

cmd/tools/                  # NEW — custom tools live here
└── README.md               # how to add your own Go tool
```

---

## 4. Data model

Three new tables in CRDB (Postgres-compatible DDL). All tenant-scoped with `ON DELETE CASCADE`.

### 4.1 `agent_pipeline_runs`

The job record. One row per pipeline execution.

```sql
CREATE TABLE IF NOT EXISTS agent_pipeline_runs (
    id            STRING PRIMARY KEY,          -- UUID
    tenant_id     INT NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    session_id    STRING,                       -- links to agent_sessions (nullable for headless)
    trigger_type  STRING NOT NULL,              -- chat | cron | webhook | manual
    trigger_ref   STRING,                       -- e.g. "intent:book_appointment" or "cron:nightly_export"
    pipeline_name STRING NOT NULL,              -- which pipeline def to run
    definition    STRING NOT NULL,              -- JSON: the parsed @step pipeline (snapshot at trigger time)
    input         STRING,                       -- JSON: initial input/seed data
    status        STRING NOT NULL DEFAULT 'queued',  -- queued | running | completed | failed | cancelled
    step_cursor   INT NOT NULL DEFAULT 0,       -- current step index
    step_name     STRING,                        -- current step name
    iterations    INT NOT NULL DEFAULT 0,       -- total LLM iterations across all steps
    max_iterations INT NOT NULL DEFAULT 50,     -- safety bound
    started_at    TIMESTAMPTZ,
    stopped_at    TIMESTAMPTZ,
    error_message STRING,
    version       INT NOT NULL DEFAULT 1,        -- for CAS state transitions
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

State transitions (CAS-guarded, copy of Bridge pattern from `store/bridge.go:205`):
- `queued → running` (runner claims)
- `running → completed` (all steps done, stop signal fired)
- `running → failed` (error or max iterations)
- `running → cancelled` (API cancel)
- `running → running` (step transition — updates step_cursor, increments version)

### 4.2 `agent_pipeline_steps`

Per-step execution log. Append-only audit trail.

```sql
CREATE TABLE IF NOT EXISTS agent_pipeline_steps (
    id            STRING PRIMARY KEY,           -- UUID
    run_id        STRING NOT NULL REFERENCES agent_pipeline_runs(id) ON DELETE CASCADE,
    tenant_id     INT NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    step_index    INT NOT NULL,
    step_name     STRING NOT NULL,
    status        STRING NOT NULL DEFAULT 'pending', -- pending | running | completed | failed | skipped
    tools_allowed STRING,                        -- JSON array: ["crdb_query","http_fetch"]
    on_complete   STRING,                        -- JSON: hooks to fire when step completes
    iterations    INT NOT NULL DEFAULT 0,         -- LLM iterations within this step
    result_summary STRING,                       -- LLM-generated summary of step outcome
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    error_message STRING,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pipeline_steps_run ON agent_pipeline_steps(run_id);
```

### 4.3 `agent_tool_calls`

Every tool invocation, ever. This is the audit log that makes the system production-grade and demo-friendly ("here's every action the agent took").

```sql
CREATE TABLE IF NOT EXISTS agent_tool_calls (
    id           STRING PRIMARY KEY,            -- UUID
    run_id       STRING NOT NULL REFERENCES agent_pipeline_runs(id) ON DELETE CASCADE,
    tenant_id    INT NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    step_index   INT NOT NULL,
    tool_name    STRING NOT NULL,
    tool_input   STRING NOT NULL,                -- JSON: the arguments the LLM provided
    tool_output  STRING,                         -- JSON: the result
    status       STRING NOT NULL DEFAULT 'pending', -- pending | success | error | denied
    error_message STRING,
    duration_ms  INT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tool_calls_run ON agent_tool_calls(run_id);
CREATE INDEX IF NOT EXISTS idx_tool_calls_tenant ON agent_tool_calls(tenant_id);
```

### 4.4 `agent_pipeline_schedules`

Cron-triggered pipelines.

```sql
CREATE TABLE IF NOT EXISTS agent_pipeline_schedules (
    id            STRING PRIMARY KEY,
    tenant_id     INT NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    pipeline_name STRING NOT NULL,
    cron_expr     STRING NOT NULL,         -- e.g. "0 2 * * *"
    input         STRING,                  -- JSON: seed input for each run
    last_fired_at TIMESTAMPTZ,
    is_active     BOOL NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pipeline_schedules_tenant ON agent_pipeline_schedules(tenant_id);
```

---

## 5. Declarative config additions

### 5.1 New SCRIPT.MD annotations — `@step`

```markdown
## Step: Discover Requirements
<!-- @step: discover, tools: none, on_complete: none -->
- Ask what the business does, services, brand
- Collect contact info
- Confirm and move to next step

## Step: Build Landing Page
<!-- @step: build, tools: http_fetch,crdb_query,write_memo, on_complete: emit_page_built -->
- Fetch business data from the URL the customer provided
- Generate content sections based on the discovered requirements
- Write the tenant YAML and content markdown to the database
- Confirm the page was created

## Step: Confirm and Deploy
<!-- @step: deploy, tools: s3_upload,send_webhook, on_complete: emit_pipeline_completed -->
- Summarize what was built
- Upload a preview snapshot to S3
- Send webhook notification to the business owner
- Share the live URL
```

**Parsed into Go struct:**
```go
type PipelineStep struct {
    Index        int      `json:"index"`
    Name         string   `json:"name"`         // "discover", "build", "deploy" (the @step code)
    Header       string   `json:"header"`       // "Step: Discover Requirements"
    Instructions []string `json:"instructions"` // bullet points
    Tools        []string `json:"tools"`        // ["http_fetch", "crdb_query", "write_memo"]
    OnComplete   string   `json:"on_complete"`  // hook name: "emit_page_built"
    Required     bool     `json:"required"`     // default true
}
```

### 5.2 New POLICY.MD annotations — `@lifecycle`

```markdown
<!-- @lifecycle: on_start -->
- Adopt persona from @identity
- Load KB context for the active vertical
- Emit pipeline.started event
- Initialize Observational Memory context

<!-- @lifecycle: on_stop -->
- Summarize session to Observational Memory
- Emit pipeline.completed event
- Trigger any pending webhook integrations
- Upload final state snapshot to S3
```

**Parsed into:**
```go
type LifecycleHooks struct {
    OnStart []string `json:"on_start"`  // bullet instructions
    OnStop  []string `json:"on_stop"`
}
```

### 5.3 Intent-to-pipeline mapping (POLICY.MD)

The existing `@intent` annotation already has an `action` field (`agent/parser.go:441`, default `"standard_flow"` at `:462`). We extend its semantics: if `action` starts with `pipeline:`, it triggers that pipeline.

```markdown
<!-- @intent: build_landing_page, category: conversion, urgency: normal, action: pipeline:landing_page_builder, confidence_threshold: 0.7 -->
## Build Landing Page
Customer wants to create a landing page for their business.
```

When `processChat` detects this intent (existing `classifyIntent` at `agent/service.go:2582`), it:
1. Finishes the normal chat response (untouched)
2. Enqueues a pipeline run via `PipelineRunner.Enqueue(triggerType="chat", triggerRef="intent:build_landing_page")`
3. Returns the chat response with a `pipeline_run_id` in metadata

**Zero changes to processChat's core flow** — just a post-dispatch hook.

---

## 6. Agent loop runtime

### 6.1 PipelineRunner

```go
// server/router/api/v1/pipeline/runner.go

type PipelineRunner struct {
    store        PipelineStore
    llm          LLMClient            // Bedrock primary, OpenRouter fallback
    vectorDB     agent.VectorDB       // existing — CRDB vectors for RAG
    toolRegistry *ToolRegistry
    sandbox      *Sandbox
    observer     *agent.Observer      // existing OM
    eventBus     EventDispatcher      // existing dispatchEvent
    configCache  *agent.ConfigCache   // existing LoadConfig
}

// Enqueue creates a pipeline run in 'queued' state and starts async execution.
func (r *PipelineRunner) Enqueue(ctx context.Context, req EnqueueRequest) (runID string, err error)

// Run is the main agent loop. Called async by Enqueue.
func (r *PipelineRunner) Run(ctx context.Context, runID string) error

// Cancel transitions a run to 'cancelled' via CAS.
func (r *PipelineRunner) Cancel(ctx context.Context, runID string) error

// Status returns the current run state + step cursor + recent tool calls.
func (r *PipelineRunner) Status(ctx context.Context, runID string) (*RunStatus, error)
```

### 6.2 Run loop pseudocode

```
func Run(ctx, runID):
    run = store.GetPipelineRun(runID)
    config = agent.LoadConfig(run.tenant, audience)   // existing
    steps = parsePipelineSteps(config.Script)         // new parser
    hooks = parseLifecycleHooks(config.Policy)        // new parser

    # START signal
    fireOnStart(hooks, config, run)
    store.UpdateRun(runID, status="running", started_at=now)
    emit("pipeline.started")

    for step := range steps:
        store.UpdateRun(runID, step_cursor=step.Index, step_name=step.Name, version++)

        # Agent loop within step
        messages = [systemPrompt(persona, KB, step, OM)]
        messages += runHistory(step)   # prior steps' outputs as context

        for iter < maxIterationsPerStep:
            resp = llm.Converse(ctx, ConverseRequest{
                Messages: messages,
                Tools:     registry.GetSchemas(step.Tools),
                ToolChoice: "auto",
            })

            if resp.FinishReason == "stop":
                store.AppendStepResult(step, resp.Text)
                fireOnComplete(step.OnComplete)
                break

            if resp.FinishReason == "tool_use":
                for toolCall := range resp.ToolCalls:
                    if !sandbox.Allowed(tenant, toolCall.Name):
                        audit.Append(status="denied")
                        messages += toolResult(toolCall, "denied by sandbox")
                        continue

                    result = registry.Execute(ctx, toolCall, tenant)
                    audit.Append(toolCall, result)   # to agent_tool_calls
                    messages += toolResult(toolCall, result)
                iter++
                continue

        # Step done — summarize to OM
        observer.RecordStepCompletion(step, resp.Text)

    # STOP signal
    observer.SummarizeSession(run)
    emit("pipeline.completed")
    fireOnStop(hooks, config, run)
    store.UpdateRun(runID, status="completed", stopped_at=now)
```

### 6.3 System prompt for pipeline steps

The pipeline uses a dedicated system prompt builder (separate from `buildSystemPrompt` in `agent/service.go:3113`):

```
You are {persona}.

## KNOWLEDGE BASE
{RAG-retrieved KB sections for this step's context}

## OBSERVATIONAL MEMORY
{compressed facts from prior interactions}

## CURRENT PIPELINE STEP: {step.Name}
{step.Instructions}

## AVAILABLE TOOLS
{JSON schemas of step.Tools}

## RULES
{policy rules from @rule annotations}

Execute this step. Use tools when needed. When the step's goal is achieved, respond with your summary and no tool calls.
```

---

## 7. Tool interface + built-in tools

### 7.1 The Tool interface

```go
// server/router/api/v1/pipeline/tools/interface.go

type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any  // JSON schema for Bedrock/OpenRouter
    Execute(ctx context.Context, tenant int32, input json.RawMessage) (ToolResult, error)
}

type ToolResult struct {
    Content  string          // text result fed back to the LLM
    Artifact string          // optional: S3 URI, DB row ID, etc.
    IsError  bool
}

type ToolRegistry struct {
    tools map[string]Tool
    mu    sync.RWMutex
}

func (r *ToolRegistry) Register(t Tool)                    // at startup
func (r *ToolRegistry) Get(name string) (Tool, bool)
func (r *ToolRegistry) GetSchemas(names []string) []ToolSchema
func (r *ToolRegistry) Execute(ctx, call ToolCall, tenant int32) (ToolResult, error)
```

### 7.2 Built-in tools

| Tool | Description | AWS/CRDB? |
|---|---|---|
| `crdb_query` | Execute parameterized SQL against the tenant's CRDB database (read-only by default, write with explicit grant) | CRDB |
| `crdb_vector_search` | Semantic search over the tenant's KB embeddings (wraps `vectordb_cockroach.go`) | CRDB vectors |
| `http_fetch` | Fetch a URL (SSRF-safe, reuses `validateAndResolveWebhookURL` pattern from `integrations.go:28`) | — |
| `s3_upload` | Upload a file/artifact to S3 (uses existing `aws-sdk-go-v2/service/s3`) | AWS S3 |
| `s3_download` | Download a file from S3 | AWS S3 |
| `create_ticket` | Create a support ticket in bchat's ticket system | CRDB |
| `write_memo` | Write a memo/note to bchat's note store | CRDB |
| `send_webhook` | Dispatch a webhook via the existing outbox (`dispatchEvent`) | — |

### 7.3 Adding a custom tool

```go
// cmd/tools/my_tool.go
package mytools

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/usememos/memos/server/router/api/v1/pipeline/tools"
)

type PriceChecker struct{}

func (p *PriceChecker) Name() string { return "check_price" }
func (p *PriceChecker) Description() string { return "Check competitor pricing for a service" }
func (p *PriceChecker) Parameters() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "service":  map[string]any{"type": "string", "description": "Service name"},
            "zip_code": map[string]any{"type": "string", "description": "Location"},
        },
        "required": []string{"service"},
    }
}
func (p *PriceChecker) Execute(ctx context.Context, tenant int32, input json.RawMessage) (tools.ToolResult, error) {
    var args struct{ Service, ZipCode string }
    json.Unmarshal(input, &args)
    // ... your logic ...
    return tools.ToolResult{Content: fmt.Sprintf("Avg price for %s: $%v", args.Service, price)}, nil
}
```

Register at startup in `bin/memos/main.go`:
```go
pipelineRunner.ToolRegistry().Register(&mytools.PriceChecker{})
```

---

## 8. Sandbox + security

### 8.1 Per-tenant tool whitelist

```sql
-- new column on agent_tenants, or part of ProcessingOptions JSON
pipeline_tools_allowed STRING  -- JSON: ["crdb_query","http_fetch","s3_upload"]
```

If a tenant hasn't configured a whitelist, only `http_fetch` and `crdb_vector_search` are available (safe defaults).

### 8.2 Rate limits + bounds

| Bound | Default | Env var |
|---|---|---|
| Max iterations per step | 20 | `PIPELINE_MAX_ITERATIONS_PER_STEP` |
| Max iterations per run | 50 | `PIPELINE_MAX_ITERATIONS` |
| Max concurrent runs per tenant | 3 | `PIPELINE_MAX_CONCURRENT_RUNS` |
| Tool execution timeout | 30s | `PIPELINE_TOOL_TIMEOUT` |
| LLM timeout | 60s | `PIPELINE_LLM_TIMEOUT` |
| Max tool input size | 16KB | `PIPELINE_MAX_TOOL_INPUT` |

### 8.3 CRDB query safety

The `crdb_query` tool:
- **Default: read-only** (`SELECT`/`WITH ... SELECT` only). Parse the SQL, reject anything that doesn't start with `SELECT` or `WITH`.
- **Write mode**: requires `pipeline:db_write` permission on the tenant RBAC. Writes use a separate CRDB user with limited grants (the ccloud service account pattern).
- **Parameterized only** — no string interpolation. Inputs come as JSON, we bind as `$1, $2, ...`.

### 8.4 Network egress

`http_fetch` reuses the SSRF protection from `integrations.go:validateAndResolveWebhookURL`:
- Reject private IPs (RFC 1918)
- Reject link-local (169.254.0.0/16) — blocks AWS metadata endpoint
- Reject loopback
- Redirect re-validation

### 8.5 Tenant isolation

All pipeline store methods take `tenantID *int32` and apply `ApplyTenantFilter` — same pattern as existing agent store methods. The pipeline runner extracts tenant from the run record, never from user input.

---

## 9. LLM client

### 9.1 Interface

```go
// server/router/api/v1/pipeline/llmclient.go

type LLMClient interface {
    Converse(ctx context.Context, req ConverseRequest) (*ConverseResponse, error)
}

type ConverseRequest struct {
    SystemPrompt string
    Messages     []Message
    Tools        []ToolSchema
    ToolChoice   string  // "auto" | "any" | "none"
    MaxTokens    int
    Temperature  float64
}

type ConverseResponse struct {
    Text         string
    ToolCalls    []ToolCall
    FinishReason string  // "stop" | "tool_use"
    Usage        TokenUsage
}
```

### 9.2 Bedrock implementation

```go
// server/router/api/v1/pipeline/llm_bedrock.go

type BedrockClient struct {
    client    *bedrockruntime.Client
    modelID   string  // e.g. "anthropic.claude-3-5-sonnet-20241022-v2:0"
}

func (b *BedrockClient) Converse(ctx, req) (*ConverseResponse, error) {
    // Use Bedrock Converse API with toolConfig
    // Map our ConverseRequest to bedrockruntime.ConverseInput
    // Parse response: output.message.content → text or tool_use
}
```

**go.mod addition:**
```
go get github.com/aws/aws-sdk-go-v2/service/bedrockruntime
```

### 9.3 OpenRouter fallback

Wraps the existing `go-openrouter` client at `agent/service.go:3011`. Uses OpenRouter's tool-calling support (the SDK already has `Tools`, `ToolChoice`, `ToolCalls` fields — confirmed in `chat.go:181-542`). Activates if Bedrock returns an error, or if `LLM_PROVIDER=openrouter` is set.

### 9.4 Provider selection

```bash
# .env
LLM_PROVIDER=bedrock          # bedrock | openrouter
BEDROCK_MODEL_ID=anthropic.claude-3-5-sonnet-20241022-v2:0
BEDROCK_REGION=us-east-1
# Fallback: if Bedrock fails, OpenRouter is used (always configured as secondary)
# AWS credentials via standard AWS SDK env (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
```

---

## 10. Triggers

### 10.1 Chat trigger (via processChat — minimal hook)

In `processChat` at `agent/service.go:2784-2798` (after response is final, before return):

```go
// NEW: post-response pipeline trigger (does not modify the response)
if decision.Action != "" && strings.HasPrefix(decision.Action, "pipeline:") {
    pipelineName := strings.TrimPrefix(decision.Action, "pipeline:")
    runID, _ := s.pipelineRunner.Enqueue(ctx, pipeline.EnqueueRequest{
        TenantID:    config.TenantID,
        TriggerType:  "chat",
        TriggerRef:   fmt.Sprintf("intent:%s", classification.PrimaryIntent),
        SessionID:   session.ID,
        Pipeline:     pipelineName,
    })
    response.Metadata.PipelineRunID = runID  // surfaced to widget
}
```

**This is the ONLY change to processChat** — a 10-line post-dispatch hook. The chat response is already final; the pipeline runs async.

### 10.2 Cron trigger (enhance existing HandleTriggerCron)

The existing `HandleTriggerCron` (`agent/integrations.go:197`) currently only polls `agent_events`. We add: it also enqueues scheduled pipelines.

```go
// In HandleTriggerCron, after processEventPoller:
s.service.processScheduledPipelines(ctx)  // new
```

```go
func (s *Service) processScheduledPipelines(ctx context.Context) {
    // Query agent_pipeline_schedules table (cron expressions per tenant)
    // For each due schedule: pipelineRunner.Enqueue(triggerType="cron", ...)
}
```

### 10.3 Webhook trigger

New endpoint: `POST /api/v1/pipeline/:slug/trigger`
- Auth: pipeline-specific token (HMAC, like `X-Cron-Token`)
- Body: `{"pipeline": "landing_page_builder", "input": {...}}`
- Enqueues a run with `triggerType="webhook"`

### 10.4 Manual API trigger

New endpoint: `POST /api/v1/agent/:slug/pipeline/run`
- Auth: `tenant:admin` permission (existing RBAC)
- Body: `{"pipeline": "landing_page_builder", "input": {...}}`
- Returns: `{"run_id": "...", "status": "queued"}`

---

## 11. Comparison to n8n

| Dimension | n8n | bchat Pipeline Engine |
|---|---|---|
| **Authoring** | Drag nodes on a canvas | Write intent in markdown (`@step` annotations) |
| **Logic** | Explicit node connections | LLM interprets instructions + uses tools |
| **Memory** | No memory — each run is stateless | CockroachDB: sessions, observations, vectors, tool call audit |
| **Multi-tenancy** | Single workspace | Native multi-tenant isolation |
| **Custom nodes** | JavaScript in a code node | Go tools — compiled-in, sandboxed |
| **Triggers** | Webhook, cron, manual | Webhook, cron, chat, manual |
| **Determinism** | Fully deterministic | Step machine is deterministic; within step, the agent is autonomous |
| **Setup time** | Configure each node | Write 3 markdown files + optionally add Go tools |
| **Failure handling** | Error workflow per node | Stop signal + OM summary + audit log + retry via outbox |
| **Memory layer** | SQLite/Postgres (thin) | CockroachDB (distributed, always-on, native vectors) |
| **Cost** | $49/mo + your infra | Open source + CRDB + Bedrock |

**The n8n-killer narrative:** n8n requires you to know the exact sequence of 47 nodes to automate "build a landing page for this customer." bchat requires you to write "Build a landing page" as a step in SCRIPT.MD with 3 bullet points of instructions and a tool whitelist. The agent figures out the rest. **You write intent, not nodes.**

---

## 12. Implementation plan

Sequenced for a ~20 day deadline. Each task is a unit of work with a clear "done" signal.

### Phase 0 — Foundation (days 1-2)

| # | Task | Files | Done signal |
|---|---|---|---|
| 0.1 | Add `aws-sdk-go-v2/service/bedrockruntime` to go.mod | `go.mod`, `go.sum` | `go mod tidy` succeeds |
| 0.2 | Create pipeline package skeleton | `server/router/api/v1/pipeline/*.go` (empty files with types) | `go build ./...` passes |
| 0.3 | Write pipeline store types + interface | `store/pipeline.go` | types compile |
| 0.4 | Write CRDB migration (4 new tables) | `store/migration/postgres/<ver>/01__pipeline_runs.sql` ... | migration runs on local CRDB |
| 0.5 | Implement pipeline store methods (Postgres/CRDB) | `store/db/postgres/pipeline.go` | unit tests pass |

### Phase 1 — Core engine (days 3-6)

| # | Task | Files | Done signal |
|---|---|---|---|
| 1.1 | Implement `LLMClient` interface + Bedrock impl | `pipeline/llmclient.go`, `pipeline/llm_bedrock.go` | Bedrock `Converse` returns text + tool calls |
| 1.2 | Implement OpenRouter fallback | `pipeline/llm_openrouter.go` | OpenRouter tool-calling works |
| 1.3 | Implement `Tool` interface + registry | `pipeline/tools/interface.go`, `pipeline/tools/registry.go` | Register + Execute works |
| 1.4 | Implement `PipelineRunner.Run` (the loop) | `pipeline/runner.go` | E2E: mock LLM + mock tool → run completes |
| 1.5 | Implement state machine (CAS transitions) | `pipeline/state.go` | Concurrent claim test passes |
| 1.6 | Implement sandbox | `pipeline/sandbox.go` | denied tool call returns "denied by sandbox" |

### Phase 2 — Parser + config (days 7-8)

| # | Task | Files | Done signal |
|---|---|---|---|
| 2.1 | Extend parser with `@step` annotation | `agent/parser.go` | MyBiz.Fit SCRIPT.MD parses to steps |
| 2.2 | Extend parser with `@lifecycle` annotation | `agent/parser.go` | POLICY.MD parses lifecycle hooks |
| 2.3 | Extend `@intent` action to support `pipeline:` prefix | `agent/parser.go` | intent.action="pipeline:landing_page_builder" triggers |

### Phase 3 — Built-in tools (days 9-11)

| # | Task | Files | Done signal |
|---|---|---|---|
| 3.1 | `crdb_query` tool (read-only mode) | `pipeline/tools/crdb_query.go` | SELECT works, INSERT blocked |
| 3.2 | `crdb_vector_search` tool | `pipeline/tools/crdb_vector.go` | semantic search returns KB chunks |
| 3.3 | `http_fetch` tool (SSRF-safe) | `pipeline/tools/http_fetch.go` | fetches public URL, blocks metadata IP |
| 3.4 | `s3_upload` + `s3_download` tools | `pipeline/tools/s3.go` | round-trip a file to S3 |
| 3.5 | `create_ticket`, `write_memo`, `send_webhook` tools | `pipeline/tools/ticket.go`, `memo.go`, `webhook.go` | each works against store layer |

### Phase 4 — API + triggers (days 12-14)

| # | Task | Files | Done signal |
|---|---|---|---|
| 4.1 | Pipeline HTTP handlers | `pipeline/handlers.go` | trigger/status/cancel/list endpoints |
| 4.2 | Register routes in v1.go | `server/router/api/v1/v1.go` | routes reachable |
| 4.3 | Chat trigger hook in processChat | `agent/service.go` (10-line hook) | intent triggers pipeline |
| 4.4 | Cron trigger via HandleTriggerCron | `agent/integrations.go` | supercronic triggers pipeline |
| 4.5 | Webhook trigger endpoint | `pipeline/handlers.go` | HMAC auth works |

### Phase 5 — Demo 1: MyBiz.Fit (days 15-16)

| # | Task | Files | Done signal |
|---|---|---|---|
| 5.1 | Write MyBiz.Fit pipeline SCRIPT.MD with @step | tenant config | 3 steps: discover → build → deploy |
| 5.2 | Write MyBiz.Fit POLICY.MD with @lifecycle | tenant config | start/stop hooks |
| 5.3 | Run the demo: chat with agent → agent builds landing page | manual | agent executes all steps, landing page created |
| 5.4 | Record 3-min demo video | — | shows chat → pipeline → tool calls → result |

### Phase 6 — Demo 2: cron lead-export (days 17-18)

| # | Task | Files | Done signal |
|---|---|---|---|
| 6.1 | Write lead-export pipeline SCRIPT.MD | tenant config | 2 steps: query leads → export to S3 |
| 6.2 | Set up cron schedule | `agent_pipeline_schedules` row | fires nightly |
| 6.3 | Run the demo: cron triggers → S3 upload | manual | CSV appears in S3 |

### Phase 7 — Polish + submit (days 19-20)

| # | Task | Done signal |
|---|---|---|
| 7.1 | Write README with architecture diagram | README.md updated |
| 7.2 | Deploy to Fly.io + CRDB Cloud | live demo URL works |
| 7.3 | Record final 3-min video | uploaded to YouTube |
| 7.4 | Submit on Devpost | submitted |

---

## 13. Demo scripts

### Demo 1: MyBiz.Fit landing-page builder (chat-triggered)

**Setup:** A business owner opens the bchat widget on mybiz.fit.

```
Owner: "I want a landing page for my dental practice"
Agent:  [classifies intent: build_landing_page → action: pipeline:landing_page_builder]
       [Chat response:] "I'd love to help! Let me gather some info and build your page."

       ── PIPELINE STARTS (async) ──

       Step 1: Discover Requirements
       [Agent asks about services, hours, contact → owner replies in chat]
       [Agent uses crdb_vector_search to find dental templates in KB]

       Step 2: Build Landing Page
       [Agent uses crdb_query to write tenant config to DB]
       [Agent uses write_memo to create content sections]
       [Agent uses s3_upload to store a preview snapshot]

       Step 3: Confirm and Deploy
       [Agent uses send_webhook to notify owner]
       [Agent responds in chat: "Your page is live at mybiz.fit/sunset-dental"]
       ── PIPELINE COMPLETES ──

Owner: sees: "Your landing page is ready!"
       [pipeline_run_id in response metadata → can track in admin UI]
```

**What the video shows:**
1. Chat conversation (natural)
2. Admin UI: pipeline run status, step-by-step
3. Admin UI: tool call audit log (every action the agent took)
4. The landing page is live

### Demo 2: Nightly lead-export (cron-triggered)

**Setup:** supercronic fires at 2 AM UTC.

```
[Cron fires HandleTriggerCron]
[processScheduledPipelines finds due schedule]
[PipelineRunner.Enqueue(trigger=cron, pipeline=lead_export)]

    Step 1: Query Leads
    [Agent uses crdb_query: SELECT * FROM agent_leads WHERE created_at > now() - interval '1 day']
    [100 leads found]

    Step 2: Export to S3
    [Agent uses s3_upload: writes leads_2026-08-19.csv to bucket]
    [send_webhook: notifies a webhook endpoint]

    ── PIPELINE COMPLETES ──
```

**What the video shows:**
1. supercronic config
2. Pipeline runs headless (no chat)
3. CSV appears in S3 bucket
4. Tool call audit log

---

## 14. Hackathon compliance checklist

- [ ] **Public open source repo** (MIT or Apache 2.0 license)
- [ ] **Live demo URL** (Fly.io + CRDB Cloud)
- [ ] **3-min video** (YouTube or Vimeo)
- [ ] **CRDB tools used (≥2):**
  - [ ] CockroachDB Distributed Vector Indexing (`vectordb_cockroach.go` — RAG for pipeline steps)
  - [ ] ccloud CLI (ops: provision, backup, monitor in deploy scripts)
  - [ ] (optional) CockroachDB Cloud MCP Server — for the admin UI to query pipeline state
- [ ] **AWS services used (≥1):**
  - [ ] Amazon Bedrock (LLM inference via Converse API + tool-calling)
  - [ ] Amazon S3 (pipeline artifact storage)
- [ ] **Architecture diagram** (this document serves as it)
- [ ] **Identify which tools used** (in Devpost submission text)
- [ ] **License file** detectable in repo About section

---

## Appendix A — Environment variables (new)

```bash
# Pipeline engine
PIPELINE_ENABLED=true
PIPELINE_MAX_ITERATIONS=50
PIPELINE_MAX_ITERATIONS_PER_STEP=20
PIPELINE_MAX_CONCURRENT_RUNS=3
PIPELINE_TOOL_TIMEOUT=30s
PIPELINE_LLM_TIMEOUT=60s
PIPELINE_MAX_TOOL_INPUT=16384

# LLM provider (pipeline)
LLM_PROVIDER=bedrock                    # bedrock | openrouter
BEDROCK_MODEL_ID=anthropic.claude-3-5-sonnet-20241022-v2:0
BEDROCK_REGION=us-east-1
# AWS credentials via standard AWS SDK env (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)

# S3 (pipeline artifacts — can reuse existing LanceDB S3 config)
PIPELINE_S3_BUCKET=bchat-pipeline-artifacts
PIPELINE_S3_PREFIX=pipeline/
# AWS_ENDPOINT_URL_S3=... (reuse existing)
```

## Appendix B — Existing code that is NOT touched

| File | Reason |
|---|---|
| `agent/service.go:2531` processChat core | Only a 10-line post-response hook added |
| `agent/handlers.go` chat handlers | Untouched |
| `agent/vectordb_lance.go` | Untouched (pipeline uses vectordb interface) |
| `agent/observer.go` | Untouched (pipeline calls it, doesn't modify it) |
| `store/bridge.go` | Untouched (pattern copied, not shared) |
| `web/src/` React admin | Minimal: add pipeline status panel (optional) |
| `widget/` chat widget | Untouched (just surfaces run_id in metadata) |