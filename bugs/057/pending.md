# Bug 057 — Pending Items & Verification Plan (live deployment phase)

**Date:** 2026-08-02
**Purpose:** Track what remains to be done and tested in the live-deployment phase of the CockroachDB profile, and what to expect next. Companion to `pre_code.md` (§6–§8) and `code.md` (§8).
**Status:** Live-1 + Live-2 complete (cluster provisioned, app + secrets staged) — **Live-3 deploy in progress**. All local gates remain green.

---

## 1. Status Snapshot

**Done (verified locally):**
- pre_code.md steps 1–10, 12; all local gates green: P0 (2.48s), e2e (109.47s) + boot idempotency, `crdb:test`, parity + compat exit 0, both builds, vet, gofmt (touched files clean)
- §7 exit checklist: **9/11 boxes checked**; "Fly deploy works" + "Rollback demonstrated" open (both need live execution)
- Artifacts in place: `scripts/crdb-deploy.sh`, `scripts/verify-production.sh`, `fly_pg-rollback.toml`, Taskfile verbs (`deploy:cockroach/postgres`, `verify:production`, `rollback:postgres`, `crdb:harden`), README + `docs/docs_flyio_cockroach_deploy.md`
- `ccloud` v0.6.12 installed (`~/.local/bin/ccloud`) + authenticated (Pithom Labs / org-32ndt)

**Done (live, 2026-08-02):**
- Cluster `great-goat` CREATED — Cloud SERVERLESS (Basic), AWS, v26.2.1, spend limit $17, **3 regions** (us-east-1, us-east-2, **us-west-2 primary**); `silver-fish` untouched
- cockroach client binary **v26.2.0** installed to `~/.local/bin` (download URL pattern changed: `binaries.cockroachdb.com/cockroach-v26.2.0.linux-amd64.tgz` — **no `/cockroach/` path segment**; all old-style URLs 404)
- SQL user `bchat_user` created (generated password, stored in `/tmp/opencode/bchat_pw` + `.env`); allowlist `0.0.0.0/0 --sql` confirmed
- Database `bchat` created via `cockroach sql` (see §4 gap); round-trip `SELECT version()` = v26.2.1 OK; `feature.vector_index.enabled` = `true`
- `COCKROACH_DSN` written to `.env` (gitignored); `CLUSTER_NAME=great-goat deploy/ccloud/setup.sh` printed export lines
- Fly app `bchat-crdb` created (fly, personal org)
- Secrets **staged** on `bchat-crdb`: COCKROACH_DSN / ENCRYPTION_MASTER_KEY / OPENROUTER_API_KEY (status `Staged` — normal; Fly applies them on next deploy)

**In flight (this phase):** `deploy:cockroach`, admin signup, `verify:production`, native §6.2 checks, `rollback:postgres`, close-out docs.

---

## 2. Pending Items

| # | Item | Owner | Blocked by | Expected |
|---|------|-------|-----------|----------|
| 1 | Create Basic cluster in Cockroach Cloud console | **user** | console action | ✅ **DONE** — `great-goat`, CREATED, AWS, v26.2.1, 3 regions (us-east-1/us-east-2/us-west-2 primary) |
| 2 | `fly auth login` | **user** | browser auth | ✅ **DONE** — `gani.mendoza@gmail.com` |
| 3 | SQL user + allowlist + DSN capture | me | #1 | ✅ **DONE** — `bchat_user` + `bchat` DB + allowlist `0.0.0.0/0 --sql` + DSN in `.env` |
| 4 | Vector-index check | me | #1 | ✅ **DONE** — `feature.vector_index.enabled` = `true` (default on v26.2.1, no-op) |
| 5 | `deploy/ccloud/setup.sh` | me | #3 | ✅ **DONE** — printed export lines |
| 6 | `fly apps create bchat-crdb` | me | #2 | ✅ **DONE** — app exists |
| 7 | `scripts/fly-cockroach-secrets.sh` (COCKROACH_DSN / OPENROUTER_API_KEY / ENCRYPTION_MASTER_KEY) | **user** | #5, has OPENROUTER_API_KEY | ✅ **DONE** — 3 secrets `Staged` on bchat-crdb (deploy applies them) |
| 8 | `task deploy:cockroach` | me | #6, #7 | **IN PROGRESS** — chain stages; **closes §7 "Fly deploy works"** |
| 9 | Memos admin signup on `bchat-crdb.fly.dev` | me | #8 | Admin can sign in (smoke prerequisite) |
| 10 | `task verify:production` (`BCHAT_URL/BCHAT_USER/BCHAT_PASS`) | me | #9 | 7/7 steps PASS, test tenant destroyed |
| 11 | `task rollback:postgres` | me + **user** | #8, Neon `DATABASE_URL` + S3 secrets on bchat-crdb (see §4) | Redeploy on Neon profile; `verify:production` green; **closes §7 "Rollback demonstrated"** |
| 12 | Re-cutover to Cockroach (optional) | me | #11 | `COCKROACH_DSN` secret back, `fly_cockroach.toml` deploy |
| 13 | `crdb:harden` (egress IP, ~$3.60/mo) | me | **explicit user approval** | Allowlist = `<fly-egress-ip>/32` only, connectivity verified |

