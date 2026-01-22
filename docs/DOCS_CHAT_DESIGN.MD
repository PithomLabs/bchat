# AI Chat Agent: CLI to API Transformation Design

**Reference Document**

See [DOCS_CHAT_PAGE.MD](./DOCS_CHAT_PAGE.MD) for the existing chat integration implementation.

---

## Executive Summary

This document analyzes the existing CLI-based chat agent at `/home/chaschel/Documents/ibm/ai/distance.so/agent` and provides a comprehensive design for transforming it into a multi-tenant API service integrated with the Memos chat platform. The design supports multiple service verticals (HVAC, plumbing, restoration, etc.) with a unified architecture.

---

## Part 1: Current CLI Architecture Analysis

### 1.1 PocketFlow Node Pipeline

The agent uses a node-based workflow engine (PocketFlow) with the following execution flow:

```
User Input → P0Node → [Emergency|Extract]Node → DecideNode → ChatNode → SupervisorNode
                ↓                                    ↓
           Policy Decision                     LLM Response
```

#### Node Responsibilities

| Node | Purpose | I/O |
|------|---------|-----|
| **P0Node** | Triage: urgency scoring, coverage detection, phase management | Input → PolicyOutput |
| **EmergencyNode** | Emergency data extraction (name, phone, address) | Structured extraction |
| **ExtractNode** | Standard info extraction | Customer data |
| **DecideNode** | Simple routing logic | Phase → Action |
| **ChatNode** | LLM-based conversation | KB + History → Response |
| **SupervisorNode** | Oversight and quality control | Response validation |

### 1.2 Scoring System (P0Node)

The urgency scoring uses a three-tier approach:

1. **Semantic Pre-Check** - Fast keyword/fuzzy matching
2. **LLM Analysis** - Structured JSON response with weighted factors
3. **Heuristic Fallback** - Legacy keyword detection

```
Score >= 75 → Emergency
Score >= 1  → Ambiguous (clarify_urgency)
Score == 0  → Standard
```

### 1.3 Coverage Detection (Geo-Fencing)

Three-tier location validation:

1. **LLM Semantic Geofencing** - `service_status: in|out|unknown`
2. **Database Whitelist** - Explicit include/exclude locations
3. **Heuristic Regex** - String matching fallback

### 1.4 Escalation Verification (V8)

Two-stage escalation detection:

1. **Stage 1**: Keyword pre-filter (e.g., "lawyer", "manager", "complain")
2. **Stage 2**: LLM intent verification with confidence threshold

This prevents false positives like "my tenant is complaining" from triggering escalation.

### 1.5 Current Limitations

| Limitation | Impact |
|------------|--------|
| CLI-only interface | No web/mobile integration |
| Single-tenant | One company per deployment |
| Hardcoded KB path | No dynamic content loading |
| No session persistence | Conversations lost on restart |
| No analytics | No insight into agent performance |
| Monolithic config | KB/Policy/Prompt tightly coupled |

---

## Part 2: API Transformation Design

