# bchat Durable Execution — Adversarial Code Review (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** Implementation vs. `plan6.md` (approved) — reads of every new file and all modified-file diffs.
**Build/Tests:** `go build ./store/... ./server/router/api/v1/agent/...` clean; unit tests pass (`-run 'Skill|Execution|Evaluator|Builtin|Parse'`).
**Caveat:** Tests cover only isolated components. Nothing exercises a real persistence round-trip on postgres/cockroach and nothing tests handler↔store integration — which is exactly where the critical bugs live.

---

## Summary

| Severity | Count |
|----------|-------|
| Critical | 3 |
| High | 5 |
| Medium | 6 |
| Low | 4 |

The SQLite path works end-to-end; the feature is fundamentally broken on PostgreSQL and CockroachDB (the hackathon target) at the driver layer, leaks cross-tenant data via the list endpoint, and the stop/state-machine semantics cannot produce a `stopped` terminal state. The parsed graph's stop signals, triggers, retries, and timeouts are dead code.

---

## Critical

### CRITICAL-1 — DDL/Go type mismatch breaks all persistence on PostgreSQL and CockroachDB

- **Type:** DB / Integration
- **Files:** `store/migration/cockroach/0.36/00__add_skill_executions.sql`, `store/migration/postgres/0.36/00__add_skill_executions.sql`, `store/agent.go:1364`, `store/db/postgres/agent_skill.go:29-48,265-281`
- **Description:** The postgres/cockroach migrations declare `claimed_at`, `claim_expires_at`, `created_at`, `updated_at`, `completed_at` as `TIMESTAMPTZ`. The Go structs hold `int64` unix epochs (`SkillExecution.CreatedAt int64`, etc.) and the postgres driver writes them directly with pgx (`execution.CreatedAt`, `execution.UpdatedAt`). pgx cannot cast int64↔timestamptz, so every `CreateSkillExecution`, `UpdateSkillExecution`, `ClaimSkillExecution`, and `scanSkillExecutions` read/write fails at runtime on both PostgreSQL and CockroachDB.
- **Reproduction:** Run the stack with `MEMOS_DRIVER=cockroach` (or `postgres`), start any workflow, watch every store call error.
- **Fix:** Match the existing codebase convention — `time.Time` fields + `TIMESTAMPTZ` columns for the pgx drivers (see `AgentTenant.CreatedAt time.Time` pattern in `store/agent.go`), or change the DDL to `INT8/bigint epoch` and adapt Cockroach's `verifyCockroachIndexes`-style handling. Single shared struct means the cleanest fix is `time.Time` in the store + converting to/from epoch only in the SQLite driver.

### CRITICAL-1b — postgres migration file is CockroachDB dialect and cannot run on real PostgreSQL

- **Type:** DB / Migration
- **File:** `store/migration/postgres/0.36/00__add_skill_executions.sql`
- **Description:** The "postgres" migration is a copy of the CockroachDB DDL and uses `STRING`, `INT8`, `INT4`, `JSONB`, `TIMESTAMPTZ DEFAULT NOW()`. `STRING` is not a valid PostgreSQL type; the migration fails. (Cockroach accepts all of these, so only the CRDB file is self-consistent.)
- **Fix:** Rewrite the postgres file in PG dialect (`TEXT`, `BIGINT`, `JSONB`, `TIMESTAMPTZ DEFAULT NOW()`), and keep the Cockroach dialect only under `store/migration/cockroach/`.

### CRITICAL-2 — Cross-tenant data leak in the executions list endpoint

