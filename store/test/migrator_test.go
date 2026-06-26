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
	// Schema version should start with the current minor version (0.26.x).
	// Using Contains to avoid updating test on every patch version bump
	require.Contains(t, currentSchemaVersion, "0.26.", "schema version should be 0.26.x")
}
