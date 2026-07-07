# Adversarial Review: plan_028_v2.md

**Verdict: APPROVED WITH NITS** — v2 fixes every substantive error from v1. The plan is now implementation-ready with two minor wording/spec issues.

---

## What v2 Fixed Correctly

| v1 Issue | v2 Resolution |
|----------|--------------|
| Phase 1a implied both DBs needed 5 columns | v2 explicitly says "These columns exist in SQLite LATEST.sql but are missing from Postgres LATEST.sql" |
| Phase 1d contradicted body vs. note | v2 shows inline `CHECK (...)` clauses inside `CREATE TABLE` blocks, consistent with the fresh-DB note |
| Phase 2e overgeneralized bridge FK gap | v2 1g/2e correctly notes Postgres already has FK on 3 of 4 bridge tables; only `bridge_handoffs` needs it in Postgres |
| Phase 3 overstated store-layer work | v2 correctly reduces to only `vector_db_s3_override` missing from Postgres `rbac.go`; other columns already present in Go queries |
| Phase 5b created a coverage hole | v2 adds Phase 5d (Postgres bridge cascade test) |
| Phase 2a/2c lacked migration path for existing DBs | v2 adds deduplication SQL and explicit fresh-DB-only warnings |
| Phase 4a guard behavior was undefined | v2 recommends an unexported rename (`addColumnIfNotExistsSQLite`) with a clear comment |

---

## Nits (non-blocking)

### Nit 1 — Phase 4a: "panic guard" language is inconsistent with the actual recommended code

The section header and intro say "Add panic guard" and "fail loudly", but the recommended code block just shows an unexported rename with a comment:

```go
func addColumnIfNotExistsSQLite(ctx context.Context, db *sql.DB, ...) error {
    // SQLite-only: uses PRAGMA table_info(). Do not call on Postgres.
    query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
    ...
}
```

No runtime panic is actually implemented. Since `migrator.go:44` already gates the entire pre-migration block with `if s.profile.Driver == "sqlite"`, the callers are already protected. The rename is sufficient defense-in-depth.

**Recommendation:** Either
- Drop the word "panic" and call it "unexported rename + caller guard," or
- Actually add a runtime check inside the helper (e.g., inspect the driver via `db.Driver()` and return an explicit error if it's not SQLite).

### Nit 2 — Phase 5b: `bridge_endpoints_test.go:1146` does not need a skip guard

Line 1146 reads the SQLite `LATEST.sql` repo file via `os.ReadFile`:

```go
content, err := os.ReadFile("../../../../../store/migration/sqlite/LATEST.sql")
```

This is not SQLite-engine-specific — it reads a static repo file, so it will not fail on Postgres. It is a design-intent test (verifying the SQLite migration file doesn't contain unwanted table names), and it passes regardless of which driver the test suite runs against.

The truly SQLite-engine-specific lines in that file are:
- **Line 741** — `sqlite_master` query, will error on Postgres
- **Line 300 in `bridge_delivery_test.go`** — `PRAGMA foreign_keys = OFF`

**Recommendation:** Remove `bridge_endpoints_test.go:1146` from the skip-guard table. Keep `741` and the other `PRAGMA`/`sqlite_master`/`TRIGGER` entries.

---

## Confirmed Correctness of Key Claims

- **Phase 2d (13 tables):** Cross-checked each table against `FATEST.sql`. All 13 listed tables (`user_tenant_permission`, `agent_audiences`, `agent_services`, `agent_exclusions`, `agent_coverage`, `agent_faqs`, `agent_safety_protocols`, `agent_kb_sections`, `agent_rules`, `agent_sessions`, `agent_source_files`, `agent_rate_limits`, `agent_observations`) genuinely lack `FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE`. Tables already having it (`agent_intents`, `tenant_role_templates`, `tenant_config`, `agent_tenant_scripts`, `agent_transcripts`, `agent_simulation_transcripts`, `agent_analysis_results`, `agent_learning_memory`, `agent_compliance_audits`, `agent_scoring_config`, `agent_qa_pairs`, `agent_reindex_checkpoints`, `agent_leads`) are correctly excluded from the list.
- **Phase 3 scope:** `store/db/postgres/rbac.go` already contains `source_template_id` (lines 17, 52, 55, 59, 97, 119, 402) and `admin_mutation_rate_limit_rpm` (lines 140, 183). `store/db/postgres/memo_relation.go` already contains `tenant_id` (lines 18, 21, 22, 56, 84, 125). Only `vector_db_s3_override` is genuinely absent.
- **Phase 1g:** `bridge_handoffs.tenant_id` at Postgres line 756 is the only bridge table missing the direct FK to `agent_tenants`.
- **Phase 5d:** The new Postgres bridge cascade test correctly closes the coverage gap created by the SQLite skip guards.

---

## Final Verdict

**APPROVED WITH NITS.** Address the two nits above and the plan is ready to implement.
