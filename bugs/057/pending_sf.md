# Pending Items & Difficulties Log — session-057 / plan3_e2e

**Date:** 2026-08-02  
**Author:** Kilo  
**Context:** E2E safety hardening + CockroachDB reference deployment close-out

---

## 1. Difficulties Encountered

### 1.1 Production panic in `UpsertAccessTokenToStore` (nil pointer dereference)

**What happened:** After deploying the CockroachDB build, `/api/v1/auth/tenants` crashed with:
```
panic: runtime error: invalid memory address or nil pointer dereference
  at server/router/api/v1/user_service.go:579 (aClaims.IssuedAt.Unix())
```

**Root cause:** The dedup-sort comparator calls `aClaims.IssuedAt.Unix()` and `bClaims.IssuedAt.Unix()` without nil checks. Some stored tokens (e.g., `selection:<token>`, tokens created during incident recovery) have no `iat` claim or produce malformed JWTs. `jwt.NewParser().ParseUnverified()` returns a `ClaimsMessage` whose `IssuedAt` is `nil` in those cases. Accessing `.Unix()` on nil pointer panicked the entire handler.

**Fix applied:**
```go
aIat := int64(0)
if aClaims.IssuedAt != nil {
    aIat = aClaims.IssuedAt.Unix()
}
bIat := int64(0)
if bClaims.IssuedAt != nil {
    bIat = bClaims.IssuedAt.Unix()
}
if aIat < bIat { return -1 }
return 1
```
Non-zero default preserves sort order for unparseable tokens.

**Files changed:** `server/router/api/v1/user_service.go`

---

### 1.2 NULL scan crash in `FindUserByAccessToken` on CockroachDB

**What happened:** After fixing the panic, select-tenant returned `401 invalid or expired selection token`. Debug logging revealed:
```
sql: Scan error on column index 11, name "allowed_tenant_ids": unsupported Scan, storing driver.Value type <nil> into type *[]string
```

**Root cause:** The admin user's `allowed_tenant_ids` column is `NULL` in the database (meaning "all tenants"). The `FindUserByAccessToken` query scans directly into `&user.AllowedTenantIDs` (`*[]string`). CockroachDB's pgx driver returns `nil` for NULL columns, which cannot be scanned into a pointer-to-slice. SQLite happens to tolerate this, but CockroachDB is strict.

**Fix applied:**
- Scan into `allowedTenantIDsRaw *string` instead
- Unmarshal JSON manually: `json.Unmarshal([]byte(*allowedTenantIDsRaw), &user.AllowedTenantIDs)`
- Handle `nil` gracefully (leaves slice as `nil`, meaning "all tenants")

**Files changed:** `store/db/postgres/user_setting.go` (added `encoding/json`, `fmt` imports; updated `FindUserByAccessToken`)

**Same pattern likely exists in:** `store/db/sqlite/user_setting.go`, `store/db/mysql/user_setting.go` — same direct scan into `&user.AllowedTenantIDs`. If those drivers ever return a NULL `driver.Value`, they will also crash. **Should be fixed as a follow-up for consistency, even if not yet observed.**

---

### 1.3 Verify script step 6 timeout / RAG mode mismatch

**What happened:** `verify-production.sh` step 6 (RAG search) timed out repeatedly. The script appeared stuck at "[6/7] RAG search" for 5+ minutes before the outer `timeout 300` killed it. Investigation revealed:

1. The original KB content (2 service + FAQ entries, ~47 tokens) was below the `DefaultTokenThreshold` of 30,000 tokens
2. `EstimateTokens` uses `len(content)/4` fallback when the global tokenizer is uninitialized
3. 47 tokens → `retrieval_mode = long_context` in `tenant_config`
4. Reindex succeeded but produced zero LanceDB records in long_context mode (RAG skip path)
5. RAG search against an empty vector DB returns `{"total_results":0}` on every retry → infinite loop in the script's 12-attempt wait

**Fix applied:** Increased KB size in `verify-production.sh` from 500x repetitions (26,875 tokens) → 1000x repetitions (53,750 tokens). This reliably crosses the 30K threshold and triggers `retrieval_mode = rag`, causing the reindex to actually populate `agent_vectors`.

**After fix:** Reindex produces 44 chunks; embedding completes in ~8s. **However**, RAG search then fails with a new CockroachDB error:
```
Search failed: failed to execute search: ERROR: could not parse vector: malformed vector literal: Vector contents must start with "[" and end with "]"
```

This indicates the stored embedding is not being formatted as a proper CockroachDB vector literal (`[0.1,0.2,...]`) during the `<=>` distance query. The `agent_vectors` table confirms 44 rows exist with `embedding VECTOR(1536)` column populated. This is a CockroachDB `<=>` operator compatibility issue in `vectordb_cockroach.go` search path — the query passes the embedding as `$1::VECTOR`, but CockroachDB requires the literal to be wrapped in `ARRAY[...]` or `[]::VECTOR` syntax.

