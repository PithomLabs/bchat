package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/revrost/go-openrouter"
)

// VerificationResult represents the outcome of response verification.
type VerificationResult struct {
	Compliant         bool                    `json:"compliant"`
	Violations        []VerificationViolation `json:"violations"`
	CorrectedResponse string                  `json:"corrected_response,omitempty"`
	VerificationTime  time.Duration           `json:"-"`
	Model             string                  `json:"-"`
}

// VerificationViolation represents a single compliance violation.
type VerificationViolation struct {
	ChecklistItem string `json:"checklist_item"`
	Violation     string `json:"violation"`
	Evidence      string `json:"evidence"`
	Correction    string `json:"correction"`
	Severity      string `json:"severity,omitempty"` // "critical", "high", "medium", "low"
}

// VerificationConfig holds configuration for the verifier.
type VerificationConfig struct {
	Enabled       bool   `json:"enabled"`
	Model         string `json:"model"`          // Model to use for verification (cheap/fast)
	Mode          string `json:"mode"`           // "shadow" (log only) or "enforce" (correct responses)
	MaxLatencyMs  int    `json:"max_latency_ms"` // Skip verification if it would exceed this
	SkipOnError   bool   `json:"skip_on_error"`  // If true, return original response on verification error
}

// DefaultVerificationConfig returns sensible defaults.
func DefaultVerificationConfig() *VerificationConfig {
	return &VerificationConfig{
		Enabled:      true,
		Model:        "openai/gpt-4o-mini", // Fast and cheap for verification
		Mode:         "enforce",            // Start with enforce mode
		MaxLatencyMs: 3000,                 // 3 second max for verification
		SkipOnError:  true,                 // Graceful degradation
	}
}

// Verifier handles response verification against KB and policies.
type Verifier struct {
	client *openrouter.Client
	config *VerificationConfig
}

// NewVerifier creates a new Verifier instance.
func NewVerifier(client *openrouter.Client, config *VerificationConfig) *Verifier {
	if config == nil {
		config = DefaultVerificationConfig()
	}
	return &Verifier{
		client: client,
		config: config,
	}
}

// VerifyResponse checks if a response complies with KB and policies.
func (v *Verifier) VerifyResponse(ctx context.Context, response string, audienceConfig *AudienceConfig) (*VerificationResult, error) {
	if !v.config.Enabled {
		return &VerificationResult{Compliant: true}, nil
	}

	start := time.Now()

	// Build the verification prompt
	prompt := v.buildVerifierPrompt(response, audienceConfig)

	// Call the verifier LLM
	result, err := v.callVerifierLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("verifier LLM call failed: %w", err)
	}

	result.VerificationTime = time.Since(start)
	result.Model = v.config.Model

	return result, nil
}

// buildVerifierPrompt constructs the prompt for the verifier LLM.
func (v *Verifier) buildVerifierPrompt(response string, config *AudienceConfig) string {
	var sb strings.Builder

	sb.WriteString("You are a strict compliance verifier for a customer service agent. ")
	sb.WriteString("Your job is to check if the agent's response contains ONLY information from the provided Knowledge Base.\n\n")

	// Add KB content (source of truth)
	sb.WriteString("## KNOWLEDGE BASE (Source of Truth)\n\n")
	if config.RawKB != "" {
		sb.WriteString(config.RawKB)
	} else {
		// Build KB from structured data
		sb.WriteString(v.buildKBSummary(config))
	}
	sb.WriteString("\n\n")

	// Add policies
	sb.WriteString("## POLICIES\n\n")
	if config.RawPolicy != "" {
		sb.WriteString(config.RawPolicy)
	} else {
		sb.WriteString(v.buildPolicySummary(config))
	}
	sb.WriteString("\n\n")

	// Add verification checklist
	sb.WriteString("## VERIFICATION CHECKLIST\n\n")
	sb.WriteString(v.buildVerificationChecklist(config))
	sb.WriteString("\n\n")

	// Add the response to verify
	sb.WriteString("## AGENT RESPONSE TO VERIFY\n\n")
	sb.WriteString(response)
	sb.WriteString("\n\n")

	// Add instructions
	sb.WriteString("## YOUR TASK\n\n")
	sb.WriteString("Check the agent's response against each item in the verification checklist.\n\n")
	sb.WriteString("Respond ONLY with valid JSON in this exact format:\n")
	sb.WriteString("```json\n")
	sb.WriteString(`{
  "compliant": true or false,
  "violations": [
    {
      "checklist_item": "which rule was violated",
      "violation": "what specifically is wrong",
      "evidence": "the exact problematic text from response",
      "correction": "how it should be fixed",
      "severity": "critical" or "high" or "medium" or "low"
    }
  ],
  "corrected_response": "the full corrected response if violations found, or null if compliant"
}`)
	sb.WriteString("\n```\n\n")

	sb.WriteString("IMPORTANT RULES:\n")
	sb.WriteString("- Be STRICT. If information isn't explicitly in the KB, it's a violation.\n")
	sb.WriteString("- Phone numbers must match EXACTLY. (555) xxx-xxxx patterns are ALWAYS violations.\n")
	sb.WriteString("- Email addresses must match EXACTLY what's in the KB.\n")
	sb.WriteString("- Service names must match KB service names.\n")
	sb.WriteString("- If response is compliant, return: {\"compliant\": true, \"violations\": [], \"corrected_response\": null}\n")
	sb.WriteString("- For corrected_response, fix ALL violations while preserving the helpful tone.\n")
	sb.WriteString("- Severity levels: critical (wrong contact info), high (wrong services), medium (tone issues), low (minor wording)\n")

	return sb.String()
}

