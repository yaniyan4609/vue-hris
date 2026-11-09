-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE disciplinary_actions
  ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  ADD COLUMN cancelled_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN cancelled_at DATETIME(6) NULL,
  ADD COLUMN cancellation_reason VARCHAR(500) NULL,
  ADD KEY idx_discipline_sequence (tenant_id,employee_id,severity,status,issued_date,valid_until),
  ADD CONSTRAINT fk_discipline_creator FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  ADD CONSTRAINT fk_discipline_canceller FOREIGN KEY (tenant_id,cancelled_by) REFERENCES tenant_memberships (tenant_id,user_id),
  ADD CONSTRAINT ck_discipline_severity CHECK (severity IN ('KONSELING','SP1','SP2','SP3')),
  ADD CONSTRAINT ck_discipline_status CHECK (status IN ('active','cancelled'));

INSERT INTO permission_registry (code,module,action,requires_scope,is_sensitive,entitlement_code)
VALUES ('disiplin.hapus','disiplin','hapus',TRUE,TRUE,'core_hr');
INSERT INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'disiplin.hapus' FROM roles WHERE code='ADMIN_TENANT';
INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT tenant_id,LEFT(REPLACE(UUID(),'-',''),26),id,'disiplin.hapus','all_tenant' FROM roles WHERE code='ADMIN_TENANT';

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00052';
