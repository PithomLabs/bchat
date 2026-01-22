# AI Chat Agent: CLI to API Transformation Design v2

**Version:** 2.0
**Previous Version:** [DOCS_CHAT_DESIGN.MD](./DOCS_CHAT_DESIGN.MD)
**Date:** 2026-01-18

---

## Revision Summary

| Change | v1 Approach | v2 Approach |
|--------|-------------|-------------|
| Keywords | Hardcoded lists | LLM-based semantic classification |
| Configuration | YAML files | SQLite JSON columns with admin API |
| Policy Rules | Static text | Intent taxonomy + example-based learning |
| Workflow | Implicit | Explicit state machine |

---

## Part 1: Revised Workflow Architecture

### 1.1 High-Level Message Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER MESSAGE                                    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  STAGE 1: CONTEXT LOADING (5-10ms)                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                          │
│  │ Load Tenant │  │Load Session │  │ Load Intents│                          │
│  │   Config    │  │   State     │  │  Taxonomy   │                          │
│  └─────────────┘  └─────────────┘  └─────────────┘                          │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  STAGE 2: INTENT CLASSIFICATION (Single LLM Call, 200-500ms)                │
│  ┌──────────────────────────────────────────────────────────────────┐       │
│  │  LLM classifies message against tenant's intent taxonomy:        │       │
│  │  • Primary Intent (emergency_water, routine_inquiry, escalation) │       │
│  │  • Urgency Level (1-5 scale with reasoning)                      │       │
│  │  • Service Match (which service category)                        │       │
│  │  • Location Detection (city/county if mentioned)                 │       │
│  │  • Customer Data (name/phone if provided)                        │       │
│  │  • Sentiment (frustrated, neutral, satisfied)                    │       │
│  └──────────────────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  STAGE 3: POLICY ENGINE (Deterministic, <5ms)                               │
│  ┌──────────────────────────────────────────────────────────────────┐       │
│  │  Rules evaluated in priority order:                              │       │
│  │  1. Escalation Check → If true, route to escalation handler      │       │
│  │  2. Coverage Check   → If out-of-area, polite rejection          │       │
│  │  3. Emergency Check  → If urgent >= 4, emergency flow            │       │
│  │  4. Service Match    → Route to appropriate service flow         │       │
│  │  5. Default          → Standard conversation flow                │       │
│  └──────────────────────────────────────────────────────────────────┘       │
│                                    │                                         │
│            ┌───────────────────────┼───────────────────────┐                │
│            ▼                       ▼                       ▼                │
│     ┌─────────────┐         ┌─────────────┐         ┌─────────────┐        │
│     │  EMERGENCY  │         │  STANDARD   │         │ ESCALATION  │        │
│     │    FLOW     │         │    FLOW     │         │    FLOW     │        │
│     └─────────────┘         └─────────────┘         └─────────────┘        │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  STAGE 4: RESPONSE GENERATION (Single LLM Call, 300-800ms)                  │
│  ┌──────────────────────────────────────────────────────────────────┐       │
│  │  Context-aware prompt assembled from:                            │       │
│  │  • Tenant identity & tone                                        │       │
│  │  • Relevant KB section (based on detected service)               │       │
│  │  • Phase-specific instructions                                   │       │
│  │  • Safety protocol (if emergency)                                │       │
│  │  • Known customer data (to avoid re-asking)                      │       │
│  │  • Conversation history (last 5 turns)                           │       │
│  └──────────────────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  STAGE 5: STATE UPDATE & RESPONSE (5-10ms)                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                          │
│  │Update Session│  │Log Analytics│  │Return Response                        │
│  │   State     │  │   Event     │  │ + Metadata  │                          │
│  └─────────────┘  └─────────────┘  └─────────────┘                          │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Conversation State Machine

```
                    ┌─────────────┐
                    │   START     │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
            ┌───────│   TRIAGE    │───────┐
            │       └──────┬──────┘       │
            │              │              │
     [out-of-area]    [in-area]     [escalation]
            │              │              │
            ▼              ▼              ▼
     ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
     │  REJECTION  │ │  HANDSHAKE  │ │ ESCALATION  │
     └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
            │              │              │
            ▼              │              ▼
     ┌─────────────┐       │       ┌─────────────┐
     │    END      │       │       │    END      │
     └─────────────┘       │       └─────────────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
       [emergency]    [standard]   [question]
              │            │            │
              ▼            ▼            ▼
       ┌───────────┐ ┌───────────┐ ┌───────────┐
       │ EMERGENCY │ │ BOOKING   │ │   INFO    │
       │  DISPATCH │ │   FLOW    │ │   FLOW    │
       └─────┬─────┘ └─────┬─────┘ └─────┬─────┘
             │             │             │
             └─────────────┼─────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ CONCLUSION  │◄────────┐
                    └──────┬──────┘         │
                           │          [follow-up]
                    [confirmed]             │
                           │                │
                           ▼                │
                    ┌─────────────┐         │
                    │    END      │─────────┘
                    └─────────────┘
```

### 1.3 Two-LLM-Call Architecture

**Why only 2 LLM calls per turn?**

