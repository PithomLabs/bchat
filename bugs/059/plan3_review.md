# Adversarial Plan Review — bugs/059 Plan 3.0 (plan3.md)

**Reviewer posture:** Senior Go architect, database expert, automation/RAG pipeline designer.  
**Reviewed artifact:** [plan3.md](file:///home/chaschel/Documents/go/bchat/bugs/059/plan3.md)  
**Codebase baseline:** bchat @ current HEAD — [service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go), [handlers.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go), [parser.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go), [store/agent.go](file:///home/chaschel/Documents/go/bchat/store/agent.go), [store/driver.go](file:///home/chaschel/Documents/go/bchat/store/driver.go)

---

## Verdict: **APPROVED WITH NITS** 🟢

`plan3.md` is a complete, master-class architectural blueprint. It seamlessly integrates:
1. **Durable Execution & State Management** (6-state machine, CockroachDB/SQLite/MySQL multi-driver parity, startup crash recovery worker).
2. **Hybrid Execution Model** (Curated Go `builtin:` side-effect tools + `llm:` reasoning steps).
3. **Start & Stop Signal Lifecycle** (`@trigger: start` / `@signal: stop` annotations, `/workflows/start` & `/executions/:id/stop` API signals, outbound `AgentEvent` emissions).
4. **All plan2 Review Refinements (N1–N6)** (MySQL driver DDL/interface parity, single-line annotation parsing, SSE progress event streaming, top-level CEL variable evaluation, fail-fast upload validation, exact OpenRouter Go SDK types).

This plan fully transforms bchat into a general-purpose, durable automation pipeline while strictly respecting bchat's core design principles (multi-tenant isolation, 3-driver parity, declarative configuration, and backward compatibility).

---

## 🟢 Changelog Validation (Plan 2.0 → Plan 3.0)

| ID | Feature / Refinement | Verification in Plan 3.0 | Status |
|----|----------------------|---------------------------|--------|
| **S1** | `@trigger` & `@signal` Annotations | Declared in `SCRIPT.md` section 3.4 (`type`, `event_type`, `condition`, `emit_event`). | ✅ Verified |
| **S2** | `POST /workflows/start` API | Handler implementation in Section 11.2 with event/API payload seeding. | ✅ Verified |
| **S3** | `POST /executions/:id/stop` API | Handler implementation in Section 11.2 updating status to `stopped`. | ✅ Verified |
| **S4** | Lifecycle Helper Functions | `StartWorkflowSignal()` and `EvaluateStopSignal()` detailed in Sections 3.4 & 7.3. | ✅ Verified |
| **S5** | 6-State Lifecycle Machine | States (`created`, `pending`, `running`, `completed`, `stopped`, `failed`) and rules detailed in Section 6. | ✅ Verified |
| **N1** | MySQL Driver Parity | MySQL DDL provided in Section 5.3 alongside SQLite and CockroachDB. | ✅ Verified |
| **N2** | Single-Line Annotations | Inline syntax `<!-- @skill: name, handler: "...", ... -->` adopted in Section 3.1. | ✅ Verified |
| **N3** | SSE Progress Streaming | `emitSSEEvent` with `skill_start` and `skill_complete` events added to Section 8.2 & 8.3. | ✅ Verified |
| **N4** | Top-Level CEL Context | Top-level node variables populated in `evaluateCondition` (Section 7.2). | ✅ Verified |
| **N5** | Fail-Fast Upload Validation | `graph.Validate()` called during upload in Section 3.2 & 3.3. | ✅ Verified |
| **N6** | Exact SDK Type Names | `openrouter.ChatCompletionRequest` and `ChatCompletionMessage` used in Section 4.3 & 8.2. | ✅ Verified |

---

## 🔍 Minor Implementation Nits for Phase 1–3

The plan is approved for immediate implementation. Keep the following 3 minor code-level nits in mind during execution:

### Nit 1: Tenant ID Struct Field Pointer Alignment
* **Observation:** In `StartWorkflowSignal` (Section 3.4), `tenantID` is passed as an `int32` value, whereas `SkillExecution.TenantID` in `store/agent.go` (Section 5.4) is typed as `*int32`.
* **Action:** Wrap `tenantID` with a pointer (`&tenantID`) when populating `SkillExecution` structs to stay consistent with bchat's pointer-based tenant ID convention (`*int32`).

### Nit 2: SSE Channel Lifecycle & Cleanup
* **Observation:** In Section 8.3 (`emitSSEEvent`), events are written to `s.eventChan`.
* **Action:** Ensure the Echo SSE handler allocates per-client event channels and cleans up (closes/unsubscribes) when the HTTP context closes (`c.Request().Context().Done()`) to prevent goroutine or memory leaks during high concurrency.

### Nit 3: API Endpoint RBAC Middleware Wiring
* **Observation:** Section 11.2 defines handlers for `/workflows/start` and `/executions/:id/stop`.
* **Action:** Register these routes under `/api/v1/agent/:slug/...` in `server/router/api/v1/v1.go` using `TenantBindingMiddleware` and verify RBAC permissions (`PermChatTest` or `PermApiConfig`) using `h.hasPermission(c, tenant.ID, ...)`.

---

## 🚀 Execution Approval

**Plan 3.0 is APPROVED.** Proceed to Phase 1 (Core Infrastructure) implementation as outlined in Section 15.
