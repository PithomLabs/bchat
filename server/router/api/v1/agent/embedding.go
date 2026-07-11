package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tiktoken-go/tokenizer"
)

var (
	ErrEmbeddingProviderMisconfigured = errors.New("embedding provider misconfigured")
	ErrEmbeddingProviderUnavailable   = errors.New("embedding provider unavailable")
	ErrVectorStoreUnavailable         = errors.New("vector store unavailable")
)

var globalTokenizer tokenizer.Codec
var fallbackWarnOnce sync.Once

// estimateTokenizerConfig holds the embedding config so EstimateTokens can
// self-heal (initialize the tokenizer on demand) if it was missed at startup.
// Set once from NewVectorDB via SetEstimateTokenizerConfig.
var estimateTokenizerConfig *EmbeddingConfig

// embedSafetyMargin is the token headroom kept below the model's hard input
// limit when deciding whether to split an input (covers local-vs-API token
// counting discrepancies and the splitByHardLimit binary-search off-by-one).
const embedSafetyMargin = 16

// SetEstimateTokenizerConfig stores the embedding config for on-demand
// tokenizer initialization in EstimateTokens (see Plan 8 / R4).
func SetEstimateTokenizerConfig(cfg *EmbeddingConfig) {
	estimateTokenizerConfig = cfg
}

// maybeInitTokenizer attempts to initialize the global tokenizer from the
// captured embedding config if it is not already initialized.
func maybeInitTokenizer() {
	if globalTokenizer != nil || estimateTokenizerConfig == nil {
		return
	}
	InitTokenizer(estimateTokenizerConfig.Provider, estimateTokenizerConfig.Model)
}

// InitTokenizer initializes the global tokenizer for accurate token counting.
// Must be called before any EstimateTokens calls. Uses the embedding model name
// to select the correct tokenizer encoding.
func InitTokenizer(provider, model string) {
	if globalTokenizer != nil {
		return
	}
	var enc tokenizer.Codec
	var encName tokenizer.Encoding
	switch {
	case strings.Contains(model, "text-embedding-3-small"),
		strings.Contains(model, "text-embedding-3-large"),
		strings.Contains(model, "text-embedding-ada-002"):
		encName = tokenizer.Cl100kBase
	case strings.Contains(model, "gpt-4o"),
		strings.Contains(model, "gpt-5"):
		encName = tokenizer.O200kBase
	default:
		encName = tokenizer.Cl100kBase
	}
	enc, err := tokenizer.Get(encName)
	if err != nil {
		slog.Error("CRITICAL: Tokenizer initialization failed, falling back to len/4 heuristic. Embedding token counts will be inaccurate.", "encoding", encName, "error", err)
		return
	}
	globalTokenizer = enc
	testTokens, _ := enc.Count("The quick brown fox jumps over the lazy dog.")
	slog.Info("Tokenizer verified", "encoding", encName, "testStringTokens", testTokens)
	slog.Info("Tokenizer initialized", "encoding", encName, "provider", provider, "model", model)
}

type embeddingHTTPError struct {
	statusCode int
	message    string
}

func (e *embeddingHTTPError) Error() string {
	if e.message == "" {
		return fmt.Sprintf("OpenRouter embedding request failed (status %d)", e.statusCode)
	}
	return fmt.Sprintf("OpenRouter embedding request failed (status %d): %s", e.statusCode, e.message)
}

// EmbeddingService defines the interface for generating text embeddings.
type EmbeddingService interface {
	// Embed generates embeddings for a batch of texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension returns the embedding vector dimension.
	Dimension() int
	// Provider returns the provider name ("local", "openrouter", or "mock").
	Provider() string
	// MaxInputTokens returns the embedding model's hard input token limit.
	// Returns math.MaxInt32 for providers without a hard limit.
	MaxInputTokens() int
}

