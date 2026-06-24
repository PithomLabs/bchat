package bridgeworker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

var (
	ErrInvalidWorkerConfig = errors.New("invalid worker config")
)

type OutboxStore interface {
	ClaimPendingBridgeReplyOutbox(
		ctx context.Context,
		tenantID int32,
		limit int,
		claimedBy string,
		now time.Time,
		claimDurationSeconds int64,
	) ([]*store.BridgeReplyOutbox, error)

	CompleteClaimedBridgeReplyOutbox(
		ctx context.Context,
		complete *store.CompleteBridgeReplyOutbox,
	) (*store.BridgeReplyOutbox, error)

	FailClaimedBridgeReplyOutbox(
		ctx context.Context,
		fail *store.FailBridgeReplyOutbox,
	) (*store.BridgeReplyOutbox, error)
}

type AdapterResult struct {
	Success        bool
	FailureCode    string
	FailureMessage string
}

type FakeDeliveryAdapter interface {
	Deliver(ctx context.Context, row *store.BridgeReplyOutbox) AdapterResult
}

type WorkerConfig struct {
	TenantID              int32
	ClaimLimit            int
	ClaimedBy             string
	ClaimDurationSeconds int64
	MaxRowsPerRun         int
}

type RunResult struct {
	ClaimedCount   int
	CompletedCount int
	FailedCount    int
	SkippedCount   int
	ErrorCount     int
	Errors         []string
}

type Worker struct {
	cfg   WorkerConfig
	store OutboxStore
	adapt FakeDeliveryAdapter
}

func (c WorkerConfig) Validate() error {
	if c.TenantID <= 0 {
		return fmt.Errorf("tenant ID must be greater than 0")
	}
	if c.ClaimLimit < 1 || c.ClaimLimit > 100 {
		return fmt.Errorf("claim limit must be between 1 and 100")
	}
	if len(c.ClaimedBy) < 1 || len(c.ClaimedBy) > 128 {
		return fmt.Errorf("claimed by length must be between 1 and 128")
	}
	for _, r := range c.ClaimedBy {
		if r < 32 || r > 126 {
			return fmt.Errorf("claimed by contains invalid characters")
		}
	}
	if c.ClaimDurationSeconds <= 0 {
		return fmt.Errorf("claim duration seconds must be greater than 0")
	}
	if c.MaxRowsPerRun < 0 {
		return fmt.Errorf("max rows per run must be greater than or equal to 0")
	}
	return nil
}

func NewWorker(cfg WorkerConfig, store OutboxStore, adapt FakeDeliveryAdapter) (*Worker, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidWorkerConfig, err)
	}
	if store == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}
	if adapt == nil {
		return nil, fmt.Errorf("adapter cannot be nil")
	}
	return &Worker{
		cfg:   cfg,
		store: store,
		adapt: adapt,
	}, nil
}

func (w *Worker) RunOnce(ctx context.Context, now time.Time) (*RunResult, error) {
	if now.Unix() <= 0 {
		return nil, fmt.Errorf("invalid now timestamp")
	}

	result := &RunResult{}

	if err := ctx.Err(); err != nil {
		return result, err
	}

	limit := w.cfg.ClaimLimit
	if w.cfg.MaxRowsPerRun > 0 && w.cfg.MaxRowsPerRun < limit {
		limit = w.cfg.MaxRowsPerRun
	}

	claimed, err := w.store.ClaimPendingBridgeReplyOutbox(ctx, w.cfg.TenantID, limit, w.cfg.ClaimedBy, now, w.cfg.ClaimDurationSeconds)
	if err != nil {
		result.ErrorCount++
		result.Errors = append(result.Errors, fmt.Sprintf("claim failed: %v", err))
		return result, err
	}

	result.ClaimedCount = len(claimed)

	for i, row := range claimed {
		if err := ctx.Err(); err != nil {
			result.SkippedCount += len(claimed) - i
			break
		}

		w.processRow(ctx, row, now, result)
	}

	return result, nil
}

