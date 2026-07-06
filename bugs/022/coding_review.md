# Adversarial Code Review — bugs/022

**Date:** 2026-07-06
**Reviewer:** opencode
**Status:** Rework Required

---

## Verdict: Rework

The implementation covers all 14 issues from the DeepWiki analysis plus the 6 nits from `plan_imp_review.md`. However, the adversarial code review uncovered **6 critical** and **10 high** severity bugs — several of which are showstoppers that would cause runtime failures or security regressions if shipped. The memo comment isolation has fundamental design gaps, the tenant binding feature is completely inoperable, and there are memory safety and CSRF issues.

---

## Critical (6) — Must Fix Before Approval

### CRIT-1: Migration Table Name Mismatch — Migration Always Fails

**File:** `store/migration/sqlite/0.28/01__add_max_message_length.sql:3`

```sql
ALTER TABLE agent_audience ADD COLUMN max_message_length INTEGER DEFAULT 4000;
```

Table name is `agent_audience` (singular). Actual table is `agent_audiences` (plural). **This migration will fail at runtime** with `no such table: agent_audience`. Every database that applies this migration will not get the column, and the Go code that reads/writes `MaxMessageLength` will fail with SQL errors.

**Fix:** `ALTER TABLE agent_audiences ADD COLUMN max_message_length INTEGER DEFAULT 4000;`

---

### CRIT-2: `allowed_tenant_ids` Never Read/Written — Tenant Binding Is Dead Code

**Files:** `store/db/sqlite/user.go` (CreateUser, UpdateUser, ListUsers)

The migration adds the column, the Go struct declares the field, but:
- **CreateUser** INSERT never includes `allowed_tenant_ids` — silently dropped
- **UpdateUser** UPDATE never touches `allowed_tenant_ids` — impossible to set
- **ListUsers** SELECT never scans `allowed_tenant_ids` — always returns nil

**Consequence:** `TenantBindingMiddleware` checks `len(user.AllowedTenantIDs) == 0` which is always true, so the middleware **always grants super-user bypass**. The entire Issue #12 feature is completely non-functional.

**Fix:** Add `allowed_tenant_ids` to all three SQL operations in `user.go`. Add `AllowedTenantIDs` field to `UpdateUser` struct.

---

### CRIT-3: `DeleteMemoRelation` Has No TenantID — All Deletes Unscoped

**File:** `store/memo_relation.go:31-35`

```go
type DeleteMemoRelation struct {
    MemoID        *int32
    RelatedMemoID *int32
    Type          *MemoRelationType
    // TenantID is ABSENT
}
```

The SQL delete has no `tenant_id` clause. Every `DeleteMemoRelation` call wipes rows across all tenants. Combined with the cascade delete pattern in `DeleteMemo` (memo_service.go:460), deleting one memo can destroy cross-tenant relations.

**Fix:** Add `TenantID *int32` to `DeleteMemoRelation`. Add `AND tenant_id = ?` to the SQL delete when set.

---

### CRIT-4: `ListMemoRelations` Handler Omits Tenant Filter

**File:** `server/router/api/v1/memo_relation_service.go:99,113`

```go
tempList, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
    MemoID:     &memo.ID,
    MemoFilter: memoFilter,
})
// No TenantID set
```

Both the forward and reverse relation queries have no `TenantID`. A user in Tenant A can see REFERENCE relations pointing to/from Tenant B memos. The `MemoFilter` only checks visibility, not tenant ownership.

**Fix:** Add `TenantID: memo.TenantID` to both `FindMemoRelation` calls.

---

### CRIT-5: Session Lock Memory Leak — Unbounded Growth

**File:** `server/router/api/v1/agent/service.go:948-1072`

The `MemorySessionStore` has `sessions` and `sessionLocks` maps. The `cleanup()` method only evicts from `sessions`. The `sessionLocks` map is **never cleaned up**. Each unique `(tenantID, sessionID)` pair creates a permanent `*sync.Mutex`.

An attacker can exhaust memory by sending many unique `session_id` values via the public `POST /api/v1/agent/:slug/chat/ext` endpoint — each creates both a session entry AND a permanent lock entry.

**Fix:** In `cleanup()`, also delete from `sessionLocks` when deleting from `sessions`:
```go
func (s *MemorySessionStore) cleanup() {
    s.mu.Lock()
    defer s.mu.Unlock()
    cutoff := time.Now().Add(-s.ttl)
    for key, session := range s.sessions {
        if session.UpdatedAt.Before(cutoff) {
            delete(s.sessions, key)
            delete(s.sessionLocks, key)
        }
    }
}
```

---

### CRIT-6: `HandleSelectTenant` Has No Rate Limiting

**File:** `server/router/api/v1/auth_service.go:447-541`

`HandleAuthTenants` has rate limiting (line 363), but `HandleSelectTenant` has none. An attacker can attempt many selection tokens without throttling. Additionally, the token lookup is O(N*M) — iterates ALL users and ALL their access tokens — which is itself a DoS vector.

