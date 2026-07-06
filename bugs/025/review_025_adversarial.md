# Adversarial Code Review — plan_025 Implementation

**Reviewer:** Go Security Engineer (adversarial)
**Date:** 2026-07-07
**Target:** plan_025 security hardening implementation
**Files reviewed:** store/agent.go, store/driver.go, store/db/sqlite/agent.go, store/db/postgres/agent.go, store/db/mysql/agent.go, store/migration/sqlite/LATEST.sql, server/server.go, server/router/api/v1/v1.go, server/router/api/v1/agent/handlers.go, server/router/api/v1/agent/service.go, server/router/api/v1/agent/playground.go

---

## Executive Summary

The plan_025 implementation is structurally sound and addresses the threat model correctly. The layered defense architecture (widget key edge gate + atomic rate limiting + session turn cap + body limit + transcript HMAC rekey) is the right approach. Most security properties are implemented correctly: `subtle.ConstantTimeCompare` for widget keys, `hmac.Equal` for session tokens, atomic `INSERT ... ON CONFLICT DO UPDATE ... RETURNING` for rate limits, and `BodyLimit("16KB")` at the Echo server level.

However, the adversarial review identified **2 High** and **3 Medium** findings that must be resolved before this is considered production-ready. The most critical issue is that an empty `WidgetKey` field causes the widget key check to be **skipped entirely** (fail-open), which immediately exposes the endpoint to unauthenticated use for tenants that have not been backfilled. Additionally, the global `BodyLimit("16KB")` breaks authenticated file upload endpoints.

---

## Verified Threat Model

| # | Threat | Code Reference | Implementation Status |
|---|--------|---------------|----------------------|
| T1 | Public chat endpoint | `handlers.go:404` — `HandleChatExternal`, no auth by design | ✅ Mitigated by widget key + rate limits |
| T2 | Domain allowlist bypass | `handlers.go:428-434` — `isDomainAllowed` runs AFTER widget key | ⚠️ Widget key hardens; allowlist still optional |
| T3 | IP-rotation botnet | `service.go:1594` — global tenant cap (300 RPM) added | ✅ Mitigated |
| T4 | TOCTOU race | `sqlite/agent.go:1286-1321` — atomic upsert with RETURNING | ✅ Fixed |
| T5 | Transcript HMAC on leaked GUID | `service.go:1518`, `handlers.go:519` — derives from `WidgetKey` | ✅ Remediated |
| T6 | Unbounded body/session | `server.go:54` — BodyLimit("16KB"); `service.go:1607` — 50-turn cap | ✅ Mitigated |
| T7 | Fly hard_limit=25 only backstop | `fly.toml:39` — `request_timeout="30s"` | ✅ Mitigated by global tenant cap |

---

## Findings

### FINDING-1: Empty `WidgetKey` Creates Fail-Open Path (High)

**File:** `server/router/api/v1/agent/handlers.go:416-425`  
**Severity:** High

**Description:**

The widget key check is conditional on `tenant.WidgetKey != ""`:

```go
if tenant.WidgetKey != "" {
    // validate X-Widget-Key ...
}
```

If `WidgetKey` is empty (e.g., migration not yet run, or backfill failed), the endpoint skips validation entirely. The comment claims "fail-closed" but the code is **fail-open** for tenants without a widget key.

**Exploit Scenario:**

1. Attacker identifies a tenant whose `widget_key` column is NULL (e.g., fresh tenant, migration in progress).
2. Attacker sends `POST /api/v1/agent/{slug}/chat/ext` without `X-Widget-Key`.
3. Response: `200 OK` with LLM response. No authentication required.

**Impact:** Complete bypass of Layer A (widget key edge gate) for affected tenants.

**Recommended Fix:**

- Option A: Backfill/validate at startup. If a tenant has an empty `WidgetKey`, either auto-generate one or refuse to start the endpoint until configured.
- Option B: Fail-closed. If `WidgetKey` is empty, return 403:
  ```go
  if tenant.WidgetKey == "" {
      slog.Error("chat external: widget key not configured", "slug", slug)
      return echo.NewHTTPError(http.StatusForbidden, "Access denied")
  }
  ```
- Option C: Migrate existing tenants with an auto-generated key on startup, and log a warning.

---

### FINDING-2: Global BodyLimit("16KB") Breaks Authenticated File Uploads (High)

**File:** `server/server.go:54`  
**Severity:** High

**Description:**

Echo's `middleware.BodyLimit("16KB")` is applied at the server level before any route registration. This means it applies to **all** routes, including authenticated admin endpoints like `HandleImportSingleFile` (`handlers.go:1028`) which accepts multipart file uploads for KB/policy files.

