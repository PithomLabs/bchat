package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/revrost/go-openrouter"

	"github.com/usememos/memos/internal/crypto"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
)

// Default timeout for LLM client requests
const defaultLLMTimeout = 180 * time.Second

// newOpenRouterClient creates an OpenRouter client with a timeout.
func newOpenRouterClient(apiKey string) *openrouter.Client {
	config := openrouter.DefaultConfig(apiKey)
	config.HTTPClient = &http.Client{
		Timeout: defaultLLMTimeout,
	}
	return openrouter.NewClientWithConfig(*config)
}

// Service handles all agent-related business logic.
type Service struct {
	store               *store.Store
	profile             *profile.Profile
	parser              *Parser
	memorySessions      *MemorySessionStore
	configCache         *ConfigCache
	encryptionService   *crypto.EncryptionService
	verifier            *Verifier
	verificationMetrics *VerificationMetrics
	vectorDB            VectorDB
	vectorDBConfig      *VectorDBConfig
	chunker             *Chunker
	vectorDBMu          sync.RWMutex // Protects vectorDB access
	observerBuffer      *ObserverBuffer
}

// NewService creates a new agent service.
func NewService(s *store.Store, p *profile.Profile) *Service {
	svc := &Service{
		store:          s,
		profile:        p,
		parser:         NewParser(),
		memorySessions: NewMemorySessionStore(30 * time.Minute),
		configCache:    NewConfigCache(5 * time.Minute),
	}

	// Initialize encryption service if master key is set
	if p.EncryptionMasterKey != "" {
		ctx := context.Background()
		secret, err := s.GetSystemSecret(ctx)
		if err != nil || secret == nil {
			// Generate new salt and store
			salt, _ := crypto.GenerateSalt()
			secret = &store.SystemSecret{
				EncryptionSalt: salt,
				KeyVersion:     1,
			}
			s.UpsertSystemSecret(ctx, secret)
		}
		svc.encryptionService = crypto.NewEncryptionService(p.EncryptionMasterKey, secret.EncryptionSalt)
		slog.Info("Encryption service initialized for tenant API keys")
	}

	// Initialize verification metrics
	svc.verificationMetrics = NewVerificationMetrics()
	slog.Info("Verification layer initialized")

	// Initialize chunker
	svc.chunker = NewChunker()

	// Initialize vector database for RAG pipeline
	vectorDBConfig := NewVectorDBConfigFromEnv()
	vectorDB, err := NewVectorDB(vectorDBConfig)
	if err != nil {
		slog.Error("Failed to initialize vector database", "error", err)
		// Fall back to no-op if initialization fails
		vectorDB = NewNoOpVectorDB()
	}
	svc.vectorDB = vectorDB
	svc.vectorDBConfig = vectorDBConfig

	// Log hybrid search configuration
	if vectorDBConfig.HybridSearchEnabled {
		slog.Info("Hybrid search enabled",
			"vector_weight", vectorDBConfig.HybridVectorWeight,
			"text_weight", vectorDBConfig.HybridTextWeight)
	}

	// Initialize observer buffer for async observation pre-computation
	omConfig := GetOMConfig()
	if omConfig.Enabled && omConfig.BufferTokens > 0 {
		svc.observerBuffer = NewObserverBuffer(svc, omConfig)
		slog.Info("Observer buffer initialized",
			"buffer_tokens_fraction", omConfig.BufferTokens,
			"activation_fraction", omConfig.BufferActivation,
			"block_after_fraction", omConfig.BlockAfter)
	}

	// Check if we should reindex all content on startup
	// Renamed from REINDEX_RAG to be more explicit and avoid confusion with enablement flags
	if os.Getenv("FORCE_REINDEX_ON_STARTUP") == "true" {
		go func() {
			// Small delay to ensure everything is initialized
			time.Sleep(2 * time.Second)
			if err := svc.ReindexAllContent(context.Background()); err != nil {
				slog.Error("Failed to reindex RAG content on startup", "error", err)
			}
		}()
	} else {
		// Auto-bootstrap: Check if RAG is enabled, and if the vector database has 0 chunks
		// but the SQLite source files database is not empty. If so, trigger a reindex.
		go func() {
			// Startup delay: Sleep for 5 seconds to allow other components (database connection pools,
			// embedding services, and network stacks) to fully initialize before we probe the vector DB.
			time.Sleep(5 * time.Second)
			ctx := context.Background()
			if svc.IsRAGEnabled() {
				stats, err := svc.GetVectorDB().Stats(ctx)
				if err == nil && stats.TotalChunks == 0 {
					// Audit note (tenant scoping): Calling ListAgentSourceFiles with LatestOnly: true
					// and TenantID == nil searches globally across ALL active tenants.
					files, err := s.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{LatestOnly: true})
					if err == nil && len(files) > 0 {
						slog.Info("RAG vector database table is empty but source files exist. Auto-triggering bootstrap reindexing in the background...", "sourceFilesCount", len(files))
						// Self-correcting design: If bootstrap reindexing fails (e.g. rate-limited), we log it
						// and exit gracefully. On the next container boot, because TotalChunks remains 0,
						// the bootstrap check will auto-retry.
						if err := svc.ReindexAllContent(ctx); err != nil {
							slog.Error("Failed to auto-bootstrap RAG content reindexing", "error", err)
						}
					}
				}
			}
		}()
	}

	return svc
}

// GetVectorDB returns the current VectorDB instance.
// Thread-safe accessor for the VectorDB.
func (s *Service) GetVectorDB() VectorDB {
	s.vectorDBMu.RLock()
	defer s.vectorDBMu.RUnlock()
	return s.vectorDB
}

// EncryptionService returns the encryption service.
// NOTE: This accessor is strictly for wiring the RequireBridgeHMAC middleware.
// It MUST NOT be used casually within request handlers.
func (s *Service) EncryptionService() *crypto.EncryptionService {
	return s.encryptionService
}

// IsRAGEnabled returns true if RAG pipeline is enabled (not using NoOpVectorDB).
func (s *Service) IsRAGEnabled() bool {
	s.vectorDBMu.RLock()
	defer s.vectorDBMu.RUnlock()
	if s.vectorDB == nil {
		return false
	}
	_, isNoOp := s.vectorDB.(*NoOpVectorDB)
	return !isNoOp
}

// GetEmbeddingDimension returns the embedding dimension for the current VectorDB.
// Returns 0 if VectorDB is not initialized or is a no-op implementation.
// Useful for debugging dimension mismatch issues.
func (s *Service) GetEmbeddingDimension() int {
	s.vectorDBMu.RLock()
	defer s.vectorDBMu.RUnlock()
	if s.vectorDB == nil {
		return 0
	}
	return s.vectorDB.Dimension()
}

// RefreshVectorDB recreates the VectorDB with current embedding configuration.
// Call this after changing embedding model env vars and restarting.
// This is typically only needed for development/debugging purposes.
func (s *Service) RefreshVectorDB() error {
	s.vectorDBMu.Lock()
	defer s.vectorDBMu.Unlock()

	// Close old VectorDB
	if s.vectorDB != nil {
		if err := s.vectorDB.Close(); err != nil {
			slog.Warn("Failed to close old VectorDB during refresh", "error", err)
		}
	}

	// Create new VectorDB with current config from environment
	vectorDBConfig := NewVectorDBConfigFromEnv()
	vectorDB, err := NewVectorDB(vectorDBConfig)
	if err != nil {
		return fmt.Errorf("failed to refresh VectorDB: %w", err)
	}

	s.vectorDB = vectorDB
	s.vectorDBConfig = vectorDBConfig

	slog.Info("VectorDB refreshed",
		"dimension", vectorDB.Dimension(),
		"provider", vectorDBConfig.StorageProvider,
		"enabled", vectorDBConfig.Enabled)
	return nil
}

// ReindexAllContent re-indexes all existing KB and Policy content from the database.
// This is useful when changing embedding providers or after a fresh deployment.
func (s *Service) ReindexAllContent(ctx context.Context) error {
	if s.vectorDB == nil || s.chunker == nil {
		return fmt.Errorf("RAG pipeline not initialized")
	}

	// Check if using NoOpVectorDB
	if _, isNoOp := s.vectorDB.(*NoOpVectorDB); isNoOp {
		return fmt.Errorf("RAG pipeline disabled (using NoOpVectorDB)")
	}

	slog.Info("Starting RAG reindex of all content...")

	// Get all tenants
	tenants, err := s.store.ListAgentTenants(ctx, &store.FindAgentTenant{})
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	totalChunks := 0
	for _, tenant := range tenants {
		// Get latest version of each source file for this tenant
		files, err := s.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
			TenantID:   &tenant.ID,
			LatestOnly: true, // Only get latest version of each file type
		})
		if err != nil {
			slog.Warn("Failed to list source files for tenant", "tenantID", tenant.ID, "error", err)
			continue
		}

		// Group files by audience type
		audienceFiles := make(map[string]map[string]string) // audience -> fileType -> content
		for _, f := range files {
			if _, ok := audienceFiles[f.AudienceType]; !ok {
				audienceFiles[f.AudienceType] = make(map[string]string)
			}
			audienceFiles[f.AudienceType][f.FileType] = f.Content
		}

		// Index each audience using heading-based chunker
		for audience, fileMap := range audienceFiles {
			kbContent := fileMap["kb"]
			policyContent := fileMap["policy"]

			if kbContent == "" && policyContent == "" {
				continue
			}

			// Delete existing chunks
			if err := s.vectorDB.Delete(ctx, tenant.ID, audience); err != nil {
				slog.Warn("Failed to delete existing chunks", "tenantID", tenant.ID, "audience", audience, "error", err)
			}

			// Use heading-based chunker for raw markdown content
			// Get chunk size based on embedding provider
			embeddingProvider := ""
			if s.vectorDBConfig != nil && s.vectorDBConfig.EmbeddingConfig != nil {
				embeddingProvider = s.vectorDBConfig.EmbeddingConfig.Provider
			}
			maxChunkTokens := GetMaxChunkTokens(embeddingProvider)

			var allChunks []DocumentChunk
			if kbContent != "" {
				kbChunks := s.chunker.ChunkMarkdownContent(kbContent, tenant.ID, audience, "kb", 1, maxChunkTokens)
				allChunks = append(allChunks, kbChunks...)
			}
			if policyContent != "" {
				policyChunks := s.chunker.ChunkMarkdownContent(policyContent, tenant.ID, audience, "policy", 1, maxChunkTokens)
				allChunks = append(allChunks, policyChunks...)
			}

			if len(allChunks) == 0 {
				continue
			}

			// Insert chunks
			if err := s.vectorDB.Insert(ctx, allChunks); err != nil {
				slog.Warn("Failed to insert chunks", "tenantID", tenant.ID, "audience", audience, "error", err)
				continue
			}

			totalChunks += len(allChunks)
			slog.Info("Reindexed content for tenant",
				"tenantID", tenant.ID,
				"tenant", tenant.Slug,
				"audience", audience,
				"chunks", len(allChunks),
				"method", "heading-based")
		}
	}

	slog.Info("RAG reindex completed", "totalChunks", totalChunks, "tenants", len(tenants))
	return nil
}

// ReindexTenantContent re-indexes KB and Policy content for a specific tenant.
// If audienceType is provided (non-empty), only that audience is indexed.
// Returns the number of chunks indexed.
func (s *Service) ReindexTenantContent(ctx context.Context, tenantID int32, audienceType string) (int, error) {
	if s.vectorDB == nil || s.chunker == nil {
		return 0, fmt.Errorf("RAG pipeline not initialized")
	}

	// Check if using NoOpVectorDB
	if _, isNoOp := s.vectorDB.(*NoOpVectorDB); isNoOp {
		return 0, fmt.Errorf("RAG pipeline disabled (using NoOpVectorDB)")
	}

	// If audienceType is "all", we treat it as empty to get all source files
	if audienceType == "all" {
		audienceType = ""
	}

	// Get tenant info for logging
	tenant, err := s.store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: &tenantID})
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Get chunk size based on embedding provider
	embeddingProvider := ""
	if s.vectorDBConfig != nil && s.vectorDBConfig.EmbeddingConfig != nil {
		embeddingProvider = s.vectorDBConfig.EmbeddingConfig.Provider
	}
	maxChunkTokens := GetMaxChunkTokens(embeddingProvider)

	slog.Info("Starting RAG reindex for tenant",
		"tenantID", tenantID,
		"tenant", tenant.Slug,
		"audienceFilter", audienceType,
		"embeddingProvider", embeddingProvider,
		"maxChunkTokens", maxChunkTokens)

	// Get latest version of each source file for this tenant
	findParams := &store.FindAgentSourceFile{
		TenantID:   &tenantID,
		LatestOnly: true, // Only get latest version of each file type
	}

	// Optional: filter by audience type
	if audienceType != "" {
		findParams.AudienceType = &audienceType
	}

	files, err := s.store.ListAgentSourceFiles(ctx, findParams)
	if err != nil {
		return 0, fmt.Errorf("failed to list source files: %w", err)
	}

	// DEBUG: Log found files
	slog.Info("DEBUG: Found source files for reindex",
		"tenantID", tenantID,
		"audienceFilter", audienceType,
		"fileCount", len(files))
	for _, f := range files {
		slog.Info("DEBUG: Source file details",
			"id", f.ID,
			"audience", f.AudienceType,
			"fileType", f.FileType,
			"contentLen", len(f.Content),
			"version", f.Version)
	}

	// Group files by audience type
	audienceFiles := make(map[string]map[string]string) // audience -> fileType -> content
	for _, f := range files {
		if _, ok := audienceFiles[f.AudienceType]; !ok {
			audienceFiles[f.AudienceType] = make(map[string]string)
		}
		audienceFiles[f.AudienceType][f.FileType] = f.Content
	}

	totalChunks := 0

	// Index each audience using heading-based chunker
	for audience, fileMap := range audienceFiles {
		kbContent := fileMap["kb"]
		policyContent := fileMap["policy"]

		if kbContent == "" && policyContent == "" {
			continue
		}

		// Delete existing chunks for this tenant/audience
		if err := s.vectorDB.Delete(ctx, tenantID, audience); err != nil {
			slog.Warn("Failed to delete existing chunks", "tenantID", tenantID, "audience", audience, "error", err)
		}

		// Use heading-based chunker for raw markdown content (maxChunkTokens set at function start)
		var allChunks []DocumentChunk
		if kbContent != "" {
			kbChunks := s.chunker.ChunkMarkdownContent(kbContent, tenantID, audience, "kb", 1, maxChunkTokens)
			allChunks = append(allChunks, kbChunks...)
		}
		if policyContent != "" {
			policyChunks := s.chunker.ChunkMarkdownContent(policyContent, tenantID, audience, "policy", 1, maxChunkTokens)
			allChunks = append(allChunks, policyChunks...)
		}

		if len(allChunks) == 0 {
			slog.Warn("No chunks created from content",
				"tenantID", tenantID,
				"audience", audience,
				"kbLength", len(kbContent),
				"policyLength", len(policyContent))
			continue
		}

		// DEBUG: Log chunks about to insert
		slog.Info("DEBUG: About to insert chunks",
			"tenantID", tenantID,
			"audience", audience,
			"chunkCount", len(allChunks))

		// Insert chunks
		if err := s.vectorDB.Insert(ctx, allChunks); err != nil {
			slog.Error("DEBUG: Insert failed", "error", err)
			return totalChunks, fmt.Errorf("failed to insert chunks for audience %s: %w", audience, err)
		}

		totalChunks += len(allChunks)
		slog.Info("Reindexed content for tenant",
			"tenantID", tenantID,
			"tenant", tenant.Slug,
			"audience", audience,
			"chunks", len(allChunks),
			"method", "heading-based")
	}

	slog.Info("RAG reindex completed for tenant", "tenantID", tenantID, "tenant", tenant.Slug, "totalChunks", totalChunks)
	return totalChunks, nil
}

