# bchat Durable Execution — Adversarial Review of code2.md Fix Plan (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** `bugs/059/code2.md` (Code Review Fix Plan) verified against the working tree and plan6.md (APPROVED baseline).
**Method:** Read every planned fix; verified the current source state at the file/line level; assessed each fix for correctness, atomicity, tenant isolation, and plan6 conformance.

---

## 0. Blocking Finding: code2.md is NOT implemented

The code2.md plan (mtime 08:22) was written **after** the last edit of every source file (07:09–07:33). The working tree is byte-identical to the implementation reviewed in `code_review.md`. **None of the 11 planned fixes exist in the code.**

| code2.md fix | Tree status | Evidence |
|--------------|-------------|----------|
| CRITICAL-1 (time.Time) | NOT APPLIED | `store/agent.go:1361-1366` still `*int64`/`int64` |
| CRITICAL-1b (PG dialect) | NOT APPLIED | `store/migration/postgres/0.36/00__add_skill_executions.sql` still `STRING`/`INT8`/`UUID` |
| CRITICAL-2 (tenant-scoped list) | NOT APPLIED | no `ListSkillExecutions` in `store/driver.go`; `checkpoint.go:187-208` untouched |
| CRITICAL-3 (terminal guards) | NOT APPLIED | `failExecution`/`completeExecution` unconditional (`checkpoint.go:49-107`) |
| HIGH-1 (dynamic CEL / D4) | NOT APPLIED | `evaluator.go:18-27` still `standardCELVars` only; no `EvalConditionDynamic` |
| HIGH-5 (stop eval) | NOT APPLIED | `executeWorkflow` never reads `graph.Stop` (`execution.go:107-188`) |
| HIGH-3 (wire GenerateFn) | NOT APPLIED | `skill_builtins.go:19` registers bare `&LLMHandler{}`; no wiring in `service.go` |
| MED-1, MED-2, MED-5, MED-6, LOW-1, LOW-3, LOW-4 | NOT APPLIED | files match pre-fix state |

`go build ./store/... ./server/router/api/v1/agent/...` clean; unit tests pass — but this only proves the *pre-fix* code compiles. **Every Critical/High/Medium finding in `code_review.md` remains open in the tree.**

Consequence: this document reviews **the code2.md plan itself** (the only new artifact). Until it is implemented, there is nothing new to sign off as green.

---

## 1. Plan-Level Findings (code2.md)

### CRITICAL (in the plan as written)

#### C-1 — CRITICAL-1/1b fix omits the four `LATEST.sql` files

- **File:** code2.md Fix 1 → `store/migration/{postgres,cockroach,mysql,sqlite}/LATEST.sql`
- **Description:** Fix 1 rewrites only the versioned `0.36/00__add_skill_executions.sql` files. All four `LATEST.sql` files still carry the broken schema: postgres/cockroach LATEST retains CockroachDB dialect (`STRING`) and `TIMESTAMPTZ` columns that the `int64` Go struct can't read/write. Fresh DBs materialize from `LATEST.sql`, not from versioned files → new deployments still break in exactly the way CRITICAL-1/1b describe.
- **Fix:** Include all four `LATEST.sql` rewrites in Fix 1 and add a `validate:parity` step to the checklist.

#### C-2 — CRITICAL-3 fix is non-atomic and runs on a cancelled context

- **File:** code2.md Fix 3 (proposed `failExecution`/`completeExecution` re-read + `runDetachedExecution` guard)
- **Description:** Two defects:
  1. Re-read-then-write is a TOCTOU window: a concurrent stop between the `GetSkillExecution` and `UpdateSkillExecution` still lands `failed` on top of `stopped`.
  2. The proposed `runDetachedExecution` guard does `GetSkillExecution(ctx, ...)` with the **already-cancelled** `ctx` (`execution.go:97-100` calls it after `executeWorkflow` returns `ctx.Err()`). The read fails, `current == nil`, and the code falls through to `failExecution`, which still clobbers `stopped` → `failed`. The fix as written does not deliver the `stopped` terminal state it claims.
