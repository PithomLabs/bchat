# Adversarial Security Review — plan_025

**Reviewer:** Go Security Expert (adversarial)
**Date:** 2026-07-07
**Verdict: APPROVED WITH NITS** — plan is sound in structure and threat model, but has 7 issues that need resolution before implementation.

---

## Summary

The plan is well-researched, correctly grounded in the actual code, and the layered defense approach is architecturally correct. The threat model table is accurate (I verified every `file:line` reference). The core design — widget key at edge + atomic rate limiting + input hygiene + transcript HMAC rekey — is the right approach. No fundamental redesign needed.

However, there are issues ranging from missed threats to implementation gaps that should be addressed.

---

## Verified Threat Model (all correct)

| Threat | Code Reference | Status |
|--------|---------------|--------|
| T1: Public chat endpoint | `handlers.go:403` — `HandleChatExternal`, no auth | ✅ Correct |
| T2: Domain allowlist bypass | `handlers.go:1948` — `isDomainAllowed` returns `true` when empty | ✅ Correct |
| T3: IP-rotation defeats rate limit | `service.go:1603` — `CheckRateLimit` keyed only on `clientIP` | ✅ Correct |
| T4: TOCTOU in rate limiter | `sqlite/agent.go:1221`→`1256` — `GetOrCreate` then `Increment` separate | ✅ Correct |
| T5: Transcript HMAC on leaked GUID | `service.go:1545-1551` — `deriveSessionTokenKey(tenant.GUID)` | ✅ Correct |
| T6: Unbounded body / session | No `BodyLimit`; 4000-char per-message only | ✅ Correct |
| T7: Fly hard_limit=25 only backstop | `fly.toml:42` — `hard_limit = 25` | ✅ Correct |

---

## Nits / Issues (must resolve)

### NIT-1: Missing Threat — Playground Endpoint is Also Public and Untargeted

**Severity: Medium**

`POST /api/v1/agent/:slug/playground/run` is registered on the same `publicGroup` (`v1.go:257`) and triggers LLM calls without any rate limiting or widget key check. A bot can hit this endpoint directly to burn OpenRouter credits.

**Recommendation:** Either apply the same widget key check to the playground endpoint, or add it to the threat model as a known gap with a documented follow-up. If it's intentionally left open (e.g., for demo purposes), state this explicitly in the plan.

---

### NIT-2: Per-Session Turn Cap is In-Memory Only — Session Spinning Bypasses It

**Severity: High**

