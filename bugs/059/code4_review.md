# bchat Durable Execution — Adversarial Review of code4.md (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** `bugs/059/code4.md` (Plan 4, "Ready for Implementation") verified against plan6.md (APPROVED baseline) and the actual codebase (schema, exported API surface, driver capabilities).
**Method:** Read the full plan; verified every fix's claims against the code rather than trusting plan pseudocode — column existence, struct field types, exported fields, private-method reusability, handler signatures.

---

## 0. Tree Status (verified)

code4.md (mtime 08:55) was written after the last source edit (store 07:09–07:13, agent engine 07:27–07:33). The working tree is byte-identical to the implementation reviewed in `code_review.md`. **No code4.md change is present in the code** — as before, this is a plan-approval gate, not a post-implementation review. All `code_review.md` findings remain live; nothing is green until this plan is executed.

---

## 1. What code4.md Gets Right (verified correct)

- **C3-1 resolved:** stop-signal branch now calls `s.store.StopSkillExecution(ctx, exec.ID)` **before** returning `errStopSignal`, and `runDetachedExecution` handles the sentinel separately. The "never writes `stopped`" defect is closed.
- **C3-3 resolved:** `CompleteSkillExecution` / `FailSkillExecution` / `StopSkillExecution` are real driver methods with conditional `WHERE` clauses and `RowsAffected()==0 → no-op`. Verified implementable: the store `Driver` interface (`store/driver.go`) and `Store` wrapper (`store/agent.go`) pattern supports adding these cleanly.
- **C3-4 resolved:** `GenerateFn` signature gains `tenantID *int32`, and `svc.requireLLMConfig(ctx, int(*tenantID))` runs inside the callback. `requireLLMConfig` exists at `service.go:1759`; `newOpenRouterClient` at `service.go:58`.
- **Fix 9 implementable as written:** `APIV1Service.Profile` is exported (`v1.go:49`) and `Service.profile` exists (`service.go:72`) — `profile.Driver != "mysql"` gating is valid, unlike the previous env-var approach.
- **Fix 2 sound:** sqlite/postgres already implement a private, tenant-filtered, status-filtered `listSkillExecutions` (`store/db/sqlite/agent_skill.go:233`, `store/db/postgres/agent_skill.go:221`); exposing it as `ListSkillExecutions(ctx, find, limit)` is mechanical.
- C3-2 (JSON-parse output contract), C3-5…C3-9 (nits) are all correctly folded in.

---

## 2. Findings

### HIGH — must resolve before/during implementation

#### K-1 — `FailSkillExecution` SQL targets a non-existent `error_message` column

- **File:** code4.md Fix 3 → `store/migration/{sqlite,postgres,cockroach,mysql}/.../00__add_skill_executions.sql`
- **Description:** The planned SQL is `UPDATE agent_skill_executions SET status='failed', error_message=?, completed_at=?, updated_at=? WHERE id=? AND status NOT IN ('stopped','completed')`. **`agent_skill_executions` has no `error_message` column** — the only `error_message` column is on `agent_skill_logs` (e.g. `store/migration/sqlite/0.36/00__add_skill_executions.sql:39`). Every fail path (`runDetachedExecution`, recovery deserialize-failure) would throw a SQL error at runtime, and the fail error would be silently dropped.
- **Decision required — pick one:**
  - **(a)** Add `error_message TEXT` to `agent_skill_executions` in all four versioned DDLs **and all four `LATEST.sql`**, plus a `db.QueryRow`-style read in `scanSkillExecutions`/`SkillExecution` struct. Cleanest fit for the atomic method.
  - **(b)** Drop `error_message` from the atomic SQL and persist the error inside `failed_nodes` JSON via a follow-up `UpdateSkillExecution` (preserves the MED-5 concern originally raised — do not reintroduce error storage in `checkpoint_data`).
- **Why it matters for the tests:** the P0 row "FailSkillExecution … 0-row no-op on stopped" does not exercise error persistence at all. Add a P0/P1 test asserting the error lands where the plan commits to landing it.
- **Verdict:** REWORK (small, but must be in the plan before implementation).

### NITS — fold in during implementation

