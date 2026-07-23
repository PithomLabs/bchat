# Bug 047: Comprehensive Adversarial Code Review & Remediation Plan (v2)

**Status:** Planning  
**Severity:** Critical (Multiple P0 findings)  
**Created:** 2026-07-23  
**Updated:** 2026-07-23 (incorporating plan_review.md findings)  
**Assigned:** Senior Go Architect  

---

## Executive Summary

The bchat codebase is a multi-tenant AI chat agent platform with a configuration-driven architecture. While the design principles are sound (GENERAL PURPOSE agent, tenant-agnostic, config-driven), the implementation has **critical security gaps, architectural flaws, and operational risks** that block production deployment.

### Overall Risk Rating: **HIGH 🔴**

| Category | Risk | Blocking Production? |
|----------|------|---------------------|
| Tenant Isolation | 🔴 CRITICAL | **YES** |
| RAG Pipeline | 🔴 CRITICAL | **YES** |
| Error Handling | 🟠 HIGH | Conditional |
| Testing Coverage | 🟠 HIGH | **YES** |
| Build/Deploy | 🟡 MEDIUM | Conditional |
| Observability | 🟡 MEDIUM | No |
| Frontend | 🟡 MEDIUM | No |

---

## P0 - Critical Security & Correctness (Must Fix Before Any Deploy)

### 1. Agent Handlers Lack Middleware-Level Tenant Enforcement
**Files:** `server/router/api/v1/agent/handlers.go`, `server/router/api/v1/v1.go`, `server/router/api/v1/tenant_binding.go`

**Finding (Corrected from plan_review.md):** The REST `/auth/tenants` + `/auth/select-tenant` flow **is fully implemented** and correctly sets JWT `tenant_id`. The `TenantBindingMiddleware` correctly validates slug-to-user RBAC permission. However, **agent handlers bypass this middleware entirely** — they extract tenant from `:slug` and check RBAC directly, never using `getTenantFromContext(c)`.

**Architectural Gap:** Two isolation models that don't intersect:
| Model | Isolation Mechanism | JWT `tenant_id` Role |
|-------|--------------------|---------------------|
| Memo/Ticket | `ApplyTenantFilter` → SQL WHERE clause | **Enforcement point** |
| Agent | Slug → RBAC check in handler | **Informational only** (never checked) |

**Risk:** Handlers duplicate RBAC logic inconsistently. 87 slug-extracting handlers with 0 comparing JWT `tenant_id` against slug-derived tenant. No defense-in-depth if handler logic has bug.

**Required Fix (Middleware-Level, Not Handler-Level):**
1. Create `AgentTenantBindingMiddleware` that:
   - Extracts slug from URL
   - Resolves tenant by slug
   - Validates user has RBAC permission (reuses `TenantBindingMiddleware` logic)
   - Sets resolved tenant ID in context (so handlers can use it)
2. Register this middleware on `/api/v1/agent/:slug/*` routes
3. Keep JWT `tenant_id` as informational for downstream use (memo queries, logging), not as authorization boundary
4. Add `ApplyAgentTenantFilter(ctx, find)` that injects tenant_id from context into any agent Find struct

---

### 2. Hardcoded 30K Token Threshold Forces RAG Mode
**File:** `server/router/api/v1/agent/chunker.go:37-42`

**Finding:** Binary decision at exactly 30,000 tokens with no configurability, no gradual degradation, no model-awareness.

```go
const DefaultTokenThreshold = 30000  // HARDCODED

func ShouldUseRAG(kbContent, policyContent string) bool {
    totalTokens := EstimateTokens(kbContent) + EstimateTokens(policyContent)
    return totalTokens >= DefaultTokenThreshold  // Cliff edge
}
```

**Problems:**
- Content at 29,999 tokens uses long-context; 30,001 forces RAG (quality cliff)
- Different models have different context windows (GPT-4o: 128K, Claude: 200K)
- Token estimation uses `len(content)/4` — wrong for non-English, code, mixed content
- No tenant-level override

**Required Fix:**
- Make threshold configurable per-tenant (tenant_config.retrieval_mode + token_threshold)
- Add gradual transition (e.g., hybrid mode: RAG for KB, full context for policy/script)
- Improve token estimation using actual tokenizer or model-specific ratios

---

### 3. Mock Embeddings Can Reach Production
**File:** `server/router/api/v1/agent/embedding.go:150-162`

**Finding:** No startup validation prevents `EMBEDDING_PROVIDER=mock` with `RAG_PIPELINE_ENABLED=true`.

