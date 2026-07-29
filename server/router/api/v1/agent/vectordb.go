package agent

import (
	"context"
	"errors"
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

// CheckpointFunc is a callback called after each successful batch.
type CheckpointFunc func(currentBatch, processedChunks, totalBatches, totalChunks, chunksInBatch int) error

// InsertOptions configures the InsertWithCheckpoint operation.
type InsertOptions struct {
	StartBatch     int            // Resume from this batch (0-indexed)
	CheckpointFunc CheckpointFunc // Called after each batch
	MaxRetries     int            // Max retries per batch (default: 3)
	RetryDelay     time.Duration  // Initial delay between retries (default: 5s)
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

	// DeleteByVersion removes chunks for a specific (tenant, audience, file_type, version).
	// Used for retention cleanup and cutover of pre-versioning data.
	DeleteByVersion(ctx context.Context, tenantID int32, audienceType, fileType string, version int32) error

	// PurgePreVersionedChunks removes chunks that predate versioning
	// (source_version IS NULL OR 0 OR 1). Used for one-time cutover before the
	// first versioned reindex.
	PurgePreVersionedChunks(ctx context.Context, tenantID int32, audienceType, fileType string) error

	// DeleteByIDPrefix removes chunks whose IDs start with the given prefix.
	// This is useful for deleting all observations for a specific session.
	DeleteByIDPrefix(ctx context.Context, tenantID int32, idPrefix string) (int, error)

	// ListIndexedVersions returns the distinct indexed source_version values for a
	// given (tenant, audience, file_type). Used to resolve the default query version.
	ListIndexedVersions(ctx context.Context, tenantID int32, audienceType, fileType string) ([]int32, error)

	// Search performs hybrid search (vector + metadata filtering).
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)

	// Close releases resources.
	Close() error

	// Stats returns database statistics.
	Stats(ctx context.Context) (*VectorDBStats, error)

	// ListChunks returns all chunks for a given tenant (used for stats/counting).
	ListChunks(ctx context.Context, tenantID int32) ([]DocumentChunk, error)

	// Dimension returns the embedding dimension this VectorDB handles.
	// Returns 0 if not applicable (e.g., NoOpVectorDB).
	Dimension() int

	// Validate checks if the database and its dependencies (like embedding API) are functional.
	Validate(ctx context.Context) error
}

// VectorDBConfig holds configuration for the vector database.
type VectorDBConfig struct {
	// Storage configuration
	StorageProvider string // "memory", "local", or "s3"
	LocalPath       string // For local: "build/data/lancedb/"

	// URI override — when set, newLanceVectorDB uses this instead of building from bucket/path.
	// Used by the per-tenant pool to pass pre-built per-tenant URIs.
	URI string

	// S3/Tigrisdata configuration (for production)
	S3Endpoint      string // "t3.storage.dev" for Tigrisdata (canonical)
	S3Bucket        string
	S3Region        string // "auto" for Tigrisdata
	S3AccessKey     string
	S3SecretKey     string
	S3ForcePathStyle bool   // false for Tigris (virtual-hosted); true for MinIO/R2 path-style
	S3AllowHTTP     bool   // false for production (HTTPS); true for local dev
	S3Prefix        string // Deployment namespace prefix (default "lancedb"); set to app name for multi-deploy isolation

	// Embedding configuration
	EmbeddingConfig *EmbeddingConfig

	// CockroachDB configuration (for vector storage)
	CockroachDSN string // Connection string for CockroachDB

	// RAG feature flag
	Enabled bool

	// Hybrid search configuration
	HybridSearchEnabled bool    // Global default for enabling hybrid search
	HybridVectorWeight  float64 // Default weight for vector similarity (0-1)
	HybridTextWeight    float64 // Default weight for BM25/text match (0-1)
}

