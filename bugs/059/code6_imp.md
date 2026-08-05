# code6.md Implementation Review Checklist

**Purpose:** Checklist for a coding agent to verify the implementation against all findings from the 5-review chain.
**Plan source:** `code6.md` (APPROVED — final plan)
**Review chain:** code_review.md → code2_review.md → code3_review.md → code4_review.md → code5_review.md
**Scope:** 10 fixes, 26 files modified, ~1060 LOC + ~280 LOC tests

---

## 1. Review Finding Checklist

### CRITICAL Findings

#### CRITICAL-1 (code_review) — DDL/Go type mismatch
**Fixed in:** Fix 1
- [ ] `store/db/postgres/agent_skill.go` — All timestamps use `time.Time` (not `int64` epoch) for TIMESTAMPTZ columns
- [ ] `store/db/sqlite/agent_skill.go` — Time helpers (`timeToUnix`/`unixToTime`/`nullInt64ToTime`) convert `time.Time` ↔ `int64` epoch
- [ ] `store/db/postgres/agent_skill.go` — JSON columns (`checkpoint_data`, `completed_nodes`, `failed_nodes`) passed as raw `[]byte` (JSONB), NOT `string(checkpointJSON)`
- [ ] `store/agent.go` — `SkillExecution` struct uses `time.Time` for `CreatedAt`, `UpdatedAt`; `*time.Time` for `CompletedAt`, `ClaimedAt`, `ClaimExpiresAt`
- [ ] `store/agent.go` — `SkillLog` struct uses `time.Time` for `StartedAt`; `*time.Time` for `CompletedAt`
- [ ] `store/db/postgres/agent_skill.go` — `ClaimSkillExecution` uses `time.Now()` and `time.Duration(leaseSeconds) * time.Second` (NOT `time.Now().Unix()`) for TIMESTAMPTZ args

#### CRITICAL-1b (code_review) — Postgres migration CockroachDB dialect
**Fixed in:** Fix 1
- [ ] `store/migration/postgres/0.36/00__add_skill_executions.sql` — Uses `TEXT` (not `STRING`), `BIGINT` (not `INT8`), `INTEGER` (not `INT4`)
- [ ] `store/migration/postgres/LATEST.sql` — Same dialect fixes applied
- [ ] `store/migration/cockroach/0.36/00__add_skill_executions.sql` — Correctly uses CockroachDB dialect (`STRING`, `INT8`, `INT4`) — NOT modified to Postgres dialect
- [ ] `store/migration/cockroach/LATEST.sql` — Correctly uses CockroachDB dialect

#### CRITICAL-2 (code_review) — Cross-tenant data leak
**Fixed in:** Fix 2
- [ ] `store/driver.go` — `ListSkillExecutions(ctx, find, limit)` exists with `FindSkillExecution` containing `TenantID *int32`
- [ ] `store/db/sqlite/agent_skill.go` — `ListSkillExecutions` WHERE clause includes `tenant_id = ?` when `find.TenantID != nil`
- [ ] `store/db/postgres/agent_skill.go` — `ListSkillExecutions` WHERE clause includes `tenant_id = $N` when `find.TenantID != nil`
- [ ] `server/router/api/v1/agent/checkpoint.go` — `listExecutionsByTenant` delegates to `s.store.ListSkillExecutions(ctx, find, limit)` (NOT `ListPendingSkillExecutions`)

#### CRITICAL-3 (code_review) — Stop can never produce `stopped` state
**Fixed in:** Fix 3 + Fix 5
- [ ] `server/router/api/v1/agent/execution.go` — `errStopSignal` sentinel defined at package level
- [ ] `server/router/api/v1/agent/execution.go` — `executeWorkflow` evaluates `graph.Stop.Condition` after each step, calls `s.store.StopSkillExecution(ctx, exec.ID)` BEFORE returning `errStopSignal`
- [ ] `server/router/api/v1/agent/execution.go` — `runDetachedExecution` checks `errors.Is(execErr, errStopSignal)` and returns early (does NOT call `failExecution`)
- [ ] `server/router/api/v1/agent/execution.go` — `runDetachedExecution` re-reads status with `context.Background()` (NOT cancelled `ctx`) before deciding fail vs stop

### HIGH Findings

