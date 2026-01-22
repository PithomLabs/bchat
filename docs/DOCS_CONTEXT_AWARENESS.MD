# Context Awareness Issue: Phone Number Hallucination Analysis

**Status:** Analysis Complete
**Date:** 2026-01-23
**Issue:** Simulation transcript `sim-33b295f7-5927-4d1f-b5da-0e901c4c73da`
**Symptom:** Agent provided wrong phone number "(555) 000-0000" instead of "(631) 358-5200"

---

## Executive Summary

The AI agent hallucinated a placeholder phone number because of a **context awareness gap** in the system. When the simulation ran, the audience-specific knowledge base (KB) had not yet been uploaded, and the system had no fallback mechanism to retrieve critical contact information from alternative sources.

---

## Incident Analysis

### Transcript Evidence

**Turn 1 - Agent Response:**
```
Our emergency line for restoration help is (555) 000-0000. Please call right away...
```

**Turn 2 - Agent Response:**
```
...for your location you'll need a local restoration provider.
```

The agent gave "(555) 000-0000" which is a clearly invalid placeholder pattern (555 area code is reserved for fiction).

### Timeline

| Date | Event |
|------|-------|
| 2026-01-19 | External audience created with placeholder phone "(555) 000-0000" |
| 2026-01-20 15:44 | **Simulation ran** - Agent hallucinated placeholder phone |
| 2026-01-21 10:01 | Internal KB first uploaded with correct phone |
| 2026-01-21 12:13 | Internal audience updated with correct phone "(631) 358-5200" |

---

## Root Cause Analysis

### 1. Hardcoded Audience Type in Simulation

**File:** `server/router/api/v1/agent/handlers.go:1935`

```go
config, err := h.service.LoadConfig(ctx, slug, "internal")
```

The simulation handler **always** loads the "internal" audience configuration, regardless of whether internal-specific KB content exists.

### 2. Missing Knowledge Base Content

At the time of simulation (Jan 20), the database contained:

**agent_audiences:**
```
ID  | tenant_id | audience_type | emergency_phone   | updated_at
----+-----------+---------------+-------------------+------------------
2   | 1         | external      | (555) 000-0000    | 2026-01-19
10  | 1         | internal      | (631) 358-5200    | 2026-01-21  ← AFTER simulation
```

**agent_source_files (KB):**
```
ID  | tenant_id | audience_type | file_type | imported_at
----+-----------+---------------+-----------+------------------
15  | 1         | external      | kb        | Before Jan 20  ← Had correct phone
21  | 1         | internal      | kb        | 2026-01-21     ← AFTER simulation
```

### 3. Phone Validation Flow Failure

**File:** `server/router/api/v1/agent/service.go:743-746`

```go
validatedPhone := GetValidatedReplacementPhone(
    config.Audience.EmergencyPhone,  // "(555) 000-0000" - INVALID
    config.RawKB,                     // Empty - no internal KB
)
```

The flow:
1. **Check DB phone**: "(555) 000-0000" → Fails `IsValidReplacementPhone()` (contains "555")
2. **Extract from KB**: `config.RawKB` is empty (no internal KB file existed)
3. **Result**: Returns empty string

### 4. No Fallback to External KB

The system had a valid external KB with the correct phone number, but there was no mechanism to use it as a fallback when the audience-specific KB was missing.

### 5. LLM Hallucination

With `validatedPhone = ""`, the system prompt:
- Did not include a valid phone number
- Instructed "do not provide any phone number"

But the LLM still generated "(555) 000-0000" from its training data pattern.

---

## Code Path Trace

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        SIMULATION REQUEST (Jan 20)                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  handlers.go:1935                                                           │
│  └─ LoadConfig(ctx, slug, "internal")  ← Hardcoded "internal"               │
│                                                                             │
│  service.go:417-423                                                         │
│  └─ GetAgentAudience(tenantID, "internal")                                  │
│     └─ Returns audience with EmergencyPhone = "(555) 000-0000"              │
│                                                                             │
│  service.go:468-475                                                         │
│  └─ GetAgentSourceFile(tenantID, "internal", "kb")                          │
│     └─ Returns nil (no internal KB file existed on Jan 20)                  │
│     └─ config.RawKB = ""                                                    │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                        CHAT PROCESSING                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  service.go:743-746                                                         │
│  └─ GetValidatedReplacementPhone("(555) 000-0000", "")                      │
│     │                                                                       │
│     ├─ IsValidReplacementPhone("(555) 000-0000")                            │
│     │  └─ Returns FALSE (contains "555" placeholder pattern)                │
│     │                                                                       │
│     ├─ ExtractPhoneFromKB("")                                               │
│     │  └─ Returns "" (empty KB content)                                     │
│     │                                                                       │
│     └─ Returns "" (no valid phone found)                                    │
│                                                                             │
│  service.go:1045 (buildSystemPrompt)                                        │
│  └─ validatedPhone = ""                                                     │
│     └─ Prompt says: "do not provide any phone number"                       │
│                                                                             │
│  LLM Generation                                                             │
│  └─ Hallucinates "(555) 000-0000" from training patterns                    │
│                                                                             │
│  service.go:755                                                             │
│  └─ CorrectContactsInResponse(response, "")                                 │
│     └─ Replaces with "[phone number]" but pattern might not match           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Gap Identification

### Gap 1: No Cross-Audience KB Fallback

**Current Behavior:**
- Load audience-specific KB only
- If not found, `RawKB = ""`

