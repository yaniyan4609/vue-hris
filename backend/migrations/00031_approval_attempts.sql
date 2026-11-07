-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE approval_flows
  ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1;

ALTER TABLE approval_flow_steps
  DROP CONSTRAINT ck_approval_step_kind,
  ADD CONSTRAINT ck_approval_step_kind CHECK (approver_kind IN ('manager','role','user','fallback_role','position'));

ALTER TABLE requests
  ADD COLUMN attempt_no INT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  DROP CONSTRAINT ck_requests_status,
  ADD CONSTRAINT ck_requests_status CHECK (status IN ('draft','submitted','in_review','returned','approved','rejected','cancelled'));

ALTER TABLE approval_instances
  ADD COLUMN attempt_no INT UNSIGNED NOT NULL DEFAULT 1,
  DROP CONSTRAINT ck_approval_instance_status,
  ADD CONSTRAINT ck_approval_instance_status CHECK (status IN ('in_review','returned','approved','rejected','cancelled'));

ALTER TABLE approval_tasks
  ADD COLUMN attempt_no INT UNSIGNED NOT NULL DEFAULT 1,
  DROP INDEX uq_approval_task_assignee,
  ADD UNIQUE KEY uq_approval_task_assignee_attempt (tenant_id,instance_id,attempt_no,level_no,assignee_user_id),
  DROP CONSTRAINT ck_approval_task_status,
  ADD CONSTRAINT ck_approval_task_status CHECK (status IN ('pending','approved','rejected','returned','skipped','cancelled'));

ALTER TABLE approval_actions
  DROP CONSTRAINT ck_approval_action,
  ADD CONSTRAINT ck_approval_action CHECK (action IN ('approve','reject','return'));

-- +goose Down
-- Approval attempt history is retained. A destructive rollback requires a reviewed migration.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00031';
