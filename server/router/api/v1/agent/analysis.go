package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/revrost/go-openrouter"

	"github.com/usememos/memos/store"
)

// AnalysisResult represents the result of analyzing a transcript.
type AnalysisResult struct {
	ID               string                    `json:"id"`
	TenantID         int32                     `json:"tenant_id"`
	ConversationID   string                    `json:"conversation_id"`
	ConversationType string                    `json:"conversation_type"`
	Score            int                       `json:"score"`
	Grade            string                    `json:"grade"`
	Breakdown        store.AnalysisBreakdown   `json:"breakdown"`
	Issues           []store.AnalysisIssue     `json:"issues"`
	Suggestions      []string                  `json:"suggestions,omitempty"`
	BenchmarkVersion string                    `json:"benchmark_version"`
	CreatedAt        time.Time                 `json:"created_at"`
}

// AnalyzeTranscript evaluates a conversation against benchmarks.
func (s *Service) AnalyzeTranscript(ctx context.Context, tenantID int32, conversationID string, userID int32, includeSuggestions bool) (*AnalysisResult, error) {
	// 1. Load the conversation
	var conversationType string
	var transcript string
	var messageCount int

	if strings.HasPrefix(conversationID, "sim-") {
		// It's a simulation
		conversationType = "simulation"
		sim, err := s.GetSimulationTranscript(ctx, conversationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get simulation: %w", err)
		}
		if sim == nil || sim.TenantID != tenantID {
			return nil, fmt.Errorf("simulation not found")
		}
		transcript = formatSimulationTranscript(sim)
		messageCount = len(sim.Messages)
	} else {
		// It's a chat session
		conversationType = "chat"
		session, err := s.store.GetAgentSession(ctx, &store.FindAgentSession{ID: &conversationID, TenantID: &tenantID})
		if err != nil {
			return nil, fmt.Errorf("failed to get session: %w", err)
		}
		if session == nil {
			return nil, fmt.Errorf("session not found")
		}
		transcript = formatSessionTranscript(session)
		messageCount = len(session.Messages)
	}

	if messageCount < 2 {
		return nil, fmt.Errorf("conversation too short for analysis (minimum 2 messages)")
	}

	// 2. Load tenant's benchmark documents
	kbFile, _ := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
		TenantID:     &tenantID,
		AudienceType: stringPtr("external"),
		FileType:     stringPtr("kb"),
	})
	policyFile, _ := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
		TenantID:     &tenantID,
		AudienceType: stringPtr("external"),
		FileType:     stringPtr("policy"),
	})
	script, _ := s.store.GetAgentTenantScript(ctx, &store.FindAgentTenantScript{TenantID: &tenantID})

	kbContent := ""
	if kbFile != nil {
		kbContent = kbFile.Content
	}
	policyContent := ""
	if policyFile != nil {
		policyContent = policyFile.Content
	}
	scriptContent := ""
	if script != nil {
		scriptContent = script.Content
	}

	// 3. Build analysis prompt
	analysisPrompt := buildAnalysisPrompt(kbContent, policyContent, scriptContent, transcript, includeSuggestions)

	// 4. Call LLM
	model, apiKey := s.getLLMConfig(ctx, tenantID)
	client := openrouter.NewClient(apiKey)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model: model,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage("You are an expert conversation analyst. Respond ONLY with valid JSON, no other text."),
			openrouter.UserMessage(analysisPrompt),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	// 5. Parse response
	responseText := resp.Choices[0].Message.Content.Text
	responseText = cleanJSONResponse(responseText)

	var llmResult struct {
		Score      int                       `json:"score"`
		Grade      string                    `json:"grade"`
		Breakdown  store.AnalysisBreakdown   `json:"breakdown"`
		Issues     []store.AnalysisIssue     `json:"issues"`
		Suggestions []string                 `json:"suggestions"`
	}

	if err := json.Unmarshal([]byte(responseText), &llmResult); err != nil {
		slog.Error("failed to parse LLM response", "error", err, "response", responseText)
		return nil, fmt.Errorf("failed to parse analysis response")
	}

	// 6. Create result
	result := &AnalysisResult{
		ID:               "ana-" + uuid.New().String(),
		TenantID:         tenantID,
		ConversationID:   conversationID,
		ConversationType: conversationType,
		Score:            llmResult.Score,
		Grade:            llmResult.Grade,
		Breakdown:        llmResult.Breakdown,
		Issues:           llmResult.Issues,
		BenchmarkVersion: time.Now().Format("2006-01-02"),
		CreatedAt:        time.Now(),
	}

	if includeSuggestions {
		result.Suggestions = llmResult.Suggestions
	}

	// 7. Store result
	storeResult := &store.AgentAnalysisResult{
		ID:               result.ID,
		TenantID:         tenantID,
		ConversationID:   conversationID,
		ConversationType: conversationType,
		UserID:           userID,
		Score:            result.Score,
		Grade:            result.Grade,
		Breakdown:        result.Breakdown,
		Issues:           result.Issues,
		Suggestions:      result.Suggestions,
		BenchmarkVersion: result.BenchmarkVersion,
	}

	if _, err := s.store.CreateAgentAnalysisResult(ctx, storeResult); err != nil {
		slog.Error("failed to store analysis result", "error", err)
		// Continue anyway - analysis succeeded even if storage failed
	}

	return result, nil
}

