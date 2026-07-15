# Plan: memstate Integration — Iteration 4 (FINAL)

## Source documents
- `plan3.md` — base revision
- `plan3_review_deepseek.md` — READY FOR IMPLEMENTATION (2 high findings)
- `plan3_review_hy3.md` — APPROVE WITH REQUIRED FIXES (2 high, 4 medium)

## Verdict
Both reviewers say architecture is sound. Remaining work is making the extraction faithful and proving supersession actually fires with real IDF-based scoring.

## What's new from plan3 → plan4

---

### HIGH #1: `extractLatestField` discards safeguards (both reviewers)

The naive generic scanner returns `m[1]` for all patterns. Wrong for phone (area code only) and address (street only). Missing: user-role filter, name false-positive filter, phone tenant exclusion, address length guard.

**Fix:** Hoist patterns to package-level (single source of truth). Write three per-field latest extractors that mirror `extractCollectedInfo`'s exact safeguards:

```go
// Package-level patterns — shared with extractCollectedInfo
var (
    latestNamePatterns = []*regexp.Regexp{ /* same as extractCollectedInfo lines 3653-3658 */ }
    latestPhonePattern = regexp.MustCompile(`...`)     // same as line 3661
    latestPhoneCorrectionPatterns = []*regexp.Regexp{...} // same as lines 3666-3671
    latestAddressPattern = regexp.MustCompile(`...`)   // same as line 3677
)

// Scan messages newest-first, user-role only, with per-field safeguards
func extractLatestName(messages []store.AgentMessage) string {
    for i := len(messages) - 1; i >= 0; i-- {
        if messages[i].Role != "user" { continue }
        for _, re := range latestNamePatterns {
            if m := re.FindStringSubmatch(messages[i].Content); len(m) > 1 {
                name := strings.TrimSpace(m[1])
                if !isCommonWord(name) && len(name) > 2 {
                    return name
                }
            }
        }
    }
    return ""
}

func extractLatestPhone(messages []store.AgentMessage, tenantPhone string) string {
    tenantNorm := normalizePhoneDigits(tenantPhone)
    // Check correction patterns first (newest-first)
    for i := len(messages) - 1; i >= 0; i-- {
        if messages[i].Role != "user" { continue }
        for _, re := range latestPhoneCorrectionPatterns {
            if m := re.FindStringSubmatch(messages[i].Content); len(m) > 1 {
                num := normalizePhoneDigits(m[1])
                if num != "" && !isPlaceholderPhoneDigits(num) && num != tenantNorm {
                    return m[1]
                }
            }
        }
    }
    // Fall back to main pattern (newest-first)
    for i := len(messages) - 1; i >= 0; i-- {
        if messages[i].Role != "user" { continue }
        if m := latestPhonePattern.FindStringSubmatch(messages[i].Content); len(m) > 0 {
            full := m[0] // full match, not m[1] (which is area code only)
            num := normalizePhoneDigits(full)
            if num != "" && !isPlaceholderPhoneDigits(num) && num != tenantNorm {
                return full
            }
        }
    }
    return ""
}

func extractLatestAddress(messages []store.AgentMessage) string {
    for i := len(messages) - 1; i >= 0; i-- {
        if messages[i].Role != "user" { continue }
        if m := latestAddressPattern.FindStringSubmatch(messages[i].Content); len(m) > 0 {
            addr := strings.TrimSpace(m[0]) // full match, not m[1]
            if len(addr) > 10 {
                return addr
            }
        }
    }
    return ""
}
```

Call sites in `processChat`:
```go
if isMemstateEnabled() && session.Facts != nil {
    if name := extractLatestName(session.Messages); name != "" {
        session.Facts.Add("Customer name is " + name)
    }
    if phone := extractLatestPhone(session.Messages, tenant.Phone); phone != "" {
        session.Facts.Add("Customer phone is " + phone)
    }
    if addr := extractLatestAddress(session.Messages); addr != "" {
        session.Facts.Add("Customer location is " + addr)
    }
}
```

