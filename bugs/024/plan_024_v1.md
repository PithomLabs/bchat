# Plan 024: S3 (Tigris) storage for LanceDB on Fly.io

**Status:** Draft (interactive — awaiting sign-off)
**Date:** 2026-07-07
**Context doc:** `bugs/024/tigris.md`

## Goal

Replace the **local-disk LanceDB** backend (currently stored on the Fly volume at
`/var/opt/memos/lancedb`) with an **S3-compatible object store backed by Tigris**
(`t3.storage.dev`), so the RAG vector index is durable, shareable across machines,
and not tied to a single Fly volume.

This change is **LanceDB-only**. The SQLite database stays on the Fly volume for now
(Neon Postgres migration is a separate, later roadmap item). The `[[mounts]]` block in
`fly.toml` is retained.

## Decisions (confirmed)

| Decision | Choice |
|----------|--------|
| SQLite storage | Keep Fly volume at `/var/opt/memos` (Neon later) |
| Tigris endpoint | `t3.storage.dev` (region `auto`) — canonical, per tigris.md |
| Deploy shape | Keep `[[mounts]]` volume block in `fly.toml` |
| Bucket name | Provided via `fly secrets set LANCEDB_S3_BUCKET=...` (not baked in) |
| Credentials | `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` set by `fly storage create` |

## Current state (investigation findings)

1. **S3 support already exists in code.** `vectordb_lance.go:53-69` builds an S3 URI
   `s3://<bucket>/lancedb` and passes `S3Config{Endpoint, Region, AccessKeyID,
   SecretAccessKey, ForcePathStyle}` to the lancedb-go connection.
2. **Bug — `ForcePathStyle: ptr(true)`** (`vectordb_lance.go:66`). Tigris
   (`t3.storage.dev`) is designed for **virtual-hosted-style** addressing. Path-style
   is deprecated by AWS and is the wrong setting for the canonical Tigris endpoint.
   -> Must flip to `false` (or make configurable).
3. **`Dockerfile.s3.fly` sets `LANCEDB_STORAGE_PROVIDER=s3` but never sets
   `LANCEDB_S3_BUCKET`** -> startup will fail with "LANCEDB_S3_BUCKET is required".
4. **Endpoint mismatch.** `Dockerfile.s3.fly` and `Dockerfile.fly` use the legacy
   `fly.storage.tigris.dev`; tigris.md recommends `https://t3.storage.dev`.
5. **`fly.toml` currently deploys `Dockerfile.local.fly`** with
   `LANCEDB_STORAGE_PROVIDER=local` + volume mount. It must switch to
   `Dockerfile.s3.fly` and S3 env vars.
6. **Entrypoint already supports `_FILE` secret indirection** for
   `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` (`scripts/entrypoint.sh:40-42`).
7. **SDK:** `github.com/lancedb/lancedb-go v0.1.2` — `S3Config` supports
   `Endpoint`, `Region`, `AccessKeyID`, `SecretAccessKey`, `ForcePathStyle`,
   `AllowHTTP`. TLS to `t3.storage.dev` works over HTTPS by default.

## Implementation steps

### 1. Fix S3 addressing in code (`vectordb_lance.go`)
- Change `ForcePathStyle` to `ptr(false)` for Tigris/virtual-hosted style.
- Make it overridable via new env `LANCEDB_S3_FORCE_PATH_STYLE` (default `false`) so
  MinIO/R2 users can opt back in. Read it in `NewVectorDBConfigFromEnv()`.
- Add `AllowHTTP` support (default false; allow via `LANCEDB_S3_ALLOW_HTTP` only for
  local testing against plain MinIO).
- File: `server/router/api/v1/agent/vectordb_lance.go:61-68`,
  `server/router/api/v1/agent/vectordb.go:65-105`.

### 2. New env vars in config
Add to `VectorDBConfig` / `NewVectorDBConfigFromEnv()`:
```
LANCEDB_S3_BUCKET            (required when provider=s3)
LANCEDB_S3_ENDPOINT          default t3.storage.dev
LANCEDB_S3_REGION            default auto
LANCEDB_S3_FORCE_PATH_STYLE  default false
LANCEDB_S3_ALLOW_HTTP        default false
```
Credentials continue to come from `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`
(with `_FILE` secret support already in entrypoint).

