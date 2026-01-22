# AI Chat Agent: CLI to API Transformation Design v3

**Version:** 3.0
**Previous Version:** [DOCS_CHAT_DESIGN_2.MD](./DOCS_CHAT_DESIGN_2.MD)
**Date:** 2026-01-18

---

## Revision Summary

| Change | v2 Approach | v3 Approach |
|--------|-------------|-------------|
| Configuration source | Database-only | Files ↔ Database bidirectional sync |
| Business owner workflow | Admin UI | Edit familiar documents (MD, DOCX, Google Docs) |
| Source of truth | Database | Files (business documents) with database as runtime cache |
| Reconciliation | None | Automatic sync with conflict detection |

---

## Part 1: Document-First Architecture

### 1.1 Design Philosophy

**Business Reality:**
- Business owners maintain KB in Google Docs, Word, or Markdown
- Policies are written in natural language documents
- Teams collaborate on documents, not database UIs
- Version control (Google Docs history, Git) already exists for documents

**Technical Reality:**
- Chat agent needs fast, structured access to configuration
- LLM prompts need clean, parsed content
- Database enables caching, indexing, and quick lookups

**Solution: Files as Source of Truth, Database as Runtime Cache**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        BUSINESS OWNER WORKFLOW                               │
│                                                                              │
│   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                  │
│   │  KB.MD       │    │ POLICY.MD    │    │ CONFIG.MD    │                  │
│   │  (Knowledge  │    │ (Business    │    │ (Identity,   │                  │
│   │   Base)      │    │  Rules)      │    │  Contact)    │                  │
│   └──────┬───────┘    └──────┬───────┘    └──────┬───────┘                  │
│          │                   │                   │                          │
│          └───────────────────┼───────────────────┘                          │
│                              │                                               │
│                              ▼                                               │
│                    ┌─────────────────┐                                       │
│                    │   SYNC ENGINE   │                                       │
│                    │  (Parse + Map)  │                                       │
│                    └────────┬────────┘                                       │
│                              │                                               │
└──────────────────────────────┼───────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         RUNTIME DATABASE                                     │
│                                                                              │
│   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                  │
│   │ kb_sections  │    │   intents    │    │   config     │                  │
│   │ (parsed,     │    │ (extracted   │    │ (structured  │                  │
│   │  indexed)    │    │  rules)      │    │  JSON)       │                  │
│   └──────────────┘    └──────────────┘    └──────────────┘                  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          CHAT AGENT                                          │
│                    (reads from database)                                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 File Structure for Business Owners

```
/tenants/bulovas/
├── KB.MD                    # Knowledge Base (services, FAQs, procedures)
├── POLICY.MD                # Business rules and policies
├── CONFIG.MD                # Identity, contact, settings
├── INTENTS.MD               # Intent definitions (optional, can be in POLICY.MD)
├── SAFETY.MD                # Safety protocols (optional, can be in KB.MD)
└── .sync_state.json         # Auto-generated sync metadata (git-ignored)
```

**Business owners only edit `.MD` files.** Everything else is auto-generated.

---

## Part 2: Document Format Specification

### 2.1 KB.MD Format

Business-friendly markdown with optional structured hints:

```markdown
# [Company Name] Knowledge Base

> Last Updated: [Date]
> Author: [Name]

---

## Company Overview

[Company Name] is [description]. We provide [services] in [region].

**Key Facts:**
- Founded: [year]
- Certifications: [list]
- Availability: [hours]

---

## Contact Information

| Type | Value |
|------|-------|
| Emergency Phone | (631) 358-5200 |
| Toll-Free | 855-DRYITNOW |
| Email | info@company.com |
| Address | 123 Main St, City, ST 12345 |

---

## Services

### Water Damage Restoration
<!-- @service: water_damage, emergency: true -->

[Description of service...]

**Common Causes:**
- Floods
- Burst pipes
- Sewage backup

**Response Time:** 30 minutes for emergencies

---

### Fire & Smoke Restoration
<!-- @service: fire_damage, emergency: true -->

[Description...]

---

### Mold Remediation
<!-- @service: mold_remediation, emergency: false -->

[Description...]

---

## Services We Don't Provide

### General Plumbing
<!-- @exclusion: general_plumbing -->
<!-- @exception: unless it caused water damage to the property -->

We specialize in damage restoration, not general plumbing repairs.
**Recommendation:** Contact a licensed plumber for routine plumbing issues.

### General Construction
<!-- @exclusion: general_construction -->

We focus on restoration after damage, not new construction projects.

---

## Service Areas

### Areas We Serve
<!-- @coverage: include -->

**Nassau County, NY:**
Albertson, Atlantic Beach, Baldwin, Bayville, [...]

**Suffolk County, NY:**
Amagansett, Amityville, Aquebogue, Babylon, [...]

### Areas Outside Our Coverage
<!-- @coverage: exclude -->

We do not currently serve the NYC boroughs:
- Queens
- Brooklyn
- Bronx
- Manhattan
- Staten Island

---

## Frequently Asked Questions

### How quickly can you respond?
<!-- @faq: response_time -->

For emergencies, we have a 30-minute response time within Nassau and Suffolk County.
Standard appointments can typically be scheduled within 24-48 hours.

### Do you work with insurance?
<!-- @faq: insurance -->

Yes, we work with ALL insurance carriers and handle the paperwork directly.
You won't need to pay out of pocket immediately.

### What should I do while waiting for help?
<!-- @faq: while_waiting, trigger_intent: emergency_* -->

[Safety instructions - this section loads when emergency intent detected]

---

## Emergency Procedures

### Water Emergency
<!-- @safety: water_emergency -->
<!-- @triggers: emergency_water, emergency_flood, emergency_sewage -->

**Immediate Steps:**
1. Turn off main water valve if safe to do so
2. Turn off electricity to affected areas
3. Move away from standing water near electrical outlets
4. Do NOT use electrical appliances near water
5. Call our emergency line: **(631) 358-5200**

### Fire/Smoke Emergency
<!-- @safety: fire_emergency -->
<!-- @triggers: emergency_fire, emergency_smoke -->

**Immediate Steps:**
1. Ensure everyone has evacuated safely
2. Do NOT re-enter until fire department clears the building
3. Open windows for ventilation if safe
4. Document damage with photos from a safe distance
5. Call our emergency line: **(631) 358-5200**
```

