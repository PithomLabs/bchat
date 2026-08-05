# Adversarial Review of coverage.md — bugs/059 Durable Skill Execution

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-06
**Scope:** `bugs/059/coverage.md` (coverage assessment written 08-05 12:20) verified against the live source tree, the plan gatechain (plan.md … plan6.md), the review chain (code_review … code10_review), and the deferred-item registries recorded in code6_imp_review.md, code9.md §8, and code10_review.md.
**Method:** Read-only verification. Every claimed implementation point was checked against the actual tree (grep/read, exact line numbers), every deferred item was cross-referenced against the review chain to test registry completeness, and the coverage's arithmetic was recomputed. No source edits applied; this is the review gate.

---

## 1. Verdict

**coverage.md is substantially accurate on the core.** The per-scope evidence lines I spot-verified (parser extraction, CompileError/errors.As, same-worker carve-out, stop guards, `workflow.completed` dispatch, recovery gating, max_retries inertness, F-1/F-2/F-3 open, rounds 7-8 as pure plan gates) all check out. **The document has three structural defects** (state-machine scoring baseline, the headline percentage's hidden arithmetic, and a mischaracterized D2), **five registry gaps** (items tracked in the review chain that never made it into the 16-item registry, plus one fabricated citation), and several minor accuracy nits. None of the defects invalidate the conclusion that the core engine is implemented and green — but two of them change the headline number (65% vs 76%) and one changes a phase score (state machine 60% → ~83%).

---

## 2. Confirmed Accurate (spot-verified in tree)

| coverage.md claim | Verification |
|-------------------|--------------|
| Parser: `@skill`/`@trigger`/`@signal`, `LineStart`, SkillGraph, DAG validation | parser.go:112-140 (TriggerDefinition/StopSignalDefinition/SkillGraph/ValidationError), :1206-1297 (ParseScriptWithSkills, Validate, detectCycle) |
| CEL with real cel-go, CompileError, tolerant eval, normalizeNumbers, map output contract | evaluator.go:73-83 (CompileError), :89-153 (EvalConditionDynamic, isMissingKeyError); execution.go:375-412 (buildNodeOutput/normalizeNumbers) |
| errors.As compile → hard-fail | execution.go:256-265 |
| Same-worker carve-out in both drivers | sqlite: `OR claimed_by = ?`; postgres: `OR claimed_by = $6` (verified binding order) |
| Stop sentinel + status re-read + ctx cancellation | execution.go:109-121 (re-read, terminal-guard on fail), :226-229 (StopSkillExecution before errStopSignal) |
| `workflow.completed` dispatch | checkpoint.go:65 |
| Recovery worker 30s ticker, SKILL_RECOVERY_ENABLED + IsRAGEnabled gating | recovery.go:13-35 |
| Tenant scoping `*int32`, tenant-filtered list | checkpoint.go:115-122; store/db/sqlite/agent_skill.go tenant WHERE |
| Endpoints RBAC-gated, mysql-gated | v1.go:352-357 (`if s.Profile.Driver != "mysql"`); handlers.go PermWorkflowStart/PermTenantRead + tenant-ownership checks |
| max_retries column only, 0% driving retries | RetryCount never incremented; StartDetachedExecution hardcodes `createExecution(..., 3)` (execution.go:28) |
| Only SKILL_RECOVERY_ENABLED env var added | grep: single SKILL_ env var in tree |
| F-1 / F-2 / F-3 still open | evaluator_test.go has no CompileError case; no different-worker-exclusivity assertion; execution.go:217-222 still log-and-skip |
| Rounds 7-8 were plan gates, no source changes | code7_review.md ("No source edits applied"), code8_review.md ("code8.md has NOT been implemented") |
| §9 metrics per plan.md (99.9%, 5-iter, 100%, <2s, 100+) | plan.md:660-674 confirmed |

---

## 3. Structural Findings

### F-1 (HIGH) — State-machine scoring measures a superseded baseline; the real gap is `created`, not `suspended`

coverage.md §2 scores "State machine (6 states) | 60% | 5 statuses implemented …; `suspended` + retry-by-clone **absent**" and carries a deferred item "`suspended` state + retry-by-clone resume | MED".

- plan6.md §6 — the document coverage.md itself declares the **final baseline** — defines the 6 states as `created → pending → running → completed/failed/stopped`. **`suspended` is not among them.**
- `suspended` + retry-by-clone originate in plan.md/plan2.md and were dropped as early as plan4.md:671-695 (transitions table: created/pending/running/completed/failed/stopped). plan4 still had a `/suspend` endpoint row (:1123), but plan6 §11 (the approved baseline) ships only `start` and `stop`.
- The genuine gap vs plan6 is `created`: it is never written anywhere — `createExecution` (checkpoint.go:94) sets `Status: "pending"` directly; grep for a `"created"` status write across the agent/store code returns nothing. Implementation fuses `created`→`pending`.

**Corrected view:** vs the declared baseline the machine implements 5 of 6 states (~83%), with `created` fused into `pending` (a defensible by-design fusion that plan6's chat-path note already implies). Penalizing 60% for a state that the final baseline deliberately removed overstates the gap and pollutes the deferred registry with a dead-plan artifact. Either re-scope the item to "`created` never materialized (by-design fusion)" or drop it and mark the 6-state machine ~83% with a documented deviation.

### F-2 (HIGH) — The headline "~72-78%" is arithmetic-dependent on silently dropping §9 metrics

Recomputing coverage's own numbers, scope-count-weighted:

- **Including §9:** (95·5 + 90·5 + 65·8 + 60·5 + 15·5)/28 = **65.0%**
- **Excluding §9:** (95·5 + 90·5 + 65·8 + 60·5)/23 = **75.9%**
- Equal-weight per phase, including §9: (95+90+65+60+15)/5 = **65.0%**; excluding: **77.5%**

The 72-78% band only exists if §9 is excluded — but §3's "Overall Completion" block lists §9 as part of scope, and §5's "remaining ~25%" likewise only reconciles with the 76% figure. The document never states its weighting. Additionally, §9 metrics are quoted from **plan.md**, which received a REWORK verdict; the final baseline (plan6) carries no §9. The mixed-baseline choice is declared in the header but never justified — why does plan.md's §9 survive its own REWORK while nothing else from plan.md is normative?

**Required correction:** state the method ("weighted by scope count, §9 metrics excluded as not-yet-measured" ⇒ ~76%) or include §9 honestly (⇒ ~65%, remaining ~35%). Either way the headline and the "remaining ~25%" sentence must match the stated method.

### F-3 (MED) — Bottom-line contradictions with the tables

§5 claims the three rounds "land **all of Phase 1**" while the §2 table gives P1 = **95%** (migrations 90%, store 90% — both flagged as mysql stubs). Same paragraph: "the **complete** test skeleton" vs P4 = 60%. And "Not started work … the remaining ~25%" vs §3's own inclusive math (~35%). These are the classic sign of headline drift; the numbers in §2 are the defensible ones.

### F-4 (MED) — D2 is mischaracterized: the chat path does not "execute the DAG synchronously"

Registry row: "D2 | Chat-path **durable** loop (chat executes DAG synchronously in toolCallingLoop instead) | HIGH".

The chat path executes **no DAG at all**. `toolCallingLoop` (service.go:3600-3680) resolves each LLM-chosen tool to a single handler and runs it ad hoc with `args` + `vars = {tenant_id}` — no topological order, no `depends_on`, no per-node conditions, no stop-signal checks, no checkpoints, no audit logs. What coverage.md describes as "synchronous DAG execution" is flat LLM-driven tool calling.

Compounding: the system prompt Section 7B (service.go:3436-3443) tells the model *"To trigger a workflow, call the appropriate tool. **The system will execute the workflow steps automatically.**"* — which is false. Calling a skill tool executes only that one handler; nothing downstream fires. A tenant scripting a multi-step DAG in SCRIPT.md and using chat will believe the full workflow ran when only a single node executed. The D2 row understates both the mechanism and the misleading-prompt risk.

### F-5 (HIGH) — Everything is uncommitted; "In Tree? yes" means "uncommitted working tree"

`git status`: all skill files (`execution.go`, `checkpoint.go`, `evaluator.go`, `recovery.go`, `skill*.go`, both store drivers, migrations, tests) are untracked (`??`) or modified (` M`) on top of commit `635668f` (🐛 058). coverage.md's §5 "implemented and verified in the source tree" omits the durability caveat: the entire 10-round, ~72-76% deliverable is one `git checkout .` / `git clean -fd` away from zero. This is not a scope-coverage question, but a "what has been done and still pending" document that cannot be read as a stable inventory while the work is uncommitted. A "Tree state: UNCOMMITTED" banner belongs at the top of the assessment.

---

## 4. Registry Gaps (tracked in the review chain, missing from coverage's 16-item registry)

### G-1 (MED) — Per-node `timeout` parsed but never enforced; silently dropped from the registry

coverage.md's MED-3 row is "Retry/backoff; `max_retries` honored (column only)". code9.md §8's own deferred table listed **"MaxRetries/Timeout honored — retry_count not driving retry"** as a single row. The Timeout half vanished in coverage.md:
- parser.go:105, :1228 parses `timeout: "30s"` into `SkillDefinition.Timeout`.
- execution.go:290 calls `h.Execute(ctx, node.Params, celVars)` with the workflow ctx — **no per-node deadline is applied anywhere** (grep: Timeout read only in parser).
- plan6's §3.1 example annotations all carry timeouts; a hung `sleep`/LLM handler runs unbounded within a 300s lease.

Restore "Timeout honored" to the registry (MED, shares the retry row).

### G-2 (MED-LOW) — Wasted first LLM call on every chat message with skills configured

Flagged in code_review.md round 1 ("makes a first LLM call without tools, then another with them (wasted round trip)") and still live: service.go:3088-3092 makes the primary completion call **without** tools, then, if a skill graph exists, `toolCallingLoop` (service.go:3097-3106) makes a **second** call **with** tools and discards the first response. Every chat message against a skill-bearing tenant costs a doubled LLM round trip (latency + tokens). Not in the registry.

### G-3 (MED) — `graph.Trigger` is dead config; plan6's "event-triggered runs" do not exist

parser.go:1240 populates `graph.Trigger` (Type/EventType). Nothing in the tree reads it (grep: the only other `.Trigger` refs are unrelated audit/behavior triggers). The only start path is the API (`HandleStartWorkflow` → `StartDetachedExecution`, handlers.go:6770), and `req.Trigger` is just a free-form `trigger_path` string. plan6 §1 architecture promises the detached worker owns "event-triggered … runs"; no inbound event→workflow wiring exists. The registry's "plan2 §11-13" row covers timer/scheduler annotations but not plan6's event-triggered start. Add: "Event-triggered start unimplemented; `@trigger` annotation parsed but unconsumed | MED".

### G-4 (LOW-MED) — Lease-renewal hazard under-specified; "dormant" ≠ harmless

Registry INFO row: "Same-worker carve-out dormant in prod (per-run UUID workerID, 300s lease) — accepted". The original hazard from code_review.md round 1 and code9_imp_review.md ("lease-heartbeat capability" recommendation) is: **a step running longer than the 300s lease lets the recovery worker re-claim the execution and re-run side-effecting handlers mid-step** (double execution of e.g. create_ticket). The carve-out only mitigates *same-worker* re-claims; a crashed worker's run is re-claimed by a *different* worker at lease expiry regardless. coverage.md's framing ("dormant … harmless … strictly additive", lifted from code10_review INFO) loses the actual risk statement. Reframe the INFO row to carry the hazard, or the registry implies a mitigation that doesn't exist.

### G-5 (LOW) — Cadence row cites a fabricated "15s"

"INFO | 30s recovery ticker vs plan's 15s cadence". No plan, code, or review anywhere specifies 15s. plan6 §10 = 10s initial sleep + 60s poll; the deviation was documented in every round as "30s ticker vs (10s sleep + 60s poll)" (code2_review.md:115, code3.md:529, code4.md:575). The accepted-deviation conclusion survives; the reference number does not. Correct to "(10s sleep + 60s poll)" or cite plan6 §10 verbatim.

---

## 5. Accuracy Nits (do not change the verdict)

| # | Item | Detail |
|---|------|--------|
| N-1 | evaluator test count | coverage says "evaluator 9". `evaluator_test.go` has **20** test funcs (11 EvalCondition + 3 EvalConditionDynamic + 6 BuildNodeOutput). "9" is only the code9.md additions; as a count of existing coverage it understates 2.2×. |
| N-2 | v1.go route range | coverage cites "v1.go:354-356 (executions/stop/list/get)". The block is **353-357** and includes `POST /:slug/workflows/start` at :353 — the primary start route is dropped from the citation. |
| N-3 | parser.go citation | "parser.go ~108-134, ~1206 ValidationError" — `ValidationError` is at parser.go:134; :1206 is `ParseScriptWithSkills`. Garbled line ref. |
| N-4 | Mock-LLM integration 50% | The evidence coverage itself cites ("builtins + store round-trip only; no full chat-skill E2E") does not support 50%. No mock-LLM chat test exists anywhere (the httptest hits in the agent suite are bridge-telemetry tests, unrelated). Realistic: 15-25%. |
| N-5 | Simulation extension 40% | Zero skill-specific changes in simulation.go (grep: no skill refs). The 40% is credit for *pre-existing* endpoints. Defensible only if relabeled "pre-existing capability, not extended"; 0-10% otherwise. |
| N-6 | Interface drift vs plan6 | plan6 §5.4 declares `ClaimSkillExecution(ctx, id, workerID, leaseDuration time.Duration)`; the tree ships `leaseSeconds int` (store/driver.go:307). Cosmetic, but it means plan6 was not followed verbatim — worth a note if plan-conformance is claimed. |
| N-7 | MySQL migration versioning | Evidence line lists only {sqlite,postgres,cockroach}/0.36 and parenthetically "mysql stub". The MySQL **migration exists** — `store/migration/mysql/0.26/00__add_skill_executions.sql` + LATEST.sql:145-189. The "stub" is the driver, not the migration. Note: a MySQL deployment creates the tables, then every skill call errors — acceptable only because routes are gated (v1.go:352), which coverage does not mention in this row. |
| N-8 | `created` vs `pending` semantics in store DDL | Deferred row F-1 covers this; N-8 just flags that plan6 §5's schema has no `created` status column either (default `'pending'`), further evidence the 60% is a plan.md/plan2 artifact. |

---

## 6. Registry Corrections (delta)

| Action | Item | Severity |
|--------|------|----------|
| ADD | Per-node `timeout` parsed, never enforced (restore code9 §8 row) | MED |
| ADD | Chat-path double LLM call (no-tools call discarded before tool loop) | MED-LOW |
| ADD | `@trigger` dead config / event-triggered start unimplemented | MED |
| REFRAME | Lease row: state the >300s-step double-claim hazard, not just carve-out dormancy | LOW-MED |
| REFRAME | `suspended`+retry-by-clone → `created`-fused-by-design (or drop; superseded in plan4) | MED |
| FIX | Cadence row: "plan's 15s cadence" → plan6's "(10s sleep + 60s poll)" | LOW |
| ADD | Tree state: UNCOMMITTED (all of bugs/059 on top of 635668f) | HIGH |

Net registry: 16 → **19** items (+3, 2 reframed, 1 corrected).

---

## 7. Corrected Numbers

```
P1 Core Infra     95%   (unchanged)
P2 Engine         90%   (state machine: 5/6 states vs plan6 ≈ 83% ⇒ phase ≈ 90% — unchanged net,
                        with the suspended-vs-created baseline error removed)
P3 Integration    65%   (unchanged; D2 wording corrected)
P4 Testing        60%   (mock-LLM 50% → ~20%, simulation 40% → ~10% ⇒ phase ≈ 45-55%)
§9 Metrics        ~15%  (unchanged; labelled plan.md-sourced, not final-baseline)

Weighted overall: ~65% (incl. §9) / ~76% (excl. §9, method-stated)
```

The doc's own conclusion — core runtime complete and green; shortfall concentrated in deferred glue, breadth, and validation — **survives** the review. What does not survive is the unstated weighting, the plan2-era state-machine penalty, and the registry's four omissions.

---

## 8. Bottom Line / Sign-off

**Verdict: APPROVED WITH REQUIRED REVISIONS (non-blocking for the code, blocking for the assessment).** coverage.md is a credible assessment of a genuinely ~2/3-3/4-complete, green-tested feature. It must, however: (1) re-score the state machine against plan6 (§ F-1) or explicitly mark the plan.md/plan2 baseline debt; (2) state the weighting and reconcile 65% vs 76% vs "remaining ~25%" (§ F-2/F-3); (3) restore the four registry items (§ G-1…G-4) and correct the cadence citation (§ G-5); (4) add the uncommitted-tree banner (§ F-5). The corrected bottom line: **~65% of full scope (incl. metrics) / ~76% of engineering scope**, with the honest largest open item being D2 — and the honest largest *risk* item being that none of it is committed yet.
