# Adversarial Code Review — plan_025 Implementation

**Reviewer:** Senior Go security engineer (adversarial)
**Mode:** Every input is hostile. Looking for bypasses, not happy paths.
**Code state:** Uncommitted working-tree changes on `bchat0534` (Fly.io prod deploy target).
**Verdict:** NOT SAFE TO SHIP. Multiple fail-open / breakage issues. See Critical/High findings.

---

## Summary of Findings

| # | Severity | Area | Finding |
|---|----------|------|---------|
| F1 | **CRITICAL** | Widget key gate | Gate is fail-OPEN when `tenant.WidgetKey == ""` (handlers.go:416) → existing/un-migrated tenants are fully exposed |
| F2 | **CRITICAL** | Migration | No `ALTER TABLE agent_tenants ADD COLUMN widget_key` migration; only `LATEST.sql` CREATE has it → existing prod DBs error on `SELECT widget_key` → all chat 403 (deployment breakage) OR, if column added manually with NULLs, combined with F1 = open |
| F3 | **HIGH** | Backfill | No backfill of `widget_key` for existing tenants → silent fail-open (F1) after column exists |
| F4 | **HIGH** | MySQL driver | `CheckAndIncrement*` stubs return `(false, errNotImplemented)` → MySQL deployments deny ALL requests (functional breakage), and the bool path is meaningless |
| F5 | **HIGH** | BodyLimit scope | `middleware.BodyLimit("16KB")` applied globally (server.go:54) → breaks file-upload / KB-import / admin endpoints expecting larger bodies |
| F6 | **MEDIUM** | Rate limit off-by-one | `request_count <= rpm` allows `rpm+1` requests per window (atomic SQL + service.go:1585) |
| F7 | **MEDIUM** | Constant-time empty key | `subtle.ConstantTimeCompare([]byte(""),[]byte(""))` returns 1; combined with F1 (empty tenant key) attacker with empty `X-Widget-Key` passes |
| F8 | **MEDIUM** | Transcript token after migration | Tokens minted pre-migration with GUID are immediately invalid; no grace period implemented (documented but NOT present in code) → in-flight visitor transcripts break on deploy |
| F9 | **LOW** | Global cap hardcoded | `globalTenantRPM = 300` hardcoded in service.go:1594 (duplicate of store default); not per-tenant configurable |
| F10 | **LOW** | `julianday`/local-tz window | SQLite window uses local-time `julianday(time.Now())`; DST boundary can shave/extend a window by up to 1h (minor) |
| F11 | **INFO** | CORS wildcard | Public endpoints still allow `*` origin (flagged follow-up; acceptable for now but pairs with F1) |

---

## Detailed Findings

