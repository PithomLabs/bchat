# bchat Durable Execution — Adversarial Review of code3.md (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** `bugs/059/code3.md` (Plan 3, "Ready for Implementation") verified against plan6.md (APPROVED baseline) and `code2_review.md`.
**Method:** Read the full plan; verified the working tree state; checked every fix against the actual code (types, signatures, driver layout, handler registration) rather than assuming plan pseudocode is valid Go.

---

## 0. Tree Status (verified)

code3.md (mtime 08:37) was written **after** the last source edit (store layer 07:09–07:13, agent engine 07:27–07:33). The working tree is byte-identical to the implementation reviewed in `code_review.md`. **No code3.md change is present in the code.**

| Plan area | Tree status |
|-----------|-------------|
| Fix 1 (time.Time struct) | NOT APPLIED — `store/agent.go:1361-1366` still `*int64`/`int64` |
| Fix 2 (ListSkillExecutions) | NOT APPLIED — absent from `store/driver.go` |
| Fix 3 (atomic guards) | NOT APPLIED — `checkpoint.go` unconditional updates |
| Fix 4 (dynamic CEL) | NOT APPLIED — `evaluator.go` static `standardCELVars` |
| Fix 5–10 | NOT APPLIED — files match pre-fix state |

`go build ./store/... ./server/router/api/v1/agent/...` clean; unit tests pass — Green only proves the pre-fix code. **Every `code_review.md` finding remains open.** This document reviews the code3.md plan itself.

---

## 1. What code3.md Gets Right

- Correctly adopts C-1 (four `LATEST.sql` in scope), the atomic-guard direction + fresh-context re-read (C-2), per-driver placeholder concern (M-1), MySQL gating (M-2), and no-pre-claim recovery (L-1).
- Sensible implementation order: DDL/types first, then tenant isolation, then terminal guards, then CEL.
- Test plan is materially better than code2's (P0 round-trip, stop-race, tenant-isolation, CEL, coercion, recovery).

---

## 2. Findings (plan-level)

### CRITICAL

#### C3-1 — Stop signal never writes `stopped` (Fix 5)

- **File:** code3.md Fix 5 → `execution.go`
- **Description:** The fix returns `errStopSignal` from `executeWorkflow` and comments `// stopExecution already wrote 'stopped'`. No code in the stop path calls `stopExecution` — the only write to `stopped` is the API/`StopExecution` handler path (`checkpoint.go:93-107`). On a matched `@signal` condition the execution stays `running`, the 5-minute lease expires, the recovery worker re-claims it, and the **entire workflow re-runs** (checkpoints still mark nodes complete, so pathologically it resumes, not re-runs — but either way it never terminates as `stopped`, and if the signal is re-matched it loops until lease swaps). The promised `stopped` terminal state for stop signals does not exist in the plan.
- **Fix:** In the stop branch, issue the conditional write itself before returning the sentinel — `s.store.StopSkillExecution(ctx, exec.ID)` (see C3-3) or an in-line `UPDATE ... SET status='stopped' WHERE id=... AND status IN ('pending','running')` — then `return errStopSignal`.
- **Verdict:** REWORK.

#### C3-2 — Structured-output wrapper does not satisfy plan6 D4 (Fix 4)

- **File:** code3.md Fix 4 → `execution.go` `executeWorkflow`
- **Description:** The fix stores `state[nodeName] = {"output": output, "node": nodeName}`. The documented conditions `search_kb.found == false` and `create_ticket.ticket_id != ''` access `.found`/`.ticket_id`, which **do not exist on that wrapper**. `SkillHandler.Execute` returns a `string` (`skill.go:19`), so no handler can emit typed fields; CEL field access on a map key that's absent is a runtime "no such key" error. Declaring the var as `cel.DynType` fixes the compile error but not the runtime field access. D4 ("conditions reference prior skill outputs") is satisfied only nominally — the plan's own examples still fail.
- **Fix:** Define a canonical output contract. Either (a) change `SkillHandler.Execute` to return `map[string]any` (or parse handler JSON output into an object) and make the CEL node var equal to that object, so `search_kb.found`/`create_ticket.ticket_id` resolve; or (b) rewrite the SCRIPT.md documentation/examples to reference `node.output`. The plan must pick one and test it.
- **Verdict:** REWORK.

### HIGH (must-fix)

#### C3-3 — Atomicity is planned at the service layer but no store API exists for it (Fix 3)

- **File:** code3.md Fix 3 → `store/driver.go`
- **Description:** The plan shows conditional `UPDATE ... WHERE id=... AND status NOT IN (...)` SQL at the service layer, but the only store method is `UpdateSkillExecution` which rewrites the whole row by ID with **no status predicate**. Nothing in the plan adds driver methods to carry the WHERE guard, so once implemented naively it reintroduces the exact TOCTOU C-2 was meant to close.
- **Fix:** Add to `store/driver.go` + all drivers (and the `Store` wrapper in `store/agent.go`):
  - `CompleteSkillExecution(ctx, id) error` → `UPDATE ... SET status='completed', completed_at=... WHERE id=? AND status='running'`
  - `FailSkillExecution(ctx, id, errorMsg) error` → `WHERE id=? AND status NOT IN ('stopped','completed')`
  - `StopSkillExecution(ctx, id) error` → `WHERE id=? AND status IN ('pending','running')`
  - RowsAffected==0 → no-op (log), not error. Then have `failExecution`/`completeExecution`/`stopExecution` call these.
