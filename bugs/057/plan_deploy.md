# Bug 057 — Local-First Test & Redeploy Plan (plan_deploy)

**Date:** 2026-08-02
**Purpose:** Diagnose + fix the failed first deploy of `bchat-crdb` (healthz never 200; migration killed mid-flight by Fly autostop) using a local-first strategy: reproduce the full first-boot path locally (single-node, then 3-region), prove the fix locally, then redeploy.
**Status:** Deploy attempt 1 failed 02:19Z–02:25Z; diagnosis complete; fixes not yet applied.

---

## 1. Live Evidence (deploy attempt 1)

| Fact | Value |
|------|-------|
| Fly machine | `860312fe920408`, sjc, boot 02:19:27Z, autostop 02:25:03Z (~5.5 min), restart 02:25:16Z |
| App logs | OpenRouter key loaded → pre-migrate WARN (`relation "migration_history" does not exist`, expected A1) → **no further logs** → cron curl exit 7 (nothing listening) |
| Proxy | "waiting for machine to be reachable on 0.0.0.0:5230" → "gave up after 15 attempts" → 503 |
| DB state | `great-goat`: **42/57 tables** created, `migration_history` **empty** (written only at end, migrator.go:241) → migration mid-flight when killed |
| Rate | 39→42 tables in ~13 min (~18–40s/DDL) on serverless 3-region zone-survival |
| Cause chain | health-check `grace_period 15s` (fly_cockroach.toml:48) << migration time (~20+ min) → Fly marks unhealthy → `auto_stop_machines='stop'` kills machine → restart re-runs LATEST.sql idempotently → killed again |

## 2. Facts That Constrain the Fix

