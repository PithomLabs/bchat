package agent

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CheckpointCallback is called after each successful batch to update progress.
type CheckpointCallback func(currentBatch, processedChunks, totalBatches, totalChunks int) error

// InsertOptions configures the InsertWithCheckpoint operation.
type InsertOptions struct {
	StartBatch     int                // Resume from this batch (0-indexed)
	CheckpointFunc CheckpointCallback // Called after each batch
	MaxRetries     int                // Max retries per batch (default: 3)
	RetryDelay     time.Duration      // Initial delay between retries (default: 5s)
}

// VectorDB defines the interface for vector database operations.
// This abstraction allows switching between implementations (in-memory, LanceDB, etc.)
type VectorDB interface {
	// Insert adds or updates chunks in the vector database.
	Insert(ctx context.Context, chunks []DocumentChunk) error

	// InsertWithCheckpoint adds chunks with progress tracking and resume capability.
	InsertWithCheckpoint(ctx context.Context, chunks []DocumentChunk, opts InsertOptions) error

	// Delete removes chunks matching the filter criteria.
	Delete(ctx context.Context, tenantID int32, audienceType string) error

	// Search performs hybrid search (vector + metadata filtering).
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)

	// Close releases resources.
	Close() error

	// Stats returns database statistics.
	Stats(ctx context.Context) (*VectorDBStats, error)

	// ListChunks returns all chunks for a given tenant (used for stats/counting).
	ListChunks(ctx context.Context, tenantID int32) ([]DocumentChunk, error)
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

	// Hybrid search configuration
	HybridSearchEnabled bool    // Global default for enabling hybrid search
	HybridVectorWeight  float64 // Default weight for vector similarity (0-1)
	HybridTextWeight    float64 // Default weight for BM25/text match (0-1)
}

// NewVectorDBConfigFromEnv creates a VectorDBConfig from environment variables.
func NewVectorDBConfigFromEnv() *VectorDBConfig {
	return &VectorDBConfig{
		StorageProvider:     getEnvOrDefault("LANCEDB_STORAGE_PROVIDER", "memory"),
		LocalPath:           getEnvOrDefault("LANCEDB_LOCAL_PATH", "build/data/lancedb"),
		S3Endpoint:          getEnvOrDefault("LANCEDB_S3_ENDPOINT", "fly.storage.tigris.dev"),
		S3Bucket:            os.Getenv("LANCEDB_S3_BUCKET"),
		S3Region:            getEnvOrDefault("LANCEDB_S3_REGION", "auto"),
		S3AccessKey:         os.Getenv("AWS_ACCESS_KEY_ID"),
		S3SecretKey:         os.Getenv("AWS_SECRET_ACCESS_KEY"),
		EmbeddingConfig:     NewEmbeddingConfigFromEnv(),
		Enabled:             os.Getenv("RAG_PIPELINE_ENABLED") == "true",
		HybridSearchEnabled: os.Getenv("HYBRID_SEARCH_ENABLED") == "true",
		HybridVectorWeight:  parseFloatOrDefault("HYBRID_VECTOR_WEIGHT", 0.7),
		HybridTextWeight:    parseFloatOrDefault("HYBRID_TEXT_WEIGHT", 0.3),
	}
}

// parseFloatOrDefault reads a float64 from an environment variable, returning default if not set or invalid.
func parseFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
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
	TopK     int     // Number of results to return
	MinScore float64 // Minimum similarity score (0-1)

	// Hybrid search parameters
	UseHybridSearch bool    // Enable hybrid mode (vector + BM25)
	VectorWeight    float64 // Weight for vector score (0-1, default: 0.7)
	TextWeight      float64 // Weight for BM25 score (0-1, default: 0.3)
}

