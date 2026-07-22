# Adversarial Code Review — Bug 046 Implementation

**Reviewer:** AI Architect  
**Implementation Record:** `bugs/046/code.md`  
**Files reviewed:** All 16 files created/modified across both repos  
**Status:** ✅ APPROVED WITH NITS

---

## Verdict

**Safe to ship.** All 6 design-level findings from the plan review cycles were correctly implemented. Two high-severity portability issues exist (GNU-specific features in scripts) but will not affect the primary deployment target (Linux/Fly.io). Medium-severity findings (dead code, test structure) should be addressed post-ship.

---

## High Severity

### H1. `declare -A` Requires Bash 4+ — Breaks on macOS and Older Linux

**File:** `scripts/validate-parity.sh:37`

```bash
declare -A KNOWN_DIVERGENCES=(
    ["0.2"]="SQLite-only early migration"
    ...
)
```

**Problem:** `declare -A` is a bash 4.0+ feature. macOS ships bash 3.2 (GPLv2 licensing freeze). Many CI environments also run bash 3.x. With `set -euo pipefail`, the script will abort at line 37 with `declare: -A: invalid option` when run on these systems.

**Fix:** Replace the associative array with a newline-separated list and a simple grep check:

```bash
KNOWN_DIVERGENCES=$(cat << 'EOF'
0.2:SQLite-only early migration
0.3:SQLite-only early migration
0.4:SQLite-only early migration
...
0.33:Postgres has system_secret table; SQLite has different migration
EOF
)

# Check at line 82:
if echo "$KNOWN_DIVERGENCES" | grep -q "^$dirname:"; then
    reason=$(echo "$KNOWN_DIVERGENCES" | grep "^$dirname:" | sed 's/^[^:]*://')
    if [ "$VERBOSE" = true ]; then
        echo "  SKIP $dirname (known divergence: $reason)"
    fi
    continue
fi
```

This is portable to any POSIX shell (sh, bash 3+, dash, etc.) and preserves the reason text for verbose mode.

### H2. `grep -P` Is GNU-Only — Not Available on macOS

**Problem:** Three files use `grep -P` (Perl-compatible regex) which requires GNU grep. macOS ships BSD grep which does not support `-P`.

