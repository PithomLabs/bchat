package agent

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/usememos/memos/store"
)

// ============================================================================
// LEAD EXTRACTION TYPES
// ============================================================================

// LeadDraft holds incremental extraction state across multiple messages.
type LeadDraft struct {
	Name        string             `json:"name,omitempty"`
	Email       string             `json:"email,omitempty"`
	Phone       string             `json:"phone,omitempty"`
	Location    string             `json:"location,omitempty"`
	Confidence  map[string]float64 `json:"confidence"`
	Sources     map[string]string  `json:"sources"`
	Corrected   map[string]bool    `json:"corrected"`
	Declined    bool               `json:"declined"`
	LastMessage string             `json:"last_message"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// NewLeadDraft creates a new empty lead draft.
func NewLeadDraft() *LeadDraft {
	return &LeadDraft{
		Confidence: make(map[string]float64),
		Sources:    make(map[string]string),
		Corrected:  make(map[string]bool),
	}
}

// HasContactInfo returns true if we have name + (email or phone).
func (d *LeadDraft) HasContactInfo() bool {
	return d.Name != "" && (d.Email != "" || d.Phone != "")
}

// ConfidenceScore returns overall confidence (0.0-1.0).
func (d *LeadDraft) ConfidenceScore() float64 {
	nameConf := d.Confidence["name"]
	emailConf := d.Confidence["email"]
	phoneConf := d.Confidence["phone"]
	contactConf := max(emailConf, phoneConf)
	return 0.4*nameConf + 0.6*contactConf
}

// ExtractionResult holds the output of a single extraction attempt.
type ExtractionResult struct {
	Name       string  `json:"name,omitempty"`
	Email      string  `json:"email,omitempty"`
	Phone      string  `json:"phone,omitempty"`
	Address    string  `json:"address,omitempty"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"` // "regex", "structural", "llm"
	Corrected  bool    `json:"corrected"`
	Declined   bool    `json:"declined"`
}

// ============================================================================
// LAYER 1: ENHANCED REGEX + HEURISTICS
// ============================================================================

var (
	// Name patterns — support Unicode, various formats
	namePatterns = []*regexp.Regexp{
		// Prefix-based: "I'm John", "my name is Alice", "call me Bob"
		// Captures 1-2 words after prefix, stopping at punctuation or email
		regexp.MustCompile(`(?i)(?:I'm|I am|my name is|this is|it's|call me|you can call me|people call me|name's)\s+([\p{L}][\p{L}'\-]*(?:\s+[\p{L}][\p{L}'\-]*)?)(?:\s*[,;.]|\s+(?:and|but|or|so|that|which|who|whom|at|in|on|to|for|with|from)\s|$)`),
		// Standalone: "izak zuk" or "JOHN SMITH" (1-2 words, letters only)
		regexp.MustCompile("(?i)^([\\p{L}][\\p{L}'\\-]*(?:\\s+[\\p{L}][\\p{L}'\\-]*)?)$"),
		// Title + name: "Dr. Smith", "Mr. Johnson"
		regexp.MustCompile(`(?i)(?:Mr|Mrs|Ms|Dr|Prof|Sir|Madam)\.?\s+([\p{L}][\p{L}'\-]*(?:\s+[\p{L}][\p{L}'\-]*)?)`),
	}

	// Email pattern — supports internationalized email
	emailPattern = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)

	// Phone patterns — US and international
	phonePatternUS   = regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?([2-9]\d{2})\)?[-.\s]?(\d{3})[-.\s]?(\d{4})\b`)
	phonePatternIntl = regexp.MustCompile(`\+[1-9]\d{6,14}`)

	// Correction patterns
	correctionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:no|nope|not\s+quite|that'?s\s+not|incorrect|wrong|mistake)`),
		regexp.MustCompile(`(?i)(?:I\s+meant|actually|correction|let\s+me\s+clarify|I\s+misspoke|I\s+spoke\s+wrong)`),
		regexp.MustCompile(`(?i)(?:not\s+\S+,?\s*(?:but|it'?s|is)\s+\S+)`),
	}

	// Decline patterns
	declinePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:I'?d\s+rather\s+not|I'?ll\s+pass|no\s+thanks|not\s+(?:really|interested|right\s+now)|skip\s+(?:that|this)|maybe\s+later)`),
		regexp.MustCompile(`(?i)(?:don'?t\s+(?:want|need)\s+(?:to\s+)?(?:share|give|provide))`),
		regexp.MustCompile(`(?i)(?:prefer\s+not\s+to|rather\s+not\s+say)`),
	}

	// Common words that should NOT be extracted as names
	leadCommonWords = map[string]bool{
		"hello": true, "hi": true, "hey": true, "the": true, "a": true,
		"an": true, "is": true, "it": true, "my": true, "your": true,
		"here": true, "there": true, "this": true, "that": true,
		"yes": true, "no": true, "okay": true, "ok": true, "need": true,
		"sure": true, "yeah": true, "yep": true, "right": true,
		"absolutely": true, "certainly": true, "definitely": true,
		"great": true, "perfect": true, "thanks": true, "thank": true,
		"please": true, "sorry": true, "well": true, "just": true,
		"maybe": true, "probably": true, "about": true, "very": true,
		"good": true, "morning": true, "afternoon": true, "evening": true,
		"today": true, "tomorrow": true, "yesterday": true,
		"can": true, "could": true, "would": true, "should": true,
		"do": true, "does": true, "did": true, "want": true, "like": true,
		"know": true, "think": true, "help": true, "helping": true,
	}

	// Spam placeholder words (exact match)
	spamPlaceholderPattern = regexp.MustCompile(`^(?:asdf|qwer|zxcv|test|foo|bar|xyz)$`)
)

