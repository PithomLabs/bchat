# Adversarial code review per prompt_code_review.md

**Session ID:** ses_0c7517636ffe6zzIUUZqVlDP7F
**Created:** Invalid Date
**Updated:** 7/7/2026, 2:45:01 AM

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

