# Adversarial Plan Review — bugs/059 Plan 5.0 (plan5.md)

**Reviewer posture:** Senior Go architect, database/automation expert, RAG + agent-framework designer.
**Reviewed artifact:** [plan5.md](./plan5.md) — Durable Execution Architecture, v5.0.
**Review level:** **Implementation plan** — architecture, state machines, seams, ownership, sequencing.
  NOT a line-level code review. Pseudocode is judged only where it exposes a design gap, never for field/type exactness.
**Codebase baseline:** bchat @ current HEAD — `server/router/api/v1/agent/*.go`, `store/*.go`, `store/db/*.go`, `store/migration/*`, `server/router/api/v1/v1.go`.
**Prior-art lineage read:** `plan.md` (REWORK), `plan2.md` (APPROVED+nits), `plan3.md`, `plan_review_deepseek.md` (D1–D9), `plan4.md`, `plan4_review.md` (R1–R6), `state_machine.md`.

---

## Verdict: **REWORK — targeted (smaller than plan4's carve list)** 🔴

plan5 is the most implementable version of the architecture so far. R2, R5, R6, D6, and D9 are now genuinely closed against verified seams, and R4's core (name key + line tracking) is correct. The plan's **"All review findings (R1–R6) are addressed against real codebase seams"** claim, however, is still false on five points — and two of them (R1, and an R4 sub-seam) repeat the exact failure mode plan4 was reworked for: **referencing an API that does not exist.**

None of these require re-architecture. Each is a one-slot correction whose blast radius is mostly contained. Hence "targeted rework," not redesign.

---

## ✅ What plan5 gets genuinely right (verified)

| # | Decision | Why it's correct |
|---|----------|------------------|
| R6 | RBAC uses `h.hasPermission(c, tenant.ID, PermXxx)` + `h.isAdmin(c)` | All constants exist (`permissions.go:12-19`: `PermTenantAdmin`, `PermTenantRead`, `PermChatTest`, `PermChatLogs`, `PermAPIConfig`); real signature `hasPermission(c echo.Context, tenantID int32, permission string) bool` (handlers.go:2391) + `isAdmin` (handlers.go:2311). This is the correct seam. |
| R5 | CockroachDB is versioned, not single-file | Verified: `store/migration/cockroach/0.35/00__tickets_add_internal_notes.sql` + `LATEST.sql`. The 4 versioned dirs + 4 × LATEST.sql + `task validate:parity` statement is now factual. |
| R2 | `trigger_path` + `!= 'chat'` claim predicate | Soundly removes the double-owner race: chat rows carry `'chat'` and are never claimable; the claim SQL and `ListPendingSkillExecutions` both exclude them. |
| R4 core | Skill name read from `params["code"]`; `LineStart` via byte offsets | Matches real `parseParams` behavior (parser.go:172-200) — first positional value is stored under `"code"` (parser.go:195). |
| D6 | Outbound events reuse `dispatchEvent` | Signature exactly matches `dispatchEvent(ctx, tenantID int32, leadID string, eventType string, data string)` (service.go:5422). |
| D9 | `LoadConfig`/`configCache`/`InvalidateConfigCache` | Verified real path: `LoadConfig` (service.go:1845), `ConfigCache.Get/Set/Invalidate` (service.go:1587-1688), `HandleImportScript` → `InvalidateConfigCache` (handlers.go:4129). |
| D5 | Claim/lease schema + atomic claim SQL | Single-statement lease claim is correct; aligns with the `agent_events` pre-claim philosophy. |

The seam map below is therefore narrower than plan4's — most of the real seams are finally sewn.

---

## 🔴 Design-level gaps that survive — must rework

### R1 — `cel.Vars` does not exist in cel-go v0.25.0 either
plan5 replaced `env.Eval(ast)` with `prg.Eval(cel.Vars(vars))` (§8.1:939). Verified in the **pinned cel-go v0.25.0** (`/home/chaschel/go/pkg/mod/github.com/google/cel-go@v0.25.0/cel/`):

