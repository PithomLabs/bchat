package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
)

func setupLiveHandoff(t *testing.T, ctx context.Context, ts *store.Store, tenantID int32, sessionID string) *store.BridgeHandoff {
	t.Helper()
	_, _, err := ts.EnsureBridgeExternalSession(ctx, tenantID, sessionID, time.Now(), time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	handoff, err := ts.GetDriver().CreateBridgeHandoff(ctx, tenantID, sessionID, time.Now())
	require.NoError(t, err)
	updated, err := ts.GetDriver().UpdateBridgeHandoffRoutingModeCAS(
		ctx, tenantID, sessionID, handoff.Generation, handoff.HandoffID,
		handoff.Version, store.BridgeRoutingModeHandoffQueued, store.BridgeRoutingModeHumanActive,
		"operator joined", time.Now(),
	)
	require.NoError(t, err)
	return updated
}

func TestBChatLiveHumanReplyAppearsInVisitorTranscript(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-reply-appear")
	defer ts.Close()

	sessionID := "session-visitor-1"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-msg-1",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.NoError(t, err)

	// Verify in memory session
	updatedSession := service.memorySessions.Get(tenant.ID, sessionID)
	require.Len(t, updatedSession.Messages, 2)
	lastMsg := updatedSession.Messages[1]
	require.Equal(t, "assistant", lastMsg.Role)
	require.Equal(t, "operator message", lastMsg.Content)
	require.Equal(t, "bridge_human_reply", lastMsg.Source)
	require.Equal(t, replyID, lastMsg.SourceID)

	// Verify via public GET transcript endpoint DTO mapping
	e := echo.New()
	handler := NewHandler(service, ts)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/"+tenant.Slug+"/chat/ext/transcript?session_id="+sessionID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues(tenant.Slug)

	err = handler.HandleGetExternalTranscript(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	messages, ok := resp["messages"].([]interface{})
	require.True(t, ok)
	require.Len(t, messages, 2)

	firstMsg := messages[0].(map[string]interface{})
	require.Equal(t, "user", firstMsg["role"])
	require.Equal(t, "need help", firstMsg["content"])
	require.Nil(t, firstMsg["kind"])

	secondMsg := messages[1].(map[string]interface{})
	require.Equal(t, "assistant", secondMsg["role"])
	require.Equal(t, "operator message", secondMsg["content"])
	require.Equal(t, "human_agent", secondMsg["kind"])

	// Omit internal fields
	require.Nil(t, secondMsg["source"])
	require.Nil(t, secondMsg["source_id"])
}

func TestBChatLiveHumanReplyDoesNotDuplicateOnSecondWorkerRun(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-reply-no-dup")
	defer ts.Close()

	sessionID := "session-visitor-2"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-msg-2",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	// First run
	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.NoError(t, err)

	// Create second outbox referring to the same replyID
	// In the real system, this doesn't happen normally, but we test delivery-level safety
	// by resetting outbox or delivering manually again.
	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.Error(t, err) // Should return already completed or conflict error since it was already claimed/settled

	// Now check duplicates in memory
	updatedSession := service.memorySessions.Get(tenant.ID, sessionID)
	require.Len(t, updatedSession.Messages, 2)
}

func TestBChatLiveHumanReplyMarkedAsHumanNotAI(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-reply-marked")
	defer ts.Close()

	sessionID := "session-visitor-3"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-msg-3",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.NoError(t, err)

	updatedSession := service.memorySessions.Get(tenant.ID, sessionID)
	lastMsg := updatedSession.Messages[1]
	require.Equal(t, "assistant", lastMsg.Role)
	require.Equal(t, "bridge_human_reply", lastMsg.Source)
	require.Equal(t, replyID, lastMsg.SourceID)
}

func TestBChatLiveAIStillSuppressedDuringHumanHandoff(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-ai-suppressed")
	defer ts.Close()

	sessionID := "session-visitor-4"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{
		SessionID: sessionID,
		Message:   "are you there?",
	})
	require.NoError(t, err)
	require.Equal(t, "system", resp.Message.Role)
	require.Equal(t, "A human operator is handling this conversation.", resp.Message.Content)
}

func TestBChatLiveReleaseAllowsAIResume(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-ai-resume")
	defer ts.Close()

	sessionID := "session-visitor-5"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	// Close handoff to simulate release
	_, err := ts.GetDriver().UpdateBridgeHandoffRoutingModeCAS(
		ctx, tenant.ID, sessionID, handoff.Generation, handoff.HandoffID,
		handoff.Version, store.BridgeRoutingModeHumanActive, store.BridgeRoutingModeClosed,
		"released", time.Now(),
	)
	require.NoError(t, err)

	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{
		SessionID: sessionID,
		Message:   "hello again",
	})
	require.NoError(t, err)
	require.Equal(t, "assistant", resp.Message.Role) // AI resumed!
	require.NotEqual(t, "A human operator is handling this conversation.", resp.Message.Content)
}

