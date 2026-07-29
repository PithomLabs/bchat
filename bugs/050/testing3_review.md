# Testing3 Review: `testing3.md` — Adversarial Review

**File:** `bugs/050/testing3.md`
**Skill repo reference:** `cockroachdb-skills-main/skills/cockroachdb-query-and-schema-design/cockroachdb-sql/references/cockroachdb-rules/01-schema-design.md:150-162`
**Previous review:** `testing2_review.md` (7 findings, 1 CRITICAL, 1 HIGH, 2 MEDIUM, 3 NIT)

---

## Verdict: APPROVED WITH NITS

testing3.md addresses **all 7 findings** from `testing2_review.md`:

| testing2_review Finding | Severity | Status |
|--------------------------|----------|--------|
| #1 Non-existent test function | CRITICAL | ✅ Replaced with compile check + transparent coverage note |
| #2 Unverified v26.2 claim | HIGH | ✅ Added explicit v26.2.4 Docker test step |
| #3 Duplicate `errors.As` | MEDIUM | ✅ Fixed with single call + `switch pgErr.Code` |
| #4 Step ordering | MEDIUM | ✅ CRDB started before any SQL commands |
| #5 Missing IF NOT EXISTS | NIT | ✅ Added to CREATE TABLE DDL |
| #6 docker compose version | NIT | ✅ Noted in prerequisites |
| #7 Reconciliation table stale | NIT | ✅ Documented in section 3 |

The skills repo (`01-schema-design.md:150-162`) shows `vector_ip_ops` in all examples — aspirational for a future CRDB version. testing3.md correctly removes the opclass, acknowledges the gap, and adds a v26.2.4 Docker verification step. The remaining findings are minor documentation/safety nits.

### Findings

| # | Category | Severity | Finding | Fix |
|---|----------|----------|---------|-----|
| 1 | **Docker tag** | NIT | `v26.2.4` Docker tag existence is assumed but not verified. If the tag doesn't exist on Docker Hub, the v26.2 verification step fails with `manifest not found`. | Add a prerequisite note: verify tag exists via `docker pull cockroachdb/cockroach:v26.2.4` before proceeding. |
| 2 | **v26.2 DDL style** | NIT | Line 101 omits the index name: `CREATE VECTOR INDEX ON test.vec (embedding vector_ip_ops)`. The skills repo (lines 152-159) always uses explicit names (e.g., `idx_embedding`). While the 0A000 error fires at parse time (before name validation), matching the skills repo pattern is safer against future CRDB versions that might reject unnamed VECTOR INDEX. | Add explicit name: `CREATE VECTOR INDEX idx_vec_ip ON test.vec (embedding vector_ip_ops)` |
| 3 | **Command consistency** | NIT | Step 3 (DROP TABLE) shown as raw SQL while steps 1-2 use `docker exec` inline syntax. Not functionally wrong, but inconsistent presentation. | Wrap step 3 in docker exec: `` `docker exec bchat-crdb cockroach sql --insecure --host=localhost -d bchat -e "DROP TABLE IF EXISTS agent_vectors CASCADE;"` `` |
| 4 | **Line range reference** | NIT | Step 4 says "edit `vectordb_cockroach.go:113`" but the fix spans both line 113 (remove opclass) and lines 115-127 (error handler). The single-line reference is misleading. | Update to "edit `vectordb_cockroach.go:113,115-127` per section 1" |

### Verified Correct

| Q | Check | Verdict |
|---|-------|---------|
| 1 | `switch pgErr.Code` pattern handles all branches (42P07, 0A000, default, non-PG else) | ✅ Correct — idiomatic, no leak paths |
| 1 | Non-PG error else branch needed | ✅ Correct — handles network errors, timeouts, context cancellation |
| 2 | v26.2 ports 26258/8081 avoid conflict with v25.2 on 26257/8080 | ✅ Correct |
| 2 | v26.2 test table `CREATE TABLE test.vec (id INT PRIMARY KEY, embedding VECTOR(3))` | ✅ Correct syntax |
| 2 | `docker stop && docker rm` cleanup | ✅ Correct |
| 3 | CRDB started (step 1) before SQL (step 3+) | ✅ Correct ordering |
| 3 | `DROP TABLE IF EXISTS` safe whether table exists or not | ✅ Correct |
| 4 | `CREATE TABLE IF NOT EXISTS` matches `vectordb_cockroach.go:84` | ✅ |
| 4 | `CREATE VECTOR INDEX` does NOT support IF NOT EXISTS | ✅ Correct — 42P07 handler is needed |
| 5 | Compile checks (`-tags cockroach` + no tag) verify both build paths | ✅ Sufficient for now |
| 5 | Test coverage gap transparently documented | ✅ Honest acknowledgment |
| 6 | Reconciliation table code row: ❌ → ✅ after fix | ✅ Documented in section 3 |
| 7 | Docker compose v2 compatibility note in prerequisites | ✅ Sufficient — v1 is deprecated |
