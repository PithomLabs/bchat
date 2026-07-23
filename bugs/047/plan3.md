# Bug 047: Comprehensive Adversarial Code Review & Remediation Plan (v3)

**Status:** Planning  
**Severity:** Critical (Multiple P0 findings)  
**Created:** 2026-07-23  
**Updated:** 2026-07-23 (incorporating plan2_review.md findings)  
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

### 1. Enhance Existing `TenantBindingMiddleware` to Set Tenant Context (NOT Create New Middleware)

**Correction from plan2_review.md:** The plan originally said "Create `AgentTenantBindingMiddleware`" — but `TenantBindingMiddleware` **already exists and is already registered** on all agent routes.

**File:** `server/router/api/v1/tenant_binding.go:16-72`  
**Registration:** `v1.go:324` (authGroup) and `v1.go:382` (adminGroup)

**Current State:**
- ✅ Extracts slug from URL (line 39)
- ✅ Resolves tenant by slug (line 47)
- ✅ Validates RBAC permission (lines 52-67)
- ✅ Bypasses for super users (line 35)
- ❌ Does **NOT** set resolved tenant in context

**Corrected Fix:**
```go
// In TenantBindingMiddleware, after successful RBAC check (around line 67):
setTenantInContext(c, tenant.ID)  // ADD THIS LINE
```

**Additionally:** 79 handler calls to `h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})` are **redundant** — the middleware already resolved the tenant. After the fix:
1. Handlers should use `getTenantFromContext(c)` for tenant ID
2. Keep `GetAgentTenant` only where full `AgentTenant` struct is needed

---

### 2. Hardcoded RAG Activation Threshold (30,000 Tokens)

**File:** `server/router/api/v1/agent/chunker.go:71`

```go
const DefaultTokenThreshold = 30000  // RAG ACTIVATION TRIGGER
```

**Clarification from review:** This is the **RAG activation trigger** (decides whether to use RAG mode), NOT the per-chunk token limit. Per-chunk limits are separate and configurable via `RAG_MAX_CHUNK_TOKENS` env var with provider-specific defaults (openrouter=1024, local=150, mock=500).

**Problems:**
- No per-tenant override
- Binary cliff: 29,999 tokens = long-context mode, 30,001 = RAG mode
- Token estimation uses `len(content)/4` (English-only assumption)

**Required Fix:**
- Add `retrieval_token_threshold` to `tenant_config` table
- Fallback chain: tenant config → env var → hardcoded default
- Improve token estimation (at minimum make ratio configurable)

---

### 3. Mock Embeddings Can Reach Production (No Startup Guard)

**File:** `server/router/api/v1/agent/embedding.go:220-233`

```go
case "mock":
    return NewMockEmbedding(config), nil  // ALLOWED IN PROD — NO GUARD
```

**Impact:** Random 1536-dim vectors → semantically meaningless search → hallucinated responses.

**Required Fix:** Startup validation in `NewService()`:
```go
if svc.profile.Mode == "prod" && svc.vectorDBConfig.Enabled && svc.embeddingConfig.Provider == "mock" {
    return nil, fmt.Errorf("mock embedding provider not allowed in production mode")
}
```

---

### 4. `ApplyAgentTenantFilter` Missing (Caller-Side Enforcement Gap)

**Finding from review:** All 20 tenant-scoped `List*` methods in sqlite driver correctly include `tenant_id` in SQL **when `find.TenantID` is set**. The risk is **caller omission** — no enforcement layer.

**Required Fix:** Create `ApplyAgentTenantFilter(ctx context.Context, find interface{})` that injects tenant ID from context into any agent `Find*` struct. Call from enhanced `TenantBindingMiddleware`.

---

### 5. Hybrid Search BM25 Normalization Broken

**Files:** `vectordb.go:914-917` and `vectordb_lance.go:1273-1281`

```go
// Current - BROKEN
normalized := score / (score + 1)  // BM25 scores 0.1-10+ → 0.09-0.91
// Vector cosine 0-1 → already normalized
// Result: BM25 dominates regardless of configured weights
```

**Required Fix:** Proper score calibration — min-max normalize per query, or use reciprocal rank fusion (RRF). Add unit test verifying configured weights are respected.

---

### 6. Selection Token Lookup O(N×M) String Comparisons + N+1 DB Queries

**File:** `auth_service.go:469-499`

**Actual Complexity (per review):**
- 1 query: `ListUsers` → all N users
- N queries: `GetUserAccessTokens` per user
- O(N×M) string comparisons in memory

**Total DB queries: N+1** (not N×M queries — the review corrects this)

**Required Fix:** Add direct token lookup query (single SQL with WHERE on selection token hash), reducing to 1 query.

---

### 7. REST SignIn Sets nil Tenant (Undocumented Behavior)

**File:** `auth_service.go:664`

```go
token, err := s.generateAuthToken(user, nil)  // tenant_id = nil ALWAYS
```

**Impact:** REST-only users get no tenant context without separate `/auth/select-tenant` flow. This is by design but **undocumented**.

**Required Fix:** Document clearly in API docs and CLAUDE.md.

---

## P1 - High Severity (Fix in Next Sprint)

### 8. No Circuit Breakers on External Dependencies
**Files:** `embedding.go`, `service.go` (OpenRouter), `vectordb_lance.go`

### 9. God Objects: `service.go` (5,482 lines) & `handlers.go` (6,542 lines)
**Required Fix:** Split into focused services (ChatService, RAGService, SimulationService, etc.)

### 10. Token Estimation Inaccurate for Non-English
**File:** `chunker.go:102-109` — `len(content)/4` fails for CJK (~1.5 chars/token), code (~3 chars/token)

### 11. Startup Race: VectorDB Initialized Before Store Ready
**File:** `service.go:143-157` — `pool.SetStore(s)` called before store fully initialized

