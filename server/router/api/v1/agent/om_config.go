package agent

import (
	"log/slog"
	"os"
	"strconv"
	"sync"
)

// OMScope defines the scope for Observational Memory
type OMScope string

const (
	// OMScopeThread limits observations to the current session (default)
	OMScopeThread OMScope = "thread"
	// OMScopeResource shares observations across all sessions for a user
	OMScopeResource OMScope = "resource"
)

// OMConfig holds configuration for Observational Memory
type OMConfig struct {
	Enabled                bool
	MessageThreshold       int // Deprecated: Use ObserverTokenThreshold instead
	TokenThreshold         int // Reflector trigger threshold
	ObserverTokenThreshold int // Observer trigger threshold (token-based)
	RetryAttempts          int
	RetryDelayMs           int
	BufferTokens           float64 // Fraction of threshold to trigger buffer (0.2 = 20%)
	BufferActivation       float64 // Activation point (0.8 = 80%)
	BlockAfter             float64 // Safety threshold multiplier (1.2 = 120%)
	Scope                  OMScope // Memory scope: thread or resource

	mu          sync.RWMutex
	initialized bool
}

var omConfig *OMConfig
var omConfigOnce sync.Once

// GetOMConfig returns the singleton OM configuration
func GetOMConfig() *OMConfig {
	omConfigOnce.Do(func() {
		omConfig = loadOMConfig()
	})
	return omConfig
}

func loadOMConfig() *OMConfig {
	return &OMConfig{
		Enabled:                getEnvBool("OM_ENABLED", true),
		MessageThreshold:       getEnvInt("OM_MESSAGE_THRESHOLD", 10), // Deprecated: for backward compatibility
		TokenThreshold:         getEnvInt("OM_TOKEN_THRESHOLD", 2000),
		ObserverTokenThreshold: getEnvInt("OM_OBSERVER_TOKEN_THRESHOLD", 30000), // Mastra default
		RetryAttempts:          getEnvInt("OM_RETRY_ATTEMPTS", 3),
		RetryDelayMs:           getEnvInt("OM_RETRY_DELAY_MS", 1000),
		BufferTokens:           getEnvFloat("OM_BUFFER_TOKENS", 0.2),     // 20% of threshold
		BufferActivation:       getEnvFloat("OM_BUFFER_ACTIVATION", 0.8), // 80% of threshold
		BlockAfter:             getEnvFloat("OM_BLOCK_AFTER", 1.2),       // 120% of threshold
		Scope:                  getEnvScope("OM_SCOPE", OMScopeThread),   // Default to thread scope
	}
}

// ReloadOMConfig reloads configuration from environment variables
// Useful for testing or hot reconfiguration
func ReloadOMConfig() *OMConfig {
	omConfig = loadOMConfig()
	return omConfig
}

// GetConfig returns a thread-safe copy of the config
func (c *OMConfig) GetConfig() OMConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return OMConfig{
		Enabled:                c.Enabled,
		MessageThreshold:       c.MessageThreshold,
		TokenThreshold:         c.TokenThreshold,
		ObserverTokenThreshold: c.ObserverTokenThreshold,
		RetryAttempts:          c.RetryAttempts,
		RetryDelayMs:           c.RetryDelayMs,
		BufferTokens:           c.BufferTokens,
		BufferActivation:       c.BufferActivation,
		BlockAfter:             c.BlockAfter,
		Scope:                  c.Scope,
	}
}

// Helper functions for environment variable parsing

// getEnvBool returns a boolean from environment variable or default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		// Handle common boolean representations
		lower := value
		if lower == "true" || lower == "1" || lower == "yes" || lower == "on" {
			return true
		}
		if lower == "false" || lower == "0" || lower == "no" || lower == "off" {
			return false
		}
	}
	return defaultValue
}

// getEnvInt returns an integer from environment variable or default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvFloat returns a float64 from environment variable or default
func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

// getEnvScope returns an OMScope from environment variable or default
func getEnvScope(key string, defaultValue OMScope) OMScope {
	if value := os.Getenv(key); value != "" {
		if value == string(OMScopeResource) {
			return OMScopeResource
		}
		if value == string(OMScopeThread) {
			return OMScopeThread
		}
		// Warn on invalid value but don't fail - use default
		slog.Warn("Invalid OM_SCOPE value, using default", "value", value, "expected", []string{string(OMScopeThread), string(OMScopeResource)}, "default", defaultValue)
	}
	return defaultValue
}

// ObserverMutex provides per-session mutex for debouncing observer runs
type ObserverMutex struct {
	mu sync.Map // map[string]chan struct{}
}

// NewObserverMutex creates a new ObserverMutex
func NewObserverMutex() *ObserverMutex {
	return &ObserverMutex{
		mu: sync.Map{},
	}
}

// TryLock attempts to acquire a lock for the given session ID
// Returns true if lock was acquired, false if already locked
func (om *ObserverMutex) TryLock(sessionID string) bool {
	lock, _ := om.mu.LoadOrStore(sessionID, make(chan struct{}, 1))
	ch := lock.(chan struct{})

	select {
	case ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Unlock releases the lock for the given session ID
func (om *ObserverMutex) Unlock(sessionID string) {
	if lock, ok := om.mu.Load(sessionID); ok {
		ch := lock.(chan struct{})
		<-ch
	}
}

// Global observer mutex instance
var globalObserverMutex *ObserverMutex

func init() {
	globalObserverMutex = NewObserverMutex()
}

// GetObserverMutex returns the global observer mutex
func GetObserverMutex() *ObserverMutex {
	return globalObserverMutex
}
