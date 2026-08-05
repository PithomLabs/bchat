# bchat Durable Execution — Adversarial Review of code9.md Implementation (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** Implementation of `code9.md` per `code9_imp.md`, verified against the code8_review.md baseline (R-C1, R-C2, Section 3, N-N1…N-N6) and the live tree.
**Method:** Read `code9.md` + `code9_imp.md`; independently re-verified every fix against the source (mtimes, exact lines), ran the agent + store suites and `validate:parity`, and checked driver/CEL semantics against the actual code (cel-go `Int.Equal`, claim SQL, DDLs).

---

## 0. Tree Status (verified)

Implementation **is in the tree** (this round differs from code7/code8): all code9 targets modified 11:19–11:26, after code9.md (11:14). No further edits pending.

| File | mtime | Fix |
|------|-------|-----|
| execution.go | 11:19:57 | 1, 1b, 2, 5, 5b |
| evaluator.go | 11:19:30 | 2 |
| service.go | 11:20:15 | 3 |
| handlers.go | 11:20:25 | 7 |
| pg/crdb 0.36 DDLs | 11:20:42/45 | 6 |
| pg/crdb LATEST.sql | 11:21:03 | 6 |
| validate-parity.sh | 11:21:26 | 6b |
| evaluator_test.go | 11:26:16 | 4a |
| store/test/skill_execution{,_postgres}_test.go | new | 4b/4c |

Independent verification run (this review):
- `go test ./server/router/api/v1/agent/...` → **PASS (11.155s)**
- `go test ./store/test/... -run TestSkillExecutionRoundTrip -v` → **PASS** (sqlite); postgres variant **SKIPPED** (gated, as designed)
- `bash scripts/validate-parity.sh` → **PASS**, Check 2c `postgres=2, crdb=2`

---

## 1. Findings

### R-F1 (HIGH) — Postgres "lease re-entry" test contradicts the driver and deterministically fails

- **Plan intent (code9.md Fix 4b Case 3 / Fix 4c):** "second claim same worker → success (lease re-entry)."
- **Driver reality (both sqlite and postgres, identical semantics):**
  `store/db/postgres/agent_skill.go` ClaimSkillExecution:
  ```
  WHERE id = $4 AND (status = 'pending' OR (status = 'running' AND claim_expires_at < $5))
  ```
  There is **no same-worker carve-out**. After the first claim the row is `running` with `claim_expires_at = now+60s`, so a second claim yields `rows == 0` → `error("execution %s could not be claimed")`.
- **Test:** `skill_execution_postgres_test.go:80-82` performs an immediate re-claim by the same worker and asserts `require.NoError(t, err)`. This is a **guaranteed failure on any real Postgres/Cockroach run** (lease not expired, worker not exempt). Because the test is env-gated, the default `go test ./store/...` never executes it — so code9_imp's claim "all 8 cases pass with DRIVER=postgres" is both unverified and **unachievable** as written.
- **Root cause:** code9.md's "lease re-entry" expectation was never implemented in the driver (code6 round shipped lease-exclusive claims); the new test asserts the unimplemented intent.
- **Resolution options:**
  1. **Recommended — implement the same-worker carve-out in both drivers**, matching plan intent and adding a lease-heartbeat capability:
     ```sql
     AND (status = 'pending'
          OR (status = 'running' AND (claim_expires_at < $now OR claimed_by = $worker)))
     ```
     Then add the re-claim assertion to the **sqlite** round-trip test too (it currently does not test re-claim; the sqlite Case 3 only claims once, then Case 4 stops).
  2. Alternatively drop the re-claim block from the PG test and document lease-exclusive semantics.
- **Note:** option 1 changes driver behavior; must be covered by both round-trip tests (sqlite + pg) to lock it in.

### R-F2 (MEDIUM) — Fix 5b silently masks node-condition errors; reduces error observability

- After the empirical `map == string → false` discovery (confirmed by `TestEvalConditionDynamic_MapStringCompare_ReturnsFalse`, which **passes**), the errors `EvalConditionDynamic` actually returns are narrow: **compile errors** (malformed expressions) and missing-attribute strings not matched by the 4 `isMissingKeyError` terms.
- `executeStep` (execution.go:256-261) now folds all of them into `slog.Warn` + skip. Consequences:
  - A tenant condition typo (`search_kb.found == treu`) silently **skips the node forever**; execution completes and the defect is invisible in the transcript / `error_message` (K-1 path never fires). Pre-Fix-5b behavior hard-failed and surfaced the message.
  - The warn log lacks `exec_id` (unlike the adjacent stop-condition warn), so an operator cannot correlate which execution hit it.
- **Resolution options:** (a) add `"exec_id", exec.ID` to the warn; (b) make compile errors hard-fail again while runtime-eval errors stay tolerant — e.g., `EvalConditionDynamic` returns a typed compile error (from `env.Compile` issues) that `executeStep` treats as fatal and untyped runtime eval errors as not-met. Recommended: do both.

---

## 2. Nits / INFO

