# bchat Durable Execution — Plan 5 (code5.md)

**Version:** 5.0
**Date:** 2026-08-05
**Status:** PLANNED — Ready for Implementation
**Review Source:** `bugs/059/code4_review.md` (DeepSeek adversarial review of code4.md)
**Baseline:** `plan6.md` (APPROVED) + `code4.md` + all prior reviews

---

## Executive Summary

This plan incorporates 1 High + 5 Nit findings from `code4_review.md`. code4_review is the **first approving review** — verdict: "APPROVED with nits & one rework item". With K-1 resolved and K-2…K-5 folded in, the plan is ready to ship.

**Key corrections from code4.md:**
1. **K-1:** `FailSkillExecution` SQL targets non-existent `error_message` column — add column to all DDLs
2. **K-2:** Fix 1 scope omits `checkpoint.go` timestamp assignments — build breaks
3. **K-3:** `GenerateFn` signature change breaks chat path — add fallback
4. **K-4:** Absent-node CEL semantics hard-error — write placeholders
5. **K-5:** Recovery leaks skill-less rows — fail them consistently
6. **K-6:** `Latest.sql` → `LATEST.sql` casing

**Current state:** All `code_review.md` findings remain live. No fixes applied.

---

## 1. Findings from code4_review.md

### K-1 — FailSkillExecution targets non-existent error_message column (HIGH)

- **Issue:** `FailSkillExecution` SQL uses `SET error_message=?` but `agent_skill_executions` has no `error_message` column. Column exists only on `agent_skill_logs`. Every fail path throws SQL error at runtime.
- **Decision:** Add `error_message TEXT` to `agent_skill_executions` in all four DDLs + all four LATEST.sql. Add `ErrorMessage string` to `SkillExecution` struct. Scan in `scanSkillExecutions`.
- **Status:** VALID — adopted.

### K-2 — Fix 1 breaks build as scoped (NIT)

- **Issue:** `checkpoint.go:116` (`CreatedAt: now`), `checkpoint.go:130-131` (`CreatedAt/UpdatedAt: now`), `checkpoint.go:43` (`UpdatedAt = time.Now().Unix()`) assign `int64` to fields that become `time.Time`. Package won't compile.
- **Fix:** Add `server/router/api/v1/agent/checkpoint.go` to Fix 1 scope.
- **Status:** VALID — adopted.

### K-3 — GenerateFn signature breaks chat path (NIT)

- **Issue:** New `GenerateFn(ctx, tenantID, prompt, vars)` requires tenant. Chat path `toolCallingLoop` calls `h.Execute(ctx, args, nil)` without tenant — always errors.
- **Fix:** `LLMHandler` falls back to default config when `tenant_id` param absent. Chat path is non-durable (D2 deferred), so fallback is acceptable.
- **Status:** VALID — adopted.

### K-4 — Absent-node CEL semantics hard-error (NIT)

- **Issue:** Skipped nodes have no state entry → missing var → eval error instead of clean `false`.
- **Fix:** Write `{"output": ""}` placeholder for skipped nodes in `executeWorkflow`.
- **Status:** VALID — adopted.

### K-5 — Recovery leaks skill-less pending rows (NIT)

- **Issue:** `if !graph.HasSkills { continue }` re-scans every tick forever.
- **Fix:** Call `FailSkillExecution` for skill-less rows too.
- **Status:** VALID — adopted.

### K-6 — Cosmetic casing (NIT)

- **Issue:** Plan references `Latest.sql` but files are `LATEST.sql`.
- **Fix:** Fix casing in all references.
- **Status:** VALID — adopted.

---

## 2. Revised Implementation Plan

### Fix 1 — DDL/Go type mismatch (CRITICAL-1/1b) + LATEST.sql + K-1 + K-2

**Scope:** 12 files

**Schema change (K-1):** Add `error_message TEXT` to `agent_skill_executions`:
- SQLite: `error_message TEXT DEFAULT ''`
- Postgres: `error_message TEXT DEFAULT ''`
- CockroachDB: `error_message STRING DEFAULT ''`
- MySQL: `error_message TEXT DEFAULT ''`