**Expected Behavior:**
- Load audience-specific KB
- If not found, fall back to other audience's KB (e.g., external)
- Critical contact info should always be available

### Gap 2: Simulation Uses Hardcoded Audience

**Current Behavior:**
- Simulation always uses "internal" audience
- If internal config incomplete, simulation fails silently

**Expected Behavior:**
- Simulation should work with any available config
- Or allow audience type to be configurable

### Gap 3: No Validation at Config Load Time

**Current Behavior:**
- Config loads without validating critical fields
- Missing phone discovered only during response generation

**Expected Behavior:**
- Validate critical fields (phone, email) during LoadConfig
- Log warnings if essential data missing
- Attempt fallback sources proactively

---

## Proposed Solutions

### Solution 1: Cross-Audience KB Fallback (Recommended)

Modify `LoadConfig()` to load fallback KB when audience-specific KB is missing.

**Changes to `service.go`:**

```go
// In AudienceConfig struct
type AudienceConfig struct {
    // ... existing fields
    RawKB         string
    FallbackKB    string  // NEW: Fallback KB content from other audience
    RawPolicy     string
}

// In LoadConfig()
var rawKB, fallbackKB, rawPolicy string

// Try audience-specific KB first
kbFileType := "kb"
latestOnly := true
if kbFile, err := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
    TenantID:     &tenant.ID,
    AudienceType: &audienceType,
    FileType:     &kbFileType,
    LatestOnly:   latestOnly,
}); err == nil && kbFile != nil {
    rawKB = kbFile.Content
} else {
    // NEW: Fall back to other audience's KB
    fallbackAudience := "external"
    if audienceType == "external" {
        fallbackAudience = "internal"
    }
    if fallbackFile, err := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
        TenantID:     &tenant.ID,
        AudienceType: &fallbackAudience,
        FileType:     &kbFileType,
        LatestOnly:   latestOnly,
    }); err == nil && fallbackFile != nil {
        fallbackKB = fallbackFile.Content
        slog.Warn("using fallback KB from other audience",
            "requested", audienceType,
            "fallback", fallbackAudience)
    }
}
```

**Changes to `sanitizer.go`:**

```go
// Modified function signature
func GetValidatedReplacementPhone(configuredPhone, rawKB, fallbackKB string) string {
    // First, check if the configured phone is valid
    if IsValidReplacementPhone(configuredPhone) {
        return configuredPhone
    }

    // Try extracting from primary KB
    if extractedPhone := ExtractPhoneFromKB(rawKB); extractedPhone != "" {
        return extractedPhone
    }

    // NEW: Try extracting from fallback KB
    if extractedPhone := ExtractPhoneFromKB(fallbackKB); extractedPhone != "" {
        slog.Debug("using phone from fallback KB", "phone", extractedPhone)
        return extractedPhone
    }

    return ""
}
```

### Solution 2: Proactive Validation at Config Load

Add validation step in `LoadConfig()` to warn about missing critical data:

```go
// After building config
if config.Audience != nil {
    phone := GetValidatedReplacementPhone(config.Audience.EmergencyPhone, config.RawKB, config.FallbackKB)
    if phone == "" {
        slog.Warn("no valid emergency phone configured",
            "tenant", tenantSlug,
            "audience", audienceType,
            "db_phone", config.Audience.EmergencyPhone)
    }
}
```

### Solution 3: Fix External Audience Phone in DB

As an immediate mitigation, update the external audience to have the correct phone:

```sql
UPDATE agent_audiences
SET emergency_phone = '(631) 358-5200'
WHERE tenant_id = 1 AND audience_type = 'external';
```

---

## Implementation Priority

| Solution | Priority | Effort | Impact |
|----------|----------|--------|--------|
| Solution 3 (Fix DB) | Immediate | Low | Fixes current issue |
| Solution 1 (KB Fallback) | High | Medium | Prevents future issues |
| Solution 2 (Validation) | Medium | Low | Improves observability |

---

## Files to Modify

| File | Change |
|------|--------|
| `server/router/api/v1/agent/service.go` | Add FallbackKB field, load fallback KB |
| `server/router/api/v1/agent/sanitizer.go` | Add fallbackKB parameter to GetValidatedReplacementPhone |
| All callers of GetValidatedReplacementPhone | Update to pass fallback KB |

**Callers to update:**
- `service.go:647` - generateRAGResponse
- `service.go:745` - processChat
- `service.go:1045` - buildSystemPrompt
- `service.go:1396` - buildRAGSystemPrompt
- `compliance.go:175` - checkContactInfo
- `verifier.go:171` - buildVerifierPrompt

---

## Testing Plan

1. **Unit Test**: `GetValidatedReplacementPhone` with fallback
   - Primary KB has phone → returns primary
   - Primary KB empty, fallback has phone → returns fallback
   - Both empty → returns ""

2. **Integration Test**: Simulation with missing audience KB
   - Create simulation for tenant with only external KB
   - Verify agent uses correct phone from external KB

3. **Regression Test**: Existing functionality
   - Verify normal chat flow unchanged
   - Verify phone correction still works

---

## Conclusion

The phone hallucination occurred because:
1. Simulation used "internal" audience (hardcoded)
2. Internal KB didn't exist at simulation time
3. No fallback to external KB which had correct phone
4. LLM hallucinated "(555) 000-0000" placeholder

The recommended fix is implementing **cross-audience KB fallback** to ensure critical contact information is always available, regardless of which audience-specific files have been uploaded.

---

*Document Author: Claude Code*
*Analysis Date: 2026-01-23*
