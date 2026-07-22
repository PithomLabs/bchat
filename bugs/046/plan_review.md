# Adversarial Plan Review — Bug 046

**Reviewer:** AI Architect  
**Plan under review:** `plan2.md` (supersedes `plan.md`)  
**Status:** ⚠️ REWORK (3 critical, 2 moderate, 1 low issue)

---

## Verdict

**Rework recommended on plan2.md before implementation.** Three critical issues (Parity Validator assumptions, version bump fragility, Hugo `default` pipe semantics) will produce incorrect behavior or silent failures if implemented as written. Two moderate issues (SQL parsing approach, no CI gating) will erode trust in the automation over time. One low issue (Postgres auto-generation overwrite) is an ergonomic concern.

---

## CRITICAL Issues

### 1. Postgres 0.33 Schema Divergence (Parity Validator Blind Spot)

**Plan2 asserts:** "Auto-generates the Postgres equivalent by applying type substitutions." This assumes the per-directory migration files are structurally parallel — but they are not today.

**Current reality:**
- `store/migration/sqlite/0.33/00__fix_max_message_length_default.sql` — UPDATE statement, no schema change
- `store/migration/postgres/0.33/00__add_system_secret.sql` — CREATE TABLE for `system_secret`, adds a persistent table

These are entirely different operations with different semantics. A parity validator that only compares LATEST.sql table/column structure will PASS today (both LATEST files already have `system_secret`). But the validator won't detect that the Postgres incremental migration path has a table that the SQLite incremental path lacks. More problematically, `create-migration.sh`'s auto-generation script will produce wrong output or silently overwrite intentionally divergent Postgres files.

**Required rework:**
- Document Postgres 0.33 divergence explicitly in the migration guide as an intentional historical case
- Add a check in `validate-parity.sh` that compares the *set of migration files* (not just LATEST.sql structure): every file in `sqlite/<dir>/` should have a corresponding `postgres/<dir>/` file, and vice versa, with the same patch number. Unexpected differences should warn, not fail (to accommodate legitimate divergence)
- In `create-migration.sh`, when the Postgres file already exists with content beyond the template, skip auto-generation and emit a message: `"Postgres file exists — manual review required"`

### 2. `bump-version.sh` sed Patterns Are Fragile

**Plan states:** "Uses sed to update version.go. Idempotent, supports --dry-run, no external dependencies."

