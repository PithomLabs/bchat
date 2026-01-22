package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/usememos/memos/store"
)

// BeadsService handles all bd CLI interactions
type BeadsService struct {
	store *store.Store
}

func NewBeadsService(s *store.Store) *BeadsService {
	return &BeadsService{store: s}
}

// BeadsIssueRequest represents a request to create a beads issue
type BeadsIssueRequest struct {
	Title       string
	Description string
	Type        string // bug, feature, task, epic, chore, docs, investigation
	Priority    int    // 0-4
	Labels      []string
}

// BeadsIssueResponse represents the created beads issue
type BeadsIssueResponse struct {
	BeadsID     string
	Title       string
	Description string
	Type        string
	Priority    int
	Labels      []string
	CreatedTs   int64
}

// CreateIssue creates a beads issue via bd create and returns the beads_id
func (s *BeadsService) CreateIssue(ctx context.Context, req *BeadsIssueRequest) (*BeadsIssueResponse, error) {
	// Validate inputs
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	// TODO: Implement store.ValidateBeadsType after defining valid types
	// if err := store.ValidateBeadsType(req.Type); err != nil {
	// 	return nil, fmt.Errorf("invalid issue type: %s", req.Type)
	// }
	if req.Priority < 0 || req.Priority > 4 {
		return nil, fmt.Errorf("priority must be between 0 and 4")
	}

	// Build bd create command
	args := []string{
		"create",
		req.Title,
		"-t", req.Type,
		"-p", fmt.Sprintf("%d", req.Priority),
	}

	if req.Description != "" {
		args = append(args, "-d", req.Description)
	}

	if len(req.Labels) > 0 {
		args = append(args, "--label", strings.Join(req.Labels, ","))
	}

	slog.Info("Executing bd create", "args", args)

	// Execute bd create
	cmd := exec.CommandContext(ctx, "bd", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("bd create failed", "error", err, "output", string(output))
		return nil, fmt.Errorf("bd create failed: %w (output: %s)", err, string(output))
	}

	slog.Info("bd create output", "output", string(output))

	// Parse beads_id from output
	beadsID, err := parseBeadsIDFromOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse beads_id from output: %w", err)
	}

	// Sync with git
	if err := s.SyncBeads(ctx); err != nil {
		slog.Warn("bd sync failed after create", "error", err)
		// Don't fail the request, just log warning
	}

	return &BeadsIssueResponse{
		BeadsID:     beadsID,
		Title:       req.Title,
		Description: req.Description,
		Type:        req.Type,
		Priority:    req.Priority,
		Labels:      req.Labels,
		CreatedTs:   time.Now().Unix(),
	}, nil
}

// UpdateIssue updates a beads issue via bd update
func (s *BeadsService) UpdateIssue(ctx context.Context, beadsID string, status *string, priority *int) error {
	args := []string{"update", beadsID}

	if status != nil {
		args = append(args, "--status", *status)
	}

	if priority != nil {
		args = append(args, "--priority", fmt.Sprintf("%d", *priority))
	}

	slog.Info("Executing bd update", "beadsID", beadsID, "args", args)

	cmd := exec.CommandContext(ctx, "bd", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("bd update failed", "error", err, "output", string(output))
		return fmt.Errorf("bd update failed: %w", err)
	}

	slog.Info("bd update successful", "beadsID", beadsID)

	// Sync with git
	if err := s.SyncBeads(ctx); err != nil {
		slog.Warn("bd sync failed after update", "error", err)
	}

	return nil
}

// CloseIssue closes a beads issue via bd close
func (s *BeadsService) CloseIssue(ctx context.Context, beadsID string, reason string) error {
	args := []string{"close", beadsID}

	if reason != "" {
		args = append(args, "--reason", reason)
	}

	slog.Info("Executing bd close", "beadsID", beadsID, "reason", reason)

	cmd := exec.CommandContext(ctx, "bd", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("bd close failed", "error", err, "output", string(output))
		return fmt.Errorf("bd close failed: %w", err)
	}

	slog.Info("bd close successful", "beadsID", beadsID)

	// Sync with git
	if err := s.SyncBeads(ctx); err != nil {
		slog.Warn("bd sync failed after close", "error", err)
	}

	return nil
}

// SyncBeads syncs beads state with git via bd sync
func (s *BeadsService) SyncBeads(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "bd", "sync")
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("bd sync failed", "error", err, "output", string(output))
		return fmt.Errorf("bd sync failed: %w", err)
	}

	slog.Info("bd sync successful")
	return nil
}

// LogWorkflow logs an agent workflow event to the database
func (s *BeadsService) LogWorkflow(ctx context.Context, ticketID int32, sessionID, taskName, taskMode, taskStatus, taskSummary string, predictedSize int32, metadata map[string]interface{}) error {
	metadataJSON := "{}"
	if len(metadata) > 0 {
		bytes, err := json.Marshal(metadata)
		if err != nil {
			slog.Warn("Failed to marshal workflow metadata", "error", err)
		} else {
			metadataJSON = string(bytes)
		}
	}

	_, err := s.store.CreateAgentWorkflow(ctx, &store.CreateAgentWorkflow{
		TicketID:      ticketID,
		SessionID:     sessionID,
		AgentName:     "antigravity",
		TaskName:      taskName,
		TaskMode:      taskMode,
		TaskStatus:    taskStatus,
		TaskSummary:   taskSummary,
		PredictedSize: predictedSize,
		CreatedTs:     time.Now().Unix(),
		Metadata:      metadataJSON,
	})

	if err != nil {
		slog.Error("Failed to log workflow", "error", err, "ticketID", ticketID)
		return err
	}

	slog.Info("Logged agent workflow", "ticketID", ticketID, "taskName", taskName, "mode", taskMode)
	return nil
}

// parseBeadsIDFromOutput extracts beads_id from bd command output
// Expected formats:
// - "✓ Created issue: base-dfy" (new format)
// - "Created issue bd-a3f8e9" (legacy)
// - Returns: base-{hash} or bd-{hash}
func parseBeadsIDFromOutput(output string) (string, error) {
	// Try multiple patterns - both 'base-' and 'bd-' prefixes
	patterns := []string{
		`base-[a-z0-9]+`,  // New format: base-dfy, base-wyf
		`bd-[a-f0-9]+`,    // Legacy: bd-a3f8e9
		`bd-[a-zA-Z0-9]+`, // Legacy alphanumeric
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		match := re.FindString(output)
		if match != "" {
			return match, nil
		}
	}

	// Fallback: look for any "base-" or "bd-" followed by word characters
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "base-") || strings.Contains(line, "bd-") {
			// Extract the base-XXX or bd-XXX part
			words := strings.FieldsFunc(line, func(r rune) bool {
				return unicode.IsSpace(r) || r == ',' || r == '.' || r == ':'
			})
			for _, word := range words {
				if strings.HasPrefix(word, "base-") || strings.HasPrefix(word, "bd-") {
					return word, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not parse beads_id from output: %s", output)
}
