# Plan v3: Addressing Pending Items (Final — Incorporating All Review Feedback)

**Date:** 2026-08-02  
**Source:** `plan_pending.md` → `plan2_pending.md` → `plan2_pending_review.md`  
**Status:** Ready to execute

---

## Executive Summary

All review feedback incorporated. The plan now follows a disciplined evidence-first workflow:

```
Observation → Evidence → Hypothesis → Spike → Implementation → Validation
```

**P0 (spike) is the gate** — no production code changes until spike proves the binding approach.

---

## Phase 0: P0 — Vector Binding Spike (Minimal, Single-Question)

### Goal
Answer **one question** with empirical evidence:
> **Can pgx bind `[]float32` into CockroachDB VECTOR via `ARRAY[$1]::VECTOR`?**

### Spike Implementation
**Location:** `bugs/057/spike_vector_binding/main.go` *(temporary — delete after use)*

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "os"
    "strings"

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

    // Test 1: Control — raw SQL vector literal
    fmt.Println("=== Test 1: Raw SQL literal (control) ===")
    var result1 string
    err = db.QueryRowContext(ctx, `SELECT ARRAY[0.1,0.2]::VECTOR(2)`).Scan(&result1)
    if err != nil {
        log.Printf("FAIL: %v", err)
    } else {
        log.Printf("OK: %s", result1)
    }

    // Test 2: THE QUESTION — bound parameter via ARRAY[$1]::VECTOR
    fmt.Println("\n=== Test 2: Bound []float32 via ARRAY[$1]::VECTOR ===")
    embedding := []float32{0.1, 0.2}
    var result2 string
    err = db.QueryRowContext(ctx, `SELECT ARRAY[$1]::VECTOR(2)`, embedding).Scan(&result2)
    if err != nil {
        log.Printf("FAIL: %v", err)
    } else {
        log.Printf("OK: %s", result2)
    }

    // Test 3: Fallback candidate — bound parameter via $1::VECTOR (no ARRAY wrapper)
    fmt.Println("\n=== Test 3: Bound []float32 via $1::VECTOR (fallback candidate) ===")
    var result3 string
    err = db.QueryRowContext(ctx, `SELECT $1::VECTOR`, embedding).Scan(&result3)
    if err != nil {
        log.Printf("FAIL: %v", err)
    } else {
        log.Printf("OK: %s", result3)
    }
}
```

### Run Spike
```bash
cd bugs/057/spike_vector_binding
export COCKROACH_DSN="postgresql://root@localhost:26257/bchat?sslmode=disable"
go run main.go
```

### Spike Success Criteria (Objective)

| Criterion | Required |
|-----------|----------|
| ✓ Test 1 passes (control) | Yes |
| ✓ Test 2 passes (`ARRAY[$1]::VECTOR` accepted) | **Primary success** |
| ✓ Test 2 returns same VECTOR type as Test 1 | Yes |
| ✓ No pgx serialization error | Yes |
| If Test 2 fails: Test 3 passes (`$1::VECTOR` works) | Fallback candidate |

### Timebox & Escalation
```
30 minutes
    ↓
No conclusion / all fail
    ↓
STOP implementation
    ↓
Open investigation (file GitHub issue, check CRDB version, consider LanceDB fallback)
```
**Do not keep trying.** Escalate explicitly.

---

## Phase 1: P1 — CockroachDB Vector Search (CONDITIONAL on P0)

**Do NOT modify `vectordb_cockroach.go` until P0 spike completes.**

### Implementation After Spike

| Spike Outcome | Implementation |
|---------------|----------------|
| **Test 2 PASS** | Use `ARRAY[$1]::VECTOR(1536)` with direct `[]float32` binding |
| **Test 2 FAIL, Test 3 PASS** | Use `$1::VECTOR` with direct `[]float32` binding (candidate) |
| **Both FAIL** | Format vector literal string on Go side: `fmt.Sprintf("[%s]", ...)` |

### Code Change (if Test 2 passes)
```go
// In Search() method — REMOVE json.Marshal, pass queryEmbedding directly
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

