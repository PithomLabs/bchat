# Adversarial Plan Review — plan2.md

**Reviewer:** Senior Go Architect (automated)
**Date:** 2026-07-23
**Scope:** Full adversarial review of plan2.md remediation plan

---

## Verdict: Approved with Nits (2 corrections required)

The plan is **fundamentally sound** after incorporating plan_review.md findings. One factual error must be corrected. Several characterizations are imprecise.

---

## Critical Correction: P0 Issue #1

### Plan Says: "Create `AgentTenantBindingMiddleware`"

### Reality: `TenantBindingMiddleware` Already Exists and Is Already Registered

**File:** `server/router/api/v1/tenant_binding.go:16-72`

The middleware already:
- ✅ Extracts slug from URL (line 39): `slug := c.Param("slug")`
- ✅ Resolves tenant by slug (line 47): `s.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})`
- ✅ Validates RBAC permission (lines 52-67)
- ✅ Bypasses for super users (line 35)
- ❌ Does NOT set resolved tenant in context

**Registration in `v1.go`:**
- Line 324: `authGroup.Use(TenantBindingMiddleware(s.Store))` — applied to all authenticated agent routes
- Line 382: `adminGroup.Use(TenantBindingMiddleware(s.Store))` — applied to all admin agent routes

### Corrected Fix

The fix is to **enhance the existing `TenantBindingMiddleware`** to also call `setTenantInContext(c, tenant.ID)` after resolving the tenant. This is a smaller change than creating a new middleware.

**Additionally:** Since the middleware already resolves the tenant, every agent handler doing `h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})` is **redundant**. The fix should:
1. Add `setTenantInContext(c, tenant.ID)` to `TenantBindingMiddleware`
2. Agent handlers should use `getTenantFromContext(c)` instead of re-resolving by slug
3. Handlers that need the full `AgentTenant` struct can still call `GetAgentTenant`, but the tenant ID is already validated

---

## Nit 1: P0 Issue #2 — Precision on Threshold vs Chunk Size

The plan says "Hardcoded 30K Token Threshold Forces RAG Mode" which is correct but imprecise about what the value controls.

**Actual behavior:**
- `DefaultTokenThreshold = 30000` at `chunker.go:71` — **RAG activation trigger** (decides whether to use RAG mode)
- Per-chunk token limits are separate and configurable via `RAG_MAX_CHUNK_TOKENS` env var
- Provider-specific defaults: openrouter=1024, local=150, mock=500 (lines 90-106)

**Corrected language:** "The RAG activation threshold is hardcoded at 30,000 tokens with no per-tenant override."

---

## Nit 2: P0 Issue #6 — Complexity Characterization

The plan says O(N×M) which is correct for string comparisons, but the database access pattern is different.

**Actual complexity:**
- 1 query: `ListUsers` → returns all N users
- N queries: `GetUserAccessTokens` (one per user)
- O(N×M) string comparisons in inner loop

**Total database queries: N+1** (not N×M)

**Corrected fix:** Add a direct token lookup query (single SQL with WHERE clause on selection token), reducing from N+1 queries to 1 query.

---

## Nit 3: Missing Finding — Agent Handler Redundancy

Since `TenantBindingMiddleware` already resolves the tenant by slug for every request, **79 handler calls** to `h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})` are redundant database lookups.

**Impact:** Each agent request makes 1 extra unnecessary DB query (the tenant lookup that middleware already performed).

**Recommendation:** After enhancing middleware to set context, refactor handlers to use `getTenantFromContext(c)` for the tenant ID. Keep `GetAgentTenant` only where the full `AgentTenant` struct is needed (not just the ID).

---

## Nit 4: Test Plan Correction

**Plan says:** `TestTenantBindingMiddleware_NotUsedByAgentRoutes`

**Reality:** `TenantBindingMiddleware` **IS** used on agent routes (confirmed at `v1.go:324` and `v1.go:382`). This test name is factually incorrect.

**Corrected test:** `TestTenantBindingMiddleware_SetsTenantContext` — verify that after middleware runs, `getTenantFromContext(c)` returns the resolved tenant ID.

---

## Verified Claims (All Correct)

| Issue | Claim | Status |
|-------|-------|--------|
| P0 #2 | Token threshold hardcoded at 30K | ✅ Confirmed at `chunker.go:71` |
| P0 #3 | No mock embedding production guard | ✅ Confirmed at `embedding.go:220-233` |
| P0 #4 | `ApplyAgentTenantFilter` missing | ✅ Confirmed — does not exist |
| P0 #5 | BM25 normalized via `score/(score+1)` | ✅ Confirmed at `vectordb.go:914-917` and `vectordb_lance.go:1273-1281` |
| P0 #6 | O(N×M) token scan | ✅ Confirmed at `auth_service.go:469-499` |
| P1 #12 | REST SignIn sets nil tenant | ✅ Confirmed at `auth_service.go:664` |

---

## Summary Table

| Item | Plan's Claim | Actual | Action |
|------|-------------|--------|--------|
| P0 #1 | Create new middleware | Existing middleware already registered | **CORRECT** — enhance existing, don't create new |
| P0 #2 | Hardcoded 30K threshold | Confirmed | **OK** — clarify it's activation trigger |
| P0 #3 | No mock guard | Confirmed | **OK** |
| P0 #4 | No ApplyAgentTenantFilter | Confirmed | **OK** |
| P0 #5 | Broken BM25 normalization | Confirmed | **OK** |
| P0 #6 | O(N×M) token scan | Confirmed (but N+1 DB queries) | **OK** — fix wording |
| Test | `NotUsedByAgentRoutes` | Middleware IS used on agent routes | **CORRECT** test name |
| Missing | Handler redundancy | 79 redundant DB lookups | **ADD** to plan |

---

## Bottom Line

The plan is production-ready after:
1. Changing "Create `AgentTenantBindingMiddleware`" to "Enhance existing `TenantBindingMiddleware` to set tenant context"
2. Clarifying the 30K threshold is the RAG activation trigger (not chunk limit)
3. Fixing the test name to `TestTenantBindingMiddleware_SetsTenantContext`
4. Adding the handler redundancy finding (79 redundant `GetAgentTenant` calls)