### 2.2 POLICY.MD Format

```markdown
# [Company Name] Agent Policy

> Version: 1.0
> Effective Date: [Date]
> Owner: [Name/Role]

---

## 1. Agent Identity

<!-- @identity -->

**Role:** First Responder for [Company Name]
**Tone:** Urgent but calm, empathetic, authoritative on safety
**Brand Voice:** "We take the burden off your shoulders immediately."

**Constraints:**
- Never sound panic-stricken
- Use clear, short sentences in emergencies
- Always prioritize safety advice before collecting information

---

## 2. Intent Definitions

### Emergency Intents

#### Water Emergency
<!-- @intent: emergency_water -->
<!-- @category: emergency -->
<!-- @urgency: 5 -->
<!-- @action: emergency_flow -->

**Description:**
Active water damage requiring immediate response - flooding in progress,
burst pipe with water flowing, sewage backup, water actively entering property.

**Indicators:**
- Water described as "active", "flowing", "spreading", "pouring"
- Urgency language ("right now", "emergency", "help")
- Safety concerns mentioned

**Examples that MATCH:**
- "My basement is flooding right now"
- "A pipe burst and water is everywhere"
- "Sewage is backing up into my house"
- "Water is coming through the ceiling"

**Examples that DO NOT match:**
- "I had a flood last week, need cleanup"
- "There's some water stains on my ceiling"
- "My basement sometimes floods when it rains"

---

#### Fire/Smoke Emergency
<!-- @intent: emergency_fire -->
<!-- @category: emergency -->
<!-- @urgency: 5 -->
<!-- @action: emergency_flow -->

**Description:**
Recent fire or smoke damage requiring immediate assessment and response.

**Examples that MATCH:**
- "We just had a house fire"
- "There's smoke damage everywhere"
- "The fire department just left"

**Examples that DO NOT match:**
- "I smell something burning from the kitchen" (active fire - call 911)
- "We had a fire years ago"

---

### Escalation Intent

<!-- @intent: escalation -->
<!-- @category: meta -->
<!-- @urgency: N/A -->
<!-- @action: escalation_handler -->
<!-- @confidence_threshold: 0.85 -->

**Description:**
Customer wants to escalate AGAINST the company - file a formal complaint,
speak to a manager about the company's failure, threaten legal action,
or express serious dissatisfaction WITH the company's service.

**This is NOT escalation:**
- Mentioning third parties' complaints ("my tenant is complaining")
- Describing urgent property damage ("the situation is escalating")
- Requesting faster service ("I need to speak with someone who can help today")

**Examples that MATCH:**
- "I want to speak with your manager about this terrible service"
- "I'm going to file a complaint with the BBB"
- "This is unacceptable, get me a supervisor"
- "I'm contacting my lawyer about this"

**Examples that DO NOT match:**
- "My tenant is complaining about the leak"
- "The damage is getting worse, this is escalating"
- "Can I speak with someone who handles emergency calls?"

---

### Standard Intents

#### Routine Inquiry
<!-- @intent: routine_inquiry -->
<!-- @category: standard -->
<!-- @urgency: 1 -->
<!-- @action: standard_flow -->

**Description:**
General questions about services, pricing, availability, or process.

**Examples:**
- "How much does mold inspection cost?"
- "Do you service my area?"
- "What's your availability this week?"

---

#### Booking Request
<!-- @intent: booking_request -->
<!-- @category: standard -->
<!-- @urgency: 2 -->
<!-- @action: booking_flow -->

**Description:**
Customer wants to schedule a service appointment.

**Examples:**
- "I'd like to schedule an inspection"
- "Can someone come out tomorrow?"
- "I need to book a cleanup"

---

## 3. Conversation Rules

### Rule: Safety First
<!-- @rule: safety_first -->
<!-- @priority: 1 -->
<!-- @applies_to: emergency_* -->

Before collecting contact information in ANY emergency situation,
provide relevant safety advice first. Never skip safety instructions.

### Rule: No Diagnosis
<!-- @rule: no_diagnosis -->
<!-- @priority: 2 -->

Never attempt to estimate costs or extent of damage over chat.
Always say: "We need a certified technician to inspect it to give you an accurate assessment."

### Rule: No Repetition
<!-- @rule: no_repetition -->
<!-- @priority: 3 -->

If you have already provided the emergency phone number in this conversation,
do not repeat the full emergency script. Acknowledge their question,
answer it briefly, and gently remind them to call.

### Rule: Known Data
<!-- @rule: known_data -->
<!-- @priority: 4 -->

If customer has already provided their name, phone, or location in this conversation,
do NOT ask for it again. Reference what you know.

---

## 4. Scoring Thresholds

<!-- @thresholds -->

| Metric | Value | Description |
|--------|-------|-------------|
| Emergency Urgency | >= 4 | Triggers emergency flow |
| Urgent | >= 3 | Prioritized but not emergency |
| Escalation Confidence | >= 0.85 | Required to confirm escalation |
| Service Match Confidence | >= 0.70 | Required to suggest a service |
| Location Match Confidence | >= 0.75 | Required to confirm coverage |

---

## 5. Response Templates

### Out of Area Response
<!-- @template: out_of_area -->

I appreciate you reaching out. Unfortunately, we currently only service
Long Island (Nassau and Suffolk Counties).

For [detected location], I recommend searching for local [service type]
services in your area. In an emergency, your local fire department
can often provide referrals.

### Service Exclusion Response
<!-- @template: service_exclusion -->

We specialize in [our specialty], not [requested service].
[Exception note if applicable]

For [requested service], we recommend contacting a licensed [professional type].

Is there anything related to [our services] I can help you with?
```

### 2.3 CONFIG.MD Format

