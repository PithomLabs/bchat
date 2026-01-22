package agent

import (
	"regexp"
	"strings"
)

// Sanitizer handles output sanitization to remove hallucinated content.
type Sanitizer struct {
	patterns []*regexp.Regexp
}

// NewSanitizer creates a new Sanitizer with default patterns.
func NewSanitizer() *Sanitizer {
	patterns := []string{
		// Remove hallucinated system tags
		`(?m)^\[SYSTEM\].*$`,
		`(?m)^\[DEBUG\].*$`,
		`(?m)^\[THOUGHT\].*$`,
		`(?m)^\[INTERNAL\].*$`,
		`(?m)^\[NOTE\].*$`,
		`(?m)^\[ASSISTANT\].*$`,
		// Remove inline system tags
		`\[(SYSTEM|DEBUG|THOUGHT|INTERNAL|NOTE)[^\]]*\]`,
		// Remove self-references
		`(?m)^Assistant:.*`,
		`(?m)^AI:.*`,
		`(?m)^Bot:.*`,
		// Remove thinking out loud patterns
		`(?m)^Let me think.*$`,
		`(?m)^I should.*$`,
		`(?m)^I need to.*$`,
		// Remove action descriptions
		`(?m)^\*[^*]+\*$`,
		// Remove markdown that slipped through
		`(?m)^#{1,6}\s+`,
		`\*\*([^*]+)\*\*`,
		`\*([^*]+)\*`,
		`__([^_]+)__`,
		`_([^_]+)_`,
		// Remove code blocks
		"(?s)```[^`]*```",
		"`[^`]+`",
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	return &Sanitizer{patterns: compiled}
}

// Sanitize removes hallucinated content and cleans up the response.
func (s *Sanitizer) Sanitize(response string) string {
	result := response

	// Apply all patterns
	for _, re := range s.patterns {
		// For patterns that capture groups, replace with the captured content
		if re.NumSubexp() > 0 {
			result = re.ReplaceAllString(result, "$1")
		} else {
			result = re.ReplaceAllString(result, "")
		}
	}

	// Clean up excessive whitespace
	result = strings.TrimSpace(result)

	// Replace multiple newlines with double newline
	multiNewline := regexp.MustCompile(`\n{3,}`)
	result = multiNewline.ReplaceAllString(result, "\n\n")

	// Replace multiple spaces with single space
	multiSpace := regexp.MustCompile(` {2,}`)
	result = multiSpace.ReplaceAllString(result, " ")

	return result
}

// SanitizeWithReport returns the sanitized response and a report of what was removed.
func (s *Sanitizer) SanitizeWithReport(response string) (string, []SanitizationReport) {
	var reports []SanitizationReport
	result := response

	patternNames := []string{
		"system_tags_multiline",
		"debug_tags_multiline",
		"thought_tags_multiline",
		"internal_tags_multiline",
		"note_tags_multiline",
		"assistant_tags_multiline",
		"inline_system_tags",
		"assistant_prefix",
		"ai_prefix",
		"bot_prefix",
		"thinking_let_me",
		"thinking_i_should",
		"thinking_i_need",
		"action_descriptions",
		"markdown_headers",
		"markdown_bold_double",
		"markdown_bold_single",
		"markdown_italic_double",
		"markdown_italic_single",
		"code_blocks",
		"inline_code",
	}

	for i, re := range s.patterns {
		matches := re.FindAllString(result, -1)
		if len(matches) > 0 {
			name := "unknown"
			if i < len(patternNames) {
				name = patternNames[i]
			}
			reports = append(reports, SanitizationReport{
				PatternName: name,
				Matches:     matches,
				Count:       len(matches),
			})

			// Apply replacement
			if re.NumSubexp() > 0 {
				result = re.ReplaceAllString(result, "$1")
			} else {
				result = re.ReplaceAllString(result, "")
			}
		}
	}

	// Clean up whitespace
	result = strings.TrimSpace(result)
	multiNewline := regexp.MustCompile(`\n{3,}`)
	result = multiNewline.ReplaceAllString(result, "\n\n")
	multiSpace := regexp.MustCompile(` {2,}`)
	result = multiSpace.ReplaceAllString(result, " ")

	return result, reports
}

// SanitizationReport contains details about what was sanitized.
type SanitizationReport struct {
	PatternName string   `json:"pattern_name"`
	Matches     []string `json:"matches"`
	Count       int      `json:"count"`
}

// DefaultSanitizer is a package-level sanitizer instance.
var DefaultSanitizer = NewSanitizer()

// SanitizeResponse is a convenience function using the default sanitizer.
func SanitizeResponse(response string) string {
	return DefaultSanitizer.Sanitize(response)
}

// CorrectContacts replaces impossible/hallucinated placeholder phone numbers with the correct one.
// Only catches patterns that are clearly impossible (no real phone would use these):
// - 000-000-0000 (all zeros)
// - 999-999-9999 (all nines)
// - 123-456-7890 (sequential test number)
//
// NOTE: 555-XXX-XXXX patterns are NOT corrected here because:
// 1. The LLM is instructed via system prompt to use only the company phone
// 2. Customer-provided numbers (even if they say "555-...") should be preserved verbatim
func (s *Sanitizer) CorrectContacts(response string, replacementPhone string) string {
	// Only impossible patterns that no real customer would provide
	impossiblePatterns := []string{
		// All zeros: 000-000-0000, (000) 000-0000
		`\(?000\)?[-.\s]?000[-.\s]?0000`,
		// All nines: 999-999-9999, (999) 999-9999
		`\(?999\)?[-.\s]?999[-.\s]?9999`,
		// Sequential test number: 123-456-7890, (123) 456-7890
		`\(?123\)?[-.\s]?456[-.\s]?7890`,
	}

	result := response

	for _, pattern := range impossiblePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		if replacementPhone != "" {
			// Replace with the correct phone
			result = re.ReplaceAllString(result, replacementPhone)
		} else {
			// Remove the placeholder (replace with generic message)
			result = re.ReplaceAllString(result, "[phone number]")
		}
	}

	return result
}

