# Hallucination Detection & Grounding

**Status:** Active
**Date:** 2026-01-23
**Related Documents:**
- [DOCS_EVALUATION.MD](./DOCS_EVALUATION.MD) - Evaluation framework
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture

---

## Overview

### What Are Hallucinations?

In AI chat systems, **hallucinations** are responses that contain information not grounded in the provided knowledge base:

- **Invented contact information** - Placeholder phones (555-xxx-xxxx) or emails
- **Made-up services** - Offering services not in KB
- **Unauthorized claims** - Pricing, guarantees, or timelines not documented
- **Fabricated procedures** - Steps or processes not in KB

### What Is Grounding?

**Grounding** means every claim in the response can be traced back to the knowledge base:

| Claim | Grounded? | Reason |
|-------|-----------|--------|
| "We offer water extraction" | Yes | Listed in KB Services |
| "Our phone is (555) 123-4567" | No | Placeholder pattern |
| "Service takes 30 minutes" | Maybe | Only if documented in KB |
| "We guarantee same-day service" | No | Unauthorized promise |

### Why Grounding Matters

Enterprise chat agents require **factual accuracy**:
- **Customer trust** - Wrong contact info loses customers
- **Legal compliance** - Unauthorized promises create liability
- **Brand consistency** - Off-brand tone damages reputation

---

## Three-Layer Validation System

bchat implements a **defense-in-depth** approach with three validation layers:

```
┌─────────────────────────────────────────────────────────────────┐
│                     LLM Response Generated                       │
└─────────────────────────┬───────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│ LAYER 1: Regex Compliance Check (compliance.go)                  │
│ ─────────────────────────────────────────────────────────────── │
│ • Placeholder phone detection (555-xxxx, 000-0000)              │
│ • Placeholder email detection (@example.com)                     │
│ • Excluded service mentions                                      │
│ • Status: OPTIONAL (marked for removal in Phase 5)              │
└─────────────────────────┬───────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│ LAYER 2: Output Sanitization (sanitizer.go)                      │
│ ─────────────────────────────────────────────────────────────── │
│ • System tag removal ([SYSTEM], [DEBUG], [THOUGHT])             │
│ • Self-reference removal (Assistant:, AI:, Bot:)                │
│ • Impossible contact correction (000-000-0000 → real phone)     │
│ • Markdown cleanup                                               │
│ • Status: ALWAYS ACTIVE                                         │
└─────────────────────────┬───────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│ LAYER 3: LLM-Based Verification (verifier.go)                    │
│ ─────────────────────────────────────────────────────────────── │
│ • Semantic verification against KB                               │
│ • Violation detection with severity levels                       │
│ • Auto-correction capability                                     │
│ • Status: DISABLED BY DEFAULT (enable with env var)             │
└─────────────────────────┬───────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Final Response                             │
└─────────────────────────────────────────────────────────────────┘
```

---

## Layer 1: Regex-Based Compliance

**File:** `compliance.go`
**Status:** Active but marked for removal

### Detected Patterns

| Category | Pattern | Example |
|----------|---------|---------|
| Placeholder phone | `555-XXX-XXXX` | (555) 123-4567 |
| All-zeros phone | `000-000-0000` | 000-000-0000 |
| Test phone | `123-456-7890` | (123) 456-7890 |
| Placeholder email | `@example.com` | test@example.com |
| Test email | `@test.com` | user@test.com |

### Limitations

- **Pattern-only** - Cannot understand semantic meaning
- **False positives** - May flag legitimate customer-provided numbers
- **Maintenance burden** - Requires updating patterns manually

---

## Layer 2: Output Sanitization

**File:** `sanitizer.go`
**Status:** Always active

### System Tag Removal

The sanitizer removes hallucinated system tags that LLMs sometimes generate:

