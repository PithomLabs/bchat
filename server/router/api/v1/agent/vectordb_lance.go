//go:build rag

package agent

import (
	"context"
	"fmt"
	"log/slog"
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

const (
	lanceTableName     = "kb_documents"
	embeddingDimension = 384 // Default for all-MiniLM-L6-v2
)

// LanceVectorDB is a LanceDB-backed implementation of VectorDB.
type LanceVectorDB struct {
	conn           contracts.IConnection
	table          contracts.ITable
	embedSvc       EmbeddingService
	config         *VectorDBConfig
	mu             sync.RWMutex
	hasVectorIndex bool // Track if IVF-PQ index has been created (requires data)
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
		uri = fmt.Sprintf("s3://%s/lancedb", config.S3Bucket)
		connOpts = &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				S3Config: &contracts.S3Config{
					Endpoint:        ptr(config.S3Endpoint),
					Region:          ptr(config.S3Region),
					AccessKeyID:     ptr(config.S3AccessKey),
					SecretAccessKey: ptr(config.S3SecretKey),
					ForcePathStyle:  ptr(true),
				},
			},
		}
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

	db := &LanceVectorDB{
		conn:     conn,
		embedSvc: embedSvc,
		config:   config,
	}

	// Open or create the table
	if err := db.ensureTable(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ensure table: %w", err)
	}

	slog.Info("LanceDB vector database initialized", "uri", uri, "provider", config.StorageProvider)
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
		if name == lanceTableName {
			tableExists = true
			break
		}
	}

	if tableExists {
		table, err := db.conn.OpenTable(ctx, lanceTableName)
		if err != nil {
			return fmt.Errorf("failed to open table: %w", err)
		}
		db.table = table
		slog.Debug("Opened existing LanceDB table", "name", lanceTableName)
	} else {
		// Create schema
		schema, err := db.buildSchema()
		if err != nil {
			return fmt.Errorf("failed to build schema: %w", err)
		}

		table, err := db.conn.CreateTable(ctx, lanceTableName, schema)
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
		db.table = table

		// Create indexes
		if err := db.createIndexes(ctx); err != nil {
			slog.Warn("Failed to create indexes", "error", err)
		}

		slog.Info("Created new LanceDB table", "name", lanceTableName)
	}

	return nil
}