**Pending:** See §2.4 below.

---

### 1.4 `isLocalDSN` heuristic is a future maintenance hazard

**What happened:** The two-key safety system requires determining whether a DSN is "local". The implementation uses substring matching:
```go
func isLocalDSN(dsn string) bool {
    return strings.Contains(dsn, "localhost") || strings.Contains(dsn, "127.0.0.1")
}
```

**Challenge:** This is fragile. A developer with a local CockroachDB listening on a hostname like `cockroach.local` or `192.168.1.100` would need to also set `BCHAT_ALLOW_REMOTE_DB_RESET=1`, even for a truly local machine. Conversely, a remote database URL that happens to contain `localhost` in its connection string (e.g., SSH tunnel `localhost:26257` forwarding to a remote) would bypass the second key.

**Why not fixed now:** Fully correct behavior requires either:
- Parsing the DSN URL and extracting the host, then checking against a configurable allowlist
- Or an explicit `BCHAT_DB_HOST=localhost` convention

Both are more invasive than the substring heuristic. For the hackathon close-out, the substring heuristic is documented in `plan3_e2e.md` as the accepted behavior, with the understanding that any non-localhost DSN must pass both keys.

**Future action:** Replace with proper URL host parsing; see `pending` item §2.5.

---

## 2. Pending Items — Requires Further Testing

### 2.1 [BLOCKING] CockroachDB vector search format (`<=>` operator)

**Severity:** P1 — blocks verify:production step 6 completion

**Symptom:**
```json
{"error":"Search failed: ERROR: could not parse vector: malformed vector literal: Vector contents must start with \"[\" and end with \"]\""}
```

**Likely cause:** `vectordb_cockroach.go` search query passes the embedding parameter as `$1::VECTOR(1536)`. CockroachDB's `<=>` operator requires the vector literal to be in `[x,x,x,...]` format when passed as a query parameter. The pgx driver may serialize `[]float32` as `(0.1,0.2,0.3)` (parentheses, comma-separated) instead of `[0.1,0.2,0.3]` (square brackets).

**Location:** `server/router/api/v1/agent/vectordb_cockroach.go` — the `Search` method's SQL query around line 323.

**Proposed fix options:**
- A. Cast to `ARRAY[...]` explicitly: `$1::VECTOR(1536)` → `ARRAY[$1]::VECTOR(1536)` (if pgx handles `[]float32` as array input)
- B. Format the vector as a string literal on the Go side: `fmt.Sprintf("[%s]", strings.Trim(strings.ReplaceAll(fmt.Sprint(embedding), " ", ","), "[]"))` — fragile
- C. Use CockroachDB's `vec` constructor if available: `vec($1)` — requires checking CockroachDB version compatibility

**Test plan:**
```bash
# Reproduce directly:
cockroach sql --url "$COCKROACH_DSN" -e "
SELECT 1 - (embedding <=> ARRAY[0.1,0.2,...]::VECTOR(1536)) AS similarity
FROM agent_vectors LIMIT 1;
"
# Then verify via API:
curl -X POST "https://bchat-crdb.fly.dev/api/v1/agent/<slug>/rag/search" \
  -H "Content-Type: application/json" \
  -d '{"query":"smoke test"}'
```

**Blocking:** Cannot mark verify:production as passing until step 6 consistently returns hits.

---

### 2.2 [MEDIUM] Same NULL scan issue in SQLite and MySQL `FindUserByAccessToken`

**Severity:** Medium — may surface when SQLite/MySQL `allowed_tenant_ids` is NULL

**Current status:** PostgreSQL version fixed. SQLite and MySQL implementations in `store/db/sqlite/user_setting.go` and `store/db/mysql/user_setting.go` use the same direct-scan pattern:
```go
&user.Description, &user.AllowedTenantIDs, &description,
```

**Why not triggered yet:** SQLite's `database/sql` driver may coerce NULL into an empty slice, masking the bug. MySQL driver behavior is untested.

**Fix needed:** Mirror the PostgreSQL fix — scan into `*string`, unmarshal JSON, handle nil. Must be applied to all three driver implementations for consistency.

---

### 2.3 [MEDIUM] Debug `slog.Info` left in `HandleSelectTenant`

**Severity:** Medium — leaks internal token state to production logs

**File:** `server/router/api/v1/auth_service.go` line ~510

```go
slog.Info("select-tenant debug", "token", accessTokenValue, "err", err, "user", matchedUser, "desc", description)
```