#### HIGH-1 (code_review) — CEL condition pattern cannot compile
**Fixed in:** Fix 4
- [ ] `server/router/api/v1/agent/evaluator.go` — `EvalConditionDynamic` function exists
- [ ] `server/router/api/v1/agent/evaluator.go` — `EvalConditionDynamic` uses `cel.DynType` for ALL variables (standard + graph node names)
- [ ] `server/router/api/v1/agent/evaluator.go` — `isMissingKeyError` helper checks for "no such key", "missing variable", "undeclared identifier"
- [ ] `server/router/api/v1/agent/execution.go` — `executeWorkflow` stop condition uses `EvalConditionDynamic(ctx, graph.Stop.Condition, celVars, graph)`
- [ ] `server/router/api/v1/agent/execution.go` — `executeStep` condition uses `EvalConditionDynamic(ctx, node.Condition, celVars, graph)`

#### HIGH-2 (code_review) — Chat path not durable
**Status:** DEFERRED (non-blocking per code6.md). Not implemented in this phase.

#### HIGH-3 (code_review) — `llm:` delegation non-functional
**Fixed in:** Fix 6
- [ ] `server/router/api/v1/agent/service.go` — `GenerateFn` is wired on `LLMHandler` after `RegisterBuiltins` (NOT nil)
- [ ] `server/router/api/v1/agent/service.go` — `GenerateFn` closure resolves tenant ID from `vars["tenant_id"]` (handles float64/int/int32/string types)
- [ ] `server/router/api/v1/agent/service.go` — `GenerateFn` calls `svc.requireLLMConfig(ctx, tenantID)` for per-tenant model+key
- [ ] `server/router/api/v1/agent/service.go` — `GenerateFn` creates OpenRouter client and calls `CreateChatCompletion`

#### HIGH-4 (code_review) — List endpoint returns wrong data
**Fixed in:** Fix 2
- [ ] `server/router/api/v1/agent/checkpoint.go` — `listExecutionsByTenant` passes `limit` to `s.store.ListSkillExecutions`
- [ ] `store/db/sqlite/agent_skill.go` — `ListSkillExecutions` applies `LIMIT ?` with the passed limit value
- [ ] `store/db/postgres/agent_skill.go` — `ListSkillExecutions` applies `LIMIT $N` with the passed limit value

#### HIGH-5 (code_review) — `@signal`/`@trigger` are parse-only dead code
**Fixed in:** Fix 3 + Fix 5
- [ ] `server/router/api/v1/agent/execution.go` — `executeWorkflow` reads `graph.Stop` and evaluates its `Condition` field
- [ ] `server/router/api/v1/agent/execution.go` — Stop condition evaluation happens after each checkpointed step

### MEDIUM Findings

#### MED-1 (code_review) — `current_node` stores full output, not node name
**Fixed in:** Fix 8
- [ ] `server/router/api/v1/agent/checkpoint.go` — `writeCheckpoint` parameter named `nodeName` (not `output`), assigned to `exec.CurrentNode = nodeName`

#### MED-2 (code_review) — Postgres `ListSkillLogs` JSONB scan error
**Fixed in:** Fix 1
- [ ] `store/db/postgres/agent_skill.go` — `ListSkillLogs` scans JSONB into `[]byte` then `json.Unmarshal` (NOT directly into `map[string]any`)

#### MED-3 (code_review) — `max_retries`/`retry_count` unused
**Status:** DEFERRED (non-blocking per code6.md). Fields exist in schema but retry logic not implemented.

#### MED-4 (code_review) — No graceful shutdown for goroutines
**Status:** DEFERRED (non-blocking per code6.md).

#### MED-5 (code_review) — `failExecution` stores error in checkpoint data
**Fixed in:** Fix 1 + Fix 2
- [ ] `server/router/api/v1/agent/checkpoint.go` — `failExecution` delegates to `s.store.FailSkillExecution(ctx, exec.ID, errMsg)` (NOT `UpdateSkillExecution` with manual checkpoint mutation)
- [ ] `store/db/sqlite/agent_skill.go` — `FailSkillExecution` writes `error_message` column (NOT `checkpoint_data["error"]`)

