# bchat Durable Execution — Plan 3 (code3.md)

**Version:** 3.0
**Date:** 2026-08-05
**Status:** PLANNED — Ready for Implementation
**Review Source:** `bugs/059/code2_review.md` (DeepSeek adversarial review of code2.md)
**Baseline:** `plan6.md` (APPROVED) + `code_review.md` (original code review)

---

## Executive Summary

This plan incorporates the valid findings from the `code2_review.md` review of the `code2.md` fix plan. The code2.md plan was found to be **not ready for implementation as written** — it had 3 Critical, 2 High, and 3 Nit-level issues in the plan itself, even though none of the fixes had been applied to the codebase yet.

**Current state:** All 11 original findings from `code_review.md` remain live in the tree. code2.md was never implemented. This plan provides the corrected implementation approach.

---

## 1. Blocking Finding: code2.md Was Never Implemented

The code2.md plan was written after the last edit of every source file. The working tree is byte-identical to the pre-fix state. Every Critical/High/Medium finding from `code_review.md` remains open.

**Consequence:** This plan must implement everything from scratch, not patch code2.md's partial fixes.

---

## 2. Plan-Level Findings (from code2_review.md)

### C-1 — CRITICAL-1/1b fix omits LATEST.sql files

- **Issue:** code2.md Fix 1 rewrites only the versioned `0.36/00__add_skill_executions.sql` files. Fresh DBs materialize from `LATEST.sql`, not from versioned files → new deployments still break.
- **Fix:** Include all four `LATEST.sql` rewrites in Fix 1.
- **Status:** VALID — adopted.

### C-2 — CRITICAL-3 fix is non-atomic and runs on cancelled context

- **Issue:** code2.md Fix 3 proposed re-read-then-write for terminal guards. Two defects:
  1. TOCTOU window: concurrent stop between Get and Update still lands `failed` on `stopped`.
  2. Post-cancel re-read uses already-cancelled `ctx` → read fails → falls through to `failExecution` → clobbers `stopped`.
- **Fix:** Use atomic conditional writes:
  ```sql
  UPDATE ... SET status='failed' WHERE id=$1 AND status NOT IN ('stopped','completed')
  UPDATE ... SET status='completed' WHERE id=$1 AND status='running'
  ```
  Post-cancel re-read uses `context.WithoutCancel(ctx)` or `context.Background()`.
- **Status:** VALID — adopted.

### H-1 — HIGH-1 fix only implements half of plan6 D4

- **Issue:** Two fatal gaps remain:
  1. **String outputs can't be field-accessed.** Node outputs are plain strings (`state[nodeName] = output`). CEL conditions like `search_kb.found == false` do field selection on a string → runtime "no such field" error.
  2. **Int-typed standard vars vs JSON float64.** `standardCELVars` declares `urgency`/`tenant_id` as `cel.IntType`. API-triggered runs pass `initial_vars` as JSON → `float64` in Go. `condition: "urgency > 3"` fails eval with type error.
- **Fix:** (a) Persist structured node outputs (map/summary objects) or project `state` into CEL-accessible values; (b) declare standard numeric vars as `cel.DynType` and normalize `float64`/`int` in the vars map.
- **Status:** VALID — adopted.

### H-2 — HIGH-3 fix binds the LLM callback to tenant 0

- **Issue:** code2.md Fix 6 calls `svc.requireLLMConfig(ctx, 0)` with hardcoded tenant 0. The LLMHandler is registered once in `NewService` with no per-execution tenant context → wrong model and wrong API key in multi-tenant.
- **Fix:** Thread tenant/model into the handler at call time — store resolved engine config in `node.Params` at execution start, or create the LLMHandler per-execution with `exec.TenantID`'s engine config.
- **Status:** VALID — adopted.

### H-3 — HIGH-5 stop-condition eval silently mislogs

- **Issue:** `return nil` on matched stop makes `runDetachedExecution` log "workflow execution finished" for a stopped run.
- **Fix:** Return sentinel `errStopSignal`, log "workflow stopped by signal".
- **Status:** VALID — adopted.

### M-1 — HIGH-4 fix has driver-specific placeholder detail

- **Issue:** code2.md Fix 2 SQL uses `$1`/`$2`/`$3` which won't compile for SQLite driver (`?`).
- **Status:** VALID — nits, adopted.

### M-2 — MED-6/MySQL gating unspecified

