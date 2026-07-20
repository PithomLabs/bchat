# Plan Review: Database Migration Parity Audit

**Reviewer:** Senior Database Architect
**Date:** 2026-07-20
**Status:** APPROVED WITH NITS
**Plan:** `plan.md`

---

## Verdict

**APPROVED WITH NITS.** The plan is sound, the analysis is thorough, and the proposed implementations correctly follow existing patterns. One item requires rework; the rest are style/robustness concerns.

---

## Issue 1: Postgres SystemSecret Store (CRITICAL)

### Approved — No Rework

The SQL implementation correctly follows the SQLite pattern at `store/db/sqlite/rbac.go:520-569`, using Postgres `$N` parameter style, `EXCLUDED` for `ON CONFLICT`, and `RETURNING id`. No injection vectors. `BYTEA`, `INTEGER`, and `BIGINT` all scan correctly into Go types via pgx simple-protocol.

### Nits (3)

1. **`CreatedAt` overwrite on upsert conflict.** Both the proposed Postgres implementation and the existing SQLite implementation (`store/db/sqlite/rbac.go:567`) overwrite `secret.CreatedAt = now` on every call, even when the row already exists. This is a pre-existing correctness bug — it returns a mutated `CreatedAt` on the returned pointer for conflict case. Current callers at `service.go:121` ignore the return value, but it should be tracked as tech debt.

2. **`created_at` default missing `::BIGINT` cast.** The LATEST.sql for `system_secret` uses `DEFAULT EXTRACT(EPOCH FROM NOW())` without the `::BIGINT` cast that newer migrations (0.31+) use. The plan matches LATEST.sql faithfully, but consider adding the cast for type safety.

3. **MySQL stubs not addressed.** `store/db/mysql/rbac.go:54-60` also has `GetSystemSecret`/`UpsertSystemSecret` as `return nil, nil`. Out of scope for this audit but worth tracking.

---

## Issue 2: Missing Incremental Migration (CRITICAL)

### Approved — No Rework

The migration file, `IF NOT EXISTS` reasoning, and version numbering (`0.33/00__` → schema version `0.33.1`) are correct. The column types match LATEST.sql exactly.

### Nit (1)

The migration file naming `00__add_system_secret.sql` is correct per the framework (`00__` prefix yields version suffix `.1` after `strconv.Atoi("00") + 1`).

---

## Issue 3: `max_message_length` Default (MEDIUM)

### Rework Required (1)

**The corrective UPDATE does not handle NULL values.** The original SQLite migration (`0.28/01`) lacked `NOT NULL`, so a row with `max_message_length IS NULL` is possible. The proposed `WHERE max_message_length = 4000` misses these rows.

**Fix:** Change the UPDATE to:

```sql
UPDATE agent_audiences
   SET max_message_length = 2000
 WHERE max_message_length IS NULL
    OR max_message_length = 4000;
```

### Nits (0)

Correct otherwise.

---

## Issue 4: CHECK Constraint (LOW)

### Approved — No Rework

Documentation-only is the correct approach. SQLite cannot `ALTER TABLE ADD CONSTRAINT`. Go application layer already enforces the invariant. Correctly scoped.

---

## Cross-Cutting Concerns

1. **`KeyVersion` type:** Go struct uses `int`, Postgres column is `INTEGER`, pgx simple-protocol handles coercion. ✅
2. **`EncryptionSalt` type:** Go struct uses `[]byte`, Postgres column is `BYTEA`. pgx simple-protocol returns hex-encoded format that scans correctly. ✅
3. **Import path:** `"github.com/usememos/memos/store"` matches existing Postgres driver. ✅
4. **No new dependencies:** Only `database/sql` and `time` needed. ✅
5. **Migration file collision avoided:** Postgres `0.33/00__add_system_secret.sql` and SQLite `0.33/00__fix_max_message_length_default.sql` are in separate driver directories. ✅

---

## Summary

| Item | Severity | Action |
|------|----------|--------|
| Issue 3: Handle `max_message_length IS NULL` in corrective UPDATE | MEDIUM | **Rework before implementation** |
| Issue 1: Fix `CreatedAt` overwrite on upsert conflict (SQLite + Postgres) | LOW | File tech-debt ticket or fix inline |
| Issue 1: MySQL stubs tracking | LOW | Track as follow-up |
| Issue 1: `::BIGINT` cast for `created_at` default | LOW | Style preference, optional |

The core technical approach is correct. With the Issue 3 rework, this plan is ready for implementation.