---

## Phase 2: P2 — NULL Scan Fix (Three Drivers, No Shared Package)

### Review Finding (H1)
> "For three drivers, just duplicate the 15-line helper. That's okay. Don't introduce a fake abstraction."

### Implementation: Duplicate Helper in Each Driver

**Pattern (add to each `user_setting.go`):**
```go
// scanAllowedTenantIDs scans a nullable JSON column into *[]string.
// NULL → leaves dest as nil (meaning "all tenants").
func scanAllowedTenantIDs(row *sql.Row, dest *[]string, otherDest ...interface{}) error {
    var allowedTenantIDsRaw *string
    // Scan all columns including the raw JSON string
    args := append([]interface{}{&allowedTenantIDsRaw}, otherDest...)
    if err := row.Scan(args...); err != nil {
        return err
    }
    if allowedTenantIDsRaw != nil {
        return json.Unmarshal([]byte(*allowedTenantIDsRaw), dest)
    }
    return nil
}
```

### Files to Update
1. `store/db/sqlite/user_setting.go` — `FindUserByAccessToken`
2. `store/db/mysql/user_setting.go` — `FindUserByAccessToken`
3. `store/db/postgres/user_setting.go` — Refactor existing fix to use helper

### Test Plan
```bash
# SQLite: INSERT user with NULL allowed_tenant_ids
sqlite3 build/data/memos_dev.db "INSERT INTO users (username, allowed_tenant_ids) VALUES ('test_null', NULL);"
# Call FindUserByAccessToken — verify no crash, AllowedTenantIDs is nil

go test -v ./store/db/sqlite/...
go test -v ./store/db/postgres/...
```

---

## Phase 3: P3 — Remove Debug Log Entirely

### Review Finding (H3)
> "Remove it entirely, not downgrade to Debug. Credentials should never accidentally become loggable again."

### Fix
**File:** `server/router/api/v1/auth_service.go` ~line 510

**Delete this line:**
```go
slog.Info("select-tenant debug", "token", accessTokenValue, "err", err, "user", matchedUser, "desc", description)
```

---

## Phase 4: P4 — verify-production.sh Retry Logic (Enhanced)

### Review Finding (M1, H3)
- Distinguish HTTP error / JSON parse error / 0 results
- Different exit codes for CI interpretability

### Enhanced Fix
```bash
# Add at top after TMP_KB:
TMP_RESP=$(mktemp)
trap 'rm -f "$COOKIE_JAR" "$TMP_KB" "$TMP_RESP"' EXIT

# Replace lines 92-102:
EXIT_CODE=0
for i in $(seq 1 12); do
  HTTP_CODE=$(curl -fsS -w "%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H "Content-Type: application/json" \
    -d '{"query":"smoke test"}' \
    "$URL/api/v1/agent/$SLUG/rag/search" -o "$TMP_RESP" 2>/dev/null || echo "000")

  if [[ "$HTTP_CODE" -ge 400 ]]; then
    echo "  Attempt $i: HTTP $HTTP_CODE"
    cat "$TMP_RESP"
    EXIT_CODE=1
    sleep 5
    continue
  fi

  TOTAL=$(jq -r '.total_results // 0' "$TMP_RESP" 2>/dev/null || echo "parse_error")
  if [[ "$TOTAL" == "parse_error" ]]; then
    echo "  Attempt $i: JSON parse failed"
    cat "$TMP_RESP"
    EXIT_CODE=2
    sleep 5
    continue
  fi

  if [[ "$TOTAL" -gt 0 ]]; then
    echo "  Attempt $i: SUCCESS (total_results=$TOTAL)"
    EXIT_CODE=0
    break
  fi

  echo "  Attempt $i: 0 results (total_results=0)"
  EXIT_CODE=3
  sleep 5
done

[[ "$EXIT_CODE" -eq 0 ]] || fail "RAG search failed after 12 attempts (exit=$EXIT_CODE: 1=HTTP, 2=JSON, 3=0 results)"
```

