# bchat Durable Execution — Plan 4 (code4.md)

**Version:** 4.0
**Date:** 2026-08-05
**Status:** PLANNED — Ready for Implementation
**Review Source:** `bugs/059/code3_review.md` (DeepSeek adversarial review of code3.md)
**Baseline:** `plan6.md` (APPROVED) + `code3.md` (prior plan) + `code2_review.md` + `code_review.md`

---

## Executive Summary

This plan incorporates the valid findings from the `code3_review.md` review of the `code3.md` plan. code3.md was found to have 4 Critical-level plan defects (C3-1 through C3-4) and 5 nits (C3-5 through C3-9). **None of code3.md's fixes have been applied to the codebase.**

**Key corrections from code3.md:**
1. **C3-1:** Stop signal path must write `stopped` to DB — code3.md's Fix 5 returned a sentinel without writing the terminal state
2. **C3-2:** Structured output wrapper doesn't satisfy D4 — node outputs must be parsed objects for CEL field access
3. **C3-3:** Atomic terminal guards need dedicated store methods — code3.md planned SQL at service layer but no store API carries the WHERE predicate
4. **C3-4:** Fix 6 tenant injection targets unreachable code — `executorType == "llm"` never matches `builtin:llm_call`

**Current state:** All `code_review.md` findings remain live in the tree. No fixes have been applied.

---

## 1. Findings from code3_review.md

### C3-1 — Stop signal never writes `stopped` (Fix 5)

- **Issue:** code3.md Fix 5 returns `errStopSignal` from `executeWorkflow` with comment `// stopExecution already wrote 'stopped'`. No code in the stop path calls `stopExecution`. On a matched `@signal` condition the execution stays `running`, the 5-minute lease expires, the recovery worker re-claims it, and the workflow re-runs. The promised `stopped` terminal state does not exist.
- **Fix:** In the stop branch, write `stopped` via `StopSkillExecution` before returning the sentinel.
- **Status:** VALID — adopted.

### C3-2 — Structured-output wrapper does not satisfy plan6 D4 (Fix 4)

- **Issue:** code3.md stores `state[nodeName] = {"output": output, "node": nodeName}`. CEL conditions like `search_kb.found == false` access `.found` which doesn't exist on that wrapper. `SkillHandler.Execute` returns `string` (`skill.go:19`), so no handler emits typed fields. Declaring the var as `cel.DynType` fixes compile but not runtime field access.
- **Fix:** Define a canonical output contract. Two options:
  - **(a) JSON-parse handler output:** If handler output is valid JSON, parse it into `map[string]any` and store as the CEL node variable. `search_kb.found` resolves because `found` is a key in the parsed map.
  - **(b) Change `SkillHandler.Execute` to return `map[string]any`:** More invasive but cleaner long-term.
  - **Decision:** Option (a) — parse JSON output at storage time, fallback to `{"output": raw_string}` for non-JSON handlers. This is backward-compatible.
- **Status:** VALID — adopted.

### C3-3 — No store API for atomic guards (Fix 3)

- **Issue:** code3.md plans conditional `UPDATE ... WHERE status NOT IN (...)` SQL at the service layer, but the only store method is `UpdateSkillExecution` which rewrites the whole row by ID with no status predicate. Nothing in the plan adds driver methods to carry the WHERE guard.
- **Fix:** Add to `store/driver.go` + all drivers + `store/agent.go` wrapper:
  - `CompleteSkillExecution(ctx, id string) error` — `UPDATE ... SET status='completed', completed_at=NOW() WHERE id=? AND status='running'`
  - `FailSkillExecution(ctx, id string, errorMsg string) error` — `UPDATE ... SET status='failed', error_message=?, completed_at=NOW() WHERE id=? AND status NOT IN ('stopped','completed')`
  - `StopSkillExecution(ctx, id string) error` — `UPDATE ... SET status='stopped' WHERE id=? AND status IN ('pending','running')`
  - RowsAffected==0 → log "already terminal", return nil (not error).
- **Status:** VALID — adopted.

### C3-4 — Fix 6 tenant injection targets unreachable code (Fix 6)

