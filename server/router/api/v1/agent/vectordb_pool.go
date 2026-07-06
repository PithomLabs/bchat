//go:build rag

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/usememos/memos/store"
)

// TenantVectorDBPool manages per-tenant LanceDB connections for S3/local tenant scoping.
// Each tenant gets its own LanceDB connection pointing to its own S3 prefix or local directory.
// This pool satisfies the VectorDB interface by routing calls to the correct tenant's connection.
type TenantVectorDBPool struct {
	mu       sync.RWMutex
	tenants  map[int32]VectorDB
	global   *VectorDBConfig
	embedSvc EmbeddingService
	store    *store.Store
}

// newPool creates a per-tenant connection pool (rag build).
func newPool(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
	return NewTenantVectorDBPool(config, embedSvc, nil)
}

// NewTenantVectorDBPool creates a new per-tenant vector database pool.
// Pass store for TenantConfig override resolution; nil store disables overrides.
func NewTenantVectorDBPool(config *VectorDBConfig, embedSvc EmbeddingService, s *store.Store) (VectorDB, error) {
	return &TenantVectorDBPool{
		tenants:  make(map[int32]VectorDB),
		global:   config,
		embedSvc: embedSvc,
		store:    s,
	}, nil
}

// SetStore updates the store reference for TenantConfig override resolution.
// Must be called before any tenant connections are created.
func (p *TenantVectorDBPool) SetStore(s *store.Store) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.store = s
}

// getOrCreate returns the cached VectorDB for a tenant, or creates one lazily.
func (p *TenantVectorDBPool) getOrCreate(ctx context.Context, tenantID int32) (VectorDB, error) {
	p.mu.RLock()
	if db, ok := p.tenants[tenantID]; ok {
		p.mu.RUnlock()
		return db, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if db, ok := p.tenants[tenantID]; ok {
		return db, nil
	}

	// Read per-tenant override from store
	var override *TenantS3Override
	if tc, err := p.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID}); err == nil && tc != nil && tc.VectorDBS3Override != "" {
		var o TenantS3Override
		if json.Unmarshal([]byte(tc.VectorDBS3Override), &o) == nil {
			override = &o
		}
	}

	// Build per-tenant config
	var db VectorDB
	var err error

	switch p.global.StorageProvider {
	case "s3":
		uri, resolvedCfg := resolveStorageTarget(p.global, override, tenantID)
		resolvedCfg.URI = uri
		slog.Info("Creating per-tenant S3 LanceDB connection",
			"tenantID", tenantID,
			"uri", uri,
			"endpoint", resolvedCfg.S3Endpoint)
		db, err = newLanceVectorDB(resolvedCfg, p.embedSvc)
	case "local":
		tenantPath := resolveLocalTarget(p.global, tenantID)
		tenantCfg := *p.global
		tenantCfg.LocalPath = tenantPath
		slog.Info("Creating per-tenant local LanceDB connection",
			"tenantID", tenantID,
			"path", tenantPath)
		db, err = newLanceVectorDB(&tenantCfg, p.embedSvc)
	default:
		return nil, fmt.Errorf("unsupported storage provider for tenant pool: %s", p.global.StorageProvider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create VectorDB for tenant %d: %w", tenantID, err)
	}

	p.tenants[tenantID] = db
	return db, nil
}

// Evict closes and removes a tenant's cached connection.
// Called when a tenant's S3 override changes.
func (p *TenantVectorDBPool) Evict(tenantID int32) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if db, ok := p.tenants[tenantID]; ok {
		db.Close()
		delete(p.tenants, tenantID)
		slog.Info("Evicted tenant VectorDB from pool", "tenantID", tenantID)
	}
}

// tenantIDs returns the list of tenant IDs currently cached in the pool.
func (p *TenantVectorDBPool) tenantIDs() []int32 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]int32, 0, len(p.tenants))
	for id := range p.tenants {
		ids = append(ids, id)
	}
	return ids
}

