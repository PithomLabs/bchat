# Plan 3: Ask Rovo E2E Test (Final Revised)

**Date:** 2026-08-01
**Status:** Revised based on plan2_test_review.md findings
**Review:** 10 findings — 8 addressed, 1 not-a-gap, 1 deferred
**Related:** [Bug 052](../052/) (per-ticket RAG indexing), [Bug 054](../054/) (tenant association fixes)

---

## 1. Goal

Write a comprehensive integration test suite that proves the Ask Rovo pipeline works correctly — search logic, scoring thresholds, concurrency controls, and version management all function as designed.

### What Changed from Plan 2

| Finding | Severity | Action Taken |
|---------|----------|--------------|
| 1: `CreateMemo` omits required `UID` | HIGH | **Fixed** — added `UID` to all CreateMemo calls, added `createTestMemo` helper |
| 2: `require.NoError` from goroutines in Test 9 | HIGH | **Fixed** — collect errors in thread-safe slice, assert after `wg.Wait()` |
| 3: No environment isolation in `setupAskRovoTest` | MEDIUM | **Fixed** — added `t.Setenv` for `RAG_PIPELINE_ENABLED` and `LANCEDB_STORAGE_PROVIDER` |
| 4: No test for `TopK` limiting | MEDIUM | **Added** Test 11 |
| 5: No explicit `ContentTypes` filtering test | MEDIUM | **Documented** — binary mock limitation, deferred to Future Work |
| 6: No test for `IsActive` filtering | MEDIUM | **Not a gap** — `InferResolutionForNewTicket` doesn't set `ActiveOnly`, inactive chunks ARE included (current production behavior) |
| 7: `ticketIndexMu` collision claim misleading | LOW | **Fixed** — comment now reflects sequential execution as actual safeguard |
| 8: No near-threshold MinScore test | LOW | **Deferred** to Future Work — binary mock can't produce 0.69 similarity |
| 9: Goroutine path deferred | LOW | **Documented** in Risk Assessment as known coverage gap |
| 10: LOW vectors identical across texts | LOW | **Documented** in `controlledEmbeddingService` comments — no test impact |

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

**Limitation (Finding 10):** LOW vectors are identical across all non-keyword texts. Two unrelated texts without keywords both get LOW vectors → cosine=1.0 → they match each other. This is unrealistic compared to production embeddings. However, the test suite is designed to avoid this scenario: queries and expected matches either both contain keywords (HIGH) or the query lacks keywords (LOW). No false-positive scenario occurs.

