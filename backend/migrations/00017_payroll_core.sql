-- +goose NO TRANSACTION
-- +goose Up
-- Storage only. Tax/BPJS calculation and finalization remain disabled until
-- rule ownership, legal entity, and approval policy are specified.
CREATE TABLE pay_components (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(160) NOT NULL,
  component_type VARCHAR(24) NOT NULL,
  taxable BOOLEAN NOT NULL DEFAULT FALSE,
  calc_type VARCHAR(24) NOT NULL,
  is_system BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_pay_component_code (tenant_id,code),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT ck_pay_component_type CHECK (component_type IN ('earning','deduction','employer'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employee_pay_components (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  component_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  amount DECIMAL(18,2) NOT NULL,
  valid_from DATE NOT NULL,
  valid_to DATE NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_employee_pay_component_period (tenant_id,employee_id,component_id,valid_from),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,component_id) REFERENCES pay_components (tenant_id,id),
  CONSTRAINT ck_employee_pay_component_dates CHECK (valid_to IS NULL OR valid_to >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE payroll_periods (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  period_year SMALLINT UNSIGNED NOT NULL,
  period_month TINYINT UNSIGNED NOT NULL,
  start_date DATE NOT NULL,
  end_date DATE NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'open',
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_payroll_period_month (tenant_id,period_year,period_month),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT ck_payroll_month CHECK (period_month BETWEEN 1 AND 12),
  CONSTRAINT ck_payroll_period_dates CHECK (end_date >= start_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE payroll_runs (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  period_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_no SMALLINT UNSIGNED NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  rules_snapshot JSON NULL,
  total_gross DECIMAL(20,2) NOT NULL DEFAULT 0,
  total_net DECIMAL(20,2) NOT NULL DEFAULT 0,
  calculated_at DATETIME(6) NULL,
  finalized_at DATETIME(6) NULL,
  finalized_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_payroll_run_no (tenant_id,period_id,run_no),
  FOREIGN KEY (tenant_id,period_id) REFERENCES payroll_periods (tenant_id,id),
  FOREIGN KEY (tenant_id,finalized_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_payroll_run_status CHECK (status IN ('draft','calculated','in_review','finalized','cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE payroll_items (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  gross DECIMAL(18,2) NOT NULL,
  total_deduction DECIMAL(18,2) NOT NULL,
  tax DECIMAL(18,2) NOT NULL,
  net DECIMAL(18,2) NOT NULL,
  employee_snapshot JSON NOT NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_payroll_item_employee (tenant_id,run_id,employee_id),
  FOREIGN KEY (tenant_id,run_id) REFERENCES payroll_runs (tenant_id,id),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE payroll_item_lines (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  item_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  component_code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  line_type VARCHAR(24) NOT NULL,
  amount DECIMAL(18,2) NOT NULL,
  source_type VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NULL,
  source_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  description VARCHAR(500) NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_payroll_lines_item (tenant_id,item_id),
  FOREIGN KEY (tenant_id,item_id) REFERENCES payroll_items (tenant_id,id),
  CONSTRAINT ck_payroll_line_source CHECK ((source_type IS NULL AND source_id IS NULL) OR (source_type IS NOT NULL AND source_id IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE loan_installments
  ADD CONSTRAINT fk_loan_installment_payroll_line FOREIGN KEY (tenant_id,payroll_item_line_id) REFERENCES payroll_item_lines (tenant_id,id);

CREATE TABLE payroll_adjustments (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reason TEXT NOT NULL,
  amount DECIMAL(18,2) NOT NULL,
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  FOREIGN KEY (tenant_id,run_id) REFERENCES payroll_runs (tenant_id,id),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id),
  FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_payroll_adjustment_amount CHECK (amount <> 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE payslips (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payroll_item_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  published_at DATETIME(6) NULL,
  viewed_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_payslip_item (tenant_id,payroll_item_id),
  FOREIGN KEY (tenant_id,payroll_item_id) REFERENCES payroll_items (tenant_id,id),
  FOREIGN KEY (tenant_id,file_id) REFERENCES files (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Payroll financial history is retained on rollback.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00017';
