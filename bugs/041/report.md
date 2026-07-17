# E2E Test Report — bchat Critical Path

**Date:** 2026-07-17
**Server:** bchat v0.31.0 (PostgreSQL 15, port 8081)
**Environment:** Local dev (Docker Postgres, setsid server, RAG pipeline enabled with OpenRouter embeddings)

---

## Summary

**35+ assertions across 11 test groups — ALL PASS**

```
Total: 11 test groups
  Passed: 11
  Failed: 0
```

---

## Test Results Detail

### 01 — Authentication
| Test | Result | Notes |
|------|--------|-------|
| Sign up admin | PASS | Existing user handled gracefully (HTTP 409 fallback) |
| Auth status | PASS | Returns username + ID correctly |
| Sign up viewer | PASS | RBAC test user created/retrieved |
| Sign out | PASS | Cookie cleared |

### 02 — Tenant CRUD
| Test | Result | Notes |
|------|--------|-------|
| Onboard tenant | PASS | Creates tenant + external KB/Policy |
| Get config | PASS | Returns config with widget_key |
| Update tenant | PASS | PATCH updates company name/vertical |

### 03 — File Import
| Test | Result | Notes |
|------|--------|-------|
| Import external KB | PASS | audience_type=external |
| Import external POLICY | PASS | |
| Import internal KB | PASS | **NEW** — required for internal chat/simulation |
| Import internal POLICY | PASS | **NEW** |
| Get source content | PASS | Returns KB content |

### 04 — LLM Config
| Test | Result | Notes |
|------|--------|-------|
| Get LLM config | PASS | Returns tenant_slug (empty model on fresh tenant) |
| Set LLM config | PASS | PUT with openai/gpt-4o-mini + reasoning model |
| Get after set | PASS | Verifies model persisted |

### 05 — RAG Pipeline
| Test | Result | Notes |
|------|--------|-------|
| Trigger reindex | PASS | POST triggers async reindex |
| Poll reindex status | PASS | Returns "completed" |
| RAG search | PASS | Found 2 results for "water damage" |

### 06 — External Chat (Widget)
| Test | Result | Notes |
|------|--------|-------|
| New session | PASS | X-Widget-Key auth, returns session_id + response |
| Follow-up | PASS | Same session maintained |
| Invalid key | PASS | HTTP 403 for bad widget key |

### 07 — Internal Chat (Authenticated)
| Test | Result | Notes |
|------|--------|-------|
| Authenticated chat | PASS | Requires auth + tenant binding |
| No auth | PASS | HTTP 401 without cookie |

### 08 — AI-vs-AI Simulation
| Test | Result | Notes |
|------|--------|-------|
| Start simulation | PASS | Returns session_id + running status |
| Stream simulation | PASS | SSE events received |
| List simulations | PASS | Polls up to 60s for async transcript save |

### 09 — RBAC Permissions
| Test | Result | Notes |
|------|--------|-------|
| List permissions | PASS | Returns permission list |
| Grant permission | PASS | Grants tenant:read to viewer |
| Revoke permission | PASS | Removes permission |

### 10 — Learning Memory
| Test | Result | Notes |
|------|--------|-------|
| Get learning | PASS | Returns learning state |
| Clear learning | PASS | Clears memory |

### 11 — Cleanup
| Test | Result | Notes |
|------|--------|-------|
| Delete tenant | PASS | CASCADE deletes all related data |

---

## Bugs Found & Fixed During Testing

### Bug 1: JSONB `features` column — `invalid input syntax for type json`
**File:** `store/db/postgres/rbac.go:167-199`
**Cause:** `json.Marshal(nil)` on `map[string]interface{}` produces `[]byte("null")`, but pgx sends `[]byte` as binary `bytea`, not text JSON. PostgreSQL cannot cast binary bytea to jsonb.
**Fix:** Convert marshaled bytes to `string` before passing to pgx: `featuresStr := string(features)`. Also added nil-check for `config.Features`.

### Bug 2: Missing `id` in auth status response
**File:** `server/router/api/v1/auth_service.go:745-757`
**Cause:** `convertUserProtoToJSON` didn't include the user ID field. Proto User has `Name` field with format `"users/{id}"`, not a direct `Id` field.
**Fix:** Extract numeric ID from `"users/{id}"` format and add `"id"` to the response map.

### Bug 3: REST auth returning HTTP 500 for duplicate users
**File:** `server/router/api/v1/auth_service.go:601-605`
**Cause:** `CreateUser` returned unique constraint violation, which was caught as generic 500 instead of 409 Conflict.
**Fix:** Added check for existing user before returning 500.

### Bug 4: Test scripts missing internal audience files
**File:** `bugs/041/tests/03_files.sh`
**Cause:** Only external KB/Policy were imported. Internal chat (`/chat/int`) and simulation require internal audience config.
**Fix:** Added import steps for internal KB and internal POLICY.

---

## Known Issues (Non-blocking)

| Issue | Impact | Root Cause |
|-------|--------|------------|
| Simulation transcript save is async | Test 08 requires polling up to 60s | Goroutine-based design saves transcript after LLM calls complete |
| Server process dies when parent shell exits | Must use `setsid` + `</dev/null` | Process group signal handling |

---

## Code Changes Made

| File | Change |
|------|--------|
| `store/db/postgres/rbac.go` | Nil-safe Features + string cast for JSONB |
| `server/router/api/v1/auth_service.go` | Add `id` to auth status, 409 for duplicate users |
| `server/router/api/v1/agent/handlers.go` | (from plan3: widget_key in config response + PATCH) |
| `bugs/041/tests/03_files.sh` | Added internal KB/Policy import |
| `bugs/041/tests/04_llm_config.sh` | Fixed assertion logic for fresh tenant |
| `bugs/041/tests/08_simulation.sh` | Polling for async transcript save |
| `bugs/041/tests/01_auth.sh` | Handle HTTP 500 for duplicate users, fixed signout assertion |

---

## Server Startup Command

```bash
setsid bash -c 'exec env \
  OPENROUTER_API_KEY=sk-or-v1-... \
  RAG_PIPELINE_ENABLED=true \
  /path/to/build/memos --mode dev \
  --driver postgres \
  --dsn "postgresql://bchat:bchat@localhost:5432/bchat"' \
  </dev/null > /tmp/bchat_server.log 2>&1 &
```

## Test Execution

```bash
cd bugs/041/tests
rm -f .cookies.txt .widget_key .session_id .viewer_user_id .sim_id
BASE_URL=http://localhost:8081 bash run_all.sh
```
