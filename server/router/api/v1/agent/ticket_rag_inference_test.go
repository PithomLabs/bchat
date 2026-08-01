// WARNING: Tests in this file must NOT use t.Parallel() because
// ticketIndexMu and memoUIDCounter are package-level shared state
// that rely on sequential execution for uniqueness.
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

// ============================================================================
// TEST HELPERS
// ============================================================================

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
	t.Setenv("TICKET_EMBEDDING_ENABLED", "false") // Prevent background goroutine

	ctx = context.Background()
	ts = teststore.NewTestingStore(ctx, t)
	t.Cleanup(func() { ts.Close() })

	var err error
	user, err = ts.CreateUser(ctx, &store.User{
		Username: fmt.Sprintf("ask-rovo-user-%d", atomic.LoadInt32(&testTenantIDCounter)),
		Role:     store.RoleHost,
	})
	require.NoError(t, err)

	// Capture atomic return value for uniqueness
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

var memoUIDCounter int32 = 0

// createTestMemo creates a memo with a unique UID for testing.
// Accepts ctx instead of using context.Background() for future-proofing.
func createTestMemo(t *testing.T, ctx context.Context, ts *store.Store, user *store.User, tenant *store.AgentTenant, content string) *store.Memo {
	t.Helper()
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

// ============================================================================
// TEST CASES
// ============================================================================

func TestAskRovo_InferResolutionFromSimilarTickets(t *testing.T) {
	ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	svc.vectorDB.Insert(ctx, []DocumentChunk{
		{ID: "seed_ticket_1", TenantID: tenant.ID, AudienceType: "internal",
			ContentType: "ticket_section", Title: "Per-Ticket RAG Indexing",
			Content:     "Per-ticket RAG indexing for Jira-style Ask Rovo feature...",
			IsActive:    true},
		{ID: "seed_ticket_2", TenantID: tenant.ID, AudienceType: "internal",
			ContentType: "ticket_section", Title: "IndexTicketContent Helper",
			Content:     "IndexTicketContent builds a markdown blob...",
			IsActive:    true},
	})

	memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
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

func TestAskRovo_InferResolutionFromBugSections(t *testing.T) {
	ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	svc.vectorDB.Insert(ctx, []DocumentChunk{
		{ID: "seed_bug_1", TenantID: tenant.ID, AudienceType: "internal",
			ContentType: "bug_section", Title: "Bug 052 — RAG Index Gap",
			Content:     "InferResolutionForNewTicket searches for ContentTypes ticket but no code ever indexes ticket content into the vector DB...",
			IsActive:    true},
	})

	memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
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

func TestAskRovo_NoResultsReturnsEmpty(t *testing.T) {
	ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
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

func TestAskRovo_TenantIsolation(t *testing.T) {
	ctx, ts, svc, tenant1, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	tenant2, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug: "ask-rovo-isolation", CompanyName: "Isolation Test",
		Vertical: "test", IsActive: true,
	})
	require.NoError(t, err)

	svc.vectorDB.Insert(ctx, []DocumentChunk{
		{ID: "seed_t1", TenantID: tenant1.ID, AudienceType: "internal",
			ContentType: "ticket_section", Title: "Tenant 1 Content",
			Content: "RAG indexing for tenant 1", IsActive: true},
	})

	memo := createTestMemo(t, ctx, ts, user, tenant2, "Test")
	ticket, err := ts.CreateTicket(ctx, &store.Ticket{
		Title: "RAG Indexing for Tenant 2", Description: "/m/" + memo.UID,
		Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
		CreatorID: user.ID, TenantID: &tenant2.ID,
	})
	require.NoError(t, err)

	result := svc.InferResolutionForNewTicket(ctx, ticket)
	require.Equal(t, "", result)
}

func TestAskRovo_ContentHashDedup(t *testing.T) {
	ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
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

func TestAskRovo_ContentChangedCreatesNewVersion(t *testing.T) {
	ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
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

func TestAskRovo_IndexTicketContentWithComments(t *testing.T) {
	ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	memo := createTestMemo(t, ctx, ts, user, tenant, "Parent memo")
	ticket, err := ts.CreateTicket(ctx, &store.Ticket{
		Title: "Comment Test", Description: "/m/" + memo.UID,
		Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
		CreatorID: user.ID, TenantID: &tenant.ID,
	})
	require.NoError(t, err)

	comment1 := createTestMemo(t, ctx, ts, user, tenant, "First comment about RAG indexing")
	comment2 := createTestMemo(t, ctx, ts, user, tenant, "Second comment about ticket indexing")
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

func TestAskRovo_NilTenantSkipsInference(t *testing.T) {
	_, _, svc, _, _ := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	ticket := &store.Ticket{ID: 99999, Title: "Nil Tenant", TenantID: nil}
	result := svc.InferResolutionForNewTicket(context.Background(), ticket)
	require.Equal(t, "", result)
}

func TestAskRovo_ConcurrentIndexing(t *testing.T) {
	ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
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

func TestAskRovo_MinScoreThreshold(t *testing.T) {
	ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	svc.vectorDB.Insert(ctx, []DocumentChunk{
		{ID: "seed_1", TenantID: tenant.ID, AudienceType: "internal",
			ContentType: "ticket_section", Title: "RAG Indexing Feature",
			Content: "Per-ticket RAG indexing for Ask Rovo", IsActive: true},
	})

	memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
	ticket, err := ts.CreateTicket(ctx, &store.Ticket{
		Title: "Fire Damage Repair", Description: "/m/" + memo.UID,
		Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
		CreatorID: user.ID, TenantID: &tenant.ID,
	})
	require.NoError(t, err)

	result := svc.InferResolutionForNewTicket(ctx, ticket)
	require.Equal(t, "", result)
}

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
	// Uses count-based assertion to avoid flakiness from non-deterministic sort of equal-score chunks.
	chunkCount := strings.Count(result, "### ")
	require.Equal(t, 3, chunkCount, "TopK=3 should limit ticket suggestions to 3")
}

func TestAskRovo_DualTypeSearchFindsOldAndNewChunks(t *testing.T) {
	ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	// Seed old-format chunk (from ticket_embedder.go before fix)
	svc.vectorDB.Insert(ctx, []DocumentChunk{{
		ID: "old_ticket", TenantID: tenant.ID, AudienceType: "internal",
		ContentType: "ticket", Title: "Old Format Ticket",
		Content: "Per-ticket RAG indexing ticket", IsActive: true,
	}})

	// Seed new-format chunk (from ChunkMarkdownContent after fix)
	svc.vectorDB.Insert(ctx, []DocumentChunk{{
		ID: "new_ticket", TenantID: tenant.ID, AudienceType: "internal",
		ContentType: "ticket_section", Title: "New Format Ticket",
		Content: "Per-ticket RAG indexing ticket section", IsActive: true,
	}})

	// Title MUST contain seed keywords for controlledEmbeddingService to return HIGH vector
	memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
	ticket, err := ts.CreateTicket(ctx, &store.Ticket{
		Title: "Ticket RAG Indexing Dual Type", // contains "ticket", "rag", "indexing"
		Description: "/m/" + memo.UID,
		Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
		CreatorID: user.ID, TenantID: &tenant.ID,
	})
	require.NoError(t, err)

	result := svc.InferResolutionForNewTicket(ctx, ticket)
	require.NotEmpty(t, result)
	require.Contains(t, result, "Old Format Ticket")
	require.Contains(t, result, "New Format Ticket")
}

func TestSearchVectorDB_DualTypeContentTypes(t *testing.T) {
	ctx, _, svc, tenant, _ := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

	// Seed old-format KB chunk
	svc.vectorDB.Insert(ctx, []DocumentChunk{{
		ID: "old_kb", TenantID: tenant.ID, AudienceType: "external",
		ContentType: "kb", Title: "Old KB",
		Content: "RAG indexing for kb", IsActive: true,
	}})

	// Seed new-format KB chunk
	svc.vectorDB.Insert(ctx, []DocumentChunk{{
		ID: "new_kb", TenantID: tenant.ID, AudienceType: "external",
		ContentType: "kb_section", Title: "New KB",
		Content: "RAG indexing for kb section", IsActive: true,
	}})

	result, err := svc.SearchVectorDB(ctx, tenant.ID, "external", "kb", "RAG indexing", 5, nil)
	require.NoError(t, err)
	require.NotEmpty(t, result.Chunks)

	ids := make([]string, len(result.Chunks))
	for i, c := range result.Chunks {
		ids[i] = c.ID
	}
	require.Contains(t, ids, "old_kb")
	require.Contains(t, ids, "new_kb")
}

func TestLinkifyMemoRefs_SingleUID(t *testing.T) {
	input := "Similar ticket: /m/Fj8uFtHShTtZbD8y8KjusS"
	expected := "Similar ticket: [m/Fj8uFtHShTtZbD8y8KjusS](/m/Fj8uFtHShTtZbD8y8KjusS)"
	require.Equal(t, expected, linkifyMemoRefs(input))
}

func TestLinkifyMemoRefs_MultipleUIDs(t *testing.T) {
	input := "/m/Fj8uFtHShTtZbD8y8KjusS\n/m/MCSNigc5QCrgsycAnTJfih"
	expected := "[m/Fj8uFtHShTtZbD8y8KjusS](/m/Fj8uFtHShTtZbD8y8KjusS)\n[m/MCSNigc5QCrgsycAnTJfih](/m/MCSNigc5QCrgsycAnTJfih)"
	require.Equal(t, expected, linkifyMemoRefs(input))
}

func TestLinkifyMemoRefs_Idempotent(t *testing.T) {
	input := "/m/Fj8uFtHShTtZbD8y8KjusS"
	once := linkifyMemoRefs(input)
	twice := linkifyMemoRefs(once)
	require.Equal(t, once, twice)
}

func TestLinkifyMemoRefs_InsideMarkdownLink(t *testing.T) {
	input := "[m/Fj8uFtHShTtZbD8y8KjusS](/m/Fj8uFtHShTtZbD8y8KjusS)"
	require.Equal(t, input, linkifyMemoRefs(input))
}

func TestLinkifyMemoRefs_InvalidUID(t *testing.T) {
	// /m/invalid-uid matches the UID pattern; only characters outside the pattern
	// (like !@#) should remain untouched.
	input := "/m/invalid-uid-!@#"
	expected := "[m/invalid-uid](/m/invalid-uid)-!@#"
	require.Equal(t, expected, linkifyMemoRefs(input))
}

func TestLinkifyMemoRefs_EmptyInput(t *testing.T) {
	require.Equal(t, "", linkifyMemoRefs(""))
}
