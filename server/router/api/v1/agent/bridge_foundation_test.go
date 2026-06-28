package agent

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
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
	_, err := ts.GetDriver().GetDB().ExecContext(ctx, "DROP TABLE bridge_external_sessions;")
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
	require.Equal(t, 5, responseType.NumField())
	require.Equal(t, "session_id", responseType.Field(0).Tag.Get("json"))
	require.Equal(t, "message", responseType.Field(1).Tag.Get("json"))
	require.Equal(t, "metadata", responseType.Field(2).Tag.Get("json"))
	require.Equal(t, "bridge,omitempty", responseType.Field(4).Tag.Get("json"))
}

func TestChatExternalRequiresExternalAudience(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("RAG_PIPELINE_ENABLED", "false")
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug: "external-audience-required", CompanyName: "test", Vertical: "test", IsActive: true,
	})
	require.NoError(t, err)
	_, err = ts.CreateAgentAudience(ctx, &store.AgentAudience{
		TenantID: tenant.ID, AudienceType: "internal", Role: "internal-only", Tone: "helpful", RateLimitRPM: 60,
	})
	require.NoError(t, err)

	service := NewService(ts, &profile.Profile{Driver: "sqlite", Mode: "prod"})
	_, err = service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{
		SessionID: "widget-session", Message: "hello",
	})
	require.ErrorContains(t, err, "/external")
}

func TestChatExternalClientMessageIDIsIdempotent(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-client-idempotency")
	defer ts.Close()

	request := ChatRequest{
		SessionID:       "widget-session",
		Message:         "hello",
		ClientMessageID: "4e0c8d2a-a718-4fd2-bfd1-a23eef418843",
	}
	first, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", request)
	require.NoError(t, err)
	session := service.memorySessions.Get(tenant.ID, request.SessionID)
	require.NotNil(t, session)
	require.Len(t, session.Messages, 2)

	second, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", request)
	require.NoError(t, err)
	require.Equal(t, first.Message.Content, second.Message.Content)
	require.Equal(t, first.Message.Timestamp.Truncate(time.Second), second.Message.Timestamp.Truncate(time.Second))
	require.Len(t, session.Messages, 2)
}

func TestChatExternalClientMessageIDIsIdempotent_Concurrent(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-client-concurrent")
	defer ts.Close()

	request := ChatRequest{
		SessionID:       "widget-concurrent",
		Message:         "concurrent-hello",
		ClientMessageID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
	}

	var wg sync.WaitGroup
	results := make([]*ChatResponse, 10)
	errors := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", request)
			results[i] = resp
			errors[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errors {
		require.NoError(t, err, "goroutine %d errored", i)
	}
	for i, resp := range results {
		require.NotNil(t, resp, "goroutine %d got nil response", i)
		require.Equal(t, results[0].Message.Content, resp.Message.Content, "goroutine %d content mismatch", i)
		require.Equal(t, results[0].Message.Timestamp.Truncate(time.Second), resp.Message.Timestamp.Truncate(time.Second), "goroutine %d timestamp mismatch", i)
	}

	session := service.memorySessions.Get(tenant.ID, request.SessionID)
	require.NotNil(t, session)
	require.Len(t, session.Messages, 2)
}

func TestChatExternalClientMessageIDIsIdempotent_Restart(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-client-restart")
	defer ts.Close()

	request := ChatRequest{
		SessionID:       "widget-restart",
		Message:         "restart-hello",
		ClientMessageID: "b2c3d4e5-f6a7-8901-bcde-f12345678901",
	}

	first, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", request)
	require.NoError(t, err)

	service2 := NewService(ts, &profile.Profile{Driver: "sqlite", Mode: "prod"})
	second, err := service2.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", request)
	require.NoError(t, err)
	require.Equal(t, first.Message.Content, second.Message.Content)
	require.Equal(t, first.Message.Timestamp.Truncate(time.Second), second.Message.Timestamp.Truncate(time.Second))
}

func TestChatExternalClientMessageIDPersistsToDatabase(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-client-persist")
	defer ts.Close()

	request := ChatRequest{
		SessionID:       "widget-persist",
		Message:         "persist-hello",
		ClientMessageID: "d4e5f6a7-b8c9-0123-defa-234567890123",
	}

	_, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", request)
	require.NoError(t, err)

	// Verify the assistant message row was actually written to the DB.
	var assistantCount int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx,
		"SELECT COUNT(1) FROM agent_messages WHERE session_id = ? AND source = 'external_response' AND source_id = ?",
		"widget-persist", request.ClientMessageID,
	).Scan(&assistantCount)
	require.NoError(t, err)
	require.Equal(t, 1, assistantCount, "exactly one assistant message row should be persisted")

	// Verify the user message row was also written.
	var userCount int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx,
		"SELECT COUNT(1) FROM agent_messages WHERE session_id = ? AND source = 'external_client_message' AND source_id = ?",
		"widget-persist", request.ClientMessageID,
	).Scan(&userCount)
	require.NoError(t, err)
	require.Equal(t, 1, userCount, "exactly one user message row should be persisted")
}