// modelMaxInputTokens returns the embedding model's hard input token limit.
// This is the single authoritative source of truth for the limit; callers must
// derive the safe chunk/embed size from it rather than using magic constants.
// (Plan 8 / R1)
func modelMaxInputTokens(model string) int {
	switch {
	case strings.Contains(model, "text-embedding-3-small"),
		strings.Contains(model, "text-embedding-3-large"),
		strings.Contains(model, "text-embedding-ada-002"):
		return 8192
	case strings.Contains(model, "qwen3-embedding-8b"):
		return 32768
	default:
		return 8192 // OpenAI default; the API is the final authority
	}
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

type embeddingAPIKeyContextKey struct{}

// WithEmbeddingOpenRouterAPIKey stores a tenant-scoped OpenRouter key for embedding calls.
func WithEmbeddingOpenRouterAPIKey(ctx context.Context, apiKey string) context.Context {
	if apiKey == "" {
		return ctx
	}
	return context.WithValue(ctx, embeddingAPIKeyContextKey{}, apiKey)
}

func embeddingOpenRouterAPIKeyFromContext(ctx context.Context) string {
	apiKey, _ := ctx.Value(embeddingAPIKeyContextKey{}).(string)
	return apiKey
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

// MaxInputTokens returns math.MaxInt32 (local provider has no hard limit).
func (e *LocalEmbedding) MaxInputTokens() int {
	return math.MaxInt32
}

// ============================================================================
// OPENROUTER EMBEDDING SERVICE (Production)
// ============================================================================

// OpenRouterEmbedding implements EmbeddingService using OpenRouter's API.
type OpenRouterEmbedding struct {
	apiKey         string
	model          string
	endpoint       string
	dimension      int
	maxInputTokens int
	client         *http.Client
}

// NewOpenRouterEmbedding creates a new OpenRouter embedding service.
func NewOpenRouterEmbedding(config *EmbeddingConfig) (*OpenRouterEmbedding, error) {
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
		apiKey:         config.OpenRouterAPIKey,
		model:          model,
		endpoint:       "https://openrouter.ai/api/v1/embeddings",
		dimension:      dimension,
		maxInputTokens: modelMaxInputTokens(model),
		client:         &http.Client{Timeout: timeout},
	}, nil
}

// MaxInputTokens returns the model's hard input token limit.
func (e *OpenRouterEmbedding) MaxInputTokens() int {
	return e.maxInputTokens
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
	maxRetries := 10
	baseBackoff := 2 * time.Second
	maxBackoff := 30 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with cap
			backoff := baseBackoff * time.Duration(1<<(attempt-1))
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
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
	return nil, fmt.Errorf("%w: failed after %d retries: %v", ErrEmbeddingProviderUnavailable, maxRetries, lastErr)
}

// doEmbedEnforceLimit expands oversized inputs and embeds them, returning one
// embedding per input. It is the API-authoritative boundary (Plan 8 / R2, R7):
// if the embedding provider rejects an input for exceeding its hard token
// limit, doEmbed recurses with a halved split limit until every input fits.
// This makes the guard immune to EstimateTokens under/over-estimation.
func (e *OpenRouterEmbedding) doEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	limit := e.maxInputTokens - embedSafetyMargin
	if limit <= 0 {
		limit = e.maxInputTokens
	}
	return e.doEmbedWith(ctx, texts, limit, 0, e.doEmbedRaw)
}

// doEmbedMaxDepth bounds the re-split recursion. Starting from ~8176 tokens and
// halving each iteration reaches a safe size well within any model limit long
// before this depth.
const doEmbedMaxDepth = 12

// doEmbedWith is the recursive core of doEmbed. embedFunc performs the actual
// (already-expanded) embedding request; it is a parameter so tests can inject a
// fake that simulates the provider's "maximum input length" rejection.
func (e *OpenRouterEmbedding) doEmbedWith(
	ctx context.Context,
	texts []string,
	splitLimit int,
	depth int,
	embedFunc func(context.Context, []string) ([][]float32, error),
) ([][]float32, error) {
	if depth > doEmbedMaxDepth {
		return nil, fmt.Errorf("%w: unable to fit input within model limit after %d re-splits", ErrEmbeddingProviderUnavailable, depth)
	}

	// Expand oversized inputs into sub-inputs, tracking each original input's
	// sub-input indices so we can collapse them back afterward.
	expanded := make([]string, 0, len(texts))
	groups := make([]embedGroup, len(texts))
	for i, t := range texts {
		if EstimateTokens(t) > splitLimit {
			parts := splitByHardLimit(t, splitLimit)
			slog.Warn("Embedding input exceeded model limit; splitting",
				"origIndex", i,
				"tokens", EstimateTokens(t),
				"limit", splitLimit,
				"parts", len(parts),
				"depth", depth)
			for _, p := range parts {
				groups[i].subIndices = append(groups[i].subIndices, len(expanded))
				expanded = append(expanded, p)
			}
		} else {
			groups[i].subIndices = append(groups[i].subIndices, len(expanded))
			expanded = append(expanded, t)
		}
	}

	if len(expanded) == 0 {
		return [][]float32{}, nil
	}

	raw, err := embedFunc(ctx, expanded)
	if err == nil {
		// Collapse: average sub-embeddings per original input, then renormalize.
		return collapseEmbeddings(raw, groups, e.dimension), nil
	}

	// The API is the final authority: if it rejected an input for length,
	// halve the split limit and re-split everything. Even if EstimateTokens is
	// inaccurate, this converges because the limit shrinks each iteration.
	if isMaxInputLengthError(err) {
		slog.Warn("OpenRouter rejected input for length; re-splitting with smaller limit",
			"depth", depth,
			"splitLimit", splitLimit,
			"error", err.Error())
		return e.doEmbedWith(ctx, texts, splitLimit/2, depth+1, embedFunc)
	}
	return nil, err
}

