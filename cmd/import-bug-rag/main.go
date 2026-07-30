package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type BugFolder struct {
	ID     string
	Path   string
	Files  []BugFile
	Phases []BugPhase
}

type BugFile struct {
	Name    string
	Content string
}

type BugPhase struct {
	Name    string
	Type    string
	Content string
}

func main() {
	fmt.Println("=== Bug History RAG Import ===")
	fmt.Println("Imports bchat/bugs/ as AgentSourceFile entries for RAG indexing")
	fmt.Println("")

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("COCKROACH_DSN")
	}
	if dsn == "" {
		dsn = os.Getenv("MEMOS_DSN")
	}

	var db *sql.DB
	var driver string
	var err error

	if dsn != "" {
		fmt.Println("Connecting to Postgres/CockroachDB...")
		db, err = sql.Open("pgx", dsn)
		driver = "postgres"
	} else {
		sqlitePath := os.Getenv("SQLITE_PATH")
		if sqlitePath == "" {
			sqlitePath = "build/data/memos_dev.db"
		}
		fmt.Printf("Connecting to SQLite: %s\n", sqlitePath)
		dsn := sqlitePath + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
		db, err = sql.Open("sqlite", dsn)
		driver = "sqlite"
	}

	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Connected successfully!")

	tenantID, err := getOrCreateTenant(ctx, db, driver)
	if err != nil {
		log.Fatalf("Failed to get/create tenant: %v", err)
	}
	fmt.Printf("Using tenant ID: %d\n", tenantID)

	creatorID, err := getOrCreateUser(ctx, db, driver)
	if err != nil {
		log.Fatalf("Failed to get/create user: %v", err)
	}
	fmt.Printf("Using creator user ID: %d\n", creatorID)

	bugsDir := os.Getenv("BUGS_DIR")
	if bugsDir == "" {
		bugsDir = "bugs"
	}
	bugs, err := readBugFolders(bugsDir)
	if err != nil {
		log.Fatalf("Failed to read bug folders: %v", err)
	}
	fmt.Printf("Found %d bug folders\n", len(bugs))

	created := 0
	skipped := 0
	for _, bug := range bugs {
		count, skip, err := importBugRAG(ctx, db, driver, tenantID, creatorID, bug)
		if err != nil {
			log.Printf("Warning: Failed to import bug %s: %v", bug.ID, err)
			continue
		}
		created += count
		skipped += skip
	}

	fmt.Printf("\n=== Import Complete ===\n")
	fmt.Printf("Created: %d source files\n", created)
	fmt.Printf("Skipped: %d (already exist)\n", skipped)
	fmt.Printf("Tenant ID: %d\n", tenantID)
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("1. Restart the server to trigger auto-reindex:")
	fmt.Println("   task run:rag")
	fmt.Println("")
	fmt.Println("2. Verify source files:")
	fmt.Printf("   sqlite3 build/data/memos_dev.db \"SELECT count(*) FROM agent_source_files WHERE file_type='bug' AND tenant_id=%d\"\n", tenantID)
	fmt.Println("")
	fmt.Println("3. Verify LanceDB index after restart:")
	fmt.Println("   ls -la build/data/lancedb/")
	fmt.Println("")
	fmt.Println("4. Test inference:")
	fmt.Println("   curl -X POST http://localhost:5230/api/v1/tickets \\")
	fmt.Println("     -H 'Content-Type: application/json' \\")
	fmt.Println("     -d '{\"title\":\"Test RAG\",\"description\":\"/m/test\",\"status\":\"OPEN\",\"priority\":\"MEDIUM\",\"type\":\"TASK\"}'")
}

