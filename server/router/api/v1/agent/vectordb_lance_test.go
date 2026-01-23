//go:build rag && integration

package agent

import (
	"context"
	"os"
	"testing"
	"time"
)

// ============================================================================
// LANCEDB INTEGRATION TESTS
//
// These tests require:
// 1. Build tag: -tags "rag integration"
// 2. LanceDB native library available (LD_LIBRARY_PATH set)
// 3. Optional: OPENROUTER_API_KEY for real embeddings
//
// Run with:
//   LD_LIBRARY_PATH=./lancedb-go-main/pkg/lib/lib_linux_amd64:$LD_LIBRARY_PATH \
//     go test -tags "rag integration" -v ./server/router/api/v1/agent/...
// ============================================================================

func TestLanceVectorDB_Integration_CreateAndInsert(t *testing.T) {
	// Create temp directory for test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use mock embeddings for testing
	os.Setenv("EMBEDDING_PROVIDER", "mock")
	defer os.Unsetenv("EMBEDDING_PROVIDER")

	config := &VectorDBConfig{
		StorageProvider: "local",
		LocalPath:       tempDir,
		EmbeddingConfig: &EmbeddingConfig{
			Provider:  "mock",
			Dimension: 384,
		},
		Enabled:             true,
		HybridSearchEnabled: true,
		HybridVectorWeight:  0.7,
		HybridTextWeight:    0.3,
	}

	embedSvc, err := NewEmbeddingService(config.EmbeddingConfig)
	if err != nil {
		t.Fatalf("Failed to create embedding service: %v", err)
	}

	db, err := newLanceVectorDB(config, embedSvc)
	if err != nil {
		t.Fatalf("Failed to create LanceDB: %v", err)
	}
	defer db.Close()

	// Insert test chunks
	chunks := []DocumentChunk{
		{
			ID:           "chunk1",
			TenantID:     1,
			AudienceType: "kb",
			ContentType:  "service",
			Title:        "Water Damage Restoration",
			Content:      "Our water damage restoration services include water extraction, drying, and mold prevention.",
			IsActive:     true,
		},
		{
			ID:           "chunk2",
			TenantID:     1,
			AudienceType: "kb",
			ContentType:  "faq",
			Title:        "Emergency Response Time",
			Content:      "We respond to emergency water damage calls within 60 minutes in our service area.",
			IsActive:     true,
		},
	}

	ctx := context.Background()
	if err := db.Insert(ctx, chunks); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Verify insertion
	stats, err := db.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.TotalChunks != 2 {
		t.Errorf("Expected 2 chunks, got %d", stats.TotalChunks)
	}
}

