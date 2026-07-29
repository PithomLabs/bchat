//go:build !cockroach

package agent

import (
	"context"
	"database/sql"
	"fmt"
)

// NewCockroachVectorDB is a stub when building without the cockroach tag.
func NewCockroachVectorDB(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
	return nil, fmt.Errorf("CockroachDB vector storage requires cockroach build. Use 'task build:backend:cockroach'")
}

// CockroachVectorDB is a stub for non-cockroach builds.
type CockroachVectorDB struct {
	db       *sql.DB
	embedSvc EmbeddingService
	config   *VectorDBConfig
}

func (v *CockroachVectorDB) SetDB(_ *sql.DB)                                      {}
func (v *CockroachVectorDB) Validate(_ context.Context) error                      { return nil }
func (v *CockroachVectorDB) Dimension() int                                        { return 0 }
func (v *CockroachVectorDB) Close() error                                          { return nil }
func (v *CockroachVectorDB) Stats(_ context.Context) (*VectorDBStats, error)       { return nil, nil }
func (v *CockroachVectorDB) ListChunks(_ context.Context, _ int32) ([]DocumentChunk, error) {
	return nil, nil
}
func (v *CockroachVectorDB) Insert(_ context.Context, _ []DocumentChunk) error {
	return fmt.Errorf("CockroachDB unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) InsertWithCheckpoint(_ context.Context, _ []DocumentChunk, _ InsertOptions) error {
	return fmt.Errorf("CockroachDB unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) Delete(_ context.Context, _ int32, _ string) error {
	return fmt.Errorf("CockroachDB unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) DeleteByVersion(_ context.Context, _ int32, _ string, _ string, _ int32) error {
	return fmt.Errorf("CockroachDB unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) PurgePreVersionedChunks(_ context.Context, _ int32, _ string, _ string) error {
	return fmt.Errorf("CockroachDB unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) DeleteByIDPrefix(_ context.Context, _ int32, _ string) (int, error) {
	return 0, fmt.Errorf("CockroachDB unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) ListIndexedVersions(_ context.Context, _ int32, _ string, _ string) ([]int32, error) {
	return nil, fmt.Errorf("CockroachDB unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) Search(_ context.Context, _ SearchQuery) (*SearchResult, error) {
	return nil, fmt.Errorf("CockroachDB unavailable (non-cockroach build)")
}
