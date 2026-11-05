-- +goose NO TRANSACTION
-- +goose Up
-- Hak lihat per submenu Admin mengikuti kolom OW/AT/HM/HR/KC/RM/SP/FN pada Matriks Menu.
CREATE TEMPORARY TABLE menu_presets (
  id VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  entitlement_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  role_codes VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL
);
INSERT INTO menu_presets VALUES
('dashboard',NULL,'OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('employees','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('contracts','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,FINANCE_PAYROLL'),
('mutations','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER'),
('discipline','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV'),
('terminations','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('assets','core_hr','ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG'),
('regions','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('offices','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('divisions','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('departments','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('positions','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('attendance-today','attendance','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV'),
('attendance-history','attendance','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('attendance-correction','attendance','ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV'),
('attendance-monitor','attendance','ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV'),
('shifts','attendance','ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('roster','attendance','ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV'),
('holidays','attendance','ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('shift-swaps','attendance','ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV'),
('leave-requests','leave','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV'),
('leave-policy','leave','ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('leave-calendar','leave','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV'),
('overtime','leave','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('approvals','requests','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('loans','requests','OWNER,ADMIN_TENANT,HR_MANAGER,FINANCE_PAYROLL'),
('reimbursements','requests','OWNER,ADMIN_TENANT,HR_MANAGER,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('payroll-runs','payroll','OWNER,ADMIN_TENANT,HR_MANAGER,FINANCE_PAYROLL'),
('pay-components','payroll','ADMIN_TENANT,HR_MANAGER,FINANCE_PAYROLL'),
('tax-bpjs','payroll','ADMIN_TENANT,HR_MANAGER,FINANCE_PAYROLL'),
('payslips','payroll','OWNER,ADMIN_TENANT,HR_MANAGER,FINANCE_PAYROLL'),
('bank-export','payroll','FINANCE_PAYROLL'),
('performance','talent','ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('recruitment','talent','ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('training','talent','ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('broadcasts','communication','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER'),
('notifications','communication','ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('agenda','communication','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV'),
('directory','communication','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('employee-documents','documents','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,FINANCE_PAYROLL'),
('company-documents','documents','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('document-templates','documents','ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('reports','reports','OWNER,ADMIN_TENANT,HR_MANAGER,HR_STAFF,KEPALA_CABANG,REGIONAL_MANAGER,SPV,FINANCE_PAYROLL'),
('company-profile','core_hr','ADMIN_TENANT,HR_MANAGER'),
('users-access','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER'),
('approval-flow','requests','ADMIN_TENANT,HR_MANAGER'),
('reminders','communication','ADMIN_TENANT,HR_MANAGER'),
('hr-policy','core_hr','ADMIN_TENANT,HR_MANAGER,HR_STAFF'),
('theme','core_hr','ADMIN_TENANT,HR_MANAGER'),
('integrations','integrations','ADMIN_TENANT'),
('audit','core_hr','OWNER,ADMIN_TENANT,HR_MANAGER');

INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive,entitlement_code)
SELECT CONCAT('menu.',id,'.lihat'),CONCAT('menu.',id),'lihat',FALSE,FALSE,entitlement_code FROM menu_presets;
INSERT INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,CONCAT('menu.',m.id,'.lihat') FROM roles r JOIN menu_presets m ON FIND_IN_SET(r.code,m.role_codes)>0;

-- Modul talenta masih pratinjau, tetapi menu bertahap tampil bagi Admin/HR.
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,'talenta.lihat' FROM roles r WHERE r.code IN ('ADMIN_TENANT','HR_MANAGER','HR_STAFF');
INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT r.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),r.id,'talenta.lihat','all_tenant'
FROM roles r LEFT JOIN role_scope_rules sr ON sr.tenant_id=r.tenant_id AND sr.role_id=r.id AND sr.permission_code='talenta.lihat'
WHERE r.code IN ('ADMIN_TENANT','HR_MANAGER','HR_STAFF') AND sr.id IS NULL;

-- Grant sensitif yang terlalu luas dari preset "full" awal dibatasi pada aksi yang disebut matriks.
CREATE TEMPORARY TABLE sensitive_excess (tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, role_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, permission_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, PRIMARY KEY(tenant_id,role_id,permission_code));
INSERT INTO sensitive_excess
SELECT rp.tenant_id,rp.role_id,rp.permission_code FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id
JOIN permission_registry p ON p.code=rp.permission_code
WHERE r.code IN ('OWNER','HR_MANAGER','HR_STAFF','KEPALA_CABANG','REGIONAL_MANAGER','SPV','FINANCE_PAYROLL')
AND (p.action='hapus' OR p.action IN ('kelola_akses','delegasikan')
  OR (p.action='ekspor' AND NOT (r.code IN ('HR_MANAGER','HR_STAFF') AND p.code IN ('karyawan.ekspor','absensi.ekspor','laporan.ekspor')))
  OR (p.action='approve' AND NOT (
    (r.code IN ('HR_MANAGER','HR_STAFF') AND p.module IN ('mutasi','terminasi','absensi','cuti','lembur'))
    OR (r.code IN ('KEPALA_CABANG','REGIONAL_MANAGER','SPV') AND p.code='cuti.approve')
    OR (r.code='FINANCE_PAYROLL' AND p.module IN ('kasbon','reimbursement','penggajian'))
  )));
DELETE v FROM role_scope_values v JOIN role_scope_rules sr ON sr.tenant_id=v.tenant_id AND sr.id=v.rule_id JOIN sensitive_excess x ON x.tenant_id=sr.tenant_id AND x.role_id=sr.role_id AND x.permission_code=sr.permission_code;
DELETE sr FROM role_scope_rules sr JOIN sensitive_excess x ON x.tenant_id=sr.tenant_id AND x.role_id=sr.role_id AND x.permission_code=sr.permission_code;
DELETE rp FROM role_permissions rp JOIN sensitive_excess x ON x.tenant_id=rp.tenant_id AND x.role_id=rp.role_id AND x.permission_code=rp.permission_code;
DROP TEMPORARY TABLE sensitive_excess;

DROP TEMPORARY TABLE menu_presets;

-- +goose Down
-- Hak yang sudah diubah per pengguna tidak dibatalkan otomatis.
