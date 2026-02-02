package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/usememos/memos/store"
)

// Parser handles parsing of KB.MD and POLICY.MD files with HTML comment annotations.
type Parser struct{}

// NewParser creates a new Parser instance.
func NewParser() *Parser {
	return &Parser{}
}

// ParsedKB represents the parsed knowledge base.
type ParsedKB struct {
	CompanyName string
	Services    []*store.AgentService
	Exclusions  []*store.AgentExclusion
	Coverage    []*store.AgentCoverage
	FAQs        []*store.AgentFAQ
	Safety      []*store.AgentSafetyProtocol
	Sections    []*store.AgentKBSection
}

// ParsedPolicy represents the parsed policy document.
type ParsedPolicy struct {
	Identity *ParsedIdentity
	Intents  []*store.AgentIntent
	Rules    []*store.AgentRule
	Audience *store.AgentAudience
}

// ParseKBResult wraps ParsedKB with parsing metadata.
type ParseKBResult struct {
	KB           *ParsedKB
	IsStructured bool // true if meaningful structured content was found
	ParsedCount  int  // total items parsed (services + faqs + exclusions + coverage + safety + sections)
}

// ParsePolicyResult wraps ParsedPolicy with parsing metadata.
type ParsePolicyResult struct {
	Policy       *ParsedPolicy
	IsStructured bool // true if meaningful structured content was found
	ParsedCount  int  // total items parsed (intents + rules)
}

// StructuredContentThreshold is the minimum number of parsed items
// to consider content as "structured" (vs unstructured prose).
const StructuredContentThreshold = 2

// ParsedIdentity represents the identity section from a policy file.
type ParsedIdentity struct {
	Role       string
	Tone       string
	BrandVoice string
	Guidelines []string
}

// ContentHash generates a SHA256 hash of the content.
func ContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// annotationBlock represents a parsed annotation block
type annotationBlock struct {
	annotationType string            // e.g., "service", "exclusion", "faq"
	params         map[string]string // parsed parameters
	title          string            // the heading/title line after annotation
	content        string            // the content until next annotation or section
}

// extractAnnotationBlocks finds all <!-- @type: params --> blocks and their content
func extractAnnotationBlocks(content string) []annotationBlock {
	var blocks []annotationBlock

	// Find all annotation positions
	annotationPattern := regexp.MustCompile(`<!--\s*@(\w+)(?::\s*([^>]*))?\s*-->`)
	matches := annotationPattern.FindAllStringSubmatchIndex(content, -1)

	for i, match := range matches {
		if len(match) < 6 {
			continue
		}

		annotationType := content[match[2]:match[3]]
		params := ""
		if match[4] >= 0 && match[5] >= 0 {
			params = content[match[4]:match[5]]
		}

		// Find where this block ends (next annotation or section marker or end)
		blockStart := match[1] // End of annotation comment
		blockEnd := len(content)
		if i+1 < len(matches) {
			blockEnd = matches[i+1][0]
		}

		// Also check for section separators (---)
		blockContent := content[blockStart:blockEnd]
		if sepIdx := strings.Index(blockContent, "\n---"); sepIdx >= 0 {
			blockEnd = blockStart + sepIdx
			blockContent = content[blockStart:blockEnd]
		}

		// Also check for next ## header that's not part of our content
		lines := strings.Split(blockContent, "\n")
		for j, line := range lines {
			if j > 2 && (strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ")) {
				blockEnd = blockStart + strings.Index(blockContent, "\n"+line)
				blockContent = content[blockStart:blockEnd]
				break
			}
		}

		// Extract title (first non-empty line, often a heading)
		title := ""
		contentStart := 0
		for _, line := range strings.Split(strings.TrimSpace(blockContent), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				title = strings.TrimPrefix(line, "### ")
				title = strings.TrimPrefix(title, "## ")
				title = strings.TrimPrefix(title, "#### ")
				contentStart = strings.Index(blockContent, line) + len(line)
				break
			}
		}

		actualContent := ""
		if contentStart < len(blockContent) {
			actualContent = strings.TrimSpace(blockContent[contentStart:])
		}

		blocks = append(blocks, annotationBlock{
			annotationType: annotationType,
			params:         parseParams(params),
			title:          title,
			content:        actualContent,
		})
	}

	return blocks
}

