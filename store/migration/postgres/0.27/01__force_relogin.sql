-- Force re-login by deleting all access tokens
-- Existing JWT tokens without tenant_id will be rejected
TRUNCATE TABLE user_access_token RESTART IDENTITY;
