# bchat Durable Execution — Adversarial Review of code8.md (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** `bugs/059/code8.md` (fix plan folding code7_review.md corrections C-1/C-2 + nits N-1…N-6) verified against the code7-review baseline (plan6.md → code6.md → code6_imp_review.md → code7.md → code7_review.md) and the current tree.
**Method:** Re-verified every claimed fix target against the live source (mtimes, exact lines) and against the existing test/validation infrastructure (`store/test`, `scripts/validate-*.sh`, `Taskfile.yml`). No implementation edits applied — this is a plan-approval gate.

---

## 0. Tree Status (verified — critical)

**code8.md has NOT been implemented.** Every file it touches is byte-identical to the state reviewed in `code7_review.md`:

| Target | mtime | Gate state |
|--------|-------|-----------|
| execution.go | 09:34 | No Fix 1 JSON-parse, no K-4 placeholder (deps-skip), no stop-error log (215), no N-4 unwrap |
| evaluator.go | 09:33 | `isMissingKeyError` lacks `no such attribute` (R-2) |
| evaluator_test.go | 07:27 | No `EvalConditionDynamic` tests (Fix 4a) |
| service.go | 09:42 | `h.Execute(ctx, args, nil)` at :3657 (Fix 3 not applied) |
| handlers.go | 07:38 | No `limit >= 1` clamp (Fix 7) |
| postgres 0.36 DDL | 09:19 | `tenant_id BIGINT NOT NULL` (Fix 6 not applied) |
| cockroach 0.36 DDL | 09:19 | `tenant_id INT8 NOT NULL` (C-2 not applied) |
| postgres/cockroach LATEST.sql | 09:20 | Skill blocks still `NOT NULL` (lines 1035, 1063) |
| agent_skill_test.go | — | Does not exist anywhere (Fix 4b/4c) |

R-1…R-8 therefore remain live until this plan is executed. Review below is of the *plan's correctness*.

---

## 1. Corrections — findings

### R-C1 — C-1 as drafted cannot compile or run (blocker)

Three independent defects in the `setupTestDB` snippet (code8.md §C-1):

1. **Self-import.** File declares `package sqlite` then imports `sqlitedb "github.com/usememos/memos/store/db/sqlite"` — a package importing its own path is a compile error. (The existing `memo_filter_test.go` is also `package sqlite`, so the internal-test convention exists — but it never imports itself.)
2. **Nonexistent API.** `migration.New(db.GetDB(), p)` — `store/migration/` is a directory of driver subfolders with **no root Go package** (no `store/migration/*.go`). The migration engine is `Store.Migrate` in `store/migrator.go:38` (with `//go:embed migration`). No `New` constructor exists to call.
3. **Import cycle for the intended fallback.** An in-package `sqlite` test cannot call `store.New(driver, p)` + `store.Migrate(ctx)`: `store` imports `store/db` which imports `store/db/sqlite`. (An external `package sqlite_test` *could* import `store`, but that still duplicates machinery that already exists.)

**Correction:** use the established driver-test pattern instead — relocate the round-trip tests to `store/test/` (package `teststore`), which has `NewTestingStore(ctx, t)` (`store/test/store.go:24`) already doing driver + full migration on a temp dir. Create `skill_execution_test.go` (runs on default `sqlite`) + `skill_execution_postgres_test.go` (gated `if driver != "postgres" { t.Skip(...) }`, per `agent_lead_postgres_test.go:17` / `bridge_postgres_cascade_test.go:17`). This simultaneously replaces the invented N-2 infrastructure (see N-N2) and avoids any migration-package coupling.

### R-C2 — C-2 is necessary but incomplete; parity assertion was dropped (blocker)

C-2 correctly identifies that CRDB is served by the postgres driver and that both cockroach tables declare `tenant_id INT8 NOT NULL` (lines 1035, 1063 — verified). However:

