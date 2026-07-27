# Dry Run 4 — Preference Benchmark Review

**Date:** 2026-07-28
**Test:** `TestBenchmarkPreference` (30 questions)
**Result:** 30/30 completed, all saved to JSONL
**File:** `server/router/api/v1/agent/build/benchmark/implicit_preference_v2_20260728.jsonl`

---

## Summary

| Metric | Value |
|--------|-------|
| Total | 30 |
| Pass | 14 (46.7%) |
| Fail | 16 (53.3%) |
| Skip | 0 |

**Significantly lower than SingleHop (92.2%).** This is a different failure mode, not an OM pipeline issue.

---

## Root Cause Analysis

**Not just model routing — it's task difficulty + model capability.**

### Same Models, Different Results

| Model | SingleHop | Preference | Delta |
|-------|-----------|------------|-------|
| openai/gpt-oss-20b:free | 100% (6/6) | **0%** (0/2) | -100% |
| poolside/laguna-xs-2.1:free | 100% (4/4) | **20%** (1/5) | -80% |
| cohere/north-mini-code:free | 100% (5/5) | **33%** (1/3) | -67% |
| nvidia/nemotron-3-ultra-550b-a55b:free | 100% (4/4) | **25%** (1/4) | -75% |
| google/gemma-4-31b-it:free | 100% (3/3) | 100% (2/2) | 0% |
| inclusionai/ling-3.0-flash:free | 100% (3/3) | 100% (2/2) | 0% |
| nvidia/nemotron-nano-9b-v2:free | 100% (7/7) | 100% (2/2) | 0% |

Models that excel at factual recall (SingleHop) fail at preference inference (Preference).

---

## Failure Pattern

All 16 failures share the same root cause:

| Failure Type | Count | Description |
|-------------|-------|-------------|
| "I don't have enough information" | 14 | Answer model didn't use observation log context |
| "User Safety: safe" | 2 | Same `nemotron-3.5-content-safety` routing issue |

**The observer correctly captures facts** (observation logs show the right information). The bottleneck is the **answer model** — free models routed by `openrouter/free` are not strong enough to extract implicit preferences from observation logs.

### Example Failure

```
Question: "Can you recommend some resources where I can learn more about video editing?"
Observation Log: User is interested in video editing, has Adobe Premiere Pro, wants to improve skills
Expected: "The user would prefer responses that suggest resources specific to their interests"
Actual: "I don't have enough information to answer this question."
```

The observation log contains the context. The answer model failed to use it.

---

## Three Factors

| Factor | Impact | Fix |
|--------|--------|-----|
| **Task difficulty** | Preference inference is fundamentally harder than factual recall | Accept lower baseline |
| **Model capability** | Not all free models can reason about implicit preferences from context | Use consistent, capable model |
| **Answer prompt** | Current prompt doesn't explicitly instruct "use the observation log to infer preferences" | Improve prompt |

---

## Comparison: SingleHop vs Preference

| Test | Observer | Answer Model | Judge | Result |
|------|----------|-------------|-------|--------|
| SingleHop | ✅ Works | ✅ Works | Strict | 92.2% |
| Preference | ✅ Works | ❌ **Fails to use context** | Strict | 46.7% |

**The observer is working correctly.** The issue is answer model capability for preference-based reasoning.

---

## Model Performance

| Model | Questions | Pass | Fail | Rate |
|-------|-----------|------|------|------|
| google/gemma-4-31b-it:free | 2 | 2 | 0 | 100% |
| inclusionai/ling-3.0-flash:free | 2 | 2 | 0 | 100% |
| nvidia/nemotron-nano-9b-v2:free | 2 | 2 | 0 | 100% |
| nvidia/nemotron-3-super-120b-a12b:free | 1 | 1 | 0 | 100% |
| nvidia/nemotron-nano-12b-v2-vl:free | 3 | 2 | 1 | 67% |
| nvidia/nemotron-3-nano-30b-a3b:free | 2 | 1 | 1 | 50% |
| poolside/laguna-xs-2.1:free | 5 | 1 | 4 | 20% |
| cohere/north-mini-code:free | 3 | 1 | 2 | 33% |
| nvidia/nemotron-3-ultra-550b-a55b:free | 4 | 1 | 3 | 25% |
| openai/gpt-oss-20b:free | 2 | 0 | 2 | 0% |
| nvidia/nemotron-3.5-content-safety:free | 2 | 0 | 2 | 0% |

---

## Recommendation: Stick to One LLM

**Yes, for benchmarking consistency.**

| Approach | Pros | Cons |
|----------|------|------|
| **One model** | Consistent, reproducible, easier to diagnose | Higher cost (if paid), single point of failure |
| **Random free models** | $0 cost, reflects production variance | 46.7% vs 92.2% is noise, not signal |

### Options

1. **`openai/gpt-4o-mini`** — ~$0.05 for 30 questions, consistent, capable of preference reasoning
2. **`google/gemma-4-31b-it:free`** — scored 100% on both tasks, still free but with potential variance

---

## Next Steps

- [ ] Choose answer model strategy (one model vs random free)
- [ ] Run KnowledgeUpdate type (72 questions, ~19 min)
- [ ] Run Abstention type (12 questions, ~3 min)
- [ ] Run Aggregate summary