The plan (task #8) proposes enforcing a 50-turn cap using `session.MessageCount` from the in-memory `MemorySessionStore` (`service.go:957`). But a bot can trivially bypass this by rotating `session_id` values — each new session ID creates a fresh in-memory session with `MessageCount = 0`.

The rate limit (per-IP + global tenant cap) partially mitigates this, but if the global tenant cap is 300 RPM and each request is a new session, the bot gets 300 LLM calls/minute regardless of the turn cap.

**Recommendation:** The session turn cap should be documented as a **defense-in-depth** measure against legitimate-user spam, not as a cost-control boundary. The real cost boundary is the global tenant rate cap (Layer B). Alternatively, enforce a **per-IP session count limit** (e.g., max 5 concurrent sessions per IP) in addition to the per-session turn cap.

---

### NIT-3: Atomic Rate Limit — TOCTOU Fix Needs Concrete SQL, Not Just "Single Statement"

**Severity: Medium**

Task #6 says "replace `GetOrCreate`+`Increment` with a single atomic `INSERT ... ON CONFLICT DO UPDATE`" but doesn't specify the full logic, which is non-trivial. The current flow is:

1. `GetOrCreate` — SELECT or INSERT row (returns current count)
2. Check `count >= rpm` in Go
3. `Increment` — UPDATE count+1

The atomic version needs to combine all three in one SQL statement. For SQLite, this requires:

```sql
INSERT INTO agent_rate_limits (tenant_id, audience_type, client_ip, request_count, window_start)
VALUES (?, ?, ?, 1, ?)
ON CONFLICT(tenant_id, audience_type, client_ip) DO UPDATE SET
  request_count = CASE
    WHEN julianday('now') - julianday(window_start) > 1.0/1440 THEN 1
    WHEN request_count < ? THEN request_count + 1
    ELSE request_count
  END,
  window_start = CASE
    WHEN julianday('now') - julianday(window_start) > 1.0/1440 THEN ?
    ELSE window_start
  END
RETURNING request_count, window_start
```

But the unique constraint `(tenant_id, audience_type, client_ip)` doesn't exist yet — the current table has no UNIQUE constraint on these columns (verified in `LATEST.sql`). You need a migration to add the UNIQUE constraint first, otherwise `ON CONFLICT` won't work.

**Recommendation:** Add a prerequisite migration step: `CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_rate_limits_lookup ON agent_rate_limits(tenant_id, audience_type, client_ip);`. Also, for Postgres the syntax differs (`INSERT ... ON CONFLICT ... DO UPDATE`). The task list should note that all three DB drivers need separate atomic implementations.

---

### NIT-4: Widget Key Rotation Has No Grace Period Mechanism

**Severity: Medium**

Task #3 says "admin update/rotate path" but there's no plan for how rotation works. If you rotate the `widget_key`, all currently-deployed widgets immediately break (they embed the old key in their cached `embed.js`). The `Cache-Control: max-age=3600` on `embed.js` (`handlers.go:1696`) means cached versions persist for up to 1 hour.

The plan mentions a "grace period" in Section 4 (Risks) but task #3 doesn't implement it.

**Recommendation:** Either:
- (a) Store the previous `widget_key` in a new column (`widget_key_prev`) and accept either key during a configurable grace window, OR
- (b) Accept the 1-hour cache breakage and document it as expected behavior, OR
- (c) Add explicit cache-busting to the widget key rotation endpoint.

Pick one and add it to the task list.

---

### NIT-5: `embed.js` Does NOT Currently Inject GUID — Widget Key Injection Point is Ambiguous

**Severity: Low (clarification needed)**

The plan (task #5) says "inject `widget_key` where GUID is injected (`handlers.go:1691`)." But at `handlers.go:1691`, GUID is injected into `widget.js` (the legacy inline script endpoint), NOT into `embed.js`.

The `embed.js` endpoint (`HandleWidgetEmbed`, `handlers.go:1991`) serves the built bundle and injects config at `handlers.go:2032-2036`:
```go
configScript := fmt.Sprintf(`window.AgentChatConfig=window.AgentChatConfig||{};
window.AgentChatConfig.baseUrl=window.AgentChatConfig.baseUrl||%q;
window.AgentChatConfig.tenant=window.AgentChatConfig.tenant||%q;
window.AgentChatConfig.companyName=window.AgentChatConfig.companyName||%q;
`, baseURL, tenant.Slug, tenant.CompanyName)
```

No GUID or key is injected into `embed.js` today. The widget_key needs to be added here AND the client-side widget code needs to read `window.AgentChatConfig.widgetKey` and send it as `X-Widget-Key` header.

**Recommendation:** Task #5 should explicitly state: (a) add `widgetKey` to the config injection in `HandleWidgetEmbed`, (b) modify the widget client code to read and send `X-Widget-Key`, (c) update both `embed.js` (built bundle) AND `widget.js` (legacy inline).

---

### NIT-6: Global CORS is `AllowOrigins: ["*"]` — Widget Key + CORS = Defense-in-Depth, Not Redundancy

**Severity: Low (design clarification)**

The public CORS policy (`v1.go:239`) sets `AllowOrigins: []string{"*"}`. This means browsers from ANY origin can make cross-origin requests to `chat/ext`. The widget key (Layer A) is not a replacement for fixing CORS — it's complementary.

A bot calling the API server-side doesn't send `Origin` at all, so CORS doesn't help there. But a malicious script running in a user's browser on a different domain CAN call `chat/ext` with the widget key if it can extract it from the served `embed.js`.

**Recommendation:** Consider tightening the public CORS to only `AllowedDomains` per-tenant (dynamic CORS) as a future hardening step. Not blocking for initial implementation, but should be in the "follow-up" list alongside spend caps.

---

### NIT-7: `AllowedDomains` Empty vs `[]` Behavior Should Be Documented

**Severity: Low (discrepancy)**

The plan (T2) states: "AllowedDomains is opt-in; empty = allow all (handlers.go:1948 returns true)." This is correct.

However, the actual code has two different behaviors:
- `handlers.go:1948-1949`: `""` (empty string) = return `true` (no restrictions)
- `handlers.go:1958-1959`: `[]` (empty JSON array) = return `false` (deny all)

A tenant admin who sets `AllowedDomains = "[]"` thinks they're locking things down, but a tenant who leaves it blank (the default) is wide open.

**Recommendation:** Document this behavior explicitly in the plan. Consider adding a migration to default new tenants to `[]` (deny all) instead of `""` (allow all) for safer defaults.

---

## What the Plan Gets Right

1. **Threat model is accurate** — every file:line reference checks out.
2. **Layered defense** — widget key + rate limit + input hygiene + transcript HMAC + deployment hardening is the correct approach.
3. **Widget key as obfuscation, not secret** — the plan honestly documents this ("obfuscation-grade"). The real enforceable controls are the global tenant cap + session turn cap + body limit.
4. **Atomic rate limit** — correct direction, just needs implementation detail.
5. **Transcript HMAC rekey** — clean fix, uses `WidgetKey` instead of leaked `GUID`.
6. **Fail-closed** — 403 on missing/invalid key, 429 on rate limit, 413 on body too large. Correct.
7. **Multi-instance caveat** — honestly flagged as a known limitation.
8. **Rollback safety** — the grace period approach (Section 4) is the right mitigation.

---

## Required Changes Before Implementation

| # | Issue | Action |
|---|-------|--------|
| 1 | NIT-1: Playground endpoint unaddressed | Add to threat model or document as known gap |
| 2 | NIT-2: Session turn cap bypassable | Document as defense-in-depth; emphasize global tenant cap as the real boundary |
| 3 | NIT-3: Atomic rate limit needs UNIQUE constraint | Add migration prerequisite + concrete SQL for all 3 drivers |
| 4 | NIT-4: Key rotation breaks widgets | Pick a grace period strategy and add to task list |
| 5 | NIT-5: embed.js injection point unclear | Specify exact injection in `HandleWidgetEmbed` + client-side `X-Widget-Key` header |
| 6 | NIT-6: CORS wildcard not addressed | Add to follow-up list |
| 7 | NIT-7: Empty vs `[]` allowlist behavior | Document in plan, consider safer default |

---

## Final Verdict

**APPROVED WITH NITS.** The plan is solid and the core security architecture is correct. The 7 issues above are all resolvable without redesign. Issues #2 (session spinning) and #3 (atomic rate limit UNIQUE constraint) are the most important to resolve before coding begins. The rest are clarifications and documentation gaps.