**Exploit Scenario:**

1. Tenant admin tries to upload a `KB.MD` or `POLICY.MD` file larger than 16KB (typical for production knowledge bases).
2. Request is rejected with `413 Payload Too Large` before the admin handler ever runs.
3. Admin workflow for updating knowledge bases is broken.

**Impact:** Admin-visible denial of service against file upload and any other authenticated endpoints that need larger bodies (e.g., JSON payloads with embedded documents, bulk operations).

**Recommended Fix:**

- Move `BodyLimit("16KB")` to only the public group routes (`publicGroup.Use(middleware.BodyLimit("16KB"))`) rather than the top-level Echo server.
- Alternatively, exempt authenticated routes that require larger bodies.
- Or increase the limit if the design intent is to protect public endpoints specifically, but this is weak (16KB is a soft ceiling; the real controls are message length + rate limit).

---

### FINDING-3: Playground Endpoint Bypasses Widget Key (Medium)

**File:** `server/router/api/v1/agent/playground.go:521-580`  
**Severity:** Medium

**Description:**

`HandlePlaygroundRun` calls `h.service.ChatExternal(...)` directly at `playground.go:558` without performing the widget key check that exists in `HandleChatExternal` (`handlers.go:416-425`). The playground endpoint is registered on the `publicGroup` (`v1.go:257`).

**Exploit Scenario:**

1. Attacker sends `POST /api/v1/agent/{demo-slug}/playground/run` without any `X-Widget-Key` header.
2. Request bypasses widget key validation entirely.
3. Request still goes through rate limiting (per-IP + global tenant cap), so this is not a full bypass, but it removes the speed-bump that the widget key provides.

**Impact:** Reduces the widget key's effectiveness as a bot deterrent. All playlist demo tenants are exposed to unauthenticated LLM abuse at the rate-limit-only boundary.

**Recommended Fix:**

- Extract widget key validation into a shared helper (e.g., `h.validateWidgetKey(c, slug)`) and call it from both `HandleChatExternal` and `HandlePlaygroundRun`.
- Alternatively, document that the playground is intentionally open and accept the reduced protection for demo tenants only.

---

### FINDING-4: No Grace Period for Widget Key Rotation (Medium)

**File:** `handlers.go:166-173`, `handlers.go:2032-2051`  
**Severity:** Medium

**Description:**

`HandleWidget` sets `Cache-Control: public, max-age=3600` at `handlers.go:1688`, and `HandleWidgetEmbed` injects `widgetKey` into `window.AgentChatConfig` at `handlers.go:2046-2051`. When an admin rotates the `WidgetKey` via `UpdateAgentTenant`, all visitors who have cached `embed.js` or `widget.js` will continue sending the old key for up to 1 hour.

**Exploit Scenario (or rather, operational impact):**

1. Admin rotates `WidgetKey` for security reasons.
2. Cached `embed.js` (with 1-hour TTL) in browser caches/CDN still contains the old key.
3. All existing widgets on customer sites start receiving `403 Access denied`.
4. The admin has no way to force immediate invalidation.

**Impact:** Accidental or emergency key rotation causes 1-hour outage of the chat widget for all visitors.

**Recommended Fix:**

- Add a `widget_key_prev` column to `agent_tenants`.
- On rotation, copy the old key to `widget_key_prev` and set a rotation timestamp.
- Accept either `widget_key` or `widget_key_prev` during a grace window (e.g., 1 hour).
- Alternatively, serve `widget.js` and `embed.js` with a cache-busting versioned URL and set shorter cache TTL for the key-bearing asset.

---

### FINDING-5: PostgreSQL SQL Cast Using `$5::timestamp` Is Safe But Fragile (Medium)

**File:** `store/db/postgres/agent.go:1189-1222`, `store/db/postgres/agent.go:1224-1262`  
**Severity:** Medium

**Description:**

The PostgreSQL atomic rate limit SQL uses `$5::timestamp` to cast the incoming `time.Time` parameter. This works because the parameter is passed as a Go `time.Time` which the `lib/pq` driver converts to a Postgres `timestamptz`. However, the cast `$5::timestamp` is redundant if the driver already sends a proper timestamp type. More importantly, if any dialect mismatch occurs (e.g., using `now` as `$5` in a context where type inference fails), the cast could mask a silent failure.

**Exploit Scenario (low probability):**

None directly. But if the driver binding behavior changes, the query could silently fail or return incorrect rate-limit decisions.

**Impact:** Fragile SQL. On SQL driver upgrades or driver swaps, the rate limiter could malfunction.