```markdown
# [Company Name] Configuration

> Tenant Slug: bulovas
> Vertical: restoration
> Last Updated: [Date]

---

## Company Information

**Company Name:** Bulovas Restorations Inc.
**Vertical:** Restoration (Water, Fire, Mold)
**Founded:** [Year]
**Certifications:** IICRC Certified

---

## Contact Details

| Field | Value |
|-------|-------|
| Emergency Phone | (631) 358-5200 |
| Toll-Free | 855-DRYITNOW (855-379-4866) |
| Secondary Lines | (833) 256-8453, (844) 379-2677 |
| Email | info@bulovasrestorations.com |
| Fax | 631-730-7446 |
| Address | 85-7 Air Park Drive, Ronkonkoma, NY 11779 |
| Hours | 24/7/365 |

---

## Conversation Settings

| Setting | Value |
|---------|-------|
| Max turns before suggesting phone | 8 |
| Session timeout (minutes) | 30 |
| Require phone for booking | Yes |
| Allow callback scheduling | Yes |

---

## Escalation Contact

| Field | Value |
|-------|-------|
| Supervisor Line | (631) 555-9999 |
| Escalation Email | escalations@bulovas.com |

---

## Integrations

### Ticket System
- **Enabled:** Yes
- **Provider:** Memos
- **Auto-create on escalation:** Yes

### CRM
- **Enabled:** No
```

---

## Part 3: Sync Engine Architecture

### 3.1 Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           SYNC ENGINE                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                   │
│   │   PARSER    │     │   MAPPER    │     │   WRITER    │                   │
│   │             │     │             │     │             │                   │
│   │ MD → AST    │ ──▶ │ AST → DB    │ ──▶ │ DB → Tables │                   │
│   │ DOCX → AST  │     │ Rows        │     │             │                   │
│   │ GDocs → AST │     │             │     │             │                   │
│   └─────────────┘     └─────────────┘     └─────────────┘                   │
│          │                                       │                           │
│          │            BIDIRECTIONAL              │                           │
│          │                                       │                           │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                   │
│   │  GENERATOR  │     │  FORMATTER  │     │   READER    │                   │
│   │             │     │             │     │             │                   │
│   │ AST → MD    │ ◀── │ DB Rows →   │ ◀── │ Tables →    │                   │
│   │ AST → DOCX  │     │ AST         │     │ DB Rows     │                   │
│   └─────────────┘     └─────────────┘     └─────────────┘                   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Parser Implementation

```go
// sync/parser.go

type DocumentAST struct {
    Metadata    DocumentMetadata
    Sections    []Section
    Services    []ServiceDef
    Exclusions  []ExclusionDef
    Coverage    CoverageDef
    Intents     []IntentDef
    Safety      []SafetyProtocol
    Rules       []RuleDef
    Templates   []TemplateDef
    FAQs        []FAQDef
    Thresholds  ThresholdsDef
    Config      ConfigDef
}

type Section struct {
    ID          string
    Title       string
    Content     string
    Annotations map[string]string  // Parsed from <!-- @key: value -->
    Children    []Section
}

// ParseMarkdown converts business markdown to structured AST
func ParseMarkdown(content []byte) (*DocumentAST, error) {
    ast := &DocumentAST{}

    // 1. Parse markdown structure
    doc := markdown.Parse(content)

    // 2. Walk the tree and extract sections
    sections := extractSections(doc)

    // 3. Process annotations (<!-- @service: water_damage -->)
    for _, section := range sections {
        annotations := extractAnnotations(section)

        switch {
        case annotations["service"] != "":
            ast.Services = append(ast.Services, parseService(section, annotations))

        case annotations["exclusion"] != "":
            ast.Exclusions = append(ast.Exclusions, parseExclusion(section, annotations))

        case annotations["intent"] != "":
            ast.Intents = append(ast.Intents, parseIntent(section, annotations))

        case annotations["safety"] != "":
            ast.Safety = append(ast.Safety, parseSafety(section, annotations))

        case annotations["rule"] != "":
            ast.Rules = append(ast.Rules, parseRule(section, annotations))

        case annotations["coverage"] != "":
            ast.Coverage = parseCoverage(section, annotations, ast.Coverage)

        case annotations["faq"] != "":
            ast.FAQs = append(ast.FAQs, parseFAQ(section, annotations))

        case annotations["template"] != "":
            ast.Templates = append(ast.Templates, parseTemplate(section, annotations))

        case annotations["thresholds"] != "":
            ast.Thresholds = parseThresholds(section)

        case annotations["identity"] != "":
            ast.Config.Identity = parseIdentity(section)

        default:
            // Regular content section
            ast.Sections = append(ast.Sections, section)
        }
    }

    return ast, nil
}

// extractAnnotations finds HTML comments with @key: value format
func extractAnnotations(section Section) map[string]string {
    annotations := make(map[string]string)

    // Match: <!-- @key: value --> or <!-- @key -->
    re := regexp.MustCompile(`<!--\s*@(\w+)(?::\s*(.+?))?\s*-->`)
    matches := re.FindAllStringSubmatch(section.Content, -1)

    for _, match := range matches {
        key := match[1]
        value := "true"
        if len(match) > 2 && match[2] != "" {
            value = strings.TrimSpace(match[2])
        }
        annotations[key] = value
    }

    return annotations
}

// parseIntent extracts intent definition from section
func parseIntent(section Section, annotations map[string]string) IntentDef {
    intent := IntentDef{
        Code:        annotations["intent"],
        Category:    annotations["category"],
        Action:      annotations["action"],
        DisplayName: section.Title,
    }

    // Parse urgency if present
    if u, ok := annotations["urgency"]; ok && u != "N/A" {
        intent.Urgency, _ = strconv.Atoi(u)
    }

    // Parse confidence threshold
    if ct, ok := annotations["confidence_threshold"]; ok {
        intent.ConfidenceThreshold, _ = strconv.ParseFloat(ct, 64)
    }

    // Extract description (first paragraph after title)
    intent.Description = extractFirstParagraph(section.Content)

    // Extract examples from "Examples that MATCH" section
    intent.Examples = extractListItems(section.Content, "Examples that MATCH")

    // Extract counter-examples from "Examples that DO NOT match" section
    intent.CounterExamples = extractListItems(section.Content, "Examples that DO NOT match")

    return intent
}
```

