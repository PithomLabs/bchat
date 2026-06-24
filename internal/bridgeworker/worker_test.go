package bridgeworker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

type mockOutboxStore struct {
	claimFn    func(ctx context.Context, tenantID int32, limit int, claimedBy string, now time.Time, claimDurationSeconds int64) ([]*store.BridgeReplyOutbox, error)
	completeFn func(ctx context.Context, complete *store.CompleteBridgeReplyOutbox) (*store.BridgeReplyOutbox, error)
	failFn     func(ctx context.Context, fail *store.FailBridgeReplyOutbox) (*store.BridgeReplyOutbox, error)
}

func (m *mockOutboxStore) ClaimPendingBridgeReplyOutbox(ctx context.Context, tenantID int32, limit int, claimedBy string, now time.Time, claimDurationSeconds int64) ([]*store.BridgeReplyOutbox, error) {
	if m.claimFn != nil {
		return m.claimFn(ctx, tenantID, limit, claimedBy, now, claimDurationSeconds)
	}
	return nil, nil
}

func (m *mockOutboxStore) CompleteClaimedBridgeReplyOutbox(ctx context.Context, complete *store.CompleteBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
	if m.completeFn != nil {
		return m.completeFn(ctx, complete)
	}
	return nil, nil
}

func (m *mockOutboxStore) FailClaimedBridgeReplyOutbox(ctx context.Context, fail *store.FailBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
	if m.failFn != nil {
		return m.failFn(ctx, fail)
	}
	return nil, nil
}

func TestBridgeWorkerRunOnceInvalidConfigRejected(t *testing.T) {
	s := &mockOutboxStore{}
	a := &StaticFakeAdapter{}

	cases := []struct {
		name string
		cfg  WorkerConfig
	}{
		{"zero tenant", WorkerConfig{TenantID: 0, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}},
		{"negative tenant", WorkerConfig{TenantID: -1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}},
		{"zero limit", WorkerConfig{TenantID: 1, ClaimLimit: 0, ClaimedBy: "worker", ClaimDurationSeconds: 60}},
		{"over limit", WorkerConfig{TenantID: 1, ClaimLimit: 101, ClaimedBy: "worker", ClaimDurationSeconds: 60}},
		{"empty claimedBy", WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "", ClaimDurationSeconds: 60}},
		{"unsafe claimedBy", WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker\n", ClaimDurationSeconds: 60}},
		{"zero claim duration", WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 0}},
		{"negative claim duration", WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: -10}},
		{"negative max rows", WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60, MaxRowsPerRun: -1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewWorker(tc.cfg, s, a)
			require.ErrorIs(t, err, ErrInvalidWorkerConfig)
		})
	}
}

func TestBridgeWorkerRunOnceClaimsAndCompletesWithFakeSuccess(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	token := "token-1"
	rows := []*store.BridgeReplyOutbox{
		{OutboxID: "outbox-1", ClaimToken: &token},
	}

	claimCalled := false
	completeCalled := false

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			claimCalled = true
			require.Equal(t, int32(1), tenantID)
			require.Equal(t, 5, limit)
			require.Equal(t, "worker", claimedBy)
			require.Equal(t, now, n)
			require.Equal(t, int64(60), dur)
			return rows, nil
		},
		completeFn: func(c context.Context, comp *store.CompleteBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
			completeCalled = true
			require.Equal(t, int32(1), comp.TenantID)
			require.Equal(t, "outbox-1", comp.OutboxID)
			require.Equal(t, "token-1", comp.ClaimToken)
			require.Equal(t, now.Unix(), comp.Now)
			return &store.BridgeReplyOutbox{Status: "completed"}, nil
		},
	}

	a := &StaticFakeAdapter{Result: AdapterResult{Success: true}}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, now)
	require.NoError(t, err)
	require.True(t, claimCalled)
	require.True(t, completeCalled)
	require.Equal(t, 1, res.ClaimedCount)
	require.Equal(t, 1, res.CompletedCount)
	require.Equal(t, 0, res.FailedCount)
	require.Equal(t, 0, res.ErrorCount)
}

