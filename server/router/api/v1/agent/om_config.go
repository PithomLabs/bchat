package agent

import (
	"os"
	"strconv"
	"sync"
)

// OMConfig holds configuration for Observational Memory
type OMConfig struct {
	Enabled                bool
	MessageThreshold       int // Deprecated: Use ObserverTokenThreshold instead
	TokenThreshold         int // Reflector trigger threshold
	ObserverTokenThreshold int // Observer trigger threshold (token-based)
	RetryAttempts          int
	RetryDelayMs           int

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
