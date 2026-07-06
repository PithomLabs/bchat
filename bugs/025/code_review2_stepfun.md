# Adversarial code review per prompt_code_review.md

**Session ID:** ses_0c7517636ffe6zzIUUZqVlDP7F
**Created:** Invalid Date
**Updated:** 7/7/2026, 3:33:50 AM

---

## User

do adversarial code review per bchat/bugs/025/prompt_code_review.md

---


## Assistant

Review is complete. I traced every checklist section against the actual plan_025 implementation and wrote the adversarial report to:

`bchat/bugs/025/review_025_adversarial.md`

## Verdict: **REJECTED — 2 High, 3 Medium findings**

### High Severity (Blocking)

| Finding | File:Line | Summary |
|---------|-----------|---------|
| **FINDING-1** | `handlers.go:416-425` | **Empty `WidgetKey` = fail-open.** If a tenant has no `widget_key` (migration not backfilled), the edge gate check is skipped entirely. Attacker calls `chat/ext` unauthenticated. |
| **FINDING-2** | `server.go:54` | **Global `BodyLimit("16KB")` breaks admin file uploads.** Authenticated endpoints like `HandleImportSingleFile` accept multipart files; any file > 16KB now gets 413 before the handler runs. |

### Medium Severity

| Finding | File:Line | Summary |
|---------|-----------|---------|
| **FINDING-3** | `playground.go:521-580` | **Playground endpoint has NO widget key check.** It calls `ChatExternal` directly, bypassing the edge gate entirely. |
| **FINDING-4** | `handlers.go:166-173` | **No grace period for key rotation.** Rotating `WidgetKey` breaks cached `embed.js` for up to 1 hour (`max-age=3600`) with no fallback/prev-key mechanism. |
| **FINDING-5** | `postgres/agent.go:1189-1222` | **Fragile `$5::timestamp` cast.** Works today but breaks if driver binding behavior changes; recommend removing the explicit cast and relying on parameterized `timestamptz` binding. |

### What Is Working Well
- Atomic `INSERT ... ON CONFLICT DO UPDATE ... RETURNING` SQL — no TOCTOU race
- `subtle.ConstantTimeCompare` for widget key
- `hmac.Equal` for transcript HMAC verification
- Session turn cap correctly at `>= 50` (50 turns allowed, 51st denied)
- PostgreSQL EXTRACT(EPOCH FROM ...) is parameterized and injection-safe
- `UNIQUE(tenant_id, audience_type, client_ip)` constraint is present

### Notable Gap
- **No widget-key validation tests exist** for missing/invalid/empty key scenarios. Test coverage is a follow-up concern across all 7 listed scenarios.

The full report with exploit scenarios and recommended fixes is in `bchat/bugs/025/review_025_adversarial.md`.

---

## User

do adversarial code review per bchat/bugs/025/prompt_code_review2.md

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: todowrite**

---

## Assistant

**Tool: bash**

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: read**

**Tool: read**

**Tool: read**

**Tool: bash**

**Tool: bash**

---

## Assistant

**Tool: bash**

**Tool: bash**

**Tool: bash**

**Tool: bash**

**Tool: bash**

---

## Assistant

**Tool: grep**

**Tool: bash**

**Tool: bash**

**Tool: bash**

---

## Assistant

**Tool: bash**

**Tool: bash**

**Tool: bash**

**Tool: bash**

---

## Assistant

**Tool: read**

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: bash**

**Tool: bash**

**Tool: bash**

**Tool: read**

---

## Assistant

**Tool: read**

**Tool: bash**

**Tool: bash**

**Tool: bash**

---

## Assistant

**Tool: bash**

**Tool: bash**

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: grep**

**Tool: grep**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: todowrite**

---

## Assistant

The adversarial v2 review is complete. Report written to:

`bchat/bugs/025/review_025_adversarial_v2.md`

## Verdict: **NOT SAFE TO SHIP** — 3 blocking findings remain

### Key Findings

| # | Verdict | Finding |
|---|---------|---------|
| 1 | **REGRESSION** | **Rate limit off-by-one:** Both UPDATE and RETURNING use `< rpm`. With rpm=5, only **4 requests** are allowed. The RETURNING clause must use `<=` instead of `<`. Affects all 4 rate-limit SQL functions (SQLite + Postgres, per-IP + global). |
| 2 | **REGRESSION** | **Iframe embed broken:** `HandleWidgetIframe` (`handlers.go:2135`) generates `window.AgentChatConfig` **without `widgetKey`**. Widgets embedded via iframe will fail with `403 Access denied` because no `X-Widget-Key` header is sent. |
| 3 | **PARTIAL** | **Transcript grace period is unbounded:** The fallback to `tenant.GUID` has **no time limit**. A previously-exposed GUID (from old `widget.js`) can forge transcripts forever, even after key rotation. |
| 4 | **PARTIAL** | **Migration not idempotent:** `ALTER TABLE ADD COLUMN` will error on manual re-run. `CREATE INDEX IF NOT EXISTS` is safe, but the ALTER is not. |
| 5 | **INFO** | **MySQL logs lack context:** Warning logs omit `tenant_id` and `client_ip`, making abuse diagnosis impossible. |

### What Was Fixed Correctly ✅
- Fail-closed widget key gate (empty `WidgetKey` → 403)
- Global `BodyLimit("16KB")` moved to `publicGroup` only
- Playground endpoint now validates widget key
- Postgres `::timestamp` casts removed
- Schema aligned (`max_message_length` default 2000)
- Version bumped to 0.29.0

### Recommended Fixes Before Merge
1. **Rate limit SQL:** Change `RETURNING ... WHEN request_count < ?` → `WHEN request_count <= ?` in all 4 functions.
2. **Iframe HTML:** Add `widgetKey: '%s'` to `generateIframeHTML` and pass it through `HandleWidgetIframe`.
3. **Grace period:** Add a cutoff or rotation timestamp so GUID fallback expires.

---

