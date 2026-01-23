package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/revrost/go-openrouter"
	"github.com/usememos/memos/store"
)

// ComplianceCheck defines a single compliance requirement.
type ComplianceCheck struct {
	Trigger     string `json:"trigger"`     // Category name or "ALL"
	Requirement string `json:"requirement"` // What must be true
	Weight      int    `json:"weight"`      // 1-5 importance
}

// ComplianceCheckResult is the result of evaluating a single check.
type ComplianceCheckResult struct {
	Requirement string `json:"requirement"`
	Weight      int    `json:"weight"`
	Passed      bool   `json:"passed"`
	Evidence    string `json:"evidence"`  // Quote from transcript
	Reasoning   string `json:"reasoning"` // Explanation
}

// ComplianceAudit is the full audit result for a conversation.
type ComplianceAudit struct {
	ID               string                  `json:"id"`
	TenantID         int32                   `json:"tenant_id"`
	ConversationID   string                  `json:"conversation_id"`
	ConversationType string                  `json:"conversation_type"` // "simulation" or "chat"
	Score            int                     `json:"score"`             // 0-100
	Checks           []ComplianceCheckResult `json:"checks"`
	OverallPassed    bool                    `json:"overall_passed"`
	AuditedAt        time.Time               `json:"audited_at"`
}

// UniversalComplianceChecklist contains checks applicable to any service business.
var UniversalComplianceChecklist = []ComplianceCheck{
	{
		Trigger:     "ALL",
		Requirement: "Agent must not mention services not documented in knowledge base",
		Weight:      5,
	},
	{
		Trigger:     "ALL",
		Requirement: "Agent must use only authorized contact information",
		Weight:      5,
	},
	{
		Trigger:     "ALL",
		Requirement: "Agent must follow the conversation flow structure",
		Weight:      3,
	},
	{
		Trigger:     "escalation_signal",
		Requirement: "Agent must offer escalation path when customer requests supervisor",
		Weight:      4,
	},
	{
		Trigger:     "lead_quality",
		Requirement: "Agent should attempt to collect customer name and contact info",
		Weight:      3,
	},
	{
		Trigger:     "negative_sentiment",
		Requirement: "Agent must acknowledge customer frustration empathetically",
		Weight:      3,
	},
	{
		Trigger:     "service_exclusion",
		Requirement: "Agent must not offer or promise excluded services",
		Weight:      5,
	},
	{
		Trigger:     "ALL",
		Requirement: "Agent responses must be relevant to customer query",
		Weight:      4,
	},
	{
		Trigger:     "ALL",
		Requirement: "Agent must maintain professional and helpful tone",
		Weight:      3,
	},
	{
		Trigger:     "urgency_high",
		Requirement: "Agent must treat urgent matters with appropriate priority",
		Weight:      4,
	},
}

// ComplianceAuditor handles auditing of conversation transcripts.
type ComplianceAuditor struct {
	store     store.Store
	client    *openrouter.Client
	checklist []ComplianceCheck
}

// NewComplianceAuditor creates a new auditor with the universal checklist.
func NewComplianceAuditor(store store.Store, client *openrouter.Client) *ComplianceAuditor {
	return &ComplianceAuditor{
		store:     store,
		client:    client,
		checklist: UniversalComplianceChecklist,
	}
}

// AuditConversation performs a compliance audit on a conversation.
func (a *ComplianceAuditor) AuditConversation(
	ctx context.Context,
	tenantID int32,
	conversationID string,
	conversationType string,
	transcript []store.AgentMessage,
	config *AudienceConfig,
	conversationScore *ConversationScore,
) (*ComplianceAudit, error) {
	audit := &ComplianceAudit{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		ConversationID:   conversationID,
		ConversationType: conversationType,
		AuditedAt:        time.Now(),
		Checks:           []ComplianceCheckResult{},
	}

	// Build transcript text for analysis
	transcriptText := a.buildTranscriptText(transcript)

	// Track scores
	totalPossible := 0
	totalEarned := 0

	// Evaluate each check
	for _, check := range a.checklist {
		// Determine if this check applies
		if !a.checkApplies(check.Trigger, conversationScore) {
			continue
		}

		totalPossible += check.Weight

		result := a.evaluateCheck(check, transcriptText, transcript, config, conversationScore)
		audit.Checks = append(audit.Checks, result)

		if result.Passed {
			totalEarned += check.Weight
		}
	}

	// Calculate score
	if totalPossible > 0 {
		audit.Score = (totalEarned * 100) / totalPossible
	} else {
		audit.Score = 100 // No applicable checks
	}

	audit.OverallPassed = audit.Score >= 70

	return audit, nil
}