// ReindexStatus represents the current state of a reindex operation.
type ReindexStatus struct {
	Status          string `json:"status"` // "idle", "in_progress", "completed", "failed", "stale_in_progress"
	CurrentBatch    int    `json:"current_batch,omitempty"`
	TotalBatches    int    `json:"total_batches,omitempty"`
	ProcessedChunks int    `json:"processed_chunks,omitempty"`
	TotalChunks     int    `json:"total_chunks,omitempty"`
	ErrorMessage    string `json:"error,omitempty"`
	LastMessage     string `json:"last_message,omitempty"`
	ErrorBatch      *int   `json:"error_batch,omitempty"`
	CanResume       bool   `json:"can_resume"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

// GetReindexStatus returns the current reindex status for a tenant.
// It satisfies the recovered invariant:
// INV_RAG_REINDEX_STATUS_MUST_REFLECT_EFFECTIVE_SCOPE.
func (s *Service) GetReindexStatus(ctx context.Context, tenantID int32, audience string) (*ReindexStatus, error) {
	var checkpoints []*store.ReindexCheckpoint

	if audience == "all" {
		internalAudience := "internal"
		internalCp, err := s.store.GetReindexCheckpoint(ctx, &store.FindReindexCheckpoint{
			TenantID: &tenantID,
			Audience: &internalAudience,
		})
		if err != nil {
			return nil, err
		}
		if internalCp != nil {
			checkpoints = append(checkpoints, internalCp)
		}

		externalAudience := "external"
		externalCp, err := s.store.GetReindexCheckpoint(ctx, &store.FindReindexCheckpoint{
			TenantID: &tenantID,
			Audience: &externalAudience,
		})
		if err != nil {
			return nil, err
		}
		if externalCp != nil {
			checkpoints = append(checkpoints, externalCp)
		}

		allAudience := "all"
		allCp, err := s.store.GetReindexCheckpoint(ctx, &store.FindReindexCheckpoint{
			TenantID: &tenantID,
			Audience: &allAudience,
		})
		if err != nil {
			return nil, err
		}
		if allCp != nil {
			checkpoints = append(checkpoints, allCp)
		}
	} else {
		checkpoint, err := s.store.GetReindexCheckpoint(ctx, &store.FindReindexCheckpoint{
			TenantID: &tenantID,
			Audience: &audience,
		})
		if err != nil {
			return nil, err
		}
		if checkpoint != nil {
			checkpoints = append(checkpoints, checkpoint)
		}
	}

	if len(checkpoints) == 0 {
		return &ReindexStatus{Status: "idle", CanResume: false}, nil
	}

	// Helper to resolve status and stale/resume state
	resolveState := func(cp *store.ReindexCheckpoint) (string, bool) {
		status := cp.Status
		canResume := cp.Status == "failed"
		if cp.Status == "in_progress" && !cp.UpdatedAt.IsZero() {
			// Stale threshold: 1 hour
			if time.Since(cp.UpdatedAt) > 1*time.Hour {
				status = "stale_in_progress"
				canResume = true
			}
		}
		return status, canResume
	}

	// Single checkpoint standard behavior
	if len(checkpoints) == 1 {
		cp := checkpoints[0]
		status, canResume := resolveState(cp)

		res := &ReindexStatus{
			Status:          status,
			CurrentBatch:    int(cp.CurrentBatch),
			TotalBatches:    int(cp.TotalBatches),
			ProcessedChunks: int(cp.ProcessedChunks),
			TotalChunks:     int(cp.TotalChunks),
			ErrorMessage:    cp.ErrorMessage,
			LastMessage:     cp.LastMessage,
			CanResume:       canResume,
		}
		if !cp.UpdatedAt.IsZero() {
			res.UpdatedAt = cp.UpdatedAt.Format(time.RFC3339)
		}
		if cp.ErrorBatch != nil {
			batch := int(*cp.ErrorBatch)
			res.ErrorBatch = &batch
		}
		return res, nil
	}

	// Aggregate multiple checkpoints
	combinedStatus := "completed"
	var totalChunks, processedChunks, currentBatch, totalBatches int
	var errorMsg, lastMsg string
	var errorBatch *int
	var canResume bool
	var latestUpdate time.Time

	hasInProgress := false
	hasStaleInProgress := false
	hasFailed := false

	for _, cp := range checkpoints {
		totalChunks += int(cp.TotalChunks)
		processedChunks += int(cp.ProcessedChunks)
		currentBatch += int(cp.CurrentBatch)
		totalBatches += int(cp.TotalBatches)

		status, resCanResume := resolveState(cp)
		if resCanResume {
			canResume = true
		}

		if status == "in_progress" {
			hasInProgress = true
		} else if status == "stale_in_progress" {
			hasStaleInProgress = true
		} else if status == "failed" {
			hasFailed = true
		}

		if cp.ErrorMessage != "" {
			if errorMsg != "" {
				errorMsg += "; "
			}
			errorMsg += fmt.Sprintf("[%s]: %s", cp.Audience, cp.ErrorMessage)
		}
		if cp.LastMessage != "" {
			if lastMsg != "" {
				lastMsg += "; "
			}
			lastMsg += fmt.Sprintf("[%s]: %s", cp.Audience, cp.LastMessage)
		}

		if cp.ErrorBatch != nil && errorBatch == nil {
			batch := int(*cp.ErrorBatch)
			errorBatch = &batch
		}

		if cp.UpdatedAt.After(latestUpdate) {
			latestUpdate = cp.UpdatedAt
		}
	}

	// Precedence order: in_progress > stale_in_progress > failed > completed
	if hasInProgress {
		combinedStatus = "in_progress"
	} else if hasStaleInProgress {
		combinedStatus = "stale_in_progress"
	} else if hasFailed {
		combinedStatus = "failed"
	}

	res := &ReindexStatus{
		Status:          combinedStatus,
		CurrentBatch:    currentBatch,
		TotalBatches:    totalBatches,
		ProcessedChunks: processedChunks,
		TotalChunks:     totalChunks,
		ErrorMessage:    errorMsg,
		LastMessage:     lastMsg,
		ErrorBatch:      errorBatch,
		CanResume:       canResume,
	}
	if !latestUpdate.IsZero() {
		res.UpdatedAt = latestUpdate.Format(time.RFC3339)
	}

	return res, nil
}

// ReindexTenantContentWithResume re-indexes with checkpoint support for resume-from-error.
func (s *Service) ReindexTenantContentWithResume(ctx context.Context, tenantID int32, audienceType string, resume bool) (int, error) {
	if s.vectorDB == nil || s.chunker == nil {
		return 0, fmt.Errorf("RAG pipeline not initialized")
	}

	// Check if using NoOpVectorDB
	if _, isNoOp := s.vectorDB.(*NoOpVectorDB); isNoOp {
		return 0, fmt.Errorf("RAG pipeline disabled (using NoOpVectorDB)")
	}

	// If audienceType is "all", we process all audiences
	if audienceType == "all" {
		// Keep audienceType as "all" for checkpointing purposes
	} else if audienceType == "" {
		audienceType = "internal"
	}

	// Check for existing checkpoint
	var existingCheckpoint *store.ReindexCheckpoint
	if resume {
		checkpoint, err := s.store.GetReindexCheckpoint(ctx, &store.FindReindexCheckpoint{
			TenantID: &tenantID,
			Audience: &audienceType,
		})
		if err != nil {
			slog.Warn("Failed to get checkpoint", "error", err)
		} else if checkpoint != nil && checkpoint.Status == "failed" {
			existingCheckpoint = checkpoint
			slog.Info("Resuming from checkpoint",
				"tenantID", tenantID,
				"audience", audienceType,
				"startBatch", checkpoint.CurrentBatch,
				"totalBatches", checkpoint.TotalBatches)
		}
	}

	// Get tenant info
	tenant, err := s.store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: &tenantID})
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Get chunk size based on embedding provider
	embeddingProvider := ""
	if s.vectorDBConfig != nil && s.vectorDBConfig.EmbeddingConfig != nil {
		embeddingProvider = s.vectorDBConfig.EmbeddingConfig.Provider
	}
	maxChunkTokens := GetMaxChunkTokens(embeddingProvider)

	slog.Info("Starting RAG reindex with checkpoint support",
		"tenantID", tenantID,
		"tenant", tenant.Slug,
		"audienceFilter", audienceType,
		"resume", resume,
		"hasCheckpoint", existingCheckpoint != nil)

	// Get latest version of source files
	findParams := &store.FindAgentSourceFile{
		TenantID:   &tenantID,
		LatestOnly: true,
	}
	if audienceType != "all" {
		findParams.AudienceType = &audienceType
	}

	files, err := s.store.ListAgentSourceFiles(ctx, findParams)
	if err != nil {
		return 0, fmt.Errorf("failed to list source files: %w", err)
	}

	// Group files by audience for correct chunking
	audienceFiles := make(map[string]map[string]string) // audience -> fileType -> content
	for _, f := range files {
		if _, ok := audienceFiles[f.AudienceType]; !ok {
			audienceFiles[f.AudienceType] = make(map[string]string)
		}
		audienceFiles[f.AudienceType][f.FileType] = f.Content
	}

	var allChunks []DocumentChunk
	for audience, fileMap := range audienceFiles {
		kbContent := fileMap["kb"]
		policyContent := fileMap["policy"]

		if kbContent != "" {
			kbChunks := s.chunker.ChunkMarkdownContent(kbContent, tenantID, audience, "kb", 1, maxChunkTokens)
			allChunks = append(allChunks, kbChunks...)
		}
		if policyContent != "" {
			policyChunks := s.chunker.ChunkMarkdownContent(policyContent, tenantID, audience, "policy", 1, maxChunkTokens)
			allChunks = append(allChunks, policyChunks...)
		}
	}

	if len(allChunks) == 0 {
		return 0, nil
	}

	totalChunks := len(allChunks)
	// Batch size configurable via EMBEDDING_BATCH_SIZE env var (default: 25, max: 200)
	batchSize := GetEmbeddingBatchSize()
	totalBatches := (totalChunks + batchSize - 1) / batchSize
	startBatch := 0

	// If not resuming, delete existing content and start fresh
	if existingCheckpoint == nil {
		if err := s.vectorDB.Delete(ctx, tenantID, audienceType); err != nil {
			slog.Warn("Failed to delete existing chunks", "error", err)
		}

		// Create new checkpoint
		checkpoint := &store.ReindexCheckpoint{
			TenantID:     tenantID,
			Audience:     audienceType,
			TotalChunks:  int32(totalChunks),
			TotalBatches: int32(totalBatches),
			BatchSize:    int32(batchSize),
			Status:       "in_progress",
			StartedAt:    time.Now(),
		}
		if _, err := s.store.UpsertReindexCheckpoint(ctx, checkpoint); err != nil {
			slog.Warn("Failed to create checkpoint", "error", err)
		}
	} else {
		// Resume from existing checkpoint
		startBatch = int(existingCheckpoint.CurrentBatch)

		// Update checkpoint status to in_progress
		existingCheckpoint.Status = "in_progress"
		existingCheckpoint.ErrorMessage = ""
		existingCheckpoint.ErrorBatch = nil
		if _, err := s.store.UpsertReindexCheckpoint(ctx, existingCheckpoint); err != nil {
			slog.Warn("Failed to update checkpoint", "error", err)
		}
	}

	// Create checkpoint callback
	checkpointFunc := func(currentBatch, processedChunks, totalBatches, totalChunks, chunksInBatch int) error {
		checkpoint := &store.ReindexCheckpoint{
			TenantID:        tenantID,
			Audience:        audienceType,
			TotalChunks:     int32(totalChunks),
			ProcessedChunks: int32(processedChunks),
			CurrentBatch:    int32(currentBatch),
			TotalBatches:    int32(totalBatches),
			BatchSize:       int32(batchSize),
			Status:          "in_progress",
			LastMessage: fmt.Sprintf("Processing batch batch=%d totalBatches=%d chunksInBatch=%d progress=%d/%d...",
				currentBatch, totalBatches, chunksInBatch, processedChunks, totalChunks),
		}
		_, err := s.store.UpsertReindexCheckpoint(ctx, checkpoint)
		return err
	}

	// Use InsertWithCheckpoint for progress tracking and retry logic
	opts := InsertOptions{
		StartBatch:     startBatch,
		CheckpointFunc: checkpointFunc,
		MaxRetries:     3,
		RetryDelay:     5 * time.Second,
	}

	if err := s.vectorDB.InsertWithCheckpoint(ctx, allChunks, opts); err != nil {
		// Mark checkpoint as failed
		errBatch := extractBatchFromError(err)
		failedCheckpoint := &store.ReindexCheckpoint{
			TenantID:     tenantID,
			Audience:     audienceType,
			TotalChunks:  int32(totalChunks),
			TotalBatches: int32(totalBatches),
			BatchSize:    int32(batchSize),
			Status:       "failed",
			ErrorMessage: err.Error(),
			ErrorBatch:   errBatch,
		}
		// [CODE-LOCAL INVARIANT BOUNDARY COMMENT]
		// INV_RAG_CHECKPOINT_STATE_MUST_PERSIST_ON_CANCEL:
		// When the main request context ctx is cancelled or timed out, we must detach
		// from it and use a short, bounded context to write the failure checkpoint to DB.
		checkpointCtx, checkpointCancel := context.WithTimeout(context.Background(), 5*time.Second)
		s.store.UpsertReindexCheckpoint(checkpointCtx, failedCheckpoint)
		checkpointCancel()

		return 0, err
	}

	// Mark checkpoint as completed
	completedCheckpoint := &store.ReindexCheckpoint{
		TenantID:        tenantID,
		Audience:        audienceType,
		TotalChunks:     int32(totalChunks),
		ProcessedChunks: int32(totalChunks),
		CurrentBatch:    int32(totalBatches),
		TotalBatches:    int32(totalBatches),
		BatchSize:       int32(batchSize),
		Status:          "completed",
	}
	// Detached but bounded context for durability
	completedCtx, completedCancel := context.WithTimeout(context.Background(), 5*time.Second)
	s.store.UpsertReindexCheckpoint(completedCtx, completedCheckpoint)
	completedCancel()

	slog.Info("RAG reindex completed with checkpoint",
		"tenantID", tenantID,
		"tenant", tenant.Slug,
		"totalChunks", totalChunks)

	return totalChunks, nil
}

// extractBatchFromError tries to extract batch number from error message.
func extractBatchFromError(err error) *int32 {
	// Error format: "failed at batch X: ..."
	errStr := err.Error()
	var batch int
	if _, scanErr := fmt.Sscanf(errStr, "failed at batch %d", &batch); scanErr == nil {
		b := int32(batch)
		return &b
	}
	return nil
}

// ============================================================================
// MEMORY SESSION STORE (for external anonymous sessions)
// ============================================================================

// MemorySessionStore manages in-memory sessions for external users.
type MemorySessionStore struct {
	sessions map[memorySessionKey]*store.AgentSession
	mu       sync.RWMutex
	ttl      time.Duration
}

type memorySessionKey struct {
	TenantID  int32
	SessionID string
}

// NewMemorySessionStore creates a new memory session store.
func NewMemorySessionStore(ttl time.Duration) *MemorySessionStore {
	store := &MemorySessionStore{
		sessions: make(map[memorySessionKey]*store.AgentSession),
		ttl:      ttl,
	}
	go store.cleanupLoop()
	return store
}

// GetOrCreate retrieves or creates a new session.
func (s *MemorySessionStore) GetOrCreate(tenantID int32, sessionID string) *store.AgentSession {
	if sessionID == "" {
		return nil
	}
	key := memorySessionKey{TenantID: tenantID, SessionID: sessionID}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if session, ok := s.sessions[key]; ok {
		if session.TenantID == tenantID && session.ID == sessionID {
			session.UpdatedAt = now
			return session
		}
		delete(s.sessions, key)
	}

	// Create new session
	session := &store.AgentSession{
		ID:             sessionID,
		TenantID:       tenantID,
		AudienceType:   "external",
		Phase:          "triage",
		UrgencyLevel:   0,
		CoverageStatus: "unknown",
		MessageCount:   0,
		Messages:       []store.AgentMessage{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.sessions[key] = session
	return session
}

// Get retrieves a session by ID.
func (s *MemorySessionStore) Get(tenantID int32, sessionID string) *store.AgentSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session := s.sessions[memorySessionKey{TenantID: tenantID, SessionID: sessionID}]
	if session == nil || session.TenantID != tenantID {
		return nil
	}
	return session
}

// Update updates a session in the store.
func (s *MemorySessionStore) Update(session *store.AgentSession) error {
	if session == nil || session.TenantID <= 0 {
		return fmt.Errorf("invalid memory session tenant")
	}
	if err := store.ValidateExternalSessionID(session.ID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memorySessionKey{TenantID: session.TenantID, SessionID: session.ID}
	for existingKey, existing := range s.sessions {
		if existing == session && existingKey != key {
			return fmt.Errorf("memory session tenant or id mutation rejected")
		}
	}
	session.UpdatedAt = time.Now()
	s.sessions[key] = session
	return nil
}

func (s *MemorySessionStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.cleanup()
	}
}

func (s *MemorySessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.ttl)
	for key, session := range s.sessions {
		if session.UpdatedAt.Before(cutoff) {
			delete(s.sessions, key)
		}
	}
}

// NormalizeExternalSessionID validates caller-provided external session IDs.
// Missing IDs receive a server-generated UUID; malformed non-empty IDs are rejected.
func NormalizeExternalSessionID(input string) (string, bool, error) {
	if input == "" {
		return uuid.NewString(), true, nil
	}
	if err := store.ValidateExternalSessionID(input); err != nil {
		return "", false, err
	}
	return input, false, nil
}

// ============================================================================
// CONFIG CACHE
// ============================================================================

// ConfigCache caches tenant configurations.
type ConfigCache struct {
	cache map[string]*CachedConfig
	mu    sync.RWMutex
	ttl   time.Duration
}

// CachedConfig holds a cached configuration with timestamp.
type CachedConfig struct {
	Config   *AudienceConfig
	LoadedAt time.Time
}

// AudienceConfig represents the complete configuration for an audience.
type AudienceConfig struct {
	TenantID     int32
	TenantSlug   string
	CompanyName  string
	AudienceType string

	// Identity
	Audience *store.AgentAudience

	// Knowledge Base
	Services   []*store.AgentService
	Exclusions []*store.AgentExclusion
	Coverage   []*store.AgentCoverage
	FAQs       []*store.AgentFAQ
	Safety     []*store.AgentSafetyProtocol
	Sections   []*store.AgentKBSection

	// Policy
	Intents []*store.AgentIntent
	Rules   []*store.AgentRule

	// Conversation Flow Script (SCRIPT.MD - tenant-level, same for all audiences)
	Script *store.AgentTenantScript

	// Learned Behaviors (from agent self-improvement)
	LearnedBehaviors []store.LearnedBehavior

	// Raw file content for verification (set by parser)
	RawKB     string
	RawPolicy string

	// Verification rules (parsed from POLICY.MD)
	VerificationRules []VerificationRule

	// HasStructuredContent indicates whether meaningful structured annotations were found
	// in KB.MD or POLICY.MD. When false, the tenant relies entirely on RAG retrieval
	// from unstructured content (e.g., uploaded novels, plain text documents).
	HasStructuredContent bool
}

// VerificationRule represents a custom verification rule from POLICY.MD.
type VerificationRule struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"` // "exact_match", "blocklist", "conditional"
	Description string   `json:"description"`
	Sources     []string `json:"sources"`  // KB sections to check against
	Fallback    string   `json:"fallback"` // Fallback text if rule cannot be satisfied
}

