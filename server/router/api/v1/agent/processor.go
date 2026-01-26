package agent

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ProcessingOptions contains all configurable options for rule-based content processing.
type ProcessingOptions struct {
	// Content Extraction
	ExtractFAQs       bool `json:"extract_faqs"`
	ExtractServices   bool `json:"extract_services"`
	ExtractExclusions bool `json:"extract_exclusions"`
	ExtractCoverage   bool `json:"extract_coverage"`
	ExtractSafety     bool `json:"extract_safety"`

	// Text Normalization
	RemoveWhitespace   bool `json:"remove_whitespace"`
	StripHTML          bool `json:"strip_html"`
	FixEncoding        bool `json:"fix_encoding"`
	RemovePageNumbers  bool `json:"remove_page_numbers"`
	RemoveHeaderFooter bool `json:"remove_header_footer"`

	// Structure Splitting
	SplitH2         bool `json:"split_h2"`
	SplitH3         bool `json:"split_h3"`
	SplitParagraphs bool `json:"split_paragraphs"`
	SplitSentences  bool `json:"split_sentences"`
	PreserveLists   bool `json:"preserve_lists"`

	// Chunking Controls
	MaxChunkSize     int  `json:"max_chunk_size"`
	MinChunkSize     int  `json:"min_chunk_size"`
	ChunkOverlap     int  `json:"chunk_overlap"`
	MergeSmallChunks bool `json:"merge_small_chunks"`

	// Metadata
	GenerateTitles    bool `json:"generate_titles"`
	AddSourceRef      bool `json:"add_source_ref"`
	PreserveHierarchy bool `json:"preserve_hierarchy"`
}

// ProcessingResult contains the output of content processing.
type ProcessingResult struct {
	Content string           `json:"content"`
	Chunks  []ProcessedChunk `json:"chunks"`
	Stats   ProcessingStats  `json:"stats"`
}

// ProcessedChunk represents a single chunk after processing.
type ProcessedChunk struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Tokens  int    `json:"tokens"`
}

// ProcessingStats contains statistics about the processing operation.
type ProcessingStats struct {
	OriginalTokens    int `json:"original_tokens"`
	ProcessedTokens   int `json:"processed_tokens"`
	ChunksCreated     int `json:"chunks_created"`
	FAQsExtracted     int `json:"faqs_extracted"`
	ServicesExtracted int `json:"services_extracted"`
}

// ExtractedContent holds content extracted by type.
type ExtractedContent struct {
	FAQs       []ExtractedFAQ
	Services   []ExtractedService
	Exclusions []string
	Coverage   []string
	Safety     []string
	Sections   []ExtractedSection
}

// ExtractedFAQ represents an extracted FAQ item.
type ExtractedFAQ struct {
	Question string
	Answer   string
}

// ExtractedService represents an extracted service item.
type ExtractedService struct {
	Name        string
	Description string
}

// ExtractedSection represents a section of content.
type ExtractedSection struct {
	Title   string
	Content string
	Level   int // 1 for H1, 2 for H2, 3 for H3
}

// DefaultProcessingOptions returns sensible default options.
func DefaultProcessingOptions() ProcessingOptions {
	return ProcessingOptions{
		// Content Extraction - off by default
		ExtractFAQs:       false,
		ExtractServices:   false,
		ExtractExclusions: false,
		ExtractCoverage:   false,
		ExtractSafety:     false,

		// Text Normalization - some on by default
		RemoveWhitespace:   true,
		StripHTML:          false,
		FixEncoding:        false,
		RemovePageNumbers:  false,
		RemoveHeaderFooter: false,

		// Structure Splitting - on by default
		SplitH2:         true,
		SplitH3:         true,
		SplitParagraphs: true,
		SplitSentences:  false,
		PreserveLists:   true,

		// Chunking Controls
		MaxChunkSize:     800,
		MinChunkSize:     100,
		ChunkOverlap:     50,
		MergeSmallChunks: true,

		// Metadata - on by default
		GenerateTitles:    true,
		AddSourceRef:      false,
		PreserveHierarchy: true,
	}
}

