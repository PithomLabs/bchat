# Adversarial Test Plan Review: Plan 3 — Ask Rovo E2E Test (Final Revised)

**Bug/Task:** Plan for `plan3_test.md`  
**Reviewer:** Kilo (Senior Go Architect)  
**Date:** 2026-08-01  
**Verdict:** APPROVED WITH NITS — plan fixes all critical bugs from Plan 2 and is nearly implementation-ready. One medium-severity flakiness issue in the TopK test and two minor nits remain.

---

## Executive Summary

Plan 3 correctly addresses all 10 findings from the Plan 2 review. The `CreateMemo` UID issue is fixed with a `createTestMemo` helper, the concurrent `require.NoError` data race is fixed with thread-safe error collection, environment isolation is added to `setupAskRovoTest`, and a TopK limiting test is added. The documented limitations of the `controlledEmbeddingService` are accurate and acceptable for a hackathon deliverable.

However, the new TopK test (Test 11) has a **flakiness bug**: `controlledEmbeddingService` produces identical HIGH vectors for all keyword-matching texts, and Go's `sort.Slice` is not stable for equal scores. The selection of which 3 of 5 seeded chunks appear in the result is non-deterministic, causing intermittent test failures. Additionally, `setupAskRovoTest` does not guard against `TICKET_EMBEDDING_ENABLED=true` side effects.

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | TopK test is flaky — non-deterministic selection of 3 from 5 equal-score chunks | MEDIUM | Fix |
| 2 | Missing `TICKET_EMBEDDING_ENABLED` env isolation | MEDIUM | Fix |
| 3 | `createTestMemo` uses `context.Background()` instead of test `ctx` | LOW | Fix |
| 4 | `atomic.AddInt32` return value not captured — fragile if `t.Parallel()` added | LOW | Document |
| 5 | No explicit prohibition of `t.Parallel()` in test file | LOW | Document |

---

## Finding 1: TopK Test Is Flaky — Non-Deterministic Selection

**Severity:** MEDIUM

Test 11 (`TestAskRovo_TopKLimiting`) seeds 5 ticket chunks, all containing keywords, so all receive HIGH vectors from `controlledEmbeddingService`. The query also receives a HIGH vector. All 5 chunks have cosine similarity = 1.0.

`InferResolutionForNewTicket` calls `vectorDB.Search` with `TopK: 3`. Inside `MemoryVectorDB.Search`:

```go
// Sort by score (descending)
sort.Slice(scored, func(i, j int) bool {
    return scored[i].score > scored[j].score
})
```

Go's `sort.Slice` is **not guaranteed to be stable** for equal elements. When all 5 chunks have identical scores (1.0), the sort order is non-deterministic. The subsequent truncation:

```go
if len(scored) > topK {
    scored = scored[:topK]
}
```

selects the **first 3 elements** of the non-deterministically sorted slice. Which 3 chunks are returned depends on the internal iteration order of `db.chunks` (a Go map, which has randomized iteration since Go 1.12) and the sort algorithm's behavior for equal elements.

The test asserts:
```go
require.Contains(t, result, "Ticket 0")
require.Contains(t, result, "Ticket 1")
require.Contains(t, result, "Ticket 2")
require.NotContains(t, result, "Ticket 3")
require.NotContains(t, result, "Ticket 4")
```

If the sort happens to place `Ticket 3` or `Ticket 4` in the top 3, the test fails intermittently. This is a **race condition in the test logic**, not in the production code.

**Required fix:** Replace the fragile title-based assertions with a deterministic count check:

```go
result := svc.InferResolutionForNewTicket(ctx, ticket)
require.NotEmpty(t, result)

// TopK=3, so exactly 3 chunks should be returned
// Count markdown headers (each chunk gets "### Title" in the output)
chunkCount := strings.Count(result, "### ")
require.Equal(t, 3, chunkCount, "TopK=3 should limit results to 3 chunks")
```

If you need to verify that specific chunks are included, add a separate test with a mock that produces distinguishable scores, or reduce the seed count to exactly 3 and assert all are present.

Alternatively, for a stronger integration test, create a **custom embedding service for this test only** that produces vectors with deterministic, distinguishable scores:

