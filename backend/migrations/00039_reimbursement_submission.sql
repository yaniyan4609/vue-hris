-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX idx_reimbursement_duplicate
  ON reimbursements (tenant_id,employee_id,duplicate_hash);

CREATE UNIQUE INDEX uq_reimbursement_receipt_file
  ON reimbursement_receipts (tenant_id,file_id);

INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'reimbursement.buat' FROM roles WHERE code='KARYAWAN';

INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT r.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),r.id,'reimbursement.buat','self'
FROM roles r
JOIN role_permissions p ON p.tenant_id=r.tenant_id AND p.role_id=r.id AND p.permission_code='reimbursement.buat'
LEFT JOIN role_scope_rules s ON s.tenant_id=r.tenant_id AND s.role_id=r.id AND s.permission_code='reimbursement.buat' AND s.kind='self'
WHERE r.code='KARYAWAN' AND s.id IS NULL;

-- +goose Down
-- Claims and role grants may have changed after migration.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00039';