```go
// Patterns removed (sanitizer.go:16-28)
`[SYSTEM]...`
`[DEBUG]...`
`[THOUGHT]...`
`[INTERNAL]...`
`[NOTE]...`
`Assistant:...`
`AI:...`
`Bot:...`
```

### Thinking Pattern Removal

Removes "thinking out loud" that shouldn't appear in responses:

```go
// Patterns removed (sanitizer.go:29-33)
`Let me think...`
`I should...`
`I need to...`
`*action descriptions*`
```

### Contact Correction

**Impossible phone patterns** are corrected automatically:

```go
// From sanitizer.go:171-178
impossiblePatterns := []string{
    `\(?000\)?[-.\s]?000[-.\s]?0000`,  // All zeros
    `\(?999\)?[-.\s]?999[-.\s]?9999`,  // All nines
    `\(?123\)?[-.\s]?456[-.\s]?7890`,  // Sequential test
}
```

**Placeholder email patterns** are also corrected:

```go
// From sanitizer.go:260-278
placeholderPatterns := []string{
    `[a-z]+\.[a-z]+@email\.com`,           // firstname.lastname@email.com
    `[a-zA-Z0-9._%+-]+@example\.(com|org|net)`, // @example.com
    `[a-zA-Z0-9._%+-]+@test\.(com|org|net)`,    // @test.com
    `user@user\.com`,
    `test@test\.com`,
}
```

### Phone Validation

Before using a phone for replacement, it's validated:

```go
// From sanitizer.go:354-393
func IsValidReplacementPhone(phone string) bool {
    // Reject patterns:
    // - 555 area code (reserved for fiction)
    // - 000 area code (invalid)
    // - 1234567890 (test number)
    // - Less than 10 digits
}
```

### Usage Example

```go
// Basic sanitization
clean := SanitizeResponse(response)

// With contact correction
clean = DefaultSanitizer.CorrectContacts(response, "(631) 358-5200")
clean = DefaultSanitizer.CorrectEmails(clean, "info@company.com")
```

---

## Layer 3: LLM-Based Verification

**File:** `verifier.go`
**Status:** Disabled by default

### Enabling Verification

```bash
export LLM_VERIFIER_ENABLED=true
```

### How It Works

1. **Build verification prompt** with KB, Policy, and checklist
2. **Call verifier LLM** (fast/cheap model like gpt-4o-mini)
3. **Parse JSON response** with violations and corrections
4. **Apply corrections** if mode is "enforce"

### Verification Checklist

The verifier checks against a comprehensive checklist (verifier.go:244-278):

#### Contact Information Grounding
```
- [ ] All phone numbers appear exactly in KB
- [ ] All emails appear exactly in KB
- [ ] No placeholder numbers like (555) xxx-xxxx
- [ ] No placeholder emails like @example.com
```

#### Service Grounding
```
- [ ] All services mentioned appear in KB
- [ ] No excluded services offered
- [ ] Service descriptions match KB
```

#### Claims Grounding
```
- [ ] No specific pricing unless in KB
- [ ] No response time promises unless in KB
- [ ] No guarantees not in KB
- [ ] No invented procedures
```

#### Timeline & Process Claims
```
- [ ] No duration claims unless in KB
- [ ] No preparation instructions unless in KB
- [ ] No arrival time promises unless in KB
```

#### Customer Information Handling
```
- [ ] Customer phone echoed correctly
- [ ] Customer name echoed correctly
- [ ] Customer address echoed correctly
```

#### Tone & Identity
```
- [ ] Response matches configured tone
- [ ] Company name used correctly
- [ ] No inappropriate speculation
```

### Violation Types & Severity

| Violation Type | Severity | Example |
|----------------|----------|---------|
| `contact_information_grounding` | Critical | Wrong phone number |
| `service_grounding` | High | Offering mold remediation when excluded |
| `claims_grounding` | High | "We guarantee 24-hour response" |
| `timeline_claims` | Medium | "Inspection takes 30 minutes" |
| `process_claims` | Medium | Invented preparation steps |
| `customer_info_handling` | Medium | Wrong customer phone echo |
| `tone_and_identity` | Low | Off-brand language |