**Fix:** Add rate limiting to `HandleSelectTenant` using the `loginRateLimiter` (same as `HandleAuthTenants`):
```go
clientIP := c.RealIP()
if !s.loginRateLimiter.Allow(clientIP) {
    return echo.NewHTTPError(http.StatusTooManyRequests, "Too many attempts. Please try again in 60 seconds.")
}
```

---

## High (10) — Should Fix Before Approval

### HIGH-1: Backup Key Reuses Same Salt as Primary

**File:** `internal/crypto/encryption.go:40-44`

```go
backupKey = argon2.IDKey(
    []byte(backup),
    salt,         // <-- same salt as primary key
    1, 64*1024, 4, KeySize,
)
```

The backup key uses the same Argon2 salt as the primary key. This halves the brute-force work factor: one password candidate derives both keys.

**Fix:** Use a deterministic derivation from the primary salt (e.g., `HMAC(salt, "backup")`) or store a second salt.

---

### HIGH-2: Argon2 `time=1` Below OWASP Minimum

**File:** `internal/crypto/encryption.go:31`

OWASP recommends `time >= 3` for interactive login and `time >= 4` for sensitive data. Current `time=1` makes offline brute-force attacks significantly cheaper.

**Note:** This is a **pre-existing** issue, not introduced by this implementation. Should be addressed in a separate hardening pass.

---

### HIGH-3: TenantBindingMiddleware Fails Open on DB Error

**File:** `server/router/api/v1/tenant_binding.go:22-24`

```go
user, err := s.GetUser(c.Request().Context(), &store.FindUser{ID: &userID})
if err != nil || user == nil {
    return next(c)  // silently allows request through
}
```

A transient DB error during a restricted admin's request would grant them access to tenants they should not access. This is fail-open.

**Fix:** Return HTTP 500 on error:
```go
if err != nil {
    return echo.NewHTTPError(http.StatusInternalServerError, "failed to verify tenant binding")
}
if user == nil {
    return echo.NewHTTPError(http.StatusForbidden, "access denied")
}
return next(c)
```

---

### HIGH-4: Access Token Leaked in JSON Response Body

**File:** `server/router/api/v1/auth_service.go:536-540`

```go
return c.JSON(http.StatusOK, map[string]interface{}{
    "access_token": accessToken,  // accessible to JavaScript
    "cookie":       cookie,
    "tenant_id":    req.TenantID,
})
```

The access token is in both the JSON body (JS-accessible) AND the HttpOnly cookie. Any XSS can exfiltrate the token from the JSON body.

**Fix:** Remove `access_token` and `cookie` from JSON response. Return only `tenant_id`. The token is already set via `Set-Cookie` header.

---

### HIGH-5: `ChatInternal` Missing Message Length Validation

**File:** `server/router/api/v1/agent/service.go:1767-1804`

`ChatExternal` validates message length (lines 1558-1565). `ChatInternal` does not. An authenticated user with `chat:test` permission can send arbitrarily large messages, causing LLM cost amplification and memory exhaustion.

**Fix:** Add the same `MaxMessageLength` validation to `ChatInternal` before calling `processChat`.

---

### HIGH-6: HMAC Session Token Key (GUID) Is Publicly Exposed

**File:** `server/router/api/v1/agent/service.go:1526-1544`

```go
func generateSessionToken(sessionID string, expiry time.Time, tenantGUID string) string {
    mac := hmac.New(sha256.New, []byte(tenantGUID))
```

The HMAC signing key is `tenant.GUID`, which is exposed in admin API responses (handlers.go:657,726,818,3019). Any admin-level user can forge transcript access tokens for any session.

**Fix:** Use a separate, non-public signing key per tenant (e.g., derived from `ENCRYPTION_MASTER_KEY + tenant_id`, or stored encrypted in `tenant_config`).

---

### HIGH-7: CSRF on Auth Endpoints with SameSite=None Cookie

**File:** `server/router/api/v1/auth_service.go:293-321`, `server/router/api/v1/v1.go:192-196`

When the request is HTTPS, `SameSite=None` is set on the auth cookie. The REST auth endpoints use `adminCORS` but have no CSRF token protection. An attacker on a different HTTPS origin can POST to `/api/v1/auth/select-tenant` with the user's cookie to obtain a JWT.

**Fix:** Either add CSRF token protection, or set `SameSite=Lax` for auth cookies (the admin frontend can use the Authorization header instead of cookies).

---

### HIGH-8: XSS via Incomplete JS Escaping in Iframe HTML

**File:** `server/router/api/v1/agent/handlers.go:2097-2162`

`escapeJS` handles `\`, `'`, `\n`, `\r` but does NOT escape `</script>` breakout sequences. The `companyName` query parameter is user-controlled. An attacker can set it to `</script><script>alert(1)</script>`.

Compare with `generateWidgetScript` (line 1721) which correctly uses `json.Marshal`.

