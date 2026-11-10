-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE leave_accrual_runs (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  period_year SMALLINT UNSIGNED NOT NULL,
  as_of_date DATE NOT NULL,
  source VARCHAR(16) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  processed_count INT UNSIGNED NOT NULL DEFAULT 0,
  credited_count INT UNSIGNED NOT NULL DEFAULT 0,
  started_at DATETIME(6) NULL,
  finished_at DATETIME(6) NULL,
  lease_until DATETIME(6) NULL,
  last_error VARCHAR(500) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_leave_accrual_day (tenant_id,period_year,as_of_date),
  KEY idx_leave_accrual_pending (status,lease_until,created_at),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_leave_accrual_run_source CHECK (source IN ('manual','automatic')),
  CONSTRAINT ck_leave_accrual_run_status CHECK (status IN ('pending','running','completed','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE leave_accrual_grants (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  leave_type_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  period_year SMALLINT UNSIGNED NOT NULL,
  policy_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ledger_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  days DECIMAL(7,2) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_leave_grant_year (tenant_id,employee_id,leave_type_id,period_year),
  UNIQUE KEY uq_leave_grant_ledger (tenant_id,ledger_id),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,leave_type_id) REFERENCES leave_types (tenant_id,id),
  FOREIGN KEY (tenant_id,policy_id) REFERENCES leave_policies (tenant_id,id),
  FOREIGN KEY (tenant_id,run_id) REFERENCES leave_accrual_runs (tenant_id,id),
  FOREIGN KEY (tenant_id,ledger_id) REFERENCES leave_balance_ledger (tenant_id,id),
  CONSTRAINT ck_leave_grant_days CHECK (days > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00059';
