# Plan 024 (v2): S3 (Tigris) storage for LanceDB on Fly.io

**Status:** Implementation-ready
**Date:** 2026-07-07
**Context doc:** `bugs/024/tigris.md`

## Goal

Make `Dockerfile.s3.fly` a faithful S3/Tigris port of `Dockerfile.local.fly`, so that
when `fly.toml` is pointed at `Dockerfile.s3.fly` (under `[build] dockerfile`), the app
runs with the LanceDB vector index stored in **Tigris** (`t3.storage.dev`) instead of the
local Fly volume.

Scope is **LanceDB only**. SQLite stays on the Fly volume (Neon later). `fly.toml` is
**not** modified by this plan — the user will switch its `dockerfile` field manually.

## Decisions (confirmed)

| Decision | Choice |
|----------|--------|
| SQLite storage | Keep Fly volume at `/var/opt/memos` (Neon later) |
| Tigris endpoint | `t3.storage.dev` (region `auto`) — canonical, per tigris.md |
| `fly.toml` | Left as-is; user points `[build] dockerfile` to `Dockerfile.s3.fly` |
| Bucket name | Supplied only via `fly secrets set LANCEDB_S3_BUCKET` (NOT baked in image) |
| Credentials | `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` set by `fly storage create` |
| Dockerfile parity | `Dockerfile.s3.fly` mirrors `Dockerfile.local.fly` exactly except storage provider |
| `ForcePathStyle` | `false` for Tigris (virtual-hosted style), overridable via env |

## Current state (investigation findings)

1. **S3 support already exists in code.** `vectordb_lance.go:53-69` builds `s3://<bucket>/lancedb`
   and passes `S3Config` to lancedb-go. Reads `LANCEDB_S3_BUCKET`, `LANCEDB_S3_ENDPOINT`,
   `LANCEDB_S3_REGION`, `AWS_ACCESS_KEY_ID/SECRET`.
2. **Bug — `ForcePathStyle: ptr(true)`** (`vectordb_lance.go:66`). Tigris `t3.storage.dev`
   uses virtual-hosted-style; path-style is deprecated by AWS and wrong for the canonical
   endpoint. Must flip to `false` + env override.
