# plan5.md — Native Webhook Integrations v1

> Source: plan4.md (APPROVED FINAL) + plan4_review.md (amendments A-D) + plan4_imp.md + plan4_imp_review.md
> Scope: Webhooks only. Scheduled SMS deferred to v1.1.
> Cron: supercronic (external process, launched from existing entrypoint)
> Review: plan4_imp_review.md — 2 critical, 2 major, 3 moderate, 5 nits (all addressed below)

---

## 1. Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│  Fly.io Container                                        │
│                                                          │
│  scripts/entrypoint.sh (PID 1)                           │
│  ├─ file_env expansion (MEMOS_DSN, ENCRYPTION_MASTER_KEY,│
│  │   OPENROUTER_API_KEY, AWS_*, CRON_TOKEN)             │
│  ├─ fix volume ownership                                │
│  ├─ launch supercronic in background (if CRON_TOKEN set)│
│  └─ exec gosu memos ./memos --mode prod (non-root)      │
│                                                          │
│  supercronic (background)                                │
│  └─ every 5 min:                                         │
│     curl -X POST http://localhost:5230/                   │
│       api/v1/system/trigger-cron                          │
│       -H "X-Cron-Token: ${CRON_TOKEN}"                   │
│                                                          │
│  bchat server (non-root, via gosu)                       │
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

### v1 Contract (CR2 fix)

> **Immediate dispatch handles the live path.** The poller only catches
> failed/stale events while the machine is already awake. A fully-idle
> machine defers retries until it next wakes (via user traffic). This is
> defensible for v1 given lease reclaim + immediate dispatch. v1.1 may add
> an external pinger (Fly scheduled Machine or UptimeRobot) for guaranteed
> retry latency.

---

## 2. Scope Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scheduled SMS | **Defer to v1.1** | Webhooks alone deliver value; SMS adds ~2 days complexity |
| Cron mechanism | **supercronic in existing entrypoint** | Container-optimized, standard cron syntax, extends existing scripts/entrypoint.sh |
| Entrypoint | **Extend `scripts/entrypoint.sh`** | CR1 fix: preserve _FILE secret expansion + gosu privilege drop |
| Scale-to-zero | **Awake-only retries** | CR2 fix: in-container cron can't wake stopped machine; document honestly |
| Frontend split | **New component file** | Keeps AgentAdmin.tsx manageable |
| Testing | **Plain `testing` package** | No external test framework |
| MySQL stubs | **Add one-line stubs** | Amendment A — compile safety |
| Event claim | **Pre-claimed on insert** | Amendment B — eliminates failure window |
| Max attempts | **Column default 0** | Amendment C — safe by default |
| CRON_TOKEN | **fly secrets (runtime env)** | MJ2 fix: not committed to git |
| SSRF helpers | **Copy to agent package** | MO1 fix: unexported in plugin/webhook |
| sms.go | **Cut from v1** | MO3 fix: dead code, reintroduce in v1.1 |

---

## 3. Database Migrations

### 3.1 SQLite

**File: `store/migration/sqlite/0.31/00__agent_integrations.sql`**
```sql
CREATE TABLE IF NOT EXISTS agent_integrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    integration_type TEXT NOT NULL,          -- 'webhook' | 'twilio'
    label TEXT NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',       -- JSON: url, secret, headers (webhook)
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
-- NOTE: status DEFAULT 'processing' is intentional — every insert path pre-claims.
-- A brand-new row is always "in-flight" because the insertor dispatches immediately.
-- Non-dispatch insert paths should explicitly set status='pending'.
CREATE TABLE IF NOT EXISTS agent_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    integration_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,                -- 'lead.captured'
    payload TEXT NOT NULL DEFAULT '{}',      -- JSON payload
    status TEXT NOT NULL DEFAULT 'processing',
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

### 3.2 Postgres

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
-- NOTE: status DEFAULT 'processing' is intentional — every insert path pre-claims.
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

---

## 4. Store Layer

### 4.1 New Structs (`store/agent.go` — append at end of file)

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

### 4.2 Driver Interface (`store/driver.go` — append at end of file)

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

### 4.3 SQLite Implementation (`store/db/sqlite/agent.go` — append at end of file)

Key claim query (single-writer, no FOR UPDATE SKIP LOCKED):
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

### 4.4 Postgres Implementation (`store/db/postgres/agent.go` — append at end of file)

Key claim query (with FOR UPDATE SKIP LOCKED):
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
    // 57P01 (admin_shutdown), 08006 (connection_failure), 08003 (connection_does_not_exist)
}

func RunResiliently(fn func() error) error {
    // Exponential backoff: 1s, 2s, 4s, 8s, 16s + jitter
    // Treat 23505 (unique_violation) as success
}
```

