package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

func TestTopologicalSort_Linear(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"a": {Name: "a", Handler: "builtin:log", DependsOn: nil},
			"b": {Name: "b", Handler: "builtin:log", DependsOn: []string{"a"}},
			"c": {Name: "c", Handler: "builtin:log", DependsOn: []string{"b"}},
		},
	}

	order, err := topologicalSort(graph)
	if err != nil {
		t.Fatal(err)
	}

	// a must come before b, b before c
	idx := map[string]int{}
	for i, name := range order {
		idx[name] = i
	}
	if idx["a"] >= idx["b"] || idx["b"] >= idx["c"] {
		t.Fatalf("wrong order: %v", order)
	}
}

func TestTopologicalSort_Diamond(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"a": {Name: "a", Handler: "builtin:log"},
			"b": {Name: "b", Handler: "builtin:log", DependsOn: []string{"a"}},
			"c": {Name: "c", Handler: "builtin:log", DependsOn: []string{"a"}},
			"d": {Name: "d", Handler: "builtin:log", DependsOn: []string{"b", "c"}},
		},
	}

	order, err := topologicalSort(graph)
	if err != nil {
		t.Fatal(err)
	}

	idx := map[string]int{}
	for i, name := range order {
		idx[name] = i
	}
	if idx["a"] >= idx["b"] || idx["a"] >= idx["c"] {
		t.Fatal("a must come before b and c")
	}
	if idx["b"] >= idx["d"] || idx["c"] >= idx["d"] {
		t.Fatal("b and c must come before d")
	}
}

func TestTopologicalSort_Cycle(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"a": {Name: "a", Handler: "builtin:log", DependsOn: []string{"b"}},
			"b": {Name: "b", Handler: "builtin:log", DependsOn: []string{"a"}},
		},
	}

	_, err := topologicalSort(graph)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestBuildWorkflowOutput(t *testing.T) {
	state := map[string]any{
		"step1": "output1",
		"step2": "output2",
	}
	completed := map[string]bool{
		"step1": true,
		"step2": true,
	}

	output := buildWorkflowOutput(state, completed)
	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestBuildWorkflowOutput_Empty(t *testing.T) {
	output := buildWorkflowOutput(nil, nil)
	if output != "workflow completed (no output)" {
		t.Fatalf("expected default message, got %s", output)
	}
}

func TestExecuteStep_SkipsConditionNodes(t *testing.T) {
	node := &SkillDefinition{
		Name:    "condition_check",
		Handler: "condition",
	}
	registry := NewSkillRegistry()
	exec := &store.SkillExecution{
		ID:             "test-exec",
		ConversationID: "conv-1",
	}

	output, err := executeStepHelper(t, context.Background(), exec, node, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output != "graph:condition_check" {
		t.Fatalf("expected graph:condition_check, got %s", output)
	}
}

func TestExecuteStep_HandlerNotFound(t *testing.T) {
	node := &SkillDefinition{
		Name:    "unknown_skill",
		Handler: "builtin:nonexistent",
	}
	registry := NewSkillRegistry()
	exec := &store.SkillExecution{
		ID:             "test-exec",
		ConversationID: "conv-1",
	}

	_, err := executeStepHelper(t, context.Background(), exec, node, registry, nil)
	if err == nil {
		t.Fatal("expected error for unknown handler")
	}
}

func TestExecuteStep_ConditionNotMet(t *testing.T) {
	node := &SkillDefinition{
		Name:      "conditional_skill",
		Handler:   "builtin:log",
		Condition: "urgency > 5",
		Params:    map[string]string{"message": "test"},
	}
	registry := NewSkillRegistry()
	RegisterBuiltins(registry)

	exec := &store.SkillExecution{
		ID:             "test-exec",
		ConversationID: "conv-1",
	}
	state := map[string]any{"urgency": 2}

	output, err := executeStepHelper(t, context.Background(), exec, node, registry, state)
	if err != nil {
		t.Fatal(err)
	}
	if output != "" {
		t.Fatalf("expected empty output when condition not met, got %s", output)
	}
}

func TestExecuteStep_ConditionMet(t *testing.T) {
	node := &SkillDefinition{
		Name:      "log_skill",
		Handler:   "builtin:log",
		Condition: "urgency > 3",
		Params:    map[string]string{"message": "high urgency logged"},
	}
	registry := NewSkillRegistry()
	RegisterBuiltins(registry)

	exec := &store.SkillExecution{
		ID:             "test-exec",
		ConversationID: "conv-1",
	}
	state := map[string]any{"urgency": 5}

	output, err := executeStepHelper(t, context.Background(), exec, node, registry, state)
	if err != nil {
		t.Fatal(err)
	}
	if output != "logged" {
		t.Fatalf("expected 'logged', got %s", output)
	}
}

func TestExecuteStep_WithParams(t *testing.T) {
	node := &SkillDefinition{
		Name:    "log_msg",
		Handler: "builtin:log",
		Params:  map[string]string{"level": "warn", "message": "test warning"},
	}
	registry := NewSkillRegistry()
	RegisterBuiltins(registry)

	exec := &store.SkillExecution{
		ID:             "test-exec",
		ConversationID: "conv-1",
	}

	output, err := executeStepHelper(t, context.Background(), exec, node, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output != "logged" {
		t.Fatalf("expected 'logged', got %s", output)
	}
}

func TestSkillGraphJSON_RoundTrip(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"a": {Name: "a", Handler: "builtin:log", Params: map[string]string{"message": "hello"}},
		},
		EntryPoints: []string{"a"},
		HasSkills:   true,
	}

	data, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}

	var decoded SkillGraph
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if !decoded.HasSkills {
		t.Fatal("expected HasSkills to be true")
	}
	if len(decoded.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(decoded.Nodes))
	}
}

