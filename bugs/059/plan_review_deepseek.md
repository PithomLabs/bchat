# Adversarial Plan Review — bugs/059 Plan 3.0 (plan3.md)

**Reviewer posture:** Senior Go architect, database/automation expert, RAG + agent-framework designer.
**Reviewed artifact:** [plan3.md](./plan3.md) — Durable Execution Architecture, v3.0.
**Review level:** **Implementation plan** — architecture, state machines, seams, ownership, sequencing.
  NOT a line-level code review. Pseudocode is judged only where it exposes a design gap, never for field/type exactness.
**Codebase baseline:** bchat @ current HEAD — `server/router/api/v1/agent/*.go`, `store/*.go`, `store/db/*.go`, `store/migration/*`, `server/router/api/v1/v1.go`.
**Prior-art lineage read:** `plan.md` (REWORK), `plan059_review.md` (R1–R6 + Q1–Q4), `plan2.md` (APPROVED+nits), `plan2_review.md` (N1–N6), `state_machine.md`, `plan3_review.md`.

---

## Verdict: **CONDITIONAL APPROVE — carry the D1–D9 carve list into plan4** 🟡

`plan3.md` is architecturally the right shape: declarative SCRIPT.md extension, DAG-constrains/LLM-decides execution, hybrid builtin+LLM handlers, clone-not-mutate retry, a 6-state machine, a start/stop lifecycle, and genuine multi-driver intent. It correctly resolves every structural flaw (`F1–F6`) and gap (`G1–G9`) that killed `plan.md`, and it incorporates all of `plan2_review.md`'s N1–N6.

It is **not** "APPROVED WITH NITS" as `plan3_review.md` concludes. Nine subsystems are specified to *draft depth* rather than *design-complete*: their interfaces are named but their **data paths, owners, and lifecycle boundaries are undefined**. None of these invalidate the architecture; each must be closed as a design decision before — or very early during — Phase 1 implementation. Hence "conditional."

The distinguishing judgement of this review: **drawing/specifying a state machine is not the same as assigning an owner to it.** plan3 defines the states but never says *who* moves executions between them, *what* runtime drives the loop, or *who* resumes a recovered execution. That single omission — the execution-loop ownership model (D2 below) — is the one item that, if left open, makes "durable execution" a recording system rather than a durable one.

---

## ✅ What plan3 gets genuinely right

| # | Decision | Why it's correct |
|---|----------|------------------|
| R1 | Extend SCRIPT.md via `@skill` annotations instead of a 4th SKILLS.md file | No upload-flow change, parser extended not replaced, backward compatible, single source of truth feeding `buildSystemPrompt()`. |
| R2 | Hybrid execution: curated `builtin:` Go handlers + `llm:` reasoning steps | Preserves "no code changes per tenant" (tenants compose, never write Go) while keeping side-effects deterministic and sandbox-contained. |
| R3 | DAG-constrains / LLM-decides | The only architecture that survives production contact: rigid (n8n-style) breaks when a customer changes intent mid-flow; unconstrained tool-calling is chaotic. |
| R4 | Tool-calling as the execution protocol | Matches industry standard, reuses OpenRouter's existing (currently unused) `Tools` surface, keeps persona/policy in the loop, OM works automatically. |
| R5 | Clone-not-mutate retry, immutable terminal states | Preserves the audit trail; matches how bchat already treats `AgentEvent`/`ReindexCheckpoint` terminal states and Temporal's new-run-ID retry model. |
| R6 | 6-state machine incl. `stopped`, with crash recovery | Upgrade over plan2's 5-state; correct that a user-initiated stop is a real terminal state. |
| S1–S5 | `@trigger`/`@signal` lifecycle + `/workflows/start` + `/executions/:id/stop` | Answers the original mandate — explicit start and stop signals so an agent can be *turned on and off* as an automation. |
| N5 | Fail-fast DAG validation at upload time | Cycle/missing-dep errors with line numbers belong at upload, not mid-run. |
| -- | Multi-driver parity intent (SQLite/CRDB/MySQL DDL), tenant scoping section, recovery worker, SSE progress, CEL conditions | All the right architecture blocks are named. |

