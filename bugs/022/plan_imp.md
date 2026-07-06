# Implementation Plan — bugs/022 Security Fixes

**Date:** 2026-07-06
**Source:** DeepWiki Q&A security analysis + codebase investigation
**Status:** Awaiting approval before implementation

---

## Table of Contents

1. [Decisions Made](#decisions-made)
2. [Issue #14: Memo Comment Isolation (Critical)](#issue-14-memo-comment-isolation-critical)
3. [Issue #1: Wildcard CORS](#issue-1-wildcard-cors)
4. [Issue #2: No Message Length Limit](#issue-2-no-message-length-limit)
5. [Issue #3: Prompt Injection](#issue-3-prompt-injection)
6. [Issue #4: Never-Expire Tokens](#issue-4-never-expire-tokens)
7. [Issue #5: gRPC Insecure Credentials](#issue-5-grpc-insecure-credentials)
8. [Issue #6: Public Transcript Endpoint](#issue-6-public-transcript-endpoint)
9. [Issue #7: Public Playground Endpoints](#issue-7-public-playground-endpoints)
10. [Issue #8: Tenant Slug Enumeration](#issue-8-tenant-slug-enumeration)
11. [Issue #9: No Rate Limiting on Internal/Login](#issue-9-no-rate-limiting-on-internallogin)
12. [Issue #10: ENCRYPTION_MASTER_KEY](#issue-10-encryption_master_key)
13. [Issue #11: Cross-Tenant Audit](#issue-11-cross-tenant-audit)
14. [Issue #12: Admin No Tenant Isolation](#issue-12-admin-no-tenant-isolation)
15. [Issue #13: Domain Allowlist Bypassable](#issue-13-domain-allowlist-bypassable)
16. [Implementation Order](#implementation-order)
17. [Sign-Off](#sign-off)

---

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| CORS default | Empty (deny cross-origin admin) | Security-first: admin API should not be accessible cross-origin by default |
| gRPC TLS | Accept loopback risk, document only | Single-container deployment: gRPC traffic never leaves process boundary |
| Playground | Stay public, add rate limiting + message length | Intentionally public for demo/marketing; existing defenses sufficient |
| Prompt injection defense | Length limit + basic filtering + optional LLM verifier | Balance security vs. breaking legitimate user messages |
| Never-expire tokens | Remove feature entirely | Standard 7-day expiry for all tokens |
| Public endpoint auth | HMAC session tokens | Prevents transcript access by anyone who just knows/guesses a UUID |
| Scope | All 13 issues + memo comment isolation | Complete fix in one plan |

---

## Issue #14: Memo Comment Isolation (Critical)

### Problem

`memo_relation` table has no `tenant_id` column. All comment operations are tenant-unaware. Cross-tenant comment read/write is possible.

**Affected locations:**
| Operation | File | Lines | Gap |
|-----------|------|-------|-----|
| `CreateMemoComment` parent lookup | `memo_service.go` | 501, 521 | No tenant filter |
| `ListMemoComments` parent lookup | `memo_service.go` | 580, 598-602, 609-611 | No tenant filter |
| `handleTicketAIResponse` memo lookups | `memo_service.go` | 1016, 1024-1027, 1031, 1039, 1097-1100 | No tenant filter |
| `DeleteMemo` cascade | `memo_service.go` | 475-485 | Uses tenant-unaware `ListMemoRelations` |
| `ListMemoRelations` SQL | `memo_relation.go` | 40-101 | No `tenant_id` WHERE clause |

### Schema Migration

**File:** `store/migration/sqlite/XX__add_tenant_to_memo_relation.sql`

```sql
ALTER TABLE memo_relation ADD COLUMN tenant_id INTEGER DEFAULT NULL;

UPDATE memo_relation mr
SET tenant_id = (
    SELECT m.tenant_id FROM memo m WHERE m.id = mr.memo_id
);

CREATE INDEX IF NOT EXISTS idx_memo_relation_tenant ON memo_relation(tenant_id);
```

### Store Layer Changes

**File:** `store/memo_relation.go`

1. Add `TenantID *int32` to `FindMemoRelation` struct (line 22-27)
2. Add `TenantID` field to `MemoRelation` struct (line 16-20) for read-back

**File:** `store/db/sqlite/memo_relation.go`

1. Update `UpsertMemoRelation` (lines 12-38) to include `tenant_id` column
2. Update `ListMemoRelations` (lines 40-101) to add `WHERE tenant_id = ?` when provided
3. Add defense-in-depth JOIN: even with `tenant_id` column, JOIN on parent memo's `tenant_id`

### Handler Layer Changes

**File:** `server/router/api/v1/memo_service.go`

1. **`CreateMemoComment`** (lines 496-573):
   - Line 501: Add `TenantID` to `FindMemo` when fetching `relatedMemo`
   - Line 521: Add `TenantID` to `FindMemo` when fetching `memo`
   - Pass `TenantID` to `UpsertMemoRelation` for the new relation

2. **`ListMemoComments`** (lines 575-628):
   - Line 580: Add `TenantID` to `FindMemo` when fetching parent memo
   - Lines 598-602: Add `TenantID` to `FindMemoRelation` for comment listing
   - Lines 609-611: Add `TenantID` to `FindMemo` for each comment memo fetch

3. **`handleTicketAIResponse`** (lines 1006-1184):
   - Line 1016: Add `TenantID` to memo lookup
   - Lines 1024-1027: Add `TenantID` to `ListMemoRelations`
   - Line 1031: Add `TenantID` to parent memo lookup
   - Lines 1097-1100: Add `TenantID` to comment listing
   - Fix fallback logic (lines 1059-1061): Don't default to first tenant — use ticket's tenant

4. **`DeleteMemo` cascade** (lines 475-485):
   - Add `TenantID` to `ListMemoRelations` call for comment cascade deletion

### Defense-in-Depth

Even after adding `tenant_id` to `memo_relation`, keep JOIN-based filtering as safety net:

```sql
SELECT mr.* FROM memo_relation mr
JOIN memo m ON m.id = mr.memo_id
WHERE mr.memo_id = ? AND m.tenant_id = ?
```

---

## Issue #1: Wildcard CORS

### Problem

`v1.go:139-144` sets `AllowOrigins: ["*"]` globally, applying to admin APIs. gRPC-web proxy at `v1.go:167-170` also allows all origins.

### Fix

**File:** `server/router/api/v1/v1.go`

1. **Remove global CORS middleware** (lines 139-144)

2. **Create two CORS configs:**
   ```go
   // Public CORS - permissive for widget/chat from any origin
   publicCORS := middleware.CORSWithConfig(middleware.CORSConfig{
       AllowOrigins: []string{"*"},
       AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
       AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
   })

   // Admin CORS - restrictive, configurable via env
   adminOrigins := getEnvSlice("ADMIN_CORS_ORIGINS", []string{}) // empty = deny all
   adminCORS := middleware.CORSWithConfig(middleware.CORSConfig{
       AllowOrigins: adminOrigins,
       AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
       AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
   })
   ```

3. **Apply per-group:**
   - `publicGroup` (line 192): `publicCORS`
   - `widgetGroup` (line 207): `publicCORS`
   - `authRESTGroup` (line 157): `adminCORS`
   - `authGroup` (line 212): `adminCORS`
   - `adminGroup` (line 267): `adminCORS`
   - `userGroup` (line 261): `adminCORS`
   - `ragGroup` (line 324): `adminCORS`
   - `ticketGroup` (line 148): `adminCORS`

4. **Fix gRPC-web proxy** (lines 167-170):
   ```go
   grpcweb.WithOriginFunc(func(origin string) bool {
       return isOriginAllowed(origin, adminOrigins)
   })
   ```

**Env var:** `ADMIN_CORS_ORIGINS=https://admin.example.com,https://dev.example.com`
**Default:** Empty (deny all cross-origin admin access)

---

## Issue #2: No Message Length Limit

### Problem

`handlers.go:434-436` only checks `req.Message == ""`. `service.go:1520-1522` only validates `client_message_id` length. No upper bound on message size.

### Fix

**File:** `store/agent.go`

1. Add `MaxMessageLength int` to `AgentAudience` struct (around line 32-60)

**File:** `server/router/api/v1/agent/service.go`

1. Add validation in `ChatExternal` after session normalization (around line 1519):
   ```go
   maxLen := config.Audience.MaxMessageLength
   if maxLen <= 0 {
       maxLen = 4000 // default
   }
   if len(req.Message) > maxLen {
       return nil, fmt.Errorf("message exceeds maximum length of %d characters", maxLen)
   }
   ```

**File:** `server/router/api/v1/agent/handlers.go`

1. The error from `ChatExternal` will propagate to `HandleChatExternal` (line 440) and return as 500 currently. Update error handling to detect the length error and return 400:
   ```go
   if strings.Contains(err.Error(), "exceeds maximum length") {
       return echo.NewHTTPError(http.StatusBadRequest, err.Error())
   }
   ```

---

## Issue #3: Prompt Injection

### Problem

User messages pass directly into LLM with zero filtering. Output sanitizer exists (`sanitizer.go`) but nothing on input.

### Fix

**File:** `server/router/api/v1/agent/service.go`

1. **Add `SanitizeUserInput` function:**
   ```go
   func SanitizeUserInput(message string) string {
       // Strip control characters (keep \t \n \r)
       re := regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F]`)
       message = re.ReplaceAllString(message, "")

       // Collapse 3+ consecutive newlines to 2
       re2 := regexp.MustCompile(`\n{3,}`)
       message = re2.ReplaceAllString(message, "\n\n")

       return strings.TrimSpace(message)
   }
   ```

2. **Apply in `processChat`** at line 1773 before adding to session:
   ```go
   userMessage = SanitizeUserInput(userMessage)
   ```

3. **Add injection pattern logging** (warning only, not block):
   ```go
   injectionPatterns := []string{"ignore previous instructions", "you are now", "system prompt:"}
   lower := strings.ToLower(userMessage)
   for _, pattern := range injectionPatterns {
       if strings.Contains(lower, pattern) {
           slog.Warn("potential prompt injection detected", "pattern", pattern, "slug", config.Slug)
           break
       }
   }
   ```

4. **Add per-tenant LLM verifier toggle** to `AgentAudience`:
   - Field: `LLMVerifierEnabled bool`
   - In `processChat`, if enabled, call existing `verifier.go` before sending to LLM

---

## Issue #4: Never-Expire Tokens

### Problem

`auth_service.go:161-166` caps "never expire" at 30 days. Tokens that long are a security risk.

### Fix

**File:** `server/router/api/v1/auth_service.go`

1. At lines 161-166, always use standard expiry:
   ```go
   expireTime := time.Now().Add(AccessTokenDuration) // 7 days
   if request.NeverExpire {
       slog.Warn("never_expire is deprecated, using standard 7-day expiry", "user", user.Username)
   }
   ```

2. Keep proto field `never_expire` for backward compatibility (ignored in code)

3. All existing 30-day tokens will naturally expire on next validation cycle

---

## Issue #5: gRPC Insecure Credentials

### Problem

`v1.go:90-93` uses `insecure.NewCredentials()` for the internal gRPC connection.

### Fix (Accept Risk)

**File:** `server/router/api/v1/v1.go`

1. Add comment documenting the acceptance:
   ```go
   // SECURITY NOTE: Using insecure credentials for the internal gRPC connection.
   // This is acceptable because:
   // - Deployment is always single-container (gRPC traffic never leaves process boundary)
   // - The connection is loopback only (127.0.0.1)
   // - If multi-container deployment is needed in the future, add TLS here
   conn, err := grpc.NewClient(
       target,
       grpc.WithTransportCredentials(insecure.NewCredentials()),
   ```

2. Add note in `docs/DOCS_SECURITY.MD` or equivalent documenting this as accepted risk

---

## Issue #6: Public Transcript Endpoint

### Problem

`GET /api/v1/agent/:slug/chat/ext/transcript` is public, only needs `session_id`. Anyone who knows a UUID can retrieve the full conversation.

### Fix

**Files:** `server/router/api/v1/agent/handlers.go`, `server/router/api/v1/agent/service.go`

1. **Generate HMAC-signed token** when chat session is created in `ChatExternal`:
   ```go
   // After session is created
   expiry := time.Now().Add(30 * time.Minute)
   tokenMAC := hmac.New(sha256.New, []byte(tenant.GUID))
   tokenMAC.Write([]byte(sessionID + expiry.Format(time.RFC3339)))
   sessionToken := hex.EncodeToString(tokenMAC.Sum(nil))
   ```

2. **Return `session_token` in `ChatResponse`** struct

3. **Transcript endpoint verification** in `HandleGetExternalTranscript`:
   ```go
   token := c.QueryParam("token")
   if token == "" {
       return echo.NewHTTPError(http.StatusBadRequest, "token is required")
   }

   // Verify HMAC
   expiryTime, err := verifySessionToken(token, sessionID, tenant.GUID)
   if err != nil || time.Now().After(expiryTime) {
       return echo.NewHTTPError(http.StatusForbidden, "invalid or expired token")
   }
   ```

4. Token expires with session (30-min TTL)

---

## Issue #7: Public Playground Endpoints

### Problem

`playground/catalog` and `playground/run` are public with no auth. Catalog auto-provisions demos on every GET.

### Fix (Keep Public)

**File:** `server/router/api/v1/agent/playground.go`

1. **Move `ensurePlaygroundDemo` to startup task** (lines 448-453):
   - Remove from `HandlePlaygroundCatalog` — don't auto-provision on every GET
   - Add startup goroutine that provisions demos once at boot

2. **Rate limiting** already applies via `ChatExternal` → `CheckRateLimit` (the playground calls `ChatExternal`)

3. **Message length limit** from Issue #2 applies via `ChatExternal`

4. No auth required — intentionally public for demo/marketing

---

## Issue #8: Tenant Slug Enumeration

### Problem

`handlers.go:406-409` returns 404 "Agent not found" for nonexistent tenants, 200 for active ones. Domain check at line 414-416 also leaks existence.

### Fix

**File:** `server/router/api/v1/agent/handlers.go`

1. **Change `HandleChatExternal`** (lines 406-409):
   ```go
   tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
   if err != nil || tenant == nil || !tenant.IsActive {
       slog.Info("chat external: tenant not found or inactive", "slug", slug)
       return echo.NewHTTPError(http.StatusForbidden, "Access denied")
   }
   ```

2. **Change domain allowlist error** (lines 414-416):
   ```go
   if tenant.AllowedDomains != "" {
       origin := c.Request().Header.Get("Origin")
       if !h.isDomainAllowed(tenant.AllowedDomains, origin, "") {
           slog.Info("chat external: domain not allowed", "slug", slug, "origin", origin)
           return echo.NewHTTPError(http.StatusForbidden, "Access denied")
       }
   }
   ```

3. **Same pattern for `HandleGetExternalTranscript`** (lines 489-492)

4. Log actual reason server-side for debugging

---

## Issue #9: No Rate Limiting on Internal/Login

### Problem

Rate limiting only exists on external chat and admin mutations. Login and internal chat have no rate limiting.

### Fix

**File:** `server/router/api/v1/auth_service.go`

1. **Login endpoints** (`HandleAuthTenants`, `HandleSelectTenant`):
   - Add rate limit check: 5 attempts per IP per minute
   - Use existing `CheckRateLimit` with `"login"` audience type
   - Client IP from request (no tenant context available at this point)

   ```go
   // In HandleAuthTenants, before processing
   clientIP := c.RealIP()
   allowed, err := s.CheckRateLimit(ctx, 0, "login", clientIP, 5) // tenantID=0 for login
   if !allowed {
       return echo.NewHTTPError(http.StatusTooManyRequests, "Too many login attempts. Please try again in 60 seconds.")
   }
   ```

**File:** `server/router/api/v1/agent/handlers.go`

2. **Internal chat** (`HandleChatInternal`):
   - Add rate limit check: 30 RPM per user
   - Use existing `CheckRateLimit` with `"internal"` audience type

---

## Issue #10: ENCRYPTION_MASTER_KEY

### Problem

`encryption.go:25-34` derives AES key from single env var. If weak or lost, all tenant API keys are compromised/unrecoverable.

### Fix

**File:** `bin/memos/main.go`

1. **Add startup validation:**
   ```go
   if key := os.Getenv("ENCRYPTION_MASTER_KEY"); key == "" || len(key) < 16 {
       slog.Warn("ENCRYPTION_MASTER_KEY is empty or too short (< 16 chars). Encrypted tenant API keys may be insecure.")
   }
   ```

**File:** `internal/crypto/encryption.go`

2. **Add backup key support:**
   ```go
   func NewEncryptionService(masterPassword string, salt []byte) *EncryptionService {
       // ... existing key derivation ...

       // Try backup key if primary fails
       var backupKey []byte
       if backup := os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP"); backup != "" {
           backupKey = argon2.IDKey([]byte(backup), salt, 1, 64*1024, 4, KeySize)
       }

       return &EncryptionService{key: key, backupKey: backupKey}
   }
   ```

3. **Document key management** in `docs/`

---

## Issue #11: Cross-Tenant Audit

### Problem

QA pair deletion was fixed, but the same pattern may exist in other tenant-scoped resources.

### Fix

**Files:** `server/router/api/v1/agent/handlers.go`, `store/db/sqlite/agent.go`

1. **Grep all DELETE handlers** for tenant ownership verification:
   - `DeleteAgentTranscript`
   - `DeleteAgentSession`
   - Any other tenant-scoped DELETE operations

2. **Audit SQL queries** for `WHERE id = ?` without `AND tenant_id = ?`

3. **Apply compound-check pattern** where missing:
   ```go
   // Before delete
   item, err := h.store.GetThing(ctx, &store.FindThing{ID: &id})
   if tenantID != nil && item.TenantID != nil && *item.TenantID != *tenantID && !isSuperUser(user) {
       return echo.NewHTTPError(http.StatusForbidden, "permission denied")
   }
   ```

4. **Fix `DeleteMemo` cascade** (lines 475-485):
   - Add tenant filter to `ListMemoRelations` call
   - Verify each comment before deletion

---

## Issue #12: Admin No Tenant Isolation

### Problem

`adminGroup` uses `AuthMiddleware` but no tenant-scoping. Any ADMIN can access all tenants.

### Fix

**File:** `store/migration/sqlite/XX__add_allowed_tenant_ids_to_user.sql`

```sql
ALTER TABLE user ADD COLUMN allowed_tenant_ids TEXT DEFAULT NULL;
```

**File:** `store/agent.go`

1. Add `AllowedTenantIDs []string` to user store struct

**File:** `store/db/sqlite/agent.go`

1. Add migration handling for new column

**File:** `server/router/api/v1/v1.go`

1. Add middleware for admin routes:
   ```go
   func TenantBindingMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
       return func(c echo.Context) error {
           user := getUserFromContext(c)
           if isSuperUser(user) {
               return next(c) // superusers bypass
           }
           slug := c.Param("slug")
           if slug == "" {
               return next(c) // no slug in URL, skip
           }
           // Check if user has access to this tenant
           tenant, _ := store.GetAgentTenantBySlug(slug)
           if user.AllowedTenantIDs != nil && !contains(user.AllowedTenantIDs, tenant.GUID) {
               return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
           }
           return next(c)
       }
   }
   ```

2. Apply to `adminGroup`

---

## Issue #13: Domain Allowlist Bypassable

### Problem

`AllowedDomains == "[]"` currently allows all (empty check). Origin header is spoofable by non-browser clients.

### Fix

**File:** `server/router/api/v1/agent/handlers.go`

1. **Fix empty array behavior** at lines 411-417:
   ```go
   if tenant.AllowedDomains != "" {
       origin := c.Request().Header.Get("Origin")
       if !h.isDomainAllowed(tenant.AllowedDomains, origin, "") {
           return echo.NewHTTPError(http.StatusForbidden, "Access denied")
       }
   }
   ```

2. **Update `isDomainAllowed`** to treat `"[]"` as deny-all (currently allows all when empty array)

3. **Document** that domain allowlist is for embedding protection (widget), not API security — API security comes from auth and HMAC tokens (Issue #6)

---

## Implementation Order

| Phase | Issues | Effort | Description |
|-------|--------|--------|-------------|
| **Phase 1: Critical Isolation** | #14 (memo comments), #11 (audit) | 2-3 days | Structural tenant isolation fixes |
| **Phase 2: Auth Hardening** | #4 (never-expire), #6 (HMAC tokens), #9 (rate limits) | 2-3 days | Authentication and session security |
| **Phase 3: Input Validation** | #2 (message length), #3 (input filtering), #8 (slug enum) | 1-2 days | Input sanitization and enumeration prevention |
| **Phase 4: Network Security** | #1 (CORS), #13 (domain allowlist) | 1 day | CORS and domain restrictions |
| **Phase 5: Infrastructure** | #5 (gRPC docs), #10 (encryption key), #12 (admin binding) | 1-2 days | Documentation and operational safeguards |

**Total estimated effort: 7-11 days**

---

## Files Modified (Summary)

| File | Issues | Changes |
|------|--------|---------|
| `store/migration/sqlite/` | #14, #12 | New migration files |
| `store/memo_relation.go` | #14 | Add TenantID field |
| `store/db/sqlite/memo_relation.go` | #14 | Add tenant_id to SQL queries |
| `server/router/api/v1/memo_service.go` | #14 | Add tenant filtering to comment operations |
| `server/router/api/v1/v1.go` | #1, #5, #12 | CORS config, gRPC comment, tenant binding |
| `server/router/api/v1/agent/handlers.go` | #2, #6, #8, #9, #13 | Message length, HMAC tokens, slug enum, rate limits, domain fix |
| `server/router/api/v1/agent/service.go` | #2, #3 | Message validation, input sanitization |
| `server/router/api/v1/agent/playground.go` | #7 | Move demo provisioning to startup |
| `server/router/api/v1/auth_service.go` | #4, #9 | Never-expire removal, login rate limiting |
| `internal/crypto/encryption.go` | #10 | Backup key support |
| `bin/memos/main.go` | #10 | Startup validation |
| `store/agent.go` | #2, #3, #12 | Config fields |
| `docs/` | #5, #10 | Security documentation |

---

## Sign-Off

- [ ] Plan reviewed and approved
- [ ] Ready to begin Phase 1 implementation
