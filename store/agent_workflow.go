package store

import (
	"context"
)

// AgentWorkflow represents a logged agent task boundary event
// This provides durable storage of agent thoughts, processes, and workflow steps
type AgentWorkflow struct {
	ID            int32
	TicketID      int32
	SessionID     string
	AgentName     string
	TaskName      string
	TaskMode      string // PLANNING, EXECUTION, VERIFICATION
	TaskStatus    string
	TaskSummary   string
	PredictedSize int32
	CreatedTs     int64
	Metadata      string // JSON for additional context
}

type FindAgentWorkflow struct {
	ID        *int32
	TicketID  *int32
	SessionID *string
}

type CreateAgentWorkflow struct {
	TicketID      int32
	SessionID     string
	AgentName     string
	TaskName      string
	TaskMode      string
	TaskStatus    string
	TaskSummary   string
	PredictedSize int32
	CreatedTs     int64
	Metadata      string
}

type AgentWorkflowStore interface {
	CreateAgentWorkflow(ctx context.Context, create *CreateAgentWorkflow) (*AgentWorkflow, error)
	ListAgentWorkflows(ctx context.Context, find *FindAgentWorkflow) ([]*AgentWorkflow, error)
	GetAgentWorkflow(ctx context.Context, find *FindAgentWorkflow) (*AgentWorkflow, error)
}

// TODO: Implement Driver interface methods for AgentWorkflow
// Currently commented out to unblock compilation - needs proper implementation

func (s *Store) CreateAgentWorkflow(ctx context.Context, create *CreateAgentWorkflow) (*AgentWorkflow, error) {
	// TODO: Restore after implementing Driver methods
	// return s.driver.CreateAgentWorkflow(ctx, create)
	return nil, nil
}

func (s *Store) ListAgentWorkflows(ctx context.Context, find *FindAgentWorkflow) ([]*AgentWorkflow, error) {
	// TODO: Restore after implementing Driver methods
	// return s.driver.ListAgentWorkflows(ctx, find)
	return nil, nil
}

func (s *Store) GetAgentWorkflow(ctx context.Context, find *FindAgentWorkflow) (*AgentWorkflow, error) {
	// TODO: Restore after implementing Driver methods
	// return s.driver.GetAgentWorkflow(ctx, find)
	return nil, nil
}
