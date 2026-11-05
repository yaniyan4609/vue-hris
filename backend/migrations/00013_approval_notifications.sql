-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE requests (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_type VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  subject_employee_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  requested_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ref_table VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ref_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  submitted_at DATETIME(6) NULL,
  final_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  KEY idx_requests_subject (tenant_id,subject_employee_id,created_at),
  KEY idx_requests_actor (tenant_id,requested_by,status,created_at),
  CONSTRAINT fk_requests_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT fk_requests_subject FOREIGN KEY (tenant_id,subject_employee_id) REFERENCES employees (tenant_id,id),
  CONSTRAINT fk_requests_actor FOREIGN KEY (tenant_id,requested_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_requests_status CHECK (status IN ('draft','submitted','in_review','approved','rejected','cancelled')),
  CONSTRAINT ck_requests_ref CHECK ((ref_table IS NULL AND ref_id IS NULL) OR (ref_table IS NOT NULL AND ref_id IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE approval_flows (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_type VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(160) NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  valid_from DATE NOT NULL,
  valid_to DATE NULL,
  fallback_role_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  KEY idx_approval_flow_lookup (tenant_id,request_type,is_default,valid_from,valid_to),
  CONSTRAINT fk_approval_flow_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT fk_approval_flow_role FOREIGN KEY (tenant_id,fallback_role_id) REFERENCES roles (tenant_id,id),
  CONSTRAINT ck_approval_flow_dates CHECK (valid_to IS NULL OR valid_to >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE approval_flow_steps (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  flow_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  level_no SMALLINT UNSIGNED NOT NULL,
  approver_kind VARCHAR(24) NOT NULL,
  approver_ref_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  required_permission VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  sla_hours SMALLINT UNSIGNED NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_approval_step_level (tenant_id,flow_id,level_no),
  CONSTRAINT fk_approval_step_flow FOREIGN KEY (tenant_id,flow_id) REFERENCES approval_flows (tenant_id,id),
  CONSTRAINT fk_approval_step_permission FOREIGN KEY (required_permission) REFERENCES permission_registry (code),
  CONSTRAINT ck_approval_step_kind CHECK (approver_kind IN ('manager','role','user','fallback_role')),
  CONSTRAINT ck_approval_step_level CHECK (level_no > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE approval_instances (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  flow_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  current_level SMALLINT UNSIGNED NOT NULL DEFAULT 1,
  status VARCHAR(24) NOT NULL DEFAULT 'in_review',
  version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  started_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  finished_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_approval_instance_request (tenant_id,request_id),
  CONSTRAINT fk_approval_instance_request FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id),
  CONSTRAINT fk_approval_instance_flow FOREIGN KEY (tenant_id,flow_id) REFERENCES approval_flows (tenant_id,id),
  CONSTRAINT ck_approval_instance_status CHECK (status IN ('in_review','approved','rejected','cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE approval_tasks (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  instance_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  level_no SMALLINT UNSIGNED NOT NULL,
  assignee_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  decided_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_approval_task_assignee (tenant_id,instance_id,level_no,assignee_user_id),
  KEY idx_approval_inbox (tenant_id,assignee_user_id,status,created_at),
  CONSTRAINT fk_approval_task_instance FOREIGN KEY (tenant_id,instance_id) REFERENCES approval_instances (tenant_id,id),
  CONSTRAINT fk_approval_task_member FOREIGN KEY (tenant_id,assignee_user_id) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_approval_task_status CHECK (status IN ('pending','approved','rejected','skipped','cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE approval_actions (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  instance_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  level_no SMALLINT UNSIGNED NOT NULL,
  actor_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(24) NOT NULL,
  reason TEXT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_approval_action_task (tenant_id,task_id),
  KEY idx_approval_actions_instance (tenant_id,instance_id,created_at),
  CONSTRAINT fk_approval_action_instance FOREIGN KEY (tenant_id,instance_id) REFERENCES approval_instances (tenant_id,id),
  CONSTRAINT fk_approval_action_task FOREIGN KEY (tenant_id,task_id) REFERENCES approval_tasks (tenant_id,id),
  CONSTRAINT fk_approval_action_actor FOREIGN KEY (tenant_id,actor_user_id) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_approval_action CHECK (action IN ('approve','reject'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_outbox (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  aggregate_type VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  aggregate_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payload_json JSON NOT NULL,
  dedupe_key VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  attempts SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(6) NULL,
  processed_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_outbox_dedupe (tenant_id,dedupe_key),
  KEY idx_outbox_pending (status,next_attempt_at,created_at),
  CONSTRAINT fk_outbox_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT ck_outbox_status CHECK (status IN ('pending','processing','processed','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notifications (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recipient_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  category VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  title VARCHAR(255) NOT NULL,
  body TEXT NOT NULL,
  deep_link VARCHAR(512) NULL,
  payload_json JSON NULL,
  dedupe_key VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  priority VARCHAR(12) NOT NULL DEFAULT 'normal',
  read_at DATETIME(6) NULL,
  archived_at DATETIME(6) NULL,
  obsolete_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_notifications_recipient_dedupe (tenant_id,recipient_user_id,dedupe_key),
  KEY idx_notifications_inbox (tenant_id,recipient_user_id,created_at,id),
  CONSTRAINT fk_notifications_recipient FOREIGN KEY (tenant_id,recipient_user_id) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_notifications_priority CHECK (priority IN ('low','normal','high','urgent'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Approval and outbox records are retained for audit; a destructive rollback
-- requires an explicit reviewed migration.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00013';
