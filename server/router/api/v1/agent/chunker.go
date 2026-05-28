package agent

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
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
		return 1000 // Reduced from 2000 to 1000 to be safe for Qwen (likely 8k limit)
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

// RAGSanitizeReport holds diagnostic details of the sanitization process.
type RAGSanitizeReport struct {
	OriginalBytes       int
	SanitizedBytes      int
	RemovedSections     int
	RemovedScriptBlocks int
	RemovedStyleBlocks  int
	RejectedChunks      int
}

var (
	// Regex to match <script>...</script> tags (case-insensitive, multi-line/dot matches newline)
	scriptRegex = regexp.MustCompile(`(?is)<script[^>]*?>.*?</script>`)
	// Regex to match <style>...</style> tags
	styleRegex  = regexp.MustCompile(`(?is)<style[^>]*?>.*?</style>`)
	// Regex to match markdown file/section delimiters
	sectionDelimiterRegex = regexp.MustCompile(`(?m)^---\n([a-zA-Z0-9_\-\./]+)\n---\n`)
)

// CleanRAGSourceContent removes script, style, tracking, and minified boilerplate code
// before chunking and vector indexing to satisfy the recovered invariant:
// INV_RAG_SOURCE_CONTENT_MUST_BE_CANONICAL_BEFORE_CHUNKING.
func CleanRAGSourceContent(content string) (string, RAGSanitizeReport) {
	var report RAGSanitizeReport
	report.OriginalBytes = len(content)

	// 1. Remove HTML script and style elements
	scriptMatches := scriptRegex.FindAllStringIndex(content, -1)
	report.RemovedScriptBlocks = len(scriptMatches)
	content = scriptRegex.ReplaceAllString(content, "")

	styleMatches := styleRegex.FindAllStringIndex(content, -1)
	report.RemovedStyleBlocks = len(styleMatches)
	content = styleRegex.ReplaceAllString(content, "")

	// 2. Split content by the markdown file/section delimiters
	locs := sectionDelimiterRegex.FindAllStringSubmatchIndex(content, -1)
	if len(locs) == 0 {
		if isBoilerplateBlock("", content) {
			report.RemovedSections = 1
			report.SanitizedBytes = 0
			return "", report
		}
		report.SanitizedBytes = len(content)
		return content, report
	}

	var sb strings.Builder
	firstBlock := content[:locs[0][0]]
	if !isBoilerplateBlock("", firstBlock) {
		sb.WriteString(firstBlock)
	} else {
		report.RemovedSections++
	}

	for i := 0; i < len(locs); i++ {
		filePath := content[locs[i][2]:locs[i][3]]
		endOfSection := len(content)
		if i+1 < len(locs) {
			endOfSection = locs[i+1][0]
		}
		sectionStart := locs[i][1]
		sectionBody := content[sectionStart:endOfSection]

		if isBoilerplateBlock(filePath, sectionBody) {
			report.RemovedSections++
			continue // Skip boilerplate section
		}

		// Keep the valid section and its delimiter
		sb.WriteString(content[locs[i][0]:sectionStart])
		sb.WriteString(sectionBody)
	}

	sanitized := sb.String()
	report.SanitizedBytes = len(sanitized)
	return sanitized, report
}

// isBoilerplateBlock checks if a block of content is purely or predominantly tracking script, minified code, or style boilerplate.
func isBoilerplateBlock(filePath, body string) bool {
	filePathLower := strings.ToLower(filePath)
	
	// Preserve legitimate documentation/code-reference paths unless it is raw/minified/tracker-like
	if strings.Contains(filePathLower, "googletagmanager") || 
		strings.Contains(filePathLower, "google_tag_manager") || 
		strings.Contains(filePathLower, "google-analytics") {
		return true
	}

	// Path hints combined with body tracker keywords (safe, non-destructive check)
	if (strings.Contains(filePathLower, "gtm") || 
		strings.Contains(filePathLower, "analytics") || 
		strings.Contains(filePathLower, "script") || 
		strings.HasSuffix(filePathLower, ".js")) && 
		(strings.Contains(body, "googletagmanager") || 
			strings.Contains(body, "google_tag_manager") || 
			strings.Contains(body, "dataLayer") || 
			strings.Contains(body, "GTM-")) {
		return true
	}

	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 500 {
			spaces := strings.Count(line, " ")
			spaceRatio := float64(spaces) / float64(len(line))
			
			if spaceRatio < 0.05 {
				// Check for minified JS keywords/signatures
				jsSignatures := []string{"(function(", "eval(", "window.", "document.", "var ", "const ", "let ", "function(", "dataLayer.push("}
				for _, sig := range jsSignatures {
					if strings.Contains(line, sig) {
						return true
					}
				}
				
				// Check for minified CSS signatures
				if strings.Contains(line, "{") && strings.Contains(line, "}") && strings.Contains(line, ";") {
					return true
				}
			}
		}
	}

	return false
}

