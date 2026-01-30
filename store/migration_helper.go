package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// AddColumnIfNotExists adds a column to a table if it doesn't already exist
// This is a helper for SQLite migrations since ALTER TABLE ADD COLUMN IF NOT EXISTS is not supported
func AddColumnIfNotExists(ctx context.Context, db *sql.DB, tableName, columnName, columnDef string) error {
	// Check if column exists
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query table info: %w", err)
	}
	defer rows.Close()

	exists := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}
		if name == columnName {
			exists = true
			break
		}
	}

	if exists {
		slog.Info("Column already exists, skipping", "table", tableName, "column", columnName)
		return nil
	}

	// Add column
	alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef)
	slog.Info("Adding column", "table", tableName, "column", columnName, "sql", alterSQL)
	_, err = db.ExecContext(ctx, alterSQL)
	if err != nil {
		return fmt.Errorf("failed to add column: %w", err)
	}

	slog.Info("Column added successfully", "table", tableName, "column", columnName)
	return nil
}

// EnsureTicketBeadsColumns ensures all beads-related columns exist in tickets table
func (s *Store) EnsureTicketBeadsColumns(ctx context.Context) error {
	db := s.driver.GetDB()

	columns := []struct {
		name string
		def  string
	}{
		{"beads_id", "TEXT"},
		{"parent_id", "INTEGER REFERENCES tickets(id)"},
		{"labels", "TEXT DEFAULT '[]'"},
		{"dependencies", "TEXT DEFAULT '[]'"},
		{"discovery_context", "TEXT"},
		{"closed_reason", "TEXT"},
		{"issue_type", "TEXT"},
	}

	for _, col := range columns {
		if err := AddColumnIfNotExists(ctx, db, "tickets", col.name, col.def); err != nil {
			return fmt.Errorf("failed to add column %s: %w", col.name, err)
		}
	}

	// Create unique index on beads_id after column is added
	_, err := db.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_tickets_beads_id ON tickets(beads_id)")
	if err != nil {
		slog.Warn("Failed to create unique index on beads_id, it may already exist", "error", err)
	}

	return nil
}

// EnsureTicketTypeColumn ensures the type and tags columns exist in tickets table
// This is called during migration to fix databases that may have been created before these columns were added
func EnsureTicketTypeColumn(ctx context.Context, db *sql.DB) error {
	columns := []struct {
		name string
		def  string
	}{
		{"type", "TEXT NOT NULL DEFAULT 'TASK'"},
		{"tags", "TEXT NOT NULL DEFAULT '[]'"},
	}

	for _, col := range columns {
		if err := AddColumnIfNotExists(ctx, db, "tickets", col.name, col.def); err != nil {
			return fmt.Errorf("failed to add column %s: %w", col.name, err)
		}
	}

	return nil
}

// ValidateTicketReferences checks for orphaned ticket references before enabling foreign keys.
// This is called during migration to ensure data integrity.
func ValidateTicketReferences(ctx context.Context, db *sql.DB) error {
	// Check creator_id orphans
	var orphanedCreators int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tickets
		WHERE creator_id NOT IN (SELECT id FROM user)
	`).Scan(&orphanedCreators)
	if err != nil {
		return fmt.Errorf("failed to check creator orphans: %w", err)
	}
	if orphanedCreators > 0 {
		return fmt.Errorf("found %d tickets with invalid creator_id - fix data before enabling foreign keys", orphanedCreators)
	}

	// Check assignee_id orphans
	var orphanedAssignees int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tickets
		WHERE assignee_id IS NOT NULL
		AND assignee_id NOT IN (SELECT id FROM user)
	`).Scan(&orphanedAssignees)
	if err != nil {
		return fmt.Errorf("failed to check assignee orphans: %w", err)
	}
	if orphanedAssignees > 0 {
		return fmt.Errorf("found %d tickets with invalid assignee_id - fix data before enabling foreign keys", orphanedAssignees)
	}

	return nil
}

// GetTableColumns returns all column names for a table (SQLite only)
// This is used for schema validation to ensure migrations create expected columns.
func GetTableColumns(ctx context.Context, db *sql.DB, tableName string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query table info for %s: %w", tableName, err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}
		columns = append(columns, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating columns: %w", err)
	}

	return columns, nil
}

// ValidateTableSchema checks if a table has all required columns (SQLite only)
// Returns an error listing any missing columns. Used for pre-deployment schema validation.
func ValidateTableSchema(ctx context.Context, db *sql.DB, tableName string, requiredColumns []string) error {
	columns, err := GetTableColumns(ctx, db, tableName)
	if err != nil {
		return fmt.Errorf("failed to get columns for %s: %w", tableName, err)
	}

	// Build a set of existing columns for O(1) lookup
	columnSet := make(map[string]bool)
	for _, col := range columns {
		columnSet[col] = true
	}

	// Check for missing columns
	var missing []string
	for _, required := range requiredColumns {
		if !columnSet[required] {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("table %s missing required columns: %v", tableName, missing)
	}

	return nil
}
