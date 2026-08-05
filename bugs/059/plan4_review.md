# Adversarial Plan Review — bugs/059 Plan 4.0 (plan4.md)

**Reviewer posture:** Senior Go architect, database/automation expert, RAG + agent-framework designer.
**Reviewed artifact:** [plan4.md](./plan4.md) — Durable Execution Architecture, v4.0.
**Review level:** **Implementation plan** — architecture, state machines, seams, ownership, sequencing.
  NOT a line-level code review. Pseudocode is judged only where it exposes a design gap, never for field/type exactness.
**Codebase baseline:** bchat @ current HEAD — `server/router/api/v1/agent/*.go`, `store/*.go`, `store/db/*.go`, `store/migration/*`, `server/router/api/v1/v1.go`.
**Prior-art lineage read:** `plan.md` (REWORK), `plan059_review.md`, `plan2.md` (APPROVED+nits), `plan2_review.md` (N1–N6), `plan3.md`, `plan3_review.md`, `plan_review_deepseek.md` (D1–D9 vault), `state_machine.md`.

---

## Verdict: **REWORK — targeted, do not start Phase 1 yet** 🔴

`plan4.md` is the strongest version of the architecture to date. Every D1–D9 carve from `plan_review_deepseek.md` is addressed *on paper*, and home my claim that the architecture direction is sound. D2, D5, D6, D7, D9 are the most credibly closed here.

However, the plan opens with **"Ready for implementation — All design gaps (D1–D9) are closed."** Three of the six resolved carves are **not actually closed** — they are *re-stated attractively with code that still does not bind to the real seams*. Because the mandate is adversarial review and this plan's *entire point* is "all gaps are closed," the honest verdict is a targeted rework, not approve-with-nits.

Summary of what must change before Phase 1:

1. **D4 (CEL binding) — still unimplementable as written.** `env.Eval(ast)` is not an API that exists in cel-go v0.25.0; the `vars` map built at §8.1:862 is never passed to an evaluator; and the binding contract's top-level identifiers (`search_kb`, `create_ticket`, …) are never declared to the CEL env, so they fail at compile, not eval.
2. **D2 (loop owner) — resolved, but introduces a double-owner race.** The chat path and the detached worker can now own the *same* execution row concurrently.
3. **D3 (stop semantics) — resolved for chat, silent for detached.** Detached runs are never cancelled and can overwrite `stopped`.
4. **D1 (signal data path) — the parser contract and the annotation format disagree** about where the skill name lives (`params["code"]` vs `block.Params["name"]`), and line numbers for fail-fast errors do not exist on `annotationBlock` today.
5. **D8 — CockroachDB migration statement is factually wrong** vs the current repo (Cockroach *does* have versioned migrations now, not just a single LATEST.sql).
6. **N8 — handler pseudocode calls a permission helper that does not exist** (`service.CheckUserPermission`); the real seam is `h.hasPermission` + permission constants.

Each is a design-level gap exposed by reading plan4 against the actual codebase, not a field/type nit. Resolving them is edit-class work, not a re-architecture — hence "targeted rework."

---

## ✅ What plan4 gets genuinely right (verified against the codebase)

| # | Decision | Why it's correct |
|---|----------|------------------|
| D6 | Outbound events reuse `dispatchEvent` outbox | Signature matches exactly — `dispatchEvent(ctx, tenantID int32, leadID string, eventType string, data string)` (service.go:5422). §9 is a faithful reuse of the pre-claim outbox + immediate-delivery goroutine. |
| D9 | Skill graph rides `LoadConfig`/`configCache`, invalidated on import | Verified: `LoadConfig(ctx, tenantSlug, audienceType)` (service.go:1845), `ConfigCache.Get/Set/Invalidate` (service.go:1587-1688), and `HandleImportScript` → `InvalidateConfigCache(tenant.Slug)` (handlers.go:4129). The lifecycle described in §5.6/§3.2 is the real path. |
| D5 | Claim/lease schema + atomic claim SQL (CRDB flavor) | The `UPDATE … WHERE status='pending' OR (status='running' AND claim_expires_at < NOW()) RETURNING *` pattern is the correct single-statement lease claim. Matches the `agent_events` pre-claim philosophy. |
| D7 | SSE explicitly scoped out for demo, documented as deliberate | Matches reality: no `/chat/stream` route exists; simulation SSE (`HandleSimulationStream`) is the only stream transport. §12.3 is honest and correct. |
| N7 | `*int32` tenant pointer convention throughout | Matches `AgentSession`/store conventions. §5.4 structs are consistent. |
| N5/N9 | Fail-fast graph validation wired into `HandleImportScript` (handlers.go:4059) with line numbers; empty-`depends_on` entry-point convention, no `"none"` | Both land on the right seam with the right UX (upload-time 400). |
| -- | State machine (6 states), clone-not-mutate retry, hybrid builtin+LLM handlers, `SkillHandler`/`SkillRegistry` | All structurally sound and consistent with prior-art carry. |