// NewConfigCache creates a new config cache.
func NewConfigCache(ttl time.Duration) *ConfigCache {
	return &ConfigCache{
		cache: make(map[string]*CachedConfig),
		ttl:   ttl,
	}
}

// Get retrieves a cached config.
func (c *ConfigCache) Get(tenantSlug, audienceType string) *AudienceConfig {
	key := tenantSlug + ":" + audienceType
	c.mu.RLock()
	defer c.mu.RUnlock()

	if cached, ok := c.cache[key]; ok {
		if time.Since(cached.LoadedAt) < c.ttl {
			return cached.Config
		}
	}
	return nil
}

// Set stores a config in the cache.
func (c *ConfigCache) Set(config *AudienceConfig) {
	key := config.TenantSlug + ":" + config.AudienceType
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = &CachedConfig{
		Config:   config,
		LoadedAt: time.Now(),
	}
}

// Invalidate removes a config from the cache.
func (c *ConfigCache) Invalidate(tenantSlug string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, tenantSlug+":external")
	delete(c.cache, tenantSlug+":internal")
}

// InvalidateConfigCache invalidates the config cache for a tenant.
func (s *Service) InvalidateConfigCache(tenantSlug string) {
	s.configCache.Invalidate(tenantSlug)
}

// ============================================================================
// LLM CONFIGURATION
// ============================================================================

// getLLMConfig returns the LLM model and API key for a tenant with fallback to env vars.
func (s *Service) getLLMConfig(ctx context.Context, tenantID int32) (model string, apiKey string) {
	// 1. Try tenant-specific config
	config, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID})
	if config != nil {
		if config.LLMModel != "" {
			model = config.LLMModel
		}
		if len(config.OpenRouterAPIKeyEncrypted) > 0 && s.encryptionService != nil {
			decrypted, err := s.encryptionService.Decrypt(
				config.OpenRouterAPIKeyEncrypted,
				config.OpenRouterAPIKeyNonce,
			)
			if err == nil && decrypted != "" {
				apiKey = decrypted
			}
		}
	}

	// 2. Fallback to environment variables
	if model == "" {
		model = s.profile.LLMModel
		if model == "" {
			model = "openai/gpt-oss-120b:free"
		}
	}
	if apiKey == "" {
		apiKey = s.profile.OpenRouterAPIKey
	}

	return model, apiKey
}

// getSimulationHumanModel returns the LLM model for the human role in simulations.
// Falls back to the main LLM model if not configured.
func (s *Service) getSimulationHumanModel(ctx context.Context, tenantID int32) string {
	// 1. Try tenant-specific simulation human model
	config, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID})
	if config != nil && config.SimulationHumanModel != "" {
		return config.SimulationHumanModel
	}

	// 2. Fallback to main LLM model
	model, _ := s.getLLMConfig(ctx, tenantID)
	return model
}

// verifyResponseWithLLM uses an LLM to verify the response against KB and policies.
// Returns the (potentially corrected) response and the verification result.
// Can be disabled via LLM_VERIFIER_ENABLED=false environment variable.
func (s *Service) verifyResponseWithLLM(ctx context.Context, response string, config *AudienceConfig) (string, *VerificationResult) {
	// Check if verifier is enabled via environment variable (default: false for RAG pipeline)
	if os.Getenv("LLM_VERIFIER_ENABLED") != "true" {
		slog.Debug("LLM verifier disabled (set LLM_VERIFIER_ENABLED=true to enable)")
		return response, nil
	}

	// Get API key for verification (use same config as main LLM)
	_, apiKey := s.getLLMConfig(ctx, config.TenantID)
	if apiKey == "" {
		slog.Debug("skipping LLM verification - no API key configured")
		return response, nil
	}

	// Create verifier with fast model for verification
	verifierConfig := &VerificationConfig{
		Enabled:      true,
		Model:        "openai/gpt-4o-mini", // Fast and cheap for verification
		Mode:         "enforce",
		MaxLatencyMs: 3000,
		SkipOnError:  true,
	}

	client := newOpenRouterClient(apiKey)
	verifier := NewVerifier(client, verifierConfig)

	// Run verification
	result, err := verifier.VerifyResponse(ctx, response, config)
	if err != nil {
		slog.Warn("verification failed", "error", err)
		s.verificationMetrics.RecordVerification(nil, err)
		return response, nil // Return original on error
	}

	// Return corrected response if available
	if !result.Compliant && result.CorrectedResponse != "" {
		return result.CorrectedResponse, result
	}

	return response, result
}

// ============================================================================
// RATE LIMITING
// ============================================================================

// CheckRateLimit checks if a request is within rate limits.
func (s *Service) CheckRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string, rpm int) (bool, error) {
	if rpm <= 0 {
		rpm = 60 // default
	}

	rl, err := s.store.GetOrCreateAgentRateLimit(ctx, tenantID, audienceType, clientIP)
	if err != nil {
		return false, err
	}

	// Check if window has expired (1 minute)
	if time.Since(rl.WindowStart) > time.Minute {
		if err := s.store.ResetAgentRateLimit(ctx, tenantID, audienceType, clientIP); err != nil {
			return false, err
		}
		return true, nil
	}

	// Check if under limit
	if rl.RequestCount >= rpm {
		return false, nil
	}

	// Increment counter
	if err := s.store.IncrementAgentRateLimit(ctx, tenantID, audienceType, clientIP); err != nil {
		return false, err
	}

	return true, nil
}

// ============================================================================
// CONFIG LOADING
// ============================================================================

// LoadConfig loads the configuration for a tenant and audience.
func (s *Service) LoadConfig(ctx context.Context, tenantSlug, audienceType string) (*AudienceConfig, error) {
	// Check cache first
	if config := s.configCache.Get(tenantSlug, audienceType); config != nil {
		return config, nil
	}

	// Load tenant
	tenant, err := s.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &tenantSlug})
	if err != nil || tenant == nil {
		return nil, fmt.Errorf("tenant not found: %s", tenantSlug)
	}
	if !tenant.IsActive {
		return nil, fmt.Errorf("tenant is not active: %s", tenantSlug)
	}

	// Load audience config
	audience, err := s.store.GetAgentAudience(ctx, &store.FindAgentAudience{
		TenantID:     &tenant.ID,
		AudienceType: &audienceType,
	})
	if err != nil || audience == nil {
		return nil, fmt.Errorf("audience config not found for %s/%s", tenantSlug, audienceType)
	}

	// Load KB data
	active := true
	services, _ := s.store.ListAgentServices(ctx, &store.FindAgentService{
		TenantID: &tenant.ID, AudienceType: &audienceType, IsActive: &active,
	})
	exclusions, _ := s.store.ListAgentExclusions(ctx, &store.FindAgentExclusion{
		TenantID: &tenant.ID, AudienceType: &audienceType, IsActive: &active,
	})
	coverage, _ := s.store.ListAgentCoverage(ctx, &store.FindAgentCoverage{
		TenantID: &tenant.ID,
	})
	faqs, _ := s.store.ListAgentFAQs(ctx, &store.FindAgentFAQ{
		TenantID: &tenant.ID, AudienceType: &audienceType, IsActive: &active,
	})
	safety, _ := s.store.ListAgentSafetyProtocols(ctx, &store.FindAgentSafetyProtocol{
		TenantID: &tenant.ID, AudienceType: &audienceType, IsActive: &active,
	})
	sections, _ := s.store.ListAgentKBSections(ctx, &store.FindAgentKBSection{
		TenantID: &tenant.ID, AudienceType: &audienceType, IsActive: &active,
	})

	// Load policy data
	intents, _ := s.store.ListAgentIntents(ctx, &store.FindAgentIntent{
		TenantID: &tenant.ID, AudienceType: &audienceType, IsActive: &active,
	})
	rules, _ := s.store.ListAgentRules(ctx, &store.FindAgentRule{
		TenantID: &tenant.ID, AudienceType: &audienceType, IsActive: &active,
	})

	// Load conversation flow script (tenant-level, same for all audiences)
	script, _ := s.store.GetAgentTenantScript(ctx, &store.FindAgentTenantScript{
		TenantID: &tenant.ID,
	})

	// Load active learned behaviors (tenant-level)
	learningService := NewLearningService(s.store)
	learnedBehaviors, _ := learningService.GetActiveLearnedBehaviors(ctx, tenant.ID)

	// Load raw KB content for phone extraction fallback
	// This is needed when DB has placeholder phone but KB.MD has real phone
	var rawKB, rawPolicy string
	kbFileType := "kb"
	latestOnly := true
	if kbFile, err := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
		TenantID:     &tenant.ID,
		AudienceType: &audienceType,
		FileType:     &kbFileType,
		LatestOnly:   latestOnly,
	}); err == nil && kbFile != nil {
		rawKB = kbFile.Content
	}

	policyFileType := "policy"
	if policyFile, err := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
		TenantID:     &tenant.ID,
		AudienceType: &audienceType,
		FileType:     &policyFileType,
		LatestOnly:   latestOnly,
	}); err == nil && policyFile != nil {
		rawPolicy = policyFile.Content
	}

	// Determine if tenant has structured content
	// This affects whether we use RAG-only mode or can fall back to long_context mode
	hasStructuredContent := len(services) > 0 || len(faqs) > 0 || len(exclusions) > 0 ||
		len(coverage) > 0 || len(safety) > 0 || len(sections) > 0 ||
		len(intents) > 0 || len(rules) > 0

	config := &AudienceConfig{
		TenantID:             tenant.ID,
		TenantSlug:           tenant.Slug,
		CompanyName:          tenant.CompanyName,
		AudienceType:         audienceType,
		Audience:             audience,
		Services:             services,
		Exclusions:           exclusions,
		Coverage:             coverage,
		FAQs:                 faqs,
		Safety:               safety,
		Sections:             sections,
		Intents:              intents,
		Rules:                rules,
		Script:               script,
		LearnedBehaviors:     learnedBehaviors,
		RawKB:                rawKB,
		RawPolicy:            rawPolicy,
		HasStructuredContent: hasStructuredContent,
	}

	s.configCache.Set(config)
	return config, nil
}

