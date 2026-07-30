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

	"github.com/lithammer/shortuuid/v4"
	_ "github.com/jackc/pgx/v5/stdlib"
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
	Type    string // plan, code, testing, review
	Content string
}

func main() {
	fmt.Println("=== Bug Import Script ===")
	fmt.Println("Imports bugs/001-050 as tickets with memo-comment summaries")
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
		count, skip, err := importBug(ctx, db, driver, tenantID, creatorID, bug)
		if err != nil {
			log.Printf("Warning: Failed to import bug %s: %v", bug.ID, err)
			continue
		}
		created += count
		skipped += skip
	}

	fmt.Printf("\n=== Import Complete ===\n")
	fmt.Printf("Created: %d tickets\n", created)
	fmt.Printf("Skipped: %d (already exist)\n", skipped)
	fmt.Printf("Tenant ID: %d\n", tenantID)
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("1. Verify tickets:")
	fmt.Println("   sqlite3 build/data/memos_dev.db \"SELECT id, description FROM tickets WHERE type='BUG' LIMIT 5;\"")
	fmt.Println("")
	fmt.Println("2. Verify memo comments:")
	fmt.Println("   sqlite3 build/data/memos_dev.db \"SELECT m.uid, substr(m.content,1,40) FROM memo m JOIN memo_relation r ON m.id=r.memo_id WHERE r.type='COMMENT' LIMIT 5;\"")
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

func importBug(ctx context.Context, db *sql.DB, driver string, tenantID, creatorID int32, bug BugFolder) (created, skipped int, err error) {
	if len(bug.Files) == 0 {
		return 0, 0, nil
	}

	title := fmt.Sprintf("Bug #%s: %s", bug.ID, extractTopic(bug))
	exists, err := ticketExists(ctx, db, driver, title, tenantID)
	if err != nil {
		return 0, 0, err
	}
	if exists {
		return 0, 1, nil
	}

	status := determineStatus(bug)
	priority := determinePriority(bug)

	descMemoID, descUID, err := createDescriptionMemo(ctx, db, driver, tenantID, creatorID, bug)
	if err != nil {
		return 0, 0, fmt.Errorf("create description memo: %w", err)
	}

	description := "/m/" + descUID
	if err := createTicket(ctx, db, driver, tenantID, creatorID, title, description, status, priority); err != nil {
		return 0, 0, fmt.Errorf("create ticket: %w", err)
	}

	commentMemoID, err := createCommentMemo(ctx, db, driver, tenantID, creatorID, bug)
	if err != nil {
		return 0, 0, fmt.Errorf("create comment memo: %w", err)
	}

	if err := linkMemoComment(ctx, db, driver, commentMemoID, descMemoID, &tenantID); err != nil {
		return 0, 0, fmt.Errorf("link memo comment: %w", err)
	}

	return 1, 0, nil
}

func createDescriptionMemo(ctx context.Context, db *sql.DB, driver string, tenantID, creatorID int32, bug BugFolder) (int32, string, error) {
	uid := shortuuid.New()
	topic := extractTopic(bug)
	content := fmt.Sprintf("Bug #%s — %s", bug.ID, topic)

	var query string
	if driver == "postgres" {
		query = `INSERT INTO memo (uid, creator_id, content, visibility, payload, tenant_id)
			VALUES ($1, $2, $3, 'PUBLIC', '{}', $4) RETURNING id`
	} else {
		query = `INSERT INTO memo (uid, creator_id, content, visibility, payload, tenant_id)
			VALUES (?, ?, ?, 'PUBLIC', '{}', ?) RETURNING id`
	}

	var memoID int32
	err := db.QueryRowContext(ctx, query, uid, creatorID, content, tenantID).Scan(&memoID)
	return memoID, uid, err
}

func createCommentMemo(ctx context.Context, db *sql.DB, driver string, tenantID, creatorID int32, bug BugFolder) (int32, error) {
	uid := shortuuid.New()
	content := buildInternalNotes(bug)

	var query string
	if driver == "postgres" {
		query = `INSERT INTO memo (uid, creator_id, content, visibility, payload, tenant_id)
			VALUES ($1, $2, $3, 'PUBLIC', '{}', $4) RETURNING id`
	} else {
		query = `INSERT INTO memo (uid, creator_id, content, visibility, payload, tenant_id)
			VALUES (?, ?, ?, 'PUBLIC', '{}', ?) RETURNING id`
	}

	var memoID int32
	err := db.QueryRowContext(ctx, query, uid, creatorID, content, tenantID).Scan(&memoID)
	return memoID, err
}

