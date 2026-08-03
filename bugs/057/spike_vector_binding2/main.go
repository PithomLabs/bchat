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

	// Test 4: Try with explicit float4[] cast
	fmt.Println("=== Test 4: Bound []float32 via ARRAY[$1]::float4[]::VECTOR(2) ===")
	embedding := []float32{0.1, 0.2}
	var result4 string
	err = db.QueryRowContext(ctx, `SELECT ARRAY[$1]::float4[]::VECTOR(2)`, embedding).Scan(&result4)
	if err != nil {
		log.Printf("FAIL: %v", err)
	} else {
		log.Printf("OK: %s", result4)
	}

	// Test 5: Try with explicit format as string literal
	fmt.Println("\n=== Test 5: Format vector as string literal on Go side ===")
	vecStr := fmt.Sprintf("[%v]", embedding)
	// Replace " " with "," and clean up
	vecStr = strings.ReplaceAll(strings.Trim(vecStr, "[]"), " ", ",")
	vecStr = "[" + vecStr + "]"
	fmt.Printf("Vector string: %s\n", vecStr)
	var result5 string
	err = db.QueryRowContext(ctx, `SELECT $1::VECTOR`, vecStr).Scan(&result5)
	if err != nil {
		log.Printf("FAIL: %v", err)
	} else {
		log.Printf("OK: %s", result5)
	}

	// Test 6: Try with []float64 instead
	fmt.Println("\n=== Test 6: Bound []float64 via ARRAY[$1]::VECTOR(2) ===")
	embedding64 := []float64{0.1, 0.2}
	var result6 string
	err = db.QueryRowContext(ctx, `SELECT ARRAY[$1]::VECTOR(2)`, embedding64).Scan(&result6)
	if err != nil {
		log.Printf("FAIL: %v", err)
	} else {
		log.Printf("OK: %s", result6)
	}

	// Test 7: Check what type pgx actually sends - use pgtype
	fmt.Println("\n=== Test 7: Use pgtype.Float4Array ===")
	// This requires importing pgtype - skip for now, just test with array syntax
	var result7 string
	err = db.QueryRowContext(ctx, `SELECT ARRAY[0.1,0.2]::float4[]::VECTOR(2)`).Scan(&result7)
	if err != nil {
		log.Printf("FAIL: %v", err)
	} else {
		log.Printf("OK: %s", result7)
	}

	// Test 8: Try using the vector type directly with ::vector
	fmt.Println("\n=== Test 8: Direct ::vector cast on array literal ===")
	var result8 string
	err = db.QueryRowContext(ctx, `SELECT ARRAY[0.1,0.2]::vector`).Scan(&result8)
	if err != nil {
		log.Printf("FAIL: %v", err)
	} else {
		log.Printf("OK: %s", result8)
	}
}