| Call | Purpose | Model | Latency Target |
|------|---------|-------|----------------|
| **Classification** | Intent + extraction | Fast (gpt-4o-mini) | <500ms |
| **Response** | Generate reply | Quality (gpt-4o) | <800ms |

**Total target latency: <1.5 seconds**

The classification call replaces ALL keyword matching:
- No emergency keyword lists
- No escalation keyword lists
- No satisfaction keyword lists
- No location regex patterns

---

## Part 2: Eliminating Brittle Keywords

### 2.1 The Problem with Keywords

**Current approach (brittle):**
```go
escalationKeywords := []string{
    "lawyer", "sue", "manager", "complain", "BBB"
}
for _, kw := range escalationKeywords {
    if strings.Contains(input, kw) {
        isEscalation = true
    }
}
```

**Why this fails at scale:**
- "My lawyer neighbor recommended you" → False positive
- "I want to speak with someone in charge" → False negative (no keyword match)
- Different verticals need different keywords
- Different companies have different escalation thresholds
- Maintaining keyword lists across 100+ tenants is impossible

### 2.2 The Solution: Intent Taxonomy + LLM Classification

Replace keyword lists with **intent definitions** that describe WHAT to detect, not HOW:

```sql
-- Intent taxonomy stored in database
CREATE TABLE intent_taxonomy (
    id INTEGER PRIMARY KEY,
    tenant_id INTEGER REFERENCES agent_tenants(id),
    intent_code TEXT NOT NULL,           -- 'escalation', 'emergency_water'
    intent_category TEXT NOT NULL,       -- 'meta', 'service', 'operational'
    display_name TEXT NOT NULL,          -- 'Customer Escalation'
    description TEXT NOT NULL,           -- LLM-readable description
    examples JSON,                       -- Example phrases for few-shot
    priority INTEGER DEFAULT 0,          -- Higher = evaluated first
    action_type TEXT NOT NULL,           -- 'route', 'flag', 'extract'
    action_config JSON,                  -- Action-specific settings
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Example intent definitions:**

```json
[
  {
    "intent_code": "escalation",
    "intent_category": "meta",
    "display_name": "Customer Escalation",
    "description": "Customer wants to escalate AGAINST the company - file complaint, speak to manager about company's failure, threaten legal action against company, express serious dissatisfaction with company's service. NOT: mentioning third parties' complaints, describing property damage, requesting urgent service.",
    "examples": [
      "I want to speak with your manager about this terrible service",
      "I'm going to file a complaint with the BBB",
      "This is unacceptable, I need to talk to someone in charge",
      "Your company has failed me, I want a supervisor"
    ],
    "counter_examples": [
      "My tenant is complaining about the leak",
      "The situation is getting out of control",
      "I need to speak with someone who can help me today"
    ],
    "priority": 100,
    "action_type": "route",
    "action_config": {
      "route_to": "escalation_handler",
      "confidence_threshold": 0.85
    }
  },
  {
    "intent_code": "emergency_water",
    "intent_category": "service",
    "display_name": "Water Emergency",
    "description": "Active water damage situation requiring immediate response - flooding in progress, burst pipe with water flowing, sewage backup, water actively entering property. Indicators: water described as active/flowing/spreading, urgency language, safety concerns.",
    "examples": [
      "My basement is flooding right now",
      "A pipe burst and water is everywhere",
      "Sewage is backing up into my house",
      "Water is pouring through the ceiling"
    ],
    "counter_examples": [
      "I had a flood last week, need cleanup",
      "There's some water stains on my ceiling",
      "My basement floods sometimes when it rains"
    ],
    "priority": 90,
    "action_type": "route",
    "action_config": {
      "route_to": "emergency_flow",
      "safety_protocol": "water_emergency"
    }
  }
]
```

### 2.3 LLM Classification Prompt (Single Call)

```go
func (s *AgentService) buildClassificationPrompt(tenant *Tenant, message string, history []Message) string {
    // Load active intents for this tenant
    intents := s.store.GetActiveIntents(tenant.ID)

    // Build intent descriptions with examples
    var intentDefs strings.Builder
    for _, intent := range intents {
        intentDefs.WriteString(fmt.Sprintf(`
### %s (%s)
%s

Examples that MATCH this intent:
%s

Examples that DO NOT match:
%s

---
`, intent.DisplayName, intent.IntentCode, intent.Description,
            formatExamples(intent.Examples),
            formatExamples(intent.CounterExamples)))
    }

    return fmt.Sprintf(`You are an intent classifier for %s.

Analyze the customer message and classify it according to the intent taxonomy below.
Consider the conversation history for context.

## INTENT TAXONOMY
%s

## CONVERSATION HISTORY
%s

## CURRENT MESSAGE
"%s"

## OUTPUT FORMAT (JSON only, no markdown)
{
  "primary_intent": {
    "code": "intent_code from taxonomy",
    "confidence": 0.0-1.0,
    "reasoning": "brief explanation"
  },
  "secondary_intents": [
    {"code": "...", "confidence": 0.0-1.0}
  ],
  "urgency": {
    "level": 1-5,
    "reasoning": "why this urgency level"
  },
  "extracted_data": {
    "customer_name": "if mentioned",
    "customer_phone": "if mentioned",
    "location": "city/state if mentioned",
    "issue_type": "water/fire/mold/hvac/plumbing/etc",
    "property_type": "residential/commercial if mentioned"
  },
  "sentiment": "frustrated|neutral|satisfied|distressed",
  "coverage_check": {
    "location_mentioned": "exact text",
    "likely_in_area": true/false/unknown,
    "reasoning": "brief explanation"
  }
}`, tenant.CompanyName, intentDefs.String(), formatHistory(history), message)
}
```

### 2.4 Benefits of Intent-Based Classification

| Aspect | Keyword Approach | Intent Approach |
|--------|------------------|-----------------|
| **Accuracy** | ~70% (false positives/negatives) | ~95% (LLM understands context) |
| **Maintainability** | Edit code, redeploy | Edit database, instant |
| **Multi-tenant** | Copy/paste keyword lists | Inherit + customize intents |
| **New verticals** | Write new keyword lists | Clone taxonomy, adjust descriptions |
| **Edge cases** | Add more keywords (infinite) | Improve description + examples |
| **Localization** | Translate all keywords | LLM handles naturally |

### 2.5 Intent Inheritance for Multi-Tenant

```sql
-- Base intents shared across all tenants
INSERT INTO intent_taxonomy (tenant_id, intent_code, ...) VALUES
(NULL, 'escalation', ...),        -- tenant_id NULL = global
(NULL, 'emergency_generic', ...),
(NULL, 'greeting', ...),
(NULL, 'farewell', ...);

