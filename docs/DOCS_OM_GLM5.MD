# Observational Memory (OM) Implementation Gap Analysis

**Date:** 2026-02-13
**Status:** Complete
**Model Used:** GLM-5
**Context:** Comprehensive review of OM implementation against Mastra specifications

---

## Executive Summary

This document provides a comprehensive gap analysis of the Observational Memory (OM) implementation in bchat, comparing it against the Mastra OM specification documented in [`DOCS_OM.md`](./DOCS_OM.md), [`DOCS_OM_ANALYSIS.MD`](./DOCS_OM_ANALYSIS.MD), [`DOCS_OM_IMPLEMENTATION.MD`](./DOCS_OM_IMPLEMENTATION.MD), [`DOCS_OM_MINIMAX.MD`](./DOCS_OM_MINIMAX.MD), and [`DOCS_OM_MINIMAX_PLAN.MD`](./DOCS_OM_MINIMAX_PLAN.MD).

**Overall Implementation Completeness: ~65%**

---

## Critical Gaps

| Gap | Mastra OM Feature | bchat Status | Impact |
|-----|-------------------|--------------|--------|
| **MySQL/PostgreSQL Support** | Multi-database | ❌ Returns error | Cannot deploy on non-SQLite databases |
| **Token-based Observer Trigger** | 30,000 tokens | ❌ Uses message count (10 msgs) | Inconsistent triggering for verbose conversations |
| **Async Buffering** | Pre-computes observations in background | ❌ Not implemented | User experiences pause when threshold hit |

---

## Feature Comparison: Mastra vs bchat

```
Mastra OM Features                    bchat Implementation
─────────────────────────────────────────────────────────────────
Token-based triggers (30k/40k)   →    Message count triggers (10 msgs)
Async buffering (background)     →    Sync only (blocking)
Thread/Resource scopes           →    Thread scope only
Current task tracking            →    ❌ Not extracted
Suggested response hints         →    ❌ Not extracted
Prompt caching optimization      →    ❌ Not implemented
Multi-database support           →    SQLite only
```

---

## Detailed Gap Analysis

### 1. Token Thresholds vs Message Count

**Mastra Specification:**
- `messageTokens: 30,000` - Triggers Observer when unobserved messages exceed this
- `observationTokens: 40,000` - Triggers Reflector when observations exceed this

**bchat Implementation:**
- `MessageThreshold: 10` - Triggers Observer after 10 messages
- `TokenThreshold: 2000` - Triggers Reflector at 2000 tokens

**Issue:** A verbose conversation with 10 short messages triggers Observer unnecessarily, while 5 long messages don't trigger at all. This is inconsistent with the token-based design intent.

**Location:**
- [`om_config.go:35`](../server/router/api/v1/agent/om_config.go:35)
- [`service.go:1697-1705`](../server/router/api/v1/agent/service.go:1697)

**Recommendation:** Implement token-based triggering using a proper tokenizer (e.g., tiktoken).

---

### 2. Async Buffering (Not Implemented)

**Mastra Specification:**
- Pre-computes observations at 20% intervals (`bufferTokens: 0.2`)
- Activates instantly when threshold is reached
- User never experiences blocking

**bchat Implementation:**
- Synchronous only - user waits for Observer LLM call
- No background pre-computation

**Impact:** Poor UX when threshold is reached mid-conversation. The user must wait for the Observer LLM call to complete before receiving a response.

**Reference:** [`DOCS_OM.md:676-693`](./DOCS_OM.md:676)

**Recommendation:** Implement background goroutine that pre-computes observations at configurable intervals.

---

### 3. Current Task & Suggested Response (Not Extracted)

**Mastra Specification:**
The Observer prompt generates:
- `<current-task>` - What the agent is currently working on
- `<suggested-response>` - Hint for the agent's immediate next message

**bchat Implementation:**
- Prompt generates these tags (see [`prompts/observer.txt:116-129`](../server/router/api/v1/agent/prompts/observer.txt:116))
- Code only extracts `<observations>` tag
- Current task and suggested response are discarded

**Location:** [`observer.go:156`](../server/router/api/v1/agent/observer.go:156)

**Impact:** Lost continuity hints for the agent. The agent doesn't know what it was working on or how to continue.

**Recommendation:** Parse and store current-task and suggested-response, inject into next LLM call.

---

### 4. Thread vs Resource Scope

**Mastra Specification:**
- `thread` scope: Each conversation has its own observations (default)
- `resource` scope: Observations shared across all conversations for a user (experimental)

**bchat Implementation:**
- Only thread scope (per-session)
- No cross-conversation memory sharing

**Impact:** Cannot share memory across multiple conversations for the same user. Each session starts fresh.

**Reference:** [`DOCS_OM.md:619-651`](./DOCS_OM.md:619)

**Recommendation:** Implement resource scope as an optional feature for cross-conversation memory.

---

### 5. Prompt Caching (Not Implemented)

**Mastra Specification:**
- Stable context prefix enables 4-10x cost reduction
- Observations append-only until reflection
- System prompt + observations form cacheable prefix

**bchat Implementation:**
- No explicit caching implementation
- Observations injected into system prompt but not optimized for caching

**Reference:** 
- [`DOCS_OM.md:8`](./DOCS_OM.md:8)
- [`DOCS_OM.md:224`](./DOCS_OM.md:224)

**Recommendation:** Structure the system prompt to maximize cache hits, using provider-specific caching APIs.

