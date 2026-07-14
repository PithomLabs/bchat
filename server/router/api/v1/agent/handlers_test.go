package agent

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestReindexHTTPErrorMapsSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "embedding misconfigured",
			err:  fmt.Errorf("%w: bad key", ErrEmbeddingProviderMisconfigured),
			want: http.StatusBadRequest,
		},
		{
			name: "embedding unavailable",
			err:  fmt.Errorf("%w: upstream timeout", ErrEmbeddingProviderUnavailable),
			want: http.StatusServiceUnavailable,
		},
		{
			name: "vector store unavailable",
			err:  fmt.Errorf("%w: table unavailable", ErrVectorStoreUnavailable),
			want: http.StatusInternalServerError,
		},
		{
			name: "unknown",
			err:  errors.New("unexpected"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpErr := reindexHTTPError(tt.err)
			if httpErr.Code != tt.want {
				t.Fatalf("status = %d, want %d", httpErr.Code, tt.want)
			}
		})
	}
}

func TestSkipReasonConstants(t *testing.T) {
	if SkipReasonNone != "" {
		t.Fatalf("SkipReasonNone = %q, want empty string", SkipReasonNone)
	}
	if SkipReasonLongContext != "long_context" {
		t.Fatalf("SkipReasonLongContext = %q, want %q", SkipReasonLongContext, "long_context")
	}
	if SkipReasonNoSourceFiles != "no_source_files" {
		t.Fatalf("SkipReasonNoSourceFiles = %q, want %q", SkipReasonNoSourceFiles, "no_source_files")
	}
	if SkipReasonPipelineDisabled != "pipeline_disabled" {
		t.Fatalf("SkipReasonPipelineDisabled = %q, want %q", SkipReasonPipelineDisabled, "pipeline_disabled")
	}
}
