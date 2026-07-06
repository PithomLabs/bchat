# Adversarial Code Review (Round 2) — plan_025 Follow-Up Fixes

**Reviewer:** Senior Go/TS security engineer (adversarial)
**Mode:** Every input is hostile. Looking for bypasses, regressions, missed edge cases.
**Code state:** Uncommitted working-tree changes on `bchat0534`. Reviewed against `prompt_code_review2.md`.
**Verdict:** NOT SAFE TO SHIP — 1 blocking (startup-availability) + 1 important (permanent token forgery) must be fixed first.

---

## Verdict by Finding

| # | Checklist | File(s) | Verdict |
|---|-----------|---------|---------|
| R1 | Migration idempotency (Q1) | `0.29/01__add_widget_key.sql`, `0.29/02__add_max_message_length.sql` | **REGRESSION/robustness risk (HIGH for prod)** |
| R2 | Transcript grace period (M2, Q7) | `handlers.go:522-526` | **PARTIAL — unbounded, permanent forgery (MEDIUM-HIGH)** |
| R3 | Fail-closed gate (H2/M4) | `handlers.go:415-427` | **CORRECT** |
| R4 | Empty/whitespace key bypass (Q3 sec1) | `handlers.go:420-424` | **CORRECT** (empty rejected; whitespace != key) |
| R5 | Constant-time compare (Q8) | `handlers.go:424` | **CORRECT** (`subtle` returns 0 on length mismatch) |
| R6 | Rate-limit off-by-one (C2) | sqlite/postgres `agent.go` | **CORRECT** (`request_count < rpm`) |
| R7 | Postgres `::timestamp` casts (M3) | postgres `agent.go` | **CORRECT** (removed; driver sends `timestamp`) |
| R8 | BodyLimit scope (H1) | `v1.go:254` / `server.go:53` | **CORRECT** (scoped to publicGroup; admin imports unrestricted) |
| R9 | MySQL stubs (H3, Q3) | mysql `agent.go:224-233` | **ACCEPTED w/ caveat** (fail-open; DoS vector) |
| R10 | Widget key in client (C1) | `embed.ts`, `api.ts` | **CORRECT** |
| R11 | Playground gate (M1) | `playground.go:537-545` | **CORRECT** |
| R12 | Schema alignment (§10) | LATEST.sql, Go default 2000 | **CORRECT** |
| R13 | Version bump / tests (§11) | `version.go`, `migrator_test.go` | **CORRECT (verify CI)** |
| R14 | New attack surfaces (§12) | v1.go routes | **MOSTLY CORRECT** (one note: `/file/*` + gRPC-web CORS) |

---

## Detailed Findings

### R1 — Migration ALTER is NOT safe to re-run [HIGH — availability/startup]
**Files:** `store/migration/sqlite/0.29/01__add_widget_key.sql:2`, `store/migration/postgres/0.29/01__add_widget_key.sql:2`, `.../02__add_max_message_length.sql:2`

`ALTER TABLE agent_tenants ADD COLUMN widget_key TEXT;` has no existence guard. Neither SQLite nor Postgres support `ADD COLUMN IF NOT EXISTS`. The migrator (`store/migrator.go:91-106`) applies each file inside one transaction; the whole batch aborts on the first error, and the app refuses to start.

**Exploit / failure scenario (real, not theoretical on Fly.io):**
1. Deploy runs migration; `ALTER` succeeds, backfill succeeds, but the **commit or the `UpsertMigrationHistory` fails** (pod killed / Fly reschedule / transient DB error). `migration_history` is NOT updated to 0.29.x.
2. Next start: `schemaVersion (0.29.3) > latestMigrationHistoryVersion (≈0.29.0)` → migrator re-runs ALL 0.29 files.
3. `ALTER TABLE ... ADD COLUMN widget_key` → **"duplicate column"** (SQLite) / **"column already exists"** (Postgres) → entire migration fails → **app never starts** → full outage.

The `UPDATE ... WHERE widget_key IS NULL` portion is re-run-safe (idempotent), but the `ALTER` is not. Same for `02__add_max_message_length.sql`.

