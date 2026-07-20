# Implementation Signoff: Database Migration Parity Audit

**Reviewer:** Senior Database Architect
**Date:** 2026-07-20
**Plan:** `plan.md`
**Plan Review:** `plan_review.md`

---

## Signoff Decision

**NOT READY FOR SIGN-OFF.** One blocking issue remains unresolved from the plan review. Several adversarial edge cases are identified below that must be addressed during implementation.

---

## Verdict by Issue

### Issue 1: Postgres SystemSecret Store — CONDITIONALLY APPROVED

The would-be implementation (replacing stubs at `store/db/postgres/rbac.go:213-219` with live SQL) follows the established patterns in the codebase:
- `[]byte` ← `BYTEA` scanning matches `tenant_config.openrouter_api_key_encrypted` pattern at `store/db/postgres/rbac.go:145` ✅
- `int` ← `INTEGER` scanning matches `MaxMessageLength` pattern at `store/db/postgres/agent.go:177` ✅
- `int64` ← `BIGINT` for timestamps matches the codebase convention ✅
- Postgres `$N` parameter style and `EXCLUDED` syntax used correctly ✅

#### Adversarial edge cases identified:

| # | Edge Case | Behavior | Risk |
|---|-----------|----------|------|
| 1 | `EncryptionSalt` is `nil` | Postgres rejects with `NOT NULL` violation. Callers always generate a salt first (`service.go:116`). | None — programming error surfaces clearly |
| 2 | Concurrent `UpsertSystemSecret` | `ON CONFLICT` ensures atomicity. Both succeed; `rotated_at` reflects latest write. | None |
| 3 | Context cancelled mid-query | `QueryRowContext` returns immediately with context error. | None — standard pattern |
| 4 | `KeyVersion = 0` passed | Written as-is to DB (no `CHECK` on `key_version`). Callers always pass `1`. | Low — schema should have `CHECK (key_version > 0)` but outside scope |
| 5 | Upsert mutates caller's pointer | Sets `secret.ID = 1` and `secret.CreatedAt = now`. Current callers at `service.go:121` ignore the return, but future callers may rely on `CreatedAt` not changing on conflict. | Low — pre-existing bug inherited from SQLite |
| 6 | Multiple rows in `system_secret` | `WHERE id = 1` + `CHECK (id = 1)` in schema ensures singleton. `Get` returns one row. | None |

### Issue 2: Migration `0.33/00__add_system_secret.sql` — APPROVED

The would-be implementation creates `store/migration/postgres/0.33/00__add_system_secret.sql`.

| Concern | Assessment |
|---------|-----------|
| File discovery | `fs.Glob("migration/postgres/*/*.sql")` picks up new `0.33/` dir automatically ✅ |
| Ordering | `0.33/00__` sorts after `0.32/01__` lexicographically ✅ |
| Schema version | `0.33.1` — no conflict with existing history ✅ |
| Idempotency | `IF NOT EXISTS` is safe for fresh installs (LATEST.sql already creates table) ✅ |
| Re-runs | `IF NOT EXISTS` is idempotent ✅ |

#### Adversarial edge cases:

| # | Edge Case | Behavior | Risk |
|---|-----------|----------|------|
| 1 | Run on Postgres 9.x or older | `EXTRACT(EPOCH FROM NOW())` is supported since Postgres 8.4. Fine. | None |
| 2 | Migration runs but `UpsertSystemSecret` called before `GetSystemSecret` | `id = 1` inserted, `rotated_at` NULL on first insert. Fine. | None |
| 3 | Missing `::BIGINT` cast on `created_at` default | `EXTRACT(EPOCH FROM NOW())` returns `double precision`; Postgres truncates on insert into `BIGINT`. Produces same epoch-second integer. Pre-existing in LATEST.sql line 750. | None — matches existing code |

### Issue 3: `max_message_length` SQLite Fix — NOT READY

**The plan review's rework request has not been incorporated yet.** The corrective UPDATE must handle NULL values:

```sql
UPDATE agent_audiences
   SET max_message_length = 2000
 WHERE max_message_length IS NULL
    OR max_message_length = 4000;
```

#### Additional adversarial findings beyond the plan review:

| # | Edge Case | Issue | Severity |
|---|-----------|-------|----------|
| 1 | **False positive on explicit 4000** | A tenant admin who explicitly set `max_message_length = 4000` via the UI would have it silently reset to 2000. The UPDATE cannot distinguish default-origin from user-origin values. | **MEDIUM** — mitigate by logging affected row count and document in release notes |
| 2 | **Rollback cannot restore NULLs** | The plan's rollback `UPDATE ... SET max_message_length = 4000` would set ALL rows to 4000, including rows that were previously NULL. | Low — the plan should document a more precise rollback or accept this asymmetry |
| 3 | **Column default still 4000 after fix** | Any `INSERT` that omits `max_message_length` will get 4000 from the schema default (SQLite cannot `ALTER COLUMN ... SET DEFAULT`). The Go layer always sets it explicitly in `CreateAgentAudience` (`store/db/sqlite/agent.go:164`), so this is mitigated at the application layer. | Low — document in the migration comment |
| 4 | **Migration on empty table** | `UPDATE` on zero rows is a no-op. Safe. | None |

### Issue 4: CHECK Constraint — APPROVED

Documentation-only is correct. No implementation concerns.

---

## Cross-Cutting Adversarial Concerns

### 1. Postgres test infrastructure assumptions

The plan says "Run existing `go test ./store/db/postgres/ -run TestSystemSecret`" — but no `TestSystemSecret` test exists yet, and Postgres tests in this project require a live Postgres database (not a mock). The test must:
- Start a transaction, create the table (or use a test schema), upsert a secret, read it back, rollback
- Or require `POSTGRES_TEST_DSN` env var like other Postgres tests

**Recommendation:** Verify the existing test pattern before writing tests. Check if there's a test helper for Postgres tests.

### 2. MySQL remains stubbed

`store/db/mysql/rbac.go:54-60` still returns `nil, nil` for both `GetSystemSecret` and `UpsertSystemSecret`. If MySQL is a supported driver, this should be tracked.

### 3. Migration version gap

The SQLite `max_message_length` corrective migration is named `0.33/00__fix_max_message_length_default.sql` and the Postgres `system_secret` migration is `0.33/00__add_system_secret.sql`. Both are `0.33/00__` in their respective driver directories — no collision. ✅

### 4. Upgrade path from 0.32 → 0.33

On upgrade, the migration framework applies ALL unapplied files in order. If both `0.32/01` and `0.33/00` need to be applied, they run sequentially in one transaction. The `system_secret` table creation has no dependency on `transcript_signing_key` column addition, so ordering is safe. ✅

---

## Summary: What Must Be Done Before Sign-Off

| Priority | Item | Issue |
|----------|------|-------|
| **BLOCKING** | Fix Issue 3 UPDATE to handle `IS NULL` | Rework from plan review |
| Recommended | Log count of `max_message_length = 4000` rows updated during corrective migration | Adversarial finding #1 |
| Recommended | Add `CHECK (key_version > 0)` to `system_secret` schema (or defer) | Adversarial finding #4 (Issue 1) |
| Recommended | Track MySQL stub fix as follow-up | Adversarial finding (cross-cutting) |
| Recommended | Verify Postgres test infrastructure before writing tests | Cross-cutting concern |

---

*Once the Issue 3 rework is incorporated, this plan is ready for implementation sign-off.*
