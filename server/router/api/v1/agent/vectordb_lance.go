//go:build rag

package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// legacyTableName is the old fixed table name used before dimension-based naming.
// This is kept for migration purposes only.
const legacyTableName = "kb_documents"

// minIVFPQIndexRows is the minimum number of rows required to train the PQ
// codebook for an IVF-PQ vector index in LanceDB. Below this threshold, index
// creation is skipped and search falls back to sequential scan.
const minIVFPQIndexRows int64 = 256

// getTableNameForDimension returns the table name for a given embedding dimension.
// Format: kb_documents_<dimension> (e.g., kb_documents_1536, kb_documents_384)
func getTableNameForDimension(dim int) string {
	return fmt.Sprintf("kb_documents_%d", dim)
}

// LanceVectorDB is a LanceDB-backed implementation of VectorDB.
type LanceVectorDB struct {
	conn            contracts.IConnection
	table           contracts.ITable
	embedSvc        EmbeddingService
	config          *VectorDBConfig
	mu              sync.RWMutex
	validatedMu     sync.Mutex
	lastValidatedAt time.Time
	hasVectorIndex  bool   // Track if IVF-PQ index has been created (requires data)
	tableName       string // Computed from embedding dimension (e.g., kb_documents_1536)
}

// newLanceVectorDB creates a new LanceDB-backed vector database.
func newLanceVectorDB(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
	ctx := context.Background()

	var connOpts *contracts.ConnectionOptions
	var uri string

	switch config.StorageProvider {
	case "s3":
		if config.S3Bucket == "" {
			return nil, fmt.Errorf("LANCEDB_S3_BUCKET is required for S3 storage")
		}
		// Use pre-built URI if set (pool passes per-tenant URI), otherwise build from bucket
		if config.URI != "" {
			uri = config.URI
		} else {
			uri = fmt.Sprintf("s3://%s/lancedb", config.S3Bucket)
		}
		// Ensure endpoint has URL scheme — LanceDB's Rust S3 client ignores bare hostnames
		s3Endpoint := config.S3Endpoint
		if s3Endpoint != "" && !strings.HasPrefix(s3Endpoint, "http://") && !strings.HasPrefix(s3Endpoint, "https://") {
			s3Endpoint = "https://" + s3Endpoint
		}
		s3Config := &contracts.S3Config{
			Endpoint:       ptr(s3Endpoint),
			Region:         ptr(config.S3Region),
			ForcePathStyle: ptr(config.S3ForcePathStyle),
		}
		// Only pass credentials if set — on Fly.io with Tigris, IAM role auth is used
		// instead of explicit keys. Setting empty credentials overrides the IAM role chain.
		if config.S3AccessKey != "" {
			s3Config.AccessKeyID = ptr(config.S3AccessKey)
			s3Config.SecretAccessKey = ptr(config.S3SecretKey)
		}
		connOpts = &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				AllowHTTP: ptr(config.S3AllowHTTP),
				S3Config:  s3Config,
			},
		}
		// LanceDB Go bindings silently drop S3Config.Endpoint — set env var for Rust object_store
		os.Setenv("AWS_ENDPOINT_URL_S3", s3Endpoint)
		os.Setenv("AWS_ENDPOINT_URL", s3Endpoint)
	default: // "local"
		uri = config.LocalPath
		// Ensure directory exists
		if err := os.MkdirAll(config.LocalPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create LanceDB directory: %w", err)
		}
		connOpts = &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				LocalConfig: &contracts.LocalConfig{
					CreateDirIfNotExists: ptr(true),
				},
			},
		}
	}

	// Connect to LanceDB
	conn, err := lancedb.Connect(ctx, uri, connOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LanceDB at %s: %w", uri, err)
	}

	// Compute table name from embedding dimension
	tableName := getTableNameForDimension(embedSvc.Dimension())

	db := &LanceVectorDB{
		conn:      conn,
		embedSvc:  embedSvc,
		config:    config,
		tableName: tableName,
	}

	// Migrate legacy table if it exists
	if err := db.migrateLegacyTable(ctx); err != nil {
		slog.Warn("Legacy table migration warning", "error", err)
		// Non-fatal: continue with dimension-based table
	}

	// Open or create the table
	if err := db.ensureTable(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ensure table: %w", err)
	}

	slog.Info("LanceDB vector database initialized",
		"uri", uri,
		"provider", config.StorageProvider,
		"tableName", tableName,
		"dimension", embedSvc.Dimension())
	return db, nil
}

// ensureTable opens the table if it exists, or creates it if it doesn't.
func (db *LanceVectorDB) ensureTable(ctx context.Context) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Check if table exists
	tableNames, err := db.conn.TableNames(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}

	tableExists := false
	for _, name := range tableNames {
		if name == db.tableName {
			tableExists = true
			break
		}
	}

	if tableExists {
		table, err := db.conn.OpenTable(ctx, db.tableName)
		if err != nil {
			return fmt.Errorf("failed to open table: %w", err)
		}
		db.table = table
		slog.Debug("Opened existing LanceDB table", "name", db.tableName)
	} else {
		// Create schema
		schema, err := db.buildSchema()
		if err != nil {
			return fmt.Errorf("failed to build schema: %w", err)
		}

		table, err := db.conn.CreateTable(ctx, db.tableName, schema)
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
		db.table = table

		// Create indexes
		if err := db.createIndexes(ctx); err != nil {
			slog.Warn("Failed to create indexes", "error", err)
		}

		slog.Info("Created new LanceDB table", "name", db.tableName)
	}

	return nil
}

// buildSchema creates the Arrow schema for the documents table.
func (db *LanceVectorDB) buildSchema() (contracts.ISchema, error) {
	if db.embedSvc == nil {
		return nil, fmt.Errorf("embedding service is required to build schema")
	}
	dim := db.embedSvc.Dimension()

	schema, err := lancedb.NewSchemaBuilder().
		AddStringField("id", false).
		AddInt32Field("tenant_id", false).
		AddStringField("audience_type", false).
		AddStringField("content_type", false).
		AddStringField("title", true).
		AddStringField("content", false).
		AddStringField("code", true).
		AddBooleanField("is_emergency", true).
		AddBooleanField("is_active", false).
		AddInt32Field("priority", true).
		AddVectorField("embedding", dim, contracts.VectorDataTypeFloat32, false).
		AddInt64Field("indexed_at", false).
		AddInt32Field("source_version", true).
		Build()

	if err != nil {
		return nil, fmt.Errorf("failed to build schema: %w", err)
	}

	return schema, nil
}

