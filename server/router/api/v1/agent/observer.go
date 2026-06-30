package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/revrost/go-openrouter"
	"github.com/usememos/memos/store"
)

//go:embed prompts/observer.txt
var observerSystemPrompt string

//go:embed prompts/reflector.txt
var reflectorSystemPrompt string

// Trivial message patterns to skip during observation
var trivialPatterns = []string{
	"^ok$",
	"^ok\\.?$",
	"^thanks?$",
	"^thank you$",
	"^yeah$",
	"^yes$",
	"^no$",
	"^okay$",
	"^sure$",
	"^got it$",
	"^cool$",
	"^nice$",
	"^great$",
	"^perfect$",
	"^lol$",
	"^haha$",
}

var trivialRegex *regexp.Regexp

func init() {
	// Compile regex patterns for trivial message detection
	var patterns []string
	for _, p := range trivialPatterns {
		patterns = append(patterns, p)
	}
	// Use simpler pattern without case-insensitive flag in the pattern itself
	// and without the emoji patterns which cause escaping issues
	pattern := "^(?i)(" + strings.Join(patterns, "|") + ")"
	trivialRegex = regexp.MustCompile(pattern)
}

// RunObserver executes the Observational Memory pipeline for a given session.
// It retrieves recent messages, generates observations using an LLM, and persists them.
// If observations grow too large, it triggers the Reflector to compress them.
func (s *Service) RunObserver(ctx context.Context, tenantID int32, sessionID string) error {
	// Get configuration
	config := GetOMConfig().GetConfig()

	// Check if OM is enabled
	if !config.Enabled {
		slog.Debug("Observational Memory is disabled")
		return nil
	}

	startTime := time.Now()

	// 1. Retrieve Session
	// We check in-memory cache first
	session := s.memorySessions.Get(tenantID, sessionID)
	if session == nil {
		// Fallback to store if not in memory (common for sessions that have been idle or after a restart)
		var err error
		session, err = s.store.GetAgentSession(ctx, &store.FindAgentSession{ID: &sessionID, TenantID: &tenantID})
		if err != nil {
			return fmt.Errorf("failed to retrieve session %s from database: %w", sessionID, err)
		}
		if session == nil {
			return fmt.Errorf("session %s not found in memory or database", sessionID)
		}
	}

	// 2. Retrieve Existing Observations (with scope support)
	var obsLog *store.ObservationLog
	var err error

	// Determine resource_id for resource-scoped memory
	resourceID := ""
	if config.Scope == OMScopeResource && session.UserID != nil {
		resourceID = fmt.Sprintf("user_%d", *session.UserID)
	}

	// Query based on scope
	if config.Scope == OMScopeResource && resourceID != "" {
		// Resource scope: Get observations by resource_id (cross-conversation)
		obsLog, err = s.store.GetObservationLogByResource(ctx, resourceID)
		if err != nil {
			return fmt.Errorf("failed to get observation log by resource: %w", err)
		}
	} else {
		// Thread scope (default): Get observations by session_id
		obsLog, err = s.store.GetObservationLog(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("failed to get observation log: %w", err)
		}
	}

	if obsLog == nil {
		obsLog = &store.ObservationLog{
			SessionID:            sessionID,
			TenantID:             session.TenantID,
			ResourceID:           resourceID,
			ObservationLog:       "",
			LastObservedMsgIndex: -1,
			TokensInLog:          0,
		}
	}

	// 3. Determine Messages to Observe
	// We only want to observe messages since the last observation.
	lastIdx := obsLog.LastObservedMsgIndex
	if len(session.Messages) <= lastIdx+1 {
		return nil // Nothing new to observe
	}

	newMessages := session.Messages[lastIdx+1:]
	if len(newMessages) == 0 {
		return nil
	}

	// 3.5. Selective Observation - filter out trivial messages
	filteredMessages := filterTrivialMessages(newMessages)
	if len(filteredMessages) == 0 {
		// Update the index even if we skip trivial messages
		obsLog.LastObservedMsgIndex = lastIdx + len(newMessages)
		s.store.UpsertObservationLog(ctx, obsLog)
		slog.Debug("All messages were trivial, skipping observation", "session_id", sessionID, "count", len(newMessages))
		return nil
	}

	// Format messages for the prompt
	var msgBuilder strings.Builder
	for _, msg := range filteredMessages {
		role := strings.ToUpper(string(msg.Role[0])) + string(msg.Role[1:])
		timestamp := time.Now().Format("15:04") // Approximate timestamps
		msgBuilder.WriteString(fmt.Sprintf("**%s (%s):**\n%s\n\n", role, timestamp, msg.Content))
	}

	// 5. Call LLM (Observer) with retry logic
	model, apiKey := s.getLLMConfig(ctx, session.TenantID)
	if apiKey == "" {
		return fmt.Errorf("LLM config missing for tenant %d", session.TenantID)
	}

	client := newOpenRouterClient(apiKey)

	// Call Observer LLM with retry logic
	var resp openrouter.ChatCompletionResponse
	err = withRetry(ctx, config.RetryAttempts, config.RetryDelayMs, func() error {
		resp, err = s.callObserverLLM(ctx, client, model, obsLog.ObservationLog, msgBuilder.String())
		return err
	})
	if err != nil {
		slog.Error("Observer LLM call failed",
			"session_id", sessionID,
			"error", err)
		return fmt.Errorf("observer LLM call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response from LLM")
	}

	output := resp.Choices[0].Message.Content.Text

	// 6. Parse Output
	newObservations := parseXMLTag(output, "observations")
	if newObservations == "" {
		// Fallback: entire output might be observations if no tags
		newObservations = output
	}

	// 6.5. Extract current task and suggested response for continuity
	currentTask := parseXMLTag(output, "current-task")
	suggestedResponse := parseXMLTag(output, "suggested-response")

	// 7. Merge and Check Token Count
	updatedLog := obsLog.ObservationLog
	if updatedLog != "" {
		updatedLog += "\n"
	}
	updatedLog += newObservations

	tokenCount := estimateTokens(updatedLog)

	// 8. Reflector Logic (Compression)
	reflectorTriggered := false
	if tokenCount > config.TokenThreshold {
		slog.Info("Observation log too large, triggering reflector",
			"session_id", sessionID,
			"tokens", tokenCount,
			"threshold", config.TokenThreshold)

		tokenCountBefore := tokenCount
		var reflectedLog string
		err = withRetry(ctx, config.RetryAttempts, config.RetryDelayMs, func() error {
			reflectedLog, err = s.runReflector(ctx, client, model, updatedLog, &config)
			return err
		})
		if err == nil {
			updatedLog = reflectedLog
			tokenCount = estimateTokens(updatedLog)
			reflectorTriggered = true

			// Memory Analytics: Track compression ratio
			if tokenCountBefore > 0 {
				compressionRatio := float64(tokenCount) / float64(tokenCountBefore)
				slog.Info("Reflector compression completed",
					"session_id", sessionID,
					"tokens_before", tokenCountBefore,
					"tokens_after", tokenCount,
					"ratio", fmt.Sprintf("%.2f", compressionRatio))
			}

			// 8b. Index consolidated observations to RAG (Hybrid OM + RAG)
			// Replace old observations with consolidated version
			if config.HybridEnabled && config.HybridIndexObservations && s.vectorDB != nil {
				indexer := NewObservationIndexerWithConfig(s.vectorDB, config.HybridCompression, config.HybridTTLDays)
				indexCtx := s.withTenantEmbeddingAPIKey(ctx, session.TenantID)
				consolidatedObsLog := &store.ObservationLog{
					SessionID:      sessionID,
					ObservationLog: updatedLog,
					ResourceID:     obsLog.ResourceID,
				}
				if err := indexer.IndexReflectorObservations(indexCtx, consolidatedObsLog, session.TenantID, sessionID); err != nil {
					slog.Error("Failed to index reflector observations to RAG",
						"session_id", sessionID,
						"error", err)
				}
			}
		} else {
			slog.Error("Reflector failed, continuing with uncompressed log",
				"session_id", sessionID,
				"error", err)
		}
	}

	// 9. Persist
	obsLog.ObservationLog = updatedLog
	obsLog.LastObservedMsgIndex = lastIdx + len(newMessages)
	obsLog.TokensInLog = tokenCount
	obsLog.CurrentTask = currentTask
	obsLog.SuggestedResponse = suggestedResponse
	// Ensure resource_id is set for resource-scoped memory
	if config.Scope == OMScopeResource && obsLog.ResourceID == "" && resourceID != "" {
		obsLog.ResourceID = resourceID
	}

	_, err = s.store.UpsertObservationLog(ctx, obsLog)
	if err != nil {
		return fmt.Errorf("failed to persist observation log: %w", err)
	}

	// 10. Index observations to RAG (Hybrid OM + RAG)
	if config.HybridEnabled && config.HybridIndexObservations && s.vectorDB != nil {
		indexer := NewObservationIndexerWithConfig(s.vectorDB, config.HybridCompression, config.HybridTTLDays)
		indexCtx := s.withTenantEmbeddingAPIKey(ctx, session.TenantID)
		if err := indexer.IndexObservation(indexCtx, obsLog, session.TenantID); err != nil {
			// Log error but don't fail the observation
			slog.Error("Failed to index observations to RAG",
				"session_id", sessionID,
				"error", err)
		}
	}

	// Enhanced logging with analytics
	durationMs := time.Since(startTime).Milliseconds()
	slog.Info("Observer completed successfully",
		"session_id", sessionID,
		"resource_id", resourceID,
		"scope", config.Scope,
		"new_messages", len(filteredMessages),
		"skipped_trivial", len(newMessages)-len(filteredMessages),
		"total_tokens", tokenCount,
		"reflector_triggered", reflectorTriggered,
		"hybrid_indexed", config.HybridEnabled && config.HybridIndexObservations,
		"duration_ms", durationMs)

	return nil
}

