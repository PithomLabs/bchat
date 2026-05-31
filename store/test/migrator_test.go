package teststore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetCurrentSchemaVersion(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	currentSchemaVersion, err := ts.GetCurrentSchemaVersion()
	require.NoError(t, err)
	// Schema version should start with the minor version (0.25.x)
	// Using Contains to avoid updating test on every patch version bump
	require.Contains(t, currentSchemaVersion, "0.25.", "schema version should be 0.25.x")
}
