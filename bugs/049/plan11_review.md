# Plan 11 — Adversarial Review

**Reviewed by:** Senior Go Architect
**Date:** 2026-07-28
**Verdict:** APPROVED WITH NITS — 0 critical, 0 significant, 3 minor

---

## Review Summary

All 4 review findings from plan10 (S1, N1-N3) are properly addressed. Two new minor issues introduced.

| # | Finding | Severity | Location | Verdict |
|---|---------|----------|----------|---------|
| S1 | Denominator double-count (plan10 S1) | | | ✅ Fixed |
| N1 | Abstention duplicate runs (plan10 N1) | | | ✅ Fixed |
| N2 | Gold baseline dropped (plan10 N2) | | | ✅ Fixed |
| N3 | JSONL date-only naming (plan10 N3) | | | ⚠️ See N4 |
| N4 | Timestamp naming broke cross-process crash recovery | Minor | Lines 152-155, 192 | Fix |
| N5 | No filter function for abstention-only questions | Minor | Lines 218-231 | Fix |
| N6 | `saveResults` vs `TestBenchmarkAggregate` roles unclear | Minor | Lines 209, 247 | Fix |

---

## Minor Findings

### N4: Timestamp naming broke cross-process crash recovery

**Location:** Lines 152-155, 192

N3 fix changed from date-only (`20060102`) to timestamp (`20060102_150405`). This regressed the crash recovery feature:

- **Before (date-only):** All same-day runs share one JSONL file. `loadCompletedFromJSONL` reads previous run's results and skips completed questions. Cross-process crash recovery works.
- **After (timestamp):** Each run generates a unique filename. `loadCompletedFromJSONL` finds empty file → starts from scratch. A re-run after a crash loses all previous progress.

The original date-only naming was actually correct — `loadCompletedFromJSONL` already handles the "collision" concern by skipping completed questions.

**Fix:** Revert to date-only naming and add `BENCHMARK_FRESH=true` env var to clear existing JSONL for clean runs. Or keep timestamp but add a resume mechanism (e.g., `BENCHMARK_RESUME=true` to scan for any existing JSONL matching the type prefix).

---

### N5: No filter function for abstention-only questions

**Location:** Lines 218-231

`filterByType(questions, qType, includeAbs)` only filters by a single question type. Abstention questions span multiple types (single_hop, knowledge_update). The plan shows `filterByType(questions, "single_hop", false)` and `filterByType(questions, "knowledge_update", false)` for non-abs tests, but there is no filter for "select all abstention questions regardless of type" for `TestBenchmarkAbstention`.

Line 102 says Abstention = 12 questions but doesn't show how they are selected from the dataset.

**Fix:** Add a `filterAbstention(questions []BenchmarkQuestion) []BenchmarkQuestion` helper:
```go
func filterAbstention(questions []BenchmarkQuestion) []BenchmarkQuestion {
    var result []BenchmarkQuestion
    for _, q := range questions {
        if strings.HasSuffix(q.QuestionID, "_abs") {
            result = append(result, q)
        }
    }
    return result
}
```

---

### N6: `saveResults` vs `TestBenchmarkAggregate` roles unclear

**Location:** Lines 209, 247

Each per-type test calls `saveResults(results, "single_hop")` (line 209). Meanwhile `TestBenchmarkAggregate` reads JSONL files to compute the summary (line 247). What does `saveResults` produce that the aggregate doesn't?

If `saveResults` writes a JSON summary file that isn't consumed by the aggregate, it's confusing dead output. If it writes additional JSONL, that duplicates `appendJSONL`.

**Fix:** Clarify the relationship:
- `appendJSONL`: crash recovery (per-question, append)
- `saveResults`: human-readable JSON summary per type (optional, not consumed by aggregate)
- `TestBenchmarkAggregate`: canonical final report (reads per-question JSONL, computes all metrics)

Or simply remove `saveResults` and let `TestBenchmarkAggregate` be the single aggregation point.

---

## Additional Observations (non-blocking)

- **Cache parsed 4 times:** Each per-type test calls `loadBenchmarkData` independently, parsing 415MB cache 4× in sequential runs (or ~1.2GB in parallel). Consider lazy loading or `t.Cleanup` with a sync.Once if this becomes a performance issue.
- **Dynamic denominator:** Line 261 hardcodes 176. At runtime, 2 knowledge_update questions may be missing cache → actual testable = 174. Aggregate should compute denominator from `len(results)` across all JSONL files, not hardcode it.

---

## Implementation Readiness

| Criterion | Status |
|-----------|--------|
| Per-type decomposition with correct question split | ✅ |
| Abstention excluded from parent types | ✅ |
| Gold baseline rationale documented | ✅ |
| JSONL crash recovery format | ✅ |
| JSONL naming handles cross-process re-runs | ❌ N4 (broken by timestamp) |
| Abstention-only filter function present | ❌ N5 (missing) |
| Aggregate uses unique (not summed) denominator | ✅ |
| `saveResults` vs aggregate roles clear | ❌ N6 (unclear) |
| Implementation order clear | ✅ |

---

**Proceed after addressing N4-N6.** No rework needed — all findings are minor and the architecture is sound. N4 is the most impactful (loss of cross-process crash recovery).
