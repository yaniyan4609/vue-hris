-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE pay_components
  ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6);

CREATE INDEX idx_pay_components_active ON pay_components (tenant_id,is_active,name);

-- +goose Down
-- Component history may already be referenced by employee assignments and payroll.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00042';
