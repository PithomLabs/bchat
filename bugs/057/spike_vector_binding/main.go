package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := os.Getenv("COCKROACH_DSN")
	if dsn == "" {
		log.Fatal("COCKROACH_DSN required")
	}

	// Ensure simple_protocol for CRDB
	if !strings.Contains(dsn, "default_query_exec_mode") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "default_query_exec_mode=simple_protocol"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}

	// Test 1: Control — raw SQL vector literal
	fmt.Println("=== Test 1: Raw SQL literal (control) ===")
	var result1 string
	err = db.QueryRowContext(ctx, `SELECT ARRAY[0.1,0.2]::VECTOR(2)`).Scan(&result1)
	if err != nil {
		log.Printf("FAIL: %v", err)
	} else {
		log.Printf("OK: %s", result1)
	}

	// Test 2: THE QUESTION — bound parameter via ARRAY[$1]::VECTOR
	fmt.Println("\n=== Test 2: Bound []float32 via ARRAY[$1]::VECTOR ===")
	embedding := []float32{0.1, 0.2}
	var result2 string
	err = db.QueryRowContext(ctx, `SELECT ARRAY[$1]::VECTOR(2)`, embedding).Scan(&result2)
	if err != nil {
		log.Printf("FAIL: %v", err)
	} else {
		log.Printf("OK: %s", result2)
	}

	// Test 3: Fallback candidate — bound parameter via $1::VECTOR (no ARRAY wrapper)
	fmt.Println("\n=== Test 3: Bound []float32 via $1::VECTOR (fallback candidate) ===")
	var result3 string
	err = db.QueryRowContext(ctx, `SELECT $1::VECTOR`, embedding).Scan(&result3)
	if err != nil {
		log.Printf("FAIL: %v", err)
	} else {
		log.Printf("OK: %s", result3)
	}
}