// buildSchema creates the Arrow schema for the documents table.
func (db *LanceVectorDB) buildSchema() (contracts.ISchema, error) {
	dim := embeddingDimension
	if db.embedSvc != nil {
		dim = db.embedSvc.Dimension()
	}

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
	slog.Info("Created IVF-PQ vector index", "table", lanceTableName)
	return nil
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
	if err := db.conn.DropTable(ctx, lanceTableName); err != nil {
		// Ignore "table not found" errors - this is expected if table was already deleted
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "was not found") {
			return fmt.Errorf("failed to drop table: %w", err)
		}
		slog.Info("Table did not exist, creating fresh", "table", lanceTableName)
	}

	slog.Info("Dropped existing LanceDB table due to dimension mismatch", "table", lanceTableName)

	// Create new table with current schema
	schema, err := db.buildSchema()
	if err != nil {
		return fmt.Errorf("failed to build schema: %w", err)
	}

	table, err := db.conn.CreateTable(ctx, lanceTableName, schema)
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

	slog.Info("Created new LanceDB table with updated schema", "table", lanceTableName)
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
	// Smaller batches (25) reduce individual request time and timeout risk
	const batchSize = 25

	totalChunks := len(chunks)
	slog.Info("Starting batched insert", "totalChunks", totalChunks, "batchSize", batchSize)

	// Check for dimension mismatch before processing (use first chunk as sample)
	// We need to embed at least one chunk to check dimensions
	if len(chunks) > 0 && len(chunks[0].Embedding) == 0 {
		sampleText := fmt.Sprintf("%s: %s", chunks[0].Title, chunks[0].Content)
		sampleEmbeddings, err := db.embedSvc.Embed(ctx, []string{sampleText})
		if err != nil {
			return fmt.Errorf("failed to generate sample embedding: %w", err)
		}
		chunks[0].Embedding = sampleEmbeddings[0]

		existingDim := db.getTableEmbeddingDimension(ctx)
		newDim := len(chunks[0].Embedding)

		if existingDim > 0 && existingDim != newDim {
			slog.Warn("Embedding dimension mismatch detected, recreating table",
				"table", lanceTableName,
				"existingDimension", existingDim,
				"newDimension", newDim,
				"reason", "embedding provider likely changed")

			if err := db.dropAndRecreateTable(ctx); err != nil {
				return fmt.Errorf("failed to recreate table for dimension change: %w", err)
			}
		}
	}

	// Process chunks in batches
	for batchStart := 0; batchStart < totalChunks; batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > totalChunks {
			batchEnd = totalChunks
		}

		batch := chunks[batchStart:batchEnd]
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
			embeddings, err := db.embedSvc.Embed(ctx, textsToEmbed)
			if err != nil {
				return fmt.Errorf("failed to generate embeddings for batch %d: %w", batchNum, err)
			}

			for i, idx := range indicesToEmbed {
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

	db.mu.Lock()
	defer db.mu.Unlock()

	// Default options
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = 5 * time.Second
	}

	const batchSize = 25
	totalChunks := len(chunks)
	totalBatches := (totalChunks + batchSize - 1) / batchSize
	startBatch := opts.StartBatch

	slog.Info("Starting batched insert with checkpoint support",
		"totalChunks", totalChunks,
		"batchSize", batchSize,
		"startBatch", startBatch,
		"totalBatches", totalBatches)

	// Check for dimension mismatch before processing (only if starting fresh)
	if startBatch == 0 && len(chunks) > 0 && len(chunks[0].Embedding) == 0 {
		sampleText := fmt.Sprintf("%s: %s", chunks[0].Title, chunks[0].Content)
		sampleEmbeddings, err := db.embedSvc.Embed(ctx, []string{sampleText})
		if err != nil {
			return fmt.Errorf("failed to generate sample embedding: %w", err)
		}
		chunks[0].Embedding = sampleEmbeddings[0]

		existingDim := db.getTableEmbeddingDimension(ctx)
		newDim := len(chunks[0].Embedding)

		if existingDim > 0 && existingDim != newDim {
			slog.Warn("Embedding dimension mismatch detected, recreating table",
				"table", lanceTableName,
				"existingDimension", existingDim,
				"newDimension", newDim)

			if err := db.dropAndRecreateTable(ctx); err != nil {
				return fmt.Errorf("failed to recreate table for dimension change: %w", err)
			}
		}
	}

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

		// Process batch with retry logic
		err := db.processBatchWithRetry(ctx, batch, batchNum+1, opts.MaxRetries, opts.RetryDelay)
		if err != nil {
			return fmt.Errorf("failed at batch %d: %w", batchNum+1, err)
		}

		// Call checkpoint callback after successful batch
		if opts.CheckpointFunc != nil {
			if err := opts.CheckpointFunc(batchNum+1, batchEnd, totalBatches, totalChunks); err != nil {
				slog.Warn("Checkpoint callback failed", "batch", batchNum+1, "error", err)
				// Non-fatal: continue processing
			}
		}
	}

	// Create IVF-PQ vector index now that we have data
	if err := db.ensureVectorIndex(ctx); err != nil {
		slog.Warn("Failed to create vector index after insert", "error", err)
	}

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
		if attempt < maxRetries-1 {
			slog.Warn("Batch failed, retrying",
				"batch", batchNum,
				"attempt", attempt+1,
				"maxRetries", maxRetries,
				"delay", delay,
				"error", err)
			time.Sleep(delay)
			delay = delay * 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}

	return fmt.Errorf("batch %d failed after %d retries: %w", batchNum, maxRetries, lastErr)
}

// processSingleBatch processes a single batch of chunks.
func (db *LanceVectorDB) processSingleBatch(ctx context.Context, batch []DocumentChunk, batchNum int) error {
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
		embeddings, err := db.embedSvc.Embed(ctx, textsToEmbed)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings for batch %d: %w", batchNum, err)
		}

		for i, idx := range indicesToEmbed {
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

	return nil
}

// chunksToArrowRecord converts DocumentChunks to an Arrow Record.
func (db *LanceVectorDB) chunksToArrowRecord(chunks []DocumentChunk) (arrow.Record, error) {
	pool := memory.NewGoAllocator()

	dim := embeddingDimension
	if db.embedSvc != nil {
		dim = db.embedSvc.Dimension()
	}

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

	// Process FTS results
	for i, row := range ftsResults {
		chunk := db.rowToDocumentChunk(row)

		// FTS score from LanceDB (if available)
		var ftsScore float64 = 1.0
		if score, ok := row["_score"].(float64); ok {
			// Normalize FTS score to 0-1 range
			ftsScore = score / (score + 1)
		} else if score, ok := row["_score"].(float32); ok {
			ftsScore = float64(score) / (float64(score) + 1)
		} else {
			// Use rank-based score if no _score available
			ftsScore = 1.0 / float64(i+1)
		}

		if _, exists := docMap[chunk.ID]; !exists {
			docMap[chunk.ID] = &scoredDoc{chunk: chunk}
		}
		docMap[chunk.ID].ftsScore = ftsScore
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
