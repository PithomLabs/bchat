package teststore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func createBridgeTenant(t *testing.T, ctx context.Context, ts *store.Store, slug string) *store.AgentTenant {
	t.Helper()
	tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{Slug: slug, CompanyName: slug, Vertical: "test", IsActive: true})
	require.NoError(t, err)
	return tenant
}

func TestBridgeExternalSessionMigrationAppliesWithForeignKeysEnabled(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	var enabled int
	require.NoError(t, ts.GetDriver().GetDB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled))
	require.Equal(t, 1, enabled)
	for _, table := range []string{"bridge_external_sessions", "bridge_handoffs"} {
		var name string
		require.NoError(t, ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name))
		require.Equal(t, table, name)
	}
}

func TestBridgeExternalSessionUsesAgentTenantsTable(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	_, _, err := ts.EnsureBridgeExternalSession(ctx, 999999, "missing-tenant", time.Now(), time.Now().Add(time.Minute))
	require.Error(t, err)
}

func TestEnsureBridgeExternalSessionReturnsCreatedFlag(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "bridge-created")
	now := time.Now().Truncate(time.Second)
	first, created, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session-1", now, now.Add(time.Minute))
	require.NoError(t, err)
	require.True(t, created)
	second, created, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session-1", now.Add(time.Second), now.Add(2*time.Minute))
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, now.Add(time.Second), second.UpdatedAt)
}

func TestEnsureBridgeExternalSessionIsIdempotentPerTenantSession(t *testing.T) {
	TestEnsureBridgeExternalSessionReturnsCreatedFlag(t)
}

func TestSameSessionIDDifferentTenantsCreatesDifferentRows(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	a := createBridgeTenant(t, ctx, ts, "bridge-a")
	b := createBridgeTenant(t, ctx, ts, "bridge-b")
	now := time.Now()
	rowA, _, err := ts.EnsureBridgeExternalSession(ctx, a.ID, "shared-session", now, now.Add(time.Minute))
	require.NoError(t, err)
	rowB, _, err := ts.EnsureBridgeExternalSession(ctx, b.ID, "shared-session", now, now.Add(time.Minute))
	require.NoError(t, err)
	require.NotEqual(t, rowA.ID, rowB.ID)
}

func TestConcurrentEnsureBridgeExternalSession(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "bridge-concurrent-ensure")
	now := time.Now()
	created := make(chan bool, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, wasCreated, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "shared", now, now.Add(time.Minute))
			created <- wasCreated
			errs <- err
		}()
	}
	wg.Wait()
	close(created)
	close(errs)
	createdCount := 0
	for err := range errs {
		require.NoError(t, err)
	}
	for value := range created {
		if value {
			createdCount++
		}
	}
	require.Equal(t, 1, createdCount)
}

func TestBridgeHandoffRequiresDurableExternalSession(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "bridge-parent-required")
	_, err := ts.CreateBridgeHandoff(ctx, tenant.ID, "missing", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeExternalSessionNotFound)
}

func TestBridgeHandoffUsesSyntheticExternalSessionID(t *testing.T) {
	ctx, ts, tenant, session, handoff := createBridgeHandoffFixture(t, "bridge-synthetic")
	defer ts.Close()
	require.NotZero(t, handoff.ID)
	require.Equal(t, session.ID, handoff.ExternalSessionID)
	found, err := ts.FindActiveBridgeHandoff(ctx, tenant.ID, session.SessionID)
	require.NoError(t, err)
	require.Equal(t, handoff.ID, found.ID)
}

func TestCreateBridgeHandoffCreatesQueuedFoundationRow(t *testing.T) {
	_, ts, _, _, handoff := createBridgeHandoffFixture(t, "bridge-queued")
	defer ts.Close()
	require.Equal(t, store.BridgeRoutingModeHandoffQueued, handoff.RoutingMode)
	require.True(t, handoff.Active)
	require.Nil(t, handoff.Outcome)
	require.Equal(t, 1, handoff.Version)
}

func TestAbsenceOfActiveHandoffMeansAIRouting(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "bridge-ai-by-absence")
	now := time.Now()
	_, _, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session", now, now.Add(time.Minute))
	require.NoError(t, err)
	handoff, err := ts.FindActiveBridgeHandoff(ctx, tenant.ID, "session")
	require.NoError(t, err)
	require.Nil(t, handoff)
}

func TestHandoffCreationDoesNotUseActiveAIRow(t *testing.T) {
	_, ts, _, _, handoff := createBridgeHandoffFixture(t, "bridge-no-ai")
	defer ts.Close()
	require.NotEqual(t, store.BridgeRoutingMode("ai"), handoff.RoutingMode)
}

func TestPartialUniqueIndexRejectsSecondActiveHandoff(t *testing.T) {
	ctx, ts, tenant, session, _ := createBridgeHandoffFixture(t, "bridge-one-active")
	defer ts.Close()
	_, err := ts.CreateBridgeHandoff(ctx, tenant.ID, session.SessionID, time.Now())
	require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
}

func TestBridgeHandoffOneActivePerExternalSession(t *testing.T) {
	TestPartialUniqueIndexRejectsSecondActiveHandoff(t)
}

