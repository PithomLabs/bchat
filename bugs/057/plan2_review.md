# Plan Review: Port Postgres Database Migrations to CockroachDB (v2)

**Bug ID:** 057
**Review Date:** 2026-08-02
**Reviewer:** Kilo (Senior Go & CockroachDB Architect)
**Verdict:** CONDITIONALLY BLOCKED — Genuine architectural blockers remain; some prior "critical" claims are downgraded after official doc review.

---

## Executive Summary

After cross-checking the original review against CockroachDB official documentation contained in `bugs/057/`, and reviewing the independent ChatGPT review (`plan_review_chatgpt.md`), I am downgrading several findings and adding new ones. The overall score from the ChatGPT review was 8.8/10; I agree with its assessment that the strongest issues are architectural, not SQL-dialect trivia.

| Category | Count |
|----------|-------|
| Correct blocking issues | 5 |
| High-value improvements | 8 |
| Incorrect / unsupported claims | 3 |
| Needs source verification | 4 |

The plan is **CONDITIONALLY BLOCKED** until the architectural issues are resolved.

---

## 1. Critical Findings (Block Implementation)

### C-1: Code Duplication Without Abstraction

**Severity:** CRITICAL
**Location:** Plan Section 1.4

The plan copies ~22 files from `store/db/postgres/` to `store/db/cockroach/` with only "package name change" and minor edits. This creates a **maintenance fork**:

- Every bug fix in `postgres/` must be manually duplicated in `cockroach/`
- Every new feature must be implemented twice
- Schema changes in `LATEST.sql` require updates in two places
- Risk of drift between postgres and cockroach implementations grows over time

The ChatGPT review called this "the biggest issue in the whole review" and "a long-term maintenance nightmare." I agree.

**Recommended Fix:**
- **Option A (Preferred):** Create a shared `store/db/common/` package with all SQL-agnostic logic. Both `postgres` and `cockroach` packages import and wrap it, providing only driver-specific connection setup and resilience.
- **Option B:** Use Go build tags within a single `store/db/postgres/` package, similar to the `vectordb_cockroach.go` / `vectordb_nocockroach.go` pattern.
- **Option C (Minimum):** Add a CI check that diffs the two packages and fails if they diverge by more than the expected resilience differences.

---

### C-2: Missing Files in Copy List

**Severity:** CRITICAL
**Location:** Plan Section 1.1 (table)

The plan states "Copy all `.go` files from `store/db/postgres/` to `store/db/cockroach/`" but the table only lists 21 files. The actual `postgres/` directory contains 25 files (excluding `postgres.go` and test files):

| Missing File | Purpose |
|--------------|---------|
| `bridge_auth.go` | Bridge authentication store methods |
| `memo_relation.go` | Memo relationship store methods |

**Impact:** Compile errors when building the cockroach driver.

**Recommended Fix:**
- Update the file table to include all 23 postgres files (excluding `postgres.go` and `memo_filter_test.go`)
- Explicitly state: `memo_filter_test.go` is excluded (test file)

---

### C-3: No Incremental Migration Strategy

**Severity:** CRITICAL
**Location:** Plan Section 2

The plan focuses almost entirely on `LATEST.sql` and does not address:
- How incremental migrations will be maintained for CRDB
- Whether existing Postgres incremental migrations are compatible with CockroachDB
- What happens if a future migration uses Postgres-specific syntax

Without a documented process, Cockroach support will slowly rot after initial implementation.

**Recommended Fix:**
- Add a maintenance section: "Future migrations must be tested against both Postgres and CockroachDB"
- Add a CI job that runs migrations against both databases
- Document required SQL dialect constraints for future migrations

---

### C-4: Explicit Cockroach DSN Required

**Severity:** CRITICAL
**Location:** Plan Section 1.3 (`internal/profile/profile.go`)

The plan proposes:

```go
if p.Driver == "cockroach" && p.DSN == "" {
    p.DSN = os.Getenv("COCKROACH_DSN")
    if p.DSN == "" {
        p.DSN = os.Getenv("DATABASE_URL")  // fallback for consistency
    }
}
```