// IsGarbageChunk checks if a chunk of text is dominated by minified code or script garbage.
func IsGarbageChunk(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 300 {
			spaces := strings.Count(line, " ")
			spaceRatio := float64(spaces) / float64(len(line))
			if spaceRatio < 0.05 {
				// JS / CSS signature
				jsKeywords := []string{"function", "var ", "const ", "let ", "return", "eval", "window.", "document.", ";"}
				for _, kw := range jsKeywords {
					if strings.Contains(line, kw) {
						return true
					}
				}
			}
		}
	}
	return false
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
	// [CODE-LOCAL INVARIANT BOUNDARY COMMENT]
	// INV_RAG_SOURCE_CONTENT_MUST_BE_CANONICAL_BEFORE_CHUNKING:
	// We sanitize and canonicalize incoming raw content at this entrypoint.
	// Raw HTML style/script remnants and minified JS/CSS/tracker boilerplate are
	// stripped before splitting the document to keep the vector database canonical.
	content = sanitizeUTF8(content)
	
	sanitized, report := CleanRAGSourceContent(content)
	if report.RemovedSections > 0 || report.RemovedScriptBlocks > 0 || report.RemovedStyleBlocks > 0 {
		slog.Info("RAG source content sanitized during chunking",
			"tenantID", tenantID,
			"audience", audience,
			"originalBytes", report.OriginalBytes,
			"sanitizedBytes", report.SanitizedBytes,
			"removedSections", report.RemovedSections,
			"removedScriptBlocks", report.RemovedScriptBlocks,
			"removedStyleBlocks", report.RemovedStyleBlocks)
	}
	content = sanitized

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

	// Filter out any garbage/script-dominated chunks to satisfy:
	// INV_RAG_SOURCE_CONTENT_MUST_BE_CANONICAL_BEFORE_CHUNKING.
	var cleanChunks []DocumentChunk
	for _, chunk := range chunks {
		if !IsGarbageChunk(chunk.Content) {
			cleanChunks = append(cleanChunks, chunk)
		} else {
			slog.Warn("RAG: Rejected script-dominated garbage chunk from index",
				"tenantID", tenantID,
				"audience", audience,
				"title", chunk.Title,
				"contentLength", len(chunk.Content))
		}
	}
	chunks = cleanChunks

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
					content := strings.TrimSpace(sentenceBuffer.String())

					// Hard split if still too big (e.g. minified code)
					if EstimateTokens(content) > maxTokens {
						parts := splitByHardLimit(content, maxTokens)
						for _, p := range parts {
							chunks = append(chunks, paragraphChunk{
								title:   fmt.Sprintf("%s (Part %d)", title, len(chunks)+1),
								content: p,
							})
						}
					} else {
						chunks = append(chunks, paragraphChunk{
							title:   fmt.Sprintf("%s (Part %d)", title, len(chunks)+1),
							content: content,
						})
					}

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
				// Append content but keep original title
				pendingChunk.Content += "\n\n" + chunk.Content

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

// splitByHardLimit splits text by character count if no other delimiters exist.
func splitByHardLimit(text string, maxTokens int) []string {
	var parts []string
	// Approximate chars per token = 4. Cap at maxTokens * 4 chars.
	// We use a slightly smaller multiplier (3.5) to be safe.
	maxChars := int(float64(maxTokens) * 3.5)
	if maxChars < 100 {
		maxChars = 100
	}

	runes := []rune(text)
	for i := 0; i < len(runes); i += maxChars {
		end := i + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[i:end]))
	}
	return parts
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