- **K-2 — Fix 1 breaks the build as scoped.** The struct change to `time.Time` happens in `store/agent.go`, but `checkpoint.go:116,130-131` (`createExecution` sets `CreatedAt`/`UpdatedAt = time.Now().Unix()`) and `checkpoint.go:43` (`writeCheckpoint` sets `UpdatedAt = time.Now().Unix()`) are **not** in Fix 1's 10-file scope. After Fix 1 those assignments are `int64 → time.Time` and the package won't compile. Add `server/router/api/v1/agent/checkpoint.go` to Fix 1.
- **K-3 — `GenerateFn` signature ripple is not scoped to the chat path.** The new `GenerateFn(ctx, tenantID, prompt, vars)` requires `LLMHandler.Execute` to extract `tenantID` (from `params["tenant_id"]`) and pass it. The chat path's `toolCallingLoop` (`service.go`) calls `h.Execute(ctx, args, nil)` and never injects a tenant, so a chat-path `llm_call` will always error "tenant_id required" until D2 chat durability lands. Either thread `&config.TenantID` there too, or make the handler fall back to the default config when the param is absent. Plan only mentions `executeStep`.
- **K-4 — absent-node CEL semantics still hard-error.** `EvalConditionDynamic` declares every `graph.Nodes` key, but a node skipped by failed deps (`continue` in `executeWorkflow` writes nothing to `state`) leaves that declared var missing → missing-key eval error instead of a clean `false`. Write `{"output":""}` placeholders for skipped nodes, or make eval treat absent keys as nil/absent tolerantly. The doc examples all depend on prior nodes evaluating; this closes the edge.
- **K-5 — recovery leaks skill-less pending rows.** `if !graph.HasSkills { continue }` re-scans the row every tick forever; only the deserialize-failure path calls `FailSkillExecution`. Make both paths consistent (fail skill-less rows too).
- **K-6 — cosmetic casing.** Plan references `Latest.sql` (capital L); the four real files are `LATEST.sql`. Won't break Go but will trip an implementer/tool grepping for them.

---

## 3. Plan6 Conformance (post-fix, expected)

| plan6 requirement | Disposition | Assessment |
|-------------------|-------------|------------|
| D2 — chat-path durable loop | DEFERRED | Acknowledged, non-blocking; K-3 interaction noted |
| R3 — status guards | Fix 3 | OK (after K-1) |
| D4 — CEL env + output semantics | Fix 4 | OK (after K-4) |
| D5/R2 — recovery worker | Fix 7 | OK (after K-5) |
| D6 — outbound events | kept | OK |
| MaxRetries/Timeout honored | DEFERRED | Acknowledged gap |
| §10 cadence (30s ticker) | deviation | OK, documented |
| §11 endpoints + RBAC | kept + gating | OK |

**Unless K-1 is decided and K-2…K-5 are folded in, plan6 is not fully satisfied.**

---

## 4. Test-Plan Review

Strong expansion (P0 rows for round-trip, stop-race, tenant isolation, dynamic CEL, coercion, recovery, logs). Missing rows to add:

| Addition | Guard |
|----------|-------|
| `FailSkillExecution` error persists where the plan commits (column vs failed_nodes) | K-1 decision |
| CEL condition referencing a node skipped by un-met deps → clean false, no error | K-4 |
| Chat-path `llm_call` with tenant id resolves tenant model/key | K-3 |
| `checkpoint.go` build after Fix 1 (compile gate covers, but add create/writeCheckpoint unit assertion) | K-2 |

Also note: the plan's `store/db/postgres/agent_skill_test.go` is integration (`(integration tag)`) — the CI/skip story for it must be stated or the P0 "Postgres round-trip" row is vacuous on a local/no-PG run.

---

## 5. Implementation Order — Confirmed

The plan's ordering (Fix 1 → Fix 3 → Fix 2 → Fix 4 → Fix 5 → Fix 7 → Fix 6 → Fix 8-10 → tests) is correct: DDL/types first, atomic guards + store API second, tenant isolation third, CEL fourth, then the sentinel/cleanup work. Recommended insertion: resolve K-1 during Fix 1 (schema), apply K-2 within Fix 1, apply K-3 within Fix 6, K-4 within Fix 4, K-5 within Fix 7.

---

## 6. Bottom Line / Sign-off

- **Tree:** code4.md is un-implemented; all `code_review.md` findings remain live.
- **Plan:** the four blocking defects from code3_review (C3-1…C3-4) are correctly resolved, and the plan is verified implementable against the actual code. This is the first plan in the series that can be approved.
- **Gate condition:** **APPROVED with nits & one rework item** — decide K-1 (add `error_message` to `agent_skill_executions` or persist in `failed_nodes`) and fold K-2…K-5 into the corresponding fixes. With those, ship it.