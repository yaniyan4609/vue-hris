-- +goose NO TRANSACTION
-- +goose Up

CREATE TABLE platform_roles (
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(120) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_platform_roles_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE platform_user_roles (
    user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    role_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    valid_from DATE NOT NULL,
    valid_until DATE NULL,
    PRIMARY KEY (user_id, role_id, valid_from),
    KEY idx_platform_user_roles_role (role_id),
    CONSTRAINT fk_platform_user_roles_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_platform_user_roles_role FOREIGN KEY (role_id) REFERENCES platform_roles (id),
    CONSTRAINT ck_platform_user_roles_dates CHECK (valid_until IS NULL OR valid_until >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE regions (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(160) NOT NULL,
    PRIMARY KEY (tenant_id, id),
    UNIQUE KEY uq_regions_code (tenant_id, code),
    CONSTRAINT fk_regions_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE offices (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    region_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(160) NOT NULL,
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Jakarta',
    PRIMARY KEY (tenant_id, id),
    UNIQUE KEY uq_offices_code (tenant_id, code),
    KEY idx_offices_region (tenant_id, region_id),
    CONSTRAINT fk_offices_region FOREIGN KEY (tenant_id, region_id) REFERENCES regions (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE divisions (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(160) NOT NULL,
    PRIMARY KEY (tenant_id, id),
    UNIQUE KEY uq_divisions_code (tenant_id, code),
    CONSTRAINT fk_divisions_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE departments (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    division_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(160) NOT NULL,
    PRIMARY KEY (tenant_id, id),
    UNIQUE KEY uq_departments_code (tenant_id, code),
    KEY idx_departments_division (tenant_id, division_id),
    CONSTRAINT fk_departments_division FOREIGN KEY (tenant_id, division_id) REFERENCES divisions (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE positions (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    department_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    code VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(160) NOT NULL,
    PRIMARY KEY (tenant_id, id),
    UNIQUE KEY uq_positions_code (tenant_id, code),
    KEY idx_positions_department (tenant_id, department_id),
    CONSTRAINT fk_positions_department FOREIGN KEY (tenant_id, department_id) REFERENCES departments (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employees (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
    employee_number VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(200) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'active',
    base_office_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    position_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    reports_to_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, id),
    UNIQUE KEY uq_employees_number (tenant_id, employee_number),
    UNIQUE KEY uq_employees_user (tenant_id, user_id),
    KEY idx_employees_office (tenant_id, base_office_id, status),
    KEY idx_employees_position (tenant_id, position_id, status),
    KEY idx_employees_reports_to (tenant_id, reports_to_id),
    CONSTRAINT fk_employees_member FOREIGN KEY (tenant_id, user_id) REFERENCES tenant_memberships (tenant_id, user_id),
    CONSTRAINT fk_employees_office FOREIGN KEY (tenant_id, base_office_id) REFERENCES offices (tenant_id, id),
    CONSTRAINT fk_employees_position FOREIGN KEY (tenant_id, position_id) REFERENCES positions (tenant_id, id),
    CONSTRAINT fk_employees_manager FOREIGN KEY (tenant_id, reports_to_id) REFERENCES employees (tenant_id, id),
    CONSTRAINT ck_employees_status CHECK (status IN ('active', 'probation', 'leave', 'resigned', 'terminated'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO permission_registry (code, module, action, requires_scope)
VALUES ('panel.admin', 'panel', 'admin', FALSE);

-- +goose Down
DELETE FROM permission_registry WHERE code = 'panel.admin';
DROP TABLE employees;
DROP TABLE positions;
DROP TABLE departments;
DROP TABLE divisions;
DROP TABLE offices;
DROP TABLE regions;
DROP TABLE platform_user_roles;
DROP TABLE platform_roles;
