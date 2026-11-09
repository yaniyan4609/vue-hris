-- +goose NO TRANSACTION
-- +goose Up
-- Matriks: HM/HR penuh; KC/RM/SP/FN lihat terbatas. Scope KC/RM
-- tetap memerlukan kantor/wilayah yang ditugaskan oleh Admin.
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN permission_registry p ON p.module='terminasi'
WHERE (r.code IN ('HR_MANAGER','HR_STAFF') AND p.action IN ('lihat','buat','ubah','approve'))
   OR (r.code IN ('KEPALA_CABANG','REGIONAL_MANAGER','SPV','FINANCE_PAYROLL') AND p.action='lihat');

INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT rp.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),rp.role_id,rp.permission_code,
       CASE WHEN r.code='SPV' THEN 'direct_reports' ELSE 'all_tenant' END
FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id
LEFT JOIN role_scope_rules existing ON existing.tenant_id=rp.tenant_id AND existing.role_id=rp.role_id AND existing.permission_code=rp.permission_code
WHERE rp.permission_code IN ('terminasi.lihat','terminasi.buat','terminasi.ubah','terminasi.approve')
  AND r.code IN ('HR_MANAGER','HR_STAFF','SPV','FINANCE_PAYROLL') AND existing.id IS NULL;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00054';