// parseParams parses "key: value, key2: value2" format
func parseParams(s string) map[string]string {
	params := make(map[string]string)
	s = strings.TrimSpace(s)
	if s == "" {
		return params
	}

	// First param might not have a key (just the code)
	parts := strings.Split(s, ",")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if idx := strings.Index(part, ":"); idx > 0 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			value = strings.Trim(value, `"`)
			params[key] = value
		} else if i == 0 {
			// First param without key is the "code"
			params["code"] = part
		}
	}

	return params
}

// ParseKB parses a KB.MD file and extracts structured data.
func (p *Parser) ParseKB(content string, tenantID int32, audienceType string) (*ParsedKB, error) {
	result := &ParsedKB{
		Services:   make([]*store.AgentService, 0),
		Exclusions: make([]*store.AgentExclusion, 0),
		Coverage:   make([]*store.AgentCoverage, 0),
		FAQs:       make([]*store.AgentFAQ, 0),
		Safety:     make([]*store.AgentSafetyProtocol, 0),
		Sections:   make([]*store.AgentKBSection, 0),
	}

	// Extract company name from first header
	companyMatch := regexp.MustCompile(`(?m)^#\s+(.+?)(?:\s+Knowledge Base)?$`).FindStringSubmatch(content)
	if len(companyMatch) > 1 {
		result.CompanyName = strings.TrimSpace(companyMatch[1])
	}

	blocks := extractAnnotationBlocks(content)

	for _, block := range blocks {
		switch block.annotationType {
		case "service":
			code := block.params["code"]
			isEmergency := block.params["emergency"] == "true"

			// Extract response time if present
			responseTime := ""
			rtMatch := regexp.MustCompile(`\*\*Response Time:\*\*\s*(.+)`).FindStringSubmatch(block.content)
			if len(rtMatch) > 1 {
				responseTime = strings.TrimSpace(rtMatch[1])
			}

			result.Services = append(result.Services, &store.AgentService{
				TenantID:     tenantID,
				AudienceType: audienceType,
				Code:         code,
				Name:         block.title,
				Description:  block.content,
				IsEmergency:  isEmergency,
				ResponseTime: responseTime,
				IsActive:     true,
			})

		case "exclusion":
			code := block.params["code"]
			exception := block.params["exception"]

			// Extract referral if present
			referral := ""
			refMatch := regexp.MustCompile(`\*\*Recommendation:\*\*\s*(.+)`).FindStringSubmatch(block.content)
			if len(refMatch) > 1 {
				referral = strings.TrimSpace(refMatch[1])
			}

			result.Exclusions = append(result.Exclusions, &store.AgentExclusion{
				TenantID:      tenantID,
				AudienceType:  audienceType,
				Code:          code,
				Name:          block.title,
				Description:   block.content,
				ExceptionRule: exception,
				Referral:      referral,
				IsActive:      true,
			})

		case "coverage":
			isIncluded := block.params["code"] == "include"
			coverageContent := block.content

			// Parse area names from the content
			areaPattern := regexp.MustCompile(`\*\*([^*]+):\*\*\s*\n?([^\n*]+)`)
			for _, areaMatch := range areaPattern.FindAllStringSubmatch(coverageContent, -1) {
				areaType := strings.TrimSpace(areaMatch[1])
				areas := strings.Split(areaMatch[2], ",")
				for _, area := range areas {
					area = strings.TrimSpace(area)
					if area != "" {
						result.Coverage = append(result.Coverage, &store.AgentCoverage{
							TenantID:   tenantID,
							AreaType:   areaType,
							AreaName:   area,
							IsIncluded: isIncluded,
						})
					}
				}
			}

			// Also check for bullet points
			bulletPattern := regexp.MustCompile(`(?m)^-\s+(.+)`)
			for _, bulletMatch := range bulletPattern.FindAllStringSubmatch(coverageContent, -1) {
				area := strings.TrimSpace(bulletMatch[1])
				if area != "" && !strings.Contains(area, ":") {
					result.Coverage = append(result.Coverage, &store.AgentCoverage{
						TenantID:   tenantID,
						AreaType:   "general",
						AreaName:   area,
						IsIncluded: isIncluded,
					})
				}
			}

		case "faq":
			code := block.params["code"]
			result.FAQs = append(result.FAQs, &store.AgentFAQ{
				TenantID:     tenantID,
				AudienceType: audienceType,
				Code:         code,
				Question:     block.title,
				Answer:       block.content,
				IsActive:     true,
			})

		case "safety":
			code := block.params["code"]
			triggersStr := block.params["triggers"]

			// Parse triggers
			var triggers []string
			if triggersStr != "" {
				for _, t := range strings.Split(triggersStr, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						triggers = append(triggers, t)
					}
				}
			}

			// Parse instructions (numbered list items)
			var instructions []string
			instructionPattern := regexp.MustCompile(`(?m)^\d+\.\s+\*\*(.+?)\*\*(?:\s*-\s*(.+))?$`)
			for _, instMatch := range instructionPattern.FindAllStringSubmatch(block.content, -1) {
				instruction := strings.TrimSpace(instMatch[1])
				if instMatch[2] != "" {
					instruction += " - " + strings.TrimSpace(instMatch[2])
				}
				instructions = append(instructions, instruction)
			}

			// If no numbered list, try bullet points
			if len(instructions) == 0 {
				bulletPattern := regexp.MustCompile(`(?m)^-\s+(.+)`)
				for _, bulletMatch := range bulletPattern.FindAllStringSubmatch(block.content, -1) {
					instructions = append(instructions, strings.TrimSpace(bulletMatch[1]))
				}
			}

			result.Safety = append(result.Safety, &store.AgentSafetyProtocol{
				TenantID:       tenantID,
				AudienceType:   audienceType,
				Code:           code,
				Name:           block.title,
				TriggerIntents: triggers,
				Instructions:   instructions,
				IsActive:       true,
			})

		case "section":
			code := block.params["code"]
			sectionType := block.params["type"]
			if sectionType == "" {
				sectionType = "general"
			}

			result.Sections = append(result.Sections, &store.AgentKBSection{
				TenantID:     tenantID,
				AudienceType: audienceType,
				Code:         code,
				Title:        block.title,
				Content:      block.content,
				SectionType:  sectionType,
				IsActive:     true,
			})
		}
	}

	return result, nil
}

