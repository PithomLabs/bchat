package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/usememos/memos/store"
)

var errStopSignal = fmt.Errorf("workflow stopped by signal")

// StartDetachedExecution creates and starts a workflow execution in the background.
func (s *Service) StartDetachedExecution(ctx context.Context, tenantID *int32, conversationID, triggerPath string, graph *SkillGraph, initialState map[string]any) (*store.SkillExecution, error) {
	if s.skillRegistry == nil {
		return nil, fmt.Errorf("skill registry not initialized")
	}
	if graph == nil || !graph.HasSkills {
		return nil, fmt.Errorf("no skills in graph")
	}

	exec, err := s.createExecution(ctx, tenantID, conversationID, triggerPath, graph, 3)
	if err != nil {
		return nil, fmt.Errorf("create execution: %w", err)
	}

	// Merge initial state into checkpoint
	if initialState != nil {
		if exec.CheckpointData == nil {
			exec.CheckpointData = make(map[string]any)
		}
		for k, v := range initialState {
			exec.CheckpointData[k] = v
		}
		if err := s.store.UpdateSkillExecution(ctx, exec); err != nil {
			return nil, fmt.Errorf("update initial state: %w", err)
		}
	}

	// Start async execution
	go func() {
		bgCtx := context.Background()
		s.runDetachedExecution(bgCtx, exec)
	}()

	return exec, nil
}

// StopExecution cancels a running execution via context cancellation.
func (s *Service) StopExecution(ctx context.Context, execID string) error {
	s.executionMu.Lock()
	cancel, ok := s.activeCancellations[execID]
	s.executionMu.Unlock()

	if ok {
		cancel()
	}

	return s.stopExecution(ctx, execID)
}

// runDetachedExecution runs a workflow execution to completion.
func (s *Service) runDetachedExecution(ctx context.Context, exec *store.SkillExecution) {
	// C1: Configurable whole-run deadline (default 15min, above 300s lease)
	wholeRunBudget := 15 * time.Minute
	if v := os.Getenv("SKILL_WHOLE_RUN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			wholeRunBudget = d
		}
	}
	ctx, cancel := context.WithTimeout(ctx, wholeRunBudget)
	defer cancel()

	// Store cancel func for stop endpoint
	s.executionMu.Lock()
	s.activeCancellations[exec.ID] = cancel
	s.executionMu.Unlock()
	defer func() {
		s.executionMu.Lock()
		delete(s.activeCancellations, exec.ID)
		s.executionMu.Unlock()
	}()

	workerID := "worker-" + uuid.New().String()[:8]

	// Claim the execution
	exec, err := s.claimExecution(ctx, exec.ID, workerID)
	if err != nil {
		slog.Error("failed to claim execution", "exec_id", exec.ID, "error", err)
		return
	}

	// Deserialize the skill graph
	graph := &SkillGraph{}
	if err := json.Unmarshal([]byte(exec.SkillGraphJSON), graph); err != nil {
		slog.Error("failed to deserialize skill graph", "exec_id", exec.ID, "error", err)
		s.failExecution(ctx, exec, fmt.Sprintf("deserialize graph: %v", err))
		return
	}

	// Run the workflow
	execErr := s.executeWorkflow(ctx, exec, graph, s.skillRegistry)

	// Re-read status with fresh context to detect stop/write race
	current, readErr := s.store.GetSkillExecution(context.Background(), &store.FindSkillExecution{ID: &exec.ID})
	if readErr != nil {
		slog.Error("failed to re-read status after execution", "exec_id", exec.ID, "error", readErr)
	}

	if errors.Is(execErr, errStopSignal) {
		slog.Info("workflow stopped by signal", "exec_id", exec.ID)
		return
	}

	if execErr != nil {
		slog.Error("workflow execution failed", "exec_id", exec.ID, "error", execErr)
		// Use bounded background context for error-path writes — the parent ctx may be expired
		// (whole-run deadline). Writes MUST succeed to prevent unbounded recovery re-claim.
		errCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// N-1: Chat-triggered executions are inline (user is waiting); retry in background
		// is meaningless. The recovery worker excludes trigger_path='chat' rows, so a
		// requeued chat execution would be stuck in pending forever. Fail immediately.
		if exec.TriggerPath == "chat" {
			s.failExecution(errCtx, exec, execErr.Error())
			return
		}
		// N-2: Handle terminal re-read failure — fall through using exec (pre-read state)
		// so retry accounting is not silently skipped.
		if current != nil && current.Status != "stopped" && current.Status != "completed" || current == nil {
			// C4: Classify retryability — don't retry permanent errors (R-4)
			if isPermanentError(execErr) {
				s.failExecution(errCtx, exec, execErr.Error())
				return
			}
			exec.RetryCount++
			if exec.RetryCount < exec.MaxRetries {
				slog.Info("retrying execution",
					"exec_id", exec.ID,
					"retry_count", exec.RetryCount,
					"max_retries", exec.MaxRetries)
				// C4: Single data-preserving write — bump retry, release claim, keep all maps (R-1 fix)
				exec.Status = "pending"
				exec.ClaimedBy = nil
				exec.ClaimedAt = nil
				exec.ClaimExpiresAt = nil
				exec.ErrorMessage = ""
				if err := s.store.UpdateSkillExecution(errCtx, exec); err != nil {
					slog.Error("failed to re-queue execution", "exec_id", exec.ID, "error", err)
					s.failExecution(errCtx, exec, execErr.Error())
				}
				return
			}
			s.failExecution(errCtx, exec, execErr.Error())
		}
		return
	}

	slog.Info("workflow execution finished", "exec_id", exec.ID)
}