### Verification Result Structure

```go
type VerificationResult struct {
    Compliant         bool                    // True if no violations
    Violations        []VerificationViolation // List of issues
    CorrectedResponse string                  // Fixed response (if violations)
    VerificationTime  time.Duration           // Latency
    Model             string                  // Model used
}

type VerificationViolation struct {
    ChecklistItem string // Which rule violated
    Violation     string // What's wrong
    Evidence      string // Problematic text
    Correction    string // How to fix
    Severity      string // critical/high/medium/low
}
```

### Configuration Options

```go
type VerificationConfig struct {
    Enabled       bool   // Enable verification
    Model         string // "openai/gpt-4o-mini" (fast/cheap)
    Mode          string // "shadow" or "enforce"
    MaxLatencyMs  int    // Skip if too slow (default: 3000)
    SkipOnError   bool   // Graceful degradation (default: true)
}
```

**Modes:**
- `shadow` - Log violations but don't correct (monitoring)
- `enforce` - Apply corrections to response

---

## Grounding Score Calculation

### Current Approach

Grounding is currently **implicit** through:

1. **System prompt contains full KB** - LLM instructed to use only KB content
2. **Verification checklist** - Explicit checks against KB
3. **Violation counting** - Track grounding failures

### Calculating Grounding Score

A grounding score can be calculated from verification results:

```
Grounding Score = 1 - (violations / total_checks)
```

**Example:**
- 20 checklist items checked
- 2 violations found
- Grounding Score = 1 - (2/20) = 0.90 (90% grounded)

### Grounding Categories

| Category | Checks | Weight |
|----------|--------|--------|
| Contact Information | 4 | High |
| Service Accuracy | 3 | High |
| Claims & Promises | 4 | High |
| Timeline & Process | 4 | Medium |
| Customer Info | 3 | Medium |
| Tone & Identity | 2 | Low |

### Future: Explicit Grounding Score

Planned implementation (not yet built):

```go
type GroundingAnalysis struct {
    OverallScore      float64 // 0.0 - 1.0
    CategoryScores    map[string]float64
    UncitedClaims     []string // Claims not traceable to KB
    CitedClaims       []string // Claims with KB source
    HallucinationRate float64  // % uncited / total claims
}
```

---

## Metrics & Monitoring

### VerificationMetrics Structure

```go
// From verifier.go:422-431
type VerificationMetrics struct {
    TotalVerifications   int64
    CompliantResponses   int64
    ViolationsDetected   int64
    CorrectionsMade      int64
    VerificationErrors   int64
    AverageLatencyMs     float64
    ViolationsByType     map[string]int64
    ViolationsBySeverity map[string]int64
}
```

### Key Metrics to Track

| Metric | Formula | Target |
|--------|---------|--------|
| Compliance Rate | compliant / total | > 95% |
| Hallucination Rate | violations / total_checks | < 5% |
| Critical Violations | critical_count / total | < 1% |
| Avg Verification Latency | sum(latency) / count | < 500ms |

### Logging

```go
// Metrics summary logging (verifier.go:470-484)
slog.Info("verification metrics summary",
    "total", m.TotalVerifications,
    "compliant", m.CompliantResponses,
    "compliance_rate", fmt.Sprintf("%.1f%%", complianceRate),
    "violations", m.ViolationsDetected,
    "corrections", m.CorrectionsMade,
    "avg_latency_ms", fmt.Sprintf("%.0f", m.AverageLatencyMs),
)
```

---