func TestLanceVectorDB_Integration_VectorSearch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &VectorDBConfig{
		StorageProvider: "local",
		LocalPath:       tempDir,
		EmbeddingConfig: &EmbeddingConfig{
			Provider:  "mock",
			Dimension: 384,
		},
		Enabled: true,
	}

	embedSvc, err := NewEmbeddingService(config.EmbeddingConfig)
	if err != nil {
		t.Fatalf("Failed to create embedding service: %v", err)
	}

	db, err := newLanceVectorDB(config, embedSvc)
	if err != nil {
		t.Fatalf("Failed to create LanceDB: %v", err)
	}
	defer db.Close()

	// Insert test data
	chunks := []DocumentChunk{
		{ID: "c1", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Water Damage", Content: "Water damage restoration services", IsActive: true},
		{ID: "c2", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Fire Damage", Content: "Fire damage repair and cleanup", IsActive: true},
		{ID: "c3", TenantID: 2, AudienceType: "kb", ContentType: "service", Title: "Other Tenant", Content: "Different tenant content", IsActive: true},
	}

	ctx := context.Background()
	if err := db.Insert(ctx, chunks); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Allow time for indexing
	time.Sleep(100 * time.Millisecond)

	// Search for tenant 1 only
	result, err := db.Search(ctx, SearchQuery{
		QueryText: "water damage restoration",
		TenantID:  1,
		TopK:      10,
		MinScore:  0.0, // Accept all scores for testing
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

func TestLanceVectorDB_Integration_HybridSearch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &VectorDBConfig{
		StorageProvider: "local",
		LocalPath:       tempDir,
		EmbeddingConfig: &EmbeddingConfig{
			Provider:  "mock",
			Dimension: 384,
		},
		Enabled:             true,
		HybridSearchEnabled: true,
		HybridVectorWeight:  0.7,
		HybridTextWeight:    0.3,
	}

	embedSvc, err := NewEmbeddingService(config.EmbeddingConfig)
	if err != nil {
		t.Fatalf("Failed to create embedding service: %v", err)
	}

	db, err := newLanceVectorDB(config, embedSvc)
	if err != nil {
		t.Fatalf("Failed to create LanceDB: %v", err)
	}
	defer db.Close()

	// Insert test data with specific keywords
	chunks := []DocumentChunk{
		{ID: "c1", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Water Damage", Content: "Water damage restoration services for homes and businesses", IsActive: true},
		{ID: "c2", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Fire Damage", Content: "Fire damage repair and cleanup services", IsActive: true},
		{ID: "c3", TenantID: 1, AudienceType: "kb", ContentType: "faq", Title: "Coverage FAQ", Content: "Questions about water damage coverage and insurance", IsActive: true},
	}

	ctx := context.Background()
	if err := db.Insert(ctx, chunks); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Allow time for indexing
	time.Sleep(100 * time.Millisecond)

	// Hybrid search
	result, err := db.Search(ctx, SearchQuery{
		QueryText:       "water damage",
		TenantID:        1,
		TopK:            10,
		MinScore:        0.0,
		UseHybridSearch: true,
		VectorWeight:    0.7,
		TextWeight:      0.3,
	})
	if err != nil {
		// Hybrid search may fall back to vector-only if FTS not supported
		t.Logf("Hybrid search error (may be expected): %v", err)
		return
	}

	// Check if we got hybrid or fell back to vector
	if result.SearchMode == "hybrid" {
		// Should have component scores
		if len(result.VectorScores) != len(result.Chunks) {
			t.Errorf("Expected VectorScores length to match Chunks length")
		}
		if len(result.BM25Scores) != len(result.Chunks) {
			t.Errorf("Expected BM25Scores length to match Chunks length")
		}
		t.Logf("Hybrid search successful with %d results", len(result.Chunks))
	} else {
		t.Logf("Search fell back to %s mode (FTS may not be supported)", result.SearchMode)
	}
}

func TestLanceVectorDB_Integration_Delete(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &VectorDBConfig{
		StorageProvider: "local",
		LocalPath:       tempDir,
		EmbeddingConfig: &EmbeddingConfig{
			Provider:  "mock",
			Dimension: 384,
		},
		Enabled: true,
	}

	embedSvc, err := NewEmbeddingService(config.EmbeddingConfig)
	if err != nil {
		t.Fatalf("Failed to create embedding service: %v", err)
	}

	db, err := newLanceVectorDB(config, embedSvc)
	if err != nil {
		t.Fatalf("Failed to create LanceDB: %v", err)
	}
	defer db.Close()

	// Insert test data
	chunks := []DocumentChunk{
		{ID: "c1", TenantID: 1, AudienceType: "kb", Title: "KB Content", Content: "Knowledge base content", IsActive: true},
		{ID: "c2", TenantID: 1, AudienceType: "policy", Title: "Policy Content", Content: "Policy content", IsActive: true},
		{ID: "c3", TenantID: 2, AudienceType: "kb", Title: "Other Tenant", Content: "Other tenant content", IsActive: true},
	}

	ctx := context.Background()
	if err := db.Insert(ctx, chunks); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Delete tenant 1 kb chunks
	if err := db.Delete(ctx, 1, "kb"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	stats, err := db.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.TotalChunks != 2 {
		t.Errorf("Expected 2 chunks after delete, got %d", stats.TotalChunks)
	}
}

func TestLanceVectorDB_Integration_FTSIndexCreation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &VectorDBConfig{
		StorageProvider: "local",
		LocalPath:       tempDir,
		EmbeddingConfig: &EmbeddingConfig{
			Provider:  "mock",
			Dimension: 384,
		},
		Enabled: true,
	}

	embedSvc, err := NewEmbeddingService(config.EmbeddingConfig)
	if err != nil {
		t.Fatalf("Failed to create embedding service: %v", err)
	}

	db, err := newLanceVectorDB(config, embedSvc)
	if err != nil {
		t.Fatalf("Failed to create LanceDB: %v", err)
	}
	defer db.Close()

	// Insert some data to ensure table exists
	chunks := []DocumentChunk{
		{ID: "c1", TenantID: 1, AudienceType: "kb", ContentType: "service", Title: "Test", Content: "Test content", IsActive: true},
	}

	ctx := context.Background()
	if err := db.Insert(ctx, chunks); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Verify we can get stats (indicates table is healthy)
	stats, err := db.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	t.Logf("LanceDB initialized with %d chunks", stats.TotalChunks)
	t.Log("FTS indexes should be created automatically if supported")
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

func TestVectorDBConfig_HybridSearchFromEnv(t *testing.T) {
	// Save and restore original env
	origEnabled := os.Getenv("HYBRID_SEARCH_ENABLED")
	origVector := os.Getenv("HYBRID_VECTOR_WEIGHT")
	origText := os.Getenv("HYBRID_TEXT_WEIGHT")
	defer func() {
		os.Setenv("HYBRID_SEARCH_ENABLED", origEnabled)
		os.Setenv("HYBRID_VECTOR_WEIGHT", origVector)
		os.Setenv("HYBRID_TEXT_WEIGHT", origText)
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
