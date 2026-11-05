-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE tenant_memberships
  ADD COLUMN access_version BIGINT UNSIGNED NOT NULL DEFAULT 1;

ALTER TABLE user_roles
  ADD COLUMN revoked_at DATETIME(6) NULL,
  ADD COLUMN revoked_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  DROP INDEX uq_user_roles_period,
  ADD KEY idx_user_roles_period (tenant_id, user_id, role_id, valid_from),
  ADD CONSTRAINT fk_user_roles_revoker FOREIGN KEY (revoked_by) REFERENCES users (id);

INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive) VALUES
  ('pengguna.lihat','pengguna','lihat',FALSE,FALSE),
  ('pengguna.kelola_akses','pengguna','kelola_akses',FALSE,TRUE);

INSERT INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r CROSS JOIN permission_registry p
WHERE r.code='ADMIN_TENANT' AND p.code IN ('pengguna.lihat','pengguna.kelola_akses');

-- +goose Down
DELETE FROM role_permissions WHERE permission_code IN ('pengguna.lihat','pengguna.kelola_akses');
DELETE FROM permission_registry WHERE code IN ('pengguna.lihat','pengguna.kelola_akses');
ALTER TABLE user_roles
  DROP FOREIGN KEY fk_user_roles_revoker,
  DROP INDEX idx_user_roles_period,
  DROP COLUMN revoked_by,
  DROP COLUMN revoked_at,
  ADD UNIQUE KEY uq_user_roles_period (tenant_id,user_id,role_id,valid_from);
ALTER TABLE tenant_memberships DROP COLUMN access_version;