- **Fix:** Use atomic conditional writes:
  - `UPDATE ... SET status='failed' WHERE id=$1 AND status NOT IN ('stopped','completed')`
  - `UPDATE ... SET status='completed' WHERE id=$1 AND status='running'`
  - and perform any post-cancel re-read with a **fresh** context (`context.WithoutCancel(ctx)` or `context.Background()`).
- **Verdict:** REWORK.

### HIGH

#### H-1 — HIGH-1 fix only implements half of plan6 D4; documented conditions still break

- **File:** code2.md Fix 4 (proposed `EvalConditionDynamic`)
- **Description:** Two fatal gaps remain even after declaring node vars as `cel.DynType`:
  1. **String outputs can't be field-accessed.** Node outputs are plain strings (`state[nodeName] = output`, `execution.go:169`). The plan's own examples `search_kb.found == false` and `create_ticket.ticket_id != ''` do field selection **on a string** — a runtime "no such field" error even when the variable is declared. Fixing `search_kb` absence is useless if its value can't carry `.found`.
  2. **Int-typed standard vars vs JSON float64.** `standardCELVars` declares `urgency`/`tenant_id` as `cel.IntType` (`evaluator.go:21,23`). `initial_vars` arrive from the API as JSON → `float64` in Go. `condition: "urgency > 3"` on an API-triggered run fails eval (`prg.Eval` type error) → whole workflow fails.
- **Fix:** (a) Persist structured node outputs (map/summary objects) or project `state` into CEL-accessible values; (b) declare standard numeric vars as `cel.DynType` and normalize `float64`/`int` in the vars map (`int(v.(float64))` for integral values).
- **Verdict:** REWORK.

#### H-2 — HIGH-3 fix binds the LLM callback to tenant 0

- **File:** code2.md Fix 6 (proposed `GenerateFn` wiring)
- **Description:** The wiring calls `svc.requireLLMConfig(ctx, 0)` with tenant 0. The LLMHandler is registered once in `NewService` with no per-execution tenant context, so every `llm:`/`llm_call` step executes against the **global default engine** — wrong model and wrong API key in a multi-tenant platform. Additionally it issues a single `SystemMessage(prompt)`, discarding conversation context.
- **Fix:** Thread tenant/model into the handler at call time — e.g. store the resolved engine config in `node.Params` at execution start, or create the LLMHandler per-execution with `exec.TenantID`'s engine config. Do not hardcode tenant 0.
- **Verdict:** REWORK.

#### H-3 — HIGH-5 stop-condition eval is sound but placement silently mislogs

- **File:** code2.md Fix 5
- **Description:** Evaluating `graph.Stop.Condition` after each checkpointed step is the right seam. But the snippet `return nil` on a matched stop makes `runDetachedExecution` log `"workflow execution finished"` (`execution.go:103`) for a run that was stopped, and no `completed` event fires (correct) while the stop `emit_event` fires (also correct). Distinguish completion vs stop in the log/outcome. Minor but worth fixing in the same change.
- **Fix:** Return a sentinel (`errStopSignal`) and log `"workflow stopped by signal"`; keep the dispatch of `graph.Stop.EmitEvent`.
- **Verdict:** NITS.

### MEDIUM

#### M-1 — HIGH-4 fix is fine; missing limit/dialect detail

- **File:** code2.md Fix 2
- **Description:** The `ListSkillExecutions` SQL is correct. Two notes: placeholders are driver-specific (`?` vs `$n` vs MySQL `?`) — the sample shows `$1`/`$2`/`$3` which won't compile for the SQLite driver; and the handler must pre-bind `limit` (currently capped at 200 in `HandleListExecutions`). The planner's `($2 = '' OR status = $2)` needs a bound arg for the empty-string case per driver.
- **Verdict:** NITS.

#### M-2 — MED-6/MySQL "gating" is unspecified