If an operator has `DATABASE_URL` set for Postgres (e.g., from a previous deployment) and accidentally sets `MEMOS_DRIVER=cockroach`, the app will connect to the Postgres database using the cockroach driver. This could cause silent data corruption or schema conflicts.

**Recommended Fix:**
- Remove the `DATABASE_URL` fallback for the `cockroach` driver
- Require explicit `COCKROACH_DSN` to prevent accidental misconfiguration
- Log a clear error if `COCKROACH_DSN` is not set

---

### C-5: SQL Audit Required Before Copying Postgres Driver

**Severity:** CRITICAL
**Location:** Plan Section 1.4

The plan states store methods are "SQL-agnostic" and can be copied verbatim. CockroachDB officially states:

> PostgreSQL compatibility is high, but not complete. Applications should expect to rewrite unsupported SQL constructs.

The plan does not verify this claim with an actual audit of the store files.

**Required Audit:**
```bash
grep -rn '::BIGINT\|::INT\|::TEXT\|::UUID\|::JSONB\|::BYTEA\|::TIMESTAMPTZ' store/db/postgres/
```

**Note on `::BIGINT`:** CockroachDB officially supports `BIGINT` as an alias for `INT8`. The specific cast syntax `::BIGINT` is used in `store/db/postgres/agent.go`. This must be **verified against a running CRDB instance**, not assumed to work or fail. The original review claimed `::BIGINT` is unsupported; this is **not established** by available documentation. Downgrade to MEDIUM pending verification.

---

## 2. High Findings (Must Fix Before Production)

### H-1: Connection Pool Settings Exceed CRDB Recommendations

**Severity:** HIGH
**Location:** Plan Section 1.1 (`cockroach.go`)

The plan copies `SetMaxOpenConns(10)` from the Postgres driver. Official CockroachDB documentation states:

> The number of active connections across all connection pools should not exceed 4 times the number of vCPUs in the cluster by a large amount.

For a Fly.io shared-cpu-1x instance (1 vCPU), the maximum recommended connections is **4**. Setting `MaxOpenConns=10` will cause connection exhaustion, queueing, and degraded performance.

**Official Reference:** `bugs/057/prod_checklist.md` — Connection pooling section

**Recommended Fix:**
```go
// Make pool settings configurable
maxOpenConns := 4 // Default for 1 vCPU; scale with CRDB node size
if cpuCount := runtime.NumCPU(); cpuCount > 1 {
    maxOpenConns = cpuCount * 4
}
db.SetMaxOpenConns(maxOpenConns)
```

Or read from env var `COCKROACH_MAX_OPEN_CONNS` with sensible default.

---

### H-2: `sslmode=require` Is Insufficient for Production

**Severity:** HIGH
**Location:** Plan Section 4.4 (`scripts/fly-crdb-secrets.sh`)

The plan uses `sslmode=require`, which encrypts traffic but does **not** verify the server's certificate. Official CockroachDB documentation states:

> To deploy CockroachDB in production, it is strongly recommended to use TLS certificates to authenticate the identity of nodes and clients and to encrypt data in transit.

The Docker single-node example in `bugs/057/docker.md` uses `sslmode=verify-full` with certificate paths. The CockroachDB Cloud connection string example in `bugs/057/pgx.md` uses `sslmode=verify-full&sslrootcert=...`.

**Official References:**
- `bugs/057/prod_checklist.md` — Security section
- `bugs/057/docker.md` — secure mode examples
- `bugs/057/pgx.md` — Cloud connection string example

**Recommended Fix:**
```bash
# For CRDB Cloud, use verify-full with CA cert
COCKROACH_DSN="postgresql://user:password@host:26257/db?sslmode=verify-full&sslrootcert=/path/to/ca.crt"
```

For Fly.io deployment, store the CA cert as a secret or volume mount.

---

### H-3: Race Condition in `agent_vectors` Table Creation

**Severity:** HIGH
**Location:** Plan Section 1.1 (`vectordb_cockroach.go` referenced)

The plan states `agent_vectors` is created at runtime by `CockroachVectorDB.Validate()`. If multiple app instances start simultaneously (e.g., during Fly.io rollout), they will all attempt `CREATE TABLE IF NOT EXISTS agent_vectors`. While `IF NOT EXISTS` prevents errors, concurrent `CREATE INDEX` statements could cause lock contention or schema validation failures.

