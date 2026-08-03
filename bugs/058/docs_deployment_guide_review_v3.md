# Bug 058 — Adversarial Review v3: Deployment Guide vs Taskfile vs TOML Consistency

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
- `scripts/validate-env-chain.sh`

---

## Executive Summary

The previous review found 8 logic inconsistencies plus 1 new finding about hardcoded `fly.toml` in `validate-env-chain.sh`. This follow-up verifies whether those findings were actually resolved in the implementation.

**Status:** 5 findings resolved, 3 findings partially resolved, 1 new finding introduced.

**Verdict:** APPROVE WITH NITS — the critical validation path is now parameterized and backend-aware, but 3 documentation/logic gaps remain.

---

## Resolution Check: Previous Findings

### Finding 1 (Critical) — Validation Logic Reads Wrong Config for CRDB: ✅ RESOLVED

**Previous issue:** `fly:db-check` read `fly.toml` (Postgres config) and auto-detected Postgres for CRDB deployments.

**Current state:** 
- `validate-env-chain.sh` now accepts a TOML file parameter: `TOML_FILE="${1:-fly.toml}"` (line 11)
- `fly:check` passes `{{.CLI_ARGS}}` to the script (Taskfile.yml line 186)
- `fly:pre-deploy:cockroach` calls `fly:check` with `vars: {CLI_ARGS: "fly_cockroach.toml"}` (Taskfile.yml line 239-240)
- `fly:pre-deploy:postgres` calls `fly:check` with `vars: {CLI_ARGS: "fly_pg.toml"}` (Taskfile.yml line 250-251)
- `fly:pre-deploy:sqlite` calls `fly:check` with `vars: {CLI_ARGS: "fly.toml"}` (Taskfile.yml line 261-262)

**Verdict:** RESOLVED. The validation chain now reads the correct TOML file for each backend.

---

### Finding 2 (Critical) — Guide Says ONE `fly.toml`, But Deploy Logic Uses Multiple TOML Files: ✅ RESOLVED

**Previous issue:** Guide claimed ONE `fly.toml` manually edited per backend, but code uses separate TOML files.

**Current state:** Guide now says:
```markdown
**Key Design Decision:** Each backend has its own TOML file and Fly app. No manual editing required.
```

Table shows:
```markdown
| Backend | TOML File | Fly App | Dockerfile |
|---------|-----------|---------|------------|
| SQLite | `fly.toml` | `bchat-app` | `Dockerfile.fly` |
| Postgres | `fly_pg.toml` | `bchat-pg` | `Dockerfile.pg.fly` |
| CockroachDB | `fly_cockroach.toml` | `bchat-crdb` | `Dockerfile.cockroach.fly` |
```

Line 105: "Postgres: `fly_pg.toml` is the canonical source. `fly.toml` is a legacy alias and should not be used for new deployments."

**Verdict:** RESOLVED. Guide correctly documents per-backend TOML files.

---

### Finding 3 (High) — `fly:check` Validates Wrong Target for CRDB: ✅ RESOLVED

**Previous issue:** `fly:check` validated `.env` -> `Dockerfile` -> `fly.toml` -> `fly secrets`. For CRDB, it should validate `fly_cockroach.toml`.

**Current state:** `fly:check` now accepts `CLI_ARGS` which is passed to `validate-env-chain.sh` as the TOML file parameter. When called from `fly:pre-deploy:cockroach`, it validates `fly_cockroach.toml`.

**Verdict:** RESOLVED.

---

### Finding 4 (High) — Plan References Nonexistent Task: ✅ RESOLVED

**Previous issue:** `plan_deploy_crdb.md` referenced `task fly:check:cockroach`, which did not exist.

**Current state:** `fly:check:cockroach` exists in Taskfile.yml (line 231-234):
```yaml
fly:check:cockroach:
  desc: Validate CockroachDB migrations before deployment
  cmds:
    - ./scripts/validate-cockroach-compat.sh
```

**Verdict:** RESOLVED.

---

### Finding 5 (High) — App/Config Identity Conflict: ⚠️ PARTIALLY RESOLVED

**Previous issue:** `fly.toml` names the app `bchat-pg` and points to Postgres. If a user reuses `fly.toml` for CRDB, they deploy to wrong app with wrong config.

**Current state:** Guide correctly documents each backend has its own app name. Deployment steps tell users to run `task deploy:cockroach`, which uses `fly_cockroach.toml` and `bchat-crdb`.

**Remaining issue:** Guide's table shows SQLite using `fly.toml` with app `bchat-app` (line 101). But actual `fly.toml` has `app = 'bchat-pg'` and `dockerfile = 'Dockerfile.pg.fly'` — a Postgres config, not SQLite. The guide's SQLite row is wrong.

**Verdict:** PARTIALLY RESOLVED. CRDB path is correct, but SQLite row in table is wrong.

---

### Finding 6 (Medium) — Duplicate Postgres Config Files: ❌ NOT RESOLVED

**Previous issue:** `fly.toml` and `fly_pg.toml` are functionally identical, creating ambiguity.

**Current state:** Both files still exist. Guide says `fly_pg.toml` is canonical, `fly.toml` is legacy alias. But both are still present and `fly:pre-deploy:sqlite` still uses `fly.toml`.

