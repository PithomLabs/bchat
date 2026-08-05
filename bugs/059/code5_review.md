# bchat Durable Execution — Adversarial Review of code5.md (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** `bugs/059/code5.md` (Plan 5, "Ready for Implementation") verified against plan6.md (APPROVED baseline), code4_review.md (prior APPROVED-with-nits verdict), and the actual codebase.
**Method:** Read the full plan; verified K-1…K-6 adoption claims against the code — DDL column types, struct field types, driver SQL literal arguments, select/scan lists, and the chat-path call sites. Prior review verdicts (`code_review.md`, `code2_review.md`, `code3_review.md`) taken as context; code4_review.md dispositions re-checked where the new plan changed them.

---

## 0. Tree Status (verified)

code5.md (mtime 09:05) was written after the last source edit (store 07:09–07:13, agent engine 07:27–07:33). The working tree is byte-identical to the implementation reviewed in `code_review.md`. **No code5.md change is present in the code** — as before, this is a plan-approval gate, not a post-implementation review. All `code_review.md` findings remain live until this plan is executed.

---

## 1. What code5.md Gets Right (verified correct)

- **K-1 resolved exactly as committed:** `error_message TEXT DEFAULT ''` added to `agent_skill_executions` for sqlite/postgres and as `STRING DEFAULT ''` for CRDB, across all four versioned DDLs (including `store/migration/postgres/0.36/00__add_skill_executions.sql`) and all four `LATEST.sql`. `ErrorMessage string` on `SkillExecution` matches the atomic `FailSkillExecution` SQL, so fail paths persist the error in the column (not in `checkpoint_data` — MED-5 concern preserved). Fix 3's `SET error_message=?` now targets a real column.
- **K-2 folded into Fix 1** with the exact line references (`checkpoint.go:43`, `:116`, `:130-131`, plus `completeExecution`/`failExecution` `CompletedAt`). After the struct change those are `int64 → time.Time` assignments and the package would not compile otherwise — scope now includes `checkpoint.go`.
- **K-3 folded into Fix 6** with an explicit tenant-absent fallback to `requireLLMConfig(ctx, 0)`.
- **K-4 folded into Fix 4** — skipped-node placeholders written to `state` in `executeWorkflow`.
- **K-5 folded into Fix 7** — both deserialize-failure and skill-less rows call `FailSkillExecution`; the infinite re-scan is closed.
- **K-6 casing fixed** (`LATEST.sql` throughout).
- **Fix 1 postgres dialect is correctly enumerated:** `STRING→TEXT`, `INT8→BIGINT`, `INT4→INTEGER` for the versioned PG DDL, and `[]byte` (not `string`) for JSONB params. `pgx/v5` handles the rest.
- **Fix 3 re-read is correct:** `GetSkillExecution(context.Background(), …)` after `executeWorkflow` avoids the canceled-detached-ctx hazard the plan was once exposed to.
- **Fix 9 remains implementable as written** (`apiv1Service.Profile.Driver` exported at `v1.go:49`; recovery gate via `s.profile.Driver`).
- **Fix 2 sound:** exposing the private `listSkillExecutions` (sqlite `agent_skill.go:233`, postgres `agent_skill.go:221`) as `ListSkillExecutions(ctx, find, limit)` stays mechanical; mysql stubs are in the manifest.

---

## 2. Findings

### NITS — fold in during implementation (nothing blocks approval)

#### N5-1 — Fix 1 misses the postgres claim-path timestamps (runtime bug on PG/CRDB)

- **File:** `store/db/postgres/agent_skill.go:119-129` (`ClaimSkillExecution`)
- **Description:** This is the CRITICAL-1 defect *resurfacing inside a local variable*. The method computes `now := time.Now().Unix()` / `expiresAt := now + int64(leaseSeconds)` and passes int64 into `claimed_at = $1`, `claim_expires_at = $3`, and the `claim_expires_at < $5` predicate — all `TIMESTAMPTZ` columns (`store/migration/postgres/0.36/00__add_skill_executions.sql:17,19-22`). pgx will error at runtime: int64/bigint into a timestamptz expression. Fix 1's struct conversion (`ClaimedAt`/`ClaimExpiresAt` → `*time.Time`) reaches the INSERT and the scan, but **not** this locally-computed epoch pair, because `now` is not a struct field. SQLite is unaffected (epoch ints end-to-end; the plan's `timeToUnix`/`unixToTime` helpers keep it consistent).
- **Fix:** Enumerate in Fix 1: postgres `ClaimSkillExecution` must use `time.Now()` and `time.Now().Add(time.Duration(leaseSeconds) * time.Second)` for all three args. Add a PG/CRDB claim round-trip test row (see §4).

#### N5-2 — K-4 fixes the missing-variable case but not the missing-field case

