-- +goose NO TRANSACTION
-- +goose Up
-- Keep legacy names and names already used by the API validator.
ALTER TABLE approval_flow_steps
  DROP CONSTRAINT ck_approval_step_kind,
  ADD CONSTRAINT ck_approval_step_kind CHECK (approver_kind IN ('manager','reports_to','role','user','membership','fallback_role','position'));

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00051';
