# Code3: Consolidated Review Fixes

**Date:** 2026-07-29
**Sources:** code_review.md + code2_review.md + code2.md
**Status:** Plan only — awaiting implementation

---

## Review Results

| Source | CRITICAL | HIGH | MEDIUM | NIT | Total |
|--------|----------|------|--------|-----|-------|
| code_review.md | 3 | 5 | 5 | 3 | 16 |
| code2.md | 0 | 3 | 0 | 0 | 3 |
| Verified (this doc) | 3 | 7 | 1 | 0 | **11** |

---

## CRITICAL (Must Fix First)

### C-1. Search() Missing tenantID Parameter

**File:** `vectordb_cockroach.go:309`
**Severity:** CRITICAL

**Problem:**
```sql
SELECT id, title, content, content_type, metadata, source_version, created_at,
       1 - (embedding <=> $1::VECTOR) AS similarity
FROM agent_vectors
WHERE tenant_id = $2 AND content_type IN (%s)
ORDER BY embedding <=> $1::VECTOR
LIMIT $3
```

SQL has 3 placeholders (`$1`, `$2`, `$3`) but only 2 args passed:
```go
rows, err := v.db.QueryContext(ctx, sqlQuery, embeddingJSON, query.TopK)
// Missing: query.TenantID
```

**Impact:** Runtime parameter count error, or silently returns results from ALL tenants.

**Fix:** Add `query.TenantID` as `$2`:
```go
rows, err := v.db.QueryContext(ctx, sqlQuery, embeddingJSON, query.TenantID, query.TopK)
```

---

### C-2. Search() Column/Scan Mismatch

**File:** `vectordb_cockroach.go:299-326`
**Severity:** CRITICAL

**Problem:**
```sql
SELECT id, title, content, content_type, metadata, source_version, created_at, similarity
-- 8 columns
```

Scan targets (line 320):
```go
rows.Scan(&chunk.ID, &chunk.Title, &chunk.Content, &chunk.ContentType,
    &chunk.SourceVersion, &createdAt, &score)
// 7 targets — metadata column skipped
```

Column 5 (`metadata` JSONB) is skipped, causing misalignment: `SourceVersion` receives `metadata` value, `createdAt` receives `source_version`, etc.

**Impact:** Every `Search()` call fails with scan error or returns wrong data.

**Fix:** Add metadata scan variable:
```go
var metadata string
rows.Scan(&chunk.ID, &chunk.Title, &chunk.Content, &chunk.ContentType,
    &metadata, &chunk.SourceVersion, &createdAt, &score)
```

---

### C-3. SetDB Unconditional — Overwrites Separate Pool

**File:** `service.go:160-165`
**Severity:** CRITICAL

**Problem:**
```go
if cockroachDB, ok := vectorDB.(*CockroachVectorDB); ok {
    cockroachDB.SetDB(s.GetDriver().GetDB())
    slog.Info("CockroachDB vector store initialized with shared connection pool")
}
```

When `COCKROACH_DSN` is explicitly set, `NewCockroachVectorDB` opens its own pool. `SetDB()` then overwrites it with `s.GetDriver().GetDB()` which may be SQLite or a different Postgres database.

**Impact:** Vector operations fail with "relation agent_vectors does not exist" on wrong database.

**Fix:** Guard with DSN check:
```go
if cockroachDB, ok := vectorDB.(*CockroachVectorDB); ok {
    if vectorDBConfig.CockroachDSN == "" || vectorDBConfig.CockroachDSN == p.DSN {
        cockroachDB.SetDB(s.GetDriver().GetDB())
        slog.Info("CockroachDB vector store initialized with shared connection pool")
    } else {
        slog.Info("CockroachDB vector store using dedicated connection pool")
    }
}
```

---

## HIGH (Must Fix Before Deploy)

### H-1. nil Embedding → null::VECTOR Fails

**File:** `vectordb_cockroach.go:175-194`, `198-224`
**Severity:** HIGH

**Problem:** `json.Marshal(nil)` produces `"null"`, and `null::VECTOR` fails at DB level.

**Fix:** Pass `nil` when embedding is empty:
```go
var embeddingValue interface{} = chunk.Embedding
if len(chunk.Embedding) == 0 {
    embeddingValue = nil
}
// Use embeddingValue in ExecContext args
```

---

### H-2. Search() Empty Query Not Validated

**File:** `vectordb_cockroach.go:273-281`
**Severity:** HIGH

**Problem:** No validation for empty `QueryText` + nil `QueryEmbedding`.

**Fix:** Early return at top of Search():
```go
if query.QueryText == "" && len(query.QueryEmbedding) == 0 {
    return &SearchResult{
        Chunks:  []DocumentChunk{},
        Scores:  []float64{},
        Total:   0,
        Latency: time.Since(start),
    }, nil
}
```

---

### H-3. Description Length Not Validated

**File:** `handlers.go:486`
**Severity:** HIGH

**Problem:** No length limit on ticket description.

**Fix:** Add length check in handler:
```go
if len(req.Description) > 10000 {
    return echo.NewHTTPError(http.StatusBadRequest, "description must be under 10000 characters")
}
```

---

### H-4. Docker Go Version Mismatch

