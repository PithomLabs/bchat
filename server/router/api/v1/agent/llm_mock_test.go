package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newMockLLMServer spins up an in-memory HTTP server that mimics the OpenRouter
// chat-completions endpoint and returns a canned completion. It is used by tests
// that exercise the real chat-generation path without a live OpenRouter API key.
// The OpenRouter client is pointed at this server via the OPENROUTER_API_BASE_URL
// env var (see newOpenRouterClient).
func newMockLLMServer(t *testing.T, reply string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "mock",
			"object":  "chat.completion",
			"model":   "mock",
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": reply,
				},
				"finish_reason": "stop",
			}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// withMockLLM configures the process for a mock LLM: a non-empty fake API key
// plus OPENROUTER_API_BASE_URL pointing at the in-memory mock server. This
// satisfies requireLLMConfig (apiKey != "") and routes generation to the mock
// instead of the real OpenRouter API.
func withMockLLM(t *testing.T, reply string) {
	t.Helper()
	srv := newMockLLMServer(t, reply)
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-mock")
	t.Setenv("OPENROUTER_API_BASE_URL", srv.URL)
}