func TestBridgeWorkerRunOnceClaimsAndFailsWithFakeFailure(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	token := "token-1"
	rows := []*store.BridgeReplyOutbox{
		{OutboxID: "outbox-1", ClaimToken: &token},
	}

	failCalled := false

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			return rows, nil
		},
		failFn: func(c context.Context, fail *store.FailBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
			failCalled = true
			require.Equal(t, int32(1), fail.TenantID)
			require.Equal(t, "outbox-1", fail.OutboxID)
			require.Equal(t, "token-1", fail.ClaimToken)
			require.Equal(t, "ERR_CODE", fail.FailureCode)
			require.Equal(t, "some msg", fail.FailureMessage)
			require.Equal(t, now.Unix(), fail.Now)
			return &store.BridgeReplyOutbox{Status: "failed"}, nil
		},
	}

	a := &StaticFakeAdapter{Result: AdapterResult{Success: false, FailureCode: "ERR_CODE", FailureMessage: "some msg"}}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, now)
	require.NoError(t, err)
	require.True(t, failCalled)
	require.Equal(t, 1, res.ClaimedCount)
	require.Equal(t, 0, res.CompletedCount)
	require.Equal(t, 1, res.FailedCount)
	require.Equal(t, 0, res.ErrorCount)
}

func TestBridgeWorkerRunOnceMixedSuccessFailure(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	token1 := "token-1"
	token2 := "token-2"
	rows := []*store.BridgeReplyOutbox{
		{OutboxID: "outbox-1", ClaimToken: &token1},
		{OutboxID: "outbox-2", ClaimToken: &token2},
	}

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			return rows, nil
		},
		completeFn: func(c context.Context, comp *store.CompleteBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
			return &store.BridgeReplyOutbox{Status: "completed"}, nil
		},
		failFn: func(c context.Context, fail *store.FailBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
			return &store.BridgeReplyOutbox{Status: "failed"}, nil
		},
	}

	a := &ScriptedFakeAdapter{
		Results: []AdapterResult{
			{Success: true},
			{Success: false, FailureCode: "CODE", FailureMessage: "msg"},
		},
	}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 2, res.ClaimedCount)
	require.Equal(t, 1, res.CompletedCount)
	require.Equal(t, 1, res.FailedCount)
	require.Equal(t, 0, res.ErrorCount)
}

func TestBridgeWorkerRunOnceNoPendingRows(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			return []*store.BridgeReplyOutbox{}, nil
		},
	}

	a := &StaticFakeAdapter{Result: AdapterResult{Success: true}}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 0, res.ClaimedCount)
	require.Equal(t, 0, res.CompletedCount)
	require.Equal(t, 0, res.FailedCount)
	require.Equal(t, 0, res.ErrorCount)
}

func TestBridgeWorkerRunOnceHonorsClaimLimit(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			require.Equal(t, 2, limit)
			return []*store.BridgeReplyOutbox{}, nil
		},
	}

	a := &StaticFakeAdapter{Result: AdapterResult{Success: true}}

	// When MaxRowsPerRun is set and less than ClaimLimit
	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60, MaxRowsPerRun: 2}, s, a)
	require.NoError(t, err)

	_, err = w.RunOnce(ctx, now)
	require.NoError(t, err)
}

func TestBridgeWorkerRunOnceMaxRowsPerRunZeroFallsBackToClaimLimit(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			require.Equal(t, 5, limit)
			return nil, nil
		},
	}

	a := &StaticFakeAdapter{Result: AdapterResult{Success: true}}
	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60, MaxRowsPerRun: 0}, s, a)
	require.NoError(t, err)

	_, err = w.RunOnce(ctx, now)
	require.NoError(t, err)
}

func TestBridgeWorkerRunOnceHonorsContextCancellationBeforeClaim(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			t.Fatal("Claim should not be called")
			return nil, nil
		},
	}

	a := &StaticFakeAdapter{Result: AdapterResult{Success: true}}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, time.Now())
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, res)
	require.Equal(t, 0, res.ClaimedCount)
	require.Equal(t, 0, res.SkippedCount)
}

func TestBridgeWorkerRunOnceHonorsContextCancellationDuringProcessing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	token1 := "token-1"
	token2 := "token-2"
	rows := []*store.BridgeReplyOutbox{
		{OutboxID: "outbox-1", ClaimToken: &token1},
		{OutboxID: "outbox-2", ClaimToken: &token2},
	}

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			return rows, nil
		},
		completeFn: func(c context.Context, comp *store.CompleteBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
			cancel() // cancel context during first completion
			return &store.BridgeReplyOutbox{Status: "completed"}, nil
		},
	}

	a := &StaticFakeAdapter{Result: AdapterResult{Success: true}}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, time.Now())
	require.NoError(t, err)
	require.Equal(t, 2, res.ClaimedCount)
	require.Equal(t, 1, res.CompletedCount)
	require.Equal(t, 1, res.SkippedCount)
}

