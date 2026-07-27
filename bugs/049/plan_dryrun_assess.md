# Plan Dry Run Assessment

**Assessed by:** Senior Go Architect
**Date:** 2026-07-28
**Verdict:** PROCEED WITH MODIFICATIONS — 2 critical fixes required

---

## Review Summary

The adversarial review (`plan_dryrun_review.md`) identified 1 critical, 1 significant, and 3 minor findings. All are valid.

| # | Finding | Severity | Verdict |
|---|---------|----------|---------|
| C1 | Abstention type not tested | Critical | Fix required |
| C2 | API call count wrong (6→9) | Critical | Fix required |
| S1 | No gold baseline | Significant | Optional |
| N1 | No failure semantics | Minor | Fix |
| N2 | Gold threshold not mentioned | Minor | Depends on S1 |

---

## Critical Fixes

### C1: Add Abstention Question

**Current:** 3 questions — `single_hop`, `implicit_preference_v2`, `knowledge_update`
**Required:** 4 questions — add `_abs`-suffixed question

**Rationale:** Abstention judge prompt (`refused-to-answer` logic) is the most novel variant. If it's broken, 12 questions silently fail in the full run. Adding 1 question is trivial (~$0.002 cost, ~2 min time).

**Implementation:** Pick first `_abs`-suffixed question from any testable type.

### C2: Fix API Call Count

**Current (plan_dryrun.md:68):** "6 API calls: 3 answer + 3 judge"
**Correct:** "12 API calls: 4 observer + 4 answer + 4 judge" (after C1 fix)

Each question = observer + answer + judge = 3 LLM calls. The observer call was omitted from the original count.

---

## Gold Baseline (Optional)

**Recommendation:** Include 1 gold baseline question (5 total questions).

**Cost:** ~$0.002 additional (negligible)
**Time:** ~3 min additional
**Value:** Distinguishes "OM is bad" from "answer LLM is bad" — critical diagnostic signal

If included, update API call count to "15 API calls: 5 observer + 5 answer + 5 judge".

---

## Cost Analysis

### Dry Run (5 questions with gold + abstention)

| Call Type | Model | Count | Cost |
|-----------|-------|-------|------|
| Observer | openrouter/free | 5 | $0 |
| Answer | openrouter/free | 5 | $0 |
| Judge | openai/gpt-4o | 5 | ~$0.011 |
| **Total** | | 15 | **~$0.011** |

### Full Benchmark (176 + 10 gold = 186 questions)

| Call Type | Model | Count | Cost |
|-----------|-------|-------|------|
| Observer | openrouter/free | 186 | $0 |
| Answer | openrouter/free | 186 | $0 |
| Judge | openai/gpt-4o | 186 | ~$0.39 |
| **Total** | | 558 | **~$0.39** |

### Combined

| Run | Questions | API Calls | Cost |
|-----|-----------|-----------|------|
| Dry run | 5 | 15 | ~$0.01 |
| Full benchmark | 186 | 558 | ~$0.39 |
| **Total** | **191** | **573** | **~$0.40** |

---

## Time Estimate

| Run | Questions | Estimated Time |
|-----|-----------|----------------|
| Dry run | 5 | ~8-10 min |
| Full benchmark | 186 | ~20-40 min |

Time is dominated by `openrouter/free` tier rate limits, not cost.

---

## GPT-4o as Judge — Rationale

**Why gpt-4o and not MiMo-V2.5 or other models?**

Benchmark comparability. All published SOTA scores on LongMemEval use gpt-4o as the judge:

| System | Judge | Score |
|--------|-------|-------|
| Mastra OM | gpt-4o | 84.23% |
| EmergenceMem | gpt-4o | 86.00% |
| Zep | gpt-4o | 71.20% |
| Supermemory | gpt-4o | 81.60% |

Using a different judge introduces unmeasured calibration bias. The observer/answer model is already non-deterministic (`openrouter/free`) — the judge should be the fixed reference point.

**If cost matters more than comparability:** Use `gpt-4o-mini` (~$0.02 total) — same model family, similar calibration.

---

## Dry Run vs Plan9 Smoke Test

They serve complementary purposes:

| Aspect | Dry Run | Plan9 Smoke Test |
|--------|---------|------------------|
| Purpose | Manual inspection of artifacts | Automated pass/fail gate |
| Output | Verbose human-readable report | Binary pass/fail |
| Questions | 4-5 (expanded) | 4 (1 per type) |
| Execution | Separate test function | Built into `TestBenchmarkLongMemEval` |
| When to use | Before first full run | Every run |

The dry run should exist as a **separate test function** (`TestBenchmarkLongMemEvalDryRun`) that complements plan9's built-in smoke test.

---

## Modified Dry Run Plan

### Questions (5 total)

1. `single_hop` — first non-abs question
2. `implicit_preference_v2` — first non-abs question
3. `knowledge_update` — first non-abs question
4. `*_abs` — first abstention question (any type)
5. Gold baseline — answer injected instead of observation log

### Output Format

```
=== Dry Run: Question 1/5 ===
Question ID:     q_034
Question Type:   single_hop
Question:        What did the customer say about their water heater?

Input Turns (10 total):
  [0] user: "My water heater is making a loud noise"
  [1] agent: "I understand your concern..."
  ...

Observation Log (Raw):
  [INFO] 2024-01-15T10:30:00 | Customer reported water heater noise
  ...

Generated Answer:
  The customer reported their water heater is making a loud noise.

Expected Answer:
  The customer's water heater is making a loud noise and they want it inspected.

Judge Verdict: yes
Model Used: openrouter/free → <actual_model>

---
```

### Report File

Written to `build/benchmark/dryrun_YYYYMMDD_HHMMSS.txt`.

### Failure Semantics

On failure (observer error, LLM timeout, nil observation): log error, record as "skipped", continue to next question. Match plan9 behavior.

### Gold Baseline Gate

After gold question, print:
- "Gold baseline: 1/1 passed — answer LLM is reliable"
- "Gold baseline: 0/1 failed — UNRELIABLE — check answer LLM before full run"

---

## Run Command

```bash
BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... \
  go test ./server/router/api/v1/agent/ -run TestBenchmarkLongMemEvalDryRun \
  -v -count=1 -timeout=10m
```

---

## Files Changed

| File | Change | Lines |
|------|--------|-------|
| `server/.../agent/benchmark_longmemeval_test.go` | NEW — dry run test | ~50 |

No changes to existing files. Shares all helper code with plan9.

---

## Proceed?

**Yes, after applying 2 critical fixes:**

1. Expand to 5 questions (add `_abs` abstention + 1 gold)
2. Correct call count to "15 API calls: 5 observer + 5 answer + 5 judge"
3. Add failure semantics (1 line)
4. Add gold baseline pass/fail gate

Total implementation: ~50 lines, ~$0.01 cost, ~8-10 min runtime.
