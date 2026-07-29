# Adversarial Code Review: bugs/050 — CockroachDB Vector Store Implementation

**Review Date:** 2026-07-29
**Plan Reference:** plan5.md (approved)
**code.md Reference:** bugs/050/code.md

---

## Verdict: **REWORK REQUIRED — 3 critical bugs in production code**

The implementation has 3 critical runtime bugs (broken tenant isolation, wrong column scan, connection pool overwrite) plus significant infrastructure issues. The tests are all empty stubs — zero actual coverage.

---

## Files Reviewed

| File | Lines | Status |
|------|-------|--------|
| `server/router/api/v1/agent/vectordb_cockroach.go` | 332 | CRITICAL bugs |
| `server/router/api/v1/agent/vectordb_nocockroach.go` | 54 | OK (stub) |
| `server/router/api/v1/agent/ticket_embedder.go` | 239 | HIGH issues |
| `server/router/api/v1/agent/ticket_resolution_test.go` | 157 | EMPTY stubs |
| `server/router/api/v1/agent/vectordb.go` | +10 | OK (factory + config) |
| `server/router/api/v1/agent/service.go` | +30 | CRITICAL bug |
| `server/router/api/v1/agent/handlers.go` | +35 | OK |
| `Dockerfile.ecs` | 45 | HIGH (Go version, CGO) |
| `deploy/ccloud/setup.sh` | 55 | HIGH (region format) |
| `cmd/seed/seed_demo_tickets.go` | 144 | MEDIUM (no-op) |
| `Taskfile.yml` | +130 | MEDIUM (missing flags) |
| `go.mod` | +1 | OK (cockroach-go/v2 v2.4.3 added) |

---

## CRITICAL (Must Fix)

### C-1. `Search()` Missing `tenantID` Parameter — Tenant Isolation Broken

**File:** `vectordb_cockroach.go:309`

```go
sqlQuery := fmt.Sprintf(`
    SELECT ...
    WHERE tenant_id = $2 AND content_type IN (%s)
    ORDER BY embedding <=> $1::VECTOR
    LIMIT $3
`, contentTypeFilter)

rows, err := v.db.QueryContext(ctx, sqlQuery, embeddingJSON, query.TopK)
```

The SQL has 3 placeholders (`$1`, `$2`, `$3`) but only 2 arguments are passed. `query.TenantID` is never included as `$2`. This will cause a parameter count error at runtime (`pq: got 2 parameters but the statement requires 3`).

**Impact:** Tenant isolation is completely broken. If the query somehow ran (via simple_protocol truncation), it would return results from ALL tenants.

**Fix:**
```go
rows, err := v.db.QueryContext(ctx, sqlQuery, embeddingJSON, query.TenantID, query.TopK)
```

---

### C-2. `Search()` Column/Scan Mismatch — `metadata` Column Skipped, Wrong Types

**File:** `vectordb_cockroach.go:299-326`

SELECT order:
```
id, title, content, content_type, metadata, source_version, created_at, similarity
```

Scan order (line 320):
```go
rows.Scan(&chunk.ID, &chunk.Title, &chunk.Content, &chunk.ContentType,
    &chunk.SourceVersion, &createdAt, &score)
```

The 5th column returned is `metadata` (JSONB) but the 5th scan target is `&chunk.SourceVersion` (int32). This will fail at runtime with a scan type error when it tries to unmarshal a JSONB value into an int32.

**Impact:** Every `Search()` call fails with a column scan error. The entire vector search is non-functional.

**Fix:**
```go
// Add metadata field to DocumentChunk or scan into a local variable
var metadata string
rows.Scan(&chunk.ID, &chunk.Title, &chunk.Content, &chunk.ContentType,
    &metadata, &chunk.SourceVersion, &createdAt, &score)
```

---

### C-3. `SetDB()` Always Overwrites Separate Connection Pool

**File:** `service.go:160-165`

```go
if cockroachDB, ok := vectorDB.(*CockroachVectorDB); ok {
    cockroachDB.SetDB(s.GetDriver().GetDB())
    slog.Info("CockroachDB vector store initialized with shared connection pool")
}
```

**Problem:** This always calls `SetDB`, even when `NewCockroachVectorDB` opened its own separate pool (because `COCKROACH_DSN` was set). The plan explicitly required checking DSN before wiring:

```go
// From plan5.md lines 368-374 (never implemented):
if vectorDBConfig.CockroachDSN == "" || vectorDBConfig.CockroachDSN == p.DSN {
    cockroachDB.SetDB(s.GetDriver().GetDB())  // Only when reusing existing pool
}
// If COCKROACH_DSN is set and differs, NewCockroachVectorDB already opened its own pool
```

**Impact:** When `COCKROACH_DSN` is explicitly set (the primary use case), the separate pool is opened then immediately orphaned. The shared pool (`s.GetDriver().GetDB()`) may connect to a different database (e.g., SQLite or standard Postgres), making vector operations fail with "relation agent_vectors does not exist" on the wrong database.

