# bchat Durable Execution — Adversarial Review of code7.md (bugs/059)

**Reviewer:** DeepSeek (acting as senior Go architect)
**Date:** 2026-08-05
**Scope:** `bugs/059/code7.md` (Fix Plan for code6 implementation gaps) verified against the code6-derived implementation, the prior plan baseline (plan6.md → code6.md), and code6_imp_review.md (R-1…R-8).
**Method:** Read the full plan; re-checked each fix's claims against the current source (mtimes, exact lines) and against the empirical CEL results from code6_imp_review (cel-go error strings, dyn-nil field-access semantics). No source edits have been applied — this is a plan-approval gate.

---

## 0. Tree Status (verified)

code7.md (mtime 10:43) was written after the last source edit. The tree is byte-identical to the state reviewed in `code6_imp_review.md`:

- `execution.go` 09:34, `evaluator.go` 09:33, `service.go` 09:42, `checkpoint.go` 09:40, `skill_builtins.go` 07:27, `recovery.go` 07:33, `evaluator_test.go`/`skill_builtins_test.go` 07:27, store drivers 09:26/09:27.
- No code7.md change is present in the code. R-1…R-7 remain live until this plan is executed.

---

## 1. Fix-by-Fix Verification

### Green as drafted (verified against code + empirical probes)

- **Fix 1 (R-1, HIGH) — output contract.** JSON-map parse else `{"output": raw}` at execution.go:189 matches the code6.md Fix 4 spec. Empirically confirmed: with the fix in place, `search_kb = {"found":false}` makes `search_kb.found == false` evaluate `true` (currently impossible — raw-string dyn field access yields `nil → false` silently). Non-map JSON and invalid JSON correctly fall back to the wrapper. ✓
- **Fix 2 (R-2, HIGH) — missing-variable tolerance.** Two-part fix is correct:
  - `isMissingKeyError` gains `"no such attribute"` — empirically, cel-go's error for an absent declared variable is `no such attribute(s): search_kb` (probed in code6_imp_review). ✓
  - K-4 placeholder (`state[nodeName] = {"output":"","skipped":true}`) at execution.go:176-179 makes skipped-node vars resolve; field access on the placeholder yields absent→`nil`→`false`, no error (empirically confirmed). ✓
- **Fix 3 (R-3, MEDIUM) — chat-path tenant injection.** `vars := map[string]any{"tenant_id": config.TenantID}; h.Execute(ctx, args, vars)` at service.go:3657 reaches the `GenerateFn` closure's `vars["tenant_id"]` lookup; `config.TenantID` is `int32` and matches the type-switch `case int32` (service.go:224-225). Detached path already worked. ✓
- **Fix 5 (R-5, LOW) — stop-condition eval error logged.** Log-and-treat-as-not-met replaces the discarded `_` at execution.go:215. ✓
- **Fix 7 (R-7, LOW) — limit clamp.** `if limit < 1 { limit = 1 }` before the `limit > 200` cap at handlers.go:6877-6884 resolves the `?limit=0` empty-list and `?limit=-5` SQL-error-500 cases. ✓

### Must-fold corrections (small, but must be in the plan before implementation)

#### C-1 — Fix 4 sqlite tests use `&DB{}` with no connection → nil-deref panic

- **File:** code7.md §4b `store/db/sqlite/agent_skill_test.go` (all four tests)
- **Description:** Every test constructs `db := &DB{}`. The sqlite driver's unexported `db *sql.DB` field is nil, so `CreateSkillExecution` → `d.db.ExecContext` panics at runtime. The comment `// use test helper to get real DB` acknowledges the need but no helper is provided, and none exists in the package today (`store/db/sqlite` has only `memo_filter_test.go`, which never opens a DB; the established pattern is `store/test/store.go:90-111` — `t.TempDir()` + `db.NewDBDriver(profile)`).
- **Expected:** Specify a real setup: either an in-package `setupTestDB(t *testing.T) *DB` using `sqlite.NewDB` over `t.TempDir()` (mirroring the `store/test` pattern), or place the round-trip tests in the `store/test` package where the driver construction already exists. As drafted the suite cannot pass.

#### C-2 — Fix 6 omits the CockroachDB DDLs (same NOT NULL, same pgx driver)