---

### 6. Database Support

| Database | Status | Location |
|----------|--------|----------|
| SQLite | ✅ Fully implemented | [`store/db/sqlite/agent_observations.go`](../store/db/sqlite/agent_observations.go) |
| MySQL | ❌ Returns "not implemented" error | [`store/db/mysql/agent_observations.go:11`](../store/db/mysql/agent_observations.go:11) |
| PostgreSQL | ❌ Returns "not implemented" error | [`store/db/postgres/agent_observations.go:11`](../store/db/postgres/agent_observations.go:11) |

**Impact:** Cannot deploy OM on production databases other than SQLite.

**Recommendation:** Implement MySQL and PostgreSQL support following the SQLite pattern.

---

## Architectural Limitations (Inherent to OM Design)

These limitations are inherent to the OM architecture as documented by Mastra, not implementation gaps:

| Limitation | Description | Mitigation |
|------------|-------------|------------|
| **Lossy Compression** | 80-97% of original information discarded per cycle | Accept as tradeoff for context efficiency |
| **Hard Knowledge Ceiling** | ~70k tokens max (observations + messages) | Implement hybrid with RAG for long-term storage |
| **Irreversible** | Once compressed, cannot recover original phrasing | Store raw messages separately if needed |
| **Cost Scaling** | Observer/Reflector regenerate entire log as output tokens | Use cheaper models for Observer/Reflector |

**Reference:** [`DOCS_OM.md:742-750`](./DOCS_OM.md:742) - External critique of OM architecture

---

## Implementation Status Summary

### Completed Features ✅

| Feature | Location |
|---------|----------|
| Database schema (SQLite) | [`store/migration/sqlite/0.25/24__agent_observations.sql`](../store/migration/sqlite/0.25/24__agent_observations.sql) |
| Observer prompt | [`server/router/api/v1/agent/prompts/observer.txt`](../server/router/api/v1/agent/prompts/observer.txt) |
| Reflector prompt | [`server/router/api/v1/agent/prompts/reflector.txt`](../server/router/api/v1/agent/prompts/reflector.txt) |
| RunObserver method | [`observer.go:57`](../server/router/api/v1/agent/observer.go:57) |
| Context injection | [`service.go:1764-1773`](../server/router/api/v1/agent/service.go:1764) |
| Async execution (goroutine) | [`service.go:1714`](../server/router/api/v1/agent/service.go:1714) |
| Configuration (OM_ENABLED, thresholds) | [`om_config.go`](../server/router/api/v1/agent/om_config.go) |
| Message threshold trigger | [`service.go:1697-1705`](../server/router/api/v1/agent/service.go:1697) |
| Debouncing (ObserverMutex) | [`om_config.go:89-133`](../server/router/api/v1/agent/om_config.go:89) |
| Unit tests | [`observer_test.go`](../server/router/api/v1/agent/observer_test.go) |
| Selective observation (trivial filtering) | [`observer.go:114-122`](../server/router/api/v1/agent/observer.go:114) |
| Memory analytics (compression ratio) | [`observer.go:186-194`](../server/router/api/v1/agent/observer.go:186) |
| Retry logic | [`observer.go:140-145`](../server/router/api/v1/agent/observer.go:140) |

### Missing Features ❌

| Feature | Priority | Effort |
|---------|----------|--------|
| MySQL/PostgreSQL support | Critical | Medium |
| Token-based Observer trigger | Critical | Medium |
| Async buffering | High | High |
| Current task extraction | Medium | Low |
| Suggested response extraction | Medium | Low |
| Resource scope | Low | Medium |
| Prompt caching | Low | Medium |

---

## Recommendations

### Priority 1 (Critical)

1. **Implement MySQL/PostgreSQL support** - Required for production deployment
2. **Switch to token-based Observer trigger** - Align with Mastra specification

### Priority 2 (Important)

3. **Extract current-task and suggested-response** - Improve agent continuity
4. **Implement async buffering** - Better UX, no blocking

### Priority 3 (Enhancement)

5. **Add resource scope** - Cross-conversation memory
6. **Implement prompt caching** - Cost reduction
7. **Consider hybrid OM + RAG** - Unlimited memory ceiling

---

## Implementation Completeness Score

| Category | Completeness | Notes |
|----------|--------------|-------|
| Core OM Logic | 90% | Observer/Reflector working |
| Configuration | 85% | Missing token-based trigger |
| Database Support | 33% | SQLite only |
| Advanced Features | 20% | No async buffering, scopes, caching |
| **Overall** | **~65%** | Functional but not production-ready |

---

## Related Documents

- [`DOCS_OM.md`](./DOCS_OM.md) - Mastra OM specification (source)
- [`DOCS_OM_ANALYSIS.MD`](./DOCS_OM_ANALYSIS.MD) - Original analysis for bchat
- [`DOCS_OM_IMPLEMENTATION.MD`](./DOCS_OM_IMPLEMENTATION.MD) - Implementation plan
- [`DOCS_OM_MINIMAX.MD`](./DOCS_OM_MINIMAX.MD) - Implementation status report
- [`DOCS_OM_MINIMAX_PLAN.MD`](./DOCS_OM_MINIMAX_PLAN.MD) - Enhancement plan
- [`DOCS_ENV_VAR.MD`](./DOCS_ENV_VAR.MD) - Environment variable documentation

---

*Analysis generated: 2026-02-13*
*Model: GLM-5*
