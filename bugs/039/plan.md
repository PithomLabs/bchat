# memstate Integration into bchat

## Overview

memstate (github.com/PithomLabs/memstate) is a deterministic, zero-dependency
state memory library. It adds **belief revision** to bchat: when a customer
changes their mind, the old fact is automatically superseded — no LLM required.

This plan adds memstate as a third memory layer alongside bchat's existing
Observational Memory (OM) and RAG.

## Problem Statement

bchat currently has no mechanism to revise beliefs. When a customer says
"My name is John" then later "Actually, Jonathan", both values persist in
session state and OM. The LLM must figure out which is current at query time,
which it often gets wrong.

## Architecture

```
User message arrives
        │
        ▼
┌──────────────────────────────────┐
│  session.Facts.Add(userMessage)  │  Deterministic, instant
│  (belief revision via memstate)  │
└──────────┬───────────────────────┘
           │
           ▼
┌──────────────────────────────────┐
│  processChat() continues         │
│  (classify, policy, generate)    │
└──────────┬───────────────────────┘
           │
           ▼
┌──────────────────────────────────┐
│  buildSystemPrompt()             │
│  Section 0.5a: memstate facts    │  ← current beliefs, token-budgeted
│  Section 0.5b: OM observations   │  ← complex inferences (unchanged)
│  Section 0.5c: cross-session     │  ← user-level facts
└──────────┬───────────────────────┘
           │
           ▼
┌──────────────────────────────────┐
│  Observer (async, still runs)    │
│  Only for sentiment, intent,     │
│  and multi-turn inferences       │
└──────────────────────────────────┘
```

## Implementation Steps

### Step 1: Add memstate dependency to bchat

**File: `go.mod`**

Add:
```
require github.com/PithomLabs/memstate v0.1.0
```

Run `go mod tidy` to download and generate go.sum.

### Step 2: Add Facts field to AgentSession

**File: `store/agent.go`**

Add to the `AgentSession` struct (around line 232):

```go
// Fact-based memory for belief revision (in-memory only, not persisted)
Facts *memstate.Memory `json:"-"`
```

The `json:"-"` tag ensures Facts is not serialized to the `messages` JSON blob.
External sessions are ephemeral (30-min TTL) so facts don't need DB persistence.

**Why not persist?**
- External sessions are in-memory only (30-min TTL)
- Cross-session memory is handled separately via per-user store (Step 6)
- No DB migration needed

### Step 3: Initialize Facts on session creation

**File: `server/router/api/v1/agent/service.go`**

In `MemorySessionStore.GetOrCreate()` (around line 1177), initialize Facts:

```go
session := &store.AgentSession{
    // ... existing fields ...
    Facts: memstate.New(),  // ← NEW
}
```

### Step 4: Track facts from user messages in processChat()

**File: `server/router/api/v1/agent/service.go`**

In `processChat()`, after the user message is added to session.Messages
(around line 2095), add:

```go
// Track evolving beliefs from user message
if session.Facts != nil {
    session.Facts.Add(userMessage)

    // Track extracted customer info as superseding facts
    if customerInfo.Name != "" {
        session.Facts.Add("Customer name is " + customerInfo.Name)
    }
    if customerInfo.Phone != "" {
        session.Facts.Add("Customer phone is " + customerInfo.Phone)
    }
    if customerInfo.Address != "" {
        session.Facts.Add("Customer location is " + customerInfo.Address)
    }
}
```

This replaces the fragile regex-based `extractCollectedInfo()` approach.
Supersession handles "I live in Rome" → "I live in Milan" automatically.

### Step 5: Inject current facts into system prompt

**File: `server/router/api/v1/agent/service.go`**

In `buildSystemPrompt()`, add a new sub-section BEFORE the existing OM
injection (around line 2667):

```go
// SECTION 0.5a: MEMSTATE FACTS (Current Beliefs)
if session != nil && session.Facts != nil {
    factsBlock := session.Facts.Prompt("", 500)
    if factsBlock != "" {
        sb.WriteString("=== CURRENT FACTS (Superseded values already removed) ===\n\n")
        sb.WriteString("The following are verified current facts about this customer:\n\n")
        sb.WriteString(factsBlock)
        sb.WriteString("\n\n")
    }
}
```

The existing OM injection (Section 0.5b) stays unchanged for complex
observations like sentiment and inferred intent.

**Token budget:** 500 tokens for facts. OM still gets its full allocation.

### Step 6: Add per-user cross-session memory

**File: `server/router/api/v1/agent/service.go`**

Add to Service struct (around line 50):

```go
type Service struct {
    // ... existing fields ...
    userMemories   map[string]*memstate.Memory  // keyed by "user_{id}"
    userMemoriesMu sync.RWMutex
}
```

Initialize in `NewService()` (around line 68):

```go
userMemories: make(map[string]*memstate.Memory),
```

Add helper method:

```go
func (s *Service) getUserMemory(resourceID string) *memstate.Memory {
    s.userMemoriesMu.RLock()
    mem, ok := s.userMemories[resourceID]
    s.userMemoriesMu.RUnlock()
    if ok {
        return mem
    }
    s.userMemoriesMu.Lock()
    defer s.userMemoriesMu.Unlock()
    if mem, ok = s.userMemories[resourceID]; ok {
        return mem
    }
    mem = memstate.New()
    s.userMemories[resourceID] = mem
    return mem
}
```

### Step 7: Persist cross-session facts after response

**File: `server/router/api/v1/agent/service.go`**

In `ChatExternal()`, after `processChat()` returns and before
`memorySessions.Update(session)` (around line 1946):

