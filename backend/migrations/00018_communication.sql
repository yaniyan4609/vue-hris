-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE notification_rules (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  config_json JSON NOT NULL,
  updated_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_notification_rule_type (tenant_id,event_type),
  FOREIGN KEY (tenant_id,updated_by) REFERENCES tenant_memberships (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_schedules (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  rule_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  subject_type VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  subject_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recipient_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  fire_at DATETIME(6) NOT NULL,
  attempt_no SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  dedupe_key VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_notification_schedule_dedupe (tenant_id,dedupe_key),
  KEY idx_notification_schedule_due (status,fire_at),
  FOREIGN KEY (tenant_id,rule_id) REFERENCES notification_rules (tenant_id,id),
  FOREIGN KEY (tenant_id,recipient_user_id) REFERENCES tenant_memberships (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_events (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  aggregate_type VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  aggregate_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  KEY idx_notification_event_aggregate (tenant_id,aggregate_type,aggregate_id),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_deliveries (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  notification_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  channel VARCHAR(24) NOT NULL,
  status VARCHAR(16) NOT NULL,
  error_code VARCHAR(96) NULL,
  attempted_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  KEY idx_delivery_notification (tenant_id,notification_id,attempted_at),
  FOREIGN KEY (tenant_id,notification_id) REFERENCES notifications (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Tenant templates are separate from platform defaults so NULL uniqueness is
-- never relied on for the global row.
CREATE TABLE notification_templates (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  channel VARCHAR(24) NOT NULL,
  title_tpl VARCHAR(255) NOT NULL,
  body_tpl TEXT NOT NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_notification_template (tenant_id,event_type,channel),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE platform_notification_templates (
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  channel VARCHAR(24) NOT NULL,
  title_tpl VARCHAR(255) NOT NULL,
  body_tpl TEXT NOT NULL,
  UNIQUE KEY uq_platform_notification_template (event_type,channel)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_preferences (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  category VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  channel VARCHAR(24) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  PRIMARY KEY (tenant_id,user_id,category,channel),
  FOREIGN KEY (tenant_id,user_id) REFERENCES tenant_memberships (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE broadcasts (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  title VARCHAR(255) NOT NULL,
  body TEXT NOT NULL,
  attachment_file_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  send_at DATETIME(6) NULL,
  combine_mode VARCHAR(8) NOT NULL DEFAULT 'or',
  recipients_count INT UNSIGNED NOT NULL DEFAULT 0,
  snapshot_at DATETIME(6) NULL,
  created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  sent_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  FOREIGN KEY (tenant_id,attachment_file_id) REFERENCES files (tenant_id,id),
  FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  FOREIGN KEY (tenant_id,sent_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_broadcast_combine CHECK (combine_mode IN ('or','and'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE broadcast_targets (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  broadcast_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_type VARCHAR(24) NOT NULL,
  ref_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (tenant_id,id),
  KEY idx_broadcast_target (tenant_id,broadcast_id),
  FOREIGN KEY (tenant_id,broadcast_id) REFERENCES broadcasts (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE broadcast_recipients (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  broadcast_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  notification_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  delivery_status VARCHAR(24) NOT NULL DEFAULT 'pending',
  read_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_broadcast_recipient (tenant_id,broadcast_id,user_id),
  FOREIGN KEY (tenant_id,broadcast_id) REFERENCES broadcasts (tenant_id,id),
  FOREIGN KEY (tenant_id,user_id) REFERENCES tenant_memberships (tenant_id,user_id),
  FOREIGN KEY (tenant_id,employee_id) REFERENCES employees (tenant_id,id),
  FOREIGN KEY (tenant_id,notification_id) REFERENCES notifications (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Message history is retained on rollback.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00018';
