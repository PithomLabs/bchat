# Plan Review: Consolidated Fixes for LATEST.sql + pgx Hardening

**Reviewer:** DeepSeek V4 Flash
**Target:** `bugs/031/plan2.md`
**Status:** All claims verified against source code. Plan is correct and complete.

---

## Verification Results

Every factual claim in `plan2.md` was checked against the actual codebase:

| Claim | Source | Verified |
|-------|--------|----------|
| Zero `lib/pq` imports | `store/db/postgres/postgres.go:9`, `go.mod` | ✅ |
| Zero `Prepare()` call sites in `store/db/postgres/` | `grep "\.Prepare("` — 0 results | ✅ |
| `delivery_status` CHECK tautology in PG LATEST.sql | `LATEST.sql:808` — `CHECK(delivery_status = 'not_delivered')` | ✅ |
| Same bug in SQLite LATEST.sql | `sqlite/LATEST.sql:868` — same pattern | ✅ |
| No UPDATE of `delivery_status` in Go code | `grep "SET.*delivery_status" store/db/` — 0 results | ✅ |
| `source_template_id` missing from versioned 0.26 migration | `0.26/00__agent_tenant_rbac_foundation.sql` — not present | ✅ |
| `idx_tenant_config_tenant` missing from versioned migrations | Only in LATEST.sql, not in any versioned migration | ✅ |
| `ALTER TABLE ADD COLUMN` only used by incremental migrations | 14 occurrences in `0.19/`–`0.29/`, none in LATEST.sql | ✅ |
| `execute()` is called for both LATEST.sql and incremental migrations | `migrator.go:157,216` | ✅ |
| `validate:migrations` already exists in `Taskfile_pg.yml:22` | Calls `scripts/validate-pg-migrations.sh` | ✅ |

---

## Findings

### No errors in plan2.md

All 8 steps are logically sound, appropriately scoped, and verified against the codebase. No false claims, missing dependencies, or incorrect assumptions.

### One optional improvement

**Step 4 — `02__user_tenant_permission_source_template.sql`** bundles three unrelated changes:
```sql
ALTER TABLE user_tenant_permission ADD COLUMN IF NOT EXISTS source_template_id ...
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_template ...
CREATE INDEX IF NOT EXISTS idx_tenant_config_tenant ...
```

The `idx_tenant_config_tenant` index on `tenant_config` is unrelated to `user_tenant_permission`. Consider splitting into `02__user_tenant_permission_source_template.sql` and `03__add_missing_indexes.sql`. **Not a blocker** — functionally correct either way.

---

## Execution Order Validation

All 8 steps are independent — no step depends on a later step, no merge conflicts:

1. **Step 1** (`migrator.go`) — Fix execute tolerance
2. **Step 2** (`LATEST.sql`) — ON CONFLICT on seed INSERT
3. **Step 3** (`LATEST.sql` + `sqlite/LATEST.sql`) — Fix delivery_status CHECK
4. **Step 4** (new `0.30/` migrations) — Create incremental migration files
5. **Step 5** (`validate-pg-migrations.sh`) — Update validation script
6. **Step 6** (`postgres.go`) — Force pgx simple protocol for Neon
7. **Step 7** (Taskfile) — Add `validate:no-libpq` CI guard
8. **Step 8** (AGENTS.md) — Document pgx as sole driver

---

## Edge Cases Checked

| Edge Case | Assessment |
|-----------|-----------|
| DSN already has `default_query_exec_mode`? | Step 6 handles via `strings.Contains` guard |
| DSN has no query params (`?`)? | Step 6 handles via `sep` variable (`?` vs `&`) |
| DB already has 0.30 applied? | All statements use `IF NOT EXISTS` and `ON CONFLICT` |
| Seed data changes in the future? | `ON CONFLICT DO NOTHING` means new roles need separate INSERTs |
| Transitive `lib/pq` dep appears? | No paths in go.sum — Step 7 CI guard catches it |

---

## Verdict

**PLAN READY FOR IMPLEMENTATION.** No corrections needed. Proceed with Steps 1–8 in any order.