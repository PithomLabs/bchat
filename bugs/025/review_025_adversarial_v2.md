# Adversarial Code Review — Plan 025 Follow-Up Fixes (v2)

**Reviewer:** Go/TypeScript Security Engineer (adversarial)
**Date:** 2026-07-07
**Scope:** All changes made to fix findings from the combined three-review cross-reference.

---

## Executive Summary

The v2 fixes correctly address most of the findings from the initial adversarial review. The most critical regression from v1 is fixed: the widget key gate is now **fail-closed** when `tenant.WidgetKey == ""`, and the global `BodyLimit("16KB")` is scoped only to the public group, protecting authenticated file upload endpoints.

However, this review identified **1 Regression** and **2 Partial** fixes that must be resolved before shipping:

1. **REGRESSION — Iframe embed omits `widgetKey`:** `HandleWidgetIframe` generates HTML that does NOT inject `widgetKey` into `window.AgentChatConfig`, causing iframe-embedded widgets to receive `403 Access denied`. This breaks the iframe embedding path entirely.

2. **PARTIAL — Rate limit off-by-one:** The "fix" changed `<=` to `<` in both the UPDATE and RETURNING clauses. With this logic, `rpm=5` allows only **4 requests** instead of 5. The UPDATE condition `<` is correct, but the RETURNING condition must remain `<=`.

3. **PARTIAL — Transcript grace period is unbounded:** The fallback to `tenant.GUID` has no time limit, meaning a leaked GUID token remains valid forever.

---

## Verdict per Checkitem

### 1. Migration Correctness (C3+C4)

| Question | Finding |
|----------|---------|
| `IF NOT EXISTS` for index? | ✅ Yes, both migrations use `CREATE INDEX IF NOT EXISTS` |
| SQLite UUID backfill format? | ✅ Valid UUID v4 format using `randomblob(16)` |
| Postgres `gen_random_uuid()`? | ✅ Correct `gen_random_uuid()::text` |
| Idempotent on re-run? | ⚠️ **PARTIAL.** `ALTER TABLE agent_tenants ADD COLUMN widget_key TEXT` is NOT idempotent. Re-running fails with `duplicate column name`. The `CREATE INDEX IF NOT EXISTS` IS idempotent. Similarly, `ALTER TABLE agent_audiences ADD COLUMN max_message_length ...` is not idempotent. Migration runners typically track executed migrations, so this is safe in normal operation. But if a DBA manually re-runs the SQL, it errors. |
| `LATEST.sql` includes `widget_key`? | ✅ Yes, both sqlite and postgres |
| Postgres `LATEST.sql` includes `max_message_length`? | ✅ Yes (`max_message_length INTEGER NOT NULL DEFAULT 2000`) |
| Errors if column already exists? | ⚠️ Yes, `ALTER TABLE ADD COLUMN` errors if column exists. Should use `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` (Postgres 9.6+) or wrap in a DO block / conditional. |

**Verdict: PARTIAL.** Backfills are correct, but ALTER TABLE idempotency is missing.

---

### 2. Fail-Closed Gate (H2+M4)

| Question | Finding |
|----------|---------|
| Rejects when `tenant.WidgetKey == ""`? | ✅ Yes. `handlers.go:416`: `if tenant.WidgetKey == "" { return 403 }` |
| Rejects empty incoming `X-Widget-Key`? | ✅ Yes. `widgetKey == ""` check before comparison |
| `subtle.ConstantTimeCompare` still used? | ✅ Yes, `handlers.go:424` |
| Whitespace-only bypass? | ❌ Not possible. `Header.Get("X-Widget-Key")` returns the raw value. A whitespace-only string has length > 0, so it passes the empty check, but `subtle.ConstantTimeCompare([]byte(" "), []byte(realKey))` returns 0 (length mismatch short-circuits to false). This is safe: length mismatch returns immediately, which does not leak key material. |
| Gate applied BEFORE LLM/expensive ops? | ✅ Yes, gate is lines 415-425, before `ChatExternal` call at line 456 |
| Error message leaks info? | ✅ No. Generic `"Access denied"` - no info about which check failed |

