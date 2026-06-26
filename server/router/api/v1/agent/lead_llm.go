package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/revrost/go-openrouter"
	"github.com/usememos/memos/store"
)

// ============================================================================
// LAYER 3: LLM-POWERED EXTRACTION
// ============================================================================

const (
	llmExtractionTimeout = 2 * time.Second
	llmMaxTokens         = 300
)

// llmExtractionResponse is the structured response from the LLM.
type llmExtractionResponse struct {
	Name     *fieldExtraction `json:"name,omitempty"`
	Email    *fieldExtraction `json:"email,omitempty"`
	Phone    *fieldExtraction `json:"phone,omitempty"`
	Location *fieldExtraction `json:"location,omitempty"`
	Declined bool             `json:"declined"`
}

type fieldExtraction struct {
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
	Corrected  bool    `json:"corrected"`
}

// newLLMClient creates an OpenRouter client with a short timeout for extraction.
func newLLMClient(timeout time.Duration) *openrouter.Client {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	config := openrouter.DefaultConfig(apiKey)
	config.HTTPClient = &http.Client{Timeout: timeout}
	return openrouter.NewClientWithConfig(*config)
}

// ExtractContactInfoLLM uses an LLM to extract contact info from a message.
// Returns nil if extraction fails or times out.
func ExtractContactInfoLLM(ctx context.Context, messageContent string, conversationHistory []store.AgentMessage, currentDraft *LeadDraft) *ExtractionResult {
	if !hasLLMConfig() {
		return nil
	}

	// Build conversation context (last 3 messages)
	contextMessages := buildContextMessages(conversationHistory, 3)

	// Build prompt
	systemPrompt := `You are a lead extraction assistant for a customer chat widget. Extract customer contact information from messages.

RULES:
1. Only extract info the customer explicitly provides. Do NOT infer or guess.
2. Names should be proper nouns (first name, or first + last). Reject common words as names.
3. Emails must contain @ and a valid domain.
4. Phone numbers should have 7-15 digits, optionally with + prefix.
5. If the customer is CORRECTING info (e.g., "No, I meant...", "actually it's..."), mark as corrected.
6. If the customer DECLINES to share (e.g., "I'd rather not", "no thanks"), mark as declined.
7. If no new contact info is found, return empty fields.

Respond ONLY with valid JSON:
{
  "name": {"value": "string or null", "confidence": 0.0-1.0, "corrected": true/false},
  "email": {"value": "string or null", "confidence": 0.0-1.0, "corrected": true/false},
  "phone": {"value": "string or null", "confidence": 0.0-1.0, "corrected": true/false},
  "location": {"value": "string or null", "confidence": 0.0-1.0, "corrected": true/false},
  "declined": false
}`

	// Build messages
	messages := []openrouter.ChatCompletionMessage{
		openrouter.SystemMessage(systemPrompt),
	}

	// Add conversation context
	for _, msg := range contextMessages {
		role := "user"
		if msg.Role == "assistant" {
			role = "assistant"
		}
		messages = append(messages, openrouter.ChatCompletionMessage{
			Role:    role,
			Content: openrouter.Content{Text: msg.Content},
		})
	}

	// Add current draft state
	if currentDraft != nil && currentDraft.Name != "" {
		draftInfo := fmt.Sprintf("Previously extracted (may be incomplete): Name=%s, Email=%s, Phone=%s",
			currentDraft.Name, currentDraft.Email, currentDraft.Phone)
		messages = append(messages, openrouter.SystemMessage(draftInfo))
	}

	// Add the message to extract from
	messages = append(messages, openrouter.UserMessage(fmt.Sprintf("Extract contact info from this message:\n\n%s", messageContent)))

	// Call LLM — respect precedence: env var > hardcoded default
	model := getEnvOrDefault("LLM_MODEL", "openai/gpt-4o-mini")
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		return nil
	}

	client := newLLMClient(llmExtractionTimeout)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: llmMaxTokens,
	})
	if err != nil {
		slog.Warn("LLM extraction failed", "error", err)
		return nil
	}

	if len(resp.Choices) == 0 {
		return nil
	}

	// Parse response
	responseText := resp.Choices[0].Message.Content.Text
	llmResult, err := parseLLMExtractionResponse(responseText)
	if err != nil {
		slog.Warn("Failed to parse LLM extraction response", "error", err, "response", responseText)
		return nil
	}

	// Convert to ExtractionResult
	result := &ExtractionResult{
		Source: "llm",
	}

	if llmResult.Name != nil && llmResult.Name.Value != "" {
		result.Name = llmResult.Name.Value
		result.Confidence = llmResult.Name.Confidence
		result.Corrected = llmResult.Name.Corrected
	}
	if llmResult.Email != nil && llmResult.Email.Value != "" {
		result.Email = llmResult.Email.Value
		result.Confidence = llmResult.Email.Confidence
		result.Corrected = llmResult.Email.Corrected
	}
	if llmResult.Phone != nil && llmResult.Phone.Value != "" {
		result.Phone = llmResult.Phone.Value
		result.Confidence = llmResult.Phone.Confidence
		result.Corrected = llmResult.Phone.Corrected
	}
	if llmResult.Location != nil && llmResult.Location.Value != "" {
		result.Address = llmResult.Location.Value
	}
	result.Declined = llmResult.Declined

	if result.Name == "" && result.Email == "" && result.Phone == "" {
		return nil
	}

	return result
}

