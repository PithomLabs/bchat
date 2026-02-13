package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/revrost/go-openrouter"
	"github.com/usememos/memos/store"
)

// ObserverBuffer manages background pre-computation of observations
// This reduces latency by buffering observations before the threshold is reached
type ObserverBuffer struct {
	mu            sync.RWMutex
	buffers       map[string]*BufferState // sessionID -> buffer state
	triggerChan   chan string             // channel for triggering buffer processing
	service       *Service
	config        *OMConfig
	lastCleanup   time.Time
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// BufferState holds the buffered observations for a session
type BufferState struct {
	mu                   sync.RWMutex
	PendingObservations  string
	PendingCurrentTask   string
	PendingSuggestedResp string
	TokenCount           int
	LastBufferTime       time.Time
	LastBufferedMsgIndex int
	ResourceID           string // For resource-scoped memory
	IsActive             bool   // Whether buffer has been activated
}

// NewObserverBuffer creates a new ObserverBuffer
func NewObserverBuffer(service *Service, config *OMConfig) *ObserverBuffer {
	ob := &ObserverBuffer{
		buffers:       make(map[string]*BufferState),
		triggerChan:   make(chan string, 100), // Buffered channel to handle bursts
		service:       service,
		config:        config,
		lastCleanup:   time.Now(),
		cleanupTicker: time.NewTicker(5 * time.Minute),
		stopCleanup:   make(chan struct{}),
	}
	go ob.bufferWorker()
	go ob.cleanupWorker()
	return ob
}

// bufferWorker processes buffer triggers in the background
func (ob *ObserverBuffer) bufferWorker() {
	for sessionID := range ob.triggerChan {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		if err := ob.runBufferObservation(ctx, sessionID); err != nil {
			slog.Error("Buffer observation failed", "session_id", sessionID, "error", err)
		}
		cancel()
	}
}

// cleanupWorker periodically removes stale buffers to prevent memory leaks
func (ob *ObserverBuffer) cleanupWorker() {
	const bufferExpiration = 30 * time.Minute

	for {
		select {
		case <-ob.cleanupTicker.C:
			ob.mu.Lock()
			now := time.Now()
			cleaned := 0
			for id, buf := range ob.buffers {
				buf.mu.RLock()
				lastTime := buf.LastBufferTime
				isActive := buf.IsActive
				buf.mu.RUnlock()

				// Clean up if: no recent buffer time OR buffer was activated and processed
				if (now.Sub(lastTime) > bufferExpiration) || (isActive && now.Sub(lastTime) > 5*time.Minute) {
					delete(ob.buffers, id)
					cleaned++
				}
			}
			ob.lastCleanup = now
			ob.mu.Unlock()
			if cleaned > 0 {
				slog.Debug("ObserverBuffer cleanup completed", "cleaned_buffers", cleaned, "remaining", len(ob.buffers))
			}
		case <-ob.stopCleanup:
			ob.cleanupTicker.Stop()
			return
		}
	}
}

// runBufferObservation performs a background observation for buffering
func (ob *ObserverBuffer) runBufferObservation(ctx context.Context, sessionID string) error {
	// Get the session
	session := ob.service.memorySessions.Get(sessionID)
	if session == nil {
		var err error
		session, err = ob.service.store.GetAgentSession(ctx, &store.FindAgentSession{ID: &sessionID})
		if err != nil || session == nil {
			return nil // Session not found, skip
		}
	}

	// Get config to check scope
	config := GetOMConfig().GetConfig()

	// Determine resource_id for resource-scoped memory
	resourceID := ""
	if config.Scope == OMScopeResource && session.UserID != nil {
		resourceID = fmt.Sprintf("user_%d", *session.UserID)
	}

	// Get existing observation log (with scope support)
	var obsLog *store.ObservationLog
	var err error
	if config.Scope == OMScopeResource && resourceID != "" {
		obsLog, err = ob.service.store.GetObservationLogByResource(ctx, resourceID)
	} else {
		obsLog, err = ob.service.store.GetObservationLog(ctx, sessionID)
	}
	if err != nil {
		return err
	}

	lastObservedIdx := -1
	if obsLog != nil {
		lastObservedIdx = obsLog.LastObservedMsgIndex
	}

	// Get or create buffer state
	ob.mu.Lock()
	buffer, exists := ob.buffers[sessionID]
	if !exists {
		buffer = &BufferState{
			LastBufferedMsgIndex: lastObservedIdx,
		}
		ob.buffers[sessionID] = buffer
	}
	ob.mu.Unlock()

	// Check if we have new messages to buffer
	buffer.mu.RLock()
	lastBufferedIdx := buffer.LastBufferedMsgIndex
	buffer.mu.RUnlock()

	if len(session.Messages) <= lastBufferedIdx+1 {
		return nil // Nothing new to buffer
	}

	newMessages := session.Messages[lastBufferedIdx+1:]
	if len(newMessages) == 0 {
		return nil
	}

	// Filter trivial messages
	filteredMessages := filterTrivialMessages(newMessages)
	if len(filteredMessages) == 0 {
		buffer.mu.Lock()
		buffer.LastBufferedMsgIndex = lastBufferedIdx + len(newMessages)
		buffer.mu.Unlock()
		return nil
	}

	// Format messages for the prompt
	var msgBuilder strings.Builder
	for _, msg := range filteredMessages {
		role := strings.ToUpper(string(msg.Role[0])) + string(msg.Role[1:])
		timestamp := time.Now().Format("15:04")
		msgBuilder.WriteString(fmt.Sprintf("**%s (%s):**\n%s\n\n", role, timestamp, msg.Content))
	}

	// Get LLM config
	model, apiKey := ob.service.getLLMConfig(ctx, session.TenantID)
	if apiKey == "" {
		return fmt.Errorf("LLM config missing for tenant %d", session.TenantID)
	}

	client := openrouter.NewClient(apiKey)

	// Call Observer LLM
	var existingObs string
	if obsLog != nil {
		existingObs = obsLog.ObservationLog
	}

	resp, err := ob.service.callObserverLLM(ctx, client, model, existingObs, msgBuilder.String())
	if err != nil {
		return err
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response from LLM")
	}

	output := resp.Choices[0].Message.Content.Text

	// Parse output
	newObservations := parseXMLTag(output, "observations")
	if newObservations == "" {
		newObservations = output
	}

	currentTask := parseXMLTag(output, "current-task")
	suggestedResponse := parseXMLTag(output, "suggested-response")

	// Store in buffer
	buffer.mu.Lock()
	buffer.PendingObservations = newObservations
	buffer.PendingCurrentTask = currentTask
	buffer.PendingSuggestedResp = suggestedResponse
	buffer.TokenCount = estimateTokens(newObservations)
	buffer.LastBufferTime = time.Now()
	buffer.LastBufferedMsgIndex = lastBufferedIdx + len(newMessages)
	buffer.ResourceID = resourceID // Store for resource-scoped memory
	buffer.mu.Unlock()

	slog.Debug("Buffer observation completed",
		"session_id", sessionID,
		"buffered_messages", len(newMessages),
		"buffer_tokens", buffer.TokenCount)

	return nil
}

// TriggerBuffer schedules a background observation for a session
func (ob *ObserverBuffer) TriggerBuffer(sessionID string) {
	select {
	case ob.triggerChan <- sessionID:
		// Trigger sent
	default:
		// Channel full, skip this trigger
		slog.Debug("Buffer trigger channel full, skipping", "session_id", sessionID)
	}
}

// GetAndActivateBuffer returns the buffered observations and marks them as activated
func (ob *ObserverBuffer) GetAndActivateBuffer(sessionID string) (observations, currentTask, suggestedResponse string, tokenCount int, lastMsgIndex int, resourceID string, ok bool) {
	ob.mu.RLock()
	buffer, exists := ob.buffers[sessionID]
	ob.mu.RUnlock()

	if !exists {
		return "", "", "", 0, 0, "", false
	}

	// Use write lock to prevent race condition when checking and setting IsActive
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	if buffer.PendingObservations == "" || buffer.IsActive {
		return "", "", "", 0, 0, "", false
	}

	// Mark as activated
	buffer.IsActive = true

	return buffer.PendingObservations, buffer.PendingCurrentTask, buffer.PendingSuggestedResp, buffer.TokenCount, buffer.LastBufferedMsgIndex, buffer.ResourceID, true
}

// ClearBuffer removes the buffer for a session (after activation)
func (ob *ObserverBuffer) ClearBuffer(sessionID string) {
	ob.mu.Lock()
	delete(ob.buffers, sessionID)
	ob.mu.Unlock()
}

// HasBuffer checks if a session has a pending buffer
func (ob *ObserverBuffer) HasBuffer(sessionID string) bool {
	ob.mu.RLock()
	buffer, exists := ob.buffers[sessionID]
	ob.mu.RUnlock()

	if !exists {
		return false
	}

	buffer.mu.RLock()
	defer buffer.mu.RUnlock()
	return buffer.PendingObservations != "" && !buffer.IsActive
}

// ShouldTriggerBuffer determines if a buffer observation should be triggered
// based on the buffer tokens configuration
func (ob *ObserverBuffer) ShouldTriggerBuffer(unobservedTokens int, threshold int) bool {
	if ob.config.BufferTokens <= 0 {
		return false // Buffering disabled
	}

	// Calculate buffer trigger point
	bufferTriggerPoint := int(float64(threshold) * ob.config.BufferTokens)
	return unobservedTokens >= bufferTriggerPoint
}

// ShouldActivateBuffer determines if the buffer should be activated
func (ob *ObserverBuffer) ShouldActivateBuffer(unobservedTokens int, threshold int) bool {
	return unobservedTokens >= threshold
}

// ShouldBlock determines if we should force synchronous observation
// This happens when the buffer can't keep up
func (ob *ObserverBuffer) ShouldBlock(unobservedTokens int, threshold int) bool {
	if ob.config.BlockAfter <= 0 {
		return false
	}

	blockThreshold := int(float64(threshold) * ob.config.BlockAfter)
	return unobservedTokens >= blockThreshold
}