**Note:** No duplicate-fact guard. Memstate deduplicates via supersession — adding the same fact twice is idempotent. This also resolves the contradiction in plan3 (prose said "remove" but code kept the guard).

---

### HIGH #2: Supersession test table uses wrong scoring model (hy3 #H2)

The test table predicted outcomes from raw token counts. memstate scores with IDF-weighted overlap using `min(denL, denR)` denominator. The actual score depends on the number of facts in memory and their token distributions.

**Fix:** Make `NewSafeMemory` accept `memstate.Config` for threshold tuning. Convert tests into measured acceptance gates:

```go
func NewSafeMemory(cfg ...memstate.Config) *SafeMemory {
    return &SafeMemory{mem: memstate.New(cfg...)}
}
```

**Acceptance tests (must pass before `MEMSTATE_ENABLED=true`):**

```go
func TestSupersessionAcceptance(t *testing.T) {
    mem := NewSafeMemory() // default config
    mem.Add("Customer name is John")
    mem.Add("Customer name is Jonathan")
    facts := mem.Facts(false)
    // Only "Customer name is Jonathan" should be current
    // (print actual scores for debugging)

    mem2 := NewSafeMemory()
    mem2.Add("Customer location is Rome")
    mem2.Add("Customer location is Milan")
    facts2 := mem2.Facts(false)
    // Only "Customer location is Milan" should be current

    mem3 := NewSafeMemory()
    mem3.Add("Customer name is John Smith")
    mem3.Add("Customer location is Rome")
    facts3 := mem3.Facts(false)
    // Both should be current (cross-topic, should NOT supersede)

    mem4 := NewSafeMemory()
    mem4.Add("Customer name is John")
    mem4.Add("I need help with billing")
    facts4 := mem4.Facts(false)
    // Both should be current (different topics)
}
```

If default 0.55 doesn't work for John→Jonathan, tune threshold down (e.g., 0.45) until it does. Document the chosen threshold in the config.

---

### MEDIUM #1: Step 4 references wrong file (DeepSeek #2, hy3 #M1)

Step 4 should reference `server/.../agent/service.go` (MemorySessionStore.GetOrCreate, line 1158), not `store/bridge.go`.

---

### MEDIUM #2: Contradiction on duplicate-fact guard (hy3 #M2)

Resolved: the guard is removed entirely. Prose and code both say "no guard" — rely on memstate idempotency.

---

### MEDIUM #3: `msgs` type mismatch (hy3 #M3)

Resolved by HIGH #1 fix: extractors accept `[]store.AgentMessage` directly.

---

### MEDIUM #4: Pattern duplication will drift (hy3 #M4)

Resolved by HIGH #1 fix: patterns hoisted to package level, shared by both functions.

---

## Implementation steps (revised)

| Step | Files | Changes |
|------|-------|---------|
| 1 | `go.mod` | Add memstate dependency, pin pseudo-version |
| 2 | `store/agent.go` | Add `Facts *SafeMemory` field, `json:"-"` tag |
| 3 | `store/safe_memory.go` | **New file** — `SafeMemory` type with mutex + recover in each method. `NewSafeMemory(cfg ...memstate.Config)`. NO `Facts()` method |
| 4 | `server/.../agent/service.go` (MemorySessionStore.GetOrCreate, line 1177-1188) | Initialize `SafeMemory` when enabled: `session.Facts = store.NewSafeMemory()` |
| 5 | `server/.../agent/service.go` | Hoist patterns to package level. Add `extractLatestName`, `extractLatestPhone`, `extractLatestAddress` functions. Add `var isMemstateEnabled` (package-level, overridable). Add memstate block in `processChat` (after `extractCollectedInfo`, ~15 lines) |
| 6 | `server/.../agent/service.go` | Inject prompt block in `buildSystemPrompt` (Section 0.5a, after line 2664, before Section 0.5 at line 2666) |
| 7 | Tests | 4 supersession acceptance tests (print actual scores) + Facts-nil-by-default test + integration test |
