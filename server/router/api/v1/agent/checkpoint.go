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

// claimExecution attempts to claim a pending execution with optimistic locking.
// Returns the claimed execution or an error if the claim fails.
func (s *Service) claimExecution(ctx context.Context, execID string, workerID string) (*store.SkillExecution, error) {
	leaseSeconds := 300 // 5 minute lease
	exec, err := s.store.ClaimSkillExecution(ctx, execID, workerID, leaseSeconds)
	if err != nil {
		return nil, fmt.Errorf("claim execution %s: %w", execID, err)
	}
	slog.Debug("claimed execution", "exec_id", execID, "worker", workerID)
	return exec, nil
}

// writeCheckpoint persists execution state after each step.
// Re-reads status before writing to detect if someone stopped the execution.
func (s *Service) writeCheckpoint(ctx context.Context, exec *store.SkillExecution, state map[string]any, nodeName string) error {
	// R3: Status re-read before checkpoint write
	current, err := s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if err != nil {
		return fmt.Errorf("read status before checkpoint: %w", err)
	}
	if current == nil {
		return fmt.Errorf("execution %s not found", exec.ID)
	}
	if current.Status != "running" {
		return fmt.Errorf("execution %s is %s, not running — aborting checkpoint", exec.ID, current.Status)
	}

	exec.CheckpointData = state
	exec.CurrentNode = nodeName
	exec.UpdatedAt = time.Now()

	return s.store.UpdateSkillExecution(ctx, exec)
}

// completeExecution marks an execution as completed with the final output.
func (s *Service) completeExecution(ctx context.Context, exec *store.SkillExecution, output string) error {
	if err := s.store.CompleteSkillExecution(ctx, exec.ID); err != nil {
		return fmt.Errorf("complete execution: %w", err)
	}

	slog.Info("execution completed",
		"exec_id", exec.ID,
		"tenant_id", exec.TenantID,
		"output_len", len(output))

	// D6: Dispatch outbound event
	if exec.TenantID != nil {
		leadID := ""
		if exec.ConversationID != "" {
			leadID = exec.ConversationID
		}
		s.dispatchEvent(ctx, *exec.TenantID, leadID, "workflow.completed", output)
	}

	return nil
}

// failExecution marks an execution as failed with the error message.
func (s *Service) failExecution(ctx context.Context, exec *store.SkillExecution, errMsg string) error {
	return s.store.FailSkillExecution(ctx, exec.ID, errMsg)
}

// stopExecution marks an execution as stopped (user-initiated cancellation).
func (s *Service) stopExecution(ctx context.Context, execID string) error {
	return s.store.StopSkillExecution(ctx, execID)
}

// createExecution creates a new execution record in pending state.
func (s *Service) createExecution(ctx context.Context, tenantID *int32, conversationID, triggerPath string, graph *SkillGraph, maxRetries int) (*store.SkillExecution, error) {
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		return nil, fmt.Errorf("marshal skill graph: %w", err)
	}

	now := time.Now()
	exec := &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		ConversationID: conversationID,
		SkillGraphJSON: string(graphJSON),
		Status:         "pending",
		TriggerPath:    triggerPath,
		CurrentNode:    "",
		CheckpointData: make(map[string]any),
		CompletedNodes: make(map[string]any),
		FailedNodes:    make(map[string]any),
		RetryCount:     0,
		MaxRetries:     maxRetries,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	return s.store.CreateSkillExecution(ctx, exec)
}

// getExecution retrieves an execution by ID.
func (s *Service) getExecution(ctx context.Context, execID string) (*store.SkillExecution, error) {
	return s.store.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &execID})
}

// listExecutionsByTenant lists executions for a tenant with optional status filter.
func (s *Service) listExecutionsByTenant(ctx context.Context, tenantID int32, status string, limit int) ([]*store.SkillExecution, error) {
	find := &store.FindSkillExecution{
		TenantID: &tenantID,
	}
	if status != "" {
		find.Status = &status
	}
	return s.store.ListSkillExecutions(ctx, find, limit)
}