```go
type topKEmbeddingService struct {
    dimension int
}

func (e *topKEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    result := make([][]float32, len(texts))
    for i, text := range texts {
        v := make([]float32, e.dimension)
        v[0] = 1.0
        // Add tiny deterministic perturbation so no two texts have identical vectors
        v[1] = float32(i) * 0.001
        // Normalize to unit vector
        norm := float32(0.0)
        for j := 0; j < e.dimension; j++ {
            norm += v[j] * v[j]
        }
        norm = float32(math.Sqrt(float64(norm)))
        for j := 0; j < e.dimension; j++ {
            v[j] /= norm
        }
        result[i] = v
    }
    return result, nil
}
```

This produces cosine similarities like 1.0, 0.999, 0.998, etc., which are all above `MinScore: 0.7` but allow deterministic sorting.

**Recommendation:** Use the count-based assertion (`strings.Count(result, "### ")`) for simplicity. The test still proves TopK limiting works because without TopK, all 5 chunks would appear.

---

## Finding 2: Missing `TICKET_EMBEDDING_ENABLED` Env Isolation

**Severity:** MEDIUM

`NewService` at `service.go:196` starts a background goroutine if `TICKET_EMBEDDING_ENABLED=true`:

```go
if os.Getenv("TICKET_EMBEDDING_ENABLED") == "true" {
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        // ...
    }()
}
```

If the test environment has this variable set, the goroutine runs for the lifetime of the test process. It may:
- Access `svc.vectorDB` concurrently with test operations
- Create unexpected indexing activity
- Interfere with other tests if the service is reused

The plan's `setupAskRovoTest` sets:
```go
t.Setenv("RAG_PIPELINE_ENABLED", "false")
t.Setenv("LANCEDB_STORAGE_PROVIDER", "memory")
```

But does **not** set `TICKET_EMBEDDING_ENABLED=false`.

**Required fix:** Add to `setupAskRovoTest`:
```go
t.Setenv("TICKET_EMBEDDING_ENABLED", "false")
```

Audit other env vars that `NewService` reads and could cause side effects:
- `ENCRYPTION_MASTER_KEY` — already safe (empty string skips encryption)
- `ENCRYPTION_MASTER_KEY_BACKUP` — only read if encryption is enabled
- `COCKROACH_DSN` — safe because `RAG_PIPELINE_ENABLED=false` prevents CockroachDB init

So `TICKET_EMBEDDING_ENABLED` is the only missing guard.

---

## Finding 3: `createTestMemo` Uses `context.Background()` Instead of Test `ctx`

**Severity:** LOW

The helper:
```go
func createTestMemo(t *testing.T, ts *store.Store, user *store.User, tenant *store.AgentTenant, content string) *store.Memo {
    t.Helper()
    n := atomic.AddInt32(&memoUIDCounter, 1)
    memo, err := ts.CreateMemo(context.Background(), &store.Memo{...})
```

Ignores the `ctx` returned by `setupAskRovoTest` and uses `context.Background()` directly. Since `setupAskRovoTest` sets `ctx = context.Background()`, this is functionally equivalent. But it's inconsistent and would break if `setupAskRovoTest` ever used a derived context (e.g., with timeout or cancellation).

**Required fix:** Add `ctx context.Context` as the first parameter:

```go
func createTestMemo(t *testing.T, ctx context.Context, ts *store.Store, user *store.User, tenant *store.AgentTenant, content string) *store.Memo {
    t.Helper()
    n := atomic.AddInt32(&memoUIDCounter, 1)
    memo, err := ts.CreateMemo(ctx, &store.Memo{...})
```

---

## Finding 4: `atomic.AddInt32` Return Value Not Captured

**Severity:** LOW

```go
atomic.AddInt32(&testTenantIDCounter, 1)
tenant, err = ts.CreateAgentTenant(ctx, &store.AgentTenant{
    Slug: fmt.Sprintf("ask-rovo-%d", testTenantIDCounter),
```

`atomic.AddInt32` returns the new value, but the plan reads the variable directly afterward. This is safe under sequential execution, but fragile if `t.Parallel()` is ever added.