- **Issue:** "Gate the endpoints on driver support or add NotImplementedError check" is undeclared. MySQL deployments fail at first workflow call.
- **Fix:** Gate workflow routes and migrations on `MEMOS_DRIVER != "mysql"`.
- **Status:** VALID — adopted.

### L-1 — LOW-4 recovery fix regresses the claim path (double claim)

- **Issue:** code2.md Fix 11 pre-claims the execution, then calls `runDetachedExecution` which claims again. Inner claim predicate fails for fresh lease → 0 rows → goroutine logs "could not be claimed" and exits, leaving execution `running` and unprocessed until lease expiry.
- **Fix:** Don't pre-claim — deserialize first, then let `runDetachedExecution` claim (current behavior, just move deserialize after a cheap existence check). Or split into `runClaimedExecution(exec)` for recovery.
- **Status:** VALID — adopted.

---

## 3. Implementation Plan

### Fix 1 — DDL/Go type mismatch (CRITICAL-1/1b) + LATEST.sql (C-1)

**Scope:** 8 files

**Go struct change** (`store/agent.go`):
- Change `SkillExecution` timestamps from `int64` to `time.Time`:
  ```go
  ClaimedAt       *time.Time    `json:"claimed_at,omitempty"`
  ClaimExpiresAt  *time.Time    `json:"claim_expires_at,omitempty"`
  CreatedAt       time.Time     `json:"created_at"`
  UpdatedAt       time.Time     `json:"updated_at"`
  CompletedAt     *time.Time    `json:"completed_at,omitempty"`
  ```
- `FindSkillExecution` — no changes needed (already `*string`/`*int32`/`*string`)
- Update `SkillLog` similarly: `StartedAt`/`CompletedAt` → `time.Time`/`*time.Time`

**SQLite driver** (`store/db/sqlite/agent_skill.go`):
- Convert `time.Time` → `int64` (epoch) on write, `int64` → `time.Time` on read
- Use helper functions: `timeToUnix(t *time.Time) *int64`, `unixToTime(v *int64) *time.Time`, etc.

**Postgres driver** (`store/db/postgres/agent_skill.go`):
- Remove all `string(checkpointJSON)` wrappers — pass JSONB directly via `json.Marshal` result
- pgx handles `[]byte` ↔ JSONB natively
- Time fields: pass `time.Time`/`*time.Time` directly — pgx handles `time.Time` ↔ `TIMESTAMPTZ`
- Fix JSONB scan in `ListSkillLogs`: use `sql.NullString` + `json.Unmarshal` for `Input`/`Output`

**CockroachDB driver** (`store/db/cockroach/agent_skill.go`):
- Same as Postgres (CockroachDB accepts pgx wire protocol)

**Postgres migration** (`store/migration/postgres/0.36/00__add_skill_executions.sql`):
- Rewrite from CockroachDB dialect to PostgreSQL dialect:
  - `STRING` → `TEXT`
  - `INT8` → `BIGINT`
  - `INT4` → `INTEGER`
  - Keep `JSONB`, `TIMESTAMPTZ`, `UUID`, `gen_random_uuid()` (all valid PG)

**LATEST.sql files** (C-1):
- Rewrite skill table sections in all four:
  - `store/migration/postgres/LATEST.sql` — PG dialect
  - `store/migration/cockroach/LATEST.sql` — CRDB dialect (already correct, verify)
  - `store/migration/sqlite/LATEST.sql` — SQLite dialect (already correct, verify)
  - `store/migration/mysql/LATEST.sql` — MySQL dialect (already correct, verify)

**Files modified:**
1. `store/agent.go` — struct field types
2. `store/db/sqlite/agent_skill.go` — time.Time ↔ epoch conversion
3. `store/db/postgres/agent_skill.go` — JSONB scan fix + time.Time pass-through
4. `store/db/cockroach/agent_skill.go` — if exists, same as postgres
5. `store/migration/postgres/0.36/00__add_skill_executions.sql` — PG dialect
6. `store/migration/postgres/LATEST.sql` — PG dialect skill tables
7. `store/migration/cockroach/LATEST.sql` — verify CRDB dialect
8. `store/migration/sqlite/LATEST.sql` — verify

**Verification:** `go build ./store/...` + `go test ./store/...` + `task validate:parity`

---

### Fix 2 — Cross-tenant data leak (CRITICAL-2) + List endpoint (HIGH-4)

**Scope:** 6 files

**New store method:**
- Add `ListSkillExecutions(ctx, find *FindSkillExecution, limit int) ([]*SkillExecution, error)` to `store/driver.go`
- Implement in SQLite, Postgres, MySQL (stub)

