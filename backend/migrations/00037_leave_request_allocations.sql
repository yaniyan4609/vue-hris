-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE leave_request_allocations (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  leave_request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  period_year SMALLINT UNSIGNED NOT NULL,
  first_work_date DATE NOT NULL,
  days DECIMAL(7,2) NOT NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_leave_request_year (tenant_id,leave_request_id,period_year),
  KEY idx_leave_allocation_year (tenant_id,period_year),
  CONSTRAINT fk_leave_allocation_request FOREIGN KEY (tenant_id,leave_request_id) REFERENCES leave_requests (tenant_id,id),
  CONSTRAINT ck_leave_allocation_days CHECK (days > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'cuti.buat' FROM roles WHERE code='KARYAWAN';

INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT r.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),r.id,'cuti.buat','self'
FROM roles r
JOIN role_permissions p ON p.tenant_id=r.tenant_id AND p.role_id=r.id AND p.permission_code='cuti.buat'
LEFT JOIN role_scope_rules s ON s.tenant_id=r.tenant_id AND s.role_id=r.id AND s.permission_code='cuti.buat' AND s.kind='self'
WHERE r.code='KARYAWAN' AND s.id IS NULL;

-- +goose Down
-- Ledger and role grants may have changed after migration. Use a reviewed compensating migration.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00037';