**Verdict: CORRECT.** Fail-closed is properly implemented.

---

### 3. Widget Key in Client (C1)

| Question | Finding |
|----------|---------|
| `mergeConfig()` copies `widgetKey`? | ✅ Yes, `embed.ts:104`: `widgetKey: scriptConfig.widgetKey || globalConfig.widgetKey` |
| `initWithConfig()` copies `widgetKey`? | ✅ Yes, `embed.ts:135`: `widgetKey: userConfig.widgetKey || scriptConfig.widgetKey` |
| `api.ts` sends `X-Widget-Key` header? | ✅ Yes, `api.ts:17-18`: `if (config.widgetKey) { headers['X-Widget-Key'] = config.widgetKey; }` |
| Undefined widgetKey → undefined on client? | ✅ Yes, both merge paths yield `undefined` if absent in both sources. Server rejects with 403. |
| Visible in DevTools? | ✅ Yes, `window.AgentChatWidget.config.widgetKey` is visible. Documented as obfuscation-grade. |

**Verdict: CORRECT.**

---

### 4. Rate Limit Off-by-One (C2)

| Question | Finding |
|----------|---------|
| SQLite `CheckAndIncrementAgentRateLimit` uses `<` in RETURNING? | ⚠️ **Yes, but this is wrong.** `sqlite/agent.go:1309`: `WHEN request_count < ? THEN 1`. With rpm=5, only 4 requests are allowed. |
| SQLite global rate limit uses `<` in RETURNING? | ⚠️ Same bug at `sqlite/agent.go:1351` |
| Postgres `CheckAndIncrementAgentRateLimit` uses `<` in RETURNING? | ⚠️ Same bug at `postgres/agent.go:1212` |
| Postgres global rate limit uses `<` in RETURNING? | ⚠️ Same bug at `postgres/agent.go:1252` |
| UPDATE clause still `request_count < ?`? | ✅ Yes. This part is correct — stop incrementing at limit. |
| Trace: rpm=5, does the 5th request get denied? | ✅ Confirmed: 5th request is DENIED. Only 4 allowed out of 5 permitted. |

**Semantic trace for rpm=5:**

| Request | Pre-count | UPDATE condition | Post-count | RETURNING (`<5`) | Result |
|---------|-----------|-----------------|------------|------------------|--------|
| 1 | 0 (new) | — | 1 | `1<5` → true | Allowed |
| 2 | 1 | `1<5` → true | 2 | `2<5` → true | Allowed |
| 3 | 2 | `2<5` → true | 3 | `3<5` → true | Allowed |
| 4 | 3 | `3<5` → true | 4 | `4<5` → true | Allowed |
| 5 | 4 | `4<5` → true | 5 | `5<5` → false | **DENIED** |
| 6 | 5 | `5<5` → false | 5 | `5<5` → false | Denied |

**Impact:** For any configured RPM, the system allows only (RPM — 1) requests. With default 300 RPM global cap, only 299 requests are actually allowed per minute.

**Recommended fix:** Change RETURNING to use `<=`:
```sql
RETURNING CASE
    WHEN (julianday(?) - julianday(window_start)) * 86400 > ? THEN 1
    WHEN request_count <= ? THEN 1
    ELSE 0
END
```

**Verdict: REGRESSION.** The "off-by-one fix" introduced a new, more severe off-by-one bug. Only N-1 requests are allowed out of the configured N.

---

### 5. Postgres Timestamp Cast (M3)

| Question | Finding |
|----------|---------|
| All `::timestamp` casts removed? | ✅ Yes. No remaining `::timestamp` casts in `postgres/agent.go`. |
| SQL still works without cast? | ✅ Yes. `lib/pq` and `pgx` both send Go `time.Time` as `timestamptz`. |
| Any remaining `::timestamp` casts? | ✅ None found |