func TestBChatLiveClaimedOutboxCompletesAfterTranscriptAppend(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-outbox-complete")
	defer ts.Close()

	sessionID := "session-visitor-6"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-msg-6",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.NoError(t, err)

	// Query DB outbox status
	var dbOutboxStatus string
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT status FROM bridge_reply_outbox WHERE outbox_id = ?", result.Outbox.OutboxID).Scan(&dbOutboxStatus)
	require.NoError(t, err)
	require.Equal(t, "completed", dbOutboxStatus)
}

func TestBChatLiveClaimedOutboxFailsWhenReplyMissing(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-outbox-fail-reply")
	defer ts.Close()

	sessionID := "session-visitor-7"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-msg-7",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	// Disable foreign key checks temporarily in SQLite to allow referencing non-existent reply ID
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, "PRAGMA foreign_keys = OFF")
	require.NoError(t, err)

	nonExistentReplyID := uuid.NewString()
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, "UPDATE bridge_reply_outbox SET reply_id = ? WHERE outbox_id = ?", nonExistentReplyID, result.Outbox.OutboxID)
	require.NoError(t, err)

	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.Error(t, err)

	// Query DB outbox status
	var dbOutboxStatus string
	var failureCode string
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT status, failure_code FROM bridge_reply_outbox WHERE outbox_id = ?", result.Outbox.OutboxID).Scan(&dbOutboxStatus, &failureCode)
	require.NoError(t, err)
	require.Equal(t, "failed", dbOutboxStatus)
	require.Equal(t, "webchat_reply_missing", failureCode)
}

func TestBChatLiveClaimedOutboxFailsWhenSessionMissing(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-outbox-fail-sess")
	defer ts.Close()

	sessionID := "session-visitor-8"
	// Do NOT initialize memory session or DB transcript

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-msg-8",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.Error(t, err)

	// Query DB outbox status
	var dbOutboxStatus string
	var failureCode string
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT status, failure_code FROM bridge_reply_outbox WHERE outbox_id = ?", result.Outbox.OutboxID).Scan(&dbOutboxStatus, &failureCode)
	require.NoError(t, err)
	require.Equal(t, "failed", dbOutboxStatus)
	require.Equal(t, "webchat_session_missing", failureCode)
}

func TestBChatLiveReplyQueryErrorUsesDeliveryFailed(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-reply-query-error")
	defer ts.Close()

	sessionID := "session-reply-query-error"
	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-reply-query-error",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	_, err = ts.GetDriver().GetDB().ExecContext(ctx, "ALTER TABLE bridge_handoff_replies RENAME TO broken_bridge_handoff_replies")
	require.NoError(t, err)

	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.Error(t, err)

	var status, failureCode, failureMessage string
	err = ts.GetDriver().GetDB().QueryRowContext(ctx,
		"SELECT status, failure_code, failure_message FROM bridge_reply_outbox WHERE outbox_id = ?",
		result.Outbox.OutboxID,
	).Scan(&status, &failureCode, &failureMessage)
	require.NoError(t, err)
	require.Equal(t, "failed", status)
	require.Equal(t, "webchat_delivery_failed", failureCode)
	require.Equal(t, "failed to load reply content", failureMessage)
}

