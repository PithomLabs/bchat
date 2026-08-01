# Plan 4: Ask Rovo E2E Test (Final — Ready for Implementation)

**Date:** 2026-08-01
**Status:** Approved with nits — all 5 review findings addressed
**Review:** plan3_test_review.md — 5 nits fixed
**Related:** [Bug 052](../052/) (per-ticket RAG indexing), [Bug 054](../054/) (tenant association fixes)

---

## 1. Goal

Write a comprehensive integration test suite that proves the Ask Rovo pipeline works correctly — search logic, scoring thresholds, concurrency controls, and version management all function as designed.

### What Changed from Plan 3

| # | Finding | Severity | Fix Applied |
|---|---------|----------|-------------|
| 1 | TopK test flaky — non-deterministic sort for equal scores | MEDIUM | **Fixed** — count-based assertion: `strings.Count(result, "### ") == 3` |
| 2 | Missing `TICKET_EMBEDDING_ENABLED` env isolation | MEDIUM | **Fixed** — added `t.Setenv("TICKET_EMBEDDING_ENABLED", "false")` |
| 3 | `createTestMemo` uses `context.Background()` | LOW | **Fixed** — added `ctx context.Context` parameter |
| 4 | `atomic.AddInt32` return value not captured | LOW | **Fixed** — capture return value in both `setupAskRovoTest` and `createTestMemo` |
| 5 | No explicit prohibition of `t.Parallel()` | LOW | **Fixed** — file-level warning comment |

---

## 2. Test Infrastructure

### 2.1 Components

| Component | Source | Purpose |
|-----------|--------|---------|
| `teststore.NewTestingStore(ctx, t)` | `store/test/store.go:24` | Real SQLite DB with all migrations |
| `NewService(ts, profile)` | `service.go:89` | Creates Service with chunker initialized |
| `NewMemoryVectorDB(embedSvc)` | `vectordb.go:308` | In-memory VectorDB (full interface) |
| `NewChunker()` | `chunker.go:60` | Document chunking |
| `createTestingHostUser()` | `store/test/user_test.go:41` | Creates HOST user for ticket creator FK |
| `controlledEmbeddingService` | **new** (test file) | Keyword-based deterministic embeddings |

### 2.2 `controlledEmbeddingService`

**Why:** The `unifiedEmbeddingService` returns `[1.0, 0, 0, ...]` for all texts, making cosine similarity always 1.0. This hides bugs in `MinScore` filtering, `ContentTypes` filtering, and `TopK` limiting.

**Design:** A keyword-based embedding service that returns two distinct unit vectors:
- **High vector** (`v[0] = 1.0`): for texts containing any seed keyword
- **Low vector** (`v[1] = 1.0`): for texts without seed keywords

Cosine similarities:
- `cosine(high, high) = 1.0` → above `MinScore: 0.7` → **found**
- `cosine(high, low) = 0.0` → below `MinScore: 0.7` → **not found**
- `cosine(low, low) = 1.0` → above `MinScore: 0.5` → found (but low vectors are only used for non-matching queries)

```go
// controlledEmbeddingService returns deterministic embeddings based on keyword presence.
// Texts containing any seed keyword get a "high" vector; others get a "low" vector.
// This ensures cosine similarity is either 1.0 (match) or 0.0 (no match),
// exercising the MinScore threshold in MemoryVectorDB.Search.
//
// Limitation: LOW vectors are identical across all non-keyword texts. This is a
// test-only simplification; production embeddings produce unique vectors for unique texts.
// The test suite is designed to avoid false-positive scenarios from this limitation.
type controlledEmbeddingService struct {
    dimension    int
    seedKeywords []string
}

func (c *controlledEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    result := make([][]float32, len(texts))
    for i, text := range texts {
        if c.containsKeyword(text) {
            result[i] = c.highVector()
        } else {
            result[i] = c.lowVector()
        }
    }
    return result, nil
}

func (c *controlledEmbeddingService) containsKeyword(text string) bool {
    lower := strings.ToLower(text)
    for _, kw := range c.seedKeywords {
        if strings.Contains(lower, strings.ToLower(kw)) {
            return true
        }
    }
    return false
}

func (c *controlledEmbeddingService) highVector() []float32 {
    v := make([]float32, c.dimension)
    v[0] = 1.0
    return v
}

func (c *controlledEmbeddingService) lowVector() []float32 {
    v := make([]float32, c.dimension)
    v[1] = 1.0
    return v
}

func (c *controlledEmbeddingService) Dimension() int      { return c.dimension }
func (c *controlledEmbeddingService) Provider() string    { return "controlled-test" }
func (c *controlledEmbeddingService) MaxInputTokens() int { return 8192 }
```