-- Tenant-specific intents (override or extend)
INSERT INTO intent_taxonomy (tenant_id, intent_code, ...) VALUES
(1, 'emergency_water', ...),      -- Bulovas (restoration)
(1, 'emergency_fire', ...),
(2, 'emergency_hvac', ...),       -- CoolAir (HVAC)
(2, 'emergency_no_heat', ...);

-- Query: Get all intents for tenant (global + tenant-specific)
SELECT * FROM intent_taxonomy
WHERE tenant_id IS NULL OR tenant_id = ?
ORDER BY
    CASE WHEN tenant_id IS NOT NULL THEN 0 ELSE 1 END,  -- tenant-specific first
    priority DESC;
```

---

## Part 3: Database-Native Configuration

### 3.1 Schema Design (SQLite)

All configuration stored in SQLite with JSON columns for flexibility:

```sql
-- Core tenant table
CREATE TABLE agent_tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,              -- 'bulovas', 'coolair-hvac'
    company_name TEXT NOT NULL,
    vertical TEXT NOT NULL,                 -- 'restoration', 'hvac', 'plumbing'
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tenant configuration (versioned)
CREATE TABLE agent_tenant_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    version INTEGER NOT NULL DEFAULT 1,
    is_active BOOLEAN DEFAULT TRUE,         -- Only one active version per tenant

    -- Identity & Branding
    identity JSON NOT NULL,
    /*
    {
      "role": "First Responder",
      "tone": "Urgent but calm, empathetic, authoritative on safety",
      "brand_voice": "We take the burden off your shoulders immediately",
      "greeting_style": "warm_professional",
      "constraints": ["Never sound panic-stricken", "Use short sentences in emergencies"]
    }
    */

    -- Contact Information
    contact JSON NOT NULL,
    /*
    {
      "emergency_phone": "(631) 358-5200",
      "secondary_phones": ["855-DRYITNOW"],
      "email": "info@bulovas.com",
      "address": "85-7 Air Park Drive, Ronkonkoma, NY 11779",
      "hours": "24/7"
    }
    */

    -- Conversation Settings
    conversation JSON NOT NULL,
    /*
    {
      "max_turns_before_human": 10,
      "session_timeout_minutes": 30,
      "require_phone_for_booking": true,
      "allow_callback_scheduling": true
    }
    */

    -- Scoring Thresholds (no keywords!)
    scoring JSON NOT NULL,
    /*
    {
      "urgency_thresholds": {
        "emergency": 4,
        "urgent": 3,
        "standard": 1
      },
      "confidence_thresholds": {
        "escalation": 0.85,
        "service_match": 0.70,
        "location_match": 0.75
      }
    }
    */

    -- Integrations
    integrations JSON,
    /*
    {
      "ticket_system": {
        "enabled": true,
        "provider": "memos",
        "webhook_url": "..."
      },
      "crm": {
        "enabled": false
      }
    }
    */

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER REFERENCES user(id)
);

-- Service definitions
CREATE TABLE agent_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    service_code TEXT NOT NULL,             -- 'water_damage', 'ac_repair'
    display_name TEXT NOT NULL,             -- 'Water Damage Restoration'
    description TEXT NOT NULL,              -- For LLM context
    category TEXT NOT NULL,                 -- 'core', 'specialty', 'commercial'
    is_emergency_capable BOOLEAN DEFAULT FALSE,
    typical_response_time TEXT,             -- '30 minutes', '2-4 hours'
    price_range TEXT,                       -- 'Varies by scope', '$150-300'
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INTEGER DEFAULT 0,

    UNIQUE(tenant_id, service_code)
);

