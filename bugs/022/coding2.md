# Bchat/022 bug fix plan discussion

**Session ID:** ses_0c9d6c890ffeYrO3jmOdfKf2sN
**Created:** 7/6/2026, 2:41:38 PM
**Updated:** 7/6/2026, 4:18:07 PM

---

## Assistant

The `TestBChatLiveTranscriptEndpointDoesNotLogRawSessionID` test expects the handler to reach the tenant lookup before failing, but now it returns early because of missing token/expiry. I need to update this test to provide a valid token, then the failure will come from the dropped `agent_transcripts` table.

**Tool: edit**

---

## Assistant

Now check the other failing test:

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

The test checks `ChatResponse` has exactly 5 fields but we added `SessionToken` and `SessionTokenExpiry` (now 7). Let me fix it:

**Tool: edit**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: bash**

---

## Assistant

All tests pass. Let me update the progress:

**Tool: todowrite**

---

## Assistant

All 14 security fixes are now implemented and all tests pass. Here's what was completed in this session:

## Completed
1. **ENCRYPTION_MASTER_KEY safeguards (#10)**: Added startup validation for key length + backup key support in `internal/crypto/encryption.go` with `ENCRYPTION_MASTER_KEY_BACKUP` env var and `decryptWithKey` helper
2. **Admin tenant binding middleware (#12)**: Created `server/router/api/v1/tenant_binding.go` with closure pattern (nit #3), added `AllowedTenantIDs` to User struct, migration `02__add_allowed_tenant_ids_to_user.sql`, wired into admin group
3. **Playground demo provisioning (#7)**: Added `StartupSeedPlaygroundDemos()` method, wired into `server.go` at startup, removed per-request auto-provisioning from catalog/run endpoints

## Test fixes
- Updated `ChatResponse` field count test (5→7)
- Fixed transcript test to use HMAC tokens via `testTranscriptURL` helper
- Updated LATEST.sql with `tenant_id`, `max_message_length`, `allowed_tenant_ids` columns
- Removed duplicate `publicCORS` declaration

All 14 issues from the DeepWiki security analysis are now implemented with the 6 nits incorporated.

---

## User

read coding_review.md and plan how you will implement its findings, let us make this interactive question and answer until we agree on a plan

---

## Assistant

**Tool: read**

---

## Assistant

This is a thorough adversarial review. Let me organize the findings and propose an implementation plan.

## Summary of Findings

**6 Critical (must fix):** Runtime failures or security regressions
**10 High (should fix):** Security gaps or memory issues
**6 Medium (nice to fix):** Robustness improvements
**4 Low (future):** Design improvements

---

## Proposed Implementation Plan

### Phase 1: P0 — Critical Runtime Fixes (4 issues)

| # | Issue | Fix | Files |
|---|-------|-----|-------|
| CRIT-1 | Migration table name `agent_audience` → `agent_audiences` | Change one word | `0.28/01__add_max_message_length.sql` |
| CRIT-2 | `allowed_tenant_ids` never read/written | Add to CreateUser, UpdateUser, ListUsers SQL + add field to UpdateUser struct | `store/db/sqlite/user.go`, `store/user.go` |
| CRIT-3 | `DeleteMemoRelation` has no TenantID | Add `TenantID *int32` to struct, add `AND tenant_id = ?` to SQL | `store/memo_relation.go`, `store/db/sqlite/memo_relation.go` |
| CRIT-4 | `ListMemoRelations` handler omits tenant filter | Add `TenantID: memo.TenantID` to both FindMemoRelation calls | `server/router/api/v1/memo_relation_service.go` |

### Phase 2: P1 — Security & Memory Fixes (8 issues)

| # | Issue | Fix | Files |
|---|-------|-----|-------|
| CRIT-5 | Session lock memory leak | Delete from `sessionLocks` in `cleanup()` | `server/router/api/v1/agent/service.go` |
| CRIT-6 | `HandleSelectTenant` no rate limiting | Add `loginRateLimiter.Allow(clientIP)` | `server/router/api/v1/auth_service.go` |
| HIGH-3 | TenantBindingMiddleware fails open | Return HTTP 500 on DB error, 403 on nil user | `server/router/api/v1/tenant_binding.go` |
| HIGH-4 | Access token leaked in JSON body | Remove `access_token`/`cookie` from response, keep only `tenant_id` | `server/router/api/v1/auth_service.go` |
| HIGH-5 | `ChatInternal` missing message length | Add same `MaxMessageLength` validation | `server/router/api/v1/agent/service.go` |
| HIGH-8 | XSS via incomplete JS escaping | Add `</script>` and `</` escaping to `escapeJS` | `server/router/api/v1/agent/handlers.go` |
| HIGH-9 | Nil TenantID bypasses ownership | Fix conditional: deny if memo has tenant but context doesn't | `server/router/api/v1/memo_service.go` |
| HIGH-10 | REFERENCE relations missing tenant_id | Add `TenantID: memo.TenantID` to REFERENCE upsert | `server/router/api/v1/memo_relation_service.go` |

### Phase 3: P2 — Hardening (6 issues)

| # | Issue | Fix | Files |
|---|-------|-----|-------|
| HIGH-1 | Backup key reuses same salt | Derive backup salt via `HMAC(primarySalt, "backup")` | `internal/crypto/encryption.go` |
| MED-2 | `Sscanf` into `time.Time` corruption | Use `int64` temp variable | `server/router/api/v1/auth_service.go` |
| MED-3 | Login rate limiter has no eviction | Add cleanup goroutine with TTL | `server/router/api/v1/login_ratelimit.go` |
| MED-4 | `UpsertMemoRelation` INSERT-only | Add `ON CONFLICT` clause | `store/db/sqlite/memo_relation.go` |
| MED-5 | Playground race condition | Add mutex around `ensurePlaygroundDemo` | `server/router/api/v1/agent/playground.go` |
| MED-6 | Backup key fallback silent | Add `slog.Warn` when backup key succeeds | `internal/crypto/encryption.go` |

### Deferred (requires design discussion)

| # | Issue | Why Deferred |
|---|-------|-------------|
| HIGH-6 | HMAC key is public GUID | Needs new per-tenant signing key architecture |
| HIGH-7 | CSRF on auth endpoints | Needs CSRF token framework or SameSite policy decision |
| MED-1 | Rate limiting spoofable | Needs reverse proxy trust configuration |
| HIGH-2 | Argon2 time=1 | Pre-existing, separate hardening pass |
| LOW 23-25 | Various design improvements | Non-critical |

---

## Questions for You

1. **Phase 3 scope**: Should we tackle HIGH-6 (HMAC key architecture) and HIGH-7 (CSRF) now, or defer them? They require more design work.

2. **CRIT-2 — UpdateUser struct**: The `UpdateUser` struct in `store/user.go` doesn't have an `AllowedTenantIDs` field. Should I add it as `*string` (JSON-encoded) or `[]string` (with store-level marshaling)?

3. **MED-4 — UpsertMemoRelation**: The current INSERT has `UNIQUE(memo_id, related_memo_id, type)` but no `ON CONFLICT`. Should I add `ON CONFLICT DO UPDATE` (true upsert) or `ON CONFLICT DO NOTHING` (ignore duplicates)?

4. **HIGH-7 — CSRF**: Two options: (a) set `SameSite=Lax` on auth cookies (simpler, admin frontend uses Authorization header), or (b) add CSRF token framework. Preference?

5. **MED-1 — IP spoofing**: Do you use a reverse proxy (nginx/cloudflare) that sets `True-Client-IP`? Or should I make the trusted header configurable?

---

## User

write your implementation plan as plan2.md and include all your detailed questions, then I will get back to you

---

## Assistant

Let me first read the key files referenced in the review to understand the current state before writing the plan.

**Tool: read**

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: grep**

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: read**

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: read**

**Tool: grep**

**Tool: read**

---

## Assistant

**Tool: grep**

**Tool: grep**

---

## Assistant

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: read**

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: grep**

**Tool: grep**

---

## Assistant

Now let me check the auth rate limiting pattern and the memo_service.go delete cascade:

**Tool: grep**

**Tool: read**

---

## Assistant

Now I have enough context. Let me write the implementation plan.

**Tool: write**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: write**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: write**

---

## Assistant

The plan file write is being denied by permissions. Let me output the plan directly for you to review.

---

# Implementation Plan: Coding Review Fixes (plan2)

**Source:** `bugs/022/coding_review.md`
**Scope:** 6 Critical, 10 High, 6 Medium issues

---

## Phase 1: P0 — Critical Runtime Fixes (4 issues, ~30 min)

### CRIT-1: Migration Table Name Mismatch
**File:** `store/migration/sqlite/0.28/01__add_max_message_length.sql`
**Fix:** `agent_audience` → `agent_audiences`

### CRIT-2: `allowed_tenant_ids` Never Read/Written
**Files:** `store/user.go`, `store/db/sqlite/user.go`
**Fix:** Add `AllowedTenantIDs *string` to `UpdateUser` struct. Add to CreateUser INSERT, UpdateUser SET, ListUsers SELECT+Scan. Marshal/unmarshal JSON at SQL boundary.

### CRIT-3: `DeleteMemoRelation` Has No TenantID
**Files:** `store/memo_relation.go`, `store/db/sqlite/memo_relation.go`, `memo_service.go:460`
**Fix:** Add `TenantID *int32` to `DeleteMemoRelation` struct. Add `AND tenant_id = ?` to SQL when set. Pass `memo.TenantID` at call site.

### CRIT-4: `ListMemoRelations` Handler Omits Tenant Filter
**File:** `memo_relation_service.go:99,113`
**Fix:** Add `TenantID: memo.TenantID` to both `FindMemoRelation` calls.

---

## Phase 2: P1 — Security & Memory Fixes (8 issues, ~2 hours)

| # | Issue | Fix | File |
|---|-------|-----|------|
| CRIT-5 | Session lock leak | Delete from `sessionLocks` in `cleanup()` | `service.go:1062` |
| CRIT-6 | No rate limit on select-tenant | Add `loginRateLimiter.Allow(clientIP)` | `auth_service.go:447` |
| HIGH-3 | Middleware fails open | Return 500 on DB error, 403 on nil user | `tenant_binding.go:22` |
| HIGH-4 | Token in JSON body | Remove `access_token`/`cookie`, keep only `tenant_id` | `auth_service.go:536` |
| HIGH-5 | ChatInternal no length check | Add `MaxMessageLength` validation | `service.go:1768` |
| HIGH-8 | XSS in iframe | Add `</script>` and `</` escaping | `handlers.go:2099` |
| HIGH-9 | Nil TenantID bypass | Rewrite conditional at 3 locations | `memo_service.go:266,310,439` |
| HIGH-10 | REFERENCE no tenant_id | Add `TenantID: memo.TenantID` to upsert | `memo_relation_service.go:60` |

---

## Phase 3: P2 — Hardening (6 issues, ~2 hours)

| # | Issue | Fix | File |
|---|-------|-----|------|
| HIGH-1 | Backup key same salt | Derive via `HMAC(salt, "backup")` | `encryption.go:40` |
| MED-2 | Sscanf into time.Time | Use `int64` temp + `time.Unix()` | `auth_service.go:477` |
| MED-3 | Rate limiter no eviction | Add cleanup goroutine | `login_ratelimit.go` |
| MED-4 | UpsertMemoRelation no ON CONFLICT | Add `ON CONFLICT` clause | `memo_relation.go:12` |
| MED-5 | Playground race | Add `sync.Mutex` to Handler | `playground.go` |
| MED-6 | Backup fallback silent | Add `slog.Warn` | `encryption.go:89` |

---

## Deferred Issues

| # | Issue | Reason |
|---|-------|--------|
| HIGH-2 | Argon2 time=1 | Pre-existing, separate pass |
| HIGH-6 | HMAC key = public GUID | Needs per-tenant signing key architecture |
| HIGH-7 | CSRF on auth | Needs SameSite/Lax vs CSRF token decision |
| MED-1 | Rate limit spoofable | Needs reverse proxy trust config |
| LOW 23-25 | Various | Non-critical |

---

## Open Questions for You

**Q1 (CRIT-2):** `AllowedTenantIDs` storage — Option A (marshal at SQL boundary, keep `[]string` in Go) vs Option B (store raw JSON `*string`, marshal in middleware)?

**Q2 (MED-4):** `UpsertMemoRelation` — `ON CONFLICT DO UPDATE` (true upsert) vs `ON CONFLICT DO NOTHING` (ignore)?

**Q3 (HIGH-7):** CSRF — `SameSite=Lax` on auth cookies (simple) vs CSRF token framework (full protection)?

**Q4 (MED-1):** IP trust — configurable header via env var, or hardcode `X-Forwarded-For`?

**Q5 (HIGH-4):** Does the frontend JS read `access_token` from the JSON body, or only from the cookie?

**Q6 (CRIT-5):** Session lock cleanup — acquire both locks (`mu` → `locksMu`) or unify into single lock?

---

## User

just write your plan as plan2.md including all your questions, then I will answer back later

---

## Assistant

**Tool: write**

---

## Assistant

**Tool: bash**

---

## Assistant

Written to `bugs/022/plan2.md` (318 lines). 

The plan covers all 22 issues from the coding review across 4 phases, with 6 open questions (Q1-Q6) that need your answers before implementation. Key decisions needed:

- **Q1**: `AllowedTenantIDs` storage format (Option A: marshal at SQL boundary vs Option B: raw JSON in struct)
- **Q2**: `UpsertMemoRelation` ON CONFLICT behavior (upsert vs ignore)
- **Q3**: CSRF strategy (SameSite=Lax vs CSRF token framework)
- **Q4**: IP trust for rate limiting (configurable header vs hardcoded)
- **Q5**: Frontend dependency on `access_token` in JSON body
- **Q6**: Session lock cleanup strategy (two-lock ordering vs unified lock)

Reply with your answers and I'll start implementing.

---

## User

read plan2_review.md and plan how to implement it, ask if you need to clarify anything then let me know when you are ready to code so I can switch to build mode

---

## Assistant

**Tool: read**

---

## Assistant

Let me check the additional `DeleteMemoRelation` callsites mentioned in Nit 1:

**Tool: grep**

---

## Assistant

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

Now I have all the context. Let me summarize the implementation plan incorporating all 5 nits.

**Tool: read**

---

## Assistant

All context gathered. Here's the implementation plan incorporating all 5 nits from the review:

---

## Implementation Plan Summary

### Phase 1: P0 — Critical Runtime Fixes (4 issues)

**CRIT-1** — Trivial: Fix `agent_audience` → `agent_audiences` in migration SQL.

**CRIT-2** — `AllowedTenantIDs` (Option A: marshal at SQL boundary):
- Add `AllowedTenantIDs *string` to `UpdateUser` struct in `store/user.go`
- `CreateUser`: marshal `[]string` to JSON, add to INSERT
- `UpdateUser`: add conditional SET for `allowed_tenant_ids`
- `ListUsers`: scan `sql.NullString`, unmarshal to `[]string`
- All in `store/db/sqlite/user.go`

**CRIT-3** — `DeleteMemoRelation` tenant scoping (Nit 1: fix ALL 3 callsites):
- Add `TenantID *int32` to `DeleteMemoRelation` struct
- Add `AND tenant_id = ?` to SQL when set
- Update callsites:
  - `memo_service.go:460` — `TenantID: memo.TenantID`
  - `memo_service.go:493` — `TenantID: memo.TenantID`
  - `memo_relation_service.go:31` — `TenantID: memo.TenantID`

**CRIT-4** — Add `TenantID: memo.TenantID` to both `ListMemoRelations` calls in `memo_relation_service.go:99,113`.

### Phase 2: P1 — Security & Memory Fixes (8 issues)

**CRIT-5** — Session lock cleanup (Nit 4: handle orphaned locks):
- In `cleanup()`, acquire `mu` → `locksMu`
- Delete from both `sessions` and `sessionLocks` for expired keys
- After deleting from `sessions`, iterate `sessionLocks` and remove any key not in `sessions` (orphaned locks)

**CRIT-6** — Add `loginRateLimiter.Allow(clientIP)` to `HandleSelectTenant`.

**HIGH-3** — TenantBindingMiddleware fail-closed: return 500 on DB error, 403 on nil user.

**HIGH-4** — Remove `access_token` and `cookie` from `HandleSelectTenant` JSON response (confirmed safe: frontend doesn't read them).

**HIGH-5** — Add `MaxMessageLength` validation to `ChatInternal`.

**HIGH-8** — Add `</script>` and `</` escaping to `escapeJS`.

**HIGH-9** — Nil TenantID fix (Nit 2: document legacy memo behavior):
- If `memo.TenantID != nil` and caller has no tenant or different tenant → deny (unless superuser)
- If `memo.TenantID == nil` → allow (legacy memo, globally accessible — this is the intended behavior, document it)

**HIGH-10** — Add `TenantID: memo.TenantID` to REFERENCE upsert in `memo_relation_service.go:60`.

**Nit 5 (HIGH-6 quick mitigation)** — Derive HMAC key:
- Change `generateSessionToken` and `verifySessionToken` to use `HMAC(tenant.GUID, "session-token-key")` as the signing key instead of `tenant.GUID` directly
- This prevents the GUID (publicly exposed in admin API) from being usable as-is for token forgery

### Phase 3: P2 — Hardening (6 issues)

**HIGH-1** — Backup salt derivation via `HMAC(primarySalt, "backup-key-salt")`.

**MED-2** — `Sscanf` fix: use `int64` temp variable.

**MED-3** — Rate limiter eviction (Nit 3: bounded map):
- Add cleanup goroutine with 10-minute ticker
- Add `maxEntries int` field (default 10,000)
- In `Allow()`, if `len(windows) >= maxEntries` and entry doesn't exist, skip (don't create new entry)
- Cleanup removes expired entries

**MED-4** — `UpsertMemoRelation` add `ON CONFLICT DO UPDATE SET tenant_id = excluded.tenant_id`.

**MED-5** — Playground race: add `sync.Mutex` to Handler struct.

**MED-6** — Backup key fallback: add `slog.Warn`.

### Deferred
HIGH-2, HIGH-6 (full architecture), HIGH-7, MED-1, LOW 23/24/25.

---

No remaining questions — all 6 open questions are answered in the review, and all 5 nits are incorporated. Ready to code when you switch to build mode.

---

## User

go

---

## Assistant

Starting Phase 1 implementation.

**Tool: todowrite**

---

## Assistant

**CRIT-1:** Fix migration table name.

**Tool: edit**

---

## Assistant

**Tool: todowrite**

---

## Assistant

**CRIT-2:** Add `AllowedTenantIDs` field to `UpdateUser` struct.

**Tool: edit**

---

## Assistant

Now update `CreateUser` in sqlite to include `allowed_tenant_ids`:

**Tool: edit**

---

## Assistant

**Tool: edit**

---

## Assistant

Now update `UpdateUser` to include `allowed_tenant_ids`:

**Tool: edit**

---

## Assistant

Now update `ListUsers` to scan `allowed_tenant_ids`:

**Tool: edit**

---

## Assistant

**Tool: edit**

---

## Assistant

Now update `UpdateUser`'s RETURNING clause to include `allowed_tenant_ids`:

**Tool: edit**

---

## Assistant

Now **CRIT-3:** Add `TenantID` to `DeleteMemoRelation` and update all callsites.

**Tool: todowrite**

---

## Assistant

Add `TenantID` to `DeleteMemoRelation` struct:

**Tool: edit**

---

## Assistant

Now add `tenant_id` WHERE clause to `DeleteMemoRelation` SQL:

**Tool: edit**

---

## Assistant

Now update all 3 callsites. First `memo_service.go:460`:

**Tool: edit**

---

## Assistant

**Tool: edit**

---

## Assistant

Now update `memo_relation_service.go:31`:

**Tool: edit**

---

## Assistant

Now **CRIT-4:** Add `TenantID` to `ListMemoRelations` calls.

**Tool: edit**

---

## Assistant

**Tool: edit**

---

## Assistant

Now **CRIT-5:** Fix session lock memory leak with orphaned lock cleanup.

**Tool: todowrite**

---

## Assistant

**Tool: edit**

---

## Assistant

Now **CRIT-6:** Add rate limiting to `HandleSelectTenant`.

**Tool: edit**

---

## Assistant

Now **HIGH-3:** TenantBindingMiddleware fail-closed on DB error.

**Tool: edit**

---

## Assistant

Now **HIGH-4:** Remove `access_token`/`cookie` from select-tenant JSON response.

**Tool: edit**

---

## Assistant

Now **HIGH-5:** Add message length validation to `ChatInternal`.

**Tool: edit**

---

## Assistant

Now **HIGH-6:** Derive HMAC session key from `HMAC(GUID, "session-token-key")`.

**Tool: edit**

---

## Assistant

Now **HIGH-8:** Fix XSS in `escapeJS`.

**Tool: edit**

---

## Assistant

Now **HIGH-9:** Fix nil TenantID bypass in `memo_service.go` (3 locations).

**Tool: edit**

---

## Assistant

Now the UpdateMemo location:

**Tool: edit**

---

## Assistant

Now the DeleteMemo location:

**Tool: edit**

---

## Assistant

Now **HIGH-10:** Add `TenantID` to REFERENCE relation upsert.

**Tool: edit**

---

## Assistant

Now Phase 3 hardening. **HIGH-1:** Derive backup salt via HMAC.

**Tool: edit**

---

## Assistant

**Tool: edit**

---

## Assistant

**MED-2:** Fix `Sscanf` into `time.Time` memory corruption.

**Tool: edit**

---

## Assistant

**MED-3:** Add bounded eviction to login rate limiter.

**Tool: edit**

---

## Assistant

**MED-4:** Add `ON CONFLICT` to `UpsertMemoRelation`.

**Tool: edit**

---

## Assistant

**MED-5:** Add mutex to playground `ensurePlaygroundDemo`.

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

Now add the mutex lock/unlock to `ensurePlaygroundDemo`:

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**MED-6:** Add `slog.Warn` for backup key fallback.

**Tool: edit**

---

## Assistant

Now build and test to verify everything compiles.

**Tool: bash**

---

## Assistant

The `cookie` variable is now unused. Let me check what's still using it:

**Tool: read**

---

## Assistant

The `cookie` variable is assigned but no longer used in the response. I'll use `_`:

**Tool: edit**

---

## Assistant

**Tool: read**

---

## Assistant

I need to set SameSite and Secure flags properly on the Echo SetCookie call. Let me also remove the unused `buildAccessTokenCookie` call:

**Tool: edit**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: bash**

---

## Assistant

Build passes. Now run the tests:

**Tool: bash**

---

## Assistant

All tests pass. Let me update the todo list:

**Tool: todowrite**

---

## Assistant

All 19 issues implemented. Build and tests pass. Here's a summary:

## Completed (19 issues)

### Phase 1 — Critical Runtime Fixes
- **CRIT-1:** Fixed migration table name `agent_audience` → `agent_audiences`
- **CRIT-2:** Wired `allowed_tenant_ids` through CreateUser/UpdateUser/ListUsers + added to UpdateUser struct
- **CRIT-3:** Added `TenantID` to `DeleteMemoRelation` + updated all 3 callsites
- **CRIT-4:** Added `TenantID: memo.TenantID` to both `ListMemoRelations` calls

### Phase 2 — Security & Memory Fixes
- **CRIT-5:** Fixed session lock leak + orphaned lock cleanup in `cleanup()`
- **CRIT-6:** Added rate limiting to `HandleSelectTenant`
- **HIGH-3:** TenantBindingMiddleware now fails closed (500 on DB error, 403 on nil user)
- **HIGH-4:** Removed `access_token`/`cookie` from select-tenant JSON response
- **HIGH-5:** Added `MaxMessageLength` validation to `ChatInternal`
- **HIGH-6:** Derived HMAC session key via `HMAC(GUID, "session-token-key")`
- **HIGH-8:** Added `</script>` and `</` escaping to `escapeJS`
- **HIGH-9:** Fixed nil TenantID bypass at 3 locations (documented legacy memo behavior)
- **HIGH-10:** Added `TenantID: memo.TenantID` to REFERENCE relation upsert

### Phase 3 — Hardening
- **HIGH-1:** Derived backup salt via `HMAC(primarySalt, "backup-key-salt")`
- **MED-2:** Fixed `Sscanf` into `time.Time` using `int64` temp variable
- **MED-3:** Added bounded eviction (10K max entries) + cleanup goroutine to rate limiter
- **MED-4:** Added `ON CONFLICT DO UPDATE SET tenant_id = excluded.tenant_id` to UpsertMemoRelation
- **MED-5:** Added `sync.Mutex` to Handler struct guarding `ensurePlaygroundDemo`
- **MED-6:** Added `slog.Warn` when backup key fallback succeeds

---

