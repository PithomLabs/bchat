# bchat Durable Execution — Adversarial Review of code6_imp.md (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** The code6.md implementation (10 fixes, ~30 files) verified against the approved plan (code6.md), prior plan with revisions up to plan6.md/code6.md, and the full 5-review chain (code_review → code2_review → code3_review → code4_review → code5_review → code6_imp).
**Method:** Diffed working tree against the pre-fix baseline (mtime attribution), read every touched file, ran the build + unit tests + schema/migration/parity validators, and **empirically probed the CEL evaluator** with the documented condition patterns to confirm runtime semantics rather than trusting pseudocode.

---

## 0. Tree Status (verified)

The plan WAS actually implemented this time (unlike code5.md which was a plan gate). Files modified after plan finalization:

| Area | Files (mtime) |
|------|----------------|
| Store | `store/agent.go` 09:25, `store/driver.go` 09:24, `store/db/{sqlite,postgres,mysql}/agent_skill.go` 09:26/09:27 |
| Engine | `evaluator.go` 09:33, `execution.go` 09:34, `checkpoint.go` 09:40 |
| Service | `service.go` 09:42 |
| Tests | `execution_test.go` 09:34 |
| Migrations | all 4 versioned DDLs + all 4 `LATEST.sql` |

**Not touched this round:** `recovery.go` (07:33), `skill_builtins.go` (07:27), `evaluator_test.go` (07:27), `skill_builtins_test.go` (07:27), `handlers.go` (07:38), `v1.go`.

**Verification commands run (all clean):**
- `go build ./store/... ./server/router/api/v1/agent/... ./server/router/api/v1/` — clean
- `go test ./server/router/api/v1/agent/... -run 'Skill|Execution|Topological|EvalCondition'` — pass
- `go test ./store/...` — pass
- `./scripts/validate-parity.sh` — PASS (file-list + schema + CRDB mirror)
- `./scripts/validate-migrations.sh` — PASS (LATEST in sync)
- `task validate:schema` — PASS

**Caveats:** `validate:schema` logs `WARNING migration FS has directories newer than code version; fs_minor=0.36 code_minor=0.34` — pre-existing (0.36 migration dir predates this round), non-blocking, but the schema validator does not exercise the skill tables' PG dialect, which is confirmed correct by direct read.

---

## 1. Verified-Fixed Checklist (all checked against code, not the checklist doc)

### CRITICAL — all resolved
- **CRITICAL-1:** `SkillExecution` timestamps are `time.Time`/`*time.Time` (store/agent.go:1362-1367); sqlite converts via `timeToUnix`/`unixToTime`/`nullInt64ToTime` (store/db/sqlite/agent_skill.go:17-47); postgres passes `time.Time` directly (store/db/postgres/agent_skill.go:46-51). ✔
- **CRITICAL-1b:** postgres DDL uses `TEXT`/`BIGINT`/`INTEGER`/`TIMESTAMPTZ`/`JSONB` (store/migration/postgres/0.36/00__add_skill_executions.sql); CRDB DDL retains `STRING`/`INT8`/`INT4`; parity validator passes. ✔
- **CRITICAL-2:** `find.TenantID != nil` adds `tenant_id = ?/$N` in sqlite:281-283, postgres:241-243; service passes tenant from context (`checkpoint.go:115-122`); handlers derive tenant via `getTenantOrFail` and enforce ownership on get/stop (handlers.go:6788-6807). ✔
- **CRITICAL-3:** `errStopSignal` sentinel (execution.go:16); stop-condition branch calls `StopSkillExecution` BEFORE returning sentinel (execution.go:219-222); `runDetachedExecution` handles sentinel via `errors.Is` (execution.go:108-111) and re-reads with `context.Background()` (execution.go:103). ✔

### HIGH — resolved
- **HIGH-1:** `EvalConditionDynamic` exists, all vars as `cel.DynType`, graph node names registered (evaluator.go:77-132). ✔ (output-contract semantics see Finding R-1)
- **HIGH-3:** `GenerateFn` wired post-`RegisterBuiltins` (service.go:213-252), resolves per-tenant config in the detached path. ✔ (chat path see Finding R-3)
- **HIGH-4:** `ListSkillExecutions` applies `LIMIT %d` with the caller value (sqlite:298-309, postgres:258-269); `GetSkillExecution` uses limit 1. ✔
- **HIGH-5:** stop condition evaluated after each checkpointed step (execution.go:204-224). ✔

