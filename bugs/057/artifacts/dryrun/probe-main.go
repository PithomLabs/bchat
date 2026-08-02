package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store/db"
)

func main() {
	dsn := os.Getenv("COCKROACH_DSN")
	if dsn == "" {
		log.Fatal("COCKROACH_DSN required")
	}
	port := 5231
	if p := os.Getenv("PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}
	data := os.Getenv("DATA_DIR")
	if data == "" {
		data = "/tmp/dryrun-data"
	}
	if err := os.MkdirAll(data, 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	p := &profile.Profile{
		Mode:   "prod",
		Driver: "cockroach",
		DSN:    dsn,
		Port:   port,
		Data:   data,
	}

	start := time.Now()
	driver, err := db.NewDBDriver(p)
	if err != nil {
		log.Fatalf("driver init failed: %v", err)
	}
	defer driver.Close()
	fmt.Printf("driver init OK (%.3fs) — pgx connection + ping via app code path\n", time.Since(start).Seconds())

	ctx := context.Background()
	if _, err := driver.GetDB().ExecContext(ctx, "SELECT 1"); err != nil {
		log.Fatalf("SELECT 1 failed: %v", err)
	}
	fmt.Println("SELECT 1 OK")

	// Keep-alive loop: prove machine lifetime extends beyond the ~6 min that
	// killed attempt-1. Also serves as the no-listen window for the health
	// check (no /healthz endpoint in this probe).
	deadline := time.Now().Add(20 * time.Minute)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for now := range tick.C {
		if _, err := driver.GetDB().ExecContext(ctx, "SELECT 1"); err != nil {
			log.Printf("keepalive SELECT 1 failed: %v", err)
		} else {
			fmt.Printf("keepalive OK elapsed=%.0fs\n", now.Sub(start).Seconds())
		}
		if now.After(deadline) {
			fmt.Println("probe window complete — exiting cleanly")
			os.Exit(0)
		}
	}
}
