-- +goose NO TRANSACTION
-- +goose Up
-- Menyelaraskan role baru dari 00007 dengan baris ✔/◐ pada Matriks Menu.
-- Role yang sudah ada sebelum 00007 (ADMIN_TENANT dan KARYAWAN) tidak diubah.
CREATE TEMPORARY TABLE preset_modules (role_code VARCHAR(64), module_code VARCHAR(48), level_code VARCHAR(8), PRIMARY KEY(role_code,module_code));
INSERT INTO preset_modules VALUES
('OWNER','dashboard','view'),('OWNER','karyawan','view'),('OWNER','organisasi','view'),('OWNER','kontrak','view'),('OWNER','mutasi','view'),('OWNER','disiplin','view'),('OWNER','terminasi','view'),('OWNER','absensi','view'),('OWNER','cuti','view'),('OWNER','lembur','view'),('OWNER','persetujuan','view'),('OWNER','kasbon','view'),('OWNER','reimbursement','view'),('OWNER','penggajian','view'),('OWNER','broadcast','full'),('OWNER','agenda','view'),('OWNER','direktori','view'),('OWNER','dokumen','view'),('OWNER','laporan','view'),('OWNER','pengguna','view'),('OWNER','audit','view'),
('HR_MANAGER','dashboard','view'),('HR_MANAGER','karyawan','full'),('HR_MANAGER','organisasi','full'),('HR_MANAGER','kontrak','full'),('HR_MANAGER','mutasi','full'),('HR_MANAGER','disiplin','full'),('HR_MANAGER','terminasi','full'),('HR_MANAGER','aset','full'),('HR_MANAGER','absensi','full'),('HR_MANAGER','jadwal','full'),('HR_MANAGER','cuti','full'),('HR_MANAGER','lembur','full'),('HR_MANAGER','persetujuan','full'),('HR_MANAGER','kasbon','view'),('HR_MANAGER','reimbursement','view'),('HR_MANAGER','penggajian','view'),('HR_MANAGER','broadcast','full'),('HR_MANAGER','notifikasi','full'),('HR_MANAGER','agenda','full'),('HR_MANAGER','direktori','view'),('HR_MANAGER','dokumen','full'),('HR_MANAGER','laporan','full'),('HR_MANAGER','pengaturan','full'),('HR_MANAGER','pengguna','view'),('HR_MANAGER','audit','view'),
('HR_STAFF','dashboard','view'),('HR_STAFF','karyawan','full'),('HR_STAFF','organisasi','view'),('HR_STAFF','kontrak','full'),('HR_STAFF','mutasi','full'),('HR_STAFF','disiplin','full'),('HR_STAFF','terminasi','full'),('HR_STAFF','aset','full'),('HR_STAFF','absensi','full'),('HR_STAFF','jadwal','full'),('HR_STAFF','cuti','full'),('HR_STAFF','lembur','full'),('HR_STAFF','persetujuan','view'),('HR_STAFF','broadcast','full'),('HR_STAFF','notifikasi','full'),('HR_STAFF','agenda','full'),('HR_STAFF','direktori','view'),('HR_STAFF','dokumen','full'),('HR_STAFF','laporan','full'),('HR_STAFF','pengaturan','view'),
('KEPALA_CABANG','dashboard','view'),('KEPALA_CABANG','karyawan','view'),('KEPALA_CABANG','mutasi','view'),('KEPALA_CABANG','disiplin','view'),('KEPALA_CABANG','terminasi','view'),('KEPALA_CABANG','aset','view'),('KEPALA_CABANG','absensi','view'),('KEPALA_CABANG','jadwal','view'),('KEPALA_CABANG','cuti','view'),('KEPALA_CABANG','lembur','view'),('KEPALA_CABANG','persetujuan','view'),('KEPALA_CABANG','reimbursement','view'),('KEPALA_CABANG','broadcast','view'),('KEPALA_CABANG','agenda','view'),('KEPALA_CABANG','direktori','view'),('KEPALA_CABANG','dokumen','view'),('KEPALA_CABANG','laporan','view'),
('REGIONAL_MANAGER','dashboard','view'),('REGIONAL_MANAGER','karyawan','view'),('REGIONAL_MANAGER','mutasi','view'),('REGIONAL_MANAGER','disiplin','view'),('REGIONAL_MANAGER','terminasi','view'),('REGIONAL_MANAGER','absensi','view'),('REGIONAL_MANAGER','jadwal','view'),('REGIONAL_MANAGER','cuti','view'),('REGIONAL_MANAGER','lembur','view'),('REGIONAL_MANAGER','persetujuan','view'),('REGIONAL_MANAGER','reimbursement','view'),('REGIONAL_MANAGER','broadcast','view'),('REGIONAL_MANAGER','agenda','view'),('REGIONAL_MANAGER','direktori','view'),('REGIONAL_MANAGER','dokumen','view'),('REGIONAL_MANAGER','laporan','view'),
('SPV','dashboard','view'),('SPV','karyawan','view'),('SPV','disiplin','view'),('SPV','absensi','view'),('SPV','jadwal','view'),('SPV','cuti','view'),('SPV','lembur','view'),('SPV','persetujuan','view'),('SPV','reimbursement','view'),('SPV','agenda','view'),('SPV','direktori','view'),('SPV','dokumen','view'),('SPV','laporan','view'),
('FINANCE_PAYROLL','dashboard','view'),('FINANCE_PAYROLL','karyawan','view'),('FINANCE_PAYROLL','kontrak','view'),('FINANCE_PAYROLL','terminasi','view'),('FINANCE_PAYROLL','absensi','view'),('FINANCE_PAYROLL','lembur','view'),('FINANCE_PAYROLL','persetujuan','view'),('FINANCE_PAYROLL','kasbon','full'),('FINANCE_PAYROLL','reimbursement','full'),('FINANCE_PAYROLL','penggajian','full'),('FINANCE_PAYROLL','direktori','view'),('FINANCE_PAYROLL','dokumen','view'),('FINANCE_PAYROLL','laporan','view');

