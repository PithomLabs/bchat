package agent

import (
	"context"
	"testing"

	"github.com/revrost/go-openrouter"
)

// mockHandler is a test implementation of SkillHandler.
type mockHandler struct {
	name       string
	definition openrouter.FunctionDefinition
}

func (m *mockHandler) Name() string { return m.name }

func (m *mockHandler) Execute(_ context.Context, _ map[string]string, _ map[string]any) (string, error) {
	return "ok", nil
}

func (m *mockHandler) Definition() openrouter.FunctionDefinition { return m.definition }

func TestNewSkillRegistry(t *testing.T) {
	r := NewSkillRegistry()
	if r == nil {
		t.Fatal("NewSkillRegistry returned nil")
	}
	if len(r.List()) != 0 {
		t.Fatal("new registry should be empty")
	}
}

func TestRegisterAndGetHandler(t *testing.T) {
	r := NewSkillRegistry()
	h := &mockHandler{
		name: "builtin:classify_intent",
		definition: openrouter.FunctionDefinition{
			Name:        "classify_intent",
			Description: "Classify user intent",
		},
	}
	r.Register(h)

	got, ok := r.Get("builtin:classify_intent")
	if !ok {
		t.Fatal("expected to find handler")
	}
	if got.Name() != "builtin:classify_intent" {
		t.Fatalf("expected name builtin:classify_intent, got %s", got.Name())
	}
}

func TestGetNotFound(t *testing.T) {
	r := NewSkillRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r := NewSkillRegistry()
	r.Register(&mockHandler{name: "test"})
	r.Register(&mockHandler{name: "test"})
}

func TestList(t *testing.T) {
	r := NewSkillRegistry()
	r.Register(&mockHandler{name: "a"})
	r.Register(&mockHandler{name: "b"})
	r.Register(&mockHandler{name: "c"})
	if len(r.List()) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(r.List()))
	}
}

func TestToolsForSkills(t *testing.T) {
	r := NewSkillRegistry()
	r.Register(&mockHandler{
		name: "builtin:classify_intent",
		definition: openrouter.FunctionDefinition{
			Name:        "classify_intent",
			Description: "Classify intent",
		},
	})
	r.Register(&mockHandler{
		name: "llm:respond",
		definition: openrouter.FunctionDefinition{
			Name:        "respond",
			Description: "Generate response",
		},
	})

	skills := map[string]*SkillDefinition{
		"classify": {
			Name:    "classify",
			Handler: "builtin:classify_intent",
		},
		"respond": {
			Name:    "respond",
			Handler: "llm:respond",
		},
		"condition_check": {
			Name:      "condition_check",
			Handler:   "condition",
			Condition: "urgency > 3",
		},
	}

	tools := r.ToolsForSkills(skills)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools (condition excluded), got %d", len(tools))
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Function.Name] = true
	}
	if !names["classify_intent"] {
		t.Fatal("expected classify_intent tool")
	}
	if !names["respond"] {
		t.Fatal("expected respond tool")
	}
}

func TestToolsForSkills_SkipsUnregistered(t *testing.T) {
	r := NewSkillRegistry()
	skills := map[string]*SkillDefinition{
		"missing": {
			Name:    "missing",
			Handler: "builtin:nonexistent",
		},
	}
	tools := r.ToolsForSkills(skills)
	if len(tools) != 0 {
		t.Fatalf("expected 0 tools for unregistered handler, got %d", len(tools))
	}
}

func TestParseHandler(t *testing.T) {
	tests := []struct {
		input        string
		wantType     string
		wantCode     string
	}{
		{"builtin:classify_intent", "builtin", "classify_intent"},
		{"llm:respond", "llm", "respond"},
		{"condition", "", "condition"},
		{"handler", "", "handler"},
		{"builtin:classify:extra", "builtin", "classify:extra"},
	}
	for _, tt := range tests {
		gotType, gotCode := parseHandler(tt.input)
		if gotType != tt.wantType || gotCode != tt.wantCode {
			t.Errorf("parseHandler(%q) = (%q, %q), want (%q, %q)",
				tt.input, gotType, gotCode, tt.wantType, tt.wantCode)
		}
	}
}

func TestParseExecutorType(t *testing.T) {
	if got := parseExecutorType("builtin:classify_intent"); got != "builtin" {
		t.Fatalf("expected builtin, got %s", got)
	}
	if got := parseExecutorType("llm:respond"); got != "llm" {
		t.Fatalf("expected llm, got %s", got)
	}
	if got := parseExecutorType("condition"); got != "" {
		t.Fatalf("expected empty, got %s", got)
	}
}