The core thesis — *"n8n needed three node graphs, bchat needs three markdown files + durable state"* — is coherent, defensible, and matches the hackathon judging criteria.

---

## 🟡 Design-Level Gaps — the D1–D9 carve list

Each of these is a **design specification gap**, not a code bug. Each names a boundary that plan3 leaves undefined. Close these as documented decisions before Phase 1 is considered complete.

### D1 — The start/stop signals have no data path
`@trigger: start` and `@signal: stop` are declared (§3.4) and the endpoints are sketched (§11.2), but the plan never specifies **who parses the trigger/signal annotations** or **how the parsed definition reaches `StartWorkflowSignal`/`EvaluateStopSignal`**. The parser extension (§3.2) only handles `@skill` blocks. `TriggerDefinition`/`StopSignalDefinition` exist as structs with no producer.

> **Close:** specify that `ParseScriptWithSkills` (or a sibling pass) also extracts `@trigger`/`@signal` blocks into the `SkillGraph` containing the trigger/stop metadata, and that this rides the existing config-cache load path so both the chat path and `/workflows/start` read from the same parsed graph.

### D2 — No owner for the execution loop (the critical one)
The plan describes an execution cycle (start → run nodes → checkpoint → finalize) but never says **what runtime executes it**. Three concrete unanswered questions:

1. Is execution **synchronous within the chat request** (i.e., `processChatWithToolCalls` inside `generateResponse`)? If so, a process crash mid-loop loses in-flight work — checkpointing only makes it *recorded*, not *resumed*. "Durable" then means "auditable," not "resumable."
2. Is there a **detached worker** that drains `pending`/`resumable` executions off the chat path (for `event`/`api`/`cron` triggers with no human in the loop)? Nothing is specified that picks up an execution started via `/workflows/start`.
3. For a chat-triggered run, **where does the loop live relative to `ChatExternal`/`ChatInternal` and `buildSystemPrompt()`**? The plan's §8.2 shows a standalone function but no call site.

> **Close:** pick one of (a) synchronous-with-detached-resume — chat runs inline, a background worker owns recovery/event-triggered runs; or (b) fully detached — chat and `/workflows/start` both enqueue, a worker claims and executes. (a) is recommended for the demo: least disruption to the existing synchronous chat path, and the recovery worker in §10 already exists as the seed of the detached half. State this explicitly in plan4.

### D3 — Stop semantics: DB-row flip ≠ cancelled work
`StopExecution` (§11.2) sets `status = 'stopped'` in the DB. The executing goroutine (LLM call in flight, handler mid-run, checkpoint write) is **not cancelled**. After `stopped`, in-flight work can still write checkpoints/nodes or a subsequent tool result — racing the terminal state. Recovery (§10) may even re-mark `running` work that the operator just explicitly stopped, depending on timing.

> **Close:** specify a per-execution cancellation token (`context.WithCancel`) checked before every node write and terminal transition; define that stop is *accepted* (DB) and *applied* (next checkpoint boundary), with a short grace window documented. Decide the race policy: an explicit stop must also mark the recovery loop to skip that execution.

### D4 — Condition engine: declaration only, no binding or failure policy
CEL conditions are described as evaluation during guards and stop signals, but the plan does not specify: how the evaluation context is **bound** from checkpoint data (plan3_review noticed the top-level names, but binding itself is unstated), what happens when a condition references a **node that hasn't run yet** (`search_kb.found` before `search_kb` completed), and whether eval failure **fails-open or fails-closed**.

> **Close:** define the binding contract (checkpoint keys → top-level CEL identifiers), the pre-requisite policy (reference to an incomplete node = condition not-yet-evaluable = "don't fire", evaluated again after the node completes), and a fail-safe default (fail-closed, logged, for stop/guard conditions).

### D5 — Recovery worker has no ownership/claim model
`ListPendingSkillExecutions` + reset-`running`-to-`pending` has no **claim, lease, or idempotency token**. Single-instance deployments: acceptable (minor re-entrancy risk). Multi-instance CockroachDB deployments — which is exactly the hackathon's scale story — will have **N workers recovering the same executions simultaneously**, double-marking nodes and duplicating side effects.

