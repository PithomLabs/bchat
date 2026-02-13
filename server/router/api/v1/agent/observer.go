package agent

import (
	"context"
	_ "embed"
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

// RunObserver executes the Observational Memory pipeline for a given session.
// It retrieves recent messages, generates observations using an LLM, and persists them.
// If observations grow too large, it triggers the Reflector to compress them.
func (s *Service) RunObserver(ctx context.Context, sessionID string) error {
	// 1. Retrieve Session
	// We check in-memory cache first
	session := s.memorySessions.Get(sessionID)
	if session == nil {
		// Fallback to store if not in memory (common for sessions that have been idle or after a restart)
		var err error
		session, err = s.store.GetAgentSession(ctx, &store.FindAgentSession{ID: &sessionID})
		if err != nil {
			return fmt.Errorf("failed to retrieve session %s from database: %w", sessionID, err)
		}
		if session == nil {
			return fmt.Errorf("session %s not found in memory or database", sessionID)
		}
		// Optional: We could put it back into memory, but RunObserver is usually called asynchronously
		// after a message is already processed and stored.
	}

	// 2. Retrieve Existing Observations
	obsLog, err := s.store.GetObservationLog(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get observation log: %w", err)
	}
	if obsLog == nil {
		obsLog = &store.ObservationLog{
			SessionID:            sessionID,
			TenantID:             session.TenantID,
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

	// Format messages for the prompt
	var msgBuilder strings.Builder
	for _, msg := range newMessages {
		role := strings.ToUpper(string(msg.Role[0])) + string(msg.Role[1:])
		timestamp := time.Now().Format("15:04") // Approximate timestamps
		msgBuilder.WriteString(fmt.Sprintf("**%s (%s):**\n%s\n\n", role, timestamp, msg.Content))
	}

	// 5. Call LLM (Observer)
	model, apiKey := s.getLLMConfig(ctx, session.TenantID)
	if apiKey == "" {
		return fmt.Errorf("LLM config missing for tenant %d", session.TenantID)
	}

	client := openrouter.NewClient(apiKey)

	// Create proper messages
	systemMsg := openrouter.SystemMessage(observerSystemPrompt)
	userContent := fmt.Sprintf("## Previous Observations\n\n%s\n\n## New Message History to Observe\n\n%s",
		obsLog.ObservationLog, msgBuilder.String())
	userMsg := openrouter.UserMessage(userContent)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: []openrouter.ChatCompletionMessage{systemMsg, userMsg},
	})
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
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

	// 7. Merge and Check Token Count
	updatedLog := obsLog.ObservationLog
	if updatedLog != "" {
		updatedLog += "\n"
	}
	updatedLog += newObservations

	tokenCount := estimateTokens(updatedLog)

	// 8. Reflector Logic (Compression)
	const TokenThreshold = 2000 // Configurable?
	if tokenCount > TokenThreshold {
		slog.Info("Observation log too large, triggering reflector", "session_id", sessionID, "tokens", tokenCount)
		reflectedLog, err := s.runReflector(ctx, client, model, updatedLog)
		if err == nil {
			updatedLog = reflectedLog
			tokenCount = estimateTokens(updatedLog)
		} else {
			slog.Error("Reflector failed", "error", err)
			// Continue with uncompressed log rather than failing entirely
		}
	}

	// 9. Persist
	obsLog.ObservationLog = updatedLog
	obsLog.LastObservedMsgIndex = lastIdx + len(newMessages)
	obsLog.TokensInLog = tokenCount

	_, err = s.store.UpsertObservationLog(ctx, obsLog)
	if err != nil {
		return fmt.Errorf("failed to persist observation log: %w", err)
	}

	return nil
}

func (s *Service) runReflector(ctx context.Context, client *openrouter.Client, model string, observations string) (string, error) {
	prompt := fmt.Sprintf("%s\n\n## OBSERVATIONS TO REFLECT ON\n\n%s", reflectorSystemPrompt, observations)

	userMsg := openrouter.UserMessage(prompt)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: []openrouter.ChatCompletionMessage{userMsg},
	})
	if err != nil {
		return "", err
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
func estimateTokens(text string) int {
	return len(text) / 4
}