### 3.3 Mapper: AST to Database

```go
// sync/mapper.go

type SyncResult struct {
    Created   int
    Updated   int
    Deleted   int
    Unchanged int
    Errors    []SyncError
}

// SyncToDatabase maps parsed AST to database tables
func (s *SyncEngine) SyncToDatabase(tenantID int, ast *DocumentAST) (*SyncResult, error) {
    result := &SyncResult{}
    tx, err := s.db.Begin()
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // 1. Sync services
    if err := s.syncServices(tx, tenantID, ast.Services, result); err != nil {
        return nil, err
    }

    // 2. Sync exclusions
    if err := s.syncExclusions(tx, tenantID, ast.Exclusions, result); err != nil {
        return nil, err
    }

    // 3. Sync coverage areas
    if err := s.syncCoverage(tx, tenantID, ast.Coverage, result); err != nil {
        return nil, err
    }

    // 4. Sync intents
    if err := s.syncIntents(tx, tenantID, ast.Intents, result); err != nil {
        return nil, err
    }

    // 5. Sync safety protocols
    if err := s.syncSafety(tx, tenantID, ast.Safety, result); err != nil {
        return nil, err
    }

    // 6. Sync KB sections
    if err := s.syncKBSections(tx, tenantID, ast.Sections, ast.FAQs, result); err != nil {
        return nil, err
    }

    // 7. Sync config
    if err := s.syncConfig(tx, tenantID, ast.Config, ast.Thresholds, result); err != nil {
        return nil, err
    }

    // 8. Sync rules (stored in config JSON)
    if err := s.syncRules(tx, tenantID, ast.Rules, result); err != nil {
        return nil, err
    }

    // 9. Sync templates
    if err := s.syncTemplates(tx, tenantID, ast.Templates, result); err != nil {
        return nil, err
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return nil, err
    }

    // Update sync state file
    s.writeSyncState(tenantID, ast)

    return result, nil
}

func (s *SyncEngine) syncIntents(tx *sql.Tx, tenantID int, intents []IntentDef, result *SyncResult) error {
    // Get existing intents for this tenant
    existing, err := s.getExistingIntents(tx, tenantID)
    if err != nil {
        return err
    }

    existingMap := make(map[string]*IntentRow)
    for _, e := range existing {
        existingMap[e.IntentCode] = e
    }

    // Process each intent from AST
    for _, intent := range intents {
        examplesJSON, _ := json.Marshal(intent.Examples)
        counterExamplesJSON, _ := json.Marshal(intent.CounterExamples)

        if existingIntent, exists := existingMap[intent.Code]; exists {
            // Check if changed
            if s.intentChanged(existingIntent, intent) {
                // Update
                _, err := tx.Exec(`
                    UPDATE intent_taxonomy SET
                        display_name = ?,
                        description = ?,
                        category = ?,
                        examples = ?,
                        counter_examples = ?,
                        urgency = ?,
                        action_type = ?,
                        confidence_threshold = ?,
                        updated_at = CURRENT_TIMESTAMP
                    WHERE tenant_id = ? AND intent_code = ?
                `, intent.DisplayName, intent.Description, intent.Category,
                   examplesJSON, counterExamplesJSON, intent.Urgency,
                   intent.Action, intent.ConfidenceThreshold,
                   tenantID, intent.Code)

                if err != nil {
                    return err
                }
                result.Updated++
            } else {
                result.Unchanged++
            }
            delete(existingMap, intent.Code)
        } else {
            // Insert new
            _, err := tx.Exec(`
                INSERT INTO intent_taxonomy (
                    tenant_id, intent_code, display_name, description,
                    category, examples, counter_examples, urgency,
                    action_type, confidence_threshold
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            `, tenantID, intent.Code, intent.DisplayName, intent.Description,
               intent.Category, examplesJSON, counterExamplesJSON, intent.Urgency,
               intent.Action, intent.ConfidenceThreshold)

            if err != nil {
                return err
            }
            result.Created++
        }
    }

    // Mark remaining as inactive (soft delete)
    for code := range existingMap {
        _, err := tx.Exec(`
            UPDATE intent_taxonomy SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP
            WHERE tenant_id = ? AND intent_code = ?
        `, tenantID, code)
        if err != nil {
            return err
        }
        result.Deleted++
    }

    return nil
}
```

### 3.4 Generator: Database to Files

```go
// sync/generator.go

// GenerateMarkdown creates business-friendly markdown from database
func (s *SyncEngine) GenerateMarkdown(tenantID int) (*GeneratedFiles, error) {
    files := &GeneratedFiles{}

    // Load all data from database
    tenant, err := s.loadTenant(tenantID)
    if err != nil {
        return nil, err
    }

    // Generate KB.MD
    files.KB = s.generateKBMarkdown(tenant)

    // Generate POLICY.MD
    files.Policy = s.generatePolicyMarkdown(tenant)

    // Generate CONFIG.MD
    files.Config = s.generateConfigMarkdown(tenant)

    return files, nil
}

func (s *SyncEngine) generatePolicyMarkdown(tenant *TenantData) string {
    var buf bytes.Buffer

    // Header
    buf.WriteString(fmt.Sprintf("# %s Agent Policy\n\n", tenant.CompanyName))
    buf.WriteString(fmt.Sprintf("> Version: %d\n", tenant.ConfigVersion))
    buf.WriteString(fmt.Sprintf("> Last Synced: %s\n", time.Now().Format("2006-01-02")))
    buf.WriteString("\n---\n\n")

    // Identity section
    buf.WriteString("## 1. Agent Identity\n\n")
    buf.WriteString("<!-- @identity -->\n\n")
    buf.WriteString(fmt.Sprintf("**Role:** %s\n", tenant.Identity.Role))
    buf.WriteString(fmt.Sprintf("**Tone:** %s\n", tenant.Identity.Tone))
    buf.WriteString(fmt.Sprintf("**Brand Voice:** \"%s\"\n\n", tenant.Identity.BrandVoice))

    if len(tenant.Identity.Constraints) > 0 {
        buf.WriteString("**Constraints:**\n")
        for _, c := range tenant.Identity.Constraints {
            buf.WriteString(fmt.Sprintf("- %s\n", c))
        }
        buf.WriteString("\n")
    }
    buf.WriteString("---\n\n")

    // Intents section
    buf.WriteString("## 2. Intent Definitions\n\n")

    // Group by category
    intentsByCategory := groupIntentsByCategory(tenant.Intents)

    for category, intents := range intentsByCategory {
        buf.WriteString(fmt.Sprintf("### %s Intents\n\n", strings.Title(category)))

        for _, intent := range intents {
            buf.WriteString(fmt.Sprintf("#### %s\n", intent.DisplayName))
            buf.WriteString(fmt.Sprintf("<!-- @intent: %s -->\n", intent.Code))
            buf.WriteString(fmt.Sprintf("<!-- @category: %s -->\n", intent.Category))

            if intent.Urgency > 0 {
                buf.WriteString(fmt.Sprintf("<!-- @urgency: %d -->\n", intent.Urgency))
            } else {
                buf.WriteString("<!-- @urgency: N/A -->\n")
            }

            buf.WriteString(fmt.Sprintf("<!-- @action: %s -->\n", intent.Action))

            if intent.ConfidenceThreshold > 0 {
                buf.WriteString(fmt.Sprintf("<!-- @confidence_threshold: %.2f -->\n", intent.ConfidenceThreshold))
            }

            buf.WriteString("\n**Description:**\n")
            buf.WriteString(intent.Description + "\n\n")

            if len(intent.Examples) > 0 {
                buf.WriteString("**Examples that MATCH:**\n")
                for _, ex := range intent.Examples {
                    buf.WriteString(fmt.Sprintf("- \"%s\"\n", ex))
                }
                buf.WriteString("\n")
            }

            if len(intent.CounterExamples) > 0 {
                buf.WriteString("**Examples that DO NOT match:**\n")
                for _, ex := range intent.CounterExamples {
                    buf.WriteString(fmt.Sprintf("- \"%s\"\n", ex))
                }
                buf.WriteString("\n")
            }

            buf.WriteString("---\n\n")
        }
    }

    // Rules section
    buf.WriteString("## 3. Conversation Rules\n\n")
    for _, rule := range tenant.Rules {
        buf.WriteString(fmt.Sprintf("### Rule: %s\n", rule.DisplayName))
        buf.WriteString(fmt.Sprintf("<!-- @rule: %s -->\n", rule.Code))
        buf.WriteString(fmt.Sprintf("<!-- @priority: %d -->\n", rule.Priority))
        if rule.AppliesTo != "" {
            buf.WriteString(fmt.Sprintf("<!-- @applies_to: %s -->\n", rule.AppliesTo))
        }
        buf.WriteString("\n" + rule.Description + "\n\n")
    }

    // Thresholds section
    buf.WriteString("## 4. Scoring Thresholds\n\n")
    buf.WriteString("<!-- @thresholds -->\n\n")
    buf.WriteString("| Metric | Value | Description |\n")
    buf.WriteString("|--------|-------|-------------|\n")
    buf.WriteString(fmt.Sprintf("| Emergency Urgency | >= %d | Triggers emergency flow |\n",
        tenant.Thresholds.EmergencyUrgency))
    buf.WriteString(fmt.Sprintf("| Escalation Confidence | >= %.2f | Required to confirm escalation |\n",
        tenant.Thresholds.EscalationConfidence))

    return buf.String()
}
```

### 3.5 Reconciliation Logic

```go
// sync/reconcile.go

type ReconcileResult struct {
    Conflicts    []Conflict
    AutoResolved []AutoResolution
    NeedsReview  []NeedsReview
}

type Conflict struct {
    Type       string           // "intent", "service", "coverage"
    Key        string           // "emergency_water"
    FileValue  interface{}
    DBValue    interface{}
    Resolution RecommendedResolution
}

type RecommendedResolution string

const (
    UseFile     RecommendedResolution = "use_file"      // File is newer/authoritative
    UseDatabase RecommendedResolution = "use_database"  // DB has manual changes
    NeedsManual RecommendedResolution = "needs_manual"  // Human must decide
)

// Reconcile compares files with database and identifies conflicts
func (s *SyncEngine) Reconcile(tenantID int, files *ParsedFiles) (*ReconcileResult, error) {
    result := &ReconcileResult{}

    // Load current database state
    dbState, err := s.loadDBState(tenantID)
    if err != nil {
        return nil, err
    }

    // Load last sync state (stored in .sync_state.json)
    lastSync, err := s.loadLastSyncState(tenantID)
    if err != nil {
        // First sync - no conflicts possible
        return result, nil
    }

    // Three-way comparison: lastSync vs files vs database
    // This allows us to detect:
    // 1. File changed since last sync → Apply file changes
    // 2. DB changed since last sync → Someone made manual changes
    // 3. Both changed → Conflict!

    // Compare intents
    for code, fileIntent := range files.AST.Intents {
        lastIntent := lastSync.Intents[code]
        dbIntent := dbState.Intents[code]

        fileChanged := !intentEqual(fileIntent, lastIntent)
        dbChanged := !intentEqual(dbIntent, lastIntent)

        if fileChanged && dbChanged {
            // Both changed - conflict!
            result.Conflicts = append(result.Conflicts, Conflict{
                Type:       "intent",
                Key:        code,
                FileValue:  fileIntent,
                DBValue:    dbIntent,
                Resolution: s.recommendResolution(fileIntent, dbIntent, lastIntent),
            })
        } else if fileChanged {
            // Only file changed - auto-resolve to file
            result.AutoResolved = append(result.AutoResolved, AutoResolution{
                Type:   "intent",
                Key:    code,
                Action: "Updated from file",
            })
        } else if dbChanged {
            // Only DB changed - this means manual edit happened
            result.NeedsReview = append(result.NeedsReview, NeedsReview{
                Type:    "intent",
                Key:     code,
                Message: "Database was manually modified. Export to file to preserve changes.",
            })
        }
    }

    // Similar logic for services, coverage, etc.

    return result, nil
}

// PerformSync executes the sync based on reconciliation
func (s *SyncEngine) PerformSync(tenantID int, files *ParsedFiles, resolution ConflictResolution) (*SyncResult, error) {
    result := &SyncResult{}

    // First reconcile
    reconciled, err := s.Reconcile(tenantID, files)
    if err != nil {
        return nil, err
    }

    // Check for unresolved conflicts
    if len(reconciled.Conflicts) > 0 && resolution == nil {
        return nil, fmt.Errorf("unresolved conflicts: %d. Please provide resolution", len(reconciled.Conflicts))
    }

    // Apply file changes to database
    tx, err := s.db.Begin()
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // Apply all changes
    if err := s.applyChanges(tx, tenantID, files, reconciled, resolution); err != nil {
        return nil, err
    }

    // Commit
    if err := tx.Commit(); err != nil {
        return nil, err
    }

    // Update sync state
    s.writeSyncState(tenantID, files.AST)

    return result, nil
}
```

---

## Part 4: Sync API & CLI

### 4.1 API Endpoints

```http
# Sync Operations
POST   /api/v1/admin/tenants/:slug/sync              # Sync files → database
GET    /api/v1/admin/tenants/:slug/sync/status       # Get last sync status
POST   /api/v1/admin/tenants/:slug/sync/preview      # Preview changes without applying
POST   /api/v1/admin/tenants/:slug/sync/resolve      # Resolve conflicts and sync

# Export Operations
GET    /api/v1/admin/tenants/:slug/export            # Export database → files
GET    /api/v1/admin/tenants/:slug/export/kb         # Export KB only
GET    /api/v1/admin/tenants/:slug/export/policy     # Export policy only
GET    /api/v1/admin/tenants/:slug/export/config     # Export config only

# File Upload (for web UI)
POST   /api/v1/admin/tenants/:slug/upload            # Upload files for sync
```

### 4.2 CLI Commands

```bash
# Sync from files to database
./memos agent sync --tenant bulovas --path /tenants/bulovas/

# Preview sync (dry run)
./memos agent sync --tenant bulovas --path /tenants/bulovas/ --preview

# Export database to files
./memos agent export --tenant bulovas --output /tenants/bulovas/

# Check sync status
./memos agent sync-status --tenant bulovas

# Force sync (overwrite DB with files, ignore conflicts)
./memos agent sync --tenant bulovas --path /tenants/bulovas/ --force

# Validate files without syncing
./memos agent validate --path /tenants/bulovas/
```

### 4.3 Sync Service Implementation

```go
// sync_service.go

func (s *APIV1Service) SyncTenantFiles(c echo.Context) error {
    tenantSlug := c.Param("slug")

    // Get tenant
    tenant, err := s.store.GetTenantBySlug(tenantSlug)
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
    }

    // Parse request
    var req SyncRequest
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
    }

    // Parse uploaded files or read from configured path
    var files *ParsedFiles
    if req.Files != nil {
        files, err = s.syncEngine.ParseUploadedFiles(req.Files)
    } else if req.Path != "" {
        files, err = s.syncEngine.ParseFilesFromPath(req.Path)
    } else {
        files, err = s.syncEngine.ParseFilesFromPath(tenant.FilesPath)
    }
    if err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
    }

    // Validate
    if errs := s.syncEngine.Validate(files); len(errs) > 0 {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "errors": errs,
        })
    }

    // Preview mode?
    if req.Preview {
        reconciled, err := s.syncEngine.Reconcile(tenant.ID, files)
        if err != nil {
            return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
        }
        return c.JSON(http.StatusOK, map[string]interface{}{
            "preview":      true,
            "changes":      reconciled.AutoResolved,
            "conflicts":    reconciled.Conflicts,
            "needs_review": reconciled.NeedsReview,
        })
    }

    // Perform sync
    result, err := s.syncEngine.PerformSync(tenant.ID, files, req.Resolution)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    // Invalidate config cache
    s.configLoader.InvalidateCache(tenantSlug)

    return c.JSON(http.StatusOK, map[string]interface{}{
        "success": true,
        "result":  result,
    })
}

func (s *APIV1Service) ExportTenantFiles(c echo.Context) error {
    tenantSlug := c.Param("slug")

    tenant, err := s.store.GetTenantBySlug(tenantSlug)
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
    }

    files, err := s.syncEngine.GenerateMarkdown(tenant.ID)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    // Return as downloadable zip or JSON with file contents
    format := c.QueryParam("format")
    if format == "zip" {
        return s.sendAsZip(c, files)
    }

    return c.JSON(http.StatusOK, files)
}
```

---

## Part 5: Workflow for Different Stakeholders

### 5.1 Business Owner Workflow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     BUSINESS OWNER DAILY WORKFLOW                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. EDIT DOCUMENTS (familiar tools)                                          │
│     ┌──────────────────────────────────────────────────────────────────┐    │
│     │  • Open KB.MD in VS Code / Google Docs / Word                    │    │
│     │  • Add new service section                                        │    │
│     │  • Update FAQ                                                     │    │
│     │  • Save file                                                      │    │
│     └──────────────────────────────────────────────────────────────────┘    │
│                                      │                                       │
│                                      ▼                                       │
│  2. SYNC TO SYSTEM (one command or button)                                   │
│     ┌──────────────────────────────────────────────────────────────────┐    │
│     │  Option A: CLI                                                    │    │
│     │  $ ./memos agent sync --tenant mycompany                         │    │
│     │                                                                   │    │
│     │  Option B: Admin UI                                               │    │
│     │  [Click "Sync Files"] button                                      │    │
│     │                                                                   │    │
│     │  Option C: Git Hook (automatic on push)                          │    │
│     │  Files pushed to repo → CI/CD syncs automatically                │    │
│     └──────────────────────────────────────────────────────────────────┘    │
│                                      │                                       │
│                                      ▼                                       │
│  3. VERIFY CHANGES (optional)                                                │
│     ┌──────────────────────────────────────────────────────────────────┐    │
│     │  • Test chatbot in preview mode                                  │    │
│     │  • View sync report showing what changed                         │    │
│     │  • Rollback if needed                                            │    │
│     └──────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 Developer/Ops Workflow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        DEVELOPER/OPS WORKFLOW                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. INITIAL SETUP                                                            │
│     ┌──────────────────────────────────────────────────────────────────┐    │
│     │  # Create tenant                                                  │    │
│     │  ./memos agent create-tenant \                                   │    │
│     │      --slug mycompany \                                          │    │
│     │      --name "My Company Inc" \                                   │    │
│     │      --vertical restoration \                                    │    │
│     │      --files-path /data/tenants/mycompany                       │    │
│     │                                                                   │    │
│     │  # Copy template files                                            │    │
│     │  cp -r /templates/restoration/* /data/tenants/mycompany/         │    │
│     │                                                                   │    │
│     │  # Initial sync                                                   │    │
│     │  ./memos agent sync --tenant mycompany                           │    │
│     └──────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  2. MONITORING                                                               │
│     ┌──────────────────────────────────────────────────────────────────┐    │
│     │  # View sync status                                               │    │
│     │  ./memos agent sync-status --tenant mycompany                    │    │
│     │                                                                   │    │
│     │  # View recent analytics                                          │    │
│     │  ./memos agent analytics --tenant mycompany --period 24h         │    │
│     │                                                                   │    │
│     │  # Check for intent accuracy issues                              │    │
│     │  ./memos agent intent-report --tenant mycompany                  │    │
│     └──────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  3. TROUBLESHOOTING                                                          │
│     ┌──────────────────────────────────────────────────────────────────┐    │
│     │  # Export current DB state to files for review                   │    │
│     │  ./memos agent export --tenant mycompany --output ./debug/       │    │
│     │                                                                   │    │
│     │  # Validate files without syncing                                │    │
│     │  ./memos agent validate --path /data/tenants/mycompany/          │    │
│     │                                                                   │    │
│     │  # Force resync (overwrites DB with files)                       │    │
│     │  ./memos agent sync --tenant mycompany --force                   │    │
│     └──────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Git-Based Workflow (CI/CD)

```yaml
# .github/workflows/agent-sync.yml

name: Sync Agent Configuration

on:
  push:
    paths:
      - 'tenants/**/KB.MD'
      - 'tenants/**/POLICY.MD'
      - 'tenants/**/CONFIG.MD'

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Detect changed tenants
        id: changes
        run: |
          TENANTS=$(git diff --name-only HEAD~1 | grep "^tenants/" | cut -d'/' -f2 | sort -u)
          echo "tenants=$TENANTS" >> $GITHUB_OUTPUT

      - name: Validate files
        run: |
          for tenant in ${{ steps.changes.outputs.tenants }}; do
            ./memos agent validate --path tenants/$tenant/
          done

      - name: Sync to database
        run: |
          for tenant in ${{ steps.changes.outputs.tenants }}; do
            ./memos agent sync --tenant $tenant --path tenants/$tenant/
          done
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}

      - name: Notify on failure
        if: failure()
        run: |
          # Send Slack/email notification
