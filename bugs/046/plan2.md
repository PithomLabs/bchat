# Bug 046: Chat Widget Not Appearing — Hugo Defaults + Migration Automation + Parity

**Status:** PLANNED (supersedes plan.md)
**Date:** 2026-07-23
**Depends on:** Bug 045 (migrator.go fix for `GetCurrentSchemaVersion()`)
**Affected repos:** bchat server (`/home/chaschel/Documents/go/bchat`), Hugo site (`/home/chaschel/Documents/go/izaakmaine.github.io-main`)

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
{{ $chatUrl := .Params.chatBaseUrl | default site.Params.bchat.baseUrl | default "http://localhost:8081" }}
```

**Three-tier fallback:**
1. Page-level `chatBaseUrl` in front matter (highest priority — per-tenant overrides)
2. `site.Params.bchat.baseUrl` from `hugo.yaml` (configurable per environment)
3. Hardcoded `"http://localhost:8081"` (safe default — never produces a broken production URL)

**Security rationale:** The hardcoded fallback is localhost. A misconfigured deployment will fail locally (developer catches it), not in production (where it would expose an internal URL or fail silently).

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

#### Step 2: Create `scripts/bump-version.sh`

**File:** `scripts/bump-version.sh` (new)

Scans `store/migration/sqlite/` for the highest directory and patch file, computes the version as `<minor>.<patch+1>`, and updates `internal/version/version.go` via sed. Idempotent, supports `--dry-run`, no external dependencies.

Key logic:
- Find highest migration directory (e.g., `0.33/`)
- Find highest patch file number in that directory (e.g., `00` from `00__fix_max_message_length_default.sql`)
- Compute version: `0.33.1`
- Compare against current `DevVersion` in `version.go`
- Update if different

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

Cross-driver parity validator. Parses both `LATEST.sql` files, extracts table names, column names per table, and index names, then compares them. Applies simplified type mapping for comparison. Fails with specific items missing in each driver.

Key properties:
- Parses `CREATE TABLE` and `CREATE INDEX` from both LATEST.sql files
- Compares table names, column names per table, and index names
- Applies simplified type mapping for comparison (BLOB->BYTEA, etc.)
- Fails with specific items missing in each driver
- `--verbose` mode shows per-table column counts
- Limitations: compares structure, not SQL semantics or constraints

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
- Cross-driver parity (type mapping, auto-generation, validation)
- Gotchas and known issues (tests always use LATEST.sql, DDL transactionality, WAL visibility, go:embed rebuild, dual version tracking, LATEST.sql drift)
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
| 6 | `Taskfile.yml` | Add tasks | 2, 3, 5 |
| 7 | `AGENTS.md` | Update migration section | 9 |
| 8 | `docs/DOCS_DATABASE_MIGRATION.MD` | Deprecation notice | — |
| 9 | `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` | Definitive guide | 2, 3, 4, 5, 6 |
| 10 | Hugo `hugo.yaml` | Add bchat.baseUrl param | — |
| 11 | Hugo `layouts/_default/list.html` | 3-tier fallback | 10 |
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

### Widget (after bug 045 migrator fix)
7. `curl http://localhost:8081/widget/rgresidences/embed.js` — returns JS
8. `PRAGMA table_info(agent_tenants)` — shows transcript_signing_key columns
9. Widget appears in browser on rgresidences landing page

---

## Adversarial Review Prompt

> Review this plan for Bug 046 (chat widget not appearing + migration automation + parity) with an adversarial mindset. Focus on:
>
> ### Hugo Template Correctness
> 1. The 3-tier fallback `{{ .Params.chatBaseUrl | default site.Params.bchat.baseUrl | default "http://localhost:8081" }}` — does Hugo's `| default` pipe treat empty strings as falsy? What if someone sets `chatBaseUrl: ""` in front matter? Does it fall through or use the empty string?
> 2. Should there be a Hugo build-time warning if no bchat.baseUrl is configured and the site is being built for production?
>
> ### Security
> 3. The hardcoded fallback is `http://localhost:8081`. Is this the right security posture? What if a production deployment accidentally omits the hugo.yaml override — the widget would try to load from localhost (fails silently, which is safe but confusing). Should we log a warning?
> 4. Are there any CSP implications of loading a script from a configurable URL?
>
> ### Version Bump Script
> 5. `bump-version.sh` uses `sed -i` to update `version.go`. What if the file format changes (e.g., someone adds a comment containing the version string)? Is the regex specific enough? Should we use a more structured approach?
> 6. What if `version.go` has both `Version` and `DevVersion` set to different values — does the script handle this correctly?
>
> ### Migration Script
> 7. If two developers run `task migrate:new` simultaneously, they could get the same patch number. How likely is this in practice? Is the collision window acceptable for a solo/small-team project?
> 8. The script auto-generates the Postgres file from the SQLite file. But some past migrations (0.26/00, 0.30/00-04) required structurally different Postgres SQL (catch-up migrations). How does the script handle this edge case? Does it detect and skip if the developer has already manually edited the Postgres file?
> 9. The auto-generation uses sed substitutions. What about edge cases where `BLOB` appears inside a comment or string literal? Is the word-boundary matching (`\b`) sufficient?
>
> ### Parity Validator
> 10. `validate-parity.sh` uses awk to extract columns from CREATE TABLE blocks. How robust is this against SQL with multi-line constraints, inline comments, or nested parentheses? Could it produce false positives/negatives?
> 11. The validator compares table/column/index names but not constraints (NOT NULL, DEFAULT, CHECK, FOREIGN KEY). Is this a significant gap? Should constraints be compared?
> 12. What happens if one LATEST.sql has a table that the other doesn't — does the script exit immediately or collect all differences?
>
> ### LATEST.sql Update
> 13. The plan says "update LATEST.sql" as a manual step in adding a migration. This is the same manual step that caused bugs 008, 009, 015, 045. Should `create-migration.sh` auto-update LATEST.sql for simple cases (ALTER TABLE ADD COLUMN)?
> 14. Should `validate-parity.sh` be a build dependency (like `validate:migrations.sh` is)? Or should it be a separate CI step?
>
> ### Migration Guide
> 15. Does the guide cover all gotchas from past bugs? Is there anything from bugs 008, 009, 015 that should be explicitly called out?
> 16. Should the guide include a "first time setup" section for new developers joining the project?
>
> ### Order of Operations
> 17. The plan says bug 045's migrator.go fix must be applied for the widget to work. Should the plan explicitly gate Steps 10-14 (Hugo changes) on bug 045 being deployed first? Or can they be deployed independently?
> 18. For the bchat server changes, what is the recommended order? Should the version bump (Step 1) happen first, or can the scripts be created first?
>
> ### Testing the Automation
> 19. How do we test that `bump-version.sh` and `create-migration.sh` work correctly? Should there be a test or CI step for the scripts themselves?
> 20. Should `validate-parity.sh` have a test fixture (known-good LATEST.sql pair) to verify it catches intentional drift?
>
> Provide specific recommendations to strengthen the plan.