// createIndexes creates necessary indexes on the table.
// Note: IVF-PQ vector index is NOT created here because it requires training data.
// The vector index is created lazily in ensureVectorIndex() after first Insert().
func (db *LanceVectorDB) createIndexes(ctx context.Context) error {
	// Skip IVF-PQ vector index on empty table - requires training data
	// Will be created on first Insert() via ensureVectorIndex()

	// Create BTree index for tenant filtering - works on empty tables
	if err := db.table.CreateIndexWithName(ctx, []string{"tenant_id"}, contracts.IndexTypeBTree, "idx_tenant"); err != nil {
		slog.Warn("Failed to create tenant index", "error", err)
	}

	// Create FTS (Full-Text Search) index for BM25 keyword search
	// This enables hybrid search combining vector similarity with keyword matching
	if err := db.table.CreateIndexWithName(ctx, []string{"content"}, contracts.IndexTypeFts, "idx_content_fts"); err != nil {
		slog.Warn("Failed to create FTS index on content", "error", err)
	}

	// Create FTS index on title for title-based keyword search
	if err := db.table.CreateIndexWithName(ctx, []string{"title"}, contracts.IndexTypeFts, "idx_title_fts"); err != nil {
		slog.Warn("Failed to create FTS index on title", "error", err)
	}

	return nil
}

// ensureVectorIndex creates the IVF-PQ vector index if it doesn't exist.
// This must be called AFTER data has been inserted, as IVF-PQ requires training data.
func (db *LanceVectorDB) ensureVectorIndex(ctx context.Context) error {
	if db.hasVectorIndex {
		return nil
	}

	// IVF-PQ requires at least 256 rows for PQ codebook training.
	// Skip index creation for small datasets; search falls back to sequential scan.
	// hasVectorIndex intentionally remains false so future inserts can retry once
	// row count reaches the threshold.
	count, err := db.table.Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to get table count: %w", err)
	}
	if count < minIVFPQIndexRows {
		slog.Debug("Skipping IVF-PQ index: not enough rows for training",
			"table", db.tableName, "count", count, "required", minIVFPQIndexRows)
		return nil
	}

	// Create IVF-PQ vector index now that we have data for training
	if err := db.table.CreateIndexWithName(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq, "idx_embedding"); err != nil {
		// Check if index already exists (not an error)
		if strings.Contains(err.Error(), "already exists") {
			db.hasVectorIndex = true
			return nil
		}
		return fmt.Errorf("failed to create vector index: %w", err)
	}

	db.hasVectorIndex = true
	slog.Info("Created IVF-PQ vector index", "table", db.tableName)
	return nil
}

// migrateLegacyTable checks for the old "kb_documents" table and drops it if found.
// With dimension-based table naming, the legacy table is no longer used.
// Data must be reindexed after migration.
func (db *LanceVectorDB) migrateLegacyTable(ctx context.Context) error {
	// Check if legacy table exists
	tableNames, err := db.conn.TableNames(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}

	legacyExists := false
	for _, name := range tableNames {
		if name == legacyTableName {
			legacyExists = true
			break
		}
	}

	if !legacyExists {
		return nil // No migration needed
	}

	slog.Warn("Found legacy 'kb_documents' table - this data will be dropped",
		"reason", "migrating to dimension-based table naming",
		"newTable", db.tableName,
		"action", "Please rebuild indexes for all tenants after restart")

	// Drop the legacy table
	if err := db.conn.DropTable(ctx, legacyTableName); err != nil {
		// Ignore "not found" errors
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "was not found") {
			return fmt.Errorf("failed to drop legacy table: %w", err)
		}
	}

	slog.Info("Dropped legacy 'kb_documents' table - please rebuild indexes for all tenants")
	return nil
}

// Dimension returns the embedding dimension for this VectorDB instance.
func (db *LanceVectorDB) Dimension() int {
	if db.embedSvc == nil {
		return 0
	}
	return db.embedSvc.Dimension()
}

// getTableEmbeddingDimension returns the embedding dimension from the table schema.
// Returns 0 if unable to determine (table doesn't exist or error).
func (db *LanceVectorDB) getTableEmbeddingDimension(ctx context.Context) int {
	if db.table == nil {
		return 0
	}

	// Get schema from table (works even if table is empty)
	schema, err := db.table.Schema(ctx)
	if err != nil {
		slog.Debug("Could not get table schema for dimension check", "error", err)
		return 0
	}

	// Find the embedding field and extract dimension from fixed_size_list type
	for i := 0; i < schema.NumFields(); i++ {
		field := schema.Field(i)
		if field.Name == "embedding" {
			// embedding is a fixed_size_list type, extract the size
			if listType, ok := field.Type.(*arrow.FixedSizeListType); ok {
				return int(listType.Len())
			}
		}
	}

	slog.Debug("Could not find embedding field in schema")
	return 0
}

// dropAndRecreateTable drops the existing table and creates a new one with the current schema.
func (db *LanceVectorDB) dropAndRecreateTable(ctx context.Context) error {
	// Close existing table handle
	if db.table != nil {
		if err := db.table.Close(); err != nil {
			slog.Warn("Failed to close table before drop", "error", err)
		}
		db.table = nil
	}

	// Drop the table (ignore "not found" error - table may not exist)
	if err := db.conn.DropTable(ctx, db.tableName); err != nil {
		// Ignore "table not found" errors - this is expected if table was already deleted
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "was not found") {
			return fmt.Errorf("failed to drop table: %w", err)
		}
		slog.Info("Table did not exist, creating fresh", "table", db.tableName)
	}

	slog.Info("Dropped existing LanceDB table due to dimension mismatch", "table", db.tableName)

	// Create new table with current schema
	schema, err := db.buildSchema()
	if err != nil {
		return fmt.Errorf("failed to build schema: %w", err)
	}

	table, err := db.conn.CreateTable(ctx, db.tableName, schema)
	if err != nil {
		return fmt.Errorf("failed to create new table: %w", err)
	}
	db.table = table

	// Reset vector index flag - new table is empty, index will be created on first Insert()
	db.hasVectorIndex = false

	// Recreate BTree and FTS indexes (IVF-PQ vector index deferred to first Insert)
	if err := db.createIndexes(ctx); err != nil {
		slog.Warn("Failed to create indexes on new table", "error", err)
	}

	slog.Info("Created new LanceDB table with updated schema", "table", db.tableName)
	return nil
}

