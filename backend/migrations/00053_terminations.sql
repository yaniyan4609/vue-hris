-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE terminations
  ADD COLUMN source_assignment_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN apply_error VARCHAR(64) NULL,
  ADD COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  ADD COLUMN cancelled_at DATETIME(6) NULL,
  ADD UNIQUE KEY uq_termination_request (tenant_id,request_id),
  ADD KEY idx_termination_employee_status (tenant_id,employee_id,status),
  ADD CONSTRAINT fk_termination_source FOREIGN KEY (tenant_id,source_assignment_id) REFERENCES employee_assignments (tenant_id,id),
  ADD CONSTRAINT fk_termination_creator FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  ADD CONSTRAINT ck_termination_type CHECK (termination_type IN ('resign','dismissal','retirement')),
  ADD CONSTRAINT ck_termination_status CHECK (status IN ('draft','in_review','returned','approved','rejected','cancelled','processed'));

CREATE TABLE termination_checklist (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  termination_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  item_code VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  is_done BOOLEAN NOT NULL DEFAULT FALSE,
  note VARCHAR(500) NULL,
  updated_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,termination_id,item_code),
  CONSTRAINT fk_exit_item_termination FOREIGN KEY (tenant_id,termination_id) REFERENCES terminations (tenant_id,id),
  CONSTRAINT fk_exit_item_actor FOREIGN KEY (tenant_id,updated_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_exit_item_code CHECK (item_code IN ('assets_returned','handover_complete'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive,entitlement_code)
VALUES ('terminasi.ubah','terminasi','ubah',TRUE,TRUE,'core_hr');
INSERT INTO permission_scope_dimensions (permission_code,dimension)
SELECT 'terminasi.ubah',dimension FROM permission_scope_dimensions WHERE permission_code='terminasi.buat';
INSERT INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'terminasi.ubah' FROM roles WHERE code='ADMIN_TENANT';
INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT tenant_id,LEFT(REPLACE(UUID(),'-',''),26),id,'terminasi.ubah','all_tenant' FROM roles WHERE code='ADMIN_TENANT';

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00053';