// serializeProcessingOptions converts ProcessingOptions to JSON string.
func serializeProcessingOptions(opts ProcessingOptions) (string, error) {
	data, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// deserializeProcessingOptions converts JSON string to ProcessingOptions.
func deserializeProcessingOptions(data string) (ProcessingOptions, error) {
	var opts ProcessingOptions
	err := json.Unmarshal([]byte(data), &opts)
	return opts, err
}

// ContentProcessor handles rule-based content processing.
type ContentProcessor struct {
	options ProcessingOptions
}

// NewContentProcessor creates a new processor with given options.
func NewContentProcessor(opts ProcessingOptions) *ContentProcessor {
	// Apply defaults for zero values
	if opts.MaxChunkSize == 0 {
		opts.MaxChunkSize = 800
	}
	if opts.MinChunkSize == 0 {
		opts.MinChunkSize = 100
	}

	return &ContentProcessor{options: opts}
}

// Process applies all configured processing options to the content.
func (p *ContentProcessor) Process(content string, fileType string) ProcessingResult {
	originalTokens := EstimateTokens(content)

	// Step 1: Text Normalization
	normalized := p.normalizeText(content)

	// Step 2: Extract content by type
	extracted := p.extractContent(normalized)

	// Step 3: Build formatted output
	formatted := p.buildFormattedContent(extracted, fileType)

	// Step 4: Create chunks
	chunks := p.createChunks(formatted, fileType)

	processedTokens := EstimateTokens(formatted)

	return ProcessingResult{
		Content: formatted,
		Chunks:  chunks,
		Stats: ProcessingStats{
			OriginalTokens:    originalTokens,
			ProcessedTokens:   processedTokens,
			ChunksCreated:     len(chunks),
			FAQsExtracted:     len(extracted.FAQs),
			ServicesExtracted: len(extracted.Services),
		},
	}
}

// ============================================================================
// TEXT NORMALIZATION
// ============================================================================

func (p *ContentProcessor) normalizeText(content string) string {
	result := content

	// Normalize line endings (always do this)
	result = strings.ReplaceAll(result, "\r\n", "\n")
	result = strings.ReplaceAll(result, "\r", "\n")

	if p.options.FixEncoding {
		result = p.fixEncoding(result)
	}

	if p.options.StripHTML {
		result = p.stripHTML(result)
	}

	if p.options.RemovePageNumbers {
		result = p.removePageNumbers(result)
	}

	if p.options.RemoveHeaderFooter {
		result = p.removeHeaderFooter(result)
	}

	if p.options.RemoveWhitespace {
		result = p.removeExtraWhitespace(result)
	}

	return result
}

func (p *ContentProcessor) fixEncoding(content string) string {
	// Normalize to NFC (Canonical Decomposition, followed by Canonical Composition)
	return norm.NFC.String(content)
}

func (p *ContentProcessor) stripHTML(content string) string {
	// Remove HTML tags but preserve content
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	result := tagPattern.ReplaceAllString(content, "")

	// Decode common HTML entities
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", "\"")
	result = strings.ReplaceAll(result, "&#39;", "'")

	return result
}

func (p *ContentProcessor) removePageNumbers(content string) string {
	// Common page number patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)page\s+\d+\s*(of\s+\d+)?`),
		regexp.MustCompile(`(?m)^\s*\d+\s*$`), // Standalone numbers on their own line
		regexp.MustCompile(`(?i)-\s*\d+\s*-`), // - 1 - style
	}

	result := content
	for _, pattern := range patterns {
		result = pattern.ReplaceAllString(result, "")
	}
	return result
}

