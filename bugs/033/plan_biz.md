# plan_biz.md — bchat Integrations: Webhooks + SMS + Calendar

> **Goal:** Transform bchat into a complete "digital front desk" that replaces Intercom AI ($97/mo), Helios AI ($100/mo), and parts of n8n ($49/mo) — saving $246/mo per deployment while delivering a frictionless, superb user experience.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Security Model](#2-security-model)
3. [Phase 1: Webhook Events](#3-phase-1-webhook-events)
4. [Phase 2: SMS Integration (Twilio)](#4-phase-2-sms-integration-twilio)
5. [Phase 3: Calendar Integration](#5-phase-3-calendar-integration)
6. [Phase 4: Unified Admin UI](#6-phase-4-unified-admin-ui)
7. [Data Model Summary](#7-data-model-summary)
8. [Implementation Timeline](#8-implementation-timeline)
9. [Testing Strategy](#9-testing-strategy)
10. [Rollout Plan](#10-rollout-plan)

---

## 1. Architecture Overview

### System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        bchat Platform                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   Webhooks   │    │     SMS      │    │   Calendar   │      │
│  │   (Phase 1)  │    │   (Phase 2)  │    │   (Phase 3)  │      │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘      │
│         │                   │                   │               │
│         └───────────────────┼───────────────────┘               │
│                             │                                   │
│                    ┌────────▼────────┐                          │
│                    │  Event Engine   │                          │
│                    │  (events.go)    │                          │
│                    └────────┬────────┘                          │
│                             │                                   │
│                    ┌────────▼────────┐                          │
│                    │  Agent Service  │                          │
│                    │  (service.go)   │                          │
│                    └────────┬────────┘                          │
│                             │                                   │
│                    ┌────────▼────────┐                          │
│                    │  Integration    │                          │
│                    │  Store          │                          │
│                    │  (agent.go)     │                          │
│                    └─────────────────┘                          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
         │                   │                   │
         ▼                   ▼                   ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Zapier     │    │    Twilio    │    │   Google     │
│   n8n        │    │              │    │   Outlook    │
│   Custom     │    │              │    │   Calendar   │
│   Webhooks   │    │              │    │              │
└──────────────┘    └──────────────┘    └──────────────┘
```

### Event Flow

```
User Message → processChat() → State Transition → EventDispatcher → Integration Handler
                                    │
                                    ├─ lead.captured → Webhook + SMS Follow-up
                                    ├─ escalation.created → Webhook + SMS Alert
                                    ├─ conversation.completed → Webhook + Review Request
                                    └─ safety.triggered → Webhook + SMS Emergency
```

### Integration Priority

| Priority | Integration | Replaces | Cost Savings |
|----------|-------------|----------|--------------|
| P0 | Webhooks | Parts of n8n | $49/mo |
| P1 | SMS (Twilio) | Helios AI | $100/mo |
| P2 | Calendar | Intercom AI booking | $97/mo |

---

## 2. Security Model

### 2.1 Credential Encryption

**Algorithm:** AES-256-GCM with Argon2id key derivation (existing pattern from `internal/crypto/encryption.go`).

**Storage Pattern:**
```sql
-- Integration credentials stored as encrypted blobs
CREATE TABLE agent_integrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('webhook', 'twilio', 'google_calendar', 'outlook')),
    name TEXT NOT NULL,
    config_encrypted BLOB,      -- AES-256-GCM encrypted JSON
    config_nonce BLOB,          -- GCM nonce for decryption
    config_plaintext TEXT,      -- Non-sensitive config (URLs, names, etc.)
    is_active INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);
```

**Sensitive Fields by Integration Type:**

| Type | Encrypted Fields | Plaintext Fields |
|------|------------------|------------------|
| `webhook` | `secret` (HMAC key) | `url`, `events` |
| `twilio` | `auth_token` | `account_sid`, `from_number`, `templates` |
| `google_calendar` | `oauth_token`, `refresh_token` | `calendar_id`, `working_hours` |
| `outlook` | `oauth_token`, `refresh_token` | `calendar_id`, `working_hours` |

**Encryption Flow:**
```
Admin UI → POST /integrations → Handler encrypts → Store encrypted blobs
                                                    ↓
Agent Service → Load integration → Decrypt credentials → Use in API calls
```

### 2.2 SSRF Protection (Webhooks)

**Three-Layer Defense** (reusing `plugin/webhook/webhook.go` pattern):

**Layer 1: URL Validation**
```go
func validateWebhookURL(rawURL string) (string, error) {
    // 1. Scheme allowlist: http/https only
    // 2. Block internal IPs (127.x, 10.x, 192.168.x, 169.254.x)
    // 3. Block cloud metadata endpoints (169.254.169.254)
    // 4. DNS resolution + IP validation
    // 5. Return resolved IP for pinning
}
```

**Layer 2: IP Pinning**
```go
transport := &http.Transport{
    DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        // Force connection to validated IP, not re-resolved DNS
        _, port, _ := net.SplitHostPort(addr)
        targetAddr := net.JoinHostPort(dialIP, port)
        return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, targetAddr)
    },
}
```

**Layer 3: Redirect Re-validation**
```go
client := &http.Client{
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        if len(via) >= 3 {
            return errors.New("too many redirects")
        }
        // Re-validate redirect target
        _, err := validateWebhookURL(req.URL.String())
        return err
    },
}
```

### 2.3 HMAC Signature Verification

Every outbound webhook includes an HMAC-SHA256 signature:

```go
func signPayload(payload []byte, secret string) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    return hex.EncodeToString(mac.Sum(nil))
}

// Header: X-Bchat-Signature: sha256=<hex-digest>
```

### 2.4 SMS Security (Twilio)

**Input Validation:**
- Phone number validation via `libphonenumber` library
- Message template sanitization (prevent injection)
- Opt-out handling (TCPA compliance: "STOP" to unsubscribe)

**Rate Limiting:**
- Per-tenant SMS rate limit: 100 messages/hour (configurable)
- Global rate limit: 1000 messages/hour (Twilio account limit)

**Audit Logging:**
- All SMS messages logged to `agent_sms_messages` table
- Include: recipient, message, status, timestamps, cost

### 2.5 Calendar Security (Google/Outlook)

**OAuth2 Flow:**
```
1. Admin clicks "Authorize Calendar"
2. Redirect to Google/Microsoft OAuth consent screen
3. Callback with auth code
4. Exchange for access_token + refresh_token
5. Encrypt tokens, store in agent_integrations
6. Use refresh_token for long-lived access
```

**Token Refresh:**
```go
func (s *CalendarService) refreshToken(integration *store.AgentIntegration) error {
    // 1. Decrypt refresh_token
    // 2. Call OAuth provider token endpoint
    // 3. Encrypt new access_token
    // 4. Update database
    // 5. Return new access_token
}
```

**Scope Limitation:**
- Google: `https://www.googleapis.com/auth/calendar.events` (events only, not full calendar)
- Outlook: `Calendars.ReadWrite` (events only)

### 2.6 Webhook Secret Management

**Per-Integration Secrets:**
```go
// Generate HMAC secret on webhook creation
func generateWebhookSecret() string {
    b := make([]byte, 32)
    rand.Read(b)
    return hex.EncodeToString(b)
}
```

**Secret Rotation:**
- Admin can rotate webhook secret from UI
- Old secret valid for 24 hours after rotation (grace period)
- Signature includes timestamp: `X-Bchat-Signature: t=<timestamp>,sha256=<digest>`

---

## 3. Phase 1: Webhook Events

### 3.1 Event Types

| Event | Trigger | Payload |
|-------|---------|---------|
| `lead.captured` | After `captureLeadFromSession()` | `{lead, session, tenant}` |
| `lead.updated` | After `HandleUpdateLeadStatus` | `{lead, old_status, new_status}` |
| `escalation.created` | After ticket creation | `{ticket, session, urgency}` |
| `conversation.completed` | Session ends | `{session, summary, sentiment}` |
| `safety.triggered` | Emergency detected | `{session, protocol, customer_info}` |

### 3.2 Event Payload Schema

```json
{
  "event": "lead.captured",
  "timestamp": "2026-01-15T10:30:00Z",
  "tenant_id": "inc",
  "data": {
    "lead": {
      "id": "uuid",
      "name": "John Doe",
      "email": "john@example.com",
      "phone": "+1234567890",
      "topic": "Water damage repair",
      "status": "new"
    },
    "session": {
      "id": "session_uuid",
      "started_at": "2026-01-15T10:25:00Z",
      "message_count": 5
    }
  }
}
```

### 3.3 Implementation

**New Files:**

| File | Purpose |
|------|---------|
| `server/router/api/v1/agent/events.go` | Event dispatcher + webhook delivery |
| `server/router/api/v1/agent/events_test.go` | Unit tests |
| `store/migration/sqlite/30__agent_integrations.sql` | Database migration |

**Modified Files:**

| File | Changes |
|------|---------|
| `store/agent.go` | Add `AgentIntegration` struct + store methods |
| `store/driver.go` | Add integration CRUD interface methods |
| `server/router/api/v1/agent/service.go` | Hook event dispatcher into `processChat()` |
| `server/router/api/v1/agent/handlers.go` | Add integration CRUD endpoints |
| `server/router/api/v1/v1.go` | Register integration routes |

**Event Dispatcher (`events.go`):**

```go
type EventDispatcher struct {
    store  *store.Store
    crypto *crypto.EncryptionService
}

type Event struct {
    Type      string      `json:"event"`
    Timestamp time.Time   `json:"timestamp"`
    TenantID  string      `json:"tenant_id"`
    Data      interface{} `json:"data"`
}

func (d *EventDispatcher) Dispatch(ctx context.Context, tenantID int32, eventType string, data interface{}) error {
    // 1. Load active webhooks for tenant
    // 2. For each webhook:
    //    a. Check if event type is subscribed
    //    b. Marshal payload
    //    c. Sign with HMAC
    //    d. POST with SSRF protection
    //    e. Log delivery status
    // 3. Return errors (non-blocking: log failures, don't fail chat)
}
```

**Webhook Delivery (`events.go`):**

```go
func (d *EventDispatcher) deliverWebhook(ctx context.Context, webhook *store.AgentIntegration, event *Event) error {
    // 1. Marshal event to JSON
    payload, _ := json.Marshal(event)
    
    // 2. Sign payload
    secret := decryptSecret(webhook.ConfigEncrypted, webhook.ConfigNonce)
    signature := signPayload(payload, secret)
    
    // 3. Create request
    req, _ := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Bchat-Signature", "sha256="+signature)
    req.Header.Set("X-Bchat-Event", event.Type)
    req.Header.Set("X-Bchat-Tenant", event.TenantID)
    
    // 4. Validate URL + IP pinning (SSRF protection)
    dialIP, err := validateWebhookURL(webhook.URL)
    if err != nil {
        return fmt.Errorf("webhook URL validation failed: %w", err)
    }
    
    // 5. Send with timeout + redirect validation
    client := buildSecureHTTPClient(dialIP, 30*time.Second)
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("webhook delivery failed: %w", err)
    }
    defer resp.Body.Close()
    
    // 6. Check response
    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned status %d", resp.StatusCode)
    }
    
    return nil
}
```

### 3.4 API Endpoints

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/agent/:slug/integrations` | `HandleListIntegrations` | List all integrations |
| POST | `/api/v1/agent/:slug/integrations` | `HandleCreateIntegration` | Create integration |
| GET | `/api/v1/agent/:slug/integrations/:id` | `HandleGetIntegration` | Get integration details |
| PUT | `/api/v1/agent/:slug/integrations/:id` | `HandleUpdateIntegration` | Update integration |
| DELETE | `/api/v1/agent/:slug/integrations/:id` | `HandleDeleteIntegration` | Delete integration |
| POST | `/api/v1/agent/:slug/integrations/:id/test` | `HandleTestIntegration` | Send test event |
| POST | `/api/v1/agent/:slug/integrations/:id/rotate-secret` | `HandleRotateSecret` | Rotate webhook secret |

### 3.5 Database Migration

```sql
-- migration 30: AGENT INTEGRATIONS

CREATE TABLE IF NOT EXISTS agent_integrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('webhook', 'twilio', 'google_calendar', 'outlook')),
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

-- Event log for audit and retry
CREATE TABLE IF NOT EXISTS agent_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
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
```

---

## 4. Phase 2: SMS Integration (Twilio)

### 4.1 Features

| Feature | Trigger | Delay | Message |
|---------|---------|-------|---------|
| **Lead Follow-up** | `lead.captured` | 24h | "Hi {name}, thanks for reaching out to {company}. How can we help?" |
| **Appointment Reminder** | Calendar booking | 24h before | "Reminder: You have an appointment with {company} tomorrow at {time}." |
| **Review Request** | `conversation.completed` (positive) | 24h | "Thanks for chatting with us! Would you mind leaving a review?" |
| **Missed Call Text-back** | Future telephony | Instant | "Sorry we missed your call. How can we help?" |

### 4.2 Implementation

**New Files:**

| File | Purpose |
|------|---------|
| `server/router/api/v1/agent/sms.go` | Twilio SMS service |
| `server/router/api/v1/agent/sms_templates.go` | Message template engine |
| `server/router/api/v1/agent/sms_test.go` | Unit tests |

**Modified Files:**

| File | Changes |
|------|---------|
| `store/agent.go` | Add `AgentSMSMessage` struct |
| `store/driver.go` | Add SMS store methods |
| `server/router/api/v1/agent/service.go` | Add SMS hooks after lead capture |
| `plugin/cron/cron.go` | Add cron job for delayed SMS |

**SMS Service (`sms.go`):**

```go
type SMSService struct {
    store  *store.Store
    crypto *crypto.EncryptionService
}

type SMSMessage struct {
    ID          string
    TenantID    int32
    To          string
    Body        string
    Status      string // "queued", "sent", "delivered", "failed"
    ScheduledAt time.Time
    SentAt      *time.Time
    Error       string
    Cost        float64
}

func (s *SMSService) Send(ctx context.Context, tenantID int32, to, body string) error {
    // 1. Validate phone number
    if !validatePhoneNumber(to) {
        return fmt.Errorf("invalid phone number: %s", to)
    }
    
    // 2. Check opt-out list
    if s.isOptedOut(ctx, tenantID, to) {
        return fmt.Errorf("number opted out of SMS")
    }
    
    // 3. Load Twilio credentials
    integration, err := s.store.GetIntegrationByType(ctx, tenantID, "twilio")
    if err != nil {
        return fmt.Errorf("Twilio not configured: %w", err)
    }
    
    // 4. Decrypt auth token
    authToken, err := s.crypto.Decrypt(integration.ConfigEncrypted, integration.ConfigNonce)
    if err != nil {
        return fmt.Errorf("failed to decrypt Twilio credentials: %w", err)
    }
    
    // 5. Send via Twilio API
    msg, err := s.sendTwilio(integration.ConfigPlaintext["account_sid"], authToken, integration.ConfigPlaintext["from_number"], to, body)
    if err != nil {
        return fmt.Errorf("Twilio API error: %w", err)
    }
    
    // 6. Log message
    s.store.CreateSMSMessage(ctx, &store.AgentSMSMessage{
        TenantID:    tenantID,
        To:          to,
        Body:        body,
        TwilioSID:   msg.SID,
        Status:      "sent",
        SentAt:      time.Now(),
    })
    
    return nil
}

func (s *SMSService) ScheduleFollowUp(ctx context.Context, tenantID int32, lead *store.AgentLead, delayHours int) error {
    // 1. Load template
    template := s.getTemplate(tenantID, "follow_up")
    
    // 2. Render template with lead data
    body := renderTemplate(template, map[string]string{
        "name":    lead.Name,
        "company": getCompanyName(tenantID),
    })
    
    // 3. Schedule for delivery
    scheduledAt := time.Now().Add(time.Duration(delayHours) * time.Hour)
    
    s.store.CreateSMSMessage(ctx, &store.AgentSMSMessage{
        TenantID:    tenantID,
        To:          lead.Phone,
        Body:        body,
        Status:      "queued",
        ScheduledAt: scheduledAt,
    })
    
    return nil
}
```

### 4.3 SMS Templates

**Template Storage (in `agent_integrations.config_plaintext`):**

```json
{
  "templates": {
    "follow_up": {
      "body": "Hi {name}, thanks for reaching out to {company}. How can we help?",
      "enabled": true,
      "delay_hours": 24
    },
    "reminder": {
      "body": "Reminder: You have an appointment with {company} tomorrow at {time}.",
      "enabled": true,
      "delay_hours_before": 24
    },
    "review": {
      "body": "Thanks for chatting with us! Would you mind leaving a review? {review_link}",
      "enabled": true,
      "delay_hours": 24,
      "require_positive_sentiment": true
    }
  }
}
```

### 4.4 Cron Job for Delayed SMS

```go
// In service.go initialization
func (s *Service) startSMSCron() {
    c := cron.New()
    
    // Check for pending SMS every 5 minutes
    c.AddFunc("*/5 * * * *", func() {
        ctx := context.Background()
        pending, _ := s.store.ListPendingSMS(ctx)
        for _, sms := range pending {
            if time.Now().Before(sms.ScheduledAt) {
                continue // Not yet time
            }
            if err := s.smsService.Send(ctx, sms.TenantID, sms.To, sms.Body); err != nil {
                slog.Error("SMS send failed", "id", sms.ID, "error", err)
                s.store.UpdateSMSStatus(ctx, sms.ID, "failed", err.Error())
                continue
            }
            s.store.UpdateSMSStatus(ctx, sms.ID, "sent", "")
        }
    })
    
    c.Start()
}
```

### 4.5 TCPA Compliance

**Opt-Out Handling:**
```go
func (s *SMSService) HandleOptOut(ctx context.Context, tenantID int32, phone string) error {
    // 1. Add to opt-out list
    s.store.AddOptOut(ctx, tenantID, phone)
    
    // 2. Cancel any pending SMS for this number
    s.store.CancelPendingSMS(ctx, tenantID, phone)
    
    // 3. Send confirmation
    s.sendTwilio(..., "You have been unsubscribed from SMS notifications.")
    
    return nil
}
```

**Required Elements:**
- Opt-out mechanism (reply "STOP" to unsubscribe)
- Business identification in first message
- Message frequency disclosure
- Help instructions (reply "HELP")

### 4.6 Database Migration

```sql
-- migration 31: AGENT SMS MESSAGES

CREATE TABLE IF NOT EXISTS agent_sms_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    integration_id INTEGER REFERENCES agent_integrations(id) ON DELETE SET NULL,
    to_phone TEXT NOT NULL,
    body TEXT NOT NULL,
    twilio_sid TEXT,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'sent', 'delivered', 'failed', 'cancelled')),
    error TEXT,
    cost_cents INTEGER DEFAULT 0,
    scheduled_at BIGINT,
    sent_at BIGINT,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_agent_sms_tenant ON agent_sms_messages(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_sms_status ON agent_sms_messages(status, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_agent_sms_phone ON agent_sms_messages(tenant_id, to_phone);

-- Opt-out list
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

## 5. Phase 3: Calendar Integration

### 5.1 Supported Providers

| Provider | OAuth Scopes | API |
|----------|--------------|-----|
| Google Calendar | `calendar.events` | REST API v3 |
| Outlook | `Calendars.ReadWrite` | Microsoft Graph API |

### 5.2 Features

| Feature | Description |
|---------|-------------|
| **Check Availability** | Query calendar for open slots within working hours |
| **Book Appointment** | Create calendar event with customer info |
| **Cancel/Reschedule** | Modify existing appointments |
| **Working Hours** | Respect business hours configuration |
| **Buffer Time** | Add buffer between appointments |
| **Double-Booking Prevention** | Check before booking |

### 5.3 Implementation

**New Files:**

| File | Purpose |
|------|---------|
| `server/router/api/v1/agent/calendar.go` | Calendar service (Google + Outlook) |
| `server/router/api/v1/agent/calendar_google.go` | Google Calendar adapter |
| `server/router/api/v1/agent/calendar_outlook.go` | Outlook Calendar adapter |
| `server/router/api/v1/agent/calendar_test.go` | Unit tests |

**Calendar Interface:**

```go
type CalendarProvider interface {
    GetAvailability(ctx context.Context, date time.Time, duration time.Duration) ([]TimeSlot, error)
    BookAppointment(ctx context.Context, slot TimeSlot, customer CustomerInfo) (*Appointment, error)
    CancelAppointment(ctx context.Context, appointmentID string) error
    RescheduleAppointment(ctx context.Context, appointmentID string, newSlot TimeSlot) (*Appointment, error)
}

type TimeSlot struct {
    Start     time.Time `json:"start"`
    End       time.Time `json:"end"`
    Available bool      `json:"available"`
}

type Appointment struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Start       time.Time `json:"start"`
    End         time.Time `json:"end"`
    Customer    CustomerInfo `json:"customer"`
    CalendarID  string    `json:"calendar_id"`
}

type CustomerInfo struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Phone string `json:"phone"`
    Notes string `json:"notes"`
}
```

**Calendar Service (`calendar.go`):**

```go
type CalendarService struct {
    store    *store.Store
    crypto   *crypto.EncryptionService
    providers map[string]CalendarProvider
}

func (s *CalendarService) GetAvailability(ctx context.Context, tenantID int32, date time.Time) ([]TimeSlot, error) {
    // 1. Load calendar integration
    integration, err := s.store.GetIntegrationByType(ctx, tenantID, "google_calendar")
    if err != nil {
        integration, err = s.store.GetIntegrationByType(ctx, tenantID, "outlook")
        if err != nil {
            return nil, fmt.Errorf("no calendar configured")
        }
    }
    
    // 2. Get provider
    provider := s.providers[integration.Type]
    
    // 3. Load config (working hours, buffer, slot duration)
    config := parseCalendarConfig(integration.ConfigPlaintext)
    
    // 4. Get raw availability
    slots, err := provider.GetAvailability(ctx, date, config.SlotDuration)
    if err != nil {
        return nil, err
    }
    
    // 5. Filter by working hours
    slots = filterByWorkingHours(slots, config.WorkingHours, config.WorkingDays)
    
    // 6. Add buffer time
    slots = addBuffer(slots, config.BufferMinutes)
    
    return slots, nil
}

func (s *CalendarService) BookAppointment(ctx context.Context, tenantID int32, slot TimeSlot, customer CustomerInfo) (*Appointment, error) {
    // 1. Validate slot is still available (double-check)
    available, _ := s.GetAvailability(ctx, tenantID, slot.Start)
    if !isSlotAvailable(available, slot) {
        return nil, fmt.Errorf("slot no longer available")
    }
    
    // 2. Load provider
    provider := s.getProvider(tenantID)
    
    // 3. Book
    appointment, err := provider.BookAppointment(ctx, slot, customer)
    if err != nil {
        return nil, err
    }
    
    // 4. Send confirmation SMS (if configured)
    s.smsService.ScheduleReminder(ctx, tenantID, appointment, 24)
    
    // 5. Dispatch event
    s.eventDispatcher.Dispatch(ctx, tenantID, "appointment.booked", appointment)
    
    return appointment, nil
}
```

### 5.4 OAuth2 Flow

**Authorization Endpoint:**
```
GET /api/v1/agent/:slug/integrations/:id/authorize
```

**Flow:**
```
1. Admin clicks "Authorize Calendar"
2. Generate state parameter (CSRF protection)
3. Redirect to OAuth consent screen:
   - Google: https://accounts.google.com/o/oauth2/v2/auth
   - Outlook: https://login.microsoftonline.com/common/oauth2/v2.0/authorize
4. Callback with auth code
5. Exchange for access_token + refresh_token
6. Encrypt tokens, store in agent_integrations
7. Redirect back to admin UI
```

**State Parameter:**
```go
type OAuthState struct {
    IntegrationID int32     `json:"integration_id"`
    TenantID      int32     `json:"tenant_id"`
    Expiry        time.Time `json:"expiry"`
    Nonce         string    `json:"nonce"`
}

// Sign with HMAC, store in session/cache
// Validate on callback
```

### 5.5 Database Migration

```sql
-- No new table needed — uses agent_integrations with type='google_calendar' or 'outlook'
-- Add calendar-specific fields to config_plaintext JSON:

-- Example config_plaintext for Google Calendar:
-- {
--   "calendar_id": "primary",
--   "slot_duration_minutes": 60,
--   "buffer_minutes": 15,
--   "working_hours": {"start": "09:00", "end": "17:00"},
--   "working_days": ["mon", "tue", "wed", "thu", "fri"],
--   "timezone": "America/New_York"
-- }

-- Add appointments table
CREATE TABLE IF NOT EXISTS agent_appointments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    integration_id INTEGER REFERENCES agent_integrations(id) ON DELETE SET NULL,
    external_id TEXT,  -- Google/Outlook event ID
    title TEXT NOT NULL,
    customer_name TEXT NOT NULL,
    customer_email TEXT,
    customer_phone TEXT,
    customer_notes TEXT,
    start_time BIGINT NOT NULL,
    end_time BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'confirmed' CHECK (status IN ('confirmed', 'cancelled', 'rescheduled')),
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_agent_appointments_tenant ON agent_appointments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_appointments_time ON agent_appointments(start_time, end_time);
CREATE INDEX IF NOT EXISTS idx_agent_appointments_customer ON agent_appointments(customer_email);
```

---

## 6. Phase 4: Unified Admin UI

### 6.1 Design

**New Section in Agent Admin: "Integrations"**

Location: After "Captured Leads" section, before "Chat Transcripts"

Color theme: Indigo (consistent with other action-oriented sections)

### 6.2 UI Components

**Main Section:**
```tsx
{/* Integrations - Permission Gated */}
{isAdmin && (
  <div className="bg-indigo-50 dark:bg-indigo-900/20 rounded-xl border border-indigo-200 dark:border-indigo-800 p-4">
    <div className="flex justify-between items-center mb-3">
      <div>
        <h3 className="font-medium text-indigo-700 dark:text-indigo-300">
          Integrations
        </h3>
        <p className="text-sm text-indigo-600 dark:text-indigo-400">
          Connect webhooks, SMS, and calendar services
        </p>
      </div>
      <Button color="primary" variant="outlined" onClick={() => toggle()}>
        <Icon className="w-4 h-4 mr-2" />
        {showPanel ? "Close" : "Open"} ({integrationCount})
      </Button>
    </div>
    {showPanel && <IntegrationsPanel />}
  </div>
)}
```

**Integrations Panel:**

```tsx
const IntegrationsPanel = () => {
  return (
    <div className="mt-4 space-y-4">
      {/* Webhooks Section */}
      <IntegrationCard
        type="webhook"
        title="Webhooks"
        description="Send events to external systems"
        icon={WebhookIcon}
        color="blue"
      />
      
      {/* SMS Section */}
      <IntegrationCard
        type="twilio"
        title="SMS (Twilio)"
        description="Send follow-up and reminder messages"
        icon={SMSIcon}
        color="green"
      />
      
      {/* Calendar Section */}
      <IntegrationCard
        type="calendar"
        title="Calendar"
        description="Book appointments in chat"
        icon={CalendarIcon}
        color="purple"
      />
      
      {/* Event Log */}
      <EventLogSection />
    </div>
  );
};
```

**Integration Card:**

```tsx
const IntegrationCard = ({ type, title, description, icon, color }) => {
  const integrations = agentAdminStore.state.integrations.filter(i => i.type === type);
  
  return (
    <div className={`bg-${color}-50 dark:bg-${color}-900/20 rounded-lg border border-${color}-200 dark:border-${color}-800 p-4`}>
      <div className="flex justify-between items-start">
        <div className="flex items-center gap-3">
          <div className={`p-2 bg-${color}-100 dark:bg-${color}-800 rounded-lg`}>
            <Icon className={`w-5 h-5 text-${color}-600 dark:text-${color}-400`} />
          </div>
          <div>
            <h4 className="font-medium text-gray-900 dark:text-white">{title}</h4>
            <p className="text-sm text-gray-500 dark:text-gray-400">{description}</p>
          </div>
        </div>
        <Button size="sm" color="primary" variant="outlined" onClick={() => openModal(type)}>
          + Add
        </Button>
      </div>
      
      {integrations.length > 0 ? (
        <div className="mt-3 space-y-2">
          {integrations.map(integration => (
            <IntegrationRow key={integration.id} integration={integration} />
          ))}
        </div>
      ) : (
        <p className="mt-3 text-sm text-gray-500 dark:text-gray-400">
          No {title.toLowerCase()} configured
        </p>
      )}
    </div>
  );
};
```

**Integration Row:**

```tsx
const IntegrationRow = ({ integration }) => {
  const [isActive, setIsActive] = useState(integration.is_active);
  
  return (
    <div className="flex items-center justify-between p-3 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
      <div className="flex items-center gap-3">
        <div className={`w-2 h-2 rounded-full ${isActive ? 'bg-green-500' : 'bg-gray-400'}`} />
        <div>
          <p className="font-medium text-gray-900 dark:text-white">{integration.name}</p>
          <p className="text-xs text-gray-500 dark:text-gray-400">
            {integration.type === 'webhook' && integration.config_plaintext.url}
            {integration.type === 'twilio' && `From: ${integration.config_plaintext.from_number}`}
            {integration.type === 'google_calendar' && `Calendar: ${integration.config_plaintext.calendar_id}`}
          </p>
        </div>
      </div>
      
      <div className="flex items-center gap-2">
        <Switch
          checked={isActive}
          onChange={(e) => handleToggleIntegration(integration.id, e.target.checked)}
          size="sm"
        />
        <Button size="sm" variant="plain" onClick={() => openEditModal(integration)}>
          Edit
        </Button>
        <Button size="sm" variant="plain" color="danger" onClick={() => handleDelete(integration.id)}>
          Delete
        </Button>
      </div>
    </div>
  );
};
```

### 6.3 Modals

**Create/Edit Integration Modal:**

```tsx
const IntegrationModal = ({ type, integration, onClose }) => {
  const [formData, setFormData] = useState({
    name: '',
    // Webhook fields
    url: '',
    events: ['lead.captured'],
    secret: '',
    // Twilio fields
    account_sid: '',
    auth_token: '',
    from_number: '',
    // Calendar fields
    provider: 'google_calendar',
    calendar_id: 'primary',
    slot_duration_minutes: 60,
    buffer_minutes: 15,
    working_hours_start: '09:00',
    working_hours_end: '17:00',
    working_days: ['mon', 'tue', 'wed', 'thu', 'fri'],
  });
  
  return (
    <Modal open onClose={onClose}>
      <ModalHeader>
        {integration ? 'Edit' : 'Add'} {type === 'webhook' ? 'Webhook' : type === 'twilio' ? 'SMS' : 'Calendar'}
      </ModalHeader>
      <ModalBody>
        {type === 'webhook' && <WebhookForm data={formData} onChange={setFormData} />}
        {type === 'twilio' && <TwilioForm data={formData} onChange={setFormData} />}
        {(type === 'google_calendar' || type === 'outlook') && <CalendarForm data={formData} onChange={setFormData} />}
      </ModalBody>
      <ModalFooter>
        <Button variant="outlined" onClick={onClose}>Cancel</Button>
        <Button onClick={handleSave}>Save</Button>
      </ModalFooter>
    </Modal>
  );
};
```

### 6.4 Event Log Section

```tsx
const EventLogSection = () => {
  const [events, setEvents] = useState([]);
  const [filter, setFilter] = useState('all');
  
  return (
    <div className="mt-4">
      <h4 className="font-medium text-gray-900 dark:text-white mb-3">Event Log</h4>
      
      <div className="flex gap-2 mb-3">
        <Button size="sm" variant={filter === 'all' ? 'solid' : 'outlined'} onClick={() => setFilter('all')}>
          All
        </Button>
        <Button size="sm" variant={filter === 'delivered' ? 'solid' : 'outlined'} onClick={() => setFilter('delivered')}>
          Delivered
        </Button>
        <Button size="sm" variant={filter === 'failed' ? 'solid' : 'outlined'} onClick={() => setFilter('failed')}>
          Failed
        </Button>
      </div>
      
      <div className="space-y-2 max-h-64 overflow-y-auto">
        {events.map(event => (
          <div key={event.id} className="flex items-center justify-between p-2 bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700">
            <div className="flex items-center gap-2">
              <div className={`w-2 h-2 rounded-full ${event.status === 'delivered' ? 'bg-green-500' : 'bg-red-500'}`} />
              <span className="text-sm font-medium text-gray-900 dark:text-white">{event.event_type}</span>
              <span className="text-xs text-gray-500 dark:text-gray-400">{formatTime(event.created_at)}</span>
            </div>
            {event.status === 'failed' && (
              <Tooltip title={event.last_error}>
                <ErrorIcon className="w-4 h-4 text-red-500" />
              </Tooltip>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};
```

---

## 7. Data Model Summary

### New Tables

| Table | Purpose |
|-------|---------|
| `agent_integrations` | Per-tenant integration configs (webhooks, Twilio, calendars) |
| `agent_events` | Event log for audit and retry |
| `agent_sms_messages` | Outbound SMS log with status |
| `agent_sms_optouts` | TCPA opt-out list |
| `agent_appointments` | Calendar appointments |

### Modified Tables

| Table | Changes |
|-------|---------|
| `tenant_config` | No changes needed — integrations are separate table |

### Entity Relationship

```
agent_tenants (1) ──< (N) agent_integrations
agent_tenants (1) ──< (N) agent_events
agent_tenants (1) ──< (N) agent_sms_messages
agent_tenants (1) ──< (N) agent_sms_optouts
agent_tenants (1) ──< (N) agent_appointments
agent_integrations (1) ──< (N) agent_sms_messages
agent_integrations (1) ──< (N) agent_appointments
```

---

## 8. Implementation Timeline

### Phase 1: Webhook Events (Weeks 1-2)

| Day | Task |
|-----|------|
| 1-2 | Create migration, AgentIntegration struct, store methods |
| 3-4 | Implement EventDispatcher with SSRF protection |
| 5-6 | Add webhook delivery with HMAC signing |
| 7-8 | Create admin API endpoints |
| 9-10 | Build webhook management UI |

### Phase 2: SMS Integration (Weeks 3-5)

| Day | Task |
|-----|------|
| 11-12 | Add Twilio SDK, implement SMSService |
| 13-14 | Create SMS templates engine |
| 15-16 | Implement cron job for delayed SMS |
| 17-18 | Add TCPA compliance (opt-out, HELP) |
| 19-20 | Build SMS management UI |
| 21-22 | Testing and error handling |

### Phase 3: Calendar Integration (Weeks 6-8)

| Day | Task |
|-----|------|
| 23-24 | Implement Google Calendar adapter |
| 25-26 | Implement Outlook Calendar adapter |
| 27-28 | Add OAuth2 flow |
| 29-30 | Implement availability checking |
| 31-32 | Add booking flow with double-check |
| 33-34 | Build calendar management UI |

### Phase 4: Unified Admin UI (Week 9)

| Day | Task |
|-----|------|
| 35-36 | Build Integrations panel |
| 37-38 | Build modals for each integration type |
| 39-40 | Build event log viewer |
| 41-42 | Polish UI, add tooltips, error states |

### Testing and Polish (Week 10)

| Day | Task |
|-----|------|
| 43-44 | Unit tests for all services |
| 45-46 | Integration tests |
| 47-48 | Security audit |
| 49-50 | Documentation and deployment |

---

## 9. Testing Strategy

### Unit Tests

| File | Coverage |
|------|----------|
| `events_test.go` | Event dispatching, HMAC signing, payload serialization |
| `sms_test.go` | Template rendering, phone validation, opt-out handling |
| `calendar_test.go` | Availability filtering, double-booking prevention |
| `webhook_test.go` | SSRF protection, IP pinning, redirect validation |

### Integration Tests

| Scenario | Expected Result |
|----------|-----------------|
| Lead captured → webhook fires | Webhook receives valid payload with HMAC signature |
| Lead captured → SMS scheduled | SMS queued for delivery in 24h |
| SMS scheduled → cron runs | SMS sent via Twilio, status updated |
| Customer replies "STOP" | Opt-out recorded, pending SMS cancelled |
| User asks to book → calendar check | Available slots returned |
| User confirms booking → calendar book | Appointment created, confirmation SMS sent |

### Security Tests

| Test | Expected Result |
|------|-----------------|
| Webhook to internal IP | Blocked with SSRF error |
| Webhook to metadata endpoint | Blocked with SSRF error |
| Webhook with invalid HMAC | Rejected by receiver (test endpoint) |
| SMS to invalid phone | Rejected with validation error |
| OAuth state parameter replay | Rejected with CSRF error |
| Credential encryption/decryption | Round-trip successful |

### Performance Tests

| Metric | Target |
|--------|--------|
| Webhook delivery latency | < 500ms (p95) |
| SMS send latency | < 2s (p95) |
| Calendar availability check | < 1s (p95) |
| Event dispatch overhead | < 50ms per event |

---

## 10. Rollout Plan

### Pre-Launch Checklist

- [ ] All migrations tested on SQLite + PostgreSQL
- [ ] SSRF protection tested against known attack vectors
- [ ] Credential encryption verified (round-trip test)
- [ ] TCPA compliance reviewed (opt-out, HELP, identification)
- [ ] OAuth flows tested with real Google/Microsoft accounts
- [ ] Twilio account configured with test credentials
- [ ] Rate limiting configured and tested
- [ ] Error handling graceful degradation verified
- [ ] Admin UI tested on Chrome, Firefox, Safari
- [ ] Mobile responsiveness verified

### Deployment Steps

1. **Database Migration**
   ```bash
   # Migrations auto-apply on startup
   ./build/memos --mode dev --data build/data
   ```

2. **Environment Variables**
   ```bash
   # No new env vars needed — integrations stored in DB
   # Existing ENCRYPTION_MASTER_KEY must be set
   ```

3. **Admin Setup**
   - Navigate to Agent Admin → Integrations
   - Add webhook (optional)
   - Add Twilio credentials (optional)
   - Authorize Google/Outlook calendar (optional)

4. **Verification**
   - Send test webhook
   - Send test SMS
   - Check calendar availability
   - Book test appointment

### Rollback Plan

If issues arise:

1. **Disable integrations** — Toggle off in Admin UI (no code changes needed)
2. **Database rollback** — Drop new tables (data loss, but non-critical)
3. **Code rollback** — Revert to previous deployment

### Monitoring

| Metric | Alert Threshold |
|--------|-----------------|
| Webhook delivery failures | > 5% in 5 minutes |
| SMS delivery failures | > 10% in 5 minutes |
| Calendar API errors | > 5% in 5 minutes |
| Credential decryption failures | Any occurrence |
| SSRF blocks | Any occurrence (potential attack) |

---

## Appendix A: File Changes Summary

### New Files

| File | Lines | Purpose |
|------|-------|---------|
| `server/router/api/v1/agent/events.go` | ~400 | Event dispatcher + webhook delivery |
| `server/router/api/v1/agent/sms.go` | ~300 | Twilio SMS service |
| `server/router/api/v1/agent/sms_templates.go` | ~150 | Message template engine |
| `server/router/api/v1/agent/calendar.go` | ~350 | Calendar service |
| `server/router/api/v1/agent/calendar_google.go` | ~250 | Google Calendar adapter |
| `server/router/api/v1/agent/calendar_outlook.go` | ~250 | Outlook Calendar adapter |
| `store/migration/sqlite/30__agent_integrations.sql` | ~80 | Database migration |
| `store/migration/sqlite/31__agent_sms.sql` | ~60 | SMS tables migration |
| `store/migration/sqlite/32__agent_appointments.sql` | ~40 | Appointments table migration |

### Modified Files

| File | Changes |
|------|---------|
| `store/agent.go` | Add AgentIntegration, AgentEvent, AgentSMSMessage, AgentAppointment structs |
| `store/driver.go` | Add integration, event, SMS, appointment CRUD methods |
| `store/db/sqlite/agent.go` | Implement new store methods |
| `server/router/api/v1/agent/service.go` | Hook event dispatcher into processChat() |
| `server/router/api/v1/agent/handlers.go` | Add integration CRUD endpoints |
| `server/router/api/v1/v1.go` | Register integration routes |
| `web/src/pages/AgentAdmin.tsx` | Add Integrations section |
| `web/src/store/v2/agentAdmin.ts` | Add integration state and methods |

---

## Appendix B: Security Checklist

- [ ] All credentials encrypted with AES-256-GCM
- [ ] SSRF protection on all outbound webhooks
- [ ] HMAC signature verification on webhook payloads
- [ ] OAuth state parameter CSRF protection
- [ ] Phone number validation before SMS send
- [ ] TCPA opt-out handling implemented
- [ ] Rate limiting on SMS sending
- [ ] Credential rotation support
- [ ] Audit logging for all integration actions
- [ ] Input sanitization on all user-provided data
- [ ] No credentials in logs or error messages
- [ ] HTTPS required for all external API calls
- [ ] Token refresh handled securely
- [ ] Graceful degradation when integrations fail

---

## Appendix C: API Reference

### Webhook Endpoints

```yaml
POST /api/v1/agent/:slug/integrations
  Body:
    type: "webhook"
    name: "My Webhook"
    config:
      url: "https://example.com/webhook"
      events: ["lead.captured", "escalation.created"]
      secret: "auto-generated"
  Response:
    id: 1
    type: "webhook"
    name: "My Webhook"
    config_plaintext:
      url: "https://example.com/webhook"
      events: ["lead.captured", "escalation.created"]
    is_active: true

POST /api/v1/agent/:slug/integrations/:id/test
  Response:
    success: true
    event_id: "evt_xxx"
    delivery_status: "delivered"
```

### SMS Endpoints

```yaml
POST /api/v1/agent/:slug/integrations
  Body:
    type: "twilio"
    name: "Twilio SMS"
    config:
      account_sid: "ACxxxxx"
      auth_token: "encrypted"
      from_number: "+1234567890"
      templates:
        follow_up:
          body: "Hi {name}, thanks for reaching out!"
          delay_hours: 24
  Response:
    id: 2
    type: "twilio"
    name: "Twilio SMS"
    is_active: true
```

### Calendar Endpoints

```yaml
POST /api/v1/agent/:slug/integrations
  Body:
    type: "google_calendar"
    name: "Google Calendar"
    config:
      calendar_id: "primary"
      slot_duration_minutes: 60
      buffer_minutes: 15
      working_hours:
        start: "09:00"
        end: "17:00"
      working_days: ["mon", "tue", "wed", "thu", "fri"]
  Response:
    id: 3
    type: "google_calendar"
    name: "Google Calendar"
    is_active: false  # Needs OAuth authorization

GET /api/v1/agent/:slug/integrations/:id/authorize
  Response:
    redirect_url: "https://accounts.google.com/o/oauth2/v2/auth?..."
```

---

*Plan version: 1.0*
*Created: 2026-07-10*
*Status: Ready for implementation*