**Fix:**
```go
if cockroachDB, ok := vectorDB.(*CockroachVectorDB); ok {
    if vectorDBConfig.CockroachDSN == "" || vectorDBConfig.CockroachDSN == p.DSN {
        cockroachDB.SetDB(s.GetDriver().GetDB())
        slog.Info("CockroachDB vector store initialized with shared connection pool")
    }
}
```

---

## HIGH (Architecture / Production Issues)

### H-1. Docker Base Image Go Version Mismatch (1.21 vs 1.26 required)

**File:** `Dockerfile.ecs:5`

```dockerfile
FROM golang:1.21-alpine AS builder
```

`go.mod:3` requires `go 1.26`. All other Dockerfiles (`Dockerfile.fly:24`, `Dockerfile.pg.fly:24`) use `golang:1.26`. Using `1.21` will fail to compile the codebase which may use Go 1.22+ features (e.g., `min`, `max` builtins, `math/rand/v2`, `for range` integer, loop var change).

**Fix:** Change to `golang:1.26-alpine`.

---

### H-2. `CGO_ENABLED=1` Contradicts Plan Specification

**File:** `Dockerfile.ecs:20`

```dockerfile
RUN CGO_ENABLED=1 GOOS=linux go build -tags "cockroach" -o /app/memos ./bin/memos
```

The plan (`plan5.md:714-716`) explicitly specified `CGO_ENABLED=0`:
```dockerfile
ENV CGO_ENABLED=0
RUN go build -tags cockroach -ldflags="-s -w" -o memos ./bin/memos/main.go
```

`CGO_ENABLED=1` requires `gcc` and `musl-dev` (installed at line 8), complicating the build and increasing image size. The whole point of the ECS Dockerfile was to avoid LanceDB's CGO dependency.

**Fix:** Change to `CGO_ENABLED=0` and remove `gcc musl-dev` from build dependencies.

---

### H-3. Setup Script Region Format Wrong

**File:** `deploy/ccloud/setup.sh:8-9,19`

```bash
REGION="${REGION:-aws-us-east-1}"     # Line 8 — wrong prefix
ccloud cluster create basic "$CLUSTER_NAME" "$REGION" --cloud AWS --spend-limit 0  # Line 19
```

The `ccloud` CLI convention (`ccloud.md:69`) uses plain region names: `us-central1 --cloud GCP`. With `--cloud AWS`, the correct region format is `us-east-1`, not `aws-us-east-1`. The `aws-` prefix does not exist in any `ccloud` documentation and will cause `ccloud` to reject the region.

**Fix:** Change to `REGION="${REGION:-us-east-1}"`.

---

### H-4. IP Allowlist Wide Open (0.0.0.0/0)

**File:** `Taskfile.yml:322`

```yaml
ccloud cluster networking allowlist create hackathon-demo 0.0.0.0/0 --sql --ui --name "hackathon-allowlist"
```

Allows connections from any IP address. The plan specified developer-specific allowlisting and an ECS-specific `crdb:ip:allow:ecs` task. For a hackathon demo, this is a security concern.

**Fix:** Use current IP or require CIDR parameter.

---

### H-5. EmbeddingService Created on Every Cron Tick

**File:** `ticket_embedder.go:113`

```go
embeddings, err := NewEmbeddingService(s.vectorDBConfig.EmbeddingConfig)
```

Creates a new embedding HTTP client every 5 minutes. The embedding config already exists at `s.vectorDBConfig.EmbeddingConfig` which was used during `NewVectorDB()` at startup. The application already has an embedding service instance. This creates a second one, opening new HTTP connections and potentially exhausting connection limits.