// SearchResult holds the search results.
type SearchResult struct {
	Chunks  []DocumentChunk
	Scores  []float64 // Combined hybrid scores (or vector-only if hybrid disabled)
	Total   int       // Total matching documents
	Latency time.Duration

	// Hybrid search debug/analysis fields
	SearchMode   string    // "vector", "hybrid", or "fts"
	VectorScores []float64 // Raw vector similarity scores (optional)
	BM25Scores   []float64 // Raw BM25 scores (optional)
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
	slog.Info("Embedding service initialized", "provider", embedSvc.Provider(), "dimension", embedSvc.Dimension())

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

// InsertWithCheckpoint adds chunks with progress tracking (memory DB does simple insert).
func (db *MemoryVectorDB) InsertWithCheckpoint(ctx context.Context, chunks []DocumentChunk, opts InsertOptions) error {
	// Memory DB doesn't need batching, just call regular Insert
	if err := db.Insert(ctx, chunks); err != nil {
		return err
	}

	// Call checkpoint with final state if callback provided
	if opts.CheckpointFunc != nil {
		return opts.CheckpointFunc(1, len(chunks), 1, len(chunks))
	}
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

// Search performs vector or hybrid search based on query parameters.
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
		chunk       DocumentChunk
		score       float64
		vectorScore float64
		bm25Score   float64
	}

	var scored []scoredChunk

	// Determine weights for hybrid search
	vectorWeight := query.VectorWeight
	textWeight := query.TextWeight
	if query.UseHybridSearch && vectorWeight == 0 && textWeight == 0 {
		vectorWeight = 0.7
		textWeight = 0.3
	}

	// Build BM25 index if hybrid search is enabled
	var bm25Scorer *BM25Scorer
	if query.UseHybridSearch && query.QueryText != "" {
		bm25Scorer = NewBM25Scorer()
		for id, chunk := range db.chunks {
			// Only index chunks that pass filters
			if chunk.TenantID != query.TenantID {
				continue
			}
			if query.AudienceType != "" && chunk.AudienceType != query.AudienceType {
				continue
			}
			if query.ActiveOnly && !chunk.IsActive {
				continue
			}
			bm25Scorer.AddDocument(id, chunk.Title+" "+chunk.Content)
		}
	}

	// Filter and score chunks
	for id, chunk := range db.chunks {
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
			vectorScore := cosineSimilarity(queryEmbedding, chunk.Embedding)

			var finalScore float64
			var bm25Score float64

			if query.UseHybridSearch && bm25Scorer != nil {
				// Calculate BM25 score
				bm25Score = bm25Scorer.Score(query.QueryText, id)
				// Linear combination of vector and BM25 scores
				finalScore = vectorWeight*vectorScore + textWeight*bm25Score
			} else {
				finalScore = vectorScore
			}

			if finalScore >= query.MinScore {
				scored = append(scored, scoredChunk{
					chunk:       chunk,
					score:       finalScore,
					vectorScore: vectorScore,
					bm25Score:   bm25Score,
				})
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

	// Determine search mode
	searchMode := "vector"
	if query.UseHybridSearch {
		searchMode = "hybrid"
	}

	// Build result
	result := &SearchResult{
		Chunks:     make([]DocumentChunk, len(scored)),
		Scores:     make([]float64, len(scored)),
		Total:      len(scored),
		Latency:    time.Since(start),
		SearchMode: searchMode,
	}

	// Include component scores for hybrid search
	if query.UseHybridSearch {
		result.VectorScores = make([]float64, len(scored))
		result.BM25Scores = make([]float64, len(scored))
	}

	for i, sc := range scored {
		result.Chunks[i] = sc.chunk
		result.Scores[i] = sc.score
		if query.UseHybridSearch {
			result.VectorScores[i] = sc.vectorScore
			result.BM25Scores[i] = sc.bm25Score
		}
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

// ListChunks returns all chunks for a given tenant.
func (db *MemoryVectorDB) ListChunks(ctx context.Context, tenantID int32) ([]DocumentChunk, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var result []DocumentChunk
	for _, chunk := range db.chunks {
		if chunk.TenantID == tenantID {
			result = append(result, chunk)
		}
	}
	return result, nil
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

// InsertWithCheckpoint is a no-op.
func (db *NoOpVectorDB) InsertWithCheckpoint(ctx context.Context, chunks []DocumentChunk, opts InsertOptions) error {
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

// ListChunks returns empty results.
func (db *NoOpVectorDB) ListChunks(ctx context.Context, tenantID int32) ([]DocumentChunk, error) {
	return []DocumentChunk{}, nil
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
// BM25 SCORER (for Hybrid Search)
// ============================================================================

// BM25Scorer implements BM25 scoring for in-memory search.
// BM25 (Best Matching 25) is a ranking function used in information retrieval.
type BM25Scorer struct {
	k1        float64            // Term frequency saturation parameter (default: 1.2)
	b         float64            // Length normalization parameter (default: 0.75)
	docs      map[string][]string // Document ID -> tokenized words
	docFreq   map[string]int     // Term -> number of documents containing term
	avgLen    float64            // Average document length
	totalDocs int                // Total number of documents
}

// NewBM25Scorer creates a new BM25 scorer with standard parameters.
func NewBM25Scorer() *BM25Scorer {
	return &BM25Scorer{
		k1:      1.2,
		b:       0.75,
		docs:    make(map[string][]string),
		docFreq: make(map[string]int),
	}
}

// AddDocument adds a document to the BM25 index.
func (s *BM25Scorer) AddDocument(id, text string) {
	tokens := tokenize(text)
	s.docs[id] = tokens
	s.totalDocs++

	// Track which terms appear in this document (for IDF calculation)
	seen := make(map[string]bool)
	for _, token := range tokens {
		if !seen[token] {
			s.docFreq[token]++
			seen[token] = true
		}
	}

	// Recalculate average document length
	var totalLen int
	for _, docTokens := range s.docs {
		totalLen += len(docTokens)
	}
	s.avgLen = float64(totalLen) / float64(s.totalDocs)
}

// Score calculates the BM25 score for a query against a document.
// Returns a normalized score between 0 and 1.
func (s *BM25Scorer) Score(query, docID string) float64 {
	docTokens, exists := s.docs[docID]
	if !exists || s.totalDocs == 0 {
		return 0
	}

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return 0
	}

	// Count term frequencies in document
	termFreq := make(map[string]int)
	for _, token := range docTokens {
		termFreq[token]++
	}

	docLen := float64(len(docTokens))
	var score float64

	for _, term := range queryTokens {
		tf := float64(termFreq[term])
		if tf == 0 {
			continue
		}

		// IDF: log((N - n + 0.5) / (n + 0.5) + 1)
		// where N = total docs, n = docs containing term
		n := float64(s.docFreq[term])
		idf := math.Log((float64(s.totalDocs)-n+0.5)/(n+0.5) + 1)

		// BM25 term score: IDF * (tf * (k1 + 1)) / (tf + k1 * (1 - b + b * docLen/avgLen))
		numerator := tf * (s.k1 + 1)
		denominator := tf + s.k1*(1-s.b+s.b*docLen/s.avgLen)
		score += idf * numerator / denominator
	}

	// Normalize score to 0-1 range using sigmoid-like function
	// This ensures BM25 scores are comparable to cosine similarity scores
	normalized := score / (score + 1)
	return normalized
}

// tokenize splits text into lowercase tokens, removing common punctuation.
func tokenize(text string) []string {
	// Convert to lowercase and split on whitespace/punctuation
	text = strings.ToLower(text)

	// Replace common punctuation with spaces
	replacer := strings.NewReplacer(
		".", " ", ",", " ", "!", " ", "?", " ", ";", " ", ":", " ",
		"(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ",
		"\"", " ", "'", " ", "-", " ", "_", " ", "/", " ", "\\", " ",
	)
	text = replacer.Replace(text)

	// Split and filter empty strings
	words := strings.Fields(text)

	// Filter out very short words (likely noise)
	var tokens []string
	for _, word := range words {
		if len(word) >= 2 {
			tokens = append(tokens, word)
		}
	}

	return tokens
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

// HybridSearchOptions holds optional hybrid search configuration for retrieval.
type HybridSearchOptions struct {
	Enabled      bool    // Enable hybrid search
	VectorWeight float64 // Weight for vector similarity (0-1)
	TextWeight   float64 // Weight for BM25/text match (0-1)
}

// RetrieveContextForQuery performs retrieval based on user query.
// This simplified version searches all content types and lets embeddings rank relevance.
// The intent parameter is kept for backward compatibility but is no longer used for filtering.
func RetrieveContextForQuery(
	ctx context.Context,
	db VectorDB,
	query string,
	intent string, // Kept for API compatibility, no longer used for filtering
	tenantID int32,
	audienceType string,
	hybridOpts *HybridSearchOptions,
) (*RetrievedContext, error) {
	// Simplified: search all content types, let embeddings handle relevance
	// No longer filter by intent - embeddings are good at finding relevant content
	_ = intent // Unused, kept for backward compatibility

	searchQuery := SearchQuery{
		QueryText:    query,
		TenantID:     tenantID,
		AudienceType: audienceType,
		ContentTypes: []string{}, // Empty = search all types
		ActiveOnly:   true,
		TopK:         10,   // Fetch more results, let ranking sort them
		MinScore:     0.25, // Lower threshold, trust embeddings
	}

	// Apply hybrid search options if provided
	if hybridOpts != nil && hybridOpts.Enabled {
		searchQuery.UseHybridSearch = true
		searchQuery.VectorWeight = hybridOpts.VectorWeight
		searchQuery.TextWeight = hybridOpts.TextWeight
	}

	// Perform search
	result, err := db.Search(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Return all results as KBSections (simplified, no type-based bucketing)
	return &RetrievedContext{
		KBSections: result.Chunks,
	}, nil
}