**Fix:** Make each ALTER re-run-safe:
- **SQLite:** check via `PRAGMA table_info(agent_tenants)` before altering, or wrap in a guard:
  ```sql
  -- SQLite: no IF NOT EXISTS for ADD COLUMN; guard with a temp check
  INSERT INTO sqlite_migration_guard ... -- or run a prior SELECT and skip in Go
  ```
  Simplest robust approach: in `migrator.go`, split `execute()` to tolerate "duplicate column"/"already exists" errors for idempotent-safe migrations, OR have the migration file use a `DO $$ ... END $$` (PG) / per-driver guard. For SQLite, a common pattern:
  ```sql
  -- 01__add_widget_key.sql (sqlite)
  CREATE TABLE IF NOT EXISTS _guard_widget_key AS SELECT 1 WHERE 0;
  -- actually: check column existence is not possible in pure SQL portably -> do it in Go
  ```
  **Recommended:** add a helper in `migrator.go` `executeIdempotent()` that, for SQLite, inspects `PRAGMA table_info` and skips; for Postgres, uses `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` (Postgres 9.6+ supports it). This removes the startup-outage risk.

**Why blocking:** On Fly.io with `auto_stop`/`min_machines_running = 0` and possible crashloop on partial migration, a non-idempotent ALTER is a production outage waiting to happen.

---

### R2 — Transcript grace period is UNBOUNDED [MEDIUM-HIGH — permanent token forgery]
**File:** `handlers.go:521-526`
```go
expiry, err := verifySessionToken(token, sessionID, expiryStr, tenant.WidgetKey)
if err != nil && tenant.GUID != "" {
    expiry, err = verifySessionToken(token, sessionID, expiryStr, tenant.GUID)
}
```
The fallback to `tenant.GUID` has **no time limit**. `GUID` is PUBLIC (exposed in `widget.js` and `/config`). Consequences:
- Any attacker who knows a tenant's `GUID` (trivially obtainable — it's in the public embed script) can **forge transcript HMAC tokens for ANY session ID, forever** — even months after migration — because the grace fallback never expires.
- This re-opens the exact finding (T5 / original M2) the rekey was supposed to close, for all tenants that ever had a GUID (i.e., all of them).

The plan's "grace period" implied a time box; the implementation has none. This is a permanent transcript-disclosure vector (transcripts can contain visitor PII, emails, phones).

**Fix (choose one):**
- **(a) Time-box the grace:** store a `rekey_timestamp` (or use deploy time) and only accept GUID-signed tokens whose `expiry` is before a cutoff (e.g., tokens minted before the migration). Since transcript tokens are short-lived (30 min), a grace of e.g. 24–48h after deploy suffices; after that, reject GUID entirely.
- **(b) Remove the GUID fallback** and instead re-mint visitor transcript tokens on the client at deploy (force widget reload). Simpler and fully closes the hole.
- At minimum, document that the grace is permanent and accept the residual risk — but given GUID is public, this should be blocking for PII-sensitive tenants.

---

### R3/R4/R5 — Gate is fail-closed and correct [CORRECT]
`handlers.go:416-427`: rejects when `tenant.WidgetKey == ""` (fixes F1/F3), rejects empty incoming key, uses `subtle.ConstantTimeCompare`. `subtle.ConstantTimeCompare` returns `0` for unequal lengths, so an empty attacker key vs a non-empty tenant key → 0 → 403. Whitespace-only `" "` != tenant key → 403. Good. Gate runs before bind/LLM. Error messages are generic ("Access denied") — no leakage. ✓

### R6 — Rate-limit off-by-one fixed [CORRECT]
Both sqlite (`agent.go:1309,1342`) and postgres (`agent.go:1212,1243`) now use `request_count < ?` in RETURNING and the UPDATE increment guard. Trace rpm=5: req1→count 0<5 →allowed, count=1; ... req5→count4<5→allowed,count=5; req6→count5 not <5→denied, count stays 5. Exactly 5 allowed. ✓ No TOCTOU (single atomic statement). ✓

### R7 — Postgres casts removed [CORRECT]
`$5` used directly (no `::timestamp`); `lib/pq`/`pgx` sends `time.Time` as `timestamp with time zone`/timestamp; `EXTRACT(EPOCH FROM ($5 - window_start))` is valid. ✓

### R8 — BodyLimit scoped correctly [CORRECT]
`server.go:53` global removed; `v1.go:254` adds `middleware.BodyLimit("16KB")` to `publicGroup` only. `publicGroup` includes `chat/ext`, `chat/ext/transcript`, `playground/run`, `playground/catalog`, `widget.js`. Admin `import`, `reindex`, etc. are on `adminGroup` (no limit) → large file uploads work. ✓ A 20KB public chat body still hits 413. ✓
**Minor note:** `playground/catalog` (GET, line 255) is public & unauthenticated but read-only; acceptable.