func TestBridgeHandoffGenerationUniquePerTenantSession(t *testing.T) {
	ctx, ts, tenant, session, first := createBridgeHandoffFixture(t, "bridge-generation")
	defer ts.Close()
	_, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, first.Generation, first.HandoffID, first.Version, first.RoutingMode, store.BridgeRoutingModeClosed, "done", time.Now())
	require.NoError(t, err)
	second, err := ts.CreateBridgeHandoff(ctx, tenant.ID, session.SessionID, time.Now())
	require.NoError(t, err)
	require.Equal(t, first.Generation+1, second.Generation)
}

func TestClosingHandoffAllowsNewActiveHandoff(t *testing.T) {
	TestBridgeHandoffGenerationUniquePerTenantSession(t)
}

func TestBridgeHandoffCASUpdateSucceedsWithExpectedVersion(t *testing.T) {
	ctx, ts, tenant, session, handoff := createBridgeHandoffFixture(t, "bridge-cas-ok")
	defer ts.Close()
	updated, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, 1, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive, "accepted", time.Now())
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)
	require.Equal(t, store.BridgeRoutingModeHumanActive, updated.RoutingMode)
}

func TestBridgeHandoffCASUpdateFailsWithStaleVersion(t *testing.T) {
	ctx, ts, tenant, session, handoff := createBridgeHandoffFixture(t, "bridge-cas-stale")
	defer ts.Close()
	_, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, 99, handoff.RoutingMode, store.BridgeRoutingModeHumanActive, "stale", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
}

func TestBridgeHandoffCASWrongModeFails(t *testing.T) {
	ctx, ts, tenant, session, handoff := createBridgeHandoffFixture(t, "bridge-cas-mode")
	defer ts.Close()
	_, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, handoff.Version, store.BridgeRoutingModeHumanActive, store.BridgeRoutingModeClosed, "wrong", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
}

func TestBridgeHandoffCASWrongGenerationFails(t *testing.T) {
	ctx, ts, tenant, session, handoff := createBridgeHandoffFixture(t, "bridge-cas-generation")
	defer ts.Close()
	_, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation+1, handoff.HandoffID, handoff.Version, handoff.RoutingMode, store.BridgeRoutingModeClosed, "wrong", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
}

func TestBridgeHandoffCASWrongTenantFails(t *testing.T) {
	ctx, ts, _, session, handoff := createBridgeHandoffFixture(t, "bridge-cas-tenant")
	defer ts.Close()
	other := createBridgeTenant(t, ctx, ts, "bridge-cas-other")
	_, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, other.ID, session.SessionID, handoff.Generation, handoff.HandoffID, handoff.Version, handoff.RoutingMode, store.BridgeRoutingModeClosed, "wrong", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
}

func TestCASClosedHandoffRejected(t *testing.T) {
	ctx, ts, tenant, session, handoff := createBridgeHandoffFixture(t, "bridge-cas-closed")
	defer ts.Close()
	closed, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, handoff.Version, handoff.RoutingMode, store.BridgeRoutingModeClosed, "closed", time.Now())
	require.NoError(t, err)
	_, err = ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, closed.Version, store.BridgeRoutingModeClosed, store.BridgeRoutingModeClosed, "again", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
}

func TestSQLiteBridgeHandoffActiveBooleanCheck(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "bridge-active-check")
	now := time.Now()
	session, _, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session", now, now.Add(time.Minute))
	require.NoError(t, err)
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, `INSERT INTO bridge_handoffs
		(external_session_id, handoff_id, tenant_id, session_id, generation, routing_mode, active, version, created_at, updated_at)
		VALUES (?, 'bad-active', ?, 'session', 1, 'handoff_queued', 2, 1, ?, ?)`, session.ID, tenant.ID, now.Unix(), now.Unix())
	require.Error(t, err)
}

func TestSQLiteFKCascadeOnTenantDeletion(t *testing.T) {
	ctx, ts, tenant, _, _ := createBridgeHandoffFixture(t, "bridge-cascade")
	defer ts.Close()
	require.NoError(t, ts.DeleteAgentTenant(ctx, tenant.ID))
	for _, table := range []string{"bridge_external_sessions", "bridge_handoffs"} {
		var count int
		require.NoError(t, ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM "+table).Scan(&count))
		require.Zero(t, count)
	}
}

func TestConcurrentCreateHandoffGenerationRace(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "bridge-concurrent-handoff")
	now := time.Now()
	_, _, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session", now, now.Add(time.Minute))
	require.NoError(t, err)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ts.CreateBridgeHandoff(ctx, tenant.ID, "session", time.Now())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, store.ErrBridgeHandoffConflict) {
			conflicts++
		} else {
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func createBridgeHandoffFixture(t *testing.T, slug string) (context.Context, *store.Store, *store.AgentTenant, *store.BridgeExternalSession, *store.BridgeHandoff) {
	t.Helper()
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	tenant := createBridgeTenant(t, ctx, ts, slug)
	now := time.Now()
	session, _, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session", now, now.Add(time.Minute))
	require.NoError(t, err)
	handoff, err := ts.CreateBridgeHandoff(ctx, tenant.ID, session.SessionID, now)
	require.NoError(t, err)
	return ctx, ts, tenant, session, handoff
}

func createHumanActiveHandoffFixture(t *testing.T, slug string) (context.Context, *store.Store, *store.AgentTenant, *store.BridgeExternalSession, *store.BridgeHandoff) {
	ctx, ts, tenant, session, handoff := createBridgeHandoffFixture(t, slug)
	updated, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, handoff.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive, "accepted", time.Now())
	require.NoError(t, err)
	return ctx, ts, tenant, session, updated
}