**Recommended Fix:**

- Remove the explicit `::timestamp` cast and rely on parameter binding:
  ```sql
  WHEN EXTRACT(EPOCH FROM ($5 - agent_rate_limits.window_start)) > $6 THEN 1
  ```
- Add a comment documenting that `$5` is expected to be a `timestamptz` parameter.
- Verify that the Go `database/sql` driver passes `time.Time` as `timestamptz`.

---

## Checklist Answers (Per prompt_code_review.md)

### 1. Widget Key Bypass

- **Can an attacker extract the widget key from embed.js?** Yes, it is injected into `window.AgentChatConfig.widgetKey` in both `widget.js` (`handlers.go:1715`) and `embed.js` (`handlers.go:2046`). It's visible in DevTools and View Source. The plan honestly documents this as "obfuscation-grade". **This is an accepted limitation, not a bug.**
- **Constant-time comparison?** Yes, `subtle.ConstantTimeCompare` is used at `handlers.go:421`. ✅
- **Empty WidgetKey behavior?** See FINDING-1. The check is skipped entirely when `WidgetKey` is empty. **Fail-open, high severity.**
- **Brute-force resistance?** The key is a UUID v4 (122 bits of randomness). This is ~10¹⁸ possible values. Brute-force is infeasible against a single tenant. The real exposure is via browser extraction, not brute-force. ✅
- **Path where embed.js is served without widget key?** Yes, if `tenant.WidgetKey == ""`, both `widget.js` and `embed.js` are served with `widgetKey: ""` in the config.

### 2. Rate Limit Atomicity

- **Fresh row (no conflict):** ✅ INSERT runs, count=1.
- **Existing row, window expired:** ✅ CASE resets to 1, window_start updates.
- **Existing row, under limit:** ✅ count incremented by 1.
- **Existing row, at limit:** ✅ count unchanged, returns 0 (denied).
- **Two concurrent requests at limit boundary:** Both execute concurrently. Since the ops are atomic at the row level in SQLite (single writer lock), one will serialize. The first to get the write lock sees count<N and increments; the second sees count=N and is denied. ✅ No TOCTOU.
- **RETURNING clause:** Returns `CASE ... END` which evaluates to integer 0 or 1. In SQLite, `QueryRowContext(...).Scan(&allowed)` with `*bool` parameter may not work directly because Go's `database/sql` scanning from `int64` to `bool` is driver-specific. For `mattn/go-sqlite3`, it works. For `modernc.org/sqlite`, it may fail. **Recommend using `*int` or `sql.NullBool` for portability.**
- **julianday() correctness:** ✅ Uses UTC Julian day internally. Window arithmetic is correct.
- **Postgres EXTRACT(EPOCH FROM $5::timestamp):** ✅ Correct for computing seconds since epoch.
- **Sentinel collision:** `__tenant_global__` is extremely unlikely to collide with a real IP. **Low risk, but hardcoding a magic string is a design smell.**
- **UNIQUE constraint on (tenant_id, audience_type, client_ip):** ✅ Present in `LATEST.sql:388` as `UNIQUE(tenant_id, audience_type, client_ip)`. ✅

### 3. Session Turn Cap

- **MessageCount incremented before or after cap check?** Cap check is at `service.go:1608` (`session.MessageCount >= 50`). `MessageCount++` happens in `processChat` at `service.go:1924` AFTER the check. So the 50th message is denied; user gets 50 turns. ✅
- **Bypass by rotating session_id?** Yes. `NormalizeExternalSessionID` at `service.go:1094` accepts any valid-looking UUID. `GetOrCreate` at `service.go:982` creates a fresh session with `MessageCount=0` if not found in the in-memory store. A bot can generate new session IDs per request to bypass the cap. **This is documented as defense-in-depth only. The global tenant cap is the real boundary.** ✅ Documented.
- **What happens when count is exactly 50?** `>= 50` is false for 49, true for 50. Turn 50 passes, then MessageCount increments to 50. Turn 51 is rejected. Users get exactly 50 turns. ✅

### 4. Transcript HMAC Rekey

- **Old tokens rejected?** Yes. `generateSessionToken` and `verifySessionToken` now use `tenant.WidgetKey` at `service.go:1518` and `handlers.go:519`. Tokens signed with the old `GUID` will fail verification. **No grace period is implemented.**
- **Grace period for in-flight tokens?** **Not implemented.** The plan mentions it in the Risks section, but no code exists to accept either key during a transition window. This is a backward-compatibility bug.
- **HMAC derivation correctness?** `hmac.New(sha256.New, []byte(tenantGUID))` with salt `"session-token-key"`. HMAC-SHA256 is cryptographically sound for this purpose. ✅
- **Transcript endpoint uses WidgetKey?** Yes, `handlers.go:519`. ✅