- The only activation constructors are **`NoVars()`** (program.go:82) and **`NewActivation(bindings)`** (program.go:67). **There is no `Vars(...)` function anywhere in the module.**
- `Program.Eval(any)` (program.go:44) takes a `map[string]any` or `Activation` **directly** — every cel_test calls `prg.Eval(vars)`, never `prg.Eval(cel.Vars(...))`.

> **Close R1:** eval as `prg.Eval(vars)` (the map built at §8.1:931-936) — or `prg.Eval(cel.NewActivation(vars))` if an explicit activation is wanted. plan5 traded one nonexistent call for another; the *movement* (Env.Compile → Env.Program) is right, only the eval argument is wrong. This is the single most important item on this list because the fix is advertised, not incidental.

### D4 — node declarations derived from checkpoint *keys* contradict the prerequisite policy
`evaluateCondition` (§8.1:919-923) builds `nodeNames` from `checkpointData` keys and declares only those as `cel.Variable`. When a condition references a node that **hasn't run yet** — the plan's own prerequisite case, e.g. `search_kb.found == false` evaluated before `search_kb` completes — `search_kb` is **undeclared**, so `env.Compile` fails with `undeclared reference to 'search_kb'` at **compile** time, not evaluation. Consequences:

- §8.2's documented policy ("not-yet-evaluable → don't fire, re-evaluate later") is unreachable for exactly the input it describes.
- plan5's own `TestCELConditionIncompleteNode` (§15.1:1252-1260) asserts `assert.NoError(t, err)` + `assert.False(t, result)` on exactly that input — the implementation returns a compile error, so the test fails as written.
- The fail-closed table (§8.3) assumes "evaluation error," but the failure here happens before any evaluation.

