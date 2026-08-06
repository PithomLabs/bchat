# Local Database Testing Reference — bchat (definitive)

**Purpose:** single authoritative resource for testing the bchat app + database locally, on **any driver** — sqlite, postgres (incl. Neon), CockroachDB, and the future MySQL path. Covers the safety model (why local runs can never touch production), the hermetic-DB runbook, exact API payloads, per-driver SQL evidence, expected values, gotchas, and troubleshooting.

**Origin:** distilled from the Bug 060 end-to-end test cycle — `plan_test.md` (v1) → `plan2_test.md` (v2, executed, 5 deviations) → `plan3_test.md` (v3, hermetic, executed PASS). Every claim below was verified against source or by execution on 2026-08-06.

**Status:** current. Update this file whenever a runbook step, expected value, or safety rule changes.

---

## 1. Scope and driver support

| Driver | Store agents | Vector storage | Local run task | DSN source | Status |
|---|---|---|---|---|---|
| sqlite | ✅ `store/db/sqlite/agent.go` | LanceDB (`LANCEDB_STORAGE_PROVIDER=local`) | `task run:rag` | file `build/data/memos_dev.db` | ✅ ready |
| postgres (local/Neon) | ✅ `store/db/postgres/agent.go` | LanceDB | compose cluster (`scripts/docker-compose.postgres.yml`) + `MEMOS_DRIVER=postgres` | `DATABASE_URL` (profile.go:97–102) | ✅ ready, needs a local run task |
| cockroach | ✅ via `postgres.NewCockroachDB` | **in-DB** `agent_vectors` | `task run:cockroach:local` (hermetic) | `COCKROACH_DSN` (profile.go:104–109) | ✅ executed (Bug 060 v3) |
| mysql | ❌ 99× `errNotImplemented` in `store/db/mysql/agent.go` | none | — | — | ⛔ blocked (agents unsupported) |

**The retrieval assertion is API-primary** (`POST /api/v1/admin/rag/search`) — identical payload, route, and response shape on every driver. Only the *secondary SQL evidence* differs per driver (§9). A driver passes if the API passes; SQL is corroboration.

## 2. Principles

1. **Production-safe by construction** — a local test must be *unable* to touch the cloud cluster, not merely careful not to (§3).
2. **Hermetic** — every run starts from a fresh database with zero assumptions; assertions are exact counts, not baseline diffs.
3. **Fail loud** — boot evidence is gated: wrong port, wrong DSN, wrong baseline = abort, no API calls.
4. **Deterministic** — same input (novel KB, same query) → same evidence every run.

## 3. Safety layers (why local runs can't hit production)

Four independent layers; any one of them alone would stop the accident that v2 suffered (first boot connected to the Cockroach Labs Cloud cluster and ran an idempotent migration there).

### 3.1 App-level loopback-DSN guard (the core fix)

`internal/profile/profile.go`, in `Profile.Validate()` after DSN resolution:

```go
if p.IsDev() && (p.Driver == "cockroach" || p.Driver == "postgres") && os.Getenv("MEMOS_ALLOW_REMOTE_DSN") != "true" {
    if !isLoopbackDSN(p.DSN) {
        return errors.Errorf("refusing to start in %s mode: DSN host %q is not loopback ...", ...)
    }
}
```

Semantics (all verified by unit tests in `internal/profile/profile_test.go`):
- `p.IsDev()` = `Mode != "prod"` — covers `dev` **and** `demo` (the default when no `--mode` is passed). A bare `./build/memos --driver=cockroach` is guarded too.
- `isLoopbackDSN`: `net/url.Parse` + `net.SplitHostPort`; hostname ∈ {`localhost`, `127.0.0.1`, `::1`} (IPv6 brackets handled). **Parse failure → fail closed.**
- Opt-out: `MEMOS_ALLOW_REMOTE_DSN=true` (for legit remote-dev against staging).
- `mode=prod` (fly deploys) unaffected — cloud DSNs keep working in production.
- sqlite needs no guard (local file).

### 3.2 Env layering — `.env.local` overlay

