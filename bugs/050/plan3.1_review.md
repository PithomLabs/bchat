# Adversarial Review: plan3.1.md (bchat × CockroachDB × AWS)

**Reviewer Context:** Cross-references `plan3.1.md` against:
- `cockroachdb-skills-main/` at `/home/chaschel/Desktop/cockroach/cockroachdb-skills-main/` (CockroachDB agent skills repository)
- `ccloud.md` at `/home/chaschel/Desktop/cockroach/ccloud.md` (official ccloud CLI docs)
- bchat codebase at `/home/chaschel/Documents/go/bchat/`

---

## What Plan 3.1 Fixes from Plan 2.1

| Issue from Plan 2.1 | Status in Plan 3.1 | Evidence |
|---|---|---|
| Migration isolation for non-CRDB (M-1) | Fixed — migration removed entirely, schema in `Validate()` per provider | Lines 524-552 |
| `ccloud cluster create` flags unverified (C-2) | Fixed — syntax verified against `ccloud.md:47-70` | Lines 648-671 |
| Connection pool relationship (M-2) | Clarified — reuses existing `*sql.DB` when DSN empty | Line 455 |
| Backfill safety warning (M-4) | Fixed — documented | Lines 548-550 |
| Test dependency on live cluster (M-5) | Fixed — `TestMain` guard, build tags | Lines 697-715 |
| ECS-to-CRDB networking (M-6) | Fixed — `crdb:ip:allow:ecs` task with CIDR | Lines 1000-1004 |

---

## CRITICAL (Blockers)

### C-1. `CREATE VECTOR INDEX IF NOT EXISTS` Not Supported

**Finding:** Line 261 uses `CREATE VECTOR INDEX IF NOT EXISTS` — the `IF NOT EXISTS` clause is not documented for `CREATE VECTOR INDEX` anywhere in the skills repo.

**Evidence from skills repo (`01-schema-design.md:150-159`):**
```sql
-- Skills repo shows ONLY:
CREATE VECTOR INDEX idx_embedding ON items (embedding vector_ip_ops);
CREATE VECTOR INDEX idx_embedding ON items (embedding vector_ip_ops)
  WITH (build_beam_size = 16, min_partition_size = 8, max_partition_size = 32);

-- No IF NOT EXISTS variant exists in any example
```

The `IF NOT EXISTS` clause is shown for `CREATE TABLE` and `CREATE DATABASE` elsewhere in the skills repo (`01-schema-design.md:58`, `05-operational.md:53`), but NOT for `CREATE VECTOR INDEX`.

**Severity:** CRITICAL — the DDL will return a syntax error on every `Validate()` call after the first. The plan wraps it in a non-fatal warn log, so the application won't crash, but every startup/validate cycle will log an error.

**Recommendation:** Use `CREATE VECTOR INDEX` without `IF NOT EXISTS`. Capture the error and check if it's a "relation already exists" error (postgres error code `42P07`). Log at INFO level in that case; WARN otherwise.

---

### C-2. Cron Job Closure Captures Undefined Variable

**Finding:** Line 579-583 registers the ticket embedder cron job:

```go
if os.Getenv("VECTOR_DB_PROVIDER") == "cockroach" {
    cronJob := cron.New()
    cronJob.AddFunc("*/5 * * * *", func() {
        s.ticketEmbedder.ProcessPending(context.Background(), tenantID) // BUG
    })
    cronJob.Start()
}
```

`tenantID` is not defined in the scope of `NewService()`. A ticket embedder cron must iterate over all active tenants, not reference a single undefined variable.

**Evidence from codebase:** `service.go`'s `NewService()` constructor (`service.go:60-115`) operates on the global store, not a single tenant. Tenant-scoped operations use `store.ListAgentTenants()` and iterate.

**Severity:** CRITICAL — the code as written won't compile.

**Recommendation:** Change to iterate all active tenants:
```go
func (s *Service) processPendingTickets(ctx context.Context) {
    tenants, err := s.store.ListAgentTenants(ctx, &store.FindAgentTenant{IsActive: &[]bool{true}[0]})
    if err != nil { slog.Error("failed to list tenants", "error", err); return }
    for _, tenant := range tenants {
        s.ticketEmbedder.ProcessPending(ctx, tenant.ID)
    }
}
```

---

## HIGH (Architecture / Edge Cases)

### H-1. `NewVectorDB` Factory Lacks `*sql.DB` Parameter

**Finding:** The plan adds a `"cockroach"` case to `NewVectorDB()` factory at line 445-448:
```go
case "cockroach":
    return NewCockroachVectorDB(config, embedSvc)
```

But line 455 states: *"If `COCKROACH_DSN` is empty or matches `MEMOS_DSN`, reuse the existing `*sql.DB` from `store.Driver.GetDB()`."*

**Evidence from codebase:** `NewVectorDB()` at `vectordb.go:255` takes only `*VectorDBConfig` as a parameter — no `*sql.DB`, no `store.Store`:
```go
func NewVectorDB(config *VectorDBConfig) (VectorDB, error) {
```
The factory doesn't have access to the application's shared connection pool. Only `Service` has the store.