// ParseKBWithResult parses a KB.MD file and returns a result with metadata about parsing success.
func (p *Parser) ParseKBWithResult(content string, tenantID int32, audienceType string) (*ParseKBResult, error) {
	kb, err := p.ParseKB(content, tenantID, audienceType)
	if err != nil {
		return nil, err
	}

	// Count total parsed items
	parsedCount := len(kb.Services) + len(kb.FAQs) + len(kb.Exclusions) +
		len(kb.Coverage) + len(kb.Safety) + len(kb.Sections)

	return &ParseKBResult{
		KB:           kb,
		IsStructured: parsedCount >= StructuredContentThreshold,
		ParsedCount:  parsedCount,
	}, nil
}

// ParsePolicy parses a POLICY.MD file and extracts structured data.
func (p *Parser) ParsePolicy(content string, tenantID int32, audienceType string) (*ParsedPolicy, error) {
	result := &ParsedPolicy{
		Identity: &ParsedIdentity{},
		Intents:  make([]*store.AgentIntent, 0),
		Rules:    make([]*store.AgentRule, 0),
	}

	blocks := extractAnnotationBlocks(content)

	for _, block := range blocks {
		switch block.annotationType {
		case "identity":
			identityContent := block.title + "\n" + block.content

			// Extract role
			if roleMatch := regexp.MustCompile(`\*\*Role:\*\*\s*(.+)`).FindStringSubmatch(identityContent); len(roleMatch) > 1 {
				result.Identity.Role = strings.TrimSpace(roleMatch[1])
			}

			// Extract tone
			if toneMatch := regexp.MustCompile(`\*\*Tone:\*\*\s*(.+)`).FindStringSubmatch(identityContent); len(toneMatch) > 1 {
				result.Identity.Tone = strings.TrimSpace(toneMatch[1])
			}

			// Extract brand voice
			if voiceMatch := regexp.MustCompile(`\*\*Brand Voice:\*\*\s*"?(.+?)"?\s*$`).FindStringSubmatch(identityContent); len(voiceMatch) > 1 {
				result.Identity.BrandVoice = strings.Trim(strings.TrimSpace(voiceMatch[1]), `"`)
			}

			// Extract guidelines
			guidelinesPattern := regexp.MustCompile(`(?s)\*\*Guidelines:\*\*\s*\n((?:-[^\n]+\n?)+)`)
			if guidelinesMatch := guidelinesPattern.FindStringSubmatch(identityContent); len(guidelinesMatch) > 1 {
				bulletPattern := regexp.MustCompile(`(?m)^-\s+(.+)`)
				for _, bulletMatch := range bulletPattern.FindAllStringSubmatch(guidelinesMatch[1], -1) {
					result.Identity.Guidelines = append(result.Identity.Guidelines, strings.TrimSpace(bulletMatch[1]))
				}
			}

		case "intent":
			code := block.params["code"]
			category := block.params["category"]
			urgencyStr := block.params["urgency"]
			action := block.params["action"]
			confidenceStr := block.params["confidence_threshold"]

			// Parse urgency
			urgency := 0
			if urgencyStr != "" {
				for _, c := range urgencyStr {
					if c >= '0' && c <= '9' {
						urgency = urgency*10 + int(c-'0')
					}
				}
			}

			// Parse confidence threshold
			confidence := 0.0
			if confidenceStr != "" {
				confidence = parseFloat(confidenceStr)
			}

			// Default action
			if action == "" {
				action = "standard_flow"
			}

			// Extract description
			description := ""
			descPattern := regexp.MustCompile(`(?s)\*\*Description:\*\*\s*\n?(.+?)(?:\n\n|\*\*Examples|\*\*Key|\z)`)
			if descMatch := descPattern.FindStringSubmatch(block.content); len(descMatch) > 1 {
				description = strings.TrimSpace(descMatch[1])
			} else {
				// First paragraph as description
				paragraphs := strings.Split(block.content, "\n\n")
				if len(paragraphs) > 0 {
					description = strings.TrimSpace(paragraphs[0])
				}
			}

			// Extract examples
			var examples []string
			examplesPattern := regexp.MustCompile(`(?s)\*\*Examples that MATCH:\*\*\s*\n((?:-[^\n]+\n?)+)`)
			if examplesMatch := examplesPattern.FindStringSubmatch(block.content); len(examplesMatch) > 1 {
				bulletPattern := regexp.MustCompile(`(?m)^-\s+"?(.+?)"?\s*$`)
				for _, bulletMatch := range bulletPattern.FindAllStringSubmatch(examplesMatch[1], -1) {
					examples = append(examples, strings.Trim(strings.TrimSpace(bulletMatch[1]), `"`))
				}
			}
			// Also try **Examples:**
			if len(examples) == 0 {
				examplesPattern2 := regexp.MustCompile(`(?s)\*\*Examples:\*\*\s*([^*]+)`)
				if examplesMatch := examplesPattern2.FindStringSubmatch(block.content); len(examplesMatch) > 1 {
					for _, ex := range strings.Split(examplesMatch[1], ",") {
						ex = strings.Trim(strings.TrimSpace(ex), `"`)
						if ex != "" {
							examples = append(examples, ex)
						}
					}
				}
			}

			// Extract counter examples
			var counterExamples []string
			counterPattern := regexp.MustCompile(`(?s)\*\*Examples that DO NOT match:\*\*\s*\n((?:-[^\n]+\n?)+)`)
			if counterMatch := counterPattern.FindStringSubmatch(block.content); len(counterMatch) > 1 {
				bulletPattern := regexp.MustCompile(`(?m)^-\s+"?(.+?)"?(?:\s*\([^)]+\))?\s*$`)
				for _, bulletMatch := range bulletPattern.FindAllStringSubmatch(counterMatch[1], -1) {
					counterExamples = append(counterExamples, strings.Trim(strings.TrimSpace(bulletMatch[1]), `"`))
				}
			}

			at := audienceType
			tid := tenantID
			result.Intents = append(result.Intents, &store.AgentIntent{
				TenantID:            &tid,
				AudienceType:        &at,
				Code:                code,
				Name:                block.title,
				Category:            category,
				Description:         description,
				Examples:            examples,
				CounterExamples:     counterExamples,
				Urgency:             urgency,
				Action:              action,
				ConfidenceThreshold: confidence,
				IsActive:            true,
			})

		case "rule":
			code := block.params["code"]
			priorityStr := block.params["priority"]

			// Parse priority
			priority := 5 // default
			if priorityStr != "" {
				priority = 0
				for _, c := range priorityStr {
					if c >= '0' && c <= '9' {
						priority = priority*10 + int(c-'0')
					}
				}
			}

			result.Rules = append(result.Rules, &store.AgentRule{
				TenantID:     tenantID,
				AudienceType: audienceType,
				Code:         code,
				Name:         block.title,
				Description:  block.content,
				Priority:     priority,
				IsActive:     true,
			})

		case "thresholds":
			thresholdsContent := block.title + "\n" + block.content

			// Extract emergency urgency threshold
			emergencyThreshold := 4 // default
			if etMatch := regexp.MustCompile(`Emergency Urgency\s*\|\s*>=?\s*(\d+)`).FindStringSubmatch(thresholdsContent); len(etMatch) > 1 {
				emergencyThreshold = 0
				for _, c := range etMatch[1] {
					emergencyThreshold = emergencyThreshold*10 + int(c-'0')
				}
			}

			// Extract escalation confidence threshold
			escalationThreshold := 0.85 // default
			if ecMatch := regexp.MustCompile(`Escalation Confidence\s*\|\s*>=?\s*([\d.]+)`).FindStringSubmatch(thresholdsContent); len(ecMatch) > 1 {
				escalationThreshold = parseFloat(ecMatch[1])
			}

			result.Audience = &store.AgentAudience{
				TenantID:                      tenantID,
				AudienceType:                  audienceType,
				Role:                          result.Identity.Role,
				Tone:                          result.Identity.Tone,
				BrandVoice:                    result.Identity.BrandVoice,
				Guidelines:                    result.Identity.Guidelines,
				EmergencyUrgencyThreshold:     emergencyThreshold,
				EscalationConfidenceThreshold: escalationThreshold,
				RateLimitRPM:                  60, // default
			}
		}
	}

	return result, nil
}

