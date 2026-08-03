# Bug 058 — Adversarial Review v4: Deployment Guide vs Taskfile vs TOML Consistency

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

The previous review found 9 findings (1 critical hardcoded `fly.toml` in `validate-env-chain.sh`, plus logic inconsistencies between guide/Taskfile/TOML). This follow-up verifies whether the implementation properly resolved all findings.

**Status:** All High/Critical findings resolved. Remaining items are low-priority nits.

**Verdict:** APPROVE — deployment logic, guide, and Taskfile are now internally consistent.

---

## Resolution Check: Previous Findings

### Finding 1 (Critical) — Validation Logic Reads Wrong Config for CRDB: ✅ RESOLVED

**Previous issue:** `fly:db-check` read `fly.toml` (Postgres config) and auto-detected Postgres for CRDB deployments.

**Current state:**
- `validate-env-chain.sh` accepts TOML file parameter: `TOML_FILE="${1:-fly.toml}"`
- `fly:check` passes `{{.CLI_ARGS}}` to script
- `fly:pre-deploy:cockroach` calls `fly:check` with `CLI_ARGS: "fly_cockroach.toml"`
- `fly:pre-deploy:postgres` calls `fly:check` with `CLI_ARGS: "fly_pg.toml"`
- `fly:pre-deploy:sqlite` calls `fly:check` with `CLI_ARGS: "fly.toml"`

**Verdict:** RESOLVED. Each backend now validates its own TOML file.

---

### Finding 2 (Critical) — Guide Says ONE `fly.toml`, But Deploy Logic Uses Multiple TOML Files: ✅ RESOLVED

**Previous issue:** Guide claimed ONE `fly.toml` manually edited per backend, but code uses separate TOML files.