func TestBridgeWorkerRunOnceAdapterPanicHandled(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	token := "token-1"
	rows := []*store.BridgeReplyOutbox{
		{OutboxID: "outbox-1", ClaimToken: &token},
	}

	failCalled := false

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			return rows, nil
		},
		failFn: func(c context.Context, fail *store.FailBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
			failCalled = true
			require.Equal(t, "adapter_panic", fail.FailureCode)
			require.Contains(t, fail.FailureMessage, "something went wrong")
			return &store.BridgeReplyOutbox{Status: "failed"}, nil
		},
	}

	a := &PanicFakeAdapter{Message: "something went wrong"}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, now)
	require.NoError(t, err)
	require.True(t, failCalled)
	require.Equal(t, 1, res.ClaimedCount)
	require.Equal(t, 1, res.FailedCount)
	require.Equal(t, 1, res.ErrorCount)
}

func TestBridgeWorkerRunOnceSettlementFailureRecorded(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	token := "token-1"
	rows := []*store.BridgeReplyOutbox{
		{OutboxID: "outbox-1", ClaimToken: &token},
	}

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			return rows, nil
		},
		completeFn: func(c context.Context, comp *store.CompleteBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
			return nil, errors.New("db completion error")
		},
	}

	a := &StaticFakeAdapter{Result: AdapterResult{Success: true}}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 1, res.ClaimedCount)
	require.Equal(t, 0, res.CompletedCount)
	require.Equal(t, 1, res.ErrorCount)
	require.Len(t, res.Errors, 1)
	require.Contains(t, res.Errors[0], "db completion error")
}

func TestBridgeWorkerRunOnceDoesNotExposeClaimTokenInErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	token := "sensitive-token-12345"
	rows := []*store.BridgeReplyOutbox{
		{OutboxID: "outbox-1", ClaimToken: &token},
	}

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			return rows, nil
		},
		completeFn: func(c context.Context, comp *store.CompleteBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
			return nil, fmt.Errorf("db complete error for token %s", comp.ClaimToken)
		},
	}

	a := &StaticFakeAdapter{Result: AdapterResult{Success: true}}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, now)
	require.NoError(t, err)
	require.Len(t, res.Errors, 1)
	// Redact error messages or do not expose the token in the worker's errors
	// In our processRow implementation:
	// result.Errors = append(result.Errors, fmt.Sprintf("failed to complete outbox %s: %v", row.OutboxID, err))
	// Wait, the db completion error contains the token because comp.ClaimToken is printed.
	// To prevent this, the worker must scrub or not include comp.ClaimToken in errors,
	// or we can redact/clean it up or avoid printing raw db errors if they contain secrets.
	// Actually, w.processRow does: fmt.Sprintf("failed to complete outbox %s: %v", row.OutboxID, err)
	// If the DB error itself contains the token, it could be exposed.
	// So let's make sure the test verifies we do not expose "sensitive-token-12345" in RunResult.Errors.
	for _, e := range res.Errors {
		require.NotContains(t, e, token)
	}
}

func TestBridgeWorkerRunOnceInvalidAdapterFailureMetadataSettlesAsAdapterInvalidResult(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	token := "token-1"
	rows := []*store.BridgeReplyOutbox{
		{OutboxID: "outbox-1", ClaimToken: &token},
	}

	failCalled := false

	s := &mockOutboxStore{
		claimFn: func(c context.Context, tenantID int32, limit int, claimedBy string, n time.Time, dur int64) ([]*store.BridgeReplyOutbox, error) {
			return rows, nil
		},
		failFn: func(c context.Context, fail *store.FailBridgeReplyOutbox) (*store.BridgeReplyOutbox, error) {
			failCalled = true
			require.Equal(t, "adapter_invalid_result", fail.FailureCode)
			require.Equal(t, "fake adapter returned invalid failure metadata", fail.FailureMessage)
			return &store.BridgeReplyOutbox{Status: "failed"}, nil
		},
	}

	// Invalid failure code contains spaces, which is forbidden under store rules
	a := &StaticFakeAdapter{Result: AdapterResult{Success: false, FailureCode: "INVALID CODE", FailureMessage: "some msg"}}

	w, err := NewWorker(WorkerConfig{TenantID: 1, ClaimLimit: 5, ClaimedBy: "worker", ClaimDurationSeconds: 60}, s, a)
	require.NoError(t, err)

	res, err := w.RunOnce(ctx, now)
	require.NoError(t, err)
	require.True(t, failCalled)
	require.Equal(t, 1, res.ClaimedCount)
	require.Equal(t, 0, res.CompletedCount)
	require.Equal(t, 1, res.FailedCount)
	require.Equal(t, 1, res.ErrorCount)
}