- **Files:** `store/migration/cockroach/0.36/00__add_skill_executions.sql`, `store/migration/cockroach/LATEST.sql`
- **Description:** R-6 is applied to postgres DDL + postgres LATEST only. The cockroach files declare `tenant_id INT8 NOT NULL` (both `agent_skill_executions` and `agent_skill_logs`) — and CRDB is served by the postgres driver (C3-9), so a nil-tenant log/execution violates NOT NULL there too. `validate:parity`'s CRDB check is names-only, so the divergence would ship silently.
- **Expected:** Apply the same `NOT NULL → DEFAULT NULL` change to cockroach DDL + `LATEST.sql`, and add a parity/spelling assertion that postgres and cockroach tenant-or-nullability agree, or document the intentional divergence.

### NITS — fold in

- **N-1:** `TestEvalConditionDynamic_RawString_NoFieldAccess` (code7.md §4a) feeds a wrapper map `{"output":"logged"}`, not a raw string — the name and comment do not match the fixture. Either feed the actual Go string (which still returns `Met=false` per the empirical probe) or rename it "wrapper map". Its assertion is otherwise correct.
- **N-2:** Fix 4c's postgres tests are bodies-only (`// ... implementation with real DB connection ...`). "Tests implemented" is overstated until the N5-1 claim tour actually asserts something: connect (document the DSN/env), create → `ClaimSkillExecution` → `running` → complete/stop, and confirm TIMESTAMPTZ round-trips `time.Time` with no int64-into-timestamptz error.
- **N-3:** Fix 1 exposes a coercion edge: JSON-parsed node outputs carry numeric fields as `float64`, and CEL provides no `int == double` overload — a tenant condition like `kb.count == 5` (int literal) will eval-error (not a missing-key, so not caught by Fix 2). Either normalize ints after parse (mirroring C3-5's intent) or document the `== 5.0` / `has()` idiom for tenants.
- **N-4:** After Fix 1, `buildWorkflowOutput` (execution.go:359-378) renders node state via `%v` — outputs will show `map[found:true …]` instead of handler text. Prefer unwrapping the wrapper's `"output"` field for readability of the completed-output string and the D6 event payload.
- **N-5:** Verification section should add `./scripts/validate-migrations.sh` and `./scripts/validate-pg-migrations.sh` (Taskfile `validate-pg-migrations`) since postgres + cockroach `LATEST.sql` are edited; `task validate:migrations` covers sqlite LATEST sync only.
- **N-6:** R-8 (`EmitEvent` on stop) is intentionally not addressed — acceptable, but the plan should state it stays deferred rather than disappearing silently.

---

## 2. Conformance

| code6_imp_review finding | Fix | Disposition |
|---------------------------|-----|-------------|
| R-1 (HIGH) output contract | Fix 1 | GREEN (after Fix 1; N-3/N-4 nits) |
| R-2 (HIGH) tolerant eval | Fix 2 | GREEN (placeholder + matcher term) |
| R-3 (MEDIUM) chat tenant | Fix 3 | GREEN |
| R-4 (MEDIUM) tests | Fix 4 | BLOCKED on C-1; PG half-specified (N-2) |
| R-5 (LOW) stop error logged | Fix 5 | GREEN |
| R-6 (LOW) tenant nullability | Fix 6 | BLOCKED on C-2 (cockroach omitted) |
| R-7 (LOW) limit validation | Fix 7 | GREEN |
| R-8 (INFO) stop EmitEvent | — | Deferred (N-6) |

All previously verified-fixed items (CRITICAL-1/1b/2/3, N5-1, HIGH-3/4/5, MED/LOW/K/C3 set) are unaffected by these edits — Fix 1 and Fix 2 touch only `execution.go`/`evaluator.go` state handling; no store or migration behavior changes except Fix 6's nullability.

---

## 3. Bottom Line / Sign-off

- **Tree:** code7.md is un-implemented; R-1…R-7 remain live.
- **Plan:** all seven fixes are directionally correct and verified against the code and empirical CEL behavior. Two plan corrections and six nits remain.
- **Gate condition:** **APPROVED (conditional)** — fold C-1 (sqlite test setup — `&DB{}` panics) into Fix 4, C-2 (cockroach DDLs + parity assertion) into Fix 6, and the nits N-1…N-6 into their respective fixes. With those, this plan closes the code6_imp_review findings as drafted; implement in the listed order (Fix 1 → Fix 2 → Fix 3 → Fix 5/6/7 → Fix 4 last, since Fix 4 asserts Fixes 1-2 behavior).