package agent

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/usememos/memos/store"
)

// DocumentChunk represents a single chunk of content for vector indexing.
type DocumentChunk struct {
	// Identity
	ID           string // Unique identifier: tenantID:audience:type:code
	TenantID     int32
	AudienceType string

	// Content
	ContentType string // service, faq, exclusion, coverage, rule, safety, kb_section
	Title       string
	Content     string
	Code        string // service code, rule code, etc.

	// Metadata
	IsEmergency   bool
	IsActive      bool
	Priority      int32
	SourceVersion int32

	// Vector (populated after embedding)
	Embedding []float32

	// Timestamps
	IndexedAt time.Time
}

// ChunkID generates a unique ID for a chunk.
func ChunkID(tenantID int32, audience, contentType, code string) string {
	return fmt.Sprintf("%d:%s:%s:%s", tenantID, audience, contentType, code)
}

// ChunkedDocument holds all chunks extracted from a tenant's documents.
type ChunkedDocument struct {
	TenantID     int32
	AudienceType string
	Chunks       []DocumentChunk
	SourceHash   string // Combined hash for change detection
}

// Chunker handles document chunking for vector indexing.
type Chunker struct {
	// Configuration
	maxChunkSize int // Maximum chunk size in characters (for future use)
}

// NewChunker creates a new document chunker.
func NewChunker() *Chunker {
	return &Chunker{
		maxChunkSize: 2000, // Default max chunk size
	}
}

