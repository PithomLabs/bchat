# Adversarial Test Plan Review: Ask Rovo E2E Test

**Bug/Task:** Plan for `plan_ask_rovo_e2e_test.md`  
**Reviewer:** Kilo (Senior Go Architect)  
**Date:** 2026-07-31  
**Verdict:** REWORK REQUIRED — plan has correct intent but contains factual errors about the codebase, misses critical production paths, and uses a mock that hides real search-logic bugs.

---

## Executive Summary

The plan correctly identifies the goal (prove the Ask Rovo pipeline works end-to-end) and the right test infrastructure (`teststore`, `MemoryVectorDB`, deterministic embeddings). However, it contains factual inaccuracies about struct locations, bypasses the actual production code paths (goroutine indexing, HTTP handler, system comment creation), and uses a mock that makes `MinScore` threshold testing impossible. Several test cases are incomplete or test the wrong behavior.

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | Struct references are wrong (`store/agent.go` vs `store/ticket.go`) | MEDIUM | Fix |
| 2 | `unifiedEmbeddingService` hides all search-logic bugs (similarity always 1.0) | HIGH | Rework |
| 3 | Tests bypass the production goroutine path entirely | HIGH | Rework |
| 4 | Test 2 is a skeleton, not an executable test case | MEDIUM | Fix |
| 5 | No test for content-changed path (version increment) | MEDIUM | Add |
| 6 | No test for concurrent `IndexTicketContent` calls | MEDIUM | Add |
| 7 | No test for `MinScore` threshold boundary | MEDIUM | Add |
| 8 | `ticketIndexMu` is package-level — tests may interfere | LOW | Fix |
| 9 | `createTestingHostUser` path needs verification | LOW | Verify |
| 10 | Missing error-path tests for `ReindexFileVersion` failure | LOW | Add |

---

## Finding 1: Struct References Are Wrong

**Severity:** MEDIUM

The plan repeatedly references `store.Ticket` as living in `store/agent.go`. It is actually in `store/ticket.go:24-38`.

```go
// Actual location: store/ticket.go:24-38
type Ticket struct {
    ID          int32
    Title       string
    Description string
    Status      TicketStatus
    Priority    TicketPriority
    CreatorID   int32
    AssigneeID  *int32
    CreatedTs   int64
    UpdatedTs   int64
    Type        string
    Tags        []string
    TenantID    *int32
    InternalNotes string
}
```

Similarly, `FindTicket` is in `store/ticket.go:40-48` with `ID *int32` (not `*string`).

**Impact:** Any developer following the plan will look in the wrong file. Low functional impact since the test code uses the correct import path anyway.

**Required fix:** Update all references from `store/agent.go` to `store/ticket.go`.

---

## Finding 2: `unifiedEmbeddingService` Hides All Search-Logic Bugs

**Severity:** HIGH

The `unifiedEmbeddingService` returns the **exact same unit vector** (`[1.0, 0, 0, ...]`) for every input text. This means:

- Cosine similarity between ANY two texts = 1.0
- `MinScore` threshold (0.7 for tickets, 0.5 for bug_sections) is **always met**
- The `MinScore` filter in `MemoryVectorDB.Search` is **never exercised**
- Any bug in score comparison (`>` vs `>=`), threshold values, or filtering logic is **completely masked**

**What the test actually validates:** Only that chunks are inserted, search returns them, and `InternalNotes` is populated. It does NOT validate that the search respects `MinScore`, `TopK`, `ContentTypes`, or `TenantID` filters.

**Example bugs this test would miss:**
1. `MinScore` is accidentally set to 2.0 (impossible to reach) → test still passes because similarity is always 1.0
2. `ContentTypes` filter is broken → test still passes because all inserted chunks have matching content type
3. `TenantID` filter is broken → test still passes because all seeds use the same tenant ID
4. `TopK` limit is broken → test only inserts 2 chunks, so limit is never exceeded

