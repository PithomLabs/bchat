# Adversarial Review: plan_biz.md

**Reviewer:** AI (adversarial review mode)
**Date:** 2026-07-10
**Document:** `bugs/033/plan_biz.md` — bchat Integrations: Webhooks + SMS + Calendar

---

## Verdict: 🔁 REWORK

The plan demonstrates strong product vision and security awareness, but has critical gaps in codebase alignment, migration strategy, scoping discipline, and several phantom references that would cause implementation to stall. Needs a focused rework pass before coding begins.

---

## Summary of Findings

| Severity | Count | Category |
|----------|-------|----------|
| 🔴 Critical | 5 | Blockers that would halt implementation or cause production failures |
| 🟠 Major | 6 | Significant issues requiring plan revision |
| 🟡 Minor/Nit | 8 | Improvements that would make the plan cleaner |

---

## 🔴 Critical Issues

### C1. Migration Numbering Scheme is Wrong

The plan proposes migrations named `30__agent_integrations.sql`, `31__agent_sms.sql`, `32__agent_appointments.sql` following the `NN__snake_case.sql` convention from AGENTS.md. **This is not how bchat migrations work.**

Actual migration structure uses **versioned directories**, not flat numbered files:

```
store/migration/sqlite/
├── 0.2/
├── 0.3/
│   ...
├── 0.29/
├── 0.30/
│   ├── 00__relax_agent_rate_limits_fk.sql
│   └── 01__add_vector_db_s3_override.sql
├── LATEST.sql
```

