package agent

import (
	"regexp"
	"strings"
)

// ScoringCategory represents a universal scoring category.
type ScoringCategory struct {
	Name        string            `json:"name"`
	Weight      int               `json:"weight"`
	Score       int               `json:"score"`       // 0-100
	Level       string            `json:"level"`       // "high", "medium", "low"
	Keywords    map[string][]string `json:"keywords"`  // level -> keywords
	Description string            `json:"description"`
}

// ConversationScore represents the overall scoring of a conversation turn.
type ConversationScore struct {
	TotalScore    int                        `json:"total_score"`    // 0-100 weighted
	Categories    map[string]*ScoringCategory `json:"categories"`
	Urgency       string                      `json:"urgency"`       // "emergency", "urgent", "standard"
	ShouldEscalate bool                       `json:"should_escalate"`
	Reasoning     string                      `json:"reasoning"`
}

// ConversationScorer handles heuristic scoring of conversation turns.
type ConversationScorer struct {
	categories []*ScoringCategory
}

// NewConversationScorer creates a scorer with universal categories.
func NewConversationScorer() *ConversationScorer {
	return &ConversationScorer{
		categories: getUniversalCategories(),
	}
}

// getUniversalCategories returns the 6 universal scoring categories.
func getUniversalCategories() []*ScoringCategory {
	return []*ScoringCategory{
		{
			Name:        "urgency",
			Weight:      25,
			Description: "How immediately is help needed?",
			Keywords: map[string][]string{
				"high": {
					"immediately", "urgent", "asap", "right now", "emergency",
					"right away", "as soon as possible", "critical", "dire",
					"can't wait", "need help now", "happening now", "currently",
				},
				"medium": {
					"soon", "today", "this week", "quickly", "promptly",
					"as soon as you can", "when possible", "timely",
				},
				"low": {
					"whenever", "no rush", "at your convenience", "not urgent",
					"when you get a chance", "sometime", "eventually",
				},
			},
		},
		{
			Name:        "safety_risk",
			Weight:      20,
			Description: "Is there physical or financial danger?",
			Keywords: map[string][]string{
				"high": {
					"danger", "unsafe", "hurt", "injured", "fire", "flood",
					"gas leak", "smoke", "electrocution", "collapse", "trapped",
					"emergency", "life threatening", "hazard", "toxic",
				},
				"medium": {
					"concern", "worried", "problem", "issue", "broken",
					"not working", "damage", "leak", "malfunction",
				},
				"low": {},
			},
		},
		{
			Name:        "service_match",
			Weight:      20,
			Description: "Does the request match our service offerings?",
			Keywords:    map[string][]string{}, // Checked dynamically against KB
		},
		{
			Name:        "escalation_signal",
			Weight:      15,
			Description: "Does the customer want a supervisor or to escalate?",
			Keywords: map[string][]string{
				"high": {
					"manager", "supervisor", "lawyer", "sue", "suing", "lawsuit",
					"complaint", "bbb", "better business bureau", "attorney",
					"legal action", "report you", "your boss", "corporate",
					"headquarters", "speak to someone else", "higher up",
				},
				"medium": {
					"unhappy", "dissatisfied", "not working", "frustrated",
					"disappointed", "unacceptable", "terrible service",
					"worst experience", "never again", "waste of time",
				},
				"low": {},
			},
		},
		{
			Name:        "lead_quality",
			Weight:      10,
			Description: "Has the customer provided contact information?",
			Keywords:    map[string][]string{}, // Checked via regex patterns
		},
		{
			Name:        "sentiment",
			Weight:      10,
			Description: "What is the customer's emotional state?",
			Keywords: map[string][]string{
				"positive": {
					"thank you", "thanks", "appreciate", "great", "helpful",
					"wonderful", "excellent", "perfect", "amazing", "fantastic",
					"good job", "well done", "impressed", "satisfied",
				},
				"negative": {
					"frustrated", "angry", "upset", "terrible", "worst",
					"horrible", "awful", "disgusted", "furious", "livid",
					"outraged", "annoyed", "irritated", "fed up",
				},
				"neutral": {},
			},
		},
	}
}

// ScoreMessage scores a single user message.
func (s *ConversationScorer) ScoreMessage(message string, config *AudienceConfig) *ConversationScore {
	result := &ConversationScore{
		Categories: make(map[string]*ScoringCategory),
	}

	messageLower := strings.ToLower(message)
	totalWeightedScore := 0
	totalWeight := 0

	for _, cat := range s.categories {
		// Create a copy for this scoring
		scoredCat := &ScoringCategory{
			Name:        cat.Name,
			Weight:      cat.Weight,
			Description: cat.Description,
			Keywords:    cat.Keywords,
		}

		switch cat.Name {
		case "service_match":
			scoredCat.Score, scoredCat.Level = s.scoreServiceMatch(messageLower, config)
		case "lead_quality":
			scoredCat.Score, scoredCat.Level = s.scoreLeadQuality(message)
		case "sentiment":
			scoredCat.Score, scoredCat.Level = s.scoreSentiment(messageLower, cat.Keywords)
		default:
			scoredCat.Score, scoredCat.Level = s.scoreByKeywords(messageLower, cat.Keywords)
		}

		result.Categories[cat.Name] = scoredCat
		totalWeightedScore += scoredCat.Score * scoredCat.Weight
		totalWeight += scoredCat.Weight
	}

	// Calculate total score
	if totalWeight > 0 {
		result.TotalScore = totalWeightedScore / totalWeight
	}

	// Determine urgency level
	urgencyScore := 0
	safetyScore := 0
	if cat, ok := result.Categories["urgency"]; ok {
		urgencyScore = cat.Score
	}
	if cat, ok := result.Categories["safety_risk"]; ok {
		safetyScore = cat.Score
	}

	combinedUrgency := (urgencyScore + safetyScore) / 2
	if combinedUrgency >= 75 {
		result.Urgency = "emergency"
	} else if combinedUrgency >= 40 {
		result.Urgency = "urgent"
	} else {
		result.Urgency = "standard"
	}

	// Check for escalation
	if cat, ok := result.Categories["escalation_signal"]; ok {
		result.ShouldEscalate = cat.Level == "high"
	}

	// Build reasoning
	result.Reasoning = s.buildReasoning(result)

	return result
}