func TestCreateBridgeHandoffReplyIfActivePersists(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "reply-persist")
	defer ts.Close()

	now := time.Now().Unix()
	reply, err := ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "reply-1",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello world",
		Now:             now,
	})
	require.NoError(t, err)
	require.NotNil(t, reply)
	require.Equal(t, "reply-1", reply.ReplyID)
	require.Equal(t, tenant.ID, reply.TenantID)
	require.Equal(t, session.SessionID, reply.SessionID)
	require.Equal(t, handoff.HandoffID, reply.HandoffID)
	require.Equal(t, int64(handoff.Generation), reply.Generation)
	require.Equal(t, "msg-1", reply.ClientMessageID)
	require.Equal(t, "Hello world", reply.Text)
	require.Equal(t, "not_delivered", reply.DeliveryStatus)
	require.Equal(t, now, reply.CreatedAt)

	// Retrieve it directly from db to ensure durability
	var count int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_handoff_replies WHERE reply_id = ?", "reply-1").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestCreateBridgeHandoffReplyIfActiveRequiresHumanActive(t *testing.T) {
	ctx, ts, tenant, session, handoff := createBridgeHandoffFixture(t, "reply-queued")
	defer ts.Close()

	// Currently the handoff is handoff_queued, not human_active
	_, err := ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "reply-1",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello",
		Now:             time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
}

func TestCreateBridgeHandoffReplyIfActiveRejectsClosed(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "reply-closed")
	defer ts.Close()

	// Update CAS to closed
	_, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, handoff.Version, store.BridgeRoutingModeHumanActive, store.BridgeRoutingModeClosed, "test-close", time.Now())
	require.NoError(t, err)

	_, err = ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "reply-1",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello",
		Now:             time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
}

func TestCreateBridgeHandoffReplyIfActiveIdempotentSameText(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "reply-idem-same")
	defer ts.Close()

	now := time.Now().Unix()
	reply1, err := ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "reply-1",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Same Text",
		Now:             now,
	})
	require.NoError(t, err)

	// Call again with same client_message_id and same text, but a different reply_id
	reply2, err := ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "reply-2",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Same Text",
		Now:             now + 10,
	})
	require.NoError(t, err)
	require.Equal(t, reply1.ReplyID, reply2.ReplyID) // Must return the original reply_id ("reply-1")
	require.Equal(t, reply1.Text, reply2.Text)
	require.Equal(t, reply1.CreatedAt, reply2.CreatedAt) // Original creation time preserved
}

func TestCreateBridgeHandoffReplyIfActiveRejectsDifferentText(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "reply-idem-diff")
	defer ts.Close()

	_, err := ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "reply-1",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Original Text",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	_, err = ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "reply-2",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Different Text",
		Now:             time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeHandoffReplyTextMismatch)
}

func TestCreateBridgeHandoffReplyIfActiveTenantIsolation(t *testing.T) {
	ctx, ts, _, session1, handoff1 := createHumanActiveHandoffFixture(t, "tenant-1")
	defer ts.Close()

	// Create second tenant
	tenant2 := createBridgeTenant(t, ctx, ts, "tenant-2")
	session2, _, err := ts.EnsureBridgeExternalSession(ctx, tenant2.ID, "session", time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	handoff2, err := ts.CreateBridgeHandoff(ctx, tenant2.ID, session2.SessionID, time.Now())
	require.NoError(t, err)
	handoff2, err = ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant2.ID, session2.SessionID, handoff2.Generation, handoff2.HandoffID, handoff2.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive, "accepted", time.Now())
	require.NoError(t, err)

	// Attempt to create a reply using tenant2's ID but tenant1's handoffID/sessionID
	_, err = ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "reply-1",
		TenantID:        tenant2.ID,
		SessionID:       session1.SessionID,
		HandoffID:       handoff1.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello",
		Now:             time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeHandoffNotFound)
}

func TestCreateBridgeHandoffReplyIfActiveSessionIsolation(t *testing.T) {
	ctx, ts, tenant, _, handoff1 := createHumanActiveHandoffFixture(t, "session-1")
	defer ts.Close()

	// Create another session for the same tenant
	session2, _, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session-other", time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	handoff2, err := ts.CreateBridgeHandoff(ctx, tenant.ID, session2.SessionID, time.Now())
	require.NoError(t, err)
	handoff2, err = ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session2.SessionID, handoff2.Generation, handoff2.HandoffID, handoff2.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive, "accepted", time.Now())
	require.NoError(t, err)

	// Attempt to write reply for handoff1 but using session2's sessionID
	_, err = ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "reply-1",
		TenantID:        tenant.ID,
		SessionID:       session2.SessionID,
		HandoffID:       handoff1.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello",
		Now:             time.Now().Unix(),
	})
	require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
}

