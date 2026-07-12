package agent

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
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
	DefaultTokenThreshold   = 30000 // Threshold for switching to RAG mode
	MinChunkTokens          = 30    // Minimum tokens per chunk
	MaxChunkTokens          = 150   // Default max tokens (for local)
	ChunkOverlapTokens      = 50    // Overlap between chunks for context continuity
	MaxEmbeddingInputTokens = 8000  // Safety limit: pre-embedding guard splits chunks > this
)

// GetMaxChunkTokens returns the maximum chunk size based on embedding provider.
// With the real tokenizer (cl100k_base), counts are exact, so we target the
// embedding quality sweet spot rather than compensating for heuristic undercount.
//
// Larger chunks reduce the total chunk count (and therefore the number of
// embedding API calls) during reindex — the dominant reindex cost for large
// KBs. openrouter now defaults to 1024 tokens (balanced: fewer calls with only
// a slight retrieval-precision tradeoff); text-embedding-3-small accepts up to
// 8191 tokens/input so 1024 is well within bounds.
//
// RAG_MAX_CHUNK_TOKENS, if set (100–8000), overrides the per-provider default
// and lets each deployment tune chunk size without code changes.
func GetMaxChunkTokens(embeddingProvider string) int {
	if v := os.Getenv("RAG_MAX_CHUNK_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 100 && n <= 8000 {
			return n
		}
	}
	switch embeddingProvider {
	case "openrouter":
		return 1024 // Balanced: fewer chunks/API calls, slight precision tradeoff
	case "local":
		return 150 // 512 token limit with aggressive subword tokenization
	case "mock":
		return 500 // Mock doesn't have real limits
	default:
		return 500
	}
}

// GetMinChunkTokens returns the minimum chunk size based on embedding provider.
func GetMinChunkTokens(embeddingProvider string) int {
	switch embeddingProvider {
	case "openrouter":
		return 100 // Scaled proportionally from 512 max
	case "local":
		return 30
	default:
		return 50
	}
}

