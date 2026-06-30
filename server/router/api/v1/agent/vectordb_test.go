package agent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"testing"
)

// ============================================================================
// BM25 SCORER TESTS
// ============================================================================

func TestNewBM25Scorer(t *testing.T) {
	scorer := NewBM25Scorer()

	if scorer == nil {
		t.Fatal("NewBM25Scorer returned nil")
	}

	if scorer.k1 != 1.2 {
		t.Errorf("Expected k1=1.2, got %f", scorer.k1)
	}

	if scorer.b != 0.75 {
		t.Errorf("Expected b=0.75, got %f", scorer.b)
	}

	if scorer.totalDocs != 0 {
		t.Errorf("Expected totalDocs=0, got %d", scorer.totalDocs)
	}
}

func TestBM25Scorer_AddDocument(t *testing.T) {
	scorer := NewBM25Scorer()

	scorer.AddDocument("doc1", "hello world")
	scorer.AddDocument("doc2", "hello there")
	scorer.AddDocument("doc3", "goodbye world")

	if scorer.totalDocs != 3 {
		t.Errorf("Expected totalDocs=3, got %d", scorer.totalDocs)
	}

	// Check document frequency for "hello" (appears in 2 docs)
	if scorer.docFreq["hello"] != 2 {
		t.Errorf("Expected docFreq[hello]=2, got %d", scorer.docFreq["hello"])
	}

	// Check document frequency for "world" (appears in 2 docs)
	if scorer.docFreq["world"] != 2 {
		t.Errorf("Expected docFreq[world]=2, got %d", scorer.docFreq["world"])
	}

	// Check document frequency for "goodbye" (appears in 1 doc)
	if scorer.docFreq["goodbye"] != 1 {
		t.Errorf("Expected docFreq[goodbye]=1, got %d", scorer.docFreq["goodbye"])
	}
}

func TestBM25Scorer_Score_ExactMatch(t *testing.T) {
	scorer := NewBM25Scorer()

	scorer.AddDocument("doc1", "water damage restoration services")
	scorer.AddDocument("doc2", "fire damage repair")
	scorer.AddDocument("doc3", "mold remediation cleaning")

	// Query for "water damage"
	score1 := scorer.Score("water damage", "doc1")
	score2 := scorer.Score("water damage", "doc2")
	score3 := scorer.Score("water damage", "doc3")

	// doc1 should score highest (has both "water" and "damage")
	if score1 <= score2 {
		t.Errorf("Expected doc1 score > doc2 score, got %f <= %f", score1, score2)
	}

	// doc2 should score higher than doc3 (has "damage")
	if score2 <= score3 {
		t.Errorf("Expected doc2 score > doc3 score, got %f <= %f", score2, score3)
	}

	// doc3 should score 0 (no matching terms)
	if score3 != 0 {
		t.Errorf("Expected doc3 score = 0, got %f", score3)
	}
}

func TestBM25Scorer_Score_IDFWeighting(t *testing.T) {
	scorer := NewBM25Scorer()

	// Add documents where "the" is common and "water" is rare
	scorer.AddDocument("doc1", "the water damage")
	scorer.AddDocument("doc2", "the fire damage")
	scorer.AddDocument("doc3", "the mold damage")
	scorer.AddDocument("doc4", "the smoke damage")

	// "water" appears in 1/4 docs, "the" appears in 4/4 docs
	// Query for "water" should give higher score than "the" for doc1
	// because "water" has higher IDF (more discriminative)

	scoreWater := scorer.Score("water", "doc1")
	scoreThe := scorer.Score("the", "doc1")

	// "water" should have higher score due to higher IDF
	if scoreWater <= scoreThe {
		t.Errorf("Expected 'water' score > 'the' score, got %f <= %f", scoreWater, scoreThe)
	}
}

func TestBM25Scorer_Score_LengthNormalization(t *testing.T) {
	scorer := NewBM25Scorer()

	// Add one short and one long document with the same query term
	scorer.AddDocument("short", "water damage")
	scorer.AddDocument("long", "water damage restoration services include flood cleanup and mold remediation")

	scoreShort := scorer.Score("water", "short")
	scoreLong := scorer.Score("water", "long")

	// Short document should score higher due to length normalization
	// (term is more prominent in shorter document)
	if scoreShort <= scoreLong {
		t.Errorf("Expected short doc score > long doc score, got %f <= %f", scoreShort, scoreLong)
	}
}

func TestBM25Scorer_Score_Normalization(t *testing.T) {
	scorer := NewBM25Scorer()

	scorer.AddDocument("doc1", "water damage restoration")

	score := scorer.Score("water damage", "doc1")

	// Score should be normalized to 0-1 range
	if score < 0 || score > 1 {
		t.Errorf("Expected score in [0,1], got %f", score)
	}
}