1. **My code7_review C-2 explicitly asked for a pg↔crdb parity/spelling assertion.** code8.md does not fold this in. `scripts/validate-parity.sh` treats cockroach as a "minimal mirror (inert; version machinery only)" and compares only table/index **names** against postgres (`extract_tables` grep at lines 204-225) — nullability is never compared, so a future divergence ships silently. Add a check (or a convention note) that postgres and cockroach skill-table DDL stay in lockstep beyond names.
2. **All four files must change together:** postgres 0.36 DDL + postgres LATEST (from code7 Fix 6) and cockroach 0.36 DDL + cockroach LATEST (C-2). The skill blocks are otherwise byte-identical in layout across the two LATEST files (1035/1063), so a partial apply is easy to spot but must not happen.

---

## 2. Nits — findings

### N-N1 — N-1 rename drops raw-string coverage (code fix)

Renaming `TestEvalConditionDynamic_RawString_NoFieldAccess` → `_WrapperMap_NoFieldAccess` removes the genuine raw-string input case from unit coverage. Both are legal `EvalConditionDynamic` inputs:
- raw Go string `"logged"` → dyn field access yields `<nil>` → `Met=false`, no error (empirically probed in code6_imp_review);
- wrapper map `{"output": "logged"}` → same.

Add the wrapper-map test (per N-1 rename) **and** keep a true raw-string test — the execution-level spot-check at code8.md:225 (`search_kb = "logged"` → `Met=false`) depends on that behavior.

### N-N2 — N-2 invents infrastructure that does not exist (code fix)

`SKILL_PG_INTEGRATION_TEST` and `skipUnlessIntegration` appear **nowhere** in the repo. The established postgres gate is `getDriverFromEnv()` (`store/test/store.go:109`, default `sqlite`) + `if driver != "postgres" { t.Skip(...) }`, with the connection DSN from the `DSN` env var (`getTestingProfile`, store.go:95). Folding N-2 into R-C1 (teststore placement) removes the need for a bespoke env/helper entirely.

### N-N3 — N-3 normalization is top-level only (code fix)

The snippet fixes only keys at the top level of the parsed map. Nested JSON — `search_kb = {"meta":{"count":5}}` then `search_kb.meta.count == 5` — leaves `float64` in place → CEL eval error (no `int == double` overload) that Fix 2 will not swallow. The spot-check "kb.count == 5 works after float64 normalization" holds only for flat outputs. Make the normalization a recursive walk (small helper) or explicitly document the flat-only contract. (Also note: the `f == float64(int(f))` guard is fine on 64-bit; `int` saturates only for float64 > 2^63 — negligible.)

### N-N4 — N-4 unwrap is correct for the wrapper; parsed-JSON maps still render as maps (accept)

Code8's snippet (execution.go:359-378) renders the fallback wrapper's `"output"` correctly, and graph-only nodes (`graph:name`) come through the wrapper too. Field-indexed parsed maps (`{"found":true}`) intentionally render as `map[...]` — that is the design. The spot-check "shows handler text, not map[...]" therefore holds only for transactional/wrapper outputs; state that in the plan so the spot-check is scoped honestly.

### N-N5 — verification command is wrong (plan fix)

`task validate:pg-migrations` does not exist. Taskfile task names are `validate:migrations`, `validate:schema`, `validate:parity` only (Taskfile.yml:29/34/56). The pg script is invoked as `./scripts/validate-pg-migrations.sh` (wired into `fly:check:postgres`, Taskfile:205/229) and is **docker-backed** (spins up a PG container, `scripts/validate-pg-migrations.sh:29`). Also `task validate:migrations` already runs `./scripts/validate-migrations.sh`, so listing both is redundant. Correct the block to:
```
go build ./...
go test ./store/... ./server/router/api/v1/agent/...
task validate:migrations
task validate:schema
task validate:parity
./scripts/validate-pg-migrations.sh   # docker-backed; optional/CI
```

### N-N6 — deferral-table attributions are off (plan fix)

