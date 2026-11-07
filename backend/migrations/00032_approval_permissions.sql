-- +goose NO TRANSACTION
-- +goose Up
INSERT INTO permission_registry (code,module,action,is_sensitive,requires_scope,entitlement_code) VALUES
  ('jadwal.approve','jadwal','approve',TRUE,TRUE,'attendance'),
  ('karyawan.approve','karyawan','approve',TRUE,TRUE,'core_hr');

INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN permission_registry p ON p.code IN ('jadwal.approve','karyawan.approve')
WHERE r.code IN ('ADMIN_TENANT','HR_MANAGER');

INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT rp.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),rp.role_id,rp.permission_code,'all_tenant'
FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id
WHERE rp.permission_code IN ('jadwal.approve','karyawan.approve') AND r.code IN ('ADMIN_TENANT','HR_MANAGER')
AND NOT EXISTS (SELECT 1 FROM role_scope_rules sr WHERE sr.tenant_id=rp.tenant_id AND sr.role_id=rp.role_id AND sr.permission_code=rp.permission_code);

-- +goose Down
-- Permission grants may have been edited; use a reviewed compensating migration.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00032';