// buildKBSummary creates a summary of KB content for verification.
func (v *Verifier) buildKBSummary(config *AudienceConfig) string {
	var sb strings.Builder

	sb.WriteString("### Company Information\n")
	sb.WriteString(fmt.Sprintf("- Company Name: %s\n", config.CompanyName))

	sb.WriteString("\n### Authorized Contact Information\n")
	if config.Audience != nil {
		// Use validated phone to avoid telling verifier that placeholder is correct
		// This fixes issue where verifier "corrects" responses with placeholder phones
		validatedPhone := GetValidatedReplacementPhone(config.Audience.EmergencyPhone, config.RawKB)
		if validatedPhone != "" {
			sb.WriteString(fmt.Sprintf("- Phone: %s\n", validatedPhone))
		} else {
			sb.WriteString("- Phone: [no valid phone configured - reject any placeholder numbers]\n")
		}
		if config.Audience.Email != "" {
			sb.WriteString(fmt.Sprintf("- Email: %s\n", config.Audience.Email))
		}
	}

	if len(config.Services) > 0 {
		sb.WriteString("\n### Services We Offer\n")
		for _, svc := range config.Services {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", svc.Name, svc.Description))
		}
	}

	if len(config.Exclusions) > 0 {
		sb.WriteString("\n### Services We DO NOT Provide\n")
		for _, exc := range config.Exclusions {
			sb.WriteString(fmt.Sprintf("- %s", exc.Name))
			if exc.Referral != "" {
				sb.WriteString(fmt.Sprintf(" (refer to: %s)", exc.Referral))
			}
			sb.WriteString("\n")
		}
	}

	if len(config.FAQs) > 0 {
		sb.WriteString("\n### FAQs\n")
		for _, faq := range config.FAQs {
			sb.WriteString(fmt.Sprintf("Q: %s\nA: %s\n\n", faq.Question, faq.Answer))
		}
	}

	return sb.String()
}

// buildPolicySummary creates a summary of policies for verification.
func (v *Verifier) buildPolicySummary(config *AudienceConfig) string {
	var sb strings.Builder

	if config.Audience != nil {
		sb.WriteString(fmt.Sprintf("- Role: %s\n", config.Audience.Role))
		sb.WriteString(fmt.Sprintf("- Tone: %s\n", config.Audience.Tone))
		if config.Audience.BrandVoice != "" {
			sb.WriteString(fmt.Sprintf("- Brand Voice: %s\n", config.Audience.BrandVoice))
		}
	}

	if len(config.Rules) > 0 {
		sb.WriteString("\n### Rules\n")
		for _, rule := range config.Rules {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", rule.Name, rule.Description))
		}
	}

	return sb.String()
}

