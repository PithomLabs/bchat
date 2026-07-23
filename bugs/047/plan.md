# Bug 047: Comprehensive Adversarial Code Review & Remediation Plan

**Status:** Planning  
**Severity:** Critical (Multiple P0 findings)  
**Created:** 2026-07-23  
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

### 1. Tenant Isolation Bypass - Cross-Tenant Data Access
**Files:** `server/router/api/v1/agent/handlers.go`, `server/router/api/v1/v1.go`, `store/db/sqlite/agent.go`

**Finding:** Multiple handlers extract tenant from URL parameter (`:slug`) instead of JWT context. JWT `tenant_id` claim is set but never validated against user's allowed tenants.

**Attack Vector:** User with `tenant_id=1` in JWT accesses `tenant_id=2` by changing `:slug` parameter.

**Evidence:**
```go
// handlers.go - VULNERABLE PATTERN (multiple locations)
func (h *Handler) HandleChatExternal(c echo.Context) error {
    slug := c.Param("slug")  // USER-CONTROLLED
    tenant, _ := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
    // NO VALIDATION: tenant.ID vs JWT tenant_id
}
```

**Required Fix:**
- Every handler must extract `tenantID := getTenantFromContext(c)` (from JWT)
- Validate `tenant.ID == tenantID` with superuser bypass
- Apply `ApplyTenantFilter` to ALL List* operations in store layer

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

### 4. Missing Tenant Filter in List Operations (Data Leak)
**File:** `store/db/sqlite/agent.go` — 11 of 23 `List*` methods missing `ApplyTenantFilter`

**Finding:** Callers expected to set `find.TenantID` but no enforcement. Superuser check only in some handlers.

```go
// MISSING FILTER - returns ALL tenants' sessions
func (d *DB) ListAgentSessions(ctx context.Context, find *store.FindAgentSession) ([]*store.AgentSession, error) {
    // Should call: ApplyTenantFilter(ctx, find) but doesn't
}
```

**Affected Methods (sample):** `ListAgentSessions`, `ListAgentSourceFiles`, `ListAgentIntents`, `ListAgentRules`, `ListAgentFAQs`, `ListAgentCoverage`, `ListAgentServices`, `ListAgentExclusions`, `ListAgentSafetyProtocols`, `ListAgentKBSections`, `ListAgentTenants`

**Required Fix:**
- Add `ApplyTenantFilter(ctx, find)` to ALL List* methods in sqlite driver
- Add same to postgres/mysql stub drivers
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

## P1 - High Severity (Fix in Next Sprint)

### 6. No Circuit Breakers on External Dependencies
**Files:** `embedding.go`, `service.go` (OpenRouter calls), `vectordb_lance.go`

**Finding:** External calls (OpenRouter API, embedding service, LanceDB) have retry loops but no circuit breaker. Cascading failures take down entire service.

**Required Fix:** Add circuit breaker pattern (e.g., `go-breaker` or custom) with:
- Failure threshold (e.g., 5 failures in 10s)
- Half-open state for recovery
- Fallback responses (cached embeddings, degraded mode)

---

### 7. God Objects: service.go (5,482 lines) & handlers.go (6,542 lines)
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

### 8. Token Estimation Inaccurate for Non-English
**File:** `chunker.go:102-109`

```go
func EstimateTokens(content string) int {
    return len(content) / 4  // English-only assumption
}
```

**Impact:** Chinese/Japanese ~1.5 chars/token → 2.6x underestimation → content exceeds context window → truncation or errors.

**Required Fix:** Use `tiktoken` or model-specific tokenizer; at minimum make ratio configurable per provider.

---

### 9. Startup Race: VectorDB Initialization Before Store Ready
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

### 10. 47+ Environment Variables With No Validation
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

## P2 - Medium Severity (Technical Debt)

### 11. Observability Gaps
- Structured logging inconsistent (`slog` vs `log` vs `fmt`)
- No distributed tracing (OpenTelemetry)
- Metrics only for verification; no request latency, error rates, queue depths
- No health check endpoints for dependencies (DB, VectorDB, OpenRouter)

