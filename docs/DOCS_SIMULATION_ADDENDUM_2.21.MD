# Simulation Analysis: ID 21 - Mold Remediation Context Issues

**Date:** 2026-01-21
**Simulation ID:** `sim-94f338f5-1cb2-4f60-947c-c197b8d0765e`
**Session ID:** `ca811ddc-0820-4bb4-8fa6-4d5d449c47d6`
**Tenant:** Bulovas Restorations (ID 1)
**Total Turns:** 13

---

## Executive Summary

Simulation ID 21 revealed four distinct issues in the chat agent system when handling a mold remediation inquiry. The issues ranged from critical (phone number handling) to medium severity (ungrounded preparation instructions). All issues have been addressed through code fixes and policy updates.

---

## Initial Scenario

**Customer Prompt:**
> "I found black mold in my bathroom behind the tiles. When I was replacing the caulk around my bathtub, I noticed black spots that spread when I pulled back more tiles. The area is about 3 feet by 2 feet. I'm worried because my kids use this bathroom. What should I do and can you help?"

This scenario triggered multiple responses from the agent that contained ungrounded information.

---

## Issues Identified

### Issue 1: Compliance Checker Phone Inversion (CRITICAL)

**Symptom:**
The compliance checker was flagging the **correct** company phone `(631) 358-5200` as "unauthorized" and suggesting the **placeholder** `(555) 000-0000` instead.

**Server Log Evidence:**
```
Compliance result: passed=false, score=10, violations=3
Violation: type=unauthorized_contact, severity=critical, content=(631) 358-5200
Suggestion: Use authorized phone: (555) 000-0000
```

**Root Cause:**
`compliance.go` was using `config.Audience.EmergencyPhone` directly from the database, which contained a stale placeholder value. The KB.MD file had the correct phone, but the compliance checker wasn't using the validated value.

**Fix Applied:**
Updated `compliance.go` to use `GetValidatedReplacementPhone()`:

```go
// Use GetValidatedReplacementPhone to get the real phone from KB.MD
validatedPhone := ""
if config.Audience != nil {
    validatedPhone = GetValidatedReplacementPhone(config.Audience.EmergencyPhone, config.RawKB)
}
```

---

### Issue 2: Customer Phone Corruption (CRITICAL)

**Symptom:**
Customer provided phone `555-842-0197`, but the agent echoed back `(631) 358-5200` (the company phone).

**Example from Transcript:**
- Customer: "My number is 555-842-0197"
- Agent: "I have your number as (631) 358-5200"

**Root Cause:**
1. `session.CustomerPhone` field existed but was never populated from extracted conversation info
2. System prompt lacked explicit instruction to preserve customer phone verbatim
3. LLM generalized "use the company phone for contact" and incorrectly applied it to customer-provided phones

**Fix Applied:**

1. Updated `service.go` to populate customer info from extracted conversation:
```go
customerInfo := extractCollectedInfo(session.Messages, validatedCompanyPhone)
if customerInfo.Phone != "" && session.CustomerPhone == "" {
    session.CustomerPhone = customerInfo.Phone
}
```

2. Added explicit prompt instruction:
```go
if collectedInfo.Phone != "" {
    sb.WriteString("CRITICAL: When echoing back the customer's phone number, use EXACTLY: " + collectedInfo.Phone + "\n")
    sb.WriteString("This is the CUSTOMER's phone - do NOT replace it with the company phone number!\n")
}
```

3. Added POLICY.MD rule: "Preserve Customer-Provided Information"

---

### Issue 3: Ungrounded Timeline Claims (HIGH)

**Symptom:**
The agent invented specific durations not present in KB.MD:
- "inspection typically takes about 30-45 minutes"
- "remediation takes 1-2 days"
- "1-3 days for remediation"
- "next business day" scheduling

**Server Log Evidence:**
```
Grounding[Service]: Ungrounded - Response contains service details not in KB
Examples: inspection duration details, remediation timeline specifics
```