**Severity:** HIGH — the connection reuse strategy can't be implemented in the factory as written. Two options:
1. Pass `*sql.DB` to the factory (breaks existing callers)
2. Have `Service` wire the `*sql.DB` post-construction (similar to `TenantVectorDBPool.SetStore()` pattern at `vectordb_pool.go:33`)

**Recommendation:** Use option 2 — add a `SetDB(db *sql.DB)` method to `CockroachVectorDB`, and have `Service` call it after construction. If `COCKROACH_DSN` is set, `NewCockroachVectorDB` opens its own pool. If not, `Service` passes the existing one via `SetDB()`.

---

### H-2. `vector_cosine_ops` Not Verified; Only `vector_ip_ops` in Skills Repo

**Finding:** Line 106 claims both `vector_cosine_ops` and `vector_ip_ops` are supported in CRDB v26.2. The skills repo (`01-schema-design.md:152,158`) only documents `vector_ip_ops`. Neither `vector_cosine_ops` nor a comprehensive opclass list appears anywhere in the skills repo.

**Evidence from skills repo:**
```sql
-- Only opclass ever shown:
CREATE VECTOR INDEX idx_embedding ON items (embedding vector_ip_ops);
```

**Severity:** HIGH — if a CRDB cluster version doesn't support `vector_cosine_ops` (e.g., pre-v26.2 or a minor version gap), the index creation fails entirely. Cosine distance on normalized vectors is equivalent to inner product (`1 - cosine = inner_product` for unit vectors), so `vector_ip_ops` is a safe substitute.

**Recommendation:** Add a comment documenting the equivalence: *"Cosine distance on normalized vectors = inner product. If `vector_cosine_ops` is unavailable, `vector_ip_ops` produces identical results."* Consider using `vector_ip_ops` directly for maximum compatibility, or add a runtime fallback in `Validate()`.

---

### H-3. `feature.vector_index.enabled` Not Found in Any Verified Source

**Finding:** Line 833 lists `feature.vector_index.enabled = true` as a prerequisite. This cluster setting does not appear in the skills repo or the bchat codebase. It was not found via grep over either codebase.

**Evidence:** Grep of `cockroachdb-skills-main/` and `bchat/` for `feature.vector_index` — zero results.

**Severity:** HIGH — if this setting doesn't exist or has a different name (e.g., `feature.vectorizer.enabled`, a version-gated feature flag), the vector index creation silently creates a B-tree index instead, and the search query `ORDER BY embedding <=> $1` performs a full table scan.

**Recommendation:** Verify against CRDB v26.2 official docs. If confirmed, add the exact `SET CLUSTER SETTING` command to the setup script at line 648-671.

---

### H-4. Embedding JSON Serialization for `VECTOR(1536)` Untested

**Finding:** The plan serializes embeddings via `json.Marshal(chunk.Embedding)` at line 360, producing `[0.1,0.2,...]`, and passes the byte slice as a parameterized query argument. CRDB's `VECTOR` type accepts Postgres array format `'{0.1,0.2}'` or JSON array format `[0.1,0.2]`.

**Evidence from codebase:** `DocumentChunk.Embedding` is `[]float32` (`chunker.go:34`). `json.Marshal` on `[]float32{0.1, 0.2}` produces `[0.1,0.2]`. The query uses pgx with `default_query_exec_mode=simple_protocol` (line 149), which sends parameters as text. CRDB must parse `[0.1,0.2]` as a `VECTOR` literal.

**Severity:** HIGH — if pgx wraps the bytes in quotes (e.g., `'[0.1,0.2]'` instead of `[0.1,0.2]`), CRDB may reject it as a malformed vector. The vector literal format for CRDB is: `'[0.1,0.2]'` or `ARRAY[0.1,0.2]::VECTOR(1536)`.

**Recommendation:** Test this encoding against a real CRDB cluster before committing. If pgx/stdlib adds wrapping quotes, use `fmt.Sprintf("[%s]", strings.Trim(strings.Join(...)))` or cast via `$1::VECTOR`.

---

## MEDIUM (Improvements / Clarity)

### M-1. Seed Data Performance — 50 Single-Row Serializable Transactions

**Finding:** The seed script at line 358-371 inserts each of 50 demo tickets via a separate `crdb.ExecuteTx` call. Each call opens a serializable transaction with retry on `SQLSTATE 40001`. For 50 tickets, this is 50 round trips with potential retries.

**Severity:** MEDIUM — acceptable for a demo seed script, but the hackathon demo setup will be slower than necessary.

**Recommendation:** Use a single `crdb.ExecuteTx` containing all 50 `UPSERT` statements, or batch them into groups of 10. For seed data (no concurrent writers), retry probability is near zero, so the overhead of 50 transactions is purely latency.

---

### M-2. `psql` in `crdb:sql:query` Task is Fragile

**Finding:** Line 1172 uses `psql "$COCKROACH_DSN"` for ad-hoc queries. CRDB is PostgreSQL-compatible but `psql` has known issues with CRDB's prepared statement protocol, especially with `simple_protocol` enabled in the DSN.

