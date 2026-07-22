# Adversarial Plan Review — Bug 046 (plan4.md)

**Reviewer:** AI Architect
**Plan under review:** `plan4.md`
**Status:** ✅ APPROVED (1 pre-existing nit)

---

## Verdict

**Approved for coding.** All 11 findings from both prior review cycles are fully resolved:

| Review 1 Finding | Review 2 Finding | plan4 Resolution |
|---|---|---|
| C1 — Postgres 0.33 divergence | — | Check 3 in `validate-parity.sh` lists known divergences and skips them |
| C2 — Sed version fragility | H1 — Sed-based versioning is wrong abstraction | `bump-version.sh` is now read-only informational; version derived at runtime from embedded FS |
| C3 — Hugo `default` on empty string | — | `or` pipe replaces `default` |
| M4 — Shell SQL parsing | — | Schema parity = best-effort lint (warns only); file-list parity = CI gate (fails) |
| M5 — No CI gating | — | `validate:parity` + `test:scripts` added to `build:backend` deps |
| L6 — Postgres overwrite | H2 — Auto-generation false confidence | Removed auto-gen entirely; `create-migration.sh` creates TODO templates only |
| — | H3 — Zero script tests | `scripts/test/` directory with fixtures + `task test:scripts` |
| — | M4 — No rollback strategy | Rollback contract documented in migration guide (additive-safe, DML-not-safe) |
| — | L5 — Hugo warnf insufficient | CI grep gate on deploy pipeline fails if `bchat.baseUrl` missing |

---

## One Pre-Existing Nit (Not Blocking)

The existing test at `store/test/migrator_test.go:18` asserts:
```go
require.Contains(t, currentSchemaVersion, "0.31.", "schema version should be 0.31.x")
```

After bug 045 makes `GetCurrentSchemaVersion()` FS-derived, this will return `"0.33.1"` (latest FS migration) regardless of the `DevVersion` bump to `"0.34.0"`. The test must be updated to assert `"0.33."` instead of `"0.31."`.

This is likely part of bug 045's scope (the fix changes the version derivation), but plan4 should explicitly mention it in the verification steps or as a note on Step 1. The coding agent will discover it on first `go test ./...` run — low risk, just a heads-up.

---

## Summary for Coding Agent

Proceed with implementation in the specified dependency order. The three architectural pivots from prior plans (runtime versioning, no auto-gen, test fixtures) are all correctly captured. No further design changes needed.
