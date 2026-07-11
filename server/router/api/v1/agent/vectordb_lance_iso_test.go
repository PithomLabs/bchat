//go:build rag

package agent

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
)

// failingEmbedSvc fails the whole-batch call (to trigger the isolation path)
// and fails any single input that starts with "fail".
type failingEmbedSvc struct {
	dimension int
}

func (f *failingEmbedSvc) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) > 1 {
		return nil, fmt.Errorf("simulated batch failure")
	}
	if strings.HasPrefix(texts[0], "fail") {
		return nil, fmt.Errorf("simulated item failure")
	}
	emb := make([]float32, f.dimension)
	emb[0] = 1.0
	return [][]float32{emb}, nil
}
func (f *failingEmbedSvc) Dimension() int     { return f.dimension }
func (f *failingEmbedSvc) Provider() string    { return "mock" }
func (f *failingEmbedSvc) MaxInputTokens() int { return math.MaxInt32 }

// TestEmbedWithIsolationSkipsFailedItem verifies Plan 8 / R3: when a batch
// embed fails, items are retried individually and only the failing item is
// skipped (marked failed), while the rest are embedded successfully.
func TestEmbedWithIsolationSkipsFailedItem(t *testing.T) {
	db := &LanceVectorDB{embedSvc: &failingEmbedSvc{dimension: 3}}
	texts := []string{"ok-a", "fail-b", "ok-c"}

	embeddings, failed := db.embedWithIsolation(context.Background(), texts)

	if !failed[1] {
		t.Fatalf("expected item 1 (fail-b) to be marked failed")
	}
	if failed[0] || failed[2] {
		t.Fatalf("items 0 and 2 should have succeeded, got failed=%v", failed)
	}
	if embeddings[0] == nil || embeddings[2] == nil {
		t.Fatalf("successful items should have embeddings")
	}
	if embeddings[1] != nil {
		t.Fatalf("failed item should have nil embedding")
	}
	if len(embeddings) != len(texts) {
		t.Fatalf("expected %d slots, got %d", len(texts), len(embeddings))
	}
}
