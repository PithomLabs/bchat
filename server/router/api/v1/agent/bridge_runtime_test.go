package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
)

func TestChatExternalHumanActiveHandoffSuppressesAI(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-human-active")
	defer ts.Close()

	sessionID := "widget-session-123"
	// Materialize session first so we can create a handoff
	_, _ = service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: "hello"})

	// Create an active human handoff
	handoff, err := ts.GetDriver().CreateBridgeHandoff(ctx, tenant.ID, sessionID, time.Now())
	require.NoError(t, err)

	_, err = ts.GetDriver().UpdateBridgeHandoffRoutingModeCAS(
		ctx, tenant.ID, sessionID, handoff.Generation, handoff.HandoffID,
		handoff.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive,
		"operator joined", time.Now(),
	)
	require.NoError(t, err)

	// Second chat message should be suppressed
	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: "are you there?"})
	require.NoError(t, err)

	// Response shape checks
	require.NotNil(t, resp.Bridge)
	require.Equal(t, "human_handoff_active", resp.Bridge.Status)
	require.Equal(t, handoff.HandoffID, resp.Bridge.HandoffID)
	require.Equal(t, "human_active", resp.Bridge.RoutingMode)
	
	require.Equal(t, "handoff_active", resp.Metadata.Intent)
	require.Equal(t, "system", resp.Message.Role)
	require.Equal(t, "A human operator is handling this conversation.", resp.Message.Content)
}

func TestChatExternalQueuedHandoffSemantics(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-queued-handoff")
	defer ts.Close()

	sessionID := "widget-session-123"
	_, _ = service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: "hello"})

	// Create a queued handoff
	handoff, err := ts.GetDriver().CreateBridgeHandoff(ctx, tenant.ID, sessionID, time.Now())
	require.NoError(t, err)
	require.Equal(t, store.BridgeRoutingModeHandoffQueued, handoff.RoutingMode)

	// Second chat message should be suppressed (queued suppresses AI)
	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: "I need a human"})
	require.NoError(t, err)

	require.NotNil(t, resp.Bridge)
	require.Equal(t, "human_handoff_queued", resp.Bridge.Status)
	require.Equal(t, handoff.HandoffID, resp.Bridge.HandoffID)
	require.Equal(t, "handoff_queued", resp.Bridge.RoutingMode)
}

func TestChatExternalAfterReleaseResumesAIBehavior(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-release-resume")
	defer ts.Close()

	sessionID := "widget-session-123"
	_, _ = service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: "hello"})

	// Create a queued handoff
	handoff, err := ts.GetDriver().CreateBridgeHandoff(ctx, tenant.ID, sessionID, time.Now())
	require.NoError(t, err)

	// Close it
	_, err = ts.GetDriver().UpdateBridgeHandoffRoutingModeCAS(
		ctx, tenant.ID, sessionID, handoff.Generation, handoff.HandoffID,
		handoff.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeClosed,
		"resolved", time.Now(),
	)
	require.NoError(t, err)

	// Next chat should go to AI
	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: "what is the price?"})
	require.NoError(t, err)

	require.Nil(t, resp.Bridge)
	require.Equal(t, "assistant", resp.Message.Role)
	require.NotEqual(t, "A human operator is handling this conversation.", resp.Message.Content)
}

func TestChatExternalHumanActiveHandoffDoesNotAppendUserOrAIMessage(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "chat-no-append")
	defer ts.Close()

	sessionID := "widget-session-123"
	// Send initial message
	_, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: "hello"})
	require.NoError(t, err)

	// Verify transcript count is 2 (user + AI)
	memSession := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	require.Len(t, memSession.Messages, 2)

	// Create an active human handoff
	handoff, err := ts.GetDriver().CreateBridgeHandoff(ctx, tenant.ID, sessionID, time.Now())
	require.NoError(t, err)
	_, err = ts.GetDriver().UpdateBridgeHandoffRoutingModeCAS(
		ctx, tenant.ID, sessionID, handoff.Generation, handoff.HandoffID,
		handoff.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive,
		"operator joined", time.Now(),
	)
	require.NoError(t, err)

	// Send message during active handoff
	_, err = service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: sessionID, Message: "are you there?"})
	require.NoError(t, err)

	// Verify transcript is 3 (user + AI + user, no new AI message appended)
	memSession = service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	require.Len(t, memSession.Messages, 3)
	require.Equal(t, "user", memSession.Messages[2].Role)
	require.Equal(t, "are you there?", memSession.Messages[2].Content)
}

func TestChatExternalUnsupportedBridgeDBDoesNotBreakNormalChat(t *testing.T) {
	ctx, ts, _, tenant := newBridgeChatTestService(t, "chat-unsupported")
	defer ts.Close()

	// Swap driver to unsupported
	unsupportedDriver := &unsupportedBridgeDriver{Driver: ts.GetDriver()}
	customStore := store.New(unsupportedDriver, &profile.Profile{Driver: "postgres", Mode: "prod"})
	customService := NewService(customStore, &profile.Profile{Driver: "postgres", Mode: "prod"})

	// Should not error, normal AI chat proceeds
	resp, err := customService.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: "widget", Message: "hello"})
	require.NoError(t, err)
	require.Nil(t, resp.Bridge)
	require.Equal(t, "assistant", resp.Message.Role)
}

type failingBridgeDriver struct {
	store.Driver
}

func (d *failingBridgeDriver) EnsureBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string, now, expiresAt time.Time) (*store.BridgeExternalSession, bool, error) {
	// Let it pass so it reaches the handoff check
	return &store.BridgeExternalSession{}, false, nil
}

func (d *failingBridgeDriver) FindActiveBridgeHandoff(ctx context.Context, tenantID int32, sessionID string) (*store.BridgeHandoff, error) {
	return nil, errors.New("unexpected DB error")
}

func TestChatExternalHandoffCheckDBFailureDoesNotLetAIAnswer(t *testing.T) {
	ctx, ts, _, tenant := newBridgeChatTestService(t, "chat-failing")
	defer ts.Close()

	// Swap driver to failing
	failingDriver := &failingBridgeDriver{Driver: ts.GetDriver()}
	customStore := store.New(failingDriver, &profile.Profile{Driver: "sqlite", Mode: "prod"})
	customService := NewService(customStore, &profile.Profile{Driver: "sqlite", Mode: "prod"})

	// Should fail with 500 equivalent, preventing AI chat
	_, err := customService.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{SessionID: "widget", Message: "hello"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "bridge handoff check failed")
}
