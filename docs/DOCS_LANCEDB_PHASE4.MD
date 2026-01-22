# LanceDB RAG Implementation - Phase 4: Prompt Simplification

**Status:** Complete
**Date:** 2026-01-23
**Parent Document:** [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD)
**Previous Phase:** [DOCS_LANCEDB_PHASE3.MD](./DOCS_LANCEDB_PHASE3.MD)

---

## Overview

Phase 4 simplified the RAG system prompt to dramatically reduce token usage while maintaining response quality. The prompt was restructured from 11 sections to 7 streamlined sections, with an estimated 60-70% token reduction.

---

## Files Modified

### `server/router/api/v1/agent/service.go`

Refactored `buildRAGSystemPrompt()` with a simplified template structure.

---

## Prompt Structure Comparison

### Before (11 Sections)

```
0. CUSTOMER INFO ALREADY PROVIDED
1. CRITICAL CONSTRAINTS
2. YOUR IDENTITY
3. RELEVANT SERVICES
4. SERVICES WE DON'T PROVIDE
5. CONVERSATION FLOW
6. ACTIVE POLICIES
7. RELEVANT RULES
8. SAFETY INFORMATION
9. MANDATORY BEHAVIORS
10. RELEVANT FAQS
11. AUTHORIZED CONTACT INFO
12. RESPONSE FORMAT
13. REMINDER
```

**Issues:**
- Redundant constraint reminders at start AND end
- Scattered retrieved content across multiple sections
- Verbose section headers
- Repeated phone number in 3+ places

### After (7 Sections)

```
1. IDENTITY
2. CUSTOMER INFO (if collected)
3. CONSTRAINTS (with contact info)
4. CONVERSATION GUIDE
5. RETRIEVED CONTEXT (unified)
6. ACTIVE RULES & SAFETY
7. FORMAT
```

**Improvements:**
- Single unified RETRIEVED CONTEXT section
- Constraints and contact combined
- No redundant reminders
- Minimal section headers
- Phone number mentioned only where needed

---

## New Prompt Template

```go
func (s *Service) buildRAGSystemPrompt(...) string {
    // SECTION 1: IDENTITY
    // Who you are, tone, brand voice

    // SECTION 2: CUSTOMER INFO (conditional)
    // Info already collected - DO NOT ask again

    // SECTION 3: CONSTRAINTS
    // Combined with contact info for efficiency

    // SECTION 4: CONVERSATION GUIDE
    // From SCRIPT.MD

    // SECTION 5: RETRIEVED CONTEXT
    // All RAG content unified:
    // - SERVICES
    // - FAQS
    // - COVERAGE
    // - NOT PROVIDED (exclusions)
    // - POLICIES
    // - SAFETY
    // - INFORMATION

    // SECTION 6: ACTIVE RULES & SAFETY
    // Context-specific rules, safety triggers, emergency flags
    // Learned behaviors

    // SECTION 7: FORMAT
    // Minimal: "Plain text only, no markdown. Be concise but complete."
}
```

---

## Token Savings Analysis

### Estimated Savings by Component

| Component | Before | After | Savings |
|-----------|--------|-------|---------|
| Section headers | ~150 tokens | ~50 tokens | 67% |
| Constraints | ~200 tokens | ~80 tokens | 60% |
| Contact info | ~100 tokens (3x repeated) | ~40 tokens (1x) | 60% |
| Reminders | ~100 tokens | 0 tokens | 100% |
| Retrieved content | Same | Same | 0% |
| **Boilerplate total** | ~550 tokens | ~170 tokens | **69%** |

### Example Prompt Comparison

**Before (estimated ~800 tokens boilerplate + content):**
```
=== CRITICAL CONSTRAINTS (YOU MUST FOLLOW THESE) ===

1. ONLY discuss services listed in RELEVANT SERVICES below.
2. ONLY provide contact info from AUTHORIZED CONTACT INFO.
3. If uncertain, acknowledge honestly - do not invent information.
4. Never invent services, prices, phone numbers, or processes.
5. The ONLY valid phone number is: (555) 123-4567

=== YOUR IDENTITY ===

You are a customer service representative for ACME Corp.
Tone: Professional, empathetic
Brand voice: "Helpful and caring"

[... 9 more sections ...]

=== REMINDER ===
PHONE: (555) 123-4567
Never invent information. If unsure, say so.
```