// CorrectContactsWithReport replaces impossible placeholder phones and returns a report.
// Only catches patterns that are clearly impossible (see CorrectContacts for details).
func (s *Sanitizer) CorrectContactsWithReport(response string, replacementPhone string) (string, []ContactCorrectionReport) {
	var reports []ContactCorrectionReport

	// Only impossible patterns that no real customer would provide
	impossiblePatterns := map[string]string{
		`\(?000\)?[-.\s]?000[-.\s]?0000`: "all_zeros",
		`\(?999\)?[-.\s]?999[-.\s]?9999`: "all_nines",
		`\(?123\)?[-.\s]?456[-.\s]?7890`: "test_number_123",
	}

	result := response

	for pattern, patternName := range impossiblePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllString(result, -1)
		if len(matches) > 0 {
			reports = append(reports, ContactCorrectionReport{
				PatternName:  patternName,
				OriginalText: matches,
				Replacement:  replacementPhone,
				Count:        len(matches),
			})

			if replacementPhone != "" {
				result = re.ReplaceAllString(result, replacementPhone)
			} else {
				result = re.ReplaceAllString(result, "[phone number]")
			}
		}
	}

	return result, reports
}

// ContactCorrectionReport contains details about contact corrections made.
type ContactCorrectionReport struct {
	PatternName  string   `json:"pattern_name"`
	OriginalText []string `json:"original_text"`
	Replacement  string   `json:"replacement"`
	Count        int      `json:"count"`
}

// CorrectContactsInResponse is a convenience function using the default sanitizer.
func CorrectContactsInResponse(response string, replacementPhone string) string {
	return DefaultSanitizer.CorrectContacts(response, replacementPhone)
}