**Root Cause:**
POLICY.MD had a "No Price Guessing" rule but lacked a corresponding "No Timeline Guessing" rule. The agent filled in timeline information from training data rather than deferring.

**Fix Applied:**

Added to POLICY.MD:
```markdown
### No Timeline Guessing
<!-- @rule: no_timeline_guessing, priority: 2 -->

Never estimate inspection duration, remediation timeline, or arrival times unless explicitly documented in KB.MD.

**Instead say:**
- "Timeline depends on the inspection findings."
- "Our technician will provide a specific timeline after assessment."
```

Added to verifier.go checklist:
```go
sb.WriteString("\n### Timeline & Process Claims\n")
sb.WriteString("- [ ] No duration claims for inspections unless explicitly in KB\n")
sb.WriteString("- [ ] No duration claims for remediation/service unless explicitly in KB\n")
```

---

### Issue 4: Ungrounded Preparation Instructions (MEDIUM)

**Symptom:**
The agent invented detailed preparation steps not in KB.MD:
- "Clear the bathtub and vanity of all personal items"
- "Remove medicines from medicine cabinet"
- "Turn off water to the bathtub"
- "Keep pets and children away"

**Root Cause:**
KB.MD contains safety procedures but no customer preparation instructions. The agent inferred reasonable-sounding prep steps from training data.

**Design Decision:**
User specified preference for "Defer to technician" approach rather than adding extensive prep documentation to KB.MD.

**Fix Applied:**

Added to POLICY.MD:
```markdown
### No Preparation Instructions Unless Documented
<!-- @rule: no_prep_unless_documented, priority: 3 -->

Do not invent customer preparation steps. Only provide prep instructions if explicitly documented in KB.MD.

**Instead say:**
- "Our technician will provide specific preparation guidance when they call to confirm."
```

Added to verifier.go checklist:
```go
sb.WriteString("- [ ] No preparation instructions invented unless documented in KB\n")
```

---

## Files Modified

| File | Change Description |
|------|-------------------|
| `server/router/api/v1/agent/compliance.go` | Use `GetValidatedReplacementPhone()` instead of raw DB value |
| `server/router/api/v1/agent/service.go` | Populate `session.CustomerPhone` from extracted info; add explicit prompt instruction |
| `server/router/api/v1/agent/verifier.go` | Add "Timeline & Process Claims" and "Customer Information Handling" checklist sections |
| `docs/templates/examples/restoration/POLICY.MD` | Add 3 new rules: No Timeline Guessing, No Prep Unless Documented, Preserve Customer Info |

---

## Verification Plan

After implementation, re-run simulation with same initial prompt to verify:

| Test | Expected Outcome |
|------|-----------------|
| Phone Compliance | Agent uses `(631) 358-5200`, compliance passes |
| Customer Phone | Customer gives `555-842-0197`, agent echoes it correctly |
| Timeline Question | When asked "how long?", agent defers to inspection findings |
| Prep Question | When asked "how to prepare?", agent defers to technician guidance |

---

## Lessons Learned

1. **Validation Chain Consistency**: When introducing a validation function (like `GetValidatedReplacementPhone`), it must be used consistently across ALL components that check the same data - not just the primary processor but also the compliance checker.

2. **Customer vs Company Data Separation**: The system must maintain clear separation between customer-provided information and company information. LLMs can conflate these without explicit guardrails.

3. **Negative Space in Policies**: It's not enough to document what the agent CAN say; policies must explicitly document what the agent should NOT say or invent.

4. **Grounding Scope**: "No Price Guessing" naturally implies "No Timeline Guessing" and "No Process Guessing" - these should be treated as a family of related grounding rules.

---

## Related Documents

- Previous simulation analysis: `docs/DOCS_SIMULATION.MD`
- Implementation progress: `docs/DOCS_CHAT_DESIGN_4_IMP_2-PROGRESS.MD`
- Restoration KB: `docs/templates/examples/restoration/KB.MD`
- Restoration Policy: `docs/templates/examples/restoration/POLICY.MD`

---

*End of Analysis*
