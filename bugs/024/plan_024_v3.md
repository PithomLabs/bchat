# Plan 024 (v3, consolidated): Tenant-scoped S3 (Tigris) storage for LanceDB on Fly.io

**Status:** Implementation-ready
**Date:** 2026-07-07
**Context doc:** `bugs/024/tigris.md`

## Goal

Make `Dockerfile.s3.fly` a faithful S3/Tigris port of `Dockerfile.local.fly`, and extend
the S3 LanceDB backend so each tenant's vector index is stored under its **own S3 prefix**
in a single shared Tigris bucket:

```
s3://<bucket>/lancedb/<tenant_id>/...
```

This combines two concerns:
1. **S3 enablement + Dockerfile parity (v2):** switch LanceDB off local disk to Tigris
   (`t3.storage.dev`), fix addressing, and make `Dockerfile.s3.fly` identical to
   `Dockerfile.local.fly` except the storage provider.
2. **Tenant scoping (v3):** per-tenant S3 prefix, with optional per-tenant override.

Scope is **LanceDB only**. SQLite stays on the Fly volume (Neon later). `fly.toml` is
**not** modified by this plan — the user will point `[build] dockerfile` at
`Dockerfile.s3.fly` manually.

## Decisions (confirmed)

| Decision | Choice |
|----------|--------|
| SQLite storage | Keep Fly volume at `/var/opt/memos` (Neon later) |
| Tigris endpoint | `t3.storage.dev` (region `auto`) — canonical, per tigris.md |
| `fly.toml` | Left as-is; user points `[build] dockerfile` to `Dockerfile.s3.fly` |
| Bucket name | Supplied only via `fly secrets set LANCEDB_S3_BUCKET` (NOT baked in image) |
| Credentials | Shared `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` from `fly storage create` |
| Dockerfile parity | `Dockerfile.s3.fly` mirrors `Dockerfile.local.fly` except storage provider |
| `ForcePathStyle` | `false` for Tigris (virtual-hosted style), overridable via env |
| Tenant scoping | Per-tenant S3 **prefix** in one shared bucket |
| Prefix identifier | Numeric `tenant_id` → `s3://<bucket>/lancedb/<tenant_id>/` |
| Per-tenant override | Allowed via `TenantConfig` (bucket/prefix/creds), fallback to global |
| Connection model | Per-tenant LanceDB connection, cached in a pool |

## Current state (investigation findings)

1. **S3 support already exists in code.** `vectordb_lance.go:53-69` builds
   `s3://<bucket>/lancedb` and passes `S3Config` to lancedb-go. Reads `LANCEDB_S3_BUCKET`,
   `LANCEDB_S3_ENDPOINT`, `LANCEDB_S3_REGION`, `AWS_ACCESS_KEY_ID/SECRET`.
2. **Bug — `ForcePathStyle: ptr(true)`** (`vectordb_lance.go:66`). Tigris `t3.storage.dev`
   uses virtual-hosted-style; path-style is deprecated by AWS and wrong for the canonical
   endpoint. Must flip to `false` + env override.
3. **`Dockerfile.s3.fly` missing items present in `Dockerfile.local.fly`:**
   - build: `COPY web/vendor ./vendor` (line 13)
   - build: `RUN test -f node_modules/@usememos/mui/dist/index.css` (line 16)
   - runtime: `RUN mkdir -p /var/opt/memos/lancedb` (line 84) — keep for parity
   - env: `LLM_MODEL_REASONING` (103), `EMBEDDING_TIMEOUT` (104), `LLM_MODEL` (105)
4. **`Dockerfile.s3.fly` uses legacy endpoint** `fly.storage.tigris.dev`; should be `t3.storage.dev`.
5. **`Dockerfile.s3.fly` sets no `LANCEDB_S3_BUCKET`** — correct (supplied via secret).
6. **Entrypoint already supports `_FILE` secret indirection** for AWS keys
   (`scripts/entrypoint.sh:40-42`) — no change needed.
7. **Single shared connection today.** `Service.vectorDB` (`service.go:100-108`) is ONE
   `LanceVectorDB` with ONE table (`kb_documents_<dim>`); all tenants share it, filtered by
   a `tenant_id` column. Every `VectorDB` interface method already carries `tenantID`
   (or a tenant-scoped ctx), so routing to a per-tenant connection is clean.