**Go struct changes** (`store/agent.go`):
```go
type SkillExecution struct {
    // ... existing fields ...
    ErrorMessage    string         `json:"error_message,omitempty"` // K-1
    // Timestamps -> time.Time
    ClaimedAt       *time.Time    `json:"claimed_at,omitempty"`
    ClaimExpiresAt  *time.Time    `json:"claim_expires_at,omitempty"`
    CreatedAt       time.Time     `json:"created_at"`
    UpdatedAt       time.Time     `json:"updated_at"`
    CompletedAt     *time.Time    `json:"completed_at,omitempty"`
}

type SkillLog struct {
    // ... existing fields ...
    StartedAt    time.Time     `json:"started_at"`
    CompletedAt  *time.Time    `json:"completed_at,omitempty"`
}
```

**SQLite driver** (`store/db/sqlite/agent_skill.go`):
- Helpers: `timeToUnix(*time.Time) *int64`, `unixToTime(*int64) *time.Time`
- All CRUD: convert time.Time <-> int64
- `scanSkillExecutions`: add `ErrorMessage` scan, convert int64 -> time.Time
- `CreateSkillLog`/`ListSkillLogs`: convert timestamps

**Postgres driver** (`store/db/postgres/agent_skill.go`):
- Remove `string(checkpointJSON)` wrappers — pgx handles `[]byte` <-> JSONB
- Time fields: pass `time.Time`/`*time.Time` directly
- `ListSkillLogs`: JSONB scan via `sql.NullString` + `json.Unmarshal`
- Add `ErrorMessage` to scan

**checkpoint.go (K-2) — add to Fix 1 scope:**
- `createExecution`: `CreatedAt: time.Now()`, `UpdatedAt: time.Now()` (not `.Unix()`)
- `writeCheckpoint`: `UpdatedAt = time.Now()` (not `.Unix()`)
- `completeExecution`: `now := time.Now()`, `CompletedAt = &now`
- `failExecution`: `now := time.Now()`, `CompletedAt = &now`

**execution.go:**
- `logSkillStep`: `StartedAt: time.Now()`, `DurationMs: int(time.Since(start).Milliseconds())`

**Migrations + LATEST.sql (K-6: all caps LATEST):** Add `error_message` column to all 4 versioned DDLs + all 4 LATEST.sql. Fix Postgres DDL dialect (STRING->TEXT, INT8->BIGINT, INT4->INTEGER).

**Verification:** `go build ./store/... ./server/router/api/v1/agent/...` + `go test ./store/...`

---

### Fix 2 — Cross-tenant data leak (CRITICAL-2) + List endpoint (HIGH-4)

**Scope:** 6 files

**New store method** (`store/driver.go`):
```go
ListSkillExecutions(ctx context.Context, find *FindSkillExecution, limit int) ([]*SkillExecution, error)
```

**Implementation:** Expose existing private `listSkillExecutions` in sqlite/postgres as public. Stub in mysql.

**Service layer:** Replace `listExecutionsByTenant`/`listExecutions` with `s.store.ListSkillExecutions`.

**Verification:** `go build ./...` + test with tenant filter

---

### Fix 3 — Atomic terminal guards (CRITICAL-3) + store API (C3-3)

**Scope:** 7 files

**New store methods** (`store/driver.go` + `store/agent.go` wrapper):
```go
CompleteSkillExecution(ctx context.Context, id string) error
FailSkillExecution(ctx context.Context, id string, errorMsg string) error
StopSkillExecution(ctx context.Context, id string) error
```

**SQL (per driver):**
```sql
-- CompleteSkillExecution
UPDATE agent_skill_executions SET status='completed', completed_at=?, updated_at=? WHERE id=? AND status='running'

-- FailSkillExecution (K-1: error_message column now exists)
UPDATE agent_skill_executions SET status='failed', error_message=?, completed_at=?, updated_at=? WHERE id=? AND status NOT IN ('stopped','completed')

-- StopSkillExecution
UPDATE agent_skill_executions SET status='stopped', updated_at=? WHERE id=? AND status IN ('pending','running')
```
`RowsAffected()==0` -> log + return nil.

