-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE approval_actions ADD COLUMN data_json JSON NULL;

-- +goose Down
ALTER TABLE approval_actions DROP COLUMN data_json;