**File:** `Dockerfile.ecs:5`
**Severity:** HIGH

**Problem:** `golang:1.21-alpine` may not compile codebase using Go 1.22+ features.

**Fix:** Change to `golang:1.26-alpine`:
```dockerfile
FROM golang:1.26-alpine AS builder
```

---

### H-5. CGO_ENABLED=1 Contradicts Plan

**File:** `Dockerfile.ecs:8,20`
**Severity:** HIGH

**Problem:** `CGO_ENABLED=1` requires `gcc` and `musl-dev`, increasing image size. CockroachDB vector store uses pure Go `pgx/v5` driver.

**Fix:** Set `CGO_ENABLED=0` and remove CGO dependencies:
```dockerfile
# Remove: gcc musl-dev from apk add
RUN CGO_ENABLED=0 GOOS=linux go build -tags "cockroach" -o /app/memos ./bin/memos
```

---

### H-6. Setup Script Region Format

**File:** `deploy/ccloud/setup.sh:8`
**Severity:** HIGH

**Problem:** `aws-us-east-1` may be invalid; `ccloud` CLI expects plain region with `--cloud AWS`.

**Fix:** Change default:
```bash
REGION="${REGION:-us-east-1}"
```

---

### H-7. EmbeddingService Recreated Per Tenant

**File:** `ticket_embedder.go:113`
**Severity:** HIGH

**Problem:** `NewEmbeddingService()` called inside `embedTenantTickets()`, invoked per-tenant in cron loop. Creates new HTTP client every 5 minutes.

**Fix:** Pass embedder as parameter or cache it:
```go
func (s *Service) embedTenantTickets(ctx context.Context, vectorDB VectorDB, tenantID int32, embedSvc EmbeddingService) error {
    // ... use embedSvc instead of creating new one
}
```

Update cron caller:
```go
embedSvc, err := NewEmbeddingService(s.vectorDBConfig.EmbeddingConfig)
if err != nil {
    slog.Error("Failed to create embedding service", "error", err)
    return
}
for _, tenant := range tenants {
    s.embedTenantTickets(ctx, vectorDB, tenant.ID, embedSvc)
}
```

---

## MEDIUM (Should Fix)

### M-1. Seed Script No-Op

**File:** `cmd/seed/seed_demo_tickets.go:12-52`
**Severity:** MEDIUM

**Problem:** `main()` only prints instructions; `createDemoTickets()` defined but never called.

**Fix:** Implement `main()` to initialize store and call `createDemoTickets()`:
```go
func main() {
    // Initialize store connection
    // Get tenant ID
    // Call createDemoTickets(ctx, store, tenantID)
}
```

---

## Implementation Order

| Step | Fix | File | Est. Lines |
|------|-----|------|------------|
| 1 | C-1: Add tenantID to Search args | `vectordb_cockroach.go` | +1 |
| 2 | C-2: Add metadata scan variable | `vectordb_cockroach.go` | +2 |
| 3 | C-3: Guard SetDB with DSN check | `service.go` | +5 |
| 4 | H-1: Fix nil embedding handling | `vectordb_cockroach.go` | +6 |
| 5 | H-2: Add empty query guard | `vectordb_cockroach.go` | +8 |
| 6 | H-3: Add description length validation | `handlers.go` | +3 |
| 7 | H-4: Fix Docker Go version | `Dockerfile.ecs` | -1/+1 |
| 8 | H-5: Set CGO_ENABLED=0 | `Dockerfile.ecs` | -3/+1 |
| 9 | H-6: Fix region format | `deploy/ccloud/setup.sh` | +1 |
| 10 | H-7: Cache EmbeddingService | `ticket_embedder.go` | +10/-5 |
| 11 | M-1: Implement seed script | `cmd/seed/seed_demo_tickets.go` | +30 |
| 12 | Write real integration tests | `ticket_resolution_test.go` | +50 |
| 13 | Verify both builds | `go build` commands | — |
| 14 | Run tests | `go test` commands | — |

**Total estimated lines changed:** ~125 lines

---

## Build Verification

```bash
# Both builds must compile
go build -tags cockroach ./bin/memos/...     # passes
go build ./bin/memos/...                     # passes (non-cockroach)

# Run tests
go test -tags cockroach ./server/router/api/v1/agent/... -v

# Verify Docker builds
docker build -t bchat:cockroach -f Dockerfile.ecs .
```

---

## Risk Assessment

| Fix | Risk | Mitigation |
|-----|------|------------|
| C-1: tenantID arg | LOW | Single arg addition to existing query |
| C-2: metadata scan | LOW | Scan variable addition, no logic change |
| C-3: SetDB guard | LOW | DSN comparison is safe; fallback is documented |
| H-1: nil embedding | LOW | NULL embeddings are valid SQL |
| H-2: empty query | LOW | Early return with empty result |
| H-3: description length | LOW | Hard limit prevents memory exhaustion |
| H-4: Go version | LOW | Match go.mod requirement |
| H-5: CGO=0 | LOW | Pure Go pgx driver, no CGO needed |
| H-6: region format | LOW | Match ccloud CLI docs |
| H-7: cache embedder | LOW | Single instance, reused per tick |
| M-1: seed script | LOW | Placeholder for demo data |
