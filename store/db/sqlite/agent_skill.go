package sqlite

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

// time helpers for SQLite (epoch int64 <-> time.Time)

func timeToUnix(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	v := t.Unix()
	return &v
}

func timeToUnixVal(t time.Time) int64 {
	return t.Unix()
}

func unixToTime(v *int64) *time.Time {
	if v == nil {
		return nil
	}
	t := time.Unix(*v, 0)
	return &t
}

func unixToTimeVal(v int64) time.Time {
	return time.Unix(v, 0)
}

func nullInt64ToTime(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.Unix(v.Int64, 0)
	return &t
}

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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.ExecContext(ctx, stmt,
		execution.ID,
		execution.TenantID,
		execution.ConversationID,
		execution.SkillGraphJSON,
		execution.Status,
		execution.TriggerPath,
		execution.CurrentNode,
		string(checkpointJSON),
		string(completedJSON),
		string(failedJSON),
		execution.ErrorMessage,
		execution.RetryCount,
		execution.MaxRetries,
		execution.ParentExecutionID,
		timeToUnix(execution.ClaimedAt),
		execution.ClaimedBy,
		timeToUnix(execution.ClaimExpiresAt),
		timeToUnixVal(execution.CreatedAt),
		timeToUnixVal(execution.UpdatedAt),
		timeToUnix(execution.CompletedAt),
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
		SET status = ?, trigger_path = ?, current_node = ?,
			checkpoint_data = ?, completed_nodes = ?, failed_nodes = ?,
			error_message = ?,
			retry_count = ?, claimed_at = ?, claimed_by = ?, claim_expires_at = ?,
			updated_at = ?, completed_at = ?
		WHERE id = ?
	`
	_, err := d.db.ExecContext(ctx, stmt,
		execution.Status,
		execution.TriggerPath,
		execution.CurrentNode,
		string(checkpointJSON),
		string(completedJSON),
		string(failedJSON),
		execution.ErrorMessage,
		execution.RetryCount,
		timeToUnix(execution.ClaimedAt),
		execution.ClaimedBy,
		timeToUnix(execution.ClaimExpiresAt),
		timeToUnixVal(execution.UpdatedAt),
		timeToUnix(execution.CompletedAt),
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

func (d *DB) ClaimSkillExecution(ctx context.Context, id string, workerID string, leaseSeconds int) (*store.SkillExecution, error) {
	now := time.Now().Unix()
	expiresAt := now + int64(leaseSeconds)

	stmt := `
		UPDATE agent_skill_executions
		SET status = 'running', claimed_at = ?, claimed_by = ?, claim_expires_at = ?
		WHERE id = ?
			AND (status = 'pending'
				OR (status = 'running' AND (claim_expires_at < ? OR claimed_by = ?)))
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
	stmt := `UPDATE agent_skill_executions SET claimed_at = NULL, claimed_by = NULL, claim_expires_at = NULL WHERE id = ?`
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.ExecContext(ctx, stmt,
		log.ID,
		log.TenantID,
		log.ExecutionID,
		log.SkillName,
		log.Handler,
		log.Status,
		string(inputJSON),
		string(outputJSON),
		log.ErrorMessage,
		log.DurationMs,
		timeToUnixVal(log.StartedAt),
		timeToUnix(log.CompletedAt),
	)
	return err
}

