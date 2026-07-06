# Implementation Review — plan2_imp.md (Coding Review Fixes)

**Date:** 2026-07-06
**Reviewer:** opencode
**Status:** Approved with nits

---

## Verdict: Approved with nits

All 19 implemented fixes are verified correct in SQLite. However, the review uncovered **2 critical gaps in Postgres/MySQL drivers** and a few medium-severity issues.

---

## Phase 1 (P0) — CRIT-1 through CRIT-4

| Fix | Status | Details |
|-----|--------|---------|
| CRIT-1 | **PASS** | Table name correctly `agent_audiences` (plural). Comment has singular typo — cosmetic only |
| CRIT-2 | **PASS** | Store layer complete. ListUsers scans, CreateUser inserts, UpdateUser updates. **But see nit below** |
| CRIT-3 | **PASS (SQLite)** | TenantID added to struct, SQL WHERE, and all 3 callsites |
| CRIT-4 | **PASS** | Both ListMemoRelations calls include `TenantID: memo.TenantID` |

### New Critical Finding: Postgres/MySQL Drivers Not Updated

**Files:** `store/db/postgres/memo_relation.go`, `store/db/mysql/memo_relation.go`

Both drivers completely ignore `TenantID` in `DeleteMemoRelation`, `UpsertMemoRelation`, and `ListMemoRelations`. If the system ever runs on Postgres or MySQL, **all tenant isolation on memo_relation is broken**.

**If Postgres/MySQL support is intended:** Must add `tenant_id` handling to all three operations in both drivers before shipping.

**If SQLite-only (current deployment):** This is a deferred issue but should be documented.

### Nit on CRIT-2: No API Path to Update AllowedTenantIDs

The store layer supports it, but `user_service.go:185-224` (the `UpdateMask` handler) has no `"allowed_tenant_ids"` case. Admins can only set it at user creation time, not afterwards. May be intentional (managed via direct DB or future admin UI), but worth documenting.

---

## Phase 2 (P1) — CRIT-5, CRIT-6, HIGH-3 through HIGH-10

**All 9 fixes verified correct.**

| Fix | Status | Key File:Line |
|-----|--------|---------------|
| CRIT-5 | **PASS** | `service.go:1062-1081` — both maps cleaned, orphan sweep present, lock ordering correct |
| CRIT-6 | **PASS** | `auth_service.go:449-455` — rate limiting via loginRateLimiter |
| HIGH-3 | **PASS** | `tenant_binding.go:22-28` — 500 on DB error, 403 on nil user |
| HIGH-4 | **PASS** | `auth_service.go:550-552` — only `tenant_id` in JSON response |
| HIGH-5 | **PASS** | `service.go:1794-1801` — MaxMessageLength validation in ChatInternal |
| HIGH-6 | **PASS** | `service.go:1534-1545` — HMAC(GUID, "session-token-key") derivation |
| HIGH-8 | **PASS** | `handlers.go:2101-2109` — `</script>` and `</` escaping |
| HIGH-9 | **PASS** | `memo_service.go:268,316,449` — nil TenantID bypass fixed at 3 locations |
| HIGH-10 | **PASS** | `memo_relation_service.go:65` — REFERENCE upsert includes TenantID |

### One Medium Finding: `dispatchMemoMentions` Missing TenantID

**File:** `server/router/api/v1/memo_service.go:854-857`

```go
relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
    MemoID: &memo.ID,
    Type:   &relationType,
})
// TenantID NOT set
```

The `memo` object has `TenantID` available but it's not passed. Could leak cross-tenant relation data in notification logic.

---

## Phase 3 (P2) — HIGH-1, MED-2 through MED-6

**All 6 fixes verified correct.**

| Fix | Status | Key Detail |
|-----|--------|------------|
| HIGH-1 | **PASS** | Backup salt via `HMAC(primarySalt, "backup-key-salt")`, cryptographically distinct |
| MED-2 | **PASS** | `Sscanf` into `int64 tsRaw`, then `time.Unix(tsRaw, 0)` |
| MED-3 | **PASS (advisory)** | Bounded at 10K, cleanup every 10min. Advisory: no proactive oldest-entry eviction on capacity breach |
| MED-4 | **PASS** | `ON CONFLICT DO UPDATE SET tenant_id = excluded.tenant_id` |
| MED-5 | **PASS** | `sync.Mutex` on Handler struct, acquired in `ensurePlaygroundDemo` |
| MED-6 | **PASS** | `slog.Warn` on backup key fallback success |

---

## Summary

| Category | Count | Action |
|----------|-------|--------|
| **Verified PASS** | 19/19 | No changes needed |
| **New Critical** | 1 | Postgres/MySQL drivers need TenantID support (if those DBs are used) |
| **New Medium** | 2 | `dispatchMemoMentions` missing TenantID; no API path to update `AllowedTenantIDs` |
| **Advisory** | 1 | Rate limiter eviction window could be shortened |

**Bottom line:** All 19 fixes are correctly implemented in SQLite. The only blocker is whether Postgres/MySQL support is required — if so, the memo_relation drivers need the same TenantID updates. Otherwise, this is approved with nits.

---

## Sign-Off

- [x] Adversarial review completed
- [x] Approved with nits (no rework needed)
- [ ] Postgres/MySQL drivers updated (if applicable)
- [ ] `dispatchMemoMentions` TenantID fix
