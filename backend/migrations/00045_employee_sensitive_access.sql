-- +goose NO TRANSACTION
-- +goose Up
INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive,entitlement_code) VALUES
('karyawan.lihat_sensitif','karyawan','lihat_sensitif',TRUE,TRUE,'core_hr');

INSERT INTO permission_scope_dimensions (permission_code,dimension)
SELECT 'karyawan.lihat_sensitif',dimension FROM permission_scope_dimensions WHERE permission_code='karyawan.lihat';

INSERT INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'karyawan.lihat_sensitif' FROM roles WHERE code IN ('ADMIN_TENANT','HR_MANAGER');

INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT r.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),r.id,'karyawan.lihat_sensitif','all_tenant'
FROM roles r WHERE r.code IN ('ADMIN_TENANT','HR_MANAGER');

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00045';