func (w *Worker) processRow(ctx context.Context, row *store.BridgeReplyOutbox, now time.Time, result *RunResult) {
	var res AdapterResult
	panicMsg := ""
	panicked := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				panicMsg = sanitizePanicMessage(fmt.Sprintf("%v", r))
			}
		}()
		res = w.adapt.Deliver(ctx, row)
	}()

	if panicked {
		result.ErrorCount++
		_, err := w.store.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
			TenantID:       w.cfg.TenantID,
			OutboxID:       row.OutboxID,
			ClaimToken:     *row.ClaimToken,
			Now:            now.Unix(),
			FailureCode:    "adapter_panic",
			FailureMessage: panicMsg,
		})
		if err != nil {
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Sprintf("failed to settle panic for outbox %s: %s", row.OutboxID, redactError(err, row.ClaimToken)))
		} else {
			result.FailedCount++
		}
		return
	}

	if res.Success {
		_, err := w.store.CompleteClaimedBridgeReplyOutbox(ctx, &store.CompleteBridgeReplyOutbox{
			TenantID:   w.cfg.TenantID,
			OutboxID:   row.OutboxID,
			ClaimToken: *row.ClaimToken,
			Now:        now.Unix(),
		})
		if err != nil {
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Sprintf("failed to complete outbox %s: %s", row.OutboxID, redactError(err, row.ClaimToken)))
		} else {
			result.CompletedCount++
		}
	} else {
		failureCode := res.FailureCode
		failureMessage := res.FailureMessage
		valid := validateFailureMetadata(failureCode, failureMessage)
		if !valid {
			result.ErrorCount++
			failureCode = "adapter_invalid_result"
			failureMessage = "fake adapter returned invalid failure metadata"
		}

		_, err := w.store.FailClaimedBridgeReplyOutbox(ctx, &store.FailBridgeReplyOutbox{
			TenantID:       w.cfg.TenantID,
			OutboxID:       row.OutboxID,
			ClaimToken:     *row.ClaimToken,
			Now:            now.Unix(),
			FailureCode:    failureCode,
			FailureMessage: failureMessage,
		})
		if err != nil {
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Sprintf("failed to fail outbox %s: %s", row.OutboxID, redactError(err, row.ClaimToken)))
		} else {
			result.FailedCount++
		}
	}
}

func redactError(err error, tokenPtr *string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if tokenPtr != nil && *tokenPtr != "" {
		msg = strings.ReplaceAll(msg, *tokenPtr, "[REDACTED_CLAIM_TOKEN]")
	}
	return msg
}


func validateFailureMetadata(code, msg string) bool {
	if len(code) < 1 || len(code) > 64 {
		return false
	}
	for _, r := range code {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	if len(msg) < 1 || len(msg) > 1000 {
		return false
	}
	for _, r := range msg {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

func sanitizePanicMessage(msg string) string {
	var sb strings.Builder
	for _, r := range msg {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			sb.WriteRune(' ')
		} else {
			sb.WriteRune(r)
		}
	}
	result := sb.String()
	if len(result) > 1000 {
		result = result[:1000]
	}
	if len(result) == 0 {
		result = "adapter panicked with empty message"
	}
	return result
}

type StaticFakeAdapter struct {
	Result AdapterResult
}

func (a *StaticFakeAdapter) Deliver(ctx context.Context, row *store.BridgeReplyOutbox) AdapterResult {
	return a.Result
}

type ScriptedFakeAdapter struct {
	mu      sync.Mutex
	index   int
	Results []AdapterResult
}

func (a *ScriptedFakeAdapter) Deliver(ctx context.Context, row *store.BridgeReplyOutbox) AdapterResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.index >= len(a.Results) {
		return AdapterResult{
			Success:        false,
			FailureCode:    "no_more_results",
			FailureMessage: "ScriptedFakeAdapter ran out of results",
		}
	}
	res := a.Results[a.index]
	a.index++
	return res
}

type PanicFakeAdapter struct {
	Message string
}

func (a *PanicFakeAdapter) Deliver(ctx context.Context, row *store.BridgeReplyOutbox) AdapterResult {
	panic(a.Message)
}
