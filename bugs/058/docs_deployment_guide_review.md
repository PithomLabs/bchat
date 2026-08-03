# Bug 058 — Adversarial Review: Deployment Guide vs Taskfile vs TOML Consistency

**Date:** 2026-08-03  
**Reviewer:** Senior Go / CockroachDB Architect  
**Artifacts under review:**
- `docs_deployment_guide.md`
- `fly.toml`
- `fly_cockroach.toml`
- `fly_pg.toml`
- `Taskfile.yml`
- `scripts/crdb-deploy.sh`
- `bugs/058/plan_deploy_crdb.md`

---

## Executive Summary

The deployment guide, Taskfile, and TOML files have **critical inconsistencies** in their logical structure. The guide says there is ONE `fly.toml` manually edited per backend, but the code actually uses separate TOML files per backend, and the validation tasks read the wrong file for CockroachDB. These are logic gaps, not just documentation drift.

**Verdict:** REQUEST CHANGES — 2 Critical, 3 High, 2 Medium.

---

## Finding 1 (Critical) — Validation Logic Reads Wrong Config for CRDB

**Files:** `Taskfile.yml`, `docs_deployment_guide.md`, `scripts/crdb-deploy.sh`  
**Severity:** Critical  
**Type:** Logic inconsistency

The guide says `task fly:db-check` auto-detects the backend from `fly.toml`. But `fly.toml` points to `Dockerfile.pg.fly`, so for a CockroachDB deployment the validation logic runs the **Postgres** migration check instead of the CockroachDB compatibility check. The actual deploy command uses `fly_cockroach.toml`, so the pre-deployment validation and the deployment are checking different backends. That is a direct logical contradiction in the workflow.

---

## Finding 2 (Critical) — Guide Says ONE `fly.toml`, But Deploy Logic Uses Multiple TOML Files

**Files:** `docs_deployment_guide.md`, `fly.toml`, `fly_cockroach.toml`, `fly_pg.toml`, `Taskfile.yml`  
**Severity:** Critical  
**Type:** Logic inconsistency

The guide’s stated design is ONE `fly.toml` manually edited per backend. The actual deploy logic uses distinct TOML files: `fly_cockroach.toml` for CockroachDB, `fly_pg.toml` for Postgres, and `fly.toml` for Postgres as well. These cannot both be true. If the guide’s rule is correct, then `deploy:cockroach` and `deploy:postgres` are misconfigured. If the code is correct, then the guide’s central design claim is false. Either way, the documented logic and the execution logic diverge on the first step of deployment.

---

## Finding 3 (High) — `fly:check` Validates the Wrong Target for CRDB

**Files:** `Taskfile.yml`, `docs_deployment_guide.md`  
**Severity:** High  
**Type:** Logic inconsistency

`fly:check` validates `.env` -> `Dockerfile` -> `fly.toml` -> `fly secrets`. For CockroachDB, the deployment target is `fly_cockroach.toml`, not `fly.toml`. So the validation chain is logically incomplete: it checks a file that is not the deployment source of truth for CRDB. The guide recommends `task fly:pre-deploy` for all backends, but that chain cannot logically validate CockroachDB if it never inspects `fly_cockroach.toml`.

---

## Finding 4 (High) — Plan References Validation Task That Does Not Exist

**File:** `bugs/058/plan_deploy_crdb.md`  
**Severity:** High  
**Type:** Logic inconsistency

`plan_deploy_crdb.md` Phase 3 says to run `task fly:check` and `task fly:check:cockroach`. But `Taskfile.yml` does not define `fly:check:cockroach`. It defines `fly:check`, `fly:db-check`, and `crdb:check`. So the plan’s validation sequence is logically broken: one of the required tasks does not exist, and the remaining task validates the wrong backend.

---

## Finding 5 (High) — App/Config Identity Conflict Between `fly.toml` and CRDB Config

**Files:** `fly.toml`, `fly_cockroach.toml`, `scripts/crdb-deploy.sh`  
**Severity:** High  
**Type:** Logic inconsistency

`fly.toml` names the app `bchat-pg` and points to Postgres. `fly_cockroach.toml` names the app `bchat-crdb` and points to CockroachDB. `crdb-deploy.sh` hardcodes `APP="bchat-crdb"` and deploys with `fly_cockroach.toml`. So if a user follows the guide and reuses `fly.toml` for CockroachDB, the logical chain is: same app name, wrong Dockerfile, wrong driver env, wrong storage provider. The guide’s “edit `fly.toml`” step does not have a coherent path to a valid CockroachDB deployment.

---

## Finding 6 (Medium) — Duplicate Source of Truth for Postgres Config

**Files:** `fly.toml`, `fly_pg.toml`  
**Severity:** Medium  
**Type:** Logic inconsistency

`fly.toml` and `fly_pg.toml` are functionally identical. The guide’s ONE-file logic implies they should be the same file, but the code keeps two identical copies. That is not a bug by itself, but it creates logical ambiguity: which file is the source of truth? If they diverge, which validation or deploy path wins?

---

