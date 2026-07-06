# Adversarial Code Review Prompt — Plan 025 Follow-Up Fixes

**Reviewer:** Senior Go/TypeScript security engineer (adversarial)
**Mode:** Every input is hostile. Looking for bypasses, regressions, and missed edge cases.
**Scope:** All changes made to fix findings from the combined three-review cross-reference.

---

## Context

Plan 025 implemented widget-key edge gating, atomic rate limits, transcript HMAC rekey, and input hygiene for unauthenticated public chat endpoints. Three independent adversarial reviews (custom, Stepfun, Hy3) cross-validated against the codebase identified 11 confirmed findings (C1-C4, H1-H4, M1-M4). The implementation below addresses all of them.

**Original plan:** `bugs/025/plan_025.md`
**Combined findings:** `.local/share/opencode/plans/combined_review_findings.md`
**Prior reviews:** `bugs/025/review_025_adversarial.md` (Stepfun), `bugs/025/025_code_review_hy3.md` (Hy3)

---

## Changed Files

Review every file listed below. For each, verify the fix is correct, complete, and introduces no new vulnerabilities.

### Version & Migrations
1. `internal/version/version.go` — Version bumped to 0.29.0
2. `store/migration/sqlite/0.29/01__add_widget_key.sql` — ALTER TABLE + UUID backfill
3. `store/migration/postgres/0.29/01__add_widget_key.sql` — ALTER TABLE + UUID backfill
4. `store/migration/postgres/0.29/02__add_max_message_length.sql` — ALTER TABLE
5. `store/migration/postgres/LATEST.sql` — Added widget_key + max_message_length columns
6. `store/migration/sqlite/LATEST.sql` — Changed max_message_length default 4000→2000
7. `store/test/migrator_test.go` — Updated version assertion to 0.29.x

### Backend Go
8. `server/router/api/v1/agent/handlers.go` — Fail-closed widget key gate + transcript grace period
9. `server/router/api/v1/agent/playground.go` — Widget key gate on playground endpoint
10. `server/router/api/v1/v1.go` — BodyLimit scoped to publicGroup
11. `server/server.go` — Removed global BodyLimit
12. `store/db/sqlite/agent.go` — Rate limit off-by-one fix (<=→<)
13. `store/db/postgres/agent.go` — Rate limit off-by-one fix + removed ::timestamp casts
14. `store/db/mysql/agent.go` — Rate limit stubs fail-open with warning

### Frontend TypeScript
15. `widget/src/embed.ts` — widgetKey merged in mergeConfig() and initWithConfig()

---

## Review Checklist

For each finding below, verify the fix is correct and complete. Then check for regressions and new vulnerabilities.

### 1. Migration correctness (C3+C4)
- Does `0.29/01__add_widget_key.sql` use `IF NOT EXISTS` for the index?
- Does the SQLite backfill generate valid UUID v4 format?
- Does the Postgres backfill use `gen_random_uuid()` correctly?
- Will the migration run idempotently (safe to re-run)?
- Does `LATEST.sql` for both sqlite and postgres now include `widget_key`?
- Does the Postgres `LATEST.sql` now include `max_message_length`?
- If an existing DB already has `widget_key` (e.g., from a manual ALTER), will the migration error?

### 2. Fail-closed gate (H2+M4)
- In `handlers.go`, does the gate reject when `tenant.WidgetKey == ""`?
- Does it reject when the incoming `X-Widget-Key` header is empty?
- Is `subtle.ConstantTimeCompare` still used (timing-safe)?
- Can an attacker bypass by sending `X-Widget-Key: ` (whitespace-only)?
- Is the gate applied BEFORE any LLM call or expensive operation?
- Does the error message leak any information about why access was denied?

### 3. Widget key in client (C1)
- In `embed.ts`, does `mergeConfig()` copy `widgetKey` from `globalConfig`?
- In `embed.ts`, does `initWithConfig()` copy `widgetKey` from `scriptConfig`?
- Does `api.ts` still send the `X-Widget-Key` header when `config.widgetKey` is set?
- If `widgetKey` is undefined in both sources, does it remain undefined (safe — server rejects)?
- Is the widget key visible in `window.AgentChatWidget.config` in browser DevTools? (acceptable — documented obfuscation-grade)

### 4. Rate limit off-by-one (C2)
- In sqlite `CheckAndIncrementAgentRateLimit`: is RETURNING `request_count < ?` (not `<=`)?
- In sqlite `CheckAndIncrementTenantGlobalRateLimit`: same fix applied?
- In postgres `CheckAndIncrementAgentRateLimit`: same fix applied?
- In postgres `CheckAndIncrementTenantGlobalRateLimit`: same fix applied?
- Trace the logic: at exactly `rpm` requests, does the  request get denied?
- Is the UPDATE clause still `request_count < ?` (stop incrementing at limit)?

