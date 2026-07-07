# s3_probe — reproduce the Tigris `403` on `ListObjectsV2` (bug 026)

Standalone Go program (no CGO, no `rag` tag) that reproduces the exact failing call
from `bugs/026/s3_error.md` (tenant 5, prefix `lancedb/5/`) against `t3.storage.dev`.

## Why

`bugs/026/claude.md` theorizes the tenant-5 `403` is a **Tigris-side permission/scope**
issue (a prefix-scoped token can `Get/PutObject` under its prefix but is denied
`ListBucket`, which is what LanceDB's "ensure table" does first) — not a Lance/Go bug.
This probe runs the equivalent of:

```
aws s3api list-objects-v2 --bucket bchat --endpoint-url https://t3.storage.dev --prefix lancedb/5/
```

in Go, with the credentials production uses, to confirm that theory.

## Important: the production 403 came from the IAM-role path

The local `.env` copy of the Fly secrets returns `InvalidAccessKeyId` (key unknown to
Tigris) — a **different** error than the production log's `AccessDenied` (valid key, no
`ListBucket`). So the local explicit keys are NOT what production used. `vectordb_lance.go`
notes Fly/Tigris uses **IAM-role auth** when no explicit key is set, so production ran with
the IAM role. To reproduce the real failure, run the probe **on the Fly machine without
explicit keys** (see below).

## Build / run locally (explicit keys from `.env`)

The probe reads configuration from a `.env` file (see `.env.example`) containing your
Fly secrets. Explicit shell environment variables take precedence over values in the file.

```bash
cd bugs/026/s3_probe
cp .env.example .env      # then edit .env with your real Fly secrets
go run . --tenant 5
```

`.env` format (KEY=VALUE lines; `#` comments and blank lines ignored; optional quotes
stripped):

```
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
LANCEDB_S3_ENDPOINT=t3.storage.dev
LANCEDB_S3_BUCKET=bchat
LANCEDB_S3_REGION=auto
# optional:
LANCEDB_S3_FORCE_PATH_STYLE=true
```

## Run on Fly to exercise the IAM-role path (reproduces production)

Cross-compile for the Fly machine arch, upload it, and run **without** explicit keys or a
`.env` so the AWS default credential chain picks up Fly's Tigris IAM role:

```bash
GOOS=linux GOARCH=amd64 go build -o s3probe-linux .   # match `fly status` arch (amd64/arm64)
fly ssh sftp shell                                    # put s3probe-linux /tmp/s3probe-linux
fly ssh console -C "/tmp/s3probe-linux --tenant 5 --skip-env"
```

## Flags

- `--env-file` (default `.env`; skipped if absent)
- `--skip-env` — do not load `.env`; rely solely on the process environment / default
  credential chain. **Required on Fly** so a stray `.env` can't inject explicit keys and
  mask the IAM role.
- `--tenant` (default 5), `--prefix` (default `lancedb`),
  `--path-style` (default `true`, reproduces the observed `https://t3.storage.dev/bchat?...`),
  `--max-keys` (default 5).

## Interpreting the result

The verdict is **code-aware** (different 403 sub-codes have different root causes):

- **`api_code: InvalidAccessKeyId` / `InvalidSecretKey`** → CREDENTIAL INVALID. The key/secret
  is unknown to Tigris (wrong account, rotated, deleted, or typo'd). **Not** a scope issue,
  and it does NOT reproduce the production `AccessDenied`. Verify the key or switch to the
  Fly IAM-role path (`--skip-env`).
- **`api_code: AccessDenied`** → CONFIRMED SCOPE ISSUE. The key is valid but lacks
  `s3:ListBucket`. Fix the Tigris token scope (grant `s3:ListBucket` on bucket `bchat` with a
  `lancedb/*` prefix condition) — not app code. This matches `claude.md`.
- **`200` list succeeds** → production's `AccessDenied` came from a *different* credential
  context. Strong candidate: a per-tenant `TenantS3Override` (`vectordb.go` `resolveStorageTarget`)
  supplying tenant 5 its own scoped key. Check the DB and test those credentials.
- **Other / no HTTP response** → investigate separately (printed verbatim).

The probe is read-only (`ListObjectsV2`, `MaxKeys=5`); it creates/modifies/deletes nothing.
