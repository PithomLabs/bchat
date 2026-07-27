# Plan 10 — Adversarial Review

**Reviewed by:** Senior Go Architect
**Date:** 2026-07-28
**Verdict:** APPROVED WITH NITS — 0 critical, 1 significant, 3 minor

---

## Review Summary

| # | Finding | Severity | Location | Verdict |
|---|---------|----------|----------|---------|
| S1 | Aggregate denominator double-counts abstention | Significant | Lines 231-237 | Fix |
| N1 | Abstention test re-runs 12 already-covered questions | Minor | Lines 95-100 | Fix |
| N2 | Gold baseline from plan8 dropped without explanation | Minor | Not present | Fix |
| N3 | JSONL date-only naming collides on same-day re-runs | Minor | Line 186 | Fix |

---

## Significant Findings

### S1: Aggregate denominator double-counts abstention questions

**Location:** Lines 231-237

The aggregate reports `128/188 (68.1%)`. The 188 = 70 + 30 + 76 + 12, but abstention (12) is a **subset** of SingleHop (6) and KnowledgeUpdate (6). Unique question count is 176, not 188.

True accuracy on unique questions: **128/176 = 72.7%**, a 4.6pp difference — directly affects the benchmark's headline number.

**Root cause:** `TestBenchmarkSingleHop` runs 70 questions (including 6 abstention). `TestBenchmarkAbstention` runs 12 questions (6 of those same 6 + 6 from KnowledgeUpdate). The aggregate sums all results without deduplication.

**Fix (pick one):**
- **(A) Cleanest:** Filter abstention questions OUT of SingleHop and KnowledgeUpdate. Then: SingleHop=64, Preference=30, KnowledgeUpdate=72, Abstention=12. Unique=178. Aggregate uses unique denominator.
- **(B) Simplest:** Keep abstention in SingleHop/KnowledgeUpdate but remove `TestBenchmarkAbstention`. Derive abstention accuracy as a subset analysis in `TestBenchmarkAggregate`.
- **(C) Minimal change:** Keep all tests, set aggregate denominator to unique count (176), add note: "Abstention results are a subset of single-hop and knowledge-update."

---

## Minor Findings

### N1: Abstention test wastes 36 API calls re-running covered questions

**Location:** Lines 95-100

12 questions run twice (6 from single_hop + 6 from knowledge_update). Each question = observer + answer + judge = 3 calls × 12 = 36 wasted API calls (~$0.027, ~3 min). Also risks contradictory results if openrouter/free routes differently on re-run.

**Fix:** Resolves naturally if S1 option (A) or (B) is chosen. If (C), add a note that abstention results are re-runs and may differ from the first run.

---

### N2: Gold baseline from plan8 dropped without explanation

**Location:** Not present in plan10

Plan8 specifies a 10-question gold baseline to distinguish "OM is bad" from "answer LLM is bad." Without it, low accuracy gives no diagnostic signal about root cause.

**Fix:** Either include a gold baseline (3-5 questions, injected answer) or add a rationale: "Gold baseline deferred to dry run — run `TestBenchmarkLongMemEvalDryRun` first if accuracy is unexpectedly low."

---

### N3: JSONL date-only naming collides on same-day re-runs

**Location:** Line 186

```go
jsonlPath := fmt.Sprintf("build/benchmark/single_hop_%s.jsonl", time.Now().Format("20060102"))
```

If a test crashes and is re-run on the same day, `loadCompletedFromJSONL` skips questions from the failed run — potentially skipping questions that should be retried.

**Fix:** Include time: `Format("20060102_150405")` or add a `BENCHMARK_FORCE` env var to clear existing JSONL on fresh runs.

---

## Implementation Readiness

| Criterion | Status |
|-----------|--------|
| Per-type test decomposition | ✅ |
| Shared helpers correctly identified | ✅ |
| Crash recovery (JSONL per type) | ✅ |
| JSONL skip-completed-on-re-run | ✅ |
| Parallel execution support documented | ✅ |
| Dedicated aggregation test | ✅ |
| Aggregate denominator correct | ❌ S1 |
| Gold baseline included or rationale stated | ❌ N2 |
| Abstention avoids duplicate runs | ❌ N1 |
| JSONL naming collision-safe | ❌ N3 |

---

**Proceed after addressing S1 and N1-N3.** S1 is the only correctness issue — it directly affects the reported accuracy percentage. N1-N3 are hardening and documentation.