3. **`Dockerfile.s3.fly` missing items present in `Dockerfile.local.fly`:**
   - build: `COPY web/vendor ./vendor` (line 13)
   - build: `RUN test -f node_modules/@usememos/mui/dist/index.css` (line 16)
   - runtime: `RUN mkdir -p /var/opt/memos/lancedb` (line 84) — keep dir create for parity;
     harmless when using S3 (LanceDB won't write there).
   - env: `LLM_MODEL_REASONING` (line 103), `EMBEDDING_TIMEOUT` (line 104), `LLM_MODEL` (line 105)
4. **`Dockerfile.s3.fly` uses legacy endpoint** `fly.storage.tigris.dev`; should be `t3.storage.dev`.
5. **`Dockerfile.s3.fly` sets no `LANCEDB_S3_BUCKET`** — correct (supplied via secret).
6. **Entrypoint already supports `_FILE` secret indirection** for AWS keys
   (`scripts/entrypoint.sh:40-42`) — no change needed.
7. **SDK** `github.com/lancedb/lancedb-go v0.1.2`: `S3Config` supports `Endpoint`, `Region`,
   `AccessKeyID`, `SecretAccessKey`, `ForcePathStyle`, `AllowHTTP`. HTTPS to `t3.storage.dev` works by default.

## Implementation steps

### Step 1 — Fix S3 addressing in code (`vectordb_lance.go`, `vectordb.go`)
- In `vectordb_lance.go:61-68`, set `ForcePathStyle: ptr(config.S3ForcePathStyle)` and
  pass `AllowHTTP: ptr(config.S3AllowHTTP)`.
- In `vectordb.go` `VectorDBConfig` struct, add:
  ```go
  S3ForcePathStyle bool // default false (virtual-hosted; true for MinIO/R2 path-style)
  S3AllowHTTP       bool // default false
  ```
- In `NewVectorDBConfigFromEnv()`, add:
  ```go
  S3ForcePathStyle: getEnvOrDefault("LANCEDB_S3_FORCE_PATH_STYLE", "false") == "true",
  S3AllowHTTP:       getEnvOrDefault("LANCEDB_S3_ALLOW_HTTP", "false") == "true",
  ```
- Optional nicety: let `LANCEDB_S3_ENDPOINT` fall back to `AWS_ENDPOINT_URL_S3` if unset,
  so Fly's auto-set secret is honored. (Recommended; low risk.)

### Step 2 — Update `Dockerfile.s3.fly` to match `Dockerfile.local.fly` + S3
Apply these edits so it mirrors `Dockerfile.local.fly` exactly except the storage block:

1. Stage 1 frontend — add (same as local.fly):
   ```
   COPY web/vendor ./vendor
   RUN npm ci
   ...
   RUN test -f node_modules/@usememos/mui/dist/index.css
   ```
2. Stage 3 runtime — change endpoint + add missing env defaults:
   - `ENV LANCEDB_S3_ENDPOINT="t3.storage.dev"` (was `fly.storage.tigris.dev`)
   - `ENV LANCEDB_S3_REGION="auto"`
   - Keep `LANCEDB_S3_BUCKET` **unset**.
   - Add `ENV LLM_MODEL_REASONING="nvidia/nemotron-3-ultra-550b-a55b:free"`
   - Add `ENV EMBEDDING_TIMEOUT="10m"`
   - Add `ENV LLM_MODEL="poolside/laguna-m.1:free"`
   - Add (optional) `ENV LANCEDB_S3_FORCE_PATH_STYLE="false"`
3. Keep `RUN mkdir -p /var/opt/memos/lancedb` and `VOLUME /var/opt/memos` for parity
   (SQLite still uses the volume).
4. Do NOT bake `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` into the image.

Net effect: `Dockerfile.s3.fly` == `Dockerfile.local.fly` with
`LANCEDB_STORAGE_PROVIDER=local`+`LANCEDB_LOCAL_PATH` replaced by the S3 block
(`LANCEDB_STORAGE_PROVIDER=s3`, `LANCEDB_S3_ENDPOINT`, `LANCEDB_S3_REGION`).

### Step 3 — Migrate existing local index to S3 (operational, no code)
- Existing LanceDB data on volume (`/var/opt/memos/lancedb`) is ignored once S3 is enabled.
- After deploy, trigger a full reindex to populate `s3://<bucket>/lancedb`:
  - One-boot: set `FORCE_REINDEX_ON_STARTUP=true`, or
  - Use Agent Admin "Rebuild Index" button.
- No manual object copy required.

### Step 4 — Provision Tigris bucket (one-time, manual — not in code)
```bash
fly storage create                       # creates bucket + sets AWS_ACCESS_KEY_ID/SECRET
fly secrets set LANCEDB_S3_BUCKET=<name> # required, not in image
fly secrets set AWS_ENDPOINT_URL_S3=https://t3.storage.dev  # canonical endpoint
```

## Files touched
- `server/router/api/v1/agent/vectordb_lance.go` — `ForcePathStyle`/`AllowHTTP` from config
- `server/router/api/v1/agent/vectordb.go` — config struct + env parsing
- `Dockerfile.s3.fly` — parity with local.fly + S3 endpoint/env; no baked secrets
- `scripts/entrypoint.sh` — no change (already supports `_FILE` secrets)

## Verification
1. Local build of `Dockerfile.s3.fly` (`docker build -f Dockerfile.s3.fly .`) succeeds and
   includes the mui css test + vendor copy.
2. Run binary with `LANCEDB_STORAGE_PROVIDER=s3 LANCEDB_S3_BUCKET=... AWS_*=...` against
   real Tigris → startup log: `LanceDB vector database initialized ... provider s3`, table created.
3. Deploy to Fly with `fly.toml` pointed at `Dockerfile.s3.fly`; confirm no "bucket required"
   error and S3 provider in logs.
4. Exercise RAG via chat; confirm `GET /api/v1/agent/<slug>/validate` and
   `/api/v1/admin/rag/stats` return healthy.
5. Confirm objects appear in the Tigris bucket (`fly storage dashboard <bucket>` or `tigris ls`).

## Out of scope
- Switching `fly.toml` `dockerfile` field (user does manually).
- Neon Postgres migration for SQLite (later roadmap).
- Local `provider` path remains intact for dev.