func TestBridgeWorkerPackageDoesNotImportTransportOrTicketSystems(t *testing.T) {
	content, err := os.ReadFile("worker.go")
	require.NoError(t, err)

	s := string(content)

	forbidden := []string{
		"net/http",
		"http.Client",
		"os.WriteFile",
		"hermes",
		"telegram",
		"CreateTicket",
		"UpdateTicket",
		"ChatExternal",
		"processChat",
	}

	for _, term := range forbidden {
		if strings.Contains(strings.ToLower(s), strings.ToLower(term)) {
			t.Errorf("worker.go contains forbidden term/concept: %q", term)
		}
	}
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func createBridgeTenant(t *testing.T, ctx context.Context, ts *store.Store, slug string) *store.AgentTenant {
	t.Helper()
	tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{Slug: slug, CompanyName: slug, Vertical: "test", IsActive: true})
	require.NoError(t, err)
	return tenant
}

func createHandoffFixture(t *testing.T, ctx context.Context, ts *store.Store, slug string) (*store.AgentTenant, *store.BridgeExternalSession, *store.BridgeHandoff) {
	t.Helper()
	tenant := createBridgeTenant(t, ctx, ts, slug)
	now := time.Now()
	session, _, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session-1", now, now.Add(time.Hour))
	require.NoError(t, err)
	handoff, err := ts.CreateBridgeHandoff(ctx, tenant.ID, session.SessionID, now)
	require.NoError(t, err)
	handoff, err = ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, handoff.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive, "accepted", now)
	require.NoError(t, err)
	return tenant, session, handoff
}

func TestBridgeWorkerIntegrationPendingToCompleted(t *testing.T) {
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	tenant, session, handoff := createHandoffFixture(t, ctx, ts, "integration-complete")

	resBoth, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello completion",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	w, err := NewWorker(WorkerConfig{TenantID: tenant.ID, ClaimLimit: 5, ClaimedBy: "worker-int", ClaimDurationSeconds: 60}, ts, &StaticFakeAdapter{Result: AdapterResult{Success: true}})
	require.NoError(t, err)

	runRes, err := w.RunOnce(ctx, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, runRes.ClaimedCount)
	require.Equal(t, 1, runRes.CompletedCount)
	require.Equal(t, 0, runRes.FailedCount)
	require.Equal(t, 0, runRes.ErrorCount)

	// Verify DB state
	outbox, err := ts.GetBridgeReplyOutboxByReplyID(ctx, tenant.ID, resBoth.Reply.ReplyID)
	require.NoError(t, err)
	require.Equal(t, "completed", outbox.Status)
}

func TestBridgeWorkerIntegrationPendingToFailed(t *testing.T) {
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	tenant, session, handoff := createHandoffFixture(t, ctx, ts, "integration-fail")

	resBoth, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello failure",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	w, err := NewWorker(WorkerConfig{TenantID: tenant.ID, ClaimLimit: 5, ClaimedBy: "worker-int", ClaimDurationSeconds: 60}, ts, &StaticFakeAdapter{Result: AdapterResult{Success: false, FailureCode: "ERR_DELIVERY", FailureMessage: "Failed to deliver"}})
	require.NoError(t, err)

	runRes, err := w.RunOnce(ctx, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, runRes.ClaimedCount)
	require.Equal(t, 0, runRes.CompletedCount)
	require.Equal(t, 1, runRes.FailedCount)
	require.Equal(t, 0, runRes.ErrorCount)

	// Verify DB state
	outbox, err := ts.GetBridgeReplyOutboxByReplyID(ctx, tenant.ID, resBoth.Reply.ReplyID)
	require.NoError(t, err)
	require.Equal(t, "failed", outbox.Status)
	require.Equal(t, "ERR_DELIVERY", *outbox.FailureCode)
	require.Equal(t, "Failed to deliver", *outbox.FailureMessage)
}