**execution.go:** After `executeWorkflow` returns, re-read with `context.Background()`:
```go
fresh, _ := s.store.GetSkillExecution(context.Background(), &store.FindSkillExecution{ID: &exec.ID})
if fresh != nil && fresh.Status == "stopped" {
    slog.Info("workflow stopped by signal", "exec_id", exec.ID)
    return
}
```

**Verification:** Test: stop during step -> `stopped` not `failed`; complete after stop -> no-op

---

### Fix 4 — CEL dynamic env + output contract (HIGH-1/C3-2/K-4)

**Scope:** 3 files

**Canonical output contract (C3-2):**
```go
var cellValue any
var parsed map[string]any
if json.Unmarshal([]byte(output), &parsed) == nil && parsed != nil {
    cellValue = parsed
} else {
    cellValue = map[string]any{"output": output}
}
state[nodeName] = cellValue
```

**K-4 — Skipped node placeholders:**
```go
if !depsMet {
    state[nodeName] = map[string]any{"output": "", "skipped": true}
    continue
}
```

**evaluator.go:** `EvalConditionDynamic(ctx, expr, vars, graph)` — dynamic vars from graph.Nodes keys as `cel.DynType` + standard vars as `cel.DynType`. Normalize `float64->int`.

**Verification:** Test: `search_kb.found == false` evaluates; skipped node -> clean false

---

### Fix 5 — Stop signal evaluation + write + sentinel (C3-1 + HIGH-5 + H-3)

**Scope:** 1 file

**execution.go:**
```go
var errStopSignal = fmt.Errorf("workflow stopped by signal")

// In executeWorkflow, after each checkpointed step:
if graph.Stop != nil && graph.Stop.Condition != "" {
    result, err := EvalConditionDynamic(ctx, graph.Stop.Condition, state, graph)
    if err != nil {
        slog.Warn("stop condition eval failed", "error", err)
    } else if result.Met {
        if stopErr := s.store.StopSkillExecution(ctx, exec.ID); stopErr != nil {
            slog.Error("failed to write stopped", "exec_id", exec.ID, "error", stopErr)
        }
        if graph.Stop.EmitEvent != "" && exec.TenantID != nil {
            s.dispatchEvent(ctx, *exec.TenantID, exec.ConversationID, graph.Stop.EmitEvent, "")
        }
        return errStopSignal
    }
}

// In runDetachedExecution:
if errors.Is(err, errStopSignal) {
    slog.Info("workflow stopped by signal", "exec_id", exec.ID)
    return
}
```

**Verification:** Test: `@signal` matched -> status `stopped` + no re-claim

---

### Fix 6 — GenerateFn per-execution tenant engine (C3-4/K-3)

**Scope:** 2 files

**skill_builtins.go:**
```go
type LLMHandler struct {
    GenerateFn func(ctx context.Context, tenantID *int32, prompt string, vars map[string]any) (string, error)
}

func (h *LLMHandler) Execute(ctx context.Context, params map[string]string, vars map[string]any) (string, error) {
    if h.GenerateFn == nil {
        return "", fmt.Errorf("llm_call: GenerateFn not set")
    }
    prompt := params["prompt"]
    if prompt == "" { prompt = params["message"] }
    if prompt == "" { return "", fmt.Errorf("llm_call: prompt required") }

    var tenantID *int32
    if tid, ok := params["tenant_id"]; ok && tid != "" {
        if v, err := strconv.ParseInt(tid, 10, 32); err == nil {
            v32 := int32(v)
            tenantID = &v32
        }
    }

    contextData, _ := json.Marshal(vars)
    expandedPrompt := prompt + "\n\nContext:\n" + string(contextData)
    return h.GenerateFn(ctx, tenantID, expandedPrompt, vars)
}
```

**service.go — GenerateFn wiring:**
- `tenantID != nil` -> `svc.requireLLMConfig(ctx, int(*tenantID))`
- `tenantID == nil` -> `svc.requireLLMConfig(ctx, 0)` (K-3: fallback for chat path)

**execution.go:** Pass `exec.TenantID` via params for `builtin:llm_call` in detached path.

**Verification:** Test: llm_call with tenant -> correct model; chat path fallback -> no error

---

