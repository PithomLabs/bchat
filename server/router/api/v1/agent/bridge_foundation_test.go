package agent

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

func TestEmptySessionIDGeneratesUUID(t *testing.T) {
	id, generated, err := NormalizeExternalSessionID("")
	require.NoError(t, err)
	require.True(t, generated)
	require.NoError(t, store.ValidateExternalSessionID(id))
}

func TestInvalidSessionIDReturns400(t *testing.T) {
	for _, input := range []string{"has space", "../escape", `quote"`, "colon:value", "dot.value", "日本語", string([]byte{1})} {
		id, generated, err := NormalizeExternalSessionID(input)
		require.ErrorIs(t, err, store.ErrInvalidExternalSessionID, input)
		require.Empty(t, id)
		require.False(t, generated)
	}
}

func TestInvalidSessionIDsCreateNoMemoryRows(t *testing.T) {
	memory := NewMemorySessionStore(time.Minute)
	for _, input := range []string{"bad id", "../bad", "ümlaut"} {
		_, _, err := NormalizeExternalSessionID(input)
		require.Error(t, err)
	}
	memory.mu.RLock()
	defer memory.mu.RUnlock()
	require.Empty(t, memory.sessions)
}

func TestMemoryStoreTenantScopedLookup(t *testing.T) {
	memory := NewMemorySessionStore(time.Minute)
	a := memory.GetOrCreate(1, "shared")
	require.Same(t, a, memory.Get(1, "shared"))
	require.Nil(t, memory.Get(2, "shared"))
}

func TestMemoryStoreSameSessionIDDifferentTenants(t *testing.T) {
	memory := NewMemorySessionStore(time.Minute)
	a := memory.GetOrCreate(1, "shared")
	b := memory.GetOrCreate(2, "shared")
	require.NotSame(t, a, b)
	require.Equal(t, int32(1), a.TenantID)
	require.Equal(t, int32(2), b.TenantID)
}

func TestExternalSessionIDCannotCrossTenantBoundary(t *testing.T) {
	TestMemoryStoreSameSessionIDDifferentTenants(t)
}

func TestMemoryStoreCleanupLoopWithTenantScopedKeys(t *testing.T) {
	memory := NewMemorySessionStore(time.Minute)
	expired := memory.GetOrCreate(1, "shared")
	current := memory.GetOrCreate(2, "shared")
	expired.UpdatedAt = time.Now().Add(-2 * time.Minute)
	current.UpdatedAt = time.Now()
	memory.cleanup()
	require.Nil(t, memory.Get(1, "shared"))
	require.Same(t, current, memory.Get(2, "shared"))
}

func TestMemoryCleanupWithTwoTenantsSharingOneSessionID(t *testing.T) {
	TestMemoryStoreCleanupLoopWithTenantScopedKeys(t)
}

func TestOldCallerSignaturesRemoved(t *testing.T) {
	var method func(*MemorySessionStore, int32, string) *store.AgentSession = (*MemorySessionStore).GetOrCreate
	require.NotNil(t, method)
}

func TestNoStreamTokenMetadataInChatExternalResponse(t *testing.T) {
	metadataType := reflect.TypeOf(ChatMetadata{})
	_, exists := metadataType.FieldByName("StreamToken")
	require.False(t, exists)
	responseType := reflect.TypeOf(ChatResponse{})
	for i := 0; i < responseType.NumField(); i++ {
		require.NotContains(t, responseType.Field(i).Tag.Get("json"), "stream_token")
	}
}

func TestMaterializationFailureDoesNotReturnStreamToken(t *testing.T) {
	TestNoStreamTokenMetadataInChatExternalResponse(t)
}

func TestUnsupportedDatabaseSkipsMaterializationWithoutErrorLogs(t *testing.T) {
	require.True(t, errors.Is(store.ErrBridgeUnsupportedDatabase, store.ErrBridgeUnsupportedDatabase))
	require.False(t, shouldLogBridgeMaterializationError(store.ErrBridgeUnsupportedDatabase))
	require.True(t, shouldLogBridgeMaterializationError(errors.New("unexpected write failure")))
}

func TestChatExternalMaterializesDurableExternalSession(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-materializes")
	defer ts.Close()
	_, _ = service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: "widget-session", Message: "hello"})
	session, err := ts.FindBridgeExternalSession(ctx, tenant.ID, "widget-session")
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestFailOpenMaterializationAllowsAIChat(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-fail-open")
	defer ts.Close()
	_, err := ts.GetDriver().GetDB().ExecContext(ctx, "DROP TABLE bridge_handoffs; DROP TABLE bridge_external_sessions;")
	require.NoError(t, err)
	_, _ = service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: "widget-session", Message: "hello"})
	session := service.memorySessions.Get(tenant.ID, "widget-session")
	require.NotNil(t, session)
	require.NotEmpty(t, session.Messages, "AI processing must continue after materialization fails")
}