// Insert adds or updates chunks in the database.
func (db *LanceVectorDB) Insert(ctx context.Context, chunks []DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Batch size for embedding and insertion (handles large files)
	// Configurable via EMBEDDING_BATCH_SIZE env var (default: 25, max: 200)
	batchSize := GetEmbeddingBatchSize()

	totalChunks := len(chunks)
	slog.Info("Starting batched insert", "totalChunks", totalChunks, "batchSize", batchSize, "table", db.tableName)

	// NOTE: Dimension mismatch check removed - with dimension-based table naming,
	// each dimension gets its own table (e.g., kb_documents_1536, kb_documents_384).
	// This ensures different embedding providers can coexist without data loss.

	// Process chunks in batches
	for batchStart := 0; batchStart < totalChunks; batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > totalChunks {
			batchEnd = totalChunks
		}

		batch := chunks[batchStart:batchEnd]
		batch = expandAndValidateBatch(batch, embeddingLimit(db.embedSvc)) // per-batch expansion; totalChunks pre-calculated above
		batchNum := (batchStart / batchSize) + 1
		totalBatches := (totalChunks + batchSize - 1) / batchSize

		slog.Info("Processing batch",
			"batch", batchNum,
			"totalBatches", totalBatches,
			"chunksInBatch", len(batch),
			"progress", fmt.Sprintf("%d/%d", batchEnd, totalChunks))

		// Generate embeddings for chunks that don't have them
		var textsToEmbed []string
		var indicesToEmbed []int

		for i, chunk := range batch {
			if len(chunk.Embedding) == 0 {
				textsToEmbed = append(textsToEmbed, fmt.Sprintf("%s: %s", chunk.Title, chunk.Content))
				indicesToEmbed = append(indicesToEmbed, i)
			}
		}

		if len(textsToEmbed) > 0 {
			embeddings, failed := db.embedWithIsolation(ctx, textsToEmbed)

			allFailed := len(failed) > 0
			for _, f := range failed {
				if !f {
					allFailed = false
					break
				}
			}
			if allFailed {
				return fmt.Errorf("failed to generate embeddings for batch %d: all %d chunks failed to embed (systemic embedding failure): %w", batchNum, len(textsToEmbed), ErrEmbeddingProviderUnavailable)
			}

			for i, idx := range indicesToEmbed {
				if failed[i] {
					slog.Error("Skipping chunk due to embedding failure (batch continues)",
						"chunkID", batch[idx].ID,
						"title", batch[idx].Title,
						"tenantID", batch[idx].TenantID,
						"contentLength", len(batch[idx].Content))
					continue
				}
				batch[idx].Embedding = embeddings[i]
			}
		}

		// Build Arrow record for this batch
		record, err := db.chunksToArrowRecord(batch)
		if err != nil {
			return fmt.Errorf("failed to convert batch %d to Arrow record: %w", batchNum, err)
		}

		// Add batch to table
		if err := db.table.Add(ctx, record, nil); err != nil {
			record.Release()
			return fmt.Errorf("failed to add batch %d to LanceDB: %w", batchNum, err)
		}
		record.Release()
	}

	// Create IVF-PQ vector index now that we have data
	// This is deferred from table creation because IVF-PQ requires training data
	if err := db.ensureVectorIndex(ctx); err != nil {
		slog.Warn("Failed to create vector index after insert", "error", err)
		// Non-fatal: search will still work, just slower without index
	}

	slog.Info("Completed batched insert", "totalChunks", totalChunks)
	return nil
}

// InsertWithCheckpoint adds chunks with resume support and progress tracking.
// CheckpointCallback and InsertOptions types are defined in vectordb.go
func (db *LanceVectorDB) InsertWithCheckpoint(ctx context.Context, chunks []DocumentChunk, opts InsertOptions) error {
	if len(chunks) == 0 {
		return nil
	}

	// Default options
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = 5 * time.Second
	}

	// Batch size configurable via EMBEDDING_BATCH_SIZE env var (default: 200, min: 10, max: 200)
	batchSize := GetEmbeddingBatchSize()
	totalChunks := len(chunks)
	totalBatches := (totalChunks + batchSize - 1) / batchSize
	startBatch := opts.StartBatch

	slog.Info("Starting batched insert with checkpoint support",
		"totalChunks", totalChunks,
		"batchSize", batchSize,
		"startBatch", startBatch,
		"totalBatches", totalBatches,
		"table", db.tableName)

	// NOTE: Dimension mismatch check removed - with dimension-based table naming,
	// each dimension gets its own table (e.g., kb_documents_1536, kb_documents_384).

	// Progress-aware budgeting: track throughput and abort if projected total exceeds cap.
	// MaxReindexDuration is WriteTimeout(35m) - 5m margin to avoid split-brain.
	const (
		MaxReindexDuration     = 30 * time.Minute
		MinSampleBatches       = 3
		MinSampleElapsed       = 2 * time.Minute
		MaxConsecutiveFailures = 3
	)
	startTime := time.Now()
	consecutiveFailures := 0

	// Process chunks in batches starting from startBatch
	for batchNum := startBatch; batchNum < totalBatches; batchNum++ {
		batchStart := batchNum * batchSize
		batchEnd := batchStart + batchSize
		if batchEnd > totalChunks {
			batchEnd = totalChunks
		}

		batch := chunks[batchStart:batchEnd]

		slog.Info("Processing batch",
			"batch", batchNum+1,
			"totalBatches", totalBatches,
			"chunksInBatch", len(batch),
			"progress", fmt.Sprintf("%d/%d", batchEnd, totalChunks))

		// Per-batch mutex: hold lock only during embedding+write, not between batches.
		// This lets Search() proceed while we sleep/wait between batches.
		db.mu.Lock()
		err := db.processBatchWithRetry(ctx, batch, batchNum+1, opts.MaxRetries, opts.RetryDelay)
		db.mu.Unlock()
		if err != nil {
			consecutiveFailures++
			if consecutiveFailures >= MaxConsecutiveFailures {
				return fmt.Errorf("circuit breaker: %d consecutive batch failures, aborting reindex at batch %d/%d: %w",
					consecutiveFailures, batchNum+1, totalBatches, err)
			}
			return fmt.Errorf("failed at batch %d: %w", batchNum+1, err)
		}

		// Reset consecutive failure counter on success
		consecutiveFailures = 0

		// Call checkpoint callback after successful batch
		if opts.CheckpointFunc != nil {
			if err := opts.CheckpointFunc(batchNum+1, batchEnd, totalBatches, totalChunks, len(batch)); err != nil {
				slog.Warn("Checkpoint callback failed", "batch", batchNum+1, "error", err)
				// Non-fatal: continue processing
			}
		}

		// Progress-aware projection: after minimum samples, check if projected total exceeds cap.
		// Prevents the operation from silently running for hours when config or provider is degraded.
		elapsed := time.Since(startTime)
		batchesCompleted := batchNum - startBatch + 1
		if (batchesCompleted >= MinSampleBatches && elapsed >= MinSampleElapsed) || batchNum+1 == totalBatches {
			rate := float64(batchEnd) / elapsed.Minutes()
			if rate > 0 {
				projected := time.Duration(float64(totalChunks)/rate) * time.Minute
				if projected > MaxReindexDuration {
					return fmt.Errorf("reindex too slow: projected %v but cap is %v (rate: %.0f chunks/min, batch %d/%d)",
						projected, MaxReindexDuration, rate, batchNum+1, totalBatches)
				}
				slog.Info("Reindex progress projection",
					"batch", batchNum+1, "totalBatches", totalBatches,
					"elapsed", elapsed.Round(time.Second),
					"rate", fmt.Sprintf("%.0f chunks/min", rate),
					"projected", projected.Round(time.Second),
					"cap", MaxReindexDuration)
			}
		}
	}

	// Create IVF-PQ vector index now that we have data
	db.mu.Lock()
	if err := db.ensureVectorIndex(ctx); err != nil {
		slog.Warn("Failed to create vector index after insert", "error", err)
	}
	db.mu.Unlock()

	slog.Info("Completed batched insert with checkpoint", "totalChunks", totalChunks)
	return nil
}