**Issue:** This logs the full `selection:<token>` value on every select-tenant request. While the token is short-lived (5-min single-use), logging credentials/tokens is a security smell.

**Fix:** Remove the debug line entirely. If needed temporarily, change to `slog.Debug(...)` so it can be enabled on demand without code changes.

---

### 2.4 [LOW] `isLocalDSN` heuristic replacement

**Severity:** Low — current behavior is correct for typical localhost setups; future enhancement

See §1.4 for details.

---

### 2.5 [LOW] Verify script retry loop should distinguish "no results" from "error"

**Severity:** Low — cosmetic, but masks real failures

**Current behavior:**
```bash
HITS=$(curl ... || echo "")
N=$(echo "$HITS" | grep -o '"hits"\|"items"' | head -1 | wc -l)
[[ "$N" -ge 1 ]] && break
```
This counts whether the JSON contains `"hits"` or `"items"` keys at all — it doesn't check if `total_results > 0`. An empty result set retries the full 12 times unnecessarily.

**Fix:** Parse `total_results` from the JSON response and break immediately if `> 0`, fail after all retries if still `0`.

---

### 2.6 [LOW] `verify-production.sh` uses `--keep` inconsistently

**Severity:** Low — UX issue

**Current behavior:** `--keep` flag prevents tenant deletion, but the script's cleanup step uses `if [[ "$KEEP" == "0" ]]` which means `--keep` must be passed as `--keep=1` or the default `0` is used. Standard POSIX convention is that `--keep` alone sets it to true.

**Not blocking for hackathon** — document for next iteration.

---

## 3. Items Confirmed Working / Not Requiring Further Testing

| Item | Evidence |
|------|----------|
| `BCHAT_ALLOW_DB_RESET=1` guard in tests | `TestCockroachMigrateEndToEnd` and `TestCockroachBootIdempotency` skip without it; run with it |
| `BCHAT_ALLOW_REMOTE_DB_RESET=1` second key | Code path confirmed; remote-DSN manual test not run (intentionally — don't want to re-test against Cloud) |
| Taskfile local-DSN gating | `task crdb:verify` runs E2E for localhost; skips with clear message for remote |
| `SELECT current_database()` assertion | Present in Taskfile; not tested against non-matching DB (would require Cloud DSN) |
| `/healthz` required when `BCHAT_URL` set | Taskfile now uses `|| { echo FAIL; exit 1; }` — tested implicitly via deploy chain |
| NULL scan fix for PostgreSQL | Verified: select-tenant now returns `tenant selected (id=3/4/5/6/7/8)` consistently |
| `IssuedAt` nil-safe sort | Verified: no more panics in `HandleAuthTenants`; tokens with malformed JWT produce warnings instead of crashes |
| E2E tests pass on fresh cluster | `TestCockroachMigrateEndToEnd` (45.83s), `TestCockroachBootIdempotency` (0.02s) both PASS |
| P0 test passes | `TestCockroachP0` passes (0.91s), confirms `nextval()` defaults |
| `verify:production` steps 1–5 | healthz, signin, select tenant, onboard, KB import + reindex all PASS repeatedly |
| `tenant_selection_token` 5-min expiry | Tokens from prior attempts correctly rejected with `selection token expired` |

---

## 4. Open Questions / Decisions Needed

| Question | Options | Current status |
|----------|---------|----------------|
| Fix SQLite/MySQL NULL scan now or in separate PR? | A. Now (minimal diff, 3 files) B. Separate PR | Not decided — recommend A since the bug is real and the fix is identical |
| Work around CockroachDB vector literal format now? | A. Fix `vectordb_cockroach.go` search SQL B. Defer to next session C. Use LanceDB local filesystem instead of Cockroach vector | Not decided — needs investigation of CockroachDB `<=>` parameter binding behavior |
| Keep `--keep` default or change? | A. Keep current (default destroy) B. Change to `--keep` default for safety | Not decided — for CI, default-destroy is correct; for manual debugging, `--keep` should be easier |

---

## 5. References

| Artifact | Path |
|----------|------|
| Session plan (plan3) | `bugs/057/plan3_e2e.md` |
| Review of plan3 | `bugs/057/plan3_e2e_review.md` |
| Session transcript | `bugs/057/session-057.md` |
| E2E test file | `store/test/cockroach_migrate_test.go` |
| Taskfile | `Taskfile.yml` |
| Verify script | `scripts/verify-production.sh` |
| Postgres user setting fix | `store/db/postgres/user_setting.go` |
| Token sort nil fix | `server/router/api/v1/user_service.go` |
| Cockroach vector store | `server/router/api/v1/agent/vectordb_cockroach.go` |
| Deploy config | `fly_cockroach.toml` |
| Phase 4 completion report | `bugs/057/artifacts/phase4/completion-report.md` |
