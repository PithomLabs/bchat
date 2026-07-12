//go:build !rag

package agent

import (
	"context"
	"fmt"
)

// newLanceVectorDB is a stub when building without the rag tag.
// LanceDB requires CGO and native libraries that are only available in RAG builds.
func newLanceVectorDB(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
	return nil, fmt.Errorf("LanceDB storage (local/s3) requires RAG build. Use 'task build:backend:rag' or set LANCEDB_STORAGE_PROVIDER=memory")
}

// newPool is a stub when building without the rag tag.
// Per-tenant connection pools require LanceDB (rag build).
func newPool(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
	return nil, fmt.Errorf("per-tenant connection pool requires RAG build. Use 'task build:backend:rag' or set LANCEDB_STORAGE_PROVIDER=memory")
}

// TenantVectorDBPool is a stub for non-rag builds so that type assertions
// in handlers.go and service.go compile (rag build uses the real impl in vectordb_pool.go).
type TenantVectorDBPool struct {
	store interface{}
}

func (p *TenantVectorDBPool) Insert(_ context.Context, _ []DocumentChunk) error {
	return fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) InsertWithCheckpoint(_ context.Context, _ []DocumentChunk, _ InsertOptions) error {
	return fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) Delete(_ context.Context, _ int32, _ string) error {
	return fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) DeleteByIDPrefix(_ context.Context, _ int32, _ string) (int, error) {
	return 0, fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) DeleteByVersion(_ context.Context, _ int32, _ string, _ string, _ int32) error {
	return fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) PurgePreVersionedChunks(_ context.Context, _ int32, _ string, _ string) error {
	return fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) ListIndexedVersions(_ context.Context, _ int32, _ string, _ string) ([]int32, error) {
	return nil, fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) Search(_ context.Context, _ SearchQuery) (*SearchResult, error) {
	return nil, fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) Close() error                  { return nil }
func (p *TenantVectorDBPool) Stats(_ context.Context) (*VectorDBStats, error) {
	return nil, fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) ListChunks(_ context.Context, _ int32) ([]DocumentChunk, error) {
	return nil, fmt.Errorf("RAG pool unavailable (non-rag build)")
}
func (p *TenantVectorDBPool) Dimension() int { return 0 }
func (p *TenantVectorDBPool) Validate(_ context.Context) error {
	return fmt.Errorf("RAG pool unavailable (non-rag build)")
}

// SetStore is a no-op for non-rag builds.
func (p *TenantVectorDBPool) SetStore(_ interface{}) {}

// Evict is a no-op for non-rag builds.
func (p *TenantVectorDBPool) Evict(_ int32) {}
