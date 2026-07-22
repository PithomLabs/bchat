# Bug 046: Chat Widget Not Appearing — Hugo Defaults + Migration Automation + Parity

**Status:** PLANNED (supersedes plan3.md)
**Date:** 2026-07-23
**Depends on:** Bug 045 (migrator.go fix for `GetCurrentSchemaVersion()`)
**Affected repos:** bchat server (`/home/chaschel/Documents/go/bchat`), Hugo site (`/home/chaschel/Documents/go/izaakmaine.github.io-main`)

**Revision notes:** Incorporates findings from `bugs/046/plan3_review.md` (5 findings: 3 high, 1 medium, 1 low). Key architectural changes from plan3.md: version derivation at runtime from embedded FS (no sed), no auto-generation of Postgres SQL (templates only), script test fixtures, rollback contract documentation, Hugo CI gate on missing `bchat.baseUrl`.

---

## Problem Statement

Three independent root causes prevent the bchat chat widget from appearing and make the migration process fragile:

### Root Cause 1: `chatBaseUrl` Hardcoded to localhost in Hugo Content

Every tenant `_index.md` hardcodes `chatBaseUrl: "http://localhost:8081"`. When the Hugo site is deployed to GitHub Pages, the browser tries to fetch `embed.js` from `localhost:8081` — which is the visitor's own machine, not the bchat server. The script fails with `ERR_CONNECTION_REFUSED` and the widget never initializes.

The Hugo template (`layouts/_default/list.html:225`) already has a default:
```go
{{ $chatUrl := .Params.chatBaseUrl | default "https://bchat-pg.fly.dev" }}
```
But this default is never reached because every content file explicitly sets the value.

**Fix:** Remove hardcoded `chatBaseUrl` from all content files. Make the default configurable via `hugo.yaml` site params with a safe localhost fallback.

### Root Cause 2: Migration System Skips Files in Newer Minor-Version Directories

Bug 045 documents this fully. The short version: `GetCurrentSchemaVersion()` only scans the directory matching `DevVersion` (e.g., `0.31/`), so migrations in `0.32/` and `0.33/` are silently skipped. The `agent_tenants` table is missing `transcript_signing_key` and `transcript_signing_key_nonce` columns, causing `ListAgentTenants` to fail with a SQL error that propagates as 404 "Agent not found".

**Fix:** Already planned in bug 045 Steps 1-5. This bug adds the automation layer to prevent recurrence.

### Root Cause 3: No Cross-Driver Schema Parity Enforcement

SQLite and Postgres migrations are maintained independently with no automated check that they produce the same logical schema. The incremental migration paths are heavily divergent (SQLite 0.25 has 36 files, Postgres has 2 + catch-up). While LATEST.sql files are currently in sync, there is no CI gate to prevent drift.

**Fix:** Add a cross-driver parity validator, document the type mapping, and CI-gate file-list parity.

---

## Scope

| In Scope | Out of Scope |
|----------|-------------|
| Hugo chatBaseUrl defaults via hugo.yaml | The migrator.go code fix (bug 045) |
| Version info script (read-only, from embedded FS) | Widget source code changes |
| `task migrate:new` (templates only, no auto-gen) | bchat server feature work |
| Cross-driver parity validator (`validate-parity.sh`) | Postgres-specific fixes (covered by bug 045) |
| SQLite-Postgres type mapping documentation | Content data changes (YAML, images, etc.) |
| Definitive migration guide (supersedes old docs) | SQL parser / semantic comparison engine |
| AGENTS.md / old doc deprecation | |
| Script test fixtures (`task test:scripts`) | |

---

## Parity Design Philosophy

> **Design Committee Guidance (from plan_review.md):**
>
> The automation should detect and accommodate divergence, not enforce strict parity that doesn't exist.

Three levels of parity:

| Level | Definition | Enforce? |
|-------|-----------|----------|
| **Schema parity** | `LATEST.sql` produces the same logical schema (tables, columns, indexes) | **Yes — CI gate** |
| **File-list parity** | Migration files in corresponding directories match (same patch numbers) | **Yes — CI gate** (both SQLite and Postgres files must exist) |
| **Incremental path parity** | Identical SQL in each migration file | **No** — different drivers require different SQL |

---

## Detailed Implementation