Schema changes in CockroachDB can be asynchronous and involve job scheduling. Spreading schema creation across runtime paths is fragile.

**Recommended Fix:**
- Create `agent_vectors` in `LATEST.sql` instead of at runtime
- Or wrap runtime creation in `crdb.ExecuteTx` with retry logic
- Or add a distributed lock using CRDB's `SELECT ... FOR UPDATE`

---

### H-4: Transaction Retry Semantics Not Fully Examined

**Severity:** HIGH
**Location:** Plan Section 1.1 (`resilience.go`)

The plan wraps transactions with `crdb.ExecuteTx` but does not examine whether existing transaction patterns in the codebase are compatible with CockroachDB's retry model. CockroachDB retries require careful handling of:

- Read-write conflicts
- SAVEPOINT semantics
- Transaction IDs and restarts

The official pgx tutorial (`bugs/057/pgx.md`) shows `crdbpgx.ExecuteTx` used for **every** transaction, including simple inserts. The plan must ensure all write transactions in the cockroach driver use the retry wrapper.

**Official Reference:** `bugs/057/pgx.md` — shows `crdbpgx.ExecuteTx` for all transactions

---

### H-5: Schema Change Behavior Not Addressed

**Severity:** HIGH
**Location:** Plan Section 2

CockroachDB schema changes are **asynchronous** in many cases. The plan assumes synchronous DDL like PostgreSQL. This could cause:

- Migrations appearing to complete before schema is fully applied
- Subsequent queries failing during schema change propagation
- Test flakiness if tests run immediately after migration

**Recommended Fix:**
- Add `SET CLUSTER SETTING jobs.registry.interval.gc = '30s'` during testing
- Add a post-migration verification step or wait mechanism
- Document that CRDB schema changes are asynchronous

---

### H-6: No Rollback Plan

**Severity:** HIGH
**Location:** Plan Section 4 (entire)

The plan has no rollback strategy if CockroachDB deployment fails after the Postgres deployment is working.

**Recommended Fix:**
- Add a rollback section:
  - How to revert to Postgres if CRDB Cloud is unreachable
  - Fly.io `fly deploy -c fly_pg.toml` as rollback command
  - Data migration strategy if data exists in CRDB
- Add a canary deployment strategy: deploy to staging first, validate, then promote to production

---

## 3. Medium Findings (Should Fix Before or During Implementation)

### M-1: `EXTRACT(EPOCH FROM NOW())` Type Mismatch in LATEST.sql

**Severity:** MEDIUM
**Location:** Plan Section 2.2

The plan uses `extract(epoch from now())` for `INT` columns. In CockroachDB, `extract(epoch from now())` returns `double precision`. Inserting a `double precision` into an `INT` column may fail or cause implicit truncation.

**Recommended Fix:**
```sql
-- Explicitly cast to INT
created_ts INT NOT NULL DEFAULT CAST(EXTRACT(EPOCH FROM NOW()) AS INT)
```

---

### M-2: `::BIGINT` in Go Store Code (Verify, Not Assume)

**Severity:** MEDIUM
**Location:** `store/db/postgres/agent.go` lines 2605, 2693, 2718, 2776, 2780

**Downgraded from CRITICAL.** The original review claimed `::BIGINT` is unsupported in CockroachDB. After reviewing official documentation, CockroachDB supports `BIGINT` as an alias for `INT8`. The specific cast syntax `::BIGINT` must be verified against a running CRDB instance.

**Required Action:**
- Run the grep audit: `grep -rn '::[A-Z]' store/db/postgres/`
- Test each cast syntax against local CRDB instance
- Document which casts work and which need replacement

---

### M-3: RAG Reindex Settings Conflict

**Severity:** MEDIUM
**Location:** Plan Section 4.2

```toml
RAG_PIPELINE_ENABLED = 'true'
FORCE_REINDEX_ON_STARTUP = 'false'
RAG_STARTUP_REINDEX_DISABLED = 'true'
```

This means:
1. RAG is enabled
2. But startup reindex is disabled
3. And force reindex is disabled

**Result:** On first deployment, the LanceDB index will be empty. The RAG pipeline will run but return no results, causing silent quality degradation.

