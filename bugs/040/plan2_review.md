# Adversarial Review: Plan v2 — SQLite → PostgreSQL Parity Fixes

**Reviewer:** AI Agent
**Date:** 2026-07-17
**Status:** Approved with Nits

---

## Summary

All v1 rework items are properly addressed. The plan is ready for implementation with **one code bug** to correct and **one minor nit**.

---

## Bug: Fix 1 uses `err =` where `err :=` is required

The proposed code for `CreateBridgeAuthKey` (lines 59-66) contains:

```go
var id int64
err = d.db.QueryRowContext(ctx, `
    INSERT INTO bridge_auth_keys (...)
    VALUES (...)
    RETURNING id
`, ...).Scan(&id)
```

This **will not compile**. The original `result, err := d.db.ExecContext(...)` is removed, so `err` is never declared in the function scope. `err =` attempts to assign to an undeclared variable.

**Fix:** Use `err :=`:

```go
var id int64
err := d.db.QueryRowContext(ctx, `
    INSERT INTO bridge_auth_keys (...)
    VALUES (...)
    RETURNING id
`, ...).Scan(&id)
```

This matches the existing pattern used throughout the Postgres driver (e.g., `agent.go:2615` in `CreateAgentIntegration`).

---

## V2 Rework Items — Verified Correct

| V1 Issue | V2 Status | Check |
|----------|-----------|-------|
| Fix 1 missed `LastInsertId` in bridge_auth.go | ✅ Both bugs fixed (`.Unix()` + `RETURNING id`) | Confirmed |
| Fix 7 missing migration file | ✅ Added `04__*.sql` + LATEST.sql | Confirmed |
| Fix 5 behavior change undocumented | ✅ Noted as behavioral change | Confirmed |
| MySQL stubs silent nil | ✅ Noted in Fix 6 | Confirmed |

## Other Verifications

- **All 5 `LastInsertId` calls covered** — `bridge_auth.go:28`, `agent.go:1739,1941,1986,2513` — zero missed
- **Fix 6 insertion point** (line 262 in `driver.go`) — confirmed correct, between RAG active version and Observation Log sections
- **Migration file naming** (`04__add_tenant_id_check_to_role_templates.sql`) — follows existing 0.30/ convention (`00__`, `01__`, `02__`, `03__`)
- **Fix 5 error wrapping** — accurately preserves `fmt.Errorf("failed to upsert reindex checkpoint: %w", err)`
- **Schema analysis claim** (56 tables, fully ported) — not independently verified but unchanged from v1

## Nits

1. **Fix 6 note about MySQL stubs is slightly understated.** The plan says "consider returning an explicit 'not implemented' error in the future." Since the stubs already exist and will be activated by adding to the `Driver` interface, a recommendation to upgrade them from `return nil, nil` to `return nil, errors.New("not implemented")` **in this PR** would be cleaner — it converts a silent data loss bug into a loud crash.

---

## Final Verdict

**Approved with Nits.** Fix the `err =` → `err :=` syntax error in Fix 1 before implementation. Consider upgrading MySQL stubs to return errors in this same PR.
