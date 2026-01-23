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

// ptr is a helper to create pointers to values.
func ptr[T any](v T) *T {
	return &v
}
