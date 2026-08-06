# Bug 060 — Local End-to-End Test Plan v3 (Hermetic, Production-Safe)

**Status:** Planned — fixes agreed, not yet implemented/executed
**Date:** 2026-08-06
**Supersedes:** `plan2_test.md` v2 (executed → PASS; its deviations 1–5 are the input for this revision)
**Agreement (user decisions, 2026-08-06):**
- App-level loopback-DSN guard in `internal/profile/profile.go` (opt-out `MEMOS_ALLOW_REMOTE_DSN=true`)
- Hermetic run: fresh dedicated `bchat_test` DB (DROP+CREATE per run) — exact-count assertions, no baseline diffs
- Local port `5230` (`MEMOS_PORT=5230`, aligns with existing scripts)
- Cloud migration side effect (0.36.1 DDL on Cockroach Labs Cloud, from v2 deviation 2): **no action — documented only** (idempotent, forward-compatible)
- `plan_test_review.md` (GO-WITH-CHANGES) and `plan2_test_review.md` (GO) findings remain valid and carry over unless changed here

---

## 0. Purpose

Same as v2: verify the Bug 060 fix end-to-end (retrieval returns `kb_section`; chat answer grounded in the indexed novel; zero `RAG fallback activated`). **This revision eliminates the two production-exposure incidents from v2** (cloud DSN from `.env`; wrong port assumption) so the plan cannot touch the production cluster by construction, not by luck.

## 1. Why v3 changes what v2 did

| v2 problem (observed) | Root cause | v3 fix |
|---|---|---|
| First boot hit the **Cockroach Labs Cloud** cluster; idempotent 0.36.1 migration ran there | `run:cockroach` sources `.env`, whose `COCKROACH_DSN` is the cloud URL; nothing validated the DSN | §2 app-level guard refuses non-loopback DSN in non-prod mode; §3 `.env.local` overlay; §4 hermetic DB |
| Plan said port 5230; server actually started on **8081** | 8081 is the local default; 5230 is the fly.prod setting | §3 `.env.local` sets `MEMOS_PORT=5230`; §5.1 boot evidence must match |
| Local admin password unknown; `.env` creds are cloud-only; SQL reset + Go bcrypt helper needed | Shared `.env`; pre-seeded local DB | §5.2 fresh-DB signup bootstrap — first signup becomes HOST (auth_service.go:578), no SQL mutation |
| Baseline/isolation SQL diffs needed; residual risk of confusing shared `bchat` dev DB with prod | Test ran inside shared DB | §4 fresh `bchat_test` per run → exact per-tenant counts (3 auto-seeded demo tenants + noli; 293 `kb_section` scoped to noli), no baseline step |
| Guardrails existed but environment reality was only caught at run time | Plans hardcoded assumptions instead of proving them | §5.1 boot-evidence gate (port line, driver, `end migrate`) must pass before any API call |

## 2. Code change 1 — loopback-DSN guard (implementation pending user GO)

**File:** `internal/profile/profile.go`, in `Validate()` after the DSN resolution block (lines 97–109).

```go
// guard: refuse non-loopback DSNs in non-prod mode unless explicitly allowed.
if p.IsDev() && (p.Driver == "cockroach" || p.Driver == "postgres") && os.Getenv("MEMOS_ALLOW_REMOTE_DSN") != "true" {
    if !isLoopbackDSN(p.DSN) {
        return errors.Errorf("refusing to start in %s mode: DSN host %q is not loopback (localhost/127.0.0.1/::1). "+
            "Local dev runs must not touch remote databases; set MEMOS_ALLOW_REMOTE_DSN=true to override", p.Mode, dsnHost(p.DSN))
    }
}
```

- `p.IsDev()` covers `dev` **and** `demo` (the default when no `--mode` is passed — `Validate()` line 67–69 normalizes unknown modes to `demo`), so even a bare `./build/memos --driver=cockroach` is guarded.
- `isLoopbackDSN` (review nit 1 — applied): parse with `net/url`, then `net.SplitHostPort` to strip `:port` and IPv6 brackets; hostname ∈ {`localhost`, `127.0.0.1`, `::1`}. **Parse failure → treated as non-loopback (fail closed).** Tested with `postgresql://root@[::1]:26257/...` → allowed.
- `mode=prod` (fly deploys) unaffected — cloud DSN continues to work in production.
- Scope: `cockroach` and `postgres` drivers (sqlite is a local file; nothing to guard).

