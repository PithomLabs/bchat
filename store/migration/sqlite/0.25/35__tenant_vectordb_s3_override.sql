-- Add per-tenant S3 storage override for LanceDB
-- vector_db_s3_override: JSON-encoded TenantS3Override for per-tenant S3 storage
-- Allows tenants to use their own S3 bucket/prefix/creds instead of the global default.

ALTER TABLE tenant_config ADD COLUMN vector_db_s3_override TEXT DEFAULT '';