// NewVectorDBConfigFromEnv creates a VectorDBConfig from environment variables.
func NewVectorDBConfigFromEnv() *VectorDBConfig {
	// S3 endpoint: prefer LANCEDB_S3_ENDPOINT, fallback to AWS_ENDPOINT_URL_S3
	s3Endpoint := getEnvOrDefault("LANCEDB_S3_ENDPOINT", "")
	if s3Endpoint == "" {
		s3Endpoint = getEnvOrDefault("AWS_ENDPOINT_URL_S3", "t3.storage.dev")
	}

	return &VectorDBConfig{
		StorageProvider:     getEnvOrDefault("LANCEDB_STORAGE_PROVIDER", "memory"),
		LocalPath:           getEnvOrDefault("LANCEDB_LOCAL_PATH", "build/data/lancedb"),
		S3Endpoint:          s3Endpoint,
		S3Bucket:            os.Getenv("LANCEDB_S3_BUCKET"),
		S3Region:            getEnvOrDefault("LANCEDB_S3_REGION", "auto"),
		S3AccessKey:         os.Getenv("AWS_ACCESS_KEY_ID"),
		S3SecretKey:         os.Getenv("AWS_SECRET_ACCESS_KEY"),
		S3ForcePathStyle:    getEnvOrDefault("LANCEDB_S3_FORCE_PATH_STYLE", "false") == "true",
		S3AllowHTTP:         getEnvOrDefault("LANCEDB_S3_ALLOW_HTTP", "false") == "true",
		S3Prefix:            getEnvOrDefault("LANCEDB_S3_PREFIX", "lancedb"),
		CockroachDSN:        os.Getenv("COCKROACH_DSN"),
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
	SourceVersion *int32  // Optional: only return chunks for this indexed source version

	// Pagination
	TopK     int     // Number of results to return
	MinScore float64 // Minimum similarity score (0-1)

	// Hybrid search parameters
	UseHybridSearch bool    // Enable hybrid mode (vector + BM25)
	VectorWeight    float64 // Weight for vector score (0-1, default: 0.7)
	TextWeight      float64 // Weight for BM25 score (0-1, default: 0.3)

	// Temporal weighting parameters (for Hybrid OM + RAG)
	UseTemporalWeighting bool      // Enable temporal weighting
	ReferenceTime        time.Time // Reference time for temporal calculations (default: now)
	TemporalDecay        float64   // Decay factor per day (default: 0.1)
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
	TotalChunks   int64
	TenantCounts  map[int32]int64
	ContentCounts map[string]int64
	IndexSize     int64 // in bytes
	LastOptimized time.Time
}

// TenantS3Override holds per-tenant S3 storage overrides.
// Stored as JSON in TenantConfig.VectorDBS3Override.
type TenantS3Override struct {
	Bucket        string `json:"bucket,omitempty"`
	Prefix        string `json:"prefix,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	Region        string `json:"region,omitempty"`
	AccessKeyID   string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	ForcePathStyle *bool `json:"force_path_style,omitempty"`
}

// resolveStorageTarget resolves the S3 URI and config for a tenant,
// applying per-tenant overrides on top of global config.
func resolveStorageTarget(global *VectorDBConfig, override *TenantS3Override, tenantID int32) (uri string, s3Cfg *VectorDBConfig) {
	// Clone global config
	resolved := *global

	if override != nil {
		if override.Bucket != "" {
			resolved.S3Bucket = override.Bucket
		}
		if override.Endpoint != "" {
			resolved.S3Endpoint = override.Endpoint
		}
		if override.Region != "" {
			resolved.S3Region = override.Region
		}
		if override.AccessKeyID != "" {
			resolved.S3AccessKey = override.AccessKeyID
		}
		if override.SecretAccessKey != "" {
			resolved.S3SecretKey = override.SecretAccessKey
		}
		if override.ForcePathStyle != nil {
			resolved.S3ForcePathStyle = *override.ForcePathStyle
		}
	}

	// Build S3 URI with tenant prefix
	prefix := global.S3Prefix
	if prefix == "" {
		prefix = "lancedb"
	}
	if override != nil && override.Prefix != "" {
		prefix = override.Prefix
	}
	uri = fmt.Sprintf("s3://%s/%s/%d", resolved.S3Bucket, prefix, tenantID)

	return uri, &resolved
}

// resolveLocalTarget resolves the local storage path for a tenant.
func resolveLocalTarget(config *VectorDBConfig, tenantID int32) string {
	return fmt.Sprintf("%s/%d", config.LocalPath, tenantID)
}

// NewVectorDB creates a vector database based on configuration.
func NewVectorDB(config *VectorDBConfig) (VectorDB, error) {
	if !config.Enabled {
		slog.Info("RAG pipeline disabled, using no-op vector database")
		return NewNoOpVectorDB(), nil
	}

	// Initialize tokenizer for accurate token counting
	InitTokenizer(config.EmbeddingConfig.Provider, config.EmbeddingConfig.Model)
	// Capture config so EstimateTokens can self-heal if init was missed (Plan 8 / R4)
	SetEstimateTokenizerConfig(config.EmbeddingConfig)

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
		return newPool(config, embedSvc)
	case "s3":
		slog.Info("Using S3 LanceDB storage", "endpoint", config.S3Endpoint, "bucket", config.S3Bucket)
		return newPool(config, embedSvc)
	case "cockroach":
		slog.Info("Using CockroachDB native vector storage")
		return NewCockroachVectorDB(config, embedSvc)
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
		return opts.CheckpointFunc(1, len(chunks), 1, len(chunks), len(chunks))
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

// DeleteByIDPrefix removes chunks whose IDs start with the given prefix.
// Returns the number of chunks deleted.
func (db *MemoryVectorDB) DeleteByIDPrefix(ctx context.Context, tenantID int32, idPrefix string) (int, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var toDelete []string
	for id, chunk := range db.chunks {
		if chunk.TenantID == tenantID && strings.HasPrefix(id, idPrefix) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		delete(db.chunks, id)
	}

	slog.Debug("Deleted chunks by ID prefix from memory vector DB",
		"count", len(toDelete),
		"tenantID", tenantID,
		"idPrefix", idPrefix)
	return len(toDelete), nil
}

// DeleteByVersion removes chunks for a specific (tenant, audience, file_type, version).
func (db *MemoryVectorDB) DeleteByVersion(ctx context.Context, tenantID int32, audienceType, fileType string, version int32) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	var toDelete []string
	for id, chunk := range db.chunks {
		if chunk.TenantID == tenantID && chunk.AudienceType == audienceType &&
			chunk.ContentType == fileType && chunk.SourceVersion == version {
			toDelete = append(toDelete, id)
		}
	}
	for _, id := range toDelete {
		delete(db.chunks, id)
	}
	return nil
}

// PurgePreVersionedChunks removes chunks that predate versioning (SourceVersion 0 or 1).
func (db *MemoryVectorDB) PurgePreVersionedChunks(ctx context.Context, tenantID int32, audienceType, fileType string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	var toDelete []string
	for id, chunk := range db.chunks {
		if chunk.TenantID == tenantID && chunk.AudienceType == audienceType &&
			chunk.ContentType == fileType && (chunk.SourceVersion == 0 || chunk.SourceVersion == 1) {
			toDelete = append(toDelete, id)
		}
	}
	for _, id := range toDelete {
		delete(db.chunks, id)
	}
	return nil
}

// ListIndexedVersions returns distinct indexed SourceVersion values for a (tenant, audience, file_type).
func (db *MemoryVectorDB) ListIndexedVersions(ctx context.Context, tenantID int32, audienceType, fileType string) ([]int32, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	seen := make(map[int32]struct{})
	for _, chunk := range db.chunks {
		if chunk.TenantID == tenantID && chunk.AudienceType == audienceType && chunk.ContentType == fileType {
			seen[chunk.SourceVersion] = struct{}{}
		}
	}
	versions := make([]int32, 0, len(seen))
	for v := range seen {
		versions = append(versions, v)
	}
	return versions, nil
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
	// First pass: collect raw scores for BM25 min-max normalization
	type rawScored struct {
		id          string
		chunk       DocumentChunk
		vectorScore float64
		bm25Raw     float64
	}
	var rawScoredChunks []rawScored

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
			var bm25Raw float64
			if query.UseHybridSearch && bm25Scorer != nil {
				bm25Raw = bm25Scorer.ScoreRaw(query.QueryText, id)
			}
			rawScoredChunks = append(rawScoredChunks, rawScored{
				id:          id,
				chunk:       chunk,
				vectorScore: vectorScore,
				bm25Raw:     bm25Raw,
			})
		}
	}

	// Second pass: min-max normalize BM25 scores, then combine with vector scores
	if query.UseHybridSearch && bm25Scorer != nil && len(rawScoredChunks) > 0 {
		rawScores := make([]float64, len(rawScoredChunks))
		for i, r := range rawScoredChunks {
			rawScores[i] = r.bm25Raw
		}
		normalized := normalizeBM25Scores(rawScores)

		for i, r := range rawScoredChunks {
			finalScore := vectorWeight*r.vectorScore + textWeight*normalized[i]

			if query.UseTemporalWeighting && !r.chunk.IndexedAt.IsZero() {
				temporalWeight := calculateTemporalWeight(r.chunk.IndexedAt, query.ReferenceTime, query.TemporalDecay)
				finalScore = finalScore * temporalWeight
			}

			if finalScore >= query.MinScore {
				scored = append(scored, scoredChunk{
					chunk:       r.chunk,
					score:       finalScore,
					vectorScore: r.vectorScore,
					bm25Score:   normalized[i],
				})
			}
		}
	} else {
		for _, r := range rawScoredChunks {
			finalScore := r.vectorScore

			if query.UseTemporalWeighting && !r.chunk.IndexedAt.IsZero() {
				temporalWeight := calculateTemporalWeight(r.chunk.IndexedAt, query.ReferenceTime, query.TemporalDecay)
				finalScore = finalScore * temporalWeight
			}

			if finalScore >= query.MinScore {
				scored = append(scored, scoredChunk{
					chunk:       r.chunk,
					score:       finalScore,
					vectorScore: r.vectorScore,
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

// Dimension returns the embedding dimension for this VectorDB instance.
func (db *MemoryVectorDB) Dimension() int {
	if db.embedSvc == nil {
		return 0
	}
	return db.embedSvc.Dimension()
}

// Validate checks if the database and its dependencies (like embedding API) are functional.
func (db *MemoryVectorDB) Validate(ctx context.Context) error {
	if db.embedSvc == nil {
		return fmt.Errorf("%w: embedding service not initialized", ErrEmbeddingProviderMisconfigured)
	}
	_, err := db.embedSvc.Embed(ctx, []string{"preflight"})
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrEmbeddingProviderMisconfigured) || errors.Is(err, ErrEmbeddingProviderUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrEmbeddingProviderUnavailable, err)
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

// DeleteByIDPrefix is a no-op.
func (db *NoOpVectorDB) DeleteByIDPrefix(ctx context.Context, tenantID int32, idPrefix string) (int, error) {
	return 0, nil
}

// DeleteByVersion is a no-op.
func (db *NoOpVectorDB) DeleteByVersion(ctx context.Context, tenantID int32, audienceType, fileType string, version int32) error {
	return nil
}

// PurgePreVersionedChunks is a no-op.
func (db *NoOpVectorDB) PurgePreVersionedChunks(ctx context.Context, tenantID int32, audienceType, fileType string) error {
	return nil
}

// ListIndexedVersions returns empty for NoOp.
func (db *NoOpVectorDB) ListIndexedVersions(ctx context.Context, tenantID int32, audienceType, fileType string) ([]int32, error) {
	return nil, nil
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

// Dimension returns 0 (not applicable for no-op).
func (db *NoOpVectorDB) Dimension() int {
	return 0
}

// Validate is a no-op for disabled RAG.
func (db *NoOpVectorDB) Validate(ctx context.Context) error {
	return nil
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

// calculateTemporalWeight calculates a weight based on the age of content.
// Recent content gets higher weight, older content gets lower weight.
func calculateTemporalWeight(contentTime time.Time, referenceTime time.Time, decayFactor float64) float64 {
	if contentTime.IsZero() {
		return 0.5 // Default for unknown times
	}

	// Use current time if reference time is not set
	if referenceTime.IsZero() {
		referenceTime = time.Now()
	}

	// Calculate age in days
	age := referenceTime.Sub(contentTime).Hours() / 24

	// Apply decay
	switch {
	case age < 1:
		return 1.0 // Full weight for today
	case age < 7:
		// Linear decay from 1.0 to 0.3 over a week
		return 1.0 - (age * 0.1)
	case age < 30:
		// Slower decay from 0.3 to 0.1 over a month
		return 0.3 - ((age - 7) * 0.01)
	default:
		// Minimum weight for old content
		return 0.1
	}
}

// ============================================================================
// BM25 SCORER (for Hybrid Search)
// ============================================================================

// BM25Scorer implements BM25 scoring for in-memory search.
// BM25 (Best Matching 25) is a ranking function used in information retrieval.
type BM25Scorer struct {
	k1        float64             // Term frequency saturation parameter (default: 1.2)
	b         float64             // Length normalization parameter (default: 0.75)
	docs      map[string][]string // Document ID -> tokenized words
	docFreq   map[string]int      // Term -> number of documents containing term
	avgLen    float64             // Average document length
	totalDocs int                 // Total number of documents
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
// Returns a normalized score between 0 and 1 using min-max normalization.
func (s *BM25Scorer) Score(query, docID string) float64 {
	return s.ScoreRaw(query, docID)
}

// ScoreRaw calculates the raw BM25 score (unnormalized) for a query against a document.
// Callers should use min-max normalization across all scored documents before combining
// with other score types (e.g., cosine similarity) to ensure comparable ranges.
func (s *BM25Scorer) ScoreRaw(query, docID string) float64 {
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

	return score
}

// normalizeBM25Scores applies min-max normalization to a slice of raw BM25 scores,
// mapping them to [0, 1]. Returns zeros if all scores are identical.
func normalizeBM25Scores(scores []float64) []float64 {
	if len(scores) == 0 {
		return scores
	}
	min, max := scores[0], scores[0]
	for _, s := range scores[1:] {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}
	range_ := max - min
	if range_ == 0 {
		// All scores identical — return 1.0 for all (perfect match to themselves)
		result := make([]float64, len(scores))
		for i := range result {
			result[i] = 1.0
		}
		return result
	}
	result := make([]float64, len(scores))
	for i, s := range scores {
		result[i] = (s - min) / range_
	}
	return result
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
	Scores     []float64
}

func (r *RetrievedContext) topScore() float64 {
	if len(r.Scores) > 0 {
		return r.Scores[0]
	}
	return 0
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

	// If audience is internal, also consider external content (production chat uses internal audience)
	searchAudience := audienceType
	if audienceType == "internal" {
		searchAudience = "" // Search both to find any production-ready content
	}

	searchQuery := SearchQuery{
		QueryText:    query,
		TenantID:     tenantID,
		AudienceType: searchAudience,
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
		Scores:     result.Scores,
	}, nil
}