| File | Line | Pattern | Portable Alternative |
|------|------|---------|---------------------|
| `scripts/bump-version.sh` | 44 | `grep -oP '^\d+'` | `grep -oE '^[0-9]+'` |
| `scripts/bump-version.sh` | 54 | `grep -oP 'var DevVersion = "\K[^"]+'` | `sed -n 's/.*var DevVersion = "\([^"]*\)".*/\1/p'` |
| `scripts/validate-parity.sh` | 154 | `grep -oP 'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["`]?(\w+)["`]?'` | `grep -oE 'CREATE[[:space:]]+TABLE[[:space:]]+(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?["`'"'"']?(\w+)["`'"'"']?'` |
| `scripts/validate-parity.sh` | 187 | `grep -oP 'CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?["`]?(\w+)["`]?'` | same approach |

Line 54 is the trickiest because `\K` (lookbehind reset) is Perl-specific. The `sed` replacement works on both GNU sed and BSD sed:

```bash
# Portable replacement for line 54:
get_current_version() {
    sed -n 's/.*var DevVersion = "\([^"]*\)".*/\1/p' "$VERSION_FILE" 2>/dev/null || echo "unknown"
}
```

The `-E` flag for extended regex is supported by both GNU grep and BSD grep (though BSD grep calls it "modern" regex — `-E` is POSIX-standard).

---

## Medium Severity

### M1. `extract_columns` Is Dead Code

**File:** `scripts/validate-parity.sh:161-182`

A 22-line `awk` function is defined but never called anywhere in the script. The schema parity check only compares table names (`extract_tables`) and index names (`extract_indexes`) — NOT column types, constraints, or foreign keys.

**Problem:** A reader or future developer will assume column-level comparison is happening. The dead code is misleading. More importantly, the actual gap (no column-type comparison) is invisible — two LATEST.sql files could have the same table names with different column definitions and the validator would pass.

**Fix:** Either:
1. Remove the dead code and add a comment about the limitation
2. OR actually integrate it into the schema comparison (higher effort, but more complete)

Option 1 is preferred for this bug's scope. The schema check should remain best-effort lint. Document the column-type gap in TYPE_MAPPING.md:

> **Known limitation:** The parity validator compares table names and index names. Column types, constraints, NOT NULL, and foreign keys are NOT compared. Two LATEST.sql files with the same table names but different column definitions will pass this check. Manual review is required for type-level parity.

### M2. Test Duplicates Regex Instead of Running the Validator

**File:** `scripts/test/run-tests.sh:95-107`

Test 4 (schema comparison) duplicates the exact `grep -oP` + `sed` + `sort -u` pipeline from `validate-parity.sh` instead of running the actual script:

```bash
# Lines 95-96 duplicate validate-parity.sh lines 154-158
SQLITE_TABLES=$(cd "$TMPDIR2" && grep -oP 'CREATE\s+TABLE\s+...' ...)
POSTGRES_TABLES=$(cd "$TMPDIR2" && grep -oP 'CREATE\s+TABLE\s+...' ...)
```

**Problem:** If the regex in `validate-parity.sh` is changed but the test regex is not, the test still passes. The test tests itself, not the validator.

**Fix:** Run `validate-parity.sh` against the fixture directory and assert exit code:

```bash
# Test 4: validate-parity.sh passes on known-good pair
run_cmd bash "$REPO_ROOT/scripts/validate-parity.sh"
assert_exit "validate-parity.sh passes on known-good schema" 0 $EXIT_CODE
```

And for Test 5 (drift detection):
```bash
# Test 5: validate-parity.sh detects drift
run_cmd bash "$REPO_ROOT/scripts/validate-parity.sh"
assert_exit "validate-parity.sh warns on schema drift" 2 $EXIT_CODE
```

The fixture directory structure (`$TMPDIR2`) already has the correct layout. The only missing piece is that `validate-parity.sh` would need to be invoked from within the fixture directory or given the fixture path. Either symlink the fixture directory structure to match the expected layout, or add a `--fixtures-dir` flag to the validator.

---

## Low Severity

### L1. `sort -V` Is GNU-Only

**Files:** `scripts/bump-version.sh:28`, `scripts/create-migration.sh:61`

`sort -V` (version sort) is not available on macOS's BSD sort. This affects the version comparison logic for finding the latest migration directory.

The current pattern:
```bash
printf '%s\n' "$latest" "$dirname" | sort -V | tail -1 | grep -q "$dirname"
```

**Fix for portability:** Replace with a bash-native version comparison that doesn't require `sort -V`:

```bash
# Portable version comparison for "0.NN" format
version_gt() {
    local v1=$(echo "$1" | sed 's/^0\.//')
    local v2=$(echo "$2" | sed 's/^0\.//')
    [ "$v1" -gt "$v2" ] 2>/dev/null
}
```

Then replace the `printf | sort -V | tail -1 | grep -q` pattern with:
```bash
if [ -z "$latest" ] || version_gt "$dirname" "$latest"; then
    latest="$dirname"
fi
```

### L2. `build:backend:rag` Bypasses Parity and Script Tests

**File:** `Taskfile.yml:75`

```yaml
build:backend:rag:
    deps: [build:frontend, setup:lancedb, validate:migrations]
```

The RAG build path does NOT include `validate:parity` or `test:scripts` as dependencies. Only `task build:backend` enforces these checks. A developer who builds with `task build:backend:rag` (a common production build path) bypasses the parity gate and script tests.

**Fix:** Add both dependencies:
```yaml
build:backend:rag:
    deps: [build:frontend, setup:lancedb, validate:migrations, validate:parity, test:scripts]
```

### L3. No Bash Version Guard

None of the scripts check the bash version before using bash 4+ features. A developer on macOS (or any system with bash < 4) running `task migrate:new` will get a confusing error: `declare: -A: invalid option` instead of a clear message.

Add at the top of `validate-parity.sh`:
```bash
if [ -z "${BASH_VERSINFO:-}" ] || [ "${BASH_VERSINFO:-0}" -lt 4 ]; then
    echo "ERROR: validate-parity.sh requires bash 4.0+ (found: ${BASH_VERSION:-unknown})"
    echo "  On macOS: brew install bash"
    exit 1
fi
```

---

## Summary

| ID | Severity | Area | Action |
|----|----------|------|--------|
| H1 | High | `validate-parity.sh:37` | Replace `declare -A` with portable grep on heredoc list |
| H2 | High | `bump-version.sh:44,54` + `validate-parity.sh:154,187` | Replace `grep -P` with `grep -E` or `sed` |
| M1 | Medium | `validate-parity.sh:161-182` | Remove dead `extract_columns` or integrate it |
| M2 | Medium | `run-tests.sh:95-107` | Run `validate-parity.sh` directly instead of duplicating regex |
| L1 | Low | `bump-version.sh:28`, `create-migration.sh:61` | Replace `sort -V` with bash-native version comparison |
| L2 | Low | `Taskfile.yml:75` | Add `validate:parity` + `test:scripts` to `build:backend:rag` deps |
| L3 | Low | `validate-parity.sh` (top) | Add bash 4+ version guard |

**Deployment note:** If the codebase targets only Linux (Fly.io deployment), H1 and H2 are non-blocking — GNU grep and bash 4+ are present on all modern Linux distributions. The macOS/portability issues only matter for local development. Consider adding a `scripts/check-dependencies.sh` that verifies toolchain requirements and is called by development tasks.
