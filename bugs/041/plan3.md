# Plan v3: bchat E2E Critical Path Testing

**Date:** 2026-07-17
**Goal:** Stress test bchat major features end-to-end against local Postgres to build confidence before Fly.io + Neon deployment
**Approach:** curl-based shell scripts, critical path only (~28 scenarios across 11 groups)
**Changes from v2:** Incorporates adversarial review findings (see `plan2_review.md`)

---

## Scope Addition: Widget Key Code Change

The external chat tests (Group 6) require the tenant's `widget_key` for authentication. No API endpoint currently returns or accepts this value. Two code changes in `handlers.go` are required:

### Change 1: Add `widgetKey` to config response

**File:** `server/router/api/v1/agent/handlers.go` (line ~758, inside `HandleGetTenantFullConfig`)

Add `widgetKey` to the tenant response map:
```go
"widgetKey": tenant.WidgetKey,
```

This makes the widget key visible to admins via `GET /api/v1/agent/:slug/config`.

### Change 2: Add `widget_key` to update request

**File:** `server/router/api/v1/agent/handlers.go` (line ~801, inside `HandleUpdateTenant`)

Add `WidgetKey` field to the request struct:
```go
var req struct {
    IsActive       *bool     `json:"is_active"`
    CompanyName    *string   `json:"company_name"`
    Vertical       *string   `json:"vertical"`
    AllowedDomains *[]string `json:"allowed_domains"`
    WidgetKey      *string   `json:"widget_key"`
}
```

Add handling after existing fields:
```go
if req.WidgetKey != nil {
    tenant.WidgetKey = *req.WidgetKey
}
```

This allows widget key rotation via `PATCH /api/v1/agent/:slug`.

---

## Setup Requirements

- Local Postgres via Docker (`task -t Taskfile_pg.yml postgres:start`)
- Server running against Postgres (`task -t Taskfile_pg.yml run:testrag` or manual start)
- `jq` installed for JSON parsing in shell scripts
- Sample KB.MD and POLICY.MD files for upload testing

---

## Test Groups

### Group 1: Auth (4 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 1.1 | Sign up as first user | `POST /api/v1/auth/signup` | User creation, JWT cookie, HOST role | `role == "HOST"`, `username` present |
| 1.2 | Sign in + auth status | `POST /api/v1/auth/signin` + `POST /api/v1/auth/status` | Cookie auth, JWT validation | `username` matches |
| 1.3 | Sign up second user (viewer) | `POST /api/v1/auth/signup` | Second user for RBAC tests | `username == "viewer-user"` |
| 1.4 | Sign out | `POST /api/v1/auth/signout` | Cookie clearing | Response is `{}` |

**Note:** Test 1.1 handles re-runs — if signup returns 409/400 (user exists), falls back to sign-in. Test 1.3 creates a viewer user needed for Group 9 RBAC tests. After 1.3, sign back in as admin (1.1 re-signs in).

### Group 2: Tenant Lifecycle (3 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 2.1 | Onboard tenant with KB/Policy | `POST /api/v1/agent/onboard` | Tenant creation, file parsing | `success == true`, `tenant.slug` present, `tenant.widget_key` present (after code change) |
| 2.2 | Get tenant config | `GET /api/v1/agent/:slug/config` | Config retrieval, widget key exposure | `tenant.widgetKey` present and non-empty |
| 2.3 | Update tenant + rotate widget key | `PATCH /api/v1/agent/:slug` | Partial update, widget key rotation | `success == true`, updated fields reflected |

**Onboard — exact curl invocation (form data):**
```bash
curl -s -b cookies.txt -X POST "$BASE/api/v1/agent/onboard" \
  -F "tenant_slug=e2e-test-tenant" \
  -F "company_name=E2E Test Corp" \
  -F "vertical=restoration" \
  -F "external_kb_file=@lib/KB.MD" \
  -F "external_policy_file=@lib/POLICY.MD"
```

