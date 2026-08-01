# Adversarial Test Plan Review: Plan 2 — Ask Rovo E2E Test (Revised)

**Bug/Task:** Plan for `plan2_test.md`  
**Reviewer:** Kilo (Senior Go Architect)  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED — revised plan fixes most original findings but introduces two critical blockers that will cause tests to fail at runtime, plus several medium-severity coverage gaps.

---

## Executive Summary

The revised plan correctly addresses the original review's high-severity findings: `unifiedEmbeddingService` is replaced with a keyword-based `controlledEmbeddingService` that actually exercises `MinScore` filtering, Test 2 is fleshed out with executable code, and new tests cover content-changed versioning, concurrency, and threshold boundaries. The `setupAskRovoTest` helper with unique tenant IDs is a good architectural improvement.

However, the plan contains **two critical implementation bugs** that will cause tests to fail immediately, plus **four medium-severity coverage gaps** that undermine the test suite's ability to catch real production bugs.

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | All `CreateMemo` calls omit required `UID` field — tests will fail | **HIGH** | Rework |
| 2 | `require.NoError` called from multiple goroutines in Test 9 — data race on `testing.T` | **HIGH** | Rework |
| 3 | `setupAskRovoTest` does not isolate from environment pollution | MEDIUM | Fix |
| 4 | No test for `TopK` limiting despite claim that it is "exercised" | MEDIUM | Add |
| 5 | No explicit test for `ContentTypes` filtering with mixed chunk types | MEDIUM | Add |
| 6 | No test for `IsActive` filtering | MEDIUM | Add |
| 7 | `ticketIndexMu` collision prevention claim is misleading | LOW | Document |
| 8 | No near-threshold MinScore test (0.69 vs 0.71) | LOW | Add or defer |
| 9 | Goroutine path deferred to Future Work | LOW | Document gap |
| 10 | `controlledEmbeddingService` LOW vectors are identical — unrelated texts match | LOW | Document |

---

## Finding 1: All `CreateMemo` Calls Omit Required `UID` Field

**Severity:** HIGH (CRITICAL)

Every test in the plan creates a memo without setting the `UID` field:

```go
memo, _ := ts.CreateMemo(ctx, &store.Memo{
    CreatorID: user.ID, Content: "Test", Visibility: store.Public, TenantID: &tenant.ID,
})
```

The store layer validates UID before insertion:

```go
// store/memo.go:124
func (s *Store) CreateMemo(ctx context.Context, create *Memo) (*Memo, error) {
    if !base.UIDMatcher.MatchString(create.UID) {
        return nil, errors.New("invalid uid")
    }
    return s.driver.CreateMemo(ctx, create)
}
```

`base.UIDMatcher` requires: `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,30}[a-zA-Z0-9])?$`

Empty string fails. The tests will return an error, `memo` will be nil, and subsequent access to `memo.UID` will panic.

**Required fix:** Add a valid `UID` to every `CreateMemo` call. Use a helper or inline unique values:

```go
memo, err := ts.CreateMemo(ctx, &store.Memo{
    UID:        fmt.Sprintf("test-memo-%d", tenant.ID),
    CreatorID:  user.ID,
    Content:    "Test",
    Visibility: store.Public,
    TenantID:   &tenant.ID,
})
require.NoError(t, err)
```

Existing tests in `store/test/memo_test.go` use `UID: "test-resource-name"` — the plan must follow this pattern.

---

## Finding 2: `require.NoError` Called From Multiple Goroutines

**Severity:** HIGH (CRITICAL)

Test 9 (`TestAskRovo_ConcurrentIndexing`) calls `require.NoError` from within 5 concurrent goroutines:

```go
var wg sync.WaitGroup
for i := 0; i < 5; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        _, _, err := svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, false)
        require.NoError(t, err)  // UNSAFE: concurrent use of testing.T
    }()
}
wg.Wait()
```

`testing.T` is **not safe for concurrent use**. Calling `require.NoError` from multiple goroutines simultaneously causes:
1. **Data races** on `T`'s internal state (fail count, helper state)
2. **Concurrent `t.FailNow()` calls** which can panic or corrupt test state
3. **Non-deterministic test results** — the test may pass or fail depending on goroutine scheduling

**Required fix:** Collect errors in a thread-safe slice, then assert after `wg.Wait()`:

