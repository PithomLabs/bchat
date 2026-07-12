//go:build rag

// Command reindex performs a local RAG reindex of a tenant's source files into
// the local LanceDB store, reusing bchat's own agent.Service pipeline.
//
// It reads the latest source files (KB/Policy) for a tenant+audience from
// SQLite, chunks and embeds them (OpenRouter), and writes them into the local
// LanceDB table. This is used to populate a local index when the production
// index lives in S3 and is unreachable.
//
// Version: ragquery reindex v1 (2026-07-12)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
	"github.com/usememos/memos/store/db"
	agent "github.com/usememos/memos/server/router/api/v1/agent"
)

func main() {
	dsn := flag.String("dsn", "build/data/memos_dev.db", "SQLite DSN (path to memos DB file)")
	tenantID := flag.Int("tenant", 12, "tenant ID to reindex")
	audience := flag.String("audience", "external", "audience to reindex (external/internal/all)")
	resume := flag.Bool("resume", false, "resume from last failed checkpoint")
	flag.Parse()

	ctx := context.Background()

	prof := &profile.Profile{
		Mode:   "dev",
		Driver: "sqlite",
		DSN:    *dsn,
	}

	driver, err := db.NewDBDriver(prof)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open DB driver: %v\n", err)
		os.Exit(1)
	}
	st := store.New(driver, prof)

	// NewService builds the VectorDB from env. Ensure LANCEDB_STORAGE_PROVIDER=local
	// and RAG_PIPELINE_ENABLED=true in the environment before running this.
	svc := agent.NewService(st, prof)

	slog.Info("starting local RAG reindex",
		"tenantID", *tenantID,
		"audience", *audience,
		"resume", *resume)

	n, err := svc.ReindexTenantContentWithResume(ctx, int32(*tenantID), *audience, *resume)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reindex failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("reindex complete: %d chunks indexed for tenant=%d audience=%s\n", n, *tenantID, *audience)
}
