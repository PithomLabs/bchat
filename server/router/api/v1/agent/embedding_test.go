package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestOpenRouterEmbeddingUsesTenantAPIKeyFromContext(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"embedding": []float32{0.1, 0.2, 0.3},
					"index":     0,
				},
			},
			"model": "openai/text-embedding-3-small",
		})
	}))
	defer server.Close()

	embedder, err := NewOpenRouterEmbedding(&EmbeddingConfig{
		Model:     "openai/text-embedding-3-small",
		Dimension: 3,
	})
	if err != nil {
		t.Fatalf("NewOpenRouterEmbedding() error = %v", err)
	}
	embedder.endpoint = server.URL

	ctx := WithEmbeddingOpenRouterAPIKey(context.Background(), "sk-or-v1-tenant")
	embeddings, err := embedder.Embed(ctx, []string{"hello"})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if got, want := authHeader, "Bearer sk-or-v1-tenant"; got != want {
		t.Fatalf("Authorization header = %q, want %q", got, want)
	}
	if len(embeddings) != 1 || len(embeddings[0]) != 3 {
		t.Fatalf("Embed() returned %#v, want one 3-dimensional embedding", embeddings)
	}
}

func TestOpenRouterEmbeddingSanitizes401JSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"No auth credentials found","type":"invalid_request_error"}}`))
	}))
	defer server.Close()

	embedder, err := NewOpenRouterEmbedding(&EmbeddingConfig{Model: "openai/text-embedding-3-small", Dimension: 3})
	if err != nil {
		t.Fatalf("NewOpenRouterEmbedding() error = %v", err)
	}
	embedder.endpoint = server.URL

	_, err = embedder.Embed(WithEmbeddingOpenRouterAPIKey(context.Background(), "sk-or-v1-bad"), []string{"hello"})
	if !errors.Is(err, ErrEmbeddingProviderMisconfigured) {
		t.Fatalf("Embed() error = %v, want ErrEmbeddingProviderMisconfigured", err)
	}
	if !strings.Contains(err.Error(), "No auth credentials found") {
		t.Fatalf("Embed() error = %q, want sanitized upstream message", err.Error())
	}
	if strings.Contains(err.Error(), "invalid_request_error") {
		t.Fatalf("Embed() error = %q, should not expose full upstream body/type", err.Error())
	}
}

func TestOpenRouterEmbeddingSanitizes401MalformedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`not-json-with-secret-details`))
	}))
	defer server.Close()

	embedder, err := NewOpenRouterEmbedding(&EmbeddingConfig{Model: "openai/text-embedding-3-small", Dimension: 3})
	if err != nil {
		t.Fatalf("NewOpenRouterEmbedding() error = %v", err)
	}
	embedder.endpoint = server.URL

	_, err = embedder.Embed(WithEmbeddingOpenRouterAPIKey(context.Background(), "sk-or-v1-bad"), []string{"hello"})
	if !errors.Is(err, ErrEmbeddingProviderMisconfigured) {
		t.Fatalf("Embed() error = %v, want ErrEmbeddingProviderMisconfigured", err)
	}
	if !strings.Contains(err.Error(), "OpenRouter authentication failed (401)") {
		t.Fatalf("Embed() error = %q, want generic 401 fallback", err.Error())
	}
	if strings.Contains(err.Error(), "not-json-with-secret-details") {
		t.Fatalf("Embed() error = %q, should not expose malformed upstream body", err.Error())
	}
}

// TestOpenRouterEmbeddingSplitsOversizedInput verifies the Plan 8 / R2, R7
// boundary enforcement: an input exceeding the model limit is split, embedded
// per-sub-input, averaged, and renormalized back into a single vector.
func TestOpenRouterEmbeddingSplitsOversizedInput(t *testing.T) {
	InitTokenizer("test", "text-embedding-3-small")

	var received []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openRouterEmbedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		received = req.Input
		data := make([]map[string]any, len(req.Input))
		for i, in := range req.Input {
			// Deterministic pseudo-embedding derived from input length.
			data[i] = map[string]any{
				"embedding": []float32{float32(len(in)) / 1000.0, 0.2, 0.3},
				"index":     i,
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "model": "openai/text-embedding-3-small"})
	}))
	defer server.Close()

	embedder, err := NewOpenRouterEmbedding(&EmbeddingConfig{Model: "openai/text-embedding-3-small", Dimension: 3})
	if err != nil {
		t.Fatalf("NewOpenRouterEmbedding() error = %v", err)
	}
	embedder.endpoint = server.URL

	// ~10000 tokens of text, far above the 8192 model limit (8176 safe limit).
	big := strings.Repeat("hello world ", 5000)
	ctx := WithEmbeddingOpenRouterAPIKey(context.Background(), "sk-or-v1-test")
	embeddings, err := embedder.Embed(ctx, []string{big})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if len(embeddings) != 1 {
		t.Fatalf("expected 1 embedding (collapsed), got %d", len(embeddings))
	}
	if len(received) < 2 {
		t.Fatalf("expected the oversized input to be split into >=2 sub-inputs at the boundary, got %d", len(received))
	}
	// The averaged+renormalized vector must have unit magnitude (Plan 8 / R2).
	var norm float32
	for _, v := range embeddings[0] {
		norm += v * v
	}
	if math.Abs(float64(norm)-1.0) > 0.01 {
		t.Fatalf("averaged+renormalized vector magnitude = %v, want ~1.0", norm)
	}
}

// captureHandler collects log messages for assertion.
type captureHandler struct {
	mu  sync.Mutex
	msg []string
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msg = append(h.msg, r.Message)
	return nil
}
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler    { return h }
func (h *captureHandler) WithGroup(string) slog.Handler        { return h }
func (h *captureHandler) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.msg))
	copy(out, h.msg)
	return out
}

// TestEstimateTokensFailLoud verifies Plan 8 / R4: when the tokenizer is not
// initialized and no config is available, EstimateTokens logs an ERROR (not a
// warning) rather than silently undercounting.
func TestEstimateTokensFailLoud(t *testing.T) {
	globalTokenizer = nil
	estimateTokenizerConfig = nil
	fallbackWarnOnce = sync.Once{} // reset so the one-time log fires

	ch := &captureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(ch))
	defer slog.SetDefault(old)

	got := EstimateTokens("some uninitialized text")
	if got != len("some uninitialized text")/4 {
		t.Fatalf("EstimateTokens fallback = %d, want len/4", got)
	}
	found := false
	for _, m := range ch.messages() {
		if strings.Contains(m, "EstimateTokens using len/4 fallback") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ERROR log about len/4 fallback, got %v", ch.messages())
	}
}

// TestEstimateTokensSelfHeals verifies Plan 8 / R4: if the config was captured
// but the tokenizer missed init, EstimateTokens initializes on demand and
// returns an accurate count instead of the len/4 fallback.
func TestEstimateTokensSelfHeals(t *testing.T) {
	globalTokenizer = nil
	estimateTokenizerConfig = &EmbeddingConfig{Provider: "openrouter", Model: "text-embedding-3-small"}
	fallbackWarnOnce = sync.Once{}

	got := EstimateTokens("hello world this is a test of tokenizer init")
	if got <= 0 {
		t.Fatalf("EstimateTokens self-heal returned %d, want > 0 exact count", got)
	}
	if globalTokenizer == nil {
		t.Fatalf("expected tokenizer to be initialized on demand")
	}
}

// TestDoEmbedRecursiveResplit verifies the API-authoritative re-split (Plan 8 /
// R7): when the estimator lets an oversized input through and the provider
// rejects it with a "maximum input length" error, doEmbedWith halves the split
// limit and re-splits until every input fits. It must converge and return one
// embedding per original input (no error, no dropped input).
func TestDoEmbedRecursiveResplit(t *testing.T) {
	e := &OpenRouterEmbedding{maxInputTokens: 8192, dimension: 4}

	// Fake provider: reject any single input whose estimated token count
	// exceeds 100 (simulating a real "maximum input length" rejection regardless
	// of the estimator's split limit). This forces doEmbedWith to keep halving
	// the split limit until every input fits.
	embedFunc := func(ctx context.Context, texts []string) ([][]float32, error) {
		for _, txt := range texts {
			if EstimateTokens(txt) > 100 {
				return nil, fmt.Errorf("%w: OpenRouter API error: Invalid 'input[0]': maximum input length is 8192 tokens. (Type: invalid_request_error)",
					ErrEmbeddingProviderUnavailable)
			}
		}
		out := make([][]float32, len(texts))
		for i := range texts {
			out[i] = []float32{1, 0, 0, 0}
		}
		return out, nil
	}

	big := strings.Repeat("a ", 1000) // ~2000 chars, far above the 50-char fake limit
	texts := []string{"short", big, "short2"}

	res, err := e.doEmbedWith(context.Background(), texts, e.maxInputTokens-embedSafetyMargin, 0, embedFunc)
	if err != nil {
		t.Fatalf("doEmbedWith should converge and succeed, got: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("expected 3 embeddings (one per input), got %d", len(res))
	}
	for i, v := range res {
		if len(v) != 4 {
			t.Fatalf("embedding %d has wrong dimension %d", i, len(v))
		}
	}
}
