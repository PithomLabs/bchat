package teststore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

// TestSchemaValidation verifies that all migrations apply cleanly and produce
// a schema that matches what the Go code expects. This test helps catch issues
// like migration 12's missing version column bug before deployment.
func TestSchemaValidation(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	db := ts.GetDriver().GetDB()

	// Define critical tables and their required columns
	// These must match what the Go code in store/db/sqlite/agent.go expects
	criticalTables := map[string][]string{
		// Source files table - requires version column for file versioning
		"agent_source_files": {
			"id", "tenant_id", "audience_type", "file_type",
			"content", "content_hash", "version", "imported_at",
		},

		// Tenant scripts table - SCRIPT.MD storage
		"agent_tenant_scripts": {
			"id", "tenant_id", "content", "content_hash",
			"summary", "imported_at", "version",
		},

		// Core tenant table
		"agent_tenants": {
			"id", "slug", "company_name", "vertical",
			"is_active", "created_at", "updated_at",
		},

		// Audiences table
		"agent_audiences": {
			"id", "tenant_id", "audience_type", "role", "tone",
			"emergency_phone", "updated_at",
		},

		// Sessions table
		"agent_sessions": {
			"id", "tenant_id", "user_id", "audience_type",
			"phase", "message_count", "messages", "created_at",
		},

		// Analysis results table
		"agent_analysis_results": {
			"id", "tenant_id", "conversation_id", "conversation_type",
			"user_id", "score", "grade", "breakdown", "issues", "created_at",
		},
	}

	for tableName, requiredColumns := range criticalTables {
		t.Run("Table_"+tableName, func(t *testing.T) {
			err := store.ValidateTableSchema(ctx, db, tableName, requiredColumns)
			require.NoError(t, err, "Schema validation failed for table %s", tableName)
		})
	}
}

// TestAgentSourceFilesVersionColumn specifically tests that the version column
// exists and works correctly in agent_source_files table. This is a regression
// test for the bug where migration 12 created an index on 'version' without
// adding the column first.
func TestAgentSourceFilesVersionColumn(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	db := ts.GetDriver().GetDB()

	// Verify column exists
	columns, err := store.GetTableColumns(ctx, db, "agent_source_files")
	require.NoError(t, err)

	hasVersion := false
	for _, col := range columns {
		if col == "version" {
			hasVersion = true
			break
		}
	}
	require.True(t, hasVersion, "agent_source_files table must have 'version' column")

	// Test that we can actually use the version column in SQL operations
	// This catches issues where the column exists but has wrong type/constraints
	_, err = db.ExecContext(ctx, `
		SELECT MAX(version) FROM agent_source_files
		WHERE tenant_id = 1 AND audience_type = 'external' AND file_type = 'kb'
	`)
	require.NoError(t, err, "Should be able to query version column")
}

// TestMigrationHistoryVersion verifies the migration history table tracks versions
func TestMigrationHistoryVersion(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// Get current schema version after migrations
	currentVersion, err := ts.GetCurrentSchemaVersion()
	require.NoError(t, err)
	require.NotEmpty(t, currentVersion, "Should have a current schema version")

	// Schema version should be at least 0.25.x (current development version)
	t.Logf("Current schema version: %s", currentVersion)
}
