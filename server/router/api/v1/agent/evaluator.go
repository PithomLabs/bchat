package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
)

// ConditionResult holds the result of a CEL condition evaluation.
type ConditionResult struct {
	Met      bool
	Bindings map[string]any
}

// Standard CEL variables available in skill conditions.
var standardCELVars = []cel.EnvOption{
	cel.Variable("user_message", cel.StringType),
	cel.Variable("session_messages", cel.ListType(cel.StringType)),
	cel.Variable("urgency", cel.IntType),
	cel.Variable("customer_name", cel.StringType),
	cel.Variable("tenant_id", cel.IntType),
	cel.Variable("time_of_day", cel.StringType),
	cel.Variable("session_id", cel.StringType),
	cel.Variable("message_count", cel.IntType),
}

// EvalCondition evaluates a CEL expression against the provided variables.
// Returns whether the condition was met and any binding errors.
func EvalCondition(ctx context.Context, expr string, vars map[string]any) (*ConditionResult, error) {
	return EvalConditionWithTimeout(ctx, expr, vars, 5*time.Second)
}

// EvalConditionWithTimeout evaluates a CEL expression with a timeout.
func EvalConditionWithTimeout(ctx context.Context, expr string, vars map[string]any, timeout time.Duration) (*ConditionResult, error) {
	if expr == "" {
		return &ConditionResult{Met: true, Bindings: vars}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	env, err := cel.NewEnv(standardCELVars...)
	if err != nil {
		return nil, fmt.Errorf("cel env: %w", err)
	}

	ast, issues := env.Compile(expr)
	if issues != nil {
		return nil, fmt.Errorf("cel compile: %v", issues)
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("cel program: %w", err)
	}

	out, _, err := prg.Eval(vars)
	if err != nil {
		return nil, fmt.Errorf("cel eval: %w", err)
	}

	met, ok := out.Value().(bool)
	if !ok {
		return nil, fmt.Errorf("cel expr did not return bool: got %T", out.Value())
	}

	return &ConditionResult{Met: met, Bindings: vars}, nil
}

// CompileError indicates a CEL expression failed to compile (graph bug).
type CompileError struct {
	Expr string
	Err  error
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("cel compile %q: %v", e.Expr, e.Err)
}

func (e *CompileError) Unwrap() error { return e.Err }

// EvalConditionDynamic evaluates a CEL expression using dyn-typed variables.
// All variables from the graph node outputs and standard vars are declared as cel.DynType,
// so any key access is valid at the env level. Missing keys at eval time return Met=false
// instead of an error (N5-2 tolerant eval contract).
func EvalConditionDynamic(ctx context.Context, expr string, vars map[string]any, graph *SkillGraph) (*ConditionResult, error) {
	if expr == "" {
		return &ConditionResult{Met: true, Bindings: vars}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Build env: all standard vars + graph node outputs as DynType
	envOpts := []cel.EnvOption{
		cel.Variable("user_message", cel.DynType),
		cel.Variable("session_messages", cel.DynType),
		cel.Variable("urgency", cel.DynType),
		cel.Variable("customer_name", cel.DynType),
		cel.Variable("tenant_id", cel.DynType),
		cel.Variable("time_of_day", cel.DynType),
		cel.Variable("session_id", cel.DynType),
		cel.Variable("message_count", cel.DynType),
	}
	if graph != nil {
		for name := range graph.Nodes {
			envOpts = append(envOpts, cel.Variable(name, cel.DynType))
		}
	}

	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		return nil, fmt.Errorf("cel dynamic env: %w", err)
	}

	ast, issues := env.Compile(expr)
	if issues != nil {
		return nil, &CompileError{Expr: expr, Err: fmt.Errorf("%v", issues)}
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, &CompileError{Expr: expr, Err: err}
	}

	out, _, err := prg.Eval(vars)
	if err != nil {
		// N5-2: Tolerant eval — missing keys/fields -> Met=false
		if isMissingKeyError(err) {
			return &ConditionResult{Met: false, Bindings: vars}, nil
		}
		return nil, fmt.Errorf("cel eval: %w", err)
	}

	met, ok := out.Value().(bool)
	if !ok {
		return nil, fmt.Errorf("cel expr did not return bool: got %T", out.Value())
	}

	return &ConditionResult{Met: met, Bindings: vars}, nil
}

// isMissingKeyError checks if a CEL error is due to a missing key/variable.
func isMissingKeyError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no such key") ||
		strings.Contains(msg, "missing variable") ||
		strings.Contains(msg, "undeclared identifier") ||
		strings.Contains(msg, "no such attribute")
}
