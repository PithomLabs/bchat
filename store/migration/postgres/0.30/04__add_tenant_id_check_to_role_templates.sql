ALTER TABLE tenant_role_templates
  ADD CONSTRAINT chk_tenant_role_templates_tenant_id
  CHECK (tenant_id IS NULL OR tenant_id >= 1);
