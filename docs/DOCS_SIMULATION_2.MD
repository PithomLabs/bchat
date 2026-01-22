# Robust Hallucination Prevention Architecture

## Document Version
- **Version:** 1.1
- **Date:** 2026-01-20
- **Status:** Phase 1 Implemented

---

## Implementation Status

### Phase 1: Verifier Layer ✅ COMPLETED

| Component | Status | File |
|-----------|--------|------|
| Verifier core logic | ✅ Done | `server/router/api/v1/agent/verifier.go` |
| VerificationResult types | ✅ Done | `server/router/api/v1/agent/verifier.go` |
| Verification metrics | ✅ Done | `server/router/api/v1/agent/verifier.go` |
| Service integration | ✅ Done | `server/router/api/v1/agent/service.go` |
| POLICY.MD template | ✅ Done | `docs/templates/POLICY.MD` |
| AudienceConfig extensions | ✅ Done | `server/router/api/v1/agent/service.go` |

**How it works:**
1. After response generation, the verifier LLM (GPT-4o-mini) checks the response
2. Verifier checks against KB and Policy using a structured checklist
3. If violations found, verifier provides a corrected response
4. Corrected response replaces original before sending to customer
5. All violations are logged for monitoring

**Configuration:**
- Model: `openai/gpt-4o-mini` (fast, cheap)
- Mode: `enforce` (corrections applied automatically)
- Graceful degradation on errors (original response used)

### Phase 2: Grounded Generation ✅ COMPLETED

| Component | Status | File |
|-----------|--------|------|
| GroundedResponse/GroundedClaim types | ✅ Done | `server/router/api/v1/agent/grounding.go` |
| Grounded generation prompt | ✅ Done | `server/router/api/v1/agent/grounding.go` |
| Citation verification | ✅ Done | `server/router/api/v1/agent/grounding.go` |
| Response assembly | ✅ Done | `server/router/api/v1/agent/grounding.go` |
| Service integration | ✅ Done | `server/router/api/v1/agent/service.go` |
| Grounding metrics | ✅ Done | `server/router/api/v1/agent/grounding.go` |

**How it works:**
1. LLM is prompted to generate structured JSON with claims and citations
2. Each claim must cite a specific KB section (e.g., "KB.MD > Services > Water Damage")
3. Claims are verified against the actual KB content
4. Only claims with valid citations and allowed confidence levels are included
5. Response is assembled from verified claims only
6. Fallback to safe response if no claims pass verification

**Configuration:**
- Model: Same as primary generation model
- Confidence levels: "grounded" and "inferred" allowed by default
- Fallback: Enabled (falls back to regular generation on error)

**Enabling Grounded Generation:**
Currently disabled by default. To enable, modify `UseGroundedGeneration()` in `service.go`:
```go
func (s *Service) UseGroundedGeneration(config *AudienceConfig) bool {
    return true // Enable for all tenants
}
```

### Phase 3: Self-Critique Loop ⏳ PLANNED

Not yet implemented. See design below.

---

## Problem Statement

The current approach to hallucination prevention is not scalable:

```
Current Flow:
LLM Response → Regex Detection → Pattern Replacement → Customer
```

**Issues with current approach:**
1. **Whack-a-mole pattern:** Each new hallucination type requires new code
2. **Brittle regex:** Patterns break with slight variations
3. **No semantic understanding:** Can't catch "We offer 24/7 service" when KB says "business hours only"
4. **Maintenance burden:** Rules accumulate, become unmaintainable
5. **False negatives:** Unknown patterns slip through undetected

**What we need:**
- Semantic verification (understands meaning, not just patterns)
- Zero-code addition for new violation types
- Configuration-driven compliance rules
- Scalable to any tenant/vertical without code changes

---

## Proposed Architecture: Multi-Layer Verification

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        GENERATION LAYER                                      │
│  ┌─────────────┐    ┌──────────────────┐    ┌─────────────────────────────┐ │
│  │ User Query  │───▶│ Context Builder  │───▶│ Primary LLM (Generation)    │ │
│  └─────────────┘    │ - Relevant KB    │    │ - Grounded in retrieved KB  │ │
│                     │ - Active Policies│    │ - Structured output format  │ │
│                     │ - Session State  │    └─────────────────────────────┘ │
│                     └──────────────────┘                 │                   │
└──────────────────────────────────────────────────────────│───────────────────┘
                                                           ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        VERIFICATION LAYER                                    │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    Verifier LLM (Fast/Cheap)                         │    │