**Note:** Delete test removed — cleanup handled by Group 11. Orphaned tenant from previous failed runs handled in `run_all.sh`.

### Group 3: File Management (3 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 3.1 | Import KB.MD | `POST /api/v1/agent/:slug/import` | Upload, parsing, token count | `success == true`, `totalTokens > 0` |
| 3.2 | Import POLICY.MD | `POST /api/v1/agent/:slug/import` | Upload, parsing | `success == true` |
| 3.3 | Get source file content | `GET /api/v1/agent/:slug/source-file` | Content retrieval | `content` field present and non-empty |

**Import — exact curl invocation:**
```bash
# Import KB.MD
curl -s -b cookies.txt -X POST "$BASE/api/v1/agent/$SLUG/import" \
  -F "audience_type=external" \
  -F "file_type=kb" \
  -F "file=@lib/KB.MD"

# Import POLICY.MD
curl -s -b cookies.txt -X POST "$BASE/api/v1/agent/$SLUG/import" \
  -F "audience_type=external" \
  -F "file_type=policy" \
  -F "file=@lib/POLICY.MD"
```

Required form fields: `audience_type` ("external"/"internal"), `file_type` ("kb"/"policy"), `file` (multipart).

### Group 4: LLM Config (2 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 4.1 | Get LLM config | `GET /api/v1/agent/:slug/llm-config` | Config retrieval | `model` field present |
| 4.2 | Set LLM config | `PUT /api/v1/agent/:slug/llm-config` | Config update | HTTP 200 |

**LLM config PUT — exact curl invocation:**
```bash
curl -s -b cookies.txt -X PUT "$BASE/api/v1/agent/$SLUG/llm-config" \
  -H "Content-Type: application/json" \
  -d '{
    "llm_model": "openai/gpt-4o-mini",
    "simulation_human_model": "openai/gpt-4o-mini",
    "reasoning_model": "google/gemini-2.5-pro"
  }'
```

**Note:** This group runs BEFORE chat/simulation tests. If LLM config is missing, those tests will silently fail.

### Group 5: RAG Pipeline (3 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 5.1 | Trigger reindex | `POST /api/v1/agent/:slug/reindex` | Background indexing | `success == true` |
| 5.2 | Poll reindex status | `GET /api/v1/agent/:slug/reindex/status` | Index completion | `status` field indicates done |
| 5.3 | RAG search explorer | `POST /api/v1/agent/:slug/rag/search` | Vector search, scoring | `results` is array |

**Note:** Reindex is async (returns 202 immediately). Test 5.2 polls `GET /api/v1/agent/:slug/reindex/status` until status indicates completion (or timeout after 60s). Test 5.3 runs AFTER 5.2 confirms indexing is done.

### Group 6: External Chat (3 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 6.1 | First message (new session) | `POST /api/v1/agent/:slug/chat/ext` | Session creation, widget key auth | `session_id` present, `message.content` present |
| 6.2 | Follow-up message | `POST /api/v1/agent/:slug/chat/ext` | Session continuity, context | `session_id` matches, `message.content` present |
| 6.3 | Invalid widget key | `POST /api/v1/agent/:slug/chat/ext` | Auth rejection | HTTP 403 |

**External chat — exact curl invocation:**
```bash
# First request (no session_id):
curl -s -X POST "$BASE/api/v1/agent/$SLUG/chat/ext" \
  -H "Content-Type: application/json" \
  -H "X-Widget-Key: $WIDGET_KEY" \
  -d '{"message":"Hello, I need help with water damage"}'

# Follow-up (with session_id from previous response):
curl -s -X POST "$BASE/api/v1/agent/$SLUG/chat/ext" \
  -H "Content-Type: application/json" \
  -H "X-Widget-Key: $WIDGET_KEY" \
  -d "{\"session_id\":\"$SESSION_ID\",\"message\":\"What is the cost?\"}"
```

**Note:** Widget key obtained from Group 2.2 config response (`tenant.widgetKey`).

