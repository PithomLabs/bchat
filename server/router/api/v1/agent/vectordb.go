package agent

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// VectorDB defines the interface for vector database operations.
// This abstraction allows switching between implementations (in-memory, LanceDB, etc.)
type VectorDB interface {
	// Insert adds or updates chunks in the vector database.
	Insert(ctx context.Context, chunks []DocumentChunk) error

	// Delete removes chunks matching the filter criteria.
	Delete(ctx context.Context, tenantID int32, audienceType string) error

	// Search performs hybrid search (vector + metadata filtering).
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)

	// Close releases resources.
	Close() error

	// Stats returns database statistics.
	Stats(ctx context.Context) (*VectorDBStats, error)
}

// VectorDBConfig holds configuration for the vector database.
type VectorDBConfig struct {
	// Storage configuration
	StorageProvider string // "memory", "local", or "s3"
	LocalPath       string // For local: "build/data/lancedb/"

	// S3/Tigrisdata configuration (for production)
	S3Endpoint  string // "fly.storage.tigris.dev" for Tigrisdata
	S3Bucket    string
	S3Region    string // "auto" for Tigrisdata
	S3AccessKey string
	S3SecretKey string

	// Embedding configuration
	EmbeddingConfig *EmbeddingConfig

	// RAG feature flag
	Enabled bool
}

// NewVectorDBConfigFromEnv creates a VectorDBConfig from environment variables.
func NewVectorDBConfigFromEnv() *VectorDBConfig {
	return &VectorDBConfig{
		StorageProvider: getEnvOrDefault("LANCEDB_STORAGE_PROVIDER", "memory"),
		LocalPath:       getEnvOrDefault("LANCEDB_LOCAL_PATH", "build/data/lancedb"),
		S3Endpoint:      getEnvOrDefault("LANCEDB_S3_ENDPOINT", "fly.storage.tigris.dev"),
		S3Bucket:        os.Getenv("LANCEDB_S3_BUCKET"),
		S3Region:        getEnvOrDefault("LANCEDB_S3_REGION", "auto"),
		S3AccessKey:     os.Getenv("AWS_ACCESS_KEY_ID"),
		S3SecretKey:     os.Getenv("AWS_SECRET_ACCESS_KEY"),
		EmbeddingConfig: NewEmbeddingConfigFromEnv(),
		Enabled:         os.Getenv("RAG_PIPELINE_ENABLED") == "true",
	}
}

// SearchQuery represents a search request.
type SearchQuery struct {
	// Query text (will be embedded)
	QueryText string

	// Pre-computed query embedding (optional, if already embedded)
	QueryEmbedding []float32

	// Filters
	TenantID     int32
	AudienceType string
	ContentTypes []string // Filter by content types (service, faq, etc.)
	ActiveOnly   bool     // Only return active chunks

	// Pagination
	TopK   int     // Number of results to return
	MinScore float64 // Minimum similarity score (0-1)
}

// SearchResult holds the search results.
type SearchResult struct {
	Chunks  []DocumentChunk
	Scores  []float64 // Similarity scores (0-1, higher is better)
	Total   int       // Total matching documents
	Latency time.Duration
}

// VectorDBStats holds database statistics.
type VectorDBStats struct {
	TotalChunks    int64
	TenantCounts   map[int32]int64
	ContentCounts  map[string]int64
	IndexSize      int64 // in bytes
	LastOptimized  time.Time
}

// NewVectorDB creates a vector database based on configuration.
func NewVectorDB(config *VectorDBConfig) (VectorDB, error) {
	if !config.Enabled {
		slog.Info("RAG pipeline disabled, using no-op vector database")
		return NewNoOpVectorDB(), nil
	}

	// Initialize embedding service
	embedSvc, err := NewEmbeddingService(config.EmbeddingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding service: %w", err)
	}

	switch config.StorageProvider {
	case "memory":
		slog.Info("Using in-memory vector database for testing")
		return NewMemoryVectorDB(embedSvc), nil
	case "local":
		slog.Info("Using local LanceDB storage", "path", config.LocalPath)
		return newLanceVectorDB(config, embedSvc)
	case "s3":
		slog.Info("Using S3 LanceDB storage", "endpoint", config.S3Endpoint, "bucket", config.S3Bucket)
		return newLanceVectorDB(config, embedSvc)
	default:
		return NewMemoryVectorDB(embedSvc), nil
	}
}