### 4.6 MySQL Stubs (`store/db/mysql/agent.go` — append at end of file)

All new methods return `errNotImplemented`.

---

## 5. Agent Service Layer

### 5.1 Webhook Dispatch (`server/router/api/v1/agent/integrations.go` — new file)

~300 lines.

**SSRF helpers (copied from `plugin/webhook/webhook.go` — MO1 fix):**
```go
// These are unexported in plugin/webhook, so copy here.
func validateAndResolveWebhookURL(rawURL string) (string, error)
func isInternalIP(ip net.IP) bool
func buildSecureHTTPClient() *http.Client
```

**Webhook delivery:**
```go
func (s *Service) deliverWebhook(ctx context.Context, wh store.WebhookConfig, eventType string, payload []byte) error
```

- SSRF-protected HTTP POST via `validateAndResolveWebhookURL`
- HMAC signing: `signPayload(payload, secret)` using `crypto/hmac` + `crypto/sha256`
- Headers:
  - `X-Bchat-Signature: sha256=<hex>`
  - `X-Bchat-Event: <eventType>`
  - `Content-Type: application/json`
- Timeout: 10s context deadline

### 5.2 Event Dispatch (`server/router/api/v1/agent/service.go` — modifications)

**New methods:**
```go
// dispatchEvent inserts a pre-claimed event and spawns immediate delivery.
// On failure: leave row as 'processing' with original claimed_at (do NOT reset).
// The poller reclaims after the 300s lease. Only success flips to 'delivered'.
func (s *Service) dispatchEvent(ctx context.Context, tenantID int32, eventType string, data string) error

// processEventPoller claims pending events and delivers webhooks.
// Called by supercronic via trigger-cron endpoint.
func (s *Service) processEventPoller(ctx context.Context) error
```

**Modify existing:**
- `captureLeadFromSession()` — after lead insert, call `dispatchEvent(ctx, tenantID, "lead.captured", payload)`

### 5.3 HTTP Trigger Endpoint (`server/router/api/v1/agent/handlers.go` — modifications)

```go
func (h *Handler) HandleTriggerCron(c echo.Context) error {
    token := c.Request().Header.Get("X-Cron-Token")
    if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(os.Getenv("CRON_TOKEN"))) != 1 {
        return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
    }
    h.service.processEventPoller(c.Request().Context())
    return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
```

### 5.4 Route Registration (`server/router/api/v1/v1.go` — modifications)

Integration CRUD on `authGroup` (inherits tenant auth):
```go
authGroup.GET("/agent/:slug/integrations", s.agentHandler.HandleListIntegrations)
authGroup.POST("/agent/:slug/integrations", s.agentHandler.HandleCreateIntegration)
authGroup.PUT("/agent/:slug/integrations/:id", s.agentHandler.HandleUpdateIntegration)
authGroup.DELETE("/agent/:slug/integrations/:id", s.agentHandler.HandleDeleteIntegration)
authGroup.POST("/agent/:slug/integrations/:id/test", s.agentHandler.HandleTestIntegration)
authGroup.GET("/agent/:slug/events", s.agentHandler.HandleListEvents)
```

Bare system route (own auth via X-Cron-Token, outside tenant middleware):
```go
echoServer.POST("/api/v1/system/trigger-cron", s.agentHandler.HandleTriggerCron)
```

---

## 6. Container Setup

### 6.1 Crontab File (`build/crontab` — new file)