-- Service exclusions (what we DON'T do)
CREATE TABLE agent_service_exclusions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    exclusion_code TEXT NOT NULL,           -- 'general_plumbing'
    display_name TEXT NOT NULL,             -- 'General Plumbing'
    description TEXT NOT NULL,              -- LLM-readable explanation
    exception_rule TEXT,                    -- 'unless it caused water damage'
    referral_suggestion TEXT,               -- 'We recommend a licensed plumber'

    UNIQUE(tenant_id, exclusion_code)
);

-- Coverage areas
CREATE TABLE agent_coverage_areas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    area_type TEXT NOT NULL,                -- 'county', 'city', 'zip', 'state'
    area_value TEXT NOT NULL,               -- 'Nassau County', '11779'
    area_state TEXT,                        -- 'NY'
    is_excluded BOOLEAN DEFAULT FALSE,      -- TRUE = we don't serve here
    lat REAL,                               -- For distance calculations
    lng REAL,
    radius_miles INTEGER DEFAULT 50,
    notes TEXT,                             -- 'Primary service area'

    UNIQUE(tenant_id, area_type, area_value)
);

-- Safety protocols
CREATE TABLE agent_safety_protocols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER,                      -- NULL = global protocol
    protocol_code TEXT NOT NULL,            -- 'water_emergency', 'gas_leak'
    display_name TEXT NOT NULL,
    trigger_intents JSON NOT NULL,          -- ["emergency_water", "emergency_flood"]
    instructions JSON NOT NULL,             -- Array of safety steps
    /*
    [
      {"priority": 1, "instruction": "Turn off main water valve if safe to do so"},
      {"priority": 2, "instruction": "Turn off electricity to affected areas"},
      {"priority": 3, "instruction": "Move away from standing water near outlets"}
    ]
    */
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(COALESCE(tenant_id, 0), protocol_code)
);

-- Knowledge base sections (structured, not one big blob)
CREATE TABLE agent_kb_sections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    section_code TEXT NOT NULL,             -- 'company_overview', 'water_damage_faq'
    section_title TEXT NOT NULL,
    content TEXT NOT NULL,                  -- Markdown content
    trigger_services JSON,                  -- ["water_damage"] - when to include
    trigger_intents JSON,                   -- ["emergency_water"] - when to include
    always_include BOOLEAN DEFAULT FALSE,   -- TRUE = always in prompt
    sort_order INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, section_code)
);

-- Prompt templates (versioned)
CREATE TABLE agent_prompt_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER,                      -- NULL = global template
    template_code TEXT NOT NULL,            -- 'classification_v1', 'response_emergency_v1'
    template_type TEXT NOT NULL,            -- 'classification', 'response', 'safety'
    template_content TEXT NOT NULL,         -- Template with {{placeholders}}
    is_active BOOLEAN DEFAULT TRUE,
    version INTEGER DEFAULT 1,

    UNIQUE(COALESCE(tenant_id, 0), template_code, version)
);

-- Chat sessions
CREATE TABLE agent_sessions (
    id TEXT PRIMARY KEY,                    -- UUID
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    user_id INTEGER REFERENCES user(id),

    -- Conversation state
    phase TEXT DEFAULT 'triage',            -- 'triage', 'handshake', 'conclusion'
    urgency_level INTEGER DEFAULT 1,
    detected_intent TEXT,
    detected_service TEXT,
    coverage_status TEXT DEFAULT 'unknown', -- 'in', 'out', 'unknown'

    -- Extracted customer data
    customer_data JSON DEFAULT '{}',
    /*
    {
      "name": "John Smith",
      "phone": "555-1234",
      "location": "Ronkonkoma, NY",
      "property_type": "residential"
    }
    */

    -- History
    message_history JSON DEFAULT '[]',
    turn_count INTEGER DEFAULT 0,

    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    completion_reason TEXT,                 -- 'resolved', 'escalated', 'timeout'

    -- Analytics
    total_latency_ms INTEGER DEFAULT 0,
    llm_calls INTEGER DEFAULT 0
);

-- Analytics events
CREATE TABLE agent_analytics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT REFERENCES agent_sessions(id),
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    event_type TEXT NOT NULL,               -- 'message', 'intent_detected', 'escalation'
    event_data JSON,
    latency_ms INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_sessions_tenant ON agent_sessions(tenant_id);
CREATE INDEX idx_sessions_updated ON agent_sessions(updated_at);
CREATE INDEX idx_intents_tenant ON intent_taxonomy(tenant_id);
CREATE INDEX idx_analytics_tenant_date ON agent_analytics(tenant_id, created_at);
```

### 3.2 Configuration Loading (Cached)

```go
// config_loader.go

type TenantConfig struct {
    Tenant          *Tenant
    Identity        IdentityConfig
    Contact         ContactConfig
    Conversation    ConversationConfig
    Scoring         ScoringConfig
    Services        []Service
    Exclusions      []ServiceExclusion
    CoverageAreas   []CoverageArea
    Intents         []Intent
    SafetyProtocols map[string]SafetyProtocol
    KBSections      map[string]KBSection
    PromptTemplates map[string]string
}

type ConfigLoader struct {
    store     *store.Store
    cache     *lru.Cache           // In-memory LRU cache
    cacheTTL  time.Duration
}

func NewConfigLoader(store *store.Store) *ConfigLoader {
    cache, _ := lru.New(100)       // Cache up to 100 tenants
    return &ConfigLoader{
        store:    store,
        cache:    cache,
        cacheTTL: 5 * time.Minute, // Refresh every 5 minutes
    }
}

