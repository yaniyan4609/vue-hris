-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE approval_delegations (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  from_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  to_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_type VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NULL,
  valid_from DATETIME(6) NOT NULL,
  valid_to DATETIME(6) NOT NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_delegation_active (tenant_id,from_user_id,valid_from,valid_to),
  FOREIGN KEY (tenant_id,from_user_id) REFERENCES tenant_memberships (tenant_id,user_id),
  FOREIGN KEY (tenant_id,to_user_id) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_delegation_users CHECK (from_user_id <> to_user_id),
  CONSTRAINT ck_delegation_dates CHECK (valid_to > valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE attendance_corrections (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  shift_instance_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  original_event_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  requested_type VARCHAR(16) NOT NULL,
  requested_time DATETIME(6) NOT NULL,
  reason TEXT NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  result_event_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,shift_instance_id) REFERENCES shift_instances (tenant_id,id),
  FOREIGN KEY (tenant_id,original_event_id) REFERENCES attendance_events (tenant_id,id),
  FOREIGN KEY (tenant_id,result_event_id) REFERENCES attendance_events (tenant_id,id),
  FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE shift_swaps (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  requester_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  partner_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  instance_a_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  instance_b_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  FOREIGN KEY (tenant_id,requester_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,partner_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,instance_a_id) REFERENCES shift_instances (tenant_id,id),
  FOREIGN KEY (tenant_id,instance_b_id) REFERENCES shift_instances (tenant_id,id),
  FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id),
  CONSTRAINT ck_shift_swap_distinct CHECK (requester_id <> partner_id AND instance_a_id <> instance_b_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE leave_types (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(160) NOT NULL,
  is_paid BOOLEAN NOT NULL DEFAULT TRUE,
  requires_attachment BOOLEAN NOT NULL DEFAULT FALSE,
  max_consecutive_days SMALLINT UNSIGNED NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_leave_type_code (tenant_id,code),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE leave_policies (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  leave_type_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  accrual_type VARCHAR(32) NOT NULL,
  quota_days DECIMAL(7,2) NULL,
  carry_over_days DECIMAL(7,2) NULL,
  expire_months SMALLINT UNSIGNED NULL,
  eligibility_json JSON NULL,
  valid_from DATE NOT NULL,
  valid_to DATE NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_leave_policy_date (tenant_id,leave_type_id,valid_from),
  FOREIGN KEY (tenant_id,leave_type_id) REFERENCES leave_types (tenant_id,id),
  CONSTRAINT ck_leave_policy_dates CHECK (valid_to IS NULL OR valid_to >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE leave_balance_ledger (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  leave_type_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  entry_type VARCHAR(24) NOT NULL,
  days DECIMAL(7,2) NOT NULL,
  effective_date DATE NOT NULL,
  ref_type VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ref_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  note VARCHAR(500) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_leave_ledger_ref (tenant_id,employee_id,leave_type_id,entry_type,ref_type,ref_id),
  KEY idx_leave_ledger_balance (tenant_id,employee_id,leave_type_id,effective_date),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,leave_type_id) REFERENCES leave_types (tenant_id,id),
  CONSTRAINT ck_leave_ledger_days CHECK (days <> 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE leave_balances (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  leave_type_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  period_year SMALLINT UNSIGNED NOT NULL,
  balance_days DECIMAL(8,2) NOT NULL DEFAULT 0,
  computed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,employee_id,leave_type_id,period_year),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,leave_type_id) REFERENCES leave_types (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE leave_requests (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  leave_type_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  start_date DATE NOT NULL,
  end_date DATE NOT NULL,
  days DECIMAL(7,2) NOT NULL,
  reason TEXT NULL,
  attachment_file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_leave_request_overlap (tenant_id,employee_id,start_date,end_date),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,leave_type_id) REFERENCES leave_types (tenant_id,id),
  FOREIGN KEY (tenant_id,attachment_file_id) REFERENCES files (tenant_id,id),
  FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id),
  CONSTRAINT ck_leave_request_dates CHECK (end_date >= start_date AND days > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE overtime_requests (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  work_date DATE NOT NULL,
  shift_instance_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  proposed_minutes INT UNSIGNED NOT NULL,
  approved_minutes INT UNSIGNED NULL,
  paid_minutes INT UNSIGNED NULL,
  reason TEXT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_overtime_employee_date (tenant_id,employee_id,work_date),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,shift_instance_id) REFERENCES shift_instances (tenant_id,id),
  FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id),
  CONSTRAINT ck_overtime_minutes CHECK (proposed_minutes > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employee_loans (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  amount DECIMAL(18,2) NOT NULL,
  tenor_months SMALLINT UNSIGNED NOT NULL,
  installment_amount DECIMAL(18,2) NOT NULL,
  outstanding DECIMAL(18,2) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  disbursed_at DATETIME(6) NULL,
  disbursement_ref VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NULL,
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_loan_disbursement (tenant_id,disbursement_ref),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id),
  CONSTRAINT ck_loan_amounts CHECK (amount > 0 AND installment_amount > 0 AND outstanding >= 0 AND tenor_months > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE loan_installments (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  loan_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  seq SMALLINT UNSIGNED NOT NULL,
  due_period DATE NOT NULL,
  amount DECIMAL(18,2) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  payroll_item_line_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_loan_installment_seq (tenant_id,loan_id,seq),
  FOREIGN KEY (tenant_id,loan_id) REFERENCES employee_loans (tenant_id,id),
  CONSTRAINT ck_loan_installment_amount CHECK (amount > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE reimbursement_categories (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(160) NOT NULL,
  monthly_limit DECIMAL(18,2) NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_reimbursement_category (tenant_id,code),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE reimbursements (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  category_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  trx_date DATE NOT NULL,
  amount DECIMAL(18,2) NOT NULL,
  description TEXT NULL,
  duplicate_hash BINARY(32) NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  payment_ref VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NULL,
  paid_at DATETIME(6) NULL,
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_reimbursement_payment (tenant_id,payment_ref),
  KEY idx_reimbursement_employee_date (tenant_id,employee_id,trx_date),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,category_id) REFERENCES reimbursement_categories (tenant_id,id),
  FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id),
  CONSTRAINT ck_reimbursement_amount CHECK (amount > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE reimbursement_receipts (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reimbursement_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_reimbursement_receipt (tenant_id,reimbursement_id,file_id),
  FOREIGN KEY (tenant_id,reimbursement_id) REFERENCES reimbursements (tenant_id,id),
  FOREIGN KEY (tenant_id,file_id) REFERENCES files (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Financial and approval history is retained on rollback.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00016';
