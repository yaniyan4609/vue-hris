-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE sessions
  ADD COLUMN mfa_required BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN mfa_verified_at DATETIME(6) NULL,
  ADD COLUMN mfa_failures TINYINT UNSIGNED NOT NULL DEFAULT 0;

CREATE TABLE platform_mfa_credentials (
  user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  secret_nonce VARBINARY(12) NOT NULL,
  secret_ciphertext VARBINARY(64) NOT NULL,
  confirmed_at DATETIME(6) NULL,
  last_counter BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (user_id),
  CONSTRAINT fk_platform_mfa_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE platform_mfa_recovery_codes (
  user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  code_hash BINARY(32) NOT NULL,
  used_at DATETIME(6) NULL,
  PRIMARY KEY (user_id,code_hash),
  CONSTRAINT fk_platform_recovery_user FOREIGN KEY (user_id) REFERENCES platform_mfa_credentials (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00026';
