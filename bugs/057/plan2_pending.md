# Plan v2: Addressing Pending Items (Incorporating Review Feedback)

**Date:** 2026-08-02  
**Source:** `bugs/057/plan_pending.md` + `bugs/057/plan_pending_review.md`  
**Context:** Updated plan after critical review — P1 requires verification spike before implementation

---

## Executive Summary

The review identified that **P1 (CockroachDB vector search) is still a hypothesis** — the proposed `ARRAY[$1]::VECTOR(1536)` fix has not been verified against actual CockroachDB + pgx behavior. The plan now inserts a **P0 spike** to prove parameter binding works before touching production code.

**Review verdict:** P2–P6 approved for immediate implementation. P1 blocked on P0 verification.

---

## Phase 0: P0 — Vector Binding Spike (NEW — Must Complete First)

### Goal
Answer exactly one question with empirical evidence:
> **Can pgx bind `[]float32` into CockroachDB VECTOR via `ARRAY[$1]::VECTOR(1536)`?**

### Spike Implementation
Create a minimal standalone program (`bugs/057/spike_vector_binding/main.go`):

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "os"

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
        if strings.Contains(dsn, "?") { sep = "&" }
        dsn += sep + "default_query_exec_mode=simple_protocol"
    }

    db, err := sql.Open("pgx", dsn)
    if err != nil { log.Fatal(err) }
    defer db.Close()

    ctx := context.Background()
    if err := db.PingContext(ctx); err != nil { log.Fatal(err) }

    // Test 1: Raw SQL vector literal (control)
    fmt.Println("=== Test 1: Raw SQL literal ===")
    var result1 string
    err = db.QueryRowContext(ctx, `SELECT ARRAY[0.1,0.2,0.3]::VECTOR(3)`).Scan(&result1)
    if err != nil {
        fmt.Printf("FAIL: %v\n", err)
    } else {
        fmt.Printf("OK: %s\n", result1)
    }

    // Test 2: Bound parameter with ARRAY[$1]::VECTOR
    fmt.Println("\n=== Test 2: Bound []float32 via ARRAY[$1]::VECTOR ===")
    embedding := []float32{0.1, 0.2, 0.3}
    var result2 string
    err = db.QueryRowContext(ctx, `SELECT ARRAY[$1]::VECTOR(3)`, embedding).Scan(&result2)
    if err != nil {
        fmt.Printf("FAIL: %v\n", err)
    } else {
        fmt.Printf("OK: %s\n", result2)
    }

    // Test 3: Bound parameter with $1::VECTOR (current broken approach)
    fmt.Println("\n=== Test 3: Bound []float32 via $1::VECTOR (current) ===")
    var result3 string
    err = db.QueryRowContext(ctx, `SELECT $1::VECTOR`, embedding).Scan(&result3)
    if err != nil {
        fmt.Printf("FAIL: %v\n", err)
    } else {
        fmt.Printf("OK: %s\n", result3)
    }

    // Test 4: JSON-marshaled binding (current broken approach)
    fmt.Println("\n=== Test 4: JSON-marshaled binding (current vectordb_cockroach.go) ===")
    embeddingJSON, _ := json.Marshal(embedding)
    var result4 string
    err = db.QueryRowContext(ctx, `SELECT $1::VECTOR`, string(embeddingJSON)).Scan(&result4)
    if err != nil {
        fmt.Printf("FAIL: %v\n", err)
    } else {
        fmt.Printf("OK: %s\n", result4)
    }

    // Test 5: Full search query pattern (if test 2 passes)
    fmt.Println("\n=== Test 5: Full search pattern (requires agent_vectors table) ===")
    // Only run if table exists
    rows, err := db.QueryContext(ctx, `
        SELECT 1 - (embedding <=> ARRAY[$1]::VECTOR(1536)) AS sim
        FROM agent_vectors LIMIT 1
    `, []float32{0.1, 0.2, 0.3 /* ... 1536 dims */})
    if err != nil {
        fmt.Printf("FAIL/skip: %v\n", err)
    } else {
        fmt.Println("OK: search pattern works")
        rows.Close()
    }
}
```

### Run Spike
```bash
cd bugs/057/spike_vector_binding
export COCKROACH_DSN="postgresql://root@localhost:26257/bchat?sslmode=disable"
go run main.go
```

### Decision Matrix

| Spike Outcome | Action |
|---------------|--------|
| **Test 2 PASS** | Proceed with Option A (`ARRAY[$1]::VECTOR`) in `vectordb_cockroach.go` |
| **Test 2 FAIL, Test 3 PASS** | Use Option A variant: `$1::VECTOR` with native binding |
| **Test 2 FAIL, Test 4 PASS** | Use Option B (JSON string formatting on Go side) |
| **All FAIL** | Investigate CockroachDB version / VECTOR type support; consider LanceDB fallback |

**Timebox:** 30 minutes max. If inconclusive, escalate.

---

## Phase 1: P1 — CockroachDB Vector Search Format (CONDITIONAL on P0)

### Updated Approach
**Do NOT modify `vectordb_cockroach.go` until P0 spike completes.**

After P0 verification, implement the proven approach:

#### If P0 Test 2 passes (Option A verified):
```go
// In Search() method:
sqlQuery := fmt.Sprintf(`
    SELECT id, title, content, content_type, metadata, source_version, created_at,
           1 - (embedding <=> ARRAY[$1]::VECTOR(1536)) AS similarity
    FROM agent_vectors
    WHERE tenant_id = $2 AND content_type IN (%s)
      AND (embedding <=> ARRAY[$1]::VECTOR(1536)) <= 1 - $4
    ORDER BY embedding <=> ARRAY[$1]::VECTOR(1536)
    LIMIT $3
`, contentTypeFilter)

