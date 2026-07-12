//go:build rag

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/usememos/memos/server/router/api/v1/agent"
)

func main() {
	cfg := agent.NewVectorDBConfigFromEnv()
	cfg.StorageProvider = "local"
	cfg.LocalPath = "build/data/lancedb"
	cfg.Enabled = true
	cfg.HybridSearchEnabled = false
	db, err := agent.NewVectorDB(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init err:", err)
		os.Exit(1)
	}
	defer db.Close()
	chunks, err := db.ListChunks(context.Background(), 12)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list err:", err)
		os.Exit(1)
	}
	fmt.Printf("total chunks for tenant 12: %d\n", len(chunks))
	byAud := map[string]int{}
	byAudActive := map[string]int{}
	byType := map[string]int{}
	for _, c := range chunks {
		byAud[c.AudienceType]++
		if c.IsActive {
			byAudActive[c.AudienceType]++
		}
		byType[c.ContentType]++
	}
	fmt.Println("by audience_type (all):", byAud)
	fmt.Println("by audience_type (active only):", byAudActive)
	fmt.Println("by content_type:", byType)
	// show a few external sample ids
	n := 0
	for _, c := range chunks {
		if c.AudienceType == "external" {
			fmt.Printf("EXT sample: id=%s title=%q active=%v type=%s\n", c.ID, c.Title, c.IsActive, c.ContentType)
			n++
			if n >= 5 {
				break
			}
		}
	}
}