```go
func NewEmbeddingService(config *EmbeddingConfig) (EmbeddingService, error) {
    switch config.Provider {
    case "openrouter", "openai":
        return NewOpenRouterEmbedding(config)
    case "mock":  // ALLOWED IN PROD
        return NewMockEmbedding(config), nil  // Random vectors = garbage search
    default:
        return NewOpenRouterEmbedding(config)  // Silent fallback to openrouter
    }
}
```

**Impact:** Random 1536-dim vectors produce semantically meaningless search results. Customer-facing chat returns hallucinated/non-grounded responses.

**Required Fix:**
- Startup check: if `RAG_PIPELINE_ENABLED=true` && `EMBEDDING_PROVIDER=mock` && `MODE=prod` → **fail fast with clear error**
- Add `ValidateConfig()` method called during service initialization

---

### 4. Caller-Side Tenant Filter Omission Risk in List* Methods
**File:** `store/db/sqlite/agent.go` — 20 tenant-scoped List methods

**Finding (Corrected from plan_review.md):** All 20 tenant-scoped List methods correctly include `tenant_id` in SQL when `find.TenantID` is set. **The risk is caller-side omission** — there is no enforcement layer ensuring callers set `find.TenantID`. The `ApplyTenantFilter` helper exists only for `FindMemo`, not for agent Find structs.

**Affected Methods:** `ListAgentSessions`, `ListAgentSourceFiles`, `ListAgentIntents`, `ListAgentRules`, `ListAgentFAQs`, `ListAgentCoverage`, `ListAgentServices`, `ListAgentExclusions`, `ListAgentSafetyProtocols`, `ListAgentKBSections`, `ListAgentTenants`, etc.

**Required Fix:**
- Create `ApplyAgentTenantFilter(ctx, find interface{})` that injects tenant_id from context into any agent Find struct
- Use this in `AgentTenantBindingMiddleware` after resolving tenant
- Remove `TenantID` from `Find*` structs where it shouldn't be caller-settable

---

### 5. Hybrid Search Score Normalization Broken
**File:** `server/router/api/v1/agent/vectordb.go:729-732`

**Finding:** BM25 scores normalized with `score/(score+1)` which maps all scores to ~1.0, making BM25 weight dominate regardless of configured weights.

```go
// Current - BROKEN
normalized := score / (score + 1)  // BM25 scores are 0.1-10+ → 0.09-0.91
// Vector cosine is 0-1 → already normalized
// Result: 0.7 * vector + 0.3 * bm25 → but bm25 ~0.9, vector ~0.7 → bm25 wins
```

**Required Fix:**
- Proper score calibration: min-max normalize per query, or use reciprocal rank fusion
- Add unit test verifying weight configuration is respected

---

### 6. O(N×M) Selection Token Scan in HandleSelectTenant (NEW from plan_review.md)
**File:** `server/router/api/v1/auth_service.go:472-499`

**Finding:** `HandleSelectTenant` performs an O(N×M) scan across all users and all their tokens to find the matching selection token. This is a performance bomb at scale.

```go
// Current - O(N×M) SCAN
for _, user := range allUsers {
    tokens, _ := s.Store.GetUserAccessTokens(ctx, user.ID)
    for _, token := range tokens {
        if token.AccessToken == selectionToken { ... }
    }
}
```

**Required Fix:** Add direct token lookup by hash:
- Store selection token hash in `user_access_token.description` or new table
- Add `FindUserBySelectionToken(ctx, tokenHash)` method
- O(1) lookup instead of O(N×M)

---

## P1 - High Severity (Fix in Next Sprint)

### 7. No Circuit Breakers on External Dependencies
**Files:** `embedding.go`, `service.go` (OpenRouter calls), `vectordb_lance.go`

**Finding:** External calls (OpenRouter API, embedding service, LanceDB) have retry loops but no circuit breaker. Cascading failures take down entire service.

**Required Fix:** Add circuit breaker pattern (e.g., `go-breaker` or custom) with:
- Failure threshold (e.g., 5 failures in 10s)
- Half-open state for recovery
- Fallback responses (cached embeddings, degraded mode)

---

### 8. God Objects: service.go (5,482 lines) & handlers.go (6,542 lines)
**Finding:** Single files handling chat, RAG, simulation, analysis, verification, sanitization, learning, bridge, leads, transcripts, QA pairs, role templates, integrations.

**Required Fix:** Split into focused services:
- `ChatService` - core chat flow
- `RAGService` - indexing + retrieval
- `SimulationService` - simulation orchestration
- `AnalysisService` - transcript analysis
- `VerificationService` - LLM verification
- `BridgeService` - human handoff
- `AdminService` - tenant management