### R9 — MySQL fail-open [ACCEPTED w/ caveat]
`mysql/agent.go:224-233` returns `(true, nil)` with `slog.Warn`. This is the documented least-bad option for unsupported MySQL. **Caveat (Q3 in prompt):** it creates an unlimited-request DoS/cost vector on MySQL deployments — no rate limit at all. Since MySQL is explicitly unsupported, acceptable, but the warning must be visible in logs/monitoring. Recommend also logging the tenant slug for diagnosability. Not blocking.

### R10 — Widget key reaches the HTTP header [CORRECT]
`embed.ts:104` (mergeConfig) and `:135` (initWithConfig) copy `widgetKey` from global + script config; `api.ts:17-18` sends `X-Widget-Key` only when `config.widgetKey` set. If undefined in both → header omitted → server returns 403 (fail-closed). Data flow `window.AgentChatConfig.widgetKey → mergeConfig → Widget → api.ts → header` verified. Visible in DevTools (documented obfuscation-grade). ✓

### R11 — Playground gate [CORRECT]
`playground.go:537-545`: identical fail-closed check, `crypto/subtle` imported, rejects empty tenant key and empty incoming key. ✓

### R12 — Schema alignment [CORRECT]
sqlite LATEST `widget_key TEXT` (line 206) + index; `max_message_length INTEGER DEFAULT 2000` (line 235). postgres LATEST `widget_key TEXT` (143) + `max_message_length INTEGER NOT NULL DEFAULT 2000` (170). Go defaults `MaxMessageLength<=0 → 2000` (service.go:1569). Consistent. ✓

### R13 — Version / tests [CORRECT, verify in CI]
`version.go` 0.29.0; `migrator_test.go` updated. `bridge_delivery_test.go` already exercises widget-key header (valid + invalid). **Action:** run `go test ./store/... ./server/... ./widget/...` before ship to confirm no version/assertion regressions. The prompt asks to confirm `TestGetCurrentSchemaVersion` and any `"0.28"` string assertions pass — verify locally (not done here).

### R14 — New attack surfaces [MOSTLY CORRECT]
- Playground timing side-channel: identical constant-time compare; negligible. ✓
- Grace period window: covered in R2 (unbounded — the real issue).
- MySQL DoS: R9.
- **BodyLimit bypass via auth group:** `chat/ext` is registered ONLY on `publicGroup` (v1.go:256). It is NOT on `authGroup` or `adminGroup`, and those require JWT. The gRPC-gateway `gwGroup.Any("/api/v1/*", handler)` (v1.go:199) is registered AFTER `RegisterAgentRoutes` (line 190) and only proxies declared gRPC methods (no `chat/ext`), so it cannot shadow/bypass the gated route. ✓ No bypass.
- **Residual note (out of core scope):** `gwGroup` uses permissive `middleware.CORS()` and exposes `/file/*` and gRPC-web with `WithCorsForRegisteredEndpointsOnly(false)` (v1.go:204). Not part of the widget-key work but worth a follow-up audit; public CORS `*` on `/file/*` could allow cross-origin resource reads if any file endpoint is unauthenticated.

---

## Required Fixes Before Ship (blocking)
1. **R1** — Make the `0.29` ALTER migrations re-run-safe (guard `ADD COLUMN` for SQLite via `PRAGMA` check in Go `executeIdempotent`, and use `ADD COLUMN IF NOT EXISTS` for Postgres). Prevents permanent startup outage on partial migration on Fly.io.
2. **R2** — Bound or remove the transcript GUID grace fallback. A public `GUID` + unlimited fallback = permanent transcript-token forgery. Add a cutoff timestamp (e.g., reject GUID-signed tokens whose `expiry` is past deploy+48h) or drop the fallback.

## Recommended (non-blocking)
- R9: enrich MySQL warning with tenant slug; monitor for the warning.
- R14: follow-up audit of `/file/*` + gRPC-web CORS (not introduced by this change, but now adjacent to hardened public surface).
- R13: run full test suite in CI; confirm no stale `0.28` version assertions.

## What Was Done Well
The core fail-open (F1/F3/F7) is correctly closed; the atomic rate-limit + off-by-one fix is sound across both engines; BodyLimit scoping is correct and doesn't break admin uploads; constant-time comparison and client-key propagation are implemented properly; schema/Go defaults are aligned. This is a solid second round — the two remaining items are about production robustness (migration idempotency) and a subtle but permanent authz hole (unbounded grace).
