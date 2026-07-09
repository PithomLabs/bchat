# Adversarial Review: plan4_imp.md (Pre-Implementation)

**Reviewer:** AI (adversarial review mode)
**Date:** 2026-07-10
**Document:** `bugs/033/plan4_imp.md` — implementation plan for Native Webhook Integrations v1
**Source lineage:** plan4.md (APPROVED FINAL) + plan4_review.md (amendments A–D)
**Codebase facts verified against:** `scripts/entrypoint.sh`, `Dockerfile.pg.fly`, `fly.toml`, `plugin/webhook/webhook.go`, `server/router/api/v1/agent/handlers.go`, `server/router/api/v1/v1.go`, `store/migration/{sqlite,postgres}/LATEST.sql`, `store/db/*/agent.go`, `go.mod`

---

## Verdict: 🔶 REWORK REQUIRED — scoped to §6 (Container/Deploy) + the scale-to-zero claim

The **application layers are approved.** The migrations (§3), store layer (§4), service/dispatch logic (§5.1–5.5), API/security (§7, §9), tests (§10), and frontend (§8) are sound and correctly carry amendments A–D from `plan4_review.md`. A coding agent can start on those today.

**Do not ship §6 (Container Setup) as written.** It replaces the production entrypoint with one that drops secret-file handling and privilege de-escalation — the app would deploy **running as root with broken secret injection** — and it makes a scale-to-zero claim the architecture does not deliver. Two criticals, two majors, three moderates. All have prescriptive fixes below; none requires a plan5.

---

## Amendments A–D + Prior Nits: Resolution

| Item | Status | Evidence |
|------|--------|----------|
| **A.** MySQL compile stubs | ✅ | §4.6 stubs returning `errNotImplemented` |
| **B.** Pre-claimed insert (no double-dispatch) | ✅ | §3 `status DEFAULT 'processing'`, `claimed_at`; §5.3 dispatchEvent inserts pre-claimed |
| **C.** `attempts` cap | ✅ | `attempts INTEGER` column + `attempts < 5` in claim query (§4.3/§4.4) |
| **D.** SMS enqueue | ✅ (cut) | Scheduled SMS deferred to v1.1 — the "cut" option, cleanly chosen |
| Deterministic idempotency key | ✅ | §9.3 `hash(tenantID, leadID, eventType)` |
| `23505`-as-success | ✅ | §4.5, `TestIdempotentInsert_UniqueViolation` |
| Postgres `FOR UPDATE SKIP LOCKED` + BIGINT cast | ✅ | §4.4 query correct |
| Endpoints re-enumerated + RBAC | ✅ | §7 tables restore the permission column |
| FK types | ✅ (verified) | `agent_tenants.id` is INTEGER (sqlite) / SERIAL (pg); new `tenant_id INTEGER` matches |
| Handler wiring | ✅ (verified) | `Handler{service *Service}` exists, so `h.service.processEventPoller` is valid |

The core queue engineering is genuinely done. The problems are all in deployment plumbing and a few package/wiring details the coding agent will trip over.

---

## Critical Findings

### CR1. §6.2 replaces the production entrypoint → app runs as root with broken secret injection

**Verified fact.** The repo already has `scripts/entrypoint.sh`, invoked by `Dockerfile.pg.fly` as `ENTRYPOINT ["./entrypoint.sh", "./memos"]`. That script does two things the deploy depends on:
1. **`_FILE` secret expansion** via `file_env` for `MEMOS_DSN`, `ENCRYPTION_MASTER_KEY`, `OPENROUTER_API_KEY`, `AWS_ACCESS_KEY_ID/SECRET` — this is how Neon DSN and the tenant-credential encryption key reach the app.
2. **Privilege drop** root→`memos` via `gosu` (`exec gosu memos "$@"`), after fixing volume ownership. The Dockerfile even carries the comment: *"Do NOT add USER memos here — entrypoint handles privilege dropping."*

