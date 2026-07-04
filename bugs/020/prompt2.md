# Implementation Prompt: SQLite to PostgreSQL Migration

**Plan reference:** `bugs/020/plan3.md`
**Review status:** Approved (plan3_review.md)
**Scope:** All 6 sprints, ~4-6 days estimated

---

## Overview

Migrate bchat from SQLite to PostgreSQL (Neon serverless) for production, keeping SQLite for local dev. Fresh deploy — no data migration needed.

**Key files:**
- `store/db/postgres/postgres.go` — Postgres driver (swap lib/pq → pgx/v5)
- `store/db/postgres/agent.go` — 68 stubs to implement
- `store/db/postgres/bridge.go` — 16 stubs to implement
- `store/db/postgres/bridge_auth.go` — 7 stubs to implement
- `store/db/db.go` — store initialization with driver switch (already has postgres case)
- `store/migration/postgres/LATEST.sql` — incomplete (19/53 tables)
- `store/migration/sqlite/LATEST.sql` — complete reference (53 tables)
- `internal/profile/profile.go` — DSN/driver config
- `store/migrator.go` — migration runner (already handles version guards)

---

## Sprint 1: Driver Swap (lib/pq → pgx/v5)

### Step 1: Replace dependency
```bash
cd /home/chaschel/Documents/go/bchat
go get github.com/jackc/pgx/v5
go get github.com/lib/pq@none
go mod tidy
```

Verify:
```bash
grep "lib/pq" go.mod  # must return nothing
go list -m all | grep lib/pq  # must return nothing
```

### Step 2: Replace entire store/db/postgres/postgres.go

**Do NOT merge — replace the entire file** with this content:

```go
package postgres

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pkg/errors"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
)

type DB struct {
	db      *sql.DB
	profile *profile.Profile
}

func NewDB(profile *profile.Profile) (store.Driver, error) {
	if profile == nil {
		return nil, errors.New("profile is nil")
	}

	db, err := sql.Open("pgx", profile.DSN)
	if err != nil {
		log.Printf("Failed to open database: %s", err)
		return nil, errors.Wrapf(err, "failed to open database: %s", profile.DSN)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to ping database")
	}

	return &DB{db: db, profile: profile}, nil
}

func (d *DB) GetDB() *sql.DB {
	return d.db
}

func (d *DB) Close() error {
	return d.db.Close()
}
```

### Step 3: Verify
```bash
go build ./store/db/postgres/...
go build ./server/...
```

---

## Sprint 2: Complete Postgres Schema

### Context

SQLite `LATEST.sql` has 53 tables. Postgres `LATEST.sql` has only 19. You need to add 34 missing tables.

**Important:** The migration runner (`store/migrator.go:preMigrate()`) already handles version guards. For fresh databases, it applies `LATEST.sql` automatically. You just need to complete the file.

### Step 1: Identify missing tables

Compare:
```bash
grep "CREATE TABLE" store/migration/sqlite/LATEST.sql | sed 's/CREATE TABLE IF NOT EXISTS //;s/CREATE TABLE //;s/ (.*//;s/;//' | sort > /tmp/sqlite_tables.txt
grep "CREATE TABLE" store/migration/postgres/LATEST.sql | sed 's/CREATE TABLE IF NOT EXISTS //;s/CREATE TABLE //;s/ (.*//;s/;//' | sort > /tmp/postgres_tables.txt
comm -23 /tmp/sqlite_tables.txt /tmp/postgres_tables.txt
```

### Step 2: Add missing tables to Postgres LATEST.sql

For each missing table, read the SQLite definition and translate:

**Column translations:**

| SQLite | Postgres |
|--------|----------|
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` |
| `BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))` | `BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())` |
| `INTEGER NOT NULL CHECK (x IN (0, 1))` | `BOOLEAN NOT NULL DEFAULT FALSE` |
| `INTEGER NOT NULL DEFAULT 0` (boolean) | `BOOLEAN NOT NULL DEFAULT FALSE` |
| `payload TEXT` | `payload JSONB DEFAULT '{}'` |
| `blob BLOB` | `blob BYTEA` |
| `user` (unquoted) | `"user"` (reserved word) |

**Index translations:**

| SQLite | Postgres |
|--------|----------|
| `CREATE INDEX IF NOT EXISTS` | Same (Postgres supports this) |
| `CREATE UNIQUE INDEX IF NOT EXISTS` | Same |

**Rules:**
- Use `CREATE TABLE IF NOT EXISTS` for all tables
- Keep existing 19 tables unchanged — append new tables at the end of the file
- Include all indexes from the SQLite version
- Preserve all foreign key constraints
- Do NOT add `ON DELETE SET NULL` unless the SQLite version has it

### Step 3: Verify
```bash
# Count tables
grep -c "^CREATE TABLE" store/migration/postgres/LATEST.sql
# Should be 53

