# Final Adversarial Review: plan4.md

**Reviewer:** AI (adversarial review mode — final pass)
**Date:** 2026-07-10
**Document:** `bugs/033/plan4.md` — bchat Native Integrations, v1 (tenant-provided Twilio credentials, lease-based outbox)
**Previous reviews:** `plan_biz_review.md`, `plan2_review.md`, `plan3_review.md`
**Codebase facts verified against:** `store/driver.go` (Driver interface includes agent methods), `store/db/mysql/agent.go` (implements 87 Driver methods), `go.mod` (modernc.org/sqlite v1.37.1), plus all facts carried from the plan3 review.

---

## Verdict: ✅ APPROVED — FINAL

This is the last review. plan4 resolves all 3 criticals and all 6 majors from the plan3 review, and the scoping decisions (tenant-provided Twilio, MySQL dropped, post-insert outbox with documented failure window, Scheduler Story A) are exactly right for a minimum viable plan.

Four defects remain. **None warrants a plan5.** Each has a prescriptive fix below — fold amendments A–D into the implementation directly and treat this document as the addendum to plan4. The review cycle is closed.

---

## Resolution of plan3_review.md Items

| # | plan3 review item | Status in plan4 |
|---|-------------------|-----------------|
| 1 | C1. "Same transaction" outbox not implementable | ✅ Fixed — separate statement, failure window acknowledged up front |
| 2 | C2. Idempotency key design flawed | ✅ Fixed — deterministic `hash(tenantID, leadID, eventType)`, `23505`-as-success in wrapper, `TEXT` type, dedicated test |
| 3 | C3. Path A contradiction | ✅ Fixed — honestly rescoped as "tenant-provided Twilio credentials v1", provisioning deferred to v2 |
| 4 | M1. MySQL migrations incoherent | ✅ Fixed (migrations) / ⚠️ new compile issue — see Amendment A |
| 5 | M2. No stale-claim recovery | ✅ Fixed — `claimed_at` lease, 5-min reclaim in the claim query, at-least-once documented |
| 6 | M3. Graceful shutdown SIGKILLed | ✅ Fixed — `kill_timeout = 30`, `Server.Shutdown` → `Service.Stop()` wiring, correctly noted as an optimization |
| 7 | M4. Trigger endpoint unauthenticated | ✅ Fixed — `X-Cron-Token`, `202 Accepted`, single-flight mutex |
| 8 | M5. Scheduler story contradiction | ✅ Fixed — Story A committed, immediate post-commit dispatch + poller as catch-up / ⚠️ introduces a race — see Amendment B |
| 9 | M6. Retry wrapper blast radius | ✅ Fixed — scoped to reads / keyed writes / whole units outside transactions |
| 10 | Nits N1–N6 | ✅ Mostly fixed (vendored cron, endpoints re-enumerated incl. `/test` and `/events`, SMS on `*Service`, STOP handling explicitly deferred to Twilio Advanced Opt-Out, new tests added) |

---

## Required Amendments (fold into implementation — no plan revision needed)

### A. Adding methods to `store.Driver` without MySQL stubs breaks compilation

**Verified fact:** the `Driver` interface in `store/driver.go` includes the agent methods, and `store/db/mysql/agent.go` implements them (87 `func (d *DB)` methods today). MySQL was correctly dropped from *migration/feature* scope, but Go interface compliance is not optional: adding `ClaimPendingEvents`, `ClaimPendingSMS`, and the integration/event/SMS CRUD methods to the interface while only implementing them in sqlite + postgres means **the project stops compiling**.

**Fix (mechanical):** add `store/db/mysql/agent.go` (or a new `integrations.go` in that package) to the modified-files list, with one-line stubs returning `errors.New("agent integrations are not supported on MySQL")`. Follows the existing precedent — MySQL already compiles agent methods against tables its migrations never create.

### B. Immediate dispatch races the poller → easy double-delivery

Plan §3: event inserted (implicitly as `pending`), then a detached goroutine delivers it immediately; the cron poller claims `pending` rows. A webhook delivery can take up to 30s. If the poller fires inside that window, it claims the row while the immediate goroutine is mid-delivery — **two deliveries of the same event**, on a single machine, in the common case. At-least-once makes this tolerable, but it's gratuitous when the plan's own lease mechanism prevents it for free.

**Fix (one design line):** when immediate dispatch will be attempted, insert the event **pre-claimed**: `status = 'processing', claimed_at = now()`. The poller then ignores it for the 5-minute lease. If the goroutine succeeds it marks `delivered`; if the process dies mid-delivery, the lease expires and the poller redelivers — exactly the intended recovery path, now with no overlap window.

### C. No attempts cap — dead endpoints are retried every 5 minutes forever

plan2's `agent_events` schema had `attempts INTEGER` and a max-attempts policy; plan4's schema description lists only `idempotency_key` and `claimed_at`. As written, an event pointing at a permanently-dead webhook URL is reclaimed by the lease query every 5 minutes **indefinitely** — unbounded retries, log spam, and rows that never leave the queue.

**Fix:** keep `attempts INTEGER NOT NULL DEFAULT 0` (and `last_error TEXT`) in both the `agent_events` and `agent_sms_messages` schemas; increment on each claim; the claim query excludes `attempts >= 5`; after the 5th failure set terminal `status = 'failed'` (visible in the `/events` log UI). Add `TestMaxAttemptsTerminal` to the test list.

