# AI Chat Agent: CLI to API Transformation Design v4

**Version:** 4.0
**Previous Version:** [DOCS_CHAT_DESIGN_3.MD](./DOCS_CHAT_DESIGN_3.MD)
**Date:** 2026-01-18

---

## Workflow Overview

### The Big Picture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                              │
│   ONBOARDING (One-Time)                                                      │
│   ══════════════════════                                                     │
│                                                                              │
│   Business Owner provides 2 files:                                           │
│                                                                              │
│   ┌──────────────┐     ┌──────────────┐                                     │
│   │    KB.MD     │     │  POLICY.MD   │                                     │
│   │  (Knowledge  │     │  (Rules &    │                                     │
│   │    Base)     │     │   Intents)   │                                     │
│   └──────┬───────┘     └──────┬───────┘                                     │
│          │                    │                                              │
│          └────────┬───────────┘                                              │
│                   ▼                                                          │
│          ┌────────────────┐                                                  │
│          │  Onboarding    │                                                  │
│          │  API / Admin   │                                                  │
│          │  Page          │                                                  │
│          └────────┬───────┘                                                  │
│                   ▼                                                          │
│          ┌────────────────┐                                                  │
│          │   Database     │                                                  │
│          │   (SQLite)     │                                                  │
│          └────────┬───────┘                                                  │
│                   ▼                                                          │
│          ┌────────────────┐                                                  │
│          │  Chat Agent    │  ← Ready to use                                  │
│          │  LIVE          │                                                  │
│          └────────────────┘                                                  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                              │
│   MAINTENANCE (Ongoing)                                                      │
│   ═════════════════════                                                      │
│                                                                              │
│   Option A: Edit files, re-upload                                            │
│   ┌──────────┐     ┌──────────┐     ┌──────────┐                            │
│   │ Edit     │ ──▶ │ Upload   │ ──▶ │ Database │                            │
│   │ KB.MD    │     │ via API  │     │ Updated  │                            │
│   └──────────┘     └──────────┘     └──────────┘                            │
│                                                                              │
│   Option B: Edit directly in Admin UI                                        │
│   ┌──────────┐     ┌──────────┐                                             │
│   │ Admin    │ ──▶ │ Database │                                             │
│   │ UI       │     │ Updated  │                                             │
│   └──────────┘     └──────────┘                                             │
│                                                                              │
│   Option C: Export for review/backup                                         │
│   ┌──────────┐     ┌──────────┐     ┌──────────┐                            │
│   │ Database │ ──▶ │ Export   │ ──▶ │ KB.MD &  │                            │
│   │          │     │ API      │     │ POLICY.MD│                            │
│   └──────────┘     └──────────┘     └──────────┘                            │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Workflow Summary

| Phase | Input | Action | Output |
|-------|-------|--------|--------|
| **Onboard** | KB.MD + POLICY.MD | Upload via API/Admin | Live chat agent |
| **Update (files)** | Edited KB.MD or POLICY.MD | Re-upload | Database updated |
| **Update (UI)** | Admin UI edits | Save | Database updated |
| **Export** | Request via API/Admin | Generate | KB.MD + POLICY.MD |

---

## Part 1: Two Files Only

### 1.1 File Responsibilities

| File | Contains | Purpose |
|------|----------|---------|
| **KB.MD** | Company info, contact, services, coverage, FAQs, safety protocols | What the agent knows |
| **POLICY.MD** | Identity, tone, intents, rules, thresholds | How the agent behaves |

**That's it. No other files required.**

### 1.2 KB.MD Structure

```markdown
# [Company Name] Knowledge Base

## Company Overview
[Company name, description, certifications, availability]

## Contact Information
| Type | Value |
|------|-------|
| Emergency Phone | (555) 123-4567 |
| Email | info@company.com |

## Services
### Service Name
<!-- @service: service_code, emergency: true/false -->
[Description]

## Services We Don't Provide
### Excluded Service
<!-- @exclusion: code, exception: "unless X" -->
[Explanation and referral]

## Service Areas
### Areas We Serve
<!-- @coverage: include -->
[List of areas]

### Areas We Don't Serve
<!-- @coverage: exclude -->
[List of areas]

## FAQs
### Question?
<!-- @faq: code -->
[Answer]

## Emergency Procedures
### Emergency Type
<!-- @safety: code, triggers: intent1, intent2 -->
[Safety steps]
```

