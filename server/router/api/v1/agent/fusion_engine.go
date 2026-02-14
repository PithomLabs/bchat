package agent

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

// FusionEngine combines OM observations with RAG retrieval for unlimited memory.
type FusionEngine struct {
	TemporalDecay       float64   // Weight decay factor per day (default: 0.1)
	RelevanceWeight     float64   // Weight for relevance score (default: 0.6)
	RecencyWeight       float64   // Weight for recency (default: 0.4)
	MaxContextTokens    int       // Maximum context size (default: 30000)
	MinObservationScore float64   // Minimum score to include (default: 0.1)
	ReferenceTime       time.Time // Reference time for temporal calculations (default: now)
}

// NewFusionEngine creates a new FusionEngine with default settings.
func NewFusionEngine() *FusionEngine {
	return &FusionEngine{
		TemporalDecay:       0.1,
		RelevanceWeight:     0.6,
		RecencyWeight:       0.4,
		MaxContextTokens:    30000,
		MinObservationScore: 0.1,
		ReferenceTime:       time.Time{}, // Zero value means use current time
	}
}

// ContextItem represents a single item in the fused context.
type ContextItem struct {
	Content       string    `json:"content"`
	Score         float64   `json:"score"`
	Source        string    `json:"source"` // "om" or "rag"
	Date          time.Time `json:"date"`
	Tokens        int       `json:"tokens"`
	Priority      string    `json:"priority"` // "high", "medium", "low"
	ObservationID int       `json:"observation_id,omitempty"`
	ChunkID       string    `json:"chunk_id,omitempty"`
}

// FusionResult holds the result of fusion operation.
type FusionResult struct {
	Items        []ContextItem `json:"items"`
	TotalTokens  int           `json:"total_tokens"`
	OMItems      int           `json:"om_items"`
	RAGItems     int           `json:"rag_items"`
	DroppedItems int           `json:"dropped_items"`
	MaxScore     float64       `json:"max_score"`
	MinScore     float64       `json:"min_score"`
}

// Fuse combines observations and RAG results into a unified context.
func (f *FusionEngine) Fuse(
	ctx context.Context,
	obsLog *store.ObservationLog,
	ragResult *SearchResult,
	query string,
) (*FusionResult, error) {
	startTime := time.Now()

	items := make([]ContextItem, 0)

	// 1. Process OM observations
	if obsLog != nil && obsLog.ObservationLog != "" {
		omItems := f.parseObservations(obsLog)
		for _, item := range omItems {
			// Score based on query relevance and temporal decay
			item.Score = f.scoreObservation(item, query)
			items = append(items, item)
		}
	}

	// 2. Process RAG results
	if ragResult != nil && len(ragResult.Chunks) > 0 {
		for i, chunk := range ragResult.Chunks {
			ragScore := float64(0)
			if i < len(ragResult.Scores) {
				ragScore = ragResult.Scores[i]
			}

			item := ContextItem{
				Content: fmt.Sprintf("%s: %s", chunk.Title, chunk.Content),
				Score:   ragScore,
				Source:  "rag",
				Date:    chunk.IndexedAt,
				Tokens:  estimateTokens(chunk.Content),
				ChunkID: chunk.ID,
			}
			items = append(items, item)
		}
	}

	// 3. Deduplicate similar content
	items = f.deduplicate(items)

	// 4. Sort by score (descending)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	// 5. Truncate to token limit
	items, droppedCount := f.truncateToLimit(items)

	// Calculate stats
	omCount := 0
	ragCount := 0
	totalTokens := 0
	maxScore := float64(0)
	minScore := float64(1)

	for _, item := range items {
		totalTokens += item.Tokens
		if item.Source == "om" {
			omCount++
		} else {
			ragCount++
		}
		if item.Score > maxScore {
			maxScore = item.Score
		}
		if item.Score < minScore {
			minScore = item.Score
		}
	}

	duration := time.Since(startTime)
	slog.Info("FusionEngine completed",
		"total_items", len(items),
		"om_items", omCount,
		"rag_items", ragCount,
		"dropped_items", droppedCount,
		"total_tokens", totalTokens,
		"max_score", maxScore,
		"min_score", minScore,
		"duration_ms", duration.Milliseconds())

	return &FusionResult{
		Items:        items,
		TotalTokens:  totalTokens,
		OMItems:      omCount,
		RAGItems:     ragCount,
		DroppedItems: droppedCount,
		MaxScore:     maxScore,
		MinScore:     minScore,
	}, nil
}

