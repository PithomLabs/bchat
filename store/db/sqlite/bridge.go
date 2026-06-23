package sqlite

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
		) VALUES (?, ?, 'active', ?, ?, ?, ?)
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
		WHERE tenant_id = ? AND session_id = ?
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
	session.ExpiresAt = nullableUnixTime(expiresAt)
	session.LastSeenAt = nullableUnixTime(lastSeenAt)
	return &session, nil
}

func (d *DB) TouchBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string, now, expiresAt time.Time) error {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return err
	}
	result, err := d.db.ExecContext(ctx, `
		UPDATE bridge_external_sessions
		SET updated_at = ?, last_seen_at = ?, expires_at = ?
		WHERE tenant_id = ? AND session_id = ?
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
		if !isSQLiteRetryable(err) {
			return nil, err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return nil, store.ErrBridgeHandoffConflict
}

func (d *DB) createBridgeHandoffAttempt(ctx context.Context, tenantID int32, sessionID string, now time.Time) (*store.BridgeHandoff, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin bridge handoff transaction: %w", err)
	}
	defer tx.Rollback()

	var externalSessionID int64
	err = tx.QueryRowContext(ctx, `
		UPDATE bridge_external_sessions
		SET updated_at = updated_at
		WHERE tenant_id = ? AND session_id = ?
		RETURNING id
	`, tenantID, sessionID).Scan(&externalSessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrBridgeExternalSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock bridge external session: %w", err)
	}

	var activeCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM bridge_handoffs WHERE external_session_id = ? AND active = 1`, externalSessionID).Scan(&activeCount); err != nil {
		return nil, fmt.Errorf("check active bridge handoff: %w", err)
	}
	if activeCount != 0 {
		return nil, store.ErrBridgeHandoffConflict
	}

	var generation int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(generation), 0) + 1 FROM bridge_handoffs WHERE external_session_id = ?`, externalSessionID).Scan(&generation); err != nil {
		return nil, fmt.Errorf("allocate bridge handoff generation: %w", err)
	}

	handoffID := uuid.NewString()
	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO bridge_handoffs (
			external_session_id, handoff_id, tenant_id, session_id, generation,
			routing_mode, outcome, active, version, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 'handoff_queued', NULL, 1, 1, ?, ?)
		RETURNING id
	`, externalSessionID, handoffID, tenantID, sessionID, generation, now.Unix(), now.Unix()).Scan(&id)
	if err != nil {
		if isSQLiteConstraint(err) {
			return nil, store.ErrBridgeHandoffConflict
		}
		return nil, fmt.Errorf("insert bridge handoff: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit bridge handoff: %w", err)
	}
	return d.findBridgeHandoffByIdentity(ctx, tenantID, sessionID, generation, handoffID)
}