**Service layer** (`checkpoint.go`):
- Replace `listSkillExecutionsByTenant` with `s.store.ListSkillExecutions(ctx, find, limit)`
- Delete the broken `listExecutions` helper
- The store method already handles tenant filtering via `FindSkillExecution.TenantID`

**Handler layer** (`handlers.go`):
- `HandleListExecutions`: use `store.ListSkillExecutions` directly, bind `limit` param (max 200)

**Files modified:**
1. `store/driver.go` — add `ListSkillExecutions`
2. `store/db/sqlite/agent_skill.go` — implement `ListSkillExecutions`
3. `store/db/postgres/agent_skill.go` — implement `ListSkillExecutions`
4. `store/db/mysql/agent_skill.go` — stub `ListSkillExecutions`
5. `server/router/api/v1/agent/checkpoint.go` — rewrite `listExecutionsByTenant` + `listExecutions`
6. `server/router/api/v1/agent/handlers.go` — fix `HandleListExecutions`

**Verification:** `go build ./...` + unit test with tenant filter

---

### Fix 3 — Terminal state guards (CRITICAL-3) + atomic writes (C-2)

**Scope:** 3 files

**checkpoint.go:**
- `failExecution`: Replace unconditional update with atomic conditional:
  ```sql
  UPDATE agent_skill_executions
  SET status = 'failed', updated_at = $1, completed_at = $1, ...
  WHERE id = $2 AND status NOT IN ('stopped', 'completed')
  ```
  If 0 rows affected, log "execution already terminal" and return nil (not error).

- `completeExecution`: Replace unconditional update with atomic conditional:
  ```sql
  UPDATE agent_skill_executions
  SET status = 'completed', ...
  WHERE id = $1 AND status = 'running'
  ```
  Same 0-row guard.

- `stopExecution`: Already writes `stopped` — add guard:
  ```sql
  UPDATE agent_skill_executions
  SET status = 'stopped', ...
  WHERE id = $1 AND status IN ('pending', 'running')
  ```

**execution.go:**
- `runDetachedExecution`: After `executeWorkflow` returns, re-read status with `context.Background()` (not cancelled ctx):
  ```go
  fresh, err := s.store.GetSkillExecution(context.Background(), &store.FindSkillExecution{ID: &exec.ID})
  if err == nil && fresh.Status == "stopped" {
      slog.Info("workflow stopped by signal", "exec_id", exec.ID)
      return // don't call failExecution
  }
  ```

**Store drivers:**
- SQLite: add conditional WHERE clauses
- Postgres: add conditional WHERE clauses

**Files modified:**
1. `server/router/api/v1/agent/checkpoint.go` — atomic terminal writes
2. `server/router/api/v1/agent/execution.go` — fresh context re-read + stop sentinel
3. `store/db/sqlite/agent_skill.go` — conditional UPDATE helpers
4. `store/db/postgres/agent_skill.go` — conditional UPDATE helpers

**Verification:** Test: stop during step → DB shows `stopped`, not `failed`; complete after stop → no-op

---

### Fix 4 — CEL dynamic env + structured outputs (HIGH-1) + numeric coercion (H-1)

**Scope:** 3 files

**evaluator.go:**
- New function `EvalConditionDynamic(ctx, expr, vars, graph)`:
  - Build env from `graph.Nodes` keys as `cel.DynType` + standard vars as `cel.DynType`
  - Normalize numeric vars: `int(v.(float64))` for integral values
  - Compile + eval with 5s timeout

- Existing `EvalCondition` → used for non-workflow conditions (unchanged)

**execution.go:**
- `executeWorkflow`: After each step, store structured output:
  ```go
  state[nodeName] = map[string]any{
      "output": output,
      "node":   nodeName,
  }
  ```
  Or: store raw output and project into CEL vars with metadata.

- `executeStep`: Call `EvalConditionDynamic` instead of `EvalCondition` when `graph != nil`

**execution.go — numeric coercion:**
- Before CEL eval, normalize `initial_vars` from API:
  ```go
  for k, v := range initial_vars {
      if f, ok := v.(float64); ok == float64(int(f)) {
          initial_vars[k] = int(f)
      }
  }
  ```

**Files modified:**
1. `server/router/api/v1/agent/evaluator.go` — `EvalConditionDynamic` + dynamic vars
2. `server/router/api/v1/agent/execution.go` — structured outputs + numeric normalization
3. `server/router/api/v1/agent/skill.go` — (optional) pass graph to executeStep

