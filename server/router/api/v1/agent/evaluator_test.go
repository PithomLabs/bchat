package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestEvalCondition_Simple(t *testing.T) {
	result, err := EvalCondition(context.Background(), "true", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Met {
		t.Fatal("expected true")
	}
}

func TestEvalCondition_False(t *testing.T) {
	result, err := EvalCondition(context.Background(), "false", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Met {
		t.Fatal("expected false")
	}
}

func TestEvalCondition_WithBindings(t *testing.T) {
	vars := map[string]any{
		"urgency": 5,
	}
	result, err := EvalCondition(context.Background(), "urgency > 3", vars)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Met {
		t.Fatal("expected urgency > 3 to be true")
	}
}

func TestEvalCondition_LessThan(t *testing.T) {
	vars := map[string]any{
		"urgency": 2,
	}
	result, err := EvalCondition(context.Background(), "urgency > 3", vars)
	if err != nil {
		t.Fatal(err)
	}
	if result.Met {
		t.Fatal("expected urgency > 3 to be false for urgency=2")
	}
}

func TestEvalCondition_StringComparison(t *testing.T) {
	vars := map[string]any{
		"customer_name": "Alice",
	}
	result, err := EvalCondition(context.Background(), `customer_name == "Alice"`, vars)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Met {
		t.Fatal("expected customer_name == Alice to be true")
	}
}

func TestEvalCondition_ComplexExpr(t *testing.T) {
	vars := map[string]any{
		"urgency":       5,
		"customer_name": "Bob",
	}
	result, err := EvalCondition(context.Background(), `urgency > 3 && customer_name == "Bob"`, vars)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Met {
		t.Fatal("expected compound expression to be true")
	}
}

func TestEvalCondition_EmptyExpr(t *testing.T) {
	result, err := EvalCondition(context.Background(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Met {
		t.Fatal("empty expression should default to true")
	}
}

func TestEvalCondition_InvalidExpr(t *testing.T) {
	_, err := EvalCondition(context.Background(), "invalid !!! syntax", nil)
	if err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

func TestEvalCondition_NonBoolReturn(t *testing.T) {
	_, err := EvalCondition(context.Background(), `"not a bool"`, nil)
	if err == nil {
		t.Fatal("expected error for non-bool return")
	}
}

func TestEvalCondition_WithTimeout(t *testing.T) {
	vars := map[string]any{
		"message_count": 10,
	}
	result, err := EvalConditionWithTimeout(context.Background(), "message_count > 5", vars, 1*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Met {
		t.Fatal("expected message_count > 5 to be true")
	}
}

func TestEvalCondition_UnknownVar(t *testing.T) {
	// Unknown variables should cause a compile error
	_, err := EvalCondition(context.Background(), "unknown_var > 0", nil)
	if err == nil {
		t.Fatal("expected error for unknown variable")
	}
}

func TestEvalConditionDynamic_WrapperMap_NoFieldAccess(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"search_kb": {Name: "search_kb", Handler: "search_kb"},
		},
	}
	vars := map[string]any{
		"search_kb": map[string]any{"output": "logged"},
	}
	result, err := EvalConditionDynamic(context.Background(), `search_kb.found == false`, vars, graph)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Met {
		t.Fatal("expected Met=false (field not present in wrapper map)")
	}
}

func TestEvalConditionDynamic_RawString_NoFieldAccess(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"search_kb": {Name: "search_kb", Handler: "search_kb"},
		},
	}
	vars := map[string]any{
		"search_kb": "logged",
	}
	result, err := EvalConditionDynamic(context.Background(), `search_kb.found == false`, vars, graph)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Met {
		t.Fatal("expected Met=false (field access on raw string returns nil)")
	}
}

func TestEvalConditionDynamic_MapStringCompare_ReturnsFalse(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"search_kb": {Name: "search_kb", Handler: "search_kb"},
		},
	}
	vars := map[string]any{
		"search_kb": map[string]any{"found": true},
	}
	result, err := EvalConditionDynamic(context.Background(), `search_kb == "logged"`, vars, graph)
	if err != nil {
		t.Fatalf("expected no error (CEL DynType handles type mismatch gracefully), got: %v", err)
	}
	if result.Met {
		t.Fatal("expected Met=false (map != string)")
	}
}

