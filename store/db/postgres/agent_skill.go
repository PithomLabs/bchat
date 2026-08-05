package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateSkillExecution(ctx context.Context, execution *store.SkillExecution) (*store.SkillExecution, error) {
	checkpointJSON, _ := json.Marshal(execution.CheckpointData)
	completedJSON, _ := json.Marshal(execution.CompletedNodes)
	failedJSON, _ := json.Marshal(execution.FailedNodes)

	stmt := `
		INSERT INTO agent_skill_executions (
			id, tenant_id, conversation_id, skill_graph, status, trigger_path,
			current_node, checkpoint_data, completed_nodes, failed_nodes,
			error_message,
			retry_count, max_retries, parent_execution_id,
			claimed_at, claimed_by, claim_expires_at,
			created_at, updated_at, completed_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
	`
	_, err := d.db.ExecContext(ctx, stmt,
		execution.ID,
		execution.TenantID,
		execution.ConversationID,
		execution.SkillGraphJSON,
		execution.Status,
		execution.TriggerPath,
		execution.CurrentNode,
		checkpointJSON,
		completedJSON,
		failedJSON,
		execution.ErrorMessage,
		execution.RetryCount,
		execution.MaxRetries,
		execution.ParentExecutionID,
		execution.ClaimedAt,
		execution.ClaimedBy,
		execution.ClaimExpiresAt,
		execution.CreatedAt,
		execution.UpdatedAt,
		execution.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return execution, nil
}

func (d *DB) GetSkillExecution(ctx context.Context, find *store.FindSkillExecution) (*store.SkillExecution, error) {
	list, err := d.ListSkillExecutions(ctx, find, 1)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (d *DB) UpdateSkillExecution(ctx context.Context, execution *store.SkillExecution) error {
	checkpointJSON, _ := json.Marshal(execution.CheckpointData)
	completedJSON, _ := json.Marshal(execution.CompletedNodes)
	failedJSON, _ := json.Marshal(execution.FailedNodes)

	stmt := `
		UPDATE agent_skill_executions
		SET status = $1, trigger_path = $2, current_node = $3,
			checkpoint_data = $4, completed_nodes = $5, failed_nodes = $6,
			error_message = $7,
			retry_count = $8, claimed_at = $9, claimed_by = $10, claim_expires_at = $11,
			updated_at = $12, completed_at = $13
		WHERE id = $14
	`
	_, err := d.db.ExecContext(ctx, stmt,
		execution.Status,
		execution.TriggerPath,
		execution.CurrentNode,
		checkpointJSON,
		completedJSON,
		failedJSON,
		execution.ErrorMessage,
		execution.RetryCount,
		execution.ClaimedAt,
		execution.ClaimedBy,
		execution.ClaimExpiresAt,
		execution.UpdatedAt,
		execution.CompletedAt,
		execution.ID,
	)
	return err
}

func (d *DB) ListPendingSkillExecutions(ctx context.Context) ([]*store.SkillExecution, error) {
	stmt := `
		SELECT id, tenant_id, conversation_id, skill_graph, status, trigger_path,
			current_node, checkpoint_data, completed_nodes, failed_nodes,
			error_message,
			retry_count, max_retries, parent_execution_id,
			claimed_at, claimed_by, claim_expires_at,
			created_at, updated_at, completed_at
		FROM agent_skill_executions
		WHERE status IN ('pending', 'running')
			AND trigger_path != 'chat'
		ORDER BY created_at ASC
	`
	rows, err := d.db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSkillExecutions(rows)
}

// N5-1: Use time.Time for TIMESTAMPTZ columns, not int64 epochs.
func (d *DB) ClaimSkillExecution(ctx context.Context, id string, workerID string, leaseSeconds int) (*store.SkillExecution, error) {
	now := time.Now()
	expiresAt := now.Add(time.Duration(leaseSeconds) * time.Second)

	stmt := `
		UPDATE agent_skill_executions
		SET status = 'running', claimed_at = $1, claimed_by = $2, claim_expires_at = $3
		WHERE id = $4
			AND (status = 'pending'
				OR (status = 'running' AND (claim_expires_at < $5 OR claimed_by = $6)))
	`
	result, err := d.db.ExecContext(ctx, stmt, now, workerID, expiresAt, id, now, workerID)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, fmt.Errorf("execution %s could not be claimed", id)
	}

	return d.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &id})
}

func (d *DB) ReleaseSkillClaim(ctx context.Context, id string) error {
	stmt := `UPDATE agent_skill_executions SET claimed_at = NULL, claimed_by = NULL, claim_expires_at = NULL WHERE id = $1`
	_, err := d.db.ExecContext(ctx, stmt, id)
	return err
}

func (d *DB) CreateSkillLog(ctx context.Context, log *store.SkillLog) error {
	inputJSON, _ := json.Marshal(log.Input)
	outputJSON, _ := json.Marshal(log.Output)

	stmt := `
		INSERT INTO agent_skill_logs (
			id, tenant_id, execution_id, skill_name, handler, status,
			input, output, error_message, duration_ms, started_at, completed_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`
	_, err := d.db.ExecContext(ctx, stmt,
		log.ID,
		log.TenantID,
		log.ExecutionID,
		log.SkillName,
		log.Handler,
		log.Status,
		inputJSON,
		outputJSON,
		log.ErrorMessage,
		log.DurationMs,
		log.StartedAt,
		log.CompletedAt,
	)
	return err
}

