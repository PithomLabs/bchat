package teststore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func createClaimedOutboxFixture(t *testing.T, slug string) (context.Context, *store.Store, *store.AgentTenant, *store.BridgeReplyOutbox) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, slug)
	_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: uuid.NewString(),
		Text:            "Hello",
		Now:             time.Now().Unix() - 1000,
	})
	require.NoError(t, err)

	claimedRows, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 1, "worker", time.Now(), 300)
	require.NoError(t, err)
	require.Len(t, claimedRows, 1)
	return ctx, ts, tenant, claimedRows[0]
}

func TestCompleteClaimedBridgeReplyOutboxCompletesClaimedRow(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "complete-claimed")
	defer ts.Close()

	now := time.Now().Unix()
	ob, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   claimed.OutboxID,
		ClaimToken: *claimed.ClaimToken,
		Now:        now,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", ob.Status)
	require.Equal(t, now, *ob.CompletedAt)
}

func TestCompleteClaimedBridgeReplyOutboxRequiresClaimedStatus(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "complete-pending")
	defer ts.Close()

	res, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	_, err = ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   res.Outbox.OutboxID,
		ClaimToken: uuid.NewString(),
		Now:        time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeOutboxConflict)
}

func TestCompleteClaimedBridgeReplyOutboxRequiresClaimToken(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "complete-req-token")
	defer ts.Close()

	_, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   claimed.OutboxID,
		ClaimToken: "",
		Now:        time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

func TestCompleteClaimedBridgeReplyOutboxWrongClaimTokenRejected(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "complete-wrong-token")
	defer ts.Close()

	_, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   claimed.OutboxID,
		ClaimToken: uuid.NewString(),
		Now:        time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeOutboxConflict)
}

func TestCompleteClaimedBridgeReplyOutboxIdempotentSameClaimToken(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "complete-idempotent")
	defer ts.Close()

	now := time.Now().Unix()
	ob1, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   claimed.OutboxID,
		ClaimToken: *claimed.ClaimToken,
		Now:        now,
	})
	require.NoError(t, err)

	ob2, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   claimed.OutboxID,
		ClaimToken: *claimed.ClaimToken,
		Now:        now + 10,
	})
	require.NoError(t, err)
	require.Equal(t, ob1.Status, ob2.Status)
	require.Equal(t, ob1.CompletedAt, ob2.CompletedAt)
}

func TestCompleteClaimedBridgeReplyOutboxCannotCompleteFailedRow(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "complete-failed")
	defer ts.Close()

	now := time.Now().Unix()
	_, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     *claimed.ClaimToken,
		Now:            now,
		FailureCode:    "ERR_TEST",
		FailureMessage: "Failed",
	})
	require.NoError(t, err)

	_, err = ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   claimed.OutboxID,
		ClaimToken: *claimed.ClaimToken,
		Now:        now,
	})
	require.ErrorIs(t, err, store.ErrBridgeOutboxConflict)
}

func TestCompleteClaimedBridgeReplyOutboxTenantIsolation(t *testing.T) {
	ctx, ts, _, claimed := createClaimedOutboxFixture(t, "complete-tenant1")
	defer ts.Close()
	tenant2 := createBridgeTenant(t, ctx, ts, "complete-tenant2")

	_, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant2.ID,
		OutboxID:   claimed.OutboxID,
		ClaimToken: *claimed.ClaimToken,
		Now:        time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeOutboxNotFound)
}

func TestCompleteClaimedBridgeReplyOutboxRejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	_, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   0,
		OutboxID:   uuid.NewString(),
		ClaimToken: uuid.NewString(),
		Now:        time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

func TestCompleteClaimedBridgeReplyOutboxRejectsNowBeforeClaimedAt(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "complete-time-travel")
	defer ts.Close()

	_, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   claimed.OutboxID,
		ClaimToken: *claimed.ClaimToken,
		Now:        *claimed.ClaimedAt - 1,
	})
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

// Fail Tests

func TestFailClaimedBridgeReplyOutboxFailsClaimedRow(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "fail-claimed")
	defer ts.Close()

	now := time.Now().Unix()
	ob, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     *claimed.ClaimToken,
		Now:            now,
		FailureCode:    "TEST_ERR",
		FailureMessage: "Failed text",
	})
	require.NoError(t, err)
	require.Equal(t, "failed", ob.Status)
	require.Equal(t, now, *ob.FailedAt)
	require.Equal(t, "TEST_ERR", *ob.FailureCode)
	require.Equal(t, "Failed text", *ob.FailureMessage)
}

