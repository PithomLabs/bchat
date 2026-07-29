# Plan Testing Review

**Reviewer:** AI Agent
**Date:** 2026-07-29
**Status:** REWORK REQUIRED

---

## Summary

plan_testing.md is a well-structured local Docker testing guide. However, 2 blocking issues were found: a missing critical cluster setting and an invalid Docker tag.

---

## Findings

| # | Category | Severity | Finding | Fix |
|---|----------|----------|---------|-----|
| 1 | Setup | **CRITICAL** | Missing `SET CLUSTER SETTING feature.vector_index.enabled = true` after cluster start. Without it, `CREATE VECTOR INDEX` fails (code degrades to brute-force). The code at `vectordb_cockroach.go:111-127` catches 42P07 and logs a warning, but the plan should enable vector indexes for proper testing. | Add Step 2.5 after cluster start: `docker exec -it bchat-crdb cockroach sql --insecure --host=localhost -e "SET CLUSTER SETTING feature.vector_index.enabled = true;"` |
| 2 | Docker Image | **HIGH** | `cockroachdb/cockroach:v25.2.3` tag does NOT exist on Docker Hub. Available v25.2 tags: v25.2.0, v25.2.1, ..., v25.2.21 (latest, Jul 16, 2026). v25.2.3 is missing from the sequence. | Change to `cockroachdb/cockroach:v25.2.21` |
| 3 | Healthcheck | MEDIUM | `cockroach node status --insecure --host=localhost` may fail without `--port=26257` if healthcheck runs before cluster initialization completes. | Add `--port=26257` to healthcheck: `["CMD", "cockroach", "node", "status", "--insecure", "--host=localhost", "--port=26257"]` |
| 4 | Image size | MEDIUM | Plan says ~800 MB; Docker Hub shows compressed size is ~180 MB (amd64, v25.2.21). | Update note to `~180 MB` or remove size note |
| 5 | Test coverage | NIT | Step 8 `go test -tags cockroach` will pass but all tests use `t.Skip` (see code4 N-2). No real integration coverage. | Add note: "Tests currently skip CRDB-dependent cases — manual verification required" |

---

## Verified Correct

| Item | Verdict |
|------|---------|
| DSN format: `postgresql://user:pass@localhost:26257/db?sslmode=disable` with pgx v5 + insecure | ✅ Correct |
| Ports: 26257 (SQL) + 8080 (DB Console) | ✅ Correct |
| `CREATE TABLE IF NOT EXISTS` auto-creation on startup via `Validate()` at `service.go:1244` | ✅ Verified |
| `--advertise-addr=localhost` for native app → Docker connection | ✅ Correct |
| Volume mount persistence: `-v bchat_crdb_data:/cockroach/cockroach-data` | ✅ Correct |
| `--insecure` is safe for local dev | ✅ Correct |
| `/m/` prefix in seed script matches `EscalateTicket` convention | ✅ Verified |

---

## Verdict

**REWORK REQUIRED** — fix findings 1 (CRITICAL) and 2 (HIGH) before deploying. Findings 3-5 are non-blocking but should be addressed for accuracy. After fixes, ~25 lines changed across the plan file.