func parseLLMExtractionResponse(responseText string) (*llmExtractionResponse, error) {
	// Strip markdown code fences if present
	responseText = cleanJSONResponse(responseText)

	var result llmExtractionResponse
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LLM response: %w", err)
	}
	return &result, nil
}

func buildContextMessages(messages []store.AgentMessage, maxCount int) []store.AgentMessage {
	if len(messages) <= maxCount {
		return messages
	}
	return messages[len(messages)-maxCount:]
}

// hasLLMConfig checks if LLM is available for extraction.
func hasLLMConfig() bool {
	return os.Getenv("OPENROUTER_API_KEY") != ""
}

// ============================================================================
// CACHED LLM EXTRACTION
// ============================================================================

// llmExtractionCache caches LLM results by message hash to avoid re-processing.
var llmExtractionCache = make(map[string]*ExtractionResult)

// ExtractContactInfoLLMCached wraps ExtractContactInfoLLM with caching.
func ExtractContactInfoLLMCached(ctx context.Context, messageContent string, conversationHistory []store.AgentMessage, currentDraft *LeadDraft) *ExtractionResult {
	// Simple hash-based cache key
	cacheKey := fmt.Sprintf("%d", hashString(messageContent))
	if cached, ok := llmExtractionCache[cacheKey]; ok {
		return cached
	}

	result := ExtractContactInfoLLM(ctx, messageContent, conversationHistory, currentDraft)
	if result != nil {
		llmExtractionCache[cacheKey] = result
	}
	return result
}

func hashString(s string) uint32 {
	var h uint32 = 5381
	for i := 0; i < len(s); i++ {
		h = ((h << 5) + h) + uint32(s[i])
	}
	return h
}

// ============================================================================
// FULL EXTRACTION PIPELINE
// ============================================================================

// ExtractContactInfoFull runs the full 3-layer extraction pipeline.
// Returns the merged lead draft or nil if no useful info found.
func ExtractContactInfoFull(ctx context.Context, messageContent string, conversationHistory []store.AgentMessage, tenantPhone string, existingDraft *LeadDraft) *LeadDraft {
	// Process the single messageContent first, then iterate over conversation history
	messages := conversationHistory
	if messageContent != "" {
		messages = append(messages, store.AgentMessage{Role: "user", Content: messageContent})
	}

	draft := existingDraft
	if draft == nil {
		draft = NewLeadDraft()
	}

	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}

		// Layer 1: Regex extraction
		regexResult := ExtractContactInfo(msg.Content, tenantPhone)

		// Layer 2: Structural analysis
		intent := ClassifyMessage(msg.Content)

		// If message is a decline, mark it
		if intent == IntentDeclineContact {
			draft.Declined = true
			draft.UpdatedAt = time.Now()
			return draft
		}

		// If message is a correction, mark the regex result as corrected
		if intent == IntentCorrectPrevious && regexResult != nil {
			regexResult.Corrected = true
		}

		// Merge Layer 1 result into draft
		MergeExtractions(draft, regexResult)
	}

	// Layer 3: LLM extraction (only if regex was uncertain and last message is substantial)
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role == "user" {
			regexConfident := draft.Name != "" || draft.Email != "" || draft.Phone != ""
			shouldUseLLM := !regexConfident && len(lastMsg.Content) > 30 && !isSpamInput(lastMsg.Content)
			if shouldUseLLM && hasLLMConfig() {
				llmResult := ExtractContactInfoLLMCached(ctx, lastMsg.Content, messages, draft)
				if llmResult != nil {
					MergeExtractions(draft, llmResult)
				}
			}
		}
	}

	return draft
}