### 5. BodyLimit and Message Length

- **BodyLimit scope:** Applied globally at `server.go:54`. Affects ALL routes. **Breaking for authenticated file uploads.** See FINDING-2.
- **Echo status code for body exceeded:** Echo's `BodyLimit` middleware returns `413 Payload Too Large`. ✅
- **MaxMessageLength default lowered from 4000 to 2000:** Migration defaults to 4000 (`LATEST.sql:235`), but `service.go:1569` overrides to 2000 if `config.Audience.MaxMessageLength <= 0`. This is the correct behavior per plan. ✅
- **Body limit applies to Content-Length or actual bytes?** Echo's `BodyLimit` middleware uses `c.Request().Body.Limit()` which counts bytes read from the request body stream, not just `Content-Length`. Works correctly with chunked encoding. ✅

### 6. Global Tenant Rate Limit

- **Hardcoded 300 RPM:** Yes, at `service.go:1594` and `sqlite/agent.go:1381` / `postgres/agent.go:1259`. Not per-tenant configurable. **Consider adding a field in `AgentTenant` or store for configurable global rate limit.**
- **Sentinel pollution:** One extra row per tenant in `agent_rate_limits` with `client_ip = '__tenant_global__'`. For 100 tenants, 100 extra rows. Minimal. ✅ Acceptable.
- **Race between per-IP and global:** Both use independent atomic `INSERT ... ON CONFLICT ... RETURNING`. Each correctly tracks its own counter. There is no race where both could pass when only one should. Under concurrency, both are evaluated atomically. ✅

### 7. Edge Cases

- **tenant.WidgetKey set, X-Widget-Key empty:** `subtle.ConstantTimeCompare([]byte(""), []byte(tenant.WidgetKey))` returns 0. Request rejected with 403. ✅
- **AllowedDomains + WidgetKey both set:** Widget key check runs first at `handlers.go:416`. Both must pass. Order is correct for fail-closed. ✅
- **Concurrent requests exhausting SQLite write lock:** SQLite serializes writers. With Fly's `hard_limit=25`, max 25 concurrent writers. Under adversarial load with 25 concurrent requests all trying to hit the same rate limit row, subsequent requests queue. This is acceptable for current single-instance deployment. ✅
- **Stale widget key in embed.js cache:** 1-hour cache means widgets may use the old key after rotation. See FINDING-4.
- **BodyLimit breaks authenticated endpoints:** See FINDING-2.

### 8. Test Coverage

- **Widget key validation (missing key → 403, valid key → 200):** Partially covered. `bridge_delivery_test.go` passes `tenant.WidgetKey` for tests. But there are no explicit tests for missing/invalid widget key. ❌ **Gap: Add test cases for missing widget key (403), empty widget key (403 or fail-open?), and rotated key (403).**
- **Atomic rate limit (TOCTOU race):** No direct test for concurrent boundary. The SQL is atomic by design, but a stress test with multiple goroutines would increase confidence. ❌ **Gap: Add concurrent test hitting the RPM boundary.**
- **Global tenant rate limit:** No explicit test. ❌ **Gap: Add test validating that IP rotation does not bypass the global cap.**
- **Session turn cap:** `bridge_delivery_test.go:422` tests memory session rebuild preserving message count, but no test that sends 51 messages and expects rejection. ❌ **Gap: Add test for turn cap enforcement.**
- **Transcript HMAC rekey:** No test for old GUID tokens being rejected. ❌ **Gap: Add test that tokens signed with `tenant.GUID` return 403.**
- **Body limit (body > 16KB → 413):** No test found. ❌ **Gap: Add test for large body rejection.**
- **Message length > 2000 → 400:** Partially covered (no test found in grep). ❌ **Gap: Add test for `ErrMessageTooLong`.**

### 9. SQL Correctness

- **SQLite parameter binding order:** Verified in `sqlite/agent.go:1291-1314`. 13 parameters bound correctly: `tenantID, audienceType, clientIP, now, now, windowSeconds, rpm, now, windowSeconds, now, now, windowSeconds, rpm`. ✅ Correct.
- **RETURNING CASE returns integer for bool scan:** SQLite returns integer 0 or 1. Scanning into `*bool` is driver-specific. For `mattn/go-sqlite3`, this works. For `modernc.org/sqlite` (used in build tags), scanning `int64` to `bool` may panic. **Recommend using `*int` and converting to bool in Go for portability.**
- **Postgres `$5::timestamp` cast:** ✅ Parameterized, safe from injection. See FINDING-5 for fragility note.
- **INSERT ... ON CONFLICT works with no existing row?** ✅ Standard SQLite/Postgres upsert behavior. Fresh row is inserted; no conflict occurs.
- **julianday and EXTRACT edge cases:** Midnight boundary, DST, leap seconds. `julianday()` in SQLite uses UTC internally, so DST transitions do not affect the window calculation. Leap seconds are absorbed by the OS kernel; neither SQLite nor Postgres expose them natively. ✅ Correct.

