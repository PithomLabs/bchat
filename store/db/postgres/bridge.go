package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/usememos/memos/store"
)

func (d *DB) EnsureBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string, now, expiresAt time.Time) (*store.BridgeExternalSession, bool, error) {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return nil, false, err
	}
	result, err := d.db.ExecContext(ctx, `
		INSERT INTO bridge_external_sessions (
			tenant_id, session_id, status, created_at, updated_at, expires_at, last_seen_at
		) VALUES ($1, $2, 'active', $3, $4, $5, $6)
		ON CONFLICT(tenant_id, session_id) DO NOTHING
	`, tenantID, sessionID, now.Unix(), now.Unix(), expiresAt.Unix(), now.Unix())
	if err != nil {
		return nil, false, fmt.Errorf("ensure bridge external session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, false, fmt.Errorf("read bridge external session insert result: %w", err)
	}
	created := rows == 1
	if !created {
		if err := d.TouchBridgeExternalSession(ctx, tenantID, sessionID, now, expiresAt); err != nil {
			return nil, false, err
		}
	}
	session, err := d.FindBridgeExternalSession(ctx, tenantID, sessionID)
	return session, created, err
}

func (d *DB) FindBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string) (*store.BridgeExternalSession, error) {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return nil, err
	}
	var session store.BridgeExternalSession
	var status string
	var createdAt, updatedAt int64
	var expiresAt, lastSeenAt sql.NullInt64
	err := d.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, session_id, status, created_at, updated_at, expires_at, last_seen_at
		FROM bridge_external_sessions
		WHERE tenant_id = $1 AND session_id = $2
	`, tenantID, sessionID).Scan(
		&session.ID, &session.TenantID, &session.SessionID, &status,
		&createdAt, &updatedAt, &expiresAt, &lastSeenAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find bridge external session: %w", err)
	}
	session.Status = store.BridgeExternalSessionStatus(status)
	session.CreatedAt = time.Unix(createdAt, 0)
	session.UpdatedAt = time.Unix(updatedAt, 0)
	session.ExpiresAt = nullableUnixTimeNull(expiresAt)
	session.LastSeenAt = nullableUnixTimeNull(lastSeenAt)
	return &session, nil
}

func (d *DB) TouchBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string, now, expiresAt time.Time) error {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return err
	}
	result, err := d.db.ExecContext(ctx, `
		UPDATE bridge_external_sessions
		SET updated_at = $1, last_seen_at = $2, expires_at = $3
		WHERE tenant_id = $4 AND session_id = $5
	`, now.Unix(), now.Unix(), expiresAt.Unix(), tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("touch bridge external session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read bridge external session touch result: %w", err)
	}
	if rows == 0 {
		return store.ErrBridgeExternalSessionNotFound
	}
	return nil
}

func (d *DB) CreateBridgeHandoff(ctx context.Context, tenantID int32, sessionID string, now time.Time) (*store.BridgeHandoff, error) {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		handoff, err := d.createBridgeHandoffAttempt(ctx, tenantID, sessionID, now)
		if err == nil || errors.Is(err, store.ErrBridgeExternalSessionNotFound) || errors.Is(err, store.ErrBridgeHandoffConflict) {
			return handoff, err
		}
		if !isPostgresRetryable(err) {
			return nil, err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return nil, store.ErrBridgeHandoffConflict
}

func (d *DB) createBridgeHandoffAttempt(ctx context.Context, tenantID int32, sessionID string, now time.Time) (*store.BridgeHandoff, error) {
	// retry-safe: status-guarded — the active-handoff COUNT check and the
	// external-session touch re-read inside the retried transaction, so a
	// 40001 retry cannot double-create a handoff.
	var handoffID string
	var generation int
	err := d.execTx(ctx, func(tx *sql.Tx) error {
		var externalSessionID int64
		err := tx.QueryRowContext(ctx, `
			UPDATE bridge_external_sessions
			SET updated_at = updated_at
			WHERE tenant_id = $1 AND session_id = $2
			RETURNING id
		`, tenantID, sessionID).Scan(&externalSessionID)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrBridgeExternalSessionNotFound
		}
		if err != nil {
			return fmt.Errorf("lock bridge external session: %w", err)
		}

		var activeCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM bridge_handoffs WHERE external_session_id = $1 AND active = true`, externalSessionID).Scan(&activeCount); err != nil {
			return fmt.Errorf("check active bridge handoff: %w", err)
		}
		if activeCount != 0 {
			return store.ErrBridgeHandoffConflict
		}

		var gen int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(generation), 0) + 1 FROM bridge_handoffs WHERE external_session_id = $1`, externalSessionID).Scan(&gen); err != nil {
			return fmt.Errorf("allocate bridge handoff generation: %w", err)
		}

		hid := uuid.NewString()
		var id int64
		err = tx.QueryRowContext(ctx, `
			INSERT INTO bridge_handoffs (
				external_session_id, handoff_id, tenant_id, session_id, generation,
				routing_mode, outcome, active, version, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, 'handoff_queued', NULL, true, 1, $6, $7)
			RETURNING id
		`, externalSessionID, hid, tenantID, sessionID, gen, now.Unix(), now.Unix()).Scan(&id)
		if err != nil {
			if isPostgresConstraint(err) {
				return store.ErrBridgeHandoffConflict
			}
			return fmt.Errorf("insert bridge handoff: %w", err)
		}
		handoffID = hid
		generation = gen
		return nil
	})
	if err != nil {
		return nil, err
	}
	return d.findBridgeHandoffByIdentity(ctx, tenantID, sessionID, generation, handoffID)
}

func (d *DB) FindActiveBridgeHandoff(ctx context.Context, tenantID int32, sessionID string) (*store.BridgeHandoff, error) {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return nil, err
	}
	row := d.db.QueryRowContext(ctx, bridgeHandoffSelect+`
		WHERE tenant_id = $1 AND session_id = $2 AND active = true AND outcome IS NULL
		  AND routing_mode IN ('handoff_queued', 'human_active')
		ORDER BY generation DESC LIMIT 1
	`, tenantID, sessionID)
	handoff, err := scanBridgeHandoff(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find active bridge handoff: %w", err)
	}
	return handoff, nil
}

func (d *DB) UpdateBridgeHandoffRoutingModeCAS(ctx context.Context, tenantID int32, sessionID string, generation int, handoffID string, fromVersion int, fromMode, toMode store.BridgeRoutingMode, reason string, now time.Time) (*store.BridgeHandoff, error) {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return nil, err
	}
	if err := store.ValidateBridgeRoutingMode(fromMode); err != nil {
		return nil, err
	}
	if err := store.ValidateBridgeRoutingMode(toMode); err != nil {
		return nil, err
	}
	if len(reason) > 512 {
		return nil, fmt.Errorf("transition reason exceeds 512 characters")
	}
	closed := toMode == store.BridgeRoutingModeClosed
	result, err := d.db.ExecContext(ctx, `
		UPDATE bridge_handoffs
		SET routing_mode = $1, version = version + 1, updated_at = $2, transition_reason = $3,
			active = CASE WHEN $4 THEN false ELSE active END,
			outcome = CASE WHEN $5 THEN 'closed' ELSE outcome END,
			closed_at = CASE WHEN $6 THEN $7 ELSE closed_at END
		WHERE tenant_id = $8 AND session_id = $9 AND generation = $10 AND handoff_id = $11
		  AND version = $12 AND routing_mode = $13 AND active = true
	`, toMode, now.Unix(), nullableString(reason), closed, closed, closed, now.Unix(),
		tenantID, sessionID, generation, handoffID, fromVersion, fromMode)
	if err != nil {
		return nil, fmt.Errorf("update bridge handoff CAS: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read bridge handoff CAS result: %w", err)
	}
	if rows != 1 {
		return nil, store.ErrBridgeHandoffConflict
	}
	return d.findBridgeHandoffByIdentity(ctx, tenantID, sessionID, generation, handoffID)
}

const bridgeHandoffSelect = `
	SELECT id, external_session_id, handoff_id, tenant_id, session_id, generation,
		routing_mode, outcome, active, version, harness_id, operator_id, ticket_id,
		memo_uid, transition_reason, created_at, updated_at, closed_at
	FROM bridge_handoffs
	`

func (d *DB) findBridgeHandoffByIdentity(ctx context.Context, tenantID int32, sessionID string, generation int, handoffID string) (*store.BridgeHandoff, error) {
	row := d.db.QueryRowContext(ctx, bridgeHandoffSelect+`
		WHERE tenant_id = $1 AND session_id = $2 AND generation = $3 AND handoff_id = $4
	`, tenantID, sessionID, generation, handoffID)
	handoff, err := scanBridgeHandoff(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrBridgeHandoffNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find bridge handoff: %w", err)
	}
	return handoff, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBridgeHandoff(row rowScanner) (*store.BridgeHandoff, error) {
	var handoff store.BridgeHandoff
	var routingMode string
	var outcome, harnessID, operatorID, memoUID, transitionReason sql.NullString
	var ticketID sql.NullInt32
	var active bool
	var createdAt, updatedAt int64
	var closedAt sql.NullInt64
	if err := row.Scan(
		&handoff.ID, &handoff.ExternalSessionID, &handoff.HandoffID, &handoff.TenantID,
		&handoff.SessionID, &handoff.Generation, &routingMode, &outcome, &active,
		&handoff.Version, &harnessID, &operatorID, &ticketID, &memoUID,
		&transitionReason, &createdAt, &updatedAt, &closedAt,
	); err != nil {
		return nil, err
	}
	handoff.RoutingMode = store.BridgeRoutingMode(routingMode)
	handoff.Active = active
	handoff.CreatedAt = time.Unix(createdAt, 0)
	handoff.UpdatedAt = time.Unix(updatedAt, 0)
	handoff.ClosedAt = nullableUnixTimeNull(closedAt)
	if outcome.Valid {
		value := store.BridgeOutcome(outcome.String)
		handoff.Outcome = &value
	}
	handoff.HarnessID = nullableStringPtr(harnessID)
	handoff.OperatorID = nullableStringPtr(operatorID)
	handoff.MemoUID = nullableStringPtr(memoUID)
	handoff.TransitionReason = nullableStringPtr(transitionReason)
	if ticketID.Valid {
		value := ticketID.Int32
		handoff.TicketID = &value
	}
	return &handoff, nil
}

func nullableUnixTime(value int64) *time.Time {
	result := time.Unix(value, 0)
	return &result
}

func nullableUnixTimeNull(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	result := time.Unix(value.Int64, 0)
	return &result
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func isPostgresConstraint(err error) bool {
	message := err.Error()
	return containsAny(message,
		"unique_constraint", "foreign_key_constraint", "check_constraint",
		"violates unique constraint", "violates foreign key constraint",
		"violates check constraint", "duplicate key value violates",
	)
}

func isPostgresRetryable(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "deadlock") || strings.Contains(message, "lock not available") || strings.Contains(message, "could not serialize")
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func (d *DB) GetBridgeHandoff(ctx context.Context, tenantID int32, sessionID string, handoffID string) (*store.BridgeHandoff, error) {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return nil, err
	}
	row := d.db.QueryRowContext(ctx, bridgeHandoffSelect+` WHERE tenant_id = $1 AND session_id = $2 AND handoff_id = $3`, tenantID, sessionID, handoffID)
	handoff, err := scanBridgeHandoff(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrBridgeHandoffNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bridge handoff: %w", err)
	}
	return handoff, nil
}

func (d *DB) CreateBridgeHandoffReplyIfActive(ctx context.Context, create *store.CreateBridgeHandoffReply) (*store.BridgeHandoffReply, error) {
	// retry-safe: status-guarded — the active-handoff FOR UPDATE re-read inside
	// the retried transaction gates the insert, so a 40001 retry cannot
	// double-insert a reply; a constraint duplicate re-returns the existing row
	// instead of failing.
	var out *store.BridgeHandoffReply
	err := d.execTx(ctx, func(tx *sql.Tx) error {
		var sessionID string
		var active bool
		var routingMode string
		var generation int64

		err := tx.QueryRowContext(ctx, `
			SELECT session_id, active, routing_mode, generation
			FROM bridge_handoffs
			WHERE tenant_id = $1 AND handoff_id = $2
			FOR UPDATE
		`, create.TenantID, create.HandoffID).Scan(&sessionID, &active, &routingMode, &generation)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrBridgeHandoffNotFound
		}
		if err != nil {
			return fmt.Errorf("query handoff in transaction: %w", err)
		}

		if sessionID != create.SessionID {
			return store.ErrBridgeHandoffConflict
		}

		if !active || routingMode != string(store.BridgeRoutingModeHumanActive) {
			return store.ErrBridgeHandoffConflict
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO bridge_handoff_replies (
				reply_id, tenant_id, session_id, handoff_id, generation,
				client_message_id, text, delivery_status, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, 'not_delivered', $8)
		`, create.ReplyID, create.TenantID, create.SessionID, create.HandoffID, generation,
			create.ClientMessageID, create.Text, create.Now)

		if err != nil {
			if isPostgresConstraint(err) {
				var existingReplyID string
				var existingText string
				var existingDeliveryStatus string
				var existingCreatedAt int64
				var existingGeneration int64

				errQuery := tx.QueryRowContext(ctx, `
					SELECT reply_id, text, delivery_status, created_at, generation
					FROM bridge_handoff_replies
					WHERE tenant_id = $1 AND session_id = $2 AND handoff_id = $3 AND client_message_id = $4
				`, create.TenantID, create.SessionID, create.HandoffID, create.ClientMessageID).Scan(
					&existingReplyID, &existingText, &existingDeliveryStatus, &existingCreatedAt, &existingGeneration,
				)
				if errQuery == nil {
					if existingText == create.Text {
						out = &store.BridgeHandoffReply{
							ReplyID:         existingReplyID,
							TenantID:        create.TenantID,
							SessionID:       create.SessionID,
							HandoffID:       create.HandoffID,
							Generation:      existingGeneration,
							ClientMessageID: create.ClientMessageID,
							Text:            existingText,
							DeliveryStatus:  existingDeliveryStatus,
							CreatedAt:       existingCreatedAt,
						}
						return nil
					}
					return store.ErrBridgeHandoffReplyTextMismatch
				}
			}
			return fmt.Errorf("insert reply: %w", err)
		}

		out = &store.BridgeHandoffReply{
			ReplyID:         create.ReplyID,
			TenantID:        create.TenantID,
			SessionID:       create.SessionID,
			HandoffID:       create.HandoffID,
			Generation:      generation,
			ClientMessageID: create.ClientMessageID,
			Text:            create.Text,
			DeliveryStatus:  "not_delivered",
			CreatedAt:       create.Now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) GetBridgeHandoffReplyByClientMessageID(ctx context.Context, tenantID int32, sessionID string, handoffID string, clientMessageID string) (*store.BridgeHandoffReply, error) {
	var reply store.BridgeHandoffReply
	err := d.db.QueryRowContext(ctx, `
		SELECT id, reply_id, tenant_id, session_id, handoff_id, generation, client_message_id, text, delivery_status, created_at
		FROM bridge_handoff_replies
		WHERE tenant_id = $1 AND session_id = $2 AND handoff_id = $3 AND client_message_id = $4
	`, tenantID, sessionID, handoffID, clientMessageID).Scan(
		&reply.ID, &reply.ReplyID, &reply.TenantID, &reply.SessionID, &reply.HandoffID,
		&reply.Generation, &reply.ClientMessageID, &reply.Text, &reply.DeliveryStatus, &reply.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get bridge handoff reply by client message id: %w", err)
	}
	return &reply, nil
}