- **File:** code2.md Fix 9
- **Description:** "Gate the endpoints on driver support or add a NotImplementedError check" is undeclared. If MySQL remains a stub under a live `LATEST.sql` migration, MySQL deployments fail at first workflow call.
- **Fix:** Pick one: implement MySQL, or make the workflow routes/migrations conditional on `MEMOS_DRIVER != "mysql"`.
- **Verdict:** NITS.

### LOW

#### L-1 — LOW-4 recovery fix regresses the claim path (double claim)

- **File:** code2.md Fix 11 (recovery reorder)
- **Description:** The proposed `recoverPendingExecutions` **pre-claims** the execution (status → `running`, fresh `claim_expires_at = now + 300s`), then calls `go s.runDetachedExecution(ctx, claimed)` which **claims again**. The inner claim's predicate `(status='pending' OR (status='running' AND claim_expires_at < now))` fails for a fresh lease → 0 rows → the goroutine logs "could not be claimed" and exits, leaving the execution `running` and unprocessed until lease expiry (5 min), repeating forever. The *current* code (deserialize-then-claim) is wasteful on failed claims but functionally correct; this "fix" makes recovery dead.
- **Fix:** Either (a) don't pre-claim — deserialize first, then let `runDetachedExecution` claim (current behavior, just move deserialize after a cheap existence check), or (b) split `runDetachedExecution` into `runClaimedExecution(exec)` used by recovery with the already-claimed row.
- **Verdict:** REWORK.

---

## 2. Plan6 Conformance

| plan6 requirement | code2.md disposition | Status |
|-------------------|----------------------|--------|
| D2 — chat path runs durable loop w/ execution records | DEFER (HIGH-2) | **Deviates from plan6** |
| R3 — status re-read guards | FRACTIONAL (C-2 flaws) | Deviates |
| D4 — CEL env declares all node vars **and** works on outputs | REWORK (H-1) | Satisfied only after rework |
| D5/R2 — recovery worker | REWORK (L-1) | Deviates |
| D6 — outbound event | Kept | OK |
| MaxRetries/Timeout parsed + honored (MED-3) | DEFER | Deviates |
| §11 endpoints + RBAC | Kept | OK |
| §10 cadence (10s sleep + 60s poll) vs impl (30s ticker) | not addressed | Deviates (minor) |

**Verdict: plan6 "approved with REWORK" — code2.md is not ready for implementation as written.**

---

## 3. Test Plan Review (code2.md §Test Plan)

Required additions the plan's table misses:

| Gap | Why it matters |
|-----|----------------|
| No postgres/cockroach round-trip test | CRITICAL-1/DDB fix is the hackathon target; store tests only exercise `sqlite` (and a stub `mysql`) |
| No stop-during-step race test | The exact window C-2 describes (write `stopped`, then `failed`) |
| No recovery double-claim test | Would have caught L-1 immediately |
| No JSON `initial_vars` → float64 → CEL coercion test | H-1 gate for API-triggered workflows |
| No `ListSkillLogs` postgres JSONB scan test | MED-2 regression guard |

---

## 4. Recommended Implementation Order (reworked)

1. Fix 1 + **the four `LATEST.sql`** (includes time.Time struct change) + `go build ./...` + `go test ./store/...`
2. Fix 2 (ListSkillExecutions) — per-driver dialect
3. Fix 3 — **conditional terminal UPDATEs** + fresh context for post-cancel reads
4. Fix 4 — structured node outputs (or state projection) + numeric coercion; CEL dynamic env
5. Fix 11 — recovery claim path (no double claim)
6. Fix 6 — per-execution engine config in GenerateFn
7. Fixes 5, 7, 8, 9, 10 (nits) + Fix 11's SECTION 7B filtering
8. New tests from §3 above

---

## 5. Bottom Line

- **Tree status:** code2.md is un-implemented; all `code_review.md` findings remain live.
- **Plan status:** needs REWORK on Fixes 1 (LATEST), 3 (atomicity), 4 (output shape), 6 (tenant engine), 11 (double claim); NITS on 2, 5, 9.
- **Do not implement code2.md verbatim.** Apply the corrected versions above, then re-verify with the added test cases.