func (p *ContentProcessor) removeHeaderFooter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 10 {
		return content
	}

	// Count line occurrences to find repeated headers/footers
	lineCounts := make(map[string]int)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && len(trimmed) < 100 {
			lineCounts[trimmed]++
		}
	}

	// Lines appearing more than 3 times are likely headers/footers
	repeatedLines := make(map[string]bool)
	for line, count := range lineCounts {
		if count > 3 {
			repeatedLines[line] = true
		}
	}

	// Filter out repeated lines
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !repeatedLines[trimmed] {
			filtered = append(filtered, line)
		}
	}

	return strings.Join(filtered, "\n")
}

func (p *ContentProcessor) removeExtraWhitespace(content string) string {
	// Collapse multiple spaces to single space
	spacePattern := regexp.MustCompile(`[ \t]+`)
	result := spacePattern.ReplaceAllString(content, " ")

	// Collapse multiple newlines to double newline (paragraph break)
	newlinePattern := regexp.MustCompile(`\n{3,}`)
	result = newlinePattern.ReplaceAllString(result, "\n\n")

	// Trim whitespace from each line
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	result = strings.Join(lines, "\n")

	return strings.TrimSpace(result)
}

// ============================================================================
// CONTENT EXTRACTION
// ============================================================================

func (p *ContentProcessor) extractContent(content string) ExtractedContent {
	var extracted ExtractedContent

	// Always extract sections (structure)
	extracted.Sections = p.extractSections(content)

	if p.options.ExtractFAQs {
		extracted.FAQs = p.extractFAQs(content)
	}

	if p.options.ExtractServices {
		extracted.Services = p.extractServices(content)
	}

	if p.options.ExtractExclusions {
		extracted.Exclusions = p.extractExclusions(content)
	}

	if p.options.ExtractCoverage {
		extracted.Coverage = p.extractCoverage(content)
	}

	if p.options.ExtractSafety {
		extracted.Safety = p.extractSafety(content)
	}

	return extracted
}

func (p *ContentProcessor) extractSections(content string) []ExtractedSection {
	var sections []ExtractedSection
	lines := strings.Split(content, "\n")

	var currentSection *ExtractedSection
	var contentBuilder strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for headers
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			// H1 header
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(contentBuilder.String())
				sections = append(sections, *currentSection)
				contentBuilder.Reset()
			}
			currentSection = &ExtractedSection{
				Title: strings.TrimPrefix(trimmed, "# "),
				Level: 1,
			}
		} else if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			// H2 header
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(contentBuilder.String())
				sections = append(sections, *currentSection)
				contentBuilder.Reset()
			}
			currentSection = &ExtractedSection{
				Title: strings.TrimPrefix(trimmed, "## "),
				Level: 2,
			}
		} else if strings.HasPrefix(trimmed, "### ") {
			// H3 header
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(contentBuilder.String())
				sections = append(sections, *currentSection)
				contentBuilder.Reset()
			}
			currentSection = &ExtractedSection{
				Title: strings.TrimPrefix(trimmed, "### "),
				Level: 3,
			}
		} else if currentSection != nil {
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		} else {
			// Content before any header
			if currentSection == nil && trimmed != "" {
				currentSection = &ExtractedSection{
					Title: "Introduction",
					Level: 0,
				}
			}
			if currentSection != nil {
				contentBuilder.WriteString(line)
				contentBuilder.WriteString("\n")
			}
		}
	}

	// Don't forget the last section
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(contentBuilder.String())
		sections = append(sections, *currentSection)
	}

	return sections
}

