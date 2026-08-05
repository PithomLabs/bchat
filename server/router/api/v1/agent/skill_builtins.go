package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/revrost/go-openrouter"
	"github.com/revrost/go-openrouter/jsonschema"
)

// RegisterBuiltins registers all builtin skill handlers into the registry.
func RegisterBuiltins(reg *SkillRegistry) {
	reg.Register(&LogHandler{})
	reg.Register(&SleepHandler{})
	reg.Register(&LLMHandler{})
}

// LogHandler writes structured log entries.
type LogHandler struct{}

func (h *LogHandler) Name() string { return "builtin:log" }

func (h *LogHandler) Execute(_ context.Context, params map[string]string, _ map[string]any) (string, error) {
	level := params["level"]
	if level == "" {
		level = "info"
	}
	message := params["message"]
	if message == "" {
		message = "(no message)"
	}

	switch level {
	case "error":
		slog.Error("skill:log", "message", message)
	case "warn":
		slog.Warn("skill:log", "message", message)
	case "debug":
		slog.Debug("skill:log", "message", message)
	default:
		slog.Info("skill:log", "message", message)
	}

	return "logged", nil
}

func (h *LogHandler) Definition() openrouter.FunctionDefinition {
	return openrouter.FunctionDefinition{
		Name:        "log",
		Description: "Write a structured log entry for debugging or audit purposes",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"level": {
					Type:        jsonschema.String,
					Description: "Log level: info, warn, error, debug",
					Enum:        []string{"info", "warn", "error", "debug"},
				},
				"message": {
					Type:        jsonschema.String,
					Description: "The log message to write",
				},
			},
			Required: []string{"message"},
		},
	}
}

// SleepHandler blocks for a configurable duration (demo/testing).
type SleepHandler struct{}

func (h *SleepHandler) Name() string { return "builtin:sleep" }

func (h *SleepHandler) Execute(ctx context.Context, params map[string]string, _ map[string]any) (string, error) {
	durationStr := params["duration"]
	if durationStr == "" {
		durationStr = "1"
	}

	seconds, err := strconv.Atoi(durationStr)
	if err != nil {
		return "", fmt.Errorf("invalid duration: %w", err)
	}

	if seconds > 30 {
		seconds = 30 // cap at 30s
	}

	select {
	case <-time.After(time.Duration(seconds) * time.Second):
		return fmt.Sprintf("slept %ds", seconds), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (h *SleepHandler) Definition() openrouter.FunctionDefinition {
	return openrouter.FunctionDefinition{
		Name:        "sleep",
		Description: "Pause execution for a short duration (for testing/demo workflows)",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"duration": {
					Type:        jsonschema.Integer,
					Description: "Number of seconds to sleep (max 30)",
				},
			},
			Required: []string{"duration"},
		},
	}
}

// LLMHandler delegates to the LLM for summarization/extraction.
// Uses a callback to avoid circular dependency with Service.
type LLMHandler struct {
	GenerateFn func(ctx context.Context, prompt string, vars map[string]any) (string, error)
}

func (h *LLMHandler) Name() string { return "builtin:llm_call" }

func (h *LLMHandler) Execute(ctx context.Context, params map[string]string, vars map[string]any) (string, error) {
	if h.GenerateFn == nil {
		return "", fmt.Errorf("llm_call: GenerateFn not set")
	}

	prompt := params["prompt"]
	if prompt == "" {
		prompt = params["message"]
	}
	if prompt == "" {
		return "", fmt.Errorf("llm_call: prompt parameter is required")
	}

	// Merge vars into prompt context.
	// NOTE: Node outputs in vars are now maps (buildNodeOutput contract),
	// not raw strings. LLM prompts see {"output":"..."} / {"found":true,...}
	// instead of bare text. Tenant scripts comparing on raw node text will
	// see the wrapper. This is by design — CEL conditions use field access.
	contextData, _ := json.Marshal(vars)
	expandedPrompt := prompt + "\n\nContext:\n" + string(contextData)

	return h.GenerateFn(ctx, expandedPrompt, vars)
}

func (h *LLMHandler) Definition() openrouter.FunctionDefinition {
	return openrouter.FunctionDefinition{
		Name:        "llm_call",
		Description: "Call the LLM for summarization, extraction, or analysis tasks within a workflow",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"prompt": {
					Type:        jsonschema.String,
					Description: "The prompt to send to the LLM",
				},
				"model": {
					Type:        jsonschema.String,
					Description: "Optional model override (uses default if empty)",
				},
			},
			Required: []string{"prompt"},
		},
	}
}

// FailingHandler always returns a configurable error (for testing retry paths).
type FailingHandler struct {
	FailError error
}

func (h *FailingHandler) Name() string { return "builtin:failing" }

func (h *FailingHandler) Execute(_ context.Context, _ map[string]string, _ map[string]any) (string, error) {
	if h.FailError == nil {
		return "", fmt.Errorf("builtin:failing: no error configured")
	}
	return "", h.FailError
}

func (h *FailingHandler) Definition() openrouter.FunctionDefinition {
	return openrouter.FunctionDefinition{
		Name:        "failing",
		Description: "Always fails with a configurable error (for testing)",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"message": {
					Type:        jsonschema.String,
					Description: "Error message (ignored — FailError is used instead)",
				},
			},
		},
	}
}
