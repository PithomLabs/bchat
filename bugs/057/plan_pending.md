# Plan: Addressing Pending Items from session-057 / plan3_e2e

**Date:** 2026-08-02  
**Context:** E2E safety hardening + CockroachDB reference deployment close-out  
**Source:** `bugs/057/pending_sf.md`

---

## Executive Summary

This plan addresses the **6 pending items** from the session-057 close-out log, prioritized by severity and blocking status. The primary blocker is the CockroachDB vector search format issue (§2.1) which prevents `verify-production.sh` step 6 from passing.

---

## Priority 1: P1 — CockroachDB Vector Search Format (BLOCKING)

### Issue
**Location:** `server/router/api/v1/agent/vectordb_cockroach.go:Search()` ~line 323  
**Error:** `ERROR: could not parse vector: malformed vector literal: Vector contents must start with "[" and end with "]"`  
**Root Cause:** The search query passes the embedding as `$1::VECTOR` but pgx serializes `[]float32` (via JSON marshaling) in a format CockroachDB's `<=>` operator doesn't accept.

### Current Code (lines 305-332)
```go
embeddingJSON, err := json.Marshal(queryEmbedding)
// ...
sqlQuery := fmt.Sprintf(`
    SELECT ... 1 - (embedding <=> $1::VECTOR) AS similarity
    FROM agent_vectors
    WHERE tenant_id = $2 AND content_type IN (%s)
      AND (embedding <=> $1::VECTOR) <= 1 - $4
    ORDER BY embedding <=> $1::VECTOR
    LIMIT $3
`, contentTypeFilter)

rows, err := v.db.QueryContext(ctx, sqlQuery, embeddingJSON, query.TenantID, query.TopK, query.MinScore)
```

### Root Cause Analysis
1. `json.Marshal([]float32{0.1, 0.2})` → `"[0.1,0.2]"` (JSON string)
2. pgx sends this as a **text parameter** (quoted string literal)
3. CockroachDB sees `'[0.1,0.2]'::VECTOR` — the quotes make it a string, not a vector literal
4. CockroachDB's VECTOR parser expects **unquoted** `[0.1,0.2]` or `ARRAY[0.1,0.2]::VECTOR`

### Fix Options (in order of preference)

| Option | Approach | Pros | Cons |
|--------|----------|------|------|
| **A (Recommended)** | Use `ARRAY[$1]::VECTOR(1536)` with `[]float32` parameter directly (no JSON marshal) | Native pgx array binding; clean SQL; no string formatting | Requires verifying pgx sends `float4[]` correctly |
| **B** | Format vector as string literal on Go side: `fmt.Sprintf("[%s]", ...)` | Explicit control over format | Fragile; manual string manipulation |
| **C** | Use CockroachDB `vec($1)` constructor (if available) | Native CRDB function | Version-dependent; need to check availability |

### Recommended Fix (Option A)
Modify the Search method to:
1. **Remove** `json.Marshal` — pass `queryEmbedding` (type `[]float32`) directly as parameter
2. **Change** SQL cast from `$1::VECTOR` → `ARRAY[$1]::VECTOR(1536)`
3. **Verify** pgx sends `float4[]` (PostgreSQL array of float4) which CockroachDB accepts for VECTOR cast

```go
// Instead of:
embeddingJSON, err := json.Marshal(queryEmbedding)
// ...
sqlQuery := fmt.Sprintf(`... embedding <=> $1::VECTOR ...`, contentTypeFilter)
rows, err := v.db.QueryContext(ctx, sqlQuery, embeddingJSON, ...)

// Do:
sqlQuery := fmt.Sprintf(`... embedding <=> ARRAY[$1]::VECTOR(1536) ...`, contentTypeFilter)
rows, err := v.db.QueryContext(ctx, sqlQuery, queryEmbedding, query.TenantID, query.TopK, query.MinScore)
```

**Note:** The `ARRAY[$1]` syntax wraps the parameter as a PostgreSQL array. Since `queryEmbedding` is `[]float32`, pgx will bind it as `float4[]` (OID 1021). CockroachDB's `VECTOR` type accepts `float4[]` input via `ARRAY[...]::VECTOR(dim)`.

### Test Plan
```bash
# 1. Unit test: verify parameter binding produces correct SQL
# 2. Integration: run against local CockroachDB
cockroach sql --url "$COCKROACH_DSN" -e "
SELECT 1 - (embedding <=> ARRAY[0.1,0.2,...]::VECTOR(1536)) AS similarity
FROM agent_vectors LIMIT 1;
"

# 3. E2E: verify-production.sh step 6 passes
BCHAT_URL=... BCHAT_USER=... BCHAT_PASS=... bash scripts/verify-production.sh
```

---

## Priority 2: P2 — NULL Scan Fix for SQLite & MySQL (Consistency)

### Issue
**Location:** `store/db/sqlite/user_setting.go`, `store/db/mysql/user_setting.go`  
**Root Cause:** Same direct scan into `&user.AllowedTenantIDs` (`*[]string`) that caused CockroachDB crash. SQLite/MySQL drivers may coerce NULL → empty slice (masking bug), but fix is needed for consistency and future-proofing.

### Current Pattern (all three drivers)
```go
&user.Description, &user.AllowedTenantIDs, &description,
```

### Fix (Mirror PostgreSQL fix in `store/db/postgres/user_setting.go`)
```go
// Scan into *string first
var allowedTenantIDsRaw *string
err := row.Scan(..., &allowedTenantIDsRaw, ...)

// Then unmarshal manually
if allowedTenantIDsRaw != nil {
    json.Unmarshal([]byte(*allowedTenantIDsRaw), &user.AllowedTenantIDs)
}
// nil → user.AllowedTenantIDs remains nil (meaning "all tenants")
```