func TestFailClaimedBridgeReplyOutboxRequiresClaimedStatus(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "fail-pending")
	defer ts.Close()

	res, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	_, err = ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       res.Outbox.OutboxID,
		ClaimToken:     uuid.NewString(),
		Now:            time.Now().Unix(),
		FailureCode:    "TEST",
		FailureMessage: "Test",
	})
	require.ErrorIs(t, err, store.ErrBridgeOutboxConflict)
}

func TestFailClaimedBridgeReplyOutboxRequiresClaimToken(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "fail-req-token")
	defer ts.Close()

	_, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     "",
		Now:            time.Now().Unix(),
		FailureCode:    "TEST",
		FailureMessage: "Test",
	})
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

func TestFailClaimedBridgeReplyOutboxWrongClaimTokenRejected(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "fail-wrong-token")
	defer ts.Close()

	_, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     uuid.NewString(),
		Now:            time.Now().Unix(),
		FailureCode:    "TEST",
		FailureMessage: "Test",
	})
	require.ErrorIs(t, err, store.ErrBridgeOutboxConflict)
}

func TestFailClaimedBridgeReplyOutboxIdempotentSameFailure(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "fail-idempotent")
	defer ts.Close()

	now := time.Now().Unix()
	ob1, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     *claimed.ClaimToken,
		Now:            now,
		FailureCode:    "TEST",
		FailureMessage: "Test",
	})
	require.NoError(t, err)

	ob2, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     *claimed.ClaimToken,
		Now:            now + 10,
		FailureCode:    "TEST",
		FailureMessage: "Test",
	})
	require.NoError(t, err)
	require.Equal(t, ob1.Status, ob2.Status)
	require.Equal(t, ob1.FailedAt, ob2.FailedAt)
}

func TestFailClaimedBridgeReplyOutboxDifferentFailureConflict(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "fail-conflict")
	defer ts.Close()

	now := time.Now().Unix()
	_, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     *claimed.ClaimToken,
		Now:            now,
		FailureCode:    "TEST",
		FailureMessage: "Test",
	})
	require.NoError(t, err)

	_, err = ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     *claimed.ClaimToken,
		Now:            now + 10,
		FailureCode:    "TEST_2",
		FailureMessage: "Test",
	})
	require.ErrorIs(t, err, store.ErrBridgeOutboxConflict)
}

func TestFailClaimedBridgeReplyOutboxCannotFailCompletedRow(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "fail-completed")
	defer ts.Close()

	now := time.Now().Unix()
	_, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   claimed.OutboxID,
		ClaimToken: *claimed.ClaimToken,
		Now:        now,
	})
	require.NoError(t, err)

	_, err = ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     *claimed.ClaimToken,
		Now:            now,
		FailureCode:    "TEST",
		FailureMessage: "Test",
	})
	require.ErrorIs(t, err, store.ErrBridgeOutboxConflict)
}

func TestFailClaimedBridgeReplyOutboxTenantIsolation(t *testing.T) {
	ctx, ts, _, claimed := createClaimedOutboxFixture(t, "fail-tenant1")
	defer ts.Close()
	tenant2 := createBridgeTenant(t, ctx, ts, "fail-tenant2")

	_, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant2.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     *claimed.ClaimToken,
		Now:            time.Now().Unix(),
		FailureCode:    "TEST",
		FailureMessage: "Test",
	})
	require.ErrorIs(t, err, store.ErrBridgeOutboxNotFound)
}

func TestFailClaimedBridgeReplyOutboxRejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	_, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       0,
		OutboxID:       uuid.NewString(),
		ClaimToken:     uuid.NewString(),
		Now:            time.Now().Unix(),
		FailureCode:    "INVALID CODE !!",
		FailureMessage: "Test",
	})
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

func TestFailClaimedBridgeReplyOutboxRejectsNowBeforeClaimedAt(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "fail-time-travel")
	defer ts.Close()

	_, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       claimed.OutboxID,
		ClaimToken:     *claimed.ClaimToken,
		Now:            *claimed.ClaimedAt - 1,
		FailureCode:    "TEST",
		FailureMessage: "Test",
	})
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

func TestBridgeReplyOutboxSettlementStatusConstraint(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "status-constraint")
	defer ts.Close()

	_, err := ts.GetDriver().GetDB().ExecContext(ctx, `
		INSERT INTO bridge_reply_outbox (
			outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at,
			claim_token, claimed_by, claimed_at, claim_expires_at, completed_at
		) VALUES (?, ?, ?, ?, ?, 'completed', 0, ?, NULL, NULL, NULL, NULL, ?)
	`, uuid.NewString(), tenant.ID, session.SessionID, handoff.HandoffID, uuid.NewString(), time.Now().Unix(), time.Now().Unix())
	
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK constraint failed")
}

