package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// EmbeddingService defines the interface for generating text embeddings.
type EmbeddingService interface {
	// Embed generates embeddings for a batch of texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension returns the embedding vector dimension.
	Dimension() int
	// Provider returns the provider name ("local", "openrouter", or "mock").
	Provider() string
}

// EmbeddingConfig holds configuration for embedding services.
type EmbeddingConfig struct {
	Provider         string // "local", "openrouter", or "mock"
	Model            string // Model name/path
	Dimension        int    // Vector dimension (384 for MiniLM, 1536 for OpenAI)
	OpenRouterAPIKey string // For OpenRouter provider
	LocalEndpoint    string // For local provider (default: http://localhost:8001/embed)
	BatchSize        int    // Max texts per batch (default: 32)
}

// NewEmbeddingConfigFromEnv creates an EmbeddingConfig from environment variables.
func NewEmbeddingConfigFromEnv() *EmbeddingConfig {
	provider := getEnvOrDefault("EMBEDDING_PROVIDER", "openrouter")
	var dimension int
	var model string

	switch provider {
	case "openrouter", "openai":
		model = getEnvOrDefault("EMBEDDING_MODEL", "openai/text-embedding-3-small")
		dimension = getOpenRouterDimension(model)
	default:
		model = getEnvOrDefault("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
		// Try to detect dimension from model name
		detectedDim := getOpenRouterDimension(model)
		// If getOpenRouterDimension returns 1536 (default/unknown) but it's not an OpenAI model,
		// and we are in the default provider case (likely local/MiniLM), default to 384.
		// This preserves backward compatibility for "all-MiniLM-L6-v2" while supporting known models like Qwen.
		if detectedDim == 1536 && !strings.Contains(model, "openai") && !strings.Contains(model, "text-embedding") && !strings.Contains(model, "ada-002") {
			dimension = 384
		} else {
			dimension = detectedDim
		}
	}

	// Allow explicit override via environment variable
	if v := os.Getenv("EMBEDDING_DIMENSION"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			dimension = d
		}
	}

	return &EmbeddingConfig{
		Provider:         provider,
		Model:            model,
		Dimension:        dimension,
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		LocalEndpoint:    getEnvOrDefault("EMBEDDING_LOCAL_ENDPOINT", "http://localhost:8001/embed"),
		BatchSize:        32,
	}
}

// getOpenRouterDimension returns the embedding dimension for known OpenRouter models.
// Supports both OpenAI and sentence-transformers models available via OpenRouter.
func getOpenRouterDimension(modelName string) int {
	switch modelName {
	// OpenAI models
	case "openai/text-embedding-3-large":
		return 3072
	case "openai/text-embedding-ada-002", "openai/text-embedding-3-small":
		return 1536
	// Qwen models
	case "qwen/qwen3-embedding-8b":
		return 4096
	// Sentence-transformers models
	case "sentence-transformers/all-MiniLM-L6-v2",
		"sentence-transformers/all-MiniLM-L12-v2",
		"sentence-transformers/paraphrase-MiniLM-L6-v2":
		return 384
	case "sentence-transformers/all-mpnet-base-v2":
		return 768
	default:
		return 1536 // Default to OpenAI dimension
	}
}

// NewEmbeddingService creates an embedding service based on the configuration.
func NewEmbeddingService(config *EmbeddingConfig) (EmbeddingService, error) {
	switch config.Provider {
	case "openrouter", "openai":
		return NewOpenRouterEmbedding(config)
	case "mock":
		return NewMockEmbedding(config), nil
	case "local":
		return NewLocalEmbedding(config)
	default:
		// Default to openrouter
		return NewOpenRouterEmbedding(config)
	}
}

// ============================================================================
// LOCAL EMBEDDING SERVICE (Testing/QA)
// ============================================================================

// LocalEmbedding implements EmbeddingService using a local HTTP endpoint.
// Designed to work with a Python FastAPI server running sentence-transformers.
type LocalEmbedding struct {
	endpoint  string
	dimension int
	model     string
	client    *http.Client
}

// NewLocalEmbedding creates a new local embedding service.
func NewLocalEmbedding(config *EmbeddingConfig) (*LocalEmbedding, error) {
	endpoint := config.LocalEndpoint
	if endpoint == "" {
		endpoint = "http://localhost:8001/embed"
	}

	timeout := getEnvDuration("EMBEDDING_TIMEOUT", 180*time.Second)

	return &LocalEmbedding{
		endpoint:  endpoint,
		dimension: config.Dimension,
		model:     config.Model,
		client:    &http.Client{Timeout: timeout},
	}, nil
}

type localEmbedRequest struct {
	Texts []string `json:"texts"`
	Model string   `json:"model,omitempty"`
}

type localEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Model      string      `json:"model,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// Embed generates embeddings using the local embedding service.
func (e *LocalEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	reqBody := localEmbedRequest{
		Texts: texts,
		Model: e.model,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("local embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("local embedding error (status %d): %s", resp.StatusCode, string(body))
	}

	var result localEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("local embedding service error: %s", result.Error)
	}

	return result.Embeddings, nil
}

// Dimension returns the embedding vector dimension.
func (e *LocalEmbedding) Dimension() int {
	return e.dimension
}

// Provider returns "local".
func (e *LocalEmbedding) Provider() string {
	return "local"
}

// ============================================================================
// OPENROUTER EMBEDDING SERVICE (Production)
// ============================================================================

// OpenRouterEmbedding implements EmbeddingService using OpenRouter's API.
type OpenRouterEmbedding struct {
	apiKey    string
	model     string
	endpoint  string
	dimension int
	client    *http.Client
}

// NewOpenRouterEmbedding creates a new OpenRouter embedding service.
func NewOpenRouterEmbedding(config *EmbeddingConfig) (*OpenRouterEmbedding, error) {
	if config.OpenRouterAPIKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY required for OpenRouter embedding provider")
	}

	model := config.Model
	if model == "" {
		model = "openai/text-embedding-3-small"
	}

	// Set dimension based on model (supports OpenAI and sentence-transformers)
	dimension := config.Dimension
	if dimension == 0 {
		dimension = getOpenRouterDimension(model)
	}

	timeout := getEnvDuration("EMBEDDING_TIMEOUT", 180*time.Second)

	return &OpenRouterEmbedding{
		apiKey:    config.OpenRouterAPIKey,
		model:     model,
		endpoint:  "https://openrouter.ai/api/v1/embeddings",
		dimension: dimension,
		client:    &http.Client{Timeout: timeout},
	}, nil
}

type openRouterEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openRouterEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Embed generates embeddings using OpenRouter's API with retry logic.
func (e *OpenRouterEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	var lastErr error
	maxRetries := 5
	baseBackoff := 2 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s, 16s...
			backoff := baseBackoff * time.Duration(1<<(attempt-1))
			slog.Info("Retrying embedding request", "attempt", attempt+1, "backoff", backoff, "textsCount", len(texts))
			time.Sleep(backoff)
		}

		embeddings, err := e.doEmbed(ctx, texts)
		if err == nil {
			return embeddings, nil
		}
		lastErr = err

		// Only retry on timeout/network errors, not API errors
		if !isRetryableError(err) {
			return nil, err
		}
		slog.Warn("Embedding request failed, will retry", "attempt", attempt+1, "error", err.Error())
	}
	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// doEmbed performs the actual HTTP request to OpenRouter.
func (e *OpenRouterEmbedding) doEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := openRouterEmbedRequest{
		Model: e.model,
		Input: texts,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/usememos/memos")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("OpenRouter embedding error: 401 Unauthorized. Please check your OPENROUTER_API_KEY and account status (ensure you have credits if using paid models).")
		}
		return nil, fmt.Errorf("OpenRouter embedding error (status %d): %s", resp.StatusCode, string(body))
	}

	var result openRouterEmbedResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		slog.Warn("OpenRouter API returned error",
			"model", e.model,
			"errorMessage", result.Error.Message,
			"errorType", result.Error.Type,
			"textsCount", len(texts))
		return nil, fmt.Errorf("OpenRouter API error: %s", result.Error.Message)
	}

	// Sort embeddings by index to maintain order
	embeddings := make([][]float32, len(result.Data))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
}

// isRetryableError returns true if the error is likely transient and worth retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "No successful provider") ||
		strings.Contains(errStr, "server error") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "429")
}

// Dimension returns the embedding vector dimension.
func (e *OpenRouterEmbedding) Dimension() int {
	return e.dimension
}

// Provider returns "openrouter".
func (e *OpenRouterEmbedding) Provider() string {
	return "openrouter"
}

// ============================================================================
// MOCK EMBEDDING SERVICE (Testing without external dependencies)
// ============================================================================

// MockEmbedding implements EmbeddingService using deterministic pseudo-random vectors.
// This is useful for testing the RAG pipeline without requiring an embedding server or API.
type MockEmbedding struct {
	dimension int
}

// NewMockEmbedding creates a new mock embedding service.
func NewMockEmbedding(config *EmbeddingConfig) *MockEmbedding {
	dimension := config.Dimension
	if dimension == 0 {
		dimension = 384 // Default dimension
	}
	return &MockEmbedding{
		dimension: dimension,
	}
}

// Embed generates deterministic pseudo-random embeddings based on text hash.
// The same text will always produce the same embedding, enabling consistent search results.
func (e *MockEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		embedding := make([]float32, e.dimension)
		// Use a simple hash-based approach for deterministic embeddings
		hash := uint64(0)
		for _, c := range text {
			hash = hash*31 + uint64(c)
		}

		// Generate pseudo-random values from hash
		for j := 0; j < e.dimension; j++ {
			// Linear congruential generator for reproducible values
			hash = hash*6364136223846793005 + 1442695040888963407
			// Normalize to [-1, 1] range
			embedding[j] = float32(int64(hash>>33)-int64(1<<30)) / float32(1<<30)
		}

		// Normalize the vector to unit length
		var norm float32
		for _, v := range embedding {
			norm += v * v
		}
		if norm > 0 {
			norm = float32(1.0 / float64(norm))
			for j := range embedding {
				embedding[j] *= norm
			}
		}

		embeddings[i] = embedding
	}

	return embeddings, nil
}

// Dimension returns the embedding vector dimension.
func (e *MockEmbedding) Dimension() int {
	return e.dimension
}

// Provider returns "mock".
func (e *MockEmbedding) Provider() string {
	return "mock"
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvDuration returns a duration from an environment variable or default.
// Accepts formats like "180s", "3m", "1h30m".
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		slog.Warn("Invalid duration format for env var, using default", "key", key, "value", v, "default", defaultVal)
	}
	return defaultVal
}

// GetEmbeddingBatchSize returns the embedding batch size from env or default.
// Controls how many chunks are sent to embedding API per request.
// Default is 25. For Qwen3 (32K context), up to 40 is safe with 800-token chunks.
func GetEmbeddingBatchSize() int {
	if v := os.Getenv("EMBEDDING_BATCH_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil && size > 0 && size <= 200 {
			return size
		}
	}
	return 10
}