### Group 7: Internal Chat (2 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 7.1 | Authenticated chat | `POST /api/v1/agent/:slug/chat/int` | Full 18-step pipeline | `message.content` present, `metadata.intent` present |
| 7.2 | Chat without auth | `POST /api/v1/agent/:slug/chat/int` | Auth rejection | HTTP 401 or 403 |

### Group 8: Simulation (3 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 8.1 | Start simulation | `POST /api/v1/agent/:slug/simulate` | AI-vs-AI creation | `session_id` present, `status == "running"` |
| 8.2 | Stream simulation (SSE) | `GET /api/v1/agent/:slug/simulate/:sessionId/stream` | Real-time message delivery | SSE events received |
| 8.3 | List simulations | `GET /api/v1/agent/:slug/simulations` | Transcript persistence | `simulations` is array |

**Note:** Route uses `:sessionId` (not `:id`). Stream timeout set to 120s for LLM response time.

### Group 9: RBAC (3 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 9.1 | List permissions | `GET /api/v1/agent/:slug/permissions` | Permission retrieval | Response contains permission entries |
| 9.2 | Grant permission to viewer | `POST /api/v1/agent/:slug/permissions` | User permission assignment | HTTP 200 or `success == true` |
| 9.3 | Revoke permission | `DELETE /api/v1/agent/:slug/permissions/:userId` | Permission removal | HTTP 200 |

**Grant permission — exact curl invocation:**
```bash
curl -s -b cookies.txt -X POST "$BASE/api/v1/agent/$SLUG/permissions" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":$VIEWER_USER_ID,\"permissions\":\"tenant:read\"}"
```

**Note:** `VIEWER_USER_ID` captured from Group 1.3 signup response.

### Group 10: Learning Memory (2 tests)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 10.1 | Get learning memory | `GET /api/v1/agent/:slug/learning` | Memory retrieval | Response is object (may be empty) |
| 10.2 | Clear learning memory | `DELETE /api/v1/agent/:slug/learning` | Memory reset | HTTP 200 |

### Group 11: Cleanup (1 test)

| # | Test | API Endpoint | Validates | JSON Assertions |
|---|------|-------------|-----------|-----------------|
| 11.1 | Delete test tenant | `DELETE /api/v1/agent/:slug` | Cascade deletion | HTTP 200 or `success == true` |

---

## Script Structure

```
bugs/041/tests/
├── run_all.sh              # Execute all tests in order, report pass/fail
├── lib/
│   ├── common.sh           # Shared functions (auth, assert, curl wrapper)
│   ├── KB.MD               # Sample KB file for testing
│   └── POLICY.MD           # Sample Policy file for testing
├── 01_auth.sh              # Sign up, sign in, status, sign out (+ create viewer)
├── 02_tenant_crud.sh       # Onboard, config, update (+ capture widget key)
├── 03_files.sh             # Import KB/Policy, get content
├── 04_llm_config.sh        # Get/set LLM config
├── 05_rag.sh               # Reindex, poll status, search
├── 06_external_chat.sh     # Widget chat flow (uses widget key)
├── 07_internal_chat.sh     # Authenticated chat
├── 08_simulation.sh        # AI-vs-AI testing
├── 09_rbac.sh              # Permissions (uses viewer user)
├── 10_learning.sh          # Learning memory
└── 11_cleanup.sh           # Remove test data
```

---

## Shared Library: lib/common.sh

Provides:
- `BASE_URL` variable (default `http://localhost:5230`)
- `COOKIE_FILE` for session persistence
- `SLUG` variable for test tenant slug (`e2e-test-tenant`)
- `WIDGET_KEY` variable (captured from config response)
- `VIEWER_USER_ID` variable (captured from signup response)
- `auth_signup()` — sign up with re-run fallback (if 409, sign in instead)
- `auth_signin()` — sign in and save cookie
- `auth_signout()` — clear session
- `assert_status <expected> <actual> <test_name>` — check HTTP status code
- `assert_json <json> <field> <expected> <test_name>` — check JSON field value with jq
- `assert_json_present <json> <field> <test_name>` — check JSON field exists
- `poll_reindex_status <slug> <max_attempts>` — poll until reindex completes
- `log_test()` / `log_pass()` / `log_fail()` — test output formatting
- `cleanup_tenant()` — delete tenant if exists (for Group 11 and orphaned tenant cleanup)
- `wait_for_server()` — poll `GET /healthz` until 200

