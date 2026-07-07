# Debugging go test ./... errors

**Session ID:** ses_0c2588179ffesn8Fl7oBERViZv
**Created:** Invalid Date
**Updated:** 7/8/2026, 1:38:52 AM

---

## User

when I run go test ./... there are errors, why?

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

Let me examine the failing test files to understand each failure.

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: bash**

**Tool: grep**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

I found the root cause. Let me confirm the details of the bridge auth test and the schema version.

**Tool: read**

**Tool: read**

---

## Assistant

Let me confirm the schema version source.

**Tool: bash**

---

## Assistant

**Tool: read**

---

## Assistant

I have enough to explain the root causes. Let me verify `EnsureBridgeExternalSession` doesn't do an explicit tenant-existence check (which would confirm it relies purely on the FK).

**Tool: bash**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

I ran the suite and found exactly **4 failing tests** in `store/test`, caused by **2 distinct root causes**. (The `WARN failed to find migration history` / `INFO Column already exists` lines are benign log noise from the test store setup, not failures.)

## Failure 1 — Stale schema-version assertion (1 test)

`TestGetCurrentSchemaVersion` (store/test/migrator_test.go:18) asserts:
```go
require.Contains(t, currentSchemaVersion, "0.29.", "schema version should be 0.29.x")
```
but `GetCurrentSchemaVersion()` returns **`0.30.2`**. The minor version was bumped to `0.30` (see `internal/version/version.go:12` → `Version = "0.30.0"`) without the test being updated — note the comment above it still says "0.28.x".

**Fix:** update the expected substring to `"0.30."` (or whatever the intended minor is).

## Failure 2 — Missing `FOREIGN KEY` on the bridge tables' `tenant_id` (3 tests)

These three tests all fail for the same reason:

- `TestBridgeExternalSessionUsesAgentTenantsTable` → `"An error is expected but got nil"`
- `TestBridgeAuthKeyTenantCascade` → `"Should be zero, but was 1"`
- `TestSQLiteFKCascadeOnTenantDeletion` → `"Should be zero, but was 1"`

**Important:** foreign keys *are* enabled. The driver opens the DB with `_pragma=foreign_keys(1)` (store/db/sqlite/sqlite.go:42), and the passing test `TestBridgeExternalSessionMigrationAppliesWithForeignKeysEnabled` confirms `PRAGMA foreign_keys` returns `1`.

The problem is the **schema itself**. In `store/migration/sqlite/LATEST.sql`, the bridge tables declare `tenant_id INTEGER NOT NULL` **without** a foreign-key reference back to `agent_tenants`:

```sql
CREATE TABLE bridge_external_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,          -- ❌ no FK to agent_tenants
    ...
);

CREATE TABLE bridge_handoffs (
    ...
    external_session_id INTEGER NOT NULL REFERENCES bridge_external_sessions(id) ON DELETE CASCADE, -- FK to sibling table, but
    tenant_id INTEGER NOT NULL,          -- ❌ no FK to agent_tenants
    ...
);

CREATE TABLE bridge_auth_keys (
    tenant_id INTEGER NOT NULL,          -- ❌ no FK to agent_tenants
    ...
);
```

(Compare with tables that *do* have it, e.g. `agent_intents`, `tenant_config`, `agent_tenant_scripts`, `agent_qa_pairs` — all declare `FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE`.)

Consequences:

1. **`TestBridgeExternalSessionUsesAgentTenantsTable`** calls `EnsureBridgeExternalSession(ctx, 999999, ...)` for a non-existent tenant expecting an FK violation. But `EnsureBridgeExternalSession` (store/db/sqlite/bridge.go:16) does **no explicit tenant-existence check** — it relies entirely on the FK. Since the FK doesn't exist, the insert silently succeeds → `nil` error → test fails.

2. **`TestBridgeAuthKeyTenantCascade` / `TestSQLiteFKCascadeOnTenantDeletion`** delete a tenant, then expect the child bridge rows to be gone. Because there is no `ON DELETE CASCADE` from the bridge tables' `tenant_id` to `agent_tenants`, the rows survive → count is `1`, not `0`.

## Summary

| Test | Root cause |
|------|-----------|
| `TestGetCurrentSchemaVersion` | Stale assertion: expects `0.29.x`, actual is `0.30.x` |
| `TestBridgeExternalSessionUsesAgentTenantsTable` | `bridge_external_sessions.tenant_id` has no FK → missing-tenant insert is allowed |
| `TestBridgeAuthKeyTenantCascade` | `bridge_auth_keys.tenant_id` has no FK → tenant delete doesn't cascade |
| `TestSQLiteFKCascadeOnTenantDeletion` | `bridge_external_sessions`/`bridge_handoffs.tenant_id` have no FK → tenant delete doesn't cascade |

The bridge tables were never given the `FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE` that these tests assume. You can resolve them either by adding those FK constraints (and a migration) to the schema, or by updating the tests if the no-FK behavior is intentional. Since you're in Ask mode, I haven't changed anything — let me know if you'd like me to switch to making the edits.

---