### 1.3 POLICY.MD Structure

```markdown
# [Company Name] Agent Policy

## Identity
<!-- @identity -->
- **Role:** [Role name]
- **Tone:** [Tone description]
- **Brand Voice:** "[Voice quote]"

## Intents
### Intent Name
<!-- @intent: code, category: emergency/standard/meta, urgency: 1-5, action: flow_name -->
**Description:** [What this intent means]
**Examples:** [List of matching phrases]
**Not This:** [List of non-matching phrases]

## Rules
### Rule Name
<!-- @rule: code, priority: 1-10 -->
[Rule description]

## Thresholds
<!-- @thresholds -->
| Metric | Value |
|--------|-------|
| Emergency Urgency | 4 |
| Escalation Confidence | 0.85 |
```

---

## Part 2: Onboarding Flow

### 2.1 API Endpoint

```http
POST /api/v1/agent/onboard
Content-Type: multipart/form-data

tenant_slug: "acme-restoration"
kb_file: [KB.MD file upload]
policy_file: [POLICY.MD file upload]
```

**Response:**
```json
{
  "success": true,
  "tenant_id": 1,
  "tenant_slug": "acme-restoration",
  "parsed": {
    "company_name": "ACME Restoration Inc.",
    "services": 5,
    "intents": 8,
    "coverage_areas": 12
  },
  "warnings": [
    "Intent 'booking_request' has no examples - accuracy may be lower"
  ],
  "agent_url": "/api/v1/agent/acme-restoration/chat"
}
```

### 2.2 Onboarding Process (Internal)

```
┌─────────────────────────────────────────────────────────────────┐
│  ONBOARDING PIPELINE                                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. RECEIVE FILES                                                │
│     └─▶ Validate file formats (must be .md or .txt)             │
│                                                                  │
│  2. PARSE KB.MD                                                  │
│     └─▶ Extract: company info, contact, services,               │
│         exclusions, coverage, FAQs, safety protocols            │
│                                                                  │
│  3. PARSE POLICY.MD                                              │
│     └─▶ Extract: identity, intents, rules, thresholds           │
│                                                                  │
│  4. VALIDATE                                                     │
│     └─▶ Check required fields present                           │
│     └─▶ Check intent definitions have descriptions              │
│     └─▶ Check emergency phone exists                            │
│     └─▶ Return warnings for optional missing items              │
│                                                                  │
│  5. CREATE TENANT                                                │
│     └─▶ Insert into agent_tenants table                         │
│                                                                  │
│  6. POPULATE DATABASE                                            │
│     └─▶ Insert services, intents, coverage, etc.                │
│     └─▶ Store original file content for export                  │
│                                                                  │
│  7. ACTIVATE                                                     │
│     └─▶ Set tenant status to active                             │
│     └─▶ Agent is now live                                       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 2.3 Onboarding Admin Page (Future UI)

```
┌─────────────────────────────────────────────────────────────────┐
│  Onboard New Company                                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Company Slug: [acme-restoration        ]                       │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                                                          │    │
│  │   Drag & drop KB.MD here                                │    │
│  │   or click to browse                                     │    │
│  │                                                          │    │
│  │   ✓ KB.MD uploaded (24 KB)                              │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                                                          │    │
│  │   Drag & drop POLICY.MD here                            │    │
│  │   or click to browse                                     │    │
│  │                                                          │    │
│  │   ✓ POLICY.MD uploaded (8 KB)                           │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Preview                                                 │    │
│  │  ────────                                                │    │
│  │  Company: ACME Restoration Inc.                         │    │
│  │  Services: 5 (3 emergency-capable)                      │    │
│  │  Intents: 8 defined                                      │    │
│  │  Coverage: Nassau County, Suffolk County                │    │
│  │                                                          │    │
│  │  ⚠ Warning: Intent 'booking' has no examples           │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  [Cancel]                              [Create Agent]           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Part 3: Two-Way Sync Design

