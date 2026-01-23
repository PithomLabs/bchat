# Agent Architecture: Configuration-Driven Design

**Status:** Active
**Date:** 2026-01-23
**Related Documents:**
- [CLAUDE.md](/CLAUDE.md) - Design principles
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG implementation
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment configuration

---

## Overview

The bchat agent system is a **configuration-driven, vertically-agnostic** AI chat platform. While existing examples focus on service businesses (HVAC, plumbing, water restoration), the underlying architecture is designed to support **any knowledge-intensive use case** including Q&A systems, technical support, HR portals, and legal compliance.

**Key Principle:** The agent's behavior is defined entirely by three configuration files (KB.MD, POLICY.MD, SCRIPT.MD), with no hardcoded business logic in the codebase.

---

## Table of Contents

1. [Design Principle](#design-principle)
2. [Service Business Origins](#service-business-origins)
3. [The Annotation-Based Parsing System](#the-annotation-based-parsing-system)
4. [Flexible Architecture Elements](#flexible-architecture-elements)
5. [Adaptation Examples](#adaptation-examples)
6. [What Requires Code Changes vs Configuration](#what-requires-code-changes-vs-configuration)
7. [Anti-Patterns to Avoid](#anti-patterns-to-avoid)

---

## Design Principle

From the project's core documentation (CLAUDE.md):

> **CRITICAL: Chat Agent Design Principle**
>
> The chat agent must be GENERAL PURPOSE, not tenant-specific.
>
> The agent's behavior is driven entirely by:
> 1. **KB.MD** - Knowledge base content (services, FAQs, coverage areas)
> 2. **POLICY.MD** - Rules, identity, tone, intents
> 3. **SCRIPT.MD** - Conversation flow structure
>
> **DO NOT:**
> - Hardcode tenant-specific logic in the codebase
> - Add conditional behavior based on tenant ID or slug
> - Create tenant-specific prompts or handlers
> - Build features that only work for certain verticals
>
> **DO:**
> - Design features that work for ANY tenant configuration
> - Let the KB/Policy/Script files define all tenant-specific behavior
> - Keep the agent code generic and configuration-driven

This principle enables a single codebase to serve unlimited verticals without code changes.

---

## Service Business Origins

The current examples in the codebase focus on service businesses because:

1. **Initial Use Case** - The system was first deployed for water restoration services
2. **Concrete Examples** - Service businesses have clear, testable scenarios (appointments, emergencies, coverage areas)
3. **Template Availability** - Example KB.MD and POLICY.MD files exist for these verticals

### Service-Oriented Content Types

The example files demonstrate these content patterns:

**KB.MD for Service Businesses:**
```markdown
## Services
<!-- @service: water_extraction, emergency: true -->
### Water Extraction
24/7 emergency response for standing water removal...

**Response Time:** 60 minutes or less

---

## Services We Don't Provide
<!-- @exclusion: mold_remediation, exception: "secondary to water damage" -->
### Mold Remediation
We don't provide standalone mold services...

---

## Service Areas
<!-- @coverage: include -->
### Areas We Serve
**Primary:** Phoenix, Scottsdale, Tempe
**Secondary:** Mesa, Chandler, Gilbert
```

**POLICY.MD for Service Businesses:**
```markdown
## Identity
<!-- @identity -->
- **Role:** Virtual assistant for [Company] restoration services
- **Tone:** Professional, empathetic, urgent when needed

---

## Intents
<!-- @intent: request_appointment, category: standard, urgency: 3, action: schedule_flow -->
### Request Appointment
Customer wants to schedule a service visit...

<!-- @intent: emergency_situation, category: emergency, urgency: 5, action: emergency_flow -->
### Emergency Situation
Active water damage requiring immediate response...
```

**SCRIPT.MD for Service Businesses:**
```markdown
## CONVERSATION OPENING
- Greet customer warmly
- Identify if this is emergency or standard inquiry

## DISCOVERY
- What type of damage?
- When did it occur?
- Is it active/ongoing?

## RESOLUTION
- Schedule appointment
- Provide confirmation
- Set expectations
```

These patterns are **examples**, not requirements. The annotation system is generic.

---

## The Annotation-Based Parsing System

The parser (`parser.go`) uses a generic annotation extraction system:

### How Annotations Work

```go
// From parser.go:67
annotationPattern := regexp.MustCompile(`<!--\s*@(\w+)(?::\s*([^>]*))?\s*-->`)
```

This pattern matches any annotation of the form:
```
<!-- @type: params -->
```

Where:
- `@type` can be **any identifier** (service, faq, product, policy, etc.)
- `params` are optional key-value pairs

### Currently Supported Annotation Types

| Annotation | Purpose | Required |
|------------|---------|----------|
| `@service` | Service/product entries | No |
| `@exclusion` | Things NOT offered | No |
| `@coverage` | Geographic/scope areas | No |
| `@faq` | Q&A pairs | No |
| `@safety` | Emergency protocols | No |
| `@section` | Generic content sections | No |
| `@identity` | Agent persona | Recommended |
| `@intent` | User intent categories | Recommended |
| `@rule` | Behavioral rules | No |
| `@thresholds` | Numeric thresholds | No |

### Generic Section Annotation

The `@section` type is fully generic and can hold any content:

```markdown
<!-- @section: products, type: catalog -->
### Product Catalog
Our complete line of products includes...

<!-- @section: procedures, type: internal -->
### Internal Procedures
Step-by-step process for handling returns...

<!-- @section: regulations, type: compliance -->
### GDPR Requirements
Data handling requirements under GDPR...
```

### Adding New Annotation Types

To add a new annotation type:

1. **Add to ParsedKB/ParsedPolicy struct** (parser.go)
2. **Add case in switch statement** (parser.go:184-337)
3. **Add corresponding database table** (if persistence needed)

Or, use `@section` with custom `type` parameter for quick additions without code changes.

---

## Flexible Architecture Elements

### 1. Multi-Tenant Isolation

Every operation is scoped to a tenant:

```go
// From handlers.go - typical pattern
tenant, err := h.service.GetTenantBySlug(ctx, slug)
// All subsequent operations use tenant.ID
```

Each tenant can have:
- Different KB.MD content
- Different POLICY.MD rules
- Different SCRIPT.MD flows
- Different LLM model configuration
- Different API keys

### 2. Multi-Audience Support

Each tenant supports multiple audiences:

| Audience Type | Use Case |
|---------------|----------|
| `internal` | Employee-facing assistant |
| `external` | Customer-facing chatbot |
| Custom types | Any identifier works |

```markdown
<!-- KB.MD for internal audience might include: -->
## Internal Procedures
<!-- @section: escalation, type: internal -->
### Escalation Procedure
For issues requiring manager approval...
```

### 3. Template-Based Prompt Building

The system prompt is built from templates, not conditionals:

```go
// From service.go - simplified
systemPrompt := buildSystemPrompt(
    tenant.Config,        // Generic config
    kb,                   // Parsed from KB.MD
    policy,               // Parsed from POLICY.MD
    script,               // Parsed from SCRIPT.MD
    ragContext,           // Retrieved documents
)
```

There are no `if tenant.Slug == "hvac-company"` checks.

### 4. Content-Agnostic RAG Pipeline

The RAG system indexes any text content:

```go
// From chunker.go - chunks any markdown
chunks := ChunkDocument(content, ChunkConfig{
    ChunkSize:    500,
    ChunkOverlap: 50,
})
```

The vector database stores:
- `Content` - The text chunk
- `TenantID` - Multi-tenant isolation
- `ContentType` - Metadata (service, faq, policy, etc.)
- `AudienceType` - Audience filtering

Hybrid search (vector + BM25) works regardless of content domain.

### 5. Generic Intent System

Intents are defined in POLICY.MD, not code:

```markdown
<!-- For a Q&A system: -->
<!-- @intent: ask_question, category: standard, action: search_kb -->
### Ask Question
User is asking a factual question that can be answered from knowledge base.

**Examples:**
- "What's the return policy?"
- "How do I reset my password?"

<!-- For technical support: -->
<!-- @intent: report_bug, category: standard, action: create_ticket -->
### Report Bug
User is reporting a software issue.

**Examples:**
- "The app crashes when I click save"
- "Error message appears on login"
```

---

## Adaptation Examples

### A) Q&A / Knowledge Base System

**Use Case:** Internal knowledge management, customer FAQ portal

**KB.MD Structure:**
```markdown
# Company Knowledge Base

---

## Categories
<!-- @section: hr_policies, type: category -->
### HR Policies
Human resources policies and procedures.

---

## Frequently Asked Questions

<!-- @faq: vacation_policy -->
### How many vacation days do I get?
Full-time employees receive 15 days PTO in their first year...

<!-- @faq: remote_work -->
### What is the remote work policy?
Employees may work remotely up to 3 days per week...
```

**POLICY.MD Structure:**
```markdown
## Identity
<!-- @identity -->
- **Role:** Knowledge assistant for [Company] employees
- **Tone:** Helpful, professional, concise

---

## Intents

<!-- @intent: ask_factual, category: standard, action: search_kb -->
### Factual Question
User is seeking specific information from the knowledge base.

<!-- @intent: unclear_question, category: meta, action: clarify -->
### Unclear Question
User's question is ambiguous and needs clarification.

<!-- @intent: out_of_scope, category: meta, action: escalate -->
### Out of Scope
Question cannot be answered from available knowledge.
```

**SCRIPT.MD Structure:**
```markdown
## QUESTION UNDERSTANDING
- Identify the topic area
- Clarify ambiguous terms
- Confirm understanding

## ANSWER DELIVERY
- Provide concise answer
- Cite source if available
- Offer related information

## FOLLOW-UP
- Ask if answer was helpful
- Offer to search for more
```

---

### B) Technical Support System

**Use Case:** Software product support, IT helpdesk

**KB.MD Structure:**
```markdown
# Product Support Knowledge Base

---

## Product Documentation

<!-- @section: getting_started, type: documentation -->
### Getting Started Guide
Step-by-step instructions for new users...

<!-- @section: api_reference, type: documentation -->
### API Reference
Endpoint documentation and examples...

---

## Troubleshooting

<!-- @faq: login_issues -->
### I can't log in
Common login issues and solutions:
1. Clear browser cache
2. Reset password
3. Check account status

<!-- @faq: performance_slow -->
### The application is slow
Performance troubleshooting steps...
```

**POLICY.MD Structure:**
```markdown
## Identity
<!-- @identity -->
- **Role:** Technical support specialist for [Product]
- **Tone:** Patient, technical but accessible

---

## Intents

<!-- @intent: bug_report, category: standard, urgency: 3, action: collect_details -->
### Bug Report
User is reporting a software defect.

<!-- @intent: feature_request, category: standard, urgency: 1, action: log_request -->
### Feature Request
User is suggesting a new feature.

<!-- @intent: urgent_outage, category: emergency, urgency: 5, action: escalate_immediately -->
### System Outage
Production system is down affecting business.
```

**SCRIPT.MD Structure:**
```markdown
## ISSUE IDENTIFICATION
- What product/feature?
- What were you trying to do?
- What happened instead?

## DIAGNOSTIC
- Browser/OS version
- Steps to reproduce
- Error messages

## RESOLUTION
- Try known solutions
- Create ticket if unresolved
- Provide ticket number
```

---

### C) HR / Employee Portal

**Use Case:** Benefits inquiries, policy questions, onboarding

**KB.MD Structure:**
```markdown
# HR Knowledge Base

---

## Benefits

<!-- @section: health_insurance, type: benefit -->
### Health Insurance
Medical, dental, and vision coverage options...

<!-- @section: retirement, type: benefit -->
### 401(k) Plan
Company matches 50% up to 6% of salary...

---

## Policies

<!-- @section: code_of_conduct, type: policy -->
### Code of Conduct
Professional behavior expectations...

<!-- @section: harassment, type: policy -->
### Anti-Harassment Policy
Zero tolerance policy details...
```

**POLICY.MD Structure:**
```markdown
## Identity
<!-- @identity -->
- **Role:** HR assistant for [Company] employees
- **Tone:** Supportive, confidential, informative

---

## Intents

<!-- @intent: benefits_question, category: standard, action: search_benefits -->
### Benefits Question
Employee asking about compensation or benefits.

<!-- @intent: report_concern, category: sensitive, urgency: 4, action: hr_escalation -->
### Report Concern
Employee reporting harassment or ethical concern.

---

## Rules

<!-- @rule: confidentiality, priority: 1 -->
### Confidentiality
Never discuss another employee's information. Always maintain privacy.

<!-- @rule: no_legal_advice, priority: 2 -->
### No Legal Advice
Do not provide legal interpretations. Direct to HR or legal department.
```

---

### D) Legal / Compliance System

**Use Case:** Regulatory lookup, compliance guidance, contract Q&A

**KB.MD Structure:**
```markdown
# Compliance Knowledge Base

---

## Regulations

<!-- @section: gdpr, type: regulation -->
### GDPR Requirements
General Data Protection Regulation compliance requirements...

<!-- @section: hipaa, type: regulation -->
### HIPAA Guidelines
Health information privacy requirements...

---

## Procedures

<!-- @section: data_breach, type: procedure -->
### Data Breach Response
72-hour notification requirement...

<!-- @section: audit_prep, type: procedure -->
### Audit Preparation Checklist
Steps to prepare for compliance audit...
```

**POLICY.MD Structure:**
```markdown
## Identity
<!-- @identity -->
- **Role:** Compliance guidance assistant
- **Tone:** Precise, cautious, factual

---

## Intents

<!-- @intent: regulation_lookup, category: standard, action: search_regulations -->
### Regulation Lookup
User seeking specific regulatory information.

<!-- @intent: compliance_check, category: standard, action: checklist_flow -->
### Compliance Check
User wants to verify compliance status.

---

## Rules

<!-- @rule: disclaimer, priority: 1 -->
### Legal Disclaimer
Always clarify this is guidance, not legal advice. Recommend consulting legal counsel for specific situations.

<!-- @rule: cite_sources, priority: 2 -->
### Citation Required
Always cite the specific regulation or policy section when providing guidance.
```

---

## What Requires Code Changes vs Configuration

### Configuration Only (No Code Changes)

| Change | How to Implement |
|--------|------------------|
| New tenant | Create via API/Admin UI |
| New services/products | Add to KB.MD with `@service` |
| New FAQ entries | Add to KB.MD with `@faq` |
| New intents | Add to POLICY.MD with `@intent` |
| Different conversation flow | Modify SCRIPT.MD |
| Different agent persona | Modify POLICY.MD `@identity` |
| Different LLM model | Tenant config or env var |
| Different coverage areas | Add to KB.MD with `@coverage` |
| Custom content sections | Use `@section` annotation |

### Requires Code Changes

| Change | Why Code is Needed |
|--------|-------------------|
| New annotation type with special parsing | Parser switch statement needs new case |
| New database field for content | Schema migration + store methods |
| New integration (e.g., CRM API) | Handler + service implementation |
| New search algorithm | VectorDB implementation |
| New response format | Service/handler changes |

### Extension Points (Designed for Customization)

1. **`@section` annotation** - Generic container for any content
2. **Intent `action` parameter** - Defines response behavior
3. **Rule `priority`** - Controls rule ordering
4. **Threshold configuration** - Numeric tuning

---

## Anti-Patterns to Avoid

### 1. Tenant-Specific Conditionals

```go
// BAD - Never do this
if tenant.Slug == "water-damage-co" {
    // Special handling
}

// GOOD - Use configuration
if tenant.Config.EmergencyEnabled {
    // Configuration-driven behavior
}
```

### 2. Hardcoded Vertical Logic

```go
// BAD - Never do this
func buildPromptForHVAC() string { ... }
func buildPromptForPlumbing() string { ... }

// GOOD - Single generic function
func buildSystemPrompt(kb *ParsedKB, policy *ParsedPolicy) string {
    // Uses KB and Policy content, not vertical knowledge
}
```

### 3. Vertical-Specific Intents in Code

```go
// BAD - Intent categories shouldn't be in code
if intent.Code == "schedule_hvac_maintenance" { ... }

// GOOD - Generic intent handling
if intent.Category == "emergency" && intent.Urgency >= thresholds.Emergency {
    // Generic emergency handling
}
```

### 4. Assumptions About Content Structure

```go
// BAD - Assumes services exist
services := kb.Services // May be empty for Q&A system

// GOOD - Check before use
if len(kb.Services) > 0 {
    // Include services in prompt
}
```

---

## Summary

The bchat agent architecture is designed for **maximum flexibility through configuration**:

1. **No hardcoded verticals** - The code doesn't know about HVAC, plumbing, or any specific industry
2. **Annotation-based parsing** - `<!-- @type: params -->` syntax works for any content type
3. **Generic content types** - Services, FAQs, sections, intents are abstract containers
4. **Multi-tenant/multi-audience** - Complete isolation between configurations
5. **RAG is content-agnostic** - Works with any knowledge base content

To adapt the system for a new use case:
1. Write appropriate KB.MD with your content using existing or `@section` annotations
2. Define intents and rules in POLICY.MD for your domain
3. Create conversation flow in SCRIPT.MD
4. Deploy - no code changes needed

---

*Document Version: 1.0*
*Created: 2026-01-23*
