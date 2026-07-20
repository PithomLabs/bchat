# Signoff Review: Implementation Adversarial Review

**Reviewer:** Senior Database Architect
**Date:** 2026-07-20
**Plan:** `plan.md`
**Plan Review:** `plan_review.md`
**Signoff:** `signoff.md`
**Reviewing commit:** Implementation present in HEAD (33d6a46)

---

## Verdict

**APPROVED — CONDITIONALLY.** The blocking issue from the signoff is resolved. Three verification gaps remain (none are correctness bugs).

---

## Issue 1: Postgres SystemSecret Store — PASS

| Check | Result |
|-------|--------|
| Stubs replaced with live SQL at `store/db/postgres/rbac.go:213-259` | ✅ Matches the planned implementation exactly |
| `[]byte` ← `BYTEA` scan matches established pattern | ✅ Same as `tenant_config.openrouter_api_key_encrypted` |
| `int` ← `INTEGER` scan for `KeyVersion` | ✅ Same as `MaxMessageLength` pattern |
| `int64` ← `BIGINT` for timestamps | ✅ `now.Unix()` ↔ `time.Unix()` round-trip correct |
| `$N` param style + `EXCLUDED` syntax | ✅ Correct Postgres syntax |
| Edge case: `EncryptionSalt` nil → `NOT NULL` violation | ✅ Callers always generate salt first |
| Edge case: concurrent upserts | ✅ `ON CONFLICT` is atomic |
| Pre-existing bug: `CreatedAt` overwrite on conflict | ⚠️ Unchanged from SQLite; deferred |

---

## Issue 2: Migration `0.33/00__add_system_secret.sql` — PASS

| Check | Result |
|-------|--------|
| File exists at `store/migration/postgres/0.33/00__add_system_secret.sql` | ✅ |
| `IF NOT EXISTS` idempotency | ✅ Safe for fresh installs and re-runs |
| Column types match LATEST.sql | ✅ `id SERIAL PRIMARY KEY CHECK (id = 1)`, `BYTEA`, `INTEGER`, `BIGINT` |
| `::BIGINT` cast added | ✅ **Improvement over plan** — the implementer added `::BIGINT` to `created_at` default (line 7), addressing the nit from plan review |
| LATEST.sql still lacks `::BIGINT` | ⚠️ Line 750: `DEFAULT EXTRACT(EPOCH FROM NOW())` without cast. Both produce identical `BIGINT` at runtime; minor schema-level inconsistency |

---

## Issue 3: `max_message_length` SQLite Fix — PASS (BLOCKER RESOLVED)

| Check | Result |
|-------|--------|
| File exists at `store/migration/sqlite/0.33/00__fix_max_message_length_default.sql` | ✅ |
| **NULL handling** (blocking signoff item) | ✅ `WHERE max_message_length IS NULL OR max_message_length = 4000` |
| False-positive-on-4000 caveat documented | ✅ "This UPDATE cannot distinguish user-set 4000 from default-origin 4000" |
| Comment explains Go-layer default mitigation | ✅ "The Go layer (CreateAgentAudience) always sets this value explicitly" |
| Original migration file unmodified (Option A) | ✅ No changes to `0.28/01__add_max_message_length.sql` |

---

## Issue 4: CHECK Constraint — PASS

Documentation-only. No action needed. ✅

---

## Verification Gaps

These were identified in the signoff's cross-cutting concerns and plan's testing section but were not implemented:

| Gap | Severity | Detail |
|-----|----------|--------|
| No `TestSystemSecret` unit test | **MEDIUM** | The plan's verification step referenced `go test ./store/db/postgres/ -run TestSystemSecret` but no such test exists. No test file was created or modified. |
| MySQL stubs remain | LOW | `store/db/mysql/rbac.go:54-60` still returns `nil, nil`. Track separately. |
| `CHECK (key_version > 0)` not added | LOW | Recommended in signoff, explicitly marked as optional. Schema allows `key_version = 0`. |

---

## LATEST.sql Consistency Check

| Driver | Table | LATEST.sql | Migration 0.33 | Match? |
|--------|-------|------------|----------------|--------|
| Postgres | `system_secret` | `created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())` | `created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT` | ⚠️ `::BIGINT` in migration only |
| SQLite | `agent_audiences` | `max_message_length INTEGER NOT NULL DEFAULT 2000` | N/A (corrective UPDATE, not schema change) | ✅ No update needed |

The `::BIGINT` inconsistency is cosmetic — both produce identical `BIGINT` values at runtime.

---

## Summary

| Area | Verdict |
|------|---------|
| Issue 1: Postgres store | ✅ Correct implementation |
| Issue 2: Postgres migration | ✅ Correct, improved with `::BIGINT` |
| Issue 3: SQLite corrective migration | ✅ **Blocker resolved**, NULL handled with thorough docs |
| Issue 4: CHECK constraint | ✅ Deferred per plan |
| Tests | ❌ Missing — verify manually before production deploy |
| MySQL | ❌ Still stubbed — non-blocking |

**The implementation is correct.** The three gaps are verification/coverage concerns, not correctness bugs. Deployable after manual testing on a real Postgres database to confirm the SystemSecret round-trip.
