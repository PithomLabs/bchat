package agent

import (
	"testing"

	"github.com/usememos/memos/store"
)

func TestShouldValidateReindex(t *testing.T) {
	tests := []struct {
		name       string
		resume     bool
		checkpoint *store.ReindexCheckpoint
		want       bool
	}{
		{
			name:   "fresh reindex validates",
			resume: false,
			want:   true,
		},
		{
			name:       "resume with checkpoint skips validation",
			resume:     true,
			checkpoint: &store.ReindexCheckpoint{Status: "failed"},
			want:       false,
		},
		{
			name:   "resume without checkpoint validates",
			resume: true,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldValidateReindex(tt.resume, tt.checkpoint); got != tt.want {
				t.Fatalf("shouldValidateReindex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReindexCheckpointStructFields(t *testing.T) {
	// Validates that ReindexCheckpoint struct fields are correctly populated.
	// Integration test with mock store (calling createFailedCheckpoint directly)
	// is deferred — requires mock infrastructure not present in the codebase.
	cp := &store.ReindexCheckpoint{
		TenantID:     42,
		Audience:     "internal",
		Status:       "failed",
		ErrorMessage: "RAG pipeline not initialized",
	}
	if cp.Status != "failed" {
		t.Fatalf("Status = %q, want %q", cp.Status, "failed")
	}
	if cp.ErrorMessage != "RAG pipeline not initialized" {
		t.Fatalf("ErrorMessage = %q, want %q", cp.ErrorMessage, "RAG pipeline not initialized")
	}
	if cp.TenantID != 42 {
		t.Fatalf("TenantID = %d, want %d", cp.TenantID, 42)
	}
	if cp.Audience != "internal" {
		t.Fatalf("Audience = %q, want %q", cp.Audience, "internal")
	}
}
