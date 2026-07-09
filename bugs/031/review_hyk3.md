# Adversarial Code Review: Postgres LATEST.sql FK Ordering Fix

**Reviewer:** hyk3 (adversarial pass)
**Target:** `store/migration/postgres/LATEST.sql` (959 lines, 53 tables)
**Execution context:** `store/migrator.go:133-180` (`preMigrate`) runs the entire file as a single `tx.ExecContext()` inside a transaction; on any error the transaction rolls back and the app crashes (`fly deploy` fails).
**Deploy target:** Fly.io app `bchat-pg`, Neon Postgres, fresh DB (`migration_history` empty → `LATEST.sql` path).

---

## CRITICAL Issues (will cause deploy failure)

**None found that block a fresh Neon deploy.**

I manually traced **every** `REFERENCES` clause in the file (not relying on any audit script). All 53 tables have correct creation order. The specific FKs cited as the original bugs all now resolve correctly:

| Referencing column | Referenced table | Created at | Ref at | OK? |
|---|---|---|---|---|
| `user_tenant_permission.source_template_id` | `tenant_role_templates` | L184 | L207 | ✅ |
| `agent_leads.transcript_id` | `agent_transcripts` | L260 | L286 | ✅ |
| `agent_script_analysis.simulation_id` | `agent_simulations` | L511 | L532 | ✅ |
| `agent_workflows.ticket_id` | `tickets` | L637 | L669 | ✅ |
| `agent_observations.session_id` | `agent_sessions` | L424 | L710 | ✅ |

Additional checks (all valid):
- `tickets.parent_id → tickets(id)` self-reference — valid in Postgres.
- All composite FKs resolve to valid composite `UNIQUE` constraints:
  - `bridge_handoff_replies (tenant_id, handoff_id) → bridge_handoffs (L789)`
  - `bridge_handoff_replies (tenant_id, session_id) → bridge_external_sessions (L760)`
  - `bridge_reply_outbox (tenant_id, session_id) → bridge_external_sessions`
  - `bridge_reply_outbox (tenant_id, handoff_id) → bridge_handoffs`
  - `bridge_reply_outbox (tenant_id, reply_id) → bridge_handoff_replies (L824)`
  - `bridge_auth_nonces (tenant_id, key_id) → bridge_auth_keys (L930)`
- Every other `tenant_id`/`user_id` FK points to `agent_tenants` (L146) or `"user"` (L15), both created early.

The two restored tables (`tenant_role_templates`, `agent_observations`) now exist as `CREATE TABLE` and are referenced correctly. No FK references a missing or later-created table.

---

## HIGH Issues (likely cause problems)

### H1. Incremental-migration drift — upgrading non-fresh DBs will be missing tables/columns (Checklist #8)

`LATEST.sql` only runs when `migration_history` is **empty**. For an existing DB, upgrades go through the versioned directories `0.26`, `0.27`, `0.29`. Those do **not** create everything the new `LATEST.sql` adds:

- `0.26` creates `agent_tenants`, `agent_audiences`, `user_tenant_permission`, `tenant_config`, `agent_messages`, `agent_leads` — but **not** `tenant_role_templates` and **not** `agent_observations`.
- `0.27` and `0.29` only `ALTER` existing tables (add `tenant_id`, `widget_key`, `max_message_length`).
- `0.26`'s `user_tenant_permission` is created **without** the `source_template_id` column that the new `LATEST.sql` (L215) and presumably the Go code now expect.
- `0.26` also omits the `idx_user_tenant_permission_template` and `idx_tenant_config_tenant` indexes.

**Impact:** Any tenant who deployed a *previous* (working) Postgres version and upgrades will end up with a DB missing `tenant_role_templates`, `agent_observations`, and the `source_template_id` column. Queries/inserts against these will fail at runtime. This does **not** affect a brand-new Neon DB (the `fly deploy` scenario in the brief), which uses `LATEST.sql`. But it is a real failure mode for the upgrade path and should be remediated with new incremental migrations (`0.30+`) that `CREATE TABLE`/`ADD COLUMN` to converge.

### H2. The seed `INSERT` will break any re-run on a non-empty DB (Checklist #4, #6)

`INSERT INTO tenant_role_templates (...)` (L199-205) has **no `ON CONFLICT`** clause. On a fresh DB it runs exactly once and is fine. However, if `preMigrate` ever executes `LATEST.sql` against a DB where tables already exist (e.g., `migration_history` is empty but tables present, or the file is run manually), the `CREATE TABLE` statements are silently tolerated by `execute()` (which swallows errors containing `"already exists"` — `migrator.go:265`), but the seed `INSERT` fails with a unique-violation error (not in the tolerate list) → deploy crash.

