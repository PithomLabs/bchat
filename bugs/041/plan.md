# Plan: bchat E2E Critical Path Testing

**Date:** 2026-07-17
**Goal:** Stress test bchat major features end-to-end against local Postgres to build confidence before Fly.io + Neon deployment
**Approach:** curl-based shell scripts, critical path only (~25 scenarios across 10 categories)

---

## Setup Requirements

- Local Postgres via Docker (`task -t Taskfile_pg.yml postgres:start`)
- Server running against Postgres (`task -t Taskfile_pg.yml run:testrag` or manual start)
- `jq` installed for JSON parsing in shell scripts
- Sample KB.MD and POLICY.MD files for upload testing

---

## Test Groups

### Group 1: Auth (3 tests)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 1.1 | Sign up as first user | `POST /api/v1/auth/signup` | User creation, JWT cookie, HOST role |
| 1.2 | Sign in + auth status | `POST /api/v1/auth/signin` + `POST /api/v1/auth/status` | Cookie auth, JWT validation |
| 1.3 | Sign out | `POST /api/v1/auth/signout` | Cookie clearing |

### Group 2: Tenant Lifecycle (4 tests)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 2.1 | Onboard tenant with KB/Policy | `POST /api/v1/agent/onboard` | Tenant creation, file parsing |
| 2.2 | Get tenant config | `GET /api/v1/agent/:slug/config` | Config retrieval, stats |
| 2.3 | Update tenant | `PATCH /api/v1/agent/:slug` | Partial update, allowed domains |
| 2.4 | Delete tenant | `DELETE /api/v1/agent/:slug` | Cascade deletion |

### Group 3: File Management (3 tests)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 3.1 | Import KB.MD | `POST /api/v1/agent/:slug/import` | Upload, parsing, token count |
| 3.2 | Import POLICY.MD | `POST /api/v1/agent/:slug/import` | Upload, parsing |
| 3.3 | Get source file content | `GET /api/v1/agent/:slug/source-file` | Content retrieval |

### Group 4: RAG Pipeline (2 tests)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 4.1 | Trigger reindex | `POST /api/v1/agent/:slug/reindex` | Background indexing |
| 4.2 | RAG search explorer | `POST /api/v1/agent/:slug/rag/search` | Vector search, scoring |

### Group 5: External Chat (3 tests)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 5.1 | First message (new session) | `POST /api/v1/agent/:slug/chat/ext` | Session creation, widget key auth |
| 5.2 | Follow-up message | `POST /api/v1/agent/:slug/chat/ext` | Session continuity, context |
| 5.3 | Invalid widget key | `POST /api/v1/agent/:slug/chat/ext` | Auth rejection (403) |

### Group 6: Internal Chat (2 tests)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 6.1 | Authenticated chat | `POST /api/v1/agent/:slug/chat/int` | Full 18-step pipeline |
| 6.2 | Chat without auth | `POST /api/v1/agent/:slug/chat/int` | Auth rejection (401/403) |

### Group 7: Simulation (3 tests)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 7.1 | Start simulation | `POST /api/v1/agent/:slug/simulate` | AI-vs-AI creation |
| 7.2 | Stream simulation (SSE) | `GET /api/v1/agent/:slug/simulate/:id/stream` | Real-time message delivery |
| 7.3 | List simulations | `GET /api/v1/agent/:slug/simulations` | Transcript persistence |

### Group 8: RBAC (3 tests)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 8.1 | List permissions | `GET /api/v1/agent/:slug/permissions` | Permission retrieval |
| 8.2 | Grant permission | `POST /api/v1/agent/:slug/permissions` | User permission assignment |
| 8.3 | Revoke permission | `DELETE /api/v1/agent/:slug/permissions/:userId` | Permission removal |

### Group 9: Learning Memory (2 tests)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 9.1 | Get learning memory | `GET /api/v1/agent/:slug/learning` | Memory retrieval |
| 9.2 | Clear learning memory | `DELETE /api/v1/agent/:slug/learning` | Memory reset |

### Group 10: Cleanup (1 test)

| # | Test | API Endpoint | Validates |
|---|------|-------------|-----------|
| 10.1 | Delete test tenant | `DELETE /api/v1/agent/:slug` | Cascade cleanup |

---

## Script Structure

