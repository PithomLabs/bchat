# Plan Dry Run 2 — Adversarial Review

**Reviewed by:** Senior Go Architect
**Date:** 2026-07-28
**Verdict:** APPROVED WITH NITS — 0 critical, 0 significant, 4 minor

---

## Review Summary

| # | Finding | Severity | Location | Verdict |
|---|---------|----------|----------|---------|
| N1 | Time estimate ignores measured observer baseline | Minor | Lines 10-15, 197 | Fix |
| N2 | Verdict parsing silently swallows empty/failed responses | Minor | Lines 122-127 | Fix |
| N3 | Gold baseline exclusion not acknowledged | Minor | Lines 17-22 | Fix |
| N4 | "Expected Answer" for abstention is misleading | Minor | Lines 142-146, 182 | Fix |

---

## Minor Findings

### N1: Time estimate ignores measured observer baseline

**Location:** Lines 10-15, 197

Dry run 1 took **63.1s** for 4 observer calls only. Answer LLM uses the same `openrouter/free` tier with identical rate limiting (~16s/call). Adding 4 answer calls adds ~64s, and 4 judge calls at paid tier adds ~8s. Total for new work: ~72s.

The plan says "Estimated time: ~2-3 min (4 observer + 4 answer + 4 judge calls)." This is correct only if the reader assumes the observer is NOT re-run. If re-running from scratch (including observer), total is ~63 + 72 = ~2.25 min.

**Fix:** Add scoping: "Estimated time: ~2-3 min for answer + judge only (observer already verified in dry run 1)." Or if full re-run: "~3-4 min total (observers are rate-limited at ~16s/call based on dry run 1 measurement)."

---

### N2: Verdict parsing silently swallows empty/failed responses

**Location:** Lines 122-127

```go
if strings.Contains(verdict, "yes") {
    return "yes", nil
}
return "no", nil
```

If the judge call returns an empty string (timeout, network error, empty choice), the function returns `"no"` with `nil` error. This silently marks a judge failure as a "fail" — indistinguishable from a legitimate "no" verdict.

**Fix:** Return an error when `verdict == ""`:
```go
if verdict == "" {
    return "", fmt.Errorf("empty judge response")
}
```

Also, `strings.Contains(verdict, "yes")` matches "yesterday", "yesn't" — a prefix or exact match is safer but this pattern is consistent with plan8/9.

---

### N3: Gold baseline exclusion not acknowledged

**Location:** Lines 17-22

The assessment and its review debated gold baseline inclusion. The plan chose 4 questions without gold. This is a valid design decision, but the plan doesn't state the rationale. A reader may think the gold baseline was overlooked.

**Fix:** Add one sentence: "Gold baseline excluded per assessment review (N=1 is noise-dominated with openrouter/free variance). Full 10-question baseline in plan9's `TestBenchmarkLongMemEval` serves as the real gate."

---

### N4: "Expected Answer" for abstention question is misleading

**Location:** Lines 142-146 (output format), Line 182 (expected results table)

The abstention judge prompt does NOT use the expected answer — it only checks if the model refused. Displaying `Expected Answer: <answer>` for the abstention question implies the judge compares against it, which is incorrect. The judge's abstention prompt omits the expected answer entirely.

**Fix:** Omit `Expected Answer` from abstention output, or label it differently: `Unanswerable Context: question cannot be answered from available information.`

---

## Implementation Readiness

| Criterion | Status |
|-----------|--------|
| Answer generation code correct | ✅ |
| Judge 3 prompt variants correct | ✅ |
| Judge prompt variant detection (`_abs` before `implicit_preference_v2`) | ✅ |
| Error handling at control-flow level (skip on failure) | ✅ |
| API call count accurate (12 calls) | ✅ |
| Cost analysis correct (~$0.008) | ✅ |
| Run command correct | ✅ |
| Dry run 1 observer results validated | ✅ |
| Gold baseline decision documented | ❌ Nit 3 |
| Verdict parsing handles empty response | ❌ Nit 2 |
| Time estimate matches measured baseline | ❌ Nit 1 |
| Abstention output not misleading | ❌ Nit 4 |

---

**Proceed after addressing 4 minor nits.** No rework needed — all findings are documentation and edge-case hardening. The core logic (answer generation, judge with 3 prompt variants, control flow) is correct.
