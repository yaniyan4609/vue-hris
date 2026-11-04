-- +goose NO TRANSACTION
-- +goose Up

CREATE TABLE tenants (
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    slug VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(200) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'pending_verification',
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Jakarta',
    idle_timeout_minutes SMALLINT UNSIGNED NOT NULL DEFAULT 60,
    remember_admin_allowed BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_tenants_slug (slug),
    CONSTRAINT ck_tenants_status CHECK (status IN ('pending_verification', 'trial', 'active', 'grace', 'suspended', 'stopped')),
    CONSTRAINT ck_tenants_idle CHECK (idle_timeout_minutes BETWEEN 5 AND 1440)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE users (
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    email VARCHAR(320) NOT NULL,
    email_normalized VARCHAR(320) NOT NULL,
    display_name VARCHAR(200) NOT NULL,
    password_hash VARCHAR(255) NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_email_normalized (email_normalized),
    CONSTRAINT ck_users_status CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE tenant_memberships (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    joined_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    ended_at DATETIME(6) NULL,
    PRIMARY KEY (tenant_id, user_id),
    KEY idx_memberships_user_status (user_id, status, tenant_id),
    CONSTRAINT fk_memberships_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT fk_memberships_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT ck_memberships_status CHECK (status IN ('active', 'inactive'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE permission_registry (
    code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    module VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    action VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    is_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    requires_scope BOOLEAN NOT NULL DEFAULT TRUE,
    entitlement_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (code),
    UNIQUE KEY uq_permission_module_action (module, action),
    CONSTRAINT ck_permission_code CHECK (code = CONCAT(module, '.', action))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE permission_scope_dimensions (
    permission_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    dimension VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    PRIMARY KEY (permission_code, dimension),
    CONSTRAINT fk_permission_dimension_registry FOREIGN KEY (permission_code) REFERENCES permission_registry (code),
    CONSTRAINT ck_permission_dimension CHECK (dimension IN ('region', 'office', 'division', 'department', 'position', 'direct_reports', 'self', 'all_tenant'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE roles (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(120) NOT NULL,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, id),
    UNIQUE KEY uq_roles_code (tenant_id, code),
    CONSTRAINT fk_roles_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_roles (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    role_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    valid_from DATE NOT NULL,
    valid_until DATE NULL,
    assigned_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
    assigned_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, id),
    UNIQUE KEY uq_user_roles_period (tenant_id, user_id, role_id, valid_from),
    KEY idx_user_roles_role (tenant_id, role_id),
    CONSTRAINT fk_user_roles_member FOREIGN KEY (tenant_id, user_id) REFERENCES tenant_memberships (tenant_id, user_id),
    CONSTRAINT fk_user_roles_role FOREIGN KEY (tenant_id, role_id) REFERENCES roles (tenant_id, id),
    CONSTRAINT fk_user_roles_actor FOREIGN KEY (assigned_by) REFERENCES users (id),
    CONSTRAINT ck_user_roles_dates CHECK (valid_until IS NULL OR valid_until >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE role_permissions (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    role_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    permission_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    granted_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, role_id, permission_code),
    KEY idx_role_permissions_code (permission_code),
    CONSTRAINT fk_role_permissions_role FOREIGN KEY (tenant_id, role_id) REFERENCES roles (tenant_id, id),
    CONSTRAINT fk_role_permissions_registry FOREIGN KEY (permission_code) REFERENCES permission_registry (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE role_scope_rules (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    role_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    permission_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    kind VARCHAR(20) NOT NULL,
    PRIMARY KEY (tenant_id, id),
    KEY idx_role_scope_grant (tenant_id, role_id, permission_code),
    CONSTRAINT fk_role_scope_grant FOREIGN KEY (tenant_id, role_id, permission_code) REFERENCES role_permissions (tenant_id, role_id, permission_code),
    CONSTRAINT ck_role_scope_kind CHECK (kind IN ('all_tenant', 'self', 'direct_reports', 'filter'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE role_scope_values (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    rule_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    dimension VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    node_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    PRIMARY KEY (tenant_id, rule_id, dimension, node_id),
    CONSTRAINT fk_role_scope_values_rule FOREIGN KEY (tenant_id, rule_id) REFERENCES role_scope_rules (tenant_id, id),
    CONSTRAINT ck_role_scope_values_dimension CHECK (dimension IN ('region', 'office', 'division', 'department', 'position'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_permission_overrides (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    permission_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    decision VARCHAR(8) NOT NULL,
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, user_id, permission_code),
    KEY idx_user_overrides_code (permission_code),
    CONSTRAINT fk_user_overrides_member FOREIGN KEY (tenant_id, user_id) REFERENCES tenant_memberships (tenant_id, user_id),
    CONSTRAINT fk_user_overrides_registry FOREIGN KEY (permission_code) REFERENCES permission_registry (code),
    CONSTRAINT ck_user_overrides_decision CHECK (decision IN ('allow', 'deny'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- The header row records replacement even when it has zero rules; this means
-- an empty user scope override fails closed instead of restoring the role scope.
CREATE TABLE user_scope_overrides (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    permission_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, user_id, permission_code),
    KEY idx_user_scope_overrides_code (permission_code),
    CONSTRAINT fk_user_scope_overrides_member FOREIGN KEY (tenant_id, user_id) REFERENCES tenant_memberships (tenant_id, user_id),
    CONSTRAINT fk_user_scope_overrides_registry FOREIGN KEY (permission_code) REFERENCES permission_registry (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_scope_rules (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    permission_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    kind VARCHAR(20) NOT NULL,
    PRIMARY KEY (tenant_id, id),
    KEY idx_user_scope_header (tenant_id, user_id, permission_code),
    CONSTRAINT fk_user_scope_header FOREIGN KEY (tenant_id, user_id, permission_code) REFERENCES user_scope_overrides (tenant_id, user_id, permission_code),
    CONSTRAINT ck_user_scope_kind CHECK (kind IN ('all_tenant', 'self', 'direct_reports', 'filter'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_scope_values (
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    rule_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    dimension VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    node_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    PRIMARY KEY (tenant_id, rule_id, dimension, node_id),
    CONSTRAINT fk_user_scope_values_rule FOREIGN KEY (tenant_id, rule_id) REFERENCES user_scope_rules (tenant_id, id),
    CONSTRAINT ck_user_scope_values_dimension CHECK (dimension IN ('region', 'office', 'division', 'department', 'position'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions (
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    token_hash BINARY(32) NOT NULL,
    user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    selected_tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
    remember BOOLEAN NOT NULL DEFAULT FALSE,
    idle_timeout_minutes SMALLINT UNSIGNED NOT NULL DEFAULT 60,
    expires_at DATETIME(6) NULL,
    last_seen_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    rotated_at DATETIME(6) NULL,
    revoked_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_sessions_token_hash (token_hash),
    KEY idx_sessions_user_active (user_id, revoked_at, last_seen_at),
    KEY idx_sessions_selected_membership (selected_tenant_id, user_id),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_sessions_membership FOREIGN KEY (selected_tenant_id, user_id) REFERENCES tenant_memberships (tenant_id, user_id),
    CONSTRAINT ck_sessions_idle CHECK (idle_timeout_minutes BETWEEN 5 AND 1440)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE sessions;
DROP TABLE user_scope_values;
DROP TABLE user_scope_rules;
DROP TABLE user_scope_overrides;
DROP TABLE user_permission_overrides;
DROP TABLE role_scope_values;
DROP TABLE role_scope_rules;
DROP TABLE role_permissions;
DROP TABLE user_roles;
DROP TABLE roles;
DROP TABLE permission_scope_dimensions;
DROP TABLE permission_registry;
DROP TABLE tenant_memberships;
DROP TABLE users;
DROP TABLE tenants;
