# Plan: Ask Rovo E2E Test — Per-Ticket RAG Inference

**Date:** 2026-07-31
**Status:** Draft — Awaiting approval before implementation
**Related:** [Bug 052](../052/) (per-ticket RAG indexing), [Bug 054](../054/) (tenant association fixes)

---

## 1. Goal

Write a comprehensive integration test that proves the Ask Rovo pipeline works end-to-end:

1. Seed the vector DB with historical ticket content
2. Create a new ticket with similar content
3. `IndexTicketContent` indexes the new ticket
4. `InferResolutionForNewTicket` searches for similar tickets, finds seeds, populates `InternalNotes`
5. Assert the generated suggestions are correct

This is a **hackathon deliverable** — the test must pass against the current codebase with no additional production code changes.

---

## 2. Test Infrastructure

### 2.1 Components Used

| Component | Source | Purpose |
|-----------|--------|---------|
| `teststore.NewTestingStore(ctx, t)` | `store/test/store.go:24` | Real SQLite DB with all migrations |
| `NewService(ts, profile)` | `service.go:89` | Creates Service with chunker initialized |
| `NewMemoryVectorDB(embedSvc)` | `vectordb.go:308` | In-memory VectorDB (full interface) |
| `NewChunker()` | `chunker.go:60` | Document chunking |
| `createTestingHostUser()` | `store/test/user_test.go:41` | Creates a HOST user for ticket creator FK |
| `unifiedEmbeddingService` | **new** (test file) | Deterministic mock embedding |

### 2.2 Unified Embedding Service (new, test-only)

**Problem:** `InferResolutionForNewTicket` uses `MinScore: 0.7` for ticket search. The existing `mockEmbeddingService` (hash-based) produces different vectors for different texts, so cosine similarity may fall below 0.7, causing false negatives.

**Solution:** A `unifiedEmbeddingService` that returns the **same unit vector** for all texts. This guarantees cosine similarity = 1.0 for all chunk pairs, ensuring the MinScore threshold is always met. The test validates pipeline plumbing (indexing → search → inference → write-back), not embedding quality.

```go
type unifiedEmbeddingService struct{ dimension int }

func (u *unifiedEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    result := make([][]float32, len(texts))
    for i := range texts {
        v := make([]float32, u.dimension)
        v[0] = 1.0 // unit vector along first axis
        result[i] = v
    }
    return result, nil
}

func (u *unifiedEmbeddingService) Dimension() int            { return u.dimension }
func (u *unifiedEmbeddingService) Provider() string          { return "unified-test" }
func (u *unifiedEmbeddingService) MaxInputTokens() int       { return 8192 }
```

### 2.3 Service Setup Pattern

After `NewService(ts, profile)`, we replace the vectorDB and chunker:

```go
svc := NewService(ts, &profile.Profile{Driver: "sqlite", Mode: "prod"})
svc.vectorDB = NewMemoryVectorDB(&unifiedEmbeddingService{dimension: 8})
svc.vectorDBMu.Lock()
defer svc.vectorDBMu.Unlock()
```

This is safe because the test file is in `package agent` (same package as `Service`), and `vectorDB` is an exported struct field.

### 2.4 Seed Data

Use actual bugs/052 content (truncated for test readability):

**Seed chunk 1 (ContentType="ticket"):**
```
Title: "Per-Ticket RAG Indexing"
Content: "Per-ticket RAG indexing for Jira-style Ask Rovo feature. Each ticket's
content is indexed into the vector DB so that InferResolutionForNewTicket can
surface similar historical tickets and bug sections when a new ticket is created."
```

**Seed chunk 2 (ContentType="ticket"):**
```
Title: "IndexTicketContent Helper"
Content: "IndexTicketContent builds a markdown blob from title + description +
comments, computes ContentHash, queries latest AgentSourceFile, skips upsert
if hash matches, otherwise upserts and calls ReindexFileVersion."
```

**Seed chunk 3 (ContentType="bug_section"):**
```
Title: "Bug 052 — RAG Index Gap"
Content: "InferResolutionForNewTicket searches for ContentTypes ticket but no
code ever indexes ticket content into the vector DB. The search always returns
empty. This is the missing piece that per-ticket indexing resolves."
```

---

## 3. Test Cases

### Test 1: `TestAskRovo_InferResolutionFromSimilarTickets`

**Happy path.** Proves the full pipeline: seed → index → infer → write-back.

