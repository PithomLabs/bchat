# bchat Public Endpoint Security & Bot-Defense Plan (plan_025)

**Scope:** Hardening the unauthenticated public chat surface of bchat when deployed to Fly.io (`app = bchat0534`), focused on `POST /api/v1/agent/:slug/chat/ext`, the widget asset endpoints, and the transcript endpoint. Goal: stop scripted bots from abusing the LLM/embedding pipeline (cost-based DoS) and from degrading availability or leaking data — via a **layered, defense-in-depth** approach covering all three threat classes (cost/LLM abuse, availability/DoS, data isolation/leakage).

---

## 1. Threat Model (verified against code)

| # | Threat | Current gap (file:line) | Impact |
|---|--------|--------------------------|--------|
| T1 | Scripted bot hits `chat/ext` server-side | Endpoint is fully public, no auth/CAPTCHA (`handlers.go:403`) | Unlimited LLM/embedding spend (OpenRouter billed per call) |
| T2 | No `Origin` header → domain allowlist bypassed | `AllowedDomains` is opt-in; empty = allow all (`handlers.go:1948` returns `true`); bots send no `Origin` | Any host/tool can call the endpoint |
| T3 | IP-rotation botnet defeats rate limit | Rate limit is per-IP only (`service.go:1594`); trivially bypassed by rotating IPs | Availability + cost exhaustion |
| T4 | TOCTOU race in rate limiter | `GetOrCreate` then `Increment` are separate statements (`sqlite/agent.go:1221`,`1256`) | Two concurrent requests can both pass the limit |
| T5 | Transcript token forgeable from leaked GUID | HMAC uses `tenant.GUID` (`service.go:1551`); GUID is exposed in public `widget.js`/config (`handlers.go:658`,`1691`) | Attacker reads any session transcript |
| T6 | Unbounded request body / session length | No global `BodyLimit`; only per-message 4000-char cap (`service.go:1578`); no per-session turn cap | Memory/LLM spend amplification per conversation |
| T7 | Fly concurrency hard cap (25) is the only backstop | `fly.toml` `hard_limit = 25`; no app-level global ceiling | Single tenant can saturate the instance for all tenants |

---

## 2. Defense Layers

### Layer A — Edge gate: static per-tenant Widget Key (addresses T1, T2)
- Add a **`widget_key`** column to `agent_tenants` (random 32-byte hex, generated at tenant creation, rotatable). Keep `GUID` for widget **identity** only (display/iframe), never for secrets.
- `embed.js` (served only to allowed domains) is generated to include the `widget_key` (server injects it, same place `GUID` is used today at `handlers.go:1691`).
- `HandleChatExternal` requires header `X-Widget-Key: <key>` (or `?widget_key=` for SSE). On mismatch/empty → `403 Access denied`. **Fail-closed.**
- This blocks any caller that did not fetch the served `embed.js` asset (i.e. raw scripted bots), independent of `Origin`.
- Note: key embedded in public JS is obfuscation-grade, not cryptographically secret — it raises the bar for bots and pairs with Layer B/C. Document this honestly.

### Layer B — Rate limiting: atomic + global tenant cap (addresses T3, T4, T7)
- Fix TOCTOU: replace `GetOrCreate`+`Increment` with a **single atomic `INSERT ... ON CONFLICT DO UPDATE` / `UPDATE ... WHERE count < limit`** statement (SQLite + Postgres + MySQL variants) so the check-and-increment is one row op.
- Add a **global per-tenant rate limit** (keyed by `tenant_id` + `audience_type` only, ignoring IP) with a hard ceiling (default e.g. 300 RPM/tenant, configurable). This caps blast radius even when a botnet rotates IPs.
- Keep per-IP limit as a second dimension; both must pass.
- Add **per-session message budget** (e.g. 50 turns/session) enforced in `ChatExternal` before the LLM call, to bound spend per conversation.
- Multi-instance caveat: on Fly with >1 instance the DB-backed global cap is eventually-consistent across instances (acceptable now since `min_machines_running = 0`, single instance). Flag Redis token-bucket as a future upgrade.

### Layer C — Input hygiene & LLM-cost controls (addresses T6)
- Add global Echo `BodyLimit` middleware (e.g. `16KB`) in `server/router` setup.
- Lower default `MaxMessageLength` from 4000 → 2000 chars (tenant-configurable via `AgentAudience.MaxMessageLength`).
- Per-session turn cap (Layer B) is the spend backstop.

### Layer D — Transcript trust boundary (addresses T5)
- Derive the transcript HMAC (`generateSessionToken`/`verifySessionToken`, `service.go:1543`,`1551`) from the **private `widget_key`**, not `GUID`.
- Remove `GUID` from any response that is reachable without auth if it is not needed; at minimum, stop treating GUID as a secret. Keep `GUID` in `widget.js`/config for identity only.
- Result: a leaked GUID can no longer forge transcript access.

