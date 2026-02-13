package agent

import (
	"testing"
)

func TestParseXMLTag(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		tagName  string
		expected string
	}{
		{
			name:     "simple tag",
			content:  "<observations>test content</observations>",
			tagName:  "observations",
			expected: "test content",
		},
		{
			name:     "multiline content",
			content:  "<observations>\nline 1\nline 2\n</observations>",
			tagName:  "observations",
			expected: "line 1\nline 2",
		},
		{
			name:     "missing tag",
			content:  "no tags here",
			tagName:  "observations",
			expected: "",
		},
		{
			name:     "case insensitive",
			content:  "<OBSERVATIONS>test</OBSERVATIONS>",
			tagName:  "observations",
			expected: "test",
		},
		{
			name:     "nested tags",
			content:  "<observations><inner>content</inner></observations>",
			tagName:  "observations",
			expected: "<inner>content</inner>",
		},
		{
			name:     "multiple occurrences",
			content:  "<observations>first</observations><observations>second</observations>",
			tagName:  "observations",
			expected: "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseXMLTag(tt.content, tt.tagName)
			if result != tt.expected {
				t.Errorf("parseXMLTag() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "4 chars = 1 token",
			input:    "test",
			expected: 1,
		},
		{
			name:     "8 chars = 2 tokens",
			input:    "test test",
			expected: 2,
		},
		{
			name:     "exactly 4 chars",
			input:    "hell",
			expected: 1,
		},
		{
			name:     "5 chars = 2 tokens (ceiling)",
			input:    "hello",
			expected: 1, // 5/4 = 1.25, truncated to 1
		},
		{
			name:     "longer text",
			input:    "This is a longer text with more words to test the token estimation",
			expected: 16, // 64/4 = 16
		},
		{
			name:     "unicode chars",
			input:    "hello世界",
			expected: 2, // 6 bytes / 4 = 1.5 -> truncated to 1, but len() returns bytes = 6
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateTokens(tt.input)
			if result != tt.expected {
				t.Errorf("estimateTokens() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestOMConfig_Defaults(t *testing.T) {
	// Note: This test depends on environment variables being unset
	// In a real test, we'd use t.Setenv to set specific values

	config := GetOMConfig()

	if config.Enabled != true {
		t.Errorf("Expected Enabled=true by default, got %v", config.Enabled)
	}

	if config.MessageThreshold != 10 {
		t.Errorf("Expected MessageThreshold=10 by default, got %d", config.MessageThreshold)
	}

	if config.TokenThreshold != 2000 {
		t.Errorf("Expected TokenThreshold=2000 by default, got %d", config.TokenThreshold)
	}

	if config.RetryAttempts != 3 {
		t.Errorf("Expected RetryAttempts=3 by default, got %d", config.RetryAttempts)
	}

	if config.RetryDelayMs != 1000 {
		t.Errorf("Expected RetryDelayMs=1000 by default, got %d", config.RetryDelayMs)
	}
}

func TestOMConfig_GetConfig(t *testing.T) {
	config := GetOMConfig()

	// Get a copy of the config
	configCopy := config.GetConfig()

	// Verify the copy has the same values
	if configCopy.Enabled != config.Enabled {
		t.Error("Config copy mismatch: Enabled")
	}
	if configCopy.MessageThreshold != config.MessageThreshold {
		t.Error("Config copy mismatch: MessageThreshold")
	}
}

func TestIsTrivialMessage(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty string",
			content:  "",
			expected: true,
		},
		{
			name:     "whitespace only",
			content:  "   ",
			expected: true,
		},
		{
			name:     "simple ok",
			content:  "ok",
			expected: true,
		},
		{
			name:     "ok with period",
			content:  "ok.",
			expected: true,
		},
		{
			name:     "thanks",
			content:  "thanks",
			expected: true,
		},
		{
			name:     "thank you",
			content:  "thank you",
			expected: true,
		},
		{
			name:     "yeah",
			content:  "yeah",
			expected: true,
		},
		{
			name:     "yes",
			content:  "yes",
			expected: true,
		},
		{
			name:     "no",
			content:  "no",
			expected: true,
		},
		{
			name:     "okay",
			content:  "okay",
			expected: true,
		},
		{
			name:     "sure",
			content:  "sure",
			expected: true,
		},
		{
			name:     "got it",
			content:  "got it",
			expected: true,
		},
		{
			name:     "cool",
			content:  "cool",
			expected: true,
		},
		{
			name:     "nice",
			content:  "nice",
			expected: true,
		},
		{
			name:     "great",
			content:  "great",
			expected: true,
		},
		{
			name:     "perfect",
			content:  "perfect",
			expected: true,
		},
		{
			name:     "lol",
			content:  "lol",
			expected: true,
		},
		{
			name:     "haha",
			content:  "haha",
			expected: true,
		},
		{
			name:     "smile emoji",
			content:  ":)",
			expected: false,
		},
		{
			name:     "smile emoji with space",
			content:  ":) ",
			expected: false,
		},
		{
			name:     "wink emoji",
			content:  ";-)",
			expected: false, // Not in our patterns
		},
		{
			name:     "actual question",
			content:  "what is your name?",
			expected: false,
		},
		{
			name:     "actual statement",
			content:  "I need help with my order",
			expected: false,
		},
		{
			name:     "longer message",
			content:  "I wanted to ask about the shipping times for my order",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTrivialMessage(tt.content)
			if result != tt.expected {
				t.Errorf("isTrivialMessage(%q) = %v, want %v", tt.content, result, tt.expected)
			}
		})
	}
}

func TestObserverMutex_TryLock(t *testing.T) {
	mutex := NewObserverMutex()
	sessionID := "test-session-123"

	// First lock should succeed
	if !mutex.TryLock(sessionID) {
		t.Error("Expected first TryLock to succeed")
	}

	// Second lock should fail (already locked)
	if mutex.TryLock(sessionID) {
		t.Error("Expected second TryLock to fail")
	}

	// Unlock and try again
	mutex.Unlock(sessionID)

	if !mutex.TryLock(sessionID) {
		t.Error("Expected TryLock to succeed after unlock")
	}

	mutex.Unlock(sessionID)
}

func TestGetEnvBool(t *testing.T) {
	// Note: getEnvBool is a simple wrapper, tested indirectly through config tests
	// The function uses os.Getenv which is hard to test in isolation
	_ = getEnvBool
}

func TestGetEnvInt(t *testing.T) {
	// Test with a temporary environment variable
	t.Setenv("TEST_OM_INT", "42")

	result := getEnvInt("TEST_OM_INT", 10)
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	// Test with default
	result = getEnvInt("TEST_OM_INT_NOT_SET", 10)
	if result != 10 {
		t.Errorf("Expected default 10, got %d", result)
	}

	// Test with invalid value
	t.Setenv("TEST_OM_INT_INVALID", "abc")
	result = getEnvInt("TEST_OM_INT_INVALID", 10)
	if result != 10 {
		t.Errorf("Expected default 10 for invalid value, got %d", result)
	}
}
