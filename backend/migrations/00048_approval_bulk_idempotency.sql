-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE approval_bulk_batches (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  actor_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  idempotency_key CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_hash BINARY(32) NOT NULL,
  response_json JSON NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  completed_at DATETIME(6) NULL,
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_approval_bulk_replay (tenant_id,actor_user_id,idempotency_key),
  CONSTRAINT fk_approval_bulk_actor FOREIGN KEY (tenant_id,actor_user_id) REFERENCES tenant_memberships (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE approval_bulk_items (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  batch_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  item_index SMALLINT UNSIGNED NOT NULL,
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status_code SMALLINT UNSIGNED NOT NULL,
  response_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,batch_id,item_index),
  KEY idx_approval_bulk_request (tenant_id,request_id),
  CONSTRAINT fk_approval_bulk_item_batch FOREIGN KEY (tenant_id,batch_id) REFERENCES approval_bulk_batches (tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00048';
