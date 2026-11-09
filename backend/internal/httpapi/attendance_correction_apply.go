package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
)

// The approved correction appends an event. Device events remain immutable.
func (a adminAPI) applyApprovedAttendanceCorrection(r *http.Request, tx *sql.Tx, c adminContext, d overtimeDecision) error {
	var instanceID, requestedType, status string
	var requestedAt time.Time
	err := tx.QueryRowContext(r.Context(), `SELECT shift_instance_id,requested_type,requested_time,status FROM attendance_corrections WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.tenant.ID, d.RefID, d.RequestID, d.Employee.ID).Scan(&instanceID, &requestedType, &requestedAt, &status)
	if err != nil || status != "in_review" {
		return errors.New("koreksi sudah berubah")
	}
	instance, err := scanInstance(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(instanceColumns, "i.", "")+` FROM shift_instances WHERE tenant_id=? AND id=? AND employee_id=? AND status<>'cancelled' FOR UPDATE`, c.tenant.ID, instanceID, d.Employee.ID))
	if err != nil {
		return errors.New("shift koreksi tidak tersedia")
	}
	if requestedAt.Before(instance.ExpectedStart.Add(-24*time.Hour)) || requestedAt.After(instance.ExpectedEnd.Add(24*time.Hour)) {
		return errors.New("waktu koreksi di luar shift")
	}
	eventType := "correction_in"
	timing := "on_time"
	if requestedType == "clock_in" {
		var rule struct {
			Shift struct {
				LateToleranceMin int `json:"late_tolerance_min"`
			} `json:"shift"`
		}
		if err = json.Unmarshal(instance.RuleSnapshot, &rule); err != nil {
			return errors.New("snapshot shift tidak valid")
		}
		if requestedAt.After(instance.ExpectedStart.Add(time.Duration(rule.Shift.LateToleranceMin) * time.Minute)) {
			timing = "late"
		}
	} else if requestedType == "clock_out" {
		eventType = "correction_out"
		if requestedAt.Before(instance.ExpectedEnd) {
			timing = "early_leave"
		}
	} else {
		return errors.New("jenis koreksi tidak valid")
	}
	eventID, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO attendance_events (tenant_id,id,employee_id,shift_instance_id,event_type,occurred_at,location_status,timing_status,source,client_event_id) VALUES (?,?,?,?,?,?,'unknown',?,'correction',?)`, c.tenant.ID, eventID, d.Employee.ID, instanceID, eventType, requestedAt, timing, "correction:"+d.RefID)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE attendance_corrections SET status='approved',result_event_id=?,version=version+1 WHERE tenant_id=? AND id=? AND request_id=? AND status='in_review'`, eventID, c.tenant.ID, d.RefID, d.RequestID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return errors.New("koreksi sudah berubah")
	}
	return recalculateAttendanceDay(r.Context(), tx, c.tenant.ID, instance)
}
