//go:build cockroach

package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
	"github.com/jackc/pgx/v5/pgconn"
)

// formatVectorString converts a float32 slice to CockroachDB vector string format "[1.0, 2.0, ...]"
func formatVectorString(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, v := range vec {
		if i > 0 {
			sb.WriteString(", ")
		}
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			sb.WriteString("0")
		} else {
			fmt.Fprintf(&sb, "%g", v)
		}
	}
	sb.WriteString("]")
	return sb.String()
}

// CockroachVectorDB implements VectorDB using CockroachDB's native vector support.
type CockroachVectorDB struct {
	db       *sql.DB
	embedSvc EmbeddingService
	config   *VectorDBConfig
}

// NewCockroachVectorDB creates a new CockroachDB-backed vector database.
func NewCockroachVectorDB(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
	if config.CockroachDSN == "" {
		return nil, fmt.Errorf("COCKROACH_DSN is required for CockroachDB vector storage")
	}

	db, err := newCockroachDB(config.CockroachDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CockroachDB: %w", err)
	}

	return &CockroachVectorDB{
		db:       db,
		embedSvc: embedSvc,
		config:   config,
	}, nil
}

// SetDB sets the database connection (post-construction wiring for shared pool).
func (v *CockroachVectorDB) SetDB(db *sql.DB) {
	v.db = db
}

// newCockroachDB opens a connection to CockroachDB using pgx stdlib.
func newCockroachDB(dsn string) (*sql.DB, error) {
	// CRDB requires simple_protocol to avoid prepared statement issues
	if !strings.Contains(dsn, "default_query_exec_mode") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "default_query_exec_mode=simple_protocol"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open CockroachDB: %w", err)
	}

	// CRDB Serverless compatibility: limit connections
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping CockroachDB: %w", err)
	}

	return db, nil
}

// Validate creates the agent_vectors table and vector index if they don't exist.
func (v *CockroachVectorDB) Validate(ctx context.Context) error {
	// 1. Create table with native VECTOR type (no extension needed)
	_, err := v.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS agent_vectors (
			id STRING PRIMARY KEY,
			tenant_id INT NOT NULL,
			content_type STRING NOT NULL,
			title STRING,
			content TEXT NOT NULL,
			embedding VECTOR(1536),
			metadata JSONB,
			source_version INT DEFAULT 1,
			created_at TIMESTAMPTZ DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create agent_vectors table: %w", err)
	}

	// 2. B-tree index for tenant filter
	_, err = v.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_agent_vectors_tenant ON agent_vectors (tenant_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create tenant index: %w", err)
	}

	// 3. Vector index (CRDB-specific syntax — NOT pgvector USING hnsw)
	// IF NOT EXISTS is supported for VECTOR INDEX in CRDB v26.1+ (docs confirmed).
	// vector_ip_ops is NOT supported (CRDB issue #144016) — default to vector_l2_ops
	// SQLSTATE fallback kept as defense-in-depth until concurrent startup is verified.
	// TODO(post-hackathon): remove SQLSTATE fallback after concurrent startup exercised.
	_, err = v.db.ExecContext(ctx, `
		CREATE VECTOR INDEX IF NOT EXISTS idx_agent_vectors_embedding
		ON agent_vectors (embedding)
	`)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "42P07":
				slog.Info("Vector index already exists", "index", "idx_agent_vectors_embedding")
			case "0A000":
				slog.Warn("Vector index feature not supported, using brute-force search",
					"error", err,
					"hint", "Ensure feature.vector_index.enabled = true or upgrade CRDB")
			default:
				slog.Warn("Vector index creation failed",
					"error", err,
					"hint", "May need feature.vector_index.enabled or CRDB v25.2+")
			}
		} else {
			slog.Warn("Vector index creation failed (non-PG error)",
				"error", err)
		}
	}

	return nil
}

// Dimension returns the embedding dimension (1536 for text-embedding-3-small).
func (v *CockroachVectorDB) Dimension() int { return 1536 }

// Close releases the database connection.
func (v *CockroachVectorDB) Close() error { return v.db.Close() }