// ============================================================================
// IN-MEMORY VECTOR DATABASE (Testing/Development)
// ============================================================================

// MemoryVectorDB is an in-memory implementation of VectorDB for testing.
type MemoryVectorDB struct {
	chunks   map[string]DocumentChunk // key: chunk ID
	embedSvc EmbeddingService
	mu       sync.RWMutex
}

// NewMemoryVectorDB creates a new in-memory vector database.
func NewMemoryVectorDB(embedSvc EmbeddingService) *MemoryVectorDB {
	return &MemoryVectorDB{
		chunks:   make(map[string]DocumentChunk),
		embedSvc: embedSvc,
	}
}

// Insert adds or updates chunks in the database.
func (db *MemoryVectorDB) Insert(ctx context.Context, chunks []DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Generate embeddings for chunks that don't have them
	var textsToEmbed []string
	var indicesToEmbed []int

	for i, chunk := range chunks {
		if len(chunk.Embedding) == 0 {
			textsToEmbed = append(textsToEmbed, fmt.Sprintf("%s: %s", chunk.Title, chunk.Content))
			indicesToEmbed = append(indicesToEmbed, i)
		}
	}

	if len(textsToEmbed) > 0 {
		embeddings, err := db.embedSvc.Embed(ctx, textsToEmbed)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings: %w", err)
		}

		for i, idx := range indicesToEmbed {
			chunks[idx].Embedding = embeddings[i]
		}
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	for _, chunk := range chunks {
		db.chunks[chunk.ID] = chunk
	}

	slog.Debug("Inserted chunks into memory vector DB", "count", len(chunks))
	return nil
}

