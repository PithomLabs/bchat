# plan2.md — bchat Integrations (Revised)

> **Revised from:** `plan_biz.md` after adversarial review
> **Changes:** Addresses 5 critical + 6 major findings from review
> **Scope:** Phases 1-2 (Webhooks + SMS) only. Phase 3 (Calendar) deferred to separate plan.

---

## Table of Contents

1. [What Changed from plan_biz.md](#1-what-changed-from-plan_bizmd)
2. [Architecture Overview](#2-architecture-overview)
3. [Security Model](#3-security-model)
4. [Phase 1: Webhook Events](#4-phase-1-webhook-events)
5. [Phase 2: SMS Integration (Twilio)](#5-phase-2-sms-integration-twilio)
6. [Admin UI](#6-admin-ui)
7. [Data Model](#7-data-model)
8. [Implementation Timeline](#8-implementation-timeline)
9. [Testing Strategy](#9-testing-strategy)

---

## 1. What Changed from plan_biz.md

| Finding | Severity | Resolution |
|---------|----------|------------|
| **C1. Migration numbering wrong** | Critical | Use versioned dirs: `0.31/00__*.sql` for SQLite + Postgres |
| **C2. EventDispatcher DI path** | Critical | Methods on `*Service`, not separate struct |
| **C3. No retry design** | Critical | v1: best-effort with async goroutine + event log. Retry deferred to v2. |
| **C4. EncryptionService injection** | Critical | Already wired into `Service.encryptionService` — use existing getter |
| **C5. Twilio SDK dependency** | Critical | Thin HTTP wrapper (like `plugin/webhook/webhook.go`), no SDK |
| **M1. events missing integration_id** | Major | Added `integration_id` column to `agent_events` |
| **M2. OAuth state storage** | Major | Deferred (Calendar removed from scope) |
| **M3. AgentIntegration struct undefined** | Major | Full struct definition with JSON handling below |
| **M4. UI uses Tailwind, not Joy** | Major | Hybrid approach — Joy UI controls + Tailwind layout (matches existing pattern) |
| **M5. Calendar scope too large** | Major | Deferred to separate plan |
| **M6. processChat async dispatch** | Major | Fire-and-forget goroutine with context, like existing OM pattern |

---

## 2. Architecture Overview

### Scope (Phases 1-2 Only)

```
┌─────────────────────────────────────────────────────────────┐
│                      bchat Platform                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐         ┌──────────────┐                 │
│  │   Webhooks   │         │     SMS      │                 │
│  │   (Phase 1)  │         │   (Phase 2)  │                 │
│  └──────┬───────┘         └──────┬───────┘                 │
│         │                        │                          │
│         └────────────┬───────────┘                          │
│                      │                                      │
│             ┌────────▼────────┐                             │
│             │  Event Methods  │                             │
│             │  on *Service    │                             │
│             └────────┬────────┘                             │
│                      │                                      │
│             ┌────────▼────────┐                             │
│             │  Agent Service  │                             │
│             │  (service.go)   │                             │
│             └─────────────────┘                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
          │                        │
          ▼                        ▼
┌──────────────┐          ┌──────────────┐
│   Zapier     │          │    Twilio    │
│   n8n        │          │              │
│   Custom     │          │              │
│   Webhooks   │          │              │
└──────────────┘          └──────────────┘
```

### Event Flow

```
processChat() → State Transition → go s.dispatchEvent(...) → EventDispatcher
                                    (goroutine, fire-and-forget)
                                         │
                                         ├─ webhook delivery (SSRF-protected)
                                         └─ SMS scheduling (if configured)
```

---

## 3. Security Model

### 3.1 Credential Encryption

**Existing infrastructure:** `Service.encryptionService` (`*crypto.EncryptionService`) is already initialized in `NewService()` when `ENCRYPTION_MASTER_KEY` is set.

**Usage pattern (matches existing OpenRouter key decryption):**
```go
// Encrypt (in handler)
ciphertext, nonce, err := s.encryptionService.Encrypt(authToken)

// Decrypt (in service)
decrypted, err := s.encryptionService.Decrypt(
    integration.ConfigEncrypted,
    integration.ConfigNonce,
)
```

**No new DI wiring needed** — the `encryptionService` field already exists on `*Service` (line 55 of `service.go`).

### 3.2 SSRF Protection (Webhooks)

Reuse exact pattern from `plugin/webhook/webhook.go`:
- URL validation (scheme allowlist, internal IP blocking)
- DNS pinning (resolve once, dial validated IP)
- Redirect re-validation (max 3 redirects)

### 3.3 HMAC Signature

```go
func signPayload(payload []byte, secret string) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    return hex.EncodeToString(mac.Sum(nil))
}
// Header: X-Bchat-Signature: sha256=<hex-digest>
```

### 3.4 SMS Security

- Phone validation: `github.com/nyaruka/phonenumbers` (maintained Go port)
- TCPA compliance: opt-out ("STOP"), business identification, help ("HELP")
- Rate limiting: per-tenant 100 SMS/hour

---

## 4. Phase 1: Webhook Events

### 4.1 Event Types

| Event | Trigger | Payload |
|-------|---------|---------|
| `lead.captured` | After `captureLeadFromSession()` | Lead info + session |
| `lead.updated` | After `HandleUpdateLeadStatus` | Lead + status change |
| `escalation.created` | After ticket creation | Ticket + urgency |
| `conversation.completed` | Session ends | Summary + sentiment |

### 4.2 Implementation — Methods on `*Service`

No separate `EventDispatcher` struct. Event dispatch is a method set on `*Service`:

```go
// service.go — new methods

func (s *Service) dispatchEvent(ctx context.Context, tenantID int32, eventType string, data interface{}) {
    // Fire-and-forget goroutine (matches OM pattern at line 2376)
    go func() {
        defer func() {
            if r := recover(); r != nil {
                slog.Error("event dispatch panic", "error", r)
            }
        }()
        
        // 1. Marshal event
        event := Event{
            Type:      eventType,
            Timestamp: time.Now(),
            TenantID:  tenantID,
            Data:      data,
        }
        payload, _ := json.Marshal(event)
        
        // 2. Load active webhooks for tenant
        webhooks, err := s.store.ListIntegrationsByType(ctx, tenantID, "webhook")
        if err != nil {
            slog.Error("failed to load webhooks", "tenant_id", tenantID, "error", err)
            return
        }
        
        // 3. Deliver to each webhook
        for _, wh := range webhooks {
            if !wh.IsActive {
                continue
            }
            if err := s.deliverWebhook(ctx, wh, eventType, payload); err != nil {
                slog.Error("webhook delivery failed",
                    "tenant_id", tenantID,
                    "webhook_id", wh.ID,
                    "event", eventType,
                    "error", err)
                s.store.CreateEvent(ctx, &store.AgentEvent{
                    TenantID:      tenantID,
                    IntegrationID: wh.ID,
                    EventType:     eventType,
                    Payload:       string(payload),
                    Status:        "failed",
                    LastError:     err.Error(),
                })
                continue
            }
            s.store.CreateEvent(ctx, &store.AgentEvent{
                TenantID:      tenantID,
                IntegrationID: wh.ID,
                EventType:     eventType,
                Payload:       string(payload),
                Status:        "delivered",
                DeliveredAt:   time.Now(),
            })
        }
    }()
}

func (s *Service) deliverWebhook(ctx context.Context, wh *store.AgentIntegration, eventType string, payload []byte) error {
    // 1. Decrypt HMAC secret
    secret, err := s.encryptionService.Decrypt(wh.ConfigEncrypted, wh.ConfigNonce)
    if err != nil {
        return fmt.Errorf("failed to decrypt webhook secret: %w", err)
    }
    
    // 2. Sign payload
    signature := signPayload(payload, secret)
    
    // 3. Parse URL from config_plaintext
    var config WebhookConfig
    json.Unmarshal([]byte(wh.ConfigPlaintext), &config)
    
    // 4. Validate URL + IP pinning (SSRF protection)
    dialIP, err := validateWebhookURL(config.URL)
    if err != nil {
        return fmt.Errorf("webhook URL validation failed: %w", err)
    }
    
    // 5. Build secure HTTP client
    req, _ := http.NewRequestWithContext(ctx, "POST", config.URL, bytes.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Bchat-Signature", "sha256="+signature)
    req.Header.Set("X-Bchat-Event", eventType)
    req.Header.Set("X-Bchat-Tenant", fmt.Sprintf("%d", wh.TenantID))
    
    client := buildSecureHTTPClient(dialIP, 30*time.Second)
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("webhook request failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned status %d", resp.StatusCode)
    }
    
    return nil
}
```

### 4.3 Hook into `processChat()`

At the end of `processChat()`, after the response is built but before returning:

```go
// In processChat(), after line ~2160 (before return)
// Dispatch events asynchronously — don't block the chat response
if config.AudienceType == "external" {
    // Lead captured?
    if lead != nil && lead.Status == "new" {
        s.dispatchEvent(ctx, config.TenantID, "lead.captured", lead)
    }
    // Escalation created?
    if escalationTicket != nil {
        s.dispatchEvent(ctx, config.TenantID, "escalation.created", escalationTicket)
    }
}
```

**Latency impact:** Zero — goroutine is fire-and-forget, chat response returns immediately.

### 4.4 Retry Design (v1: Best-Effort)

**v1 decision:** Webhook delivery is best-effort. The `agent_events` table provides audit trail and debugging, but automatic retry is deferred to v2.

**Rationale:**
- Most webhook consumers (Zapier, n8n) have their own retry/backoff
- Adding retry infrastructure (polling goroutine, backoff, dead-letter) is significant complexity
- The event log table enables manual retry from admin UI (v2 feature)
- Existing OM pattern (line 2376) is fire-and-forget — consistent with codebase style

**v2 Enhancement (future):**
- Add `next_retry_at` column (already in schema)
- Add polling goroutine in `NewService()` that checks for failed events
- Exponential backoff: 1min, 5min, 30min, 2hr, 12hr (5 attempts)
- Max 5 attempts before permanent failure

### 4.5 API Endpoints

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/v1/agent/:slug/integrations` | `tenant:read` | List integrations |
| POST | `/api/v1/agent/:slug/integrations` | `tenant:admin` | Create integration |
| GET | `/api/v1/agent/:slug/integrations/:id` | `tenant:read` | Get integration |
| PUT | `/api/v1/agent/:slug/integrations/:id` | `tenant:admin` | Update integration |
| DELETE | `/api/v1/agent/:slug/integrations/:id` | `tenant:admin` | Delete integration |
| POST | `/api/v1/agent/:slug/integrations/:id/test` | `tenant:admin` | Send test event |
| GET | `/api/v1/agent/:slug/events` | `tenant:read` | List event log |

### 4.6 Database Migration

**Version:** `0.31/00__agent_integrations.sql`

```sql
-- store/migration/sqlite/0.31/00__agent_integrations.sql

CREATE TABLE IF NOT EXISTS agent_integrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('webhook', 'twilio')),
    name TEXT NOT NULL,
    config_encrypted BLOB,
    config_nonce BLOB,
    config_plaintext TEXT NOT NULL DEFAULT '{}',
    is_active INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
    last_used_at BIGINT,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_agent_integrations_tenant ON agent_integrations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_integrations_type ON agent_integrations(tenant_id, type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_integrations_unique ON agent_integrations(tenant_id, type, name);
```

**Version:** `0.31/01__agent_events.sql`

```sql
-- store/migration/sqlite/0.31/01__agent_events.sql

CREATE TABLE IF NOT EXISTS agent_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    integration_id INTEGER REFERENCES agent_integrations(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'delivered', 'failed', 'retrying')),
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    next_retry_at BIGINT,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    delivered_at BIGINT
);

CREATE INDEX IF NOT EXISTS idx_agent_events_tenant ON agent_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_events_status ON agent_events(status, next_retry_at);
CREATE INDEX IF NOT EXISTS idx_agent_events_integration ON agent_events(integration_id);
```

**Postgres migrations** in `store/migration/postgres/0.31/` with equivalent syntax.

---

## 5. Phase 2: SMS Integration (Twilio)

### 5.1 Twilio Approach: Thin HTTP Client

**No SDK dependency.** Follow the existing `plugin/webhook/webhook.go` pattern — direct HTTP calls to Twilio REST API.

```go
// server/router/api/v1/agent/sms.go

const twilioAPIBase = "https://api.twilio.com/2010-04-01"

type TwilioClient struct {
    httpClient *http.Client
}

func (c *TwilioClient) SendMessage(ctx context.Context, accountSID, authToken, from, to, body string) (*TwilioMessage, error) {
    url := fmt.Sprintf("%s/Accounts/%s/Messages.json", twilioAPIBase, accountSID)
    
    // Form-encoded body (Twilio API format)
    data := url.Values{}
    data.Set("From", from)
    data.Set("To", to)
    data.Set("Body", body)
    
    req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(data.Encode()))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.SetBasicAuth(accountSID, authToken)
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 400 {
        // Parse Twilio error response
        return nil, fmt.Errorf("twilio API error (status %d)", resp.StatusCode)
    }
    
    var msg TwilioMessage
    json.NewDecoder(resp.Body).Decode(&msg)
    return &msg, nil
}

type TwilioMessage struct {
    SID   string `json:"sid"`
    Status string `json:"status"`
    ErrorCode int `json:"error_code"`
}
```

### 5.2 SMS Service

```go
// server/router/api/v1/agent/sms.go

type SMSService struct {
    store    *store.Store
    crypto   *crypto.EncryptionService
    twilio   *TwilioClient
}

func (s *SMSService) Send(ctx context.Context, tenantID int32, to, body string) error {
    // 1. Validate phone
    if !phonenumbers.IsValidNumber(to) {
        return fmt.Errorf("invalid phone number")
    }
    
    // 2. Check opt-out
    if s.store.IsOptedOut(ctx, tenantID, to) {
        return fmt.Errorf("number opted out")
    }
    
    // 3. Load Twilio integration
    integration, err := s.store.GetIntegrationByType(ctx, tenantID, "twilio")
    if err != nil {
        return fmt.Errorf("Twilio not configured: %w", err)
    }
    
    // 4. Decrypt auth token
    authToken, err := s.crypto.Decrypt(integration.ConfigEncrypted, integration.ConfigNonce)
    if err != nil {
        return fmt.Errorf("failed to decrypt Twilio credentials: %w", err)
    }
    
    // 5. Parse config
    var config TwilioConfig
    json.Unmarshal([]byte(integration.ConfigPlaintext), &config)
    
    // 6. Send
    msg, err := s.twilio.SendMessage(ctx, config.AccountSID, authToken, config.FromNumber, to, body)
    if err != nil {
        return fmt.Errorf("Twilio send failed: %w", err)
    }
    
    // 7. Log
    s.store.CreateSMSMessage(ctx, &store.AgentSMSMessage{
        TenantID:    tenantID,
        IntegrationID: integration.ID,
        ToPhone:     to,
        Body:        body,
        TwilioSID:   msg.SID,
        Status:      "sent",
        SentAt:      time.Now(),
    })
    
    return nil
}
```

### 5.3 Cron Job for Delayed SMS

Uses existing `plugin/cron/cron.go` (vendored `robfig/cron`):

```go
// In service.go NewService()

// Start SMS cron for delayed messages
s.smsCron = cron.New()
s.smsCron.AddFunc("*/5 * * * *", func() {
    ctx := context.Background()
    pending, _ := s.store.ListPendingSMS(ctx)
    for _, sms := range pending {
        if time.Now().Before(sms.ScheduledAt) {
            continue
        }
        if err := s.smsService.Send(ctx, sms.TenantID, sms.ToPhone, sms.Body); err != nil {
            slog.Error("SMS send failed", "id", sms.ID, "error", err)
            s.store.UpdateSMSStatus(ctx, sms.ID, "failed", err.Error())
            continue
        }
        s.store.UpdateSMSStatus(ctx, sms.ID, "sent", "")
    }
})
s.smsCron.Start()
```

### 5.4 SMS Templates

Stored as JSON in `config_plaintext`:

```json
{
  "account_sid": "ACxxxxx",
  "from_number": "+1234567890",
  "templates": {
    "follow_up": {
      "body": "Hi {name}, thanks for reaching out to {company}. How can we help?",
      "delay_hours": 24
    },
    "review": {
      "body": "Thanks for chatting with us! Would you mind leaving a review?",
      "delay_hours": 24
    }
  }
}
```

### 5.5 TCPA Compliance

- Opt-out: reply "STOP" → `agent_sms_optouts` table
- Help: reply "HELP" → auto-response with business contact
- Business identification: first message includes company name

### 5.6 Database Migration

**Version:** `0.31/02__agent_sms.sql`

```sql
-- store/migration/sqlite/0.31/02__agent_sms.sql

CREATE TABLE IF NOT EXISTS agent_sms_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    integration_id INTEGER REFERENCES agent_integrations(id) ON DELETE SET NULL,
    to_phone TEXT NOT NULL,
    body TEXT NOT NULL,
    twilio_sid TEXT,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'sent', 'delivered', 'failed', 'cancelled')),
    error TEXT,
    scheduled_at BIGINT,
    sent_at BIGINT,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_agent_sms_tenant ON agent_sms_messages(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_sms_status ON agent_sms_messages(status, scheduled_at);

CREATE TABLE IF NOT EXISTS agent_sms_optouts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    phone TEXT NOT NULL,
    opted_out_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    UNIQUE(tenant_id, phone)
);

CREATE INDEX IF NOT EXISTS idx_agent_sms_optouts_tenant ON agent_sms_optouts(tenant_id);
```

---

## 6. Admin UI

### 6.1 Hybrid Approach (Joy UI + Tailwind)

Matches existing `AgentAdmin.tsx` pattern:
- **Joy UI** for controls: `Button`, `Modal`, `ModalDialog`, `Input`, `Select`, `Switch`, `Chip`
- **Tailwind** for layout: `className="bg-indigo-50 dark:bg-indigo-900/20 rounded-xl border ..."`
- **lucide-react** for icons

### 6.2 Section Placement

After "Captured Leads" section, before "Chat Transcripts". Permission gate: `isAdmin`.

### 6.3 Component Structure

```tsx
// New sub-component: IntegrationsSection.tsx

import { Button, Chip, Input, Modal, ModalDialog, Switch } from "@mui/joy";

const IntegrationsSection = ({ tenantSlug, isAdmin }) => {
  const [showPanel, setShowPanel] = useState(false);
  const [integrations, setIntegrations] = useState([]);
  
  if (!isAdmin) return null;
  
  return (
    <div className="bg-indigo-50 dark:bg-indigo-900/20 rounded-xl border border-indigo-200 dark:border-indigo-800 p-4">
      <div className="flex justify-between items-center mb-3">
        <div>
          <h3 className="font-medium text-indigo-700 dark:text-indigo-300">
            Integrations
          </h3>
          <p className="text-sm text-indigo-600 dark:text-indigo-400">
            Webhooks and SMS configuration
          </p>
        </div>
        <Button color="primary" variant="outlined" size="sm"
          onClick={() => setShowPanel(!showPanel)}>
          {showPanel ? "Close" : "Open"} ({integrations.length})
        </Button>
      </div>
      
      {showPanel && (
        <div className="mt-4 space-y-3">
          {/* Webhook cards */}
          {/* SMS card */}
          {/* Event log */}
        </div>
      )}
    </div>
  );
};
```

### 6.4 Integration Card (Joy UI + Tailwind)

```tsx
const IntegrationCard = ({ type, title, description, integrations, onAdd }) => (
  <div className="bg-white dark:bg-zinc-800 rounded-lg border border-gray-200 dark:border-zinc-700 p-4">
    <div className="flex justify-between items-start mb-3">
      <div>
        <h4 className="font-medium text-gray-900 dark:text-white">{title}</h4>
        <p className="text-sm text-gray-500 dark:text-gray-400">{description}</p>
      </div>
      <Button size="sm" variant="outlined" color="primary" onClick={onAdd}>
        + Add
      </Button>
    </div>
    
    {integrations.map(integration => (
      <div key={integration.id} 
        className="flex items-center justify-between p-3 bg-gray-50 dark:bg-zinc-700 rounded-lg mb-2">
        <div className="flex items-center gap-3">
          <div className={`w-2 h-2 rounded-full ${integration.is_active ? 'bg-green-500' : 'bg-gray-400'}`} />
          <span className="text-sm font-medium text-gray-900 dark:text-white">
            {integration.name}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <Switch size="sm" checked={integration.is_active}
            onChange={(e) => handleToggle(integration.id, e.target.checked)} />
          <Button size="sm" variant="plain" onClick={() => handleEdit(integration)}>
            Edit
          </Button>
        </div>
      </div>
    ))}
  </div>
);
```

---

## 7. Data Model

### 7.1 Go Structs

```go
// store/agent.go — new types

type AgentIntegration struct {
    ID              int32
    TenantID        int32
    Type            string  // "webhook", "twilio"
    Name            string
    ConfigEncrypted []byte  // AES-256-GCM encrypted JSON
    ConfigNonce     []byte  // GCM nonce
    ConfigPlaintext string  // JSON: non-sensitive config (URLs, phone numbers, templates)
    IsActive        bool
    LastUsedAt      *time.Time
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// ConfigPlaintext JSON structures per type:

type WebhookConfig struct {
    URL    string   `json:"url"`
    Events []string `json:"events"`
}

type TwilioConfig struct {
    AccountSID string            `json:"account_sid"`
    FromNumber string            `json:"from_number"`
    Templates  map[string]SMSTemplate `json:"templates"`
}

type SMSTemplate struct {
    Body       string `json:"body"`
    DelayHours int    `json:"delay_hours"`
    Enabled    bool   `json:"enabled"`
}

type AgentEvent struct {
    ID            int32
    TenantID      int32
    IntegrationID *int32
    EventType     string
    Payload       string
    Status        string
    Attempts      int
    LastError     string
    NextRetryAt   *time.Time
    CreatedAt     time.Time
    DeliveredAt   *time.Time
}

type AgentSMSMessage struct {
    ID            int32
    TenantID      int32
    IntegrationID *int32
    ToPhone       string
    Body          string
    TwilioSID     string
    Status        string
    Error         string
    ScheduledAt   *time.Time
    SentAt        *time.Time
    CreatedAt     time.Time
}

type AgentSMSOptOut struct {
    ID          int32
    TenantID    int32
    Phone       string
    OptedOutAt  time.Time
}
```

### 7.2 Store Interface Methods

```go
// store/driver.go — new interface methods

// Integrations
CreateIntegration(ctx context.Context, item *AgentIntegration) (*AgentIntegration, error)
GetIntegration(ctx context.Context, find *FindIntegration) (*AgentIntegration, error)
ListIntegrations(ctx context.Context, find *FindIntegration) ([]*AgentIntegration, error)
UpdateIntegration(ctx context.Context, item *AgentIntegration) (*AgentIntegration, error)
DeleteIntegration(ctx context.Context, id int32) error
GetIntegrationByType(ctx context.Context, tenantID int32, integrationType string) (*AgentIntegration, error)

// Events
CreateEvent(ctx context.Context, item *AgentEvent) (*AgentEvent, error)
ListEvents(ctx context.Context, find *FindEvent) ([]*AgentEvent, error)

// SMS
CreateSMSMessage(ctx context.Context, item *AgentSMSMessage) (*AgentSMSMessage, error)
ListPendingSMS(ctx context.Context) ([]*AgentSMSMessage, error)
UpdateSMSStatus(ctx context.Context, id int32, status, errMsg string) error
IsOptedOut(ctx context.Context, tenantID int32, phone string) bool
AddOptOut(ctx context.Context, tenantID int32, phone string) error

type FindIntegration struct {
    ID       *int32
    TenantID *int32
    Type     *string
}

type FindEvent struct {
    TenantID      *int32
    IntegrationID *int32
    EventType     *string
    Status        *string
    Limit         int
    Offset        int
}
```

---

## 8. Implementation Timeline

### Phase 1: Webhook Events (Weeks 1-2)

| Day | Task |
|-----|------|
| 1 | Create migration files (0.31/ for SQLite + Postgres) |
| 2 | Add Go structs + store interface methods |
| 3-4 | Implement SQLite store methods |
| 5-6 | Implement event dispatch methods on `*Service` |
| 7-8 | Add SSRF protection (port from `plugin/webhook/webhook.go`) |
| 9 | Add HMAC signing |
| 10 | Create admin API endpoints |
| 11-12 | Build webhook management UI (Joy UI + Tailwind) |
| 13 | Hook into `processChat()` |
| 14 | Testing |

### Phase 2: SMS Integration (Weeks 3-4)

| Day | Task |
|-----|------|
| 15-16 | Implement Twilio HTTP client |
| 17-18 | Implement SMSService |
| 19-20 | Add cron job for delayed SMS |
| 21-22 | TCPA compliance (opt-out, HELP) |
| 23-24 | SMS template engine |
| 25-26 | Build SMS management UI |
| 27-28 | Testing |

### Buffer (Week 5)

| Day | Task |
|-----|------|
| 29-30 | Bug fixes, edge cases |
| 31-32 | Cross-browser testing |
| 33-34 | Postgres migration testing |
| 35 | Documentation |

**Total: 5 weeks (25 working days)** — realistic for single developer with buffer.

---

## 9. Testing Strategy

### Unit Tests

| Test | What |
|------|------|
| `TestSignPayload` | HMAC signature correctness |
| `TestValidateWebhookURL` | SSRF blocks internal IPs |
| `TestValidateWebhookURL_Allowlist` | Valid URLs pass |
| `TestTwilioClient_SendMessage` | HTTP basic auth, form encoding |
| `TestSMSService_OptOut` | Opt-out prevents send |
| `TestSMSService_PhoneValidation` | Invalid numbers rejected |
| `TestEventDispatch_Goroutine` | Non-blocking dispatch |

### Integration Tests

| Scenario | Expected |
|----------|----------|
| Lead captured → webhook fires | Valid payload with HMAC signature |
| Lead captured → SMS queued | SMS in DB with status "queued" |
| Cron runs → SMS sent | Status updated to "sent" |
| Customer replies "STOP" | Opt-out recorded, pending SMS cancelled |
| Webhook to internal IP | Blocked, event logged as "failed" |

### Manual Verification

- [ ] Create webhook → send test event → verify receipt
- [ ] Configure Twilio → send test SMS → verify delivery
- [ ] Trigger lead capture → verify webhook + SMS scheduling
- [ ] Reply "STOP" → verify opt-out works
- [ ] Check event log shows delivery status

---

## 10. Files Changed Summary

### New Files

| File | Purpose |
|------|---------|
| `store/migration/sqlite/0.31/00__agent_integrations.sql` | Integrations table |
| `store/migration/sqlite/0.31/01__agent_events.sql` | Events table |
| `store/migration/sqlite/0.31/02__agent_sms.sql` | SMS tables |
| `store/migration/postgres/0.31/00__agent_integrations.sql` | Postgres migrations |
| `store/migration/postgres/0.31/01__agent_events.sql` | Postgres migrations |
| `store/migration/postgres/0.31/02__agent_sms.sql` | Postgres migrations |
| `server/router/api/v1/agent/sms.go` | Twilio HTTP client + SMS service |

### Modified Files

| File | Changes |
|------|---------|
| `store/agent.go` | Add AgentIntegration, AgentEvent, AgentSMSMessage structs |
| `store/driver.go` | Add integration, event, SMS interface methods |
| `store/db/sqlite/agent.go` | Implement new store methods |
| `store/db/postgres/agent.go` | Implement new store methods (stub initially) |
| `server/router/api/v1/agent/service.go` | Add dispatchEvent, deliverWebhook, smsService, smsCron |
| `server/router/api/v1/agent/handlers.go` | Add integration CRUD endpoints |
| `server/router/api/v1/v1.go` | Register integration routes |
| `web/src/pages/AgentAdmin.tsx` | Add IntegrationsSection |
| `web/src/store/v2/agentAdmin.ts` | Add integration state + methods |

---

*Plan version: 2.0*
*Created: 2026-07-10*
*Status: Ready for implementation*
