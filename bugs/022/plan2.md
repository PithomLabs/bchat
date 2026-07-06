# Implementation Plan: Coding Review Fixes (plan2)

**Date:** 2026-07-06
**Source:** `bugs/022/coding_review.md`
**Scope:** 6 Critical, 10 High, 6 Medium issues

---

## Phase 1: P0 — Critical Runtime Fixes (4 issues, ~30 min)

### CRIT-1: Migration Table Name Mismatch

**File:** `store/migration/sqlite/0.28/01__add_max_message_length.sql`

**Problem:** Table name `agent_audience` (singular) vs actual `agent_audiences` (plural). Migration always fails.

**Fix:** One-word change:
```sql
ALTER TABLE agent_audiences ADD COLUMN max_message_length INTEGER DEFAULT 4000;
```

**Risk:** None. Trivial fix.

---

### CRIT-2: `allowed_tenant_ids` Never Read/Written — Tenant Binding Dead Code

**Files:** `store/user.go`, `store/db/sqlite/user.go`

**Problem:** The migration adds `allowed_tenant_ids` column, the Go struct has `AllowedTenantIDs []string`, but:
- `CreateUser` INSERT never includes the column
- `UpdateUser` UPDATE never touches it
- `ListUsers` SELECT never scans it
- `UpdateUser` struct lacks the field

**Result:** `AllowedTenantIDs` is always nil. `TenantBindingMiddleware` always sees `len(user.AllowedTenantIDs) == 0` → always grants superuser bypass.

**Fix:**

1. **`store/user.go`** — Add field to `UpdateUser`:
   ```go
   type UpdateUser struct {
       // ... existing fields ...
       AllowedTenantIDs *string  // JSON-encoded array of tenant GUIDs, null = all tenants
   }
   ```

2. **`store/db/sqlite/user.go` — `CreateUser`** — Add `allowed_tenant_ids` to INSERT:
   - Marshal `AllowedTenantIDs` to JSON before INSERT
   - Add column and placeholder to the SQL

3. **`store/db/sqlite/user.go` — `UpdateUser`** — Add conditional SET:
   ```go
   if v := update.AllowedTenantIDs; v != nil {
       set, args = append(set, "allowed_tenant_ids = ?"), append(args, *v)
   }
   ```

4. **`store/db/sqlite/user.go` — `ListUsers`** — Add to SELECT and Scan:
   - Scan `allowed_tenant_ids` into `sql.NullString`
   - Unmarshal JSON into `[]string`

**See Question Q1 for storage format decision.**

---

### CRIT-3: `DeleteMemoRelation` Has No TenantID — All Deletes Unscoped

**Files:** `store/memo_relation.go`, `store/db/sqlite/memo_relation.go`, `server/router/api/v1/memo_service.go`

**Problem:** `DeleteMemoRelation` struct has no `TenantID`. SQL DELETE has no `tenant_id` clause. Deleting one memo can destroy cross-tenant relations.

**Fix:**

1. **`store/memo_relation.go`** — Add field:
   ```go
   type DeleteMemoRelation struct {
       MemoID        *int32
       RelatedMemoID *int32
       Type          *MemoRelationType
       TenantID      *int32  // NEW: scope delete to tenant
   }
   ```

2. **`store/db/sqlite/memo_relation.go` — `DeleteMemoRelation`** — Add conditional WHERE:
   ```go
   if delete.TenantID != nil {
       where, args = append(where, "tenant_id = ?"), append(args, delete.TenantID)
   }
   ```

3. **`server/router/api/v1/memo_service.go:460`** — Pass TenantID:
   ```go
   s.Store.DeleteMemoRelation(ctx, &store.DeleteMemoRelation{
       MemoID:   &memo.ID,
       TenantID: memo.TenantID,  // NEW
   })
   ```

---

### CRIT-4: `ListMemoRelations` Handler Omits Tenant Filter

**File:** `server/router/api/v1/memo_relation_service.go:99,113`

**Problem:** Both forward and reverse `ListMemoRelations` calls have no `TenantID`. Cross-tenant reference data leaks.

**Fix:** Add `TenantID: memo.TenantID` to both calls (lines 99 and 113).

---

## Phase 2: P1 — Security & Memory Fixes (8 issues, ~2 hours)

### CRIT-5: Session Lock Memory Leak — Unbounded Growth

**File:** `server/router/api/v1/agent/service.go:1062-1072`

**Problem:** `cleanup()` only evicts from `sessions` map. `sessionLocks` map is never cleaned.

**Fix:** Delete from both maps in `cleanup()`. Acquire both `s.mu` and `s.locksMu` (in that order to avoid deadlock).

**See Question Q6 for lock ordering decision.**

---

### CRIT-6: `HandleSelectTenant` Has No Rate Limiting

**File:** `server/router/api/v1/auth_service.go:447-541`

**Fix:** Add rate limiting at the top of `HandleSelectTenant` using `s.loginRateLimiter.Allow(clientIP)`.

---

### HIGH-3: TenantBindingMiddleware Fails Open on DB Error

**File:** `server/router/api/v1/tenant_binding.go:22-24`

**Fix:** Return HTTP 500 on DB error, HTTP 403 on nil user (fail-closed).

---

### HIGH-4: Access Token Leaked in JSON Response Body

**File:** `server/router/api/v1/auth_service.go:536-540`

**Fix:** Remove `access_token` and `cookie` from JSON response. Return only `tenant_id`.

**See Question Q5 for frontend dependency check.**

---

### HIGH-5: `ChatInternal` Missing Message Length Validation

**File:** `server/router/api/v1/agent/service.go:1768-1804`