- **Issue:** The injection guard `if executorType == "llm"` is unreachable for real graphs: `llm:respond`-style handlers are never registered, so `executeStep` errors "handler not found" before the branch runs. The only working LLM handler is `builtin:llm_call` whose `executorType` is `"builtin"`.
- **Fix:** (a) `GenerateFn` signature carries `tenantID *int32`; (b) invoke `s.requireLLMConfig(ctx, *exec.TenantID)` **inside** `GenerateFn`, not before the loop; (c) `executeStep` passes `exec.TenantID` to the handler via params/context for `builtin:llm_call`.
- **Status:** VALID — adopted.

### C3-5 — Numeric coercion is not valid Go (nit)

- **Issue:** `if f, ok := v.(float64); ok == float64(int(f))` compares `bool == float64` and uses `f` in its own declaration.
- **Fix:** `if f, ok := v.(float64); ok && f == float64(int(f)) { initial_vars[k] = int(f) }`
- **Status:** VALID — adopted.

### C3-6 — DurationMs type mismatch (nit)

- **Issue:** code3.md Fix 10 types `DurationMs` as `int32`, but `SkillLog.DurationMs` is `int` (`store/agent.go:1389`). Compile error.
- **Fix:** Use `int(duration.Milliseconds())`.
- **Status:** VALID — adopted.

### C3-7 — MySQL gating keyed on env var, not profile driver (nit)

- **Issue:** `os.Getenv("MEMOS_DRIVER")` may not match `profile.Driver` derived from viper.
- **Fix:** Gate on `s.profile.Driver != "mysql"` in recovery.go and `apiv1Service.Profile.Driver != "mysql"` in v1.go.
- **Status:** VALID — adopted.

### C3-8 — Fix 1 scope omits timestamp conversions in log paths (nit)

- **Issue:** `logSkillStep` still does `time.Now().Unix()` (`execution.go:261`); sqlite/postgres `CreateSkillLog`/`ListSkillLogs` still pass `int64` epoch. Struct change to `time.Time` breaks these paths.
- **Fix:** Enumerate all timestamp conversion sites in Fix 1 scope:
  - `execution.go:261` — `StartedAt: time.Now()` (not `.Unix()`)
  - `sqlite/agent_skill.go:160-174` — `CreateSkillLog` epoch→time conversion
  - `sqlite/agent_skill.go:200-218` — `ListSkillLogs` scan time→epoch
  - `postgres/agent_skill.go:161-175` — `CreateSkillLog` pass time.Time directly
  - `postgres/agent_skill.go:210-212` — `ListSkillLogs` JSONB scan fix (already in plan)
- **Status:** VALID — adopted.

### C3-9 — No `store/db/cockroach/` directory exists (nit)

- **Issue:** CockroachDB is served by the postgres driver via `NewCockroachDB` (`store/db/postgres/cockroach.go:18`). The plan's "if exists" item could mislead an implementer.
- **Fix:** State explicitly: CRDB uses `store/db/postgres/` code. No separate cockroach driver needed. Fix postgres driver → CRDB gets it for free.
- **Status:** VALID — adopted.

---

## 2. Revised Implementation Plan

### Fix 1 — DDL/Go type mismatch (CRITICAL-1/1b) + LATEST.sql (C-1) + C3-8