│  │  Input: Response + KB + Policies + Verification Checklist            │    │
│  │  Output: { compliant: bool, violations: [], corrected_response: "" } │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│            ┌───────────────────────┼───────────────────────┐                │
│            ▼                       ▼                       ▼                │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐         │
│  │ Fact Grounding  │    │ Policy Compliance│    │ Tone/Identity   │         │
│  │ Check           │    │ Check            │    │ Check           │         │
│  │                 │    │                  │    │                 │         │
│  │ "Is every fact  │    │ "Does response   │    │ "Does response  │         │
│  │ in the response │    │ follow all       │    │ match configured│         │
│  │ from the KB?"   │    │ policy rules?"   │    │ tone/identity?" │         │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        CORRECTION LAYER (if needed)                          │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │ If violations detected:                                              │    │
│  │ 1. Use verifier's corrected_response if available                    │    │
│  │ 2. Or: Re-generate with explicit constraints from violation report   │    │
│  │ 3. Or: Fall back to safe template response                           │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                            Customer Response
```

---

## Component Design

### 1. Verification Checklist (Configuration-Driven)

Instead of hardcoded rules, define verification checks in `POLICY.MD`:

```markdown
## Verification Checklist

### Fact Grounding
- [ ] All phone numbers appear in KB.MD under "Contact Information"
- [ ] All email addresses appear in KB.MD under "Contact Information"
- [ ] All services mentioned appear in KB.MD under "Services"
- [ ] All pricing/timing claims appear in KB.MD or are explicitly stated as "varies"
- [ ] No invented processes, procedures, or steps

### Policy Compliance
- [ ] No promises of specific response times unless in KB
- [ ] No pricing quotes unless in KB
- [ ] Excluded services are not offered
- [ ] Safety protocols followed for emergency keywords

### Tone/Identity
- [ ] Response matches configured tone (professional, friendly, etc.)
- [ ] No first-person "I think" or "I believe" for factual claims
- [ ] Company name used correctly
```

**Benefits:**
- Tenants can customize their own verification rules
- No code changes needed for new rules
- Rules are version-controlled with KB/Policy files

---

### 2. Verifier Prompt Template

```
You are a compliance verifier for a customer service agent. Your job is to check if the agent's response follows the rules.

## Knowledge Base (Source of Truth)
{kb_content}

## Policies
{policy_content}

## Verification Checklist
{checklist}

## Agent's Response to Verify
{response}

## Your Task
Check the response against each item in the verification checklist.

Respond in this exact JSON format:
{
  "compliant": true/false,
  "violations": [
    {
      "checklist_item": "which rule was violated",
      "violation": "what specifically is wrong",
      "evidence": "the problematic text from response",
      "correction": "how it should be corrected"
    }
  ],
  "corrected_response": "full corrected response if violations found, otherwise null"
}

Important:
- Be strict. If something isn't explicitly in the KB, it's a violation.
- Phone numbers must match exactly.
- Services must match KB service names exactly.
- If response is compliant, return {"compliant": true, "violations": [], "corrected_response": null}
```

---

### 3. Grounded Generation (Prevent hallucination at source)

Instead of letting the LLM generate freely, use **structured output with citations**:

```
## Response Format (Internal)

When generating a response, structure it as:

{
  "greeting": "string",
  "acknowledgment": "string",
  "information": [
    {
      "claim": "We offer water damage restoration",
      "source": "KB.MD > Services > Water Damage",
      "confidence": "grounded"
    },
    {
      "claim": "Our emergency line is (631) 358-5200",
      "source": "KB.MD > Contact > Emergency Phone",
      "confidence": "grounded"
    }
  ],
  "next_step": "string",
  "closing": "string"
}

Rules:
- Every claim in "information" MUST have a source from KB
- If no source exists, set confidence to "uncertain" and omit from final response
- Final response is assembled from grounded claims only
```

**Benefits:**
- Forces LLM to cite sources
- Easy to verify citations programmatically
- Ungrounded claims are automatically filtered

---

### 4. Implementation Phases

#### Phase 1: Verifier Layer (Recommended First)
**Effort:** Medium | **Impact:** High

Add a verification step after generation:

```go
// service.go - New verification flow

