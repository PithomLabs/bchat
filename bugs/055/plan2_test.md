# Plan 2: Ask Rovo E2E Test (Revised)

**Date:** 2026-07-31
**Status:** Revised based on adversarial review findings
**Review:** `plan_test_review.md` — 10 findings, all addressed
**Related:** [Bug 052](../052/) (per-ticket RAG indexing), [Bug 054](../054/) (tenant association fixes)

---

## 1. Goal

Write a comprehensive integration test suite that proves the Ask Rovo pipeline works correctly — not just that it plumbs, but that the search logic, scoring thresholds, and concurrency controls actually work.

### What Changed from Plan 1

| Finding | Severity | Action Taken |
|---------|----------|--------------|
| 1: Struct references wrong | MEDIUM | Fixed — all references point to `store/ticket.go` |
| 2: `unifiedEmbeddingService` hides bugs | HIGH | **Replaced** with `controlledEmbeddingService` |
| 3: Tests bypass goroutine path | HIGH | Acknowledged — sync tests remain primary, goroutine test added to Future Work |
| 4: Test 2 is skeleton | MEDIUM | **Fleshed out** with full executable code |
| 5: No content-changed test | MEDIUM | **Added** Test 6 |
| 6: No concurrent indexing test | MEDIUM | **Added** Test 9 |
| 7: No MinScore threshold test | MEDIUM | **Added** Test 10 |
| 8: `ticketIndexMu` package-level collision | LOW | **Fixed** — unique IDs per test |
| 9: `createTestingHostUser` verification | LOW | Verified — exists at `store/test/user_test.go:41` |
| 10: Missing error-path tests | LOW | Deferred to Future Work |

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

### 2.2 `controlledEmbeddingService` — Replacing `unifiedEmbeddingService`

**Why:** The `unifiedEmbeddingService` returns `[1.0, 0, 0, ...]` for all texts, making cosine similarity always 1.0. This hides bugs in `MinScore` filtering, `ContentTypes` filtering, and `TopK` limiting. The test would pass even if the search logic were completely broken.

**Design:** A keyword-based embedding service that returns two distinct unit vectors:
- **High vector** (`v[0] = 1.0`): for texts containing any seed keyword
- **Low vector** (`v[1] = 1.0`): for texts without seed keywords

This produces controlled cosine similarities:
- `cosine(high, high) = 1.0` → above `MinScore: 0.7` → **found**
- `cosine(high, low) = 0.0` → below `MinScore: 0.7` → **not found**
- `cosine(low, low) = 1.0` → above `MinScore: 0.5` → found (but low vectors are only used for non-matching queries)

```go
// controlledEmbeddingService returns deterministic embeddings based on keyword presence.
// Texts containing any seed keyword get a "high" vector; others get a "low" vector.
// This ensures cosine similarity is either 1.0 (match) or 0.0 (no match),
// exercising the MinScore threshold in MemoryVectorDB.Search.
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

**Key properties:**
- Same text always produces the same vector (deterministic)
- Similar texts (sharing keywords) produce identical vectors (cosine = 1.0)
- Dissimilar texts produce orthogonal vectors (cosine = 0.0)
- `MinScore: 0.7` is exercised: matching queries pass, non-matching queries fail
- `ContentTypes` filter is exercised: only chunks with matching ContentType are returned
- `TenantID` filter is exercised: only chunks with matching TenantID are returned

### 2.3 Service Setup Helper

```go
var testTenantIDCounter int32 = 100

func setupAskRovoTest(t *testing.T, seedKeywords []string) (
    ctx context.Context,
    ts *store.Store,
    svc *Service,
    tenant *store.AgentTenant,
    user *store.User,
) {
    t.Helper()
    ctx = context.Background()
    ts = teststore.NewTestingStore(ctx, t)
    t.Cleanup(func() { ts.Close() })

    user, err := createTestingHostUser(ctx, ts)
    require.NoError(t, err)

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

### 2.4 Seed Data

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

    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Test", Visibility: store.Public, TenantID: &tenant.ID,
    })
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Indexing Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })

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

    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Test", Visibility: store.Public, TenantID: &tenant.ID,
    })
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Index Gap Fix", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })

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

    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Test", Visibility: store.Public, TenantID: &tenant.ID,
    })
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Unrelated Bug", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })

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

    tenant2, _ := ts.CreateAgentTenant(ctx, &store.AgentTenant{
        Slug: "ask-rovo-isolation", CompanyName: "Isolation Test",
        Vertical: "test", IsActive: true,
    })

    svc.vectorDB.Insert(ctx, []DocumentChunk{
        {ID: "seed_t1", TenantID: tenant1.ID, AudienceType: "internal",
         ContentType: "ticket", Title: "Tenant 1 Content",
         Content: "RAG indexing for tenant 1", IsActive: true},
    })

    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Test", Visibility: store.Public, TenantID: &tenant2.ID,
    })
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Indexing for Tenant 2", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant2.ID,
    })

    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.Equal(t, "", result)
}
```

### Test 5: `TestAskRovo_ContentHashDedup`

**Unchanged content dedup.** Proves: indexing same content twice does NOT create a second version row.

```go
func TestAskRovo_ContentHashDedup(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Test", Visibility: store.Public, TenantID: &tenant.ID,
    })
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Dedup Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })

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

    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Test", Visibility: store.Public, TenantID: &tenant.ID,
    })
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Version Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })

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

    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Parent memo", Visibility: store.Public, TenantID: &tenant.ID,
    })
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Comment Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })

    comment1, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "First comment about RAG indexing",
        Visibility: store.Public, TenantID: &tenant.ID,
    })
    comment2, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Second comment about ticket indexing",
        Visibility: store.Public, TenantID: &tenant.ID,
    })
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

