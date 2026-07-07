package teststore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestPostgresBridgeFKCascade verifies that bridge table FK cascades work on Postgres.
// Requires: DRIVER=postgres and a running PostgreSQL instance with the configured DSN.
// This test closes the coverage gap from SQLite skip guards by verifying the same
// FK behavior using Postgres-compatible SQL (no sqlite_master or PRAGMA).
func TestPostgresBridgeFKCascade(t *testing.T) {
	driver := getDriverFromEnv()
	if driver != "postgres" {
		t.Skip("Skipping Postgres bridge cascade test; set DRIVER=postgres to run")
	}

	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	// Create a tenant
	tenant := createBridgeTenant(t, ctx, ts, "bridge-fk-cascade")
	require.NotNil(t, tenant)

	now := time.Now()
	expiresAt := now.Add(1 * time.Hour)

	// Create a bridge external session
	session, _, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "fk-cascade-session", now, expiresAt)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Verify the session exists
	fetched, err := ts.FindBridgeExternalSession(ctx, tenant.ID, "fk-cascade-session")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, tenant.ID, fetched.TenantID)

	// Delete the tenant — FK CASCADE should remove the session
	err = ts.DeleteAgentTenant(ctx, tenant.ID)
	require.NoError(t, err)

	// Verify the session was cascade-deleted
	deleted, err := ts.FindBridgeExternalSession(ctx, tenant.ID, "fk-cascade-session")
	require.NoError(t, err)
	require.Nil(t, deleted, "Session should have been cascade-deleted with tenant")
}