**After (estimated ~300 tokens boilerplate + content):**
```
=== IDENTITY ===
You are a customer service representative for ACME Corp.
Tone: Professional, empathetic
Voice: "Helpful and caring"

=== CONSTRAINTS ===
- Only discuss services in RETRIEVED CONTEXT below
- Never invent services, prices, phone numbers, or processes
- If uncertain, acknowledge honestly
- Phone: (555) 123-4567 (ONLY this number)
- Email: support@acme.com

=== RETRIEVED CONTEXT (Use ONLY this information) ===

SERVICES:
- Water Damage Restoration [EMERGENCY]: 24/7 emergency water extraction...

FAQS:
Q: How quickly can you respond?
A: We typically arrive within 60 minutes...

=== FORMAT ===
Plain text only, no markdown. Be concise but complete.
```

---

## Key Design Decisions

### 1. Unified Retrieved Context Section

Instead of scattering retrieved chunks across 5+ sections, all RAG content now lives in a single `RETRIEVED CONTEXT` section with clear subsections:

```
=== RETRIEVED CONTEXT (Use ONLY this information) ===

SERVICES:
[retrieved services]

FAQS:
[retrieved FAQs]

COVERAGE:
[retrieved coverage]

NOT PROVIDED:
[retrieved exclusions]

POLICIES:
[retrieved rules]

SAFETY:
[retrieved safety]

INFORMATION:
[retrieved KB sections]
```

**Benefits:**
- Clearer mental model for the LLM
- Explicit "Use ONLY this" instruction
- Easier to audit what context was provided

### 2. Removed Redundant Reminders

The legacy prompt had:
- Constraints at the top
- Same constraints repeated in "CRITICAL REMINDER" at bottom
- Phone number mentioned 3+ times

The new prompt:
- States constraints once
- Phone number mentioned only where needed
- No "REMINDER" section (wastes tokens)

### 3. Combined Constraints and Contact

Instead of separate sections:
- "CRITICAL CONSTRAINTS"
- "AUTHORIZED CONTACT INFO"

Combined into single "CONSTRAINTS" section that includes contact info inline.

### 4. Minimal Response Format

Before:
```
=== RESPONSE FORMAT ===

- Use plain text only. NO markdown (no **, no *, no # headers, no - bullets).
- Use natural sentence structure.
- For lists, use numbered sentences or comma-separated items.
- Be concise but complete.
- If this is an emergency, lead with safety instructions and the emergency phone number.
```

After:
```
=== FORMAT ===
Plain text only, no markdown. Be concise but complete.
```

The LLM doesn't need verbose format instructions - a simple directive suffices.

---

## Legacy Prompt Preserved

The original `buildSystemPrompt()` function remains unchanged for non-RAG mode. This ensures:

- Backward compatibility when RAG is disabled
- Safe rollback path if issues arise
- A/B testing between legacy and RAG prompts

The choice between prompts happens in `processChat()`:

```go
if s.UseRAGPipeline() {
    response, genErr = s.generateRAGResponse(ctx, config, session, classification, decision, userMessage)
} else {
    response, genErr = s.generateResponse(ctx, config, session, classification, decision, userMessage)
}
```

---

## Testing

### Build Verification

```bash
# Standard build
task build
# Success

# RAG build
task build:backend:rag
# Success
```

### Prompt Quality Testing

To verify prompt quality:

1. Enable RAG pipeline: `RAG_PIPELINE_ENABLED=true`
2. Upload KB/Policy files to trigger indexing
3. Send test messages via chat
4. Review generated prompts in debug logs

Add debug logging to see prompts:

```go
slog.Debug("RAG system prompt", "prompt", systemPrompt, "tokens", estimateTokens(systemPrompt))
```

---

## Metrics to Track

When deploying, monitor:

| Metric | Expected Change |
|--------|-----------------|
| Avg prompt tokens | -50% to -70% |
| LLM latency | -10% to -20% (fewer tokens) |
| LLM cost | -40% to -60% |
| Response quality | Maintain or improve |
| Hallucination rate | Decrease |

---

## Next Steps (Phase 5)

Phase 5 will focus on cleanup and optimization:

1. Remove `compliance.go` entirely (regex checks no longer needed with RAG)
2. Remove `grounding.go` entirely (replaced by RAG pipeline)
3. Make LLM verifier optional via `LLM_VERIFIER_ENABLED` env var
4. Simplify sanitizer to basic text cleanup only
5. Performance tuning and monitoring