**Fix:** Add same `MaxMessageLength` validation as `ChatExternal`, after config load.

---

### HIGH-8: XSS via Incomplete JS Escaping in Iframe HTML

**File:** `server/router/api/v1/agent/handlers.go:2099-2105`

**Fix:** Add `</script>` and `</` escaping to `escapeJS`.

---

### HIGH-9: Nil TenantID in Context Bypasses Ownership Checks

**File:** `server/router/api/v1/memo_service.go:265-268, 309-312, 438-441`

**Fix:** Change conditional logic at 3 locations:
```go
if memo.TenantID != nil {
    if tenantID == nil || *memo.TenantID != *tenantID {
        if !isSuperUser(user) {
            return nil, status.Errorf(codes.PermissionDenied, "permission denied")
        }
    }
}
```

---

### HIGH-10: REFERENCE Relations Never Get tenant_id Set

**File:** `server/router/api/v1/memo_relation_service.go:60-64`

**Fix:** Add `TenantID: memo.TenantID` to the REFERENCE upsert.

---

## Phase 3: P2 — Hardening (6 issues, ~2 hours)

### HIGH-1: Backup Key Reuses Same Salt

**File:** `internal/crypto/encryption.go:40-44`

**Fix:** Derive backup salt via `HMAC(primarySalt, "backup-key-salt")`.

---

### MED-2: `Sscanf` into time.Time — Memory Corruption

**File:** `server/router/api/v1/auth_service.go:477-478`

**Fix:** Use `int64` temp variable with `time.Unix()`.

---

### MED-3: Login Rate Limiter Has No Eviction

**File:** `server/router/api/v1/login_ratelimit.go`

**Fix:** Add cleanup goroutine with 10-minute ticker.

---

### MED-4: `UpsertMemoRelation` Is INSERT-Only, No ON CONFLICT

**File:** `store/db/sqlite/memo_relation.go:12-21`

**Fix:** Add `ON CONFLICT` clause.

**See Question Q2 for ON CONFLICT behavior decision.**

---

### MED-5: Playground Race Between Startup Seeding and Catalog

**File:** `server/router/api/v1/agent/playground.go`

**Fix:** Add `sync.Mutex` to Handler struct, guard `ensurePlaygroundDemo`.

---

### MED-6: Backup Key Fallback Succeeds Silently

**File:** `internal/crypto/encryption.go:78-97`

**Fix:** Add `slog.Warn` when backup key succeeds.

---

## Deferred Issues

| # | Issue | Reason Deferred |
|---|-------|----------------|
| HIGH-2 | Argon2 time=1 | Pre-existing, separate hardening pass |
| HIGH-6 | HMAC key is public GUID | Needs per-tenant signing key architecture |
| HIGH-7 | CSRF on auth endpoints | Needs SameSite/Lax vs CSRF token decision |
| MED-1 | Rate limiting spoofable | Needs reverse proxy trust config decision |
| LOW-23 | MaxMessageLength zero-value | Breaking change, use `*int` |
| LOW-24 | Internal sessions never cleaned up | Needs TTL/pruning design |
| LOW-25 | Playground definitions re-allocated | Minor optimization |

---

## Open Questions

### Q1: CRIT-2 — `AllowedTenantIDs` storage format

**Option A:** Keep `[]string` in Go struct, marshal/unmarshal at SQL boundary in `store/db/sqlite/user.go`. Keeps Go types clean, marshaling isolated to store layer.

**Option B:** Change `User.AllowedTenantIDs` to `*string` (raw JSON), marshal in handler/middleware. Simpler store layer but more work in TenantBindingMiddleware.

**Recommendation:** Option A.

### Q2: MED-4 — `UpsertMemoRelation` ON CONFLICT behavior

**Option A:** `ON CONFLICT DO UPDATE SET tenant_id = excluded.tenant_id` — true upsert, updates tenant_id if relation exists.

**Option B:** `ON CONFLICT DO NOTHING` — silently ignore duplicates.

**Recommendation:** Option A (function is named "Upsert").

### Q3: HIGH-7 — CSRF protection strategy

**Option A:** Set `SameSite=Lax` for auth cookies (admin frontend uses Authorization header anyway).

**Option B:** Add CSRF token framework (significant effort, new endpoint, frontend changes).

**Recommendation:** Option A.

### Q4: MED-1 — Rate limiting IP trust

**Option A:** Use `True-Client-IP` or `X-Real-IP` (requires knowing reverse proxy).

**Option B:** Make trusted header configurable via env var.

**Option C:** Accept limitation, document it.

**Recommendation:** Option B for flexibility.

### Q5: HIGH-4 — Frontend dependency on access_token in JSON

Before removing `access_token` from JSON response: does the frontend JS read it from the body, or rely solely on the cookie?

### Q6: CRIT-5 — Lock ordering for session cleanup

The `MemorySessionStore` has two locks: `mu` (sessions) and `locksMu` (sessionLocks). Cleanup needs both. Options:

**Option A:** Acquire `mu` → `locksMu` in `cleanup()`. Consistent ordering, safe.

**Option B:** Unify into single lock. Simpler but changes `SessionLock` concurrency semantics.

**Recommendation:** Option A (preserves existing concurrency semantics of `SessionLock`).

---

## Implementation Order

1. **Phase 1** (CRIT-1 through CRIT-4): ~30 min, zero design decisions
2. **Phase 2** (CRIT-5, CRIT-6, HIGH-3 through HIGH-10): ~2 hours
3. **Phase 3** (HIGH-1, MED-2 through MED-6): ~2 hours
4. **Deferred**: Requires separate design discussion

## Estimated Total: ~4-5 hours implementation + testing