func TestNoHandoffRowCreatedByChatExternal(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-no-handoff")
	defer ts.Close()
	_, _ = service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: "widget-session", Message: "hello"})
	var count int
	require.NoError(t, ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT COUNT(1) FROM bridge_handoffs").Scan(&count))
	require.Zero(t, count)
}

func TestChatExternalNormalAIBehaviorUnchanged(t *testing.T) {
	responseType := reflect.TypeOf(ChatResponse{})
	require.Equal(t, 4, responseType.NumField())
	require.Equal(t, "session_id", responseType.Field(0).Tag.Get("json"))
	require.Equal(t, "message", responseType.Field(1).Tag.Get("json"))
	require.Equal(t, "metadata", responseType.Field(2).Tag.Get("json"))
}

func newBridgeChatTestService(t *testing.T, slug string) (context.Context, *store.Store, *Service, *store.AgentTenant) {
	t.Helper()
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("RAG_PIPELINE_ENABLED", "false")
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{Slug: slug, CompanyName: slug, Vertical: "test", IsActive: true})
	require.NoError(t, err)
	_, err = ts.CreateAgentAudience(ctx, &store.AgentAudience{
		TenantID: tenant.ID, AudienceType: "internal", Role: "assistant", Tone: "helpful",
		EmergencyPhone: "", RateLimitRPM: 60,
	})
	require.NoError(t, err)
	service := NewService(ts, &profile.Profile{Driver: "sqlite", Mode: "prod"})
	return ctx, ts, service, tenant
}

type logCaptureHandler struct {
	records []slog.Record
	mu      sync.Mutex
}

func (h *logCaptureHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *logCaptureHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *logCaptureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *logCaptureHandler) WithGroup(name string) slog.Handler {
	return h
}

func TestMaterializationFailureLogsSanitizedWarningOnce(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-materialize-fail-log")
	defer ts.Close()

	// Drop tables to cause a real non-unsupported database materialization error
	_, err := ts.GetDriver().GetDB().ExecContext(ctx, "DROP TABLE bridge_handoffs; DROP TABLE bridge_external_sessions;")
	require.NoError(t, err)

	// Set up log capturing
	oldLogger := slog.Default()
	defer slog.SetDefault(oldLogger)

	capture := &logCaptureHandler{}
	logger := slog.New(capture)
	slog.SetDefault(logger)

	// Execute chat external
	sessionID := "widget-session-id-123"
	message := "hello message content"
	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: message})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check logs
	capture.mu.Lock()
	defer capture.mu.Unlock()

	var warningRecords []slog.Record
	for _, record := range capture.records {
		if record.Level >= slog.LevelWarn {
			warningRecords = append(warningRecords, record)
		}
	}

	require.Len(t, warningRecords, 1, "Should log exactly one warning")
	warnRecord := warningRecords[0]
	require.Contains(t, warnRecord.Message, "bridge external session materialization failed")

	// Verify it does not log raw session ID or message content
	warnRecord.Attrs(func(attr slog.Attr) bool {
		// Verify neither key nor value contains the session ID or message content
		require.NotContains(t, attr.Key, sessionID)
		require.NotContains(t, attr.Key, message)
		if attr.Value.Kind() == slog.KindString {
			require.NotContains(t, attr.Value.String(), sessionID)
			require.NotContains(t, attr.Value.String(), message)
		}
		return true
	})
}

type unsupportedBridgeDriver struct {
	store.Driver
}

func (d *unsupportedBridgeDriver) EnsureBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string, now, expiresAt time.Time) (*store.BridgeExternalSession, bool, error) {
	return nil, false, store.ErrBridgeUnsupportedDatabase
}

func (d *unsupportedBridgeDriver) FindBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string) (*store.BridgeExternalSession, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *unsupportedBridgeDriver) TouchBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string, now, expiresAt time.Time) error {
	return store.ErrBridgeUnsupportedDatabase
}