// embedGroup tracks the expanded sub-input indices that belong to one original
// input, so they can be collapsed back into a single embedding.
type embedGroup struct {
	subIndices []int
}

// collapseEmbeddings averages the sub-embeddings of each original input back
// into a single vector and renormalizes it (Plan 8 / R2).
func collapseEmbeddings(raw [][]float32, groups []embedGroup, dim int) [][]float32 {
	result := make([][]float32, len(groups))
	for i, g := range groups {
		if len(g.subIndices) == 0 {
			continue
		}
		if len(g.subIndices) == 1 {
			result[i] = raw[g.subIndices[0]]
			continue
		}
		merged := make([]float32, dim)
		for _, si := range g.subIndices {
			for d := 0; d < dim; d++ {
				merged[d] += raw[si][d]
			}
		}
		n := float32(len(g.subIndices))
		for d := 0; d < dim; d++ {
			merged[d] /= n
		}
		// Renormalize so cosine similarity downstream stays valid (Plan 8 / R2).
		var norm float32
		for d := 0; d < dim; d++ {
			norm += merged[d] * merged[d]
		}
		if norm > 0 {
			scale := float32(1.0 / math.Sqrt(float64(norm)))
			for d := 0; d < dim; d++ {
				merged[d] *= scale
			}
		}
		result[i] = merged
	}
	return result
}

// isMaxInputLengthError reports whether err is the provider's hard input-length
// rejection (e.g. OpenRouter: 'Invalid "input[0]": maximum input length is 8192
// tokens.'). This is the trigger for the recursive re-split in doEmbed.
func isMaxInputLengthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "maximum input length") || strings.Contains(msg, "maximum input")
}

// doEmbedRaw performs the actual HTTP request to OpenRouter for the given
// (already expanded) texts and returns embeddings aligned by response index.
func (e *OpenRouterEmbedding) doEmbedRaw(ctx context.Context, texts []string) ([][]float32, error) {
	apiKey := e.apiKey
	if tenantAPIKey := embeddingOpenRouterAPIKeyFromContext(ctx); tenantAPIKey != "" {
		apiKey = tenantAPIKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("%w: OPENROUTER_API_KEY is not configured and no tenant API key is available", ErrEmbeddingProviderMisconfigured)
	}

	// Prepare request body
	reqBody := openRouterEmbedRequest{
		Model: e.model,
		Input: texts,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	// OpenRouter requires these headers for identification
	req.Header.Set("HTTP-Referer", "https://github.com/usememos/memos")
	req.Header.Set("X-Title", "Memos bchat")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: HTTP request failed: %w", ErrEmbeddingProviderUnavailable, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		httpErr := &embeddingHTTPError{
			statusCode: resp.StatusCode,
			message:    sanitizedOpenRouterErrorMessage(body, resp.StatusCode),
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: %w", ErrEmbeddingProviderMisconfigured, httpErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("%w: %w", ErrEmbeddingProviderUnavailable, httpErr)
		}
		return nil, fmt.Errorf("%w: %w", ErrEmbeddingProviderMisconfigured, httpErr)
	}

	var result openRouterEmbedResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		slog.Warn("OpenRouter API returned error",
			"model", e.model,
			"errorMessage", result.Error.Message,
			"errorType", result.Error.Type)
		return nil, fmt.Errorf("%w: OpenRouter API error: %s (Type: %s)", ErrEmbeddingProviderUnavailable, result.Error.Message, result.Error.Type)
	}

	// Sort embeddings by index to maintain order
	embeddings := make([][]float32, len(result.Data))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
}

func sanitizedOpenRouterErrorMessage(body []byte, statusCode int) string {
	var result openRouterEmbedResponse
	if err := json.Unmarshal(body, &result); err == nil && result.Error != nil && result.Error.Message != "" {
		return result.Error.Message
	}
	if statusCode == http.StatusUnauthorized {
		return "OpenRouter authentication failed (401)"
	}
	return fmt.Sprintf("OpenRouter embedding request failed (status %d)", statusCode)
}

// isRetryableError returns true if the error is likely transient and worth retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrEmbeddingProviderMisconfigured) || errors.Is(err, ErrVectorStoreUnavailable) {
		return false
	}

	var httpErr *embeddingHTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.statusCode {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		default:
			return false
		}
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	return false
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

// MaxInputTokens returns math.MaxInt32 (mock provider has no hard limit).
func (e *MockEmbedding) MaxInputTokens() int {
	return math.MaxInt32
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
