-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE attendance_corrections
  ADD COLUMN attachment_file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  ADD UNIQUE KEY uq_attendance_correction_request (tenant_id,request_id),
  ADD KEY idx_attendance_correction_status (tenant_id,status,created_at),
  ADD KEY idx_attendance_correction_employee (tenant_id,employee_id,created_at),
  ADD CONSTRAINT fk_attendance_correction_attachment FOREIGN KEY (tenant_id,attachment_file_id) REFERENCES files (tenant_id,id),
  ADD CONSTRAINT fk_attendance_correction_creator FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  ADD CONSTRAINT ck_attendance_correction_status CHECK (status IN ('draft','in_review','returned','approved','rejected','cancelled')),
  ADD CONSTRAINT ck_attendance_correction_type CHECK (requested_type IN ('clock_in','clock_out'));

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00058';