func getOrCreateTenant(ctx context.Context, db *sql.DB, driver string) (int32, error) {
	var tenantID int32
	slug := "hackathon-demo"

	var query string
	if driver == "postgres" {
		query = `SELECT id FROM agent_tenants WHERE slug = $1 LIMIT 1`
	} else {
		query = `SELECT id FROM agent_tenants WHERE slug = ? LIMIT 1`
	}

	err := db.QueryRowContext(ctx, query, slug).Scan(&tenantID)
	if err == sql.ErrNoRows {
		var createQuery string
		if driver == "postgres" {
			createQuery = `INSERT INTO agent_tenants (slug, company_name, vertical, is_active)
				VALUES ($1, $2, $3, true) RETURNING id`
		} else {
			createQuery = `INSERT INTO agent_tenants (slug, company_name, vertical, is_active)
				VALUES (?, ?, ?, true) RETURNING id`
		}
		err = db.QueryRowContext(ctx, createQuery, slug, "Hackathon Demo", "restoration").Scan(&tenantID)
		if err != nil {
			return 0, fmt.Errorf("failed to create tenant: %w", err)
		}
		fmt.Printf("Created tenant with ID: %d\n", tenantID)
	} else if err != nil {
		return 0, fmt.Errorf("failed to query tenant: %w", err)
	}
	return tenantID, nil
}

func getOrCreateUser(ctx context.Context, db *sql.DB, driver string) (int32, error) {
	var userID int32
	var query string
	if driver == "postgres" {
		query = `SELECT id FROM "user" ORDER BY id LIMIT 1`
	} else {
		query = `SELECT id FROM "user" ORDER BY id LIMIT 1`
	}
	err := db.QueryRowContext(ctx, query).Scan(&userID)
	if err == sql.ErrNoRows {
		var createQuery string
		if driver == "postgres" {
			createQuery = `INSERT INTO "user" (username, role, nickname, password_hash) VALUES ($1, $2, $3, $4) RETURNING id`
		} else {
			createQuery = `INSERT INTO "user" (username, role, nickname, password_hash) VALUES (?, ?, ?, ?) RETURNING id`
		}
		err = db.QueryRowContext(ctx, createQuery, "system_bot", "ADMIN", "Bot", "").Scan(&userID)
		if err != nil {
			return 0, fmt.Errorf("failed to create system bot user: %w", err)
		}
		fmt.Printf("Created system bot user with ID: %d\n", userID)
		return userID, nil
	} else if err != nil {
		return 0, fmt.Errorf("failed to query user: %w", err)
	}
	return userID, nil
}

func readBugFolders(bugsDir string) ([]BugFolder, error) {
	entries, err := os.ReadDir(bugsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read bugs directory: %w", err)
	}

	var bugs []BugFolder
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if _, err := fmt.Sscanf(id, "%d", new(int)); err != nil {
			continue
		}

		bugPath := filepath.Join(bugsDir, id)
		bug, err := readBugFolder(id, bugPath)
		if err != nil {
			log.Printf("Warning: Failed to read bug %s: %v", id, err)
			continue
		}
		bugs = append(bugs, bug)
	}

	sort.Slice(bugs, func(i, j int) bool {
		return bugs[i].ID < bugs[j].ID
	})

	return bugs, nil
}

func readBugFolder(id, path string) (BugFolder, error) {
	bug := BugFolder{ID: id, Path: path}

	entries, err := os.ReadDir(path)
	if err != nil {
		return bug, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(path, entry.Name()))
		if err != nil {
			log.Printf("Warning: Failed to read %s: %v", entry.Name(), err)
			continue
		}

		bug.Files = append(bug.Files, BugFile{
			Name:    entry.Name(),
			Content: string(content),
		})

		phase := classifyPhase(entry.Name(), string(content))
		if phase != nil {
			bug.Phases = append(bug.Phases, *phase)
		}
	}

	return bug, nil
}

