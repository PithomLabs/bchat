# Adversarial Code Review Prompt — plan_025 Implementation

**Target:** Security hardening of bchat public chat endpoints (plan_025)
**Reviewer role:** Senior Go security engineer performing adversarial review
**Mode:** Assume every input is hostile. Look for bypasses, not happy paths.

---

## Context

This implementation adds five defense layers to the unauthenticated `POST /api/v1/agent/:slug/chat/ext` endpoint:

1. **Widget Key** — static per-tenant key sent via `X-Widget-Key` header
2. **Atomic Rate Limit** — TOCTOU-free per-IP rate limit + global per-tenant cap
3. **Input Hygiene** — 16KB body limit, 2000-char message cap, 50-turn session cap
4. **Transcript HMAC** — rekeyed from leaked GUID to private WidgetKey
5. **Deployment** — Fly.io request timeout

---

## Files Changed

| File | What changed |
|------|-------------|
| `store/migration/sqlite/LATEST.sql` | Added `widget_key` column + index to `agent_tenants` |
| `store/agent.go` | Added `WidgetKey` field to `AgentTenant` struct |
| `store/db/sqlite/agent.go` | CRUD updated for `widget_key`; atomic rate limit; global tenant rate limit |
| `store/db/postgres/agent.go` | Same for Postgres |
| `store/db/mysql/agent.go` | Stubs added |
| `store/driver.go` | New interface methods |
| `server/router/api/v1/agent/handlers.go` | Widget key validation in `HandleChatExternal`; `widgetKey` in embed.js/widget.js config; transcript verify uses `WidgetKey` |
| `server/router/api/v1/agent/service.go` | Atomic `CheckRateLimit`; global tenant cap; session turn cap; `generateSessionToken` uses `WidgetKey`; message length default 2000 |
| `server/server.go` | `BodyLimit("16KB")` middleware |
| `fly.toml` | `request_timeout = "30s"` |
| `widget/src/core/types.ts` | `widgetKey` in `WidgetConfig` |
| `widget/src/core/api.ts` | Sends `X-Widget-Key` header |
| `web/src/components/AgentChatWidget.tsx` | `widgetKey` prop + header |
| `server/router/api/v1/agent/bridge_delivery_test.go` | Test updates for widget key |

---

## Review Checklist — Answer Each With Findings

### 1. Widget Key Bypass

