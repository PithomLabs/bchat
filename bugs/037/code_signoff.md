# Bug 037 — Code Signoff

**Date:** 2026-07-14
**Status:** APPROVED

---

## Summary

Two bugs prevented the evpn tenant from answering questions that exist in its KB.
The `forceRAG` variable overrode explicit `RetrievalMode`, and `buildSystemPrompt`
never injected raw KB content for unannotated tenants. Both are fixed. The evpn
agent now correctly answers "Does throttling affect mobile data?"

---

## Fixes Applied

### Fix 1: `forceRAG` overrides `RetrievalMode` (P0-2)

**Root cause:** In `processChat`, `forceRAG` was set to `!config.HasStructuredContent && s.UseRAGPipeline()`. Plain markdown (like evpn.md) has no structured annotations, so `HasStructuredContent=false`, making `forceRAG=true` regardless of the explicit `RetrievalMode`. The agent ran in RAG mode with no index, producing "retrieved context does not provide."

**What changed:** `service.go:2163-2202` — Rewrote the mode decision to check `RetrievalMode` first, then fall back to `HasStructuredContent`. Removed `forceRAG` variable entirely.

**Decision tree (after fix):**

| Priority | Condition | Mode |
|----------|-----------|------|
| 1 | RAG pipeline disabled | long_context |
| 2 | `RetrievalMode == "rag"` | RAG |
| 3 | `RetrievalMode == "long_context"` | long_context |
| 4 | `RetrievalMode == ""` + no structured content + RAG enabled | RAG |
| 5 | `RetrievalMode == ""` + structured content | long_context |

### Fix 2: `buildSystemPrompt` missing RawKB injection

**Root cause:** `buildSystemPrompt` (long_context mode) only includes parsed sections (`config.Services`, `config.FAQs`, etc.). For plain markdown without `@service`/`@faq` annotations, these are empty. The LLM sees zero KB content and cannot answer any question.

**What changed:** `service.go:2856-2867` — Added Section 8B between FAQs (Section 8) and Contact Info (Section 9). Injects `config.RawKB` truncated to 25K tokens when parsed sections are empty.

**Scope:** Only applies to long_context mode tenants with plain markdown (no annotations). Annotated tenants are unaffected — the condition `len(config.Services) == 0 && len(config.FAQs) == 0` prevents injection when structured sections exist.

### Fix 3: `recalcContentTokens` (already implemented)

**What it does:** Detects `content_tokens == 0` on first chat request, recalculates from source files, and persists the correct `RetrievalMode` (rag for >= 30K tokens, long_context otherwise).

**Location:** `service.go:2173-2175` (call site), `service.go:2951-2980` (implementation)

---

## Files Modified

| File | Lines | Change |
|------|-------|--------|
| `service.go` | 2163-2202 | Rewrote mode decision — removed `forceRAG`, added `recalcContentTokens` call, priority-based logic |
| `service.go` | 2856-2867 | Added Section 8B — RawKB injection for long_context mode with unannotated KB |
| `service.go` | 2948-2980 | `recalcContentTokens` helper — one-time fix for `content_tokens=0` tenants |
| `service.go` | 2923-2931 | `truncateToTokenBudget` — used by Section 8B for safe truncation |

---

## Verification

### Go tests

```
$ go test ./server/router/api/v1/agent/... -count=1 -timeout 120s
ok  github.com/usememos/memos/server/router/api/v1/agent  6.209s
```

All tests pass.

### Manual test: evpn "Does throttling affect mobile data?"

**Before fix:** Agent replied "the provided materials do not specifically address whether throttling affects mobile data."

**After fix:** Agent correctly answers that mobile carriers can throttle data like home broadband providers, and ExpressVPN works across mobile networks to reduce activity-based slowdowns.

**Verified content source:** `evpn.md:3376-3378`:
```
### Does throttling affect mobile data?
Mobile carriers can throttle data just like home broadband providers, especially
during congestion or after you hit usage limits. ExpressVPN works across mobile
networkes and encrypts your traffic, helping reduce activity-based slowdowns.
```

### Decision tree coverage

| Test case | Input | Expected | Result |
|-----------|-------|----------|--------|
| RAG pipeline disabled | `UseRAGPipeline()=false` | long_context | PASS |
| Explicit RAG | `RetrievalMode="rag"` | RAG | PASS |
| Explicit long_context | `RetrievalMode="long_context"` | long_context | PASS |
| Unset + unstructured + RAG enabled | `RetrievalMode=""`, `HasStructuredContent=false` | RAG | PASS |
| Unset + structured | `RetrievalMode=""`, `HasStructuredContent=true` | long_context | PASS |

---

## Known Limitations

### 1. Bootstrapping gap (first chat before reindex)

When `recalcContentTokens` switches a tenant from long_context to RAG on the first chat:
- The mode switch persists immediately
- But no RAG index exists yet
- The first chat uses RAG fallback (`truncateToTokenBudget(RawKB, 6000)`) which may not include the relevant section
- User must trigger reindex (manual button or server restart with auto-bootstrap) before the second chat

**Mitigation:** Server auto-bootstrap at startup (`service.go:156-179`) detects empty vector DB + existing source files and reindexes all tenants.

### 2. Section 8B truncation for large KBs

Section 8B uses a 25K token budget (~100KB). For KBs larger than ~100KB, content beyond the first ~28% is truncated. This is by design — long_context mode is intended for KBs < 30K tokens (the `DefaultTokenThreshold`). Large KBs should use RAG mode.

### 3. RAG fallback token budget

The RAG fallback (`ragFallbackTokenBudget = 6000`) is even smaller. For large KBs without a RAG index, the fallback provides minimal context. This is acceptable because large KBs should have a RAG index.

---

## Remaining Items

- [ ] Deploy and test with production evpn tenant
- [ ] Monitor `recalcContentTokens` log output for other tenants with `content_tokens=0`
- [ ] Consider auto-reindex trigger when mode switches (optional enhancement)
