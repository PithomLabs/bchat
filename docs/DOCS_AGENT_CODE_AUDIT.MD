# Agent Code Audit: Business Logic Patterns

**Date:** 2026-01-23
**Purpose:** Identify if/then/else patterns in agent code that could be moved to prompt instructions

## Background

The chat agent should be **general-purpose, not tenant-specific**. Per CLAUDE.md:

> The agent's behavior is driven entirely by:
> 1. **KB.MD** - Knowledge base content
> 2. **POLICY.MD** - Rules, identity, tone, intents
> 3. **SCRIPT.MD** - Conversation flow structure

This audit identifies code patterns that violate this principle by hardcoding business logic that should be configurable per tenant.

---

## Classification Framework

| Type | Description | Action |
|------|-------------|--------|
| **Business Logic** | Tenant-specific workflow decisions hardcoded in Go | REMOVE - Move to POLICY.MD/SCRIPT.MD |
| **Output Validation** | Catches LLM hallucinations/errors | KEEP - Valid safety measure |
| **Infrastructure** | Ticket systems, database ops, parsing | KEEP - Generic plumbing |

---

## Findings Summary

| Priority | Count | Action |
|----------|-------|--------|
| HIGH | 3 | Should remove - hardcoded business logic |
| MEDIUM | 4 | Should review - partially move to prompt |
| KEEP | 6+ | Valid output validation / infrastructure |

---

## HIGH Priority Findings

### 1. Escalation Ticket Auto-Creation

**File:** `service.go`, lines 680-695

```go
if classification.PrimaryIntent == "escalation" && GetEscalationTicket(session) == "" {
    customerInfo := map[string]string{
        "name":  session.CustomerName,
        "phone": session.CustomerPhone,
    }
    ticketInfo, err := s.CreateEscalationTicket(ctx, config.TenantID, "supervisor_request", customerInfo, userMessage)
    if err != nil {
        slog.Error("failed to create escalation ticket", "error", err)
    } else {
        SetEscalationTicket(session, ticketInfo.TicketNumber)
    }
}
```

**Issue:** Hardcoded behavior that creates escalation tickets automatically when intent is "escalation". This prevents tenants from having different escalation workflows.

**Recommended Fix:** Move to POLICY.MD:
```markdown
## Escalation Rules
- When customer requests supervisor: Create ticket, provide reference number
- When customer expresses frustration 3+ times: Offer escalation option
```

---

### 2. Out-of-Coverage Session Termination

**File:** `service.go`, lines 699-706

```go
if classification.PrimaryIntent == "out_of_coverage" || session.CoverageStatus == "outside" {
    count := IncrementOutOfCoverageCount(session)
    if count >= 2 {
        session.Phase = "closing"
        slog.Info("out-of-coverage limit reached, closing conversation", "count", count, "session_id", session.ID)
    }
}
```

**Issue:** Hardcoded "close after 2nd out-of-coverage request" logic. Different tenants may want different handling:
- Insurance company: Different escalation paths
- Restoration company: Referral to partner network
- Other verticals: May allow more attempts

**Recommended Fix:** Move to POLICY.MD:
```markdown
## Out-of-Coverage Handling
- First request: Explain coverage area, offer alternatives
- Second request: [tenant-specific behavior]
- Threshold for closing: [configurable]
```

---

### 3. FAQ Exclusion in Emergency

**File:** `service.go`, line 1240

```go
if len(config.FAQs) > 0 && classification.Category != "emergency" {
    sb.WriteString("=== FAQS (Use these for accurate answers) ===\n\n")
    // ... FAQs written to prompt
}
```

**Issue:** Hardcoded: "Don't show FAQs during emergency." This is tenant-specific:
- Some tenants might want emergency FAQs (quick reference)
- Some might want to exclude them for response speed

**Recommended Fix:** Move to POLICY.MD:
```markdown
## Emergency Response Rules
- Include/exclude FAQs: [tenant choice]
- Response style: [concise vs detailed]
```

---

## MEDIUM Priority Findings

### 4. Emergency Phase on Urgency Threshold

**File:** `service.go`, lines 954-958 (also 1211-1219, 1578-1584)

```go
if classification.Urgency >= config.Audience.EmergencyUrgencyThreshold {
    decision.Action = "emergency_flow"
    decision.Phase = "emergency"
}
```

**Issue:** While the threshold is configurable, the phase transition is hardcoded. Different tenants might handle high-urgency cases differently.

**Recommendation:** Keep threshold checking (valid infrastructure), but move consequences to POLICY.MD rules.

---

### 5. Safety Protocol Trigger Logic

**File:** `service.go`, lines 960-970