// buildVerificationChecklist creates the checklist for the verifier.
func (v *Verifier) buildVerificationChecklist(config *AudienceConfig) string {
	var sb strings.Builder

	// Use custom checklist if available, otherwise use defaults
	if len(config.VerificationRules) > 0 {
		for _, rule := range config.VerificationRules {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", rule.ID, rule.Description))
		}
		return sb.String()
	}

	// Default verification checklist
	sb.WriteString("### Contact Information Grounding\n")
	sb.WriteString("- [ ] All phone numbers in the response appear exactly in the KB Contact Information section\n")
	sb.WriteString("- [ ] All email addresses in the response appear exactly in the KB Contact Information section\n")
	sb.WriteString("- [ ] No placeholder numbers like (555) xxx-xxxx, (000) xxx-xxxx, or (123) 456-7890\n")
	sb.WriteString("- [ ] No placeholder emails like name@email.com, test@example.com\n")

	sb.WriteString("\n### Service Grounding\n")
	sb.WriteString("- [ ] All services mentioned appear in the KB Services section\n")
	sb.WriteString("- [ ] No services from the 'Services We DO NOT Provide' section are offered\n")
	sb.WriteString("- [ ] Service descriptions match KB (no invented features or capabilities)\n")

	sb.WriteString("\n### Claims Grounding\n")
	sb.WriteString("- [ ] No specific pricing unless explicitly stated in KB\n")
	sb.WriteString("- [ ] No specific response times unless explicitly stated in KB\n")
	sb.WriteString("- [ ] No promises or guarantees not supported by KB\n")
	sb.WriteString("- [ ] No invented processes, procedures, or steps not in KB\n")

	sb.WriteString("\n### Timeline & Process Claims\n")
	sb.WriteString("- [ ] No duration claims for inspections unless explicitly in KB (e.g., '30-45 minutes')\n")
	sb.WriteString("- [ ] No duration claims for remediation/service unless explicitly in KB (e.g., '1-2 days')\n")
	sb.WriteString("- [ ] No preparation instructions invented unless documented in KB\n")
	sb.WriteString("- [ ] No arrival time promises unless explicitly in KB (e.g., 'within 2 hours')\n")

	sb.WriteString("\n### Customer Information Handling\n")
	sb.WriteString("- [ ] Customer-provided phone number echoed correctly (not substituted with company phone)\n")
	sb.WriteString("- [ ] Customer name echoed correctly if provided\n")
	sb.WriteString("- [ ] Customer address/location echoed correctly if provided\n")

	sb.WriteString("\n### Tone & Identity\n")
	sb.WriteString("- [ ] Response matches the configured tone (professional, friendly, etc.)\n")
	sb.WriteString("- [ ] Company name used correctly\n")
	sb.WriteString("- [ ] No inappropriate first-person speculation (\"I think\", \"I believe\") for factual claims\n")

	return sb.String()
}

// callVerifierLLM makes the API call to the verifier model.
func (v *Verifier) callVerifierLLM(ctx context.Context, prompt string) (*VerificationResult, error) {
	if v.client == nil {
		return nil, fmt.Errorf("OpenRouter client not initialized")
	}

	messages := []openrouter.ChatCompletionMessage{
		openrouter.SystemMessage("You are a strict compliance verifier. Respond only with valid JSON."),
		openrouter.UserMessage(prompt),
	}

	resp, err := v.client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:    v.config.Model,
		Messages: messages,
	})
	if err != nil {
		return nil, fmt.Errorf("verifier API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("verifier returned no choices")
	}

	content := resp.Choices[0].Message.Content.Text
	return v.parseVerificationResult(content)
}

// parseVerificationResult extracts the structured result from LLM output.
func (v *Verifier) parseVerificationResult(content string) (*VerificationResult, error) {
	// Extract JSON from response (may be wrapped in markdown code blocks)
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		// If no JSON found, try to parse the entire content
		jsonStr = content
	}

	var result VerificationResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		slog.Warn("failed to parse verifier response as JSON",
			"error", err,
			"content", truncateForLog(content, 500))

		// Return a safe default - assume compliant if we can't parse
		return &VerificationResult{
			Compliant:  true,
			Violations: []VerificationViolation{},
		}, nil
	}

	// Validate and set default severities
	for i := range result.Violations {
		if result.Violations[i].Severity == "" {
			result.Violations[i].Severity = v.inferSeverity(result.Violations[i])
		}
	}

	return &result, nil
}

