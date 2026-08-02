# Bug 057 — Local-First Test & Redeploy Plan, Rev 4 (plan4_deploy)

**Date:** 2026-08-02 | **Supersedes:** plan3_deploy.md (rev 3) after adversarial review (plan3_deploy_review.md). Prior revisions remain unmodified (plan_deploy.md = rev 1, plan2_deploy.md = rev 2, plan3_deploy.md = rev 3).

## 1. Disposition of Rev-3 Review Findings

| Finding | Verdict | Where addressed |
|---------|---------|-----------------|
| C1: "254/254 jobs succeeded" proves empty-while-stopped, not post-restart convergence | **Adopted** — claim reworded; post-restart convergence observation added | §2, Phase 4 |
| C2: Phase 3.5 dry-run proves networking, not pgx init path | **Adopted** — dry-run runs a pgx probe using the app's real driver init (`build/dryrun/`, gitignored), CLI check kept as secondary | Phase 3.5 |
| H1: three timeout values drift | **Adopted** — documented ordering: `wait_timeout (informational) < poll (authoritative) < grace (machine bound)`; crdb-deploy.sh stage 5 becomes non-fatal | Phase 3 |
| H2: adaptive ETA unstable on non-linear work | **Adopted** — ETA from median rate over last 5 samples; stop rule uses smoothed ETA | Phase 4 |
| H3: sampler resilience | **Adopted** — sampler runs detached (nohup), timestamped log | Phase 4 |
| H4: healthz before/after reindex explicit | **Adopted + config bug found** — healthz 200 before reindex; `RAG_STARTUP_REINDEX_DISABLED=true` short-circuits `FORCE_REINDEX_ON_STARTUP=true` (service.go:213 vs 224), so no startup reindex ever runs | §2, Phase 5 |
| M1: artifact naming (dryrun ≠ attempt2) | **Adopted** — `deploy-attempt1/`, `dryrun/`, `phase1/`, `phase2/`, `phase4/` | Phase 0 |
| M2: record every ETA estimate | **Adopted** — ETA history appended to sampler log + completion report | Phase 4 |
| M3: local retry metrics compromise | **Accepted** — as-is (local `crdb_internal.jobs num_runs`; Cloud failed-job proxy) | Phase 2 |
| N1–N3 | **Accepted** (no change) | — |
| New: Phase 4 completion report | **Adopted** — auto-generated from sampler log; hackathon-submission artifact | Phase 4 |

## 2. Evidence & Facts (collected read-only 2026-08-02)

| Observation | Value | Implication |
|-------------|-------|-------------|
| `[SHOW JOBS]` status totals | **254 total: 254 succeeded, 0 running, 0 failed, 0 pending** | Queue **empty at stop** — NOT proof of post-restart convergence (C1); convergence re-checked in Phase 4 |
| Job breakdown | 132 SCHEMA CHANGE + 59 NEW SCHEMA CHANGE + 59 SCHEMA CHANGE GC + 3 TYPEDESC (all succeeded) | DDL jobs complete; GC drains in background; zero statement errors ever |
| `crdb_internal` access | **Restricted** on Cloud serverless (SQLSTATE 42501) | Retry stats local-only; Cloud proxy = failed-job count |
| Table count | 42/57 | Progressing; migration_history empty (written only at end) |
| Fly machine | stopped at 02:31:27Z (3rd autostop) | ~6-min lifetime per boot vs ~25–60 min needed |
| Startup reindex | **Never runs** — `RAG_STARTUP_REINDEX_DISABLED=true` checked first (service.go:213) short-circuits `FORCE_REINDEX_ON_STARTUP=true` (service.go:224) | healthz 200 = migration+workspace+listen; reindex is async (service.go:225) and currently disabled; dead FORCE var is close-out cleanup |
| `build/` gitignored | `.gitignore:2` | In-module pgx probe (`build/dryrun/`) without repo pollution |
| pgx | v5.10.0 in go.mod | Dry-run probe uses same driver as app |