func (d *DB) ListSkillLogs(ctx context.Context, find *store.FindSkillLog) ([]*store.SkillLog, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.ExecutionID != nil {
		where = append(where, "execution_id = ?")
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
		var inputStr, outputStr sql.NullString
		var completedAt sql.NullInt64
		var startedAt int64
		if err := rows.Scan(
			&l.ID, &l.TenantID, &l.ExecutionID, &l.SkillName, &l.Handler, &l.Status,
			&inputStr, &outputStr, &l.ErrorMessage, &l.DurationMs, &startedAt, &completedAt,
		); err != nil {
			return nil, err
		}
		if inputStr.Valid {
			json.Unmarshal([]byte(inputStr.String), &l.Input)
		}
		if outputStr.Valid {
			json.Unmarshal([]byte(outputStr.String), &l.Output)
		}
		l.StartedAt = unixToTimeVal(startedAt)
		if completedAt.Valid {
			l.CompletedAt = unixToTime(&completedAt.Int64)
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

// ListSkillExecutions returns executions matching the filter.
func (d *DB) ListSkillExecutions(ctx context.Context, find *store.FindSkillExecution, limit int) ([]*store.SkillExecution, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.ConversationID != nil {
		where = append(where, "conversation_id = ?")
		args = append(args, *find.ConversationID)
	}
	if find.Status != nil {
		where = append(where, "status = ?")
		args = append(args, *find.Status)
	}
	if find.TriggerPath != nil {
		where = append(where, "trigger_path = ?")
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
		var checkpointStr, completedStr, failedStr sql.NullString
		var parentID, claimedBy sql.NullString
		var claimedAt, claimExpiresAt, completedAt sql.NullInt64
		var createdAt, updatedAt int64

		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.ConversationID, &e.SkillGraphJSON, &e.Status, &e.TriggerPath,
			&e.CurrentNode, &checkpointStr, &completedStr, &failedStr,
			&e.ErrorMessage,
			&e.RetryCount, &e.MaxRetries, &parentID,
			&claimedAt, &claimedBy, &claimExpiresAt,
			&createdAt, &updatedAt, &completedAt,
		); err != nil {
			return nil, err
		}

		e.CheckpointData = make(map[string]any)
		if checkpointStr.Valid {
			json.Unmarshal([]byte(checkpointStr.String), &e.CheckpointData)
		}
		e.CompletedNodes = make(map[string]any)
		if completedStr.Valid {
			json.Unmarshal([]byte(completedStr.String), &e.CompletedNodes)
		}
		e.FailedNodes = make(map[string]any)
		if failedStr.Valid {
			json.Unmarshal([]byte(failedStr.String), &e.FailedNodes)
		}
		if parentID.Valid {
			e.ParentExecutionID = &parentID.String
		}
		e.ClaimedAt = nullInt64ToTime(claimedAt)
		if claimedBy.Valid {
			e.ClaimedBy = &claimedBy.String
		}
		e.ClaimExpiresAt = nullInt64ToTime(claimExpiresAt)
		e.CreatedAt = unixToTimeVal(createdAt)
		e.UpdatedAt = unixToTimeVal(updatedAt)
		e.CompletedAt = nullInt64ToTime(completedAt)

		list = append(list, e)
	}
	return list, rows.Err()
}

// CompleteSkillExecution marks an execution as completed (conditional on status=running).
func (d *DB) CompleteSkillExecution(ctx context.Context, id string) error {
	result, err := d.db.ExecContext(ctx,
		`UPDATE agent_skill_executions
		 SET status = 'completed', completed_at = ?, updated_at = ?
		 WHERE id = ? AND status = 'running'`,
		time.Now().Unix(), time.Now().Unix(), id,
	)
	if err != nil {
		return fmt.Errorf("complete skill execution: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		slog.Debug("CompleteSkillExecution: no-op (already terminal)", "id", id)
	}
	return nil
}

// FailSkillExecution marks an execution as failed with error message.
// Only applies if the execution is not already stopped or completed.
func (d *DB) FailSkillExecution(ctx context.Context, id string, errorMsg string) error {
	result, err := d.db.ExecContext(ctx,
		`UPDATE agent_skill_executions
		 SET status = 'failed', error_message = ?, completed_at = ?, updated_at = ?
		 WHERE id = ? AND status NOT IN ('stopped', 'completed')`,
		errorMsg, time.Now().Unix(), time.Now().Unix(), id,
	)
	if err != nil {
		return fmt.Errorf("fail skill execution: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		slog.Debug("FailSkillExecution: no-op (already terminal)", "id", id)
	}
	return nil
}

// StopSkillExecution marks an execution as stopped (user-initiated cancellation).
// Only applies if the execution is in pending or running state.
func (d *DB) StopSkillExecution(ctx context.Context, id string) error {
	result, err := d.db.ExecContext(ctx,
		`UPDATE agent_skill_executions
		 SET status = 'stopped', updated_at = ?
		 WHERE id = ? AND status IN ('pending', 'running')`,
		time.Now().Unix(), id,
	)
	if err != nil {
		return fmt.Errorf("stop skill execution: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		slog.Debug("StopSkillExecution: no-op (already terminal)", "id", id)
	}
	return nil
}
