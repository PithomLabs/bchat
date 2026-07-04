# Implementation Plan: SQLite to PostgreSQL Migration (Neon)

**Plan reference:** `bugs/020/prompt2.md`  
**Scope:** Fresh deploy on Neon serverless Postgres, keeping SQLite for local dev  
**Deploy target:** Fly.io (existing)  
**Sprints:** 6  

---

## Context

- SQLite `LATEST.sql` has 53 tables; Postgres `LATEST.sql` has only 19
- Postgres driver has 91 stub methods returning `errNotImplemented` or `ErrBridgeUnsupportedDatabase`
- `store/db/db.go` already has the postgres driver switch
- `store/migrator.go` already handles version guards for fresh deploys

---

## Sprint 1: Driver Swap (`lib/pq` → `pgx/v5`) — 1-2 hours

### Tasks

1. **Replace dependency**
   ```bash
   cd /home/chaschel/Documents/go/bchat
   go get github.com/jackc/pgx/v5
   go get github.com/lib/pq@none
   go mod tidy
   ```

2. **Verify removal**
   ```bash
   grep "lib/pq" go.mod                    # must return nothing
   go list -m all | grep lib/pq            # must return nothing
   ```

3. **Replace `store/db/postgres/postgres.go` entirely** with pgx/v5 stdlib driver:
   - Import `_ "github.com/jackc/pgx/v5/stdlib"`
   - Use `sql.Open("pgx", profile.DSN)`
   - Add Neon connection pooling: `MaxOpenConns=10`, `MaxIdleConns=5`, `ConnMaxLifetime=5m`, `ConnMaxIdleTime=1m`
   - Add 60s ping timeout via `context.WithTimeout`
   - Keep nil profile check and error wrapping

4. **Verify compilation**
   ```bash
   go build ./store/db/postgres/...
   go build ./server/...
   ```

---

## Sprint 2: Complete Postgres Schema — 2-3 hours

### Tasks

1. **Identify missing tables** (34 total)
   ```bash
   grep "CREATE TABLE" store/migration/sqlite/LATEST.sql | sed 's/CREATE TABLE IF NOT EXISTS //;s/CREATE TABLE //;s/ (.*//;s/;//' | sort > /tmp/sqlite_tables.txt
   grep "CREATE TABLE" store/migration/postgres/LATEST.sql | sed 's/CREATE TABLE IF NOT EXISTS //;s/CREATE TABLE //;s/ (.*//;s/;//' | sort > /tmp/postgres_tables.txt
   comm -23 /tmp/sqlite_tables.txt /tmp/postgres_tables.txt
   ```

2. **Add missing tables to `store/migration/postgres/LATEST.sql`**
   - Append at end of file
   - Translate each table using column mapping below
   - Include all indexes and foreign key constraints
   - Do NOT modify existing 19 tables

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
   | `TIMESTAMP DEFAULT CURRENT_TIMESTAMP` | `TIMESTAMPTZ DEFAULT NOW()` |

   **Index translations:**
   | SQLite | Postgres |
   |--------|----------|
   | `CREATE INDEX IF NOT EXISTS` | Same |
   | `CREATE UNIQUE INDEX IF NOT EXISTS` | Same |
   | Partial indexes (`WHERE ...`) | Same |

3. **Verify**
   ```bash
   grep -c "^CREATE TABLE" store/migration/postgres/LATEST.sql   # should be 53
   grep -n "AUTOINCREMENT\|strftime\|PRAGMA" store/migration/postgres/LATEST.sql  # should return nothing
   ```

---

## Sprint 3: Implement Driver Stubs (91 methods) — 3-5 days

### Files to modify
- `store/db/postgres/agent.go` — 68 stubs
- `store/db/postgres/bridge.go` — 16 stubs
- `store/db/postgres/bridge_auth.go` — 7 stubs

### Files already complete (do NOT modify)
- `store/db/postgres/agent_observations.go`
- `store/db/postgres/agent_workflow.go`
- `store/db/postgres/memo_filter.go`
- `store/db/postgres/rbac.go`

