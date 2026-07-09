# Adversarial Code Review Prompt — Native Webhook Integrations v1

## Context

This is an adversarial code review of a **webhook integration implementation** for bchat, a multi-tenant AI chat agent platform. The implementation adds native webhook support so service businesses can capture leads via chat and send events to external systems (Zapier, n8n, etc.).

## Implementation Summary

### Phase 1: Database Migrations (4 files)
- SQLite: `store/migration/sqlite/0.31/00__agent_integrations.sql`, `01__agent_events.sql`
- Postgres: `store/migration/postgres/0.31/00__agent_integrations.sql`, `01__agent_events.sql`
- Schema: `agent_integrations` (webhook config), `agent_events` (outbox queue with idempotency, lease-based claiming)
- Key design: `status DEFAULT 'processing'` (pre-claimed on insert), `claimed_at` for lease-based reclaim, `attempts` counter with max 5

### Phase 2: Store Layer (6 files)
- `store/agent.go` — Added AgentIntegration, AgentEvent, WebhookConfig, Find structs
- `store/driver.go` — Added 9 interface methods (CRUD + ClaimPendingEvents)
- `store/db/sqlite/agent.go` — SQLite implementations with `RETURNING` for claim query
- `store/db/postgres/agent.go` — Postgres implementations with `FOR UPDATE SKIP LOCKED`
- `store/db/postgres/resilience.go` — `RunResiliently` (exponential backoff), `isTransientError`, unique violation as success
- `store/db/mysql/agent.go` — Stubs returning `errNotImplemented`

### Phase 3: Service Layer (4 files)
- `server/router/api/v1/agent/integrations.go` — SSRF-protected webhook delivery (copied from plugin/webhook), HMAC signing, idempotency keys, CRUD handlers, trigger-cron handler with constant-time compare
- `server/router/api/v1/agent/service.go` — `dispatchEvent` (pre-claimed insert + goroutine delivery), `processEventPoller` (lease-based claim + deliver), `captureLeadFromSession` modified to dispatch events
- `server/router/api/v1/v1.go` — Routes on adminGroup + bare system route for trigger-cron

### Phase 4: Container Setup (4 files)
- `build/crontab` — Supercronic schedule (*/5 * * * *)
- `scripts/entrypoint.sh` — Extended with supercronic launch before `exec gosu memos` (preserves _FILE secret expansion + privilege drop)
- `Dockerfile.pg.fly` — Installs supercronic, copies crontab
- `fly.toml` — Added top-level `kill_timeout = "30s"`

### Phase 5: Frontend UI (3 files)
- `web/src/store/v2/agentAdmin.ts` — Added AgentIntegration, AgentEvent interfaces, state, and CRUD methods
- `web/src/pages/AgentAdminSections/IntegrationsSection.tsx` — Joy UI + Tailwind component (list, add, toggle, delete, test, event log)
- `web/src/pages/AgentAdmin.tsx` — Imported and rendered IntegrationsSection

### Phase 6: Tests (1 file)
- `server/router/api/v1/agent/integrations_test.go` — TestSignPayload, TestIsInternalIP, TestComputeIdempotencyKey (all passing)

## Review Scope

Review ALL files listed above for:
1. **Security** — SSRF, HMAC, idempotency, tenant isolation, secret handling
2. **Correctness** — SQL queries, claim semantics, failure modes, race conditions
3. **Postgres resilience** — FOR UPDATE SKIP LOCKED usage, transient error handling
4. **Entrypoint safety** — _FILE expansion, privilege drop, supercronic lifecycle
5. **Frontend** — Store patterns, component structure, permission gates
6. **Edge cases** — Empty integrations, duplicate events, max attempts, stale claims

## Specific Questions

1. Does `dispatchEvent` correctly handle the failure path (leave as processing, don't reset claimed_at)?
2. Is the `ClaimPendingEvents` query correct for both SQLite (single-writer) and Postgres (FOR UPDATE SKIP LOCKED)?
3. Is the SSRF protection complete (scheme validation, DNS resolution, IP blocking, redirect re-validation)?
4. Does the entrypoint correctly preserve _FILE secret expansion while adding supercronic?
5. Are there any race conditions between immediate dispatch goroutine and poller?
6. Is the idempotency key deterministic and collision-resistant?
7. Does the frontend correctly gate admin-only operations?
8. Are the MySQL stubs complete and safe?
9. Is the `kill_timeout` placement correct (top-level, not under [processes])?
10. Does the trigger-cron handler use constant-time comparison for the token?

## Files to Review

All files in `store/migration/sqlite/0.31/`, `store/migration/postgres/0.31/`, `store/agent.go`, `store/driver.go`, `store/db/sqlite/agent.go`, `store/db/postgres/agent.go`, `store/db/postgres/resilience.go`, `store/db/mysql/agent.go`, `server/router/api/v1/agent/integrations.go`, `server/router/api/v1/agent/integrations_test.go`, `server/router/api/v1/agent/service.go`, `server/router/api/v1/v1.go`, `build/crontab`, `scripts/entrypoint.sh`, `Dockerfile.pg.fly`, `fly.toml`, `web/src/store/v2/agentAdmin.ts`, `web/src/pages/AgentAdminSections/IntegrationsSection.tsx`, `web/src/pages/AgentAdmin.tsx`.
