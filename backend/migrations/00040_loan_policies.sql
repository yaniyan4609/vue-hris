-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE loan_policies (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  max_outstanding DECIMAL(18,2) NOT NULL,
  max_tenor_months SMALLINT UNSIGNED NOT NULL,
  version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  updated_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  FOREIGN KEY (tenant_id,updated_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_loan_policy_limit CHECK (max_outstanding > 0 AND max_tenor_months BETWEEN 1 AND 120)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_employee_loans_exposure ON employee_loans (tenant_id,employee_id,status,outstanding);
CREATE INDEX idx_loan_installments_due ON loan_installments (tenant_id,due_period,status);

INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive,entitlement_code)
VALUES ('kasbon.kelola','kasbon','kelola',FALSE,TRUE,'requests');

INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'kasbon.kelola' FROM roles WHERE code IN ('ADMIN_TENANT','FINANCE_PAYROLL');

INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'kasbon.buat' FROM roles WHERE code='KARYAWAN';

INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT r.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),r.id,'kasbon.buat','self'
FROM roles r
JOIN role_permissions p ON p.tenant_id=r.tenant_id AND p.role_id=r.id AND p.permission_code='kasbon.buat'
LEFT JOIN role_scope_rules s ON s.tenant_id=r.tenant_id AND s.role_id=r.id AND s.permission_code='kasbon.buat' AND s.kind='self'
WHERE r.code='KARYAWAN' AND s.id IS NULL;

-- +goose Down
-- Policies and grants may have been used after migration.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00040';