### 2.3 Service Setup Helper

```go
var testTenantIDCounter int32 = 100

// setupAskRovoTest creates an isolated test environment with a fresh SQLite store,
// a Service with MemoryVectorDB backed by controlledEmbeddingService, and a test tenant/user.
// Environment variables are isolated to prevent LanceDB/TICKET_EMBEDDING initialization side effects.
func setupAskRovoTest(t *testing.T, seedKeywords []string) (
    ctx context.Context,
    ts *store.Store,
    svc *Service,
    tenant *store.AgentTenant,
    user *store.User,
) {
    t.Helper()
    // Isolate from environment pollution
    t.Setenv("RAG_PIPELINE_ENABLED", "false")
    t.Setenv("LANCEDB_STORAGE_PROVIDER", "memory")
    t.Setenv("TICKET_EMBEDDING_ENABLED", "false") // Prevent background goroutine (Review Finding 2)

    ctx = context.Background()
    ts = teststore.NewTestingStore(ctx, t)
    t.Cleanup(func() { ts.Close() })

    var err error
    user, err = createTestingHostUser(ctx, ts)
    require.NoError(t, err)

    // Capture atomic return value for uniqueness (Review Finding 4)
    slugCounter := atomic.AddInt32(&testTenantIDCounter, 1)
    tenant, err = ts.CreateAgentTenant(ctx, &store.AgentTenant{
        Slug:        fmt.Sprintf("ask-rovo-%d", slugCounter),
        CompanyName: "Ask Rovo Test",
        Vertical:    "test",
        IsActive:    true,
    })
    require.NoError(t, err)

    svc = NewService(ts, &profile.Profile{Driver: "sqlite", Mode: "prod"})
    svc.vectorDB = NewMemoryVectorDB(&controlledEmbeddingService{
        dimension: 8, seedKeywords: seedKeywords,
    })
    return
}
```

### 2.4 Memo Creation Helper

`store/memo.go:124` validates UID with `base.UIDMatcher` (`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,30}[a-zA-Z0-9])?$`). Empty UID fails with `"invalid uid"`. All `CreateMemo` calls must include a valid `UID`.

```go
var memoUIDCounter int32 = 0

// createTestMemo creates a memo with a unique UID for testing.
// Accepts ctx instead of using context.Background() for future-proofing (Review Finding 3).
func createTestMemo(t *testing.T, ctx context.Context, ts *store.Store, user *store.User, tenant *store.AgentTenant, content string) *store.Memo {
    t.Helper()
    // Capture atomic return value for uniqueness (Review Finding 4)
    n := atomic.AddInt32(&memoUIDCounter, 1)
    memo, err := ts.CreateMemo(ctx, &store.Memo{
        UID:        fmt.Sprintf("test-memo-%d-%d", tenant.ID, n),
        CreatorID:  user.ID,
        Content:    content,
        Visibility: store.Public,
        TenantID:   &tenant.ID,
    })
    require.NoError(t, err)
    return memo
}
```

---

## 3. Test Cases

### Test 1: `TestAskRovo_InferResolutionFromSimilarTickets`

Happy path. Seed → index → infer → `InternalNotes` populated with ticket suggestions.

### Test 2: `TestAskRovo_InferResolutionFromBugSections`

Bug history search. bug_section chunks found by search 2, even when search 1 returns nothing.

### Test 3: `TestAskRovo_NoResultsReturnsEmpty`

Empty state. No seeds → inference returns `""`, `InternalNotes` not updated.

### Test 4: `TestAskRovo_TenantIsolation`

Cross-tenant isolation. Seeds for tenant 1 are NOT returned for tenant 2 queries.

### Test 5: `TestAskRovo_ContentHashDedup`

Unchanged content dedup. Indexing same content twice does NOT create a second version row.

### Test 6: `TestAskRovo_ContentChangedCreatesNewVersion`

Changed content increments version. Indexing different content creates a new version row.