### 3. Update `Dockerfile.s3.fly`
- Change `LANCEDB_S3_ENDPOINT` from `fly.storage.tigris.dev` -> `t3.storage.dev`.
- Remove hardcoded `AWS_*` secrets (secrets are injected by Fly, not baked into image).
- Keep `LANCEDB_S3_BUCKET` **unset** here (supplied at runtime via `fly secrets`).
- Keep `VOLUME /var/opt/memos` and the volume mount for SQLite.
- Optionally switch runtime base to match `Dockerfile.fly` (debian:bookworm-slim) for
  consistency — minor, confirm if desired.

### 4. Update `fly.toml`
- `[build] dockerfile = 'Dockerfile.s3.fly'` (currently `Dockerfile.local.fly`).
- In `[env]`:
  - `LANCEDB_STORAGE_PROVIDER = 's3'`
  - `LANCEDB_S3_ENDPOINT = 't3.storage.dev'`
  - `LANCEDB_S3_REGION = 'auto'`
  - remove `LANCEDB_LOCAL_PATH` (no longer used) — or keep harmless.
- Keep `[[mounts]]` block (SQLite stays on volume).
- Do **not** put `LANCEDB_S3_BUCKET` or AWS keys in `[env]`; set via secrets.

### 5. Provision Tigris bucket (one-time, manual)
```bash
fly storage create  # creates bucket + sets AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY
fly secrets set LANCEDB_S3_BUCKET=<bucket-name>
# Optional: migrate to canonical endpoint
fly secrets set AWS_ENDPOINT_URL_S3=https://t3.storage.dev
```
Note: `fly storage create` sets `AWS_ENDPOINT_URL_S3` to the legacy endpoint; we want
the canonical one, so explicitly `fly secrets set AWS_ENDPOINT_URL_S3=https://t3.storage.dev`.
(Code reads `LANCEDB_S3_ENDPOINT`, not `AWS_ENDPOINT_URL_S3`, so the env var in code path
is the source of truth — keep both consistent.)

### 6. Wire endpoint from `AWS_ENDPOINT_URL_S3` (optional nicety)
Allow `LANCEDB_S3_ENDPOINT` to fall back to `AWS_ENDPOINT_URL_S3` if unset, so Fly's
auto-set secret is honored automatically. Recommended but optional.

### 7. Migrate existing local index -> S3
- Existing LanceDB data is on the volume (`/var/opt/memos/lancedb`). With S3 enabled,
  the app connects to a new empty `s3://<bucket>/lancedb` — old local data is ignored.
- Trigger a full reindex (`FORCE_REINDEX_ON_STARTUP=true` for one boot, or use the
  Agent Admin "Rebuild Index" button) to populate S3. No manual object copy needed.

### 8. Verification
- `task build:rag` (or build `Dockerfile.s3.fly`) and run locally with
  `LANCEDB_STORAGE_PROVIDER=s3` + real Tigris creds -> confirm "LanceDB vector database
  initialized ... provider s3" log and table creation.
- Deploy to Fly; confirm startup log shows S3 provider and no "bucket required" error.
- Run a chat that exercises RAG; confirm search hits S3-backed index.
- `curl .../api/v1/agent/<slug>/validate` and `/api/v1/admin/rag/stats` succeed.

## Files touched
- `server/router/api/v1/agent/vectordb_lance.go` (ForcePathStyle, AllowHTTP)
- `server/router/api/v1/agent/vectordb.go` (config struct + env parsing)
- `Dockerfile.s3.fly` (endpoint, drop baked secrets)
- `fly.toml` (dockerfile, S3 env, keep mounts)
- `scripts/entrypoint.sh` (already supports `_FILE`; no change needed)

## Open questions for sign-off
1. Should `LANCEDB_S3_ENDPOINT` fall back to `AWS_ENDPOINT_URL_S3`? (recommended yes)
2. Align `Dockerfile.s3.fly` runtime base to `debian:bookworm-slim` like `Dockerfile.fly`?
3. For migration, use `FORCE_REINDEX_ON_STARTUP=true` for one boot, or manual reindex button?
4. Any need to keep `local` provider path operable for dev? (yes — leave intact)