#### MED-6 (code_review) — Stop on terminal executions unconditional
**Fixed in:** Fix 2 + Fix 9
- [ ] `store/db/sqlite/agent_skill.go` — `StopSkillExecution` WHERE clause includes `status IN ('pending', 'running')` (no-op on completed/failed)
- [ ] `store/db/sqlite/agent_skill.go` — `FailSkillExecution` WHERE clause includes `status NOT IN ('stopped', 'completed')` (no-op on stopped/completed)
- [ ] `store/db/sqlite/agent_skill.go` — `CompleteSkillExecution` WHERE clause includes `status = 'running'` (no-op on stopped/failed)
- [ ] `store/db/postgres/agent_skill.go` — Same conditional WHERE clauses in all three atomic methods

### LOW Findings

#### LOW-1 (code_review) — `logSkillStep` hardcodes `DurationMs: 0`
**Fixed in:** Fix 10
- [ ] `server/router/api/v1/agent/execution.go` — `logSkillStep` computes `DurationMs: int(time.Since(start).Milliseconds())` (NOT hardcoded 0)

#### LOW-2 (code_review) — CEL env compiled fresh every evaluation
**Status:** DEFERRED (non-blocking per code6.md).

#### LOW-3 (code_review) — SECTION 7B lists unregistered handlers
**Fixed in:** Fix 5 (SECTION 7B filter)
- [ ] `server/router/api/v1/agent/service.go` — SECTION 7B skips nodes where `parseHandler(node.Handler)` returns `executorType == "condition"` (NOT listing condition nodes as callable tools)

#### LOW-4 (code_review) — Recovery deserializes full graph for every row
**Status:** DEFERRED (non-blocking per code6.md).

---

### K-Items (code4_review)

#### K-1 — `error_message` column missing from DDL
**Fixed in:** Fix 1
- [ ] `store/migration/sqlite/0.36/00__add_skill_executions.sql` — `error_message TEXT DEFAULT ''` in `agent_skill_executions`
- [ ] `store/migration/postgres/0.36/00__add_skill_executions.sql` — `error_message TEXT DEFAULT ''` in `agent_skill_executions`
- [ ] `store/migration/cockroach/0.36/00__add_skill_executions.sql` — `error_message STRING DEFAULT ''` in `agent_skill_executions`
- [ ] `store/migration/mysql/0.26/00__add_skill_executions.sql` — `error_message TEXT DEFAULT ''` in `agent_skill_executions`
- [ ] `store/migration/sqlite/LATEST.sql` — `error_message TEXT DEFAULT ''` present
- [ ] `store/migration/postgres/LATEST.sql` — `error_message TEXT DEFAULT ''` present
- [ ] `store/migration/cockroach/LATEST.sql` — `error_message STRING DEFAULT ''` present
- [ ] `store/migration/mysql/LATEST.sql` — `error_message TEXT DEFAULT ''` present
- [ ] `store/agent.go` — `ErrorMessage string` field on `SkillExecution` struct
- [ ] All SELECT lists in sqlite/postgres `agent_skill.go` include `error_message`

#### K-2 — checkpoint.go still uses `time.Now().Unix()` after struct changes
**Fixed in:** Fix 1
- [ ] `server/router/api/v1/agent/checkpoint.go` — `writeCheckpoint` uses `time.Now()` for `UpdatedAt` (NOT `time.Now().Unix()`)
- [ ] `server/router/api/v1/agent/checkpoint.go` — `createExecution` uses `time.Now()` for `CreatedAt`/`UpdatedAt` (NOT `time.Now().Unix()`)

#### K-3 — `GenerateFn` not scoped to chat path
**Fixed in:** Fix 6
- [ ] `server/router/api/v1/agent/service.go` — `toolCallingLoop` injects `args["tenant_id"] = strconv.Itoa(int(config.TenantID))` before `h.Execute`

#### K-4 — Absent-node CEL semantics hard-error
**Fixed in:** Fix 4
- [ ] `server/router/api/v1/agent/evaluator.go` — `EvalConditionDynamic` catches missing-key errors via `isMissingKeyError` and returns `Met=false`

#### K-5 — Recovery leaks skill-less pending rows
**Fixed in:** Fix 7
- [ ] `server/router/api/v1/agent/recovery.go` — `recoverPendingExecutions` calls `s.failExecution(ctx, exec, "recovery: no skills in graph")` for `!graph.HasSkills` (NOT `continue`)

#### K-6 — Casing `Latest.sql` vs `LATEST.sql`
**Fixed in:** Fix 1
- [ ] All file references use `LATEST.sql` (uppercase)

---