// Stats returns database statistics.
func (v *CockroachVectorDB) Stats(ctx context.Context) (*VectorDBStats, error) {
	var totalChunks int64
	err := v.db.QueryRowContext(ctx, `SELECT count(*) FROM agent_vectors`).Scan(&totalChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	return &VectorDBStats{TotalChunks: totalChunks}, nil
}

// ListChunks returns all chunks for a given tenant.
func (v *CockroachVectorDB) ListChunks(ctx context.Context, tenantID int32) ([]DocumentChunk, error) {
	rows, err := v.db.QueryContext(ctx, `
		SELECT id, tenant_id, content_type, title, content, source_version, created_at
		FROM agent_vectors
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}
	defer rows.Close()

	var chunks []DocumentChunk
	for rows.Next() {
		var chunk DocumentChunk
		var createdAt time.Time
		if err := rows.Scan(&chunk.ID, &chunk.TenantID, &chunk.ContentType, &chunk.Title, &chunk.Content, &chunk.SourceVersion, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}
		chunk.IndexedAt = createdAt
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

// Insert adds or updates chunks (single-row UPSERT via crdb.ExecuteTx).
func (v *CockroachVectorDB) Insert(ctx context.Context, chunks []DocumentChunk) error {
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
		embeddings, err := v.embedSvc.Embed(ctx, textsToEmbed)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings: %w", err)
		}

		for i, idx := range indicesToEmbed {
			chunks[idx].Embedding = embeddings[i]
		}
	}

	for _, chunk := range chunks {
		// Handle nil/empty embeddings gracefully — pass Go nil → SQL NULL
		var embeddingValue interface{} = formatVectorString(chunk.Embedding)
		if len(chunk.Embedding) == 0 {
			embeddingValue = nil
		}

		err := crdb.ExecuteTx(ctx, v.db, nil, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				UPSERT INTO agent_vectors (id, tenant_id, content_type, title, content, embedding, metadata, source_version, created_at)
				VALUES ($1, $2, $3, $4, $5, $6::VECTOR, $7, $8, $9)
			`, chunk.ID, chunk.TenantID, chunk.ContentType, chunk.Title, chunk.Content,
				embeddingValue, "{}", chunk.SourceVersion, time.Now())
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to insert vector: %w", err)
		}
	}
	return nil
}

