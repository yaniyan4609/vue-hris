-- +goose NO TRANSACTION
-- +goose Up
-- Compare local clock bounds as seconds relative to work_date. Application
-- materialization additionally converts these bounds with the office zone.
ALTER TABLE shift_templates
  ADD CONSTRAINT ck_shift_expected_order CHECK ((crosses_midnight=FALSE AND end_time > start_time) OR (crosses_midnight=TRUE AND end_time <= start_time)),
  ADD CONSTRAINT ck_shift_in_window_order CHECK (
    clock_in_window_end_day_offset*86400 + TIME_TO_SEC(clock_in_window_end_time) >=
    clock_in_window_start_day_offset*86400 + TIME_TO_SEC(clock_in_window_start_time)),
  ADD CONSTRAINT ck_shift_out_window_order CHECK (
    clock_out_window_end_day_offset*86400 + TIME_TO_SEC(clock_out_window_end_time) >=
    clock_out_window_start_day_offset*86400 + TIME_TO_SEC(clock_out_window_start_time));

ALTER TABLE work_schedules
  ADD CONSTRAINT ck_roster_publish_time CHECK ((status='published' AND published_at IS NOT NULL) OR (status<>'published' AND published_at IS NULL));

-- +goose Down
-- Constraint removal requires a reviewed compatibility migration.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00023';
