# Bug 058 — E2E Test Review Findings (v2)

**Date:** 2026-08-03
**Author:** opencode
**Status:** Implemented
**Depends on:** `e2e.md`, `e2e_review.md`

---

## Executive Summary

This document captures the findings from the adversarial review of `e2e.md` and their implementation. All actionable findings (excluding OpenRouter API key setup) have been applied.

**Review Verdict:** APPROVE WITH NITS — 3 documentation gaps + 1 code bug found.

**Implementation Status:**

| # | Finding | Severity | Action | Status |
|---|---------|----------|--------|--------|
| 1 | P6 check false positive (header line bug) | High | Fix: add `| tail -1` to count queries | ✅ Fixed |
| 2 | Missing admin user creation step | Medium | Fix: add signup step to e2e.md Phase 3 | ✅ Fixed |
| 3 | Missing OpenRouter API key setup | Medium | Document only (per user request) | ✅ Documented |
| 4 | `run:cockroach` MEMOS_DRIVER gap | Low | Fix: add `MEMOS_DRIVER=cockroach` inline | ✅ Fixed |

---

## Finding 1 (High) — P6 Check False Positive on Fresh Database

**File:** `Taskfile.yml` — `crdb:verify` and `crdb:verify-vectors`

### Problem

The P6 check queries `information_schema.statistics` for index count:
```bash
I=$(run_sql "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';")
echo "$I" | grep -qv "^0" || { echo "FAIL: agent_vectors has no indexes"; exit 1; }
```

`run_sql` captures ALL output from `cockroach sql`, including the header line:
```
count
0
```

So `I` becomes `count\n0`. When piped to `grep -qv "^0"`:
- Line `count` does NOT match `^0` → printed
- Line `0` matches `^0` → suppressed
- Since `count` is printed, `grep` exits 0 (success)
- The check passes even though index count is 0

**Impact:** `crdb:verify` reports P6 as PASS on a fresh database where `agent_vectors` does not exist. This is a **false positive** that undermines the verification.

### Fix

Add `| tail -1` to strip the header line before checking:

**crdb:verify (line 369):**
```bash
# Before:
I=$(run_sql "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';")

# After:
I=$(run_sql "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';" | tail -1)
```

**crdb:verify-vectors (line 407):**
```bash
# Before:
I=$(run_sql "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';")

# After:
I=$(run_sql "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';" | tail -1)
```

**Also fixed:** Same issue on line 360 (`migration_history` count) for consistency:
```bash
# Before:
H=$(run_sql "SELECT count(*) FROM migration_history;")

# After:
H=$(run_sql "SELECT count(*) FROM migration_history;" | tail -1)
```

---

## Finding 2 (Medium) — Missing Admin User Creation Step

**File:** `e2e.md` — Phase 3

### Problem

The `verify-production.sh` script requires a valid admin user (`BCHAT_USER=admin BCHAT_PASS=<your-password>`), but `e2e.md` does not include a step to create this user. On a fresh database, the sign-in step fails with `400 invalid credentials`.

### Fix

Added admin user creation step before `verify-production.sh` in Phase 3:

```bash
curl -fsS -X POST http://localhost:8081/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<your-password>","email":"admin@test.com"}'
```

**Note:** This step is required on a fresh database. If the user already exists, the endpoint will return an error, which is harmless.

---

## Finding 3 (Medium) — Missing OpenRouter API Key Setup

**File:** `e2e.md` — Known Limitations

### Problem

The `verify-production.sh` script triggers RAG reindex, which requires `OPENROUTER_API_KEY`. The `e2e.md` does not document that this key must be set.

### Resolution

Documented as a prerequisite in `e2e.md` Known Limitations section. No code change — per user request, OpenRouter API key setup is excluded from implementation.

---

## Finding 4 (Low) — run:cockroach Missing MEMOS_DRIVER

**File:** `Taskfile.yml` — `run:cockroach` (line 241)

### Problem

