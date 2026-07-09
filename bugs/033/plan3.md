# Implementation Plan: bchat Native Integrations (Webhooks & SMS)

Based on our discussion, we are proceeding with **Path A (Platform Managed)**. This revised plan incorporates both the fixes from the adversarial review AND the robust, serverless-ready architecture required for Fly.io/Neon (Outbox pattern, distributed locking, and Postgres connection resilience).

## User Review Required

> [!IMPORTANT]
> This plan now includes advanced database resilience features (Idempotency Keys, `FOR UPDATE SKIP LOCKED`, exponential backoff) to survive Fly.io machine restarts and Neon scale-to-zero cold starts. Please confirm this final architecture.

## Proposed Changes

We will implement Phase 1 (Webhooks) and Phase 2 (SMS) in a single unified push, engineered for a distributed environment.

### 1. Database Migrations & Schema

Create migrations for SQLite, PostgreSQL, and MySQL.

#### [NEW] [store/migration/sqlite/0.31/00__agent_integrations.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/00__agent_integrations.sql)
#### [NEW] [store/migration/postgres/0.31/00__agent_integrations.sql](file:///home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/00__agent_integrations.sql)
#### [NEW] [store/migration/mysql/0.31/00__agent_integrations.sql](file:///home/chaschel/Documents/go/bchat/store/migration/mysql/0.31/00__agent_integrations.sql)
- Add `agent_integrations` table.

#### [NEW] [store/migration/sqlite/0.31/01__agent_events.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/01__agent_events.sql)
#### [NEW] [store/migration/postgres/0.31/01__agent_events.sql](file:///home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/01__agent_events.sql)
#### [NEW] [store/migration/mysql/0.31/01__agent_events.sql](file:///home/chaschel/Documents/go/bchat/store/migration/mysql/0.31/01__agent_events.sql)
- Add `agent_events` table (The Transactional Outbox).
- **Resilience Addition:** Add `idempotency_key UUID UNIQUE` to prevent duplicate event dispatching if a connection drops right after a commit but before the app receives the DB acknowledgment.

#### [NEW] [store/migration/sqlite/0.31/02__agent_sms.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/02__agent_sms.sql)
#### [NEW] [store/migration/postgres/0.31/02__agent_sms.sql](file:///home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/02__agent_sms.sql)
#### [NEW] [store/migration/mysql/0.31/02__agent_sms.sql](file:///home/chaschel/Documents/go/bchat/store/migration/mysql/0.31/02__agent_sms.sql)
- Add `agent_sms_messages` and `agent_sms_optouts` tables.
- **Resilience Addition:** Add `idempotency_key UUID UNIQUE` to `agent_sms_messages` to ensure scheduled messages aren't inserted twice on retries.

---

### 2. Store Layer (Resilience & Concurrency)

#### [MODIFY] [store/agent.go](file:///home/chaschel/Documents/go/bchat/store/agent.go)
- Define `AgentIntegration`, `AgentEvent`, `AgentSMSMessage`, `AgentSMSOptOut` structs.
- Define strongly-typed config structs (`WebhookConfig`, `TwilioConfig`).

#### [MODIFY] [store/driver.go](file:///home/chaschel/Documents/go/bchat/store/driver.go)
- Add `ListAndLockPendingEvents()` and `ListAndLockPendingSMS()` methods. 

#### [MODIFY] [store/db/postgres/agent.go](file:///home/chaschel/Documents/go/bchat/store/db/postgres/agent.go)
- Implement `ListAndLockPendingEvents` using **`SELECT ... FOR UPDATE SKIP LOCKED LIMIT X`**. This guarantees that if 5 Fly.io machines wake up at the same time, they safely grab different batches of tasks without double-sending.

#### [NEW] [store/db/postgres/resilience.go](file:///home/chaschel/Documents/go/bchat/store/db/postgres/resilience.go)
- Implement Go retry wrappers for database queries with exponential backoff (e.g., 1s to 16s) and Jitter.
- Implement `isTransientError(err error) bool` checking for Neon-specific `SQLSTATE` codes (`57P01`, `08006`, `08003`) and connection drop error strings. 
- Ensure connection timeouts in the pool are set to `15s` to handle Neon scale-to-zero cold starts.

#### [MODIFY] [store/db/sqlite/agent.go](file:///home/chaschel/Documents/go/bchat/store/db/sqlite/agent.go) & [store/db/mysql/agent.go](file:///home/chaschel/Documents/go/bchat/store/db/mysql/agent.go)
- Implement queue queries (simulating row locks where applicable, or relying on status updates for SQLite).

