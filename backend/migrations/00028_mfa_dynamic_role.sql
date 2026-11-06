-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE sessions
  ADD COLUMN mfa_demo_exempt BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00028';