func TestBuildNodeOutput_FlatJSON(t *testing.T) {
	output := `{"found":true,"ticket_id":"T1"}`
	result := buildNodeOutput(output)
	if found, ok := result["found"].(bool); !ok || !found {
		t.Fatalf("expected found=true, got %v", result["found"])
	}
}

func TestBuildNodeOutput_NestedJSON(t *testing.T) {
	output := `{"meta":{"count":5},"name":"test"}`
	result := buildNodeOutput(output)
	meta, ok := result["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", result["meta"])
	}
	count, ok := meta["count"].(int64)
	if !ok || count != 5 {
		t.Fatalf("expected count=5 (int64), got %v %T", meta["count"], meta["count"])
	}
}

func TestBuildNodeOutput_ArrayJSON(t *testing.T) {
	output := `[1,2,3]`
	result := buildNodeOutput(output)
	if out, ok := result["output"].(string); !ok || out != output {
		t.Fatalf("expected output=%s, got %v", output, result["output"])
	}
}

func TestBuildNodeOutput_PlainString(t *testing.T) {
	output := "hello world"
	result := buildNodeOutput(output)
	if out, ok := result["output"].(string); !ok || out != output {
		t.Fatalf("expected output=%s, got %v", output, result["output"])
	}
}

func TestBuildNodeOutput_InvalidJSON(t *testing.T) {
	output := "not json at all"
	result := buildNodeOutput(output)
	if out, ok := result["output"].(string); !ok || out != output {
		t.Fatalf("expected output=%s, got %v", output, result["output"])
	}
}

func TestBuildNodeOutput_NumberNormalization(t *testing.T) {
	raw := `{"count":5,"ratio":2.5,"nested":{"x":10}}`
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["count"].(float64); !ok {
		t.Fatal("precondition: count should be float64 from JSON unmarshal")
	}

	result := buildNodeOutput(raw)
	// After normalization: count is int64
	if count, ok := result["count"].(int64); !ok || count != 5 {
		t.Fatalf("expected count=5 (int64), got %v %T", result["count"], result["count"])
	}
	// ratio stays float64 (not integral)
	if ratio, ok := result["ratio"].(float64); !ok || ratio != 2.5 {
		t.Fatalf("expected ratio=2.5 (float64), got %v %T", result["ratio"], result["ratio"])
	}
	// nested.x is int64
	nested, ok := result["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", result["nested"])
	}
	if x, ok := nested["x"].(int64); !ok || x != 10 {
		t.Fatalf("expected nested.x=10 (int64), got %v %T", nested["x"], nested["x"])
	}
}

func TestEvalConditionDynamic_CompileError(t *testing.T) {
	ctx := context.Background()
	graph := &SkillGraph{Nodes: map[string]*SkillDefinition{
		"search_kb": {Name: "search_kb", Handler: "builtin:log"},
	}}
	// "treu" is a valid identifier but undeclared → check-time error → CompileError
	_, err := EvalConditionDynamic(ctx, "search_kb.found == treu", map[string]any{}, graph)
	if err == nil {
		t.Fatal("expected error for undeclared identifier")
	}
	var compileErr *CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("expected CompileError, got %T: %v", err, err)
	}
}

func TestEvalConditionDynamic_RuntimeError(t *testing.T) {
	ctx := context.Background()
	graph := &SkillGraph{Nodes: map[string]*SkillDefinition{
		"x": {Name: "x", Handler: "builtin:log"},
	}}
	// "x" declared as DynType; zero at eval time → genuine runtime division error
	_, err := EvalConditionDynamic(ctx, "1 / x", map[string]any{"x": int64(0)}, graph)
	if err == nil {
		t.Fatal("expected runtime error for division by zero")
	}
	var compileErr *CompileError
	if errors.As(err, &compileErr) {
		t.Fatalf("runtime error should not be CompileError, got %T: %v", err, err)
	}
}