**Required rework:** Use a **deterministic but distinguishable** embedding service. For example:

```go
type deterministicEmbeddingService struct {
    dimension int
}

func (d *deterministicEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    result := make([][]float32, len(texts))
    for i, text := range texts {
        // Hash the text to a deterministic vector in [0, 1] range
        h := fnv.New32a()
        h.Write([]byte(text))
        hash := h.Sum32()
        v := make([]float32, d.dimension)
        for j := 0; j < d.dimension; j++ {
            // Spread hash across dimensions, normalize to [0.3, 1.0] to ensure
            // similar texts score above 0.7 but dissimilar texts score below
            v[j] = 0.3 + float32((hash>>j)%100)/100.0 * 0.7
        }
        // Normalize to unit vector
        norm := float32(0.0)
        for j := 0; j < d.dimension; j++ {
            norm += v[j] * v[j]
        }
        norm = float32(math.Sqrt(float64(norm)))
        for j := 0; j < d.dimension; j++ {
            v[j] /= norm
        }
        result[i] = v
    }
    return result, nil
}
```

This ensures:
- Same text always produces the same vector
- Different texts produce different vectors
- Similar texts (shared words) have higher cosine similarity
- The `MinScore` threshold is actually exercised

Alternatively, if the hackathon constraint truly requires guaranteed passes, add a **separate** test with controlled scores near the threshold:

```go
func TestAskRovo_MinScoreThreshold(t *testing.T) {
    // Use a mock embedding service that returns vectors with
    // cosine similarity exactly 0.69 (below MinScore 0.7)
    // Assert: no results returned
}
```

---

## Finding 3: Tests Bypass the Production Goroutine Path

**Severity:** HIGH

The production code in `ticket_service.go:193-209` runs `IndexTicketContent` in a **goroutine**:

```go
if s.agentHandler != nil && ticket.TenantID != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        _, _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, true)
        if err != nil {
            slog.Error("failed to index new ticket for RAG", "ticket_id", ticket.ID, "error", err)
            return
        }

        // Fetch updated ticket to read populated internal_notes
        updated, fetchErr := s.Store.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
        if fetchErr != nil || updated == nil || updated.InternalNotes == "" {
            return
        }

        // Create system-authored comment with the inferred resolution
        suggestion := updated.InternalNotes
        if commentErr := s.createSystemResolutionComment(ctx, *ticket.TenantID, ticket, suggestion); commentErr != nil {
            slog.Error("failed to create system resolution comment", "error", commentErr)
        }
    }()
}
```

The test calls `svc.IndexTicketContent` **directly and synchronously**. This means:

1. **Goroutine race conditions are never tested**: If `IndexTicketContent` has race conditions in the goroutine path, the test won't catch them.
2. **`createSystemResolutionComment` is never tested**: The production code creates a system comment after inference. The test only checks `InternalNotes`.
3. **`context.WithoutCancel` behavior is never tested**: The goroutine uses a detached context. If the HTTP request is canceled, the goroutine continues. The test doesn't simulate request cancellation.
4. **Error handling in goroutine is never tested**: Errors are only logged, not returned. The test can't verify that the goroutine handles errors gracefully.

**Required rework:** Add a test that exercises the goroutine path through the actual `CreateTicket` handler, or at minimum:

1. Test `IndexTicketContent` with `triggerInference: true` and verify the goroutine completes
2. Use a `sync.WaitGroup` or polling to wait for `InternalNotes` to be populated
3. Test that `createSystemResolutionComment` is called (or mock it)
4. Test cancellation: create a context, cancel it, and verify the goroutine still completes (since it uses `context.WithoutCancel`)

---

## Finding 4: Test 2 Is a Skeleton, Not an Executable Test

**Severity:** MEDIUM

The plan shows Test 2 as comments only:

```go
func TestAskRovo_InferResolutionFromBugSections(t *testing.T) {
    // Seed only bug_section chunks (no ticket chunks)
    // Create ticket with similar content
    // Assert: internal_notes contains "## Relevant Bug History" section
    // Assert: internal_notes contains bug chunk titles
}
```

