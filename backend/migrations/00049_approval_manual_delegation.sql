-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE approval_delegations
  MODIFY COLUMN valid_to DATETIME(6) NULL,
  ADD COLUMN reason VARCHAR(500) NOT NULL DEFAULT '',
  ADD COLUMN created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD COLUMN revoked_at DATETIME(6) NULL,
  ADD COLUMN revoked_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD KEY idx_approval_delegation_window (tenant_id,from_user_id,request_type,valid_from,valid_to,revoked_at),
  ADD CONSTRAINT fk_approval_delegation_actor FOREIGN KEY (tenant_id,created_by) REFERENCES tenant_memberships (tenant_id,user_id),
  ADD CONSTRAINT fk_approval_delegation_revoker FOREIGN KEY (tenant_id,revoked_by) REFERENCES tenant_memberships (tenant_id,user_id),
  DROP CONSTRAINT ck_delegation_dates,
  ADD CONSTRAINT ck_delegation_dates CHECK (valid_to IS NULL OR valid_to>=valid_from);

CREATE TABLE approval_reassignments (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  instance_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  from_task_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  to_task_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  actor_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reason VARCHAR(500) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_approval_reassign_from (tenant_id,from_task_id),
  CONSTRAINT fk_approval_reassign_instance FOREIGN KEY (tenant_id,instance_id) REFERENCES approval_instances (tenant_id,id),
  CONSTRAINT fk_approval_reassign_from FOREIGN KEY (tenant_id,from_task_id) REFERENCES approval_tasks (tenant_id,id),
  CONSTRAINT fk_approval_reassign_to FOREIGN KEY (tenant_id,to_task_id) REFERENCES approval_tasks (tenant_id,id),
  CONSTRAINT fk_approval_reassign_actor FOREIGN KEY (tenant_id,actor_user_id) REFERENCES tenant_memberships (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00049';
