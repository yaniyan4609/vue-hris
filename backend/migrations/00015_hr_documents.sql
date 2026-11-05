-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE employee_families (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  relation VARCHAR(32) NOT NULL,
  name VARCHAR(200) NOT NULL,
  birth_date DATE NULL,
  is_dependent BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (tenant_id,id),
  KEY idx_family_employee (tenant_id,employee_id),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employee_emergency_contacts (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(200) NOT NULL,
  relation VARCHAR(32) NOT NULL,
  phone VARCHAR(32) NOT NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_emergency_employee (tenant_id,employee_id),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE contracts (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_type VARCHAR(32) NOT NULL,
  start_date DATE NOT NULL,
  end_date DATE NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  KEY idx_contract_expiry (tenant_id,end_date,status),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,file_id) REFERENCES files (tenant_id,id),
  CONSTRAINT ck_contract_date CHECK (end_date IS NULL OR end_date >= start_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employee_changes (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  change_type VARCHAR(32) NOT NULL,
  effective_date DATE NOT NULL,
  new_office_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  new_division_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  new_department_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  new_position_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  new_reports_to_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  applied_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_employee_changes_date (tenant_id,employee_id,effective_date),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,new_office_id) REFERENCES offices (tenant_id,id),
  FOREIGN KEY (tenant_id,new_division_id) REFERENCES divisions (tenant_id,id),
  FOREIGN KEY (tenant_id,new_department_id) REFERENCES departments (tenant_id,id),
  FOREIGN KEY (tenant_id,new_position_id) REFERENCES positions (tenant_id,id),
  FOREIGN KEY (tenant_id,new_reports_to_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE disciplinary_actions (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  severity VARCHAR(32) NOT NULL,
  issued_date DATE NOT NULL,
  valid_until DATE NULL,
  description TEXT NOT NULL,
  file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_discipline_employee (tenant_id,employee_id,issued_date),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,file_id) REFERENCES files (tenant_id,id),
  CONSTRAINT ck_discipline_dates CHECK (valid_until IS NULL OR valid_until >= issued_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE terminations (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  termination_type VARCHAR(32) NOT NULL,
  notice_date DATE NULL,
  effective_date DATE NOT NULL,
  reason TEXT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  processed_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_termination_due (tenant_id,status,effective_date),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE assets (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(160) NOT NULL,
  category VARCHAR(64) NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'available',
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_asset_code (tenant_id,code),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employee_assets (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  asset_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  issued_at DATETIME(6) NOT NULL,
  returned_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_employee_assets (tenant_id,employee_id,issued_at),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,asset_id) REFERENCES assets (tenant_id,id),
  CONSTRAINT ck_asset_dates CHECK (returned_at IS NULL OR returned_at >= issued_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE document_types (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(160) NOT NULL,
  requires_expiry BOOLEAN NOT NULL DEFAULT FALSE,
  is_mandatory BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_document_type_code (tenant_id,code),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employee_documents (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  document_type_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  issued_date DATE NULL,
  expires_at DATE NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'active',
  PRIMARY KEY (tenant_id,id),
  KEY idx_employee_document_expiry (tenant_id,expires_at,status),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,document_type_id) REFERENCES document_types (tenant_id,id),
  FOREIGN KEY (tenant_id,file_id) REFERENCES files (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE company_documents (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  title VARCHAR(255) NOT NULL,
  category VARCHAR(64) NOT NULL,
  file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  visibility VARCHAR(24) NOT NULL DEFAULT 'admin',
  published_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  FOREIGN KEY (tenant_id,file_id) REFERENCES files (tenant_id,id),
  CONSTRAINT ck_company_doc_visibility CHECK (visibility IN ('admin','all_employees'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE document_templates (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(160) NOT NULL,
  body LONGTEXT NOT NULL,
  variables_json JSON NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_document_template_code (tenant_id,code),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE agenda_events (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  title VARCHAR(255) NOT NULL,
  description TEXT NULL,
  starts_at DATETIME(6) NOT NULL,
  ends_at DATETIME(6) NOT NULL,
  all_day BOOLEAN NOT NULL DEFAULT FALSE,
  location VARCHAR(255) NULL,
  visibility VARCHAR(24) NOT NULL DEFAULT 'all_employees',
  created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_agenda_start (tenant_id,starts_at),
  FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_agenda_dates CHECK (ends_at > starts_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE import_jobs (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  job_type VARCHAR(48) NOT NULL,
  file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  total_rows INT UNSIGNED NOT NULL DEFAULT 0,
  ok_rows INT UNSIGNED NOT NULL DEFAULT 0,
  error_rows INT UNSIGNED NOT NULL DEFAULT 0,
  created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  FOREIGN KEY (tenant_id,file_id) REFERENCES files (tenant_id,id),
  FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE import_job_errors (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  job_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_no INT UNSIGNED NOT NULL,
  field_name VARCHAR(96) NULL,
  message VARCHAR(500) NOT NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_import_errors_job (tenant_id,job_id,row_no),
  FOREIGN KEY (tenant_id,job_id) REFERENCES import_jobs (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE export_jobs (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  job_type VARCHAR(48) NOT NULL,
  filters_json JSON NOT NULL,
  scope_snapshot JSON NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  row_count INT UNSIGNED NOT NULL DEFAULT 0,
  created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  expires_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  FOREIGN KEY (tenant_id,file_id) REFERENCES files (tenant_id,id),
  FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Data-bearing HR and document tables are retained on rollback.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00015';