---

## Server Readiness Check

```bash
wait_for_server() {
  local max_attempts=30
  local attempt=0
  while [ $attempt -lt $max_attempts ]; do
    if curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/healthz" | grep -q "200"; then
      return 0
    fi
    sleep 2
    attempt=$((attempt + 1))
  done
  echo "FAIL: Server did not become ready after $max_attempts attempts"
  return 1
}
```

---

## Orphaned Tenant Cleanup

`run_all.sh` attempts to delete the test tenant before onboarding:

```bash
# Attempt cleanup of orphaned tenant from previous failed run
cleanup_orphaned_tenant() {
  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" -b cookies.txt \
    -X DELETE "$BASE_URL/api/v1/agent/$SLUG")
  if [ "$status" = "200" ]; then
    echo "Cleaned up orphaned tenant from previous run"
  fi
  # Ignore 404 (tenant doesn't exist) — that's fine
}
```

---

## Sample Test Files: lib/KB.MD

```markdown
<!-- @service: water_extraction, emergency: true -->
## Water Extraction
24/7 emergency response for standing water removal.
We use industrial-grade pumps and dehumidifiers.
Response time: 30-60 minutes.

<!-- @service: fire_damage -->
## Fire Damage Restoration
Complete fire and smoke damage cleanup.
Includes structural assessment, soot removal, and deodorization.

<!-- @faq: pricing -->
## How much does water extraction cost?
Costs vary based on extent of damage. Typical range: $500-$3,000.
Free estimates available.

<!-- @faq: availability -->
## Are you available 24/7?
Yes, we offer 24/7 emergency service for water and fire damage.
Call our emergency line anytime.
```

---

## Sample Test Files: lib/POLICY.MD

```markdown
<!-- @identity -->
- Role: Customer Service Representative
- Tone: Professional, empathetic
- Company: Test Restoration Corp

<!-- @rule: greeting -->
Always greet the customer warmly.
Acknowledge their distress before asking questions.

<!-- @rule: emergency -->
For emergencies (water, fire, mold), express urgency.
Provide emergency phone number immediately.

<!-- @intent: schedule_service -->
## Schedule Service
Customer wants to book an appointment.
Ask for: name, phone, address, preferred time.
Confirm booking with reference number.
```

---

## Test Execution Flow

```
run_all.sh
  ├── Start Postgres Docker (if not running)
  ├── Start bchat server (background)
  ├── Wait for server ready (poll GET /healthz)
  ├── Cleanup orphaned tenant from previous run
  ├── 01_auth.sh          → Sign up admin + viewer, get JWT cookie
  ├── 02_tenant_crud.sh   → Create test tenant, get config (capture widget key), update
  ├── 03_files.sh         → Upload KB/Policy, verify parsing
  ├── 04_llm_config.sh    → Verify/set LLM config (must precede chat)
  ├── 05_rag.sh           → Reindex, poll status until done, verify search
  ├── 06_external_chat.sh → Test anonymous widget chat (uses widget key)
  ├── 07_internal_chat.sh → Test authenticated chat
  ├── 08_simulation.sh    → Run AI-vs-AI simulation
  ├── 09_rbac.sh          → Test permission grant/revoke (uses viewer user)
  ├── 10_learning.sh      → Test learning memory
  ├── 11_cleanup.sh       → Remove test tenant
  └── Report: X passed, Y failed
```

---

## Expected Output

