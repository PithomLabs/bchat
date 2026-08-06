# Bug 060 — Local End-to-End Test Plan (DB-Agnostic)

**Status:** Planned — not yet executed
**Date:** 2026-08-06
**Agreement:** Confirmed with user — full driver matrix in plan; this cycle executes **CockroachDB only**; retrieval assertion is **API-primary** (admin RAG search) with driver-native row counts as secondary; adversarial review prompt lives in `plan_test_review_prompt.md`; keep `noli` tenant + data after the run; SQL admin-password reset allowed as fallback; OpenRouter real embeddings.

---

## 0. Purpose

Verify the Bug 060 fix end-to-end:

1. **Retrieval** from an indexed KB actually returns `kb_section` chunks (pre-fix: `content_type IN ('')` → zero results → silent 6k-token fallback).
2. **Answer** to "what did maria clara give to the leper?" is grounded in the indexed novel, not the fallback path.

The plan is written to be **reusable across database drivers**: sqlite, postgres (Neon), cockroach, mysql (future). Only the vector-store verification step (§6) is driver-specific; everything else runs identically through the HTTP API.

## 1. Layers

| Layer | Variability | Notes |
|---|---|---|
| HTTP API | Identical on all drivers | Routes, auth, body limits, CSRF, rate limits are driver-independent |
| App store | Driver-switched | `MEMOS_DRIVER` → `store/db/{sqlite,postgres,mysql}`; cockroach = `postgres.NewCockroachDB` (db.go:25) |
| Vector store | **Driver-specific** | CRDB: in-DB `agent_vectors` table; sqlite/neon: LanceDB (local/S3); mysql: no vector support |

## 2. CRDB facts validated via CockroachDB MCP knowledge

| Fact | Source | Impact on this plan |
|---|---|---|
| `<=>` = cosine **distance** (0..1); `1 - (embedding <=> $1)` = cosine similarity | MCP: VECTOR docs + semantic-search blog | App scoring math confirmed correct |
| Vector literal `'[1.0, 0.0, ...]'::VECTOR` | MCP: VECTOR docs | SQL checks in §6 use this form |
| `CREATE VECTOR INDEX` requires cluster setting `feature.vector_index.enabled = true` | MCP: vector-indexes docs | Already set in `scripts/crdb-init.sql`; local node v26.2.1 |
| Default opclass = `vector_l2_ops`; `<=>` needs `vector_cosine_ops` | MCP: vector-indexes docs | Existing `idx_agent_vectors_embedding` does **not** accelerate `<=>` → queries run brute-force but are **correct** → does not affect this test (Finding A, deferred to bugs/061) |
| Vector index only used when all prefix columns equality-constrained | MCP: prefix-columns section | Future index should be `(tenant_id, embedding vector_cosine_ops)` |
| Adding vector index to non-empty table blocks writes during backfill; needs `sql_safe_updates=false` | MCP: vector-indexes docs | Not applicable this cycle (index exists); note for fresh-DB runs |
| Large batch VECTOR inserts cause performance degradation — avoid batching | MCP: VECTOR docs (v25.2+) | Perf note for reindex step; do not batch-insert vector rows manually |
| VECTOR values recommended < 1 MB | MCP: VECTOR docs | Single chunk vectors are ~6 KB — no concern |

## 3. Driver support matrix

| Driver | Store agents | Vector storage | Run command | Status |
|---|---|---|---|---|
| sqlite | ✅ fully implemented (store/db/sqlite/agent.go) | LanceDB local | `task run:rag` (or `run`) | **Not run this cycle** |
| postgres (Neon) | ✅ fully implemented (store/db/postgres/agent.go) | LanceDB | no local task yet (fly.toml only) | **Not run this cycle** |
| cockroach | ✅ (via `postgres.NewCockroachDB`) | **in-DB** `agent_vectors` | `task run:cockroach` | **RUNS THIS CYCLE** |
| mysql | ❌ 99× `errNotImplemented` in store/db/mysql/agent.go | none | none | **Blocked** — agents unsupported (future) |

## 4. Env matrix (per driver)