## Configuration Guide

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_VERIFIER_ENABLED` | `false` | Enable LLM verification |

### Recommended Settings

**Development:**
```bash
# Disable verification for faster iteration
export LLM_VERIFIER_ENABLED=false
```

**Staging:**
```bash
# Enable in shadow mode for monitoring
export LLM_VERIFIER_ENABLED=true
# Set mode to "shadow" in code
```

**Production:**
```bash
# Enable in enforce mode
export LLM_VERIFIER_ENABLED=true
# Set mode to "enforce" in code
```

### Performance Considerations

| Component | Latency | Impact |
|-----------|---------|--------|
| Regex compliance | < 1ms | Negligible |
| Output sanitization | < 5ms | Negligible |
| LLM verification | 200-500ms | Significant |

**Recommendation:** Enable LLM verification only when accuracy is critical and latency budget allows.

---

## Integration Points

### Service.go Integration

```go
// From service.go:522-565
func (s *Service) verifyResponseWithLLM(ctx context.Context, response string,
    audienceConfig *AudienceConfig) (string, error) {

    // Check if verification is enabled
    if !s.profile.LLMVerifierEnabled {
        return response, nil
    }

    // Create verifier and run verification
    verifier := NewVerifier(s.client, DefaultVerificationConfig())
    result, err := verifier.VerifyResponse(ctx, response, audienceConfig)

    // Apply corrections if enforce mode
    if !result.Compliant && result.CorrectedResponse != "" {
        return result.CorrectedResponse, nil
    }

    return response, nil
}
```

### Post-Verification Sanitization

```go
// From service.go:973-1001
// Re-sanitize after verification (verifier may introduce placeholders)
response = SanitizeResponse(response)
response = DefaultSanitizer.CorrectContacts(response, validPhone)
response = DefaultSanitizer.CorrectEmails(response, validEmail)
```

---

## Future Enhancements

### Planned Features

| Feature | Description | Priority |
|---------|-------------|----------|
| Explicit Grounding Score | 0-100 score per response | High |
| Citation Tracking | Map claims to KB sources | High |
| Hallucination Rate Dashboard | Real-time monitoring | Medium |
| RAG-Based Grounding | Verify against retrieved chunks | Medium |
| A/B Testing | Compare verification strategies | Low |

### RAG-Based Grounding (Phase 5)

The RAG pipeline will improve grounding by:

1. **Retrieving relevant chunks** instead of full KB
2. **Tracking which chunks** were used for response
3. **Scoring retrieval quality** (Recall, Precision)
4. **Detecting claims** not in retrieved context

```
Query → RAG Retrieval → Relevant Chunks → LLM Response
                              ↓
                    Grounding Verification
                              ↓
                    Claims traced to chunks?
```

---

## Troubleshooting

### Common Issues

**Issue:** Verification always returns compliant
**Cause:** Verifier disabled or KB empty
**Fix:** Check `LLM_VERIFIER_ENABLED=true` and KB content

**Issue:** High false positive rate
**Cause:** Strict checklist flagging legitimate content
**Fix:** Customize verification rules for your domain

**Issue:** Verification too slow
**Cause:** Model latency or network issues
**Fix:** Use faster model (gpt-4o-mini) or increase `MaxLatencyMs`

**Issue:** Placeholders still appearing
**Cause:** LLM verification can re-introduce placeholders
**Fix:** Post-verification sanitization handles this automatically

### Debug Logging

Enable debug logging to troubleshoot:

```go
slog.Debug("verification result",
    "compliant", result.Compliant,
    "violations", len(result.Violations),
    "latency_ms", result.VerificationTime.Milliseconds(),
)
```

---

## Summary

| Layer | Purpose | Status | Latency |
|-------|---------|--------|---------|
| Regex Compliance | Pattern detection | Deprecated | < 1ms |
| Output Sanitization | Cleanup & correction | Always on | < 5ms |
| LLM Verification | Semantic grounding | Optional | 200-500ms |

**Best Practice:** Use output sanitization always, enable LLM verification in production when accuracy is critical.

---

*Document Version: 1.0*
*Created: 2026-01-23*
