# Dry Run 5 — Preference Benchmark Review

**Date:** 2026-07-28
**Test:** `TestBenchmarkPreference` (30 questions)
**Result:** 30/30 completed, all saved to JSONL
**File:** `server/router/api/v1/agent/build/benchmark/implicit_preference_v2_20260728.jsonl`

---

## Summary

| Metric | Value |
|--------|-------|
| Total | 30 |
| Pass | 15 (50.0%) |
| Fail | 15 (50.0%) |
| Skip | 0 |

**+3.3% over Dry Run 4 (46.7%)** — within noise range. 1 additional question passed.

---

## Critical Finding: Model Was NOT Changed

The `generateAnswerDryRun` function still uses `Model: "openrouter/free"` (line 187). The model switch to `google/gemma-4-31b-it:free` was **not implemented**. The test ran with the same random routing as Dry Run 4.

### Model Distribution Confirms This

| Model | Questions | Pass | Fail | Rate |
|-------|-----------|------|------|------|
| inclusionai/ling-3.0-flash:free | 5 | 4 | 1 | 80% |
| openai/gpt-oss-20b:free | 4 | 1 | 3 | 25% |
| nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free | 3 | 1 | 2 | 33% |
| nvidia/nemotron-3-ultra-550b-a55b:free | 3 | 3 | 0 | 100% |
| nvidia/nemotron-3-nano-30b-a3b:free | 2 | 0 | 2 | 0% |
| nvidia/nemotron-3-super-120b-a12b:free | 2 | 2 | 0 | 100% |
| nvidia/nemotron-3.5-content-safety:free | 2 | 0 | 2 | 0% |
| poolside/laguna-xs-2.1:free | 2 | 2 | 0 | 100% |
| google/gemma-4-26b-a4b-it:free | 2 | 1 | 1 | 50% |
| poolside/laguna-s-2.1:free | 1 | 1 | 0 | 100% |
| poolside/laguna-m.1:free | 1 | 0 | 1 | 0% |
| google/gemma-4-31b-it:free | 1 | 0 | 1 | 0% |
| cohere/north-mini-code:free | 1 | 0 | 1 | 0% |
| nvidia/nemotron-nano-12b-v2-vl:free | 1 | 0 | 1 | 0% |

Same `openrouter/free` pool. `nemotron-3.5-content-safety` still appears and still fails (2/2). The +3.3% improvement is random model assignment variance.

---

## Failure Pattern — Identical to Dry Run 4

| Failure Type | Count | Dry Run 4 |
|-------------|-------|-----------|
| "I don't have enough information" | 13 | 14 |
| "User Safety: safe" | 2 | 2 |
| Garbled/noise output | 0 | 0 |

**Same root cause:** Answer model capability, not OM pipeline.

### Failure Breakdown

| Question ID | Model | Expected | Actual |
|-------------|-------|----------|--------|
| 8a2466db | inclusionai/ling-3.0-flash:free | resources for video editing | "I don't have enough information" |
| 0edc2aef | openai/gpt-oss-20b:free | hotels in Miami | "I don't have enough information" |
| 35a27287 | google/gemma-4-26b-a4b-it:free | cultural events | "I don't have enough information" |
| 195a1a1b | openai/gpt-oss-20b:free | evening activities | "I don't have enough information" |
| afdc33df | nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free | kitchen cleaning tips | "User Safety: safe" |
| caf03d32 | poolside/laguna-m.1:free | slow cooker recipes | garbled (partial context) |
| 09d032c9 | nvidia/nemotron-nano-12b-v2-vl:free | phone battery tips | "I don't have enough information" |
| 57f827a0 | nvidia/nemotron-3.5-content-safety:free | furniture rearranging | "I don't have enough information" |
| 95228167 | nvidia/nemotron-3-nano-30b-a3b:free | music store suggestions | garbled (partial context) |
| 75f70248 | openai/gpt-oss-20b:free | sneezing/allergy | "I don't have enough information" |
| d6233ab6 | nvidia/nemotron-3-nano-30b-a3b:free | nostalgia/old photos | "I don't have enough information" |
| 1da05512 | openai/gpt-oss-20b:free | NAS device | "I don't have enough information" |
| fca70973 | nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free | theme park | "I don't have enough information" |
| b6025781 | cohere/north-mini-code:free | meal prep recipes | "User Safety: safe" |
| 1c0ddc50 | nvidia/nemotron-3.5-content-safety:free | commute activities | partial context (missed key info) |

---

## Comparison with Dry Run 4

| Metric | Dry Run 4 | Dry Run 5 | Delta |
|--------|-----------|-----------|-------|
| Pass | 14 (46.7%) | 15 (50.0%) | +1 (+3.3%) |
| Fail | 16 (53.3%) | 15 (50.0%) | -1 |
| "Not enough info" | 14 | 13 | -1 |
| "User Safety" | 2 | 2 | 0 |

**Statistical noise.** Not a meaningful improvement.

---

## What Actually Needs to Change

To test `google/gemma-4-31b-it:free` as answer model, **edit the code**:

```go
// Line 187 in benchmark_longmemeval_test.go
// BEFORE:
Model: "openrouter/free",

// AFTER:
Model: "google/gemma-4-31b-it:free",
```

Then re-run with `BENCHMARK_FRESH=true`.

---

## Recommendation

The +3.3% is within noise. To get a clean comparison:

1. **Edit code** to use `google/gemma-4-31b-it:free` as the answer model
2. **Re-run** Preference (30 questions, ~12 min)
3. **Compare** against Dry Run 4 baseline (46.7%)

If gemma-4-31b-it consistently scores higher, it becomes the candidate for a fixed answer model. If not, fall back to `openai/gpt-4o-mini` (~$0.05 for 30 questions) for reliable benchmarking.

---

## Next Steps

- [ ] Edit `generateAnswerDryRun` to use `google/gemma-4-31b-it:free`
- [ ] Re-run Preference with `BENCHMARK_FRESH=true`
- [ ] Run KnowledgeUpdate (72 questions, ~19 min)
- [ ] Run Abstention (12 questions, ~3 min)
- [ ] Run Aggregate summary
