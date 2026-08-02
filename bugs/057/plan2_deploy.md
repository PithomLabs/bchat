# Bug 057 — Local-First Test & Redeploy Plan, Rev 2 (plan2_deploy)

**Date:** 2026-08-02 | **Supersedes:** plan_deploy.md (rev 1) after adversarial review (plan_deploy_review.md)

## 1. Disposition of Review Findings

| Finding | Verdict | Where addressed |
|---------|---------|-----------------|
| C1 root cause is inference (slow vs stuck) | **Adopted + partially answered already** — new Cloud evidence below proves "progressing, not hung" | §2, Phase 4 |
| C2 timeouts may mask wrong problem; need upper bound | **Adopted** — explicit 60-min threshold + fallback | Phase 3 |
| C3 prove one-shot is the bottleneck (richer metrics) | **Adopted** — wall clock + per-statement + CPU + jobs + retries + connections | Phase 2 |
| H1 local 3-node ≠ Cloud | **Adopted** — Phase 2 reframed as *functional* validation | Phase 2 |
| H2 separate migration/workspace/reindex/listen timings | **Adopted** | Phase 1 |
| H3 healthz placement (readiness vs startup) | **Documented, not changed** | §8 |
| H4 verify Fly behavior experimentally | **Adopted** — cheap experiment, pending Q4 | Phase 3 |
| M1 Phase 1 should use v26.2.1 | **Adopted** — compose bump pending Q5 | Phase 1 |
| M2 per-phase timestamps | **Adopted** — shell-side timeline capture; optional slog instrumentation pending Q6 | Phase 1 |
| M3 archive first-failed-deploy artifacts | **Adopted** — `bugs/057/artifacts/` | Phase 0 |
| M4 `SHOW JOBS` in Phase 2 | **Adopted + proven feasible on Cloud** (bchat_user can run it) | Phase 2 |
| N1–N3 | **Accepted** (no change) | — |
| Phase 0 startup-timeline instrumentation | **Adopted** as evidence baseline phase | Phase 0 |

## 2. New Evidence (answers C1 before the fix)

Collected read-only from `great-goat` on 2026-08-02 ~02:35Z:

| Observation | Value | Implication |
|-------------|-------|-------------|
| Table count | 42 (was 39 at 02:27, 42 at 02:38) | **Progressing** — not hung |
| `SHOW JOBS` | ~15 SCHEMA CHANGE jobs created 02:27–02:31, each ~10–14s apart; 8 still `running (waiting for MVCC GC)`, others `succeeded` in ~5 min | Each DDL creates a job; ~10–24s client-side per statement; GC drains in background |
| Statement math | 57 tables + 83 indexes + 7 unique indexes ≈ **147 DDL statements** × ~10–24s ≈ **25–60 min** continuous | Migration needs one long-lived boot, not many short ones |
| Fly machine | **stopped** at 02:31:27Z (3rd autostop) | Killed again mid-flight; ~6-min lifetime per boot |

**Root cause (now evidence-backed):** per-statement serverless DDL latency × 147 statements ≈ 25–60 min needed vs machine killed every ~6 min by `grace_period 15s` + `auto_stop_machines='stop'` + `min_machines_running=0`. Idempotent re-runs (IF NOT EXISTS) mean each boot only pays for remaining statements — so one uninterrupted window suffices.

## 3. Phase 0 — Evidence Baseline & Artifacts (adopted from review)

1. Create `bugs/057/artifacts/`; archive:
   - Attempt-1 Fly logs (`fly logs --no-tail` output, already captured) — machine timeline 02:19:27→02:31:27
   - `build/crdb-deploy.log` (already exists)
   - Cloud `SHOW JOBS` snapshot (from §2) + table-count history
2. Record baseline numbers in `artifacts/evidence.md`: 42/57 tables, job creation cadence, kill timestamps.
3. Gate: artifacts present before any further changes.

## 4. Phase 1 — Single-Node Rehearsal on v26.2.1 (matches Cloud; M1)