func linkMemoComment(ctx context.Context, db *sql.DB, driver string, commentMemoID, descriptionMemoID int32, tenantID *int32) error {
	var query string
	if driver == "postgres" {
		query = `INSERT INTO memo_relation (memo_id, related_memo_id, type, tenant_id)
			VALUES ($1, $2, 'COMMENT', $3)`
	} else {
		query = `INSERT INTO memo_relation (memo_id, related_memo_id, type, tenant_id)
			VALUES (?, ?, 'COMMENT', ?)`
	}
	_, err := db.ExecContext(ctx, query, commentMemoID, descriptionMemoID, tenantID)
	return err
}

func buildInternalNotes(bug BugFolder) string {
	if len(bug.Phases) == 0 {
		return "Pending summary..."
	}

	var notes []string
	notes = append(notes, fmt.Sprintf("Bug #%s - %d files across %d phases", bug.ID, len(bug.Files), len(bug.Phases)))
	notes = append(notes, "")

	for _, phase := range bug.Phases {
		summary := extractKeyPoints(phase.Content, 500)
		notes = append(notes, fmt.Sprintf("### %s (%s)\n%s", phase.Name, phase.Type, summary))
	}

	return strings.Join(notes, "\n\n")
}

func extractKeyPoints(content string, maxLen int) string {
	lines := strings.Split(content, "\n")
	var keyPoints []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") ||
			strings.Contains(line, "root cause") || strings.Contains(line, "fix") || strings.Contains(line, "solution") {
			keyPoints = append(keyPoints, line)
		}
		if len(strings.Join(keyPoints, "\n")) > maxLen {
			break
		}
	}

	result := strings.Join(keyPoints, "\n")
	if result == "" {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}
	return result
}

func determineStatus(bug BugFolder) string {
	for _, phase := range bug.Phases {
		if phase.Type == "signoff" {
			return "CLOSED"
		}
		if strings.Contains(strings.ToLower(phase.Name), "signoff") {
			return "CLOSED"
		}
	}
	for _, file := range bug.Files {
		if strings.Contains(strings.ToLower(file.Name), "signoff") {
			return "CLOSED"
		}
	}
	return "IN_PROGRESS"
}

func determinePriority(bug BugFolder) string {
	for _, file := range bug.Files {
		content := strings.ToLower(file.Content)
		if strings.Contains(content, "critical") || strings.Contains(content, "urgent") || strings.Contains(content, "p0") {
			return "HIGH"
		}
	}
	if len(bug.Files) > 15 {
		return "HIGH"
	}
	if len(bug.Files) > 5 {
		return "MEDIUM"
	}
	return "LOW"
}

func extractTopic(bug BugFolder) string {
	for _, file := range bug.Files {
		if strings.Contains(strings.ToLower(file.Name), "summary") {
			lines := strings.Split(file.Content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "#") {
					topic := strings.TrimLeft(line, "# ")
					if topic != "" {
						return topic
					}
				}
			}
		}
	}

	for _, file := range bug.Files {
		if strings.Contains(strings.ToLower(file.Name), "plan") && !strings.Contains(strings.ToLower(file.Name), "review") {
			lines := strings.Split(file.Content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "#") {
					topic := strings.TrimLeft(line, "# ")
					if topic != "" {
						return topic
					}
				}
			}
		}
	}

	return fmt.Sprintf("Bug %s", bug.ID)
}

func ticketExists(ctx context.Context, db *sql.DB, driver string, title string, tenantID int32) (bool, error) {
	var exists bool
	var query string
	if driver == "postgres" {
		query = `SELECT EXISTS(SELECT 1 FROM tickets WHERE title = $1 AND tenant_id = $2)`
	} else {
		query = `SELECT EXISTS(SELECT 1 FROM tickets WHERE title = ? AND tenant_id = ?)`
	}
	err := db.QueryRowContext(ctx, query, title, tenantID).Scan(&exists)
	return exists, err
}

func createTicket(ctx context.Context, db *sql.DB, driver string, tenantID, creatorID int32, title, description, status, priority string) error {
	now := time.Now().Unix()
	var query string
	if driver == "postgres" {
		query = `INSERT INTO tickets (title, description, status, priority, creator_id,
			created_ts, updated_ts, type, tags, tenant_id, internal_notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'')`
	} else {
		query = `INSERT INTO tickets (title, description, status, priority, creator_id,
			created_ts, updated_ts, type, tags, tenant_id, internal_notes)
			VALUES (?,?,?,?,?,?,?,?,?,?,'')`
	}

	_, err := db.ExecContext(ctx, query,
		title, description, status, priority,
		creatorID,
		now, now,
		"BUG",
		`["imported","bug"]`,
		tenantID,
	)
	return err
}