func TestBChatLiveTranscriptQueryErrorUsesDeliveryFailed(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-transcript-query-error")
	defer ts.Close()

	sessionID := "session-transcript-query-error"
	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         uuid.NewString(),
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-transcript-query-error",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	_, err = ts.GetDriver().GetDB().ExecContext(ctx, "DROP TABLE agent_transcripts")
	require.NoError(t, err)

	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.Error(t, err)

	var status, failureCode, failureMessage string
	err = ts.GetDriver().GetDB().QueryRowContext(ctx,
		"SELECT status, failure_code, failure_message FROM bridge_reply_outbox WHERE outbox_id = ?",
		result.Outbox.OutboxID,
	).Scan(&status, &failureCode, &failureMessage)
	require.NoError(t, err)
	require.Equal(t, "failed", status)
	require.Equal(t, "webchat_delivery_failed", failureCode)
	require.Equal(t, "failed to load session transcript", failureMessage)
}

func TestBChatLiveRebuildMemorySessionDoesNotOverwriteNewerMemoryWithHigherMessageCount(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-rebuild-message-count")
	defer ts.Close()

	sessionID := "session-rebuild-message-count"
	baseTime := time.Now().Add(-time.Minute)
	existing := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	existing.Messages = []store.AgentMessage{
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
		{Role: "user", Content: "three"},
		{Role: "assistant", Content: "four"},
		{Role: "user", Content: "five"},
	}
	existing.MessageCount = len(existing.Messages)
	existing.UpdatedAt = baseTime

	transcript := &store.AgentTranscript{
		TenantID:      tenant.ID,
		SessionID:     sessionID,
		Messages:      existing.Messages[:3],
		MessageCount:  3,
		StartedAt:     baseTime.Add(-time.Minute),
		LastMessageAt: baseTime.Add(time.Second),
	}

	rebuilt := service.rebuildMemorySession(ctx, tenant.ID, sessionID, transcript)

	require.Same(t, existing, rebuilt)
	require.Equal(t, 5, rebuilt.MessageCount)
	require.Len(t, rebuilt.Messages, 5)
	require.Equal(t, "five", rebuilt.Messages[4].Content)
}

func TestBChatLiveDuplicateTranscriptAppendCompletesIdempotently(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-dup-completes")
	defer ts.Close()

	sessionID := "session-visitor-9"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-msg-9",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	// First delivery
	err = service.DeliverWebChatReply(ctx, tenant.ID, ClimateOutbox(result.Outbox.OutboxID))
	require.NoError(t, err)

	// Manually change outbox back to pending so we can claim and deliver again to test duplicate detection completing idempotently
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, "UPDATE bridge_reply_outbox SET status = 'pending', claim_token = NULL, claimed_by = NULL, claimed_at = NULL, claim_expires_at = NULL, completed_at = NULL WHERE outbox_id = ?", result.Outbox.OutboxID)
	require.NoError(t, err)

	// Second delivery (duplicate)
	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.NoError(t, err) // Should succeed idempotently without error

	// Verify outbox status in DB is completed
	var dbOutboxStatus string
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT status FROM bridge_reply_outbox WHERE outbox_id = ?", result.Outbox.OutboxID).Scan(&dbOutboxStatus)
	require.NoError(t, err)
	require.Equal(t, "completed", dbOutboxStatus)
}

func ClimateOutbox(id string) string {
	return id
}

