# Plan 13: LongMemEval Per-Type Benchmark (Post-Review Fixes)

**Date:** 2026-07-28
**Depends on:** plan12.md, plan12_review.md
**Status:** READY TO IMPLEMENT

---

## Review Findings (from plan12_review.md)

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| C1 | `readJSONL` uses `dec.More()` — only reads 1 entry | Critical | Replace with `io.EOF` loop |
| S1 | Aggregate dedup subtracts abstention from non-overlapping total | Significant | Remove subtraction, `uniqueTotal = len(allResults)` |
| N1 | `readJSONL` silently returns empty on missing file | Minor | Log message when file not found |

Also fixes pre-existing B1: `loadCompletedFromJSONL` has same `dec.More()` bug.

---

## Fix 1: `loadCompletedFromJSONL` (existing code, line 327)

**Before:**
```go
func loadCompletedFromJSONL(path string) map[string]bool {
    completed := make(map[string]bool)
    data, err := os.ReadFile(path)
    if err != nil {
        return completed
    }
    dec := json.NewDecoder(strings.NewReader(string(data)))
    for dec.More() {
        var r BenchResult
        if err := dec.Decode(&r); err != nil {
            continue
        }
        completed[r.QuestionID] = true
    }
    return completed
}
```

**After:**
```go
func loadCompletedFromJSONL(path string) map[string]bool {
    completed := make(map[string]bool)
    data, err := os.ReadFile(path)
    if err != nil {
        return completed
    }
    dec := json.NewDecoder(strings.NewReader(string(data)))
    for {
        var r BenchResult
        if err := dec.Decode(&r); err == io.EOF {
            break
        } else if err != nil {
            continue
        }
        completed[r.QuestionID] = true
    }
    return completed
}
```

---

## Fix 2: `readJSONL` (new code in plan12)

**Before:**
```go
func readJSONL(path string) []BenchResult {
    var results []BenchResult
    data, err := os.ReadFile(path)
    if err != nil {
        return results
    }
    dec := json.NewDecoder(strings.NewReader(string(data)))
    for dec.More() {
        var r BenchResult
        if err := dec.Decode(&r); err != nil {
            continue
        }
        results = append(results, r)
    }
    return results
}
```

**After:**
```go
func readJSONL(t *testing.T, path string) []BenchResult {
    t.Helper()
    var results []BenchResult
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            t.Logf("no results file: %s (run per-type test first)", filepath.Base(path))
        }
        return results
    }
    dec := json.NewDecoder(strings.NewReader(string(data)))
    for {
        var r BenchResult
        if err := dec.Decode(&r); err == io.EOF {
            break
        } else if err != nil {
            continue
        }
        results = append(results, r)
    }
    return results
}
```

---

## Fix 3: `TestBenchmarkAggregate` dedup logic

**Before:**
```go
abstentionCount := len(readJSONL(benchmarkJSONLPath("abstention")))
uniqueTotal := len(allResults) - abstentionCount
```

**After:**
```go
// All questions are unique — abstention excluded from parent types via filterByType(..., false).
uniqueTotal := len(allResults)
```

Update all `readJSONL` calls in aggregate to pass `t`:
```go
results := readJSONL(t, jsonlPath)
```

---

## Fix 4: Add `"io"` to imports

Add `"io"` to the import block (line 3-16).

---

## Implementation Order

1. Add `"io"` to imports
2. Fix `loadCompletedFromJSONL` (io.EOF loop)
3. Add `readJSONL` (io.EOF loop + missing file logging)
4. Fix `TestBenchmarkAggregate` (remove dedup subtraction, update readJSONL calls)
5. Verify compilation (`go vet`)

---

## Impact

- **C1 fix:** Crash recovery now reads all JSONL entries, not just 1
- **S1 fix:** Aggregate denominator is correct (178, not 166)
- **N1 fix:** Clear message when JSONL file doesn't exist

~15 lines changed across 4 locations.