func (p *ContentProcessor) extractFAQs(content string) []ExtractedFAQ {
	var faqs []ExtractedFAQ

	// Pattern 1: Q: ... A: ...
	qaPattern := regexp.MustCompile(`(?i)Q:\s*(.+?)\n+A:\s*(.+?)(?:\n\n|\z)`)
	matches := qaPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			faqs = append(faqs, ExtractedFAQ{
				Question: strings.TrimSpace(match[1]),
				Answer:   strings.TrimSpace(match[2]),
			})
		}
	}

	// Pattern 2: **Question?** Answer
	boldQPattern := regexp.MustCompile(`\*\*(.+?\?)\*\*\s*\n(.+?)(?:\n\n|\*\*|\z)`)
	matches = boldQPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			faqs = append(faqs, ExtractedFAQ{
				Question: strings.TrimSpace(match[1]),
				Answer:   strings.TrimSpace(match[2]),
			})
		}
	}

	// Pattern 3: Lines ending with ? followed by answer
	questionPattern := regexp.MustCompile(`(?m)^(.+\?)\s*\n([^?\n]+(?:\n[^?\n]+)*)`)
	matches = questionPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			question := strings.TrimSpace(match[1])
			answer := strings.TrimSpace(match[2])
			// Avoid duplicates
			isDuplicate := false
			for _, existing := range faqs {
				if existing.Question == question {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate && len(answer) > 20 {
				faqs = append(faqs, ExtractedFAQ{
					Question: question,
					Answer:   answer,
				})
			}
		}
	}

	return faqs
}

func (p *ContentProcessor) extractServices(content string) []ExtractedService {
	var services []ExtractedService

	// Pattern 1: Bullet list items under "Services" header
	serviceHeaderPattern := regexp.MustCompile(`(?i)(?:^|\n)##?\s*(?:Our\s+)?Services?\s*\n((?:[-*]\s*.+\n?)+)`)
	matches := serviceHeaderPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			bulletPattern := regexp.MustCompile(`[-*]\s*(.+)`)
			bullets := bulletPattern.FindAllStringSubmatch(match[1], -1)
			for _, bullet := range bullets {
				if len(bullet) >= 2 {
					name := strings.TrimSpace(bullet[1])
					// Split on : or - for description
					parts := regexp.MustCompile(`[:\-–]`).Split(name, 2)
					if len(parts) == 2 {
						services = append(services, ExtractedService{
							Name:        strings.TrimSpace(parts[0]),
							Description: strings.TrimSpace(parts[1]),
						})
					} else {
						services = append(services, ExtractedService{
							Name:        name,
							Description: "",
						})
					}
				}
			}
		}
	}

	// Pattern 2: Service: Description format
	serviceLinePattern := regexp.MustCompile(`(?m)^([A-Z][^:\n]{2,30}):\s*(.+)$`)
	matches = serviceLinePattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			name := strings.TrimSpace(match[1])
			desc := strings.TrimSpace(match[2])
			// Avoid duplicates
			isDuplicate := false
			for _, existing := range services {
				if existing.Name == name {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate {
				services = append(services, ExtractedService{
					Name:        name,
					Description: desc,
				})
			}
		}
	}

	return services
}

