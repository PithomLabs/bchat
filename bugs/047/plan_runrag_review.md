# Adversarial Plan Review — plan_runrag.md (Parity Validation Fix)

**Reviewer:** Senior Go Architect (automated)
**Date:** 2026-07-23
**Scope:** Fix `validate:parity` exit status 3 blocking `task run:rag`

---

## Verdict: APPROVED — No Issues

Root cause analysis is correct, both proposed files are syntactically and semantically correct, and the fix fully resolves the parity validation failure. No nits, no rework needed.

---

## Verified Correct

| Claim | Status | Evidence |
|-------|--------|----------|
| Error chain: `run:rag` → `build:backend:rag` → `validate:parity` | ✅ | `Taskfile.yml:122` (run:rag deps: build:backend:rag), `Taskfile.yml:75` (build:backend:rag deps: validate:parity) |
| Parity exit code 3 = file-list + schema mismatch | ✅ | `scripts/validate-parity.sh:251`: `EXIT_CODE=$((FILE_LIST_ISSUES * 1 + SCHEMA_ISSUES * 2))` |
| SQLite has `0.34/00__add_user_access_token_lookup.sql` | ✅ | File exists in `store/migration/sqlite/0.34/` |
| Postgres has NO `0.34/` directory | ✅ | Postgres migration dirs end at `0.33/` |
| `0.34` NOT in known divergences list | ✅ | `validate-parity.sh:44-68` — known list ends at `0.33` |
| Postgres LATEST.sql missing `user_access_token_lookup` | ✅ | LATEST.sql ends at agent_events (line 1015) |
| SQLite LATEST.sql has `user_access_token_lookup` | ✅ | Appended after agent_events block |
| FK `user` column exists in Postgres | ✅ | Postgres LATEST.sql uses `"user"` throughout |
| Proposed Postgres SQL syntax correct | ✅ | `EXTRACT(EPOCH FROM NOW())::BIGINT` matches existing agent_events default; `"user"` quoted reserved word |
| `00__` naming matches SQLite and Postgres convention | ✅ | Both `sqlite/0.34/` and `postgres/0.33/` use `00__` prefix for single-file batches |

---

## Minor Observation (Not a Nit)

The plan says: "LATEST.sql uses plain `CREATE TABLE` (no `IF NOT EXISTS`) — consistent with all other tables in the file."

This is **almost** correct — `migration_history` at `postgres/LATEST.sql:2` does use `CREATE TABLE IF NOT EXISTS`. The overwhelmingly dominant pattern in the file is plain `CREATE TABLE` (every table after `migration_history`), so the proposed table correctly follows the majority convention. Since `LATEST.sql` runs once per fresh install, the difference is academic. Observation only — not worth flagging as a nit.

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| Postgres LATEST.sql line insertion point wrong | Low | Medium | The plan says "after agent_events block, before end of file" — verified correct (line 1015 is last table block) |
| `"user"` quoting wrong | None | — | Verified against existing LATEST.sql: all user FK references use `"user"` |
| `EXTRACT(EPOCH FROM NOW())::BIGINT` not matching existing | None | — | Identical to `agent_events.created_at` default at LATEST.sql:1011 |
| `00__` collides with future Postgres migration | Low | Low | Same file naming is already established by `postgres/0.33/` and `sqlite/0.34/` |
| Parity check regex won't match | Low | Low | `validate-parity.sh:172` regex captures `CREATE TABLE user_access_token_lookup` and `CREATE TABLE IF NOT EXISTS` patterns correctly |

---

## Summary

The plan is correct, minimal, and production-ready. Two files, seven lines of SQL each, one simple table definition. No architectural concerns, no edge cases missed.

**Recommendation:** Proceed with implementation.

## Definition of Done (Copied from plan for reference)

- [ ] `store/migration/postgres/0.34/00__add_user_access_token_lookup.sql` created
- [ ] `store/migration/postgres/LATEST.sql` updated with table + index
- [ ] `./scripts/validate-parity.sh` exits 0
- [ ] `task build:backend:rag` succeeds
- [ ] `task run:rag` starts successfully
