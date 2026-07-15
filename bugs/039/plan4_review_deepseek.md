# Adversarial Plan Review: memstate Integration — Iteration 4 (FINAL)

**Reviewer:** DeepSeek (final adversarial)
**Plan:** `bugs/039/plan4.md`
**Date:** 2026-07-15

---

## Verdict: APPROVED — ready for implementation

All critical and high issues from three prior review rounds are resolved. Two medium implementation-level items remain — neither is a design flaw, both are trivially fixable during coding.

---

## What's Fixed

| Issue | Round | Status |
|-------|-------|--------|
| `MEMSTATE_ENABLED` defaults to true → false | plan1→plan2 | ✅ |
| Cross-session memory unreachable | plan1→plan2 | ✅ Removed |
| `userMemories` memory leak | plan1→plan2 | ✅ Removed |
| Raw `userMessage` as facts | plan1→plan2 | ✅ Only extracted facts |
| Panic recovery inconsistent | plan2→plan3 | ✅ recover() in each SafeMemory method |
| OMConfig pollution | plan1→plan2 | ✅ Standalone function |
| Token savings speculative | plan1→plan2 | ✅ Removed |
| Import cycle (store→agent→store) | plan2→plan3 | ✅ SafeMemory in store package |
| Belief revision broken by first-match extractor | plan2→plan3 | ✅ Three per-field extractors |
| extractLatestField wrong submatch for phone/address | plan3→plan4 | ✅ Per-field extractors use correct m[0]/m[1] |
| Wrong file reference for GetOrCreate | plan3→plan4 | ✅ agent/service.go |
| Type mismatch `[]string` vs `[]store.AgentMessage` | plan3→plan4 | ✅ Extractors accept `[]store.AgentMessage` |
| Supersession test with naive token math | plan3→plan4 | ✅ Measured acceptance gates |
| Extractors missing safeguards (common word, tenant phone, address length, role filter) | plan3→plan4 | ✅ All four per-field extractors mirror `extractCollectedInfo` |

---

## Remaining Issues

### Medium — fix during implementation

**#1. Tests call `mem.Facts(false)` but SafeMemory has no `Facts()` method**

The acceptance tests in HIGH #2 use `mem.Facts(false)` to assert supersession behavior:

```go
facts := mem.Facts(false)
```

But Step 3 explicitly says "NO `Facts()` method" (removed in plan3 as dead code and pointer-unsafe). This is a contradiction.

**Fix:** Either (a) re-add `Facts()` to SafeMemory with a deep copy inside the lock scope, or (b) use `mem.Prompt("", 0)` for verification instead. Option (a) is recommended — the method is essential for testing and a deep copy eliminates the pointer-safety concern:

```go
func (s *SafeMemory) Facts(includePrivate bool) []*memstate.Fact {
    if s == nil || s.mem == nil { return nil }
    defer func() { if r := recover(); r != nil { slog.Error(...) } }()
    s.mu.Lock()
    defer s.mu.Unlock()
    raw := s.mem.Facts(includePrivate)
    // Deep copy to return
    result := make([]*memstate.Fact, len(raw))
    for i, f := range raw {
        result[i] = &memstate.Fact{Text: f.Text, ...}
    }
    return result
}
```

---

**#2. `extractLatestPhone` call site references `tenant.Phone` which doesn't exist in `processChat` scope**

The plan's call site:
```go
if phone := extractLatestPhone(session.Messages, tenant.Phone); phone != "" {
```

`tenant.Phone` is not defined in `processChat`. The existing code at line 2104 already computes:

```go
validatedCompanyPhone := GetValidatedReplacementPhone(config.Audience.EmergencyPhone, config.RawKB)
```

**Fix:** Use `validatedCompanyPhone` instead:
```go
if phone := extractLatestPhone(session.Messages, validatedCompanyPhone); phone != "" {
```

---

### Low — accept for MVP

**#3. Pattern drift risk.** `latestNamePatterns` and the patterns inside `extractCollectedInfo` are separate variables with identical values. Add a comment on each:

```go
var latestNamePatterns = []*regexp.Regexp{
    // Must be kept in sync with patterns in extractCollectedInfo()
}
```

---

**#4. No duplicate guard → redundant `Add()` calls every turn after first mention.** `extractLatestName` returns the same name on every turn after it's first mentioned. For a 50-turn session where the name was stated once in turn 1, `session.Facts.Add("Customer name is John")` is called 49 redundant times. Memstate handles idempotency via supersession, so behavior is correct — but each call does O(n) IDF overlap checks. Acceptable at 50-turn cap.

---

## Summary

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | `Facts()` needed by tests but removed from SafeMemory | **Medium** | Re-add with deep copy during implementation |
| 2 | `tenant.Phone` doesn't exist → use `validatedCompanyPhone` | **Medium** | Fix call site during implementation |
| 3 | Pattern drift risk | Low | Add sync comments |
| 4 | Redundant Add calls after first mention | Low | Accept |

### Bottom line

Fix #1 (re-add Facts() with deep copy for tests) and #2 (use `validatedCompanyPhone` in call site) during implementation. Everything else is clean.

**This plan is ready for a coding agent. No more review iterations needed.**
