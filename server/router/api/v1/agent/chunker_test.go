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
	// Multi-section content to generate multiple chunks with overlap
	content := `## Section One
This is the first section with enough content to be a chunk by itself.
It contains several sentences that describe the first topic.
The embedding model needs meaningful text to generate useful vectors.
We need enough text here to make this section at least minTokens.
More text to ensure this section stands alone as a proper chunk.
Even more content to push this over the minimum threshold.

## Section Two
This is the second section. It also needs enough content.
The overlap between sections should not push any chunk over maxTokens.
Each chunk should remain within the safety limit after overlap.
More sentences to make this a properly sized chunk.
Additional text to ensure this section is large enough.
And some more content just to be safe.

## Section Three
The third section provides even more content.
Three sections should produce at least three chunks with overlap.
The overlap between adjacent chunks adds context continuity.
Each chunk after the first gets prepended text from the prior chunk.
This means their total token count increases slightly.
We want to verify this doesn't push anything over the limit.
`

	chunker := NewChunker()
	chunks := chunker.ChunkMarkdownContent(content, 1, "test", "kb", 1, maxTokens)

	if len(chunks) < 2 {
		t.Fatal("expected at least 2 chunks for overlap test")
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
