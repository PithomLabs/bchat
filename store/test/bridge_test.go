package teststore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