rows, err := v.db.QueryContext(ctx, sqlQuery, queryEmbedding, query.TenantID, query.TopK, query.MinScore)
```

#### If P0 Test 2 fails, Test 3 passes:
```go
sqlQuery := fmt.Sprintf(`
    ...
    1 - (embedding <=> $1::VECTOR) AS similarity
    ...
`, contentTypeFilter)
rows, err := v.db.QueryContext(ctx, sqlQuery, queryEmbedding, ...)
```

#### If both fail (Option B):
```go
// Format as vector literal string on Go side
vecStr := formatVectorLiteral(queryEmbedding) // "[0.1,0.2,...]"
sqlQuery := fmt.Sprintf(`
    ...
    1 - (embedding <=> $1::VECTOR) AS similarity
    ...
`, contentTypeFilter)
rows, err := v.db.QueryContext(ctx, sqlQuery, vecStr, ...)
```

### Test Plan (Post-Implementation)
```bash
# 1. Build with cockroach tag
task build:backend:cockroach

# 2. Run verify-production step 6
BCHAT_URL=... BCHAT_USER=... BCHAT_PASS=... bash scripts/verify-production.sh
# Step 6 should now PASS
```

---

## Phase 2: P2 — NULL Scan Fix with Shared Helper (APPROVED)

### Review Finding (H2)
> "This is now duplicated logic. Instead of three implementations, I'd strongly consider `scanAllowedTenantIDs(...)` or equivalent."

### Implementation: Shared Helper Function

**New file:** `store/db/shared/user_scan.go`
```go
//go:build ignore
// This file is NOT compiled directly. It's a template for the shared logic.
// Each driver copies the scanAllowedTenantIDs function into its user_setting.go.
// OR: Use a build-tag-free shared package if preferred.

package shared

import (
    "database/sql"
    "encoding/json"
)

// scanAllowedTenantIDs scans a nullable JSON column into *[]string.
// Returns nil slice if column is NULL (meaning "all tenants").
func scanAllowedTenantIDs(scanner interface{ Scan(...interface{}) error }, dest *[]string, rawPtr **string) error {
    // Scan into *string first to handle NULL
    if err := scanner.Scan(rawPtr); err != nil {
        return err
    }
    if *rawPtr != nil {
        return json.Unmarshal([]byte(**rawPtr), dest)
    }
    // NULL → leave dest as nil (meaning "all tenants")
    return nil
}
```

**Actually simpler:** Just add a helper function in each driver file (no new package needed to avoid build complexity):

```go
// In each driver's user_setting.go, add:
func scanAllowedTenantIDs(row *sql.Row, user *User) error {
    var allowedTenantIDsRaw *string
    // ... other scans ...
    // Replace direct &user.AllowedTenantIDs with &allowedTenantIDsRaw
    // Then:
    if allowedTenantIDsRaw != nil {
        json.Unmarshal([]byte(*allowedTenantIDsRaw), &user.AllowedTenantIDs)
    }
    return nil
}
```

### Files to Update (3 drivers)
1. `store/db/sqlite/user_setting.go` — `FindUserByAccessToken`
2. `store/db/mysql/user_setting.go` — `FindUserByAccessToken`
3. `store/db/postgres/user_setting.go` — Already fixed; refactor to use helper for consistency

### Test Plan
```bash
# SQLite: INSERT user with NULL allowed_tenant_ids
sqlite3 build/data/memos_dev.db "INSERT INTO users (username, allowed_tenant_ids) VALUES ('test_null', NULL);"
# Call FindUserByAccessToken — verify no crash, AllowedTenantIDs is nil