**Scope:** 10 files (expanded from code3's 8)

**Go struct change** (`store/agent.go`):
```go
// SkillExecution — timestamps
ClaimedAt       *time.Time    `json:"claimed_at,omitempty"`
ClaimExpiresAt  *time.Time    `json:"claim_expires_at,omitempty"`
CreatedAt       time.Time     `json:"created_at"`
UpdatedAt       time.Time     `json:"updated_at"`
CompletedAt     *time.Time    `json:"completed_at,omitempty"`

// SkillLog — timestamps (C3-8)
StartedAt       time.Time     `json:"started_at"`
CompletedAt     *time.Time    `json:"completed_at,omitempty"`
// DurationMs stays int (no type change)
```

**SQLite driver** (`store/db/sqlite/agent_skill.go`):
- Add helpers: `timeToUnix(*time.Time) *int64`, `unixToTime(*int64) *time.Time`, `timeToUnixVal(time.Time) int64`, `unixToTimeVal(int64) time.Time`
- `CreateSkillExecution`: convert time.Time → int64 on write
- `scanSkillExecutions`: convert int64 → time.Time on read
- `ClaimSkillExecution`: convert time.Time → int64 on write
- `UpdateSkillExecution`: convert time.Time → int64 on write
- `CreateSkillLog` (C3-8): `StartedAt: timeToUnixVal(log.StartedAt)`, `CompletedAt: timeToUnix(log.CompletedAt)`
- `ListSkillLogs` (C3-8): scan int64 → time.Time

**Postgres driver** (`store/db/postgres/agent_skill.go`):
- Remove `string(checkpointJSON)` wrappers — pgx handles `[]byte` ↔ JSONB
- Time fields: pass `time.Time`/`*time.Time` directly — pgx handles ↔ TIMESTAMPTZ
- `ListSkillLogs` JSONB scan (MED-2): use `sql.NullString` + `json.Unmarshal` for `Input`/`Output`
- `CreateSkillLog` (C3-8): pass `time.Time` directly

**CockroachDB** (C3-9): No separate driver. `store/db/postgres/cockroach.go` delegates to `NewDB`. All postgres fixes apply to CRDB.

**Postgres migration** (`store/migration/postgres/0.36/00__add_skill_executions.sql`):
- `STRING` → `TEXT`, `INT8` → `BIGINT`, `INT4` → `INTEGER`
- Keep `JSONB`, `TIMESTAMPTZ`, `UUID`, `gen_random_uuid()`

**LATEST.sql files** (C-1):
- `store/migration/postgres/LATEST.sql` — PG dialect skill tables
- `store/migration/cockroach/Latest.sql` — CRDB dialect (verify correct)
- `store/migration/sqlite/LATEST.sql` — SQLite dialect (verify correct)
- `store/migration/mysql/LATEST.sql` — MySQL dialect (verify correct)

**execution.go** (C3-8):
- `logSkillStep`: `StartedAt: time.Now()` (not `.Unix()`)
- Pass `start time.Time` to `logSkillStep`, compute `DurationMs: int(time.Since(start).Milliseconds())`

**Files modified:**
1. `store/agent.go` — struct field types
2. `store/db/sqlite/agent_skill.go` — time conversion helpers + all CRUD + log methods
3. `store/db/postgres/agent_skill.go` — JSONB fix + time pass-through + log methods
4. `store/migration/postgres/0.36/00__add_skill_executions.sql` — PG dialect
5. `store/migration/postgres/Latest.sql` — PG dialect skill tables
6. `store/migration/cockroach/Latest.sql` — verify
7. `store/migration/sqlite/Latest.sql` — verify
8. `store/migration/mysql/Latest.sql` — verify
9. `server/router/api/v1/agent/execution.go` — logSkillStep timestamp fix
10. `store/db/sqlite/agent_skill_test.go` — (new) time round-trip tests

**Verification:** `go build ./store/...` + `go test ./store/...` + `task validate:parity`

---

### Fix 2 — Cross-tenant data leak (CRITICAL-2) + List endpoint (HIGH-4)

**Scope:** 6 files

**New store method** (`store/driver.go`):
```go
ListSkillExecutions(ctx context.Context, find *FindSkillExecution, limit int) ([]*SkillExecution, error)
```

**SQLite driver**: Use existing `listSkillExecutions` (already works, just add to interface)
**Postgres driver**: Use existing `listSkillExecutions` (already works, just add to interface)
**MySQL driver**: Stub returning `nil, fmt.Errorf("not implemented")`

**Service layer** (`checkpoint.go`):
- `listExecutionsByTenant`: replace with `s.store.ListSkillExecutions(ctx, find, limit)`
- Delete broken `listExecutions` helper and `listSkillExecutionsByTenant`

**Handler layer** (`handlers.go`):
- `HandleListExecutions`: use `store.ListSkillExecutions` directly

**Files modified:**
1. `store/driver.go` — add `ListSkillExecutions`
2. `store/db/sqlite/agent_skill.go` — expose `listSkillExecutions` as `ListSkillExecutions`
3. `store/db/postgres/agent_skill.go` — expose `listSkillExecutions` as `ListSkillExecutions`
4. `store/db/mysql/agent_skill.go` — stub `ListSkillExecutions`
5. `server/router/api/v1/agent/checkpoint.go` — rewrite list methods
6. `server/router/api/v1/agent/handlers.go` — fix `HandleListExecutions`

**Verification:** `go build ./...` + test with tenant filter

---

### Fix 3 — Atomic terminal guards (CRITICAL-3) + C3-3 store API

**Scope:** 7 files (expanded from code3's 3)

**New store methods** (`store/driver.go` + `store/agent.go` wrapper):
```go
CompleteSkillExecution(ctx context.Context, id string) error
FailSkillExecution(ctx context.Context, id string, errorMsg string) error
StopSkillExecution(ctx context.Context, id string) error
```

**SQLite driver** (`store/db/sqlite/agent_skill.go`):
```sql
-- CompleteSkillExecution
UPDATE agent_skill_executions SET status='completed', completed_at=?, updated_at=? WHERE id=? AND status='running'

-- FailSkillExecution
UPDATE agent_skill_executions SET status='failed', error_message=?, completed_at=?, updated_at=? WHERE id=? AND status NOT IN ('stopped','completed')

-- StopSkillExecution
UPDATE agent_skill_executions SET status='stopped', updated_at=? WHERE id=? AND status IN ('pending','running')
```
Check `RowsAffected() == 0` → log + return nil.

**Postgres driver** (`store/db/postgres/agent_skill.go`):
Same SQL with `$n` placeholders. Same RowsAffected guard.

**checkpoint.go:**
- `completeExecution`: call `s.store.CompleteSkillExecution(ctx, exec.ID)` instead of `UpdateSkillExecution`
- `failExecution`: call `s.store.FailSkillExecution(ctx, exec.ID, errMsg)` instead of `UpdateSkillExecution`
- `stopExecution`: call `s.store.StopSkillExecution(ctx, execID)` instead of `UpdateSkillExecution`

**execution.go:**
- `runDetachedExecution`: after `executeWorkflow` returns, re-read status with `context.Background()`:
  ```go
  fresh, err := s.store.GetSkillExecution(context.Background(), &store.FindSkillExecution{ID: &exec.ID})
  if err == nil && fresh.Status == "stopped" {
      slog.Info("workflow stopped by signal", "exec_id", exec.ID)
      return
  }
  ```
  Only call `failExecution` if status is NOT `stopped`.

**Files modified:**
1. `store/driver.go` — 3 new interface methods
2. `store/agent.go` — 3 new wrapper methods
3. `store/db/sqlite/agent_skill.go` — 3 new implementations
4. `store/db/postgres/agent_skill.go` — 3 new implementations
5. `store/db/mysql/agent_skill.go` — 3 stubs
6. `server/router/api/v1/agent/checkpoint.go` — use new store methods
7. `server/router/api/v1/agent/execution.go` — fresh context re-read

**Verification:** Test: stop during step → `stopped` not `failed`; complete after stop → no-op

---

### Fix 4 — CEL dynamic env + output contract (HIGH-1/C3-2)

**Scope:** 3 files

**Canonical output contract (C3-2):**
- At node output storage time, attempt JSON parse of handler output:
  ```go
  var cellValue any
  var parsed map[string]any
  if json.Unmarshal([]byte(output), &parsed) == nil && parsed != nil {
      cellValue = parsed  // search_kb.found resolves
  } else {
      cellValue = map[string]any{"output": output}  // fallback: node.output
  }
  state[nodeName] = cellValue
  ```
- This satisfies D4: `search_kb.found == false` works because `search_kb` is a parsed map.

**evaluator.go:**
- New function `EvalConditionDynamic(ctx, expr, vars map[string]any, graph *SkillGraph) (*ConditionResult, error)`:
  - Build env from `graph.Nodes` keys as `cel.DynType` + standard vars as `cel.DynType`
  - Normalize numeric vars: `int(v.(float64))` for integral values (C3-5 fix)
  - Compile + eval with 5s timeout
- Existing `EvalCondition` kept for non-workflow conditions

**execution.go:**
- `executeStep`: call `EvalConditionDynamic` when graph is available
- `executeWorkflow`: store parsed output per canonical contract above
- `executeStep` signature gains `graph *SkillGraph` parameter

**Files modified:**
1. `server/router/api/v1/agent/evaluator.go` — `EvalConditionDynamic`
2. `server/router/api/v1/agent/execution.go` — parsed output storage + call `EvalConditionDynamic`
3. `server/router/api/v1/agent/skill.go` — (no change needed, Execute stays `string` return)

**Verification:** Test: `condition: "search_kb.found == false"` compiles + evaluates; JSON output parsed, non-JSON falls back

---

### Fix 5 — Stop signal evaluation + write + sentinel (C3-1 + HIGH-5 + H-3)

**Scope:** 1 file

**execution.go:**
- Define sentinel: `var errStopSignal = fmt.Errorf("workflow stopped by signal")`
- In `executeWorkflow`, after each checkpointed step:
  ```go
  if graph.Stop != nil && graph.Stop.Condition != "" {
      result, err := EvalConditionDynamic(ctx, graph.Stop.Condition, state, graph)
      if err != nil {
          slog.Warn("stop condition eval failed", "error", err)
      } else if result.Met {
          // C3-1: Write stopped to DB BEFORE returning sentinel
          if stopErr := s.store.StopSkillExecution(ctx, exec.ID); stopErr != nil {
              slog.Error("failed to write stopped status", "exec_id", exec.ID, "error", stopErr)
          }
          // Emit stop event
          if graph.Stop.EmitEvent != "" && exec.TenantID != nil {
              s.dispatchEvent(ctx, *exec.TenantID, exec.ConversationID, graph.Stop.EmitEvent, "")
          }
          return errStopSignal
      }
  }
  ```

- In `runDetachedExecution`:
  ```go
  if err := s.executeWorkflow(ctx, exec, graph, s.skillRegistry); err != nil {
      if errors.Is(err, errStopSignal) {
          slog.Info("workflow stopped by signal", "exec_id", exec.ID)
          return  // already written by executeWorkflow
      }
      s.failExecution(ctx, exec, err.Error())
      return
  }
  ```

**Files modified:**
1. `server/router/api/v1/agent/execution.go` — stop write + sentinel + logging

**Verification:** Test: SCRIPT with `@signal: condition: "urgency > 5"`, run with `urgency=7` → stops, status=`stopped`, no re-claim

---

### Fix 6 — GenerateFn per-execution tenant engine (C3-4)

**Scope:** 2 files

**skill_builtins.go:**
- Change `LLMHandler.GenerateFn` signature:
  ```go
  GenerateFn func(ctx context.Context, tenantID *int32, prompt string, vars map[string]any) (string, error)
  ```

**service.go — GenerateFn wiring in NewService:**
```go
llmHandler := &LLMHandler{
    GenerateFn: func(ctx context.Context, tenantID *int32, prompt string, vars map[string]any) (string, error) {
        if tenantID == nil {
            return "", fmt.Errorf("llm_call: tenant_id required")
        }
        model, apiKey, err := svc.requireLLMConfig(ctx, int(*tenantID))
        if err != nil {
            return "", fmt.Errorf("llm_call: resolve config: %w", err)
        }
        client := newOpenRouterClient(apiKey)
        resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
            Model: model,
            Messages: []openrouter.ChatCompletionMessage{
                {Role: openrouter.ChatMessageRoleSystem, Content: openrouter.Content{Text: prompt}},
            },
        })
        if err != nil {
            return "", fmt.Errorf("llm_call: %w", err)
        }
        if len(resp.Choices) == 0 {
            return "", fmt.Errorf("llm_call: no response")
        }
        return resp.Choices[0].Message.Content.Text, nil
    },
}
svc.skillRegistry.Register(llmHandler)
```

**execution.go — executeStep:**
- Pass tenant ID to handler via params for `builtin:llm_call`:
  ```go
  if executorType == "builtin" && code == "llm_call" {
      if exec.TenantID != nil {
          node.Params["tenant_id"] = strconv.Itoa(int(*exec.TenantID))
      }
  }
  ```
  Or: pass `exec.TenantID` via the `vars` map.

**Files modified:**
1. `server/router/api/v1/agent/skill_builtins.go` — GenerateFn signature
2. `server/router/api/v1/agent/service.go` — wire GenerateFn with tenant resolution

**Verification:** Test: multi-tenant, `llm_call` step → uses correct tenant's model/API key

---

### Fix 7 — Recovery worker no double-claim (L-1)

**Scope:** 1 file

**recovery.go:**
- Current code is actually correct (no pre-claim). Just verify and clean up:
  ```go
  func (s *Service) recoverPendingExecutions(ctx context.Context) {
      pending, err := s.store.ListPendingSkillExecutions(ctx)
      if err != nil { ... }
      for _, exec := range pending {
          graph := &SkillGraph{}
          if err := json.Unmarshal([]byte(exec.SkillGraphJSON), graph); err != nil {
              s.store.FailSkillExecution(ctx, exec.ID, "recovery: deserialize failed")
              continue
          }
          if !graph.HasSkills { continue }
          go s.runDetachedExecution(ctx, exec)  // claims internally
      }
  }
  ```
- Use `FailSkillExecution` (Fix 3) instead of `s.failExecution` for consistency.

**Files modified:**
1. `server/router/api/v1/agent/recovery.go` — use new store method

**Verification:** Test: recovery picks pending → runDetachedExecution claims → executes

---

### Fix 8 — current_node stores node name, not output (MED-1)

**Scope:** 2 files

**checkpoint.go:**
- `writeCheckpoint(ctx, exec, state, nodeName string)` — change signature from `output` to `nodeName`
- `exec.CurrentNode = nodeName`

**execution.go:**
- Call `s.writeCheckpoint(ctx, exec, state, nodeName)` — already passes nodeName in code3

**Files modified:**
1. `server/router/api/v1/agent/checkpoint.go` — signature change
2. `server/router/api/v1/agent/execution.go` — already correct, verify

**Verification:** `current_node` = `classify_intent`, not `{"intent":"emergency"}`

---

### Fix 9 — MySQL gating (C3-7)

**Scope:** 2 files

**v1.go** (C3-7):
- Use `apiv1Service.Profile.Driver` not `os.Getenv`:
  ```go
  if apiv1Service.Profile.Driver != "mysql" {
      authGroup.POST("/:slug/workflows/start", s.agentHandler.HandleStartWorkflow)
      // ... other routes
  }
  ```

**recovery.go** (C3-7):
- Use `s.profile.Driver`:
  ```go
  if s.profile.Driver == "mysql" {
      return
  }
  ```

**Files modified:**
1. `server/router/api/v1/v1.go` — conditional routes on profile driver
2. `server/router/api/v1/agent/recovery.go` — skip on mysql

**Verification:** MySQL → workflow routes 404, no recovery worker

---

### Fix 10 — Minor fixes

**execution.go — duration tracking (C3-6):**
- `logSkillStep(ctx, exec, node, output string, start time.Time)`
- `DurationMs: int(time.Since(start).Milliseconds())` — use `int`, not `int32`

**service.go — SECTION 7B filtering (LOW-3):**
- Only list skills with registered handlers:
  ```go
  for name, skill := range config.SkillGraph.Nodes {
      executorType, code := parseHandler(skill.Handler)
      if executorType == "condition" || executorType == "handler" { continue }
      if _, ok := s.skillRegistry.Get(skill.Handler); !ok {
          if _, ok = s.skillRegistry.Get("builtin:" + code); !ok { continue }
      }
      // include in prompt
  }
  ```

**Files modified:**
1. `server/router/api/v1/agent/execution.go` — duration fix
2. `server/router/api/v1/agent/service.go` — SECTION 7B filter

---

## 3. Test Plan

### New tests (expanded from code3)

| Test | Catches | Priority |
|------|---------|----------|
| Postgres round-trip: create → get → update → list | CRITICAL-1 | P0 |
| Stop-during-step race: stop while step running → `stopped` not `failed` | CRITICAL-3/C3-1 | P0 |
| `@signal` matched → DB status `stopped` + no re-claim by recovery | C3-1 | P0 |
| List executions: tenant A sees only A's rows | CRITICAL-2 | P0 |
| CEL dynamic: `search_kb.found == false` evaluates with no runtime key error | C3-2 | P0 |
| `FailSkillExecution` with row already `stopped` → 0-row no-op | C3-3 | P0 |
| `StopSkillExecution` leaves `completed` rows untouched | C3-3 | P0 |
| Float64 → int coercion: `urgency > 3` with API initial_vars | C3-5 | P0 |
| Recovery: picks pending → runDetachedExecution claims → executes | L-1 | P0 |
| Postgres round-trip for `CreateSkillLog`/`ListSkillLogs` (JSONB + TIMESTAMPTZ) | C3-8 | P0 |
| `int64`→`time.Time` JSON shape in `GET /executions/:id` | Fix 1 side effect | P1 |
| Stop sentinel: `errStopSignal` → DB shows `stopped` | H-3 | P1 |
| LLM handler: tenant-specific model/API key | C3-4 | P1 |
| current_node: stores node name not output | MED-1 | P1 |
| MySQL gating: workflow routes 404 on MySQL | C3-7 | P1 |
| DurationMs: non-zero actual duration | C3-6 | P1 |

### Existing tests to verify

- `parser_skill_test.go` — 8 tests (unchanged)
- `skill_test.go` — 8 tests (unchanged)
- `evaluator_test.go` — 10 tests (add dynamic variants)
- `skill_builtins_test.go` — 8 tests (update GenerateFn signature)
- `execution_test.go` — 9 tests (add stop + structured output)

### Manual verification checklist

- [ ] `go build ./...` clean
- [ ] `go test ./store/... ./server/router/api/v1/agent/...` all pass
- [ ] `task validate:parity` — all 4 drivers match schema
- [ ] Stop mid-step → DB shows `stopped`, not `failed`
- [ ] `@signal` matched → DB shows `stopped`, no re-claim
- [ ] List executions → tenant-scoped only
- [ ] CEL `search_kb.found == false` compiles + evaluates
- [ ] Float64 initial_vars → CEL eval succeeds
- [ ] `current_node` = node name, not output
- [ ] `llm_call` step uses tenant's model/key
- [ ] Recovery worker picks pending without double-claim
- [ ] `DurationMs` > 0 for actual steps
- [ ] MySQL driver → workflow routes 404

---

## 4. Implementation Order

| Step | Fix | Files | Est. LOC | Depends On |
|------|-----|-------|----------|------------|
| 1 | Fix 1 — DDL/types + LATEST.sql + C3-8 | 10 files | ~180 | — |
| 2 | Fix 3 — Atomic terminal guards + C3-3 store API | 7 files | ~150 | Fix 1 |
| 3 | Fix 2 — ListSkillExecutions (CRITICAL-2) | 6 files | ~100 | Fix 1 |
| 4 | Fix 4 — CEL dynamic env + output contract (C3-2) | 3 files | ~120 | — |
| 5 | Fix 5 — Stop write + sentinel (C3-1) | 1 file | ~50 | Fix 3, Fix 4 |
| 6 | Fix 7 — Recovery cleanup | 1 file | ~15 | Fix 3 |
| 7 | Fix 6 — GenerateFn tenant engine (C3-4) | 2 files | ~60 | — |
| 8 | Fix 8 — current_node stores name | 2 files | ~10 | — |
| 9 | Fix 9 — MySQL gating (C3-7) | 2 files | ~20 | — |
| 10 | Fix 10 — Minor fixes (C3-6) | 2 files | ~20 | — |
| 11 | New tests | 3-5 files | ~250 | All above |
| | **Total** | | **~975** | |

---

## 5. Plan6 Conformance (post-fix)

| plan6 requirement | Status after code4 |
|-------------------|---------------------|
| D2 — chat path durable loop | DEFERRED (HIGH-2) — documented gap, not blocking hackathon |
| R3 — status re-read guards | FIXED (Fix 3) — atomic store methods + fresh context |
| D4 — CEL env + output semantics | FIXED (Fix 4) — parsed JSON output + dynamic env |
| D5/R2 — recovery worker | FIXED (Fix 7) — no double-claim, uses new store methods |
| D6 — outbound events | Already working |
| MaxRetries/Timeout honored | DEFERRED — parsed but not driving retry logic |
| §10 cadence (10s+60s) vs 30s ticker | Minor deviation, acceptable |
| §11 endpoints + RBAC | Already working + Fix 9 gating |

---

## 6. Files Modified (complete manifest)

### Store layer
1. `store/agent.go` — SkillExecution/SkillLog time.Time + 3 new wrapper methods
2. `store/driver.go` — +ListSkillExecutions + CompleteSkillExecution + FailSkillExecution + StopSkillExecution
3. `store/db/sqlite/agent_skill.go` — time conversion helpers + all CRUD + 3 new methods + log methods
4. `store/db/postgres/agent_skill.go` — JSONB fix + time pass-through + 3 new methods + log methods
5. `store/db/mysql/agent_skill.go` — stubs for all new methods

### Migrations
6. `store/migration/postgres/0.36/00__add_skill_executions.sql` — PG dialect
7. `store/migration/postgres/Latest.sql` — PG dialect skill tables
8. `store/migration/cockroach/Latest.sql` — verify CRDB dialect
9. `store/migration/sqlite/Latest.sql` — verify SQLite dialect
10. `store/migration/mysql/Latest.sql` — verify MySQL dialect

### Agent engine
11. `server/router/api/v1/agent/evaluator.go` — EvalConditionDynamic + dynamic vars
12. `server/router/api/v1/agent/checkpoint.go` — use atomic store methods + current_node fix
13. `server/router/api/v1/agent/execution.go` — parsed output + stop write + sentinel + fresh context + duration
14. `server/router/api/v1/agent/recovery.go` — use FailSkillExecution + mysql gating
15. `server/router/api/v1/agent/skill_builtins.go` — tenant-aware GenerateFn signature
16. `server/router/api/v1/agent/service.go` — wire GenerateFn + SECTION 7B filter

### API
17. `server/router/api/v1/v1.go` — conditional routes on profile driver
18. `server/router/api/v1/agent/handlers.go` — HandleListExecutions fix

### Tests
19. `server/router/api/v1/agent/evaluator_test.go` — add dynamic CEL tests
20. `server/router/api/v1/agent/execution_test.go` — add stop + structured output tests
21. `server/router/api/v1/agent/skill_builtins_test.go` — update for new GenerateFn signature
22. `store/db/sqlite/agent_skill_test.go` — (new) time round-trip tests
23. `store/db/postgres/agent_skill_test.go` — (new) round-trip tests (integration tag)

---

## 7. Bottom Line

- **code3.md had 4 fatal plan defects** (C3-1 through C3-4) plus 5 nits — all incorporated.
- **code4.md** provides corrected versions: atomic store API, parsed output contract, stop-write-before-sentinel, per-execution tenant engine.
- **Estimated effort:** ~975 LOC across ~30 files, plus ~250 LOC of tests.
- **Implementation order:** DDL/types first (blocks everything), then atomic guards + store API, then tenant isolation, then CEL, then minor fixes.
- **Hackathon readiness:** After code4.md is implemented, the durable execution pipeline works on SQLite, PostgreSQL, and CockroachDB with correct tenant isolation, stop semantics, CEL conditions, and structured output parsing.

---

**This plan is ready for implementation.** All code3_review.md findings are incorporated. The plan maintains plan6 conformance while fixing every identified defect.