### N5-Items (code5_review)

#### N5-1 — Postgres claim timestamps use int64
**Fixed in:** Fix 1
- [ ] `store/db/postgres/agent_skill.go` — `ClaimSkillExecution` uses `time.Now()` and `time.Now().Add(time.Duration(leaseSeconds) * time.Second)` for TIMESTAMPTZ args (NOT `time.Now().Unix()`)

#### N5-2 — Missing-field CEL eval still errors
**Fixed in:** Fix 4
- [ ] `server/router/api/v1/agent/evaluator.go` — `EvalConditionDynamic` registers ALL graph node names as `cel.DynType` variables
- [ ] `server/router/api/v1/agent/evaluator.go` — `isMissingKeyError` catches runtime missing-field errors

#### N5-3 — `toolCallingLoop` doesn't inject tenant
**Fixed in:** Fix 6
- [ ] `server/router/api/v1/agent/service.go` — `toolCallingLoop` sets `args["tenant_id"]` from `config.TenantID` before handler execution

#### N5-4 — `ErrorMessage` missing from SELECT lists
**Fixed in:** Fix 1
- [ ] `store/db/sqlite/agent_skill.go` — `ListSkillExecutions` SELECT includes `error_message`
- [ ] `store/db/sqlite/agent_skill.go` — `ListSkillLogs` SELECT includes `error_message`
- [ ] `store/db/postgres/agent_skill.go` — `ListSkillExecutions` SELECT includes `error_message`
- [ ] `store/db/postgres/agent_skill.go` — `ListSkillLogs` SELECT includes `error_message`

#### N5-5 — Postgres integration test gating
**Fixed in:** Fix 1
- [ ] Postgres integration tests (if present) use `testing.Short()` skip with `SKILL_PG_INTEGRATION_TEST=true` opt-in

---

### C3-Items (code3_review)

#### C3-1 — Stop never writes `stopped`
**Fixed in:** Fix 3 + Fix 5
- [ ] Stop condition in `executeWorkflow` calls `s.store.StopSkillExecution(ctx, exec.ID)` BEFORE returning `errStopSignal`

#### C3-2 — Output wrapper doesn't satisfy D4
**Fixed in:** Fix 4
- [ ] `EvalConditionDynamic` uses `cel.DynType` — any field access on any type is valid at compile time

#### C3-3 — No store API for atomic guards
**Fixed in:** Fix 2
- [ ] `store/driver.go` — `CompleteSkillExecution`, `FailSkillExecution`, `StopSkillExecution` exist
- [ ] `store/db/sqlite/agent_skill.go` — All three have conditional WHERE clauses with `RowsAffected==0` → log+nil
- [ ] `store/db/postgres/agent_skill.go` — Same

#### C3-4 — Fix 6 tenant injection targets wrong handler type
**Fixed in:** Fix 6
- [ ] Injection happens in `toolCallingLoop` (generic), NOT in `executeStep` handler-type switch

#### C3-5 — Invalid Go numeric coercion
**Fixed in:** Fix 4
- [ ] `EvalConditionDynamic` does not use `if f, ok := v.(float64); ok == float64(int(f))` pattern

#### C3-6 — `DurationMs` type mismatch
**Fixed in:** Fix 10
- [ ] `logSkillStep` computes `int(time.Since(start).Milliseconds())` — matches `SkillLog.DurationMs` type (`int`)

#### C3-7 — MySQL gating keyed on env var, not profile
**Fixed in:** Fix 9
- [ ] `server/router/api/v1/v1.go` — Routes gated on `s.Profile.Driver != "mysql"` (NOT `os.Getenv("MEMOS_DRIVER")`)
- [ ] `server/router/api/v1/agent/service.go` — Recovery worker gated on `svc.profile.Driver != "mysql"`

#### C3-8 — Timestamp conversions missing from `logSkillStep`
**Fixed in:** Fix 1
- [ ] `server/router/api/v1/agent/execution.go` — `logSkillStep` accepts `start time.Time`, computes duration in ms

#### C3-9 — No `store/db/cockroach/` directory
**Status:** Correct — CockroachDB served by postgres driver via `NewCockroachDB`. CockroachDB DDLs correctly use CockroachDB dialect.

---

## 2. File-by-File Review Guide

### Store Layer

