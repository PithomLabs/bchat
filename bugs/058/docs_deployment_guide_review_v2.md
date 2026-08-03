# Bug 058 — Adversarial Review v2: Deployment Guide vs Taskfile vs TOML Consistency

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

The previous review found 8 logic inconsistencies. This follow-up verifies whether the implementation properly resolved them.

**Status:** 2 findings fully resolved, 3 findings partially resolved, 1 finding not resolved, 1 new finding introduced.

**Verdict:** REQUEST CHANGES — 2 Critical, 1 High, 2 Medium.

---

## Resolution Check: Previous Findings

### Finding 4 (High) — Plan References Nonexistent Task: ✅ RESOLVED

**Previous issue:** `plan_deploy_crdb.md` referenced `task fly:check:cockroach`, which did not exist in `Taskfile.yml`.

**Current state:** `Taskfile.yml:231-234` now defines:
```yaml
fly:check:cockroach:
  desc: Validate CockroachDB migrations before deployment
  cmds:
    - ./scripts/validate-cockroach-compat.sh
```

`plan_deploy_crdb.md` Phase 3 references `task fly:check:cockroach`, which now exists.

**Verdict:** RESOLVED.

---

### Finding 7 (Medium) — Guide and Plan Disagree on CRDB TOML File: ✅ RESOLVED

**Previous issue:** `docs_deployment_guide.md` said to edit `fly.toml` for CRDB, while `plan_deploy_crdb.md` correctly used `fly_cockroach.toml`.

**Current state:** `docs_deployment_guide.md` now says:
```markdown
| CockroachDB | `fly_cockroach.toml` | `bchat-crdb` | `Dockerfile.cockroach.fly` |
```

And the deployment steps say:
```markdown
### Step 1: Set Secrets
### Step 2: Run Pre-deployment Checks
  task fly:pre-deploy:cockroach
### Step 3: Deploy
  task deploy:cockroach
```

`plan_deploy_crdb.md` also correctly references `fly_cockroach.toml`.

**Verdict:** RESOLVED.

---

### Finding 1 (Critical) — Validation Logic Reads Wrong Config for CRDB: ⚠️ PARTIALLY RESOLVED

**Previous issue:** `fly:db-check` read `fly.toml` (Postgres config) and auto-detected Postgres for CRDB deployments.

**Current state:** The old `fly:db-check` still exists and still reads `fly.toml`. However, the guide no longer recommends it for CRDB. Instead, it recommends `fly:pre-deploy:cockroach`, which calls:
1. `fly:check` → runs `validate-env-chain.sh` → reads `fly.toml` (Postgres config)
2. `fly:check:cockroach` → runs `validate-cockroach-compat.sh`

The CRDB-specific compatibility check is now present, but the environment chain validation still validates the Postgres config file, not the CRDB config file.

**Remaining issue:** `fly:pre-deploy:cockroach` validates Postgres env vars against `fly.toml`, then runs CRDB compat checks. The two halves of the validation are still checking different backends.

**Verdict:** PARTIALLY RESOLVED. The CRDB-specific validation exists, but the environment chain validation is still backend-wrong.

---

### Finding 2 (Critical) — Guide Says ONE `fly.toml`, But Deploy Logic Uses Multiple TOML Files: ⚠️ PARTIALLY RESOLVED

**Previous issue:** Guide claimed ONE `fly.toml` manually edited per backend, but code uses separate TOML files.

**Current state:** The guide now says:
```markdown
**Key Design Decision:** Each backend has its own TOML file and Fly app. No manual editing required.
```

And the table shows:
```markdown
| Backend | TOML File | Fly App | Dockerfile |
|---------|-----------|---------|------------|
| SQLite | `fly.toml` | `bchat-pg` | `Dockerfile.fly` |
| Postgres | `fly.toml` or `fly_pg.toml` | `bchat-pg` | `Dockerfile.pg.fly` |
| CockroachDB | `fly_cockroach.toml` | `bchat-crdb` | `Dockerfile.cockroach.fly` |
```

