# Simulation Analysis: ID 25 - Dog Accident/Carpet Cleaning Scenario

> **Date:** 2026-01-21
> **Simulation ID:** 25 (sim-92c2054d-93f3-4914-a5ef-7ec59d92c40f)
> **Status:** Fixes Implemented

---

## Scenario

**User message:** "My dog had an accident on the living‑room carpet—there's a pretty strong smell and a dark stain that just won't come out. Can you guys help clean it up?"

---

## Benchmark Scores

| Metric | Score | Issue |
|--------|-------|-------|
| Conversation Flow | 5/20 | Phases never followed, agent jumped to refusal each turn |
| Tone Resolution | 5/15 | Repetitive, lacked empathy, no useful resolution |

---

## Issues Identified

| ID | Issue | Severity | Root Cause |
|----|-------|----------|------------|
| 1 | Agent skips greeting/intro phases | HIGH | SCRIPT.MD lacks out-of-scope handling |
| 2 | Immediate rejection without probing | CRITICAL | KB.MD doesn't clarify pet damage = mold risk |
| 3 | Phone corrupted to company line | HIGH | LLM substitutes company phone (MD behavior) |
| 4 | Name becomes "Sure" | MEDIUM | Code bug - affirmation words not filtered |
| 5 | Refuses safety tips for odors | MEDIUM | POLICY.MD "Stay In Scope" too strict |

---

## Root Cause Analysis

### Issue 1: Agent Skips Greeting/Intro

**Evidence:** Turn 1 agent response: "We do not offer carpet cleaning services. However, if you're experiencing any property damage..."

No "Hello", no company intro, no empathy.

**Root Cause:** SCRIPT.MD had no guidance for "service doesn't match" scenarios. When agent detected "carpet cleaning" (out of scope), it jumped directly to rejection.

**Fix:** Added PHASE 0.5 (Out-of-Scope Service Handling) to SCRIPT_DRAFT.MD

---

### Issue 2: No Probing for In-Scope Damage

**Evidence:** User mentioned "smell and dark stain" - this could indicate mold risk from wet carpet padding.

**Root Cause:** KB.MD didn't explicitly include "pet accidents with mold risk" as in-scope.

**Fix:** Added "Common Scenarios We Handle" section under Mold Remediation in KB_DRAFT.MD

---

### Issue 3: Phone Corrupted to Company Line

**Evidence:** Turns 5+: Agent shows "(631) 358-5200" instead of customer's "555-123-4567"

**Root Cause:** LLM behavior - sees company phone prominently in KB.MD contact section and substitutes it when echoing customer info.

**Fix:** Added "Preserve Customer Phone Verbatim" rule to POLICY_DRAFT.MD with explicit wrong/correct examples.

---

### Issue 4: Name Becomes "Sure"

**Evidence:** Turn 7: Agent shows "Name: Sure" instead of "John Doe"

**Root Cause:** `isCommonWord()` function in service.go didn't include affirmation words like "sure", "yeah", "yep".

**Fix:** Added affirmation words to `isCommonWord()` in service.go:
- "sure", "yeah", "yep", "right"
- "absolutely", "certainly", "definitely"
- "great", "perfect", "thanks", "thank"

---

### Issue 5: Refuses Safety Tips for Odors

**Evidence:** User asks for odor mitigation tips; agent refuses because "odor-remediation" isn't a listed service.

**Root Cause:** POLICY.MD "Stay In Scope" rule was too strict - blocked even safety advice.

**Fix:** Added two items:
1. "Safety Tips Always Permissible" rule in POLICY_DRAFT.MD
2. "General Odor Safety" section in KB_DRAFT.MD Emergency Safety Procedures

---

## Files Modified

### MD Files (Surgical Updates)

| File | Changes |
|------|---------|
| `SCRIPT_DRAFT.MD` | Added PHASE 0.5 (Out-of-Scope Service Handling) between PHASE 0 and PHASE 1 |
| `KB_DRAFT.MD` | Added "Common Scenarios We Handle" to Mold section; Added "General Odor Safety" section |
| `POLICY_DRAFT.MD` | Added "Preserve Customer Phone Verbatim" rule; Added "Safety Tips Always Permissible" rule |

### Code Files

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/service.go` | Added affirmation words to `isCommonWord()` function |

---

## Versioned Files Created

| File | Purpose |
|------|---------|
| `KB_1768967917.MD` | Database snapshot for comparison |
| `POLICY_1768967917.MD` | Database snapshot for comparison |
| `SCRIPT_1768967917.MD` | Database snapshot for comparison |

---

## Does Tightening MD Files Help?

**Answer: YES for issues 1, 2, 3 (partial), 5. NO for issue 4.**

| Issue | MD Fix Helps? | Explanation |
|-------|---------------|-------------|
| Skip greeting/intro | **YES** | SCRIPT.MD now has explicit out-of-scope handling flow |
| No probing | **YES** | KB.MD now clarifies pet damage with mold risk is in scope |
| Phone corruption | **Partial** | POLICY.MD rule helps but LLM behavior may still vary |
| Name "Sure" | **NO** | Code bug requiring code fix |
| Refuses safety tips | **YES** | POLICY.MD now explicitly permits safety tips |

---

## Verification Plan

1. Re-run simulation with same scenario ("dog accident on carpet")
2. Verify agent greets and probes before declining
3. Verify agent asks: "Is the area still wet or damp?"
4. Verify phone number preserved correctly in confirmation
5. Verify name extracted correctly (not "Sure")
6. Verify safety/ventilation tips provided when asked
7. Verify agent pivots to mold services if moisture detected

---

## Protocol Reminder

When updating MD files:
1. Query database for latest versions first
2. Save with UNIX epoch suffix for version control
3. Make surgical updates only
4. Save to *_DRAFT.MD files for review

---

*End of Analysis*
