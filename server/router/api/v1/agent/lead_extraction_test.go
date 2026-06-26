package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func TestExtractContactInfo_LowercaseName(t *testing.T) {
	result := ExtractContactInfo("izak zuk", "")
	require.NotNil(t, result)
	require.Equal(t, "Izak Zuk", result.Name)
	require.Equal(t, "regex", result.Source)
}

func TestExtractContactInfo_Email(t *testing.T) {
	result := ExtractContactInfo("izk@izk.net", "")
	require.NotNil(t, result)
	require.Equal(t, "izk@izk.net", result.Email)
}

func TestExtractContactInfo_PrefixBasedName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"I'm John Smith", "John Smith"},
		{"my name is alice", "Alice"},
		{"call me Bob", "Bob"},
		{"this is Mary-Jane", "Mary-Jane"},
		{"you can call me Sarah", "Sarah"},
		{"I am O'Brien", "O'Brien"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractContactInfo(tt.input, "")
			require.NotNil(t, result, "input: %q", tt.input)
			require.Equal(t, tt.expected, result.Name, "input: %q", tt.input)
		})
	}
}

func TestExtractContactInfo_UnicodeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"José García", "José García"},
		{"田中太郎", "田中太郎"},
		{"Иван Петров", "Иван Петров"},
		{"François Müller", "François Müller"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractContactInfo(tt.input, "")
			require.NotNil(t, result)
			require.Equal(t, tt.expected, result.Name)
		})
	}
}

func TestExtractContactInfo_PhoneUS(t *testing.T) {
	result := ExtractContactInfo("555-123-4567", "")
	require.NotNil(t, result)
	require.Equal(t, "555-123-4567", result.Phone)
}

func TestExtractContactInfo_PhoneInternational(t *testing.T) {
	result := ExtractContactInfo("+442079460958", "")
	require.NotNil(t, result)
	require.Contains(t, result.Phone, "+44")
}

func TestExtractContactInfo_SpamRejection(t *testing.T) {
	spamInputs := []string{"asdf", "aaaaaa", "test", "qwer"}
	for _, input := range spamInputs {
		result := ExtractContactInfo(input, "")
		require.Nil(t, result, "spam input %q should be rejected", input)
	}
}

func TestExtractContactInfo_CommonWordFilter(t *testing.T) {
	// These should NOT produce name extractions
	nonNames := []string{"yes please", "no need", "hi there", "ok thanks", "good morning"}
	for _, input := range nonNames {
		result := ExtractContactInfo(input, "")
		if result != nil {
			require.Empty(t, result.Name, "common phrase %q should not produce a name", input)
		}
	}
}

func TestExtractContactInfo_EmailWithPlus(t *testing.T) {
	result := ExtractContactInfo("user+tag@domain.com", "")
	require.NotNil(t, result)
	require.Equal(t, "user+tag@domain.com", result.Email)
}

func TestExtractContactInfo_PlaceholderEmail(t *testing.T) {
	result := ExtractContactInfo("test@example.com", "")
	require.Nil(t, result)
}

func TestIsCorrectionMessage(t *testing.T) {
	corrections := []string{
		"No, I meant john2000@email.com",
		"actually it's alice@email.com",
		"not john, but alice@email.com",
	}
	for _, input := range corrections {
		require.True(t, IsCorrectionMessage(input), "should detect correction: %q", input)
	}

	nonCorrections := []string{
		"hello",
		"what services do you offer?",
		"my name is john",
	}
	for _, input := range nonCorrections {
		require.False(t, IsCorrectionMessage(input), "should not detect correction: %q", input)
	}
}

func TestIsDeclineMessage(t *testing.T) {
	declines := []string{
		"I'd rather not share that",
		"no thanks",
		"not really",
		"maybe later",
		"I'll pass",
	}
	for _, input := range declines {
		require.True(t, IsDeclineMessage(input), "should detect decline: %q", input)
	}

	nonDeclines := []string{
		"hello",
		"my name is john",
		"what services?",
	}
	for _, input := range nonDeclines {
		require.False(t, IsDeclineMessage(input), "should not detect decline: %q", input)
	}
}