func (s *Service) generateVerifiedResponse(ctx context.Context, ...) (string, error) {
    // Step 1: Generate initial response
    response, err := s.generateResponse(ctx, ...)
    if err != nil {
        return "", err
    }

    // Step 2: Verify response
    verification, err := s.verifyResponse(ctx, response, config)
    if err != nil {
        slog.Warn("verification failed, using unverified response", "error", err)
        return response, nil // Graceful degradation
    }

    // Step 3: Return corrected response if violations found
    if !verification.Compliant && verification.CorrectedResponse != "" {
        slog.Info("response corrected by verifier",
            "violations", len(verification.Violations),
            "original_length", len(response),
            "corrected_length", len(verification.CorrectedResponse))
        return verification.CorrectedResponse, nil
    }

    return response, nil
}

func (s *Service) verifyResponse(ctx context.Context, response string, config *AudienceConfig) (*VerificationResult, error) {
    // Build verification prompt from POLICY.MD checklist
    checklist := extractVerificationChecklist(config.Policy)

    prompt := buildVerifierPrompt(response, config.KB, config.Policy, checklist)

    // Use fast/cheap model for verification
    result, err := s.callVerifierLLM(ctx, prompt)
    if err != nil {
        return nil, err
    }

    return parseVerificationResult(result)
}
```

#### Phase 2: Grounded Generation
**Effort:** High | **Impact:** Very High

Modify generation to use structured outputs with citations:

```go
// New structured generation approach

type GroundedResponse struct {
    Claims []GroundedClaim `json:"claims"`
}

type GroundedClaim struct {
    Text       string `json:"text"`
    Source     string `json:"source"`      // "KB.MD > Services > Water Damage"
    Confidence string `json:"confidence"`  // "grounded" | "uncertain" | "inferred"
}

func (s *Service) generateGroundedResponse(ctx context.Context, ...) (string, error) {
    // Prompt LLM to generate structured response with citations
    structured, err := s.generateStructuredResponse(ctx, ...)
    if err != nil {
        return "", err
    }

    // Filter to only grounded claims
    var verifiedClaims []string
    for _, claim := range structured.Claims {
        if claim.Confidence == "grounded" {
            // Verify citation actually exists in KB
            if s.verifyCitation(claim.Source, config.KB) {
                verifiedClaims = append(verifiedClaims, claim.Text)
            }
        }
    }

    // Assemble final response from verified claims only
    return assembleResponse(verifiedClaims), nil
}
```

#### Phase 3: Self-Critique Loop
**Effort:** Low | **Impact:** Medium

Add self-critique before verification:

```
After generating your response, critique it:

1. Did I mention any services not in the KB? If yes, remove them.
2. Did I provide any contact info not in the KB? If yes, remove it.
3. Did I make any promises not supported by KB? If yes, soften them.
4. Did I follow the conversation flow from SCRIPT.MD? If no, adjust.

Provide your final, corrected response.
```

---

## Verification Checklist Schema

Add to POLICY.MD parser to support structured checklists:

```markdown
## Verification Rules

### contact_grounding
- type: exact_match
- description: All contact information must exactly match KB
- sources:
  - KB.MD > Contact Information
- patterns:
  - phone_numbers
  - email_addresses
  - physical_addresses

### service_grounding
- type: exact_match
- description: All services mentioned must exist in KB
- sources:
  - KB.MD > Services
  - KB.MD > Emergency Services

### no_excluded_services
- type: blocklist
- description: Never mention excluded services
- sources:
  - KB.MD > Services We Don't Provide

### pricing_grounding
- type: conditional
- description: Only mention pricing if explicitly in KB
- fallback: "Pricing varies based on the situation. We can provide a detailed estimate after assessment."