// isLeadCommonWord checks if a string or any of its words is a common word.
func isLeadCommonWord(s string) bool {
	words := strings.Fields(s)
	for _, w := range words {
		if leadCommonWords[strings.ToLower(w)] {
			return true
		}
	}
	return false
}

// isSpamInput checks if input looks like spam/test data.
func isSpamInput(content string) bool {
	// Check placeholder words
	if spamPlaceholderPattern.MatchString(strings.ToLower(content)) {
		return true
	}
	// Check low entropy (too few unique characters relative to length)
	if utf8.RuneCountInString(content) > 3 {
		runes := []rune(content)
		unique := make(map[rune]bool)
		for _, r := range runes {
			unique[r] = true
		}
		ratio := float64(len(unique)) / float64(len(runes))
		if ratio < 0.3 {
			return true
		}
	}
	return false
}

// isLikelyName validates if a string looks like a real name.
func isLikelyName(s string) bool {
	if len(s) < 2 {
		return false
	}
	if isLeadCommonWord(s) {
		return false
	}
	// Check spam placeholder pattern (asdf, qwer, etc.)
	if spamPlaceholderPattern.MatchString(strings.ToLower(s)) {
		return false
	}
	// Must contain at least one letter
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return false
	}
	// Reject if contains digits (likely phone or order number)
	for _, r := range s {
		if unicode.IsDigit(r) {
			return false
		}
	}
	// Reject if matches email pattern
	if emailPattern.MatchString(s) {
		return false
	}
	// Reject if >4 words (probably a sentence)
	if len(strings.Fields(s)) > 4 {
		return false
	}
	return true
}

// normalizeAndTitleCase normalizes a name to title case for storage consistency.
// Preserves casing after hyphens (Mary-Jane) and apostrophes (O'Brien).
func normalizeAndTitleCase(name string) string {
	// Split by spaces, then by hyphens and apostrophes, capitalizing each segment
	words := strings.Fields(name)
	for i, word := range words {
		// Handle hyphenated/apostrophe names: "mary-jane" -> "Mary-Jane", "o'brien" -> "O'Brien"
		// First split by hyphen, then each part by apostrophe
		hyphenParts := strings.Split(word, "-")
		for j, hyphenPart := range hyphenParts {
			apostropheParts := strings.Split(hyphenPart, "'")
			for k, segment := range apostropheParts {
				runes := []rune(segment)
				if len(runes) > 0 {
					runes[0] = unicode.ToUpper(runes[0])
					for m := 1; m < len(runes); m++ {
						runes[m] = unicode.ToLower(runes[m])
					}
					apostropheParts[k] = string(runes)
				}
			}
			hyphenParts[j] = strings.Join(apostropheParts, "'")
		}
		words[i] = strings.Join(hyphenParts, "-")
	}
	return strings.Join(words, " ")
}

