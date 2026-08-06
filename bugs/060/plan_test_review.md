# Adversarial Test-Plan Review — Bug 060

**Reviewer:** Senior Go Architect & CockroachDB Expert
**Date:** 2026-08-06
**Verdict:** GO-WITH-CHANGES

---

## Executive Summary

The test plan is well-structured and covers the right ground. The primary assertion path (RAG search → check results) correctly exercises the bug fix. However, there are concrete issues with the auth flow, baseline isolation checks, and pass criteria that could cause silent false passes or false failures. Most are minor; two are major.

---

## Findings

### 1. [MAJOR] Auth REST signin response has no `accessToken` field

**Category:** A (Assumption validity)
**Evidence:** `auth_service.go:647-693` — `HandleSignIn` returns `convertUserProtoToJSON(convertUserFromStore(user))`, which produces `{"id": "...", "name": "...", "username": "...", ...}`. There is **no `accessToken` field** in the JSON response. The access token is set as an HTTP cookie (`memos.access-token`), not returned in the body.

**Impact:** Step 2 (`TOKEN=$(python3 -c "import json,sys; print(json.load(open('/tmp/signin.json'))['accessToken'])")`) will fail with a `KeyError`.

**Fix:** Either:
- (a) Extract token from the `Set-Cookie` header in the signin response:
  ```bash
  curl -s -c /tmp/bchat_cookies -X POST http://localhost:5230/api/v1/auth/signin \
    -H "Content-Type: application/json" \
    -d "{\"password_credentials\":{\"username\":\"$BCHAT_USER\",\"password\":\"$BCHAT_PASS\"}}"
  TOKEN=$(python3 -c "import http.cookiejar; cj=http.cookiejar.MozillaCookieJar('/tmp/bchat_cookies'); cj.load(); print([c.value for c in cj if c.name=='memos.access-token'][0])")
  ```