// ParsePolicyWithResult parses a POLICY.MD file and returns a result with metadata about parsing success.
func (p *Parser) ParsePolicyWithResult(content string, tenantID int32, audienceType string) (*ParsePolicyResult, error) {
	policy, err := p.ParsePolicy(content, tenantID, audienceType)
	if err != nil {
		return nil, err
	}

	// Count total parsed items
	parsedCount := len(policy.Intents) + len(policy.Rules)

	// Also check if identity was extracted (role/tone)
	hasIdentity := policy.Identity != nil && (policy.Identity.Role != "" || policy.Identity.Tone != "")
	if hasIdentity {
		parsedCount++ // Count identity as one item
	}

	return &ParsePolicyResult{
		Policy:       policy,
		IsStructured: parsedCount >= StructuredContentThreshold,
		ParsedCount:  parsedCount,
	}, nil
}

// parseFloat is a simple float parser for threshold values.
func parseFloat(s string) float64 {
	result := 0.0
	decimal := false
	decimalPlace := 0.1
	for _, c := range s {
		if c == '.' {
			decimal = true
			continue
		}
		if c < '0' || c > '9' {
			continue
		}
		if decimal {
			result += float64(c-'0') * decimalPlace
			decimalPlace *= 0.1
		} else {
			result = result*10 + float64(c-'0')
		}
	}
	return result
}