| File | Lines | Review Focus |
|------|-------|-------------|
| `store/agent.go` | 1452 | `SkillExecution`/`SkillLog` struct field types (time.Time vs int64); `ErrorMessage` present on both structs; 12 wrapper methods delegate correctly |
| `store/driver.go` | 316 | 12 new interface methods: signatures match implementations in sqlite/postgres/mysql |
| `store/db/sqlite/agent_skill.go` | 423 | Time conversion helpers correct; `nullInt64ToTime` handles `sql.NullInt64`; `ErrorMessage` in all INSERT/UPDATE/SELECT; atomic methods have conditional WHERE |
| `store/db/postgres/agent_skill.go` | 350 | JSONB `[]byte` pass-through (no `string()` wrapper); `time.Time` for TIMESTAMPTZ; `ClaimSkillExecution` uses `time.Now()` not `.Unix()`; `NOW()` in atomic methods |
| `store/db/mysql/agent_skill.go` | 59 | All 12 methods return `errors.New("not implemented for MySQL")` |

### Migrations

| File | Review Focus |
|------|-------------|
| `store/migration/postgres/0.36/00__add_skill_executions.sql` | Postgres dialect: `TEXT`/`BIGINT`/`INTEGER` (NOT `STRING`/`INT8`/`INT4`); `error_message` present |
| `store/migration/cockroach/0.36/00__add_skill_executions.sql` | CockroachDB dialect: `STRING`/`INT8`/`INT4`; `error_message` present |
| `store/migration/sqlite/0.36/00__add_skill_executions.sql` | `error_message TEXT DEFAULT ''` present |
| `store/migration/mysql/0.26/00__add_skill_executions.sql` | `error_message TEXT DEFAULT ''` present |
| All 4 `LATEST.sql` | `error_message` column present in both tables; correct dialect per driver |

### Agent Engine

| File | Lines | Review Focus |
|------|-------|-------------|
| `server/router/api/v1/agent/checkpoint.go` | 123 | `writeCheckpoint` re-reads status before writing (R3); `completeExecution`/`failExecution`/`stopExecution` delegate to atomic store methods; timestamps use `time.Now()` |
| `server/router/api/v1/agent/execution.go` | 379 | `errStopSignal` sentinel; stop condition check calls `StopSkillExecution` before returning; `runDetachedExecution` re-reads with `context.Background()`; `logSkillStep` computes duration |
| `server/router/api/v1/agent/evaluator.go` | 140 | `EvalConditionDynamic` uses `cel.DynType`; `isMissingKeyError` catches missing keys; graph node names registered as variables |
| `server/router/api/v1/agent/recovery.go` | 78 | Both deserialize-failure and skill-less paths call `FailSkillExecution`; gated on `SKILL_RECOVERY_ENABLED` env var |
| `server/router/api/v1/agent/service.go` | 1410+ | `GenerateFn` wired post-`RegisterBuiltins`; `toolCallingLoop` injects `tenant_id`; SECTION 7B skips condition nodes; recovery worker gated on `profile.Driver != "mysql"` |
| `server/router/api/v1/agent/skill_builtins.go` | 165 | `LLMHandler.GenerateFn` field exists; `Execute` passes prompt+vars to callback |

### API

| File | Review Focus |
|------|-------------|
| `server/router/api/v1/v1.go` | Workflow routes gated on `s.Profile.Driver != "mysql"` |
| `server/router/api/v1/agent/handlers.go` | `HandleListExecutions` uses `s.store.ListSkillExecutions` with proper tenant filter |

### Tests

| File | Review Focus |
|------|-------------|
| `server/router/api/v1/agent/execution_test.go` | `executeStepHelper` passes nil graph as 6th arg |
| `server/router/api/v1/agent/skill_builtins_test.go` | Tests exercise `GenerateFn` callback |

---

## 3. Verification Checklist

### Automated
```
go build ./...                    → expect clean (no errors)
go test ./store/...               → expect all pass
go test ./server/router/api/v1/agent/...  → expect all pass
task validate:schema              → expect PASS
task validate:parity              → expect PASS
```