- `.env` (gitignored) holds deploy/cloud config — including the **cloud** `COCKROACH_DSN`. Never use it directly for local runs.
- `.env.local` (gitignored; template `.env.local.example` committed) holds local overrides. All 9 Taskfile run tasks that source `.env` now also overlay `.env.local` (local wins):
  ```bash
  set -a && . .env && set +a
  [ -f .env.local ] && set -a && . .env.local && set +a
  ```
- `.env.local.example`:
  ```bash
  COCKROACH_DSN="postgresql://root@localhost:26257/bchat_test?sslmode=disable"
  ```

### 3.3 Hermetic DB per run

- Cockroach: `task run:cockroach:local` → `scripts/init-local-cockroach-db.sh` → `DROP DATABASE IF EXISTS bchat_test; CREATE DATABASE bchat_test;` (executed via `defaultdb` — you cannot drop the DB you're connected to). The local compose cluster (`task crdb:reset`) is a throwaway; prod can never be the target of the drop.
- sqlite: fresh run = delete `build/data/memos_dev.db` (the app recreates and migrates it).
- postgres: same drop/create pattern against the local compose cluster (`scripts/docker-compose.postgres.yml`).

### 3.4 Port binding

- Default port is **8081** (`bin/memos/main.go:210,214`). fly.prod uses 5230.
- Local test runs set **`MEMOS_PORT=5230`** (viper env prefix `memos` + `AutomaticEnv`, main.go:258–259 maps `MEMOS_PORT` → `port`). This aligns with `scripts/rag_query.sh` (API_BASE 5230) and other tooling.
- Never assume the port — the boot-evidence gate reads it from the server log (§5).

## 4. Platform setup

**Preconditions:**
- Tools: `task`, `curl`, `python3`, `uuidgen`, plus per driver: `cockroach` / `sqlite3` / `psql`.
- `.env` present (sourced by all run tasks) with `OPENROUTER_API_KEY`, `LLM_MODEL`, `EMBEDDING_PROVIDER`, `RAG_STARTUP_REINDEX_DISABLED`. **Do not rely on `.env` for local DSNs or credentials** — those are cloud-facing.
- `.env.local` present with the local DSN (copy from `.env.local.example`).
- Test KB: `docs/templates/examples/rizal/noli.txt` (1,047,875 B, 20,975 lines).

**Cockroach local cluster:**
```bash
task crdb:up      # docker compose -f scripts/docker-compose.cockroach.yml up -d --wait
task crdb:init    # idempotent cluster settings (incl. feature.vector_index.enabled)
task crdb:reset   # wipe + restart to A1 state (down -v, up, init)
```
Local node: insecure `root@localhost:26257`, v26.2.1.

**Postgres local cluster:** `docker compose -f scripts/docker-compose.postgres.yml up -d --wait` (local task alias not yet added — see §14).

**Run entry point (cockroach):** `task run:cockroach:local` — builds with `-tags cockroach`, sources `.env` + `.env.local`, resets `bchat_test`, starts `./build/memos --mode dev` with `MEMOS_DRIVER=cockroach RAG_PIPELINE_ENABLED=true LANCEDB_STORAGE_PROVIDER=cockroach TICKET_EMBEDDING_ENABLED=true RAG_STARTUP_REINDEX_DISABLED=true MEMOS_PORT=5230`.

## 5. Boot-evidence gate (before ANY API call)

Start with log capture:
```bash
task run:cockroach:local > /tmp/bchat.log 2>&1 &   SERVER_PID=$!
```

Gate — all must pass, else abort:
1. `curl -s -o /dev/null -w "%{http_code}" http://localhost:5230/` → 200 (retry up to 60×2 s)
2. `/tmp/bchat.log` contains: `started on port 5230` AND `driver: cockroach` AND `end migrate`
3. SQL baseline on the hermetic DB (see §6 — the fresh-DB baseline is **not** empty)

This gate is what v2 lacked; it catches wrong port, wrong DSN host, and non-fresh databases.

## 6. Fresh-DB baseline reality (playground demo seeding)

On any fresh DB, startup seeds **3 playground demo tenants** (`StartupSeedPlaygroundDemos`, v1.go:120–124 → `ensurePlaygroundDemo`, playground.go:355). Unconditional, idempotent, **no env gate**.

```sql
SELECT id, slug FROM agent_tenants ORDER BY id;
-- 1 demo-home-services, 2 demo-clinic, 3 demo-saas
SELECT count(*) FROM agent_vectors;              -- 0  (demos are imported, NEVER embedded)
SELECT count(*) FROM agent_rag_active_versions;  -- 0  (no reindex has run)
```

Each demo tenant gets 4 source files (external kb/policy, internal kb/policy, version 1) — 12 files total. The demos are **inert** for testing: they hold files only; without a reindex they never produce vectors. Consequences:
- First real tenant gets `id=4` (cockroach fresh run; id 1–3 taken by demos).
- Exact counts are per-tenant-scoped (`WHERE tenant_id = ...`), never global "1 tenant".
- noli's source file is `id=13` on a fresh DB (12 demo files + 1).

## 7. Auth bootstrap

**No SQL, no helpers, no pre-seeded credentials.** Fresh DB has zero users → first signup becomes **HOST** (`HandleSignUp`, auth_service.go:578; role decision at 610–618; blocked only if `DisallowUserRegistration` is set, which fresh DBs default to unset).

```bash
curl -s -c /tmp/bchat_cookies -X POST http://localhost:5230/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | tee /tmp/signup.json
# expect: {"id":"1", ..., "role":"HOST"}

# Signup does NOT set a session cookie — sign in:
curl -s -c /tmp/bchat_cookies -X POST http://localhost:5230/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"password_credentials":{"username":"admin","password":"admin123"}}' | tee /tmp/signin.json

curl -s -X POST -b /tmp/bchat_cookies http://localhost:5230/api/v1/auth/status
```

Facts (all verified):
- The token is the **`memos.access-token` cookie** (HttpOnly) — NOT an `accessToken` field in the body. AuthMiddleware accepts the cookie (acl.go:152); Bearer also works.
- `/auth/status` is **POST** (v1.go:219) and returns the **user object at top level** — `role` is `user.role`'s top-level key, not nested under `"user"`.
- Signin rate limit: 5 attempts/min (`login_ratelimit.go:25`) — don't loop retries.
- Contingency fallback (only needed if signup is disabled): SQL password reset with a bcrypt hash (python `bcrypt`/`htpasswd` are absent on this machine — use a tiny Go helper with `golang.org/x/crypto/bcrypt`, then `UPDATE "user" SET password_hash='...' WHERE username='admin';` on the hermetic DB).

## 8. Onboard a tenant + KB + widget key

```bash
curl -s -b /tmp/bchat_cookies -X POST http://localhost:5230/api/v1/agent/onboard \
  -F "tenant_slug=noli" -F "company_name=Noli Me Tangere" -F "vertical=literature" \
  -F "external_kb_file=@/home/chaschel/Documents/go/bchat/docs/templates/examples/rizal/noli.txt" \
  | tee /tmp/onboard.json
# expect: {"success":true, "tenant":{"id":4,"slug":"noli"}, "audiences":{"external":{...}}}

NOLI_ID=$(python3 -c "import json,sys; print(json.load(open('/tmp/onboard.json'))['tenant']['id'])")
WIDGET_KEY=$(curl -s -b /tmp/bchat_cookies http://localhost:5230/api/v1/agent/noli/config \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['tenant']['widgetKey'])")
```

Facts:
- `HandleOnboard` (handlers.go:1394) creates the tenant (auto GUID + widget_key) and imports the KB per audience. The **admin group has no body limit** (1 MB OK; the 16 KB limit is publicGroup-only, v1.go:292).
- Widget key response nesting: `{"tenant": {"widgetKey": ...}}` (handlers.go:779–786).
- Always parse `NOLI_ID` — never hardcode it (it is 4 on a fresh cockroach run, but only because of demo seeding; parsing keeps the runbook robust).

## 9. Reindex + exact SQL evidence

```bash
curl -s -b /tmp/bchat_cookies -X POST "http://localhost:5230/api/v1/agent/noli/reindex?audience_type=external"
# 202 Accepted (async; 30-min window)

# Bounded poll — max 60 × 5 s = 5 min; abort loudly if not done:
for i in $(seq 1 60); do
  curl -s -b /tmp/bchat_cookies http://localhost:5230/api/v1/agent/noli/reindex/status \
    | python3 -c "import json,sys; d=json.load(sys.stdin); exit(0 if d.get('status')=='completed' else 1)" \
    && { echo "reindex completed after ${i} polls"; break; }
  [ "$i" -eq 60 ] && { echo "REINDEX DID NOT COMPLETE IN 5 MIN" >&2; exit 1; }
  sleep 5
done
```

Observed: 293/293 chunks after 3 polls (~15 s, batch 2/2).

**Version-gate pre-check (mandatory):** search endpoints gate on `resolveQueryVersion` (service.go:5189). Successful reindex writes the pointer via `UpsertAgentRAGActiveVersion` (service.go:657). If the pointer is missing, abort — search would return empty regardless of any fix.

**Exact SQL evidence (per driver):**

| Driver | Command | Expected |
|---|---|---|
| cockroach | `cockroach sql --url "postgresql://root@localhost:26257/bchat_test?sslmode=disable" -e "SELECT content_type, count(*) FROM agent_vectors WHERE tenant_id=<NOLI_ID> GROUP BY content_type;"` | `kb_section 293` (0 bare `kb`) |
| sqlite | `sqlite3 build/data/memos_dev.db "SELECT content_type, count(*) FROM agent_vectors WHERE tenant_id=<NOLI_ID> GROUP BY content_type;"` | same |
| postgres | `psql "$DATABASE_URL" -c "SELECT content_type, count(*) FROM agent_vectors WHERE tenant_id=<NOLI_ID> GROUP BY content_type;"` | same |

Pointer + source-file check (cockroach):
```sql
SELECT tenant_id, audience_type, file_type, version FROM agent_rag_active_versions;
-- exactly 1 row: <NOLI_ID> external kb 1
SELECT id, file_type, version FROM agent_source_files WHERE tenant_id = <NOLI_ID>;
-- id=13, kb, 1 (fresh DB)
```

Full fresh-DB global picture after reindex: `agent_tenants` = 4 (3 demos + noli), `agent_vectors` = 293 total, `agent_rag_active_versions` = 1 row.

## 10. Retrieval assertion (PRIMARY — fix proof)

```bash
curl -s -b /tmp/bchat_cookies -X POST http://localhost:5230/api/v1/admin/rag/search \
  -H "Content-Type: application/json" \
  -d '{"query":"what did maria clara give to the leper?","tenantId":<NOLI_ID>,"topK":5,"audienceType":"external"}' \
  | tee /tmp/rag_search.json
```

Facts:
- Endpoint `HandleTestRAGSearch` (handlers.go:4910); request fields are **camelCase** — `tenantId`, `audienceType`, `topK`, `fileType`, `sourceVersion` (handlers.go:4699).
- Leaving `fileType` empty exercises the fixed path: empty ContentTypes → **no `content_type` filter** in `buildCockroachSearchQuery` (the Bug 060 fix; pre-fix rendered `IN ('')`, matching zero rows).
- Determinism option: pass `"sourceVersion": V` (from `agent_source_files.version`) to bypass pointer resolution entirely.

**Pass criteria (v2/v3 verified):**
1. `results` non-empty (`totalResults > 0`);
2. every chunk `contentType == "kb_section"` (fresh DB has no legacy `kb` rows);
3. no chunk content from the Translator's Introduction (the pre-fix fallback signal);
4. top-1 score ≈ 0.5 (0.499951 on 2026-08-06 v3; 0.49997 v2 — same leper scene chunk: *"He's a leper," Iday told her*; minor embedding nondeterminism expected).

## 11. Chat assertion (customer view)

```bash
curl -s -X POST http://localhost:5230/api/v1/agent/noli/chat/ext \
  -H "X-Widget-Key: $WIDGET_KEY" -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$(uuidgen)\",\"message\":\"What did Maria Clara give to the leper?\"}" \
  | tee /tmp/chat_reply.json
```

**Pass criteria:**
1. reply names the gift and the scene — observed: *"Maria Clara gave the leper her locket, which her father had given her."* (v2) / *"…her mother's locket…"* (v3 wording variants);
2. **zero** `RAG fallback activated` lines in `/tmp/bchat.log` (exact string, service.go:3736, emitted via slog → stderr — captured because the server runs with output redirected);
3. record response `metadata` retrieval indicators.

## 12. Cleanup

- Keep the tenant + vectors (default; enables follow-up debugging).
- Truly clean slate: drop the hermetic DB (`bchat_test` via `defaultdb`), or for sqlite delete the dev `.db` file — the next `run:cockroach:local` resets it anyway.
- Stop server: `kill $SERVER_PID` (or `pkill -f "build/memos --mode dev"`).

## 13. Expected values — fresh-DB summary (cockroach run)

| Item | Expected |
|---|---|
| Boot port line | `Version 0.35.0 has been started on port 5230` |
| Baseline tenants | 3 (demo-home-services, demo-clinic, demo-saas) |
| Baseline vectors / active versions | 0 / 0 |
| First real tenant | id=4, slug=noli (parse, don't hardcode) |
| Reindex | 293/293, ~3 polls × 5 s |
| Vectors | 293 `kb_section`, 0 `kb` |
| Active-version pointer | 1 row: 4 / external / kb / 1 |
| Source file | id=13, kb, v1 |
| Search | 5/5 `kb_section`, top-1 ≈ 0.5 leper scene, no Intro |
| Chat | locket answer, 0 fallback lines |

## 14. Cross-driver adapters & how to run a new driver

**Primary assertion (identical every driver):** §10. **Secondary evidence:** §9 table.

| Step | sqlite | postgres (local) | cockroach |
|---|---|---|---|
| Fresh DB | `rm -f build/data/memos_dev.db` | drop/create via compose cluster | `task run:cockroach:local` (script does it) |
| Run task | `task run:rag` | (add `run:postgres:local` analogous to §4) | `task run:cockroach:local` |
| DSN env | none (file) | `DATABASE_URL` | `COCKROACH_DSN` (`.env.local`) |
| SQL client | `sqlite3` | `psql` | `cockroach sql` |
| Vector store | LanceDB local | LanceDB | in-DB `agent_vectors` |

Everything else (§5–§12) is driver-independent: same endpoints, same payloads, same assertions. MySQL: blocked — 99 `errNotImplemented` in `store/db/mysql/agent.go`; revisit when agents are implemented.

## 15. Gotchas & CRDB facts (all verified)

- **`<=>` is cosine *distance*** (0..1); similarity = `1 - distance`. App scoring math confirmed correct.
- Existing `idx_agent_vectors_embedding` uses default opclass **`vector_l2_ops`** — `<=>` queries run **brute-force but correctly**. Not a test blocker; tracked as bugs/061 (future index: `(tenant_id, embedding vector_cosine_ops)`, and vector indexes are only used when all prefix columns are equality-constrained).
- `CREATE VECTOR INDEX` needs `feature.vector_index.enabled = true` (set by `scripts/crdb-init.sql`).
- Adding a vector index to a **non-empty** table blocks writes during backfill (needs `sql_safe_updates=false`) — apply indexes on fresh/empty tables.
- Avoid large manual vector batch inserts (performance degradation); the app's own write path (EMBEDDING_BATCH_SIZE) is what tests exercise.
- `RAG_STARTUP_REINDEX_DISABLED=true` IS consumed (service.go:280): it skips ALL automatic startup reindexing. Keep it set in tests so the only reindex is the one you trigger. (A review once claimed it was dead code — rebutted; verify: startup auto-bootstrap only fires when the vector DB is empty.)
- Startup demo seeding (§6) exists on every driver's fresh DB — plan counts around it.
- `.env` `COCKROACH_DSN` is the **cloud** cluster URL — never run `task run:cockroach` (non-local) and assume it's local. Use `run:cockroach:local`.
- Port default is 8081; local tooling (rag_query.sh) assumes 5230 — set `MEMOS_PORT=5230` explicitly.
- Chat fallback path (pre-fix signal): first-6k-tokens fallback when retrieval is empty; detected via the log scan, not by reply content alone.
- Embedding cost per reindex of noli.txt ≈ $0.005 (OpenRouter, ~293 chunks).

## 16. Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Boot panic: `refusing to start in dev mode: DSN host "…" is not loopback` | `COCKROACH_DSN`/`DATABASE_URL` points at cloud/remote | Put the local DSN in `.env.local`; or `MEMOS_ALLOW_REMOTE_DSN=true` (only for legit remote-dev) |
| Server on 8081, not 5230 | `MEMOS_PORT` not set | Set `MEMOS_PORT=5230` (or use `run:cockroach:local` which sets it) |
| Signin 401 / rate-limited | wrong creds (`.env` creds are cloud-only) | signup on fresh DB (HOST) or SQL reset; wait out 5/min limit |
| `/auth/status` shows no role | read the body wrong — user object is top-level | check `json.user` is wrong; read `json.role` directly |
| Search returns 0 results | version pointer missing (reindex failed/incomplete) | check `agent_rag_active_versions`; reindex again; abort if still missing |
| Results contain `kb` or Intro content | stale/non-fresh DB or old build | drop/recreate hermetic DB; rebuild with `-tags cockroach` |
| Reindex never completes | poll bound too small / embeddings API down | check `/tmp/bchat.log` for embed errors; raise bound |
| Tenants count ≠ 3 at boot | DB not fresh (leftover from earlier run) | `task run:cockroach:local` resets; or drop `bchat_test` manually |
| 5230 busy | leftover server | `pkill -f "build/memos --mode dev"` |
| `agent_vectors` has 0 rows after reindex | RAG flags missing on run (e.g., `MEMOS_DRIVER` unset) | use the task, not a bare `./build/memos` |
| Signup rejected | `DisallowUserRegistration` set (non-fresh DB) | use a fresh DB, or SQL reset fallback (§7) |

## 17. Reusable process (how a driver test gets from plan to verdict)

1. **Write a plan** in `bugs/<n>/plan<k>_test.md` following §5–§12 as the skeleton: steps, exact payloads, pass criteria, expected values.
2. **Adversarial review** — run `plan_test_review_prompt.md` (categories A–H: correctness of claims, false-pass risk, shared-state corruption, determinism, env assumptions) against the plan. Dispositions go in the plan (valid findings applied; invalid rebutted with source lines).
3. **Execute** — boot-evidence gate first, then the runbook; record results in the plan's results table **including execution deviations** (the deviations of v2 are what hardened v3).
4. **Verdict** — PASS only if every gate and criterion passed; record exact observed values (scores, counts, ports, timestamps).

History that produced this doc (read them for context): `plan_test.md` → `plan_test_review.md` (GO-WITH-CHANGES, 12 findings) → `plan2_test.md` (executed; 5 deviations: port 8081 not 5230; `.env` DSN is cloud; local creds ≠ `.env` creds; chunker produces only `kb_section`; counts are local-DB numbers) → `plan2_test_review.md` (GO) → `plan3_test.md` (hermetic; guard; fresh DB; exact counts) → `plan3_test_review.md` (GO, 3 nits, all applied) → executed PASS.

## 18. Key source references

| File | Lines | What |
|---|---|---|
| `internal/profile/profile.go` | 66–130 | DSN resolution + loopback guard |
| `internal/profile/profile_test.go` | — | Guard test matrix |
| `bin/memos/main.go` | 210, 214, 258–259, 296 | Port default 8081; `MEMOS_PORT` binding |
| `Taskfile.yml` | 109–177, 293–325 | run tasks; `.env.local` overlay; `run:cockroach:local` |
| `scripts/init-local-cockroach-db.sh` | — | Hermetic DB reset |
| `scripts/rag_query.sh` | — | Multi-mode RAG query tool (search/source/read/grep/all), API_BASE 5230 |
| `server/router/api/v1/v1.go` | 120–124, 216, 219, 292 | Demo seeding call; signup; status POST; public body limit |
| `server/router/api/v1/agent/auth_service.go` | 578, 610–618, 707 | Signup → HOST; cookie issuance |
| `server/router/api/v1/agent/handlers.go` | 779–786, 1394, 4699, 4910 | Config nesting; onboard; search fields; test-search |
| `server/router/api/v1/agent/service.go` | 280, 657, 3736, 5189, 5238 | Startup-reindex flag; version pointer; fallback log; version resolution |
| `server/router/api/v1/agent/playground.go` | 355–439, 481–492 | Demo seeding semantics |
| `server/router/api/v1/agent/vectordb_cockroach.go` | 388+ | Bug 060 fix (empty ContentTypes → no filter) |
| `store/db/mysql/agent.go` | — | 99 `errNotImplemented` → MySQL blocked |
| `scripts/crdb-init.sql` | — | Cluster settings (vector index feature) |