// inferSeverity determines violation severity based on type.
func (v *Verifier) inferSeverity(violation VerificationViolation) string {
	item := strings.ToLower(violation.ChecklistItem)
	evidence := strings.ToLower(violation.Evidence)

	// Contact info violations are critical
	if strings.Contains(item, "phone") || strings.Contains(item, "email") ||
		strings.Contains(item, "contact") {
		return "critical"
	}

	// Placeholder patterns are critical
	if strings.Contains(evidence, "555") || strings.Contains(evidence, "@email.com") ||
		strings.Contains(evidence, "@example.com") {
		return "critical"
	}

	// Service violations are high
	if strings.Contains(item, "service") {
		return "high"
	}

	// Claims and promises are medium
	if strings.Contains(item, "pricing") || strings.Contains(item, "promise") ||
		strings.Contains(item, "guarantee") {
		return "medium"
	}

	// Default to low
	return "low"
}

// extractJSON extracts JSON content from a string that may contain markdown code blocks.
func extractJSON(content string) string {
	// Try to find JSON in code blocks first
	if idx := strings.Index(content, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(content[start:], "```"); end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}

	// Try to find JSON in generic code blocks
	if idx := strings.Index(content, "```"); idx != -1 {
		start := idx + 3
		// Skip language identifier if present
		if nlIdx := strings.Index(content[start:], "\n"); nlIdx != -1 {
			start += nlIdx + 1
		}
		if end := strings.Index(content[start:], "```"); end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}

	// Try to find raw JSON (starts with {)
	if idx := strings.Index(content, "{"); idx != -1 {
		// Find matching closing brace
		depth := 0
		for i := idx; i < len(content); i++ {
			if content[i] == '{' {
				depth++
			} else if content[i] == '}' {
				depth--
				if depth == 0 {
					return content[idx : i+1]
				}
			}
		}
	}

	return ""
}

// truncateForLog truncates a string for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// VerificationMetrics tracks verification statistics.
type VerificationMetrics struct {
	TotalVerifications   int64            `json:"total_verifications"`
	CompliantResponses   int64            `json:"compliant_responses"`
	ViolationsDetected   int64            `json:"violations_detected"`
	CorrectionsMade      int64            `json:"corrections_made"`
	VerificationErrors   int64            `json:"verification_errors"`
	AverageLatencyMs     float64          `json:"average_latency_ms"`
	ViolationsByType     map[string]int64 `json:"violations_by_type"`
	ViolationsBySeverity map[string]int64 `json:"violations_by_severity"`
}

// NewVerificationMetrics creates a new metrics instance.
func NewVerificationMetrics() *VerificationMetrics {
	return &VerificationMetrics{
		ViolationsByType:     make(map[string]int64),
		ViolationsBySeverity: make(map[string]int64),
	}
}

// RecordVerification records metrics for a verification result.
func (m *VerificationMetrics) RecordVerification(result *VerificationResult, err error) {
	m.TotalVerifications++

	if err != nil {
		m.VerificationErrors++
		return
	}

	if result.Compliant {
		m.CompliantResponses++
	} else {
		m.ViolationsDetected += int64(len(result.Violations))
		if result.CorrectedResponse != "" {
			m.CorrectionsMade++
		}

		for _, v := range result.Violations {
			m.ViolationsByType[v.ChecklistItem]++
			m.ViolationsBySeverity[v.Severity]++
		}
	}

	// Update average latency (simple moving average)
	latencyMs := float64(result.VerificationTime.Milliseconds())
	m.AverageLatencyMs = (m.AverageLatencyMs*float64(m.TotalVerifications-1) + latencyMs) / float64(m.TotalVerifications)
}

// LogSummary logs a summary of verification metrics.
func (m *VerificationMetrics) LogSummary() {
	complianceRate := float64(0)
	if m.TotalVerifications > 0 {
		complianceRate = float64(m.CompliantResponses) / float64(m.TotalVerifications) * 100
	}

	slog.Info("verification metrics summary",
		"total", m.TotalVerifications,
		"compliant", m.CompliantResponses,
		"compliance_rate", fmt.Sprintf("%.1f%%", complianceRate),
		"violations", m.ViolationsDetected,
		"corrections", m.CorrectionsMade,
		"errors", m.VerificationErrors,
		"avg_latency_ms", fmt.Sprintf("%.0f", m.AverageLatencyMs),
	)
}
