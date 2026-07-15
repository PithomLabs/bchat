# Adversarial Plan Review: memstate Integration — Revised Plan (plan2.md)

**Reviewer:** DeepSeek (adversarial)
**Plan:** `bugs/039/plan2.md`
**Date:** 2026-07-15

---

## Verdict: APPROVED WITH NITS

The revised plan successfully addresses all 4 critical and 4 high issues from the original review. However, one **new critical** issue undermines the core feature, plus several new high/medium findings were introduced by the fixes.

---

## What's Fixed (from original review)

| Original Issue | Resolution | Status |
|----------------|------------|--------|
| #1 `MEMSTATE_ENABLED` defaults to `true` | D4: now defaults to `false` | ✅ Fixed |
| #2 Cross-session memory unreachable | D1: removed entirely | ✅ Fixed |
| #3 `userMemories` memory leak | D1: removed cross-session memory | ✅ Fixed |
| #4 Raw `userMessage` as facts | D2: only extracted structured facts | ✅ Fixed |
| #5 No panic recovery | D5: `safeFactsAdd` with `recover()` | ✅ Fixed |
| #6 Cross-session memory lost on restart | D1: removed from scope | ✅ Fixed |
| #7 OMConfig pollution | D5: standalone `isMemstateEnabled()` | ✅ Fixed |
| #8 Token savings claims speculative | Removed from plan | ✅ Fixed |
| #9 "Replace" vs "supplement" | Now says "supplements" | ✅ Fixed |
| #10 External-only undocumented | Limitations section added | ✅ Fixed |

---

## New Critical Finding

### #C1. Belief revision doesn't work — `extractCollectedInfo` returns the first match only

**Severity:** Critical

**Failure scenario:** Step 5 feeds `customerInfo.Name` from `extractCollectedInfo()` into memstate. But `extractCollectedInfo` uses first-match-wins for name (`service.go:3694`: `if info.Name == ""`). After a multi-turn conversation:

1. Turn 1: customer says *"I'm John"*
   - `customerInfo.Name` = `"John"` (extracted from first message)
   - `session.Facts.Add("Customer name is John")` ✓

2. Turn 2: customer says *"Call me Jonathan"*
   - `customerInfo.Name` = `"John"` still — `extractCollectedInfo` iterates all messages and stops at the *first* match ("I'm John")
   - `session.Facts.Add("Customer name is John")` again — identical fact
   - **memstate never sees "Jonathan"** — no supersession occurs

3. Result: On every turn, memstate receives the same static fact. The core promise — *"when a customer changes their mind, the old fact is automatically superseded"* — cannot materialize.

Note: Phone *does* have a correction override path (lines 3707-3718), so `customerInfo.Phone` can reflect changes. But name and address do not.

**Root cause:** The plan assumes `customerInfo` reflects the *latest* stated value. It actually reflects the *first* match. These are different when a customer revises information.

**Fix:** Don't feed `customerInfo` directly. Compare against previously known values and only add when a change is detected. The cleanest approach:

```go
// After extractCollectedInfo and the existing session field updates:
if isMemstateEnabled() && session.Facts != nil {
    // Name: check if the latest extraction differs from what we tracked
    latestName := extractLatestValue(session.Messages, namePatterns)
    if latestName != "" && latestName != session.CustomerName {
        safeFactsAdd(session.Facts, "Customer name is "+latestName)
    }
    // Phone: customerInfo.Phone already supports corrections via phoneCorrectionPatterns
    if customerInfo.Phone != "" && customerInfo.Phone != session.CustomerPhone {
        safeFactsAdd(session.Facts, "Customer phone is "+customerInfo.Phone)
    }
    // Address: same first-match limitation as name
    if customerInfo.Address != "" && customerInfo.Address != session.CustomerLocation {
        safeFactsAdd(session.Facts, "Customer location is "+customerInfo.Address)
    }
}
```

This requires:
- A new `extractLatestValue()` function (or modify `extractCollectedInfo` to track last index per field)
- Or simply: process the messages *in reverse* to get the latest match
- Or: feed memstate from the *most recent user message only*, concatenated with previous known values when they change

Without this fix, the feature delivers no value — memstate sees duplicate identical facts on every turn.

---

## New High Findings

### #H1. Inconsistent panic coverage across SafeMemory methods

**Severity:** High

**Failure scenario:** `safeFactsAdd` wraps `mem.Add()` with `recover()`, but `session.Facts.Prompt("", 500)` (Step 6) and `session.Facts.Facts()` are called directly without any recover wrapper. If memstate panics inside `Prompt()` (e.g., due to a token counting bug or nil dereference), the handler crashes with no recovery.