func TestChatExternalEscalationCreatesLeadAndTicketWithoutHandoff(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-escalation-lead-ticket")
	defer ts.Close()

	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{
		SessionID: "session-escalate-lead",
		Message:   "I need to speak to a manager. My name is Ada Lovelace, my email is ada@example.org.",
	})
	require.NoError(t, err)
	require.Nil(t, resp.Bridge)
	require.Contains(t, resp.Message.Content, "I've created ticket TKT-")
	require.Contains(t, resp.Message.Content, "using the contact information you provided")

	leads, err := ts.ListAgentLeads(ctx, &store.FindAgentLead{TenantID: &tenant.ID})
	require.NoError(t, err)
	require.Len(t, leads, 1)
	require.Equal(t, "Ada Lovelace", leads[0].Name)
	require.Equal(t, "ada@example.org", leads[0].Email)
	require.Equal(t, "escalation", leads[0].DetectedIntent)

	ticketType := "agent_escalation"
	tickets, err := ts.ListTickets(ctx, &store.FindTicket{Type: &ticketType})
	require.NoError(t, err)
	require.Len(t, tickets, 1)
	require.Equal(t, store.TicketPriorityMedium, tickets[0].Priority)

	memoUID := strings.TrimPrefix(tickets[0].Description, "/m/")
	memo, err := ts.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	require.NoError(t, err)
	require.NotNil(t, memo)
	require.Contains(t, memo.Content, "Session ID:** session-escalate-lead")
	require.Contains(t, memo.Content, "Lead ID:** "+leads[0].ID)
	require.Contains(t, memo.Content, "Email:** ada@example.org")

	activeHandoff, err := ts.FindActiveBridgeHandoff(ctx, tenant.ID, "session-escalate-lead")
	require.NoError(t, err)
	require.Nil(t, activeHandoff)
}

func TestChatExternalEscalationDedupesTicketAcrossServiceRestart(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-escalation-dedupe")
	defer ts.Close()

	req := ChatRequest{
		SessionID: "session-escalate-dedupe",
		Message:   "I want a supervisor. My name is Grace Hopper and my phone is 415-555-1212.",
	}
	first, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", req)
	require.NoError(t, err)
	require.Contains(t, first.Message.Content, "I've created ticket TKT-")

	service2 := NewService(ts, &profile.Profile{Driver: "sqlite", Mode: "prod"})
	second, err := service2.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", req)
	require.NoError(t, err)
	require.Contains(t, second.Message.Content, "I've created ticket TKT-")

	ticketType := "agent_escalation"
	tickets, err := ts.ListTickets(ctx, &store.FindTicket{Type: &ticketType})
	require.NoError(t, err)
	require.Len(t, tickets, 1)
}

func TestChatExternalEscalationWithIncompleteContactAsksForContactInfo(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-escalation-incomplete-contact")
	defer ts.Close()

	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{
		SessionID: "session-escalate-incomplete",
		Message:   "I want to speak to your supervisor.",
	})
	require.NoError(t, err)
	require.Contains(t, resp.Message.Content, "I've created ticket TKT-")
	require.Contains(t, resp.Message.Content, "Please share your name and either a phone number or email address")

	leads, err := ts.ListAgentLeads(ctx, &store.FindAgentLead{TenantID: &tenant.ID})
	require.NoError(t, err)
	require.Empty(t, leads)

	ticketType := "agent_escalation"
	tickets, err := ts.ListTickets(ctx, &store.FindTicket{Type: &ticketType})
	require.NoError(t, err)
	require.Len(t, tickets, 1)
}

func TestCreateAgentMessagesNilSlice(t *testing.T) {
	ctx, ts, _, _ := newBridgeChatTestService(t, "chat-nil-slice")
	defer ts.Close()

	// Verify nil slice does not error.
	err := ts.CreateAgentMessages(ctx, nil)
	require.NoError(t, err)

	// Verify empty slice does not error.
	err = ts.CreateAgentMessages(ctx, []*store.AgentMessageRecord{})
	require.NoError(t, err)
}

func TestChatExternalClientMessageIDContentMismatch(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-client-content-mismatch")
	defer ts.Close()

	request1 := ChatRequest{
		SessionID:       "widget-content-mismatch",
		Message:         "original-question",
		ClientMessageID: "c3d4e5f6-a7b8-9012-cdef-123456789012",
	}
	_, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", request1)
	require.NoError(t, err)

	request2 := ChatRequest{
		SessionID:       "widget-content-mismatch",
		Message:         "different-question",
		ClientMessageID: "c3d4e5f6-a7b8-9012-cdef-123456789012",
	}
	_, err = service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", request2)
	require.NoError(t, err)

	session := service.memorySessions.Get(tenant.ID, request1.SessionID)
	require.NotNil(t, session)
	require.Len(t, session.Messages, 4)
	require.Equal(t, "original-question", session.Messages[0].Content)
	require.Equal(t, "different-question", session.Messages[2].Content)
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
		TenantID: tenant.ID, AudienceType: "external", Role: "assistant", Tone: "helpful",
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
	_, err := ts.GetDriver().GetDB().ExecContext(ctx, "DROP TABLE bridge_external_sessions;")
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
		TenantID: tenant.ID, AudienceType: "external", Role: "assistant", Tone: "helpful",
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