**Limitation (Finding 5):** The binary mock cannot distinguish between chunk types by similarity. Both ticket and bug_section seeds get HIGH vectors if they contain keywords. Explicit `ContentTypes` filtering (verifying ticket search excludes bug chunks) cannot be tested with this mock. The filter is implicitly exercised by the two separate search runs in `InferResolutionForNewTicket`.

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
// Environment variables are isolated to prevent LanceDB initialization side effects.
func setupAskRovoTest(t *testing.T, seedKeywords []string) (
    ctx context.Context,
    ts *store.Store,
    svc *Service,
    tenant *store.AgentTenant,
    user *store.User,
) {
    t.Helper()
    // Isolate from environment pollution (Finding 3)
    t.Setenv("RAG_PIPELINE_ENABLED", "false")
    t.Setenv("LANCEDB_STORAGE_PROVIDER", "memory")

    ctx = context.Background()
    ts = teststore.NewTestingStore(ctx, t)
    t.Cleanup(func() { ts.Close() })

    var err error
    user, err = createTestingHostUser(ctx, ts)
    require.NoError(t, err)

    // Unique slugs prevent accidental tenant confusion during manual debugging.
    // Actual mutex key collision is prevented by sequential test execution
    // (Go tests run sequentially unless t.Parallel() is called) (Finding 7).
    atomic.AddInt32(&testTenantIDCounter, 1)
    tenant, err = ts.CreateAgentTenant(ctx, &store.AgentTenant{
        Slug:        fmt.Sprintf("ask-rovo-%d", testTenantIDCounter),
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

### 2.4 Memo Creation Helper (Finding 1)

`store/memo.go:124` validates UID with `base.UIDMatcher` (`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,30}[a-zA-Z0-9])?$`). Empty UID fails with `"invalid uid"`. All `CreateMemo` calls must include a valid `UID`.

```go
var memoUIDCounter int32 = 0

// createTestMemo creates a memo with a unique UID for testing.
// Follows the pattern from store/test/memo_test.go which uses "test-resource-name" UIDs.
func createTestMemo(t *testing.T, ts *store.Store, user *store.User, tenant *store.AgentTenant, content string) *store.Memo {
    t.Helper()
    n := atomic.AddInt32(&memoUIDCounter, 1)
    memo, err := ts.CreateMemo(context.Background(), &store.Memo{
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

### 2.5 Seed Data

**Seed chunk 1 (ContentType="ticket"):**
```
ID:     "seed_ticket_1"
Title:  "Per-Ticket RAG Indexing"
Content: "Per-ticket RAG indexing for Jira-style Ask Rovo feature. Each ticket's
content is indexed into the vector DB so that InferResolutionForNewTicket can
surface similar historical tickets and bug sections when a new ticket is created."
```

**Seed chunk 2 (ContentType="ticket"):**
```
ID:     "seed_ticket_2"
Title:  "IndexTicketContent Helper"
Content: "IndexTicketContent builds a markdown blob from title + description +
comments, computes ContentHash, queries latest AgentSourceFile, skips upsert
if hash matches, otherwise upserts and calls ReindexFileVersion."
```

**Seed chunk 3 (ContentType="bug_section"):**
```
ID:     "seed_bug_1"
Title:  "Bug 052 — RAG Index Gap"
Content: "InferResolutionForNewTicket searches for ContentTypes ticket but no
code ever indexes ticket content into the vector DB. The search always returns
empty. This is the missing piece that per-ticket indexing resolves."
```

**Seed keywords:** `["rag", "indexing", "ticket"]`

---

## 3. Test Cases

### Test 1: `TestAskRovo_InferResolutionFromSimilarTickets`

**Happy path.** Proves: seed → index → infer → `InternalNotes` populated with ticket suggestions.

```go
func TestAskRovo_InferResolutionFromSimilarTickets(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    svc.vectorDB.Insert(ctx, []DocumentChunk{
        {ID: "seed_ticket_1", TenantID: tenant.ID, AudienceType: "internal",
         ContentType: "ticket", Title: "Per-Ticket RAG Indexing",
         Content: "Per-ticket RAG indexing for Jira-style Ask Rovo feature...",
         IsActive: true},
        {ID: "seed_ticket_2", TenantID: tenant.ID, AudienceType: "internal",
         ContentType: "ticket", Title: "IndexTicketContent Helper",
         Content: "IndexTicketContent builds a markdown blob...",
         IsActive: true},
    })

    memo := createTestMemo(t, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Indexing Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

    chunks, inferred, err := svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, true)
    require.NoError(t, err)
    require.Greater(t, chunks, 0)
    require.True(t, inferred)

    updated, _ := ts.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
    require.Contains(t, updated.InternalNotes, "## Suggested Resolution (Auto-generated)")
    require.Contains(t, updated.InternalNotes, "Per-Ticket RAG Indexing")
}
```

### Test 2: `TestAskRovo_InferResolutionFromBugSections`

**Bug history search.** Proves: bug_section chunks are found by search 2, even when search 1 (tickets) returns nothing.

```go
func TestAskRovo_InferResolutionFromBugSections(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    svc.vectorDB.Insert(ctx, []DocumentChunk{
        {ID: "seed_bug_1", TenantID: tenant.ID, AudienceType: "internal",
         ContentType: "bug_section", Title: "Bug 052 — RAG Index Gap",
         Content: "InferResolutionForNewTicket searches for ContentTypes ticket but no code ever indexes ticket content into the vector DB...",
         IsActive: true},
    })

    memo := createTestMemo(t, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Index Gap Fix", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

    _, inferred, err := svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, true)
    require.NoError(t, err)
    require.True(t, inferred)

    updated, _ := ts.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
    require.Contains(t, updated.InternalNotes, "Relevant Bug History")
    require.Contains(t, updated.InternalNotes, "Bug 052 — RAG Index Gap")
}
```

### Test 3: `TestAskRovo_NoResultsReturnsEmpty`

**Empty state.** Proves: no seeds → inference returns `""`, `InternalNotes` not updated.

```go
func TestAskRovo_NoResultsReturnsEmpty(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    memo := createTestMemo(t, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Unrelated Bug", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.Equal(t, "", result)

    fetched, _ := ts.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
    require.Empty(t, fetched.InternalNotes)
}
```

### Test 4: `TestAskRovo_TenantIsolation`

**Cross-tenant isolation.** Proves: seeds for tenant 1 are NOT returned for tenant 2 queries.

```go
func TestAskRovo_TenantIsolation(t *testing.T) {
    ctx, ts, svc, tenant1, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    tenant2, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{
        Slug: "ask-rovo-isolation", CompanyName: "Isolation Test",
        Vertical: "test", IsActive: true,
    })
    require.NoError(t, err)

    svc.vectorDB.Insert(ctx, []DocumentChunk{
        {ID: "seed_t1", TenantID: tenant1.ID, AudienceType: "internal",
         ContentType: "ticket", Title: "Tenant 1 Content",
         Content: "RAG indexing for tenant 1", IsActive: true},
    })

    memo := createTestMemo(t, ts, user, tenant2, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Indexing for Tenant 2", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant2.ID,
    })
    require.NoError(t, err)

    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.Equal(t, "", result)
}
```

### Test 5: `TestAskRovo_ContentHashDedup`

**Unchanged content dedup.** Proves: indexing same content twice does NOT create a second version row.

```go
func TestAskRovo_ContentHashDedup(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    memo := createTestMemo(t, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Dedup Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

    svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, false)
    svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, false)

    fileType := "ticket"
    files, _ := ts.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
        TenantID: &tenant.ID, FileType: &fileType,
    })
    require.Equal(t, 1, len(files))
    require.Equal(t, int32(1), files[0].Version)
}
```

### Test 6: `TestAskRovo_ContentChangedCreatesNewVersion`

**Changed content increments version.** Proves: indexing different content creates a new version row.

```go
func TestAskRovo_ContentChangedCreatesNewVersion(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    memo := createTestMemo(t, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Version Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

    svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, false)

    newTitle := "Version Test — Updated"
    ts.UpdateTicket(ctx, &store.UpdateTicket{ID: ticket.ID, Title: &newTitle})
    ticket.Title = newTitle

    svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, false)

    fileType := "ticket"
    files, _ := ts.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
        TenantID: &tenant.ID, FileType: &fileType,
    })
    require.Equal(t, 2, len(files))
    versions := []int32{files[0].Version, files[1].Version}
    require.Contains(t, versions, int32(1))
    require.Contains(t, versions, int32(2))
}
```

### Test 7: `TestAskRovo_IndexTicketContentWithComments`

**Comments included in indexed blob.** Proves: comment text appears in the indexed content.

```go
func TestAskRovo_IndexTicketContentWithComments(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    memo := createTestMemo(t, ts, user, tenant, "Parent memo")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Comment Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

    comment1 := createTestMemo(t, ts, user, tenant, "First comment about RAG indexing")
    comment2 := createTestMemo(t, ts, user, tenant, "Second comment about ticket indexing")
    ts.UpsertMemoRelation(ctx, &store.MemoRelation{
        MemoID: comment1.ID, RelatedMemoID: memo.ID,
        Type: store.MemoRelationComment, TenantID: &tenant.ID,
    })
    ts.UpsertMemoRelation(ctx, &store.MemoRelation{
        MemoID: comment2.ID, RelatedMemoID: memo.ID,
        Type: store.MemoRelationComment, TenantID: &tenant.ID,
    })

    comments := []*store.Memo{comment1, comment2}
    chunks, _, err := svc.IndexTicketContent(ctx, tenant.ID, ticket, comments, false)
    require.NoError(t, err)
    require.Greater(t, chunks, 0)

    fileType := "ticket"
    latestOnly := true
    files, _ := ts.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
        TenantID: &tenant.ID, FileType: &fileType, LatestOnly: latestOnly,
    })
    require.Greater(t, len(files), 0)
    require.Contains(t, files[0].Content, "First comment about RAG indexing")
    require.Contains(t, files[0].Content, "Second comment about ticket indexing")
}
```

### Test 8: `TestAskRovo_NilTenantSkipsInference`

**Nil tenant safety.** Proves: nil `TenantID` → early return, no panic.

```go
func TestAskRovo_NilTenantSkipsInference(t *testing.T) {
    _, ts, svc, _, _ := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    ticket := &store.Ticket{ID: 99999, Title: "Nil Tenant", TenantID: nil}
    result := svc.InferResolutionForNewTicket(context.Background(), ticket)
    require.Equal(t, "", result)
}
```

### Test 9: `TestAskRovo_ConcurrentIndexing`

**Concurrency control.** Proves: concurrent `IndexTicketContent` calls for the same ticket do NOT create duplicate versions or panic. Must pass with `-race`.

**Fix (Finding 2):** `testing.T` is not safe for concurrent use. `require.NoError` called from multiple goroutines causes data races on `T`'s internal state. Collect errors in a thread-safe slice, assert after `wg.Wait()`.

```go
func TestAskRovo_ConcurrentIndexing(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    memo := createTestMemo(t, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Concurrent Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
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

    fileType := "ticket"
    files, _ := ts.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
        TenantID: &tenant.ID, FileType: &fileType,
    })
    require.Equal(t, 1, len(files))
}
```

### Test 10: `TestAskRovo_MinScoreThreshold`

**Threshold boundary.** Proves: queries without matching keywords produce low-similarity vectors that fall below `MinScore: 0.7`, returning no results.

```go
func TestAskRovo_MinScoreThreshold(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    svc.vectorDB.Insert(ctx, []DocumentChunk{
        {ID: "seed_1", TenantID: tenant.ID, AudienceType: "internal",
         ContentType: "ticket", Title: "RAG Indexing Feature",
         Content: "Per-ticket RAG indexing for Ask Rovo", IsActive: true},
    })

    memo := createTestMemo(t, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Fire Damage Repair", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.Equal(t, "", result)
}
```

### Test 11: `TestAskRovo_TopKLimiting` (Finding 4)

**TopK limiting.** Proves: when >3 matching chunks exist, only `TopK=3` results are returned. `InferResolutionForNewTicket` uses `TopK: 3` for both ticket and bug_section searches.

```go
func TestAskRovo_TopKLimiting(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    // Seed 5 ticket chunks (all contain keywords → all match)
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

    memo := createTestMemo(t, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Indexing TopK Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

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

## 4. Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `server/router/api/v1/agent/ticket_rag_inference_test.go` | **CREATE** | 11 test cases + `controlledEmbeddingService` + `setupAskRovoTest` + `createTestMemo` helpers |

**No production code changes.** This test exercises existing code only.

---

## 5. Test Execution

```bash
# Run all Ask Rovo tests
cd /home/chaschel/Documents/go/bchat
go test -v -run TestAskRovo ./server/router/api/v1/agent/ -count=1

# Run specific test
go test -v -run TestAskRovo_InferResolutionFromSimilarTickets ./server/router/api/v1/agent/ -count=1

# Run with race detector (critical for Test 9)
go test -v -race -run TestAskRovo ./server/router/api/v1/agent/ -count=1
```

---

## 6. Expected Assertions Summary

| Test | Key Assertions |
|------|---------------|
| 1 (happy path) | `chunks > 0`, `inferred == true`, `InternalNotes` contains "Suggested Resolution" + seed chunk titles |
| 2 (bug sections) | `inferred == true`, `InternalNotes` contains "Relevant Bug History" + bug chunk title |
| 3 (no results) | `result == ""`, `InternalNotes` empty |
| 4 (tenant isolation) | `result == ""` for cross-tenant query |
| 5 (dedup unchanged) | `count(*)` = 1, `version = 1` |
| 6 (dedup changed) | `count(*)` = 2, versions = {1, 2} |
| 7 (comments) | Indexed content blob contains comment text |
| 8 (nil tenant) | Returns `""`, no panic |
| 9 (concurrent) | `count(*)` = 1 after 5 concurrent calls, no race detected, no errors |
| 10 (MinScore) | `result == ""` for non-matching query (below threshold) |
| 11 (TopK) | `result` contains exactly 3 of 5 seeded tickets |

---

## 7. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| `controlledEmbeddingService` doesn't match production behavior | Low | Test validates search logic correctness, not embedding quality. Documented limitation. |
| `NewService` env var side effects | Low | `t.Setenv("RAG_PIPELINE_ENABLED", "false")` prevents LanceDB initialization (Finding 3) |
| `ticketIndexMu` package-level collision between tests | Low | Sequential test execution prevents collision (Finding 7). Unique slugs for debugging. |
| SQLite vs CockroachDB behavior differences | None | `teststore.NewTestingStore` runs full migration suite |
| **Goroutine race conditions not tested** | **Medium** | **Known gap.** Production goroutine in `ticket_service.go:193` is not tested. Concurrency errors in the async path will not be caught. Acceptable for hackathon; must be addressed post-hackathon. |
| `UpsertAgentRAGActiveVersion` store method | None | Real SQLite store handles it |
| `createTestingHostUser` returns error | Low | Verified at `store/test/user_test.go:41` |
| `ContentTypes` filtering not explicitly tested | Low | Binary mock limitation. Filter implicitly exercised by separate search runs. Deferred to Future Work. |
| `IsActive` filtering not tested | None | Not a gap — `InferResolutionForNewTicket` doesn't set `ActiveOnly`, inactive chunks ARE included (current production behavior) |

---

## 8. Future Work

| Item | Description | Priority |
|------|-------------|----------|
| Goroutine path test | Test the `go func()` path in `ticket_service.go:193` with `sync.WaitGroup` polling | HIGH |
| `createSystemResolutionComment` test | Verify system comment is created after inference | HIGH |
| Near-threshold MinScore test | Use a mock that produces cosine similarity=0.69 vs 0.71 to test exact boundary | MEDIUM |
| Error path test | Mock `ReindexFileVersion` failure, assert `IndexTicketContent` returns error | MEDIUM |
| Comment edit re-index | Test `UpdateMemo` hook triggers re-index | MEDIUM |
| `ContentTypes` filtering with type-dependent mock | Mock that produces different embeddings for ticket vs bug_section content | LOW |
| Hybrid search test | Test with `UseHybridSearch: true` and real BM25 scoring | LOW |
| Multi-version retention | Test that version retention (keep last 5) works correctly | LOW |
| LLM suggestion quality | Replace mock with real OpenRouter to test suggestion quality | LOW |

---

## 9. Adversarial Review Prompt

Before implementing, please review this final revised plan critically:

```
You are an adversarial code reviewer. Review this FINAL revised test plan for
the Ask Rovo E2E test suite. The plan was reworked twice based on review
findings. Key changes from Plan 2:

1. Added UID to all CreateMemo calls (Finding 1 — was causing "invalid uid" errors)
2. Fixed Test 9 concurrent require.NoError (Finding 2 — data race on testing.T)
3. Added environment isolation to setupAskRovoTest (Finding 3 — LanceDB init side effects)
4. Added Test 11 for TopK limiting (Finding 4 — TopK=3 never exercised)
5. Documented ContentTypes and mock limitations (Findings 5, 10)
6. Removed IsActive from scope (Finding 6 — not a gap in current production path)
7. Fixed ticketIndexMu comment (Finding 7 — sequential execution is actual safeguard)
8. Documented goroutine gap in Risk Assessment (Finding 9)

Focus on:

1. CORRECTNESS: Do the CreateMemo UID patterns match the store validation regex?
   Will the createTestMemo helper produce unique UIDs across parallel test runs?

2. CONCURRENCY: Is the thread-safe error collection in Test 9 correct? Could
   the mutex contention affect test timing or mask real errors?

3. ENVIRONMENT: Is t.Setenv sufficient to prevent LanceDB initialization?
   Are there other env vars that could cause side effects?

4. TOPK: Does the TopK test correctly verify that only 3 of 5 chunks are
   returned? Could the ordering be non-deterministic with the controlled mock?

5. COVERAGE: Are 11 tests sufficient? What critical paths remain untested?

6. GOROUTINE GAP: Is the documented goroutine coverage gap acceptable for
   the hackathon? What production bugs could hide in the untested async path?

7. MOCK LIMITATIONS: Are the documented limitations (binary vectors, identical
   LOW vectors) acceptable? Could they cause false-positive test results?

8. ASSERTION STRENGTH: Are the assertions strong enough? Should we verify
   the exact number of chunks, or is contains/notContains sufficient?
```
