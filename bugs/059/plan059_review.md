# Plan 059 Review — Recommendations & Rationale

**Context:** These recommendations address the 6 required rework items and 4 open questions from the adversarial review of [plan.md](file:///home/chaschel/Documents/go/bchat/bugs/059/plan.md). Each recommendation is grounded in the current bchat codebase and gives a concrete "do this" with "because".

---

## Rework Recommendations

### R1. Reconcile SKILLS.md with the existing 3-file model

**Recommendation: Extend SCRIPT.md — do not add a 4th file.**

SCRIPT.md already defines "what the agent does in what order." Skills are a structured version of the same concept. Extend the existing [parser.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go) annotation system to support skill annotations inside SCRIPT.md:

```markdown
<!-- @skill: classify_intent -->
<!-- @handler: builtin:classify_intent -->
<!-- @depends_on: none -->
<!-- @timeout: 30s -->
## Classify Intent
Determine what the customer wants from their message...
```

**Why:**

1. **No upload flow changes.** The existing `POST /api/v1/agent/:slug/files` already handles `file_type=script`. No new enum value, no new handler, no new frontend form.

2. **Parser is already extensible.** The [extractAnnotationBlocks()](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go#L99) function extracts `<!-- @type: params -->` generically. Adding `@skill`, `@handler`, `@depends_on` is a few lines of extraction code on top of the existing pattern — not a new parser.

3. **Single source of truth.** SCRIPT.md already feeds into [buildSystemPrompt() SECTION 5](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L3279). If skills live here, the prompt builder naturally picks them up. A 4th file creates a "which file wins?" conflict — does the SCRIPT.md conversation flow override the SKILLS.md DAG, or vice versa?

4. **Backward compatible.** A SCRIPT.md with no `@skill` annotations parses exactly as it does today (sections + questions). Tenants that don't need durable execution are unaffected.

5. **Matches the real mental model.** When a tenant defines "Step 1: Classify intent → Step 2: Search KB → Step 3: Create ticket," they're defining a *script with structure*, not a separate artifact. The DAG is implicit in the script's flow.

**What this means for the plan:** Replace every reference to `SKILLS.md` with "skill-annotated sections within SCRIPT.md." The `agent_skill_definitions` table stores the parsed graph from a SCRIPT.md that contains skill annotations, keyed by `tenant_id`.

---

### R2. Define the handler execution model

**Recommendation: Hybrid model — LLM-as-executor for reasoning steps, built-in handlers for deterministic side-effects.**

Concretely:

| Handler Prefix | Execution Model | Example |
|----------------|-----------------|---------|
| `builtin:classify_intent` | Go function in a curated registry | Intent classification via LLM with structured output |
| `builtin:search_kb` | Go function (RAG pipeline call) | Existing `vectorDB.Search()` |
| `builtin:create_ticket` | Go function (DB write) | Existing `store.CreateTicket()` |
| `builtin:webhook` | Go function (HTTP POST to configured URL) | Existing `AgentEvent` → integration delivery |
| `llm:respond` | LLM call with skill instructions as system prompt | Free-form response generation |
| `llm:analyze` | LLM call with structured output schema | Sentiment analysis, summarization |

**Why:**

1. **"No code changes per tenant" is preserved.** The `builtin:` handlers are a fixed, curated set compiled into the binary — they're like SQL functions, not user code. Tenants compose them via SCRIPT.md annotations, but never write Go. The `llm:` handlers let the LLM do anything the skill instructions describe, which is inherently tenant-configurable.

2. **The LLM is already the execution engine.** bchat's entire chat pipeline is "build system prompt → call LLM → verify response." Making the LLM also the skill executor for reasoning-heavy steps is natural. The DAG orchestrates *when* the LLM is called and *what instructions* it receives, but the LLM is still the brain.

3. **Side-effects need deterministic handlers.** You don't want the LLM to "decide" to create a ticket by generating SQL. Deterministic handlers (create_ticket, webhook, search_kb) are compiled Go functions with proper error handling, tenant isolation, and audit logging. This is the security boundary.

4. **It's how tool-calling already works.** OpenRouter supports tool/function calling. The plan's skill graph becomes a list of tools the LLM can invoke. The existing [service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go) already talks to OpenRouter — adding tool definitions to the request is incremental.

**What this means for the plan:** The `SkillRegistry` contains ~10-15 built-in handlers. The `@handler` annotation in SCRIPT.md references these by name. If a handler name is unrecognized, the skill is treated as an `llm:` skill and the section's markdown content becomes the LLM's instructions for that step.

---

### R3. Define SQLite schema equivalents

**Recommendation: Use the type mapping from [docs/TYPE_MAPPING.md](file:///home/chaschel/Documents/go/bchat/docs) and follow the existing migration pattern.**

Concrete translations:

| CockroachDB | SQLite Equivalent |
|-------------|-------------------|
| `UUID PRIMARY KEY DEFAULT gen_random_uuid()` | `TEXT PRIMARY KEY` (generate UUID in Go, pass as string) |
| `JSONB NOT NULL` | `TEXT NOT NULL DEFAULT '{}'` |
| `TIMESTAMPTZ DEFAULT NOW()` | `INTEGER DEFAULT 0` (Unix epoch, matches existing pattern) |
| `INT REFERENCES agent_tenants(id)` | `INTEGER DEFAULT NULL` (SQLite doesn't enforce FK by default) |
| Partial index `WHERE status = 'pending'` | Regular index (SQLite doesn't support partial indexes) |

**Why:**

1. **The existing codebase already does this everywhere.** Every table in bchat has both a SQLite and a PostgreSQL/CockroachDB migration. UUID is `TEXT` in SQLite across the board. Timestamps are `INTEGER` (Unix epoch). JSON is `TEXT`. This is established convention, not a design choice.

2. **`task validate:parity` enforces it.** The CI/CD pipeline runs cross-driver schema parity checks. Shipping CockroachDB-only tables will fail the build.

3. **Dev workflow depends on it.** `task build` uses SQLite by default. If the skill execution tables don't exist in SQLite, no developer can test the feature locally without spinning up CockroachDB.

**What this means for the plan:** Add a `store/migration/sqlite/` section for every DDL statement. Use `task migrate:new NAME=add_skill_executions` to create the template, then write SQL for both drivers.

---

### R4. Add tenant scoping to executor

**Recommendation: Follow the existing `getTenantFromContext(c)` + `ApplyTenantFilter(c, find)` pattern exactly.**

Concretely, every executor method should look like:

```go
func (e *SkillExecutor) StartExecution(c echo.Context, graph *SkillGraph) (*SkillExecution, error) {
    tenantID := getTenantFromContext(c)
    if tenantID == nil {
        return nil, echo.NewHTTPError(http.StatusForbidden, "tenant context required")
    }

    // Verify the skill definition belongs to this tenant
    def, err := e.store.GetAgentSkillDefinition(ctx, &FindAgentSkillDefinition{
        TenantID:  tenantID,
        SkillName: &graph.Name,
    })
    if err != nil || def == nil {
        return nil, echo.NewHTTPError(http.StatusNotFound, "skill not found for tenant")
    }

    execution := &SkillExecution{
        TenantID:       *tenantID,
        ConversationID: session.ID,
        SkillGraph:     graph,
        Status:         "pending",
    }
    return e.store.CreateSkillExecution(ctx, execution)
}
```

**Why:**

1. **It's a security invariant.** The AGENTS.md states: "Every API request must be scoped to a single tenant." Skill executions are tenant-scoped data. Checkpoint data contains tenant-specific business information (customer intents, ticket IDs, KB search results). Cross-tenant leakage is a data breach.

2. **The pattern is established and well-documented.** See [handlers.go L62-97](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go#L62) for the canonical pattern. Don't invent a new one.

3. **Superuser bypass must be explicit.** If a superuser needs to view cross-tenant executions (e.g., admin dashboard), use the existing `!isSuperUser(user)` guard.

**What this means for the plan:** Every pseudocode example in the plan needs `tenantID` extraction and filtering. The `SkillExecutor` struct should hold a `tenantID int32` field set at construction time from the request context, not extracted per-method.

---

### R5. Define integration points with existing pipeline

**Recommendation: Skills inject as tool definitions in the LLM request and as a new prompt section between SECTION 5 (Conversation Flow) and SECTION 6 (Policies & Rules).**

Concretely:

**A. System prompt injection** — add a new section in [buildSystemPrompt()](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L3113):

```
=== SECTION 5: CONVERSATION FLOW ===    ← existing
=== SECTION 5.5: AVAILABLE SKILLS ===   ← NEW
  You have the following skills available. Use them when appropriate:
  - classify_intent: Classify the customer's intent from their message
  - search_kb: Search the knowledge base for relevant information
  - create_ticket: Create a support ticket
  When you want to use a skill, call it as a tool.
=== SECTION 6: POLICIES & RULES ===     ← existing
```

**B. Tool-calling integration** — when the LLM response contains a tool call, the executor:
1. Looks up the handler in the `SkillRegistry`
2. Executes it with tenant-scoped context
3. Returns the result as a tool response message
4. The LLM continues with the tool result in context

**C. Conversation history** — skill inputs/outputs are inserted as tool-call/tool-response messages in the session's message history, exactly like how OpenAI/Anthropic/Google tool-calling works.

**D. Verifier** — the [verifier.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/verifier.go) runs on the final LLM response (after all tool calls are resolved), not on intermediate skill outputs.

**Why:**

1. **Tool-calling is the industry standard.** OpenRouter already supports function/tool calling. The LLM sees tool definitions, decides when to call them, and the runtime executes the handler. This is exactly how Claude, GPT-4, and Gemini implement agentic workflows. bchat should use the same protocol.

2. **The LLM maintains persona compliance.** If the DAG executor bypasses the LLM, then POLICY.md rules (tone, brand voice, escalation thresholds) are not enforced during skill execution. By keeping the LLM in the loop, the agent's persona governs how skill results are communicated to the user.

3. **Observational Memory works automatically.** Tool-call/tool-response messages are part of conversation history. The [ObserverBuffer](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L198) will compress them like any other messages. No special OM integration needed.

4. **The DAG becomes advisory, not authoritative.** The LLM uses skills when they're appropriate, not mechanically. The DAG defines *available* skills and their dependencies (you can't call `create_ticket` before `classify_intent`), but the LLM decides *whether* to call them based on the conversation. This is more natural than a rigid pipeline.

**What this means for the plan:** The "Skill Execution Pipeline" (Parse → Resolve Deps → Execute → Checkpoint) becomes "Skill Availability Pipeline" — the DAG is parsed and validated at SCRIPT.md upload time, but execution is driven by the LLM's tool-calling decisions. Checkpointing happens after each tool call resolves.

---

### R6. Add startup recovery worker

**Recommendation: Add a recovery goroutine in [NewService()](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L89) that follows the existing pattern of startup background workers.**

Concretely:

```go
// In NewService(), after existing startup workers:
if skillExecutionEnabled {
    go func() {
        time.Sleep(10 * time.Second) // let other subsystems initialize
        ctx := context.Background()
        executions, err := s.ListPendingSkillExecutions(ctx)
        if err != nil {
            slog.Error("Failed to list pending skill executions", "error", err)
            return
        }
        for _, exec := range executions {
            // Mark 'running' nodes as 'pending' for retry
            exec.Status = "pending"
            if exec.CurrentNode != "" {
                exec.FailedNodes[exec.CurrentNode] = "process restart"
            }
            s.UpdateSkillExecution(ctx, exec)
            slog.Info("Recovered skill execution after restart",
                "execution_id", exec.ID,
                "tenant_id", exec.TenantID,
                "completed_nodes", len(exec.CompletedNodes))
        }
    }()
}
```

**Why:**

1. **The pattern already exists.** The codebase has three startup goroutines: ticket embedding cron ([service.go L206-217](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L206)), forced reindex ([L234-241](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L234)), and auto-bootstrap RAG ([L243-268](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L243)). Skill recovery is the same shape.

2. **Without this, "durable execution" is a lie.** The entire point of checkpointing to CockroachDB/SQLite is to survive process restarts. If nobody reads the checkpoints on startup, they're just audit logs — not durable state.

3. **Recovery must be conservative.** Don't auto-resume — mark `running` nodes as `pending` and let the next user message or a scheduled retry pick them up. This avoids the case where a restart triggers 100 parallel skill executions that overwhelm the LLM API.

4. **Add a startup delay.** The existing workers use 2-10 second delays to let the DB, embedding service, and vector DB initialize. Skill recovery should use 10 seconds (after RAG bootstrap) to avoid querying tables that haven't been migrated yet.

**What this means for the plan:** Add a "Startup Recovery" section that defines the recovery goroutine, its delay, and its conservative strategy. Don't auto-execute — just mark recovered executions as resumable and let the normal execution path handle them.

---

## Open Question Recommendations

### Q1. Token budget — will skills blow out the context window?

**Recommendation: Cap skill definitions at 1,500 tokens in the system prompt. Use progressive disclosure.**

Concretely:
- In `buildSystemPrompt()`, emit only skill **names and one-line descriptions** (like existing tool-calling APIs do)
- When the LLM calls a skill, load the full skill definition (input/output schemas, instructions, conditions) and inject it as a tool-response message
- This is exactly how [agentskills.io progressive disclosure](file:///home/chaschel/Documents/go/bchat/bugs/059/agent_script.md) works: "metadata at startup, full body on activation, resources on demand"

**Why:** The existing system prompt already consumes 4-8K tokens (identity + RAG + services + exclusions + script + policies + FAQs + OM). Adding 5 skill definitions with full schemas would add 2-3K tokens. With GPT-4o-mini's 128K window this is fine, but with smaller models or long conversations, it becomes the tipping point. Progressive disclosure keeps the baseline prompt lean.

**Budget allocation:**

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

### Q2. Determinism — who controls execution order, the DAG or the LLM?

**Recommendation: The DAG constrains; the LLM decides.**

Concretely:
- The DAG defines **dependency order** (can't call `create_ticket` before `classify_intent`) and **guard conditions** (only call `escalate` if priority is high)
- The LLM decides **whether** to call a skill at all, and **when** within the conversation
- The runtime **enforces** the DAG constraints: if the LLM tries to call `create_ticket` before `classify_intent` has completed, the tool call returns an error message: "Dependency not met: classify_intent must complete first"

**Why:** This is the architecture that works in production:

| Model | Example | Failure Mode |
|-------|---------|--------------|
| DAG-authoritative | n8n, Temporal | Rigid — can't handle "customer changes mind mid-flow" |
| LLM-authoritative | Raw tool-calling | Chaotic — LLM skips steps, calls things out of order |
| **DAG-constrained, LLM-driven** | **This recommendation** | **Flexible within guardrails** |

The agent can skip a skill if it's not needed (customer already provided their intent — no need to classify). But it can't jump ahead past dependencies. This matches how a human agent follows a script: the script is a guide, not a rigid sequence. You can skip "ask for contact info" if you already have it, but you can't close a ticket before creating it.

---

### Q3. Backward compatibility — do existing tenants break?

**Recommendation: Fully backward compatible. Skills are opt-in, activated by the presence of `@skill` annotations in SCRIPT.md.**

Concretely:
- If SCRIPT.md has no `@skill` annotations → parse as today (sections + questions → conversation flow summary)
- If SCRIPT.md has `@skill` annotations → parse as skill graph + add tool definitions to prompt
- Detection is simple: `if len(extractAnnotationBlocks(content, "skill")) > 0`

```go
func (p *Parser) ParseScript(content string) (*ParsedScript, error) {
    result := &ParsedScript{RawContent: content}
    
    // Check for skill annotations
    skillBlocks := extractAnnotationBlocksByType(content, "skill")
    if len(skillBlocks) > 0 {
        // New path: parse as skill-annotated script
        result.SkillGraph = p.buildSkillGraph(skillBlocks)
        result.Summary = buildSkillSummary(result.SkillGraph)
    } else {
        // Legacy path: parse as conversation flow
        result.Sections = p.parseSections(content)
        result.Summary = buildScriptSummary(result.Sections)
    }
    
    return result, nil
}
```

**Why:**

1. **Zero migration cost.** Existing tenants don't need to change anything. Their SCRIPT.md files parse identically.

2. **Gradual adoption.** Tenants can start with a simple SCRIPT.md, then add `@skill` annotations to individual sections when they want durable execution for those steps. You can even mix: some sections are conversation flow guidance, others are executable skills.

3. **No feature flags needed.** The presence of `@skill` annotations is the feature flag. No env var, no config toggle, no admin UI setting.

---

### Q4. Testing — how do you test skill DAGs without burning API credits?

**Recommendation: Three-layer test strategy using existing infrastructure.**

**Layer 1: Unit tests (no LLM, no DB)**

Test the DAG parser, dependency resolution, condition evaluation, and state machine transitions. These are pure Go functions with no external dependencies.

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
```

**Layer 2: Integration tests with mock LLM (no real API calls)**

The codebase already has `OPENROUTER_API_BASE_URL` for routing to a mock server ([service.go L60-61](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L60)). Use the same pattern:

```go
// Start httptest.Server that returns canned tool-call responses
mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Return a tool_calls response that invokes classify_intent
    json.NewEncoder(w).Encode(mockToolCallResponse)
}))
os.Setenv("OPENROUTER_API_BASE_URL", mockServer.URL)
```

**Layer 3: Simulation framework (existing)**

bchat already has a [simulation.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/simulation.go) framework that runs scripted conversations and evaluates agent responses. Extend `HandleRunSimulation` to support skill-graph assertions:

```json
{
    "scenarios": [{
        "messages": ["I need emergency water extraction"],
        "expected_skills_called": ["classify_intent", "search_kb"],
        "expected_skills_not_called": ["escalate_to_human"],
        "max_skill_duration_ms": 5000
    }]
}
```

**Why:**

1. **Mock embedding already exists.** `EMBEDDING_PROVIDER=mock` is used for RAG testing without API calls. The same pattern (mock LLM server) works for skill testing.

2. **The simulation framework is the end-to-end test.** It already runs full conversations, scores responses, and persists transcripts. Adding skill-graph assertions is incremental — check which tools were called, in what order, with what inputs.

3. **Unit tests are the fast feedback loop.** DAG parsing, validation, and state machine logic should be 100% testable without any external dependencies. These run in `go test` in milliseconds.

---

## Summary Decision Matrix

| Item | Recommendation | Risk if Ignored |
|------|---------------|-----------------|
| R1 | Extend SCRIPT.md, no 4th file | Two competing config systems |
| R2 | Hybrid: builtin handlers + LLM executor | Either rigid pipelines or unsafe arbitrary execution |
| R3 | SQLite parity via existing type mapping | Feature broken for all SQLite/dev users |
| R4 | Tenant scoping on every executor method | Cross-tenant data breach |
| R5 | Tool-calling integration into LLM pipeline | Persona/policy bypass |
| R6 | Startup recovery goroutine (conservative) | "Durable execution" is marketing, not reality |
| Q1 | 1,500 token cap + progressive disclosure | Context window overflow on small models |
| Q2 | DAG constrains, LLM decides | Either too rigid or too chaotic |
| Q3 | Opt-in via `@skill` annotation presence | Existing tenants break |
| Q4 | 3-layer: unit + mock LLM + simulation | Untestable feature, can't iterate safely |