# Verify no SQLite-specific syntax
grep -n "AUTOINCREMENT\|strftime\|PRAGMA" store/migration/postgres/LATEST.sql
# Should return nothing
```

---

## Sprint 3: Implement Driver Stubs (91 methods)

### Context

The Postgres driver has 91 stub methods that return `errNotImplemented` or `ErrBridgeUnsupportedDatabase`. You need to implement them by translating from the SQLite driver.

**Files to modify:**
- `store/db/postgres/agent.go` — 68 stubs (source: `store/db/sqlite/agent.go`)
- `store/db/postgres/bridge.go` — 16 stubs (source: `store/db/sqlite/bridge.go`)
- `store/db/postgres/bridge_auth.go` — 7 stubs (source: `store/db/sqlite/bridge_auth.go`)

**Files already complete (0 stubs, do NOT modify):**
- `store/db/postgres/agent_observations.go`
- `store/db/postgres/agent_workflow.go`
- `store/db/postgres/memo_filter.go`
- `store/db/postgres/rbac.go`

### SQL Translation Rules

For every query string:

1. **Placeholders:** `?` → `$1`, `$2`, `$3`... (positional, not named)
2. **Timestamps:** `strftime('%s', 'now')` → `EXTRACT(EPOCH FROM NOW())`
3. **PRAGMA:** `PRAGMA table_info(...)` → `SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1`
4. **Locking:** `BEGIN IMMEDIATE` → `BEGIN` (use `SELECT FOR UPDATE` where needed)
5. **Upsert:** `ON CONFLICT(col) DO UPDATE SET` → same (Postgres 9.5+)
6. **Returning:** `INSERT ... RETURNING id` → same
7. **Boolean values:** `1`/`0` literals → `TRUE`/`FALSE`
8. **Boolean defaults:** `DEFAULT 0` → `DEFAULT FALSE`, `DEFAULT 1` → `DEFAULT TRUE`
9. **String concat:** `||` → same (Postgres supports this)
10. **Auto-increment:** `INTEGER PRIMARY KEY` (implicit autoincrement in SQLite) → `SERIAL PRIMARY KEY`

### Implementation Strategy

For each stub method:
1. Find the corresponding SQLite implementation in `store/db/sqlite/`
2. Copy the logic
3. Translate SQL placeholders and syntax
4. Replace `errNotImplemented` / `ErrBridgeUnsupportedDatabase` with actual implementation
5. Verify compilation after each batch of ~10 methods
6. If compilation fails, fix the errors before moving to the next batch (do not revert)

### Critical: Bridge Delivery

After implementing all 23 bridge stubs, update `SupportsBridgeDelivery()` in `store/db/postgres/` to return `true`.

### Critical: Remove obsolete test

Remove `TestBridgeAuthPostgresUnsupported` from `store/db/postgres/bridge_auth_test.go` — it tests that bridge is unsupported, which is no longer true after implementation. If this test is not removed, the build will fail.

### Verification

After all stubs are implemented:
```bash
# Should return 0 results
grep -rn "errNotImplemented" store/db/postgres/ --include="*.go" | grep -v "_test.go"
grep -rn "ErrBridgeUnsupportedDatabase" store/db/postgres/ --include="*.go" | grep -v "_test.go"

# Build must pass
go build ./store/db/postgres/...
go vet ./store/db/postgres/...
```

---

## Sprint 4: Profile & Configuration

### Step 1: Update internal/profile/profile.go

Add Postgres DSN handling in `Validate()`. Find the existing SQLite DSN block (around line 92) and add after it:

```go
// Postgres DSN from environment
if p.Driver == "postgres" && p.DSN == "" {
    p.DSN = os.Getenv("DATABASE_URL")
    if p.DSN == "" {
        return errors.New("postgres driver requires DSN or DATABASE_URL environment variable")
    }
}
```

Make sure `os` is imported.

### Step 2: Verify store initialization (DO NOT EDIT)

The driver switch already exists in `store/db/db.go`. **Do not create a new driver switch** — verify the existing one includes the postgres case:

```go
// Should already exist in store/db/db.go
switch profile.Driver {
case "sqlite":
    driver, err = sqlite.NewDB(profile)
case "postgres":
    driver, err = postgres.NewDB(profile)
}
```

If the postgres case is missing, add it. If it exists, skip this step.

### Step 3: Update .env.example (project root)

Add at the end of `.env.example`:
```bash
# Database driver: "sqlite" or "postgres"
DB_DRIVER=sqlite

# For Postgres (Neon):
DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require"
```

### Verification
```bash
go build ./...
```

---

## Sprint 5: Testing

### Step 1: Remove obsolete test

Delete `TestBridgeAuthPostgresUnsupported` from `store/db/postgres/bridge_auth_test.go`. This test asserts bridge is unsupported on Postgres, which is no longer true.

### Step 2: Run full test suite
```bash
go test ./store/db/postgres/... -v
go test ./store/... -v
```

### Step 3: Manual verification against Neon
```bash
# Set env vars
export DB_DRIVER=postgres
export DATABASE_URL="postgresql://...?sslmode=require&channel_binding=require"

# Run the app
go run ./bin/memos --mode dev

# Test endpoints
curl http://localhost:5230/api/v1/status
```

---

## Sprint 6: Deploy

### Step 1: Create Neon project
1. Go to https://console.neon.tech
2. Create project "bchat-prod"
3. Copy connection string

### Step 2: Set Fly.io secrets
```bash
fly secrets set DB_DRIVER=postgres
fly secrets set DATABASE_URL="postgresql://...?sslmode=require&channel_binding=require"
```

### Step 3: Deploy
```bash
fly deploy
```

### Step 4: Verify
```bash
fly logs
psql $DATABASE_URL -c "\dt"  # should show 53 tables
```

---

## Do NOT

- Modify any SQLite driver files
- Change the existing 19 Postgres tables in LATEST.sql
- Skip the `go list -m all | grep lib/pq` verification
- Implement stubs in `agent_observations.go`, `agent_workflow.go`, `memo_filter.go`, or `rbac.go` (already complete)
- Use `DO $$` blocks in SQL migrations (no plpgsql)
- Merge into `postgres.go` — replace the entire file
- Create a new driver switch in `store/db/db.go` — it already exists