// processBatchWithRetry processes a single batch with exponential backoff retry.
func (db *LanceVectorDB) processBatchWithRetry(ctx context.Context, batch []DocumentChunk, batchNum, maxRetries int, initialDelay time.Duration) error {
	var lastErr error
	delay := initialDelay
	maxDelay := 60 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := db.processSingleBatch(ctx, batch, batchNum)
		if err == nil {
			return nil
		}

		lastErr = err
		if !isRetryableError(err) {
			return fmt.Errorf("batch %d failed with permanent error: %w", batchNum, err)
		}
		if attempt < maxRetries-1 {
			slog.Warn("Batch failed, retrying",
				"batch", batchNum,
				"attempt", attempt+1,
				"maxRetries", maxRetries,
				"delay", delay,
				"error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay = delay * 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}

	return fmt.Errorf("batch %d failed after %d retries: %w", batchNum, maxRetries, lastErr)
}

// Validate checks the embedding provider and LanceDB table before a reindex starts.
func (db *LanceVectorDB) Validate(ctx context.Context) error {
	db.validatedMu.Lock()
	defer db.validatedMu.Unlock()

	if !db.lastValidatedAt.IsZero() && time.Since(db.lastValidatedAt) < 30*time.Second {
		return nil
	}

	if db.embedSvc == nil {
		return fmt.Errorf("%w: embedding service not initialized", ErrEmbeddingProviderMisconfigured)
	}
	if _, err := db.embedSvc.Embed(ctx, []string{"preflight"}); err != nil {
		if errors.Is(err, ErrEmbeddingProviderMisconfigured) || errors.Is(err, ErrEmbeddingProviderUnavailable) {
			return err
		}
		return fmt.Errorf("%w: %v", ErrEmbeddingProviderUnavailable, err)
	}

	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.conn == nil || db.table == nil {
		return fmt.Errorf("%w: LanceDB connection or table is not initialized", ErrVectorStoreUnavailable)
	}
	if _, err := db.table.Count(ctx); err != nil {
		return fmt.Errorf("%w: LanceDB table count failed: %v", ErrVectorStoreUnavailable, err)
	}
	if _, err := db.table.Version(ctx); err != nil {
		return fmt.Errorf("%w: LanceDB table version failed: %v", ErrVectorStoreUnavailable, err)
	}

	db.lastValidatedAt = time.Now()
	return nil
}

// embeddingLimit returns the embedding model's hard input token limit for the
// given service. For OpenRouter it is the authoritative model limit; for local
// and mock providers it is math.MaxInt32 (unlimited), so validation is skipped.
// (Plan 8 / R5)
func embeddingLimit(embedSvc EmbeddingService) int {
	if ore, ok := embedSvc.(*OpenRouterEmbedding); ok {
		return ore.MaxInputTokens()
	}
	return math.MaxInt32
}

// processSingleBatch processes a single batch of chunks.
// expandAndValidateBatch splits chunks whose combined Title+Content exceeds the
// model's input limit, preventing 400 errors from the embedding API. Each
// oversized chunk is split via splitByHardLimit with a content limit adjusted
// for title overhead. The embedding boundary (OpenRouterEmbedding.doEmbed)
// is the authoritative final guard; this is defense-in-depth.
func expandAndValidateBatch(batch []DocumentChunk, limit int) []DocumentChunk {
	// When the provider has no hard limit, skip validation entirely.
	if limit >= math.MaxInt32 {
		return batch
	}
	// Keep a safety margin consistent with the embedding boundary.
	guardLimit := limit - embedSafetyMargin
	if guardLimit <= 0 {
		guardLimit = limit
	}
	var expanded []DocumentChunk
	for _, chunk := range batch {
		embedText := fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)
		if EstimateTokens(embedText) > guardLimit {
			// contentLimit excludes the title + ": " overhead
			titleCost := EstimateTokens(chunk.Title) + 2
			contentLimit := guardLimit - titleCost
			if contentLimit < 100 {
				contentLimit = 100
			}
			slog.Error("Oversized embedding input detected and split",
				"tokens", EstimateTokens(embedText),
				"limit", guardLimit,
				"title", chunk.Title,
				"contentLength", len(chunk.Content),
				"contentPreview", chunk.Content[:min(200, len(chunk.Content))])
			parts := splitByHardLimit(chunk.Content, contentLimit)
			for p, part := range parts {
				newChunk := chunk
				newChunk.Content = part
				newChunk.Title = fmt.Sprintf("%s (Part %d)", chunk.Title, p+1)
				newChunk.Code = fmt.Sprintf("%s_split_%d", chunk.Code, p+1)
				newChunk.ID = fmt.Sprintf("%s_split_%d", chunk.ID, p+1)
				expanded = append(expanded, newChunk)
			}
		} else {
			expanded = append(expanded, chunk)
		}
	}
	return expanded
}

func (db *LanceVectorDB) processSingleBatch(ctx context.Context, batch []DocumentChunk, batchNum int) error {
	batch = expandAndValidateBatch(batch, embeddingLimit(db.embedSvc))

	// Generate embeddings for chunks that don't have them
	var textsToEmbed []string
	var indicesToEmbed []int

	for i, chunk := range batch {
		if len(chunk.Embedding) == 0 {
			textsToEmbed = append(textsToEmbed, fmt.Sprintf("%s: %s", chunk.Title, chunk.Content))
			indicesToEmbed = append(indicesToEmbed, i)
		}
	}

	if len(textsToEmbed) > 0 {
		// embedWithIsolation embeds the batch, retrying item-by-item and
		// skipping any chunk that still fails (Plan 8 / R3). This prevents a
		// single bad chunk from aborting the entire reindex.
		embeddings, failed := db.embedWithIsolation(ctx, textsToEmbed)

		// Partial failures are isolated (skipped). But if EVERY chunk in the
		// batch failed to embed, the cause is almost certainly systemic (e.g.
		// a misconfigured/down embedding provider) rather than a few bad
		// chunks — abort loudly so the reindex does not silently produce an
		// empty index (Plan 8 / R3 refinement).
		allFailed := len(failed) > 0
		for _, f := range failed {
			if !f {
				allFailed = false
				break
			}
		}
		if allFailed {
			return fmt.Errorf("failed to generate embeddings for batch %d: all %d chunks failed to embed (systemic embedding failure): %w", batchNum, len(textsToEmbed), ErrEmbeddingProviderUnavailable)
		}

		for k, idx := range indicesToEmbed {
			if failed[k] {
				slog.Error("Skipping chunk due to embedding failure (batch continues)",
					"chunkID", batch[idx].ID,
					"title", batch[idx].Title,
					"tenantID", batch[idx].TenantID,
					"contentLength", len(batch[idx].Content))
				continue
			}
			batch[idx].Embedding = embeddings[k]
		}
	}

	// Keep only chunks that received an embedding; skipped chunks are dropped
	// so the Arrow record and index stay consistent.
	var kept []DocumentChunk
	for _, c := range batch {
		if len(c.Embedding) > 0 {
			kept = append(kept, c)
		}
	}
	if len(kept) == 0 {
		slog.Warn("Batch had no embeddable chunks after isolation; skipping", "batch", batchNum)
		return nil
	}
	batch = kept

	// Build Arrow record for this batch
	record, err := db.chunksToArrowRecord(batch)
	if err != nil {
		return fmt.Errorf("failed to convert batch %d to Arrow record: %w", batchNum, err)
	}

	// Add batch to table
	if err := db.table.Add(ctx, record, nil); err != nil {
		record.Release()
		return fmt.Errorf("failed to add batch %d to LanceDB: %w", batchNum, err)
	}
	record.Release()

	return nil
}

// embedWithIsolation embeds a batch of texts with per-item fault isolation
// (Plan 8 / R3). It first attempts a single batched Embed call. If that fails,
// it retries each text individually and records which indices failed so the
// caller can skip them instead of aborting the whole batch.
//
// Returns embeddings aligned 1:1 with texts, and failed[k]=true for any text
// that could not be embedded.
func (db *LanceVectorDB) embedWithIsolation(ctx context.Context, texts []string) ([][]float32, []bool) {
	embeddings, err := db.embedSvc.Embed(ctx, texts)
	if err == nil {
		return embeddings, make([]bool, len(texts))
	}

	slog.Warn("Batch embedding failed; retrying items individually",
		"texts", len(texts),
		"error", err.Error())

	out := make([][]float32, len(texts))
	failed := make([]bool, len(texts))
	for i, t := range texts {
		emb, e2 := db.embedSvc.Embed(ctx, []string{t})
		if e2 != nil || len(emb) == 0 {
			failed[i] = true
			slog.Warn("Individual embedding failed; chunk will be skipped",
				"index", i,
				"error", firstErr(e2, err))
			continue
		}
		out[i] = emb[0]
	}
	return out, failed
}

// firstErr returns a human-readable error string from the most specific error.
func firstErr(errs ...error) string {
	for _, e := range errs {
		if e != nil {
			return e.Error()
		}
	}
	return "unknown error"
}
func (db *LanceVectorDB) chunksToArrowRecord(chunks []DocumentChunk) (arrow.Record, error) {
	if db.embedSvc == nil {
		return nil, fmt.Errorf("embedding service is required")
	}
	pool := memory.NewGoAllocator()
	dim := db.embedSvc.Dimension()

	// Build schema
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "tenant_id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "audience_type", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "content_type", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "title", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "content", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "code", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "is_emergency", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
		{Name: "is_active", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
		{Name: "priority", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "embedding", Type: arrow.FixedSizeListOf(int32(dim), arrow.PrimitiveTypes.Float32), Nullable: false},
		{Name: "indexed_at", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "source_version", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
	}, nil)

	// Create builders
	idBuilder := array.NewStringBuilder(pool)
	tenantIDBuilder := array.NewInt32Builder(pool)
	audienceTypeBuilder := array.NewStringBuilder(pool)
	contentTypeBuilder := array.NewStringBuilder(pool)
	titleBuilder := array.NewStringBuilder(pool)
	contentBuilder := array.NewStringBuilder(pool)
	codeBuilder := array.NewStringBuilder(pool)
	isEmergencyBuilder := array.NewBooleanBuilder(pool)
	isActiveBuilder := array.NewBooleanBuilder(pool)
	priorityBuilder := array.NewInt32Builder(pool)
	embeddingBuilder := array.NewFixedSizeListBuilder(pool, int32(dim), arrow.PrimitiveTypes.Float32)
	indexedAtBuilder := array.NewInt64Builder(pool)
	sourceVersionBuilder := array.NewInt32Builder(pool)

	defer idBuilder.Release()
	defer tenantIDBuilder.Release()
	defer audienceTypeBuilder.Release()
	defer contentTypeBuilder.Release()
	defer titleBuilder.Release()
	defer contentBuilder.Release()
	defer codeBuilder.Release()
	defer isEmergencyBuilder.Release()
	defer isActiveBuilder.Release()
	defer priorityBuilder.Release()
	defer embeddingBuilder.Release()
	defer indexedAtBuilder.Release()
	defer sourceVersionBuilder.Release()

	now := time.Now().Unix()

	for _, chunk := range chunks {
		// Sanitize ALL string fields to prevent UTF-8 errors in Arrow serialization
		idBuilder.Append(strings.ToValidUTF8(chunk.ID, ""))
		tenantIDBuilder.Append(chunk.TenantID)
		audienceTypeBuilder.Append(strings.ToValidUTF8(chunk.AudienceType, ""))
		contentTypeBuilder.Append(strings.ToValidUTF8(chunk.ContentType, ""))
		titleBuilder.Append(strings.ToValidUTF8(chunk.Title, ""))
		contentBuilder.Append(strings.ToValidUTF8(chunk.Content, ""))
		codeBuilder.Append(strings.ToValidUTF8(chunk.Code, ""))
		isEmergencyBuilder.Append(chunk.IsEmergency)
		isActiveBuilder.Append(chunk.IsActive)
		priorityBuilder.Append(chunk.Priority)

		// Build embedding array
		embeddingBuilder.Append(true)
		valueBuilder := embeddingBuilder.ValueBuilder().(*array.Float32Builder)
		for _, v := range chunk.Embedding {
			valueBuilder.Append(v)
		}
		// Pad if embedding is shorter than dimension
		for i := len(chunk.Embedding); i < dim; i++ {
			valueBuilder.Append(0)
		}

		indexedAtBuilder.Append(now)
		sourceVersionBuilder.Append(chunk.SourceVersion)
	}

	// Build arrays
	idArr := idBuilder.NewArray()
	tenantIDArr := tenantIDBuilder.NewArray()
	audienceTypeArr := audienceTypeBuilder.NewArray()
	contentTypeArr := contentTypeBuilder.NewArray()
	titleArr := titleBuilder.NewArray()
	contentArr := contentBuilder.NewArray()
	codeArr := codeBuilder.NewArray()
	isEmergencyArr := isEmergencyBuilder.NewArray()
	isActiveArr := isActiveBuilder.NewArray()
	priorityArr := priorityBuilder.NewArray()
	embeddingArr := embeddingBuilder.NewArray()
	indexedAtArr := indexedAtBuilder.NewArray()
	sourceVersionArr := sourceVersionBuilder.NewArray()

	defer idArr.Release()
	defer tenantIDArr.Release()
	defer audienceTypeArr.Release()
	defer contentTypeArr.Release()
	defer titleArr.Release()
	defer contentArr.Release()
	defer codeArr.Release()
	defer isEmergencyArr.Release()
	defer isActiveArr.Release()
	defer priorityArr.Release()
	defer embeddingArr.Release()
	defer indexedAtArr.Release()
	defer sourceVersionArr.Release()

	// Create record
	record := array.NewRecord(schema, []arrow.Array{
		idArr, tenantIDArr, audienceTypeArr, contentTypeArr, titleArr,
		contentArr, codeArr, isEmergencyArr, isActiveArr, priorityArr,
		embeddingArr, indexedAtArr, sourceVersionArr,
	}, int64(len(chunks)))

	return record, nil
}

// Delete removes chunks matching the filter criteria.
func (db *LanceVectorDB) Delete(ctx context.Context, tenantID int32, audienceType string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	filter := fmt.Sprintf("tenant_id = %d AND audience_type = '%s'", tenantID, audienceType)
	if err := db.table.Delete(ctx, filter); err != nil {
		return fmt.Errorf("failed to delete from LanceDB: %w", err)
	}

	slog.Debug("Deleted chunks from LanceDB", "tenantID", tenantID, "audience", audienceType)
	return nil
}

// DeleteByIDPrefix removes chunks whose IDs start with the given prefix.
// Returns the number of chunks deleted.
func (db *LanceVectorDB) DeleteByIDPrefix(ctx context.Context, tenantID int32, idPrefix string) (int, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// LanceDB uses SQL-like filter syntax
	// Use LIKE for prefix matching
	filter := fmt.Sprintf("tenant_id = %d AND id LIKE '%s%%'", tenantID, idPrefix)

	// Note: LanceDB doesn't return count from Delete, so we return 0
	// The actual deletion is logged for debugging

	if err := db.table.Delete(ctx, filter); err != nil {
		return 0, fmt.Errorf("failed to delete by ID prefix from LanceDB: %w", err)
	}

	slog.Debug("Deleted chunks by ID prefix from LanceDB",
		"tenantID", tenantID,
		"idPrefix", idPrefix)
	return 0, nil // Return 0 since LanceDB doesn't provide count
}

// TableName returns the table name for this VectorDB instance.
// Useful for debugging dimension mismatch issues.
func (db *LanceVectorDB) TableName() string {
	return db.tableName
}

// DeleteByVersion removes chunks for a specific (tenant, audience, file_type, version).
func (db *LanceVectorDB) DeleteByVersion(ctx context.Context, tenantID int32, audienceType, fileType string, version int32) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	filter := fmt.Sprintf("tenant_id = %d AND audience_type = '%s' AND content_type = '%s' AND source_version = %d",
		tenantID, audienceType, fileType, version)
	if err := db.table.Delete(ctx, filter); err != nil {
		return fmt.Errorf("failed to delete version from LanceDB: %w", err)
	}
	slog.Debug("Deleted versioned chunks from LanceDB",
		"tenantID", tenantID, "audience", audienceType, "fileType", fileType, "version", version)
	return nil
}