func (l *ConfigLoader) GetTenantConfig(tenantSlug string) (*TenantConfig, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("tenant:%s", tenantSlug)
    if cached, ok := l.cache.Get(cacheKey); ok {
        entry := cached.(*cacheEntry)
        if time.Since(entry.loadedAt) < l.cacheTTL {
            return entry.config, nil
        }
    }

    // Load from database
    config, err := l.loadFromDB(tenantSlug)
    if err != nil {
        return nil, err
    }

    // Cache it
    l.cache.Add(cacheKey, &cacheEntry{
        config:   config,
        loadedAt: time.Now(),
    })

    return config, nil
}

func (l *ConfigLoader) InvalidateCache(tenantSlug string) {
    l.cache.Remove(fmt.Sprintf("tenant:%s", tenantSlug))
}
```

### 3.3 Admin API for Configuration Management

```go
// tenant_admin_service.go

// Tenant CRUD
func (s *APIV1Service) CreateTenant(c echo.Context) error
func (s *APIV1Service) GetTenant(c echo.Context) error
func (s *APIV1Service) UpdateTenant(c echo.Context) error
func (s *APIV1Service) DeleteTenant(c echo.Context) error
func (s *APIV1Service) ListTenants(c echo.Context) error

// Configuration versioning
func (s *APIV1Service) GetTenantConfig(c echo.Context) error
func (s *APIV1Service) UpdateTenantConfig(c echo.Context) error    // Creates new version
func (s *APIV1Service) GetConfigVersions(c echo.Context) error
func (s *APIV1Service) RollbackConfig(c echo.Context) error        // Activate old version

// Intent management
func (s *APIV1Service) ListIntents(c echo.Context) error
func (s *APIV1Service) CreateIntent(c echo.Context) error
func (s *APIV1Service) UpdateIntent(c echo.Context) error
func (s *APIV1Service) DeleteIntent(c echo.Context) error
func (s *APIV1Service) TestIntent(c echo.Context) error            // Test classification

// Service management
func (s *APIV1Service) ListServices(c echo.Context) error
func (s *APIV1Service) CreateService(c echo.Context) error
func (s *APIV1Service) UpdateService(c echo.Context) error

// Coverage areas
func (s *APIV1Service) ListCoverageAreas(c echo.Context) error
func (s *APIV1Service) UpdateCoverageAreas(c echo.Context) error   // Bulk update

// KB sections
func (s *APIV1Service) ListKBSections(c echo.Context) error
func (s *APIV1Service) UpdateKBSection(c echo.Context) error

// Safety protocols
func (s *APIV1Service) ListSafetyProtocols(c echo.Context) error
func (s *APIV1Service) UpdateSafetyProtocol(c echo.Context) error
```

**API Endpoints:**

```http
# Tenant Management
GET    /api/v1/admin/tenants
POST   /api/v1/admin/tenants
GET    /api/v1/admin/tenants/:slug
PUT    /api/v1/admin/tenants/:slug
DELETE /api/v1/admin/tenants/:slug

# Configuration
GET    /api/v1/admin/tenants/:slug/config
PUT    /api/v1/admin/tenants/:slug/config
GET    /api/v1/admin/tenants/:slug/config/versions
POST   /api/v1/admin/tenants/:slug/config/rollback/:version

# Intents
GET    /api/v1/admin/tenants/:slug/intents
POST   /api/v1/admin/tenants/:slug/intents
PUT    /api/v1/admin/tenants/:slug/intents/:code
DELETE /api/v1/admin/tenants/:slug/intents/:code
POST   /api/v1/admin/tenants/:slug/intents/:code/test

# Services
GET    /api/v1/admin/tenants/:slug/services
POST   /api/v1/admin/tenants/:slug/services
PUT    /api/v1/admin/tenants/:slug/services/:code

# Coverage
GET    /api/v1/admin/tenants/:slug/coverage
PUT    /api/v1/admin/tenants/:slug/coverage

# KB
GET    /api/v1/admin/tenants/:slug/kb
PUT    /api/v1/admin/tenants/:slug/kb/:section

# Safety Protocols
GET    /api/v1/admin/tenants/:slug/safety
PUT    /api/v1/admin/tenants/:slug/safety/:code

# Analytics
GET    /api/v1/admin/tenants/:slug/analytics/summary
GET    /api/v1/admin/tenants/:slug/analytics/sessions
GET    /api/v1/admin/tenants/:slug/analytics/intents
```

---

## Part 4: Optimized Policy Engine

### 4.1 No More Keywords - Pure LLM + Rules

The policy engine now operates on LLM-classified intents, not raw text:

```go
// policy_engine.go

type ClassificationResult struct {
    PrimaryIntent   IntentMatch        `json:"primary_intent"`
    SecondaryIntents []IntentMatch     `json:"secondary_intents"`
    Urgency         UrgencyResult      `json:"urgency"`
    ExtractedData   ExtractedData      `json:"extracted_data"`
    Sentiment       string             `json:"sentiment"`
    CoverageCheck   CoverageCheckResult `json:"coverage_check"`
}