- **Type:** Security — Tenant Isolation
- **Files:** `server/router/api/v1/agent/checkpoint.go:191-193`, `store/db/sqlite/agent_skill.go:98-117`, `store/db/postgres/agent_skill.go:98-117`
- **Description:** `listSkillExecutionsByTenant` returns `store.ListPendingSkillExecutions(ctx)` for `pending`/`running` status filters. That query has **no `tenant_id` filter** (`WHERE status IN ('pending','running') AND trigger_path != 'chat'`), so it returns every tenant's executions. `GET /api/v1/agent/:slug/executions?status=pending` with only `tenant:read` exposes other tenants' execution IDs, conversation IDs, and (via `HandleGetExecution`) their checkpoint data.
- **Reproduction:** Tenant A lists `?status=running`; rows from tenant B appear.
- **Fix:** Add a tenant-scoped list store method (`ListSkillExecutionsByTenant(tenantID, status, limit)`) and use it here. Never reuse the global pending query for HTTP.

### CRITICAL-3 — Stop can never produce a `stopped` terminal state for in-flight executions

- **Type:** State Machine / Race Condition
- **Files:** `server/router/api/v1/agent/execution.go:97-100`, `server/router/api/v1/agent/checkpoint.go:78-90`
- **Description:** `StopExecution` (checkpoint.go:93-107) writes `status='stopped'`. The executing goroutine observes `ctx.Done()` (via `executeWidget`/`select` in `executeWorkflow`), `executeWorkflow` returns an error, and `runDetachedExecution` then calls `s.failExecution(ctx, exec, err.Error())` unconditionally, overwriting `stopped` with `failed`. Terminal state for a user-initiated stop is therefore always `failed`.
- **Reproduction:** Start a detached run, stop it mid-step → DB status ends as `failed`.
- **Fix:** Make `failExecution` no-op (or status-guarded) when the current status is `stopped`/`completed`; or have `runDetachedExecution` re-read status before failing. Also add the same status re-read guard to `completeExecution` (it currently overwrites a concurrent `stopped` with `completed`, since R3 re-read only exists in `writeCheckpoint`).

---

## High

### HIGH-1 — Documented CEL condition pattern cannot compile (plan6 D4 not implemented)

- **Type:** Correctness — CEL
- **Files:** `server/router/api/v1/agent/evaluator.go:18-27`, `server/router/api/v1/agent/execution.go:206-214`
- **Description:** plan6 §8.1 (fix D4) declares **all graph node names** in the CEL env so conditions can reference prior skill outputs. The implementation instead declares only `standardCELVars` (`user_message`, `session_messages`, `urgency`, ...). The documented examples — `condition: "search_kb.found == false"` and `@signal: condition: "create_ticket.ticket_id != ''"` — reference `search_kb`/`create_ticket`, which are undeclared variables → `env.Compile` returns "undeclared reference" → the whole workflow fails.
- **Reproduction:** Author any SCRIPT.md using the plan's own sample conditions.
- **Fix:** Implement D4: build the env from `graph.Nodes` keys (dyn type) plus the standard vars, and evaluate against vars that carry `state[nodeName]` values.

### HIGH-2 — Chat path is not durable (plan §7.1 not implemented)

- **Type:** Architecture — Integration
- **Files:** `server/router/api/v1/agent/service.go:3556+` (`toolCallingLoop`), `server/router/api/v1/agent/service.go:3048-3063`
- **Description:** plan6 D2 says the chat path runs the same tool-calling loop and carries an execution record (`trigger_path='chat'`, status `running→completed/…`). The implementation folds an ad-hoc loop into `generateResponse` that executes handlers with **no `SkillExecution` created, no checkpoints, no audit logs, no state machine**. It also makes a first LLM call without `tools`, then another with them (wasted round trip), and returns the loop's last response. Non-chat durable runs (`api`) use a separate, duplicated code path in `execution.go`.
- **Fix:** Create a `running` SkillExecution in the chat path, exec handlers through `s.executeStep`-style logic, write checkpoints/logs, and complete/fail the execution. Alternatively, explicitly document chat as non-durable and remove the misleading SECTION 7B tool invitation for it.

### HIGH-3 — `llm:` delegation and documented builtins are non-functional

