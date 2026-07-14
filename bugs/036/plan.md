# Plan 036 — SQLite → PostgreSQL Parity for Fly.io + Neon Deployment

## Problem / Context (why this exists)

bchat needs to run in production on Fly.io with a Neon PostgreSQL backend. PostgreSQL
support is already ~95% complete — full driver, migration system, Fly.io configs, and
Neon-compatible connection pooling all exist. However, there are **6 concrete gaps**
between the SQLite and PostgreSQL schemas that will cause failures on fresh installs
and existing-database upgrades.

### Consequence

Without fixing these gaps:
- **Fresh Postgres installs** (via `LATEST.sql`) will fail on `agent_integrations` and
  `agent_events` code paths — any integration or webhook event processing crashes.
- **Existing Postgres DBs** upgrading from 0.27 will be missing `memo_relation.tenant_id`
  and `user.allowed_tenant_ids`, breaking tenant isolation for comments and multi-tenant
  user bindings.
- **Audience creation/update via Postgres** silently drops `max_message_length`, so
  message validation thresholds are never persisted — the service falls back to defaults.

### Goal

Achieve full SQLite → PostgreSQL parity so that:
1. Fresh Postgres installs via `LATEST.sql` are complete.
2. Incremental upgrades from 0.27 work without data loss.
3. The Go driver handles all fields identically across both databases.
4. Fly.io + Neon deployment is validated end-to-end.

## Current-state gaps (grounded in code)

| # | Severity | Area | Location | Issue |
|---|----------|------|----------|-------|
| 1 | CRITICAL | Schema | `store/migration/postgres/LATEST.sql:975` | Missing `agent_integrations` table (exists in `0.31/00` migration only) |
| 2 | CRITICAL | Schema | `store/migration/postgres/LATEST.sql:975` | Missing `agent_events` table (exists in `0.31/01` migration only) |
| 3 | HIGH | Migration | `store/migration/postgres/` | No `0.28` directory — `memo_relation.tenant_id` and `user.allowed_tenant_ids` lack incremental upgrade path from 0.27 |
| 4 | HIGH | Go Driver | `store/db/postgres/agent.go:120-131` | `CreateAgentAudience` INSERT omits `max_message_length` (SQLite includes it at line 153) |
| 5 | HIGH | Go Driver | `store/db/postgres/agent.go:199-208` | `UpdateAgentAudience` UPDATE omits `max_message_length` (SQLite includes it at line 249) |
| 6 | HIGH | Go Driver | `store/db/postgres/agent.go:158-162` | `ListAgentAudiences` SELECT omits `max_message_length` (SQLite includes it at line 197) |

### Notes on what's already correct

- `max_message_length` column exists in Postgres `LATEST.sql:178` and migration `0.29/02`.
- `memo_relation.tenant_id` exists in Postgres `LATEST.sql:69` and index at line 73.
- `user.allowed_tenant_ids` exists in Postgres `LATEST.sql:27`.
- All other table columns, indexes, and Go driver function signatures are at parity.
- `pgx/v5` with `default_query_exec_mode=simple_protocol` handles Neon pgbouncer.
- Connection pooling: 10 max open, 5 max idle, 5min lifetime, 1min idle timeout.

## Proposed changes

### Change 1: Append missing tables to Postgres `LATEST.sql`

**File:** `store/migration/postgres/LATEST.sql`

Append after line 975 (after `bridge_auth_nonces`):

```sql
-- agent_integrations
CREATE TABLE agent_integrations (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    integration_type TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    updated_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_integrations_tenant ON agent_integrations(tenant_id);
CREATE UNIQUE INDEX idx_agent_integrations_tenant_type ON agent_integrations(tenant_id, integration_type);

-- agent_events
-- NOTE: status DEFAULT 'processing' is intentional — every insert path pre-claims.
CREATE TABLE agent_events (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    integration_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'processing',
    claimed_at BIGINT DEFAULT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT DEFAULT NULL,
    idempotency_key TEXT UNIQUE,
    created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (integration_id) REFERENCES agent_integrations(id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_events_tenant ON agent_events(tenant_id);
CREATE INDEX idx_agent_events_status ON agent_events(status);
CREATE INDEX idx_agent_events_claimed ON agent_events(claimed_at);
```

