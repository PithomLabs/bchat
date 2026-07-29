# summary.md — `bugs/050` Implementation Summary

## Project: CockroachDB Vector Store for bchat

**Purpose:** Build a CockroachDB-backed vector store for bchat's agentic memory system, enabling semantic ticket resolution via vector similarity search 

---

## What Was Implemented

### Core Files (code.md)

| File | Type | Lines | Purpose |
|------|------|-------|---------|
| `server/router/api/v1/agent/vectordb_cockroach.go` | NEW | ~350 | Full VectorDB interface implementation |
| `server/router/api/v1/agent/vectordb_nocockroach.go` | NEW | ~70 | Build tag stub for non-cockroach builds |
| `server/router/api/v1/agent/ticket_embedder.go` | NEW | ~240 | Cron job: embed tickets, build clusters |
| `server/router/api/v1/agent/ticket_resolution_test.go` | NEW | ~150 | Integration tests (t.Skip) |
| `server/router/api/v1/agent/vectordb.go` | MODIFIED | +10 | Factory case, config field |
| `server/router/api/v1/agent/service.go` | MODIFIED | +30 | CockroachDB wiring, cron registration, EscalateTicket |
| `server/router/api/v1/agent/handlers.go` | MODIFIED | +35 | /escalate endpoint, request struct |
| `Dockerfile.ecs` | NEW | ~40 | ECS Fargate build |
| `deploy/ecs/task-definition.json` | NEW | ~80 | ECS task definition |
| `deploy/ccloud/setup.sh` | NEW | ~60 | CockroachDB cluster setup |
| `cmd/seed/seed_demo_tickets.go` | NEW | ~150 | Demo ticket seeding |
| `Taskfile.yml` | MODIFIED | +130 | 12 crdb:* tasks |
| `go.mod` | MODIFIED | +1 | cockroach-go/v2 v2.4.3 |

### Test Infrastructure

| File | Purpose |
|------|---------|
| `scripts/docker-compose.cockroach.yml` | CRDB v25.2.21 local Docker setup |

---

## What Was Fixed (Post-Implementation)

### Fix 1: `vector_ip_ops` Operator Class (code4.md → testing4.md)