- **Type:** Integration — Tool Registry
- **Files:** `server/router/api/v1/agent/skill_builtins.go:16-20,120-144`, `server/router/api/v1/agent/service.go:206-210`, `server/router/api/v1/agent/skill.go:70-103`
- **Description:** `RegisterBuiltins` registers only `log`, `sleep`, `llm_call`, and `LLMHandler.GenerateFn` is never wired in `NewService` (known limitation in code.md but still shipped). Consequences: (a) any `llm:`-prefixed skill errors at runtime with "GenerateFn not set"; (b) every example handler from the SCRIPT.md docs (`builtin:classify_intent`, `builtin:search_kb`, `builtin:create_ticket`, `llm:respond`) is not in the registry, so `ToolsForSkills` silently drops them and the detached `executeWorkflow` fails those nodes with "handler not found". The feature cannot run any realistic tenant configuration.
- **Fix:** Wire `GenerateFn` to the service LLM path and register real `builtin:classify_intent`/`search_kb`/`create_ticket`-style handlers, or strip the doc/examples to the three real builtins.

### HIGH-4 — List endpoint returns wrong data (single-row LIMIT 1 + broken status handling)

- **Type:** Correctness — API
- **File:** `server/router/api/v1/agent/checkpoint.go:157-208`
- **Description:** For any status other than `pending`/`running`, `listSkillExecutionsByTenant` calls `GetSkillExecution` (hard `LIMIT 1`), so `GET /executions?status=completed` returns at most one execution and ignores the `limit` param (`assert`ed total is always 0/1). For `pending`/`running` it leaks cross-tenant rows (CRITICAL-2).
- **Reproduction:** Create 3 completed runs, list `?status=completed&limit=50` → 1 item.
- **Fix:** Implement `ListSkillExecutions(find, limit)` per driver (tenant-filtered) and delete the two fake `list*` helpers.

### HIGH-5 — `@signal` / `@trigger` are parse-only dead code

- **Type:** Correctness — Workflow Semantics
- **Files:** `server/router/api/v1/agent/parser.go:1240-1252`, `server/router/api/v1/agent/execution.go:107-188`
- **Description:** `graph.Trigger`, `graph.Stop`, `StopSignalDefinition.Condition`, and `StopSignalDefinition.EmitEvent` are populated by the parser and asserted in tests but never read by the execution engine. `executeWorkflow` never evaluates the stop condition, never emits the stop event, and the trigger type is never consulted. "workflow.cancelled"/"pipeline_completed" events and mid-pipeline termination cannot occur.
- **Reproduction:** SCRIPT with `@signal: condition: "urgency > 5"` runs to completion regardless of `urgency`.
- **Fix:** In `executeWorkflow`, evaluate `graph.Stop.Condition` after each step; on met, `emitEvent` + mark execution `stopped`. Consider declaring the stop vars in the CEL env (ties into HIGH-1).

---

## Medium

### MED-1 — `current_node` stores full handler output, not the node name

- **File:** `server/router/api/v1/agent/checkpoint.go:42`
- **Description:** `writeCheckpoint` sets `exec.CurrentNode = output` — the handler result (potentially a long LLM blob), not the node identifier. Bloats the row and defeats the intended "current position" semantics on resume.
- **Fix:** Pass/store the node name; keep output in checkpoint state only.

### MED-2 — postgres `ListSkillLogs` scans JSONB into `map[string]any` and errors

- **File:** `store/db/postgres/agent_skill.go:210-212`
- **Description:** `rows.Scan(&l.Input, &l.Output, ...)` with `Input/Output map[string]any` — `database/sql`/pgx cannot scan a JSONB value into a map; runtime scan error. The SQLite implementation correctly round-trips through `sql.NullString` + `json.Unmarshal`.
- **Fix:** Mirror the SQLite scan (null string → unmarshal).

### MED-3 — `max_retries`, `timeout`, `retry_count`, `failed_nodes` are parsed but unused

