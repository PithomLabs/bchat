package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateAgentWorkflow(ctx context.Context, create *store.CreateAgentWorkflow) (*store.AgentWorkflow, error) {
	stmt := `
		INSERT INTO agent_workflows (
			ticket_id,
			session_id,
			agent_name,
			task_name,
			task_mode,
			task_status,
			task_summary,
			predicted_size,
			created_ts,
			metadata
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	workflow := &store.AgentWorkflow{
		TicketID:      create.TicketID,
		SessionID:     create.SessionID,
		AgentName:     create.AgentName,
		TaskName:      create.TaskName,
		TaskMode:      create.TaskMode,
		TaskStatus:    create.TaskStatus,
		TaskSummary:   create.TaskSummary,
		PredictedSize: create.PredictedSize,
		CreatedTs:     create.CreatedTs,
		Metadata:      create.Metadata,
	}

	if err := d.db.QueryRowContext(
		ctx,
		stmt,
		workflow.TicketID,
		workflow.SessionID,
		workflow.AgentName,
		workflow.TaskName,
		workflow.TaskMode,
		workflow.TaskStatus,
		workflow.TaskSummary,
		workflow.PredictedSize,
		workflow.CreatedTs,
		workflow.Metadata,
	).Scan(&workflow.ID); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (d *DB) ListAgentWorkflows(ctx context.Context, find *store.FindAgentWorkflow) ([]*store.AgentWorkflow, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TicketID != nil {
		where = append(where, "ticket_id = ?")
		args = append(args, *find.TicketID)
	}
	if find.SessionID != nil {
		where = append(where, "session_id = ?")
		args = append(args, *find.SessionID)
	}

	query := fmt.Sprintf(`
		SELECT
			id,
			ticket_id,
			session_id,
			agent_name,
			task_name,
			task_mode,
			task_status,
			task_summary,
			predicted_size,
			created_ts,
			metadata
		FROM agent_workflows
		WHERE %s
		ORDER BY created_ts DESC
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*store.AgentWorkflow, 0)
	for rows.Next() {
		var workflow store.AgentWorkflow
		if err := rows.Scan(
			&workflow.ID,
			&workflow.TicketID,
			&workflow.SessionID,
			&workflow.AgentName,
			&workflow.TaskName,
			&workflow.TaskMode,
			&workflow.TaskStatus,
			&workflow.TaskSummary,
			&workflow.PredictedSize,
			&workflow.CreatedTs,
			&workflow.Metadata,
		); err != nil {
			return nil, err
		}
		list = append(list, &workflow)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func (d *DB) GetAgentWorkflow(ctx context.Context, find *store.FindAgentWorkflow) (*store.AgentWorkflow, error) {
	list, err := d.ListAgentWorkflows(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}