**Test:** new `internal/profile/profile_test.go` (none exists today) — table tests on `Validate()` (must set `Data` to `t.TempDir()`; `checkDataDir` requires an existing dir):
| Case | Mode | Driver | DSN host | Env | Expect |
|---|---|---|---|---|---|
| loopback OK | dev | cockroach | `localhost:26257` | — | nil |
| loopback OK | dev | postgres | `127.0.0.1` | — | nil |
| remote rejected | dev | cockroach | `great-goat-30894.j77.cockroachlabs.cloud` | — | error mentions host |
| remote rejected | demo | cockroach | remote | — | error (demo guarded too) |
| opt-out bypass | dev | cockroach | remote | `MEMOS_ALLOW_REMOTE_DSN=true` | nil |
| unparseable rejected | dev | cockroach | `not a url` | — | error (fail closed) |
| prod allows remote | prod | cockroach | remote | — | nil |

## 3. Change 2 — env layering + Taskfile (implementation pending user GO)

**Problem:** `run:cockroach` etc. source `.env` only (`Taskfile.yml` lines 116, 127, 138, 150, 160, 171, 300, 310, 372 — 9 sites), and `.env`'s `COCKROACH_DSN` is the cloud URL.

**Fix:** every `.env` source site appends a `.env.local` overlay (local wins):

```bash
set -a && . .env && set +a
[ -f .env.local ] && set -a && . .env.local && set +a
```

`.env.local` is already gitignored (`.gitignore` lines 92–99). Commit a template as **`.env.local.example`** (not matched by any ignore pattern):

```bash
# Local-only overrides. Copy to .env.local. Never commit .env.local.
# Local dev runs must target the local CRDB node, never the cloud cluster.
COCKROACH_DSN="postgresql://root@localhost:26257/bchat_test?sslmode=disable"
```

`MEMOS_PORT=5230` stays inline in the task only (review nit 2 — applied: inline wins over `.env.local`, avoids duplication).

The cloud DSN stays in `.env` — used only by fly deploy tooling, and now unusable for local dev because of the §2 guard.

## 4. Change 3 — hermetic DB + local run entrypoint (implementation pending user GO)

**New script `scripts/init-local-cockroach-db.sh`:**
1. Assert `COCKROACH_DSN` host is loopback (belt + suspenders behind the app guard).
2. Reset the hermetic DB against the local compose cluster (`crdb:reset`, insecure root@localhost:26257):
   ```bash
   cockroach sql --url "postgresql://root@localhost:26257/defaultdb?sslmode=disable" \
     -e "DROP DATABASE IF EXISTS bchat_test; CREATE DATABASE bchat_test;"
   ```
   (Drop via `defaultdb` — you cannot drop the DB you are connected to.)
3. Boot applies all migrations from zero (empty `migration_history` → `LATEST.sql` mirror + history upsert — the standard fresh-DB path).

**New Taskfile task `run:cockroach:local`** (next to `run:cockroach`, line 303):
- `deps: [build:backend:cockroach]`
- sources `.env` + `.env.local`
- runs the reset script
- starts server with log capture and boot-evidence echo:
  ```bash
  MEMOS_DRIVER=cockroach RAG_PIPELINE_ENABLED=true LANCEDB_STORAGE_PROVIDER=cockroach \
  TICKET_EMBEDDING_ENABLED=true RAG_STARTUP_REINDEX_DISABLED=true \
  MEMOS_PORT=5230 ./build/memos --mode dev --data {{.ROOT_DIR}}/build/data
  ```

## 5. Executed run (once changes above are implemented)

### 5.1 Step 1 — Build, start server, **boot-evidence gate**

```bash
task build:backend:cockroach
task run:cockroach:local > /tmp/bchat.log 2>&1 &      # fresh bchat_test + start + log capture
SERVER_PID=$!
```

**Gate (must all pass before any API call):**
- `curl -s -o /dev/null -w "%{http_code}" http://localhost:5230/` → 200 (retry up to 60×2 s)
- `/tmp/bchat.log` contains: `started on port 5230` AND `driver: cockroach` AND `end migrate`
- SQL baseline on `bchat_test` (verified 2026-08-06 — **the fresh-DB baseline is 3 demo tenants, NOT 0**):
  ```sql
  SELECT id, slug FROM agent_tenants ORDER BY id;
  -- expect exactly: 1 demo-home-services, 2 demo-clinic, 3 demo-saas
  -- (StartupSeedPlaygroundDemos, v1.go:120-124 — unconditional, idempotent, no env gate)
  SELECT count(*) FROM agent_vectors;               -- expect 0 (demos are imported, never embedded)
  SELECT count(*) FROM agent_rag_active_versions;   -- expect 0 (no reindex has run)
  ```
- Any of these failing = abort; the environment is wrong (this gate is what v2 lacked). The demo tenants are inert for this test: they hold source files only, no vectors, no active versions.

### 5.2 Step 2 — Bootstrap admin (no SQL, no helper)

