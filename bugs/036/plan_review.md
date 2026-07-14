# Adversarial Plan Review — Bugs/036

**Plan:** `bugs/036/plan.md` — SQLite → PostgreSQL Parity for Fly.io + Neon Deployment

**Reviewer:** AI Agent

**Status:** ✅ Approved with nits

---

## Verified Claims

| # | Claim | Result |
|---|-------|--------|
| 1 | `agent_integrations` missing from Postgres `LATEST.sql` | ✅ Confirmed — absent from `LATEST.sql` (ends at line 975 with `bridge_auth_nonces`). Source DDL exists at `0.31/00__agent_integrations.sql`. |
| 2 | `agent_events` missing from Postgres `LATEST.sql` | ✅ Confirmed — absent from `LATEST.sql`. Source DDL exists at `0.31/01__agent_events.sql`. |
| 3 | No `0.28/` Postgres migration directory | ✅ Confirmed — directory does not exist. SQLite has `0.28/00`, `0.28/01`, `0.28/02`. |
| 4 | `CreateAgentAudience` INSERT omits `max_message_length` | ✅ Confirmed — Postgres `agent.go:120-131` vs SQLite `agent.go:148-164`. |
| 5 | `ListAgentAudiences` SELECT omits `max_message_length` | ✅ Confirmed — Postgres `agent.go:158-162` vs SQLite `agent.go:193-197`. |
| 6 | `UpdateAgentAudience` UPDATE omits `max_message_length` | ✅ Confirmed — Postgres `agent.go:199-208` vs SQLite `agent.go:244-250`. |
| 7 | `idx_user_username` missing from Postgres `LATEST.sql` | ✅ Confirmed — absent. SQLite `LATEST.sql` line 31 has `CREATE INDEX idx_user_username ON user (username);`. |
| 8 | `max_message_length` column exists in Postgres `LATEST.sql:178` | ✅ Confirmed. |
| 9 | `memo_relation.tenant_id` exists in Postgres `LATEST.sql:69` | ✅ Confirmed. Index at line 73. |
| 10 | `user.allowed_tenant_ids` exists in Postgres `LATEST.sql:27` | ✅ Confirmed. |
| 11 | `agent_rag_active_versions` already in Postgres `LATEST.sql` | ✅ Confirmed at lines 713-724. Not missing — correct to exclude from proposed changes. |
| 12 | Go driver for `AgentIntegration`/`AgentEvent` exists in Postgres | ✅ Confirmed at `agent.go:2593-2810`. Tables are missing from schema but driver code is complete. |
| 13 | `max_message_length` covered by `0.29/02` migration | ✅ Confirmed — `0.29/02__add_max_message_length.sql` exists with `ALTER TABLE agent_audiences ADD COLUMN max_message_length INTEGER NOT NULL DEFAULT 2000;`. |
| 14 | Plan's `agent_integrations` DDL matches source | ✅ Confirmed — matches `0.31/00__agent_integrations.sql` column-for-column. |
| 15 | Plan's `agent_events` DDL matches source | ✅ Confirmed — table definition matches `0.31/01__agent_events.sql`. |

---

## Nits

### 1. `IF NOT EXISTS` on CREATE TABLE doesn't match LATEST.sql convention

The plan proposes `CREATE TABLE IF NOT EXISTS agent_integrations (...)` and `CREATE TABLE IF NOT EXISTS agent_events (...)`. LATEST.sql creates tables **without** `IF NOT EXISTS` (e.g., line 15 `CREATE TABLE "user" (...)`, line 714 `CREATE TABLE agent_rag_active_versions (...)`; only `migration_history` and `system_setting` are exceptions). Since LATEST.sql is a fresh-install baseline, `IF NOT EXISTS` adds no value and breaks consistency.

**Fix:** Drop `IF NOT EXISTS` from the `CREATE TABLE` statements.

### 2. "Copied verbatim" claim is slightly inaccurate

Plan line 97 says "DDL copied verbatim" from `0.31/01__agent_events.sql`, but the `-- NOTE: status DEFAULT 'processing' is intentional...` header comment (line 1 of the source) is omitted from the plan's DDL block.

**Impact:** None — cosmetic only.

### 3. No verification step for `idx_user_username`

Verification Step 2 checks table count but never explicitly verifies that the `idx_user_username` index on `"user"`(username) was created. A fresh Postgres install could silently skip this index without the table-count check catching it (index count ≠ table count).

**Fix:** Add a verification step:
```bash
psql ... -c "\di idx_user_username"
```

### 4. Backfill performance not risk-assessed

The `0.28` migration backfills `memo_relation.tenant_id` with a correlated subquery:
```sql
UPDATE memo_relation
SET tenant_id = (SELECT m.tenant_id FROM memo m WHERE m.id = memo_relation.memo_id)
WHERE tenant_id IS NULL;
```
On existing databases with a large `memo_relation` table, this could be slow. The Risks table doesn't mention it.

**Fix:** Add a risk note with mitigation (e.g., "batch in transactions of 10K rows if necessary").

---

## Missed Opportunities (not blockers)

- **Migration naming alignment**: SQLite `0.28/00` is `add_tenant_to_memo_relation` and `0.28/02` is `add_allowed_tenant_ids_to_user`. The plan merges both into `0.28/00__tenant_isolation.sql`. This skips the `01` and `02` suffixes. Consider `0.28/00__tenant_isolation.sql` vs splitting into two files for 1:1 SQLite parity.
- **`agent_events` DEFAULT comment**: Consider including the `-- NOTE` comment from the source migration to document why `DEFAULT 'processing'` is intentional.

---

## Summary

The plan is **sound, thorough, and implementation-ready**. All 6 gaps are correctly identified, the proposed SQL and Go changes are accurate, and the verification steps are concrete. The nits above are cosmetic or edge-case — none block implementation. Approved with nits.
