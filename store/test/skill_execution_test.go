package teststore

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func TestSkillExecutionRoundTrip(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "skill-exec-rt")

	graphJSON := `{"nodes":{"search_kb":{"name":"search_kb","handler":"search_kb"}}}`

	// Case 1: Create with nil TenantID (proves R-6 necessity on postgres later)
	nilTenantID := uuid.New().String()
	nilTenantExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             nilTenantID,
		TenantID:       nil,
		ConversationID: "conv-nil",
		SkillGraphJSON: graphJSON,
		Status:         "pending",
		TriggerPath:    "chat",
		MaxRetries:     3,
	})
	require.NoError(t, err)
	require.NotEmpty(t, nilTenantExec.ID)

	fetched, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &nilTenantExec.ID})
	require.NoError(t, err)
	require.Nil(t, fetched.TenantID)
	require.Equal(t, "pending", fetched.Status)

	// Case 2: Tenant-filter scoping
	otherTenant := createBridgeTenant(t, ctx, ts, "skill-exec-other")
	_, err = ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       &otherTenant.ID,
		ConversationID: "conv-other",
		SkillGraphJSON: graphJSON,
		Status:         "pending",
		TriggerPath:    "api",
		MaxRetries:     3,
	})
	require.NoError(t, err)

	tenantExecID := uuid.New().String()
	tenantExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             tenantExecID,
		TenantID:       &tenant.ID,
		ConversationID: "conv-scoped",
		SkillGraphJSON: graphJSON,
		Status:         "pending",
		TriggerPath:    "event",
		MaxRetries:     3,
	})
	require.NoError(t, err)

	execs, err := ts.ListSkillExecutions(ctx, &store.FindSkillExecution{TenantID: &tenant.ID}, 10)
	require.NoError(t, err)
	require.Len(t, execs, 1)
	require.Equal(t, tenantExec.ID, execs[0].ID)

	// Case 3: Claim lifecycle
	claimed, err := ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-1", 60)
	require.NoError(t, err)
	require.Equal(t, "running", claimed.Status)
	require.NotNil(t, claimed.ClaimedAt)
	require.NotNil(t, claimed.ClaimedBy)
	require.Equal(t, "worker-1", *claimed.ClaimedBy)

	// Re-claim by same worker (lease re-entry) — should succeed
	_, err = ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-1", 60)
	require.NoError(t, err)

	// T2: Different-worker exclusivity — unexpired lease must block worker-2
	_, err = ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-2", 60)
	require.Error(t, err)

	// Case 4: Stop
	err = ts.StopSkillExecution(ctx, tenantExec.ID)
	require.NoError(t, err)

	stopped, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &tenantExec.ID})
	require.NoError(t, err)
	require.Equal(t, "stopped", stopped.Status)

	// Claim on stopped row should fail (only pending/running are claimable)
	_, err = ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-2", 60)
	require.Error(t, err)

	// Case 5: Fail persists K-1 error message
	failExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       &tenant.ID,
		ConversationID: "conv-fail",
		SkillGraphJSON: graphJSON,
		Status:         "running",
		TriggerPath:    "chat",
		MaxRetries:     3,
	})
	require.NoError(t, err)

	err = ts.FailSkillExecution(ctx, failExec.ID, "boom")
	require.NoError(t, err)

	failed, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &failExec.ID})
	require.NoError(t, err)
	require.Equal(t, "failed", failed.Status)
	require.Equal(t, "boom", failed.ErrorMessage)

	// Case 6: Complete
	completeExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       &tenant.ID,
		ConversationID: "conv-complete",
		SkillGraphJSON: graphJSON,
		Status:         "running",
		TriggerPath:    "cron",
		MaxRetries:     3,
	})
	require.NoError(t, err)

	err = ts.CompleteSkillExecution(ctx, completeExec.ID)
	require.NoError(t, err)

	completed, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &completeExec.ID})
	require.NoError(t, err)
	require.Equal(t, "completed", completed.Status)
	require.NotNil(t, completed.CompletedAt)

	// Case 7: Log round-trip (nil tenant)
	logExecID := uuid.New().String()
	logExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             logExecID,
		TenantID:       nil,
		ConversationID: "conv-log",
		SkillGraphJSON: graphJSON,
		Status:         "completed",
		TriggerPath:    "chat",
		MaxRetries:     3,
	})
	require.NoError(t, err)

	err = ts.CreateSkillLog(ctx, &store.SkillLog{
		ID:          uuid.New().String(),
		TenantID:    nil,
		ExecutionID: logExec.ID,
		SkillName:   "search_kb",
		Handler:     "search_kb",
		Status:      "completed",
		DurationMs:  150,
		StartedAt:   time.Now(),
	})
	require.NoError(t, err)

	logs, err := ts.ListSkillLogs(ctx, &store.FindSkillLog{ExecutionID: &logExec.ID})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "search_kb", logs[0].SkillName)
	require.False(t, logs[0].StartedAt.IsZero())

	// Case 8: Stop on pending (unclaimed) row transitions
	pendingExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       &tenant.ID,
		ConversationID: "conv-pending-stop",
		SkillGraphJSON: graphJSON,
		Status:         "pending",
		TriggerPath:    "chat",
		MaxRetries:     3,
	})
	require.NoError(t, err)

	err = ts.StopSkillExecution(ctx, pendingExec.ID)
	require.NoError(t, err)

	stoppedPending, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &pendingExec.ID})
	require.NoError(t, err)
	require.Equal(t, "stopped", stoppedPending.Status)
}