Fresh DB → no users → first signup becomes HOST (`HandleSignUp`, auth_service.go:578; blocked only if `DisallowUserRegistration` is set, which fresh DBs default to unset):

```bash
curl -s -c /tmp/bchat_cookies -X POST http://localhost:5230/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | tee /tmp/signup.json
curl -s -X POST -b /tmp/bchat_cookies http://localhost:5230/api/v1/auth/status   # 200, role host
```

**Fallback (contingency only):** if signup is rejected (registration disabled), use the v2 Step 2b SQL reset — but it must NOT be needed on a fresh DB.

### 5.3 Step 3 — Onboard `noli` + KB + widget key (identical to v2)

```bash
curl -s -b /tmp/bchat_cookies -X POST http://localhost:5230/api/v1/agent/onboard \
  -F "tenant_slug=noli" -F "company_name=Noli Me Tangere" -F "vertical=literature" \
  -F "external_kb_file=@/home/chaschel/Documents/go/bchat/docs/templates/examples/rizal/noli.txt" \
  | tee /tmp/onboard.json     # expect tenant id=4 (fresh DB: 3 demo tenants + noli), slug=noli
NOLI_ID=$(python3 -c "import json,sys; print(json.load(open('/tmp/onboard.json'))['tenant']['id'])")
WIDGET_KEY=$(curl -s -b /tmp/bchat_cookies http://localhost:5230/api/v1/agent/noli/config \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['tenant']['widgetKey'])")
```

### 5.4 Step 4 — Reindex (bounded poll) + **exact** SQL evidence

```bash
curl -s -b /tmp/bchat_cookies -X POST "http://localhost:5230/api/v1/agent/noli/reindex?audience_type=external"   # 202
for i in $(seq 1 60); do
  curl -s -b /tmp/bchat_cookies http://localhost:5230/api/v1/agent/noli/reindex/status \
    | python3 -c "import json,sys; d=json.load(sys.stdin); exit(0 if d.get('status')=='completed' else 1)" \
    && { echo "reindex completed after ${i} polls"; break; }
  [ "$i" -eq 60 ] && { echo "REINDEX DID NOT COMPLETE IN 5 MIN" >&2; exit 1; }
  sleep 5
done
```

Exact-count SQL (fresh DB → 3 benchmark demo tenants + noli; zero vectors anywhere before this step):
```sql
SELECT count(*) FROM agent_tenants;                                     -- expect 4 (3 demo + noli)
SELECT content_type, count(*) FROM agent_vectors WHERE tenant_id = <NOLI_ID> GROUP BY content_type; -- 293 kb_section, 0 kb
SELECT count(*) FROM agent_vectors;                                     -- expect 293 (demos: 0 — never embedded)
SELECT tenant_id, audience_type, file_type, version FROM agent_rag_active_versions; -- exactly 1 row: <NOLI_ID> external kb v1
SELECT id, file_type, version FROM agent_source_files WHERE tenant_id = <NOLI_ID>; -- kb, v1 (id=13 on fresh DB: 12 demo files + 1)
```

`<NOLI_ID>` = the parsed tenant id from Step 3 (review nit 3 — applied: never hardcoded; on a fresh DB it is 4 = 3 demo tenants + noli).

### 5.5 Step 5 — Retrieval assertion (PRIMARY — fix proof; same payload as v2)

```bash
curl -s -b /tmp/bchat_cookies -X POST http://localhost:5230/api/v1/admin/rag/search \
  -H "Content-Type: application/json" \
  -d '{"query":"what did maria clara give to the leper?","tenantId":<NOLI_ID>,"topK":5,"audienceType":"external"}' \
  | tee /tmp/rag_search.json
```

**Pass criteria:** results non-empty; **all** chunks `kb_section`; no Translator's Introduction content; top-1 score > 0.5 (record exact; v2 observed 0.500 = the leper scene).

### 5.6 Step 6 — End-to-end chat (customer view; same as v2)

```bash
curl -s -X POST http://localhost:5230/api/v1/agent/noli/chat/ext \
  -H "X-Widget-Key: $WIDGET_KEY" -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$(uuidgen)\",\"message\":\"What did Maria Clara give to the leper?\"}" \
  | tee /tmp/chat_reply.json
```

**Pass criteria:** reply names the gift and the scene (v2: "…her mother's locket…"); **zero** `RAG fallback activated` lines in `/tmp/bchat.log`; record response `metadata`.

### 5.7 Step 7 — Cleanup

- Keep tenant + vectors (user decision, as v2). Fresh `bchat_test` means the dev `bchat` DB is untouched by this whole run.
- Optional: `cockroach sql --url "…/defaultdb?sslmode=disable" -e "DROP DATABASE IF EXISTS bchat_test;"` for a truly clean slate next run.
- Stop server: `kill $SERVER_PID`.