// executeWorkflow runs a workflow DAG to completion.
func (s *Service) executeWorkflow(ctx context.Context, exec *store.SkillExecution, graph *SkillGraph, registry *SkillRegistry) error {
	// Build execution order (topological sort)
	order, err := topologicalSort(graph)
	if err != nil {
		return fmt.Errorf("topological sort: %w", err)
	}

	// Initialize state from checkpoint
	state := make(map[string]any)
	if exec.CheckpointData != nil {
		for k, v := range exec.CheckpointData {
			state[k] = v
		}
	}
	completed := make(map[string]bool)
	if exec.CompletedNodes != nil {
		for k, v := range exec.CompletedNodes {
			if b, ok := v.(bool); ok {
				completed[k] = b
			}
		}
	}

	// Execute each node in order
	for _, nodeName := range order {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("execution cancelled: %w", ctx.Err())
		default:
		}

		node, ok := graph.Nodes[nodeName]
		if !ok {
			continue
		}

		// Skip if already completed (resume from checkpoint)
		if completed[nodeName] {
			continue
		}

		// Check dependencies
		depsMet := true
		for _, dep := range node.DependsOn {
			if !completed[dep] {
				depsMet = false
				break
			}
		}
		if !depsMet {
			state[nodeName] = map[string]any{"output": "", "skipped": true}
			slog.Debug("skipping node: dependencies not met", "node", nodeName)
			continue
		}

		// Execute the node
		stepStart := time.Now()
		output, err := s.executeStep(ctx, exec, node, registry, state, graph)
		if err != nil {
			return fmt.Errorf("execute step %s: %w", nodeName, err)
		}

		// Mark as completed — wrap output for CEL field access (C3-2 contract)
		state[nodeName] = buildNodeOutput(output)
		completed[nodeName] = true

		// Write checkpoint after each step
		exec.CompletedNodes = make(map[string]any)
		for k, v := range completed {
			exec.CompletedNodes[k] = v
		}
		if err := s.writeCheckpoint(ctx, exec, state, nodeName); err != nil {
			return fmt.Errorf("checkpoint after %s: %w", nodeName, err)
		}

		// Log the step
		s.logSkillStep(ctx, exec, node, output, stepStart)

		// Check stop condition after each step
		if graph.Stop != nil && graph.Stop.Condition != "" {
			celVars := make(map[string]any)
			for k, v := range state {
				celVars[k] = v
			}
			if exec.TenantID != nil {
				celVars["tenant_id"] = *exec.TenantID
			}
			celVars["session_id"] = exec.ConversationID

			result, evalErr := EvalConditionDynamic(ctx, graph.Stop.Condition, celVars, graph)
			if evalErr != nil {
				// C5: Hard-fail on compile errors (consistent with node-condition handling at :258-261)
				var compileErr *CompileError
				if errors.As(evalErr, &compileErr) {
					return fmt.Errorf("stop condition compile error: %w", evalErr)
				}
				slog.Warn("stop condition eval error, treating as not met",
					"exec_id", exec.ID, "condition", graph.Stop.Condition, "error", evalErr)
				result = nil
			}
			if result != nil && result.Met {
				slog.Info("stop condition matched", "exec_id", exec.ID, "condition", graph.Stop.Condition)
				// Write 'stopped' before returning sentinel
				if stopErr := s.store.StopSkillExecution(ctx, exec.ID); stopErr != nil {
					slog.Error("failed to write stopped status", "exec_id", exec.ID, "error", stopErr)
				}
				// C3/R-8: Dispatch EmitEvent if configured
				if graph.Stop.EmitEvent != "" && exec.TenantID != nil {
					leadID := ""
					if exec.ConversationID != "" {
						leadID = exec.ConversationID
					}
					s.dispatchEvent(ctx, *exec.TenantID, leadID, graph.Stop.EmitEvent, "")
				}
				return errStopSignal
			}
		}
	}

	// Build final output from all completed nodes
	output := buildWorkflowOutput(state, completed)
	return s.completeExecution(ctx, exec, output)
}