// InsertWithCheckpoint adds chunks with progress tracking.
func (v *CockroachVectorDB) InsertWithCheckpoint(ctx context.Context, chunks []DocumentChunk, opts InsertOptions) error {
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
		embeddings, err := v.embedSvc.Embed(ctx, textsToEmbed)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings: %w", err)
		}

		for i, idx := range indicesToEmbed {
			chunks[idx].Embedding = embeddings[i]
		}
	}

	for i, chunk := range chunks {
		// Handle nil/empty embeddings gracefully — pass Go nil → SQL NULL
		var embeddingValue interface{} = formatVectorString(chunk.Embedding)
		if len(chunk.Embedding) == 0 {
			embeddingValue = nil
		}

		err := crdb.ExecuteTx(ctx, v.db, nil, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				UPSERT INTO agent_vectors (id, tenant_id, content_type, title, content, embedding, metadata, source_version, created_at)
				VALUES ($1, $2, $3, $4, $5, $6::VECTOR, $7, $8, $9)
			`, chunk.ID, chunk.TenantID, chunk.ContentType, chunk.Title, chunk.Content,
				embeddingValue, "{}", chunk.SourceVersion, time.Now())
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to insert vector: %w", err)
		}

		if opts.CheckpointFunc != nil {
			if err := opts.CheckpointFunc(i+1, i+1, len(chunks), len(chunks), 1); err != nil {
				return fmt.Errorf("checkpoint callback failed: %w", err)
			}
		}
	}
	return nil
}

// Delete removes chunks matching filter criteria.
func (v *CockroachVectorDB) Delete(ctx context.Context, tenantID int32, audienceType string) error {
	_, err := v.db.ExecContext(ctx, `DELETE FROM agent_vectors WHERE tenant_id = $1 AND content_type = $2`, tenantID, audienceType)
	return err
}

// DeleteByVersion removes chunks for a specific version.
func (v *CockroachVectorDB) DeleteByVersion(ctx context.Context, tenantID int32, audienceType, fileType string, version int32) error {
	_, err := v.db.ExecContext(ctx, `DELETE FROM agent_vectors WHERE tenant_id = $1 AND content_type = $2 AND source_version = $3`, tenantID, fileType, version)
	return err
}

// PurgePreVersionedChunks removes chunks that predate versioning.
func (v *CockroachVectorDB) PurgePreVersionedChunks(ctx context.Context, tenantID int32, audienceType, fileType string) error {
	_, err := v.db.ExecContext(ctx, `DELETE FROM agent_vectors WHERE tenant_id = $1 AND content_type = $2 AND (source_version IS NULL OR source_version <= 1)`, tenantID, fileType)
	return err
}

// DeleteByIDPrefix removes chunks whose IDs start with prefix.
func (v *CockroachVectorDB) DeleteByIDPrefix(ctx context.Context, tenantID int32, idPrefix string) (int, error) {
	result, err := v.db.ExecContext(ctx, `DELETE FROM agent_vectors WHERE tenant_id = $1 AND id LIKE $2 || '%'`, tenantID, idPrefix)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

// ListIndexedVersions returns distinct source_version values.
func (v *CockroachVectorDB) ListIndexedVersions(ctx context.Context, tenantID int32, audienceType, fileType string) ([]int32, error) {
	rows, err := v.db.QueryContext(ctx, `SELECT DISTINCT source_version FROM agent_vectors WHERE tenant_id = $1 AND content_type = $2 ORDER BY source_version`, tenantID, fileType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []int32
	for rows.Next() {
		var ver int32
		if err := rows.Scan(&ver); err != nil {
			return nil, err
		}
		versions = append(versions, ver)
	}
	return versions, rows.Err()
}

// Search performs vector similarity search.
func (v *CockroachVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	start := time.Now()

	// 1. Get or generate query embedding (match LanceDB priority pattern)
	var queryEmbedding []float32
	if len(query.QueryEmbedding) > 0 {
		queryEmbedding = query.QueryEmbedding
	} else if query.QueryText != "" {
		embeddings, err := v.embedSvc.Embed(ctx, []string{query.QueryText})
		if err != nil {
			return nil, fmt.Errorf("failed to embed query: %w", err)
		}
		queryEmbedding = embeddings[0]
	} else {
		return &SearchResult{
			Chunks:  []DocumentChunk{},
			Scores:  []float64{},
			Total:   0,
			Latency: time.Since(start),
		}, nil
	}

	// 2. Format embedding as CockroachDB vector literal string: [0.1,0.2,...]
	// CockroachDB v25.2 does NOT support FormatBinary for VECTOR type (OID 90006).
	// See GitHub issues #147844, #170485. Fix backported to 25.3 (PR #148843) but NOT 25.2.
	// We pass the formatted string as a TEXT parameter to $1::VECTOR, which uses text format
	// and bypasses the binary format bug. If upgrading to a CockroachDB version with the fix,
	// native []float32 parameter binding can be used instead.
	vecStr := formatVectorLiteral(queryEmbedding)

	// 3. Build query with filters
	contentTypeFilter := "''"
	if len(query.ContentTypes) > 0 {
		quoted := make([]string, len(query.ContentTypes))
		for i, ct := range query.ContentTypes {
			quoted[i] = fmt.Sprintf("'%s'", ct)
		}
		contentTypeFilter = strings.Join(quoted, ",")
	}

	sqlQuery := fmt.Sprintf(`
		SELECT id, title, content, content_type, metadata, source_version, created_at,
		       1 - (embedding <=> $1::VECTOR) AS similarity
		FROM agent_vectors
		WHERE tenant_id = $2 AND content_type IN (%s)
		  AND (embedding <=> $1::VECTOR) <= 1 - $4
		ORDER BY embedding <=> $1::VECTOR
		LIMIT $3
	`, contentTypeFilter)

	// 4. Execute and scan into SearchResult
	rows, err := v.db.QueryContext(ctx, sqlQuery, vecStr, query.TenantID, query.TopK, query.MinScore)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer rows.Close()

	var result SearchResult
	for rows.Next() {
		var chunk DocumentChunk
		var metadata string
		var score float64
		var createdAt time.Time
		if err := rows.Scan(&chunk.ID, &chunk.Title, &chunk.Content, &chunk.ContentType, &metadata, &chunk.SourceVersion, &createdAt, &score); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		chunk.IndexedAt = createdAt
		result.Chunks = append(result.Chunks, chunk)
		result.Scores = append(result.Scores, score)
	}
	result.Total = len(result.Chunks)
	result.Latency = time.Since(start)
	result.SearchMode = "vector"

	return &result, rows.Err()
}

// formatVectorLiteral formats a []float32 as a CockroachDB vector literal string: [0.1,0.2,...]
// NOTE:
// CockroachDB VECTOR parameters could not be bound correctly through pgx (see Bug 057).
// Root cause: CockroachDB v25.2 does not support FormatBinary for VECTOR type (OID 90006).
// Fix exists in master (PR #148719) and backported to 25.3 (PR #148843), but NOT 25.2.
// formatVectorLiteral() intentionally emits only numeric vector literals for safe text-format interpolation.
// If CockroachDB v25.2+ gains native VECTOR parameter binding (binary format support), revisit this implementation.
func formatVectorLiteral(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
