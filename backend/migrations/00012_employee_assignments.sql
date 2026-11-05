-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE regions ADD COLUMN deleted_at DATETIME(6) NULL;
ALTER TABLE divisions ADD COLUMN deleted_at DATETIME(6) NULL;
ALTER TABLE departments ADD COLUMN deleted_at DATETIME(6) NULL;
ALTER TABLE offices
  ADD COLUMN address VARCHAR(500) NULL,
  ADD COLUMN latitude DECIMAL(10,7) NULL,
  ADD COLUMN longitude DECIMAL(10,7) NULL,
  ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN deleted_at DATETIME(6) NULL,
  ADD CONSTRAINT ck_offices_lat CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
  ADD CONSTRAINT ck_offices_lng CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180);
ALTER TABLE positions
  ADD COLUMN grade VARCHAR(32) NULL,
  ADD COLUMN level_name VARCHAR(64) NULL,
  ADD COLUMN deleted_at DATETIME(6) NULL;
ALTER TABLE employees
  ADD COLUMN nickname VARCHAR(120) NULL,
  ADD COLUMN gender VARCHAR(16) NULL,
  ADD COLUMN birth_date DATE NULL,
  ADD COLUMN birth_place VARCHAR(160) NULL,
  ADD COLUMN marital_status VARCHAR(32) NULL,
  ADD COLUMN work_email VARCHAR(320) NULL,
  ADD COLUMN mobile VARCHAR(32) NULL,
  ADD COLUMN join_date DATE NULL,
  ADD COLUMN end_date DATE NULL,
  ADD COLUMN photo_file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN deleted_at DATETIME(6) NULL,
  ADD CONSTRAINT fk_employees_photo FOREIGN KEY (tenant_id,photo_file_id) REFERENCES files (tenant_id,id);

CREATE TABLE employee_assignments (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  region_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  office_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  division_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  department_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  position_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reports_to_employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  grade VARCHAR(32) NULL,
  valid_from DATETIME(6) NOT NULL,
  valid_to DATETIME(6) NULL,
  current_flag TINYINT UNSIGNED NULL,
  source_history_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_assignment_current (tenant_id,employee_id,current_flag),
  UNIQUE KEY uq_assignment_source (tenant_id,source_history_id),
  KEY idx_assignment_timeline (tenant_id,employee_id,valid_from,valid_to),
  KEY idx_assignment_scope (tenant_id,office_id,department_id,valid_to),
  CONSTRAINT fk_ea_employee FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  CONSTRAINT fk_ea_region FOREIGN KEY (tenant_id,region_id) REFERENCES regions (tenant_id,id),
  CONSTRAINT fk_ea_office FOREIGN KEY (tenant_id,office_id) REFERENCES offices (tenant_id,id),
  CONSTRAINT fk_ea_division FOREIGN KEY (tenant_id,division_id) REFERENCES divisions (tenant_id,id),
  CONSTRAINT fk_ea_department FOREIGN KEY (tenant_id,department_id) REFERENCES departments (tenant_id,id),
  CONSTRAINT fk_ea_position FOREIGN KEY (tenant_id,position_id) REFERENCES positions (tenant_id,id),
  CONSTRAINT fk_ea_manager FOREIGN KEY (tenant_id,reports_to_employee_id) REFERENCES employees (tenant_id,id),
  CONSTRAINT fk_ea_history FOREIGN KEY (tenant_id,source_history_id) REFERENCES employee_assignment_history (tenant_id,id),
  CONSTRAINT fk_ea_actor FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_ea_dates CHECK (valid_to IS NULL OR valid_to >= valid_from),
  CONSTRAINT ck_ea_current CHECK ((current_flag = 1 AND valid_to IS NULL) OR (current_flag IS NULL AND valid_to IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- The existing history remains the write path while API dual-write is introduced.
INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,valid_to,current_flag,source_history_id)
SELECT h.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),h.employee_id,o.region_id,h.base_office_id,d.division_id,p.department_id,h.position_id,h.reports_to_id,p.grade,h.valid_from,h.valid_until,IF(h.valid_until IS NULL,1,NULL),h.id
FROM employee_assignment_history h
JOIN offices o ON o.tenant_id=h.tenant_id AND o.id=h.base_office_id
JOIN positions p ON p.tenant_id=h.tenant_id AND p.id=h.position_id
JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id;

-- Seed current assignment where legacy history is absent.
INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,current_flag)
SELECT e.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),e.id,o.region_id,e.base_office_id,d.division_id,p.department_id,e.position_id,e.reports_to_id,p.grade,e.created_at,1
FROM employees e
JOIN offices o ON o.tenant_id=e.tenant_id AND o.id=e.base_office_id
JOIN positions p ON p.tenant_id=e.tenant_id AND p.id=e.position_id
JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id
LEFT JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1
WHERE a.id IS NULL;

CREATE TABLE employee_allowed_offices (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  office_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  valid_from DATE NOT NULL,
  valid_to DATE NULL,
  reason VARCHAR(255) NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_allowed_office_period (tenant_id,employee_id,office_id,valid_from),
  CONSTRAINT fk_eao_employee FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  CONSTRAINT fk_eao_office FOREIGN KEY (tenant_id,office_id) REFERENCES offices (tenant_id,id),
  CONSTRAINT ck_eao_dates CHECK (valid_to IS NULL OR valid_to >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE tenant_memberships
  ADD COLUMN employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD UNIQUE KEY uq_membership_employee (tenant_id,employee_id),
  ADD CONSTRAINT fk_membership_employee FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id);
UPDATE tenant_memberships tm JOIN employees e ON e.tenant_id=tm.tenant_id AND e.user_id=tm.user_id
SET tm.employee_id=e.id;

-- +goose Down
-- Data-bearing assignment history is retained; reversal requires a reviewed
-- compensating migration after API cutover is reverted.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00012';
