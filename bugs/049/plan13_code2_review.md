# Plan 13 Code — Per-Type Tests Review

**Reviewed by:** Senior Go Architect
**Date:** 2026-07-28
**File:** `server/router/api/v1/agent/benchmark_longmemeval_test.go` (lines 694-893)

---

## Verdict

| Severity | Count | Key |
|----------|-------|-----|
| Critical | 0 | |
| Significant | 1 | S1: `os.MkdirAll` missing in per-type tests |
| Minor | 4 | N1-N4 |

Recommended fix before running: **S1** (~2 lines). Remaining are cosmetic/hardening.

---

## Significant

### S1: Per-type tests fail if `build/benchmark/` doesn't exist

**Location:** `runPerTypeBenchmark` lines 765-807, `TestBenchmarkAbstention` lines 821-861

`appendJSONL` uses `os.OpenFile(..., O_CREATE, ...)`. This creates the **file** but not the parent directory. `clearBenchmarkJSONLs` returns silently on missing directory (line 401-403). Only `writeDryRunReport` calls `os.MkdirAll` (line 495).

If `build/benchmark/` doesn't exist (fresh checkout, cleaned directory, dry run never run), the per-type tests fail on the first `appendJSONL` call with a misleading "no such file or directory" error wrapped as "open JSONL: ..." — and the test continues silently (WARN logged, but the JSONL is never written).

**Fix:** Add `os.MkdirAll("build/benchmark", 0755)` at the top of `runPerTypeBenchmark` and `TestBenchmarkAbstention`, before any JSONL operations:

```go
// In runPerTypeBenchmark (line 765), after t.Helper():
require.NoError(t, os.MkdirAll("build/benchmark", 0755), "failed to create benchmark directory")

// In TestBenchmarkAbstention (line 821), after gate checks:
require.NoError(t, os.MkdirAll("build/benchmark", 0755), "failed to create benchmark directory")
```

---

## Minor

### N1: `BENCHMARK_FRESH` not logged in per-type tests

**Lines 774-776**

```go
if os.Getenv("BENCHMARK_FRESH") == "true" {
    clearBenchmarkJSONLs()
}
```

Silently clears. Dry run logs it (line 553):
```go
if os.Getenv("BENCHMARK_FRESH") == "true" {
    clearBenchmarkJSONLs()
    t.Log("Cleared existing JSONL files (BENCHMARK_FRESH=true)")
}
```

User running a per-type test may not realize existing results were wiped.

---

### N2: `BenchResult` lacks `ObservationLog`

**Lines 308-318** — `BenchResult` has no `ObservationLog` field. Per-type JSONL files can't be used to inspect what the observer produced for a failed question. `DryRunResult` (line 89) has the field and includes it in reports.

Add `ObservationLog string `json:"observation_log,omitempty"`` to `BenchResult`. Then `runBenchmarkQuestion` populates it on the error path. Makes debugging from JSONL possible.

---

### N3: `clearBenchmarkJSONLs` deletes ALL types' JSONLs

**Lines 399-410** — Deletes every `.jsonl` file in `build/benchmark/`. Running `TestBenchmarkSingleHop` with `BENCHMARK_FRESH=true` also deletes `knowledge_update_*.jsonl` and `abstention_*.jsonl`. Should scope to a glob matching the current type, or document that `BENCHMARK_FRESH` is a full-reset switch.

---

### N4: `TestBenchmarkAbstention` duplicates per-type test logic

**Lines 821-861** — 41 lines nearly identical to `runPerTypeBenchmark`. The only difference is the filter function (`filterAbstention` vs `filterByType`).

Refactor `runPerTypeBenchmark` to accept `func([]BenchmarkQuestion) []BenchmarkQuestion` instead of the current `(questions, qType, includeAbs)` pattern:

```go
func runPerTypeBenchmark(t *testing.T, qType, sessionPrefix string, filter func([]BenchmarkQuestion) []BenchmarkQuestion) {
    ...
    typeQuestions := filter(questions)
    ...
}
```

Then:
```go
func TestBenchmarkSingleHop(t *testing.T) {
    runPerTypeBenchmark(t, "single_hop", "sh", func(qs []BenchmarkQuestion) []BenchmarkQuestion {
        return filterByType(qs, "single_hop", false)
    })
}
func TestBenchmarkPreference(t *testing.T) { ... }
func TestBenchmarkKnowledgeUpdate(t *testing.T) { ... }
func TestBenchmarkAbstention(t *testing.T) {
    runPerTypeBenchmark(t, "abstention", "abs", filterAbstention)
}
```

All 5 tests become one-liners. Eliminates the 41-line duplication.

---

## Correctness Verified

| Component | Lines | Status |
|-----------|-------|--------|
| `runBenchmarkQuestion` pipeline | 696-763 | ✅ Observer → answer → judge → error handling |
| `runPerTypeBenchmark` gate + CR | 765-807 | ✅ Gate, JSONL recovery, skip-completed, append with error logging |
| `TestBenchmarkSingleHop` | 809-811 | ✅ `filterByType(..., false)` |
| `TestBenchmarkPreference` | 813-815 | ✅ Same pattern |
| `TestBenchmarkKnowledgeUpdate` | 817-819 | ✅ Same pattern |
| `TestBenchmarkAbstention` | 821-861 | ✅ `filterAbstention`, correct session prefix |
| `TestBenchmarkAggregate` | 863-893 | ✅ Unique total = `len(allResults)`, no dedup bug, empty case handled |
| `appendJSONL` error surfaced | 799-801, 853-855 | ✅ WARN logged on write failure |
| JSONL naming consistency | All | ✅ All use `benchmarkJSONLPath` → matching filenames |

---

**Proceed after fixing S1.** N1-N4 are cosmetic — the per-type tests are functionally correct and ready to run once the directory precondition is handled.