**Source:** DDL adapted from `store/migration/postgres/0.31/00__agent_integrations.sql`
and `0.31/01__agent_events.sql`. Dropped `IF NOT EXISTS` to match LATEST.sql convention
(fresh-install baseline doesn't need idempotent guards).

Also append the missing index:

```sql
-- idx_user_username (matches SQLite LATEST.sql:31)
CREATE INDEX idx_user_username ON "user" (username);
```

**Rationale:** Fresh installs use `LATEST.sql` as the baseline. Without these tables,
any code path touching integrations or events will crash with "relation does not exist."

### Change 2: Create Postgres `0.28` migration directory

**New file:** `store/migration/postgres/0.28/00__tenant_isolation.sql`

```sql
-- Add tenant_id to memo_relation for comment tenant isolation (SQLite 0.28/00).
-- Backfills from parent memo's tenant_id for existing rows.
ALTER TABLE memo_relation ADD COLUMN IF NOT EXISTS tenant_id INTEGER DEFAULT NULL;

UPDATE memo_relation
SET tenant_id = (
    SELECT m.tenant_id FROM memo m WHERE m.id = memo_relation.memo_id
)
WHERE tenant_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_memo_relation_tenant ON memo_relation(tenant_id);

-- Add allowed_tenant_ids to user for admin tenant binding (SQLite 0.28/02).
-- Null means user can access all tenants; non-null restricts to listed GUIDs.
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS allowed_tenant_ids TEXT DEFAULT NULL;
```

**Rationale:** Existing Postgres databases at version 0.27 will skip straight to 0.29+
when upgraded. The `memo_relation.tenant_id` column is needed for comment tenant
isolation (used by `memo_service.go` and `memo_relation` queries). The
`user.allowed_tenant_ids` column is needed for admin tenant binding (used by
`auth_service.go`). Without this migration, these columns are silently missing on
upgraded databases, causing query failures or incorrect tenant scoping.

**Note:** `agent_audiences.max_message_length` is already covered by `0.29/02`, so it
does not need to be in this migration.

### Change 3: Fix `max_message_length` in Postgres Go driver

**File:** `store/db/postgres/agent.go`

#### 3a. `CreateAgentAudience` (lines 120-131)

**Current** (line 121-124):
```go
INSERT INTO agent_audiences (
    tenant_id,audience_type,role,tone,brand_voice,guidelines,emergency_phone,
    secondary_phones,email,address,emergency_urgency_threshold,
    escalation_confidence_threshold,rate_limit_rpm,require_contact_on_fallback,updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
```

**Change to:**
```go
INSERT INTO agent_audiences (
    tenant_id,audience_type,role,tone,brand_voice,guidelines,emergency_phone,
    secondary_phones,email,address,emergency_urgency_threshold,
    escalation_confidence_threshold,rate_limit_rpm,require_contact_on_fallback,
    max_message_length,updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
```

And add `audience.MaxMessageLength` to the parameter list before `now` (line 130):
```go
audience.RequireContactOnFallback, audience.MaxMessageLength, now,
```

**Reference:** SQLite equivalent at `store/db/sqlite/agent.go:148-164`.

#### 3b. `ListAgentAudiences` (lines 158-162)

**Current** (line 159-161):
```go
SELECT id,tenant_id,audience_type,role,tone,brand_voice,guidelines,emergency_phone,
    secondary_phones,email,address,emergency_urgency_threshold,
    escalation_confidence_threshold,rate_limit_rpm,require_contact_on_fallback,updated_at
```

**Change to:**
```go
SELECT id,tenant_id,audience_type,role,tone,brand_voice,guidelines,emergency_phone,
    secondary_phones,email,address,emergency_urgency_threshold,
    escalation_confidence_threshold,rate_limit_rpm,require_contact_on_fallback,
    max_message_length,updated_at
```

And add `&audience.MaxMessageLength` to the `Scan` call (line 171-174), inserting it
before `&audience.UpdatedAt`:
```go
&audience.RequireContactOnFallback, &audience.MaxMessageLength, &audience.UpdatedAt
```

**Reference:** SQLite equivalent at `store/db/sqlite/agent.go:193-217`.

#### 3c. `UpdateAgentAudience` (lines 199-208)

**Current** (line 200-204):
```go
UPDATE agent_audiences SET role=$1,tone=$2,brand_voice=$3,guidelines=$4,
    emergency_phone=$5,secondary_phones=$6,email=$7,address=$8,
    emergency_urgency_threshold=$9,escalation_confidence_threshold=$10,
    rate_limit_rpm=$11,require_contact_on_fallback=$12,updated_at=$13
WHERE tenant_id=$14 AND audience_type=$15
```

**Change to:**
```go
UPDATE agent_audiences SET role=$1,tone=$2,brand_voice=$3,guidelines=$4,
    emergency_phone=$5,secondary_phones=$6,email=$7,address=$8,
    emergency_urgency_threshold=$9,escalation_confidence_threshold=$10,
    rate_limit_rpm=$11,require_contact_on_fallback=$12,
    max_message_length=$13,updated_at=$14
WHERE tenant_id=$15 AND audience_type=$16
```

And shift the parameter values (lines 205-208):
```go
audience.Role, audience.Tone, audience.BrandVoice, string(guidelines), audience.EmergencyPhone,
string(phones), audience.Email, audience.Address, audience.EmergencyUrgencyThreshold,
audience.EscalationConfidenceThreshold, audience.RateLimitRPM, audience.RequireContactOnFallback,
audience.MaxMessageLength, now, audience.TenantID, audience.AudienceType
```

**Reference:** SQLite equivalent at `store/db/sqlite/agent.go:244-258`.

### Change 4: Add `fly:deploy` task to `Taskfile_pg.yml`

**File:** `Taskfile_pg.yml`

Append after the `fly:db-check` task (after line 134):

```yaml
  fly:deploy:
    desc: Validate migrations then deploy to fly.io
    deps: [validate:migrations]
    cmds:
      - fly deploy --config fly.toml
```

**Rationale:** Convenience task that chains migration validation → deploy in one shot.
Uses `fly.toml` (which already references `Dockerfile.pg.fly` and `MEMOS_DRIVER=postgres`).

## Files to modify (summary)

| File | Change |
|------|--------|
| `store/migration/postgres/LATEST.sql` | Append `agent_integrations`, `agent_events`, `idx_user_username` after line 975 |
| `store/migration/postgres/0.28/00__tenant_isolation.sql` | **New file** — `memo_relation.tenant_id` + backfill + index + `user.allowed_tenant_ids` |
| `store/db/postgres/agent.go:120-131` | Add `max_message_length` to INSERT |
| `store/db/postgres/agent.go:158-174` | Add `max_message_length` to SELECT + Scan |
| `store/db/postgres/agent.go:199-208` | Add `max_message_length` to UPDATE, shift param numbers |
| `Taskfile_pg.yml:134` | Add `fly:deploy` task |

## Verification

### Step 1: Validate migrations apply cleanly

```bash
# Start local Postgres
task -t Taskfile_pg.yml postgres:start

# Run the migration validation script (LATEST.sql vs incremental)
task -t Taskfile_pg.yml validate:migrations
```

Expected: "All Checks Passed — Postgres migrations are ready for deployment"

### Step 2: Test fresh install via LATEST.sql

```bash
# Drop and recreate test database
psql postgresql://bchat:bchat@localhost:5432/bchat -c "DROP DATABASE IF EXISTS bchat_fresh_test;"
psql postgresql://bchat:bchat@localhost:5432/bchat -c "CREATE DATABASE bchat_fresh_test;"

# Apply LATEST.sql
psql postgresql://bchat:bchat@localhost:5432/bchat_fresh_test < store/migration/postgres/LATEST.sql

# Verify all tables exist
psql postgresql://bchat:bchat@localhost:5432/bchat_fresh_test -c \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
```

Expected: ~55 tables (same count as SQLite LATEST.sql)

### Step 2b: Verify idx_user_username index

```bash
psql postgresql://bchat:bchat@localhost:5432/bchat_fresh_test -c "\di idx_user_username"
```

Expected: Index exists on `"user"` (username).

### Step 3: Test incremental upgrade from 0.27

```bash
# Create database, apply migrations up to 0.27
psql postgresql://bchat:bchat@localhost:5432/bchat -c "DROP DATABASE IF EXISTS bchat_upgrade_test;"
psql postgresql://bchat:bchat@localhost:5432/bchat -c "CREATE DATABASE bchat_upgrade_test;"

# Apply LATEST.sql first (baseline), then re-apply 0.28 on top
# (simulates an existing DB at 0.27 getting the 0.28 migration)
psql postgresql://bchat:bchat@localhost:5432/bchat_upgrade_test -c \
  "ALTER TABLE memo_relation ADD COLUMN IF NOT EXISTS tenant_id INTEGER DEFAULT NULL;"
psql postgresql://bchat:bchat@localhost:5432/bchat_upgrade_test -c \
  'ALTER TABLE "user" ADD COLUMN IF NOT EXISTS allowed_tenant_ids TEXT DEFAULT NULL;'

# Verify columns exist
psql postgresql://bchat:bchat@localhost:5432/bchat_upgrade_test -c \
  "SELECT column_name FROM information_schema.columns WHERE table_name='memo_relation' AND column_name='tenant_id';"
psql postgresql://bchat:bchat@localhost:5432/bchat_upgrade_test -c \
  "SELECT column_name FROM information_schema.columns WHERE table_name='user' AND column_name='allowed_tenant_ids';"
```

Expected: Both columns present.

### Step 4: Run Go tests against Postgres

```bash
DRIVER=postgres DSN="postgresql://bchat:bchat@localhost:5432/bchat" \
  go test -v ./store/test/... -run TestAgentAudience
```

Expected: Audience CRUD tests pass, including `max_message_length` round-trip.

### Step 5: Compile check

```bash
go build ./...
```

Expected: No compile errors.

## Risks / notes

| Risk | Mitigation |
|------|------------|
| `0.28` migration uses `IF NOT EXISTS` — safe to re-run | Migration system tolerates idempotent DDL |
| `max_message_length` default differs: SQLite 4000, Postgres 2000 | Existing rows already have correct defaults from `0.29/02`; only new inserts affected |
| `fly.toml` and `fly_pg.toml` are identical | User chose to keep both; no consolidation |
| `idx_user_username` uses quoted `"user"` table | Required — `user` is a reserved word in Postgres |
| Neon pgbouncer may reject prepared statements | Already handled: `default_query_exec_mode=simple_protocol` in `postgres.go:28-34` |
| Backfill of `memo_relation.tenant_id` may be slow on large tables | Correlated subquery; batch in transactions of 10K rows if needed for large existing databases |

## Status

Plan implemented. All 6 gaps fixed.