func TestCreateBridgeHandoffReplyIfActiveRaceWithRelease(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "reply-race-release")
	defer ts.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, handoff.Version, store.BridgeRoutingModeHumanActive, store.BridgeRoutingModeClosed, "released", time.Now())
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
			ReplyID:         "reply-1",
			TenantID:        tenant.ID,
			SessionID:       session.SessionID,
			HandoffID:       handoff.HandoffID,
			ClientMessageID: "msg-1",
			Text:            "Hello world",
			Now:             time.Now().Unix(),
		})
		errs <- err
	}()

	wg.Wait()
	close(errs)

	hasNoError := false
	for err := range errs {
		if err == nil {
			hasNoError = true
		} else {
			require.ErrorIs(t, err, store.ErrBridgeHandoffConflict)
		}
	}
	require.True(t, hasNoError)
}

func TestCreateBridgeHandoffReplyIfActiveDuplicateMessageIDRace(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "reply-race-msg")
	defer ts.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(id int) {
			defer wg.Done()
			_, err := ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
				ReplyID:         "reply-id",
				TenantID:        tenant.ID,
				SessionID:       session.SessionID,
				HandoffID:       handoff.HandoffID,
				ClientMessageID: "msg-shared",
				Text:            "Hello world",
				Now:             time.Now().Unix(),
			})
			errs <- err
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

func TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "reply-delivery-constraint")
	defer ts.Close()

	_, err := ts.GetDriver().GetDB().ExecContext(ctx, `
		INSERT INTO bridge_handoff_replies (
			reply_id, tenant_id, session_id, handoff_id, generation,
			client_message_id, text, delivery_status, created_at
		) VALUES ('reply-fail', ?, ?, ?, ?, 'msg-fail', 'some text', 'delivered', ?)
	`, tenant.ID, session.SessionID, handoff.HandoffID, handoff.Generation, time.Now().Unix())

	require.Error(t, err)
	require.Contains(t, err.Error(), "constraint failed")
}

func TestCreateBridgeHandoffReplyAndOutboxIfActivePersistsBoth(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "persist-both")
	defer ts.Close()

	now := time.Now().Unix()
	res, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "11111111-1111-1111-1111-111111111111",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello world",
		Now:             now,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.Reply)
	require.NotNil(t, res.Outbox)

	require.Equal(t, "11111111-1111-1111-1111-111111111111", res.Reply.ReplyID)
	require.Equal(t, "msg-1", res.Reply.ClientMessageID)
	require.Equal(t, "Hello world", res.Reply.Text)
	require.Equal(t, "not_delivered", res.Reply.DeliveryStatus)

	require.NotEmpty(t, res.Outbox.OutboxID)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", res.Outbox.ReplyID)
	require.Equal(t, "pending", res.Outbox.Status)
	require.Equal(t, 0, res.Outbox.AttemptCount)
	require.Equal(t, now, res.Outbox.CreatedAt)

	// Verify directly from SQLite
	var replyCount, outboxCount int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_handoff_replies WHERE reply_id = ?", "11111111-1111-1111-1111-111111111111").Scan(&replyCount)
	require.NoError(t, err)
	require.Equal(t, 1, replyCount)

	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_reply_outbox WHERE reply_id = ?", "11111111-1111-1111-1111-111111111111").Scan(&outboxCount)
	require.NoError(t, err)
	require.Equal(t, 1, outboxCount)
}

func TestCreateBridgeHandoffReplyAndOutboxIfActiveIdempotentSameTextSameOutbox(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "idempotent-both")
	defer ts.Close()

	now := time.Now().Unix()
	res1, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "11111111-1111-1111-1111-111111111111",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello world",
		Now:             now,
	})
	require.NoError(t, err)

	res2, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "22222222-2222-2222-2222-222222222222", // different reply ID proposed
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello world",
		Now:             now + 10,
	})
	require.NoError(t, err)

	require.Equal(t, res1.Reply.ReplyID, res2.Reply.ReplyID)
	require.Equal(t, res1.Outbox.OutboxID, res2.Outbox.OutboxID)
	require.Equal(t, res1.Outbox.CreatedAt, res2.Outbox.CreatedAt)

	// Verify directly from SQLite
	var replyCount, outboxCount int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_handoff_replies WHERE client_message_id = ?", "msg-1").Scan(&replyCount)
	require.NoError(t, err)
	require.Equal(t, 1, replyCount)

	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_reply_outbox WHERE tenant_id = ? AND reply_id = ?", tenant.ID, res1.Reply.ReplyID).Scan(&outboxCount)
	require.NoError(t, err)
	require.Equal(t, 1, outboxCount)
}

func TestCreateBridgeHandoffReplyAndOutboxIfActiveDifferentTextConflict(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "text-conflict")
	defer ts.Close()

	now := time.Now().Unix()
	_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "11111111-1111-1111-1111-111111111111",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello world",
		Now:             now,
	})
	require.NoError(t, err)

	_, err = ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "22222222-2222-2222-2222-222222222222",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Different text",
		Now:             now + 10,
	})
	require.ErrorIs(t, err, store.ErrBridgeHandoffReplyTextMismatch)
}