func TestBChatLiveDuplicatePreventionDurableAcrossEvictionAndRestart(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-durable-dup")
	defer ts.Close()

	sessionID := "session-visitor-10"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-msg-10",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	// 1. First delivery
	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.NoError(t, err)

	// Verify transcript was saved durably to DB
	transcript, err := ts.GetAgentTranscript(ctx, &store.FindAgentTranscript{
		SessionID: &sessionID,
		TenantID:  &tenant.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, transcript)
	require.Len(t, transcript.Messages, 2)

	// 2. Simulate server restart / eviction by clearing in-memory session store completely
	service.memorySessions.mu.Lock()
	service.memorySessions.sessions = make(map[memorySessionKey]*store.AgentSession)
	service.memorySessions.mu.Unlock()

	// Verify it's evicted
	require.Nil(t, service.memorySessions.Get(tenant.ID, sessionID))

	// Reset outbox row status to pending to deliver again
	_, err = ts.GetDriver().GetDB().ExecContext(ctx, "UPDATE bridge_reply_outbox SET status = 'pending', claim_token = NULL, claimed_by = NULL, claimed_at = NULL, claim_expires_at = NULL, completed_at = NULL WHERE outbox_id = ?", result.Outbox.OutboxID)
	require.NoError(t, err)

	// 3. Second delivery attempt (evicted memory)
	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.NoError(t, err)

	// 4. Verify no duplicate message was added to transcript
	dbTranscript, err := ts.GetAgentTranscript(ctx, &store.FindAgentTranscript{
		SessionID: &sessionID,
		TenantID:  &tenant.ID,
	})
	require.NoError(t, err)
	require.Len(t, dbTranscript.Messages, 2, "Transcript should not contain duplicate messages after memory eviction")

	// Verify memory session was rebuilt and matches DB
	rebuiltSession := service.memorySessions.Get(tenant.ID, sessionID)
	require.NotNil(t, rebuiltSession)
	require.Len(t, rebuiltSession.Messages, 2)
}

func TestBChatLiveDoesNotAddTelegramHermesSlackEmail(t *testing.T) {
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "node_modules" || info.Name() == ".git" || info.Name() == "build" || info.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}
		if strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		contentStr := string(content)
		if strings.Contains(path, "server/router/api/v1/agent/") {
			for _, pkg := range []string{"telegram", "hermes", "slack", "discord", "email"} {
				if strings.Contains(contentStr, `"`+pkg+`"`) || strings.Contains(contentStr, `"`+pkg+`/`) {
					t.Errorf("File %s imports third-party transport package: %s", path, pkg)
				}
			}
		}
		return nil
	})
	require.NoError(t, err)
}

func TestBChatLiveDoesNotAddGenericTransportRegistry(t *testing.T) {
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}
		if strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		contentStr := string(content)
		if strings.Contains(contentStr, "RegisterTransport") || strings.Contains(contentStr, "TransportRegistry") || strings.Contains(contentStr, "GenericTransport") {
			t.Errorf("File %s seems to declare or use generic transport registries", path)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestBChatLiveDoesNotExposeClaimTokenToVisitor(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-token-leak")
	defer ts.Close()

	sessionID := "session-visitor-11"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "client-msg-11",
		Text:            "operator message",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	err = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)
	require.NoError(t, err)

	e := echo.New()
	handler := NewHandler(service, ts)

	// Test GET transcript response body doesn't leak claim_token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/"+tenant.Slug+"/chat/ext/transcript?session_id="+sessionID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues(tenant.Slug)

	err = handler.HandleGetExternalTranscript(c)
	require.NoError(t, err)

	bodyStr := rec.Body.String()
	require.NotContains(t, bodyStr, "claim_token")
	require.NotContains(t, bodyStr, "source_id")
	require.NotContains(t, bodyStr, "source")
	require.NotContains(t, bodyStr, "reply_id")
	require.NotContains(t, bodyStr, "outbox_id")
}

func TestBChatLiveDoesNotAddDeliveryEndpoint(t *testing.T) {
	p, err := findWorkspaceFile("server/router/api/v1/v1.go")
	require.NoError(t, err)
	content, err := os.ReadFile(p)
	require.NoError(t, err)

	s := string(content)
	// Check no HTTP routes for delivery registered
	require.NotContains(t, s, "/bridge/delivery")
	require.NotContains(t, s, "/bridge/worker")
	require.NotContains(t, s, "/bridge/run")
}

func TestBChatLiveBridgeReplyResponseAddsOnlyWebChatDeliveryTelemetry(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "live-reply-telemetry", true)
	defer ts.Close()

	// Create agent audience
	_, err := ts.CreateAgentAudience(ctx, &store.AgentAudience{
		TenantID: tenant.ID, AudienceType: "internal", Role: "assistant", Tone: "helpful",
		EmergencyPhone: "", RateLimitRPM: 60,
	})
	require.NoError(t, err)

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	// Takeover
	bodyTakeover := []byte(`{"session_id": "session_telemetry"}`)
	now := time.Now().Unix()
	nonce1 := "takeover_nonce_1"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/live-reply-telemetry/bridge/takeover", "application/json", now, nonce1, bodyTakeover)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/live-reply-telemetry/bridge/takeover", bytes.NewReader(bodyTakeover))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce1)
	req1.Header.Set("X-Bridge-Signature", sig1)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var respTakeover BridgeTakeoverResponse
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &respTakeover))
	handoffID := respTakeover.HandoffID

	// Create visitor session in memory so reply succeeds
	session := svc.memorySessions.GetOrCreate(tenant.ID, "session_telemetry")
	session.Messages = []store.AgentMessage{
		{Role: "user", Content: "hello", Timestamp: time.Now()},
	}
	svc.memorySessions.Update(session)

	// Reply
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_telemetry", "handoff_id": "%s", "message_id": "msg_telemetry", "text": "hello"}`, handoffID))
	nonce2 := "reply_nonce_2_longer"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/live-reply-telemetry/bridge/reply", "application/json", now, nonce2, bodyReply)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/live-reply-telemetry/bridge/reply", bytes.NewReader(bodyReply))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce2)
	req2.Header.Set("X-Bridge-Signature", sig2)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(rec2.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Assert telemetry fields are present
	webchatDelivery, ok := resp["webchat_delivery"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, true, webchatDelivery["attempted"])
	require.Equal(t, "completed", webchatDelivery["status"])

	// Check response shape - no other new fields
	require.Len(t, resp, 7) // status, reply_id, handoff_id, message_id, delivery_status, outbox, webchat_delivery
}