```
=== bchat E2E Critical Path Tests ===
[01] Auth: Sign up admin ............... PASS
[01] Auth: Sign in + status ............. PASS
[01] Auth: Sign up viewer ............... PASS
[01] Auth: Sign out ..................... PASS
[02] Tenant: Onboard ................... PASS
[02] Tenant: Get config ................ PASS
[02] Tenant: Update + rotate key ........ PASS
[03] Files: Import KB .................. PASS
[03] Files: Import POLICY .............. PASS
[03] Files: Get content ................ PASS
[04] LLM: Get config ................... PASS
[04] LLM: Set config ................... PASS
[05] RAG: Reindex ...................... PASS
[05] RAG: Poll status .................. PASS
[05] RAG: Search ....................... PASS
[06] External: New session ............. PASS
[06] External: Follow-up ............... PASS
[06] External: Invalid key ............. PASS
[07] Internal: Authenticated chat ...... PASS
[07] Internal: No auth ................. PASS
[08] Simulation: Start ................. PASS
[08] Simulation: Stream ................ PASS
[08] Simulation: List .................. PASS
[09] RBAC: List permissions ............ PASS
[09] RBAC: Grant permission ............ PASS
[09] RBAC: Revoke permission ........... PASS
[10] Learning: Get ..................... PASS
[10] Learning: Clear ................... PASS
[11] Cleanup: Delete tenant ............ PASS

=== Results: 29 passed, 0 failed ===
```

---

## Files to Create

| # | File | Purpose |
|---|------|---------|
| 1 | `bugs/041/tests/run_all.sh` | Main orchestrator |
| 2 | `bugs/041/tests/lib/common.sh` | Shared functions |
| 3 | `bugs/041/tests/lib/KB.MD` | Sample KB content |
| 4 | `bugs/041/tests/lib/POLICY.MD` | Sample Policy content |
| 5 | `bugs/041/tests/01_auth.sh` | Auth tests |
| 6 | `bugs/041/tests/02_tenant_crud.sh` | Tenant lifecycle tests |
| 7 | `bugs/041/tests/03_files.sh` | File management tests |
| 8 | `bugs/041/tests/04_llm_config.sh` | LLM config tests |
| 9 | `bugs/041/tests/05_rag.sh` | RAG pipeline tests |
| 10 | `bugs/041/tests/06_external_chat.sh` | External chat tests |
| 11 | `bugs/041/tests/07_internal_chat.sh` | Internal chat tests |
| 12 | `bugs/041/tests/08_simulation.sh` | Simulation tests |
| 13 | `bugs/041/tests/09_rbac.sh` | RBAC tests |
| 14 | `bugs/041/tests/10_learning.sh` | Learning memory tests |
| 15 | `bugs/041/tests/11_cleanup.sh` | Cleanup tests |

## Files to Modify

| # | File | Change |
|---|------|--------|
| 16 | `server/router/api/v1/agent/handlers.go` | Add `widgetKey` to config response (~line 758) |
| 17 | `server/router/api/v1/agent/handlers.go` | Add `widget_key` to update request struct + handler (~line 801) |

---

## Key Implementation Notes

1. **Cookie-based auth**: All authenticated tests use `curl -c cookies.txt -b cookies.txt` for session persistence
2. **JSON assertions**: Use `jq` to parse responses and validate minimum fields per test
3. **Sequential dependencies**: Tests run in order; Group 2 creates the tenant used by Groups 3-10
4. **Error handling**: Each test checks HTTP status AND JSON content; failures logged with context
5. **Timeout**: Chat tests use 30s timeout; simulation stream uses 120s timeout for LLM responses
6. **Cleanup**: Group 11 always runs (even on failure) to clean up test data
7. **Re-run safety**: Auth tests handle existing users; orphaned tenants cleaned up before onboarding
8. **Health check**: Server readiness verified via `GET /healthz` before any tests run
9. **Widget key**: Obtained from config response (after code change), used for external chat auth
10. **Reindex polling**: Async reindex completion verified before RAG search test