// callObserverLLM makes the actual LLM call for observation
// Uses prompt caching for the system prompt and existing observations
func (s *Service) callObserverLLM(ctx context.Context, client *openrouter.Client, model string, existingObservations string, messagesToObserve string) (openrouter.ChatCompletionResponse, error) {
	// Create proper messages with caching structure:
	// [System Prompt] <- Cacheable (stable)
	// [Previous Observations] <- Cacheable (semi-stable, changes after reflection)
	// [New Messages] <- Dynamic (changes every call)

	// System message with cache control for prompt caching
	systemMsg := openrouter.SystemMessage(observerSystemPrompt)

	// Build user content with cacheable sections
	// We structure it so the previous observations can be cached
	userContent := fmt.Sprintf("## Previous Observations\n\n%s\n\n## New Message History to Observe\n\n%s",
		existingObservations, messagesToObserve)
	userMsg := openrouter.UserMessage(userContent)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: []openrouter.ChatCompletionMessage{systemMsg, userMsg},
	})
	if err != nil {
		return openrouter.ChatCompletionResponse{}, fmt.Errorf("LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return openrouter.ChatCompletionResponse{}, fmt.Errorf("no response from LLM")
	}
	return resp, nil
}

// callObserverLLMWithCache makes the LLM call with explicit prompt caching
// This uses OpenRouter/Anthropic-style cache control for 4-10x cost reduction
func (s *Service) callObserverLLMWithCache(ctx context.Context, client *openrouter.Client, model string, existingObservations string, messagesToObserve string) (openrouter.ChatCompletionResponse, error) {
	// Create messages with cache control for prompt caching
	// Structure: [System Prompt (cached)] -> [Observations (cached)] -> [New Messages (dynamic)]

	// System message with cache control using MultiContent
	systemMsg := openrouter.ChatCompletionMessage{
		Role: openrouter.ChatMessageRoleSystem,
		Content: openrouter.Content{
			Multi: []openrouter.ChatMessagePart{
				{
					Type: openrouter.ChatMessagePartTypeText,
					Text: observerSystemPrompt,
					CacheControl: &openrouter.CacheControl{
						Type: "ephemeral",
					},
				},
			},
		},
	}

	// Previous observations as a separate message with cache control
	obsMsg := openrouter.ChatCompletionMessage{
		Role: openrouter.ChatMessageRoleUser,
		Content: openrouter.Content{
			Multi: []openrouter.ChatMessagePart{
				{
					Type: openrouter.ChatMessagePartTypeText,
					Text: fmt.Sprintf("## Previous Observations\n\n%s", existingObservations),
					CacheControl: &openrouter.CacheControl{
						Type: "ephemeral",
					},
				},
			},
		},
	}

	// New messages to observe (dynamic, no caching)
	newMsgsContent := fmt.Sprintf("## New Message History to Observe\n\n%s", messagesToObserve)
	newMsgsMsg := openrouter.UserMessage(newMsgsContent)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: []openrouter.ChatCompletionMessage{systemMsg, obsMsg, newMsgsMsg},
	})
	if err != nil {
		return openrouter.ChatCompletionResponse{}, fmt.Errorf("LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return openrouter.ChatCompletionResponse{}, fmt.Errorf("no response from LLM")
	}

	// Log cache usage if available (OpenRouter provides these fields)
	slog.Debug("Observer LLM call completed",
		"total_tokens", resp.Usage.TotalTokens,
		"prompt_tokens", resp.Usage.PromptTokens,
		"completion_tokens", resp.Usage.CompletionTokens)

	return resp, nil
}

