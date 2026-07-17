# Adversarial Review: bchat E2E Critical Path Testing

**Reviewer:** AI Agent (Senior Go Architect)
**Date:** 2026-07-17
**Status:** Rework Required

---

## Summary

The plan covers the right feature areas but has **one structural flaw** that breaks test sequencing, **two endpoint mismatches**, and several coverage gaps that should be addressed before implementation.

---

## Must Rework

### 1. Test 2.4 destroys the tenant that Groups 3-9 depend on (HIGH)

Test 2.4 (`DELETE /api/v1/agent/:slug`) deletes the tenant, but the execution flow shows Groups 3-9 using the same tenant afterwards. The execution flow and test table are contradictory.

**Options:**
- Remove 2.4 from Group 2 entirely (10.1 already covers cleanup)
- Move 2.4 to Group 10 and merge with 10.1
- Keep 2.4 but recreate the tenant afterwards before continuing

The simplest fix: delete 2.4 from the test table since 10.1 already exists.

### 2. Import endpoint parameters undocumented (MEDIUM)

Test 3.1/3.2 uses `POST /api/v1/agent/:slug/import`. The registered handler is `HandleImportSingleFile` — it requires **three** form fields:
- `audience_type` (e.g., `default`)
- `file_type` (`kb` or `policy`)
- `file` (the actual file content)

Without documenting these required params in the plan, the test scripts will fail on first write. The plan should specify the exact curl invocation.

### 3. Simulation stream route param mismatch (LOW)

Test 7.2 documents the route as `GET /api/v1/agent/:slug/simulate/:id/stream`. The registered route uses `:sessionId`, not `:id`:
```
GET /api/v1/agent/:slug/simulate/:sessionId/stream
```

---

## Coverage Gaps

### Missing: Health check endpoint for "Wait for server ready"

The execution flow has "Wait for server ready (health check)" but no health check endpoint is defined. The server exposes `GET /healthz` (typical for Echo). The plan should specify this explicitly.

### Missing: LLM Config endpoint test

`GET /api/v1/agent/:slug/llm-config` and `PUT /api/v1/agent/:slug/llm-config` are critical for deployment validation — if LLM config is missing or wrong, every chat/simulation test silently fails. This should be tested before Group 5-7.

### Missing: Streaming chat test

The public API also exposes `GET /api/v1/agent/:slug/chat/stream` (SSE). If streaming is part of the critical path, this should be tested.

---

## Nits

### Naming: run_all.sh references Taskfile_pg.yml that may not exist

The plan references `task -t Taskfile_pg.yml postgres:start` but the repo uses `Taskfile.yml`. Verify the correct task name or use direct `docker compose` commands.

### Auth: First-run assumption

Test 1.1 (sign up) assumes the first user doesn't exist. If re-running tests, signup will fail (user already exists). The test should either:
- Delete the user first (if possible)
- Fall back to sign-in if signup returns 409/400
- Use a unique username per run

### JSON assertion robustness

The plan mentions `assert_json()` for validation but doesn't specify what fields to check. For example:
- Signup should validate `role == "HOST"`
- Tenant creation should validate `slug` is returned
- RAG search should validate `results` is an array

The plan should include minimum field assertions per test to be useful.

### Orphaned handler note

There is an unregistered `HandleGetConfig` in `handlers.go:1668` alongside the active `HandleGetTenantFullConfig`. If `GET /api/v1/agent/:slug/config` returns unexpected results, this orphan is a likely source of confusion. Worth a comment in the test.

---

## Endpoint Verification Summary

| Test | Claimed Route | Actual Route | Match |
|------|---------------|--------------|-------|
| 1.1 | `POST /api/v1/auth/signup` | `POST /api/v1/auth/signup` | ✅ |
| 1.2 | `POST /api/v1/auth/signin` + `status` | `POST /api/v1/auth/signin` + `POST /api/v1/auth/status` | ✅ |
| 1.3 | `POST /api/v1/auth/signout` | `POST /api/v1/auth/signout` | ✅ |
| 2.1 | `POST /api/v1/agent/onboard` | `POST /api/v1/agent/onboard` | ✅ |
| 2.2 | `GET /api/v1/agent/:slug/config` | `GET /api/v1/agent/:slug/config` | ✅ |
| 2.3 | `PATCH /api/v1/agent/:slug` | `PATCH /api/v1/agent/:slug` | ✅ |
| 2.4 | `DELETE /api/v1/agent/:slug` | `DELETE /api/v1/agent/:slug` | ✅ (but breaks sequencing) |
| 3.1 | `POST /api/v1/agent/:slug/import` | `POST /api/v1/agent/:slug/import` | ⚠️ (undocumented params) |
| 3.2 | `POST /api/v1/agent/:slug/import` | `POST /api/v1/agent/:slug/import` | ⚠️ (same) |
| 3.3 | `GET /api/v1/agent/:slug/source-file` | `GET /api/v1/agent/:slug/source-file` | ✅ |
| 4.1 | `POST /api/v1/agent/:slug/reindex` | `POST /api/v1/agent/:slug/reindex` | ✅ |
| 4.2 | `POST /api/v1/agent/:slug/rag/search` | `POST /api/v1/agent/:slug/rag/search` | ✅ |
| 5.1 | `POST /api/v1/agent/:slug/chat/ext` | `POST /api/v1/agent/:slug/chat/ext` | ✅ |
| 5.2 | `POST /api/v1/agent/:slug/chat/ext` | `POST /api/v1/agent/:slug/chat/ext` | ✅ |
| 5.3 | `POST /api/v1/agent/:slug/chat/ext` | `POST /api/v1/agent/:slug/chat/ext` | ✅ |
| 6.1 | `POST /api/v1/agent/:slug/chat/int` | `POST /api/v1/agent/:slug/chat/int` | ✅ |
| 6.2 | `POST /api/v1/agent/:slug/chat/int` | `POST /api/v1/agent/:slug/chat/int` | ✅ |
| 7.1 | `POST /api/v1/agent/:slug/simulate` | `POST /api/v1/agent/:slug/simulate` | ✅ |
| 7.2 | `GET .../simulate/:id/stream` | `GET .../simulate/:sessionId/stream` | ❌ (param name mismatch) |
| 7.3 | `GET /api/v1/agent/:slug/simulations` | `GET /api/v1/agent/:slug/simulations` | ✅ |
| 8.1 | `GET /api/v1/agent/:slug/permissions` | `GET /api/v1/agent/:slug/permissions` | ✅ |
| 8.2 | `POST /api/v1/agent/:slug/permissions` | `POST /api/v1/agent/:slug/permissions` | ✅ |
| 8.3 | `DELETE .../permissions/:userId` | `DELETE .../permissions/:userId` | ✅ |
| 9.1 | `GET /api/v1/agent/:slug/learning` | `GET /api/v1/agent/:slug/learning` | ✅ |
| 9.2 | `DELETE /api/v1/agent/:slug/learning` | `DELETE /api/v1/agent/:slug/learning` | ✅ |
| 10.1 | `DELETE /api/v1/agent/:slug` | `DELETE /api/v1/agent/:slug` | ✅ |

---

## Final Verdict

**Rework Required.** Fix the sequencing contradiction (remove 2.4 or merge with 10.1), document the import endpoint's required form parameters, correct the simulation stream route param, and add LLM config + health check to the critical path.
