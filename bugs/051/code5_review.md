# Code5 Review: Internal Notes + RAG-Based Bug Inference (Final)

**Reviewer:** Senior Go Architect
**Date:** 2026-07-30
**Code:** `code5.md` (Bug 051 — Final)
**Verdict:** **Approved** — All prior issues resolved. code5.md accurately reflects source code. No nits.

---

## Changelog Verification

| # | Finding | Status | Evidence |
|---|---------|--------|----------|
| 1 | Both drivers use quoted `"user"` identifier | ✅ | `cmd/import-bugs/main.go:172-174` — both paths use `FROM "user"` |
| 2 | File line count ~490 | ✅ | `wc -l`: 490 lines |
| 3 | Creates system bot user if user table empty | ✅ | `main.go:177-190` — INSERT system_bot with ADMIN role |

---

## Hackathon Demo Guide

### Prerequisites

```bash
# Required: OpenRouter API key
export OPENROUTER_API_KEY=sk-or-v1-xxx

# Optional: CockroachDB (skip for SQLite local demo)
export COCKROACH_DSN="postgresql://user:pass@host/db?sslmode=require"
```

### Step 1: Build Everything

```bash
go build ./bin/memos/main.go
go build ./cmd/import-bugs/
go test ./store/... -count=1
go test ./server/router/api/v1/agent/... -count=1
task validate:schema
task validate:parity
```

### Step 2: Start the Server

```bash
task run
# Server starts at http://localhost:5230
# Migration (internal_notes column) runs automatically
```

### Step 3: Run the Import Pipeline

```bash
# SQLite (local development — default path: build/data/memos_dev.db)
go run ./cmd/import-bugs/

# Expected output:
# === Bug Import Script ===
# Importing bugs/001-050 as tickets with internal_notes
# Connecting to SQLite: build/data/memos_dev.db
# Connected successfully!
# Created system bot user with ID: 1
# Using tenant ID: 1
# Using creator user ID: 1
# Found 50 bug folders
# Created: 130 tickets
# Skipped: 0 (already exist)

# Second run (verify idempotency):
go run ./cmd/import-bugs/
# Expected: Created: 0, Skipped: ~130
```

### Step 4: Verify Tickets Created

```bash
sqlite3 build/data/memos_dev.db "
SELECT id, title, substr(internal_notes, 1, 60)
FROM tickets
WHERE type = 'BUG'
LIMIT 10;
"
```

Expected output shows tickets with internal_notes populated (e.g., `Bug #001 - 8 files across 3 phases`).

### Step 5: Test RBAC (Internal Notes Visibility)

```bash
# 1. Create a ticket as a regular user (no ticket:internal_notes permission)
curl -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{
    "title":"Test — customer ticket",
    "description":"/m/test-description",
    "status":"OPEN",
    "priority":"MEDIUM",
    "type":"TASK"
  }'
# Response: internalNotes will be "" (filtered by RBAC)

# 2. View the same ticket as admin/superuser
# internalNotes will contain the auto-generated suggestion
```

### Step 6: Test Resolution Inference

```bash
# Create a new ticket — the goroutine triggers InferResolutionForNewTicket
curl -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{
    "title":"Water damage claim — urgent extraction needed",
    "description":"/m/water-damage-claim",
    "status":"OPEN",
    "priority":"HIGH",
    "type":"BUG"
  }'

# Verify auto-populated internal_notes:
sqlite3 build/data/memos_dev.db "
SELECT internal_notes FROM tickets
WHERE title = 'Water damage claim — urgent extraction needed'
AND tenant_id = 1;
"
```

Expected: `internal_notes` contains "## Suggested Resolution (Auto-generated)" with matches from similar imported bug tickets.

### Step 7: Test Cross-Tenant Isolation

Create a ticket in a second tenant — verify that `InferResolutionForNewTicket` only returns results from the same tenant.

---

## Invariants (All Pass)

| Invariant | Description | Status |
|-----------|-------------|--------|
| `INV_TICKET_INTERNAL_NOTES_RBAC` | Hidden from unauthorized users | ✅ |
| `INV_TICKET_INTERNAL_NOTES_PERSISTENCE` | Survives read/write cycles | ✅ |
| `INV_VECTOR_SEARCH_TENANT_ISOLATION` | No cross-tenant data leak | ✅ |
| `INV_IMPORT_IDEMPOTENCY` | No duplicate tickets on re-run | ✅ |
| `INV_RESOLUTION_INFERENCE_GRACEFUL_DEGRADATION` | Works when CockroachDB unavailable | ✅ |
| `INV_IMPORT_USER_RESOLUTION` | Valid creator_id via getOrCreateUser | ✅ |

---

## Demo Script (Quick Walkthrough)

```bash
# Terminal 1 — Start server
task run

# Terminal 2 — Import bugs
go run ./cmd/import-bugs/

# Terminal 2 — Create a test ticket (triggers inference)
curl -s -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"New bug","description":"/m/new","status":"OPEN","priority":"MEDIUM","type":"BUG"}' \
  | python3 -m json.tool

# Terminal 2 — Check the auto-generated resolution
sqlite3 build/data/memos_dev.db "SELECT internal_notes FROM tickets ORDER BY id DESC LIMIT 1;"
```