type PolicyDecision struct {
    Action          string             // 'emergency_flow', 'standard_flow', 'escalation', 'rejection'
    Phase           string             // 'triage', 'handshake', 'conclusion'
    SafetyProtocol  *SafetyProtocol    // If emergency, which safety protocol
    ResponseContext ResponseContext     // Context for response generation
}

func (e *PolicyEngine) Evaluate(
    config *TenantConfig,
    session *Session,
    classification *ClassificationResult,
) *PolicyDecision {

    // Rule 1: Escalation (highest priority)
    if e.isEscalation(config, classification) {
        return &PolicyDecision{
            Action: "escalation",
            Phase:  "escalation",
            ResponseContext: ResponseContext{
                EscalationPhone: config.Contact.EscalationPhone,
            },
        }
    }

    // Rule 2: Out of coverage area
    if e.isOutOfArea(config, classification, session) {
        return &PolicyDecision{
            Action: "rejection",
            Phase:  "rejection",
            ResponseContext: ResponseContext{
                RejectionReason: "out_of_area",
                DetectedLocation: classification.CoverageCheck.LocationMentioned,
            },
        }
    }

    // Rule 3: Service exclusion
    if exclusion := e.matchesExclusion(config, classification); exclusion != nil {
        return &PolicyDecision{
            Action: "rejection",
            Phase:  "rejection",
            ResponseContext: ResponseContext{
                RejectionReason: "service_exclusion",
                Exclusion: exclusion,
            },
        }
    }

    // Rule 4: Emergency
    if e.isEmergency(config, classification) {
        protocol := e.getSafetyProtocol(config, classification)
        return &PolicyDecision{
            Action:         "emergency_flow",
            Phase:          e.determinePhase(session, "emergency"),
            SafetyProtocol: protocol,
            ResponseContext: ResponseContext{
                EmergencyPhone: config.Contact.EmergencyPhone,
                IsEmergency:    true,
            },
        }
    }

    // Rule 5: Standard service inquiry
    return &PolicyDecision{
        Action: "standard_flow",
        Phase:  e.determinePhase(session, "standard"),
        ResponseContext: ResponseContext{
            MatchedService: e.matchService(config, classification),
        },
    }
}

func (e *PolicyEngine) isEscalation(config *TenantConfig, c *ClassificationResult) bool {
    // Check if primary intent is escalation with sufficient confidence
    if c.PrimaryIntent.Code == "escalation" {
        threshold := config.Scoring.ConfidenceThresholds.Escalation
        return c.PrimaryIntent.Confidence >= threshold
    }
    return false
}

func (e *PolicyEngine) isEmergency(config *TenantConfig, c *ClassificationResult) bool {
    // Check urgency level against threshold
    emergencyThreshold := config.Scoring.UrgencyThresholds.Emergency
    return c.Urgency.Level >= emergencyThreshold
}

func (e *PolicyEngine) isOutOfArea(config *TenantConfig, c *ClassificationResult, session *Session) bool {
    // If location detected and classified as out-of-area
    if c.CoverageCheck.LocationMentioned != "" {
        if !c.CoverageCheck.LikelyInArea {
            // Verify against our coverage database
            return !e.verifyInCoverage(config, c.CoverageCheck.LocationMentioned)
        }
    }

    // Check sticky coverage from session
    if session.CoverageStatus == "out" {
        return true
    }

    return false
}
```

### 4.2 Confidence-Based Decision Making

All decisions use confidence thresholds stored in database:

```json
{
  "scoring": {
    "urgency_thresholds": {
      "emergency": 4,
      "urgent": 3,
      "standard": 1
    },
    "confidence_thresholds": {
      "escalation": 0.85,
      "service_match": 0.70,
      "location_match": 0.75,
      "intent_primary": 0.60
    }
  }
}
```

**Business owners can tune these thresholds via admin UI:**
- Lower escalation threshold → More sensitive to complaints
- Higher emergency threshold → Fewer false emergency dispatches
- Adjust per-vertical based on business needs

---

## Part 5: Response Generation

### 5.1 Context-Aware Prompt Assembly

```go
// response_generator.go

func (g *ResponseGenerator) GenerateResponse(
    config *TenantConfig,
    session *Session,
    classification *ClassificationResult,
    decision *PolicyDecision,
) (string, error) {

    // 1. Get base template for this flow type
    templateCode := fmt.Sprintf("response_%s_v1", decision.Action)
    template := config.PromptTemplates[templateCode]

    // 2. Build dynamic context
    context := g.buildContext(config, session, classification, decision)

    // 3. Assemble prompt
    prompt := g.assemblePrompt(template, context)

    // 4. Call LLM
    response, err := g.llmClient.Generate(session.Context(), prompt.System, prompt.User)
    if err != nil {
        return "", err
    }

    return response, nil
}

