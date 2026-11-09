-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE assets
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN archived_at DATETIME(6) NULL,
  ADD COLUMN created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  ADD CONSTRAINT fk_assets_creator FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id);

ALTER TABLE employee_assets
  ADD COLUMN active_flag TINYINT UNSIGNED NULL,
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN issued_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN returned_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN issue_note VARCHAR(500) NULL,
  ADD COLUMN return_note VARCHAR(500) NULL,
  ADD COLUMN return_condition VARCHAR(16) NULL,
  ADD KEY idx_asset_loan_history (tenant_id,asset_id,issued_at),
  ADD CONSTRAINT fk_asset_issued_by FOREIGN KEY (tenant_id,issued_by) REFERENCES tenant_memberships (tenant_id,user_id),
  ADD CONSTRAINT fk_asset_returned_by FOREIGN KEY (tenant_id,returned_by) REFERENCES tenant_memberships (tenant_id,user_id);
UPDATE employee_assets SET active_flag=1 WHERE returned_at IS NULL;
ALTER TABLE employee_assets
  ADD UNIQUE KEY uq_asset_active_loan (tenant_id,asset_id,active_flag),
  ADD CONSTRAINT ck_asset_active_loan CHECK ((active_flag=1 AND returned_at IS NULL) OR (active_flag IS NULL AND returned_at IS NOT NULL)),
  ADD CONSTRAINT ck_asset_return_condition CHECK (return_condition IS NULL OR return_condition IN ('good','damaged'));

INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive,entitlement_code)
VALUES ('aset.hapus','aset','hapus',TRUE,TRUE,'core_hr');
INSERT INTO permission_scope_dimensions (permission_code,dimension)
SELECT 'aset.hapus',dimension FROM permission_scope_dimensions WHERE permission_code='aset.ubah';
INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT r.tenant_id,r.id,p.code FROM roles r JOIN permission_registry p ON p.module='aset'
WHERE (r.code='ADMIN_TENANT' AND p.code='aset.hapus')
   OR (r.code IN ('HR_MANAGER','HR_STAFF') AND p.action IN ('buat','ubah'))
   OR (r.code='KEPALA_CABANG' AND p.action='lihat');
INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT rp.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),rp.role_id,rp.permission_code,'all_tenant'
FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id
LEFT JOIN role_scope_rules existing ON existing.tenant_id=rp.tenant_id AND existing.role_id=rp.role_id AND existing.permission_code=rp.permission_code
WHERE rp.permission_code IN ('aset.lihat','aset.buat','aset.ubah','aset.hapus')
  AND r.code IN ('ADMIN_TENANT','HR_MANAGER','HR_STAFF') AND existing.id IS NULL;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00055';