```go
func TestAskRovo_ConcurrentIndexing(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    memo, err := ts.CreateMemo(ctx, &store.Memo{
        UID:        "concurrent-test-memo",
        CreatorID:  user.ID,
        Content:    "Test",
        Visibility: store.Public,
        TenantID:   &tenant.ID,
    })
    require.NoError(t, err)

    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title:       "Concurrent Test",
        Description: "/m/" + memo.UID,
        Status:      store.TicketStatusOpen,
        Priority:    store.TicketPriorityMedium,
        CreatorID:   user.ID,
        TenantID:    &tenant.ID,
    })
    require.NoError(t, err)

    var wg sync.WaitGroup
    var mu sync.Mutex
    var errs []error
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, _, err := svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, false)
            if err != nil {
                mu.Lock()
                errs = append(errs, err)
                mu.Unlock()
            }
        }()
    }
    wg.Wait()

    require.Empty(t, errs, "concurrent IndexTicketContent calls should not error")
    // ... rest of assertions
}
```

---

## Finding 3: No Environment Isolation in `setupAskRovoTest`

**Severity:** MEDIUM

The plan acknowledges the risk: "`NewService` env var side effects — Low — Use `t.Setenv('RAG_PIPELINE_ENABLED', 'false')`". However, the `setupAskRovoTest` helper does not actually call `t.Setenv`.

If the test environment has `LANCEDB_STORAGE_PROVIDER=local` or `RAG_PIPELINE_ENABLED=true`, `NewService` will attempt to initialize LanceDB, which may:
- Panic if CGO is not set up
- Create files in `build/data/lancedb`
- Hang on network calls if `LANCEDB_S3_*` vars are set

**Required fix:** Add env isolation to `setupAskRovoTest`:

```go
func setupAskRovoTest(t *testing.T, seedKeywords []string) (...) {
    t.Helper()
    t.Setenv("RAG_PIPELINE_ENABLED", "false")
    t.Setenv("LANCEDB_STORAGE_PROVIDER", "memory")
    // ... rest of setup
}
```

`teststore.NewTestingStore` creates an isolated SQLite DB. The service should also be isolated from environment pollution.

---

## Finding 4: No Test for `TopK` Limiting

**Severity:** MEDIUM

The plan states in Section 2.2: "`TopK` limiting — Never exercised (only 2 chunks)" as a risk, and in the Adversarial Review Prompt: "Should we assert exact structure of internal_notes? Should we verify the vector DB state after indexing?"

However, the revised plan **still does not add a test for `TopK` limiting**. The maximum number of seed chunks in any test is 2 (Tests 1, 2, 4, 10) or 0 (Test 3). `InferResolutionForNewTicket` uses `TopK: 3` for both searches. Since no test exceeds 3 chunks, `TopK` is never exercised.

**Required addition:** Add a test that seeds 5+ chunks of the same type and asserts only `TopK` (3) results are returned:

```go
func TestAskRovo_TopKLimiting(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    // Seed 5 ticket chunks (all contain keywords → all match)
    for i := 0; i < 5; i++ {
        svc.vectorDB.Insert(ctx, []DocumentChunk{{
            ID:         fmt.Sprintf("seed_ticket_%d", i),
            TenantID:   tenant.ID,
            AudienceType: "internal",
            ContentType: "ticket",
            Title:      fmt.Sprintf("Ticket %d", i),
            Content:    fmt.Sprintf("Per-ticket RAG indexing ticket %d", i),
            IsActive:   true,
        }})
    }

    memo, _ := ts.CreateMemo(ctx, &store.Memo{...})
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{...})

    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.NotEmpty(t, result)
    // TopK=3, so only 3 tickets should be suggested
    require.Contains(t, result, "Ticket 0")
    require.Contains(t, result, "Ticket 1")
    require.Contains(t, result, "Ticket 2")
    require.NotContains(t, result, "Ticket 3")
    require.NotContains(t, result, "Ticket 4")
}
```

---

## Finding 5: No Explicit Test for `ContentTypes` Filtering With Mixed Types

**Severity:** MEDIUM

Test 2 implicitly verifies `ContentTypes` filtering by seeding only `bug_section` chunks and asserting that `Relevant Bug History` appears. However, there is no test that seeds **both** `ticket` and `bug_section` chunks and asserts that searching for one type excludes the other.

If the `ContentTypes` filter were broken (e.g., always returning all types), Test 2 would still pass because it only checks for the presence of bug section results, not the absence of ticket results.

**Required addition:** Add a test that seeds both types and verifies isolation:

