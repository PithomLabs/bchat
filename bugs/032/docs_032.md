# Bug 032: LanceDB S3 path conflict across multiple deployments

## Problem

LanceDB indexes are stored in S3 at `s3://<bucket>/lancedb/<tenant_id>/<table_name>/`. Tenant IDs are auto-incremented integers shared across all deployments. When multiple bchat apps (e.g., `bchat-pg`, `bchat-sqlite`) share the same Tigris S3 bucket, tenant ID collisions occur:

```
s3://shared-bucket/lancedb/4/kb_documents_1536/  # App A
s3://shared-bucket/lancedb/4/kb_documents_1536/  # App B (COLLISION)
```

There was no deployment-level namespace in the S3 path — only the bucket name separated deployments.

## Root Cause

`resolveStorageTarget()` in `vectordb.go:222` hardcoded the prefix `"lancedb"`:

```go
prefix := "lancedb"
uri = fmt.Sprintf("s3://%s/%s/%d", resolved.S3Bucket, prefix, tenantID)
```

The `TenantS3Override.Prefix` field existed but was per-tenant only — no global deployment-level default existed.

## Solution

Added `LANCEDB_S3_PREFIX` env var as a deployment namespace. The S3 path becomes:

```
s3://<bucket>/<prefix>/<tenant_id>/<table_name>/
```

Each deployment sets its prefix to the Fly app name, automatically isolating data:

```
s3://shared-bucket/bchat-pg/lancedb/4/kb_documents_1536/
s3://shared-bucket/bchat-sqlite/lancedb/4/kb_documents_1536/
```

## Changes

### `server/router/api/v1/agent/vectordb.go`

1. **VectorDBConfig struct** — added `S3Prefix string` field (line 82):
   ```go
   S3Prefix string // Deployment namespace prefix (default "lancedb"); set to app name for multi-deploy isolation
   ```

2. **NewVectorDBConfigFromEnv** — reads env var (line 114):
   ```go
   S3Prefix: getEnvOrDefault("LANCEDB_S3_PREFIX", "lancedb"),
   ```

3. **resolveStorageTarget** — uses `global.S3Prefix` instead of hardcoded `"lancedb"` (lines 224-231):
   ```go
   prefix := global.S3Prefix
   if prefix == "" {
       prefix = "lancedb"
   }
   if override != nil && override.Prefix != "" {
       prefix = override.Prefix
   }
   uri = fmt.Sprintf("s3://%s/%s/%d", resolved.S3Bucket, prefix, tenantID)
   ```

### `scripts/fly-pg-secrets.sh`

Added `LANCEDB_S3_PREFIX=$APP_NAME` to the S3 secrets step (line 236):
```bash
fly secrets set \
    LANCEDB_S3_BUCKET="$BUCKET_NAME" \
    LANCEDB_S3_PREFIX="$APP_NAME" \
    --app "$APP_NAME"
```

Updated header comment block to document the new secret.

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `LANCEDB_S3_PREFIX` | `lancedb` | Deployment namespace in S3 path |

**Priority chain:**
1. `TenantS3Override.Prefix` (per-tenant, highest priority)
2. `LANCEDB_S3_PREFIX` env var (deployment-level)
3. `"lancedb"` (fallback)

## Backward Compatibility

- Existing single deployments: no change needed (default `lancedb` prefix is equivalent to old behavior)
- Existing `TenantS3Override.Prefix` per-tenant overrides: still work, override the global prefix
- New multi-deploy setups: set `LANCEDB_S3_PREFIX` to app name for isolation