func (d *DB) FindActiveBridgeHandoff(ctx context.Context, tenantID int32, sessionID string) (*store.BridgeHandoff, error) {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return nil, err
	}
	row := d.db.QueryRowContext(ctx, bridgeHandoffSelect+`
		WHERE tenant_id = ? AND session_id = ? AND active = 1 AND outcome IS NULL
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
		SET routing_mode = ?, version = version + 1, updated_at = ?, transition_reason = ?,
			active = CASE WHEN ? THEN 0 ELSE active END,
			outcome = CASE WHEN ? THEN 'closed' ELSE outcome END,
			closed_at = CASE WHEN ? THEN ? ELSE closed_at END
		WHERE tenant_id = ? AND session_id = ? AND generation = ? AND handoff_id = ?
		  AND version = ? AND routing_mode = ? AND active = 1
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
		WHERE tenant_id = ? AND session_id = ? AND generation = ? AND handoff_id = ?
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
	var active int
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
	handoff.Active = active == 1
	handoff.CreatedAt = time.Unix(createdAt, 0)
	handoff.UpdatedAt = time.Unix(updatedAt, 0)
	handoff.ClosedAt = nullableUnixTime(closedAt)
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

func nullableUnixTime(value sql.NullInt64) *time.Time {
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

func (d *DB) GetBridgeHandoff(ctx context.Context, tenantID int32, sessionID string, handoffID string) (*store.BridgeHandoff, error) {
	if err := store.ValidateExternalSessionID(sessionID); err != nil {
		return nil, err
	}
	row := d.db.QueryRowContext(ctx, bridgeHandoffSelect+` WHERE tenant_id = ? AND session_id = ? AND handoff_id = ?`, tenantID, sessionID, handoffID)
	handoff, err := scanBridgeHandoff(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrBridgeHandoffNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bridge handoff: %w", err)
	}
	return handoff, nil
}

func isSQLiteConstraint(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "constraint failed")
}

func isSQLiteRetryable(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database is busy")
}

func (d *DB) CreateBridgeHandoffReplyIfActive(ctx context.Context, create *store.CreateBridgeHandoffReply) (*store.BridgeHandoffReply, error) {
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
	if err != nil {
		return nil, fmt.Errorf("begin immediate transaction: %w", err)
	}

	rollback := func() {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
	}

	var sessionID string
	var active int
	var routingMode string
	var generation int64

	err = conn.QueryRowContext(ctx, `
		SELECT session_id, active, routing_mode, generation
		FROM bridge_handoffs
		WHERE tenant_id = ? AND handoff_id = ?
	`, create.TenantID, create.HandoffID).Scan(&sessionID, &active, &routingMode, &generation)
	if errors.Is(err, sql.ErrNoRows) {
		rollback()
		return nil, store.ErrBridgeHandoffNotFound
	}
	if err != nil {
		rollback()
		return nil, fmt.Errorf("query handoff in transaction: %w", err)
	}

	if sessionID != create.SessionID {
		rollback()
		return nil, store.ErrBridgeHandoffConflict
	}

	if active != 1 || routingMode != string(store.BridgeRoutingModeHumanActive) {
		rollback()
		return nil, store.ErrBridgeHandoffConflict
	}

	_, err = conn.ExecContext(ctx, `
		INSERT INTO bridge_handoff_replies (
			reply_id, tenant_id, session_id, handoff_id, generation,
			client_message_id, text, delivery_status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'not_delivered', ?)
	`, create.ReplyID, create.TenantID, create.SessionID, create.HandoffID, generation,
		create.ClientMessageID, create.Text, create.Now)

	if err != nil {
		if isSQLiteConstraint(err) {
			var existingReplyID string
			var existingText string
			var existingDeliveryStatus string
			var existingCreatedAt int64
			var existingGeneration int64

			errQuery := conn.QueryRowContext(ctx, `
				SELECT reply_id, text, delivery_status, created_at, generation
				FROM bridge_handoff_replies
				WHERE tenant_id = ? AND session_id = ? AND handoff_id = ? AND client_message_id = ?
			`, create.TenantID, create.SessionID, create.HandoffID, create.ClientMessageID).Scan(
				&existingReplyID, &existingText, &existingDeliveryStatus, &existingCreatedAt, &existingGeneration,
			)
			if errQuery == nil {
				if existingText == create.Text {
					rollback()
					return &store.BridgeHandoffReply{
						ReplyID:         existingReplyID,
						TenantID:        create.TenantID,
						SessionID:       create.SessionID,
						HandoffID:       create.HandoffID,
						Generation:      existingGeneration,
						ClientMessageID: create.ClientMessageID,
						Text:            existingText,
						DeliveryStatus:  existingDeliveryStatus,
						CreatedAt:       existingCreatedAt,
					}, nil
				} else {
					rollback()
					return nil, store.ErrBridgeHandoffReplyTextMismatch
				}
			}
		}
		rollback()
		return nil, fmt.Errorf("insert reply: %w", err)
	}

	_, err = conn.ExecContext(ctx, "COMMIT")
	if err != nil {
		rollback()
		return nil, fmt.Errorf("commit reply: %w", err)
	}

	return &store.BridgeHandoffReply{
		ReplyID:         create.ReplyID,
		TenantID:        create.TenantID,
		SessionID:       create.SessionID,
		HandoffID:       create.HandoffID,
		Generation:      generation,
		ClientMessageID: create.ClientMessageID,
		Text:            create.Text,
		DeliveryStatus:  "not_delivered",
		CreatedAt:       create.Now,
	}, nil
}

func (d *DB) GetBridgeHandoffReplyByClientMessageID(ctx context.Context, tenantID int32, sessionID string, handoffID string, clientMessageID string) (*store.BridgeHandoffReply, error) {
	var reply store.BridgeHandoffReply
	err := d.db.QueryRowContext(ctx, `
		SELECT id, reply_id, tenant_id, session_id, handoff_id, generation, client_message_id, text, delivery_status, created_at
		FROM bridge_handoff_replies
		WHERE tenant_id = ? AND session_id = ? AND handoff_id = ? AND client_message_id = ?
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

func (d *DB) CreateBridgeHandoffReplyAndOutboxIfActive(ctx context.Context, create *store.CreateBridgeHandoffReply) (*store.BridgeHandoffReplyWithOutbox, error) {
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
	if err != nil {
		return nil, fmt.Errorf("begin immediate transaction: %w", err)
	}

	rollback := func() {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
	}

	var sessionID string
	var active int
	var routingMode string
	var generation int64

	err = conn.QueryRowContext(ctx, `
		SELECT session_id, active, routing_mode, generation
		FROM bridge_handoffs
		WHERE tenant_id = ? AND handoff_id = ?
	`, create.TenantID, create.HandoffID).Scan(&sessionID, &active, &routingMode, &generation)
	if errors.Is(err, sql.ErrNoRows) {
		rollback()
		return nil, store.ErrBridgeHandoffNotFound
	}
	if err != nil {
		rollback()
		return nil, fmt.Errorf("query handoff in transaction: %w", err)
	}

	if sessionID != create.SessionID {
		rollback()
		return nil, store.ErrBridgeHandoffConflict
	}

	if active != 1 || routingMode != string(store.BridgeRoutingModeHumanActive) {
		rollback()
		return nil, store.ErrBridgeHandoffConflict
	}

	var finalReply *store.BridgeHandoffReply
	var finalOutbox *store.BridgeReplyOutbox

	// Try inserting the reply
	_, err = conn.ExecContext(ctx, `
		INSERT INTO bridge_handoff_replies (
			reply_id, tenant_id, session_id, handoff_id, generation,
			client_message_id, text, delivery_status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'not_delivered', ?)
	`, create.ReplyID, create.TenantID, create.SessionID, create.HandoffID, generation,
		create.ClientMessageID, create.Text, create.Now)

	if err == nil {
		// Inserted reply successfully, now insert the outbox
		outboxID := uuid.NewString()
		_, err = conn.ExecContext(ctx, `
			INSERT INTO bridge_reply_outbox (
				outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at,
				claim_token, claimed_by, claimed_at, claim_expires_at
			) VALUES (?, ?, ?, ?, ?, 'pending', 0, ?, NULL, NULL, NULL, NULL)
		`, outboxID, create.TenantID, create.SessionID, create.HandoffID, create.ReplyID, create.Now)
		if err != nil {
			rollback()
			return nil, fmt.Errorf("insert outbox: %w", err)
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
	} else if isSQLiteConstraint(err) {
		// Unique constraint failed. Retrieve existing reply.
		var existingReplyID string
		var existingText string
		var existingDeliveryStatus string
		var existingCreatedAt int64
		var existingGeneration int64

		errQuery := conn.QueryRowContext(ctx, `
			SELECT reply_id, text, delivery_status, created_at, generation
			FROM bridge_handoff_replies
			WHERE tenant_id = ? AND session_id = ? AND handoff_id = ? AND client_message_id = ?
		`, create.TenantID, create.SessionID, create.HandoffID, create.ClientMessageID).Scan(
			&existingReplyID, &existingText, &existingDeliveryStatus, &existingCreatedAt, &existingGeneration,
		)
		if errQuery != nil {
			rollback()
			return nil, fmt.Errorf("query existing reply on constraint violation: %w", errQuery)
		}

		if existingText != create.Text {
			rollback()
			return nil, store.ErrBridgeHandoffReplyTextMismatch
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

		// Now check if outbox row exists for this reply_id
		var ob store.BridgeReplyOutbox
		var obID int64
		var obCreatedAt int64
		errOutbox := conn.QueryRowContext(ctx, `
			SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
			FROM bridge_reply_outbox
			WHERE tenant_id = ? AND reply_id = ?
		`, create.TenantID, existingReplyID).Scan(
			&obID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &obCreatedAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
		)

		if errOutbox == nil {
			// Found existing outbox row, reuse it
			ob.ID = obID
			ob.CreatedAt = obCreatedAt
			finalOutbox = &ob
		} else if errors.Is(errOutbox, sql.ErrNoRows) {
			// Legacy recovery path: insert new outbox row
			outboxID := uuid.NewString()
			_, err = conn.ExecContext(ctx, `
				INSERT INTO bridge_reply_outbox (
					outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at,
					claim_token, claimed_by, claimed_at, claim_expires_at
				) VALUES (?, ?, ?, ?, ?, 'pending', 0, ?, NULL, NULL, NULL, NULL)
			`, outboxID, create.TenantID, create.SessionID, create.HandoffID, existingReplyID, create.Now)
			if err != nil {
				// Check if another concurrent call inserted the outbox
				if isSQLiteConstraint(err) {
					errOutboxRetry := conn.QueryRowContext(ctx, `
						SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
						FROM bridge_reply_outbox
						WHERE tenant_id = ? AND reply_id = ?
					`, create.TenantID, existingReplyID).Scan(
						&obID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &obCreatedAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
					)
					if errOutboxRetry == nil {
						ob.ID = obID
						ob.CreatedAt = obCreatedAt
						finalOutbox = &ob
					} else {
						rollback()
						return nil, fmt.Errorf("recover legacy outbox concurrency fallback: %w", errOutboxRetry)
					}
				} else {
					rollback()
					return nil, fmt.Errorf("insert legacy recovery outbox: %w", err)
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
			rollback()
			return nil, fmt.Errorf("query outbox: %w", errOutbox)
		}
	} else {
		rollback()
		return nil, fmt.Errorf("insert reply failed: %w", err)
	}

	_, err = conn.ExecContext(ctx, "COMMIT")
	if err != nil {
		rollback()
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &store.BridgeHandoffReplyWithOutbox{
		Reply:  finalReply,
		Outbox: finalOutbox,
	}, nil
}

func (d *DB) GetBridgeReplyOutboxByReplyID(ctx context.Context, tenantID int32, replyID string) (*store.BridgeReplyOutbox, error) {
	var ob store.BridgeReplyOutbox
	var createdAt int64
	err := d.db.QueryRowContext(ctx, `
		SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
		FROM bridge_reply_outbox
		WHERE tenant_id = ? AND reply_id = ?
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

	conn, err := d.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
	if err != nil {
		return nil, fmt.Errorf("begin immediate transaction: %w", err)
	}

	rollback := func() {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
	}

	rows, err := conn.QueryContext(ctx, `
		SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
		FROM bridge_reply_outbox
		WHERE tenant_id = ? AND status = 'pending'
		ORDER BY created_at ASC, id ASC
		LIMIT ?
	`, tenantID, limit)
	if err != nil {
		rollback()
		return nil, fmt.Errorf("query pending rows: %w", err)
	}

	var candidates []*store.BridgeReplyOutbox
	for rows.Next() {
		var ob store.BridgeReplyOutbox
		var createdAt int64
		err = rows.Scan(&ob.ID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &createdAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage)
		if err != nil {
			rows.Close()
			rollback()
			return nil, fmt.Errorf("scan pending row: %w", err)
		}
		ob.CreatedAt = createdAt
		candidates = append(candidates, &ob)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		rollback()
		return nil, fmt.Errorf("iterate pending rows: %w", err)
	}

	var claimed []*store.BridgeReplyOutbox
	expiresAt := nowUnix + claimDurationSeconds

	for _, ob := range candidates {
		claimToken := uuid.NewString()

		res, err := conn.ExecContext(ctx, `
			UPDATE bridge_reply_outbox
			SET status='claimed',
			    claim_token=?,
			    claimed_by=?,
			    claimed_at=?,
			    claim_expires_at=?,
			    attempt_count=attempt_count+1
			WHERE id=? AND tenant_id=? AND status='pending'
		`, claimToken, claimedBy, nowUnix, expiresAt, ob.ID, tenantID)
		if err != nil {
			rollback()
			return nil, fmt.Errorf("update claim: %w", err)
		}

		rowsAffected, err := res.RowsAffected()
		if err != nil {
			rollback()
			return nil, fmt.Errorf("check rows affected: %w", err)
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

	_, err = conn.ExecContext(ctx, "COMMIT")
	if err != nil {
		rollback()
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return claimed, nil
}

func (d *DB) CompleteClaimedBridgeReplyOutbox(ctx context.Context, complete *store.CompleteBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
	// Settlement is claim-token based. claim_expires_at is recorded for future recovery
	// workflows, but BRIDGE-DELIVERY-0008 intentionally does not enforce claim
	// expiration or recycle expired claims.
	if complete.TenantID <= 0 || len(complete.OutboxID) != 36 || len(complete.ClaimToken) != 36 || complete.Now <= 0 {
		return nil, store.ErrBridgeInvalidArgument
	}

	conn, err := d.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
	if err != nil {
		return nil, fmt.Errorf("begin immediate transaction: %w", err)
	}

	rollback := func() {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
	}

	var ob store.BridgeReplyOutbox
	var createdAt int64
	err = conn.QueryRowContext(ctx, `
		SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
		FROM bridge_reply_outbox
		WHERE tenant_id = ? AND outbox_id = ?
	`, complete.TenantID, complete.OutboxID).Scan(
		&ob.ID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &createdAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		rollback()
		return nil, store.ErrBridgeOutboxNotFound
	}
	if err != nil {
		rollback()
		return nil, fmt.Errorf("query outbox: %w", err)
	}

	if ob.ClaimedAt == nil {
		rollback()
		return nil, store.ErrBridgeOutboxConflict
	}
	if complete.Now < *ob.ClaimedAt {
		rollback()
		return nil, store.ErrBridgeInvalidArgument
	}

	res, err := conn.ExecContext(ctx, `
		UPDATE bridge_reply_outbox
		SET status='completed', completed_at=?
		WHERE tenant_id=? AND outbox_id=? AND claim_token=? AND status='claimed'
	`, complete.Now, complete.TenantID, complete.OutboxID, complete.ClaimToken)
	if err != nil {
		rollback()
		return nil, fmt.Errorf("update complete: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		rollback()
		return nil, fmt.Errorf("check complete rows affected: %w", err)
	}

	if rowsAffected == 1 {
		_, err = conn.ExecContext(ctx, "COMMIT")
		if err != nil {
			rollback()
			return nil, fmt.Errorf("commit transaction: %w", err)
		}
		ob.CreatedAt = createdAt
		ob.Status = "completed"
		ob.CompletedAt = &complete.Now
		return &ob, nil
	}

	// Idempotency / conflict check
	if ob.Status == "completed" && ob.ClaimToken != nil && *ob.ClaimToken == complete.ClaimToken {
		_, err = conn.ExecContext(ctx, "COMMIT")
		if err != nil {
			rollback()
			return nil, fmt.Errorf("commit transaction on idempotent complete: %w", err)
		}
		ob.CreatedAt = createdAt
		return &ob, nil
	}

	rollback()
	return nil, store.ErrBridgeOutboxConflict
}

func (d *DB) FailClaimedBridgeReplyOutbox(ctx context.Context, fail *store.FailBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
	// Settlement is claim-token based. claim_expires_at is recorded for future recovery
	// workflows, but BRIDGE-DELIVERY-0008 intentionally does not enforce claim
	// expiration or recycle expired claims.
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

	conn, err := d.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
	if err != nil {
		return nil, fmt.Errorf("begin immediate transaction: %w", err)
	}

	rollback := func() {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
	}

	var ob store.BridgeReplyOutbox
	var createdAt int64
	err = conn.QueryRowContext(ctx, `
		SELECT id, outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at, claim_token, claimed_by, claimed_at, claim_expires_at, completed_at, failed_at, failure_code, failure_message
		FROM bridge_reply_outbox
		WHERE tenant_id = ? AND outbox_id = ?
	`, fail.TenantID, fail.OutboxID).Scan(
		&ob.ID, &ob.OutboxID, &ob.TenantID, &ob.SessionID, &ob.HandoffID, &ob.ReplyID, &ob.Status, &ob.AttemptCount, &createdAt, &ob.ClaimToken, &ob.ClaimedBy, &ob.ClaimedAt, &ob.ClaimExpiresAt, &ob.CompletedAt, &ob.FailedAt, &ob.FailureCode, &ob.FailureMessage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		rollback()
		return nil, store.ErrBridgeOutboxNotFound
	}
	if err != nil {
		rollback()
		return nil, fmt.Errorf("query outbox: %w", err)
	}

	if ob.ClaimedAt == nil {
		rollback()
		return nil, store.ErrBridgeOutboxConflict
	}
	if fail.Now < *ob.ClaimedAt {
		rollback()
		return nil, store.ErrBridgeInvalidArgument
	}

	res, err := conn.ExecContext(ctx, `
		UPDATE bridge_reply_outbox
		SET status='failed', failed_at=?, failure_code=?, failure_message=?
		WHERE tenant_id=? AND outbox_id=? AND claim_token=? AND status='claimed'
	`, fail.Now, fail.FailureCode, fail.FailureMessage, fail.TenantID, fail.OutboxID, fail.ClaimToken)
	if err != nil {
		rollback()
		return nil, fmt.Errorf("update fail: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		rollback()
		return nil, fmt.Errorf("check fail rows affected: %w", err)
	}

	if rowsAffected == 1 {
		_, err = conn.ExecContext(ctx, "COMMIT")
		if err != nil {
			rollback()
			return nil, fmt.Errorf("commit transaction: %w", err)
		}
		ob.CreatedAt = createdAt
		ob.Status = "failed"
		ob.FailedAt = &fail.Now
		ob.FailureCode = &fail.FailureCode
		ob.FailureMessage = &fail.FailureMessage
		return &ob, nil
	}

	// Idempotency / conflict check
	if ob.Status == "failed" && ob.ClaimToken != nil && *ob.ClaimToken == fail.ClaimToken &&
		ob.FailureCode != nil && *ob.FailureCode == fail.FailureCode &&
		ob.FailureMessage != nil && *ob.FailureMessage == fail.FailureMessage {
		_, err = conn.ExecContext(ctx, "COMMIT")
		if err != nil {
			rollback()
			return nil, fmt.Errorf("commit transaction on idempotent fail: %w", err)
		}
		ob.CreatedAt = createdAt
		return &ob, nil
	}

	rollback()
	return nil, store.ErrBridgeOutboxConflict
}
