# Testing4 Review: `testing4.md` — Adversarial Review

**File:** `bugs/050/testing4.md`
**Skill repo reference:** `cockroachdb-skills-main/skills/cockroachdb-query-and-schema-design/cockroachdb-sql/references/cockroachdb-rules/01-schema-design.md:150-162`
**Previous reviews:** `testing_review.md`, `testing2_review.md`, `testing3_review.md`

---

## Verdict: APPROVED WITH NITS

testing4.md is a post-implementation results document — the code fix is applied and verified against both v25.2.21 and v26.2.4. **Critical discovery: `vector_ip_ops` WORKS on v26.2.4** (✅ Success) but fails on v25.2.21 (0A000). This validates the skills repo examples (`01-schema-design.md:152-159`) as correct for v26.2.4+, but they don't acknowledge the v25.2.21 gap, and no version note exists in the skills repo.

The code fix (no opclass → defaults to `vector_l2_ops`) is correct for cross-version compatibility. The tradeoff between universal compatibility (current approach) vs version-conditional `vector_ip_ops` (future optimization) should be documented explicitly.

### Findings

| # | Category | Severity | Finding | Fix |
|---|----------|----------|---------|-----|
| 1 | **Name resolution** | MEDIUM | Lines 86-87 run `CREATE DATABASE test; CREATE TABLE test.vec (...)` without `-d test` or `USE test`. The two-part name `test.vec` may resolve as `defaultdb.test.vec` (schema=test, not database=test). Subsequent commands (lines 90-103) use `-d test` and reference `test.vec` — if the table was created in the wrong database/schema, they'd fail. The reported ✅ suggests CRDB handles it, but the steps may not be reproducible on a fresh session. | Use `USE test;` after `CREATE DATABASE test` in the same `-e` string, or use `-d test` for the CREATE TABLE command. |
| 2 | **Code fix vs v26.2 tradeoff** | MEDIUM | `vector_ip_ops` works on v26.2.4 but the fix removes it unconditionally. The doc doesn't discuss this design tradeoff. Removing the opclass is correct for compatibility, but the decision to forgo `vector_ip_ops` on v26.2+ lacks justification. | Add a subsection: "Tradeoff: No version-conditional opclass — `vector_ip_ops` would work on v26.2.4+ but would break on v25.2.21. Version detection deferred as future enhancement." |
| 3 | **Skills repo version scope** | MEDIUM | The skills repo (`01-schema-design.md:152`) shows `vector_ip_ops` without any version qualifier. testing4.md proves it works on v26.2.4 but not v25.2.21. The doc should call out that the skills repo examples target v26.2.4+. | Add a note: "Skills repo `01-schema-design.md:152` examples validated for v26.2.4+; v25.2.21 requires omitting the opclass (defaults to `vector_l2_ops`)." |
| 4 | **Test coverage gap** | NIT | Section 4 shows `go test -tags cockroach` passing in 8.598s but doesn't explain these are memory/LanceDB tests, not CRDB tests. A reader may think CRDB integration tests passed. | Add brief note: "Note: `go test` runs memory and LanceDB tests only — no cockroach-specific integration tests exist yet (compile-check only)." |
| 5 | **Reconciliation table format** | NIT | The version matrix in lines 111-117 shows the comparison but doesn't include the full reconciliation (skills repo vs code before/after vs testing). | Ensure `testing.md` reconciliation includes four rows: skills repo, code before fix, code after fix, testing4.md workaround — each with v25.2.21 / v26.2.4 columns. |

### Verified Correct

| Q | Check | Verdict |
|---|-------|---------|
| 1 | `switch pgErr.Code` pattern (42P07, 0A000, default, non-PG else) | ✅ Correct — no leak paths |
| 1 | Non-PG else branch handles network errors, timeouts, context cancellation | ✅ Correct |
| 1 | Backward compatible with v25.2.21 (no opclass works on both versions) | ✅ Correct |
| 2 | v26.2 Docker port 26258/8081 avoids conflict with v25.2 on 26257/8080 | ✅ Correct |
| 2 | Cleanup: `docker stop bchat-crdb-v26 && docker rm bchat-crdb-v26` | ✅ Complete |
| 3 | `vector_ip_ops` works on v26.2.4 (unexpected, confirmed) | ✅ Correct — skills repo examples validated for 26.2+ |
| 3 | Default opclass (no opclass) works on both versions | ✅ Correct |
| 3 | Cosine distance `<=>` works with `vector_ip_ops` index on v26.2 | ✅ Correct — C-SPANN adapts to query operator |
| 4 | `go build -tags cockroach ./server/router/api/v1/agent/` passes | ✅ Compilation check |
| 4 | `go build ./server/router/api/v1/agent/` (no tag) passes | ✅ Stub check |
| 5 | Cosine query `1 - (embedding <=> ...)` AS similarity — distance-to-similarity formula | ✅ Correct formula for cosine similarity |
| 5 | Similarity scores correct for test data | ✅ [0.1,0.2,0.3] vs identity → distance 0 → similarity 1 |
| 8 | No regression for existing deploy — 42P07 catches pre-existing index | ✅ Correct on both v25.2.21 and v26.2.4 |
| 8 | Fresh deploy on v25.2.21 works (no opclass) | ✅ Correct |
| 8 | Fresh deploy on v26.2.4 works (no opclass, suboptimal) | ✅ Correct — C-SPANN works with any opclass |
