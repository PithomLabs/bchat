# Plan3 Testing Review

**Reviewer:** AI Agent
**Date:** 2026-07-29
**Status:** APPROVED WITH NITS

---

## Summary

plan3_testing.md correctly addresses all 6 findings from plan2_testing_review.md. Only 3 new issues found: 1 HIGH (wrong vector index DDL syntax — `USING vector` is pgvector syntax, not CRDB), 2 MEDIUM. All other items verified correct.

---

## Findings

| # | Category | Severity | Finding | Fix |
|---|----------|----------|---------|-----|
| 1 | DDL syntax | **HIGH** | Step 7.5 Option A line 199 uses `USING vector (embedding)` — that's **pgvector syntax**, not CRDB. CRDB v25.2 uses `CREATE VECTOR INDEX name ON table (column [opclass])`. The `USING vector` clause will produce a syntax error. Code at `vectordb_cockroach.go:112-113` uses `ON agent_vectors (embedding vector_ip_ops)`. | Change to: `CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding vector_ip_ops);` |
| 2 | `IF NOT EXISTS` | MEDIUM | Step 7.5 Option A uses `IF NOT EXISTS` for `CREATE VECTOR INDEX`. Code comment at `vectordb_cockroach.go:108-110` explicitly says: "CREATE VECTOR INDEX does NOT support IF NOT EXISTS (verified against 01-schema-design.md:152)". The code catches 42P07 as fallback instead. | Remove `IF NOT EXISTS` to match code: `CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding vector_ip_ops);` |
| 3 | Env vars | MEDIUM | Step 5 Option B (docker-compose, line 146) does NOT set `EMBEDDING_PROVIDER=mock`. Default is `openrouter` (`embedding.go:157`). Option A correctly sets it. Option B users without `OPENROUTER_API_KEY` will get an error. | Add `EMBEDDING_PROVIDER=mock` to Step 5 Option B env setup |

---

## All 6 Prior Findings: ✅ Resolved

| # | Prior Finding | Status |
|---|---------------|--------|
| 1 | Table not auto-created on startup (CRITICAL) | ✅ Step 7.5 added with Option A (manual table) + Option B (reindex) |
| 2 | Step 9 missing auth cookie (HIGH) | ✅ Added `-H "Cookie: memos.access-token=YOUR_ADMIN_TOKEN"` |
| 3 | Wrong tenant slug `acme-corp` (HIGH) | ✅ Changed to `hackathon-demo` |
| 4 | Auto-bootstrap silent failure (MEDIUM) | ✅ Documented in troubleshoot + note |
| 5 | Mixed image tags (NIT) | ✅ Both use `v25.2.21` |
| 6 | Seed script env var dependency (NIT) | ✅ Step 8 shows inline `COCKROACH_DSN=` |

---

## Verified Correct

| Item | Verdict |
|------|---------|
| Table DDL matches `vectordb_cockroach.go:84-95` exactly | ✅ |
| DSN format for pgx v5 + insecure | ✅ |
| Auth middleware at `v1.go:382,417` — Step 9 now includes cookie | ✅ |
| Tenant slug `hackathon-demo` matches seed script | ✅ |
| Feature flag persists across restarts via volume | ✅ |
| `--insecure` safe for local dev | ✅ |
| Volume persists across `docker compose down` (without `-v`) | ✅ |
| All 3 Taskfile tasks exist | ✅ |

---

## Verdict

**APPROVED WITH NITS** — fix findings 1 (HIGH), 2 (MEDIUM), 3 (MEDIUM) before deploying. After fixes, ~3 lines changed across the plan file.
