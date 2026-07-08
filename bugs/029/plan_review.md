**Validated Claims (`bugs/029/docs_neon_v2.md`)**

| Claim | Evidence |
|-------|----------|
| 24 files in `store/db/postgres/` | Confirmed |
| 6 unimplemented stubs (3 OM + 3 workflow) | Confirmed (`agent_observations.go`, `agent_workflow.go`) |
| `DB_DRIVER=postgres` broken in `Taskfile_pg.yml` at lines 72/83/94/104/115 | Confirmed; viper prefix is `memos`, so `MEMOS_DRIVER` is required |
| `.env.example` line 92 is `DB_DRIVER=sqlite` | Confirmed |
| `DATABASE_URL` resolved via `os.Getenv()` in `internal/profile/profile.go:98` | Confirmed |
| `agent.go` in postgres is 2474 lines | Confirmed |
| `fly.toml` uses `Dockerfile.s3.fly` | Confirmed |
| `/healthz` endpoint exists | Confirmed (`server/server.go:77`) |
| `RAG_STARTUP_REINDEX_DISABLED` env var exists | Confirmed (`server/router/api/v1/agent/service.go:136`) |
| `validate-pg-migrations.sh` exists | Confirmed |

---

## Adversarial Review: Bugs/029 `docs_neon_v2.md`

### CRITICAL

1. **`fly_pg.toml` template omits required production env vars**
   - Missing `MEMOS_MODE = 'prod'` (present in `fly.toml:27`, `Dockerfile.s3.fly:86`). Without it, `profile.go:67-69` defaults to `demo` mode.
   - Missing `MEMOS_PORT = '5230'` (present in `fly.toml:28`).
   - Missing `EMBEDDING_TIMEOUT = '10m'` (present in `fly.toml:16`).
   - Template claims "All other env: Same" but this is false for at least three variables.

2. **`fly_pg.toml` template omits Fly.io `[http_service]` and `[[vm]]` fields in `fly.toml`**
   - Missing `request_timeout = "30s"` (`fly.toml:43`).
   - Missing `processes = ['app']` (`fly.toml:42`).
   - Missing `memory_mb = 1024` in `[[vm]]` (`fly.toml:60`).

3. **Step 3 `.env` instructions do not prescribe removing the dead `DB_DRIVER` variable**
   - If a user adds `MEMOS_DRIVER=postgres` to an existing `.env` that already contains `DB_DRIVER=sqlite`, they now have conflicting configuration. The plan should explicitly say to comment out or remove `DB_DRIVER`.

---

### HIGH

4. **Step 1 implementation instructions are too vague to port stubs safely**
   - No explicit Postgres `ON CONFLICT(session_id) DO UPDATE SET ...` syntax for `UpsertObservationLog`. The SQLite version (`agent_observations.go:17-24`) uses this exact pattern; the plan only says "Use `INSERT ... ON CONFLICT` for upsert" which is ambiguous between `DO NOTHING` and `DO UPDATE`.
   - No explicit instruction to use `RETURNING id` for `CreateAgentWorkflow`, which the SQLite implementation relies on (`agent_workflow.go:26`, `:54`).
   - Mentions `common.go` `$N` placeholders but never instructs the implementer to actually import and use `placeholders(n)` in the loop-based queries (e.g., `ListAgentWorkflows`).

5. **Seeding gap is described but has no remediation path**
   - "Seeding is SQLite-only | Default tenant_role_templates only seeded on SQLite". The plan does not describe how to seed `tenant_role_templates` on Postgres after deployment. If RBAC depends on these templates, production Postgres deployments will have missing permissions.

6. **Bridge delivery limitation has no guidance**
   - "Bridge delivery not on SQLite | `SupportsBridgeDelivery()` returns false for SQLite | Test bridge features on Postgres"
   - This implies Phase 1 (`task run` with SQLite) cannot test bridge. The plan should explicitly forbid bridge testing in Phase 1.

---

### MEDIUM

7. **`.env.example` modification is misleading**
   - Changing `.env.example` line 92 from `DB_DRIVER=sqlite` to `MEMOS_DRIVER=sqlite` is harmless but `.env.example` is just documentation. The real fix is in `.env`. Also, `.env.example` already has `MEMOS_DSN` commented out at line 96, so it is already inconsistent with `DB_DRIVER` at line 92. Updating only line 92 is half-fixing an existing documentation inconsistency.

8. **`auto_stop_machines` syntax divergence is unexplained**
   - `fly.toml:39` uses `auto_stop_machines = 'stop'` (string).
   - Template uses `auto_stop_machines = true` (boolean).
   - `fly.local.toml:38` and `fly_prod.toml:40` also use `true`.
   - The plan should explain whether `'stop'` is deprecated or if `fly.toml` should be updated for consistency before `fly_pg.toml` is created.

9. **`ENCRYPTION_MASTER_KEY` is misrepresented as required for deployment**
   - Step 6a lists it as a required secret. However, `profile.go` does not validate `EncryptionMasterKey` at startup; the app runs fine without it and only fails when encryption is invoked (`handlers.go:2456`). The plan should flag it as conditional on tenant encryption features.

10. **`Dockerfile.s3.fly` has a harmless-but-unnecessary `VOLUME /var/opt/memos`**
    - `Dockerfile.s3.fly:82` declares `VOLUME /var/opt/memos`. For Postgres, LanceDB uses S3 and SQLite is not used. The volume is dead weight. The plan should note this or strip it in a Postgres-specific Dockerfile variant.

---

### LOW / NITS

11. **`fly_pg.toml` hardcodes `bchat0534-pg` without a "MUST CHANGE" callout**
    - Example values like `postgresql://user:password@...` are clearly placeholders, but the app name is not flagged as requiring modification before `fly deploy`.

12. **`channel_binding=require` is included without justification**
    - Neon supports `channel_binding=require`, but it requires SCRAM-SHA-256 which Neon supports. It is safe but adds a requirement. The plan should note that it can be removed if not needed.

13. **Step 7 says "Before deploying, validate..." but doesn't gate deployment on success**
    - The plan should say: "Fail deployment (do not run `fly deploy`) if validation returns non-zero."

14. **`fly_pg.toml` template does not include `[[http_service.checks]]` timeout/method consistency check against `fly.toml`**
    - Actually it does include it. Strike this.

15. **`LANCEDB_S3_BUCKET` is listed in `fly secrets set` but is not mentioned anywhere in `fly_pg.toml` template `[env]`**
    - This is actually correct behavior (secrets override `[env]`), but a reader might look for it in the template and not find it.

---

## Verdict

The plan is **directionally correct and validated on mechanics** (viper prefix, `DATABASE_URL` fallback, stub locations), but **not ready to implement as-is** because the `fly_pg.toml` template is incomplete and Step 1 implementation guidance is insufficient for a safe Postgres port. The two most dangerous gaps are the missing `MEMOS_MODE=prod` and the non-specific upsert syntax guidance.