1. **Compose version bump** (pending Q5): `scripts/docker-compose.cockroach.yml` v25.2.21 → **v26.2.1**; recreate `bchat-crdb` container (data volume intact; DB reset in step 2 anyway).
2. Reset `bchat` DB to A1 (drop all public tables CASCADE, same helper as `resetCockroachDB`).
3. `task build:cockroach`; run locally: `MEMOS_DRIVER=cockroach MEMOS_MODE=prod RAG_PIPELINE_ENABLED=true LANCEDB_STORAGE_PROVIDER=cockroach EMBEDDING_PROVIDER=mock FORCE_REINDEX_ON_STARTUP=true COCKROACH_DSN=postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable`.
4. **Per-phase timeline (M2/H2)** via shell-side capture:
   - `T0` process start → `T1` migration start (first DDL visible) → `T2` migration complete (`migration_history` = 1 row) → `T3` workspace init → `T4` reindex start/end → `T5` HTTP listen → `T6` `/healthz` 200
   - Record each delta; identify dominant phase.
5. Assert: 57 tables, history = 1 row with FS version, nextval defaults intact.
6. Restart → boot idempotency (history still 1 row; no re-migration). Record re-boot time.
7. Gate: full pass → Phase 2.

## 5. Phase 2 — 3-Node Functional Validation (H1 reframe, C3 metrics)

**Purpose:** validate the migration SQL is correct and *complete* on a v26.2.1 multi-region topology, and test whether execution mode matters. **Not** a Cloud performance reproduction (loopback has no serverless scheduling tax).