### Layer E — Deployment hardening (Fly.io) (addresses ops surface)
- `fly.toml`: confirm `force_https = true` (already set); add `request_timeout` (e.g. `30s`) to `[http_service]`; keep `internal_port = 5230`.
- Ensure `OPENROUTER_API_KEY` is set via Fly secrets (not in `fly.toml` env — currently it is NOT in `fly.toml`, good; verify it is a secret, not plaintext).
- Document management/rotation of the new `widget_key` (stored in DB; rotate via admin endpoint or migration).
- Verify admin API remains behind JWT (it is today); only `chat/ext`, `widget.js`, `embed.js`, `iframe`, and `chat/ext/transcript` are public.

---

## 3. Implementation Task List (ordered)

1. **Migration** — new `store/migration/sqlite/NN__add_widget_key.sql`:
   ```sql
   ALTER TABLE agent_tenants ADD COLUMN widget_key TEXT;
   CREATE INDEX IF NOT EXISTS idx_agent_tenants_widget_key ON agent_tenants(widget_key);
   ```
   Backfill existing tenants with a generated `widget_key` (one-time migration step / startup job).
2. **Store** — add `WidgetKey string` to `AgentTenant` (`store/agent.go:11`); include in INSERT/SELECT in `sqlite/agent.go:26` & `postgres/agent.go:29` (and mysql).
3. **Key generation** — generate `widget_key = uuid/rand` at tenant create (mirror `GUID` generation at `sqlite/agent.go:26`) and admin update/rotate path.
4. **Edge gate** — `HandleChatExternal` (`handlers.go:403`): validate `X-Widget-Key` against `tenant.WidgetKey` before any work; `403` on failure. Apply same to `HandleWidgetEmbed`/`HandleWidgetIframe` (optional, defense-in-depth).
5. **embed.js** — inject `widget_key` where `GUID` is injected (`handlers.go:1691`, `generateWidgetScript`); widget sends `X-Widget-Key` header on `chat/ext`.
6. **Atomic rate limit** — rewrite `GetOrCreateAgentRateLimit`/`IncrementAgentRateLimit`/`ResetAgentRateLimit` across sqlite/postgres/mysql as a single atomic upsert-with-ceiling.
7. **Global tenant cap** — add `CheckRateLimit` variant keyed by tenant only; call both per-IP and global in `ChatExternal` (`service.go:1594`).
8. **Session turn cap** — track `session.MessageCount` (or a counter) in `memorySessions`; reject at e.g. 50 turns with `429`.
9. **Transcript HMAC** — change `deriveSessionTokenKey` (`service.go:1536`) to use `tenant.WidgetKey` instead of `tenant.GUID`; update `verifySessionToken` caller at `handlers.go:507`.
10. **BodyLimit + message length** — add `e.BodyLimit("16KB")` in router setup; lower `MaxMessageLength` default 4000→2000 in `service.go:1578` (and the internal mirror at `service.go:1796`).
11. **fly.toml** — add `request_timeout = "30s"` under `[http_service]`; document secrets.
12. **Tests** — add handler tests: missing/invalid widget key → 403; valid key → 200; rate-limit global cap enforced under IP rotation; transcript token still valid after GUID change but invalid with wrong widget_key; body > limit rejected.

---

## 4. Risks / Open Questions
- **Widget key exposure:** key ships in public `embed.js`, so a determined attacker can extract it. It is a speed-bump, not absolute. The real enforceable controls are the global tenant cap + session turn cap + body limit (Layers B/C), which bound cost regardless. Decision: accepted; key + allowlist together raise the bot bar.
- **Backward compat:** existing widgets/integrations calling `chat/ext` without the key will break. Mitigation: during rollout, support a **grace period** where missing key is allowed *only if* `AllowedDomains` is configured AND Origin passes; after cutover, require key. Confirm rollout strategy.
- **Multi-instance:** global cap is DB-consistent but not perfectly atomic across Fly instances under heavy concurrency; acceptable for current single-instance deploy.
- **Not in this plan (flagged follow-ups):** per-tenant daily LLM **spend cap** (fail-closed budget), inbound content moderation, structured security-event logging/alerting, abuse `/metrics` endpoint. These strengthen the posture but are out of initial scope per decisions.

---

## 5. Validation
- Load `embed.js` from an allowed domain → `chat/ext` with `X-Widget-Key` → `200`.
- `curl` `chat/ext` with no key / wrong key → `403`.
- `curl` with rotated `Origin`/no `Origin` and no key → `403`.
- Fire >global-cap requests from many fake IPs → global cap still triggers `429`.
- Two concurrent requests at the limit boundary → exactly one `429` (no TOCTOU leak).
- Transcript endpoint: token signed with old `GUID` after migration → `403`; token signed with `widget_key` → `200`.
- Body >16KB → `413`; message >2000 chars → `400 message too long`; session >50 turns → `429`.
