# Plan 13 Code: Fix Remaining Bugs Before Per-Type Tests

## Current State

Plan13's review fixes (C1, S1, N1) are correctly applied. Two pre-existing bugs remain:

| Bug | Location | Issue | Severity |
|-----|----------|-------|----------|
| S1 | `appendJSONL` (line 317) | Silently drops all write errors — crash recovery silently fails | Significant |
| S2 | `content()` (line 41) | Silently drops marshal/unmarshal errors — `AnswerString()` returns `"<nil>"` | Significant |

These must be fixed before implementing per-type tests (plan12 scope). Without S1, a disk write failure during a 176-question run loses all results without warning. Without S2, a data format issue produces `"<nil>"` expected answers that the judge compares against, causing false fails indistinguishable from OM bugs.

---

## Implementation Order

1. Fix S2 — `content()` validation (3 line changes)
2. Fix S1 — `appendJSONL` error propagation (5 line changes)
3. Verify compilation: `go vet ./server/router/api/v1/agent/`

---

## Instructions for Coding Agent

### Fix 1: S2 — `content()` validation (lines 41-45)

**Problem:** `content()` marshals `QuestionContent` (typed `interface{}`) back to JSON and unmarshals to `questionContent`. Both errors silently ignored. If the JSON shape is unexpected, `AnswerString()` returns `"<nil>"` via `Sprintf("%v", nil)`.

**Current code (lines 41-46):**
```go
func (q BenchmarkQuestion) content() questionContent {
	var c questionContent
	data, _ := json.Marshal(q.QuestionContent)
	_ = json.Unmarshal(data, &c)
	return c
}
```

**Required change:** After unmarshal, validate that `c.Question` is non-empty and `c.Answer` is non-nil. If either is invalid, log a warning with the question ID and return a zero value. Do NOT panic — the test should continue with a visible warning so the user can inspect the question file.

Add after `_ = json.Unmarshal(data, &c)`:
```go
if c.Question == "" {
    fmt.Fprintf(os.Stderr, "WARN: question %s: content unmarshal produced empty question (data: %s)\n", q.QuestionID, string(data))
}
```

This ensures data format issues are visible in test output but don't crash the run.

**Files changed:** `server/router/api/v1/agent/benchmark_longmemeval_test.go` — 2 lines added.

---

### Fix 2: S1 — `appendJSONL` error propagation (lines 317-326)

**Problem:** `appendJSONL` silently drops errors at three points: file open, JSON marshal, and file write. The per-type test loop (being added in plan12) calls `appendJSONL` after each question. If a write fails, the test continues silently, and crash recovery is broken.

**Current code (lines 317-326):**
```go
func appendJSONL(path string, result BenchResult) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := json.Marshal(result)
	data = append(data, '\n')
	f.Write(data)
}
```

**Required change:** Change the signature to return `error`. Check all error points. Keep the existing defer pattern but surface errors:

```go
func appendJSONL(path string, result BenchResult) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open JSONL: %w", err)
	}
	defer f.Close()
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write JSONL: %w", err)
	}
	return nil
}
```

**Files changed:** `server/router/api/v1/agent/benchmark_longmemeval_test.go` — function body rewritten.

**Call site update:** The dry run test at line 189 (plan12's per-type test loop) will call `appendJSONL` like this:

```go
if err := appendJSONL(jsonlPath, result); err != nil {
    t.Logf("WARN: failed to write JSONL: %v", err)
}
```

This keeps the test running on write failure (non-blocking) but surfaces the error visibly.

---

## Files Changed

| File | Change | Lines |
|------|--------|-------|
| `server/.../agent/benchmark_longmemeval_test.go` | Fix S2: add content validation after unmarshal | +2 |
| `server/.../agent/benchmark_longmemeval_test.go` | Fix S1: make `appendJSONL` return `error` | ~6 changed |

---

## Verification

```bash
go vet ./server/router/api/v1/agent/
```
