# Implementation Plan: bchat Native Integrations (Plan 4)

Based on the adversarial review of the previous plan, we are proceeding with **"Native SMS with tenant-provided Twilio credentials" (v1)**. This plan strictly aligns with the codebase's existing architecture, abandoning the "same transaction" myth, dropping MySQL, and implementing a robust lease-based outbox for Fly.io/Neon.

## User Review Required

> [!IMPORTANT]
> - **V1 Scope Definition:** Tenants will provide their own Twilio credentials. Master-account subaccount provisioning is deferred to v2. 
> - **Supported DBs:** Features target **SQLite** and **PostgreSQL** only. MySQL schema is frozen at 0.25 and not supported for agent features.
> - **Failure Window Acknowledgment:** The event outbox is inserted immediately after the lead. If the process dies between these two statements, the event is lost. This is acceptable for v1 as the lead itself is durable and visible in the admin UI.

---

## Proposed Changes

### 1. Database Migrations & Schema (SQLite & Postgres)

*MySQL is dropped from scope. The new tables will only be added to SQLite and PostgreSQL migrations.*

#### [NEW] [store/migration/sqlite/0.31/00__agent_integrations.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/00__agent_integrations.sql)
#### [NEW] [store/migration/postgres/0.31/00__agent_integrations.sql](file:///home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/00__agent_integrations.sql)
- Add `agent_integrations` table.

#### [NEW] [store/migration/sqlite/0.31/01__agent_events.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/01__agent_events.sql)
#### [NEW] [store/migration/postgres/0.31/01__agent_events.sql](file:///home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/01__agent_events.sql)
- Add `agent_events` table (The Outbox).
- **Idempotency & Lease:** Add `idempotency_key TEXT UNIQUE` (for both SQLite and PG for consistency) and `claimed_at BIGINT`.

#### [NEW] [store/migration/sqlite/0.31/02__agent_sms.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/02__agent_sms.sql)
#### [NEW] [store/migration/postgres/0.31/02__agent_sms.sql](file:///home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/02__agent_sms.sql)
- Add `agent_sms_messages` and `agent_sms_optouts` tables (optouts table is for v2, v1 relies on Twilio carrier-side Advanced Opt-Out).
- Add `idempotency_key TEXT UNIQUE` and `claimed_at BIGINT` to `agent_sms_messages`.

---

### 2. Store Layer (Resilience & Leased Concurrency)

#### [MODIFY] [store/agent.go](file:///home/chaschel/Documents/go/bchat/store/agent.go)
- Define `AgentIntegration`, `AgentEvent`, `AgentSMSMessage`, `AgentSMSOptOut` structs.
- Define typed config structs (`WebhookConfig`, `TwilioConfig`).

#### [MODIFY] [store/driver.go](file:///home/chaschel/Documents/go/bchat/store/driver.go) & [store/db/postgres/agent.go](file:///home/chaschel/Documents/go/bchat/store/db/postgres/agent.go) & [store/db/sqlite/agent.go](file:///home/chaschel/Documents/go/bchat/store/db/sqlite/agent.go)
- Add `ClaimPendingEvents(limit int) ([]*AgentEvent, error)` and `ClaimPendingSMS(limit int) ([]*AgentSMSMessage, error)`.
- **Claim-then-Release Lease:** 
  - PG: Use `UPDATE agent_events SET status = 'processing', claimed_at = EXTRACT(EPOCH FROM NOW()) WHERE id IN (SELECT id FROM agent_events WHERE (status = 'pending') OR (status = 'processing' AND claimed_at < EXTRACT(EPOCH FROM NOW()) - 300) FOR UPDATE SKIP LOCKED LIMIT X) RETURNING *`.
  - SQLite: Emulate with a simple status update and returning the rows.
- This ensures stale claims (machine killed mid-delivery) are reclaimed after 5 minutes. Consumers must dedupe via `idempotency_key` (at-least-once delivery).

#### [NEW] [store/db/postgres/resilience.go](file:///home/chaschel/Documents/go/bchat/store/db/postgres/resilience.go)
- Implement `isTransientError(err)` checking for `57P01`, `08006`, `08003`.
- Implement `RunResiliently(fn)` with exponential backoff (1s-16s) and Jitter. 
- **Critical Constraints:** 
  - **Success on `23505`:** The wrapper must treat unique-violations (SQLSTATE `23505`) as a success state (i.e. retry after lost ack). 
  - **Scoped usage:** This wrapper will ONLY be applied to (a) read queries, (b) writes with an `idempotency_key`, and (c) whole logical units outside transactions.

