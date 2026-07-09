# Adversarial Review: plan3.md

**Reviewer:** AI (adversarial review mode)
**Date:** 2026-07-10
**Document:** `bugs/033/plan3.md` — bchat Native Integrations (Webhooks & SMS), Path A + Resilience
**Previous reviews:** `plan_biz_review.md`, `plan2_review.md`
**Codebase facts verified against:** `store/driver.go`, `store/db/postgres/postgres.go`, `store/migration/{sqlite,postgres,mysql}/`, `plugin/cron/`, `fly.toml`, `bin/memos/main.go`, `server/server.go`, `server/router/api/v1/agent/service.go`

---

## summary

The review is written to [plan3_review.md](bugs/033/plan3_review.md). **Verdict: REWORK REQUIRED** — not approve-with-nits this time. The outbox/poller architecture is the right direction, but I verified plan3's claims against the actual codebase and several headline features don't hold up as written.

The three critical findings:

1. **The "same transaction" outbox is not implementable.** I checked `store/driver.go` and the store layer — there is no transaction API anywhere; every store method runs on its own connection. A true transactional outbox would require plumbing `*sql.Tx` through the lead-capture path across all drivers, which the plan neither scopes nor estimates. Recommendation: insert the event row right after the lead insert and document the tiny failure window honestly.

2. **The idempotency key as designed provides no idempotency.** A random UUID generated per attempt dedupes nothing — a retry generates a fresh key and inserts a duplicate. The key must be deterministic (e.g., derived from tenant + lead + event type), and the retry wrapper must treat a unique-violation (`23505`) as success, which the plan never mentions.

3. **The plan says "Path A (Platform Managed)" but designs the opposite.** Path A was chosen specifically so tenants wouldn't need their own Twilio account (master account + auto-provisioned subaccounts). Plan3's `TwilioConfig` stores per-tenant credentials pasted into the admin UI — tenants still bring their own Twilio, and there's no subaccount provisioning, number purchasing, or usage metering scoped anywhere.

Notable majors, all verified against the repo: MySQL migrations stop at 0.25 and `mysql/LATEST.sql` has no `agent_tenants` table, so the proposed MySQL migrations would fail on a foreign key that doesn't exist — MySQL should be dropped from scope. `fly.toml` has no `kill_timeout`, so Fly's default 5-second SIGTERM grace will SIGKILL the planned graceful shutdown, and nothing wires `Server.Shutdown` into the agent service anyway. The claim-then-deliver locking has no stale-claim recovery, so a machine killed mid-delivery orphans rows in `processing` forever. And the every-minute external ping keeps the machine awake 24/7, making the scale-to-zero config cosmetic — the plan needs to commit to one scheduler story.

The review ends with a 10-item change table for a plan4 revision; none of it materially changes the file list or timeline, and I'd expect a clean approval once those are written in.

## Verdict: 🔶 REWORK REQUIRED (targeted)

The architectural direction is **right**: replacing plan2's fire-and-forget goroutines with a transactional-outbox + poller structurally eliminates the context-cancellation hazard (N2) and gives you durability and retries for free. Approve the direction.

But three of the plan's headline resilience claims are **not implementable as written in this codebase**, and one strategic promise (Path A) is contradicted by the plan's own data model. These are plan-document defects, not code defects — fixing them is a day of plan revision, not a redesign. Do not start implementation from this version.

---

## Resolution of plan2_review.md Nits

| Nit | Status in plan3 | Notes |
|-----|-----------------|-------|
| N1. `SMSService` parallel store access | ⚠️ Ambiguous | plan3_draft.md explicitly said "methods on `*Service`"; plan3.md dropped that sentence. Restate it. |
| N2. Goroutine uses cancelled request ctx | ✅ Fixed structurally | Outbox + cron poller means delivery never depends on the request context. |
| N3. `X-Bchat-Tenant` leaks numeric ID | ✅ Fixed | "use slug or omit" carried over. |
| N4. `phonenumbers` API misuse | ✅ Fixed | `phonenumbers.Parse` specified. |
| N5. MySQL migrations missing | ❌ Made worse | plan3 *added* MySQL migrations — but see C4: MySQL cannot support this feature at all today. |
| N6. Unbounded SMS poll | ✅ Fixed | `LIMIT X` in the locked batch query. |
| N7. Ignored `json.Unmarshal` errors | ⚠️ Not mentioned | Carry into implementation checklist. |
| N8. Postgres "stub initially" | ✅ Fixed | Postgres is now a first-class implementation (`FOR UPDATE SKIP LOCKED`). |

---

## Critical Findings

### C1. "Insert the AgentEvent in the same transaction" is not implementable with the current store layer

Plan §3 (service.go): *"`processChat()` will insert an `AgentEvent` into the database in the same transaction."*

