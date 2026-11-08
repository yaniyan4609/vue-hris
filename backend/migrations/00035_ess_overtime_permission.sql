-- +goose NO TRANSACTION
-- +goose Up
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'lembur.buat' FROM roles WHERE code='KARYAWAN';

-- +goose Down
-- Role grants may have been modified after migration; use a reviewed compensating migration.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00035';