func (p *ContentProcessor) extractExclusions(content string) []string {
	var exclusions []string

	// Keywords indicating exclusions
	exclusionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:we\s+)?(?:do\s+)?not\s+(?:offer|provide|support|cover|handle)\s+(.+?)(?:\.|$)`),
		regexp.MustCompile(`(?i)(?:this\s+)?(?:does\s+)?not\s+include\s+(.+?)(?:\.|$)`),
		regexp.MustCompile(`(?i)exclud(?:es?|ing)\s+(.+?)(?:\.|$)`),
		regexp.MustCompile(`(?i)except(?:ion)?s?:\s*(.+?)(?:\.|$)`),
	}

	for _, pattern := range exclusionPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				exclusion := strings.TrimSpace(match[1])
				if exclusion != "" && len(exclusion) < 200 {
					exclusions = append(exclusions, exclusion)
				}
			}
		}
	}

	return exclusions
}

func (p *ContentProcessor) extractCoverage(content string) []string {
	var coverage []string

	// US State codes
	statePattern := regexp.MustCompile(`\b(AL|AK|AZ|AR|CA|CO|CT|DE|FL|GA|HI|ID|IL|IN|IA|KS|KY|LA|ME|MD|MA|MI|MN|MS|MO|MT|NE|NV|NH|NJ|NM|NY|NC|ND|OH|OK|OR|PA|RI|SC|SD|TN|TX|UT|VT|VA|WA|WV|WI|WY)\b`)
	matches := statePattern.FindAllString(content, -1)
	for _, match := range matches {
		// Deduplicate
		found := false
		for _, existing := range coverage {
			if existing == match {
				found = true
				break
			}
		}
		if !found {
			coverage = append(coverage, match)
		}
	}

	// ZIP code patterns
	zipPattern := regexp.MustCompile(`\b\d{5}(?:-\d{4})?\b`)
	matches = zipPattern.FindAllString(content, -1)
	for _, match := range matches {
		coverage = append(coverage, match)
	}

	// City, State patterns
	cityStatePattern := regexp.MustCompile(`([A-Z][a-z]+(?:\s+[A-Z][a-z]+)?),\s*([A-Z]{2})\b`)
	cityMatches := cityStatePattern.FindAllStringSubmatch(content, -1)
	for _, match := range cityMatches {
		if len(match) >= 3 {
			coverage = append(coverage, match[0])
		}
	}

	return coverage
}

func (p *ContentProcessor) extractSafety(content string) []string {
	var safety []string

	// Safety keywords
	safetyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:in\s+case\s+of\s+)?emergency[,:]?\s*(.+?)(?:\.|$)`),
		regexp.MustCompile(`(?i)warning[!:]?\s*(.+?)(?:\.|$)`),
		regexp.MustCompile(`(?i)danger[!:]?\s*(.+?)(?:\.|$)`),
		regexp.MustCompile(`(?i)caution[!:]?\s*(.+?)(?:\.|$)`),
		regexp.MustCompile(`(?i)call\s+911\s*(.+?)(?:\.|$)`),
		regexp.MustCompile(`(?i)safety\s+(?:tip|note|warning)[:]?\s*(.+?)(?:\.|$)`),
	}

	for _, pattern := range safetyPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				instruction := strings.TrimSpace(match[0])
				if instruction != "" && len(instruction) < 500 {
					safety = append(safety, instruction)
				}
			}
		}
	}

	return safety
}

// ============================================================================
// CONTENT FORMATTING
// ============================================================================