func (g *ResponseGenerator) buildContext(
    config *TenantConfig,
    session *Session,
    classification *ClassificationResult,
    decision *PolicyDecision,
) *ResponseContext {

    ctx := &ResponseContext{
        // Identity
        CompanyName:  config.Tenant.CompanyName,
        Role:         config.Identity.Role,
        Tone:         config.Identity.Tone,
        BrandVoice:   config.Identity.BrandVoice,

        // Contact
        EmergencyPhone: config.Contact.EmergencyPhone,

        // Session
        Phase:          decision.Phase,
        CustomerData:   session.CustomerData,
        TurnCount:      session.TurnCount,

        // Classification
        DetectedIntent:  classification.PrimaryIntent.Code,
        DetectedService: decision.ResponseContext.MatchedService,
        Urgency:         classification.Urgency.Level,
        Sentiment:       classification.Sentiment,
    }

    // Add relevant KB sections based on detected service/intent
    ctx.KBContent = g.selectKBSections(config, classification, decision)

    // Add safety protocol if emergency
    if decision.SafetyProtocol != nil {
        ctx.SafetyInstructions = decision.SafetyProtocol.Instructions
    }

    // Add conversation history (last N turns)
    ctx.RecentHistory = g.formatRecentHistory(session, 5)

    // Add known customer data to avoid re-asking
    ctx.KnownDataNote = g.formatKnownData(session)

    return ctx
}

func (g *ResponseGenerator) selectKBSections(
    config *TenantConfig,
    classification *ClassificationResult,
    decision *PolicyDecision,
) string {

    var sections []string

    for _, kb := range config.KBSections {
        include := false

        // Always include sections marked as always_include
        if kb.AlwaysInclude {
            include = true
        }

        // Include if triggered by detected service
        if decision.ResponseContext.MatchedService != "" {
            for _, svc := range kb.TriggerServices {
                if svc == decision.ResponseContext.MatchedService {
                    include = true
                    break
                }
            }
        }

        // Include if triggered by detected intent
        for _, intent := range kb.TriggerIntents {
            if intent == classification.PrimaryIntent.Code {
                include = true
                break
            }
        }

        if include {
            sections = append(sections, fmt.Sprintf("## %s\n%s", kb.Title, kb.Content))
        }
    }

    return strings.Join(sections, "\n\n")
}
```

### 5.2 Response Template Example (Stored in DB)

```
template_code: response_emergency_flow_v1
template_type: response
template_content:
---
You are the AI Agent for {{.CompanyName}}.

ROLE: {{.Role}}
TONE: {{.Tone}}
BRAND VOICE: "{{.BrandVoice}}"

=== CURRENT SITUATION ===
This is an EMERGENCY situation.
Urgency Level: {{.Urgency}}/5
Detected Issue: {{.DetectedService}}
Customer Sentiment: {{.Sentiment}}

=== SAFETY PROTOCOL (MUST INCLUDE) ===
{{range .SafetyInstructions}}
- {{.Instruction}}
{{end}}

=== EMERGENCY CONTACT ===
Phone: {{.EmergencyPhone}}

=== RELEVANT INFORMATION ===
{{.KBContent}}

=== KNOWN CUSTOMER DATA ===
{{.KnownDataNote}}
(If data is shown above, DO NOT ask for it again)

=== CONVERSATION HISTORY ===
{{.RecentHistory}}

=== PHASE: {{.Phase}} ===
{{if eq .Phase "triage"}}
- Provide safety instructions FIRST
- Direct to call emergency line
- Confirm their location if unknown
{{else if eq .Phase "handshake"}}
- Safety was provided, focus on information collection
- Get name and callback number
- Reassure help is on the way
{{end}}

