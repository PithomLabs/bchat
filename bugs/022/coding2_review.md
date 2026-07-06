# Adversarial Code Review — plan2 Implementation

**Reviewer:** opencode  
**Date:** 2026-07-06  
**Input:** `plan2_review.md`, `plan2_imp_review.md`, `coding2.md`, current working tree  
**Status:** Approved with nits

---

## Review Document Assessment

### plan2_review.md — Approved with nits
- Phasing and prioritization are correct (P0 runtime fixes before security hardening).
- Q1–Q6 answers are consistent with the codebase.
- Nits are valid: all-callsite TenantID coverage, legacy-memo documentation, bounded rate-limiter map, orphaned-lock cleanup, quick HMAC-key mitigation.
- Plan accurately reflects the intended fixes.

### plan2_imp_review.md — Approved with nits (with factual corrections)
The document contains two inaccurate claims and one outdated finding:

1. *Claim:* Both drivers completely ignore TenantID in `DeleteMemoRelation`, `UpsertMemoRelation`, and `ListMemoRelations`.  
   *Reality:* Postgres/MySQL `DeleteMemoRelation` and `UpsertMemoRelation` already correctly include `tenant_id`. Only `ListMemoRelations` omits `tenant_id` from the SELECT/Scan. The finding is real but the scope is overstated.

2. *Claim:* `dispatchMemoMentions` Missing TenantID.  
   *Reality:* The current code passes `TenantID: memo.TenantID` at line 857 of `memo_service.go`. This nit was already fixed before the review was written.

---

## Implementation Verification

### Phase 1 — P0 (CRIT-1 to CRIT-4): PASS

| Fix | Status | Evidence |
|-----|--------|----------|
| CRIT-1 | PASS | `store/migration/sqlite/0.28/01__add_max_message_length.sql` uses `agent_audiences` (plural). Comment has singular typo — cosmetic only. |
| CRIT-2 | PASS | `AllowedTenantIDs` wired through `CreateUser`, `UpdateUser`, `ListUsers` in `store/db/sqlite/user.go`. `UpdateUser` struct has `*string` field. |
| CRIT-3 | PASS | `DeleteMemoRelation` has `TenantID *int32`, WHERE clause is conditional, all 3 callsites pass it (`memo_service.go:474`, `memo_service.go:493`, `memo_relation_service.go:31`). |
| CRIT-4 | PASS | Both `ListMemoRelations` calls in `memo_relation_service.go` include `TenantID: memo.TenantID` (lines 104, 119). |

### Phase 2 — P1 (CRIT-5 to HIGH-10): PASS

| Fix | Status | Evidence |
|-----|--------|----------|
| CRIT-5 | PASS | `agent/service.go:1062-1081` — `cleanup()` acquires `mu` then `locksMu`, deletes from both maps, sweeps orphaned locks. |
| CRIT-6 | PASS | `auth_service.go:449-455` — `loginRateLimiter.Allow(clientIP)` applied in `HandleSelectTenant`. |
| HIGH-3 | PASS | `tenant_binding.go:22-28` — returns 500 on DB error, 403 on nil user. |
| HIGH-4 | PASS | `auth_service.go:550-552` — JSON response contains only `tenant_id`. `cookie` variable is unused and discarded. |
| HIGH-5 | PASS | `agent/service.go:1794-1801` — `MaxMessageLength` validation in `ChatInternal`. |
| HIGH-6 | PASS | `agent/service.go:1534-1545` — `deriveSessionTokenKey` uses `HMAC(GUID, "session-token-key")`. |
| HIGH-8 | PASS | `agent/handlers.go:2101-2109` — `escapeJS` escapes `</script>` and `</`. |
| HIGH-9 | PASS | `memo_service.go:268,316,449` — nil TenantID bypass fixed at GetMemo, UpdateMemo, DeleteMemo. Legacy memos (`TenantID == nil`) remain globally accessible, behavior documented. |
| HIGH-10 | PASS | `memo_relation_service.go:65` — REFERENCE upsert includes `TenantID: memo.TenantID`. |

### Phase 3 — P2 (HIGH-1, MED-2 to MED-6): PASS