func TestBChatLiveWidgetLocalStorageUsage(t *testing.T) {
	// Verify React widget uses bchat_session_id:<slug>
	p1, err := findWorkspaceFile("web/src/components/AgentChatWidget.tsx")
	require.NoError(t, err)
	reactContent, err := os.ReadFile(p1)
	require.NoError(t, err)
	require.Contains(t, string(reactContent), "`bchat_session_id:${tenantSlug}`")

	// Verify Vanilla JS widget/state uses bchat_session_id namespaced by tenant slug
	p2, err := findWorkspaceFile("widget/src/core/state.ts")
	require.NoError(t, err)
	stateContent, err := os.ReadFile(p2)
	require.NoError(t, err)
	require.Contains(t, string(stateContent), "`bchat_session_id:${this.tenantSlug}`")
}

func TestBChatLiveWidgetPollingToggle(t *testing.T) {
	// Verify React widget polling conditional loop
	p1, err := findWorkspaceFile("web/src/components/AgentChatWidget.tsx")
	require.NoError(t, err)
	reactContent, err := os.ReadFile(p1)
	require.NoError(t, err)
	reactStr := string(reactContent)
	require.Contains(t, reactStr, "setInterval")
	require.Contains(t, reactStr, "clearInterval")
	require.Contains(t, reactStr, "human_handoff_active")
	require.Contains(t, reactStr, "isOpen")
	require.Contains(t, reactStr, "isMinimized")

	// Verify Vanilla JS widget polling conditional loop
	p2, err := findWorkspaceFile("widget/src/ui/Widget.ts")
	require.NoError(t, err)
	widgetContent, err := os.ReadFile(p2)
	require.NoError(t, err)
	widgetStr := string(widgetContent)
	require.Contains(t, widgetStr, "setInterval")
	require.Contains(t, widgetStr, "clearInterval")
	require.Contains(t, widgetStr, "human_handoff_active")
	require.Contains(t, widgetStr, "isOpen")
	require.Contains(t, widgetStr, "isMinimized")
}

