# Testing3: Code Fix for `vector_ip_ops` + Re-verification (Revised)

## Objective

Fix the `vector_ip_ops` operator class bug in `vectordb_cockroach.go:113`, verify v26.2.4 support, and re-verify against local CRDB Docker.

---

## 1. Code Fix: `vectordb_cockroach.go`

### Problem

`vector_ip_ops` is NOT supported in CRDB v25.2.21 (error `0A000`, issue [#144016](https://go.crdb.dev/issue-v/144016)). The code at line 113 hardcodes this opclass:

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

**Lines 115-127:** Replace duplicate `errors.As` calls with single call + `switch`:

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
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "42P07":
            slog.Info("Vector index already exists", "index", "idx_agent_vectors_embedding")
        case "0A000":
            slog.Warn("Vector index feature not supported, using brute-force search",
                "error", err,
                "hint", "Ensure feature.vector_index.enabled = true or upgrade CRDB")
        default:
            slog.Warn("Vector index creation failed",
                "error", err,
                "hint", "May need feature.vector_index.enabled or CRDB v25.2+")
        }
    } else {
        slog.Warn("Vector index creation failed (non-PG error)",
            "error", err)
    }
}
```

### Files Modified

| File | Line | Change |
|------|------|--------|
| `server/router/api/v1/agent/vectordb_cockroach.go` | 113 | Remove `vector_ip_ops` |
| `server/router/api/v1/agent/vectordb_cockroach.go` | 115-127 | Single `errors.As` + `switch pgErr.Code` |

---

## 2. v26.2.4 Verification

### Problem

testing2.md claims "v26.2.4 also does not support vector_ip_ops" but this was **not tested** — only v25.2.21 was verified.

### Verification Step

Pull v26.2.4 and test `vector_ip_ops` explicitly:

```bash
# Pull v26.2.4
docker pull cockroachdb/cockroach:v26.2.4

# Start temporary v26.2.4 instance
docker run --name bchat-crdb-v26 \
  -p 26258:26257 -p 8081:8080 \
  -d cockroachdb/cockroach:v26.2.4 \
  start-single-node --insecure --advertise-addr=localhost

# Enable vector indexes
docker exec bchat-crdb-v26 cockroach sql --insecure --host=localhost \
  -e "SET CLUSTER SETTING feature.vector_index.enabled = true;"

# Create test table
docker exec bchat-crdb-v26 cockroach sql --insecure --host=localhost \
  -e "CREATE DATABASE test; CREATE TABLE test.vec (id INT PRIMARY KEY, embedding VECTOR(3));"

# Test vector_ip_ops (expect 0A000 error)
docker exec bchat-crdb-v26 cockroach sql --insecure --host=localhost -d test \
  -e "CREATE VECTOR INDEX ON test.vec (embedding vector_ip_ops);"

# Test default opclass (expect success)
docker exec bchat-crdb-v26 cockroach sql --insecure --host=localhost -d test \
  -e "CREATE VECTOR INDEX ON test.vec (embedding);"

