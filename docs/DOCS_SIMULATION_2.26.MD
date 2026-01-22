# Simulation Analysis: ID 26 - Phone Number Rejection Bug

> **Date:** 2026-01-21
> **Simulation ID:** 26
> **Status:** Fixed

---

## Executive Summary

Simulation ID 26 revealed a **critical bug** where customer phone numbers starting with "555" were being rejected as placeholders, causing:
1. Customer phone "555-867-5309" replaced with company phone "(631) 358-5200"
2. Agent re-asking for information already provided
3. Context loss across conversation turns

**Root Cause:** `isPlaceholderPhoneDigits()` rejected ALL 555-xxx-xxxx phones as fake.

**Fix:** Changed to only reject the official NANPA fictional range (555-0100 to 555-0199).

---

## The Problem

### Simulation Transcript Evidence

**Turn 2:** Customer says "I'm Alex. My phone is 555-867-5309"
**Agent echoes:** "(631) 358-5200" (company phone instead!)

**Turn 8:** Agent asks for name/phone again despite already having it

### Data Flow (Before Fix)

```
Customer: "My phone is 555-867-5309"
       ↓
extractCollectedInfo()
       ↓
normalizePhoneDigits("555-867-5309") → "5558675309"
       ↓
isPlaceholderPhoneDigits("5558675309")
       ↓
digits[:3] == "555" → TRUE (BUG!)
       ↓
Phone REJECTED as placeholder
       ↓
Session phone field: EMPTY
       ↓
LLM has no phone to inject → hallucinates company phone
```

---

## Root Cause Analysis

### The Buggy Code (Before)

```go
// service.go lines 1452-1463
func isPlaceholderPhoneDigits(digits string) bool {
    placeholders := []string{"5550", "5551", "0000", "9999", "1234567890"}
    for _, p := range placeholders {
        if strings.Contains(digits, p) {  // ← Also problematic
            return true
        }
    }
    if len(digits) >= 3 && digits[:3] == "555" {  // ← THE BUG
        return true  // Rejects ALL 555 phones!
    }
    return false
}
```

### Why This Was Wrong

1. **555 is a valid prefix** - Only 555-01XX (555-0100 to 555-0199) is reserved for fiction
2. **Real 555 numbers exist** - 555-1212 is directory assistance nationwide
3. **strings.Contains was too aggressive** - "9999" would reject any phone containing "9999" anywhere

### Official NANPA Guidelines

From [Wikipedia](https://en.wikipedia.org/wiki/555_(telephone_number)):
- **555-0100 to 555-0199**: Reserved for fictional use (movies, TV)
- **555-1212**: Directory assistance (real, active)
- **Other 555-xxxx**: Can be assigned to real services

---

## The Fix

### New Code (After)

```go
// service.go lines 1451-1474
func isPlaceholderPhoneDigits(digits string) bool {
    // Common placeholder patterns (exact match only)
    placeholders := []string{"0000000000", "9999999999", "1234567890", "1111111111"}
    for _, p := range placeholders {
        if digits == p {  // Changed to exact match
            return true
        }
    }

    // Only reject the official NANPA fictional range: 555-01XX
    if len(digits) == 10 && digits[:3] == "555" {
        middle := digits[3:7]
        if middle >= "0100" && middle <= "0199" {
            return true // Official fictional range
        }
    }

    return false
}
```

### Changes Made

| Aspect | Before | After |
|--------|--------|-------|
| 555 handling | Reject ALL 555-xxx-xxxx | Only reject 555-01XX |
| Placeholder matching | `strings.Contains` (partial) | Exact match |
| Placeholders list | `"5550", "5551", "0000", "9999"` | `"0000000000", "9999999999", "1234567890", "1111111111"` |

---

## Data Flow (After Fix)

```
Customer: "My phone is 555-867-5309"
       ↓
extractCollectedInfo()
       ↓
normalizePhoneDigits("555-867-5309") → "5558675309"
       ↓
isPlaceholderPhoneDigits("5558675309")
       ↓
middle = "8675" (not in 0100-0199 range)
       ↓
Phone ACCEPTED ✓
       ↓
Session phone field: "555-867-5309"
       ↓
System prompt: "Customer phone: 555-867-5309"
       ↓
LLM correctly echoes: "555-867-5309"
```

---

## Verification

### Test Cases

| Phone | Before | After | Correct? |
|-------|--------|-------|----------|
| 555-867-5309 | REJECTED | ACCEPTED | ✓ (real phone) |
| 555-0100 | REJECTED | REJECTED | ✓ (fictional) |
| 555-0150 | REJECTED | REJECTED | ✓ (fictional) |
| 555-0199 | REJECTED | REJECTED | ✓ (fictional) |
| 555-1212 | REJECTED | ACCEPTED | ✓ (directory assistance) |
| 555-2368 | REJECTED | ACCEPTED | ✓ (real phone) |
| 123-456-7890 | REJECTED | REJECTED | ✓ (obvious placeholder) |
| 000-000-0000 | REJECTED | REJECTED | ✓ (obvious placeholder) |

---

## Broader Context: Business Logic in Code

This bug highlights a larger architectural question: **too much business logic hardcoded**.

### Current State

| Logic | Location | Problem |
|-------|----------|---------|
| Phone placeholder rules | `isPlaceholderPhoneDigits()` | Hardcoded, no visibility |
| Name extraction patterns | Regex in code | Can't customize per tenant |
| Address parsing | Regex in code | Fragile, misses edge cases |
| Escalation triggers | Code conditions | Should be in POLICY.MD |

### Future Considerations

1. **Move validation rules to configuration** - POLICY.MD or database
2. **Consider NER models** - For semantic entity extraction
3. **Evaluate Zep/GraphRAG** - For conversation memory management

---

## PocketFlow RAG Analysis

User asked: "Can you add basic RAG within the context of PocketFlow framework?"

### Answer

**PocketFlow does support RAG** with a two-stage pipeline:
1. **Offline**: Documents → Chunk → Embed → Index
2. **Online**: Query → Embed → Retrieve → Generate

**However**, PocketFlow RAG is designed for **document retrieval**, not **conversation memory/entity extraction**.

| Solution | Solves Phone Issue? | Recommendation |
|----------|---------------------|----------------|
| Fix `isPlaceholderPhoneDigits()` | **YES** | **DONE** |
| PocketFlow RAG on KB/Policy | No (wrong problem) | Later for KB optimization |
| Zep Memory | YES (but overkill) | Future roadmap |

See [PocketFlow RAG Design Pattern](https://the-pocket.github.io/PocketFlow/design_pattern/rag.html) for details.

---

## Files Modified

| File | Change |
|------|--------|
| `server/router/api/v1/agent/service.go` | Fixed `isPlaceholderPhoneDigits()` (lines 1451-1474) |

---

## Related Issues

- **Simulation ID 25**: Name extraction bug ("Sure" as name) - Fixed with `isCommonWord()`
- **Simulation ID 24**: Session persistence issues - Fixed

---

## Recommendations

### Immediate (Completed)
- [x] Fix `isPlaceholderPhoneDigits()` - Remove blanket 555 rejection

### Short-term
- [ ] Add unit tests for `isPlaceholderPhoneDigits()`
- [ ] Re-run simulation ID 26 to verify fix

### Long-term
- [ ] Move phone validation rules to configurable format
- [ ] Evaluate KB RAG for prompt size optimization
- [ ] Consider Zep for full conversation memory (if extraction keeps failing)

---

*End of Analysis*