### Test 7: `TestAskRovo_IndexTicketContentWithComments`

Comments included in indexed blob. Comment text appears in the indexed content.

### Test 8: `TestAskRovo_NilTenantSkipsInference`

Nil tenant safety. Nil `TenantID` → early return, no panic.

### Test 9: `TestAskRovo_ConcurrentIndexing`

Concurrency control. Concurrent `IndexTicketContent` calls do NOT create duplicate versions or panic.

### Test 10: `TestAskRovo_MinScoreThreshold`

Threshold boundary. Queries without matching keywords produce low-similarity vectors below `MinScore: 0.7`.

### Test 11: `TestAskRovo_TopKLimiting`

TopK limiting. When >3 matching chunks exist, only `TopK=3` results are returned. **Uses count-based assertion** to avoid flakiness from non-deterministic sort of equal-score chunks.

```go
func TestAskRovo_TopKLimiting(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    for i := 0; i < 5; i++ {
        svc.vectorDB.Insert(ctx, []DocumentChunk{{
            ID:           fmt.Sprintf("seed_ticket_%d", i),
            TenantID:     tenant.ID,
            AudienceType: "internal",
            ContentType:  "ticket",
            Title:        fmt.Sprintf("Ticket %d", i),
            Content:      fmt.Sprintf("Per-ticket RAG indexing ticket %d", i),
            IsActive:     true,
        }})
    }

    memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Indexing TopK Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.NotEmpty(t, result)

    // TopK=3 limits results to 3 chunks. Count markdown headers to verify.
    chunkCount := strings.Count(result, "### ")
    require.Equal(t, 3, chunkCount, "TopK=3 should limit ticket suggestions to 3")
}
```

---

## 4. Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `server/router/api/v1/agent/ticket_rag_inference_test.go` | **CREATE** | 11 test cases + helpers |

**No production code changes.** This test exercises existing code only.

---

## 5. Test Execution

```bash
cd /home/chaschel/Documents/go/bchat
go test -v -run TestAskRovo ./server/router/api/v1/agent/ -count=1
go test -v -race -run TestAskRovo ./server/router/api/v1/agent/ -count=1
```

---

## 6. Expected Assertions Summary

| Test | Key Assertions |
|------|---------------|
| 1 (happy path) | `chunks > 0`, `inferred == true`, `InternalNotes` contains "Suggested Resolution" + seed titles |
| 2 (bug sections) | `inferred == true`, `InternalNotes` contains "Relevant Bug History" + bug title |
| 3 (no results) | `result == ""`, `InternalNotes` empty |
| 4 (tenant isolation) | `result == ""` for cross-tenant query |
| 5 (dedup unchanged) | `count(*)` = 1, `version = 1` |
| 6 (dedup changed) | `count(*)` = 2, versions = {1, 2} |
| 7 (comments) | Indexed content blob contains comment text |
| 8 (nil tenant) | Returns `""`, no panic |
| 9 (concurrent) | `count(*)` = 1 after 5 concurrent calls, no race, no errors |
| 10 (MinScore) | `result == ""` for non-matching query |
| 11 (TopK) | `strings.Count(result, "### ")` == 3 |

---

## 7. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| `controlledEmbeddingService` doesn't match production behavior | Low | Test validates search logic, not embedding quality. Documented limitation. |
| `NewService` env var side effects | Low | `t.Setenv` guards for `RAG_PIPELINE_ENABLED`, `LANCEDB_STORAGE_PROVIDER`, `TICKET_EMBEDDING_ENABLED` |
| `ticketIndexMu` package-level collision between tests | Low | Sequential test execution (no `t.Parallel()`). File-level warning comment. |
| Goroutine race conditions not tested | Medium | Known gap. Acceptable for hackathon. |

---

## 8. Future Work

| Item | Description | Priority |
|------|-------------|----------|
| Goroutine path test | Test `go func()` path in `ticket_service.go:193` | HIGH |
| `createSystemResolutionComment` test | Verify system comment created | HIGH |
| Near-threshold MinScore test | Cosine=0.69 vs 0.71 boundary | MEDIUM |
| Error path test | Mock `ReindexFileVersion` failure | MEDIUM |
| Comment edit re-index | Test `UpdateMemo` hook triggers re-index | MEDIUM |
| `ContentTypes` filtering with type-dependent mock | Different embeddings per content type | LOW |