### 5. Postgres timestamp cast (M3)
- Are all `::timestamp` casts removed from both postgres rate limit functions?
- Does the SQL still work without the cast? (PG driver sends time.Time as timestamp)
- Are there any other `::timestamp` casts remaining in the postgres rate limit code?

### 6. BodyLimit scope (H1)
- Is `middleware.BodyLimit("16KB")` removed from `server.go`?
- Is it added to `publicGroup` in `v1.go`?
- Does `publicGroup` include the chat/ext, transcript, and playground routes?
- Does the admin/auth group NOT have the 16KB limit?
- Can a 50KB file upload succeed on `HandleImportSingleFile`?
- Can a 20KB chat request still hit the 413 on public endpoints?

### 7. MySQL stubs (H3)
- Do `CheckAndIncrementAgentRateLimit` and `CheckAndIncrementTenantGlobalRateLimit` now return `(true, nil)`?
- Is `slog` imported?
- Does the warning log include enough context to diagnose?
- Is fail-open the right choice here? (Consider: MySQL deployments are unsupported; failing closed would break all traffic; fail-open with warning is the least-bad option)

### 8. Playground widget key (M1)
- Does `HandlePlaygroundRun` now check the widget key?
- Is the check identical to the one in `HandleChatExternal`?
- Does it use `subtle.ConstantTimeCompare`?
- Is `crypto/subtle` imported?
- Does it reject empty `tenant.WidgetKey`?
- Does it reject empty incoming key?

### 9. Transcript grace period (M2)
- Does `HandleGetExternalTranscript` try WidgetKey first, then fall back to GUID?
- Is the fallback conditional on `tenant.GUID != ""`?
- If both WidgetKey and GUID fail, is the request rejected?
- Is there a time limit on the grace period? (Should there be?)
- Can an attacker exploit the fallback to use an old GUID key indefinitely?

### 10. Schema alignment
- Does `LATEST.sql` for sqlite have `max_message_length INTEGER DEFAULT 2000`?
- Does `LATEST.sql` for postgres have `max_message_length INTEGER NOT NULL DEFAULT 2000`?
- Does the Go code default to 2000 when `MaxMessageLength <= 0`?
- Are there any remaining mismatches between DB schema and Go defaults?

### 11. Regressions
- Does the version bump from 0.28.0 to 0.29.0 break anything?
- Do all existing tests still pass?
- Is the `bridge_delivery_test.go` still valid (it uses `widgetKey` in test requests)?
- Does the `TestGetCurrentSchemaVersion` test pass with the new version?
- Are there any other tests that assert on the version string "0.28"?
- Does the `embed.js` config injection (`HandleWidgetEmbed`) still work (it injects `widgetKey` into the script)?

### 12. New attack surfaces
- Does the playground widget key check introduce a new timing side-channel?
- Does the transcript grace period extend the window for token forgery?
- Does the MySQL fail-open create a denial-of-service vector (unlimited requests)?
- Does moving BodyLimit to publicGroup create a bypass (e.g., sending chat/ext via auth group)?

---

## Specific Questions to Answer

1. **Is the migration idempotent?** If run twice on the same DB, does it error or succeed silently?
2. **Is the backfill safe for large tenant tables?** The `UPDATE ... WHERE widget_key IS NULL` runs in a transaction with the ALTER — is this safe for production?
3. **Does the Postgres driver actually send `time.Time` as `timestamp` without the cast?** Verify with the `pgx` or `lib/pq` driver documentation.
4. **Is the `embed.ts` fix sufficient?** Does the widget actually send `X-Widget-Key` in the browser after this fix? Trace the data flow from `window.AgentChatConfig` → `mergeConfig()` → `Widget` → `api.ts` → HTTP header.
5. **Is the rate limit semantics now correct?** With `request_count < rpm` in both UPDATE and RETURNING, trace the exact behavior for rpm=5: requests 1-5 allowed, request 6 denied.
6. **Can an attacker bypass the widget key by calling the internal chat endpoint?** `HandleChatInternal` is behind auth — verify it's not accessible without a valid JWT.
7. **Does the transcript grace period have an expiration?** If not, should it? An old GUID key would be valid forever.
8. **Is the `constant-time` comparison actually constant-time for unequal-length inputs?** `subtle.ConstantTimeCompare` returns 0 for different lengths — verify this is the case.

---

## Verdict Format

For each finding, report:
- **CORRECT** — Fix is correct and complete
- **PARTIAL** — Fix addresses the issue but has gaps
- **REGRESSION** — Fix introduces a new bug or vulnerability
- **MISSED** — Original finding was not fully addressed

Final verdict: **SAFE TO SHIP** or **NOT SAFE TO SHIP** (with blocking issues listed)
