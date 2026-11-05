-- +goose NO TRANSACTION
-- +goose Up

INSERT INTO permission_registry (code, module, action, requires_scope) VALUES
  ('organisasi.lihat', 'organisasi', 'lihat', FALSE),
  ('organisasi.buat', 'organisasi', 'buat', FALSE),
  ('organisasi.ubah', 'organisasi', 'ubah', FALSE),
  ('karyawan.lihat', 'karyawan', 'lihat', TRUE),
  ('karyawan.buat', 'karyawan', 'buat', TRUE),
  ('karyawan.ubah', 'karyawan', 'ubah', TRUE);

INSERT INTO permission_scope_dimensions (permission_code, dimension) VALUES
  ('karyawan.lihat', 'all_tenant'), ('karyawan.lihat', 'self'), ('karyawan.lihat', 'direct_reports'),
  ('karyawan.lihat', 'region'), ('karyawan.lihat', 'office'), ('karyawan.lihat', 'division'),
  ('karyawan.lihat', 'department'), ('karyawan.lihat', 'position'),
  ('karyawan.buat', 'all_tenant'), ('karyawan.buat', 'region'), ('karyawan.buat', 'office'),
  ('karyawan.buat', 'division'), ('karyawan.buat', 'department'), ('karyawan.buat', 'position'),
  ('karyawan.ubah', 'all_tenant'), ('karyawan.ubah', 'region'), ('karyawan.ubah', 'office'),
  ('karyawan.ubah', 'division'), ('karyawan.ubah', 'department'), ('karyawan.ubah', 'position');

INSERT INTO role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code FROM roles r
CROSS JOIN permission_registry p
WHERE r.code='ADMIN_TENANT' AND p.code IN ('organisasi.lihat','organisasi.buat','organisasi.ubah','karyawan.lihat','karyawan.buat','karyawan.ubah');

CREATE TABLE audit_logs (
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    actor_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    action VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    entity_type VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    entity_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    before_json JSON NULL,
    after_json JSON NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, id),
    KEY idx_audit_entity (tenant_id, entity_type, entity_id, created_at),
    KEY idx_audit_actor (tenant_id, actor_user_id, created_at),
    CONSTRAINT fk_audit_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT fk_audit_actor FOREIGN KEY (actor_user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employee_assignment_history (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    base_office_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    position_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    reports_to_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
    valid_from DATETIME(6) NOT NULL,
    valid_until DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, id),
    KEY idx_assignment_employee (tenant_id, employee_id, valid_from),
    CONSTRAINT fk_assignment_employee FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id),
    CONSTRAINT fk_assignment_office FOREIGN KEY (tenant_id, base_office_id) REFERENCES offices (tenant_id, id),
    CONSTRAINT fk_assignment_position FOREIGN KEY (tenant_id, position_id) REFERENCES positions (tenant_id, id),
    CONSTRAINT fk_assignment_manager FOREIGN KEY (tenant_id, reports_to_id) REFERENCES employees (tenant_id, id),
    CONSTRAINT ck_assignment_dates CHECK (valid_until IS NULL OR valid_until >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE employee_assignment_history;
DROP TABLE audit_logs;
DELETE v FROM role_scope_values v JOIN role_scope_rules r ON r.tenant_id=v.tenant_id AND r.id=v.rule_id WHERE r.permission_code IN ('karyawan.lihat','karyawan.buat','karyawan.ubah');
DELETE FROM role_scope_rules WHERE permission_code IN ('karyawan.lihat','karyawan.buat','karyawan.ubah');
DELETE v FROM user_scope_values v JOIN user_scope_rules r ON r.tenant_id=v.tenant_id AND r.id=v.rule_id WHERE r.permission_code IN ('karyawan.lihat','karyawan.buat','karyawan.ubah');
DELETE FROM user_scope_rules WHERE permission_code IN ('karyawan.lihat','karyawan.buat','karyawan.ubah');
DELETE FROM user_scope_overrides WHERE permission_code IN ('karyawan.lihat','karyawan.buat','karyawan.ubah');
DELETE FROM user_permission_overrides WHERE permission_code IN ('organisasi.lihat','organisasi.buat','organisasi.ubah','karyawan.lihat','karyawan.buat','karyawan.ubah');
DELETE FROM role_permissions WHERE permission_code IN ('organisasi.lihat','organisasi.buat','organisasi.ubah','karyawan.lihat','karyawan.buat','karyawan.ubah');
DELETE FROM permission_scope_dimensions WHERE permission_code IN ('karyawan.lihat','karyawan.buat','karyawan.ubah');
DELETE FROM permission_registry WHERE code IN ('organisasi.lihat','organisasi.buat','organisasi.ubah','karyawan.lihat','karyawan.buat','karyawan.ubah');