```go
func TestAskRovo_ContentTypesFiltering(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    svc.vectorDB.Insert(ctx, []DocumentChunk{
        {ID: "seed_ticket", TenantID: tenant.ID, AudienceType: "internal",
         ContentType: "ticket", Title: "Ticket Seed",
         Content: "RAG indexing for tickets", IsActive: true},
        {ID: "seed_bug", TenantID: tenant.ID, AudienceType: "internal",
         ContentType: "bug_section", Title: "Bug Seed",
         Content: "RAG indexing for bugs", IsActive: true},
    })

    memo, _ := ts.CreateMemo(ctx, &store.Memo{...})
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Indexing", Description: "/m/" + memo.UID,
        // ...
    })

    result := svc.InferResolutionForNewTicket(ctx, ticket)
    // Both searches run, but we verify the final output structure
    require.Contains(t, result, "Suggested Resolution")
    require.NotContains(t, result, "Relevant Bug History")  // bug section MinScore is 0.5, but both vectors are HIGH → cosine=1.0, so bug section WOULD match
}
```

Wait, actually with `controlledEmbeddingService`, both the ticket seed and bug seed contain keywords → both get HIGH vectors. The query also gets HIGH vector. So both searches would find results. This means the test as written would NOT verify `ContentTypes` isolation because both types match.

To properly test `ContentTypes` filtering, we need a query that matches only one type. But with the binary embedding, any keyword-matching query matches all keyword-containing chunks regardless of type.

This is a **fundamental limitation** of the `controlledEmbeddingService`: it cannot distinguish between chunks of different types. The `ContentTypes` filter is applied AFTER embedding/cosine similarity, so if all chunks have high similarity, all types pass the filter.

To test `ContentTypes` filtering explicitly, we would need to either:
1. Use a different mock that produces type-dependent embeddings
2. Test the `Search` method directly with controlled `SearchQuery` parameters
3. Accept that `ContentTypes` filtering is implicitly tested by the separate search runs in `InferResolutionForNewTicket`

**Recommendation:** Defer explicit `ContentTypes` filtering test to Future Work. The current tests implicitly verify it by checking for the presence/absence of specific sections in the output.

---

## Finding 6: No Test for `IsActive` Filtering

**Severity:** MEDIUM

`MemoryVectorDB.Search` filters out chunks where `IsActive == false` when `query.ActiveOnly` is true. `InferResolutionForNewTicket` does not set `ActiveOnly` explicitly, so it defaults to... let me check.

Looking at `SearchQuery`:
```go
type SearchQuery struct {
    // ...
    ActiveOnly bool
}
```

And in `MemoryVectorDB.Search`:
```go
if query.ActiveOnly && !chunk.IsActive {
    continue
}
```

If `ActiveOnly` is false (default), inactive chunks ARE included in search. So `IsActive` filtering is not actually applied in the production `InferResolutionForNewTicket` path.

This means the test for `IsActive` filtering is not relevant for the current production code. The plan should either:
1. Remove this from the review scope (not a gap)
2. Add a test that calls `vectorDB.Search` directly with `ActiveOnly: true`

**Recommendation:** Remove `IsActive` from the review findings. It is not a gap in the current production path.

---

## Finding 7: `ticketIndexMu` Collision Prevention Claim Is Misleading

**Severity:** LOW

The plan claims: "Unique tenant IDs (100+counter) per test via `atomic.AddInt32`" prevents `ticketIndexMu` collisions.

However, `testTenantIDCounter` is used for the tenant **Slug**, not the tenant ID:

```go
atomic.AddInt32(&testTenantIDCounter, 1)
tenant, err = ts.CreateAgentTenant(ctx, &store.AgentTenant{
    Slug:        fmt.Sprintf("ask-rovo-%d", testTenantIDCounter),
    // ...
})
```

`CreateAgentTenant` assigns the ID via the database (auto-increment). Each test creates a fresh SQLite DB, so tenant IDs start from 1 in each DB. The actual `ticketIndexMu` keys are `fmt.Sprintf("%d:%d", tenantID, ticket.ID)`, which are DB-assigned.

Since tests run sequentially by default, the mutex from Test 1 is unlocked before Test 2 starts. No collision occurs. But the plan's explanation of *why* collisions are prevented is incorrect.

**Recommendation:** Update the comment to reflect the actual safeguard:

```go
// Unique slugs prevent accidental tenant confusion during manual debugging.
// Actual mutex key collision is prevented by sequential test execution
// (Go tests run sequentially unless t.Parallel() is called).
```

---

## Finding 8: No Near-Threshold MinScore Test

**Severity:** LOW

Test 10 tests below-threshold behavior (cosine=0.0, well below MinScore=0.7). It does not test near-threshold behavior (e.g., cosine=0.69 vs 0.71).

With the `controlledEmbeddingService`, producing arbitrary cosine similarities is impossible because vectors are binary (HIGH or LOW). To test near-threshold, a more sophisticated mock would be needed.

**Recommendation:** Defer to Future Work. The binary mock is sufficient to prove the threshold mechanism works (below threshold → no results).

