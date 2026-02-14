package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

// ObservationIndexer handles indexing observations to the RAG system.
type ObservationIndexer struct {
	vectorDB    VectorDB
	bufferSize  int  // Number of observations to buffer before indexing
	compression bool // Enable compression for storage
	ttlDays     int  // TTL for observations in days (0 = no TTL)
	enableTTL   bool // Enable TTL enforcement
}

// NewObservationIndexer creates a new observation indexer.
func NewObservationIndexer(vectorDB VectorDB) *ObservationIndexer {
	return &ObservationIndexer{
		vectorDB:   vectorDB,
		bufferSize: 10, // Default buffer size
	}
}

// NewObservationIndexerWithConfig creates a new observation indexer with full configuration.
func NewObservationIndexerWithConfig(vectorDB VectorDB, compression bool, ttlDays int) *ObservationIndexer {
	return &ObservationIndexer{
		vectorDB:    vectorDB,
		bufferSize:  10,
		compression: compression,
		ttlDays:     ttlDays,
		enableTTL:   ttlDays > 0,
	}
}

// IndexObservation indexes a single observation to the RAG system.
// This enables unlimited memory by storing observations in the vector database.
func (oi *ObservationIndexer) IndexObservation(
	ctx context.Context,
	obsLog *store.ObservationLog,
	tenantID int32,
) error {
	if obsLog == nil || obsLog.ObservationLog == "" {
		return nil
	}

	// Parse observations into chunks
	chunks := oi.parseObservationsToChunks(obsLog, tenantID)
	if len(chunks) == 0 {
		return nil
	}

	// Apply compression if enabled
	if oi.compression {
		chunks = oi.compressChunks(chunks)
	}

	// Insert chunks into vector database
	if err := oi.vectorDB.Insert(ctx, chunks); err != nil {
		return fmt.Errorf("failed to index observations: %w", err)
	}

	slog.Info("Indexed observations to RAG",
		"session_id", obsLog.SessionID,
		"tenant_id", tenantID,
		"chunks", len(chunks),
		"compression", oi.compression)

	return nil
}

// parseObservationsToChunks parses the observation log into document chunks.
func (oi *ObservationIndexer) parseObservationsToChunks(obsLog *store.ObservationLog, tenantID int32) []DocumentChunk {
	chunks := make([]DocumentChunk, 0)

	// Parse observation log format:
	// Date: YYYY-MM-DD
	// - [PRIORITY] HH:MM Content
	//   - [PRIORITY] HH:MM Sub-content

	lines := strings.Split(obsLog.ObservationLog, "\n")
	var currentDate time.Time
	var currentPriority string
	var chunkIndex int

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for date header
		if strings.HasPrefix(line, "Date: ") {
			dateStr := strings.TrimPrefix(line, "Date: ")
			parsed, err := time.Parse("2006-01-02", dateStr)
			if err == nil {
				currentDate = parsed
			}
			continue
		}

		// Check for observation line
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "  - ") {
			content := strings.TrimPrefix(line, "- ")
			content = strings.TrimPrefix(content, "  - ")

			// Extract priority
			priority := "medium"
			if strings.Contains(content, "red_circle") || strings.Contains(content, "high") {
				priority = "high"
				currentPriority = "high"
			} else if strings.Contains(content, "yellow_circle") || strings.Contains(content, "medium") {
				priority = "medium"
				currentPriority = "medium"
			} else if strings.Contains(content, "green_circle") || strings.Contains(content, "low") {
				priority = "low"
				currentPriority = "low"
			} else if currentPriority != "" {
				priority = currentPriority
			}

			// Clean content
			content = cleanObservationContent(content)

			if content != "" && len(content) > 10 { // Skip very short observations
				chunkID := fmt.Sprintf("%d:observation:%s:%d", tenantID, obsLog.SessionID, chunkIndex)

				chunk := DocumentChunk{
					ID:           chunkID,
					TenantID:     tenantID,
					AudienceType: "internal", // Observations are internal
					ContentType:  "observation",
					Title:        fmt.Sprintf("Observation (%s)", priority),
					Content:      content,
					Code:         fmt.Sprintf("obs_%d", chunkIndex),
					IsActive:     true,
					Priority:     oi.priorityToInt(priority),
					IndexedAt:    currentDate,
				}
				chunks = append(chunks, chunk)
				chunkIndex++
			}
		}
	}

	return chunks
}

// priorityToInt converts priority string to int.
func (oi *ObservationIndexer) priorityToInt(priority string) int32 {
	switch priority {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 2
	}
}

// SearchObservations searches for relevant observations in the RAG system.
func (oi *ObservationIndexer) SearchObservations(
	ctx context.Context,
	query string,
	tenantID int32,
	maxResults int,
) (*SearchResult, error) {
	searchQuery := SearchQuery{
		QueryText:            query,
		TenantID:             tenantID,
		AudienceType:         "internal",
		ContentTypes:         []string{"observation"},
		ActiveOnly:           true,
		TopK:                 maxResults,
		MinScore:             0.1,
		UseHybridSearch:      true,
		VectorWeight:         0.7,
		TextWeight:           0.3,
		UseTemporalWeighting: true,
		ReferenceTime:        time.Now(),
		TemporalDecay:        0.1,
	}

	return oi.vectorDB.Search(ctx, searchQuery)
}