**Root cause (evidence-backed):** per-statement serverless DDL latency (~10–24s) × ~147 statements ≈ 25–60 min continuous, vs machine killed every ~6 min by `grace_period 15s` + `auto_stop_machines='stop'` + `min_machines_running=0`. Idempotent re-runs (IF NOT EXISTS) resume remaining work; one uninterrupted window should suffice.

## 3. Phase 0 — Evidence Baseline & Artifacts

1. Create `bugs/057/artifacts/` with subdirectories: `deploy-attempt1/`, `dryrun/`, `phase1/`, `phase2/`, `phase4/` (M1).
2. Archive into `deploy-attempt1/`:
   - Fly logs (machine timeline 02:19:27→02:31:27) + `build/crdb-deploy.log`
   - Cloud `SHOW JOBS` snapshots + table-count history (254/254 jobs, 42/57 tables, kill timestamps)
3. Write `deploy-attempt1/evidence.md`: baseline numbers; note "progress inferred from schema objects, not migration_history (written only at end)".
4. Gate: artifacts present before any further changes.

## 4. Phase 1 — Single-Node Rehearsal on v26.2.1 (matches Cloud; M1)

1. Compose bump: `scripts/docker-compose.cockroach.yml` v25.2.21 → **v26.2.1**; recreate `bchat-crdb` container.
2. Reset `bchat` DB to A1 (drop all public tables CASCADE — same helper as `resetCockroachDB`, store/test/cockroach_migrate_test.go:48).
3. `task build:cockroach`; run locally: `MEMOS_DRIVER=cockroach MEMOS_MODE=prod RAG_PIPELINE_ENABLED=true LANCEDB_STORAGE_PROVIDER=cockroach EMBEDDING_PROVIDER=mock FORCE_REINDEX_ON_STARTUP=true COCKROACH_DSN=postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable`.
4. Per-phase timeline with absolute timestamps: `T0` process start → `T1` migration start → `T2` migration complete (`migration_history` = 1 row) → `T3` workspace init → `T4` reindex start/end → `T5` HTTP listen → `T6` `/healthz` 200. Record deltas + `date -u +%FT%TZ`.
5. Assert: 57 tables, history = 1 row with FS version, nextval defaults intact.
6. Restart → boot idempotency (history still 1 row). Record re-boot time.
7. Save outputs to `artifacts/phase1/`. Gate: full pass → Phase 2.

## 5. Phase 2 — 3-Node Functional Validation (v26.2.1)

**Purpose:** validate migration SQL correctness/completeness on multi-region topology; test execution mode. **Not** a Cloud performance reproduction.

1. Throwaway compose in `/tmp/opencode/`: 3× `cockroachdb/cockroach:v26.2.1`, localities `aws-us-east-1/2`, `aws-us-west-2`, join cluster, zone-survival mirroring Cloud.
2. Reset DB → same first-boot flow (mock embeddings) → record timeline.
3. **Experiment A — execution mode:** run `LATEST.sql` via (a) one-shot ExecContext, (b) per-statement autocommit. Measure: total wall clock, per-statement latency distribution, `SHOW JOBS` samples, docker `stats` CPU, connection count, retry stats via `crdb_internal.jobs num_runs > 1` (local).
4. Expected outcome: both fast locally (validation only); if one-shot materially slower → migrator chunking (Q2); otherwise config-only.
5. Save outputs to `artifacts/phase2/`. Gate: correctness + idempotent re-run proven → Phase 3.

## 6. Phase 3 — Fly Config Fix (timeout relationship documented; H1)

1. `fly_cockroach.toml:48`: `grace_period = "15s"` → **`"60m"`**.
2. `crdb-deploy.sh:51`: `fly deploy --wait-timeout 45m`; **stage 5 non-fatal** — a wait-timeout expiry mid-migration is expected; only stage 6 poll decides success.
3. `crdb-deploy.sh:53-65`: healthz poll → **600×5s (~50m)**; fix stale comments.
4. **Timeout ordering (documented in script header + runbook):** `fly --wait-timeout 45m (informational) < poll 50m (authoritative) < grace 60m (machine-side bound)`.
5. **Adaptive stop rule (H2):** ETA = remaining / median(rate over last 5 samples); if smoothed ETA > 60 min with near-zero progress for 3 consecutive samples → stop, switch to fallback (migrator redesign).
6. Gate: config recorded → Phase 3.5.

