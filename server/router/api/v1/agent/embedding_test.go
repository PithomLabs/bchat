package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
