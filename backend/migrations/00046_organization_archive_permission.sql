-- +goose NO TRANSACTION
-- +goose Up
INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive,entitlement_code)
VALUES ('organisasi.hapus','organisasi','hapus',FALSE,TRUE,'core_hr');

INSERT INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'organisasi.hapus' FROM roles WHERE code='ADMIN_TENANT';

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00046';
