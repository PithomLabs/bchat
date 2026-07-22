# Bug 046: Chat Widget Not Appearing — Hugo Defaults + Migration Automation + Parity

**Status:** PLANNED (supersedes plan2.md)
**Date:** 2026-07-23
**Depends on:** Bug 045 (migrator.go fix for `GetCurrentSchemaVersion()`)
**Affected repos:** bchat server (`/home/chaschel/Documents/go/bchat`), Hugo site (`/home/chaschel/Documents/go/izaakmaine.github.io-main`)

**Revision notes:** Incorporates findings from `bugs/045/plan_review.md` (Layers 1-4 prevention strategy) and `bugs/046/plan_review.md` (6 findings: 3 critical, 2 moderate, 1 low). Key changes from plan2.md: Hugo `or` pipe replaces `default`, anchored sed patterns, parity validator handles file-list divergence, CI gating for parity check, overwrite protection for Postgres auto-generation.

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

SQLite and Postgres migrations are maintained independently with no automated check that they produce the same logical schema. The incremental migration paths are heavily divergent (SQLite 0.25 has 36 files, Postgres has 2 + catch-up). While LATEST.sql files are currently in sync, there is no CI gate to prevent drift. The `create-migration.sh` approach of creating identical files is insufficient because the same logical change requires different SQL syntax per driver.

**Fix:** Add a cross-driver parity validator, auto-generate Postgres migrations from SQLite with type substitutions, and document the type mapping.

---

## Scope

| In Scope | Out of Scope |
|----------|-------------|
| Hugo chatBaseUrl defaults via hugo.yaml | The migrator.go code fix (bug 045) |
| Version bump script (derive from FS) | Widget source code changes |
| `task migrate:new` with auto Postgres generation | bchat server feature work |
| Cross-driver parity validator (`validate-parity.sh`) | Postgres-specific fixes (covered by bug 045) |
| SQLite-Postgres type mapping documentation | Content data changes (YAML, images, etc.) |
| Definitive migration guide (supersedes old docs) | SQL parser / semantic comparison engine |
| AGENTS.md / old doc deprecation | |

---

## Parity Design Philosophy

> **Design Committee Guidance (from plan_review.md):**
>
> The three root causes share a root cause: **assuming the two drivers will always be structurally parallel**. History shows they have already diverged (0.33), and the incremental migration paths are heavily different (SQLite has 30+ directories, Postgres has 13). The automation should detect and accommodate divergence, not enforce strict parity that doesn't exist.

Three levels of parity:

| Level | Definition | Enforce? |
|-------|-----------|----------|
| **Schema parity** | `LATEST.sql` produces the same logical schema (tables, columns, indexes) | **Yes — CI gate** |
| **File-list parity** | Migration files in corresponding directories match (same patch numbers) | **Warn only** — legitimate divergence exists |
| **Incremental path parity** | Identical SQL in each migration file | **No** — different drivers require different SQL |

---

## Detailed Implementation

### Part A: Hugo Chat Widget Fix (Hugo repo)

#### Step 10: Add `bchat.baseUrl` to `hugo.yaml`

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

#### Step 11: Update Landing Page Template

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

#### Steps 12-14: Remove Hardcoded `chatBaseUrl` from Content Files

Remove the `chatBaseUrl: "http://localhost:8081"` line from:
- `content/rgresidences/_index.md` (line 5)
- `content/bchat/_index.md`
- `content/evpn/_index.md`

After removal, the template falls through to `site.Params.bchat.baseUrl` (from hugo.yaml), which defaults to `http://localhost:8081` for local dev.

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

**Rationale:** `0.33/` is the latest migration directory. Next development cycle starts at `0.34.0`. This is a documentation convention — the auto-detection fix in bug 045 makes it non-critical, but it keeps the version string honest.

**Invariant:** `Version` and `DevVersion` must always be set to the same value. This is enforced by `bump-version.sh` (updates both) and code review.

#### Step 2: Create `scripts/bump-version.sh`

**File:** `scripts/bump-version.sh` (new)

Scans `store/migration/sqlite/` for the highest directory and patch file, computes the version as `<minor>.<patch+1>`, and updates `internal/version/version.go` via sed. Idempotent, supports `--dry-run`, no external dependencies.

Key logic:
- Find highest migration directory (e.g., `0.33/`)
- Find highest patch file number in that directory (e.g., `00` from `00__fix_max_message_length_default.sql`)
- Compute version: `0.33.1`
- Compare against current `DevVersion` in `version.go`
- Update if different