// ============================================================================
// CHAT PROCESSING
// ============================================================================

// ChatRequest represents a chat request.
type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// ChatResponse represents a chat response.
type ChatResponse struct {
	SessionID        string          `json:"session_id"`
	Message          ResponseMessage `json:"message"`
	Metadata         ChatMetadata    `json:"metadata"`
	SessionPersisted bool            `json:"session_persisted,omitempty"`
}

// ResponseMessage represents the assistant's response.
type ResponseMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatMetadata contains metadata about the chat response.
type ChatMetadata struct {
	Intent  string `json:"intent"`
	Urgency int    `json:"urgency"`
	Phase   string `json:"phase"`
}

// ChatExternal handles chat for external (anonymous) users.
// Uses internal audience config (cleaned/production-ready content).
func (s *Service) ChatExternal(ctx context.Context, tenantSlug, clientIP, userAgent string, req ChatRequest) (*ChatResponse, error) {
	// Load config - use internal audience (cleaned content) for external users
	config, err := s.LoadConfig(ctx, tenantSlug, "internal")
	if err != nil {
		return nil, err
	}

	// Validate before any per-session memory or durable lookup. Missing IDs are
	// generated; malformed caller-provided IDs are rejected rather than replaced.
	sessionID, _, err := NormalizeExternalSessionID(req.SessionID)
	if err != nil {
		return nil, err
	}

	// Check rate limit (still track as "external" for rate limiting purposes)
	allowed, err := s.CheckRateLimit(ctx, config.TenantID, "external", clientIP, config.Audience.RateLimitRPM)
	if err != nil {
		slog.Error("rate limit check failed", "error", err)
	}
	if !allowed {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Get or create a tenant-scoped in-memory session.
	session := s.memorySessions.GetOrCreate(config.TenantID, sessionID)

	// Materialization is best-effort until bridge routing is enabled. Unsupported
	// database drivers are expected and intentionally do not produce per-request logs.
	now := time.Now()
	if _, _, materializeErr := s.store.EnsureBridgeExternalSession(ctx, config.TenantID, session.ID, now, now.Add(30*time.Minute)); shouldLogBridgeMaterializationError(materializeErr) {
		slog.Warn("bridge external session materialization failed", "tenant_id", config.TenantID, "error", materializeErr)
	}

	// Process chat
	response, err := s.processChat(ctx, config, session, req.Message)
	if err != nil {
		return nil, err
	}

	// Update memory session
	if err := s.memorySessions.Update(session); err != nil {
		return nil, fmt.Errorf("failed to update external session: %w", err)
	}

	// Save transcript if recording is enabled
	if s.shouldRecordTranscript(ctx, config.TenantID) {
		if err := s.saveTranscript(ctx, session, clientIP, userAgent); err != nil {
			slog.Warn("Failed to save transcript", "sessionID", session.ID, "error", err)
			// Don't fail the request, just log the error
		}
	}

	return response, nil
}

func shouldLogBridgeMaterializationError(err error) bool {
	return err != nil && !errors.Is(err, store.ErrBridgeUnsupportedDatabase)
}

// ChatInternal handles chat for internal (authenticated) users.
func (s *Service) ChatInternal(ctx context.Context, tenantSlug string, userID int32, req ChatRequest) (*ChatResponse, error) {
	// Load config
	config, err := s.LoadConfig(ctx, tenantSlug, "internal")
	if err != nil {
		return nil, err
	}

	// Get or create session
	var session *store.AgentSession
	if req.SessionID != "" {
		session, _ = s.store.GetAgentSession(ctx, &store.FindAgentSession{ID: &req.SessionID})
	}
	if session == nil {
		session = &store.AgentSession{
			ID:             uuid.New().String(),
			TenantID:       config.TenantID,
			UserID:         &userID,
			AudienceType:   "internal",
			Phase:          "triage",
			UrgencyLevel:   0,
			CoverageStatus: "unknown",
			MessageCount:   0,
			Messages:       []store.AgentMessage{},
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		session, err = s.store.CreateAgentSession(ctx, session)
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
	}

	// Process chat
	response, err := s.processChat(ctx, config, session, req.Message)
	if err != nil {
		return nil, err
	}

	// Persist session with customer info for context retention
	msgCount := session.MessageCount
	_, err = s.store.UpdateAgentSession(ctx, &store.UpdateAgentSession{
		ID:               session.ID,
		Phase:            &session.Phase,
		CurrentIntent:    &session.CurrentIntent,
		UrgencyLevel:     &session.UrgencyLevel,
		MessageCount:     &msgCount,
		Messages:         session.Messages,
		CustomerName:     &session.CustomerName,
		CustomerPhone:    &session.CustomerPhone,
		CustomerLocation: &session.CustomerLocation,
	})
	if err != nil {
		slog.Error("failed to persist session", "error", err)
	}

	// Save transcript if recording is enabled
	if s.shouldRecordTranscript(ctx, config.TenantID) {
		if err := s.saveTranscript(ctx, session, "", "internal"); err != nil {
			slog.Warn("Failed to save transcript", "sessionID", session.ID, "error", err)
		}
	}

	response.SessionPersisted = true
	return response, nil
}

// processChat is the core chat processing logic.
func (s *Service) processChat(ctx context.Context, config *AudienceConfig, session *store.AgentSession, userMessage string) (*ChatResponse, error) {
	// Add user message to history
	session.Messages = append(session.Messages, store.AgentMessage{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})
	session.MessageCount++

	// Extract and store customer-provided info for context retention
	// This ensures we track the customer's phone/name/email separately from company info
	validatedCompanyPhone := GetValidatedReplacementPhone(config.Audience.EmergencyPhone, config.RawKB)
	customerInfo := extractCollectedInfo(session.Messages, validatedCompanyPhone)
	if customerInfo.Name != "" && session.CustomerName == "" {
		session.CustomerName = customerInfo.Name
	}
	if customerInfo.Phone != "" && session.CustomerPhone == "" {
		session.CustomerPhone = customerInfo.Phone
	}
	if customerInfo.Address != "" && session.CustomerLocation == "" {
		session.CustomerLocation = customerInfo.Address
	}

	// Score the user message for urgency, sentiment, escalation signals, etc.
	messageScore := ScoreUserMessage(userMessage, config)
	_ = messageScore // Score available for future use in routing decisions

	// Classify intent
	classification, err := s.classifyIntent(ctx, config, userMessage)
	if err != nil {
		slog.Error("classification failed", "error", err)
		// Continue with default intent
		classification = &Classification{
			PrimaryIntent: "unknown",
			Category:      "standard",
			Urgency:       0,
			Confidence:    0.5,
		}
	}

	// Update session state
	session.CurrentIntent = classification.PrimaryIntent
	session.UrgencyLevel = classification.Urgency

	// Handle escalation intent - create ticket if needed
	if classification.PrimaryIntent == "escalation" && GetEscalationTicket(session) == "" {
		// Extract customer info for ticket
		customerInfo := map[string]string{
			"name":  session.CustomerName,
			"phone": session.CustomerPhone,
		}
		// Create escalation ticket
		ticketInfo, err := s.CreateEscalationTicket(ctx, config.TenantID, "supervisor_request", customerInfo, userMessage)
		if err != nil {
			slog.Error("failed to create escalation ticket", "error", err)
		} else {
			SetEscalationTicket(session, ticketInfo.TicketNumber)
			slog.Info("escalation ticket created", "ticket", ticketInfo.TicketNumber, "session_id", session.ID)
		}
	}

	// Handle out-of-coverage - track count and potentially close conversation
	// After 2 insistences (not 3), end the conversation politely
	if classification.PrimaryIntent == "out_of_coverage" || session.CoverageStatus == "outside" {
		count := IncrementOutOfCoverageCount(session)
		if count >= 2 {
			// Mark session as needing closure after 2nd insistence
			session.Phase = "closing"
			slog.Info("out-of-coverage limit reached, closing conversation", "count", count, "session_id", session.ID)
		}
	}

	// Evaluate policy to determine response action
	decision := s.evaluatePolicy(config, session, classification)

	// Generate response
	var response string
	var genErr error

	// Determine which generation method to use based on tenant's retrieval mode:
	// - "rag" mode: Use RAG pipeline with retrieved chunks (for large KBs)
	// - "long_context" mode (default): Full KB in system prompt (for small/medium KBs)
	// - For unstructured content (no structured KB/Policy), always use RAG mode
	useRAG := false
	forceRAG := !config.HasStructuredContent && s.UseRAGPipeline()
	if forceRAG {
		// No structured content - must use RAG mode since there's nothing to build long_context from
		useRAG = true
		slog.Debug("forcing RAG mode for unstructured content",
			"tenant_slug", config.TenantSlug,
			"session_id", session.ID)
	} else if s.UseRAGPipeline() {
		// Check tenant-specific retrieval mode
		tenantConfig, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &config.TenantID})
		if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
			useRAG = true
		}
	}

	if useRAG {
		// Use RAG retrieval for focused context (large KBs or unstructured content)
		response, genErr = s.generateRAGResponse(ctx, config, session, classification, decision, userMessage)
		if genErr != nil {
			if forceRAG {
				// Can't fall back to long context for unstructured content
				slog.Error("RAG generation failed for unstructured content, no fallback available",
					"error", genErr, "session_id", session.ID)
				return nil, fmt.Errorf("failed to generate response: %w", genErr)
			}
			slog.Warn("RAG generation failed, falling back to long context",
				"error", genErr, "session_id", session.ID)
			response, genErr = s.generateResponse(ctx, config, session, classification, decision)
		} else {
			slog.Debug("using RAG pipeline response", "session_id", session.ID)
		}
	} else {
		// Long context mode - full KB in system prompt (small/medium KBs)
		response, genErr = s.generateResponse(ctx, config, session, classification, decision)
	}

	if genErr != nil {
		return nil, fmt.Errorf("failed to generate response: %w", genErr)
	}

	// Sanitize response (remove hallucinated system tags, markdown, etc.)
	response = SanitizeResponse(response)

	// Get validated replacement phone - checks if DB phone is valid, falls back to KB extraction
	// This fixes the issue where DB has placeholder (555) 000-0000 but KB.MD has real phone
	validatedPhone := GetValidatedReplacementPhone(
		config.Audience.EmergencyPhone,
		config.RawKB,
	)
	if validatedPhone == "" && config.Audience != nil && config.Audience.EmergencyPhone != "" {
		slog.Warn("configured emergency phone is invalid placeholder, no valid phone found",
			"configured_phone", config.Audience.EmergencyPhone,
			"session_id", session.ID)
	}

	// Auto-correct placeholder phone numbers (Option C - hybrid approach)
	// This catches hallucinated phones like (555) 000-0000 and replaces with the validated one
	response = CorrectContactsInResponse(response, validatedPhone)

	// Auto-correct placeholder emails (catches hallucinated emails like alex.martinez@email.com)
	if config.Audience != nil && config.Audience.Email != "" {
		response = CorrectEmailsInResponse(response, config.Audience.Email)
	} else {
		// No replacement email configured - just flag placeholders with [email address]
		response = CorrectEmailsInResponse(response, "")
	}

	// LLM-based verification layer (semantic compliance checking)
	verifiedResponse, verificationResult := s.verifyResponseWithLLM(ctx, response, config)
	if verificationResult != nil {
		// Record metrics
		s.verificationMetrics.RecordVerification(verificationResult, nil)

		if !verificationResult.Compliant && verificationResult.CorrectedResponse != "" {
			slog.Info("response corrected by verifier",
				"violations", len(verificationResult.Violations),
				"latency_ms", verificationResult.VerificationTime.Milliseconds(),
				"session_id", session.ID)
			response = verifiedResponse

			// POST-VERIFICATION SANITIZATION (Fix 2)
			// The verifier's corrected response may re-introduce placeholders
			// because the verifier LLM can also hallucinate phone numbers.
			// Apply sanitization again to catch any new placeholder phones.
			response = CorrectContactsInResponse(response, validatedPhone)
			response = CorrectEmailsInResponse(response, config.Audience.Email)
		}

		// Log violations for monitoring
		for _, v := range verificationResult.Violations {
			slog.Warn("verification violation",
				"checklist_item", v.ChecklistItem,
				"severity", v.Severity,
				"evidence", truncate(v.Evidence, 100),
				"session_id", session.ID)
		}
	}

	// If escalation ticket was just created, inject ticket number into response
	if ticketNum := GetEscalationTicket(session); ticketNum != "" {
		// Check if response doesn't already contain a ticket number
		if !strings.Contains(response, "TKT-") && !strings.Contains(response, "CMP-") {
			// Add ticket number to response
			response = fmt.Sprintf("I've created ticket %s for your request. A supervisor will call you at the phone number you provided within 30 minutes.\n\n%s", ticketNum, response)
		}
	}

	// Mark safety as given after first response (for brevity in subsequent responses)
	if !IsSafetyGiven(session) && (classification.Category == "emergency" || classification.Urgency >= 4) {
		MarkSafetyGiven(session)
	}

	// Add assistant message to history
	session.Messages = append(session.Messages, store.AgentMessage{
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	})
	session.MessageCount++
	session.Phase = decision.Phase

	return &ChatResponse{
		SessionID: session.ID,
		Message: ResponseMessage{
			Role:      "assistant",
			Content:   response,
			Timestamp: time.Now(),
		},
		Metadata: ChatMetadata{
			Intent:  classification.PrimaryIntent,
			Urgency: classification.Urgency,
			Phase:   decision.Phase,
		},
	}, nil
}

// ============================================================================
// CLASSIFICATION
// ============================================================================

// Classification represents the result of intent classification.
type Classification struct {
	PrimaryIntent string  `json:"primary_intent"`
	Category      string  `json:"category"`
	Urgency       int     `json:"urgency"`
	Confidence    float64 `json:"confidence"`
}