// ExtractContactInfo performs Layer 1 extraction from a single message.
// Returns nil if no useful info found.
func ExtractContactInfo(content string, tenantPhone string) *ExtractionResult {
	if isSpamInput(content) {
		return nil
	}

	result := &ExtractionResult{
		Source: "regex",
	}

	// Extract name
	name := extractName(content)
	if name != "" {
		result.Name = name
		result.Confidence = 0.7
	}

	// Extract email
	email := extractEmail(content)
	if email != "" {
		result.Email = email
		result.Confidence = 0.9
	}

	// Extract phone
	phone := extractPhone(content, tenantPhone)
	if phone != "" {
		result.Phone = phone
		result.Confidence = 0.8
	}

	if result.Name == "" && result.Email == "" && result.Phone == "" {
		return nil
	}

	return result
}

func extractName(content string) string {
	for _, pattern := range namePatterns {
		if match := pattern.FindStringSubmatch(content); len(match) > 1 {
			name := strings.TrimSpace(match[1])
			if isLikelyName(name) {
				return normalizeAndTitleCase(name)
			}
		}
	}
	return ""
}

func extractEmail(content string) string {
	match := emailPattern.FindString(content)
	if match != "" && !isPlaceholderEmailCheck(match) {
		return match
	}
	return ""
}

func extractPhone(content string, tenantPhone string) string {
	// Try US format first
	if match := phonePatternUS.FindString(content); match != "" {
		normalized := normalizePhoneDigits(match)
		if !isPlaceholderPhoneDigits(normalized) && normalized != normalizePhoneDigits(tenantPhone) {
			return match
		}
	}
	// Try international format
	if match := phonePatternIntl.FindString(content); match != "" {
		normalized := normalizePhoneDigits(match)
		if len(normalized) >= 7 && !isPlaceholderPhoneDigits(normalized) {
			return match
		}
	}
	return ""
}

// IsCorrectionMessage detects if the user is correcting previously given info.
func IsCorrectionMessage(content string) bool {
	hasCorrection := false
	for _, pattern := range correctionPatterns {
		if pattern.MatchString(content) {
			hasCorrection = true
			break
		}
	}
	if !hasCorrection {
		return false
	}
	// Must also contain contact-like data
	return containsContactSignals(content)
}

// IsDeclineMessage detects if the user is declining to provide contact info.
func IsDeclineMessage(content string) bool {
	for _, pattern := range declinePatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

func containsContactSignals(content string) bool {
	if emailPattern.MatchString(content) {
		return true
	}
	if phonePatternUS.MatchString(content) || phonePatternIntl.MatchString(content) {
		return true
	}
	// Check for name-like content (2+ words that could be a name)
	words := strings.Fields(content)
	if len(words) >= 2 && len(words) <= 4 {
		hasLetter := false
		for _, w := range words {
			for _, r := range w {
				if unicode.IsLetter(r) {
					hasLetter = true
					break
				}
			}
		}
		if hasLetter {
			return true
		}
	}
	return false
}

// ============================================================================
// LAYER 2: STRUCTURAL ANALYSIS
// ============================================================================

// MessageIntent represents what a user message is trying to do.
type MessageIntent int

const (
	IntentUnknown MessageIntent = iota
	IntentProvideName
	IntentProvideEmail
	IntentProvidePhone
	IntentProvideLocation
	IntentCorrectPrevious
	IntentDeclineContact
	IntentGreeting
	IntentQuestion
)

// ClassifyMessage determines the intent of a user message.
func ClassifyMessage(content string) MessageIntent {
	if IsDeclineMessage(content) {
		return IntentDeclineContact
	}
	if IsCorrectionMessage(content) {
		return IntentCorrectPrevious
	}

	// Check for greeting-only messages
	if isGreetingOnly(content) {
		return IntentGreeting
	}

	// Check for question-only messages
	if isQuestionOnly(content) {
		return IntentQuestion
	}

	// Check for contact provision
	hasEmail := emailPattern.MatchString(content)
	hasPhone := phonePatternUS.MatchString(content) || phonePatternIntl.MatchString(content)
	hasName := extractName(content) != ""

	if hasName && !hasEmail && !hasPhone {
		return IntentProvideName
	}
	if hasEmail && !hasName && !hasPhone {
		return IntentProvideEmail
	}
	if hasPhone && !hasName && !hasEmail {
		return IntentProvidePhone
	}
	if hasName || hasEmail || hasPhone {
		return IntentProvideName // Multi-field, treat as name-centric
	}

	return IntentUnknown
}

func isGreetingOnly(content string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(content))
	greetings := []string{"hi", "hello", "hey", "good morning", "good afternoon", "good evening", "greetings", "yo", "sup"}
	for _, g := range greetings {
		if trimmed == g {
			return true
		}
	}
	return false
}