// scoreByKeywords scores based on keyword matching.
func (s *ConversationScorer) scoreByKeywords(message string, keywords map[string][]string) (int, string) {
	// Check high keywords first
	for _, kw := range keywords["high"] {
		if strings.Contains(message, kw) {
			return 100, "high"
		}
	}

	// Check medium keywords
	for _, kw := range keywords["medium"] {
		if strings.Contains(message, kw) {
			return 50, "medium"
		}
	}

	// Check low keywords
	for _, kw := range keywords["low"] {
		if strings.Contains(message, kw) {
			return 10, "low"
		}
	}

	return 0, "none"
}

// scoreServiceMatch checks if the message mentions services we offer.
func (s *ConversationScorer) scoreServiceMatch(message string, config *AudienceConfig) (int, string) {
	if config == nil || len(config.Services) == 0 {
		return 50, "unknown" // No KB to check against
	}

	matchedServices := 0
	matchedExclusions := 0

	// Check for service matches
	for _, svc := range config.Services {
		svcLower := strings.ToLower(svc.Name)
		if strings.Contains(message, svcLower) {
			matchedServices++
		}
		// Also check keywords in description
		descWords := strings.Fields(strings.ToLower(svc.Description))
		for _, word := range descWords {
			if len(word) > 5 && strings.Contains(message, word) {
				matchedServices++
				break
			}
		}
	}

	// Check for exclusion matches
	for _, exc := range config.Exclusions {
		excLower := strings.ToLower(exc.Name)
		if strings.Contains(message, excLower) {
			matchedExclusions++
		}
	}

	if matchedServices > 0 && matchedExclusions == 0 {
		return 100, "high" // Good match to our services
	} else if matchedServices > 0 && matchedExclusions > 0 {
		return 50, "medium" // Mixed - mentions both
	} else if matchedExclusions > 0 {
		return 20, "low" // Mentions excluded services
	}

	return 30, "unknown" // No clear service mentioned
}

// scoreLeadQuality checks for contact information in the message.
func (s *ConversationScorer) scoreLeadQuality(message string) (int, string) {
	score := 0

	// Check for phone number
	phonePattern := regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b`)
	if phonePattern.MatchString(message) {
		score += 40
	}

	// Check for email
	emailPattern := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
	if emailPattern.MatchString(message) {
		score += 30
	}

	// Check for name patterns (e.g., "my name is", "I'm", "this is")
	namePatterns := []string{
		`my name is \w+`,
		`i'm \w+`,
		`this is \w+`,
		`call me \w+`,
	}
	for _, pattern := range namePatterns {
		if matched, _ := regexp.MatchString(`(?i)`+pattern, message); matched {
			score += 20
			break
		}
	}

	// Check for address patterns
	addressPattern := regexp.MustCompile(`\b\d+\s+[\w\s]+\s+(street|st|avenue|ave|road|rd|drive|dr|lane|ln|court|ct|boulevard|blvd)\b`)
	if addressPattern.MatchString(strings.ToLower(message)) {
		score += 10
	}

	if score >= 70 {
		return score, "high"
	} else if score >= 30 {
		return score, "medium"
	} else if score > 0 {
		return score, "low"
	}

	return 0, "none"
}

// scoreSentiment analyzes the emotional tone.
func (s *ConversationScorer) scoreSentiment(message string, keywords map[string][]string) (int, string) {
	positiveCount := 0
	negativeCount := 0

	for _, kw := range keywords["positive"] {
		if strings.Contains(message, kw) {
			positiveCount++
		}
	}

	for _, kw := range keywords["negative"] {
		if strings.Contains(message, kw) {
			negativeCount++
		}
	}

	if negativeCount > positiveCount {
		// Negative sentiment - higher score means more attention needed
		if negativeCount >= 3 {
			return 100, "negative"
		}
		return 70, "negative"
	} else if positiveCount > negativeCount {
		// Positive sentiment
		return 30, "positive"
	}

	return 50, "neutral"
}

// buildReasoning generates a human-readable reasoning string.
func (s *ConversationScorer) buildReasoning(score *ConversationScore) string {
	var parts []string

	if cat, ok := score.Categories["urgency"]; ok && cat.Level == "high" {
		parts = append(parts, "High urgency detected")
	}
	if cat, ok := score.Categories["safety_risk"]; ok && cat.Level == "high" {
		parts = append(parts, "Safety risk indicated")
	}
	if cat, ok := score.Categories["escalation_signal"]; ok && cat.Level == "high" {
		parts = append(parts, "Escalation requested")
	}
	if cat, ok := score.Categories["sentiment"]; ok && cat.Level == "negative" {
		parts = append(parts, "Negative sentiment")
	}
	if cat, ok := score.Categories["lead_quality"]; ok && cat.Score >= 70 {
		parts = append(parts, "Good lead info provided")
	}

	if len(parts) == 0 {
		return "Standard inquiry"
	}

	return strings.Join(parts, "; ")
}

// DefaultScorer is a package-level scorer instance.
var DefaultScorer = NewConversationScorer()

// ScoreUserMessage is a convenience function using the default scorer.
func ScoreUserMessage(message string, config *AudienceConfig) *ConversationScore {
	return DefaultScorer.ScoreMessage(message, config)
}
