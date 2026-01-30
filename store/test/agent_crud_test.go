package teststore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

// TestAgentSourceFileCRUD verifies that basic CRUD operations work on the
// agent_source_files table after migrations. This is a smoke test to catch
// schema/code mismatches early.
func TestAgentSourceFileCRUD(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// First, create a tenant (required foreign key)
	tenant := &store.AgentTenant{
		Slug:        "test-tenant",
		CompanyName: "Test Company",
		Vertical:    "testing",
		IsActive:    true,
	}
	tenant, err := ts.CreateAgentTenant(ctx, tenant)
	require.NoError(t, err, "Should be able to create tenant")
	require.NotZero(t, tenant.ID, "Tenant should have ID")

	// Create a source file (KB)
	kbFile := &store.AgentSourceFile{
		TenantID:     tenant.ID,
		AudienceType: "external",
		FileType:     "kb",
		Content:      "# Test KB\n\nThis is test content.",
		ContentHash:  "abc123",
	}
	kbFile, err = ts.UpsertAgentSourceFile(ctx, kbFile)
	require.NoError(t, err, "Should be able to create KB source file")
	require.NotZero(t, kbFile.ID, "Source file should have ID")
	require.Equal(t, int32(1), kbFile.Version, "First version should be 1")

	// Read the source file back
	foundFile, err := ts.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
		TenantID:     &tenant.ID,
		AudienceType: strPtr("external"),
		FileType:     strPtr("kb"),
	})
	require.NoError(t, err, "Should be able to find source file")
	require.Equal(t, kbFile.Content, foundFile.Content, "Content should match")
	require.Equal(t, int32(1), foundFile.Version, "Version should be 1")

	// Update the source file (creates new version)
	kbFile.Content = "# Updated KB\n\nNew content."
	kbFile.ContentHash = "def456"
	kbFile, err = ts.UpsertAgentSourceFile(ctx, kbFile)
	require.NoError(t, err, "Should be able to update KB source file")
	require.Equal(t, int32(2), kbFile.Version, "Second version should be 2")

	// List all source files for tenant
	files, err := ts.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
		TenantID: &tenant.ID,
	})
	require.NoError(t, err, "Should be able to list source files")
	require.GreaterOrEqual(t, len(files), 1, "Should have at least one file")

	// Delete source files
	err = ts.DeleteAgentSourceFiles(ctx, tenant.ID, nil)
	require.NoError(t, err, "Should be able to delete source files")

	// Verify deletion
	files, err = ts.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
		TenantID: &tenant.ID,
	})
	require.NoError(t, err)
	require.Empty(t, files, "Should have no files after deletion")
}

// TestAgentTenantScriptCRUD verifies script CRUD operations
func TestAgentTenantScriptCRUD(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// Create a tenant
	tenant := &store.AgentTenant{
		Slug:        "script-test-tenant",
		CompanyName: "Script Test Company",
		IsActive:    true,
	}
	tenant, err := ts.CreateAgentTenant(ctx, tenant)
	require.NoError(t, err)

	// Create a script
	script := &store.AgentTenantScript{
		TenantID:    tenant.ID,
		Content:     "# Greeting\n\nSay hello to the user.",
		ContentHash: "script123",
		Summary:     "Test script for greeting users",
	}
	script, err = ts.UpsertAgentTenantScript(ctx, script)
	require.NoError(t, err, "Should be able to create script")
	require.NotZero(t, script.ID)
	require.Equal(t, 1, script.Version, "First version should be 1")

	// Read script back
	foundScript, err := ts.GetAgentTenantScript(ctx, &store.FindAgentTenantScript{
		TenantID: &tenant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, script.Content, foundScript.Content)
	require.Equal(t, 1, foundScript.Version)

	// Update script
	script.Content = "# Updated Greeting\n\nSay hello nicely."
	script.ContentHash = "script456"
	script, err = ts.UpsertAgentTenantScript(ctx, script)
	require.NoError(t, err)
	require.Equal(t, 2, script.Version, "Second version should be 2")
}

// Helper function to get string pointer
func strPtr(s string) *string {
	return &s
}