---

### 3. Agent Service Layer (Outbox & Delivery)

#### [MODIFY] [server/router/api/v1/agent/service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go)
- **Immediate + Poller Dispatch:** After a lead is inserted, insert the `AgentEvent` in a separate statement. Generate a **deterministic idempotency key** (e.g., `hash(tenantID, leadID, "lead.captured")`). 
- Spawn a detached goroutine (using `context.Background()`) for an immediate best-effort delivery to ensure near-zero latency in the common case.
- Initialize the vendored `plugin/cron` to poll the lease claims as the catch-up/retry path. 

#### [MODIFY] [server/server.go](file:///home/chaschel/Documents/go/bchat/server/server.go) & [fly.toml](file:///home/chaschel/Documents/go/bchat/fly.toml)
- Wire `Server.Shutdown` -> agent `Service.Stop()` -> `cron.Stop()`. Wait with a bounded timeout (`sync.WaitGroup`).
- Add `kill_timeout = 30` to `fly.toml` so Fly.io waits for our graceful shutdown.
- *Note: Since we have the 5-minute lease reclaim, graceful shutdown is an optimization. A hard kill is safe.*

#### [NEW] [server/router/api/v1/agent/integrations.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/integrations.go)
- Implement `deliverWebhook` with SSRF protection (IP pinning). Omit numeric `TenantID` in headers.

#### [NEW] [server/router/api/v1/agent/sms.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/sms.go)
- SMS methods (`SendSMS`) live on `*Service`. 
- `TwilioClient` is a stateless HTTP helper.
- Use `phonenumbers.Parse` for number validation.

---

### 4. API & Handlers (Solving Scale-to-Zero)

#### [MODIFY] [server/router/api/v1/agent/handlers.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go)
- **`POST /api/v1/system/trigger-cron`:** 
  - Authenticated via `X-Cron-Token` shared secret. 
  - Returns `202 Accepted` immediately. 
  - Single-flight lock (mutex flag) to prevent overlapping manual polls. Runs the poller async. This fully supports Fly's scale-to-zero (Scheduler Story A).
- **CRUD Endpoints:**
  - `GET /api/v1/agent/:slug/integrations` (List)
  - `POST /api/v1/agent/:slug/integrations` (Create)
  - `PUT /api/v1/agent/:slug/integrations/:id` (Update)
  - `DELETE /api/v1/agent/:slug/integrations/:id` (Delete)
  - `POST /api/v1/agent/:slug/integrations/:id/test` (Test Webhook)
  - `GET /api/v1/agent/:slug/events` (Event Log for UI delivery status)

#### [MODIFY] [server/router/api/v1/v1.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/v1.go)
- Register the new API routes.

---

### 5. Frontend Admin UI

#### [MODIFY] [web/src/pages/AgentAdmin.tsx](file:///home/chaschel/Documents/go/bchat/web/src/pages/AgentAdmin.tsx)
- Build "Integrations" section (Joy UI + Tailwind CSS).

#### [MODIFY] [web/src/store/v2/agentAdmin.ts](file:///home/chaschel/Documents/go/bchat/web/src/store/v2/agentAdmin.ts)
- Add MobX state for integrations and event logs.

## Verification Plan

### Automated Tests
- `TestSignPayload`: Ensure HMAC correctness.
- `TestValidateWebhookURL`: Ensure SSRF blocks internal IPs.
- `TestPostgresTransientRetry`: Verify exponential backoff fires on `57P01`.
- `TestIdempotentInsert_UniqueViolation`: Verify that `SQLSTATE 23505` correctly short-circuits as a success in the retry wrapper.
- `TestStaleClaimReclaim`: Verify the `claimed_at < now() - 300` logic works correctly.
- `TestTriggerCronAuth`: Verify unauthenticated pings to `/api/v1/system/trigger-cron` are rejected (401).

### Manual Verification
- Start server using PostgreSQL. Add `connect_timeout=15` to DSN.
- Trigger lead capture; verify `agent_events` row has deterministic `idempotency_key`.
- Verify the immediate dispatch successfully delivers the webhook, and the cron catch-up ignores it (status `delivered`).
- Simulate `SIGTERM` and verify bounded graceful shutdown wait.