```go
for _, safety := range config.Safety {
    for _, trigger := range safety.TriggerIntents {
        if trigger == classification.PrimaryIntent {
            decision.SafetyTrigger = safety
            decision.Action = "safety_flow"
            decision.Phase = "safety"
            break
        }
    }
}
```

**Issue:** Automatic phase transition when safety trigger is matched. A restaurant and hospital should handle "medical_emergency" very differently.

**Recommendation:** Keep trigger matching (data lookup), move phase transition orchestration to prompt.

---

### 6. Phase Progression Logic

**File:** `service.go`, lines 981-983

```go
if decision.Phase == "triage" && classification.Confidence > 0.7 {
    decision.Phase = "handshake"
}
```

**Issue:** Hardcoded conversation phase progression. Different industries have different information-gathering requirements.

**Recommendation:** Move to SCRIPT.MD:
```markdown
## Phase Transitions
- Triage to Handshake: When [tenant-specific conditions]
- Handshake to Resolution: When [tenant-specific conditions]
```

---

### 7. Safety Flag Marking Condition

**File:** `service.go`, lines 806-808

```go
if !IsSafetyGiven(session) && (classification.Category == "emergency" || classification.Urgency >= 4) {
    MarkSafetyGiven(session)
}
```

**Issue:** Hardcoded logic that marks safety as "given" based on specific category/urgency thresholds. Prevents tenants from defining their own safety completion criteria.

**Recommendation:** Let POLICY.MD define: "Safety instructions are complete when [condition]"

---

## KEEP - Valid Patterns

### Response Sanitization

**File:** `service.go`, lines 739, 755-762, 782-783

```go
response = SanitizeResponse(response)
response = CorrectContactsInResponse(response, validatedPhone)
response = CorrectEmailsInResponse(response, config.Audience.Email)
```

**Why Keep:** Output validation that catches LLM hallucinations. Not business logic - all tenants benefit from removing placeholder phone numbers and hallucinated system tags.

---

### Ticket Generation Logic

**File:** `service.go`, lines 1834-1921

```go
var ticketNumber string
if ticketType == "complaint" {
    ticketNumber = GenerateComplaintNumber()
} else {
    ticketNumber = GenerateTicketNumber()
}
```

**Why Keep:** Infrastructure logic. Ticket numbering format is consistent for all tenants. This is ticket system mechanics, not business logic.

---

### Intent Classification

**File:** `service.go`, lines 847-932

**Why Keep:** Data-driven from tenant config. Each tenant defines their own intents in POLICY.MD. The code is general-purpose; intents come from configuration.

---

### Parser Logic

**File:** `parser.go`

**Why Keep:** Configuration reading, not business logic. Extracts structured data from MD files. No hardcoded workflow decisions.

---

### Compliance Checking

**File:** `compliance.go`, lines 40-64

**Why Keep:** Validation logic that compares LLM output against KB. Performing validation, not making business decisions. Checks are data-driven.

---

### Simulation Control

**File:** `simulation.go`, lines 501-504

```go
if lastResponse.Metadata.Phase == "resolved" || lastResponse.Metadata.Phase == "closed" {
    return nil
}
```

**Why Keep:** Simulation infrastructure. Stopping at terminal states is generic for any tenant.

---

## Refactoring Pattern

When addressing findings marked REMOVE or REVIEW, follow this pattern:

### Before (Code-Driven)

```go
// Hardcoded in service.go
if classification.PrimaryIntent == "escalation" {
    s.CreateEscalationTicket(...)
}
```

### After (Prompt-Driven)

**POLICY.MD:**
```markdown
## Escalation Rules
- When customer requests supervisor: Create support ticket
- Provide ticket reference number in response
- Escalation threshold: [tenant configures]
```

**Code:**
```go
// Just passes data to LLM, no conditionals
// LLM reads POLICY.MD rules and decides action
// Response parsing extracts any ticket creation requests
```

---

## Implementation Roadmap

### Phase 1: High Priority (Recommended First)
1. Remove escalation ticket auto-creation logic
2. Remove out-of-coverage counter logic
3. Remove FAQ exclusion by emergency category

### Phase 2: Medium Priority
4. Refactor emergency urgency phase transitions
5. Review safety protocol trigger orchestration
6. Move phase progression to SCRIPT.MD
7. Remove hardcoded safety flag conditions

### Phase 3: Validation
- Test with multiple tenant configurations
- Verify prompt-driven behavior matches previous code behavior
- Monitor for regressions

---

## Related Documents

- `CLAUDE.md` - Agent design principles
- `DOCS_CHAT_DESIGN_4_IMP_2.MD` - Main implementation specification
- `DOCS_CONTACT_SANITIZATION_REFACTOR.MD` - Contact handling refactor (completed)