```

---

## Part 6: Validation Rules

### 6.1 File Validation

```go
// sync/validator.go

type ValidationError struct {
    File    string
    Line    int
    Type    string   // "error", "warning"
    Message string
}

func (s *SyncEngine) Validate(files *ParsedFiles) []ValidationError {
    var errors []ValidationError

    // 1. Required sections in KB.MD
    if files.KB != nil {
        if files.KB.Contact.EmergencyPhone == "" {
            errors = append(errors, ValidationError{
                File:    "KB.MD",
                Type:    "error",
                Message: "Emergency phone number is required in Contact Information section",
            })
        }

        if len(files.KB.Services) == 0 {
            errors = append(errors, ValidationError{
                File:    "KB.MD",
                Type:    "error",
                Message: "At least one service must be defined",
            })
        }
    }

    // 2. Required intents in POLICY.MD
    if files.Policy != nil {
        hasEscalation := false
        for _, intent := range files.Policy.Intents {
            if intent.Code == "escalation" {
                hasEscalation = true
            }

            // Intent must have description
            if intent.Description == "" {
                errors = append(errors, ValidationError{
                    File:    "POLICY.MD",
                    Type:    "error",
                    Message: fmt.Sprintf("Intent '%s' must have a description", intent.Code),
                })
            }

            // Intent should have examples
            if len(intent.Examples) == 0 {
                errors = append(errors, ValidationError{
                    File:    "POLICY.MD",
                    Type:    "warning",
                    Message: fmt.Sprintf("Intent '%s' has no examples - classification may be less accurate", intent.Code),
                })
            }
        }

        if !hasEscalation {
            errors = append(errors, ValidationError{
                File:    "POLICY.MD",
                Type:    "warning",
                Message: "No 'escalation' intent defined - escalation detection will use defaults",
            })
        }
    }

    // 3. CONFIG.MD validation
    if files.Config != nil {
        if files.Config.Identity.Role == "" {
            errors = append(errors, ValidationError{
                File:    "CONFIG.MD",
                Type:    "error",
                Message: "Agent role must be defined",
            })
        }
    }

    // 4. Cross-file validation
    // Check that safety protocols reference valid intents
    for _, safety := range files.KB.Safety {
        for _, trigger := range safety.Triggers {
            found := false
            for _, intent := range files.Policy.Intents {
                if matchesPattern(trigger, intent.Code) {
                    found = true
                    break
                }
            }
            if !found {
                errors = append(errors, ValidationError{
                    File:    "KB.MD",
                    Type:    "warning",
                    Message: fmt.Sprintf("Safety protocol '%s' triggers on '%s' which is not a defined intent", safety.Code, trigger),
                })
            }
        }
    }

    return errors
}
```

---

## Part 7: Complete Database Schema (Updated)

```sql
-- Core tenant table
CREATE TABLE agent_tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    company_name TEXT NOT NULL,
    vertical TEXT NOT NULL,
    files_path TEXT,                        -- Path to tenant's document files
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Sync state tracking
CREATE TABLE agent_sync_state (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    last_sync_at TIMESTAMP,
    last_sync_hash TEXT,                    -- Hash of files at last sync
    last_sync_result JSON,                  -- Result summary
    files_modified_at TIMESTAMP,            -- Last known file modification time
    db_modified_at TIMESTAMP,               -- Last database modification time
    sync_status TEXT DEFAULT 'synced',      -- 'synced', 'files_ahead', 'db_ahead', 'conflict'

    UNIQUE(tenant_id)
);