The seam map below (§9 of this review) is more accurate than plan3's because D6/D9 are now actually *sewn*, not just named.

---

## 🔴 Design-level gaps that survive — must rework

### R1 — D4 is *re-stated*, not *closed*: the CEL binding still cannot run

§8.1 `evaluateCondition` (plan4.md:856-877):

- **`env.Eval(ast)` does not exist.** In the pinned cel-go v0.25.0, `*cel.Env` has `Compile`, `Program`, `Check`, `Parse` — **no `Eval` method** (verified against `cel/env.go:349-749`). The repo's own consumer (`plugin/filter/filter.go:37-46`) uses `cel.NewEnv` → `e.Compile` → clear to `prg.Eval`. plan4 writes a *different, nonexistent* evaluation API.
- **The `vars` map is built and never used.** §8.1:862-869 populates `vars["checkpoint"]` + top-level node names, then the next executable statement evaluates `env.Eval(ast)` with **no variables at all**. Even if a correct Program-based eval were used, `prg.Eval(cel.Vars(vars))` is absent — nothing binds `checkpoint` or any node output to the expression.
- **Top-level identifiers are not declared to the env.** The binding contract (§8.1 table) promises `search_kb`, `create_ticket`, etc. as top-level identifiers. cel-go requires `cel.Variable("search_kb", ….Type)` at env construction; undeclared identifiers fail at **compile/check** time. `newCLEnv` (§8.1) declares only `checkpoint`. So the moment a tenant writes `condition: "search_kb.found == false"` — the plan's *own example* — the expression fails to compile, and per §8.3 fail-closed policy the skill never fires.
- **`compileCLE` is referenced but never defined** (§8.1:857), while `newCLEnv` (defined) is unreferenced — the two helpers disagree and neither produces a bound evaluator.

> **Close R1:** (a) bind via `env.Compile` → `env.Program` → `prg.Eval(cel.Vars(vars))`; (b) declare every runtime node name as `cel.Variable(name, cel.DynType)` (or commit to accessing everything through `checkpoint.<node>`, and rewrite the contract table + example conditions); (c) define `compileCLE` or delete it; (d) add a unit test §15.1 actually exercises the real API (current tests call `evaluateCondition` directly — fix them to the real function). This is the single highest-risk item because a silently-failing condition engine defects the *whole* guard/stop system at runtime, not at upload.

### R2 — D2's sync-chat + detached-worker model has a double-owner race

§7.1/§7.3 make the chat path create execution rows with checkpoints (the recovery worker "marks the execution pending and the next user message… picks it up" on restart). §10.2's worker drains **every** `pending` row via `ListPendingSkillExecutions` with **no path/trigger discriminator**:

- A chat-path run creates a row (say status `pending`/`created`) and executes in-request. The 60s worker scan can **claim the same row** (`status='pending'`) while the sync request is mid-flight → two owners, duplicated side effects, checkpoint clobber.
- Nothing in the schema distinguishes "owned by an in-flight HTTP request" from "owned by the detached worker" — no per-row owner field beyond the claim columns, which the chat path never sets.

> **Close R2:** either (a) chat-path executions are *never* visible to `ListPendingSkillExecutions` — add a `trigger`/`path` column (`'chat'` vs `'event'|'api'|'cron'`) and filter the list; or (b) the chat path claims its own row synchronously at start and the worker's claim predicate excludes rows whose human in-flight owner is alive. Also state what the chat path does on abnormal termination (does it crash-left a row that the worker *should* then resume?). The plan's §7.1 text claims this, but §7.3 code and §10.2 code never establish it.

### R3 — D3's stop is applied for chat, silent for detached