func TestBChatLiveEndToEndVisitorHumanReplyFlow(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "live-e2e-flow", true)
	defer ts.Close()

	// Create agent audience
	_, err := ts.CreateAgentAudience(ctx, &store.AgentAudience{
		TenantID: tenant.ID, AudienceType: "internal", Role: "assistant", Tone: "helpful",
		EmergencyPhone: "", RateLimitRPM: 60,
	})
	require.NoError(t, err)

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/chat/ext", handler.HandleChatExternal)
	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))
	e.GET("/api/v1/agent/:slug/chat/ext/transcript", handler.HandleGetExternalTranscript)

	// 1. Visitor initiates chat
	bodyChat1 := []byte(`{"message": "I need live operator help"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/live-e2e-flow/chat/ext", bytes.NewReader(bodyChat1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	c1.SetParamNames("slug")
	c1.SetParamValues("live-e2e-flow")
	err = handler.HandleChatExternal(c1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec1.Code)

	var respChat1 ChatResponse
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &respChat1))
	sessionID := respChat1.SessionID
	require.NotEmpty(t, sessionID)

	// 2. Operator takes over (Bridge Takeover)
	bodyTakeover := []byte(fmt.Sprintf(`{"session_id": "%s"}`, sessionID))
	now := time.Now().Unix()
	nonce1 := "takeover_nonce_e2e"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/live-e2e-flow/bridge/takeover", "application/json", now, nonce1, bodyTakeover)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/live-e2e-flow/bridge/takeover", bytes.NewReader(bodyTakeover))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce1)
	req2.Header.Set("X-Bridge-Signature", sig1)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var respTakeover BridgeTakeoverResponse
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &respTakeover))
	handoffID := respTakeover.HandoffID

	// 3. Visitor sends message during handoff (should be appended to transcript, but AI suppressed)
	bodyChat2 := []byte(fmt.Sprintf(`{"session_id": "%s", "message": "still waiting"}`, sessionID))
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/live-e2e-flow/chat/ext", bytes.NewReader(bodyChat2))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	c3 := e.NewContext(req3, rec3)
	c3.SetParamNames("slug")
	c3.SetParamValues("live-e2e-flow")
	err = handler.HandleChatExternal(c3)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec3.Code)

	var respChat2 ChatResponse
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &respChat2))
	require.Equal(t, "system", respChat2.Message.Role)
	require.Equal(t, "A human operator is handling this conversation.", respChat2.Message.Content)

	// 4. Operator replies (Bridge Reply) which delivers synchronously
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "%s", "handoff_id": "%s", "message_id": "operator-msg-e2e", "text": "Hello, I am a human operator."}`, sessionID, handoffID))
	nonce2 := "reply_nonce_e2e_longer"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/live-e2e-flow/bridge/reply", "application/json", now, nonce2, bodyReply)

	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/live-e2e-flow/bridge/reply", bytes.NewReader(bodyReply))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req4.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req4.Header.Set("X-Bridge-Nonce", nonce2)
	req4.Header.Set("X-Bridge-Signature", sig2)
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req4)
	require.Equal(t, http.StatusOK, rec4.Code)

	var respReply BridgeReplyResponse
	require.NoError(t, json.Unmarshal(rec4.Body.Bytes(), &respReply))
	require.Equal(t, "completed", respReply.Outbox.Status)
	require.Equal(t, "completed", respReply.WebChatDelivery.Status)

	// 5. Visitor polls transcript (GET /chat/ext/transcript) and sees human reply
	req5 := httptest.NewRequest(http.MethodGet, "/api/v1/agent/live-e2e-flow/chat/ext/transcript?session_id="+sessionID, nil)
	rec5 := httptest.NewRecorder()
	c5 := e.NewContext(req5, rec5)
	c5.SetParamNames("slug")
	c5.SetParamValues("live-e2e-flow")
	err = handler.HandleGetExternalTranscript(c5)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec5.Code)

	var respTranscript map[string]interface{}
	require.NoError(t, json.Unmarshal(rec5.Body.Bytes(), &respTranscript))
	messages := respTranscript["messages"].([]interface{})

	// We expect:
	// 0: Visitor: I need live operator help
	// 1: AI reply (from step 1)
	// 2: Visitor: still waiting (from step 3)
	// 3: Human Operator: Hello, I am a human operator. (from step 4)
	require.Len(t, messages, 4)

	lastMsg := messages[3].(map[string]interface{})
	require.Equal(t, "assistant", lastMsg["role"])
	require.Equal(t, "Hello, I am a human operator.", lastMsg["content"])
	require.Equal(t, "human_agent", lastMsg["kind"])

	// 6. Release handoff
	_, err = ts.GetDriver().UpdateBridgeHandoffRoutingModeCAS(
		ctx, tenant.ID, sessionID, respTakeover.Handoff.Generation, handoffID,
		respTakeover.Handoff.Version, store.BridgeRoutingModeHumanActive, store.BridgeRoutingModeClosed,
		"released", time.Now(),
	)
	require.NoError(t, err)

	// 7. Visitor sends another message, AI resumes
	bodyChat3 := []byte(fmt.Sprintf(`{"session_id": "%s", "message": "thanks"}`, sessionID))
	req6 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/live-e2e-flow/chat/ext", bytes.NewReader(bodyChat3))
	req6.Header.Set("Content-Type", "application/json")
	rec6 := httptest.NewRecorder()
	c6 := e.NewContext(req6, rec6)
	c6.SetParamNames("slug")
	c6.SetParamValues("live-e2e-flow")
	err = handler.HandleChatExternal(c6)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec6.Code)

	var respChat3 ChatResponse
	require.NoError(t, json.Unmarshal(rec6.Body.Bytes(), &respChat3))
	require.Equal(t, "assistant", respChat3.Message.Role)
	require.NotEqual(t, "A human operator is handling this conversation.", respChat3.Message.Content)
}

