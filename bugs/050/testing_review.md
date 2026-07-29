# Testing Review: `testing.md` — Adversarial Review

**File:** `bugs/050/testing.md`
**Skill repo reference:** `cockroachdb-skills-main/skills/cockroachdb-query-and-schema-design/cockroachdb-sql/references/cockroachdb-rules/01-schema-design.md:152`
**Code reference:** `vectordb_cockroach.go:108-127`

---

## Verdict: APPROVED WITH NITS

testing.md accurately documents the local CRDB Docker setup and a critical bug: `vector_ip_ops` is NOT supported in CRDB v25.2.21 (error 0A000, issue #144016). The skills repo at `01-schema-design.md:152` shows `vector_ip_ops` in its example, but actual testing proves the operator class is not yet implemented. The code at `vectordb_cockroach.go:112-113` uses `vector_ip_ops`, causing index creation to fail. The error handler checks for 42P07 (duplicate_table) but the actual error is 0A000 (feature_not_supported), so it falls through to the warning-and-continue path.

### Verified Correct

| # | Check | Verdict |
|---|-------|---------|
| 1 | Container config: `v25.2.21`, `start-single-node --insecure --advertise-addr=localhost` | ✅ Correct for local Docker testing |
| 2 | Feature flag `SET CLUSTER SETTING feature.vector_index.enabled = true` | ✅ Correct syntax; persists via volume; no restart required |
| 3 | Table DDL matches `vectordb_cockroach.go:84-95` (columns, types, VECTOR(1536), JSONB) | ✅ Exact match |
| 4 | `CREATE USER bchat_user` (no password) for insecure mode | ✅ Correct — `WITH PASSWORD` fails with 28P01 in insecure mode |
| 5 | DSN `postgresql://bchat_user@localhost:26257/bchat?sslmode=disable` | ✅ Correct for insecure mode; no password required |
| 6 | Healthcheck `cockroach node status --insecure --host=localhost --port=26257` | ✅ Correct; works inside container |
| 7 | Volume `-v bchat_crdb_data:/cockroach/cockroach-data` persists across `down` (without -v) | ✅ Correct; survives restarts |
| 8 | Ports 26257 (SQL) + 8080 (DB Console) | ✅ CRDB defaults |
| 9 | No missing setup steps | ✅ Complete |
| 10 | Docker preferred over binary for this use case | ✅ Correct — `docker compose` is simpler for local dev than `cockroach start-single-node` directly |
| 11 | Code error handling catches `vector_ip_ops` failure gracefully (though via 0A000, not 42P07) | ✅ Correct — falls back to brute-force search with a warning |
| 12 | `--insecure` safe for local dev | ✅ Correct; no port mapping gotchas with Docker |

### Findings

| # | Category | Severity | Finding | Fix |
|---|----------|----------|---------|-----|
| 1 | **Code bug** | CRITICAL | `vector_ip_ops` is NOT supported in CRDB v25.2.21. The code at `vectordb_cockroach.go:112-113` uses `vector_ip_ops` → index creation fails with 0A000 (feature_not_supported). The error handler only checks `pgErr.Code == "42P07"` (duplicate_table) — 0A000 falls through to the warning-and-brute-force path. The skills repo (`01-schema-design.md:152`) shows `vector_ip_ops` in its example but the feature is gated behind CRDB issue #144016 (not yet implemented). | Remove opclass from DDL: `CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding)` — defaults to `vector_l2_ops`. Also update the error handler to check for 0A000 alongside 42P07 for a more accurate log message. |
| 2 | **Presentation** | NIT | Line 49 "What Was Verified" table shows the vector index with ✅ but this reflects the **workaround result** (no opclass, defaulting to `vector_l2_ops`), not the **code result** (`vector_ip_ops` fails). A reader could misinterpret this as the code's index creation working correctly. | Add a footnote to the Vector index row: "Created via workaround (opclass omitted). Code's `vector_ip_ops` fails — see Key Finding below." |

### Key Reconciliation: Skills Repo vs Code vs Testing

| Source | Statement | Reality |
|--------|-----------|---------|
| Skills repo `01-schema-design.md:152` | `CREATE VECTOR INDEX ... (embedding vector_ip_ops)` | ❌ Does not work in CRDB v25.2.21. Feature #144016 not yet implemented. |
| Code `vectordb_cockroach.go:112-113` | `CREATE VECTOR INDEX ... (embedding vector_ip_ops)` | ❌ Fails in v25.2.21. Error handler catches via 42P07-only check; 0A000 falls to warning path. |
| testing.md (workaround) | `CREATE VECTOR INDEX ... (embedding)` — no opclass | ✅ Defaults to `vector_l2_ops`. The `<=>` operator works (C-SPANN adapts), but index is optimized for L2, not cosine. |

### Next Action

**Fix `vectordb_cockroach.go:112-113`** — remove `vector_ip_ops` opclass from the CREATE VECTOR INDEX DDL. The workaround DDL at `testing.md:73` (`CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding)`) has been verified as working on CRDB v25.2.21.