```
# Poll webhook event outbox every 5 minutes
*/5 * * * * curl -sf -X POST http://localhost:${PORT:-5230}/api/v1/system/trigger-cron -H "X-Cron-Token: ${CRON_TOKEN}"
```

### 6.2 Extend Existing Entrypoint (`scripts/entrypoint.sh` — modification)

**DO NOT create a new entrypoint.** Extend the existing one.

Add after file_env calls, before `exec gosu memos "$@"`:
```sh
# Launch supercronic in background if available and CRON_TOKEN is set
if command -v supercronic >/dev/null 2>&1 && [ -n "$CRON_TOKEN" ]; then
    supercronic /etc/bchat/crontab &
fi
```

**Keep existing:** `ENTRYPOINT ["./entrypoint.sh", "./memos"]` in Dockerfile.

### 6.3 Dockerfile Modification (`Dockerfile.pg.fly` — modification)

Add after existing RUN commands (before ENTRYPOINT):
```dockerfile
# Install supercronic
ARG SUPERCRONIC_URL=https://github.com/aptible/supercronic/releases/download/v0.2.33/supercronic-linux-amd64
ARG SUPERCRONIC_SHA1SUM=<get-actual-sha1sum>
RUN curl -fsSL "$SUPERCRONIC_URL" -o /usr/local/bin/supercronic \
    && echo "$SUPERCRONIC_SHA1SUM /usr/local/bin/supercronic" | sha1sum -c - \
    && chmod +x /usr/local/bin/supercronic

# Copy crontab
COPY build/crontab /etc/bchat/crontab
```

### 6.4 Fly.io Configuration (`fly.toml` — modification)

Add top-level (NOT under [processes]):
```toml
kill_timeout = "30s"
```

**CRON_TOKEN via fly secrets (NOT in [env]):**
```bash
fly secrets set CRON_TOKEN=$(openssl rand -hex 32)
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

## 8. Security Details

### 8.1 SSRF Protection (copied from plugin/webhook/webhook.go)

```go
func validateAndResolveWebhookURL(rawURL string) (string, error) {
    // 1. Validate scheme (http/https only)
    // 2. Resolve DNS
    // 3. Check resolved IPs against private ranges (10.x, 172.16-31.x, 192.168.x, 127.x, 169.254.x)
    // 4. Reject link-local and IPv6 loopback
}
```

### 8.2 HMAC Signing

```go
func signPayload(payload []byte, secret string) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
```

### 8.3 Idempotency Key

```go
func computeIdempotencyKey(tenantID int32, leadID string, eventType string) string {
    h := sha256.New()
    fmt.Fprintf(h, "%d:%s:%s", tenantID, leadID, eventType)
    return hex.EncodeToString(h.Sum(nil))
}
```

### 8.4 Cron Token

- 32-byte random hex string: `openssl rand -hex 32`
- Set via `fly secrets set CRON_TOKEN=...` (runtime env, NOT in fly.toml)
- Validated via constant-time compare: `subtle.ConstantTimeCompare`

---

## 9. Frontend Admin UI

### 9.1 Store Updates (`web/src/store/v2/agentAdmin.ts`)

Add to LocalState:
```typescript
integrations: AgentIntegration[] = [];
events: AgentEvent[] = [];
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

### 9.2 New Component (`web/src/pages/AgentAdminSections/IntegrationsSection.tsx`)

~300 lines. Joy UI + Tailwind:
- Card with Typography for section header
- Button to add integration
- Integration cards with toggle (Switch), edit, delete
- Event log viewer with status Chip (delivered=green, failed=red, pending=yellow)
- Test webhook button with loading state
- Permission gate: isAdmin

### 9.3 AgentAdmin.tsx Modification

```typescript
import IntegrationsSection from './AgentAdminSections/IntegrationsSection';
```

Add after Captured Leads section:
```tsx
<IntegrationsSection
    slug={slug}
    isAdmin={isAdmin}
    integrations={agentAdminStore.integrations}
    events={agentAdminStore.events}
/>
```

---

## 10. Testing

### 10.1 Unit Tests (plain `testing` package)