### MED / LOW / K / N5 / C3 — resolved
- **MED-1** `writeCheckpoint(..., nodeName string)` → `exec.CurrentNode = nodeName` (checkpoint.go:28-46). ✔
- **MED-2** postgres `ListSkillLogs` scans JSONB into `[]byte` then unmarshals (postgres/agent_skill.go:217-229). ✔
- **MED-5** `failExecution` → `FailSkillExecution` (checkpoint.go:72-74). ✔
- **MED-6 / C3-3** atomic methods with conditional WHERE + `RowsAffected()==0 → log+nil` (sqlite:370-423, postgres:300-350). ✔
- **LOW-1 / C3-6 / C3-8** `DurationMs: int(time.Since(start).Milliseconds())` (execution.go:302). ✔
- **LOW-3** `ToolsForSkills` skips `condition`/`handler` nodes (skill.go:77-79). ✔
- **C3-7** routes gated `s.Profile.Driver != "mysql"` (v1.go:352-356); recovery gated at service.go:272. ✔
- **K-1** `error_message` in all 4 DDLs + all 4 LATEST (incl. mysql, whose LATEST carries the column for both tables), struct (store/agent.go:1358), every scan/SELECT in both drivers. ✔
- **K-2** `time.Now()` everywhere in checkpoint.go (checkpoint.go:88,43). ✔
- **K-5** recovery fails skill-less rows (recovery.go:67-72) and deserialize-failure rows (recovery.go:60-65). ✔ — **note:** file mtime is 07:33, i.e. the fix pre-dates this implementation round; the review chain's K-5 finding appears to have been inaccurate about the tree it reviewed.
- **K-6** `LATEST.sql` casing correct in all references. ✔
- **N5-1** postgres `ClaimSkillExecution` uses `time.Now()`/`time.Now().Add(...)` (postgres/agent_skill.go:127-128); sqlite correctly keeps epoch ints. ✔
- **C3-2 / K-4 / N5-2 / N5-3 / N5-4 / N5-5** — see Findings; N5-4 (SELECT lists) is ✔ but N5-2/N5-3 are **not effective as implemented**.

---

## 2. Findings

### R-1 (HIGH) — C3-2 output contract not implemented; conditions silently mis-evaluate

- **File:** `server/router/api/v1/agent/execution.go:189` (`state[nodeName] = output`)
- **Description:** The plan mandates the canonical output contract (parse handler output as JSON → map; else wrap in `map[string]any{"output": raw}`). The implementation stores the **raw output string**. Empirically verified with the actual evaluator:
  - `search_kb = {"found":true,"ticket_id":"T1"}` (raw JSON string), expr `search_kb.found == false` → `Met=false`, **no error**.
  - `search_kb = "logged"` (plain string), expr `search_kb.found == false` → `Met=false`, no error.
  - cel-go dyn field access on a non-map returns absent→`nil`, and `nil == false` is `false` — so `.field` conditions on any string-valued node output **always evaluate false**, silently.
- **Impact:** The parser's own documented example (`parser_skill_test.go:26`: `condition: "search_kb.found == false"`) can never fire: `create_ticket` is skipped even when the KB search correctly returned `found:false`. The failure is silent — no error, no log.
- **Expected:** Apply the plan's Fix 4 contract verbatim at execution.go:189 (JSON-unmarshal else `{"output": raw}`).

### R-2 (HIGH) — N5-2/K-4 ineffective: missing-variable still hard-errors the workflow

- **Files:** `server/router/api/v1/agent/evaluator.go:135-139` (`isMissingKeyError`), `execution.go:176-179` (deps-not-met `continue` with no placeholder)
- **Description:** K-4's placeholder for skipped nodes was never written, and `isMissingKeyError` does not match cel-go's actual missing-variable error. Empirically verified:
  - var absent, expr `search_kb.found == false` → error `cel eval: no such attribute(s): search_kb`
  - var absent, expr `search_kb.output == ''` → same error
  - `isMissingKeyError` matches only `no such key` / `missing variable` / `undeclared identifier` — **not** `no such attribute(s)`.
- **Impact:** In `executeStep` (execution.go:249-252) this error propagates → `executeWorkflow` aborts → `runDetachedExecution` calls `FailSkillExecution`. The exact scenario N5-2 was designed to fix — a node referencing a downstream-skipped node — now **fails the whole execution**. The plan's P0 test "skipped node → clean false, no error" would fail; it was never written.
- **Expected:** Either write the K-4 placeholder (`state[nodeName] = map[string]any{"output":"","skipped":true}`) so the var resolves and field access yields `false`, or extend `isMissingKeyError` to match `no such attribute` / `could not find attribute`. Both are one-liners.

### R-3 (MEDIUM) — N5-3 tenant injection is inert in the chat path

- **Files:** `server/router/api/v1/agent/service.go:3653-3657`, `server/router/api/v1/agent/skill_builtins.go:126-143`
- **Description:** `toolCallingLoop` injects `args["tenant_id"]` — that is the *params* map. `LLMHandler.Execute` then calls `GenerateFn(ctx, expandedPrompt, vars)` passing the **vars** map, which is `nil` in the chat path (`service.go:3657`). The `GenerateFn` closure resolves the tenant from `vars["tenant_id"]` (service.go:218) and therefore always misses → `tenantID = 0` → `requireLLMConfig(ctx, 0)` → **env-default model/key**, exactly what K-3/N5-3 were raised to prevent.
- **Impact:** Chat-path `llm_call` silently uses the default config even when the tenant has a custom engine config. The detached path works only by accident (executeStep passes `celVars` as vars; execution.go:237-245).
- **Expected:** Inject into the vars map the closure reads: `h.Execute(ctx, args, map[string]any{"tenant_id": config.TenantID})` at service.go:3657 (or read `params["tenant_id"]` in `LLMHandler.Execute`).

