package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/usememos/memos/store"
)

func (d *DB) UpsertObservationLog(ctx context.Context, log *store.ObservationLog) (*store.ObservationLog, error) {
	stmt := `
		INSERT INTO agent_observations (
			session_id, tenant_id, observation_log, last_observed_msg_index, tokens_in_log, current_task, suggested_response, last_updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			observation_log = excluded.observation_log,
			last_observed_msg_index = excluded.last_observed_msg_index,
			tokens_in_log = excluded.tokens_in_log,
			current_task = excluded.current_task,
			suggested_response = excluded.suggested_response,
			last_updated_at = excluded.last_updated_at
		RETURNING created_at
	`
	now := time.Now()
	log.LastUpdatedAt = now

	// If it's a new record, CreatedAt will be set by DB default, but we need it back.
	// We can't easily rely on DB default if we want to return it immediately without a second query for new records.
	// But RETURNING created_at handles that in SQLite.

	if err := d.db.QueryRowContext(ctx, stmt,
		log.SessionID, log.TenantID, log.ObservationLog, log.LastObservedMsgIndex, log.TokensInLog, log.CurrentTask, log.SuggestedResponse, log.LastUpdatedAt,
	).Scan(&log.CreatedAt); err != nil {
		return nil, err
	}

	return log, nil
}

func (d *DB) GetObservationLog(ctx context.Context, sessionID string) (*store.ObservationLog, error) {
	stmt := `
		SELECT session_id, tenant_id, observation_log, last_observed_msg_index, tokens_in_log, current_task, suggested_response, created_at, last_updated_at
		FROM agent_observations
		WHERE session_id = ?
	`
	row := d.db.QueryRowContext(ctx, stmt, sessionID)
	log := &store.ObservationLog{}
	if err := row.Scan(
		&log.SessionID, &log.TenantID, &log.ObservationLog, &log.LastObservedMsgIndex, &log.TokensInLog, &log.CurrentTask, &log.SuggestedResponse, &log.CreatedAt, &log.LastUpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil if not found
		}
		return nil, err
	}
	return log, nil
}