// classifyIntent uses LLM to classify the user's intent.
func (s *Service) classifyIntent(ctx context.Context, config *AudienceConfig, message string) (*Classification, error) {
	// Get LLM config with tenant-specific fallback
	model, apiKey := s.getLLMConfig(ctx, config.TenantID)
	if apiKey == "" {
		return &Classification{
			PrimaryIntent: "unknown",
			Category:      "standard",
			Urgency:       0,
			Confidence:    0.5,
		}, nil
	}

	// Build intent list for prompt
	var intentList strings.Builder
	for _, intent := range config.Intents {
		intentList.WriteString(fmt.Sprintf("- %s (%s, urgency: %d): %s\n", intent.Code, intent.Category, intent.Urgency, intent.Description))
		if len(intent.Examples) > 0 {
			intentList.WriteString("  Examples: " + strings.Join(intent.Examples[:min(3, len(intent.Examples))], ", ") + "\n")
		}
	}

	prompt := fmt.Sprintf(`You are an intent classifier for %s, a %s company.

Available intents:
%s

Classify the following user message and respond ONLY with a JSON object:
{
  "primary_intent": "<intent_code>",
  "category": "<emergency|standard|meta>",
  "urgency": <0-5>,
  "confidence": <0.0-1.0>
}

User message: "%s"

JSON response:`, config.CompanyName, config.Audience.Role, intentList.String(), message)

	client := newOpenRouterClient(apiKey)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model: model,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage("You are an intent classifier. Respond only with valid JSON."),
			openrouter.UserMessage(prompt),
		},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	// Parse response
	responseText := resp.Choices[0].Message.Content.Text
	responseText = strings.TrimSpace(responseText)
	// Remove markdown code blocks if present
	if strings.HasPrefix(responseText, "```") {
		lines := strings.Split(responseText, "\n")
		if len(lines) > 2 {
			responseText = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var classification Classification
	if err := json.Unmarshal([]byte(responseText), &classification); err != nil {
		// Try to extract just the JSON part
		start := strings.Index(responseText, "{")
		end := strings.LastIndex(responseText, "}")
		if start >= 0 && end > start {
			if err := json.Unmarshal([]byte(responseText[start:end+1]), &classification); err != nil {
				slog.Error("failed to parse classification", "response", responseText, "error", err)
				return &Classification{
					PrimaryIntent: "unknown",
					Category:      "standard",
					Urgency:       0,
					Confidence:    0.5,
				}, nil
			}
		}
	}

	return &classification, nil
}

// ============================================================================
// POLICY EVALUATION
// ============================================================================

// PolicyDecision represents the result of policy evaluation.
type PolicyDecision struct {
	Action        string
	Phase         string
	SafetyTrigger *store.AgentSafetyProtocol
	AppliedRules  []string
}

// evaluatePolicy evaluates the policy rules and determines the response action.
func (s *Service) evaluatePolicy(config *AudienceConfig, session *store.AgentSession, classification *Classification) *PolicyDecision {
	decision := &PolicyDecision{
		Action:       "standard_flow",
		Phase:        session.Phase,
		AppliedRules: []string{},
	}

	// Check for emergency urgency threshold
	if classification.Urgency >= config.Audience.EmergencyUrgencyThreshold {
		decision.Action = "emergency_flow"
		decision.Phase = "emergency"
	}

	// Check for safety protocol triggers
	for _, safety := range config.Safety {
		for _, trigger := range safety.TriggerIntents {
			if trigger == classification.PrimaryIntent {
				decision.SafetyTrigger = safety
				decision.Action = "safety_flow"
				decision.Phase = "safety"
				break
			}
		}
	}

	// Apply rules based on priority
	for _, rule := range config.Rules {
		// Check if rule applies to current intent or category
		if rule.AppliesTo == "" || rule.AppliesTo == classification.PrimaryIntent || rule.AppliesTo == classification.Category {
			decision.AppliedRules = append(decision.AppliedRules, rule.Code)
		}
	}

	// Determine phase progression
	if decision.Phase == "triage" && classification.Confidence > 0.7 {
		decision.Phase = "handshake"
	}

	return decision
}

// ============================================================================
// RESPONSE GENERATION
// ============================================================================

// generateResponse uses LLM to generate a contextual response.
func (s *Service) generateResponse(ctx context.Context, config *AudienceConfig, session *store.AgentSession, classification *Classification, decision *PolicyDecision) (string, error) {
	// Get LLM config with tenant-specific fallback
	model, apiKey := s.getLLMConfig(ctx, config.TenantID)
	if apiKey == "" {
		return "I apologize, but the chat service is not currently available. Please call us directly.", nil
	}

	// Build system prompt (passing session for context retention)
	systemPrompt := s.buildSystemPrompt(ctx, config, session, classification, decision)

	// Build conversation history
	messages := []openrouter.ChatCompletionMessage{
		openrouter.SystemMessage(systemPrompt),
	}

	// Add conversation history (limited to last 10 messages)
	historyStart := 0
	if len(session.Messages) > 10 {
		historyStart = len(session.Messages) - 10
	}
	for _, msg := range session.Messages[historyStart:] {
		if msg.Role == "user" {
			messages = append(messages, openrouter.UserMessage(msg.Content))
		} else {
			messages = append(messages, openrouter.AssistantMessage(msg.Content))
		}
	}

	client := newOpenRouterClient(apiKey)

	slog.Debug("LLM: Calling OpenRouter",
		"session_id", session.ID,
		"model", model,
		"message_count", len(messages))

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		slog.Error("LLM: OpenRouter call failed",
			"error", err,
			"session_id", session.ID)
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	// Trigger Observational Memory update asynchronously with threshold check and debouncing
	// We use a background context to ensure the observer runs even if the request context is cancelled
	go func() {
		// Check if OM is enabled
		omConfig := GetOMConfig()
		if !omConfig.Enabled {
			return
		}

		// Check message threshold before running observer
		obsLog, err := s.store.GetObservationLog(context.Background(), session.ID)
		if err != nil {
			slog.Debug("Failed to get observation log for threshold check", "session_id", session.ID, "error", err)
			return
		}

		lastObservedIdx := -1
		if obsLog != nil {
			lastObservedIdx = obsLog.LastObservedMsgIndex
		}

		// Calculate unobserved token count (token-based trigger)
		// This aligns with Mastra's OM specification for consistent triggering
		unobservedTokens := 0
		for i := lastObservedIdx + 1; i < len(session.Messages); i++ {
			unobservedTokens += estimateTokens(session.Messages[i].Content)
		}

		threshold := omConfig.ObserverTokenThreshold

		// Check if we should trigger buffer pre-computation
		if s.observerBuffer != nil && s.observerBuffer.ShouldTriggerBuffer(unobservedTokens, threshold) {
			// Check if we already have a buffer for this session
			if !s.observerBuffer.HasBuffer(session.TenantID, session.ID) {
				slog.Debug("Triggering buffer observation", "session_id", session.ID, "unobserved_tokens", unobservedTokens)
				s.observerBuffer.TriggerBuffer(session.TenantID, session.ID)
			}
		}

		// Check if we should activate the buffer (threshold reached)
		if s.observerBuffer != nil && s.observerBuffer.ShouldActivateBuffer(unobservedTokens, threshold) {
			// Try to get buffered observations
			observations, currentTask, suggestedResp, tokenCount, lastMsgIdx, resourceID, ok := s.observerBuffer.GetAndActivateBuffer(session.TenantID, session.ID)
			if ok {
				slog.Debug("Activating buffered observation", "session_id", session.ID, "tokens", tokenCount)
				// Store the buffered observation
				if err := s.storeObservationFromBuffer(context.Background(), session.TenantID, session.ID, observations, currentTask, suggestedResp, lastMsgIdx, resourceID); err != nil {
					slog.Error("Failed to store buffered observation", "session_id", session.ID, "error", err)
				}
				s.observerBuffer.ClearBuffer(session.TenantID, session.ID)
				return
			}
		}

		// Check if we're past the block threshold - force synchronous observation
		if s.observerBuffer != nil && s.observerBuffer.ShouldBlock(unobservedTokens, threshold) {
			slog.Warn("Observer buffer can't keep up, forcing synchronous observation",
				"session_id", session.ID,
				"unobserved_tokens", unobservedTokens,
				"threshold", threshold)
		}

		if unobservedTokens < threshold {
			slog.Debug("Token threshold not reached, skipping observer",
				"session_id", session.ID,
				"unobserved_tokens", unobservedTokens,
				"threshold", threshold)
			return
		}

		// Try to acquire lock for debouncing
		if !GetObserverMutex().TryLock(session.ID) {
			slog.Debug("Observer already running, skipping", "session_id", session.ID)
			return
		}
		defer GetObserverMutex().Unlock(session.ID)

		if err := s.RunObserver(context.Background(), session.TenantID, session.ID); err != nil {
			slog.Error("Failed to run observer", "session_id", session.ID, "error", err)
		}
	}()

	return resp.Choices[0].Message.Content.Text, nil
}

// buildSystemPrompt constructs the system prompt for the LLM.
// Structure optimized for compliance: constraints first, then context.
func (s *Service) buildSystemPrompt(ctx context.Context, config *AudienceConfig, session *store.AgentSession, classification *Classification, decision *PolicyDecision) string {
	var sb strings.Builder

	// Compute validated phone number once for use throughout prompt
	// This prevents telling the LLM to use placeholder phones like (555) 000-0000
	validatedPhone := GetValidatedReplacementPhone(config.Audience.EmergencyPhone, config.RawKB)

	// =========================================================================
	// SECTION 0: CUSTOMER INFO ALREADY PROVIDED (Context Retention)
	// =========================================================================
	// Extract info the customer has already provided to prevent re-asking
	if session != nil && len(session.Messages) > 0 {
		collectedInfo := extractCollectedInfo(session.Messages, validatedPhone)
		hasInfo := collectedInfo.Name != "" || collectedInfo.Phone != "" || collectedInfo.Email != "" || collectedInfo.Address != ""

		if hasInfo {
			sb.WriteString("=== CUSTOMER INFO ALREADY PROVIDED (DO NOT ASK AGAIN) ===\n\n")
			sb.WriteString("The customer has already provided the following information in this conversation:\n")
			if collectedInfo.Name != "" {
				sb.WriteString("- Customer Name: " + collectedInfo.Name + "\n")
			}
			if collectedInfo.Phone != "" {
				sb.WriteString("- Customer Phone: " + collectedInfo.Phone + "\n")
			}
			if collectedInfo.Email != "" {
				sb.WriteString("- Customer Email: " + collectedInfo.Email + "\n")
			}
			if collectedInfo.Address != "" {
				sb.WriteString("- Customer Address: " + collectedInfo.Address + "\n")
			}
			sb.WriteString("\nIMPORTANT: Do NOT ask for this information again. Acknowledge that you have it.\n")
			// CRITICAL: Add explicit instruction to preserve customer phone verbatim
			if collectedInfo.Phone != "" {
				sb.WriteString("CRITICAL: When echoing back the customer's phone number, use EXACTLY: " + collectedInfo.Phone + "\n")
				sb.WriteString("This is the CUSTOMER's phone - do NOT replace it with the company phone number!\n")
			}
			sb.WriteString("\n")
		}
	}

	// =========================================================================
	// SECTION 0.5: OBSERVATIONAL MEMORY (Long-term Context)
	// =========================================================================
	if session != nil {
		obsLog, _ := s.store.GetObservationLog(ctx, session.ID)
		if obsLog != nil {
			// Inject observation log
			if obsLog.ObservationLog != "" {
				sb.WriteString("=== OBSERVATIONAL MEMORY (Historical Context) ===\n\n")
				sb.WriteString("The following are observations from previous interactions with this user. Use this context to personalize your responses.\n\n")
				sb.WriteString(obsLog.ObservationLog)
				sb.WriteString("\n\n")
			}
			// Inject current task for continuity
			if obsLog.CurrentTask != "" {
				sb.WriteString("=== CURRENT TASK ===\n\n")
				sb.WriteString(obsLog.CurrentTask)
				sb.WriteString("\n\n")
			}
			// Inject suggested response hint
			if obsLog.SuggestedResponse != "" {
				sb.WriteString("=== SUGGESTED NEXT ACTION ===\n\n")
				sb.WriteString(obsLog.SuggestedResponse)
				sb.WriteString("\n\n")
			}
		}
	}

	// =========================================================================
	// SECTION 1: CRITICAL CONSTRAINTS (Highest Priority - Must be at TOP)
	// =========================================================================
	sb.WriteString("=== CRITICAL CONSTRAINTS (YOU MUST FOLLOW THESE) ===\n\n")

	sb.WriteString("1. DO NOT INVENT SERVICES: You may ONLY mention services listed in the \"SERVICES WE OFFER\" section below. ")
	sb.WriteString("If a customer asks about a service not listed, say \"I don't have information about that service\" or offer to connect them with someone who can help.\n\n")

	sb.WriteString("2. DO NOT INVENT CONTACT INFO: You may ONLY provide phone numbers and emails listed in the \"AUTHORIZED CONTACT INFO\" section below. ")
	sb.WriteString("Never make up or guess contact information.\n\n")

	// Add explicit phone number constraint with specific placeholder detection
	sb.WriteString("3. PHONE NUMBER REQUIREMENT: ")
	if validatedPhone != "" {
		sb.WriteString(fmt.Sprintf("The ONLY valid phone number is: %s. ", validatedPhone))
	}
	sb.WriteString("NEVER use placeholder numbers like (555) xxx-xxxx, (000) xxx-xxxx, or (123) 456-7890. ")
	sb.WriteString("If you don't know a phone number, say \"Please call our emergency line\" without inventing a number.\n\n")

	sb.WriteString("4. DO NOT OFFER EXCLUDED SERVICES: Services in the \"SERVICES WE DON'T PROVIDE\" section are explicitly excluded. ")
	sb.WriteString("Never offer or promise these services.\n\n")

	sb.WriteString("5. DO NOT INVENT PROCESSES: Only describe processes, procedures, or steps that are documented in the \"CONVERSATION FLOW\" or \"FAQS\" sections. ")
	sb.WriteString("If you don't know a specific process, say \"I'll need to check on that\" or \"Let me connect you with someone who can explain that.\"\n\n")

	sb.WriteString("6. DO NOT MAKE PROMISES: Never promise specific response times, prices, or outcomes unless explicitly stated in the knowledge base.\n\n")

	sb.WriteString("7. WHEN UNCERTAIN: If you're unsure about any information, acknowledge it honestly. ")
	sb.WriteString("Say \"I'm not certain about that\" rather than guessing.\n\n")

	sb.WriteString("8. CONTACT INFORMATION HANDLING:\n")
	if validatedPhone != "" {
		sb.WriteString("   - COMPANY CONTACT: When providing YOUR phone number, use ONLY: " + validatedPhone + "\n")
	}
	sb.WriteString("   - CUSTOMER CONTACT: When echoing back a customer's phone, use EXACTLY what they said\n")
	sb.WriteString("   - NEVER modify or 'correct' a customer-provided phone number\n")
	sb.WriteString("   - Example: Customer says '555-123-4567' → You respond '555-123-4567'\n\n")

	// =========================================================================
	// SECTION 1B: SCOPE OF KNOWLEDGE (Tenant Boundary)
	// =========================================================================
	sb.WriteString("=== SCOPE OF KNOWLEDGE ===\n\n")
	sb.WriteString(fmt.Sprintf("You are the assistant for %s ONLY.\n", config.CompanyName))
	sb.WriteString("Your knowledge is LIMITED to:\n")
	sb.WriteString("- The services, policies, and information provided in this prompt\n")
	sb.WriteString("- The conversation history with this customer\n\n")
	sb.WriteString("If asked about:\n")
	sb.WriteString(fmt.Sprintf("- Other companies or businesses: Politely explain you can only assist with %s inquiries\n", config.CompanyName))
	sb.WriteString("- Topics not covered in your knowledge base: Politely explain you don't have that information and offer to help with what you DO know\n")
	sb.WriteString(fmt.Sprintf("- General knowledge questions unrelated to %s: Redirect to how you can help with %s services\n\n", config.CompanyName, config.CompanyName))
	sb.WriteString("NEVER:\n")
	sb.WriteString("- Pretend to have knowledge you don't have\n")
	sb.WriteString("- Answer questions about other tenants or businesses\n")
	sb.WriteString("- Provide generic answers when you should decline\n\n")

	// =========================================================================
	// SECTION 2: IDENTITY (from POLICY.MD)
	// =========================================================================
	sb.WriteString("=== YOUR IDENTITY ===\n\n")
	sb.WriteString(fmt.Sprintf("You are a %s for %s.\n", config.Audience.Role, config.CompanyName))
	sb.WriteString(fmt.Sprintf("Tone: %s\n", config.Audience.Tone))
	if config.Audience.BrandVoice != "" {
		sb.WriteString(fmt.Sprintf("Brand voice: \"%s\"\n", config.Audience.BrandVoice))
	}
	sb.WriteString("\n")

	// Guidelines
	if len(config.Audience.Guidelines) > 0 {
		sb.WriteString("Guidelines:\n")
		for _, g := range config.Audience.Guidelines {
			sb.WriteString("- " + g + "\n")
		}
		sb.WriteString("\n")
	}

	// =========================================================================
	// SECTION 3: SERVICES WE OFFER (from KB.MD) - ONLY these can be mentioned
	// =========================================================================
	if len(config.Services) > 0 {
		sb.WriteString("=== SERVICES WE OFFER (Only mention these) ===\n\n")
		for _, svc := range config.Services {
			emergency := ""
			if svc.IsEmergency {
				emergency = " [EMERGENCY SERVICE]"
			}
			sb.WriteString(fmt.Sprintf("- %s%s: %s\n", svc.Name, emergency, truncate(svc.Description, 100)))
		}
		sb.WriteString("\n")
	}

	// =========================================================================
	// SECTION 4: SERVICES WE DON'T PROVIDE (from KB.MD) - NEVER offer these
	// =========================================================================
	if len(config.Exclusions) > 0 {
		sb.WriteString("=== SERVICES WE DON'T PROVIDE (Never offer these) ===\n\n")
		for _, exc := range config.Exclusions {
			sb.WriteString(fmt.Sprintf("- %s", exc.Name))
			if exc.Referral != "" {
				sb.WriteString(fmt.Sprintf(" (if asked, recommend: %s)", exc.Referral))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// =========================================================================
	// SECTION 5: CONVERSATION FLOW (from SCRIPT.MD)
	// =========================================================================
	if config.Script != nil {
		scriptContent := config.Script.Summary
		if scriptContent == "" {
			scriptContent = config.Script.Content
		}
		if scriptContent != "" {
			sb.WriteString("=== CONVERSATION FLOW (Follow this structure) ===\n\n")
			sb.WriteString(scriptContent)
			sb.WriteString("\n\n")
		}
	}

	// =========================================================================
	// SECTION 6: POLICIES & RULES (from POLICY.MD)
	// =========================================================================
	if len(decision.AppliedRules) > 0 {
		sb.WriteString("=== POLICIES & RULES (Follow these) ===\n\n")
		for _, ruleCode := range decision.AppliedRules {
			for _, rule := range config.Rules {
				if rule.Code == ruleCode {
					sb.WriteString("- " + rule.Name + ": " + rule.Description + "\n")
					break
				}
			}
		}
		sb.WriteString("\n")
	}

	// Safety trigger (high priority when present)
	if decision.SafetyTrigger != nil {
		sb.WriteString("!!! SAFETY PROTOCOL TRIGGERED !!!\n")
		sb.WriteString("This is a " + decision.SafetyTrigger.Name + " situation.\n")
		sb.WriteString("You MUST provide these instructions FIRST in your response:\n")
		for i, inst := range decision.SafetyTrigger.Instructions {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, inst))
		}
		sb.WriteString("\n")
	}

	// Emergency handling
	if classification.Urgency >= config.Audience.EmergencyUrgencyThreshold {
		sb.WriteString("!!! EMERGENCY SITUATION !!!\n")
		if validatedPhone != "" {
			sb.WriteString(fmt.Sprintf("Provide the emergency phone number: %s\n", validatedPhone))
		} else {
			sb.WriteString("Ask the customer to call our emergency line directly.\n")
		}
		sb.WriteString("Treat this with urgency and empathy.\n\n")
	}

	// =========================================================================
	// SECTION 7: LEARNED BEHAVIORS (from analysis - enforced)
	// =========================================================================
	if len(config.LearnedBehaviors) > 0 {
		sb.WriteString("=== MANDATORY BEHAVIORS (Apply these in every response) ===\n\n")
		for i, b := range config.LearnedBehaviors {
			// v2 format uses Content directly, v1 uses Trigger + Behavior
			if b.Content != "" {
				sb.WriteString(fmt.Sprintf("L%d. %s\n", i+1, b.Content))
			} else if b.Trigger != "" && b.Behavior != "" {
				sb.WriteString(fmt.Sprintf("L%d. When %s: %s\n", i+1, b.Trigger, b.Behavior))
			}
		}
		sb.WriteString("\n")
	}

	// =========================================================================
	// SECTION 8: FAQS (Reference material)
	// =========================================================================
	if len(config.FAQs) > 0 && classification.Category != "emergency" {
		sb.WriteString("=== FAQS (Use these for accurate answers) ===\n\n")
		for _, faq := range config.FAQs {
			sb.WriteString(fmt.Sprintf("Q: %s\nA: %s\n\n", faq.Question, truncate(faq.Answer, 150)))
		}
	}

	// =========================================================================
	// SECTION 9: AUTHORIZED CONTACT INFO (Only provide these)
	// =========================================================================
	sb.WriteString("=== AUTHORIZED CONTACT INFO (Only provide these) ===\n\n")
	if validatedPhone != "" {
		sb.WriteString(fmt.Sprintf("Phone: %s\n", validatedPhone))
	} else {
		sb.WriteString("Phone: [No valid phone configured - do not provide any phone number]\n")
	}
	if config.Audience.Email != "" {
		sb.WriteString(fmt.Sprintf("Email: %s\n", config.Audience.Email))
	}
	sb.WriteString("Do NOT provide any other phone numbers or emails.\n\n")

	// =========================================================================
	// SECTION 10: RESPONSE FORMAT
	// =========================================================================
	sb.WriteString("=== RESPONSE FORMAT ===\n\n")
	sb.WriteString("- Use plain text only. NO markdown (no **, no *, no # headers, no - bullets).\n")
	sb.WriteString("- Use natural sentence structure.\n")
	sb.WriteString("- For lists, use numbered sentences or comma-separated items.\n")
	sb.WriteString("- Be concise but complete.\n")
	sb.WriteString("- If this is an emergency, lead with safety instructions and the emergency phone number.\n\n")

	// =========================================================================
	// SECTION 11: CRITICAL REMINDER (Context Reinforcement - Anchored at END)
	// =========================================================================
	// This section reinforces critical constraints at the end of the prompt
	// to combat context dilution over long conversations.
	sb.WriteString("=== CRITICAL REMINDER (DO NOT IGNORE) ===\n\n")
	if validatedPhone != "" {
		sb.WriteString("EMERGENCY PHONE: " + validatedPhone + " (USE ONLY THIS NUMBER)\n")
	} else {
		sb.WriteString("EMERGENCY PHONE: [No valid phone configured - do not provide any phone number]\n")
	}
	if config.Audience.Email != "" {
		sb.WriteString("EMAIL: " + config.Audience.Email + "\n")
	}
	sb.WriteString("NEVER use placeholder numbers like (555), (000), or (123) 456-7890.\n")
	sb.WriteString("If you don't know a phone number, say 'Please call us directly' - DO NOT invent one.\n")
	sb.WriteString("DO NOT ask for information the customer has already provided in this conversation.\n")
	sb.WriteString(fmt.Sprintf("You represent %s ONLY - politely decline queries about other businesses.\n", config.CompanyName))
	sb.WriteString("If you don't have the information, say so politely rather than guessing.\n")

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ============================================================================
// RAG PIPELINE (Phase 3)
// ============================================================================

// UseRAGPipeline determines if RAG pipeline should be used for this request.
// Returns true if RAG is enabled and vector database is available.
func (s *Service) UseRAGPipeline() bool {
	if s.vectorDB == nil {
		return false
	}
	// Check if it's a NoOp (RAG disabled)
	_, isNoOp := s.vectorDB.(*NoOpVectorDB)
	return !isNoOp
}

// generateRAGResponse generates a response using RAG retrieval for context.
// This retrieves only relevant content from the vector database instead of
// including the entire KB in the system prompt.
func (s *Service) generateRAGResponse(
	ctx context.Context,
	config *AudienceConfig,
	session *store.AgentSession,
	classification *Classification,
	decision *PolicyDecision,
	userMessage string,
) (string, error) {
	// Get LLM config
	model, apiKey := s.getLLMConfig(ctx, config.TenantID)
	if apiKey == "" {
		return "I apologize, but the chat service is not currently available. Please call us directly.", nil
	}

	// Retrieve relevant context from vector database with hybrid search if enabled
	var hybridOpts *HybridSearchOptions
	if s.vectorDBConfig != nil && s.vectorDBConfig.HybridSearchEnabled {
		hybridOpts = &HybridSearchOptions{
			Enabled:      true,
			VectorWeight: s.vectorDBConfig.HybridVectorWeight,
			TextWeight:   s.vectorDBConfig.HybridTextWeight,
		}
	}
	slog.Debug("RAG: Starting context retrieval",
		"session_id", session.ID,
		"query", userMessage)
	retrieved, err := RetrieveContextForQuery(
		ctx,
		s.vectorDB,
		userMessage,
		classification.PrimaryIntent,
		config.TenantID,
		config.AudienceType,
		hybridOpts,
	)
	if err != nil {
		slog.Warn("RAG retrieval failed, falling back to full context",
			"error", err,
			"session_id", session.ID)
		// Fall back to regular generation
		return s.generateResponse(ctx, config, session, classification, decision)
	}

	// Build RAG-optimized system prompt
	systemPrompt := s.buildRAGSystemPrompt(config, session, classification, decision, retrieved)

	// Build conversation history
	messages := []openrouter.ChatCompletionMessage{
		openrouter.SystemMessage(systemPrompt),
	}

	// Add conversation history (limited to last 10 messages)
	historyStart := 0
	if len(session.Messages) > 10 {
		historyStart = len(session.Messages) - 10
	}
	for _, msg := range session.Messages[historyStart:] {
		if msg.Role == "user" {
			messages = append(messages, openrouter.UserMessage(msg.Content))
		} else {
			messages = append(messages, openrouter.AssistantMessage(msg.Content))
		}
	}

	client := newOpenRouterClient(apiKey)

	slog.Debug("RAG: Calling LLM",
		"session_id", session.ID,
		"model", model,
		"message_count", len(messages))

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		slog.Error("RAG: LLM call failed",
			"error", err,
			"session_id", session.ID)
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	slog.Debug("RAG response generated",
		"session_id", session.ID,
		"retrieved_services", len(retrieved.Services),
		"retrieved_faqs", len(retrieved.FAQs),
		"intent", classification.PrimaryIntent)

	return resp.Choices[0].Message.Content.Text, nil
}

// buildRAGSystemPrompt constructs a system prompt using RAG-retrieved context.
// This is more focused than buildSystemPrompt as it only includes relevant content.
func (s *Service) buildRAGSystemPrompt(
	config *AudienceConfig,
	session *store.AgentSession,
	classification *Classification,
	decision *PolicyDecision,
	retrieved *RetrievedContext,
) string {
	var sb strings.Builder

	// Compute validated phone number once
	var validatedPhone string
	if config.Audience != nil {
		validatedPhone = GetValidatedReplacementPhone(config.Audience.EmergencyPhone, config.RawKB)
	}

	// =========================================================================
	// SECTION 1: IDENTITY (Who you are)
	// =========================================================================
	sb.WriteString("=== IDENTITY ===\n")

	// For unstructured content (no structured Policy), use minimal identity
	// or derive from SCRIPT.MD if available
	hasStructuredIdentity := config.Audience != nil && (config.Audience.Role != "" || config.Audience.Tone != "")

	if hasStructuredIdentity {
		sb.WriteString(fmt.Sprintf("You are a %s for %s.\n", config.Audience.Role, config.CompanyName))
		if config.Audience.Tone != "" {
			sb.WriteString(fmt.Sprintf("Tone: %s\n", config.Audience.Tone))
		}
		if config.Audience.BrandVoice != "" {
			sb.WriteString(fmt.Sprintf("Voice: \"%s\"\n", config.Audience.BrandVoice))
		}
	} else {
		// Minimal identity for unstructured content
		if config.CompanyName != "" {
			sb.WriteString(fmt.Sprintf("You are a helpful assistant for %s.\n", config.CompanyName))
		} else {
			sb.WriteString("You are a helpful assistant.\n")
		}
		sb.WriteString("Tone: Professional and helpful\n")
	}
	sb.WriteString("\n")

	// =========================================================================
	// SECTION 2: CUSTOMER CONTEXT (Info already collected - critical)
	// =========================================================================
	if session != nil && len(session.Messages) > 0 {
		collectedInfo := extractCollectedInfo(session.Messages, validatedPhone)
		hasInfo := collectedInfo.Name != "" || collectedInfo.Phone != "" || collectedInfo.Email != "" || collectedInfo.Address != ""

		if hasInfo {
			sb.WriteString("=== CUSTOMER INFO (DO NOT ASK AGAIN) ===\n")
			if collectedInfo.Name != "" {
				sb.WriteString("Name: " + collectedInfo.Name + "\n")
			}
			if collectedInfo.Phone != "" {
				sb.WriteString("Phone: " + collectedInfo.Phone + " (use exactly this when echoing back)\n")
			}
			if collectedInfo.Email != "" {
				sb.WriteString("Email: " + collectedInfo.Email + "\n")
			}
			if collectedInfo.Address != "" {
				sb.WriteString("Address: " + collectedInfo.Address + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// =========================================================================
	// SECTION 3: CONSTRAINTS & CONTACT (Combined for efficiency)
	// =========================================================================
	sb.WriteString("=== CONSTRAINTS ===\n")
	sb.WriteString("- Only discuss information in RETRIEVED CONTEXT below\n")
	sb.WriteString("- Never invent information, facts, or details not in the context\n")
	sb.WriteString("- If uncertain, acknowledge honestly\n")
	if validatedPhone != "" {
		sb.WriteString(fmt.Sprintf("- Phone: %s (ONLY this number)\n", validatedPhone))
	}
	if config.Audience != nil && config.Audience.Email != "" {
		sb.WriteString(fmt.Sprintf("- Email: %s\n", config.Audience.Email))
	}
	if config.CompanyName != "" {
		sb.WriteString(fmt.Sprintf("- You assist with %s topics ONLY - decline unrelated queries\n", config.CompanyName))
	}
	sb.WriteString("- If topic not in retrieved context, politely decline and offer relevant help\n")
	sb.WriteString("\n")

	// =========================================================================
	// SECTION 4: CONVERSATION GUIDE (from SCRIPT.MD)
	// =========================================================================
	if config.Script != nil {
		scriptContent := config.Script.Summary
		if scriptContent == "" {
			scriptContent = config.Script.Content
		}
		if scriptContent != "" {
			sb.WriteString("=== CONVERSATION GUIDE ===\n")
			sb.WriteString(scriptContent)
			sb.WriteString("\n\n")
		}
	}

	// =========================================================================
	// SECTION 5: RETRIEVED CONTEXT (All RAG content unified)
	// =========================================================================
	hasRetrievedContent := len(retrieved.Services) > 0 || len(retrieved.FAQs) > 0 ||
		len(retrieved.Coverage) > 0 || len(retrieved.Rules) > 0 ||
		len(retrieved.Safety) > 0 || len(retrieved.KBSections) > 0 ||
		len(retrieved.Exclusions) > 0

	if hasRetrievedContent {
		sb.WriteString("=== RETRIEVED CONTEXT (Use ONLY this information) ===\n\n")

		// Services
		if len(retrieved.Services) > 0 {
			sb.WriteString("SERVICES:\n")
			for _, chunk := range retrieved.Services {
				emergency := ""
				if chunk.IsEmergency {
					emergency = " [EMERGENCY]"
				}
				sb.WriteString(fmt.Sprintf("- %s%s: %s\n", chunk.Title, emergency, chunk.Content))
			}
			sb.WriteString("\n")
		}

		// FAQs
		if len(retrieved.FAQs) > 0 {
			sb.WriteString("FAQS:\n")
			for _, chunk := range retrieved.FAQs {
				sb.WriteString(chunk.Content + "\n\n")
			}
		}

		// Coverage
		if len(retrieved.Coverage) > 0 {
			sb.WriteString("COVERAGE:\n")
			for _, chunk := range retrieved.Coverage {
				sb.WriteString(fmt.Sprintf("- %s\n", chunk.Content))
			}
			sb.WriteString("\n")
		}

		// Exclusions (only if relevant to query)
		if len(retrieved.Exclusions) > 0 {
			sb.WriteString("NOT PROVIDED:\n")
			for _, chunk := range retrieved.Exclusions {
				sb.WriteString(fmt.Sprintf("- %s\n", chunk.Content))
			}
			sb.WriteString("\n")
		}

		// Rules (policy-based)
		if len(retrieved.Rules) > 0 {
			sb.WriteString("POLICIES:\n")
			for _, chunk := range retrieved.Rules {
				sb.WriteString(fmt.Sprintf("- %s\n", chunk.Content))
			}
			sb.WriteString("\n")
		}

		// Safety protocols
		if len(retrieved.Safety) > 0 {
			sb.WriteString("SAFETY:\n")
			for _, chunk := range retrieved.Safety {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", chunk.Title, chunk.Content))
			}
			sb.WriteString("\n")
		}

		// General KB sections
		if len(retrieved.KBSections) > 0 {
			sb.WriteString("INFORMATION:\n")
			for _, chunk := range retrieved.KBSections {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", chunk.Title, chunk.Content))
			}
			sb.WriteString("\n")
		}
	}

	// =========================================================================
	// SECTION 6: ACTIVE RULES & SAFETY (Context-specific, high priority)
	// =========================================================================
	// Applied rules from policy decision
	if len(decision.AppliedRules) > 0 {
		sb.WriteString("=== ACTIVE RULES ===\n")
		for _, ruleCode := range decision.AppliedRules {
			for _, rule := range config.Rules {
				if rule.Code == ruleCode {
					sb.WriteString("- " + rule.Name + ": " + rule.Description + "\n")
					break
				}
			}
		}
		sb.WriteString("\n")
	}

	// Safety trigger (highest priority)
	if decision.SafetyTrigger != nil {
		sb.WriteString("!!! SAFETY PROTOCOL: " + decision.SafetyTrigger.Name + " !!!\n")
		sb.WriteString("Provide these instructions FIRST:\n")
		for i, inst := range decision.SafetyTrigger.Instructions {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, inst))
		}
		sb.WriteString("\n")
	}

	// Emergency flag (check urgency threshold if audience config exists)
	emergencyThreshold := 4 // Default threshold
	if config.Audience != nil && config.Audience.EmergencyUrgencyThreshold > 0 {
		emergencyThreshold = config.Audience.EmergencyUrgencyThreshold
	}
	if classification.Urgency >= emergencyThreshold {
		sb.WriteString("!!! EMERGENCY - Respond with urgency !!!\n")
		if validatedPhone != "" {
			sb.WriteString(fmt.Sprintf("Provide phone immediately: %s\n", validatedPhone))
		}
		sb.WriteString("\n")
	}

	// Learned behaviors (if any)
	if len(config.LearnedBehaviors) > 0 {
		sb.WriteString("=== BEHAVIORS ===\n")
		for _, b := range config.LearnedBehaviors {
			if b.Content != "" {
				sb.WriteString("- " + b.Content + "\n")
			} else if b.Trigger != "" && b.Behavior != "" {
				sb.WriteString(fmt.Sprintf("- When %s: %s\n", b.Trigger, b.Behavior))
			}
		}
		sb.WriteString("\n")
	}

	// =========================================================================
	// SECTION 7: RESPONSE FORMAT (Minimal)
	// =========================================================================
	sb.WriteString("=== FORMAT ===\n")
	sb.WriteString("Plain text only, no markdown. Be concise but complete.\n")

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CollectedCustomerInfo holds information extracted from customer messages.
type CollectedCustomerInfo struct {
	Name    string
	Phone   string
	Email   string
	Address string
}

// extractCollectedInfo scans conversation history to find customer-provided information.
// This helps prevent the agent from re-asking for info already provided.
// It also detects corrections (e.g., "my phone is actually X") and updates accordingly.
func extractCollectedInfo(messages []store.AgentMessage, tenantPhone string) *CollectedCustomerInfo {
	info := &CollectedCustomerInfo{}

	// Patterns for extracting info
	// Name patterns: "I'm John", "My name is John Smith", "This is John"
	namePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:I'm|I am|my name is|this is|it's|call me)\s+([A-Z][a-z]+(?:\s+[A-Z][a-z]+)?)`),
		regexp.MustCompile(`(?i)^([A-Z][a-z]+(?:\s+[A-Z][a-z]+)?)[,.]?\s+(?:here|speaking)`),
	}

	// Phone pattern (10 digits with various formats)
	phonePattern := regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?([2-9]\d{2})\)?[-.\s]?(\d{3})[-.\s]?(\d{4})\b`)

	// Phone correction patterns - customer is correcting previously given phone
	// Matches: "my phone is actually X", "phone should be X", "phone is still X",
	// "correct my phone to X", "you got my phone wrong - it's X"
	phoneCorrectionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:phone|number)\s+(?:is\s+)?(?:actually|should\s+be|still)\s+(\d{3}[-.\s]?\d{3}[-.\s]?\d{4})`),
		regexp.MustCompile(`(?i)(?:correct|change|update)\s+(?:my\s+)?(?:phone|number)\s+to\s+(\d{3}[-.\s]?\d{3}[-.\s]?\d{4})`),
		regexp.MustCompile(`(?i)(?:got|have)\s+(?:my\s+)?(?:phone|number)\s+wrong[^0-9]*(\d{3}[-.\s]?\d{3}[-.\s]?\d{4})`),
		regexp.MustCompile(`(?i)(?:it's|its|it\s+is)\s+(?:still\s+)?(\d{3}[-.\s]?\d{3}[-.\s]?\d{4})`),
	}

	// Email pattern
	emailPattern := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)

	// Address pattern (simple: number + street name + optional city/state/zip)
	addressPattern := regexp.MustCompile(`\b(\d+\s+[A-Za-z]+(?:\s+[A-Za-z]+)*(?:\s+(?:St|Street|Ave|Avenue|Rd|Road|Dr|Drive|Ln|Lane|Blvd|Boulevard|Way|Ct|Court|Pl|Place)\.?)?)(?:,?\s+([A-Za-z\s]+),?\s+([A-Z]{2})?\s*(\d{5}(?:-\d{4})?)?)?`)

	// Normalize tenant phone for comparison
	tenantPhoneNorm := normalizePhoneDigits(tenantPhone)

	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}

		content := msg.Content

		// Extract name (only if not already found)
		if info.Name == "" {
			for _, pattern := range namePatterns {
				if match := pattern.FindStringSubmatch(content); len(match) > 1 {
					name := strings.TrimSpace(match[1])
					// Filter out common false positives
					if !isCommonWord(name) && len(name) > 2 {
						info.Name = name
						break
					}
				}
			}
		}

		// Check for phone corrections FIRST (these override any previous phone)
		for _, corrPattern := range phoneCorrectionPatterns {
			if match := corrPattern.FindStringSubmatch(content); len(match) > 1 {
				correctedPhone := match[1]
				phoneNorm := normalizePhoneDigits(correctedPhone)
				if phoneNorm != tenantPhoneNorm && !isPlaceholderPhoneDigits(phoneNorm) {
					info.Phone = correctedPhone // Override with corrected phone
					break
				}
			}
		}

		// Extract phone (only if not already found and not the tenant's phone)
		if info.Phone == "" {
			if match := phonePattern.FindString(content); match != "" {
				phoneNorm := normalizePhoneDigits(match)
				// Don't capture the tenant's own phone number
				if phoneNorm != tenantPhoneNorm && !isPlaceholderPhoneDigits(phoneNorm) {
					info.Phone = match
				}
			}
		}

		// Extract email (only if not already found)
		if info.Email == "" {
			if match := emailPattern.FindString(content); match != "" {
				// Filter out placeholder emails
				if !isPlaceholderEmailCheck(match) {
					info.Email = match
				}
			}
		}

		// Extract address (only if not already found)
		if info.Address == "" {
			if match := addressPattern.FindStringSubmatch(content); len(match) > 1 {
				addr := strings.TrimSpace(match[0])
				if len(addr) > 10 { // Reasonable address length
					info.Address = addr
				}
			}
		}
	}

	return info
}

