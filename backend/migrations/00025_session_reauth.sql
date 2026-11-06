-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE sessions
  ADD COLUMN reauthenticated_at DATETIME(6) NULL;

CREATE TABLE user_preferences (
  user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  theme_mode VARCHAR(10) NOT NULL DEFAULT 'system',
  theme_color VARCHAR(20) NOT NULL DEFAULT 'default',
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (user_id),
  CONSTRAINT fk_user_preferences_user FOREIGN KEY (user_id) REFERENCES users (id),
  CONSTRAINT ck_user_preferences_theme_mode CHECK (theme_mode IN ('system','light','dark'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00025';
