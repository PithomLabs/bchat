**Verdict: APPROVED WITH NITS** — plan3 is minimum viable and addresses the prior review's critical issues. The remaining concerns are minor and do not block execution.

---

## Verified Against Codebase

| Claim | Status |
|-------|--------|
| 34 missing tables | ✅ Confirmed (53 SQLite - 19 Postgres) |
| 68 agent stubs + 16 bridge stubs + 7 bridge_auth stubs = 91 total | ✅ Math checks out; bridge counts verified via grep |
| `memo_filter_test.go` exists in postgres | ✅ Confirmed in directory listing |
| Baseline via `CREATE TABLE IF NOT EXISTS` | ✅ No plpgsql dependency |
| Channel binding in DSN examples | ✅ Present |

---

## Nits (non-blocking)

1. **Agent stub count should be re-verified at Sprint 3 kickoff.** If the actual count deviates from 68, adjust the 3-5 day estimate proportionally. The plan should track this as a checkpoint, not a fixed assumption.

2. **60s timeout is still hardcoded.** Acceptable for MVP, but consider exposing via `profile.go` or env var in a follow-up PR so Neon cold-start tuning doesn't require a code change.

3. **Version guard implementation location is unspecified.** "In Go migration runner, not SQL" tells the implementer *what* to do but not *where*. Add a pointer to the migration runner file/function to avoid a 30-minute search at Sprint 2.3.

4. **Line counts for sqlite source files (2485, 1139, 552) may be stale.** These are from the prior review. The implementer should use the current file sizes, not the quoted numbers.

---

## What Plan3 Fixed Well

- Removed plpgsql `DO $$` dependency from baseline migration
- Moved version guard to Go layer (safer for Neon permissions)
- Added systematic `?` → `$N` translation strategy
- Explicitly marked `agent_observations.go`, `agent_workflow.go`, `memo_filter.go`, `rbac.go` as already complete
- Fixed duplicate `agent_workflows` table entry
- Added `channel_binding=require` to all DSN examples
- Added `lib/pq` removal verification steps

---

**You are cleared to instruct the coding agent to implement plan3.md as written.** The nits above can be handled inline during implementation without requiring a plan revision.