**Impact:** The reviewer cannot assess coverage or assertion strength. The test is not implementation-ready.

**Required fix:** Write out the full test case with actual code, or remove it from the plan and add it as a follow-up.

---

## Finding 5: No Test for Content-Changed Path (Version Increment)

**Severity:** MEDIUM

`IndexTicketContent` has two distinct paths:

1. **Unchanged content** (`existing.ContentHash == contentHash`): Calls `ReindexFileVersion`, does NOT create a new `AgentSourceFile` row
2. **Changed content** (`existing == nil || hashes differ`): Calls `UpsertAgentSourceFile` which **creates a new version row**

Test 5 (`TestAskRovo_ContentHashDedup`) only tests path 1 (unchanged content). It never tests path 2.

**Required addition:** Add a test that:

1. Creates a ticket and indexes it (version 1 created)
2. Updates the ticket content (or creates a different ticket with different content)
3. Calls `IndexTicketContent` again
4. Asserts that a **second** `AgentSourceFile` row was created with `version = 2`

---

## Finding 6: No Test for Concurrent `IndexTicketContent` Calls

**Severity:** MEDIUM

The `ticketIndexMu` is a package-level `sync.Map` designed to prevent TOCTOU races:

```go
var ticketIndexMu sync.Map

func (s *Service) IndexTicketContent(...) {
    muKey := fmt.Sprintf("%d:%d", tenantID, ticket.ID)
    muVal, _ := ticketIndexMu.LoadOrStore(muKey, &sync.Mutex{})
    mu := muVal.(*sync.Mutex)
    mu.Lock()
    defer mu.Unlock()
    // ...
}
```

The test never exercises concurrent calls. If there's a bug in the mutex key generation or the `sync.Map` usage, the test won't catch it.

**Required addition:** Add a test that:

1. Calls `IndexTicketContent` concurrently for the same ticket (e.g., 5 goroutines)
2. Asserts that only one version is created (no duplicate rows)
3. Asserts no panic or data race occurs

Run with `-race` flag.

---

## Finding 7: No Test for `MinScore` Threshold Boundary

**Severity:** MEDIUM

With the current `unifiedEmbeddingService`, similarity is always 1.0. The test never verifies what happens when similarity is near or below the threshold.

**Required addition:** Add a test with a **controllable** embedding service that returns vectors with known cosine similarities:

```go
func TestAskRovo_MinScoreThreshold(t *testing.T) {
    // Embedding service that returns vectors with cosine similarity = 0.69
    // (below ticket search MinScore of 0.7)
    svc := setupServiceWithEmbedding(&thresholdEmbeddingService{
        ticketSimilarity: 0.69,  // below 0.7
        bugSimilarity:    0.49,  // below 0.5
    })
    
    // Seed chunks, create ticket, call InferResolutionForNewTicket
    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.Equal(t, "", result)  // no results below threshold
}
```

---

## Finding 8: `ticketIndexMu` Is Package-Level — Tests May Interfere

**Severity:** LOW

`ticketIndexMu` is a `sync.Map` at the package level in `service.go`. If tests run in parallel (Go test parallelism), they may share mutex state.

**Required fix:** Add `t.Parallel()` awareness or reset `ticketIndexMu` in test cleanup. Since `sync.Map` cannot be easily reset, use unique tenant/ticket IDs per test to avoid key collisions.

**Recommendation:** Use tenant IDs like `100 + testIndex` and ticket IDs like `1000 + testIndex` to ensure uniqueness.

---

## Finding 9: `createTestingHostUser` Path Needs Verification

**Severity:** LOW

The plan references `createTestingHostUser(ctx, ts)` from `store/test/user_test.go:41`. This function may not exist, or may have a different signature.

**Required verification:** Read `store/test/user_test.go` and confirm the function exists and returns `(*store.User, error)`.