// CorrectEmails replaces hallucinated placeholder emails with the correct one.
// Common patterns detected:
// - firstname.lastname@email.com (generic placeholder)
// - name@example.com (example domains)
// - test@test.com, user@user.com (test patterns)
func (s *Sanitizer) CorrectEmails(response string, replacementEmail string) string {
	// Common placeholder email patterns that indicate hallucination
	placeholderPatterns := []string{
		// firstname.lastname@email.com pattern (common LLM placeholder)
		`[a-z]+\.[a-z]+@email\.com`,
		// example.com/org/net domains (RFC 2606 reserved)
		`[a-zA-Z0-9._%+-]+@example\.(com|org|net)`,
		// test domains
		`[a-zA-Z0-9._%+-]+@test\.(com|org|net)`,
		// placeholder patterns
		`user@user\.com`,
		`test@test\.com`,
		`customer@customer\.com`,
		`name@name\.com`,
		// fake domains
		`[a-zA-Z0-9._%+-]+@fake(mail)?\.com`,
		`[a-zA-Z0-9._%+-]+@placeholder\.com`,
		// numbered placeholders
		`user\d+@.*\.com`,
		`customer\d+@.*\.com`,
	}

	result := response

	for _, pattern := range placeholderPatterns {
		re, err := regexp.Compile(`(?i)` + pattern) // case-insensitive
		if err != nil {
			continue
		}

		if replacementEmail != "" {
			// Replace with the correct email
			result = re.ReplaceAllString(result, replacementEmail)
		} else {
			// Remove the placeholder (replace with generic text)
			result = re.ReplaceAllString(result, "[email address]")
		}
	}

	return result
}

