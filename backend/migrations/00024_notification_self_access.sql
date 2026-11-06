-- +goose NO TRANSACTION
-- +goose Up
-- Every tenant role may read only its holder's own inbox. User Deny overrides
-- and package entitlements are still evaluated by the access service.
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'notifikasi.lihat_sendiri' FROM roles;

-- +goose Down
-- A later administrator may have changed role grants, so do not remove them.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00024';