func TestCreateBridgeHandoffReplyAndOutboxIfActiveDuplicateMessageIDRaceSingleReplySingleOutbox(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "race-both")
	defer ts.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func(id int) {
			defer wg.Done()
			_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
				ReplyID:         "77777777-7777-7777-7777-777777777777",
				TenantID:        tenant.ID,
				SessionID:       session.SessionID,
				HandoffID:       handoff.HandoffID,
				ClientMessageID: "msg-race",
				Text:            "Race Text",
				Now:             time.Now().Unix(),
			})
			errs <- err
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	// Verify directly from SQLite
	var replyCount, outboxCount int
	err := ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_handoff_replies WHERE client_message_id = ?", "msg-race").Scan(&replyCount)
	require.NoError(t, err)
	require.Equal(t, 1, replyCount)

	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_reply_outbox WHERE tenant_id = ?", tenant.ID).Scan(&outboxCount)
	require.NoError(t, err)
	require.Equal(t, 1, outboxCount)
}

func TestCreateBridgeHandoffReplyAndOutboxIfActiveRaceWithRelease(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "race-release")
	defer ts.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Millisecond) // brief delay to increase chance of interleaving
		_, err := ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session.SessionID, handoff.Generation, handoff.HandoffID, handoff.Version, store.BridgeRoutingModeHumanActive, store.BridgeRoutingModeClosed, "closed", time.Now())
		errs <- err
	}()

	go func() {
		defer wg.Done()
		_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
			ReplyID:         "11111111-1111-1111-1111-111111111111",
			TenantID:        tenant.ID,
			SessionID:       session.SessionID,
			HandoffID:       handoff.HandoffID,
			ClientMessageID: "msg-1",
			Text:            "Hello world",
			Now:             time.Now().Unix(),
		})
		errs <- err
	}()

	wg.Wait()
	close(errs)

	hasNoError := false
	for err := range errs {
		if err == nil {
			hasNoError = true
		} else {
			require.True(t, errors.Is(err, store.ErrBridgeHandoffConflict) || errors.Is(err, store.ErrBridgeHandoffNotFound))
		}
	}
	require.True(t, hasNoError)
}

func TestCreateBridgeReplyOutboxStatusConstraint(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "outbox-status-constraint")
	defer ts.Close()

	now := time.Now().Unix()
	// Insert reply first
	_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "11111111-1111-1111-1111-111111111111",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello",
		Now:             now,
	})
	require.NoError(t, err)

	// Try inserting outbox directly with invalid status
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, `
		INSERT INTO bridge_reply_outbox (
			outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at
		) VALUES ('ffffffff-ffff-ffff-ffff-ffffffffffff', ?, ?, ?, '11111111-1111-1111-1111-111111111111', 'delivered', 0, ?)
	`, tenant.ID, session.SessionID, handoff.HandoffID, now)

	require.Error(t, err)
	require.Contains(t, err.Error(), "constraint failed")
}

func TestCreateBridgeReplyOutboxAttemptCountConstraint(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "outbox-attempt-constraint")
	defer ts.Close()

	now := time.Now().Unix()
	// Insert reply first
	_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "11111111-1111-1111-1111-111111111111",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello",
		Now:             now,
	})
	require.NoError(t, err)

	// Try inserting outbox directly with invalid attempt_count
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, `
		INSERT INTO bridge_reply_outbox (
			outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at
		) VALUES ('ffffffff-ffff-ffff-ffff-ffffffffffff', ?, ?, ?, '11111111-1111-1111-1111-111111111111', 'pending', 1, ?)
	`, tenant.ID, session.SessionID, handoff.HandoffID, now)

	require.Error(t, err)
	require.Contains(t, err.Error(), "constraint failed")
}

func TestCreateBridgeReplyOutboxTenantIsolation(t *testing.T) {
	ctx, ts, tenant1, session1, handoff1 := createHumanActiveHandoffFixture(t, "tenant-1")
	defer ts.Close()

	// Create a second tenant
	tenant2, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug:        "tenant-2",
		CompanyName: "Tenant 2 Ltd",
	})
	require.NoError(t, err)

	session2, _, err := ts.EnsureBridgeExternalSession(ctx, tenant2.ID, "session-2", time.Now(), time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	handoff2, err := ts.CreateBridgeHandoff(ctx, tenant2.ID, session2.SessionID, time.Now())
	require.NoError(t, err)

	handoff2, err = ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant2.ID, session2.SessionID, handoff2.Generation, handoff2.HandoffID, handoff2.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive, "takeover", time.Now())
	require.NoError(t, err)

	now := time.Now().Unix()
	res1, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "11111111-1111-1111-1111-111111111111",
		TenantID:        tenant1.ID,
		SessionID:       session1.SessionID,
		HandoffID:       handoff1.HandoffID,
		ClientMessageID: "msg-1",
		Text:            "Hello Tenant 1",
		Now:             now,
	})
	require.NoError(t, err)

	res2, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "22222222-2222-2222-2222-222222222222",
		TenantID:        tenant2.ID,
		SessionID:       session2.SessionID,
		HandoffID:       handoff2.HandoffID,
		ClientMessageID: "msg-1", // same client message ID, isolated by tenant
		Text:            "Hello Tenant 2",
		Now:             now,
	})
	require.NoError(t, err)

	require.NotEqual(t, res1.Reply.ReplyID, res2.Reply.ReplyID)
	require.NotEqual(t, res1.Outbox.OutboxID, res2.Outbox.OutboxID)
}