func (p *ContentProcessor) buildFormattedContent(extracted ExtractedContent, fileType string) string {
	var builder strings.Builder

	// Title
	builder.WriteString("# Formatted ")
	if fileType == "kb" {
		builder.WriteString("Knowledge Base")
	} else {
		builder.WriteString("Policy Document")
	}
	builder.WriteString("\n\n")

	// FAQs section
	if len(extracted.FAQs) > 0 {
		builder.WriteString("## Frequently Asked Questions\n\n")
		for _, faq := range extracted.FAQs {
			builder.WriteString("**")
			builder.WriteString(faq.Question)
			builder.WriteString("**\n\n")
			builder.WriteString(faq.Answer)
			builder.WriteString("\n\n")
		}
		builder.WriteString("---\n\n")
	}

	// Services section
	if len(extracted.Services) > 0 {
		builder.WriteString("## Services\n\n")
		for _, svc := range extracted.Services {
			builder.WriteString("- **")
			builder.WriteString(svc.Name)
			builder.WriteString("**")
			if svc.Description != "" {
				builder.WriteString(": ")
				builder.WriteString(svc.Description)
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n---\n\n")
	}

	// Exclusions section
	if len(extracted.Exclusions) > 0 {
		builder.WriteString("## Exclusions\n\n")
		for _, exc := range extracted.Exclusions {
			builder.WriteString("- ")
			builder.WriteString(exc)
			builder.WriteString("\n")
		}
		builder.WriteString("\n---\n\n")
	}

	// Coverage section
	if len(extracted.Coverage) > 0 {
		builder.WriteString("## Coverage Areas\n\n")
		builder.WriteString(strings.Join(extracted.Coverage, ", "))
		builder.WriteString("\n\n---\n\n")
	}

	// Safety section
	if len(extracted.Safety) > 0 {
		builder.WriteString("## Safety Information\n\n")
		for _, s := range extracted.Safety {
			builder.WriteString("- ")
			builder.WriteString(s)
			builder.WriteString("\n")
		}
		builder.WriteString("\n---\n\n")
	}

	// Main content sections
	if len(extracted.Sections) > 0 {
		for _, section := range extracted.Sections {
			if section.Title != "" {
				switch section.Level {
				case 1:
					builder.WriteString("## ")
				case 2:
					builder.WriteString("## ")
				case 3:
					builder.WriteString("### ")
				default:
					builder.WriteString("## ")
				}
				builder.WriteString(section.Title)
				builder.WriteString("\n\n")
			}
			if section.Content != "" {
				builder.WriteString(section.Content)
				builder.WriteString("\n\n")
			}
		}
	}

	return strings.TrimSpace(builder.String())
}

// ============================================================================
// CHUNKING
// ============================================================================

func (p *ContentProcessor) createChunks(content string, fileType string) []ProcessedChunk {
	var chunks []ProcessedChunk

	// Split by structure based on options
	var sections []string

	if p.options.SplitH2 {
		sections = p.splitByHeaders(content, 2)
	} else if p.options.SplitH3 {
		sections = p.splitByHeaders(content, 3)
	} else if p.options.SplitParagraphs {
		sections = p.splitByParagraphs(content)
	} else if p.options.SplitSentences {
		sections = p.splitBySentences(content)
	} else {
		sections = []string{content}
	}

	// Process each section
	for i, section := range sections {
		tokens := EstimateTokens(section)

		// If section is too large, split further
		if tokens > p.options.MaxChunkSize {
			subChunks := p.splitLargeSection(section, p.options.MaxChunkSize)
			for j, sub := range subChunks {
				title := p.extractTitle(sub)
				if p.options.PreserveHierarchy && len(sections) > 1 {
					parentTitle := p.extractTitle(section)
					if parentTitle != title {
						title = parentTitle + " > " + title
					}
				}
				chunks = append(chunks, ProcessedChunk{
					ID:      generateChunkID(fileType, i, j),
					Title:   title,
					Content: sub,
					Tokens:  EstimateTokens(sub),
				})
			}
		} else {
			title := p.extractTitle(section)
			if p.options.GenerateTitles && title == "" {
				title = p.generateTitle(section)
			}
			chunks = append(chunks, ProcessedChunk{
				ID:      generateChunkID(fileType, i, 0),
				Title:   title,
				Content: section,
				Tokens:  tokens,
			})
		}
	}

	// Merge small chunks if enabled
	if p.options.MergeSmallChunks {
		chunks = p.mergeSmallChunks(chunks)
	}

	return chunks
}

func (p *ContentProcessor) splitByHeaders(content string, level int) []string {
	prefix := strings.Repeat("#", level) + " "
	lines := strings.Split(content, "\n")
	var sections []string
	var currentSection strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			if currentSection.Len() > 0 {
				sections = append(sections, strings.TrimSpace(currentSection.String()))
				currentSection.Reset()
			}
		}
		currentSection.WriteString(line)
		currentSection.WriteString("\n")
	}

	if currentSection.Len() > 0 {
		sections = append(sections, strings.TrimSpace(currentSection.String()))
	}

	if len(sections) == 0 && content != "" {
		sections = []string{content}
	}

	return sections
}