```
bugs/041/tests/
├── run_all.sh              # Execute all tests in order, report pass/fail
├── lib/
│   ├── common.sh           # Shared functions (auth, assert, curl wrapper)
│   ├── KB.MD               # Sample KB file for testing
│   └── POLICY.MD           # Sample Policy file for testing
├── 01_auth.sh              # Sign up, sign in, status, sign out
├── 02_tenant_crud.sh       # Onboard, config, update, delete
├── 03_files.sh             # Import KB/Policy, get content
├── 04_rag.sh               # Reindex, search
├── 05_external_chat.sh     # Widget chat flow
├── 06_internal_chat.sh     # Authenticated chat
├── 07_simulation.sh        # AI-vs-AI testing
├── 08_rbac.sh              # Permissions
├── 09_learning.sh          # Learning memory
└── 10_cleanup.sh           # Remove test data
```

---

## Shared Library: lib/common.sh

Provides:
- `BASE_URL` variable (default `http://localhost:5230`)
- `COOKIE_FILE` for session persistence
- `auth_signin()` — sign in and save cookie
- `auth_signout()` — clear session
- `assert_status()` — check HTTP status code
- `assert_json()` — check JSON field value with jq
- `log_test()` / `log_pass()` / `log_fail()` — test output formatting
- `cleanup_tenant()` — delete tenant if exists

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
  ├── Wait for server ready (health check)
  ├── 01_auth.sh          → Get JWT cookie
  ├── 02_tenant_crud.sh   → Create test tenant, verify config
  ├── 03_files.sh         → Upload KB/Policy, verify parsing
  ├── 04_rag.sh           → Reindex, verify search works
  ├── 05_external_chat.sh → Test anonymous widget chat
  ├── 06_internal_chat.sh → Test authenticated chat
  ├── 07_simulation.sh    → Run AI-vs-AI simulation
  ├── 08_rbac.sh          → Test permission grant/revoke
  ├── 09_learning.sh      → Test learning memory
  ├── 10_cleanup.sh       → Remove test tenant
  └── Report: X passed, Y failed
```

---

## Expected Output

```
=== bchat E2E Critical Path Tests ===
[01] Auth: Sign up ...................... PASS
[01] Auth: Sign in + status ............. PASS
[01] Auth: Sign out ..................... PASS
[02] Tenant: Onboard ................... PASS
[02] Tenant: Get config ................ PASS
[02] Tenant: Update .................... PASS
[02] Tenant: Delete .................... PASS
[03] Files: Import KB .................. PASS
[03] Files: Import POLICY .............. PASS
[03] Files: Get content ................ PASS
[04] RAG: Reindex ...................... PASS
[04] RAG: Search ....................... PASS
[05] External: New session ............. PASS
[05] External: Follow-up ............... PASS
[05] External: Invalid key ............. PASS
[06] Internal: Authenticated chat ...... PASS
[06] Internal: No auth ................. PASS
[07] Simulation: Start ................. PASS
[07] Simulation: Stream ................ PASS
[07] Simulation: List .................. PASS
[08] RBAC: List permissions ............ PASS
[08] RBAC: Grant permission ............ PASS
[08] RBAC: Revoke permission ........... PASS
[09] Learning: Get ..................... PASS
[09] Learning: Clear ................... PASS
[10] Cleanup: Delete tenant ............ PASS

=== Results: 26 passed, 0 failed ===
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
| 8 | `bugs/041/tests/04_rag.sh` | RAG pipeline tests |
| 9 | `bugs/041/tests/05_external_chat.sh` | External chat tests |
| 10 | `bugs/041/tests/06_internal_chat.sh` | Internal chat tests |
| 11 | `bugs/041/tests/07_simulation.sh` | Simulation tests |
| 12 | `bugs/041/tests/08_rbac.sh` | RBAC tests |
| 13 | `bugs/041/tests/09_learning.sh` | Learning memory tests |
| 14 | `bugs/041/tests/10_cleanup.sh` | Cleanup tests |

---

## Key Implementation Notes

1. **Cookie-based auth**: All authenticated tests use `curl -c cookies.txt -b cookies.txt` for session persistence
2. **JSON assertions**: Use `jq` to parse responses and validate fields
3. **Sequential dependencies**: Tests run in order; Group 2 creates the tenant used by Groups 3-9
4. **Error handling**: Each test checks HTTP status AND JSON content; failures logged with context
5. **Timeout**: Chat and simulation tests use longer timeouts (30s-120s) for LLM responses
6. **Cleanup**: Group 10 always runs (even on failure) to clean up test data