**Verified fact:** the `store.Driver` interface (`store/driver.go`) exposes no transaction API. Every method (`CreateLead`, etc.) executes against the shared `*sql.DB` on its own connection. `captureLeadFromSession()` (service.go:4342) calls standalone store methods. There is no `BeginTx`, no tx-scoped variants of any store method, anywhere in `store/`.

A true same-transaction outbox requires either (a) adding tx-scoped store methods (`CreateLeadTx(ctx, tx, ...)`) and plumbing a `*sql.Tx` through lead capture — a cross-cutting refactor across 3 driver implementations that the plan does not scope, estimate, or even mention — or (b) dropping the claim.

**Recommendation:** Drop the claim. Insert the outbox row as a **separate statement immediately after** the lead insert, and document the residual failure window honestly: "if the process dies between lead insert and event insert, the event is lost — acceptable for v1; the lead itself is durable and visible in the admin UI." This preserves 95% of the outbox value (durable queue, retry, no context-cancellation) without the tx refactor. If you want the last 5%, scope the tx plumbing as explicit work items with their own timeline.

### C2. The idempotency-key design as described provides no idempotency

Plan §1: *"`idempotency_key UUID UNIQUE` to prevent duplicate event dispatching if a connection drops right after a commit but before the app receives the DB acknowledgment."*

Two defects:

1. **A random UUID generated per attempt dedupes nothing.** If the retry wrapper re-executes the insert closure and the closure generates a fresh `uuid.New()`, the retry inserts a second row with a different key, and the UNIQUE constraint happily accepts it. The key must be **deterministic from business identity** (e.g., `hash(tenantID, leadID, eventType)`) or generated **once, outside the retry loop**, and reused across attempts. The plan must state which.
2. **The retry wrapper must treat unique-violation as success.** In the commit-succeeded-but-ack-lost scenario the plan describes, the retry's insert fails with SQLSTATE `23505` (unique_violation). If `isTransientError` doesn't special-case this as "already done, return success," the operation reports failure for work that succeeded — the exact bug the key was meant to prevent, relocated. Neither `resilience.go`'s spec (§2) nor the test list covers this.

Also a portability nit folded in here: `UUID` is not a type in SQLite or MySQL. Use `TEXT` (SQLite) / `CHAR(36)` (MySQL) / `UUID` (Postgres), or `TEXT` everywhere for consistency with the existing schema style.

### C3. The plan says "Path A (Platform Managed)" but designs Path B-with-native-code

The discussion in `prompt.md` defined Path A precisely: **bchat owns a single master Twilio account and auto-provisions Twilio subaccounts per tenant; tenants never need their own Twilio account** — that was the entire reason Path A was chosen over webhooks.

What plan3 actually specifies (inherited from plan2 §7.1): a `TwilioConfig{AccountSID, FromNumber}` per tenant integration with an encrypted per-tenant auth token — i.e., **each tenant brings their own Twilio account and pastes credentials into the admin UI**. Nothing in the plan covers subaccount provisioning (`POST /Accounts.json`), phone-number purchase, per-tenant usage metering, or cost attribution for billing — all of which Path A requires.

This isn't pedantry: the user's stated goal was *minimizing tenant-side integrations*. As written, a tenant still needs a Twilio account, which is Path B friction with Path A code complexity — the worst quadrant of the 2×2.

**Recommendation:** Pick one explicitly:
- **v1 honest rescope (recommended):** rename to "Native SMS with tenant-provided Twilio credentials." Ship it; add subaccount provisioning as a scoped v2 phase. The schema barely changes later (master creds live in env, subaccount SID goes in `TwilioConfig`).
- **True Path A now:** add a "Twilio Provisioning" section — master-account env vars, subaccount creation API, number purchasing UX, usage webhooks for billing — and add ~1–2 weeks to the timeline.

---

## Major Findings

### M1. MySQL is not a viable target — the referenced parent tables don't exist there

**Verified facts:** `store/migration/mysql/` stops at version **0.25** (SQLite and Postgres are at 0.30), and `store/migration/mysql/LATEST.sql` contains **no `agent_tenants` table at all**. The new tables all declare `REFERENCES agent_tenants(id)` — on MySQL that FK target doesn't exist, so the proposed `store/migration/mysql/0.31/*.sql` files would fail on any MySQL install, and versions 0.26–0.30 are missing in between anyway.

**Recommendation:** Drop MySQL from scope with one explicit sentence ("agent features are SQLite + Postgres only; MySQL schema is frozen at 0.25"). Remove the three MySQL migration files and the `store/db/mysql/agent.go` work item. This also deletes the awkward "simulating row locks" hand-waving for a backend that can't run the feature.

### M2. Claimed rows can be orphaned forever — no stale-claim recovery

The plan specifies `SELECT ... FOR UPDATE SKIP LOCKED` but never says what happens *after* the lock. There are only two ways to implement it, and each has a failure mode the plan must address:

- **Hold the tx open during delivery:** the row lock is held across a webhook/Twilio HTTP call of up to 30s. This pins a pooled connection (pool is `MaxOpenConns(10)` per `postgres.go:42`) and holds a long-lived transaction on Neon — precisely what the resilience guide says not to do.
- **Claim-then-release (correct):** `UPDATE status='processing'` inside a short tx, commit, deliver, then mark `delivered`/`failed`. But now a machine killed mid-delivery (Fly does this routinely — see M3) leaves rows stuck in `processing` **forever**; no poller will ever pick them up.

**Recommendation:** Specify claim-then-release plus a **lease**: add `claimed_at BIGINT` and have the poller also reclaim rows where `status='processing' AND claimed_at < now() - 5min`. Combined with C2's idempotency handling, a reclaimed-and-redelivered event is at-least-once, which is the correct contract for webhooks (document it in the webhook docs: consumers must dedupe on `idempotency_key`).

### M3. Graceful shutdown as designed will be SIGKILLed after 5 seconds

**Verified facts:** `fly.toml` sets no `kill_timeout`, so Fly's default **5 seconds** applies between SIGTERM and SIGKILL. `bin/memos/main.go:93` catches SIGTERM and calls `Server.Shutdown` (`server/server.go:172`), which stops Echo and gRPC — it never calls into the agent service, and the plan doesn't scope that wiring.

So the planned `sync.WaitGroup` waiting for in-flight 30s webhook calls (§3) will: (a) never be triggered, because nothing connects `Server.Shutdown` to the agent service, and (b) be killed at 5s even if it were.