### 12. 47+ Env Vars With No Central Validation
**Required Fix:** Central config struct with validation at startup

---

## P2 - Medium Severity (Technical Debt)

### 13. Observability Gaps
- Inconsistent structured logging
- No distributed tracing (OpenTelemetry)
- Metrics only for verification; no RED metrics

### 14. CGO Dependency Hell (LanceDB)
- Native `.so`/`.a` files, no vendoring, supply chain risk
- Cross-compilation broken

### 15. Frontend: MobX Store Duplication & No Type-Safe API
- `agentAdmin.ts`, `agentChat.ts`, `agentSimulation.ts` duplicate CRUD patterns
- No generated TypeScript client from OpenAPI spec

### 16. Parser Fragility
- No fuzz/property-based tests
- Edge cases: nested annotations, malformed markdown, unicode, empty sections

---

## P3 - Low Priority (Nice to Have)

### 17. Database Migration Parity (PostgreSQL/MySQL stubs return `errNotImplemented`)
### 18. Simulation Concurrency (global mutex, no queue/priority)
### 19. Widget Security (CSP headers, iframe sandbox, per-message key validation)

---

## Remediation Priority Matrix

| Priority | Issues | Timeline | Owner |
|----------|--------|----------|-------|
| **P0** | 1-7 (Tenant middleware, RAG threshold, Mock guard, AgentTenantFilter, Hybrid search, Token lookup, REST SignIn docs) | **Week 1-2** | Backend Lead |
| **P1** | 8-12 (Circuit breakers, God objects, Token estimation, Startup race, Config validation) | **Week 3-4** | Backend Team |
| **P2** | 13-16 (Observability, CGO, Frontend, Parser) | **Week 5-8** | Full Team |
| **P3** | 17-19 (Migration parity, Simulation queue, Widget security) | **Backlog** | As capacity |

---

## Proposed Test Plan (tests/001/)

### 1. Tenant Isolation Tests (`tenant_isolation_test.go`)
```go
func TestTenantBindingMiddleware_SetsTenantContext(t *testing.T)
func TestTenantBindingMiddleware_RBACValidation(t *testing.T)
func TestTenantBindingMiddleware_SuperUserBypass(t *testing.T)
func TestCrossTenantAccessDenied_ChatExternal(t *testing.T)
func TestCrossTenantAccessDenied_ChatInternal(t *testing.T)
func TestCrossTenantAccessDenied_ListOperations(t *testing.T)
func TestHandlerUsesContextNotSlug_ChatExternal(t *testing.T)
func TestHandlerUsesContextNotSlug_ChatInternal(t *testing.T)
func TestApplyAgentTenantFilter_InjectsTenantID(t *testing.T)
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
func TestAPI_AuthSignIn_ReturnsNilTenantID(t *testing.T)
func TestAPI_AuthSelectTenant_O1Lookup(t *testing.T)
```

### 5. Chaos/Load Tests (`chaos_test.go`)
```go
func TestChaos_EmbeddingServiceLatency(t *testing.T)
func TestChaos_VectorDBMemoryPressure(t *testing.T)
func TestChaos_ConcurrentReindexAndChat(t *testing.T)
func TestChaos_SSEStreamCancellation(t *testing.T)
func TestChaos_OpenRouterRateLimit(t *testing.T)
func TestChaos_SelectionTokenLookup_NoNPlus1Queries(t *testing.T)
```

---

## Definition of Done for P0

- [ ] `TenantBindingMiddleware` enhanced with `setTenantInContext` 
- [ ] All agent handlers use `getTenantFromContext(c)` instead of re-resolving by slug (79 redundant calls eliminated)
- [ ] `ApplyAgentTenantFilter` implemented and called from middleware
- [ ] RAG activation threshold configurable per tenant (`tenant_config.retrieval_token_threshold`)
- [ ] Mock embedding rejected in production mode with clear error
- [ ] Hybrid search weights verified by unit test (BM25/vector weight config respected)
- [ ] Selection token lookup uses direct query (1 query, not N+1)
- [ ] REST SignIn nil tenant behavior documented in API docs and CLAUDE.md
- [ ] `go test ./...` passes with `-race` flag
- [ ] No `os.Getenv` in hot paths; central config validated at startup

---

## Questions for Clarification

1. **Production deployment target:** fly.io with SQLite or PostgreSQL? LanceDB S3 or local? This affects CGO and migration priorities.

2. **Team capacity:** How many engineers can work on P0 fixes in parallel? Determines if we split god objects now or after P0.

3. **Observability stack:** Prometheus/Grafana? Datadog? Custom? Affects P2 implementation.

4. **Frontend priority:** Is the React admin UI in scope for this review cycle, or backend-only?

---

## Appendix: Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| plan.md | 2026-07-23 | Senior Go Architect | Initial adversarial review |
| plan2.md | 2026-07-23 | Senior Go Architect | Incorporated plan_review.md (Q1 on auth flow) |
| **plan3.md** | **2026-07-23** | **Senior Go Architect** | **Incorporated plan2_review.md (4 nits + 1 missing finding)** |

**Key Corrections in v3:**
- P0 #1: Enhanced existing `TenantBindingMiddleware` instead of creating new middleware
- P0 #2: Clarified 30K is RAG activation trigger, not chunk limit
- P0 #6: Corrected N+1 DB queries (not N×M queries), though string comparisons are O(N×M)
- Test: Fixed `TestTenantBindingMiddleware_NotUsedByAgentRoutes` → `TestTenantBindingMiddleware_SetsTenantContext`
- Added: 79 redundant `GetAgentTenant` calls in handlers (middleware already resolves tenant)

---

**No code changes yet — awaiting approval to proceed with P0 implementation.**