// parseObservations parses the observation log into individual items.
func (f *FusionEngine) parseObservations(obsLog *store.ObservationLog) []ContextItem {
	items := make([]ContextItem, 0)

	// Parse observation log format:
	// Date: YYYY-MM-DD
	// - [PRIORITY] HH:MM Content
	//   - [PRIORITY] HH:MM Sub-content

	lines := strings.Split(obsLog.ObservationLog, "\n")
	var currentDate time.Time
	var currentPriority string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for date header
		if strings.HasPrefix(line, "Date: ") {
			dateStr := strings.TrimPrefix(line, "Date: ")
			parsed, err := time.Parse("2006-01-02", dateStr)
			if err == nil {
				currentDate = parsed
			}
			continue
		}

		// Check for observation line
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "  - ") {
			content := strings.TrimPrefix(line, "- ")
			content = strings.TrimPrefix(content, "  - ")

			// Extract priority emoji
			priority := "medium"
			if strings.Contains(content, "red_circle") || strings.Contains(content, "high") {
				priority = "high"
				currentPriority = "high"
			} else if strings.Contains(content, "yellow_circle") || strings.Contains(content, "medium") {
				priority = "medium"
				currentPriority = "medium"
			} else if strings.Contains(content, "green_circle") || strings.Contains(content, "low") {
				priority = "low"
				currentPriority = "low"
			} else if currentPriority != "" {
				priority = currentPriority
			}

			// Remove emoji and timestamp prefix
			content = cleanObservationContent(content)

			if content != "" {
				item := ContextItem{
					Content:  content,
					Date:     currentDate,
					Tokens:   estimateTokens(content),
					Source:   "om",
					Priority: priority,
				}
				items = append(items, item)
			}
		}
	}

	return items
}

// scoreObservation calculates a score for an observation based on query relevance and temporal decay.
func (f *FusionEngine) scoreObservation(item ContextItem, query string) float64 {
	// 1. Base relevance score (simple keyword matching for now)
	relevanceScore := f.calculateRelevance(item.Content, query)

	// 2. Temporal decay
	temporalScore := f.calculateTemporalWeight(item.Date)

	// 3. Priority boost
	priorityBoost := float64(1.0)
	switch item.Priority {
	case "high":
		priorityBoost = 1.5
	case "medium":
		priorityBoost = 1.0
	case "low":
		priorityBoost = 0.7
	}

	// Combine scores
	finalScore := (relevanceScore*f.RelevanceWeight + temporalScore*f.RecencyWeight) * priorityBoost

	// Normalize to 0-1 range
	if finalScore > 1 {
		finalScore = 1
	}

	return finalScore
}

// calculateRelevance calculates relevance between content and query.
func (f *FusionEngine) calculateRelevance(content, query string) float64 {
	if query == "" || content == "" {
		return 0.5 // Default score when no query
	}

	// Simple keyword overlap scoring
	contentLower := strings.ToLower(content)
	queryLower := strings.ToLower(query)

	queryTokens := tokenize(queryLower)
	contentTokens := tokenize(contentLower)

	if len(queryTokens) == 0 {
		return 0.5
	}

	// Count matching tokens
	matches := 0
	contentTokenSet := make(map[string]bool)
	for _, t := range contentTokens {
		contentTokenSet[t] = true
	}

	for _, t := range queryTokens {
		if contentTokenSet[t] {
			matches++
		}
	}

	// Jaccard-like similarity
	relevance := float64(matches) / float64(len(queryTokens))

	// Boost for exact phrase matches
	if strings.Contains(contentLower, queryLower) {
		relevance = math.Min(1.0, relevance+0.3)
	}

	return relevance
}

// calculateTemporalWeight calculates weight based on age.
func (f *FusionEngine) calculateTemporalWeight(obsDate time.Time) float64 {
	if obsDate.IsZero() {
		return 0.5 // Default for unknown dates
	}

	// Use ReferenceTime if set, otherwise use current time
	refTime := f.ReferenceTime
	if refTime.IsZero() {
		refTime = time.Now()
	}

	daysOld := refTime.Sub(obsDate).Hours() / 24

	switch {
	case daysOld < 1:
		return 1.0 // Full weight for today
	case daysOld < 7:
		// Linear decay from 1.0 to 0.3 over a week
		return 1.0 - (daysOld * 0.1)
	case daysOld < 30:
		// Slower decay from 0.3 to 0.1 over a month
		return 0.3 - ((daysOld - 7) * 0.01)
	default:
		// Minimum weight for old observations
		return 0.1
	}
}

