# Plan: memstate Integration — Iteration 3

## Source documents
- `plan2.md` — base revision
- `plan2_review_deepseek.md` — approved with nits, 1 critical
- `plan2_review_hy3.md` — rework, 2 criticals

## What's new from plan2 → plan3

### CRITICAL FIX: Belief revision broken by first-match extractor (both reviewers #CR2/#C1)

`extractCollectedInfo` returns the **first** match for name/address across entire history. "I'm John" → "Call me Jonathan" → `customerInfo.Name` stays `"John"`. Memstate never sees "Jonathan"; supersession never fires.

**Fix:** Add a `extractLatestField()` helper that scans messages newest-first and returns the last match per field. Feed memstate from this, not from `extractCollectedInfo` output.

```go
func extractLatestField(messages []string, patterns []*regexp.Regexp) string {
    for i := len(messages) - 1; i >= 0; i-- {
        for _, re := range patterns {
            if m := re.FindStringSubmatch(messages[i]); len(m) > 1 {
                return strings.TrimSpace(m[1])
            }
        }
    }
    return ""
}
```

Call sites in `processChat`:
```go
if isMemstateEnabled() && session.Facts != nil {
    if latestName := extractLatestField(msgs, namePatterns); latestName != "" && latestName != session.CustomerName {
        session.Facts.Add("Customer name is " + latestName)
    }
    if latestPhone := extractLatestField(msgs, phonePatterns); latestPhone != "" && latestPhone != session.CustomerPhone {
        session.Facts.Add("Customer phone is " + latestPhone)
    }
    if latestAddr := extractLatestField(msgs, addressPatterns); latestAddr != "" && latestAddr != session.CustomerLocation {
        session.Facts.Add("Customer location is " + latestAddr)
    }
}
```

Requires: define `namePatterns`, `phonePatterns`, `addressPatterns` as package-level `[]*regexp.Regexp` constants (duplicated from `extractCollectedInfo` but only used here — acceptable since both reviewers say this is pre-merge critical).

**Also:** Remove the duplicate-fact guard ("only add if changed") — memstate itself deduplicates via supersession. Adding the same fact twice is idempotent.

### CRITICAL FIX: Import cycle (hy3 #CR1)

`SafeMemory` in `agent` package → referenced from `store.AgentSession` → `store` already imports `agent`. Cycle: `store → agent → store`.

**Fix:** Move `SafeMemory` into the `store` package. It only needs `sync` + `memstate`. Field type becomes `*store.SafeMemory`.

### HIGH FIX: Panic protection consistent (DeepSeek #H1, hy3 #M1)

Move `recover()` into each `SafeMemory` method internally. Remove the standalone `safeFactsAdd` function — callers use `session.Facts.Add()` directly.

```go
func (s *SafeMemory) Add(text string) {
    if s == nil || s.mem == nil { return }
    defer func() {
        if r := recover(); r != nil {
            slog.Error("memstate panicked on Add", "panic", r, "text", text)
        }
    }()
    s.mu.Lock()
    defer s.mu.Unlock()
    s.mem.Add(text)
}

func (s *SafeMemory) Prompt(query string, limit int) string {
    if s == nil || s.mem == nil { return "" }
    defer func() {
        if r := recover(); r != nil {
            slog.Error("memstate panicked on Prompt", "panic", r)
        }
    }()
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.mem.Prompt(query, limit)
}
```

### MEDIUM: Remove `SafeMemory.Facts()` (DeepSeek #M2, #H2)

Dead code, never called. Pointer safety concerns outside mutex scope. Remove entirely.

### MEDIUM: Testable `isMemstateEnabled()` (DeepSeek #M1, hy3 #L2)

Package-level variable, overridable in tests:

```go
var isMemstateEnabled = func() bool {
    return os.Getenv("MEMSTATE_ENABLED") == "true"
}
```

### MEDIUM: Remove D3 `session.Messages` claim (hy3 #M2)

The wrapper only guards `Facts`. `session.Messages` race is pre-existing and out of scope. Correct the claim.

### MEDIUM: Concrete supersession test criteria (DeepSeek #M3)

| Test | Expected | Rationale |
|------|----------|-----------|
| "Customer name is John" → "Customer name is Jonathan" | Supersede (overlap > 0.55) | 3/4 tokens shared |
| "Customer name is John Smith" → "Customer location is Rome" | NOT supersede (overlap < 0.55) | 1/5 tokens shared |
| "Customer phone is 555-1234" → "Customer phone is 555-5678" | Supersede (overlap > 0.55) | 3/5 tokens shared |
| "Customer name is John" → "I need help with billing" | NOT supersede (overlap < 0.55) | 1/8 tokens shared |

### NITS

- **DeepSeek #N1:** Be explicit about Section 0.5a insertion point (after line 2664, before Section 0.5).
- **DeepSeek #N3:** Add test verifying `session.Facts == nil` by default and `!= nil` when enabled.
- **hy3 #L1:** `safeFactsAdd` removed (folded into wrapper).

---

## Implementation steps (revised)

| Step | Files | Changes |
|------|-------|---------|
| 1 | `go.mod` | Add memstate dependency, pin pseudo-version |
| 2 | `store/agent.go` | Add `Facts *SafeMemory` field, `json:"-"` tag |
| 3 | `store/safe_memory.go` | **New file** — `SafeMemory` type with mutex + recover in each method. NO `Facts()` method |
| 4 | `store/bridge.go` (GetOrCreate) | Initialize `SafeMemory` when enabled |
| 5 | `server/.../agent/service.go` | Add `extractLatestField()`, `namePatterns`/`phonePatterns`/`addressPatterns` as package-level vars. Add 9-line memstate block in processChat (after extractCollectedInfo, feeds from `extractLatestField`, not `customerInfo`) |
| 6 | `server/.../agent/service.go` | Inject prompt block in `buildSystemPrompt` (Section 0.5a) |
| 7 | `server/.../agent/service.go` | Add `var isMemstateEnabled` (package-level, overridable) |
| 8 | Tests | 4 supersession unit tests + 2 integration tests + Facts-nil-by-default test |