func TestCreateBridgeReplyOutboxSessionIsolation(t *testing.T) {
	ctx, ts, tenant, session1, handoff1 := createHumanActiveHandoffFixture(t, "session-1")
	defer ts.Close()

	session2, _, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session-2", time.Now(), time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	handoff2, err := ts.CreateBridgeHandoff(ctx, tenant.ID, session2.SessionID, time.Now())
	require.NoError(t, err)

	handoff2, err = ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant.ID, session2.SessionID, handoff2.Generation, handoff2.HandoffID, handoff2.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive, "takeover", time.Now())
	require.NoError(t, err)

	now := time.Now().Unix()
	res1, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "11111111-1111-1111-1111-111111111111",
		TenantID:        tenant.ID,
		SessionID:       session1.SessionID,
		HandoffID:       handoff1.HandoffID,
		ClientMessageID: "msg-s1",
		Text:            "Hello Session 1",
		Now:             now,
	})
	require.NoError(t, err)

	// Ensure cascade delete works on session
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, "DELETE FROM bridge_external_sessions WHERE tenant_id = ? AND session_id = ?", tenant.ID, session1.SessionID)
	require.NoError(t, err)

	// Verify reply and outbox are deleted Cascade
	var replyCount, outboxCount int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_handoff_replies WHERE reply_id = ?", res1.Reply.ReplyID).Scan(&replyCount)
	require.NoError(t, err)
	require.Equal(t, 0, replyCount)

	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_reply_outbox WHERE outbox_id = ?", res1.Outbox.OutboxID).Scan(&outboxCount)
	require.NoError(t, err)
	require.Equal(t, 0, outboxCount)
}

func TestCreateBridgeHandoffReplyAndOutboxIfActiveLegacyRecovery(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "legacy-recovery")
	defer ts.Close()

	now := time.Now().Unix()
	// Insert reply using old/direct insertion method, bypassing outbox creation
	_, err := ts.CreateBridgeHandoffReplyIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-legacy",
		Text:            "Legacy Text",
		Now:             now,
	})
	require.NoError(t, err)

	// Verify outbox row does NOT exist yet
	ob, err := ts.GetBridgeReplyOutboxByReplyID(ctx, tenant.ID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	require.NoError(t, err)
	require.Nil(t, ob)

	// Call CreateBridgeHandoffReplyAndOutboxIfActive with same client message ID and same text
	res, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-legacy",
		Text:            "Legacy Text",
		Now:             now,
	})
	require.NoError(t, err)

	require.Equal(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", res.Reply.ReplyID)
	require.NotNil(t, res.Outbox)
	require.NotEmpty(t, res.Outbox.OutboxID)
	require.Equal(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", res.Outbox.ReplyID)
	require.Equal(t, "pending", res.Outbox.Status)

	// Verify database state: exactly 1 reply, exactly 1 outbox row
	var replyCount, outboxCount int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_handoff_replies WHERE tenant_id = ?", tenant.ID).Scan(&replyCount)
	require.NoError(t, err)
	require.Equal(t, 1, replyCount)

	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_reply_outbox WHERE tenant_id = ?", tenant.ID).Scan(&outboxCount)
	require.NoError(t, err)
	require.Equal(t, 1, outboxCount)
}

func TestCreateBridgeHandoffReplyAndOutboxIfActiveAtomicRollbackOnOutboxFailure(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "atomic-rollback")
	defer ts.Close()

	now := time.Now().Unix()
	// Disable foreign key checks temporarily in SQLite to insert a pre-existing outbox row
	_, err := ts.GetDriver().GetDB().ExecContext(ctx, "PRAGMA foreign_keys = OFF")
	require.NoError(t, err)
	defer func() {
		_, _ = ts.GetDriver().GetDB().ExecContext(ctx, "PRAGMA foreign_keys = ON")
	}()

	// Insert outbox row for "reply-atomic"
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, `
		INSERT INTO bridge_reply_outbox (
			outbox_id, tenant_id, session_id, handoff_id, reply_id, status, attempt_count, created_at
		) VALUES ('cccccccc-cccc-cccc-cccc-cccccccccccc', ?, ?, ?, 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'pending', 0, ?)
	`, tenant.ID, session.SessionID, handoff.HandoffID, now)
	require.NoError(t, err)

	// Re-enable foreign key constraints
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, "PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	// Now try to create a reply with ReplyID = "reply-atomic".
	// The reply insert will succeed (since "reply-atomic" doesn't exist in bridge_handoff_replies).
	// But the outbox insert will fail with unique constraint violation.
	// This will test the rollback!
	_, err = ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "dddddddd-dddd-dddd-dddd-dddddddddddd",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-atomic",
		Text:            "Atomic rollback text",
		Now:             now,
	})
	require.Error(t, err)

	// Assert that "reply-atomic" was NOT committed to bridge_handoff_replies
	var replyCount int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_handoff_replies WHERE reply_id = ?", "dddddddd-dddd-dddd-dddd-dddddddddddd").Scan(&replyCount)
	require.NoError(t, err)
	require.Equal(t, 0, replyCount)
}

func TestBridgeOutboxScopeNoDeliveryWorkerReadsOutbox(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "scope-test")
	defer ts.Close()

	now := time.Now().Unix()
	res, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         "11111111-1111-1111-1111-111111111111",
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-scope",
		Text:            "Scope check",
		Now:             now,
	})
	require.NoError(t, err)

	// Wait a bit to simulate background process time
	time.Sleep(50 * time.Millisecond)

	// Retrieve the outbox row and verify status is still strictly pending and attempt_count is 0
	ob, err := ts.GetBridgeReplyOutboxByReplyID(ctx, tenant.ID, res.Reply.ReplyID)
	require.NoError(t, err)
	require.NotNil(t, ob)
	require.Equal(t, "pending", ob.Status)
	require.Equal(t, 0, ob.AttemptCount)
}