// PurgePreVersionedChunks removes chunks that predate versioning
// (source_version IS NULL OR 0 OR 1).
func (db *LanceVectorDB) PurgePreVersionedChunks(ctx context.Context, tenantID int32, audienceType, fileType string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	filter := fmt.Sprintf(
		"tenant_id = %d AND audience_type = '%s' AND content_type = '%s' AND (source_version IS NULL OR source_version = 0 OR source_version = 1)",
		tenantID, audienceType, fileType)
	if err := db.table.Delete(ctx, filter); err != nil {
		return fmt.Errorf("failed to purge pre-versioned chunks from LanceDB: %w", err)
	}
	slog.Debug("Purged pre-versioned chunks from LanceDB",
		"tenantID", tenantID, "audience", audienceType, "fileType", fileType)
	return nil
}

// ListIndexedVersions returns the distinct indexed source_version values for a
// given (tenant, audience, file_type).
func (db *LanceVectorDB) ListIndexedVersions(ctx context.Context, tenantID int32, audienceType, fileType string) ([]int32, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	filter := fmt.Sprintf("tenant_id = %d AND audience_type = '%s' AND content_type = '%s'",
		tenantID, audienceType, fileType)
	records, err := db.table.Query().Filter(filter).Limit(200000).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to query indexed versions: %w", err)
	}

	seen := make(map[int32]struct{})
	for _, rec := range records {
		idx := -1
		fields := rec.Schema().Fields()
		for i, f := range fields {
			if f.Name == "source_version" {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		if arr, ok := rec.Column(idx).(*array.Int32); ok {
			for r := 0; r < arr.Len(); r++ {
				if arr.IsNull(r) {
					continue
				}
				seen[arr.Value(r)] = struct{}{}
			}
		}
	}

	versions := make([]int32, 0, len(seen))
	for v := range seen {
		versions = append(versions, v)
	}
	return versions, nil
}

// Search performs vector or hybrid search based on query parameters.
func (db *LanceVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	start := time.Now()

	db.mu.RLock()
	defer db.mu.RUnlock()

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

	// Validate query embedding dimension matches table dimension
	// This catches edge cases where the embedding model changed without server restart
	expectedDim := db.embedSvc.Dimension()
	actualDim := len(queryEmbedding)
	if actualDim != expectedDim {
		return nil, fmt.Errorf("embedding dimension mismatch: query has %d dims but embedding service expects %d dims. "+
			"This may indicate the embedding model changed. Please restart server or rebuild index",
			actualDim, expectedDim)
	}

	// Check table dimension matches embedding service dimension
	// This is a safety check for when embedding provider changed between runs
	tableDim := db.getTableEmbeddingDimension(ctx)
	if tableDim > 0 && tableDim != expectedDim {
		return nil, fmt.Errorf("embedding dimension mismatch: table '%s' has %d dims but current embedding service uses %d dims. "+
			"The embedding model may have changed since content was indexed. Please rebuild the index with 'Rebuild Index' button in Agent Admin",
			db.tableName, tableDim, expectedDim)
	}

	// Build filter
	filter := db.buildFilter(query)

	// Determine topK
	topK := query.TopK
	if topK <= 0 {
		topK = 10
	}

	// Determine weights for hybrid search
	vectorWeight := query.VectorWeight
	textWeight := query.TextWeight
	if query.UseHybridSearch && vectorWeight == 0 && textWeight == 0 {
		vectorWeight = 0.7
		textWeight = 0.3
	}

	// Execute search based on mode
	if query.UseHybridSearch && query.QueryText != "" {
		return db.hybridSearch(ctx, query.QueryText, queryEmbedding, filter, topK, query.MinScore, vectorWeight, textWeight, start)
	}

	// Vector-only search (default)
	return db.vectorOnlySearch(ctx, queryEmbedding, filter, topK, query.MinScore, start)
}

// buildFilter constructs the SQL filter string from query parameters.
func (db *LanceVectorDB) buildFilter(query SearchQuery) string {
	var filterParts []string
	filterParts = append(filterParts, fmt.Sprintf("tenant_id = %d", query.TenantID))

	if query.AudienceType != "" {
		filterParts = append(filterParts, fmt.Sprintf("audience_type = '%s'", query.AudienceType))
	}

	if query.ActiveOnly {
		filterParts = append(filterParts, "is_active = true")
	}

	if len(query.ContentTypes) > 0 {
		types := make([]string, len(query.ContentTypes))
		for i, ct := range query.ContentTypes {
			types[i] = fmt.Sprintf("'%s'", ct)
		}
		filterParts = append(filterParts, fmt.Sprintf("content_type IN (%s)", strings.Join(types, ", ")))
	}

	if query.SourceVersion != nil {
		filterParts = append(filterParts, fmt.Sprintf("source_version = %d", *query.SourceVersion))
	}

	return strings.Join(filterParts, " AND ")
}

// vectorOnlySearch performs pure vector similarity search.
func (db *LanceVectorDB) vectorOnlySearch(ctx context.Context, queryEmbedding []float32, filter string, topK int, minScore float64, start time.Time) (*SearchResult, error) {
	results, err := db.table.VectorSearchWithFilter(ctx, "embedding", queryEmbedding, topK, filter)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	chunks := make([]DocumentChunk, 0, len(results))
	scores := make([]float64, 0, len(results))

	for _, row := range results {
		chunk := db.rowToDocumentChunk(row)
		score := db.distanceToScore(row)

		if score >= minScore {
			chunks = append(chunks, chunk)
			scores = append(scores, score)
		}
	}

	return &SearchResult{
		Chunks:     chunks,
		Scores:     scores,
		Total:      len(chunks),
		Latency:    time.Since(start),
		SearchMode: "vector",
	}, nil
}

// hybridSearch performs combined vector + FTS search with score fusion.
func (db *LanceVectorDB) hybridSearch(ctx context.Context, queryText string, queryEmbedding []float32, filter string, topK int, minScore float64, vectorWeight, textWeight float64, start time.Time) (*SearchResult, error) {
	// Fetch more candidates for fusion (2x topK from each source)
	candidateK := topK * 2

	// Run vector search
	vectorResults, err := db.table.VectorSearchWithFilter(ctx, "embedding", queryEmbedding, candidateK, filter)
	if err != nil {
		slog.Warn("Vector search failed in hybrid mode", "error", err)
		vectorResults = nil
	}

	// Run FTS search on content column
	ftsResults, err := db.table.FullTextSearchWithFilter(ctx, "content", queryText, filter)
	if err != nil {
		// FTS may not be supported - fall back to vector-only
		slog.Debug("FTS search failed, falling back to vector-only", "error", err)
		return db.vectorOnlySearch(ctx, queryEmbedding, filter, topK, minScore, start)
	}

	// Build score maps for fusion
	type scoredDoc struct {
		chunk       DocumentChunk
		vectorScore float64
		ftsScore    float64
		hybridScore float64
	}

	docMap := make(map[string]*scoredDoc)

	// Process vector results
	for i, row := range vectorResults {
		chunk := db.rowToDocumentChunk(row)
		score := db.distanceToScore(row)

		if _, exists := docMap[chunk.ID]; !exists {
			docMap[chunk.ID] = &scoredDoc{chunk: chunk}
		}
		docMap[chunk.ID].vectorScore = score

		// Track rank for potential RRF fusion (not currently used, but available)
		_ = i
	}

	// Process FTS results — collect raw scores for min-max normalization
	var ftsRawScores []float64
	type ftsEntry struct {
		chunkID string
		raw     float64
	}
	var ftsEntries []ftsEntry

	for i, row := range ftsResults {
		chunk := db.rowToDocumentChunk(row)

		// Collect raw FTS score (no normalization yet)
		var rawScore float64
		if score, ok := row["_score"].(float64); ok {
			rawScore = score
		} else if score, ok := row["_score"].(float32); ok {
			rawScore = float64(score)
		} else {
			// Use rank-based score if no _score available
			rawScore = 1.0 / float64(i+1)
		}

		ftsRawScores = append(ftsRawScores, rawScore)
		ftsEntries = append(ftsEntries, ftsEntry{chunkID: chunk.ID, raw: rawScore})

		if _, exists := docMap[chunk.ID]; !exists {
			docMap[chunk.ID] = &scoredDoc{chunk: chunk}
		}
	}

	// Min-max normalize FTS scores to [0, 1] so they're comparable to cosine similarity
	normalizedFTS := normalizeBM25Scores(ftsRawScores)
	for i, entry := range ftsEntries {
		docMap[entry.chunkID].ftsScore = normalizedFTS[i]
	}

	// Calculate hybrid scores using linear combination
	for _, doc := range docMap {
		doc.hybridScore = vectorWeight*doc.vectorScore + textWeight*doc.ftsScore
	}

	// Convert to slice and sort by hybrid score
	docs := make([]*scoredDoc, 0, len(docMap))
	for _, doc := range docMap {
		if doc.hybridScore >= minScore {
			docs = append(docs, doc)
		}
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].hybridScore > docs[j].hybridScore
	})

	// Apply topK limit
	if len(docs) > topK {
		docs = docs[:topK]
	}

	// Build result
	result := &SearchResult{
		Chunks:       make([]DocumentChunk, len(docs)),
		Scores:       make([]float64, len(docs)),
		Total:        len(docs),
		Latency:      time.Since(start),
		SearchMode:   "hybrid",
		VectorScores: make([]float64, len(docs)),
		BM25Scores:   make([]float64, len(docs)),
	}

	for i, doc := range docs {
		result.Chunks[i] = doc.chunk
		result.Scores[i] = doc.hybridScore
		result.VectorScores[i] = doc.vectorScore
		result.BM25Scores[i] = doc.ftsScore
	}

	return result, nil
}