// ============================================================================
// VectorDB interface implementation
// ============================================================================

func (p *TenantVectorDBPool) Insert(ctx context.Context, chunks []DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Validate all chunks belong to the same tenant (review fix #2)
	tenantID := chunks[0].TenantID
	for _, c := range chunks[1:] {
		if c.TenantID != tenantID {
			return fmt.Errorf("mixed-tenant batch: first=%d, found=%d", tenantID, c.TenantID)
		}
	}

	db, err := p.getOrCreate(ctx, tenantID)
	if err != nil {
		return err
	}
	return db.Insert(ctx, chunks)
}

func (p *TenantVectorDBPool) InsertWithCheckpoint(ctx context.Context, chunks []DocumentChunk, opts InsertOptions) error {
	if len(chunks) == 0 {
		return nil
	}

	// Validate all chunks belong to the same tenant
	tenantID := chunks[0].TenantID
	for _, c := range chunks[1:] {
		if c.TenantID != tenantID {
			return fmt.Errorf("mixed-tenant batch: first=%d, found=%d", tenantID, c.TenantID)
		}
	}

	db, err := p.getOrCreate(ctx, tenantID)
	if err != nil {
		return err
	}
	return db.InsertWithCheckpoint(ctx, chunks, opts)
}

func (p *TenantVectorDBPool) Delete(ctx context.Context, tenantID int32, audienceType string) error {
	db, err := p.getOrCreate(ctx, tenantID)
	if err != nil {
		return err
	}
	return db.Delete(ctx, tenantID, audienceType)
}

func (p *TenantVectorDBPool) DeleteByIDPrefix(ctx context.Context, tenantID int32, idPrefix string) (int, error) {
	db, err := p.getOrCreate(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return db.DeleteByIDPrefix(ctx, tenantID, idPrefix)
}

func (p *TenantVectorDBPool) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	db, err := p.getOrCreate(ctx, query.TenantID)
	if err != nil {
		return nil, err
	}
	return db.Search(ctx, query)
}

func (p *TenantVectorDBPool) ListChunks(ctx context.Context, tenantID int32) ([]DocumentChunk, error) {
	db, err := p.getOrCreate(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return db.ListChunks(ctx, tenantID)
}

// Stats aggregates statistics across all cached tenant connections.
// Note: Only reflects tenants that have been accessed (lazy-loaded).
func (p *TenantVectorDBPool) Stats(ctx context.Context) (*VectorDBStats, error) {
	combined := &VectorDBStats{
		TenantCounts:  make(map[int32]int64),
		ContentCounts: make(map[string]int64),
	}

	for _, tenantID := range p.tenantIDs() {
		db, err := p.getOrCreate(ctx, tenantID)
		if err != nil {
			slog.Warn("Failed to get tenant for stats", "tenantID", tenantID, "error", err)
			continue
		}
		stats, err := db.Stats(ctx)
		if err != nil {
			slog.Warn("Failed to get tenant stats", "tenantID", tenantID, "error", err)
			continue
		}
		combined.TotalChunks += stats.TotalChunks
		combined.TenantCounts[tenantID] = stats.TotalChunks
		for ct, count := range stats.ContentCounts {
			combined.ContentCounts[ct] += count
		}
	}

	return combined, nil
}

func (p *TenantVectorDBPool) Dimension() int {
	return p.embedSvc.Dimension()
}

func (p *TenantVectorDBPool) Validate(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for tenantID, db := range p.tenants {
		if err := db.Validate(ctx); err != nil {
			slog.Warn("Tenant VectorDB validation failed", "tenantID", tenantID, "error", err)
			return fmt.Errorf("tenant %d validation failed: %w", tenantID, err)
		}
	}
	return nil
}

func (p *TenantVectorDBPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for tenantID, db := range p.tenants {
		if err := db.Close(); err != nil {
			slog.Warn("Failed to close tenant VectorDB", "tenantID", tenantID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	p.tenants = make(map[int32]VectorDB)
	return firstErr
}
