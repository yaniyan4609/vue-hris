-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE roles
  ADD COLUMN description VARCHAR(500) NULL,
  ADD COLUMN version INT UNSIGNED NOT NULL DEFAULT 1;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00029';