plan4_imp §6.2 introduces a **brand-new root `/entrypoint.sh`** and §6.3 overrides `ENTRYPOINT ["/entrypoint.sh"]`. The new script does neither of the above. Consequences:
- `ENCRYPTION_MASTER_KEY_FILE` / `MEMOS_DSN_FILE` stop being honored → encryption service fails to init, or DSN is empty → app can't start or can't decrypt tenant Twilio/webhook secrets. This directly breaks the feature being shipped.
- No `gosu` drop → **bchat runs as root** in production, reversing hardening H5.
- Wrong binary path (`./build/memos`) and `--data /data` (there is no volume — Neon replaced it, per fly.toml's own comment).

**Fix (prescriptive):** Do **not** author a new entrypoint. Extend `scripts/entrypoint.sh` — launch supercronic in the background just before the final `exec gosu memos "$@"`, e.g.:
```sh
# after file_env calls, before the gosu exec
if command -v supercronic >/dev/null 2>&1 && [ -n "$CRON_TOKEN" ]; then
    supercronic /etc/bchat/crontab &
fi
...
exec gosu memos "$@"   # unchanged — bchat stays PID-forwarded, non-root
```
Keep the existing `ENTRYPOINT ["./entrypoint.sh", "./memos"]`. Delete §6.2's new script and §6.3's `ENTRYPOINT` override entirely.

### CR2. The scale-to-zero claim is false — in-container supercronic cannot wake a stopped machine

**Verified fact.** `fly.toml` sets `auto_stop_machines = 'stop'` and `min_machines_running = 0`. The machine stops when idle. Supercronic (§6.1/§6.2) runs **inside that same container**, so it stops with the machine. §4 and §5.4 nonetheless claim the trigger endpoint "fully supports Fly's scale-to-zero (Scheduler Story A)."

It does not. An in-container cron pinging `localhost` is functionally identical to the in-process cron that `plan3_review.md` M5 rejected: once the machine sleeps, nothing fires the poller, so failed-webhook **retries never run until unrelated traffic happens to wake the machine.** This silently reverts the M5 fix. Story A's whole premise was an **external** pinger.

**Fix — pick one and state it honestly:**
- **(a) Keep it truly external (matches Story A):** point an external scheduler at the *public* URL — a Fly scheduled Machine (`fly machine run … --schedule`), Fly cron-style app, or a free uptime pinger (UptimeRobot/cron-job.org) hitting `https://<app>/api/v1/system/trigger-cron` with the `X-Cron-Token` header. Remove supercronic from the container. This actually wakes the machine.
- **(b) Accept awake-only retries for MVP (simplest):** keep in-container supercronic, but **drop the "supports scale-to-zero" claim** and document the real contract: *immediate dispatch handles the live path; the poller only catches failed/stale events while the machine is already awake; a fully-idle machine defers retries until it next wakes.* Given lease reclaim + immediate dispatch, this is defensible for v1 — but the plan must say so instead of claiming the opposite.

Recommendation: **(b)** for v1 (least moving parts), with a one-line note that **(a)** is the v1.1 upgrade if guaranteed retry latency is needed.

---

## Major Findings

### MJ1. §6.4 fly.toml edits are invalid and self-contradictory

- There is **no `[processes]` table** in `fly.toml` (only `processes = ['app']` nested under `[http_service]`). `kill_timeout` is a **top-level app key**, not a `[processes]` key. As written the edit either no-ops or fails to parse.
- It also contradicts §5.6 ("No changes needed" to `server.go`; no in-flight drain). A 30s `kill_timeout` with no graceful-drain logic buys nothing — the process just idles 30s before SIGKILL.

**Fix:** Since CR1's fix keeps the single `exec gosu memos` process and lease reclaim makes graceful drain optional (as plan4 itself noted), **either** add top-level `kill_timeout = "30s"` *and* a minimal drain, **or** drop `kill_timeout` entirely and rely on lease reclaim. Don't put it under `[processes]`.

### MJ2. `CRON_TOKEN` is a secret and must not live in `fly.toml [env]`

§6.4/§9.4 place `CRON_TOKEN = "your-secret-token-here"` in `[env]`, which is committed to git. That defeats the auth added in Amendment M4.

**Fix:** `fly secrets set CRON_TOKEN=…` (runtime env, not in repo). Optionally support `CRON_TOKEN_FILE` via the existing `file_env` mechanism in `scripts/entrypoint.sh` for consistency with the other secrets. Also make the comparison constant-time (`hmac.Equal` / `subtle.ConstantTimeCompare`) rather than `!=`.

---

## Moderate Findings

### MO1. `validateAndResolveWebhookURL` is unexported — decide copy-vs-export now
**Verified fact.** In `plugin/webhook/webhook.go` the helper is lowercase (`validateAndResolveWebhookURL`, `isInternalIP`), so §5.1's "port … from `plugin/webhook`" cannot mean import. The plan's own open-item #4 leaves this unresolved. Pick one before coding: **copy** the SSRF helpers into `agent/integrations.go` (simplest, keeps packages decoupled) or **export** them from `plugin/webhook`. Copying is recommended — the two callers will diverge (protobuf payload vs raw JSON) anyway.

### MO2. Trigger-cron handler package + route registration don't match the codebase
- §5.4 puts `HandleTriggerCron` in `server/router/api/v1/handlers.go`, but `processEventPoller` is on the **agent** `*Service`, and the agent `Handler{service}` lives in `server/router/api/v1/agent/handlers.go`. Put the handler there (alongside the CRUD handlers) so `h.service.processEventPoller` resolves.
- §5.5 invents `agentGroup`/`systemGroup` and `h.HandleX`. Real registration (v1.go) uses `s.agentHandler.HandleX` on existing groups: authenticated agent routes go on **`authGroup`** (`/api/v1/agent`), public ones on `publicGroup`. There is **no `systemGroup`** — register trigger-cron as a bare `echoServer.POST("/api/v1/system/trigger-cron", s.agentHandler.HandleTriggerCron)` (its own auth via `X-Cron-Token`, outside tenant middleware). Integration CRUD belongs on `authGroup` so it inherits tenant auth; enforce the §7 `tenant:admin`/`tenant:read` split there.

### MO3. Don't add `phonenumbers` + `sms.go` for deferred dead code
`github.com/nyaruka/phonenumbers` is **not in go.mod**, and §5.2's `sms.go` is explicitly "Not called in v1." Adding a dependency and a file for code that never runs is scope creep that must be maintained and security-scanned. **Cut `sms.go` and the dep from v1**; reintroduce both in the v1.1 SMS work where they're actually exercised. (This also tightens the file count in §12.)

---

## Nits (fix in passing)

1. **Line numbers are unreliable — ignore them.** `store/db/mysql/agent.go` is 409 lines but §4.6 says "append after line 884"; `store/agent.go` (1194 lines) and `sqlite/agent.go` (2594) get mid-file "append after" targets. Instruct the coding agent to add methods to the correct file/interface, not at a line offset.
2. **Drop the `02__agent_sms_deferred.sql` placeholders.** A migration whose body is `SELECT 1;` just consumes a version slot and clutters history. Omit the file; add the real `02__agent_sms.sql` in v1.1.
3. **`status DEFAULT 'processing'` is a queue smell.** A brand-new row defaulting to "in-flight" is unusual; it's only correct because every insert path here pre-claims. Add a schema comment saying so, so a future non-dispatch insert path doesn't inherit a silently-claimed row.
4. **State the immediate-dispatch failure semantics.** §5.3 says the goroutine delivers immediately but not what happens on failure. Specify: on failure, leave the row `processing` with its original `claimed_at` (do **not** reset it) so the poller reclaims after the 300s lease; only success flips it to `delivered`. Otherwise a reset `claimed_at` would pin the row forever.
5. **`exec supercronic … &` in §6.2 is malformed** (moot once CR1's fix removes that script) — `exec` replaces the shell, `&` backgrounds; together they're a bug. The CR1 fix (`supercronic … &` without `exec`, before the final `exec gosu`) is the correct form.

---

## Prescriptive Fix Summary

| # | Severity | Fix |
|---|----------|-----|
| CR1 | Critical | Extend `scripts/entrypoint.sh` (launch supercronic before `exec gosu memos "$@"`); delete new `/entrypoint.sh` + `ENTRYPOINT` override |
| CR2 | Critical | Drop the false scale-to-zero claim; choose (a) external pinger on the public URL or (b) document awake-only retries for v1 |
| MJ1 | Major | `kill_timeout = "30s"` top-level (not `[processes]`) + minimal drain, or drop it; reconcile with §5.6 |
| MJ2 | Major | `CRON_TOKEN` via `fly secrets`, not `[env]`; constant-time compare |
| MO1 | Moderate | Copy SSRF helpers into `agent/integrations.go` (they're unexported) |
| MO2 | Moderate | Handler in agent package; register on `authGroup` + bare system route; no `systemGroup` |
| MO3 | Moderate | Cut `sms.go` + `phonenumbers` from v1 |
| N1–N5 | Nit | Ignore line numbers; drop placeholder migrations; comment status default; specify immediate-failure path; fix shell |

---

## Closing

The hard part — the durable, idempotent, lease-reclaimed outbox — is designed correctly and carries every amendment from the prior review. What's left is deployment plumbing that ignored the repo's existing entrypoint and over-claimed the scheduler. Apply CR1/CR2 and the majors before the coding agent touches §6; the app layers (§3–5, §7–10) can proceed in parallel now. **No plan5 needed — fix inline and build.**

---

*Review version: 1.0*
*Verdict: REWORK REQUIRED (deploy layer only) — 2 critical, 2 major, 3 moderate, 5 nits. Application layers APPROVED.*