func (d *DB) GetBridgeHandoffReplyByReplyID(ctx context.Context, tenantID int32, replyID string) (*store.BridgeHandoffReply, error) {
	var reply store.BridgeHandoffReply
	err := d.db.QueryRowContext(ctx, `
		SELECT id, reply_id, tenant_id, session_id, handoff_id, generation, client_message_id, text, delivery_status, created_at
		FROM bridge_handoff_replies
		WHERE tenant_id = $1 AND reply_id = $2
	`, tenantID, replyID).Scan(
		&reply.ID, &reply.ReplyID, &reply.TenantID, &reply.SessionID, &reply.HandoffID,
		&reply.Generation, &reply.ClientMessageID, &reply.Text, &reply.DeliveryStatus, &reply.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get bridge handoff reply by reply id: %w", err)
	}
	return &reply, nil
}

func (d *DB) CreateBridgeHandoffReplyAndOutboxIfActive(ctx context.Context, create *store.CreateBridgeHandoffReply) (*store.BridgeHandoffReplyWithOutbox, error) {
	// retry-safe: status-guarded — the active-handoff FOR UPDATE re-read inside
	// the retried transaction gates the inserts, so a 40001 retry cannot
	// double-insert a reply or outbox; constraint duplicates re-read the
	// existing rows instead of failing.
	var out *store.BridgeHandoffReplyWithOutbox
	err := d.execTx(ctx, func(tx *sql.Tx) error {
		var sessionID string
		var active bool
		var routingMode string
		var generation int64

		err := tx.QueryRowContext(ctx, `
			SELECT session_id, active, routing_mode, generation
			FROM bridge_handoffs
			WHERE tenant_id = $1 AND handoff_id = $2
			FOR UPDATE
		`, create.TenantID, create.HandoffID).Scan(&sessionID, &active, &routingMode, &generation)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrBridgeHandoffNotFound
		}
		if err != nil {
			return fmt.Errorf("query handoff in transaction: %w", err)
		}

		if sessionID != create.SessionID {
			return store.ErrBridgeHandoffConflict
		}

		if !active || routingMode != string(store.BridgeRoutingModeHumanActive) {
			return store.ErrBridgeHandoffConflict
		}

		var finalReply *store.BridgeHandoffReply
		var finalOutbox *store.BridgeReplyOutbox

		_, err = tx.ExecContext(ctx, `
			INSERT INTO bridge_handoff_replies (
				reply_id, tenant_id, session_id, handoff_id, generation,
				client_message_id, text, delivery_status, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, 'not_delivered', $8)
		`, create.ReplyID, create.TenantID, create.SessionID, create.HandoffID, generation,
			create.ClientMessageID, create.Text, create.Now)

		if err == nil {
			outboxID := uuid.NewString()
			_, err = tx.ExecContext(ctx, `
				INSERT INTO bridge_reply_outbox (
					outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at,
					claim_token, claimed_by, claimed_at, claim_expires_at
				) VALUES ($1, $2, $3, $4, $5, 'pending', 0, $6, NULL, NULL, NULL, NULL)
			`, outboxID, create.TenantID, create.SessionID, create.HandoffID, create.ReplyID, create.Now)
			if err != nil {
				return fmt.Errorf("insert outbox: %w", err)
			}

			finalReply = &store.BridgeHandoffReply{
				ReplyID:         create.ReplyID,
				TenantID:        create.TenantID,
				SessionID:       create.SessionID,
				HandoffID:       create.HandoffID,
				Generation:      generation,
				ClientMessageID: create.ClientMessageID,
				Text:            create.Text,
				DeliveryStatus:  "not_delivered",
				CreatedAt:       create.Now,
			}
			finalOutbox = &store.BridgeReplyOutbox{
				OutboxID:     outboxID,
				TenantID:     create.TenantID,
				SessionID:    create.SessionID,
				HandoffID:    create.HandoffID,
				ReplyID:      create.ReplyID,
				Status:       "pending",
				AttemptCount: 0,
				CreatedAt:    create.Now,
			}
		} else if isPostgresConstraint(err) {
			var existingReplyID string
			var existingText string
			var existingDeliveryStatus string
			var existingCreatedAt int64
			var existingGeneration int64

			errQuery := tx.QueryRowContext(ctx, `
				SELECT reply_id, text, delivery_status, created_at, generation
				FROM bridge_handoff_replies
				WHERE tenant_id = $1 AND session_id = $2 AND handoff_id = $3 AND client_message_id = $4
			`, create.TenantID, create.SessionID, create.HandoffID, create.ClientMessageID).Scan(
				&existingReplyID, &existingText, &existingDeliveryStatus, &existingCreatedAt, &existingGeneration,
			)
			if errQuery != nil {
				return fmt.Errorf("query existing reply on constraint violation: %w", errQuery)
			}

			if existingText != create.Text {
				return store.ErrBridgeHandoffReplyTextMismatch
			}

			finalReply = &store.BridgeHandoffReply{
				ReplyID:         existingReplyID,
				TenantID:        create.TenantID,
				SessionID:       create.SessionID,
				HandoffID:       create.HandoffID,
				Generation:      existingGeneration,
				ClientMessageID: create.ClientMessageID,
				Text:            existingText,
				DeliveryStatus:  existingDeliveryStatus,
				CreatedAt:       existingCreatedAt,
			}

			var ob store.BridgeReplyOutbox
			var obID int64
			var obCreatedAt int64
			errOutbox := tx.QueryRowContext(ctx, `
				SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
				FROM bridge_reply_outbox
				WHERE tenant_id = $1 AND reply_id = $2
			`, create.TenantID, existingReplyID).Scan(
				&obID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &obCreatedAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
			)

			if errOutbox == nil {
				ob.ID = obID
				ob.CreatedAt = obCreatedAt
				finalOutbox = &ob
			} else if errors.Is(errOutbox, sql.ErrNoRows) {
				outboxID := uuid.NewString()
				_, err = tx.ExecContext(ctx, `
					INSERT INTO bridge_reply_outbox (
						outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at,
						claim_token, claimed_by, claimed_at, claim_expires_at
					) VALUES ($1, $2, $3, $4, $5, 'pending', 0, $6, NULL, NULL, NULL, NULL)
				`, outboxID, create.TenantID, create.SessionID, create.HandoffID, existingReplyID, create.Now)
				if err != nil {
					if isPostgresConstraint(err) {
						errOutboxRetry := tx.QueryRowContext(ctx, `
							SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
							FROM bridge_reply_outbox
							WHERE tenant_id = $1 AND reply_id = $2
						`, create.TenantID, existingReplyID).Scan(
							&obID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &obCreatedAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
						)
						if errOutboxRetry == nil {
							ob.ID = obID
							ob.CreatedAt = obCreatedAt
							finalOutbox = &ob
						} else {
							return fmt.Errorf("recover legacy outbox concurrency fallback: %w", errOutboxRetry)
						}
					} else {
						return fmt.Errorf("insert legacy recovery outbox: %w", err)
					}
				} else {
					finalOutbox = &store.BridgeReplyOutbox{
						OutboxID:     outboxID,
						TenantID:     create.TenantID,
						SessionID:    create.SessionID,
						HandoffID:    create.HandoffID,
						ReplyID:      existingReplyID,
						Status:       "pending",
						AttemptCount: 0,
						CreatedAt:    create.Now,
					}
				}
			} else {
				return fmt.Errorf("query outbox: %w", errOutbox)
			}
		} else {
			return fmt.Errorf("insert reply failed: %w", err)
		}

		out = &store.BridgeHandoffReplyWithOutbox{
			Reply:  finalReply,
			Outbox: finalOutbox,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) GetBridgeReplyOutboxByReplyID(ctx context.Context, tenantID int32, replyID string) (*store.BridgeReplyOutbox, error) {
	var ob store.BridgeReplyOutbox
	var createdAt int64
	err := d.db.QueryRowContext(ctx, `
		SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
		FROM bridge_reply_outbox
		WHERE tenant_id = $1 AND reply_id = $2
	`, tenantID, replyID).Scan(
		&ob.ID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &createdAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get bridge reply outbox by reply id: %w", err)
	}
	ob.CreatedAt = createdAt
	return &ob, nil
}

func (d *DB) ClaimPendingBridgeReplyOutbox(ctx context.Context, tenantID int32, limit int, claimedBy string, now time.Time, claimDurationSeconds int64) ([]*store.BridgeReplyOutbox, error) {
	if tenantID <= 0 || limit < 1 || limit > 100 || len(claimedBy) < 1 || len(claimedBy) > 128 {
		return nil, store.ErrBridgeInvalidArgument
	}
	for _, r := range claimedBy {
		if r < 32 || r > 126 {
			return nil, store.ErrBridgeInvalidArgument
		}
	}
	nowUnix := now.Unix()
	if nowUnix <= 0 || claimDurationSeconds <= 0 {
		return nil, store.ErrBridgeInvalidArgument
	}

	// retry-safe: status-guarded — the conditional UPDATE ... WHERE status =
	// 'pending' (or expired claim) is re-evaluated inside the retried
	// transaction, so a 40001 retry cannot claim the same outbox twice.
	var claimed []*store.BridgeReplyOutbox
	err := d.execTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
			FROM bridge_reply_outbox
			WHERE tenant_id = $1
			  AND (
			    status = 'pending'
			    OR (status = 'claimed' AND claim_expires_at <= $2)
			  )
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, nowUnix, limit)
		if err != nil {
			return fmt.Errorf("query pending rows: %w", err)
		}

		var candidates []*store.BridgeReplyOutbox
		for rows.Next() {
			var ob store.BridgeReplyOutbox
			var createdAt int64
			err = rows.Scan(&ob.ID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &createdAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage)
			if err != nil {
				rows.Close()
				return fmt.Errorf("scan pending row: %w", err)
			}
			ob.CreatedAt = createdAt
			candidates = append(candidates, &ob)
		}
		rows.Close()

		expiresAt := nowUnix + claimDurationSeconds

		for _, ob := range candidates {
			claimToken := uuid.NewString()

			res, err := tx.ExecContext(ctx, `
				UPDATE bridge_reply_outbox
				SET status='claimed',
				    claim_token=$1,
				    claimed_by=$2,
				    claimed_at=$3,
				    claim_expires_at=$4,
				    attempt_count=attempt_count+1
				WHERE id=$5
				  AND tenant_id=$6
				  AND (
				    status='pending'
				    OR (status='claimed' AND claim_expires_at <= $7)
				  )
			`, claimToken, claimedBy, nowUnix, expiresAt, ob.ID, tenantID, nowUnix)
			if err != nil {
				return fmt.Errorf("update claim: %w", err)
			}

			rowsAffected, err := res.RowsAffected()
			if err != nil {
				return fmt.Errorf("check rows affected: %w", err)
			}

			if rowsAffected == 1 {
				ob.Status = "claimed"
				ob.ClaimToken = &claimToken
				ob.ClaimedBy = &claimedBy
				ob.ClaimedAt = &nowUnix
				ob.ClaimExpiresAt = &expiresAt
				ob.AttemptCount++
				claimed = append(claimed, ob)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}
func (d *DB) CompleteClaimedBridgeReplyOutbox(ctx context.Context, complete *store.CompleteBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
	if complete.TenantID <= 0 || len(complete.OutboxID) != 36 || len(complete.ClaimToken) != 36 || complete.Now <= 0 {
		return nil, store.ErrBridgeInvalidArgument
	}

	// retry-safe: token-guarded — the conditional UPDATE ... WHERE claim_token
	// = $4 AND status = 'claimed' filters already-completed rows on re-read, so
	// a 40001 retry cannot complete an outbox twice (the idempotent re-read
	// path returns the already-completed row).
	var out *store.BridgeReplyOutbox
	err := d.execTx(ctx, func(tx *sql.Tx) error {
		var ob store.BridgeReplyOutbox
		var createdAt int64
		err := tx.QueryRowContext(ctx, `
			SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
			FROM bridge_reply_outbox
			WHERE tenant_id = $1 AND outbox_id = $2
		`, complete.TenantID, complete.OutboxID).Scan(
			&ob.ID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &createdAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrBridgeOutboxNotFound
		}
		if err != nil {
			return fmt.Errorf("query outbox: %w", err)
		}

		if ob.ClaimedAt == nil {
			return store.ErrBridgeOutboxConflict
		}
		if complete.Now < *ob.ClaimedAt {
			return store.ErrBridgeInvalidArgument
		}

		res, err := tx.ExecContext(ctx, `
			UPDATE bridge_reply_outbox
			SET status='completed', completed_at=$1
			WHERE tenant_id=$2 AND outbox_id=$3 AND claim_token=$4 AND status='claimed'
		`, complete.Now, complete.TenantID, complete.OutboxID, complete.ClaimToken)
		if err != nil {
			return fmt.Errorf("update complete: %w", err)
		}

		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("check complete rows affected: %w", err)
		}

		if rowsAffected == 1 {
			ob.CreatedAt = createdAt
			ob.Status = "completed"
			ob.CompletedAt = &complete.Now
			out = &ob
			return nil
		}

		if ob.Status == "completed" && ob.ClaimToken != nil && *ob.ClaimToken == complete.ClaimToken {
			ob.CreatedAt = createdAt
			out = &ob
			return nil
		}

		return store.ErrBridgeOutboxConflict
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) FailClaimedBridgeReplyOutbox(ctx context.Context, fail *store.FailBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
	if fail.TenantID <= 0 || len(fail.OutboxID) != 36 || len(fail.ClaimToken) != 36 || fail.Now <= 0 {
		return nil, store.ErrBridgeInvalidArgument
	}
	if len(fail.FailureCode) < 1 || len(fail.FailureCode) > 64 {
		return nil, store.ErrBridgeInvalidArgument
	}
	for _, r := range fail.FailureCode {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return nil, store.ErrBridgeInvalidArgument
		}
	}
	if len(fail.FailureMessage) < 1 || len(fail.FailureMessage) > 1000 {
		return nil, store.ErrBridgeInvalidArgument
	}
	for _, r := range fail.FailureMessage {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return nil, store.ErrBridgeInvalidArgument
		}
	}

	// retry-safe: token-guarded — the conditional UPDATE ... WHERE claim_token
	// = $6 AND status = 'claimed' filters already-failed rows on re-read, so a
	// 40001 retry cannot fail an outbox twice (the idempotent re-read path
	// returns the already-failed row).
	var out *store.BridgeReplyOutbox
	err := d.execTx(ctx, func(tx *sql.Tx) error {
		var ob store.BridgeReplyOutbox
		var createdAt int64
		err := tx.QueryRowContext(ctx, `
			SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
			FROM bridge_reply_outbox
			WHERE tenant_id = $1 AND outbox_id = $2
		`, fail.TenantID, fail.OutboxID).Scan(
			&ob.ID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &createdAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrBridgeOutboxNotFound
		}
		if err != nil {
			return fmt.Errorf("query outbox: %w", err)
		}

		if ob.ClaimedAt == nil {
			return store.ErrBridgeOutboxConflict
		}
		if fail.Now < *ob.ClaimedAt {
			return store.ErrBridgeInvalidArgument
		}

		res, err := tx.ExecContext(ctx, `
			UPDATE bridge_reply_outbox
			SET status='failed', failed_at=$1, failure_code=$2, failure_message=$3
			WHERE tenant_id=$4 AND outbox_id=$5 AND claim_token=$6 AND status='claimed'
		`, fail.Now, fail.FailureCode, fail.FailureMessage, fail.TenantID, fail.OutboxID, fail.ClaimToken)
		if err != nil {
			return fmt.Errorf("update fail: %w", err)
		}

		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("check fail rows affected: %w", err)
		}

		if rowsAffected == 1 {
			ob.CreatedAt = createdAt
			ob.Status = "failed"
			ob.FailedAt = &fail.Now
			ob.FailureCode = &fail.FailureCode
			ob.FailureMessage = &fail.FailureMessage
			out = &ob
			return nil
		}

		if ob.Status == "failed" && ob.ClaimToken != nil && *ob.ClaimToken == fail.ClaimToken &&
			ob.FailureCode != nil && *ob.FailureCode == fail.FailureCode &&
			ob.FailureMessage != nil && *ob.FailureMessage == fail.FailureMessage {
			ob.CreatedAt = createdAt
			out = &ob
			return nil
		}

		return store.ErrBridgeOutboxConflict
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) ClaimBridgeReplyOutboxByOutboxID(ctx context.Context, tenantID int32, outboxID string, claimedBy string, now time.Time, claimDurationSeconds int64) (*store.BridgeReplyOutbox, error) {
	if tenantID <= 0 || len(outboxID) != 36 || len(claimedBy) < 1 || len(claimedBy) > 128 {
		return nil, store.ErrBridgeInvalidArgument
	}
	for _, r := range claimedBy {
		if r < 32 || r > 126 {
			return nil, store.ErrBridgeInvalidArgument
		}
	}
	nowUnix := now.Unix()
	if nowUnix <= 0 || claimDurationSeconds <= 0 {
		return nil, store.ErrBridgeInvalidArgument
	}

	// retry-safe: status-guarded — the conditional UPDATE ... WHERE status =
	// 'pending' (or expired claim) is re-evaluated inside the retried
	// transaction, so a 40001 retry cannot claim the outbox twice.
	var out *store.BridgeReplyOutbox
	err := d.execTx(ctx, func(tx *sql.Tx) error {
		var ob store.BridgeReplyOutbox
		var createdAt int64
		err := tx.QueryRowContext(ctx, `
			SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
			FROM bridge_reply_outbox
			WHERE tenant_id = $1 AND outbox_id = $2
		`, tenantID, outboxID).Scan(
			&ob.ID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &createdAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrBridgeOutboxNotFound
		}
		if err != nil {
			return fmt.Errorf("query outbox by id: %w", err)
		}

		if ob.Status == "completed" {
			return store.ErrBridgeOutboxAlreadyCompleted
		}
		if ob.Status == "failed" {
			return store.ErrBridgeOutboxAlreadyFailed
		}
		if ob.Status != "pending" && !(ob.Status == "claimed" && ob.ClaimExpiresAt != nil && *ob.ClaimExpiresAt <= nowUnix) {
			return store.ErrBridgeOutboxConflict
		}

		claimToken := uuid.NewString()
		expiresAt := nowUnix + claimDurationSeconds

		res, err := tx.ExecContext(ctx, `
			UPDATE bridge_reply_outbox
			SET status='claimed',
			    claim_token=$1,
			    claimed_by=$2,
			    claimed_at=$3,
			    claim_expires_at=$4,
			    attempt_count=attempt_count+1
			WHERE id=$5
			  AND tenant_id=$6
			  AND (
			    status='pending'
			    OR (status='claimed' AND claim_expires_at <= $7)
			  )
		`, claimToken, claimedBy, nowUnix, expiresAt, ob.ID, tenantID, nowUnix)
		if err != nil {
			return fmt.Errorf("update claim: %w", err)
		}

		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("check rows affected: %w", err)
		}

		if rowsAffected == 1 {
			ob.CreatedAt = createdAt
			ob.Status = "claimed"
			ob.ClaimToken = &claimToken
			ob.ClaimedBy = &claimedBy
			ob.ClaimedAt = &nowUnix
			ob.ClaimExpiresAt = &expiresAt
			ob.AttemptCount++
			out = &ob
			return nil
		}

		return store.ErrBridgeOutboxConflict
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
