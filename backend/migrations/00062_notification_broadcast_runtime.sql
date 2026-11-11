-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE notification_deliveries
  ADD COLUMN attempt_no SMALLINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN next_attempt_at DATETIME(6) NULL,
  ADD COLUMN delivered_at DATETIME(6) NULL,
  ADD KEY idx_delivery_retry (status,next_attempt_at,attempted_at);

ALTER TABLE broadcasts
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN processed_count INT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN failed_count INT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN completed_at DATETIME(6) NULL,
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  ADD KEY idx_broadcast_due (status,send_at);

CREATE TABLE notification_stream_events (
  sequence_no BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recipient_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  notification_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  payload_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (sequence_no),
  KEY idx_stream_recipient (tenant_id,recipient_user_id,sequence_no),
  CONSTRAINT fk_stream_member FOREIGN KEY (tenant_id,recipient_user_id) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT fk_stream_notification FOREIGN KEY (tenant_id,notification_id) REFERENCES notifications (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00062';