---

## Finding 10: Missing Error-Path Tests for `ReindexFileVersion` Failure

**Severity:** LOW

If `ReindexFileVersion` fails, `IndexTicketContent` returns an error. The test never simulates this failure.

**Required addition:** Mock `ReindexFileVersion` to return an error and assert that `IndexTicketContent` returns the error.

---

## Behavioral Correctness Concerns

### Mock Fidelity vs. Production

| Aspect | `unifiedEmbeddingService` | Production (OpenRouter) |
|--------|---------------------------|------------------------|
| Cosine similarity for any text pair | 1.0 (always) | 0.0-1.0 (varies) |
| `MinScore` filtering | Never exercised | Critical filter |
| `TopK` limiting | Never exercised (only 2 chunks) | Critical for large corpora |
| Hybrid search | Not tested | `HYBRID_SEARCH_ENABLED` affects scoring |

**Conclusion:** The test validates that the pipeline "plumbs" correctly, but does NOT validate that the search logic works correctly. This is a **unit test masquerading as an integration test**.

### Production Goroutine Path

The production `ticket_service.go` goroutine does the following after indexing:
1. Fetches the updated ticket
2. Checks if `InternalNotes` is non-empty
3. Calls `createSystemResolutionComment` to create a system-authored comment

The test does NONE of this. It only checks `InternalNotes` directly.

---

## Recommended Rework

### 1. Replace `unifiedEmbeddingService` with Deterministic Embeddings

Use a deterministic embedding service that produces **different** vectors for different texts, with cosine similarity in the range [0.3, 1.0]. This ensures:
- `MinScore` is actually exercised
- Similar texts score higher than dissimilar texts
- Tests remain deterministic and fast

### 2. Add Goroutine-Path Test

Create a test that exercises the full `CreateTicket` handler path (or at minimum, the goroutine in `ticket_service.go`). Use a `sync.WaitGroup` or polling to wait for completion.

### 3. Complete Test 2

Write out the full `TestAskRovo_InferResolutionFromBugSections` test with actual code.

### 4. Add Missing Test Cases

| Test | Purpose |
|------|---------|
| `TestAskRovo_ContentChangedCreatesNewVersion` | Verifies version increment on content change |
| `TestAskRovo_ConcurrentIndexing` | Verifies `ticketIndexMu` correctness under concurrency |
| `TestAskRovo_MinScoreThreshold` | Verifies results below `MinScore` are filtered |
| `TestAskRovo_ReindexFailure` | Verifies error propagation from `ReindexFileVersion` |

### 5. Fix Struct References

Update all `store/agent.go` references to `store/ticket.go`.

### 6. Use Unique IDs Per Test

To avoid `ticketIndexMu` collisions, use tenant/ticket IDs that are unique per test (e.g., tenant 100+test index).

---

## What Is Correct

| Check | Status | Notes |
|-------|--------|-------|
| Test infrastructure choice | CORRECT | `teststore.NewTestingStore` + `MemoryVectorDB` is the right approach |
| Seed data design | CORRECT | Uses real bug content, truncated for readability |
| Tenant isolation test | CORRECT | Cross-tenant query returns empty |
| Nil tenant test | CORRECT | `TenantID == nil` returns empty string |
| Content hash dedup test (unchanged path) | CORRECT | Verifies no duplicate version is created |
| Comments inclusion test | CORRECT | Verifies comments appear in indexed content |

---

## Final Verdict

**REWORK REQUIRED.** The plan has the right architecture but contains factual errors and critical gaps that would produce a test suite that passes regardless of whether the search logic is correct. The two highest-priority changes are:

1. **Replace `unifiedEmbeddingService` with a deterministic embedding service** that exercises `MinScore` filtering
2. **Add a test for the production goroutine path** through `CreateTicket` handler or `ticket_service.go`

Without these changes, the test suite is a "smoke test" that validates plumbing but not correctness.
