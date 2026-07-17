package mysql

import (
	"context"
	"errors"

	"github.com/usememos/memos/store"
)

// Stub implementations for AgentWorkflow to satisfy Driver interface
// MySQL support can be added later if needed

func (d *DB) CreateAgentWorkflow(ctx context.Context, create *store.CreateAgentWorkflow) (*store.AgentWorkflow, error) {
	return nil, errors.New("agent workflows not implemented for MySQL")
}

func (d *DB) ListAgentWorkflows(ctx context.Context, find *store.FindAgentWorkflow) ([]*store.AgentWorkflow, error) {
	return nil, errors.New("agent workflows not implemented for MySQL")
}

func (d *DB) GetAgentWorkflow(ctx context.Context, find *store.FindAgentWorkflow) (*store.AgentWorkflow, error) {
	return nil, errors.New("agent workflows not implemented for MySQL")
}
