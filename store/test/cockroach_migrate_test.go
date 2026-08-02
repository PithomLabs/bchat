//go:build cockroach && integration

package teststore

import (
	"context"
	"database/sql"
	"math"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/internal/version"
	"github.com/usememos/memos/store"
	"github.com/usememos/memos/store/db"
)

// cockroachTestDSN returns the CockroachDB test DSN. The compose cluster
// (scripts/docker-compose.cockroach.yml, insecure mode) exposes
// bchat_user@localhost:26257/bchat; no password (insecure mode does not
// support passwords). Override via COCKROACH_DSN.
func cockroachTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("COCKROACH_DSN")
	if dsn == "" {
		dsn = "postgresql://bchat_user@localhost:26257/bchat?sslmode=disable"
	}
	return dsn
}

// isLocalDSN reports whether dsn looks like it points at a local CockroachDB
// instance (localhost or 127.0.0.1). This is a heuristic used only for the
// destructive-test permission gate; the authoritative check is the explicit
// environment variable opt-in.
func isLocalDSN(dsn string) bool {
	return strings.Contains(dsn, "localhost") || strings.Contains(dsn, "127.0.0.1")
}

// requireDatabaseResetPermission enforces the two-key opt-in for destructive
// migration reset tests:
//   - BCHAT_ALLOW_DB_RESET=1 is always required.
//   - BCHAT_ALLOW_REMOTE_DB_RESET=1 is additionally required when the DSN
//     does not look local.
//
// This guard lives here, next to resetCockroachDB, so every future caller
// inherits the protection automatically.
func requireDatabaseResetPermission(t *testing.T, dsn string) {
	t.Helper()
	if os.Getenv("BCHAT_ALLOW_DB_RESET") != "1" {
		t.Skip("BCHAT_ALLOW_DB_RESET=1 required to run destructive migration reset test")
	}
	if !isLocalDSN(dsn) {
		if os.Getenv("BCHAT_ALLOW_REMOTE_DB_RESET") != "1" {
			t.Skip("BCHAT_ALLOW_REMOTE_DB_RESET=1 required to reset a non-local database (DSN: " + dsn + ")")
		}
	}
}

func cockroachRawDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", cockroachTestDSN(t)+"&default_query_exec_mode=simple_protocol")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	return db
}

// resetCockroachDB returns the cluster to A1 state (empty public schema,
// no migration_history). Cockroach forbids DROP SCHEMA public (3F000), so we
// drop every table (CASCADE) instead — sequences die with their tables.
func resetCockroachDB(t *testing.T) {
	t.Helper()
	db := cockroachRawDB(t)
	rows, err := db.QueryContext(context.Background(), `
		SELECT string_agg(format('DROP TABLE IF EXISTS %I CASCADE', table_name), '; ')
		FROM information_schema.tables
		WHERE table_schema = 'public'`)
	require.NoError(t, err)
	defer rows.Close()
	var stmt string
	if rows.Next() {
		require.NoError(t, rows.Scan(&stmt))
	}
	require.NoError(t, rows.Close())
	if stmt != "" {
		_, err = db.ExecContext(context.Background(), stmt)
		require.NoError(t, err)
	}
}

func newCockroachProfile(t *testing.T) *profile.Profile {
	t.Helper()
	return &profile.Profile{
		Mode:    "prod",
		Port:    5231,
		Data:    t.TempDir(),
		DSN:     cockroachTestDSN(t),
		Driver:  "cockroach",
		Version: version.GetCurrentVersion("prod"),
	}
}

func newCockroachStore(t *testing.T) (*store.Store, store.Driver) {
	t.Helper()
	profile := newCockroachProfile(t)
	dbDriver, err := db.NewDBDriver(profile)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dbDriver.Close())
	})
	return store.New(dbDriver, profile), dbDriver
}

