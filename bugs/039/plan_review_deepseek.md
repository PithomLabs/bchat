# Adversarial Plan Review: memstate Integration into bchat

**Reviewer:** DeepSeek (adversarial)
**Plan:** `bugs/039/plan.md`
**Date:** 2026-07-15

---

## Verdict: APPROVED WITH NITS

The core idea — adding deterministic belief revision via memstate alongside Observational Memory — is sound. The architecture diagram cleanly separates concerns. However, four critical and four high-severity issues must be addressed before this plan is ready for implementation.

---

## Critical

### 1. `MEMSTATE_ENABLED` defaults to `true` — breaks backward compatibility

**Severity:** Critical

**Failure scenario:** Step 9 sets `MemstateEnabled: getEnvBool("MEMSTATE_ENABLED", true)`. Every existing deployment without this env var will silently activate memstate, changing LLM prompt composition. If memstate bugs or unexpected supersession behavior manifest, all tenants are affected with no grace period.

**Fix:** Default to `false`. Existing behavior is the safe default. Early adopters can opt in explicitly. Change the loadOMConfig line to:

```go
MemstateEnabled: getEnvBool("MEMSTATE_ENABLED", false),
```

**Handling:** Fix in this plan before implementation.

---

### 2. Cross-session memory (Steps 6–8) is unreachable for external sessions

**Severity:** Critical

**Failure scenario:** The plan gates cross-session persistence on `if session.UserID != nil`. But `MemorySessionStore.GetOrCreate()` — the sole session factory for `ChatExternal` — never sets `UserID`. It remains `nil` (see `store/agent.go:236`). `ChatExternal` has no code path that assigns `UserID`. Therefore the entire cross-session memory subsystem (Steps 6, 7, 8) is unreachable for its intended audience.

`ChatInternal` does set `UserID` (line 1996), but uses DB-backed sessions created via `store.CreateAgentSession()`, not `MemorySessionStore`. The plan never modifies `ChatInternal` or the DB session initialization path, so cross-session memory doesn't work for internal sessions either.

**Fix:** Either:
- (a) Remove cross-session memory from this plan entirely; ship session-scoped facts only.
- (b) Add a mechanism to identify external users (e.g., HMAC-authenticated sessions or a user lookup flow) and set `session.UserID` before `processChat()`.

Option (a) is strongly recommended — cross-session memory is a separate feature with its own persistence and eviction concerns.

**Handling:** Fix in this plan before implementation.

---

### 3. `userMemories` map grows unbounded — memory leak

**Severity:** Critical

**Failure scenario:** The `map[string]*memstate.Memory` on `Service` has no eviction, TTL, or size cap. Each `memstate.Memory` grows with every cross-session fact added. For a SaaS platform serving thousands of users, this will consume unbounded heap memory. A busy tenant with 10K users × ~1KB per memory = 10MB, growing monotonically. Server never reclaims it.

**Fix:** Add a TTL-based cleanup goroutine (pattern already exists in `MemorySessionStore.cleanupLoop()` at line 1240) or cap the map with LRU eviction. The `cleanupLoop` goroutine pattern is already proven in this codebase and should be reused.