### SQL Translation Rules
1. `?` → `$1`, `$2`, `$3`... (positional)
2. `strftime('%s', 'now')` → `EXTRACT(EPOCH FROM NOW())`
3. `PRAGMA table_info(...)` → `SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1`
4. `BEGIN IMMEDIATE` → `BEGIN` (use `SELECT FOR UPDATE` for explicit locking)
5. `ON CONFLICT(col) DO UPDATE SET` → same
6. `INSERT ... RETURNING id` → same
7. `1`/`0` literals → `TRUE`/`FALSE`
8. `DEFAULT 0` → `DEFAULT FALSE`, `DEFAULT 1` → `DEFAULT TRUE`
9. `INSERT OR IGNORE` → `INSERT ... ON CONFLICT DO NOTHING` (seed data only)
10. `INTEGER PRIMARY KEY` (implicit autoincrement) → `SERIAL PRIMARY KEY`

### Implementation Strategy
For each stub:
1. Find corresponding SQLite implementation in `store/db/sqlite/`
2. Copy logic structure
3. Translate SQL placeholders and syntax
4. Replace `errNotImplemented` / `ErrBridgeUnsupportedDatabase` with actual implementation
5. Verify compilation after each batch of ~10 methods

### Critical: Bridge Delivery
After implementing all 23 bridge stubs, update `SupportsBridgeDelivery()` to return `true`.

### Critical: Remove obsolete test
Delete `TestBridgeAuthPostgresUnsupported` from `store/test/bridge_auth_test.go` — it asserts bridge is unsupported, which will fail after implementation. Also remove unused `postgres` import from that test file.

### Verification
```bash
grep -rn "errNotImplemented" store/db/postgres/ --include="*.go" | grep -v "_test.go" | grep -v "var errNotImplemented"   # should return nothing
grep -rn "ErrBridgeUnsupportedDatabase" store/db/postgres/ --include="*.go" | grep -v "_test.go"                        # should return nothing
go build ./store/db/postgres/...
go vet ./store/db/postgres/...
```

---

## Sprint 4: Profile & Configuration — 30 min

### Step 1: Update `internal/profile/profile.go`

Add Postgres DSN handling in `Validate()` after the SQLite block (around line 92):

```go
if p.Driver == "postgres" && p.DSN == "" {
    p.DSN = os.Getenv("DATABASE_URL")
    if p.DSN == "" {
        return errors.New("postgres driver requires DSN or DATABASE_URL environment variable")
    }
}
```

Ensure `os` is imported.

### Step 2: Verify store initialization (DO NOT EDIT)

Check `store/db/db.go` already has:
```go
case "postgres":
    driver, err = postgres.NewDB(profile)
```

If present, skip. If missing, add it.

### Step 3: Update `.env.example`

Add at the end:
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

## Sprint 5: Testing — 2-3 hours

### Step 1: Remove obsolete test
- Delete `TestBridgeAuthPostgresUnsupported` from `store/test/bridge_auth_test.go`
- Remove unused `postgres` import from that file

### Step 2: Run test suite
```bash
go test ./store/db/postgres/... -v
go test ./store/... -v
```

### Step 3: Manual verification against Neon
```bash
export DB_DRIVER=postgres
export DATABASE_URL="postgresql://...?sslmode=require&channel_binding=require"
go run ./bin/memos --mode dev
curl http://localhost:5230/api/v1/status
```

---

## Sprint 6: Deploy — 1 hour

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
- Change existing 19 Postgres tables in `LATEST.sql`
- Skip `go list -m all | grep lib/pq` verification
- Implement stubs in `agent_observations.go`, `agent_workflow.go`, `memo_filter.go`, or `rbac.go`
- Use `DO $$` blocks in SQL migrations
- Merge into `postgres.go` — replace the entire file
- Create a new driver switch in `store/db/db.go` — it already exists
