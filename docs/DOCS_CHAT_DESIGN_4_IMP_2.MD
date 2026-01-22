# AI Chat Agent Implementation Specification

**Version:** 4.0-IMP-2
**Based On:** [DOCS_CHAT_DESIGN_4_IMP.MD](./DOCS_CHAT_DESIGN_4_IMP.MD)
**Date:** 2026-01-18

---

## Revision Summary

| Aspect | v1 | v2 |
|--------|-----|-----|
| Audiences | Single | Dual (external + internal) |
| Authentication | TBD | External: none, Internal: Memos auth |
| Session storage | TBD | External: in-memory, Internal: SQLite |
| Files per tenant | 2 (KB + POLICY) | 4 (2 per audience) |
| Frontend | API only | Includes chat widget |
| Admin auth | TBD | Memos user/role system |
| Rate limiting | TBD | Yes, per tenant |

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Dual-Audience Design](#2-dual-audience-design)
3. [Database Schema](#3-database-schema)
4. [File Structure per Tenant](#4-file-structure-per-tenant)
5. [API Endpoints](#5-api-endpoints)
6. [Core Services](#6-core-services)
7. [Frontend Chat Widget](#7-frontend-chat-widget)
8. [Rate Limiting](#8-rate-limiting)
9. [Testing Plan](#9-testing-plan)
10. [Implementation Sequence](#10-implementation-sequence)

---

## 1. Architecture Overview

### 1.1 System Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              MEMOS APPLICATION                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  FRONTEND                                                                    │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                                                                      │    │
│  │  ┌──────────────────┐              ┌──────────────────┐             │    │
│  │  │  External Chat   │              │  Internal Chat   │             │    │
│  │  │  Widget          │              │  Page            │             │    │
│  │  │  (Lower-right    │              │  (Authenticated  │             │    │
│  │  │   corner, anon)  │              │   users only)    │             │    │
│  │  └────────┬─────────┘              └────────┬─────────┘             │    │
│  │           │                                 │                        │    │
│  └───────────┼─────────────────────────────────┼────────────────────────┘    │
│              │                                 │                              │
│              ▼                                 ▼                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         HTTP LAYER (Echo)                              │  │
│  │                                                                        │  │
│  │  EXTERNAL (No Auth)                 INTERNAL (Memos Auth Required)     │  │
│  │  ──────────────────                 ──────────────────────────────     │  │
│  │  POST /api/v1/agent/:slug/chat/ext  POST /api/v1/agent/:slug/chat/int  │  │
│  │                                                                        │  │
│  │  ADMIN (Memos Admin Role Required)                                     │  │
│  │  ─────────────────────────────────                                     │  │
│  │  POST /api/v1/agent/onboard                                            │  │
│  │  POST /api/v1/agent/:slug/import                                       │  │
│  │  GET  /api/v1/agent/:slug/export                                       │  │
│  │  GET  /api/v1/agent/:slug/config                                       │  │
│  │  PUT  /api/v1/agent/:slug/config                                       │  │
│  │                                                                        │  │
│  └────────────────────────────────────┬───────────────────────────────────┘  │
│                                       │                                      │
│  ┌────────────────────────────────────▼───────────────────────────────────┐  │
│  │                          SERVICE LAYER                                  │  │
│  │                                                                         │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │  │
│  │  │    Agent     │  │   Config     │  │   Session    │                  │  │
│  │  │   Service    │  │   Loader     │  │   Store      │                  │  │
│  │  │              │  │              │  │              │                  │  │
│  │  │ • ChatExt()  │  │ • Load()     │  │ • Memory     │                  │  │
│  │  │ • ChatInt()  │  │ • Cache()    │  │   (external) │                  │  │
│  │  │ • Classify() │  │              │  │ • SQLite     │                  │  │
│  │  │ • Respond()  │  │              │  │   (internal) │                  │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                  │  │
│  │                                                                         │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │  │
│  │  │   Policy     │  │    LLM       │  │    Rate      │                  │  │
│  │  │   Engine     │  │   Client     │  │   Limiter    │                  │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                  │  │
│  │                                                                         │  │
│  └─────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                          DATA LAYER (SQLite)                            │ │
│  │                                                                         │ │
│  │  agent_tenants │ agent_audiences │ agent_configs │ agent_sessions      │ │
│  │                                                                         │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Request Flow Comparison

```
EXTERNAL USER (Anonymous)                 INTERNAL USER (Authenticated)
─────────────────────────                 ────────────────────────────

Customer visits website                   Employee logs into Memos
        │                                         │
        ▼                                         ▼
┌───────────────┐                         ┌───────────────┐
│ Chat Widget   │                         │ Internal Chat │
│ (lower-right) │                         │ Page          │
└───────┬───────┘                         └───────┬───────┘
        │                                         │
        ▼                                         ▼
POST /chat/ext                            POST /chat/int
(no auth header)                          (with auth token)
        │                                         │
        ▼                                         ▼
Load EXTERNAL config                      Load INTERNAL config
(booking, inquiries KB)                   (troubleshooting KB)
        │                                         │
        ▼                                         ▼
In-memory session                         SQLite session
(lost on restart)                         (persisted)
        │                                         │
        ▼                                         ▼
LLM Classification                        LLM Classification
        │                                         │
        ▼                                         ▼
Policy Evaluation                         Policy Evaluation
        │                                         │
        ▼                                         ▼
LLM Response                              LLM Response
        │                                         │
        ▼                                         ▼
Return to widget                          Return to chat page
```

---

## 2. Dual-Audience Design

### 2.1 Audience Concept

Each tenant has **two audiences** with separate configurations:

| Audience | Code | Authentication | Session | Purpose |
|----------|------|----------------|---------|---------|
| **External** | `ext` | None (anonymous) | In-memory | Customer-facing: booking, inquiries, emergencies |
| **Internal** | `int` | Memos user auth | SQLite | Employee-facing: troubleshooting, internal processes |

### 2.2 Files per Tenant

```
/tenants/acme-restoration/
├── external/
│   ├── KB.MD           # Customer-facing knowledge base
│   └── POLICY.MD       # Customer-facing policies
└── internal/
    ├── KB.MD           # Internal knowledge base (includes troubleshooting)
    └── POLICY.MD       # Internal policies
```

### 2.3 Content Differences

| Aspect | External KB/Policy | Internal KB/Policy |
|--------|-------------------|-------------------|
| **Services** | Customer-facing descriptions | Technical details |
| **FAQs** | Common customer questions | Internal procedures |
| **Safety** | Customer safety instructions | Technician protocols |
| **Intents** | Booking, inquiries, emergencies | Troubleshooting, escalation paths |
| **Tone** | Professional, empathetic | Direct, technical |
| **Contact info** | Customer hotline | Internal extensions |

### 2.4 Example: External vs Internal KB

**External KB.MD (Customer-Facing):**
```markdown
## Services

### Water Damage Restoration
<!-- @service: water_damage, emergency: true -->

We provide 24/7 emergency water damage restoration.
Call us immediately at (555) 123-4567.

**Response Time:** 30 minutes
```

**Internal KB.MD (Employee-Facing):**
```markdown
## Services

### Water Damage Restoration
<!-- @service: water_damage, emergency: true -->

**Dispatch Protocol:**
1. Verify customer location is in service area
2. Check technician availability in dispatch system
3. Assign nearest available crew
4. ETA must be within 30 minutes

**Equipment Required:**
- Truck-mounted extractor
- Industrial dehumidifiers (2+)
- Moisture meters

**Troubleshooting Common Issues:**
- If customer reports "water still coming" → Advise shutoff valve location
- If customer in basement → Verify electrical safety first
```

---

## 3. Database Schema

### 3.1 Updated Schema

```sql
-- ============================================================================
-- TENANT TABLES
-- ============================================================================

CREATE TABLE agent_tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    company_name TEXT NOT NULL,
    vertical TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audience-specific configuration
CREATE TABLE agent_audiences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,            -- 'external' or 'internal'

    -- Identity
    role TEXT NOT NULL,
    tone TEXT NOT NULL,
    brand_voice TEXT,
    guidelines TEXT,                        -- JSON array

    -- Contact
    emergency_phone TEXT NOT NULL,
    secondary_phones TEXT,                  -- JSON array
    email TEXT,
    address TEXT,

    -- Thresholds
    emergency_urgency_threshold INTEGER DEFAULT 4,
    escalation_confidence_threshold REAL DEFAULT 0.85,

    -- Rate limiting (per audience)
    rate_limit_rpm INTEGER DEFAULT 60,      -- Requests per minute

    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(tenant_id, audience_type)
);

-- ============================================================================
-- KNOWLEDGE BASE TABLES (per audience)
-- ============================================================================

CREATE TABLE agent_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,            -- 'external' or 'internal'
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    is_emergency BOOLEAN DEFAULT FALSE,
    response_time TEXT,
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, audience_type, code)
);

CREATE TABLE agent_exclusions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    exception_rule TEXT,
    referral TEXT,
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, audience_type, code)
);

CREATE TABLE agent_coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    -- Coverage is shared across audiences (same service area)
    area_type TEXT NOT NULL,
    area_name TEXT NOT NULL,
    state_code TEXT,
    is_included BOOLEAN NOT NULL,

    UNIQUE(tenant_id, area_type, area_name)
);

CREATE TABLE agent_faqs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, audience_type, code)
);

CREATE TABLE agent_safety_protocols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    trigger_intents TEXT NOT NULL,          -- JSON
    instructions TEXT NOT NULL,             -- JSON
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, audience_type, code)
);

CREATE TABLE agent_kb_sections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    section_type TEXT DEFAULT 'general',
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, audience_type, code)
);

-- ============================================================================
-- POLICY TABLES (per audience)
-- ============================================================================

CREATE TABLE agent_intents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT,                     -- NULL = global, 'external'/'internal' = audience-specific
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    description TEXT NOT NULL,
    examples TEXT,                          -- JSON
    counter_examples TEXT,                  -- JSON
    urgency INTEGER,
    action TEXT NOT NULL,
    confidence_threshold REAL,
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(COALESCE(tenant_id, 0), COALESCE(audience_type, ''), code)
);

CREATE TABLE agent_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    priority INTEGER DEFAULT 5,
    applies_to TEXT,
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, audience_type, code)
);

-- ============================================================================
-- SESSION TABLES
-- ============================================================================

-- Internal sessions only (external sessions are in-memory)
CREATE TABLE agent_sessions (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES user(id),    -- Memos user for internal sessions
    audience_type TEXT NOT NULL DEFAULT 'internal',

    -- State
    phase TEXT DEFAULT 'triage',
    current_intent TEXT,
    urgency_level INTEGER DEFAULT 0,
    coverage_status TEXT DEFAULT 'unknown',

    -- Extracted data
    customer_name TEXT,
    customer_phone TEXT,
    customer_location TEXT,
    detected_service TEXT,

    -- History
    message_count INTEGER DEFAULT 0,
    messages TEXT DEFAULT '[]',             -- JSON

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    is_completed BOOLEAN DEFAULT FALSE,
    completion_reason TEXT
);

-- ============================================================================
-- SOURCE FILES
-- ============================================================================

CREATE TABLE agent_source_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,            -- 'external' or 'internal'
    file_type TEXT NOT NULL,                -- 'kb' or 'policy'
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(tenant_id, audience_type, file_type)
);

-- ============================================================================
-- RATE LIMITING
-- ============================================================================

CREATE TABLE agent_rate_limits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    request_count INTEGER DEFAULT 0,
    window_start TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(tenant_id, audience_type, client_ip)
);

-- ============================================================================
-- INDEXES
-- ============================================================================

CREATE INDEX idx_audiences_tenant ON agent_audiences(tenant_id, audience_type);
CREATE INDEX idx_services_tenant_audience ON agent_services(tenant_id, audience_type);
CREATE INDEX idx_intents_tenant_audience ON agent_intents(tenant_id, audience_type);
CREATE INDEX idx_sessions_tenant ON agent_sessions(tenant_id);
CREATE INDEX idx_sessions_user ON agent_sessions(user_id);
CREATE INDEX idx_rate_limits_lookup ON agent_rate_limits(tenant_id, audience_type, client_ip);
```

---

## 4. File Structure per Tenant

### 4.1 Onboarding File Structure

```
/tenants/acme-restoration/
├── external/
│   ├── KB.MD           # Customer-facing KB
│   └── POLICY.MD       # Customer-facing policy
└── internal/
    ├── KB.MD           # Internal KB (troubleshooting, procedures)
    └── POLICY.MD       # Internal policy
```

### 4.2 Onboarding API

```http
POST /api/v1/agent/onboard
Content-Type: multipart/form-data
Authorization: Bearer <admin-token>

tenant_slug: acme-restoration
vertical: restoration

# External audience files
external_kb_file: [KB.MD]
external_policy_file: [POLICY.MD]

# Internal audience files
internal_kb_file: [KB.MD]
internal_policy_file: [POLICY.MD]
```

**Response:**
```json
{
  "success": true,
  "tenant": {
    "id": 1,
    "slug": "acme-restoration",
    "company_name": "ACME Restoration Inc."
  },
  "audiences": {
    "external": {
      "services_count": 5,
      "intents_count": 10,
      "faqs_count": 8
    },
    "internal": {
      "services_count": 5,
      "intents_count": 15,
      "faqs_count": 20
    }
  },
  "endpoints": {
    "external_chat": "/api/v1/agent/acme-restoration/chat/ext",
    "internal_chat": "/api/v1/agent/acme-restoration/chat/int",
    "widget_script": "/api/v1/agent/acme-restoration/widget.js"
  }
}
```

---

## 5. API Endpoints

### 5.1 Endpoint Summary

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| **Chat** |
| POST | `/api/v1/agent/:slug/chat/ext` | None | External chat (customers) |
| POST | `/api/v1/agent/:slug/chat/int` | Memos User | Internal chat (employees) |
| **Admin** |
| POST | `/api/v1/agent/onboard` | Memos Admin | Create tenant |
| POST | `/api/v1/agent/:slug/import` | Memos Admin | Re-import files |
| GET | `/api/v1/agent/:slug/export` | Memos Admin | Export to files |
| GET | `/api/v1/agent/:slug/config` | Memos Admin | Get configuration |
| PUT | `/api/v1/agent/:slug/config` | Memos Admin | Update configuration |
| DELETE | `/api/v1/agent/:slug` | Memos Admin | Delete tenant |
| **Widget** |
| GET | `/api/v1/agent/:slug/widget.js` | None | Chat widget script |

### 5.2 External Chat Endpoint

```http
POST /api/v1/agent/:slug/chat/ext
Content-Type: application/json
X-Client-IP: <optional, for rate limiting behind proxy>
```

**Request:**
```json
{
  "session_id": "optional-uuid",
  "message": "I need help with water damage"
}
```

**Response:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": {
    "role": "assistant",
    "content": "I'm sorry to hear about the water damage...",
    "timestamp": "2026-01-18T10:30:00Z"
  },
  "metadata": {
    "intent": "service_inquiry",
    "urgency": 2,
    "phase": "triage"
  }
}
```

**Rate Limit Response (429):**
```json
{
  "error": "rate_limit_exceeded",
  "message": "Too many requests. Please try again in 60 seconds.",
  "retry_after": 60
}
```

### 5.3 Internal Chat Endpoint

```http
POST /api/v1/agent/:slug/chat/int
Content-Type: application/json
Authorization: Bearer <memos-token>
```

**Request:**
```json
{
  "session_id": "optional-uuid",
  "message": "Customer reports water still coming after shutoff"
}
```

**Response:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": {
    "role": "assistant",
    "content": "If water continues after main shutoff, check:\n1. Water heater isolation valve...",
    "timestamp": "2026-01-18T10:30:00Z"
  },
  "metadata": {
    "intent": "troubleshooting_water",
    "urgency": 3,
    "phase": "handshake"
  },
  "session_persisted": true
}
```

### 5.4 Widget Script Endpoint

```http
GET /api/v1/agent/:slug/widget.js
```

Returns a JavaScript file that can be embedded in any website:

```html
<!-- Add to customer-facing website -->
<script src="https://memos.example.com/api/v1/agent/acme-restoration/widget.js"></script>
```

---

## 6. Core Services

### 6.1 Agent Service (Updated)

```go
// services/agent/agent.go

type AgentService struct {
    store           *store.Store
    configLoader    *ConfigLoader
    llmClient       llm.Client
    memorySessions  *MemorySessionStore  // For external
    sqliteSessions  *SQLiteSessionStore  // For internal
    rateLimiter     *RateLimiter
}

// ChatExternal handles anonymous customer chat
func (s *AgentService) ChatExternal(ctx context.Context, tenantSlug string, clientIP string, req ChatRequest) (*ChatResponse, error) {
    // 1. Check rate limit
    if !s.rateLimiter.Allow(tenantSlug, "external", clientIP) {
        return nil, ErrRateLimitExceeded
    }

    // 2. Load external config
    config, err := s.configLoader.Load(tenantSlug, "external")
    if err != nil {
        return nil, err
    }

    // 3. Get or create in-memory session
    session := s.memorySessions.GetOrCreate(req.SessionID, config.TenantID)

    // 4. Process chat
    return s.processChat(ctx, config, session, req)
}

// ChatInternal handles authenticated employee chat
func (s *AgentService) ChatInternal(ctx context.Context, tenantSlug string, userID int32, req ChatRequest) (*ChatResponse, error) {
    // 1. Load internal config
    config, err := s.configLoader.Load(tenantSlug, "internal")
    if err != nil {
        return nil, err
    }

    // 2. Get or create SQLite session (persisted)
    session := s.sqliteSessions.GetOrCreate(req.SessionID, config.TenantID, userID)

    // 3. Process chat
    response, err := s.processChat(ctx, config, session, req)
    if err != nil {
        return nil, err
    }

    // 4. Persist session to SQLite
    if err := s.sqliteSessions.Save(session); err != nil {
        // Log but don't fail the request
        log.Printf("Failed to persist session: %v", err)
    }

    response.SessionPersisted = true
    return response, nil
}

// processChat is shared logic for both audiences
func (s *AgentService) processChat(ctx context.Context, config *AudienceConfig, session *Session, req ChatRequest) (*ChatResponse, error) {
    // Add user message
    session.AddMessage("user", req.Message)

    // Classify
    classification, err := s.classify(ctx, config, session, req.Message)
    if err != nil {
        return nil, err
    }

    // Evaluate policy
    decision := s.policyEngine.Evaluate(config, session, classification)

    // Generate response
    response, err := s.generateResponse(ctx, config, session, classification, decision)
    if err != nil {
        return nil, err
    }

    // Update session
    session.AddMessage("assistant", response)
    session.Phase = decision.Phase
    session.CurrentIntent = classification.PrimaryIntent.Code
    session.UrgencyLevel = classification.Urgency.Level

    return &ChatResponse{
        SessionID: session.ID,
        Message: ResponseMessage{
            Role:      "assistant",
            Content:   response,
            Timestamp: time.Now(),
        },
        Metadata: ChatMetadata{
            Intent:   classification.PrimaryIntent.Code,
            Urgency:  classification.Urgency.Level,
            Phase:    decision.Phase,
        },
    }, nil
}
```

### 6.2 Config Loader (Updated)

```go
// services/agent/config.go

type ConfigLoader struct {
    store    *store.Store
    cache    map[string]*cachedConfig  // key: "tenant:audience"
    cacheMu  sync.RWMutex
    cacheTTL time.Duration
}

type AudienceConfig struct {
    TenantID     int
    TenantSlug   string
    AudienceType string  // "external" or "internal"
    CompanyName  string

    // Identity
    Role       string
    Tone       string
    BrandVoice string

    // Contact
    EmergencyPhone string

    // Thresholds
    EmergencyUrgencyThreshold     int
    EscalationConfidenceThreshold float64

    // Rate limiting
    RateLimitRPM int

    // Data (audience-specific)
    Services        []Service
    Exclusions      []Exclusion
    CoverageInclude []CoverageArea  // Shared across audiences
    CoverageExclude []CoverageArea  // Shared across audiences
    Intents         []Intent
    Rules           []Rule
    SafetyProtocols []SafetyProtocol
    FAQs            []FAQ
}

func (l *ConfigLoader) Load(tenantSlug, audienceType string) (*AudienceConfig, error) {
    cacheKey := fmt.Sprintf("%s:%s", tenantSlug, audienceType)

    // Check cache
    l.cacheMu.RLock()
    if cached, ok := l.cache[cacheKey]; ok {
        if time.Since(cached.loadedAt) < l.cacheTTL {
            l.cacheMu.RUnlock()
            return cached.config, nil
        }
    }
    l.cacheMu.RUnlock()

    // Load from database
    config, err := l.loadFromDB(tenantSlug, audienceType)
    if err != nil {
        return nil, err
    }

    // Cache
    l.cacheMu.Lock()
    l.cache[cacheKey] = &cachedConfig{config: config, loadedAt: time.Now()}
    l.cacheMu.Unlock()

    return config, nil
}
```

### 6.3 Session Stores

```go
// services/agent/session_memory.go

// MemorySessionStore for external (anonymous) sessions
type MemorySessionStore struct {
    sessions map[string]*Session
    mu       sync.RWMutex
    ttl      time.Duration
}

func NewMemorySessionStore(ttl time.Duration) *MemorySessionStore {
    store := &MemorySessionStore{
        sessions: make(map[string]*Session),
        ttl:      ttl,
    }
    // Start cleanup goroutine
    go store.cleanupLoop()
    return store
}

func (s *MemorySessionStore) GetOrCreate(sessionID string, tenantID int) *Session {
    if sessionID != "" {
        s.mu.RLock()
        if session, ok := s.sessions[sessionID]; ok {
            s.mu.RUnlock()
            return session
        }
        s.mu.RUnlock()
    }

    // Create new
    session := &Session{
        ID:           uuid.New().String(),
        TenantID:     tenantID,
        AudienceType: "external",
        Phase:        "triage",
        Messages:     []Message{},
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }

    s.mu.Lock()
    s.sessions[session.ID] = session
    s.mu.Unlock()

    return session
}

func (s *MemorySessionStore) cleanupLoop() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        s.cleanup()
    }
}

func (s *MemorySessionStore) cleanup() {
    s.mu.Lock()
    defer s.mu.Unlock()

    cutoff := time.Now().Add(-s.ttl)
    for id, session := range s.sessions {
        if session.UpdatedAt.Before(cutoff) {
            delete(s.sessions, id)
        }
    }
}
```

```go
// services/agent/session_sqlite.go

// SQLiteSessionStore for internal (authenticated) sessions
type SQLiteSessionStore struct {
    store *store.Store
}

func (s *SQLiteSessionStore) GetOrCreate(sessionID string, tenantID int, userID int32) *Session {
    if sessionID != "" {
        session, err := s.store.GetAgentSession(sessionID)
        if err == nil && session.TenantID == tenantID {
            return session
        }
    }

    // Create new
    session := &Session{
        ID:           uuid.New().String(),
        TenantID:     tenantID,
        UserID:       userID,
        AudienceType: "internal",
        Phase:        "triage",
        Messages:     []Message{},
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }

    return session
}

func (s *SQLiteSessionStore) Save(session *Session) error {
    return s.store.UpsertAgentSession(session)
}

func (s *SQLiteSessionStore) GetUserSessions(userID int32) ([]*Session, error) {
    return s.store.GetAgentSessionsByUser(userID)
}
```

---

## 7. Frontend Chat Widget

### 7.1 Widget Component (External)

```typescript
// web/src/components/AgentChatWidget.tsx

import { useState, useRef, useEffect } from "react";
import { MessageSquareIcon, SendIcon, XIcon, MinusIcon } from "lucide-react";

interface Message {
  role: "user" | "assistant";
  content: string;
  timestamp: Date;
}

interface AgentChatWidgetProps {
  tenantSlug: string;
  position?: "bottom-right" | "bottom-left";
  primaryColor?: string;
}

const AgentChatWidget = ({
  tenantSlug,
  position = "bottom-right",
  primaryColor = "#0d9488"
}: AgentChatWidgetProps) => {
  const [isOpen, setIsOpen] = useState(false);
  const [isMinimized, setIsMinimized] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const sendMessage = async () => {
    if (!input.trim() || isLoading) return;

    const userMessage = input.trim();
    setInput("");
    setError(null);

    // Add user message immediately
    setMessages(prev => [...prev, {
      role: "user",
      content: userMessage,
      timestamp: new Date()
    }]);

    setIsLoading(true);

    try {
      const response = await fetch(`/api/v1/agent/${tenantSlug}/chat/ext`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          session_id: sessionId,
          message: userMessage
        })
      });

      if (response.status === 429) {
        setError("Too many messages. Please wait a moment.");
        return;
      }

      if (!response.ok) {
        throw new Error("Failed to send message");
      }

      const data = await response.json();
      setSessionId(data.session_id);

      // Add assistant message
      setMessages(prev => [...prev, {
        role: "assistant",
        content: data.message.content,
        timestamp: new Date(data.message.timestamp)
      }]);
    } catch (err) {
      setError("Something went wrong. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const positionClasses = position === "bottom-right"
    ? "right-4 bottom-4"
    : "left-4 bottom-4";

  if (!isOpen) {
    return (
      <button
        onClick={() => setIsOpen(true)}
        className={`fixed ${positionClasses} z-50 p-4 rounded-full shadow-lg text-white transition-transform hover:scale-110`}
        style={{ backgroundColor: primaryColor }}
        aria-label="Open chat"
      >
        <MessageSquareIcon className="w-6 h-6" />
      </button>
    );
  }

  return (
    <div className={`fixed ${positionClasses} z-50 w-80 sm:w-96 flex flex-col bg-white rounded-lg shadow-2xl overflow-hidden`}
         style={{ height: isMinimized ? "auto" : "500px" }}>
      {/* Header */}
      <div
        className="flex items-center justify-between p-3 text-white"
        style={{ backgroundColor: primaryColor }}
      >
        <div className="flex items-center gap-2">
          <MessageSquareIcon className="w-5 h-5" />
          <span className="font-medium">Chat with us</span>
        </div>
        <div className="flex gap-1">
          <button onClick={() => setIsMinimized(!isMinimized)} className="p-1 hover:bg-white/20 rounded">
            <MinusIcon className="w-4 h-4" />
          </button>
          <button onClick={() => setIsOpen(false)} className="p-1 hover:bg-white/20 rounded">
            <XIcon className="w-4 h-4" />
          </button>
        </div>
      </div>

      {!isMinimized && (
        <>
          {/* Messages */}
          <div className="flex-1 overflow-y-auto p-3 space-y-3 bg-gray-50">
            {messages.length === 0 && (
              <div className="text-center text-gray-500 py-8">
                <MessageSquareIcon className="w-10 h-10 mx-auto mb-2 opacity-30" />
                <p>How can we help you today?</p>
              </div>
            )}
            {messages.map((msg, idx) => (
              <div
                key={idx}
                className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
              >
                <div
                  className={`max-w-[80%] p-2.5 rounded-lg text-sm ${
                    msg.role === "user"
                      ? "bg-teal-500 text-white"
                      : "bg-white border border-gray-200 text-gray-800"
                  }`}
                >
                  <p className="whitespace-pre-wrap">{msg.content}</p>
                </div>
              </div>
            ))}
            {isLoading && (
              <div className="flex justify-start">
                <div className="bg-white border border-gray-200 p-2.5 rounded-lg">
                  <span className="animate-pulse text-gray-500">Typing...</span>
                </div>
              </div>
            )}
            {error && (
              <div className="text-center text-red-500 text-sm">{error}</div>
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Input */}
          <div className="p-3 border-t bg-white">
            <div className="flex gap-2">
              <input
                type="text"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Type your message..."
                className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:border-teal-500"
                disabled={isLoading}
              />
              <button
                onClick={sendMessage}
                disabled={!input.trim() || isLoading}
                className="p-2 rounded-lg text-white disabled:opacity-50"
                style={{ backgroundColor: primaryColor }}
              >
                <SendIcon className="w-4 h-4" />
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  );
};

export default AgentChatWidget;
```

### 7.2 Widget Script Generator

```go
// handlers/agent_widget.go

func (s *APIV1Service) GetAgentWidget(c echo.Context) error {
    tenantSlug := c.Param("slug")

    // Verify tenant exists and is active
    tenant, err := s.store.GetAgentTenantBySlug(tenantSlug)
    if err != nil || !tenant.IsActive {
        return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
    }

    // Get external config for branding
    config, err := s.agentService.configLoader.Load(tenantSlug, "external")
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load config")
    }

    // Generate widget script
    script := fmt.Sprintf(`
(function() {
  var container = document.createElement('div');
  container.id = 'agent-chat-widget-root';
  document.body.appendChild(container);

  var script = document.createElement('script');
  script.src = '%s/static/agent-widget.js';
  script.onload = function() {
    window.AgentChatWidget.init({
      container: container,
      tenantSlug: '%s',
      apiBase: '%s',
      primaryColor: '%s',
      companyName: '%s'
    });
  };
  document.body.appendChild(script);

  var style = document.createElement('link');
  style.rel = 'stylesheet';
  style.href = '%s/static/agent-widget.css';
  document.head.appendChild(style);
})();
`, s.baseURL, tenantSlug, s.baseURL, config.PrimaryColor, config.CompanyName, s.baseURL)

    c.Response().Header().Set("Content-Type", "application/javascript")
    return c.String(http.StatusOK, script)
}
```

### 7.3 Internal Chat Page

```typescript
// web/src/pages/AgentChat.tsx

import { observer } from "mobx-react-lite";
import { useState, useEffect, useRef } from "react";
import { useTranslate } from "@/utils/i18n";
import { agentChatStore } from "@/store/v2";

const AgentChat = observer(() => {
  const t = useTranslate();
  const [input, setInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const { messages, isLoading, error, sessionId } = agentChatStore.state;

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSend = async () => {
    if (!input.trim() || isLoading) return;
    const message = input.trim();
    setInput("");
    await agentChatStore.sendMessage(message);
  };

  // ... similar to Chat.tsx but calls /chat/int endpoint
  // and shows session history for internal users
});

export default AgentChat;
```

---

## 8. Rate Limiting

### 8.1 Rate Limiter Implementation

```go
// services/agent/ratelimit.go

type RateLimiter struct {
    store       *store.Store
    defaultRPM  int
    windowSize  time.Duration
}

func NewRateLimiter(store *store.Store, defaultRPM int) *RateLimiter {
    return &RateLimiter{
        store:      store,
        defaultRPM: defaultRPM,
        windowSize: time.Minute,
    }
}

func (r *RateLimiter) Allow(tenantSlug, audienceType, clientIP string) bool {
    // Get tenant-specific limit
    config, err := r.store.GetAgentAudienceConfig(tenantSlug, audienceType)
    limit := r.defaultRPM
    if err == nil && config.RateLimitRPM > 0 {
        limit = config.RateLimitRPM
    }

    // Check current count
    tenant, _ := r.store.GetAgentTenantBySlug(tenantSlug)
    count, windowStart, err := r.store.GetRateLimitCount(tenant.ID, audienceType, clientIP)

    now := time.Now()

    // Reset window if expired
    if err != nil || now.Sub(windowStart) > r.windowSize {
        r.store.ResetRateLimit(tenant.ID, audienceType, clientIP, now)
        return true
    }

    // Check if under limit
    if count >= limit {
        return false
    }

    // Increment counter
    r.store.IncrementRateLimit(tenant.ID, audienceType, clientIP)
    return true
}
```

### 8.2 Default Limits

| Audience | Default RPM | Configurable |
|----------|-------------|--------------|
| External | 30 | Yes, per tenant |
| Internal | 60 | Yes, per tenant |

---

## 9. Testing Plan

### 9.1 Test Matrix

| Test | External | Internal |
|------|----------|----------|
| Chat without session | ✓ | ✓ |
| Chat with session | ✓ | ✓ |
| Session persistence | Memory only | SQLite |
| Auth required | No | Yes |
| Rate limiting | ✓ | Optional |
| Emergency flow | ✓ | ✓ |
| Troubleshooting flow | N/A | ✓ |

### 9.2 API Test Script

```bash
#!/bin/bash
# test_agent_api.sh

BASE_URL="http://localhost:8080/api/v1/agent"
ADMIN_TOKEN="<memos-admin-token>"
USER_TOKEN="<memos-user-token>"

echo "=== 1. Onboard Tenant ==="
curl -X POST "$BASE_URL/onboard" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -F "tenant_slug=test-company" \
  -F "external_kb_file=@testdata/external/KB.MD" \
  -F "external_policy_file=@testdata/external/POLICY.MD" \
  -F "internal_kb_file=@testdata/internal/KB.MD" \
  -F "internal_policy_file=@testdata/internal/POLICY.MD" \
  | jq .

echo ""
echo "=== 2. External Chat (No Auth) ==="
curl -X POST "$BASE_URL/test-company/chat/ext" \
  -H "Content-Type: application/json" \
  -d '{"message": "My basement is flooding!"}' \
  | jq .

echo ""
echo "=== 3. Internal Chat (With Auth) ==="
curl -X POST "$BASE_URL/test-company/chat/int" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "Customer reports water still flowing after shutoff"}' \
  | jq .

echo ""
echo "=== 4. Rate Limit Test ==="
for i in {1..35}; do
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$BASE_URL/test-company/chat/ext" \
    -H "Content-Type: application/json" \
    -d '{"message": "test"}')
  echo "Request $i: HTTP $STATUS"
done

echo ""
echo "=== 5. Export ==="
curl "$BASE_URL/test-company/export?format=json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  | jq .
```

### 9.3 Widget Test

```html
<!-- test_widget.html -->
<!DOCTYPE html>
<html>
<head>
  <title>Widget Test</title>
</head>
<body>
  <h1>Test Page</h1>
  <p>The chat widget should appear in the bottom-right corner.</p>

  <script src="http://localhost:8080/api/v1/agent/test-company/widget.js"></script>
</body>
</html>
```

---

## 10. Implementation Sequence

### Phase 1: Database & Parser (2-3 days)
1. [ ] Create database migrations for all tables
2. [ ] Implement KB.MD parser
3. [ ] Implement POLICY.MD parser
4. [ ] Implement validation
5. [ ] Unit tests for parser

### Phase 2: Core Services (3-4 days)
6. [ ] Implement ConfigLoader with audience support
7. [ ] Implement MemorySessionStore (external)
8. [ ] Implement SQLiteSessionStore (internal)
9. [ ] Implement RateLimiter
10. [ ] Implement PolicyEngine
11. [ ] Implement Classification service
12. [ ] Implement Response generation
13. [ ] Unit tests for services

### Phase 3: API Layer (2-3 days)
14. [ ] Implement onboard handler
15. [ ] Implement external chat handler
16. [ ] Implement internal chat handler
17. [ ] Implement import handler
18. [ ] Implement export handler
19. [ ] Implement widget.js handler
20. [ ] Register routes with auth middleware
21. [ ] API tests

### Phase 4: Frontend (2-3 days)
22. [ ] Create AgentChatWidget component
23. [ ] Create AgentChat page (internal)
24. [ ] Add navigation for internal chat
25. [ ] Build widget.js bundle
26. [ ] Widget integration tests

### Phase 5: Testing & Polish (2 days)
27. [ ] End-to-end testing
28. [ ] Rate limiting tests
29. [ ] Session persistence tests
30. [ ] Manual testing with sample tenants
31. [ ] Documentation updates

**Total Estimated Time: 11-15 days**

---

## Summary of Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Dual audience | External + Internal | Different needs, content, auth |
| External sessions | In-memory | Anonymous, no persistence needed |
| Internal sessions | SQLite | Employees need history |
| External auth | None | Customer-facing widget |
| Internal auth | Memos user | Employee access control |
| Admin auth | Memos admin role | Existing system |
| LLM model | Single `LLM_MODEL` | Simplicity |
| Rate limiting | Per tenant, per audience | Prevent abuse |
| Widget | Embedded JS | Easy integration |

---

**Document Version:** 4.0-IMP-2
**Created:** 2026-01-18

*Ready for implementation.*