---

### 3. Agent Service Layer (Outbox & Delivery)

#### [MODIFY] [server/router/api/v1/agent/service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go)
- **Transactional Outbox:** Instead of fire-and-forget goroutines, `processChat()` will insert an `AgentEvent` into the database in the same transaction. 
- Initialize a `robfig/cron` instance to poll `ListAndLockPendingEvents` and `ListAndLockPendingSMS`.
- Implement graceful shutdown using `sync.WaitGroup` to wait for active webhook/Twilio HTTP calls to finish when Fly.io sends `SIGTERM`.

#### [NEW] [server/router/api/v1/agent/integrations.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/integrations.go)
- Implement `deliverWebhook` with SSRF protection (IP pinning, no internal IPs).
- Do not expose numeric `TenantID` in headers; use slug or omit.

#### [NEW] [server/router/api/v1/agent/sms.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/sms.go)
- Implement a thin Twilio HTTP client (no SDK dependency).
- Add proper phone number validation using `phonenumbers.Parse`.

---

### 4. API & Handlers (Solving Scale-to-Zero)

#### [MODIFY] [server/router/api/v1/agent/handlers.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go)
- Add `HandleTriggerCron` endpoint. This allows an external ping (like a Fly.io scheduled task or UptimeRobot) to hit the API every minute, waking up the Fly.io machine if it scaled to zero, ensuring delayed SMS goes out on time.
- Add standard CRUD handlers for Integrations (`HandleListIntegrations`, `HandleCreateIntegration`, etc.).

#### [MODIFY] [server/router/api/v1/v1.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/v1.go)
- Register the new API routes.

---

### 5. Frontend Admin UI

#### [MODIFY] [web/src/pages/AgentAdmin.tsx](file:///home/chaschel/Documents/go/bchat/web/src/pages/AgentAdmin.tsx)
- Build the "Integrations" section below the "Captured Leads" panel.
- Use a hybrid of **Joy UI** components styled with **Tailwind CSS**.

#### [MODIFY] [web/src/store/v2/agentAdmin.ts](file:///home/chaschel/Documents/go/bchat/web/src/store/v2/agentAdmin.ts)
- Add MobX state and async actions for fetching, creating, testing, and toggling integrations.

## Verification Plan

### Automated Tests
- Build `TestSignPayload` to ensure HMAC correctness.
- Build `TestValidateWebhookURL` to ensure SSRF blocks internal IPs.
- Build `TestPostgresTransientRetry` to simulate `57P01` errors and verify the exponential backoff wrapper fires.
- Build `TestIdempotentInsert` to verify duplicate inserts with the same idempotency key are safely ignored.

### Manual Verification
- Start the server using PostgreSQL.
- Trigger a lead capture; verify the `agent_events` row is created with an `idempotency_key`.
- Verify the cron job picks up the pending event, dispatches the webhook, and updates the status to `delivered`.
- Stop the server mid-dispatch (simulate `SIGTERM`) and verify the graceful shutdown logic waits for the HTTP call to complete.



## Summary

I have successfully updated the `implementation_plan.md` artifact! 

It now incorporates the following advanced distributed systems and database resilience techniques that are essential for running this reliably on Fly.io and Neon:

1. **Transactional Outbox & Idempotency Keys:** `agent_events` and `agent_sms_messages` now get a `UNIQUE UUID idempotency_key`. This guarantees that if a Neon Postgres compute node restarts right as a webhook event is being committed, the retry mechanism won't accidentally duplicate the data.
2. **Distributed Queue Locking:** The plan specifies `SELECT ... FOR UPDATE SKIP LOCKED` for the Postgres implementation. This ensures multiple Fly.io instances can't double-send the same queued SMS simultaneously. 
3. **Transient Retry Wrapper:** A new `resilience.go` layer will wrap database calls with exponential backoff and jitter, specifically intercepting the `57P01`, `08006`, and `08003` SQLSTATE codes mentioned in the resilience guide to survive Neon's cold-starts and compute restarts. 
4. **Solving Scale-to-Zero:** Added the `POST /api/v1/system/trigger-cron` endpoint so you can configure a lightweight external cron-ping to wake the app up on schedule, bypassing Fly.io's scale-to-zero sleeping behavior when you have pending background tasks.

Please review the updated `implementation_plan.md` artifact. If this looks robust enough for production, let me know and we can finally start writing the code!