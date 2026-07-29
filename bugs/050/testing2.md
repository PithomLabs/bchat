# Testing2: Code Fix for `vector_ip_ops` + Re-verification

## Objective

Fix the `vector_ip_ops` operator class bug in `vectordb_cockroach.go:113` and re-verify against local CRDB Docker.

---

## 1. Code Fix: `vectordb_cockroach.go`

### Problem

`vector_ip_ops` is NOT supported in CRDB v25.2.21 or v26.2.4 (error `0A000`, issue [#144016](https://go.crdb.dev/issue-v/144016)). The code at line 113 hardcodes this opclass:

```go
// vectordb_cockroach.go:111-113
_, err = v.db.ExecContext(ctx, `
    CREATE VECTOR INDEX idx_agent_vectors_embedding
    ON agent_vectors (embedding vector_ip_ops)
`)
```

Error path: line 119 only checks `pgErr.Code == "42P07"` (duplicate_table). The `0A000` error falls through to the warning-and-brute-force path at line 122.

### Fix

**Line 113:** Remove `vector_ip_ops` → defaults to `vector_l2_ops` (the only supported opclass in v25.2+).

**Lines 115-127:** Extend error handler to also check `0A000` (feature_not_supported) for a more accurate log message.

```go
// 3. Vector index (CRDB-specific syntax — NOT pgvector USING hnsw)
// NOTE: CREATE VECTOR INDEX does NOT support IF NOT EXISTS
// vector_ip_ops is NOT supported (CRDB issue #144016) — default to vector_l2_ops
// Check for "relation already exists" (42P07) or "feature not supported" (0A000) and treat as non-fatal
_, err = v.db.ExecContext(ctx, `
    CREATE VECTOR INDEX idx_agent_vectors_embedding
    ON agent_vectors (embedding)
`)
if err != nil {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) && pgErr.Code == "42P07" {
        slog.Info("Vector index already exists", "index", "idx_agent_vectors_embedding")
    } else if errors.As(err, &pgErr) && pgErr.Code == "0A000" {
        slog.Warn("Vector index feature not supported, using brute-force search",
            "error", err,
            "hint", "Ensure feature.vector_index.enabled = true or upgrade CRDB")
    } else {
        slog.Warn("Vector index creation failed",
            "error", err,
            "hint", "May need feature.vector_index.enabled or CRDB v25.2+")
    }
}
```

### Files Modified

| File | Line | Change |
|------|------|--------|
| `server/router/api/v1/agent/vectordb_cockroach.go` | 113 | Remove `vector_ip_ops` |
| `server/router/api/v1/agent/vectordb_cockroach.go` | 119-127 | Add `0A000` error check with specific log message |

---

## 2. Testing.md Update

### Changes to `bugs/050/testing.md`

| Section | Change |
|---------|--------|
| "What Was Verified" table, Vector index row | Add footnote: "Created via workaround (opclass omitted). Code's `vector_ip_ops` fails — see Key Finding below." |
| "Key Finding: vector_ip_ops Not Supported" | Update to note v26.2.4 also does not support `vector_ip_ops` (verified 2026-07-29) |
| New section "v26.2 Verification" | Document that v26.2.4 exists on Docker Hub, `vector_ip_ops` NOT listed in v26.2 vector indexes docs, issue #144016 still open |
| "Code Fix Needed" | Remove — fix is now implemented in this plan |

---

## 3. Re-verification Steps

### Prerequisites

- Local CRDB container running (`bchat-crdb`)
- Feature flag enabled (`SET CLUSTER SETTING feature.vector_index.enabled = true`)
- `bchat` database and `bchat_user` created (from previous testing)

### Steps

1. **Drop old table + indexes** (clean slate):
   ```sql
   DROP TABLE IF EXISTS agent_vectors CASCADE;
   ```

2. **Apply code fix** — edit `vectordb_cockroach.go:113` per section 1

3. **Build with cockroach tag**:
   ```bash
   go build -tags cockroach ./...
   ```

4. **Run unit tests**:
   ```bash
   go test -tags cockroach ./server/router/api/v1/agent/ -run TestVectorDB
   ```

5. **Start CRDB and verify**:
   ```bash
   docker compose -f scripts/docker-compose.cockroach.yml up -d
   # Enable vector indexes (already persisted via volume)
   docker exec bchat-crdb cockroach sql --insecure --host=localhost \
     -e "SET CLUSTER SETTING feature.vector_index.enabled = true;"
   ```

6. **Test DDL manually** (no opclass):
   ```sql
   CREATE TABLE agent_vectors (
       id STRING PRIMARY KEY,
       tenant_id INT NOT NULL,
       content_type STRING NOT NULL,
       title STRING,
       content TEXT NOT NULL,
       embedding VECTOR(1536),
       metadata JSONB,
       source_version INT DEFAULT 1,
       created_at TIMESTAMPTZ DEFAULT now()
   );
   CREATE INDEX idx_agent_vectors_tenant ON agent_vectors (tenant_id);
   CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding);
   ```

7. **Verify vector index exists**:
   ```sql
   SHOW INDEXES FROM agent_vectors;
   -- Should show idx_agent_vectors_embedding with vector_l2_ops
   ```

8. **Test cosine distance query** (confirm `<=>` works with `vector_l2_ops` index):
   ```sql
   SELECT '[0.1, 0.2, 0.3]'::VECTOR(3) <=> '[0.1, 0.2, 0.3]'::VECTOR(3);
   -- Should return 0.0 (identical vectors)
   ```

9. **Stop CRDB**:
   ```bash
   docker compose -f scripts/docker-compose.cockroach.yml down
   ```

---

## 4. Adversarial Testing2 Review Prompt

Review the changes described in `testing2.md` for correctness, completeness, and risks. Check:

1. **Code fix correctness:** Is removing the opclass the right approach? Does `CREATE VECTOR INDEX ... ON agent_vectors (embedding)` (no opclass) default to `vector_l2_ops`? Is `vector_l2_ops` compatible with the cosine distance `<=>` operator used in Search()?

2. **Error handler:** Is checking `pgErr.Code == "0A000"` the correct way to detect "feature not supported"? Does the `pgconn.PgError` type from pgx/v5 correctly surface this code?

3. **Non-fatal path:** The fix keeps the vector index creation as non-fatal (brute-force fallback). Is this acceptable for production? Should we fail-fast instead?

4. **Testing.md updates:** Are the v26.2 verification notes accurate? Is the reconciliation table still correct after the code fix?

5. **Build tags:** Does `go build -tags cockroach ./...` include the fixed file? Does `go build` (without tag) exclude it correctly?

6. **Regression risk:** Does removing the opclass change any behavior for existing deployments that already have the index created with `vector_l2_ops` (from the workaround DDL)?

7. **Documentation:** Should `bugs/050/plan4_testing.md` also be updated to reflect the code fix, or is testing2.md sufficient?

Return findings as a table with columns: #, Category, Severity (CRITICAL/HIGH/MEDIUM/NIT), Finding, Fix.