## 7. Phase 3.5 — Dry-Run Fly Deployment (pgx probe; C2)

1. **Pgx probe** at `build/dryrun/main.go` (gitignored, in-module): `db.NewDBDriver(&profile.Profile{Driver:"cockroach", DSN: os.Getenv("COCKROACH_DSN"), Mode:"prod", Port:5231, Data:t.TempDir...})` → `SELECT 1` → `sleep 1200`-equivalent loop. Exercises the app's exact driver init (DSN parsing, TLS verify-full, pool, QueryExecMode).
2. Dry-run app `bchat-crdb-dryrun` (Fly), assets in `/tmp/opencode/` (no repo changes beyond gitignored build/dryrun):
   - Dockerfile: multi-stage golang build of `build/dryrun` → minimal runtime; entrypoint runs probe
   - Toml: same `[http_service]` grace 60m + `auto_stop`/`min_machines_running` as fly_cockroach.toml
3. Set COCKROACH_DSN secret; deploy with `--wait-timeout 45m`.
4. Observe: (a) pgx SELECT 1 from Fly (app init path), (b) machine lifetime beyond ~6 min, (c) health-check behavior during no-listen window, (d) `--wait-timeout` expiry semantics, (e) autostop interaction with long grace. Secondary: cockroach CLI SELECT 1 (network layer).
5. Record results in `artifacts/dryrun/`. Destroy the app; remove `build/dryrun`.
6. Gate: connectivity (pgx) + lifetime + wait semantics confirmed → Phase 4.

## 8. Phase 4 — Cloud Redeploy with Live Evidence (C1/H1/H2/H3 closed loop)

1. `task deploy:cockroach` (resumes at 42/57 — stateful-safe).
2. **Detached sampler (H3)** — `nohup` script, every 60s, absolute timestamps, log to `artifacts/phase4/sampler.log`: table count, index count, succeeded SCHEMA CHANGE jobs, Fly machine state, app logs tail; append ETA each sample (M2).
3. **Convergence observation (C1):** job count at T+0 (machine start), T+2min, T+5min — must stabilize, not grow.
4. Continue even if stage 5 reports deploy timeout — stage 6 poll is authoritative.
5. Adaptive stop (Phase 3 step 5) if smoothed progress stalls.
6. Healthz 200 → **§7 "Fly deploy works" checked** in pre_code.md.
7. `task crdb:verify` native §6.2 checks; failed-job re-check (H3 proxy, expect 0); admin signup; `task verify:production` (7/7, test tenant destroyed).
8. **Completion report (M2/New):** auto-generate `artifacts/phase4/completion-report.md` from sampler log: migration/workspace/reindex/health milestones, tables, indexes, jobs, ETA history, duration, outcome.
9. Gate: all pass → Phase 5.

## 9. Phase 5 — Close-Out

