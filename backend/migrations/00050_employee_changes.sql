-- +goose NO TRANSACTION
-- +goose Up
-- The legacy employee_changes table was created in 00015. Preserve its
-- destination columns and foreign keys while adding workflow safeguards.
ALTER TABLE employee_changes
  ADD COLUMN source_assignment_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN reason VARCHAR(500) NOT NULL DEFAULT '',
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN applied_assignment_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN apply_error VARCHAR(64) NULL,
  ADD COLUMN created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  ADD UNIQUE KEY uq_employee_change_request (tenant_id,request_id),
  ADD KEY idx_employee_change_active (tenant_id,employee_id,status),
  ADD KEY idx_employee_change_due (tenant_id,status,effective_date,id),
  ADD CONSTRAINT fk_employee_change_source FOREIGN KEY (tenant_id,source_assignment_id) REFERENCES employee_assignments (tenant_id,id),
  ADD CONSTRAINT fk_employee_change_actor FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  ADD CONSTRAINT fk_employee_change_applied FOREIGN KEY (tenant_id,applied_assignment_id) REFERENCES employee_assignments (tenant_id,id),
  ADD CONSTRAINT ck_employee_change_type CHECK (change_type IN ('mutasi','promosi','demosi')),
  ADD CONSTRAINT ck_employee_change_status CHECK (status IN ('draft','in_review','returned','approved','rejected','cancelled','applied'));

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00050';
