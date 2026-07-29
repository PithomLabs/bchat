# Testing2 Review: `testing2.md` — Adversarial Review

**File:** `bugs/050/testing2.md`
**Skill repo reference:** `cockroachdb-skills-main/skills/cockroachdb-query-and-schema-design/cockroachdb-sql/references/cockroachdb-rules/01-schema-design.md:150-162`
**Code reference:** `vectordb_cockroach.go:108-127`

---

## Verdict: APPROVED WITH NITS

The code fix (remove `vector_ip_ops`, default to `vector_l2_ops`) is correct. The skills repo at `01-schema-design.md:152` shows `vector_ip_ops` in all examples, but testing proves it is NOT implemented in CRDB v25.2.21 (issue #144016) — the skills repo examples appear aspirational for a future CRDB version. The proposed DDL `ON agent_vectors (embedding)` (no opclass) matches what testing.md verified as working. However, the verification plan has a critical gap: the test command targets a non-existent test function.

### Findings

| # | Category | Severity | Finding | Fix |
|---|----------|----------|---------|-----|
| 1 | **Test coverage** | CRITICAL | `go test -tags cockroach -run TestVectorDB` (line 102) matches **zero tests**. No `vectordb_cockroach_test.go` exists; no test file carries `//go:build cockroach`; no `TestVectorDB` or `TestCockroach*` function exists anywhere in the agent package. Command passes silently with `ok` and zero tests run. | Replace with a meaningful command, e.g. `go build -tags cockroach ./server/router/api/v1/agent/` (compile check only), or add a new `vectordb_cockroach_test.go` with `//go:build cockroach` that connects via `COCKROACH_DSN` to the local Docker instance. |
| 2 | **v26.2 claim** | HIGH | Doc states "v26.2.4 also does not support vector_ip_ops (verified 2026-07-29)" but this was **not tested** — only v25.2.21 was pulled and verified. No v26.2.x Docker image was inspected. The skills repo shows `vector_ip_ops` in all examples, which could target a v26.x where the operator class ships. | Remove unverified v26.2 claim, or add a verification step to pull `cockroachdb/cockroach:v26.2.4` and test `vector_ip_ops` explicitly before documenting as fact. |
| 3 | **Duplicate errors.As** | MEDIUM | Lines 42/44 call `errors.As(err, &pgErr)` twice (once for 42P07, once for 0A000). Works but shadows the variable and is redundant. Skills repo has no pattern guidance here. | Replace with single `errors.As` + `switch pgErr.Code { case "42P07": ... case "0A000": ... default: ... }` |
| 4 | **Step ordering** | MEDIUM | Step 1 (`DROP TABLE`) assumes CRDB is already running, but step 5 (`docker compose up -d`) starts it. If CRDB was previously stopped, step 1 fails with connection error. | Reorder: start CRDB first, then drop table, then apply fix, build, test, verify. |
| 5 | **Missing IF NOT EXISTS** | NIT | Line 115 uses `CREATE TABLE agent_vectors` without `IF NOT EXISTS`. The drop at step 1 is `IF EXISTS CASCADE` so the table is gone, but adding `IF NOT EXISTS` is defensive against re-runs without the drop. | Use `CREATE TABLE IF NOT EXISTS agent_vectors` (matches the code at `vectordb_cockroach.go:84`) |
| 6 | **docker compose version** | NIT | All commands use `docker compose` (v2). Users with `docker-compose` (v1) get `command not found`. | Add note: "Requires `docker compose` v2 (not `docker-compose` v1)" |
| 7 | **Reconciliation table stale** | NIT | testing2.md updates testing.md but doesn't mention updating the reconciliation table in `testing_review.md`. After the code fix, the code row changes from ❌ to ✅. | Add a note that `testing_review.md` reconciliation table should also be updated. |

### Verified Correct

| # | Check | Verdict |
|---|-------|---------|
| 1 | Remove opclass → defaults to `vector_l2_ops` | ✅ Correct — confirmed by testing.md |
| 1 | `<=>` works with `vector_l2_ops` (unoptimized) | ✅ Correct — C-SPANN adapts to query operator (L2 index, cosine query — less efficient but functional) |
| 2 | `0A000` is correct SQLSTATE for "feature not supported" | ✅ Correct per PostgreSQL/CRDB spec |
| 2 | `pgconn.PgError.Code` surfaces SQLSTATE correctly | ✅ Correct per pgx/v5 API |
| 3 | Non-fatal path acceptable for production | ✅ Correct — same pattern as existing code; graceful degradation |
| 5 | `go build -tags cockroach ./...` includes the fixed file | ✅ Correct — `vectordb_cockroach.go:1` has `//go:build cockroach` |
| 5 | `go build` (no tag) excludes the file | ✅ Correct — stub files with `//go:build !cockroach` take over |
| 6 | No regression for existing deploy with `vector_l2_ops` index | ✅ Correct — 42P07 handler catches "already exists" |
| 7 | Remove "Code Fix Needed" section from testing.md | ✅ Correct — fix is now implemented |
| 7 | Add footnote to Vector index row in verification table | ✅ Correct — clarifies workaround vs code result |
