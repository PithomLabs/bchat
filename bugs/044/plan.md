# Database Migration Parity Audit: SQLite ↔ PostgreSQL

**Date:** 2026-07-20
**Source:** Senior DB architect review
**Status:** Awaiting plan review before implementation

---

## Changes Overview

| # | Severity | Issue | Fix | Files Changed |
|---|----------|-------|-----|---------------|
| 1 | CRITICAL | `GetSystemSecret`/`UpsertSystemSecret` are stubs in Postgres store | Implement real SQL queries | `store/db/postgres/rbac.go` |
| 2 | CRITICAL | Missing incremental migration for `system_secret` table in Postgres | Add migration `0.33/00__add_system_secret.sql` | `store/migration/postgres/0.33/00__add_system_secret.sql` |
| 3 | MEDIUM | `max_message_length` default is `4000` in SQLite migration (should be `2000`) | Fix default value + add NOT NULL | `store/migration/sqlite/0.28/01__add_max_message_length.sql` |
| 4 | MEDIUM | `tenant_role_templates.tenant_id` CHECK constraint missing in incremental paths | Document limitation for upgraded databases | N/A (LATEST.sql correct, upgrades unaffected for correctness) |

---

## Issue 1: Postgres SystemSecret Store Stub (CRITICAL)

### Problem

`GetSystemSecret()` and `UpsertSystemSecret()` in `store/db/postgres/rbac.go:213-218` return `nil, nil` — they are dead stubs:

```go
func (d *DB) GetSystemSecret(ctx context.Context) (*store.SystemSecret, error) {
    return nil, nil
}

func (d *DB) UpsertSystemSecret(ctx context.Context, secret *store.SystemSecret) (*store.SystemSecret, error) {
    return nil, nil
}
```

The `system_secret` table exists in `store/migration/postgres/LATEST.sql` (line 746) but is never queried or written.

**Impact:**
- API key encryption (`openrouter_api_key_encrypted` in `tenant_config`) cannot initialize its encryption salt on Postgres
- Transcript signing (`transcript_signing_key` in `agent_tenants`) cannot generate keys
- Any feature depending on `SystemSecret` silently fails

**Reference:** SQLite has a full implementation at `store/db/sqlite/rbac.go:520-569`.

### Solution

Implement `GetSystemSecret` and `UpsertSystemSecret` with real SQL queries.

**New code in `store/db/postgres/rbac.go`:**

```go
func (d *DB) GetSystemSecret(ctx context.Context) (*store.SystemSecret, error) {
    query := `
        SELECT id, encryption_salt, key_version, created_at, rotated_at
        FROM system_secret
        WHERE id = 1
    `
    var secret store.SystemSecret
    var createdAtUnix int64
    var rotatedAtUnix sql.NullInt64

    err := d.db.QueryRowContext(ctx, query).Scan(
        &secret.ID, &secret.EncryptionSalt, &secret.KeyVersion, &createdAtUnix, &rotatedAtUnix,
    )
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    secret.CreatedAt = time.Unix(createdAtUnix, 0)
    if rotatedAtUnix.Valid {
        t := time.Unix(rotatedAtUnix.Int64, 0)
        secret.RotatedAt = &t
    }
    return &secret, nil
}

func (d *DB) UpsertSystemSecret(ctx context.Context, secret *store.SystemSecret) (*store.SystemSecret, error) {
    now := time.Now()
    stmt := `
        INSERT INTO system_secret (id, encryption_salt, key_version, created_at)
        VALUES (1, $1, $2, $3)
        ON CONFLICT(id) DO UPDATE SET
            encryption_salt = EXCLUDED.encryption_salt,
            key_version = EXCLUDED.key_version,
            rotated_at = $4
        RETURNING id
    `
    err := d.db.QueryRowContext(ctx, stmt,
        secret.EncryptionSalt, secret.KeyVersion, now.Unix(), now.Unix(),
    ).Scan(&secret.ID)
    if err != nil {
        return nil, err
    }
    secret.CreatedAt = now
    return secret, nil
}
```

**Note:** Uses Postgres parameter style (`$1, $2, $3, $4`) and `EXCLUDED` syntax for `ON CONFLICT`.

### Verification

1. `go build ./...` — compiles without errors
2. Unit test: Create a `SystemSecret`, upsert, read back, verify fields match
3. Run existing RBAC tests: `go test ./store/db/postgres/ -run TestSystemSecret`

---

## Issue 2: Missing Incremental Migration for `system_secret` (CRITICAL)

### Problem

The `system_secret` table is **only** in `LATEST.sql` for Postgres — never in any incremental migration file (0.19 through 0.32).

- **Fresh installs:** OK — `LATEST.sql` creates it
- **Upgrades from pre-0.26:** The table will be **missing**, and no migration adds it
- **SQLite equivalent:** Created in `0.25/07__rbac_tables.sql` (line 36)

### Solution

Create a new migration file `store/migration/postgres/0.33/00__add_system_secret.sql`:

```sql
CREATE TABLE IF NOT EXISTS system_secret (
    id SERIAL PRIMARY KEY CHECK (id = 1),
    encryption_salt BYTEA NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
    rotated_at BIGINT
);
```

### Why `IF NOT EXISTS` is safe here

- Fresh installs: Table already exists from `LATEST.sql`, `IF NOT EXISTS` skips it
- Upgrades: Table doesn't exist, migration creates it
- Re-runs: `IF NOT EXISTS` is idempotent

### Verification

