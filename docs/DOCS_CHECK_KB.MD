# KB.MD Format Requirements for RAG Indexing

## Overview

This document explains the **required format** for KB.MD files to be properly parsed and indexed by the RAG pipeline. The parser extracts content using **HTML comment annotations** - without these annotations, no chunks are created.

---

## Problem Analysis: Iglesia ni Cristo KB.MD

### Current State (INVALID)

The file at `docs/templates/examples/inc/KB.MD` is **NOT** in the correct format:

```
Project Structure:
├── https_iglesianicristo.net.md (Size: 6.19kb; Lines: 120)
├── https_iglesianicristo.net_2026_01_18_.md (Size: 6.19kb; Lines: 120)
├── https_iglesianicristo.net_about-us_.md (Size: 4.01kb; Lines: 70)
...
```

**Issues:**
| Problem | Impact |
|---------|--------|
| File is 1.7MB | Too large, likely raw concatenated data |
| Content is web scrapes | Not structured knowledge base |
| No HTML comment annotations | Parser cannot extract anything |
| Result | **0 chunks indexed** |

### Verification

```bash
# Check for annotations in INC KB.MD
grep -c "<!-- @" docs/templates/examples/inc/KB.MD
# Result: 0

# Compare with working restoration KB.MD
grep -c "<!-- @" docs/templates/examples/restoration/KB.MD
# Result: 15+
```

---

## Required Format

### Core Principle

The parser (`server/router/api/v1/agent/parser.go`) uses a regex pattern to find HTML comment annotations:

```go
// parser.go:67
annotationPattern := regexp.MustCompile(`<!--\s*@(\w+)(?::\s*([^>]*))?\s*-->`)
```

This matches patterns like:
- `<!-- @service: water_damage, emergency: true -->`
- `<!-- @faq: response_time -->`
- `<!-- @section: about_us, type: general -->`

**Without these annotations, the parser extracts NOTHING.**

---

## Annotation Reference

### @service - Service Definition

**Purpose:** Define a service the company provides

**Syntax:**
```markdown
### Service Name
<!-- @service: code, emergency: true/false -->

Service description here.

**Response Time:** X minutes
```

**Parameters:**
| Parameter | Required | Values | Description |
|-----------|----------|--------|-------------|
| `code` | Yes | string | Unique service identifier (e.g., `water_damage`) |
| `emergency` | No | `true`/`false` | Is this an emergency service? |

**Example:**
```markdown
### Water Damage Restoration
<!-- @service: water_damage, emergency: true -->

We provide emergency water damage restoration services including water extraction, drying, and restoration.

**Common Causes:**
- Flooding and storms
- Burst or leaking pipes
- Appliance failures

**Response Time:** 30 minutes for emergencies
```

---

### @exclusion - Services NOT Provided

**Purpose:** Define services the company does NOT offer

**Syntax:**
```markdown
### Service Name
<!-- @exclusion: code, exception: "exception text" -->

Explanation of why this is not provided.

**Recommendation:** Alternative suggestion
```

**Parameters:**
| Parameter | Required | Values | Description |
|-----------|----------|--------|-------------|
| `code` | Yes | string | Unique exclusion identifier |
| `exception` | No | string | When this exclusion doesn't apply |

**Example:**
```markdown
### General Plumbing
<!-- @exclusion: general_plumbing, exception: "unless it caused water damage" -->

We specialize in water damage restoration, not routine plumbing repairs.

**Recommendation:** Contact a licensed plumber for routine issues.
```

---

### @faq - Frequently Asked Question

**Purpose:** Define a FAQ entry

**Syntax:**
```markdown
### Question text here?
<!-- @faq: code -->

Answer to the question.
```

**Parameters:**
| Parameter | Required | Values | Description |
|-----------|----------|--------|-------------|
| `code` | Yes | string | Unique FAQ identifier |

**Example:**
```markdown
### How quickly can you respond to an emergency?
<!-- @faq: response_time -->

For emergencies, we typically arrive within 30 minutes. Our 24/7 dispatch team ensures rapid response any time of day or night.
```

---

### @coverage - Service Areas

**Purpose:** Define geographic coverage

**Syntax:**
```markdown
### Areas We Serve
<!-- @coverage: include -->

**Region:** City1, City2, City3

### Areas Outside Coverage
<!-- @coverage: exclude -->

- Area not served
```

**Parameters:**
| Parameter | Required | Values | Description |
|-----------|----------|--------|-------------|
| `code` | Yes | `include`/`exclude` | Is this area served? |

**Example:**
```markdown
### Areas We Serve
<!-- @coverage: include -->

**Metro Manila:** Quezon City, Makati, Pasig, Taguig, Mandaluyong
**Cebu:** Cebu City, Mandaue, Lapu-Lapu

### Areas Outside Our Coverage
<!-- @coverage: exclude -->

We do not currently service:
- Mindanao
- Palawan
- Batanes
```

---

### @safety - Safety Protocols

**Purpose:** Define emergency safety procedures

**Syntax:**
```markdown
### Protocol Name
<!-- @safety: code, triggers: intent1, intent2 -->

1. **First step** - Description
2. **Second step** - Description
```

**Parameters:**
| Parameter | Required | Values | Description |
|-----------|----------|--------|-------------|
| `code` | Yes | string | Unique safety protocol identifier |
| `triggers` | No | comma-separated | Intents that trigger this protocol |

**Example:**
```markdown
### Gas Leak Emergency
<!-- @safety: gas_leak, triggers: emergency_gas, smell_gas -->

1. **Evacuate immediately** - Leave the building
2. **Do not use electronics** - No switches, phones, or appliances
3. **Call emergency services** - From outside the building
4. **Do not re-enter** - Wait for clearance
```