```go
func TestAskRovo_InferResolutionFromSimilarTickets(t *testing.T) {
    ctx := context.Background()
    ts := teststore.NewTestingStore(ctx, t)
    defer ts.Close()

    // Setup
    user, _ := createTestingHostUser(ctx, ts)
    tenant, _ := ts.CreateAgentTenant(ctx, &store.AgentTenant{
        Slug: "ask-rovo-test", CompanyName: "Test", Vertical: "test", IsActive: true,
    })
    svc := NewService(ts, &profile.Profile{Driver: "sqlite", Mode: "prod"})
    embedSvc := &unifiedEmbeddingService{dimension: 8}
    svc.vectorDB = NewMemoryVectorDB(embedSvc)

    // Seed vector DB with 2 historical ticket chunks
    svc.vectorDB.Insert(ctx, []DocumentChunk{
        {
            ID: "seed_ticket_1", TenantID: tenant.ID, AudienceType: "internal",
            ContentType: "ticket", Title: "Per-Ticket RAG Indexing",
            Content: "Per-ticket RAG indexing for Jira-style Ask Rovo feature...",
            IsActive: true,
        },
        {
            ID: "seed_ticket_2", TenantID: tenant.ID, AudienceType: "internal",
            ContentType: "ticket", Title: "IndexTicketContent Helper",
            Content: "IndexTicketContent builds a markdown blob...",
            IsActive: true,
        },
    })

    // Create memo (ticket Description must be "/m/<uid>")
    memo, _ := ts.CreateMemo(ctx, &store.Memo{
        CreatorID: user.ID, Content: "Test ticket", Visibility: store.Public,
        TenantID: &tenant.ID,
    })

    // Create ticket
    ticket, _ := ts.CreateTicket(ctx, &store.Ticket{
        Title: "RAG Indexing Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })

    // Index + Infer (synchronous)
    chunks, inferred, err := svc.IndexTicketContent(ctx, tenant.ID, ticket, nil, true)
    require.NoError(t, err)
    require.Greater(t, chunks, 0)
    require.True(t, inferred)

    // Verify internal_notes was populated
    updated, _ := ts.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
    require.Contains(t, updated.InternalNotes, "## Suggested Resolution (Auto-generated)")
    require.Contains(t, updated.InternalNotes, "Per-Ticket RAG Indexing")
}
```

### Test 2: `TestAskRovo_InferResolutionFromBugSections`

Proves bug_section search (search 2) works independently of ticket search (search 1).

```go
func TestAskRovo_InferResolutionFromBugSections(t *testing.T) {
    // Seed only bug_section chunks (no ticket chunks)
    // Create ticket with similar content
    // Assert: internal_notes contains "## Relevant Bug History" section
    // Assert: internal_notes contains bug chunk titles
}
```

### Test 3: `TestAskRovo_NoResultsReturnsEmpty`

Proves graceful handling when vector DB has no matching content.

```go
func TestAskRovo_NoResultsReturnsEmpty(t *testing.T) {
    // Empty vector DB (no seeds)
    // Create ticket
    // Call InferResolutionForNewTicket
    // Assert: returns ""
    // Assert: internal_notes remains empty
}
```

### Test 4: `TestAskRovo_TenantIsolation`

Proves cross-tenant data is not leaked.

```go
func TestAskRovo_TenantIsolation(t *testing.T) {
    // Seed chunks for tenant_id=1
    // Create ticket for tenant_id=2
    // Call InferResolutionForNewTicket
    // Assert: returns "" (no results from tenant 1)
}
```

### Test 5: `TestAskRovo_ContentHashDedup`

Proves content-hash dedup prevents version inflation.

```go
func TestAskRovo_ContentHashDedup(t *testing.T) {
    // Create ticket, call IndexTicketContent twice with same content
    // Assert: only 1 AgentSourceFile row created (not 2)
    // Assert: version = 1 (not 2)
}
```

### Test 6: `TestAskRovo_IndexTicketContentWithComments`

Proves comments are included in the indexed content blob.

```go
func TestAskRovo_IndexTicketContentWithComments(t *testing.T) {
    // Create ticket + 2 comment memos
    // Call IndexTicketContent with comments
    // Assert: chunks > 0
    // Fetch the AgentSourceFile content, assert it contains comment text
}
```

### Test 7: `TestAskRovo_NilTenantSkipsInference`

Proves nil TenantID is handled gracefully (no panic).

