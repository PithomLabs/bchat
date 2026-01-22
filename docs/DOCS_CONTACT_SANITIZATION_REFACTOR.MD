# Contact Sanitization Refactor Plan

## Problem Statement

The current `CorrectContacts()` function in `sanitizer.go` uses regex-based post-processing to replace "placeholder" phone numbers (555-XXX-XXXX, 000-XXX-XXXX, etc.) with the company phone. This approach has fundamental flaws:

1. **Context blindness**: Cannot distinguish between company contact info (should correct) and customer-provided info (should preserve)
2. **False positives**: Customer phone "555-123-4567" gets incorrectly replaced with company phone
3. **Code complexity**: Adding context-aware regex patterns increases maintenance burden
4. **Violates design principle**: Adds tenant-specific logic to generic agent code

### Example Failure

```
Customer: my name's Mark Reynolds, you can reach me at 555-123-4567
Agent: I have recorded your phone number (631) 358-5200  ← WRONG!
```

---

## Design Principle

From CLAUDE.md:

> **The chat agent must be GENERAL PURPOSE, not tenant-specific.**
> The agent's behavior is driven entirely by:
> 1. **KB.MD** - Knowledge base content
> 2. **POLICY.MD** - Rules, identity, tone, intents
> 3. **SCRIPT.MD** - Conversation flow structure

**Goal**: Move contact handling logic from code → configuration/prompts.

---

## Proposed Solution: Prompt-First with Minimal Safety Net

### Phase 1: Strengthen System Prompt (Primary Fix)

The LLM should never hallucinate contact info in the first place. Enhance `buildSystemPrompt()` with explicit instructions:

**Add to Critical Constraints section:**

```
CONTACT INFORMATION RULES:
1. COMPANY PHONE: When providing YOUR contact number, use ONLY: [validated_phone]
   - Never invent or guess phone numbers
   - Never use placeholder patterns (555, 000, 999)

2. CUSTOMER PHONE: When echoing back a customer's phone number:
   - Use EXACTLY what they provided, character for character
   - Do NOT modify, format, or "correct" their number
   - Example: Customer says "555-123-4567" → You say "555-123-4567"

3. DISTINCTION: Company phone ≠ Customer phone
   - "Call us at..." → Use company phone
   - "Your number is..." → Use customer's exact input
```

**Location**: `service.go`, `buildSystemPrompt()` function, Section 1 (Critical Constraints)

### Phase 2: Simplify CorrectContacts() (Safety Net Only)

Reduce `CorrectContacts()` to catch only **impossible** phone numbers that could never be valid customer input:

**Keep only these patterns:**
- `000-000-0000` (all zeros)
- `999-999-9999` (all nines)
- `123-456-7890` (sequential test number)

**Remove these patterns:**
- `555-XXX-XXXX` (could be valid customer phone)
- `000-XXX-XXXX` (partial zeros could be customer typo)
- `999-XXX-XXXX` (partial nines could be customer typo)

**New implementation:**

```go
func (s *Sanitizer) CorrectContacts(response string, replacementPhone string) string {
    if replacementPhone == "" {
        return response
    }

    // Only correct IMPOSSIBLE phone numbers (obvious test patterns)
    impossiblePatterns := []string{
        `\(?000\)?[-.\s]?000[-.\s]?0000`,  // 000-000-0000
        `\(?999\)?[-.\s]?999[-.\s]?9999`,  // 999-999-9999
        `\(?123\)?[-.\s]?456[-.\s]?7890`,  // 123-456-7890
    }

    result := response
    for _, pattern := range impossiblePatterns {
        re, err := regexp.Compile(pattern)
        if err != nil {
            continue
        }
        result = re.ReplaceAllString(result, replacementPhone)
    }

    return result
}
```

### Phase 3: Add POLICY.MD Guidance (Optional)

Allow tenants to configure contact handling behavior via POLICY.MD:

```markdown
## Contact Handling

### Company Contact
- Phone: (631) 358-5200
- Always use this exact number when directing customers to call

### Customer Information
- Echo back customer-provided phone numbers exactly as given
- Do not attempt to format or correct customer phone numbers
```

This keeps logic in configuration, not code.

---

## Alternative Approaches Considered

### A. Context-Aware Regex (Rejected)

Add patterns to detect company vs customer context:
```go
companyPatterns := []string{"call us at", "our number is"...}
customerPatterns := []string{"your phone", "I recorded"...}
```

**Rejected because:**
- Increases code complexity
- Brittle to phrasing variations
- Still fundamentally a code solution, not configuration

### B. LLM-Based Classification (Rejected)

Use a second LLM call to classify phone numbers.

**Rejected because:**
- Adds latency and cost
- Over-engineered for the problem

### C. Structured Output Tags (Considered)

Have LLM use tags like `<company_phone>` and `<customer_phone>`:
```
Call us at <company_phone/> for emergencies.
I have recorded <customer_phone>555-123-4567</customer_phone>.
```

**Status:** Worth considering for future enhancement, but requires prompt changes and post-processing updates. More invasive than the proposed solution.

### D. Disable CorrectContacts Entirely (Too Aggressive)

Remove all post-processing phone correction.

**Rejected because:**
- LLMs occasionally still hallucinate despite good prompts
- Minimal safety net has low cost and catches obvious errors

---

## Implementation Plan

### Files to Modify

| File | Change | Effort |
|------|--------|--------|
| `server/router/api/v1/agent/service.go` | Enhance prompt in `buildSystemPrompt()` | Small |
| `server/router/api/v1/agent/sanitizer.go` | Simplify `CorrectContacts()` to impossible patterns only | Small |

### Changes Detail

**1. service.go - buildSystemPrompt() (~line 1081)**

Add explicit contact handling rules to Critical Constraints section:

```go
// After existing phone constraint
sb.WriteString("\n")
sb.WriteString("CONTACT INFORMATION HANDLING:\n")
sb.WriteString("- COMPANY CONTACT: When providing YOUR phone number, use ONLY: " + validatedPhone + "\n")
sb.WriteString("- CUSTOMER CONTACT: When echoing back customer's phone, use EXACTLY what they said\n")
sb.WriteString("- NEVER modify, format, or 'correct' a customer-provided phone number\n")
sb.WriteString("- Customer says '555-123-4567' → You respond '555-123-4567' (not the company number)\n")
```

**2. sanitizer.go - CorrectContacts() (~line 164)**

Replace current implementation with minimal safety net version (see Phase 2 above).

---

## Verification

### Test Cases

1. **Customer phone preserved:**
   ```
   Input:  "I have recorded your phone number 555-123-4567"
   Output: "I have recorded your phone number 555-123-4567"  ← UNCHANGED
   ```

2. **Obvious placeholder corrected:**
   ```
   Input:  "Call us at 000-000-0000"
   Output: "Call us at (631) 358-5200"  ← CORRECTED
   ```

3. **Company phone in response (via prompt):**
   ```
   LLM generates: "Call us at (631) 358-5200" directly (no post-processing needed)
   ```

### Build Commands

```bash
task build
task build:backend:rag
```

### Manual Testing

1. Start dev server: `task dev`
2. Test conversation with customer providing 555-XXX-XXXX number
3. Verify agent echoes back exact customer number
4. Test agent providing company contact (should use real number from KB)

---

## Risk Assessment

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| LLM ignores prompt and hallucinates | Low | Minimal safety net catches obvious cases |
| Prompt bloat | Low | Instructions are concise (~4 lines) |
| Breaking existing behavior | Low | Changes are additive to prompt, reductive to sanitizer |

---

## Summary

**Before:** Complex regex replacement that can't distinguish context
**After:**
1. Prompt tells LLM exactly how to handle each case
2. Minimal safety net only catches impossible numbers (000-000-0000, etc.)
3. Logic lives in prompts/configuration, not code

This aligns with the design principle: **configuration-driven behavior, generic code**.
