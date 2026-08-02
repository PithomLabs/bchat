//go:build cockroach && integration

package store

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestCockroachP0 is the P0 gate from bugs/057/pre_code.md §4.0.1: it proves
// that the exact execute() path (one ExecContext carrying the whole
// multi-statement file over pgx simple protocol) accepts
//
//	SET serial_normalization = 'sql_sequence';
//
// followed by the entire cockroach LATEST.sql, that the schema lands with
// nextval() defaults (int32-safe, no unique_rowid()), and that a re-run is
// idempotent. Blocking gate: must pass before store/migrator.go's cockroach
// branch is written.
//
// Run against the local compose cluster:
//
//	docker compose -f scripts/docker-compose.cockroach.yml up -d
//	go test -tags "cockroach integration" ./store/ -run TestCockroachP0 -v
func TestCockroachP0(t *testing.T) {
	dsn := os.Getenv("COCKROACH_DSN")
	if dsn == "" {
		dsn = "postgresql://bchat_user@localhost:26257/bchat?sslmode=disable"
	}
	if !strings.Contains(dsn, "default_query_exec_mode") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "default_query_exec_mode=simple_protocol"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping (is the compose cluster up?): %v", err)
	}

	content, err := MigrationFS.ReadFile("migration/cockroach/LATEST.sql")
	if err != nil {
		t.Fatalf("read cockroach LATEST.sql: %v", err)
	}

	stmt := "SET serial_normalization = 'sql_sequence';\n" + string(content)

	// (a) SET + whole-file multi-statement exec through the exact execute() shape.
	if err := execWholeFile(ctx, db, stmt); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// (b) nextval() defaults on serial tables, zero unique_rowid() anywhere.
	for _, table := range []string{"agent_tenants", "memo", "tickets"} {
		var tableName, createSQL string
		if err := db.QueryRowContext(ctx, "SHOW CREATE TABLE "+table).Scan(&tableName, &createSQL); err != nil {
			t.Fatalf("SHOW CREATE TABLE %s: %v", table, err)
		}
		if !strings.Contains(createSQL, "nextval(") {
			t.Errorf("%s: no nextval() default found (serial_normalization not applied)", table)
		}
	}
	// Comprehensive scan: any column default using unique_rowid() (e.g. a
	// synthesized rowid on a PRIMARY KEY-less table) breaks the int32 ID
	// invariant and fails the P0 gate.
	rows, err := db.QueryContext(ctx, `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND column_default LIKE '%unique_rowid%'`)
	if err != nil {
		t.Fatalf("unique_rowid scan: %v", err)
	}
	defer rows.Close()
	var offenders []string
	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			t.Fatalf("scan: %v", err)
		}
		offenders = append(offenders, tableName+"."+columnName)
	}
	if len(offenders) > 0 {
		t.Errorf("unique_rowid() defaults found: %v — PRIMARY KEY-less tables", offenders)
	}

	// (c) idempotent re-run: whole file again must not error (IF NOT EXISTS + ON CONFLICT).
	if err := execWholeFile(ctx, db, stmt); err != nil {
		t.Fatalf("re-run failed — not idempotent: %v", err)
	}

	// (d) execute() tolerance: Cockroach's duplicate-table error must contain
	// "already exists" so migrator's tolerance (migrator.go:326-328) swallows it.
	_, err = db.ExecContext(ctx, "CREATE TABLE migration_history (version STRING PRIMARY KEY)")
	if err == nil {
		t.Fatal("expected duplicate-table error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate-table error does not match execute() tolerance: %v", err)
	}
}

func execWholeFile(ctx context.Context, db *sql.DB, stmt string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return err
	}
	return tx.Commit()
}
