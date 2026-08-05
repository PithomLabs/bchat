package agent

import (
	"context"
	"testing"
	"time"
)

func TestLogHandler(t *testing.T) {
	h := &LogHandler{}
	if h.Name() != "builtin:log" {
		t.Fatalf("expected builtin:log, got %s", h.Name())
	}
	result, err := h.Execute(context.Background(), map[string]string{
		"level":   "info",
		"message": "test log entry",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "logged" {
		t.Fatalf("expected 'logged', got %s", result)
	}
}

func TestLogHandler_DefaultLevel(t *testing.T) {
	h := &LogHandler{}
	result, err := h.Execute(context.Background(), map[string]string{
		"message": "no level specified",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "logged" {
		t.Fatalf("expected 'logged', got %s", result)
	}
}

func TestSleepHandler(t *testing.T) {
	h := &SleepHandler{}
	if h.Name() != "builtin:sleep" {
		t.Fatalf("expected builtin:sleep, got %s", h.Name())
	}

	start := time.Now()
	result, err := h.Execute(context.Background(), map[string]string{
		"duration": "1",
	}, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if result != "slept 1s" {
		t.Fatalf("expected 'slept 1s', got %s", result)
	}
	if elapsed < 900*time.Millisecond {
		t.Fatal("expected at least ~1s sleep")
	}
}

func TestSleepHandler_ContextCancelled(t *testing.T) {
	h := &SleepHandler{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := h.Execute(ctx, map[string]string{
		"duration": "5",
	}, nil)
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestSleepHandler_InvalidDuration(t *testing.T) {
	h := &SleepHandler{}
	_, err := h.Execute(context.Background(), map[string]string{
		"duration": "abc",
	}, nil)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestLLMHandler_NoGenerateFn(t *testing.T) {
	h := &LLMHandler{}
	_, err := h.Execute(context.Background(), map[string]string{
		"prompt": "summarize this",
	}, nil)
	if err == nil {
		t.Fatal("expected error when GenerateFn is nil")
	}
}

func TestLLMHandler_NoPrompt(t *testing.T) {
	h := &LLMHandler{
		GenerateFn: func(_ context.Context, _ string, _ map[string]any) (string, error) {
			return "response", nil
		},
	}
	_, err := h.Execute(context.Background(), map[string]string{}, nil)
	if err == nil {
		t.Fatal("expected error when prompt is empty")
	}
}

func TestLLMHandler_WithPrompt(t *testing.T) {
	called := false
	h := &LLMHandler{
		GenerateFn: func(_ context.Context, prompt string, _ map[string]any) (string, error) {
			called = true
			if prompt == "" {
				t.Fatal("expected non-empty prompt")
			}
			return "llm response", nil
		},
	}
	result, err := h.Execute(context.Background(), map[string]string{
		"prompt": "summarize the conversation",
	}, map[string]any{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("GenerateFn was not called")
	}
	if result != "llm response" {
		t.Fatalf("expected 'llm response', got %s", result)
	}
}

func TestLLMHandler_MessageFallback(t *testing.T) {
	called := false
	h := &LLMHandler{
		GenerateFn: func(_ context.Context, prompt string, _ map[string]any) (string, error) {
			called = true
			return "ok", nil
		},
	}
	_, err := h.Execute(context.Background(), map[string]string{
		"message": "use message param",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("GenerateFn was not called")
	}
}