8. **SDK** `github.com/lancedb/lancedb-go v0.1.2`: `S3Config` supports `Endpoint`,
   `Region`, `AccessKeyID`, `SecretAccessKey`, `ForcePathStyle`, `AllowHTTP`. HTTPS to
   `t3.storage.dev` works by default.

## Implementation steps

### Step 1 — Fix S3 addressing in code (`vectordb_lance.go`, `vectordb.go`)
- In `vectordb_lance.go:61-68`, set `ForcePathStyle: ptr(config.S3ForcePathStyle)` and
  pass `AllowHTTP: ptr(config.S3AllowHTTP)`.
- In `vectordb.go` `VectorDBConfig`, add:
  ```go
  S3ForcePathStyle bool // default false (virtual-hosted; true for MinIO/R2 path-style)
  S3AllowHTTP       bool // default false
  ```
- In `NewVectorDBConfigFromEnv()`:
  ```go
  S3ForcePathStyle: getEnvOrDefault("LANCEDB_S3_FORCE_PATH_STYLE", "false") == "true",
  S3AllowHTTP:       getEnvOrDefault("LANCEDB_S3_ALLOW_HTTP", "false") == "true",
  ```
- Optional nicety: `LANCEDB_S3_ENDPOINT` falls back to `AWS_ENDPOINT_URL_S3` if unset.

### Step 2 — Config resolution helper (`vectordb.go`)
- Add `type TenantS3Override struct { Bucket, Prefix, Endpoint, Region, AccessKeyID,
  SecretAccessKey string; ForcePathStyle *bool }`.
- Add `func resolveStorageTarget(global *VectorDBConfig, override *TenantS3Override,
  tenantID int32) (uri string, s3Cfg *contracts.S3Config)`:
  - bucket = override.Bucket or global.S3Bucket
  - endpoint = override.Endpoint or global.S3Endpoint
  - region = override.Region or global.S3Region
  - creds = override keys or global (AWS_*) keys
  - path = `lancedb/<tenant_id>` (or `override.Prefix/<tenant_id>`)
  - forcePathStyle = override.ForcePathStyle != nil ? *v : global.S3ForcePathStyle

### Step 3 — Per-tenant connection pool (`vectordb_pool.go`, build tag `rag`)
- `type TenantVectorDBPool struct { mu sync.RWMutex; tenants map[int32]VectorDB;
  global *VectorDBConfig; embedSvc EmbeddingService; store *store.Store }`
- `Get(ctx, tenantID) (VectorDB, error)` — lazy create + cache; reads `TenantConfig`
  override via store, calls `resolveStorageTarget`, opens `newLanceVectorDB` for that
  tenant.
- Implements `VectorDB` by resolving tenant from call args:
  - `Search`: `query.TenantID`
  - `Insert`: first chunk's `TenantID`
  - `Delete` / `DeleteByIDPrefix` / `ListChunks`: passed `tenantID`
  - `Stats`: enumerate tenants, aggregate per-tenant `Stats`
  - `Validate` / `Close`: iterate all cached
- Memory / NoOp paths unchanged (pool only for s3/local tenant scoping).

### Step 4 — Per-tenant target in `newLanceVectorDB` (`vectordb_lance.go`)
- S3 URI → `s3://<bucket>/<path>/` where `path` = `lancedb/<tenant_id>` (or override
  prefix). Pass resolved `S3Config` instead of global.
- Local → `filepath.Join(config.LocalPath, strconv.Itoa(tenantID))` for parity.
- `migrateLegacyTable` / `ensureTable` continue to run per connection; confirm they don't
  cross tenant boundaries (now naturally scoped by prefix).

### Step 5 — Wire pool into `Service` (`service.go`)
- `NewService`: when `config.StorageProvider` is `s3` (or `local` w/ tenant scoping),
  set `svc.vectorDB = NewTenantVectorDBPool(...)`. Memory/no-op unchanged.
- `RefreshVectorDB`: rebuild pool.
- `GetVectorDB()` returns the pool (still satisfies `VectorDB`).
- `ReindexAllContent` (`service.go:263`) already iterates `ListAgentTenants`; each
  `Delete`/`Insert` routes to that tenant's connection automatically.

### Step 6 — `TenantConfig` storage + migration
- `store/agent.go`: add `VectorDBS3Override string` to `TenantConfig` (and
  find/create structs as needed). Opaque JSON string.
- Migration `store/migration/sqlite/NN__add_tenant_vectordb_s3_override.sql`:
  `ALTER TABLE agent_tenant_config ADD COLUMN vector_db_s3_override TEXT DEFAULT '';`