### Part A: Hugo Chat Widget Fix (Hugo repo)

#### Step 12: Add `bchat.baseUrl` to `hugo.yaml`

**File:** `/home/chaschel/Documents/go/izaakmaine.github.io-main/hugo.yaml`

Add under the existing `params:` block (after line 66):

```yaml
params:
  # ... existing params ...

  bchat:
    # Base URL of the bchat server for the chat widget.
    # Override per-environment via config/ directory or environment variables.
    # Default: localhost for local dev (hugo server), set explicitly for production.
    baseUrl: "http://localhost:8081"
```

**Rationale:** The default is `localhost:8081` because:
- `hugo server` (local dev) always runs against the local bchat server
- Production deployments override this via Hugo's environment config (`config/production/params.yaml`) or CLI flags
- No production URL is hardcoded in the repo — security-conscious default

**For production deployments**, create `config/production/params.yaml`:
```yaml
params:
  bchat:
    baseUrl: "https://bchat.example.com"
```
Or pass at build time:
```bash
hugo --environment production --set params.bchat.baseUrl=https://bchat.example.com
```

#### Step 13: Update Landing Page Template

**File:** `/home/chaschel/Documents/go/izaakmaine.github.io-main/layouts/_default/list.html`

Change line 225 from:
```go
{{ $chatUrl := .Params.chatBaseUrl | default "https://bchat-pg.fly.dev" }}
```

To:
```go
{{ $chatUrl := or .Params.chatBaseUrl site.Params.bchat.baseUrl "http://localhost:8081" }}
{{ if not site.Params.bchat.baseUrl }}
  {{ warnf "WARNING: bchat.baseUrl not set in site params. Widget will use localhost fallback." }}
{{ end }}
```

**Why `or` instead of `default`:** Hugo's `default` checks for `nil` (undefined). An empty string `""` is defined and non-nil, so `default` will NOT fall through. If a developer accidentally sets `chatBaseUrl: ""` in front matter (easy copy-paste error), the widget receives `""` and loads `embed.js` from the relative URL root instead of the bchat server. `or` returns the first non-zero value (non-nil, non-empty, non-false), handling both `nil` and `""` correctly.

**Three-tier fallback:**
1. Page-level `chatBaseUrl` in front matter (highest priority — per-tenant overrides)
2. `site.Params.bchat.baseUrl` from `hugo.yaml` (configurable per environment)
3. Hardcoded `"http://localhost:8081"` (safe default — never produces a broken production URL)

**Security rationale:** The hardcoded fallback is localhost. A misconfigured deployment will fail locally (developer catches it), not in production (where it would expose an internal URL or fail silently).

**Build-time warning:** The `warnf` block alerts developers when `bchat.baseUrl` is not configured. Production sites can use `--templateMetrics` to catch this during CI.

#### Steps 14-16: Remove Hardcoded `chatBaseUrl` from Content Files

Remove the `chatBaseUrl: "http://localhost:8081"` line from:
- `content/rgresidences/_index.md` (line 5)
- `content/bchat/_index.md`
- `content/evpn/_index.md`

After removal, the template falls through to `site.Params.bchat.baseUrl` (from hugo.yaml), which defaults to `http://localhost:8081` for local dev.

#### Step 17: Hugo Deploy CI Gate

**File:** Deploy pipeline (GitHub Actions, deploy script)

Add a grep on Hugo build output to fail production builds missing `bchat.baseUrl`:
```bash
hugo --environment production 2>&1 | tee hugo-build.log
if grep -q "bchat.baseUrl not set" hugo-build.log; then
  echo "FATAL: Production build missing bchat.baseUrl"
  exit 1
fi
```

**Rationale:** Hugo's `warnf` prints to stderr but doesn't fail the build. A production deployment that omits `bchat.baseUrl` will build successfully, deploy successfully, and serve pages where the widget silently fails. This grep catches the warning and fails the deploy step.

---

### Part B: Migration Automation (bchat server repo)

#### Step 1: Bump Version to 0.34.0

**File:** `internal/version/version.go`

Change:
```go
var Version = "0.31.0"
var DevVersion = "0.31.0"
```

To:
```go
var Version = "0.34.0"
var DevVersion = "0.34.0"
```