// distanceToScore converts LanceDB distance to similarity score (0-1).
func (db *LanceVectorDB) distanceToScore(row map[string]interface{}) float64 {
	// LanceDB returns L2 distance, convert to similarity
	if dist, ok := row["_distance"].(float64); ok {
		return 1.0 / (1.0 + dist)
	}
	if dist, ok := row["_distance"].(float32); ok {
		return 1.0 / (1.0 + float64(dist))
	}
	return 1.0
}

// rowToDocumentChunk converts a LanceDB result row to a DocumentChunk.
func (db *LanceVectorDB) rowToDocumentChunk(row map[string]interface{}) DocumentChunk {
	chunk := DocumentChunk{}

	if v, ok := row["id"].(string); ok {
		chunk.ID = v
	}
	if v, ok := row["tenant_id"].(int32); ok {
		chunk.TenantID = v
	} else if v, ok := row["tenant_id"].(float64); ok {
		chunk.TenantID = int32(v)
	}
	if v, ok := row["audience_type"].(string); ok {
		chunk.AudienceType = v
	}
	if v, ok := row["content_type"].(string); ok {
		chunk.ContentType = v
	}
	if v, ok := row["title"].(string); ok {
		chunk.Title = v
	}
	if v, ok := row["content"].(string); ok {
		chunk.Content = v
	}
	if v, ok := row["code"].(string); ok {
		chunk.Code = v
	}
	if v, ok := row["is_emergency"].(bool); ok {
		chunk.IsEmergency = v
	}
	if v, ok := row["is_active"].(bool); ok {
		chunk.IsActive = v
	}
	if v, ok := row["priority"].(int32); ok {
		chunk.Priority = v
	} else if v, ok := row["priority"].(float64); ok {
		chunk.Priority = int32(v)
	}
	if v, ok := row["source_version"].(int32); ok {
		chunk.SourceVersion = v
	} else if v, ok := row["source_version"].(float64); ok {
		chunk.SourceVersion = int32(v)
	}

	// Embedding is typically not returned in search results to save bandwidth
	// but we can handle it if present
	if v, ok := row["embedding"].([]float32); ok {
		chunk.Embedding = v
	} else if v, ok := row["embedding"].([]interface{}); ok {
		chunk.Embedding = make([]float32, len(v))
		for i, val := range v {
			if f, ok := val.(float64); ok {
				chunk.Embedding[i] = float32(f)
			} else if f, ok := val.(float32); ok {
				chunk.Embedding[i] = f
			}
		}
	}

	return chunk
}