The guide now correctly documents multiple TOML files. However, the table still suggests SQLite uses `fly.toml` with app `bchat-pg`, which is wrong — `fly.toml` is a Postgres config, not a SQLite config. The SQLite row should not reference `bchat-pg`.

Also, the table says Postgres uses `fly.toml` OR `fly_pg.toml`, which preserves the duplicate-config ambiguity instead of resolving it.

**Verdict:** PARTIALLY RESOLVED. The guide now acknowledges multiple TOML files, but the table is still internally inconsistent.

---

### Finding 3 (High) — `fly:check` Validates Wrong Target for CRDB: ❌ NOT RESOLVED

**Previous issue:** `fly:check` validates `.env` -> `Dockerfile` -> `fly.toml` -> `fly secrets`. For CRDB, it should validate `fly_cockroach.toml`.

**Current state:** `fly:check` still runs `scripts/validate-env-chain.sh`, which is hardcoded to read `fly.toml`:
```bash
# Step 2: Determine which Dockerfile to check from fly.toml
DOCKERFILE=$(grep -E '^\s*dockerfile\s*=' fly.toml | head -1 | ...)
```

The script does not accept a TOML file parameter. The new `fly:pre-deploy:cockroach` task calls `fly:check`, which validates the Postgres config, not the CRDB config.

**Impact:** When deploying CockroachDB, the pre-deployment validation chain checks Postgres env vars (`MEMOS_DRIVER=postgres`, `LANCEDB_STORAGE_PROVIDER=s3`) against the CRDB Dockerfile and `.env`. This will produce false validation errors or missed mismatches.

**Verdict:** NOT RESOLVED.

---

### Finding 5 (High) — App/Config Identity Conflict: ⚠️ PARTIALLY RESOLVED

**Previous issue:** `fly.toml` names the app `bchat-pg` and points to Postgres. If a user follows the guide and reuses `fly.toml` for CRDB, they deploy to the wrong app with wrong config.

**Current state:** The guide now correctly documents that each backend has its own app name and TOML file. The deployment steps tell users to run `task deploy:cockroach`, which uses `fly_cockroach.toml` and `bchat-crdb`.

However, the guide's table still shows SQLite using `fly.toml` with app `bchat-pg`, which is misleading. SQLite should not use a Postgres app name.

**Verdict:** PARTIALLY RESOLVED. The CRDB path is now correct, but the SQLite row in the table is wrong.

---

### Finding 6 (Medium) — Duplicate Postgres Config Files: ❌ NOT RESOLVED

**Previous issue:** `fly.toml` and `fly_pg.toml` are functionally identical, creating ambiguity about which is the source of truth.

**Current state:** Both files still exist and are still identical in structure. The guide acknowledges both but does not resolve the duplication:
```markdown
| Postgres | `fly.toml` or `fly_pg.toml` | `bchat-pg` | `Dockerfile.pg.fly` |
```

The "or" preserves the ambiguity instead of resolving it.

**Verdict:** NOT RESOLVED.

---

### Finding 8 (Medium) — `fly:pre-deploy` Is Backend-Agnostic In Name Only: ⚠️ PARTIALLY RESOLVED

**Previous issue:** `fly:pre-deploy` was presented as universal but validated only `fly.toml` (Postgres).

**Current state:** The guide now has backend-specific pre-deploy tasks:
- `fly:pre-deploy:cockroach`
- `fly:pre-deploy:postgres`
- `fly:pre-deploy:sqlite`

However, all three tasks call `fly:check`, which reads `fly.toml`. So `fly:pre-deploy:cockroach` validates Postgres env vars, not CRDB env vars. The task names are backend-specific, but the underlying validation is not.

**Verdict:** PARTIALLY RESOLVED. The tasks are now named correctly, but the validation logic inside them is still wrong for CRDB.

---

## New Finding

### Finding 9 (Critical) — `validate-env-chain.sh` Is Hardcoded to `fly.toml`

