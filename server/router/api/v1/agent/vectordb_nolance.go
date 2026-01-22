//go:build !rag

package agent

import (
	"fmt"
)

// newLanceVectorDB is a stub when building without the rag tag.
// LanceDB requires CGO and native libraries that are only available in RAG builds.
func newLanceVectorDB(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
	return nil, fmt.Errorf("LanceDB storage (local/s3) requires RAG build. Use 'task build:backend:rag' or set LANCEDB_STORAGE_PROVIDER=memory")
}