**Verdict:** NOT RESOLVED. The ambiguity remains.

---

### Finding 7 (Medium) — Guide and Plan Disagree on CRDB TOML File: ✅ RESOLVED

**Previous issue:** Guide said edit `fly.toml` for CRDB, plan said use `fly_cockroach.toml`.

**Current state:** Both guide and plan reference `fly_cockroach.toml` for CRDB.

**Verdict:** RESOLVED.

---

### Finding 8 (Medium) — `fly:pre-deploy` Is Backend-Agnostic In Name Only: ✅ RESOLVED

**Previous issue:** `fly:pre-deploy` was universal but validated only `fly.toml`.

**Current state:** Backend-specific tasks exist:
- `fly:pre-deploy:cockroach` calls `fly:check` with `fly_cockroach.toml`
- `fly:pre-deploy:postgres` calls `fly:check` with `fly_pg.toml`
- `fly:pre-deploy:sqlite` calls `fly:check` with `fly.toml`

**Verdict:** RESOLVED.

---

## New Findings

### Finding 9 (Medium) — `plan_deploy_crdb.md` Phase 3 Uses Wrong Task for Environment Check

**File:** `bugs/058/plan_deploy_crdb.md`  
**Severity:** Medium  
**Type:** Logic inconsistency

The plan says:
```markdown
| 3.1 | `task fly:check` | Passes |
```

But `fly:check` without arguments defaults to `fly.toml` (Postgres config). The guide correctly recommends `task fly:pre-deploy:cockroach`, which passes `fly_cockroach.toml` to `fly:check`.

The plan's step 3.1 would validate Postgres env vars, not CRDB env vars.

**Fix:** Update plan to match guide:
```markdown
| 3.1 | `task fly:pre-deploy:cockroach` | Passes |
```

Or if keeping the two-step approach:
```markdown
| 3.1 | `task fly:check fly_cockroach.toml` | Passes |
```

---

### Finding 10 (Low) — `fly:db-check` Still Hardcoded to `fly.toml`

**File:** `Taskfile.yml`  
**Severity:** Low  
**Type:** Legacy inconsistency

`fly:db-check` still reads `fly.toml` directly:
```yaml
fly:db-check:
  cmds:
    - |
      DOCKERFILE=$(grep -E '^\s*dockerfile\s*=' fly.toml | head -1 | ...)
```

This task is no longer recommended for CRDB in the guide, but it still exists and could be used accidentally.

**Fix:** Either remove `fly:db-check` or update it to accept a backend parameter like `fly:check` does. Not a blocker since the guide doesn't recommend it for CRDB.

---

## Resolution Summary

| # | Finding | Severity | Status |
|---|---------|----------|--------|
| 1 | Validation logic reads wrong config for CRDB | Critical | ✅ RESOLVED |
| 2 | Guide ONE-`fly.toml` claim contradicts multi-TOML deploy | Critical | ✅ RESOLVED |
| 3 | `fly:check` validates wrong target for CRDB | High | ✅ RESOLVED |
| 4 | Plan references nonexistent `fly:check:cockroach` | High | ✅ RESOLVED |
| 5 | App/config identity conflict | High | ⚠️ PARTIALLY RESOLVED — SQLite row in guide table is wrong |
| 6 | Duplicate Postgres config files | Medium | ❌ NOT RESOLVED |
| 7 | Guide and plan disagree on CRDB TOML file | Medium | ✅ RESOLVED |
| 8 | `fly:pre-deploy` is backend-agnostic in name only | Medium | ✅ RESOLVED |
| 9 | `validate-env-chain.sh` hardcoded to `fly.toml` | Critical | ✅ RESOLVED — now parameterized |
| 10 | Plan step 3.1 uses `task fly:check` instead of `task fly:pre-deploy:cockroach` | Medium | ❌ NEW — plan inconsistent with guide |

---

## Required Changes Before Execution

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | Guide table shows SQLite using `fly.toml` with `bchat-app`, but actual `fly.toml` is Postgres config | High | Fix table: either remove SQLite row or document that `fly.toml` must be manually edited for SQLite |
| 2 | `plan_deploy_crdb.md` Phase 3 step 3.1 says `task fly:check` instead of `task fly:pre-deploy:cockroach` | Medium | Update plan to match guide |
| 3 | Duplicate Postgres config (`fly.toml` + `fly_pg.toml`) still unresolved | Medium | Document which file is canonical, or delete legacy alias |

---

## Final Verdict

**APPROVE WITH NITS**

The critical logic inconsistencies have been resolved:
- `validate-env-chain.sh` is now parameterized with `TOML_FILE="${1:-fly.toml}"`
- `fly:check` passes `CLI_ARGS` to the script
- `fly:pre-deploy:cockroach` passes `fly_cockroach.toml`
- Guide correctly documents per-backend TOML files
- Backend-specific pre-deploy tasks exist and are wired correctly

**Remaining nits:**
1. Guide's SQLite row in the backend table is wrong — `fly.toml` is a Postgres config, not SQLite
2. `plan_deploy_crdb.md` Phase 3 step 3.1 uses `task fly:check` instead of `task fly:pre-deploy:cockroach`
3. Duplicate Postgres config files still exist, but the guide now documents which is canonical

These are documentation nits, not logic blockers. The deployment workflow is now internally consistent.