> **Close:** add a claim/lease (e.g., reuse the `claimed_at`/`attempts` pattern already used by `agent_events` pre-claim) so only one worker resumes an execution. Treat as a design requirement in the schema; defer live multi-node testing to post-demo if needed.

### D6 — Outbound event emission reinvents a queue instead of reusing the outbox
§7.3's `emit_event` creates `AgentEvent` rows directly, but the codebase already has a pre-claim outbox + immediate-delivery pattern (`dispatchEvent` in `service.go`, `agent_events` table, `ClaimPendingEvents` poller). Additionally the `AgentEvent` schema carries `integration_id NOT NULL` FK — the plan's emission has no integration coupling. Two outbox paths for the same event concept will drift.

> **Close:** `EvaluateStopSignal` (and any other emit point) **invokes the existing dispatch/outbox path** with a resolved integration, rather than inserting `AgentEvent` rows by hand.

### D7 — SSE progress events have no transport
`/api/v1/agent/:slug/chat/stream` (referenced by AGENTS.md and the plan) **does not exist in the codebase** — chat is synchronous JSON (`/chat/ext`, `/chat/int`); the only SSE is simulation (`HandleSimulationStream`) and there is no chat SSE route in `v1.go`. N3's `skill_start`/`skill_complete` events are emitted to an invented channel with no route, no payload contract, and no client.

> **Close:** either (a) spec a new public SSE route (`GET /api/v1/agent/:slug/chat/stream`) mirroring the simulation-stream pattern with an explicit event payload schema, or (b) explicitly drop N3 for the demo and stream progress only in the simulation/playground surface where SSE already exists. Recommend (b) to de-risk timing; document it as a deliberate scope cut, not an oversight.

### D8 — Multi-driver scope is under-stated (Cockroach *is* the postgres driver)
The plan frames parity as "SQLite, CockroachDB, MySQL DDL." The codebase reality:

- `NewCockroachDB()` **reuses `store/db/postgres`** (`postgres.NewCockroachDB`) — CockroachDB is not a fourth driver implementation; it is the postgres driver + CRDB-flavored migrations/vectors.
- Adding methods to the `store.Driver` interface (the plan's §5.4) requires implementations in **sqlite, postgres (postgres + cockroach), and mysql** — three implementations, four deployment targets.
- Migrations are **versioned per driver** (`sqlite/0.35`, `postgres/0.35`, `mysql/0.25`, cockroach single-file) **plus four `LATEST.sql` files** with a drift check (`validate-migrations.sh`).

> **Close:** restate D8 precisely in plan4: "3 Driver implementations, 4 deploy targets, 4 × versioned SQL + 4 × LATEST.sql, `task validate:parity` must pass." Reduces surprise, avoids the exact failure `task validate:parity` exists to catch.

### D9 — Per-tenant skill-graph lifecycle (caching/versioning) is unspecified
`buildSkillPrompt(session.Skills)` needs the tenant's *current* parsed `SkillGraph` at chat time, but plan3 only persists the graph **per execution** (`agent_skill_executions.skill_graph`). It does not specify where the per-tenant graph that feeds the chat loop comes from: re-parse SCRIPT.md every request? Cache keyed by content-hash? Invalidate on upload? Versioned?

> **Close:** define the graph as a derived, cached artifact of the SCRIPT.md content-hash riding the existing `LoadConfig`/config-cache path, invalidated on `HandleImportScript`. Per-execution graph rows then only store what ran, not the source of truth.

---

## 🗺️ Seam map — where each new subsystem must attach (coarse, not line-level)

| New subsystem (plan3 §) | Real bchat seam it must attach to |
|---|---|
| Parser extension (§3.2, §3.4) | `Parser` in `parser.go`; `HandleImportScript` (`handlers.go`) for fail-fast validation; `LoadConfig` config-cache path so graphs reach the prompt builder |
| Skill definitions in prompt (§8.1) | `buildSystemPrompt()` (`service.go`) — new SECTION 5.5 between conversation flow and policies |
| Tool-calling loop (§8.2) | `generateResponse()` / `ChatExternal` / `ChatInternal` (`service.go`) — the synchronous chat owner |
| Recovery worker (§10) | `NewService()` startup-goroutine pattern already used by ticket-embedding/reindex/auto-RAG workers |
| Outbound events (§7.3) | Existing `dispatchEvent` outbox + `agent_events` pre-claim delivery (`service.go`) |
| SSE progress (§8.3) | Simulation SSE pattern (`HandleSimulationStream`) — new chat stream route required, or cut |
| New endpoints (§11) | `v1.go` `authGroup` (authenticated) + `TenantBindingMiddleware`, RBAC via `h.hasPermission` (`PermChatTest`/`PermApiConfig`) |
| Store methods (§5.4) | `store.Driver` interface + sqlite / postgres(+cockroach) / mysql implementations + `Store` wrapper methods |
| Migrations (§5) | `store/migration/{sqlite,postgres,mysql}/<version>/NN__*.sql` + all four `LATEST.sql` + `task validate:parity` |
| Tenancy (§9) | `getTenantFromContext(c)` + tenant-scoped `Find*` filters everywhere, per AGENTS.md security invariants |

---

## 🔸 Nits (acknowledged once, not the focus at this review level)

1. **`SkillExecution.TenantID` pointer convention** — keep `*int32` to match `AgentSession`/store conventions; ensure `StartWorkflowSignal` populates it correctly (plan3_review Nit 1 already flagged).
2. **SSE per-client channel cleanup** — if a new chat stream route lands, replicate the simulation stream's request-context-done teardown (plan3_review Nit 2 agreed).
3. **RBAC wiring for the two new endpoints** — assign explicit permission (recommend `tenant:admin` for `/workflows/start`, `api:config`-adjacent for stop, or keep both RBAC-checked admin ops) rather than leaving default auth-only (plan3_review Nit 3 agreed).
4. **`depends_on: "none"` vs empty** — normalize to one convention in the parser so entry-point detection is single-valued.
5. **`agent_workflows` name proximity** — bchat already has an `AgentWorkflow`/`agent_workflows` concept (`store/agent_workflow.go`). Not a collision (different table, different purpose), but document the distinction so admins don't confuse task-boundary logs with skill executions.
6. **Recovery worker cadence/env** — `SKILL_RECOVERY_ENABLED` already named; also gate the worker on feature availability so non-RAG/local `task build` runs don't spawn it against un-migrated tables (mirror existing worker pattern).

---

## 🧭 Recommendation on the two open architecture questions

- **Q-A (execution ownership):** adopt **synchronous chat execution + detached recovery worker** (D2 option a). It preserves the existing synchronous chat contract for `/chat/ext` (no client impact), keeps the hackathon demo simple, and the §10 recovery worker is already the seed of the detached half. Event/API-triggered runs are coordinated by the same worker.
- **Q-B (multi-instance):** put **claim/lease on the schema now** (D5), but **defer live multi-node load testing to post-demo**. Single-instance demo remains safe; the design doesn't paint you into a corner.

---

## ✅ What to approve, and what to carry forward

**Approve now:** the SCRIPT.md extension contract, hybrid handler model, tool-calling protocol, clone-retry model, 6-state machine, start/stop lifecycle, fail-fast upload validation, multi-driver parity *intent*, and the demo carve of D7 (option b) + D5 (deferred testing).

**Carry as required design closures before Phase 1 is "done":** D1 (signal data path), D2 (loop owner), D3 (stop-cancel semantics), D4 (condition binding/failure policy), D6 (outbox reuse), D8 (3 impls / 4 targets / parity), D9 (graph lifecycle). D5 and D7 have explicit deferral paths above.

**Bottom line:** the architecture is approved to build toward; the carve list is the difference between *specifying* an automation platform and *operating* one. Close D2 first — it determines which of the other eight items are exercised at all.

---

*Review completed: 2026-08-05 (plan-level, per author calibration).*