func TestClassifyMessage(t *testing.T) {
	tests := []struct {
		input    string
		expected MessageIntent
	}{
		{"hi", IntentGreeting},
		{"hello", IntentGreeting},
		{"what services do you offer?", IntentQuestion},
		{"I'm John", IntentProvideName},
		{"john@email.com", IntentProvideEmail},
		{"555-123-4567", IntentProvidePhone},
		{"No, I meant john2000@email.com", IntentCorrectPrevious},
		{"I'd rather not", IntentDeclineContact},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			intent := ClassifyMessage(tt.input)
			require.Equal(t, tt.expected, intent)
		})
	}
}

func TestMergeExtractions(t *testing.T) {
	draft := NewLeadDraft()

	// First extraction: name only
	result1 := &ExtractionResult{
		Name:       "John",
		Confidence: 0.7,
		Source:     "regex",
	}
	MergeExtractions(draft, result1)
	require.Equal(t, "John", draft.Name)
	require.Equal(t, 0.7, draft.Confidence["name"])

	// Second extraction: email only
	result2 := &ExtractionResult{
		Email:      "john@email.com",
		Confidence: 0.9,
		Source:     "regex",
	}
	MergeExtractions(draft, result2)
	require.Equal(t, "John", draft.Name)
	require.Equal(t, "john@email.com", draft.Email)

	// Third extraction: corrected name (higher confidence)
	result3 := &ExtractionResult{
		Name:       "John Smith",
		Confidence: 0.8,
		Source:     "llm",
		Corrected:  true,
	}
	MergeExtractions(draft, result3)
	require.Equal(t, "John Smith", draft.Name)
	require.True(t, draft.Corrected["name"])
}

func TestMergeExtractions_Declined(t *testing.T) {
	draft := NewLeadDraft()
	result := &ExtractionResult{
		Declined: true,
	}
	MergeExtractions(draft, result)
	require.True(t, draft.Declined)
}

func TestShouldCommitLead(t *testing.T) {
	// Empty draft
	draft := NewLeadDraft()
	require.False(t, ShouldCommitLead(draft))

	// Name only (no contact)
	draft.Name = "John"
	require.False(t, ShouldCommitLead(draft))

	// Name + email (sufficient)
	draft.Email = "john@email.com"
	draft.Confidence["name"] = 0.7
	draft.Confidence["email"] = 0.9
	require.True(t, ShouldCommitLead(draft))

	// Declined
	draft.Declined = true
	require.False(t, ShouldCommitLead(draft))
}

func TestExtractContactInfoFull(t *testing.T) {
	messages := []store.AgentMessage{
		{Role: "user", Content: "do you handle onboarding via zoom"},
		{Role: "assistant", Content: "I don't have information about that. Can I collect your name and email for follow-up?"},
		{Role: "user", Content: "izak zuk"},
		{Role: "user", Content: "izk@izk.net"},
	}

	ctx := context.Background()
	draft := ExtractContactInfoFull(ctx, "", messages, "", nil)
	require.NotNil(t, draft)
	require.Equal(t, "Izak Zuk", draft.Name)
	require.Equal(t, "izk@izk.net", draft.Email)
	require.True(t, draft.HasContactInfo())
}

func TestExtractContactInfoFull_Declined(t *testing.T) {
	messages := []store.AgentMessage{
		{Role: "user", Content: "do you handle onboarding via zoom"},
		{Role: "assistant", Content: "I don't have information about that. Can I collect your name and email for follow-up?"},
		{Role: "user", Content: "I'd rather not share that"},
	}

	ctx := context.Background()
	draft := ExtractContactInfoFull(ctx, "", messages, "", nil)
	require.NotNil(t, draft)
	require.True(t, draft.Declined)
}

func TestExtractContactInfoFull_Correction(t *testing.T) {
	messages := []store.AgentMessage{
		{Role: "user", Content: "I'm John"},
		{Role: "user", Content: "actually my name is James and james@email.com"},
	}

	ctx := context.Background()
	draft := ExtractContactInfoFull(ctx, "", messages, "", nil)
	require.NotNil(t, draft)
	require.Equal(t, "James", draft.Name)
	require.Equal(t, "james@email.com", draft.Email)
}

func TestIsLikelyName(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"John Smith", true},
		{"izak zuk", true},
		{"José García", true},
		{"hello", false},
		{"yes please", false},
		{"123 Main St", false},
		{"user@email.com", false},
		{"a", false},
		{"asdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isLikelyName(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeAndTitleCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"izak zuk", "Izak Zuk"},
		{"JOHN SMITH", "John Smith"},
		{"jOSÉ gARCÍA", "José García"},
		{"o'brien", "O'Brien"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeAndTitleCase(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
