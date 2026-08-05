package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/revrost/go-openrouter"
)

// SkillHandler defines the interface for executing a skill step.
type SkillHandler interface {
	// Name returns the unique handler identifier (e.g. "builtin:classify_intent").
	Name() string

	// Execute runs the handler with the given params and evaluated variables.
	// Returns the output text or an error.
	Execute(ctx context.Context, params map[string]string, vars map[string]any) (string, error)

	// Definition returns the OpenRouter tool definition for LLM function calling.
	Definition() openrouter.FunctionDefinition
}

// SkillRegistry manages registered skill handlers.
type SkillRegistry struct {
	handlers map[string]SkillHandler
	mu       sync.RWMutex
}

// NewSkillRegistry creates an empty registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		handlers: make(map[string]SkillHandler),
	}
}

// Register adds a handler. Panics on duplicate.
func (r *SkillRegistry) Register(h SkillHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[h.Name()]; exists {
		panic(fmt.Sprintf("duplicate skill handler: %s", h.Name()))
	}
	r.handlers[h.Name()] = h
}

// Get retrieves a handler by its full name (e.g. "builtin:classify_intent").
func (r *SkillRegistry) Get(name string) (SkillHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}

// List returns all registered handlers.
func (r *SkillRegistry) List() []SkillHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]SkillHandler, 0, len(r.handlers))
	for _, h := range r.handlers {
		out = append(out, h)
	}
	return out
}

// ToolsForSkills converts parsed SkillDefinitions into OpenRouter tool definitions.
// Only skills with builtin/llm executor types are included.
// Condition and handler-type skills are graph logic, not LLM-callable.
func (r *SkillRegistry) ToolsForSkills(skills map[string]*SkillDefinition) []openrouter.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []openrouter.Tool
	for _, skill := range skills {
		executorType, code := parseHandler(skill.Handler)
		if executorType == "condition" || executorType == "handler" {
			continue
		}

		h, ok := r.handlers[skill.Handler]
		if !ok {
			// Try matching by code (without executor prefix)
			for _, reg := range r.handlers {
				if reg.Name() == code || reg.Name() == skill.Handler {
					h = reg
					ok = true
					break
				}
			}
		}
		if !ok || h == nil {
			continue
		}

		def := h.Definition()
		tools = append(tools, openrouter.Tool{
			Type:     openrouter.ToolTypeFunction,
			Function: &def,
		})
	}
	return tools
}

// parseHandler splits "builtin:classify_intent" into ("builtin", "classify_intent").
func parseHandler(handler string) (executorType, code string) {
	parts := strings.SplitN(handler, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", handler
}

// parseExecutorType extracts just the executor type prefix from a handler string.
func parseExecutorType(handler string) string {
	t, _ := parseHandler(handler)
	return t
}