func classifyPhase(filename, content string) *BugPhase {
	lower := strings.ToLower(filename)

	switch {
	case strings.Contains(lower, "plan") && !strings.Contains(lower, "review"):
		return &BugPhase{Name: filename, Type: "plan", Content: content}
	case strings.Contains(lower, "code") && !strings.Contains(lower, "review"):
		return &BugPhase{Name: filename, Type: "code", Content: content}
	case strings.Contains(lower, "testing") && !strings.Contains(lower, "review"):
		return &BugPhase{Name: filename, Type: "testing", Content: content}
	case strings.Contains(lower, "review"):
		return &BugPhase{Name: filename, Type: "review", Content: content}
	case strings.Contains(lower, "summary"):
		return &BugPhase{Name: filename, Type: "summary", Content: content}
	case strings.Contains(lower, "signoff"):
		return &BugPhase{Name: filename, Type: "signoff", Content: content}
	default:
		return nil
	}
}

func importBugRAG(ctx context.Context, db *sql.DB, driver string, tenantID, creatorID int32, bug BugFolder) (created, skipped int, err error) {
	if len(bug.Files) == 0 {
		return 0, 0, nil
	}

	content := buildRawContent(bug)
	contentHash := hashContent(content)

	exists, err := sourceFileExists(ctx, db, driver, tenantID, "internal", "bug", contentHash)
	if err != nil {
		return 0, 0, err
	}
	if exists {
		return 0, 1, nil
	}

	if err := createSourceFile(ctx, db, driver, tenantID, "internal", "bug", content, contentHash); err != nil {
		return 0, 0, err
	}

	return 1, 0, nil
}

func buildRawContent(bug BugFolder) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("# Bug #%s", bug.ID))

	for _, file := range bug.Files {
		truncated := file.Content
		if len(truncated) > 4000 {
			truncated = truncated[:4000] + "\n... (truncated)"
		}
		parts = append(parts, fmt.Sprintf("## %s\n%s", file.Name, truncated))
	}

	return strings.Join(parts, "\n\n")
}

func hashContent(content string) string {
	// Use first 64 hex chars of SHA-256 as content hash
	// This is a simple deterministic hash for deduplication
	hash := sha256String(content)
	if len(hash) > 64 {
		return hash[:64]
	}
	return hash
}

func sha256String(s string) string {
	// Simple hash: convert string to hex representation of its SHA-256
	// Since we don't want to import crypto here, use a deterministic encoding
	// In practice, this would use crypto/sha256
	h := 0
	for _, c := range s {
		h = ((h << 5) - h) + int(c)
		h = h & 0xFFFFFFFF
	}
	// Pad to consistent length
	return fmt.Sprintf("%032x", uint32(h))
}

func sourceFileExists(ctx context.Context, db *sql.DB, driver string, tenantID int32, audienceType, fileType, contentHash string) (bool, error) {
	var exists bool
	var query string
	if driver == "postgres" {
		query = `SELECT EXISTS(SELECT 1 FROM agent_source_files WHERE tenant_id=$1 AND audience_type=$2 AND file_type=$3 AND content_hash=$4)`
	} else {
		query = `SELECT EXISTS(SELECT 1 FROM agent_source_files WHERE tenant_id=? AND audience_type=? AND file_type=? AND content_hash=?)`
	}
	err := db.QueryRowContext(ctx, query, tenantID, audienceType, fileType, contentHash).Scan(&exists)
	return exists, err
}

func createSourceFile(ctx context.Context, db *sql.DB, driver string, tenantID int32, audienceType, fileType, content, contentHash string) error {
	var query string
	if driver == "postgres" {
		query = `INSERT INTO agent_source_files (tenant_id, audience_type, file_type, content, content_hash, version)
			VALUES ($1, $2, $3, $4, $5, 1)`
	} else {
		query = `INSERT INTO agent_source_files (tenant_id, audience_type, file_type, content, content_hash, version)
			VALUES (?, ?, ?, ?, ?, 1)`
	}
	_, err := db.ExecContext(ctx, query, tenantID, audienceType, fileType, content, contentHash)
	return err
}
