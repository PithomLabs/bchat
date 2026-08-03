package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: spike_migration <mode: oneshot|perstmt> <cockroach_dsn>")
	}

	mode := os.Args[1]
	dsn := os.Args[2]

	if !strings.Contains(dsn, "default_query_exec_mode") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "default_query_exec_mode=simple_protocol"
	}

	// Read LATEST.sql
	sqlContent, err := os.ReadFile("/home/chaschel/Documents/go/bchat/store/migration/cockroach/LATEST.sql")
	if err != nil {
		log.Fatalf("Failed to read LATEST.sql: %v", err)
	}

	// Parse statements (split on semicolon + newline)
	statements := parseStatements(string(sqlContent))
	fmt.Printf("Total statements: %d\n", len(statements))

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}

	// Reset DB to A1
	fmt.Println("Resetting database...")
	resetDB(ctx, db)

	fmt.Printf("\n=== Experiment Mode: %s ===\n", mode)
	start := time.Now()

	if mode == "oneshot" {
		runOneshot(ctx, db, string(sqlContent))
	} else if mode == "perstmt" {
		runPerStatement(ctx, db, statements)
	} else {
		log.Fatalf("Unknown mode: %s", mode)
	}

	elapsed := time.Since(start)
	fmt.Printf("\nTotal time: %v\n", elapsed)

	// Verify migration history
	var version string
	err = db.QueryRowContext(ctx, "SELECT version FROM migration_history").Scan(&version)
	if err != nil {
		log.Printf("Migration history check failed: %v", err)
	} else {
		log.Printf("Migration version: %s", version)
	}

	// Count tables
	var count int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'`).Scan(&count)
	if err == nil {
		log.Printf("Tables created: %d", count)
	}
}

func parseStatements(sql string) []string {
	// Split on semicolon followed by newline
	var statements []string
	var current strings.Builder
	lines := strings.Split(sql, "\n")

	for _, line := range lines {
		current.WriteString(line + "\n")
		if strings.HasSuffix(strings.TrimSpace(line), ";") {
			stmt := current.String()
			current.Reset()
			stmt = strings.TrimSpace(stmt)
			if stmt != "" && !strings.HasPrefix(stmt, "--") {
				statements = append(statements, stmt)
			}
		}
	}
	return statements
}

func resetDB(ctx context.Context, db *sql.DB) {
	// Drop all tables
	rows, err := db.QueryContext(ctx, `
		SELECT string_agg(format('DROP TABLE IF EXISTS %I CASCADE', table_name), '; ')
		FROM information_schema.tables
		WHERE table_schema = 'public'
	`)
	if err != nil {
		log.Printf("Failed to list tables: %v", err)
		return
	}
	defer rows.Close()

	var stmt string
	if rows.Next() {
		rows.Scan(&stmt)
	}
	rows.Close()

	if stmt != "" {
		_, err = db.ExecContext(ctx, stmt)
		if err != nil {
			log.Printf("Failed to drop tables: %v", err)
		}
	}
}

func runOneshot(ctx context.Context, db *sql.DB, sqlContent string) {
	fmt.Println("Running one-shot ExecContext...")
	start := time.Now()

	_, err := db.ExecContext(ctx, sqlContent)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("Oneshot failed: %v", err)
	} else {
		log.Printf("Oneshot completed in: %v", elapsed)
	}
}

func runPerStatement(ctx context.Context, db *sql.DB, statements []string) {
	fmt.Printf("Running %d statements individually (autocommit)...\n", len(statements))
	start := time.Now()

	success := 0
	failed := 0

	for i, stmt := range statements {
		if strings.HasPrefix(strings.TrimSpace(stmt), "--") {
			continue
		}

		stmtStart := time.Now()
		_, err := db.ExecContext(ctx, stmt)
		elapsed := time.Since(stmtStart)

		if err != nil {
			failed++
			log.Printf("Statement %d failed (%v): %v\n%s", i, elapsed, err, stmt[:min(100, len(stmt))])
		} else {
			success++
			if elapsed > 500*time.Millisecond {
				log.Printf("Statement %d took %v: %s", i, elapsed, stmt[:min(80, len(stmt))])
			}
		}

		if i%50 == 0 && i > 0 {
			log.Printf("Progress: %d/%d statements, %d failed", i, len(statements), failed)
		}
	}

	totalElapsed := time.Since(start)
	log.Printf("Per-statement completed in: %v (success: %d, failed: %d)", totalElapsed, success, failed)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}