//go:build rag

// Command ragquery performs a standalone semantic (hybrid vector + BM25) search
// against a tenant's LanceDB index and appends a human-readable, versioned log
// of every query it issues to bin/ragquery/queries/.
//
// Version: ragquery v1 (2026-07-12)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/usememos/memos/server/router/api/v1/agent"
)

// version identifies this CLI build.
const version = "ragquery v1 (2026-07-12)"

func main() {
	query := flag.String("query", "", "RAG search query text")
	tenantID := flag.Int("tenant", 12, "tenant ID to query")
	lancedbPath := flag.String("lancedb", "build/data/lancedb", "local LanceDB root path (tenant subdir appended automatically)")
	topK := flag.Int("topk", 10, "number of results to return")
	minScore := flag.Float64("min-score", 0.0, "minimum combined score threshold")
	hybrid := flag.Bool("hybrid", true, "enable hybrid (vector + BM25) search")
	vectorWeight := flag.Float64("vector-weight", 0.7, "vector weight for hybrid search")
	textWeight := flag.Float64("text-weight", 0.3, "BM25 weight for hybrid search")
	audience := flag.String("audience", "", "audience filter (empty = both internal and external)")
	logDir := flag.String("log-dir", "bin/ragquery/queries", "directory where per-run query logs are written")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if *query == "" && flag.NArg() > 0 {
		*query = flag.Arg(0)
	}
	if *query == "" {
		fmt.Fprintln(os.Stderr, "error: query is required (use --query or pass as positional arg)")
		os.Exit(2)
	}

	ctx := context.Background()

	cfg := agent.NewVectorDBConfigFromEnv()
	cfg.StorageProvider = "local"
	cfg.LocalPath = *lancedbPath
	cfg.Enabled = true
	cfg.HybridSearchEnabled = *hybrid

	db, err := agent.NewVectorDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize vector DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	embedDim := db.Dimension()

	searchQuery := agent.SearchQuery{
		QueryText:       *query,
		TenantID:        int32(*tenantID),
		AudienceType:    *audience,
		TopK:            *topK,
		MinScore:        *minScore,
		ActiveOnly:      true,
		UseHybridSearch: *hybrid,
		VectorWeight:    *vectorWeight,
		TextWeight:      *textWeight,
	}

	start := time.Now()
	result, err := db.Search(ctx, searchQuery)
	if err != nil {
		fmt.Fprintf(os.Stderr, "search failed: %v\n", err)
		os.Exit(1)
	}
	latencyMs := time.Since(start).Milliseconds()

	mode := result.SearchMode
	if mode == "" {
		mode = "vector"
	}

	// Build the text log entry.
	var sb strings.Builder
	sb.WriteString("=== RAG QUERY " + time.Now().Format(time.RFC3339) + " ===\n")
	fmt.Fprintf(&sb, "cli_version:  %s\n", version)
	fmt.Fprintf(&sb, "query:        %s\n", *query)
	fmt.Fprintf(&sb, "tenant_id:    %d  audience: %s  topk: %d  min_score: %.2f\n",
		*tenantID, audienceLabel(*audience), *topK, *minScore)
	fmt.Fprintf(&sb, "mode:         %s  vector_w: %.2f  text_w: %.2f  embed_dim: %d\n",
		mode, *vectorWeight, *textWeight, embedDim)
	fmt.Fprintf(&sb, "latency_ms:   %d  total_results: %d\n", latencyMs, result.Total)
	sb.WriteString("--- results ---\n")

	for i, chunk := range result.Chunks {
		score, vecScore, bm25Score := 0.0, 0.0, 0.0
		if i < len(result.Scores) {
			score = result.Scores[i]
		}
		if i < len(result.VectorScores) {
			vecScore = result.VectorScores[i]
		}
		if i < len(result.BM25Scores) {
			bm25Score = result.BM25Scores[i]
		}
		content := chunk.Content
		const cap = 2000
		if len(content) > cap {
			content = content[:cap] + "... [truncated]"
		}
		fmt.Fprintf(&sb, "[%d] score=%.4f vec=%.4f bm25=%.4f type=%s audience=%s title=\"%s\" id=%s\n",
			i+1, score, vecScore, bm25Score, chunk.ContentType, chunk.AudienceType, chunk.Title, chunk.ID)
		fmt.Fprintf(&sb, "    %s\n", strings.ReplaceAll(content, "\n", "\n    "))
	}
	sb.WriteString("==============================\n\n")

	// Write to an auto-versioned per-run log file (never overwrites a prior run).
	if err := os.MkdirAll(*logDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log dir %s: %v\n", *logDir, err)
		os.Exit(1)
	}
	logName := fmt.Sprintf("ragquery_%s_t%d_%s_q%s.log",
		time.Now().UTC().Format("2006-01-02T150405Z"),
		*tenantID,
		audienceSlug(*audience),
		slugify(*query))
	logPath := filepath.Join(*logDir, logName)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		// Fall back to an incrementing suffix if the exact timestamp collides.
		logPath = filepath.Join(*logDir, logName+".1")
		f, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
			os.Exit(1)
		}
	}
	defer f.Close()
	if _, err := f.WriteString(sb.String()); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write log: %v\n", err)
		os.Exit(1)
	}

	slog.Info("rag query logged",
		"cliVersion", version,
		"query", *query,
		"tenantID", *tenantID,
		"audience", *audience,
		"mode", mode,
		"embedDim", embedDim,
		"latencyMs", latencyMs,
		"totalResults", result.Total,
		"logFile", logPath)
}

func audienceLabel(a string) string {
	if a == "" {
		return "(both)"
	}
	return a
}

func audienceSlug(a string) string {
	if a == "" {
		return "both"
	}
	return a
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
	}
	if s == "" {
		s = "query"
	}
	return s
}