// executeStepHelper is a test wrapper that calls executeStep directly.
func executeStepHelper(t *testing.T, ctx context.Context, exec *store.SkillExecution, node *SkillDefinition, registry *SkillRegistry, state map[string]any) (string, error) {
	t.Helper()
	svc := &Service{
		skillRegistry: registry,
	}
	return svc.executeStep(ctx, exec, node, registry, state, nil)
}

func TestExecuteStep_Timeout(t *testing.T) {
	ctx := context.Background()
	exec := &store.SkillExecution{ID: "t", TenantID: nil, ConversationID: "c"}
	node := &SkillDefinition{
		Name:    "slow",
		Handler: "builtin:sleep",
		Params:  map[string]string{"duration": "30"},
		Timeout: "1s",
	}
	registry := NewSkillRegistry()
	RegisterBuiltins(registry)
	_, err := executeStepHelper(t, ctx, exec, node, registry, map[string]any{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected error containing 'deadline', got: %v", err)
	}
}

func TestExecuteWorkflow_DAGBuiltin(t *testing.T) {
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	registry := NewSkillRegistry()
	RegisterBuiltins(registry)

	graph := &SkillGraph{
		HasSkills: true,
		Nodes: map[string]*SkillDefinition{
			"step1": {Name: "step1", Handler: "builtin:log", Params: map[string]string{"message": "hello"}},
			"step2": {Name: "step2", Handler: "builtin:log", Params: map[string]string{"message": "world"}, DependsOn: []string{"step1"}},
		},
		EntryPoints: []string{"step1"},
	}

	execID := uuid.New().String()
	exec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             execID,
		Status:         "running",
		SkillGraphJSON: "{}",
		TriggerPath:    "test",
		MaxRetries:     3,
	})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}

	svc := &Service{
		store:         ts,
		skillRegistry: registry,
	}

	err = svc.executeWorkflow(ctx, exec, graph, registry)
	if err != nil {
		t.Fatalf("executeWorkflow: %v", err)
	}

	// Verify execution completed
	updated, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &execID})
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.Status != "completed" {
		t.Fatalf("expected status completed, got %s", updated.Status)
	}

	// Verify both steps completed
	if len(exec.CompletedNodes) != 2 {
		t.Fatalf("expected 2 completed nodes, got %d", len(exec.CompletedNodes))
	}
}

func TestIsPermanentError_Table(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		permanent  bool
	}{
		{
			name:      "CompileError is permanent",
			err:       &CompileError{Expr: "x", Err: fmt.Errorf("bad syntax")},
			permanent: true,
		},
		{
			name:      "handler not found is permanent",
			err:       fmt.Errorf("handler not found: foo"),
			permanent: true,
		},
		{
			name:      "deserialize graph is permanent",
			err:       fmt.Errorf("deserialize graph: bad json"),
			permanent: true,
		},
		{
			name:      "DeadlineExceeded is transient",
			err:       context.DeadlineExceeded,
			permanent: false,
		},
		{
			name:      "random error is transient",
			err:       errors.New("something broke"),
			permanent: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPermanentError(tt.err)
			if got != tt.permanent {
				t.Fatalf("isPermanentError(%v) = %v, want %v", tt.err, got, tt.permanent)
			}
		})
	}
}