### 10. Security Properties

- **Fail-closed:** ⚠️ Partially. Widget key check is fail-closed when key exists, but **fail-open when WidgetKey is empty** (FINDING-1). Rate limiting is fail-closed (returns 429 on limit). Body limit is fail-closed (413).
- **Least privilege:** Widget key is the minimum secret for edge gate. ✅ Correct.
- **Defense in depth:**
  - Widget key bypassed → per-IP + global tenant rate limit still holds. ✅
  - Rate limit bypassed → session turn cap + body limit + message length. ✅ (session cap is defense-in-depth per documented intent)
- **No PII leakage:** Error messages are generic. `Access denied` does not reveal tenant IDs. ✅
- **Timing attacks:** `subtle.ConstantTimeCompare` for widget key. `hmac.Equal` for session token verification. ✅ Both are constant-time.

---

## Additional Findings

### FINDING-6: MySQL Stubs Are Non-Functional for Rate Limiting

**File:** `store/db/mysql/agent.go:223-227`  
**Severity:** Info

The `CheckAndIncrementAgentRateLimit` and `CheckAndIncrementTenantGlobalRateLimit` methods return `errNotImplemented` for MySQL. If a MySQL-backed instance is deployed, rate limiting silently fails, exposing the LLM pipeline to unbounded abuse.

**Recommended Fix:** Implement the same atomic upsert logic for MySQL, OR document that rate limiting requires SQLite or Postgres.

### FINDING-7: `deriveSessionTokenKey` Parameter Name Mismatch

**File:** `server/router/api/v1/agent/service.go:1525`  
**Severity:** Low

```go
func deriveSessionTokenKey(tenantGUID string) []byte {
```

The parameter is named `tenantGUID`, but the call site at `handlers.go:519` passes `tenant.WidgetKey`. This is a documentation bug that could confuse future maintainers.

**Recommended Fix:** Rename to `func deriveSessionTokenKey(edgeGateKey string) []byte` and update comments.

---

## Missing Test Coverage Summary

| Test Scenario | Status | Priority |
|---------------|--------|----------|
| Missing widget key → 403 | ❌ Missing | High |
| Empty WidgetKey on tenant → fail-closed | ❌ Missing | High |
| Invalid widget key → 403 | ❌ Missing | High |
| Atomic rate limit at boundary (concurrent) | ❌ Missing | Medium |
| Global tenant cap under IP rotation | ❌ Missing | Medium |
| Session turn cap enforcement | ❌ Missing | Medium |
| Transcript with GUID token → 403 | ❌ Missing | Medium |
| Body > 16KB → 413 | ❌ Missing | Medium |
| Message > 2000 → 400 | ❌ Missing | Low |

---

## Implementation Gaps vs. Plan

| Plan Task | Status | Gap |
|-----------|--------|-----|
| Migration + backfill | ⚠️ Partial | Column added but no startup backfill code found |
| Widget key rotation grace period | ❌ Not implemented | No grace period mechanism |
| Playground endpoint protection | ❌ Not implemented | Widget key check missing |
| BodyLimit scoped correctly | ❌ Incorrect | Applied globally, breaks file uploads |
| MySQL atomic rate limit | ❌ Stubs only | `errNotImplemented` |

---

## Final Verdict: **REJECTED — 2 High, 3 Medium findings must be resolved before production**

| # | Severity | Finding | Blocker? |
|---|----------|---------|----------|
| 1 | High | Empty WidgetKey creates fail-open path | Yes |
| 2 | High | Global BodyLimit breaks authenticated file uploads | Yes |
| 3 | Medium | Playground endpoint bypasses widget key | Yes |
| 4 | Medium | No grace period for widget key rotation | Yes |
| 5 | Medium | PostgreSQL cast is safe but fragile | Recommended |

**Recommendation:** Address FINDING-1 and FINDING-2 immediately (High severity). Findings 3-5 are Medium but worth addressing before merging. The core security architecture (atomic rate limiting, transcript HMAC rekey, session turn cap) is well-implemented and can be greenlit for Low/Info items post-merge with follow-up tickets.