**Fix:** Use `json.Marshal` for all JS-embedded values, or add `</script>` escaping to `escapeJS`:
```go
func escapeJS(s string) string {
    s = strings.ReplaceAll(s, `\`, `\\`)
    s = strings.ReplaceAll(s, `'`, `\'`)
    s = strings.ReplaceAll(s, `\n`, `\n`)
    s = strings.ReplaceAll(s, `\r`, `\r`)
    s = strings.ReplaceAll(s, `</script>`, `<\/script>`)
    s = strings.ReplaceAll(s, `</`, `<\/`)
    return s
}
```

---

### HIGH-9: Nil TenantID in Context Bypasses Ownership Checks

**File:** `server/router/api/v1/memo_service.go:265-268, 309-312, 438-441`

```go
if tenantID != nil && memo.TenantID != nil && *memo.TenantID != *tenantID && !isSuperUser(user) {
    return nil, status.Errorf(codes.PermissionDenied, "permission denied")
}
```

If `tenantID` is nil (no tenant in context) OR `memo.TenantID` is nil, the check is skipped. Any memo with `tenant_id = NULL` becomes globally visible.

**Fix:** Add a check: if `memo.TenantID != nil && tenantID == nil`, deny access (non-tenant user cannot access tenant-scoped data). Only allow when both are nil (legacy mode):
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

```go
if _, err := s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
    MemoID:        memo.ID,
    RelatedMemoID: relatedMemo.ID,
    Type:          convertMemoRelationTypeToStore(relation.Type),
    // TenantID is NOT set -- defaults to nil
}); err != nil {
```

While COMMENT relations properly set TenantID, REFERENCE relations always get `tenant_id = NULL`. Combined with CRIT-4, cross-tenant reference data leaks.

**Fix:** Set `TenantID: memo.TenantID` in the REFERENCE relation upsert.

---

## Medium (6) — Should Address

### MED-1: Rate Limiting by IP — Spoofable via X-Forwarded-For

**File:** `server/router/api/v1/agent/handlers.go:590-594`

`c.RealIP()` trusts `X-Forwarded-For`. An attacker can rotate IPs, bypassing rate limits entirely.

### MED-2: `Sscanf` into time.Time — Memory Corruption

**File:** `server/router/api/v1/auth_service.go:477-478`

```go
if _, err := fmt.Sscanf(token.Description, "tenant-selection-token:%d", &tokenCreatedAt); err == nil {
```

`tokenCreatedAt` is `time.Time` but `%d` writes to it as `*int`. This corrupts the struct. Should use a temporary `int64`:
```go
var tsRaw int64
if _, err := fmt.Sscanf(token.Description, "tenant-selection-token:%d", &tsRaw); err == nil {
    tokenCreatedAt = time.Unix(tsRaw, 0)
}
```

### MED-3: Login Rate Limiter Has No Eviction

**File:** `server/router/api/v1/login_ratelimit.go`

The `windows` map grows indefinitely. Every unique IP (including spoofed) creates a permanent entry. No cleanup mechanism exists.

### MED-4: `UpsertMemoRelation` Is INSERT-Only, No ON CONFLICT

**File:** `store/db/sqlite/memo_relation.go:13-21`

Has `UNIQUE(memo_id, related_memo_id, type)` constraint but no `ON CONFLICT`. Calling it twice fails with constraint violation.

### MED-5: Playground Race Between Startup Seeding and Catalog

**File:** `server/router/api/v1/agent/playground.go:356-370, 449, 479-487`

Startup seeding and catalog handler can both call `ensurePlaygroundDemo` concurrently. TOCTOU race on `GetAgentTenant` → `CreateAgentTenant`.

### MED-6: Backup Key Fallback Succeeds Silently

**File:** `internal/crypto/encryption.go:78-97`

When primary key fails and backup succeeds, there is no logging. Operational visibility gap.

---

## Low (4) — Notes for Future

1. **Argon2 time=1** — Pre-existing issue, not introduced by this implementation
2. **`MaxMessageLength` zero-value confusion** — Go `int` zero is `0`, not "use default 4000". Should use `*int`.
3. **Internal sessions never cleaned up** — No TTL or pruning for DB-persisted sessions
4. **Playground demo definitions re-allocated per request** — Should be package-level vars

---

## Recommended Fix Priority

| Priority | Issues | Effort |
|----------|--------|--------|
| **P0: Must fix** | CRIT-1 (table name), CRIT-2 (tenant binding dead code), CRIT-3 (delete unscoped), CRIT-4 (list unscoped) | 0.5 days |
| **P1: Should fix** | CRIT-5 (session lock leak), CRIT-6 (select-tenant rate limit), HIGH-3 (fail-open), HIGH-4 (token leak), HIGH-5 (ChatInternal length), HIGH-8 (XSS), HIGH-9 (nil tenant check), HIGH-10 (reference tenant_id) | 1-2 days |
| **P2: Nice to fix** | HIGH-1 (backup salt), HIGH-6 (HMAC key), HIGH-7 (CSRF), MED-1 through MED-6 | 1-2 days |

---

## Sign-Off

- [x] Adversarial review completed
- [x] Rework required (6 critical, 10 high)
- [ ] P0 issues fixed
- [ ] P1 issues fixed
- [ ] Re-review after fixes
