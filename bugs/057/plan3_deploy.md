# Bug 057 — Local-First Test & Redeploy Plan, Rev 3 (plan3_deploy)

**Date:** 2026-08-02 | **Supersedes:** plan2_deploy.md (rev 2) after adversarial review (plan2_deploy_review.md). Prior revisions remain unmodified (plan_deploy.md = rev 1, plan2_deploy.md = rev 2).

## 1. Disposition of Rev-2 Review Findings

| Finding | Verdict | Where addressed |
|---------|---------|-----------------|
| C1: 25–60 min estimate rests on linear extrapolation | **Adopted** — adaptive rate-based prediction | Phase 4 |
| C2: schema-job backlog may accumulate across boots | **Closed with data** — see §2 (254/254 jobs succeeded; queue converged) | §2, Phase 4 |
| H1: Phase 4 should observe migration progress continuously | **Adopted** — dual-metric sampling (tables + indexes + succeeded jobs) | Phase 4 |
| H2: upper bound should be adaptive | **Adopted** — predicted-completion stop rule, not blind 60-min wait | Phase 3 |
| H3: capture server-side retries | **Adopted, scope-limited** — `crdb_internal` is restricted on Cloud serverless (verified); retry stats only in local Phase 2; Cloud proxy = failed-job count (verified 0) | Phase 2, Phase 4 |
| H4: document expected first-boot startup latency | **Adopted** | Phase 5 |
| M1: version alignment (v26.2.1) | **Adopted** — compose bump (pending Q1) | Phase 1 |
| M2: absolute timestamps alongside deltas | **Adopted** | Phase 4 |
| M3: artifact subdirectories | **Adopted** — `{attempt1, attempt2, phase1, phase2, phase4}/` | Phase 0 |
| M4: shell-side only, no slog instrumentation | **Accepted** — zero production code changes | §9 |
| N1–N3 | **Accepted** (no change) | — |
| New: Phase 3.5 dry-run Fly deployment | **Adopted** — best finding; also verifies Fly→Cloud connectivity (never verified before) | Phase 3.5 |

## 2. New Evidence (collected read-only 2026-08-02 ~02:40Z)

| Observation | Value | Implication |
|-------------|-------|-------------|
| `[SHOW JOBS]` status totals | **254 total: 254 succeeded, 0 running, 0 failed, 0 pending** | C2 closed: repeated interrupted boots do NOT accumulate backlog; queue fully drained; zero statement errors ever |
| Job breakdown | 132 SCHEMA CHANGE + 59 NEW SCHEMA CHANGE + 59 SCHEMA CHANGE GC + 3 TYPEDESC (all succeeded) | DDL jobs complete; GC drains in background; no retry-induced failures |
| `crdb_internal` access | **Restricted** on Cloud serverless (SQLSTATE 42501) | Retry stats (`num_runs`) unavailable on Cloud; local Phase 2 only |
| Table count | 42/57 | Progressing; migration_history still empty (written only at end — progress must be inferred from schema objects) |
| Fly machine | stopped at 02:31:27Z (3rd autostop) | ~6-min lifetime per boot vs ~25–60 min needed |

**Root cause (evidence-backed):** per-statement serverless DDL latency (~10–24s) × ~147 statements ≈ 25–60 min continuous, vs machine killed every ~6 min by `grace_period 15s` + `auto_stop_machines='stop'` + `min_machines_running=0`. Idempotent re-runs (IF NOT EXISTS) resume remaining work; one uninterrupted window should suffice.

## 3. Phase 0 — Evidence Baseline & Artifacts

1. Create `bugs/057/artifacts/` with subdirectories: `attempt1/`, `attempt2/`, `phase1/`, `phase2/`, `phase4/` (M3).
2. Archive into `attempt1/`:
   - Fly logs (`fly logs --no-tail`, captured 2026-08-02) — machine timeline 02:19:27→02:31:27
   - `build/crdb-deploy.log`
   - Cloud `SHOW JOBS` snapshots (from §2) + table-count history
   - This review's evidence: 254/254 jobs, 42/57 tables, kill timestamps
3. Write `attempt1/evidence.md`: baseline numbers + note "progress inferred from schema objects, not migration_history (written only at end of migration)".
4. Gate: artifacts present before any further changes.

## 4. Phase 1 — Single-Node Rehearsal on v26.2.1 (matches Cloud; M1)

