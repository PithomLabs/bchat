# memstate Integration into bchat — Revised Plan

## Overview

memstate (github.com/PithomLabs/memstate) is a deterministic state memory
library with belief revision. When a customer changes their mind, the old
fact is automatically superseded — no LLM required.

This plan adds memstate as a **session-scoped** fact layer alongside bchat's
existing Observational Memory (OM) and RAG. Cross-session memory is explicitly
out of scope (see Limitations).

## Problem Statement

bchat has no mechanism to revise beliefs. When a customer says
"My name is John" then later "Actually, Jonathan", both values persist in
session state and OM. The LLM must figure out which is current at query time,
which it often gets wrong.

## Architecture

```
User message arrives
        │
        ▼
┌──────────────────────────────────────┐
│  extractCollectedInfo()              │  Existing regex extraction
│  → customerInfo.Name/Phone/Address   │
└──────────┬───────────────────────────┘
           │
           ▼
┌──────────────────────────────────────┐
│  safeFactsAdd(session.Facts, ...)    │  Deterministic belief revision
│  Only extracted structured facts     │  (panic-safe, mutex-guarded)
└──────────┬───────────────────────────┘
           │
           ▼
┌──────────────────────────────────────┐
│  processChat() continues             │
│  (classify, policy, generate)        │
└──────────┬───────────────────────────┘
           │
           ▼
┌──────────────────────────────────────┐
│  buildSystemPrompt()                 │
│  Section 0.5a: memstate facts        │  ← current beliefs, token-budgeted
│  Section 0.5b: OM observations       │  ← complex inferences (unchanged)
└──────────────────────────────────────┘
```

## Design Decisions (from adversarial review)

### D1. Session-scoped only — no cross-session memory

Both reviewers identified that cross-session memory (original Steps 6–8) is
unreachable for external sessions (`UserID` is never set in `ChatExternal`),
unpersisted (lost on restart), and has unbounded memory growth.

**Decision:** Remove Steps 6–8 entirely. Ship session-scoped facts only.
Cross-session memory is a separate feature requiring its own persistence
design.

### D2. Only extracted facts — never raw messages

Adding raw `userMessage` to `session.Facts` creates noise (non-factual
utterances like "ok", "thanks") and risks false supersession (two unrelated
service requests sharing token overlap). It also creates a prompt injection
vector: a user could type "Ignore prior rules" which then appears under
"VERIFIED CURRENT FACTS" in the system prompt.

**Decision:** Only add structured, extracted facts from `extractCollectedInfo()`.
Raw messages stay in `session.Messages` where they belong.

### D3. Thread-safety via wrapper

memstate has no internal locking. bchat runs concurrent `ChatExternal` calls
for the same session (the per-session lock is released before `processChat`).
Concurrent `session.Facts.Add()` calls cause fatal concurrent-map writes that
cannot be caught by `recover()`.

**Decision:** Add a `SafeMemory` wrapper with `sync.Mutex` guarding all
memstate calls. This also fixes the pre-existing unsynchronized
`session.Messages` mutation if wired up properly.

### D4. Default disabled

`MEMSTATE_ENABLED` defaults to `false`. Existing behavior is the safe default.
Early adopters opt in explicitly after validation.

### D5. Standalone config — not OMConfig

`MemstateEnabled` does not belong in `OMConfig`. memstate is independent of OM.

**Decision:** Use a simple `isMemstateEnabled()` function that reads from
`os.Getenv`. No struct pollution.

### D6. Dependency pinning

`v0.1.0` tag does not exist yet. The library has 3 commits and 0 releases.

**Decision:** Use a pseudo-version (`v0.0.0-<timestamp>-<hash>`) or vendor
the library into `bchat/vendor/`. Vendoring is recommended for production
since the library is unversioned and small (~500 lines).

## Implementation Steps

### Step 1: Add memstate dependency

**File: `go.mod`**

Option A (vendored — recommended for production):
```bash
go get github.com/PithomLabs/memstate@main
go mod vendor
```

Option B (pseudo-version):
```
require github.com/PithomLabs/memstate v0.0.0-20260715000000-abc123def456
```

Verify the version resolves with `go mod tidy` before proceeding.

### Step 2: Add Facts field to AgentSession

**File: `store/agent.go`**