// EstimateTokens returns the exact token count for a given text using the
// embedding model's real tokenizer (cl100k_base). If the tokenizer was not
// initialized at startup, it attempts a one-time on-demand init from the
// captured embedding config (Plan 8 / R4). Only if that also fails does it
// fall back to the len/4 heuristic, and it logs an ERROR (not a warning) so the
// misconfiguration is never silently masked.
func EstimateTokens(content string) int {
	if globalTokenizer == nil {
		maybeInitTokenizer()
	}
	if globalTokenizer != nil {
		count, err := globalTokenizer.Count(content)
		if err == nil {
			return count
		}
	}
	fallbackWarnOnce.Do(func() {
		slog.Error("EstimateTokens using len/4 fallback — globalTokenizer not initialized",
			"contentLength", len(content))
	})
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

	// Recursive split: tries H2 → H3 → paragraph → sentence → hard limit
	parts := splitContent(content, maxTokens)
	var chunks []DocumentChunk
	now := time.Now()
	for i, part := range parts {
		title, body := extractTitleAndBody(part)
		if strings.TrimSpace(body) == "" {
			continue
		}
		// Flat chunk ID format (full-reindex-safe; Delete is called before Insert)
		code := fmt.Sprintf("%s_chunk_%d", fileType, i)
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

	// Final guard: split any chunk that still exceeds maxTokens (e.g., from
	// the splitByParagraphs escape hatch). This runs before addChunkOverlap
	// so overlap inflation doesn't cause false triggers.
	var guardedChunks []DocumentChunk
	for _, chunk := range chunks {
		if EstimateTokens(chunk.Content) > maxTokens {
			slog.Warn("Chunk exceeded maxTokens, splitting",
				"actualTokens", EstimateTokens(chunk.Content),
				"maxTokens", maxTokens,
				"title", chunk.Title,
				"contentLength", len(chunk.Content),
				"contentPreview", chunk.Content[:min(200, len(chunk.Content))])
			parts := splitByHardLimit(chunk.Content, maxTokens)
			for p, part := range parts {
				code := fmt.Sprintf("%s_guard_%d", chunk.Code, p+1)
				guardedChunks = append(guardedChunks, DocumentChunk{
					ID:            ChunkID(chunk.TenantID, chunk.AudienceType, chunk.ContentType, code),
					TenantID:      chunk.TenantID,
					AudienceType:  chunk.AudienceType,
					ContentType:   chunk.ContentType,
					Title:         fmt.Sprintf("%s (Part %d)", chunk.Title, p+1),
					Content:       part,
					Code:          code,
					IsActive:      true,
					SourceVersion: chunk.SourceVersion,
					IndexedAt:     chunk.IndexedAt,
				})
			}
		} else {
			guardedChunks = append(guardedChunks, chunk)
		}
	}
	chunks = guardedChunks

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

// paragraphChunk removed in favor of returning []string from splitByParagraphs;
// title extraction is handled by the caller (extractTitleAndBody).

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

// splitByParagraphs splits content by blank lines and accumulates paragraphs
// into groups that approach maxTokens. Returns []string — title extraction
// is handled by the caller (extractTitleAndBody).
// Inline sentence/hard-limit fallbacks removed; recursion in splitContent
// handles oversized paragraphs through splitBySentences → splitByHardLimit.
func splitByParagraphs(content string, maxTokens int) []string {
	paragraphs := strings.Split(content, "\n\n")
	var result []string
	var buf strings.Builder
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		combined := buf.String()
		if combined != "" {
			combined += "\n\n"
		}
		combined += para
		if EstimateTokens(combined) > maxTokens && buf.Len() > 0 {
			result = append(result, strings.TrimSpace(buf.String()))
			buf.Reset()
		}
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(para)
	}
	if buf.Len() > 0 {
		result = append(result, strings.TrimSpace(buf.String()))
	}
	return result
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

// splitByHardLimit splits text using the real tokenizer to ensure each part
// fits within maxTokens. Uses binary search to find optimal split points.
// This is a last resort when all other splitting methods fail.
func splitByHardLimit(text string, maxTokens int) []string {
	var parts []string
	runes := []rune(text)
	start := 0
	for start < len(runes) {
		remainder := string(runes[start:])
		if EstimateTokens(remainder) <= maxTokens {
			parts = append(parts, remainder)
			break
		}
		// Binary search for the split point
		lo, hi := start+1, len(runes)
		for lo < hi {
			mid := (lo + hi) / 2
			if EstimateTokens(string(runes[start:mid])) <= maxTokens {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo-1 > start {
			parts = append(parts, string(runes[start:lo-1]))
			start = lo - 1
		} else {
			// Single rune exceeds maxTokens (edge case)
			parts = append(parts, string(runes[start:start+1]))
			start++
		}
	}
	return parts
}

// splitContent recursively splits content using a chain of strategies:
// H2 headers → H3 headers → paragraph accumulation → sentences → hard limit.
// Each strategy is tried in order; the first that produces multiple parts
// triggers recursion on each part. Falls back to splitByHardLimit.
func splitContent(content string, maxTokens int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return []string{content}
	}
	if parts := splitByH2Headers(content); len(parts) > 1 {
		return splitParts(parts, maxTokens)
	}
	if parts := splitByH3Headers(content); len(parts) > 1 {
		return splitParts(parts, maxTokens)
	}
	if parts := splitByParagraphs(content, maxTokens); len(parts) > 1 {
		return splitParts(parts, maxTokens)
	}
	if parts := splitBySentences(content); len(parts) > 1 {
		return splitParts(parts, maxTokens)
	}
	return splitByHardLimit(content, maxTokens)
}

// splitParts recursively splits multiple content parts.
func splitParts(parts []string, maxTokens int) []string {
	var result []string
	for _, part := range parts {
		result = append(result, splitContent(part, maxTokens)...)
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