## Finding 7 (Medium) — Guide and Plan disagree on Which TOML File CRDB Uses

**Files:** `docs_deployment_guide.md`, `bugs/058/plan_deploy_crdb.md`  
**Severity:** Medium  
**Type:** Logic inconsistency

The guide’s CockroachDB section says to edit `fly.toml`. `plan_deploy_crdb.md` says CockroachDB uses `fly_cockroach.toml`. These are mutually exclusive instructions for the same deployment. The guide’s step and the plan’s step cannot both be correct.

---

## Finding 8 (Medium) — `fly:pre-deploy` Is Logically Backend-Agnostic But Backend-Specific In Practice

**Files:** `Taskfile.yml`, `docs_deployment_guide.md`  
**Severity:** Medium  
**Type:** Logic inconsistency

`fly:pre-deploy` is documented as the universal pre-deployment check. It calls `fly:check` and `fly:db-check`. Both read `fly.toml`. So `fly:pre-deploy` is logically Postgres-specific while being presented as backend-agnostic. The guide tells users to run it for CockroachDB, but the task chain does not logically cover CockroachDB.

---

## Approved As-Is

### `crdb-deploy.sh`
The deploy chain is logically consistent: it uses `fly_cockroach.toml`, builds with the `cockroach` tag, runs CRDB-specific validation, deploys, polls healthz, and runs CRDB verification. The inconsistency is in the surrounding docs and validation tasks, not in this chain.

### `deploy:postgres`
Logically consistent: uses `fly_pg.toml` and `-a bchat-pg`.

### `rollback:postgres`
Logically consistent: flips `bchat-crdb` back to Postgres by changing secrets and redeploying with `fly_pg-rollback.toml`.

### `crdb:check`
Logically consistent: validates CockroachDB env and runs the CRDB compatibility scanner.

---

## Required Changes Before Execution

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | `fly:db-check` reads wrong config for CRDB | Critical | Make backend explicit in validation logic |
| 2 | Guide ONE-`fly.toml` claim contradicts multi-TOML deploy | Critical | Align guide with actual multi-TOML structure |
| 3 | `fly:check` validates wrong target for CRDB | High | Add CRDB-specific validation path |
| 4 | Plan references nonexistent `fly:check:cockroach` | High | Update plan to match actual Taskfile targets |
| 5 | App/config identity conflict between `fly.toml` and CRDB config | High | Clarify which TOML/app pairing is authoritative per backend |
| 6 | Duplicate Postgres config files | Medium | Consolidate or document single source of truth |
| 7 | Guide and plan disagree on CRDB TOML file | Medium | Standardize on `fly_cockroach.toml` everywhere |
| 8 | `fly:pre-deploy` is backend-agnostic in name only | Medium | Add backend-specific pre-deploy tasks or make validation backend-aware |

---

## Final Verdict

**REQUEST CHANGES**

The deployment logic itself is coherent, but the surrounding documentation and validation logic contain direct contradictions. The core issue is not bad code; it is a guide and validation layer that describe a different workflow than the one the Taskfile and TOML files actually execute. Resolve the logical inconsistencies above before using this as an authoritative deployment runbook.

---

## Recommendations

1. Make the backend explicit everywhere.
   - Add a `BACKEND` variable to `fly:check`, `fly:db-check`, and `fly:pre-deploy` so the same task can validate SQLite, Postgres, or CockroachDB without guessing from `fly.toml`.
   - Example: `task fly:db-check cockroach` should run CockroachDB checks regardless of what `fly.toml` says.

2. Treat `fly_cockroach.toml` and `fly_pg.toml` as the source of truth.
   - Rewrite `docs_deployment_guide.md` to say: each backend has its own TOML file and Fly app.
   - Remove the ONE-`fly.toml` narrative entirely. It does not match how `deploy:cockroach`, `deploy:postgres`, or `rollback:postgres` actually work.

3. Fix the validation/deploy pairing.
   - `deploy:cockroach` must pair with CockroachDB validation.
   - `deploy:postgres` must pair with Postgres validation.
   - Do not let a CRDB deploy depend on a Postgres-validated `fly:db-check` result.

4. Resolve the duplicate Postgres config.
   - Pick one canonical Postgres TOML file.
   - If both `fly.toml` and `fly_pg.toml` must exist for now, document which one `fly:check` reads and why.

5. Update `bugs/058/plan_deploy_crdb.md` to reference only existing Taskfile targets.
   - Replace `fly:check:cockroach` with the actual CRDB validation task (`crdb:check` or a new parameterized `fly:db-check` target).

6. Add backend-specific pre-deploy tasks.
   - `fly:pre-deploy:cockroach`
   - `fly:pre-deploy:postgres`
   - `fly:pre-deploy:sqlite`
   This removes ambiguity about which validation chain to run for which backend.

7. Keep the deploy chains as-is.
   - `crdb-deploy.sh`, `deploy:postgres`, `rollback:postgres`, and `crdb:check` are internally consistent. The problem is the docs and generic validation tasks, not these backend-specific paths.
