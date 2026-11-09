-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE work_schedules ADD KEY idx_roster_materialization (status,work_date,tenant_id);

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00057';
