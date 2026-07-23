package teststore

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/internal/version"
	"github.com/usememos/memos/store"
)

func TestGetCurrentSchemaVersion(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	currentSchemaVersion, err := ts.GetCurrentSchemaVersion()
	require.NoError(t, err)
	require.NotEmpty(t, currentSchemaVersion)

	// The schema version must be a valid semver (X.Y.Z)
	parts := strings.Split(currentSchemaVersion, ".")
	require.Len(t, parts, 3, "schema version must have 3 parts: %s", currentSchemaVersion)
	for _, p := range parts {
		_, err := strconv.Atoi(p)
		require.NoError(t, err, "schema version part must be numeric: %s", currentSchemaVersion)
	}

	// The schema version must be >= the latest migration directory
	// This directly tests the guard condition that caused bugs 028, 045, 046
	migrationDirs := getMigrationDirs(t)
	if len(migrationDirs) > 0 {
		latestDir := migrationDirs[len(migrationDirs)-1]
		require.True(t,
			version.IsVersionGreaterOrEqualThan(currentSchemaVersion, latestDir+".0"),
			"schema version %s must be >= latest migration directory %s",
			currentSchemaVersion, latestDir)
	}
}

func TestAllMigrationFilesCoveredBySchemaVersion(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	schemaVersion, err := ts.GetCurrentSchemaVersion()
	require.NoError(t, err)

	// For EVERY migration file in the glob, its version must be <= schemaVersion
	// This directly tests the guard condition at migrator.go:96
	filePaths, err := fs.Glob(store.MigrationFS, fmt.Sprintf("migration/sqlite/*/*.sql"))
	require.NoError(t, err)

	for _, filePath := range filePaths {
		// Skip LATEST.sql — it's at the base path, not in a subdirectory,
		// so the */*.sql glob won't match it. But guard against future changes.
		if strings.HasSuffix(filePath, "LATEST.sql") {
			continue
		}

		// Parse version from path: migration/sqlite/0.33/00__fix.sql → "0.33.1"
		pathParts := strings.Split(strings.ReplaceAll(filePath, "\\", "/"), "/")
		require.True(t, len(pathParts) >= 3, "migration path too short: %s", filePath)
		minorVersion := pathParts[len(pathParts)-2] // "0.33"
		patchStr := strings.Split(pathParts[len(pathParts)-1], "__")[0] // "00"
		patchNum, err := strconv.Atoi(patchStr)
		require.NoError(t, err, "invalid patch number in %s", filePath)
		fileVersion := fmt.Sprintf("%s.%d", minorVersion, patchNum+1)

		require.True(t,
			version.IsVersionGreaterOrEqualThan(schemaVersion, fileVersion),
			"schema version %s must be >= migration file version %s (from %s)",
			schemaVersion, fileVersion, filePath)
	}
}

func getMigrationDirs(t *testing.T) []string {
	t.Helper()
	entries, err := fs.ReadDir(store.MigrationFS, fmt.Sprintf("migration/%s", "sqlite"))
	require.NoError(t, err)
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && regexp.MustCompile(`^[0-9]+\.[0-9]+$`).MatchString(e.Name()) {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

func TestMigrationLoopSkipsAlreadyApplied(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t) // DB is at current FS version

	// Simulate a database that was at 0.31.3 before Plan 5's GetCurrentSchemaVersion fix
	// surfaced the dormant skip-logic bug.
	_, err := ts.GetDriver().GetDB().ExecContext(ctx,
		"DELETE FROM migration_history; INSERT INTO migration_history (version) VALUES ('0.31.3')")
	require.NoError(t, err)

	// Re-run migration. This previously crashed on 0.14/00 because the skip logic
	// used exact-version-match, which missed all file versions not individually
	// recorded in migration_history.
	err = ts.Migrate(ctx)
	require.NoError(t, err)

	// Verify 0.32.x and 0.33.x were applied by checking that the latest history
	// entry is the FS max version.
	rows, err := ts.GetDriver().GetDB().QueryContext(ctx, "SELECT version FROM migration_history ORDER BY created_ts DESC")
	require.NoError(t, err)
	var versions []string
	for rows.Next() {
		var v string
		require.NoError(t, rows.Scan(&v))
		versions = append(versions, v)
	}
	require.NoError(t, rows.Err())
	schemaVersion, err := ts.GetCurrentSchemaVersion()
	require.NoError(t, err)
	require.Contains(t, versions, schemaVersion, "latest migrations should have been applied")
}

func TestNonIdempotentMigrationsNeverRerun(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// Simulate a database at 0.31.3 (the exact state that triggered taskrunrag_fail.md).
	_, err := ts.GetDriver().GetDB().ExecContext(ctx,
		"DELETE FROM migration_history; INSERT INTO migration_history (version) VALUES ('0.31.3')")
	require.NoError(t, err)

	// Re-run migration. Before Plan 6, this crashed with:
	//   "no such column: external_link" (from 0.14/00)
	// After the range-check fix, 0.14/00 is skipped because 0.14.1 <= 0.31.3.
	err = ts.Migrate(ctx)
	require.NoError(t, err)
}