func TestBM25Scorer_Score_NonExistentDocument(t *testing.T) {
	scorer := NewBM25Scorer()

	scorer.AddDocument("doc1", "hello world")

	score := scorer.Score("hello", "nonexistent")

	if score != 0 {
		t.Errorf("Expected score=0 for nonexistent document, got %f", score)
	}
}

func TestBM25Scorer_Score_EmptyQuery(t *testing.T) {
	scorer := NewBM25Scorer()

	scorer.AddDocument("doc1", "hello world")

	score := scorer.Score("", "doc1")

	if score != 0 {
		t.Errorf("Expected score=0 for empty query, got %f", score)
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"Hello World", []string{"hello", "world"}},
		{"hello, world!", []string{"hello", "world"}},
		{"water-damage restoration", []string{"water", "damage", "restoration"}},
		{"one a b", []string{"one"}}, // "a" and "b" filtered (too short)
		{"", []string{}},
	}

	for _, tt := range tests {
		result := tokenize(tt.input)

		if len(result) != len(tt.expected) {
			t.Errorf("tokenize(%q): expected %d tokens, got %d", tt.input, len(tt.expected), len(result))
			continue
		}

		for i, token := range result {
			if token != tt.expected[i] {
				t.Errorf("tokenize(%q)[%d]: expected %q, got %q", tt.input, i, tt.expected[i], token)
			}
		}
	}
}

// ============================================================================
// MEMORY VECTOR DB TESTS
// ============================================================================

// MockEmbeddingService for testing
type mockEmbeddingService struct {
	dimension int
	err       error
	calls     int
}

func (m *mockEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	result := make([][]float32, len(texts))
	for i, text := range texts {
		// Generate deterministic embedding based on text hash
		embedding := make([]float32, m.dimension)
		for j := 0; j < m.dimension; j++ {
			// Simple hash-based pseudo-random
			hash := 0
			for _, c := range text {
				hash = hash*31 + int(c)
			}
			embedding[j] = float32(math.Sin(float64(hash+j))) * 0.5
		}
		result[i] = embedding
	}
	return result, nil
}

func (m *mockEmbeddingService) Dimension() int {
	return m.dimension
}

func (m *mockEmbeddingService) Provider() string {
	return "mock"
}