### timing_grounding
- type: conditional
- description: Only promise response times if explicitly in KB
- fallback: "We'll respond as quickly as possible."
```

---

## Cost Analysis

| Approach | Extra LLM Calls | Latency Impact | Accuracy Improvement |
|----------|-----------------|----------------|---------------------|
| Current (regex) | 0 | 0ms | Low (pattern-based only) |
| Verifier Layer | +1 (cheap model) | +200-500ms | High (semantic) |
| Grounded Generation | 0 (different prompt) | +100ms | Very High |
| Self-Critique | 0 (same call) | +50ms | Medium |
| Full Pipeline | +1 | +300-600ms | Very High |

**Recommended:** Start with Verifier Layer, then add Grounded Generation.

Use a fast/cheap model for verification (e.g., GPT-4o-mini, Claude Haiku) to minimize cost.

---

## Model Selection for Verification

| Model | Speed | Cost | Accuracy | Recommendation |
|-------|-------|------|----------|----------------|
| GPT-4o-mini | Fast | $0.15/1M | Good | Best for verification |
| Claude 3 Haiku | Fast | $0.25/1M | Good | Alternative |
| Gemini Flash | Very Fast | $0.075/1M | Moderate | Budget option |

**Strategy:** Use expensive model for generation, cheap model for verification.

---

## Fallback Responses

When verification fails repeatedly, use safe template responses:

```go
var SafeFallbacks = map[string]string{
    "contact_request": "For immediate assistance, please call our main line. Our team will be happy to help you.",
    "service_inquiry": "We offer a range of services. Could you tell me more about your situation so I can better assist you?",
    "pricing_request": "Pricing depends on the specific situation. We'd be happy to provide a detailed estimate after learning more about your needs.",
    "unknown": "I want to make sure I give you accurate information. Let me connect you with someone who can help with that specific question.",
}
```

---

## Metrics & Monitoring

Track verification effectiveness:

```go
type VerificationMetrics struct {
    TotalResponses       int
    VerifiedCompliant    int
    VerifiedViolations   int
    CorrectionsMade      int
    FallbacksUsed        int

    ViolationsByType     map[string]int  // "contact_hallucination": 15
    AverageLatencyMs     float64
    VerifierErrorRate    float64
}
```

Dashboard queries:
- Violation rate over time (should decrease as LLM learns from corrections)
- Most common violation types (identify KB gaps)
- Correction success rate (verifier effectiveness)
- Latency impact (performance monitoring)

---

## Migration Path

### Week 1-2: Verifier Layer
1. Implement `verifyResponse()` function
2. Add verification checklist parsing to POLICY.MD parser
3. Deploy in "shadow mode" (verify but don't block)
4. Collect metrics on violation rates

### Week 3-4: Enable Correction
1. Enable automatic correction for high-confidence violations
2. Add fallback responses for uncertain cases
3. Monitor customer satisfaction metrics

### Week 5-6: Grounded Generation
1. Implement structured output format
2. Add citation verification
3. A/B test against non-grounded generation

### Week 7+: Optimize
1. Tune verification prompts based on false positive/negative rates
2. Add tenant-specific verification rules
3. Implement caching for common verifications

---

## Example: Verification in Action

**User:** "What's your emergency number?"

**Initial Response (hallucinated):**
> "Our emergency line is (555) 123-4567. We're available 24/7 for urgent situations."

**Verifier Check:**
```json
{
  "compliant": false,
  "violations": [
    {
      "checklist_item": "contact_grounding",
      "violation": "Phone number not found in KB",
      "evidence": "(555) 123-4567",
      "correction": "Replace with (631) 358-5200 from KB"
    },
    {
      "checklist_item": "timing_grounding",
      "violation": "24/7 availability not stated in KB",
      "evidence": "We're available 24/7",
      "correction": "Remove or verify against KB hours"
    }
  ],
  "corrected_response": "Our emergency line is (631) 358-5200. Please call us for urgent situations and we'll respond as quickly as possible."
}
```

**Final Response (corrected):**
> "Our emergency line is (631) 358-5200. Please call us for urgent situations and we'll respond as quickly as possible."

---

## Summary

| Current State | Target State |
|--------------|--------------|
| Regex pattern matching | Semantic verification |
| Hardcoded rules in code | Configuration in POLICY.MD |
| Detect & log violations | Detect, correct & serve |
| New patterns = new code | New patterns = update config |
| Scales linearly with rules | Scales with LLM capability |

**Key Principle:** Let LLMs verify LLMs. Use the same technology that creates hallucinations to detect them, but with stricter grounding requirements.

---

## Appendix: Files to Create/Modify

### New Files
- `server/router/api/v1/agent/verifier.go` - Verification logic
- `server/router/api/v1/agent/grounding.go` - Grounded generation

### Modified Files
- `server/router/api/v1/agent/parser.go` - Parse verification checklist from POLICY.MD
- `server/router/api/v1/agent/service.go` - Integrate verification into response flow
- `store/agent.go` - Add VerificationMetrics type

### Configuration Changes
- `POLICY.MD` template - Add verification checklist section

---

## Addendum: Testing Phase 2 (Grounded Generation)

### Prerequisites
1. Build the application: `task build`
2. Ensure `OPENROUTER_API_KEY` is set in environment
3. Have a tenant configured with KB.MD (services, contact info, etc.)

### Method 1: Enable and Test via Chat

1. **Enable grounded generation** by modifying `service.go`:
   ```go
   func (s *Service) UseGroundedGeneration(config *AudienceConfig) bool {
       return true // Enable for all tenants
   }
   ```

2. **Rebuild**: `task build`

3. **Start the server**: `./build/memos`

4. **Run a simulation or chat session** through the Agent Admin UI

5. **Check logs** for grounded generation output:
   ```
   grounded generation completed total_claims=4 grounded_claims=3 filtered_claims=1 latency_ms=850
   ```

### Method 2: Unit Test the Grounder Directly

Create a test file or use Go playground:

```go
package main