-- Grant awal role baru pada 00007 belum pernah dipakai untuk perubahan data.
DELETE v FROM role_scope_values v JOIN role_scope_rules sr ON sr.tenant_id=v.tenant_id AND sr.id=v.rule_id JOIN roles r ON r.tenant_id=sr.tenant_id AND r.id=sr.role_id WHERE r.code IN ('OWNER','HR_MANAGER','HR_STAFF','KEPALA_CABANG','REGIONAL_MANAGER','SPV','FINANCE_PAYROLL');
DELETE sr FROM role_scope_rules sr JOIN roles r ON r.tenant_id=sr.tenant_id AND r.id=sr.role_id WHERE r.code IN ('OWNER','HR_MANAGER','HR_STAFF','KEPALA_CABANG','REGIONAL_MANAGER','SPV','FINANCE_PAYROLL');
DELETE rp FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id WHERE r.code IN ('OWNER','HR_MANAGER','HR_STAFF','KEPALA_CABANG','REGIONAL_MANAGER','SPV','FINANCE_PAYROLL');

INSERT INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN preset_modules m ON m.role_code=r.code JOIN permission_registry p ON p.module=m.module_code
WHERE m.level_code='full' OR p.action='lihat';
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN permission_registry p ON p.code='panel.admin'
WHERE r.code IN ('OWNER','HR_MANAGER','HR_STAFF','KEPALA_CABANG','REGIONAL_MANAGER','SPV','FINANCE_PAYROLL');
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN permission_registry p ON p.code IN ('dashboard.lihat_keuangan','penggajian.lihat_slip') WHERE r.code='OWNER';
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN permission_registry p ON p.code IN ('absensi.monitor_bawahan','cuti.approve') WHERE r.code IN ('SPV','KEPALA_CABANG','REGIONAL_MANAGER');
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN permission_registry p ON p.code='absensi.monitor_bawahan' WHERE r.code IN ('HR_MANAGER','HR_STAFF');
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN permission_registry p ON p.code='dashboard.lihat_keuangan' WHERE r.code='FINANCE_PAYROLL';
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN permission_registry p ON p.code='notifikasi.kelola' WHERE r.code IN ('HR_MANAGER','HR_STAFF');
DELETE rp FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id WHERE rp.permission_code='broadcast.target_semua' AND r.code='HR_STAFF';

INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT rp.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),rp.role_id,rp.permission_code,
 CASE WHEN r.code='SPV' THEN 'direct_reports' ELSE 'all_tenant' END
FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id JOIN permission_registry p ON p.code=rp.permission_code
WHERE p.requires_scope=TRUE AND r.code IN ('OWNER','HR_MANAGER','HR_STAFF','SPV','FINANCE_PAYROLL');
-- Kepala Cabang dan Regional Manager mulai tanpa node; editor scope harus menugaskan kantor/wilayah dahulu.
DROP TEMPORARY TABLE preset_modules;

-- +goose Down
-- Preset tidak otomatis dikembalikan agar perubahan hak pengguna tidak tertimpa.
