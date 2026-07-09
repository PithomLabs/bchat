# Implementation Plan: bchat Native Integrations (Webhooks & SMS)

Based on our discussion, we are proceeding with **Path A (Platform Managed)** to provide a seamless, "batteries-included" experience for tenants. This implementation is based on `bugs/033/plan2.md`, but directly incorporates all the fixes for the critical and minor issues identified during the adversarial review.

## User Review Required

> [!IMPORTANT]
> Since we are proceeding with Path A, bchat will directly handle Twilio API calls and SMS cron jobs. Please confirm this final technical design before I begin execution.

## Proposed Changes

We will implement Phase 1 (Webhooks) and Phase 2 (SMS) in a single unified push.

### Database Migrations

Create migrations for SQLite, PostgreSQL, and MySQL.

#### [NEW] [store/migration/sqlite/0.31/00__agent_integrations.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/00__agent_integrations.sql)
#### [NEW] [store/migration/sqlite/0.31/01__agent_events.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/01__agent_events.sql)
#### [NEW] [store/migration/sqlite/0.31/02__agent_sms.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/02__agent_sms.sql)
#### [NEW] [store/migration/postgres/0.31/00__agent_integrations.sql](file:///home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/00__agent_integrations.sql)
#### [NEW] [store/migration/postgres/0.31/01__agent_events.sql](file:///home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/01__agent_events.sql)
#### [NEW] [store/migration/postgres/0.31/02__agent_sms.sql](file:///home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/02__agent_sms.sql)
#### [NEW] [store/migration/mysql/0.31/00__agent_integrations.sql](file:///home/chaschel/Documents/go/bchat/store/migration/mysql/0.31/00__agent_integrations.sql)
#### [NEW] [store/migration/mysql/0.31/01__agent_events.sql](file:///home/chaschel/Documents/go/bchat/store/migration/mysql/0.31/01__agent_events.sql)
#### [NEW] [store/migration/mysql/0.31/02__agent_sms.sql](file:///home/chaschel/Documents/go/bchat/store/migration/mysql/0.31/02__agent_sms.sql)
- Add `agent_integrations`, `agent_events`, `agent_sms_messages`, and `agent_sms_optouts` tables across all 3 supported database backends.

---

### Store Layer

#### [MODIFY] [store/agent.go](file:///home/chaschel/Documents/go/bchat/store/agent.go)
- Define `AgentIntegration`, `AgentEvent`, `AgentSMSMessage`, `AgentSMSOptOut` structs.
- Define strongly-typed config structs (`WebhookConfig`, `TwilioConfig`).

#### [MODIFY] [store/driver.go](file:///home/chaschel/Documents/go/bchat/store/driver.go)
- Add CRUD interface methods for the new entities.

#### [MODIFY] [store/db/sqlite/agent.go](file:///home/chaschel/Documents/go/bchat/store/db/sqlite/agent.go)
- Implement all SQLite queries for the new integration methods.

#### [MODIFY] [store/db/postgres/agent.go](file:///home/chaschel/Documents/go/bchat/store/db/postgres/agent.go)
- Implement all Postgres queries for the new integration methods.

#### [MODIFY] [store/db/mysql/agent.go](file:///home/chaschel/Documents/go/bchat/store/db/mysql/agent.go)
- Implement all MySQL queries for the new integration methods.

---

### Agent Service Layer

#### [MODIFY] [server/router/api/v1/agent/service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go)
- Initialize a `robfig/cron` instance for processing queued SMS messages (using a `LIMIT` to prevent unbounded memory usage).
- Add `dispatchEvent(ctx context.Context, tenantID int32, eventType string, data interface{})` to handle webhook and SMS routing.
- **Critical Fix:** Use `context.WithoutCancel(ctx)` or `context.Background()` inside the fire-and-forget goroutine to ensure HTTP requests succeed after the parent Echo request terminates.
- Wire event dispatching into the end of `processChat()` for `lead.captured` and `escalation.created`.

#### [NEW] [server/router/api/v1/agent/integrations.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/integrations.go)
- Implement `deliverWebhook` with SSRF protection (IP pinning, no internal IPs), borrowing logic from the existing webhook plugin.
- Do not expose numeric `TenantID` in headers; use slug or omit.

#### [NEW] [server/router/api/v1/agent/sms.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/sms.go)
- Implement a thin Twilio HTTP client (no bulky external SDK).
- Implement SMS sending logic as methods on `*Service` to maintain architectural consistency (avoids parallel store access).
- Add proper phone number validation using `phonenumbers.Parse`.

---

### API & Handlers

#### [MODIFY] [server/router/api/v1/agent/handlers.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go)
- Add handlers for `HandleListIntegrations`, `HandleCreateIntegration`, `HandleUpdateIntegration`, `HandleDeleteIntegration`, and `HandleTestIntegration`.
- Ensure endpoints enforce `tenant:admin` or `tenant:read` RBAC permissions as appropriate.

#### [MODIFY] [server/router/api/v1/v1.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/v1.go)
- Register the new API routes.

---

### Frontend Admin UI

#### [MODIFY] [web/src/pages/AgentAdmin.tsx](file:///home/chaschel/Documents/go/bchat/web/src/pages/AgentAdmin.tsx)
- Build the "Integrations" section below the "Captured Leads" panel.
- Use a hybrid of **Joy UI** components (Buttons, Modals, Switches) styled with **Tailwind CSS** (for layout), matching the existing codebase conventions.

#### [MODIFY] [web/src/store/v2/agentAdmin.ts](file:///home/chaschel/Documents/go/bchat/web/src/store/v2/agentAdmin.ts)
- Add MobX state and async actions for fetching, creating, testing, and toggling integrations.

## Verification Plan

### Automated Tests
- Build `TestSignPayload` to ensure HMAC correctness.
- Build `TestValidateWebhookURL` to ensure SSRF blocks internal metadata IPs (`169.254.169.254`, `127.0.0.1`, etc.).
- Build `TestTwilioClient_SendMessage` to ensure form-encoding is correct.

### Manual Verification
- Start the server using SQLite.
- Provision a test webhook via the Admin UI.
- Use `HandleTestIntegration` to verify the payload arrives at a test endpoint (e.g., webhook.site).
- Trigger a lead capture via the chat widget and verify the event is dispatched in the background without blocking the LLM response.