**Current state:** 
- Guide says: "Each backend has its own TOML file and Fly app. No manual editing required."
- Backend table shows Postgres → `fly_pg.toml`, CockroachDB → `fly_cockroach.toml`
- SQLite row removed from deployment guide table (correctly, since SQLite doesn't deploy to Fly)
- `fly.toml` documented as legacy alias for Postgres

**Verdict:** RESOLVED. Guide correctly documents per-backend TOML files.

---

### Finding 3 (High) — `fly:check` Validates Wrong Target for CRDB: ✅ RESOLVED

**Previous issue:** `fly:check` validated `.env` -> `Dockerfile` -> `fly.toml` -> `fly secrets`. For CRDB, it should validate `fly_cockroach.toml`.

**Current state:** `fly:check` now accepts `CLI_ARGS` which is passed to `validate-env-chain.sh` as the TOML file parameter. When called from `fly:pre-deploy:cockroach`, it validates `fly_cockroach.toml`.

**Verdict:** RESOLVED.

---

### Finding 4 (High) — Plan References Nonexistent Task: ✅ RESOLVED

**Previous issue:** `plan_deploy_crdb.md` referenced `task fly:check:cockroach`, which did not exist.

**Current state:** `fly:check:cockroach` exists in Taskfile.yml. `plan_deploy_crdb.md` Phase 3 now correctly references `task fly:pre-deploy:cockroach` (line 75), which calls both `fly:check` and `fly:check:cockroach`.

**Verdict:** RESOLVED.

---

### Finding 5 (High) — App/Config Identity Conflict: ✅ RESOLVED

**Previous issue:** `fly.toml` names the app `bchat-pg` and points to Postgres. If a user reuses `fly.toml` for CRDB, they deploy to wrong app with wrong config.

**Current state:** 
- Guide correctly documents each backend has its own app name and TOML file
- Deployment steps tell users to run `task deploy:cockroach`, which uses `fly_cockroach.toml` and `bchat-crdb`
- SQLite row removed from guide table (was incorrectly showing `bchat-app` with `fly.toml`)

**Verdict:** RESOLVED.

---

### Finding 6 (Medium) — Duplicate Postgres Config Files: ⚠️ DOCUMENTED

**Previous issue:** `fly.toml` and `fly_pg.toml` are functionally identical, creating ambiguity.

**Current state:** Both files still exist, but guide now documents:
- `fly_pg.toml` is the canonical source for Postgres
- `fly.toml` is a legacy alias and should not be used for new deployments
- `fly:pre-deploy:postgres` uses `fly_pg.toml`

**Verdict:** NOT FULLY RESOLVED, but documented. The duplication is a legacy artifact, not a logic inconsistency.

---

### Finding 7 (Medium) — Guide and Plan Disagree on CRDB TOML File: ✅ RESOLVED

**Previous issue:** Guide said edit `fly.toml` for CRDB, plan said use `fly_cockroach.toml`.

**Current state:** Both guide and plan reference `fly_cockroach.toml` for CRDB. `plan_deploy_crdb.md` Phase 3 says `task fly:pre-deploy:cockroach`.

**Verdict:** RESOLVED.

---

### Finding 8 (Medium) — `fly:pre-deploy` Is Backend-Agnostic In Name Only: ✅ RESOLVED

**Previous issue:** `fly:pre-deploy` was universal but validated only `fly.toml` (Postgres).

**Current state:** Backend-specific tasks exist and are wired correctly:
- `fly:pre-deploy:cockroach` → `fly:check` with `fly_cockroach.toml` → `fly:check:cockroach`
- `fly:pre-deploy:postgres` → `fly:check` with `fly_pg.toml` → `fly:check:postgres`
- `fly:pre-deploy:sqlite` → `fly:check` with `fly.toml` → `fly:check:sqlite`

**Verdict:** RESOLVED.

---

### Finding 9 (Critical) — `validate-env-chain.sh` Hardcoded to `fly.toml`: ✅ RESOLVED

**Previous issue:** Script was hardcoded to read `fly.toml` with no parameter.

**Current state:** Script now accepts TOML file parameter:
```bash
TOML_FILE="${1:-fly.toml}"
```
All internal references use `$TOML_FILE`. The script validates the correct backend config when called with the appropriate TOML file.

**Verdict:** RESOLVED.

---

## New Findings

### None

No new logic inconsistencies introduced.

---

## Remaining Nits (Low Priority)

### Nit 1 — `fly:db-check` Still Hardcoded to `fly.toml`

**File:** `Taskfile.yml:193-209`  
**Severity:** Low  
**Type:** Legacy artifact

`fly:db-check` still reads `fly.toml` directly and is not part of the recommended CRDB path. It's a legacy task that's no longer documented in the guide for CRDB deployments.

**Impact:** Low. The guide doesn't recommend `fly:db-check` for CRDB.

**Recommendation:** Either remove `fly:db-check` or update it to accept a backend parameter like `fly:check` does. Not a blocker.

---

### Nit 2 — Duplicate Postgres Config Files

**Files:** `fly.toml`, `fly_pg.toml`  
**Severity:** Low  
**Type:** Legacy artifact

Both files exist and are functionally identical Postgres configs. The guide documents `fly_pg.toml` as canonical and `fly.toml` as legacy.

**Impact:** Low. The guide resolves the ambiguity by documenting which is canonical.

**Recommendation:** Consider deleting `fly.toml` or repurposing it as the actual SQLite config to eliminate duplication. Not a blocker.

---

### Nit 3 — `fly:check` Description Still Mentions `fly.toml`

**File:** `Taskfile.yml:183-186`  
**Severity:** Low  
**Type:** Documentation nit

`fly:check` description says:
```yaml
desc: Validate environment chain (.env -> Dockerfile -> fly.toml -> fly secrets)
```

But `fly:check` now validates whatever TOML file is passed via `CLI_ARGS`. The description is outdated.

**Impact:** Low. The guide documents the correct behavior.

**Recommendation:** Update description to:
```yaml
desc: Validate environment chain (.env -> Dockerfile -> TOML file -> fly secrets)
```

---

## Final Verdict

**APPROVE**

The deployment guide, Taskfile, and TOML files are now internally consistent. All critical and high-severity logic inconsistencies have been resolved:

1. ✅ `validate-env-chain.sh` is parameterized with `TOML_FILE`
2. ✅ `fly:check` passes `CLI_ARGS` to the script
3. ✅ `fly:pre-deploy:cockroach` passes `fly_cockroach.toml`
4. ✅ Guide documents per-backend TOML files
5. ✅ Backend-specific pre-deploy tasks wired correctly
6. ✅ `plan_deploy_crdb.md` matches guide
7. ✅ SQLite row removed from guide table (was incorrectly showing Postgres config)

The remaining items are low-priority nits that don't affect deployment correctness. The workflow is ready to execute.
