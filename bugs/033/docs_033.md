# Native Webhook Integrations v1 — Documentation

## Business Summary

**What:** bchat now sends real-time events to your existing tools (Zapier, n8n, webhooks) when leads are captured during chat conversations.

**Why:** Replaces expensive third-party integrations (Intercom AI at $97/mo, Helios AI at $100/mo) with native webhook support included in your bchat subscription.

**How it works:**
1. Configure a webhook URL in the Admin UI (Settings → Integrations)
2. When a visitor shares their name + email or phone during chat, bchat captures the lead
3. bchat immediately sends the lead data to your webhook URL with an HMAC signature for verification
4. If delivery fails, bchat retries every 5 minutes for up to 5 attempts

**What you get:**
- Real-time lead notifications to your CRM, Slack, email, or any webhook-compatible service
- HMAC-SHA256 signatures for security verification
- Event log in the Admin UI showing delivery status
- Test webhook button to verify configuration

---

## How to Test

### Prerequisites
1. bchat running with Postgres/SQLite database
2. At least one tenant configured
3. Admin access to the bchat UI

### Step 1: Run Migrations
```bash
# SQLite (auto-runs on startup)
# Postgres:
psql $DATABASE_URL < store/migration/postgres/0.31/00__agent_integrations.sql
psql $DATABASE_URL < store/migration/postgres/0.31/01__agent_events.sql
```

### Step 2: Set CRON_TOKEN
```bash
fly secrets set CRON_TOKEN=$(openssl rand -hex 32)
```

### Step 3: Create Webhook Integration
1. Go to Admin UI → Select tenant → Integrations section
2. Click "Add Webhook"
3. Enter a label (e.g., "Test Webhook")
4. Enter a webhook URL:
   - For testing: Use https://webhook.site or https://requestbin.com
   - For production: Use your Zapier/n8n webhook URL
5. Enter a secret (any string, used for HMAC signing)
6. Click "Create"

### Step 4: Test the Webhook
1. In the Integrations section, click the play button (▶) next to your webhook
2. Check webhook.site/requestbin for the test payload
3. Verify you see:
   ```json
   {
     "event": "test.ping",
     "tenant_id": 123,
     "timestamp": 1720000000,
     "data": { "message": "This is a test webhook from bchat" }
   }
   ```

### Step 5: Test Real Lead Capture
1. Open the chat widget as a visitor
2. Have a conversation and provide name + email or phone
3. Check webhook.site/requestbin for the lead payload
4. Verify the payload contains lead data (name, email, phone, topic)

### Step 6: Verify Event Log
1. In Admin UI → Integrations section, click "Show Events"
2. Verify events show status "delivered" (green) or "failed" (red)
3. Filter by status to see specific events

### Step 7: Test Failure Retry
1. Create a webhook with an invalid URL (e.g., https://httpbin.org/status/500)
2. Trigger a lead capture
3. Check events — status should be "processing" initially
4. Wait 5 minutes (or trigger cron manually)
5. Check events — should show increased attempts

### Manual Cron Trigger (Optional)
```bash
curl -X POST http://localhost:5230/api/v1/system/trigger-cron \
  -H "X-Cron-Token: YOUR_CRON_TOKEN"
```

---

## Technical Changes

### Database (4 migration files)
- `agent_integrations` table: Stores webhook configurations per tenant
- `agent_events` table: Outbox queue for reliable event delivery with idempotency and lease-based claiming

### Backend (6 files modified/created)
- **Webhook dispatch** (`integrations.go`): SSRF-protected HTTP delivery with HMAC signing
- **Event outbox** (`service.go`): Pre-claimed events for immediate delivery + 5-minute poller for retries
- **API endpoints**: CRUD for integrations, event log, test webhook, cron trigger
- **Postgres resilience** (`resilience.go`): Exponential backoff with transient error handling
- **MySQL stubs**: Compile safety for excluded driver

### Container (4 files)
- **Supercronic**: External cron runner polls event outbox every 5 minutes
- **Entrypoint**: Extended to launch supercronic before privilege drop
- **Dockerfile**: Installs supercronic binary and crontab

### Frontend (3 files)
- **Store**: Integration state and CRUD methods
- **IntegrationsSection**: Joy UI component for webhook management
- **AgentAdmin**: Integrated new section after Captured Leads

### Tests (1 file)
- HMAC signing verification
- SSRF protection (internal IP blocking)
- Idempotency key determinism (all components included)

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/agent/:slug/integrations` | List webhooks |
| POST | `/api/v1/agent/:slug/integrations` | Create webhook |
| PUT | `/api/v1/agent/:slug/integrations/:id` | Update webhook |
| DELETE | `/api/v1/agent/:slug/integrations/:id` | Delete webhook |
| POST | `/api/v1/agent/:slug/integrations/:id/test` | Send test webhook |
| GET | `/api/v1/agent/:slug/events` | List event log |
| POST | `/api/v1/system/trigger-cron` | Cron trigger (internal) |

---

## Webhook Payload

When a lead is captured, bchat sends:

```json
{
  "event": "lead.captured",
  "tenant_id": 123,
  "timestamp": 1720000000,
  "data": {
    "lead_id": "abc-123",
    "name": "John Smith",
    "email": "john@example.com",
    "phone": "+1-555-0123",
    "topic": "Water damage repair",
    "location": "123 Main St",
    "intent": "schedule_service",
    "session_id": "def-456"
  }
}
```

**Headers:**
- `X-Bchat-Signature: sha256=<hmac-hex>` — Verify with your webhook secret
- `X-Bchat-Event: lead.captured` — Event type
- `Content-Type: application/json`

---

## Configuration

### Environment Variables
```bash
# Set via fly secrets (NOT in fly.toml)
fly secrets set CRON_TOKEN=$(openssl rand -hex 32)
```

### fly.toml
```toml
kill_timeout = "30s"
```

---

## Limitations

- **Webhooks only**: SMS follow-ups deferred to v1.1
- **Awake-only retries**: Poller only runs while machine is awake; idle machines defer retries until next wake
- **SQLite write lock**: Event claiming acquires table-level lock (acceptable at v1 scale)

---

## Files Changed

| File | Action |
|------|--------|
| `store/migration/sqlite/0.31/00__agent_integrations.sql` | Created |
| `store/migration/sqlite/0.31/01__agent_events.sql` | Created |
| `store/migration/postgres/0.31/00__agent_integrations.sql` | Created |
| `store/migration/postgres/0.31/01__agent_events.sql` | Created |
| `store/agent.go` | Modified |
| `store/driver.go` | Modified |
| `store/db/sqlite/agent.go` | Modified |
| `store/db/postgres/agent.go` | Modified |
| `store/db/postgres/resilience.go` | Created |
| `store/db/mysql/agent.go` | Modified |
| `server/router/api/v1/agent/integrations.go` | Created |
| `server/router/api/v1/agent/integrations_test.go` | Created |
| `server/router/api/v1/agent/service.go` | Modified |
| `server/router/api/v1/v1.go` | Modified |
| `build/crontab` | Created |
| `scripts/entrypoint.sh` | Modified |
| `Dockerfile.pg.fly` | Modified |
| `fly.toml` | Modified |
| `web/src/store/v2/agentAdmin.ts` | Modified |
| `web/src/pages/AgentAdminSections/IntegrationsSection.tsx` | Created |
| `web/src/pages/AgentAdmin.tsx` | Modified |
