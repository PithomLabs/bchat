**Verdict: APPROVED WITH NITS** — plan4.md is the strongest version yet. It adds critical SQL translation rules missing from prompt2.md and fixes the test file path. The remaining issues are minor.

---

## What plan4.md Fixed (vs prompt2.md)

| Issue | prompt2.md | plan4.md |
|-------|-----------|----------|
| Test file path | `store/db/postgres/bridge_auth_test.go` (wrong) | `store/test/bridge_auth_test.go` (correct) |
| TIMESTAMP translation | Missing | `TIMESTAMP DEFAULT CURRENT_TIMESTAMP` → `TIMESTAMPTZ DEFAULT NOW()` |
| INSERT OR IGNORE | Missing | `INSERT OR IGNORE` → `INSERT ... ON CONFLICT DO NOTHING` |
| Partial indexes | Not mentioned | Confirmed Postgres supports `WHERE ...` on indexes |
| grep errNotImplemented | Returns false positive on var declaration | Excludes `var errNotImplemented` line |

---

## Verified Against Codebase

| Claim | Status |
|-------|--------|
| Test file at `store/test/bridge_auth_test.go` | ✅ Confirmed |
| `var errNotImplemented` declared in `agent.go:16` | ✅ Confirmed — grep exclusion needed |
| `TIMESTAMP DEFAULT CURRENT_TIMESTAMP` in SQLite schema | ✅ Confirmed (lines 202, 203, 225, 353, etc.) |
| `INSERT OR IGNORE` in SQLite schema | ✅ Confirmed (line 413, `tenant_role_templates` seed) |
| Partial indexes with `WHERE` clause | ✅ Confirmed (lines 170, 173, 802) |
| Existing Postgres uses `TIMESTAMPTZ DEFAULT NOW()` | ✅ Confirmed (lines 144, 145, 164, etc.) |

---

## Must Fix Before Execution

### 1. Test removal is a no-op

Both plan4.md and prompt2.md instruct removing `TestBridgeAuthPostgresUnsupported`. **This test does not exist in the codebase.** Verified via `grep -rn` across all `.go` files.

The test removal steps in Sprint 3 and Sprint 5 are dead instructions. The coding agent will search for the test, not find it, and waste time wondering if it's in the wrong location.

**Fix:** Remove the test removal instructions from both sprints. If the test is created during implementation (unlikely), it should be deleted — but the current instruction is noise.

### 2. `INSERT OR IGNORE` context is wrong

plan4.md says: `INSERT OR IGNORE` → `INSERT ... ON CONFLICT DO NOTHING` (seed data only)

The only occurrence of `INSERT OR IGNORE` is in the baseline seed for `tenant_role_templates` (line 413 of SQLite LATEST.sql). This is seed data that runs once during `preMigrate()`. The new baseline migration for Postgres will have its own seed data. The agent doesn't need to translate this — it writes new Postgres SQL, not converts existing SQLite queries.

**Fix:** Remove this translation rule or clarify it applies to `LATEST.sql` seed data only.

---

## Nits

| # | Issue | Location | Severity |
|---|-------|----------|----------|
| 1 | `TIMESTAMP DEFAULT CURRENT_TIMESTAMP` translation is for LATEST.sql schema only, not for Go queries. Go queries use `time.Now().Unix()` which works for both drivers. Clarify scope. | Sprint 2 | Low |
| 2 | Sprint 5 Step 1 says "Remove unused `postgres` import" from `store/test/bridge_auth_test.go`. If the test doesn't exist, this is also a no-op. | Sprint 5.1 | Low |
| 3 | The `go list -m all \| grep lib/pq` verification should be `go list -m all 2>&1 \| grep lib/pq` to handle stderr. | Sprint 1.2 | Low |
| 4 | Sprint 6 doesn't specify which Fly.io app name to use. If the app doesn't exist yet, `fly deploy` will fail. Should add `fly apps list` or `fly launch` step. | Sprint 6 | Medium |
| 5 | The `Do NOT` section says "Create a new driver switch in `store/db/db.go`" but doesn't verify the existing switch actually works. Should add a `go build` check after Sprint 4 to catch missing imports or type mismatches. | Sprint 4 | Low |

---

## Structural Notes

1. **plan4.md is more correct than prompt2.md.** The test file path fix alone prevents a build failure. The TIMESTAMP and INSERT OR IGNORE rules prevent schema migration errors.

2. **Both documents are over-specified for Sprint 6.** Neon setup and Fly.io deployment are operational tasks, not code changes. The coding agent doesn't need step-by-step Neon console instructions. Consider moving Sprint 6 to a separate operational runbook (which `neon_deploy.md` already covers).

3. **The translation rules are now comprehensive.** Between plan3.md, prompt2.md, and plan4.md, the SQL translation surface is fully covered. No additional rules are needed.

---

## Recommended Corrections

1. Remove `TestBridgeAuthPostgresUnsupported` removal instructions from Sprint 3 and Sprint 5 (test doesn't exist)
2. Remove or clarify `INSERT OR IGNORE` rule (only applies to LATEST.sql seed, not Go queries)
3. Add `fly apps list` to Sprint 6 before `fly deploy`
4. Add `go build ./...` verification step after Sprint 4

**Do not proceed to execution until item 1 is corrected.** The dead test removal instructions will confuse the coding agent.
