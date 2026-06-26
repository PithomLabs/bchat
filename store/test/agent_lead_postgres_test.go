package teststore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

// TestAgentLeadPostgresParity runs the same lead lifecycle tests against PostgreSQL.
// Requires: DRIVER=postgres and a running PostgreSQL instance with the configured DSN.
func TestAgentLeadPostgresParity(t *testing.T) {
	driver := getDriverFromEnv()
	if driver != "postgres" {
		t.Skip("Skipping Postgres parity test; set DRIVER=postgres to run")
	}

	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "lead-pg-lifecycle")

	lead, err := ts.UpsertAgentLead(ctx, &store.AgentLead{
		TenantID:      tenant.ID,
		SessionID:     "pg-session-1",
		Name:          "Alan Turing",
		Email:         "alan@example.org",
		Topic:         "Postgres parity test",
		Status:        "new",
		LastMessageAt: time.Now(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, lead.ID)
	require.Equal(t, tenant.ID, lead.TenantID)

	fetched, err := ts.GetAgentLead(ctx, &store.FindAgentLead{ID: &lead.ID, TenantID: &tenant.ID})
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, lead.ID, fetched.ID)

	updated, err := ts.UpdateAgentLeadStatus(ctx, tenant.ID, lead.ID, "contacted", nil)
	require.NoError(t, err)
	require.Equal(t, "contacted", updated.Status)

	convertedAt := time.Now()
	converted, err := ts.UpdateAgentLeadStatus(ctx, tenant.ID, lead.ID, "converted", &convertedAt)
	require.NoError(t, err)
	require.Equal(t, "converted", converted.Status)
	require.NotNil(t, converted.ConvertedAt)
}