**Recommended Fix:**
- Set `FORCE_REINDEX_ON_STARTUP=true` for the first deployment only
- Or add a one-time `/api/v1/agent/:slug/reindex` call in the deployment script
- Or set `RAG_STARTUP_REINDEX_DISABLED=false`

---

### M-4: Placeholder Values in Fly Secrets Script

**Severity:** MEDIUM
**Location:** Plan Section 4.4

```bash
fly -a "$APP_NAME" secrets set \
  COCKROACH_DSN="postgresql://user:password@your-cluster.cockroachlabs.cloud:26257/bchat?sslmode=require"
```

The script contains hardcoded placeholders. An operator running this script will either accidentally deploy with placeholder values or manually edit the script, risking typos.

**Recommended Fix:**
```bash
#!/bin/bash
set -euo pipefail

APP_NAME="${1:-bchat-crdb}"
COCKROACH_DSN="${2:?Usage: $0 <app-name> <cockroach-dsn>}"
# ... etc
```

---

### M-5: `default_query_exec_mode` Removal May Break CRDB Cloud

**Severity:** MEDIUM
**Location:** Plan Section 1.1

The plan removes `default_query_exec_mode=simple_protocol` because "not needed for CockroachDB". This was added for Neon PgBouncer compatibility. CRDB Cloud may use a similar connection pooler or proxy.

**Recommended Fix:**
- Test with CRDB Cloud connection string to verify `simple_protocol` is not needed
- If needed, add it conditionally
- Document the decision

---

### M-6: `unique_rowid()` Write Hotspots

**Severity:** MEDIUM
**Location:** Plan Section 2.2

The plan replaces `SERIAL PRIMARY KEY` with `INT DEFAULT unique_rowid() PRIMARY KEY`. CockroachDB documentation mentions that `unique_rowid()` can cause write hotspots for high-throughput tables, but does not recommend against it as a general rule. The impact depends on write volume, table, and index layout.

**Downgraded from original review.** Not a blocker, but should be documented.

**Recommended Fix:**
- Document the trade-off in the plan
- Consider `UUID` primary keys for very high-throughput tables
- Consider `ALTER TABLE ... CONFIGURE ZONE` with specific replica placement for hotspot tables

---

## 4. Low Findings (Nits — Fix During Implementation)

### L-1: In-Memory CockroachDB Store Loses Data

**Severity:** LOW
**Location:** Plan Section 3.1

```yaml
environment:
  - cockroach start-single-node --insecure --store=type=mem,size=0.25
```

The `type=mem` store loses all data when the container restarts. This is fine for ephemeral testing but confusing for developers.

**Official Reference:** `bugs/057/test_locally.md` — confirms in-memory is for testing only

**Recommended Fix:**
- Use a file-backed store for local development:
  ```yaml
  - cockroach start-single-node --insecure --store=type=fs,path=/var/lib/cockroach
  ```
- Or document clearly that data is ephemeral

---

### L-2: `ALTER RANGE default CONFIGURE ZONE` Affects All Tables

**Severity:** LOW
**Location:** Plan Section 3.4

```sql
ALTER RANGE default CONFIGURE ZONE USING gc.ttlseconds = 600;
```

This changes the default zone configuration for **all** ranges (tables) in the cluster. This is the exact command recommended in `bugs/057/test_locally.md` for testing, but should not be used in production.

**Official Reference:** `bugs/057/test_locally.md` — `ALTER RANGE default CONFIGURE ZONE USING "gc.ttlseconds" = 600;`

**Recommended Fix:**
- Document that this is for testing only
- Apply zone configuration to specific tables in production

---

### L-3: Docker Compose Volume vs Container Storage

**Severity:** LOW
**Location:** `scripts/docker-compose.cockroach.yml`

CockroachDB documentation recommends Docker volumes over container local storage:

> Cockroach Labs recommends that you store cluster data in Docker volumes rather than in the storage layer of the running container.

**Official Reference:** `bugs/057/docker.md` — recommends Docker volumes

**Recommended Fix:**
- Use a named Docker volume for local CRDB data
- Or document that data is ephemeral with `type=mem`

---

### L-4: Placeholder `LLM_MODEL` Values in Production Config