1. Throwaway compose in `/tmp/opencode/` (no repo changes): 3× `cockroachdb/cockroach:v26.2.1`, localities `aws-us-east-1/2`, `aws-us-west-2`, join cluster, zone-survival mirroring Cloud.
2. Reset DB → same first-boot flow (mock embeddings) → record Phase-1 timeline.
3. **Experiment A — execution mode (C3):** run `LATEST.sql` via (a) one-shot ExecContext, (b) per-statement autocommit. Measure: total wall clock, per-statement latency distribution, `SHOW JOBS` samples (M4), docker `stats` CPU, connection count, retries observed in logs.
4. **Expected outcome:** both fast locally (validation only); if one-shot is materially slower, evidence supports migrator chunking (then Q2's approval applies); otherwise **config-only default**.
5. Gate: SQL correctness + idempotent re-run proven on v26.2.1 → Phase 3.

## 6. Phase 3 — Fly Config Fix (with upper bound, C2)

1. `fly_cockroach.toml:48`: `grace_period = "15s"` → **`"60m"`** (covers 25–60 min migration + reindex).
2. `crdb-deploy.sh:51`: `fly deploy --wait-timeout 45m`.
3. `crdb-deploy.sh:53-65`: healthz poll → **600×5s (~50m)**; fix stale "grace 15s per fly_pg.toml" comment.
4. **H4 experiment (pending Q4):** cheap Fly behavior check — create a throwaway app/machine with a sleeping entrypoint, run `fly deploy --wait-timeout`, observe: does wait expiry abort-and-stop the machine, or leave it running with checks continuing? (Alternatively rely on attempt-1 observation: deploy-wait expiry → deploy failed → machine autostopped — which is exactly why `--wait-timeout` must exceed migration time.)
5. **Upper bound (C2):** if the Cloud migration has not completed within **60 min of continuous uptime** during Phase 4, stop config tuning and redesign migration execution (per-statement chunking with preserved tolerance strings; or pre-boot one-shot migrator job). Threshold documented in runbook.
6. Gate: config + experiment results recorded → Phase 4.

## 7. Phase 4 — Cloud Redeploy with Live Evidence (C1 closed loop)

1. `task deploy:cockroach` (resumes at 42/57 — stateful-safe).
2. **During first boot:** sample every 60s — table count, `SHOW JOBS` (proven feasible, §2), Fly machine state, app logs. Record timeline: migration start→complete→workspace→reindex→listen→healthz 200.
3. Continue even if stage 5 logs a deploy timeout — poll independently (phase 3 step 3).
4. Healthz 200 → **§7 "Fly deploy works" checked** in pre_code.md.
5. `task crdb:verify` native §6.2 checks (host cockroach v26.2.0 + `COCKROACH_DSN` from `.env`); admin signup; `task verify:production` (7/7, test tenant destroyed).
6. Gate: all pass → Phase 5.

## 8. Phase 5 — Close-Out

- Flip `FORCE_REINDEX_ON_STARTUP=false` + redeploy (post-first-boot).
- Archive Phase-4 evidence in `bugs/057/artifacts/` (M3).
- Update pending.md / code.md / runbook with observed timings, the ~10–24s/DDL figure, grace-period rationale.
- **H3 documented decision:** `/healthz` reflects startup completion (incl. migration), not merely liveness — accepted for Bug 057; noted as future work (server.go:104).
- Rollback phase next (Neon `DATABASE_URL` + S3 secrets from bchat-pg) — separate plan.
- Cleanup: `/tmp/opencode/bchat_pw`, `.env` DSN handling; git-diff isolation guard (Neon files untouched); commit decision.

## 9. Guardrails

- No Cloud redeploy until Phases 0–2 pass. No migrator change unless Phase 2 proves it (Q2).
- Never patch around the chain; scripts re-runnable; every gate fails loudly.
- Do not touch `fly_pg.toml` / `Dockerfile.pg.fly` / `fly-pg-secrets.sh`.
- Leave the Fly machine stopped as-is during Phases 0–2 (harmless; auto-start on demand).
- Mock embeddings locally; Cloud keeps `openrouter` (Q3 — defaulted yes).

## 10. Open Questions

1. **Compose bump to v26.2.1** (`scripts/docker-compose.cockroach.yml`) — OK to change the shared local-dev compose to match Cloud? (Recommended: yes.)
2. **Migrator chunking approval** — only if Phase 2 proves one-shot is materially slower; default is config-only. Pre-approve or decide after evidence?
3. **Mock embeddings locally** — confirm (default yes).
4. **H4 Fly experiment** — run the cheap throwaway-app test, or rely on attempt-1 observed behavior? (Recommended: cheap test, ~2 min.)
5. **Optional slog instrumentation** (migration start/complete lines in migrator.go cockroach arm) to make Phase 1/4 timelines visible in logs — include or keep shell-side capture only? (Recommended: shell-side only, zero code change.)

## 11. Rev-2 Adversarial Review Prompt (delta scope)

> Review only the deltas in plan2_deploy vs plan_deploy: (1) the C1 evidence in §2, (2) the 60-min upper bound + fallback in Phase 3, (3) Phase 0 artifacts, (4) Phase 2 reframe as functional validation, (5) the 60m/45m/50m wait parameters, (6) Q1–Q5. Do not re-review unchanged content.
>
> **C:** Could the §2 evidence still misattribute? (e.g., GC job pile-up from repeated boots — does the `waiting for MVCC GC` backlog accumulate across boots and eventually stall new jobs? Check MVCC GC TTL vs boot cadence.) Is 60m truly sufficient given the observed ~10–24s/statement, or should the upper bound use a jobs-per-minute measurement instead of wall clock?
> **H:** With 147 statements per boot re-issued (IF NOT EXISTS), does each re-issued statement still spawn a job or cost a schema lease — i.e., is re-run cost really "only remaining statements"? Any Cloud spend/queue risk from 6+ boots (attempts so far: 3)?
> **M:** `fly deploy --wait-timeout 45m` — verify flag semantics (wait for health checks vs destroy-on-expiry) before relying; does the compose bump orphan the existing `bchat_crdb_data` volume (schema created by v25.2.21)?
> **N:** artifacts dir naming/structure; evidence.md format.
> Produce numbered findings with severity and the exact closing step for each C/H item.