### 2.1 Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Memos Application                         │
├─────────────────────────────────────────────────────────────────┤
│  Frontend (React)                                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │  Chat.tsx   │  │ chatStore   │  │  API Client │              │
│  │  (UI Layer) │→ │  (MobX)     │→ │  /api/v1/*  │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
├─────────────────────────────────────────────────────────────────┤
│  Backend (Go)                                                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │  Echo HTTP  │→ │ Agent Core  │→ │  OpenRouter │              │
│  │  Handlers   │  │  (Ported)   │  │  Client     │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
│         ↓                ↓                                       │
│  ┌─────────────┐  ┌─────────────┐                               │
│  │  Sessions   │  │  Tenant DB  │                               │
│  │  (Redis)    │  │  (SQLite)   │                               │
│  └─────────────┘  └─────────────┘                               │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 New API Endpoints

#### Core Chat Endpoint (Enhanced)

```http
POST /api/v1/chat
Authorization: Bearer <token>
Content-Type: application/json

{
  "tenant_id": "bulovas",
  "session_id": "uuid-v4",
  "messages": [
    {"role": "user", "content": "My basement is flooding"}
  ],
  "context": {
    "customer_name": "John",
    "customer_phone": "555-1234",
    "location": "Ronkonkoma, NY"
  }
}
```

**Response:**

```json
{
  "message": {
    "role": "assistant",
    "content": "I understand this is an emergency..."
  },
  "metadata": {
    "urgency": "emergency",
    "coverage": "in",
    "phase": "triage",
    "action": "emergency_in",
    "extracted_data": {
      "customer_name": "John",
      "issue_type": "water_damage",
      "urgency_level": "emergency"
    }
  },
  "session_id": "uuid-v4"
}
```

#### Tenant Management Endpoints

```http
GET    /api/v1/tenants              # List all tenants
POST   /api/v1/tenants              # Create tenant
GET    /api/v1/tenants/:id          # Get tenant config
PUT    /api/v1/tenants/:id          # Update tenant config
DELETE /api/v1/tenants/:id          # Delete tenant

GET    /api/v1/tenants/:id/kb       # Get knowledge base
PUT    /api/v1/tenants/:id/kb       # Update knowledge base
GET    /api/v1/tenants/:id/policy   # Get policy rules
PUT    /api/v1/tenants/:id/policy   # Update policy rules
```

#### Session Management

```http
GET    /api/v1/sessions/:id         # Get session state
DELETE /api/v1/sessions/:id         # End session
GET    /api/v1/sessions/:id/history # Get conversation history
```

### 2.3 Database Schema Extensions

```sql
-- Tenant Configuration
CREATE TABLE agent_tenants (
    id INTEGER PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,          -- "bulovas", "hvac-pros"
    company_name TEXT NOT NULL,
    vertical TEXT NOT NULL,             -- "restoration", "hvac", "plumbing"
    config JSON NOT NULL,               -- P0Config, Identity, Integrations
    kb_text TEXT,                       -- Knowledge base content
    policy_text TEXT,                   -- Policy rules
    script_text TEXT,                   -- Conversation flow script
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Service Area Locations
CREATE TABLE agent_locations (
    id INTEGER PRIMARY KEY,
    tenant_id INTEGER REFERENCES agent_tenants(id),
    value TEXT NOT NULL,                -- "Nassau County", "Suffolk County"
    type TEXT NOT NULL,                 -- "county", "city", "zip"
    exclude BOOLEAN DEFAULT FALSE,
    lat REAL,
    lng REAL,
    radius_miles INTEGER DEFAULT 50
);

-- Chat Sessions
CREATE TABLE agent_sessions (
    id TEXT PRIMARY KEY,                -- UUID
    tenant_id INTEGER REFERENCES agent_tenants(id),
    user_id INTEGER REFERENCES user(id),
    state JSON NOT NULL,                -- ConversationState serialized
    history JSON NOT NULL,              -- Message history
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    completed BOOLEAN DEFAULT FALSE
);

-- Analytics Events
CREATE TABLE agent_analytics (
    id INTEGER PRIMARY KEY,
    session_id TEXT REFERENCES agent_sessions(id),
    event_type TEXT NOT NULL,           -- "message", "escalation", "completion"
    event_data JSON,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### 2.4 Go Service Architecture

#### File Structure

```
server/router/api/v1/
├── chat_service.go          # Existing (enhanced)
├── agent_service.go         # NEW - Agent core logic
├── tenant_service.go        # NEW - Tenant CRUD
├── session_service.go       # NEW - Session management
└── agent/
    ├── node.go              # PocketFlow node abstraction
    ├── p0_node.go           # Urgency scoring (ported)
    ├── chat_node.go         # LLM conversation (ported)
    ├── extract_node.go      # Data extraction (ported)
    ├── decision.go          # Policy decision logic
    ├── scoring.go           # Semantic + LLM scoring
    └── geofence.go          # Location validation
```

#### Agent Service Interface

```go
// agent_service.go

type AgentService struct {
    store       *store.Store
    llmClient   llm.Client
    sessions    SessionStore      // Redis or in-memory
}

type AgentRequest struct {
    TenantID   string            `json:"tenant_id"`
    SessionID  string            `json:"session_id"`
    Messages   []ChatMessage     `json:"messages"`
    Context    map[string]string `json:"context"`
}

type AgentResponse struct {
    Message   ChatMessage       `json:"message"`
    Metadata  ResponseMetadata  `json:"metadata"`
    SessionID string            `json:"session_id"`
}

type ResponseMetadata struct {
    Urgency       string            `json:"urgency"`
    Coverage      string            `json:"coverage"`
    Phase         string            `json:"phase"`
    Action        string            `json:"action"`
    ExtractedData map[string]string `json:"extracted_data"`
}

func (s *AgentService) ProcessMessage(ctx context.Context, req AgentRequest) (*AgentResponse, error) {
    // 1. Load tenant config
    tenant, err := s.store.GetTenant(req.TenantID)

    // 2. Load or create session
    session := s.sessions.GetOrCreate(req.SessionID, tenant)

    // 3. Execute P0 scoring
    p0Result := s.executeP0(ctx, req.Messages[len(req.Messages)-1].Content, tenant, session)

    // 4. Execute chat node
    response := s.executeChat(ctx, req.Messages, tenant, session, p0Result)

    // 5. Update session state
    s.sessions.Update(req.SessionID, session)

    // 6. Log analytics
    s.logEvent(req.SessionID, "message", p0Result)

    return response, nil
}
```

---

## Part 3: Multi-Vertical Support Design

### 3.1 Vertical Abstraction Layer

Each service vertical shares the same engine but with different configurations:

| Vertical | Emergency Keywords | Services | Geo-Scope |
|----------|-------------------|----------|-----------|
| **Restoration** | flooding, fire, mold, sewage | Water/Fire/Mold remediation | Regional |
| **HVAC** | no heat, AC broken, furnace out | Install/Repair/Maintenance | Regional |
| **Plumbing** | burst pipe, no water, leak | Pipe repair, drain cleaning | Regional |
| **Electrical** | sparks, no power, burning smell | Wiring, panel, emergency | Regional |

### 3.2 Configuration Template System

```json
{
  "vertical": "hvac",
  "identity": {
    "company_name": "CoolAir HVAC",
    "role": "HVAC Support Specialist",
    "tone": "Professional and reassuring",
    "brand_voice": "Keeping you comfortable, year-round."
  },
  "p0_config": {
    "role_definition": "HVAC Emergency Triage Specialist",
    "factors": [
      {
        "name": "safety_risk",
        "weight": 40,
        "description": "Risk to occupant safety (gas leak, fire hazard)",
        "semantic_map": {
          "symptoms": ["gas smell", "burning smell", "sparks"],
          "objects": ["furnace", "boiler", "heater", "AC unit"],
          "conditions": ["not working", "broken", "failed", "stopped"]
        }
      },
      {
        "name": "comfort_impact",
        "weight": 30,
        "description": "Impact on livability (no heat in winter, no AC in summer)",
        "keywords": ["freezing", "no heat", "no cooling", "sweltering"]
      },
      {
        "name": "property_risk",
        "weight": 20,
        "description": "Risk of property damage",
        "keywords": ["leaking", "water damage", "ice buildup"]
      },
      {
        "name": "time_sensitivity",
        "weight": 10,
        "description": "Urgency indicators",
        "keywords": ["emergency", "urgent", "immediately", "right now"]
      }
    ],
    "thresholds": {
      "emergency": 75,
      "urgent": 30,
      "standard": 0
    }
  },
  "services": {
    "core": [
      {"name": "AC Repair", "emergency": true},
      {"name": "Heating Repair", "emergency": true},
      {"name": "Furnace Installation", "emergency": false},
      {"name": "Duct Cleaning", "emergency": false},
      {"name": "Thermostat Install", "emergency": false}
    ],
    "exclusions": [
      "Plumbing",
      "Electrical (unless HVAC-related)",
      "General Construction"
    ]
  },
  "escalation_keywords": [
    "speak to manager", "lawyer", "complaint", "sue", "BBB"
  ],
  "satisfaction_keywords": [
    "thanks", "got it", "okay", "perfect", "goodbye"
  ]
}
```

### 3.3 Vertical-Specific Safety Protocols

#### HVAC Safety Rules

```markdown
## Safety Protocols - HVAC

### Gas Smell Emergency
1. DO NOT turn on any lights or electrical switches
2. Leave the building immediately
3. Call gas company from outside
4. Do NOT re-enter until cleared

### No Heat Emergency (Extreme Cold)
1. Check thermostat batteries
2. Check circuit breaker
3. If elderly/infants present, seek warm shelter
4. We prioritize vulnerable households

### AC Failure (Extreme Heat)
1. Stay hydrated
2. Use fans, open windows at night
3. Check for tripped breaker
4. We offer same-day emergency service
```

#### Plumbing Safety Rules

```markdown
## Safety Protocols - Plumbing

### Active Water Leak
1. Locate and turn OFF main water valve
2. Turn OFF water heater (gas: pilot, electric: breaker)
3. Move valuables away from water
4. DO NOT use electrical outlets near water

### Sewage Backup
1. DO NOT flush toilets or run water
2. Keep children and pets away
3. Ventilate the area if possible
4. This is a BIOHAZARD - do not attempt cleanup
```

### 3.4 Multi-Tenant Routing

```go
// Tenant routing middleware
func (s *APIV1Service) AgentRouter(c echo.Context) error {
    tenantID := c.Param("tenant_id")

    // Option 1: Subdomain routing (hvac.company.com)
    // Option 2: Path routing (/api/v1/agent/hvac/chat)
    // Option 3: Header routing (X-Tenant-ID: hvac)

    tenant, err := s.store.GetTenant(tenantID)
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "Unknown tenant")
    }

    c.Set("tenant", tenant)
    return s.HandleAgentChat(c)
}
```

---

## Part 4: Recommendations for KB, Policy, and Prompt Simplification

### 4.1 Knowledge Base (KB.MD) Recommendations

#### Current Issues

1. **Too Long** - 689 lines, overwhelming for LLM context
2. **Redundant Information** - Service areas repeated multiple times
3. **Mixed Concerns** - Combines operational data with AI instructions
4. **Static Format** - No dynamic sections for seasonal updates

#### Recommended Structure

```markdown
# Knowledge Base Schema v3

## 1. Company Identity (5-10 lines)
- Company name, mission, key differentiators
- Single paragraph, no tables

## 2. Services (Structured JSON)
- Core services with emergency flags
- Exclusions list
- Loaded from database, not markdown

## 3. Contact Information (5 lines)
- Primary phone (emergency)
- Secondary contact methods
- Address (if relevant)

## 4. Service Area (Database-driven)
- Counties/cities as database records
- Include/exclude flags
- Geo-coordinates for distance calc

## 5. Safety Protocols (Per-Service Type)
- Only load relevant protocols based on detected issue
- Water safety for water issues
- Fire safety for fire issues
- Not all protocols in every prompt

## 6. FAQ (Top 10)
- Most common questions
- Pre-written answers
- Reduces LLM improvisation
```

#### Proposed KB.MD Template

```markdown
# {Company Name} Knowledge Base

## Company
{Company Name} provides {service_type} services in {region}. We are {certifications} certified with {availability} availability.

## Emergency Contact
**{primary_phone}** - 24/7 Emergency Dispatch

## Services
{{services_from_database}}

## Service Area
{{locations_from_database}}

## Safety - {current_issue_type}
{{relevant_safety_protocol}}

## FAQ
{{top_10_faqs}}
```

**Benefit**: KB shrinks from 689 lines to ~50 dynamic lines.

### 4.2 Policy (POLICY.MD) Recommendations

#### Current Issues

1. **Prose Format** - Hard to parse programmatically
2. **No Priority Ordering** - All rules appear equal
3. **Missing Edge Cases** - What about partial matches?
4. **Example-Heavy** - Examples should be in tests, not policy

#### Recommended Structure: Policy as Code

```yaml
# policy.yaml - Machine-readable policy definition

version: "1.0"
tenant: "bulovas"

identity:
  role: "First Responder"
  tone: ["urgent_but_calm", "empathetic", "authoritative_on_safety"]
  brand_voice: "We take the burden off your shoulders immediately."
  constraints:
    - "Never sound panic-stricken"
    - "Use short sentences in emergencies"

service_rules:
  accept:
    - service: "water_damage"
      includes: ["flooding", "burst_pipes", "leaks", "sewage"]
    - service: "fire_damage"
      includes: ["smoke", "soot", "structural_fire"]
    - service: "mold"
      includes: ["inspection", "removal", "remediation"]

  reject:
    - category: "general_plumbing"
      exception: "unless caused flooding"
      response: "We specialize in damage restoration. For plumbing repairs, we recommend..."
    - category: "general_construction"
      response: "We focus on restoration after damage. For new construction..."

geo_rules:
  include:
    - type: "county"
      values: ["Nassau County", "Suffolk County"]
  exclude:
    - type: "borough"
      values: ["Queens", "Brooklyn", "Bronx", "Manhattan", "Staten Island"]
  out_of_area_response: |
    I'm sorry, we currently only service Long Island (Nassau and Suffolk Counties).
    For your area, I recommend searching for local restoration services.

operational_protocols:
  emergency_dispatch:
    trigger: "active_situation"
    indicators: ["water flowing", "fire just out", "gas smell"]
    action: "direct_to_phone"
    phone: "(631) 358-5200"

  safety_first:
    priority: 1
    rule: "Always provide safety advice before collecting contact info"
    protocols:
      water: ["Turn off main valve", "Avoid outlets near water"]
      fire: ["Do not enter until cleared", "Document with photos"]
      gas: ["Leave immediately", "Call from outside"]

conversation_guardrails:
  no_diagnosis:
    rule: "Never estimate costs or damage extent"
    response: "We need a certified tech to inspect it to be sure."

  no_repetition:
    rule: "Don't repeat full emergency script"
    action: "Acknowledge, answer, nudge to call"

escalation:
  triggers:
    - "asks for manager"
    - "threatens legal action"
    - "incoherent due to distress"
  action: "provide_supervisor_line"
  phone: "(631) 555-9999"
```

**Benefit**: Policy becomes testable, versionable, and machine-readable.

### 4.3 Prompt Architecture Recommendations

#### Current Issues in ChatNode

1. **Monolithic Prompt** - One giant system prompt with everything
2. **Inline KB** - Entire KB embedded in every request
3. **No Prompt Versioning** - Can't A/B test prompts
4. **Token Waste** - Repeated context on every turn

#### Recommended: Modular Prompt Assembly

```go
type PromptBuilder struct {
    tenant      *Tenant
    session     *Session
    currentTurn *Turn
}

func (b *PromptBuilder) BuildSystemPrompt() string {
    var parts []string

    // 1. Identity (always included)
    parts = append(parts, b.buildIdentity())

    // 2. Relevant KB section (dynamic)
    parts = append(parts, b.buildRelevantKB())

    // 3. Active policies (filtered by context)
    parts = append(parts, b.buildActivePolicies())

    // 4. Safety protocol (only if emergency detected)
    if b.session.IsEmergency() {
        parts = append(parts, b.buildSafetyProtocol())
    }

    // 5. Known customer data (if available)
    if b.session.HasCustomerData() {
        parts = append(parts, b.buildCustomerContext())
    }

    // 6. Phase-specific instructions
    parts = append(parts, b.buildPhaseInstructions())

    // 7. Output format
    parts = append(parts, b.buildOutputFormat())

    return strings.Join(parts, "\n\n")
}

func (b *PromptBuilder) buildRelevantKB() string {
    // Only include KB sections relevant to detected issue
    issueType := b.session.DetectedIssueType

    switch issueType {
    case "water_damage":
        return b.tenant.KB.WaterDamageSection()
    case "fire_damage":
        return b.tenant.KB.FireDamageSection()
    default:
        return b.tenant.KB.GeneralSection()
    }
}
```

#### Prompt Template Registry

```go
var promptTemplates = map[string]string{
    "identity_v1": `You are the AI Agent for {{.CompanyName}}.
Role: {{.Role}}
Tone: {{.Tone}}
Brand Voice: "{{.BrandVoice}}"`,

    "triage_v1": `=== TRIAGE PHASE ===
Your goal: Determine urgency and coverage.
Questions to answer:
1. Is this an emergency? (active damage)
2. Are they in our service area?
3. What type of damage?

Do NOT ask for contact info yet.`,

    "handshake_v1": `=== HANDSHAKE PHASE ===
Customer situation is understood.
Now: Collect necessary information.
- Name (if not known)
- Phone (if not known)
- Address (if relevant)
- Best time to contact

Safety advice has been given if emergency.`,

    "conclusion_v1": `=== CONCLUSION PHASE ===
All information collected.
Do NOT ask more questions.
Provide:
- Summary of next steps
- Timeline expectations
- Any final safety reminders
- Warm farewell`,
}
```

### 4.4 Unified Configuration Schema

Replace three separate files with one JSON/YAML config:

```yaml
# agent_config.yaml

metadata:
  version: "2.0"
  tenant: "bulovas"
  vertical: "restoration"
  last_updated: "2026-01-18"

identity:
  company_name: "Bulovas Restorations Inc."
  role: "First Responder"
  tone: "Urgent but calm, empathetic, authoritative on safety"
  brand_voice: "We take the burden off your shoulders immediately."

contact:
  emergency_phone: "(631) 358-5200"
  toll_free: "855-DRYITNOW"
  email: "info@bulovasrestorations.com"
  address: "85-7 Air Park Drive, Ronkonkoma, NY 11779"

services:
  core:
    - name: "Water Damage Restoration"
      keywords: ["flood", "burst pipe", "leak", "sewage"]
      emergency: true
    - name: "Fire & Smoke Restoration"
      keywords: ["fire", "smoke", "soot", "burned"]
      emergency: true
    - name: "Mold Remediation"
      keywords: ["mold", "mildew", "fungus"]
      emergency: false

  exclusions:
    - category: "General Plumbing"
      exception: "unless caused water damage"
    - category: "General Construction"
      exception: null

coverage:
  include:
    - {type: "county", value: "Nassau County, NY"}
    - {type: "county", value: "Suffolk County, NY"}
  exclude:
    - {type: "borough", value: "Queens, NY"}
    - {type: "borough", value: "Brooklyn, NY"}

p0_scoring:
  factors:
    - name: "safety_risk"
      weight: 40
      keywords: ["gas", "electric", "trapped", "injured"]
    - name: "property_damage"
      weight: 30
      keywords: ["flooding", "fire", "collapse"]
    - name: "time_sensitivity"
      weight: 20
      keywords: ["now", "immediately", "emergency"]
    - name: "scope"
      weight: 10
      keywords: ["entire house", "multiple rooms", "building"]
  thresholds:
    emergency: 75
    urgent: 30

safety_protocols:
  water:
    - "Turn off main water valve if safe"
    - "Turn off electricity to affected areas"
    - "Do not use electrical appliances near water"
  fire:
    - "Ensure everyone evacuated safely"
    - "Do not re-enter until fire department clears"
    - "Document damage with photos"
  gas:
    - "Do not turn on lights or electrical switches"
    - "Leave the building immediately"
    - "Call gas company from outside"

conversation:
  phases: ["triage", "handshake", "conclusion"]
  escalation_keywords: ["manager", "lawyer", "complain", "sue"]
  satisfaction_keywords: ["thanks", "got it", "okay", "bye"]
  completion_keywords: ["no", "nothing", "goodbye"]

integrations:
  ticket_system:
    enabled: true
    url: "https://tickets.example.com/api"
    api_key: "${TICKET_API_KEY}"
```

**Benefit**: Single source of truth, easily versionable, supports multi-tenant.

---

## Part 5: Implementation Roadmap

### Phase 1: Core API (Week 1-2)

- [ ] Port P0Node scoring logic to Go
- [ ] Port ChatNode conversation logic to Go
- [ ] Implement session management (in-memory first)
- [ ] Enhance `/api/v1/chat` endpoint with agent logic
- [ ] Add tenant configuration loading from database

### Phase 2: Multi-Tenant (Week 3-4)

- [ ] Create tenant management tables
- [ ] Implement tenant CRUD endpoints
- [ ] Add subdomain/path-based routing
- [ ] Create admin UI for tenant configuration
- [ ] Implement per-tenant KB/Policy storage

### Phase 3: Analytics & Persistence (Week 5-6)

- [ ] Implement Redis session store
- [ ] Add analytics event logging
- [ ] Create analytics dashboard
- [ ] Implement conversation export
- [ ] Add A/B testing for prompts

### Phase 4: Vertical Templates (Week 7-8)

- [ ] Create HVAC vertical template
- [ ] Create Plumbing vertical template
- [ ] Create Electrical vertical template
- [ ] Build template selection UI
- [ ] Document vertical customization

---

## Part 6: Migration Strategy

### From CLI to API

1. **Extract Core Logic** - Move `internal/nodes/*.go` to `server/router/api/v1/agent/`
2. **Adapt Interfaces** - Replace `pocketflow.PfContext` with `AgentContext`
3. **Database Integration** - Replace file-based config with database queries
4. **Session State** - Replace in-memory context with Redis/database sessions
5. **HTTP Layer** - Wrap logic in Echo handlers

### Backward Compatibility

The CLI can continue to work by calling the API:

```go
// CLI wrapper
func main() {
    client := &AgentClient{
        BaseURL: "http://localhost:8080",
        Tenant:  "bulovas",
    }

    reader := bufio.NewReader(os.Stdin)
    sessionID := uuid.New().String()

    for {
        fmt.Print("You: ")
        input, _ := reader.ReadString('\n')

        resp, _ := client.Chat(sessionID, input)
        fmt.Printf("Agent: %s\n", resp.Message.Content)
    }
}
```

---

## Appendix A: Sample API Request/Response Flow

### Emergency Water Damage Scenario

**Turn 1: Initial Contact**

```http
POST /api/v1/chat
{
  "tenant_id": "bulovas",
  "session_id": "abc-123",
  "messages": [
    {"role": "user", "content": "My basement is flooding! Water is everywhere!"}
  ]
}
```

```json
{
  "message": {
    "role": "assistant",
    "content": "I understand this is an emergency. Your safety is the priority.\n\n**Immediately:** If you can safely reach your main water valve, turn it off now. Stay away from any electrical outlets or panels near the water.\n\nCall our emergency dispatch right now: **(631) 358-5200**\n\nWe have crews available 24/7 with 30-minute response time for Long Island.\n\nAre you in Nassau or Suffolk County?"
  },
  "metadata": {
    "urgency": "emergency",
    "coverage": "unknown",
    "phase": "triage",
    "action": "emergency_unknown",
    "extracted_data": {
      "issue_type": "water_damage",
      "severity": "emergency",
      "active": true
    }
  }
}
```

**Turn 2: Location Confirmation**

```json
{
  "messages": [
    {"role": "user", "content": "My basement is flooding! Water is everywhere!"},
    {"role": "assistant", "content": "I understand this is an emergency..."},
    {"role": "user", "content": "Yes, I'm in Ronkonkoma"}
  ]
}
```

```json
{
  "message": {
    "role": "assistant",
    "content": "Ronkonkoma is right in our service area - we're actually based there.\n\nPlease call **(631) 358-5200** now. Tell them it's an active flood and give your address. They'll dispatch immediately.\n\nWhile you wait: Stay out of the flooded area. If you haven't already, turn off your water heater to prevent damage.\n\nIs everyone safe?"
  },
  "metadata": {
    "urgency": "emergency",
    "coverage": "in",
    "phase": "handshake",
    "action": "emergency_in"
  }
}
```

---

## Appendix B: Vertical Configuration Examples

### HVAC Vertical

```yaml
identity:
  company_name: "CoolAir HVAC"
  role: "HVAC Support Specialist"
  tone: "Professional and reassuring"

services:
  core:
    - {name: "AC Repair", emergency: true}
    - {name: "Heating Repair", emergency: true}
    - {name: "Furnace Installation", emergency: false}
    - {name: "Duct Cleaning", emergency: false}
```

### Plumbing Vertical

```yaml
identity:
  company_name: "FastFlow Plumbing"
  role: "Plumbing Emergency Responder"
  tone: "Calm and efficient"

services:
  core:
    - {name: "Burst Pipe Repair", emergency: true}
    - {name: "Sewage Backup", emergency: true}
    - {name: "Water Heater Repair", emergency: true}
    - {name: "Drain Cleaning", emergency: false}
```

---

**Document Version:** 1.0
**Created:** 2026-01-18
**Author:** Claude Code Assistant

---

*This document provides the architectural foundation for transforming the CLI-based chat agent into a production-ready, multi-tenant API service.*
