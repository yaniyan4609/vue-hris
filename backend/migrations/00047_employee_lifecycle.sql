-- +goose NO TRANSACTION
-- +goose Up
INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive,entitlement_code)
VALUES ('karyawan.hapus','karyawan','hapus',TRUE,TRUE,'core_hr');

INSERT INTO permission_scope_dimensions (permission_code,dimension)
SELECT 'karyawan.hapus',dimension FROM permission_scope_dimensions WHERE permission_code='karyawan.ubah';

INSERT INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'karyawan.hapus' FROM roles WHERE code='ADMIN_TENANT';

INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT tenant_id,LEFT(REPLACE(UUID(),'-',''),26),id,'karyawan.hapus','all_tenant'
FROM roles WHERE code='ADMIN_TENANT';

ALTER TABLE employees
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  ADD KEY idx_employees_archive (tenant_id,deleted_at,status);

ALTER TABLE employee_bank_accounts
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN revoked_at DATETIME(6) NULL,
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  ADD KEY idx_bank_active (tenant_id,employee_id,revoked_at,valid_from,valid_to);

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00047';
