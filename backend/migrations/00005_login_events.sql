-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE login_events (
    id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
    tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
    event VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    ip VARBINARY(16) NULL,
    user_agent VARCHAR(300) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_login_events_user_time (user_id, created_at),
    KEY idx_login_events_tenant_time (tenant_id, created_at),
    KEY idx_login_events_event_time (event, created_at),
    CONSTRAINT fk_login_events_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_login_events_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT ck_login_events_event CHECK (event IN ('login_success','login_failure','demo_login','tenant_selected'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE login_events;
