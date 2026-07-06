# Implementation Plan Review — bugs/022

**Date:** 2026-07-06
**Reviewer:** opencode
**Status:** Approved with nits — no implementation yet

---

## Verdict: Approved

The implementation plan (`plan_imp.md`) is thorough and well-structured. All 13 issues from the DeepWiki analysis plus the memo comment isolation finding are covered with clear root causes, implementation steps, and file references. The phased ordering is correct — critical isolation first, auth hardening second, input validation third, network security fourth, infrastructure last.

No rework needed. The following nits should be addressed during implementation.

---

## Nits

### Nit 1: Issue #2 — Fragile error detection via string matching

**Current approach:**
```go
if strings.Contains(err.Error(), "exceeds maximum length") {
    return echo.NewHTTPError(http.StatusBadRequest, err.Error())
}
```

**Problem:** Brittle. If the error message wording changes, the check breaks silently and the error becomes a 500 instead of 400.

**Fix:** Use a custom sentinel error:
```go
var ErrMessageTooLong = errors.New("message too long")

// In ChatExternal:
return nil, fmt.Errorf("%w: %d characters max", ErrMessageTooLong, maxLen)

// In handler:
if errors.Is(err, service.ErrMessageTooLong) {
    return echo.NewHTTPError(http.StatusBadRequest, err.Error())
}
```

---

### Nit 2: Issue #9 — Login rate limiting with tenantID=0

**Current approach:**
```go
allowed, err := s.CheckRateLimit(ctx, 0, "login", clientIP, 5)
```

**Problem:** `CheckRateLimit` writes to `agent_rate_limits` keyed on `(tenant_id, audience_type, client_ip)`. Passing `tenant_id=0` could collide with other rate limit entries if any real tenant has ID 0, or cause a foreign key violation if the table has an FK constraint on `tenant_id`.

**Fix:** Verify that `CheckRateLimit` handles `tenant_id=0` correctly. If not, either:
- Use a separate `"login"` audience type with a distinct key (no tenant FK)
- Add a dedicated `login_rate_limits` table
- Use `tenant_id = -1` as a sentinel for "no tenant" and ensure the FK allows it

---

### Nit 3: Issue #12 — Middleware needs store access

**Current approach:**
```go
func TenantBindingMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // ...
        tenant, _ := store.GetAgentTenantBySlug(slug)  // no store reference
```

**Problem:** Standalone middleware function has no reference to the store. `store.GetAgentTenantBySlug` is not a package-level function.

**Fix:** Capture the store in a closure:
```go
func TenantBindingMiddleware(s *store.Store) echo.HandlerFunc {
    return func(c echo.Context) error {
        user := getUserFromContext(c)
        if isSuperUser(user) {
            return next(c)
        }
        slug := c.Param("slug")
        if slug == "" {
            return next(c)
        }
        tenant, _ := s.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
        if tenant == nil {
            return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
        }
        if user.AllowedTenantIDs != nil && !contains(user.AllowedTenantIDs, tenant.GUID) {
            return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
        }
        return next(c)
    }
}
```

---

### Nit 4: Issue #6 — Token verification must reconstruct signed data from query params

**Current approach:** HMAC signs `sessionID + expiry`, transcript endpoint receives both as query params.

**Correctness:** This is fine architecturally, but the verify function must reconstruct the exact same byte sequence that was signed:
```go
func verifySessionToken(token, sessionID, tenantGUID string) (time.Time, error) {
    // token is HMAC(sessionID + expiry)
    // Need to try each possible expiry... 
```

**Problem:** The expiry is embedded in the HMAC but not returned separately. The verifier can't know the expiry without decoding it. Two options:
- Return `expiry` as a separate query param alongside `token`, and verify `HMAC(sessionID + expiry)` against `token`
- Store the expiry in the HMAC signature itself and extract it during verification

**Fix:** Include expiry as a query param:
```
GET /api/v1/agent/:slug/chat/ext/transcript?session_id=xxx&expiry=2026-07-06T12:00:00Z&token=<hmac>
```
Verify: `HMAC(sessionID + expiry, tenantGUID) == token` and `time.Now().Before(expiry)`.

---

### Nit 5: Issue #14 — SQLite ALTER TABLE idempotency

**Current migration:**
```sql
ALTER TABLE memo_relation ADD COLUMN tenant_id INTEGER DEFAULT NULL;
```

**Problem:** SQLite does not support `IF NOT EXISTS` for `ALTER TABLE ADD COLUMN`. If the migration runs twice (e.g., partial failure, re-run), it will fail with "duplicate column name".

**Fix:** Either:
- Wrap in a programmatic check (Go migration code checks if column exists before altering)
- Use the migration framework's idempotency mechanism if available
- Accept that the migration framework ensures single-execution

---

### Nit 6: Issue #1 — Missing `getEnvSlice` helper

**Current approach:**
```go
adminOrigins := getEnvSlice("ADMIN_CORS_ORIGINS", []string{})
```

**Problem:** `getEnvSlice` is not defined anywhere in the codebase.

**Fix:** Add the helper:
```go
func getEnvSlice(key string, defaultVal []string) []string {
    val := os.Getenv(key)
    if val == "" {
        return defaultVal
    }
    parts := strings.Split(val, ",")
    result := make([]string, 0, len(parts))
    for _, p := range parts {
        p = strings.TrimSpace(p)
        if p != "" {
            result = append(result, p)
        }
    }
    return result
}
```

---

## Summary

| Issue | Status | Action Needed |
|-------|--------|---------------|
| #14 Memo comments | Approved | No changes |
| #1 CORS | Approved | Add `getEnvSlice` helper (Nit 6) |
| #2 Message length | Approved | Use sentinel error (Nit 1) |
| #3 Prompt injection | Approved | No changes |
| #4 Never-expire tokens | Approved | No changes |
| #5 gRPC insecure | Approved | No changes |
| #6 HMAC tokens | Approved | Return expiry as separate param (Nit 4) |
| #7 Playground | Approved | No changes |
| #8 Slug enumeration | Approved | No changes |
| #9 Rate limiting | Approved | Verify tenant_id=0 handling (Nit 2) |
| #10 Encryption key | Approved | No changes |
| #11 Cross-tenant audit | Approved | No changes |
| #12 Admin tenant binding | Approved | Fix middleware signature (Nit 3) |
| #13 Domain allowlist | Approved | No changes |

**Total: 14 issues approved. 6 nits to address during implementation.**

---

## Sign-Off

- [x] Plan reviewed
- [x] Approved with nits (no rework needed)
- [ ] Nits addressed during implementation
- [ ] Ready to begin Phase 1