| Test | File | Purpose |
|------|------|---------|
| `TestSignPayload` | `integrations_test.go` | HMAC correctness |
| `TestValidateWebhookURL` | `integrations_test.go` | SSRF blocks internal IPs |
| `TestIdempotentInsert_UniqueViolation` | `resilience_test.go` | 23505 treated as success |
| `TestStaleClaimReclaim` | `sqlite/agent_test.go` | 5-minute lease reclaim |
| `TestMaxAttemptsTerminal` | `sqlite/agent_test.go` | attempts >= 5 excluded |
| `TestImmediateDispatchNoDoubleClaim` | `service_test.go` | Pre-claimed events |
| `TestTriggerCronAuth` | `handlers_test.go` | Unauthenticated pings rejected |

### 10.2 Compile Gate

```bash
go build ./...  # All three drivers in-tree
```

---

## 11. Implementation Order

| Phase | Files | Estimated |
|-------|-------|-----------|
| **1. Migrations** | 4 migration files (2 SQLite + 2 Postgres) | Day 1 |
| **2. Store layer** | store/agent.go, store/driver.go, sqlite/agent.go, postgres/agent.go, postgres/resilience.go, mysql/agent.go | Days 2-3 |
| **3. Service layer** | agent/integrations.go, agent/service.go, agent/handlers.go, v1/v1.go | Days 4-6 |
| **4. Container setup** | build/crontab, scripts/entrypoint.sh, Dockerfile.pg.fly, fly.toml | Day 7 |
| **5. Frontend UI** | agentAdmin.ts, IntegrationsSection.tsx, AgentAdmin.tsx | Days 8-10 |
| **6. Testing** | Unit tests + compile gate | Days 11-12 |
| **Total** | **21 files** (8 new, 13 modified) | **12 days** |

---

## 12. File Change Summary

### New Files (8)
| File | Purpose |
|------|---------|
| `store/migration/sqlite/0.31/00__agent_integrations.sql` | SQLite integrations table |
| `store/migration/sqlite/0.31/01__agent_events.sql` | SQLite events table |
| `store/migration/postgres/0.31/00__agent_integrations.sql` | Postgres integrations table |
| `store/migration/postgres/0.31/01__agent_events.sql` | Postgres events table |
| `store/db/postgres/resilience.go` | RunResiliently + isTransientError |
| `server/router/api/v1/agent/integrations.go` | Webhook dispatch + SSRF (copied helpers) |
| `build/crontab` | Supercronic schedule |
| `web/src/pages/AgentAdminSections/IntegrationsSection.tsx` | Frontend component |

### Modified Files (13)
| File | Changes |
|------|---------|
| `store/agent.go` | Add AgentIntegration, AgentEvent, WebhookConfig, Find structs |
| `store/driver.go` | Add integration + event interface methods |
| `store/db/sqlite/agent.go` | Implement integration + event methods |
| `store/db/postgres/agent.go` | Implement integration + event methods |
| `store/db/mysql/agent.go` | Add stubs returning errNotImplemented |
| `server/router/api/v1/agent/service.go` | Add dispatchEvent + processEventPoller |
| `server/router/api/v1/agent/handlers.go` | Add CRUD + test + trigger-cron endpoints |
| `server/router/api/v1/v1.go` | Register routes on authGroup + bare system route |
| `web/src/store/v2/agentAdmin.ts` | Add integration state + methods |
| `web/src/pages/AgentAdmin.tsx` | Import IntegrationsSection |
| `scripts/entrypoint.sh` | Add supercronic launch before exec gosu |
| `Dockerfile.pg.fly` | Install supercronic + copy crontab |
| `fly.toml` | Add kill_timeout (top-level) |

---

## 13. Open Items

- [ ] Get actual SHA1 checksum for supercronic binary: `curl -fsSL <url> | sha1sum`
- [ ] Generate CRON_TOKEN: `fly secrets set CRON_TOKEN=$(openssl rand -hex 32)`
- [ ] Verify `scripts/entrypoint.sh` has correct structure (read before modifying)
- [ ] Check if `github.com/nyaruka/phonenumbers` is in go.mod (not needed for v1 — MO3)
- [ ] Verify `plugin/webhook/webhook.go` SSRF helpers are correctly copied