// normalizePhoneDigits extracts just digits from a phone number.
func normalizePhoneDigits(phone string) string {
	re := regexp.MustCompile(`[^0-9]`)
	digits := re.ReplaceAllString(phone, "")
	// Remove country code if present
	if len(digits) == 11 && digits[0] == '1' {
		digits = digits[1:]
	}
	return digits
}

// isPlaceholderPhoneDigits checks if a normalized phone is a placeholder.
// Fixed: Previously rejected ALL 555-xxx-xxxx phones. Now only rejects
// the official NANPA fictional range 555-01XX (555-0100 to 555-0199).
// See: https://en.wikipedia.org/wiki/555_(telephone_number)
func isPlaceholderPhoneDigits(digits string) bool {
	// Common placeholder patterns (exact match)
	placeholders := []string{"0000000000", "9999999999", "1234567890", "1111111111"}
	for _, p := range placeholders {
		if digits == p {
			return true
		}
	}

	// Only reject the official NANPA fictional range: 555-01XX (555-0100 to 555-0199)
	// Real 555 numbers DO exist outside this range (e.g., 555-1212 for directory assistance)
	if len(digits) == 10 && digits[:3] == "555" {
		middle := digits[3:7]
		if middle >= "0100" && middle <= "0199" {
			return true // Official fictional range
		}
	}

	return false
}

