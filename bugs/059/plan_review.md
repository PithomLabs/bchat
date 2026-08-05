# Adversarial Plan Review — bugs/059 Durable Execution Architecture

**Reviewer posture:** Senior Go architect, database expert, automation/RAG pipeline designer.  
**Reviewed artifact:** [plan.md](file:///home/chaschel/Documents/go/bchat/bugs/059/plan.md)  
**Codebase baseline:** bchat @ current HEAD — [service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go), [handlers.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go), [parser.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go), [store/agent.go](file:///home/chaschel/Documents/go/bchat/store/agent.go), [store/driver.go](file:///home/chaschel/Documents/go/bchat/store/driver.go), [agent_workflow.go](file:///home/chaschel/Documents/go/bchat/store/agent_workflow.go)

---

## Verdict: **REWORK** 🔴

The plan has the right instincts — durable execution, DAG-based skill graphs, checkpoint/resume, CockroachDB state — but it has **6 structural flaws** and **9 significant gaps** that would cause it to fail in implementation against the actual bchat codebase. The issues are fundamental enough that bolting the plan onto the codebase as-written would create an entirely separate runtime that doesn't integrate with the existing chat pipeline; it would be a greenfield system cosplaying as a bchat enhancement.

---

## 🔴 Structural Flaws (Blockers)

### F1. The plan introduces a 4th file (SKILLS.md) without reconciling with the existing 3-file model

The existing bchat architecture is **KB.md + POLICY.md + SCRIPT.md**, parsed by [parser.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go) into structured types (`ParsedKB`, `ParsedPolicy`, `ParsedScript`), stored in `agent_source_files` and `agent_tenant_scripts`, and injected into the system prompt via [buildSystemPrompt()](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L3113).

The plan introduces `SKILLS.md` as a 4th artifact but never explains:
- Does it **replace** SCRIPT.md? (SCRIPT.md already defines conversation flow — are they composable or competing?)
- Does it **extend** the upload API at `/api/v1/agent/:slug/files`? The current handler expects `file_type` ∈ {`kb`, `policy`, `script`}.
- How does `SKILLS.md` coexist with the existing `@skill` / `@handler` annotation concept in the plan vs the existing `@service`, `@faq`, `@intent` annotation system in [parser.go L92-97](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go#L92)?

> **Impact:** Without this reconciliation, you either break the existing upload flow or have two parallel configuration systems that don't know about each other.

### F2. Handler Registration assumes compiled Go code — violates "no code changes per tenant" principle

The plan's core execution mechanism is:

```go
registry.RegisterHandler("classifyIntentHandler", classifyIntentHandler)
```

This requires **compiled Go functions** for every skill step. But bchat's core design principle (stated in AGENTS.md) is:

> "Each tenant can define their own knowledge base, policies, and conversation scripts **without requiring code changes**."

If `SKILLS.md` references `@handler: createTicketHandler` and that handler must be a Go function compiled into the binary, then:
- Every new tenant skill requires a code deploy
- The handler registry is a static compile-time artifact, not a runtime-configurable one
- This is the opposite of "declarative markdown config"

> **What's needed:** The plan must define a **sandbox runtime** for skill execution — either:
> - LLM-driven tool calls (the existing pattern — the LLM *is* the execution engine, skills are tool descriptions)
> - A small embedded interpreter (Tengo, Expr, or Starlark) for non-LLM logic
> - Or explicitly scope built-in handlers to a curated set (classify, search_kb, create_ticket, etc.) and make them generic

### F3. The DAG executor is disconnected from the existing chat pipeline

The plan describes an execution engine that is essentially a standalone workflow runner:

```
User Message → Parse SKILLS.md → Build DAG → Create Execution Record → Execute nodes → Finalize
```

But the actual bchat chat pipeline is:

```
User Message → Classify intent → Policy decision → Build system prompt → LLM call → Verify → Respond
```

([service.go L3113](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L3113), [handlers.go L386](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go#L386))

The plan never explains **how the DAG executor integrates with the existing flow**:
- Does the LLM trigger skill execution via tool-calling? If so, where's the tool-calling protocol?
- Does the DAG bypass the LLM entirely? If so, how does the agent maintain its persona/policy compliance?
- When a skill produces output, does it become part of the conversation history? How?

### F4. SQLite parity is ignored — the plan is CockroachDB-only

The plan's schema uses CockroachDB-specific features:
- `UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `JSONB` columns
- `TIMESTAMPTZ`
- Partial indexes (`WHERE status = 'pending'`)

But bchat supports **SQLite as the default** database ([store/driver.go](file:///home/chaschel/Documents/go/bchat/store/driver.go)), and every feature requires **all three driver implementations** (SQLite, PostgreSQL, CockroachDB). The plan adds 3 new tables with no SQLite equivalents. Per the migration guide, `task validate:parity` will fail.

> **Impact:** The feature won't work for any developer running `task build` (SQLite default) or the existing PostgreSQL deployments.

### F5. No tenant isolation in the skill execution model

The plan's schema has `tenant_id` on `agent_skill_executions` and `agent_skill_definitions`, but the execution engine pseudocode never:
- Extracts tenant ID from context (violates the mandatory pattern in AGENTS.md)
- Applies tenant filtering on queries
- Validates that skill definitions belong to the executing tenant
- Prevents cross-tenant checkpoint data leakage

The [existing codebase](file:///home/chaschel/Documents/go/bchat/store/driver.go) enforces tenant isolation at every layer. The plan's `SkillExecutor` has zero tenant-scoping code.

### F6. The agentskills.io "compliance" is cosmetic

The plan claims agentskills.io compliance, but:
- agentskills.io `SKILL.md` uses **YAML frontmatter + markdown instructions** (as shown in [agent_script.md](file:///home/chaschel/Documents/go/bchat/bugs/059/agent_script.md))
- The plan's SKILLS.md uses **HTML comment annotations** (`<!-- @skill: ... -->`) — this is bchat's custom format, not agentskills.io
- agentskills.io skills are **instructions for the agent to follow**, not compiled handler references
- The `scripts/` directory in agentskills.io contains shell/Python scripts the agent runs, not compiled Go functions

The plan conflates two different paradigms: "agent follows skill instructions" (agentskills.io) vs "compiled handler DAG" (this plan). These are fundamentally different execution models.

---

## 🟡 Significant Gaps (Must Address Before Implementation)

### G1. Missing: How do skills relate to the existing system prompt pipeline?

The existing `buildSystemPrompt()` at [service.go L3113](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L3113) assembles:
1. Identity (from POLICY.md)
2. RAG context
3. Services (from KB.md)
4. Exclusions (from KB.md)
5. **Conversation flow (from SCRIPT.md)**
6. Policies & rules
7. Learned behaviors
8. FAQs
9. Observational memory

Where do skills appear in this prompt? As tool descriptions? As additional system prompt sections? The plan is silent.

### G2. Missing: Condition evaluator specification

The plan says:
```go
evaluator := NewExpressionEvaluator(checkpointData)
```

But never specifies:
- What language the conditions are written in (CEL? Custom? Go templates?)
- How they handle type safety (the checkpoint data is `map[string]any`)
- Injection risks (user-uploaded SKILLS.md contains condition strings)
- bchat already uses CEL for other filtering — should this reuse it?

### G3. Missing: Concurrency model for parallel DAG execution

The plan mentions parallel execution in Open Questions but the DAG executor is serial:
```go
for attempt := 0; attempt <= node.MaxRetries; attempt++ {
```

For a DAG where multiple nodes at the same level have no dependencies on each other, serial execution wastes time. The plan needs to define:
- `sync.WaitGroup` or `errgroup.Group` for parallel level execution
- How parallel node results are merged into `checkpoint_data`
- What happens when one parallel branch fails

### G4. Missing: State machine transitions for SkillExecution

The plan defines states: `pending, running, completed, failed, suspended`. But there's no transition diagram or guard:
- Can `completed` → `running`? (re-execution)
- Can `failed` → `pending`? (manual retry)
- What triggers `suspended`?
- How does `suspended` → `running` work?

The existing [AgentWorkflow](file:///home/chaschel/Documents/go/bchat/store/agent_workflow.go) already has `TaskMode` (PLANNING, EXECUTION, VERIFICATION) and `TaskStatus` — is there overlap or conflict?

### G5. Missing: Integration with existing AgentEvent system

The codebase already has an event system: [AgentEvent](file:///home/chaschel/Documents/go/bchat/store/agent.go#L1269) with `pending → processing → delivered → failed` lifecycle and `ClaimPendingEvents()` for worker polling. The plan creates a parallel system (`agent_skill_executions`) with similar semantics but no connection to the existing events.

### G6. Missing: What is a "handler" at runtime?

The plan shows `node.Handler.Execute(ctx, node.Input)` but `SkillNode.Handler` is typed as `string` (the handler name). There's no:
- `SkillHandler` interface definition
- Resolution from string name to callable
- Error handling for unregistered handlers
- Sandbox/permission model for what handlers can access

### G7. Missing: How does the DAG persist across restarts?

The plan says "resume from checkpoint" but:
- Go goroutines don't survive process restarts
- The plan uses `time.Sleep` for retry backoff — this blocks but doesn't survive restarts
- There's no background worker/scheduler that polls for resumable executions on startup
- The existing codebase uses `go func()` goroutines for background work (ticket embedding, reindex) — these all lose state on restart

### G8. Missing: Schema for `input_schema` / `output_schema`

The plan uses JSON Schema for input/output validation but never defines:
- The validation library (jsonschema? gjson? custom?)
- How inputs are sourced from the conversation vs upstream skill outputs
- How the first skill in a DAG gets its input (from the user message? from the LLM's tool call?)
- Type coercion rules

### G9. Missing: Observational Memory interaction

The existing [ObserverBuffer](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L198) compresses conversation history. How do skill executions interact with OM?
- Do skill outputs count toward the token budget?
- Are skill execution logs part of conversation history that gets observed?
- Does the observer compress skill DAG state?

---

## 🟢 What the Plan Gets Right

1. **Durable execution is the right primitive.** The insight that bchat needs start/stop signals and crash-recoverable state is correct.
2. **DAG-based skill orchestration** is the right topology for multi-step automation.
3. **Checkpoint/resume** is the right approach for long-running workflows.
4. **CockroachDB as the state store** is architecturally sound for distributed execution.
5. **Audit trail via skill logs** is necessary and well-structured.
6. **Error classification** (transient vs permanent) is production-grade thinking.

---

## Required Rework

### R1. Reconcile SKILLS.md with the existing 3-file model

**Option A (Recommended):** Make SCRIPT.md the skill definition file. Extend the existing `<!-- @annotation -->` system in [parser.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go) to support `@skill`, `@handler`, `@depends_on`, etc. No 4th file. The parser already handles sections and annotations — extend, don't replace.

**Option B:** Add SKILLS.md as a 4th file type. Update the upload handler, add `skill` to the `file_type` enum, create parser, update `buildSystemPrompt()` to inject skill descriptions as tool definitions.

### R2. Define the handler execution model

Pick one:
1. **LLM-as-executor:** Skills become tool descriptions in the system prompt. The LLM decides which to call. Handlers are LLM-callable tools (bchat already calls OpenRouter — this is the natural extension).
2. **Built-in handler set:** A curated set of Go handlers (classify_intent, search_kb, create_ticket, send_message, webhook_call) that are generic and configured via the skill's input schema.
3. **Hybrid:** Some skills are LLM-executed (reasoning), some are deterministic Go handlers (DB writes, webhooks). The `@handler` annotation distinguishes them.

### R3. Define SQLite schema equivalents

For every CockroachDB table, provide SQLite DDL. Use `TEXT` for JSON, `TEXT` for UUID, `INTEGER` for timestamps. Follow the existing migration pattern in `store/migration/sqlite/`.

### R4. Add tenant scoping to executor

Every `SkillExecutor` method must extract `tenantID` from context and scope all queries. Follow the [existing pattern](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go#L62-L97).

### R5. Define integration points with existing pipeline

Specify exactly:
- Where in `buildSystemPrompt()` skill definitions appear
- How skill execution is triggered (tool call from LLM? explicit API call? intent match?)
- How skill results feed back into the conversation
- How the verifier ([verifier.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/verifier.go)) validates skill outputs

### R6. Add startup recovery worker

On process start, query `agent_skill_executions WHERE status IN ('running', 'pending') AND retry_after < NOW()` and resume. This is the missing piece for true durability.

---

## Open Questions I'd Add

1. **Token budget:** A SKILLS.md with 5 skills, each with schemas and conditions, adds ~2K tokens to the system prompt. With KB + Policy + Script + RAG context + OM, are we hitting context window limits?

2. **Determinism:** If the LLM decides skill execution order, the DAG is advisory at best. If the DAG is authoritative, the LLM is demoted to a tool. Which is it?

3. **Backward compatibility:** Can tenants that only have KB.md + POLICY.md + SCRIPT.md (no skills) continue to work unchanged? The plan doesn't address this.

4. **Testing:** The plan has no test strategy beyond "unit tests" and "integration tests." How do you test a skill DAG that involves LLM calls without burning API credits? Mock embedding already exists — does the plan need a mock skill executor?

---

*Review completed: 2026-08-05T04:44+08:00*