The `run:cockroach` task sets `LANCEDB_STORAGE_PROVIDER=cockroach` and `RAG_PIPELINE_ENABLED=true` but does not set `MEMOS_DRIVER=cockroach`. The `.env` file doesn't contain `MEMOS_DRIVER=cockroach` either (it has the cloud CRDB DSN).

**Impact:** `task run:cockroach` silently falls back to SQLite instead of using CockroachDB.

### Fix

Added `MEMOS_DRIVER=cockroach` inline to the task command:

```yaml
# Before:
RAG_PIPELINE_ENABLED=true LANCEDB_STORAGE_PROVIDER=cockroach TICKET_EMBEDDING_ENABLED=true ./build/memos --mode dev --data {{.ROOT_DIR}}/build/data

# After:
MEMOS_DRIVER=cockroach RAG_PIPELINE_ENABLED=true LANCEDB_STORAGE_PROVIDER=cockroach TICKET_EMBEDDING_ENABLED=true ./build/memos --mode dev --data {{.ROOT_DIR}}/build/data
```

---

## Files Changed

| File | Change | Lines |
|------|--------|-------|
| `Taskfile.yml` | Added `| tail -1` to P6 count checks (lines 360, 369, 407); added `MEMOS_DRIVER=cockroach` to `run:cockroach` (line 241) | 4 lines changed |
| `bugs/058/e2e.md` | Added admin user creation step in Phase 3; updated Bug 4 status; updated Known Limitations | ~15 lines changed |

---

## Verification

After applying the fixes:

1. **P6 check false positive:** Verified that `task crdb:verify` correctly reports "FAIL: agent_vectors has no indexes" on a fresh database (before reindex).

2. **Admin user creation:** Verified that `verify-production.sh` succeeds after creating admin user via signup endpoint.

3. **MEMOS_DRIVER:** Verified that `task run:cockroach` now starts with `driver: cockroach` in logs (no longer falls back to SQLite).

---

## Adversarial Review Prompt (for e2e_v2.md)

```
You are reviewing the implementation of review findings for a local CockroachDB E2E
test (bchat). The original review found 3 documentation gaps + 1 code bug.

CONTEXT:
- Finding 1 (High): P6 check false positive — header line in cockroach sql output
  causes grep to match "count" instead of actual count value
- Finding 2 (Medium): Missing admin user creation step in e2e.md Phase 3
- Finding 3 (Medium): Missing OpenRouter API key setup — documented only, no code change
- Finding 4 (Low): run:cockroach MEMOS_DRIVER gap — now fixed inline

CHANGES MADE:
1. Taskfile.yml: Added "| tail -1" to lines 360, 369, 407 (P6 count checks)
2. Taskfile.yml: Added "MEMOS_DRIVER=cockroach" to run:cockroach (line 241)
3. e2e.md: Added admin user creation step in Phase 3
4. e2e.md: Updated Bug 4 status from "Workaround" to "Fix"
5. e2e.md: Updated Known Limitations (removed MEMOS_DRIVER gap, added OPENROUTER_API_KEY)

REVIEW FOR:

1. CORRECTNESS:
   - Is "| tail -1" the right fix for the header line issue?
   - Does adding MEMOS_DRIVER=cockroach to run:cockroach break anything?
   - Is the admin user creation step correct (POST /api/v1/auth/signup)?

2. COMPLETENESS:
   - Are all findings from e2e_review.md addressed?
   - Is there anything missing from the implementation?

3. RISK:
   - Could the "| tail -1" fix cause issues if cockroach sql output format changes?
   - Does MEMOS_DRIVER=cockroach in run:cockroach conflict with .env settings?

4. DOCUMENTATION:
   - Is e2e.md now accurate for fresh checkout execution?
   - Are the known limitations complete?

EXPECTED OUTPUT:
- For each finding: severity (Critical/High/Medium/Low), whether it's addressed
- Overall verdict: APPROVE / APPROVE WITH NITS / REQUEST CHANGES
- Any remaining gaps before cloud deployment
```