### Manual Spot-Checks
- [ ] Stop mid-step → execution status is `stopped` (not `failed`)
- [ ] `@signal` condition matched → status is `stopped`, no re-claim by recovery
- [ ] List executions → returns only current tenant's executions
- [ ] CEL `search_kb.found == false` evaluates without error
- [ ] Skipped node field condition → clean `false` (not error)
- [ ] Float64 `initial_vars` → CEL eval succeeds
- [ ] `current_node` = node name (not full output text)
- [ ] `llm_call` uses tenant's model/key (not env default)
- [ ] Recovery handles skill-less rows (marks failed, doesn't loop)
- [ ] `error_message` populated on failure
- [ ] MySQL driver → workflow routes return 404
- [ ] Postgres claim produces no type error
- [ ] `DurationMs` > 0 for actual steps

---

## 4. Adversarial Code Review Prompt

Use this prompt with an independent coding agent to verify the implementation:

```
You are a senior Go security and correctness reviewer. Your job is to find defects
that the implementation team may have missed.

CONTEXT:
- The codebase is a multi-tenant AI chat agent platform (bchat)
- A durable automation pipeline was implemented per plan code6.md
- The plan was reviewed 5 times (code_review → code2_review → code3_review → code4_review → code5_review)
- All review findings were incorporated into the final implementation

YOUR TASK:
For each file listed below, read the file and verify the checklist items. Report any
defects you find with severity (Critical/High/Medium/Low) and exact file:line reference.

FILES TO REVIEW:
1. store/agent.go — SkillExecution/SkillLog structs, wrapper methods
2. store/driver.go — Interface methods
3. store/db/sqlite/agent_skill.go — Full SQLite implementation
4. store/db/postgres/agent_skill.go — Full Postgres implementation
5. store/db/mysql/agent_skill.go — Stubs
6. server/router/api/v1/agent/checkpoint.go — Checkpoint management
7. server/router/api/v1/agent/execution.go — Workflow execution engine
8. server/router/api/v1/agent/evaluator.go — CEL evaluation
9. server/router/api/v1/agent/recovery.go — Recovery worker
10. server/router/api/v1/agent/service.go — Service wiring, GenerateFn, SECTION 7B
11. server/router/api/v1/v1.go — Route registration
12. store/migration/postgres/0.36/00__add_skill_executions.sql — Postgres DDL
13. store/migration/cockroach/0.36/00__add_skill_executions.sql — CockroachDB DDL

VERIFICATION CHECKLIST (verify ALL items):
- [ ] CRITICAL-1: No int64 epoch values passed to TIMESTAMPTZ columns in Postgres driver
- [ ] CRITICAL-1b: Postgres DDL uses TEXT/BIGINT/INTEGER, CockroachDB uses STRING/INT8/INT4
- [ ] CRITICAL-2: ListSkillExecutions always filters by tenant_id when TenantID is set
- [ ] CRITICAL-3: Stop condition writes 'stopped' BEFORE returning errStopSignal; runDetachedExecution handles sentinel separately
- [ ] HIGH-1: EvalConditionDynamic uses cel.DynType for all variables
- [ ] HIGH-3: GenerateFn is wired (not nil) and resolves per-tenant LLM config
- [ ] HIGH-4: ListSkillExecutions applies LIMIT with passed value
- [ ] MED-1: writeCheckpoint stores node name (not output) in CurrentNode
- [ ] MED-5: failExecution uses FailSkillExecution store method (not manual checkpoint mutation)
- [ ] K-1: error_message column exists in all DDLs and LATEST.sql
- [ ] K-2: No time.Now().Unix() calls remain in checkpoint.go
- [ ] N5-1: ClaimSkillExecution uses time.Now() not .Unix() for TIMESTAMPTZ
- [ ] N5-2: EvalConditionDynamic catches missing-key errors and returns Met=false
- [ ] N5-3: toolCallingLoop injects tenant_id into args before handler execution
- [ ] N5-4: error_message included in all SELECT lists
- [ ] C3-3: Atomic methods have conditional WHERE clauses (no-op on terminal states)
- [ ] C3-7: MySQL gating uses profile.Driver (not os.Getenv)

REPORT FORMAT:
For each defect found:
- Severity: Critical/High/Medium/Low
- File: path/to/file.go:line_number
- Description: What is wrong
- Expected: What should be there instead

If no defects found, state: "All checklist items verified. No defects found."
```

---

## 5. Open Items (Deferred, Non-Blocking)

These items were acknowledged in code6.md as deferred and are NOT defects:

| Item | Description | Status |
|------|-------------|--------|
| D2 | Chat path durability (no SkillExecution created for chat-path tool calls) | Deferred |
| MED-3 | `max_retries`/`retry_count` parsed but no retry logic implemented | Deferred |
| MED-4 | No graceful shutdown for recovery worker goroutines | Deferred |
| LOW-2 | CEL env compiled fresh per evaluation (no program caching) | Deferred |
| LOW-4 | Recovery deserializes full graph JSON for every pending row | Deferred |

---

## 6. Review Chain Traceability

| Finding | Source Review | Fix | Files Changed | Status |
|---------|--------------|-----|---------------|--------|
| CRITICAL-1 | code_review | Fix 1 | store/agent.go, store/db/sqlite, store/db/postgres, checkpoint.go, execution.go | ✅ Resolved |
| CRITICAL-1b | code_review | Fix 1 | store/migration/postgres/*, store/migration/cockroach/* | ✅ Resolved |
| CRITICAL-2 | code_review | Fix 2 | store/driver.go, store/db/sqlite, store/db/postgres, store/db/mysql, checkpoint.go, handlers.go | ✅ Resolved |
| CRITICAL-3 | code_review | Fix 3+5 | execution.go, checkpoint.go | ✅ Resolved |
| HIGH-1 | code_review | Fix 4 | evaluator.go, execution.go | ✅ Resolved |
| HIGH-2 | code_review | — | — | ⏳ Deferred |
| HIGH-3 | code_review | Fix 6 | service.go, skill_builtins.go | ✅ Resolved |
| HIGH-4 | code_review | Fix 2 | checkpoint.go, store/db/sqlite, store/db/postgres | ✅ Resolved |
| HIGH-5 | code_review | Fix 3+5 | execution.go | ✅ Resolved |
| MED-1 | code_review | Fix 8 | checkpoint.go | ✅ Resolved |
| MED-2 | code_review | Fix 1 | store/db/postgres | ✅ Resolved |
| MED-3 | code_review | — | — | ⏳ Deferred |
| MED-4 | code_review | — | — | ⏳ Deferred |
| MED-5 | code_review | Fix 1+2 | checkpoint.go, store/db/sqlite, store/db/postgres | ✅ Resolved |
| MED-6 | code_review | Fix 2+9 | store/db/sqlite, store/db/postgres, v1.go, service.go | ✅ Resolved |
| LOW-1 | code_review | Fix 10 | execution.go | ✅ Resolved |
| LOW-2 | code_review | — | — | ⏳ Deferred |
| LOW-3 | code_review | Fix 5 | service.go | ✅ Resolved |
| LOW-4 | code_review | — | — | ⏳ Deferred |
| K-1 | code4_review | Fix 1 | All 8 DDL/LATEST.sql + store/agent.go | ✅ Resolved |
| K-2 | code4_review | Fix 1 | checkpoint.go | ✅ Resolved |
| K-3 | code4_review | Fix 6 | service.go | ✅ Resolved |
| K-4 | code4_review | Fix 4 | evaluator.go | ✅ Resolved |
| K-5 | code4_review | Fix 7 | recovery.go | ✅ Resolved |
| K-6 | code4_review | Fix 1 | All file references | ✅ Resolved |
| N5-1 | code5_review | Fix 1 | store/db/postgres | ✅ Resolved |
| N5-2 | code5_review | Fix 4 | evaluator.go | ✅ Resolved |
| N5-3 | code5_review | Fix 6 | service.go | ✅ Resolved |
| N5-4 | code5_review | Fix 1 | store/db/sqlite, store/db/postgres | ✅ Resolved |
| N5-5 | code5_review | Fix 1 | Tests | ✅ Resolved |
| C3-1 | code3_review | Fix 3+5 | execution.go | ✅ Resolved |
| C3-2 | code3_review | Fix 4 | evaluator.go | ✅ Resolved |
| C3-3 | code3_review | Fix 2 | store/driver.go, store/db/sqlite, store/db/postgres | ✅ Resolved |
| C3-4 | code3_review | Fix 6 | service.go | ✅ Resolved |
| C3-5 | code3_review | Fix 4 | evaluator.go | ✅ Resolved |
| C3-6 | code3_review | Fix 10 | execution.go | ✅ Resolved |
| C3-7 | code3_review | Fix 9 | v1.go, service.go | ✅ Resolved |
| C3-8 | code3_review | Fix 1 | execution.go | ✅ Resolved |