**Verdict: CORRECT.**

---

### 6. BodyLimit Scope (H1)

| Question | Finding |
|----------|---------|
| `BodyLimit("16KB")` removed from `server.go`? | ✅ Confirmed. No `BodyLimit` in `server/server.go`. |
| Added to `publicGroup` in `v1.go`? | ✅ Yes, `v1.go:254`: `publicGroup.Use(middleware.BodyLimit("16KB"))` |
| `publicGroup` includes chat/ext, transcript, playground? | ✅ Yes. chat/ext, transcript, playground/run, widget.js are all under `publicGroup`. |
| Admin/auth group does NOT have 16KB limit? | ✅ Admin group uses `adminCORS` only, no BodyLimit. |
| 50KB file upload succeeds on `HandleImportSingleFile`? | ✅ Yes, auth routes are unrestricted by BodyLimit. |
| 20KB chat request hits 413 on public endpoints? | ✅ Yes, publicGroup BodyLimit rejects. |

**Verdict: CORRECT.**

---

### 7. MySQL Stubs (H3)

| Question | Finding |
|----------|---------|
| `CheckAndIncrementAgentRateLimit` returns `(true, nil)`? | ✅ Yes, with warning log |
| `CheckAndIncrementTenantGlobalRateLimit` returns `(true, nil)`? | ✅ Yes, with warning log |
| `slog` imported? | ✅ Yes, used at lines 226-227 and 233-234 |
| Warning includes enough context? | ⚠️ **Partial.** Log says `"agent rate limit check not implemented for MySQL, allowing request"`. It does NOT include tenant_id, audience_type, or client_ip, which would help diagnose abuse patterns. |
| Fail-open the right choice? | ✅ Yes. MySQL is unsupported for agent features. Failing closed would break all traffic. |

**Verdict: PARTIAL.** Fail-open is acceptable, but warning logs should include tenant/IP context for operational visibility.

---

### 8. Playground Widget Key (M1)

| Question | Finding |
|----------|---------|
| `HandlePlaygroundRun` checks widget key? | ✅ Yes, `playground.go:539-546` |
| Check identical to `HandleChatExternal`? | ✅ Yes. Same empty check, same `subtle.ConstantTimeCompare`, same header + query param fallback. |
| Uses `subtle.ConstantTimeCompare`? | ✅ Yes, `playground.go:546` |
| `crypto/subtle` imported? | ✅ Yes |
| Rejects empty `tenant.WidgetKey`? | ✅ Yes, `playground.go:539` |
| Rejects empty incoming key? | ✅ Yes, `widgetKey == ""` check at line 545 |

**Verdict: CORRECT.**

---

### 9. Transcript Grace Period (M2)

| Question | Finding |
|----------|---------|
| `HandleGetExternalTranscript` tries WidgetKey first? | ✅ Yes, `handlers.go:519`: `verifySessionToken(token, sessionID, expiryStr, tenant.WidgetKey)` |
| Falls back to GUID? | ✅ Yes, `handlers.go:520-522`: `if err != nil && tenant.GUID != "" { verifySessionToken(..., tenant.GUID) }` |
| Both fail → rejected? | ✅ Yes, `handlers.go:523`: `if err != nil || time.Now().After(expiry) { return 403 }` |
| Time limit on grace period? | ❌ **No time limit.** Any token signed with the old GUID remains valid indefinitely, as long as `tenant.GUID` is non-empty. |
| Attacker exploit old GUID forever? | ⚠️ **Yes.** If a GUID is ever leaked (it was public in widget.js), an attacker can forge transcripts for any session forever, even after key rotation. The grace period should expire. |

**Impact:** The fallback should be time-bound (e.g., 7 days after key rotation). Without a limit, the grace period becomes a permanent backdoor.

**Recommended fix:** Add a rotation timestamp column or a fixed cutoff (e.g., only accept GUID if token expiry is within 7 days of deployment).