```go
// Persist cross-session facts for authenticated users
if session.UserID != nil {
    resourceID := fmt.Sprintf("user_%d", *session.UserID)
    userMem := s.getUserMemory(resourceID)
    for _, fact := range session.Facts.Facts(false) {
        if fact.Current() {
            userMem.Add(fact.Text)
        }
    }
}
```

### Step 8: Inject cross-session memory into prompt

**File: `server/router/api/v1/agent/service.go`**

In `buildSystemPrompt()`, after Section 0.5a:

```go
// SECTION 0.5c: CROSS-SESSION USER MEMORY
if session != nil && session.UserID != nil {
    resourceID := fmt.Sprintf("user_%d", *session.UserID)
    userMem := s.getUserMemory(resourceID)
    userFacts := userMem.Prompt("", 300)
    if userFacts != "" {
        sb.WriteString("=== KNOWN FACTS FROM PREVIOUS CONVERSATIONS ===\n\n")
        sb.WriteString(userFacts)
        sb.WriteString("\n\n")
    }
}
```

### Step 9: Add memstate toggle to OM config

**File: `server/router/api/v1/agent/om_config.go`**

Add to OMConfig struct:

```go
MemstateEnabled bool  // Enable memstate fact tracking (default: true)
```

Add to `loadOMConfig()`:

```go
MemstateEnabled: getEnvBool("MEMSTATE_ENABLED", true),
```

Add to `GetConfig()` copy:

```go
MemstateEnabled: c.MemstateEnabled,
```

### Step 10: Guard all memstate calls with the toggle

Wrap all memstate calls in `if config.MemstateEnabled` checks so the feature
can be disabled without code changes.

## Files Changed

| File | Change | Lines affected |
|------|--------|----------------|
| `go.mod` | Add memstate dependency | 1 line |
| `store/agent.go` | Add Facts field to AgentSession | 2 lines |
| `service.go` (MemorySessionStore) | Initialize Facts | 1 line |
| `service.go` (processChat) | Add facts from user message | ~10 lines |
| `service.go` (buildSystemPrompt) | Inject facts into prompt | ~20 lines |
| `service.go` (Service struct) | Add userMemories field | 3 lines |
| `service.go` (ChatExternal) | Persist cross-session facts | ~10 lines |
| `om_config.go` | Add MemstateEnabled toggle | ~5 lines |

**Total: ~50 lines of new code, 0 lines removed.**

## What Stays Unchanged

- RAG pipeline (vector search over KB chunks)
- FusionEngine (merges OM + RAG)
- Observer (still runs for complex inferences)
- Reflector (still compresses OM)
- All existing DB schemas (no migrations needed)
- All existing tests

## Token Savings Estimate

| Before | After | Change |
|--------|-------|--------|
| OM: ~500 tokens in prompt | OM: ~300 tokens | -200 |
| Customer info: scattered | Facts: ~200 tokens (budgeted) | Consolidated |
| Observer: triggers at 30K tokens | Fewer triggers | Fewer LLM calls |
| `extractCollectedInfo()`: regex | `session.Facts.Recall()`: deterministic | Removed |

## Testing Strategy

1. **Unit tests** in memstate — already verified (12/12 pass)
2. **Manual test**: simulate conversation where customer changes name, verify
   only current name appears in system prompt
3. **Regression test**: run existing bchat test suite with `MEMSTATE_ENABLED=false`
   to verify no breakage
4. **A/B comparison**: toggle MEMSTATE_ENABLED and measure Observer call frequency,
   token usage, and customer info accuracy

## Rollback

Set `MEMSTATE_ENABLED=false` in environment. All memstate calls are guarded
by the toggle. bchat falls back to existing OM-only behavior.

---

## Adversarial Review Prompt

You are a senior security and reliability engineer reviewing this integration
plan for a production AI agent platform. Your job is to find every possible
failure mode, race condition, security issue, and design flaw.

Review the following integration plan and identify:

1. **Race conditions**: The bchat system is highly concurrent. ChatExternal
   calls are serialized per-session, but cross-session memory access is not.
   Are there races on userMemories? On session.Facts?

2. **Memory leaks**: userMemories is a map that grows unbounded. Is there a
   cleanup mechanism? What happens after thousands of users?

3. **Token budget violations**: memstate.Prompt(budget=500) is an estimate.
   Could the actual token count exceed the budget and break the LLM context
   window?

4. **Supersession false positives**: Could memstate's token-overlap-based
   topic matching incorrectly supersede unrelated facts? For example, could
   "I need help with my account" supersede "I need help with billing"?

5. **Data consistency**: If a session is evicted from MemorySessionStore
   (30-min TTL) and a new session starts, are cross-session facts properly
   loaded? Could facts be lost?

6. **Security**: Could a malicious user craft messages that manipulate
   memstate's supersession to inject false beliefs?

7. **Performance**: memstate adds O(n) supersession checks per Add() call.
   With long conversations (100+ messages), could this become a bottleneck?

8. **Backward compatibility**: If MEMSTATE_ENABLED is not set, does the
   system behave identically to before? Are there any implicit defaults
   that could change behavior?

9. **Graceful degradation**: What happens if memstate panics? Should there
   be a recover() wrapper?

10. **Interaction with existing OM**: Could memstate facts conflict with
    OM observations? For example, memstate says "Customer prefers email"
    while OM says "Customer seems frustrated with email". Which wins?

For each issue found, provide:
- Severity (critical / high / medium / low)
- Failure scenario
- Recommended fix
- Whether the fix should be in this plan or handled separately

Output your findings as a structured report.