```go
func TestAskRovo_NilTenantSkipsInference(t *testing.T) {
    ticket := &store.Ticket{ID: 1, Title: "Test", TenantID: nil}
    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.Equal(t, "", result)
}
```

---

## 4. Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `server/router/api/v1/agent/ticket_rag_inference_test.go` | **CREATE** | All 7 test cases + `unifiedEmbeddingService` |

**No production code changes.** This test exercises existing code only.

---

## 5. Test Execution

```bash
# Run all Ask Rovo tests
cd /home/chaschel/Documents/go/bchat
go test -v -run TestAskRovo ./server/router/api/v1/agent/ -count=1

# Run specific test
go test -v -run TestAskRovo_InferResolutionFromSimilarTickets ./server/router/api/v1/agent/ -count=1

# Run with race detector
go test -v -race -run TestAskRovo ./server/router/api/v1/agent/ -count=1
```

---

## 6. Expected Assertions Summary

| Test | Key Assertions |
|------|---------------|
| 1 (happy path) | `chunks > 0`, `inferred == true`, `InternalNotes` contains "Suggested Resolution" + seed chunk titles |
| 2 (bug sections) | `InternalNotes` contains "Relevant Bug History" + bug chunk titles |
| 3 (no results) | `result == ""`, `InternalNotes` empty |
| 4 (tenant isolation) | `result == ""` for cross-tenant query |
| 5 (dedup) | `count(*)` of source files = 1 after 2 index calls |
| 6 (comments) | Indexed content blob contains comment text |
| 7 (nil tenant) | Returns `""`, no panic |

---

## 7. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| `unifiedEmbeddingService` doesn't match production behavior | Low | Test validates plumbing, not embedding quality |
| `NewService` env var side effects | Low | Test sets `RAG_PIPELINE_ENABLED=false` via `t.Setenv` |
| SQLite migration differences | None | `teststore.NewTestingStore` runs full migration suite |
| Goroutine races in `IndexTicketContent` | Low | Test uses synchronous calls (not goroutine path) |
| `UpsertAgentRAGActiveVersion` store method | None | Real SQLite store handles it |

---

## 8. Future Work

| Item | Description |
|------|-------------|
| Async goroutine test | Test the `go func()` path in `ticket_service.go:193` |
| LLM-generated suggestions | Replace mock with real OpenRouter to test suggestion quality |
| Comment edit re-index | Test `UpdateMemo` hook triggers re-index |
| Hybrid search test | Test with `UseHybridSearch: true` and real BM25 scoring |
| Multi-version retention | Test that version retention (keep last 5) works correctly |

---

## 9. Adversarial Review Prompt

Before implementing, please review this test plan critically:

```
You are an adversarial code reviewer. Review this test plan for the Ask Rovo
E2E test suite. The plan has 7 test cases that verify the per-ticket RAG
inference pipeline end-to-end using SQLite + MemoryVectorDB + unified embedding.

Focus on:

1. COVERAGE GAPS: Are there important edge cases missing from the 7 tests?
   Consider: concurrent indexing, race conditions between IndexTicketContent
   and InferResolutionForNewTicket, version retention cleanup, error paths
   in UpsertAgentSourceFile.

2. TEST ISOLATION: Does the unifiedEmbeddingService (same vector for all texts)
   hide real bugs? Could a production deployment with actual embeddings behave
   differently than the test predicts?

3. ASSERTION STRENGTH: Are the assertions too weak (just checking contains)?
   Should we assert exact structure of internal_notes? Should we verify the
   vector DB state after indexing?

4. SQLITE-SPECIFIC ISSUES: Could behavior differ between SQLite (test) and
   CockroachDB (production)? Are there SQL dialect issues that the test
   would miss?

5. GOROUTINE PATHS: The plan tests synchronous calls only. The production
   code runs IndexTicketContent in a goroutine (ticket_service.go:193).
   What bugs could hide in the async path that the sync test won't catch?

6. MOCK FIDELITY: Does NewMemoryVectorDB accurately represent LanceDB behavior?
   Are there LanceDB-specific features (ANN search, HNSW indexing) that
   MemoryVectorDB doesn't simulate?

7. DEPENDENCY CHAIN: The test depends on teststore.NewTestingStore running
   all migrations. If a migration adds a column or constraint that breaks
   IndexTicketContent, would this test catch it?

8. MINSCORE THRESHOLD: The unified embedding guarantees similarity=1.0, but
   production embeddings may score 0.7-0.9 for similar content. Should we
   add a test with controlled similarity scores near the threshold?
```
