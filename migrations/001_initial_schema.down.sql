DROP INDEX IF EXISTS idx_audit_logs_created_at;
DROP INDEX IF EXISTS idx_audit_logs_tenant_id;
DROP INDEX IF EXISTS idx_tenant_members_tenant_id;
DROP INDEX IF EXISTS idx_tenants_namespace;
DROP INDEX IF EXISTS idx_tenants_name;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS tenant_members;
DROP TABLE IF EXISTS tenants;