**Recommendation:**
1. Add `kill_timeout = 30` (or similar) to `fly.toml` as an explicit work item.
2. Scope the wiring: `Server.Shutdown` → agent `Service.Stop()` → `cron.Stop()` (the vendored fork's `Stop()` returns a context that completes when running jobs finish — use it) → bounded `wg.Wait()` with a timeout.
3. Note that with M2's lease-based reclaim in place, graceful shutdown becomes an optimization, not a correctness requirement — a hard kill just means redelivery after the lease expires. Say so in the plan; it lowers the stakes correctly.

### M4. `HandleTriggerCron` has no authentication and an inconsistent identity

The wake-up endpoint is described as hit by "an external ping (like UptimeRobot)". As specified it is an unauthenticated public endpoint that triggers database polling and outbound HTTP on every request — a free amplification/abuse lever. It's also listed under agent `handlers.go` in §4 but as `POST /api/v1/system/trigger-cron` in the summary — pick one path and owner.

**Recommendation:** Require a shared secret (`X-Cron-Token` checked against an env var), return `202` immediately and run the poll async (the pinger doesn't need to wait 30s), and rate-limit to one concurrent poll (a simple `sync.Mutex`/atomic flag — redundant polls should no-op, which M2's claim mechanism gives you anyway).

### M5. The cron story contradicts the scale-to-zero story — pick a scheduler

**Verified facts:** `fly.toml` has `auto_stop_machines = 'stop'`, `min_machines_running = 0`. An in-process `robfig/cron` (§3) does not run while the machine is stopped. Meanwhile §4's external ping "every minute" keeps the machine awake essentially 24/7, making `min_machines_running = 0` cosmetic — you pay for always-on while architecting for scale-to-zero.

Two coherent configurations exist; the plan currently specifies both halves of each:

- **A (scale-to-zero, recommended):** No always-on in-process cron dependency for correctness. The external ping (every 1–5 min, matching your SMS latency tolerance) hits `HandleTriggerCron`, which runs the poll inline. In-process cron still runs as a bonus while the machine happens to be awake. Cost: SMS/webhook retry latency = ping interval.
- **B (always-on):** Set `min_machines_running = 1`, keep in-process cron as the sole scheduler, delete the trigger endpoint. Cost: one always-on machine (~$3–5/mo for a small VM); simplest code.

**Recommendation:** State the choice explicitly (A fits the stated Fly/Neon posture). Also note: with the outbox pattern, *newly created* events should get one immediate best-effort dispatch attempt (post-commit, detached context) so the common case has near-zero latency and the poller is only the retry/catch-up path — otherwise plan2's "zero latency impact" promise silently becomes "up to one poll interval."

### M6. The retry wrapper's blast radius is unspecified — "wrap database calls" is dangerous

§2 says `resilience.go` will "implement Go retry wrappers for database queries." Wrapping *everything* is wrong in two known ways, both flagged by the resilience guide itself:

1. **Statements inside a transaction cannot be individually retried** — after a connection error the tx is dead; you must retry the whole unit.
2. **Non-idempotent writes** (any insert without an idempotency key — which in this plan is every table except `agent_events`/`agent_sms_messages`) can be duplicated by retry in the ack-lost scenario.

**Recommendation:** The plan should scope the wrapper to: (a) reads, (b) writes carrying an idempotency key or natural unique constraint, (c) whole logical operations (not statements). Everything else keeps single-attempt semantics. One sentence per category is enough — but it must be written down, or the implementer will wrap `ExecContext` globally.

---

## Nits

### N1. Connect timeout: say where it goes
`postgres.go:36` uses `sql.Open("pgx", dsn)` — there is no "pool timeout" knob to set to 15s. The correct change is `connect_timeout=15` in the DSN (or `pgx` config), plus per-operation `context.WithTimeout` at call sites. Minor wording fix so the implementer doesn't hunt for a nonexistent setting.

### N2. Vendored cron, not a new dependency
The plan says "Initialize a `robfig/cron` instance." The repo vendors a fork at `plugin/cron` (`cron.New()`, `AddFunc(spec, fn) (EntryID, error)`). Use it; don't add the upstream module.

### N3. Endpoints dropped relative to plan2
plan2 §4.5 enumerated `GET /events` (event log — the admin UI's delivery-status view depends on it) and `POST /integrations/:id/test`. plan3 compresses to "standard CRUD handlers." Re-enumerate the full endpoint table so nothing silently drops.

### N4. Inbound STOP handling is unscoped
`agent_sms_optouts` exists in the schema, but nothing in the plan receives Twilio's inbound "STOP" webhook or delivery-status callbacks, so the table is never populated. Either scope a `POST /api/v1/agent/twilio/inbound` endpoint (with Twilio signature validation), or state explicitly that v1 relies on Twilio's built-in Advanced Opt-Out (which blocks STOPped numbers carrier-side) and the table is a v2 feature.

### N5. Restate the SMSService resolution
plan3_draft.md resolved plan2-review N1 by putting SMS send logic on `*Service`; plan3.md's §3 lost that sentence and the summary's phrasing ("thin Twilio HTTP client") leaves the old `SMSService{store, crypto}` struct plausible. One sentence: "SMS methods live on `*Service`; `TwilioClient` is a stateless HTTP helper."

### N6. Verification plan gaps
Add to the test list: unique-violation-treated-as-success (C2), stale-claim reclaim (M2), and trigger-endpoint auth rejection (M4). The existing manual SIGTERM test should be re-specified against the 5s/`kill_timeout` reality (M3).

---

## What Improved

Credit where due — plan3 is a real architectural upgrade over plan2:

1. **Outbox over fire-and-forget** — durability, auditability, and retries replace best-effort goroutines; N2 (context cancellation) is fixed by construction rather than by `context.WithoutCancel` band-aid.
2. **`FOR UPDATE SKIP LOCKED`** — correct primitive for multi-machine safety on Postgres, and cheap even while you run one machine.
3. **Resilience layer grounded in the guide** — the SQLSTATE list (`57P01`, `08006`, `08003`), backoff-with-jitter parameters, and 15s cold-start allowance match `resilience_guide.md` exactly.
4. **Scale-to-zero acknowledged at all** — most plans ignore that in-process schedulers die with the machine; this one at least reaches for a wake-up mechanism (it just needs to commit to one story — M5).

---

## Recommendation

**Rework the plan document; keep the architecture.** Concretely, the revision (call it plan4.md) needs:

| # | Change | Driven by |
|---|--------|-----------|
| 1 | Replace "same transaction" with post-insert outbox row + documented failure window (or scope tx plumbing as explicit work) | C1 |
| 2 | Deterministic idempotency keys; retry wrapper treats `23505` as success; portable column types | C2 |
| 3 | Rescope as "tenant-provided Twilio credentials" v1, or add subaccount-provisioning phase | C3 |
| 4 | Drop MySQL from scope (one sentence + remove 4 work items) | M1 |
| 5 | Claim-then-release with `claimed_at` lease + reclaim query; document at-least-once delivery | M2 |
| 6 | `kill_timeout` in fly.toml + `Server.Shutdown` → agent `Service.Stop()` wiring + bounded wait | M3 |
| 7 | Shared-secret auth + async 202 + single-flight on the trigger endpoint; fix its path | M4 |
| 8 | Commit to scheduler story A (ping-driven, scale-to-zero) with immediate post-commit dispatch attempt | M5 |
| 9 | Scope the retry wrapper (reads / keyed writes / whole units) | M6 |
| 10 | Nits N1–N6 | — |

None of this changes the file list materially (minus MySQL, plus fly.toml) or the timeline by more than a few days. Once these are written into plan4, I'd expect a clean APPROVED.

---

*Review version: 1.0*
*Verdict: REWORK REQUIRED — 3 critical (outbox tx claim, idempotency design, Path A contradiction), 6 major, 6 nits. Architecture direction approved; do not begin implementation from plan3.md as written.*