**Handling:** Fix in this plan before implementation. If cross-session memory is removed (see #2), this concern is moot.

---

### 4. Adding raw `userMessage` to facts creates noise and risks false supersession

**Severity:** Critical

**Failure scenario:** Step 4 calls `session.Facts.Add(userMessage)` with the entire user message turn. memstate uses token-overlap-based topic matching for supersession. A user saying *"I need help with my refrigerator"* then *"I need help with my car"* shares significant token overlap ("I need help with my"). memstate may incorrectly supersede the refrigerator fact with the car fact.

Additionally, non-factual utterances (*"I'm not sure"*, *"What did you say?"*, *"Can you repeat that?"*) are added as facts, polluting the belief store with noise.

**Fix:** Only add *extracted* structured facts, never raw messages. The code already has `customerInfo` from `extractCollectedInfo()`:

```go
if session.Facts != nil {
    if customerInfo.Name != "" {
        session.Facts.Add("Customer name is " + customerInfo.Name)
    }
    if customerInfo.Phone != "" {
        session.Facts.Add("Customer phone is " + customerInfo.Phone)
    }
    // ... other extracted fields
}
```

Remove the `session.Facts.Add(userMessage)` line entirely.

**Handling:** Fix in this plan before implementation.

---

## High

### 5. No panic recovery around memstate calls

**Severity:** High

**Failure scenario:** If memstate panics — due to a nil pointer inside `memstate.New()`, a version mismatch, or a bug in `Add()` — the entire `ChatExternal` handler crashes. The "toggle to disable" guard is useless if the panic happens before the toggle check, or if the toggle is `true` and a bug surfaces.

**Fix:** Wrap all memstate calls (`Add`, `Prompt`, `Facts`) in `defer recover()` blocks, or provide a single panic-safe wrapper method on `Service`. At minimum:

```go
func safeFactsAdd(mem *memstate.Memory, text string) {
    if mem == nil { return }
    defer func() {
        if r := recover(); r != nil {
            slog.Error("memstate panicked on Add", "panic", r, "text", text)
        }
    }()
    mem.Add(text)
}
```

**Handling:** Fix in this plan before implementation.

---

### 6. Cross-session memory lost on server restart

**Severity:** High

**Failure scenario:** `userMemories` is purely in-memory with no persistence. A deployment restart, pod scale-down, or crash loses all accumulated cross-session facts. For a production system supporting multi-day or multi-week customer journeys, this is a significant data loss regression.

**Fix:** If cross-session memory is retained (see #2), it must be persisted — either to a new `agent_user_memory` table or appended to the existing `agent_session` table. This breaks the plan's "no DB migration" constraint, which means either:
- Accept the tradeoff and add a migration, or
- Remove cross-session memory from scope.

**Handling:** Fix in this plan, or remove cross-session memory from scope.

---

### 7. `OMConfig` is the wrong conceptual home for `MemstateEnabled`

**Severity:** High

**Failure scenario:** Adding `MemstateEnabled` to `OMConfig` in `om_config.go` conflates two independent subsystems. memstate has nothing to do with Observational Memory. A future developer looking at `OMConfig` will reasonably assume the field affects OM behavior. The `GetConfig()` copy (Step 9) also needs updating, increasing the maintenance surface.

**Fix:** Create a standalone `memstateConfig` package-level singleton, or use a simple `os.Getenv` check directly in the toggle guards. Do not pollute `OMConfig`.

**Handling:** Fix in this plan before implementation.

---

### 8. Token savings claims are speculative, not enforced

**Severity:** High

**Failure scenario:** The plan claims "OM: ~500 tokens → ~300 tokens" and "Fewer Observer triggers." Neither is enforced by the code changes:
- The OM section in `buildSystemPrompt` (line 2667) is untouched. The Observer still writes full observations into the prompt.
- The Observer triggers on a 30K token threshold of *unobserved content*. memstate doesn't change user message volume, so trigger frequency is unchanged.
- The `memstate.Prompt("", 500)` budget parameter is advisory — if memstate doesn't enforce it, the actual token count could exceed the budget and silently eat into the context window.

**Fix:** Remove token savings from the "Before/After" table, or label them as aspirational/untargeted. Add a hard token cap (e.g., truncate the facts block to 500 tokens using a tokenizer, not a character count). Be honest that the primary benefit is *answer accuracy*, not *token efficiency*.

**Handling:** Fix in this plan before implementation.

---

## Medium

### 9. Plan claims to "replace" `extractCollectedInfo` but doesn't

**Severity:** Medium

The plan says *"This replaces the fragile regex-based `extractCollectedInfo()` approach."* But Step 4 still calls `extractCollectedInfo()` and uses its results. memstate is an addition, not a replacement. The regex code stays in place unchanged.

**Fix:** Rephrase as *"This supplements the regex-based approach for inline belief revision."* A full replacement would require a separate plan to deprecate `extractCollectedInfo`.

---

### 10. Integration is external-session-only but undocumented

**Severity:** Medium

The `json:"-"` tag on `Facts` means internal/DB sessions loaded from storage will have `Facts == nil`. The plan doesn't state this limitation, which will confuse future developers who try to use `session.Facts` in both paths.

**Fix:** Add a "Limitations" section: *"This integration applies to external (in-memory) sessions only. Internal sessions are not affected because Facts is excluded from serialization."*

---

### 11. Supersession false positive risk from token overlap

**Severity:** Medium

As identified by the plan's own adversarial review prompt (item 4), token-overlap-based supersession could match across unrelated service lines. The plan provides no mitigation.

**Mitigation:** If memstate supports a minimum similarity threshold, add one (e.g., `memstate.WithMinSimilarity(0.6)`). Otherwise, document the accepted risk.

---

## Nits

12. **`go.mod` dependency resolution:** The plan specifies `require github.com/PithomLabs/memstate v0.1.0`. Verify this package resolves with `go get` before implementation. If it's a private or unpublished package, document the install instruction.

13. **Unverified API surface:** The plan assumes `memstate.Memory`, `memstate.New()`, `.Add()`, `.Prompt(budget)`, `.Facts(includeNonCurrent)`, `.Current()`, and `.Text` all exist with exactly these signatures. Verify against the actual memstate v0.1.0 API.

14. **Line number fragility:** The plan references line offsets in `service.go` (e.g., "around line 1177", "around line 2095"). These will shift when other features merge. Replace with function-name-anchored references: *"In `MemorySessionStore.GetOrCreate()`, after initializing the `AgentSession` struct"*.

15. **Missing `go.mod` context:** The plan shows `require github.com/PithomLabs/memstate v0.1.0` but doesn't show which `require` block it belongs to. The `go.mod` has multiple `require` blocks (lines 5, 40, 88). Specify the correct location.

16. **Double-checked locking idiomatic alternative:** The `getUserMemory` pattern is correct Go, but `sync.Map` or `github.com/hashicorp/golang-lru` would be more idiomatic and provide free eviction. Consider if the effort of a third-party dep is justified for a single use case.

---

## Summary

| Severity | Count | Action required |
|----------|-------|-----------------|
| Critical | 4 | Must fix before merge |
| High     | 4 | Strongly recommended before merge |
| Medium   | 3 | Fix if time permits |
| Nit      | 5 | Consider before merge |

### Minimal viable scope (recommended)

To ship this feature quickly and safely:

1. Default `MEMSTATE_ENABLED` to `false` (#1)
2. Remove cross-session memory (Steps 6–8) entirely — ship session-scoped facts only (#2, #3, #6)
3. Remove `session.Facts.Add(userMessage)` — only add extracted facts (#4)
4. Add `recover()` wrappers around all memstate calls (#5)
5. Move `MemstateEnabled` out of `OMConfig` into standalone config (#7)
6. Remove speculative token savings claims (#8)
7. Document the external-session-only limitation (#10)

This shrinks the plan to ~25 lines of production code, eliminates all critical and high issues, and delivers the core value (deterministic belief revision) without the cross-session persistence risk.
