-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE bpjs_rules
  ADD COLUMN source_reference VARCHAR(500) NULL;

-- +goose Down
-- Published financial-rule provenance is retained.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00044';
