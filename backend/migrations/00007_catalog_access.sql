-- +goose NO TRANSACTION
-- +goose Up

CREATE TABLE entitlements (
  code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  name VARCHAR(120) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE plans (
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL UNIQUE,
  name VARCHAR(120) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'active'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE plan_entitlements (
  plan_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  entitlement_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (plan_id, entitlement_code),
  FOREIGN KEY (plan_id) REFERENCES plans(id),
  FOREIGN KEY (entitlement_code) REFERENCES entitlements(code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE tenant_subscriptions (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  plan_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  FOREIGN KEY (plan_id) REFERENCES plans(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO entitlements (code,name) VALUES
('core_hr','SDM Inti'),('attendance','Kehadiran dan Jadwal'),('leave','Cuti dan Lembur'),
('requests','Pengajuan'),('payroll','Payroll'),('talent','Talenta'),
('communication','Komunikasi'),('documents','Dokumen'),('reports','Laporan'),('integrations','Integrasi');
INSERT INTO plans (id,code,name) VALUES (LEFT(REPLACE(UUID(),'-',''),26),'staging_full','Staging Lengkap');
INSERT INTO plan_entitlements (plan_id,entitlement_code)
SELECT p.id,e.code FROM plans p CROSS JOIN entitlements e WHERE p.code='staging_full';
INSERT INTO tenant_subscriptions (tenant_id,plan_id)
SELECT t.id,p.id FROM tenants t JOIN plans p ON p.code='staging_full';

UPDATE permission_registry SET entitlement_code='core_hr' WHERE module IN ('organisasi','karyawan','pengguna');
INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive,entitlement_code) VALUES
('dashboard.lihat','dashboard','lihat',FALSE,FALSE,NULL),('dashboard.lihat_keuangan','dashboard','lihat_keuangan',FALSE,TRUE,'payroll'),
('kontrak.lihat','kontrak','lihat',TRUE,FALSE,'core_hr'),('kontrak.buat','kontrak','buat',TRUE,FALSE,'core_hr'),('kontrak.ubah','kontrak','ubah',TRUE,FALSE,'core_hr'),('kontrak.hapus','kontrak','hapus',TRUE,TRUE,'core_hr'),('kontrak.ekspor','kontrak','ekspor',TRUE,TRUE,'core_hr'),
('mutasi.lihat','mutasi','lihat',TRUE,FALSE,'core_hr'),('mutasi.buat','mutasi','buat',TRUE,FALSE,'core_hr'),('mutasi.ubah','mutasi','ubah',TRUE,FALSE,'core_hr'),('mutasi.approve','mutasi','approve',TRUE,TRUE,'core_hr'),
('disiplin.lihat','disiplin','lihat',TRUE,FALSE,'core_hr'),('disiplin.buat','disiplin','buat',TRUE,FALSE,'core_hr'),('disiplin.ubah','disiplin','ubah',TRUE,FALSE,'core_hr'),
('terminasi.lihat','terminasi','lihat',TRUE,FALSE,'core_hr'),('terminasi.buat','terminasi','buat',TRUE,TRUE,'core_hr'),('terminasi.approve','terminasi','approve',TRUE,TRUE,'core_hr'),
('aset.lihat','aset','lihat',TRUE,FALSE,'core_hr'),('aset.buat','aset','buat',TRUE,FALSE,'core_hr'),('aset.ubah','aset','ubah',TRUE,FALSE,'core_hr'),
('absensi.lihat','absensi','lihat',TRUE,FALSE,'attendance'),('absensi.koreksi','absensi','koreksi',TRUE,TRUE,'attendance'),('absensi.approve','absensi','approve',TRUE,TRUE,'attendance'),('absensi.monitor_bawahan','absensi','monitor_bawahan',TRUE,FALSE,'attendance'),('absensi.ekspor','absensi','ekspor',TRUE,TRUE,'attendance'),
('jadwal.lihat','jadwal','lihat',TRUE,FALSE,'attendance'),('jadwal.buat','jadwal','buat',TRUE,FALSE,'attendance'),('jadwal.ubah','jadwal','ubah',TRUE,FALSE,'attendance'),('jadwal.hapus','jadwal','hapus',TRUE,TRUE,'attendance'),('jadwal.impor','jadwal','impor',TRUE,TRUE,'attendance'),('jadwal.ekspor','jadwal','ekspor',TRUE,TRUE,'attendance'),('jadwal.publikasi','jadwal','publikasi',TRUE,TRUE,'attendance'),
('cuti.lihat','cuti','lihat',TRUE,FALSE,'leave'),('cuti.buat','cuti','buat',TRUE,FALSE,'leave'),('cuti.ubah','cuti','ubah',TRUE,FALSE,'leave'),('cuti.approve','cuti','approve',TRUE,TRUE,'leave'),('cuti.kelola','cuti','kelola',TRUE,TRUE,'leave'),('cuti.ekspor','cuti','ekspor',TRUE,TRUE,'leave'),
('lembur.lihat','lembur','lihat',TRUE,FALSE,'leave'),('lembur.buat','lembur','buat',TRUE,FALSE,'leave'),('lembur.ubah','lembur','ubah',TRUE,FALSE,'leave'),('lembur.approve','lembur','approve',TRUE,TRUE,'leave'),('lembur.ekspor','lembur','ekspor',TRUE,TRUE,'leave'),
('persetujuan.lihat','persetujuan','lihat',TRUE,FALSE,'requests'),('persetujuan.delegasikan','persetujuan','delegasikan',TRUE,TRUE,'requests'),
('kasbon.lihat','kasbon','lihat',TRUE,FALSE,'requests'),('kasbon.buat','kasbon','buat',TRUE,FALSE,'requests'),('kasbon.approve','kasbon','approve',TRUE,TRUE,'requests'),('kasbon.cairkan','kasbon','cairkan',TRUE,TRUE,'requests'),('kasbon.ekspor','kasbon','ekspor',TRUE,TRUE,'requests'),
('reimbursement.lihat','reimbursement','lihat',TRUE,FALSE,'requests'),('reimbursement.buat','reimbursement','buat',TRUE,FALSE,'requests'),('reimbursement.approve','reimbursement','approve',TRUE,TRUE,'requests'),('reimbursement.bayar','reimbursement','bayar',TRUE,TRUE,'requests'),('reimbursement.ekspor','reimbursement','ekspor',TRUE,TRUE,'requests'),
('penggajian.lihat','penggajian','lihat',TRUE,TRUE,'payroll'),('penggajian.proses','penggajian','proses',TRUE,TRUE,'payroll'),('penggajian.approve','penggajian','approve',TRUE,TRUE,'payroll'),('penggajian.finalisasi','penggajian','finalisasi',TRUE,TRUE,'payroll'),('penggajian.ekspor_bank','penggajian','ekspor_bank',TRUE,TRUE,'payroll'),('penggajian.lihat_slip','penggajian','lihat_slip',TRUE,TRUE,'payroll'),('penggajian.lihat_sensitif','penggajian','lihat_sensitif',TRUE,TRUE,'payroll'),('penggajian.kelola','penggajian','kelola',TRUE,TRUE,'payroll'),
('talenta.lihat','talenta','lihat',TRUE,FALSE,'talent'),
('broadcast.lihat','broadcast','lihat',FALSE,FALSE,'communication'),('broadcast.buat','broadcast','buat',FALSE,FALSE,'communication'),('broadcast.ubah','broadcast','ubah',FALSE,FALSE,'communication'),('broadcast.kirim','broadcast','kirim',FALSE,TRUE,'communication'),('broadcast.target_semua','broadcast','target_semua',FALSE,TRUE,'communication'),('broadcast.lihat_penerima','broadcast','lihat_penerima',FALSE,TRUE,'communication'),
('notifikasi.lihat_sendiri','notifikasi','lihat_sendiri',FALSE,FALSE,'communication'),('notifikasi.kelola','notifikasi','kelola',FALSE,TRUE,'communication'),
('agenda.lihat','agenda','lihat',FALSE,FALSE,'communication'),('agenda.buat','agenda','buat',FALSE,FALSE,'communication'),('agenda.ubah','agenda','ubah',FALSE,FALSE,'communication'),
('direktori.lihat','direktori','lihat',TRUE,FALSE,'communication'),
('dokumen.lihat','dokumen','lihat',TRUE,FALSE,'documents'),('dokumen.buat','dokumen','buat',TRUE,FALSE,'documents'),('dokumen.ubah','dokumen','ubah',TRUE,FALSE,'documents'),('dokumen.hapus','dokumen','hapus',TRUE,TRUE,'documents'),('dokumen.unduh','dokumen','unduh',TRUE,TRUE,'documents'),
('laporan.lihat','laporan','lihat',TRUE,FALSE,'reports'),('laporan.ekspor','laporan','ekspor',TRUE,TRUE,'reports'),
('pengaturan.lihat','pengaturan','lihat',FALSE,FALSE,'core_hr'),('pengaturan.kelola','pengaturan','kelola',FALSE,TRUE,'core_hr'),
('audit.lihat','audit','lihat',FALSE,TRUE,'core_hr'),('audit.ekspor','audit','ekspor',FALSE,TRUE,'core_hr'),
('pengguna.buat','pengguna','buat',FALSE,TRUE,'core_hr'),('pengguna.ubah','pengguna','ubah',FALSE,TRUE,'core_hr'),('pengguna.nonaktifkan','pengguna','nonaktifkan',FALSE,TRUE,'core_hr'),('pengguna.cabut_sesi','pengguna','cabut_sesi',FALSE,TRUE,'core_hr');

INSERT INTO permission_scope_dimensions (permission_code,dimension)
SELECT p.code,d.dimension FROM permission_registry p
CROSS JOIN (SELECT 'all_tenant' dimension UNION ALL SELECT 'self' UNION ALL SELECT 'direct_reports' UNION ALL SELECT 'region' UNION ALL SELECT 'office' UNION ALL SELECT 'division' UNION ALL SELECT 'department' UNION ALL SELECT 'position') d
LEFT JOIN permission_scope_dimensions existing ON existing.permission_code=p.code AND existing.dimension=d.dimension
WHERE p.requires_scope=TRUE AND existing.permission_code IS NULL;

INSERT INTO roles (tenant_id,id,code,name,is_system)
SELECT t.id,LEFT(REPLACE(UUID(),'-',''),26),preset.code,preset.name,TRUE FROM tenants t CROSS JOIN (
 SELECT 'OWNER' code,'Owner / Direktur' name UNION ALL SELECT 'HR_MANAGER','HR Manager' UNION ALL SELECT 'HR_STAFF','HR Staff'
 UNION ALL SELECT 'KEPALA_CABANG','Kepala Cabang' UNION ALL SELECT 'REGIONAL_MANAGER','Regional Manager'
 UNION ALL SELECT 'SPV','Supervisor' UNION ALL SELECT 'FINANCE_PAYROLL','Finance / Payroll'
) preset LEFT JOIN roles r ON r.tenant_id=t.id AND r.code=preset.code WHERE r.id IS NULL;

-- Default aman: ✔ mendapat aksi rutin, ◐ hanya lihat. Aksi sensitif perlu grant eksplisit.
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r CROSS JOIN permission_registry p
WHERE (r.code='ADMIN_TENANT' AND p.module<>'panel' AND (p.module NOT IN ('penggajian','talenta') OR p.action IN ('lihat','lihat_slip')))
 OR (r.code='OWNER' AND (p.action='lihat' OR p.code IN ('panel.admin','dashboard.lihat_keuangan','penggajian.lihat_slip')))
 OR (r.code IN ('HR_MANAGER','HR_STAFF') AND (p.action='lihat' OR p.code IN ('panel.admin','pengguna.lihat','absensi.monitor_bawahan','notifikasi.kelola','cuti.kelola','pengaturan.lihat')) AND p.module NOT IN ('penggajian','talenta'))
 OR (r.code='FINANCE_PAYROLL' AND (p.module IN ('penggajian','kasbon','reimbursement') OR p.code IN ('panel.admin','dashboard.lihat','dashboard.lihat_keuangan','persetujuan.lihat','laporan.lihat')))
 OR (r.code='SPV' AND p.code IN ('panel.admin','dashboard.lihat','karyawan.lihat','absensi.lihat','absensi.monitor_bawahan','cuti.lihat','cuti.approve','lembur.lihat','persetujuan.lihat','direktori.lihat'))
 OR (r.code IN ('KEPALA_CABANG','REGIONAL_MANAGER') AND p.code IN ('panel.admin','dashboard.lihat','karyawan.lihat','absensi.lihat','absensi.monitor_bawahan','cuti.lihat','lembur.lihat','persetujuan.lihat','direktori.lihat'));

INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT rp.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),rp.role_id,rp.permission_code,
 CASE WHEN r.code='SPV' THEN 'direct_reports' ELSE 'all_tenant' END
FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id
JOIN permission_registry p ON p.code=rp.permission_code
LEFT JOIN role_scope_rules existing ON existing.tenant_id=rp.tenant_id AND existing.role_id=rp.role_id AND existing.permission_code=rp.permission_code
WHERE p.requires_scope=TRUE AND existing.id IS NULL AND r.code NOT IN ('KEPALA_CABANG','REGIONAL_MANAGER');

CREATE TABLE platform_permission_registry (
  code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  module VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  UNIQUE KEY uq_platform_permission (module,action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE platform_role_permissions (
  role_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  permission_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (role_id,permission_code),
  FOREIGN KEY (role_id) REFERENCES platform_roles(id),
  FOREIGN KEY (permission_code) REFERENCES platform_permission_registry(code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE platform_user_permission_overrides (
  user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  permission_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  decision VARCHAR(8) NOT NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (user_id,permission_code),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (permission_code) REFERENCES platform_permission_registry(code),
  CONSTRAINT ck_platform_override_decision CHECK (decision IN ('allow','deny'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE platform_user_tenants (
  user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (user_id,tenant_id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE platform_audit_logs (
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  actor_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  action VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  detail_json JSON NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  FOREIGN KEY (actor_user_id) REFERENCES users(id),
  FOREIGN KEY (target_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO platform_roles (id,code,name) VALUES
(LEFT(REPLACE(UUID(),'-',''),26),'ADMIN_PLATFORM','Admin Platform'),
(LEFT(REPLACE(UUID(),'-',''),26),'SUPPORT','Support'),
(LEFT(REPLACE(UUID(),'-',''),26),'FINANCE_PLATFORM','Finance Platform');
INSERT INTO platform_permission_registry (code,module,action) VALUES
('platform.dashboard.lihat','dashboard','lihat'),('platform.perusahaan.lihat','perusahaan','lihat'),('platform.perusahaan.kelola','perusahaan','kelola'),
('platform.paket.lihat','paket','lihat'),('platform.paket.kelola','paket','kelola'),
('platform.billing.lihat','billing','lihat'),('platform.billing.kelola','billing','kelola'),
('platform.pengguna.lihat','pengguna','lihat'),('platform.pengguna.kelola','pengguna','kelola'),
('platform.dukungan.lihat','dukungan','lihat'),('platform.dukungan.kelola','dukungan','kelola'),
('platform.analitik.lihat','analitik','lihat'),('platform.pengaturan.lihat','pengaturan','lihat'),('platform.pengaturan.kelola','pengaturan','kelola');
INSERT INTO platform_role_permissions (role_id,permission_code)
SELECT r.id,p.code FROM platform_roles r CROSS JOIN platform_permission_registry p
WHERE r.code='SUPERADMIN'
 OR (r.code='ADMIN_PLATFORM' AND p.module IN ('dashboard','perusahaan','paket','billing','dukungan','analitik','pengaturan') AND NOT (p.module IN ('billing','pengaturan') AND p.action='kelola'))
 OR (r.code='SUPPORT' AND p.code IN ('platform.dashboard.lihat','platform.perusahaan.lihat','platform.dukungan.lihat','platform.dukungan.kelola'))
 OR (r.code='FINANCE_PLATFORM' AND p.code IN ('platform.dashboard.lihat','platform.perusahaan.lihat','platform.paket.lihat','platform.billing.lihat','platform.billing.kelola','platform.analitik.lihat'));

-- +goose Down
-- Migrasi akses bersifat aditif dan sengaja tidak menyediakan rollback yang menghapus grant pengguna.