-- Configuration versions (keeps history)
CREATE TABLE agent_tenant_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    version INTEGER NOT NULL DEFAULT 1,
    is_active BOOLEAN DEFAULT TRUE,
    source TEXT DEFAULT 'file',             -- 'file' or 'api' (how this version was created)
    source_file_hash TEXT,                  -- Hash of source file (for reconciliation)

    identity JSON NOT NULL,
    contact JSON NOT NULL,
    conversation JSON NOT NULL,
    scoring JSON NOT NULL,
    rules JSON,
    integrations JSON,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,                        -- User or 'sync_engine'

    UNIQUE(tenant_id, version)
);

-- Intent taxonomy with source tracking
CREATE TABLE intent_taxonomy (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER REFERENCES agent_tenants(id),  -- NULL = global
    intent_code TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL,
    examples JSON,
    counter_examples JSON,
    urgency INTEGER,
    action_type TEXT NOT NULL,
    confidence_threshold REAL DEFAULT 0.7,
    priority INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,

    source TEXT DEFAULT 'file',             -- 'file' or 'api'
    source_line INTEGER,                    -- Line number in source file
    last_synced_at TIMESTAMP,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(COALESCE(tenant_id, 0), intent_code)
);

-- Services with source tracking
CREATE TABLE agent_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    service_code TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL,
    is_emergency_capable BOOLEAN DEFAULT FALSE,
    typical_response_time TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INTEGER DEFAULT 0,

    source TEXT DEFAULT 'file',
    source_line INTEGER,
    last_synced_at TIMESTAMP,

    UNIQUE(tenant_id, service_code)
);

