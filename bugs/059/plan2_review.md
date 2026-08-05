# Adversarial Plan Review — bugs/059 Plan 2.0 (plan2.md)

**Reviewer posture:** Senior Go architect, database expert, automation/RAG pipeline designer.  
**Reviewed artifact:** [plan2.md](file:///home/chaschel/Documents/go/bchat/bugs/059/plan2.md)  
**Codebase baseline:** bchat @ current HEAD — [service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go), [handlers.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go), [parser.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go), [store/agent.go](file:///home/chaschel/Documents/go/bchat/store/agent.go), [store/driver.go](file:///home/chaschel/Documents/go/bchat/store/driver.go)

---

## Verdict: **APPROVED WITH NITS** 🟢

`plan2.md` is a comprehensive, production-grade architecture revision. It successfully resolves all **6 structural flaws**, **9 significant gaps**, and **4 open questions** raised in the initial review of `plan.md`.

By unifying skill execution under extended `SCRIPT.md` annotations, utilizing a hybrid execution model (curated Go `builtin:` side-effect handlers + `llm:` reasoning handlers), integrating directly with OpenRouter tool-calling, and implementing multi-driver database parity with tenant scoping, this plan transforms bchat into a durable automation pipeline without violating core architectural constraints.

---

## 🟢 Summary of Resolved Flaws from Plan 1.0

| Issue | Resolution in Plan 2.0 | Status |
|-------|------------------------|--------|
| **F1: 4th file (SKILLS.md)** | Extended `SCRIPT.md` with `<!-- @skill: ... -->` annotations. Reuses existing upload API (`file_type=script`) and `parser.go` infrastructure. | ✅ Fixed |
| **F2: Compiled handlers** | Hybrid model: curated Go `builtin:` handlers for deterministic actions + `llm:` handlers for dynamic reasoning. Preserves "no code changes per tenant". | ✅ Fixed |
| **F3: Disconnected pipeline** | Embedded skills into standard LLM tool-calling loop. Tool outputs feed back into chat context while preserving policy/persona enforcement. | ✅ Fixed |
| **F4: CockroachDB-only** | Added full SQLite schema (Section 5.2) with Unix epoch timestamps, `TEXT` UUIDs, and `TEXT` JSON fields. Preserves `task build` and `task validate:parity`. | ✅ Fixed |
| **F5: No tenant isolation** | Added mandatory `tenantID` extraction and scoping across all `SkillExecutor` and store methods (Section 9). | ✅ Fixed |
| **F6: Cosmetic compliance** | Adopted bchat's native HTML annotation syntax in `SCRIPT.md` instead of mismatched external formats. | ✅ Fixed |
| **State Machine & Durability** | Clean 5-state machine (`pending`, `running`, `completed`, `failed`, `suspended`). Immutable terminal states with clone pattern (`/retry`). | ✅ Fixed |
| **Crash Recovery** | Startup recovery worker (Section 10) resets interrupted `running` states to `pending` after process restarts. | ✅ Fixed |

---

## 🔍 Implementation Nits & Refinements

While `plan2.md` is approved for execution, the following **6 implementation-level nits** should be incorporated during development:

### Nit 1: Missing MySQL Driver Parity
* **Observation:** `plan2.md` provides CockroachDB (Section 5.1) and SQLite (Section 5.2) DDL, but bchat explicitly supports **three** database drivers: SQLite, PostgreSQL/CockroachDB, and MySQL (`store/db/mysql`).
* **Requirement:** When adding store methods to `store/driver.go` (e.g. `CreateSkillExecution`), stub or implement them in `store/db/mysql/` as well so the `*mysql.DB` struct continues to satisfy the `store.Driver` interface. Include MySQL DDL script templates in `store/migration/mysql/` if MySQL migrations are maintained in your environment.

### Nit 2: HTML Comment Annotation Parser Formatting
* **Observation:** In Section 3.1, the example shows multi-line annotations:
  ```markdown
  <!-- @skill: classify_intent -->
  <!-- @handler: builtin:classify_intent -->
  <!-- @depends_on: none -->
  ```
  In [parser.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go#L99), `extractAnnotationBlocks` matches individual `<!-- @type: params -->` lines as single `annotationBlock` objects.
* **Requirement:** Either use inline parameters per skill block:
  ```markdown
  <!-- @skill: classify_intent, handler: "builtin:classify_intent", depends_on: "none", timeout: "30s" -->
  ```
  Or update `parser.go`'s `extractSkillBlocks` to associate contiguous annotation comment lines with the preceding `@skill` declaration. Single-line inline parameters are recommended as they are cleaner and less ambiguous.

### Nit 3: SSE Streaming Progress Feedback (`/chat/stream`)
* **Observation:** Section 8.2 outlines `processChatWithToolCalls` synchronously. When a client uses `GET /api/v1/agent/:slug/chat/stream`, multiple tool-calling iterations may occur before final text generation.
* **Requirement:** Ensure the SSE handler emits progress events (e.g., `event: skill_start`, `event: skill_complete`) during long-running tool executions. This prevents stream timeouts and gives the chat UI immediate visibility into background skill steps.

### Nit 4: CEL Evaluator Variable Scope
* **Observation:** Section 7.3 defines the CEL environment with a single top-level `checkpoint` map variable (`cel.Variable("checkpoint", ...)`), while Section 3.1 writes guard conditions as `search_kb.found == false`.
* **Requirement:** In CEL, evaluating `search_kb.found == false` directly expects `search_kb` as a top-level identifier. Ensure `evaluateCondition` populates top-level node names into the CEL evaluation context map, or format expressions as `checkpoint.search_kb.found == false`.

### Nit 5: Fail-Fast Upload Validation
* **Observation:** Section 3.2 shows DAG construction during `ParseScriptWithSkills`.
* **Requirement:** When a user uploads a new `SCRIPT.md` via `POST /api/v1/agent/:slug/files`, run `graph.Validate()` immediately. If cycle detection or missing dependencies fail, return `400 Bad Request` with exact line numbers and missing dependency details to prevent invalid DAGs from reaching the database.

### Nit 6: OpenRouter Go SDK Type Names
* **Observation:** Section 4.3 and 8.2 pseudocode references `openrouter.ChatRequest` and `openrouter.Tool`.
* **Requirement:** The `github.com/revrost/go-openrouter` package (v1.1.5 in `go.mod`) uses `openrouter.ChatCompletionRequest`, `openrouter.ChatCompletionResponse`, and `openrouter.ChatCompletionMessage`. Use exact SDK type names during Phase 3 integration.

---

## 🚀 Final Recommendation

Proceed directly to **Phase 1 (Core Infrastructure)** implementation following the 5-phase schedule in `plan2.md`. All foundational design decisions are solid and aligned with the bchat architecture.