| Fix | Status | Evidence |
|-----|--------|----------|
| HIGH-1 | PASS | `internal/crypto/encryption.go:44-46` — backup salt derived via `HMAC(primarySalt, "backup-key-salt")`. |
| MED-2 | PASS | `auth_service.go:486-488` — `Sscanf` into `int64 tsRaw`, then `time.Unix(tsRaw, 0)`. |
| MED-3 | PASS | `login_ratelimit.go:8,25-34` — bounded at 10K, cleanup every 10 min. Advisory: no proactive oldest-entry eviction on capacity breach. |
| MED-4 | PASS | SQLite/Postgres `UpsertMemoRelation` uses `ON CONFLICT DO UPDATE SET tenant_id = excluded.tenant_id`. |
| MED-5 | PASS | `server/router/api/v1/agent/handlers.go:32` — `playgroundMu sync.Mutex` on Handler struct, acquired in `ensurePlaygroundDemo`. |
| MED-6 | PASS | `internal/crypto/encryption.go:101` — `slog.Warn` on backup-key fallback success. |

---

## New Critical Finding Overlooked by the Review

**Postgres/MySQL `ListMemoRelations` does not SELECT/SCAN `tenant_id`**

Files:
- `store/db/postgres/memo_relation.go:79-85, 94-98`
- `store/db/mysql/memo_relation.go:70, 79-84`

Both drivers correctly build the WHERE clause with `tenant_id = ?`, but the SELECT statement only retrieves `memo_id, related_memo_id, type`, and `Scan` unmarshals only 3 fields. Consequently `memoRelation.TenantID` is always `nil` on non-SQLite databases.

Impact: Downstream code that reads `TenantID` from a `MemoRelation` receives `nil` on Postgres/MySQL. The `plan2_imp_review.md` labeled this issue as a critical gap; the actual scope is limited to `ListMemoRelations`, but the bug is real and should be fixed.

---

## Other Issues Found During Adversarial Review

### Medium — `AllowedTenantIDs` not exposed via API UpdateMask
`server/router/api/v1/user_service.go:185-224` handles update fields like `role`, `email`, `nickname`, but `allowed_tenant_ids` falls through to the `else` branch and returns `invalid update path`. Administrators can only set this at user creation, not update it. Not a regression; a missing feature.

### Low — SQLite `CreateUser` does not RETURN `allowed_tenant_ids`
`store/db/sqlite/user.go:25` — RETURNING clause omits `allowed_tenant_ids`. Functionally correct because the caller’s in-memory struct retains the marshaled value, but inconsistent with Postgres/MySQL behavior if those drivers are later checked.

### Advisory — `HandleSelectTenant` O(N) token lookup
`auth_service.go:470-496` iterates all users and their access tokens to find the selection token. This is O(users × tokens) and leaks a timing side-channel. Pre-existing, not introduced by plan2.

### Advisory — Rate-limiter eviction window
`login_ratelimit.go:64` — cleanup runs every 10 minutes. If traffic bursts fill all 10K slots quickly, legitimate IPs may be rejected until cleanup runs. Not a security issue, but a tuning consideration.

---

## Verdict

**APPROVED WITH NITS**

All 19 plan2 fixes are correctly implemented on SQLite/Postgres/MySQL with the exception of the missing `tenant_id` projection in `ListMemoRelations` for Postgres/MySQL.

### Required before ship
| # | Action | File |
|---|--------|------|
| 1 | Add `mr.tenant_id` to SELECT and Scan in postgres/mysql `ListMemoRelations` | `store/db/postgres/memo_relation.go`, `store/db/mysql/memo_relation.go` |

### Tracked but not blockers
| # | Issue | Notes |
|---|-------|-------|
| 2 | Expose `allowed_tenant_ids` in `user_service.go` UpdateMask | Add case in the `for _, field := range request.UpdateMask.Paths` loop |
| 3 | `CreateUser` RETURNING includes `allowed_tenant_ids` for consistency | Paranoia fix; current behavior is functionally correct |
| 4 | Optimize token lookup in `HandleSelectTenant` to O(1) | Pre-existing; requires adding an index or lookup table on `access_token` |
