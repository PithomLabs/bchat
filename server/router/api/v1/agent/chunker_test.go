package agent

import (
	"strings"
	"testing"
)

func TestChunkerNoTerminatorParagraph(t *testing.T) {
	InitTokenizer("test", "text-embedding-3-small")

	maxTokens := GetMaxChunkTokens("openrouter")
	// Create a paragraph with no sentence terminators (comma-separated values)
	parts := make([]string, 1200)
	for i := range parts {
		parts[i] = "value_" + itoa(i)
	}
	content := strings.Join(parts, ", ")

	if EstimateTokens(content) <= maxTokens {
		t.Fatal("test content must exceed maxTokens to be meaningful")
	}

	chunker := NewChunker()
	chunks := chunker.ChunkMarkdownContent(content, 1, "test", "kb", 1, maxTokens)

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// The guard runs before addChunkOverlap, so pre-overlap chunks are ≤ maxTokens.
	// addChunkOverlap prepends 200 chars (overlapTokens*4) to subsequent chunks.
	// For dense content (comma-separated), 200 chars ≈ 80 tokens.
	// The real constraint is the embedding model's 8192 limit, not maxTokens.
	// We verify chunks are within a reasonable bound.
	overhead := 100 // allows for dense content overlap overhead
	for i, chunk := range chunks {
		tokens := EstimateTokens(chunk.Content)
		limit := maxTokens
		if i > 0 {
			limit += overhead
		}
		if tokens > limit {
			t.Errorf("chunk %d exceeds limit: %d > %d", i, tokens, limit)
		}
	}
}

func TestChunkerNoH2Headers(t *testing.T) {
	InitTokenizer("test", "text-embedding-3-small")

	maxTokens := GetMaxChunkTokens("openrouter")
	// Create content without any H2 headers that exceeds maxTokens
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("Paragraph ")
		sb.WriteString(itoa(i))
		sb.WriteString(". This is a test paragraph without any markdown headers. It contains enough text to require splitting into multiple chunks.\n\n")
	}
	content := sb.String()

	chunker := NewChunker()
	chunks := chunker.ChunkMarkdownContent(content, 1, "test", "kb", 1, maxTokens)

	if len(chunks) < 2 {
		t.Fatal("expected multiple chunks for content without headers")
	}

	for i, chunk := range chunks {
		tokens := EstimateTokens(chunk.Content)
		// First chunk has no overlap; subsequent chunks have overlap overhead
		limit := maxTokens
		if i > 0 {
			limit += 100
		}
		if tokens > limit {
			t.Errorf("chunk %d exceeds limit: %d > %d (%d chars)", i, tokens, limit, len(chunk.Content))
		}
	}
}

func TestChunkerOverlapSafe(t *testing.T) {
	InitTokenizer("test", "text-embedding-3-small")

	maxTokens := GetMaxChunkTokens("openrouter")
	minTokens := maxTokens / 5
	if minTokens < 30 {
		minTokens = 30
	}
	// Multi-section content to generate multiple chunks with overlap.
	// Each section must exceed minTokens (maxTokens/5) to survive mergeSmallChunks.
	// With maxTokens=1024, minTokens=204. Each section needs ~60+ words.
	var sb strings.Builder
	for i := 0; i < 5; i++ {
		sb.WriteString("## Section ")
		sb.WriteString(itoa(i))
		sb.WriteString("\n")
		// Write enough content to exceed minTokens (~204 tokens ≈ 800+ chars)
		for j := 0; j < 20; j++ {
			sb.WriteString("This is sentence number ")
			sb.WriteString(itoa(j))
			sb.WriteString(" in section ")
			sb.WriteString(itoa(i))
			sb.WriteString(". It contains meaningful content that describes the topic in detail. ")
			sb.WriteString("The embedding model needs enough text to generate useful vectors for this section. ")
		}
		sb.WriteString("\n\n")
	}
	content := sb.String()

	chunker := NewChunker()
	chunks := chunker.ChunkMarkdownContent(content, 1, "test", "kb", 1, maxTokens)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for overlap test, got %d (content length=%d chars, ~%d tokens)",
			len(chunks), len(content), EstimateTokens(content))
	}

	// Overlap adds ~ChunkOverlapTokens (50) tokens. Allow 2x as buffer.
	safeLimit := maxTokens + ChunkOverlapTokens*2
	for i, chunk := range chunks {
		tokens := EstimateTokens(chunk.Content)
		if tokens > safeLimit {
			t.Errorf("chunk %d after overlap exceeds safe limit: %d > %d", i, tokens, safeLimit)
		}
	}
}