func TestBridgeReplyOutbox_ClaimSuccess(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "claim-success")
	defer ts.Close()

	now := time.Now().Unix()

	for i := 0; i < 3; i++ {
		_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
			ReplyID:         uuid.NewString(),
			TenantID:        tenant.ID,
			SessionID:       session.SessionID,
			HandoffID:       handoff.HandoffID,
			ClientMessageID: fmt.Sprintf("msg-claim-%d", i),
			Text:            "text",
			Now:             now,
		})
		require.NoError(t, err)
	}

	claimedBy := "worker-1"
	claimDuration := int64(300)
	claimedRows, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 2, claimedBy, time.Now(), claimDuration)
	require.NoError(t, err)
	require.Len(t, claimedRows, 2)

	for _, ob := range claimedRows {
		require.Equal(t, "claimed", ob.Status)
		require.NotNil(t, ob.ClaimToken)
		require.Equal(t, 36, len(*ob.ClaimToken))
		require.NotNil(t, ob.ClaimedBy)
		require.Equal(t, claimedBy, *ob.ClaimedBy)
		require.NotNil(t, ob.ClaimedAt)
		require.NotNil(t, ob.ClaimExpiresAt)
		require.True(t, *ob.ClaimExpiresAt > *ob.ClaimedAt)
		require.Equal(t, 1, ob.AttemptCount)
	}

	claimedRows2, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 2, claimedBy, time.Now(), claimDuration)
	require.NoError(t, err)
	require.Len(t, claimedRows2, 1) // Only 1 left
}

func TestBridgeReplyOutbox_ClaimEmpty(t *testing.T) {
	ctx, ts, tenant, _, _ := createHumanActiveHandoffFixture(t, "claim-empty")
	defer ts.Close()

	claimedRows, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 5, "worker-1", time.Now(), 300)
	require.NoError(t, err)
	require.Empty(t, claimedRows)
}

func TestBridgeReplyOutbox_ClaimConcurrency(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "claim-race")
	defer ts.Close()

	now := time.Now().Unix()

	for i := 0; i < 20; i++ {
		_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
			ReplyID:         uuid.NewString(),
			TenantID:        tenant.ID,
			SessionID:       session.SessionID,
			HandoffID:       handoff.HandoffID,
			ClientMessageID: fmt.Sprintf("msg-race-%d", i),
			Text:            "text",
			Now:             now,
		})
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	claimedMap := make(map[string]string)
	errorCount := 0

	workerCount := 10
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		workerID := fmt.Sprintf("worker-%d", i)
		go func() {
			defer wg.Done()
			claimedRows, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 5, workerID, time.Now(), 300)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errorCount++
			} else {
				for _, ob := range claimedRows {
					if existing, ok := claimedMap[ob.OutboxID]; ok {
						t.Errorf("Outbox %s claimed twice! Existing: %s, New: %s", ob.OutboxID, existing, workerID)
					}
					claimedMap[ob.OutboxID] = workerID
				}
			}
		}()
	}

	wg.Wait()
	require.Equal(t, 0, errorCount)
	require.Len(t, claimedMap, 20)
}

func TestBridgeReplyOutbox_CrossTenantIsolation(t *testing.T) {
	ctx, ts, tenant1, session1, handoff1 := createHumanActiveHandoffFixture(t, "claim-iso-1")
	defer ts.Close()

	tenant2 := createBridgeTenant(t, ctx, ts, "claim-iso-2")
	now2 := time.Now()
	session2, _, err := ts.EnsureBridgeExternalSession(ctx, tenant2.ID, "session", now2, now2.Add(time.Minute))
	require.NoError(t, err)
	handoff2, err := ts.CreateBridgeHandoff(ctx, tenant2.ID, session2.SessionID, now2)
	require.NoError(t, err)
	handoff2, err = ts.UpdateBridgeHandoffRoutingModeCAS(ctx, tenant2.ID, session2.SessionID, handoff2.Generation, handoff2.HandoffID, handoff2.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive, "accepted", time.Now())
	require.NoError(t, err)

	now := time.Now().Unix()

	_, err = ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant1.ID,
		SessionID:       session1.SessionID,
		HandoffID:       handoff1.HandoffID,
		ClientMessageID: "msg-iso-1",
		Text:            "t1",
		Now:             now,
	})
	require.NoError(t, err)

	_, err = ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant2.ID,
		SessionID:       session2.SessionID,
		HandoffID:       handoff2.HandoffID,
		ClientMessageID: "msg-iso-2",
		Text:            "t2",
		Now:             now,
	})
	require.NoError(t, err)

	claimedRows, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant1.ID, 5, "worker-1", time.Now(), 300)
	require.NoError(t, err)
	require.Len(t, claimedRows, 1)
	require.Equal(t, tenant1.ID, claimedRows[0].TenantID)
}

