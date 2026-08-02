# Bug 057 — Live Deployment Phase Plan (plan_phase)

**Date:** 2026-08-02
**Purpose:** Execute the live-deployment phase of the CockroachDB profile (pending.md items 3–10): cluster user/DB/DSN setup, Fly app + secrets, deploy, smoke verification.
**Preconditions (done):** ccloud authenticated (Pithom Labs / org-32ndt); 2-region+ Basic cluster `great-goat` (AWS, v26.2.1, CREATED, regions us-east-1 / us-east-2 / **us-west-2 primary**, spend limit $17); `fly auth login` done (`gani.mendoza@gmail.com`); Fly app `bchat-crdb` does **not** exist yet (bchat / bchat0534 / bchat-pg are suspended).

---

## Phase Live-1 — Cluster provisioning (me, automatable, ~2 min)

| # | Step | Command / detail | Expected |
|---|------|------------------|----------|
| 1 | Install cockroach client binary | `curl https://binaries.cockroachdb.com/cockroach/cockroach-v26.2.1.linux-amd64.tgz \| tar -xz && install cockroach-v26.2.1.linux-amd64/cockroach ~/.local/bin/` | `cockroach version` works (enables native §6.2 checks in `crdb:verify`) |
| 2 | Create SQL user `bchat_user` | `ccloud cluster user create great-goat bchat_user -p <generated>` (openssl rand; non-interactive via `-p`) | "Success!" |
| 3 | Create database `bchat` | `ccloud cluster database create great-goat bchat` | "Successfully created database" |
| 4 | Verify allowlist | `ccloud cluster networking allowlist list great-goat` | `0.0.0.0/0 --sql --ui` present (Basic default; add if missing) |
| 5 | Build DSN | host from `ccloud cluster sql great-goat --connection-url` → `postgresql://bchat_user:PASS@<host>:26257/bchat?sslmode=verify-full` → write to `.env` (gitignored, line 93) | `.env` contains COCKROACH_DSN |
| 6 | Round-trip SQL | `cockroach sql --url "$COCKROACH_DSN" -e "SELECT version();"` | Cockroach version string |
| 7 | Vector-index check | `SHOW CLUSTER SETTING feature.vector_index.enabled` | `true` (v26.2.1 default; `SET` only if false) |
| 8 | Documented setup flow | `CLUSTER_NAME=great-goat deploy/ccloud/setup.sh` | prints export lines; user already exists → skip |

**Gate:** steps 2–7 must pass before Live-2.

## Phase Live-2 — Fly app + secrets (user + me, ~2 min)

| # | Step | Owner | Expected |
|---|------|-------|----------|
| 9 | `fly apps create bchat-crdb` | me | App created |
| 10 | `scripts/fly-cockroach-secrets.sh` (interactive: COCKROACH_DSN + OPENROUTER_API_KEY; ENCRYPTION_MASTER_KEY auto) | **user** | `fly secrets list` shows all three |

**Gate:** secrets set before Live-3 (deploy requires COCKROACH_DSN at boot).

## Phase Live-3 — Deploy + verify (me, ~10 min)

| # | Step | Expected |
|---|------|----------|
| 11 | `task deploy:cockroach` (chain: build → parity → compat → fly deploy → healthz poll 15s grace/2min → crdb:verify → verify:production) | All stages PASS; healthz 200; **closes §7 "Fly deploy works"** |
| 12 | First-boot overrun contingency | If migration+reindex exceeds poll window → re-run the chain (stateful-safe) |
| 13 | Admin signup on `https://bchat-crdb.fly.dev` | `POST /api/v1/auth/signup {username, password}` → admin exists (smoke prerequisite) |
| 14 | `task verify:production` (`BCHAT_URL/BCHAT_USER/BCHAT_PASS`) | 7/7 PASS; test tenant destroyed |
| 15 | `task crdb:verify` native §6.2 checks (host cockroach binary + COCKROACH_DSN) | SELECT 1; version()=Cockroach; migration_history=1; nextval defaults; vector setting true; agent_vectors indexed; healthz 200 |

**Close-out after Live-3 (per pending.md §6):** flip `FORCE_REINDEX_ON_STARTUP` → `false` in `fly_cockroach.toml` + redeploy; then rollback phase (user provides Neon `DATABASE_URL` + S3 secrets copied from `bchat-pg`) → `rollback:postgres` → `verify:production`; update pre_code.md / code.md / runbook; final git-diff isolation check; commit decision.

---

## Deviations / notes
- `great-goat` has **3 regions** (plan said 2) with **primary us-west-2** (plan said us-east-1) — acceptable; us-west-2 is closer to Fly `sjc`.
- ccloud 0.6.12 has no `-e` flag → native SQL via installed cockroach binary (step 1).
- `ccloud cluster user create` supports `-p` → non-interactive.
- `verify:production` bridge-handoff cycle: covered by store suites (bridge endpoints need per-tenant HMAC, not admin auth).
