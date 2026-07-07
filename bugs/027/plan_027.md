# Plan: Fix "Failed to save config" on Agent Admin > LLM Configuration (local)

## Root cause

Saving any LLM config (including a Custom Model) fails with `Failed to save config`.

Call path: `web/src/pages/AgentAdmin.tsx` `LLMConfigSection.handleSave` →
`agentAdminStore.updateLLMConfig` → `PUT /api/v1/agent/:slug/llm-config` →
`HandleSetLLMConfig` (`server/router/api/v1/agent/handlers.go:2409`) →
`store.UpsertTenantConfig`.

- `UpsertTenantConfig` (`store/db/sqlite/rbac.go:264-282`) INSERTs into `tenant_config`
  and references the column **`vector_db_s3_override`**, but the local (and any upgraded)
  `tenant_config` table **does not have that column** → SQLite error
  `no such column: vector_db_s3_override`.
- `HandleSetLLMConfig` returns the generic `echo.NewHTTPError(500, "Failed to save config")`
  at `handlers.go:2470` **without logging the underlying error**, so the real cause is
  invisible in the server logs.

Why the column is missing: `store/migration/sqlite/0.25/35__tenant_vectordb_s3_override.sql`
(version `0.25.36`, since the migrator computes `minor.(patch+1)`) adds the column. But the
migrator only applies files with version **greater than the latest recorded** (currently
`0.30.1`). That file was added *after* this DB had already advanced past `0.25.36`, so
upgraded DBs never executed it. Fresh DBs are fine because `LATEST.sql` (line 461) already
includes the column. → Affects all LLM-config saves locally (confirmed: all saves fail).

## Fix

### 1. Add an idempotent SQLite migration (primary fix)
New file `store/migration/sqlite/0.30/01__add_vector_db_s3_override.sql`:
```sql
-- Backfill the per-tenant S3 override column that was added to the schema (LATEST.sql)
-- but missed on databases upgraded before 0.25/35 existed. The migrator tolerates
-- "duplicate column", so this is safe to re-run on DBs that already have the column.
ALTER TABLE tenant_config ADD COLUMN vector_db_s3_override TEXT DEFAULT '';
```
- Placed in the `0.30` folder as file `01` ⇒ computed version `0.30.2`
  (`minor.(patch+1)`). `0.30.2 > recorded latest 0.30.1` ⇒ applied on next server start.
- `execute()` (`store/migrator.go:264`) already tolerates `duplicate column`, so fresh DBs
  (column already from `LATEST.sql`) are unaffected.
- `schemaVersion` is derived from migration **files** (`GetCurrentSchemaVersion`), not the
  `Version` constant, so no constant bump is strictly required. Optionally bump
  `DevVersion`/`Version` in `internal/version/version.go` to `0.30.2` for consistency
  (currently `0.30.0`).

### 2. Log the underlying error in the handler (diagnosability)
In `server/router/api/v1/agent/handlers.go`, change the upsert-failure branch
(`handlers.go:2468-2471`) to log before returning:
```go
config, err = h.store.UpsertTenantConfig(ctx, config)
if err != nil {
    slog.Error("failed to save tenant config", "tenant", slug, "error", err)
    return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save config: "+err.Error())
}
```
(Optional, same treatment for the encrypt-failure branch at `handlers.go:2459-2462`.)

## Validation

1. Start backend (`task dev:backend`). Confirm migration runs: server logs
   `start migration` … `end migrate` (or query PRAGMA).
2. `sqlite3 build/data/memos_dev.db "PRAGMA table_info(tenant_config);"` shows
   `vector_db_s3_override`.
3. In Agent Admin > LLM Configuration, save a **Custom Model** → "LLM configuration saved"
   toast; no `Failed to save config`.
4. Restart backend → no migration error (idempotent / duplicate-column tolerated).
5. Fresh DB: delete `build/data/memos_dev.db`, re-init → column present via `LATEST.sql`,
   save works.

### Immediate unblock (user, optional)
To test the UI now without waiting for the code fix:
`sqlite3 build/data/memos_dev.db "ALTER TABLE tenant_config ADD COLUMN vector_db_s3_override TEXT DEFAULT '';"`

## Risks / notes

- Do **not** reuse `0.25/35` (already recorded in `migration_history`) — use a new
  `0.30/01` file.
- The handler/store are driver-agnostic. Verify Postgres/MySQL: their `rbac.go` upserts
  also reference `vector_db_s3_override`, and no migration there adds the column. If those
  schemas lack it, add equivalent `ADD COLUMN` migrations (follow-up; user is on SQLite).
- Keeping a generic user-facing message is fine once the root cause is fixed; the key
  improvement is server-side logging so future failures are diagnosable.

## Files touched

- `store/migration/sqlite/0.30/01__add_vector_db_s3_override.sql` (new)
- `server/router/api/v1/agent/handlers.go` (log error at `handlers.go:2468-2471`)
- `internal/version/version.go` (optional version bump to `0.30.2`)