// CorrectEmailsWithReport replaces placeholder emails and returns a report.
func (s *Sanitizer) CorrectEmailsWithReport(response string, replacementEmail string) (string, []ContactCorrectionReport) {
	var reports []ContactCorrectionReport

	placeholderPatterns := map[string]string{
		`[a-z]+\.[a-z]+@email\.com`:                "firstname_lastname_email_com",
		`[a-zA-Z0-9._%+-]+@example\.(com|org|net)`: "example_domain",
		`[a-zA-Z0-9._%+-]+@test\.(com|org|net)`:    "test_domain",
		`user@user\.com`:                           "user_user",
		`test@test\.com`:                           "test_test",
		`customer@customer\.com`:                   "customer_customer",
		`[a-zA-Z0-9._%+-]+@fake(mail)?\.com`:       "fake_domain",
		`[a-zA-Z0-9._%+-]+@placeholder\.com`:       "placeholder_domain",
	}

	result := response

	for pattern, patternName := range placeholderPatterns {
		re, err := regexp.Compile(`(?i)` + pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllString(result, -1)
		if len(matches) > 0 {
			reports = append(reports, ContactCorrectionReport{
				PatternName:  patternName,
				OriginalText: matches,
				Replacement:  replacementEmail,
				Count:        len(matches),
			})

			if replacementEmail != "" {
				result = re.ReplaceAllString(result, replacementEmail)
			} else {
				result = re.ReplaceAllString(result, "[email address]")
			}
		}
	}

	return result, reports
}

// CorrectEmailsInResponse is a convenience function using the default sanitizer.
func CorrectEmailsInResponse(response string, replacementEmail string) string {
	return DefaultSanitizer.CorrectEmails(response, replacementEmail)
}

// ============================================================================
// PHONE NUMBER VALIDATION AND EXTRACTION
// ============================================================================

// IsValidReplacementPhone checks if a phone number is a valid replacement
// (i.e., not a placeholder pattern that shouldn't be used for replacement).
func IsValidReplacementPhone(phone string) bool {
	if phone == "" {
		return false
	}

	// Normalize to digits only for pattern matching
	digits := extractDigits(phone)

	// Reject common placeholder patterns
	placeholderPatterns := []string{
		"555",        // (555) area code - reserved for fiction
		"000",        // (000) area code - invalid
		"999",        // (999) area code - invalid
		"1234567890", // Common test number
		"0000000000", // All zeros
		"9999999999", // All nines
	}

	for _, pattern := range placeholderPatterns {
		if strings.Contains(digits, pattern) {
			return false
		}
	}

	// Check if it starts with 555 (common placeholder area code)
	if len(digits) >= 3 && digits[:3] == "555" {
		return false
	}

	// Check for area code starting with 0 (invalid in NANP)
	if len(digits) >= 10 && digits[0] == '0' {
		return false
	}

	// Must have at least 10 digits for a valid US phone
	if len(digits) < 10 {
		return false
	}

	return true
}

// extractDigits extracts only digits from a string.
func extractDigits(s string) string {
	var result strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// ExtractPhoneFromKB extracts the emergency phone number from raw KB content.
// Looks for common patterns in KB.MD files.
func ExtractPhoneFromKB(rawKB string) string {
	if rawKB == "" {
		return ""
	}

	// Patterns to find phone numbers in KB content
	// Priority order: Emergency Phone > Contact > General phone patterns
	patterns := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		// Emergency Phone: **(631) 358-5200** or similar
		{
			name:    "emergency_phone_bold",
			pattern: regexp.MustCompile(`\*\*Emergency Phone\*\*.*?\|\s*\*\*([^*]+)\*\*`),
		},
		// Phone: **(631) 358-5200**
		{
			name:    "phone_bold",
			pattern: regexp.MustCompile(`\*\*Phone:\*\*\s*\*?\*?([^*\n]+)`),
		},
		// Phone | **(631) 358-5200**
		{
			name:    "phone_table",
			pattern: regexp.MustCompile(`Phone\s*\|\s*\*\*([^*]+)\*\*`),
		},
		// Contact Number: (631) 358-5200
		{
			name:    "contact_number",
			pattern: regexp.MustCompile(`(?i)contact\s*(?:number|phone)?\s*[:|-]?\s*\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`),
		},
		// Emergency Line: (631) 358-5200
		{
			name:    "emergency_line",
			pattern: regexp.MustCompile(`(?i)emergency\s+(?:line|number|phone)\s*[:|-]?\s*(\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4})`),
		},
		// General phone pattern (fallback)
		{
			name:    "general_phone",
			pattern: regexp.MustCompile(`\(([2-9]\d{2})\)\s*(\d{3})[-.\s]?(\d{4})`),
		},
	}

	for _, p := range patterns {
		matches := p.pattern.FindStringSubmatch(rawKB)
		if len(matches) > 0 {
			// Extract the phone number from the match
			phone := ""
			if len(matches) > 1 {
				phone = strings.TrimSpace(matches[1])
			} else {
				phone = strings.TrimSpace(matches[0])
			}

			// Clean up markdown formatting
			phone = strings.ReplaceAll(phone, "*", "")
			phone = strings.TrimSpace(phone)

			// Extract the actual phone number if there's extra text
			phonePattern := regexp.MustCompile(`\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
			if extracted := phonePattern.FindString(phone); extracted != "" {
				phone = extracted
			}

			// Validate the extracted phone
			if IsValidReplacementPhone(phone) {
				return phone
			}
		}
	}

	return ""
}

// GetValidatedReplacementPhone returns a valid phone number for replacement.
// It first checks if the configured phone is valid, then falls back to extracting from KB.
func GetValidatedReplacementPhone(configuredPhone, rawKB string) string {
	// First, check if the configured phone is valid
	if IsValidReplacementPhone(configuredPhone) {
		return configuredPhone
	}

	// Fall back to extracting from KB
	if extractedPhone := ExtractPhoneFromKB(rawKB); extractedPhone != "" {
		return extractedPhone
	}

	// No valid phone found - return empty to trigger placeholder replacement mode
	return ""
}