**Sed safety (from plan_review.md #2):** The sed patterns are anchored at line start to prevent substring matches:
```bash
# Anchor at line start with optional whitespace — prevents matching version
# strings in comments or unrelated constants
sed -i "s/^\tvar Version = \".*\"/\tvar Version = \"$NEW_VERSION\"/" "$VERSION_FILE"
sed -i "s/^\tvar DevVersion = \".*\"/\tvar DevVersion = \"$NEW_VERSION\"/" "$VERSION_FILE"
```

**Regex sanity check:** Before applying sed, verify the targeted line matches the expected pattern:
```bash
if ! grep -qP '^\tvar DevVersion = "[0-9]+\.[0-9]+\.[0-9]+"' "$VERSION_FILE"; then
    echo "ERROR: DevVersion line in $VERSION_FILE does not match expected format"
    echo "Expected: var DevVersion = \"X.Y.Z\""
    exit 1
fi
```

**Alternative considered (version.txt + `//go:embed`):** Cleaner, eliminates sed entirely, but requires refactoring all consumers of `Version`/`DevVersion`. Out of scope for this bug — documented as future improvement.

#### Step 3: Create `scripts/create-migration.sh`

**File:** `scripts/create-migration.sh` (new)

Creates a SQLite migration file with a TODO template, then auto-generates the Postgres equivalent by applying type substitutions from `docs/TYPE_MAPPING.md`. Developer reviews the Postgres file before committing.

Key properties:
- Validates migration name (snake_case)
- Creates SQLite file with comment header and TODO placeholder
- Auto-generates Postgres file with type substitutions when SQLite file has actual SQL
- Flags items for human review (INSERT OR IGNORE, JSON columns, boolean defaults)
- Calls `bump-version.sh` automatically
- Supports `--dry-run`

**Postgres overwrite protection (from plan_review.md #6):** Before auto-generating, check if the Postgres file already has content beyond the template boilerplate:
```bash
if [ -f "$POSTGRES_PATH" ] && [ "$(wc -c < "$POSTGRES_PATH")" -gt 150 ]; then
    echo "Postgres file already has content — skipping auto-generation"
    echo "  Review and update manually: $POSTGRES_PATH"
    exit 0
fi
```
(150 bytes covers the template header, leaving room for a few lines of SQL.)

**Handling structural divergence (from plan_review.md #1):** Some migrations require structurally different Postgres SQL (e.g., `INSERT ... ON CONFLICT DO NOTHING` instead of `INSERT OR IGNORE`, or catch-up migrations like 0.26/00, 0.30/00-04). When the Postgres file already exists with content, the script skips auto-generation and emits a message requiring manual review.

Auto-generation substitution rules (from `docs/TYPE_MAPPING.md`):

| SQLite | Postgres | Confidence |
|--------|----------|-----------|
| `BLOB` | `BYTEA` | High — direct mapping |
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` | High — direct mapping |
| `INTEGER CHECK (x IN (0,1))` | `BOOLEAN` | High — direct mapping |
| `DEFAULT (strftime('%s','now'))` | `DEFAULT EXTRACT(EPOCH FROM NOW())` | High — direct mapping |
| backtick quoting | double-quote quoting | High — direct mapping |
| Unquoted `user` | `"user"` | High — reserved word |
| `INSERT OR IGNORE` | `INSERT INTO ... ON CONFLICT DO NOTHING` | Medium — needs manual review |
| `TEXT` (for JSON data) | `JSONB` | Low — needs manual identification |
| `INTEGER` (for booleans) | `BOOLEAN` | Low — needs context to identify |

#### Step 4: Create `docs/TYPE_MAPPING.md`

**File:** `docs/TYPE_MAPPING.md` (new)

Explicit SQLite-Postgres type mapping reference. Covers:
- Type mapping table (BLOB->BYTEA, SERIAL, BOOLEAN, JSONB, TIMESTAMPTZ, etc.)
- Syntax differences (quoting, INSERT OR IGNORE, timestamp functions, reserved words)
- Migration writing rules for each driver
- Review checklist for auto-generated Postgres migrations
- Historical type differences from past migrations (0.19, 0.20, 0.22, 0.24, 0.32)

#### Step 5: Create `scripts/validate-parity.sh`

**File:** `scripts/validate-parity.sh` (new)

Cross-driver parity validator. **Two checks:**

**Check 1 — Schema parity (CI gate):** Parses both `LATEST.sql` files, extracts table names, column names per table, and index names, then compares them. Applies simplified type mapping for comparison. Fails with specific items missing in each driver.

**Check 2 — File-list parity (warn only):** Compares the set of migration files in corresponding `sqlite/<dir>/` and `postgres/<dir>/` directories. Unexpected differences warn but do not fail (to accommodate legitimate divergence like 0.33).

**SQL parsing limitations (from plan_review.md #4):** Shell awk/grep cannot parse SQL reliably — nested parentheses in `CHECK` constraints, multi-line column definitions, inline comments, and type functions in default expressions all produce false positives/negatives.

Mitigations:
1. Document parsing limitations prominently in TYPE_MAPPING.md
2. Ship the SQL-parsing validator as a **best-effort lint**, not a CI gate for schema comparison
3. Note database introspection approach (piggyback on `validate-pg-migrations.sh` which uses `information_schema`) as the definitive future enhancement
4. The file-list parity check is reliable (file existence, not SQL semantics) and serves as the practical CI gate

Key properties:
- `--verbose` mode shows per-table column counts
- Collects ALL differences before exiting (does not fail-fast on first difference)
- Exit codes: 0 = pass, 1 = schema differences, 2 = file-list differences (warn), 3 = both

#### Step 6: Add Taskfile Tasks

**File:** `Taskfile.yml`

```yaml
  version:bump:
    desc: Auto-derive version from migration filesystem and update version.go
    cmds:
      - ./scripts/bump-version.sh

  migrate:new:
    desc: Create new migration file with auto Postgres parity (usage: task migrate:new NAME=add_widget_config)
    cmds:
      - ./scripts/create-migration.sh "{{.NAME}}"

  validate:parity:
    desc: Validate SQLite and Postgres LATEST.sql schema parity
    cmds:
      - ./scripts/validate-parity.sh
```

**CI gating (from plan_review.md #5):** Add `validate:parity` as a dependency of `build:backend` (alongside existing `validate:migrations`):

```yaml
  build:backend:
    desc: Build backend binary
    deps: [validate:migrations, validate:parity]
```

This makes parity a build-time gate — if schema parity fails, the binary won't compile.

#### Step 7: Update AGENTS.md Migration Section

**File:** `AGENTS.md`

Replace the "Database Migrations" section (lines 526-540) with a quick reference pointing to the new guide, documenting `task migrate:new` as the primary workflow, and listing the three validation commands (validate-migrations.sh, validate:parity, validate:schema).

#### Step 8: Add Deprecation Notice to Old Doc

**File:** `docs/DOCS_DATABASE_MIGRATION.MD`

Add at the very top:
```markdown
> **DEPRECATED:** This document is superseded by [DOCS_DATABASE_MIGRATION_GUIDE.md](./DOCS_DATABASE_MIGRATION_GUIDE.md).
> It is retained for historical reference on the fly.io incident and LATEST.sql validation options.
> For current migration procedures, see the new guide.

---
```

#### Step 9: Create Definitive Migration Guide

**File:** `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` (new)

Comprehensive guide covering:
- Overview of the compile-time embedded migration system
- Quick reference table (all commands)
- Architecture (directory structure, version detection, migration flow)
- Step-by-step: adding a new migration (with `task migrate:new`)
- Step-by-step: adding a new version directory
- Rules and conventions (migration file rules, LATEST.sql rules, version naming)
- Cross-driver parity (three levels: schema/file-list/incremental path; type mapping; auto-generation; validation)
- **CI gates matrix** (which gate catches which historical bug):

| Historical Bug | CI Gate That Would Catch It |
|----------------|---------------------------|
| 008 — Unique constraint failure | `validate-pg-migrations.sh` (runs all migrations against real PG) |
| 009 — Migration 28 hotfix | `validate-db-migrations.sh` (applies incrementally, compares schemas) |
| 045 — Version directory skip | `version:bump` (derives version from FS, detects unsorted directories) |
| 046 — LATEST.sql drift | `validate:parity` (compares sqlite vs postgres schema) |

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
| 2 | `scripts/bump-version.sh` | Version derivation from FS | — |
| 3 | `scripts/create-migration.sh` | Migration creation + auto Postgres | 2, 4 |
| 4 | `docs/TYPE_MAPPING.md` | Type mapping reference | — |
| 5 | `scripts/validate-parity.sh` | Cross-driver parity validator | 4 |
| 6 | `Taskfile.yml` | Add tasks + CI gating | 2, 3, 5 |
| 7 | `AGENTS.md` | Update migration section | 9 |
| 8 | `docs/DOCS_DATABASE_MIGRATION.MD` | Deprecation notice | — |
| 9 | `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` | Definitive guide | 2, 3, 4, 5, 6 |
| 10 | Hugo `hugo.yaml` | Add bchat.baseUrl param | — |
| 11 | Hugo `layouts/_default/list.html` | 3-tier fallback with `or` | 10 |
| 12-14 | Hugo content files | Remove chatBaseUrl | 11 |

Steps 1-9 (bchat server) and 10-14 (Hugo site) are independent. Step 9 depends on 2, 3, 4, 5, 6.

---

## Verification

### Automation Scripts
1. `task migrate:new NAME=test_migration` — creates file, bumps version, auto-generates Postgres
2. `task version:bump` — idempotent, shows current version
3. `task validate:parity` — passes with current LATEST.sql files
4. `go build ./bin/memos/` — succeeds with new version

### Hugo Site
5. `hugo server` — loads rgresidences page, widget loads from localhost:8081 (from hugo.yaml param)
6. Template falls through correctly when chatBaseUrl is omitted from front matter
7. Template handles `chatBaseUrl: ""` correctly (falls through to site param, not empty string)
8. `warnf` fires when `bchat.baseUrl` is not set in site params

### Widget (after bug 045 migrator fix)
9. `curl http://localhost:8081/widget/rgresidences/embed.js` — returns JS
10. `PRAGMA table_info(agent_tenants)` — shows transcript_signing_key columns
11. Widget appears in browser on rgresidences landing page