# Cleanup
docker stop bchat-crdb-v26 && docker rm bchat-crdb-v26
```

### Update testing.md

| Section | Change |
|---------|--------|
| "v26.2 Verification" | Replace doc-only claim with actual test results |
| Reconciliation table | Update code row from ❌ to ✅ after fix |

---

## 3. Testing.md Update

### Changes to `bugs/050/testing.md`

| Section | Change |
|---------|--------|
| "What Was Verified" table, Vector index row | Add footnote: "Created via workaround (opclass omitted). Code's `vector_ip_ops` fails — see Key Finding below." |
| "Key Finding: vector_ip_ops Not Supported" | Update to include v26.2.4 verification results (from step 2) |
| New section "v26.2 Verification" | Document actual Docker test results (not just doc check) |
| "Code Fix Needed" | Remove — fix is now implemented in this plan |
| Reconciliation table | Update code row: ❌ → ✅ (after fix applied) |

---

## 4. Re-verification Steps

### Prerequisites

- `docker compose` v2 (not `docker-compose` v1) installed
- Local CRDB container previously ran (`bchat-crdb` volume exists)

### Steps

1. **Start CRDB** (must be running before any SQL commands):
   ```bash
   docker compose -f scripts/docker-compose.cockroach.yml up -d
   ```

2. **Enable vector indexes** (persists via volume):
   ```bash
   docker exec bchat-crdb cockroach sql --insecure --host=localhost \
     -e "SET CLUSTER SETTING feature.vector_index.enabled = true;"
   ```

3. **Drop old table + indexes** (clean slate):
   ```sql
   DROP TABLE IF EXISTS agent_vectors CASCADE;
   ```

4. **Apply code fix** — edit `vectordb_cockroach.go:113` per section 1

5. **Build with cockroach tag** (compile check):
   ```bash
   go build -tags cockroach ./server/router/api/v1/agent/
   ```

6. **Build without cockroach tag** (stub check):
   ```bash
   go build ./server/router/api/v1/agent/
   ```

7. **Test DDL manually** (with `IF NOT EXISTS` for defensive re-runs):
   ```sql
   CREATE TABLE IF NOT EXISTS agent_vectors (
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
   CREATE INDEX IF NOT EXISTS idx_agent_vectors_tenant ON agent_vectors (tenant_id);
   CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding);
   ```

8. **Verify vector index exists**:
   ```sql
   SHOW INDEXES FROM agent_vectors;
   -- Should show idx_agent_vectors_embedding with vector_l2_ops
   ```

9. **Test cosine distance query** (confirm `<=>` works with `vector_l2_ops` index):
   ```sql
   SELECT '[0.1, 0.2, 0.3]'::VECTOR(3) <=> '[0.1, 0.2, 0.3]'::VECTOR(3);
   -- Should return 0.0 (identical vectors)
   ```

10. **Run v26.2.4 verification** (per section 2)

11. **Stop CRDB**:
    ```bash
    docker compose -f scripts/docker-compose.cockroach.yml down
    ```

### Note on Test Coverage

No `vectordb_cockroach_test.go` file exists. The `go test -tags cockroach -run TestVectorDB` command in testing2.md matches zero tests and passes silently. Verification is via compile check (steps 5-6) and manual DDL testing (steps 7-9). Adding a cockroach-specific integration test file is a future task.

---

## 5. Adversarial Testing3 Review Prompt

Review the changes described in `testing3.md` for correctness, completeness, and risks. Check:

1. **Code fix correctness:** Is the `switch pgErr.Code` pattern correct? Does it handle all error codes gracefully? Is the non-PG error fallback (`else` branch) needed?

2. **v26.2 verification:** Is the Docker test sequence correct? Does `CREATE VECTOR INDEX ON test.vec (embedding vector_ip_ops)` produce error `0A000` in v26.2.4? Is port 26258/8081 correct to avoid conflict with v25.2 instance?

3. **Step ordering:** Is CRDB started before any SQL commands? Does `DROP TABLE IF EXISTS` work if the table doesn't exist yet?

4. **IF NOT EXISTS:** Is `CREATE TABLE IF NOT EXISTS` correct for the DDL test? Does `CREATE VECTOR INDEX` still need the 42P07 handler (it doesn't support IF NOT EXISTS)?

5. **Test coverage gap:** Is the compile-check approach sufficient? Should we add `vectordb_cockroach_test.go` as part of this plan or defer it?

6. **Reconciliation table:** After the code fix, should the code row in testing_review.md change from ❌ to ✅? Is this documented?

7. **Docker compose v2:** Is the compatibility note sufficient? Should we add a fallback for v1 users?

Return findings as a table with columns: #, Category, Severity (CRITICAL/HIGH/MEDIUM/NIT), Finding, Fix.
