# Plan Dry Run Review

**Reviewed by:** OpenCode (adversarial)
**Date:** 2026-07-27
**Verdict:** REWORK — 1 critical, 1 significant, 3 minor

---

## Critical Findings

### C1: Abstention type not tested

**Location:** Lines 12-15 (question selection)

Dry run picks 3 types: `single_hop`, `implicit_preference_v2`, `knowledge_update`. Omits `abstention` — which is the most novel judge prompt variant (refused-to-answer logic). If the abstention judge prompt is broken, the dry run won't catch it.

**Impact:** User proceeds to full run thinking pipeline is validated, but abstention judging silently fails on 12 questions.

**Fix:** Add a 4th question: pick a `_abs`-suffixed question from any testable type. Total becomes 4 questions.

---

### C2: API call count wrong

**Location:** Line 68

Says "6 API calls: 3 answer + 3 judge." But each question also requires a **observer call** (`RunObserver` → openrouter/free). That's 3 additional calls = **9 total**.

**Impact:** Time estimate is misleading (3-5 min is too low for 9 LLM calls with free tier rate limits).

**Fix:** Update to "9 API calls: 3 observer + 3 answer + 3 judge" and adjust time estimate to ~5-8 minutes.

---

## Significant Findings

### S1: No gold baseline in dry run

**Location:** Not present in plan

Plan8 specifies a 10-question gold baseline to validate the answer LLM itself. The dry run omits this entirely. If the answer LLM is noisy, the dry run can't distinguish "OM is bad" from "answer LLM is bad."

**Impact:** User can't trust answer quality — a "fail" could be OM fault or answer LLM fault.

**Fix:** Add 1 gold baseline question (answer injected instead of observation log). Report pass/fail. Total becomes 5 questions, ~12 API calls.

---

## Minor Findings

### N1: No failure semantics

**Location:** Not specified

If a question fails (observer error, LLM timeout, nil observation), does the dry run stop or continue? Plan9's full benchmark continues. Dry run should match.

**Fix:** Add one line: "On failure, log error, record as skipped, continue to next question."

---

### N2: Gold baseline threshold not mentioned

**Location:** Not present

Plan8 specifies `≥8/10` pass for gold baseline. Dry run should report this as a pass/fail gate before user proceeds to full run.

**Fix:** After gold question, print: "Gold baseline: X/1 — answer LLM is reliable" or "UNRELIABLE — check answer LLM before full run."

---

## Implementation Readiness

| Criterion | Status |
|-----------|--------|
| All 4 testable types covered | ❌ (missing abstention) |
| Gold baseline included | ❌ |
| API call count accurate | ❌ |
| Output format documented | ✅ |
| Run command correct | ✅ |
| Shares helper code with plan9 | ✅ |

**Plan needs rework before implementation.** Expand question set to 5 (4 types + 1 gold), correct call count, add failure semantics.