### Files to Update
1. `store/db/sqlite/user_setting.go` — `FindUserByAccessToken`
2. `store/db/mysql/user_setting.go` — `FindUserByAccessToken`

### Test Plan
```bash
# 1. SQLite: INSERT user with NULL allowed_tenant_ids, then call FindUserByAccessToken
# 2. MySQL: Same test (requires MySQL instance)
# 3. Verify no crash, slice remains nil
```

---

## Priority 3: P3 — Debug Log Removal in HandleSelectTenant

### Issue
**Location:** `server/router/api/v1/auth_service.go` ~line 510  
**Problem:** `slog.Info` logs full `selection:<token>` value on every request — security smell (short-lived but still a credential).

### Current Code
```go
slog.Info("select-tenant debug", "token", accessTokenValue, "err", err, "user", matchedUser, "desc", description)
```

### Fix
**Remove entirely** — debug logging served its purpose during incident investigation. If needed in future, re-add as `slog.Debug(...)`.

---

## Priority 4: P4 — verify-production.sh Retry Logic Improvement

### Issue
**Location:** `scripts/verify-production.sh` lines 92-100  
**Current Behavior:** Retries 12× on empty results (`"hits"` or `"items"` key presence check), doesn't distinguish "no results" from "error".

### Current Code
```bash
for i in $(seq 1 12); do
  HITS=$(curl ... || echo "")
  N=$(echo "$HITS" | grep -o '"hits"\|"items"' | head -1 | wc -l)
  [[ "$N" -ge 1 ]] && break
  sleep 5
done
[[ "$N" -ge 1 ]] || fail "RAG search returned no hits"
```

### Fix
Parse `total_results` from JSON response:
```bash
for i in $(seq 1 12); do
  HITS=$(curl -fsS ... 2>/dev/null || echo "")
  TOTAL=$(echo "$HITS" | jq -r '.total_results // 0' 2>/dev/null || echo "0")
  if [[ "$TOTAL" -gt 0 ]]; then
    break
  fi
  sleep 5
done
[[ "$TOTAL" -gt 0 ]] || fail "RAG search returned no hits after 12 attempts"
```

**Requires:** `jq` installed (already available in Fly.io build containers).

---

## Priority 5: P5 — isLocalDSN Heuristic Replacement (Future Enhancement)

### Issue
**Location:** Wherever `isLocalDSN` is used (likely `scripts/verify-production.sh` or Taskfile)  
**Current:** Substring match on `localhost` / `127.0.0.1`  
**Problem:** Fragile — breaks with hostnames like `cockroach.local`, SSH tunnels, etc.

### Recommended Fix (Post-Hackathon)
Replace with proper URL parsing:
```go
func isLocalDSN(dsn string) bool {
    u, err := url.Parse(dsn)
    if err != nil { return false }
    host := u.Hostname()
    return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
```

**Deferred:** Current heuristic works for typical localhost setups. Document as known limitation.

---

## Priority 6: P6 — verify-production.sh --keep Flag Convention

### Issue
**Current:** `--keep` requires `--keep=1` (non-standard)  
**Expected:** `--keep` alone should set `KEEP=1`

### Fix
```bash
KEEP=0
for arg in "$@"; do
  case $arg in
    --keep) KEEP=1 ;;
    --keep=*) KEEP="${arg#*=}" ;;
  esac
done
```

**Priority:** Low — UX only, not blocking.

---

## Implementation Order

| Phase | Item | File(s) | Est. Effort |
|-------|------|---------|-------------|
| **1** | P1: CockroachDB vector search format | `vectordb_cockroach.go` | 30 min |
| **2** | P2: SQLite/MySQL NULL scan fix | `sqlite/user_setting.go`, `mysql/user_setting.go` | 20 min |
| **3** | P3: Remove debug log | `auth_service.go` | 5 min |
| **4** | P4: verify-production.sh retry logic | `verify-production.sh` | 10 min |
| **5** | P6: --keep flag convention | `verify-production.sh` | 5 min |
| **6** | P5: isLocalDSN replacement | (wherever used) | 15 min (deferred) |

---

## Validation Checklist

After implementing Phase 1-4, run:
```bash
# 1. Build with CockroachDB tag
task build:backend:cockroach

# 2. Run E2E tests locally (requires local CockroachDB)
export COCKROACH_DSN="postgresql://root@localhost:26257/bchat?sslmode=disable"
export BCHAT_ALLOW_DB_RESET=1
export BCHAT_ALLOW_REMOTE_DB_RESET=1
go test -v -tags=cockroach -run TestCockroach ./store/test/...

# 3. Run verify-production against local instance
export BCHAT_URL=http://localhost:5230
export BCHAT_USER=admin
export BCHAT_PASS=...
bash scripts/verify-production.sh

# 4. All steps 1-7 should PASS
```

---

## Notes

- **P1 is the only blocker** for `verify-production.sh` completion
- P2/P3 are code hygiene/consistency — fix alongside P1 to avoid regression
- P4/P6 improve script reliability — low risk, high value
- P5 is architectural — defer to next iteration with proper URL parsing library

---

## References

| Artifact | Path |
|----------|------|
| Session log | `bugs/057/session-057.md` |
| Plan 3 (E2E) | `bugs/057/plan3_e2e.md` |
| Review of Plan 3 | `bugs/057/plan3_e2e_review.md` |
| CockroachDB vector store | `server/router/api/v1/agent/vectordb_cockroach.go` |
| PostgreSQL NULL scan fix | `store/db/postgres/user_setting.go` |
| Verify script | `scripts/verify-production.sh` |