---

## 3. What Remains to Be Tested

| Test | How | Expected | Status |
|------|-----|----------|--------|
| §6.2 seven checks on Cloud | `task crdb:verify` (env-gated) | `SELECT 1` OK; `version()` = Cockroach; `migration_history` = 1 row; `nextval()` defaults; vector index enabled; `agent_vectors` indexed; `/healthz` 200 | PENDING — **tooling note:** §6.2 SQL checks need a host `cockroach` binary (task skips gracefully without it); fallback: run the same SQL via `ccloud cluster sql <cluster> -e "..."` |
| A1 migrations on Cloud | first boot of `bchat-crdb` | `migration_history` = 1 row; `FORCE_REINDEX_ON_STARTUP` initial reindex completes | PENDING |
| RAG round-trip with real embeddings | `verify:production` step 6 | ≥1 hit on `rag/search` (OpenRouter embeddings) | PENDING |
| Post-rollback Neon path | `rollback:postgres` + `verify:production` | Smoke green on Neon + S3/Tigris RAG env | PENDING |
| 2-region evidence | `SHOW REGIONS`, `SHOW CREATE TABLE agent_vectors` (C-SPANN index) | 2 rows; `VECTOR(1536)` + index | PENDING — demo material |
| P4/P5 (optional) | `bash scripts/crdb-deploy.sh --experiments` | not required (Q&A decision 4) | SKIPPED by design |
| Demo spine (§6.6) | `db:local` → `deploy:postgres` → `deploy:cockroach` → `rollback:postgres` | all four work from the same Taskfile | PENDING — rehearsal before demo |

---

## 4. Gotchas / Gaps Discovered

