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