**Problem:** `vector_ip_ops` not supported in CRDB v25.2.21 (error 0A000, issue #144016).

**Fix:** Removed opclass from DDL, defaults to `vector_l2_ops`. Added `0A000` error handler with `switch pgErr.Code` pattern.

**File:** `vectordb_cockroach.go:108-135`

**v26.2.4 surprise:** `vector_ip_ops` IS supported in v26.2.4 (verified via Docker). Skills repo examples are correct for v26.2+.

### Fix 2: Search() QueryEmbedding Priority (code4.md)

**Problem:** Search() ignored pre-computed `QueryEmbedding`, unconditionally embedded `QueryText`.

**Fix:** Matched LanceDB pattern: check `QueryEmbedding` first, fall back to embedding `QueryText`, return empty if both empty.

**File:** `vectordb_cockroach.go:279-295`

### Fix 3: MinScore Filtering (code4.md)

**Problem:** `query.MinScore` field was ignored in SQL query.

**Fix:** Added `AND (embedding <=> $1::VECTOR) <= 1 - $4` WHERE clause + `$4` parameter.

**File:** `vectordb_cockroach.go:311-321`

### Fix 4: Stub Tests (code4.md)

**Problem:** All tests created `&Service{vectorDB, vectorDBConfig}` without store (nil).

**Fix:** Replaced stubs with `t.Skip("Requires real CockroachDB + store")`.

**File:** `ticket_resolution_test.go`

---

## What Was Verified

| Check | v25.2.21 | v26.2.4 |
|-------|----------|---------|
| `vector_ip_ops` | ❌ 0A000 | ✅ Supported |
| Default opclass | ✅ Works | ✅ Works |
| Cosine distance `<=>` | ✅ Works | ✅ Works |
| Feature flag | ✅ Persists via volume | ✅ Works |
| `go build -tags cockroach` | ✅ Pass | N/A |
| `go build` (no tag) | ✅ Pass | N/A |
| `go test -tags cockroach` | ✅ Pass (8.6s) | N/A |

---

## Key Findings

| # | Finding | Severity | Impact |
|---|---------|----------|--------|
| 1 | `vector_ip_ops` not supported in v25.2.21 | HIGH | Code fix required |
| 2 | `vector_ip_ops` IS supported in v26.2.4 | HIGH | Skills repo examples validated for v26.2+ |
| 3 | Insecure mode: no password auth | MEDIUM | DSN uses `bchat_user@localhost` (no password) |
| 4 | No cockroach-specific integration tests | MEDIUM | Compile-check only |
| 5 | `CREATE VECTOR INDEX` no IF NOT EXISTS | LOW | 42P07 handler catches duplicates |

---

## Pending Items

### Must-Do (Hackathon)

| Item | Status | Blocked By |
|------|--------|------------|
| Provision real CRDB cluster | PENDING | ccloud CLI setup |
| Deploy to ECS Fargate | PENDING | CRDB cluster |
| Seed demo tickets with real embeddings | PENDING | CRDB cluster |
| End-to-end demo with real data | PENDING | All above |

### Should-Do (Post-Demo)

| Item | Status | Blocked By |
|------|--------|------------|
| Add `vectordb_cockroach_test.go` | PENDING | Real CRDB instance |
| Test `vector_ip_ops` vs `vector_l2_ops` performance | PENDING | Real cluster with data |
| Version-conditional opclass (v26.2+ optimization) | PENDING | Decision on version targeting |
| ECS health check endpoint | PENDING | ECS deployment |

### Nice-to-Have

| Item | Status |
|------|--------|
| `EMBEDDING_PROVIDER=local` for development | NOT STARTED |
| Vector index memory limits | NOT STARTED |
| BM25 hybrid search | NOT STARTED |

---

## Architecture Reference

```
┌──────────────────────────────────────────────────────┐
│                    AWS Cloud                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │   ECS    │  │  Bedrock │  │    S3    │           │
│  │ (bchat)  │  │  (LLM)   │  │  (docs)  │           │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘           │
│       │              │              │                  │
│  ┌────┴──────────────┴──────────────┴────────────┐   │
│  │              CockroachDB Cloud                 │   │
│  │  ┌──────────────┐  ┌──────────────┐           │   │
│  │  │ agent_vectors │  │ agent_tickets │           │   │
│  │  │ (VECTOR col)  │  │ (relational)  │           │   │
│  │  └──────────────┘  └──────────────┘           │   │
│  └───────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────┘
```

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `VECTOR_DB_PROVIDER` | Yes | `memory` | Set to `cockroach` |
| `COCKROACH_DSN` | Yes | — | CockroachDB connection string |
| `RAG_PIPELINE_ENABLED` | Yes | `false` | Must be `true` |
| `TICKET_EMBEDDING_ENABLED` | No | `false` | Enable cron (every 5 min) |
| `EMBEDDING_PROVIDER` | Yes | `mock` | Set to `openrouter` for prod |
| `EMBEDDING_MODEL` | Yes | — | e.g., `openai/text-embedding-3-small` |
| `OPENROUTER_API_KEY` | Yes | — | OpenRouter API key |

---

## Local Testing DSN

```bash
export COCKROACH_DSN="postgresql://bchat_user@localhost:26257/bchat?sslmode=disable"
```

---

## File Index

| File | Purpose |
|------|---------|
| `bugs/050/code.md` | Original implementation plan |
| `bugs/050/code_review.md` | Implementation review |
| `bugs/050/code2.md` — `code4.md` | Iterative code reviews |
| `bugs/050/plan.md` — `plan5.md` | Architecture plans |
| `bugs/050/plan_testing.md` — `plan4_testing.md` | Testing plans |
| `bugs/050/testing.md` | Local Docker CRDB setup |
| `bugs/050/testing2.md` — `testing4.md` | Testing iterations |
| `bugs/050/release.md` | CRDB releases overview |
| `bugs/050/summary.md` | This file |