- **Verdict:** REWORK (additive to Fix 3).

#### C3-4 — Fix 6 tenant injection targets a handler type that cannot exist

- **File:** code3.md Fix 6 → `execution.go` `executeStep`
- **Description:** The injection guard `if executorType == "llm"` is unreachable for real graphs: `llm:respond`-style handlers are never registered, so `executeStep` errors "handler not found" (`execution.go:229-231`) before the branch runs. The only working LLM handler is `builtin:llm_call`, whose `executorType` is `"builtin"` — the injection misses it. Related: `GenerateFn` is wired once in `NewService` and cannot capture per-run state; the plan's own note ("register handler with config at NewService" vs "resolve per execution") is contradictory.
- **Fix:** (a) Inject/read the tenant at call time inside `GenerateFn` (signature carries `tenantID *int32`), with `executeStep` passing `exec.TenantID` for the `builtin:llm_call` node; (b) invoke `s.requireLLMConfig(ctx, *exec.TenantID)` **inside** `GenerateFn`, not before the loop; (c) drop the dead `executorType == "llm"` branch or scope it to registered llm handlers.
- **Verdict:** REWORK.

### NITS

- **C3-5 — Fix 4 numeric coercion is not valid Go.** `if f, ok := v.(float64); ok == float64(int(f))` compares `bool == float64` and uses `f` in its own declaration. Must be `if f, ok := v.(float64); ok && f == float64(int(f)) { initial_vars[k] = int(f) }`. (`evaluator.go`/`execution.go`.)
- **C3-6 — Fix 10 types `DurationMs` as `int32`, but `SkillLog.DurationMs` is `int`** (`store/agent.go:1389`). Compile error. Use `int(duration.Milliseconds())`.
- **C3-7 — MySQL gating keyed on `os.Getenv("MEMOS_DRIVER")`;** the active driver is `profile.Driver` derived from viper (`bin/memos/main.go:59,159`). Gate on the service/route profile driver, not an env read that may not match.
- **C3-8 — Fix 1's scope omits the timestamp conversions in `logSkillStep`/`CreateSkillLog`/`ListSkillLogs`.** `execution.go:261` still does `time.Now().Unix()`; sqlite/postgres `CreateSkillLog`/`ListSkillLogs` still pass `int64` epoch (`store/db/sqlite/agent_skill.go:160-174`, postgres `210-212`). Enumerate them so the struct change doesn't leave a partial migration (breaks build or runtime).
- **C3-9 — No `store/db/cockroach/` directory exists.** CockroachDB is served by the postgres driver via `NewCockroachDB` (`store/db/postgres/cockroach.go:18`). The plan's "if exists" item should state this explicitly so an implementer doesn't create a third driver or, worse, skip the CRDB fixes entirely.

---

## 3. Plan6 Conformance (as planned, if fixes land correctly)

| plan6 requirement | code3.md | Assessment |
|-------------------|----------|------------|
| D2 — chat path durable loop | DEFERRED (HIGH-2) | Explicit gap vs plan6; acceptable only with product sign-off |
| R3 — status re-read guards | Fixed (Fix 3) | Only after C3-3 store API added |
| D4 — CEL env + output semantics | Fix 4 / structural | Not satisfied until C3-2 output contract |
| D5/R2 — recovery worker | Fixed (Fix 7) | OK |
| D6 — outbound events | kept | OK |
| MaxRetries/Timeout honored | DEFERRED | Explicit gap vs plan6 |
| §10 cadence (10s+60s) vs 30s ticker | deviation noted | OK (documented) |
| §11 endpoints + RBAC | kept + gating | OK after C3-7 |

**Expected-state verdict after our fixes:** "approved with nits" for conformance-with-deferrals. **As written now: REWORK** (C3-1, C3-2 must be fixed before implementation starts).

---

## 4. Test-Plan Review

Add to the P0/P1 table:

| Test | Catches |
|------|---------|
| `@signal` matched → DB status becomes `stopped` and **no subsequent re-claim/re-run by recovery** | C3-1 (missing write) |
| `condition: "search_kb.found == false"` evaluates with no runtime key error | C3-2 (output contract) |
| `FailSkillExecution` with a row already `stopped` → 0-row no-op | C3-3 (atomicity API) |
| `StopSkillExecution` leaves `completed` rows untouched | C3-3 |
| postgres/cockroach round-trip for `CreateSkillLog`/`ListSkillLogs` (JSONB + TIMESTAMPTZ) | C3-8 |
| `int64`→`time.Time` JSON shape change in `GET /executions/:id` (frontend contract) | Fix 1 side effect |

Also note the plan's `store/db/postgres/agent_skill_test.go` tests need a live PG/CRDB (integration) — the CI story for that must be stated (test tags/skip when not configured), or the P0 row is vacuous.

---

## 5. Bottom Line / Sign-off

- **Tree:** code3.md is un-implemented; all `code_review.md` findings remain live.
- **Plan:** incorporates all code2_review findings correctly, but Fix 5 (stop write) and Fix 4 (output contract) are **fatal as written**, and Fix 3/Fix 6 need the store-API / call-time-tenant corrections.
- **Recommended gate:** implement only after C3-1, C3-2, C3-3, C3-4 are resolved; apply C3-5…C3-9 as nits during implementation. Then run the expanded test table (including live PG/CRDB round-trips) before any re-review.