- **File:** Fix 4 → `execution.go`/`evaluator.go`
- **Description:** The placeholder `map[string]any{"output": "", "skipped": true}` resolves "node never ran → var missing", but a CEL condition referencing a *field* the placeholder lacks — e.g. `search_kb.found == false` against a skipped `search_kb` — still raises a missing-key eval error rather than evaluating clean. The same holds for a node whose own condition was not met: `executeStep` returns `("", nil)`, the C3-2 contract falls back to `{"output": ""}`, and any downstream `…_node.field` check errors. The plan's own verification row ("skipped node → clean false") will trip on this as specified.
- **Fix:** Specify the mechanism in Fix 4, do not leave it to the implementer: either have `EvalConditionDynamic` treat missing-key eval errors as `Met=false` (documented), or `cel.DynType` + a `has()`-style tolerant map lookup. Optionally document the `has(search_kb.found)` idiom for tenants.

#### N5-3 — K-3 picks the weaker fallback despite the tenant being in scope

- **File:** Fix 6 → `service.go`
- **Description:** `toolCallingLoop` (`service.go:3556`) already holds `config` and already resolves `requireLLMConfig(ctx, config.TenantID)` for the RAG path (`service.go:3643`); its tool execution site `h.Execute(ctx, args, nil)` is at `service.go:3608`. Threading `&config.TenantID` into `args["tenant_id"]` there is a one-line change and keeps the tenant's engine config (custom model/key) active in the chat path. Falling back to `requireLLMConfig(ctx, 0)` silently substitutes env-var defaults even when the real tenant has a custom config.
- **Fix:** Prefer injecting `config.TenantID` in `toolCallingLoop`; keep the `tenant_id == nil` → default-config fallback only as the safety net.

#### N5-4 (info) — `ErrorMessage` must be appended to every SELECT list, not just the scan helper

- Both drivers' shared scan + the three standalone SELECT column lists in `store/db/postgres/agent_skill.go` (INSERT-following GET, UPDATE-following GET, list) must carry `error_message` or the scan misaligns. Postgres INSERT can omit the column (relies on `DEFAULT ''`). Add to the Fix 1 manifest explicitly.

#### N5-5 (info) — carried: postgres integration test gating

- `store/db/postgres/agent_skill_test.go` is `(integration)`. The plan lists it but does not state the CI/skip story. Either run it in a job with a real PG/CRDB, or mark it skipped by default with an env-driven opt-in — otherwise the P0 "Postgres round-trip" and new claim round-trip rows are vacuous on a local/no-PG run.

---

## 3. Plan6 Conformance (verified)

| plan6 requirement | code5 disposition | Assessment |
|-------------------|-------------------|------------|
| D2 — chat-path durable loop | DEFERRED | Acknowledged, non-blocking; N5-3 improves the interim |
| R3 — status guards | Fix 3 atomic methods | OK |
| D4 — CEL env + output semantics | Fix 4 | OK (after N5-2) |
| D5/R2 — recovery worker | Fix 7 | OK (K-5 closed) |
| D6 — outbound events | kept | OK |
| MaxRetries/Timeout honored | DEFERRED | Acknowledged gap |
| Cadence (30s ticker) | deviation | OK, documented |
| Endpoints + RBAC | kept + Fix 9 gating | OK |

No plan6 item regressed from code4_review. All five K-item resolutions confirm the code4_review verdict path.

---

## 4. Test-Plan Review

The plan now covers every K-item with a dedicated row. Edits to make:

| Addition / change | Guards |
|-------------------|--------|
| PG/CRDB claim round-trip: create → `ClaimSkillExecution` → running → complete/stop | N5-1 (would have caught it in code4) |
| Refine the K-4 row: assert `search_kb.found` against a skipped node is a clean `false`, **not merely "no error"** | N5-2 |
| `llm_call` invoked from the chat loop resolves the tenant's model/key (assert tenant config, not env default) | N5-3 |
| Keep the K-2 compile row, but reframe as the Fix-1 compile gate (checkpoint.go is now inside Fix 1) | conj. with K-2 |
| State the postgres integration CI/skip story | N5-5 |

Plus the manual list's two new checks should be: postgres claim produces no type error; skipped-node field condition evaluates, not errors.

---

## 5. Implementation Order — Confirmed

Fix 1 → Fix 3 → Fix 2 → Fix 4 → Fix 5 → Fix 7 → Fix 6 → Fix 8-10 → tests. Insertions: **N5-1 within Fix 1** (enumerate with the dialect work), **N5-2 within Fix 4** (specify the missing-key mechanism), **N5-3 within Fix 6** (thread `config.TenantID`). N5-4/N5-5 are last-pass housekeeping before/with the test step.

---

## 6. Bottom Line / Sign-off

- **Tree:** code5.md is un-implemented; all `code_review.md` findings remain live.
- **Plan:** all K-1…K-6 findings from code4_review are correctly adopted, verified against the code with file:line evidence. This is the first plan in the series with no rework item.
- **Gate condition:** **APPROVED (conditional)** — fold N5-1 into Fix 1, N5-2 into Fix 4, N5-3 into Fix 6 (all one-line-or-less additions with a matching test row each), then implement as ordered. There are no blocking defects remaining.
- **Echo of the standing constraint:** the tree is still the pre-fix implementation; nothing is verified until the plan is actually executed and the manual checklist (incl. claim-path PG round-trip) passes.