| Gap | Detail | Mitigation |
|-----|--------|------------|
| **ccloud 0.6.12 has no `database create` subcommand** | `ccloud cluster database create` prints help (unknown subcommand); DB created instead via `cockroach sql` `CREATE DATABASE` (defaulted to primary region aws-us-west-2) | Use installed client binary for SQL admin ops; note in runbook |
| **`ccloud cluster sql` installs its own client** | Requires sudo (`error installing cockroach-sql binary`); useless for automation | Use the host `cockroach` binary + DSN from `.env` |
| **ccloud `--connection-url` is SSO-token based** | Returns URL with no user/password; unusable outside `ccloud cluster sql` | Build DSN manually: `postgresql://bchat_user:PASS@host:26257/bchat?sslmode=verify-full` |
| **Fly "secrets not deployed" is expected** | `fly secrets list` shows `Staged` until next deploy | Normal; `task deploy:cockroach` applies them — no `fly secrets deploy` needed |
| **Rollback needs S3 secrets on `bchat-crdb`** | `rollback:postgres` only sets `DATABASE_URL` + unsets `COCKROACH_DSN`; the rollback profile (`fly_pg-rollback.toml`) uses `LANCEDB_STORAGE_PROVIDER=s3` — `LANCEDB_S3_BUCKET`/`LANCEDB_S3_PREFIX` + AWS creds live on `bchat-pg`, not `bchat-crdb` | Before #11: copy S3/AWS secrets from `bchat-pg` to `bchat-crdb` (or set them in the secrets step) |
| `verify:production` needs a pre-existing memos admin | First boot has no users; smoke signin will 401 | Step 9: signup admin on the deployed instance first |
| `FORCE_REINDEX_ON_STARTUP='true'` is boot-time only | Vector backfill blocks writes; not wanted on later restarts | Flip to `false` in `fly_cockroach.toml` after first successful boot (documented in runbook §4) |
| `crdb:verify` §6.2 SQL needs a host `cockroach` binary | Not installed; task degrades to skipping SQL checks | Install client binary or run SQL via `ccloud cluster sql` |
| silver-fish is single-region GCP | Cannot add regions to an existing single-region Basic cluster (verified constraint) | New console cluster required (item #1); silver-fish stays untouched |
| 26 pre-existing postgres `store/test` failures | Identical on master before/after this work | Separate backlog; not blockers |
| Pre-existing gofmt debt (store/agent.go, store/bridge.go, store/ticket.go, store/user.go, store/rbac.go, store/db/sqlite/rbac.go, 4 store/test files) | Unformatted on HEAD, untouched by this work | Separate backlog |
| Neon files must stay untouched | `fly_pg.toml`, `Dockerfile.pg.fly`, `scripts/fly-pg-secrets.sh` | Final `git diff` guard at close-out |
| Everything is still uncommitted | ~26 modified/untracked items in `git status` | Commit decision at close-out (§6) |

---

## 5. What to Expect Next (sequence + timings)

1. ✅ **Console cluster** — `great-goat` CREATED (3 regions; see plan_phase.md deviations).
2. ✅ **Automatable setup** — SQL user `bchat_user`, DB `bchat`, allowlist, DSN → `.env`, vector check, `setup.sh`. (~1 min)
3. ✅ **Fly prep** — `fly apps create bchat-crdb`; secrets staged via `fly-cockroach-secrets.sh`. (~2 min)
4. **Deploy** (me, ~5–10 min, **now**): `task deploy:cockroach` — image build (3–5 min) + first-boot migration (~1–2 min incl. initial reindex) + healthz poll (15s grace, up to 2 min) + `crdb:verify` + `verify:production`. **Expect:** full PASS; §7 "Fly deploy works" box closes.
5. **Rollback demo** (me + user, ~5 min): user provides Neon `DATABASE_URL`; S3 secrets copied from `bchat-pg`; `task rollback:postgres` → `verify:production`. **Expect:** PASS on Neon; §7 rollback box closes.
6. **Close-out** (me, ~10 min): flip `FORCE_REINDEX_ON_STARTUP` to `false`, update `pre_code.md` (§7 two boxes, §8 steps 10–11, §9 evidence) + `code.md` (status, §8) + runbook alignment, final git-diff isolation check, present commit decision.

**Failure expectations:** each chain stage fails loudly with exit ≠ 0 and a pointer to `build/crdb-deploy.log`; scripts are re-runnable — re-run after fixing the cause, never patch around the chain.

---

## 6. Close-out Checklist

- [ ] §7 "Fly deploy works" box checked (healthz 200 on `bchat-crdb.fly.dev`)
- [ ] §7 "Rollback demonstrated" box checked (`rollback:postgres` + post-rollback `verify:production` green)
- [ ] pre_code.md updated: §8 steps 10–11 marked done with live evidence; §9 evidence section extended (Cloud version/regions/vector-setting results)
- [ ] code.md updated: Status line complete; §8 open items resolved; §4 evidence additions
- [ ] `docs/docs_flyio_cockroach_deploy.md` aligned with observed values (region names, timings, any deviations)
- [ ] `FORCE_REINDEX_ON_STARTUP` flipped to `false` post-first-boot
- [ ] Neon isolation re-verified: `git diff` empty for `fly_pg.toml`, `Dockerfile.pg.fly`, `scripts/fly-pg-secrets.sh`
- [ ] Optional: `crdb:harden` executed (only with user approval)
- [ ] Optional: demo spine rehearsal (§6.6) + `SHOW REGIONS` capture
- [ ] Commit decision presented (everything uncommitted as of 2026-08-02)