---

## Phase 5: P6 — --keep Flag Convention

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

## Phase 6: P5 — isLocalDSN (DEFERRED)

**Status:** Deferred. Environment gates (`BCHAT_ALLOW_DB_RESET`, `BCHAT_ALLOW_REMOTE_DB_RESET`) provide sufficient safety.

---

## Validation Order (Per Review H2)

```
P0 Spike
    ↓
Build (task build:backend:cockroach)
    ↓
Unit/Integration Test (NEW) — verify Search() works against real CRDB
    ↓
E2E Tests (go test -tags=cockroach ./store/test/...)
    ↓
verify-production.sh (full stack)
```

### Unit/Integration Test (Insert Between Build and E2E)
```bash
# Quick search test against local CockroachDB
cd bugs/057/spike_vector_binding
# Reuse spike but test the actual Search() method via a minimal integration
# Or: add a test in store/test/cockroach_search_test.go
```

---

## Implementation Order Summary

| Phase | Item | File(s) | Status | Est. Effort |
|-------|------|---------|--------|-------------|
| **0** | **P0: Spike (minimal)** | `bugs/057/spike_vector_binding/main.go` | **REQUIRED FIRST** | 20 min |
| **1** | **P1: Vector search fix** | `vectordb_cockroach.go` | Conditional on P0 | 15 min |
| **2** | **P2: NULL scan (3 drivers)** | `sqlite/`, `mysql/`, `postgres/user_setting.go` | Ready | 25 min |
| **3** | **P3: Remove debug log** | `auth_service.go` | Ready | 5 min |
| **4** | **P4: verify-production retry** | `verify-production.sh` | Ready | 15 min |
| **5** | **P6: --keep flag** | `verify-production.sh` | Ready | 5 min |
| **6** | **P5: isLocalDSN** | (deferred) | Deferred | — |

---

## Validation Checklist (Post-P0/P1)

```bash
# 1. Spike verification
cd bugs/057/spike_vector_binding
export COCKROACH_DSN="postgresql://root@localhost:26257/bchat?sslmode=disable"
go run main.go
# Verify: Test 1 OK, Test 2 OK (or Test 3 as fallback)

# 2. Build with CockroachDB tag
task build:backend:cockroach

# 3. Unit/Integration test (NEW)
go test -v -tags=cockroach -run TestCockroachSearch ./store/test/...

# 4. E2E tests
export BCHAT_ALLOW_DB_RESET=1
export BCHAT_ALLOW_REMOTE_DB_RESET=1
go test -v -tags=cockroach -run TestCockroach ./store/test/...

# 5. verify-production (full)
export BCHAT_URL=http://localhost:5230
export BCHAT_USER=admin
export BCHAT_PASS=...
bash scripts/verify-production.sh

# Expected: All 7 steps PASS
```

---

## Cleanup (Post-Implementation)

```bash
# Remove spike program (temporary)
rm -rf bugs/057/spike_vector_binding
```

---

## References

| Artifact | Path |
|----------|------|
| Session log | `bugs/057/session-057.md` |
| Original plan | `bugs/057/plan_pending.md` |
| Plan v2 | `bugs/057/plan2_pending.md` |
| Review v2 | `bugs/057/plan2_pending_review.md` |
| CockroachDB vector store | `server/router/api/v1/agent/vectordb_cockroach.go` |
| PostgreSQL NULL scan fix | `store/db/postgres/user_setting.go` |
| Verify script | `scripts/verify-production.sh` |

---

**Plan Status:** Ready to execute. P0 spike is the only blocker for P1. P2–P6 can proceed in parallel after P0.