### 3.1 Sync Modes

| Mode | Direction | When to Use |
|------|-----------|-------------|
| **Import** | Files → Database | Onboarding, bulk updates |
| **Export** | Database → Files | Backup, review, handoff |
| **Update** | Files → Database (merge) | Incremental changes |

### 3.2 How It Works

**Database is the runtime source of truth.**
Files are used for:
- Initial onboarding (import)
- Bulk updates (re-import)
- Backup/review (export)

```
┌────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│   SYNC LOGIC                                                                │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │                                                                      │  │
│   │  IMPORT (Files → DB)                                                 │  │
│   │  ════════════════════                                                │  │
│   │                                                                      │  │
│   │  1. Parse files                                                      │  │
│   │  2. Validate content                                                 │  │
│   │  3. Compare with existing DB data                                    │  │
│   │     • New items → INSERT                                             │  │
│   │     • Changed items → UPDATE                                         │  │
│   │     • Missing from file → Mark inactive (soft delete)                │  │
│   │  4. Store file hash for change detection                             │  │
│   │                                                                      │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │                                                                      │  │
│   │  EXPORT (DB → Files)                                                 │  │
│   │  ════════════════════                                                │  │
│   │                                                                      │  │
│   │  1. Read all active data from DB                                     │  │
│   │  2. Generate markdown with annotations                               │  │
│   │  3. Return as downloadable files                                     │  │
│   │                                                                      │  │
│   │  Use cases:                                                          │  │
│   │  • "Give me the current KB for review"                               │  │
│   │  • "Export for another developer to edit"                            │  │
│   │  • "Backup before major changes"                                     │  │
│   │                                                                      │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└────────────────────────────────────────────────────────────────────────────┘
```

### 3.3 No Conflicts by Design

The system avoids complex conflicts by using **last-write-wins**:

| Scenario | What Happens |
|----------|--------------|
| Edit files, upload | File content replaces DB content |
| Edit in Admin UI | DB updated directly |
| Edit both, upload | File content wins (DB changes overwritten) |
| Want to preserve DB edits? | Export first, then merge manually |

**Why this is simpler:**
- No merge conflicts to resolve
- Clear mental model: "Upload = Replace"
- Export before upload if you need to preserve changes

### 3.4 API Endpoints

```http
# Onboarding
POST   /api/v1/agent/onboard                    # Create new tenant from files

# Updates
POST   /api/v1/agent/:slug/import               # Re-import files (update)
POST   /api/v1/agent/:slug/import/preview       # Preview what would change

# Export
GET    /api/v1/agent/:slug/export               # Export DB to files
GET    /api/v1/agent/:slug/export/kb            # Export KB.MD only
GET    /api/v1/agent/:slug/export/policy        # Export POLICY.MD only

# Admin (direct DB edits)
GET    /api/v1/agent/:slug/config               # Get current config
PUT    /api/v1/agent/:slug/config               # Update config
GET    /api/v1/agent/:slug/intents              # List intents
PUT    /api/v1/agent/:slug/intents/:code        # Update intent
# ... etc for services, coverage, etc.
```

---

## Part 4: Maintenance Workflows

### 4.1 Workflow A: Edit Files (Recommended for Large Changes)

```
Business Owner                     System
─────────────────                 ────────
     │                               │
     │  1. Request export            │
     │  ─────────────────────────▶   │
     │                               │
     │  2. Receive KB.MD, POLICY.MD  │
     │  ◀─────────────────────────   │
     │                               │
     │  3. Edit files locally        │
     │  (VS Code, Word, etc.)        │
     │                               │
     │  4. Upload edited files       │
     │  ─────────────────────────▶   │
     │                               │
     │                    5. Parse & validate
     │                    6. Show preview
     │                               │
     │  7. Confirm changes           │
     │  ─────────────────────────▶   │
     │                               │
     │                    8. Update database
     │                    9. Agent updated
     │                               │
     │  Done ✓                       │
     │                               │
```

### 4.2 Workflow B: Admin UI (Recommended for Small Changes)

