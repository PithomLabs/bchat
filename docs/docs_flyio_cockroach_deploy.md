# Fly.io + CockroachDB Deploy Runbook

**Scope:** deploying bchat on Fly.io with a CockroachDB backend (application data + LanceDB vector store, native `VECTOR` + C-SPANN indexes). Companion to `bugs/057/code.md` and `bugs/057/pre_code.md`; the Neon/Postgres equivalent lives in `docs/DOCS_DEPLOY_FLY.MD`.

**Topology:** parallel Fly app `bchat-crdb` alongside the existing `bchat-pg` (Neon). This is a migration/demo strategy, not the intended permanent topology — a permanent deployment picks one profile.

---

## 1. Pre-flight

| Check | Command |
|-------|---------|
| Local build | `task build:backend:cockroach` (tags `cockroach`, pure Go — no CGO/liblancedb) |
| Migration parity | `bash scripts/validate-parity.sh` (includes cockroach↔postgres pair) |
| Cockroach SQL compat | `bash scripts/validate-cockroach-compat.sh` (exit 0 = clean) |
| Environment | `COCKROACH_DSN`, `LANCEDB_STORAGE_PROVIDER=cockroach` |
| Local cluster (optional) | `task crdb:up` + `task crdb:migrate` (compose, v25.2.21; `SET feature.vector_index.enabled = true` on v25.x) |

## 2. Cluster (console-first, 2 regions)

Multi-region Basic clusters are created **in the Cockroach Cloud Console only** (`ccloud cluster create basic` accepts exactly one region):

1. Console → Create Cluster → Basic → AWS → **2 regions: `us-east-1` (primary) + `us-west-2`** (nearest to Fly `sjc`; primary is editable later).
2. Create a SQL user + password in the console.
3. DSN: `postgresql://user:pass@<cluster>-<id>.<org>.crdb.cloud:26257/bchat?sslmode=verify-full` (CA is Let's Encrypt — system roots suffice).
4. Vector index: default `true` on stable (v26.x); set explicitly on v25.x:
   `cockroach sql --url "$COCKROACH_DSN" -e "SET CLUSTER SETTING feature.vector_index.enabled = true;"` (no-op on v26).
5. `task crdb:cluster:bootstrap` prints the guided flow.

Automated post-creation steps (cluster must already exist): `deploy/ccloud/setup.sh` (user, allowlist, DSN).

## 3. Fly secrets

```bash
bash scripts/fly-cockroach-secrets.sh   # interactive: COCKROACH_DSN, OPENROUTER_API_KEY, ENCRYPTION_MASTER_KEY
```

Then create the app if missing: `fly apps create bchat-crdb`.

## 4. Deploy

```bash
task deploy:cockroach
```

`scripts/crdb-deploy.sh` runs: `build:backend:cockroach` → `validate-parity.sh` → `validate-cockroach-compat.sh` → `fly deploy -c fly_cockroach.toml` → `/healthz` poll (15s grace) → `task crdb:verify` → `task verify:production`. Every stage logs to `build/crdb-deploy.log`; the script is re-runnable after any failure.

**First boot:** `FORCE_REINDEX_ON_STARTUP='true'` performs the initial A1 reindex (vector-index backfill blocks writes — safe pre-traffic). Set it to `false` in `fly_cockroach.toml` after the first successful boot.

## 5. Verify

| Level | Task | Asserts |
|-------|------|---------|
| Production P1–P6 | `task crdb:verify` | `SELECT 1`, `version()` = Cockroach, `migration_history` = 1 row (0.35.1), `nextval()` defaults, vector index enabled + `agent_vectors` indexed, `/healthz` 200 |
| App-first smoke | `task verify:production` | signin → tenant select → onboard `verify-<ts>` → KB import → reindex → RAG search ≥1 hit → destroy test tenant (set `BCHAT_URL`/`BCHAT_USER`/`BCHAT_PASS`; `--keep` to retain) |

## 6. Regions & backups (Basic tier)

- 2 regions survive zone failure; 3 survive regional failure. Geographically-local reads; write locality governed by primary.
- Backups every 24h, 30-day retention (`ccloud cluster backup list hackathon-demo`).
- Databases inherit cluster regions automatically; no per-database region config needed.

## 7. Diagnostics

- `fly -a bchat-crdb logs` / `fly logs | grep -E "RAG|reindex|migration"` — boot, migration, reindex traces.
- `curl https://bchat-crdb.fly.dev/healthz` — liveness.
- `ccloud cluster sql hackathon-demo -e "SHOW REGIONS;"` — topology proof (screenshot for demos).
- `task crdb:verify` — schema-level invariants (nextval, no `unique_rowid`, history).
- `cockroach sql --url "$COCKROACH_DSN" -e "SELECT * FROM migration_history;"` — applied migrations.

## 8. Hardening (optional, ~$3.60/mo)

Basic ships with allowlist `0.0.0.0/0`. `task crdb:harden`:

1. `fly ips allocate-egress -a bchat-crdb` → egress IP
2. allowlist `<ip>/32` (`--sql --ui`), remove `0.0.0.0/0`
3. mandatory connectivity verification — if broken (community-reported after egress allocation), it auto-reverts and restores `0.0.0.0/0`.

## 9. Rollback to Neon (demo capability)

```bash
export DATABASE_URL="postgresql://user:pass@ep-xxx.neon.tech/neondb?sslmode=require"
task rollback:postgres
```

Flips `bchat-crdb` to the Neon profile (`fly_pg-rollback.toml`, `MEMOS_DRIVER=postgres`, `DATABASE_URL` secret, `COCKROACH_DSN` unset) and re-runs `verify:production`. **No schema downgrade** — CRDB data stays at the last applied schema; re-cutover is a plain forward migration. RAG env resets to Neon values (S3/Tigris).

## 10. Demo runbook (distributed SQL)

1. `task db:local` — SQLite works (default profile).
2. `task deploy:postgres` — Fly+Neon works (same Taskfile, live today).
3. `task deploy:cockroach` — Fly+Cockroach works (one command).
4. `task rollback:postgres` — back to Neon works.

Distributed SQL is shown **app-first** (the §5 smoke on the 2-region cluster), with `SHOW REGIONS` as a supporting clip. Native features to mention: automatic retries (concurrent outbox claims → 40001 in logs), native `VECTOR(1536)` + C-SPANN index (`SHOW CREATE TABLE agent_vectors`), online schema change (ALTER while a query loop runs — zero blocking).