func (p *ContentProcessor) splitByParagraphs(content string) []string {
	paragraphs := strings.Split(content, "\n\n")
	var result []string
	for _, para := range paragraphs {
		trimmed := strings.TrimSpace(para)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (p *ContentProcessor) splitBySentences(content string) []string {
	// Simple sentence splitting - split on . ! ? followed by space and capital
	sentencePattern := regexp.MustCompile(`([.!?])\s+([A-Z])`)
	result := sentencePattern.ReplaceAllString(content, "$1\n\n$2")
	return p.splitByParagraphs(result)
}

func (p *ContentProcessor) splitLargeSection(content string, maxTokens int) []string {
	var result []string

	// First try splitting by H3 headers
	if p.options.SplitH3 {
		subSections := p.splitByHeaders(content, 3)
		if len(subSections) > 1 {
			for _, sub := range subSections {
				if EstimateTokens(sub) > maxTokens {
					// Still too large, split by paragraphs
					result = append(result, p.splitByParagraphsWithLimit(sub, maxTokens)...)
				} else {
					result = append(result, sub)
				}
			}
			return result
		}
	}

	// Fall back to paragraph splitting
	return p.splitByParagraphsWithLimit(content, maxTokens)
}

func (p *ContentProcessor) splitByParagraphsWithLimit(content string, maxTokens int) []string {
	paragraphs := p.splitByParagraphs(content)
	var result []string
	var currentChunk strings.Builder
	currentTokens := 0

	for _, para := range paragraphs {
		paraTokens := EstimateTokens(para)

		if currentTokens+paraTokens > maxTokens && currentChunk.Len() > 0 {
			result = append(result, strings.TrimSpace(currentChunk.String()))
			currentChunk.Reset()
			currentTokens = 0
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)
		currentTokens += paraTokens
	}

	if currentChunk.Len() > 0 {
		result = append(result, strings.TrimSpace(currentChunk.String()))
	}

	return result
}

func (p *ContentProcessor) extractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
		if strings.HasPrefix(trimmed, "## ") {
			return strings.TrimPrefix(trimmed, "## ")
		}
		if strings.HasPrefix(trimmed, "### ") {
			return strings.TrimPrefix(trimmed, "### ")
		}
	}
	return ""
}

func (p *ContentProcessor) generateTitle(content string) string {
	// Use first non-empty line up to 50 chars
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip markdown formatting
		trimmed = strings.TrimPrefix(trimmed, "# ")
		trimmed = strings.TrimPrefix(trimmed, "## ")
		trimmed = strings.TrimPrefix(trimmed, "### ")
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")

		if trimmed != "" {
			if len(trimmed) > 50 {
				// Find word boundary
				runes := []rune(trimmed)
				for i := 50; i >= 0; i-- {
					if unicode.IsSpace(runes[i]) {
						return string(runes[:i]) + "..."
					}
				}
				return string(runes[:47]) + "..."
			}
			return trimmed
		}
	}
	return "Untitled"
}

func (p *ContentProcessor) mergeSmallChunks(chunks []ProcessedChunk) []ProcessedChunk {
	if len(chunks) <= 1 {
		return chunks
	}

	var result []ProcessedChunk
	var pending *ProcessedChunk

	for i := range chunks {
		chunk := chunks[i]

		if pending != nil {
			combinedTokens := pending.Tokens + chunk.Tokens
			if combinedTokens <= p.options.MaxChunkSize {
				// Merge
				pending.Content += "\n\n" + chunk.Content
				pending.Tokens = combinedTokens
				if pending.Tokens >= p.options.MinChunkSize {
					result = append(result, *pending)
					pending = nil
				}
			} else {
				// Can't merge
				result = append(result, *pending)
				if chunk.Tokens < p.options.MinChunkSize {
					pending = &chunk
				} else {
					result = append(result, chunk)
					pending = nil
				}
			}
		} else {
			if chunk.Tokens < p.options.MinChunkSize {
				pending = &chunk
			} else {
				result = append(result, chunk)
			}
		}
	}

	if pending != nil {
		result = append(result, *pending)
	}

	return result
}

func generateChunkID(fileType string, sectionIdx, subIdx int) string {
	if subIdx > 0 {
		return strings.ToLower(fileType) + "_section_" + itoa(sectionIdx) + "_" + itoa(subIdx)
	}
	return strings.ToLower(fileType) + "_section_" + itoa(sectionIdx)
}

// itoa converts int to string (simple implementation to avoid strconv import)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