---

### @section - General KB Section

**Purpose:** Define a general knowledge base section (catches content that doesn't fit other types)

**Syntax:**
```markdown
## Section Title
<!-- @section: code, type: section_type -->

Content here.
```

**Parameters:**
| Parameter | Required | Values | Description |
|-----------|----------|--------|-------------|
| `code` | Yes | string | Unique section identifier |
| `type` | No | string | Section type (e.g., `general`, `doctrine`, `history`) |

**Example:**
```markdown
## About the Church
<!-- @section: about_inc, type: general -->

The Iglesia Ni Cristo (Church of Christ) is a Christian denomination that was registered in the Philippines on July 27, 1914 by Felix Y. Manalo.

## Core Beliefs
<!-- @section: beliefs, type: doctrine -->

The church teaches that the Bible is the sole basis for faith and practice.
```

---

## Complete KB.MD Template

```markdown
# [Company/Organization Name] Knowledge Base

> **AI Chat Agent Reference**
> Last Updated: [Date]

---

## Company Overview
<!-- @section: overview, type: general -->

Brief description of the company/organization.

---

## Services

### Service One
<!-- @service: service_one, emergency: true -->

Description of service one.

**Response Time:** X minutes

---

### Service Two
<!-- @service: service_two, emergency: false -->

Description of service two.

---

## Services We Don't Provide

### Excluded Service
<!-- @exclusion: excluded_service, exception: "unless condition" -->

Explanation.

**Recommendation:** Alternative suggestion.

---

## Service Areas

### Areas We Serve
<!-- @coverage: include -->

**Region 1:** City1, City2, City3
**Region 2:** City4, City5, City6

---

### Areas Outside Our Coverage
<!-- @coverage: exclude -->

- Area 1
- Area 2

---

## Frequently Asked Questions

### Question One?
<!-- @faq: faq_001 -->

Answer one.

---

### Question Two?
<!-- @faq: faq_002 -->

Answer two.

---

## Safety Protocols

### Emergency Protocol
<!-- @safety: emergency_001, triggers: emergency_intent -->

1. **Step one**
2. **Step two**
3. **Step three**

---
```

---

## Validation Checklist

Before uploading a KB.MD file, verify:

- [ ] File has `<!-- @service: ... -->` annotations for each service
- [ ] File has `<!-- @faq: ... -->` annotations for each FAQ
- [ ] File has `<!-- @section: ... -->` annotations for general content
- [ ] Each annotation has a unique `code` parameter
- [ ] File is properly formatted markdown
- [ ] File size is reasonable (< 100KB typically)

### Quick Validation Command

```bash
# Count annotations in your KB.MD file
grep -c "<!-- @" your_kb.md

# Should return a number > 0
# If 0, no content will be indexed
```

---

## Converting Raw Content to KB.MD Format

If you have raw content (like web scrapes), you need to:

### Step 1: Organize Content by Category

Group your content into:
- Services/Products
- FAQs
- Coverage areas
- General information

### Step 2: Add Annotations

For each piece of content, add the appropriate HTML comment annotation BEFORE the content block.

### Step 3: Create Unique Codes

Each annotation needs a unique code (identifier):
- Use lowercase with underscores: `water_damage`, `faq_response_time`
- Keep codes short but descriptive
- Ensure no duplicates within a file

### Example Conversion

**Before (raw content):**
```
The church was founded in 1914. Members can attend worship services twice a week. The central office is located in Quezon City.
```

**After (proper KB.MD format):**
```markdown
## About the Church
<!-- @section: about_church, type: history -->

The church was founded in 1914.

---

## Frequently Asked Questions

### How often are worship services held?
<!-- @faq: worship_frequency -->

Members can attend worship services twice a week.

---

## Locations
<!-- @section: locations, type: general -->

The central office is located in Quezon City.
```

---

## Parser Code Reference

### Key File
`server/router/api/v1/agent/parser.go`

### Annotation Extraction (line 62-133)
```go
func extractAnnotationBlocks(content string) []annotationBlock {
    annotationPattern := regexp.MustCompile(`<!--\s*@(\w+)(?::\s*([^>]*))?\s*-->`)
    matches := annotationPattern.FindAllStringSubmatchIndex(content, -1)
    // ...
}
```

### ParseKB Function (line 166-341)
```go
func (p *Parser) ParseKB(content string, tenantID int32, audienceType string) (*ParsedKB, error) {
    blocks := extractAnnotationBlocks(content)

    for _, block := range blocks {
        switch block.annotationType {
        case "service":
            // Extract service
        case "exclusion":
            // Extract exclusion
        case "faq":
            // Extract FAQ
        case "coverage":
            // Extract coverage
        case "safety":
            // Extract safety protocol
        case "section":
            // Extract general section
        }
    }
}
```

---

## Summary

| Question | Answer |
|----------|--------|
| Why 0 chunks for INC? | KB.MD has no HTML comment annotations |
| What format is required? | Markdown with `<!-- @type: params -->` annotations |
| What annotations are supported? | `@service`, `@exclusion`, `@faq`, `@coverage`, `@safety`, `@section` |
| How to fix? | Convert raw content to proper format with annotations |

---

## Related Documentation

- [DOCS_RAG_REINDEX.MD](./DOCS_RAG_REINDEX.MD) - Reindexing guide
- [DOCS_DEBUG_REBUILD_INDEX.MD](./DOCS_DEBUG_REBUILD_INDEX.MD) - Debugging guide
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture
- Template: `docs/templates/KB.MD` - Full KB.MD template