1. **Compose version bump** (pending Q1): `scripts/docker-compose.cockroach.yml` v25.2.21 → **v26.2.1**; recreate `bchat-crdb` container (data volume intact; DB reset in step 2 anyway).
2. Reset `bchat` DB to A1 (drop all public tables CASCADE, same helper as `resetCockroachDB`).
3. `task build:cockroach`; run locally: `MEMOS_DRIVER=cockroach MEMOS_MODE=prod RAG_PIPELINE_ENABLED=true LANCEDB_STORAGE_PROVIDER=cockroach EMBEDDING_PROVIDER=mock FORCE_REINDEX_ON_STARTUP=true COCKROACH_DSN=postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable`.
4. **Per-phase timeline** via shell-side capture with absolute timestamps: `T0` process start → `T1` migration start → `T2` migration complete (`migration_history` = 1 row) → `T3` workspace init → `T4` reindex start/end → `T5` HTTP listen → `T6` `/healthz` 200. Record deltas AND `date -u +%FT%TZ` for correlation with Fly logs (M2).
5. Assert: 57 tables, history = 1 row with FS version, nextval defaults intact.
6. Restart → boot idempotency (history still 1 row; no re-migration). Record re-boot time.
7. Save all outputs to `artifacts/phase1/`. Gate: full pass → Phase 2.

## 5. Phase 2 — 3-Node Functional Validation (H1 reframe, C3 metrics)

**Purpose:** validate migration SQL correctness/completeness on v26.2.1 multi-region topology; test execution mode. **Not** a Cloud performance reproduction (loopback has no serverless scheduling tax).

1. Throwaway compose in `/tmp/opencode/` (no repo changes): 3× `cockroachdb/cockroach:v26.2.1`, localities `aws-us-east-1/2`, `aws-us-west-2`, join cluster, zone-survival mirroring Cloud.
2. Reset DB → same first-boot flow (mock embeddings) → record Phase-1 timeline.
3. **Experiment A — execution mode (C3/H3):** run `LATEST.sql` via (a) one-shot ExecContext, (b) per-statement autocommit. Measure: total wall clock, per-statement latency distribution, `SHOW JOBS` samples, docker `stats` CPU, connection count, and **retry stats** via `crdb_internal.jobs num_runs > 1` (available locally; M4/H3).
4. **Expected outcome:** both fast locally (validation only); if one-shot is materially slower, evidence supports migrator chunking (Q2); otherwise **config-only default**.
5. Save outputs to `artifacts/phase2/`. Gate: SQL correctness + idempotent re-run proven → Phase 3.

## 6. Phase 3 — Fly Config Fix (with adaptive upper bound, C2/H2)

1. `fly_cockroach.toml:48`: `grace_period = "15s"` → **`"60m"`**.
2. `crdb-deploy.sh:51`: `fly deploy --wait-timeout 45m`.
3. `crdb-deploy.sh:53-65`: healthz poll → **600×5s (~50m)**; fix stale "grace 15s per fly_pg.toml" comment.
4. **Adaptive stop rule (H2):** during Phase 4, compute ETA = remaining-work / observed-rate every sample; if ETA > 60 min while progress is near-zero for 3 consecutive samples, stop and switch to fallback (migrator redesign). Documented in runbook.
5. **Upper bound (C2):** if Cloud migration has not completed within 60 min of continuous uptime with positive progress, stop config tuning and redesign migration execution (per-statement chunking with preserved tolerance strings; or pre-boot one-shot migrator job).
6. Gate: config recorded → Phase 3.5.

## 7. Phase 3.5 — Dry-Run Fly Deployment (isolates Fly mechanics from migration)

1. Throwaway app `bchat-crdb-dryrun` (Fly), assets in `/tmp/opencode/` (no repo changes):
   - Dockerfile: `FROM cockroachdb/cockroach:v26.2.1` (ships the `cockroach` binary)
   - Entrypoint: `cockroach sql --url "$COCKROACH_DSN" -e "SELECT 1" && sleep 1200`
   - Toml: same `[http_service]` grace 60m + `auto_stop`/`min_machines_running` as fly_cockroach.toml, `internal_port` unused