Add to the `AgentSession` struct:

```go
// Fact-based memory for belief revision (in-memory only).
// Not serialized — the session store is in-memory, not DB-persisted.
Facts *SafeMemory `json:"-"`
```

**Note:** `json:"-"` is correct here not because of the messages blob
(only `Messages` is JSON-encoded), but because `Facts` is an in-memory
structure that should never be serialized. No code path marshals the
whole `AgentSession`.

### Step 3: Add SafeMemory wrapper

**New file: `server/router/api/v1/agent/safe_memory.go`**

```go
package agent

import (
    "log/slog"
    "sync"

    memstate "github.com/PithomLabs/memstate"
)

// SafeMemory wraps memstate.Memory with a mutex for concurrent access.
// memstate has no internal locking; bchat accesses Facts from multiple
// goroutines. This wrapper prevents fatal concurrent-map writes.
type SafeMemory struct {
    mu   sync.Mutex
    mem  *memstate.Memory
}

func NewSafeMemory() *SafeMemory {
    return &SafeMemory{mem: memstate.New()}
}

func (s *SafeMemory) Add(text string) {
    if s == nil || s.mem == nil {
        return
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    s.mem.Add(text)
}

func (s *SafeMemory) Prompt(query string, budget int) string {
    if s == nil || s.mem == nil {
        return ""
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.mem.Prompt(query, budget)
}

func (s *SafeMemory) Facts(includePrivate bool) []*memstate.Fact {
    if s == nil || s.mem == nil {
        return nil
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.mem.Facts(includePrivate)
}
```

### Step 4: Initialize SafeMemory on session creation

**File: `server/router/api/v1/agent/service.go`**

In `MemorySessionStore.GetOrCreate()`, after creating the session:

```go
session := &store.AgentSession{
    // ... existing fields ...
}

// Initialize memstate if enabled
if isMemstateEnabled() {
    session.Facts = NewSafeMemory()
}
```

### Step 5: Track extracted facts in processChat()

**File: `server/router/api/v1/agent/service.go`**

In `processChat()`, AFTER `extractCollectedInfo()` runs (which produces
`customerInfo`), add:

```go
// Track extracted customer info as superseding facts
if isMemstateEnabled() && session.Facts != nil {
    if customerInfo.Name != "" {
        safeFactsAdd(session.Facts, "Customer name is "+customerInfo.Name)
    }
    if customerInfo.Phone != "" {
        safeFactsAdd(session.Facts, "Customer phone is "+customerInfo.Phone)
    }
    if customerInfo.Address != "" {
        safeFactsAdd(session.Facts, "Customer location is "+customerInfo.Address)
    }
}

func safeFactsAdd(mem *SafeMemory, text string) {
    if mem == nil {
        return
    }
    defer func() {
        if r := recover(); r != nil {
            slog.Error("memstate panicked on Add", "panic", r, "text", text)
        }
    }()
    mem.Add(text)
}
```

**Important:** `extractCollectedInfo()` is NOT replaced. It continues to
populate `customerInfo.Name/Phone/Address` as before. memstate supplements
the regex approach by adding belief revision — the same fact being extracted
multiple times supersedes the old value deterministically.

### Step 6: Inject current facts into system prompt

**File: `server/router/api/v1/agent/service.go`**

In `buildSystemPrompt()`, BEFORE the existing OM injection:

```go
// SECTION 0.5a: MEMSTATE FACTS (Current Beliefs)
if isMemstateEnabled() && session != nil && session.Facts != nil {
    factsBlock := session.Facts.Prompt("", 500)
    if factsBlock != "" {
        sb.WriteString("=== FACTS EXTRACTED FROM CUSTOMER ===\n\n")
        sb.WriteString("These facts were extracted from the customer's stated details. Use them to personalize your responses.\n\n")
        sb.WriteString(factsBlock)
        sb.WriteString("\n\n")
    }
}
```

**Header wording:** "FACTS EXTRACTED FROM CUSTOMER" — neutral, not
"verified." Avoids the prompt injection risk identified in adversarial Q4.

