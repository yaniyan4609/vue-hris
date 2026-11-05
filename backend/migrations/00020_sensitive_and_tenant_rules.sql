-- +goose NO TRANSACTION
-- +goose Up
-- Wrapped tenant data keys are never stored in plaintext. A version remains
-- available for decrypting old records after key rotation.
CREATE TABLE tenant_crypto_keys (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  key_version INT UNSIGNED NOT NULL,
  master_key_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  wrapped_data_key VARBINARY(512) NOT NULL,
  status VARCHAR(16) NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  retired_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,key_version),
  UNIQUE KEY uq_tenant_active_crypto_key (tenant_id,active_slot),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT ck_tenant_crypto_status CHECK ((status='active' AND active_slot=1 AND retired_at IS NULL) OR (status='retired' AND active_slot IS NULL AND retired_at IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE employees
  ADD COLUMN nik_enc VARBINARY(512) NULL,
  ADD COLUMN nik_key_version INT UNSIGNED NULL,
  ADD COLUMN nik_hash BINARY(32) NULL,
  ADD KEY idx_employees_nik_hash (tenant_id,nik_hash),
  ADD CONSTRAINT fk_employees_nik_key FOREIGN KEY (tenant_id,nik_key_version) REFERENCES tenant_crypto_keys (tenant_id,key_version),
  ADD CONSTRAINT ck_employees_nik_cipher CHECK ((nik_enc IS NULL AND nik_key_version IS NULL AND nik_hash IS NULL) OR (nik_enc IS NOT NULL AND nik_key_version IS NOT NULL AND nik_hash IS NOT NULL));

CREATE TABLE employee_bank_accounts (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  bank_code VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_no_enc VARBINARY(512) NOT NULL,
  holder_name_enc VARBINARY(512) NOT NULL,
  key_version INT UNSIGNED NOT NULL,
  is_primary BOOLEAN NOT NULL DEFAULT FALSE,
  primary_slot TINYINT UNSIGNED NULL,
  valid_from DATE NOT NULL,
  valid_to DATE NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_primary_bank_account (tenant_id,employee_id,primary_slot),
  KEY idx_employee_bank_account (tenant_id,employee_id,valid_from),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,key_version) REFERENCES tenant_crypto_keys (tenant_id,key_version),
  CONSTRAINT ck_bank_primary CHECK ((is_primary=TRUE AND primary_slot=1) OR (is_primary=FALSE AND primary_slot IS NULL)),
  CONSTRAINT ck_bank_validity CHECK (valid_to IS NULL OR valid_to >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employee_tax_bpjs (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  npwp_enc VARBINARY(512) NULL,
  ptkp_status VARCHAR(16) NULL,
  bpjs_kes_no_enc VARBINARY(512) NULL,
  bpjs_tk_no_enc VARBINARY(512) NULL,
  key_version INT UNSIGNED NOT NULL,
  valid_from DATE NOT NULL,
  valid_to DATE NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_employee_tax_period (tenant_id,employee_id,valid_from),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,key_version) REFERENCES tenant_crypto_keys (tenant_id,key_version),
  CONSTRAINT ck_employee_tax_dates CHECK (valid_to IS NULL OR valid_to >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- One legal employer per tenant for the initial product version.
CREATE TABLE tenant_legal_profiles (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  legal_name VARCHAR(255) NOT NULL,
  registration_no VARCHAR(96) NULL,
  tax_no_enc VARBINARY(512) NULL,
  key_version INT UNSIGNED NULL,
  address TEXT NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  FOREIGN KEY (tenant_id,key_version) REFERENCES tenant_crypto_keys (tenant_id,key_version),
  CONSTRAINT ck_legal_tax_cipher CHECK ((tax_no_enc IS NULL AND key_version IS NULL) OR (tax_no_enc IS NOT NULL AND key_version IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE tax_rules (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  rule_set VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effective_from DATE NOT NULL,
  effective_to DATE NULL,
  data_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_tax_rule_version (tenant_id,rule_set,effective_from),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT ck_tax_rule_dates CHECK (effective_to IS NULL OR effective_to >= effective_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE bpjs_rules (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  program VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effective_from DATE NOT NULL,
  effective_to DATE NULL,
  rate_employee DECIMAL(9,6) NOT NULL,
  rate_employer DECIMAL(9,6) NOT NULL,
  wage_cap DECIMAL(18,2) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_bpjs_rule_version (tenant_id,program,effective_from),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT ck_bpjs_rule_dates CHECK (effective_to IS NULL OR effective_to >= effective_from),
  CONSTRAINT ck_bpjs_rule_rates CHECK (rate_employee BETWEEN 0 AND 1 AND rate_employer BETWEEN 0 AND 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Key versions and ciphertext are retained on rollback. Destructive removal
-- would make encrypted data unrecoverable.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00020';