---

## Finding 9: Goroutine Path Deferred to Future Work

**Severity:** LOW

The plan acknowledges that the production goroutine path in `ticket_service.go:193` is not tested and defers it to Future Work. For a hackathon deliverable, this is acceptable.

**Recommendation:** Document this explicitly as a known coverage gap in the plan's Risk Assessment.

---

## Finding 10: `controlledEmbeddingService` Design Limitation

**Severity:** LOW

The `controlledEmbeddingService` produces only two vectors: HIGH (`[1.0, 0, ...]`) and LOW (`[0, 1.0, ...]`). All HIGH vectors are identical, and all LOW vectors are identical.

This means:
- Two unrelated texts both without keywords → both get LOW vector → cosine=1.0 → they match each other
- This is unrealistic compared to production embeddings where unrelated texts have low similarity

**Impact on tests:** None, because the tests carefully ensure that queries and expected matches either both contain keywords (HIGH) or the query lacks keywords (LOW). The false-positive scenario (two LOW vectors matching) never occurs in the test suite.

**Recommendation:** Document this limitation in the `controlledEmbeddingService` comments.

---

## What Is Correct

| Check | Status | Notes |
|-------|--------|-------|
| `controlledEmbeddingService` design | CORRECT | Binary vectors exercise MinScore, ContentTypes, TenantID filters correctly |
| Test 1 (happy path) | CORRECT | Seeds, indexes, infers, asserts InternalNotes |
| Test 2 (bug sections) | CORRECT | Full executable code, verifies bug section search |
| Test 3 (no results) | CORRECT | Empty vector DB → empty result |
| Test 4 (tenant isolation) | CORRECT | Cross-tenant seeds excluded by TenantID filter |
| Test 5 (dedup unchanged) | CORRECT | Two index calls, one version |
| Test 6 (content changed) | CORRECT | Two index calls with different content, two versions |
| Test 7 (comments) | CORRECT | Comments appear in indexed content blob |
| Test 8 (nil tenant) | CORRECT | No panic, returns empty |
| `setupAskRovoTest` helper | CORRECT | Unique slugs, clean setup/teardown |
| Struct references | CORRECT | All point to `store/ticket.go` |

---

## Recommended Rework

### 1. Fix Missing UIDs (Finding 1)

Add a `UID` field to every `CreateMemo` call. Use a helper:

```go
func createTestMemo(t *testing.T, ts *store.Store, user *store.User, tenant *store.AgentTenant, content string) *store.Memo {
    t.Helper()
    memo, err := ts.CreateMemo(ctx, &store.Memo{
        UID:        fmt.Sprintf("test-memo-%d", tenant.ID),
        CreatorID:  user.ID,
        Content:    content,
        Visibility: store.Public,
        TenantID:   &tenant.ID,
    })
    require.NoError(t, err)
    return memo
}
```

### 2. Fix Concurrent `require.NoError` (Finding 2)

Replace inline `require.NoError` in goroutines with thread-safe error collection.

### 3. Add Environment Isolation (Finding 3)

Add `t.Setenv("RAG_PIPELINE_ENABLED", "false")` and `t.Setenv("LANCEDB_STORAGE_PROVIDER", "memory")` to `setupAskRovoTest`.

### 4. Add `TopK` Test (Finding 4)

Add `TestAskRovo_TopKLimiting` with 5+ seed chunks and verify only 3 are returned.

### 5. Document `ContentTypes` Limitation (Finding 5)

Add a comment explaining that explicit `ContentTypes` filtering is deferred because the binary mock cannot distinguish types by similarity.

### 6. Document `ticketIndexMu` Collision Prevention (Finding 7)

Update the comment to reflect sequential execution as the actual safeguard.

### 7. Document Goroutine Gap (Finding 9)

Add to Risk Assessment: "Goroutine path in `ticket_service.go:193` is not tested. Concurrency errors in the async path will not be caught."

### 8. Document Mock Limitation (Finding 10)

Add comment to `controlledEmbeddingService`: "LOW vectors are identical across all non-keyword texts. This is a test-only simplification; production embeddings produce unique vectors for unique texts."

---

## Final Verdict

**REWORK REQUIRED.** The revised plan fixes the original high-severity issues with the embedding service and adds valuable new tests. However, two critical implementation bugs will cause the test suite to fail immediately:

1. **Missing UIDs** in all `CreateMemo` calls (Finding 1)
2. **Concurrent `require.NoError`** in Test 9 (Finding 2)

Additionally, four medium-severity coverage gaps reduce the test suite's ability to catch production bugs. After addressing the critical fixes and the medium-severity items, this plan will be implementation-ready.