## 6. Cross-driver verification adapters

Unchanged from v2 — primary assertion is API-primary (`POST /api/v1/admin/rag/search`, camelCase payload) and identical on every driver. Per-driver secondary evidence (sqlite: `sqlite3 … agent_vectors`; neon: `psql`; cockroach: §5.4 SQL). The guard (§2) protects postgres/cockroach; sqlite needs none.

## 7. Pass criteria (summary)

1. **Boot evidence:** port 5230, `driver: cockroach`, `end migrate`, fresh-DB baseline (3 demo tenants, 0 vectors) — §5.1 gate.
2. **Bootstrap:** signup admin = HOST, no SQL reset used.
3. **Exact counts:** 4 tenants (3 auto-seeded demos + noli); noli: 293 `kb_section` / 0 `kb`; demos: 0 vectors; active-version pointer = single noli row external/kb/v1.
4. **Fix proof:** search results all `kb_section`, no Intro content, top-1 score > 0.5 (leper scene).
5. **Grounded chat:** locket answer; **0** `RAG fallback activated` lines.
6. **Safety invariant (the point of v3):** zero connections to the cloud cluster — guaranteed by the §2 guard (app refuses), §3 overlay (local DSN wins), and the boot-evidence gate (fails loudly otherwise).

## 8. Not run / blocked this cycle

Unchanged from v2: sqlite ready but not run; Neon needs a local `run` config; MySQL blocked (`errNotImplemented`). Cross-driver reuse relies on the API-primary assertion, §6.

## 9. Adversarial review

v2 chain (`plan_test_review.md` GO-WITH-CHANGES; `plan2_test_review.md` GO) found zero defects in v2's API steps — the failures were **environmental** (DSN, port), which this revision addresses structurally. After implementation of §2–§4, run one targeted review of the guard + runbook against `plan_test_review_prompt.md` (categories A–H) before execution.

## 10. Results (fill in after execution)

| Check | Result |
|---|---|
| Boot evidence (port 5230, driver, migrate, fresh DB) | ✅ port 5230 + `driver: cockroach` + DSN `bchat_test`; baseline 3 demo tenants (demo-home-services, demo-clinic, demo-saas), 0 vectors, 0 active versions — matches §5.1 gate (gate caught plan's "0 tenants" assumption was wrong) |
| Signup bootstrap → HOST (no SQL reset) | ✅ `POST /auth/signup` admin → role HOST (auth_service.go:578 first-user path); no SQL, no helper |
| Onboard: tenant id=4, slug=noli, external audience | ✅ id=4 (3 demos + noli), external audience, KB imported |
| Widget key fetched | ✅ `7076913c-28d4-43d8-8a36-b0ccaf03edf9` |
| Reindex completed within poll bound | ✅ 293/293 chunks after 3 polls (~15 s) |
| Exact counts: 4 tenants / noli 293 kb_section + 0 kb / single version pointer | ✅ tenants=4; noli: 293 kb_section, 0 kb; total vectors 293 (demos 0); active_versions = exactly 1 row (4/external/kb/v1); source file id=13 v1 |
| Search: all kb_section, no Intro, top-1 score | ✅ 5/5 kb_section; top-1 0.499951 = leper scene ("He's a leper," Iday told her — same chunk as v2's 0.49997); no Translator's Introduction content |
| Chat reply (locket) + 0 fallback lines | ✅ "Maria Clara gave the leper her locket, which her father had given her." — 0 `RAG fallback activated` in /tmp/bchat.log |
| Guard behavior spot-check: remote DSN rejected on boot (negative test) | ✅ `--mode dev` + cloud DSN → panic "refusing to start in dev mode: DSN host …is not loopback"; `MEMOS_ALLOW_REMOTE_DSN=true` bypasses (no refusal); prod-mode + remote DSN allowed (unit test) |
| Verdict | ✅ **PASS** — hermetic run, zero connections to the cloud cluster, fix proven end-to-end on local CRDB |

## 11. Notes on the v2 cloud side effect (no action)

During v2 execution, first boot sourced `.env` and connected to the Cockroach Labs Cloud cluster (`great-goat-30894.j77.cockroachlabs.cloud`), running the idempotent 0.36.1 DDL migration there. **Decision: no action.** The migration is idempotent and forward-compatible (verified pre-existing pattern; same DDL applied cleanly to local 0.35.1→0.36.1). Prod data was not modified beyond adding the new-schema tables. v3's guard makes this class of accident impossible for future local runs. Optional follow-up (not this cycle): read-only SQL audit on the cluster confirming only expected tables were added.
