# Implementation Plan — Native Webhook & SMS Integrations (v1)

> Source: `plan4.md` (APPROVED FINAL) + `plan4_review.md` (APPROVED with amendments A-D)
> Scope: Webhooks only. Scheduled SMS deferred to v1.1.
> Cron: supercronic (external process, not in-process Go library)

---

## 1. Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│  Fly.io Container                                        │
│                                                          │
│  entrypoint.sh (PID 1)                                   │
│  ├─ start bchat server (./build/memos --mode prod)       │
│  └─ start supercronic (/etc/bchat/crontab)               │
│      └─ every 5 min:                                     │
│         curl -X POST http://localhost:5230/               │
│           api/v1/system/trigger-cron                      │
│           -H "X-Cron-Token: ${CRON_TOKEN}"               │
│                                                          │
│  bchat server                                            │
│  ├─ POST /api/v1/agent/:slug/integrations  (CRUD)       │
│  ├─ POST /api/v1/agent/:slug/integrations/:id/test      │
│  ├─ POST /api/v1/system/trigger-cron        (cron auth)  │
│  └─ captureLeadFromSession()                             │
│      ├─ insert lead                                      │
│      ├─ insert event (status=processing, claimed_at=now) │
│      └─ spawn goroutine → deliverWebhook()               │
│                                                          │
│  processEventPoller() (via trigger-cron)                  │
│  └─ poll pending events → deliver webhooks               │
└─────────────────────────────────────────────────────────┘
```

---

## 2. Scope Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scheduled SMS | **Defer to v1.1** | Webhooks alone deliver value; SMS adds ~2 days complexity |
| Cron mechanism | **supercronic** | External process, standard cron syntax, container-optimized |
| Frontend split | **New component file** | Keeps AgentAdmin.tsx manageable |
| Testing | **Plain `testing` package** | No external test framework |
| MySQL stubs | **Add one-line stubs** | Amendment A — compile safety |
| Event claim | **Pre-claimed on insert** | Amendment B — eliminates failure window |
| Max attempts | **Column default 0** | Amendment C — safe by default |
| SMS enqueue | **Deferred** | Amendment D — keep v1 focused |

---

## 3. Database Migrations

### 3.1 SQLite Migrations

**File: `store/migration/sqlite/0.31/00__agent_integrations.sql`**
```sql
CREATE TABLE IF NOT EXISTS agent_integrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    integration_type TEXT NOT NULL,          -- 'webhook' | 'twilio'
    label TEXT NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',       -- JSON: url, secret, headers (webhook) | account_sid, auth_token, from_number (twilio)
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
    updated_at BIGINT NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_integrations_tenant ON agent_integrations(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_integrations_tenant_type ON agent_integrations(tenant_id, integration_type);
```

**File: `store/migration/sqlite/0.31/01__agent_events.sql`**
```sql
CREATE TABLE IF NOT EXISTS agent_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    integration_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,                -- 'lead.captured'
    payload TEXT NOT NULL DEFAULT '{}',      -- JSON payload
    status TEXT NOT NULL DEFAULT 'processing', -- 'pending' | 'processing' | 'delivered' | 'failed'
    claimed_at BIGINT DEFAULT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT DEFAULT NULL,
    idempotency_key TEXT UNIQUE,             -- hash(tenantID, leadID, eventType)
    created_at BIGINT NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (integration_id) REFERENCES agent_integrations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_events_tenant ON agent_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_events_status ON agent_events(status);
CREATE INDEX IF NOT EXISTS idx_agent_events_claimed ON agent_events(claimed_at);
```

**File: `store/migration/sqlite/0.31/02__agent_sms_deferred.sql`**
```sql
-- Placeholder for v1.1 SMS tables
-- Tables: agent_sms_messages, agent_sms_optouts
-- Deferred per scope decision
SELECT 1;
```

### 3.2 Postgres Migrations

**File: `store/migration/postgres/0.31/00__agent_integrations.sql`**
```sql
CREATE TABLE IF NOT EXISTS agent_integrations (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    integration_type TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    updated_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_integrations_tenant ON agent_integrations(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_integrations_tenant_type ON agent_integrations(tenant_id, integration_type);
```

**File: `store/migration/postgres/0.31/01__agent_events.sql`**
```sql
CREATE TABLE IF NOT EXISTS agent_events (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    integration_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'processing',
    claimed_at BIGINT DEFAULT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT DEFAULT NULL,
    idempotency_key TEXT UNIQUE,
    created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (integration_id) REFERENCES agent_integrations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_events_tenant ON agent_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_events_status ON agent_events(status);
CREATE INDEX IF NOT EXISTS idx_agent_events_claimed ON agent_events(claimed_at);
```

**File: `store/migration/postgres/0.31/02__agent_sms_deferred.sql`**
```sql
-- Placeholder for v1.1 SMS tables
SELECT 1;
```

---

## 4. Store Layer

### 4.1 New Structs (`store/agent.go` — append after line 928)

```go
// AgentIntegration represents a configured integration for a tenant.
type AgentIntegration struct {
    ID              int32
    TenantID        int32
    IntegrationType string  // "webhook" | "twilio"
    Label           string
    Config          string  // JSON-encoded config
    IsActive        bool
    CreatedAt       int64
    UpdatedAt       int64
}

// AgentEvent represents an outbound event to be delivered via integration.
type AgentEvent struct {
    ID             int32
    TenantID       int32
    IntegrationID  int32
    EventType      string
    Payload        string  // JSON payload
    Status         string  // "pending" | "processing" | "delivered" | "failed"
    ClaimedAt      *int64
    Attempts       int32
    LastError      *string
    IdempotencyKey *string
    CreatedAt      int64
}

// WebhookConfig holds typed webhook configuration.
type WebhookConfig struct {
    URL     string            `json:"url"`
    Secret  string            `json:"secret"`
    Headers map[string]string `json:"headers,omitempty"`
}

// TwilioConfig holds typed Twilio configuration (for v1.1).
type TwilioConfig struct {
    AccountSID string `json:"account_sid"`
    AuthToken  string `json:"auth_token"`
    FromNumber string `json:"from_number"`
}

// FindAgentIntegration is the query filter for agent integrations.
type FindAgentIntegration struct {
    ID              *int32
    TenantID        *int32
    IntegrationType *string
}

// FindAgentEvent is the query filter for agent events.
type FindAgentEvent struct {
    ID        *int32
    TenantID  *int32
    Status    *string
    EventID   *int32
}
```

### 4.2 Driver Interface (`store/driver.go` — append after line 278)

```go
// Agent Integration CRUD
CreateAgentIntegration(ctx context.Context, integration *AgentIntegration) (*AgentIntegration, error)
GetAgentIntegration(ctx context.Context, find *FindAgentIntegration) (*AgentIntegration, error)
ListAgentIntegrations(ctx context.Context, find *FindAgentIntegration) ([]*AgentIntegration, error)
UpdateAgentIntegration(ctx context.Context, update *AgentIntegration) error
DeleteAgentIntegration(ctx context.Context, id int32) error

// Agent Event CRUD + claim
CreateAgentEvent(ctx context.Context, event *AgentEvent) (*AgentEvent, error)
ListAgentEvents(ctx context.Context, find *FindAgentEvent) ([]*AgentEvent, error)
ClaimPendingEvents(ctx context.Context, limit int32) ([]*AgentEvent, error)
UpdateAgentEvent(ctx context.Context, update *AgentEvent) error
```

### 4.3 SQLite Implementation (`store/db/sqlite/agent.go` — append after line 1936)

Key implementation details:
- `ClaimPendingEvents`: Single-writer claim (no `FOR UPDATE SKIP LOCKED`):
  ```sql
  UPDATE agent_events
  SET status = 'processing', claimed_at = CAST(strftime('%s','now') AS INTEGER), attempts = attempts + 1
  WHERE id IN (
      SELECT id FROM agent_events
      WHERE (status = 'pending' AND attempts < 5)
         OR (status = 'processing' AND claimed_at < CAST(strftime('%s','now') AS INTEGER) - 300 AND attempts < 5)
      LIMIT ?
  )
  RETURNING *
  ```

### 4.4 Postgres Implementation (`store/db/postgres/agent.go` — append after line 2477)

Key implementation details:
- `ClaimPendingEvents`: With `FOR UPDATE SKIP LOCKED`:
  ```sql
  UPDATE agent_events
  SET status = 'processing', claimed_at = EXTRACT(EPOCH FROM NOW())::BIGINT, attempts = attempts + 1
  WHERE id IN (
      SELECT id FROM agent_events
      WHERE (status = 'pending' AND attempts < 5)
         OR (status = 'processing' AND claimed_at < EXTRACT(EPOCH FROM NOW())::BIGINT - 300 AND attempts < 5)
      FOR UPDATE SKIP LOCKED
      LIMIT $1
  )
  RETURNING *
  ```

### 4.5 Postgres Resilience (`store/db/postgres/resilience.go` — new file)

```go
func isTransientError(err error) bool {
    // Check for 57P01 (admin_shutdown), 08006 (connection_failure), 08003 (connection_does_not_exist)
}

func RunResiliently(fn func() error) error {
    // Exponential backoff: 1s, 2s, 4s, 8s, 16s + jitter
    // Treat 23505 (unique_violation) as success
}
```

### 4.6 MySQL Stubs (`store/db/mysql/agent.go` — append after line 884)

```go
func (*MySQLDriver) CreateAgentIntegration(_ context.Context, _ *store.AgentIntegration) (*store.AgentIntegration, error) {
    return nil, errNotImplemented
}
// ... same pattern for all new methods
```

---

## 5. Agent Service Layer

### 5.1 Webhook Dispatch (`server/router/api/v1/agent/integrations.go` — new file)

~300 lines. Contains:

- `deliverWebhook(ctx, wh, eventType, payload)` — SSRF-protected HTTP POST
- Port `validateAndResolveWebhookURL` and `buildSecureHTTPClient` from `plugin/webhook/webhook.go`
- HMAC signing: `signPayload(payload, secret)` using `crypto/hmac` + `crypto/sha256`
- Headers:
  - `X-Bchat-Signature: sha256=<hex>`
  - `X-Bchat-Event: <eventType>`
  - `Content-Type: application/json`

### 5.2 SMS Client (`server/router/api/v1/agent/sms.go` — new file)

~200 lines. Contains:
- `TwilioClient` struct (stateless HTTP helper)
- `SendSMS(ctx, tenantID, to, body)` method
- Phone validation via `github.com/nyaruka/phonenumbers`
- **Not called in v1** — prepared for v1.1

### 5.3 Event Dispatch (`server/router/api/v1/agent/service.go` — modifications)

Add to `Service` struct (line 49-63):
```go
// No cron scheduler — supercronic handles scheduling externally
```

New methods:
- `dispatchEvent(ctx, tenantID, eventType, data string) error` — inserts pre-claimed event (status='processing', claimed_at=now), spawns goroutine for immediate delivery
- `processEventPoller(ctx) error` — claims pending events via `ClaimPendingEvents`, delivers webhooks

Modify existing method:
- `captureLeadFromSession()` — after lead insert, call `dispatchEvent(ctx, tenantID, "lead.captured", payload)`

### 5.4 HTTP Trigger Endpoint (`server/router/api/v1/handlers.go` — modification)

Add handler:
```go
func (h *Handler) HandleTriggerCron(c echo.Context) error {
    token := c.Request().Header.Get("X-Cron-Token")
    if token == "" || token != os.Getenv("CRON_TOKEN") {
        return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
    }
    h.service.processEventPoller(c.Request().Context())
    return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
```

### 5.5 Route Registration (`server/router/api/v1/v1.go` — modification)

After line 182, add under `agentGroup`:
```go
agentGroup.GET("/:slug/integrations", h.HandleListIntegrations)
agentGroup.POST("/:slug/integrations", h.HandleCreateIntegration)
agentGroup.PUT("/:slug/integrations/:id", h.HandleUpdateIntegration)
agentGroup.DELETE("/:slug/integrations/:id", h.HandleDeleteIntegration)
agentGroup.POST("/:slug/integrations/:id/test", h.HandleTestIntegration)
agentGroup.GET("/:slug/events", h.HandleListEvents)
```

System route (separate group):
```go
systemGroup.POST("/system/trigger-cron", h.HandleTriggerCron)
```

### 5.6 Server Shutdown (`server/server.go` — modification)

No changes needed. Supercronic handles its own shutdown via SIGTERM. The entrypoint script manages process lifecycle.

---

## 6. Container Setup

### 6.1 Crontab File (`build/crontab` — new file)

```
# Poll webhook event outbox every 5 minutes
*/5 * * * * curl -sf -X POST http://localhost:${PORT:-5230}/api/v1/system/trigger-cron -H "X-Cron-Token: ${CRON_TOKEN}"
```

### 6.2 Entrypoint Script (`entrypoint.sh` — new file)

```bash
#!/bin/sh
set -e

PORT="${PORT:-5230}"
CRON_TOKEN="${CRON_TOKEN:?CRON_TOKEN environment variable is required}"

echo "Starting bchat server..."
./build/memos --mode prod --data /data &
BCHAT_PID=$!

echo "Starting supercronic..."
exec supercronic /etc/bchat/crontab &
SUPERCRONIC_PID=$!

# Wait for either process to exit
trap "kill $BCHAT_PID $SUPERCRONIC_PID 2>/dev/null; exit 0" SIGTERM SIGINT

wait $BCHAT_PID $SUPERCRONIC_PID
```

### 6.3 Dockerfile Modification (`Dockerfile.pg.fly` — modification)

Add after existing RUN commands:
```dockerfile
# Install supercronic
ARG SUPERCRONIC_URL=https://github.com/aptible/supercronic/releases/download/v0.2.33/supercronic-linux-amd64
ARG SUPERCRONIC_SHA1SUM=077b3e9779777b3b3b3b3b3b3b3b3b3b3b3b3b3b
RUN curl -fsSL "$SUPERCRONIC_URL" -o /usr/local/bin/supercronic \
    && echo "$SUPERCRONIC_SHA1SUM /usr/local/bin/supercronic" | sha1sum -c - \
    && chmod +x /usr/local/bin/supercronic

# Copy crontab
COPY build/crontab /etc/bchat/crontab

# Copy entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
```

### 6.4 Fly.io Configuration (`fly.toml` — modification)

Add to `[env]` section:
```toml
CRON_TOKEN = "your-secret-token-here"
```

Add to `[processes]`:
```toml
kill_timeout = 30
```

---

## 7. API Endpoints

### 7.1 Integration CRUD

| Method | Path | Handler | Permission |
|--------|------|---------|------------|
| `GET` | `/api/v1/agent/:slug/integrations` | `HandleListIntegrations` | `tenant:read` |
| `POST` | `/api/v1/agent/:slug/integrations` | `HandleCreateIntegration` | `tenant:admin` |
| `PUT` | `/api/v1/agent/:slug/integrations/:id` | `HandleUpdateIntegration` | `tenant:admin` |
| `DELETE` | `/api/v1/agent/:slug/integrations/:id` | `HandleDeleteIntegration` | `tenant:admin` |
| `POST` | `/api/v1/agent/:slug/integrations/:id/test` | `HandleTestIntegration` | `tenant:admin` |

### 7.2 Event Log

| Method | Path | Handler | Permission |
|--------|------|---------|------------|
| `GET` | `/api/v1/agent/:slug/events` | `HandleListEvents` | `tenant:read` |

Query params: `?status=delivered&limit=50&offset=0`

### 7.3 System Trigger

| Method | Path | Handler | Auth |
|--------|------|---------|------|
| `POST` | `/api/v1/system/trigger-cron` | `HandleTriggerCron` | `X-Cron-Token` header |

---

## 8. Frontend Admin UI

### 8.1 Store Updates (`web/src/store/v2/agentAdmin.ts`)

Add to `LocalState`:
```typescript
integrations: AgentIntegration[] = [];
events: AgentEvent[] = [];
isEditingIntegration: boolean = false;
editingIntegration: AgentIntegration | null = null;
```

Add methods:
```typescript
const fetchIntegrations = async (slug: string) => { ... };
const createIntegration = async (slug: string, data: CreateIntegrationRequest) => { ... };
const updateIntegration = async (slug: string, id: number, data: UpdateIntegrationRequest) => { ... };
const deleteIntegration = async (slug: string, id: number) => { ... };
const testIntegration = async (slug: string, id: number) => { ... };
const fetchEvents = async (slug: string, params?: EventQueryParams) => { ... };
```

### 8.2 New Component (`web/src/pages/AgentAdminSections/IntegrationsSection.tsx`)

~300 lines. Joy UI components + Tailwind:
- `Card` with `Typography` for section header
- `Button` to add integration
- Integration cards with toggle (`Switch`), edit, delete
- Event log viewer with status `Chip` (delivered=green, failed=red, pending=yellow)
- Test webhook button with loading state
- Permission gate: `isAdmin`

### 8.3 AgentAdmin.tsx Modification

Add import:
```typescript
import IntegrationsSection from './AgentAdminSections/IntegrationsSection';
```

Add after "Captured Leads" section:
```tsx
<IntegrationsSection
    slug={slug}
    isAdmin={isAdmin}
    integrations={agentAdminStore.integrations}
    events={agentAdminStore.events}
/>
```

---

## 9. Security Details

### 9.1 SSRF Protection (ported from `plugin/webhook/webhook.go`)

```go
func validateAndResolveWebhookURL(rawURL string) (string, error) {
    // 1. Validate scheme (http/https only)
    // 2. Resolve DNS
    // 3. Check resolved IPs against private ranges (10.x, 172.16-31.x, 192.168.x, 127.x, 169.254.x)
    // 4. Reject link-local and IPv6 loopback
}
```

### 9.2 HMAC Signing

```go
func signPayload(payload []byte, secret string) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
```

### 9.3 Idempotency Key

```go
func computeIdempotencyKey(tenantID int32, leadID string, eventType string) string {
    h := sha256.New()
    fmt.Fprintf(h, "%d:%s:%s", tenantID, leadID, eventType)
    return hex.EncodeToString(h.Sum(nil))
}
```

### 9.4 Cron Token

- 32-byte random hex string
- Set as `CRON_TOKEN` env var in `fly.toml`
- Validated in `HandleTriggerCron` via `X-Cron-Token` header

---

## 10. Testing

### 10.1 Unit Tests (7 cases)

| Test | File | Purpose |
|------|------|---------|
| `TestSignPayload` | `integrations_test.go` | HMAC correctness |
| `TestValidateWebhookURL` | `integrations_test.go` | SSRF blocks internal IPs |
| `TestIdempotentInsert_UniqueViolation` | `resilience_test.go` | `23505` treated as success |
| `TestStaleClaimReclaim` | `sqlite/agent_test.go` | 5-minute lease reclaim |
| `TestMaxAttemptsTerminal` | `sqlite/agent_test.go` | attempts >= 5 excluded |
| `TestImmediateDispatchNoDoubleClaim` | `service_test.go` | Pre-claimed events |
| `TestTriggerCronAuth` | `handlers_test.go` | Unauthenticated pings rejected |

### 10.2 Compile Gate

```bash
go build ./...  # All three drivers in-tree (SQLite, Postgres, MySQL stubs)
```

---

## 11. Implementation Order

| Phase | Files | Days |
|-------|-------|------|
| **1. Migrations** | 6 migration files (3 SQLite + 3 Postgres) | 1 |
| **2. Store layer** | `store/agent.go`, `store/driver.go`, `store/db/sqlite/agent.go`, `store/db/postgres/agent.go`, `store/db/postgres/resilience.go`, `store/db/mysql/agent.go` | 2 |
| **3. Service layer** | `agent/integrations.go`, `agent/sms.go`, `agent/service.go`, `v1/handlers.go`, `v1/v1.go` | 3 |
| **4. Container setup** | `build/crontab`, `entrypoint.sh`, `Dockerfile.pg.fly`, `fly.toml` | 1 |
| **5. Frontend UI** | `web/src/store/v2/agentAdmin.ts`, `IntegrationsSection.tsx`, `AgentAdmin.tsx` | 3 |
| **6. Testing** | Unit tests + compile gate | 2 |
| **Total** | **22 files** (12 new, 10 modified) | **12 days** |

---

## 12. File Change Summary

### New Files (12)
| File | Purpose |
|------|---------|
| `store/migration/sqlite/0.31/00__agent_integrations.sql` | SQLite integrations table |
| `store/migration/sqlite/0.31/01__agent_events.sql` | SQLite events table |
| `store/migration/sqlite/0.31/02__agent_sms_deferred.sql` | SQLite SMS placeholder |
| `store/migration/postgres/0.31/00__agent_integrations.sql` | Postgres integrations table |
| `store/migration/postgres/0.31/01__agent_events.sql` | Postgres events table |
| `store/migration/postgres/0.31/02__agent_sms_deferred.sql` | Postgres SMS placeholder |
| `store/db/postgres/resilience.go` | RunResiliently + isTransientError |
| `server/router/api/v1/agent/integrations.go` | Webhook dispatch + SSRF |
| `server/router/api/v1/agent/sms.go` | Twilio client (v1.1 prepared) |
| `build/crontab` | Supercronic schedule |
| `entrypoint.sh` | Process manager |
| `web/src/pages/AgentAdminSections/IntegrationsSection.tsx` | Frontend component |

### Modified Files (10)
| File | Changes |
|------|---------|
| `store/agent.go` | Add structs (append after line 928) |
| `store/driver.go` | Add interface methods (append after line 278) |
| `store/db/sqlite/agent.go` | Implement methods (append after line 1936) |
| `store/db/postgres/agent.go` | Implement methods (append after line 2477) |
| `store/db/mysql/agent.go` | Add stubs (append after line 884) |
| `server/router/api/v1/agent/service.go` | Add dispatchEvent + processEventPoller |
| `server/router/api/v1/agent/handlers.go` | Add CRUD + test endpoints |
| `server/router/api/v1/v1.go` | Register routes |
| `web/src/store/v2/agentAdmin.ts` | Add integration state |
| `web/src/pages/AgentAdmin.tsx` | Import IntegrationsSection |

---

## 13. Open Items

- [ ] Get actual SHA1 checksum for supercronic binary
- [ ] Generate CRON_TOKEN value for fly.toml
- [ ] Verify `Dockerfile.pg.fly` has correct base image
- [ ] Check if `plugin/webhook/webhook.go` functions are exported (may need to copy vs import)
- [ ] Verify `github.com/nyaruka/phonenumbers` is in go.mod (may need to add)