func TestBridgeWorkerIntegrationNoDoubleClaimUnderConcurrentRunOnce(t *testing.T) {
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	tenant, session, handoff := createHandoffFixture(t, ctx, ts, "integration-concurrent")

	// Insert 10 outbox rows
	for i := 0; i < 10; i++ {
		_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
			ReplyID:         uuid.NewString(),
			TenantID:        tenant.ID,
			SessionID:       session.SessionID,
			HandoffID:       handoff.HandoffID,
			ClientMessageID: fmt.Sprintf("msg-%d", i),
			Text:            fmt.Sprintf("Hello %d", i),
			Now:             time.Now().Unix(),
		})
		require.NoError(t, err)
	}

	w1, err := NewWorker(WorkerConfig{TenantID: tenant.ID, ClaimLimit: 5, ClaimedBy: "worker-1", ClaimDurationSeconds: 60}, ts, &StaticFakeAdapter{Result: AdapterResult{Success: true}})
	require.NoError(t, err)
	w2, err := NewWorker(WorkerConfig{TenantID: tenant.ID, ClaimLimit: 5, ClaimedBy: "worker-2", ClaimDurationSeconds: 60}, ts, &StaticFakeAdapter{Result: AdapterResult{Success: true}})
	require.NoError(t, err)

	var wg sync.WaitGroup
	var r1, r2 *RunResult
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		r1, err1 = w1.RunOnce(ctx, time.Now())
	}()
	go func() {
		defer wg.Done()
		r2, err2 = w2.RunOnce(ctx, time.Now())
	}()

	wg.Wait()

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Since limit is 5, and total is 10, both should claim exactly 5 rows without overlaps
	require.Equal(t, 5, r1.ClaimedCount)
	require.Equal(t, 5, r2.ClaimedCount)
	require.Equal(t, 5, r1.CompletedCount)
	require.Equal(t, 5, r2.CompletedCount)
}

func TestBridgeWorkerIntegrationCompletedFailedTerminalStatesPreserved(t *testing.T) {
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	tenant, session, handoff := createHandoffFixture(t, ctx, ts, "integration-terminal")

	resBoth1, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello terminal 1",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	resBoth2, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-2",
		Text:            "Hello terminal 2",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	// Settle first outbox manually as completed, second outbox as failed
	claimed, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 2, "manual-claim", time.Now(), 60)
	require.NoError(t, err)
	require.Len(t, claimed, 2)

	var ob1, ob2 *store.BridgeReplyOutbox
	if claimed[0].ReplyID == resBoth1.Reply.ReplyID {
		ob1 = claimed[0]
		ob2 = claimed[1]
	} else {
		ob1 = claimed[1]
		ob2 = claimed[0]
	}

	_, err = ts.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenant.ID,
		OutboxID:   ob1.OutboxID,
		ClaimToken: *ob1.ClaimToken,
		Now:        time.Now().Unix(),
	})
	require.NoError(t, err)

	_, err = ts.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
		TenantID:       tenant.ID,
		OutboxID:       ob2.OutboxID,
		ClaimToken:     *ob2.ClaimToken,
		Now:            time.Now().Unix(),
		FailureCode:    "MANUAL_ERR",
		FailureMessage: "Manual failure",
	})
	require.NoError(t, err)

	// Run worker. Since both outbox rows are completed/failed, claiming should find 0 pending rows.
	w, err := NewWorker(WorkerConfig{TenantID: tenant.ID, ClaimLimit: 5, ClaimedBy: "worker-int", ClaimDurationSeconds: 60}, ts, &StaticFakeAdapter{Result: AdapterResult{Success: true}})
	require.NoError(t, err)

	runRes, err := w.RunOnce(ctx, time.Now())
	require.NoError(t, err)
	require.Equal(t, 0, runRes.ClaimedCount)

	// Assert they remain in their terminal states
	dbOb1, err := ts.GetBridgeReplyOutboxByReplyID(ctx, tenant.ID, resBoth1.Reply.ReplyID)
	require.NoError(t, err)
	require.Equal(t, "completed", dbOb1.Status)

	dbOb2, err := ts.GetBridgeReplyOutboxByReplyID(ctx, tenant.ID, resBoth2.Reply.ReplyID)
	require.NoError(t, err)
	require.Equal(t, "failed", dbOb2.Status)
}