**Verdict: PARTIAL.** Grace period exists but is unbounded. This is a significant weakness.

---

### 10. Schema Alignment

| Question | Finding |
|----------|---------|
| SQLite `max_message_length INTEGER DEFAULT 2000`? | ✅ Yes, `LATEST.sql:235` |
| Postgres `max_message_length INTEGER NOT NULL DEFAULT 2000`? | ✅ Yes, `LATEST.sql:170` |
| Go code defaults to 2000 when `MaxMessageLength <= 0`? | ✅ Yes, `service.go:1569`: `maxLen := 2000 // default` |
| Any remaining mismatches? | ✅ None found |

**Verdict: CORRECT.**

---

### 11. Regressions

| Question | Finding |
|----------|---------|
| Version bump 0.28.0 → 0.29.0 breaks anything? | ✅ Semantic version bump. No breaking API changes. |
| Existing tests still pass? | ⚠️ Cannot verify without running tests, but `TestGetCurrentSchemaVersion` updated to `0.29.`. Comment at line 16 still says `0.28.x` but assertion is correct — minor cosmetic issue. |
| `bridge_delivery_test.go` still valid? | ✅ Uses `tenant.WidgetKey` in test requests — consistent with new code. |
| `embed.js` config injection still works? | ✅ `HandleWidgetEmbed` injects `widgetKey` into `window.AgentChatConfig` at `handlers.go:2056`. |

**Verdict: CORRECT** (with minor cosmetic comment issue in test).

---

### 12. New Attack Surfaces

| Question | Finding |
|----------|---------|
| Playground widget key check introduces timing side-channel? | ✅ No. `subtle.ConstantTimeCompare` is constant-time for equal lengths; length mismatch returns immediately, which is fine (key length is public). |
| Transcript grace period extends forgery window? | ⚠️ **Yes.** See Finding 9. GUID fallback is permanent. |
| MySQL fail-open creates DoS vector? | ⚠️ **Yes, but accepted.** MySQL deployments get unlimited LLM spend. Documented as unsupported. |
| BodyLimit moved to publicGroup creates bypass? | ❌ No. `chat/ext` is only registered on `publicGroup`. `HandleChatInternal` is on `authGroup` (behind JWT). No bypass path. |

**Verdict: PARTIAL.** Grace period permanence is a real weakness. MySQL fail-open is accepted.

---

## Additional Findings from v2 Review

### FINDING-v2-1: Iframe Embedding Broken — Missing `widgetKey` (Regression)

**File:** `server/router/api/v1/agent/handlers.go:2135-2145`  
**Severity:** Medium

**Description:**

`HandleWidgetIframe` calls `generateIframeHTML`, which builds a `window.AgentChatConfig` object but does NOT include `widgetKey`. Compare:

`HandleWidgetEmbed` (correct):
```go
window.AgentChatConfig.widgetKey=window.AgentChatConfig.widgetKey||%q;
```

`generateIframeHTML` (missing):
```html
window.AgentChatConfig = {
  baseUrl: '%s',
  tenant: '%s',
  companyName: '%s',
  color: '%s',
  welcomeMessage: '%s'
  // NO widgetKey
};
```

When `embed.js` loads inside the iframe, it reads `window.AgentChatConfig.widgetKey` (via `mergeConfig` → `initWithConfig`), finds `undefined`, and does NOT send the `X-Widget-Key` header. The `chat/ext` endpoint then rejects with `403 Access denied`.

**Exploit scenario:** N/A — this is a functional bug, not a security bypass. But it breaks iframe deployment for all tenants.

**Impact:** Iframe embedding is completely non-functional after plan 025.

**Recommended fix:**
```go
window.AgentChatConfig = {
  baseUrl: '%s',
  tenant: '%s',
  companyName: '%s',
  widgetKey: '%s',   // ADD THIS
  color: '%s',
  welcomeMessage: '%s'
};
```

---