**Required fix:** Capture the return value:

```go
slugCounter := atomic.AddInt32(&testTenantIDCounter, 1)
tenant, err = ts.CreateAgentTenant(ctx, &store.AgentTenant{
    Slug: fmt.Sprintf("ask-rovo-%d", slugCounter),
```

Same for `memoUIDCounter` in `createTestMemo`.

---

## Finding 5: No Explicit Prohibition of `t.Parallel()`

**Severity:** LOW

The plan relies on sequential test execution to prevent `ticketIndexMu` and `memoUIDCounter` collisions. If a future contributor adds `t.Parallel()` to any test, collisions will occur.

**Required fix:** Add a comment at the top of the test file:

```go
// WARNING: Tests in this file must NOT use t.Parallel() because
// ticketIndexMu and memoUIDCounter are package-level shared state
// that rely on sequential execution for uniqueness.
```

---

## What Is Correct

| Check | Status | Notes |
|-------|--------|-------|
| `CreateMemo` UID pattern | CORRECT | `test-memo-<tenantID>-<counter>` matches `UIDMatcher` regex |
| Concurrent error collection in Test 9 | CORRECT | Thread-safe `sync.Mutex` + slice, asserts after `wg.Wait()` |
| Environment isolation | CORRECT | `t.Setenv` for `RAG_PIPELINE_ENABLED` and `LANCEDB_STORAGE_PROVIDER` |
| `createTestMemo` helper | CORRECT | Generates unique UIDs per tenant via atomic counter |
| Test 1 (happy path) | CORRECT | Seeds, indexes, infers, asserts InternalNotes |
| Test 2 (bug sections) | CORRECT | Full executable code, verifies bug section search |
| Test 3 (no results) | CORRECT | Empty vector DB → empty result |
| Test 4 (tenant isolation) | CORRECT | Cross-tenant seeds excluded by TenantID filter |
| Test 5 (dedup unchanged) | CORRECT | Two index calls, one version |
| Test 6 (content changed) | CORRECT | Two index calls with different content, two versions |
| Test 7 (comments) | CORRECT | Comments appear in indexed content blob |
| Test 8 (nil tenant) | CORRECT | No panic, returns empty |
| Test 9 (concurrent) | CORRECT | Thread-safe error collection, no `testing.T` race |
| Test 10 (MinScore) | CORRECT | Non-matching query gets LOW vector, below threshold |
| `setupAskRovoTest` helper | CORRECT | Unique slugs, env isolation, clean teardown |
| Struct references | CORRECT | All point to `store/ticket.go` |
| `controlledEmbeddingService` documentation | CORRECT | Binary vector limitation clearly documented |
| Goroutine gap documentation | CORRECT | Documented in Risk Assessment |

---

## Behavioral Correctness Check

### Test 1: Happy Path
| Step | Expected | Actual |
|------|----------|--------|
| Seed 2 ticket chunks | Inserted with HIGH vectors | CORRECT |
| Create ticket | `/m/` + memo UID | CORRECT |
| `IndexTicketContent` | `chunks > 0`, `inferred == true` | CORRECT |
| `InternalNotes` | Contains "Suggested Resolution" + seed titles | CORRECT |

### Test 2: Bug Sections
| Step | Expected | Actual |
|------|----------|--------|
| Seed 1 bug_section chunk | Inserted with HIGH vector | CORRECT |
| Query with keywords | HIGH vector | CORRECT |
| Search 1 (tickets) | No results (no ticket seeds) | CORRECT |
| Search 2 (bug_sections) | 1 result, score=1.0 ≥ 0.5 | CORRECT |
| `InternalNotes` | Contains "Relevant Bug History" | CORRECT |

### Test 9: Concurrent Indexing
| Step | Expected | Actual |
|------|----------|--------|
| 5 goroutines call `IndexTicketContent` | Serialized by `ticketIndexMu` | CORRECT |
| Error collection | Thread-safe via `sync.Mutex` | CORRECT |
| Final version count | 1 (no duplicates) | CORRECT |
| No `testing.T` race | Assertions on main goroutine only | CORRECT |