func TestBChatLiveTranscriptEndpointDoesNotReturnSessionIDOrInternalIDs(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-transcript-noleak")
	defer ts.Close()

	sessionID := "session-noleak-1"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "need help", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-leak",
		Text:            "operator reply",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	_ = service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID)

	e := echo.New()
	handler := NewHandler(service, ts)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/"+tenant.Slug+"/chat/ext/transcript?session_id="+sessionID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues(tenant.Slug)

	require.NoError(t, handler.HandleGetExternalTranscript(c))
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	for _, field := range []string{
		"session_id", "source", "source_id", "reply_id",
		"outbox_id", "handoff_id", "claim_token",
		"failure_code", "failure_message", "delivery_status",
	} {
		require.NotContains(t, body, field, "transcript response leaks %s", field)
	}
}

func TestBChatLiveTranscriptEndpointDoesNotLogRawSessionID(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-transcript-log-token")
	defer ts.Close()

	sessionID := "session-secret-log-token"
	_, err := ts.GetDriver().GetDB().ExecContext(ctx, "DROP TABLE agent_transcripts")
	require.NoError(t, err)

	var logs bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(oldLogger)

	e := echo.New()
	handler := NewHandler(service, ts)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/"+tenant.Slug+"/chat/ext/transcript?session_id="+sessionID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues(tenant.Slug)

	err = handler.HandleGetExternalTranscript(c)
	require.Error(t, err)
	require.NotContains(t, logs.String(), sessionID)
	require.Contains(t, logs.String(), `"tenantID":`+strconv.Itoa(int(tenant.ID)))
}

func TestBChatLivePollThenChatExternalDoesNotDuplicateHumanReply(t *testing.T) {
	ctx, ts, service, tenant := newBridgeChatTestService(t, "live-poll-chat-dup")
	defer ts.Close()

	sessionID := "session-poll-chat"
	session := service.memorySessions.GetOrCreate(tenant.ID, sessionID)
	session.Messages = []store.AgentMessage{{Role: "user", Content: "hello", Timestamp: time.Now()}}
	service.memorySessions.Update(session)

	handoff := setupLiveHandoff(t, ctx, ts, tenant.ID, sessionID)

	replyID := uuid.NewString()
	result, err := ts.CreateBridgeHandoffReplyAndOutboxIfActive(ctx, &store.CreateBridgeHandoffReply{
		ReplyID:         replyID,
		TenantID:        tenant.ID,
		SessionID:       sessionID,
		HandoffID:       handoff.HandoffID,
		ClientMessageID: "msg-poll",
		Text:            "operator reply",
		Now:             time.Now().Unix(),
	})
	require.NoError(t, err)

	require.NoError(t, service.DeliverWebChatReply(ctx, tenant.ID, result.Outbox.OutboxID))

	resp, err := service.ChatExternal(ctx, tenant.Slug, "127.0.0.1", "test", ChatRequest{
		SessionID: sessionID,
		Message:   "still here",
	})
	require.NoError(t, err)

	updatedSession := service.memorySessions.Get(tenant.ID, sessionID)
	assistantCount := 0
	for _, m := range updatedSession.Messages {
		if m.Role == "assistant" {
			assistantCount++
		}
	}
	require.Equal(t, 1, assistantCount, "ChatExternal must not duplicate human replies already in transcript")

	require.Equal(t, "system", resp.Message.Role)
	require.Equal(t, "A human operator is handling this conversation.", resp.Message.Content)
}

func findWorkspaceFile(relativePath string) (string, error) {
	prefixes := []string{
		"",
		"../../../../../",
		"../../../../",
		"../../../",
		"../../",
		"../",
	}
	for _, p := range prefixes {
		path := filepath.Join(p, relativePath)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("file not found: %s", relativePath)
}
