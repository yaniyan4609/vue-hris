-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE sessions
  ADD COLUMN user_agent VARCHAR(300) NULL,
  ADD COLUMN ip VARBINARY(16) NULL,
  ADD COLUMN device_label VARCHAR(100) NULL,
  ADD COLUMN revoked_reason VARCHAR(40) NULL;

-- +goose Down
ALTER TABLE sessions
  DROP COLUMN revoked_reason,
  DROP COLUMN device_label,
  DROP COLUMN ip,
  DROP COLUMN user_agent;