- (b) Use the gRPC-gateway `SignIn` which returns a token in the response body (but that's a different endpoint).
- (c) Use cookie-based auth for all subsequent calls (`-b /tmp/bchat_cookies`).

**Recommendation:** Option (a) — cookie extraction. Then all subsequent `curl` calls use `-b /tmp/bchat_cookies` instead of `-H "Authorization: Bearer $TOKEN"`.

---

### 2. [MAJOR] `auth/status` is POST, not GET

**Category:** A (Assumption validity)
**Evidence:** `v1.go:219` — `echoServer.POST("/api/v1/auth/status", s.HandleGetAuthStatus)`. The plan's Step 2 says `GET /api/v1/auth/status` which will 404 or 405.

**Impact:** Auth verification step fails, but doesn't block the test.

**Fix:** Change to `curl -s -X POST -b /tmp/bchat_cookies http://localhost:5230/api/v1/auth/status`.

---

### 3. [MAJOR] RAG search endpoint path mismatch

**Category:** A (Assumption validity)
**Evidence:** The plan's Step 5 uses `POST /api/v1/admin/rag/search` (HandleTestRAGSearch). This route exists (`v1.go:474`) and is under `ragGroup` with `AuthMiddleware`. The request body field is `"tenantId"` (camelCase). But the handler struct (`handlers.go:4919-4930`) uses `TenantID int` with json tag `tenantId`. The plan's Step 5 payload uses `"tenantId":<NOLI_ID>` which matches.

**However:** the handler also requires `resolveQueryVersion` (line 4956), which checks `agent_source_files` for a versioned entry. If `HandleOnboard` → `importFiles` doesn't create a versioned source file entry, `resolveQueryVersion` returns nil and the handler returns `{"totalResults": 0}` — a false pass if the bug were present but also a false failure after the fix.

**Impact:** Could silently return empty results even with the fix, if version resolution fails.

**Fix:** Verify that `importFiles` creates a source file entry with a version. Add a pre-check in Step 4:
```sql
SELECT id, file_type, version FROM agent_source_files WHERE tenant_id = <NOLI_ID>;
```
Or use `POST /api/v1/agent/:slug/rag/search` (HandleRAGSearch, line 6037) which doesn't require version resolution and uses `SearchVectorDB` directly.

---

### 4. [MAJOR] Baseline isolation check is read-only but not proven

**Category:** C (Isolation & idempotency)
**Evidence:** The plan says "Isolation: other tenants' data (5 tenants, 88 rows) unchanged before/after" but Steps 1 and 7 don't actually run the SQL diff. The preflight SQL (Step 1) checks `SELECT count(*) FROM agent_vectors` (expect 88) but there's no post-run verification query.

**Impact:** If the test corrupts other tenants' data, the plan won't detect it.

**Fix:** Add post-run SQL verification in Step 7:
```sql
SELECT id, slug FROM agent_tenants;  -- expect 5 rows (no noli)
SELECT count(*) FROM agent_vectors WHERE tenant_id != <NOLI_ID>;  -- expect 88
```

---

### 5. [MAJOR] ContentType assertion may cause false failure

**Category:** F (Pass-criteria rigor)
**Evidence:** Step 5 says "every chunk `contentType == kb_section`". But `HandleTestRAGSearch` (line 4976-4978) builds `ContentTypes = []string{req.FileType, req.FileType + "_section"}` only when `req.FileType != ""`. When `FileType` is empty (which the plan's payload doesn't set), the search hits the fixed path with no content_type filter. Results may include chunks with `content_type = 'kb'` (legacy rows) or other types.

**Impact:** The assertion "all `contentType == kb_section`" could fail if legacy `kb` rows exist, producing a false failure.

**Fix:** Relax assertion to: "results non-empty; at least one chunk has `contentType == kb_section`; no chunks are from the Translator's Introduction".

---

### 6. [MINOR] `BCHAT_USER`/`BCHAT_PASS` not in `.env.example`

**Category:** A (Assumption validity)
**Evidence:** `.env.example` doesn't mention `BCHAT_USER` or `BCHAT_PASS`. The plan assumes they exist in the actual `.env`. If missing, signin fails.

**Impact:** Step 2 fails. The SQL fallback is available but adds complexity.

**Fix:** Document in plan: "Ensure `BCHAT_USER` and `BCHAT_PASS` are set in `.env`, or use the SQL password-reset fallback."

---

### 7. [MINOR] bcrypt fallback depends on Python bcrypt package

**Category:** A (Assumption validity)
**Evidence:** The SQL fallback (Step 2) uses `python3 -c "import bcrypt; ..."`. This requires `pip install bcrypt` or system package. Not guaranteed on fresh machines.

**Impact:** Fallback fails on systems without Python bcrypt.

**Fix:** Use Go for hash generation instead:
```bash
HASH=$(go run -C /home/chaschel/Documents/go/bchat -run '' --modfile <(echo 'module temp') -e 'package main; import ("fmt"; "golang.org/x/crypto/bcrypt"); func main() { h, _ := bcrypt.GenerateFromPassword([]byte("admin123"), 10); fmt.Println(string(h)) }')
```
Or pre-generate the hash and hardcode it.

---

### 8. [MINOR] `RAG_STARTUP_REINDEX_DISABLED` may not be consumed

**Category:** C (Isolation & idempotency)
**Evidence:** The plan claims `RAG_STARTUP_REINDEX_DISABLED=true` prevents surprise reindexing at boot. This env var isn't referenced in Taskfile.yml's `run:cockroach` task (line 302). Need to verify it's consumed by the Go binary.

**Impact:** If not consumed, a non-empty DB could trigger auto-reindex at startup, potentially racing with the test.

**Fix:** Verify in `service.go` startup logic that `RAG_STARTUP_REINDEX_DISABLED` is checked. If not, add `RAG_STARTUP_REINDEX_DISABLED=true` inline to the `run:cockroach` command.

---

### 9. [MINOR] Chat/ext requires widget key, but Step 3 doesn't fetch it explicitly

**Category:** A (Assumption validity)
**Evidence:** `HandleChatExternal` (line 396-407) requires `X-Widget-Key` header. The plan says to fetch widget key via `GET /api/v1/agent/noli/config` (Bearer) → `WIDGET_KEY`. But Step 3's response parsing (`OnboardResponse`) doesn't include `widgetKey` — it's in the separate config endpoint.

**Impact:** Step 6 (chat) will fail with 403 if widget key isn't fetched.

**Fix:** Add explicit Step 3b:
```bash
WIDGET_KEY=$(curl -s http://localhost:5230/api/v1/agent/noli/config \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import json,sys; print(json.load(sys.stdin)['tenant']['widgetKey'])")
```

---

### 10. [MINOR] Reindex status poll loop is unbounded

**Category:** E (Cost & time bounds)
**Evidence:** Step 4 says "Poll until done" but doesn't bound the loop. If reindex hangs, the test hangs.

**Impact:** Potential infinite loop.

**Fix:** Add a bounded poll:
```bash
for i in $(seq 1 60); do
  STATUS=$(curl -s http://localhost:5230/api/v1/agent/noli/reindex/status -H "Authorization: Bearer $TOKEN")
  echo "$STATUS" | python3 -c "import json,sys; d=json.load(sys.stdin); exit(0 if d.get('status')=='completed' else 1)" && break
  sleep 5
done
```

---

### 11. [NIT] Embedding cost estimate is inflated

**Category:** E (Cost & time bounds)
**Evidence:** Plan says "≈ $0.02 for the 1 MB novel (text-embedding-3-small via OpenRouter)". OpenRouter text-embedding-3-small is $0.02/M tokens. A 1 MB novel is ~250K tokens → ~$0.005, not $0.02.

**Impact:** None (cost is still negligible).

**Fix:** Update estimate to "~$0.005".

---

### 12. [NIT] Log-scan robustness for "RAG fallback"

**Category:** F (Pass-criteria rigor)
**Evidence:** Step 6 checks server logs for `RAG fallback`. The actual log string (service.go:3736) is `"RAG fallback activated"` via `slog.Info(...)`. slog outputs to stderr by default. The plan doesn't specify how to capture this (the server runs in foreground via `task run:cockroach`).

**Impact:** If the server output isn't captured to a file, the log check can't be performed.

**Fix:** Redirect server output: `task run:cockroach > /tmp/bchat.log 2>&1 &` or check the terminal output.

---

## Required Changes Before Execution

| # | Issue | Severity | Action |
|---|-------|----------|--------|
| 1 | Auth signin returns cookie, not accessToken | Major | Rewrite Step 2 to extract cookie |
| 2 | auth/status is POST, not GET | Major | Fix curl command |
| 3 | RAG search requires version resolution | Major | Add version pre-check or use HandleRAGSearch |
| 4 | Baseline isolation diff not executed | Major | Add post-run SQL verification |
| 5 | ContentType assertion may be too strict | Major | Relax to "at least one kb_section" |
| 6 | BCHAT_USER/BCHAT_PASS undocumented | Minor | Add to plan preconditions |
| 7 | bcrypt fallback depends on Python package | Minor | Use Go-based fallback or pre-generate |
| 8 | RAG_STARTUP_REINDEX_DISABLED not in run task | Minor | Verify or add inline |
| 9 | Widget key fetch not in Step 3 | Minor | Add Step 3b |
| 10 | Reindex poll loop unbounded | Minor | Add bounded retry loop |
| 11 | Embedding cost estimate inflated | Nit | Update to ~$0.005 |
| 12 | Log capture for fallback check | Nit | Redirect server output |

---

## Confidence

**Test catches the bug if present:** 90% (after fixes — the RAG search path correctly exercises the fix; the only gap is version resolution which can be worked around).

**Test doesn't produce false passes:** 85% (the non-empty check is sound; the ContentType assertion is the main risk).

**Test completes in <30 min:** 95% (reindex of 1 MB novel should take ~2-5 min with OpenRouter embeddings).

---

## Recommendation

**GO-WITH-CHANGES** — The plan is sound in structure and correctly identifies the right test path. Apply the 5 major fixes (auth flow, version resolution, baseline diff, ContentType assertion, widget key fetch) and the 5 minor fixes before execution. The test will then reliably verify the Bug 060 fix.