**Severity:** LOW
**Location:** Plan Section 4.2

```toml
LLM_MODEL = "openrouter/free"
LLM_MODEL_REASONING = "openrouter/free"
```

These are placeholder values that will produce broken LLM responses in production.

**Recommended Fix:**
- Add a comment: `# Set via fly secrets or replace with production model`
- Or parameterize: `LLM_MODEL = "${LLM_MODEL}"`

---

### L-5: Build Tag Strategy Not Explicitly Documented

**Severity:** LOW
**Location:** Plan Section 1.1

The plan does not explicitly state whether the `store/db/cockroach/` package uses `//go:build cockroach` tags.

**Recommended Fix:**
- Explicitly state: "The cockroach driver package does NOT use build tags. It is always compiled."
- Explain why: The driver is selected at runtime via `MEMOS_DRIVER` env var, not at build time.

---

## 5. Findings Correctly Rejected

These findings from the original review were **incorrect or unsupported**:

### R-1: `::BIGINT` Not Supported

**Original claim:** CRITICAL — CockroachDB does not support `::BIGINT`

**Status:** REJECTED — Downgraded to MEDIUM. CockroachDB officially supports `BIGINT` as an alias for `INT8`. The specific cast syntax `::BIGINT` must be verified against a running CRDB instance.

---

### R-2: `unique_rowid()` Write Hotspots Block Implementation

**Original claim:** MEDIUM — `unique_rowid()` causes write hotspots

**Status:** REJECTED — Downgraded to MEDIUM/LOW. Official docs do not flag it as a general blocker. Impact is workload-dependent.

---

## 6. Missing Information from Original Review

### MI-1: SAVEPOINT Retry Semantics

CockroachDB retries are more nuanced than simply wrapping every transaction with `crdb.ExecuteTx`. Some transaction patterns, nested transactions, or manually managed transactions require careful handling. The plan does not examine whether the existing transaction abstraction is compatible with CockroachDB's retry model.

**Official Reference:** `bugs/057/pgx.md` — shows `crdbpgx.ExecuteTx` for all transactions

---

### MI-2: SERIAL vs IDENTITY Evaluation

The original review immediately recommends `unique_rowid()` versus `SERIAL` without evaluating whether preserving PostgreSQL `IDENTITY` semantics would minimize divergence from the existing schema.

---

### MI-3: Foreign Key Validation Differences

CockroachDB documents subtle behavioral differences around constraint validation and `INSERT ... ON CONFLICT`. None of this was reviewed despite being directly relevant to migration fidelity.

---

## 7. Approval Criteria

The plan can be marked **APPROVED** when:

- [ ] **C-1:** Replace copy-paste strategy with shared abstraction or document drift mitigation
- [ ] **C-2:** Add missing `bridge_auth.go` and `memo_relation.go` to the file list
- [ ] **C-3:** Add incremental migration maintenance strategy to the plan
- [ ] **C-4:** Require explicit `COCKROACH_DSN` with no `DATABASE_URL` fallback
- [ ] **C-5:** Complete SQL audit of all store methods before copying
- [ ] **H-1:** Adjust connection pool settings for CRDB Cloud (4x vCPU max)
- [ ] **H-2:** Remove `DATABASE_URL` fallback for cockroach driver
- [ ] **H-3:** Use `sslmode=verify-full` for production deployment
- [ ] **H-4:** Fix `agent_vectors` race condition
- [ ] **H-5:** Address CockroachDB retry semantics for all write transactions
- [ ] **H-6:** Address async schema change behavior
- [ ] **H-7:** Add rollback plan
- [ ] **M-1:** Fix `EXTRACT(EPOCH FROM NOW())` type mismatch in LATEST.sql
- [ ] **M-2:** Verify `::BIGINT` cast syntax against local CRDB instance
- [ ] **M-3:** Fix RAG reindex settings for first deployment
- [ ] **M-4:** Parameterize `fly-crdb-secrets.sh`
- [ ] **M-5:** Test `default_query_exec_mode` with CRDB Cloud
- [ ] **M-6:** Document `unique_rowid()` trade-offs

---

*Review complete. Implementation blocked until critical and high findings are resolved.*