> **Close D4:** declare **all** node names at env build from the `SkillGraph` (resolve `checkpointData` values against the graph's nodes, not against present keys only), so the expression always compiles and missing-node semantics stay in the evaluator/prerequisite gate where §8.2 puts them. Align the unit test with the fixed semantics.

### D1/R4 — `ParsedScript.AnnotationBlocks` does not exist
§3.2 `ParseScriptWithSkills` iterates `parsed.AnnotationBlocks` (plan5.md:254). Verified: `ParsedScript` (parser.go:975-981) exposes **only `Summary`, `Sections`, `RawContent`** — no `AnnotationBlocks` field. The blocks the plan needs are produced by the package-level `extractAnnotationBlocks(content)` (parser.go:100; already consumed at parser.go:219 and :406). The plan's `LineStart`/`params["code"]` logic is right; it just reads from a struct field the parser never populates.

> **Close D1/R4:** iterate the result of the existing package-level `extractAnnotationBlocks(content)` inside `ParseScriptWithSkills` (or expose blocks on `ParsedScript`), not `parsed.AnnotationBlocks`.

### R3 — the status re-read guard is applied *after* the unconditional write
§7.3's flow: `executeSkill` (§11:1107-1113) calls `UpdateSkillExecution(ctx, exec)` **immediately and unconditionally** on every node; `checkpointWithStatusReRead` (§7.4) runs only *afterwards* as the loop's checkpoint step. An execution stopped mid-loop therefore gets: stop flips row → `stopped` (accepted), then `executeSkill` writes the row back out of the in-memory struct — **overwriting status to `running`** before the re-read ever gets a chance to abort. The guard in §7.4 never precedes the mutation it is meant to protect.

> **Close R3:** re-read the row status at the **top of `executeSkill`** (or route every skill write through the guarded checkpoint path), so no write lands on a `stopped` row. The current placement guarantees the race happens first.

### I1 — the detached path can never pass the tenant check
§11 `executeSkill` extracts the tenant via `getTenantFromContext(ctx)` (plan → tenant_helpers.go:17), which reads `c.Get("tenant-id")` from an **echo.Context**. The detached worker (§10.2:1074-1078) executes from a **plain `context.WithCancel(context.Background())`** — no echo context, thus `getTenantFromContext` returns nil and every detached skill returns `"tenant ID required"`. N7's "tenant scoping on all executor methods" silently fails for the entire event/API/cron surface — which is half the hackathon story.

> **Close I1:** seed tenant into the execution context from the **claimed row** (`exec.TenantID`) before running the detached pipeline (e.g., `context.WithValue(ctx, tenantContextKey, *claimed.TenantID)` or thread it explicitly through the executor), and split the tenant extraction so the detached path doesn't depend on an echo request.

---

## 🔸 Minor seam mismatches & nits (lower priority)

1. **`h.getTenant(c, slug)` (§12)** — does not exist. Real helper is `getTenantOrFail(ctx, h.store, c)` (handlers.go:66, 120, 228, …). Same class of error as `parsed.AnnotationBlocks` — a named-but-nonexistent seam.
2. **`s.ragEnabled` (§10.2)** — not a field on `Service`. Real API: `s.IsRAGEnabled()` (service.go:481).
3. **`LLMHandler` Content (§4.3)** — `openrouter.ChatCompletionMessage.Content` is the `openrouter.Content` struct (chat.go:527), not a plain string; assigning `buildSkillPrompt(...)` raw won't compile. Residual of N6.
4. **Chat path status shortcut** — §7.3 creates the row at `Status: "running"` (skipping `created`/`pending`). Harmless, but the 6-state machine is then only partially exercised; document which states the chat path actually uses.
5. **Dead `stopped` check (§10.2:1065)** — the claim SQL already excludes `stopped` rows (`status='pending' OR (running+expired)`), so the skip is unreachable; harmless.
6. **`activeExecutions` re-registration window** — §7.5 stop cancels via the map; for a detached row the cancel func is registered *after* claim (good), but ensure a stop landing between claim and registration is caught by the status re-read (ties into R3 — make the re-read unconditional at write time).

---

## 🗺️ Seam map — where plan5 still misattaches (only the wrong seams)

| New subsystem (plan5 §) | Correct real seam | Fix |
|---|---|---|
| CEL eval (§8.1) | cel-go v0.25.0 `Env.Compile` → `Env.Program` → `prg.Eval(vars)` | R1 (swap `cel.Vars` → direct map) |
| CEL env construction (§8.1) | `SkillGraph.Nodes` names, not checkpoint keys | D4 |
| Parser extension (§3.2) | package-level `extractAnnotationBlocks(content)` (parser.go:100) | D1/R4 |
| Skill writes (§11 / §7.4) | re-read status before every write, not after | R3 |
| Detached tenancy (§10/§11) | tenant from claimed row (`exec.TenantID`), not echo ctx | I1 |
| Handler tenant resolution (§12) | `getTenantOrFail(ctx, h.store, c)` | Minor 1 |
| Recovery gating (§10.2) | `s.IsRAGEnabled()` (service.go:481) | Minor 2 |

All other seams (dispatchEvent, configCache invalidation, RBAC, migrations, claim/lease SQL, trigger_path filtering) are verified correct as written.

---

## 🧭 Recommendation

Do **one** targeted correction pass on the five primary items (R1, D4, D1/R4, R3, I1) plus the three minor seam fixes, then re-verify. R1 is the gate...

Wait — **R1 is not actually a gate blocker for the architecture; it is a one-line API fix.** What makes this review's verdict "rework" rather than "approve with nits" is the *combination* of five seam-attachment errors in a plan that explicitly advertises every R1–R6 as closed. The risk is not that any single item is hard — it is that they cluster in exactly the places the plan claims are done, which means Phase 1 would fail at first compile/eval rather than later. Fix the five, and this plan earns APPROVED.

---

## ✅ Bottom line

**plan5 is not "ready for implementation."** It is ready for a **targeted rework of R1, D4, D1/R4, R3, I1** (+ three minor seam fixes), after which it earns **APPROVED WITH NITS**. R2, R5, R6, D5, D6, D9 are verified good-to-build as-is. The architecture is correct; the claim that all seams are real is what forces the rework verdict.

---

*Review completed: 2026-08-05 (plan-level, per author calibration — no line-level code review performed).*