**Verification:** Test: `condition: "search_kb.found == false"` compiles and evaluates; `urgency > 3` works with float64 initial_vars

---

### Fix 5 — Stop signal evaluation (HIGH-5) + stop sentinel (H-3)

**Scope:** 2 files

**execution.go:**
- After each checkpointed step, evaluate `graph.Stop.Condition`:
  ```go
  if graph.Stop != nil && graph.Stop.Condition != "" {
      result, err := EvalConditionDynamic(ctx, graph.Stop.Condition, state, graph)
      if err != nil {
          slog.Warn("stop condition eval failed", "error", err)
      } else if result.Met {
          // Emit stop event
          if graph.Stop.EmitEvent != "" {
              s.dispatchEvent(ctx, *exec.TenantID, exec.ConversationID, graph.Stop.EmitEvent, "")
          }
          return errStopSignal
      }
  }
  ```

- Define sentinel: `var errStopSignal = fmt.Errorf("workflow stopped by signal")`

**runDetachedExecution:**
- Check for `errStopSignal`:
  ```go
  if err := s.executeWorkflow(ctx, exec, graph, s.skillRegistry); err != nil {
      if errors.Is(err, errStopSignal) {
          slog.Info("workflow stopped by signal", "exec_id", exec.ID)
          return // stopExecution already wrote 'stopped'
      }
      s.failExecution(ctx, exec, err.Error())
      return
  }
  ```

**Files modified:**
1. `server/router/api/v1/agent/execution.go` — stop eval + sentinel

**Verification:** Test: SCRIPT with `@signal: condition: "urgency > 5"`, run with `urgency=7` → stops after step, status=`stopped`

---

### Fix 6 — GenerateFn per-execution tenant engine (HIGH-3/H-2)

**Scope:** 2 files

**service.go:**
- In `executeWorkflow`, before starting the loop, resolve LLM config for `exec.TenantID`:
  ```go
  llmConfig, err := s.requireLLMConfig(ctx, int(*exec.TenantID))
  if err != nil {
      return fmt.Errorf("resolve LLM config: %w", err)
  }
  ```
  Store `llmConfig` in execution context or pass to handlers.

**skill_builtins.go:**
- `LLMHandler.GenerateFn` signature change to accept tenant context:
  ```go
  GenerateFn func(ctx context.Context, tenantID *int32, prompt string, vars map[string]any) (string, error)
  ```
  Or: store tenant ID in `node.Params` at execution start and read from there.

**execution.go:**
- In `executeStep`, when handler is `llm:` type, inject tenant ID into params or context:
  ```go
  if executorType == "llm" {
      node.Params["tenant_id"] = strconv.Itoa(int(*exec.TenantID))
  }
  ```

**Files modified:**
1. `server/router/api/v1/agent/skill_builtins.go` — tenant-aware GenerateFn
2. `server/router/api/v1/agent/execution.go` — inject tenant into llm steps
3. `server/router/api/v1/agent/service.go` — wire GenerateFn with tenant resolution

**Verification:** Test: multi-tenant, llm_call step → uses correct tenant's model/API key

---

### Fix 7 — Recovery worker no double-claim (L-1)

**Scope:** 1 file

**recovery.go:**
- Remove pre-claim logic. Deserialize first, then let `runDetachedExecution` claim:
  ```go
  func (s *Service) recoverPendingExecutions(ctx context.Context) {
      pending, err := s.store.ListPendingSkillExecutions(ctx)
      if err != nil { ... }
      for _, exec := range pending {
          graph := &SkillGraph{}
          if err := json.Unmarshal([]byte(exec.SkillGraphJSON), graph); err != nil {
              s.failExecution(ctx, exec, "recovery: deserialize failed")
              continue
          }
          if !graph.HasSkills { continue }
          go s.runDetachedExecution(ctx, exec)  // runDetachedExecution claims internally
      }
  }
  ```

**Files modified:**
1. `server/router/api/v1/agent/recovery.go` — remove pre-claim, just dispatch

**Verification:** Test: recovery picks up pending → runDetachedExecution claims → executes

---

### Fix 8 — current_node stores node name, not output (MED-1)

**Scope:** 1 file

**checkpoint.go:**
- `writeCheckpoint`: Change `exec.CurrentNode = output` to `exec.CurrentNode = nodeName` (pass node name as parameter)

**execution.go:**
- `executeWorkflow`: Pass node name to `writeCheckpoint`:
  ```go
  if err := s.writeCheckpoint(ctx, exec, state, nodeName); err != nil { ... }
  ```