// deduplicate removes similar content items.
func (f *FusionEngine) deduplicate(items []ContextItem) []ContextItem {
	if len(items) <= 1 {
		return items
	}

	result := make([]ContextItem, 0, len(items))
	seen := make(map[string]bool)

	for _, item := range items {
		// Create a simplified key for deduplication
		key := simplifyContent(item.Content)

		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		} else {
			// If duplicate, keep the one with higher score
			for i, existing := range result {
				existingKey := simplifyContent(existing.Content)
				if existingKey == key && item.Score > existing.Score {
					result[i] = item
					break
				}
			}
		}
	}

	return result
}

// simplifyContent creates a simplified version of content for comparison.
func simplifyContent(content string) string {
	// Lowercase, remove punctuation, collapse whitespace
	content = strings.ToLower(content)

	replacer := strings.NewReplacer(
		".", " ", ",", " ", "!", " ", "?", " ", ";", " ", ":", " ",
		"(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ",
		"\"", " ", "'", " ", "-", " ", "_", " ", "/", " ", "\\", " ",
	)
	content = replacer.Replace(content)

	// Collapse whitespace
	words := strings.Fields(content)
	if len(words) > 10 {
		// Use first 10 words as key
		words = words[:10]
	}

	return strings.Join(words, " ")
}

// truncateToLimit truncates items to fit within token limit.
func (f *FusionEngine) truncateToLimit(items []ContextItem) ([]ContextItem, int) {
	if len(items) == 0 {
		return items, 0
	}

	totalTokens := 0
	result := make([]ContextItem, 0, len(items))
	droppedCount := 0

	for _, item := range items {
		if item.Score < f.MinObservationScore {
			droppedCount++
			continue
		}

		if totalTokens+item.Tokens <= f.MaxContextTokens {
			result = append(result, item)
			totalTokens += item.Tokens
		} else {
			droppedCount++
		}
	}

	return result, droppedCount
}

// cleanObservationContent removes emoji and timestamp prefixes from observation content.
func cleanObservationContent(content string) string {
	// Remove common emoji prefixes
	emojiPatterns := []string{"red_circle", "yellow_circle", "green_circle", "circle"}
	for _, emoji := range emojiPatterns {
		content = strings.ReplaceAll(content, emoji, "")
	}

	// Remove timestamp pattern (HH:MM)
	// Pattern: "HH:MM " at the start
	if len(content) > 5 {
		// Check for timestamp pattern
		if content[2] == ':' && content[5] == ' ' {
			// Looks like "HH:MM content"
			content = content[6:]
		}
	}

	// Trim whitespace
	content = strings.TrimSpace(content)

	return content
}

// BuildContextString builds a formatted context string from fusion result.
func (f *FusionEngine) BuildContextString(result *FusionResult) string {
	var sb strings.Builder

	sb.WriteString("## Retrieved Context\n\n")

	// Group by source
	omItems := make([]ContextItem, 0)
	ragItems := make([]ContextItem, 0)

	for _, item := range result.Items {
		if item.Source == "om" {
			omItems = append(omItems, item)
		} else {
			ragItems = append(ragItems, item)
		}
	}

	// Write OM observations first
	if len(omItems) > 0 {
		sb.WriteString("### Recent Observations\n\n")
		for _, item := range omItems {
			priorityMarker := ""
			switch item.Priority {
			case "high":
				priorityMarker = "[IMPORTANT] "
			case "low":
				priorityMarker = "[INFO] "
			}
			sb.WriteString(fmt.Sprintf("- %s%s\n", priorityMarker, item.Content))
		}
		sb.WriteString("\n")
	}

	// Write RAG results
	if len(ragItems) > 0 {
		sb.WriteString("### Relevant Knowledge\n\n")
		for _, item := range ragItems {
			sb.WriteString(fmt.Sprintf("- %s\n", item.Content))
		}
	}

	return sb.String()
}