func TestMemoryVectorDB_Validate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		embedSvc := &mockEmbeddingService{dimension: 8}
		db := NewMemoryVectorDB(embedSvc)

		if err := db.Validate(context.Background()); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if embedSvc.calls != 1 {
			t.Fatalf("Embed calls = %d, want 1", embedSvc.calls)
		}
	})

	t.Run("embedding error", func(t *testing.T) {
		wantErr := fmt.Errorf("%w: missing key", ErrEmbeddingProviderMisconfigured)
		embedSvc := &mockEmbeddingService{dimension: 8, err: wantErr}
		db := NewMemoryVectorDB(embedSvc)

		err := db.Validate(context.Background())
		if !errors.Is(err, ErrEmbeddingProviderMisconfigured) {
			t.Fatalf("Validate() error = %v, want ErrEmbeddingProviderMisconfigured", err)
		}
	})
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "timeout", err: errors.New("request timeout"), want: false},
		{name: "deadline", err: context.DeadlineExceeded, want: true},
		{name: "rate_limit", err: &embeddingHTTPError{statusCode: http.StatusTooManyRequests}, want: true},
		{name: "bad_gateway", err: &embeddingHTTPError{statusCode: http.StatusBadGateway}, want: true},
		{name: "misconfigured", err: fmt.Errorf("%w: bad key", ErrEmbeddingProviderMisconfigured), want: false},
		{name: "unauthorized", err: errors.New("401 Unauthorized"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.want {
				t.Fatalf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestMemoryVectorDB_Insert(t *testing.T) {
	embedSvc := &mockEmbeddingService{dimension: 8}
	db := NewMemoryVectorDB(embedSvc)

	chunks := []DocumentChunk{
		{
			ID:           "chunk1",
			TenantID:     1,
			AudienceType: "kb",
			ContentType:  "service",
			Title:        "Water Damage",
			Content:      "Water damage restoration services",
			IsActive:     true,
		},
		{
			ID:           "chunk2",
			TenantID:     1,
			AudienceType: "kb",
			ContentType:  "faq",
			Title:        "FAQ",
			Content:      "Frequently asked questions",
			IsActive:     true,
		},
	}

	err := db.Insert(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	stats, err := db.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.TotalChunks != 2 {
		t.Errorf("Expected 2 chunks, got %d", stats.TotalChunks)
	}
}

func TestMemoryVectorDB_Search_VectorOnly(t *testing.T) {
	embedSvc := &mockEmbeddingService{dimension: 8}
	db := NewMemoryVectorDB(embedSvc)

	// Insert test chunks
	chunks := []DocumentChunk{
		{ID: "c1", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Water Damage", Content: "Water damage restoration", IsActive: true},
		{ID: "c2", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Fire Damage", Content: "Fire damage repair", IsActive: true},
		{ID: "c3", TenantID: 2, AudienceType: "kb", ContentType: "service", Title: "Other", Content: "Other tenant content", IsActive: true},
	}

	err := db.Insert(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Search for tenant 1 only
	result, err := db.Search(context.Background(), SearchQuery{
		QueryText: "water damage",
		TenantID:  1,
		TopK:      10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should not include tenant 2's content
	for _, chunk := range result.Chunks {
		if chunk.TenantID != 1 {
			t.Errorf("Search returned chunk from wrong tenant: %d", chunk.TenantID)
		}
	}

	if result.SearchMode != "vector" {
		t.Errorf("Expected SearchMode='vector', got %q", result.SearchMode)
	}
}

func TestMemoryVectorDB_Search_Hybrid(t *testing.T) {
	embedSvc := &mockEmbeddingService{dimension: 8}
	db := NewMemoryVectorDB(embedSvc)

	// Insert test chunks
	chunks := []DocumentChunk{
		{ID: "c1", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Water Damage", Content: "Water damage restoration services for your home", IsActive: true},
		{ID: "c2", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Fire Damage", Content: "Fire damage repair and cleanup", IsActive: true},
		{ID: "c3", TenantID: 1, AudienceType: "kb", ContentType: "faq", Title: "FAQ", Content: "How much does water damage cost", IsActive: true},
	}

	err := db.Insert(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Hybrid search
	result, err := db.Search(context.Background(), SearchQuery{
		QueryText:       "water damage",
		TenantID:        1,
		TopK:            10,
		UseHybridSearch: true,
		VectorWeight:    0.7,
		TextWeight:      0.3,
	})
	if err != nil {
		t.Fatalf("Hybrid search failed: %v", err)
	}

	if result.SearchMode != "hybrid" {
		t.Errorf("Expected SearchMode='hybrid', got %q", result.SearchMode)
	}

	// Should have component scores
	if len(result.VectorScores) != len(result.Chunks) {
		t.Errorf("Expected VectorScores length to match Chunks length")
	}

	if len(result.BM25Scores) != len(result.Chunks) {
		t.Errorf("Expected BM25Scores length to match Chunks length")
	}

	// Chunks with "water" and "damage" in content should rank higher
	// due to BM25 contribution
	if len(result.Chunks) > 0 {
		topChunk := result.Chunks[0]
		if topChunk.ID != "c1" && topChunk.ID != "c3" {
			t.Logf("Top result was %q - expected c1 or c3 for 'water damage' query", topChunk.ID)
		}
	}
}

func TestMemoryVectorDB_Delete(t *testing.T) {
	embedSvc := &mockEmbeddingService{dimension: 8}
	db := NewMemoryVectorDB(embedSvc)

	chunks := []DocumentChunk{
		{ID: "c1", TenantID: 1, AudienceType: "kb", Title: "Test", Content: "Test", IsActive: true},
		{ID: "c2", TenantID: 1, AudienceType: "policy", Title: "Test", Content: "Test", IsActive: true},
		{ID: "c3", TenantID: 2, AudienceType: "kb", Title: "Test", Content: "Test", IsActive: true},
	}

	db.Insert(context.Background(), chunks)

	// Delete tenant 1 kb chunks
	err := db.Delete(context.Background(), 1, "kb")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	stats, _ := db.Stats(context.Background())
	if stats.TotalChunks != 2 {
		t.Errorf("Expected 2 chunks after delete, got %d", stats.TotalChunks)
	}
}

func TestMemoryVectorDB_ContentTypeFilter(t *testing.T) {
	embedSvc := &mockEmbeddingService{dimension: 8}
	db := NewMemoryVectorDB(embedSvc)

	chunks := []DocumentChunk{
		{ID: "c1", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Service", Content: "Service content", IsActive: true},
		{ID: "c2", TenantID: 1, AudienceType: "kb", ContentType: "faq", Title: "FAQ", Content: "FAQ content", IsActive: true},
		{ID: "c3", TenantID: 1, AudienceType: "kb", ContentType: "coverage", Title: "Coverage", Content: "Coverage content", IsActive: true},
	}

	db.Insert(context.Background(), chunks)

	// Search for service and faq only
	result, err := db.Search(context.Background(), SearchQuery{
		QueryText:    "content",
		TenantID:     1,
		ContentTypes: []string{"service", "faq"},
		TopK:         10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, chunk := range result.Chunks {
		if chunk.ContentType != "service" && chunk.ContentType != "faq" {
			t.Errorf("Unexpected content type: %s", chunk.ContentType)
		}
	}
}

// ============================================================================
// COSINE SIMILARITY TESTS
// ============================================================================

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0.0,
		},
		{
			name:     "different lengths",
			a:        []float32{1, 0},
			b:        []float32{1, 0, 0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)

			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// NOOP VECTOR DB TESTS
// ============================================================================

func TestNoOpVectorDB(t *testing.T) {
	db := NewNoOpVectorDB()

	// Insert should be no-op
	err := db.Insert(context.Background(), []DocumentChunk{{ID: "test"}})
	if err != nil {
		t.Errorf("Insert should not return error: %v", err)
	}

	// Search should return empty
	result, err := db.Search(context.Background(), SearchQuery{QueryText: "test"})
	if err != nil {
		t.Errorf("Search should not return error: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Errorf("Expected 0 chunks, got %d", len(result.Chunks))
	}

	// Delete should be no-op
	err = db.Delete(context.Background(), 1, "kb")
	if err != nil {
		t.Errorf("Delete should not return error: %v", err)
	}

	// Stats should return zeros
	stats, err := db.Stats(context.Background())
	if err != nil {
		t.Errorf("Stats should not return error: %v", err)
	}
	if stats.TotalChunks != 0 {
		t.Errorf("Expected 0 total chunks, got %d", stats.TotalChunks)
	}
}

// ============================================================================
// ENVIRONMENT CONFIGURATION TESTS
// ============================================================================

func TestParseFloatOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		envKey     string
		envValue   string
		defaultVal float64
		expected   float64
	}{
		{"empty env", "TEST_FLOAT_EMPTY", "", 0.7, 0.7},
		{"valid float", "TEST_FLOAT_VALID", "0.8", 0.7, 0.8},
		{"invalid float", "TEST_FLOAT_INVALID", "not-a-number", 0.7, 0.7},
		{"zero value", "TEST_FLOAT_ZERO", "0", 0.7, 0},
		{"negative value", "TEST_FLOAT_NEG", "-0.5", 0.7, -0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := parseFloatOrDefault(tt.envKey, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestVectorDBConfig_HybridSearchDefaults(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("HYBRID_SEARCH_ENABLED")
	os.Unsetenv("HYBRID_VECTOR_WEIGHT")
	os.Unsetenv("HYBRID_TEXT_WEIGHT")

	config := NewVectorDBConfigFromEnv()

	// Defaults should be: disabled, 0.7 vector, 0.3 text
	if config.HybridSearchEnabled {
		t.Error("Expected HybridSearchEnabled to be false by default")
	}
	if config.HybridVectorWeight != 0.7 {
		t.Errorf("Expected default HybridVectorWeight=0.7, got %f", config.HybridVectorWeight)
	}
	if config.HybridTextWeight != 0.3 {
		t.Errorf("Expected default HybridTextWeight=0.3, got %f", config.HybridTextWeight)
	}
}

func TestVectorDBConfig_HybridSearchEnabled(t *testing.T) {
	// Save and restore original env
	origEnabled := os.Getenv("HYBRID_SEARCH_ENABLED")
	origVector := os.Getenv("HYBRID_VECTOR_WEIGHT")
	origText := os.Getenv("HYBRID_TEXT_WEIGHT")
	defer func() {
		if origEnabled != "" {
			os.Setenv("HYBRID_SEARCH_ENABLED", origEnabled)
		} else {
			os.Unsetenv("HYBRID_SEARCH_ENABLED")
		}
		if origVector != "" {
			os.Setenv("HYBRID_VECTOR_WEIGHT", origVector)
		} else {
			os.Unsetenv("HYBRID_VECTOR_WEIGHT")
		}
		if origText != "" {
			os.Setenv("HYBRID_TEXT_WEIGHT", origText)
		} else {
			os.Unsetenv("HYBRID_TEXT_WEIGHT")
		}
	}()

	// Test with hybrid search enabled
	os.Setenv("HYBRID_SEARCH_ENABLED", "true")
	os.Setenv("HYBRID_VECTOR_WEIGHT", "0.6")
	os.Setenv("HYBRID_TEXT_WEIGHT", "0.4")

	config := NewVectorDBConfigFromEnv()

	if !config.HybridSearchEnabled {
		t.Error("Expected HybridSearchEnabled to be true")
	}
	if config.HybridVectorWeight != 0.6 {
		t.Errorf("Expected HybridVectorWeight=0.6, got %f", config.HybridVectorWeight)
	}
	if config.HybridTextWeight != 0.4 {
		t.Errorf("Expected HybridTextWeight=0.4, got %f", config.HybridTextWeight)
	}
}
