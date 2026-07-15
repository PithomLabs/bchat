# Implementation: memstate Integration into bchat

## Overview
Added deterministic belief revision to bchat's external chat agent using the `memstate` library (Go port of mnem). When `MEMSTATE_ENABLED=true`, customer facts (name, phone, address) are tracked in-memory per session. When a customer changes their mind (e.g., "My name is Jonathan" after previously saying "John"), the old fact is automatically superseded and only the current truth is injected into the LLM prompt.

## Files changed

### New files
| File | Purpose |
|------|---------|
| `store/safe_memory.go` | Thread-safe wrapper around `memstate.Memory` with mutex + panic recovery |
| `server/.../agent/memstate_test.go` | 11 tests: 4 supersession acceptance, 2 nil/init, 5 extraction |

### Modified files
| File | Change |
|------|--------|
| `go.mod` | Added `github.com/PithomLabs/memstate` dependency |
| `store/agent.go:268` | Added `Facts *SafeMemory` field to `AgentSession` |
| `server/.../agent/service.go:40` | Added `var isMemstateEnabled` (package-level, testable) |
| `server/.../agent/service.go:1190-1192` | `GetOrCreate` initializes `SafeMemory` when enabled |
| `server/.../agent/service.go:3862-3960` | Added `extractLatestName`, `extractLatestPhone`, `extractLatestAddress` functions + package-level patterns |
| `server/.../agent/service.go:2125-2135` | `processChat` feeds latest extracted facts into memstate |
| `server/.../agent/service.go:2689-2700` | `buildSystemPrompt` injects Section 0.5a (fact prompt) |

## Architecture

```
processChat (per turn)
  │
  ├─ extractCollectedInfo()        ← existing, unchanged (first-match, sets session.CustomerName etc.)
  │
  ├─ extractLatest*()              ← NEW: newest-first, user-role only, with safeguards
  │   ├─ extractLatestName()       ← explicit-marker patterns only (no standalone #3)
  │   ├─ extractLatestPhone()      ← correction patterns first, then main, tenant excluded
  │   └─ extractLatestAddress()    ← match[0], len>10 guard
  │
  └─ session.Facts.Add(...)        ← NEW: feeds memstate (supersession automatic)

buildSystemPrompt (per turn)
  │
  ├─ Section 0:    Contact info    ← existing
  ├─ Section 0.5a: Facts prompt    ← NEW: "FACTS EXTRACTED FROM CUSTOMER"
  └─ Section 0.5:  OM injection    ← existing
```

## Key design decisions

1. **`SafeMemory` in `store` package** — avoids import cycle (`store` → `agent` → `store`)
2. **`recover()` in each method** — not just `Add`; `Prompt` and `Facts` are also protected
3. **`Facts()` returns deep copy** — pointer safety after mutex release
4. **Standalone name pattern #3 excluded** — prevents "sounds good" from overwriting a correct name under newest-first extraction
5. **No duplicate-fact guard** — memstate deduplicates via supersession; adding same fact twice is idempotent
6. **Default-off** — `MEMSTATE_ENABLED` must be explicitly set to `"true"`
7. **Default threshold 0.55 works** — supersession acceptance tests pass without tuning

## Verification
- `go vet ./server/...` — clean
- `go test ./server/router/api/v1/agent/` — all tests pass (existing + 11 new)
- Supersession works with default 0.55 threshold for John→Jonathan, (555) 123-4567→(555) 567-8901
- Cross-topic pairs (name vs location, name vs billing) do NOT supersede

---

## Adversarial Code Review Prompt

You are a senior security and reliability engineer reviewing a code change that
adds in-memory belief revision (memstate) to an AI chat agent platform (bchat).

**CONTEXT:**
- memstate is a Go port of mnem, a deterministic state-memory library
- It tracks facts per chat session and automatically supersedes old facts when
  new information contradicts them (IDF-weighted token overlap, threshold 0.55)
- The feature is gated behind MEMSTATE_ENABLED env var (default: false)
- SafeMemory wraps memstate with mutex + panic recovery

**WHAT TO REVIEW:**
1. **Security:** Is the prompt injection surface acceptable? Could extracted facts
   be poisoned by adversarial user input?
2. **Correctness:** Do the extractLatest* functions faithfully mirror the safeguards
   in extractCollectedInfo? Are there edge cases where belief revision fires
   incorrectly or fails to fire?
3. **Concurrency:** Is the SafeMemory wrapper sufficient? Are there remaining race
   conditions on session.Messages or other session fields?
4. **Failure modes:** What happens if memstate panics? If MEMSTATE_ENABLED is set
   but the dependency is broken? If the same fact is added 50 times?
5. **Performance:** Is the per-turn O(n) extraction acceptable? Does the 50-turn
   session cap bound memory growth?
6. **Prompt quality:** Does Section 0.5a inject at the right position? Could the
   fact prompt conflict with Section 0 (contact info) or Section 0.5 (OM)?

**FILES TO READ:**
- `store/safe_memory.go` (new)
- `store/agent.go` (Facts field added)
- `server/.../agent/service.go` (extractLatest*, processChat memstate block,
  buildSystemPrompt Section 0.5a, isMemstateEnabled, GetOrCreate init)
- `server/.../agent/memstate_test.go` (new)

**OUTPUT FORMAT:**
Return findings as a table:

| ID | Severity | File:Line | Description | Fix |
|----|----------|-----------|-------------|-----|
| 1  | Critical/High/Medium/Low | path/to/file.go:123 | What's wrong | How to fix |