// Close releases resources.
func (db *LanceVectorDB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.table != nil {
		if err := db.table.Close(); err != nil {
			slog.Warn("Failed to close LanceDB table", "error", err)
		}
		db.table = nil
	}

	if db.conn != nil {
		if err := db.conn.Close(); err != nil {
			slog.Warn("Failed to close LanceDB connection", "error", err)
		}
		db.conn = nil
	}

	slog.Info("LanceDB vector database closed")
	return nil
}

// Stats returns database statistics.
func (db *LanceVectorDB) Stats(ctx context.Context) (*VectorDBStats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := &VectorDBStats{
		TenantCounts:  make(map[int32]int64),
		ContentCounts: make(map[string]int64),
	}

	// Get total count
	count, err := db.table.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get count: %w", err)
	}
	stats.TotalChunks = count

	// Get version as a proxy for tracking changes
	version, err := db.table.Version(ctx)
	if err == nil {
		stats.IndexSize = int64(version) // Use version as a rough indicator
	}

	return stats, nil
}

// ListChunks returns all chunks for a given tenant using SelectWithFilter.
func (db *LanceVectorDB) ListChunks(ctx context.Context, tenantID int32) ([]DocumentChunk, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	filter := fmt.Sprintf("tenant_id = %d", tenantID)
	rows, err := db.table.SelectWithFilter(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}

	chunks := make([]DocumentChunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, db.rowToDocumentChunk(row))
	}
	return chunks, nil
}

// ptr is a helper to create pointers to values.
func ptr[T any](v T) *T {
	return &v
}