func TestRetryRequeue_SkipsChat(t *testing.T) {
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	registry := NewSkillRegistry()
	RegisterBuiltins(registry)

	// Build a graph with a handler that fails (unknown handler → permanent error)
	graph := &SkillGraph{
		HasSkills: true,
		Nodes: map[string]*SkillDefinition{
			"bad": {Name: "bad", Handler: "builtin:nonexistent"},
		},
		EntryPoints: []string{"bad"},
	}
	graphJSON, _ := json.Marshal(graph)

	// Create a chat-triggered execution in pending status with the graph already set
	execID := uuid.New().String()
	exec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             execID,
		Status:         "pending",
		SkillGraphJSON: string(graphJSON),
		TriggerPath:    "chat",
		MaxRetries:     3,
	})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}

	svc := &Service{
		store:               ts,
		skillRegistry:       registry,
		activeCancellations: make(map[string]context.CancelFunc),
	}

	svc.runDetachedExecution(ctx, exec)

	// Verify execution failed (not requeued to pending)
	updated, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &execID})
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.Status != "failed" {
		t.Fatalf("expected status failed for chat-triggered, got %s", updated.Status)
	}
	if updated.RetryCount != 0 {
		t.Fatalf("expected retry_count 0 (no retry for chat), got %d", updated.RetryCount)
	}
}

func TestRunDetachedExecution_PermanentFailsImmediately(t *testing.T) {
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	registry := NewSkillRegistry()
	RegisterBuiltins(registry)

	graph := &SkillGraph{
		HasSkills: true,
		Nodes: map[string]*SkillDefinition{
			"bad": {Name: "bad", Handler: "builtin:nonexistent"},
		},
		EntryPoints: []string{"bad"},
	}
	graphJSON, _ := json.Marshal(graph)

	exec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		Status:         "pending",
		SkillGraphJSON: string(graphJSON),
		TriggerPath:    "api",
		MaxRetries:     3,
	})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}

	svc := &Service{
		store:               ts,
		skillRegistry:       registry,
		activeCancellations: make(map[string]context.CancelFunc),
	}

	svc.runDetachedExecution(ctx, exec)

	updated, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.Status != "failed" {
		t.Fatalf("expected status failed (permanent error), got %s", updated.Status)
	}
	if updated.RetryCount != 0 {
		t.Fatalf("expected retry_count 0 (permanent error, no retry), got %d", updated.RetryCount)
	}
}

func TestExecuteStep_NonPositiveTimeout(t *testing.T) {
	ctx := context.Background()
	exec := &store.SkillExecution{ID: "t", TenantID: nil, ConversationID: "c"}
	node := &SkillDefinition{
		Name:    "sleep",
		Handler: "builtin:sleep",
		Params:  map[string]string{"duration": "1"},
		Timeout: "0s",
	}
	registry := NewSkillRegistry()
	RegisterBuiltins(registry)

	output, err := executeStepHelper(t, ctx, exec, node, registry, map[string]any{})
	if err != nil {
		t.Fatalf("expected no error for zero timeout (ignored), got: %v", err)
	}
	if output != "slept 1s" {
		t.Fatalf("expected 'slept 1s', got: %s", output)
	}
}

func TestRunDetachedExecution_RetryPreservesCheckpoint(t *testing.T) {
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	defer ts.Close()

	// Create a fresh registry with only FailingHandler (no builtins to avoid duplicate)
	registry := NewSkillRegistry()
	registry.Register(&FailingHandler{FailError: context.DeadlineExceeded})

	graph := &SkillGraph{
		HasSkills: true,
		Nodes: map[string]*SkillDefinition{
			"step": {Name: "step", Handler: "builtin:failing"},
		},
		EntryPoints: []string{"step"},
	}
	graphJSON, _ := json.Marshal(graph)

	exec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		Status:         "pending",
		SkillGraphJSON: string(graphJSON),
		TriggerPath:    "api",
		MaxRetries:     3,
		CheckpointData: map[string]any{"key": "value"},
	})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}

	svc := &Service{
		store:               ts,
		skillRegistry:       registry,
		activeCancellations: make(map[string]context.CancelFunc),
	}

	svc.runDetachedExecution(ctx, exec)

	updated, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	// After first transient failure: requeued to pending, RetryCount=1
	if updated.Status != "pending" {
		t.Fatalf("expected status pending after requeue, got %s", updated.Status)
	}
	if updated.RetryCount != 1 {
		t.Fatalf("expected retry_count 1, got %d", updated.RetryCount)
	}
	if updated.CheckpointData == nil {
		t.Fatal("expected CheckpointData to be preserved")
	}
	if updated.CheckpointData["key"] != "value" {
		t.Fatalf("expected CheckpointData['key']='value', got %v", updated.CheckpointData["key"])
	}
}