Mitigation: a *failed* deploy rolls back fully because Postgres DDL is transactional (`preMigrate` returns before `tx.Commit()` on error, `defer tx.Rollback()` at L156). So a failed-then-retried deploy is safe. The risk is only the manually-corrupted state described above. Still, the absence of `ON CONFLICT DO NOTHING` (which the brief says the old version had) removes an idempotency guard. **Recommend** restoring `ON CONFLICT DO NOTHING` (or `ON CONFLICT (tenant_id, code) DO NOTHING`) for safety.

---

## MEDIUM Issues (should fix)

### M1. `validate-pg-migrations.sh` gives false confidence (Checklist #6, #9)

- The script runs `psql "$TEST_URL" < "$LATEST_SQL"` (Step 1) which validates raw SQL syntax but does **not** exercise the single-`ExecContext` transaction path that `fly deploy` actually uses. The deploy path enforces FK constraints statement-by-statement; the local check does not reproduce that execution mode.
- Step 4 compares fresh-vs-migrated table lists and only prints a **YELLOW warning** (`exit 0`) on differences. Given H1, the fresh DB (53 tables) and the migrated DB (≈ base + 6 agent tables) will differ significantly, yet validation still reports "All Checks Passed". This masks the upgrade drift.
- **Premise correction:** the brief states psql "handles FK constraints differently (defers to end-of-transaction)." This is inaccurate for Postgres — a `FOREIGN KEY` requires the referenced table to **exist at `CREATE TABLE` time**, in psql just as in `ExecContext`. So psql would *also* have failed on the broken ordering. The local validation passing does not prove the FK fix; only the corrected ordering (verified above) does.

### M2. Inconsistent `IF NOT EXISTS` usage (Checklist #3)

Most tables use bare `CREATE TABLE` (fail if present), while a handful use `CREATE TABLE IF NOT EXISTS`: `migration_history` (L2), `agent_messages` (L244), `agent_leads` (L286), `bridge_handoff_replies` (L799), `bridge_reply_outbox` (L827). Combined with M1/H2 this makes partial/odd re-runs behave unpredictably (some skipped, some fail). Standardize on `IF NOT EXISTS` for all tables to make `LATEST.sql` idempotent.

### M3. `gen_random_uuid()` dependency in `0.29` (related, not in `LATEST.sql`)

`UPDATE agent_tenants SET widget_key = gen_random_uuid()::text ...` assumes `pgcrypto`/core UUID function. Available by default on modern Neon Postgres (PG13+), but worth pinning/confirming. Not a blocker for a fresh `LATEST.sql` run, but relevant to the upgrade path (H1).

---

## LOW Issues (nice to fix)

### L1. "Duplicate `-- agent_transcripts` comment" is a non-issue

The brief flags a duplicate comment at diff lines ~257-258. In the actual file, `-- agent_transcripts` appears **exactly once** (L259); the following lines are `CREATE INDEX` statements (L282-285) with proper semicolons. No duplicate comment, no syntax problem.

### L2. Premise confirmation — pgx multi-statement execution

`store/db/postgres/postgres.go:26` uses `sql.Open("pgx", …)` with **no** `query_exec_mode=simple_protocol` in the DSN. pgx defaults to `QueryExecModeExec` (extended protocol). The prior failed deploy progressed far enough to surface FK-ordering errors, which proves the driver *does* execute the whole `LATEST.sql` as one statement on Neon. No action needed, but it is worth a note that the deploy path relies on this behavior and is not covered by the local validation script (see M1).

### L3. VM memory headroom

`fly_pg.toml` sets `memory = '1024mb'` with `RAG_PIPELINE_ENABLED='true'` and LanceDB (S3-backed). 1 GB is borderline for cold-start + RAG initialization but is not a schema/deploy-blocking issue. Monitor; consider 2 GB if health checks flap.

---

## VERDICT

**SHIP IT** — for the described scenario (fresh Neon Postgres, `migration_history` empty → `LATEST.sql` runs once in a transaction):

- FK ordering is correct (verified manually for all 53 tables).
- All referenced tables exist; no missing tables vs SQLite.
- Indexes reference valid columns; no duplicate index names; partial indexes are syntactically valid.
- Seed `INSERT` is valid JSON, `tenant_id` allows NULL, runs after its `CREATE TABLE`.
- `COPY lib/linux_amd64/liblancedb_go.so` succeeds (file present at L37).

**BUT fix before any existing-DB upgrade (HIGH):** add incremental migrations (`0.30+`) to create `tenant_role_templates`, `agent_observations`, and add `user_tenant_permission.source_template_id` so the upgrade path converges with `LATEST.sql`. Also restore `ON CONFLICT DO NOTHING` on the seed `INSERT` (H2) and standardize `IF NOT EXISTS` (M2) to harden against re-runs.
