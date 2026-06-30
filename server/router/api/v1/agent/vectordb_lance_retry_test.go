//go:build rag

package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestLanceVectorDBProcessBatchWithRetryShortCircuitsPermanentError(t *testing.T) {
	embedSvc := &mockEmbeddingService{
		dimension: 8,
		err:       fmt.Errorf("%w: bad key", ErrEmbeddingProviderMisconfigured),
	}
	db := &LanceVectorDB{embedSvc: embedSvc}

	err := db.processBatchWithRetry(context.Background(), []DocumentChunk{
		{
			ID:       "chunk-1",
			TenantID: 1,
			Title:    "Title",
			Content:  "Content",
		},
	}, 1, 3, time.Millisecond)
	if !errors.Is(err, ErrEmbeddingProviderMisconfigured) {
		t.Fatalf("processBatchWithRetry() error = %v, want ErrEmbeddingProviderMisconfigured", err)
	}
	if embedSvc.calls != 1 {
		t.Fatalf("Embed calls = %d, want 1", embedSvc.calls)
	}
}