| Driver | Build | Run | DSN/DB | Port | RAG flags |
|---|---|---|---|---|---|
| sqlite | `task build:backend:rag` | `task run:rag` | `build/data/memos_dev.db` (file) | 5230 | `LANCEDB_STORAGE_PROVIDER=local` (via task) |
| neon | `task build:backend:rag` | none local; needs `MEMOS_DRIVER=postgres` + `PG_DSN` | Neon URL | 5230 | same as sqlite |
| cockroach | `task build:backend:cockroach` | `task run:cockroach` | `COCKROACH_DSN` from `.env` | 5230 | `MEMOS_DRIVER=cockroach LANCEDB_STORAGE_PROVIDER=cockroach RAG_PIPELINE_ENABLED=true TICKET_EMBEDDING_ENABLED=true` |
| mysql | — | — | — | — | blocked |

`.env` is sourced by the run tasks (`set -a && . .env && set +a`) — contains `OPENROUTER_API_KEY`, `LLM_MODEL`, `EMBEDDING_PROVIDER`, `COCKROACH_DSN`, `BCHAT_USER`/`BCHAT_PASS`.

---

## 5. Executed run: CockroachDB

### Step 1 — Build and start server

```bash
task build:backend:cockroach      # -tags cockroach; includes fixed vectordb_cockroach.go
task run:cockroach                # sources .env; starts ./build/memos --mode dev on :5230
```

- Wait: `curl -s -o /dev/null -w "%{http_code}" http://localhost:5230/` → 200.
- Preflight SQL (read-only):
  ```sql
  SELECT version();                      -- expect v26.2.1 (VECTOR capable)
  SELECT id, slug FROM agent_tenants;    -- expect 5 rows; NO noli
  SELECT count(*) FROM agent_vectors;    -- expect 88 (unchanged baseline)
  ```

### Step 2 — Admin auth

```bash
curl -s -X POST http://localhost:5230/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$BCHAT_USER\",\"password\":\"$BCHAT_PASS\"}" | tee /tmp/signin.json
TOKEN=$(python3 -c "import json,sys; print(json.load(open('/tmp/signin.json'))['accessToken'])")
echo -n "$TOKEN" > /tmp/bchat_token
```

- **Fallback (user-approved):** if signin fails, reset local admin password via SQL:
  ```bash
  HASH=$(python3 -c "import bcrypt; print(bcrypt.hashpw(b'admin123', bcrypt.gensalt(10)).decode())")
  cockroach sql --url "postgresql://root@localhost:26257/bchat?sslmode=disable" \
    -e "UPDATE \"user\" SET password_hash='$HASH' WHERE username='admin';"
  ```
  then signin with `admin` / `admin123`. Local dev DB only; reversible.
- Verify: `GET /api/v1/auth/status` with Bearer → 200.
- Notes: CSRF middleware skips Bearer requests; admin mutations are per-tenant rate-limited (429 → wait and retry).

### Step 3 — Create tenant `noli` + upload KB (single multipart call)

```bash
curl -s -X POST http://localhost:5230/api/v1/agent/onboard \
  -H "Authorization: Bearer $TOKEN" \
  -F "tenant_slug=noli" \
  -F "company_name=Noli Me Tangere" \
  -F "vertical=literature" \
  -F "external_kb_file=@/home/chaschel/Documents/go/bchat/docs/templates/examples/rizal/noli.txt" \
  | tee /tmp/onboard.json
```

- Expect `{"success": true, "tenant": {"id": <NOLI_ID>, "slug": "noli"}, "audiences": {...}}`.
- `HandleOnboard` (handlers.go:1394) creates the tenant (auto guid + widget_key) and calls `importFiles` per audience with a KB file. `adminGroup` has **no body limit** (1 MB upload OK; the 16 KB limit is publicGroup-only, v1.go:292).
- Parse `NOLI_ID` from response; fetch widget key: `GET /api/v1/agent/noli/config` (Bearer) → `WIDGET_KEY`.
- **Isolation:** only `noli` is created; the 5 existing tenants are untouched. `RAG_STARTUP_REINDEX_DISABLED=true` prevents surprise reindexing at boot.

### Step 4 — Rebuild index

```bash
curl -s -X POST "http://localhost:5230/api/v1/agent/noli/reindex?audience_type=external" \
  -H "Authorization: Bearer $TOKEN"        # → 202 Accepted (async, 30-min window)
# Poll until done:
curl -s http://localhost:5230/api/v1/agent/noli/reindex/status -H "Authorization: Bearer $TOKEN"
```

- **Secondary evidence (SQL):** vector rows exist with correct content types:
  ```sql
  SELECT content_type, count(*)
  FROM agent_vectors WHERE tenant_id = <NOLI_ID>
  GROUP BY content_type;
  -- expect rows with content_type IN ('kb', 'kb_section'); kb_section > 0
  ```
