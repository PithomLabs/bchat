# Plan: bugs/057 Pending Items — Verification & Remaining Fixes

**Date:** 2026-08-02  
**Context:** Verify which pending_sf.md items are already addressed in the current HEAD (commit `2910362`) and plan the remaining fix.

---

## 1. Status of Each Pending Item

### ✅ Already Fixed (confirmed in HEAD)

| Item | File | Evidence |
|------|------|----------|
| §1.1 — `IssuedAt` nil panic | `server/router/api/v1/user_service.go:579-586` | Nil guard before `.Unix()` — confirmed in source |
| §1.2 — NULL scan (PostgreSQL) | `store/db/postgres/user_setting.go:108-120` | Scans into `*string`, `json.Unmarshal`, nil-safe |
| §2.2 — NULL scan (SQLite) | `store/db/sqlite/user_setting.go:111-123` | Same `*string` + unmarshal pattern |
| §2.2 — NULL scan (MySQL) | `store/db/mysql/user_setting.go:91-103` | Same `*string` + unmarshal pattern |
| §2.3 — Debug `slog.Info` token leak | `server/router/api/v1/auth_service.go` | No `slog.Info("select-tenant debug"...)` found |
| §2.5 — Verify script retry loop | `scripts/verify-production.sh:113-133` | Parses `total_results`, breaks on `> 0` |

### 🔴 Still Pending (not yet resolved)

| Item | Severity | File | Description |
|------|----------|------|-------------|
| §2.1 — CRDB vector search format | **P1** | `server/router/api/v1/agent/vectordb_cockroach.go:318-329` | `$1::VECTOR` with string parameter still fails; CockroachDB requires vector literal in query text |

### ℹ️ Accepted / Deferred (no code change required)

| Item | Reason |
|------|--------|
| §1.4 / §2.4 — `isLocalDSN` heuristic | Documented in `plan3_e2e.md` as accepted behavior; not a blocker |
| §2.6 — `--keep` flag | UX cosmetic; `--keep` and `--keep=1` both work correctly |

---

## 2. Root Cause — CockroachDB Vector Search (P1)

**Symptom:**  
```
Search failed: ERROR: could not parse vector: malformed vector literal:
Vector contents must start with "[" and end with "]"
```

**Location:** `vectordb_cockroach.go` `Search()` method, lines 318–329.

**Root cause:**  
`formatVectorLiteral()` produces a Go string `"[0.1,0.2,...]"`. This string is passed as `$1` parameter with the SQL cast `$1::VECTOR`. With `default_query_exec_mode=simple_protocol`, pgx sends `$1` as a TEXT parameter. CockroachDB's `<=>` operator requires the vector to appear as a literal in the query text (the bracketed form must be lexed by the SQL parser, not cast from TEXT).

**Why the previous fix (bug 057) is insufficient:**  
The bug 057 commit changed from `json.Marshal` (which produced `[0.1,0.2,...]` as JSON bytes cast to `::VECTOR`) to `formatVectorLiteral` (which produces the same bracketed string but as a TEXT parameter). The underlying problem — parameter binding a string as `::VECTOR` — is unchanged.

---

## 3. Fix Plan — vectordb_cockroach.go Search

**File:** `server/router/api/v1/agent/vectordb_cockroach.go`

**Change:** Replace the `$1::VECTOR` parameter with the vector literal embedded directly in the SQL text. The `formatVectorLiteral` output contains only numeric characters, commas, dots, and brackets — no user-supplied content — so direct interpolation is safe (no SQL injection risk).

```go
// Current (broken):
sqlQuery := fmt.Sprintf(`
    SELECT ..., 1 - (embedding <=> $1::VECTOR) AS similarity
    FROM agent_vectors
    WHERE tenant_id = $2 AND content_type IN (%s)
      AND (embedding <=> $1::VECTOR) <= 1 - $4
    ORDER BY embedding <=> $1::VECTOR
    LIMIT $3
`, contentTypeFilter)
rows, err := v.db.QueryContext(ctx, sqlQuery, vecStr, query.TenantID, query.TopK, query.MinScore)

// Fixed:
sqlQuery := fmt.Sprintf(`
    SELECT ..., 1 - (embedding <=> %s::VECTOR) AS similarity
    FROM agent_vectors
    WHERE tenant_id = $1 AND content_type IN (%s)
      AND (embedding <=> %s::VECTOR) <= $3
    ORDER BY embedding <=> %s::VECTOR
    LIMIT $2
`, vecStr, contentTypeFilter, vecStr, vecStr)
rows, err := v.db.QueryContext(ctx, sqlQuery, query.TenantID, query.TopK, query.MinScore)
```

**Parameter renumbering:** After removing `$1` (vector), parameters shift: tenant_id → `$1`, top_k → `$2`, min_score → `$3`. Update the `QueryContext` call accordingly.

**Scope:** Single method (`Search`) in one file. No interface changes, no handler changes, no migration needed.

---

## 4. Test Plan

### 4.1 Unit-level (local CockroachDB)
```bash
# Start local CRDB (docker-compose)
docker compose -f scripts/docker-compose.cockroach.yml up -d

# Run cockroach integration tests
BCHAT_ALLOW_DB_RESET=1 go test -tags=cockroach,integration ./store/test/ -run TestCockroach -v -timeout 120s
```

### 4.2 E2E vector round-trip
1. Start bchat with `COCKROACH_DSN` configured
2. Onboard a test tenant
3. Import KB content (>30K tokens to trigger `retrieval_mode=rag`)
4. Trigger reindex
5. Call `POST /api/v1/agent/<slug>/rag/search` with `{"query":"smoke test"}`
6. Verify `total_results > 0` and HTTP 200

### 4.3 Verify script step 6
```bash
BCHAT_URL=http://localhost:5231 BCHAT_USER=admin BCHAT_PASS=... \
  bash scripts/verify-production.sh
# Step [6/7] should show SUCCESS (total_results=N)
```

---

## 5. Risk Assessment

| Risk | Mitigation |
|------|-----------|
| SQL injection via `vecStr` | `formatVectorLiteral` only emits `[0-9,.,e,E,+]` chars — no user input reaches this path |
| Parameter numbering shift breaks other callers | `Search` is the only caller of this internal SQL; no external impact |
| CRDB version incompatibility | `ARRAY[...]` / `[...]::VECTOR` syntax is supported since CRDB v22.2+ (same version that introduced `VECTOR` type) |

---

## 6. Rollout

- Single-file change (`vectordb_cockroach.go`), no migration, no config change
- Rebuild binary: `task build:backend` (or full `task build:rag`)
- Deploy: standard Fly deploy with `fly_cockroach.toml`
- Verify: `scripts/verify-production.sh` step 6 must pass

---

## 7. Open Decisions

| Question | Recommended |
|----------|-------------|
| None — plan is implementation-ready | — |
