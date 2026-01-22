package agent

import (
	"context"
	"regexp"
	"strings"
)

// ComplianceResult contains the result of compliance verification.
type ComplianceResult struct {
	Passed     bool                  `json:"passed"`
	Violations []ComplianceViolation `json:"violations"`
	Score      int                   `json:"score"`      // 0-100
	Confidence int                   `json:"confidence"` // 0-100
}

// ComplianceViolation represents a single compliance violation.
type ComplianceViolation struct {
	Type       string `json:"type"`       // "hallucinated_service", "wrong_contact", etc.
	Severity   string `json:"severity"`   // "critical", "warning", "info"
	Content    string `json:"content"`    // The problematic text
	Suggestion string `json:"suggestion"` // How to fix
}

// ComplianceChecker handles compliance verification of agent responses.
type ComplianceChecker struct {
	phonePattern *regexp.Regexp
	emailPattern *regexp.Regexp
}

// NewComplianceChecker creates a new ComplianceChecker.
func NewComplianceChecker() *ComplianceChecker {
	return &ComplianceChecker{
		phonePattern: regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b`),
		emailPattern: regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
	}
}

// VerifyCompliance checks an agent response against the configuration.
func (c *ComplianceChecker) VerifyCompliance(ctx context.Context, response string, config *AudienceConfig) *ComplianceResult {
	result := &ComplianceResult{
		Passed:     true,
		Score:      100,
		Confidence: 100,
		Violations: []ComplianceViolation{},
	}

	// Check 1: Service mentions
	c.checkServiceMentions(response, config, result)

	// Check 2: Contact information
	c.checkContactInfo(response, config, result)

	// Check 3: Excluded services
	c.checkExcludedServices(response, config, result)

	// Check 4: Price/time promises (heuristic)
	c.checkPromises(response, result)

	// Calculate final pass/fail
	result.Passed = result.Score >= 70 && !c.hasCriticalViolation(result.Violations)

	return result
}

// checkServiceMentions verifies that mentioned services exist in KB.
func (c *ComplianceChecker) checkServiceMentions(response string, config *AudienceConfig, result *ComplianceResult) {
	if len(config.Services) == 0 {
		return
	}

	responseLower := strings.ToLower(response)

	// Build a set of valid service names and keywords
	validServices := make(map[string]bool)
	for _, svc := range config.Services {
		validServices[strings.ToLower(svc.Name)] = true
		// Also add keywords from description
		words := strings.Fields(strings.ToLower(svc.Description))
		for _, w := range words {
			if len(w) > 5 { // Only significant words
				validServices[w] = true
			}
		}
	}

	// Common service-related words that might indicate hallucination
	serviceIndicators := []string{
		"we offer", "we provide", "our service", "we can help with",
		"we specialize in", "we handle", "we do",
	}

	for _, indicator := range serviceIndicators {
		if idx := strings.Index(responseLower, indicator); idx != -1 {
			// Extract the phrase after the indicator
			remaining := responseLower[idx+len(indicator):]
			// Get the next few words (potential service name)
			words := strings.Fields(remaining)
			if len(words) >= 2 {
				potentialService := strings.Join(words[:min(3, len(words))], " ")
				potentialService = strings.Trim(potentialService, ".,!?")

				// Check if this looks like a service but isn't in our list
				found := false
				for validSvc := range validServices {
					if strings.Contains(potentialService, validSvc) || strings.Contains(validSvc, potentialService) {
						found = true
						break
					}
				}

				if !found && len(potentialService) > 3 {
					// Potential hallucination - but be conservative
					// Only flag if it really looks like a service name
					if c.looksLikeServiceName(potentialService) {
						result.Violations = append(result.Violations, ComplianceViolation{
							Type:       "potential_hallucinated_service",
							Severity:   "warning",
							Content:    potentialService,
							Suggestion: "Verify this service is in KB.MD or remove mention",
						})
						result.Score -= 10
					}
				}
			}
		}
	}
}

// looksLikeServiceName checks if a phrase looks like a service name.
func (c *ComplianceChecker) looksLikeServiceName(phrase string) bool {
	serviceWords := []string{
		"service", "repair", "installation", "maintenance", "cleaning",
		"restoration", "removal", "treatment", "inspection", "assessment",
		"consultation", "support", "assistance", "help", "work",
	}

	phraseLower := strings.ToLower(phrase)
	for _, word := range serviceWords {
		if strings.Contains(phraseLower, word) {
			return true
		}
	}
	return false
}

// checkContactInfo verifies that provided contact info matches config.
// Uses Option C hybrid approach:
// - Tier 1 (always active): Detect obvious placeholder patterns (555, 000, 123-456-7890)
// - Tier 2 (when config available): Validate against authorized contacts list
func (c *ComplianceChecker) checkContactInfo(response string, config *AudienceConfig, result *ComplianceResult) {
	// Extract phone numbers from response
	phones := c.phonePattern.FindAllString(response, -1)

	for _, phone := range phones {
		normalized := c.normalizePhone(phone)

		// === TIER 1: Always check for placeholder patterns (no config required) ===
		if c.isPlaceholderPhone(normalized) {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:       "hallucinated_contact",
				Severity:   "critical",
				Content:    phone,
				Suggestion: "Detected placeholder phone number - this appears to be hallucinated",
			})
			result.Score -= 40
			continue // Skip Tier 2 check for this phone
		}

		// === TIER 2: Validate against config if available ===
		// Use GetValidatedReplacementPhone to get the real phone from KB.MD
		// This fixes the issue where DB has placeholder (555) 000-0000 but KB.MD has real phone
		validatedPhone := ""
		if config.Audience != nil {
			validatedPhone = GetValidatedReplacementPhone(config.Audience.EmergencyPhone, config.RawKB)
		}

		if validatedPhone != "" {
			validatedNormalized := c.normalizePhone(validatedPhone)

			// Build list of authorized phones
			authorizedPhones := []string{validatedNormalized}

			// Add secondary phones if available and valid
			for _, secondary := range config.Audience.SecondaryPhones {
				if secondary != "" && IsValidReplacementPhone(secondary) {
					authorizedPhones = append(authorizedPhones, c.normalizePhone(secondary))
				}
			}

			// Check if phone is in authorized list
			isAuthorized := false
			for _, auth := range authorizedPhones {
				if normalized == auth {
					isAuthorized = true
					break
				}
			}

			if !isAuthorized {
				result.Violations = append(result.Violations, ComplianceViolation{
					Type:       "unauthorized_contact",
					Severity:   "critical",
					Content:    phone,
					Suggestion: "Use authorized phone: " + validatedPhone, // Use validated, not raw DB value
				})
				result.Score -= 30
			}
		}
	}

	// Extract emails from response
	emails := c.emailPattern.FindAllString(response, -1)
	for _, email := range emails {
		emailLower := strings.ToLower(email)

		// === TIER 1: Check for placeholder emails ===
		if c.isPlaceholderEmail(emailLower) {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:       "hallucinated_contact",
				Severity:   "critical",
				Content:    email,
				Suggestion: "Detected placeholder email - this appears to be hallucinated",
			})
			result.Score -= 40
			continue
		}

		// === TIER 2: Validate against config if available ===
		configEmail := strings.ToLower(config.Audience.Email)
		if configEmail != "" && emailLower != configEmail {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:       "unauthorized_contact",
				Severity:   "critical",
				Content:    email,
				Suggestion: "Use configured email: " + config.Audience.Email,
			})
			result.Score -= 30
		}
	}
}

// isPlaceholderPhone checks if a normalized phone number is a common placeholder pattern.
// These patterns are ALWAYS flagged regardless of tenant configuration.
func (c *ComplianceChecker) isPlaceholderPhone(normalized string) bool {
	// Common placeholder patterns (after normalization to digits only)
	placeholderPatterns := []string{
		"5550",      // 555-0XXX (reserved for fiction)
		"5551",      // 555-1XXX (reserved for fiction)
		"0000",      // Contains 0000
		"1234567890", // Sequential test number
		"9999",      // Contains 9999
		"1111111111", // All 1s
		"0000000000", // All 0s
	}

	for _, pattern := range placeholderPatterns {
		if strings.Contains(normalized, pattern) {
			return true
		}
	}

	// Check for 555 area code (reserved for fiction in US)
	if len(normalized) >= 10 && normalized[:3] == "555" {
		return true
	}

	// Check for obvious fake patterns
	if len(normalized) == 10 {
		// 123-456-7890
		if normalized == "1234567890" {
			return true
		}
		// 000-000-0000
		if normalized == "0000000000" {
			return true
		}
		// 999-999-9999
		if normalized == "9999999999" {
			return true
		}
	}

	return false
}

// isPlaceholderEmail checks if an email is a common placeholder pattern.
func (c *ComplianceChecker) isPlaceholderEmail(email string) bool {
	placeholderDomains := []string{
		"example.com",
		"example.org",
		"example.net",
		"test.com",
		"fake.com",
		"placeholder.com",
		"sample.com",
		"domain.com",
		"email.com",
		"mail.test",
	}

	placeholderNames := []string{
		"test@",
		"fake@",
		"sample@",
		"placeholder@",
		"user@",
		"admin@example",
		"info@example",
		"contact@example",
		"john.doe@",
		"jane.doe@",
		"johndoe@",
		"janedoe@",
	}

	emailLower := strings.ToLower(email)

	// Check domains
	for _, domain := range placeholderDomains {
		if strings.HasSuffix(emailLower, "@"+domain) {
			return true
		}
	}

	// Check names
	for _, name := range placeholderNames {
		if strings.HasPrefix(emailLower, name) {
			return true
		}
	}

	return false
}

// normalizePhone removes formatting from phone numbers for comparison.
func (c *ComplianceChecker) normalizePhone(phone string) string {
	re := regexp.MustCompile(`[^0-9]`)
	digits := re.ReplaceAllString(phone, "")
	// Handle country code
	if len(digits) == 11 && digits[0] == '1' {
		digits = digits[1:]
	}
	return digits
}

// checkExcludedServices verifies that excluded services are not offered.
func (c *ComplianceChecker) checkExcludedServices(response string, config *AudienceConfig, result *ComplianceResult) {
	if len(config.Exclusions) == 0 {
		return
	}

	responseLower := strings.ToLower(response)

	// Phrases that indicate offering a service
	offerIndicators := []string{
		"we can", "we will", "we offer", "we provide", "we handle",
		"we'll", "we do", "happy to help with", "assist with",
		"take care of", "help you with",
	}

	for _, exc := range config.Exclusions {
		excLower := strings.ToLower(exc.Name)

		// Check if the excluded service is mentioned in an offering context
		for _, indicator := range offerIndicators {
			pattern := indicator + `[^.]*` + regexp.QuoteMeta(excLower)
			if matched, _ := regexp.MatchString(pattern, responseLower); matched {
				result.Violations = append(result.Violations, ComplianceViolation{
					Type:       "excluded_service_offered",
					Severity:   "critical",
					Content:    exc.Name,
					Suggestion: "This service is explicitly excluded. Do not offer it.",
				})
				result.Score -= 30
				break
			}
		}

		// Also check for direct mention without clear context of not offering
		if strings.Contains(responseLower, excLower) {
			// Check if it's in a negative context (acceptable)
			negativeContexts := []string{
				"don't", "do not", "cannot", "can't", "won't", "will not",
				"not able", "unable", "not offer", "not provide", "not handle",
			}

			hasNegativeContext := false
			for _, neg := range negativeContexts {
				// Check if negative word appears near the excluded service
				idx := strings.Index(responseLower, excLower)
				if idx > 0 {
					// Check 50 chars before
					start := max(0, idx-50)
					context := responseLower[start:idx]
					if strings.Contains(context, neg) {
						hasNegativeContext = true
						break
					}
				}
			}

			if !hasNegativeContext {
				result.Violations = append(result.Violations, ComplianceViolation{
					Type:       "excluded_service_mentioned",
					Severity:   "warning",
					Content:    exc.Name,
					Suggestion: "Excluded service mentioned without clear denial. Clarify we don't offer this.",
				})
				result.Score -= 10
			}
		}
	}
}

// checkPromises looks for unauthorized promises about times or prices.
func (c *ComplianceChecker) checkPromises(response string, result *ComplianceResult) {
	responseLower := strings.ToLower(response)

	// Time promise patterns
	timePromises := []string{
		`within \d+ (minute|hour|day)`,
		`in \d+ (minute|hour|day)`,
		`(arrive|be there|come) (in|within) \d+`,
		`guaranteed (within|in) \d+`,
		`no more than \d+ (minute|hour|day)`,
	}

	for _, pattern := range timePromises {
		if matched, _ := regexp.MatchString(pattern, responseLower); matched {
			re := regexp.MustCompile(pattern)
			match := re.FindString(responseLower)
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:       "time_promise",
				Severity:   "warning",
				Content:    match,
				Suggestion: "Avoid specific time commitments unless documented in KB",
			})
			result.Score -= 5
		}
	}

	// Price promise patterns
	pricePromises := []string{
		`\$\d+`,
		`costs? (only|just|around|about) \d+`,
		`(price|fee|rate) (is|of|at) \d+`,
		`free (of charge|estimate|consultation|quote)`,
	}

	for _, pattern := range pricePromises {
		if matched, _ := regexp.MatchString(pattern, responseLower); matched {
			re := regexp.MustCompile(pattern)
			match := re.FindString(responseLower)

			// "free estimate" and "free consultation" are usually OK
			if strings.Contains(match, "free estimate") || strings.Contains(match, "free consultation") || strings.Contains(match, "free quote") {
				continue
			}

			result.Violations = append(result.Violations, ComplianceViolation{
				Type:       "price_promise",
				Severity:   "warning",
				Content:    match,
				Suggestion: "Avoid specific price commitments unless documented in KB",
			})
			result.Score -= 5
		}
	}
}

// hasCriticalViolation checks if there are any critical violations.
func (c *ComplianceChecker) hasCriticalViolation(violations []ComplianceViolation) bool {
	for _, v := range violations {
		if v.Severity == "critical" {
			return true
		}
	}
	return false
}

// DefaultComplianceChecker is a package-level checker instance.
var DefaultComplianceChecker = NewComplianceChecker()

// VerifyResponseCompliance is a convenience function using the default checker.
func VerifyResponseCompliance(ctx context.Context, response string, config *AudienceConfig) *ComplianceResult {
	return DefaultComplianceChecker.VerifyCompliance(ctx, response, config)
}
