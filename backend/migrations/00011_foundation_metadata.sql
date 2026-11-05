-- +goose NO TRANSACTION
-- +goose Up
-- Additive foundations. Existing subscription and plan rows remain authoritative.
ALTER TABLE tenants
  ADD COLUMN trial_ends_at DATETIME(6) NULL;

ALTER TABLE plans
  ADD COLUMN price_monthly DECIMAL(18,2) NULL,
  ADD COLUMN price_yearly DECIMAL(18,2) NULL,
  ADD COLUMN max_employees INT UNSIGNED NULL,
  ADD COLUMN max_users INT UNSIGNED NULL,
  ADD COLUMN max_offices INT UNSIGNED NULL,
  ADD COLUMN max_storage_mb BIGINT UNSIGNED NULL;

ALTER TABLE plan_entitlements
  ADD COLUMN limit_value BIGINT UNSIGNED NULL;

ALTER TABLE tenant_subscriptions
  ADD COLUMN billing_cycle VARCHAR(16) NULL,
  ADD COLUMN period_start DATETIME(6) NULL,
  ADD COLUMN period_end DATETIME(6) NULL,
  ADD COLUMN auto_renew BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN canceled_at DATETIME(6) NULL,
  ADD CONSTRAINT ck_subscription_period CHECK (period_end IS NULL OR period_start IS NOT NULL AND period_end > period_start),
  ADD CONSTRAINT ck_subscription_cycle CHECK (billing_cycle IS NULL OR billing_cycle IN ('monthly','yearly'));

ALTER TABLE users
  ADD COLUMN phone VARCHAR(32) NULL,
  ADD COLUMN password_changed_at DATETIME(6) NULL,
  ADD COLUMN last_login_at DATETIME(6) NULL;

CREATE TABLE tenant_settings (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  setting_key VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  value_json JSON NOT NULL,
  updated_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id, setting_key),
  CONSTRAINT fk_tenant_settings_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT fk_tenant_settings_actor FOREIGN KEY (updated_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE files (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  storage_key VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  original_name VARCHAR(255) NOT NULL,
  mime_type VARCHAR(160) NOT NULL,
  size_bytes BIGINT UNSIGNED NOT NULL,
  sha256 BINARY(32) NOT NULL,
  purpose VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  uploaded_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  scan_status VARCHAR(16) NOT NULL DEFAULT 'pending',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  deleted_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_files_storage (tenant_id,storage_key),
  KEY idx_files_purpose (tenant_id,purpose,created_at),
  CONSTRAINT fk_files_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT fk_files_uploader FOREIGN KEY (tenant_id,uploaded_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_files_scan CHECK (scan_status IN ('pending','clean','rejected','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Destructive rollback is intentionally omitted. Use a reviewed compensating
-- migration after confirming no new data uses these columns or tables.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00011';
