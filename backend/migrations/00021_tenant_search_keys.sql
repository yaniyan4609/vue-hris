-- +goose NO TRANSACTION
-- +goose Up
-- Stable blind-index key is separate from rotating data-encryption keys.
CREATE TABLE tenant_search_keys (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  master_key_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  wrapped_search_key VARBINARY(512) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Search keys cannot be discarded while blind indexes depend on them.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00021';