-- KB sections with source tracking
CREATE TABLE agent_kb_sections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    section_code TEXT NOT NULL,
    section_title TEXT NOT NULL,
    content TEXT NOT NULL,
    trigger_services JSON,
    trigger_intents JSON,
    always_include BOOLEAN DEFAULT FALSE,
    sort_order INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,

    source TEXT DEFAULT 'file',
    source_file TEXT,                       -- 'KB.MD' or 'POLICY.MD'
    source_line INTEGER,
    last_synced_at TIMESTAMP,

    UNIQUE(tenant_id, section_code)
);

-- Other tables follow same pattern with source tracking...

-- Indexes
CREATE INDEX idx_sync_state_tenant ON agent_sync_state(tenant_id);
CREATE INDEX idx_intents_tenant_active ON intent_taxonomy(tenant_id, is_active);
CREATE INDEX idx_services_tenant_active ON agent_services(tenant_id, is_active);
CREATE INDEX idx_kb_tenant_active ON agent_kb_sections(tenant_id, is_active);
```

---

## Part 8: Summary of Changes from v2

| Aspect | v2 | v3 |
|--------|-----|-----|
| **Source of Truth** | Database | Files (MD, DOCX) |
| **Business Owner Edits** | Admin UI | Familiar documents |
| **Configuration Storage** | Database only | Files ↔ Database sync |
| **Sync Direction** | One-way (API → DB) | Bidirectional |
| **Conflict Handling** | None | Three-way reconciliation |
| **Version Control** | Database versions | Git + database versions |
| **Validation** | API-level | File parsing + cross-file |
| **CI/CD Integration** | Manual | Git hooks supported |

---

## Appendix A: Annotation Reference

Quick reference for business owners writing documents:

| Annotation | Usage | File |
|------------|-------|------|
| `<!-- @service: code, emergency: true -->` | Define a service | KB.MD |
| `<!-- @exclusion: code -->` | Service we don't provide | KB.MD |
| `<!-- @exception: text -->` | Exception to exclusion | KB.MD |
| `<!-- @coverage: include -->` | Areas we serve | KB.MD |
| `<!-- @coverage: exclude -->` | Areas we don't serve | KB.MD |
| `<!-- @faq: code -->` | FAQ entry | KB.MD |
| `<!-- @safety: code -->` | Safety protocol | KB.MD |
| `<!-- @triggers: intent1, intent2 -->` | When to show safety | KB.MD |
| `<!-- @intent: code -->` | Define intent | POLICY.MD |
| `<!-- @category: emergency\|standard\|meta -->` | Intent category | POLICY.MD |
| `<!-- @urgency: 1-5 -->` | Urgency level | POLICY.MD |
| `<!-- @action: flow_name -->` | What action to take | POLICY.MD |
| `<!-- @confidence_threshold: 0.85 -->` | Required confidence | POLICY.MD |
| `<!-- @rule: code -->` | Conversation rule | POLICY.MD |
| `<!-- @priority: 1-10 -->` | Rule priority | POLICY.MD |
| `<!-- @applies_to: pattern -->` | When rule applies | POLICY.MD |
| `<!-- @identity -->` | Agent identity section | CONFIG.MD |
| `<!-- @thresholds -->` | Scoring thresholds | POLICY.MD |
| `<!-- @template: code -->` | Response template | POLICY.MD |

---

**Document Version:** 3.0
**Created:** 2026-01-18
**Author:** Claude Code Assistant

---

*This design enables business owners to work with familiar documents while the system maintains a synchronized database for optimal runtime performance.*