### Test 11: TopK Limiting (FLaky)
| Step | Expected | Actual |
|------|----------|--------|
| Seed 5 ticket chunks | All get HIGH vectors, cosine=1.0 | CORRECT |
| Search with TopK=3 | Should return exactly 3 | **FLAKY** — non-deterministic sort for equal scores |

---

## Recommended Rework

### 1. Fix TopK Test Flakiness (Finding 1)

Replace title-based assertions with count-based assertion:

```go
result := svc.InferResolutionForNewTicket(ctx, ticket)
require.NotEmpty(t, result)

// TopK=3 limits results to 3 chunks. Count markdown headers to verify.
chunkCount := strings.Count(result, "### ")
require.Equal(t, 3, chunkCount, "TopK=3 should limit ticket suggestions to 3")
```

Or, for stronger determinism, add a custom embedding service that produces slightly different vectors:

```go
type topKEmbeddingService struct {
    dimension int
}

func (e *topKEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    result := make([][]float32, len(texts))
    for i, text := range texts {
        v := make([]float32, e.dimension)
        v[0] = 1.0
        v[1] = float32(i) * 0.001 // deterministic perturbation
        norm := float32(0.0)
        for j := 0; j < e.dimension; j++ {
            norm += v[j] * v[j]
        }
        norm = float32(math.Sqrt(float64(norm)))
        for j := 0; j < e.dimension; j++ {
            v[j] /= norm
        }
        result[i] = v
    }
    return result, nil
}
```

Then use `svc.vectorDB = NewMemoryVectorDB(&topKEmbeddingService{dimension: 8})` for Test 11 only.

### 2. Add Missing Env Guard (Finding 2)

```go
func setupAskRovoTest(t *testing.T, seedKeywords []string) (...) {
    t.Helper()
    t.Setenv("RAG_PIPELINE_ENABLED", "false")
    t.Setenv("LANCEDB_STORAGE_PROVIDER", "memory")
    t.Setenv("TICKET_EMBEDDING_ENABLED", "false")  // ADD THIS
    // ...
}
```

### 3. Fix `createTestMemo` Signature (Finding 3)

```go
func createTestMemo(t *testing.T, ctx context.Context, ts *store.Store, user *store.User, tenant *store.AgentTenant, content string) *store.Memo {
    t.Helper()
    n := atomic.AddInt32(&memoUIDCounter, 1)
    memo, err := ts.CreateMemo(ctx, &store.Memo{...})
```

Update all call sites to pass `ctx`.

### 4. Capture Atomic Return Values (Finding 4)

```go
slugCounter := atomic.AddInt32(&testTenantIDCounter, 1)
tenant, err = ts.CreateAgentTenant(ctx, &store.AgentTenant{
    Slug: fmt.Sprintf("ask-rovo-%d", slugCounter),
```

```go
n := atomic.AddInt32(&memoUIDCounter, 1)
memo, err := ts.CreateMemo(ctx, &store.Memo{
    UID: fmt.Sprintf("test-memo-%d-%d", tenant.ID, n),
```

### 5. Add Parallel Prohibition Comment (Finding 5)

```go
// WARNING: Tests in this file must NOT use t.Parallel() because
// ticketIndexMu and memoUIDCounter are package-level shared state
// that rely on sequential execution for uniqueness.
```

---

## Final Verdict

**APPROVED WITH NITS.** Plan 3 fixes all critical bugs from Plan 2 and is architecturally sound. The test suite covers the core Ask Rovo pipeline: indexing, inference, dedup, concurrency, tenant isolation, nil safety, MinScore threshold, and TopK limiting.

Five nits remain before implementation:

1. **MEDIUM** — TopK test assertions are flaky due to non-deterministic sort for equal scores; use count-based assertion or deterministic perturbation
2. **MEDIUM** — Add `TICKET_EMBEDDING_ENABLED=false` to `setupAskRovoTest`
3. **LOW** — `createTestMemo` should accept `ctx` parameter instead of using `context.Background()`
4. **LOW** — Capture `atomic.AddInt32` return values instead of reading variables directly
5. **LOW** — Add file-level comment prohibiting `t.Parallel()`

No critical or high-severity blockers. After addressing these 5 nits, the plan is implementation-ready.