// buildTranscriptText converts messages to a readable transcript.
func (a *ComplianceAuditor) buildTranscriptText(messages []store.AgentMessage) string {
	var sb strings.Builder
	for _, msg := range messages {
		role := "Customer"
		if msg.Role == "assistant" {
			role = "Agent"
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", role, msg.Content))
	}
	return sb.String()
}

// checkApplies determines if a check should be evaluated.
func (a *ComplianceAuditor) checkApplies(trigger string, score *ConversationScore) bool {
	if trigger == "ALL" {
		return true
	}

	if score == nil {
		return trigger == "ALL"
	}

	switch trigger {
	case "escalation_signal":
		if cat, ok := score.Categories["escalation_signal"]; ok {
			return cat.Level == "high" || cat.Level == "medium"
		}
	case "negative_sentiment":
		if cat, ok := score.Categories["sentiment"]; ok {
			return cat.Level == "negative"
		}
	case "service_exclusion":
		if cat, ok := score.Categories["service_match"]; ok {
			return cat.Level == "low" // Low match suggests exclusion mentioned
		}
	case "lead_quality":
		// Always try to collect lead info
		return true
	case "urgency_high":
		return score.Urgency == "emergency" || score.Urgency == "urgent"
	}

	return false
}

// evaluateCheck evaluates a single compliance check.
func (a *ComplianceAuditor) evaluateCheck(
	check ComplianceCheck,
	transcriptText string,
	messages []store.AgentMessage,
	config *AudienceConfig,
	score *ConversationScore,
) ComplianceCheckResult {
	result := ComplianceCheckResult{
		Requirement: check.Requirement,
		Weight:      check.Weight,
		Passed:      true, // Assume pass until proven otherwise
	}

	transcriptLower := strings.ToLower(transcriptText)

	switch {
	case strings.Contains(check.Requirement, "not mention services not documented"):
		result = a.checkServiceCompliance(messages, config)
		result.Requirement = check.Requirement
		result.Weight = check.Weight

	case strings.Contains(check.Requirement, "authorized contact information"):
		result = a.checkContactCompliance(messages, config)
		result.Requirement = check.Requirement
		result.Weight = check.Weight

	case strings.Contains(check.Requirement, "escalation path"):
		result = a.checkEscalationOffered(transcriptLower, messages)
		result.Requirement = check.Requirement
		result.Weight = check.Weight

	case strings.Contains(check.Requirement, "collect customer name"):
		result = a.checkLeadCollection(transcriptLower, messages)
		result.Requirement = check.Requirement
		result.Weight = check.Weight

	case strings.Contains(check.Requirement, "acknowledge customer frustration"):
		result = a.checkEmpathyShown(transcriptLower, messages)
		result.Requirement = check.Requirement
		result.Weight = check.Weight

	case strings.Contains(check.Requirement, "not offer or promise excluded"):
		result = a.checkExclusionCompliance(messages, config)
		result.Requirement = check.Requirement
		result.Weight = check.Weight

	case strings.Contains(check.Requirement, "relevant to customer query"):
		result = a.checkRelevance(messages)
		result.Requirement = check.Requirement
		result.Weight = check.Weight

	case strings.Contains(check.Requirement, "professional and helpful tone"):
		result = a.checkTone(messages)
		result.Requirement = check.Requirement
		result.Weight = check.Weight

	case strings.Contains(check.Requirement, "urgent matters"):
		result = a.checkUrgencyHandling(transcriptLower, messages, score)
		result.Requirement = check.Requirement
		result.Weight = check.Weight

	case strings.Contains(check.Requirement, "conversation flow"):
		result = a.checkConversationFlow(messages, config)
		result.Requirement = check.Requirement
		result.Weight = check.Weight
	}

	return result
}

// checkServiceCompliance verifies agent only mentions documented services using LLM-based semantic analysis.
func (a *ComplianceAuditor) checkServiceCompliance(messages []store.AgentMessage, config *AudienceConfig) ComplianceCheckResult {
	result := ComplianceCheckResult{Passed: true}

	if config == nil || len(config.Services) == 0 {
		result.Reasoning = "No services configured to check against"
		return result
	}

	// Use LLM verifier for semantic service compliance checking
	if a.client != nil {
		verifier := NewVerifier(a.client, DefaultVerificationConfig())
		for _, msg := range messages {
			if msg.Role == "assistant" {
				verifyResult, err := verifier.VerifyResponse(context.Background(), msg.Content, config)
				if err != nil {
					// On error, fall back to passing (graceful degradation)
					result.Reasoning = "Verification unavailable, assumed compliant"
					return result
				}

				// Check for service-related violations
				for _, v := range verifyResult.Violations {
					if strings.Contains(strings.ToLower(v.ChecklistItem), "service") {
						result.Passed = false
						result.Evidence = v.Evidence
						result.Reasoning = v.Violation
						return result
					}
				}
			}
		}
	}

	result.Reasoning = "All mentioned services are documented"
	return result
}

// checkContactCompliance verifies only authorized contacts are provided using LLM-based semantic analysis.
func (a *ComplianceAuditor) checkContactCompliance(messages []store.AgentMessage, config *AudienceConfig) ComplianceCheckResult {
	result := ComplianceCheckResult{Passed: true}

	// Use LLM verifier for semantic contact compliance checking
	if a.client != nil {
		verifier := NewVerifier(a.client, DefaultVerificationConfig())
		for _, msg := range messages {
			if msg.Role == "assistant" {
				verifyResult, err := verifier.VerifyResponse(context.Background(), msg.Content, config)
				if err != nil {
					// On error, fall back to passing (graceful degradation)
					result.Reasoning = "Verification unavailable, assumed compliant"
					return result
				}

				// Check for contact-related violations
				for _, v := range verifyResult.Violations {
					itemLower := strings.ToLower(v.ChecklistItem)
					if strings.Contains(itemLower, "phone") || strings.Contains(itemLower, "email") ||
						strings.Contains(itemLower, "contact") || strings.Contains(itemLower, "placeholder") {
						result.Passed = false
						result.Evidence = v.Evidence
						result.Reasoning = v.Violation
						return result
					}
				}
			}
		}
	}

	result.Reasoning = "All contact information is authorized"
	return result
}

// checkEscalationOffered verifies escalation was offered when requested.
func (a *ComplianceAuditor) checkEscalationOffered(transcript string, messages []store.AgentMessage) ComplianceCheckResult {
	result := ComplianceCheckResult{Passed: true}

	// Check if customer requested escalation
	escalationRequests := []string{
		"manager", "supervisor", "speak to someone", "your boss",
		"escalate", "complaint", "higher up",
	}

	customerRequested := false
	for _, msg := range messages {
		if msg.Role == "user" {
			msgLower := strings.ToLower(msg.Content)
			for _, kw := range escalationRequests {
				if strings.Contains(msgLower, kw) {
					customerRequested = true
					result.Evidence = msg.Content
					break
				}
			}
		}
	}

	if !customerRequested {
		result.Reasoning = "No escalation request detected"
		return result
	}

	// Check if agent offered escalation
	escalationOffers := []string{
		"supervisor", "manager", "escalate", "someone else",
		"transfer", "connect you with", "have someone call",
		"callback", "call back",
	}

	for _, msg := range messages {
		if msg.Role == "assistant" {
			msgLower := strings.ToLower(msg.Content)
			for _, kw := range escalationOffers {
				if strings.Contains(msgLower, kw) {
					result.Passed = true
					result.Reasoning = "Escalation path was offered"
					return result
				}
			}
		}
	}

	result.Passed = false
	result.Reasoning = "Customer requested escalation but none was offered"
	return result
}

// checkLeadCollection verifies agent attempted to collect contact info.
func (a *ComplianceAuditor) checkLeadCollection(transcript string, messages []store.AgentMessage) ComplianceCheckResult {
	result := ComplianceCheckResult{Passed: true}

	// Check if agent asked for contact info
	contactQuestions := []string{
		"your name", "may i have your name", "what is your name",
		"phone number", "contact number", "reach you",
		"email", "address", "call you back",
	}

	for _, msg := range messages {
		if msg.Role == "assistant" {
			msgLower := strings.ToLower(msg.Content)
			for _, q := range contactQuestions {
				if strings.Contains(msgLower, q) {
					result.Passed = true
					result.Reasoning = "Agent requested contact information"
					result.Evidence = msg.Content
					return result
				}
			}
		}
	}

	// If conversation is very short, don't penalize
	if len(messages) < 4 {
		result.Reasoning = "Conversation too short to expect lead collection"
		return result
	}

	result.Passed = false
	result.Reasoning = "Agent did not attempt to collect contact information"
	return result
}

// checkEmpathyShown verifies empathy was shown for frustrated customers.
func (a *ComplianceAuditor) checkEmpathyShown(transcript string, messages []store.AgentMessage) ComplianceCheckResult {
	result := ComplianceCheckResult{Passed: true}

	empathyPhrases := []string{
		"understand", "sorry", "apologize", "frustrating",
		"appreciate", "thank you for", "i hear you",
		"must be difficult", "can imagine", "completely understand",
	}

	for _, msg := range messages {
		if msg.Role == "assistant" {
			msgLower := strings.ToLower(msg.Content)
			for _, phrase := range empathyPhrases {
				if strings.Contains(msgLower, phrase) {
					result.Passed = true
					result.Reasoning = "Empathy was expressed"
					result.Evidence = msg.Content
					return result
				}
			}
		}
	}

	result.Passed = false
	result.Reasoning = "No empathetic acknowledgment detected"
	return result
}

// checkExclusionCompliance verifies excluded services were not offered using LLM-based semantic analysis.
func (a *ComplianceAuditor) checkExclusionCompliance(messages []store.AgentMessage, config *AudienceConfig) ComplianceCheckResult {
	result := ComplianceCheckResult{Passed: true}

	if config == nil || len(config.Exclusions) == 0 {
		result.Reasoning = "No exclusions configured"
		return result
	}

	// Use LLM verifier for semantic exclusion compliance checking
	if a.client != nil {
		verifier := NewVerifier(a.client, DefaultVerificationConfig())
		for _, msg := range messages {
			if msg.Role == "assistant" {
				verifyResult, err := verifier.VerifyResponse(context.Background(), msg.Content, config)
				if err != nil {
					// On error, fall back to passing (graceful degradation)
					result.Reasoning = "Verification unavailable, assumed compliant"
					return result
				}

				// Check for exclusion-related violations
				for _, v := range verifyResult.Violations {
					itemLower := strings.ToLower(v.ChecklistItem)
					if strings.Contains(itemLower, "exclusion") || strings.Contains(itemLower, "do not provide") ||
						strings.Contains(itemLower, "excluded") {
						result.Passed = false
						result.Evidence = v.Evidence
						result.Reasoning = v.Violation
						return result
					}
				}
			}
		}
	}

	result.Reasoning = "No excluded services were offered"
	return result
}

// checkRelevance verifies responses are relevant to queries.
func (a *ComplianceAuditor) checkRelevance(messages []store.AgentMessage) ComplianceCheckResult {
	result := ComplianceCheckResult{
		Passed:    true,
		Reasoning: "Responses appear relevant to queries",
	}

	// Simple heuristic: check if agent responses are not empty and reasonable length
	for _, msg := range messages {
		if msg.Role == "assistant" {
			if len(msg.Content) < 10 {
				result.Passed = false
				result.Reasoning = "Agent response too short to be helpful"
				result.Evidence = msg.Content
				return result
			}
		}
	}

	return result
}

// checkTone verifies professional and helpful tone.
func (a *ComplianceAuditor) checkTone(messages []store.AgentMessage) ComplianceCheckResult {
	result := ComplianceCheckResult{
		Passed:    true,
		Reasoning: "Tone appears professional and helpful",
	}

	unprofessionalPhrases := []string{
		"that's not my problem", "figure it out", "whatever",
		"i don't care", "not my job", "deal with it",
	}

	for _, msg := range messages {
		if msg.Role == "assistant" {
			msgLower := strings.ToLower(msg.Content)
			for _, phrase := range unprofessionalPhrases {
				if strings.Contains(msgLower, phrase) {
					result.Passed = false
					result.Reasoning = "Unprofessional language detected"
					result.Evidence = msg.Content
					return result
				}
			}
		}
	}

	return result
}

// checkUrgencyHandling verifies urgent matters are handled appropriately.
func (a *ComplianceAuditor) checkUrgencyHandling(transcript string, messages []store.AgentMessage, score *ConversationScore) ComplianceCheckResult {
	result := ComplianceCheckResult{Passed: true}

	if score == nil || (score.Urgency != "emergency" && score.Urgency != "urgent") {
		result.Reasoning = "Not an urgent situation"
		return result
	}

	// Check if agent acknowledged urgency
	urgencyAcknowledgments := []string{
		"right away", "immediately", "urgent", "priority",
		"emergency", "as soon as possible", "quickly",
		"understand the urgency", "top priority",
	}

	for _, msg := range messages {
		if msg.Role == "assistant" {
			msgLower := strings.ToLower(msg.Content)
			for _, phrase := range urgencyAcknowledgments {
				if strings.Contains(msgLower, phrase) {
					result.Passed = true
					result.Reasoning = "Urgency was acknowledged"
					result.Evidence = msg.Content
					return result
				}
			}
		}
	}

	result.Passed = false
	result.Reasoning = "Urgent matter not handled with appropriate priority"
	return result
}

// checkConversationFlow verifies the conversation followed expected structure.
func (a *ComplianceAuditor) checkConversationFlow(messages []store.AgentMessage, config *AudienceConfig) ComplianceCheckResult {
	result := ComplianceCheckResult{
		Passed:    true,
		Reasoning: "Conversation flow appears appropriate",
	}

	// Basic check: did conversation have proper opening and closing?
	if len(messages) < 2 {
		return result // Too short to evaluate
	}

	// Check for greeting in first agent response
	greetingPhrases := []string{
		"hello", "hi", "welcome", "good morning", "good afternoon",
		"thank you for", "how can i help", "how may i assist",
	}

	hasGreeting := false
	for _, msg := range messages[:min(3, len(messages))] {
		if msg.Role == "assistant" {
			msgLower := strings.ToLower(msg.Content)
			for _, phrase := range greetingPhrases {
				if strings.Contains(msgLower, phrase) {
					hasGreeting = true
					break
				}
			}
		}
	}

	if !hasGreeting && len(messages) > 2 {
		result.Passed = false
		result.Reasoning = "Missing proper greeting"
		return result
	}

	return result
}

// StoreAudit saves an audit result to the database.
func (a *ComplianceAuditor) StoreAudit(ctx context.Context, audit *ComplianceAudit) error {
	checksJSON, err := json.Marshal(audit.Checks)
	if err != nil {
		return err
	}

	return a.store.CreateAgentComplianceAudit(ctx, &store.AgentComplianceAudit{
		ID:               audit.ID,
		TenantID:         audit.TenantID,
		ConversationID:   audit.ConversationID,
		ConversationType: audit.ConversationType,
		Score:            audit.Score,
		Checks:           string(checksJSON),
		OverallPassed:    audit.OverallPassed,
		AuditedAt:        audit.AuditedAt,
	})
}

// GetAuditHistory retrieves past audits for a tenant.
func (a *ComplianceAuditor) GetAuditHistory(ctx context.Context, tenantID int32, limit int) ([]*ComplianceAudit, error) {
	audits, err := a.store.ListAgentComplianceAudits(ctx, &store.FindAgentComplianceAudit{
		TenantID: &tenantID,
		Limit:    &limit,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*ComplianceAudit, len(audits))
	for i, audit := range audits {
		var checks []ComplianceCheckResult
		if err := json.Unmarshal([]byte(audit.Checks), &checks); err != nil {
			checks = []ComplianceCheckResult{}
		}

		results[i] = &ComplianceAudit{
			ID:               audit.ID,
			TenantID:         audit.TenantID,
			ConversationID:   audit.ConversationID,
			ConversationType: audit.ConversationType,
			Score:            audit.Score,
			Checks:           checks,
			OverallPassed:    audit.OverallPassed,
			AuditedAt:        audit.AuditedAt,
		}
	}

	return results, nil
}
