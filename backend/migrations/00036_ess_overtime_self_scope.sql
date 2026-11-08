-- +goose NO TRANSACTION
-- +goose Up
INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT r.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),r.id,'lembur.buat','self'
FROM roles r
JOIN role_permissions p ON p.tenant_id=r.tenant_id AND p.role_id=r.id AND p.permission_code='lembur.buat'
LEFT JOIN role_scope_rules s ON s.tenant_id=r.tenant_id AND s.role_id=r.id AND s.permission_code='lembur.buat' AND s.kind='self'
WHERE r.code='KARYAWAN' AND s.id IS NULL;

-- +goose Down
-- Self scope may have been modified after migration; use a reviewed compensating migration.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00036';