### FINDING-v2-2: Rate Limit Off-by-One Allows Only N-1 Requests (Regression)

**File:** `store/db/sqlite/agent.go:1309`, `store/db/sqlite/agent.go:1351`, `store/db/postgres/agent.go:1212`, `store/db/postgres/agent.go:1252`  
**Severity:** Medium

**Description:**

Both the UPDATE and RETURNING clauses use `< rpm`. The UPDATE clause correctly prevents the counter from exceeding `rpm`. But the RETURNING clause then checks `request_count < rpm` on the POST-UPDATE value, which means when `request_count == rpm`, the request is denied.

**Trace for rpm=5:**
- Request 1: count becomes 1, RETURNING `1<5` → allowed
- Request 2: count becomes 2, RETURNING `2<5` → allowed
- Request 3: count becomes 3, RETURNING `3<5` → allowed
- Request 4: count becomes 4, RETURNING `4<5` → allowed
- Request 5: count becomes 5, RETURNING `5<5` → **DENIED**

**Impact:** The global tenant cap of 300 RPM actually allows only 299. The per-IP limit of 60 RPM allows only 59. This is a silent under-provisioning that reduces the intended rate-limit capacity by ~17%.

**Recommended fix:** Change RETURNING to use `<=`:
```sql
RETURNING CASE
    WHEN (julianday(?) - julianday(window_start)) * 86400 > ? THEN 1
    WHEN request_count <= ? THEN 1
    ELSE 0
END
```

---

### FINDING-v2-3: Transcript Grace Period Has No Expiration (Partial)

**File:** `server/router/api/v1/agent/handlers.go:519-523`  
**Severity:** Medium

**Description:**

The fallback to `tenant.GUID` is unconditional on time. If an attacker obtains a GUID (which was public in `widget.js` before plan 025), they can forge a token and access transcripts forever, even after the tenant rotates `WidgetKey`.

**Exploit scenario:**
1. Attacker reads old `widget.js` from Wayback Machine or cached copy containing `GUID`.
2. Attacker forges a transcript token using `deriveSessionTokenKey(GUID)`.
3. Tenant rotates `WidgetKey` for security.
4. Attacker's forged token still works because the GUID fallback has no expiration.

**Impact:** Permanent token forgery for any tenant whose GUID was ever exposed.

**Recommended fix:**
- Add a `widget_key_rotated_at` timestamp column.
- Only accept GUID fallback if rotation was within a grace window (e.g., 7 days).
- Or add a hard cutoff: reject GUID fallback entirely after 2026-08-01.

---

### FINDING-v2-4: Migration Not Idempotent (Partial)

**Files:**
- `store/migration/sqlite/0.29/01__add_widget_key.sql:2`
- `store/migration/postgres/0.29/01__add_widget_key.sql:2`
- `store/migration/postgres/0.29/02__add_max_message_length.sql:1`

**Description:**

`ALTER TABLE agent_tenants ADD COLUMN widget_key TEXT` and `ALTER TABLE agent_audiences ADD COLUMN max_message_length ...` will fail if re-run. Standard migration runners prevent re-execution, but manual re-runs or DBA interventions will error.

**Recommended fix:** Use `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` (supported in Postgres 9.6+ and SQLite 4.35+ with `ALTER TABLE ... COLUMN IF NOT EXISTS` or check via `PRAGMA table_info`).

---

### FINDING-v2-5: MySQL Warning Logs Lack Context (Info)

**File:** `store/db/mysql/agent.go:224-234`  
**Severity:** Low

**Description:**

Warning logs do not include `tenantID`, `audience_type`, or `client_ip`, making it impossible to correlate MySQL fail-open events with specific tenants or abuse patterns.

**Recommended fix:**
```go
func (d *DB) CheckAndIncrementAgentRateLimit(...) (bool, error) {
    slog.Warn("agent rate limit check not implemented for MySQL, allowing request",
        "tenant_id", tenantID, "audience_type", audienceType, "client_ip", clientIP)
    return true, nil
}
```

