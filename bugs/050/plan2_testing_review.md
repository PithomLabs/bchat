# Plan2 Testing Review

**Reviewer:** AI Agent
**Date:** 2026-07-29
**Status:** REWORK REQUIRED

---

## Summary

plan2_testing.md correctly addressed all 5 findings from plan_testing.md (feature flag, Docker tag, healthcheck port, image size, test coverage note). However, 3 new issues were found: 1 CRITICAL (table not auto-created on startup), 2 HIGH (missing auth in Step 9, wrong tenant slug).

---

## Findings

| # | Category | Severity | Finding | Fix |
|---|----------|----------|---------|-----|
| 1 | Schema | **CRITICAL** | Table `agent_vectors` is NOT auto-created on startup. `Validate()` is only called during reindex (`service.go:1244`), not during `NewService()` or `NewCockroachVectorDB()`. The auto-bootstrap path (`service.go:240-242`) calls `Stats()` which fails with "relation does not exist" and silently skips reindex. The troubleshooting entry "Run bchat once (it creates schema on startup via Validate())" is incorrect. | Fix troubleshooting. Add Step 7.5: either create table manually via `docker exec -it bchat-crdb cockroach sql --insecure -d bchat -e "CREATE TABLE IF NOT EXISTS agent_vectors (id STRING PRIMARY KEY, tenant_id INT NOT NULL, content_type STRING NOT NULL, title STRING, content TEXT NOT NULL, embedding VECTOR(1536), metadata JSONB, source_version INT DEFAULT 1, created_at TIMESTAMPTZ DEFAULT now())"` OR trigger reindex: `curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/reindex` (with auth cookie) |
| 2 | Auth | **HIGH** | Step 9 curl command lacks auth cookie/header. RAG search endpoint (`POST /api/v1/agent/:slug/rag/search`) requires ADMIN role via `AuthMiddleware` (`v1.go:382,417`). Both `adminGroup` and `ragGroup` require authentication. | Add auth to curl: `-H "Cookie: memos.access-token=YOUR_TOKEN"` or note that a valid admin session is required |
| 3 | Tenant slug | **HIGH** | Step 9 uses `acme-corp` but the seed script creates `hackathon-demo`. API returns 404 if `acme-corp` doesn't exist. | Change `acme-corp` to `hackathon-demo` in Step 9 curl |
| 4 | Silent failure | MEDIUM | Auto-bootstrap silently swallows the `Stats()` error. When `agent_vectors` table doesn't exist, `Stats()` returns error, `if err == nil` fails, no reindex is triggered, and no log message informs the user. | Add a note in the plan: "If vector DB remains empty, check startup logs for auto-bootstrap messages" |
| 5 | Image tags | NIT | Step 1-2 `docker run` uses `cockroachdb/cockroach:latest` but Step 5 docker-compose uses `v25.2.21`. | Use consistent tag on both paths |
| 6 | Env vars | NIT | Step 8 `task run:cockroach:seed` requires `COCKROACH_DSN` but plan doesn't mention this dependency. | Add note: "Ensure `COCKROACH_DSN` is set (from Step 5) or available in `.env`" |

---

## Verified Correct

| Item | Verdict |
|------|---------|
| Feature flag syntax `SET CLUSTER SETTING feature.vector_index.enabled = true` | ✅ Correct per CRDB v25.2 docs; no restart needed; persists across restarts via volume |
| Docker tag `v25.2.21` | ✅ Valid tag on Docker Hub (Jul 16, 2026) |
| DSN `postgresql://user:pass@localhost:26257/bchat?sslmode=disable` | ✅ Correct for pgx v5 + insecure mode |
| Healthcheck `cockroach node status --insecure --host=localhost --port=26257` | ✅ Valid command, `--port` flag added |
| Step ordering (feature flag before DB/user creation) | ✅ Cluster setting is cluster-wide, not database-scoped |
| `task build:backend:cockroach`, `task run:cockroach`, `task run:cockroach:seed` | ✅ All exist in Taskfile.yml |
| `--insecure` safe for local dev | ✅ |
| Volume persists across `docker compose down` (without `-v`) | ✅ |

---

## All 5 plan_testing.md Findings: ✅ Resolved

| # | Prior Finding | Status |
|---|---------------|--------|
| 1 | Missing `feature.vector_index.enabled` cluster setting | ✅ Added as Step 2.5 + docker-compose note + troubleshooting entry |
| 2 | Invalid Docker tag `v25.2.3` | ✅ Changed to `v25.2.21` (+ `latest` for docker run) |
| 3 | Healthcheck missing `--port=26257` | ✅ Added to docker-compose healthcheck |
| 4 | Wrong image size (800 MB → 180 MB) | ✅ Updated |
| 5 | Test note missing skip explanation | ✅ Added "NOTE: tests currently skip CRDB-dependent cases" |

---

## Verdict

**REWORK REQUIRED** — fix findings 1 (CRITICAL), 2 (HIGH), and 3 (HIGH) before deploying. Findings 4-6 are non-blocking. After fixes, ~30 lines changed across the plan file.