---

### 9. Token Estimation Inaccurate for Non-English
**File:** `chunker.go:102-109`

```go
func EstimateTokens(content string) int {
    return len(content) / 4  // English-only assumption
}
```

**Impact:** Chinese/Japanese ~1.5 chars/token → 2.6x underestimation → content exceeds context window → truncation or errors.

**Required Fix:** Use `tiktoken` or model-specific tokenizer; at minimum make ratio configurable per provider.

---

### 10. Startup Race: VectorDB Initialization Before Store Ready
**File:** `server/router/api/v1/agent/service.go:143-157`

```go
vectorDB, err := NewVectorDB(vectorDBConfig)
if pool, ok := vectorDB.(*TenantVectorDBPool); ok {
    pool.SetStore(s)  // s (store) may not be fully initialized
}
```

**Impact:** Reindex on startup fails silently if store not ready.

**Required Fix:** Initialize VectorDB after store is fully ready; add health check.

---

### 11. 47+ Environment Variables With No Validation
**Files:** `service.go`, `embedding.go`, `vectordb.go`, `om_config.go`, `chunker.go`, `v1.go`

**Finding:** Env vars read via `os.Getenv` scattered across codebase. No central validation, no documentation of required vs optional, no type safety.

**Required Fix:** Central config struct with validation at startup:
```go
type Config struct {
    OpenRouterAPIKey     string `env:"OPENROUTER_API_KEY,required"`
    EmbeddingProvider    string `env:"EMBEDDING_PROVIDER,oneof=openrouter local mock"`
    // ...
}
func ValidateConfig() error { ... }
```

---

### 12. REST SignIn Sets nil tenant_id (NEW from plan_review.md)
**File:** `server/router/api/v1/auth_service.go:664`

**Finding:** `HandleSignIn` REST endpoint always sets `tenant_id=nil`. REST-only users get no tenant context without the separate selection flow. This is by design but should be documented as a known behavior, not a bug.

**Required Fix:** Document clearly in AGENTS.md and API docs that REST SignIn requires subsequent `/auth/select-tenant` call for multi-tenant users.

---

## P2 - Medium Severity (Technical Debt)

### 13. Observability Gaps
- Structured logging inconsistent (`slog` vs `log` vs `fmt`)
- No distributed tracing (OpenTelemetry)
- Metrics only for verification; no request latency, error rates, queue depths
- No health check endpoints for dependencies (DB, VectorDB, OpenRouter)