// ExportKB exports the parsed KB data back to markdown format.
func (p *Parser) ExportKB(kb *ParsedKB) string {
	var sb strings.Builder

	sb.WriteString("# " + kb.CompanyName + " Knowledge Base\n\n")
	sb.WriteString("---\n\n")

	// Services
	if len(kb.Services) > 0 {
		sb.WriteString("## Services\n\n")
		for _, s := range kb.Services {
			emergency := "false"
			if s.IsEmergency {
				emergency = "true"
			}
			sb.WriteString("### " + s.Name + "\n")
			sb.WriteString("<!-- @service: " + s.Code + ", emergency: " + emergency + " -->\n\n")
			sb.WriteString(s.Description + "\n\n")
			if s.ResponseTime != "" {
				sb.WriteString("**Response Time:** " + s.ResponseTime + "\n\n")
			}
			sb.WriteString("---\n\n")
		}
	}

	// Exclusions
	if len(kb.Exclusions) > 0 {
		sb.WriteString("## Services We Don't Provide\n\n")
		for _, e := range kb.Exclusions {
			sb.WriteString("### " + e.Name + "\n")
			annotation := "<!-- @exclusion: " + e.Code
			if e.ExceptionRule != "" {
				annotation += ", exception: \"" + e.ExceptionRule + "\""
			}
			annotation += " -->\n\n"
			sb.WriteString(annotation)
			sb.WriteString(e.Description + "\n\n")
			if e.Referral != "" {
				sb.WriteString("**Recommendation:** " + e.Referral + "\n\n")
			}
			sb.WriteString("---\n\n")
		}
	}

	// Coverage
	includedCoverage := make([]*store.AgentCoverage, 0)
	excludedCoverage := make([]*store.AgentCoverage, 0)
	for _, c := range kb.Coverage {
		if c.IsIncluded {
			includedCoverage = append(includedCoverage, c)
		} else {
			excludedCoverage = append(excludedCoverage, c)
		}
	}

	if len(includedCoverage) > 0 || len(excludedCoverage) > 0 {
		sb.WriteString("## Service Areas\n\n")

		if len(includedCoverage) > 0 {
			sb.WriteString("### Areas We Serve\n")
			sb.WriteString("<!-- @coverage: include -->\n\n")
			// Group by area type
			byType := make(map[string][]string)
			for _, c := range includedCoverage {
				byType[c.AreaType] = append(byType[c.AreaType], c.AreaName)
			}
			for areaType, areas := range byType {
				sb.WriteString("**" + areaType + ":**\n")
				sb.WriteString(strings.Join(areas, ", ") + "\n\n")
			}
			sb.WriteString("---\n\n")
		}

		if len(excludedCoverage) > 0 {
			sb.WriteString("### Areas Outside Our Coverage\n")
			sb.WriteString("<!-- @coverage: exclude -->\n\n")
			sb.WriteString("We do not currently service:\n")
			for _, c := range excludedCoverage {
				sb.WriteString("- " + c.AreaName + "\n")
			}
			sb.WriteString("\n---\n\n")
		}
	}

	// FAQs
	if len(kb.FAQs) > 0 {
		sb.WriteString("## Frequently Asked Questions\n\n")
		for _, f := range kb.FAQs {
			sb.WriteString("### " + f.Question + "\n")
			sb.WriteString("<!-- @faq: " + f.Code + " -->\n\n")
			sb.WriteString(f.Answer + "\n\n")
			sb.WriteString("---\n\n")
		}
	}

	// Safety Protocols
	if len(kb.Safety) > 0 {
		sb.WriteString("## Emergency Safety Procedures\n\n")
		for _, s := range kb.Safety {
			sb.WriteString("### " + s.Name + "\n")
			triggers := ""
			if len(s.TriggerIntents) > 0 {
				triggers = ", triggers: " + strings.Join(s.TriggerIntents, ", ")
			}
			sb.WriteString("<!-- @safety: " + s.Code + triggers + " -->\n\n")
			for i, inst := range s.Instructions {
				sb.WriteString(string(rune('1'+i)) + ". **" + inst + "**\n")
			}
			sb.WriteString("\n---\n\n")
		}
	}

	return sb.String()
}