```
Business Owner                     System
─────────────────                 ────────
     │                               │
     │  1. Open Admin UI             │
     │  ─────────────────────────▶   │
     │                               │
     │  2. See current intents       │
     │  ◀─────────────────────────   │
     │                               │
     │  3. Edit "emergency_water"    │
     │     - Add new example         │
     │     - Click Save              │
     │  ─────────────────────────▶   │
     │                               │
     │                    4. Validate
     │                    5. Update database
     │                    6. Agent updated
     │                               │
     │  Done ✓                       │
     │                               │
```

### 4.3 When to Use Which

| Situation | Recommended Approach |
|-----------|---------------------|
| Initial setup | Files (onboard API) |
| Add one FAQ | Admin UI |
| Add new service with safety protocol | Files (easier to write markdown) |
| Change emergency phone | Admin UI |
| Major restructure | Files |
| Quick fix to intent example | Admin UI |
| Onboard similar company | Export → Edit → Onboard new |

---

## Part 5: Simplified Database Schema

### 5.1 Core Tables

```sql
-- Tenants
CREATE TABLE agent_tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,              -- 'acme-restoration'
    company_name TEXT NOT NULL,
    vertical TEXT,                          -- 'restoration', 'hvac', 'plumbing'
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Configuration (from POLICY.MD @identity and @thresholds)
CREATE TABLE agent_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER UNIQUE NOT NULL REFERENCES agent_tenants(id),

    -- Identity (from POLICY.MD)
    role TEXT NOT NULL,
    tone TEXT NOT NULL,
    brand_voice TEXT,
    constraints JSON,                       -- ["Never panic", "Short sentences"]

    -- Contact (from KB.MD)
    emergency_phone TEXT NOT NULL,
    secondary_phones JSON,
    email TEXT,
    address TEXT,

    -- Thresholds (from POLICY.MD)
    emergency_urgency INTEGER DEFAULT 4,
    escalation_confidence REAL DEFAULT 0.85,
    service_match_confidence REAL DEFAULT 0.70,

    -- Conversation settings
    max_turns INTEGER DEFAULT 10,
    session_timeout_minutes INTEGER DEFAULT 30,

    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Services (from KB.MD @service)
CREATE TABLE agent_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    is_emergency BOOLEAN DEFAULT FALSE,
    response_time TEXT,
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, code)
);

-- Service Exclusions (from KB.MD @exclusion)
CREATE TABLE agent_exclusions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    exception_rule TEXT,                    -- "unless it caused water damage"
    referral TEXT,                          -- "Contact a licensed plumber"
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, code)
);

-- Coverage Areas (from KB.MD @coverage)
CREATE TABLE agent_coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    area_type TEXT NOT NULL,                -- 'county', 'city', 'state'
    area_name TEXT NOT NULL,
    is_included BOOLEAN NOT NULL,           -- TRUE = we serve, FALSE = excluded

    UNIQUE(tenant_id, area_type, area_name)
);

-- Intents (from POLICY.MD @intent)
CREATE TABLE agent_intents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER REFERENCES agent_tenants(id),  -- NULL = global
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,                 -- 'emergency', 'standard', 'meta'
    description TEXT NOT NULL,
    examples JSON,                          -- ["example 1", "example 2"]
    counter_examples JSON,                  -- ["not this", "not that"]
    urgency INTEGER,                        -- 1-5, NULL for non-urgency intents
    action TEXT NOT NULL,                   -- 'emergency_flow', 'standard_flow'
    confidence_threshold REAL,
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(COALESCE(tenant_id, 0), code)
);

-- Rules (from POLICY.MD @rule)
CREATE TABLE agent_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    priority INTEGER DEFAULT 5,
    applies_to TEXT,                        -- 'emergency_*', 'all', specific intent
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, code)
);

-- Safety Protocols (from KB.MD @safety)
CREATE TABLE agent_safety (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    trigger_intents JSON NOT NULL,          -- ["emergency_water", "emergency_flood"]
    instructions JSON NOT NULL,             -- ["Step 1", "Step 2", "Step 3"]
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, code)
);

-- KB Sections (from KB.MD - general content and FAQs)
CREATE TABLE agent_kb_sections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    code TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    section_type TEXT NOT NULL,             -- 'overview', 'faq', 'service_detail'
    trigger_intents JSON,                   -- When to include in prompt
    is_active BOOLEAN DEFAULT TRUE,

    UNIQUE(tenant_id, code)
);

-- Source files (for export)
CREATE TABLE agent_source_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id),
    file_type TEXT NOT NULL,                -- 'kb' or 'policy'
    content TEXT NOT NULL,                  -- Original file content
    content_hash TEXT NOT NULL,             -- For change detection
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

---

## Part 6: Implementation Summary

### 6.1 What Gets Built

| Component | Purpose | Priority |
|-----------|---------|----------|
| **Parser** | MD files → structured data | P0 |
| **Onboard API** | POST /api/v1/agent/onboard | P0 |
| **Chat API** | POST /api/v1/agent/:slug/chat | P0 |
| **Export API** | GET /api/v1/agent/:slug/export | P1 |
| **Import API** | POST /api/v1/agent/:slug/import | P1 |
| **Admin APIs** | CRUD for intents, services, etc. | P1 |
| **Onboard UI** | Web page for file upload | P2 |
| **Admin UI** | Web page for editing | P2 |

### 6.2 Onboarding Checklist

For a new company, business owner provides:

- [ ] **KB.MD** containing:
  - [ ] Company name and overview
  - [ ] Emergency phone number
  - [ ] At least one service
  - [ ] Service areas
  - [ ] Safety protocols (if emergency services)

- [ ] **POLICY.MD** containing:
  - [ ] Agent identity (role, tone)
  - [ ] At least one intent definition
  - [ ] Thresholds (or use defaults)

**Time to go live: Upload 2 files → Agent ready**

---

## Part 7: Cognitive Load Minimization

### 7.1 For Business Owners

| Task | Complexity | How |
|------|------------|-----|
| Onboard | Low | Upload 2 markdown files |
| Add FAQ | Low | Edit KB.MD or use Admin UI |
| Add intent | Medium | Edit POLICY.MD with examples |
| Update phone | Low | Admin UI, one field |
| Review config | Low | Export to readable markdown |

### 7.2 For Developers/Ops

| Task | Complexity | How |
|------|------------|-----|
| Deploy new tenant | Low | API call with 2 files |
| Debug intent | Low | Check examples in DB/export |
| Bulk update | Low | Edit files, re-import |
| Rollback | Low | Re-import previous file version |

### 7.3 Design Decisions That Reduce Complexity

| Decision | Benefit |
|----------|---------|
| Only 2 files | Nothing else to learn or manage |
| Markdown format | Business owners already know it |
| Annotations are HTML comments | Invisible in normal viewing |
| Last-write-wins | No merge conflicts |
| Database as runtime | Fast, queryable, cacheable |
| Export capability | Always get readable files back |

---

## Appendix: Quick Reference

### Annotation Cheat Sheet

**In KB.MD:**
```markdown
<!-- @service: code, emergency: true -->
<!-- @exclusion: code, exception: "unless X" -->
<!-- @coverage: include -->
<!-- @coverage: exclude -->
<!-- @faq: code -->
<!-- @safety: code, triggers: intent1, intent2 -->
```

**In POLICY.MD:**
```markdown
<!-- @identity -->
<!-- @intent: code, category: emergency, urgency: 5, action: emergency_flow -->
<!-- @rule: code, priority: 1 -->
<!-- @thresholds -->
```

### API Quick Reference

```bash
# Onboard new company
curl -X POST /api/v1/agent/onboard \
  -F "tenant_slug=acme" \
  -F "kb_file=@KB.MD" \
  -F "policy_file=@POLICY.MD"

# Chat with agent
curl -X POST /api/v1/agent/acme/chat \
  -d '{"message": "My basement is flooding"}'

# Export current config
curl /api/v1/agent/acme/export > backup.zip

# Re-import after edits
curl -X POST /api/v1/agent/acme/import \
  -F "kb_file=@KB.MD" \
  -F "policy_file=@POLICY.MD"
```

---

**Document Version:** 4.0
**Created:** 2026-01-18
**Author:** Claude Code Assistant

---

*Two files in. Chat agent out. Edit anytime.*