- [ ] Can an attacker extract the widget key from `embed.js` and use it from a different origin? What stops them?
- [ ] The widget key is injected into `window.AgentChatConfig.widgetKey` in the served JS. Is this visible in browser DevTools/network tab? Is this acceptable?
- [ ] Does the `X-Widget-Key` check use constant-time comparison? (`crypto/subtle.ConstantTimeCompare`)
- [ ] What happens if `tenant.WidgetKey` is empty (migration not yet run)? Does the check skip or fail-closed?
- [ ] Can the widget key be brute-forced? (It's a UUID v4 — 122 bits of randomness. Is this sufficient?)
- [ ] Is there any path where `embed.js` is served without the widget key in the config?

### 2. Rate Limit Atomicity

- [ ] The atomic SQL uses `ON CONFLICT DO UPDATE` with a `CASE` expression. Trace the exact row behavior for these scenarios:
  - Fresh row (no conflict)
  - Existing row, window expired
  - Existing row, under limit
  - Existing row, at limit
  - Two concurrent requests at the limit boundary
- [ ] Does the `RETURNING` clause return the correct allowed/denied status?
- [ ] Is the `julianday()` function correct for SQLite window expiry? What timezone does it use?
- [ ] For Postgres: is `EXTRACT(EPOCH FROM ...)` correct with the `$5::timestamp` cast?
- [ ] The sentinel `__tenant_global__` for the global tenant cap — could a legitimate client IP collide with this value?
- [ ] Is the UNIQUE constraint on `(tenant_id, audience_type, client_ip)` actually present in the existing schema? (It should be — verify.)

### 3. Session Turn Cap

- [ ] The session turn cap is 50 turns. Is `session.MessageCount` incremented before or after the cap check? (It should be checked before processing.)
- [ ] Can an attacker bypass the cap by rotating `session_id` values? (Yes — this is documented as defense-in-depth. Verify the global tenant cap is the real boundary.)
- [ ] What happens when `session.MessageCount` is exactly 50? Is the 50th message allowed or rejected?

### 4. Transcript HMAC Rekey

- [ ] `generateSessionToken` now uses `tenant.WidgetKey`. What happens to tokens generated BEFORE the migration (signed with GUID)? Are they rejected?
- [ ] Is there a migration path for existing in-flight tokens? (The plan mentions a grace period — is it implemented?)
- [ ] The `deriveSessionTokenKey` function uses HMAC-SHA256 with the raw key material. Is this cryptographically sound?
- [ ] Verify that the transcript endpoint (`HandleGetExternalTranscript`) now uses `tenant.WidgetKey` for verification, not `tenant.GUID`.

### 5. BodyLimit and Message Length

- [ ] Does `middleware.BodyLimit("16KB")` apply to all routes or only the public group? Should authenticated routes have a higher limit?
- [ ] What HTTP status code does Echo return when the body exceeds the limit? (Should be 413.)
- [ ] The default `MaxMessageLength` was lowered from 4000 to 2000. Are there any callers that depend on the old default?
- [ ] Does the body limit apply to the `Content-Length` header or the actual body read? What about chunked encoding?

### 6. Global Tenant Rate Limit

- [ ] The global tenant cap is hardcoded at 300 RPM in `service.go`. Should this be configurable per-tenant?
- [ ] The sentinel `__tenant_global__` is inserted into the `agent_rate_limits` table with `client_ip = '__tenant_global__'`. Does this pollute the rate limit table? How many extra rows does this create per tenant?
- [ ] Can the per-IP rate limit and global tenant rate limit race each other? (Both are atomic independently, but could both pass when only one should?)

### 7. Edge Cases

- [ ] What happens when a tenant has `WidgetKey` set but the request has `X-Widget-Key: ""` (empty string)? Is it rejected?
- [ ] What happens when `AllowedDomains` is set AND `WidgetKey` is set? Both checks run — is the order correct?
- [ ] Can an attacker send a very large number of concurrent requests to exhaust the SQLite write lock?
- [ ] What happens if the `embed.js` cache (`max-age=3600`) serves a stale widget key after rotation?
- [ ] The `BodyLimit("16KB")` middleware runs on ALL routes. Does this break any existing endpoints that expect larger bodies (e.g., file uploads)?

### 8. Test Coverage

- [ ] Are there tests for the widget key validation (missing key → 403, valid key → 200)?
- [ ] Are there tests for the atomic rate limit (TOCTOU race at boundary)?
- [ ] Are there tests for the global tenant rate limit?
- [ ] Are there tests for the session turn cap?
- [ ] Are there tests for the transcript HMAC rekey (old GUID token → 403)?
- [ ] Are there tests for the body limit (body > 16KB → 413)?
- [ ] Are there tests for message length > 2000 → 400?

### 9. SQL Correctness

- [ ] Trace the exact SQL for `CheckAndIncrementAgentRateLimit` in SQLite. Write out the parameter binding order. Is it correct?
- [ ] Does `RETURNING CASE ... END` in SQLite return 1 or 0 (integer)? Does Go scan this correctly into a `bool`?
- [ ] The Postgres version uses `$5::timestamp` — is this safe against SQL injection? (It's parameterized, but verify.)
- [ ] Does the `INSERT ... ON CONFLICT` work correctly when the `agent_rate_limits` table has no existing row for the key?
- [ ] Are the `julianday()` and `EXTRACT(EPOCH FROM ...)` calculations correct for edge cases (midnight boundary, DST, leap seconds)?

### 10. Security Properties

- [ ] **Fail-closed:** Does every security check default to denying access when the check cannot be performed?
- [ ] **Least privilege:** Is the widget key the minimum secret needed? Could a weaker mechanism work?
- [ ] **Defense in depth:** If the widget key is bypassed, do the rate limits still hold? If rate limits are bypassed, does the session turn cap hold?
- [ ] **No PII leakage:** Does any error message reveal the widget key, GUID, or internal state?
- [ ] **Timing attacks:** Is the widget key comparison constant-time? Is the HMAC verification constant-time?

---

## How to Report Findings

For each finding, provide:
1. **Severity:** Critical / High / Medium / Low / Info
2. **File:line** reference
3. **Description** of the issue
4. **Exploit scenario** (how an attacker would trigger it)
5. **Recommended fix**

---

## Known Limitations (documented, not bugs)

These are accepted risks per the plan review:
- Widget key is obfuscation-grade (visible in browser DevTools)
- Session turn cap is bypassable by rotating session IDs (defense-in-depth only)
- Global tenant cap is eventually-consistent across Fly instances
- No per-tenant LLM spend cap (flagged as follow-up)
- CORS wildcard `*` on public endpoints (flagged as follow-up)