R-8 was raised in **code6_imp_review.md** (INFO) and dropped in code7.md/code7_review N-6 — not "deferred per code6.md"; code6.md's conformance table (line 289) defers only D2 and MaxRetries/Timeout. MED-3/MED-4/LOW-2/LOW-4 are legacy codes from the code2–code4 chain whose meanings should be pinned in the table. Directionally the table is right; the wording is wrong. (Of the five "MED/LOW" items currently deferred, note the earlier rounds did not actually close LOW-2 CEL-program caching or LOW-4 recovery dedup either — confirm they were not quietly lost.)

---

## 3. Behavior-contract gap in Fix 1 (must be resolved pre-implementation)

After Fix 1, a completed node's state value is **always a map** (parsed JSON or `{"output": raw}`). Consequences the plan does not acknowledge:

- **Tenant conditions that compare a node output to a string literal** — `search_kb == "logged"` (valid before Fix 1 on the raw string) — now fail at eval with a CEL no-matching-overload error `== (map, string)`. This is not a missing-key error, so Fix 2's tolerance does not swallow it: `executeStep` returns the wrapped error and the workflow fails.
- **Stop-condition** errors under the same conditions: Fix 5 logs instead of failing — consistent.
- **Resolution options:** (a) declare and document the map contract (`output`/field access) so tenant conditions are correct; and/or (b) route node-`Condition` eval errors through the same tolerant log-and-treat-as-not-met umbrella as the missing-key path, keeping hard failures only for compile errors. At minimum the contract change must be stated — silently changing `search_kb == "logged"` from `false` to a hard workflow failure is a user-visible regression under the "no silent breakage" earlier-round principle.

---

## 4. Conformance

| code7_review gate | code8 treatment | Disposition |
|-------------------|-----------------|-------------|
| C-1 (sqlite test setup) | §C-1 | **R-C1 — will not compile/run as drafted**; use teststore |
| C-2 (cockroach DDLs + parity) | §C-2 | **R-C2 — DDL correct lines, parity assertion dropped** |
| N-1 (rename) | §N-1 | Fold w/ N-New1 (keep raw-string case) |
| N-2 (pg bodies) | §N-2 | Invented infra; absorbed by R-C1 |
| N-3 (float normalization) | §N-3 | **Top-level only**; recursive or document flat-only |
| N-4 (buildWorkflowOutput) | §N-4 | Accept (scope the spot-check) |
| N-5 (validation scripts) | §N-5 | **`task validate:pg-migrations` does not exist**; use script path |
| N-6 (deferred note) | §N-6 | Attribution wrong; otherwise OK |

Underlying code7.md fixes 1/2/3/5/7 remain sound (verified against execution.go/evaluator.go/service.go/handlers.go); R-1/R-2 empirical evidence still holds. New gap from Fix 1's map contract (Section 3) must be resolved before implementation.

---

## 5. Implementation-order notes

- Sound: Fix 1 → N-3 → N-4 → Fix 2 → Fix 3 → Fix 5/6/7 ordering; Fix 4a dynamic tests correctly gated on Fix 1 + Fix 2.
- **Run the round-trip tests (Fix 4b) with a nil-`TenantID` insert first** — this proves R-6 semantics on sqlite (NULL allowed) and makes the postgres NOT NULL constraint fail red before Fix 6 turns it green. Nice adversarial check of the fix's necessity.
- Steps 10/11 both name the file `agent_skill_test.go` across two packages — ambiguous; under R-C1 both become `store/test/skill_execution_test.go` + `store/test/skill_execution_postgres_test.go`.

---

## 6. Bottom Line / Sign-off

- **Tree:** code8.md is un-implemented; R-1…R-8 live.
- **Plan:** corrections C-1/C-2 are directionally right but one is uncompilable (R-C1) and one is incomplete (R-C2); nits N-1…N-6 have code/command defects (N-N1/N-N2/N-N3/N-N5) and attribution errors (N-N6); Fix 1 introduces an unstated behavior-contract change (Section 3).
- **Gate condition:** **RE-WORK (conditional)** — must fold R-C1 (relocate to teststore) and R-C2 (apply all four DDL files + add the pg↔crdb assertion) and resolve Section 3 before implementation. Nits N-N1…N-N6 are foldable into the same pass. After that, code8.md can be implemented in the listed order with the round-trip test run first.