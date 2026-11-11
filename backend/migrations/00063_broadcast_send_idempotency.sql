-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE broadcasts
  ADD COLUMN send_idempotency_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00063';