import (
    "context"
    "fmt"
    "github.com/revrost/go-openrouter"
    "github.com/usememos/memos/server/router/api/v1/agent"
)

func main() {
    apiKey := "your-openrouter-api-key"
    client := openrouter.NewClient(apiKey)

    config := agent.DefaultGroundingConfig()
    grounder := agent.NewGrounder(client, config)

    // Create a minimal audience config for testing
    audienceConfig := &agent.AudienceConfig{
        CompanyName: "Test Company",
        Audience: &store.AgentAudience{
            Role:           "customer service representative",
            Tone:           "professional and friendly",
            EmergencyPhone: "(555) 123-4567",
        },
        Services: []*store.AgentService{
            {Name: "Water Damage Restoration", Code: "water_damage", Description: "24/7 water extraction"},
        },
    }

    result, err := grounder.GenerateGroundedResponse(
        context.Background(),
        "openai/gpt-4o-mini",
        audienceConfig,
        "What services do you offer?",
        &agent.Classification{PrimaryIntent: "service_inquiry"},
    )

    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Response: %s\n", result.Response)
    fmt.Printf("Total claims: %d\n", result.TotalClaims)
    fmt.Printf("Grounded claims: %d\n", result.GroundedClaims)
    fmt.Printf("Filtered claims: %d\n", result.FilteredClaims)
}
```

### What to Verify

1. **Grounded claims pass**: Claims citing valid KB sections should be included
2. **Uncertain claims filtered**: Claims with `"confidence": "uncertain"` should be filtered
3. **Placeholder detection**: Claims containing (555) numbers or fake emails should be filtered
4. **Fallback behavior**: If all claims are filtered, a safe fallback response should be returned
5. **Metrics recorded**: Check `grounding_metrics` are being tracked

### Expected Log Output

When grounded generation is working:
```
INFO grounded generation completed total_claims=5 grounded_claims=4 filtered_claims=1 latency_ms=920
DEBUG using grounded generation response session_id=abc123
```

When falling back to regular generation:
```
WARN grounded generation failed, falling back to regular generation error="failed to parse grounded response" session_id=abc123
```

### Comparing Phase 1 vs Phase 2

| Scenario | Phase 1 (Verifier) | Phase 2 (Grounded) |
|----------|-------------------|-------------------|
| Hallucinated phone | Catches & corrects after generation | Filters claim before including |
| Unknown service | May slip through | Marked "uncertain", filtered out |
| Latency | +200-500ms (separate call) | Included in generation |
| Cost | Extra LLM call | Same cost as regular |

### Troubleshooting

**Issue: Grounded response is empty or just a fallback**
- Check if the LLM is returning valid JSON structure
- Verify KB has services/FAQs that can be cited
- Check confidence levels in response (should be "grounded" or "inferred")

**Issue: JSON parsing errors**
- The LLM may not be following the structured format
- Try a more capable model (gpt-4o instead of gpt-4o-mini)
- Check for markdown code blocks wrapping the JSON

**Issue: All claims filtered**
- Citation paths may not match KB structure
- Check `verifyCitation()` logic against your KB sections