### 14. CGO Dependency Hell (LanceDB)
- Requires native `.so` / `.a` files
- `task setup:lancedb` downloads from GitHub (supply chain risk)
- No version pinning in go.mod for CGO libs
- Cross-compilation broken (linux/amd64 binary won't run on linux/arm64)

### 15. Frontend: MobX Store Duplication & No Type-Safe API
- `agentAdmin.ts`, `agentChat.ts`, `agentSimulation.ts` duplicate CRUD patterns
- No generated TypeScript client from OpenAPI spec
- Translations incomplete (only English in `en.json`)

### 16. Parser Fragility
- `parser.go` rewritten to avoid Go regexp lookahead limitation
- No fuzz tests, no property-based tests
- Edge cases: nested annotations, malformed markdown, unicode, empty sections

---

## P3 - Low Priority (Nice to Have)

### 17. Database Migration Parity
- SQLite has 33 migration versions; Postgres/MySQL stubs return `errNotImplemented`
- `validate:parity` script exists but Postgres tests never run in CI

### 18. Simulation Concurrency
- Only one simulation per tenant at a time (global mutex)
- No queue, no priority, no cancellation propagation to LLM calls

### 19. Widget Security
- Embed script serves with `AllowOrigins: ["*"]`
- No CSP headers, no iframe sandbox attributes
- Widget key validation only on init, not per-message

---

## Remediation Priority Matrix (Updated)

| Priority | Issues | Timeline | Owner |
|----------|--------|----------|-------|
| **P0** | 1-6 (Middleware tenant enforcement, RAG threshold, Mock embeddings, ApplyAgentTenantFilter, Hybrid search, O(N×M) token scan) | **Week 1-2** | Backend Lead |
| **P1** | 7-12 (Circuit breakers, God objects, Token estimation, Startup race, Config validation, REST SignIn nil tenant) | **Week 3-4** | Backend Team |
| **P2** | 13-16 (Observability, CGO, Frontend, Parser) | **Week 5-8** | Full Team |
| **P3** | 17-19 (Migration parity, Simulation queue, Widget security) | **Backlog** | As capacity |

---

## Proposed Test Plan (tests/001)

### 1. Tenant Isolation Tests (`tenant_isolation_test.go`)
```go
func TestAgentMiddleware_ResolvesSlugToTenant(t *testing.T)
func TestAgentMiddleware_ValidatesRBAC(t *testing.T)
func TestAgentMiddleware_SetsTenantInContext(t *testing.T)
func TestApplyAgentTenantFilter_InjectsTenantID(t *testing.T)
func TestAgentMiddleware_AllowsScopedAdmin(t *testing.T)
func TestAgentMiddleware_DenylistsSuperuserBypass(t *testing.T)
func TestTenantBindingMiddleware_NotUsedByAgentRoutes(t *testing.T)  // Verify separation
```

### 2. RAG Pipeline Tests (`rag_pipeline_test.go`)
```go
func TestRAG_ThresholdConfigurable_PerTenant(t *testing.T)
func TestRAG_MockEmbeddingRejectedInProdMode(t *testing.T)
func TestRAG_HybridSearch_WeightConfigRespected(t *testing.T)
func TestRAG_TokenEstimation_NonEnglishContent(t *testing.T)
func TestRAG_ReindexIdempotent(t *testing.T)
func TestRAG_CircuitBreaker_EmbeddingServiceDown(t *testing.T)
```

### 3. Parser Fuzz Tests (`parser_fuzz_test.go`)
```go
func FuzzParseKB(f *testing.F)
func FuzzParsePolicy(f *testing.F)
func FuzzParseScript(f *testing.F)
func TestParser_Annotations_BoundaryDetection(t *testing.T)
func TestParser_UnicodeSupport(t *testing.T)
func TestParser_MalformedMarkdown_NoPanic(t *testing.T)
```

### 4. API Contract Tests (`api_contract_test.go`)
```go
// Verify all endpoints in DOCS_README.MD API Reference
func TestAPI_PublicChatEndpoints(t *testing.T)
func TestAPI_AdminEndpoints_Permissions(t *testing.T)
func TestAPI_AuthFlows_MultiTenant(t *testing.T)
func TestAPI_ErrorResponseFormat_Consistent(t *testing.T)
func TestAPI_RateLimiting_PerTenantPerAudience(t *testing.T)
func TestAPI_AuthSignIn_ReturnsNilTenantID(t *testing.T)  // Documented behavior
func TestAPI_AuthSelectTenant_O1Lookup(t *testing.T)     // Performance test
```

### 5. Chaos/Load Tests (`chaos_test.go`)
```go
func TestChaos_EmbeddingServiceLatency(t *testing.T)
func TestChaos_VectorDBMemoryPressure(t *testing.T)
func TestChaos_ConcurrentReindexAndChat(t *testing.T)
func TestChaos_SSEStreamCancellation(t *testing.T)
func TestChaos_OpenRouterRateLimit(t *testing.T)
func TestChaos_SelectionTokenLookup_NoO(N×M)(t *testing.T)
```

---

## Definition of Done for P0

- [ ] `AgentTenantBindingMiddleware` created and registered on all `/api/v1/agent/:slug/*` routes
- [ ] `ApplyAgentTenantFilter` implemented and used in middleware
- [ ] All 6 P0 issues fixed with tests
- [ ] `go test ./...` passes with `-race` flag
- [ ] No `os.Getenv` in hot paths; central config validated at startup
- [ ] RAG threshold configurable per tenant; no hardcoded constants
- [ ] Mock embedding rejected in production mode with clear error
- [ ] Hybrid search weights verified by unit test
- [ ] Selection token lookup O(1) — add benchmark test
- [ ] REST SignIn nil tenant behavior documented

---

## Questions for Clarification

1. **Production deployment target:** fly.io with SQLite or PostgreSQL? LanceDB S3 or local? This affects CGO and migration priorities.

2. **Team capacity:** How many engineers can work on P0 fixes in parallel? Determines if we split god objects now or after P0.

3. **Observability stack:** Prometheus/Grafana? Datadog? Custom? Affects P2 implementation.

4. **Frontend priority:** Is the React admin UI in scope for this review cycle, or backend-only?

---

## Appendix: Files Modified in This Plan

- `bugs/047/plan.md` — Original plan (v1)
- `bugs/047/plan_review.md` — Automated review of Q1
- `bugs/047/plan2.md` — This document (v2, incorporating review findings)
- No code changes yet — awaiting approval to proceed

---

**Next Steps:** Review this updated plan, clarify questions above, then assign P0 tasks to begin implementation.