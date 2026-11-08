-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE reimbursement_categories
  ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX idx_reimbursement_categories_active
  ON reimbursement_categories (tenant_id,is_active,name);

-- +goose Down
-- Existing category state and claims must be preserved.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00038';