```go
func TestAskRovo_ConcurrentIndexing(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Test", Visibility: store.Public, TenantID: &tenant.ID,
    })
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Concurrent Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })

    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, _, err := svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, false)
            require.NoError(t, err)
        }()
    }
    wg.Wait()

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

    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Test", Visibility: store.Public, TenantID: &tenant.ID,
    })
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Fire Damage Repair", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })

    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.Equal(t, "", result)
}
```

---

## 4. Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `server/router/api/v1/agent/ticket_rag_inference_test.go` | **CREATE** | 10 test cases + `controlledEmbeddingService` + `setupAskRovoTest` helper |

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
| 9 (concurrent) | `count(*)` = 1 after 5 concurrent calls, no race detected |
| 10 (MinScore) | `result == ""` for non-matching query (below threshold) |

---

## 7. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| `controlledEmbeddingService` doesn't match production behavior | Low | Test validates search logic correctness, not embedding quality |
| `NewService` env var side effects | Low | Use `t.Setenv("RAG_PIPELINE_ENABLED", "false")` |
| `ticketIndexMu` package-level collision between tests | Low | Unique tenant IDs (100+counter) per test via `atomic.AddInt32` |
| SQLite vs CockroachDB behavior differences | None | `teststore.NewTestingStore` runs full migration suite |
| Goroutine race conditions not tested | Medium | Synchronous tests are primary; goroutine test deferred to Future Work |
| `UpsertAgentRAGActiveVersion` store method | None | Real SQLite store handles it |
| `createTestingHostUser` returns error | Low | Verified at `store/test/user_test.go:41` |

---

## 8. Future Work

| Item | Description | Priority |
|------|-------------|----------|
| Goroutine path test | Test the `go func()` path in `ticket_service.go:193` with `sync.WaitGroup` polling | HIGH |
| `createSystemResolutionComment` test | Verify system comment is created after inference | HIGH |
| Error path test | Mock `ReindexFileVersion` failure, assert `IndexTicketContent` returns error | MEDIUM |
| Comment edit re-index | Test `UpdateMemo` hook triggers re-index | MEDIUM |
| Hybrid search test | Test with `UseHybridSearch: true` and real BM25 scoring | LOW |
| Multi-version retention | Test that version retention (keep last 5) works correctly | LOW |
| LLM suggestion quality | Replace mock with real OpenRouter to test suggestion quality | LOW |

---

## 9. Adversarial Review Prompt

Before implementing, please review this revised plan critically:

```
You are an adversarial code reviewer. Review this REVISED test plan for the
Ask Rovo E2E test suite. The plan was reworked based on 10 findings from the
original review. Key changes:

1. Replaced `unifiedEmbeddingService` with `controlledEmbeddingService`
   (keyword-based, exercises MinScore threshold)
2. Fleshed out Test 2 (bug sections) with full executable code
3. Added Test 6 (content-changed path), Test 9 (concurrent indexing),
   Test 10 (MinScore threshold)
4. Added `setupAskRovoTest` helper with unique tenant IDs per test
5. Kept synchronous tests as primary, goroutine test deferred to Future Work

Focus on:

1. CORRECTNESS: Does the `controlledEmbeddingService` correctly exercise
   MinScore, ContentTypes, TenantID, and TopK filters? Are there edge cases
   where the keyword matching produces wrong results?

2. COVERAGE: Are the 10 tests sufficient to prove the pipeline works?
   What critical paths are still untested?

3. TEST ISOLATION: Do the unique tenant IDs (100+counter) actually prevent
   `ticketIndexMu` collisions? Are there other shared state issues?

4. ASSERTION STRENGTH: Are the assertions strong enough to catch real bugs?
   Should we assert more specific conditions?

5. PRODUCTION READINESS: Would these tests catch bugs that would manifest
   in production (CockroachDB, real embeddings, concurrent requests)?

6. GOROUTINE PATH: Is deferring the goroutine test to Future Work acceptable
   for the hackathon, or is it critical for correctness?

7. SEED DATA: Is the seed data realistic enough? Should we use actual
   bugs/052 content instead of synthetic text?

8. MINSCORE BOUNDARY: Test 10 tests "below threshold" (cosine=0.0). Should
   we also test "near threshold" (cosine=0.69 vs 0.71)?
```
