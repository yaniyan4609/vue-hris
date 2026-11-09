package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// recalculateAttendanceDay derives the daily row from immutable events. Approved
// correction events take precedence over the original device events.
func recalculateAttendanceDay(ctx context.Context, tx *sql.Tx, tenantID string, instance shiftInstance) error {
	rows, err := tx.QueryContext(ctx, `SELECT event_type,occurred_at FROM attendance_events WHERE tenant_id=? AND shift_instance_id=? ORDER BY created_at,id`, tenantID, instance.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var firstRaw, lastRaw, correctedIn, correctedOut *time.Time
	for rows.Next() {
		var kind string
		var at time.Time
		if err = rows.Scan(&kind, &at); err != nil {
			return err
		}
		at = at.UTC()
		switch kind {
		case "clock_in":
			if firstRaw == nil || at.Before(*firstRaw) {
				firstRaw = &at
			}
		case "clock_out":
			if lastRaw == nil || at.After(*lastRaw) {
				lastRaw = &at
			}
		case "correction_in":
			correctedIn = &at
		case "correction_out":
			correctedOut = &at
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	first, last := firstRaw, lastRaw
	if correctedIn != nil {
		first = correctedIn
	}
	if correctedOut != nil {
		last = correctedOut
	}
	var firstValue, lastValue any
	if first != nil {
		firstValue = *first
	}
	if last != nil {
		lastValue = *last
	}
	late, worked := 0, 0
	status := "absent"
	if first == nil && last != nil {
		status = "incomplete"
	}
	if first != nil {
		status = "incomplete"
		var rule struct {
			Shift struct {
				LateToleranceMin int `json:"late_tolerance_min"`
				BreakMinutes     int `json:"break_minutes"`
			} `json:"shift"`
		}
		if err = json.Unmarshal(instance.RuleSnapshot, &rule); err != nil {
			return err
		}
		if first.After(instance.ExpectedStart.Add(time.Duration(rule.Shift.LateToleranceMin) * time.Minute)) {
			late = int(first.Sub(instance.ExpectedStart).Minutes())
		}
		if last != nil && last.After(*first) {
			worked = int(last.Sub(*first).Minutes()) - rule.Shift.BreakMinutes
			if worked < 0 {
				worked = 0
			}
			status = "present"
			if late > 0 {
				status = "late"
			}
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO attendance_daily (tenant_id,employee_id,work_date,shift_instance_id,first_in,last_out,worked_minutes,late_minutes,status,computed_at) VALUES (?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE shift_instance_id=VALUES(shift_instance_id),first_in=VALUES(first_in),last_out=VALUES(last_out),worked_minutes=VALUES(worked_minutes),late_minutes=VALUES(late_minutes),status=VALUES(status),computed_at=UTC_TIMESTAMP(6)`, tenantID, instance.EmployeeID, instance.WorkDate, instance.ID, firstValue, lastValue, worked, late, status)
	return err
}
