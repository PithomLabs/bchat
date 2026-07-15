# Adversarial Plan Review: memstate Integration — Iteration 3 (plan3.md)

**Reviewer:** DeepSeek (final adversarial)
**Plan:** `bugs/039/plan3.md`
**Date:** 2026-07-15

---

## Verdict: READY FOR IMPLEMENTATION

2 high findings remain — both are implementation-level corrections, not design flaws. Fix them during implementation and this plan is safe to code.

---

## What's Fixed (from plan1 + plan2 reviews)

| Issue | Resolution | Status |
|-------|------------|--------|
| #C1 Belief revision broken by first-match extractor | `extractLatestField` scans newest-first | ✅ |
| Import cycle (`store → agent → store`) | `SafeMemory` moved to `store` package | ✅ |
| #H1 Inconsistent panic coverage | `recover()` inside each `SafeMemory` method | ✅ |
| #M2 `SafeMemory.Facts()` dead + pointer-unsafe | Removed entirely | ✅ |
| #M1 `isMemstateEnabled()` not overridable in tests | Package-level var, overridable | ✅ |
| #M3 Supersession criteria missing | Concrete test table added | ✅ |
| All nits from both reviews | Addressed | ✅ |

---

## New Findings (introduced in plan3)

### High — fix before implementation

#### #1. `extractLatestField` returns wrong value for phone and address

**The problem:** The function always returns `m[1]` (first regex capture group), but capture group semantics differ by pattern type:

| Pattern | `m[0]` (full match) | `m[1]` (capture group 1) |
|---------|---------------------|--------------------------|
| Name    | `"I'm John"` | `"John"` ✅ |
| Phone   | `"(555) 123-4567"` | `"555"` (area code only) ❌ |
| Address | `"123 Main St, Springfield, IL 62701"` | `"123 Main St"` (street only) ❌ |

**Consequence:** Facts become `"Customer phone is 555"` and `"Customer location is 123 Main St"` — broken data. The core feature delivers incorrect facts to the LLM.

**Root cause:** The function was designed for name patterns where `m[1]` captures just the name. Phone and address patterns have different capture group layouts — phone has 3 groups (area, prefix, line) and address has 4 groups (street, city, state, zip). The full meaningful value is in `m[0]`.

**Fix:** Parameterize the submatch index. Also accept `[]store.AgentMessage` (not `[]string`) and filter by `role == "user"` to avoid extracting from assistant responses:

```go
func extractLatestField(messages []store.AgentMessage, patterns []*regexp.Regexp, submatchIdx int) string {
    for i := len(messages) - 1; i >= 0; i-- {
        if messages[i].Role != "user" {
            continue
        }
        for _, re := range patterns {
            if m := re.FindStringSubmatch(messages[i].Content); len(m) > submatchIdx {
                return strings.TrimSpace(m[submatchIdx])
            }
        }
    }
    return ""
}
```

Call sites in `processChat`:
```go
extractLatestField(session.Messages, namePatterns, 1)     // "John"
extractLatestField(session.Messages, phonePatterns, 0)    // "(555) 123-4567"
extractLatestField(session.Messages, addressPatterns, 0)  // "123 Main St, Springf...
```

---

#### #2. Step 4 references `store/bridge.go (GetOrCreate)` — wrong file

**The problem:** Step 4 of the implementation table says:

```
| 4 | `store/bridge.go` (GetOrCreate) | Initialize `SafeMemory` when enabled |
```

`MemorySessionStore.GetOrCreate()` lives at `server/router/api/v1/agent/service.go:1158` in the `agent` package. `store/bridge.go` exists but contains bridge routing types and methods — it has no `GetOrCreate` for sessions. An implementer following the plan literally will look in the wrong package.

**Fix:** Step 4 should read:

```
| 4 | `server/.../agent/service.go` (MemorySessionStore.GetOrCreate) | Initialize `SafeMemory` when enabled |
```

The code at the call site (since `SafeMemory` is now in `store`):
```go
// In MemorySessionStore.GetOrCreate(), after the session struct literal:
if isMemstateEnabled() {
    session.Facts = store.NewSafeMemory()
}
```

---

### Medium

#### #3. `extractLatestField` signature type mismatch (resolved by #1 fix)

The plan declares `func extractLatestField(messages []string, ...)` but `session.Messages` is `[]store.AgentMessage`. Won't compile as written. **Resolved by fix #1** above which changes the signature to `[]store.AgentMessage`.

---

### Low — accept for MVP

#### #4. Duplicate-fact guard doesn't prevent re-adds after first change

The guard `latestName != session.CustomerName` compares against `session.CustomerName`, which is set first-match only (line 2106-2107: `if session.CustomerName == ""`). After a name change from "John" to "Jonathan":
- `session.CustomerName` is still `"John"` (never updated)
- `extractLatestField` returns `"Jonathan"` every turn
- `"Jonathan" != "John"` is `true` every turn → re-adds the same fact on every subsequent turn

Memstate deduplicates via supersession, so this is functionally correct but wasteful: up to 49 redundant `Add()` calls per field in a 50-turn session. Acceptable for the MVP. Can be optimized later by updating `session.CustomerName` in the memstate block.

---

#### #5. No tenant-phone filtering in `extractLatestField`

`extractLatestField` for phone doesn't exclude the tenant's own phone number (`extractCollectedInfo` does this via `phoneNorm != tenantPhoneNorm`). If a customer types the company's phone, it could appear as a customer fact. Mitigate in a follow-up.

---

## Summary

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | `extractLatestField` wrong submatch for phone/address (needs `m[0]`, not `m[1]`) | **High** | Fix during implementation |
| 2 | Step 4 references wrong file (`store/bridge.go` → `agent/service.go`) | **High** | Fix during implementation |
| 3 | Signature `[]string` vs `[]store.AgentMessage` | Medium | Resolved by #1 fix |
| 4 | Re-adds same fact after change every turn | Low | Accept |
| 5 | No tenant-phone filter | Low | Accept |

### Bottom line

Correct #1 (submatch index parameter + role filtering) and #2 (file path in implementation table) during implementation. Everything else is good to ship. This plan is **ready for a coding agent**.