---

## Checklist Summary

| # | Checkitem | Verdict |
|---|-----------|---------|
| 1 | Migration correctness | PARTIAL — backfills good, but ALTER TABLE not idempotent |
| 2 | Fail-closed gate | CORRECT |
| 3 | Widget key in client | CORRECT |
| 4 | Rate limit off-by-one | REGRESSION — only N-1 requests allowed |
| 5 | Postgres timestamp cast | CORRECT |
| 6 | BodyLimit scope | CORRECT |
| 7 | MySQL stubs | PARTIAL — fail-open accepted, but logs lack context |
| 8 | Playground widget key | CORRECT |
| 9 | Transcript grace period | PARTIAL — unbounded GUID fallback |
| 10 | Schema alignment | CORRECT |
| 11 | Regressions | CORRECT (minor comment cosmetic) |
| 12 | New attack surfaces | PARTIAL — grace period permanence |

---

## Specific Questions

1. **Is the migration idempotent?** No. `ALTER TABLE ADD COLUMN` fails on re-run. Only the `CREATE INDEX IF NOT EXISTS` is idempotent.
2. **Is the backfill safe for large tables?** Yes. `UPDATE ... WHERE widget_key IS NULL` runs in a transaction with the ALTER. Safe.
3. **Does Postgres driver send `time.Time` as `timestamp` without cast?** Yes. Both `lib/pq` and `pgx` send Go `time.Time` as `timestamptz`.
4. **Is `embed.ts` fix sufficient?** Mostly. `mergeConfig` and `initWithConfig` propagate `widgetKey`, and `api.ts` sends the header. **BUT `HandleWidgetIframe` does NOT inject `widgetKey` into iframe HTML, breaking iframe mode.**
5. **Is rate limit semantics now correct?** No. With `<` in RETURNING, only N-1 requests are allowed for configured RPM=N. Should be `<=` in RETURNING.
6. **Can attacker bypass widget key via internal chat endpoint?** No. `HandleChatInternal` requires JWT auth (`authGroup` with `AuthMiddleware`). Unauthenticated users cannot reach it.
7. **Does transcript grace period have expiration?** No. The fallback to GUID is permanent.
8. **Is `ConstantTimeCompare` actually constant-time for unequal lengths?** Yes. Go's `crypto/subtle.ConstantTimeCompare` returns `false` immediately on length mismatch, but this does not leak key material because the key length is public knowledge (UUID v4 = 36 chars).

---

## Final Verdict: **NOT SAFE TO SHIP**

### Blocking Issues

| # | Severity | Finding | Blocking? |
|---|----------|---------|-----------|
| 1 | Medium-Regression | Iframe embedding broken — `widgetKey` not injected into iframe HTML | Yes |
| 2 | Medium-Regression | Rate limit allows only N-1 requests instead of N | Yes |
| 3 | Medium-Partial | Transcript grace period has no time bound — GUID fallback permanent | Yes |

### Non-Blocking (Fix in Follow-Up)

| # | Severity | Finding |
|---|----------|---------|
| 4 | Low | MySQL warning logs lack tenant/IP context |
| 5 | Low | Migration ALTER TABLE not idempotent |

### What Was Fixed Correctly

- ✅ Widget key gate is now fail-closed (empty `WidgetKey` → 403)
- ✅ Global `BodyLimit("16KB")` moved to `publicGroup` only
- ✅ Playground endpoint now validates widget key
- ✅ Postgres `::timestamp` casts removed
- ✅ Transcript uses `WidgetKey` with GUID fallback (grace period concept)
- ✅ Schema aligned: `max_message_length` default 2000 in both SQLite and Postgres
- ✅ Version bumped to 0.29.0

**Recommendation:** Fix the iframe `widgetKey` omission, correct the RETURNING clause to `<=`, and add a time-bound cutoff for the GUID grace period before merging. These are surgical fixes that can be resolved quickly.