§7.4 `StopExecution` cancels via `s.activeExecutions[cancel]` — but only the **chat path** registers there (§7.3). Detached goroutines (`go s.executeDetachedPipeline`, §10.2:1067) run on `context.Background()` and are **never registered, never cancelled**. Sequence:

1. Worker claims exec, status `running`, goroutine running.
2. Operator calls `/executions/:id/stop` → row flipped to `stopped`, cancel lookup in `activeExecutions` misses (no entry for detached).
3. In-flight goroutine writes the *next* checkpoint (`executeSkill` → `UpdateSkillExecution`, §11:1104) — overwriting `stopped` with its in-memory status → terminal state effectively lost; or the worker's "skip if `stopped`" guard (§10.2:1061-1064) only runs at *claim time*, long after.

> **Close R3:** register detached executions' cancel funcs in `activeExecutions` too (or a second map), and make every checkpoint write **re-read row status** — `if row.Status == "stopped"` → abort before write. Alternatively (documented): stop is *accepted-only* for detached runs and applied on next claim post-lease-expiry. Pick one and write it; the plan currently asserts "applied," which only holds for chat.

### R4 — D1 parser contract vs the real annotation parser disagree

plan4 §3.2 iterates `block.Type`, `block.Params["name"]`, `block.Params["handler"]`, `block.LineStart`:

- **The skill name lives under `params["code"]`, not `params["name"]`.** The existing `parseParams` (parser.go:172-200) stores a key-less first positional value as `params["code"]` (parser.go:195). The plan's own format `@skill: classify_intent, handler: …` parses `classify_intent` into `params["code"]`, so `block.Params["name"]` is **always empty** → `graph.Nodes[""] = …`, entry-point detection and `depends_on` resolution break.
- **`annotationBlock` has no `LineStart`.** Verified struct (parser.go:92-97): `annotationType`, `params`, `title`, `content` only. plan4 uses `block.LineStart` for the upload error line (§3.3:273) — the parser does not track line positions today, so the promised "line number in fail-fast validation" needs a parser extension that plan4 does not specify.

> **Close R4:** (a) read `block.Params["code"]` for the skill name (or normalize the annotation syntax so a named key maps to `name` — whichever, the plan must match `parseParams` behavior); (b) add line-offset tracking to `extractAnnotationBlocks` (the `matches` returned by `FindAllStringSubmatchIndex` already carry byte offsets — a `LineStart int` field can be derived cheaply). This is where Phase 1's parser work actually starts.

### R5 — D8's CockroachDB migration statement is factually wrong today

§5.5: "4 × versioned SQL + 4 × LATEST.sql … # CockroachDB: single LATEST.sql file". Verified the repo:

```
store/migration/cockroach/         # NOW VERSIONED, not single-file
  0.35/00__tickets_add_internal_notes.sql
  LATEST.sql
```

Cockroach uses the **same versioned-folder + LATEST.sql drift pattern** as sqlite/postgres/mysql. So the precise statement D8 was supposed to stand on is inaccurate, and the plan's own "3 implementations / 4 deploy targets" table (§5.5) is right while its migration-file line is wrong.

> **Close R5:** restate as **4 versioned dirs (`sqlite`, `postgres`, `mysql`, `cockroach`) + 4 × `LATEST.sql` + `task validate:parity` must pass.** No design change — a factual correction that matters because D8's whole purpose was eliminating `validate:parity` surprise.

### R6 — N8 handler pseudocode calls a permission helper that does not exist

§12 uses `h.service.CheckUserPermission(c.Request().Context(), userID, *tenantID, "…")` in both new handlers. Verified: no `CheckUserPermission` anywhere in the agent package; the real seam is **`func (h *Handler) hasPermission(c echo.Context, tenantID int32, permission string) bool`** (handlers.go:2391) with constants (`PermTenantRead`, `PermTenantAdmin`, `PermChatTest`, `PermChatLogs`, `PermFilesUpload`, `PermApiConfig`). String literals `"tenant:admin"` / `"api:config"` are not the RBAC vocabulary.

> **Close R6:** write the handlers against `h.hasPermission(c, tenant.ID, PermXxx)` matching the existing pattern (`!h.isAdmin(c) && !h.hasPermission(c, tenant.ID, Perm…)`), and document the RBAC assignment (recommend `PermApiConfig` for `/executions/:id/stop`, `PermTenantAdmin` for `/workflows/start`).