func (d *DB) ListSkillLogs(ctx context.Context, find *store.FindSkillLog) ([]*store.SkillLog, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = "+placeholder(len(args)+1))
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = "+placeholder(len(args)+1))
		args = append(args, *find.TenantID)
	}
	if find.ExecutionID != nil {
		where = append(where, "execution_id = "+placeholder(len(args)+1))
		args = append(args, *find.ExecutionID)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, execution_id, skill_name, handler, status,
			input, output, error_message, duration_ms, started_at, completed_at
		FROM agent_skill_logs
		WHERE %s
		ORDER BY started_at DESC
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*store.SkillLog
	for rows.Next() {
		l := &store.SkillLog{}
		var inputJSON, outputJSON []byte
		if err := rows.Scan(
			&l.ID, &l.TenantID, &l.ExecutionID, &l.SkillName, &l.Handler, &l.Status,
			&inputJSON, &outputJSON, &l.ErrorMessage, &l.DurationMs, &l.StartedAt, &l.CompletedAt,
		); err != nil {
			return nil, err
		}
		if len(inputJSON) > 0 {
			json.Unmarshal(inputJSON, &l.Input)
		}
		if len(outputJSON) > 0 {
			json.Unmarshal(outputJSON, &l.Output)
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

func (d *DB) ListSkillExecutions(ctx context.Context, find *store.FindSkillExecution, limit int) ([]*store.SkillExecution, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = "+placeholder(len(args)+1))
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = "+placeholder(len(args)+1))
		args = append(args, *find.TenantID)
	}
	if find.ConversationID != nil {
		where = append(where, "conversation_id = "+placeholder(len(args)+1))
		args = append(args, *find.ConversationID)
	}
	if find.Status != nil {
		where = append(where, "status = "+placeholder(len(args)+1))
		args = append(args, *find.Status)
	}
	if find.TriggerPath != nil {
		where = append(where, "trigger_path = "+placeholder(len(args)+1))
		args = append(args, *find.TriggerPath)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, conversation_id, skill_graph, status, trigger_path,
			current_node, checkpoint_data, completed_nodes, failed_nodes,
			error_message,
			retry_count, max_retries, parent_execution_id,
			claimed_at, claimed_by, claim_expires_at,
			created_at, updated_at, completed_at
		FROM agent_skill_executions
		WHERE %s
		ORDER BY created_at DESC
		LIMIT %d
	`, strings.Join(where, " AND "), limit)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSkillExecutions(rows)
}

func scanSkillExecutions(rows *sql.Rows) ([]*store.SkillExecution, error) {
	var list []*store.SkillExecution
	for rows.Next() {
		e := &store.SkillExecution{}
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.ConversationID, &e.SkillGraphJSON, &e.Status, &e.TriggerPath,
			&e.CurrentNode, &e.CheckpointData, &e.CompletedNodes, &e.FailedNodes,
			&e.ErrorMessage,
			&e.RetryCount, &e.MaxRetries, &e.ParentExecutionID,
			&e.ClaimedAt, &e.ClaimedBy, &e.ClaimExpiresAt,
			&e.CreatedAt, &e.UpdatedAt, &e.CompletedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

// CompleteSkillExecution marks an execution as completed (conditional on status=running).
func (d *DB) CompleteSkillExecution(ctx context.Context, id string) error {
	tag, err := d.db.ExecContext(ctx,
		`UPDATE agent_skill_executions
		 SET status = 'completed', completed_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND status = 'running'`, id,
	)
	if err != nil {
		return fmt.Errorf("complete skill execution: %w", err)
	}
	n, _ := tag.RowsAffected()
	if n == 0 {
		slog.Debug("CompleteSkillExecution: no-op (already terminal)", "id", id)
	}
	return nil
}

// FailSkillExecution marks an execution as failed with error message.
// Only applies if the execution is not already stopped or completed.
func (d *DB) FailSkillExecution(ctx context.Context, id string, errorMsg string) error {
	tag, err := d.db.ExecContext(ctx,
		`UPDATE agent_skill_executions
		 SET status = 'failed', error_message = $1, completed_at = NOW(), updated_at = NOW()
		 WHERE id = $2 AND status NOT IN ('stopped', 'completed')`, errorMsg, id,
	)
	if err != nil {
		return fmt.Errorf("fail skill execution: %w", err)
	}
	n, _ := tag.RowsAffected()
	if n == 0 {
		slog.Debug("FailSkillExecution: no-op (already terminal)", "id", id)
	}
	return nil
}

// StopSkillExecution marks an execution as stopped (user-initiated cancellation).
// Only applies if the execution is in pending or running state.
func (d *DB) StopSkillExecution(ctx context.Context, id string) error {
	tag, err := d.db.ExecContext(ctx,
		`UPDATE agent_skill_executions
		 SET status = 'stopped', updated_at = NOW()
		 WHERE id = $1 AND status IN ('pending', 'running')`, id,
	)
	if err != nil {
		return fmt.Errorf("stop skill execution: %w", err)
	}
	n, _ := tag.RowsAffected()
	if n == 0 {
		slog.Debug("StopSkillExecution: no-op (already terminal)", "id", id)
	}
	return nil
}