- Perf note (MCP §2): do not manually batch-insert vector rows; app's own write path (EMBEDDING_BATCH_SIZE from `.env`) is what's exercised. Embedding cost ≈ $0.02 for the 1 MB novel (text-embedding-3-small via OpenRouter).

### Step 5 — Retrieval assertion (PRIMARY — fix proof)

```bash
curl -s -X POST http://localhost:5230/api/v1/admin/rag/search \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"query":"what did maria clara give to the leper?","tenantId":<NOLI_ID>,"topK":5,"audienceType":"external"}'
```

- **Pass criteria (DB-agnostic):** `results` non-empty; every chunk `contentType == "kb_section"`; titles/content reference mid-novel scenes (leper, alms/rosary, Elias), **not** the Translator's Introduction.
- This exercises `Service.SearchVectorDB` → `CockroachVectorDB.Search` → fixed `buildCockroachSearchQuery` (empty fileType → no `content_type` filter).

### Step 6 — End-to-end chat (customer view)

```bash
curl -s -X POST http://localhost:5230/api/v1/agent/noli/chat/ext \
  -H "X-Widget-Key: $WIDGET_KEY" -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$(uuidgen)\",\"message\":\"What did Maria Clara give to the leper?\"}"
```

- **Pass criteria:** reply content names the gift and the scene; server log shows **no** `RAG fallback` lines during the request; inspect response `metadata` for retrieval indicators; record reply + Step-5 chunks as evidence.
- Rationale: the pre-fix behavior (fallback to first 6k tokens) is documented in plan.md §1 — a purely positive run suffices; **no negative-control code revert** this cycle (agreed).

### Step 7 — Cleanup

- **Keep** tenant `noli` + indexed vectors (user decision). Stop server if no follow-ups.

## 6. Cross-driver verification adapters

**Primary assertion (identical on every driver):** `POST /api/v1/admin/rag/search` — same route, same payload, same response shape (`results[].chunk.contentType`, `score`). This is the pass/fail gate for reuse.

**Secondary evidence per driver:**

| Driver | Command | What to check |
|---|---|---|
| cockroach | `cockroach sql --url "$COCKROACH_DSN" -e "SELECT content_type, count(*) FROM agent_vectors WHERE tenant_id=<NOLI_ID> GROUP BY content_type;"` | `kb` + `kb_section` rows > 0 |
| sqlite | `sqlite3 build/data/memos_dev.db "SELECT content_type, count(*) FROM agent_vectors WHERE tenant_id=<NOLI_ID> GROUP BY content_type;"` (table exists in sqlite schema) | same as above |
| neon | `psql "$PG_DSN" -c "SELECT content_type, count(*) FROM agent_vectors WHERE tenant_id=<NOLI_ID> GROUP BY content_type;"` | same as above |
| mysql | — (blocked) | — |

## 7. Pass criteria (summary)

1. **DB-agnostic:** RAG-search results non-empty; all `contentType == kb_section`; chat reply grounded (gift + scene); zero `RAG fallback` log lines.
2. **Per-driver secondary:** vector row counts per §6.
3. **Isolation:** other tenants' data (5 tenants, 88 rows) unchanged before/after.

## 8. Not run / blocked this cycle

- **sqlite** — ready (run via `task run:rag` + same API steps); not run this cycle.
- **Neon** — needs local `MEMOS_DRIVER=postgres` + `PG_DSN` run config; not run this cycle.
- **MySQL** — agents unsupported (`errNotImplemented`, 99 call sites); blocked.

## 9. Adversarial review

Before execution, run the reviewer prompt in `bugs/060/plan_test_review_prompt.md` against this plan; record the verdict in `bugs/060/plan_test_review.md`. Execute only after verdict `GO` or `GO-WITH-CHANGES` (with changes applied).

## 10. Results (fill in after execution)

| Check | Result |
|---|---|
| Server + CRDB preflight (version, baseline rows) | |
| Signin OK (or SQL reset used) | |
| Onboard: tenant id / widget key / audiences.external | |
| Reindex status + `agent_vectors` counts (kb / kb_section) | |
| RAG search: results non-empty, all kb_section | |
| Top-1 chunk title | |
| Chat reply content (gift + scene) | |
| Fallback log lines (should be 0) | |
| Baseline isolation (5 tenants / 88 rows unchanged) | |
| Verdict | |