**Files modified:**
1. `server/router/api/v1/agent/checkpoint.go` — signature change
2. `server/router/api/v1/agent/execution.go` — pass nodeName

**Verification:** `current_node` in DB is `classify_intent`, not `{"intent":"emergency"}`

---

### Fix 9 — MySQL gating (M-2/MED-6)

**Scope:** 2 files

**Option chosen:** Gate on `MEMOS_DRIVER != "mysql"`.

**v1.go:**
- Conditionally register workflow routes:
  ```go
  if os.Getenv("MEMOS_DRIVER") != "mysql" {
      authGroup.POST("/agent/:slug/workflow/start", h.HandleStartWorkflow)
      authGroup.POST("/agent/:slug/executions/:id/stop", h.HandleStopExecution)
      authGroup.GET("/agent/:slug/executions/:id", h.HandleGetExecution)
      authGroup.GET("/agent/:slug/executions", h.HandleListExecutions)
  }
  ```

**recovery.go:**
- Skip recovery worker if MySQL:
  ```go
  if os.Getenv("MEMOS_DRIVER") == "mysql" {
      return
  }
  ```

**Files modified:**
1. `server/router/api/v1/v1.go` — conditional route registration
2. `server/router/api/v1/agent/recovery.go` — skip on MySQL

**Verification:** MySQL driver → workflow routes 404, no recovery worker

---

### Fix 10 — Minor fixes

**execution.go — logSkillStep duration:**
- Pass `start time.Time` and compute `durationMs`:
  ```go
  durationMs := time.Since(start).Milliseconds()
  log.DurationMs = int32(durationMs)
  ```

**service.go — buildSystemPrompt SECTION 7B filtering:**
- Only list skills with registered handlers:
  ```go
  for name, skill := range config.SkillGraph.Nodes {
      if _, ok := s.skillRegistry.Get(skill.Handler); !ok {
          continue // skip unregistered
      }
      // include in prompt
  }
  ```

**Files modified:**
1. `server/router/api/v1/agent/execution.go` — duration tracking
2. `server/router/api/v1/agent/service.go` — SECTION 7B filter

---

## 4. Test Plan

### New tests required

| Test | Why | Priority |
|------|-----|----------|
| Postgres round-trip: create → get → update → list | CRITICAL-1/DDB fix is the hackathon target | P0 |
| Stop-during-step race: stop while step running → `stopped` not `failed` | CRITICAL-3/TOCTOU window | P0 |
| List executions: tenant A sees only A's rows | CRITICAL-2/tenant isolation | P0 |
| CEL dynamic: `search_kb.found == false` compiles + evaluates | HIGH-1/D4 | P0 |
| Float64 → int coercion: `urgency > 3` with API initial_vars | H-1/JSON round-trip | P0 |
| Recovery: picks pending → runDetachedExecution claims → executes | L-1/double claim regression | P0 |
| Stop sentinel: `errStopSignal` → DB shows `stopped` | H-3/stop logging | P1 |
| LLM handler: tenant-specific model/API key | H-2/GenerateFn wiring | P1 |
| current_node: stores node name not output | MED-1 | P1 |
| MySQL gating: workflow routes 404 on MySQL | M-2 | P1 |

### Existing tests to verify

- `parser_skill_test.go` — 8 tests (unchanged)
- `skill_test.go` — 8 tests (unchanged)
- `evaluator_test.go` — 10 tests (add dynamic variants)
- `skill_builtins_test.go` — 8 tests (add tenant-aware variant)
- `execution_test.go` — 9 tests (add stop + structured output)

### Manual verification checklist

- [ ] `go build ./...` clean
- [ ] `go test ./store/... ./server/router/api/v1/agent/...` all pass
- [ ] `task validate:parity` — all 4 drivers match schema
- [ ] Stop mid-step → DB shows `stopped`, not `failed`
- [ ] List executions → tenant-scoped only
- [ ] CEL `search_kb.found == false` compiles
- [ ] Float64 initial_vars → CEL eval succeeds
- [ ] `current_node` = node name, not output
- [ ] `llm_call` step uses tenant's model/key
- [ ] Recovery worker picks pending without double-claim

---

## 5. Implementation Order