func TestBridgeReplyOutbox_ExpiredClaimReclaimed(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "claim-expired")
	defer ts.Close()

	now := time.Now().Unix()

	res, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-exp",
		Text:            "t",
		Now:             now,
	})
	require.NoError(t, err)

	claimedRows, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 1, "worker-1", time.Now(), 1) // 1 second expiry
	require.NoError(t, err)
	require.Len(t, claimedRows, 1)

	// wait for expiry
	time.Sleep(1200 * time.Millisecond)

	firstClaimToken := *claimedRows[0].ClaimToken

	// Expired processing leases are reclaimed so crashed workers cannot strand delivery.
	claimedRows2, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 1, "worker-2", time.Now(), 300)
	require.NoError(t, err)
	require.Len(t, claimedRows2, 1)
	require.Equal(t, "worker-2", *claimedRows2[0].ClaimedBy)
	require.NotEqual(t, firstClaimToken, *claimedRows2[0].ClaimToken)
	require.Equal(t, 2, claimedRows2[0].AttemptCount)

	// Verify the durable row now belongs to the replacement worker.
	ob, err := ts.GetBridgeReplyOutboxByReplyID(ctx, tenant.ID, res.Reply.ReplyID)
	require.NoError(t, err)
	require.Equal(t, "claimed", ob.Status)
	require.Equal(t, "worker-2", *ob.ClaimedBy)
	require.Equal(t, 2, ob.AttemptCount)
}

func TestBridgeReplyOutbox_DirectClaimReclaimsExpiredLease(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "direct-claim-expired")
	defer ts.Close()

	res, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       session.SessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-direct-exp",
		Text:            "t",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	first, err := ts.ClaimBridgeReplyOutboxByOutboxID(ctx, tenant.ID, res.Outbox.OutboxID, "worker-1", time.Now(), 1)
	require.NoError(t, err)
	require.NotNil(t, first)

	time.Sleep(1200 * time.Millisecond)

	second, err := ts.ClaimBridgeReplyOutboxByOutboxID(ctx, tenant.ID, res.Outbox.OutboxID, "worker-2", time.Now(), 300)
	require.NoError(t, err)
	require.Equal(t, "worker-2", *second.ClaimedBy)
	require.NotEqual(t, *first.ClaimToken, *second.ClaimToken)
	require.Equal(t, 2, second.AttemptCount)
}

func TestBridgeReplyOutbox_RejectsInvalidLimit(t *testing.T) {
	ctx, ts, tenant, _, _ := createHumanActiveHandoffFixture(t, "claim-inv-limit")
	defer ts.Close()

	_, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 0, "worker", time.Now(), 300)
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)

	_, err = ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 101, "worker", time.Now(), 300)
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

func TestBridgeReplyOutbox_RejectsInvalidClaimedBy(t *testing.T) {
	ctx, ts, tenant, _, _ := createHumanActiveHandoffFixture(t, "claim-inv-by")
	defer ts.Close()

	_, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 5, "", time.Now(), 300)
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)

	tooLong := ""
	for i := 0; i < 130; i++ {
		tooLong += "a"
	}
	_, err = ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 5, tooLong, time.Now(), 300)
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)

	_, err = ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 5, "hello\nworld", time.Now(), 300)
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

func TestBridgeReplyOutbox_RejectsInvalidNow(t *testing.T) {
	ctx, ts, tenant, _, _ := createHumanActiveHandoffFixture(t, "claim-inv-now")
	defer ts.Close()

	_, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 5, "worker", time.Time{}, 300)
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

func TestBridgeReplyOutbox_RejectsInvalidClaimDuration(t *testing.T) {
	ctx, ts, tenant, _, _ := createHumanActiveHandoffFixture(t, "claim-inv-dur")
	defer ts.Close()

	_, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 5, "worker", time.Now(), 0)
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)

	_, err = ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 5, "worker", time.Now(), -1)
	require.ErrorIs(t, err, store.ErrBridgeInvalidArgument)
}

func TestBridgeReplyOutbox_ClaimDeterministicOrder(t *testing.T) {
	ctx, ts, tenant, session, handoff := createHumanActiveHandoffFixture(t, "claim-order")
	defer ts.Close()

	// Seed 3+ pending rows with distinct created_at/id
	now := time.Now().Unix()
	var replies []string
	for i := 0; i < 4; i++ {
		replyID := uuid.NewString()
		replies = append(replies, replyID)
		_, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
			ReplyID:         replyID,
			TenantID:        tenant.ID,
			SessionID:       session.SessionID,
			HandoffID:       handoff.HandoffID,
			ClientMessageID: fmt.Sprintf("msg-order-%d", i),
			Text:            fmt.Sprintf("text %d", i),
			Now:             now + int64(i),
		})
		require.NoError(t, err)
		// Small sleep to ensure distinct physical insertion time if created_at uses current_timestamp
		time.Sleep(10 * time.Millisecond)
	}

	// Claim limit=2
	claimedRows, err := ts.ClaimPendingBridgeReplyOutbox(ctx, tenant.ID, 2, "worker", time.Now(), 300)
	require.NoError(t, err)
	require.Len(t, claimedRows, 2)

	// Assert returned order is created_at ASC, id ASC
	// The oldest should be claimed first (i=0, then i=1)
	require.Equal(t, replies[0], claimedRows[0].ReplyID)
	require.Equal(t, replies[1], claimedRows[1].ReplyID)
}