**File:** `scripts/validate-env-chain.sh`  
**Severity:** Critical  
**Type:** Logic inconsistency

The script is hardcoded to read `fly.toml`:
```bash
# Step 2: Determine which Dockerfile to check from fly.toml
DOCKERFILE=$(grep -E '^\s*dockerfile\s*=' fly.toml | head -1 | ...)
```

And:
```bash
# Step 4: Read fly.toml [env] vars
done < fly.toml
```

There is no parameter to specify a different TOML file. The new `fly:pre-deploy:cockroach` task calls `fly:check`, which calls this script, which reads `fly.toml`. So the CRDB pre-deployment validation validates Postgres configuration.

**Impact:** The entire backend-specific validation structure is undermined by a single hardcoded filename. Even though `fly:check:cockroach` runs the CRDB compat scanner, the environment chain validation that runs before it is checking the wrong file.

**Fix:** Add a parameter to `validate-env-chain.sh`:
```bash
#!/bin/bash
# Usage: ./scripts/validate-env-chain.sh [toml-file]
TOML_FILE="${1:-fly.toml}"
```

Then update `fly:check` to accept a backend parameter and pass the correct TOML file:
```yaml
fly:check:
  cmds:
    - ./scripts/validate-env-chain.sh {{.CLI_ARGS}}

fly:pre-deploy:cockroach:
  cmds:
    - task: fly:check
      args: fly_cockroach.toml
    - task: fly:check:cockroach
```

---

## Resolution Summary

| # | Finding | Severity | Status |
|---|---------|----------|--------|
| 1 | Validation logic reads wrong config for CRDB | Critical | ⚠️ PARTIALLY RESOLVED |
| 2 | Guide ONE-`fly.toml` claim contradicts multi-TOML deploy | Critical | ⚠️ PARTIALLY RESOLVED |
| 3 | `fly:check` validates wrong target for CRDB | High | ❌ NOT RESOLVED |
| 4 | Plan references nonexistent `fly:check:cockroach` | High | ✅ RESOLVED |
| 5 | App/config identity conflict | High | ⚠️ PARTIALLY RESOLVED |
| 6 | Duplicate Postgres config files | Medium | ❌ NOT RESOLVED |
| 7 | Guide and plan disagree on CRDB TOML file | Medium | ✅ RESOLVED |
| 8 | `fly:pre-deploy` is backend-agnostic in name only | Medium | ⚠️ PARTIALLY RESOLVED |
| 9 | `validate-env-chain.sh` hardcoded to `fly.toml` | Critical | ❌ NEW |

---

## Required Changes Before Execution

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | `validate-env-chain.sh` hardcoded to `fly.toml` | Critical | Add TOML file parameter |
| 2 | `fly:check` validates wrong TOML for CRDB | Critical | Pass correct TOML file based on backend |
| 3 | Guide table still shows SQLite using `fly.toml` with `bchat-pg` app | High | Fix table: SQLite should not reference Postgres app |
| 4 | Guide table preserves duplicate Postgres config ambiguity | Medium | Document which TOML is canonical for Postgres |
| 5 | `fly:pre-deploy:*` tasks call `fly:check` with wrong TOML | Medium | Update tasks to pass correct TOML file |

---

## Final Verdict

**REQUEST CHANGES**

The implementation made real progress: `fly:check:cockroach` exists, the guide now documents per-backend TOML files, and backend-specific pre-deploy tasks are present. However, the **core validation script is still hardcoded to `fly.toml`**, which means the entire backend-specific validation structure is theater — `fly:pre-deploy:cockroach` validates Postgres configuration, not CockroachDB configuration.

**Minimum viable fixes:**
1. Add TOML file parameter to `validate-env-chain.sh`
2. Update `fly:pre-deploy:*` tasks to pass the correct TOML file
3. Fix the guide's backend table to show correct app names per backend
4. Resolve the duplicate Postgres config ambiguity

Do not use this as an authoritative deployment guide until the validation logic actually validates the correct backend.