| Step | Fix | Files | Est. LOC |
|------|-----|-------|----------|
| 1 | Fix 1 — DDL/types + LATEST.sql | 8 files | ~150 |
| 2 | Fix 2 — ListSkillExecutions (CRITICAL-2) | 6 files | ~120 |
| 3 | Fix 3 — Atomic terminal guards (CRITICAL-3) | 4 files | ~80 |
| 4 | Fix 4 — CEL dynamic env + structured outputs | 3 files | ~100 |
| 5 | Fix 7 — Recovery no double-claim | 1 file | ~15 |
| 6 | Fix 6 — GenerateFn tenant engine | 3 files | ~50 |
| 7 | Fix 5 — Stop signal eval + sentinel | 1 file | ~40 |
| 8 | Fix 8 — current_node stores name | 2 files | ~10 |
| 9 | Fix 9 — MySQL gating | 2 files | ~20 |
| 10 | Fix 10 — Minor fixes | 2 files | ~20 |
| 11 | New tests | 3-5 files | ~200 |
| | **Total** | | **~805** |

---

## 6. Plan6 Conformance (post-fix)

| plan6 requirement | Status after code3 |
|-------------------|---------------------|
| D2 — chat path runs durable loop | DEFERRED (HIGH-2) — documented, not blocking hackathon |
| R3 — status re-read guards | FIXED (Fix 3) — atomic conditional writes |
| D4 — CEL env declares all node vars + works on outputs | FIXED (Fix 4) — dynamic env + structured outputs |
| D5/R2 — recovery worker | FIXED (Fix 7) — no double-claim |
| D6 — outbound event | Already working (dispatchEvent) |
| MaxRetries/Timeout parsed + honored | DEFERRED — fields parsed but not driving retry logic |
| §11 endpoints + RBAC | Already working + Fix 9 gating |
| §10 cadence (10s sleep + 60s poll) vs impl (30s ticker) | Minor deviation, acceptable |

---

## 7. Bottom Line

- **code2.md was never implemented.** All code_review.md findings remain live.
- **code2_review.md found 8 valid issues** in the code2.md plan itself (3 Critical, 2 High, 3 Nits).
- **This plan (code3.md)** provides corrected versions of all fixes, incorporating every valid finding.
- **Estimated effort:** ~805 LOC across ~30 files, plus ~200 LOC of tests.
- **Implementation order:** DDL/types first (blocks everything), then tenant isolation, then terminal guards, then CEL, then minor fixes.
- **Hackathon readiness:** After this plan is implemented, the durable execution pipeline will work on SQLite, PostgreSQL, and CockroachDB with correct tenant isolation, stop semantics, and CEL conditions.

---

## 8. Files Modified (complete manifest)

### Store layer
1. `store/agent.go` — SkillExecution/SkillLog struct time.Time
2. `store/driver.go` — +ListSkillExecutions interface method
3. `store/db/sqlite/agent_skill.go` — time conversion + ListSkillExecutions + conditional UPDATE
4. `store/db/postgres/agent_skill.go` — JSONB scan fix + time pass-through + ListSkillExecutions + conditional UPDATE
5. `store/db/mysql/agent_skill.go` — stub ListSkillExecutions

### Migrations
6. `store/migration/postgres/0.36/00__add_skill_executions.sql` — PG dialect
7. `store/migration/postgres/LATEST.sql` — PG dialect skill tables
8. `store/migration/cockroach/LATEST.sql` — verify
9. `store/migration/sqlite/LATEST.sql` — verify
10. `store/migration/mysql/LATEST.sql` — verify

### Agent engine
11. `server/router/api/v1/agent/evaluator.go` — EvalConditionDynamic + dynamic vars
12. `server/router/api/v1/agent/checkpoint.go` — atomic terminal writes + current_node fix
13. `server/router/api/v1/agent/execution.go` — structured outputs + stop eval + stop sentinel + fresh context re-read
14. `server/router/api/v1/agent/recovery.go` — no double-claim
15. `server/router/api/v1/agent/skill_builtins.go` — tenant-aware GenerateFn
16. `server/router/api/v1/agent/service.go` — wire GenerateFn + SECTION 7B filter

### API
17. `server/router/api/v1/v1.go` — conditional route registration
18. `server/router/api/v1/agent/handlers.go` — HandleListExecutions fix

### Tests
19. `server/router/api/v1/agent/evaluator_test.go` — add dynamic CEL tests
20. `server/router/api/v1/agent/execution_test.go` — add stop + structured output tests
21. `store/db/sqlite/agent_skill_test.go` — (new) round-trip tests
22. `store/db/postgres/agent_skill_test.go` — (new) round-trip tests

---

**This plan is ready for implementation.** All code2_review.md findings are incorporated. The plan maintains plan6 conformance while fixing every identified defect.