### Fix 7 — Recovery cleanup (L-1/K-5)

**Scope:** 1 file

**recovery.go (K-5):** Both deserialize-failure AND skill-less rows call `FailSkillExecution`:
```go
if !graph.HasSkills {
    s.store.FailSkillExecution(ctx, exec.ID, "recovery: no skills in graph")
    continue
}
```

**Verification:** Test: skill-less pending -> failed, not re-scanned

---

### Fix 8 — current_node stores node name (MED-1)

**Scope:** 2 files

**checkpoint.go:** `writeCheckpoint(ctx, exec, state, nodeName string)` — `exec.CurrentNode = nodeName`

---

### Fix 9 — MySQL gating (C3-7)

**Scope:** 2 files

**v1.go:** `if apiv1Service.Profile.Driver != "mysql" { ... }` for workflow routes.

**recovery.go:** `if s.profile.Driver == "mysql" { return }` before recovery worker.

---

### Fix 10 — Minor fixes

**execution.go:** Duration tracking — `int(time.Since(start).Milliseconds())` (C3-6).

**service.go:** SECTION 7B — only list skills with registered handlers.

---

## 3. Test Plan

### New tests

| Test | Catches | Priority |
|------|---------|----------|
| Postgres round-trip: create -> get -> update -> list | CRITICAL-1 | P0 |
| Stop-during-step race -> `stopped` not `failed` | CRITICAL-3 | P0 |
| `@signal` matched -> `stopped` + no re-claim | C3-1 | P0 |
| List executions: tenant-scoped only | CRITICAL-2 | P0 |
| CEL: `search_kb.found == false` evaluates | C3-2 | P0 |
| `FailSkillExecution` error persists in `error_message` column | K-1 | P0 |
| `FailSkillExecution` with row already `stopped` -> no-op | C3-3 | P0 |
| `StopSkillExecution` leaves `completed` untouched | C3-3 | P0 |
| Float64 -> int coercion with API initial_vars | C3-5 | P0 |
| Recovery: pending -> claims -> executes | L-1 | P0 |
| Postgres `CreateSkillLog`/`ListSkillLogs` round-trip | C3-8 | P0 |
| CEL: skipped node -> clean false, no error | K-4 | P0 |
| Chat-path `llm_call` with tenant id resolves model | K-3 | P1 |
| `current_node` = node name | MED-1 | P1 |
| MySQL -> workflow routes 404 | C3-7 | P1 |
| `DurationMs` > 0 for actual steps | C3-6 | P1 |
| `checkpoint.go` compiles after Fix 1 | K-2 | P1 |

### Manual verification

- [ ] `go build ./...` clean
- [ ] `go test ./store/... ./server/router/api/v1/agent/...` all pass
- [ ] `task validate:parity`
- [ ] Stop mid-step -> `stopped`
- [ ] `@signal` -> `stopped`, no re-claim
- [ ] List -> tenant-scoped
- [ ] CEL `search_kb.found == false` works
- [ ] Float64 initial_vars -> eval succeeds
- [ ] `current_node` = node name
- [ ] `llm_call` uses tenant's model/key
- [ ] Recovery handles skill-less rows
- [ ] `error_message` populated on failure
- [ ] MySQL -> routes 404

---

## 4. Implementation Order

| Step | Fix | Files | Est. LOC | Depends On |
|------|-----|-------|----------|------------|
| 1 | Fix 1 — DDL/types + LATEST.sql + K-1 + K-2 | 12 | ~200 | — |
| 2 | Fix 3 — Atomic guards + store API | 7 | ~150 | Fix 1 |
| 3 | Fix 2 — ListSkillExecutions | 6 | ~100 | Fix 1 |
| 4 | Fix 4 — CEL + output contract + K-4 | 3 | ~120 | — |
| 5 | Fix 5 — Stop write + sentinel | 1 | ~50 | Fix 3, Fix 4 |
| 6 | Fix 7 — Recovery + K-5 | 1 | ~15 | Fix 3 |
| 7 | Fix 6 — GenerateFn tenant + K-3 | 2 | ~60 | — |
| 8 | Fix 8 — current_node | 2 | ~10 | — |
| 9 | Fix 9 — MySQL gating | 2 | ~20 | — |
| 10 | Fix 10 — Minor (C3-6) | 2 | ~20 | — |
| 11 | New tests | 3-5 | ~250 | All |
| | **Total** | | **~995** | |

