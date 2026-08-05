package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"
)

// startRecoveryWorker runs a periodic loop that reclaims pending/abandoned executions.
// Gated on SKILL_RECOVERY_ENABLED=true env var AND IsRAGEnabled().
func (s *Service) startRecoveryWorker(ctx context.Context) {
	if os.Getenv("SKILL_RECOVERY_ENABLED") != "true" {
		return
	}
	if !s.IsRAGEnabled() {
		slog.Warn("Skill recovery worker disabled: RAG not enabled")
		return
	}

	slog.Info("Skill recovery worker started", "interval", "30s")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Skill recovery worker stopped")
			return
		case <-ticker.C:
			s.recoverPendingExecutions(ctx)
		}
	}
}

// recoverPendingExecutions finds and reclaims pending/abandoned executions.
func (s *Service) recoverPendingExecutions(ctx context.Context) {
	pending, err := s.store.ListPendingSkillExecutions(ctx)
	if err != nil {
		slog.Error("recovery: failed to list pending executions", "error", err)
		return
	}

	if len(pending) == 0 {
		return
	}

	slog.Info("recovery: found pending executions", "count", len(pending))

	for _, exec := range pending {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Deserialize the skill graph
		graph := &SkillGraph{}
		if err := json.Unmarshal([]byte(exec.SkillGraphJSON), graph); err != nil {
			slog.Error("recovery: failed to deserialize graph",
				"exec_id", exec.ID, "error", err)
			s.failExecution(ctx, exec, "recovery: deserialize graph failed")
			continue
		}

		if !graph.HasSkills {
			slog.Warn("recovery: execution has no skills, marking failed",
				"exec_id", exec.ID)
			s.failExecution(ctx, exec, "recovery: no skills in graph")
			continue
		}

		// Start async execution
		slog.Info("recovery: reclaiming execution", "exec_id", exec.ID)
		go s.runDetachedExecution(ctx, exec)
	}
}