**Evidence from codebase:** CRDB DSNs in bchat always include `default_query_exec_mode=simple_protocol` (line 149, and `postgres.go:36`). This setting is for `pgx`, not `psql`. `psql` uses `libpq` which has its own CRDB compatibility behaviors.

**Severity:** MEDIUM — the task will fail on queries that touch vector types or use CRDB-specific syntax.

**Recommendation:** Use `ccloud cluster sql --connection-url {{.NAME}}` for interactive queries, or add a note that `psql` must be installed and may not support all CRDB features. Alternatively, use `cockroach sql` if the CRDB binary is available.

---

### M-3. Existing LanceDB Vector Table Not Addressed

**Finding:** The existing codebase creates `agent_vectors` in LanceDB (for `local`/`s3` storage providers). Plan 3.1 creates a SQL table of the same name in CRDB. If a developer switches `VECTOR_DB_PROVIDER` from `local` to `cockroach`, the old LanceDB data becomes inaccessible but isn't cleaned up. Conversely, switching back to `local` after using CRDB leaves CRDB tables orphaned.

**Severity:** MEDIUM — no data loss risk (each provider manages its own storage), but could confuse judges during demo if residual data appears.

**Recommendation:** Add a note in the setup docs: *"Switching VECTOR_DB_PROVIDER creates a fresh namespace. Data in the previous provider is preserved but not accessible through the new provider."*

---

### M-4. Dockerfile.ecs Uses Go 1.26 — Not Verified in Codebase

**Finding:** Line 639: `FROM golang:1.26 AS backend`. The bchat codebase's existing Dockerfiles (`Dockerfile.fly:24`, `Dockerfile.pg.fly:24`) also use `golang:1.26`. This matches.

**Severity:** LOW — consistent with existing patterns.

**Recommendation:** None.

---

### M-5. `ccloud cluster create basic ... --cloud AWS` Verified, but `--spend-limit 0` Means No Spend Limit

**Finding:** Line 653: `ccloud cluster create basic bchat-db us-east-1 --cloud AWS --spend-limit 0`. Verified at `ccloud.md:69`. However, `--spend-limit 0` means "no spend limit" (the cluster will accrue charges without a cap). For a hackathon demo, this could result in unexpected costs if left running.

**Severity:** LOW — a note about cost management would help judges and developers.

**Recommendation:** Add a note: *"`--spend-limit 0` disables the monthly spend cap. For hackathon demos, consider a budget or set `task crdb:cluster:delete` after demo."*

---

### M-6. Migration Removal Means DB Schema Is Invisible to Migration Runner

**Finding:** By removing the migration file entirely (Component 1b, lines 524-552), the `agent_vectors` table is now created at runtime via `Validate()` rather than through the migration system. This means:
- `task validate:schema` won't see `agent_vectors` in the expected schema
- `task validate:parity` can't cross-check it between SQLite/Postgres
- DB admin tools and `psql` won't find a migration record for this table

**Severity:** MEDIUM — acceptable for the hackathon (the table is CRDB-only and provider-managed), but the schema validation tasks will need adjustment if `Validate()` is expected to be verified.

**Recommendation:** Add a note: *"The agent_vectors table is created by CockroachVectorDB.Validate() at runtime and is excluded from migration-based schema validation tasks."*

---

## Summary

| Severity | Count | Key Issues |
|----------|-------|-----------|
| CRITICAL | 2 | `CREATE VECTOR INDEX IF NOT EXISTS` unsupported (C-1), cron captures undefined `tenantID` (C-2) |
| HIGH | 4 | Factory lacks `*sql.DB` parameter (H-1), `vector_cosine_ops` unverified (H-2), `feature.vector_index.enabled` not found (H-3), embedding JSON serialization untested (H-4) |
| MEDIUM | 6 | Seed performace (M-1), `psql` fragility (M-2), LanceDB orphan data (M-3), Go version OK (M-4), spend limit cost note (M-5), migration invisibility (M-6) |

### Key Improvements Over Plan 2.1

1. **Migration isolation is solved** — no migration file means no risk of breaking non-CRDB deployments. Schema creation in `Validate()` per provider is the correct approach.
2. **ccloud commands verified** against `ccloud.md` — syntax matches official docs.
3. **Test harness** has proper `TestMain` guard and build tag isolation.
4. **Connection pool strategy** clarified (reuse vs separate pool).
5. **Documentation annotations** (`# Source: ccloud.md:N-M`) make verification transparent.

### Critical Remaining Gaps

1. C-1 (`CREATE VECTOR INDEX IF NOT EXISTS`) must be fixed — it will error on every `Validate()` call after the first. Use bare `CREATE VECTOR INDEX` with `42P07` error handling.
2. C-2 (undefined `tenantID` in cron closure) must be fixed — the code won't compile. Iterate all active tenants.
3. H-1 (factory lacks `*sql.DB`) must be resolved before connection reuse works. Use post-construction wiring like the existing `TenantVectorDBPool.SetStore()` pattern.
