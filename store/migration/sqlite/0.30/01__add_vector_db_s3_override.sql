-- Backfill the per-tenant S3 override column that was added to the schema (LATEST.sql)
-- but missed on databases upgraded before 0.25/35 existed. The migrator tolerates
-- "duplicate column" errors (store/migrator.go), so this is safe to re-run on DBs that
-- already have the column (e.g. fresh DBs created from LATEST.sql).
ALTER TABLE tenant_config ADD COLUMN vector_db_s3_override TEXT DEFAULT '';
