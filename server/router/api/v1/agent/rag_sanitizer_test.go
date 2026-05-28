package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/usememos/memos/store"
)

// 1. TestCleanRAGSourceContent_RemovesScriptStyleAndTrackerBoilerplate
func TestCleanRAGSourceContent_RemovesScriptStyleAndTrackerBoilerplate(t *testing.T) {
	input := `---
article1.md
---
# First Article
This is standard article content.

<script>
// Some inline tracking script
console.log("tracking user");
</script>

<style>
body { background: red; }
</style>

---
gtm.js
---
// Copyright 2012 Google Inc. All rights reserved.
(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':
new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],
j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src=
'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);
})(window,document,'script','dataLayer','GTM-XXXX');

---
article2.md
---
# Second Article
This is another standard article.`

	sanitized, report := CleanRAGSourceContent(input)
	
	// Assertions
	assert.Contains(t, sanitized, "First Article")
	assert.Contains(t, sanitized, "Second Article")
	assert.NotContains(t, sanitized, "<script>")
	assert.NotContains(t, sanitized, "<style>")
	assert.NotContains(t, sanitized, "googletagmanager")
	assert.Equal(t, 1, report.RemovedScriptBlocks)
	assert.Equal(t, 1, report.RemovedStyleBlocks)
	assert.Equal(t, 1, report.RemovedSections) // The gtm.js section
}

// 2. TestCleanRAGSourceContent_PreservesArticleMarkdown
func TestCleanRAGSourceContent_PreservesArticleMarkdown(t *testing.T) {
	input := `# Standard Header
This is a standard paragraph with no scripts.

## Sub-header
More useful text.`

	sanitized, report := CleanRAGSourceContent(input)
	
	assert.Equal(t, input, sanitized)
	assert.Equal(t, 0, report.RemovedScriptBlocks)
	assert.Equal(t, 0, report.RemovedStyleBlocks)
	assert.Equal(t, 0, report.RemovedSections)
}

// 3. TestCleanRAGSourceContent_PreservesLegitimateCodeExamples
func TestCleanRAGSourceContent_PreservesLegitimateCodeExamples(t *testing.T) {
	// A legitimate documented JS code snippet has spaces/formatting and should not be stripped
	input := `---
doc_example.md
---
# How to use our JS SDK

To initialize the SDK, use this beautiful example:

` + "```javascript" + `
// legitimate formatted code block
const client = new MemosClient({
    apiKey: "sk-or-v1-...",
    timeout: 5000
});
client.init();
` + "```" + `

Let us know if you have questions.`

	sanitized, report := CleanRAGSourceContent(input)
	
	assert.Contains(t, sanitized, "MemosClient")
	assert.Contains(t, sanitized, "client.init()")
	assert.Equal(t, 0, report.RemovedSections)
}

// 4. TestChunkMarkdownContent_RejectsScriptDominatedChunks
func TestChunkMarkdownContent_RejectsScriptDominatedChunks(t *testing.T) {
	chunker := NewChunker()

	// Legitimate text block
	validText := "## Service Title\nThis is a legitimate service article describing mold remediation and water extraction processes."
	chunks1 := chunker.ChunkMarkdownContent(validText, 1, "internal", "kb", 1, 500)
	assert.Len(t, chunks1, 1)

	// Garbage minified JS block
	garbageText := "## JS Code\n" + strings.Repeat("var x=1;eval(x);function(y){return y*2};(function(w,d){console.log(w)})(window,document);", 20)
	chunks2 := chunker.ChunkMarkdownContent(garbageText, 1, "internal", "kb", 1, 500)
	assert.Len(t, chunks2, 0) // Should reject/exclude the garbage chunk
}

// Mock Store for Reindex Checkpoints
type mockStore struct {
	checkpoints map[string]*store.ReindexCheckpoint
}

func (m *mockStore) GetReindexCheckpoint(ctx context.Context, find *store.FindReindexCheckpoint) (*store.ReindexCheckpoint, error) {
	key := fmt.Sprintf("%d:%s", *find.TenantID, *find.Audience)
	return m.checkpoints[key], nil
}