**Precedence:** Section 0.5a appears before 0.5b (OM). When facts conflict
with OM observations (e.g., memstate "prefers email" vs OM "frustrated with
email"), the LLM should treat memstate facts as stated customer preferences
and OM observations as inferred sentiment. An explicit instruction could be
added in a future iteration.

### Step 7: Add standalone memstate config

**New file: `server/router/api/v1/agent/memstate_config.go`**

```go
package agent

import "os"

// isMemstateEnabled returns whether memstate fact tracking is enabled.
// Defaults to false for safe rollout. Set MEMSTATE_ENABLED=true to enable.
func isMemstateEnabled() bool {
    return os.Getenv("MEMSTATE_ENABLED") == "true"
}
```

No `OMConfig` pollution. No struct. No singleton. Just a function.

## Files Changed

| File | Change |
|------|--------|
| `go.mod` | Add memstate dependency |
| `store/agent.go` | Add `Facts *SafeMemory` field to AgentSession |
| `agent/safe_memory.go` | **New:** thread-safe wrapper around memstate |
| `agent/memstate_config.go` | **New:** standalone `isMemstateEnabled()` |
| `agent/service.go` | Initialize Facts in GetOrCreate; add facts in processChat; inject into prompt |

**Total: ~60 lines of new code across 5 files. 0 lines removed.**

## What Stays Unchanged

- RAG pipeline
- FusionEngine
- Observer (still runs for complex inferences)
- Reflector
- All existing DB schemas (no migrations)
- All existing tests
- `extractCollectedInfo()` — continues to work as before

## Limitations

1. **External sessions only.** Internal/DB sessions loaded from storage will
   have `Facts == nil` because `Facts` is excluded from serialization.

2. **Session-scoped.** Facts do not persist across sessions or server restarts.
   Cross-session memory is a separate feature.

3. **Supersession tuning required.** The default threshold (0.55, IDF overlap)
   has not been validated against bchat's fact phrasings. Empirical testing
   with representative data is needed before enabling in production.

## Testing Strategy

1. **Unit tests** in memstate — already verified (12/12 pass)
2. **SafeMemory tests** — verify concurrent Add/Prompt calls don't crash
3. **Supersession test** — "Customer location is Rome" → "Customer location
   is Milan" supersedes; "I need help with billing" → "I need help with my
   account" does NOT supersede
4. **Regression test** — run bchat test suite with `MEMSTATE_ENABLED=false`
   (default) to verify no breakage
5. **Integration test** — simulate conversation with changing customer info,
   verify only current facts appear in system prompt
6. **A/B comparison** — enable for a subset of tenants, measure answer accuracy

## Rollback

`MEMSTATE_ENABLED` defaults to `false`. No action needed to disable.
If enabled and issues arise, set `MEMSTATE_ENABLED=false` in environment.

---

## Adversarial Review Prompt

You are a senior security and reliability engineer reviewing this revised
integration plan for a production AI agent platform. The original plan was
reviewed by two adversarial reviewers who identified critical issues that
have been addressed in this revision.

Your job is to verify that all previously identified issues are resolved
and find any NEW issues introduced by the fixes.

Specifically verify:

1. **Thread-safety:** The SafeMemory wrapper adds a mutex. Is the locking
   scope correct? Could there be deadlocks? Is the mutex held for the
   minimum necessary time?

2. **Panic recovery:** recover() is used in safeFactsAdd. Is this sufficient?
   Are there code paths that bypass the wrapper?

3. **Prompt injection:** Raw messages are no longer stored as facts. Is the
   new header wording ("FACTS EXTRACTED FROM CUSTOMER") neutral enough?
   Could extracted facts still be manipulated?

4. **Dependency management:** The plan recommends vendoring. Is this
   appropriate for a ~500-line library? What are the audit implications?

5. **Supersession correctness:** The plan acknowledges the threshold is
   unvalidated. What specific test cases should be required before enabling
   in production?

6. **Config isolation:** isMemstateEnabled() reads from os.Getenv directly.
   Is this testable? Should it be overridable for tests?

7. **Backward compatibility:** With MEMSTATE_ENABLED=false (default), does
   the system behave identically to before? Are there any implicit defaults
   that could change behavior?

8. **What's missing?** Are there any integration points the plan overlooks?
   Any edge cases at the boundary between memstate and the rest of bchat?

For each issue found, provide:
- Severity (critical / high / medium / low)
- Whether it's a regression from the original plan or a new finding
- Recommended fix

Output your findings as a structured report.
