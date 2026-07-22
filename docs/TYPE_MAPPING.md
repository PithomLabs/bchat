# SQLite → Postgres Type Mapping Reference

**Purpose:** Reference for manually writing Postgres migrations that correspond to SQLite migrations. Both drivers must produce the same logical schema (tables, columns, indexes) but use different SQL syntax.

**Important:** This is a reference document. `create-migration.sh` creates TODO templates for both drivers — you must write the SQL for each driver manually. See the parity philosophy section below for why auto-generation was removed.

---

## Parity Philosophy

Three levels of parity exist between SQLite and Postgres migrations:

| Level | Definition | Enforced? |
|-------|-----------|-----------|
| **Schema parity** | LATEST.sql produces the same logical schema | **Yes — CI gate** (`validate:parity`) |
| **File-list parity** | Migration files exist in both drivers for each directory | **Yes — CI gate** (`validate:parity`) |
| **Incremental path parity** | Identical SQL in each migration file | **No** — different drivers require different SQL |

The automation enforces schema parity and file-list parity. It does NOT enforce incremental path parity because the same logical change requires different SQL syntax per driver.

---

## Type Mapping Table

| SQLite | Postgres | Notes |
|--------|----------|-------|
| `INTEGER` | `BIGINT` | SQLite INTEGER is a type affinity; Postgres has concrete types |
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` | Auto-increment semantics differ |
| `INTEGER PRIMARY KEY` (without AUTOINCREMENT) | `BIGINT GENERATED ALWAYS AS IDENTITY` | Or just `BIGINT` with a sequence |
| `REAL` | `DOUBLE PRECISION` | SQLite REAL is a type affinity |
| `TEXT` | `TEXT` | Same in both (but see JSONB below) |
| `BLOB` | `BYTEA` | Binary data |
| `INTEGER CHECK (x IN (0,1))` | `BOOLEAN` | SQLite has no native boolean |
| `TEXT` (for JSON data) | `JSONB` | Must be identified by context; not auto-detectable |
| `TIMESTAMP` / `DATETIME` | `TIMESTAMPTZ` | Always use timezone-aware in Postgres |
| `DEFAULT (strftime('%s','now'))` | `DEFAULT EXTRACT(EPOCH FROM NOW())` | Epoch seconds |
| `DEFAULT CURRENT_TIMESTAMP` | `DEFAULT NOW()` | Or `DEFAULT CURRENT_TIMESTAMP` (both work) |

---

## Syntax Differences

### Identifier Quoting

| SQLite | Postgres |
|--------|----------|
| `` `column_name` `` | `"column_name"` |
| `` `table_name` `` | `"table_name"` |

Backtick quoting is SQLite-specific. Postgres uses double-quote quoting for identifiers.

### Reserved Words

| Word | SQLite | Postgres |
|------|--------|----------|
| `user` | Unquoted OK | Must be quoted: `"user"` |
| `order` | Unquoted OK | Must be quoted: `"order"` |
| `group` | Unquoted OK | Must be quoted: `"group"` |
| `check` | Unquoted OK | Must be quoted: `"check"` |

When a column or table name is a reserved word in Postgres, it must be quoted with double quotes.

### INSERT OR IGNORE

| SQLite | Postgres |
|--------|----------|
| `INSERT OR IGNORE INTO table (...) VALUES (...)` | `INSERT INTO table (...) VALUES (...) ON CONFLICT DO NOTHING` |
| `INSERT OR REPLACE INTO table (...) VALUES (...)` | `INSERT INTO table (...) ON CONFLICT (...) DO UPDATE SET ...` |

`INSERT OR REPLACE` has different semantics than `ON CONFLICT DO UPDATE` — the former deletes the conflicting row first, the latter updates it. Review carefully.

### Timestamp Functions

| SQLite | Postgres |
|--------|----------|
| `strftime('%s','now')` | `EXTRACT(EPOCH FROM NOW())` |
| `strftime('%Y-%m-%d %H:%M:%S','now')` | `NOW()` or `CURRENT_TIMESTAMP` |
| `datetime('now')` | `NOW()` |

### DDL Transactionality

| Aspect | SQLite | Postgres |
|--------|--------|----------|
| `ALTER TABLE` | Transactional | **NOT transactional** |
| `CREATE TABLE` | Transactional | Transactional |
| `DROP TABLE` | Transactional | Transactional |
| `CREATE INDEX` | Transactional | Transactional (but `CONCURRENTLY` is not) |

This is the most dangerous difference. A Postgres migration that fails mid-`ALTER TABLE` may leave the schema in an inconsistent state. SQLite migrations are always rolled back on failure.

---

## Migration Writing Rules

### SQLite Rules

1. Use backtick quoting for identifiers: `` `column_name` ``
2. Use `INTEGER PRIMARY KEY AUTOINCREMENT` for auto-increment
3. Use `INTEGER CHECK (x IN (0,1))` for booleans
4. Use `BLOB` for binary data
5. Use `strftime('%s','now')` for epoch timestamps
6. Use `INSERT OR IGNORE` for idempotent inserts
7. `ALTER TABLE` is transactional — safe to use in migrations

### Postgres Rules

1. Use double-quote quoting for identifiers: `"column_name"`
2. Use `SERIAL PRIMARY KEY` for auto-increment
3. Use `BOOLEAN` for booleans
4. Use `BYTEA` for binary data
5. Use `EXTRACT(EPOCH FROM NOW())` for epoch timestamps
6. Use `INSERT INTO ... ON CONFLICT DO NOTHING` for idempotent inserts
7. `ALTER TABLE` is NOT transactional — wrap in a transaction where possible, or use `IF EXISTS`/`IF NOT EXISTS` for idempotency
8. Quote reserved words: `"user"`, `"order"`, `"group"`
9. Use `TIMESTAMPTZ` instead of `TIMESTAMP`
10. Use `JSONB` instead of `TEXT` for JSON data (if the context is clear)

---

## Review Checklist for Postgres Migrations

After writing a Postgres migration, verify:

- [ ] All identifiers use double-quote quoting (not backticks)
- [ ] Reserved words are quoted (`"user"`, `"order"`, etc.)
- [ ] `SERIAL PRIMARY KEY` used instead of `INTEGER PRIMARY KEY AUTOINCREMENT`
- [ ] `BOOLEAN` used instead of `INTEGER CHECK (x IN (0,1))`
- [ ] `BYTEA` used instead of `BLOB`
- [ ] `TIMESTAMPTZ` used instead of `TIMESTAMP`
- [ ] `EXTRACT(EPOCH FROM NOW())` used instead of `strftime('%s','now')`
- [ ] `ON CONFLICT DO NOTHING` used instead of `INSERT OR IGNORE`
- [ ] No `ALTER TABLE` inside a transaction (it's not transactional in Postgres)
- [ ] Schema matches the corresponding SQLite migration

---

## Historical Type Differences

These migrations had type or syntax differences between SQLite and Postgres:

| Version | Migration | Difference |
|---------|-----------|------------|
| 0.19 | Initial schema | SQLite uses `INTEGER PRIMARY KEY AUTOINCREMENT`; Postgres uses `SERIAL PRIMARY KEY` |
| 0.20 | Add token fields | SQLite uses `BLOB`; Postgres uses `BYTEA` |
| 0.22 | Add agent config | SQLite uses `INTEGER CHECK (x IN (0,1))`; Postgres uses `BOOLEAN` |
| 0.24 | Add webhook fields | SQLite uses `TEXT` for JSON; Postgres uses `JSONB` |
| 0.32 | Transcript signing | SQLite uses `BLOB`; Postgres uses `BYTEA` |

---

## SQL Parsing Limitations

The `validate-parity.sh` script uses shell-level awk/grep to parse SQL for schema comparison. This approach has known limitations:

- Nested parentheses in `CHECK` constraints (e.g., `CHECK((status = 'active' AND ...) OR (status = 'revoked' AND ...))`)
- Multi-line column definitions spanning 5+ lines
- Inline comments (`--` and `/* */`) that can appear mid-statement
- Type functions in default expressions (`EXTRACT(EPOCH FROM NOW())`, `CAST(strftime('%s','now') AS INTEGER)`)
- CTE-style subqueries in index conditions

The validator is **best-effort lint**, not a semantic comparison engine. For definitive schema comparison, use database introspection (piggyback on `validate-pg-migrations.sh` which uses `information_schema`).

---

## Historical Divergence Cases

These are intentional divergences between SQLite and Postgres migration paths:

| Version | Divergence | Reason |
|---------|-----------|--------|
| 0.33 | Postgres has `00__add_system_secret.sql` (CREATE TABLE); SQLite has `00__fix_max_message_length_default.sql` (UPDATE only) | Different operations — Postgres needs a new table, SQLite only needs a default fix |

When `validate-parity.sh` reports these divergences, they are expected and documented here. The script skips known divergences.