// executeStep runs a single workflow step (condition or skill).
func (s *Service) executeStep(ctx context.Context, exec *store.SkillExecution, node *SkillDefinition, registry *SkillRegistry, state map[string]any, graph *SkillGraph) (string, error) {
	start := time.Now()

	// Build CEL variables from state
	celVars := make(map[string]any)
	for k, v := range state {
		celVars[k] = v
	}
	// Add standard variables
	if exec.TenantID != nil {
		celVars["tenant_id"] = *exec.TenantID
	}
	celVars["session_id"] = exec.ConversationID

	// Evaluate condition if present (N5-2: tolerant eval for missing keys)
	if node.Condition != "" {
		result, err := EvalConditionDynamic(ctx, node.Condition, celVars, graph)
		if err != nil {
			var compileErr *CompileError
			if errors.As(err, &compileErr) {
				return "", fmt.Errorf("condition compile error for %s: %w", node.Name, err)
			}
			slog.Warn("condition eval error, treating node as skipped",
				"exec_id", exec.ID, "node", node.Name,
				"condition", node.Condition, "error", err)
			return "", nil
		}
		if !result.Met {
			slog.Debug("condition not met, skipping", "node", node.Name, "condition", node.Condition)
			return "", nil
		}
	}

	// Get handler
	executorType, code := parseHandler(node.Handler)
	if executorType == "condition" || executorType == "handler" || node.Handler == "condition" || node.Handler == "handler" {
		// Graph-only nodes, just mark as completed
		return fmt.Sprintf("graph:%s", node.Name), nil
	}

	h, ok := registry.Get(node.Handler)
	if !ok {
		// Try by code
		h, ok = registry.Get(code)
	}
	if !ok {
		return "", fmt.Errorf("handler not found: %s", node.Handler)
	}

	// C1: Per-node timeout enforcement
	execCtx := ctx
	if node.Timeout != "" {
		d, parseErr := time.ParseDuration(node.Timeout)
		if parseErr != nil {
			slog.Warn("invalid timeout, ignoring",
				"node", node.Name, "timeout", node.Timeout, "error", parseErr)
		} else {
			if d <= 0 {
				slog.Warn("non-positive timeout, ignoring",
					"node", node.Name, "timeout", node.Timeout)
			} else {
				if d > 280*time.Second {
					slog.Warn("timeout exceeds safe bound, clamping to 280s",
						"node", node.Name, "timeout", node.Timeout)
					d = 280 * time.Second
				}
				var cancel context.CancelFunc
				execCtx, cancel = context.WithTimeout(ctx, d)
				defer cancel()
			}
		}
	}

	// Execute handler
	output, err := h.Execute(execCtx, node.Params, celVars)
	if err != nil {
		return "", err
	}

	duration := time.Since(start)
	slog.Debug("step executed",
		"node", node.Name,
		"handler", node.Handler,
		"duration_ms", duration.Milliseconds(),
		"output_len", len(output))

	return output, nil
}