// DeleteObservations removes observations for a session from the RAG index.
func (oi *ObservationIndexer) DeleteObservations(
	ctx context.Context,
	tenantID int32,
	sessionID string,
) error {
	// Build the ID prefix for this session's observations
	// Format: "{tenantID}:observation:{sessionID}:"
	idPrefix := fmt.Sprintf("%d:observation:%s:", tenantID, sessionID)

	// Use the new DeleteByIDPrefix method
	deletedCount, err := oi.vectorDB.DeleteByIDPrefix(ctx, tenantID, idPrefix)
	if err != nil {
		return fmt.Errorf("failed to delete observations from RAG index: %w", err)
	}

	slog.Info("Deleted observations from RAG index",
		"tenant_id", tenantID,
		"session_id", sessionID,
		"deleted_count", deletedCount)

	return nil
}

// GetObservationStats returns statistics about indexed observations.
func (oi *ObservationIndexer) GetObservationStats(ctx context.Context, tenantID int32) (int64, error) {
	// Count observation chunks
	chunks, err := oi.vectorDB.ListChunks(ctx, tenantID)
	if err != nil {
		return 0, err
	}

	var observationCount int64
	for _, chunk := range chunks {
		if chunk.ContentType == "observation" {
			observationCount++
		}
	}

	return observationCount, nil
}

// compressChunks applies compression to observation chunks for storage optimization.
// This reduces storage size by consolidating similar observations.
func (oi *ObservationIndexer) compressChunks(chunks []DocumentChunk) []DocumentChunk {
	if len(chunks) <= 1 {
		return chunks
	}

	// Group chunks by priority for potential consolidation
	highPriority := make([]DocumentChunk, 0)
	mediumPriority := make([]DocumentChunk, 0)
	lowPriority := make([]DocumentChunk, 0)

	for _, chunk := range chunks {
		switch chunk.Priority {
		case 3:
			highPriority = append(highPriority, chunk)
		case 1:
			lowPriority = append(lowPriority, chunk)
		default:
			mediumPriority = append(mediumPriority, chunk)
		}
	}

	// For now, we keep all chunks but could implement consolidation here
	// Future: merge similar low-priority observations into single chunks
	result := make([]DocumentChunk, 0, len(chunks))
	result = append(result, highPriority...)
	result = append(result, mediumPriority...)
	result = append(result, lowPriority...)

	return result
}

// EnforceTTL removes observations older than the configured TTL.
// This should be called periodically (e.g., daily) to clean up old observations.
func (oi *ObservationIndexer) EnforceTTL(ctx context.Context, tenantID int32) (int64, error) {
	if !oi.enableTTL || oi.ttlDays <= 0 {
		return 0, nil // TTL not enabled
	}

	// Get all observation chunks
	chunks, err := oi.vectorDB.ListChunks(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to list chunks for TTL enforcement: %w", err)
	}

	cutoffDate := time.Now().AddDate(0, 0, -oi.ttlDays)
	var deletedCount int64

	for _, chunk := range chunks {
		if chunk.ContentType != "observation" {
			continue
		}

		// Check if observation is older than TTL
		if !chunk.IndexedAt.IsZero() && chunk.IndexedAt.Before(cutoffDate) {
			// Delete by ID prefix (session-level deletion)
			// Extract session ID from chunk ID: "{tenantID}:observation:{sessionID}:{index}"
			parts := strings.Split(chunk.ID, ":")
			if len(parts) >= 3 {
				sessionID := parts[2]
				idPrefix := fmt.Sprintf("%d:observation:%s:", tenantID, sessionID)
				_, err := oi.vectorDB.DeleteByIDPrefix(ctx, tenantID, idPrefix)
				if err != nil {
					slog.Error("Failed to delete expired observation",
						"chunk_id", chunk.ID,
						"session_id", sessionID,
						"error", err)
					continue
				}
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		slog.Info("TTL enforcement completed",
			"tenant_id", tenantID,
			"ttl_days", oi.ttlDays,
			"deleted_count", deletedCount,
			"cutoff_date", cutoffDate.Format("2006-01-02"))
	}

	return deletedCount, nil
}

// IndexReflectorObservations indexes consolidated observations from the Reflector.
// This is called after the Reflector compresses the observation log.
func (oi *ObservationIndexer) IndexReflectorObservations(
	ctx context.Context,
	obsLog *store.ObservationLog,
	tenantID int32,
	sessionID string,
) error {
	if obsLog == nil || obsLog.ObservationLog == "" {
		return nil
	}

	// First, delete old observations for this session (reflector replaces them)
	idPrefix := fmt.Sprintf("%d:observation:%s:", tenantID, sessionID)
	oi.vectorDB.DeleteByIDPrefix(ctx, tenantID, idPrefix)

	// Parse and index the consolidated observations
	chunks := oi.parseObservationsToChunks(obsLog, tenantID)
	if len(chunks) == 0 {
		return nil
	}

	// Apply compression if enabled
	if oi.compression {
		chunks = oi.compressChunks(chunks)
	}

	// Mark as consolidated (from reflector)
	for i := range chunks {
		chunks[i].Code = fmt.Sprintf("consolidated_%d", i)
	}

	// Insert chunks into vector database
	if err := oi.vectorDB.Insert(ctx, chunks); err != nil {
		return fmt.Errorf("failed to index reflector observations: %w", err)
	}

	slog.Info("Indexed reflector observations to RAG",
		"session_id", sessionID,
		"tenant_id", tenantID,
		"chunks", len(chunks))

	return nil
}
