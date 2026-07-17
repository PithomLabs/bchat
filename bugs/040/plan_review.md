# Adversarial Review: SQLite → PostgreSQL Parity Fixes

**Reviewer:** AI Agent
**Date:** 2026-07-17
**Status:** Approved with Nits

---

## Summary

The plan correctly identifies 7 bugs and proposes sound fixes. However, **2 issues need rework** before implementation, and **3 nits** should be addressed.

---

## Must Rework

### 1. Fix 1 is incomplete — Missing `LastInsertId` → `RETURNING id` in bridge_auth.go

`store/db/postgres/bridge_auth.go:28` also calls `result.LastInsertId()` — the same pgx-via-`database/sql` incompatibility the plan correctly flags in Fixes 2-5. Since the file is already being modified for the `.Unix()` fix, both issues should be fixed together.

**Current (line 19-33):**
```go
result, err := d.db.ExecContext(ctx, `...`)
// ...
id, err := result.LastInsertId()  // ← also broken, same root cause
```

**Needed:** Convert to `RETURNING id` + `QueryRowContext` + `.Scan(&key.ID)`, identical pattern to Fixes 2-5.

### 2. Fix 7 needs a migration file, not just LATEST.sql edit

Modifying `store/migration/postgres/LATEST.sql` only affects fresh installs. Existing PostgreSQL databases (e.g., Neon on Fly.io, which is the stated deployment target) will **not** receive the CHECK constraint. A dedicated migration file (e.g., `store/migration/postgres/0.30/04__add_tenant_id_check_to_role_templates.sql`) is required:

```sql
ALTER TABLE tenant_role_templates
  ADD CONSTRAINT chk_tenant_role_templates_tenant_id
  CHECK (tenant_id IS NULL OR tenant_id >= 1);
```

The plan's "Files to Modify" table should list both files.

---

## Nits

### 3. Fix 5 behavior change is undocumented

The proposed change removes the `if checkpoint.ID == 0` guard:

```go
// Current — only fetches ID for new inserts
if checkpoint.ID == 0 {
    id, err := result.LastInsertId()
    if err == nil {
        checkpoint.ID = int32(id)
    }
}

// Proposed — always overwrites ID on every upsert
err := d.db.QueryRowContext(ctx, stmt, ...).Scan(&checkpoint.ID)
```

This is actually **better** (guarantees ID is always populated), but the behavioral change should be called out in the plan.

### 4. MySQL stubs compile but silently drop errors

`store/db/mysql/agent_workflow.go` returns `nil, nil` for all three workflow methods. Adding them to the `Driver` interface won't break compilation (they already exist), but if anyone ever switches to MySQL, workflow calls will silently return nil. Consider returning an explicit "not implemented" error.

### 5. Fix 5: error handling in the RETURNING path

The plan's proposed code:
```go
err := d.db.QueryRowContext(ctx, stmt, ...).Scan(&checkpoint.ID)
if err != nil {
    return nil, fmt.Errorf("failed to upsert reindex checkpoint: %w", err)
}
```

This is correct, but note that the original code returned `fmt.Errorf("failed to upsert reindex checkpoint: %w", err)` only on `err != nil` from `ExecContext`. The new code wraps the error from `Scan` instead — should verify this error wrapping is still accurate (it is, since `Scan` error from `RETURNING` effectively means the upsert failed).

---

## Verified Correct

| # | Claim | Result |
|---|-------|--------|
| 1 | bridge_auth.go passes `time.Time` to `BIGINT` | **Confirmed** — runtime crash waiting to happen |
| 2 | 3x `LastInsertId` in agent.go (lines 1739, 1941, 1986) | **Confirmed** — all silently ignore or return errors |
| 3 | UpsertReindexCheckpoint LastInsertId (line 2513) | **Confirmed** |
| 4 | Workflow methods missing from Driver interface | **Confirmed** — `driver.go` ends at line 290, no workflow entry |
| 5 | Store wrapper methods commented out | **Confirmed** — `store/agent_workflow.go:53-65` returns `nil, nil` |
| 6 | PG LATEST.sql missing CHECK on tenant_role_templates | **Confirmed** — SQLite has it (line 411), PG does not |
| 7 | Postgres driver already implements workflow methods | **Confirmed** — `agent_workflow.go:11-121` uses `RETURNING id` correctly |
| 8 | 40+ existing uses of `RETURNING id` in postgres driver | **Confirmed** — pattern is well-established |

---

## Final Verdict

**Approved with Nits.** Fix #1 (incomplete) and #7 (missing migration) must be reworked. The remaining items are minor documentation/behavioral notes.