func isQuestionOnly(content string) bool {
	trimmed := strings.TrimSpace(content)
	if strings.HasSuffix(trimmed, "?") {
		// Check if it's a short question without contact data
		if !emailPattern.MatchString(content) && !phonePatternUS.MatchString(content) {
			words := strings.Fields(content)
			if len(words) <= 15 {
				return true
			}
		}
	}
	return false
}

// MergeExtractions merges a new extraction into an existing lead draft.
// Uses confidence-weighted replacement.
func MergeExtractions(draft *LeadDraft, result *ExtractionResult) {
	if result == nil {
		return
	}

	// Handle decline
	if result.Declined {
		draft.Declined = true
		draft.UpdatedAt = time.Now()
		return
	}

	// Merge fields
	type mergeField struct {
		name    string
		newVal  string
		newConf float64
	}

	fields := []mergeField{
		{"name", result.Name, result.Confidence},
		{"email", result.Email, 0.9},
		{"phone", result.Phone, 0.8},
	}

	for _, f := range fields {
		if f.newVal == "" {
			continue
		}

		existingVal := getField(draft, f.name)
		existingConf := draft.Confidence[f.name]

		shouldReplace := false
		switch {
		case existingVal == "":
			shouldReplace = true // Fill empty field
		case result.Corrected && f.newVal != "":
			shouldReplace = true // Customer explicitly corrected
		case f.newConf > existingConf+0.2:
			shouldReplace = true // Significantly better extraction
		}

		if shouldReplace {
			setField(draft, f.name, f.newVal)
			draft.Confidence[f.name] = f.newConf
			draft.Sources[f.name] = result.Source
			draft.Corrected[f.name] = result.Corrected
		}
	}

	draft.LastMessage = result.Source // track last processed source
	draft.UpdatedAt = time.Now()
}

func getField(draft *LeadDraft, field string) string {
	switch field {
	case "name":
		return draft.Name
	case "email":
		return draft.Email
	case "phone":
		return draft.Phone
	case "location":
		return draft.Location
	}
	return ""
}

func setField(draft *LeadDraft, field, value string) {
	switch field {
	case "name":
		draft.Name = value
	case "email":
		draft.Email = value
	case "phone":
		draft.Phone = value
	case "location":
		draft.Location = value
	}
}

// ShouldCommitLead determines if the draft has enough confidence to persist.
func ShouldCommitLead(draft *LeadDraft) bool {
	if draft == nil || draft.Declined {
		return false
	}
	if !draft.HasContactInfo() {
		return false
	}
	return draft.ConfidenceScore() >= 0.6
}

// ============================================================================
// SESSION LEAD DRAFT INTEGRATION
// ============================================================================

// GetOrCreateLeadDraft gets or creates the lead draft on a session.
func GetOrCreateLeadDraft(session *store.AgentSession) *LeadDraft {
	// LeadDraft is stored in-memory on the session via a special tag.
	// Since we can't modify the store.AgentSession struct without migration,
	// we use a package-level map keyed by session ID.
	// This is acceptable for the MVP; for production, add to session struct.
	return getSessionLeadDraft(session.ID)
}

// sessionLeadDrafts maps session ID to its lead draft (in-memory).
var sessionLeadDrafts = make(map[string]*LeadDraft)

func getSessionLeadDraft(sessionID string) *LeadDraft {
	if d, ok := sessionLeadDrafts[sessionID]; ok {
		return d
	}
	d := NewLeadDraft()
	sessionLeadDrafts[sessionID] = d
	return d
}

// ClearSessionLeadDraft clears the lead draft for a session.
func ClearSessionLeadDraft(sessionID string) {
	delete(sessionLeadDrafts, sessionID)
}
