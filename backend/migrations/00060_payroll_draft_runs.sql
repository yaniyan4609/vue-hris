-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE payroll_runs
  ADD COLUMN created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN idempotency_key CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN calculation_state VARCHAR(24) NOT NULL DEFAULT 'not_started',
  ADD COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD UNIQUE KEY uq_payroll_run_idempotency (tenant_id,period_id,idempotency_key),
  ADD KEY idx_payroll_run_created (tenant_id,created_at),
  ADD CONSTRAINT fk_payroll_run_creator FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  ADD CONSTRAINT ck_payroll_calculation_state CHECK (calculation_state IN ('not_started','blocked','ready','completed'));

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00060';