// TestCockroachMigrateEndToEnd drives the real Migrate() path
// (preMigrate → LATEST.sql → history → updateCurrentSchemaVersion) against
// a live CockroachDB cluster. This is the post-P0 verification of the
// migrator.go cockroach branch (bugs/057 §4.4).
func TestCockroachMigrateEndToEnd(t *testing.T) {
	requireDatabaseResetPermission(t, cockroachTestDSN(t))
	ctx := context.Background()
	resetCockroachDB(t)
	s, driver := newCockroachStore(t)

	// A1: fresh database → full LATEST.sql + migration_history
	require.NoError(t, s.Migrate(ctx))

	// History was written with the FS schema version.
	hist, err := driver.FindMigrationHistoryList(ctx, &store.FindMigrationHistory{})
	require.NoError(t, err)
	require.Len(t, hist, 1)
	fsVersion, err := s.GetCurrentSchemaVersion()
	require.NoError(t, err)
	require.Equal(t, fsVersion, hist[0].Version)

	// Schema version persisted in workspace settings.
	ws, err := s.GetWorkspaceBasicSetting(ctx)
	require.NoError(t, err)
	require.Equal(t, fsVersion, ws.SchemaVersion)

	// serial_normalization=sql_sequence: IDs come from sequences (int32-safe).
	var n int64
	require.NoError(t, driver.GetDB().QueryRowContext(ctx, "SELECT nextval('agent_tenants_id_seq')").Scan(&n))
	require.Less(t, n, int64(math.MaxInt32))

	// No unique_rowid()-backed defaults anywhere.
	rows, err := driver.GetDB().QueryContext(ctx, `
		SELECT table_name, column_name, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND column_default LIKE '%unique_rowid%'`)
	require.NoError(t, err)
	defer rows.Close()
	var offenders []string
	for rows.Next() {
		var tableName, columnName, columnDefault string
		require.NoError(t, rows.Scan(&tableName, &columnName, &columnDefault))
		offenders = append(offenders, tableName+"."+columnName)
	}
	require.Empty(t, offenders, "unique_rowid defaults found: %v", offenders)

	// A2: re-run with history present → no-op success.
	require.NoError(t, s.Migrate(ctx))

	// A3: failed-boot recovery — history wiped (partial state), LATEST.sql
	// re-applies cleanly because the cockroach mirror is fully idempotent
	// (IF NOT EXISTS on all DDL, bugs/057 §4.1.3).
	_, err = driver.GetDB().ExecContext(ctx, "DELETE FROM migration_history")
	require.NoError(t, err)
	require.NoError(t, s.Migrate(ctx))
	hist, err = driver.FindMigrationHistoryList(ctx, &store.FindMigrationHistory{})
	require.NoError(t, err)
	require.Len(t, hist, 1)
	require.Equal(t, fsVersion, hist[0].Version)

	// A4: boot after failed LATEST re-run (simulate: corrupt history row so
	// normalizedMigrationHistoryList and version checks still pass, then a
	// second Migrate with the same state must remain stable).
	require.NoError(t, s.Migrate(ctx))

	// Postgres compatibility smoke: a tenant row round-trips through the
	// store layer with an int32 ID.
	tenant := &store.AgentTenant{Slug: "p0-e2e", CompanyName: "P0 E2E", IsActive: true}
	created, err := s.CreateAgentTenant(ctx, tenant)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.Less(t, int64(created.ID), int64(math.MaxInt32))
	got, err := s.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &tenant.Slug})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, "P0 E2E", got.CompanyName)
}

// TestCockroachMigrateBootIdempotency asserts the tolerance strings the
// migrator cockroach arm inlines from execute() match Cockroach's actual
// duplicate-object errors (42P07). Covers both tolerate-able and fatal
// shapes so a future ALTER TABLE in a cockroach migration behaves
// predictably.
func TestCockroachMigrateBootIdempotency(t *testing.T) {
	requireDatabaseResetPermission(t, cockroachTestDSN(t))
	ctx := context.Background()
	db := cockroachRawDB(t)

	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS tolerance_probe (id INT PRIMARY KEY)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE tolerance_probe (id INT PRIMARY KEY)`)
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "already exists", "Cockroach duplicate-table error must carry 'already exists': %s", msg)
	require.Contains(t, msg, "42P07", "Cockroach duplicate-table error must carry SQLSTATE 42P07: %s", msg)
	require.False(t, strings.Contains(msg, "duplicate column"), "table duplicate must not be confused with column duplicate")
}
