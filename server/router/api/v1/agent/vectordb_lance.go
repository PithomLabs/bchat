//go:build rag

package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	conn     contracts.IConnection
	table    contracts.ITable
	embedSvc EmbeddingService
	config   *VectorDBConfig
	mu       sync.RWMutex
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
func (db *LanceVectorDB) createIndexes(ctx context.Context) error {
	// Create vector index for similarity search
	if err := db.table.CreateIndexWithName(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq, "idx_embedding"); err != nil {
		slog.Warn("Failed to create vector index", "error", err)
	}

	// Create BTree index for tenant filtering
	if err := db.table.CreateIndexWithName(ctx, []string{"tenant_id"}, contracts.IndexTypeBTree, "idx_tenant"); err != nil {
		slog.Warn("Failed to create tenant index", "error", err)
	}

	return nil
}

// Insert adds or updates chunks in the database.
func (db *LanceVectorDB) Insert(ctx context.Context, chunks []DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	db.mu.Lock()
	defer db.mu.Unlock()

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

	// Build Arrow record
	record, err := db.chunksToArrowRecord(chunks)
	if err != nil {
		return fmt.Errorf("failed to convert chunks to Arrow record: %w", err)
	}
	defer record.Release()

	// Add to table
	if err := db.table.Add(ctx, record, nil); err != nil {
		return fmt.Errorf("failed to add records to LanceDB: %w", err)
	}

	slog.Debug("Inserted chunks into LanceDB", "count", len(chunks))
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
		idBuilder.Append(chunk.ID)
		tenantIDBuilder.Append(chunk.TenantID)
		audienceTypeBuilder.Append(chunk.AudienceType)
		contentTypeBuilder.Append(chunk.ContentType)
		titleBuilder.Append(chunk.Title)
		contentBuilder.Append(chunk.Content)
		codeBuilder.Append(chunk.Code)
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

// Search performs hybrid search.
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

	filter := strings.Join(filterParts, " AND ")

	// Determine topK
	topK := query.TopK
	if topK <= 0 {
		topK = 10
	}

	// Execute vector search with filter
	results, err := db.table.VectorSearchWithFilter(ctx, "embedding", queryEmbedding, topK, filter)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Parse results
	chunks := make([]DocumentChunk, 0, len(results))
	scores := make([]float64, 0, len(results))

	for _, row := range results {
		chunk := db.rowToDocumentChunk(row)

		// Calculate similarity score from distance
		// LanceDB returns L2 distance, convert to similarity
		var score float64 = 1.0
		if dist, ok := row["_distance"].(float64); ok {
			// Convert L2 distance to similarity score (0-1)
			score = 1.0 / (1.0 + dist)
		} else if dist, ok := row["_distance"].(float32); ok {
			score = 1.0 / (1.0 + float64(dist))
		}

		if score >= query.MinScore {
			chunks = append(chunks, chunk)
			scores = append(scores, score)
		}
	}

	return &SearchResult{
		Chunks:  chunks,
		Scores:  scores,
		Total:   len(chunks),
		Latency: time.Since(start),
	}, nil
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

// ptr is a helper to create pointers to values.
func ptr[T any](v T) *T {
	return &v
}