// isPlaceholderEmailCheck checks if an email is a placeholder.
func isPlaceholderEmailCheck(email string) bool {
	lower := strings.ToLower(email)
	placeholders := []string{"example.com", "test.com", "fake.com", "sample.com"}
	for _, p := range placeholders {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// isCommonWord checks if a string is a common word that shouldn't be a name.
func isCommonWord(s string) bool {
	commonWords := map[string]bool{
		"hello": true, "hi": true, "hey": true, "the": true, "a": true,
		"an": true, "is": true, "it": true, "my": true, "your": true,
		"here": true, "there": true, "this": true, "that": true,
		"yes": true, "no": true, "okay": true, "ok": true,
		// Affirmation words that could be falsely extracted as names
		"sure": true, "yeah": true, "yep": true, "right": true,
		"absolutely": true, "certainly": true, "definitely": true,
		"great": true, "perfect": true, "thanks": true, "thank": true,
	}
	return commonWords[strings.ToLower(s)]
}

// ============================================================================
// TICKET GENERATION FOR ESCALATIONS
// ============================================================================

// ticketCounter is used to generate unique ticket numbers within the same second
var ticketCounter int32
var ticketCounterMux sync.Mutex

// GenerateTicketNumber creates a ticket number in format TKT-YYYYMMDD-XXXX
func GenerateTicketNumber() string {
	ticketCounterMux.Lock()
	defer ticketCounterMux.Unlock()

	ticketCounter++
	if ticketCounter > 9999 {
		ticketCounter = 1
	}

	return fmt.Sprintf("TKT-%s-%04d", time.Now().Format("20060102"), ticketCounter)
}

// GenerateComplaintNumber creates a complaint number in format CMP-YYYYMMDD-XXXX
func GenerateComplaintNumber() string {
	ticketCounterMux.Lock()
	defer ticketCounterMux.Unlock()

	ticketCounter++
	if ticketCounter > 9999 {
		ticketCounter = 1
	}

	return fmt.Sprintf("CMP-%s-%04d", time.Now().Format("20060102"), ticketCounter)
}

// EscalationTicketInfo contains information about a created escalation ticket
type EscalationTicketInfo struct {
	TicketNumber  string
	TicketID      int32
	Type          string // "supervisor_request" or "complaint"
	CustomerPhone string
	CustomerEmail string
	CustomerName  string
	Issue         string
}

// CreateEscalationTicket creates a ticket with a linked memo for supervisor request or complaint
func (s *Service) CreateEscalationTicket(ctx context.Context, tenantID int32, ticketType string, customerInfo map[string]string, issue string) (*EscalationTicketInfo, error) {
	// Generate ticket number based on type
	var ticketNumber string
	if ticketType == "complaint" {
		ticketNumber = GenerateComplaintNumber()
	} else {
		ticketNumber = GenerateTicketNumber()
	}

	// Generate unique memo UID for this escalation
	memoUID := "esc-" + uuid.New().String()[:12]

	// Build memo content with all escalation details
	var memoContent strings.Builder
	memoContent.WriteString(fmt.Sprintf("## Escalation Ticket: %s\n\n", ticketNumber))
	memoContent.WriteString(fmt.Sprintf("**Type:** %s\n", ticketType))
	memoContent.WriteString(fmt.Sprintf("**Created:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	memoContent.WriteString("### Customer Information\n\n")
	if name, ok := customerInfo["name"]; ok && name != "" {
		memoContent.WriteString(fmt.Sprintf("- **Name:** %s\n", name))
	}
	if phone, ok := customerInfo["phone"]; ok && phone != "" {
		memoContent.WriteString(fmt.Sprintf("- **Phone:** %s\n", phone))
	}
	if email, ok := customerInfo["email"]; ok && email != "" {
		memoContent.WriteString(fmt.Sprintf("- **Email:** %s\n", email))
	}

	if issue != "" {
		memoContent.WriteString("\n### Issue Summary\n\n")
		memoContent.WriteString(issue)
		memoContent.WriteString("\n")
	}

	// Create the memo with Protected visibility (visible to logged-in users)
	memo := &store.Memo{
		UID:        memoUID,
		CreatorID:  1, // System user
		Content:    memoContent.String(),
		Visibility: store.Protected,
	}

	createdMemo, err := s.store.CreateMemo(ctx, memo)
	if err != nil {
		slog.Error("failed to create escalation memo", "error", err, "ticket_number", ticketNumber)
		// Fall back to old behavior if memo creation fails
		return s.createEscalationTicketFallback(ctx, ticketNumber, ticketType, customerInfo, issue)
	}

	// Determine priority
	priority := store.TicketPriorityMedium
	if ticketType == "complaint" {
		priority = store.TicketPriorityHigh
	}

	// Create ticket with ONLY the memo link in description
	now := time.Now().Unix()
	ticket := &store.Ticket{
		Title:       fmt.Sprintf("[%s] Agent Escalation - %s", ticketNumber, ticketType),
		Description: "/m/" + createdMemo.UID, // Only the memo link
		Status:      store.TicketStatusOpen,
		Priority:    priority,
		CreatorID:   1, // System user for agent-created tickets
		CreatedTs:   now,
		UpdatedTs:   now,
		Type:        "agent_escalation",
	}

	created, err := s.store.CreateTicket(ctx, ticket)
	if err != nil {
		slog.Error("failed to create escalation ticket", "error", err, "ticket_number", ticketNumber)
		return nil, err
	}

	slog.Info("escalation ticket created with memo", "ticket", ticketNumber, "memo_uid", createdMemo.UID)

	return &EscalationTicketInfo{
		TicketNumber:  ticketNumber,
		TicketID:      created.ID,
		Type:          ticketType,
		CustomerPhone: customerInfo["phone"],
		CustomerEmail: customerInfo["email"],
		CustomerName:  customerInfo["name"],
		Issue:         issue,
	}, nil
}

// createEscalationTicketFallback creates a ticket without memo (legacy fallback)
func (s *Service) createEscalationTicketFallback(ctx context.Context, ticketNumber, ticketType string, customerInfo map[string]string, issue string) (*EscalationTicketInfo, error) {
	// Build description with embedded content (fallback)
	description := fmt.Sprintf("/m/agent-escalation\n\nTicket: %s\nType: %s\n", ticketNumber, ticketType)
	if name, ok := customerInfo["name"]; ok && name != "" {
		description += fmt.Sprintf("Customer: %s\n", name)
	}
	if phone, ok := customerInfo["phone"]; ok && phone != "" {
		description += fmt.Sprintf("Phone: %s\n", phone)
	}
	if email, ok := customerInfo["email"]; ok && email != "" {
		description += fmt.Sprintf("Email: %s\n", email)
	}
	if issue != "" {
		description += fmt.Sprintf("\nIssue: %s\n", issue)
	}

	priority := store.TicketPriorityMedium
	if ticketType == "complaint" {
		priority = store.TicketPriorityHigh
	}

	now := time.Now().Unix()
	ticket := &store.Ticket{
		Title:       fmt.Sprintf("[%s] Agent Escalation - %s", ticketNumber, ticketType),
		Description: description,
		Status:      store.TicketStatusOpen,
		Priority:    priority,
		CreatorID:   1,
		CreatedTs:   now,
		UpdatedTs:   now,
		Type:        "agent_escalation",
	}

	created, err := s.store.CreateTicket(ctx, ticket)
	if err != nil {
		return nil, err
	}

	return &EscalationTicketInfo{
		TicketNumber:  ticketNumber,
		TicketID:      created.ID,
		Type:          ticketType,
		CustomerPhone: customerInfo["phone"],
		CustomerEmail: customerInfo["email"],
		CustomerName:  customerInfo["name"],
		Issue:         issue,
	}, nil
}

// ============================================================================
// SESSION STATE TRACKING
// ============================================================================

// IncrementOutOfCoverageCount increments and returns the out-of-coverage counter
func IncrementOutOfCoverageCount(session *store.AgentSession) int {
	session.OutOfCoverageCount++
	return session.OutOfCoverageCount
}

// IsSafetyGiven checks if full safety instructions have been given
func IsSafetyGiven(session *store.AgentSession) bool {
	return session.SafetyGiven
}

// MarkSafetyGiven marks that full safety instructions have been given
func MarkSafetyGiven(session *store.AgentSession) {
	session.SafetyGiven = true
}

// GetEscalationTicket retrieves the escalation ticket number if one was created
func GetEscalationTicket(session *store.AgentSession) string {
	return session.EscalationTicket
}

// SetEscalationTicket stores the escalation ticket number in session
func SetEscalationTicket(session *store.AgentSession, ticketNumber string) {
	session.EscalationTicket = ticketNumber
}

// ============================================================================
// AUTO-GENERATE ANNOTATED KB.MD / POLICY.MD
// ============================================================================

// getReasoningModel returns the LLM model for reasoning tasks.
// Priority: tenant config > LLM_MODEL_REASONING env var > hardcoded default.
func (s *Service) getReasoningModel(ctx context.Context, tenantID int32) string {
	// 1. Try tenant-specific config
	config, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID})
	if config != nil && config.ReasoningModel != "" {
		return config.ReasoningModel
	}

	// 2. Fallback to env var
	model := os.Getenv("LLM_MODEL_REASONING")
	if model != "" {
		return model
	}

	// 3. Hardcoded default
	return "google/gemini-2.5-pro"
}

// GenerateAnnotatedKB uses an LLM to convert raw KB content into properly annotated KB.MD format.
func (s *Service) GenerateAnnotatedKB(ctx context.Context, tenantID int32, companyName, rawContent string) (string, error) {
	_, apiKey := s.getLLMConfig(ctx, tenantID)
	if apiKey == "" {
		return "", fmt.Errorf("OpenRouter API key not configured")
	}

	model := s.getReasoningModel(ctx, tenantID)

	prompt := buildKBGenerationPrompt(companyName, rawContent)

	client := newOpenRouterClient(apiKey)
	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model: model,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage("You are a technical writer that creates structured knowledge base documents. Output ONLY the formatted KB.MD content with no explanations or commentary."),
			openrouter.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content.Text, nil
}

// GenerateAnnotatedPolicy uses an LLM to convert raw Policy content into properly annotated POLICY.MD format.
func (s *Service) GenerateAnnotatedPolicy(ctx context.Context, tenantID int32, companyName, rawContent string) (string, error) {
	_, apiKey := s.getLLMConfig(ctx, tenantID)
	if apiKey == "" {
		return "", fmt.Errorf("OpenRouter API key not configured")
	}

	model := s.getReasoningModel(ctx, tenantID)

	prompt := buildPolicyGenerationPrompt(companyName, rawContent)

	client := newOpenRouterClient(apiKey)
	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model: model,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage("You are a technical writer that creates structured policy documents. Output ONLY the formatted POLICY.MD content with no explanations or commentary."),
			openrouter.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content.Text, nil
}

// CallLLMSimple makes a simple LLM call with a system prompt and user message.
// This is a helper for handlers that need to make LLM calls.
func (s *Service) CallLLMSimple(ctx context.Context, tenantID int32, systemPrompt, userMessage string) (string, error) {
	model, apiKey := s.getLLMConfig(ctx, tenantID)
	if apiKey == "" {
		return "", fmt.Errorf("no API key configured")
	}

	client := newOpenRouterClient(apiKey)
	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model: model,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage(systemPrompt),
			openrouter.UserMessage(userMessage),
		},
	})
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content.Text, nil
}