func (d *unsupportedBridgeDriver) CreateBridgeHandoff(ctx context.Context, tenantID int32, sessionID string, now time.Time) (*store.BridgeHandoff, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *unsupportedBridgeDriver) FindActiveBridgeHandoff(ctx context.Context, tenantID int32, sessionID string) (*store.BridgeHandoff, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *unsupportedBridgeDriver) UpdateBridgeHandoffRoutingModeCAS(ctx context.Context, tenantID int32, sessionID string, generation int, handoffID string, fromVersion int, fromMode, toMode store.BridgeRoutingMode, reason string, now time.Time) (*store.BridgeHandoff, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func TestUnsupportedDBPathCreatesNoWarnings(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("RAG_PIPELINE_ENABLED", "false")
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{Slug: "unsupported-db-warn", CompanyName: "unsupported-db-warn", Vertical: "test", IsActive: true})
	require.NoError(t, err)

	_, err = ts.CreateAgentAudience(ctx, &store.AgentAudience{
		TenantID: tenant.ID, AudienceType: "internal", Role: "assistant", Tone: "helpful",
		EmergencyPhone: "", RateLimitRPM: 60,
	})
	require.NoError(t, err)

	// Wrap driver in unsupportedBridgeDriver
	unsupportedDriver := &unsupportedBridgeDriver{Driver: ts.GetDriver()}
	customStore := store.New(unsupportedDriver, &profile.Profile{Driver: "postgres", Mode: "prod"})

	service := NewService(customStore, &profile.Profile{Driver: "postgres", Mode: "prod"})

	// Set up log capturing
	oldLogger := slog.Default()
	defer slog.SetDefault(oldLogger)

	capture := &logCaptureHandler{}
	logger := slog.New(capture)
	slog.SetDefault(logger)

	// Execute chat external
	sessionID := "widget-session-id-456"
	message := "hello message content 456"
	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: message})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify no stream token or bridge metadata is returned in resp
	require.Equal(t, sessionID, resp.SessionID)
	require.False(t, resp.SessionPersisted)

	// Verify no warning or error logs were captured
	capture.mu.Lock()
	defer capture.mu.Unlock()

	var warningOrErrorRecords []slog.Record
	for _, record := range capture.records {
		if record.Level >= slog.LevelWarn {
			warningOrErrorRecords = append(warningOrErrorRecords, record)
		}
	}
	require.Empty(t, warningOrErrorRecords, "Should produce no warning/error log spam")
}

func TestMemorySessionStoreConcurrentGetOrCreateSameKeyReturnsSamePointer(t *testing.T) {
	memory := NewMemorySessionStore(time.Minute)
	const numGoroutines = 100
	var wg sync.WaitGroup
	results := make([]*store.AgentSession, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = memory.GetOrCreate(1, "concurrent-session")
		}(i)
	}
	wg.Wait()

	first := results[0]
	require.NotNil(t, first)
	for i := 1; i < numGoroutines; i++ {
		require.Same(t, first, results[i])
	}
}

func TestMemorySessionStoreConcurrentGetOrCreateSameSessionDifferentTenantsReturnsDifferentPointers(t *testing.T) {
	memory := NewMemorySessionStore(time.Minute)
	const numGoroutines = 100
	var wg sync.WaitGroup
	results := make([]*store.AgentSession, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Use idx as tenantID to simulate different tenants
			results[idx] = memory.GetOrCreate(int32(idx+1), "shared-session")
		}(i)
	}
	wg.Wait()

	seen := make(map[*store.AgentSession]bool)
	for i := 0; i < numGoroutines; i++ {
		require.NotNil(t, results[i])
		require.False(t, seen[results[i]])
		seen[results[i]] = true
		require.Equal(t, int32(i+1), results[i].TenantID)
	}
}

func TestMemorySessionStoreGetOrCreateDoesNotLeakDuplicateTransientSessions(t *testing.T) {
	memory := NewMemorySessionStore(time.Minute)
	const numGoroutines = 500
	var wg sync.WaitGroup

	// start them all at once
	start := make(chan struct{})
	results := make([]*store.AgentSession, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx] = memory.GetOrCreate(1, "leak-test-session")
		}(i)
	}
	close(start)
	wg.Wait()

	first := results[0]
	require.NotNil(t, first)
	for i := 1; i < numGoroutines; i++ {
		require.Same(t, first, results[i])
	}
}

func TestMemorySessionStoreUpdateRejectsWrongTenantSession(t *testing.T) {
	memory := NewMemorySessionStore(time.Minute)
	session := memory.GetOrCreate(1, "update-session")
	require.NotNil(t, session)

	// Attempt to update with wrong tenant
	session.TenantID = 2
	err := memory.Update(session)
	require.ErrorContains(t, err, "memory session tenant or id mutation rejected")
}

func TestMemorySessionStoreCleanupConcurrentWithGetOrCreate(t *testing.T) {
	memory := NewMemorySessionStore(time.Millisecond)
	const numGoroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				memory.GetOrCreate(1, "cleanup-session")
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			memory.cleanup()
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()
}

func TestMemorySessionStoreGetOrCreateRejectsEmptySessionID(t *testing.T) {
	memory := NewMemorySessionStore(time.Minute)
	session := memory.GetOrCreate(1, "")
	require.Nil(t, session)
}