### Step 7 — Admin API for override (recommended)
- In tenant config GET/PUT (`handlers.go` ~1100-1172, 2353), add `vectorDbS3Override`
  field to request/response JSON. Validate JSON shape. Guarded by `tenant:admin` (already
  enforced).

### Step 8 — Update `Dockerfile.s3.fly` (v2 parity)
1. Stage 1 frontend — add `COPY web/vendor ./vendor` and
   `RUN test -f node_modules/@usememos/mui/dist/index.css` (match local.fly).
2. Stage 3 runtime:
   - `ENV LANCEDB_S3_ENDPOINT="t3.storage.dev"` (was `fly.storage.tigris.dev`)
   - `ENV LANCEDB_S3_REGION="auto"`
   - Keep `LANCEDB_S3_BUCKET` **unset** (secret).
   - Add `ENV LLM_MODEL_REASONING="nvidia/nemotron-3-ultra-550b-a55b:free"`
   - Add `ENV EMBEDDING_TIMEOUT="10m"`
   - Add `ENV LLM_MODEL="poolside/laguna-m.1:free"`
   - Add `ENV LANCEDB_S3_FORCE_PATH_STYLE="false"`
   - Keep `RUN mkdir -p /var/opt/memos/lancedb` and `VOLUME /var/opt/memos` (SQLite on volume).
   - Do NOT bake `AWS_*` secrets into the image.
3. Net effect: `Dockerfile.s3.fly` == `Dockerfile.local.fly` with
   `LANCEDB_STORAGE_PROVIDER=local`+`LANCEDB_LOCAL_PATH` replaced by the S3 block.

### Step 9 — Provision Tigris bucket (one-time, manual — not in code)
```bash
fly storage create                       # creates bucket + sets AWS_ACCESS_KEY_ID/SECRET
fly secrets set LANCEDB_S3_BUCKET=<name> # required, not in image
fly secrets set AWS_ENDPOINT_URL_S3=https://t3.storage.dev  # canonical endpoint
```

## Files touched
- `server/router/api/v1/agent/vectordb.go` — config struct/env + `resolveStorageTarget`
- `server/router/api/v1/agent/vectordb_pool.go` — NEW per-tenant pool (build tag `rag`)
- `server/router/api/v1/agent/vectordb_lance.go` — `ForcePathStyle`/`AllowHTTP`, per-tenant URI/S3Config
- `server/router/api/v1/agent/service.go` — wire pool into `Service`
- `store/agent.go` — `TenantConfig.VectorDBS3Override`
- `store/migration/sqlite/NN__add_tenant_vectordb_s3_override.sql` — migration
- `server/router/api/v1/agent/handlers.go` — admin override field (optional)
- `Dockerfile.s3.fly` — parity with local.fly + S3 endpoint/env; no baked secrets
- `scripts/entrypoint.sh` — no change (already supports `_FILE` secrets)

## Migration / rollout
- Existing shared `s3://<bucket>/lancedb` table (`kb_documents_<dim>`) is NOT auto-migrated.
  With per-tenant prefix `s3://<bucket>/lancedb/<tenant_id>/`, old data is orphaned.
- After deploy: trigger full reindex (`FORCE_REINDEX_ON_STARTUP=true` or Admin "Rebuild
  Index" per tenant) to populate per-tenant prefixes. Delete old shared table from bucket later.
- `migrateLegacyTable` runs per connection; verify it stays within the tenant prefix.

## Verification
1. Build `Dockerfile.s3.fly`; confirm mui css test + vendor copy present.
2. Run with 2 tenants + real Tigris creds → startup log shows S3 provider, no "bucket required".
3. Reindex both tenants; confirm objects at
   `s3://<bucket>/lancedb/<tid1>/kb_documents_<dim>/` and `.../<tid2>/...`.
4. Query tenant A → only tenant A chunks (isolation by prefix).
5. Set `TenantConfig.VectorDBS3Override` for one tenant → confirm writes to override location.
6. `GET /api/v1/agent/<slug>/validate` and `/api/v1/admin/rag/stats` healthy/aggregated.
7. `fly storage dashboard <bucket>` shows per-tenant prefixes.

## Out of scope
- Switching `fly.toml` `dockerfile` field (user does manually).
- Neon Postgres migration for SQLite (later roadmap).
- Per-tenant IAM credential provisioning on Fly (override is data-driven, optional).
- Local `provider` path remains intact for dev.