### F1 — Widget key gate is FAIL-OPEN [CRITICAL]
**File:** `server/router/api/v1/agent/handlers.go:416-425`
```go
if tenant.WidgetKey != "" {
    widgetKey := c.Request().Header.Get("X-Widget-Key")
    ...
    if subtle.ConstantTimeCompare(...) != 1 {
        return 403
    }
}
```
The entire edge gate is wrapped in `if tenant.WidgetKey != ""`. If the tenant has no widget key (every tenant created before this change, or a tenant whose key wasn't backfilled), the check is **skipped entirely** and the request proceeds. An attacker does not need the key — they just need a tenant without one.

**Exploit:** Enumerate slugs (or target any pre-existing tenant) → POST `chat/ext` with no `X-Widget-Key` → request is processed, LLM billed. The "edge gate" provides zero protection for the majority of existing tenants.

**Fix:** Gate must be fail-CLOSED. Require a valid widget key for ALL tenants. For tenants lacking one, either (a) generate+persist a key at startup/migration, or (b) deny with 403 until a key exists. Do not skip the check when the key is empty.

---

### F2 — Missing ALTER migration breaks existing deployments [CRITICAL]
**File:** `store/migration/sqlite/LATEST.sql:206` (CREATE only); no `ALTER` under `store/migration/sqlite/0.25/`.
The `widget_key` column exists only in the `LATEST.sql` `CREATE TABLE agent_tenants`. Existing production databases were created by earlier migrations and will NOT have this column. `ListAgentTenants` now `SELECT ... widget_key ...` (sqlite/agent.go:65). On an existing DB this yields `no such column: widget_key` → `GetAgentTenant` errors → `tenant == nil` → handler returns 403 for **every** chat request. The public chat is fully down after deploy until a manual `ALTER` is run.

**Fix:** Add a numbered migration `store/migration/sqlite/0.29/NN__add_widget_key.sql`:
```sql
ALTER TABLE agent_tenants ADD COLUMN widget_key TEXT;
CREATE INDEX IF NOT EXISTS idx_agent_tenants_widget_key ON agent_tenants(widget_key);
```
(Do not rely on LATEST.sql for existing DBs — LATEST only runs on fresh init.)

---

### F3 — No backfill → silent fail-open (chains with F1) [HIGH]
Even after F2's ALTER runs, existing rows have `widget_key = NULL`. With F1's `if tenant.WidgetKey != ""` guard, those tenants remain fully open. There is no backfill (grep confirms none). New tenants get a key (sqlite/agent.go:27, postgres/agent.go:26), but 100% of pre-existing tenants are unprotected.

**Fix:** Backfill in the migration or a one-time startup job:
```sql
UPDATE agent_tenants SET widget_key = lower(hex(randomblob(16))) WHERE widget_key IS NULL OR widget_key = '';
```
Then make the gate fail-closed (F1) so a NULL key → 403, never open.

---

### F4 — MySQL stubs return `(false, errNotImplemented)` [HIGH]
**File:** `store/db/mysql/agent.go:223-229`
```go
func (d *DB) CheckAndIncrementAgentRateLimit(...) (bool, error) {
    return false, errNotImplemented
}
```
The callers (`service.go:1349`, `service.go:1595`) **ignore the error** and only test `allowed`. So under MySQL: `allowed == false` → `fmt.Errorf("rate limit exceeded")` → 429 for EVERY request. The public chat is completely non-functional on MySQL. (The bool being `false` is "safe" against abuse but breaks all legitimate traffic.)

**Fix:** Either implement the atomic rate limit for MySQL (mirror sqlite/postgres) or, if MySQL is unsupported, remove it from build / return a clear startup error rather than silently denying all traffic. Also: consider failing CLOSED on store errors in the rate-limit path (currently `err` is logged but `allowed` from a prior successful call is used) — see service.go:1586-1588 and 1596-1598.

---

### F5 — Global BodyLimit(16KB) breaks larger endpoints [HIGH]
**File:** `server/server.go:54`
```go
echoServer.Use(middleware.BodyLimit("16KB"))
```
This applies to **all** routes, including KB/Policy/Script file imports, `POST /api/v1/agent/:slug/import`, avatar/uploads, and any admin payload. A 16KB cap will reject legitimate large document uploads, breaking the admin import flow and potentially ticket/webhook payloads.

**Fix:** Apply the body limit only to the public unauthenticated group (or a named middleware on `chat/ext`, `chat/ext/transcript`). Authenticated/admin routes should have a higher or no limit. Example: register `BodyLimit` on the specific public echo.Group rather than `echoServer.Use(...)`.

---

### F6 — Off-by-one in rate limit [MEDIUM]
**File:** `store/db/sqlite/agent.go:1309` / `postgres/agent.go:1212` / `service.go:1585`
```sql
RETURNING CASE ... WHEN request_count <= ? THEN 1 ELSE 0 END
```
After the increment, `request_count` can equal `rpm+1` and still be allowed (since the check is `<= rpm`). So `rpm` permits `rpm+1` requests per window. Minor (`rpm=60` → 61), but it means the cap is not exact and compounds with the global cap.

**Fix:** Compare `request_count < rpm` (i.e., allow only if strictly under limit before increment) or check `< rpm` after a pre-increment semantics. Decide and document the exact semantics.

---

### F7 — Empty-key constant-time compare passes [MEDIUM]
**File:** `handlers.go:421` + F1
`subtle.ConstantTimeCompare([]byte(widgetKey), []byte(tenant.WidgetKey))` returns 1 when BOTH are empty. An attacker sending no `X-Widget-Key` (empty) against a tenant with empty `WidgetKey` passes the comparison. This is the concrete exploit path for F1.

**Fix:** Fail-closed gate (F1) removes this: require non-empty expected key; if `tenant.WidgetKey == ""` → 403 (or generate one). Never compare two possibly-empty secrets as "match = authorized".

---

### F8 — No transcript token grace period (breaks in-flight visitors) [MEDIUM]
**File:** `service.go:1617-1618` (mints with WidgetKey); `handlers.go:520` (verifies with WidgetKey)
Tokens issued before deploy were signed with GUID. After deploy, `verifySessionToken(..., tenant.WidgetKey)` rejects all of them immediately. Any visitor mid-conversation at deploy time loses transcript access (the widget polls `chat/ext/transcript` to resume). The plan mentioned a grace period; **it is not implemented**.

**Fix:** Either (a) accept tokens signed with the old GUID for a short, configurable grace window (dual-verify: try WidgetKey, fall back to GUID if within grace), or (b) document that in-flight sessions are invalidated on deploy and force widget reload. Don't silently break them.

---

### F9 — Global cap hardcoded [LOW]
**File:** `service.go:1594` `const globalTenantRPM = 300`
Duplicates the store default (sqlite/agent.go:1329). Not per-tenant configurable. Acceptable short-term but should be tenant-configurable (and consistent with the per-IP `RateLimitRPM`).

---

### F10 — Local-time window math [LOW/INFO]
**File:** `sqlite/agent.go:1299` `julianday(?)-julianday(window_start)`
`time.Now()` is local; stored `window_start` is local. Consistent, but DST transitions can make a 60s window 3600s different at the boundary (rare, self-healing). Postgres uses `EXTRACT(EPOCH FROM ($5::timestamp - window_start))` which is correct for absolute intervals. Low risk; note for completeness.

---

### F11 — CORS wildcard / origin trust [INFO]
Public endpoints still reflect/allow `*` origins (flagged follow-up in plan). With F1/F3 fixed (fail-closed key), this is less critical, but combined with the domain-allowlist opt-in it remains a defense gap. Out of core scope per plan.

---

## Answers to the Prompt Checklist (concise)

1. **Widget Key Bypass**
   - Extractable from `embed.js` (DevTools) → yes, documented obfuscation-grade. Real boundary = rate caps.
   - Visible in network tab → yes, `window.AgentChatConfig.widgetKey`. Accepted risk.
   - Constant-time? YES (`subtle.ConstantTimeCompare`, handlers.go:421) — but see F7.
   - Empty `WidgetKey` → **skipped check (FAIL-OPEN), F1/F7**. MUST fail-closed.
   - Brute force: UUID v4 = 122 bits, sufficient IF gate is enforced.
   - `embed.js` always includes key (handlers.go:1706) — good.

2. **Rate Limit Atomicity**
   - Atomic upsert present; TOCTOU removed. ✓
   - `RETURNING CASE` returns integer 1/0 → scanned into Go `bool` ✓ (driver converts).
   - `julianday` local-tz, DST edge (F10).
   - Postgres `$5::timestamp` parameterized ✓ (no injection).
   - Sentinel `__tenant_global__` collide with real IP? A client spoofing `X-Forwarded-For: __tenant_global__` could hit the global row — but `c.RealIP()` is server-controlled, not user-trusting the header naively (verify RealIP config). Low.
   - UNIQUE constraint present ✓ (LATEST.sql:394).

3. **Session Turn Cap**
   - Checked BEFORE increment (service.go:1608, increment at 1924) ✓ order correct.
   - Rotating `session_id` bypasses it → by design; global cap is the real boundary ✓.
   - `>= 50` → 50th message: `MessageCount` starts 0, after 49 messages count=49 <50 allowed; 50th allowed (count becomes 50 post-increment); 51st blocked. So 50 messages allowed. Acceptable, document.

4. **Transcript HMAC Rekey**
   - Now uses `WidgetKey` ✓ (handlers.go:520, service.go:1618).
   - Old GUID tokens → immediately invalid, NO grace period (F8) ✗.
   - HMAC-SHA256 sound ✓.
   - Transcript endpoint uses WidgetKey ✓.

5. **BodyLimit / Message Length**
   - Applied GLOBALLY → breaks uploads (F5) ✗. Should scope to public group.
   - Echo returns 413 on exceed ✓.
   - Default 2000 — check callers; `MaxMessageLength` configurable.
   - BodyLimit reads Content-Length / actual body; chunked also enforced by Echo ✓.

6. **Global Tenant Rate Limit**
   - Hardcoded 300 (F9) — should be configurable.
   - Sentinel creates 1 extra row/tenant/audience — negligible pollution ✓.
   - Per-IP + global both atomic independently; both must pass (AND) → cannot both falsely pass; they can both deny. Safe.

7. **Edge Cases**
   - `X-Widget-Key: ""` vs non-empty tenant key → compare fails → 403 ✓ (only unsafe when tenant key empty, F1).
   - `AllowedDomains` + `WidgetKey` both run, key first then domain ✓ order fine.
   - SQLite write-lock exhaustion: many concurrent INSERTs serialize on DB lock; bounded by Fly concurrency 25. Acceptable but note.
   - `embed.js` cache `max-age=3600` (handlers.go:1709): after key rotation, stale cached widget sends old key → 403 until cache expires. Add cache-busting or shorten TTL / version the key.
   - Global BodyLimit breaks large-body endpoints (F5).

8. **Test Coverage** — `bridge_delivery_test.go` updated (widget key header tests present, lines 845/883/955). Need explicit tests for: atomic TOCTOU race, global cap under IP rotation, session turn cap, body>16KB→413, message>2000→400, old-GUID transcript token→403. Verify these exist; if not, add.

9. **SQL Correctness**
   - Parameter order for sqlite `CheckAndIncrement`: `(tenantID, audienceType, clientIP, now, now, windowSeconds, rpm, now, windowSeconds, now, now, windowSeconds, rpm)` — 13 args for 13 `?` ✓.
   - `RETURNING CASE ... END` integer → Go `bool` scan ✓.
   - Postgres `$5::timestamp` safe ✓.
   - INSERT...ON CONFLICT works with no existing row ✓ (fresh insert path).
   - DST/leap-second: minor (F10).

10. **Security Properties**
    - **Fail-closed:** NOT met for widget key (F1/F3) — this is the headline failure.
    - Least privilege: widget key is reasonable minimum ✓.
    - Defense in depth: if key bypassed (it is, for old tenants), rate caps still hold ✓; if rate bypassed, turn cap holds ✓.
    - No PII leakage: error messages generic ("Access denied") ✓.
    - Timing: key compare constant-time ✓; HMAC verify constant-time (standard lib) ✓.

---

## Required Fixes Before Ship (blocking)
1. **F1/F7** — Make widget-key gate fail-CLOSED (deny when key missing/empty; never skip).
2. **F2** — Add `ALTER TABLE` migration for `widget_key` (existing DBs).
3. **F3** — Backfill `widget_key` for existing tenants in migration/startup.
4. **F4** — Implement MySQL rate-limit or fail loudly at startup; don't deny all traffic.
5. **F5** — Scope `BodyLimit` to public endpoints only.
6. **F8** — Add transcript token grace period (dual-verify GUID→WidgetKey) or document mid-session breakage.

## Recommended (non-blocking)
- F6 off-by-one semantics, F9 per-tenant global cap, F10 tz note, embed.js cache-bust on rotation, add missing unit tests (§8).
