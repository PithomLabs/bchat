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
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/nlpodyssey/cybertron/pkg/tasks"
	"github.com/nlpodyssey/cybertron/pkg/tasks/textencoding"
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
	Provider         string // "cybertron", "local", "openrouter", or "mock"
	Model            string // Model name/path
	Dimension        int    // Vector dimension (384 for MiniLM, 1536 for OpenAI)
	OpenRouterAPIKey string // For OpenRouter provider
	LocalEndpoint    string // For local provider (default: http://localhost:8001/embed)
	CybertronDir     string // For cybertron provider - models directory
	CybertronWorkers int    // For cybertron provider - number of parallel workers (default: NumCPU)
	BatchSize        int    // Max texts per batch (default: 32)
}

// NewEmbeddingConfigFromEnv creates an EmbeddingConfig from environment variables.
func NewEmbeddingConfigFromEnv() *EmbeddingConfig {
	provider := getEnvOrDefault("EMBEDDING_PROVIDER", "cybertron")
	var dimension int
	var model string

	switch provider {
	case "openrouter":
		model = getEnvOrDefault("EMBEDDING_MODEL", "openai/text-embedding-3-small")
		dimension = getOpenRouterDimension(model)
	case "cybertron":
		model = getEnvOrDefault("EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2")
		dimension = getCybertronDimension(model)
	default:
		model = getEnvOrDefault("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
		dimension = 384 // Default for MiniLM
	}

	// Parse CYBERTRON_WORKERS (default to NumCPU)
	workers := runtime.NumCPU()
	if envWorkers := os.Getenv("CYBERTRON_WORKERS"); envWorkers != "" {
		if w, err := strconv.Atoi(envWorkers); err == nil && w > 0 {
			workers = w
		}
	}

	return &EmbeddingConfig{
		Provider:         provider,
		Model:            model,
		Dimension:        dimension,
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		LocalEndpoint:    getEnvOrDefault("EMBEDDING_LOCAL_ENDPOINT", "http://localhost:8001/embed"),
		CybertronDir:     getEnvOrDefault("CYBERTRON_MODELS_DIR", ""),
		CybertronWorkers: workers,
		BatchSize:        32,
	}
}

// getCybertronDimension returns the embedding dimension for known Cybertron models.
func getCybertronDimension(modelName string) int {
	switch modelName {
	case "sentence-transformers/all-mpnet-base-v2":
		return 768
	case "sentence-transformers/all-MiniLM-L6-v2",
		"sentence-transformers/all-MiniLM-L12-v2",
		"sentence-transformers/paraphrase-MiniLM-L6-v2":
		return 384
	default:
		return 384 // Default for unknown models
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
	// Sentence-transformers models (same dimensions as Cybertron)
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
	case "openrouter":
		return NewOpenRouterEmbedding(config)
	case "mock":
		return NewMockEmbedding(config), nil
	case "local":
		return NewLocalEmbedding(config)
	case "cybertron":
		return NewCybertronEmbedding(config)
	default:
		// Default to cybertron for zero-dependency experience
		return NewCybertronEmbedding(config)
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

	return &LocalEmbedding{
		endpoint:  endpoint,
		dimension: config.Dimension,
		model:     config.Model,
		client:    &http.Client{Timeout: 60 * time.Second},
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

	return &OpenRouterEmbedding{
		apiKey:    config.OpenRouterAPIKey,
		model:     model,
		endpoint:  "https://openrouter.ai/api/v1/embeddings",
		dimension: dimension,
		client:    &http.Client{Timeout: 60 * time.Second},
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

// Embed generates embeddings using OpenRouter's API.
func (e *OpenRouterEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

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
		return nil, fmt.Errorf("OpenRouter embedding error (status %d): %s", resp.StatusCode, string(body))
	}

	var result openRouterEmbedResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("OpenRouter API error: %s", result.Error.Message)
	}

	// Sort embeddings by index to maintain order
	embeddings := make([][]float32, len(result.Data))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
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
// CYBERTRON EMBEDDING SERVICE (Native Go, Zero Dependencies)
// ============================================================================

// CybertronEmbedding implements EmbeddingService using pure Go transformers.
// This provides zero-dependency local embeddings without requiring Python or external servers.
type CybertronEmbedding struct {
	model     textencoding.Interface
	modelName string
	dimension int
	modelsDir string
	workers   int // Number of parallel workers for embedding generation
}

// NewCybertronEmbedding creates a new Cybertron embedding service.
func NewCybertronEmbedding(config *EmbeddingConfig) (*CybertronEmbedding, error) {
	modelName := config.Model
	if modelName == "" {
		modelName = "sentence-transformers/all-MiniLM-L6-v2"
	}

	modelsDir := config.CybertronDir
	if modelsDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		modelsDir = filepath.Join(homeDir, ".cybertron", "models")
	}

	// Determine dimension based on model
	dimension := getCybertronDimension(modelName)
	if config.Dimension > 0 {
		dimension = config.Dimension
	}

	// Set number of workers (default to NumCPU)
	workers := config.CybertronWorkers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	slog.Info("Loading Cybertron embedding model",
		"model", modelName,
		"dimension", dimension,
		"models_dir", modelsDir,
		"workers", workers,
	)

	// Load model (downloads on first use)
	model, err := tasks.Load[textencoding.Interface](&tasks.Config{
		ModelsDir:        modelsDir,
		ModelName:        modelName,
		DownloadPolicy:   tasks.DownloadMissing,
		ConversionPolicy: tasks.ConvertMissing,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load Cybertron model %s: %w", modelName, err)
	}

	slog.Info("Cybertron embedding model loaded successfully",
		"model", modelName,
		"dimension", dimension,
	)

	return &CybertronEmbedding{
		model:     model,
		modelName: modelName,
		dimension: dimension,
		modelsDir: modelsDir,
		workers:   workers,
	}, nil
}

// cybertronJob represents a text to be embedded with its index.
type cybertronJob struct {
	index int
	text  string
}

// cybertronResult represents an embedding result with its index.
type cybertronResult struct {
	index     int
	embedding []float32
	err       error
}

// Embed generates embeddings using Cybertron with parallel processing.
func (e *CybertronEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// For small batches, use sequential processing (less overhead)
	if len(texts) <= e.workers || e.workers <= 1 {
		return e.embedSequential(ctx, texts)
	}

	return e.embedParallel(ctx, texts)
}

// embedParallel processes texts concurrently using a worker pool.
func (e *CybertronEmbedding) embedParallel(ctx context.Context, texts []string) ([][]float32, error) {
	const maxChars = 750

	numWorkers := e.workers
	if numWorkers > len(texts) {
		numWorkers = len(texts)
	}

	jobs := make(chan cybertronJob, len(texts))
	results := make(chan cybertronResult, len(texts))

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// Check for context cancellation
				select {
				case <-ctx.Done():
					results <- cybertronResult{index: job.index, err: ctx.Err()}
					continue
				default:
				}

				text := job.text
				if len(text) > maxChars {
					text = text[:maxChars]
				}

				// MeanPooling = 1
				result, err := e.model.Encode(ctx, text, 1)
				if err != nil {
					results <- cybertronResult{index: job.index, err: fmt.Errorf("failed to encode text %d: %w", job.index, err)}
					continue
				}

				// Convert to []float32
				vector := result.Vector.Data().F64()
				embedding := make([]float32, len(vector))
				for j, v := range vector {
					embedding[j] = float32(v)
				}

				results <- cybertronResult{index: job.index, embedding: embedding}
			}
		}()
	}

	// Send jobs
	for i, text := range texts {
		jobs <- cybertronJob{index: i, text: text}
	}
	close(jobs)

	// Wait for workers to finish and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	embeddings := make([][]float32, len(texts))
	var firstErr error
	completed := 0

	for result := range results {
		if result.err != nil && firstErr == nil {
			firstErr = result.err
		}
		if result.embedding != nil {
			embeddings[result.index] = result.embedding
		}
		completed++

		// Log progress for large batches
		if len(texts) > 50 && completed%50 == 0 {
			slog.Debug("Cybertron parallel embedding progress",
				"completed", completed,
				"total", len(texts),
				"workers", numWorkers)
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}

	slog.Debug("Cybertron parallel embedding complete",
		"total", len(texts),
		"workers", numWorkers)

	return embeddings, nil
}

// embedSequential processes texts one at a time (for small batches or single worker).
func (e *CybertronEmbedding) embedSequential(ctx context.Context, texts []string) ([][]float32, error) {
	const maxChars = 750

	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		// Truncate if necessary (safety net for oversized chunks)
		if len(text) > maxChars {
			slog.Warn("truncating text for embedding",
				"index", i,
				"originalLen", len(text),
				"truncatedTo", maxChars)
			text = text[:maxChars]
		}

		// MeanPooling = 1 (from github.com/nlpodyssey/cybertron/pkg/models/bert)
		result, err := e.model.Encode(ctx, text, 1)
		if err != nil {
			return nil, fmt.Errorf("failed to encode text %d: %w", i, err)
		}

		// Convert mat.Matrix to []float32
		vector := result.Vector.Data().F64()
		embedding := make([]float32, len(vector))
		for j, v := range vector {
			embedding[j] = float32(v)
		}
		embeddings[i] = embedding

		// Log progress for large batches
		if len(texts) > 100 && (i+1)%100 == 0 {
			slog.Debug("Cybertron embedding progress", "completed", i+1, "total", len(texts))
		}
	}

	return embeddings, nil
}

// Dimension returns the embedding vector dimension.
func (e *CybertronEmbedding) Dimension() int {
	return e.dimension
}

// Provider returns "cybertron".
func (e *CybertronEmbedding) Provider() string {
	return "cybertron"
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
