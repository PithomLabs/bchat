package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/usememos/memos/store"
)

const (
	// MinOccurrencesForSuggestion is the minimum times an issue must appear to generate a suggestion
	MinOccurrencesForSuggestion = 3
	// MaxLearnedBehaviors is the maximum number of learned behaviors per tenant
	MaxLearnedBehaviors = 10
)

// LearningService handles agent self-improvement based on analysis results
type LearningService struct {
	store *store.Store
}

// NewLearningService creates a new learning service
func NewLearningService(s *store.Store) *LearningService {
	return &LearningService{store: s}
}

// GetLearningMemory retrieves the learning memory for a tenant
func (s *LearningService) GetLearningMemory(ctx context.Context, tenantID int32) (*store.AgentLearningMemory, error) {
	return s.store.GetOrCreateAgentLearningMemory(ctx, tenantID)
}

// AggregateFromAnalysis updates learning memory based on recent analysis results
func (s *LearningService) AggregateFromAnalysis(ctx context.Context, tenantID int32) (*store.AgentLearningMemory, error) {
	// Get existing learning memory
	memory, err := s.store.GetOrCreateAgentLearningMemory(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get learning memory: %w", err)
	}

	// Get recent analysis results (last 50)
	results, _, err := s.store.ListAgentAnalysisResults(ctx, &store.FindAgentAnalysisResult{
		TenantID: &tenantID,
		Limit:    50,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get analysis results: %w", err)
	}

	if len(results) == 0 {
		return memory, nil
	}

	// Aggregate issues by category
	issuesByCategory := make(map[string][]string)
	categoryScores := make(map[string][]int)
	categoryMaxScores := map[string]int{
		"intent_recognition":    15,
		"service_alignment":     15,
		"policy_compliance":     20,
		"conversation_flow":     20,
		"information_gathering": 15,
		"tone_resolution":       15,
	}

	for _, result := range results {
		// Aggregate scores
		categoryScores["intent_recognition"] = append(categoryScores["intent_recognition"], result.Breakdown.IntentRecognition.Score)
		categoryScores["service_alignment"] = append(categoryScores["service_alignment"], result.Breakdown.ServiceAlignment.Score)
		categoryScores["policy_compliance"] = append(categoryScores["policy_compliance"], result.Breakdown.PolicyCompliance.Score)
		categoryScores["conversation_flow"] = append(categoryScores["conversation_flow"], result.Breakdown.ConversationFlow.Score)
		categoryScores["information_gathering"] = append(categoryScores["information_gathering"], result.Breakdown.InformationGathering.Score)
		categoryScores["tone_resolution"] = append(categoryScores["tone_resolution"], result.Breakdown.ToneResolution.Score)

		// Collect issues
		for _, issue := range result.Issues {
			if issue.Severity == "critical" || issue.Severity == "warning" {
				// Map issue to category based on content
				category := categorizeIssue(issue.Message)
				issuesByCategory[category] = append(issuesByCategory[category], issue.Message)
			}
		}
	}

	// Calculate improvement areas
	memory.ImprovementAreas = []store.ImprovementArea{}
	for category, scores := range categoryScores {
		if len(scores) == 0 {
			continue
		}
		avg := 0.0
		for _, s := range scores {
			avg += float64(s)
		}
		avg /= float64(len(scores))

		maxScore := categoryMaxScores[category]
		// Only add if average is below 80% of max
		if avg < float64(maxScore)*0.8 {
			memory.ImprovementAreas = append(memory.ImprovementAreas, store.ImprovementArea{
				Category:     category,
				AverageScore: avg,
				MaxScore:     maxScore,
				TrendPercent: 0, // TODO: calculate trend from historical data
			})
		}
	}

	// Generate pending suggestions from common issues
	now := time.Now().Format("2006-01-02")
	existingSuggestions := make(map[string]bool)
	for _, s := range memory.PendingSuggestions {
		existingSuggestions[s.Category+":"+s.Behavior] = true
	}

	for category, issues := range issuesByCategory {
		// Count unique issue patterns
		issueCounts := make(map[string]int)
		for _, issue := range issues {
			issueCounts[issue]++
		}

		// Generate suggestions for issues that occur >= MinOccurrencesForSuggestion times
		for issueText, count := range issueCounts {
			if count >= MinOccurrencesForSuggestion {
				suggestion := generateSuggestion(category, issueText)
				key := category + ":" + suggestion.Behavior
				if !existingSuggestions[key] {
					suggestion.ID = uuid.New().String()[:8]
					suggestion.Occurrences = count
					suggestion.CreatedAt = now
					memory.PendingSuggestions = append(memory.PendingSuggestions, suggestion)
					existingSuggestions[key] = true
				}
			}
		}
	}

	// Update common issues
	memory.CommonIssues = []store.CommonIssue{}
	for category, issues := range issuesByCategory {
		if len(issues) > 0 {
			// Find most common issue in this category
			issueCounts := make(map[string]int)
			for _, issue := range issues {
				issueCounts[issue]++
			}
			var mostCommon string
			maxCount := 0
			for issue, count := range issueCounts {
				if count > maxCount {
					mostCommon = issue
					maxCount = count
				}
			}
			memory.CommonIssues = append(memory.CommonIssues, store.CommonIssue{
				Category:    category,
				Description: mostCommon,
				Occurrences: maxCount,
				LastSeen:    now,
			})
		}
	}

	memory.AnalysisCount = len(results)

	// Save updated memory
	return s.store.UpdateAgentLearningMemory(ctx, memory)
}

// ApproveSuggestion moves a pending suggestion to learned behaviors
func (s *LearningService) ApproveSuggestion(ctx context.Context, tenantID int32, suggestionID string, editedBehavior *string) (*store.AgentLearningMemory, error) {
	memory, err := s.store.GetOrCreateAgentLearningMemory(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Check max behaviors limit
	activeCount := 0
	for _, b := range memory.LearnedBehaviors {
		if b.IsActive {
			activeCount++
		}
	}
	if activeCount >= MaxLearnedBehaviors {
		return nil, fmt.Errorf("maximum learned behaviors (%d) reached, remove some before adding new ones", MaxLearnedBehaviors)
	}

	// Find and move suggestion
	var suggestion *store.PendingSuggestion
	newPending := []store.PendingSuggestion{}
	for _, s := range memory.PendingSuggestions {
		if s.ID == suggestionID {
			suggestion = &s
		} else {
			newPending = append(newPending, s)
		}
	}

	if suggestion == nil {
		return nil, fmt.Errorf("suggestion not found: %s", suggestionID)
	}

	behavior := suggestion.Behavior
	if editedBehavior != nil {
		behavior = *editedBehavior
	}

	// Create learned behavior
	learned := store.LearnedBehavior{
		ID:       uuid.New().String()[:8],
		Trigger:  suggestion.Trigger,
		Behavior: behavior,
		Source:   fmt.Sprintf("Approved from %d occurrences", suggestion.Occurrences),
		AddedAt:  time.Now().Format("2006-01-02"),
		IsActive: true,
	}

	memory.PendingSuggestions = newPending
	memory.LearnedBehaviors = append(memory.LearnedBehaviors, learned)

	return s.store.UpdateAgentLearningMemory(ctx, memory)
}

// DismissSuggestion removes a pending suggestion without approving it
func (s *LearningService) DismissSuggestion(ctx context.Context, tenantID int32, suggestionID string) (*store.AgentLearningMemory, error) {
	memory, err := s.store.GetOrCreateAgentLearningMemory(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	newPending := []store.PendingSuggestion{}
	for _, s := range memory.PendingSuggestions {
		if s.ID != suggestionID {
			newPending = append(newPending, s)
		}
	}
	memory.PendingSuggestions = newPending

	return s.store.UpdateAgentLearningMemory(ctx, memory)
}

// RemoveLearnedBehavior removes a learned behavior
func (s *LearningService) RemoveLearnedBehavior(ctx context.Context, tenantID int32, behaviorID string) (*store.AgentLearningMemory, error) {
	memory, err := s.store.GetOrCreateAgentLearningMemory(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	newBehaviors := []store.LearnedBehavior{}
	for _, b := range memory.LearnedBehaviors {
		if b.ID != behaviorID {
			newBehaviors = append(newBehaviors, b)
		}
	}
	memory.LearnedBehaviors = newBehaviors

	return s.store.UpdateAgentLearningMemory(ctx, memory)
}

// ToggleLearnedBehavior toggles a learned behavior's active state
func (s *LearningService) ToggleLearnedBehavior(ctx context.Context, tenantID int32, behaviorID string) (*store.AgentLearningMemory, error) {
	memory, err := s.store.GetOrCreateAgentLearningMemory(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	for i := range memory.LearnedBehaviors {
		if memory.LearnedBehaviors[i].ID == behaviorID {
			memory.LearnedBehaviors[i].IsActive = !memory.LearnedBehaviors[i].IsActive
			break
		}
	}

	return s.store.UpdateAgentLearningMemory(ctx, memory)
}

// ClearAllLearning removes all learned behaviors and pending suggestions
func (s *LearningService) ClearAllLearning(ctx context.Context, tenantID int32) error {
	return s.store.DeleteAgentLearningMemory(ctx, tenantID)
}

// GetActiveLearnedBehaviors returns only active learned behaviors for system prompt injection
func (s *LearningService) GetActiveLearnedBehaviors(ctx context.Context, tenantID int32) ([]store.LearnedBehavior, error) {
	memory, err := s.store.GetOrCreateAgentLearningMemory(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	active := []store.LearnedBehavior{}
	for _, b := range memory.LearnedBehaviors {
		if b.IsActive {
			active = append(active, b)
		}
	}
	return active, nil
}

// ApplySelectedLearnings applies selected issues and suggestions from an analysis result
// directly to the learned behaviors (simplified v2 workflow)
func (s *LearningService) ApplySelectedLearnings(ctx context.Context, tenantID int32, analysisID string, selectedIssues []int, selectedSuggestions []int) (*store.AgentLearningMemory, int, error) {
	// Get the analysis result
	analysis, err := s.store.GetAgentAnalysisResult(ctx, &store.FindAgentAnalysisResult{ID: &analysisID})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get analysis result: %w", err)
	}
	if analysis == nil {
		return nil, 0, fmt.Errorf("analysis result not found: %s", analysisID)
	}
	if analysis.TenantID != tenantID {
		return nil, 0, fmt.Errorf("analysis does not belong to this tenant")
	}

	// Get existing learning memory
	memory, err := s.store.GetOrCreateAgentLearningMemory(ctx, tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get learning memory: %w", err)
	}

	appliedCount := 0
	now := time.Now().Format("2006-01-02")

	// Apply selected issues
	for _, idx := range selectedIssues {
		if idx >= 0 && idx < len(analysis.Issues) {
			issue := analysis.Issues[idx]
			behavior := store.LearnedBehavior{
				ID:         uuid.New().String()[:8],
				Content:    issue.Message,
				Type:       "issue",
				SourceID:   analysisID,
				SourceTurn: issue.Turn,
				Source:     fmt.Sprintf("Analysis %s (Turn %d)", analysisID[:8], issue.Turn),
				AddedAt:    now,
				IsActive:   true,
			}
			memory.LearnedBehaviors = append(memory.LearnedBehaviors, behavior)
			appliedCount++
		}
	}

	// Apply selected suggestions
	for _, idx := range selectedSuggestions {
		if idx >= 0 && idx < len(analysis.Suggestions) {
			suggestion := analysis.Suggestions[idx]
			behavior := store.LearnedBehavior{
				ID:       uuid.New().String()[:8],
				Content:  suggestion,
				Type:     "suggestion",
				SourceID: analysisID,
				Source:   fmt.Sprintf("Analysis %s", analysisID[:8]),
				AddedAt:  now,
				IsActive: true,
			}
			memory.LearnedBehaviors = append(memory.LearnedBehaviors, behavior)
			appliedCount++
		}
	}

	if appliedCount == 0 {
		return memory, 0, nil
	}

	// Enforce maximum behaviors limit
	if len(memory.LearnedBehaviors) > MaxLearnedBehaviors {
		// Keep only the most recent MaxLearnedBehaviors
		memory.LearnedBehaviors = memory.LearnedBehaviors[len(memory.LearnedBehaviors)-MaxLearnedBehaviors:]
	}

	memory.AnalysisCount++

	// Save updated memory
	updated, err := s.store.UpdateAgentLearningMemory(ctx, memory)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to update learning memory: %w", err)
	}

	return updated, appliedCount, nil
}

// categorizeIssue maps an issue message to a category
func categorizeIssue(message string) string {
	// Simple keyword-based categorization
	keywords := map[string][]string{
		"information_gathering": {"phone", "contact", "name", "address", "email", "collect", "gather", "missing"},
		"tone_resolution":       {"tone", "empathy", "resolution", "abrupt", "ending", "closing", "acknowledge"},
		"conversation_flow":     {"flow", "structure", "skip", "section", "order", "sequence"},
		"policy_compliance":     {"rule", "policy", "comply", "compliance", "violat", "should not"},
		"service_alignment":     {"service", "offer", "recommend", "exclude", "available"},
		"intent_recognition":    {"intent", "identify", "recogni", "understand", "misunderst"},
	}

	messageLower := message
	for category, kws := range keywords {
		for _, kw := range kws {
			if containsIgnoreCase(messageLower, kw) {
				return category
			}
		}
	}
	return "general"
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && containsIgnoreCaseImpl(s, substr)))
}

func containsIgnoreCaseImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			pc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if pc >= 'A' && pc <= 'Z' {
				pc += 32
			}
			if sc != pc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// generateSuggestion creates a suggestion from a category and issue
func generateSuggestion(category, issue string) store.PendingSuggestion {
	triggers := map[string]string{
		"information_gathering": "During information gathering phase",
		"tone_resolution":       "When addressing customer concerns or closing",
		"conversation_flow":     "Throughout the conversation",
		"policy_compliance":     "When discussing services or policies",
		"service_alignment":     "When recommending services",
		"intent_recognition":    "At the start of conversation",
		"general":               "Throughout the conversation",
	}

	trigger := triggers[category]
	if trigger == "" {
		trigger = triggers["general"]
	}

	return store.PendingSuggestion{
		Category: category,
		Trigger:  trigger,
		Behavior: issue, // The issue description becomes the behavior to fix
	}
}