// SearchVectorDB performs a direct vector search for testing/evaluation purposes.
// Returns nil if RAG is not enabled.
func (s *Service) SearchVectorDB(ctx context.Context, tenantID int32, audienceType, query string, topK int) (*SearchResult, error) {
	if s.vectorDB == nil {
		return nil, fmt.Errorf("RAG pipeline not enabled")
	}

	if topK <= 0 {
		topK = 5
	}

	return s.vectorDB.Search(ctx, SearchQuery{
		TenantID:     tenantID,
		AudienceType: audienceType,
		QueryText:    query,
		TopK:         topK,
	})
}

// buildKBGenerationPrompt constructs the prompt for KB.MD generation.
func buildKBGenerationPrompt(companyName, rawContent string) string {
	return fmt.Sprintf(`Analyze the following raw content and generate a properly formatted KB.MD file using HTML comment annotations.

## Company Name
%s

## Raw Content to Analyze
%s

## Required Output Format

Generate a KB.MD file with these annotation types:

1. **@section** - For general content sections
   Format: <!-- @section: code, type: category -->
   Example: <!-- @section: about_us, type: general -->

2. **@service** - For services/products offered
   Format: <!-- @service: code, emergency: true/false -->
   Example: <!-- @service: water_damage, emergency: true -->

3. **@faq** - For Q&A pairs
   Format: <!-- @faq: code -->
   Example: <!-- @faq: response_time -->

4. **@exclusion** - For things NOT offered
   Format: <!-- @exclusion: code, exception: "when applicable" -->
   Example: <!-- @exclusion: general_plumbing, exception: "unless it caused water damage" -->

5. **@coverage** - For geographic/scope areas
   Format: <!-- @coverage: include/exclude -->
   Example: <!-- @coverage: include -->

6. **@safety** - For safety protocols
   Format: <!-- @safety: code, triggers: intent1, intent2 -->
   Example: <!-- @safety: gas_leak, triggers: emergency_gas, smell_gas -->

## Rules

1. Each annotation must have a unique code (lowercase_snake_case)
2. Group related content under appropriate headings (## for main sections, ### for items)
3. Identify and extract ALL FAQs from the content (look for Q&A patterns, common questions)
4. Identify services/products if applicable
5. Create custom @section types for content that doesn't fit other categories
6. Use clear, descriptive titles
7. Maintain the original content's meaning - do not invent information
8. Add section separators (---) between major sections

## Output

Return ONLY the formatted KB.MD content, starting with:
# %s Knowledge Base

Do not include any explanations or commentary before or after the content.`, companyName, rawContent, companyName)
}

// buildPolicyGenerationPrompt constructs the prompt for POLICY.MD generation.
func buildPolicyGenerationPrompt(companyName, rawContent string) string {
	return fmt.Sprintf(`Analyze the following raw content and generate a properly formatted POLICY.MD file using HTML comment annotations.

## Company Name
%s

## Raw Content to Analyze
%s

## Required Output Format

Generate a POLICY.MD file with these annotation types:

1. **@identity** - Agent identity definition
   Format: <!-- @identity: agent -->
   Place at the start of the identity section

2. **@intent** - User intent classification
   Format: <!-- @intent: code, category: emergency|standard|meta, urgency: 0-5 -->
   Example: <!-- @intent: report_water_damage, category: emergency, urgency: 5 -->

3. **@rule** - Policy rules
   Format: <!-- @rule: code, priority: 1-10 -->
   Example: <!-- @rule: greeting, priority: 1 -->

4. **@thresholds** - Escalation thresholds
   Format: <!-- @thresholds: escalation -->
   Example: <!-- @thresholds: escalation -->

## Rules

1. Each annotation must have a unique code (lowercase_snake_case)
2. Identify the agent's role, tone, and brand voice for the @identity section
3. Extract any conversation guidelines or rules
4. Identify possible user intents from the content
5. Look for escalation criteria or thresholds
6. Group related content under appropriate headings
7. Maintain the original content's meaning - do not invent information
8. Add section separators (---) between major sections

## Output

Return ONLY the formatted POLICY.MD content, starting with:
# %s Policy

Do not include any explanations or commentary before or after the content.`, companyName, rawContent, companyName)
}

// shouldRecordTranscript checks if transcript recording is enabled for the tenant.
func (s *Service) shouldRecordTranscript(ctx context.Context, tenantID int32) bool {
	config, err := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID})
	if err != nil || config == nil {
		return true // Default to recording if config not found
	}
	return config.RecordTranscripts
}

// saveTranscript persists the chat session to the transcripts table.
func (s *Service) saveTranscript(ctx context.Context, session *store.AgentSession, clientIP, userAgent string) error {
	transcript := &store.AgentTranscript{
		ID:               session.ID,
		TenantID:         session.TenantID,
		SessionID:        session.ID,
		AudienceType:     session.AudienceType,
		Messages:         session.Messages,
		MessageCount:     session.MessageCount,
		ClientIP:         clientIP,
		UserAgent:        userAgent,
		CustomerName:     session.CustomerName,
		CustomerPhone:    session.CustomerPhone,
		CustomerLocation: session.CustomerLocation,
		DetectedIntent:   session.CurrentIntent,
		StartedAt:        session.CreatedAt,
		LastMessageAt:    time.Now(),
		IsCompleted:      session.IsCompleted,
		CompletionReason: session.CompletionReason,
	}

	// Check if transcript already exists (upsert logic)
	existing, err := s.store.GetAgentTranscript(ctx, &store.FindAgentTranscript{SessionID: &session.ID})
	if err != nil {
		slog.Warn("Failed to check existing transcript", "sessionID", session.ID, "error", err)
	}

	if existing != nil {
		// Update existing transcript
		return s.store.UpdateAgentTranscript(ctx, transcript)
	}

	// Create new transcript
	_, err = s.store.CreateAgentTranscript(ctx, transcript)
	return err
}

// ProcessTicketChat processes a support ticket message using the advanced agent pipelines.
func (s *Service) ProcessTicketChat(ctx context.Context, tenantSlug string, history []store.AgentMessage, latestMessage string) (string, error) {
	// Load config - use internal audience
	config, err := s.LoadConfig(ctx, tenantSlug, "internal")
	if err != nil {
		return "", err
	}

	// Create a temporary mock session
	session := &store.AgentSession{
		ID:             uuid.New().String(),
		TenantID:       config.TenantID,
		AudienceType:   "internal",
		Phase:          "triage",
		UrgencyLevel:   0,
		CoverageStatus: "unknown",
		MessageCount:   len(history),
		Messages:       history,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Process chat using the standard pipeline
	response, err := s.processChat(ctx, config, session, latestMessage)
	if err != nil {
		return "", err
	}

	return response.Message.Content, nil
}

type BridgeTakeoverRequest struct {
	SessionID string `json:"session_id"`
}

type BridgeTakeoverResponse struct {
	Status    string               `json:"status"`
	HandoffID string               `json:"handoff_id,omitempty"`
	Handoff   *store.BridgeHandoff `json:"handoff,omitempty"`
}

type BridgeReplyRequest struct {
	SessionID string `json:"session_id"`
	HandoffID string `json:"handoff_id"`
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

type BridgeReplyResponse struct {
	Status string `json:"status"`
}

type BridgeReleaseRequest struct {
	SessionID string  `json:"session_id"`
	HandoffID string  `json:"handoff_id"`
	Reason    *string `json:"reason,omitempty"`
}

type BridgeReleaseResponse struct {
	Status string `json:"status"`
}