2. Set COCKROACH_DSN secret; deploy with `--wait-timeout 45m`.
3. Observe: (a) **Fly→Cloud Cockroach connectivity** (SELECT 1 from Fly — never verified), (b) machine stays alive beyond the old ~6-min lifetime, (c) health-check behavior during long no-listen window, (d) `--wait-timeout` semantics on expiry (machine left running vs stopped), (e) autostop interaction with long grace.
4. Record results in `artifacts/attempt2/` (this is attempt 2's evidence). Destroy the dry-run app.
5. Gate: connectivity + machine-lifetime + wait semantics confirmed → Phase 4.

## 8. Phase 4 — Cloud Redeploy with Live Evidence (C1/H1 closed loop)

1. `task deploy:cockroach` (resumes at 42/57 — stateful-safe).
2. **Dual-metric sampler (H1/C1)** every 60s, absolute timestamps: table count (information_schema), index count, `succeeded SCHEMA CHANGE` job count (`[SHOW JOBS]`), Fly machine state, app logs tail. Compute per-sample rates: tables/min, indexes/min, jobs/min → ETA.
3. Continue even if stage 5 logs a deploy timeout — poll independently (Phase 3 step 3).
4. Apply adaptive stop rule (Phase 3 step 4) if progress stalls.
5. Healthz 200 → **§7 "Fly deploy works" checked** in pre_code.md.
6. `task crdb:verify` native §6.2 checks (host cockroach v26.2.0 + `COCKROACH_DSN` from `.env`); failed-job re-check (H3 proxy, expect 0); admin signup; `task verify:production` (7/7, test tenant destroyed).
7. Save all sampler output to `artifacts/phase4/`. Gate: all pass → Phase 5.

## 9. Phase 5 — Close-Out

- Flip `FORCE_REINDEX_ON_STARTUP=false` + redeploy (post-first-boot).
- Archive Phase-4 evidence (M3); update pending.md / code.md / runbook.
- **H4 documented:** "first boot performs full migration + reindex; expect 30–60 min; `/healthz` returns 503 until complete; this is expected, not a failure."
- **H3 documented decision:** `/healthz` reflects startup completion (incl. migration), not mere liveness — accepted for Bug 057; future work (server.go:104).
- Rollback phase next (Neon `DATABASE_URL` + S3 secrets from bchat-pg) — separate plan.
- Cleanup: `/tmp/opencode/bchat_pw`, `.env` DSN handling; git-diff isolation guard (Neon files untouched); commit decision.

## 10. Guardrails

- No Cloud redeploy until Phases 0–2 pass. No migrator change unless Phase 2 proves it (Q2).
- Never patch around the chain; scripts re-runnable; every gate fails loudly.
- Do not touch `fly_pg.toml` / `Dockerfile.pg.fly` / `fly-pg-secrets.sh`.
- Leave the Fly machine stopped as-is during Phases 0–3.5 (harmless; auto-start on demand).
- Mock embeddings locally; Cloud keeps `openrouter` (Q3 — defaulted yes).
- Dry-run app `bchat-crdb-dryrun` is throwaway: created in Phase 3.5, destroyed same phase, never holds state.

## 11. Open Questions

1. **Compose bump to v26.2.1** (`scripts/docker-compose.cockroach.yml`) — OK? (Recommended: yes.)
2. **Migrator chunking approval** — only if Phase 2 proves one-shot is materially slower; default config-only. Pre-approve or decide after evidence?
3. **Mock embeddings locally** — confirm (default yes).
4. **Phase 3.5 dry-run app** — OK to create + destroy throwaway Fly app `bchat-crdb-dryrun` (~2–3 min, minimal cost)? (Recommended: yes.)
5. **Shell-only instrumentation** (no slog, zero production code changes) — confirm.

## 12. Adversarial Plan Review Prompt (Rev 3)

> You are an adversarial reviewer for Rev 3 of this deployment plan (superseding revs 1–2 reviewed previously). Focus ONLY on what changed or what remains unproven: the C2 closure claim, the adaptive rate-based ETA/stop rule, the dual-metric sampler, the Phase 3.5 dry-run, the scope-limited retry metrics, and the artifact structure. Do not re-review unchanged content.
>
> **Critical (C):**
> - C2 closure: 254/254 jobs succeeded while only 42/57 tables exist. Is the mapping "jobs ≠ statements" fully understood? (e.g., do re-issued IF NOT EXISTS statements spawn no-op jobs that inflate the succeeded count?) If the queue is only converged because the machine is stopped, what happens to the job queue when the machine starts re-running again? Propose the exact observation that distinguishes "converged" from "dormant".
> - The ETA/stop rule assumes progress is measurable and monotonic. What if per-statement latency is bimodal (fast early, slow late — e.g., later CREATE INDEX on larger tables)? Would the adaptive rule falsely stop or falsely continue? Define the exact stop decision boundary.
> - Phase 3.5 dry-run uses the cockroach CLI image, not the bchat binary. Does that fully validate connectivity? (pgx TLS behavior, verify-full cert chain from the Fly image, DSN parsing). What residual risk remains that the dry-run cannot detect?
>
> **High (H):**
> - Phase 4 poll during a 50-min window with 60s cadence = ~50 samples; who observes if the sampler itself dies mid-deploy? Should the sampler run detached (nohup/log file)?
> - `--wait-timeout 45m` vs grace 60m vs poll 50m: three different numbers. Which one is the binding constraint if migration takes exactly 55 min? Propose consistent ordering.
> - Reindex: Phase 4 closes at healthz 200, but FORCE_REINDEX_ON_STARTUP reindex runs after listen. Is "Fly deploy works" honestly closable before reindex completes? Where does reindex sit relative to healthz 200 in the timeline?
>
> **Medium (M):**
> - Artifact subdir naming `attempt1` vs `attempt2` vs `phase4`: is attempt2 (dry-run) correctly the "attempt 2" artifact, or should naming be phase-keyed only?
> - Compose bump: does the existing `bchat_crdb_data` volume persist schema created by v25.2.21 (downgrade risk on revert)?
> - If Phase 1/2 reveal the migration is fine and only Cloud is slow, do we still run Phase 3.5 (yes/no — cost/benefit)?
>
> **Non-blocking (N):**
> - evidence.md format; docs update ownership; commit decision timing.
>
> Produce numbered findings with severity; for each C/H finding state the exact closing observation or test before the Phase 4 redeploy.