// ExportPolicy exports the parsed policy data back to markdown format.
func (p *Parser) ExportPolicy(policy *ParsedPolicy) string {
	var sb strings.Builder

	sb.WriteString("# Agent Policy\n\n")
	sb.WriteString("---\n\n")

	// Identity
	sb.WriteString("## Identity\n")
	sb.WriteString("<!-- @identity -->\n\n")
	sb.WriteString("- **Role:** " + policy.Identity.Role + "\n")
	sb.WriteString("- **Tone:** " + policy.Identity.Tone + "\n")
	sb.WriteString("- **Brand Voice:** \"" + policy.Identity.BrandVoice + "\"\n\n")
	if len(policy.Identity.Guidelines) > 0 {
		sb.WriteString("**Guidelines:**\n")
		for _, g := range policy.Identity.Guidelines {
			sb.WriteString("- " + g + "\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("---\n\n")

	// Intents by category
	intentsByCategory := make(map[string][]*store.AgentIntent)
	for _, i := range policy.Intents {
		intentsByCategory[i.Category] = append(intentsByCategory[i.Category], i)
	}

	sb.WriteString("## Intents\n\n")

	categoryOrder := []string{"emergency", "standard", "meta"}
	for _, category := range categoryOrder {
		intents, ok := intentsByCategory[category]
		if !ok || len(intents) == 0 {
			continue
		}

		sb.WriteString("### " + strings.Title(category) + " Intents\n\n")
		for _, i := range intents {
			sb.WriteString("#### " + i.Name + "\n")

			annotation := "<!-- @intent: " + i.Code + ", category: " + i.Category
			if i.Urgency > 0 {
				annotation += ", urgency: " + string(rune('0'+i.Urgency))
			}
			annotation += ", action: " + i.Action
			if i.ConfidenceThreshold > 0 {
				annotation += ", confidence_threshold: " + formatFloat(i.ConfidenceThreshold)
			}
			annotation += " -->\n\n"
			sb.WriteString(annotation)

			sb.WriteString("**Description:**\n" + i.Description + "\n\n")

			if len(i.Examples) > 0 {
				sb.WriteString("**Examples that MATCH:**\n")
				for _, ex := range i.Examples {
					sb.WriteString("- \"" + ex + "\"\n")
				}
				sb.WriteString("\n")
			}

			if len(i.CounterExamples) > 0 {
				sb.WriteString("**Examples that DO NOT match:**\n")
				for _, ex := range i.CounterExamples {
					sb.WriteString("- \"" + ex + "\"\n")
				}
				sb.WriteString("\n")
			}

			sb.WriteString("---\n\n")
		}
	}

	// Rules
	if len(policy.Rules) > 0 {
		sb.WriteString("## Rules\n\n")
		for _, r := range policy.Rules {
			sb.WriteString("### " + r.Name + "\n")
			sb.WriteString("<!-- @rule: " + r.Code + ", priority: " + string(rune('0'+r.Priority)) + " -->\n\n")
			sb.WriteString(r.Description + "\n\n")
			sb.WriteString("---\n\n")
		}
	}

	// Thresholds
	if policy.Audience != nil {
		sb.WriteString("## Thresholds\n")
		sb.WriteString("<!-- @thresholds -->\n\n")
		sb.WriteString("| Metric | Value | Description |\n")
		sb.WriteString("|--------|-------|-------------|\n")
		sb.WriteString("| Emergency Urgency | >= " + string(rune('0'+policy.Audience.EmergencyUrgencyThreshold)) + " | Route to emergency flow |\n")
		sb.WriteString("| Escalation Confidence | >= " + formatFloat(policy.Audience.EscalationConfidenceThreshold) + " | Confirm escalation |\n")
		sb.WriteString("\n---\n\n")
	}

	return sb.String()
}

func formatFloat(f float64) string {
	s := ""
	intPart := int(f)
	decPart := f - float64(intPart)

	if intPart == 0 {
		s = "0"
	} else {
		s = string(rune('0' + intPart))
	}

	if decPart > 0 {
		s += "."
		for i := 0; i < 2; i++ {
			decPart *= 10
			digit := int(decPart)
			s += string(rune('0' + digit))
			decPart -= float64(digit)
		}
	}

	return s
}

// ValidationResult holds validation errors and warnings.
type ValidationResult struct {
	Errors   []string
	Warnings []string
}

// IsValid returns true if there are no errors.
func (v *ValidationResult) IsValid() bool {
	return len(v.Errors) == 0
}

// ToJSON returns the validation result as JSON.
func (v *ValidationResult) ToJSON() string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ValidateKB validates the parsed KB for completeness.
func (p *Parser) ValidateKB(kb *ParsedKB) *ValidationResult {
	result := &ValidationResult{
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	if kb.CompanyName == "" {
		result.Errors = append(result.Errors, "Company name is required (# Company Name header)")
	}

	if len(kb.Services) == 0 {
		result.Warnings = append(result.Warnings, "No services defined")
	}

	// Check for duplicate service codes
	serviceCodes := make(map[string]bool)
	for _, s := range kb.Services {
		if serviceCodes[s.Code] {
			result.Errors = append(result.Errors, "Duplicate service code: "+s.Code)
		}
		serviceCodes[s.Code] = true
	}

	return result
}

// ValidatePolicy validates the parsed policy for completeness.
func (p *Parser) ValidatePolicy(policy *ParsedPolicy) *ValidationResult {
	result := &ValidationResult{
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	if policy.Identity.Role == "" {
		result.Errors = append(result.Errors, "Role is required in identity section")
	}

	if policy.Identity.Tone == "" {
		result.Errors = append(result.Errors, "Tone is required in identity section")
	}

	if len(policy.Intents) == 0 {
		result.Warnings = append(result.Warnings, "No intents defined")
	}

	// Check for intents without examples
	for _, i := range policy.Intents {
		if len(i.Examples) == 0 {
			result.Warnings = append(result.Warnings, "Intent '"+i.Code+"' has no examples")
		}
	}

	// Check for duplicate intent codes
	intentCodes := make(map[string]bool)
	for _, i := range policy.Intents {
		if intentCodes[i.Code] {
			result.Errors = append(result.Errors, "Duplicate intent code: "+i.Code)
		}
		intentCodes[i.Code] = true
	}

	return result
}

// ============================================================================
// SCRIPT.MD PARSING
// ============================================================================

// ParsedScript represents the parsed conversation flow script.
type ParsedScript struct {
	Summary    string          // Condensed version for system prompt
	Sections   []ScriptSection // Parsed sections
	RawContent string          // Original markdown
}

// ScriptSection represents a section in the conversation flow script.
type ScriptSection struct {
	Name      string   // Section name
	Questions []string // Questions/actions in this section
	Required  bool     // Is this section mandatory
}

// ParseScript parses a SCRIPT.MD file and creates a condensed summary for the system prompt.
func (p *Parser) ParseScript(content string) (*ParsedScript, error) {
	result := &ParsedScript{
		RawContent: content,
		Sections:   make([]ScriptSection, 0),
	}

	// Split by section headers (## or lines with all caps)
	lines := strings.Split(content, "\n")
	var currentSection *ScriptSection
	var summaryParts []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Check if it's a section header (## SECTION NAME or ALL CAPS)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			// Save previous section
			if currentSection != nil && currentSection.Name != "" {
				result.Sections = append(result.Sections, *currentSection)
			}

			name := strings.TrimPrefix(trimmed, "## ")
			name = strings.TrimPrefix(name, "# ")
			currentSection = &ScriptSection{
				Name:      name,
				Questions: make([]string, 0),
				Required:  true, // Default to required
			}
			summaryParts = append(summaryParts, name)
			continue
		}

		// Check for all-caps section headers (like "CONVERSATION OPENING")
		if isAllCapsHeader(trimmed) {
			if currentSection != nil && currentSection.Name != "" {
				result.Sections = append(result.Sections, *currentSection)
			}
			currentSection = &ScriptSection{
				Name:      trimmed,
				Questions: make([]string, 0),
				Required:  true,
			}
			summaryParts = append(summaryParts, trimmed)
			continue
		}

		// Check for list items (questions/actions)
		if currentSection != nil {
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
				strings.HasPrefix(trimmed, "├── ") || strings.HasPrefix(trimmed, "└── ") {
				item := strings.TrimPrefix(trimmed, "- ")
				item = strings.TrimPrefix(item, "* ")
				item = strings.TrimPrefix(item, "├── ")
				item = strings.TrimPrefix(item, "└── ")
				currentSection.Questions = append(currentSection.Questions, item)
			} else if len(trimmed) > 0 && trimmed[0] >= '1' && trimmed[0] <= '9' {
				// Numbered list
				if idx := strings.Index(trimmed, "."); idx > 0 && idx < 3 {
					item := strings.TrimSpace(trimmed[idx+1:])
					currentSection.Questions = append(currentSection.Questions, item)
				}
			}
		}
	}

	// Add last section
	if currentSection != nil && currentSection.Name != "" {
		result.Sections = append(result.Sections, *currentSection)
	}

	// Build summary for system prompt (condensed)
	result.Summary = buildScriptSummary(result.Sections)

	return result, nil
}

// isAllCapsHeader checks if a line is an all-caps header (like "CONVERSATION OPENING")
func isAllCapsHeader(s string) bool {
	if len(s) < 5 {
		return false
	}
	// Must have at least some uppercase letters and be mostly uppercase
	upperCount := 0
	lowerCount := 0
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			upperCount++
		} else if c >= 'a' && c <= 'z' {
			lowerCount++
		}
	}
	return upperCount > 3 && lowerCount == 0
}

// buildScriptSummary creates a condensed summary of the script for the system prompt.
func buildScriptSummary(sections []ScriptSection) string {
	if len(sections) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Follow this conversation flow structure:\n")

	for i, section := range sections {
		sb.WriteString(string(rune('1'+i)) + ". " + section.Name)
		if len(section.Questions) > 0 && len(section.Questions) <= 5 {
			sb.WriteString(": ")
			// List first few questions
			questions := section.Questions
			if len(questions) > 3 {
				questions = questions[:3]
			}
			sb.WriteString(strings.Join(questions, ", "))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ValidateScript validates the parsed script for completeness.
func (p *Parser) ValidateScript(script *ParsedScript) *ValidationResult {
	result := &ValidationResult{
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	if len(script.Sections) == 0 {
		result.Warnings = append(result.Warnings, "No sections defined in script")
	}

	// Check for key conversation sections
	hasOpening := false
	hasClosing := false
	for _, s := range script.Sections {
		name := strings.ToUpper(s.Name)
		if strings.Contains(name, "OPENING") || strings.Contains(name, "INTRODUCTION") {
			hasOpening = true
		}
		if strings.Contains(name, "CLOSING") || strings.Contains(name, "CONCLUSION") {
			hasClosing = true
		}
	}

	if !hasOpening {
		result.Warnings = append(result.Warnings, "No opening/introduction section found")
	}
	if !hasClosing {
		result.Warnings = append(result.Warnings, "No closing/conclusion section found")
	}

	return result
}