func TestBridgeReplyOutboxSettlementNoDeliveryStates(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "no-delivery-states")
	defer ts.Close()

	_, err := ts.GetDriver().GetDB().ExecContext(ctx, `
		INSERT INTO bridge_reply_outbox (
			outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at
		) VALUES (?, ?, ?, ?, ?, 'delivered', 0, ?)
	`, uuid.NewString(), tenant.ID, session.SessionID, handoff.HandoffID, uuid.NewString(), time.Now().Unix())
	
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK constraint failed")
}

func TestBridgeReplyOutboxSettlementNoInvalidMetadataCombinations(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "metadata-constraint")
	defer ts.Close()

	_, err := ts.GetDriver().GetDB().ExecContext(ctx, `
		INSERT INTO bridge_reply_outbox (
			outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at,
			claim_token, claimed_by, claimed_at, claim_expires_at, failed_at, failure_code
		) VALUES (?, ?, ?, ?, ?, 'failed', 0, ?, ?, 'worker', ?, ?, ?, 'ERR')
	`, uuid.NewString(), tenant.ID, session.SessionID, handoff.HandoffID, uuid.NewString(), time.Now().Unix(), uuid.NewString(), time.Now().Unix(), time.Now().Unix()+300, time.Now().Unix())
	
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK constraint failed") // failure_message is NULL
}

func TestBridgeReplyOutboxConcurrentCompleteAndFailOnlyOneWins(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "concurrent-win")
	defer ts.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
			TenantID:   tenant.ID,
			OutboxID:   claimed.OutboxID,
			ClaimToken: *claimed.ClaimToken,
			Now:        time.Now().Unix(),
		})
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
			TenantID:       tenant.ID,
			OutboxID:       claimed.OutboxID,
			ClaimToken:     *claimed.ClaimToken,
			Now:            time.Now().Unix(),
			FailureCode:    "TEST",
			FailureMessage: "Test",
		})
		errs <- err
	}()

	wg.Wait()
	close(errs)

	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, store.ErrBridgeOutboxConflict) {
			conflicts++
		} else {
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func TestBridgeReplyOutboxConcurrentDuplicateCompleteIdempotent(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "concurrent-idem-complete")
	defer ts.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	now := time.Now().Unix()
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, err := ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
				TenantID:   tenant.ID,
				OutboxID:   claimed.OutboxID,
				ClaimToken: *claimed.ClaimToken,
				Now:        now,
			})
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

func TestBridgeReplyOutboxConcurrentDuplicateFailIdempotent(t *testing.T) {
	ctx, ts, tenant, claimed := createClaimedOutboxFixture(t, "concurrent-idem-fail")
	defer ts.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	now := time.Now().Unix()
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, err := ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
				TenantID:       tenant.ID,
				OutboxID:       claimed.OutboxID,
				ClaimToken:     *claimed.ClaimToken,
				Now:            now,
				FailureCode:    "TEST",
				FailureMessage: "Test",
			})
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

func TestBridgeReplyOutboxSettlementRejectsCompletedBeforeClaimed(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "completed-before")
	defer ts.Close()

	now := time.Now().Unix()
	_, err := ts.GetDriver().GetDB().ExecContext(ctx, `
		INSERT INTO bridge_reply_outbox (
			outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at,
			claim_token, claimed_by, claimed_at, claim_expires_at, completed_at
		) VALUES (?, ?, ?, ?, ?, 'completed', 0, ?, ?, 'worker', ?, ?, ?)
	`, uuid.NewString(), tenant.ID, session.SessionID, handoff.HandoffID, uuid.NewString(), now, uuid.NewString(), now, now+300, now-1)
	
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK constraint failed")
}

func TestBridgeReplyOutboxSettlementRejectsFailedBeforeClaimed(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "failed-before")
	defer ts.Close()

	now := time.Now().Unix()
	_, err := ts.GetDriver().GetDB().ExecContext(ctx, `
		INSERT INTO bridge_reply_outbox (
			outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at,
			claim_token, claimed_by, claimed_at, claim_expires_at, failed_at, failure_code, failure_message
		) VALUES (?, ?, ?, ?, ?, 'failed', 0, ?, ?, 'worker', ?, ?, ?, 'ERR', 'msg')
	`, uuid.NewString(), tenant.ID, session.SessionID, handoff.HandoffID, uuid.NewString(), now, uuid.NewString(), now, now+300, now-1)
	
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK constraint failed")
}
