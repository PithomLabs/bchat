-- add_admin_mutation_rate_limit
ALTER TABLE tenant_config ADD COLUMN admin_mutation_rate_limit_rpm INTEGER NOT NULL DEFAULT 30;
