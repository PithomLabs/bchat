package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/usememos/memos/store"
)

// processPendingTickets periodically embeds pending tickets and builds clusters.
func (s *Service) processPendingTickets(ctx context.Context) {
	s.vectorDBMu.RLock()
	vectorDB := s.vectorDB
	s.vectorDBMu.RUnlock()

	if vectorDB == nil {
		slog.Warn("VectorDB not available, skipping ticket embedding")
		return
	}

	// Create embedding service once (reused across all tenants)
	embedSvc, err := NewEmbeddingService(s.vectorDBConfig.EmbeddingConfig)
	if err != nil {
		slog.Error("Failed to create embedding service", "error", err)
		return
	}

	// Fetch all tenants (DB-level filter)
	isActive := true
	tenants, err := s.store.ListAgentTenants(ctx, &store.FindAgentTenant{IsActive: &isActive})
	if err != nil {
		slog.Error("Failed to list tenants for ticket embedding", "error", err)
		return
	}

	for _, tenant := range tenants {
		embedErr := s.embedTenantTickets(ctx, vectorDB, tenant.ID, embedSvc)
		if embedErr != nil {
			slog.Error("Failed to embed tickets for tenant",
				"tenant_id", tenant.ID,
				"slug", tenant.Slug,
				"error", embedErr)
		}

		// Run clustering (topological sort)
		clusterErr := s.buildTicketClusters(ctx, vectorDB, tenant.ID)
		if clusterErr != nil {
			slog.Error("Failed to cluster tickets for tenant",
				"tenant_id", tenant.ID,
				"slug", tenant.Slug,
				"error", clusterErr)
		}
	}
}

// embedTenantTickets fetches unembedded tickets and upserts into vector DB.
func (s *Service) embedTenantTickets(ctx context.Context, vectorDB VectorDB, tenantID int32, embedSvc EmbeddingService) error {
	// Fetch tickets without embeddings
	finding := &store.FindTicket{TenantID: &tenantID}
	tickets, err := s.store.ListTickets(ctx, finding)
	if err != nil {
		return fmt.Errorf("failed to list tickets: %w", err)
	}

	if len(tickets) == 0 {
		return nil
	}

	// Check which tickets already have embeddings
	existingChunks, err := vectorDB.ListChunks(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to list existing chunks: %w", err)
	}

	embeddedIDs := make(map[string]bool)
	for _, chunk := range existingChunks {
		if chunk.ContentType == "ticket" {
			embeddedIDs[chunk.ID] = true
		}
	}

	// Filter to unembedded tickets
	var toEmbed []*store.Ticket
	for _, ticket := range tickets {
		ticketID := fmt.Sprintf("ticket_%d", ticket.ID)
		if !embeddedIDs[ticketID] {
			toEmbed = append(toEmbed, ticket)
		}
	}

	if len(toEmbed) == 0 {
		return nil
	}

	// Create chunks for embedding
	chunks := make([]DocumentChunk, len(toEmbed))
	for i, ticket := range toEmbed {
		content := fmt.Sprintf("%s\n%s\n%s", ticket.Title, ticket.Description, ticket.InternalNotes)
		chunks[i] = DocumentChunk{
			ID:          fmt.Sprintf("ticket_%d", ticket.ID),
			TenantID:    tenantID,
			ContentType: "ticket",
			Title:       ticket.Title,
			Content:     content,
			IsActive:    true,
			IndexedAt:   time.Now(),
		}
	}

	// Generate embeddings via cached EmbeddingService
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)
	}

	embedResults, err := embedSvc.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	for i := range chunks {
		chunks[i].Embedding = embedResults[i]
	}

	// Upsert into vector DB
	err = vectorDB.Insert(ctx, chunks)
	if err != nil {
		return fmt.Errorf("failed to insert tickets: %w", err)
	}

	slog.Info("Embedded tickets",
		"count", len(chunks),
		"tenant_id", tenantID)

	return nil
}

// buildTicketClusters performs topological sort on embedded tickets and stores the result.
func (s *Service) buildTicketClusters(ctx context.Context, vectorDB VectorDB, tenantID int32) error {
	// Fetch all ticket chunks for this tenant
	chunks, err := vectorDB.ListChunks(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to list chunks: %w", err)
	}

	// Filter to ticket chunks only
	var ticketChunks []DocumentChunk
	for _, chunk := range chunks {
		if chunk.ContentType == "ticket" {
			ticketChunks = append(ticketChunks, chunk)
		}
	}

	if len(ticketChunks) < 2 {
		return nil
	}

	// Build adjacency list based on embedding similarity (cosine > 0.7)
	adjacency := make(map[string][]string)
	for i, chunkA := range ticketChunks {
		for j, chunkB := range ticketChunks {
			if i >= j {
				continue
			}
			sim := cosineSimilarity(chunkA.Embedding, chunkB.Embedding)
			if sim > 0.7 {
				adjacency[chunkA.ID] = append(adjacency[chunkA.ID], chunkB.ID)
			}
		}
	}

	// Topological sort (Kahn's algorithm)
	inDegree := make(map[string]int)
	for _, chunk := range ticketChunks {
		inDegree[chunk.ID] = 0
	}
	for _, neighbors := range adjacency {
		for _, neighbor := range neighbors {
			inDegree[neighbor]++
		}
	}

	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		// Pop front
		current := queue[0]
		queue = queue[1:]
		sorted = append(sorted, current)

		for _, neighbor := range adjacency[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// Store the sorted cluster as metadata
	clusterData := map[string]interface{}{
		"sorted_tickets": sorted,
		"created_at":     time.Now().Format(time.RFC3339),
	}

	clusterJSON, err := json.Marshal(clusterData)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster: %w", err)
	}

	// Upsert cluster metadata as a chunk
	clusterChunk := DocumentChunk{
		ID:          fmt.Sprintf("cluster_%d_%s", tenantID, uuid.New().String()[:8]),
		TenantID:    tenantID,
		ContentType: "cluster",
		Title:       "Ticket Cluster",
		Content:     string(clusterJSON),
		IsActive:    true,
		IndexedAt:   time.Now(),
	}

	err = vectorDB.Insert(ctx, []DocumentChunk{clusterChunk})
	if err != nil {
		return fmt.Errorf("failed to store cluster: %w", err)
	}

	slog.Info("Built ticket cluster",
		"tenant_id", tenantID,
		"ticket_count", len(sorted))

	return nil
}