### R-4 (MEDIUM) — Test plan largely unimplemented; the two HIGH defects are unguarded

- **Files:** missing — `store/db/sqlite/agent_skill_test.go`, `store/db/postgres/agent_skill_test.go`
- **Description:** code6.md's P0/P1 rows that do not exist anywhere in the tree: Postgres round-trip (CRITICAL-1), PG/CRDB claim round-trip (N5-1), `FailSkillExecution` persists to `error_message` (K-1), atomic no-op guards (C3-3), tenant-scoped list (CRITICAL-2), `DurationMs`/`current_node`, dynamic CEL + N5-2 tolerant eval (evaluator_test.go is untouched and tests only `EvalCondition`, never `EvalConditionDynamic`), stop-during-step, `@signal`. N5-5's `SKILL_PG_INTEGRATION_TEST` gating is absent (grep over the repo yields nothing).
- **Impact:** R-1 and R-2 are each caught by a planned P0 row that was never written. A round-trip suite for sqlite would also serve as the compile+runtime regression gate the plan promised.
- **Expected:** Add `store/db/sqlite/agent_skill_test.go` (round-trip, atomic guards, tenant filter, claim), `store/db/postgres/agent_skill_test.go` (integration, `testing.Short()` skip + `SKILL_PG_INTEGRATION_TEST=true` opt-in), and `EvalConditionDynamic` cases for: raw-string output, placeholder-map, and missing-variable field access (these would have caught R-1 and R-2).

### NITS — fold in
- **R-5 (LOW):** stop-condition eval error swallowed — `result, _ := EvalConditionDynamic(...)` at execution.go:215. A non-missing-key error (e.g. a misspelled identifier at compile time) silently disables the stop rule. Log + treat as not-met.
- **R-6 (LOW):** postgres/cockroach `agent_skill_executions.tenant_id BIGINT NOT NULL` vs sqlite `DEFAULT NULL`, and `CreateSkillLog` (postgres) writes `tenant_id` from a `*int32` that may be nil (execution.go:295). A nil-tenant execution/log hits a NOT NULL violation on PG/CRDB. Either make the column nullable like sqlite, or refuse to start executions with nil tenant.
- **R-7 (LOW):** `HandleListExecutions` parses `limit` with `fmt.Sscanf` (handlers.go:6877-6884) — `?limit=0` returns an empty list; `?limit=-5` produces `LIMIT -5` → runtime SQL error → 500. Validate `limit >= 1` before the query (and `limit <= 200` is already capped).
- **R-8 (INFO):** `EmitEvent` on stop (planned in Fix 5 pseudocode) is not dispatched; code6.md itself dropped it, so this is informational, not a regression.

---

## 3. Conformance

| plan6 / code6 requirement | Status | Evidence |
|----------------------------|--------|----------|
| R3 status guards | FIXED | checkpoint.go:30-38 + atomic methods |
| D4 CEL env + output semantics | **BROKEN** | R-1, R-2 |
| D5/R2 recovery | FIXED | recovery.go both fail paths; claim lease |
| D6 outbound events | OK | checkpoint.go:60-66 complete event |
| HIGH-3 GenerateFn wired | OK (detach) / **BROKEN (chat)** | R-3 |
| MySQL gating | FIXED | v1.go:352, service.go:272 |
| Test plan | **NOT DONE** | R-4 |
| D2 chat durability, MED-3, MED-4, LOW-2, LOW-4, MaxRetries | deferred per code6.md | acknowledged |

---

## 4. Bottom Line / Sign-off

- **Tree:** implementation genuinely applied this round; all build/test/migration/parity gates pass; the clean fixes (timestamps, dialect, atomic guards, tenant filters, MySQL gating, GenerateFn wiring) are correctly implemented with file:line evidence.
- **Plan:** three of the plan's headline guarantees are **not effective in the final code**: the documented C3-2/HIGH-1 condition pattern silently mis-evaluates (R-1), the N5-2/K-4 tolerant eval still hard-errors on skipped nodes (R-2), and the N5-3 chat-path tenant injection is dead (R-3). Two of the three would have been caught by the plan's own P0 tests, which were never written (R-4).
- **Verdict: REWORK** — resolve R-1 and R-2 (each a one-line contract fix at execution.go:189 and evaluator.go:135-139, or the K-4 placeholder at execution.go:176-179), fix R-3 (inject into the vars map at service.go:3657), and implement the R-4 test rows that guard all three. Until then the claim "review chain complete, no remaining defects" (code6.md §7) does not hold for the code as written.
- **Honest correction to the chain:** the K-5 recovery-skillless leak flagged in code4/code5 reviews was already fixed in the tree that those reviews inspected (recovery.go mtime 07:33, unchanged); that finding was overstated rather than under-fixed.