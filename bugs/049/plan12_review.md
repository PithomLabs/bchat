# Plan 12 — Adversarial Review

**Reviewed by:** Senior Go Architect
**Date:** 2026-07-28
**Verdict:** REWORK — 1 critical, 1 significant, 1 minor

---

## Review Summary

| # | Finding | Severity | Location | Verdict |
|---|---------|----------|----------|---------|
| C1 | `readJSONL` uses `dec.More()` — same critical bug as B1 in code review | Critical | Lines 274-282 | Fix |
| S1 | Aggregate dedup subtracts abstention from non-overlapping total | Significant | Lines 256-260 | Fix |
| N1 | `readJSONL` silently returns empty on missing file | Minor | Lines 269-272 | Fix |

Prior review findings status:

| Prior Finding | Status in plan12 |
|---------------|------------------|
| N4: Timestamp broke crash recovery | ✅ Fixed (date-only + `BENCHMARK_FRESH=true`) |
| N5: No abstention filter | ✅ Fixed (`filterAbstention` used) |
| N6: `saveResults` vs aggregate unclear | ✅ Fixed (`saveResults` removed) |
| B1: `loadCompletedFromJSONL` `dec.More()` | ❌ **Unfixed** — and now replicated |
| S1: `appendJSONL` silent errors | ❌ Not addressed (deferred) |
| S2: `content()` silent errors | ❌ Not addressed (deferred) |
| S3: Per-type tests missing | ✅ Fixed |

---

## Critical

### C1: `readJSONL` uses `dec.More()` — same critical bug as B1

**Location:** Lines 274-282

```go
dec := json.NewDecoder(strings.NewReader(string(data)))
for dec.More() {   // ← BUG: same pattern as loadCompletedFromJSONL
    var r BenchResult
    if err := dec.Decode(&r); err != nil {
        continue
    }
    results = append(results, r)
}
```

`dec.More()` reports whether another element exists in the **current array**. For JSONL (top-level streaming objects), `More()` returns `false` after the first `Decode`. This loop reads **exactly one entry** then exits.

**Impact:** `TestBenchmarkAggregate` reads 1 question per type, reporting e.g. "SingleHop: 1/1 passed" when the JSONL has 64 entries. The aggregate becomes worthless — wrong counts, wrong percentages.

**Fix:** Replace with `io.EOF` loop (requires adding `"io"` to imports):

```go
for {
    var r BenchResult
    err := dec.Decode(&r)
    if err == io.EOF {
        break
    }
    if err != nil {
        continue
    }
    results = append(results, r)
}
```

Also fix the pre-existing `loadCompletedFromJSONL` (lines 327-342) with the same pattern — plan12 must include both fixes.

---

## Significant

### S1: Aggregate dedup logic is incorrect

**Location:** Lines 256-260

```go
abstentionCount := len(readJSONL(benchmarkJSONLPath("abstention")))
uniqueTotal := len(allResults) - abstentionCount
```

This is a leftover from plan10 where abstention was **included** in parent types AND run separately. Plan11/12 **excludes** abstention from parent types via `filterByType(..., false)`. So each question runs exactly once — there is no overlap to deduplicate.

With all cache entries present:
- `allResults` = single_hop(64) + preference(30) + knowledge_update(72) + abstention(12) = **178 unique**
- `uniqueTotal = 178 - 12 = 166` ❌

Correct: **`uniqueTotal = len(allResults)`** = 178.

**Fix:** Remove the subtraction:

```go
uniqueTotal := len(allResults)
```

And update the comment: `// All questions are unique — abstention excluded from parent types via filterByType(..., false).`

---

## Minor

### N1: `readJSONL` silently returns empty on read error

**Location:** Lines 269-272

```go
data, err := os.ReadFile(path)
if err != nil {
    return results  // ← silent: caller thinks file is empty
}
```

If `TestBenchmarkAggregate` runs before any per-type test, or a file is missing, the aggregate shows "SingleHop: (no results)" with no indication the file doesn't exist.

**Fix:** Check `os.IsNotExist(err)` and log a meaningful message:

```go
if err != nil {
    if os.IsNotExist(err) {
        t.Logf("no results for %s (run per-type test first)", path)
    }
    return results
}
```

(Requires adding `t *testing.T` as a parameter to `readJSONL`, or inlining the read in `TestBenchmarkAggregate`.)

---

## Pre-Existing Bugs (not addressed, deferred)

These exist in the current codebase and are not fixed by plan12. They should be addressed before running the full benchmark:

| Bug | Location | Severity |
|-----|----------|----------|
| `loadCompletedFromJSONL` `dec.More()` | Lines 327-342 | Critical — must be fixed (covered by C1) |
| `appendJSONL` silent errors | Lines 316-325 | Significant — crash recovery writes silently fail |
| `content()` silent marshal errors | Lines 40-45 | Significant — data issues go undetected |
| `AnswerString()` literal `"<nil>"` | Lines 47-62 | Minor — misleading expected answer |

---

## Everything Else: Correct

| Component | Verdict |
|-----------|---------|
| Per-type test pattern (SingleHop/Preference/KnowledgeUpdate/Abstention) | ✅ Correct |
| `runBenchmarkQuestion` shared pipeline | ✅ Correct |
| `filterByType` with `includeAbs=false` | ✅ Correct |
| `filterAbstention` for abstention-only test | ✅ Correct |
| `benchmarkJSONLPath` date-only + `BENCHMARK_FRESH=true` | ✅ Correct |
| Dynamic denominator from actual results | ✅ Correct |
| `countResults` helper | ✅ Correct |
| Run commands | ✅ Correct |
| Implementation order | ✅ Correct |

---

**Rework needed:** Fix C1 (`readJSONL` loop + existing `loadCompletedFromJSONL`), fix S1 (dedup subtraction), add N1 (missing file handling). These are ~10 lines of changes across 3 functions. After those fixes, the plan is ready.
