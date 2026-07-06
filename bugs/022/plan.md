# Security Fix Plan — bugs/022

**Date:** 2026-07-06
**Source:** DeepWiki Q&A security analysis + codebase investigation
**Status:** Plan — awaiting sign-off before implementation

---

## Table of Contents

1. [Issue Summary](#issue-summary)
2. [Q&A Decisions](#qa-decisions)
3. [Detailed Fix Plans](#detailed-fix-plans)
4. [Memo Comment Isolation (NEW)](#memo-comment-isolation-new)
5. [Implementation Order](#implementation-order)

---

## Issue Summary

13 issues identified from DeepWiki security analysis, plus 1 new finding (memo comments).

| # | Issue | Severity | Category |
|---|-------|----------|----------|
| 1 | Wildcard CORS on all routes including admin APIs | High | Network |
| 2 | No message length limit on external chat | High | Input Validation |
| 3 | Prompt injection — no input sanitization | High | Input Validation |
| 4 | Never-expire access tokens | High | Auth |
| 5 | gRPC internal connection uses insecure credentials | Medium | Infrastructure |
| 6 | Public transcript endpoint leaks conversation history | Medium | Auth |
| 7 | Public playground endpoints expose tenant data | Medium | Auth |
| 8 | Tenant slug enumeration via 404 vs 200 | Medium | Network |
| 9 | No rate limiting on internal chat, playground, login | Medium | Network |
| 10 | ENCRYPTION_MASTER_KEY single point of failure | Medium | Infrastructure |
| 11 | Cross-tenant QA pair deletion (FIXED — audit others) | Low | Isolation |
| 12 | Admin role has no tenant isolation | Low | Auth |
| 13 | Domain allowlist is opt-in and bypassable | Low | Network |
| **NEW** | **Memo comments are NOT tenant-isolated** | **Critical** | **Isolation** |

### Memo Comment Isolation — Investigation Finding

The `memo_relation` table (backbone of comments) has **no `tenant_id` column**. Key gaps:

| Operation | Tenant Filtered? | File |
|-----------|-------------------|------|
| `CreateMemoComment` parent lookup | **NO** | `memo_service.go:501` |
| `ListMemoComments` parent lookup | **NO** | `memo_service.go:580` |
| `handleTicketAIResponse` memo lookups | **NO** | `memo_service.go:1016,1031,1097` |
| `ListMemoRelations` SQL query | **NO** | `memo_relation.go:40-101` |

This is a **structural gap** — comments inherit `tenant_id` on creation but aren't filtered on read. Cross-tenant comment read/write is possible.

---

## Q&A Decisions

The following questions were asked and answered to determine the fix strategy:

### Q1: CORS Strategy

**Question:** For the CORS issue (#1), the wildcard applies to ALL routes including admin APIs. Should we implement route-level CORS (strict for admin, permissive for public chat/widget), or is the admin API intended to be called from a specific frontend origin?

| Option | Description |
|--------|-------------|
| **Route-level CORS (Chosen)** | Strict CORS for admin APIs (specific origins), permissive for public chat endpoints |
| Keep wildcard, rely on cookies | Keep wildcard CORS but add CSRF tokens for admin state-changing operations |
| Both | Route-level CORS + CSRF tokens as defense-in-depth |

### Q2: Comment Isolation Approach

**Question:** For memo comment isolation (#11 + NEW finding), the fix requires: (a) adding tenant_id to memo_relation table, (b) filtering all comment queries by tenant. Should we also add tenant_id filtering to the existing ListMemoRelations queries as a defense-in-depth?

| Option | Description |
|--------|-------------|
| **Add tenant_id to memo_relation + filter everywhere (Chosen)** | Full structural fix: schema migration + all queries filtered |
| Filter at query time only | No schema change, just JOIN on parent memo's tenant_id in queries |
| Both approaches | Add column for explicit isolation + JOIN-based filtering as belt-and-suspenders |

### Q3: Public Endpoint Authentication

**Question:** For public endpoints (#6 transcript, #7 playground), should we add lightweight auth (e.g., HMAC-signed session tokens) or keep them fully public but add rate limiting + input constraints?

| Option | Description |
|--------|-------------|
| **Add HMAC session tokens (Chosen)** | Session tokens signed with tenant secret, expires with session |
| Keep public + rate limit | Add rate limiting and message constraints, keep fully unauthenticated |
| Require domain allowlist | Make domain allowlist mandatory for public endpoints |

### Q4: Prompt Injection Defense

**Question:** For prompt injection (#3), what level of input sanitization do you want? This is a balance between security and not breaking legitimate user messages.

| Option | Description |
|--------|-------------|
| **Length limit + basic filtering (Chosen)** | Max message length + strip control chars + detect obvious injection patterns |
| LLM-based classifier | Use a lightweight LLM call to classify if message is an injection attempt |
| Both + enable verifier | Basic filtering + optional LLM verifier (currently disabled by default) |

### Q5: Never-Expire Tokens

**Question:** For the never-expire tokens (#4), should we remove the feature entirely or rebrand it as 'long-lived' with a shorter max duration?

| Option | Description |
|--------|-------------|
| **Remove feature (Chosen)** | Remove never_expire option, all tokens use standard 7-day expiry |
| Rebrand to 'long-lived' | Rename to avoid confusion, cap at 7 days instead of 30 |
| Keep as-is with warning | Keep 30-day cap but add UI warning about security implications |

### Q6: Scope

**Question:** Should we fix all 13 issues in one plan, or prioritize the critical/high ones first?

| Option | Description |
|--------|-------------|
| Critical + High first | Fix #1-9 (cross-tenant, auth, input validation), defer #10-13 |
| All 13 at once | Comprehensive fix for every issue in one plan |
| **All 13 at once (Chosen)** | Complete fix for all issues including memo comment isolation |

---

## Detailed Fix Plans

### Issue #1: Wildcard CORS on All Routes

**Root Cause:** `v1.go:140-144` sets `AllowOrigins: ["*"]` globally, applying to admin APIs.

**Fix:** Route-level CORS configuration.

**Implementation:**
1. Remove global CORS middleware
2. Create two CORS configs:
   - **Public CORS** (for `/api/v1/agent` public group): `AllowOrigins: ["*"]` — keeps widget/chat working from any origin
   - **Admin/Auth CORS** (for admin/auth groups): `AllowOrigins: ["https://your-admin-domain.com"]` — configurable via env var `ADMIN_CORS_ORIGINS`
3. Apply per-group CORS middleware in `v1.go`
4. gRPC-web proxy: replace `WithOriginFunc(func(_ string) bool { return true })` with a domain check

**Env var:** `ADMIN_CORS_ORIGINS=https://admin.example.com,https://dev.example.com`

**Files:** `server/router/api/v1/v1.go`

---

### Issue #2: No Message Length Limit on External Chat

**Root Cause:** `handlers.go:434-436` only checks `req.Message == ""` — no upper bound.

**Fix:** Add configurable max message length.

**Implementation:**
1. Add `MaxMessageLength` field to `TenantConfig.Audience` struct (default: 4000 chars)
2. Add validation in `ChatExternal` service method: `len(req.Message) > maxLen` → 400 error
3. Apply same limit to `PlaygroundRun` (it delegates to `ChatExternal`)
4. Return clear error message: `"Message exceeds maximum length of %d characters"`

**Files:** `server/router/api/v1/agent/handlers.go`, `server/router/api/v1/agent/service.go`, `store/agent.go`

---

### Issue #3: Prompt Injection (No Input Sanitization)

**Root Cause:** User messages pass directly into LLM with zero filtering. Output sanitization exists (`sanitizer.go`) but nothing on input.

**Fix:** Length limit + basic filtering + optional detector.

**Implementation:**
1. **Strip control characters**: Remove `\x00-\x08\x0B\x0C\x0E-\x1F` (keep `\t\n\r`)
2. **Normalize whitespace**: Collapse 3+ consecutive newlines to 2
3. **Basic injection detection**: Log warnings (not block) for obvious patterns like `"ignore previous instructions"`, `"you are now"`, `"system prompt:"` — these are heuristics, not hard blocks
4. **Message length limit** (from Issue #2) is the primary cost-amplification defense
5. **Enable LLM verifier as opt-in per-tenant** (already exists, currently disabled by default) — add a tenant config toggle `LLMVerifierEnabled`

**Not implementing:** Full LLM-based classifier (too expensive for every message). The existing `verifier.go` is the right tool for tenants that want it.

**Files:** `server/router/api/v1/agent/service.go` (new `SanitizeInput` function), `store/agent.go` (new config field)

---

### Issue #4: Never-Expire Access Tokens

**Root Cause:** `auth_service.proto` has `never_expire` field, `auth_service.go:161-166` caps at 30 days.

**Fix:** Remove the feature entirely.

**Implementation:**
1. Deprecate `never_expire` field in proto (keep for backward compat, ignore in code)
2. In `auth_service.go`, always use `AccessTokenDuration` (7 days) regardless of `NeverExpire` flag
3. Add a log warning when `NeverExpire` is true: `"never_expire is deprecated, using standard 7-day expiry"`
4. Frontend: remove any "remember me" or "keep me signed in" checkbox that sets `never_expire`
5. All existing 30-day tokens will naturally expire on next validation cycle

**Files:** `server/router/api/v1/auth_service.go`, `web/src/` (if checkbox exists)

---

### Issue #5: gRPC Internal Connection Uses Insecure Credentials

**Root Cause:** `v1.go:90-93` uses `insecure.NewCredentials()` for localhost gRPC connection.

**Fix:** Use TLS for the internal connection (loopback TLS).

**Implementation:**
1. Generate a self-signed certificate at startup (or use existing `ENCRYPTION_MASTER_KEY` to derive one)
2. Store cert in `system_secret` table or filesystem
3. Replace `insecure.NewCredentials()` with TLS credentials
4. Since it's loopback, the cert doesn't need to be CA-signed — self-signed is sufficient
5. Alternative (simpler): Ensure gRPC binds to `127.0.0.1` only (not `0.0.0.0`) — verify current binding

**Note:** If deployment is single-container (typical), the loopback risk is minimal. Consider skipping if deployment is always single-container.

**Files:** `server/router/api/v1/v1.go`, potentially new cert generation utility

---

### Issue #6: Public Transcript Endpoint Leaks Conversation History

**Root Cause:** `GET /api/v1/agent/:slug/chat/ext/transcript` is public, only needs `session_id`.

**Fix:** HMAC-signed session tokens (paired with Issue #7).

**Implementation:**
1. When a chat session is created, generate an HMAC-signed token: `HMAC-SHA256(tenant_secret, session_id + expiry)`
2. Return the token to the widget in the initial chat response
3. Transcript endpoint requires `?token=<hmac>&session_id=<id>` — verify HMAC and expiry
4. Token expires with the session (30-minute in-memory TTL)
5. This prevents transcript access by anyone who just knows/guesses a UUID

**Files:** `server/router/api/v1/agent/handlers.go`, `server/router/api/v1/agent/service.go`

---

### Issue #7: Public Playground Endpoints Expose Tenant Data

**Root Cause:** `playground/catalog` and `playground/run` are public, no auth.

**Fix:** Two approaches:
- **Catalog**: Keep public (marketing content for demos), but remove auto-provisioning side effect
- **Playground run**: Require admin auth

**Implementation:**
1. **Catalog**: Move `ensurePlaygroundDemo` to a startup task or admin endpoint — don't auto-provision on GET
2. **Playground run**: Move to `authGroup` (require authentication) since it invokes the full LLM pipeline
3. Alternatively, keep public but apply rate limiting (from Issue #9) and the message length limit (Issue #2)

**Files:** `server/router/api/v1/v1.go` (route registration), `server/router/api/v1/agent/playground.go`

---

### Issue #8: Tenant Slug Enumeration

**Root Cause:** `handlers.go:406-409` returns 404 for nonexistent tenants, 200 for active ones.

**Fix:** Return generic response regardless of tenant existence.

**Implementation:**
1. In `HandleChatExternal`, always return the same error format for both "not found" and "inactive" cases
2. Return `403 Forbidden` with generic message `"Access denied"` instead of `404 "Agent not found"` for nonexistent/inactive slugs
3. For valid tenants, the domain allowlist check already gates access
4. Log the actual reason server-side for debugging

**Tradeoff:** Breaks legitimate debugging (e.g., "why can't I reach my agent?"). Server-side logging mitigates.

**Files:** `server/router/api/v1/agent/handlers.go`

---

### Issue #9: No Rate Limiting on Internal Chat, Playground, Login

**Root Cause:** Rate limiting only exists on external chat and admin mutations.

**Fix:** Add rate limiting middleware and per-endpoint limits.

**Implementation:**
1. **Login endpoints** (`/auth/tenants`, `/auth/select-tenant`): 5 attempts per IP per minute
2. **Internal chat** (`/chat/int`): 30 RPM per user (authenticated users are trusted but not unlimited)
3. **Playground run**: Same rate limit as external chat (per-IP, per-tenant)
4. Reuse existing `CheckRateLimit` function from `service.go`, add new rate limit categories

**Files:** `server/router/api/v1/auth_service.go`, `server/router/api/v1/agent/handlers.go`

---

### Issue #10: ENCRYPTION_MASTER_KEY Single Point of Failure

**Root Cause:** `encryption.go:25-34` derives AES key from single env var via Argon2id.

**Fix:** Operational safeguards (design limitation, not a bug to "fix").

**Implementation:**
1. Add startup validation: if `ENCRYPTION_MASTER_KEY` is empty or < 16 chars, log fatal warning
2. Add key rotation documentation and tool (re-encrypt all `tenant_config` rows with new key)
3. Add `ENCRYPTION_MASTER_KEY_BACKUP` env var — if primary key fails decryption, try backup
4. Document key management best practices (HSM, vault, etc.)

**Files:** `internal/crypto/encryption.go`, `bin/memos/main.go`, `docs/`

---

### Issue #11: Cross-Tenant QA Pair Deletion (FIXED — Audit Other Resources)

**Root Cause:** Already fixed for QA pairs, but the same pattern may exist elsewhere.

**Fix:** Audit all tenant-scoped resources for the same vulnerability.

**Implementation:**
1. Grep for all `Delete` handlers that take an ID without tenant verification
2. Grep for all SQL `DELETE` and `UPDATE` queries that use `WHERE id = ?` without `AND tenant_id = ?`
3. Apply the same compound-check pattern from the QA pair fix to:
   - `DeleteMemo` (already has post-fetch check — verify it's enforced)
   - `DeleteAgentTranscript`
   - `DeleteAgentSession`
   - Any other tenant-scoped DELETE operations

**Files:** `server/router/api/v1/agent/handlers.go`, `store/db/sqlite/agent.go`

---

### Issue #12: Admin Role Has No Tenant Isolation

**Root Cause:** `adminGroup` uses `AuthMiddleware` but no tenant-scoping. `isSuperUser` check is in handlers, not route-level.

**Fix:** Add optional tenant-binding for admin users.

**Implementation:**
1. Add `allowed_tenant_ids` column to `user` table (JSON array, nullable — null means all tenants)
2. Add middleware for admin routes that checks `user.AllowedTenantIDs` against the `:slug` in the URL
3. Super users (empty array or null) bypass this check
4. Progressive enhancement — existing behavior preserved for super users

**Migration:** `ALTER TABLE user ADD COLUMN allowed_tenant_ids TEXT DEFAULT NULL;`

**Files:** `store/migration/sqlite/`, `store/db/sqlite/`, `server/router/api/v1/v1.go`

---

### Issue #13: Domain Allowlist Is Opt-In and Bypassable

**Root Cause:** `Origin` header is browser-enforced but spoofable by non-browser clients. Empty `AllowedDomains` means allow all.

**Fix:** Complementary defenses (design limitation).

**Implementation:**
1. Fix the empty array behavior: `AllowedDomains == "[]"` should deny all (currently allows all)
2. Add HMAC-signed widget tokens (same as Issue #6) — the widget JS gets a signed token on load, all subsequent requests include it
3. Document that domain allowlist is for embedding protection, not API security — API security comes from auth

**Files:** `server/router/api/v1/agent/handlers.go`

---

## Memo Comment Isolation (NEW — Structural Fix)

**Root Cause:** `memo_relation` table has no `tenant_id`. Comment handlers don't pass `TenantID` in `FindMemo` lookups.

**Fix:** Add `tenant_id` to `memo_relation` + filter all queries.

### Schema Migration

```sql
-- store/migration/sqlite/0.25/XX__add_tenant_to_memo_relation.sql
ALTER TABLE memo_relation ADD COLUMN tenant_id INTEGER DEFAULT NULL;

UPDATE memo_relation mr
SET tenant_id = (
    SELECT m.tenant_id FROM memo m WHERE m.id = mr.memo_id
);

CREATE INDEX IF NOT EXISTS idx_memo_relation_tenant ON memo_relation(tenant_id);
```

### Store Layer Changes

1. Add `TenantID *int32` to `FindMemoRelation` struct in `store/memo_relation.go`
2. Update `ListMemoRelations` SQL to include `WHERE tenant_id = ?` when provided
3. Add `ApplyMemoRelationTenantFilter` helper function

### Handler Layer Changes

1. **`CreateMemoComment`** (`memo_service.go:496-573`): Add `TenantID` to parent memo lookup
2. **`ListMemoComments`** (`memo_service.go:575-628`): Add `TenantID` to parent memo lookup and each comment fetch
3. **`handleTicketAIResponse`** (`memo_service.go:1006-1184`): Add `TenantID` to all memo lookups
4. **`DeleteMemo` cascade** (`memo_service.go:475-485`): Add tenant filter when fetching comment relations

### Defense-in-Depth

Even after adding `tenant_id` to `memo_relation`, keep the JOIN-based filtering as a safety net:

```sql
SELECT mr.* FROM memo_relation mr
JOIN memo m ON m.id = mr.memo_id
WHERE mr.memo_id = ? AND m.tenant_id = ?
```

**Files:** `store/migration/sqlite/`, `store/memo_relation.go`, `store/db/sqlite/memo_relation.go`, `server/router/api/v1/memo_service.go`

---

## Implementation Order

| Phase | Issues | Effort |
|-------|--------|--------|
| **Phase 1: Critical Isolation** | #11 audit, Memo comments | 2-3 days |
| **Phase 2: Auth Hardening** | #4 (remove never-expire), #6+7 (HMAC tokens), #9 (rate limits) | 2-3 days |
| **Phase 3: Input Validation** | #2 (message length), #3 (input filtering), #8 (slug enumeration) | 1-2 days |
| **Phase 4: Network Security** | #1 (CORS), #13 (domain allowlist fix) | 1 day |
| **Phase 5: Infrastructure** | #5 (gRPC TLS), #10 (encryption key safeguards), #12 (admin tenant-binding) | 1-2 days |

**Total estimated effort: 7-11 days**

---

## Sign-Off

- [ ] Plan reviewed and approved
- [ ] Ready to begin Phase 1 implementation