func (s *Service) runReflector(ctx context.Context, client *openrouter.Client, model string, observations string, config *OMConfig) (string, error) {
	prompt := fmt.Sprintf("%s\n\n## OBSERVATIONS TO REFLECT ON\n\n%s", reflectorSystemPrompt, observations)

	userMsg := openrouter.UserMessage(prompt)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: []openrouter.ChatCompletionMessage{userMsg},
	})
	if err != nil {
		return "", fmt.Errorf("reflector LLM call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from Reflector")
	}

	output := resp.Choices[0].Message.Content.Text
	reflected := parseXMLTag(output, "observations")
	if reflected == "" {
		reflected = output
	}
	return reflected, nil
}

// Helper to extract content within XML tags
func parseXMLTag(content, tagName string) string {
	// Simple regex for <tagName>...</tagName>
	// Handles multiline and case insensitivity
	re := regexp.MustCompile(fmt.Sprintf(`(?is)<%s>(.*?)</%s>`, tagName, tagName))
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// Approximate token count (1 token ~= 4 chars)
// Note: This is a simple approximation. For production use, consider using
// a proper tokenizer like tiktoken for more accurate counting.
func estimateTokens(text string) int {
	return len(text) / 4
}

// withRetry executes a function with retry logic
// maxAttempts: number of attempts including the first
// delayMs: delay in milliseconds between attempts
func withRetry(ctx context.Context, maxAttempts int, delayMs int, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err

			// Check if error is retryable
			if !isRetryable(err) {
				return err
			}

			// Don't wait after the last attempt
			if attempt < maxAttempts {
				slog.Warn("Attempt failed, retrying",
					"attempt", attempt,
					"max_attempts", maxAttempts,
					"error", err)
				time.Sleep(time.Duration(delayMs) * time.Millisecond)
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// isRetryable determines if an error should trigger a retry
// Currently retries on all errors, but could be enhanced to check for
// specific error types like network errors, rate limits, etc.
func isRetryable(err error) bool {
	// Could add more sophisticated logic here
	// For now, retry on all errors except context cancellation
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// filterTrivialMessages filters out trivial messages like "ok", "thanks", etc.
// that don't need to be observed for memory purposes
func filterTrivialMessages(messages []store.AgentMessage) []store.AgentMessage {
	if len(messages) == 0 {
		return messages
	}

	var result []store.AgentMessage
	for _, msg := range messages {
		if !isTrivialMessage(msg.Content) {
			result = append(result, msg)
		}
	}
	return result
}

// isTrivialMessage checks if a message content is trivial (acknowledgment, greeting, etc.)
func isTrivialMessage(content string) bool {
	if content == "" {
		return true
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}

	// Use pre-compiled regex for efficiency
	return trivialRegex.MatchString(trimmed)
}

// storeObservationFromBuffer stores a pre-computed observation from the buffer
// This is used when the buffer is activated at the threshold
func (s *Service) storeObservationFromBuffer(ctx context.Context, tenantID int32, sessionID string, observations string, currentTask string, suggestedResponse string, lastMsgIndex int, resourceID string) error {
	// Get the session to retrieve tenant ID
	session := s.memorySessions.Get(tenantID, sessionID)
	if session == nil {
		var err error
		session, err = s.store.GetAgentSession(ctx, &store.FindAgentSession{ID: &sessionID, TenantID: &tenantID})
		if err != nil || session == nil {
			return fmt.Errorf("session %s not found", sessionID)
		}
	}

	// Get config to check scope
	config := GetOMConfig().GetConfig()

	// Get existing observation log (with scope support)
	var obsLog *store.ObservationLog
	var err error
	if config.Scope == OMScopeResource && resourceID != "" {
		obsLog, err = s.store.GetObservationLogByResource(ctx, resourceID)
	} else {
		obsLog, err = s.store.GetObservationLog(ctx, sessionID)
	}
	if err != nil {
		return fmt.Errorf("failed to get observation log: %w", err)
	}
	if obsLog == nil {
		obsLog = &store.ObservationLog{
			SessionID:      sessionID,
			TenantID:       session.TenantID,
			ResourceID:     resourceID,
			ObservationLog: "",
		}
	}

	// Merge with existing observations
	updatedLog := obsLog.ObservationLog
	if updatedLog != "" {
		updatedLog += "\n"
	}
	updatedLog += observations

	// Update the log
	obsLog.ObservationLog = updatedLog
	obsLog.LastObservedMsgIndex = lastMsgIndex
	obsLog.TokensInLog = estimateTokens(updatedLog)
	obsLog.CurrentTask = currentTask
	obsLog.SuggestedResponse = suggestedResponse

	_, err = s.store.UpsertObservationLog(ctx, obsLog)
	if err != nil {
		return fmt.Errorf("failed to persist buffered observation: %w", err)
	}

	slog.Info("Buffered observation stored successfully",
		"session_id", sessionID,
		"tokens", obsLog.TokensInLog,
		"last_msg_index", lastMsgIndex)

	return nil
}