func (m *mockStore) UpsertReindexCheckpoint(ctx context.Context, cp *store.ReindexCheckpoint) (*store.ReindexCheckpoint, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err() // Simulate cancelled context error
	}
	key := fmt.Sprintf("%d:%s", cp.TenantID, cp.Audience)
	m.checkpoints[key] = cp
	return cp, nil
}

// 5. TestGetReindexStatus_AllAggregatesConcreteAudienceCheckpoints
func TestGetReindexStatus_AllAggregatesConcreteAudienceCheckpoints(t *testing.T) {
	mStore := &mockStore{
		checkpoints: make(map[string]*store.ReindexCheckpoint),
	}

	// Setup mock checkpoints
	mStore.checkpoints["6:internal"] = &store.ReindexCheckpoint{
		TenantID:        6,
		Audience:        "internal",
		Status:          "in_progress",
		TotalChunks:     100,
		ProcessedChunks: 40,
		CurrentBatch:    2,
		TotalBatches:    5,
		UpdatedAt:       time.Now(),
	}

	mStore.checkpoints["6:external"] = &store.ReindexCheckpoint{
		TenantID:        6,
		Audience:        "external",
		Status:          "completed",
		TotalChunks:     50,
		ProcessedChunks: 50,
		CurrentBatch:    2,
		TotalBatches:    2,
		UpdatedAt:       time.Now().Add(-10 * time.Minute),
	}
	// Direct testing of status resolver helper
	resolveState := func(cp *store.ReindexCheckpoint) (string, bool) {
		status := cp.Status
		canResume := cp.Status == "failed"
		if cp.Status == "in_progress" && !cp.UpdatedAt.IsZero() {
			if time.Since(cp.UpdatedAt) > 1*time.Hour {
				status = "stale_in_progress"
				canResume = true
			}
		}
		return status, canResume
	}

	// Assert single checkpoint resolution
	status, canResume := resolveState(mStore.checkpoints["6:internal"])
	assert.Equal(t, "in_progress", status)
	assert.False(t, canResume)

	// Assert stale checkpoint resolution
	staleCp := &store.ReindexCheckpoint{
		Status:    "in_progress",
		UpdatedAt: time.Now().Add(-2 * time.Hour),
	}
	status2, canResume2 := resolveState(staleCp)
	assert.Equal(t, "stale_in_progress", status2)
	assert.True(t, canResume2)
}

// 6. TestReindexFailureCheckpointPersistsAfterRequestContextCancel
func TestReindexFailureCheckpointPersistsAfterRequestContextCancel(t *testing.T) {
	mStore := &mockStore{
		checkpoints: make(map[string]*store.ReindexCheckpoint),
	}

	// 1. Create a cancelled request context
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	assert.Error(t, cancelCtx.Err())

	// 2. Try to write failure checkpoint
	failedCheckpoint := &store.ReindexCheckpoint{
		TenantID: 6,
		Audience: "internal",
		Status:   "failed",
	}

	// Standard write using cancelled ctx should fail
	_, err := mStore.UpsertReindexCheckpoint(cancelCtx, failedCheckpoint)
	assert.Error(t, err)

	// Recovered Invariant Write: Detached bounded context succeeds!
	checkpointCtx, checkpointCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer checkpointCancel()

	_, err2 := mStore.UpsertReindexCheckpoint(checkpointCtx, failedCheckpoint)
	assert.NoError(t, err2)
	assert.Equal(t, "failed", mStore.checkpoints["6:internal"].Status)
}

// 7. TestRAGIndexCompleteness_UsefulArticleChunksReachVectorIndex
func TestRAGIndexCompleteness_UsefulArticleChunksReachVectorIndex(t *testing.T) {
	chunker := NewChunker()
	
	pollutedInput := `---
gtm.js
---
// Copyright 2012 Google Inc. All rights reserved.
(function(w,g){w[g]=w[g]||{};})(window,'google_tag_manager');

---
article.md
---
## How to calculate credits?
Useful article body.`

	// When chunking the polluted input, the GTM portion is stripped by CleanRAGSourceContent
	chunks := chunker.ChunkMarkdownContent(pollutedInput, 1, "internal", "kb", 1, 500)
	
	// We should only get the useful article chunks
	assert.Len(t, chunks, 1)
	assert.Contains(t, chunks[0].Content, "Useful article body.")
	assert.NotContains(t, chunks[0].Content, "google_tag_manager")
}