### 12. CGO Dependency Hell (LanceDB)
- Requires native `.so` / `.a` files
- `task setup:lancedb` downloads from GitHub (supply chain risk)
- No version pinning in go.mod for CGO libs
- Cross-compilation broken (linux/amd64 binary won't run on linux/arm64)

### 13. Frontend: MobX Store Duplication & No Type-Safe API
- `agentAdmin.ts`, `agentChat.ts`, `agentSimulation.ts` duplicate CRUD patterns
- No generated TypeScript client from OpenAPI spec
- Translations incomplete (only English in `en.json`)

### 14. Parser Fragility
- `parser.go` rewritten to avoid Go regexp lookahead limitation
- No fuzz tests, no property-based tests
- Edge cases: nested annotations, malformed markdown, unicode, empty sections

---

## P3 - Low Priority (Nice to Have)

### 15. Database Migration Parity
- SQLite has 33 migration versions; Postgres/MySQL stubs return `errNotImplemented`
- `validate:parity` script exists but Postgres tests never run in CI

### 16. Simulation Concurrency
- Only one simulation per tenant at a time (global mutex)
- No queue, no priority, no cancellation propagation to LLM calls

### 17. Widget Security
- Embed script serves with `AllowOrigins: ["*"]`
- No CSP headers, no iframe sandbox attributes
- Widget key validation only on init, not per-message

---

## Remediation Priority Matrix

| Priority | Issues | Timeline | Owner |
|----------|--------|----------|-------|
| **P0** | 1-5 (Tenant isolation, RAG threshold, Mock embeddings, List filters, Hybrid search) | **Week 1-2** | Backend Lead |
| **P1** | 6-10 (Circuit breakers, God objects, Token estimation, Startup race, Config validation) | **Week 3-4** | Backend Team |
| **P2** | 11-14 (Observability, CGO, Frontend, Parser) | **Week 5-8** | Full Team |
| **P3** | 15-17 (Migration parity, Simulation queue, Widget security) | **Backlog** | As capacity |

---

## Proposed Test Plan (tests/001)

### 1. Tenant Isolation Tests (`tenant_isolation_test.go`)
```go
func TestCrossTenantAccessDenied_ChatExternal(t *testing.T)
func TestCrossTenantAccessDenied_ChatInternal(t *testing.T)
func TestCrossTenantAccessDenied_ListOperations(t *testing.T)
func TestJWTTenantIDValidation_NotInAllowedTenants(t *testing.T)
func TestSuperuserBypass_ExplicitCheckRequired(t *testing.T)
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
```

### 5. Chaos/Load Tests (`chaos_test.go`)
```go
func TestChaos_EmbeddingServiceLatency(t *testing.T)
func TestChaos_VectorDBMemoryPressure(t *testing.T)
func TestChaos_ConcurrentReindexAndChat(t *testing.T)
func TestChaos_SSEStreamCancellation(t *testing.T)
func TestChaos_OpenRouterRateLimit(t *testing.T)
```

---

## Definition of Done for P0

- [ ] All 5 P0 issues fixed with tests
- [ ] `go test ./...` passes with `-race` flag
- [ ] No `os.Getenv` in hot paths; central config validated at startup
- [ ] Tenant isolation verified by integration test against running server
- [ ] RAG threshold configurable per tenant; no hardcoded constants
- [ ] Mock embedding rejected in production mode with clear error
- [ ] All List* operations apply tenant filter automatically
- [ ] Hybrid search weights verified by unit test

---

## Questions for Clarification

1. **Multi-tenant auth flow:** The REST `/auth/tenants` + `/auth/select-tenant` flow — is this fully implemented and tested? The review found JWT `tenant_id` set but not validated against `allowed_tenant_ids`.

2. **Production deployment target:** fly.io with SQLite or PostgreSQL? LanceDB S3 or local? This affects CGO and migration priorities.

3. **Team capacity:** How many engineers can work on P0 fixes in parallel? Determines if we split god objects now or after P0.

4. **Observability stack:** Prometheus/Grafana? Datadog? Custom? Affects P2 implementation.

5. **Frontend priority:** Is the React admin UI in scope for this review cycle, or backend-only?

---

## Appendix: Files Modified in This Plan

- `bugs/047/plan.md` — This document
- No code changes yet — awaiting approval to proceed

---

**Next Steps:** Review this plan, clarify questions above, then assign P0 tasks to begin implementation.