**Fix:** Either cache the embedder or use the one from the existing VectorDB (which already holds `embedSvc`, though it's not directly accessible since `CockroachVectorDB.embedSvc` is unexported). Better: expose `EmbedSvc()` on the `VectorDB` interface, or pass the embedder as an argument to the cron function.

---

## MEDIUM (Improvements / Correctness)

### M-1. Seed Script is a No-Op

**File:** `cmd/seed/seed_demo_tickets.go:12-52`

`main()` only prints instructions and never calls `createDemoTickets()`. The `task run:cockroach:seed` command produces no actual data. The `createDemoTickets` function (line 55) takes a `*store.Store` and `tenantID` that are never instantiated in `main()`.

**Impact:** The seed task is dead code. No demo data is created.

**Fix:** Implement `main()` to initialize a store connection and call `createDemoTickets()`.

---

### M-2. `build:backend:cockroach` Missing ldflags and Deps

**File:** `Taskfile.yml:222-226`

```yaml
build:backend:cockroach:
    desc: Build the Go binary with CockroachDB vector store support
    cmds:
      - mkdir -p build
      - go build -tags "cockroach" -o build/memos ./bin/memos/main.go
```

Missing from plan:
- `deps: [validate:migrations, validate:parity]`
- `-ldflags="-s -w"` for smaller binary
- `env: CGO_ENABLED: "0"` for consistent build

---

### M-3. Integration Tests Are Empty Stubs

**File:** `ticket_resolution_test.go:1-157`

All 4 tests construct a `Service{}` but never call the actual functions:

| Test | Calls | Asserts |
|------|-------|---------|
| `TestProcessPendingTickets` | None | None |
| `TestEmbedTenantTickets` | None | None |
| `TestBuildTicketClusters` | None | None |
| `TestEscalateTicket` | None | None |

Each test constructs a minimal `Service{vectorDB, vectorDBConfig}` with an uninitialized `store` (nil), making the actual methods un-callable without panic.

**Impact:** Zero test coverage for all CockroachDB code. CI passes without verifying anything.

---

### M-4. `processPendingTickets` Lacks Per-Tenant Timeout

**File:** `ticket_embedder.go:33-50`

The caller sets a 2-minute global timeout (`service.go:196-198`), but `embedTenantTickets` and `buildTicketClusters` are called sequentially inside the loop. If tenant A has 10,000 tickets, a slow embedding call blocks all subsequent tenants until the global timeout expires.

**Fix:** Add per-tenant context with individual timeout, or run tenants in parallel with `errgroup`.

---

### M-5. `buildTicketClusters` Topological Sort on Embedding Similarity is O(n²)

**File:** `ticket_embedder.go:161-171`

```go
for i, chunkA := range ticketChunks {
    for j, chunkB := range ticketChunks {
        if i >= j { continue }
        sim := cosineSimilarity(chunkA.Embedding, chunkB.Embedding)
```

For N tickets, this computes N(N-1)/2 cosine similarities. For 50 tickets: 1,225 comparisons. For 500 tickets: 124,750 comparisons. This won't scale and runs on every 5-minute cron tick.

**Impact:** As ticket count grows, the cron job will consume excessive CPU during the clustering step.

---

## NIT (Minor / Style)

### N-1. Stub `Validate()` Returns `nil` (Success)

**File:** `vectordb_nocockroach.go:24`

```go
func (v *CockroachVectorDB) Validate(_ context.Context) error { return nil }
```

Returns `nil` as if validation succeeded, but the stub is never supposed to be active. If somehow reached (e.g., via interface assertion), callers would think CockroachDB is ready. The plan stub had all methods returning errors.

---

### N-2. `/m/` Prefix Embedded into Vector Content

**File:** `ticket_embedder.go:95`

```go
content := fmt.Sprintf("%s\n%s", ticket.Title, ticket.Description)
```

`ticket.Description` contains the `/m/` prefix (set by `service.go:5537-5539` and `seed_demo_tickets.go:127`). This internal memo link prefix gets embedded into the vector search content, polluting the semantic embedding space.

---

### N-3. Taskfile `-run` Flag Uses Wrong Separator

**File:** `Taskfile.yml:330`

```yaml
go test -v -tags "cockroach" ./server/router/api/v1/agent/... -run "TestProcessPendingTickets|TestEmbedTenantTickets|TestBuildTicketClusters|TestEscalateTicket"
```

The `-run` flag accepts a regex; `|` (single pipe) is the correct alternation operator. Using `||` (double pipe) would try to match the literal `|` character. However, Go's testing package may handle this gracefully since the pipe is a regex metacharacter and extra `|` is just an empty alternative. Works but inconsistent with convention.

---

## Summary

| Severity | Count | Key Issues |
|----------|-------|-----------|
| CRITICAL | 3 | `tenantID` missing in Search (C-1), column/scan mismatch (C-2), SetDB overwrites pool (C-3) |
| HIGH | 5 | Go 1.21 vs 1.26 Docker (H-1), CGO=1 vs plan (H-2), region format (H-3), open IP allowlist (H-4), EmbeddingService recreated (H-5) |
| MEDIUM | 5 | Seed script no-op (M-1), missing ldflags (M-2), empty test stubs (M-3), no per-tenant timeout (M-4), O(n²) clustering (M-5) |
| NIT | 3 | Stub Validate nil (N-1), /m/ in embedding (N-2), || vs | in test filter (N-3) |

### Pre-Deployment Fix Checklist

1. **C-1:** Add `query.TenantID` as `$2` in `Search()` QueryContext args
2. **C-2:** Add `metadata` variable to column scan in `Search()`
3. **C-3:** Guard `SetDB()` with `CockroachDSN == "" || CockroachDSN == p.DSN` check
4. **H-1:** Change `golang:1.21-alpine` → `golang:1.26-alpine`
5. **H-2:** Change `CGO_ENABLED=1` → `CGO_ENABLED=0`, remove `gcc musl-dev`
6. **H-3:** Change `aws-us-east-1` → `us-east-1`
7. **H-4:** Replace `0.0.0.0/0` with developer IP or CIDR parameter
8. **H-5:** Cache EmbeddingService instance instead of recreating on each cron tick
9. **M-1:** Implement `main()` to actually seed tickets
10. **M-3:** Write real integration tests that call the functions under test