**Rationale:** `0.33/` is the latest migration directory. Next development cycle starts at `0.34.0`.

**Architecture (from plan3_review.md #1):** `Version` and `DevVersion` remain as defaults. At runtime, `GetCurrentSchemaVersion()` (fixed by bug 045) scans all directories in the embedded FS and returns the highest version found, overriding these defaults. The version is derived at runtime, not at build time. This eliminates merge conflicts from sed-based version bumping and ensures traceability between a binary's version and the migration files it was built from.

#### Step 2: Create `scripts/bump-version.sh` (Informational)

**File:** `scripts/bump-version.sh` (new)

**Read-only script.** Scans `store/migration/sqlite/` for the highest directory and patch file, computes the version, and prints it. Does NOT modify `version.go`. Supports `--dry-run` (which is the default — there is no non-dry-run mode).

Key logic:
- Find highest migration directory (e.g., `0.33/`)
- Find highest patch file number in that directory (e.g., `00` from `00__fix_max_message_length_default.sql`)
- Compute version: `0.33.1`
- Print the computed version
- Print the current `DevVersion` from `version.go` (for comparison)
- Exit 0 if they match, exit 1 if they differ (informational, not blocking)

**No sed, no file modification.** The script is purely informational — it shows what `GetCurrentSchemaVersion()` would compute at runtime. If the computed version differs from `DevVersion` in `version.go`, the script prints a warning but does not modify any files. The developer updates `version.go` manually when releasing.

#### Step 3: Create `scripts/create-migration.sh` (Templates Only)

**File:** `scripts/create-migration.sh` (new)

Creates SQLite and Postgres migration files with TODO templates. **No auto-generation.** Developer writes SQL for each driver.

Key properties:
- Validates migration name (snake_case)
- Creates SQLite file with comment header and TODO placeholder
- Creates Postgres file with comment header and TODO placeholder (same template, different driver name in header)
- Calls `bump-version.sh` informationally (prints computed version)
- Supports `--dry-run`

**Why no auto-generation (from plan3_review.md #2):** The 10-row substitution table is a leaky abstraction. Real SQLite→Postgres translation involves DDL transactionality (ALTER TABLE is transactional in SQLite but not in Postgres), INSERT OR REPLACE semantics (different from ON CONFLICT), type affinity differences, generated columns, and RETURNING clauses. A developer who trusts auto-generated output will deploy broken Postgres migrations. The "flag for review" mechanism is a human process with no automation gate.

**File-list parity CI gate:** `validate-parity.sh` (Step 6) enforces that both SQLite and Postgres files exist for each migration. This catches the most common drift pattern (forgetting to create the Postgres file) without pretending the content can be machine-translated.

**Template contents:**
```sql
-- Migration: <name>
-- Driver: sqlite (or postgres)
-- Date: <YYYY-MM-DD>
-- Bug: 046
--
-- TODO: Write migration SQL here.
-- See docs/TYPE_MAPPING.md for SQLite-Postgres type mapping.

```

#### Step 4: Create `docs/TYPE_MAPPING.md`

**File:** `docs/TYPE_MAPPING.md` (new)

Explicit SQLite-Postgres type mapping reference. Covers:
- Type mapping table (BLOB->BYTEA, SERIAL, BOOLEAN, JSONB, TIMESTAMPTZ, etc.)
- Syntax differences (quoting, INSERT OR IGNORE, timestamp functions, reserved words)
- Migration writing rules for each driver
- Review checklist for manually-written Postgres migrations
- Historical type differences from past migrations (0.19, 0.20, 0.22, 0.24, 0.32)
- **SQL parsing limitations disclaimer** — documents that shell-level awk/grep cannot parse SQL reliably; the parity validator is best-effort lint, not a semantic comparison engine

#### Step 5: Create `scripts/validate-parity.sh`

**File:** `scripts/validate-parity.sh` (new)

Cross-driver parity validator. **Three checks:**

**Check 1 — File-list parity (CI gate):** Compares the set of migration files in corresponding `sqlite/<dir>/` and `postgres/<dir>/` directories. Every file in `sqlite/<dir>/` must have a corresponding `postgres/<dir>/` file with the same patch number, and vice versa. Missing files fail the build.

**Check 2 — Schema parity (best-effort lint):** Parses both `LATEST.sql` files, extracts table names, column names per table, and index names, then compares them. Applies simplified type mapping for comparison. Warns on differences (does not fail — shell SQL parsing is unreliable).

**Check 3 — Historical divergence documentation:** Lists known divergences (e.g., Postgres 0.33 has `system_secret` table that SQLite 0.33 lacks) and skips those from the comparison.

**SQL parsing limitations (from plan3_review.md #4):** Shell awk/grep cannot parse SQL reliably — nested parentheses in `CHECK` constraints, multi-line column definitions, inline comments, and type functions in default expressions all produce false positives/negatives. The validator is documented as best-effort lint. The definitive approach is database introspection (piggyback on `validate-pg-migrations.sh` which uses `information_schema`), noted as a future enhancement.

Key properties:
- `--verbose` mode shows per-table column counts
- Collects ALL differences before exiting (does not fail-fast on first difference)
- Exit codes: 0 = pass, 1 = file-list differences (fail), 2 = schema differences (warn), 3 = both

#### Step 6: Create `scripts/test/` Directory

**File:** `scripts/test/bump-version/` (new directory)

Test fixtures for automation scripts:
- `version.go.fixture` — known-good `version.go` with specific version values
- `migration-tree/` — fake migration directory tree with known structure
- `run-tests.sh` — test runner that:
  1. Runs `bump-version.sh` against the fixture, asserts exit code 0 and expected output
  2. Runs `create-migration.sh --dry-run` with a test name, asserts file creation (in temp dir)
  3. Runs `validate-parity.sh` against known-good LATEST.sql pair, asserts exit code 0
  4. Runs `validate-parity.sh` against intentionally drifted pair, asserts non-zero exit code
  5. Runs each script with invalid input, asserts non-zero exit code

**Taskfile task:**
```yaml
  test:scripts:
    desc: Run automation script tests
    cmds:
      - ./scripts/test/run-tests.sh
```

**CI integration:** Add `test:scripts` as a dependency of `build:backend` (alongside `validate:migrations` and `validate:parity`). This catches regressions in the automation scripts themselves.

#### Step 7: Add Taskfile Tasks

**File:** `Taskfile.yml`

```yaml
  version:info:
    desc: Show computed version from migration filesystem (informational, read-only)
    cmds:
      - ./scripts/bump-version.sh

  migrate:new:
    desc: Create new migration file templates for both drivers (usage: task migrate:new NAME=add_widget_config)
    cmds:
      - ./scripts/create-migration.sh "{{.NAME}}"

  validate:parity:
    desc: Validate SQLite and Postgres migration parity
    cmds:
      - ./scripts/validate-parity.sh

  test:scripts:
    desc: Run automation script tests
    cmds:
      - ./scripts/test/run-tests.sh
```

**CI gating:** Add `validate:parity` and `test:scripts` as dependencies of `build:backend`:

```yaml
  build:backend:
    desc: Build backend binary
    deps: [validate:migrations, validate:parity, test:scripts]
```

This makes parity and script correctness build-time gates — if either fails, the binary won't compile.

#### Step 8: Update AGENTS.md Migration Section

**File:** `AGENTS.md`

Replace the "Database Migrations" section (lines 526-540) with a quick reference pointing to the new guide, documenting `task migrate:new` as the primary workflow, and listing the three validation commands (validate-migrations.sh, validate:parity, test:scripts).

#### Step 9: Add Deprecation Notice to Old Doc

**File:** `docs/DOCS_DATABASE_MIGRATION.MD`

Add at the very top:
```markdown
> **DEPRECATED:** This document is superseded by [DOCS_DATABASE_MIGRATION_GUIDE.md](./DOCS_DATABASE_MIGRATION_GUIDE.md).
> It is retained for historical reference on the fly.io incident and LATEST.sql validation options.
> For current migration procedures, see the new guide.

---
```

#### Step 10: Create Definitive Migration Guide

**File:** `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` (new)

Comprehensive guide covering:
- Overview of the compile-time embedded migration system
- Quick reference table (all commands)
- Architecture (directory structure, version detection, migration flow)
- Step-by-step: adding a new migration (with `task migrate:new`)
- Step-by-step: adding a new version directory
- Rules and conventions (migration file rules, LATEST.sql rules, version naming)
- Cross-driver parity (three levels: schema/file-list/incremental path; type mapping; manual writing; validation)
- **CI gates matrix** (which gate catches which historical bug):

| Historical Bug | CI Gate That Would Catch It |
|----------------|---------------------------|
| 008 — Unique constraint failure | `validate-pg-migrations.sh` (runs all migrations against real PG) |
| 009 — Migration 28 hotfix | `validate-db-migrations.sh` (applies incrementally, compares schemas) |
| 045 — Version directory skip | `version:info` (shows computed version from FS) |
| 046 — LATEST.sql drift | `validate:parity` (compares sqlite vs postgres schema + file lists) |

- **Rollback contract (from plan3_review.md #4):**
  - **Safe to roll back:** `ALTER TABLE ADD COLUMN`, `CREATE TABLE` (additive-only changes)
  - **Not safe to roll back:** `UPDATE`/`DELETE` data changes, `DROP COLUMN`, `ALTER COLUMN TYPE`, `RENAME`
  - **Procedure:** If a forward-only migration fails mid-flight, the database may be in an unknown state. Restore from backup, not re-run. `ALTER TABLE` is transactional in SQLite but NOT in Postgres — a partial Postgres migration may leave the schema in an inconsistent state.
- Gotchas and known issues (tests always use LATEST.sql, DDL transactionality, WAL visibility, go:embed rebuild, dual version tracking, LATEST.sql drift)
- Historical divergence cases (Postgres 0.33 has `system_secret` table that SQLite 0.33 lacks — this is intentional, documented)
- Testing checklist (fresh DB, existing DB, idempotency, schema validation)
- Deployment checklist
- Troubleshooting
- Historical context (bugs 008, 009, 015, 045, 046)

---

## Implementation Order

| Step | File(s) | Change | Depends on |
|------|---------|--------|-----------|
| 1 | `internal/version/version.go` | Bump to 0.34.0 | — |
| 2 | `scripts/bump-version.sh` | Informational version script (read-only) | — |
| 3 | `scripts/create-migration.sh` | Migration templates (no auto-gen) | 2, 4 |
| 4 | `docs/TYPE_MAPPING.md` | Type mapping reference | — |
| 5 | `scripts/validate-parity.sh` | Cross-driver parity validator | 4 |
| 6 | `scripts/test/` | Script test fixtures + runner | 2, 3, 5 |
| 7 | `Taskfile.yml` | Add tasks + CI gating | 2, 3, 5, 6 |
| 8 | `AGENTS.md` | Update migration section | 10 |
| 9 | `docs/DOCS_DATABASE_MIGRATION.MD` | Deprecation notice | — |
| 10 | `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` | Definitive guide | 2, 3, 4, 5, 6, 7 |
| 12 | Hugo `hugo.yaml` | Add bchat.baseUrl param | — |
| 13 | Hugo `layouts/_default/list.html` | 3-tier fallback with `or` | 12 |
| 14-16 | Hugo content files | Remove chatBaseUrl | 13 |
| 17 | Deploy pipeline | Hugo CI gate on missing bchat.baseUrl | 13 |

Steps 1-10 (bchat server) and 12-17 (Hugo site) are independent. Step 10 depends on 2, 3, 4, 5, 6, 7.

---

## Verification

### Automation Scripts
1. `task migrate:new NAME=test_migration` — creates both files with TODO templates
2. `task version:info` — prints computed version from FS, shows current version.go value
3. `task validate:parity` — passes with current LATEST.sql files
4. `task test:scripts` — all fixtures pass
5. `go build ./bin/memos/` — succeeds with new version

### Hugo Site
6. `hugo server` — loads rgresidences page, widget loads from localhost:8081 (from hugo.yaml param)
7. Template falls through correctly when chatBaseUrl is omitted from front matter
8. Template handles `chatBaseUrl: ""` correctly (falls through to site param, not empty string)
9. `warnf` fires when `bchat.baseUrl` is not set in site params

### Widget (after bug 045 migrator fix)
10. `curl http://localhost:8081/widget/rgresidences/embed.js` — returns JS
11. `PRAGMA table_info(agent_tenants)` — shows transcript_signing_key columns
12. Widget appears in browser on rgresidences landing page