### D. Nothing ever enqueues an SMS — the poller polls an empty table

plan4 has `agent_sms_messages`, `ClaimPendingSMS`, and `SendSMS` on `*Service`, but the enqueue path vanished: plan2's template engine (§5.4 — `TwilioConfig.Templates` with `delay_hours`, triggered on lead capture) is absent from plan4. As written, no code path inserts a row into `agent_sms_messages`, so the SMS half of the feature is dead weight.

**Fix — choose one (both are valid MVP answers):**
1. **Minimal enqueue (recommended if SMS ships in v1):** on `lead.captured`, if a Twilio integration is active, iterate `TwilioConfig.Templates`, insert one `queued` row per enabled template with `scheduled_at = now + delay_hours` and idempotency key `hash(tenantID, leadID, templateName)`. ~30 lines in the same code path that inserts the event row.
2. **Cut scheduled SMS from v1 (recommended if truly minimum):** ship webhooks-only; drop migration `02__agent_sms.sql`, `sms.go`, `ClaimPendingSMS`, and the SMS UI card. Tenants get SMS today via a `lead.captured` webhook → Zapier → Twilio (the Path B fallback already works for this). SMS becomes v1.1 with the enqueue path specified.

Either resolves the finding; what's not acceptable is shipping the queue with no producer.

---

## MVP Cut List (optional, aligned with the "minimum viable" goal)

These are recommendations, not blockers — each removes work without removing v1 value:

1. **Drop `agent_sms_optouts` from the 0.31 migrations.** v1 explicitly relies on Twilio's carrier-side Advanced Opt-Out; an empty table with no reader or writer is schema debt. Migrations are versioned — add it in 0.32 when the inbound webhook lands. (Moot if Amendment D option 2 is taken.)
2. **Simplify graceful shutdown to `cron.Stop()` + `kill_timeout = 30`.** plan4 itself notes the lease reclaim makes graceful shutdown an optimization. The vendored fork's `Stop()` already returns a context that completes when running jobs finish — the additional `sync.WaitGroup` over individual in-flight HTTP calls buys nothing the 5-minute lease doesn't, and is the fiddliest code in the plan.
3. **Stage delivery: webhooks first.** Migrations 00/01, outbox + poller + trigger endpoint, integration CRUD, webhook UI card — that's a shippable, testable unit. SMS (if kept per Amendment D option 1) lands second on infrastructure that's already proven. Same total work, earlier signal.

---

## Nits (non-blocking, fix in passing)

1. **`EXTRACT(EPOCH FROM NOW())` returns `numeric`** — cast it (`EXTRACT(EPOCH FROM NOW())::BIGINT`) to match the `BIGINT` column, in both the SET and the lease comparison.
2. **Enumerate the v1 event types** (`lead.captured`, `escalation.created`). This matters because the deterministic key makes each (entity, event-type) pair once-ever: fine for these two, but a future `lead.updated` needs a discriminator (e.g., updated-at timestamp) in the hash — one sentence in the plan prevents a silent event-swallowing bug later.
3. **SQLite "emulation" is the same query.** modernc.org/sqlite v1.37.1 bundles SQLite ≥ 3.35, so `UPDATE ... RETURNING` works; the only real difference is dropping `FOR UPDATE SKIP LOCKED` (safe: single writer + the trigger endpoint's single-flight mutex). Say that instead of "emulate", so the implementer writes one query shape.
4. **Restate RBAC per endpoint** — plan2 §4.5 had the permission column (`tenant:admin` for writes, `tenant:read` for list/get/events); plan4's endpoint list dropped it. The trigger-cron endpoint is system-level (`X-Cron-Token`), the rest are tenant-scoped.
5. **`connect_timeout=15` is a config change, not a manual-verification step** — it belongs in the change list as an edit to the `DATABASE_URL` secret / DSN construction, alongside the `fly.toml` `kill_timeout` item, or it will be forgotten on every fresh deploy.

---

## Verification Plan Additions

Plan4's test list is good (`23505`-as-success, stale-claim reclaim, trigger auth). Add:

- **Compile gate:** `go build ./...` with all three drivers in-tree (catches Amendment A regressions permanently).
- **`TestImmediateDispatchNoDoubleClaim`:** insert pre-claimed event, run the poller, assert it is not re-claimed within the lease (Amendment B).
- **`TestMaxAttemptsTerminal`:** event with `attempts = 5` is excluded from claims and marked `failed` (Amendment C).
- If Amendment D option 1: **`TestSMSEnqueueOnLeadCaptured`** — lead capture with active Twilio integration inserts the expected `queued` rows with deterministic keys.

---

## Closing

**The review cycle is complete.** plan4 + Amendments A–D is the implementation spec — no plan5, no further review pass. Every remaining decision is either prescribed above (A, B, C) or a binary choice the implementer can make at the keyboard (D: enqueue vs. cut; MVP cuts 1–3). The architecture survived four adversarial passes; what's left is typing.

---

*Review version: 1.0 (final)*
*Verdict: APPROVED — implement plan4.md with amendments A–D from this document. 0 criticals, 4 required amendments (all prescriptive), 3 optional MVP cuts, 5 nits.*