- **N-1 (code):** `service.go:3650-3673` — the Fix 3 edit is mis-indented (`if err != nil` at 4 tabs, closing `} else {` at 3 tabs vs the block's 3). `gofmt -l` flags the file (repo is not gofmt-clean generally, but this is a newly-introduced block). Run gofmt on the touched region. Functionally correct — braces balance, build passes.
- **N-2 (code):** `validate-parity.sh` Check 2c (`scripts/validate-parity.sh:316-328`) greps `tenant_id BIGINT DEFAULT NULL` / `INT8 DEFAULT NULL` across the **entire** LATEST files, not just the skill tables. Today only the skill tables match (counts 2/2 — verified). Blind spot: a future unrelated `tenant_id BIGINT DEFAULT NULL` line, or a single-table divergence where one skill table regresses, keeps the count ≥2 and passes. Scope the grep to the skill-table blocks (awk between `CREATE TABLE ... agent_skill_executions`/`agent_skill_logs` and the next `CREATE`).
- **N-3 (doc):** The N-3 justification "no int == double overload" is inaccurate. Verified cel-go `Int.Equal` (`common/types/int.go:215`) switches on `Double` and compares via `compareIntDouble` — `int == double` **evaluates, not errors**. `normalizeNumbers` is therefore harmless canonicalization, not load-bearing. Corollary (INFO): checkpoint serialization round-trips int64→float64 across resume; benign for CEL equality (same widening), so no functional gap.
- **N-4 (doc):** The output-contract change also reshapes the **llm_call prompt context**: `LLMHandler.Execute` (skill_builtins.go:139-143) marshals `vars` into the LLM prompt, so node outputs now appear as `{"output": ...}` / parsed maps instead of raw strings. Documented for CEL conditions but not for the LLM-visible context; tenant prompts comparing on raw node text will see the wrapper.
- **INFO:** `buildNodeOutput("{}")` → empty map without an `"output"` key → `buildWorkflowOutput` renders `map[]` (edge; acceptable). Pre-code9 checkpoints hold raw-string outputs; a resumed execution transiently mixes raw strings and maps until nodes re-run (migration note only).

---

## 3. Verified correct (no action)

- **Fix 1** (`buildNodeOutput` + recursive `normalizeNumbers`, execution.go:191, 370-406): JSON object → parsed map; array/scalar/invalid/`null` (`m == nil` guard) → `{"output": raw}`; single-run normalization incl. nesting. Square with all 5 `TestBuildNodeOutput_*` cases.
- **Fix 1b** (`buildWorkflowOutput`, execution.go:408-431): unwraps `"output"`, `strings.Join`, gofmt-clean, scope-documented.
- **Fix 2**: K-4 placeholder at deps-not-met (execution.go:178) + `"no such attribute"` term (evaluator.go:140). Both present.
- **Fix 5**: stop-condition eval error → `slog.Warn` + treated as not-met (execution.go:217-222), includes `exec_id`. Correct.
- **Fix 3** (semantics): `vars := map[string]any{"tenant_id": config.TenantID}` → `Execute`; `int32` matches `GenerateFn`'s `case int32` (service.go:224-225). Reaches tenant LLM config. (Indentation nit N-1 only.)
- **Fix 7**: `if limit < 1 { limit = 1 }` after the cap (handlers.go:6885-6886).
- **Fix 6**: All four DDL files in lockstep (`DEFAULT NULL`, lines 5/33 versioned, 1035/1063 LATEST); sqlite/mysql untouched. Verified by grep.
- **Fix 6b**: Check 2c present and passing (robustness nit N-2).
- **Fix 4a**: 9 evaluator tests incl. `_WrapperMap_`, `_RawString_`, `_MapStringCompare_` and 5 `buildNodeOutput` cases; agent suite passes; gofmt-clean. Empirical `map == string → false` confirmed.
- **Fix 4b**: 8-case sqlite round-trip passes — nil-tenant create, tenant-filter isolation (Len==1), claim (running/claimed_by), stop, fail+ErrorMessage, complete+CompletedAt, log round-trip, stop-on-pending. Uses `NewTestingStore` + existing `createBridgeTenant`.
- **Fix 4c**: Correctly gated (`getDriverFromEnv() != "postgres"` → skip), no invented env — **except** the R-F1 re-claim defect above.

---

## 4. Conformance

| code8_review gate | Implementation | Disposition |
|-------------------|----------------|-------------|
| R-C1 (teststore placement) | Fix 4b/4c in `store/test` | FIXED |
| R-C2 (4 DDLs + parity) | Fix 6 + Check 2c | FIXED (N-2 scope nit) |
| Section 3 (map contract) | Fix 1 + Fix 5b + docs; empirical `map==string→false` | FIXED (R-F2 observability residual) |
| N-N1 (keep raw-string) | both tests present | FIXED |
| N-N2 (no invented env) | `getDriverFromEnv()` gate | FIXED |
| N-N3 (recursive normalize) | recursive walk | FIXED (rationale nit N-3) |
| N-N4 (unwrap scope) | unwrap + documented map rendering | FIXED |
| N-N5 (script not task) | verification uses script path | FIXED |
| N-N6 (deferred attribution) | corrected table in code9_imp §8 | FIXED |
| — | plan-specified lease re-entry | **R-F1 — not implemented, test asserts unimplemented behavior** |

---

## 5. Bottom Line / Sign-off

- **Tree:** code9.md implemented and verified; agent suite, store round-trip, and parity checks all green in this review.
- **Plan-requirement gap:** lease re-entry by the same worker is a code9.md-specified behavior that the drivers never implemented; the postgres test asserts it and will fail on a real PG/Cockroach run. This is the one item that must be reworked.
- **Observability regression:** Fix 5b silently converts node-condition errors into skips; restore hard-fail on compile errors (typed error) and/or add `exec_id` to the warn.
- **Gate condition:** **RE-WORK (conditional)** — resolve R-F1 (implement the same-worker carve-out in both drivers and extend the sqlite test, or drop the assertion) and R-F2 (compile-error vs runtime-error distinction + exec_id). Folds nits N-1…N-4; the rest is approved and verified.