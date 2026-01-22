# AI Chat Agent Implementation Specification

**Version:** 4.0-IMP
**Based On:** [DOCS_CHAT_DESIGN_4.MD](./DOCS_CHAT_DESIGN_4.MD)
**Date:** 2026-01-18

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Database Schema](#2-database-schema)
3. [File Parser](#3-file-parser)
4. [API Endpoints](#4-api-endpoints)
5. [Core Services](#5-core-services)
6. [LLM Integration](#6-llm-integration)
7. [Testing Plan](#7-testing-plan)
8. [File Structure](#8-file-structure)
9. [Implementation Sequence](#9-implementation-sequence)

---

## 1. Architecture Overview

### 1.1 System Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              MEMOS APPLICATION                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                         HTTP LAYER (Echo)                            │    │
│  │                                                                      │    │
│  │  /api/v1/agent/onboard          POST   Onboard new tenant           │    │
│  │  /api/v1/agent/:slug/chat       POST   Chat with agent              │    │
│  │  /api/v1/agent/:slug/import     POST   Re-import files              │    │
│  │  /api/v1/agent/:slug/export     GET    Export to files              │    │
│  │  /api/v1/agent/:slug/config     GET    Get config                   │    │
│  │  /api/v1/agent/:slug/config     PUT    Update config                │    │
│  │  /api/v1/agent/:slug/intents    GET    List intents                 │    │
│  │  /api/v1/agent/:slug/intents    PUT    Update intent                │    │
│  │                                                                      │    │
│  └──────────────────────────────────┬──────────────────────────────────┘    │
│                                     │                                        │
│  ┌──────────────────────────────────▼──────────────────────────────────┐    │
│  │                        SERVICE LAYER                                 │    │
│  │                                                                      │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │    │
│  │  │   Parser     │  │    Agent     │  │   Config     │              │    │
│  │  │   Service    │  │   Service    │  │   Loader     │              │    │
│  │  │              │  │              │  │              │              │    │
│  │  │ • ParseKB()  │  │ • Chat()     │  │ • Load()     │              │    │
│  │  │ • ParsePol() │  │ • Classify() │  │ • Cache()    │              │    │
│  │  │ • Validate() │  │ • Respond()  │  │ • Invalidate │              │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘              │    │
│  │                           │                                          │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │    │
│  │  │   Policy     │  │    LLM       │  │   Session    │              │    │
│  │  │   Engine     │  │   Client     │  │   Store      │              │    │
│  │  │              │  │              │  │              │              │    │
│  │  │ • Evaluate() │  │ • Classify() │  │ • Get()      │              │    │
│  │  │ • Route()    │  │ • Generate() │  │ • Update()   │              │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘              │    │
│  │                                                                      │    │
│  └──────────────────────────────────┬──────────────────────────────────┘    │
│                                     │                                        │
│  ┌──────────────────────────────────▼──────────────────────────────────┐    │
│  │                        DATA LAYER                                    │    │
│  │                                                                      │    │
│  │  ┌──────────────────────────────────────────────────────────────┐   │    │
│  │  │                     SQLite Database                           │   │    │
│  │  │                                                               │   │    │
│  │  │  agent_tenants │ agent_configs │ agent_intents │ agent_*     │   │    │
│  │  └──────────────────────────────────────────────────────────────┘   │    │
│  │                                                                      │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Request Flow: Chat

```
User Message
     │
     ▼
┌─────────────┐
│  HTTP POST  │  /api/v1/agent/:slug/chat
│  Handler    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Config    │  Load tenant config (cached)
│   Loader    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Session   │  Get or create session
│   Store     │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│    LLM      │  CALL 1: Classify intent
│  Classify   │  → Returns: intent, urgency, extracted data
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Policy    │  Evaluate rules (deterministic)
│   Engine    │  → Returns: action, flow, safety protocol
└──────┬──────┘
       │
       ▼
┌─────────────┐
│    LLM      │  CALL 2: Generate response
│  Generate   │  → Returns: response text
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Session   │  Update session state
│   Update    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Return    │  JSON response to user
│   Response  │
└─────────────┘
```

---

## 2. Database Schema

### 2.1 Complete Schema (SQLite)

```sql
-- ============================================================================
-- TENANT TABLES
-- ============================================================================

-- Core tenant table
CREATE TABLE agent_tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    company_name TEXT NOT NULL,
    vertical TEXT,                          -- 'restoration', 'hvac', 'plumbing'
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tenant configuration
CREATE TABLE agent_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER UNIQUE NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,

    -- Identity (from POLICY.MD @identity)
    role TEXT NOT NULL,
    tone TEXT NOT NULL,
    brand_voice TEXT,
    guidelines TEXT,                        -- JSON array of strings

    -- Contact (from KB.MD Contact section)
    emergency_phone TEXT NOT NULL,
    secondary_phones TEXT,                  -- JSON array
    email TEXT,
    address TEXT,

    -- Thresholds (from POLICY.MD @thresholds)
    emergency_urgency_threshold INTEGER DEFAULT 4,
    escalation_confidence_threshold REAL DEFAULT 0.85,
    service_match_confidence_threshold REAL DEFAULT 0.70,

    -- Conversation settings
    max_turns INTEGER DEFAULT 15,
    session_timeout_minutes INTEGER DEFAULT 30,

    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- KNOWLEDGE BASE TABLES (from KB.MD)
-- ============================================================================

-- Services
CREATE TABLE agent_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    code TEXT NOT NULL,                     -- 'water_damage'
    name TEXT NOT NULL,                     -- 'Water Damage Restoration'
    description TEXT,
    is_emergency BOOLEAN DEFAULT FALSE,
    response_time TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INTEGER DEFAULT 0,

    UNIQUE(tenant_id, code)
);

-- Service exclusions
CREATE TABLE agent_exclusions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    exception_rule TEXT,                    -- 'unless it caused water damage'
    referral TEXT,
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, code)
);

-- Coverage areas
CREATE TABLE agent_coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    area_type TEXT NOT NULL,                -- 'county', 'city', 'state', 'zip'
    area_name TEXT NOT NULL,
    state_code TEXT,                        -- 'NY', 'CA'
    is_included BOOLEAN NOT NULL,           -- TRUE = serve, FALSE = excluded

    UNIQUE(tenant_id, area_type, area_name)
);

-- FAQs
CREATE TABLE agent_faqs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INTEGER DEFAULT 0,

    UNIQUE(tenant_id, code)
);

-- Safety protocols
CREATE TABLE agent_safety_protocols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    code TEXT NOT NULL,                     -- 'water_emergency'
    name TEXT NOT NULL,                     -- 'Water Emergency'
    trigger_intents TEXT NOT NULL,          -- JSON: ["emergency_water", "emergency_flood"]
    instructions TEXT NOT NULL,             -- JSON: ["Step 1", "Step 2"]
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, code)
);

-- General KB sections (company overview, additional info)
CREATE TABLE agent_kb_sections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    section_type TEXT DEFAULT 'general',    -- 'overview', 'general', 'additional'
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INTEGER DEFAULT 0,

    UNIQUE(tenant_id, code)
);

-- ============================================================================
-- POLICY TABLES (from POLICY.MD)
-- ============================================================================

-- Intents
CREATE TABLE agent_intents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER REFERENCES agent_tenants(id) ON DELETE CASCADE,  -- NULL = global
    code TEXT NOT NULL,                     -- 'emergency_water'
    name TEXT NOT NULL,                     -- 'Water Emergency'
    category TEXT NOT NULL,                 -- 'emergency', 'standard', 'meta'
    description TEXT NOT NULL,
    examples TEXT,                          -- JSON: ["example 1", "example 2"]
    counter_examples TEXT,                  -- JSON: ["not this"]
    urgency INTEGER,                        -- 1-5 or NULL
    action TEXT NOT NULL,                   -- 'emergency_flow', 'standard_flow'
    confidence_threshold REAL,              -- Override default if set
    is_active BOOLEAN DEFAULT TRUE,
    priority INTEGER DEFAULT 0,             -- Higher = evaluated first

    UNIQUE(COALESCE(tenant_id, 0), code)
);

-- Rules
CREATE TABLE agent_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    priority INTEGER DEFAULT 5,             -- 1 = highest priority
    applies_to TEXT,                        -- 'emergency_*', 'all', specific intent
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, code)
);

-- ============================================================================
-- SESSION TABLES
-- ============================================================================

-- Chat sessions
CREATE TABLE agent_sessions (
    id TEXT PRIMARY KEY,                    -- UUID
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,

    -- State
    phase TEXT DEFAULT 'triage',            -- 'triage', 'handshake', 'emergency', 'conclusion'
    current_intent TEXT,
    urgency_level INTEGER DEFAULT 0,
    coverage_status TEXT DEFAULT 'unknown', -- 'in', 'out', 'unknown'

    -- Extracted data
    customer_name TEXT,
    customer_phone TEXT,
    customer_location TEXT,
    detected_service TEXT,

    -- History
    message_count INTEGER DEFAULT 0,
    messages TEXT DEFAULT '[]',             -- JSON array of messages

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,

    -- Completion
    is_completed BOOLEAN DEFAULT FALSE,
    completion_reason TEXT                  -- 'resolved', 'escalated', 'timeout', 'user_ended'
);

-- ============================================================================
-- SOURCE FILES (for export)
-- ============================================================================

CREATE TABLE agent_source_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    file_type TEXT NOT NULL,                -- 'kb' or 'policy'
    content TEXT NOT NULL,                  -- Original markdown content
    content_hash TEXT NOT NULL,
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(tenant_id, file_type)
);

-- ============================================================================
-- INDEXES
-- ============================================================================

CREATE INDEX idx_tenants_slug ON agent_tenants(slug);
CREATE INDEX idx_tenants_active ON agent_tenants(is_active);
CREATE INDEX idx_services_tenant ON agent_services(tenant_id, is_active);
CREATE INDEX idx_intents_tenant ON agent_intents(tenant_id, is_active);
CREATE INDEX idx_intents_category ON agent_intents(tenant_id, category);
CREATE INDEX idx_sessions_tenant ON agent_sessions(tenant_id);
CREATE INDEX idx_sessions_updated ON agent_sessions(updated_at);
CREATE INDEX idx_coverage_tenant ON agent_coverage(tenant_id, is_included);
```

### 2.2 Data Types Summary

| Table | Key Fields | Notes |
|-------|------------|-------|
| `agent_tenants` | slug (unique), company_name | Core tenant record |
| `agent_configs` | tenant_id (unique FK) | One config per tenant |
| `agent_services` | tenant_id + code (unique) | From KB.MD @service |
| `agent_exclusions` | tenant_id + code (unique) | From KB.MD @exclusion |
| `agent_coverage` | tenant_id + type + name (unique) | From KB.MD @coverage |
| `agent_faqs` | tenant_id + code (unique) | From KB.MD @faq |
| `agent_safety_protocols` | tenant_id + code (unique) | From KB.MD @safety |
| `agent_intents` | tenant_id + code (unique) | From POLICY.MD @intent |
| `agent_rules` | tenant_id + code (unique) | From POLICY.MD @rule |
| `agent_sessions` | id (UUID) | Chat session state |

---

## 3. File Parser

### 3.1 Parser Interface

```go
// parser/parser.go

package parser

// ParsedKB represents parsed KB.MD content
type ParsedKB struct {
    CompanyName     string
    CompanyOverview string
    Contact         ContactInfo
    Services        []Service
    Exclusions      []Exclusion
    CoverageInclude []CoverageArea
    CoverageExclude []CoverageArea
    FAQs            []FAQ
    SafetyProtocols []SafetyProtocol
    AdditionalSections []Section
}

type ContactInfo struct {
    EmergencyPhone  string
    SecondaryPhones []string
    Email           string
    Address         string
}

type Service struct {
    Code        string
    Name        string
    Description string
    IsEmergency bool
    ResponseTime string
}

type Exclusion struct {
    Code          string
    Name          string
    Description   string
    ExceptionRule string
    Referral      string
}

type CoverageArea struct {
    Type  string  // "county", "city", "state"
    Name  string
    State string  // Optional state code
}

type FAQ struct {
    Code     string
    Question string
    Answer   string
}

type SafetyProtocol struct {
    Code         string
    Name         string
    TriggerIntents []string
    Instructions []string
}

// ParsedPolicy represents parsed POLICY.MD content
type ParsedPolicy struct {
    Identity    Identity
    Intents     []Intent
    Rules       []Rule
    Thresholds  Thresholds
}

type Identity struct {
    Role       string
    Tone       string
    BrandVoice string
    Guidelines []string
}

type Intent struct {
    Code                string
    Name                string
    Category            string   // "emergency", "standard", "meta"
    Description         string
    Examples            []string
    CounterExamples     []string
    Urgency             *int     // nil if not applicable
    Action              string
    ConfidenceThreshold *float64 // nil to use default
}

type Rule struct {
    Code        string
    Name        string
    Description string
    Priority    int
    AppliesTo   string  // Pattern like "emergency_*" or specific intent
}

type Thresholds struct {
    EmergencyUrgency        int
    EscalationConfidence    float64
    ServiceMatchConfidence  float64
}

// Parser functions
func ParseKB(content []byte) (*ParsedKB, error)
func ParsePolicy(content []byte) (*ParsedPolicy, error)
func Validate(kb *ParsedKB, policy *ParsedPolicy) []ValidationError

type ValidationError struct {
    File    string  // "KB.MD" or "POLICY.MD"
    Field   string
    Message string
    IsError bool    // false = warning
}
```

### 3.2 Annotation Parsing

```go
// parser/annotations.go

import "regexp"

// Annotation patterns
var (
    // <!-- @service: water_damage, emergency: true -->
    servicePattern = regexp.MustCompile(`<!--\s*@service:\s*([^,>]+)(?:,\s*emergency:\s*(true|false))?\s*-->`)

    // <!-- @exclusion: general_plumbing, exception: "unless X" -->
    exclusionPattern = regexp.MustCompile(`<!--\s*@exclusion:\s*([^,>]+)(?:,\s*exception:\s*"([^"]*)")?\s*-->`)

    // <!-- @coverage: include --> or <!-- @coverage: exclude -->
    coveragePattern = regexp.MustCompile(`<!--\s*@coverage:\s*(include|exclude)\s*-->`)

    // <!-- @faq: response_time -->
    faqPattern = regexp.MustCompile(`<!--\s*@faq:\s*([^>]+)\s*-->`)

    // <!-- @safety: water_emergency, triggers: intent1, intent2 -->
    safetyPattern = regexp.MustCompile(`<!--\s*@safety:\s*([^,>]+)(?:,\s*triggers:\s*([^>]+))?\s*-->`)

    // <!-- @identity -->
    identityPattern = regexp.MustCompile(`<!--\s*@identity\s*-->`)

    // <!-- @intent: code, category: X, urgency: N, action: Y -->
    intentPattern = regexp.MustCompile(`<!--\s*@intent:\s*([^,>]+)(?:,\s*category:\s*([^,>]+))?(?:,\s*urgency:\s*(\d+))?(?:,\s*action:\s*([^,>]+))?(?:,\s*confidence_threshold:\s*([\d.]+))?\s*-->`)

    // <!-- @rule: code, priority: N -->
    rulePattern = regexp.MustCompile(`<!--\s*@rule:\s*([^,>]+)(?:,\s*priority:\s*(\d+))?\s*-->`)

    // <!-- @thresholds -->
    thresholdsPattern = regexp.MustCompile(`<!--\s*@thresholds\s*-->`)
)

// ExtractAnnotation finds and parses an annotation in a text block
func ExtractAnnotation(text string, pattern *regexp.Regexp) map[string]string
```

### 3.3 Markdown Section Parsing

```go
// parser/markdown.go

// Section represents a markdown section (heading + content)
type Section struct {
    Level   int      // 1 = #, 2 = ##, 3 = ###
    Title   string
    Content string
    Annotations map[string]string
    Children []Section
}

// ParseMarkdown converts markdown to section tree
func ParseMarkdown(content []byte) []Section

// FindSection finds a section by title (case-insensitive)
func FindSection(sections []Section, title string) *Section

// FindSectionsByAnnotation finds all sections with a specific annotation
func FindSectionsByAnnotation(sections []Section, annotationType string) []Section

// ExtractTable parses a markdown table into rows
func ExtractTable(content string) []map[string]string

// ExtractList parses a markdown list into items
func ExtractList(content string) []string
```

### 3.4 Validation Rules

```go
// parser/validate.go

func Validate(kb *ParsedKB, policy *ParsedPolicy) []ValidationError {
    var errors []ValidationError

    // Required: Emergency phone
    if kb.Contact.EmergencyPhone == "" {
        errors = append(errors, ValidationError{
            File:    "KB.MD",
            Field:   "Contact.EmergencyPhone",
            Message: "Emergency phone number is required",
            IsError: true,
        })
    }

    // Required: At least one service
    if len(kb.Services) == 0 {
        errors = append(errors, ValidationError{
            File:    "KB.MD",
            Field:   "Services",
            Message: "At least one service must be defined",
            IsError: true,
        })
    }

    // Required: Agent role
    if policy.Identity.Role == "" {
        errors = append(errors, ValidationError{
            File:    "POLICY.MD",
            Field:   "Identity.Role",
            Message: "Agent role is required",
            IsError: true,
        })
    }

    // Warning: Intent without examples
    for _, intent := range policy.Intents {
        if len(intent.Examples) == 0 {
            errors = append(errors, ValidationError{
                File:    "POLICY.MD",
                Field:   fmt.Sprintf("Intent[%s].Examples", intent.Code),
                Message: "Intent has no examples - classification accuracy may be lower",
                IsError: false,
            })
        }
    }

    // Warning: Safety protocol references undefined intent
    for _, safety := range kb.SafetyProtocols {
        for _, trigger := range safety.TriggerIntents {
            found := false
            for _, intent := range policy.Intents {
                if matchPattern(trigger, intent.Code) {
                    found = true
                    break
                }
            }
            if !found {
                errors = append(errors, ValidationError{
                    File:    "KB.MD",
                    Field:   fmt.Sprintf("Safety[%s].Triggers", safety.Code),
                    Message: fmt.Sprintf("Trigger '%s' does not match any defined intent", trigger),
                    IsError: false,
                })
            }
        }
    }

    return errors
}
```

---

## 4. API Endpoints

### 4.1 Endpoint Summary

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/api/v1/agent/onboard` | Create new tenant | Admin |
| POST | `/api/v1/agent/:slug/chat` | Chat with agent | User |
| POST | `/api/v1/agent/:slug/import` | Re-import files | Admin |
| GET | `/api/v1/agent/:slug/export` | Export to files | Admin |
| GET | `/api/v1/agent/:slug/config` | Get configuration | Admin |
| PUT | `/api/v1/agent/:slug/config` | Update configuration | Admin |
| GET | `/api/v1/agent/:slug/intents` | List intents | Admin |
| PUT | `/api/v1/agent/:slug/intents/:code` | Update intent | Admin |
| GET | `/api/v1/agent/:slug/services` | List services | Admin |
| DELETE | `/api/v1/agent/:slug` | Delete tenant | Admin |

### 4.2 Onboard Endpoint

```http
POST /api/v1/agent/onboard
Content-Type: multipart/form-data
```

**Request:**
```
tenant_slug: acme-restoration
vertical: restoration (optional)
kb_file: [file upload - KB.MD]
policy_file: [file upload - POLICY.MD]
```

**Response (Success - 201):**
```json
{
  "success": true,
  "tenant": {
    "id": 1,
    "slug": "acme-restoration",
    "company_name": "ACME Restoration Inc.",
    "vertical": "restoration"
  },
  "parsed": {
    "services_count": 5,
    "intents_count": 12,
    "coverage_areas_count": 25,
    "faqs_count": 8,
    "safety_protocols_count": 3
  },
  "warnings": [
    {
      "file": "POLICY.MD",
      "field": "Intent[booking_request].Examples",
      "message": "Intent has no examples - classification accuracy may be lower"
    }
  ],
  "endpoints": {
    "chat": "/api/v1/agent/acme-restoration/chat",
    "config": "/api/v1/agent/acme-restoration/config"
  }
}
```

**Response (Validation Error - 400):**
```json
{
  "success": false,
  "errors": [
    {
      "file": "KB.MD",
      "field": "Contact.EmergencyPhone",
      "message": "Emergency phone number is required"
    }
  ]
}
```

### 4.3 Chat Endpoint

```http
POST /api/v1/agent/:slug/chat
Content-Type: application/json
```

**Request:**
```json
{
  "session_id": "optional-uuid-to-continue-session",
  "message": "My basement is flooding right now"
}
```

**Response (Success - 200):**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": {
    "role": "assistant",
    "content": "I understand you're dealing with an emergency...",
    "timestamp": "2026-01-18T10:30:00Z"
  },
  "metadata": {
    "intent": {
      "code": "emergency_water",
      "confidence": 0.95
    },
    "urgency": 5,
    "phase": "emergency",
    "coverage": "unknown"
  }
}
```

### 4.4 Import Endpoint

```http
POST /api/v1/agent/:slug/import
Content-Type: multipart/form-data
```

**Request:**
```
kb_file: [file upload - KB.MD] (optional)
policy_file: [file upload - POLICY.MD] (optional)
preview: true/false (default: false)
```

**Response (Preview Mode):**
```json
{
  "preview": true,
  "changes": {
    "services": {
      "added": ["new_service"],
      "updated": ["water_damage"],
      "removed": []
    },
    "intents": {
      "added": [],
      "updated": ["emergency_water"],
      "removed": []
    }
  }
}
```

### 4.5 Export Endpoint

```http
GET /api/v1/agent/:slug/export
Accept: application/zip
```

**Response:** ZIP file containing KB.MD and POLICY.MD

**Alternative (JSON):**
```http
GET /api/v1/agent/:slug/export?format=json
```

```json
{
  "kb": "# Company Name Knowledge Base\n\n...",
  "policy": "# Company Name Agent Policy\n\n..."
}
```

---

## 5. Core Services

### 5.1 Agent Service

```go
// services/agent/agent.go

package agent

type AgentService struct {
    store        *store.Store
    configLoader *ConfigLoader
    llmClient    llm.Client
    sessions     SessionStore
}

type ChatRequest struct {
    SessionID string `json:"session_id"`
    Message   string `json:"message"`
}

type ChatResponse struct {
    SessionID string          `json:"session_id"`
    Message   ResponseMessage `json:"message"`
    Metadata  ChatMetadata    `json:"metadata"`
}

type ResponseMessage struct {
    Role      string    `json:"role"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}

type ChatMetadata struct {
    Intent   IntentMatch `json:"intent"`
    Urgency  int         `json:"urgency"`
    Phase    string      `json:"phase"`
    Coverage string      `json:"coverage"`
}

func (s *AgentService) Chat(ctx context.Context, tenantSlug string, req ChatRequest) (*ChatResponse, error) {
    // 1. Load tenant config
    config, err := s.configLoader.Load(tenantSlug)
    if err != nil {
        return nil, fmt.Errorf("tenant not found: %w", err)
    }

    // 2. Get or create session
    session := s.sessions.GetOrCreate(req.SessionID, config.TenantID)

    // 3. Add user message to history
    session.AddMessage("user", req.Message)

    // 4. Classify intent (LLM Call 1)
    classification, err := s.classify(ctx, config, session, req.Message)
    if err != nil {
        return nil, fmt.Errorf("classification failed: %w", err)
    }

    // 5. Evaluate policy (deterministic)
    decision := s.policyEngine.Evaluate(config, session, classification)

    // 6. Generate response (LLM Call 2)
    response, err := s.generateResponse(ctx, config, session, classification, decision)
    if err != nil {
        return nil, fmt.Errorf("generation failed: %w", err)
    }

    // 7. Update session
    session.AddMessage("assistant", response)
    session.Phase = decision.Phase
    session.CurrentIntent = classification.PrimaryIntent.Code
    session.UrgencyLevel = classification.Urgency.Level
    s.sessions.Save(session)

    // 8. Return response
    return &ChatResponse{
        SessionID: session.ID,
        Message: ResponseMessage{
            Role:      "assistant",
            Content:   response,
            Timestamp: time.Now(),
        },
        Metadata: ChatMetadata{
            Intent:   classification.PrimaryIntent,
            Urgency:  classification.Urgency.Level,
            Phase:    decision.Phase,
            Coverage: classification.Coverage.Status,
        },
    }, nil
}
```

### 5.2 Classification Service

```go
// services/agent/classify.go

type ClassificationResult struct {
    PrimaryIntent IntentMatch     `json:"primary_intent"`
    Urgency       UrgencyResult   `json:"urgency"`
    ExtractedData ExtractedData   `json:"extracted_data"`
    Coverage      CoverageResult  `json:"coverage"`
    Sentiment     string          `json:"sentiment"`
}

type IntentMatch struct {
    Code       string  `json:"code"`
    Confidence float64 `json:"confidence"`
    Reasoning  string  `json:"reasoning"`
}

type UrgencyResult struct {
    Level     int    `json:"level"`
    Reasoning string `json:"reasoning"`
}

type ExtractedData struct {
    CustomerName  string `json:"customer_name,omitempty"`
    CustomerPhone string `json:"customer_phone,omitempty"`
    Location      string `json:"location,omitempty"`
    IssueType     string `json:"issue_type,omitempty"`
}

type CoverageResult struct {
    Status          string `json:"status"`  // "in", "out", "unknown"
    DetectedLocation string `json:"detected_location,omitempty"`
    Reasoning       string `json:"reasoning,omitempty"`
}

func (s *AgentService) classify(
    ctx context.Context,
    config *TenantConfig,
    session *Session,
    message string,
) (*ClassificationResult, error) {

    // Build classification prompt
    prompt := s.buildClassificationPrompt(config, session, message)

    // Call LLM
    response, err := s.llmClient.Generate(ctx, prompt.System, prompt.User)
    if err != nil {
        return nil, err
    }

    // Parse JSON response
    var result ClassificationResult
    if err := json.Unmarshal([]byte(response), &result); err != nil {
        return nil, fmt.Errorf("failed to parse classification: %w", err)
    }

    return &result, nil
}

func (s *AgentService) buildClassificationPrompt(
    config *TenantConfig,
    session *Session,
    message string,
) *Prompt {

    // Build intent taxonomy section
    var intentDefs strings.Builder
    for _, intent := range config.Intents {
        intentDefs.WriteString(fmt.Sprintf(`
### %s (%s)
Code: %s
Category: %s
Description: %s
`, intent.Name, intent.Code, intent.Code, intent.Category, intent.Description))

        if len(intent.Examples) > 0 {
            intentDefs.WriteString("Examples that MATCH:\n")
            for _, ex := range intent.Examples {
                intentDefs.WriteString(fmt.Sprintf("- \"%s\"\n", ex))
            }
        }

        if len(intent.CounterExamples) > 0 {
            intentDefs.WriteString("Examples that DO NOT match:\n")
            for _, ex := range intent.CounterExamples {
                intentDefs.WriteString(fmt.Sprintf("- \"%s\"\n", ex))
            }
        }
        intentDefs.WriteString("\n")
    }

    // Build coverage areas section
    var coverageInfo strings.Builder
    coverageInfo.WriteString("COVERAGE AREAS:\n")
    coverageInfo.WriteString("We serve: ")
    for i, area := range config.CoverageInclude {
        if i > 0 {
            coverageInfo.WriteString(", ")
        }
        coverageInfo.WriteString(area.Name)
    }
    coverageInfo.WriteString("\nWe do NOT serve: ")
    for i, area := range config.CoverageExclude {
        if i > 0 {
            coverageInfo.WriteString(", ")
        }
        coverageInfo.WriteString(area.Name)
    }

    systemPrompt := fmt.Sprintf(`You are an intent classifier for %s.

Analyze the customer message and classify it.

## INTENT TAXONOMY
%s

## %s

## CONVERSATION CONTEXT
Phase: %s
Previous messages: %d
Known customer data: %s

## OUTPUT FORMAT (JSON only)
{
  "primary_intent": {
    "code": "intent_code",
    "confidence": 0.0-1.0,
    "reasoning": "brief explanation"
  },
  "urgency": {
    "level": 1-5,
    "reasoning": "why this level"
  },
  "extracted_data": {
    "customer_name": "if mentioned",
    "customer_phone": "if mentioned",
    "location": "city/state if mentioned",
    "issue_type": "water/fire/mold/etc if clear"
  },
  "coverage": {
    "status": "in/out/unknown",
    "detected_location": "location if found",
    "reasoning": "why this status"
  },
  "sentiment": "frustrated/neutral/satisfied/distressed"
}`,
        config.CompanyName,
        intentDefs.String(),
        coverageInfo.String(),
        session.Phase,
        len(session.Messages),
        formatKnownData(session),
    )

    return &Prompt{
        System: systemPrompt,
        User:   message,
    }
}
```

### 5.3 Policy Engine

```go
// services/agent/policy.go

type PolicyDecision struct {
    Action         string           // "emergency_flow", "standard_flow", "escalation", "out_of_area"
    Phase          string           // "triage", "handshake", "emergency", "conclusion"
    SafetyProtocol *SafetyProtocol  // If applicable
    Rejection      *RejectionInfo   // If out of area or excluded service
}

type RejectionInfo struct {
    Reason   string // "out_of_area", "excluded_service"
    Message  string // Template message
    Referral string // Suggested alternative
}

func (e *PolicyEngine) Evaluate(
    config *TenantConfig,
    session *Session,
    classification *ClassificationResult,
) *PolicyDecision {

    // Rule 1: Escalation (highest priority)
    if classification.PrimaryIntent.Code == "escalation" {
        if classification.PrimaryIntent.Confidence >= config.EscalationConfidenceThreshold {
            return &PolicyDecision{
                Action: "escalation",
                Phase:  "escalation",
            }
        }
    }

    // Rule 2: Out of coverage area
    if classification.Coverage.Status == "out" {
        return &PolicyDecision{
            Action: "out_of_area",
            Phase:  "rejection",
            Rejection: &RejectionInfo{
                Reason:  "out_of_area",
                Message: config.OutOfAreaTemplate,
            },
        }
    }

    // Rule 3: Excluded service
    if exclusion := e.matchExclusion(config, classification); exclusion != nil {
        // Check if exception applies
        if !e.exceptionApplies(exclusion, classification) {
            return &PolicyDecision{
                Action: "excluded_service",
                Phase:  "rejection",
                Rejection: &RejectionInfo{
                    Reason:   "excluded_service",
                    Message:  exclusion.Description,
                    Referral: exclusion.Referral,
                },
            }
        }
    }

    // Rule 4: Emergency
    if classification.Urgency.Level >= config.EmergencyUrgencyThreshold {
        safety := e.findSafetyProtocol(config, classification.PrimaryIntent.Code)
        return &PolicyDecision{
            Action:         "emergency_flow",
            Phase:          "emergency",
            SafetyProtocol: safety,
        }
    }

    // Rule 5: Standard flow
    phase := e.determinePhase(session, classification)
    return &PolicyDecision{
        Action: "standard_flow",
        Phase:  phase,
    }
}

func (e *PolicyEngine) determinePhase(session *Session, classification *ClassificationResult) string {
    // If farewell intent, go to conclusion
    if classification.PrimaryIntent.Code == "farewell" {
        return "conclusion"
    }

    // If we have customer info, move past triage
    if session.CustomerName != "" || session.CustomerPhone != "" {
        return "handshake"
    }

    // Stay in current phase or default to triage
    if session.Phase != "" {
        return session.Phase
    }
    return "triage"
}

func (e *PolicyEngine) findSafetyProtocol(config *TenantConfig, intentCode string) *SafetyProtocol {
    for _, safety := range config.SafetyProtocols {
        for _, trigger := range safety.TriggerIntents {
            if matchPattern(trigger, intentCode) {
                return &safety
            }
        }
    }
    return nil
}
```

### 5.4 Response Generator

```go
// services/agent/response.go

func (s *AgentService) generateResponse(
    ctx context.Context,
    config *TenantConfig,
    session *Session,
    classification *ClassificationResult,
    decision *PolicyDecision,
) (string, error) {

    prompt := s.buildResponsePrompt(config, session, classification, decision)

    response, err := s.llmClient.Generate(ctx, prompt.System, prompt.User)
    if err != nil {
        return "", err
    }

    return response, nil
}

func (s *AgentService) buildResponsePrompt(
    config *TenantConfig,
    session *Session,
    classification *ClassificationResult,
    decision *PolicyDecision,
) *Prompt {

    var sections []string

    // 1. Identity
    sections = append(sections, fmt.Sprintf(`## IDENTITY
You are the %s for %s.
Tone: %s
Brand Voice: "%s"
`, config.Role, config.CompanyName, config.Tone, config.BrandVoice))

    // 2. Current situation
    sections = append(sections, fmt.Sprintf(`## CURRENT SITUATION
Detected Intent: %s
Urgency Level: %d/5
Phase: %s
Customer Sentiment: %s
`, classification.PrimaryIntent.Code, classification.Urgency.Level, decision.Phase, classification.Sentiment))

    // 3. Safety protocol (if emergency)
    if decision.SafetyProtocol != nil {
        var instructions strings.Builder
        instructions.WriteString("## SAFETY PROTOCOL (MUST INCLUDE)\n")
        for i, step := range decision.SafetyProtocol.Instructions {
            instructions.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
        }
        instructions.WriteString(fmt.Sprintf("\nEmergency Phone: %s\n", config.EmergencyPhone))
        sections = append(sections, instructions.String())
    }

    // 4. Rejection info (if applicable)
    if decision.Rejection != nil {
        sections = append(sections, fmt.Sprintf(`## REJECTION
Reason: %s
Message to convey: %s
Referral: %s
`, decision.Rejection.Reason, decision.Rejection.Message, decision.Rejection.Referral))
    }

    // 5. Known customer data
    if session.CustomerName != "" || session.CustomerPhone != "" || session.CustomerLocation != "" {
        sections = append(sections, fmt.Sprintf(`## KNOWN CUSTOMER DATA (DO NOT ASK AGAIN)
Name: %s
Phone: %s
Location: %s
`, session.CustomerName, session.CustomerPhone, session.CustomerLocation))
    }

    // 6. Relevant KB (based on detected service/intent)
    relevantKB := s.selectRelevantKB(config, classification)
    if relevantKB != "" {
        sections = append(sections, fmt.Sprintf("## RELEVANT INFORMATION\n%s\n", relevantKB))
    }

    // 7. Conversation history (last 5 turns)
    history := formatRecentHistory(session.Messages, 5)
    sections = append(sections, fmt.Sprintf("## RECENT CONVERSATION\n%s\n", history))

    // 8. Rules
    rules := s.getActiveRules(config, classification.PrimaryIntent.Code)
    if len(rules) > 0 {
        var rulesText strings.Builder
        rulesText.WriteString("## RULES TO FOLLOW\n")
        for _, rule := range rules {
            rulesText.WriteString(fmt.Sprintf("- %s: %s\n", rule.Name, rule.Description))
        }
        sections = append(sections, rulesText.String())
    }

    // 9. Instructions
    sections = append(sections, `## INSTRUCTIONS
Generate a helpful response following the identity, rules, and current situation.
If this is an emergency, include the safety protocol.
If this is a rejection, be polite and offer referral if available.
Output only the response text, no JSON or markdown formatting.
`)

    systemPrompt := strings.Join(sections, "\n")

    // User prompt is the latest message (already in history, but explicit here)
    userMessage := session.Messages[len(session.Messages)-1].Content

    return &Prompt{
        System: systemPrompt,
        User:   userMessage,
    }
}
```

### 5.5 Config Loader

```go
// services/agent/config.go

type ConfigLoader struct {
    store     *store.Store
    cache     map[string]*cachedConfig
    cacheMu   sync.RWMutex
    cacheTTL  time.Duration
}

type cachedConfig struct {
    config    *TenantConfig
    loadedAt  time.Time
}

type TenantConfig struct {
    TenantID      int
    Slug          string
    CompanyName   string

    // Identity
    Role          string
    Tone          string
    BrandVoice    string

    // Contact
    EmergencyPhone string

    // Thresholds
    EmergencyUrgencyThreshold    int
    EscalationConfidenceThreshold float64

    // Data
    Services        []Service
    Exclusions      []Exclusion
    CoverageInclude []CoverageArea
    CoverageExclude []CoverageArea
    Intents         []Intent
    Rules           []Rule
    SafetyProtocols []SafetyProtocol
    FAQs            []FAQ
}

func NewConfigLoader(store *store.Store) *ConfigLoader {
    return &ConfigLoader{
        store:    store,
        cache:    make(map[string]*cachedConfig),
        cacheTTL: 5 * time.Minute,
    }
}

func (l *ConfigLoader) Load(slug string) (*TenantConfig, error) {
    // Check cache
    l.cacheMu.RLock()
    if cached, ok := l.cache[slug]; ok {
        if time.Since(cached.loadedAt) < l.cacheTTL {
            l.cacheMu.RUnlock()
            return cached.config, nil
        }
    }
    l.cacheMu.RUnlock()

    // Load from database
    config, err := l.loadFromDB(slug)
    if err != nil {
        return nil, err
    }

    // Update cache
    l.cacheMu.Lock()
    l.cache[slug] = &cachedConfig{
        config:   config,
        loadedAt: time.Now(),
    }
    l.cacheMu.Unlock()

    return config, nil
}

func (l *ConfigLoader) Invalidate(slug string) {
    l.cacheMu.Lock()
    delete(l.cache, slug)
    l.cacheMu.Unlock()
}
```

### 5.6 Session Store

```go
// services/agent/session.go

type SessionStore struct {
    store *store.Store
}

type Session struct {
    ID              string
    TenantID        int
    Phase           string
    CurrentIntent   string
    UrgencyLevel    int
    CoverageStatus  string
    CustomerName    string
    CustomerPhone   string
    CustomerLocation string
    DetectedService string
    Messages        []Message
    CreatedAt       time.Time
    UpdatedAt       time.Time
    IsCompleted     bool
}

type Message struct {
    Role      string    `json:"role"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}

func (s *SessionStore) GetOrCreate(sessionID string, tenantID int) *Session {
    if sessionID != "" {
        session, err := s.Get(sessionID)
        if err == nil && session.TenantID == tenantID {
            return session
        }
    }

    // Create new session
    return &Session{
        ID:        uuid.New().String(),
        TenantID:  tenantID,
        Phase:     "triage",
        Messages:  []Message{},
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
}

func (s *Session) AddMessage(role, content string) {
    s.Messages = append(s.Messages, Message{
        Role:      role,
        Content:   content,
        Timestamp: time.Now(),
    })
    s.UpdatedAt = time.Now()
}
```

---

## 6. LLM Integration

### 6.1 LLM Client Interface

```go
// llm/client.go

package llm

type Client interface {
    Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

type OpenRouterClient struct {
    apiKey string
    model  string
    client *openrouter.Client
}

func NewOpenRouterClient(apiKey, model string) *OpenRouterClient {
    return &OpenRouterClient{
        apiKey: apiKey,
        model:  model,
        client: openrouter.NewClient(apiKey),
    }
}

func (c *OpenRouterClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    messages := []openrouter.ChatCompletionMessage{
        openrouter.SystemMessage(systemPrompt),
        openrouter.UserMessage(userPrompt),
    }

    resp, err := c.client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
        Model:    c.model,
        Messages: messages,
    })
    if err != nil {
        return "", err
    }

    if len(resp.Choices) == 0 {
        return "", fmt.Errorf("no response from LLM")
    }

    return resp.Choices[0].Message.Content.Text, nil
}
```

### 6.2 Model Selection

| Use Case | Recommended Model | Reason |
|----------|-------------------|--------|
| Classification | `openai/gpt-4o-mini` | Fast, accurate for structured output |
| Response Generation | `openai/gpt-4o` | Better quality for conversation |
| Fallback | `anthropic/claude-3-haiku` | Good balance |

---

## 7. Testing Plan

### 7.1 Unit Tests

```go
// parser/parser_test.go

func TestParseKB_Services(t *testing.T) {
    content := `
# Test Company KB

## Services

### Water Damage
<!-- @service: water_damage, emergency: true -->
Description here.

### Mold Removal
<!-- @service: mold_removal, emergency: false -->
Another description.
`

    kb, err := ParseKB([]byte(content))
    assert.NoError(t, err)
    assert.Len(t, kb.Services, 2)
    assert.Equal(t, "water_damage", kb.Services[0].Code)
    assert.True(t, kb.Services[0].IsEmergency)
    assert.Equal(t, "mold_removal", kb.Services[1].Code)
    assert.False(t, kb.Services[1].IsEmergency)
}

func TestParsePolicy_Intents(t *testing.T) {
    content := `
# Test Policy

## Intents

### Water Emergency
<!-- @intent: emergency_water, category: emergency, urgency: 5, action: emergency_flow -->

**Description:**
Active water damage.

**Examples that MATCH:**
- "My basement is flooding"
- "Pipe burst"

**Examples that DO NOT match:**
- "Had a flood last week"
`

    policy, err := ParsePolicy([]byte(content))
    assert.NoError(t, err)
    assert.Len(t, policy.Intents, 1)
    assert.Equal(t, "emergency_water", policy.Intents[0].Code)
    assert.Equal(t, 5, *policy.Intents[0].Urgency)
    assert.Len(t, policy.Intents[0].Examples, 2)
    assert.Len(t, policy.Intents[0].CounterExamples, 1)
}

func TestPolicyEngine_Emergency(t *testing.T) {
    config := &TenantConfig{
        EmergencyUrgencyThreshold: 4,
        SafetyProtocols: []SafetyProtocol{
            {Code: "water", TriggerIntents: []string{"emergency_water"}},
        },
    }

    classification := &ClassificationResult{
        PrimaryIntent: IntentMatch{Code: "emergency_water", Confidence: 0.95},
        Urgency:       UrgencyResult{Level: 5},
    }

    engine := NewPolicyEngine()
    decision := engine.Evaluate(config, &Session{}, classification)

    assert.Equal(t, "emergency_flow", decision.Action)
    assert.Equal(t, "emergency", decision.Phase)
    assert.NotNil(t, decision.SafetyProtocol)
}
```

### 7.2 Integration Tests

```go
// services/agent/agent_test.go

func TestOnboard_Success(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    service := NewAgentService(db, mockLLM())

    kbContent := loadTestFile(t, "testdata/sample_kb.md")
    policyContent := loadTestFile(t, "testdata/sample_policy.md")

    result, err := service.Onboard(context.Background(), OnboardRequest{
        Slug:    "test-company",
        KB:      kbContent,
        Policy:  policyContent,
    })

    assert.NoError(t, err)
    assert.Equal(t, "test-company", result.Tenant.Slug)
    assert.Greater(t, result.Parsed.ServicesCount, 0)
    assert.Greater(t, result.Parsed.IntentsCount, 0)
}

func TestChat_Emergency(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    // Setup tenant
    setupTestTenant(t, db, "test-company")

    service := NewAgentService(db, mockLLMWithResponse(`{
        "primary_intent": {"code": "emergency_water", "confidence": 0.95},
        "urgency": {"level": 5},
        "coverage": {"status": "unknown"},
        "sentiment": "distressed"
    }`))

    resp, err := service.Chat(context.Background(), "test-company", ChatRequest{
        Message: "My basement is flooding!",
    })

    assert.NoError(t, err)
    assert.NotEmpty(t, resp.SessionID)
    assert.Equal(t, "emergency_water", resp.Metadata.Intent.Code)
    assert.Equal(t, 5, resp.Metadata.Urgency)
    assert.Equal(t, "emergency", resp.Metadata.Phase)
}
```

### 7.3 API Tests

```bash
# test_api.sh

BASE_URL="http://localhost:8080/api/v1/agent"

echo "=== Test Onboard ==="
curl -X POST "$BASE_URL/onboard" \
  -F "tenant_slug=test-company" \
  -F "kb_file=@testdata/sample_kb.md" \
  -F "policy_file=@testdata/sample_policy.md" \
  | jq .

echo "=== Test Chat (Emergency) ==="
curl -X POST "$BASE_URL/test-company/chat" \
  -H "Content-Type: application/json" \
  -d '{"message": "My basement is flooding right now!"}' \
  | jq .

echo "=== Test Chat (Standard) ==="
curl -X POST "$BASE_URL/test-company/chat" \
  -H "Content-Type: application/json" \
  -d '{"message": "What services do you offer?"}' \
  | jq .

echo "=== Test Export ==="
curl "$BASE_URL/test-company/export?format=json" | jq .
```

### 7.4 Test Scenarios

| Scenario | Input | Expected Output |
|----------|-------|-----------------|
| Emergency water | "My basement is flooding" | Emergency flow, safety protocol, urgency 5 |
| Emergency fire | "We just had a house fire" | Emergency flow, fire safety, urgency 5 |
| Standard inquiry | "What services do you offer?" | Standard flow, service list |
| Out of area | "I'm in Manhattan" | Rejection, out_of_area |
| Escalation | "I want to speak to your manager" | Escalation flow, supervisor contact |
| Booking | "Can I schedule an inspection?" | Booking flow, availability |
| Greeting | "Hello" | Greeting response |
| Past damage | "Had water damage last week" | Standard flow, not emergency |

### 7.5 Manual Testing Checklist

**Onboarding:**
- [ ] Upload valid KB.MD and POLICY.MD → Success
- [ ] Upload KB.MD missing emergency phone → Validation error
- [ ] Upload POLICY.MD missing identity → Validation error
- [ ] Upload with warnings → Success with warnings shown
- [ ] Re-import updated files → Data updated

**Chat:**
- [ ] New conversation → New session created
- [ ] Continue conversation → Same session, history preserved
- [ ] Emergency message → Safety protocol in response
- [ ] Out of area location → Polite rejection
- [ ] Excluded service → Polite rejection with referral
- [ ] Escalation request → Supervisor contact provided
- [ ] Multiple turns → Context maintained

**Export:**
- [ ] Export after onboard → Matches original (approximately)
- [ ] Export after admin edit → Reflects changes

---

## 8. File Structure

```
server/router/api/v1/
├── agent_handler.go           # HTTP handlers
├── agent_routes.go            # Route registration
└── agent/
    ├── service.go             # AgentService
    ├── classify.go            # Classification logic
    ├── policy.go              # Policy engine
    ├── response.go            # Response generation
    ├── config.go              # Config loader
    ├── session.go             # Session management
    ├── onboard.go             # Onboarding logic
    ├── export.go              # Export logic
    └── parser/
        ├── parser.go          # Main parser
        ├── kb.go              # KB.MD parsing
        ├── policy.go          # POLICY.MD parsing
        ├── annotations.go     # Annotation extraction
        ├── markdown.go        # Markdown helpers
        └── validate.go        # Validation
```

---

## 9. Implementation Sequence

### Phase 1: Foundation
1. [ ] Create database schema (migrations)
2. [ ] Implement parser for KB.MD
3. [ ] Implement parser for POLICY.MD
4. [ ] Implement validation

### Phase 2: Core Services
5. [ ] Implement config loader
6. [ ] Implement session store
7. [ ] Implement policy engine
8. [ ] Implement LLM client wrapper

### Phase 3: Agent Service
9. [ ] Implement classification
10. [ ] Implement response generation
11. [ ] Implement main Chat() function

### Phase 4: API Layer
12. [ ] Implement onboard handler
13. [ ] Implement chat handler
14. [ ] Implement import handler
15. [ ] Implement export handler
16. [ ] Register routes

### Phase 5: Testing
17. [ ] Unit tests for parser
18. [ ] Unit tests for policy engine
19. [ ] Integration tests for agent service
20. [ ] API tests
21. [ ] Manual testing with sample files

---

## Clarification Questions

Before implementation, please clarify:

1. **Authentication**: Should the chat endpoint require authentication (like the existing `/api/v1/chat`), or should it be open for anonymous users (customer-facing)?

2. **Session Storage**: Should sessions persist in SQLite (survives restart) or be in-memory only (faster, but lost on restart)?

3. **Admin UI**: Should the admin endpoints integrate with existing Memos user/role system, or use a separate admin key?

4. **LLM Model**: Should I use the existing `LLM_MODEL` environment variable, or add separate `AGENT_CLASSIFICATION_MODEL` and `AGENT_RESPONSE_MODEL` variables?

5. **Rate Limiting**: Should I add rate limiting per tenant for the chat endpoint?

---

**Document Version:** 4.0-IMP
**Created:** 2026-01-18

*Ready for implementation upon clarification.*