// logSkillStep creates a skill log entry for audit trail.
func (s *Service) logSkillStep(ctx context.Context, exec *store.SkillExecution, node *SkillDefinition, output string, start time.Time) {
	log := &store.SkillLog{
		ID:          uuid.New().String(),
		TenantID:    exec.TenantID,
		ExecutionID: exec.ID,
		SkillName:   node.Name,
		Handler:     node.Handler,
		Status:      "completed",
		Input:       map[string]any{"params": node.Params},
		Output:      map[string]any{"result": output},
		DurationMs:  int(time.Since(start).Milliseconds()),
		StartedAt:   start,
	}

	if err := s.store.CreateSkillLog(ctx, log); err != nil {
		slog.Error("failed to create skill log", "exec_id", exec.ID, "node", node.Name, "error", err)
	}
}

// topologicalSort returns nodes in dependency order.
func topologicalSort(graph *SkillGraph) ([]string, error) {
	// Kahn's algorithm
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for name, node := range graph.Nodes {
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
		for _, dep := range node.DependsOn {
			inDegree[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}

	// Start with nodes that have no dependencies
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue) // deterministic order

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, dep := range dependents[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
				sort.Strings(queue)
			}
		}
	}

	if len(order) != len(graph.Nodes) {
		return nil, fmt.Errorf("cycle detected in skill graph")
	}

	return order, nil
}

// buildNodeOutput returns a map for CEL field access (C3-2 contract):
//   - JSON object  → parsed map (float64→int for whole numbers, recursively)
//   - any other    → {"output": <raw string>}
func buildNodeOutput(output string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(output), &m); err != nil || m == nil {
		return map[string]any{"output": output}
	}
	normalizeNumbers(m)
	return m
}

// normalizeNumbers converts integral float64 (including nested) to int64 for
// canonical representation. CEL handles int==double natively, but canonicalizing
// avoids widening surprises in checkpoint serialization and keeps state maps tidy.
func normalizeNumbers(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			switch nv := val.(type) {
			case float64:
				if nv == float64(int64(nv)) {
					t[k] = int64(nv)
				}
			case map[string]any, []any:
				normalizeNumbers(val)
			}
		}
	case []any:
		for i, val := range t {
			switch nv := val.(type) {
			case float64:
				if nv == float64(int64(nv)) {
					t[i] = int64(nv)
				}
			default:
				normalizeNumbers(val)
			}
		}
	}
}

// buildWorkflowOutput creates a human-readable output from completed steps.
func buildWorkflowOutput(state map[string]any, completed map[string]bool) string {
	var parts []string
	for name, done := range completed {
		if !done {
			continue
		}
		output, ok := state[name]
		if !ok {
			continue
		}
		display := fmt.Sprintf("%v", output)
		if m, ok := output.(map[string]any); ok {
			if out, exists := m["output"]; exists {
				display = fmt.Sprintf("%v", out)
			}
		}
		parts = append(parts, fmt.Sprintf("%s: %s", name, display))
	}
	if len(parts) == 0 {
		return "workflow completed (no output)"
	}
	return strings.Join(parts, "\n")
}

// isPermanentError classifies errors that should not be retried.
// WARNING: string-match based — a handler error containing "handler not found"
// or "deserialize graph" will be misclassified as permanent. This is acceptable
// for v1 because the engine's own errors use these exact phrases; handlers should
// avoid them in user-facing messages.
func isPermanentError(err error) bool {
	var compileErr *CompileError
	if errors.As(err, &compileErr) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false // transient — timeout may succeed on retry
	}
	msg := err.Error()
	return strings.Contains(msg, "handler not found") ||
		strings.Contains(msg, "deserialize graph")
}