---

## 🟡 Carry-over nits (acknowledged, lower priority)

1. **`SKILL_MAX_RETRIES`/`SKILL_TIMEOUT` env vars exist but the loop hardcodes 10** (§7.3). Fine for demo; wire the env or document.
2. **Stop-surface duplication with N11 gating** — `SKILL_RECOVERY_ENABLED` + `s.ragEnabled` gating in §10.2 is good; keep the migration-parallel-startup `Sleep(10s)` ugly but flagged (mirror existing worker pattern instead if possible).
3. **`agent_workflows` name proximity** — §9's distinction paragraph is good; keep it visible to admins in docs.
4. **`ListPendingSkillExecutions` global scope** — no tenant filter is correct for a global worker, but ensure the per-row tenant scoping on *execution* (`FindSkillExecution{TenantID}`) is enforced in every store method, per N7.
5. **`dispatchEvent` deps** — it short-circuits when a tenant has no active `webhook` integrations (service.go:5432-5434). If `emit_event` is meant to fire pipeline-completion webhooks, that is the correct existing contract; just document that without a configured webhook the event is a no-op.

---

## 🗺️ Seam map — where plan4's subsystems attach (coarse, not line-level)

| New subsystem (plan4 §) | Real bchat seam | Status after rework |
|---|---|---|
| Parser extension (§3.1-3.3) | `Parser` (parser.go:92-200) + `HandleImportScript` (handlers.go:4059) | Fix R4 (name key + LineStart) |
| CEL evaluation (§8) | `plugin/filter/filter.go` pattern + cel-go v0.25.0 `env.Compile`/`env.Program`/`prg.Eval` | Fix R1 — block after this |
| Tool-calling loop (§7.2/7.3) | `generateResponse()` / `ChatExternal` / `ChatInternal` (service.go) | Fix R2 (chat-can't-be-claimed) + R3 (register cancel, re-read status) |
| Recovery worker (§10) | startup-goroutine pattern (ticket-embed/reindex workers) + new claim SQL | Good; add path discriminator (R2) |
| Outbound events (§9) | `dispatchEvent` (service.go:5422) | ✅ Verified correct |
| Skill graph lifecycle (§5.6) | `LoadConfig`/`configCache` (service.go:1845-1958) + `InvalidateConfigCache` (handlers.go:4129) | ✅ Verified correct |
| New endpoints (§12) | `v1.go` `authGroup` + `TenantBindingMiddleware`, RBAC `h.hasPermission` + `PermXxx` | Fix R6 |
| Store methods (§5.4) | `store.Driver` interface + sqlite / postgres(+cockroach) / mysql | OK as written |
| Migrations (§5) | `store/migration/{sqlite,postgres,mysql,cockroach}/<ver>/NN__*.sql` + 4× `LATEST.sql` + `task validate:parity` | Fix R5 (cockroach is versioned) |
| Tenancy (§11) | `getTenantFromContext(c)` + tenant-scoped `Find*` filters | Add detached-path tenant injection (execution row → ctx) |

---

## 🧭 Recommendation

Do **one** targeted update pass on plan4 (R1–R6), not a rewrite. R1 is the gate: until the condition engine is specified against the real cel-go Program API with declared variables and a bound eval, nothing downstream (§8.3 fail-safe, §6 stop, §3.3 guards) is testable. R2/R3 are the two runtime-safety holes the hackathon demo could actually trip on (duplicate side effects under the recovery worker, stop not applying to detached runs). R4/R5 are parser/migration factual corrections that Phase 1 would otherwise discover as the *first* compile or the first `validate:parity` failure. R6 is a one-line seam fix.

Everything else — D6, D7, D9, the state machine, clone-retry, hybrid handlers, N7/N9 conventions — is genuinely closed and can be built to as-is.

---

## ✅ Bottom line

**plan4 is not "ready for implementation."** It is ready for a **targeted rework of R1–R6**, after which — with R1 verified against a real cel-go evaluator and R2/R3 closed as documented decisions — it earns "APPROVED WITH NITS." The architecture is correct; the claim that all design gaps are closed is what forces the rework verdict.

---

*Review completed: 2026-08-05 (plan-level, per author calibration — no line-level code review performed).*