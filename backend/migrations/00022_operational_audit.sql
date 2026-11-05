-- +goose NO TRANSACTION
-- +goose Up
-- A globally monotonic sequence supports inbox cursors while public IDs stay
-- the existing 26-character format. Gaps are acceptable after rollbacks.
ALTER TABLE notifications
  ADD COLUMN sequence_no BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  ADD UNIQUE KEY uq_notifications_sequence (sequence_no),
  ADD KEY idx_notifications_cursor (tenant_id,recipient_user_id,sequence_no);

ALTER TABLE work_schedules
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1;

CREATE TABLE work_schedule_revisions (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  work_schedule_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  revision_no BIGINT UNSIGNED NOT NULL,
  before_json JSON NOT NULL,
  after_json JSON NOT NULL,
  actor_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reason VARCHAR(500) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_roster_revision (tenant_id,work_schedule_id,revision_no),
  FOREIGN KEY (tenant_id,work_schedule_id) REFERENCES work_schedules (tenant_id,id),
  FOREIGN KEY (tenant_id,actor_user_id) REFERENCES tenant_memberships (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE audit_logs
  ADD COLUMN support_session_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN ip VARCHAR(64) NULL,
  ADD COLUMN user_agent VARCHAR(512) NULL,
  ADD COLUMN request_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD CONSTRAINT fk_audit_support_session FOREIGN KEY (support_session_id) REFERENCES support_sessions (id);

CREATE TABLE push_subscriptions (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  endpoint_hash BINARY(32) NOT NULL,
  endpoint_enc VARBINARY(2048) NOT NULL,
  p256dh_enc VARBINARY(1024) NOT NULL,
  auth_enc VARBINARY(1024) NOT NULL,
  key_version INT UNSIGNED NOT NULL,
  user_agent VARCHAR(512) NULL,
  last_success_at DATETIME(6) NULL,
  revoked_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_push_endpoint (tenant_id,user_id,endpoint_hash),
  FOREIGN KEY (tenant_id,user_id) REFERENCES tenant_memberships (tenant_id,user_id),
  FOREIGN KEY (tenant_id,key_version) REFERENCES tenant_crypto_keys (tenant_id,key_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Inbox cursor, audit context, and roster revisions are retained.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00022';
