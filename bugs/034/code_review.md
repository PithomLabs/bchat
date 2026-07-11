# Code Review — Bug 034 Implementation

## CRITICAL: None

---

## WARNING

### W1: `truncateToTokenBudget` still uses `len*4` heuristic

**Location:** `server/router/api/v1/agent/service.go:2747`

```go
estimatedBytes := maxTokens * 4
```

This function estimates byte count as `maxTokens * 4` for UTF-8 slicing. With the real tokenizer, CJK content at ~1 char/token means `maxTokens * 4` is 4x too large. The function overshoots the byte slice, then the fallback at line 2756 checks `EstimateTokens(candidate) > maxTokens` and trims 100 chars. But 100 chars may not be enough for CJK (where 100 chars = 100 tokens). Result: truncation could return text that's still over budget, or truncates too aggressively for English.

**Severity:** Low — only affects truncation, not chunking. Text gets "..." suffix either way.

**Fix (optional, not blocking):** Replace `estimatedBytes := maxTokens * 4` with a loop that uses `EstimateTokens` to find the right byte boundary. Or leave as-is since the fallback exists.

---

### W2: `addChunkOverlap` still uses `overlapTokens * 4` heuristic

**Location:** `server/router/api/v1/agent/chunker.go:832`

```go
overlapChars := overlapTokens * 4 // Token approximation (4 chars/token)
```

For CJK content at ~1 char/token, 50 overlap tokens = 50 chars, but this takes 200 chars ≈ 200 tokens. Overlap is 4x larger than intended for CJK. Not a correctness issue — the guard runs before overlap and the 8192 limit is far above 512+200. But retrieval quality degrades for CJK-heavy content because overlap context is bloated.

**Severity:** Low — affects retrieval quality, not correctness.

**Fix (optional, not blocking):** Use real tokenizer: `overlapChars := estimateCharsForTokens(overlapTokens)` where the function uses `Encode` to find the right char boundary.

---

### W3: `InitTokenizer` has latent race condition

**Location:** `server/router/api/v1/agent/embedding.go:32-56`

```go
func InitTokenizer(provider, model string) {
    if globalTokenizer != nil {  // <- unsynchronized read
        return
    }
    // ...
    globalTokenizer = enc        // <- unsynchronized write
}
```

No mutex. Two goroutines calling `InitTokenizer` simultaneously could both pass the nil check and both write. Not a practical concern today — called once from `NewVectorDB` at startup, tests call serially. But if someone later calls it from a goroutine, it's a data race.

**Severity:** Low — not triggered in current usage.

**Fix (optional, not blocking):** Add `sync.Once`:
```go
var tokenizerOnce sync.Once

func InitTokenizer(provider, model string) {
    tokenizerOnce.Do(func() { /* ... */ })
}
```

---

## INFO

### I1: `splitByHardLimit` binary search is correct

Traced the binary search with concrete examples. No off-by-one errors. The `lo-1` split index correctly produces the largest prefix that fits within `maxTokens`. Single-rune edge case is handled.

### I2: All test expectations match real tokenizer

Verified against live cl100k_base:
- `""` → 0, `"test"` → 1, `"test test"` → 2, `"hell"` → 1, `"hello"` → 1
- `"This is a longer text..."` → 13
- `"hello世界"` → 4 (1 for "hello" + 3 for "世界")

### I3: All call sites correctly migrated

14+ `EstimateTokens` call sites across 6 files (chunker.go, observer.go, observer_buffer.go, fusion_engine.go, service.go, processor.go, handlers.go). No local copies of the old `estimateTokens` remain. `observer.go:428` old function deleted.

### I4: Guard placement is correct

Guard runs at chunker.go:450-473, between garbage filter (line 445) and `addChunkOverlap` (line 476). Catches oversized chunks before overlap inflation. Correct.

### I5: Tests pass, go vet clean

```
=== RUN   TestChunkerNoTerminatorParagraph   --- PASS
=== RUN   TestChunkerNoH2Headers             --- PASS
=== RUN   TestChunkerOverlapSafe             --- PASS
=== RUN   TestChunkerGuardCatchesOversized   --- PASS
=== RUN   TestEstimateTokens                 --- PASS (all 7 sub-tests)
ok      github.com/usememos/memos/server/router/api/v1/agent   0.299s
go vet: no issues
```

### I6: `truncateToTokenBudget` fallback is adequate

Line 2756: `if EstimateTokens(candidate) > maxTokens { candidate = candidate[:max(0, end-100)] }` — the 100-char trim is a coarse safety net. For most content, the initial `maxTokens * 4` estimate is close enough that the fallback rarely triggers. When it does, the 100-char trim is usually sufficient. Not worth fixing for this bug.

---

## VERDICT

**APPROVE** — No blocking issues. W1-W3 are optional improvements for a follow-up. The core fix (real tokenizer + guard + flush check) is correct and solves the 8192 token limit error.
