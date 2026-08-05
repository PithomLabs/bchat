package mysql

import (
	"context"
	"errors"

	"github.com/usememos/memos/store"
)

// Skill execution and log methods — stubs for MySQL.
// Full implementation can be added later if needed.

func (d *DB) CreateSkillExecution(ctx context.Context, execution *store.SkillExecution) (*store.SkillExecution, error) {
	return nil, errors.New("skill executions not implemented for MySQL")
}

func (d *DB) GetSkillExecution(ctx context.Context, find *store.FindSkillExecution) (*store.SkillExecution, error) {
	return nil, errors.New("skill executions not implemented for MySQL")
}

func (d *DB) UpdateSkillExecution(ctx context.Context, execution *store.SkillExecution) error {
	return errors.New("skill executions not implemented for MySQL")
}

func (d *DB) ListPendingSkillExecutions(ctx context.Context) ([]*store.SkillExecution, error) {
	return nil, errors.New("skill executions not implemented for MySQL")
}

func (d *DB) ClaimSkillExecution(ctx context.Context, id string, workerID string, leaseSeconds int) (*store.SkillExecution, error) {
	return nil, errors.New("skill executions not implemented for MySQL")
}

func (d *DB) ReleaseSkillClaim(ctx context.Context, id string) error {
	return errors.New("skill executions not implemented for MySQL")
}

func (d *DB) ListSkillExecutions(ctx context.Context, find *store.FindSkillExecution, limit int) ([]*store.SkillExecution, error) {
	return nil, errors.New("skill executions not implemented for MySQL")
}

func (d *DB) CompleteSkillExecution(ctx context.Context, id string) error {
	return errors.New("skill executions not implemented for MySQL")
}

func (d *DB) FailSkillExecution(ctx context.Context, id string, errorMsg string) error {
	return errors.New("skill executions not implemented for MySQL")
}

func (d *DB) StopSkillExecution(ctx context.Context, id string) error {
	return errors.New("skill executions not implemented for MySQL")
}

func (d *DB) CreateSkillLog(ctx context.Context, log *store.SkillLog) error {
	return errors.New("skill logs not implemented for MySQL")
}

func (d *DB) ListSkillLogs(ctx context.Context, find *store.FindSkillLog) ([]*store.SkillLog, error) {
	return nil, errors.New("skill logs not implemented for MySQL")
}