// ChunkKBContent extracts chunks from parsed KB content.
//
// Deprecated: Use ChunkMarkdownContent instead. This function uses structured
// annotation parsing which can produce false positives. RAG retrieval now relies
// on embeddings and hybrid search for relevance ranking, not content type classification.
func (c *Chunker) ChunkKBContent(
	kb *ParsedKB,
	tenantID int32,
	audience string,
	sourceVersion int32,
) []DocumentChunk {
	var chunks []DocumentChunk
	now := time.Now()

	// Services
	for _, svc := range kb.Services {
		content := buildServiceContent(svc)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "service", svc.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "service",
			Title:         svc.Name,
			Content:       content,
			Code:          svc.Code,
			IsEmergency:   svc.IsEmergency,
			IsActive:      svc.IsActive,
			Priority:      0, // Services don't have priority in store
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// FAQs
	for i, faq := range kb.FAQs {
		code := faq.Code
		if code == "" {
			code = fmt.Sprintf("faq_%d", i)
		}
		content := buildFAQContent(faq)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "faq", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "faq",
			Title:         faq.Question,
			Content:       content,
			Code:          code,
			IsActive:      faq.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Exclusions
	for _, exc := range kb.Exclusions {
		content := buildExclusionContent(exc)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "exclusion", exc.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "exclusion",
			Title:         exc.Name,
			Content:       content,
			Code:          exc.Code,
			IsActive:      exc.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Coverage areas
	for i, cov := range kb.Coverage {
		code := fmt.Sprintf("coverage_%d", i)
		content := buildCoverageContent(cov)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "coverage", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "coverage",
			Title:         cov.AreaName,
			Content:       content,
			Code:          code,
			IsActive:      cov.IsIncluded, // IsIncluded means it's an active coverage area
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Safety protocols
	for i, safety := range kb.Safety {
		code := safety.Code
		if code == "" {
			code = fmt.Sprintf("safety_%d", i)
		}
		content := buildSafetyContent(safety)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "safety", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "safety",
			Title:         safety.Name,
			Content:       content,
			Code:          code,
			IsEmergency:   true, // Safety protocols are always high priority
			IsActive:      safety.IsActive,
			Priority:      100, // High priority
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// KB Sections (general knowledge sections)
	for i, section := range kb.Sections {
		code := section.Code
		if code == "" {
			code = fmt.Sprintf("kb_section_%d", i)
		}
		content := buildKBSectionContent(section)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "kb_section", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "kb_section",
			Title:         section.Title,
			Content:       content,
			Code:          code,
			IsActive:      section.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	return chunks
}

// ChunkPolicyContent extracts chunks from parsed Policy content.
//
// Deprecated: Use ChunkMarkdownContent instead. This function uses structured
// annotation parsing which can produce false positives. RAG retrieval now relies
// on embeddings and hybrid search for relevance ranking, not content type classification.
func (c *Chunker) ChunkPolicyContent(
	policy *ParsedPolicy,
	tenantID int32,
	audience string,
	sourceVersion int32,
) []DocumentChunk {
	var chunks []DocumentChunk
	now := time.Now()

	// Rules
	for _, rule := range policy.Rules {
		content := buildRuleContent(rule)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "rule", rule.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "rule",
			Title:         rule.Name,
			Content:       content,
			Code:          rule.Code,
			IsActive:      rule.IsActive,
			Priority:      int32(rule.Priority),
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Intents (for context matching)
	for _, intent := range policy.Intents {
		code := fmt.Sprintf("intent_%s", intent.Code)
		content := buildIntentContent(intent)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "intent", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "intent",
			Title:         intent.Name,
			Content:       content,
			Code:          code,
			IsActive:      intent.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	return chunks
}

// ChunkFromStoreTypes creates chunks from store types (for existing data).
//
// Deprecated: Direct database chunking is no longer used. Content should be
// chunked from source files using ChunkMarkdownContent instead.
func (c *Chunker) ChunkFromStoreTypes(
	services []*store.AgentService,
	exclusions []*store.AgentExclusion,
	faqs []*store.AgentFAQ,
	coverage []*store.AgentCoverage,
	safety []*store.AgentSafetyProtocol,
	rules []*store.AgentRule,
	tenantID int32,
	audience string,
	sourceVersion int32,
) []DocumentChunk {
	var chunks []DocumentChunk
	now := time.Now()

	// Services
	for _, svc := range services {
		content := buildServiceContent(svc)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "service", svc.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "service",
			Title:         svc.Name,
			Content:       content,
			Code:          svc.Code,
			IsEmergency:   svc.IsEmergency,
			IsActive:      svc.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Exclusions
	for _, exc := range exclusions {
		content := buildExclusionContent(exc)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "exclusion", exc.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "exclusion",
			Title:         exc.Name,
			Content:       content,
			Code:          exc.Code,
			IsActive:      exc.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// FAQs
	for i, faq := range faqs {
		code := faq.Code
		if code == "" {
			code = fmt.Sprintf("faq_%d", i)
		}
		content := buildFAQContent(faq)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "faq", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "faq",
			Title:         faq.Question,
			Content:       content,
			Code:          code,
			IsActive:      faq.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Coverage
	for i, cov := range coverage {
		code := fmt.Sprintf("coverage_%d", i)
		content := buildCoverageContent(cov)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "coverage", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "coverage",
			Title:         cov.AreaName,
			Content:       content,
			Code:          code,
			IsActive:      cov.IsIncluded,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Safety protocols
	for i, sp := range safety {
		code := sp.Code
		if code == "" {
			code = fmt.Sprintf("safety_%d", i)
		}
		content := buildSafetyContent(sp)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "safety", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "safety",
			Title:         sp.Name,
			Content:       content,
			Code:          code,
			IsEmergency:   true,
			IsActive:      sp.IsActive,
			Priority:      100,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Rules
	for _, rule := range rules {
		content := buildRuleContent(rule)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "rule", rule.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "rule",
			Title:         rule.Name,
			Content:       content,
			Code:          rule.Code,
			IsActive:      rule.IsActive,
			Priority:      int32(rule.Priority),
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	return chunks
}

// GetTextsForEmbedding extracts text content from chunks for embedding.
func GetTextsForEmbedding(chunks []DocumentChunk) []string {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		// Combine title and content for better semantic matching
		texts[i] = fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)
	}
	return texts
}

// ============================================================================
// CONTENT BUILDERS (from store types)
// ============================================================================

func buildServiceContent(svc *store.AgentService) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", svc.Name, svc.Description))
	if svc.ResponseTime != "" {
		parts = append(parts, fmt.Sprintf("Response time: %s", svc.ResponseTime))
	}
	if svc.IsEmergency {
		parts = append(parts, "This is an emergency service.")
	}
	return strings.Join(parts, ". ")
}

func buildFAQContent(faq *store.AgentFAQ) string {
	return fmt.Sprintf("Question: %s\nAnswer: %s", faq.Question, faq.Answer)
}

func buildExclusionContent(exc *store.AgentExclusion) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", exc.Name, exc.Description))
	if exc.ExceptionRule != "" {
		parts = append(parts, fmt.Sprintf("Exception: %s", exc.ExceptionRule))
	}
	if exc.Referral != "" {
		parts = append(parts, fmt.Sprintf("Referral: %s", exc.Referral))
	}
	return strings.Join(parts, ". ")
}

func buildCoverageContent(cov *store.AgentCoverage) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s (%s)", cov.AreaName, cov.AreaType))
	if cov.StateCode != "" {
		parts = append(parts, fmt.Sprintf("State: %s", cov.StateCode))
	}
	if cov.IsIncluded {
		parts = append(parts, "This area is covered.")
	} else {
		parts = append(parts, "This area is NOT covered.")
	}
	return strings.Join(parts, ". ")
}

func buildSafetyContent(safety *store.AgentSafetyProtocol) string {
	var parts []string
	parts = append(parts, safety.Name)
	if len(safety.TriggerIntents) > 0 {
		parts = append(parts, fmt.Sprintf("Triggers: %s", strings.Join(safety.TriggerIntents, ", ")))
	}
	if len(safety.Instructions) > 0 {
		parts = append(parts, fmt.Sprintf("Instructions: %s", strings.Join(safety.Instructions, "; ")))
	}
	return strings.Join(parts, ". ")
}

func buildKBSectionContent(section *store.AgentKBSection) string {
	return fmt.Sprintf("%s: %s", section.Title, section.Content)
}

func buildRuleContent(rule *store.AgentRule) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", rule.Name, rule.Description))
	if rule.AppliesTo != "" {
		parts = append(parts, fmt.Sprintf("Applies to: %s", rule.AppliesTo))
	}
	return strings.Join(parts, ". ")
}

func buildIntentContent(intent *store.AgentIntent) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", intent.Name, intent.Description))
	if len(intent.Examples) > 0 {
		parts = append(parts, fmt.Sprintf("Examples: %s", strings.Join(intent.Examples, "; ")))
	}
	if intent.Category != "" {
		parts = append(parts, fmt.Sprintf("Category: %s", intent.Category))
	}
	return strings.Join(parts, ". ")
}

// ============================================================================
// HEADING-BASED CHUNKER (for RAG mode)
// ============================================================================

const (
	DefaultTokenThreshold = 30000 // Threshold for switching to RAG mode
	MinChunkTokens        = 30    // Minimum tokens per chunk
	MaxChunkTokens        = 150   // Default max tokens (for local)
	ChunkOverlapTokens    = 50    // Overlap between chunks for context continuity
)

// GetMaxChunkTokens returns the maximum chunk size based on embedding provider.
// Different providers have different token limits:
// - OpenRouter (text-embedding-3-small): 8191 tokens - can use large chunks
// - Local (sentence-transformers): 512 tokens - needs small chunks
func GetMaxChunkTokens(embeddingProvider string) int {
	switch embeddingProvider {
	case "openrouter":
		return 4000 // text-embedding-3-small supports 8191 tokens, use 4000 (50% of max)
	case "local":
		return 150 // 512 token limit with aggressive subword tokenization
	case "mock":
		return 500 // Mock doesn't have real limits
	default:
		return 500 // Conservative default
	}
}

// GetMinChunkTokens returns the minimum chunk size based on embedding provider.
func GetMinChunkTokens(embeddingProvider string) int {
	switch embeddingProvider {
	case "openrouter":
		return 200 // Larger min for larger chunks (scaled with 4000 max)
	case "local":
		return 30 // Small min for small chunks
	default:
		return 50
	}
}

// EstimateTokens estimates the token count for a given text.
// Note: This is approximate. Actual tokenization varies by model:
// - GPT-style: ~4 chars/token
// - Sentence-transformers: ~1.9 chars/token (subword)
// We use /4 as a baseline; chunk limits are set conservatively to compensate.
func EstimateTokens(content string) int {
	return len(content) / 4
}

// ShouldUseRAG determines if RAG mode should be used based on content size.
func ShouldUseRAG(kbContent, policyContent string) bool {
	totalTokens := EstimateTokens(kbContent) + EstimateTokens(policyContent)
	return totalTokens >= DefaultTokenThreshold
}

// sanitizeUTF8 removes invalid UTF-8 sequences from content.
// This prevents LanceDB serialization errors when content contains corrupted bytes.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	// Replace invalid sequences with empty string
	return strings.ToValidUTF8(s, "")
}

// ChunkMarkdownContent chunks raw markdown using heading-based splitting.
// This is the main entry point for the new chunking strategy.
// maxTokens controls chunk size - use GetMaxChunkTokens(provider) to get appropriate value.
func (c *Chunker) ChunkMarkdownContent(
	content string,
	tenantID int32,
	audience string,
	fileType string, // "kb" or "policy"
	sourceVersion int32,
	maxTokens int, // Use GetMaxChunkTokens(embeddingProvider) for this value
) []DocumentChunk {
	// Sanitize UTF-8: remove invalid sequences to prevent LanceDB errors
	content = sanitizeUTF8(content)

	if strings.TrimSpace(content) == "" {
		return nil
	}

	// Use defaults if not specified
	if maxTokens <= 0 {
		maxTokens = MaxChunkTokens
	}
	minTokens := maxTokens / 5 // Scale min proportionally
	if minTokens < 30 {
		minTokens = 30
	}

	now := time.Now()
	var chunks []DocumentChunk

	// Split by H2 headers (## )
	sections := splitByH2Headers(content)

	for i, section := range sections {
		title, body := extractTitleAndBody(section)
		if strings.TrimSpace(body) == "" {
			continue
		}

		tokens := EstimateTokens(body)

		if tokens <= maxTokens {
			// Section fits in one chunk
			code := fmt.Sprintf("%s_section_%d", fileType, i)
			chunks = append(chunks, DocumentChunk{
				ID:            ChunkID(tenantID, audience, fileType+"_section", code),
				TenantID:      tenantID,
				AudienceType:  audience,
				ContentType:   fileType + "_section",
				Title:         title,
				Content:       body,
				Code:          code,
				IsActive:      true,
				SourceVersion: sourceVersion,
				IndexedAt:     now,
			})
		} else {
			// Section too large, split by H3 headers
			subSections := splitByH3Headers(body)

			if len(subSections) > 1 {
				for j, subSection := range subSections {
					subTitle, subBody := extractTitleAndBody(subSection)
					if strings.TrimSpace(subBody) == "" {
						continue
					}

					// If subsection still too large, split by paragraphs
					if EstimateTokens(subBody) > maxTokens {
						paragraphChunks := splitByParagraphs(subBody, title+" > "+subTitle, maxTokens)
						for k, pc := range paragraphChunks {
							code := fmt.Sprintf("%s_section_%d_%d_%d", fileType, i, j, k)
							chunks = append(chunks, DocumentChunk{
								ID:            ChunkID(tenantID, audience, fileType+"_section", code),
								TenantID:      tenantID,
								AudienceType:  audience,
								ContentType:   fileType + "_section",
								Title:         pc.title,
								Content:       pc.content,
								Code:          code,
								IsActive:      true,
								SourceVersion: sourceVersion,
								IndexedAt:     now,
							})
						}
					} else {
						code := fmt.Sprintf("%s_section_%d_%d", fileType, i, j)
						fullTitle := title
						if subTitle != "" {
							fullTitle = title + " > " + subTitle
						}
						chunks = append(chunks, DocumentChunk{
							ID:            ChunkID(tenantID, audience, fileType+"_section", code),
							TenantID:      tenantID,
							AudienceType:  audience,
							ContentType:   fileType + "_section",
							Title:         fullTitle,
							Content:       subBody,
							Code:          code,
							IsActive:      true,
							SourceVersion: sourceVersion,
							IndexedAt:     now,
						})
					}
				}
			} else {
				// No H3 headers, split by paragraphs
				paragraphChunks := splitByParagraphs(body, title, maxTokens)
				for k, pc := range paragraphChunks {
					code := fmt.Sprintf("%s_section_%d_%d", fileType, i, k)
					chunks = append(chunks, DocumentChunk{
						ID:            ChunkID(tenantID, audience, fileType+"_section", code),
						TenantID:      tenantID,
						AudienceType:  audience,
						ContentType:   fileType + "_section",
						Title:         pc.title,
						Content:       pc.content,
						Code:          code,
						IsActive:      true,
						SourceVersion: sourceVersion,
						IndexedAt:     now,
					})
				}
			}
		}
	}

	// Apply minimum size filter - merge tiny chunks
	chunks = mergeSmallChunks(chunks, minTokens, maxTokens)

	// Add overlap between consecutive chunks for context continuity
	chunks = addChunkOverlap(chunks, ChunkOverlapTokens)

	return chunks
}

// splitByH2Headers splits content by ## headers.
func splitByH2Headers(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var currentSection strings.Builder
	inSection := false

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if inSection && currentSection.Len() > 0 {
				sections = append(sections, currentSection.String())
				currentSection.Reset()
			}
			inSection = true
		}
		if inSection {
			currentSection.WriteString(line)
			currentSection.WriteString("\n")
		} else {
			// Preamble content before first header
			currentSection.WriteString(line)
			currentSection.WriteString("\n")
		}
	}

	// Don't forget the last section
	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}

	// If no headers found, return entire content as one section
	if len(sections) == 0 && len(content) > 0 {
		sections = append(sections, content)
	}

	return sections
}

// splitByH3Headers splits content by ### headers.
func splitByH3Headers(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var currentSection strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			if currentSection.Len() > 0 {
				sections = append(sections, currentSection.String())
				currentSection.Reset()
			}
		}
		currentSection.WriteString(line)
		currentSection.WriteString("\n")
	}

	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}

	return sections
}