- **Files:** `server/router/api/v1/agent/parser.go:103-108`, `server/router/api/v1/agent/execution.go:107-188`, `store/agent.go:1371,1374`
- **Description:** `SkillDefinition.Timeout`, `SkillDefinition.MaxRetries`, `SkillExecution.RetryCount`, `FailedNodes` are written/read but never drive behavior — no per-node timeout, no retry/backoff, no failed-node tracking. Nodes that error fail the whole execution immediately.
- **Fix:** Either implement the semantics (timeout via `context.WithTimeout` per node, retry loop with `retry_count++`, failure recording) or remove the unused fields to avoid false affordances.

### MED-4 — Detached runs and recovery worker use `context.Background()` — no graceful shutdown

- **Files:** `server/router/api/v1/agent/service.go:228`, `server/router/api/v1/agent/execution.go:44`, `server/router/api/v1/agent/recovery.go:13-35`
- **Description:** `StartDetachedExecution` spins the goroutine on `context.Background()`; the recovery ticker is rooted on `context.Background()` from `NewService`. Nothing cancels on shutdown → goroutines and tickers leak at exit; in-flight runs are not drained.
- **Fix:** Root both on a service-level cancellable context wired to a shutdown hook (e.g. `RunServer`/app lifecycle).

### MED-5 — `failExecution` stores error strings in checkpoint data

- **File:** `server/router/api/v1/agent/checkpoint.go:84-87`
- **Description:** `exec.CheckpointData["error"] = errMsg` persists raw error text (may include driver internals) into a JSON blob that is exposed to tenants via `HandleGetExecution`. No status guard before overwriting `stopped`/`completed` (ties into CRITICAL-3).
- **Fix:** Use `error_message` column or a separate field; guard on terminal statuses; scrub driver details.

### MED-6 — Stop on terminal executions + MySQL stub under a live migration

- **Files:** `server/router/api/v1/agent/checkpoint.go:93-107`, `store/db/mysql/agent_skill.go:13-43`
- **Description:** `stopExecution` unconditionally sets `stopped` even for completed records. MySQL driver methods all return "not implemented", yet the MySQL migration + `LATEST.sql` create the tables and the `Driver` interface requires the methods — a MySQL deployment fails at runtime on first workflow call.
- **Fix:** Guard stop on `pending`/`running`; either implement MySQL or gate the migrations/endpoints on driver support.

---

## Low

- **LOW-1 —** `logSkillStep` hardcodes `DurationMs: 0` (`execution.go:260`); actual step duration is measured in `executeStep` but not recorded.
- **LOW-2 —** CEL env compiled fresh on every evaluation (`evaluator.go:44`); cache compiled programs per expression.
- **LOW-3 —** buildSystemPrompt SECTION 7B lists every skill node including ones with no registered handler, so the LLM is invited to call tools that don't exist (`service.go:3381-3398`, `skill.go:70-103`).
- **LOW-4 —** Recovery worker deserializes the full graph JSON for every pending+running row every 30s tick, even when the claim then fails (`recovery.go:38-77`); iterate candidates, attempt claim, only then deserialize.

---

## Observations / Process Notes

- `parseParams` stores the first positional value under key `"code"` and copies the entire annotation param map into `SkillDefinition.Params`, so handlers receive graph metadata (`handler`, `depends_on`, `timeout`, `condition`) mixed with their real params (`skill.go` `Execute(params)`). Consider filtering.
- plan6 stated MySQL was intentionally a stub; if so, `LATEST.sql`/migration for MySQL should be dropped or clearly marked WIP to avoid drift (see `validate:parity`).
- No integration tests exist for: detached create→claim→execute→complete, checkpoint resume after crash, stop semantics, recovery reclaim, or any postgres/cockroach store round-trip. These are the highest-value test additions.
- `claims` reuse `WHERE status='pending' OR (status='running' AND claim_expires_at < now)` — sound, but combined with MED-3 there is no lease *renewal* during long LLM/handler steps, so a long-running node can have its claim expire and be double-claimed by the recovery worker mid-step.