**Fix:** Move `recover()` into each `SafeMemory` method internally:

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
```

Same pattern for `Prompt()` and `Facts()`. Then the standalone `safeFactsAdd` function is unnecessary — callers use `session.Facts.Add()` directly and get protection everywhere.

---

### #H2. `SafeMemory.Facts()` returns pointers — mutex scope too narrow

**Severity:** High (depends on memstate internals)

**Failure scenario:** `Facts()` locks, calls `s.mem.Facts(includePrivate)` which returns `[]*memstate.Fact`, then unlocks. If `memstate.Fact` fields (e.g., `.Text`, `.Current()`) reference memstate's internal hash map state via pointers, the caller reads data outside the mutex. A concurrent `Add()` could invalidate those pointers or modify the data they reference.

**Fix:** Either:
- (a) Deep-copy each fact into value types inside the lock scope
- (b) Document that the returned facts are safe to read after unlock (requires reading memstate source to verify)

Note: This method is currently unused in the plan (see #M2). If it's not needed, remove it rather than fix it.

---

## New Medium Findings

### #M1. `isMemstateEnabled()` cannot be overridden in tests

**Severity:** Medium

`os.Getenv("MEMSTATE_ENABLED")` is called directly. Tests must use `t.Setenv()` which is global state — it pollutes parallel subtests and requires prior knowledge of the env var.

**Fix:** Make it a package-level variable:

```go
var isMemstateEnabled = func() bool {
    return os.Getenv("MEMSTATE_ENABLED") == "true"
}
```

Tests can inject:
```go
func TestMemstateIntegration(t *testing.T) {
    orig := isMemstateEnabled
    isMemstateEnabled = func() bool { return true }
    defer func() { isMemstateEnabled = orig }()
    // ...
}
```

---

### #M2. `SafeMemory.Facts()` is dead code

**Severity:** Medium

The `Facts()` method is defined in Step 3 (new `safe_memory.go`) but never called in any implementation step. If it's unused, remove it to minimize API surface and avoid needing to audit its pointer-safety (see #H2).

---

### #M3. Supersession threshold unvalidated — no concrete test criteria

**Severity:** Medium

Limitation #3 acknowledges the default threshold (0.55, IDF overlap) is unvalidated. The Testing Strategy mentions "Supersession test" but provides no expected similarity scores or pass/fail criteria.

**Fix:** Add explicit test expectations:

| Test | Expected | Rationale |
|------|----------|-----------|
| "Customer name is John" → "Customer name is Jonathan" | Supersede (overlap > 0.55) | 3/4 tokens shared |
| "Customer name is John Smith" → "Customer location is Rome" | NOT supersede (overlap < 0.55) | 1/5 tokens shared |
| "Customer phone is 555-1234" → "Customer phone is 555-5678" | Supersede (overlap > 0.55) | 3/5 tokens shared |
| "Customer name is John" → "I need help with billing" | NOT supersede (overlap < 0.55) | 1/8 tokens shared |

---

## Nits

### #N1. "Section 0.5a" placement is ambiguous

The plan says "BEFORE the existing OM injection" (Step 6). The actual code at line 2666 reads `SECTION 0.5: OBSERVATIONAL MEMORY`. The new section goes between Section 0 (ending at line 2663) and Section 0.5 (starting at line 2666). Be explicit: "insert after line 2664 (`sb.WriteString(contactInstruction.Section0Addition)`)".

### #N2. Vendoring audit implications (D6)

D6 recommends vendoring for production. A vendored library (~500 lines, 3 commits, 0 releases) won't receive updates via `go mod tidy`. Document a quarterly review cadence to check for upstream changes.

### #N3. Existing test coverage gap

The `MemorySessionStore.GetOrCreate()` tests (in `bridge_foundation_test.go`) run with default config. With `MEMSTATE_ENABLED=false` (now the default), `session.Facts` will be nil. Add a test that verifies `session.Facts == nil` by default and `session.Facts != nil` when enabled — this guards against regressions if someone changes the default back to `true`.

---

## Summary

| ID | Finding | Severity | Status |
|----|---------|----------|--------|
| #C1 | Belief revision broken by `extractCollectedInfo` first-match | **Critical** | New |
| #H1 | Inconsistent panic coverage across SafeMemory methods | High | New |
| #H2 | `SafeMemory.Facts()` pointer safety outside mutex | High | New |
| #M1 | `isMemstateEnabled()` not overridable in tests | Medium | New |
| #M2 | `SafeMemory.Facts()` is dead code | Medium | New |
| #M3 | Supersession threshold criteria missing | Medium | New |
| #N1-N3 | Various nits | Low | New |

### Recommendation

Fix **#C1** and **#H1** before merging. Without #C1, the feature provides zero value — memstate receives identical static facts on every turn and never performs belief revision. #H1 leaves half the API surface unprotected from panics.

Items #H2, #M1, #M2, #M3, and #N1-N3 can be addressed as tech debt after the initial merge, but the `Facts()` dead code (#M2) should be removed before merging to avoid shipping an un-audited API surface.
