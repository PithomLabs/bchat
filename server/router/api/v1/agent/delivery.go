package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/usememos/memos/store"
)

// DeliverWebChatReply claims and delivers a specific human operator reply outbox row
// to the visitor session transcript and completes the outbox settlement.
func (s *Service) DeliverWebChatReply(ctx context.Context, tenantID int32, outboxID string) error {
	if tenantID <= 0 || len(outboxID) != 36 {
		return store.ErrBridgeInvalidArgument
	}

	if !s.store.SupportsBridgeDelivery() {
		slog.Warn("bridge delivery not supported by database driver", "tenant_id", tenantID, "outbox_id", outboxID)
		return store.ErrBridgeUnsupportedDatabase
	}

	// 1. Claim exactly the newly created outbox row
	row, err := s.store.ClaimBridgeReplyOutboxByOutboxID(ctx, tenantID, outboxID, "webchat-delivery-worker", time.Now(), 5*60)
	if err != nil {
		// Return error directly so caller can inspect completed/failed/conflict states
		return err
	}
	if row == nil {
		return fmt.Errorf("outbox row not claimed")
	}

	// 2. Load the reply text from database
	reply, err := s.store.GetBridgeHandoffReplyByReplyID(ctx, tenantID, row.ReplyID)
	if err != nil {
		_, _ = s.store.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
			TenantID:       tenantID,
			OutboxID:       outboxID,
			ClaimToken:     *row.ClaimToken,
			Now:            time.Now().Unix(),
			FailureCode:    "webchat_delivery_failed",
			FailureMessage: "failed to load reply content",
		})
		return fmt.Errorf("failed to load reply content: %w", err)
	}
	if reply == nil {
		_, _ = s.store.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
			TenantID:       tenantID,
			OutboxID:       outboxID,
			ClaimToken:     *row.ClaimToken,
			Now:            time.Now().Unix(),
			FailureCode:    "webchat_reply_missing",
			FailureMessage: "reply content not found",
		})
		return fmt.Errorf("reply not found: %s", row.ReplyID)
	}

	// 3. Load the durable transcript from database
	transcript, err := s.store.GetAgentTranscript(ctx, &store.FindAgentTranscript{
		SessionID: &row.SessionID,
		TenantID:  &row.TenantID,
	})
	if err != nil {
		_, _ = s.store.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
			TenantID:       tenantID,
			OutboxID:       outboxID,
			ClaimToken:     *row.ClaimToken,
			Now:            time.Now().Unix(),
			FailureCode:    "webchat_delivery_failed",
			FailureMessage: "failed to load session transcript",
		})
		return fmt.Errorf("failed to load session transcript: %w", err)
	}

	// 4. Duplicate prevention scan and session rebuild
	var session *store.AgentSession
	if transcript != nil {
		// Scan durable transcript messages for duplicates
		for _, m := range transcript.Messages {
			if m.Source == "bridge_human_reply" && m.SourceID == row.ReplyID {
				// Settle completed (already delivered)
				_, err = s.store.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
					TenantID:   tenantID,
					OutboxID:   outboxID,
					ClaimToken: *row.ClaimToken,
					Now:        time.Now().Unix(),
				})
				// Re-sync memory session
				s.rebuildMemorySession(ctx, tenantID, row.SessionID, transcript)
				return err
			}
		}
		// Rebuild memory session from transcript
		session = s.rebuildMemorySession(ctx, tenantID, row.SessionID, transcript)
	} else {
		// If transcript is not in database yet, look up memory session
		session = s.memorySessions.Get(row.TenantID, row.SessionID)
		if session == nil {
			_, _ = s.store.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
				TenantID:       tenantID,
				OutboxID:       outboxID,
				ClaimToken:     *row.ClaimToken,
				Now:            time.Now().Unix(),
				FailureCode:    "webchat_session_missing",
				FailureMessage: "visitor session not found",
			})
			return fmt.Errorf("session not found: %s", row.SessionID)
		}

		// Scan memory session for duplicates
		for _, m := range session.Messages {
			if m.Source == "bridge_human_reply" && m.SourceID == row.ReplyID {
				_, err = s.store.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
					TenantID:   tenantID,
					OutboxID:   outboxID,
					ClaimToken: *row.ClaimToken,
					Now:        time.Now().Unix(),
				})
				// Make transcript durable
				_ = s.saveTranscript(ctx, session, "", "system")
				return err
			}
		}
	}

	// 5. Append message to session transcript
	session.Messages = append(session.Messages, store.AgentMessage{
		Role:      "assistant",
		Content:   reply.Text,
		Timestamp: time.Unix(reply.CreatedAt, 0),
		Source:    "bridge_human_reply",
		SourceID:  reply.ReplyID,
	})
	session.MessageCount = len(session.Messages)

	// Update memory session map
	s.memorySessions.Update(session)

	// Persist the updated transcript durably
	err = s.saveTranscript(ctx, session, "", "system")
	if err != nil {
		_, _ = s.store.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
			TenantID:       tenantID,
			OutboxID:       outboxID,
			ClaimToken:     *row.ClaimToken,
			Now:            time.Now().Unix(),
			FailureCode:    "webchat_append_failed",
			FailureMessage: "failed to persist transcript",
		})
		return err
	}

	// 6. Complete outbox settlement
	_, err = s.store.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
		TenantID:   tenantID,
		OutboxID:   outboxID,
		ClaimToken: *row.ClaimToken,
		Now:        time.Now().Unix(),
	})
	return err
}

// rebuildMemorySession reconstructs the AgentSession object from an AgentTranscript and updates memorySessions.
func (s *Service) rebuildMemorySession(ctx context.Context, tenantID int32, sessionID string, transcript *store.AgentTranscript) *store.AgentSession {
	activeHandoff, err := s.store.FindActiveBridgeHandoff(ctx, tenantID, sessionID)
	if err != nil && !errors.Is(err, store.ErrBridgeUnsupportedDatabase) {
		slog.Error("failed to find active bridge handoff during session rebuild", "tenantID", tenantID, "error", err)
	}

	phase := "triage"
	if activeHandoff != nil {
		phase = "handoff"
	}

	existing := s.memorySessions.Get(tenantID, sessionID)
	if existing != nil &&
		(existing.UpdatedAt.After(transcript.LastMessageAt) ||
			existing.MessageCount > transcript.MessageCount) {
		existing.Phase = phase
		s.memorySessions.Update(existing)
		return existing
	}

	session := s.memorySessions.GetOrCreate(tenantID, sessionID)
	session.Messages = transcript.Messages
	session.MessageCount = transcript.MessageCount
	session.CustomerName = transcript.CustomerName
	session.CustomerPhone = transcript.CustomerPhone
	session.CustomerLocation = transcript.CustomerLocation
	session.CurrentIntent = transcript.DetectedIntent
	session.CreatedAt = transcript.StartedAt
	session.UpdatedAt = transcript.LastMessageAt
	session.IsCompleted = transcript.IsCompleted
	session.CompletionReason = transcript.CompletionReason
	session.Phase = phase

	s.memorySessions.Update(session)
	return session
}