**Problems:**
- The script reads `DevVersion` with `grep -oP`, then uses that value in two `sed -i` substitutions to update both `Version` and `DevVersion`. If these two constants are ever different (which is a legitimate state — `Version` tracks released, `DevVersion` tracks development), the first sed would replace `Version`'s value with the new version, and the second sed would then fail to find the original `CURRENT_VERSION` in `DevVersion` (or worse, match a substring and corrupt a comment or unrelated constant).
- File format: `version.go` uses simple `var Version = "X.Y.Z"` format now, but a future edit that adds an inline comment (e.g., `var Version = "0.31.0" // TODO: bump`) will cause the regex to match the wrong thing or fail.
- The grep-to-sed round-trip interprets `$CURRENT_VERSION` as a literal string in the sed pattern, but if the version ever contains characters with sed special meaning (it shouldn't per semver, but defense in depth), it breaks silently.

**Required rework:**
- Both `Version` and `DevVersion` must always be set to the same value (enforce in code review or add a CI check)
- Anchor the sed patterns at line start with optional whitespace: `s/^\tvar DevVersion = ".*"/\tvar DevVersion = "NEW.VERSION"/`
- Alternatively, define the version in a separate `version.txt` file read at build time and embedded with `//go:embed` — this eliminates sed fragility entirely
- At minimum, add a regex sanity check that the targeted line actually contains the expected pattern before applying sed

### 3. Hugo `default` Pipe Does Not Fall Through on Empty String

**Plan states:** `{{ $chatUrl := .Params.chatBaseUrl | default site.Params.bchat.baseUrl | default "http://localhost:8081" }}`

**Problem:** Hugo's `default` function checks whether the piped value is `nil` (undefined). An empty string `""` is a defined, non-nil Go string, so `default` will NOT fall through. If a developer accidentally sets `chatBaseUrl: ""` in front matter (easy copy-paste error or forgetting to remove the key), the widget initialization receives `""` instead of falling through the chain, and the embed script loads from `https://embed.js` (relative URL root) instead of the bchat server.

**Required rework:**
Replace the `default`-chain with `or`, which checks both `nil` and the zero value (empty string, 0, false):
```
{{ $chatUrl := or .Params.chatBaseUrl site.Params.bchat.baseUrl "http://localhost:8081" }}
```

In Hugo, `or` returns the first non-zero (non-nil, non-empty) value. This also simplifies the chain — a single `or` with three arguments is equivalent to the three nested `default` calls but handles empty strings correctly.

Also consider adding a Hugo template warning:
```
{{ if not site.Params.bchat.baseUrl }}
  {{ warnf "WARNING: bchat.baseUrl not set in site params. Widget will use localhost fallback." }}
{{ end }}
```
Production sites can use `--templateMetrics` to catch this during CI.

---

## MODERATE Issues

### 4. Parity Validator Uses Shell-Level SQL Parsing

**Plan states:** "Parses `CREATE TABLE` and `CREATE INDEX` from both LATEST.sql files. Applies simplified type mapping."

**Problem:** SQL has:
- Nested parentheses in `CHECK` constraints (e.g., `CHECK((status = 'active' AND ...) OR (status = 'revoked' AND ...))`)
- Multi-line column definitions spanning 5+ lines
- Inline comments (`--` and `/* */`) that can appear mid-statement
- Type functions in default expressions (`EXTRACT(EPOCH FROM NOW())`, `CAST(strftime('%s','now') AS INTEGER)`)
- CTE-style subqueries in index conditions (`WHERE beads_id IS NOT NULL`)

Shell awk/grep cannot parse this reliably. The validator will produce both false positives (flags a "difference" that is just comment placement or whitespace) and false negatives (misses real schema drift because its column extractor doesn't understand the SQL structure).

**Required rework:** 
The existing `validate-pg-migrations.sh` already solves this correctly by:
1. Running each LATEST.sql against a real Postgres database
2. Reading the actual schema from `information_schema` (definitive source of truth)
3. Comparing table lists

Piggyback on this approach: run `validate-db-migrations.sh` (SQLite) and `validate-pg-migrations.sh` (Postgres) as separate steps, then compare table+column sets from `information_schema` (Postgres) vs `PRAGMA table_info` (SQLite). This produces a definitive comparison using database introspection, not text parsing. The shell script's job is just to coordinate the two runs and diff the structured output.

If a full DB-backed validator is too heavy for day-to-day use, then at minimum document the parsing limitations prominently in TYPE_MAPPING.md and ship the SQL-parsing validator as a best-effort lint, not a CI gate.

### 5. No CI Gating for Automation Scripts

**Plan states:** Scripts are created and documented, but no CI integration is specified. Without enforcement, these scripts are helpful hints that can be skipped. The same drift patterns that caused bugs 008, 009, 015, 045, and 046 will recur.

**Required rework:**
- Add `task validate:parity` as a dependency of `build:backend` (alongside the existing `validate:migrations`). This makes it a build-time gate — if the parity check fails, the binary won't compile.
- Document the CI-hardening in the migration guide. Name the specific CI checks that catch each historical bug pattern:

| Historical Bug | CI Gate That Would Catch It |
|----------------|---------------------------|
| 008 — Unique constraint failure | `validate-pg-migrations.sh` (runs all migrations against real PG) |
| 009 — Migration 28 hotfix | `validate-db-migrations.sh` (applies incrementally, compares schemas) |
| 045 — Version directory skip | `version:bump` (derives version from FS, detects unsorted directories) |
| 046 — LATEST.sql drift | `validate:parity` (compares sqlite vs postgres schema) |

---

## LOW Issue

### 6. Postgres Auto-Generation Overwrite

**Plan states:** `create-migration.sh` "auto-generates the Postgres equivalent by applying type substitutions."

**Problem:** The plan says "developer reviews the Postgres file before committing," but the script itself has no defense against accidentally overwriting a manually-crafted Postgres file. The workflow is: write SQLite → run script → script creates both files → edit Postgres differently → run script again for next migration → script overwrites Postgres edits.

**Required rework:**
Before auto-generating, check if `postgres/<version>/<filename>` already exists and has content beyond the template boilerplate:
```
if [ -f "$POSTGRES_PATH" ] && [ "$(wc -c < "$POSTGRES_PATH")" -gt 150 ]; then
    echo "Postgres file already has content — skipping auto-generation"
    echo "  Review and update manually: $POSTGRES_PATH"
    exit 0
fi
```
(150 bytes covers the template header, leaving room for a few lines of SQL.)

---

## Summary of Required Changes

| # | Severity | Area | Change Required |
|---|----------|------|-----------------|
| 1 | Critical | Parity Validator | Handle existing Postgres-0.33 divergence; validate file-list parity, not just schema; skip auto-generation when Postgres file has content |
| 2 | Critical | Version Bump | Anchor sed patterns at line start; or switch to version.txt with `//go:embed` |
| 3 | Critical | Hugo Template | Replace `default` chain with `or`; add `warnf` for missing production config |
| 4 | Moderate | SQL Parsing | Use database introspection (like `validate-pg-migrations.sh`) instead of shell awk/grep |
| 5 | Moderate | CI Gating | Add `validate:parity` to `build:backend` deps; document CI-checks matrix |
| 6 | Low | Auto-Generation | Check for existing Postgres content before overwriting |

---

## Design Committee Guidance (If Applicable)

The three critical issues share a root cause: **assuming the two drivers will always be structurally parallel**. History shows they have already diverged (0.33), and the incremental migration paths are heavily different (SQLite has 30+ directories, Postgres has 13). The automation should be designed to detect and accommodate divergence, not to enforce strict parity that doesn't exist. Consider:

- **Schema parity = LATEST.sql must produce the same logical schema** (tables, columns, indexes)
- **Migration file parity = file lists in corresponding directories should match, but the specific SQL can differ**
- **Incremental path parity = not guaranteed; the validator should warn but not fail on incremental differences**

This distinction should be documented in both `TYPE_MAPPING.md` and the migration guide.