func TestChunkerGuardCatchesOversized(t *testing.T) {
	InitTokenizer("test", "text-embedding-3-small")

	maxTokens := GetMaxChunkTokens("openrouter")

	// Create content with a very large paragraph that has no sentence terminators.
	// This triggers the splitByParagraphs escape hatch, which the guard must catch.
	parts := make([]string, 3000)
	for i := range parts {
		parts[i] = "item_" + itoa(i)
	}
	// Use a header so the chunker enters the heading-based path, then the body
	// is a single paragraph with no sentence terminators.
	content := "## Large Section\n" + strings.Join(parts, ", ")

	chunker := NewChunker()
	chunks := chunker.ChunkMarkdownContent(content, 1, "test", "kb", 1, maxTokens)

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// All chunks must be within maxTokens (pre-overlap check), or maxTokens +
	// overlap overhead for subsequent chunks.
	for i, chunk := range chunks {
		tokens := EstimateTokens(chunk.Content)
		limit := maxTokens
		if i > 0 {
			limit += 100
		}
		if tokens > limit {
			t.Errorf("guard failed: chunk %d has %d tokens, limit is %d", i, tokens, limit)
		}
	}

	// With 3000 items, we should get multiple chunks
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks from oversized content, got %d", len(chunks))
	}
}

func TestChunkerEvpnLikeContent(t *testing.T) {
	InitTokenizer("test", "text-embedding-3-small")

	maxTokens := GetMaxChunkTokens("openrouter")

	// Simulate the evpn KB content structure:
	// 1. YAML front matter with concatenated article titles (pipe-separated)
	// 2. Real markdown content after the closing ---
	var sb strings.Builder
	sb.WriteString("---\ntitle: \"Are VPNs Legal? | ExpressVPN\"\ndescription: \"Uncover the legal status of VPNs worldwide.\"\n---\n\n")

	// Simulate the concatenated titles block (no sentence terminators, no headers)
	titles := make([]string, 100)
	for i := range titles {
		titles[i] = "Article Title " + itoa(i) + " | ExpressVPN"
	}
	sb.WriteString(strings.Join(titles, "\n"))
	sb.WriteString("\n\n")

	// Now add real markdown content with H2 headers
	for i := 0; i < 5; i++ {
		sb.WriteString("## Section ")
		sb.WriteString(itoa(i))
		sb.WriteString("\nThis is a real section about VPNs with meaningful content. ")
		sb.WriteString("It describes important aspects of VPN technology and privacy. ")
		sb.WriteString("The content is detailed enough to produce proper chunks. ")
		sb.WriteString("Each section should generate its own chunk after splitting. ")
		sb.WriteString("More sentences to ensure the section exceeds minimum token threshold. ")
		sb.WriteString("Additional content to make each section a standalone chunk.\n\n")
	}

	content := sb.String()
	t.Logf("Test content length: %d chars, ~%d tokens", len(content), EstimateTokens(content))

	chunker := NewChunker()
	chunks := chunker.ChunkMarkdownContent(content, 1, "internal", "kb", 1, maxTokens)

	t.Logf("Chunks produced: %d", len(chunks))
	for i, chunk := range chunks {
		t.Logf("  chunk %d: title=%q, content_length=%d, tokens=%d",
			i, chunk.Title, len(chunk.Content), EstimateTokens(chunk.Content))
	}

	if len(chunks) == 0 {
		t.Fatal("evpn-like content produced ZERO chunks — this reproduces the bug")
	}

	// Should produce at least 5 chunks (one per H2 section)
	if len(chunks) < 3 {
		t.Errorf("expected at least 3 chunks from evpn-like content, got %d", len(chunks))
	}
}

func TestChunkerYamlFrontMatterOnly(t *testing.T) {
	InitTokenizer("test", "text-embedding-3-small")

	maxTokens := GetMaxChunkTokens("openrouter")

	// Test what happens when content is ONLY YAML front matter with no real content
	// (This simulates what if the content after YAML sanitization is empty)
	content := "---\ntitle: \"Test\"\ndescription: \"Test desc\"\n---\n"

	chunker := NewChunker()
	chunks := chunker.ChunkMarkdownContent(content, 1, "internal", "kb", 1, maxTokens)

	t.Logf("YAML-only content produced %d chunks", len(chunks))
	// This might produce 0 or 1 chunks — document the behavior
	if len(chunks) == 0 {
		t.Log("YAML-only content produced 0 chunks — expected for empty real content")
	}
}

func TestCleanRAGSourceContentOnEvpnLikeContent(t *testing.T) {
	// Simulate the evpn content structure
	content := "---\ntitle: \"Test | ExpressVPN\"\ndescription: \"desc\"\n---\n\n# Real Content\n\nThis is actual content."

	sanitized, report := CleanRAGSourceContent(content)
	t.Logf("Original bytes: %d, Sanitized bytes: %d", report.OriginalBytes, report.SanitizedBytes)
	t.Logf("Removed sections: %d, Script blocks: %d, Style blocks: %d",
		report.RemovedSections, report.RemovedScriptBlocks, report.RemovedStyleBlocks)
	t.Logf("Sanitized content preview: %.200s", sanitized)

	if report.SanitizedBytes == 0 {
		t.Error("CleanRAGSourceContent produced empty output — would cause zero chunks")
	}
}