// extractTitleAndBody extracts the title (first line if header) and body from a section.
func extractTitleAndBody(section string) (title, body string) {
	lines := strings.Split(strings.TrimSpace(section), "\n")
	if len(lines) == 0 {
		return "", ""
	}

	firstLine := strings.TrimSpace(lines[0])

	// Check if first line is a header
	if strings.HasPrefix(firstLine, "## ") {
		title = strings.TrimPrefix(firstLine, "## ")
		body = strings.Join(lines[1:], "\n")
	} else if strings.HasPrefix(firstLine, "### ") {
		title = strings.TrimPrefix(firstLine, "### ")
		body = strings.Join(lines[1:], "\n")
	} else {
		title = "Content"
		body = section
	}

	return strings.TrimSpace(title), strings.TrimSpace(body)
}

type paragraphChunk struct {
	title   string
	content string
}

// splitBySentences splits text into sentences using common sentence terminators.
// This is a fallback for when paragraph splitting produces chunks that are too large.
func splitBySentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		// Check for sentence terminators
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Look ahead to confirm sentence boundary
			// (not abbreviations like "Dr.", "e.g.", numbers like "3.14")
			if i+1 < len(runes) {
				next := runes[i+1]
				// Sentence ends if followed by space and uppercase, or end of text
				if next == ' ' || next == '\n' || next == '\r' {
					sentence := strings.TrimSpace(current.String())
					if sentence != "" {
						sentences = append(sentences, sentence)
					}
					current.Reset()
				}
			}
		}
	}

	// Don't forget remaining content
	if current.Len() > 0 {
		sentence := strings.TrimSpace(current.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// splitByParagraphs splits content by blank lines and groups into chunks.
// If a single paragraph exceeds maxTokens, it will be split by sentences.
func splitByParagraphs(content, title string, maxTokens int) []paragraphChunk {
	paragraphs := strings.Split(content, "\n\n")
	var chunks []paragraphChunk
	var currentContent strings.Builder
	chunkIndex := 0

	// Helper to flush current content as a chunk
	flushChunk := func() {
		if currentContent.Len() > 0 {
			chunkTitle := title
			if chunkIndex > 0 || len(chunks) > 0 {
				chunkTitle = fmt.Sprintf("%s (Part %d)", title, len(chunks)+1)
			}
			chunks = append(chunks, paragraphChunk{
				title:   chunkTitle,
				content: strings.TrimSpace(currentContent.String()),
			})
			currentContent.Reset()
			chunkIndex++
		}
	}

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraTokens := EstimateTokens(para)

		// If single paragraph exceeds limit, split by sentences
		if paraTokens > maxTokens {
			// First flush any existing content
			flushChunk()

			// Split this large paragraph by sentences
			sentences := splitBySentences(para)
			var sentenceBuffer strings.Builder

			for _, sent := range sentences {
				combined := sentenceBuffer.String()
				if combined != "" {
					combined += " "
				}
				combined += sent

				if EstimateTokens(combined) > maxTokens && sentenceBuffer.Len() > 0 {
					// Save current sentence buffer as chunk
					chunks = append(chunks, paragraphChunk{
						title:   fmt.Sprintf("%s (Part %d)", title, len(chunks)+1),
						content: strings.TrimSpace(sentenceBuffer.String()),
					})
					sentenceBuffer.Reset()
					sentenceBuffer.WriteString(sent)
				} else {
					if sentenceBuffer.Len() > 0 {
						sentenceBuffer.WriteString(" ")
					}
					sentenceBuffer.WriteString(sent)
				}
			}

			// Flush remaining sentences
			if sentenceBuffer.Len() > 0 {
				chunks = append(chunks, paragraphChunk{
					title:   fmt.Sprintf("%s (Part %d)", title, len(chunks)+1),
					content: strings.TrimSpace(sentenceBuffer.String()),
				})
			}
			continue
		}

		// Normal paragraph processing
		combined := currentContent.String()
		if combined != "" {
			combined += "\n\n"
		}
		combined += para

		if EstimateTokens(combined) > maxTokens && currentContent.Len() > 0 {
			// Save current chunk and start new one
			flushChunk()
			currentContent.WriteString(para)
		} else {
			if currentContent.Len() > 0 {
				currentContent.WriteString("\n\n")
			}
			currentContent.WriteString(para)
		}
	}

	// Don't forget the last chunk
	flushChunk()

	return chunks
}

// mergeSmallChunks merges chunks that are too small.
func mergeSmallChunks(chunks []DocumentChunk, minTokens, maxTokens int) []DocumentChunk {
	if len(chunks) <= 1 {
		return chunks
	}

	var result []DocumentChunk
	var pendingChunk *DocumentChunk

	for i := range chunks {
		chunk := chunks[i]
		tokens := EstimateTokens(chunk.Content)

		if pendingChunk != nil {
			// Try to merge with pending chunk
			mergedTokens := EstimateTokens(pendingChunk.Content) + tokens
			if mergedTokens <= maxTokens {
				pendingChunk.Content += "\n\n" + chunk.Content
				pendingChunk.Title = pendingChunk.Title // Keep original title

				// If merged chunk is now large enough, add it
				if EstimateTokens(pendingChunk.Content) >= minTokens {
					result = append(result, *pendingChunk)
					pendingChunk = nil
				}
			} else {
				// Can't merge, add pending and start new
				result = append(result, *pendingChunk)
				if tokens < minTokens {
					pendingChunk = &chunk
				} else {
					result = append(result, chunk)
					pendingChunk = nil
				}
			}
		} else {
			if tokens < minTokens {
				// Too small, hold for merging
				pendingChunk = &chunk
			} else {
				result = append(result, chunk)
			}
		}
	}

	// Add any remaining pending chunk
	if pendingChunk != nil {
		result = append(result, *pendingChunk)
	}

	return result
}

// addChunkOverlap prepends context from the previous chunk to each chunk.
// This improves retrieval when a query spans chunk boundaries.
func addChunkOverlap(chunks []DocumentChunk, overlapTokens int) []DocumentChunk {
	if len(chunks) <= 1 || overlapTokens <= 0 {
		return chunks
	}

	for i := 1; i < len(chunks); i++ {
		prevContent := chunks[i-1].Content
		overlapChars := overlapTokens * 4 // Token approximation (4 chars/token)

		if len(prevContent) > overlapChars {
			// Take the last N characters from the previous chunk
			overlap := prevContent[len(prevContent)-overlapChars:]
			// Find a sentence boundary to avoid cutting mid-sentence
			if idx := strings.Index(overlap, ". "); idx > 0 {
				overlap = overlap[idx+2:]
			}
			chunks[i].Content = "[...] " + overlap + "\n\n" + chunks[i].Content
		}
	}
	return chunks
}

// ============================================================================
// RAW CONTENT CHUNKING (for unstructured files)
// ============================================================================

// ChunkRawContent chunks arbitrary unstructured text content for RAG indexing.
// This is used when content doesn't have structured annotations (KB.MD/POLICY.MD format)
// and should just be chunked for retrieval.
//
// Deprecated: Use ChunkMarkdownContent directly instead. This function auto-detects
// markdown headers and delegates to ChunkMarkdownContent anyway. For plain text,
// it falls back to paragraph-based chunking which is less reliable.
//
// Parameters:
//   - content: The raw text content to chunk
//   - contentType: A label for the content type (e.g., "raw_kb", "raw_policy", "document")
//   - tenantID: The tenant this content belongs to
//   - audience: The audience type
//   - sourceVersion: Version number for the content
//   - maxTokens: Maximum tokens per chunk (use GetMaxChunkTokens)
func (c *Chunker) ChunkRawContent(
	content string,
	contentType string,
	tenantID int32,
	audience string,
	sourceVersion int32,
	maxTokens int,
) []DocumentChunk {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	// Use defaults if not specified
	if maxTokens <= 0 {
		maxTokens = MaxChunkTokens
	}
	minTokens := maxTokens / 5
	if minTokens < 30 {
		minTokens = 30
	}

	now := time.Now()
	var chunks []DocumentChunk

	// Try to detect document structure and use appropriate chunking
	hasMarkdownHeaders := strings.Contains(content, "\n## ") || strings.HasPrefix(content, "## ") ||
		strings.Contains(content, "\n# ") || strings.HasPrefix(content, "# ")

	if hasMarkdownHeaders {
		// Use heading-based chunking for markdown-like content
		return c.ChunkMarkdownContent(content, tenantID, audience, contentType, sourceVersion, maxTokens)
	}

	// For plain text, use paragraph-based chunking
	paragraphs := splitIntoParagraphs(content)

	var currentContent strings.Builder
	chunkIndex := 0

	flushChunk := func(title string) {
		if currentContent.Len() > 0 {
			code := fmt.Sprintf("%s_chunk_%d", contentType, chunkIndex)
			chunks = append(chunks, DocumentChunk{
				ID:            ChunkID(tenantID, audience, contentType, code),
				TenantID:      tenantID,
				AudienceType:  audience,
				ContentType:   contentType,
				Title:         title,
				Content:       strings.TrimSpace(currentContent.String()),
				Code:          code,
				IsActive:      true,
				SourceVersion: sourceVersion,
				IndexedAt:     now,
			})
			currentContent.Reset()
			chunkIndex++
		}
	}

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraTokens := EstimateTokens(para)

		// If single paragraph exceeds limit, split by sentences
		if paraTokens > maxTokens {
			// First flush any existing content
			if currentContent.Len() > 0 {
				flushChunk(fmt.Sprintf("Content Part %d", chunkIndex+1))
			}

			// Split large paragraph into sentence-based chunks
			sentences := splitBySentences(para)
			var sentenceBuffer strings.Builder

			for _, sent := range sentences {
				combined := sentenceBuffer.String()
				if combined != "" {
					combined += " "
				}
				combined += sent

				if EstimateTokens(combined) > maxTokens && sentenceBuffer.Len() > 0 {
					// Save current buffer
					code := fmt.Sprintf("%s_chunk_%d", contentType, chunkIndex)
					chunks = append(chunks, DocumentChunk{
						ID:            ChunkID(tenantID, audience, contentType, code),
						TenantID:      tenantID,
						AudienceType:  audience,
						ContentType:   contentType,
						Title:         fmt.Sprintf("Content Part %d", chunkIndex+1),
						Content:       strings.TrimSpace(sentenceBuffer.String()),
						Code:          code,
						IsActive:      true,
						SourceVersion: sourceVersion,
						IndexedAt:     now,
					})
					chunkIndex++
					sentenceBuffer.Reset()
					sentenceBuffer.WriteString(sent)
				} else {
					if sentenceBuffer.Len() > 0 {
						sentenceBuffer.WriteString(" ")
					}
					sentenceBuffer.WriteString(sent)
				}
			}

			// Flush remaining sentences
			if sentenceBuffer.Len() > 0 {
				code := fmt.Sprintf("%s_chunk_%d", contentType, chunkIndex)
				chunks = append(chunks, DocumentChunk{
					ID:            ChunkID(tenantID, audience, contentType, code),
					TenantID:      tenantID,
					AudienceType:  audience,
					ContentType:   contentType,
					Title:         fmt.Sprintf("Content Part %d", chunkIndex+1),
					Content:       strings.TrimSpace(sentenceBuffer.String()),
					Code:          code,
					IsActive:      true,
					SourceVersion: sourceVersion,
					IndexedAt:     now,
				})
				chunkIndex++
			}
			continue
		}

		// Normal paragraph processing
		combined := currentContent.String()
		if combined != "" {
			combined += "\n\n"
		}
		combined += para

		if EstimateTokens(combined) > maxTokens && currentContent.Len() > 0 {
			flushChunk(fmt.Sprintf("Content Part %d", chunkIndex+1))
			currentContent.WriteString(para)
		} else {
			if currentContent.Len() > 0 {
				currentContent.WriteString("\n\n")
			}
			currentContent.WriteString(para)
		}
	}

	// Don't forget the last chunk
	if currentContent.Len() > 0 {
		flushChunk(fmt.Sprintf("Content Part %d", chunkIndex+1))
	}

	// Merge small chunks
	chunks = mergeSmallChunks(chunks, minTokens, maxTokens)

	return chunks
}

// splitIntoParagraphs splits text by blank lines (double newlines).
func splitIntoParagraphs(content string) []string {
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Split by double newlines (blank lines)
	paragraphs := strings.Split(content, "\n\n")

	// Filter empty paragraphs
	var result []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}

	return result
}