The next migration must go into `0.31/00__*.sql` (or add to `0.30/02__*.sql` if the version hasn't been released). The plan's flat-file naming would be completely ignored by the migrator, which uses `fs.Glob(migrationFS, fmt.Sprintf("%s*/*.sql", ...))` against versioned subdirectories.

**Additionally**, migrations must be created for **all three database backends**: `sqlite/`, `postgres/`, and `mysql/`. The plan only mentions SQLite. Postgres is in active production use (Neon); shipping without Postgres migrations is a deployment blocker.

### C2. `EventDispatcher` Takes `*store.Store` but the Agent Service Uses `*Service`

The plan has `EventDispatcher` directly accessing the store:

```go
type EventDispatcher struct {
    store  *store.Store
    crypto *crypto.EncryptionService
}
```

But the codebase pattern is that all agent business logic lives on `*Service` (in `service.go`), which owns the store reference. The dispatcher should either:
- Be a method set on `*Service` (preferred — follows existing pattern), or
- Receive a narrower interface (not the full `*store.Store`)

As written, this creates a parallel data access path that bypasses the service layer's tenant isolation guards. The `processChat()` function signature is `func (s *Service) processChat(...)` — the event dispatch should stay within this method receiver chain.

### C3. No Retry/Dead-Letter Queue Design

The `agent_events` table has `status`, `attempts`, `next_retry_at` columns, but **there is zero implementation detail for the retry loop**. Key missing pieces:

- What component polls `agent_events` for retries? The cron plugin? A goroutine? A separate worker?
- What's the backoff strategy? (Exponential? Fixed intervals? Jitter?)
- What's the max retry count before moving to permanent failure?
- How are stuck/zombie events recovered after a crash?
- There's no `integration_id` foreign key on `agent_events` — how does the retry know *which* webhook to re-deliver to?

Without retry, webhooks are fire-and-forget, which makes the event log table mostly audit theater. This needs a concrete retry design or an explicit statement that v1 is best-effort.

### C4. Phantom `EncryptionService` Injection Path

The plan assumes `*crypto.EncryptionService` will be injected into `EventDispatcher`, `SMSService`, and `CalendarService`. The actual `EncryptionService` in `internal/crypto/encryption.go` requires a `masterPassword` and `salt` at construction time.

**Missing from the plan:**
- Where is the salt stored? Per-tenant? Global? (Currently the encryption module uses a global key — check `ENCRYPTION_MASTER_KEY`)
- How does the service layer get access to the `EncryptionService` instance? There's no evidence it's currently wired into the agent service. The plan needs to show the DI/initialization path.
- The `Encrypt()` method returns `(ciphertext, nonce []byte, err error)` — but the plan's pseudo-code calls `s.crypto.Decrypt(integration.ConfigEncrypted, integration.ConfigNonce)` which returns `(string, error)`. This matches the actual API, but the **encrypt side** in the handler needs to be shown correctly.

### C5. Twilio SDK Dependency Doesn't Exist

The plan references "Add Twilio SDK" (day 11-12) but:
- `go.mod` has no Twilio dependency
- The plan shows no `go get` step or dependency specification
- No discussion of which Go Twilio library to use (official `github.com/twilio/twilio-go`? Direct HTTP?)
- No analysis of Twilio SDK's dependency tree impact on binary size

For a project that carefully avoids unnecessary dependencies (note: pgx only, no lib/pq), adding a full vendor SDK needs justification. A thin HTTP client wrapper (like the existing `plugin/webhook/webhook.go` pattern) might be more appropriate.

---

## 🟠 Major Issues

### M1. `agent_events` Table Missing `integration_id`

The event log table doesn't link to which integration received/should receive the event:

```sql
CREATE TABLE IF NOT EXISTS agent_events (
    ...
    -- No integration_id column!
    ...
);
```

Without this, you cannot:
- Retry delivery to a specific webhook
- Show per-integration delivery history
- Debug which integration failed

### M2. Calendar OAuth2 State Token Has No Secure Storage

The plan defines an `OAuthState` struct with `IntegrationID`, `TenantID`, `Expiry`, `Nonce` and says "Sign with HMAC, store in session/cache". But bchat has no session store and no cache layer. Options not discussed:
- Short-lived encrypted DB row (like the existing tenant selection token pattern)
- Signed JWT (stateless, but needs HMAC key management)
- In-memory map (lost on restart, breaks multi-instance)

This is a security-critical component that needs a concrete design, not a handwave.

### M3. `config_plaintext` Column is TEXT, but Treated as Structured JSON Map

The plan stores `config_plaintext TEXT NOT NULL DEFAULT '{}'` and then accesses it like:

```go
integration.ConfigPlaintext["account_sid"]
```

This implies the Go struct has `ConfigPlaintext map[string]string` or similar, but the plan's `AgentIntegration` struct definition is never shown. The plan needs to define:
- The Go struct with JSON marshal/unmarshal for the text column
- Whether `config_plaintext` is `map[string]interface{}` or typed per integration
- How the templates nested JSON (deeply structured) maps to a flat `TEXT` column

### M4. UI Code Uses Tailwind Utility Classes

The UI section uses `className="bg-indigo-50 dark:bg-indigo-900/20 rounded-xl border ..."` throughout. bchat's frontend uses **Joy UI** (`@mui/joy`), not Tailwind. The existing `AgentAdmin.tsx` likely uses Joy components and inline `sx` props. All UI code snippets need to be rewritten using Joy UI patterns.

### M5. Scope Creep — Phase 3 Calendar is Massive

Calendar integration alone (Google + Outlook OAuth2, availability checking, booking, rescheduling, cancellation, double-booking prevention, buffer time, working hours, timezone handling) is easily a standalone project. The plan estimates 12 days for this, which is extremely aggressive for:
- Two OAuth2 provider integrations
- Two different APIs (Google REST v3 vs Microsoft Graph)
- Timezone-aware availability algorithms
- A booking UI

Recommend deferring Phase 3 to a separate plan, or scoping down to Google Calendar only for v1.

### M6. No Discussion of `processChat()` Hot Path Impact

The plan hooks event dispatching into `processChat()` (the core chat handler), but:
- `processChat()` is already 4499+ lines of service logic
- Is event dispatch synchronous or async? The plan says "non-blocking: log failures, don't fail chat" but the implementation shows a synchronous `Dispatch()` call
- What's the latency budget? The plan says "<50ms per event" but webhook delivery includes DNS resolution, TLS handshake, and HTTP round-trip
- Should this be a goroutine with context, or go through a channel/queue?

---

## 🟡 Minor / Nits

### N1. `tenant_id` in Webhook Payload Exposes Internal Identifier

```json
{
  "event": "lead.captured",
  "tenant_id": "inc",
  ...
}
```

The payload shows `tenant_id: "inc"` (the slug), but the schema shows `TenantID string` in the Event struct. The AGENTS.md security guidelines say "Never expose tenant IDs in error messages". Consider whether the webhook payload should use `tenant_slug` naming for clarity, and whether exposing it externally is intentional.

### N2. SMS Cost Tracking Uses `cost_cents INTEGER DEFAULT 0`

Twilio pricing varies by country, message type (SMS vs MMS), and carrier. A single `cost_cents` integer won't capture:
- Fractional cent pricing (some routes are $0.0075)
- Currency differences
- Cost not known at send time (only at delivery callback)

Consider `cost_microdollars BIGINT` or deferring cost tracking entirely.

### N3. `libphonenumber` Mentioned Without Go Package Name

The plan says "Phone number validation via `libphonenumber` library" but doesn't specify which Go port. The main options are `github.com/nyaruka/phonenumbers` (maintained) or `github.com/ttacon/libphonenumber` (less maintained). This matters for dependency evaluation.

### N4. Secret Rotation Grace Period Logic Not Specified

"Old secret valid for 24 hours after rotation" — where is the old secret stored? The plan has a single `config_encrypted` column. To support dual-secret validation during rotation, you'd need either:
- A `previous_secret_encrypted` column, or
- A JSON array of secrets with timestamps in the encrypted blob

### N5. No RBAC Permission Defined for Integrations

The RBAC system has specific permissions (`tenant:admin`, `files:upload`, `chat:test`, etc.). The plan adds integration CRUD endpoints but doesn't specify which permission gates them. Recommend adding an `integrations:manage` permission or specifying that `tenant:admin` covers it.

### N6. `HandleTestIntegration` Endpoint is Unauthenticated Risk

The "test" endpoint (`POST /:slug/integrations/:id/test`) could be abused to generate outbound HTTP requests if not properly permission-gated. Ensure it requires at minimum `tenant:admin`.

### N7. Timeline Assumes Single Developer

The 10-week (50 working-day) timeline assumes continuous, uninterrupted development. No buffer for:
- Code review cycles
- Debugging production issues on existing features
- Testing across SQLite + PostgreSQL
- Frontend testing across browsers

### N8. Monitoring Section Lists Alert Thresholds but No Implementation

"Webhook delivery failures > 5% in 5 minutes" — bchat has no metrics/alerting infrastructure mentioned elsewhere. This section reads as aspirational. Either cut it or link to a concrete monitoring plan.

---

## What's Good

To be fair, the plan gets several things right:

- **Security-first design** — SSRF protection with IP pinning, HMAC signing, credential encryption. The approach correctly references the existing `plugin/webhook/webhook.go` pattern.
- **TCPA compliance awareness** — Opt-out handling, business identification requirements, HELP instructions. This is often missed in SMS integration plans.
- **Event-driven architecture** — Clean separation of event dispatch from integration-specific handlers. The event type taxonomy (`lead.captured`, `escalation.created`, etc.) maps well to existing `processChat()` state transitions.
- **Non-blocking design intent** — The stated goal of not failing the chat path when integrations fail is correct.
- **Good use of existing encryption module** — Correctly identifies `internal/crypto/encryption.go` with AES-256-GCM and Argon2id.

---

## Required Rework Before Approval

1. **Fix migration structure** to use versioned directories + create Postgres migrations
2. **Define the DI/wiring path** for `EncryptionService` into the agent service layer
3. **Design the retry mechanism** or explicitly declare v1 as best-effort
4. **Add `integration_id` to `agent_events`** table
5. **Rewrite UI snippets** using Joy UI, not Tailwind
6. **Decide on Twilio approach**: SDK vs thin HTTP wrapper, specify the dependency
7. **Define `AgentIntegration` Go struct** with proper JSON handling for `config_plaintext`
8. **Scope Phase 3 Calendar** down to single provider or defer
9. **Clarify async dispatch** in `processChat()` — goroutine vs channel vs inline

---

*Review version: 1.0*
*Verdict: REWORK — address the 5 critical and 6 major issues above, then re-submit for approval.*
