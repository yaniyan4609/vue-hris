-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE employee_loans ADD COLUMN closed_at DATETIME(6) NULL;

ALTER TABLE loan_installments
  ADD COLUMN waived_at DATETIME(6) NULL,
  ADD COLUMN waive_reason VARCHAR(2000) NULL,
  ADD COLUMN waived_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD CONSTRAINT fk_loan_installment_waiver_actor FOREIGN KEY (tenant_id,waived_by) REFERENCES tenant_memberships (tenant_id,user_id);

CREATE TABLE loan_settlements (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  loan_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  amount DECIMAL(18,2) NOT NULL,
  settlement_ref VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recorded_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  settled_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_loan_settlement_loan (tenant_id,loan_id),
  UNIQUE KEY uq_loan_settlement_ref (tenant_id,settlement_ref),
  FOREIGN KEY (tenant_id,loan_id) REFERENCES employee_loans (tenant_id,id),
  FOREIGN KEY (tenant_id,recorded_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_loan_settlement_amount CHECK (amount > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Repayment history must be retained.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00041';