- Flip `FORCE_REINDEX_ON_STARTUP=false` + redeploy (post-first-boot). Decide dead-FORCE-var cleanup (it's short-circuited by RAG_STARTUP_REINDEX_DISABLED=true regardless).
- Archive Phase-4 evidence; update pending.md / code.md / runbook.
- **H4 documented:** first boot = migration + workspace + listen; reindex async and currently disabled; healthz 200 precedes reindex; startup latency 30–60 min is expected, not failure.
- `/healthz` readiness-vs-liveness trade-off documented (server.go:104; future work).
- Rollback phase next (Neon `DATABASE_URL` + S3 secrets from bchat-pg) — separate plan.
- Cleanup: `/tmp/opencode/bchat_pw`, `.env` DSN handling; git-diff isolation guard (Neon files untouched); commit decision.

## 10. Guardrails

- No Cloud redeploy until Phases 0–2 pass. No migrator change unless Phase 2 proves it (Q2).
- Never patch around the chain; every gate fails loudly.
- Do not touch `fly_pg.toml` / `Dockerfile.pg.fly` / `fly-pg-secrets.sh`.
- Leave the Fly machine stopped as-is during Phases 0–3.5 (harmless; auto-start on demand).
- Mock embeddings locally; Cloud keeps `openrouter` (Q3 — defaulted yes).
- Dry-run app is throwaway: created in Phase 3.5, destroyed same phase.

## 11. Open Questions

1. Compose bump to v26.2.1 (`scripts/docker-compose.cockroach.yml`) — [yes]
2. Migrator chunking — decide after Phase 2 evidence; default config-only
3. Mock embeddings locally — [yes]
4. Dry-run app `bchat-crdb-dryrun` + throwaway `build/dryrun` pgx probe — [yes]
5. Stage-5 non-fatal chain change — [yes]
6. Write plan4_deploy.md now — [yes] (this file)

## 12. Adversarial Plan Review Prompt (Rev 4)

> Review Rev 4 of this deployment plan (superseding revs 1–3 reviewed previously). Focus ONLY on the deltas: the C1 reword + convergence observation, the pgx-probe dry-run (build/dryrun), the timeout-ordering + stage-5 non-fatal change, the smoothed ETA/stop rule, the detached sampler, the H4 reindex-disabled finding + FORCE-var cleanup, artifact rename, ETA persistence, and the completion report. Do not re-review unchanged content.
>
> **Critical (C):**
> - The pgx probe in `build/dryrun` calls `db.NewDBDriver` with driver=cockroach. Does that path actually replicate the app's migration-time connection behavior (simple_protocol? auto-append of query exec mode? same TLS config as the migrator)? If the migrator uses a different connection setup than NewDBDriver, the probe misses it. Verify by reading store/db/postgres driver init.
> - Stage 5 non-fatal: if `fly deploy` fails for a REAL reason (build error, config error) rather than wait-timeout, stage 5 would now be non-fatal and stage 6 would poll a non-deploying machine for 50 min. How does the plan distinguish "deploy wait expired" from "deploy genuinely failed"? Propose the exact discriminator.
> - H4 finding: if no startup reindex ever runs, then FORCE_REINDEX_ON_STARTUP=true is dead — but verify:production step 6 (RAG search) requires an indexed vector store. Does verify:production trigger a manual reindex itself, or does the chain rely on the (never-running) startup reindex? Trace the smoke path.
>
> **High (H):**
> - Convergence observation at T+0/T+2m/T+5m: who samples it if the machine autostarts between sampler polls? Is job-count sampling from the host cockroach CLI safe during heavy Cloud load (rate limits on the $17 plan)?
> - ETA smoothing: median over 5 samples at 60s cadence = 5-min lag. For a 25–60 min migration that's fine — but the stop rule "3 consecutive samples of near-zero progress with ETA>60m" — is 3 samples enough given the 5-min lag? Propose exact stop parameters.
> - `build/dryrun` inside the module: does `go build ./build/dryrun` collide with anything (build tags, module paths)? Does the repo's `go vet ./...` or CI pick it up? Verify it's ignored by build tasks.
>
> **Medium (M):**
> - Compose bump: existing `bchat_crdb_data` volume persists v25.2.21 data — schema compatible with v26.2.1 on recreate? Downgrade risk on revert?
> - Dry-run Dockerfile builds from `build/dryrun` — but `fly deploy` builds on Fly's builder; is the build context including `build/dryrun`? (Build context is the repo root per fly_cockroach.toml; /tmp/opencode assets must not break context.)
> - Completion report: who generates it — the sampler or a separate script? Owner + timing.
>
> **Non-blocking (N):**
> - evidence.md format; docs ownership; commit decision timing.
>
> Produce numbered findings with severity; for each C/H finding state the exact closing observation or test before the Phase 4 redeploy.