- `/healthz` is registered **after** migration completes (server.go:104-107) — no health check can pass until migration + workspace-basic-setting + reindex finish.
- Cockroach mirror `LATEST.sql`: 1030 lines, **57 CREATE TABLE, 83 CREATE INDEX, 7 CREATE UNIQUE INDEX, 0 ALTER TABLE, 0 VECTOR(**, 0 triggers/views/functions. (agent_vectors table is created by code — vectordb_cockroach.go — not by migration; confirmed missing from Cloud table list.)
- Migrator cockroach arm (migrator.go:205-224): `SET serial_normalization` + whole file as **one ExecContext**, no tx; tolerance strings `duplicate column` / `already exists` / `column already exists`; fully idempotent (IF NOT EXISTS) → re-runs resume cleanly (verified by TestCockroachMigrateEndToEnd A3/A4).
- `crdb-deploy.sh` stage 5 runs `fly deploy` (its own wait timed out at ~5.5 min); stage 6 polls healthz **24×5s ≈ 2 min** (crdb-deploy.sh:53-65) — both far below migration time.
- Fly deploy supports `--wait-timeout` (confirmed via `fly deploy --help`).
- Local compose is **single-node v25.2.21** — passes all local tests but cannot reproduce multi-region DDL latency. Docker Hub confirms `cockroachdb/cockroach:v26.2.1` (amd64) **exists** = Cloud's exact version.
- Fly machine is stateful-safe: 42 tables already exist on Cloud; a redeploy resumes from there.

## 3. Phase 1 — Local first-boot rehearsal (single-node, ~5 min)

1. Reset local `bchat` DB to A1 (drop all public tables CASCADE — same helper as `resetCockroachDB` in store/test/cockroach_migrate_test.go:48).
2. `task build:cockroach` (pure-Go binary, verified earlier).
3. Run locally with prod-like env: `MEMOS_DRIVER=cockroach`, `MEMOS_MODE=prod`, `RAG_PIPELINE_ENABLED=true`, `LANCEDB_STORAGE_PROVIDER=cockroach`, `EMBEDDING_PROVIDER=mock` (no OpenRouter cost locally), `FORCE_REINDEX_ON_STARTUP=true`, `COCKROACH_DSN=postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable`.
4. Assert: migration completes → `migration_history` = 1 row with FS version → all 57 tables → workspace setting written → port binds → `/healthz` 200 → reindex runs.
5. Restart → boot idempotency (no re-migration; history still 1 row). **Time each boot** (baseline).
6. Gate: Phase 1 must fully pass before Phase 2.

## 4. Phase 2 — Multi-region repro (3-node v26.2.1, ~10 min)

1. Create throwaway compose file in `/tmp/opencode/` (no repo changes): 3 nodes `cockroachdb/cockroach:v26.2.1` with `--locality region=aws-us-east-1 / aws-us-east-2 / aws-us-west-2`, join cluster, zone-survival replication zone mirroring Cloud.
2. Reset DB → run the same first-boot flow → time the migration → **is it slow like Cloud, or fast?**
3. Statement-by-statement timing: split `LATEST.sql` on `;`, run each via `cockroach sql --url ... -e` with timing → identify slow DDL (suspects: CREATE INDEX on tables, multi-region leases).
4. Compare **one-shot ExecContext vs per-statement autocommit** → does chunking help?
5. Gate: either (a) migration fast enough on 3-node → slowness is serverless-Basic-specific → config fix only; or (b) one-shot exec is the bottleneck → migrator chunking needed (Q2).

## 5. Phase 3 — Fix deployment config (local-proven first)

1. `fly_cockroach.toml:48`: `grace_period = "15s"` → `"30m"`.
2. `crdb-deploy.sh:51`: `fly deploy --wait-timeout 25m`.
3. `crdb-deploy.sh:53-65`: healthz poll 24×5s (~2m) → 240×5s (~20m), message updated.
4. If Phase 2(b): migrator cockroach arm (migrator.go:212) → per-statement autocommit loop with tolerance strings preserved + store/test updates + re-run P0/e2e/crdb:test locally.
5. Re-run `task deploy:cockroach` (resumes Cloud migration idempotently at 42/57).

## 6. Phase 4 — Verify + close §7 box

- healthz 200 on `bchat-crdb.fly.dev` → §7 "Fly deploy works" checked in pre_code.md.
- `task crdb:verify` native §6.2 checks (host cockroach v26.2.0 + COCKROACH_DSN from `.env`): SELECT 1; version()=Cockroach; migration_history=1; nextval defaults; vector setting true; agent_vectors indexed; healthz 200.
- Admin signup → `task verify:production` (7/7, test tenant destroyed).

## 7. Phase 5 — Close-out (pending.md §6)

- Flip `FORCE_REINDEX_ON_STARTUP=false` + redeploy.
- Rollback phase next (Neon `DATABASE_URL` + S3 secrets from bchat-pg) — separate plan.
- Update pending.md / code.md / runbook with observed timings + grace-period change; git-diff isolation guard; commit decision.

## 8. Failure Expectations & Guardrails

- Every Phase 1/2 gate fails loudly; no Cloud redeploy until Phases 1–2 pass locally.
- Never patch around the chain; scripts re-runnable.
- Do not touch Neon files (`fly_pg.toml`, `Dockerfile.pg.fly`, `fly-pg-secrets.sh`).
- Leave the Fly machine as-is during Phases 1–2 (harmless; stateful-safe).
- Do not change the migrator unless Phase 2(b) proves it.

## 9. Open Questions

1. **Phase 2 (3-node repro) — do it or skip?** Recommended: do it. It's the only way to know whether the Cloud slowness is serverless-specific (config fix suffices) or a one-shot-exec problem (migrator fix needed) — and it validates v26.2.1 locally before burning another Fly deploy cycle.
2. **If Phase 2(b) proves one-shot Exec is the bottleneck — OK to chunk the migrator's cockroach arm** (per-statement autocommit loop, tolerance strings preserved, tests updated)?
3. **Local embeddings:** OK to use `EMBEDDING_PROVIDER=mock` in Phases 1–2 (avoids OpenRouter cost/rate-limits during repro)? Cloud keeps `openrouter`.

## 10. Adversarial Plan Review Prompt

> You are an adversarial reviewer for the plan above (local-first diagnosis + fix + redeploy of a CockroachDB-backed Fly app whose first boot migration is too slow for its health-check window). Attack the plan, don't compliment it. Answer ONLY what's wrong, missing, or risky.
>
> **Critical (C):**
> - Is the root-cause attribution ("migration slower than grace period → autostop kills it") actually proven by the evidence, or could the migration be hung (not merely slow) in a way Phase 1 single-node can't reveal? What specific observation would distinguish "slow" from "stuck"?
> - The fix is "make the wait longer". What if the migration on serverless Basic simply never completes in any reasonable window (e.g., 30m)? What's the upper-bound evidence path, and what is the fallback?
> - If Phase 2(b) requires migrator changes, the idempotency story (IF NOT EXISTS) is what makes mid-flight re-runs safe. Does per-statement chunking preserve that exact property, or could a partial chunk leave a state that the tolerance strings don't cover (e.g., duplicate index, constraint, sequence)?
> - `fly deploy --wait-timeout 25m` — what does fly actually do when the wait expires mid-migration? Does it abort the deploy (leaving the machine running) or stop the machine? Verify from CLI behavior before relying on it.
>
> **High (H):**
> - What else could make `/healthz` fail even with a 30m grace: reindex time with real OpenRouter embeddings at EMBEDDING_BATCH_SIZE=10, vector-index creation (C-SPANN) on agent_vectors, or workspace SecretKey bootstrap (server.go:94-102)? Have we budgeted for them?
> - Autostop is `auto_stop_machines='stop'` with `min_machines_running=0` — does grace_period actually prevent autostop, or does Fly stop unhealthy machines regardless? Check the documented interaction.
> - Phase 2 uses a throwaway compose file with 3 regions on one machine — is `--locality` + zone-survival faithful enough to Cloud (network latency to Cloud is real; local loopback is ~0ms)? What can this repro actually validate, and what can't it?
> - During Phase 1/2, the Cloud machine may be killed and restarted repeatedly, each time re-running LATEST.sql from 42 tables. Any cost/spend implication on the $17 limit, or schema-change queue contention when we finally redeploy?
>
> **Medium (M):**
> - Password for `bchat_user` sits in `/tmp/opencode/bchat_pw` and `.env` (gitignored). Cleanup steps after close-out?
> - v25.2.21 (compose) vs v26.2.1 (Cloud): does Phase 1 on v25.2.21 add any false confidence? Should Phase 1 itself use the v26.2.1 image?
> - `fly_cockroach.toml` comment at line 53 says "grace 15s per fly_pg.toml" — copy-paste debt; fix while here?
> - The 2m poll message ("grace 15s per fly_pg.toml") in crdb-deploy.sh:54 — same stale comment.
> - What happens to the cron `trigger-cron` job (exit 7 loop every 5 min) during a 20-30 min migration — log noise or retry pressure?
>
> **Non-blocking (N):**
> - Docs (runbook, pending.md) updated with the observed ~20-40s/DDL figure and the grace-period rationale?
> - Should the 3-node compose be kept in-repo for future regression testing, or is /tmp/opencode throwaway correct?
> - After rollback demo, does `bchat-crdb` keep running or get destroyed (cost)?
>
> Produce a numbered list of findings with severity, and for each Critical/High finding state the exact additional step (observation or test) to close it before the redeploy.