=== INSTRUCTIONS ===
1. Acknowledge the emergency with empathy but stay calm
2. Provide the safety protocol above (don't skip any step)
3. Direct them to call {{.EmergencyPhone}}
4. If they have questions, answer briefly then redirect to phone

Generate a response that follows the tone and instructions above.
Output the response text only, no JSON or markdown.
---
```

---

## Part 6: Experience-Focused Design

### 6.1 User Experience (End Customer)

| Goal | Implementation |
|------|----------------|
| **Fast responses** | 2 LLM calls max, <1.5s total latency |
| **No repetition** | Session tracks what's been asked/said |
| **Contextual** | LLM understands full conversation |
| **Safe** | Emergency protocols always delivered |
| **Respectful** | Polite out-of-area rejections |

### 6.2 Business Owner Experience

| Goal | Implementation |
|------|----------------|
| **Easy setup** | Clone from vertical template |
| **No code changes** | All config via admin API/UI |
| **Instant updates** | Change KB, see results immediately |
| **A/B testing** | Version configs, compare metrics |
| **Visibility** | Analytics dashboard |

**Admin UI Wireframe:**

```
┌─────────────────────────────────────────────────────────────────┐
│  Tenant Dashboard: Bulovas Restorations                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  [Identity] [Services] [Coverage] [Intents] [KB] [Analytics]    │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Active Configuration: v3 (deployed 2 hours ago)         │   │
│  │ [Edit Config] [View History] [Test Chatbot]             │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  Quick Stats (Last 24h)                                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │ 147      │ │ 23       │ │ 3        │ │ 1.2s     │          │
│  │ Sessions │ │ Emergency│ │ Escalate │ │ Avg Resp │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
│                                                                 │
│  Intent Performance                                             │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Intent              │ Count │ Avg Confidence │ Accuracy │   │
│  │ emergency_water     │ 23    │ 0.92          │ 96%      │   │
│  │ routine_inquiry     │ 89    │ 0.87          │ 94%      │   │
│  │ escalation          │ 3     │ 0.89          │ 100%     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 6.3 Operations Experience

| Goal | Implementation |
|------|----------------|
| **Monitoring** | Real-time session metrics |
| **Debugging** | Full conversation logs with LLM I/O |
| **Alerting** | Spike in escalations, low confidence |
| **Recovery** | One-click config rollback |

### 6.4 Security Considerations

| Concern | Mitigation |
|---------|------------|
| **API Key exposure** | Keys in env vars, not database |
| **Tenant isolation** | All queries scoped by tenant_id |
| **PII handling** | Customer data encrypted at rest |
| **Prompt injection** | Input sanitization, output validation |
| **Rate limiting** | Per-tenant, per-IP limits |
| **Audit trail** | All config changes logged with user |

```go
// Security middleware
func TenantIsolation() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Extract tenant from path/header
            tenantSlug := c.Param("tenant")

            // Verify user has access to this tenant
            user := getCurrentUser(c)
            if !user.HasTenantAccess(tenantSlug) {
                return echo.NewHTTPError(http.StatusForbidden, "Access denied")
            }

            // Set tenant in context for all downstream queries
            c.Set("tenant_slug", tenantSlug)

            return next(c)
        }
    }
}
```

---

## Part 7: Implementation Sequence

### Phase 1: Core Infrastructure (Week 1)

- [ ] Create database schema (all tables above)
- [ ] Implement ConfigLoader with caching
- [ ] Port classification logic (LLM-based, no keywords)
- [ ] Port policy engine (rule-based on classification)
- [ ] Update `/api/v1/chat` endpoint

### Phase 2: Admin API (Week 2)

- [ ] Tenant CRUD endpoints
- [ ] Config version management
- [ ] Intent management endpoints
- [ ] Service/Coverage endpoints
- [ ] KB section endpoints

### Phase 3: Response Generation (Week 3)

- [ ] Template-based prompt assembly
- [ ] Context-aware KB selection
- [ ] Safety protocol injection
- [ ] Session state management

### Phase 4: Multi-Tenant & Verticals (Week 4)

- [ ] Intent inheritance (global → tenant)
- [ ] Vertical templates (restoration, HVAC, plumbing)
- [ ] Tenant cloning functionality
- [ ] Onboarding wizard

### Phase 5: Analytics & Polish (Week 5)

- [ ] Event logging
- [ ] Analytics queries
- [ ] Dashboard UI
- [ ] Intent testing tool

---

## Appendix A: Vertical Templates

### Restoration Template

```json
{
  "vertical": "restoration",
  "identity": {
    "role": "First Responder",
    "tone": "Urgent but calm, empathetic, authoritative on safety"
  },
  "default_intents": [
    "emergency_water", "emergency_fire", "emergency_mold",
    "routine_inquiry", "booking_request", "escalation"
  ],
  "default_services": [
    {"code": "water_damage", "emergency": true},
    {"code": "fire_damage", "emergency": true},
    {"code": "mold_remediation", "emergency": false}
  ],
  "default_exclusions": [
    {"code": "general_plumbing", "exception": "unless caused water damage"},
    {"code": "general_construction"}
  ],
  "default_safety_protocols": ["water_emergency", "fire_emergency", "gas_leak"]
}
```

### HVAC Template

```json
{
  "vertical": "hvac",
  "identity": {
    "role": "HVAC Support Specialist",
    "tone": "Professional and reassuring"
  },
  "default_intents": [
    "emergency_no_heat", "emergency_no_ac", "emergency_gas",
    "routine_maintenance", "installation_inquiry", "escalation"
  ],
  "default_services": [
    {"code": "heating_repair", "emergency": true},
    {"code": "ac_repair", "emergency": true},
    {"code": "furnace_install", "emergency": false},
    {"code": "duct_cleaning", "emergency": false}
  ],
  "default_exclusions": [
    {"code": "plumbing"},
    {"code": "electrical", "exception": "unless HVAC-related"}
  ],
  "default_safety_protocols": ["gas_leak", "no_heat_cold_weather", "carbon_monoxide"]
}
```

---

## Appendix B: Migration from v1 Keywords

### Before (v1 - Keyword Lists)

```go
escalationKeywords := []string{
    "lawyer", "sue", "manager", "complain", "BBB",
    "speak to a manager", "talk to a supervisor"
}
```

### After (v2 - Intent Definition)

```sql
INSERT INTO intent_taxonomy (
    tenant_id, intent_code, display_name, description, examples, counter_examples
) VALUES (
    NULL,  -- Global intent
    'escalation',
    'Customer Escalation',
    'Customer wants to escalate AGAINST the company...',
    '["I want to speak with your manager", "I''m filing a complaint"]',
    '["My tenant is complaining", "The situation is escalating"]'
);
```

**Why this is better:**
1. LLM understands "I need to speak with someone in charge" (no keyword match)
2. LLM rejects "The leak is escalating" (false positive in v1)
3. Business owner can improve by adding examples, not code changes
4. Works across languages without translation

---

**Document Version:** 2.0
**Created:** 2026-01-18
**Author:** Claude Code Assistant

---

*This design maximizes flexibility, eliminates brittle keyword dependencies, and stores all configuration in SQLite for dynamic updates without code changes.*
