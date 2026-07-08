package postgres

import (
	"context"
	"database/sql"

	"github.com/usememos/memos/store"
)

func (d *DB) UpsertObservationLog(ctx context.Context, log *store.ObservationLog) (*store.ObservationLog, error) {
	stmt := `
		INSERT INTO agent_observations (
			session_id, tenant_id, resource_id, observation_log,
			last_observed_msg_index, tokens_in_log, current_task,
			suggested_response, last_updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT(session_id) DO UPDATE SET
			resource_id = EXCLUDED.resource_id,
			observation_log = EXCLUDED.observation_log,
			last_observed_msg_index = EXCLUDED.last_observed_msg_index,
			tokens_in_log = EXCLUDED.tokens_in_log,
			current_task = EXCLUDED.current_task,
			suggested_response = EXCLUDED.suggested_response,
			last_updated_at = EXCLUDED.last_updated_at
		RETURNING created_at
	`

	if err := d.db.QueryRowContext(ctx, stmt,
		log.SessionID, log.TenantID, log.ResourceID, log.ObservationLog,
		log.LastObservedMsgIndex, log.TokensInLog, log.CurrentTask,
		log.SuggestedResponse, log.LastUpdatedAt,
	).Scan(&log.CreatedAt); err != nil {
		return nil, err
	}

	return log, nil
}

func (d *DB) GetObservationLog(ctx context.Context, sessionID string) (*store.ObservationLog, error) {
	stmt := `
		SELECT session_id, tenant_id, resource_id, observation_log,
			last_observed_msg_index, tokens_in_log, current_task,
			suggested_response, created_at, last_updated_at
		FROM agent_observations
		WHERE session_id = $1
	`
	row := d.db.QueryRowContext(ctx, stmt, sessionID)
	log := &store.ObservationLog{}
	if err := row.Scan(
		&log.SessionID, &log.TenantID, &log.ResourceID, &log.ObservationLog,
		&log.LastObservedMsgIndex, &log.TokensInLog, &log.CurrentTask,
		&log.SuggestedResponse, &log.CreatedAt, &log.LastUpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return log, nil
}

func (d *DB) GetObservationLogByResource(ctx context.Context, resourceID string) (*store.ObservationLog, error) {
	stmt := `
		SELECT session_id, tenant_id, resource_id, observation_log,
			last_observed_msg_index, tokens_in_log, current_task,
			suggested_response, created_at, last_updated_at
		FROM agent_observations
		WHERE resource_id = $1
		ORDER BY last_updated_at DESC
		LIMIT 1
	`
	row := d.db.QueryRowContext(ctx, stmt, resourceID)
	log := &store.ObservationLog{}
	if err := row.Scan(
		&log.SessionID, &log.TenantID, &log.ResourceID, &log.ObservationLog,
		&log.LastObservedMsgIndex, &log.TokensInLog, &log.CurrentTask,
		&log.SuggestedResponse, &log.CreatedAt, &log.LastUpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return log, nil
}