// Delete removes chunks matching the filter criteria.
func (db *MemoryVectorDB) Delete(ctx context.Context, tenantID int32, audienceType string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	var toDelete []string
	for id, chunk := range db.chunks {
		if chunk.TenantID == tenantID && chunk.AudienceType == audienceType {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		delete(db.chunks, id)
	}

	slog.Debug("Deleted chunks from memory vector DB", "count", len(toDelete), "tenantID", tenantID, "audience", audienceType)
	return nil
}

// Search performs hybrid search.
func (db *MemoryVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	start := time.Now()

	// Get or generate query embedding
	var queryEmbedding []float32
	if len(query.QueryEmbedding) > 0 {
		queryEmbedding = query.QueryEmbedding
	} else if query.QueryText != "" {
		embeddings, err := db.embedSvc.Embed(ctx, []string{query.QueryText})
		if err != nil {
			return nil, fmt.Errorf("failed to embed query: %w", err)
		}
		queryEmbedding = embeddings[0]
	} else {
		return nil, fmt.Errorf("query must have either QueryText or QueryEmbedding")
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	type scoredChunk struct {
		chunk DocumentChunk
		score float64
	}

	var scored []scoredChunk

	// Filter and score chunks
	for _, chunk := range db.chunks {
		// Apply filters
		if chunk.TenantID != query.TenantID {
			continue
		}
		if query.AudienceType != "" && chunk.AudienceType != query.AudienceType {
			continue
		}
		if query.ActiveOnly && !chunk.IsActive {
			continue
		}
		if len(query.ContentTypes) > 0 {
			found := false
			for _, ct := range query.ContentTypes {
				if chunk.ContentType == ct {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Calculate similarity score
		if len(chunk.Embedding) > 0 {
			score := cosineSimilarity(queryEmbedding, chunk.Embedding)
			if score >= query.MinScore {
				scored = append(scored, scoredChunk{chunk: chunk, score: score})
			}
		}
	}

	// Sort by score (descending)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Apply TopK limit
	topK := query.TopK
	if topK <= 0 {
		topK = 10
	}
	if len(scored) > topK {
		scored = scored[:topK]
	}

	// Build result
	result := &SearchResult{
		Chunks:  make([]DocumentChunk, len(scored)),
		Scores:  make([]float64, len(scored)),
		Total:   len(scored),
		Latency: time.Since(start),
	}

	for i, sc := range scored {
		result.Chunks[i] = sc.chunk
		result.Scores[i] = sc.score
	}

	return result, nil
}

// Close releases resources.
func (db *MemoryVectorDB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.chunks = make(map[string]DocumentChunk)
	return nil
}

// Stats returns database statistics.
func (db *MemoryVectorDB) Stats(ctx context.Context) (*VectorDBStats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := &VectorDBStats{
		TotalChunks:   int64(len(db.chunks)),
		TenantCounts:  make(map[int32]int64),
		ContentCounts: make(map[string]int64),
	}

	for _, chunk := range db.chunks {
		stats.TenantCounts[chunk.TenantID]++
		stats.ContentCounts[chunk.ContentType]++
	}

	return stats, nil
}

// ============================================================================
// NO-OP VECTOR DATABASE (When RAG is disabled)
// ============================================================================

// NoOpVectorDB is a no-op implementation when RAG is disabled.
type NoOpVectorDB struct{}

// NewNoOpVectorDB creates a new no-op vector database.
func NewNoOpVectorDB() *NoOpVectorDB {
	return &NoOpVectorDB{}
}

// Insert is a no-op.
func (db *NoOpVectorDB) Insert(ctx context.Context, chunks []DocumentChunk) error {
	return nil
}

// Delete is a no-op.
func (db *NoOpVectorDB) Delete(ctx context.Context, tenantID int32, audienceType string) error {
	return nil
}

// Search returns empty results.
func (db *NoOpVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	return &SearchResult{
		Chunks:  []DocumentChunk{},
		Scores:  []float64{},
		Total:   0,
		Latency: 0,
	}, nil
}

// Close is a no-op.
func (db *NoOpVectorDB) Close() error {
	return nil
}

// Stats returns empty statistics.
func (db *NoOpVectorDB) Stats(ctx context.Context) (*VectorDBStats, error) {
	return &VectorDBStats{
		TotalChunks:   0,
		TenantCounts:  make(map[int32]int64),
		ContentCounts: make(map[string]int64),
	}, nil
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// cosineSimilarity calculates the cosine similarity between two vectors.
// Returns a value between -1 and 1 (1 = identical, 0 = orthogonal, -1 = opposite).
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ============================================================================
// RETRIEVAL HELPER (for Service integration)
// ============================================================================

// RetrievedContext holds the retrieved context for prompt building.
type RetrievedContext struct {
	Services   []DocumentChunk
	FAQs       []DocumentChunk
	Exclusions []DocumentChunk
	Coverage   []DocumentChunk
	Rules      []DocumentChunk
	Safety     []DocumentChunk
	KBSections []DocumentChunk
}

// RetrieveContextForQuery performs retrieval based on user query and intent.
func RetrieveContextForQuery(
	ctx context.Context,
	db VectorDB,
	query string,
	intent string,
	tenantID int32,
	audienceType string,
) (*RetrievedContext, error) {
	// Determine content types and topK based on intent
	var contentTypes []string
	var topK int

	switch strings.ToLower(intent) {
	case "service_inquiry", "booking_request":
		contentTypes = []string{"service", "faq"}
		topK = 5
	case "coverage_question":
		contentTypes = []string{"coverage", "service"}
		topK = 5
	case "pricing_question":
		contentTypes = []string{"faq", "service"}
		topK = 3
	case "emergency":
		contentTypes = []string{"service", "safety"}
		topK = 3
	case "complaint", "escalation":
		contentTypes = []string{"rule", "faq"}
		topK = 3
	default:
		// General query - search all relevant types
		contentTypes = []string{"faq", "service", "kb_section"}
		topK = 5
	}

	// Perform search
	result, err := db.Search(ctx, SearchQuery{
		QueryText:    query,
		TenantID:     tenantID,
		AudienceType: audienceType,
		ContentTypes: contentTypes,
		ActiveOnly:   true,
		TopK:         topK,
		MinScore:     0.3, // Minimum relevance threshold
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Organize results by type
	retrieved := &RetrievedContext{}
	for _, chunk := range result.Chunks {
		switch chunk.ContentType {
		case "service":
			retrieved.Services = append(retrieved.Services, chunk)
		case "faq":
			retrieved.FAQs = append(retrieved.FAQs, chunk)
		case "exclusion":
			retrieved.Exclusions = append(retrieved.Exclusions, chunk)
		case "coverage":
			retrieved.Coverage = append(retrieved.Coverage, chunk)
		case "rule":
			retrieved.Rules = append(retrieved.Rules, chunk)
		case "safety":
			retrieved.Safety = append(retrieved.Safety, chunk)
		case "kb_section":
			retrieved.KBSections = append(retrieved.KBSections, chunk)
		}
	}

	return retrieved, nil
}