---

## 5. Plan6 Conformance (post-fix)

| plan6 requirement | Status after code5 |
|-------------------|---------------------|
| D2 — chat path durable loop | DEFERRED (acknowledged, non-blocking) |
| R3 — status re-read guards | FIXED (Fix 3) — atomic store methods |
| D4 — CEL env + output semantics | FIXED (Fix 4) — parsed JSON output + dynamic env |
| D5/R2 — recovery worker | FIXED (Fix 7) — no double-claim + skill-less fail |
| D6 — outbound events | Already working |
| MaxRetries/Timeout honored | DEFERRED — parsed but not driving retry logic |
| 10 cadence (30s ticker) | Minor deviation, acceptable |
| 11 endpoints + RBAC | Already working + Fix 9 gating |

---

## 6. Files Modified (complete manifest)

### Store layer
1. `store/agent.go` — SkillExecution/SkillLog time.Time + ErrorMessage + 3 new wrapper methods
2. `store/driver.go` — +ListSkillExecutions + CompleteSkillExecution + FailSkillExecution + StopSkillExecution
3. `store/db/sqlite/agent_skill.go` — time conversion + all CRUD + 3 new methods + log methods
4. `store/db/postgres/agent_skill.go` — JSONB fix + time pass-through + 3 new methods + log methods
5. `store/db/mysql/agent_skill.go` — stubs for all new methods

### Migrations + LATEST.sql
6. `store/migration/sqlite/0.36/00__add_skill_executions.sql` — add error_message
7. `store/migration/postgres/0.36/00__add_skill_executions.sql` — PG dialect + error_message
8. `store/migration/cockroach/0.36/00__add_skill_executions.sql` — verify + error_message
9. `store/migration/mysql/0.26/00__add_skill_executions.sql` — verify + error_message
10. `store/migration/sqlite/LATEST.sql` — add error_message
11. `store/migration/postgres/LATEST.sql` — PG dialect + error_message
12. `store/migration/cockroach/LATEST.sql` — verify + error_message
13. `store/migration/mysql/LATEST.sql` — verify + error_message

### Agent engine
14. `server/router/api/v1/agent/evaluator.go` — EvalConditionDynamic
15. `server/router/api/v1/agent/checkpoint.go` — atomic store methods + current_node fix + K-2 timestamp fix
16. `server/router/api/v1/agent/execution.go` — parsed output + stop write + sentinel + fresh context + K-4 placeholders
17. `server/router/api/v1/agent/recovery.go` — K-5 skill-less fail + mysql gating
18. `server/router/api/v1/agent/skill_builtins.go` — tenant-aware GenerateFn + K-3 fallback
19. `server/router/api/v1/agent/service.go` — wire GenerateFn + SECTION 7B filter

### API
20. `server/router/api/v1/v1.go` — conditional routes on profile driver
21. `server/router/api/v1/agent/handlers.go` — HandleListExecutions fix

### Tests
22. `server/router/api/v1/agent/evaluator_test.go` — dynamic CEL tests
23. `server/router/api/v1/agent/execution_test.go` — stop + structured output tests
24. `server/router/api/v1/agent/skill_builtins_test.go` — updated GenerateFn signature
25. `store/db/sqlite/agent_skill_test.go` — (new) time round-trip tests
26. `store/db/postgres/agent_skill_test.go` — (new) integration round-trip tests

---

## 7. Bottom Line

- **code4_review.md is the first approving review** in the chain. Verdict: "APPROVED with nits & one rework item."
- **K-1 resolved:** `error_message` column added to `agent_skill_executions` schema.
- **K-2…K-5 folded** into corresponding fixes.
- **Estimated effort:** ~995 LOC across ~30 files, plus ~250 LOC of tests.
- **Implementation order:** DDL/types first, then atomic guards, tenant isolation, CEL, stop sentinel, recovery, minor fixes.
- **This is the final plan.** All review chain findings are incorporated. Ready to implement.
