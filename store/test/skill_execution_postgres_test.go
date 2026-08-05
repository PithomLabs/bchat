package teststore

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func TestSkillExecutionRoundTrip_Postgres(t *testing.T) {
	driver := getDriverFromEnv()
	if driver != "postgres" {
		t.Skip("Skipping Postgres skill round-trip; set DRIVER=postgres (and DSN) to run")
	}

	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "skill-exec-pg")

	graphJSON := `{"nodes":{"search_kb":{"name":"search_kb","handler":"search_kb"}}}`

	// Case 1: nil-TenantID insert (fails red before Fix 6 NOT NULL constraint)
	nilTenantExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       nil,
		ConversationID: "conv-nil-pg",
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
	otherTenant := createBridgeTenant(t, ctx, ts, "skill-exec-pg-other")
	_, err = ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       &otherTenant.ID,
		ConversationID: "conv-other-pg",
		SkillGraphJSON: graphJSON,
		Status:         "pending",
		TriggerPath:    "api",
		MaxRetries:     3,
	})
	require.NoError(t, err)

	tenantExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       &tenant.ID,
		ConversationID: "conv-scoped-pg",
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
	claimed, err := ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-pg", 60)
	require.NoError(t, err)
	require.Equal(t, "running", claimed.Status)
	require.NotNil(t, claimed.ClaimedAt)
	require.NotNil(t, claimed.ClaimedBy)

	// Re-claim by same worker (lease re-entry)
	_, err = ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-pg", 60)
	require.NoError(t, err)

	// Case 4: Stop
	err = ts.StopSkillExecution(ctx, tenantExec.ID)
	require.NoError(t, err)

	stopped, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &tenantExec.ID})
	require.NoError(t, err)
	require.Equal(t, "stopped", stopped.Status)

	// Case 5: Fail persists error message
	failExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       &tenant.ID,
		ConversationID: "conv-fail-pg",
		SkillGraphJSON: graphJSON,
		Status:         "running",
		TriggerPath:    "chat",
		MaxRetries:     3,
	})
	require.NoError(t, err)

	err = ts.FailSkillExecution(ctx, failExec.ID, "pg-boom")
	require.NoError(t, err)

	failed, err := ts.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &failExec.ID})
	require.NoError(t, err)
	require.Equal(t, "failed", failed.Status)
	require.Equal(t, "pg-boom", failed.ErrorMessage)

	// Case 6: Complete
	completeExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       &tenant.ID,
		ConversationID: "conv-complete-pg",
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

	// TIMESTAMPTZ round-trip assertion
	require.False(t, completed.CreatedAt.IsZero(), "CreatedAt should not be zero")
	require.False(t, completed.CompletedAt.IsZero(), "CompletedAt should not be zero")

	// Case 7: Log round-trip (nil tenant)
	logExec, err := ts.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             uuid.New().String(),
		TenantID:       nil,
		ConversationID: "conv-log-pg",
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
		DurationMs:  200,
		StartedAt:   time.Now(),
	})
	require.NoError(t, err)

	logs, err := ts.ListSkillLogs(ctx, &store.FindSkillLog{ExecutionID: &logExec.ID})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "search_kb", logs[0].SkillName)
	require.False(t, logs[0].StartedAt.IsZero())
}