func formatSimulationTranscript(sim *store.AgentSimulationTranscript) string {
	var sb strings.Builder
	for i, msg := range sim.Messages {
		role := "Human"
		if msg.Role == "agent" {
			role = "Agent"
		}
		sb.WriteString(fmt.Sprintf("Turn %d - %s: %s\n\n", i+1, role, msg.Content))
	}
	return sb.String()
}

func formatSessionTranscript(session *store.AgentSession) string {
	var sb strings.Builder
	for i, msg := range session.Messages {
		role := "Human"
		if msg.Role == "assistant" {
			role = "Agent"
		}
		sb.WriteString(fmt.Sprintf("Turn %d - %s: %s\n\n", i+1, role, msg.Content))
	}
	return sb.String()
}

func buildAnalysisPrompt(kbContent, policyContent, scriptContent, transcript string, includeSuggestions bool) string {
	var sb strings.Builder

	sb.WriteString("You are an expert conversation analyst evaluating an AI customer service agent.\n\n")

	sb.WriteString("=== BENCHMARK DOCUMENTS ===\n\n")

	if kbContent != "" {
		// Truncate if too long
		if len(kbContent) > 3000 {
			kbContent = kbContent[:3000] + "\n[truncated]"
		}
		sb.WriteString("KB.MD (Services & Knowledge):\n")
		sb.WriteString(kbContent)
		sb.WriteString("\n\n")
	}

	if policyContent != "" {
		if len(policyContent) > 2000 {
			policyContent = policyContent[:2000] + "\n[truncated]"
		}
		sb.WriteString("POLICY.MD (Rules & Identity):\n")
		sb.WriteString(policyContent)
		sb.WriteString("\n\n")
	}

	if scriptContent != "" {
		if len(scriptContent) > 1500 {
			scriptContent = scriptContent[:1500] + "\n[truncated]"
		}
		sb.WriteString("SCRIPT.MD (Conversation Flow Guide):\n")
		sb.WriteString(scriptContent)
		sb.WriteString("\n\n")
	}

	sb.WriteString("=== TRANSCRIPT TO ANALYZE ===\n")
	sb.WriteString(transcript)
	sb.WriteString("\n\n")

	sb.WriteString("=== EVALUATION RUBRIC ===\n")
	sb.WriteString("Score each category (be strict but fair):\n\n")
	sb.WriteString("1. Intent Recognition (0-15): Did the agent correctly identify the customer's primary intent early?\n")
	sb.WriteString("2. Service Alignment (0-15): Did the agent offer services from KB.MD and avoid excluded services?\n")
	sb.WriteString("3. Policy Compliance (0-20): Did the agent follow all applicable rules from POLICY.MD?\n")
	sb.WriteString("4. Conversation Flow (0-20): Did the agent follow the structure from SCRIPT.MD (opening, info gathering, confirmation, closing)?\n")
	sb.WriteString("5. Information Gathering (0-15): Did the agent collect: name, phone, address, damage type, timing?\n")
	sb.WriteString("6. Tone & Resolution (0-15): Was tone appropriate per POLICY.MD? Did conversation reach proper conclusion?\n\n")

	sb.WriteString("Respond ONLY with this JSON structure:\n")
	sb.WriteString(`{
  "score": <total 0-100>,
  "grade": "<A/B/C/D/F>",
  "breakdown": {
    "intent_recognition": { "score": <0-15>, "max": 15, "notes": "<brief explanation>" },
    "service_alignment": { "score": <0-15>, "max": 15, "notes": "<brief explanation>" },
    "policy_compliance": { "score": <0-20>, "max": 20, "notes": "<brief explanation>" },
    "conversation_flow": { "score": <0-20>, "max": 20, "notes": "<brief explanation>" },
    "information_gathering": { "score": <0-15>, "max": 15, "notes": "<brief explanation>" },
    "tone_resolution": { "score": <0-15>, "max": 15, "notes": "<brief explanation>" }
  },
  "issues": [
    { "severity": "critical|warning|info", "turn": <number>, "message": "<issue description>" }
  ]`)

	if includeSuggestions {
		sb.WriteString(`,
  "suggestions": ["<improvement suggestion>", ...]`)
	}

	sb.WriteString("\n}")

	return sb.String()
}

func cleanJSONResponse(response string) string {
	// Remove markdown code fences if present
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	}
	if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}
	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}
	return strings.TrimSpace(response)
}