1. Check migration applies cleanly on a Postgres database without `system_secret`
2. Verify `GetSystemSecret()` returns `nil, nil` (no rows) after migration on fresh DB
3. Verify `UpsertSystemSecret()` + `GetSystemSecret()` round-trips correctly

---

## Issue 3: `max_message_length` Default Value Mismatch (MEDIUM)

### Problem

| Context | Value |
|---------|-------|
| SQLite migration `0.28/01` | `DEFAULT 4000` |
| Postgres migration `0.29/02` | `DEFAULT 2000` |
| Both `LATEST.sql` files | `DEFAULT 2000` |

**Impact:** Fresh installs are correct (LATEST.sql). But an SQLite upgrade from pre-0.28 gets `4000` instead of `2000`, creating a behavioral divergence where message validation allows longer messages on upgraded databases.

Additionally, the SQLite migration lacks `NOT NULL`:

| Context | Constraint |
|---------|-----------|
| SQLite migration `0.28/01` | `INTEGER DEFAULT 4000` (nullable) |
| Postgres migration `0.29/02` | `INTEGER NOT NULL DEFAULT 2000` |
| Both `LATEST.sql` files | `INTEGER NOT NULL DEFAULT 2000` |

### Solution

Option A (preferred): Create a new SQLite migration `0.33/00__fix_max_message_length_default.sql`:

```sql
-- Fix max_message_length default from 4000 to 2000 and add NOT NULL
-- This corrects the value set by migration 0.28/01
UPDATE agent_audiences SET max_message_length = 2000 WHERE max_message_length = 4000;

-- Note: SQLite doesn't support ALTER COLUMN, so we cannot change the default
-- or add NOT NULL declaratively. The default only affects new rows, and
-- existing rows are corrected above. The Go code enforces NOT NULL at the
-- application layer via NOT NULL checks in INSERT/UPDATE queries.
```

Option B (alternative): Fix the original migration file `store/migration/sqlite/0.28/01__add_max_message_length.sql` directly:

```sql
ALTER TABLE agent_audiences ADD COLUMN max_message_length INTEGER NOT NULL DEFAULT 2000;
```

**Tradeoff:** Option B changes the migration file, which could break if someone already ran the old version. Option A adds a corrective migration, which is safer for existing databases.

### Recommendation

Use **Option A** (corrective migration). The original migration file should not be modified after it has been released and potentially run on production databases.

### Verification

1. Check that upgraded SQLite databases have `max_message_length = 2000` for all rows
2. Verify no NULL values exist in `agent_audiences.max_message_length`

---

## Issue 4: CHECK Constraint Missing in Incremental Paths (LOW)

### Problem

The `tenant_role_templates.tenant_id` column has a CHECK constraint `CHECK (tenant_id IS NULL OR tenant_id >= 1)` in both `LATEST.sql` files, but:

- **SQLite:** No incremental migration adds this constraint
- **Postgres:** Added in `0.30/04__add_tenant_id_check_to_role_templates.sql`

**Impact:** Low — the CHECK constraint prevents inserting `tenant_id = 0`, which is unlikely in practice. The Go code validates this at the application layer.

### Solution

Document this as a known limitation for upgraded databases. The constraint is enforced:
1. In `LATEST.sql` (fresh installs)
2. In Postgres incremental migration `0.30/04` (Postgres upgrades)
3. At the Go application layer (all databases)

No new migration needed for SQLite — the risk is negligible and SQLite doesn't support `ALTER TABLE ADD CONSTRAINT`.

---

## Files Changed Summary

| File | Change Type |
|------|-------------|
| `store/db/postgres/rbac.go` | Implement `GetSystemSecret` and `UpsertSystemSecret` |
| `store/migration/postgres/0.33/00__add_system_secret.sql` | New migration file |
| `store/migration/sqlite/0.33/00__fix_max_message_length_default.sql` | New migration file |

---

## Testing Plan

1. **Unit tests:**
   - `TestSystemSecretRoundTrip` — upsert + get on Postgres
   - Verify `encryption_salt`, `key_version`, `created_at`, `rotated_at` all round-trip correctly

2. **Integration tests:**
   - Run existing `go test ./store/db/postgres/` suite
   - Run existing `go test ./store/db/sqlite/` suite
   - Verify no regressions

3. **Migration tests:**
   - Apply migration `0.33/00` on a Postgres database without `system_secret`
   - Apply migration `0.33/00` on a Postgres database that already has `system_secret` (idempotent)
   - Verify `max_message_length` fix on upgraded SQLite database

4. **Manual verification:**
   - `go build ./...` compiles
   - `go vet ./...` passes
   - No new lint warnings

---

## Risk Assessment

| Fix | Risk | Mitigation |
|-----|------|-----------|
| SystemSecret Postgres store | Low — straightforward SQL implementation | Follow SQLite implementation pattern exactly |
| SystemSecret migration | Low — `IF NOT EXISTS` is idempotent | Test on both fresh and upgraded databases |
| max_message_length fix | Low — UPDATE only touches wrong values | Verify no data loss, check affected row count |
| CHECK constraint | None — documented limitation | Application layer already enforces |

---

## Rollback Plan

All changes are additive and backward-compatible:

1. **SystemSecret store:** If Postgres queries have issues, revert to stubs (current behavior)
2. **Migration 0.33:** `DROP TABLE IF EXISTS system_secret` (but shouldn't be needed)
3. **max_message_length fix:** `UPDATE agent_audiences SET max_message_length = 4000` (revert to old default)

No schema changes that cannot be rolled back.