# Run existing tests
go test -v ./store/db/sqlite/...
go test -v ./store/db/postgres/...
```

---

## Phase 3: P3 — Remove Debug Log Entirely (APPROVED)

### Review Finding (H3)
> "I'd remove it entirely, not downgrade to Debug. Credentials should never accidentally become loggable again."

### Fix
**File:** `server/router/api/v1/auth_service.go` ~line 510

**Remove this line entirely:**
```go
slog.Info("select-tenant debug", "token", accessTokenValue, "err", err, "user", matchedUser, "desc", description)
```

No replacement. Debug logging served its purpose during incident investigation.

---

## Phase 4: P4 — verify-production.sh Retry Logic (APPROVED + Enhanced)

### Review Finding (M1)
> "I'd also distinguish HTTP error / JSON parse error / 0 results. Those are operationally different. Right now they all collapse into '0'."

### Enhanced Fix
```bash
for i in $(seq 1 12); do
  # Capture HTTP status and body separately
  HTTP_CODE=$(curl -fsS -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H "Content-Type: application/json" \
    -d '{"query":"smoke test"}' \
    "$URL/api/v1/agent/$SLUG/rag/search" -o "$TMP_RESP" 2>/dev/null || echo "000")
  
  if [[ "$HTTP_CODE" -ge 400 ]]; then
    echo "  Attempt $i: HTTP $HTTP_CODE"
    cat "$TMP_RESP"
    sleep 5
    continue
  fi

  TOTAL=$(jq -r '.total_results // 0' "$TMP_RESP" 2>/dev/null || echo "parse_error")
  if [[ "$TOTAL" == "parse_error" ]]; then
    echo "  Attempt $i: JSON parse failed"
    cat "$TMP_RESP"
    sleep 5
    continue
  fi

  if [[ "$TOTAL" -gt 0 ]]; then
    echo "  Attempt $i: SUCCESS (total_results=$TOTAL)"
    break
  fi

  echo "  Attempt $i: 0 results (total_results=0)"
  sleep 5
done

[[ "$TOTAL" -gt 0 ]] || fail "RAG search returned no hits after 12 attempts (last: HTTP $HTTP_CODE, total=$TOTAL)"
```

### Files to Update
- `scripts/verify-production.sh` — lines 92-102

---

## Phase 5: P6 — --keep Flag Convention (APPROVED)

### Review Finding (M2)
> "Tiny UX improvement. Good candidate for this PR."

### Fix
```bash
# Replace lines 18-19:
KEEP=0
for arg in "$@"; do
  case $arg in
    --keep) KEEP=1 ;;
    --keep=*) KEEP="${arg#*=}" ;;
  esac
done
```

---

## Phase 6: P5 — isLocalDSN Replacement (DEFERRED)

### Review Finding (M3)
> "The plan correctly defers it. I agree with that decision. The current explicit environment-variable gates are doing most of the real safety work anyway."

**Status:** Deferred to next iteration. Document as known limitation.

---

## Updated Implementation Order

| Phase | Item | File(s) | Status | Est. Effort |
|-------|------|---------|--------|-------------|
| **0** | **P0: Vector binding spike** | `bugs/057/spike_vector_binding/main.go` | **REQUIRED FIRST** | 30 min |
| **1** | **P1: Vector search fix** | `vectordb_cockroach.go` | Conditional on P0 | 20 min |
| **2** | **P2: NULL scan + shared helper** | `sqlite/user_setting.go`, `mysql/user_setting.go`, `postgres/user_setting.go` | Ready | 30 min |
| **3** | **P3: Remove debug log** | `auth_service.go` | Ready | 5 min |
| **4** | **P4: verify-production retry logic** | `verify-production.sh` | Ready | 15 min |
| **5** | **P6: --keep flag** | `verify-production.sh` | Ready | 5 min |
| **6** | **P5: isLocalDSN** | (deferred) | Deferred | — |

---

## Validation Checklist (Post-P0/P1)

```bash
# 1. Spike verification
cd bugs/057/spike_vector_binding
export COCKROACH_DSN="postgresql://root@localhost:26257/bchat?sslmode=disable"
go run main.go
# Verify Test 2 passes (or document which test passes)

# 2. Build with CockroachDB tag
task build:backend:cockroach

# 3. Run E2E tests
export BCHAT_ALLOW_DB_RESET=1
export BCHAT_ALLOW_REMOTE_DB_RESET=1
go test -v -tags=cockroach -run TestCockroach ./store/test/...

# 4. Run verify-production (full)
export BCHAT_URL=http://localhost:5230
export BCHAT_USER=admin
export BCHAT_PASS=...
bash scripts/verify-production.sh

# Expected: All 7 steps PASS
```

---

## Notes

- **P0 is now the critical path** — no production code changes for P1 until spike proves the approach
- **P2 refactored** to use shared helper pattern per review (H2)
- **P3 strengthened** — complete removal per review (H3)
- **P4 enhanced** — distinguishes error types per review (M1)
- **P5 correctly deferred** — environment gates provide sufficient safety (M3)
- **Review score: 9.8/10** — only P1 hypothesis was the gap; now addressed with P0

---

## References

| Artifact | Path |
|----------|------|
| Original plan | `bugs/057/plan_pending.md` |
| Review | `bugs/057/plan_pending_review.md` |
| Session log | `bugs/057/session-057.md` |
| CockroachDB vector store | `server/router/api/v1/agent/vectordb_cockroach.go` |
| PostgreSQL NULL scan fix | `store/db/postgres/user_setting.go` |
| Verify script | `scripts/verify-production.sh` |